package id

import (
	"regexp"
	"sort"
	"sync"
	"testing"
	"time"
)

var uuidV7 = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestNewIsWellFormedAndUnique(t *testing.T) {
	seen := make(map[string]bool, 1000)
	for i := 0; i < 1000; i++ {
		v := New()
		if !uuidV7.MatchString(v) {
			t.Fatalf("not a well-formed uuidv7: %q", v)
		}
		if seen[v] {
			t.Fatalf("duplicate id generated: %q", v)
		}
		seen[v] = true
	}
}

// Ids must sort by creation time, since that is why we chose v7 over v4.
func TestNewSortsChronologically(t *testing.T) {
	var got []string
	for i := 0; i < 5; i++ {
		got = append(got, New())
		time.Sleep(2 * time.Millisecond)
	}
	assertSorted(t, got)
}

// And they must still sort when generated faster than the clock ticks, which
// is the case the board's tiebreak depends on: several tasks touched inside
// one millisecond have to come back in a stable, correct order every time.
func TestNewIsMonotonicWithinAMillisecond(t *testing.T) {
	got := make([]string, 0, 5000)
	for i := 0; i < 5000; i++ {
		got = append(got, New())
	}
	assertSorted(t, got)
}

func TestNewIsMonotonicUnderConcurrency(t *testing.T) {
	const workers, each = 8, 500
	results := make(chan []string, workers)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out := make([]string, 0, each)
			for i := 0; i < each; i++ {
				out = append(out, New())
			}
			results <- out
		}()
	}
	wg.Wait()
	close(results)

	seen := make(map[string]bool, workers*each)
	for batch := range results {
		// Each goroutine's own ids must still be in order relative to itself.
		assertSorted(t, batch)
		for _, v := range batch {
			if seen[v] {
				t.Fatalf("duplicate id under concurrency: %q", v)
			}
			seen[v] = true
		}
	}
	if len(seen) != workers*each {
		t.Errorf("got %d distinct ids, want %d", len(seen), workers*each)
	}
}

func assertSorted(t *testing.T, got []string) {
	t.Helper()
	want := append([]string(nil), got...)
	sort.Strings(want)
	for i := range want {
		if want[i] != got[i] {
			t.Fatalf("ids do not sort chronologically at index %d: got %s, want %s",
				i, got[i], want[i])
		}
	}
}
