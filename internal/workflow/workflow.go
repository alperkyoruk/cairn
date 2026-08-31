// Package workflow is Cairn's state machine.
//
// It is the single place the transition rules are written. It has no
// dependencies beyond the standard library: no database, no HTTP, no service
// types. Everything here is data plus pure functions, so the whole permission
// model can be exercised by a table test with nothing running.
//
// The database knows only which status strings are legal (a CHECK constraint).
// It does not know who may move between them. That question is answered here,
// and every write path in the program routes through it.
package workflow

import (
	"fmt"
	"sort"
	"strings"
)

// Status is a position in the task lifecycle.
//
//	backlog -> queue -> active -> review -> done
//	                      |
//	                   blocked
type Status string

const (
	Backlog Status = "backlog"
	Queue   Status = "queue"
	Active  Status = "active"
	Review  Status = "review"
	Done    Status = "done"
	Blocked Status = "blocked"
)

// Statuses lists every legal status. It must stay in step with the CHECK
// constraint on task.status; TestStatusesMatchSchema enforces that.
func Statuses() []Status {
	return []Status{Backlog, Queue, Active, Review, Done, Blocked}
}

// Valid reports whether s is a status the schema will accept.
func (s Status) Valid() bool {
	for _, k := range Statuses() {
		if s == k {
			return true
		}
	}
	return false
}

func (s Status) String() string { return string(s) }

// ActorType decides which transitions are available. It is derived from the
// token the caller presented, and it is the only input to the permission model:
// there is no per-project or per-task permission in Cairn.
type ActorType string

const (
	Human ActorType = "human"
	Agent ActorType = "agent"
)

func (a ActorType) Valid() bool { return a == Human || a == Agent }

func (a ActorType) String() string { return string(a) }

type edge struct{ from, to Status }

// edges is the whole state machine.
//
// Two transitions are withheld from agents by design:
//
//	backlog -> queue   only the human decides what gets worked on
//	review  -> done    only the human decides something is finished
//
// The rest of the human-only edges exist so the human can run the board:
// send work back from review, deprioritise, and reopen.
var edges = map[edge][]ActorType{
	{Backlog, Queue}:   {Human},
	{Queue, Active}:    {Human, Agent},
	{Active, Review}:   {Human, Agent},
	{Active, Blocked}:  {Human, Agent},
	{Blocked, Active}:  {Human, Agent},
	{Review, Done}:     {Human},
	{Review, Active}:   {Human},
	{Queue, Backlog}:   {Human},
	{Blocked, Backlog}: {Human},
	{Done, Queue}:      {Human},
}

// EntryStatuses are the statuses a task may be created in. Everything else has
// to be reached by a transition, so the worklog records how the task got there.
func EntryStatuses() []Status { return []Status{Backlog, Queue} }

// Allowed reports whether actor may move a task from one status to another.
// A nil return means the move is permitted. Any error is a *TransitionError,
// whose message is written to be read by an agent as much as by a person.
func Allowed(actor ActorType, from, to Status) error {
	if !actor.Valid() {
		return &TransitionError{Actor: actor, From: from, To: to, Reason: ReasonUnknownActor}
	}
	if !from.Valid() || !to.Valid() {
		return &TransitionError{Actor: actor, From: from, To: to, Reason: ReasonUnknownStatus}
	}
	if from == to {
		return &TransitionError{Actor: actor, From: from, To: to, Reason: ReasonAlreadyThere,
			Alternatives: NextFor(actor, from)}
	}
	permitted, exists := edges[edge{from, to}]
	if !exists {
		return &TransitionError{Actor: actor, From: from, To: to, Reason: ReasonNoSuchTransition,
			Alternatives: NextFor(actor, from)}
	}
	for _, p := range permitted {
		if p == actor {
			return nil
		}
	}
	return &TransitionError{Actor: actor, From: from, To: to, Reason: ReasonActorForbidden,
		Alternatives: NextFor(actor, from)}
}

// NextFor lists the statuses actor may move a task to from the given status.
// Agents use it to orient themselves without guessing, and the MCP layer
// reports it back on every task read.
func NextFor(actor ActorType, from Status) []Status {
	var out []Status
	for e, permitted := range edges {
		if e.from != from {
			continue
		}
		for _, p := range permitted {
			if p == actor {
				out = append(out, e.to)
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return progressRank[out[i]] < progressRank[out[j]] })
	return out
}

// progressRank orders the moves available from a status so that the one
// carrying the work forward comes first.
//
// Alphabetical order would put "blocked" ahead of "review" and "active" ahead
// of "done", which is backwards: the first move offered should be the one the
// caller most likely wants. Both surfaces rely on this order -- the interface
// makes the first move its primary button, and an agent reading can_move_to
// sees the main path before the escape hatch.
var progressRank = map[Status]int{
	Done: 1, Review: 2, Active: 3, Queue: 4, Blocked: 5, Backlog: 6,
}

// Requirement describes what a transition demands of the caller beyond
// permission. These are checked by the service layer, which is the only code
// that can see the payload.
//
// The organising idea is that handing a task to somebody else obliges you to
// say why. Every field below marks a move where the task changes hands.
type Requirement struct {
	// BlockedOn requires a non-empty reason, of anyone. A task parked in
	// blocked with no stated blocker is the exact dead end Cairn exists to
	// prevent.
	BlockedOn bool

	// NextStep requires a non-empty next step, of anyone including the human.
	// It marks the moves that hand work back to an agent: sending a task from
	// review to active is a rejection, and a rejection with no reason leaves
	// the agent re-reading its own note and guessing what was wrong with it.
	NextStep bool

	// WhatWasTried requires a worklog entry, of agents. It marks the end of a
	// stretch of work -- what is not written down at that moment is not
	// written down later, and the dead ends are the point of the worklog.
	WhatWasTried bool

	// ClearsBlockedOn wipes any stale blocker when work resumes.
	ClearsBlockedOn bool
}

// Requires reports what a particular move demands. It takes both ends because
// the obligation belongs to the transition, not to the destination: arriving in
// active from queue is picking work up, and arriving there from review is
// handing it back.
func Requires(from, to Status) Requirement {
	var r Requirement

	switch to {
	case Blocked:
		r.BlockedOn = true
	case Active:
		r.ClearsBlockedOn = true
	}

	// The human rejecting work owes the agent a reason.
	if from == Review && to == Active {
		r.NextStep = true
	}

	// Leaving active is the end of a stretch of work, in either direction.
	if from == Active && (to == Review || to == Blocked) {
		r.WhatWasTried = true
	}

	return r
}

// Reason classifies why a transition was refused. The distinction between "that
// move does not exist" and "that move exists but is not yours" is the whole
// point: the first is a mistake, the second is a handoff to the human.
type Reason int

const (
	ReasonUnknownStatus Reason = iota
	ReasonUnknownActor
	ReasonNoSuchTransition
	ReasonActorForbidden
	ReasonAlreadyThere
)

// TransitionError explains a refused move. Agents read error strings the way
// people read documentation, so each message says what happened, why, and what
// to do instead.
type TransitionError struct {
	Actor        ActorType
	From, To     Status
	Reason       Reason
	Alternatives []Status
}

func (e *TransitionError) Error() string {
	switch e.Reason {
	case ReasonUnknownActor:
		return fmt.Sprintf("unknown actor type %q", e.Actor)
	case ReasonUnknownStatus:
		bad := e.To
		if !e.From.Valid() {
			bad = e.From
		}
		return fmt.Sprintf("unknown status %q: valid statuses are %s", bad, join(Statuses()))
	case ReasonAlreadyThere:
		return fmt.Sprintf("task is already %s%s", e.To, e.suffix())
	case ReasonActorForbidden:
		if e.Actor == Agent && e.From == Review && e.To == Done {
			return "only the human marks work done; leave the task in review, " +
				"and make sure state.next_step says what they should check"
		}
		if e.Actor == Agent && e.From == Backlog && e.To == Queue {
			return "only the human decides what gets worked on; leave the task in backlog"
		}
		return fmt.Sprintf("%s -> %s is reserved for the human%s", e.From, e.To, e.suffix())
	default:
		return fmt.Sprintf("%s -> %s is not a transition in this workflow%s", e.From, e.To, e.suffix())
	}
}

func (e *TransitionError) suffix() string {
	if len(e.Alternatives) == 0 {
		return fmt.Sprintf("; from %s you cannot move this task", e.From)
	}
	return fmt.Sprintf("; from %s you can move it to %s", e.From, join(e.Alternatives))
}

func join(ss []Status) string {
	parts := make([]string, len(ss))
	for i, s := range ss {
		parts[i] = string(s)
	}
	return strings.Join(parts, ", ")
}
