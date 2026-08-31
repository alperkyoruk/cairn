package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/alperkyoruk/cairn/internal/service"
	"github.com/alperkyoruk/cairn/internal/workflow"
)

// bearerTransport puts the agent's token on every request, the way a real MCP
// client configured with a Cairn token does.
type bearerTransport struct {
	token string
	base  http.RoundTripper
}

func (t bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	clone := r.Clone(r.Context())
	if t.token != "" {
		clone.Header.Set("Authorization", "Bearer "+t.token)
	}
	return t.base.RoundTrip(clone)
}

type harness struct {
	t       *testing.T
	svc     *service.Service
	url     string
	human   service.Actor
	project string
	ctx     context.Context
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	ctx := context.Background()

	svc, err := service.Open(ctx, filepath.Join(t.TempDir(), "cairn.db"))
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
	project, err := svc.CreateProject(ctx, human, "cairn", "Cairn", "")
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(New(svc, "test"))
	t.Cleanup(srv.Close)

	return &harness{t: t, svc: svc, url: srv.URL, human: human, project: project.ID, ctx: ctx}
}

// connect returns an MCP session authenticated as a freshly created agent.
func (h *harness) connect(name string) *mcp.ClientSession {
	h.t.Helper()
	_, token, err := h.svc.CreateAgent(h.ctx, h.human, name)
	if err != nil {
		h.t.Fatal(err)
	}
	return h.connectWith(token)
}

func (h *harness) connectWith(token string) *mcp.ClientSession {
	h.t.Helper()
	client := mcp.NewClient(&mcp.Implementation{Name: "test-agent", Version: "0"}, nil)
	session, err := client.Connect(h.ctx, &mcp.StreamableClientTransport{
		Endpoint:             h.url,
		HTTPClient:           &http.Client{Transport: bearerTransport{token: token, base: http.DefaultTransport}},
		DisableStandaloneSSE: true,
	}, nil)
	if err != nil {
		h.t.Fatalf("connecting: %v", err)
	}
	h.t.Cleanup(func() { session.Close() })
	return session
}

// call runs a tool and decodes its structured result. It fails the test if the
// tool reported an error.
func (h *harness) call(session *mcp.ClientSession, name string, args map[string]any, out any) {
	h.t.Helper()
	result := h.callRaw(session, name, args)
	if result.IsError {
		h.t.Fatalf("%s failed: %s", name, text(result))
	}
	if out == nil {
		return
	}
	// Zero the target first. Several output fields are omitempty, and decoding
	// into a reused variable would otherwise leave a stale value standing where
	// the server deliberately sent nothing -- which reads as a product bug.
	v := reflect.ValueOf(out).Elem()
	v.Set(reflect.Zero(v.Type()))

	raw, err := json.Marshal(result.StructuredContent)
	if err != nil {
		h.t.Fatal(err)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		h.t.Fatalf("decoding %s result: %v", name, err)
	}
}

func (h *harness) callRaw(session *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
	h.t.Helper()
	result, err := session.CallTool(h.ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		h.t.Fatalf("calling %s: %v", name, err)
	}
	return result
}

// refusal runs a tool expecting it to be refused, and returns the message the
// agent would read.
func (h *harness) refusal(session *mcp.ClientSession, name string, args map[string]any) string {
	h.t.Helper()
	result := h.callRaw(session, name, args)
	if !result.IsError {
		h.t.Fatalf("%s was expected to fail but succeeded", name)
	}
	return text(result)
}

func text(result *mcp.CallToolResult) string {
	var parts []string
	for _, c := range result.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			parts = append(parts, tc.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func (h *harness) seedTask(title string, status workflow.Status) {
	h.t.Helper()
	h.seedTaskReturning(title, status)
}

func (h *harness) seedTaskReturning(title string, status workflow.Status) string {
	h.t.Helper()
	task, err := h.svc.CreateTask(h.ctx, h.human, service.CreateTaskInput{
		ProjectID: h.project, Title: title, Status: status,
	})
	if err != nil {
		h.t.Fatal(err)
	}
	return task.ID
}

// --- tests ------------------------------------------------------------------

func TestServerAnnouncesItselfAndItsTools(t *testing.T) {
	h := newHarness(t)
	session := h.connect("claude")

	tools, err := session.ListTools(h.ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]*mcp.Tool{}
	for _, tool := range tools.Tools {
		found[tool.Name] = tool
	}
	for _, want := range []string{
		"list_projects", "board", "list_tasks", "get_task",
		"create_task", "transition_task", "write_state", "append_worklog",
	} {
		if found[want] == nil {
			t.Errorf("tool %q is not advertised", want)
		}
	}

	// The instructions are how an agent learns the two rules that are not
	// guessable from tool names.
	instructions := session.InitializeResult().Instructions
	for _, phrase := range []string{"cannot move a task without writing state", "review -> done"} {
		if !strings.Contains(instructions, phrase) {
			t.Errorf("instructions do not mention %q", phrase)
		}
	}

	// The instructions described both records and then described only moves, so
	// the habit that makes this work when an agent does not come back -- writing
	// as you go, because the note is all that survives a crash -- was asked for
	// only in a tool description, which is read once the tool is already under
	// consideration. These three say it where every agent reads it on connect.
	for phrase, why := range map[string]string{
		"append_worklog":         "the worklog tool is never named",
		"write state\nas you go": "checkpointing during long work is never asked for",
		"pick up is in queue":    "where work may be claimed from is left to be inferred",
		"cannot take it over":    "finding a task another agent abandoned has no answer",
	} {
		if !strings.Contains(instructions, phrase) {
			t.Errorf("%s (looking for %q)", why, phrase)
		}
	}
}

// The project's central rule, expressed in the tool contract itself: an agent
// is told the fields are mandatory before it ever gets refused for omitting
// them.
func TestTransitionSchemaDemandsAState(t *testing.T) {
	h := newHarness(t)
	session := h.connect("claude")

	tools, err := session.ListTools(h.ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Required   []string `json:"required"`
		Properties map[string]struct {
			Description string `json:"description"`
		} `json:"properties"`
	}
	for _, tool := range tools.Tools {
		if tool.Name != "transition_task" {
			continue
		}
		raw, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, &schema); err != nil {
			t.Fatal(err)
		}
	}
	required := strings.Join(schema.Required, ",")

	// where_i_left_off is required at the one moment its own name does not fit:
	// claiming queued work, when nothing has been done yet. The rule stays; the
	// annotation has to say what the honest value is, or every agent's first
	// write on every task is a sentence it had to invent.
	if d := schema.Properties["where_i_left_off"].Description; !strings.Contains(d, "claiming queued work") {
		t.Errorf("where_i_left_off does not say what to write when claiming: %q", d)
	}
	for _, field := range []string{"ref", "to", "where_i_left_off", "next_step"} {
		if !strings.Contains(required, field) {
			t.Errorf("%q is not required by the transition_task schema (required: %v)",
				field, schema.Required)
		}
	}
	// And the optional ones stay optional, or every move would demand a worklog.
	for _, field := range []string{"blocked_on", "what_was_tried", "outcome"} {
		if strings.Contains(required, field) {
			t.Errorf("%q should be optional on transition_task", field)
		}
	}
}

// Three of these annotations describe the same database column as another one
// elsewhere in the file, and in every case the weaker of the pair sat on the
// tool an agent reaches for more often. The asymmetry is the bug: an agent gets
// a worse instruction for using the more specific tool.
func TestTheSameColumnIsDescribedAsWellOnEveryTool(t *testing.T) {
	h := newHarness(t)
	session := h.connect("claude")

	tools, err := session.ListTools(h.ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	describe := map[string]map[string]string{}
	for _, tool := range tools.Tools {
		raw, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatal(err)
		}
		var schema struct {
			Properties map[string]struct {
				Description string `json:"description"`
			} `json:"properties"`
		}
		if err := json.Unmarshal(raw, &schema); err != nil {
			t.Fatal(err)
		}
		describe[tool.Name] = map[string]string{}
		for field, prop := range schema.Properties {
			describe[tool.Name][field] = prop.Description
		}
	}

	// The dedicated worklog tool must not describe the worklog worse than the
	// transition that happens to carry one. Its whole value is that one string.
	if d := describe["append_worklog"]["what_was_tried"]; d == "what you attempted" || len(d) < 60 {
		t.Errorf("append_worklog.what_was_tried restates its own field name: %q", d)
	}
	if !strings.Contains(describe["append_worklog"]["outcome"], "failures worth not repeating") {
		t.Errorf("the dedicated worklog tool does not say dead ends are wanted: %q",
			describe["append_worklog"]["outcome"])
	}

	// A field that is only sometimes required has to lead with when, or an agent
	// that skims first clauses reads it as optional and is refused every time.
	if !strings.HasPrefix(describe["transition_task"]["what_was_tried"], "required when") {
		t.Errorf("what_was_tried buries its obligation: %q",
			describe["transition_task"]["what_was_tried"])
	}

	// Naming the statuses on the narrower tool too: README points an agent at
	// list_tasks first, and a guessed word costs a turn.
	for _, tool := range []string{"board", "list_tasks"} {
		for _, status := range []string{"backlog", "queue", "active", "review", "blocked", "done"} {
			if !strings.Contains(describe[tool]["status"], status) {
				t.Errorf("%s.status does not name %q: %q", tool, status, describe[tool]["status"])
			}
		}
	}
}

// An agent that has done everything right and handed work over reads the same
// sentence as an agent that is genuinely stuck. Three statuses leave an agent no
// move -- backlog, review, done -- and the right answer differs in each, so one
// sentence covering all three tells it it is stuck and never what to do. The
// expensive misreading is the third thing it can do: file a near-duplicate task,
// because filing is the one write it knows it is allowed.
func TestAnAgentWithNoMoveIsToldWhoseTaskItIs(t *testing.T) {
	h := newHarness(t)
	h.seedTask("still in backlog", workflow.Backlog)
	worked := h.seedTaskReturning("handed over", workflow.Queue)
	session := h.connect("claude")

	backlog := text(h.callRaw(session, "get_task", map[string]any{"ref": "cairn-1"}))
	if !strings.Contains(backlog, "only the human moves a task out of backlog") {
		t.Errorf("backlog says nothing about queueing: %q", backlog)
	}

	h.call(session, "transition_task", map[string]any{
		"ref": "cairn-2", "to": "active",
		"where_i_left_off": "read it", "next_step": "do it",
	}, nil)
	handed := text(h.callRaw(session, "transition_task", map[string]any{
		"ref": "cairn-2", "to": "review",
		"where_i_left_off": "did it", "next_step": "check it",
		"what_was_tried": "did the thing",
	}))
	if !strings.Contains(handed, "waiting to be marked done") {
		t.Errorf("handing work over reads as being stuck: %q", handed)
	}

	if _, err := h.svc.Transition(h.ctx, h.human, worked, service.TransitionInput{To: workflow.Done}); err != nil {
		t.Fatal(err)
	}
	done := text(h.callRaw(session, "get_task", map[string]any{"ref": "cairn-2"}))
	if !strings.Contains(done, "finished") {
		t.Errorf("a finished task does not say so: %q", done)
	}

	// And the refusal itself names who the move belongs to, rather than only
	// telling the agent it does not have one.
	refused := h.refusal(session, "transition_task", map[string]any{
		"ref": "cairn-1", "to": "active",
		"where_i_left_off": "read it", "next_step": "do it",
	})
	if !strings.Contains(refused, "the human moves it on from here") {
		t.Errorf("refusal does not say whose move it is: %q", refused)
	}
}

// A checkpoint may say only what changed. transition_task keeps both fields
// required, because a move is the moment the obligation bites; write_state is
// the tool called mid-run, where restating an unchanged next step on every
// checkpoint is pure cost. The note still has to end up whole, and the refusal
// when it cannot comes from the service rather than the schema validator.
func TestWriteStateTakesAPartialNote(t *testing.T) {
	h := newHarness(t)
	h.seedTask("Migrate the old invoice importer", workflow.Queue)
	session := h.connect("claude")

	tools, err := session.ListTools(h.ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools.Tools {
		if tool.Name != "write_state" {
			continue
		}
		raw, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatal(err)
		}
		var schema struct {
			Required []string `json:"required"`
		}
		if err := json.Unmarshal(raw, &schema); err != nil {
			t.Fatal(err)
		}
		for _, field := range []string{"where_i_left_off", "next_step"} {
			if strings.Contains(strings.Join(schema.Required, ","), field) {
				t.Errorf("%q is required by write_state, so a partial checkpoint is impossible", field)
			}
		}
	}

	// Nothing stored yet: there is nothing to inherit, so the whole note is
	// still owed, and Cairn says so rather than the validator.
	message := h.refusal(session, "write_state", map[string]any{
		"ref": "cairn-1", "where_i_left_off": "read the fixture",
	})
	if !strings.Contains(message, "next_step is empty") {
		t.Errorf("first partial write got the wrong refusal: %q", message)
	}

	h.call(session, "transition_task", map[string]any{
		"ref": "cairn-1", "to": "active",
		"where_i_left_off": "read the fixture", "next_step": "make the mapping name-based",
	}, nil)

	// Now a checkpoint that only reports progress keeps the standing next step.
	var detail taskDetailOut
	h.call(session, "write_state", map[string]any{
		"ref": "cairn-1", "where_i_left_off": "mapping rewritten; 2019 passes",
	}, &detail)
	if detail.State.NextStep != "make the mapping name-based" {
		t.Errorf("the standing next step was blanked by a partial checkpoint: %q", detail.State.NextStep)
	}
	if detail.State.WhereILeftOff != "mapping rewritten; 2019 passes" {
		t.Errorf("the checkpoint did not land: %q", detail.State.WhereILeftOff)
	}
}

func TestAgentWorksATaskThroughMCP(t *testing.T) {
	h := newHarness(t)
	h.seedTask("Embed the frontend", workflow.Queue)
	session := h.connect("claude")

	// Orient: the board says what is in flight.
	var board tasksOut
	h.call(session, "board", nil, &board)
	if len(board.Tasks) != 1 || board.Tasks[0].Ref != "cairn-1" {
		t.Fatalf("board = %+v", board)
	}

	// Read before acting: the task says what moves are available.
	var detail taskDetailOut
	h.call(session, "get_task", map[string]any{"ref": "cairn-1"}, &detail)
	if got := strings.Join(detail.CanMoveTo, ","); got != "active" {
		t.Errorf("can_move_to = %q, want active", got)
	}

	// Claim it.
	h.call(session, "transition_task", map[string]any{
		"ref": "cairn-1", "to": "active",
		"where_i_left_off": "picked this up",
		"next_step":        "read the embed docs",
	}, &detail)
	if detail.Status != "active" || detail.State.NextStep != "read the embed docs" {
		t.Fatalf("after claiming: %+v", detail)
	}
	if detail.State.UpdatedBy != "claude" {
		t.Errorf("state credited to %q, want claude", detail.State.UpdatedBy)
	}

	// Checkpoint mid-work without moving.
	h.call(session, "write_state", map[string]any{
		"ref":              "cairn-1",
		"where_i_left_off": "embed.FS wired, dist path still wrong",
		"next_step":        "decide where vite writes its bundle",
	}, &detail)
	if detail.Status != "active" {
		t.Error("write_state moved the task")
	}

	// Get stuck, with a reason.
	h.call(session, "transition_task", map[string]any{
		"ref": "cairn-1", "to": "blocked",
		"where_i_left_off": "build order is the problem",
		"next_step":        "confirm the build order",
		"blocked_on":       "need to know if the frontend build runs before go build",
		"what_was_tried":   "pointed embed at dist/",
		"outcome":          "no such directory at build time",
	}, &detail)
	if detail.State.BlockedOn == "" {
		t.Error("blocked task carries no blocker")
	}

	// Unstick: returning to active clears the stale blocker on its own.
	h.call(session, "transition_task", map[string]any{
		"ref": "cairn-1", "to": "active",
		"where_i_left_off": "vite runs first", "next_step": "wire the makefile",
	}, &detail)
	if detail.State.BlockedOn != "" {
		t.Errorf("blocker survived the return to active: %q", detail.State.BlockedOn)
	}

	// Hand it back. Leaving active also records the attempt.
	h.call(session, "transition_task", map[string]any{
		"ref": "cairn-1", "to": "review",
		"where_i_left_off": "make build embeds dist/",
		"next_step":        "check the binary serves / with no dist on disk",
		"what_was_tried":   "built with make build and moved web/dist aside",
		"outcome":          "the binary still served the app, so the embed is real",
	}, &detail)
	if detail.Status != "review" {
		t.Fatalf("status = %s", detail.Status)
	}
	// From review, an agent has nowhere left to go. That is the design.
	if len(detail.CanMoveTo) != 0 {
		t.Errorf("agent can still move a task in review: %v", detail.CanMoveTo)
	}
	// A write echoes the entry it just made, not the whole history -- but it
	// still says how much history there is, so nothing is hidden by the saving.
	if len(detail.Worklog) != 1 {
		t.Errorf("write echoed %d worklog entries, want 1", len(detail.Worklog))
	}
	if detail.Worklog[0].WhatWasTried != "built with make build and moved web/dist aside" {
		t.Errorf("the echoed entry is not the one just written: %+v", detail.Worklog[0])
	}
	if detail.WorklogTotal != 5 {
		t.Errorf("worklog_total = %d, want 5", detail.WorklogTotal)
	}

	// And asking for the task properly still hands over the history.
	h.call(session, "get_task", map[string]any{"ref": "cairn-1"}, &detail)
	if len(detail.Worklog) != 5 || detail.WorklogTotal != 5 {
		t.Errorf("get_task returned %d of %d entries, want 5 of 5",
			len(detail.Worklog), detail.WorklogTotal)
	}
}

// The two withheld decisions. They are now refused by the schema rather than by
// the service: `to` enumerates only the statuses an agent may name, so a client
// reading the tool cannot form the call at all. That is a better place to spend
// the refusal than an error message -- it is read before acting rather than
// after failing -- but it does mean the teaching moved into the description,
// which is asserted here so it cannot quietly go missing.
func TestTheHumansDecisionsAreNotEvenExpressible(t *testing.T) {
	h := newHarness(t)
	h.seedTask("Embed the frontend", workflow.Queue)
	session := h.connect("claude")

	tools, err := session.ListTools(h.ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	var transition *mcp.Tool
	for _, tool := range tools.Tools {
		if tool.Name == "transition_task" {
			transition = tool
		}
	}
	if transition == nil {
		t.Fatal("no transition_task tool")
	}

	raw, err := json.Marshal(transition.InputSchema)
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Properties struct {
			To struct {
				Enum []string `json:"enum"`
			} `json:"to"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	got := strings.Join(schema.Properties.To.Enum, ",")
	if got != "active,review,blocked" {
		t.Errorf("to enum = %q, want active,review,blocked", got)
	}
	for _, forbidden := range []string{"done", "queue", "backlog"} {
		if strings.Contains(got, forbidden) {
			t.Errorf("the enum offers %q, which an agent can never reach", forbidden)
		}
	}

	// And the description still says why, since the error no longer can.
	for _, phrase := range []string{"review -> done", "the human's decisions", "move it to review"} {
		if !strings.Contains(transition.Description, phrase) {
			t.Errorf("the description no longer explains the refusal: missing %q", phrase)
		}
	}

	// A client ignoring the schema is still refused, just tersely.
	h.call(session, "transition_task", map[string]any{
		"ref": "cairn-1", "to": "active",
		"where_i_left_off": "started", "next_step": "continue",
	}, nil)
	h.call(session, "transition_task", map[string]any{
		"ref": "cairn-1", "to": "review",
		"where_i_left_off": "finished", "next_step": "please check it",
		"what_was_tried": "did the work", "outcome": "it builds",
	}, nil)
	message := h.refusal(session, "transition_task", map[string]any{
		"ref": "cairn-1", "to": "done",
		"where_i_left_off": "all done", "next_step": "nothing",
	})
	if !strings.Contains(message, "done") {
		t.Errorf("refusal does not name the offending value: %q", message)
	}

	// Filing is still allowed; queueing what you filed is not, and that one is
	// still refused by the service, with its own words.
	var filed taskDetailOut
	h.call(session, "create_task", map[string]any{
		"project": "cairn", "title": "the vite config needs a comment",
	}, &filed)
	if filed.Status != "backlog" {
		t.Errorf("agent filed straight into %s", filed.Status)
	}
}

func TestStateIsMandatoryEvenWhenTheSchemaIsIgnored(t *testing.T) {
	h := newHarness(t)
	h.seedTask("Embed the frontend", workflow.Queue)
	session := h.connect("claude")

	// A client that sends blanks past the schema still cannot get through.
	message := h.refusal(session, "transition_task", map[string]any{
		"ref": "cairn-1", "to": "active",
		"where_i_left_off": "   ", "next_step": "",
	})
	if !strings.Contains(message, "where_i_left_off") {
		t.Errorf("refusal does not name the missing field: %q", message)
	}

	// And blocking still demands a blocker.
	h.call(session, "transition_task", map[string]any{
		"ref": "cairn-1", "to": "active",
		"where_i_left_off": "started", "next_step": "continue",
	}, nil)
	message = h.refusal(session, "transition_task", map[string]any{
		"ref": "cairn-1", "to": "blocked",
		"where_i_left_off": "stuck", "next_step": "unstick",
		"what_was_tried": "looked everywhere",
	})
	if !strings.Contains(message, "blocked_on") {
		t.Errorf("refusal does not name the missing field: %q", message)
	}
}

func TestUnknownProjectSaysWhichOnesExist(t *testing.T) {
	h := newHarness(t)
	session := h.connect("claude")

	message := h.refusal(session, "create_task", map[string]any{
		"project": "cair", "title": "typo in the slug",
	})
	if !strings.Contains(message, "cairn") {
		t.Errorf("refusal does not list the real projects: %q", message)
	}
}

func TestEndpointRefusesUnauthenticatedClients(t *testing.T) {
	h := newHarness(t)

	for _, tc := range []struct{ name, token string }{
		{"no token", ""},
		{"nonsense token", "cairn_not-a-real-token"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &http.Client{Transport: bearerTransport{token: tc.token, base: http.DefaultTransport}}
			resp, err := client.Post(h.url, "application/json", strings.NewReader(`{}`))
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401", resp.StatusCode)
			}
			if resp.Header.Get("WWW-Authenticate") == "" {
				t.Error("401 does not say how to authenticate")
			}
		})
	}
}

// Revoking a token in the web interface must stop the agent, not eventually.
func TestRevokingATokenStopsTheAgent(t *testing.T) {
	h := newHarness(t)
	agent, token, err := h.svc.CreateAgent(h.ctx, h.human, "claude")
	if err != nil {
		t.Fatal(err)
	}
	session := h.connectWith(token)
	h.call(session, "board", nil, nil)

	tokens, err := h.svc.ListTokens(h.ctx, h.human, agent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.svc.RevokeToken(h.ctx, h.human, tokens[0].ID); err != nil {
		t.Fatal(err)
	}

	result, err := session.CallTool(h.ctx, &mcp.CallToolParams{Name: "board"})
	if err == nil && !result.IsError {
		t.Fatal("a revoked agent could still read the board")
	}
}

// Clients decide whether to ask the human before calling a tool by reading
// these. Without them, reading the board prompts for approval like a deletion.
func TestToolsDeclareWhatTheyDo(t *testing.T) {
	h := newHarness(t)
	session := h.connect("claude")

	tools, err := session.ListTools(h.ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]*mcp.Tool{}
	for _, tool := range tools.Tools {
		byName[tool.Name] = tool
	}

	reads := []string{"board", "list_projects", "list_tasks", "get_task"}
	for _, name := range reads {
		a := byName[name].Annotations
		if a == nil {
			t.Errorf("%s has no annotations", name)
			continue
		}
		if !a.ReadOnlyHint {
			t.Errorf("%s is a read but does not say so", name)
		}
		if !a.IdempotentHint {
			t.Errorf("%s is a read but is not marked idempotent", name)
		}
	}

	writes := map[string]bool{ // name -> idempotent
		"write_state": true, "append_worklog": false,
		"create_task": false, "transition_task": false,
	}
	for name, idempotent := range writes {
		a := byName[name].Annotations
		if a == nil {
			t.Errorf("%s has no annotations", name)
			continue
		}
		if a.ReadOnlyHint {
			t.Errorf("%s writes but claims to be read-only", name)
		}
		if a.IdempotentHint != idempotent {
			t.Errorf("%s idempotent = %v, want %v", name, a.IdempotentHint, idempotent)
		}
		// Nothing an agent can reach destroys anything: state is overwritten,
		// the worklog only grows, and deletion is not exposed over MCP.
		if a.DestructiveHint == nil || *a.DestructiveHint {
			t.Errorf("%s is marked destructive; no agent-reachable tool removes anything", name)
		}
	}
}

// The SDK fills the text block with the serialised output when a handler leaves
// it empty, so every response used to cross the wire twice. The summary is both
// smaller and more useful than a second copy of the JSON.
func TestResponsesAreNotSentTwice(t *testing.T) {
	h := newHarness(t)
	h.seedTask("Migrate the old invoice importer", workflow.Queue)
	session := h.connect("claude")

	h.call(session, "transition_task", map[string]any{
		"ref": "cairn-1", "to": "active",
		"where_i_left_off": "read the importer", "next_step": "write a fixture",
	}, nil)
	for i := 0; i < 6; i++ {
		h.call(session, "append_worklog", map[string]any{
			"ref": "cairn-1", "what_was_tried": "a fairly long attempt description number " + string(rune('1'+i)),
			"outcome": "an equally long outcome, of the sort that makes a payload worth measuring",
		}, nil)
	}

	result := h.callRaw(session, "get_task", map[string]any{"ref": "cairn-1"})
	structured, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	summary := text(result)

	if summary == "" {
		t.Fatal("no text content at all; a client that only reads text gets nothing")
	}
	if json.Valid([]byte(summary)) && strings.HasPrefix(strings.TrimSpace(summary), "{") {
		t.Errorf("the text block is still a copy of the JSON: %.80s", summary)
	}
	if len(summary) > len(structured)/4 {
		t.Errorf("summary is %d chars against %d of structured data; that is not a summary",
			len(summary), len(structured))
	}
	// It has to actually say something.
	for _, want := range []string{"cairn-1", "active"} {
		if !strings.Contains(summary, want) {
			t.Errorf("summary %q does not mention %q", summary, want)
		}
	}
}

// The project-scoped tool used to answer with bare tasks while the board
// answered with the note, so an agent set up one-repo-one-project was better off
// calling board and discarding the other projects. Both go through one query
// now; this locks the note onto the narrower tool.
func TestListTasksCarriesTheSameNoteAsTheBoard(t *testing.T) {
	h := newHarness(t)
	id := h.seedTaskReturning("port the column mapping", workflow.Queue)
	if _, err := h.svc.Transition(h.ctx, h.human, id, service.TransitionInput{
		To: workflow.Active,
		State: &service.StateInput{
			WhereILeftOff: "read the failing fixture",
			NextStep:      "make the mapping name-based",
		},
	}); err != nil {
		t.Fatal(err)
	}

	session := h.connect("claude")

	var scoped, board tasksOut
	h.call(session, "list_tasks", map[string]any{"project": "cairn"}, &scoped)
	h.call(session, "board", map[string]any{}, &board)

	if len(scoped.Tasks) != 1 {
		t.Fatalf("list_tasks returned %d tasks, want 1", len(scoped.Tasks))
	}
	if scoped.Tasks[0].NextStep != "make the mapping name-based" {
		t.Errorf("list_tasks next_step = %q, want the note the board carries", scoped.Tasks[0].NextStep)
	}
	if scoped.Tasks[0].UpdatedBy == "" {
		t.Error("list_tasks dropped updated_by")
	}
	if len(board.Tasks) != 1 || !reflect.DeepEqual(board.Tasks[0], scoped.Tasks[0]) {
		t.Errorf("same task differs by tool:\n board      %+v\n list_tasks %+v",
			board.Tasks, scoped.Tasks)
	}
}

// The instructions tell every connecting client that "every task read includes
// can_move_to". A listing is a task read, so the promise has to hold there too --
// otherwise an agent scanning for work it may claim guesses, or spends one
// get_task per row. blocked_on is the same argument: on a blocked row it is the
// only sentence that matters, and it used to cost a whole extra call to see.
func TestListingsCarryBlockedOnAndTheMovesAvailable(t *testing.T) {
	h := newHarness(t)
	h.seedTask("wire up the importer", workflow.Queue)
	h.seedTask("not started", workflow.Queue)

	session := h.connect("claude")

	h.call(session, "transition_task", map[string]any{
		"ref": "cairn-1", "to": "active",
		"where_i_left_off": "read the fixture", "next_step": "call the endpoint",
	}, nil)
	h.call(session, "transition_task", map[string]any{
		"ref": "cairn-1", "to": "blocked",
		"where_i_left_off": "called the endpoint",
		"next_step":        "retry once there are credentials",
		"blocked_on":       "no API credentials for the staging importer",
		"what_was_tried":   "called it unauthenticated",
	}, nil)

	var board tasksOut
	h.call(session, "board", map[string]any{}, &board)

	byRef := map[string]taskOut{}
	for _, task := range board.Tasks {
		byRef[task.Ref] = task
	}

	blocked, ok := byRef["cairn-1"]
	if !ok {
		t.Fatalf("blocked task missing from board: %+v", board.Tasks)
	}
	if blocked.BlockedOn != "no API credentials for the staging importer" {
		t.Errorf("board dropped blocked_on: %+v", blocked)
	}
	if !reflect.DeepEqual(blocked.CanMoveTo, []string{"active"}) {
		t.Errorf("blocked row can_move_to = %v, want [active]", blocked.CanMoveTo)
	}
	if queued := byRef["cairn-2"]; !reflect.DeepEqual(queued.CanMoveTo, []string{"active"}) {
		t.Errorf("queued row can_move_to = %v, want [active]", queued.CanMoveTo)
	}

	// A row and the task detail behind it must never disagree about what is
	// permitted, which is why the service fills both.
	var detail taskDetailOut
	h.call(session, "get_task", map[string]any{"ref": "cairn-1"}, &detail)
	if !reflect.DeepEqual(detail.CanMoveTo, blocked.CanMoveTo) {
		t.Errorf("get_task says %v, board says %v", detail.CanMoveTo, blocked.CanMoveTo)
	}
	if detail.State.BlockedOn != blocked.BlockedOn {
		t.Errorf("get_task blocked_on %q, board %q", detail.State.BlockedOn, blocked.BlockedOn)
	}
}

// can_move_to is per-actor, so it has to be computed from the caller rather than
// from the status alone. A task in review offers the human "done" and the agent
// nothing at all.
func TestCanMoveToOnAListingIsTheCallersOwn(t *testing.T) {
	h := newHarness(t)
	id := h.seedTaskReturning("port the mapping", workflow.Queue)
	for _, to := range []workflow.Status{workflow.Active, workflow.Review} {
		if _, err := h.svc.Transition(h.ctx, h.human, id, service.TransitionInput{
			To:    to,
			State: &service.StateInput{WhereILeftOff: "did the thing", NextStep: "check it"},
		}); err != nil {
			t.Fatal(err)
		}
	}

	rows, err := h.svc.Board(h.ctx, h.human, service.BoardQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || !reflect.DeepEqual(rows[0].CanMoveTo, []workflow.Status{workflow.Done, workflow.Active}) {
		t.Errorf("human sees %v on a task in review, want [done active]", rows[0].CanMoveTo)
	}

	var board tasksOut
	h.call(h.connect("claude"), "board", map[string]any{}, &board)
	if len(board.Tasks) != 1 {
		t.Fatalf("board returned %d tasks, want 1", len(board.Tasks))
	}
	if len(board.Tasks[0].CanMoveTo) != 0 {
		t.Errorf("agent sees %v on a task in review, want nothing", board.Tasks[0].CanMoveTo)
	}
}

// Narrowing to a project must not swallow the status filter, nor the reverse:
// the project condition is a WHERE and the status condition then has to be an
// AND. Get it wrong and the query widens instead of narrowing.
func TestListTasksNarrowsByProjectAndStatusTogether(t *testing.T) {
	h := newHarness(t)
	h.seedTask("queued here", workflow.Queue)
	h.seedTask("also queued here", workflow.Queue)
	h.seedTask("backlogged here", workflow.Backlog)

	other, err := h.svc.CreateProject(h.ctx, h.human, "other", "Other", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.svc.CreateTask(h.ctx, h.human, service.CreateTaskInput{
		ProjectID: other.ID, Title: "queued elsewhere", Status: workflow.Queue,
	}); err != nil {
		t.Fatal(err)
	}

	session := h.connect("claude")

	var out tasksOut
	h.call(session, "list_tasks", map[string]any{
		"project": "cairn", "status": []string{"queue"},
	}, &out)
	if len(out.Tasks) != 2 {
		t.Fatalf("project+status returned %d tasks, want 2: %+v", len(out.Tasks), out.Tasks)
	}
	for _, task := range out.Tasks {
		if task.Project != "cairn" || task.Status != "queue" {
			t.Errorf("filter let %s/%s through", task.Project, task.Status)
		}
	}
}

// A board that returns everything, done included, is a board an agent stops
// reading. And a truncated one has to admit it.
func TestBoardDefaultsToOpenWorkAndAdmitsTruncation(t *testing.T) {
	h := newHarness(t)
	h.seedTask("still open", workflow.Queue)
	h.seedTask("also open", workflow.Queue)
	done := h.seedTaskReturning("finished", workflow.Queue)

	// Walk one task to done through the human.
	if _, err := h.svc.Transition(h.ctx, h.human, done, service.TransitionInput{
		To: workflow.Active, State: &service.StateInput{WhereILeftOff: "x", NextStep: "y"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := h.svc.Transition(h.ctx, h.human, done, service.TransitionInput{
		To: workflow.Review, State: &service.StateInput{WhereILeftOff: "x", NextStep: "y"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := h.svc.Transition(h.ctx, h.human, done, service.TransitionInput{To: workflow.Done}); err != nil {
		t.Fatal(err)
	}

	session := h.connect("claude")

	var board tasksOut
	h.call(session, "board", map[string]any{}, &board)
	if len(board.Tasks) != 2 {
		t.Errorf("default board returned %d tasks, want 2 (done excluded)", len(board.Tasks))
	}
	for _, task := range board.Tasks {
		if task.Status == "done" {
			t.Error("done work appeared in the default board")
		}
	}

	h.call(session, "board", map[string]any{"status": []string{"done"}}, &board)
	if len(board.Tasks) != 1 || board.Tasks[0].Status != "done" {
		t.Errorf("asking for done returned %+v", board.Tasks)
	}

	h.call(session, "board", map[string]any{"limit": 1}, &board)
	if len(board.Tasks) != 1 || !board.Truncated {
		t.Errorf("limited board returned %d tasks, truncated=%v", len(board.Tasks), board.Truncated)
	}

	// The exact-fill case. Two open tasks and a limit of two is a complete
	// answer, and calling it truncated sends the agent back for a second full
	// payload identical to the first.
	h.call(session, "board", map[string]any{"limit": 2}, &board)
	if len(board.Tasks) != 2 || board.Truncated {
		t.Errorf("board of 2 under limit 2 returned %d tasks, truncated=%v; want 2 and false",
			len(board.Tasks), board.Truncated)
	}

	// And a limit with room to spare is not truncated either.
	h.call(session, "board", map[string]any{"limit": 50}, &board)
	if len(board.Tasks) != 2 || board.Truncated {
		t.Errorf("roomy board returned %d tasks, truncated=%v", len(board.Tasks), board.Truncated)
	}

	message := h.refusal(session, "board", map[string]any{"status": []string{"nonsense"}})
	if !strings.Contains(message, "nonsense") || !strings.Contains(message, "backlog") {
		t.Errorf("bad status refusal does not list the real ones: %q", message)
	}
}

// A long history is the single biggest thing a task read can cost, and a
// truncated one that does not say so reads as complete.
func TestWorklogIsTrimmedButHonest(t *testing.T) {
	h := newHarness(t)
	h.seedTask("Migrate the old invoice importer", workflow.Queue)
	session := h.connect("claude")

	h.call(session, "transition_task", map[string]any{
		"ref": "cairn-1", "to": "active",
		"where_i_left_off": "read it", "next_step": "fixture",
	}, nil)
	for i := 0; i < 14; i++ {
		h.call(session, "append_worklog", map[string]any{
			"ref": "cairn-1", "what_was_tried": "attempt " + string(rune('a'+i)),
		}, nil)
	}

	var detail taskDetailOut
	h.call(session, "get_task", map[string]any{"ref": "cairn-1"}, &detail)
	if len(detail.Worklog) != 10 {
		t.Errorf("default worklog returned %d entries, want 10", len(detail.Worklog))
	}
	if detail.WorklogTotal != 16 { // filed + claimed + 14 appended
		t.Errorf("worklog_total = %d, want 16", detail.WorklogTotal)
	}
	// The newest, and still in reading order.
	if last := detail.Worklog[len(detail.Worklog)-1]; last.WhatWasTried != "attempt n" {
		t.Errorf("last entry is %q, want the newest", last.WhatWasTried)
	}

	h.call(session, "get_task", map[string]any{"ref": "cairn-1", "worklog_limit": 100}, &detail)
	if len(detail.Worklog) != 16 {
		t.Errorf("asking for more returned %d entries, want all 16", len(detail.Worklog))
	}

	// A read that was cut short says so, and says what to do about it. A write
	// is cut short deliberately, so saying the same thing on every checkpoint
	// would report a saving as a shortfall.
	read := text(h.callRaw(session, "get_task", map[string]any{"ref": "cairn-1"}))
	if !strings.Contains(read, "Showing the last 10 of 16") || !strings.Contains(read, "worklog_limit") {
		t.Errorf("a trimmed read does not say so and what to do: %q", read)
	}

	write := text(h.callRaw(session, "write_state", map[string]any{
		"ref": "cairn-1", "where_i_left_off": "still going", "next_step": "keep going",
	}))
	if strings.Contains(write, "Showing the last") {
		t.Errorf("a write reports its deliberate trim as truncation: %q", write)
	}
	// Still 16: write_state writes state, and only the worklog tools write the
	// worklog. The count is the real one either way.
	if !strings.Contains(write, "16 worklog entries") {
		t.Errorf("a write does not say how much history there is: %q", write)
	}
}
