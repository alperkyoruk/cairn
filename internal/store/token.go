package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/alperkyoruk/cairn/internal/clock"
	"github.com/alperkyoruk/cairn/internal/model"
	"github.com/alperkyoruk/cairn/internal/workflow"
)

// InsertToken stores a credential. hash is a digest of the secret; the secret
// itself is shown to the caller once and never persisted.
func InsertToken(ctx context.Context, q Queryer, t model.Token, hash string) error {
	_, err := q.ExecContext(ctx,
		`INSERT INTO token (id, actor_id, name, token_hash, prefix, created_at, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.ActorID, t.Name, hash, t.Prefix, clock.Format(t.CreatedAt), storeTime(t.ExpiresAt))
	if err != nil {
		return fmt.Errorf("insert token: %w", err)
	}
	return nil
}

// GetTokenByHash resolves a presented credential to its token and owner in one
// query. This is the join that makes the permission model identical for the
// web UI and for MCP: both arrive here, and both leave with an actor_type.
func GetTokenByHash(ctx context.Context, q Queryer, hash string) (model.Token, model.Actor, error) {
	var t model.Token
	var a model.Actor
	var created, actorType, actorCreated string
	var expires, lastUsed, revoked sql.NullString

	err := q.QueryRowContext(ctx, `
		SELECT tk.id, tk.actor_id, tk.name, tk.created_at, tk.expires_at, tk.last_used_at, tk.revoked_at,
		       a.id, a.actor_type, a.name, a.created_at
		FROM token tk
		JOIN actor a ON a.id = tk.actor_id
		WHERE tk.token_hash = ?`, hash).
		Scan(&t.ID, &t.ActorID, &t.Name, &created, &expires, &lastUsed, &revoked,
			&a.ID, &actorType, &a.Name, &actorCreated)
	if errors.Is(err, sql.ErrNoRows) {
		return t, a, ErrNotFound
	}
	if err != nil {
		return t, a, err
	}

	t.CreatedAt = parseTime(created)
	t.ExpiresAt, t.LastUsedAt, t.RevokedAt = optTime(expires), optTime(lastUsed), optTime(revoked)
	a.Type = workflow.ActorType(actorType)
	a.CreatedAt = parseTime(actorCreated)
	return t, a, nil
}

// TouchToken records use. Called at most once a minute per token: with a single
// write connection, a write on every authenticated request would put every read
// behind the write queue for no benefit.
func TouchToken(ctx context.Context, q Queryer, tokenID string, at time.Time) error {
	_, err := q.ExecContext(ctx, `UPDATE token SET last_used_at = ? WHERE id = ?`,
		clock.Format(at), tokenID)
	return err
}

func RevokeToken(ctx context.Context, q Queryer, tokenID string, at time.Time) error {
	res, err := q.ExecContext(ctx,
		`UPDATE token SET revoked_at = ? WHERE id = ? AND revoked_at IS NULL`,
		clock.Format(at), tokenID)
	if err != nil {
		return fmt.Errorf("revoke token: %w", err)
	}
	return expectOne(res, "token")
}

// RevokeTokensFor kills every live credential an actor holds. Used when the
// password is reset from the command line.
func RevokeTokensFor(ctx context.Context, q Queryer, actorID string, at time.Time) (int, error) {
	res, err := q.ExecContext(ctx,
		`UPDATE token SET revoked_at = ? WHERE actor_id = ? AND revoked_at IS NULL`,
		clock.Format(at), actorID)
	if err != nil {
		return 0, fmt.Errorf("revoke tokens: %w", err)
	}
	n, err := res.RowsAffected()
	return int(n), err
}

func ListTokens(ctx context.Context, q Queryer, actorID string) ([]model.Token, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT id, actor_id, name, prefix, created_at, expires_at, last_used_at, revoked_at
		FROM token WHERE actor_id = ? ORDER BY created_at`, actorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Token
	for rows.Next() {
		var t model.Token
		var created string
		var expires, lastUsed, revoked sql.NullString
		if err := rows.Scan(&t.ID, &t.ActorID, &t.Name, &t.Prefix, &created, &expires, &lastUsed, &revoked); err != nil {
			return nil, err
		}
		t.CreatedAt = parseTime(created)
		t.ExpiresAt, t.LastUsedAt, t.RevokedAt = optTime(expires), optTime(lastUsed), optTime(revoked)
		out = append(out, t)
	}
	return out, rows.Err()
}
