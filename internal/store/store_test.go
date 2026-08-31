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
	files, err := os.ReadDir("migrations")
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.w.QueryRow(`SELECT count(*) FROM schema_migration`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != len(files) {
		t.Errorf("applied %d migrations, want %d", n, len(files))
	}
}

// A token's stored prefix is what lets the interface tell two of an agent's
// tokens apart. It must survive the round trip, and it must never be the
// whole secret.
func TestTokenPrefixRoundTrips(t *testing.T) {
	db := open(t)
	ctx := context.Background()
	now := time.Now()

	err := db.Write(ctx, func(q Queryer) error {
		if err := InsertActor(ctx, q, model.Actor{
			ID: "a1", Type: workflow.Agent, Name: "claude", CreatedAt: now,
		}, ""); err != nil {
			return err
		}
		return InsertToken(ctx, q, model.Token{
			ID: "t1", ActorID: "a1", Name: "initial token",
			Prefix: "cairn_7fJqK2", CreatedAt: now,
		}, "a-hash")
	})
	if err != nil {
		t.Fatal(err)
	}

	tokens, err := ListTokens(ctx, db.Read(), "a1")
	if err != nil || len(tokens) != 1 {
		t.Fatalf("ListTokens = %v, %v", tokens, err)
	}
	if tokens[0].Prefix != "cairn_7fJqK2" {
		t.Errorf("prefix = %q", tokens[0].Prefix)
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

// Finishing a task is a touch, so ordering by recency alone would put freshly
// completed work above the thing you are stuck on. Done sinks instead --
// without hiding anything, and without a filter.
func TestBoardSinksDoneBelowOpenWork(t *testing.T) {
	db := open(t)
	ctx := context.Background()
	base := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)

	err := db.Write(ctx, func(q Queryer) error {
		if err := InsertProject(ctx, q, model.Project{ID: "p1", Slug: "cairn", Name: "Cairn", CreatedAt: base}); err != nil {
			return err
		}
		// The done task is the most recently touched of the three.
		for i, tc := range []struct {
			id     string
			status workflow.Status
			offset time.Duration
		}{
			{"t1", workflow.Done, 2 * time.Hour},
			{"t2", workflow.Blocked, 1 * time.Hour},
			{"t3", workflow.Queue, 0},
		} {
			at := base.Add(tc.offset)
			if err := InsertTask(ctx, q, model.Task{
				ID: tc.id, ProjectID: "p1", Number: i + 1, Title: tc.id,
				Status: tc.status, CreatedAt: base, UpdatedAt: at,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	rows, err := Board(ctx, db.Read(), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	for _, r := range rows {
		order = append(order, r.Task.ID+":"+string(r.Task.Status))
	}
	want := "t2:blocked t3:queue t1:done"
	if got := strings.Join(order, " "); got != want {
		t.Errorf("board order = %q, want %q", got, want)
	}

	// The same rule applies inside a project.
	tasks, err := ListTasksByProject(ctx, db.Read(), "p1", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 3 || tasks[2].Status != workflow.Done {
		t.Errorf("project list does not sink done: %v", tasks)
	}
}

// The filter has to be a WHERE on the board and an AND inside a project. Get
// that wrong and the board query silently widens its LEFT JOIN instead of
// filtering, which returns every row and looks like it worked.
func TestBoardAndProjectFiltersAndLimits(t *testing.T) {
	db := open(t)
	ctx := context.Background()
	base := time.Date(2026, 8, 31, 9, 0, 0, 0, time.UTC)

	err := db.Write(ctx, func(q Queryer) error {
		if err := InsertActor(ctx, q, model.Actor{ID: "a1", Type: workflow.Agent, Name: "claude", CreatedAt: base}, ""); err != nil {
			return err
		}
		if err := InsertProject(ctx, q, model.Project{ID: "p1", Slug: "cairn", Name: "Cairn", CreatedAt: base}); err != nil {
			return err
		}
		if err := InsertProject(ctx, q, model.Project{ID: "p2", Slug: "other", Name: "Other", CreatedAt: base}); err != nil {
			return err
		}
		statuses := []workflow.Status{workflow.Backlog, workflow.Queue, workflow.Active, workflow.Review, workflow.Done}
		for i, st := range statuses {
			if err := InsertTask(ctx, q, model.Task{
				ID: "t" + string(rune('1'+i)), ProjectID: "p1", Number: i + 1, Title: string(st),
				Status: st, CreatedAt: base, UpdatedAt: base.Add(time.Duration(i) * time.Minute),
			}); err != nil {
				return err
			}
		}
		// A task in another project, to prove the board filter is not
		// accidentally joining rather than filtering.
		return InsertTask(ctx, q, model.Task{
			ID: "x1", ProjectID: "p2", Number: 1, Title: "elsewhere",
			Status: workflow.Active, CreatedAt: base, UpdatedAt: base,
		})
	})
	if err != nil {
		t.Fatal(err)
	}

	all, err := Board(ctx, db.Read(), nil, 0)
	if err != nil || len(all) != 6 {
		t.Fatalf("unfiltered board = %d rows, %v; want 6", len(all), err)
	}

	open, err := Board(ctx, db.Read(), []workflow.Status{workflow.Active, workflow.Review}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(open) != 3 {
		t.Errorf("active+review board = %d rows, want 3", len(open))
	}
	for _, r := range open {
		if r.Task.Status != workflow.Active && r.Task.Status != workflow.Review {
			t.Errorf("filter let %s through", r.Task.Status)
		}
	}

	capped, err := Board(ctx, db.Read(), nil, 2)
	if err != nil || len(capped) != 2 {
		t.Fatalf("limited board = %d rows, %v; want 2", len(capped), err)
	}

	inProject, err := ListTasksByProject(ctx, db.Read(), "p1", []workflow.Status{workflow.Done}, 0)
	if err != nil || len(inProject) != 1 || inProject[0].Status != workflow.Done {
		t.Fatalf("project filter = %v, %v", inProject, err)
	}
}

// A truncated worklog must still read forwards, and must say what it left out.
func TestWorklogLimitReturnsTheNewestInOrder(t *testing.T) {
	db := open(t)
	ctx := context.Background()
	base := time.Date(2026, 8, 31, 9, 0, 0, 0, time.UTC)

	err := db.Write(ctx, func(q Queryer) error {
		if err := InsertActor(ctx, q, model.Actor{ID: "a1", Type: workflow.Agent, Name: "claude", CreatedAt: base}, ""); err != nil {
			return err
		}
		if err := InsertProject(ctx, q, model.Project{ID: "p1", Slug: "cairn", Name: "Cairn", CreatedAt: base}); err != nil {
			return err
		}
		if err := InsertTask(ctx, q, model.Task{
			ID: "t1", ProjectID: "p1", Number: 1, Title: "x", Status: workflow.Active,
			CreatedAt: base, UpdatedAt: base,
		}); err != nil {
			return err
		}
		for i := 0; i < 8; i++ {
			if err := InsertWorklog(ctx, q, model.WorklogEntry{
				ID: "w" + string(rune('1'+i)), TaskID: "t1", ActorID: "a1",
				CreatedAt:    base.Add(time.Duration(i) * time.Hour),
				WhatWasTried: string(rune('1' + i)),
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	total, err := CountWorklog(ctx, db.Read(), "t1")
	if err != nil || total != 8 {
		t.Fatalf("CountWorklog = %d, %v; want 8", total, err)
	}

	last3, err := ListWorklog(ctx, db.Read(), "t1", 3)
	if err != nil {
		t.Fatal(err)
	}
	var got string
	for _, e := range last3 {
		got += e.WhatWasTried
	}
	if got != "678" {
		t.Errorf("last three entries read %q, want \"678\" (newest, oldest-first)", got)
	}

	full, err := ListWorklog(ctx, db.Read(), "t1", 0)
	if err != nil || len(full) != 8 || full[0].WhatWasTried != "1" {
		t.Errorf("unlimited worklog = %d entries starting %q", len(full), full[0].WhatWasTried)
	}
}
