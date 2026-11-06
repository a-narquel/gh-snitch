package stats

import (
	"fmt"
	"time"
)

// Award is one line of the `awards` command.
type Award struct {
	Emoji, Title, Winner, Detail string
}

// minCommitsForRates keeps someone with 2 commits at 3am from winning
// "night owl" with 100%. Percentage awards need at least this many commits.
const minCommitsForRates = 5

// Awards hands out the fun titles. now is used for PRs that are still open.
func Awards(d Data, now time.Time) []Award {
	members := Members(d)
	if len(members) == 0 {
		return nil
	}
	total := Summarize(d)
	var awards []Award

	// Members is sorted by commits, so the first one carried the project.
	top := members[0]
	awards = append(awards, Award{"🏆", "Carried the project", top.Name,
		fmt.Sprintf("%.0f%% of commits", pct(top.Commits, total.Commits))})

	// best finds the member with the highest score. Passing a function as an
	// argument is how Go does what Java does with lambdas + Comparator.
	best := func(score func(Member) float64, needCommits int) (Member, bool) {
		var winner Member
		found := false
		for _, m := range members {
			if m.Commits < needCommits {
				continue
			}
			if !found || score(m) > score(winner) {
				winner, found = m, true
			}
		}
		return winner, found && score(winner) > 0
	}
	// rate wraps a percentage score so it only wins above minRate.
	const minRate = 10.0
	rate := func(score func(Member) float64) func(Member) float64 {
		return func(m Member) float64 {
			if s := score(m); s >= minRate {
				return s
			}
			return 0
		}
	}

	if m, ok := best(rate(Member.NightPct), minCommitsForRates); ok {
		awards = append(awards, Award{"🦉", "Night owl", m.Name,
			fmt.Sprintf("%.0f%% of commits after 22h", m.NightPct())})
	}
	if m, ok := best(rate(Member.WeekendPct), minCommitsForRates); ok {
		awards = append(awards, Award{"🍕", "Weekend warrior", m.Name,
			fmt.Sprintf("%.0f%% of commits on weekends", m.WeekendPct())})
	}
	if m, ok := best(func(m Member) float64 { return float64(m.Deleted) }, 0); ok {
		awards = append(awards, Award{"🧹", "The cleaner", m.Name,
			fmt.Sprintf("deleted more than anyone (%d lines)", m.Deleted)})
	}
	if m, ok := best(func(m Member) float64 { return float64(m.Reviews) }, 0); ok {
		awards = append(awards, Award{"👀", "Code police", m.Name,
			fmt.Sprintf("%d reviews given", m.Reviews)})
	}
	if m, ok := best(rate(Member.SurvivingPct), minCommitsForRates); ok {
		awards = append(awards, Award{"🧱", "Built to last", m.Name,
			fmt.Sprintf("%.0f%% of their lines still alive", m.SurvivingPct())})
	}
	if m, ok := best(func(m Member) float64 { return float64(m.Biggest) }, 0); ok {
		awards = append(awards, Award{"💣", "Biggest commit", m.Name,
			fmt.Sprintf("%d lines in one go (%s)", m.Biggest, short(m.BiggestHash))})
	}

	// Slowest PR: longest time from opening to merge/close (or until now if open).
	var slowest time.Duration
	var slowPR string
	var slowWho string
	for _, pr := range d.PRs {
		end := pr.ClosedAt
		if !pr.MergedAt.IsZero() {
			end = pr.MergedAt
		}
		verb := "took"
		if end.IsZero() {
			end, verb = now, "open for"
		}
		if dur := end.Sub(pr.CreatedAt); dur > slowest {
			slowest = dur
			slowWho = d.NameOf(pr.Author)
			slowPR = fmt.Sprintf("#%d %s %d days", pr.Number, verb, int(dur.Hours()/24))
		}
	}
	if slowPR != "" {
		awards = append(awards, Award{"🐌", "Slowest PR", slowWho, slowPR})
	}

	return awards
}

// short returns the 7-character short form of a commit hash.
func short(hash string) string {
	if len(hash) > 7 {
		return hash[:7]
	}
	return hash
}
