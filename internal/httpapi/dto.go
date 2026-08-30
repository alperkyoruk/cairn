package httpapi

import (
	"time"

	"github.com/alperkyoruk/cairn/internal/model"
	"github.com/alperkyoruk/cairn/internal/service"
	"github.com/alperkyoruk/cairn/internal/workflow"
)

// The wire types are deliberately separate from internal/model. The database
// shape and the API shape are allowed to diverge, and the JSON field names are
// the vocabulary of the product -- where_i_left_off, next_step, blocked_on --
// which should not depend on what a column happens to be called.

type actorDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type sessionDTO struct {
	NeedsSetup bool      `json:"needs_setup"`
	Actor      *actorDTO `json:"actor"`
}

type projectDTO struct {
	ID          string     `json:"id"`
	Slug        string     `json:"slug"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	CreatedAt   time.Time  `json:"created_at"`
	ArchivedAt  *time.Time `json:"archived_at"`
}

type taskDTO struct {
	ID        string    `json:"id"`
	Ref       string    `json:"ref"`
	ProjectID string    `json:"project_id"`
	Project   string    `json:"project"`
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type stateDTO struct {
	WhereILeftOff string    `json:"where_i_left_off"`
	NextStep      string    `json:"next_step"`
	BlockedOn     string    `json:"blocked_on"`
	UpdatedBy     string    `json:"updated_by"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type worklogDTO struct {
	ID           string    `json:"id"`
	Actor        string    `json:"actor"`
	CreatedAt    time.Time `json:"created_at"`
	WhatWasTried string    `json:"what_was_tried"`
	Outcome      string    `json:"outcome"`
	FromStatus   string    `json:"from_status,omitempty"`
	ToStatus     string    `json:"to_status,omitempty"`
}

type boardRowDTO struct {
	Task  taskDTO   `json:"task"`
	State *stateDTO `json:"state"`
}

// taskDetailDTO carries can_move_to so the interface never has to know the
// state machine. The buttons on a task are whatever the server says this actor
// may do next, which is how the UI stays honest without duplicating the rules.
type taskDetailDTO struct {
	Task      taskDTO      `json:"task"`
	State     *stateDTO    `json:"state"`
	Worklog   []worklogDTO `json:"worklog"`
	CanMoveTo []string     `json:"can_move_to"`
}

type tokenDTO struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	RevokedAt  *time.Time `json:"revoked_at"`
}

// newAgentDTO carries the one and only sight of a new token's secret.
type newAgentDTO struct {
	Agent actorDTO `json:"agent"`
	Token string   `json:"token"`
}

func toActor(a service.Actor) *actorDTO {
	if a.Anonymous() {
		return nil
	}
	return &actorDTO{ID: a.ID(), Name: a.Name(), Type: string(a.Type())}
}

func toModelActor(a model.Actor) actorDTO {
	return actorDTO{ID: a.ID, Name: a.Name, Type: string(a.Type)}
}

func toProject(p model.Project) projectDTO {
	return projectDTO{
		ID: p.ID, Slug: p.Slug, Name: p.Name, Description: p.Description,
		CreatedAt: p.CreatedAt, ArchivedAt: p.ArchivedAt,
	}
}

func toTask(t model.Task) taskDTO {
	return taskDTO{
		ID: t.ID, Ref: t.Ref(), ProjectID: t.ProjectID, Project: t.ProjectSlug,
		Number: t.Number, Title: t.Title, Body: t.Body, Status: string(t.Status),
		CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
	}
}

func toState(s *model.State) *stateDTO {
	if s == nil {
		return nil
	}
	return &stateDTO{
		WhereILeftOff: s.WhereILeftOff, NextStep: s.NextStep, BlockedOn: s.BlockedOn,
		UpdatedBy: s.UpdatedByName, UpdatedAt: s.UpdatedAt,
	}
}

func toWorklog(entries []model.WorklogEntry) []worklogDTO {
	out := make([]worklogDTO, 0, len(entries))
	for _, e := range entries {
		out = append(out, worklogDTO{
			ID: e.ID, Actor: e.ActorName, CreatedAt: e.CreatedAt,
			WhatWasTried: e.WhatWasTried, Outcome: e.Outcome,
			FromStatus: string(e.FromStatus), ToStatus: string(e.ToStatus),
		})
	}
	return out
}

func toDetail(d service.TaskDetail) taskDetailDTO {
	moves := make([]string, 0, len(d.CanMoveTo))
	for _, s := range d.CanMoveTo {
		moves = append(moves, string(s))
	}
	return taskDetailDTO{
		Task: toTask(d.Task), State: toState(d.State),
		Worklog: toWorklog(d.Worklog), CanMoveTo: moves,
	}
}

func toToken(t model.Token) tokenDTO {
	return tokenDTO{
		ID: t.ID, Name: t.Name, Prefix: t.Prefix, CreatedAt: t.CreatedAt,
		LastUsedAt: t.LastUsedAt, RevokedAt: t.RevokedAt,
	}
}

// stateInput is shared by the transition and state endpoints so the field names
// an agent sends over MCP and the ones the browser sends are the same words.
type stateInput struct {
	WhereILeftOff string `json:"where_i_left_off"`
	NextStep      string `json:"next_step"`
	BlockedOn     string `json:"blocked_on"`
}

func (in *stateInput) toService() *service.StateInput {
	if in == nil {
		return nil
	}
	return &service.StateInput{
		WhereILeftOff: in.WhereILeftOff, NextStep: in.NextStep, BlockedOn: in.BlockedOn,
	}
}

type worklogInput struct {
	WhatWasTried string `json:"what_was_tried"`
	Outcome      string `json:"outcome"`
}

func (in *worklogInput) toService() *service.WorklogInput {
	if in == nil {
		return nil
	}
	return &service.WorklogInput{WhatWasTried: in.WhatWasTried, Outcome: in.Outcome}
}

func statuses(ss []workflow.Status) []string {
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		out = append(out, string(s))
	}
	return out
}
