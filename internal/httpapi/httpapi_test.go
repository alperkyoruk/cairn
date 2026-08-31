package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/alperkyoruk/cairn/internal/service"
)

type harness struct {
	t      *testing.T
	server *httptest.Server
	human  *http.Client // carries the session cookie
	svc    *service.Service
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	svc, err := service.Open(context.Background(), filepath.Join(t.TempDir(), "cairn.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { svc.Close() })

	srv := httptest.NewServer(New(svc, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		io.WriteString(w, "<!doctype html><title>cairn</title>")
	})))
	t.Cleanup(srv.Close)

	jar, _ := cookiejar.New(nil)
	return &harness{t: t, server: srv, human: &http.Client{Jar: jar}, svc: svc}
}

// do sends a request as the human (cookie jar) unless bearer is set.
func (h *harness) do(method, path string, body any, bearer string) *http.Response {
	h.t.Helper()
	var r io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			h.t.Fatal(err)
		}
		r = bytes.NewReader(buf)
	}
	req, err := http.NewRequest(method, h.server.URL+path, r)
	if err != nil {
		h.t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	client := h.human
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
		client = h.server.Client()
	}
	resp, err := client.Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	return resp
}

func (h *harness) decode(resp *http.Response, v any) {
	h.t.Helper()
	defer resp.Body.Close()
	if v == nil {
		return
	}
	// Zero the target first. Several fields are omitempty, so decoding into a
	// reused variable would leave a stale true standing where the server
	// deliberately sent nothing -- which reads as a product bug and is not one.
	rv := reflect.ValueOf(v).Elem()
	rv.Set(reflect.Zero(rv.Type()))

	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		h.t.Fatalf("decoding %s: %v", resp.Request.URL, err)
	}
}

func (h *harness) wantStatus(resp *http.Response, want int) {
	h.t.Helper()
	if resp.StatusCode != want {
		body, _ := io.ReadAll(resp.Body)
		h.t.Fatalf("%s %s = %d, want %d: %s",
			resp.Request.Method, resp.Request.URL.Path, resp.StatusCode, want, body)
	}
}

// setUp completes first-launch setup and returns a signed-in harness.
func (h *harness) setUp() {
	h.t.Helper()
	resp := h.do("POST", "/api/setup", map[string]string{
		"username": "alper", "password": "correct-horse",
	}, "")
	h.wantStatus(resp, http.StatusOK)
	resp.Body.Close()
}

func TestFirstLaunchFlow(t *testing.T) {
	h := newHarness(t)

	var session sessionDTO
	resp := h.do("GET", "/api/session", nil, "")
	h.wantStatus(resp, http.StatusOK)
	h.decode(resp, &session)
	if !session.NeedsSetup || session.Actor != nil {
		t.Fatalf("fresh install reports %+v", session)
	}

	// The board is not reachable before there is anyone to be.
	resp = h.do("GET", "/api/board", nil, "")
	h.wantStatus(resp, http.StatusUnauthorized)
	resp.Body.Close()

	// Setting up signs you in in the same request.
	resp = h.do("POST", "/api/setup", map[string]string{
		"username": "alper", "password": "correct-horse",
	}, "")
	h.wantStatus(resp, http.StatusOK)
	h.decode(resp, &session)
	if session.NeedsSetup || session.Actor == nil || session.Actor.Name != "alper" {
		t.Fatalf("setup did not sign the user in: %+v", session)
	}

	resp = h.do("GET", "/api/board", nil, "")
	h.wantStatus(resp, http.StatusOK)
	resp.Body.Close()

	// And it cannot happen twice.
	resp = h.do("POST", "/api/setup", map[string]string{
		"username": "someone", "password": "another-one",
	}, "")
	h.wantStatus(resp, http.StatusConflict)
	resp.Body.Close()

	resp = h.do("POST", "/api/logout", nil, "")
	h.wantStatus(resp, http.StatusNoContent)
	resp.Body.Close()
	resp = h.do("GET", "/api/board", nil, "")
	h.wantStatus(resp, http.StatusUnauthorized)
	resp.Body.Close()
}

// The claim from step 2, tested across the wire: an agent's bearer token and a
// human's cookie reach the same rules. No permission check was written here.
func TestOneModelForBothSurfaces(t *testing.T) {
	h := newHarness(t)
	h.setUp()

	var created newAgentDTO
	resp := h.do("POST", "/api/agents", map[string]string{"name": "claude"}, "")
	h.wantStatus(resp, http.StatusCreated)
	h.decode(resp, &created)
	if !strings.HasPrefix(created.Token, "cairn_") {
		t.Errorf("token %q does not look like a Cairn token", created.Token)
	}

	var project projectDTO
	resp = h.do("POST", "/api/projects", map[string]string{"slug": "cairn", "name": "Cairn"}, "")
	h.wantStatus(resp, http.StatusCreated)
	h.decode(resp, &project)

	// The agent reads with its token.
	resp = h.do("GET", "/api/board", nil, created.Token)
	h.wantStatus(resp, http.StatusOK)
	resp.Body.Close()

	// And is refused the human's decisions, with the same words the service uses.
	resp = h.do("POST", "/api/projects", map[string]string{"slug": "x", "name": "X"}, created.Token)
	h.wantStatus(resp, http.StatusForbidden)
	var failure errorBody
	h.decode(resp, &failure)
	if failure.Error.Kind != "forbidden" {
		t.Errorf("error kind = %q", failure.Error.Kind)
	}

	// Filing into the backlog, however, is the agent's to do.
	var task taskDTO
	resp = h.do("POST", "/api/tasks", map[string]any{
		"project_id": project.ID, "title": "the vite config needs a comment",
	}, created.Token)
	h.wantStatus(resp, http.StatusCreated)
	h.decode(resp, &task)
	if task.Status != "backlog" || task.Ref != "cairn-1" {
		t.Errorf("agent filed %+v", task)
	}
}

func TestTaskLifecycleOverHTTP(t *testing.T) {
	h := newHarness(t)
	h.setUp()

	var project projectDTO
	resp := h.do("POST", "/api/projects", map[string]string{"slug": "cairn", "name": "Cairn"}, "")
	h.decode(resp, &project)

	var created taskDTO
	resp = h.do("POST", "/api/tasks", map[string]any{
		"project_id": project.ID, "title": "Embed the frontend", "status": "queue",
	}, "")
	h.wantStatus(resp, http.StatusCreated)
	h.decode(resp, &created)

	// Reads answer with the moves this caller may make, so the interface does
	// not have to know the state machine.
	var detail taskDetailDTO
	resp = h.do("GET", "/api/tasks/cairn-1", nil, "")
	h.wantStatus(resp, http.StatusOK)
	h.decode(resp, &detail)
	if got := strings.Join(detail.CanMoveTo, ","); got != "active,backlog" {
		t.Errorf("can_move_to = %q, want active,backlog", got)
	}
	if detail.State != nil {
		t.Error("an untouched task should carry no state")
	}

	// A transition with its note.
	resp = h.do("POST", "/api/tasks/cairn-1/transition", map[string]any{
		"to": "active",
		"state": map[string]string{
			"where_i_left_off": "picked this up",
			"next_step":        "read the embed docs",
		},
	}, "")
	h.wantStatus(resp, http.StatusOK)
	h.decode(resp, &detail)
	if detail.Task.Status != "active" || detail.State.NextStep != "read the embed docs" {
		t.Fatalf("transition returned %+v", detail)
	}
	if detail.State.UpdatedBy != "alper" {
		t.Errorf("state credited to %q", detail.State.UpdatedBy)
	}

	// An illegal move is a 400, and says so in words worth reading.
	resp = h.do("POST", "/api/tasks/cairn-1/transition", map[string]any{"to": "done"}, "")
	h.wantStatus(resp, http.StatusBadRequest)
	var failure errorBody
	h.decode(resp, &failure)
	if !strings.Contains(failure.Error.Message, "review") {
		t.Errorf("refusal does not point anywhere: %q", failure.Error.Message)
	}

	// Blocking without a blocker is refused.
	resp = h.do("POST", "/api/tasks/cairn-1/transition", map[string]any{
		"to":    "blocked",
		"state": map[string]string{"where_i_left_off": "stuck", "next_step": "unstick"},
	}, "")
	h.wantStatus(resp, http.StatusBadRequest)
	resp.Body.Close()

	// Both id and reference address the same task.
	resp = h.do("GET", "/api/tasks/"+created.ID, nil, "")
	h.wantStatus(resp, http.StatusOK)
	h.decode(resp, &detail)
	if detail.Task.Ref != "cairn-1" {
		t.Errorf("lookup by id returned %s", detail.Task.Ref)
	}
	resp = h.do("GET", "/api/tasks/cairn-99", nil, "")
	h.wantStatus(resp, http.StatusNotFound)
	resp.Body.Close()
}

func TestBoardShape(t *testing.T) {
	h := newHarness(t)
	h.setUp()

	var project projectDTO
	resp := h.do("POST", "/api/projects", map[string]string{"slug": "cairn", "name": "Cairn"}, "")
	h.decode(resp, &project)
	for _, title := range []string{"first", "second"} {
		resp = h.do("POST", "/api/tasks", map[string]any{"project_id": project.ID, "title": title}, "")
		resp.Body.Close()
	}

	var rows []boardRowDTO
	resp = h.do("GET", "/api/board", nil, "")
	h.wantStatus(resp, http.StatusOK)
	h.decode(resp, &rows)
	if len(rows) != 2 {
		t.Fatalf("board has %d rows", len(rows))
	}
	if rows[0].Task.Title != "second" {
		t.Errorf("board is not most-recently-touched first: %s came first", rows[0].Task.Title)
	}
	// Every column the main screen names is present on the row.
	if rows[0].Task.Project == "" || rows[0].Task.Status == "" || rows[0].Task.UpdatedAt.IsZero() {
		t.Errorf("row is missing a column the board needs: %+v", rows[0])
	}
	if rows[0].State != nil {
		t.Error("state should be null, not an empty object, when nobody has left a note")
	}
}

// A project's task list is the board narrowed to one project, so it answers in
// the same shape and carries the same note. When it answered with bare tasks the
// project page made one extra request per row to fetch the note back, each one
// dragging the task's entire worklog with it.
func TestProjectTasksAnswerInBoardShape(t *testing.T) {
	h := newHarness(t)
	h.setUp()

	var project projectDTO
	resp := h.do("POST", "/api/projects", map[string]string{"slug": "cairn", "name": "Cairn"}, "")
	h.decode(resp, &project)

	var created taskDTO
	resp = h.do("POST", "/api/tasks", map[string]any{
		"project_id": project.ID, "title": "port the importer", "status": "queue",
	}, "")
	h.decode(resp, &created)
	resp = h.do("POST", "/api/tasks", map[string]any{
		"project_id": project.ID, "title": "untouched",
	}, "")
	resp.Body.Close()

	resp = h.do("POST", "/api/tasks/"+created.ID+"/transition", map[string]any{
		"to": "active",
		"state": map[string]any{
			"where_i_left_off": "read the failing fixture",
			"next_step":        "make the column mapping name-based",
		},
	}, "")
	h.wantStatus(resp, http.StatusOK)
	resp.Body.Close()

	var rows []boardRowDTO
	resp = h.do("GET", "/api/projects/cairn/tasks", nil, "")
	h.wantStatus(resp, http.StatusOK)
	h.decode(resp, &rows)

	if len(rows) != 2 {
		t.Fatalf("project tasks returned %d rows, want 2", len(rows))
	}

	// Indexed by ref, not by position. Ordering is a property of the shared
	// query and is covered where the clock can be controlled; here every write
	// happens in the same millisecond or two, so asserting order would be
	// asserting the UUIDv7 tiebreak rather than the rule.
	byRef := map[string]boardRowDTO{}
	for _, row := range rows {
		byRef[row.Task.Ref] = row
	}

	worked, ok := byRef[created.Ref]
	if !ok {
		t.Fatalf("%s missing from the project listing", created.Ref)
	}
	if worked.State == nil || worked.State.NextStep != "make the column mapping name-based" {
		t.Errorf("the row does not carry the note: %+v", worked.State)
	}
	for ref, row := range byRef {
		if ref != created.Ref && row.State != nil {
			t.Error("state should be null, not an empty object, when nobody has left a note")
		}
	}

	// And it is the same row the board gives for the same task.
	var board []boardRowDTO
	resp = h.do("GET", "/api/board", nil, "")
	h.decode(resp, &board)
	for _, row := range board {
		if row.Task.Ref == created.Ref && !reflect.DeepEqual(row, worked) {
			t.Errorf("same task differs by endpoint:\n board   %+v\n project %+v", row, worked)
		}
	}
}

// A cross-site form cannot be made to look like this, which is the point.
func TestWritesMustBeJSON(t *testing.T) {
	h := newHarness(t)
	h.setUp()

	req, _ := http.NewRequest("POST", h.server.URL+"/api/projects",
		strings.NewReader("slug=evil&name=evil"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := h.human.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("form-encoded write got %d, want 400", resp.StatusCode)
	}
}

func TestUnknownFieldsAreRejected(t *testing.T) {
	h := newHarness(t)
	h.setUp()
	resp := h.do("POST", "/api/projects", map[string]any{
		"slug": "cairn", "name": "Cairn", "priority": "high",
	}, "")
	h.wantStatus(resp, http.StatusBadRequest)
	resp.Body.Close()
}

func TestFrontendRoutesFallThroughAndApiDoesNot(t *testing.T) {
	h := newHarness(t)

	// A client-side route is served the app shell, not a 404.
	resp := h.do("GET", "/t/cairn-12", nil, "")
	h.wantStatus(resp, http.StatusOK)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(body), "<!doctype html>") {
		t.Errorf("client-side route did not get the app shell: %q", body)
	}

	// A mistyped endpoint is a JSON 404, not the app shell.
	resp = h.do("GET", "/api/tasksss", nil, "")
	h.wantStatus(resp, http.StatusNotFound)
	var failure errorBody
	h.decode(resp, &failure)
	if failure.Error.Kind != "not_found" {
		t.Errorf("unknown endpoint returned %+v", failure)
	}
}

func TestRevokedTokenStopsWorkingImmediately(t *testing.T) {
	h := newHarness(t)
	h.setUp()

	var created newAgentDTO
	resp := h.do("POST", "/api/agents", map[string]string{"name": "claude"}, "")
	h.decode(resp, &created)

	var tokens []tokenDTO
	resp = h.do("GET", "/api/agents/"+created.Agent.ID+"/tokens", nil, "")
	h.wantStatus(resp, http.StatusOK)
	h.decode(resp, &tokens)
	if len(tokens) != 1 {
		t.Fatalf("agent has %d tokens", len(tokens))
	}

	resp = h.do("DELETE", "/api/tokens/"+tokens[0].ID, nil, "")
	h.wantStatus(resp, http.StatusNoContent)
	resp.Body.Close()

	resp = h.do("GET", "/api/board", nil, created.Token)
	h.wantStatus(resp, http.StatusUnauthorized)
	resp.Body.Close()
}

func TestSignInAttemptsAreThrottled(t *testing.T) {
	h := newHarness(t)
	h.setUp()

	// Log out so the cookie is not doing the work.
	resp := h.do("POST", "/api/logout", nil, "")
	resp.Body.Close()

	for i := 0; i < signInLimit; i++ {
		resp := h.do("POST", "/api/login", map[string]string{
			"username": "alper", "password": "wrong",
		}, "")
		h.wantStatus(resp, http.StatusUnauthorized)
		resp.Body.Close()
	}

	// The next attempt is refused before the password is even checked, which is
	// the point: the expensive part never runs.
	resp = h.do("POST", "/api/login", map[string]string{
		"username": "alper", "password": "correct-horse",
	}, "")
	h.wantStatus(resp, http.StatusTooManyRequests)
	if resp.Header.Get("Retry-After") == "" {
		t.Error("429 does not say when to come back")
	}
	var failure errorBody
	h.decode(resp, &failure)
	if failure.Error.Kind != "too_many_attempts" {
		t.Errorf("error kind = %q", failure.Error.Kind)
	}
}

// A correct password clears the record, so a person who mistypes a few times
// and then gets it right is not left in a penalty box.
func TestASuccessfulSignInClearsTheRecord(t *testing.T) {
	h := newHarness(t)
	h.setUp()
	resp := h.do("POST", "/api/logout", nil, "")
	resp.Body.Close()

	for i := 0; i < signInLimit-1; i++ {
		resp := h.do("POST", "/api/login", map[string]string{
			"username": "alper", "password": "wrong",
		}, "")
		resp.Body.Close()
	}

	resp = h.do("POST", "/api/login", map[string]string{
		"username": "alper", "password": "correct-horse",
	}, "")
	h.wantStatus(resp, http.StatusOK)
	resp.Body.Close()

	// Back to a full allowance.
	for i := 0; i < signInLimit; i++ {
		resp := h.do("POST", "/api/login", map[string]string{
			"username": "alper", "password": "wrong",
		}, "")
		if resp.StatusCode == http.StatusTooManyRequests {
			t.Fatalf("still throttled at attempt %d after a successful sign-in", i+1)
		}
		resp.Body.Close()
	}
}

// The window has to actually roll, or a single bad afternoon locks the owner
// out of their own tracker until they restart it.
func TestTheThrottleWindowExpires(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	tr := newThrottle(3, 5*time.Minute)
	tr.now = func() time.Time { return now }

	for i := 0; i < 3; i++ {
		tr.fail()
	}
	if tr.allow() {
		t.Fatal("allowed a fourth attempt inside the window")
	}
	if tr.retryAfter() <= 0 {
		t.Error("retryAfter should be positive while throttled")
	}

	now = now.Add(5*time.Minute + time.Second)
	if !tr.allow() {
		t.Error("still throttled after the window passed")
	}
}

// A fresh Cairn on a public address must not be claimable by whoever finds the
// URL first. Setup is necessarily unauthenticated -- there is nobody to
// authenticate as -- so it is gated on a code the server printed to its log.
func TestAFreshServerCannotBeClaimedByAStranger(t *testing.T) {
	svc, err := service.Open(context.Background(), filepath.Join(t.TempDir(), "cairn.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { svc.Close() })

	const code = "kf9d-2mqx-7rvt-a4bn"
	srv := httptest.NewServer(New(svc, nil, WithSetupCode(code)))
	t.Cleanup(srv.Close)
	jar, _ := cookiejar.New(nil)
	h := &harness{t: t, server: srv, human: &http.Client{Jar: jar}, svc: svc}

	// The screen is told to ask, but never told what to ask for.
	var session sessionDTO
	resp := h.do("GET", "/api/session", nil, "")
	h.decode(resp, &session)
	if !session.NeedsSetupCode {
		t.Error("the setup screen was not told to ask for a code")
	}
	body, _ := json.Marshal(session)
	if strings.Contains(string(body), code) {
		t.Fatal("the session response leaks the setup code")
	}

	// A stranger, with nothing.
	resp = h.do("POST", "/api/setup", map[string]string{
		"username": "not-the-owner", "password": "i-got-here-first",
	}, "")
	h.wantStatus(resp, http.StatusForbidden)
	var failure errorBody
	h.decode(resp, &failure)
	if !strings.Contains(failure.Error.Message, "log") {
		t.Errorf("refusal does not say where to find the code: %q", failure.Error.Message)
	}

	// A stranger, guessing.
	resp = h.do("POST", "/api/setup", map[string]string{
		"username": "not-the-owner", "password": "i-got-here-first",
		"setup_code": "aaaa-bbbb-cccc-dddd",
	}, "")
	h.wantStatus(resp, http.StatusForbidden)
	resp.Body.Close()

	// Nobody has been created by any of that.
	resp = h.do("GET", "/api/session", nil, "")
	h.decode(resp, &session)
	if !session.NeedsSetup {
		t.Fatal("a failed attempt still created a user")
	}

	// The owner, reading their own log. Case and stray spaces are forgiven.
	resp = h.do("POST", "/api/setup", map[string]string{
		"username": "alper", "password": "correct-horse", "setup_code": "  KF9D-2MQX-7RVT-A4BN ",
	}, "")
	h.wantStatus(resp, http.StatusOK)
	h.decode(resp, &session)
	if session.Actor == nil || session.Actor.Name != "alper" {
		t.Fatalf("the owner was not signed in: %+v", session)
	}
	if session.NeedsSetupCode {
		t.Error("the code is still being asked for after setup")
	}
}

// Guessing the code has to cost what guessing a password costs.
func TestSetupCodeGuessesAreThrottled(t *testing.T) {
	svc, err := service.Open(context.Background(), filepath.Join(t.TempDir(), "cairn.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { svc.Close() })

	srv := httptest.NewServer(New(svc, nil, WithSetupCode("kf9d-2mqx-7rvt-a4bn")))
	t.Cleanup(srv.Close)
	jar, _ := cookiejar.New(nil)
	h := &harness{t: t, server: srv, human: &http.Client{Jar: jar}, svc: svc}

	for i := 0; i < signInLimit; i++ {
		resp := h.do("POST", "/api/setup", map[string]string{
			"username": "x", "password": "yyyyyyyy", "setup_code": "wrong",
		}, "")
		resp.Body.Close()
	}
	resp := h.do("POST", "/api/setup", map[string]string{
		"username": "x", "password": "yyyyyyyy", "setup_code": "wrong",
	}, "")
	h.wantStatus(resp, http.StatusTooManyRequests)
	resp.Body.Close()
}

// The code is only demanded where it is needed. Running cairn on a laptop must
// not turn first launch into a scavenger hunt through the terminal.
func TestOnlyRemotelyReachableServersDemandACode(t *testing.T) {
	local := []string{"127.0.0.1:7777", "localhost:7777", "[::1]:7777", "127.0.0.1"}
	for _, addr := range local {
		if !ListenerIsLocalOnly(addr) {
			t.Errorf("%q should be treated as local-only", addr)
		}
	}
	remote := []string{":7777", "0.0.0.0:7777", "192.168.1.10:7777", "[::]:7777"}
	for _, addr := range remote {
		if ListenerIsLocalOnly(addr) {
			t.Errorf("%q is reachable from elsewhere and should demand a code", addr)
		}
	}
}

func TestSetupCodesAreDistinctAndReadable(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		code := NewSetupCode()
		if len(code) != 19 { // four groups of four, three dashes
			t.Fatalf("code %q is %d chars", code, len(code))
		}
		if strings.ContainsAny(code, "ilo01") {
			t.Errorf("code %q contains a character that is misread when copied", code)
		}
		if seen[code] {
			t.Fatalf("duplicate code %q", code)
		}
		seen[code] = true
	}
}
