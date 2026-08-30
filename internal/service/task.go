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
		// the task had nowhere to come from.
		return store.InsertWorklog(ctx, q, model.WorklogEntry{
			ID: id.New(), TaskID: task.ID, ActorID: actor.id, CreatedAt: now,
			WhatWasTried: "filed this task", Outcome: title, ToStatus: status,
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
		req := workflow.Requires(in.To)
		state := in.State
		if actor.IsAgent() && state == nil {
			return invalid("moving %s requires state: an agent cannot leave a task without "+
				"saying where it left off and what the next step is", task.Ref())
		}
		if state != nil {
			if strings.TrimSpace(state.WhereILeftOff) == "" {
				return invalid("state.where_i_left_off is empty; say what has actually been done")
			}
			if strings.TrimSpace(state.NextStep) == "" {
				return invalid("state.next_step is empty; say what whoever picks this up should do first")
			}
		}
		if req.BlockedOn {
			if state == nil || strings.TrimSpace(state.BlockedOn) == "" {
				return invalid("moving %s to blocked requires state.blocked_on; a task parked "+
					"in blocked with no stated blocker is a dead end", task.Ref())
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

// WriteState overwrites the note without moving the task. This is how an agent
// checkpoints mid-run, so a long piece of work is never one crash away from
// leaving nothing behind.
func (s *Service) WriteState(ctx context.Context, actor Actor, taskID string, in StateInput) error {
	if err := s.authorize(actor, OpStateWrite); err != nil {
		return err
	}
	if strings.TrimSpace(in.WhereILeftOff) == "" {
		return invalid("state.where_i_left_off is empty; say what has actually been done")
	}
	if strings.TrimSpace(in.NextStep) == "" {
		return invalid("state.next_step is empty; say what whoever picks this up should do first")
	}

	return s.write(ctx, func(q store.Queryer) error {
		task, err := s.loadTask(ctx, q, taskID)
		if err != nil {
			return err
		}
		if task.Status == workflow.Blocked && strings.TrimSpace(in.BlockedOn) == "" {
			return invalid("%s is blocked, so state.blocked_on cannot be cleared while it stays there",
				task.Ref())
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

func (s *Service) GetTask(ctx context.Context, actor Actor, taskID string) (TaskDetail, error) {
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
	return s.detail(ctx, actor, task)
}

// GetTaskByRef looks a task up the way it gets talked about: "cairn-12".
func (s *Service) GetTaskByRef(ctx context.Context, actor Actor, ref string) (TaskDetail, error) {
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
	return s.detail(ctx, actor, task)
}

func (s *Service) detail(ctx context.Context, actor Actor, task model.Task) (TaskDetail, error) {
	d := TaskDetail{Task: task, CanMoveTo: workflow.NextFor(actor.typ, task.Status)}

	state, err := store.GetState(ctx, s.read(), task.ID)
	if err == nil {
		d.State = &state
	} else if !errors.Is(err, store.ErrNotFound) {
		return TaskDetail{}, internal(err)
	}

	if d.Worklog, err = store.ListWorklog(ctx, s.read(), task.ID); err != nil {
		return TaskDetail{}, internal(err)
	}
	return d, nil
}

func (s *Service) ListTasks(ctx context.Context, actor Actor, projectID string) ([]model.Task, error) {
	if err := s.authorize(actor, OpRead); err != nil {
		return nil, err
	}
	tasks, err := store.ListTasksByProject(ctx, s.read(), projectID)
	if err != nil {
		return nil, internal(err)
	}
	return tasks, nil
}

// Board is the cross-project main screen: every task, most recently touched
// first, each with the note left on it.
func (s *Service) Board(ctx context.Context, actor Actor) ([]model.BoardRow, error) {
	if err := s.authorize(actor, OpRead); err != nil {
		return nil, err
	}
	rows, err := store.Board(ctx, s.read())
	if err != nil {
		return nil, internal(err)
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
func (s *Service) LookupTask(ctx context.Context, actor Actor, idOrRef string) (TaskDetail, error) {
	if looksLikeID(idOrRef) {
		return s.GetTask(ctx, actor, idOrRef)
	}
	return s.GetTaskByRef(ctx, actor, idOrRef)
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
