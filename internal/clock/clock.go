// Package clock centralises how Cairn produces and formats time.
//
// Every timestamp in the database is written by the application, never by the
// database: SQLite has no native date type, so timestamps are stored as text in
// a fixed format that sorts lexicographically in the same order it sorts
// chronologically. Keeping the formatting in one place is what keeps that true.
package clock

import "time"

// Layout is RFC3339 with fixed millisecond precision, always UTC. The fixed
// width matters: a variable number of fractional digits would break ordering.
const Layout = "2006-01-02T15:04:05.000Z"

// Clock reports the current time. Tests substitute a fixed one.
type Clock func() time.Time

// System is the real clock.
func System() time.Time { return time.Now() }

// Format renders t for storage.
func Format(t time.Time) string { return t.UTC().Format(Layout) }

// Parse reads a stored timestamp back.
func Parse(s string) (time.Time, error) { return time.Parse(Layout, s) }

// Now is shorthand for formatting a clock's current time.
func Now(c Clock) string { return Format(c()) }
