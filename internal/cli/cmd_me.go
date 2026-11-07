package cli

import (
	"cmp"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/a-narquel/gh-snitch/internal/github"
	"github.com/a-narquel/gh-snitch/internal/render"
	"github.com/a-narquel/gh-snitch/internal/stats"
)

// cmdMe shows all-time GitHub stats for the logged-in user, or for another
// user with `me <login>`. It doesn't use the repository at all, so it also
// works outside a git repo (see Run).
func cmdMe(s *Session, args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("usage: me [login]")
	}
	login := ""
	if len(args) == 1 {
		login = strings.TrimPrefix(args[0], "@")
	}

	var p github.Profile
	opt := loadOptions{progress: os.Stderr, animate: isTerminal(os.Stderr)}
	err := step(opt, "Fetching profile…", func(progress func(string)) (string, error) {
		var err error
		p, err = github.FetchProfile(login, progress)
		if err != nil {
			return "", err
		}
		return render.Plural(len(p.Years), "year"), nil
	})
	if err != nil {
		return err
	}
	fmt.Fprintln(s.out)

	now := s.now()
	meHeader(s, p)
	meCalendar(s, p, now)
	meYears(s, p)
	meHighlights(s, p, now)
	meLanguages(s, p)
	meTopRepos(s, p)
	return nil
}

// meHeader: André Narquel @a-narquel · since Mar 2025 · 3 followers · 12 repos · ★ 4
func meHeader(s *Session, p github.Profile) {
	title := render.Bold("@" + p.Login)
	if p.Name != "" {
		title = render.Bold(p.Name) + " " + render.Dim("@"+p.Login)
	}
	parts := []string{
		title,
		"since " + p.CreatedAt.Local().Format("Jan 2006"),
		render.Plural(p.Followers, "follower"),
		render.Plural(p.Repos, "repo"),
		"★ " + render.Int(p.Stars),
	}
	fmt.Fprintf(s.out, " %s\n\n", strings.Join(parts, " · "))
}

// meCalendar draws the last 52 weeks, like the grid on a GitHub profile.
func meCalendar(s *Session, p github.Profile, now time.Time) {
	from := now.AddDate(0, 0, -364)
	var start time.Time
	var counts []int
	total := 0
	for _, d := range p.Days {
		if d.Date.Before(from) || d.Date.After(now) {
			continue
		}
		if start.IsZero() {
			start = d.Date
		}
		counts = append(counts, d.Count)
		total += d.Count
	}
	fmt.Fprintf(s.out, " %s %s\n\n", render.Bold(render.Int(total)), "contributions in the last year")
	render.Calendar(s.out, start, counts)
	fmt.Fprintln(s.out)
}

// meYears is one table row per year, plus an all-time total.
func meYears(s *Session, p github.Profile) {
	t := render.Table{
		Headers: []string{"YEAR", "COMMITS", "PRs", "REVIEWS", "ISSUES", "PRIVATE", "TOTAL", ""},
		Align:   []render.Align{render.Left, render.Right, render.Right, render.Right, render.Right, render.Right, render.Right, render.Left},
	}
	var all github.Year
	top := 0
	for _, y := range p.Years {
		top = max(top, y.Total())
	}
	for _, y := range p.Years {
		all.Commits += y.Commits
		all.PRs += y.PRs
		all.Reviews += y.Reviews
		all.Issues += y.Issues
		all.Private += y.Private
		t.Rows = append(t.Rows, []string{
			fmt.Sprint(y.Year), render.Int(y.Commits), render.Int(y.PRs), render.Int(y.Reviews),
			render.Int(y.Issues), render.Int(y.Private), render.Bold(render.Int(y.Total())),
			render.Green(render.Bar(float64(y.Total()), float64(top), 20)),
		})
	}
	t.Footer = [][]string{{
		"all", render.Int(all.Commits), render.Int(all.PRs), render.Int(all.Reviews),
		render.Int(all.Issues), render.Int(all.Private), render.Bold(render.Int(all.Total())), "",
	}}
	t.Render(s.out)
	if all.Private > 0 {
		fmt.Fprintln(s.out, render.Dim(" PRIVATE: contributions to private repos GitHub won't detail."))
	}
	fmt.Fprintln(s.out)
}

// meHighlights: streaks, best day, and the weekday bars.
func meHighlights(s *Session, p github.Profile, now time.Time) {
	current, longest := stats.Streaks(p.Days, now)
	fmt.Fprintf(s.out, " 🔥 Streak      %s now · longest %s\n",
		render.Bold(render.Plural(current, "day")), render.Plural(longest, "day"))

	if best := stats.BestDay(p.Days); best.Count > 0 {
		fmt.Fprintf(s.out, " 📅 Best day    %s · %s\n", render.Date(best.Date), render.Plural(best.Count, "contribution"))
	}

	week := stats.Weekdays(p.Days)
	top := 0
	for _, n := range week {
		top = max(top, n)
	}
	if top == 0 {
		fmt.Fprintln(s.out)
		return
	}
	fmt.Fprintln(s.out, " 🗓  Weekdays")
	order := []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday, time.Saturday, time.Sunday}
	for _, wd := range order {
		n := week[wd]
		mark := ""
		if n == top {
			mark = render.Dim(" ← busiest")
		}
		fmt.Fprintf(s.out, "    %s %s %s%s\n", wd.String()[:3],
			render.Pad(render.Green(render.Bar(float64(n), float64(top), 24)), 24), render.Int(n), mark)
	}
	fmt.Fprintln(s.out)
}

func meLanguages(s *Session, p github.Profile) {
	shares := stats.TopLanguages(p.Languages, 6)
	if len(shares) == 0 {
		return
	}
	fmt.Fprintln(s.out, render.Bold(" Languages")+render.Dim(" (bytes of code in your own repos)"))
	width := 0
	for _, sh := range shares {
		width = max(width, render.Width(sh.Name))
	}
	for i, sh := range shares {
		fmt.Fprintf(s.out, "   %s %s %s\n", render.Pad(sh.Name, width),
			render.Pad(render.Tint(i, render.Bar(sh.Pct, shares[0].Pct, 24)), 24), render.Pct(sh.Pct))
	}
	fmt.Fprintln(s.out)
}

// meTopRepos adds up each year's top repos into an all-time top 5.
func meTopRepos(s *Session, p github.Profile) {
	commits := map[string]int{}
	for _, y := range p.Years {
		for _, r := range y.TopRepos {
			commits[r.Name] += r.Commits
		}
	}
	if len(commits) == 0 {
		return
	}
	type entry struct {
		name string
		n    int
	}
	var list []entry
	for name, n := range commits {
		list = append(list, entry{name, n})
	}
	// Biggest first; ties alphabetical so the output doesn't change between runs.
	slices.SortFunc(list, func(a, b entry) int {
		if c := cmp.Compare(b.n, a.n); c != 0 {
			return c
		}
		return strings.Compare(a.name, b.name)
	})
	if len(list) > 5 {
		list = list[:5]
	}

	fmt.Fprintln(s.out, render.Bold(" Top repos")+render.Dim(" (by your commits)"))
	width := 0
	for _, e := range list {
		width = max(width, render.Width(e.name))
	}
	for _, e := range list {
		fmt.Fprintf(s.out, "   %s %s\n", render.Pad(e.name, width), render.PadLeft(render.Int(e.n), 6)+" commits")
	}
}
