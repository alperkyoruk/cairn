package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"

	"github.com/alperkyoruk/cairn/internal/id"
	"github.com/alperkyoruk/cairn/internal/model"
	"github.com/alperkyoruk/cairn/internal/store"
	"github.com/alperkyoruk/cairn/internal/workflow"
)

// SessionLifetime is how long a browser session lasts. Agent tokens do not
// expire; they are revoked instead.
const SessionLifetime = 30 * 24 * time.Hour

// touchInterval throttles the last_used_at write. There is a single write
// connection, so recording every authenticated request would put reads behind
// a write queue to learn something we only display to the minute.
const touchInterval = time.Minute

const (
	minPasswordLen = 8
	tokenPrefix    = "cairn_"
)

var nameRE = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,31}$`)

// --- first launch -----------------------------------------------------------

// NeedsSetup reports whether the first-launch screen still has to run. Cairn
// has no registration: there is one human, created once, here.
func (s *Service) NeedsSetup(ctx context.Context) (bool, error) {
	n, err := store.CountActors(ctx, s.read(), workflow.Human)
	if err != nil {
		return false, internal(err)
	}
	return n == 0, nil
}

// Setup creates the single human user. It is unauthenticated because there is
// nobody to authenticate as yet, and it refuses once a human exists — the
// partial unique index on actor backs that up if two callers race.
func (s *Service) Setup(ctx context.Context, username, password string) (Actor, error) {
	if !nameRE.MatchString(username) {
		return Actor{}, invalid("username must be 1-32 characters of letters, digits, dot, dash or underscore")
	}
	if len(password) < minPasswordLen {
		return Actor{}, invalid("password must be at least %d characters", minPasswordLen)
	}

	hash, err := hashPassword(password)
	if err != nil {
		return Actor{}, internal(err)
	}

	actor := model.Actor{ID: id.New(), Type: workflow.Human, Name: username, CreatedAt: s.now()}
	err = s.write(ctx, func(q store.Queryer) error {
		n, err := store.CountActors(ctx, q, workflow.Human)
		if err != nil {
			return internal(err)
		}
		if n > 0 {
			return conflict("this Cairn has already been set up; use --reset-password to recover access")
		}
		if err := store.InsertActor(ctx, q, actor, hash); err != nil {
			return internal(err)
		}
		return nil
	})
	if err != nil {
		return Actor{}, err
	}
	return Actor{id: actor.ID, typ: actor.Type, name: actor.Name}, nil
}

// ResetPassword sets a new password for the human and revokes every credential
// they hold. It takes no Actor: its authorisation *is* access to the database
// file, because it is reachable only from the command line.
func (s *Service) ResetPassword(ctx context.Context, password string) error {
	if len(password) < minPasswordLen {
		return invalid("password must be at least %d characters", minPasswordLen)
	}
	hash, err := hashPassword(password)
	if err != nil {
		return internal(err)
	}
	return s.write(ctx, func(q store.Queryer) error {
		humans, err := store.ListActors(ctx, q, workflow.Human)
		if err != nil {
			return internal(err)
		}
		if len(humans) == 0 {
			return notFound("this Cairn has no user yet; start the server and complete setup")
		}
		if err := store.SetPasswordHash(ctx, q, humans[0].ID, hash); err != nil {
			return internal(err)
		}
		if _, err := store.RevokeTokensFor(ctx, q, humans[0].ID, s.now()); err != nil {
			return internal(err)
		}
		return nil
	})
}

// --- authentication ---------------------------------------------------------

// Login exchanges a password for a session token. The returned secret is shown
// once and stored only as a digest.
func (s *Service) Login(ctx context.Context, username, password string) (string, error) {
	actor, err := store.GetActorByName(ctx, s.read(), username)
	if errors.Is(err, store.ErrNotFound) {
		// Same message either way: do not confirm which half was wrong.
		return "", unauthenticated("incorrect username or password")
	}
	if err != nil {
		return "", internal(err)
	}
	hash, err := store.GetPasswordHash(ctx, s.read(), actor.ID)
	if err != nil {
		return "", internal(err)
	}
	if actor.Type != workflow.Human || !verifyPassword(password, hash) {
		return "", unauthenticated("incorrect username or password")
	}

	expires := s.now().Add(SessionLifetime)
	return s.issue(ctx, actor.ID, "web session", &expires)
}

// Authenticate resolves a presented secret to the actor that holds it. Both the
// web UI (cookie) and MCP (Authorization header) land here, which is why there
// is only one permission model to reason about.
func (s *Service) Authenticate(ctx context.Context, secret string) (Actor, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return Actor{}, unauthenticated("no credential presented")
	}

	token, actor, err := store.GetTokenByHash(ctx, s.read(), hashSecret(secret))
	if errors.Is(err, store.ErrNotFound) {
		return Actor{}, unauthenticated("unknown credential")
	}
	if err != nil {
		return Actor{}, internal(err)
	}
	if token.RevokedAt != nil {
		return Actor{}, unauthenticated("this credential was revoked")
	}
	if token.ExpiresAt != nil && s.now().After(*token.ExpiresAt) {
		return Actor{}, unauthenticated("this credential expired; sign in again")
	}

	if token.LastUsedAt == nil || s.now().Sub(*token.LastUsedAt) > touchInterval {
		// Best effort: failing to record use must not fail the request.
		_ = s.write(ctx, func(q store.Queryer) error {
			return store.TouchToken(ctx, q, token.ID, s.now())
		})
	}

	return Actor{id: actor.ID, typ: actor.Type, name: actor.Name, tokenID: token.ID}, nil
}

// Logout revokes the credential the caller is currently using.
func (s *Service) Logout(ctx context.Context, actor Actor) error {
	if actor.Anonymous() || actor.tokenID == "" {
		return unauthenticated("not signed in")
	}
	return s.write(ctx, func(q store.Queryer) error {
		if err := store.RevokeToken(ctx, q, actor.tokenID, s.now()); err != nil {
			return internal(err)
		}
		return nil
	})
}

// --- agents -----------------------------------------------------------------

// CreateAgent registers an agent and issues its first token. The secret is
// returned once and cannot be recovered; issue another if it is lost.
func (s *Service) CreateAgent(ctx context.Context, actor Actor, name string) (model.Actor, string, error) {
	if err := s.authorize(actor, OpAgentManage); err != nil {
		return model.Actor{}, "", err
	}
	if !nameRE.MatchString(name) {
		return model.Actor{}, "", invalid("agent name must be 1-32 characters of letters, digits, dot, dash or underscore")
	}

	agent := model.Actor{ID: id.New(), Type: workflow.Agent, Name: name, CreatedAt: s.now()}
	secret, err := newSecret()
	if err != nil {
		return model.Actor{}, "", internal(err)
	}

	err = s.write(ctx, func(q store.Queryer) error {
		if _, err := store.GetActorByName(ctx, q, name); err == nil {
			return conflict("an actor named %q already exists", name)
		} else if !errors.Is(err, store.ErrNotFound) {
			return internal(err)
		}
		if err := store.InsertActor(ctx, q, agent, ""); err != nil {
			return internal(err)
		}
		return store.InsertToken(ctx, q, model.Token{
			ID: id.New(), ActorID: agent.ID, Name: "initial token",
			Prefix: displayPrefix(secret), CreatedAt: s.now(),
		}, hashSecret(secret))
	})
	if err != nil {
		return model.Actor{}, "", err
	}
	return agent, secret, nil
}

func (s *Service) ListAgents(ctx context.Context, actor Actor) ([]model.Actor, error) {
	if err := s.authorize(actor, OpRead); err != nil {
		return nil, err
	}
	agents, err := store.ListActors(ctx, s.read(), workflow.Agent)
	if err != nil {
		return nil, internal(err)
	}
	return agents, nil
}

// IssueToken mints an additional credential for an agent.
func (s *Service) IssueToken(ctx context.Context, actor Actor, agentID, name string) (string, error) {
	if err := s.authorize(actor, OpAgentManage); err != nil {
		return "", err
	}
	if strings.TrimSpace(name) == "" {
		return "", invalid("give the token a name, so you know what to revoke later")
	}
	target, err := store.GetActor(ctx, s.read(), agentID)
	if errors.Is(err, store.ErrNotFound) {
		return "", notFound("no agent with id %s", agentID)
	}
	if err != nil {
		return "", internal(err)
	}
	if target.Type != workflow.Agent {
		return "", invalid("tokens are issued to agents; %s is the human user", target.Name)
	}
	return s.issue(ctx, agentID, name, nil)
}

func (s *Service) ListTokens(ctx context.Context, actor Actor, agentID string) ([]model.Token, error) {
	if err := s.authorize(actor, OpAgentManage); err != nil {
		return nil, err
	}
	tokens, err := store.ListTokens(ctx, s.read(), agentID)
	if err != nil {
		return nil, internal(err)
	}
	return tokens, nil
}

func (s *Service) RevokeToken(ctx context.Context, actor Actor, tokenID string) error {
	if err := s.authorize(actor, OpAgentManage); err != nil {
		return err
	}
	return s.write(ctx, func(q store.Queryer) error {
		err := store.RevokeToken(ctx, q, tokenID, s.now())
		if errors.Is(err, store.ErrNotFound) {
			return notFound("no live token with id %s", tokenID)
		}
		if err != nil {
			return internal(err)
		}
		return nil
	})
}

func (s *Service) issue(ctx context.Context, actorID, name string, expires *time.Time) (string, error) {
	secret, err := newSecret()
	if err != nil {
		return "", internal(err)
	}
	err = s.write(ctx, func(q store.Queryer) error {
		return store.InsertToken(ctx, q, model.Token{
			ID: id.New(), ActorID: actorID, Name: name, Prefix: displayPrefix(secret),
			CreatedAt: s.now(), ExpiresAt: expires,
		}, hashSecret(secret))
	})
	if err != nil {
		return "", internal(err)
	}
	return secret, nil
}

// --- secrets ----------------------------------------------------------------

// newSecret returns a fresh credential. The prefix makes it recognisable in a
// log or a config file, and gives secret scanners something to match on.
func newSecret() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return tokenPrefix + base64.RawURLEncoding.EncodeToString(b[:]), nil
}

// displayPrefix returns the part of a token safe to keep in the clear: the
// scheme prefix and six characters of the random body. What remains secret is
// still 32 bytes of entropy minus those six characters, which is not a
// meaningful reduction; what is gained is a handle for telling two of an
// agent's tokens apart.
func displayPrefix(secret string) string {
	const shown = len(tokenPrefix) + 6
	if len(secret) < shown {
		return secret
	}
	return secret[:shown]
}

// hashSecret digests a token. Tokens carry 256 bits of entropy, so a plain hash
// is the right tool: a slow KDF would buy nothing against an unguessable input,
// and we need to look the value up by an indexed column.
func hashSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

// Passwords are the opposite case: short, human-chosen, worth stretching.
const (
	argonTime    = 3
	argonMemory  = 64 * 1024
	argonThreads = 4
	argonKeyLen  = 32
	argonSaltLen = 16
)

func hashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key)), nil
}

func verifyPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var version, memory, times, threads int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &times, &threads); err != nil {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, uint32(times), uint32(memory), uint8(threads), uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}
