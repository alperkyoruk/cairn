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
		Required []string `json:"required"`
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
	if len(detail.Worklog) != 5 {
		t.Errorf("worklog has %d entries, want 5", len(detail.Worklog))
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
}
