package stats

import (
	"time"

	"github.com/a-narquel/gh-snitch/internal/github"
)

// Stats for `gh snitch me`. The days come from github.Profile.Days: one entry
// per calendar day, oldest first, Date at local midnight, Count may be 0.
// The tests in profile_test.go say exactly what each function must return:
//
//	go test ./internal/stats -run Profile

// BestDay returns the day with the most contributions (the earliest one on a
// tie), or a zero Day if days is empty.
func BestDay(days []github.Day) github.Day {
	// TODO: a plain loop that remembers the best so far. Use > (not >=) so
	// the earliest day wins a tie, since days are oldest first.
	return github.Day{}
}

// Weekdays adds up the contributions per day of the week.
// The result is indexed by time.Weekday: [0] is Sunday, [1] Monday … [6] Saturday.
func Weekdays(days []github.Day) [7]int {
	// TODO: d.Date.Weekday() returns a time.Weekday, which is just an int
	// underneath, so it can be used directly as an array index.
	// (An array [7]int is a value type, unlike a slice: returning it copies it.)
	return [7]int{}
}

// Streaks returns the current and the longest run of consecutive days with
// at least one contribution.
//
// Like GitHub, the current streak isn't broken just because today is still
// empty: if today has 0, it counts back from yesterday instead.
// Days after today (if any) are ignored.
func Streaks(days []github.Day, today time.Time) (current, longest int) {
	// TODO: two separate parts.
	//
	// longest: walk forward with a counter; +1 on a day with Count > 0,
	// back to 0 otherwise; keep the maximum seen.
	//
	// current: walk backwards from the end (for i := len(days) - 1; i >= 0; i--).
	//   - skip days after today: compare with d.Date.After(today)
	//   - the first day you reach that is today itself may be empty: skip it
	//     (to check "same day", compare d.Date.Format(time.DateOnly) for both)
	//   - otherwise count until the first day with Count == 0
	return 0, 0
}

// TopLanguages returns the n biggest languages by bytes, biggest first, with
// the rest merged into one last entry called "other" (only if there is a rest).
// Pct is the share of all bytes, 0..100.
func TopLanguages(langs []github.Language, n int) []Share {
	// TODO:
	//  1. build a map[string]int from name to bytes and pass it to toShares
	//     (members.go). It already sorts biggest first and fills Pct.
	//  2. if there are more than n, keep shares[:n] and add one Share for
	//     "other" with the sum of the Value and Pct of everything after n.
	return nil
}
