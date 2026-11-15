package cli

import (
	"fmt"
	"slices"
	"strings"

	"github.com/a-narquel/gh-snitch/internal/render"
	"github.com/a-narquel/gh-snitch/internal/stats"
)

// periods are the choices in Filters → Period. An empty label means all time.
var periods = []struct{ label, name string }{
	{"", "All time"},
	{"1w", "Last week"},
	{"2w", "Last 2 weeks"},
	{"1m", "Last month"},
	{"3m", "Last 3 months"},
	{"6m", "Last 6 months"},
	{"1y", "Last year"},
}

// setSince sets the start of the time window ("2w", "2026-10-01"), or
// clears it when arg is "".
func (s *Session) setSince(arg string) error {
	if arg == "" {
		s.filter.Since, s.filter.SinceLabel = stats.Filter{}.Since, ""
		return nil
	}
	t, err := stats.ParseWhen(arg, s.now())
	if err != nil {
		return err
	}
	s.filter.Since, s.filter.SinceLabel = t, arg
	return nil
}

// addExcludes ignores files matching the glob patterns from now on. It
// returns how many files the patterns hit, so a typo (0 files) stands out.
func (s *Session) addExcludes(patterns []string) int {
	for _, p := range patterns {
		if !slices.Contains(s.filter.Exclude, p) {
			s.filter.Exclude = append(s.filter.Exclude, p)
		}
	}
	hit := map[string]bool{}
	only := stats.Filter{Exclude: patterns}
	for _, c := range s.data.Commits {
		for _, f := range c.Files {
			if only.Excludes(f.Path) {
				hit[f.Path] = true
			}
		}
	}
	return len(hit)
}

func (s *Session) removeExclude(pattern string) {
	s.filter.Exclude = slices.DeleteFunc(s.filter.Exclude, func(p string) bool { return p == pattern })
}

// periodName is how the current period reads in the menu: "Last 2 weeks".
func (s *Session) periodName() string {
	for _, p := range periods {
		if p.label == s.filter.SinceLabel {
			return p.name
		}
	}
	return stats.DescribeWhen(s.filter.SinceLabel) // e.g. from --since 3d
}

// filterLabel sums up the filters for the header, or "" when there are none:
// "last 2 weeks · without *.proto, vendor/"
func (s *Session) filterLabel() string {
	var parts []string
	if s.filter.SinceLabel != "" {
		parts = append(parts, strings.ToLower(s.periodName()))
	}
	if len(s.filter.Exclude) > 0 {
		parts = append(parts, "without "+strings.Join(s.filter.Exclude, ", "))
	}
	return strings.Join(parts, " · ")
}

// excludeStatus is the message shown after adding excludes.
func excludeStatus(patterns []string, hits int) string {
	msg := fmt.Sprintf("excluding %s (%s)", strings.Join(patterns, ", "), render.Plural(hits, "file"))
	if hits == 0 {
		return render.Yellow("! " + msg + ": no files match, typo?")
	}
	return render.Green("✓ ") + msg
}
