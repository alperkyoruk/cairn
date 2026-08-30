// Package service holds every rule in Cairn and is the only package that
// writes to the database.
//
// The HTTP handlers, the MCP tools and the command line all call in here. None
// of them may import internal/store; TestOnlyServiceReachesTheStore enforces
// that. The point is that the permission model is written once and cannot be
// bypassed by adding a surface: an agent talking MCP and a human clicking a
// button arrive at the same function, with the same checks, in the same order.
package service

import (
	"context"

	"github.com/alperkyoruk/cairn/internal/clock"
	"github.com/alperkyoruk/cairn/internal/store"
	"github.com/alperkyoruk/cairn/internal/workflow"
)

// Service is the application. It is safe for concurrent use.
type Service struct {
	db  *store.DB
	now clock.Clock
}

type Option func(*Service)

// WithClock substitutes the clock, for tests.
func WithClock(c clock.Clock) Option { return func(s *Service) { s.now = c } }

// Open opens the Cairn database at file, applies any pending migrations, and
// returns the service.
//
// This is deliberately the only way to get a Service: it keeps internal/store
// unimported anywhere else in the program, including main, so there is no path
// to the database that skips the rules in this package.
func Open(ctx context.Context, file string, opts ...Option) (*Service, error) {
	db, err := store.Open(file)
	if err != nil {
		return nil, err
	}
	if err := db.Migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return newService(db, opts...), nil
}

func newService(db *store.DB, opts ...Option) *Service {
	s := &Service{db: db, now: clock.System}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Close releases the database.
func (s *Service) Close() error { return s.db.Close() }

// Actor is an authenticated caller.
//
// Its fields are unexported on purpose: an Actor can only be produced by
// Authenticate or Login, so no other package can mint a human and no handler
// can promote itself by filling in a struct. The zero Actor is nobody.
type Actor struct {
	id      string
	typ     workflow.ActorType
	name    string
	tokenID string
}

func (a Actor) ID() string               { return a.id }
func (a Actor) Type() workflow.ActorType { return a.typ }
func (a Actor) Name() string             { return a.name }
func (a Actor) IsHuman() bool            { return a.typ == workflow.Human }
func (a Actor) IsAgent() bool            { return a.typ == workflow.Agent }
func (a Actor) Anonymous() bool          { return a.id == "" }

// Op names something a caller can attempt. Every exported method of Service
// either requires one, or is listed in the test as deliberately unauthenticated.
type Op string

const (
	OpRead           Op = "read"
	OpTaskCreate     Op = "task.create"
	OpTaskUpdate     Op = "task.update"
	OpTaskDelete     Op = "task.delete"
	OpTaskTransition Op = "task.transition"
	OpStateWrite     Op = "state.write"
	OpWorklogAppend  Op = "worklog.append"
	OpProjectManage  Op = "project.manage"
	OpAgentManage    Op = "agent.manage"
)

// policy is the entire permission model, in one readable table.
//
// Note what is *not* here: nothing about which status a task may move to. That
// belongs to internal/workflow, because it depends on where the task is now.
// This table answers only "may this kind of caller attempt this at all".
var policy = map[Op][]workflow.ActorType{
	// Everyone reads everything. There is one human and their agents; hiding
	// tasks from an agent would only make it work with less context.
	OpRead: {workflow.Human, workflow.Agent},

	// Agents may file a task, but only into backlog (enforced in CreateTask).
	// An agent noticing follow-up work and writing it down is the behaviour we
	// want; deciding that it gets worked on stays with the human.
	OpTaskCreate: {workflow.Human, workflow.Agent},

	// Title and body are the human's statement of intent. Agents record what
	// they found in state and worklog rather than editing the ask.
	OpTaskUpdate: {workflow.Human},
	OpTaskDelete: {workflow.Human},

	// Both may move tasks; which moves are legal is workflow's business.
	OpTaskTransition: {workflow.Human, workflow.Agent},
	OpStateWrite:     {workflow.Human, workflow.Agent},
	OpWorklogAppend:  {workflow.Human, workflow.Agent},

	OpProjectManage: {workflow.Human},
	OpAgentManage:   {workflow.Human},
}

func (s *Service) authorize(a Actor, op Op) error {
	if a.Anonymous() || !a.typ.Valid() {
		return unauthenticated("not signed in")
	}
	allowed, ok := policy[op]
	if !ok {
		return internal(errUnknownOp{op})
	}
	for _, t := range allowed {
		if t == a.typ {
			return nil
		}
	}
	return forbidden("%s is reserved for the human; %s is an agent", op, a.name)
}

type errUnknownOp struct{ op Op }

func (e errUnknownOp) Error() string { return "no policy entry for operation " + string(e.op) }

// read is shorthand for the read pool.
func (s *Service) read() store.Queryer { return s.db.Read() }

// write runs fn in a write transaction.
func (s *Service) write(ctx context.Context, fn func(store.Queryer) error) error {
	return s.db.Write(ctx, fn)
}
