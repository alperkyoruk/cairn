package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/alperkyoruk/cairn/internal/clock"
	"github.com/alperkyoruk/cairn/internal/model"
	"github.com/alperkyoruk/cairn/internal/workflow"
)

const actorColumns = `id, actor_type, name, created_at`

func scanActor(row interface{ Scan(...any) error }) (model.Actor, error) {
	var a model.Actor
	var typ, created string
	if err := row.Scan(&a.ID, &typ, &a.Name, &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return a, ErrNotFound
		}
		return a, err
	}
	a.Type = workflow.ActorType(typ)
	a.CreatedAt = parseTime(created)
	return a, nil
}

// InsertActor creates an actor. passwordHash must be empty for agents; the
// schema rejects the combination otherwise.
func InsertActor(ctx context.Context, q Queryer, a model.Actor, passwordHash string) error {
	var hash any
	if passwordHash != "" {
		hash = passwordHash
	}
	_, err := q.ExecContext(ctx,
		`INSERT INTO actor (id, actor_type, name, password_hash, created_at) VALUES (?, ?, ?, ?, ?)`,
		a.ID, string(a.Type), a.Name, hash, clock.Format(a.CreatedAt))
	if err != nil {
		return fmt.Errorf("insert actor: %w", err)
	}
	return nil
}

func GetActor(ctx context.Context, q Queryer, id string) (model.Actor, error) {
	return scanActor(q.QueryRowContext(ctx, `SELECT `+actorColumns+` FROM actor WHERE id = ?`, id))
}

func GetActorByName(ctx context.Context, q Queryer, name string) (model.Actor, error) {
	return scanActor(q.QueryRowContext(ctx, `SELECT `+actorColumns+` FROM actor WHERE name = ?`, name))
}

// GetPasswordHash returns the stored hash for an actor, or "" if it has none.
func GetPasswordHash(ctx context.Context, q Queryer, actorID string) (string, error) {
	var hash sql.NullString
	err := q.QueryRowContext(ctx, `SELECT password_hash FROM actor WHERE id = ?`, actorID).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return hash.String, err
}

func SetPasswordHash(ctx context.Context, q Queryer, actorID, hash string) error {
	res, err := q.ExecContext(ctx, `UPDATE actor SET password_hash = ? WHERE id = ?`, hash, actorID)
	if err != nil {
		return fmt.Errorf("set password: %w", err)
	}
	return expectOne(res, "actor")
}

// CountActors reports how many actors of a type exist. Used to decide whether
// first-launch setup still needs to run.
func CountActors(ctx context.Context, q Queryer, t workflow.ActorType) (int, error) {
	var n int
	err := q.QueryRowContext(ctx, `SELECT count(*) FROM actor WHERE actor_type = ?`, string(t)).Scan(&n)
	return n, err
}

func ListActors(ctx context.Context, q Queryer, t workflow.ActorType) ([]model.Actor, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT `+actorColumns+` FROM actor WHERE actor_type = ? ORDER BY name`, string(t))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Actor
	for rows.Next() {
		a, err := scanActor(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func expectOne(res sql.Result, what string) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("%s: %w", what, ErrNotFound)
	}
	return nil
}
