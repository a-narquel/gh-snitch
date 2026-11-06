package stats

import (
	"cmp"
	"maps"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/a-narquel/gh-snitch/internal/github"
)

// Member is everything we compute about one person.
type Member struct {
	Name string

	Commits, Added, Deleted int
	Surviving               int // lines they wrote that still exist at HEAD

	ActiveDays    int
	LongestStreak int // most consecutive active days
	First, Last   time.Time

	Biggest     int    // lines changed in their biggest commit
	BiggestHash string // that commit's hash

	Night, Weekend int // commits made 22h–06h / on Sat–Sun

	PRs     int // pull requests opened
	Reviews int // reviews given on other people's PRs
}

// AvgCommit is the average number of lines changed per commit.
func (m Member) AvgCommit() int {
	if m.Commits == 0 {
		return 0
	}
	return (m.Added + m.Deleted) / m.Commits
}

// NightPct is the share of commits made at night, 0..100.
func (m Member) NightPct() float64 { return pct(m.Night, m.Commits) }

// WeekendPct is the share of commits made on weekends, 0..100.
func (m Member) WeekendPct() float64 { return pct(m.Weekend, m.Commits) }

// SurvivingPct is how much of what they added is still in the code, 0..100.
// Blame can credit someone with lines they moved rather than wrote, which can
// push this past 100%, so it's capped.
func (m Member) SurvivingPct() float64 { return min(100, pct(m.Surviving, m.Added)) }

// Members computes stats for every commit author, most commits first.
func Members(d Data) []Member {
	byName := map[string]*Member{} // pointers, so we can update in place
	days := map[string]map[time.Time]bool{}

	get := func(name string) *Member {
		m, ok := byName[name]
		if !ok {
			m = &Member{Name: name}
			byName[name] = m
			days[name] = map[time.Time]bool{}
		}
		return m
	}

	for _, c := range d.Commits {
		m := get(c.Author)
		m.Commits++

		lines := 0
		for _, f := range c.Files {
			m.Added += f.Added
			m.Deleted += f.Deleted
			lines += f.Added + f.Deleted
		}
		if lines > m.Biggest {
			m.Biggest, m.BiggestHash = lines, c.Hash
		}

		if m.First.IsZero() || c.When.Before(m.First) {
			m.First = c.When
		}
		if c.When.After(m.Last) {
			m.Last = c.When
		}
		if isNight(c.When) {
			m.Night++
		}
		if isWeekend(c.When) {
			m.Weekend++
		}
		days[c.Author][dayOf(c.When)] = true
	}

	for author, lines := range Ownership(d) {
		if m, ok := byName[author]; ok {
			m.Surviving = lines
		}
	}

	for _, pr := range d.PRs {
		if m, ok := byName[d.NameOf(pr.Author)]; ok {
			m.PRs++
		}
		for _, reviewer := range reviewers(pr) {
			if m, ok := byName[d.NameOf(reviewer)]; ok {
				m.Reviews++
			}
		}
	}

	members := make([]Member, 0, len(byName))
	for name, m := range byName {
		m.ActiveDays = len(days[name])
		m.LongestStreak = longestStreak(days[name])
		members = append(members, *m)
	}

	slices.SortFunc(members, func(a, b Member) int {
		if c := cmp.Compare(b.Commits, a.Commits); c != 0 {
			return c // more commits first
		}
		return strings.Compare(a.Name, b.Name)
	})
	return members
}

// longestStreak counts the most consecutive days in the set.
func longestStreak(days map[time.Time]bool) int {
	sorted := slices.SortedFunc(maps.Keys(days), func(a, b time.Time) int { return a.Compare(b) })

	best, run := 0, 0
	for i, d := range sorted {
		// AddDate (not +24h) so daylight-saving days still count as "next day".
		if i > 0 && sorted[i-1].AddDate(0, 0, 1).Equal(d) {
			run++
		} else {
			run = 1
		}
		best = max(best, run)
	}
	return best
}

// reviewers returns the distinct people who reviewed a PR, not counting its author.
func reviewers(pr github.PR) []string {
	seen := map[string]bool{}
	var out []string
	for _, r := range pr.Reviews {
		if r.Author == "" || r.Author == pr.Author || seen[r.Author] {
			continue
		}
		seen[r.Author] = true
		out = append(out, r.Author)
	}
	return out
}

// Summary is the "total" row of the who table.
type Summary struct {
	Commits, Added, Deleted, Surviving int
	ActiveDays                         int
	PRs, Reviews                       int
}

// Summarize adds up the whole (filtered) dataset.
func Summarize(d Data) Summary {
	var s Summary
	days := map[time.Time]bool{}
	for _, c := range d.Commits {
		s.Commits++
		for _, f := range c.Files {
			s.Added += f.Added
			s.Deleted += f.Deleted
		}
		days[dayOf(c.When)] = true
	}
	s.ActiveDays = len(days)
	for _, lines := range Ownership(d) {
		s.Surviving += lines
	}
	s.PRs = len(d.PRs)
	for _, pr := range d.PRs {
		s.Reviews += len(reviewers(pr))
	}
	return s
}

// Ownership counts how many current lines (at HEAD) each author wrote.
func Ownership(d Data) map[string]int {
	owned := map[string]int{}
	for _, fb := range d.Blame {
		for _, l := range fb.Lines {
			owned[l.Author]++
		}
	}
	return owned
}

// busFactorShare is the share of the code that the "bus factor" people own
// together: the fewest people who'd take more than half the code with them.
const busFactorShare = 50.0

// BusFactor returns the smallest group of authors who own more than half of
// the current code, and the share they own together.
func BusFactor(d Data) (names []string, share float64) {
	owned := Ownership(d)
	total := 0
	for _, n := range owned {
		total += n
	}
	if total == 0 {
		return nil, 0
	}

	authors := slices.SortedFunc(maps.Keys(owned), func(a, b string) int {
		return cmp.Compare(owned[b], owned[a])
	})
	sum := 0
	for _, a := range authors {
		names = append(names, a)
		sum += owned[a]
		if pct(sum, total) > busFactorShare {
			break
		}
	}
	return names, pct(sum, total)
}

// dirOf groups a path into an "area" of the project: up to two directories
// deep, so "src/client/ui/App.java" → "src/client/", "docs/x.md" → "docs/".
func dirOf(p string) string {
	parts := strings.Split(p, "/")
	switch len(parts) {
	case 1:
		return "(root)"
	case 2:
		return parts[0] + "/"
	default:
		return parts[0] + "/" + parts[1] + "/"
	}
}

// WorkAreas returns where a member changed the most lines, biggest first.
func WorkAreas(d Data, member string) []Share {
	churn := map[string]int{}
	for _, c := range d.Commits {
		if c.Author != member {
			continue
		}
		for _, f := range c.Files {
			churn[dirOf(f.Path)] += f.Added + f.Deleted
		}
	}
	return toShares(churn)
}

// conventional matches commit subjects like "feat(ui): add button" or "fix!: crash".
var conventional = regexp.MustCompile(`^([a-zA-Z]+)(\([^)]*\))?!?:`)

// commitTypes are the Conventional Commits types we recognise, plus a few
// common spellings that mean the same thing.
var commitTypes = map[string]string{
	"feat": "feat", "feature": "feat",
	"fix": "fix", "bugfix": "fix", "hotfix": "fix",
	"docs": "docs", "doc": "docs",
	"test": "test", "tests": "test",
	"refactor": "refactor", "perf": "perf", "style": "style",
	"build": "build", "ci": "ci", "chore": "chore", "revert": "revert",
}

// CommitTypes splits a member's commits by Conventional Commit type.
// It returns nil when none of their commits use the convention.
func CommitTypes(d Data, member string) []Share {
	counts := map[string]int{}
	typed := 0
	for _, c := range d.Commits {
		if c.Author != member {
			continue
		}
		kind := "other"
		if m := conventional.FindStringSubmatch(c.Subject); m != nil {
			if t, ok := commitTypes[strings.ToLower(m[1])]; ok {
				kind = t
				typed++
			}
		}
		counts[kind]++
	}
	if typed == 0 {
		return nil
	}
	return toShares(counts)
}

// toShares turns counts into shares sorted biggest first.
func toShares(counts map[string]int) []Share {
	total := 0
	for _, n := range counts {
		total += n
	}
	shares := make([]Share, 0, len(counts))
	for name, n := range counts {
		shares = append(shares, Share{Name: name, Value: n, Pct: pct(n, total)})
	}
	slices.SortFunc(shares, func(a, b Share) int {
		if c := cmp.Compare(b.Value, a.Value); c != 0 {
			return c
		}
		return strings.Compare(a.Name, b.Name)
	})
	return shares
}
