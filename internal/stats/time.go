package stats

import "time"

// Granularity is the size of each bucket on a timeline.
type Granularity int

// iota counts up from 0 inside a const block: Day=0, Week=1, Month=2.
// It's Go's lightweight version of a Java enum.
const (
	Day Granularity = iota
	Week
	Month
)

func (g Granularity) String() string {
	switch g {
	case Day:
		return "day"
	case Week:
		return "week"
	default:
		return "month"
	}
}

// bucketStart returns the start of the bucket that t falls into.
func (g Granularity) bucketStart(t time.Time) time.Time {
	d := dayOf(t)
	switch g {
	case Day:
		return d
	case Week:
		// Weeks start on Monday. Weekday() is 0 for Sunday, so shift by 6.
		return d.AddDate(0, 0, -((int(d.Weekday()) + 6) % 7))
	default:
		return time.Date(d.Year(), d.Month(), 1, 0, 0, 0, 0, time.Local)
	}
}

func (g Granularity) next(t time.Time) time.Time {
	switch g {
	case Day:
		return t.AddDate(0, 0, 1)
	case Week:
		return t.AddDate(0, 0, 7)
	default:
		return t.AddDate(0, 1, 0)
	}
}

// Timeline counts commits per bucket, for everyone and per member.
type Timeline struct {
	Gran   Granularity
	Starts []time.Time      // start of each bucket, oldest first
	Total  []int            // commits per bucket, everyone
	By     map[string][]int // commits per bucket, per member
}

// Index returns the bucket that t falls into, or -1 if it's outside.
func (tl Timeline) Index(t time.Time) int {
	if len(tl.Starts) == 0 {
		return -1
	}
	start := tl.Gran.bucketStart(t)
	for i, s := range tl.Starts {
		if s.Equal(start) {
			return i
		}
	}
	return -1
}

// Activity builds a timeline from the first to the last commit.
// Empty buckets in between are kept as zeros, so gaps show up on the chart.
func Activity(d Data, g Granularity) Timeline {
	tl := Timeline{Gran: g, By: map[string][]int{}}
	first, last := d.Span()
	if first.IsZero() {
		return tl
	}

	index := map[time.Time]int{}
	for s := g.bucketStart(first); !s.After(last); s = g.next(s) {
		index[s] = len(tl.Starts)
		tl.Starts = append(tl.Starts, s)
	}
	tl.Total = make([]int, len(tl.Starts))

	for _, c := range d.Commits {
		i := index[g.bucketStart(c.When)]
		tl.Total[i]++
		if tl.By[c.Author] == nil {
			tl.By[c.Author] = make([]int, len(tl.Starts))
		}
		tl.By[c.Author][i]++
	}
	return tl
}

// PunchCard counts commits by weekday (rows, Monday first) and 3-hour slot
// of the day (columns: 00, 03, 06, … 21), in each author's own time zone.
// If member is "", everyone's commits are counted.
func PunchCard(d Data, member string) [7][8]int {
	var card [7][8]int // arrays have a fixed size, so this is all zeros already
	for _, c := range d.Commits {
		if member != "" && c.Author != member {
			continue
		}
		row := (int(c.When.Weekday()) + 6) % 7 // Monday = 0 … Sunday = 6
		card[row][c.When.Hour()/3]++
	}
	return card
}
