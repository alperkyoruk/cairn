package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/alperkyoruk/cairn/internal/clock"
	"github.com/alperkyoruk/cairn/internal/model"
	"github.com/alperkyoruk/cairn/internal/workflow"
)

const taskColumns = `t.id, t.project_id, t.number, t.title, t.body, t.status, t.created_at, t.updated_at, p.slug`

func scanTask(row interface{ Scan(...any) error }) (model.Task, error) {
	var t model.Task
	var status, created, updated string
	err := row.Scan(&t.ID, &t.ProjectID, &t.Number, &t.Title, &t.Body, &status,
		&created, &updated, &t.ProjectSlug)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return t, ErrNotFound
		}
		return t, err
	}
	t.Status = workflow.Status(status)
	t.CreatedAt, t.UpdatedAt = parseTime(created), parseTime(updated)
	return t, nil
}

func InsertTask(ctx context.Context, q Queryer, t model.Task) error {
	_, err := q.ExecContext(ctx,
		`INSERT INTO task (id, project_id, number, title, body, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.ProjectID, t.Number, t.Title, t.Body, string(t.Status),
		clock.Format(t.CreatedAt), clock.Format(t.UpdatedAt))
	if err != nil {
		return fmt.Errorf("insert task: %w", err)
	}
	return nil
}

func GetTask(ctx context.Context, q Queryer, id string) (model.Task, error) {
	return scanTask(q.QueryRowContext(ctx,
		`SELECT `+taskColumns+` FROM task t JOIN project p ON p.id = t.project_id WHERE t.id = ?`, id))
}

// GetTaskByRef looks a task up the way people and agents refer to it: "cairn-12".
func GetTaskByRef(ctx context.Context, q Queryer, slug string, number int) (model.Task, error) {
	return scanTask(q.QueryRowContext(ctx,
		`SELECT `+taskColumns+` FROM task t JOIN project p ON p.id = t.project_id
		 WHERE p.slug = ? AND t.number = ?`, slug, number))
}

// UpdateTaskStatus moves a task, but only if it is still where the caller
// thought it was. A false return means someone got there first.
func UpdateTaskStatus(ctx context.Context, q Queryer, id string, from, to workflow.Status, at time.Time) (bool, error) {
	res, err := q.ExecContext(ctx,
		`UPDATE task SET status = ?, updated_at = ? WHERE id = ? AND status = ?`,
		string(to), clock.Format(at), id, string(from))
	if err != nil {
		return false, fmt.Errorf("update task status: %w", err)
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// TouchTask bumps updated_at, which is what the main screen sorts on.
func TouchTask(ctx context.Context, q Queryer, id string, at time.Time) error {
	res, err := q.ExecContext(ctx, `UPDATE task SET updated_at = ? WHERE id = ?`, clock.Format(at), id)
	if err != nil {
		return fmt.Errorf("touch task: %w", err)
	}
	return expectOne(res, "task")
}

func UpdateTaskFields(ctx context.Context, q Queryer, id, title, body string, at time.Time) error {
	res, err := q.ExecContext(ctx,
		`UPDATE task SET title = ?, body = ?, updated_at = ? WHERE id = ?`,
		title, body, clock.Format(at), id)
	if err != nil {
		return fmt.Errorf("update task: %w", err)
	}
	return expectOne(res, "task")
}

// DeleteTask removes the task. Its state and worklog go with it, by cascade.
func DeleteTask(ctx context.Context, q Queryer, id string) error {
	res, err := q.ExecContext(ctx, `DELETE FROM task WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	return expectOne(res, "task")
}

func ListTasksByProject(ctx context.Context, q Queryer, projectID string) ([]model.Task, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT `+taskColumns+` FROM task t JOIN project p ON p.id = t.project_id
		 WHERE t.project_id = ? ORDER BY t.updated_at DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// --- state ------------------------------------------------------------------

// UpsertState overwrites the note on a task. There is deliberately no history
// here: that is what the worklog is for.
func UpsertState(ctx context.Context, q Queryer, s model.State) error {
	_, err := q.ExecContext(ctx, `
		INSERT INTO task_state (task_id, where_i_left_off, next_step, blocked_on, updated_by, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (task_id) DO UPDATE SET
			where_i_left_off = excluded.where_i_left_off,
			next_step        = excluded.next_step,
			blocked_on       = excluded.blocked_on,
			updated_by       = excluded.updated_by,
			updated_at       = excluded.updated_at`,
		s.TaskID, s.WhereILeftOff, s.NextStep, s.BlockedOn, s.UpdatedBy, clock.Format(s.UpdatedAt))
	if err != nil {
		return fmt.Errorf("write state: %w", err)
	}
	return nil
}

// GetState returns the note on a task, or ErrNotFound if nobody has left one.
func GetState(ctx context.Context, q Queryer, taskID string) (model.State, error) {
	var s model.State
	var updated string
	err := q.QueryRowContext(ctx, `
		SELECT s.task_id, s.where_i_left_off, s.next_step, s.blocked_on, s.updated_by, a.name, s.updated_at
		FROM task_state s JOIN actor a ON a.id = s.updated_by
		WHERE s.task_id = ?`, taskID).
		Scan(&s.TaskID, &s.WhereILeftOff, &s.NextStep, &s.BlockedOn, &s.UpdatedBy, &s.UpdatedByName, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return s, ErrNotFound
	}
	if err != nil {
		return s, err
	}
	s.UpdatedAt = parseTime(updated)
	return s, nil
}

// --- worklog ----------------------------------------------------------------

// InsertWorklog appends an entry. Nothing in this package updates or deletes one.
func InsertWorklog(ctx context.Context, q Queryer, e model.WorklogEntry) error {
	var from, to any
	if e.FromStatus != "" {
		from = string(e.FromStatus)
	}
	if e.ToStatus != "" {
		to = string(e.ToStatus)
	}
	_, err := q.ExecContext(ctx, `
		INSERT INTO worklog (id, task_id, actor_id, created_at, what_was_tried, outcome, from_status, to_status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.TaskID, e.ActorID, clock.Format(e.CreatedAt), e.WhatWasTried, e.Outcome, from, to)
	if err != nil {
		return fmt.Errorf("append worklog: %w", err)
	}
	return nil
}

func ListWorklog(ctx context.Context, q Queryer, taskID string) ([]model.WorklogEntry, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT w.id, w.task_id, w.actor_id, a.name, w.created_at, w.what_was_tried, w.outcome,
		       w.from_status, w.to_status
		FROM worklog w JOIN actor a ON a.id = w.actor_id
		WHERE w.task_id = ? ORDER BY w.created_at, w.id`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.WorklogEntry
	for rows.Next() {
		var e model.WorklogEntry
		var created string
		var from, to sql.NullString
		if err := rows.Scan(&e.ID, &e.TaskID, &e.ActorID, &e.ActorName, &created,
			&e.WhatWasTried, &e.Outcome, &from, &to); err != nil {
			return nil, err
		}
		e.CreatedAt = parseTime(created)
		e.FromStatus, e.ToStatus = workflow.Status(from.String), workflow.Status(to.String)
		out = append(out, e)
	}
	return out, rows.Err()
}

// --- the main screen --------------------------------------------------------

// Board returns every task across every project, most recently touched first,
// each with its note if one exists. This is the query behind the root URL.
func Board(ctx context.Context, q Queryer) ([]model.BoardRow, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT `+taskColumns+`,
		       s.where_i_left_off, s.next_step, s.blocked_on, s.updated_by, a.name, s.updated_at
		FROM task t
		JOIN project p        ON p.id = t.project_id
		LEFT JOIN task_state s ON s.task_id = t.id
		LEFT JOIN actor a      ON a.id = s.updated_by
		ORDER BY t.updated_at DESC, t.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.BoardRow
	for rows.Next() {
		var t model.Task
		var status, created, updated string
		var where, next, blocked, by, byName, at sql.NullString

		if err := rows.Scan(&t.ID, &t.ProjectID, &t.Number, &t.Title, &t.Body, &status,
			&created, &updated, &t.ProjectSlug,
			&where, &next, &blocked, &by, &byName, &at); err != nil {
			return nil, err
		}
		t.Status = workflow.Status(status)
		t.CreatedAt, t.UpdatedAt = parseTime(created), parseTime(updated)

		row := model.BoardRow{Task: t}
		if by.Valid {
			row.State = &model.State{
				TaskID:        t.ID,
				WhereILeftOff: where.String,
				NextStep:      next.String,
				BlockedOn:     blocked.String,
				UpdatedBy:     by.String,
				UpdatedByName: byName.String,
				UpdatedAt:     parseTime(at.String),
			}
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
