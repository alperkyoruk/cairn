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
	Status  []string `json:"status,omitempty" jsonschema:"restrict to these statuses: backlog, queue, active, review, blocked, done. Omit to get everything except done"`
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
	To            string `json:"to" jsonschema:"the status to move it to: active, review, or blocked. queue and done are missing from this list because they are the human's: a task in backlog waits for them to queue it, and a task you move to review waits for them to mark it done"`
	WhereILeftOff string `json:"where_i_left_off" jsonschema:"what has actually been done, in enough detail that someone who was not here can carry on. When you are claiming queued work, that is what you have read and understood; next_step is what you are about to do first"`
	NextStep      string `json:"next_step" jsonschema:"the single next thing whoever picks this up should do"`
	BlockedOn     string `json:"blocked_on,omitempty" jsonschema:"required when moving to blocked, and only meaningful there: exactly what you need in order to continue"`
	WhatWasTried  string `json:"what_was_tried,omitempty" jsonschema:"required when leaving active, for review or for blocked: what you attempted during this stretch of work, including what did not work. That moment is when the attempt gets recorded or never does"`
	Outcome       string `json:"outcome,omitempty" jsonschema:"what happened, including failures worth not repeating"`
}

// write_state is the checkpoint tool, and unlike a transition it may write part
// of the note: the two fields are optional in the schema and an omitted one
// keeps what is already stored. An agent whose next step has not changed should
// not have to restate it to record that it got further.
//
// The service still requires the note to end up whole, so the first write on a
// task with nothing stored must supply both. That refusal comes from the service
// and reads like Cairn ("state.next_step is empty; say what whoever picks this
// up should do first") rather than from the schema validator.
type writeStateIn struct {
	Ref           string `json:"ref" jsonschema:"the task to leave a note on, like cairn-12"`
	WhereILeftOff string `json:"where_i_left_off,omitempty" jsonschema:"what has actually been done, in enough detail that someone who was not here can carry on. Leave it out to keep the note already on the task"`
	NextStep      string `json:"next_step,omitempty" jsonschema:"the single next thing to do. Leave it out to keep the one already on the task"`
	BlockedOn     string `json:"blocked_on,omitempty" jsonschema:"why the task is blocked. Only for a task already in blocked, where it is required: if you have just hit a blocker, move the task to blocked with transition_task instead, because a blocker recorded here on a task that is still active puts nothing on the board"`
}

type appendWorklogIn struct {
	Ref          string `json:"ref" jsonschema:"the task this attempt belongs to, like cairn-12"`
	WhatWasTried string `json:"what_was_tried" jsonschema:"what you actually did this stretch, in enough detail that someone about to repeat it would recognise it. The approach, not the intention"`
	Outcome      string `json:"outcome,omitempty" jsonschema:"what happened, including failures worth not repeating"`
}

// --- output -----------------------------------------------------------------

type taskOut struct {
	Ref     string `json:"ref"`
	Project string `json:"project"`
	Title   string `json:"title"`
	Status  string `json:"status"`

	NextStep string `json:"next_step,omitempty"`
	// BlockedOn rides alongside next_step rather than replacing it: on a blocked
	// task next_step is usually empty and this holds the only sentence that
	// matters, and an agent scanning a listing should not have to spend a
	// get_task to find out what the blocker is.
	BlockedOn string `json:"blocked_on,omitempty"`

	UpdatedBy string `json:"updated_by,omitempty"`
	UpdatedAt string `json:"updated_at"`

	// CanMoveTo is what the connect instructions promise of every task read,
	// and a listing is a task read. Without it an agent scanning for work it may
	// claim either guesses or spends one get_task per row.
	CanMoveTo []string `json:"can_move_to"`
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

// writeEchoEntries is how much history a write answers with. One, because the
// caller just wrote it: for append_worklog and transition_task that entry is
// the one they made, which is a useful confirmation, and for write_state it is
// a line of context. Everything else in the echo -- the status, can_move_to --
// is what the write actually saves a round trip on.
const writeEchoEntries = 1

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
		Description: "Every task in one project, most recently touched first, each with the " +
			"note left on it -- the same rows board returns, narrowed to one project. Finished " +
			"work is left out unless you ask for it by status.",
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
	}, s.createTask, summariseWrite)

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
	}, s.transitionTask, summariseWrite)

	bind(s, srv, &mcp.Tool{
		Name:        "write_state",
		Annotations: mutating(true),
		Description: "Overwrite the note on a task without moving it. Use this to checkpoint " +
			"during a long piece of work, so that if you stop unexpectedly the task still " +
			"says where things stand. It replaces the previous note rather than adding to it.",
	}, s.writeState, summariseWrite)

	bind(s, srv, &mcp.Tool{
		Name:        "append_worklog",
		Annotations: mutating(false),
		Description: "Record one attempt against a task, without moving it. The worklog is " +
			"append-only and permanent. Record failures as readily as successes: an approach " +
			"that did not work is the most useful thing you can leave for whoever tries next.",
	}, s.appendWorklog, summariseWrite)

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

// summariseDetail describes a task the caller asked to read. A history shorter
// than the total is news there, because the caller wanted the history.
func summariseDetail(o taskDetailOut) string {
	return summarise(o, true)
}

// summariseWrite describes a task the caller just wrote to. The history is
// deliberately short here and saying so on every checkpoint would report a
// deliberate saving as a shortfall, so this one reports the count and stops.
func summariseWrite(o taskDetailOut) string {
	return summarise(o, false)
}

func summarise(o taskDetailOut, sayTrimmed bool) string {
	line := fmt.Sprintf("%s is %s: %s", o.Ref, o.Status, o.Title)
	if o.State != nil {
		if o.State.BlockedOn != "" {
			line += fmt.Sprintf(". Blocked on: %s", o.State.BlockedOn)
		} else if o.State.NextStep != "" {
			line += fmt.Sprintf(". Next step: %s", o.State.NextStep)
		}
	}
	if o.WorklogTotal > 0 {
		switch {
		case sayTrimmed && len(o.Worklog) < o.WorklogTotal:
			line += fmt.Sprintf(". Showing the last %d of %d worklog entries -- the abandoned "+
				"approaches are usually among the older ones, so pass worklog_limit to read them.",
				len(o.Worklog), o.WorklogTotal)
		default:
			line += fmt.Sprintf(". %d worklog entries.", o.WorklogTotal)
		}
	}
	if len(o.CanMoveTo) > 0 {
		line += fmt.Sprintf(" You can move it to: %s.", strings.Join(o.CanMoveTo, ", "))
	} else {
		line += " " + nothingToMove(o.Status)
	}
	return line
}

// nothingToMove says who the task belongs to now, rather than only that it does
// not belong to the caller.
//
// Three statuses leave an agent no move and the right answer differs in each,
// but one sentence used to cover all three. An agent that reads "there is
// nothing you can move it to" has been told it is stuck, which is
// indistinguishable from having done everything right and handed the work over.
// The expensive misreading is the third option it has: filing a near-duplicate
// task, because filing is the one write it knows it is allowed.
func nothingToMove(status string) string {
	switch status {
	case string(workflow.Backlog):
		return "Nothing here is yours to move: only the human moves a task out of backlog, by queueing it."
	case string(workflow.Review):
		return "Nothing here is yours to move: it is with the human now, waiting to be marked done."
	case string(workflow.Done):
		return "This one is finished."
	default:
		return "Nothing here is yours to move."
	}
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
	return s.listing(ctx, actor, service.BoardQuery{Statuses: statuses, Limit: limitOr(in.Limit)})
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
	return s.listing(ctx, actor, service.BoardQuery{
		ProjectID: project.ID, Statuses: statuses, Limit: limitOr(in.Limit),
	})
}

// listing is the one shape board and list_tasks share.
//
// It asks the store for one row more than the caller wanted and hands back at
// most what they asked for: whether that extra row came back is the only honest
// way to know something was left behind. Comparing the returned count against
// the limit instead says
// "truncated" whenever a listing happens to land exactly on it, which is most
// often for the small explicit limits the flag exists to serve: an agent
// sampling with limit 10 against any board that is not nearly empty is told to
// raise the limit, does, and gets back the same rows.
func (s *server) listing(ctx context.Context, actor service.Actor, q service.BoardQuery) (tasksOut, error) {
	limit := q.Limit
	if limit > 0 {
		q.Limit = limit + 1
	}
	rows, err := s.svc.Board(ctx, actor, q)
	if err != nil {
		return tasksOut{}, err
	}
	truncated := limit > 0 && len(rows) > limit
	if truncated {
		rows = rows[:limit]
	}
	out := tasksOut{Tasks: make([]taskOut, 0, len(rows)), Truncated: truncated}
	for _, row := range rows {
		out.Tasks = append(out.Tasks, toTaskOut(row))
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
//
// It carries one worklog entry rather than get_task's ten. The echo is worth
// keeping for can_move_to and the post-move status, which genuinely save a round
// trip; the history is not, because the caller has just written it. write_state
// exists to be a cheap mid-run checkpoint, and an agent checkpointing five times
// through a long task was being sent five task reads it never asked for.
// worklog_total still reports the real count, so nothing is hidden.
func (s *server) detailOf(ctx context.Context, actor service.Actor, taskID string) (taskDetailOut, error) {
	detail, err := s.svc.GetTask(ctx, actor, taskID, worklogQuery(writeEchoEntries))
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

func toTaskOut(row model.BoardRow) taskOut {
	t := row.Task
	out := taskOut{
		Ref: t.Ref(), Project: t.ProjectSlug, Title: t.Title,
		Status: string(t.Status), UpdatedAt: clock.Format(t.UpdatedAt),
		CanMoveTo: statusStrings(row.CanMoveTo),
	}
	if row.State != nil {
		out.NextStep, out.BlockedOn = row.State.NextStep, row.State.BlockedOn
		out.UpdatedBy = row.State.UpdatedByName
	}
	return out
}

// statusStrings keeps can_move_to the same shape everywhere it appears: an
// array, empty rather than null when there are no moves.
func statusStrings(in []workflow.Status) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		out = append(out, string(s))
	}
	return out
}

func toDetailOut(d service.TaskDetail) taskDetailOut {
	out := taskDetailOut{
		Ref: d.Task.Ref(), Project: d.Task.ProjectSlug, Title: d.Task.Title,
		Body: d.Task.Body, Status: string(d.Task.Status),
		Worklog:      make([]worklogOut, 0, len(d.Worklog)),
		WorklogTotal: d.WorklogTotal,
		CanMoveTo:    statusStrings(d.CanMoveTo),
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
	return out
}
