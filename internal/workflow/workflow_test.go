package workflow

import (
	"errors"
	"strings"
	"testing"
)

// The matrix below is written out longhand on purpose. It is a second,
// independent statement of the rules, so a change to `edges` cannot slip
// through review by quietly changing the test's expectations with it.
//
// H = human only, A = agent only, B = both, . = no such transition
var matrix = map[Status]map[Status]string{
	//          backlog  queue    active   review   done     blocked
	Backlog: {Backlog: ".", Queue: "H", Active: ".", Review: ".", Done: ".", Blocked: "."},
	Queue:   {Backlog: "H", Queue: ".", Active: "B", Review: ".", Done: ".", Blocked: "."},
	Active:  {Backlog: ".", Queue: ".", Active: ".", Review: "B", Done: ".", Blocked: "B"},
	Review:  {Backlog: ".", Queue: ".", Active: "H", Review: ".", Done: "H", Blocked: "."},
	Done:    {Backlog: ".", Queue: "H", Active: ".", Review: ".", Done: ".", Blocked: "."},
	Blocked: {Backlog: "H", Queue: ".", Active: "B", Review: ".", Done: ".", Blocked: "."},
}

func TestTransitionMatrixIsExhaustive(t *testing.T) {
	for _, actor := range []ActorType{Human, Agent} {
		for _, from := range Statuses() {
			for _, to := range Statuses() {
				want := matrix[from][to]
				allowed := want == "B" ||
					(want == "H" && actor == Human) ||
					(want == "A" && actor == Agent)

				err := Allowed(actor, from, to)
				if allowed && err != nil {
					t.Errorf("%s %s -> %s: want allowed, got %v", actor, from, to, err)
				}
				if !allowed && err == nil {
					t.Errorf("%s %s -> %s: want refused, got nil", actor, from, to)
				}
			}
		}
	}
}

// The two constraints the whole design turns on. Spelled out separately so that
// if someone ever "fixes" the matrix, these fail by name.
func TestAgentsCannotQueueWorkOrDeclareItDone(t *testing.T) {
	if err := Allowed(Agent, Backlog, Queue); err == nil {
		t.Error("agent was allowed to queue its own work")
	}
	if err := Allowed(Agent, Review, Done); err == nil {
		t.Error("agent was allowed to mark work done")
	}
	if err := Allowed(Human, Backlog, Queue); err != nil {
		t.Errorf("human cannot queue work: %v", err)
	}
	if err := Allowed(Human, Review, Done); err != nil {
		t.Errorf("human cannot finish work: %v", err)
	}
}

func TestRefusalsAreClassified(t *testing.T) {
	cases := []struct {
		name           string
		actor          ActorType
		from, to       Status
		want           Reason
		messageMustSay string
	}{
		{"reserved for human", Agent, Review, Done, ReasonActorForbidden, "only the human marks work done"},
		{"queueing is the human's call", Agent, Backlog, Queue, ReasonActorForbidden, "only the human decides"},
		{"not in the graph", Human, Backlog, Done, ReasonNoSuchTransition, "not a transition"},
		{"already there", Agent, Active, Active, ReasonAlreadyThere, "already active"},
		{"nonsense status", Agent, Active, Status("shipped"), ReasonUnknownStatus, "unknown status"},
		{"nonsense actor", ActorType("robot"), Active, Review, ReasonUnknownActor, "unknown actor"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Allowed(tc.actor, tc.from, tc.to)
			var te *TransitionError
			if !errors.As(err, &te) {
				t.Fatalf("want *TransitionError, got %#v", err)
			}
			if te.Reason != tc.want {
				t.Errorf("reason = %v, want %v", te.Reason, tc.want)
			}
			if !strings.Contains(err.Error(), tc.messageMustSay) {
				t.Errorf("message %q does not mention %q", err.Error(), tc.messageMustSay)
			}
		})
	}
}

// A refusal that does not say what to do instead wastes an agent's turn.
func TestRefusalsPointSomewhere(t *testing.T) {
	err := Allowed(Agent, Backlog, Queue)
	if !strings.Contains(err.Error(), "backlog") {
		t.Errorf("refusal does not tell the agent where the task stays: %q", err)
	}
	err = Allowed(Agent, Active, Done)
	if !strings.Contains(err.Error(), "review") || !strings.Contains(err.Error(), "blocked") {
		t.Errorf("refusal does not list the agent's real options: %q", err)
	}
}

// The order is part of the contract: the first move offered is the one that
// carries the work forward, and both surfaces present it first.
func TestNextFor(t *testing.T) {
	cases := []struct {
		actor ActorType
		from  Status
		want  string
	}{
		{Agent, Backlog, ""},
		{Human, Backlog, "queue"},
		{Agent, Queue, "active"},
		{Human, Queue, "active, backlog"},
		{Agent, Active, "review, blocked"},
		{Agent, Review, ""},
		{Human, Review, "done, active"},
		{Agent, Blocked, "active"},
		{Human, Blocked, "active, backlog"},
		{Agent, Done, ""},
		{Human, Done, "queue"},
	}
	for _, tc := range cases {
		if got := join(NextFor(tc.actor, tc.from)); got != tc.want {
			t.Errorf("NextFor(%s, %s) = %q, want %q", tc.actor, tc.from, got, tc.want)
		}
	}
}

func TestRequirements(t *testing.T) {
	if !Requires(Blocked).BlockedOn {
		t.Error("moving to blocked must require a reason")
	}
	if !Requires(Active).ClearsBlockedOn {
		t.Error("resuming work must clear the stale blocker")
	}
	if Requires(Review).BlockedOn || Requires(Review).ClearsBlockedOn {
		t.Error("review should impose no blocker requirement")
	}
}
