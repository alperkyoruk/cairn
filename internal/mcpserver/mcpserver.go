// Package mcpserver exposes Cairn to coding agents over MCP.
//
// It is the second surface onto the same service layer, not a second
// application. Every tool here resolves a bearer token to an Actor and calls
// internal/service, so the rules an agent meets over MCP are the same objects
// the web interface calls -- an agent cannot move a task from review to done
// here for the same reason a button cannot: the one function that writes
// task.status refuses it.
//
// What this package does own is how the rules are *explained*. An agent reads
// tool descriptions and error strings the way a person reads documentation, so
// the prose below is part of the product, not decoration around it.
package mcpserver

import (
	"context"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/alperkyoruk/cairn/internal/service"
)

// instructions are handed to the client on connect. They exist so an agent
// arrives already knowing the two things about Cairn that are not obvious from
// a list of tool names: that leaving a note is mandatory, and that two of the
// decisions are not its to make.
const instructions = `Cairn is an issue tracker built for agents as much as for people.

Every task carries two records, and both matter:

  state   - one note, overwritten each time, always current. Where the work was
            left, what the next step is, and what is blocking it if anything.
            There is exactly one of these per task. Write it as though the next
            person to read it knows nothing about what you just did, because
            that is usually true.

  worklog - an append-only record of attempts. Never edited, never deleted.
            Record what you tried and what happened, including the things that
            did not work: the next agent should not repeat your dead ends.

You cannot move a task without writing state. This is enforced, not advised.

The workflow is: backlog -> queue -> active -> review -> done, with blocked as
a side state that active falls into and returns from.

Two moves are the human's alone and will be refused:
  backlog -> queue  only the human decides what gets worked on
  review  -> done   only the human decides that something is finished

So when you finish a piece of work, move it to review and make sure next_step
says what the human should check. When you are stuck, move it to blocked and
say exactly what you need in blocked_on. Do not leave a task in active when you
stop working: that tells the human work is in progress when it is not.

Every task read includes can_move_to, which lists the moves available to you
from where the task is now. Use it rather than guessing.`

// Version is reported to MCP clients on connect.
const Version = "0.1.0"

type server struct {
	svc *service.Service
}

// New returns the handler to mount at /mcp.
func New(svc *service.Service) http.Handler {
	s := &server{svc: svc}
	built := s.build()

	streamable := mcp.NewStreamableHTTPHandler(
		// The same server serves every request, which the SDK permits. Per-call
		// identity comes from the bearer token on each request rather than from
		// a session, so there is nothing request-scoped to build here.
		func(*http.Request) *mcp.Server { return built },
		&mcp.StreamableHTTPOptions{Stateless: true},
	)
	return s.authenticated(streamable)
}

// authenticated rejects a request with no usable credential before the MCP
// machinery sees it, so a misconfigured client gets a plain 401 instead of a
// successful handshake followed by every tool failing for reasons it cannot
// see. Tools authenticate again to obtain the Actor; that is one indexed
// lookup, and the alternative is smuggling identity through a context in a way
// that would be easy to get subtly wrong.
func (s *server) authenticated(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := s.actor(r.Context(), r.Header); err != nil {
			w.Header().Set("WWW-Authenticate", `Bearer realm="cairn"`)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"cairn: this endpoint needs an agent token. ` +
				`Create one in the web interface under agents, and send it as ` +
				`Authorization: Bearer <token>."}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *server) actor(ctx context.Context, header http.Header) (service.Actor, error) {
	return s.svc.Authenticate(ctx, bearer(header))
}

func bearer(header http.Header) string {
	h := header.Get("Authorization")
	if len(h) > 7 && strings.EqualFold(h[:7], "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}

// bind registers a tool whose handler cannot run without an authenticated
// actor. As in the HTTP layer, identity is an argument rather than something
// the handler fetches, so a tool cannot be written that forgets to check.
func bind[In, Out any](s *server, srv *mcp.Server, tool *mcp.Tool,
	fn func(context.Context, service.Actor, In) (Out, error)) {

	mcp.AddTool(srv, tool, func(ctx context.Context, req *mcp.CallToolRequest, in In) (*mcp.CallToolResult, Out, error) {
		var zero Out

		var header http.Header
		if extra := req.GetExtra(); extra != nil {
			header = extra.Header
		}
		actor, err := s.actor(ctx, header)
		if err != nil {
			return nil, zero, err
		}

		out, err := fn(ctx, actor, in)
		if err != nil {
			// The service's message is already written to be read by an agent:
			// it says what was refused and what to do instead. Passing it
			// through unchanged is the whole point.
			return nil, zero, err
		}
		return nil, out, nil
	})
}
