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
	if _, err := h.svc.CreateTask(h.ctx, h.human, service.CreateTaskInput{
		ProjectID: h.project, Title: title, Status: status,
	}); err != nil {
		h.t.Fatal(err)
	}
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

	// Hand it back.
	h.call(session, "transition_task", map[string]any{
		"ref": "cairn-1", "to": "review",
		"where_i_left_off": "make build embeds dist/",
		"next_step":        "check the binary serves / with no dist on disk",
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

// The two withheld decisions, refused over MCP with messages an agent can act on.
func TestTheHumansDecisionsAreRefusedWithAnExplanation(t *testing.T) {
	h := newHarness(t)
	h.seedTask("Embed the frontend", workflow.Queue)
	session := h.connect("claude")

	h.call(session, "transition_task", map[string]any{
		"ref": "cairn-1", "to": "active",
		"where_i_left_off": "started", "next_step": "continue",
	}, nil)
	h.call(session, "transition_task", map[string]any{
		"ref": "cairn-1", "to": "review",
		"where_i_left_off": "finished", "next_step": "please check it",
	}, nil)

	message := h.refusal(session, "transition_task", map[string]any{
		"ref": "cairn-1", "to": "done",
		"where_i_left_off": "all done", "next_step": "nothing",
	})
	if !strings.Contains(message, "only the human marks work done") {
		t.Errorf("refusal does not explain itself: %q", message)
	}
	if !strings.Contains(message, "review") {
		t.Errorf("refusal does not say where to leave the task: %q", message)
	}

	// Filing is allowed; queueing what you filed is not.
	var filed taskDetailOut
	h.call(session, "create_task", map[string]any{
		"project": "cairn", "title": "the vite config needs a comment",
	}, &filed)
	if filed.Status != "backlog" {
		t.Errorf("agent filed straight into %s", filed.Status)
	}
	message = h.refusal(session, "transition_task", map[string]any{
		"ref": filed.Ref, "to": "queue",
		"where_i_left_off": "filed it", "next_step": "work on it",
	})
	if !strings.Contains(message, "only the human decides") {
		t.Errorf("refusal does not explain itself: %q", message)
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
