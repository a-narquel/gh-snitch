package cli

import (
	"fmt"

	"github.com/a-narquel/gh-snitch/internal/render"
	"github.com/a-narquel/gh-snitch/internal/stats"
)

// maxPoints is the most buckets the activity chart shows before it keeps
// only the most recent ones.
const maxPoints = 72

// activity: commits per week of the top 5 contributors (per month for long histories).
func cmdActivity(s *Session, args []string) error {
	v := s.view()
	members := stats.Members(v)
	if len(members) == 0 {
		return errNoCommits
	}

	tl := stats.Activity(v, stats.Week)
	if len(tl.Starts) > maxPoints {
		tl = stats.Activity(v, stats.Month) // long history: months are easier to read
	}

	var who []string
	for _, m := range members[:min(5, len(members))] {
		who = append(who, m.Name)
	}

	from := 0
	if len(tl.Starts) > maxPoints {
		from = len(tl.Starts) - maxPoints
	}

	var series []render.Series
	for _, name := range who {
		vals := tl.By[name]
		if vals == nil {
			vals = make([]int, len(tl.Starts)) // no commits in this window: a flat line
		}
		series = append(series, render.Series{Name: s.name(name), Values: vals[from:]})
	}

	labels := make([]string, 0, len(tl.Starts)-from)
	for _, start := range tl.Starts[from:] {
		if tl.Gran == stats.Month {
			labels = append(labels, start.Format("Jan 2006"))
		} else {
			labels = append(labels, render.Date(start))
		}
	}

	// Git tags in the window become ▲ markers (only the last 3, to keep it tidy).
	var markers []render.Marker
	for _, tag := range s.tags {
		if i := tl.Index(tag.When); i >= from {
			markers = append(markers, render.Marker{Index: i - from, Label: tag.Name})
		}
	}
	markers = markers[max(0, len(markers)-3):]

	fmt.Fprintf(s.out, " %s\n", render.Bold("commits/"+tl.Gran.String()))
	render.LineChart(s.out, series, labels, markers)
	if from > 0 {
		fmt.Fprintln(s.out, render.Dim(fmt.Sprintf(" (last %d %ss only; pick a shorter period in Filters)", maxPoints, tl.Gran)))
	}
	return nil
}

// awards: the fun stuff.
func cmdAwards(s *Session, args []string) error {
	awards := stats.Awards(s.view(), s.now())
	if len(awards) == 0 {
		return errNoCommits
	}
	titleW, nameW := 0, 0
	for _, a := range awards {
		titleW = max(titleW, render.Width(a.Title))
		nameW = max(nameW, render.Width(s.name(a.Winner)))
	}
	for _, a := range awards {
		fmt.Fprintf(s.out, " %s  %s  %s  %s\n", render.Pad(a.Emoji, 2), render.Pad(a.Title, titleW),
			render.Bold(render.Pad(s.name(a.Winner), nameW)), render.Dim(a.Detail))
	}
	return nil
}
