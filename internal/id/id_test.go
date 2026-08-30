package id

import (
	"regexp"
	"sort"
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
	want := append([]string(nil), got...)
	sort.Strings(want)
	for i := range want {
		if want[i] != got[i] {
			t.Fatalf("ids do not sort chronologically:\n got %v\nwant %v", got, want)
		}
	}
}
