// Package stats turns raw history (commits, blame, PRs) into numbers.
//
// Everything here is pure: data in, numbers out, no I/O and no printing.
// That keeps it easy to test and lets render/cli decide how things look.
package stats

import (
	"time"

	"github.com/a-narquel/gh-snitch/internal/github"
	"github.com/a-narquel/gh-snitch/internal/gitlog"
)

// Data is everything snitch knows about the repository.
// A filtered view (see Filter.Apply) has the same type.
type Data struct {
	Commits []gitlog.Commit    // newest first, like git log
	Blame   []gitlog.FileBlame // current code at HEAD
	PRs     []github.PR
	HasPRs  bool              // false when GitHub data could not be loaded
	Names   map[string]string // GitHub login → git author name
}

// NameOf returns the git author name for a GitHub login, or the login itself
// when we couldn't link it to a commit author.
func (d Data) NameOf(login string) string {
	if name, ok := d.Names[login]; ok {
		return name
	}
	return login
}

// Span returns the first and last commit times.
func (d Data) Span() (first, last time.Time) {
	for _, c := range d.Commits {
		if first.IsZero() || c.When.Before(first) {
			first = c.When
		}
		if c.When.After(last) {
			last = c.When
		}
	}
	return first, last
}

// Days counts calendar days from first to last, both included.
func Days(first, last time.Time) int {
	if first.IsZero() {
		return 0
	}
	return int(dayOf(last).Sub(dayOf(first)).Hours()/24+0.5) + 1
}

// dayOf truncates t to midnight in the local time zone.
func dayOf(t time.Time) time.Time {
	t = t.Local()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
}

// isNight reports whether a commit was made between 22h and 06h in the
// author's own time zone (git keeps the author's offset in the date).
func isNight(t time.Time) bool {
	h := t.Hour()
	return h >= 22 || h < 6
}

func isWeekend(t time.Time) bool {
	return t.Weekday() == time.Saturday || t.Weekday() == time.Sunday
}

// Share is a named slice of a whole, used for "where they work", commit types…
type Share struct {
	Name  string
	Value int
	Pct   float64 // 0..100
}

// pct returns part/whole as a percentage, or 0 when whole is 0.
func pct(part, whole int) float64 {
	if whole == 0 {
		return 0
	}
	return 100 * float64(part) / float64(whole)
}
