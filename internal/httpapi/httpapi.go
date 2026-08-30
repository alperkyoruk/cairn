// Package httpapi is the human's way into Cairn: a JSON API and the embedded
// Vue frontend that talks to it.
//
// There is no permission logic in this package. Handlers authenticate the
// caller, hand the resulting Actor to internal/service, and translate whatever
// comes back. Every rule about who may do what lives in the service layer, so
// the MCP server added in the next step inherits the same model by construction
// rather than by remembering to reimplement it.
package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/alperkyoruk/cairn/internal/service"
)

// sessionCookie holds the same kind of token an agent sends as a bearer
// credential. One credential type, one lookup, one permission model.
const sessionCookie = "cairn_session"

// signInLimit is generous for one person and hostile to a guesser.
const (
	signInLimit  = 10
	signInWindow = 5 * time.Minute
)

type Server struct {
	svc    *service.Service
	assets http.Handler
	mux    *http.ServeMux
	secure bool
	signIn *throttle
}

type Option func(*Server)

// WithSecureCookies marks the session cookie Secure. Off by default because
// Cairn is usually reached over plain HTTP on a private network or localhost,
// where a Secure cookie would simply never be sent.
func WithSecureCookies(on bool) Option { return func(s *Server) { s.secure = on } }

func New(svc *service.Service, assets http.Handler, opts ...Option) *Server {
	s := &Server{
		svc: svc, assets: assets, mux: http.NewServeMux(),
		signIn: newThrottle(signInLimit, signInWindow),
	}
	for _, o := range opts {
		o(s)
	}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	// Unauthenticated: everything needed to become authenticated.
	s.mux.HandleFunc("GET /api/session", s.handleSession)
	s.mux.HandleFunc("POST /api/setup", s.handleSetup)
	s.mux.HandleFunc("POST /api/login", s.handleLogin)
	s.mux.HandleFunc("POST /api/logout", s.handleLogout)

	s.mux.HandleFunc("GET /api/board", s.authed(s.handleBoard))

	s.mux.HandleFunc("GET /api/projects", s.authed(s.handleListProjects))
	s.mux.HandleFunc("POST /api/projects", s.authed(s.handleCreateProject))
	s.mux.HandleFunc("GET /api/projects/{id}", s.authed(s.handleGetProject))
	s.mux.HandleFunc("PATCH /api/projects/{id}", s.authed(s.handleUpdateProject))
	s.mux.HandleFunc("DELETE /api/projects/{id}", s.authed(s.handleDeleteProject))
	s.mux.HandleFunc("GET /api/projects/{id}/tasks", s.authed(s.handleListTasks))

	s.mux.HandleFunc("POST /api/tasks", s.authed(s.handleCreateTask))
	s.mux.HandleFunc("GET /api/tasks/{ref}", s.authed(s.handleGetTask))
	s.mux.HandleFunc("PATCH /api/tasks/{ref}", s.authed(s.handleUpdateTask))
	s.mux.HandleFunc("DELETE /api/tasks/{ref}", s.authed(s.handleDeleteTask))
	s.mux.HandleFunc("POST /api/tasks/{ref}/transition", s.authed(s.handleTransition))
	s.mux.HandleFunc("PUT /api/tasks/{ref}/state", s.authed(s.handleWriteState))
	s.mux.HandleFunc("POST /api/tasks/{ref}/worklog", s.authed(s.handleAppendWorklog))

	s.mux.HandleFunc("GET /api/agents", s.authed(s.handleListAgents))
	s.mux.HandleFunc("POST /api/agents", s.authed(s.handleCreateAgent))
	s.mux.HandleFunc("GET /api/agents/{id}/tokens", s.authed(s.handleListTokens))
	s.mux.HandleFunc("POST /api/agents/{id}/tokens", s.authed(s.handleIssueToken))
	s.mux.HandleFunc("DELETE /api/tokens/{id}", s.authed(s.handleRevokeToken))

	// Anything else is the frontend, including client-side routes.
	s.mux.HandleFunc("/", s.serveFrontend)
}

func (s *Server) serveFrontend(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		// A real route would have matched; this is a typo or a stale client.
		writeError(w, &service.Error{Kind: service.KindNotFound, Msg: "no such endpoint"})
		return
	}
	if s.assets == nil {
		http.NotFound(w, r)
		return
	}
	s.assets.ServeHTTP(w, r)
}

// --- authentication ---------------------------------------------------------

// actorHandler is a handler that cannot run without an authenticated caller.
// Making the Actor an argument rather than something to fetch means a handler
// physically cannot forget to authenticate.
type actorHandler func(http.ResponseWriter, *http.Request, service.Actor)

func (s *Server) authed(h actorHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := checkJSONWrite(r); err != nil {
			writeError(w, err)
			return
		}
		actor, err := s.svc.Authenticate(r.Context(), credential(r))
		if err != nil {
			writeError(w, err)
			return
		}
		h(w, r, actor)
	}
}

// credential takes the caller's secret from either surface: a bearer token for
// agents, a cookie for the browser. They resolve through the same code path.
func credential(r *http.Request) string {
	if h := r.Header.Get("Authorization"); len(h) > 7 && strings.EqualFold(h[:7], "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	if c, err := r.Cookie(sessionCookie); err == nil {
		return c.Value
	}
	return ""
}

// checkJSONWrite is Cairn's entire CSRF defence, and it is enough for what this
// is: the session cookie is SameSite=Lax, so a cross-site POST never carries
// it, and requiring a JSON content type means an attacker cannot fall back to a
// plain HTML form, which can only send form encodings. Anything else needs a
// CORS preflight, and no CORS headers are served.
func checkJSONWrite(r *http.Request) error {
	switch r.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			return &service.Error{
				Kind: service.KindInvalid,
				Msg:  "requests that change something must be sent as application/json",
			}
		}
	}
	return nil
}

func (s *Server) setSession(w http.ResponseWriter, secret string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    secret,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(service.SessionLifetime / time.Second),
	})
}

func (s *Server) clearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: "", Path: "/",
		HttpOnly: true, Secure: s.secure, SameSite: http.SameSiteLaxMode, MaxAge: -1,
	})
}

// --- json -------------------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v != nil {
		json.NewEncoder(w).Encode(v)
	}
}

func decode(r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return &service.Error{Kind: service.KindInvalid, Msg: "could not read request body: " + err.Error()}
	}
	return nil
}

type errorBody struct {
	Error struct {
		Kind    string `json:"kind"`
		Message string `json:"message"`
	} `json:"error"`
}

// writeError is the only translation from a service failure to a status code,
// which is why the service layer classifies errors instead of returning strings.
func writeError(w http.ResponseWriter, err error) {
	kind := service.KindOf(err)
	status := http.StatusInternalServerError
	switch kind {
	case service.KindInvalid:
		status = http.StatusBadRequest
	case service.KindUnauthenticated:
		status = http.StatusUnauthorized
	case service.KindForbidden:
		status = http.StatusForbidden
	case service.KindNotFound:
		status = http.StatusNotFound
	case service.KindConflict:
		status = http.StatusConflict
	}

	var body errorBody
	body.Error.Kind = kind.String()
	body.Error.Message = err.Error()
	if kind == service.KindInternal {
		// Do not leak internals to the client; the message is for the log.
		body.Error.Message = "internal error"
	}
	writeJSON(w, status, body)
}
