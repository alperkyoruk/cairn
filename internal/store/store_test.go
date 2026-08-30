package store

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alperkyoruk/cairn/internal/model"
	"github.com/alperkyoruk/cairn/internal/workflow"
)

func open(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "cairn.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestPragmasAreApplied(t *testing.T) {
	db := open(t)
	ctx := context.Background()
	for _, tc := range []struct{ pragma, want string }{
		{"journal_mode", "wal"},
		{"foreign_keys", "1"},
		{"busy_timeout", "5000"},
	} {
		var got string
		if err := db.w.QueryRowContext(ctx, "PRAGMA "+tc.pragma).Scan(&got); err != nil {
			t.Fatalf("PRAGMA %s: %v", tc.pragma, err)
		}
		if !strings.EqualFold(got, tc.want) {
			t.Errorf("PRAGMA %s = %q, want %q", tc.pragma, got, tc.want)
		}
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	db := open(t)
	for i := 0; i < 3; i++ {
		if err := db.Migrate(context.Background()); err != nil {
			t.Fatalf("re-running migrations: %v", err)
		}
	}
	var n int
	if err := db.w.QueryRow(`SELECT count(*) FROM schema_migration`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("applied %d migrations, want 1", n)
	}
}

// The status list in internal/workflow and the CHECK constraint in the schema
// are two statements of the same fact. This is what keeps them one fact.
func TestStatusesMatchSchema(t *testing.T) {
	sql, err := os.ReadFile("migrations/001_init.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range workflow.Statuses() {
		if !strings.Contains(string(sql), "'"+string(s)+"'") {
			t.Errorf("status %q is missing from the CHECK constraint in the schema", s)
		}
	}
	// And nothing extra: count the quoted strings on the CHECK line.
	for _, line := range strings.Split(string(sql), "\n") {
		if !strings.Contains(line, "status IN (") {
			continue
		}
		if got, want := strings.Count(line, "'"), len(workflow.Statuses())*2; got != want {
			t.Errorf("schema CHECK lists %d statuses, workflow.Statuses() has %d",
				got/2, len(workflow.Statuses()))
		}
	}
}

func TestOnlyOneHumanIsPossible(t *testing.T) {
	db := open(t)
	ctx := context.Background()
	mk := func(id, name string, typ workflow.ActorType) error {
		return db.Write(ctx, func(q Queryer) error {
			hash := ""
			if typ == workflow.Human {
				hash = "argon2id$..."
			}
			return InsertActor(ctx, q, model.Actor{
				ID: id, Type: typ, Name: name, CreatedAt: time.Now(),
			}, hash)
		})
	}
	if err := mk("a1", "alper", workflow.Human); err != nil {
		t.Fatalf("first human rejected: %v", err)
	}
	if err := mk("a2", "claude", workflow.Agent); err != nil {
		t.Fatalf("agent rejected: %v", err)
	}
	if err := mk("a3", "codex", workflow.Agent); err != nil {
		t.Fatalf("second agent rejected: %v", err)
	}
	if err := mk("a4", "someone", workflow.Human); err == nil {
		t.Error("database accepted a second human")
	}
}

func TestAgentsCannotHoldAPassword(t *testing.T) {
	db := open(t)
	ctx := context.Background()
	err := db.Write(ctx, func(q Queryer) error {
		return InsertActor(ctx, q, model.Actor{
			ID: "a1", Type: workflow.Agent, Name: "claude", CreatedAt: time.Now(),
		}, "argon2id$...")
	})
	if err == nil {
		t.Error("database accepted a password hash on an agent")
	}
}

func TestDeletingATaskTakesItsRecordWithIt(t *testing.T) {
	db := open(t)
	ctx := context.Background()
	now := time.Now()

	err := db.Write(ctx, func(q Queryer) error {
		if err := InsertActor(ctx, q, model.Actor{ID: "a1", Type: workflow.Agent, Name: "claude", CreatedAt: now}, ""); err != nil {
			return err
		}
		if err := InsertProject(ctx, q, model.Project{ID: "p1", Slug: "cairn", Name: "Cairn", CreatedAt: now}); err != nil {
			return err
		}
		if err := InsertTask(ctx, q, model.Task{
			ID: "t1", ProjectID: "p1", Number: 1, Title: "first", Status: workflow.Backlog,
			CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			return err
		}
		if err := UpsertState(ctx, q, model.State{
			TaskID: "t1", WhereILeftOff: "here", NextStep: "there", UpdatedBy: "a1", UpdatedAt: now,
		}); err != nil {
			return err
		}
		return InsertWorklog(ctx, q, model.WorklogEntry{
			ID: "w1", TaskID: "t1", ActorID: "a1", CreatedAt: now, WhatWasTried: "x", Outcome: "y",
		})
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := db.Write(ctx, func(q Queryer) error { return DeleteTask(ctx, q, "t1") }); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"task", "task_state", "worklog"} {
		var n int
		if err := db.r.QueryRow(`SELECT count(*) FROM ` + table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Errorf("%s still holds %d rows after the task was deleted", table, n)
		}
	}
}

func TestTaskNumbersAreSequentialPerProject(t *testing.T) {
	db := open(t)
	ctx := context.Background()
	now := time.Now()

	err := db.Write(ctx, func(q Queryer) error {
		if err := InsertProject(ctx, q, model.Project{ID: "p1", Slug: "cairn", Name: "Cairn", CreatedAt: now}); err != nil {
			return err
		}
		return InsertProject(ctx, q, model.Project{ID: "p2", Slug: "other", Name: "Other", CreatedAt: now})
	})
	if err != nil {
		t.Fatal(err)
	}

	for i := 1; i <= 3; i++ {
		err := db.Write(ctx, func(q Queryer) error {
			n, err := NextTaskNumber(ctx, q, "p1")
			if err != nil {
				return err
			}
			if n != i {
				t.Errorf("task number = %d, want %d", n, i)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	// A second project starts its own count.
	err = db.Write(ctx, func(q Queryer) error {
		n, err := NextTaskNumber(ctx, q, "p2")
		if n != 1 {
			t.Errorf("second project started at %d, want 1", n)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}
