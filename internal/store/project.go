package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/alperkyoruk/cairn/internal/clock"
	"github.com/alperkyoruk/cairn/internal/model"
)

const projectColumns = `id, slug, name, description, created_at, archived_at`

func scanProject(row interface{ Scan(...any) error }) (model.Project, error) {
	var p model.Project
	var created string
	var archived sql.NullString
	if err := row.Scan(&p.ID, &p.Slug, &p.Name, &p.Description, &created, &archived); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return p, ErrNotFound
		}
		return p, err
	}
	p.CreatedAt = parseTime(created)
	p.ArchivedAt = optTime(archived)
	return p, nil
}

func InsertProject(ctx context.Context, q Queryer, p model.Project) error {
	_, err := q.ExecContext(ctx,
		`INSERT INTO project (id, slug, name, description, created_at) VALUES (?, ?, ?, ?, ?)`,
		p.ID, p.Slug, p.Name, p.Description, clock.Format(p.CreatedAt))
	if err != nil {
		return fmt.Errorf("insert project: %w", err)
	}
	return nil
}

func GetProject(ctx context.Context, q Queryer, id string) (model.Project, error) {
	return scanProject(q.QueryRowContext(ctx, `SELECT `+projectColumns+` FROM project WHERE id = ?`, id))
}

func GetProjectBySlug(ctx context.Context, q Queryer, slug string) (model.Project, error) {
	return scanProject(q.QueryRowContext(ctx, `SELECT `+projectColumns+` FROM project WHERE slug = ?`, slug))
}

func ListProjects(ctx context.Context, q Queryer) ([]model.Project, error) {
	rows, err := q.QueryContext(ctx, `SELECT `+projectColumns+` FROM project ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func UpdateProject(ctx context.Context, q Queryer, p model.Project) error {
	res, err := q.ExecContext(ctx,
		`UPDATE project SET name = ?, description = ?, archived_at = ? WHERE id = ?`,
		p.Name, p.Description, storeTime(p.ArchivedAt), p.ID)
	if err != nil {
		return fmt.Errorf("update project: %w", err)
	}
	return expectOne(res, "project")
}

func DeleteProject(ctx context.Context, q Queryer, id string) error {
	res, err := q.ExecContext(ctx, `DELETE FROM project WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	return expectOne(res, "project")
}

func CountTasksInProject(ctx context.Context, q Queryer, projectID string) (int, error) {
	var n int
	err := q.QueryRowContext(ctx, `SELECT count(*) FROM task WHERE project_id = ?`, projectID).Scan(&n)
	return n, err
}

// NextTaskNumber allocates the next per-project task number.
//
// SQLite has no portable sequence, and SELECT ... FOR UPDATE does not exist
// here, so the counter is bumped by an UPDATE and read back inside the same
// write transaction. The single write connection makes that atomic.
func NextTaskNumber(ctx context.Context, q Queryer, projectID string) (int, error) {
	res, err := q.ExecContext(ctx,
		`UPDATE project SET next_number = next_number + 1 WHERE id = ?`, projectID)
	if err != nil {
		return 0, fmt.Errorf("allocate task number: %w", err)
	}
	if err := expectOne(res, "project"); err != nil {
		return 0, err
	}
	var n int
	if err := q.QueryRowContext(ctx,
		`SELECT next_number - 1 FROM project WHERE id = ?`, projectID).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}
