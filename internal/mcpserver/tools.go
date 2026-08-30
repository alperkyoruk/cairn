package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/alperkyoruk/cairn/internal/clock"
	"github.com/alperkyoruk/cairn/internal/model"
	"github.com/alperkyoruk/cairn/internal/service"
	"github.com/alperkyoruk/cairn/internal/workflow"
)

// The input structs are flat on purpose. A nested state object would mirror the
// database more neatly, but models put values in the wrong place more often
// when there is a nesting level to get wrong, and a malformed call costs a turn.

type refIn struct {
	Ref string `json:"ref" jsonschema:"the task reference, like cairn-12"`
}

type projectIn struct {
	Project string `json:"project" jsonschema:"the project slug, like cairn"`
}

type createTaskIn struct {
	Project string `json:"project" jsonschema:"the project slug to file this under, like cairn"`
	Title   string `json:"title" jsonschema:"one line saying what needs doing"`
	Body    string `json:"body,omitempty" jsonschema:"optional detail: context, links, what you noticed"`
}

type transitionIn struct {
	Ref           string `json:"ref" jsonschema:"the task to move, like cairn-12"`
	To            string `json:"to" jsonschema:"the status to move it to: queue, active, review, or blocked"`
	WhereILeftOff string `json:"where_i_left_off" jsonschema:"what has actually been done, in enough detail that someone who was not here can carry on"`
	NextStep      string `json:"next_step" jsonschema:"the single next thing whoever picks this up should do"`
	BlockedOn     string `json:"blocked_on,omitempty" jsonschema:"required when moving to blocked: exactly what you need in order to continue"`
	WhatWasTried  string `json:"what_was_tried,omitempty" jsonschema:"optional worklog entry: what you attempted during this stretch of work"`
	Outcome       string `json:"outcome,omitempty" jsonschema:"optional worklog entry: what happened, including failures worth not repeating"`
}

type writeStateIn struct {
	Ref           string `json:"ref" jsonschema:"the task to leave a note on, like cairn-12"`
	WhereILeftOff string `json:"where_i_left_off" jsonschema:"what has actually been done so far"`
	NextStep      string `json:"next_step" jsonschema:"the single next thing to do"`
	BlockedOn     string `json:"blocked_on,omitempty" jsonschema:"what is blocking progress, if anything; required while the task is blocked"`
}

type appendWorklogIn struct {
	Ref          string `json:"ref" jsonschema:"the task this attempt belongs to, like cairn-12"`
	WhatWasTried string `json:"what_was_tried" jsonschema:"what you attempted"`
	Outcome      string `json:"outcome,omitempty" jsonschema:"what happened as a result"`
}

// --- output -----------------------------------------------------------------

type taskOut struct {
	Ref       string `json:"ref"`
	Project   string `json:"project"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	NextStep  string `json:"next_step,omitempty"`
	UpdatedBy string `json:"updated_by,omitempty"`
	UpdatedAt string `json:"updated_at"`
}

type stateOut struct {
	WhereILeftOff string `json:"where_i_left_off"`
	NextStep      string `json:"next_step"`
	BlockedOn     string `json:"blocked_on,omitempty"`
	UpdatedBy     string `json:"updated_by"`
	UpdatedAt     string `json:"updated_at"`
}

type worklogOut struct {
	Actor        string `json:"actor"`
	At           string `json:"at"`
	WhatWasTried string `json:"what_was_tried,omitempty"`
	Outcome      string `json:"outcome,omitempty"`
	FromStatus   string `json:"from_status,omitempty"`
	ToStatus     string `json:"to_status,omitempty"`
}

type projectOut struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type projectsOut struct {
	Projects []projectOut `json:"projects"`
}

type tasksOut struct {
	Tasks []taskOut `json:"tasks"`
}

type taskDetailOut struct {
	Ref     string `json:"ref"`
	Project string `json:"project"`
	Title   string `json:"title"`
	Body    string `json:"body,omitempty"`
	Status  string `json:"status"`

	State   *stateOut    `json:"state"`
	Worklog []worklogOut `json:"worklog"`

	// CanMoveTo is the point of returning detail rather than a bare task: it
	// tells the caller what it may actually do from here, so a transition is
	// never a guess.
	CanMoveTo []string `json:"can_move_to"`
}

// --- registration -----------------------------------------------------------

func (s *server) build() *mcp.Server {
	srv := mcp.NewServer(
		&mcp.Implementation{Name: "cairn", Version: Version},
		&mcp.ServerOptions{Instructions: instructions},
	)

	bind(s, srv, &mcp.Tool{
		Name: "list_projects",
		Description: "List the projects in this Cairn. Task references are built from a " +
			"project slug and a number, so cairn-12 is task 12 in the project with slug cairn.",
	}, s.listProjects)

	bind(s, srv, &mcp.Tool{
		Name: "board",
		Description: "Every task across every project, most recently touched first, each with " +
			"the note left on it. Start here: it is the fastest way to see what is in flight, " +
			"what is waiting on the human in review, and what is stuck in blocked.",
	}, s.board)

	bind(s, srv, &mcp.Tool{
		Name:        "list_tasks",
		Description: "Every task in one project, most recently touched first.",
	}, s.listTasks)

	bind(s, srv, &mcp.Tool{
		Name: "get_task",
		Description: "Everything about one task: the ask, the current note left on it, the full " +
			"worklog of what has already been tried, and can_move_to -- the statuses you may " +
			"move it to from where it is now. Read this before picking work up, and read the " +
			"worklog before retrying anything.",
	}, s.getTask)

	bind(s, srv, &mcp.Tool{
		Name: "create_task",
		Description: "File a new task. Use this when you notice work that needs doing but is not " +
			"what you were asked to do -- write it down rather than silently expanding your " +
			"current task. It lands in backlog. You cannot queue it; the human decides what " +
			"gets worked on.",
	}, s.createTask)

	bind(s, srv, &mcp.Tool{
		Name: "transition_task",
		Description: "Move a task to a new status, leaving a note behind. where_i_left_off and " +
			"next_step are required and the move is refused without them, because a task you " +
			"walked away from with no note is one the next agent has to reconstruct from " +
			"scratch.\n\n" +
			"Available to you: queue -> active to claim work, active -> review when you are " +
			"finished, active -> blocked when you are stuck, blocked -> active when you are " +
			"unstuck. Moving to blocked also requires blocked_on.\n\n" +
			"Refused: backlog -> queue and review -> done. Those are the human's decisions. " +
			"When your work is done, move it to review and say in next_step what they should " +
			"check.",
	}, s.transitionTask)

	bind(s, srv, &mcp.Tool{
		Name: "write_state",
		Description: "Overwrite the note on a task without moving it. Use this to checkpoint " +
			"during a long piece of work, so that if you stop unexpectedly the task still " +
			"says where things stand. It replaces the previous note rather than adding to it.",
	}, s.writeState)

	bind(s, srv, &mcp.Tool{
		Name: "append_worklog",
		Description: "Record one attempt against a task, without moving it. The worklog is " +
			"append-only and permanent. Record failures as readily as successes: an approach " +
			"that did not work is the most useful thing you can leave for whoever tries next.",
	}, s.appendWorklog)

	return srv
}

// --- handlers ---------------------------------------------------------------

func (s *server) listProjects(ctx context.Context, actor service.Actor, _ struct{}) (projectsOut, error) {
	projects, err := s.svc.ListProjects(ctx, actor)
	if err != nil {
		return projectsOut{}, err
	}
	out := projectsOut{Projects: make([]projectOut, 0, len(projects))}
	for _, p := range projects {
		out.Projects = append(out.Projects, projectOut{Slug: p.Slug, Name: p.Name, Description: p.Description})
	}
	return out, nil
}

func (s *server) board(ctx context.Context, actor service.Actor, _ struct{}) (tasksOut, error) {
	rows, err := s.svc.Board(ctx, actor)
	if err != nil {
		return tasksOut{}, err
	}
	out := tasksOut{Tasks: make([]taskOut, 0, len(rows))}
	for _, row := range rows {
		out.Tasks = append(out.Tasks, toTaskOut(row.Task, row.State))
	}
	return out, nil
}

func (s *server) listTasks(ctx context.Context, actor service.Actor, in projectIn) (tasksOut, error) {
	projects, err := s.svc.ListProjects(ctx, actor)
	if err != nil {
		return tasksOut{}, err
	}
	for _, p := range projects {
		if p.Slug != in.Project {
			continue
		}
		tasks, err := s.svc.ListTasks(ctx, actor, p.ID)
		if err != nil {
			return tasksOut{}, err
		}
		out := tasksOut{Tasks: make([]taskOut, 0, len(tasks))}
		for _, t := range tasks {
			out.Tasks = append(out.Tasks, toTaskOut(t, nil))
		}
		return out, nil
	}
	return tasksOut{}, unknownProject(in.Project, projects)
}

func (s *server) getTask(ctx context.Context, actor service.Actor, in refIn) (taskDetailOut, error) {
	detail, err := s.svc.LookupTask(ctx, actor, in.Ref)
	if err != nil {
		return taskDetailOut{}, err
	}
	return toDetailOut(detail), nil
}

func (s *server) createTask(ctx context.Context, actor service.Actor, in createTaskIn) (taskDetailOut, error) {
	projects, err := s.svc.ListProjects(ctx, actor)
	if err != nil {
		return taskDetailOut{}, err
	}
	for _, p := range projects {
		if p.Slug != in.Project {
			continue
		}
		task, err := s.svc.CreateTask(ctx, actor, service.CreateTaskInput{
			ProjectID: p.ID, Title: in.Title, Body: in.Body,
		})
		if err != nil {
			return taskDetailOut{}, err
		}
		return s.getTask(ctx, actor, refIn{Ref: task.ID})
	}
	return taskDetailOut{}, unknownProject(in.Project, projects)
}

func (s *server) transitionTask(ctx context.Context, actor service.Actor, in transitionIn) (taskDetailOut, error) {
	detail, err := s.svc.LookupTask(ctx, actor, in.Ref)
	if err != nil {
		return taskDetailOut{}, err
	}

	var worklog *service.WorklogInput
	if in.WhatWasTried != "" || in.Outcome != "" {
		worklog = &service.WorklogInput{WhatWasTried: in.WhatWasTried, Outcome: in.Outcome}
	}

	if _, err := s.svc.Transition(ctx, actor, detail.Task.ID, service.TransitionInput{
		To: workflow.Status(in.To),
		State: &service.StateInput{
			WhereILeftOff: in.WhereILeftOff,
			NextStep:      in.NextStep,
			BlockedOn:     in.BlockedOn,
		},
		Worklog: worklog,
	}); err != nil {
		return taskDetailOut{}, err
	}
	return s.getTask(ctx, actor, refIn{Ref: detail.Task.ID})
}

func (s *server) writeState(ctx context.Context, actor service.Actor, in writeStateIn) (taskDetailOut, error) {
	detail, err := s.svc.LookupTask(ctx, actor, in.Ref)
	if err != nil {
		return taskDetailOut{}, err
	}
	if err := s.svc.WriteState(ctx, actor, detail.Task.ID, service.StateInput{
		WhereILeftOff: in.WhereILeftOff, NextStep: in.NextStep, BlockedOn: in.BlockedOn,
	}); err != nil {
		return taskDetailOut{}, err
	}
	return s.getTask(ctx, actor, refIn{Ref: detail.Task.ID})
}

func (s *server) appendWorklog(ctx context.Context, actor service.Actor, in appendWorklogIn) (taskDetailOut, error) {
	detail, err := s.svc.LookupTask(ctx, actor, in.Ref)
	if err != nil {
		return taskDetailOut{}, err
	}
	if err := s.svc.AppendWorklog(ctx, actor, detail.Task.ID, service.WorklogInput{
		WhatWasTried: in.WhatWasTried, Outcome: in.Outcome,
	}); err != nil {
		return taskDetailOut{}, err
	}
	return s.getTask(ctx, actor, refIn{Ref: detail.Task.ID})
}

// --- shaping ----------------------------------------------------------------

func toTaskOut(t model.Task, state *model.State) taskOut {
	out := taskOut{
		Ref: t.Ref(), Project: t.ProjectSlug, Title: t.Title,
		Status: string(t.Status), UpdatedAt: clock.Format(t.UpdatedAt),
	}
	if state != nil {
		out.NextStep, out.UpdatedBy = state.NextStep, state.UpdatedByName
	}
	return out
}

func toDetailOut(d service.TaskDetail) taskDetailOut {
	out := taskDetailOut{
		Ref: d.Task.Ref(), Project: d.Task.ProjectSlug, Title: d.Task.Title,
		Body: d.Task.Body, Status: string(d.Task.Status),
		Worklog:   make([]worklogOut, 0, len(d.Worklog)),
		CanMoveTo: make([]string, 0, len(d.CanMoveTo)),
	}
	if d.State != nil {
		out.State = &stateOut{
			WhereILeftOff: d.State.WhereILeftOff, NextStep: d.State.NextStep,
			BlockedOn: d.State.BlockedOn, UpdatedBy: d.State.UpdatedByName,
			UpdatedAt: clock.Format(d.State.UpdatedAt),
		}
	}
	for _, e := range d.Worklog {
		out.Worklog = append(out.Worklog, worklogOut{
			Actor: e.ActorName, At: clock.Format(e.CreatedAt),
			WhatWasTried: e.WhatWasTried, Outcome: e.Outcome,
			FromStatus: string(e.FromStatus), ToStatus: string(e.ToStatus),
		})
	}
	for _, s := range d.CanMoveTo {
		out.CanMoveTo = append(out.CanMoveTo, string(s))
	}
	return out
}
