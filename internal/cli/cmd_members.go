package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/a-narquel/gh-snitch/internal/render"
	"github.com/a-narquel/gh-snitch/internal/stats"
)

// who: the leaderboard.
//
//	MEMBER  COMMITS             +LINES  -LINES  SURVIVING      ACTIVE   PRs  REVIEWS
//	andre   198 ████████▋  48%  14,203   5,110  8,940  (62%)  31 days   21       17
func cmdWho(s *Session, args []string) error {
	all := len(args) > 0 && (args[0] == "--all" || args[0] == "-a")
	v := s.view()
	members := stats.Members(v)
	if len(members) == 0 {
		return errNoCommits
	}
	sum := stats.Summarize(v)

	const maxRows = 15
	hidden := 0
	if !all && len(members) > maxRows {
		hidden = len(members) - maxRows
		members = members[:maxRows]
	}
	hasBlame := len(v.Blame) > 0

	// Widths of the number parts, so the bars line up.
	commitW := render.Width(render.Int(sum.Commits))
	survW := render.Width(render.Int(sum.Surviving))
	top := float64(members[0].Commits)

	orDash := func(ok bool, n int) string {
		if !ok {
			return "-"
		}
		return render.Int(n)
	}

	t := render.Table{
		Headers: []string{"MEMBER", "COMMITS", "+LINES", "-LINES", "SURVIVING", "ACTIVE", "PRs", "REVIEWS"},
		Align:   []render.Align{render.Left, render.Left, render.Right, render.Right, render.Left, render.Right, render.Right, render.Right},
	}
	for _, m := range members {
		commits := render.PadLeft(render.Int(m.Commits), commitW) + " " +
			render.Pad(render.Bar(float64(m.Commits), top, 10), 10) + " " +
			render.PadLeft(render.Pct(100*float64(m.Commits)/float64(sum.Commits)), 4)

		surviving := "-"
		if hasBlame {
			surviving = render.PadLeft(render.Int(m.Surviving), survW) + "  " +
				render.PadLeft("("+render.Pct(m.SurvivingPct())+")", 6)
		}

		t.Rows = append(t.Rows, []string{
			s.name(m.Name), commits,
			render.Green(render.Int(m.Added)), render.Red(render.Int(m.Deleted)),
			surviving, render.Plural(m.ActiveDays, "day"),
			orDash(v.HasPRs, m.PRs), orDash(v.HasPRs, m.Reviews),
		})
	}
	if len(members) > 1 {
		survTotal := "-"
		if hasBlame {
			survTotal = render.PadLeft(render.Int(sum.Surviving), survW)
		}
		t.Footer = [][]string{{
			render.Bold("total"), render.PadLeft(render.Int(sum.Commits), commitW),
			render.Int(sum.Added), render.Int(sum.Deleted), survTotal,
			render.Plural(sum.ActiveDays, "day"),
			orDash(v.HasPRs, sum.PRs), orDash(v.HasPRs, sum.Reviews),
		}}
	}
	t.Render(s.out)
	if hidden > 0 {
		fmt.Fprintln(s.out, render.Dim(fmt.Sprintf(" (+%d more with fewer commits: 'who --all')", hidden)))
	}

	if names, share := stats.BusFactor(v); len(names) > 0 {
		short := make([]string, len(names))
		for i, n := range names {
			short[i] = s.name(n)
		}
		verb := "own"
		if len(names) == 1 {
			verb = "owns"
		}
		fmt.Fprintf(s.out, "\n Bus factor: %s  %s\n", render.Bold(fmt.Sprint(len(names))),
			render.Dim(fmt.Sprintf("(%s %s %s of current code)", strings.Join(short, " + "), verb, render.Pct(share))))
	}
	return nil
}

// show <member>: one person's profile.
func cmdShow(s *Session, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: show <member>")
	}
	name, err := s.findMember(strings.Join(args, " "))
	if err != nil {
		return err
	}

	v := s.view()
	var m stats.Member
	found := false
	for _, mm := range stats.Members(v) {
		if mm.Name == name {
			m, found = mm, true
		}
	}
	if !found {
		return fmt.Errorf("%s has no commits in the current filters", name)
	}

	lines := []string{
		fmt.Sprintf("%s · %s / %s · %s active",
			render.Plural(m.Commits, "commit"), render.Green("+"+render.Int(m.Added)),
			render.Red("-"+render.Int(m.Deleted)), render.Plural(m.ActiveDays, "day")),
		fmt.Sprintf("First: %s   Last: %s   Longest streak: %s",
			render.Date(m.First), render.Date(m.Last), render.Plural(m.LongestStreak, "day")),
		fmt.Sprintf("Avg commit: %s lines   Biggest: %s lines (%s)",
			render.Int(m.AvgCommit()), render.Int(m.Biggest), shortHash(m.BiggestHash)),
	}
	if len(v.Blame) > 0 {
		lines = append(lines, fmt.Sprintf("Still in the code: %s lines (%s of what they added)",
			render.Int(m.Surviving), render.Pct(m.SurvivingPct())))
	}
	if v.HasPRs {
		lines = append(lines, fmt.Sprintf("PRs opened: %d   Reviews given: %d", m.PRs, m.Reviews))
	}
	box(s, name, lines)

	// Weekly sparkline, with month names underneath.
	tl := stats.Activity(v, stats.Week)
	if len(tl.Starts) > 60 {
		tl = stats.Activity(v, stats.Month)
	}
	vals := tl.By[name]
	label := fmt.Sprintf(" %-16s ", strings.ToUpper(tl.Gran.String()[:1])+tl.Gran.String()[1:]+"ly commits")
	fmt.Fprintf(s.out, "\n%s%s\n", label, render.Tint(0, render.Spark(vals)))
	fmt.Fprintln(s.out, render.Dim(strings.Repeat(" ", render.Width(label))+monthAxis(tl)))

	if areas := stats.WorkAreas(v, name); len(areas) > 0 {
		fmt.Fprintf(s.out, "\n %s\n", render.Bold("Works in"))
		areas = areas[:min(4, len(areas))]
		for _, a := range areas {
			fmt.Fprintf(s.out, "   %s %s %s\n", render.Pad(render.TruncLeft(a.Name, 22), 22),
				render.Pad(render.Bar(a.Pct, areas[0].Pct, 16), 16), render.PadLeft(render.Pct(a.Pct), 4))
		}
	}

	if types := stats.CommitTypes(v, name); len(types) > 0 {
		var parts []string
		for _, t := range types[:min(6, len(types))] {
			parts = append(parts, fmt.Sprintf("%s %s", t.Name, render.Pct(t.Pct)))
		}
		fmt.Fprintf(s.out, "\n %s   %s\n", render.Bold("Commit types"), strings.Join(parts, "  "))
	}

	night := fmt.Sprintf("Night commits (22h–06h): %s", render.Pct(m.NightPct()))
	if m.NightPct() >= 30 {
		night += "  🦉"
	}
	fmt.Fprintf(s.out, "\n %s   Weekend commits: %s\n", night, render.Pct(m.WeekendPct()))
	return nil
}

// box draws lines inside a rounded box with a title:
//
//	╭─ maria ─────────────╮
//	│ 73 commits · …      │
//	╰─────────────────────╯
func box(s *Session, title string, lines []string) {
	inner := render.Width("─ "+title+" ") + 4
	for _, l := range lines {
		inner = max(inner, render.Width(l)+2)
	}
	head := "─ " + render.Bold(title) + " "
	fmt.Fprintf(s.out, " ╭%s%s╮\n", head, strings.Repeat("─", inner-render.Width(head)))
	for _, l := range lines {
		fmt.Fprintf(s.out, " │ %s │\n", render.Pad(l, inner-2))
	}
	fmt.Fprintf(s.out, " ╰%s╯\n", strings.Repeat("─", inner))
}

// monthAxis puts month names under a sparkline, one column per bucket.
func monthAxis(tl stats.Timeline) string {
	axis := []rune(strings.Repeat(" ", len(tl.Starts)+4))
	nextFree := 0
	for i, start := range tl.Starts {
		newMonth := i == 0 || start.Month() != tl.Starts[i-1].Month()
		if tl.Gran == stats.Month {
			newMonth = i == 0 || start.Year() != tl.Starts[i-1].Year()
		}
		if !newMonth || i < nextFree {
			continue
		}
		label := start.Format("Jan")
		if tl.Gran == stats.Month {
			label = start.Format("2006")
		}
		if i+len(label) > len(axis) {
			break
		}
		copy(axis[i:], []rune(label))
		nextFree = i + len(label) + 1
	}
	return strings.TrimRight(string(axis), " ")
}

// compare a b: two members side by side.
func cmdCompare(s *Session, args []string) error {
	if len(args) != 2 {
		return errors.New("usage: compare <member> <member>")
	}
	a, err := s.findMember(args[0])
	if err != nil {
		return err
	}
	b, err := s.findMember(args[1])
	if err != nil {
		return err
	}
	if a == b {
		return fmt.Errorf("that's %s twice: pick two different people", s.name(a))
	}

	v := s.view()
	var ma, mb stats.Member
	ma.Name, mb.Name = a, b // so a member with no commits in the window still shows as zeros
	for _, m := range stats.Members(v) {
		switch m.Name {
		case a:
			ma = m
		case b:
			mb = m
		}
	}

	type row struct {
		label  string
		va, vb float64
		format func(float64) string
	}
	num := func(x float64) string { return render.Short(int(x)) }
	pctf := func(x float64) string { return render.Pct(x) }

	rows := []row{
		{"commits", float64(ma.Commits), float64(mb.Commits), num},
		{"lines added", float64(ma.Added), float64(mb.Added), num},
		{"lines deleted", float64(ma.Deleted), float64(mb.Deleted), num},
	}
	if len(v.Blame) > 0 {
		rows = append(rows, row{"surviving", float64(ma.Surviving), float64(mb.Surviving), num})
	}
	if v.HasPRs {
		rows = append(rows,
			row{"PRs opened", float64(ma.PRs), float64(mb.PRs), num},
			row{"reviews given", float64(ma.Reviews), float64(mb.Reviews), num})
	}
	rows = append(rows,
		row{"active days", float64(ma.ActiveDays), float64(mb.ActiveDays), num},
		row{"longest streak", float64(ma.LongestStreak), float64(mb.LongestStreak), num},
		row{"avg commit", float64(ma.AvgCommit()), float64(mb.AvgCommit()), num},
		row{"night commits", ma.NightPct(), mb.NightPct(), pctf},
		row{"weekend commits", ma.WeekendPct(), mb.WeekendPct(), pctf},
	)

	const labelW, numW, barW = 16, 6, 6
	col := func(i int, value, other float64, format func(float64) string) string {
		bar := render.Tint(i, render.Bar(value, max(value, other), barW))
		cell := render.PadLeft(format(value), numW) + " " + render.Pad(bar, barW)
		if value > other {
			cell = render.Bold(cell)
		}
		return cell
	}

	colW := numW + 1 + barW
	fmt.Fprintf(s.out, " %s %s   %s\n", strings.Repeat(" ", labelW),
		render.PadLeft(render.Bold(s.name(a)), colW), render.PadLeft(render.Bold(s.name(b)), colW))
	for _, r := range rows {
		fmt.Fprintf(s.out, " %s %s   %s\n", render.Pad(r.label, labelW),
			col(0, r.va, r.vb, r.format), col(1, r.vb, r.va, r.format))
	}
	for _, m := range []stats.Member{ma, mb} {
		if m.Commits == 0 {
			fmt.Fprintln(s.out, render.Dim(fmt.Sprintf(" (%s has no commits in the current filters)", s.name(m.Name))))
		}
	}
	return nil
}

func shortHash(h string) string {
	if len(h) > 7 {
		return h[:7]
	}
	return h
}
