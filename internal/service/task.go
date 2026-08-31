package service

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/alperkyoruk/cairn/internal/id"
	"github.com/alperkyoruk/cairn/internal/model"
	"github.com/alperkyoruk/cairn/internal/store"
	"github.com/alperkyoruk/cairn/internal/workflow"
)

// StateInput is the note left for whoever picks the task up next.
type StateInput struct {
	WhereILeftOff string
	NextStep      string
	BlockedOn     string
}

// WorklogInput is one attempt, recorded forever.
type WorklogInput struct {
	WhatWasTried string
	Outcome      string
}

// TransitionInput moves a task and says what to record while doing it.
//
// State is a pointer because it is optional for the human and mandatory for an
// agent, and that difference is the rule Cairn exists to enforce.
type TransitionInput struct {
	To      workflow.Status
	State   *StateInput
	Worklog *WorklogInput
}

// TaskDetail is everything about one task, including what the reader may do
// with it next. Agents get their legal moves on every read, so they never have
// to guess at a transition and burn a turn being refused.
type TaskDetail struct {
	Task      model.Task
	State     *model.State
	Worklog   []model.WorklogEntry
	CanMoveTo []workflow.Status

	// WorklogTotal is how many entries exist, which differs from len(Worklog)
	// when the caller asked for only the most recent ones. A truncated history
	// that does not say it is truncated reads as a complete one.
	WorklogTotal int
}

// BoardQuery narrows a task listing. Both fields are optional: the web
// interface asks for everything, because scrolling is free, while the MCP
// surface asks for a slice, because a few hundred tasks is a meaningful
// fraction of an agent's context window.
type BoardQuery struct {
	ProjectID string            // empty means every project
	Statuses  []workflow.Status // empty means every status
	Limit     int               // zero means no limit
}

// TaskQuery narrows a single task's detail.
type TaskQuery struct {
	WorklogLimit int // zero means the whole history
}

type CreateTaskInput struct {
	ProjectID string
	Title     string
	Body      string
	Status    workflow.Status // optional; defaults to backlog
}

// CreateTask files a new task.
//
// Agents may file, but only into backlog: noticing follow-up work and writing
// it down is exactly the behaviour we want from an agent, while deciding that
// it gets worked on stays with the human. A task can only enter the workflow at
// backlog or queue; everything past that has to be reached by a transition, so
// the worklog records how it got there.
func (s *Service) CreateTask(ctx context.Context, actor Actor, in CreateTaskInput) (model.Task, error) {
	if err := s.authorize(actor, OpTaskCreate); err != nil {
		return model.Task{}, err
	}
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return model.Task{}, invalid("task needs a title")
	}

	status := in.Status
	if status == "" {
		status = workflow.Backlog
	}
	if !isEntryStatus(status) {
		return model.Task{}, invalid("a task can only be created in %s; move it from there",
			joinStatuses(workflow.EntryStatuses()))
	}
	if actor.IsAgent() && status != workflow.Backlog {
		return model.Task{}, forbidden(
			"agents file tasks into backlog; only the human decides what gets worked on")
	}

	now := s.now()
	task := model.Task{
		ID: id.New(), ProjectID: in.ProjectID, Title: title, Body: in.Body,
		Status: status, CreatedAt: now, UpdatedAt: now,
	}

	err := s.write(ctx, func(q store.Queryer) error {
		project, err := store.GetProject(ctx, q, in.ProjectID)
		if errors.Is(err, store.ErrNotFound) {
			return notFound("no project with id %s", in.ProjectID)
		}
		if err != nil {
			return internal(err)
		}
		task.ProjectSlug = project.Slug

		if task.Number, err = store.NextTaskNumber(ctx, q, in.ProjectID); err != nil {
			return internal(err)
		}
		if err := store.InsertTask(ctx, q, task); err != nil {
			return internal(err)
		}
		// The first worklog entry records who filed it. from_status is null:
		// the task had nowhere to come from. No outcome, because repeating the
		// title here would read as though something had been attempted.
		return store.InsertWorklog(ctx, q, model.WorklogEntry{
			ID: id.New(), TaskID: task.ID, ActorID: actor.id, CreatedAt: now,
			WhatWasTried: "filed this task", ToStatus: status,
		})
	})
	if err != nil {
		return model.Task{}, err
	}
	return task, nil
}

// UpdateTask edits the ask itself, which is the human's to write. Agents record
// what they learn in state and worklog instead of rewriting the request.
func (s *Service) UpdateTask(ctx context.Context, actor Actor, taskID, title, body string) (model.Task, error) {
	if err := s.authorize(actor, OpTaskUpdate); err != nil {
		return model.Task{}, err
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return model.Task{}, invalid("task needs a title")
	}

	var out model.Task
	err := s.write(ctx, func(q store.Queryer) error {
		task, err := s.loadTask(ctx, q, taskID)
		if err != nil {
			return err
		}
		now := s.now()
		if err := store.UpdateTaskFields(ctx, q, task.ID, title, body, now); err != nil {
			return internal(err)
		}
		task.Title, task.Body, task.UpdatedAt = title, body, now
		out = task
		return nil
	})
	return out, err
}

// DeleteTask removes a task and, with it, its state and worklog.
//
// This is the one place the append-only rule ends: the worklog is append-only
// for the life of the task, and there is no way to delete a single entry while
// the task survives.
func (s *Service) DeleteTask(ctx context.Context, actor Actor, taskID string) error {
	if err := s.authorize(actor, OpTaskDelete); err != nil {
		return err
	}
	return s.write(ctx, func(q store.Queryer) error {
		if _, err := s.loadTask(ctx, q, taskID); err != nil {
			return err
		}
		if err := store.DeleteTask(ctx, q, taskID); err != nil {
			return internal(err)
		}
		return nil
	})
}

// Transition moves a task between statuses.
//
// This is the only function in Cairn that writes task.status, and it cannot be
// called without saying what to record. Status, state and worklog are written
// in one transaction, so "an agent cannot leave a task without writing state"
// is not a check that runs alongside the move — it is the shape of the only
// operation that performs one.
func (s *Service) Transition(ctx context.Context, actor Actor, taskID string, in TransitionInput) (model.Task, error) {
	if err := s.authorize(actor, OpTaskTransition); err != nil {
		return model.Task{}, err
	}

	var out model.Task
	err := s.write(ctx, func(q store.Queryer) error {
		task, err := s.loadTask(ctx, q, taskID)
		if err != nil {
			return err
		}

		// Who may make this move, given where the task is now.
		if err := workflow.Allowed(actor.typ, task.Status, in.To); err != nil {
			return transitionError(err)
		}

		// What the move demands of the caller.
		req := workflow.Requires(task.Status, in.To)

		// A copy, because the merge below fills in what the caller left out and
		// that must not reach back into the caller's own struct.
		var state *StateInput
		if in.State != nil {
			merged := *in.State
			state = &merged
		}
		if actor.IsAgent() && state == nil {
			return invalid("moving %s requires state: an agent cannot leave a task without "+
				"saying where it left off and what the next step is", task.Ref())
		}
		if req.BlockedOn {
			if state == nil || strings.TrimSpace(state.BlockedOn) == "" {
				return invalid("moving %s to blocked requires state.blocked_on; a task parked "+
					"in blocked with no stated blocker is a dead end", task.Ref())
			}
		} else if !req.ClearsBlockedOn && state != nil && strings.TrimSpace(state.BlockedOn) != "" {
			// Not every move that carries a blocker is a mistake: returning to
			// active clears a stale one deliberately, so that a task in active
			// never claims to be stuck. What is refused is a blocker that would
			// be stored on a task nobody will read it on -- review, or back to
			// backlog or queue.
			return invalid("moving %s to %s cannot set state.blocked_on: %s",
				task.Ref(), in.To, blockerBelongsInBlocked)
		}
		// Rejecting work binds the human as well. Sending a task back with no
		// reason leaves the agent re-reading its own note, with nothing to say
		// what was wrong with the work it just handed over.
		if req.NextStep {
			if state == nil || strings.TrimSpace(state.NextStep) == "" {
				return invalid("sending %s back to active requires state.next_step; say what "+
					"needs to be different, or the agent picks it up reading its own last note",
					task.Ref())
			}
		}
		// Everything a move demands fresh has now been checked against what the
		// caller actually sent. What is left is completeness, and that a stored
		// value can satisfy: the note has to end up whole, not be rewritten
		// whole every time.
		if state != nil {
			if err := s.inherit(ctx, q, task.ID, state,
				inheritable{whereILeftOff: req.InheritsWhereILeftOff}); err != nil {
				return err
			}
			if strings.TrimSpace(state.WhereILeftOff) == "" {
				return invalid("state.where_i_left_off is empty; say what has actually been done")
			}
			if strings.TrimSpace(state.NextStep) == "" {
				return invalid("state.next_step is empty; say what whoever picks this up should do first")
			}
		}

		// Leaving active is the end of a stretch of work. What is not recorded
		// at that moment does not get recorded afterwards.
		if req.WhatWasTried && actor.IsAgent() {
			if in.Worklog == nil || strings.TrimSpace(in.Worklog.WhatWasTried) == "" {
				return invalid("moving %s out of active requires worklog.what_was_tried; "+
					"record the attempt now, including what did not work", task.Ref())
			}
		}

		now := s.now()

		// Conditional on the status we just read: if anything moved underneath
		// us, we write nothing. Two agents claiming the same queued task means
		// exactly one of them wins.
		moved, err := store.UpdateTaskStatus(ctx, q, task.ID, task.Status, in.To, now)
		if err != nil {
			return internal(err)
		}
		if !moved {
			return conflict("%s moved out of %s while this request was in flight; read it again",
				task.Ref(), task.Status)
		}

		if err := s.recordState(ctx, q, task, actor, state, req, now); err != nil {
			return err
		}

		entry := model.WorklogEntry{
			ID: id.New(), TaskID: task.ID, ActorID: actor.id, CreatedAt: now,
			FromStatus: task.Status, ToStatus: in.To,
		}
		if in.Worklog != nil {
			entry.WhatWasTried, entry.Outcome = in.Worklog.WhatWasTried, in.Worklog.Outcome
		}
		if err := store.InsertWorklog(ctx, q, entry); err != nil {
			return internal(err)
		}

		task.Status, task.UpdatedAt = in.To, now
		out = task
		return nil
	})
	return out, err
}

// recordState writes the note that accompanies a transition.
func (s *Service) recordState(ctx context.Context, q store.Queryer, task model.Task, actor Actor,
	in *StateInput, req workflow.Requirement, now time.Time) error {

	if in == nil {
		// The human may move a task without leaving a note. Resuming work still
		// clears a stale blocker, so a task in active never claims to be stuck.
		if !req.ClearsBlockedOn {
			return nil
		}
		existing, err := store.GetState(ctx, q, task.ID)
		if errors.Is(err, store.ErrNotFound) || existing.BlockedOn == "" {
			return nil
		}
		if err != nil {
			return internal(err)
		}
		existing.BlockedOn, existing.UpdatedBy, existing.UpdatedAt = "", actor.id, now
		if err := store.UpsertState(ctx, q, existing); err != nil {
			return internal(err)
		}
		return nil
	}

	state := model.State{
		TaskID:        task.ID,
		WhereILeftOff: strings.TrimSpace(in.WhereILeftOff),
		NextStep:      strings.TrimSpace(in.NextStep),
		BlockedOn:     strings.TrimSpace(in.BlockedOn),
		UpdatedBy:     actor.id,
		UpdatedAt:     now,
	}
	if req.ClearsBlockedOn {
		state.BlockedOn = ""
	}
	if err := store.UpsertState(ctx, q, state); err != nil {
		return internal(err)
	}
	return nil
}

// inheritable says which fields a particular write may leave out and keep.
//
// It is deliberately per-call rather than a property of the field. A checkpoint
// may say only what changed, because nothing is changing hands. A transition
// may not, because a move is the moment the obligation bites -- with the single
// exception of the human sending work back, who owes a reason and not a
// restatement of what the agent did.
type inheritable struct{ whereILeftOff, nextStep bool }

// inherit fills in the permitted fields a state write left out from what is
// already stored, so a write replaces only what it actually says something
// about.
//
// UpsertState overwrites every column, which made silence indistinguishable
// from erasure and cost both parties in turn. The human sending work back had
// to supply where_i_left_off, so their finding landed on top of the agent's
// account of what it had done; the agent checkpointing mid-run had to restate a
// next step that had not changed. Neither had a way to say "leave that alone".
//
// Note that empty and absent are the same thing here: Go cannot tell them
// apart, and a JSON client sends "" for a field it has nothing to say about. So
// what a move demands fresh has to be refused explicitly rather than left to
// the schema's required keyword, which only checks the key is present.
//
// blocked_on is deliberately never inherited. It already carries three rules --
// required into blocked, cleared on the way out of it, refused anywhere it
// would not be read -- and a fourth that interacted with those would be worse
// than the duplication it saves.
func (s *Service) inherit(ctx context.Context, q store.Queryer, taskID string,
	in *StateInput, allow inheritable) error {

	wants := allow.whereILeftOff && strings.TrimSpace(in.WhereILeftOff) == ""
	wants = wants || (allow.nextStep && strings.TrimSpace(in.NextStep) == "")
	if !wants {
		return nil
	}
	existing, err := store.GetState(ctx, q, taskID)
	if errors.Is(err, store.ErrNotFound) {
		return nil // nothing to inherit; the completeness check will say so
	}
	if err != nil {
		return internal(err)
	}
	if allow.whereILeftOff && strings.TrimSpace(in.WhereILeftOff) == "" {
		in.WhereILeftOff = existing.WhereILeftOff
	}
	if allow.nextStep && strings.TrimSpace(in.NextStep) == "" {
		in.NextStep = existing.NextStep
	}
	return nil
}

// blockerBelongsInBlocked is the tail of both refusals for a blocker written
// somewhere it will not be read as one.
//
// status and state.blocked_on are two records of the same fact and nothing else
// keeps them agreeing, so a blocker set without a move leaves the task claiming
// to be in flight and stuck at once. The value is refused rather than quietly
// dropped: an agent told nothing would believe it had recorded the blocker, and
// go on believing it.
const blockerBelongsInBlocked = "status and state would then disagree about whether the work is " +
	"stuck. If you are stuck, move the task to blocked with transition_task -- that is what puts " +
	"the blocker on the board"

// WriteState overwrites the note without moving the task. This is how an agent
// checkpoints mid-run, so a long piece of work is never one crash away from
// leaving nothing behind.
func (s *Service) WriteState(ctx context.Context, actor Actor, taskID string, in StateInput) error {
	if err := s.authorize(actor, OpStateWrite); err != nil {
		return err
	}
	return s.write(ctx, func(q store.Queryer) error {
		task, err := s.loadTask(ctx, q, taskID)
		if err != nil {
			return err
		}
		// Same rule as a transition: an omitted field keeps what is stored, and
		// what has to be whole is the note that ends up on the task.
		if err := s.inherit(ctx, q, task.ID, &in,
			inheritable{whereILeftOff: true, nextStep: true}); err != nil {
			return err
		}
		if strings.TrimSpace(in.WhereILeftOff) == "" {
			return invalid("state.where_i_left_off is empty; say what has actually been done")
		}
		if strings.TrimSpace(in.NextStep) == "" {
			return invalid("state.next_step is empty; say what whoever picks this up should do first")
		}
		// The guard used to run one way only: it refused to clear a blocker on a
		// blocked task, and said nothing about setting one on a task that is not
		// blocked. That asymmetry is what let an agent record a blocker without
		// moving the task, leaving status and state disagreeing.
		if task.Status == workflow.Blocked && strings.TrimSpace(in.BlockedOn) == "" {
			return invalid("%s is blocked, so state.blocked_on cannot be cleared while it stays there",
				task.Ref())
		}
		if task.Status != workflow.Blocked && strings.TrimSpace(in.BlockedOn) != "" {
			return invalid("%s is %s, not blocked, so state.blocked_on cannot be set on it: %s",
				task.Ref(), task.Status, blockerBelongsInBlocked)
		}
		now := s.now()
		if err := store.UpsertState(ctx, q, model.State{
			TaskID:        task.ID,
			WhereILeftOff: strings.TrimSpace(in.WhereILeftOff),
			NextStep:      strings.TrimSpace(in.NextStep),
			BlockedOn:     strings.TrimSpace(in.BlockedOn),
			UpdatedBy:     actor.id,
			UpdatedAt:     now,
		}); err != nil {
			return internal(err)
		}
		if err := store.TouchTask(ctx, q, task.ID, now); err != nil {
			return internal(err)
		}
		return nil
	})
}

// AppendWorklog records an attempt without moving the task.
func (s *Service) AppendWorklog(ctx context.Context, actor Actor, taskID string, in WorklogInput) error {
	if err := s.authorize(actor, OpWorklogAppend); err != nil {
		return err
	}
	if strings.TrimSpace(in.WhatWasTried) == "" {
		return invalid("worklog.what_was_tried is empty; there is nothing to record")
	}

	return s.write(ctx, func(q store.Queryer) error {
		task, err := s.loadTask(ctx, q, taskID)
		if err != nil {
			return err
		}
		now := s.now()
		if err := store.InsertWorklog(ctx, q, model.WorklogEntry{
			ID: id.New(), TaskID: task.ID, ActorID: actor.id, CreatedAt: now,
			WhatWasTried: strings.TrimSpace(in.WhatWasTried), Outcome: strings.TrimSpace(in.Outcome),
		}); err != nil {
			return internal(err)
		}
		if err := store.TouchTask(ctx, q, task.ID, now); err != nil {
			return internal(err)
		}
		return nil
	})
}

// --- reads ------------------------------------------------------------------

func (s *Service) GetTask(ctx context.Context, actor Actor, taskID string, q TaskQuery) (TaskDetail, error) {
	if err := s.authorize(actor, OpRead); err != nil {
		return TaskDetail{}, err
	}
	task, err := store.GetTask(ctx, s.read(), taskID)
	if errors.Is(err, store.ErrNotFound) {
		return TaskDetail{}, notFound("no task with id %s", taskID)
	}
	if err != nil {
		return TaskDetail{}, internal(err)
	}
	return s.detail(ctx, actor, task, q)
}

// GetTaskByRef looks a task up the way it gets talked about: "cairn-12".
func (s *Service) GetTaskByRef(ctx context.Context, actor Actor, ref string, q TaskQuery) (TaskDetail, error) {
	if err := s.authorize(actor, OpRead); err != nil {
		return TaskDetail{}, err
	}
	slug, number, err := parseRef(ref)
	if err != nil {
		return TaskDetail{}, err
	}
	task, err := store.GetTaskByRef(ctx, s.read(), slug, number)
	if errors.Is(err, store.ErrNotFound) {
		return TaskDetail{}, notFound("no task %s", ref)
	}
	if err != nil {
		return TaskDetail{}, internal(err)
	}
	return s.detail(ctx, actor, task, q)
}

func (s *Service) detail(ctx context.Context, actor Actor, task model.Task, q TaskQuery) (TaskDetail, error) {
	d := TaskDetail{Task: task, CanMoveTo: workflow.NextFor(actor.typ, task.Status)}

	state, err := store.GetState(ctx, s.read(), task.ID)
	if err == nil {
		d.State = &state
	} else if !errors.Is(err, store.ErrNotFound) {
		return TaskDetail{}, internal(err)
	}

	if d.Worklog, err = store.ListWorklog(ctx, s.read(), task.ID, q.WorklogLimit); err != nil {
		return TaskDetail{}, internal(err)
	}
	if d.WorklogTotal, err = store.CountWorklog(ctx, s.read(), task.ID); err != nil {
		return TaskDetail{}, internal(err)
	}
	return d, nil
}

// Board is the main screen: tasks most recently touched first, each with the
// note left on it. With q.ProjectID set it is one project's task list, which is
// the same rows under one more condition -- so it is the same call, and a
// project listing carries the note the board carries.
func (s *Service) Board(ctx context.Context, actor Actor, q BoardQuery) ([]model.BoardRow, error) {
	if err := s.authorize(actor, OpRead); err != nil {
		return nil, err
	}
	rows, err := store.Board(ctx, s.read(), q.ProjectID, q.Statuses, q.Limit)
	if err != nil {
		return nil, internal(err)
	}
	// workflow.NextFor is a map lookup with no query behind it, so telling every
	// row what may be done to it costs nothing per row.
	for i := range rows {
		rows[i].CanMoveTo = workflow.NextFor(actor.typ, rows[i].Task.Status)
	}
	return rows, nil
}

// --- helpers ----------------------------------------------------------------

func (s *Service) loadTask(ctx context.Context, q store.Queryer, taskID string) (model.Task, error) {
	task, err := store.GetTask(ctx, q, taskID)
	if errors.Is(err, store.ErrNotFound) {
		return task, notFound("no task with id %s", taskID)
	}
	if err != nil {
		return task, internal(err)
	}
	return task, nil
}

// transitionError classifies a refusal from the state machine. The distinction
// matters to the caller: "not yours" means hand it to the human, "no such move"
// means the agent misread the workflow, "already there" means someone else won.
func transitionError(err error) error {
	var te *workflow.TransitionError
	if !errors.As(err, &te) {
		return internal(err)
	}
	switch te.Reason {
	case workflow.ReasonActorForbidden:
		return &Error{Kind: KindForbidden, Msg: te.Error(), Err: te}
	case workflow.ReasonAlreadyThere:
		return &Error{Kind: KindConflict, Msg: te.Error(), Err: te}
	default:
		return &Error{Kind: KindInvalid, Msg: te.Error(), Err: te}
	}
}

func isEntryStatus(s workflow.Status) bool {
	for _, e := range workflow.EntryStatuses() {
		if s == e {
			return true
		}
	}
	return false
}

func joinStatuses(ss []workflow.Status) string {
	parts := make([]string, len(ss))
	for i, s := range ss {
		parts[i] = string(s)
	}
	return strings.Join(parts, " or ")
}

// parseRef splits "cairn-12" into its project slug and task number. Slugs may
// contain dashes, so the split is on the last one.
func parseRef(ref string) (string, int, error) {
	ref = strings.TrimSpace(strings.ToLower(ref))
	i := strings.LastIndex(ref, "-")
	if i <= 0 || i == len(ref)-1 {
		return "", 0, invalid("%q is not a task reference; they look like cairn-12", ref)
	}
	number, err := strconv.Atoi(ref[i+1:])
	if err != nil || number < 1 {
		return "", 0, invalid("%q is not a task reference; they look like cairn-12", ref)
	}
	return ref[:i], number, nil
}

// LookupTask resolves whichever way the caller named a task: an id, or the
// short reference people and agents actually use, like "cairn-12". Both the
// HTTP routes and the MCP tools accept either, so the rule for reading one
// lives here rather than in each surface.
func (s *Service) LookupTask(ctx context.Context, actor Actor, idOrRef string, q TaskQuery) (TaskDetail, error) {
	if looksLikeID(idOrRef) {
		return s.GetTask(ctx, actor, idOrRef, q)
	}
	return s.GetTaskByRef(ctx, actor, idOrRef, q)
}

// looksLikeID distinguishes a UUID from a task reference. Both contain dashes,
// so the test is on shape: 8-4-4-4-12 hex digits.
func looksLikeID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return false
			}
		}
	}
	return true
}
