package cli

import (
	"errors"
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/a-narquel/gh-snitch/internal/render"
	"github.com/a-narquel/gh-snitch/internal/stats"
)

// maxPoints is the most buckets the activity chart shows before it keeps
// only the most recent ones.
const maxPoints = 72

// activity [--daily|--weekly|--monthly] [member…]: commits over time.
func cmdActivity(s *Session, args []string) error {
	gran, chosen := stats.Week, false
	var who []string
	for _, a := range args {
		switch a {
		case "--daily", "-d":
			gran, chosen = stats.Day, true
		case "--weekly", "-w":
			gran, chosen = stats.Week, true
		case "--monthly", "-m":
			gran, chosen = stats.Month, true
		default:
			name, err := s.findMember(a)
			if err != nil {
				return err
			}
			who = append(who, name)
		}
	}

	v := s.view()
	members := stats.Members(v)
	if len(members) == 0 {
		return errNoCommits
	}

	tl := stats.Activity(v, gran)
	if !chosen && len(tl.Starts) > maxPoints {
		tl = stats.Activity(v, stats.Month) // long history: switch to months on our own
	}

	// Without names, show the top 5 contributors.
	if len(who) == 0 {
		for _, m := range members[:min(5, len(members))] {
			who = append(who, m.Name)
		}
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
		fmt.Fprintln(s.out, render.Dim(fmt.Sprintf(" (last %d %ss only; use 'since' to pick a window)", maxPoints, tl.Gran)))
	}
	return nil
}

// when [member]: the punch card.
func cmdWhen(s *Session, args []string) error {
	member := ""
	if len(args) > 0 {
		name, err := s.findMember(strings.Join(args, " "))
		if err != nil {
			return err
		}
		member = name
	}

	v := s.view()
	card := stats.PunchCard(v, member)

	top, busiestRow, busiestCol := 0, 0, 0
	dayTotals := [7]int{}
	for r := range card {
		for c, n := range card[r] {
			dayTotals[r] += n
			if n > top {
				top, busiestRow, busiestCol = n, r, c
			}
		}
	}
	if top == 0 {
		return errNoCommits
	}

	// level maps a count to 1..5, relative to the busiest slot.
	level := func(n int) int {
		if n == 0 {
			return 0
		}
		return int(math.Ceil(float64(n) / float64(top) * render.HeatLevels))
	}

	// One annotation: a Sunday-night peak gets the special one.
	note := map[int]string{}
	if busiestRow == 6 && (busiestCol == 7 || busiestCol == 0) {
		note[6] = "← Sunday-night panic 😅"
	} else {
		busiestDay := 0
		for r, n := range dayTotals {
			if n > dayTotals[busiestDay] {
				busiestDay = r
			}
		}
		note[busiestDay] = "← busiest day"
	}

	if member != "" {
		fmt.Fprintf(s.out, " %s\n\n", render.Bold(s.name(member)))
	}
	fmt.Fprintln(s.out, render.Dim("       00  03  06  09  12  15  18  21"))
	days := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
	for r, day := range days {
		var b strings.Builder
		for _, n := range card[r] {
			b.WriteString(render.Pad(render.Tint(0, render.Heat(level(n))), 4))
		}
		line := fmt.Sprintf(" %s   %s", day, b.String())
		if n, ok := note[r]; ok {
			line = render.Pad(line, 39) + render.Yellow(n)
		}
		fmt.Fprintln(s.out, strings.TrimRight(line, " "))
	}

	// Legend: which counts each character stands for.
	var legend []string
	for l := 1; l <= render.HeatLevels; l++ {
		lo := int(math.Floor(float64(l-1)*float64(top)/render.HeatLevels)) + 1
		hi := int(math.Floor(float64(l) * float64(top) / render.HeatLevels))
		if lo > hi {
			continue // this level can't happen with so few commits
		}
		rng := strconv.Itoa(lo)
		if hi > lo {
			rng += "-" + strconv.Itoa(hi)
		}
		legend = append(legend, render.Tint(0, render.Heat(l))+" "+rng)
	}
	fmt.Fprintf(s.out, "\n %s\n", strings.Join(legend, "   "))
	fmt.Fprintln(s.out, render.Dim(" (hours in each author's own time zone)"))
	return nil
}

// files [--top N]: hotspots.
func cmdFiles(s *Session, args []string) error {
	n := 10
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--top" || a == "-n" {
			if i+1 >= len(args) {
				return errors.New("usage: files [--top N]")
			}
			i++
			a = args[i]
		}
		var err error
		if n, err = strconv.Atoi(a); err != nil || n <= 0 {
			return fmt.Errorf("%q isn't a number of files", a)
		}
	}

	files := stats.Files(s.view())
	if len(files) == 0 {
		return errNoCommits
	}
	files = files[:min(n, len(files))]

	t := render.Table{
		Headers: []string{"FILE", "CHANGES", "CHURN", "AUTHORS", "OWNER"},
		Align:   []render.Align{render.Left, render.Right, render.Right, render.Right, render.Left},
	}
	ownerW := 0
	for _, f := range files {
		ownerW = max(ownerW, render.Width(s.name(f.Owner)))
	}
	for _, f := range files {
		t.Rows = append(t.Rows, []string{
			render.TruncLeft(f.Path, 44), render.Int(f.Changes), render.Int(f.Churn),
			strconv.Itoa(f.Authors), render.Pad(s.name(f.Owner), ownerW) + " " + render.Pct(f.OwnerPct),
		})
	}
	t.Render(s.out)
	return nil
}

// reviews: who reviews whom.
func cmdReviews(s *Session, args []string) error {
	v := s.view()
	if !v.HasPRs {
		return errors.New(s.ghNote)
	}
	if len(v.PRs) == 0 {
		if s.filter.IsZero() {
			return errors.New("this repository has no pull requests yet")
		}
		return errors.New("no pull requests in the current filters")
	}

	rs := stats.Reviews(v)
	if len(rs.People) == 0 {
		fmt.Fprintln(s.out, " Nobody reviewed anyone's PRs yet.")
	}
	people := rs.People[:min(8, len(rs.People))]

	t := render.Table{Headers: []string{"REVIEWER ↓"}, Align: []render.Align{render.Left}}
	for _, p := range people {
		t.Headers = append(t.Headers, s.name(p))
		t.Align = append(t.Align, render.Right)
	}
	for _, reviewer := range people {
		row := []string{s.name(reviewer)}
		for _, author := range people {
			switch n := rs.Matrix[reviewer][author]; {
			case reviewer == author:
				row = append(row, render.Dim("-"))
			case n == 0:
				row = append(row, render.Dim("·"))
			default:
				row = append(row, strconv.Itoa(n))
			}
		}
		t.Rows = append(t.Rows, row)
	}

	if len(people) > 0 {
		fmt.Fprintf(s.out, " %s%s\n", strings.Repeat(" ", render.Width("REVIEWER ↓")+2), render.Dim("AUTHOR →"))
		t.Render(s.out)
	}
	if len(rs.People) > len(people) {
		fmt.Fprintln(s.out, render.Dim(fmt.Sprintf(" (+%d more people not shown)", len(rs.People)-len(people))))
	}

	merged, open := 0, 0
	for _, pr := range v.PRs {
		if !pr.MergedAt.IsZero() {
			merged++
		}
		if pr.State == "OPEN" {
			open++
		}
	}
	fmt.Fprintf(s.out, "\n Pull requests:               %d (%d merged, %d open)\n", len(v.PRs), merged, open)
	if rs.HasFirstReview {
		fmt.Fprintf(s.out, " Median time to first review: %s\n", render.Duration(rs.MedianFirstReview))
	}
	if rs.HasMerge {
		fmt.Fprintf(s.out, " Median time to merge:        %s\n", render.Duration(rs.MedianMerge))
	}

	if len(rs.SelfMerged) == 0 {
		fmt.Fprintln(s.out, " Self-merged without review:  nobody 👍")
		return nil
	}
	var parts []string
	for _, p := range slices.Sorted(maps.Keys(rs.SelfMerged)) {
		parts = append(parts, fmt.Sprintf("%s (%s)", s.name(p), render.Plural(rs.SelfMerged[p], "PR")))
	}
	fmt.Fprintf(s.out, " Self-merged without review:  %s %s\n", strings.Join(parts, ", "), render.Yellow("⚠"))
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
