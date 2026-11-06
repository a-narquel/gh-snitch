package cli

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/a-narquel/gh-snitch/internal/render"
	"github.com/a-narquel/gh-snitch/internal/stats"
)

func cmdSince(s *Session, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: since <when>   e.g. since 2w, since 2026-10-01")
	}
	return s.setSince(args[0], true)
}

func cmdUntil(s *Session, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: until <when>   e.g. until 1w, until 2026-10-31")
	}
	return s.setUntil(args[0], true)
}

// setSince sets the start of the time window. report prints a ✓ line.
func (s *Session) setSince(arg string, report bool) error {
	t, err := stats.ParseWhen(arg, s.now())
	if err != nil {
		return err
	}
	s.filter.Since, s.filter.SinceLabel = t, arg
	if report {
		s.reportWindow("filter: " + describe(arg, "since"))
	}
	return nil
}

// setUntil sets the end of the time window. A plain date means "up to the
// end of that day", so `until 2026-10-31` includes commits made on the 31st.
func (s *Session) setUntil(arg string, report bool) error {
	t, err := stats.ParseWhen(arg, s.now())
	if err != nil {
		return err
	}
	if _, isDate := time.Parse(time.DateOnly, arg); isDate == nil {
		t = t.AddDate(0, 0, 1).Add(-time.Nanosecond)
	}
	s.filter.Until, s.filter.UntilLabel = t, arg
	if report {
		s.reportWindow("filter: " + describe(arg, "until"))
	}
	return nil
}

// describe turns "2w" into "last 2 weeks" and a date into "since 2026-10-01".
func describe(arg, word string) string {
	if d := stats.DescribeWhen(arg); d != arg && word == "since" {
		return d
	}
	return word + " " + arg
}

// reportWindow prints a ✓ line with the dates the filter now covers.
func (s *Session) reportWindow(what string) {
	from, to := s.filter.Since, s.filter.Until
	if from.IsZero() {
		from, _ = s.data.Span()
	}
	if to.IsZero() {
		to = s.now()
	}
	s.ok("%s (%s → %s)", what, render.Date(from), render.Date(to))
	if len(s.view().Commits) == 0 {
		fmt.Fprintln(s.out, render.Dim("   no commits in that window"))
	}
}

func cmdExclude(s *Session, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: exclude <glob>…   e.g. exclude *.proto vendor/ go.sum")
	}
	for _, pat := range args {
		if !slices.Contains(s.filter.Exclude, pat) {
			s.filter.Exclude = append(s.filter.Exclude, pat)
		}
	}

	// Count how many files the new patterns hit, so typos are easy to notice.
	hit := map[string]bool{}
	only := stats.Filter{Exclude: args}
	for _, c := range s.data.Commits {
		for _, f := range c.Files {
			if only.Excludes(f.Path) {
				hit[f.Path] = true
			}
		}
	}
	s.ok("excluding %v (%s)", args, render.Plural(len(hit), "file"))
	return nil
}

func cmdInclude(s *Session, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: include <glob>   (undoes 'exclude <glob>')")
	}
	i := slices.Index(s.filter.Exclude, args[0])
	if i < 0 {
		return fmt.Errorf("%q isn't excluded (current: %v)", args[0], s.filter.Exclude)
	}
	s.filter.Exclude = slices.Delete(s.filter.Exclude, i, i+1)
	s.ok("including %s again", args[0])
	return nil
}

func cmdBots(s *Session, args []string) error {
	switch {
	case len(args) == 0:
		s.filter.ShowBots = !s.filter.ShowBots // no argument: toggle
	case args[0] == "on":
		s.filter.ShowBots = true
	case args[0] == "off":
		s.filter.ShowBots = false
	default:
		return errors.New("usage: bots [on|off]")
	}
	if s.filter.ShowBots {
		s.ok("showing bots")
	} else {
		s.ok("hiding bots")
	}
	return nil
}

func cmdReset(s *Session, args []string) error {
	s.filter = stats.Filter{} // the zero value of Filter means "no filters"
	s.ok("filters cleared")
	return nil
}
