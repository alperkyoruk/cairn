package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
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
	Ref          string `json:"ref" jsonschema:"the task reference, like cairn-12"`
	WorklogLimit int    `json:"worklog_limit,omitempty" jsonschema:"how many of the most recent worklog entries to return; defaults to 10, and worklog_total always reports how many exist. Pass a larger number when you need the earlier history"`
}

type projectIn struct {
	Project string   `json:"project" jsonschema:"the project slug, like cairn"`
	Status  []string `json:"status,omitempty" jsonschema:"restrict to these statuses; omit to get everything except done"`
	Limit   int      `json:"limit,omitempty" jsonschema:"maximum tasks to return; defaults to 50"`
}

type boardIn struct {
	Status []string `json:"status,omitempty" jsonschema:"restrict to these statuses: backlog, queue, active, review, blocked, done. Omit to get everything except done"`
	Limit  int      `json:"limit,omitempty" jsonschema:"maximum tasks to return; defaults to 50"`
}

type createTaskIn struct {
	Project string `json:"project" jsonschema:"the project slug to file this under, like cairn"`
	Title   string `json:"title" jsonschema:"one line saying what needs doing"`
	Body    string `json:"body,omitempty" jsonschema:"optional detail: context, links, what you noticed"`
}

type transitionIn struct {
	Ref           string `json:"ref" jsonschema:"the task to move, like cairn-12"`
	To            string `json:"to" jsonschema:"the status to move it to: active, review, or blocked"`
	WhereILeftOff string `json:"where_i_left_off" jsonschema:"what has actually been done, in enough detail that someone who was not here can carry on"`
	NextStep      string `json:"next_step" jsonschema:"the single next thing whoever picks this up should do"`
	BlockedOn     string `json:"blocked_on,omitempty" jsonschema:"required when moving to blocked: exactly what you need in order to continue"`
	WhatWasTried  string `json:"what_was_tried,omitempty" jsonschema:"what you attempted during this stretch of work. Required when leaving active for review or blocked, because that is the moment the attempt gets recorded or never does"`
	Outcome       string `json:"outcome,omitempty" jsonschema:"what happened, including failures worth not repeating"`
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
	// Truncated says the limit was reached and there may be more. A shortened
	// list that does not admit it is a list the reader treats as complete.
	Truncated bool `json:"truncated,omitempty"`
}

type taskDetailOut struct {
	Ref     string `json:"ref"`
	Project string `json:"project"`
	Title   string `json:"title"`
	Body    string `json:"body,omitempty"`
	Status  string `json:"status"`

	State   *stateOut    `json:"state"`
	Worklog []worklogOut `json:"worklog"`
	// WorklogTotal is how many entries exist; len(worklog) may be smaller.
	WorklogTotal int `json:"worklog_total"`

	// CanMoveTo is the point of returning detail rather than a bare task: it
	// tells the caller what it may actually do from here, so a transition is
	// never a guess.
	CanMoveTo []string `json:"can_move_to"`
}

// --- registration -----------------------------------------------------------

// defaultLimit caps a listing when the caller does not. Fifty rows is more than
// a person scrolls at once and a small fraction of a context window; an agent
// that needs more can ask, and the result says when it was cut short.
const defaultLimit = 50

// defaultWorklogEntries is how much history a task read returns unasked. The
// recent entries are what stop an agent repeating the last attempt; the rest is
// available by asking, and worklog_total always says how much there is.
const defaultWorklogEntries = 10

// openStatuses is everything except done -- the default for any listing,
// because a finished task is rarely what an agent is looking for.
func openStatuses() []workflow.Status {
	var out []workflow.Status
	for _, st := range workflow.Statuses() {
		if st != workflow.Done {
			out = append(out, st)
		}
	}
	return out
}

func parseStatuses(in []string) ([]workflow.Status, error) {
	if len(in) == 0 {
		return openStatuses(), nil
	}
	out := make([]workflow.Status, 0, len(in))
	for _, raw := range in {
		st := workflow.Status(strings.ToLower(strings.TrimSpace(raw)))
		if !st.Valid() {
			return nil, fmt.Errorf("unknown status %q; the statuses are %s",
				raw, strings.Join(statusNames(), ", "))
		}
		out = append(out, st)
	}
	return out, nil
}

func statusNames() []string {
	out := make([]string, 0, len(workflow.Statuses()))
	for _, st := range workflow.Statuses() {
		out = append(out, string(st))
	}
	return out
}

func limitOr(n int) int {
	if n <= 0 {
		return defaultLimit
	}
	return n
}

// transitionSchema is the inferred schema with the one thing inference cannot
// know bolted on: which statuses an agent may actually name. Leaving `to` as a
// free string invites a call that can only be refused.
func transitionSchema() *jsonschema.Schema {
	schema, err := jsonschema.For[transitionIn](nil)
	if err != nil {
		panic("cairn: cannot infer the transition schema: " + err.Error())
	}
	if to := schema.Properties["to"]; to != nil {
		to.Enum = []any{
			string(workflow.Active), string(workflow.Review), string(workflow.Blocked),
		}
	}
	return schema
}

func (s *server) build() *mcp.Server {
	srv := mcp.NewServer(
		&mcp.Implementation{Name: "cairn", Version: s.version},
		&mcp.ServerOptions{Instructions: instructions},
	)

	bind(s, srv, &mcp.Tool{
		Name:        "list_projects",
		Annotations: readOnly(),
		Description: "List the projects in this Cairn. Task references are built from a " +
			"project slug and a number, so cairn-12 is task 12 in the project with slug cairn.",
	}, s.listProjects, func(o projectsOut) string {
		if len(o.Projects) == 0 {
			return "no projects yet; only the human can create one"
		}
		names := make([]string, 0, len(o.Projects))
		for _, p := range o.Projects {
			names = append(names, p.Slug)
		}
		return fmt.Sprintf("%d projects: %s", len(names), strings.Join(names, ", "))
	})

	bind(s, srv, &mcp.Tool{
		Name:        "board",
		Annotations: readOnly(),
		Description: "Every task across every project, most recently touched first, each with " +
			"the note left on it. Start here: it is the fastest way to see what is in flight, " +
			"what is waiting on the human in review, and what is stuck in blocked. Finished " +
			"work is left out unless you ask for it by status.",
	}, s.board, summariseTasks)

	bind(s, srv, &mcp.Tool{
		Name:        "list_tasks",
		Annotations: readOnly(),
		Description: "Every task in one project, most recently touched first. Finished work is " +
			"left out unless you ask for it by status.",
	}, s.listTasks, summariseTasks)

	bind(s, srv, &mcp.Tool{
		Name:        "get_task",
		Annotations: readOnly(),
		Description: "Everything about one task: the ask, the current note left on it, the most " +
			"recent worklog entries, and can_move_to -- the statuses you may move it to from " +
			"where it is now. Read this before picking work up, and read the worklog before " +
			"retrying anything.",
	}, s.getTask, summariseDetail)

	bind(s, srv, &mcp.Tool{
		Name:        "create_task",
		Annotations: mutating(false),
		Description: "File a new task. Use this when you notice work that needs doing but is not " +
			"what you were asked to do -- write it down rather than silently expanding your " +
			"current task. It lands in backlog. You cannot queue it; the human decides what " +
			"gets worked on.",
	}, s.createTask, summariseDetail)

	bind(s, srv, &mcp.Tool{
		Name:        "transition_task",
		Annotations: mutating(false),
		InputSchema: transitionSchema(),
		Description: "Move a task to a new status, leaving a note behind. where_i_left_off and " +
			"next_step are required and the move is refused without them, because a task you " +
			"walked away from with no note is one the next agent has to reconstruct from " +
			"scratch.\n\n" +
			"Available to you: queue -> active to claim work, active -> review when you are " +
			"finished, active -> blocked when you are stuck, blocked -> active when you are " +
			"unstuck. Leaving active in either direction also requires what_was_tried, and " +
			"moving to blocked requires blocked_on.\n\n" +
			"Refused: backlog -> queue and review -> done. Those are the human's decisions. " +
			"When your work is done, move it to review and say in next_step what they should " +
			"check.",
	}, s.transitionTask, summariseDetail)

	bind(s, srv, &mcp.Tool{
		Name:        "write_state",
		Annotations: mutating(true),
		Description: "Overwrite the note on a task without moving it. Use this to checkpoint " +
			"during a long piece of work, so that if you stop unexpectedly the task still " +
			"says where things stand. It replaces the previous note rather than adding to it.",
	}, s.writeState, summariseDetail)

	bind(s, srv, &mcp.Tool{
		Name:        "append_worklog",
		Annotations: mutating(false),
		Description: "Record one attempt against a task, without moving it. The worklog is " +
			"append-only and permanent. Record failures as readily as successes: an approach " +
			"that did not work is the most useful thing you can leave for whoever tries next.",
	}, s.appendWorklog, summariseDetail)

	return srv
}

// --- summaries --------------------------------------------------------------
//
// One line of prose per result. Without these the SDK fills the text block with
// the serialised output, duplicating the entire payload.

func summariseTasks(o tasksOut) string {
	if len(o.Tasks) == 0 {
		return "no tasks match"
	}
	var waiting, blocked int
	for _, t := range o.Tasks {
		switch t.Status {
		case string(workflow.Review):
			waiting++
		case string(workflow.Blocked):
			blocked++
		}
	}
	parts := []string{fmt.Sprintf("%d tasks", len(o.Tasks))}
	if waiting > 0 {
		parts = append(parts, fmt.Sprintf("%d in review waiting on the human", waiting))
	}
	if blocked > 0 {
		parts = append(parts, fmt.Sprintf("%d blocked", blocked))
	}
	line := strings.Join(parts, ", ")
	if o.Truncated {
		line += " (cut off at the limit; narrow by status or raise limit for more)"
	}
	return line
}

func summariseDetail(o taskDetailOut) string {
	line := fmt.Sprintf("%s is %s: %s", o.Ref, o.Status, o.Title)
	if o.State != nil {
		if o.State.BlockedOn != "" {
			line += fmt.Sprintf(". Blocked on: %s", o.State.BlockedOn)
		} else if o.State.NextStep != "" {
			line += fmt.Sprintf(". Next step: %s", o.State.NextStep)
		}
	}
	if o.WorklogTotal > 0 {
		if len(o.Worklog) < o.WorklogTotal {
			line += fmt.Sprintf(". Showing the last %d of %d worklog entries.", len(o.Worklog), o.WorklogTotal)
		} else {
			line += fmt.Sprintf(". %d worklog entries.", o.WorklogTotal)
		}
	}
	if len(o.CanMoveTo) > 0 {
		line += fmt.Sprintf(" You can move it to: %s.", strings.Join(o.CanMoveTo, ", "))
	} else {
		line += " There is nothing you can move it to from here."
	}
	return line
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

func (s *server) board(ctx context.Context, actor service.Actor, in boardIn) (tasksOut, error) {
	statuses, err := parseStatuses(in.Status)
	if err != nil {
		return tasksOut{}, err
	}
	limit := limitOr(in.Limit)
	rows, err := s.svc.Board(ctx, actor, service.BoardQuery{Statuses: statuses, Limit: limit})
	if err != nil {
		return tasksOut{}, err
	}
	out := tasksOut{Tasks: make([]taskOut, 0, len(rows)), Truncated: len(rows) == limit}
	for _, row := range rows {
		out.Tasks = append(out.Tasks, toTaskOut(row.Task, row.State))
	}
	return out, nil
}

func (s *server) listTasks(ctx context.Context, actor service.Actor, in projectIn) (tasksOut, error) {
	statuses, err := parseStatuses(in.Status)
	if err != nil {
		return tasksOut{}, err
	}
	project, err := s.project(ctx, actor, in.Project)
	if err != nil {
		return tasksOut{}, err
	}
	limit := limitOr(in.Limit)
	tasks, err := s.svc.ListTasks(ctx, actor, project.ID, service.BoardQuery{Statuses: statuses, Limit: limit})
	if err != nil {
		return tasksOut{}, err
	}
	out := tasksOut{Tasks: make([]taskOut, 0, len(tasks)), Truncated: len(tasks) == limit}
	for _, t := range tasks {
		out.Tasks = append(out.Tasks, toTaskOut(t, nil))
	}
	return out, nil
}

func (s *server) getTask(ctx context.Context, actor service.Actor, in refIn) (taskDetailOut, error) {
	detail, err := s.svc.LookupTask(ctx, actor, in.Ref, worklogQuery(in.WorklogLimit))
	if err != nil {
		return taskDetailOut{}, err
	}
	return toDetailOut(detail), nil
}

func (s *server) createTask(ctx context.Context, actor service.Actor, in createTaskIn) (taskDetailOut, error) {
	project, err := s.project(ctx, actor, in.Project)
	if err != nil {
		return taskDetailOut{}, err
	}
	task, err := s.svc.CreateTask(ctx, actor, service.CreateTaskInput{
		ProjectID: project.ID, Title: in.Title, Body: in.Body,
	})
	if err != nil {
		return taskDetailOut{}, err
	}
	return s.detailOf(ctx, actor, task.ID)
}

func (s *server) transitionTask(ctx context.Context, actor service.Actor, in transitionIn) (taskDetailOut, error) {
	detail, err := s.svc.LookupTask(ctx, actor, in.Ref, service.TaskQuery{WorklogLimit: 1})
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
	return s.detailOf(ctx, actor, detail.Task.ID)
}

func (s *server) writeState(ctx context.Context, actor service.Actor, in writeStateIn) (taskDetailOut, error) {
	detail, err := s.svc.LookupTask(ctx, actor, in.Ref, service.TaskQuery{WorklogLimit: 1})
	if err != nil {
		return taskDetailOut{}, err
	}
	if err := s.svc.WriteState(ctx, actor, detail.Task.ID, service.StateInput{
		WhereILeftOff: in.WhereILeftOff, NextStep: in.NextStep, BlockedOn: in.BlockedOn,
	}); err != nil {
		return taskDetailOut{}, err
	}
	return s.detailOf(ctx, actor, detail.Task.ID)
}

func (s *server) appendWorklog(ctx context.Context, actor service.Actor, in appendWorklogIn) (taskDetailOut, error) {
	detail, err := s.svc.LookupTask(ctx, actor, in.Ref, service.TaskQuery{WorklogLimit: 1})
	if err != nil {
		return taskDetailOut{}, err
	}
	if err := s.svc.AppendWorklog(ctx, actor, detail.Task.ID, service.WorklogInput{
		WhatWasTried: in.WhatWasTried, Outcome: in.Outcome,
	}); err != nil {
		return taskDetailOut{}, err
	}
	return s.detailOf(ctx, actor, detail.Task.ID)
}

// detailOf is the read every write answers with, so a caller never has to make
// a second round trip to see what it just did.
func (s *server) detailOf(ctx context.Context, actor service.Actor, taskID string) (taskDetailOut, error) {
	detail, err := s.svc.GetTask(ctx, actor, taskID, worklogQuery(0))
	if err != nil {
		return taskDetailOut{}, err
	}
	return toDetailOut(detail), nil
}

func worklogQuery(limit int) service.TaskQuery {
	if limit <= 0 {
		limit = defaultWorklogEntries
	}
	return service.TaskQuery{WorklogLimit: limit}
}

// project resolves a slug, naming the real projects when it cannot.
func (s *server) project(ctx context.Context, actor service.Actor, slug string) (model.Project, error) {
	projects, err := s.svc.ListProjects(ctx, actor)
	if err != nil {
		return model.Project{}, err
	}
	for _, p := range projects {
		if p.Slug == slug {
			return p, nil
		}
	}
	return model.Project{}, unknownProject(slug, projects)
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
		Worklog:      make([]worklogOut, 0, len(d.Worklog)),
		WorklogTotal: d.WorklogTotal,
		CanMoveTo:    make([]string, 0, len(d.CanMoveTo)),
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
