// Package model holds the types that cross Cairn's layer boundaries.
//
// They live here rather than in internal/store so that the HTTP and MCP layers
// can name a Task without importing the package that knows SQL. That import
// rule is what keeps internal/service the only way to reach the database, and
// TestOnlyServiceReachesTheStore enforces it.
package model

import (
	"time"

	"github.com/alperkyoruk/cairn/internal/workflow"
)

// Actor is a human or an agent. Identity is separate from credentials: rotating
// an agent's token must not orphan the worklog entries it has already written.
type Actor struct {
	ID        string
	Type      workflow.ActorType
	Name      string
	CreatedAt time.Time
}

// Token is one credential belonging to one actor. The human's browser session
// is a token too, which is why both interfaces resolve permissions identically.
type Token struct {
	ID         string
	ActorID    string
	Name       string
	Prefix     string // the token's opening characters, for identifying it later
	CreatedAt  time.Time
	ExpiresAt  *time.Time
	LastUsedAt *time.Time
	RevokedAt  *time.Time
}

type Project struct {
	ID          string
	Slug        string
	Name        string
	Description string
	CreatedAt   time.Time
	ArchivedAt  *time.Time
}

type Task struct {
	ID          string
	ProjectID   string
	Number      int
	Title       string
	Body        string
	Status      workflow.Status
	CreatedAt   time.Time
	UpdatedAt   time.Time
	ProjectSlug string // joined for display; "cairn-12"
}

// Ref is the short handle used to talk about a task: "cairn-12".
func (t Task) Ref() string { return t.ProjectSlug + "-" + itoa(t.Number) }

// State is the note left for whoever picks the task up next. Overwritten in
// place, always current.
type State struct {
	TaskID        string
	WhereILeftOff string
	NextStep      string
	BlockedOn     string
	UpdatedBy     string // actor id
	UpdatedByName string // joined for display
	UpdatedAt     time.Time
}

// WorklogEntry is one attempt, recorded forever.
type WorklogEntry struct {
	TaskID       string
	ID           string
	ActorID      string
	ActorName    string // joined for display
	CreatedAt    time.Time
	WhatWasTried string
	Outcome      string
	FromStatus   workflow.Status // empty unless the entry accompanied a transition
	ToStatus     workflow.Status
}

// Attempt is a worklog entry recorded without moving the task, and newer than
// the note on it.
//
// It exists because that event was invisible. Appending to the worklog bumps
// task.updated_at, so the row rises to the top of a most-recently-touched
// board, while next_step and updated_by both come from task_state and do not
// change -- the row moves and says nothing about why. That is exactly what a
// second agent reviewing a task in review produces: the recommendation is the
// point of delegating the review, and it was only visible by opening the task.
type Attempt struct {
	Actor        string
	WhatWasTried string
	At           time.Time
}

// BoardRow is one line of the main screen.
type BoardRow struct {
	Task  Task
	State *State // nil when nobody has left a note yet

	// Attempt is set only when the last thing to happen was an unaccompanied
	// worklog entry. A transition writes one too, but the status carries that.
	Attempt *Attempt

	// CanMoveTo is the moves available to the actor who asked, from where the
	// task is now. It is filled by the service, which is the only layer that
	// knows who is asking, so a row and the task detail behind it cannot
	// disagree about what is permitted.
	//
	// Read by the MCP surface, where an agent scanning a listing for work it may
	// claim would otherwise guess or spend a call per row. The web board does
	// not carry it: the interface asks for a task's moves when it opens the
	// task, which is the only place it offers buttons.
	CanMoveTo []workflow.Status
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
