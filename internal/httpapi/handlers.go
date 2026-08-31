package httpapi

import (
	"net/http"

	"github.com/alperkyoruk/cairn/internal/model"
	"github.com/alperkyoruk/cairn/internal/service"
	"github.com/alperkyoruk/cairn/internal/workflow"
)

// --- session ----------------------------------------------------------------

// handleSession is what the frontend asks on load: is there a user yet, and am
// I signed in. One request decides between the setup screen, the login screen
// and the app.
func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	needsSetup, err := s.svc.NeedsSetup(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	out := sessionDTO{NeedsSetup: needsSetup}
	if actor, err := s.svc.Authenticate(r.Context(), credential(r)); err == nil {
		out.Actor = toActor(actor)
	}
	writeJSON(w, http.StatusOK, out)
}

type credentialsBody struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	if err := checkJSONWrite(r); err != nil {
		writeError(w, err)
		return
	}
	var in credentialsBody
	if err := decode(r, &in); err != nil {
		writeError(w, err)
		return
	}
	if !s.signIn.allow() {
		writeThrottled(w, s.signIn.retryAfter())
		return
	}
	if _, err := s.svc.Setup(r.Context(), in.Username, in.Password); err != nil {
		s.signIn.fail()
		writeError(w, err)
		return
	}
	// Setting up signs you in; there is no second step.
	secret, err := s.svc.Login(r.Context(), in.Username, in.Password)
	if err != nil {
		writeError(w, err)
		return
	}
	s.writeSession(w, r, secret)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if err := checkJSONWrite(r); err != nil {
		writeError(w, err)
		return
	}
	var in credentialsBody
	if err := decode(r, &in); err != nil {
		writeError(w, err)
		return
	}
	if !s.signIn.allow() {
		writeThrottled(w, s.signIn.retryAfter())
		return
	}
	secret, err := s.svc.Login(r.Context(), in.Username, in.Password)
	if err != nil {
		s.signIn.fail()
		writeError(w, err)
		return
	}
	s.signIn.succeed()
	s.writeSession(w, r, secret)
}

// writeSession sets the cookie and answers with the session the caller now
// holds. It resolves the secret it just minted rather than re-reading the
// request, which does not carry the cookie until the next round trip.
func (s *Server) writeSession(w http.ResponseWriter, r *http.Request, secret string) {
	actor, err := s.svc.Authenticate(r.Context(), secret)
	if err != nil {
		writeError(w, err)
		return
	}
	s.setSession(w, secret)
	writeJSON(w, http.StatusOK, sessionDTO{NeedsSetup: false, Actor: toActor(actor)})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if actor, err := s.svc.Authenticate(r.Context(), credential(r)); err == nil {
		_ = s.svc.Logout(r.Context(), actor)
	}
	s.clearSession(w)
	writeJSON(w, http.StatusNoContent, nil)
}

// --- board ------------------------------------------------------------------

func (s *Server) handleBoard(w http.ResponseWriter, r *http.Request, actor service.Actor) {
	rows, err := s.svc.Board(r.Context(), actor, service.BoardQuery{})
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]boardRowDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, boardRowDTO{Task: toTask(row.Task), State: toState(row.State)})
	}
	writeJSON(w, http.StatusOK, out)
}

// --- projects ---------------------------------------------------------------

func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request, actor service.Actor) {
	projects, err := s.svc.ListProjects(r.Context(), actor)
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]projectDTO, 0, len(projects))
	for _, p := range projects {
		out = append(out, toProject(p))
	}
	writeJSON(w, http.StatusOK, out)
}

type projectBody struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request, actor service.Actor) {
	var in projectBody
	if err := decode(r, &in); err != nil {
		writeError(w, err)
		return
	}
	p, err := s.svc.CreateProject(r.Context(), actor, in.Slug, in.Name, in.Description)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toProject(p))
}

// handleGetProject accepts an id or a slug, because a URL bar is a place where
// people type "cairn", not a UUID.
func (s *Server) handleGetProject(w http.ResponseWriter, r *http.Request, actor service.Actor) {
	p, err := s.resolveProject(r, actor)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toProject(p))
}

func (s *Server) handleUpdateProject(w http.ResponseWriter, r *http.Request, actor service.Actor) {
	p, err := s.resolveProject(r, actor)
	if err != nil {
		writeError(w, err)
		return
	}
	var in projectBody
	if err := decode(r, &in); err != nil {
		writeError(w, err)
		return
	}
	updated, err := s.svc.UpdateProject(r.Context(), actor, p.ID, in.Name, in.Description)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toProject(updated))
}

func (s *Server) handleDeleteProject(w http.ResponseWriter, r *http.Request, actor service.Actor) {
	p, err := s.resolveProject(r, actor)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.svc.DeleteProject(r.Context(), actor, p.ID); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request, actor service.Actor) {
	p, err := s.resolveProject(r, actor)
	if err != nil {
		writeError(w, err)
		return
	}
	tasks, err := s.svc.ListTasks(r.Context(), actor, p.ID, service.BoardQuery{})
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]taskDTO, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, toTask(t))
	}
	writeJSON(w, http.StatusOK, out)
}

// --- tasks ------------------------------------------------------------------

type createTaskBody struct {
	ProjectID string `json:"project_id"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	Status    string `json:"status"`
}

func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request, actor service.Actor) {
	var in createTaskBody
	if err := decode(r, &in); err != nil {
		writeError(w, err)
		return
	}
	task, err := s.svc.CreateTask(r.Context(), actor, service.CreateTaskInput{
		ProjectID: in.ProjectID, Title: in.Title, Body: in.Body,
		Status: workflow.Status(in.Status),
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toTask(task))
}

func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request, actor service.Actor) {
	detail, err := s.svc.LookupTask(r.Context(), actor, r.PathValue("ref"), service.TaskQuery{})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toDetail(detail))
}

type updateTaskBody struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

func (s *Server) handleUpdateTask(w http.ResponseWriter, r *http.Request, actor service.Actor) {
	id, err := s.resolveTaskID(r, actor)
	if err != nil {
		writeError(w, err)
		return
	}
	var in updateTaskBody
	if err := decode(r, &in); err != nil {
		writeError(w, err)
		return
	}
	if _, err := s.svc.UpdateTask(r.Context(), actor, id, in.Title, in.Body); err != nil {
		writeError(w, err)
		return
	}
	s.respondWithTask(w, r, actor, id)
}

func (s *Server) handleDeleteTask(w http.ResponseWriter, r *http.Request, actor service.Actor) {
	id, err := s.resolveTaskID(r, actor)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.svc.DeleteTask(r.Context(), actor, id); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

type transitionBody struct {
	To      string        `json:"to"`
	State   *stateInput   `json:"state"`
	Worklog *worklogInput `json:"worklog"`
}

func (s *Server) handleTransition(w http.ResponseWriter, r *http.Request, actor service.Actor) {
	id, err := s.resolveTaskID(r, actor)
	if err != nil {
		writeError(w, err)
		return
	}
	var in transitionBody
	if err := decode(r, &in); err != nil {
		writeError(w, err)
		return
	}
	if _, err := s.svc.Transition(r.Context(), actor, id, service.TransitionInput{
		To: workflow.Status(in.To), State: in.State.toService(), Worklog: in.Worklog.toService(),
	}); err != nil {
		writeError(w, err)
		return
	}
	s.respondWithTask(w, r, actor, id)
}

func (s *Server) handleWriteState(w http.ResponseWriter, r *http.Request, actor service.Actor) {
	id, err := s.resolveTaskID(r, actor)
	if err != nil {
		writeError(w, err)
		return
	}
	var in stateInput
	if err := decode(r, &in); err != nil {
		writeError(w, err)
		return
	}
	if err := s.svc.WriteState(r.Context(), actor, id, *in.toService()); err != nil {
		writeError(w, err)
		return
	}
	s.respondWithTask(w, r, actor, id)
}

func (s *Server) handleAppendWorklog(w http.ResponseWriter, r *http.Request, actor service.Actor) {
	id, err := s.resolveTaskID(r, actor)
	if err != nil {
		writeError(w, err)
		return
	}
	var in worklogInput
	if err := decode(r, &in); err != nil {
		writeError(w, err)
		return
	}
	if err := s.svc.AppendWorklog(r.Context(), actor, id, *in.toService()); err != nil {
		writeError(w, err)
		return
	}
	s.respondWithTask(w, r, actor, id)
}

// --- agents -----------------------------------------------------------------

func (s *Server) handleListAgents(w http.ResponseWriter, r *http.Request, actor service.Actor) {
	agents, err := s.svc.ListAgents(r.Context(), actor)
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]actorDTO, 0, len(agents))
	for _, a := range agents {
		out = append(out, toModelActor(a))
	}
	writeJSON(w, http.StatusOK, out)
}

type nameBody struct {
	Name string `json:"name"`
}

func (s *Server) handleCreateAgent(w http.ResponseWriter, r *http.Request, actor service.Actor) {
	var in nameBody
	if err := decode(r, &in); err != nil {
		writeError(w, err)
		return
	}
	agent, secret, err := s.svc.CreateAgent(r.Context(), actor, in.Name)
	if err != nil {
		writeError(w, err)
		return
	}
	// The only time this secret is ever visible.
	writeJSON(w, http.StatusCreated, newAgentDTO{Agent: toModelActor(agent), Token: secret})
}

func (s *Server) handleListTokens(w http.ResponseWriter, r *http.Request, actor service.Actor) {
	tokens, err := s.svc.ListTokens(r.Context(), actor, r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]tokenDTO, 0, len(tokens))
	for _, t := range tokens {
		out = append(out, toToken(t))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleIssueToken(w http.ResponseWriter, r *http.Request, actor service.Actor) {
	var in nameBody
	if err := decode(r, &in); err != nil {
		writeError(w, err)
		return
	}
	secret, err := s.svc.IssueToken(r.Context(), actor, r.PathValue("id"), in.Name)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"token": secret})
}

func (s *Server) handleRevokeToken(w http.ResponseWriter, r *http.Request, actor service.Actor) {
	if err := s.svc.RevokeToken(r.Context(), actor, r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

// --- shared -----------------------------------------------------------------

// respondWithTask returns the task as it now stands, including the moves the
// caller may make next. Every mutation answers with this, so the interface
// never has to guess what changed or re-derive which buttons to show.
func (s *Server) respondWithTask(w http.ResponseWriter, r *http.Request, actor service.Actor, id string) {
	detail, err := s.svc.GetTask(r.Context(), actor, id, service.TaskQuery{})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toDetail(detail))
}

func (s *Server) resolveTaskID(r *http.Request, actor service.Actor) (string, error) {
	detail, err := s.svc.LookupTask(r.Context(), actor, r.PathValue("ref"), service.TaskQuery{})
	if err != nil {
		return "", err
	}
	return detail.Task.ID, nil
}

func (s *Server) resolveProject(r *http.Request, actor service.Actor) (model.Project, error) {
	key := r.PathValue("id")
	p, err := s.svc.GetProject(r.Context(), actor, key)
	if err == nil {
		return p, nil
	}
	if service.KindOf(err) != service.KindNotFound {
		return model.Project{}, err
	}
	projects, listErr := s.svc.ListProjects(r.Context(), actor)
	if listErr != nil {
		return model.Project{}, listErr
	}
	for _, candidate := range projects {
		if candidate.Slug == key {
			return candidate, nil
		}
	}
	return model.Project{}, err
}
