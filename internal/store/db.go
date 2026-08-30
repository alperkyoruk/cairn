// Package store is the only package in Cairn that knows SQL.
//
// It contains no rules. It does not know who may move a task from review to
// done, and it must never learn: that question belongs to internal/workflow,
// asked by internal/service, which is the only caller allowed in here.
package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/alperkyoruk/cairn/internal/clock"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// ErrNotFound is returned when a lookup matches no row. Callers in the service
// layer translate it into their own error kind.
var ErrNotFound = errors.New("not found")

// DB holds two connection pools onto the same SQLite file.
//
// SQLite permits exactly one writer at a time. Rather than letting N agents
// race for that lock and cope with SQLITE_BUSY, the write pool is pinned to a
// single connection so writes queue in Go instead. Reads get their own pool and
// run concurrently against the WAL.
type DB struct {
	w *sql.DB
	r *sql.DB
}

// Queryer is the subset of *sql.DB and *sql.Tx the query functions need, so the
// same function works inside or outside a transaction.
type Queryer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func dsn(file string, immediate bool) string {
	q := url.Values{}
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "synchronous(NORMAL)")
	if immediate {
		// Take the write lock at BEGIN rather than on first write, so a
		// transaction cannot deadlock trying to upgrade from read to write.
		q.Set("_txlock", "immediate")
	}
	return "file:" + file + "?" + q.Encode()
}

// Open connects to the SQLite database at file, creating it if necessary.
func Open(file string) (*DB, error) {
	w, err := sql.Open("sqlite", dsn(file, true))
	if err != nil {
		return nil, fmt.Errorf("open write pool: %w", err)
	}
	w.SetMaxOpenConns(1)

	r, err := sql.Open("sqlite", dsn(file, false))
	if err != nil {
		w.Close()
		return nil, fmt.Errorf("open read pool: %w", err)
	}
	r.SetMaxOpenConns(8)

	db := &DB{w: w, r: r}
	if err := w.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return db, nil
}

func (db *DB) Close() error {
	return errors.Join(db.w.Close(), db.r.Close())
}

// Read returns the read pool. Callers must not write through it.
func (db *DB) Read() Queryer { return db.r }

// Write runs fn inside a write transaction, committing if it returns nil and
// rolling back otherwise. Every mutation in Cairn goes through here, which is
// what makes "status, state and worklog move together or not at all" true.
func (db *DB) Write(ctx context.Context, fn func(Queryer) error) error {
	tx, err := db.w.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// Migrate applies any migrations the database has not seen yet.
func (db *DB) Migrate(ctx context.Context) error {
	if _, err := db.w.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migration (
			version    INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL
		)`); err != nil {
		return fmt.Errorf("create schema_migration: %w", err)
	}

	applied := map[int]bool{}
	rows, err := db.w.QueryContext(ctx, `SELECT version FROM schema_migration`)
	if err != nil {
		return fmt.Errorf("read schema_migration: %w", err)
	}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			return err
		}
		applied[v] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	files, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(files))
	for _, f := range files {
		names = append(names, f.Name())
	}
	sort.Strings(names)

	for _, name := range names {
		version, err := strconv.Atoi(strings.SplitN(name, "_", 2)[0])
		if err != nil {
			return fmt.Errorf("migration %q: name must start with a version number", name)
		}
		if applied[version] {
			continue
		}
		body, err := migrationFS.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		err = db.Write(ctx, func(q Queryer) error {
			if _, err := q.ExecContext(ctx, string(body)); err != nil {
				return fmt.Errorf("apply %s: %w", name, err)
			}
			_, err := q.ExecContext(ctx,
				`INSERT INTO schema_migration (version, applied_at) VALUES (?, ?)`,
				version, clock.Format(time.Now()))
			return err
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// --- scanning helpers -------------------------------------------------------

func parseTime(s string) time.Time {
	t, err := clock.Parse(s)
	if err != nil {
		return time.Time{}
	}
	return t
}

func optTime(ns sql.NullString) *time.Time {
	if !ns.Valid || ns.String == "" {
		return nil
	}
	t := parseTime(ns.String)
	return &t
}

// storeTime renders an optional timestamp for a nullable column.
func storeTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return clock.Format(*t)
}
