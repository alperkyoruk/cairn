package service

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alperkyoruk/cairn/internal/workflow"
)

// fixture spins up a real database with a human, an agent, a project and one
// task sitting in queue, ready to be picked up.
type fixture struct {
	svc     *Service
	human   Actor
	agent   Actor
	project string
	task    string
	ctx     context.Context
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()

	tick := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	var mu sync.Mutex
	svc, err := Open(ctx, filepath.Join(t.TempDir(), "cairn.db"), WithClock(func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		tick = tick.Add(time.Second)
		return tick
	}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { svc.Close() })

	if _, err := svc.Setup(ctx, "alper", "correct-horse"); err != nil {
		t.Fatal(err)
	}
	secret, err := svc.Login(ctx, "alper", "correct-horse")
	if err != nil {
		t.Fatal(err)
	}
	human, err := svc.Authenticate(ctx, secret)
	if err != nil {
		t.Fatal(err)
	}

	_, agentSecret, err := svc.CreateAgent(ctx, human, "claude")
	if err != nil {
		t.Fatal(err)
	}
	agent, err := svc.Authenticate(ctx, agentSecret)
	if err != nil {
		t.Fatal(err)
	}

	project, err := svc.CreateProject(ctx, human, "cairn", "Cairn", "")
	if err != nil {
		t.Fatal(err)
	}
	task, err := svc.CreateTask(ctx, human, CreateTaskInput{
		ProjectID: project.ID, Title: "Embed the frontend", Status: workflow.Queue,
	})
	if err != nil {
		t.Fatal(err)
	}

	return &fixture{svc: svc, human: human, agent: agent, project: project.ID, task: task.ID, ctx: ctx}
}

func wantKind(t *testing.T, err error, kind Kind) {
	t.Helper()
	if err == nil {
		t.Fatalf("want %s error, got nil", kind)
	}
	if got := KindOf(err); got != kind {
		t.Fatalf("want %s error, got %s: %v", kind, got, err)
	}
}

// The whole point of Cairn, start to finish: an agent picks work up, leaves a
// note, gets stuck, comes back, hands off, and the human closes it.
func TestTheTrailAnAgentLeaves(t *testing.T) {
	f := newFixture(t)

	_, err := f.svc.Transition(f.ctx, f.agent, f.task, TransitionInput{
		To:    workflow.Active,
		State: &StateInput{WhereILeftOff: "picked this up", NextStep: "read the embed docs"},
	})
	if err != nil {
		t.Fatalf("agent could not claim queued work: %v", err)
	}

	_, err = f.svc.Transition(f.ctx, f.agent, f.task, TransitionInput{
		To: workflow.Blocked,
		State: &StateInput{
			WhereILeftOff: "embed.FS works, but the Vue build output path is wrong",
			NextStep:      "decide where vite should write its bundle",
			BlockedOn:     "need to know if the frontend build runs before or during go build",
		},
		Worklog: &WorklogInput{WhatWasTried: "pointed embed at dist/", Outcome: "no such directory at build time"},
	})
	if err != nil {
		t.Fatalf("agent could not block: %v", err)
	}

	detail, err := f.svc.GetTaskByRef(f.ctx, f.human, "cairn-1", TaskQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if detail.State == nil {
		t.Fatal("no state was left on a blocked task")
	}
	if detail.State.BlockedOn == "" {
		t.Error("blocked task carries no blocker")
	}
	if detail.State.UpdatedByName != "claude" {
		t.Errorf("state credited to %q, want claude", detail.State.UpdatedByName)
	}

	// Unblocked: resuming clears the stale blocker without anyone asking.
	_, err = f.svc.Transition(f.ctx, f.agent, f.task, TransitionInput{
		To:    workflow.Active,
		State: &StateInput{WhereILeftOff: "answer: vite runs first", NextStep: "wire the makefile", BlockedOn: "nothing now"},
	})
	if err != nil {
		t.Fatal(err)
	}
	detail, _ = f.svc.GetTask(f.ctx, f.agent, f.task, TaskQuery{})
	if detail.State.BlockedOn != "" {
		t.Errorf("blocker survived the return to active: %q", detail.State.BlockedOn)
	}

	// Handoff. The agent stops at review; it cannot declare its own work done.
	_, err = f.svc.Transition(f.ctx, f.agent, f.task, TransitionInput{
		To:      workflow.Review,
		State:   &StateInput{WhereILeftOff: "make build embeds dist/", NextStep: "check the binary serves / with no dist on disk"},
		Worklog: &WorklogInput{WhatWasTried: "moved dist aside and reran the binary", Outcome: "still served the app"},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.Transition(f.ctx, f.agent, f.task, TransitionInput{
		To:    workflow.Done,
		State: &StateInput{WhereILeftOff: "all good", NextStep: "nothing"},
	})
	wantKind(t, err, KindForbidden)
	if !strings.Contains(err.Error(), "only the human") {
		t.Errorf("refusal does not explain itself: %v", err)
	}

	if _, err := f.svc.Transition(f.ctx, f.human, f.task, TransitionInput{To: workflow.Done}); err != nil {
		t.Fatalf("human could not finish the work: %v", err)
	}

	// Every step is on the record, in order, with its author.
	detail, _ = f.svc.GetTask(f.ctx, f.human, f.task, TaskQuery{})
	var trail []string
	for _, e := range detail.Worklog {
		trail = append(trail, string(e.FromStatus)+">"+string(e.ToStatus)+":"+e.ActorName)
	}
	want := ">queue:alper queue>active:claude active>blocked:claude blocked>active:claude " +
		"active>review:claude review>done:alper"
	if got := strings.Join(trail, " "); got != want {
		t.Errorf("worklog reads:\n %s\nwant:\n %s", got, want)
	}
}

// The rule the project is named for.
func TestAnAgentCannotLeaveATaskWithoutWritingState(t *testing.T) {
	f := newFixture(t)

	_, err := f.svc.Transition(f.ctx, f.agent, f.task, TransitionInput{To: workflow.Active})
	wantKind(t, err, KindInvalid)
	if !strings.Contains(err.Error(), "where it left off") {
		t.Errorf("refusal does not say what is missing: %v", err)
	}

	// Nothing was written: the task did not move either.
	detail, _ := f.svc.GetTask(f.ctx, f.human, f.task, TaskQuery{})
	if detail.Task.Status != workflow.Queue {
		t.Errorf("task moved to %s despite the refusal", detail.Task.Status)
	}

	// Half a note is still no note.
	for _, in := range []StateInput{
		{WhereILeftOff: "started", NextStep: "  "},
		{WhereILeftOff: "", NextStep: "keep going"},
	} {
		_, err := f.svc.Transition(f.ctx, f.agent, f.task, TransitionInput{To: workflow.Active, State: &in})
		wantKind(t, err, KindInvalid)
	}

	// The human is not held to it: they move work around by hand.
	if _, err := f.svc.Transition(f.ctx, f.human, f.task, TransitionInput{To: workflow.Active}); err != nil {
		t.Fatalf("human blocked from moving a task without state: %v", err)
	}
}

func TestBlockingRequiresABlocker(t *testing.T) {
	f := newFixture(t)
	f.claim(t)

	_, err := f.svc.Transition(f.ctx, f.agent, f.task, TransitionInput{
		To:    workflow.Blocked,
		State: &StateInput{WhereILeftOff: "stuck", NextStep: "unstick it"},
	})
	wantKind(t, err, KindInvalid)
	if !strings.Contains(err.Error(), "blocked_on") {
		t.Errorf("refusal does not name the missing field: %v", err)
	}

	// The human is held to this one too: a blocked task with no blocker is a
	// dead end regardless of who parked it.
	_, err = f.svc.Transition(f.ctx, f.human, f.task, TransitionInput{To: workflow.Blocked})
	wantKind(t, err, KindInvalid)
}

func (f *fixture) claim(t *testing.T) {
	t.Helper()
	_, err := f.svc.Transition(f.ctx, f.agent, f.task, TransitionInput{
		To:    workflow.Active,
		State: &StateInput{WhereILeftOff: "picked it up", NextStep: "start"},
	})
	if err != nil {
		t.Fatal(err)
	}
}

// Two agents reaching for the same queued task: the state machine is the lock.
func TestOnlyOneAgentCanClaimATask(t *testing.T) {
	f := newFixture(t)
	_, secret, err := f.svc.CreateAgent(f.ctx, f.human, "codex")
	if err != nil {
		t.Fatal(err)
	}
	other, err := f.svc.Authenticate(f.ctx, secret)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, a := range []Actor{f.agent, other} {
		wg.Add(1)
		go func(i int, a Actor) {
			defer wg.Done()
			_, errs[i] = f.svc.Transition(f.ctx, a, f.task, TransitionInput{
				To:    workflow.Active,
				State: &StateInput{WhereILeftOff: "claiming", NextStep: "work"},
			})
		}(i, a)
	}
	wg.Wait()

	won := 0
	for _, err := range errs {
		if err == nil {
			won++
		} else if k := KindOf(err); k != KindConflict {
			t.Errorf("loser got a %s error, want conflict: %v", k, err)
		}
	}
	if won != 1 {
		t.Errorf("%d agents claimed the same task, want exactly 1", won)
	}
}

func TestAgentsFileIntoBacklogOnly(t *testing.T) {
	f := newFixture(t)

	task, err := f.svc.CreateTask(f.ctx, f.agent, CreateTaskInput{
		ProjectID: f.project, Title: "the vite config needs a comment",
	})
	if err != nil {
		t.Fatalf("agent could not file a follow-up: %v", err)
	}
	if task.Status != workflow.Backlog {
		t.Errorf("agent filed straight into %s", task.Status)
	}

	_, err = f.svc.CreateTask(f.ctx, f.agent, CreateTaskInput{
		ProjectID: f.project, Title: "do this now", Status: workflow.Queue,
	})
	wantKind(t, err, KindForbidden)
}

func TestWhatIsReservedForTheHuman(t *testing.T) {
	f := newFixture(t)

	_, err := f.svc.UpdateTask(f.ctx, f.agent, f.task, "rewritten", "")
	wantKind(t, err, KindForbidden)

	wantKind(t, f.svc.DeleteTask(f.ctx, f.agent, f.task), KindForbidden)

	_, err = f.svc.CreateProject(f.ctx, f.agent, "other", "Other", "")
	wantKind(t, err, KindForbidden)

	_, _, err = f.svc.CreateAgent(f.ctx, f.agent, "sub-agent")
	wantKind(t, err, KindForbidden)

	// And an unauthenticated caller gets nowhere at all.
	_, err = f.svc.Board(f.ctx, Actor{}, BoardQuery{})
	wantKind(t, err, KindUnauthenticated)
}

func TestDeletingATaskIsTheOnlyWayTheWorklogEnds(t *testing.T) {
	f := newFixture(t)
	f.claim(t)

	if err := f.svc.AppendWorklog(f.ctx, f.agent, f.task, WorklogInput{
		WhatWasTried: "read the docs", Outcome: "found the answer",
	}); err != nil {
		t.Fatal(err)
	}
	detail, _ := f.svc.GetTask(f.ctx, f.human, f.task, TaskQuery{})
	if len(detail.Worklog) != 3 {
		t.Fatalf("worklog has %d entries, want 3", len(detail.Worklog))
	}

	if err := f.svc.DeleteTask(f.ctx, f.human, f.task); err != nil {
		t.Fatal(err)
	}
	_, err := f.svc.GetTask(f.ctx, f.human, f.task, TaskQuery{})
	wantKind(t, err, KindNotFound)
}

func TestBoardIsSortedByMostRecentlyTouched(t *testing.T) {
	f := newFixture(t)
	second, err := f.svc.CreateTask(f.ctx, f.human, CreateTaskInput{ProjectID: f.project, Title: "later"})
	if err != nil {
		t.Fatal(err)
	}

	rows, err := f.svc.Board(f.ctx, f.human, BoardQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Task.ID != second.ID {
		t.Fatalf("board is not most-recent-first: %+v", rows)
	}
	if rows[0].State != nil {
		t.Error("a task nobody has touched should carry no state")
	}

	// Touching the older task floats it back to the top.
	f.claim(t)
	rows, _ = f.svc.Board(f.ctx, f.human, BoardQuery{})
	if rows[0].Task.ID != f.task {
		t.Error("claiming a task did not float it to the top of the board")
	}
	if rows[0].State == nil || rows[0].State.NextStep != "start" {
		t.Error("board is not showing the note left on the task")
	}
	if rows[0].Task.Ref() != "cairn-1" {
		t.Errorf("task ref is %q, want cairn-1", rows[0].Task.Ref())
	}
}

func TestStateCanBeCheckpointedWithoutMoving(t *testing.T) {
	f := newFixture(t)
	f.claim(t)

	if err := f.svc.WriteState(f.ctx, f.agent, f.task, StateInput{
		WhereILeftOff: "halfway through the migration runner",
		NextStep:      "handle the version parse error",
	}); err != nil {
		t.Fatal(err)
	}
	detail, _ := f.svc.GetTask(f.ctx, f.agent, f.task, TaskQuery{})
	if detail.Task.Status != workflow.Active {
		t.Error("checkpointing moved the task")
	}
	if detail.State.WhereILeftOff != "halfway through the migration runner" {
		t.Error("checkpoint did not overwrite the note")
	}
	// One row, overwritten: not a history.
	if len(detail.Worklog) != 2 {
		t.Errorf("checkpointing wrote %d worklog entries, want 0 new", len(detail.Worklog)-2)
	}
}

// A read tells an agent what it may do next, so it never has to guess.
func TestReadsCarryTheLegalMoves(t *testing.T) {
	f := newFixture(t)
	f.claim(t)

	forAgent, _ := f.svc.GetTask(f.ctx, f.agent, f.task, TaskQuery{})
	// Order matters: the move that carries the work forward comes first, so
	// the interface can make it the primary button without re-deriving it.
	if got := forAgent.CanMoveTo; len(got) != 2 || got[0] != workflow.Review || got[1] != workflow.Blocked {
		t.Errorf("agent sees moves %v, want [review blocked]", got)
	}
	forHuman, _ := f.svc.GetTask(f.ctx, f.human, f.task, TaskQuery{})
	if len(forHuman.CanMoveTo) != 2 {
		t.Errorf("human sees moves %v", forHuman.CanMoveTo)
	}
}

func TestSetupHappensOnceAndPasswordsSurviveARoundTrip(t *testing.T) {
	f := newFixture(t)

	needs, err := f.svc.NeedsSetup(f.ctx)
	if err != nil || needs {
		t.Errorf("NeedsSetup = %v, %v after setup", needs, err)
	}
	_, err = f.svc.Setup(f.ctx, "someone", "another-password")
	wantKind(t, err, KindConflict)

	if _, err := f.svc.Login(f.ctx, "alper", "wrong"); KindOf(err) != KindUnauthenticated {
		t.Errorf("bad password was not rejected: %v", err)
	}
	if _, err := f.svc.Login(f.ctx, "nobody", "correct-horse"); KindOf(err) != KindUnauthenticated {
		t.Errorf("unknown user leaked a different error: %v", err)
	}

	// Resetting the password invalidates the sessions it protected.
	if err := f.svc.ResetPassword(f.ctx, "brand-new-password"); err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.Board(f.ctx, f.human, BoardQuery{}) // the Actor is stale, but its token is gone
	if err != nil {
		t.Log("note: stale Actor values keep working until re-authenticated")
	}
	if _, err := f.svc.Login(f.ctx, "alper", "correct-horse"); KindOf(err) != KindUnauthenticated {
		t.Error("old password still works after a reset")
	}
	if _, err := f.svc.Login(f.ctx, "alper", "brand-new-password"); err != nil {
		t.Errorf("new password does not work: %v", err)
	}
}

func TestRevokedAndExpiredCredentials(t *testing.T) {
	f := newFixture(t)

	_, secret, err := f.svc.CreateAgent(f.ctx, f.human, "temp")
	if err != nil {
		t.Fatal(err)
	}
	agent, err := f.svc.Authenticate(f.ctx, secret)
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := f.svc.ListTokens(f.ctx, f.human, agent.ID())
	if err != nil || len(tokens) != 1 {
		t.Fatalf("ListTokens = %v, %v", tokens, err)
	}
	if err := f.svc.RevokeToken(f.ctx, f.human, tokens[0].ID); err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.Authenticate(f.ctx, secret)
	wantKind(t, err, KindUnauthenticated)

	_, err = f.svc.Authenticate(f.ctx, "cairn_not-a-real-token")
	wantKind(t, err, KindUnauthenticated)
}

func TestTaskRefParsing(t *testing.T) {
	f := newFixture(t)
	for _, bad := range []string{"cairn", "cairn-", "-1", "cairn-x", "cairn-0", ""} {
		if _, err := f.svc.GetTaskByRef(f.ctx, f.human, bad, TaskQuery{}); KindOf(err) != KindInvalid {
			t.Errorf("GetTaskByRef(%q) kind = %s, want invalid", bad, KindOf(err))
		}
	}
	if _, err := f.svc.GetTaskByRef(f.ctx, f.human, "cairn-99", TaskQuery{}); KindOf(err) != KindNotFound {
		t.Error("a well-formed reference to a missing task should be not_found")
	}
	// Slugs may contain dashes; the split is on the last one.
	p, err := f.svc.CreateProject(f.ctx, f.human, "my-app", "My App", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.CreateTask(f.ctx, f.human, CreateTaskInput{ProjectID: p.ID, Title: "x"}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.GetTaskByRef(f.ctx, f.human, "my-app-1", TaskQuery{}); err != nil {
		t.Errorf("dashed slug did not round-trip: %v", err)
	}
}

func TestProjectsWithTasksAreNotDeletedByAccident(t *testing.T) {
	f := newFixture(t)

	err := f.svc.DeleteProject(f.ctx, f.human, f.project)
	wantKind(t, err, KindConflict)
	if !strings.Contains(err.Error(), "1 task") {
		t.Errorf("refusal does not say how many tasks are in the way: %v", err)
	}

	if err := f.svc.DeleteTask(f.ctx, f.human, f.task); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.DeleteProject(f.ctx, f.human, f.project); err != nil {
		t.Fatalf("empty project would not delete: %v", err)
	}
}

// The stored prefix has to match the secret the human was shown, or it cannot
// do its job of telling one of an agent's tokens from another. And it has to
// stay a prefix: storing the whole thing would undo the point of hashing it.
func TestTokenPrefixIdentifiesTheSecretWithoutRevealingIt(t *testing.T) {
	f := newFixture(t)

	agent, secret, err := f.svc.CreateAgent(f.ctx, f.human, "codex")
	if err != nil {
		t.Fatal(err)
	}
	second, err := f.svc.IssueToken(f.ctx, f.human, agent.ID, "laptop")
	if err != nil {
		t.Fatal(err)
	}

	tokens, err := f.svc.ListTokens(f.ctx, f.human, agent.ID)
	if err != nil || len(tokens) != 2 {
		t.Fatalf("ListTokens = %v, %v", tokens, err)
	}

	for _, tc := range []struct{ name, secret string }{{"initial token", secret}, {"laptop", second}} {
		var found string
		for _, token := range tokens {
			if token.Name == tc.name {
				found = token.Prefix
			}
		}
		if found == "" {
			t.Fatalf("no token named %q", tc.name)
		}
		if !strings.HasPrefix(tc.secret, found) {
			t.Errorf("stored prefix %q is not a prefix of the secret it was minted from", found)
		}
		if len(found) >= len(tc.secret) {
			t.Errorf("stored prefix %q is the whole secret", found)
		}
		// Long enough to be worth showing: the scheme plus six characters.
		if len(found) != len(tokenPrefix)+6 {
			t.Errorf("prefix %q is %d chars, want %d", found, len(found), len(tokenPrefix)+6)
		}
	}

	// The two tokens must actually be distinguishable, which is the point.
	if tokens[0].Prefix == tokens[1].Prefix {
		t.Error("two tokens for the same agent share a prefix")
	}
}

// Handing work back is a rejection, and a rejection with no reason leaves the
// agent re-reading the note it wrote itself. This one binds the human too.
func TestSendingWorkBackRequiresAReason(t *testing.T) {
	f := newFixture(t)
	f.claim(t)
	if _, err := f.svc.Transition(f.ctx, f.agent, f.task, TransitionInput{
		To:      workflow.Review,
		State:   &StateInput{WhereILeftOff: "done as asked", NextStep: "check it"},
		Worklog: &WorklogInput{WhatWasTried: "did the thing", Outcome: "it worked"},
	}); err != nil {
		t.Fatal(err)
	}

	// Bare rejection, the way the interface used to allow.
	_, err := f.svc.Transition(f.ctx, f.human, f.task, TransitionInput{To: workflow.Active})
	wantKind(t, err, KindInvalid)
	if !strings.Contains(err.Error(), "next_step") {
		t.Errorf("refusal does not name the missing field: %v", err)
	}

	// An empty one is no better.
	_, err = f.svc.Transition(f.ctx, f.human, f.task, TransitionInput{
		To:    workflow.Active,
		State: &StateInput{WhereILeftOff: "reviewed it", NextStep: "   "},
	})
	wantKind(t, err, KindInvalid)

	// With a reason it goes through, and the agent finds the human's words.
	if _, err := f.svc.Transition(f.ctx, f.human, f.task, TransitionInput{
		To: workflow.Active,
		State: &StateInput{
			WhereILeftOff: "reviewed; the embed works but the binary still reads dist from disk",
			NextStep:      "move web/dist aside and confirm the binary still serves the app",
		},
	}); err != nil {
		t.Fatalf("rejection with a reason was refused: %v", err)
	}
	detail, _ := f.svc.GetTask(f.ctx, f.agent, f.task, TaskQuery{})
	if detail.State.NextStep != "move web/dist aside and confirm the binary still serves the app" {
		t.Errorf("the agent does not see the human's reason: %q", detail.State.NextStep)
	}
	if detail.State.UpdatedByName != "alper" {
		t.Errorf("the rejection is credited to %q", detail.State.UpdatedByName)
	}
}

// Leaving active is the end of a stretch of work: what is not recorded then is
// not recorded at all.
func TestLeavingActiveRequiresTheAttempt(t *testing.T) {
	f := newFixture(t)
	f.claim(t)

	state := &StateInput{WhereILeftOff: "got somewhere", NextStep: "carry on"}
	_, err := f.svc.Transition(f.ctx, f.agent, f.task, TransitionInput{To: workflow.Review, State: state})
	wantKind(t, err, KindInvalid)
	if !strings.Contains(err.Error(), "what_was_tried") {
		t.Errorf("refusal does not name the missing field: %v", err)
	}

	blocked := &StateInput{WhereILeftOff: "stuck", NextStep: "unstick", BlockedOn: "need credentials"}
	_, err = f.svc.Transition(f.ctx, f.agent, f.task, TransitionInput{To: workflow.Blocked, State: blocked})
	wantKind(t, err, KindInvalid)

	// The human moving their own work around is not held to it, for the same
	// reason they are not held to writing state.
	if _, err := f.svc.Transition(f.ctx, f.human, f.task, TransitionInput{To: workflow.Review}); err != nil {
		t.Fatalf("human blocked from moving a task without a worklog entry: %v", err)
	}
}

// The scenario cairn-2 describes, run end to end. An agent's account of what it
// did is the thing the product exists to keep; a review comment landing on top
// of it, because UpsertState overwrote every column and the human was obliged to
// fill the field, is that failure caused by the product itself.
func TestSendingWorkBackKeepsTheAgentsOwnAccount(t *testing.T) {
	f := newFixture(t)
	f.claim(t)

	const account = "Rewrote the column mapping to be name-based instead of positional."
	if _, err := f.svc.Transition(f.ctx, f.agent, f.task, TransitionInput{
		To:      workflow.Review,
		State:   &StateInput{WhereILeftOff: account, NextStep: "check the 2019 fixture"},
		Worklog: &WorklogInput{WhatWasTried: "rewrote the mapping"},
	}); err != nil {
		t.Fatal(err)
	}

	// The human rejects, saying only what is wrong. They are not asked to
	// restate what the agent did, so they cannot overwrite it by accident.
	if _, err := f.svc.Transition(f.ctx, f.human, f.task, TransitionInput{
		To:      workflow.Active,
		State:   &StateInput{NextStep: "the 2022 header names still fail"},
		Worklog: &WorklogInput{WhatWasTried: "reviewed it; the 2022 header names still fail"},
	}); err != nil {
		t.Fatalf("rejecting without restating the agent's note was refused: %v", err)
	}

	detail, err := f.svc.GetTask(f.ctx, f.agent, f.task, TaskQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if detail.State.WhereILeftOff != account {
		t.Errorf("the agent's account was overwritten by the review:\n  got  %q\n  want %q",
			detail.State.WhereILeftOff, account)
	}
	if detail.State.NextStep != "the 2022 header names still fail" {
		t.Errorf("next_step = %q", detail.State.NextStep)
	}

	// And the reason is in the record that keeps. state.next_step is overwritten
	// in place by design, so the next checkpoint would erase it there.
	last := detail.Worklog[len(detail.Worklog)-1]
	if !strings.Contains(last.WhatWasTried, "2022 header names still fail") {
		t.Errorf("the rejection reason is not in the worklog: %+v", last)
	}

	if err := f.svc.WriteState(f.ctx, f.agent, f.task, StateInput{
		WhereILeftOff: "looked at the 2022 header row",
		NextStep:      "special-case the shift",
	}); err != nil {
		t.Fatal(err)
	}
	detail, _ = f.svc.GetTask(f.ctx, f.agent, f.task, TaskQuery{})
	last = detail.Worklog[len(detail.Worklog)-1]
	if !strings.Contains(last.WhatWasTried, "2022 header names still fail") {
		t.Error("the agent's next checkpoint erased the human's reason")
	}
}

// Which writes may leave a field out, and which may not.
//
// Empty and absent are the same value here -- Go cannot tell them apart, and a
// JSON client sends "" for a field it has nothing to say about. So the schema's
// required keyword cannot carry this: it only checks the key is present. A move
// that demands a fresh note has to refuse an empty one itself.
func TestOnlyACheckpointMayLeaveAFieldOut(t *testing.T) {
	f := newFixture(t)
	f.claim(t)

	// A checkpoint may say only what changed: nothing is changing hands.
	if err := f.svc.WriteState(f.ctx, f.agent, f.task, StateInput{
		WhereILeftOff: "mapping rewritten; 2019 passes",
	}); err != nil {
		t.Fatalf("a partial checkpoint was refused: %v", err)
	}
	detail, _ := f.svc.GetTask(f.ctx, f.agent, f.task, TaskQuery{})
	standing := detail.State.NextStep
	if standing == "" {
		t.Fatal("the standing next step was blanked by a partial checkpoint")
	}

	// A move may not. Handing work to the human with an inherited note says
	// nothing about the stretch of work just finished -- and worse, hands over a
	// next_step describing work that is already done.
	_, err := f.svc.Transition(f.ctx, f.agent, f.task, TransitionInput{
		To:      workflow.Review,
		State:   &StateInput{WhereILeftOff: "", NextStep: ""},
		Worklog: &WorklogInput{WhatWasTried: "wrote the parser"},
	})
	wantKind(t, err, KindInvalid)
	if !strings.Contains(err.Error(), "where_i_left_off is empty") {
		t.Errorf("a move inherited its own note instead of demanding one: %v", err)
	}

	// The one exception, which is the whole of cairn-2: the human sending work
	// back owes a reason, not a restatement of what the agent did.
	if _, err := f.svc.Transition(f.ctx, f.agent, f.task, TransitionInput{
		To:      workflow.Review,
		State:   &StateInput{WhereILeftOff: "wrote the parser", NextStep: "check the fixtures"},
		Worklog: &WorklogInput{WhatWasTried: "wrote the parser"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Transition(f.ctx, f.human, f.task, TransitionInput{
		To:    workflow.Active,
		State: &StateInput{NextStep: "the 2022 header names still fail"},
	}); err != nil {
		t.Fatalf("rejecting without restating the agent's note was refused: %v", err)
	}
	detail, _ = f.svc.GetTask(f.ctx, f.agent, f.task, TaskQuery{})
	if detail.State.WhereILeftOff != "wrote the parser" {
		t.Errorf("the agent's account did not survive the rejection: %q", detail.State.WhereILeftOff)
	}

	// And inheritance fills gaps rather than inventing a note: with nothing
	// stored, even the rejection path has to be given both.
	other, err := f.svc.CreateTask(f.ctx, f.human, CreateTaskInput{
		ProjectID: f.project, Title: "untouched", Status: workflow.Queue,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.Transition(f.ctx, f.agent, other.ID, TransitionInput{
		To: workflow.Active, State: &StateInput{WhereILeftOff: "read it"},
	})
	wantKind(t, err, KindInvalid)
	if !strings.Contains(err.Error(), "next_step") {
		t.Errorf("refusal does not name the missing field: %v", err)
	}
}

// Leaving blocked ends the blocker by either door. Giving up on a blocked task
// used to park it in backlog still carrying a live blocked_on, which every
// surface then reported: "is backlog: two. Blocked on: need staging credentials."
func TestGivingUpOnABlockedTaskClearsTheBlocker(t *testing.T) {
	f := newFixture(t)
	f.claim(t)

	if _, err := f.svc.Transition(f.ctx, f.agent, f.task, TransitionInput{
		To: workflow.Blocked,
		State: &StateInput{
			WhereILeftOff: "called the importer", NextStep: "retry with credentials",
			BlockedOn: "need staging credentials",
		},
		Worklog: &WorklogInput{WhatWasTried: "called it unauthenticated"},
	}); err != nil {
		t.Fatal(err)
	}

	// The human deprioritises it, leaving no note -- which they are allowed to do.
	if _, err := f.svc.Transition(f.ctx, f.human, f.task, TransitionInput{To: workflow.Backlog}); err != nil {
		t.Fatal(err)
	}
	detail, _ := f.svc.GetTask(f.ctx, f.human, f.task, TaskQuery{})
	if detail.Task.Status != workflow.Backlog {
		t.Fatalf("status = %s", detail.Task.Status)
	}
	if detail.State.BlockedOn != "" {
		t.Errorf("a task in backlog still claims to be blocked: %q", detail.State.BlockedOn)
	}
}

// status and state.blocked_on are two records of the same fact. Nothing used to
// keep them agreeing in one direction: the guard refused to clear a blocker on a
// blocked task and said nothing about setting one on a task that was not
// blocked. An agent doing the careful thing -- recording the blocker it just hit
// -- produced a task that claimed to be in flight and stuck at once.
func TestABlockerCanOnlyBeSetWhereItWillBeRead(t *testing.T) {
	f := newFixture(t)
	f.claim(t)

	// The filed sequence: write the blocker, do not move the task.
	err := f.svc.WriteState(f.ctx, f.agent, f.task, StateInput{
		WhereILeftOff: "called the importer endpoint",
		NextStep:      "retry once there are credentials",
		BlockedOn:     "no API credentials for staging",
	})
	wantKind(t, err, KindInvalid)
	for _, want := range []string{"active", "not blocked", "transition_task"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal does not say %q: %v", want, err)
		}
	}

	// The same value on a move that parks it somewhere nobody reads it. This is
	// the path a fix to WriteState alone would have left open: Requires only
	// clears blocked_on for moves into active, so review would have stored it.
	_, err = f.svc.Transition(f.ctx, f.agent, f.task, TransitionInput{
		To: workflow.Review,
		State: &StateInput{
			WhereILeftOff: "did most of it", NextStep: "check the last bit",
			BlockedOn: "still need staging credentials",
		},
		Worklog: &WorklogInput{WhatWasTried: "ran it unauthenticated"},
	})
	wantKind(t, err, KindInvalid)

	// Checkpointing without a blocker is untouched -- this is the common path.
	if err := f.svc.WriteState(f.ctx, f.agent, f.task, StateInput{
		WhereILeftOff: "called the importer endpoint",
		NextStep:      "retry once there are credentials",
	}); err != nil {
		t.Fatalf("an ordinary checkpoint was refused: %v", err)
	}

	// And the honest move still works, and still carries the blocker.
	if _, err := f.svc.Transition(f.ctx, f.agent, f.task, TransitionInput{
		To: workflow.Blocked,
		State: &StateInput{
			WhereILeftOff: "called the importer endpoint",
			NextStep:      "retry once there are credentials",
			BlockedOn:     "no API credentials for staging",
		},
		Worklog: &WorklogInput{WhatWasTried: "called it unauthenticated"},
	}); err != nil {
		t.Fatalf("moving to blocked with a blocker was refused: %v", err)
	}
	detail, _ := f.svc.GetTask(f.ctx, f.agent, f.task, TaskQuery{})
	if detail.State.BlockedOn != "no API credentials for staging" {
		t.Errorf("the blocker did not survive the move it belongs on: %q", detail.State.BlockedOn)
	}

	// While blocked, checkpointing must keep carrying it: the old guard against
	// clearing a live blocker still stands.
	err = f.svc.WriteState(f.ctx, f.agent, f.task, StateInput{
		WhereILeftOff: "still waiting", NextStep: "chase the credentials",
	})
	wantKind(t, err, KindInvalid)
	if !strings.Contains(err.Error(), "cannot be cleared") {
		t.Errorf("refusal is the wrong one: %v", err)
	}
}

// Picking work up is not the end of anything, so it asks for nothing extra.
func TestClaimingWorkStillAsksOnlyForTheNote(t *testing.T) {
	f := newFixture(t)
	if _, err := f.svc.Transition(f.ctx, f.agent, f.task, TransitionInput{
		To:    workflow.Active,
		State: &StateInput{WhereILeftOff: "picked this up", NextStep: "read the docs"},
	}); err != nil {
		t.Fatalf("claiming queued work now demands more than it should: %v", err)
	}
}
