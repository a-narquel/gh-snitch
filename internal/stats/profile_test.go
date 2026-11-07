package stats

import (
	"math"
	"testing"
	"time"

	"github.com/a-narquel/gh-snitch/internal/github"
)

// calendar makes consecutive days starting at start, one per count.
func calendar(start time.Time, counts ...int) []github.Day {
	days := make([]github.Day, len(counts))
	for i, n := range counts {
		days[i] = github.Day{Date: start.AddDate(0, 0, i), Count: n}
	}
	return days
}

func TestProfileBestDay(t *testing.T) {
	start := at(2026, 11, 2, 0) // a Monday
	got := BestDay(calendar(start, 1, 7, 3, 7))
	if got.Count != 7 || !got.Date.Equal(start.AddDate(0, 0, 1)) {
		t.Errorf("BestDay = %v, want 7 on Nov 3 (the earliest of the tie)", got)
	}
	if got := BestDay(nil); got.Count != 0 {
		t.Errorf("BestDay(nil) = %v, want a zero Day", got)
	}
}

func TestProfileWeekdays(t *testing.T) {
	// Monday Nov 2 → Monday Nov 9: two Mondays.
	got := Weekdays(calendar(at(2026, 11, 2, 0), 1, 2, 0, 0, 0, 5, 6, 10))
	want := [7]int{time.Monday: 11, time.Tuesday: 2, time.Saturday: 5, time.Sunday: 6}
	if got != want { // arrays (unlike slices) can be compared with ==
		t.Errorf("Weekdays = %v, want %v", got, want)
	}
}

func TestProfileStreaks(t *testing.T) {
	start := at(2026, 11, 1, 0)
	tests := []struct {
		name             string
		counts           []int
		today            int // index of today in counts
		current, longest int
	}{
		{"active today", []int{1, 1, 0, 2, 3, 1}, 5, 3, 3},
		{"today still empty", []int{1, 1, 0, 2, 3, 0}, 5, 2, 2},
		{"broken yesterday", []int{1, 1, 1, 1, 0, 0}, 5, 0, 4},
		{"future days ignored", []int{0, 4, 4, 0, 0, 0}, 2, 2, 2},
		{"nothing at all", []int{0, 0, 0}, 2, 0, 0},
		{"no days", nil, 0, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			today := start.AddDate(0, 0, tt.today).Add(15 * time.Hour) // 15:00, not midnight
			cur, long := Streaks(calendar(start, tt.counts...), today)
			if cur != tt.current || long != tt.longest {
				t.Errorf("Streaks = %d, %d; want %d, %d", cur, long, tt.current, tt.longest)
			}
		})
	}
}

func TestProfileTopLanguages(t *testing.T) {
	langs := []github.Language{{Name: "Go", Bytes: 500}, {Name: "Java", Bytes: 300}, {Name: "Shell", Bytes: 100}, {Name: "Makefile", Bytes: 60}, {Name: "CSS", Bytes: 40}}

	got := TopLanguages(langs, 2)
	want := []Share{{"Go", 500, 50}, {"Java", 300, 30}, {"other", 200, 20}}
	if len(got) != len(want) {
		t.Fatalf("TopLanguages = %v, want %v", got, want)
	}
	for i := range want {
		if got[i].Name != want[i].Name || got[i].Value != want[i].Value || math.Abs(got[i].Pct-want[i].Pct) > 0.01 {
			t.Errorf("TopLanguages[%d] = %v, want %v", i, got[i], want[i])
		}
	}

	if got := TopLanguages(langs, 10); len(got) != 5 {
		t.Errorf("with n=10: %d entries, want 5 and no \"other\"", len(got))
	}
}
