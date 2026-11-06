// Package cli is the user-facing part of snitch: loading the data with
// spinners, the interactive prompt, and every command.
package cli

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/a-narquel/gh-snitch/internal/gitlog"
	"github.com/a-narquel/gh-snitch/internal/render"
	"github.com/a-narquel/gh-snitch/internal/stats"
)

// Session holds the loaded data and the filters the user has set.
// Commands are functions that receive a *Session (a pointer, so a command
// like `since` can change the filter and the change sticks).
type Session struct {
	data   stats.Data
	filter stats.Filter

	repo   string // "owner/name", or the folder name if there's no GitHub remote
	branch string
	tags   []gitlog.Tag
	ghNote string // why GitHub data is missing, when it is

	out   io.Writer
	now   func() time.Time  // a function so tests can freeze time
	short map[string]string // full name → short display name
}

// errNoCommits is returned by commands when the filters leave nothing to show.
var errNoCommits = errors.New("no commits match the current filters (try 'reset')")

// view is the data after applying the current filters. Every command starts here.
func (s *Session) view() stats.Data {
	return s.filter.Apply(s.data)
}

// name returns the short display name for a person: the first word of their
// git name ("André Narquel" → "André") when no one else shares it.
func (s *Session) name(full string) string {
	if n, ok := s.short[full]; ok {
		return n
	}
	return full
}

// buildShortNames fills s.short for everyone who appears in the data.
func (s *Session) buildShortNames() {
	var everyone []string
	for _, m := range stats.Members(s.data) {
		everyone = append(everyone, m.Name)
	}
	for _, pr := range s.data.PRs {
		everyone = append(everyone, s.data.NameOf(pr.Author))
		for _, r := range pr.Reviews {
			everyone = append(everyone, s.data.NameOf(r.Author))
		}
	}

	firstWord := func(n string) string {
		if f := strings.Fields(n); len(f) > 0 {
			return f[0]
		}
		return n
	}
	count := map[string]int{}
	seen := map[string]bool{}
	for _, n := range everyone {
		if !seen[n] {
			seen[n] = true
			count[fold(firstWord(n))]++
		}
	}

	s.short = map[string]string{}
	for n := range seen {
		if count[fold(firstWord(n))] == 1 {
			s.short[n] = firstWord(n)
		}
	}
}

// accents maps accented letters to plain ones, so "andre" finds "André".
var accents = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a",
	"é", "e", "è", "e", "ê", "e", "ë", "e",
	"í", "i", "ì", "i", "î", "i", "ï", "i",
	"ó", "o", "ò", "o", "ô", "o", "õ", "o", "ö", "o",
	"ú", "u", "ù", "u", "û", "u", "ü", "u",
	"ç", "c", "ñ", "n",
)

// fold makes a name easy to compare: lower case, no accents.
func fold(s string) string {
	return accents.Replace(strings.ToLower(s))
}

// findMember turns what the user typed into a full member name.
// It tries an exact match first, then the start of a name, then anywhere in it.
func (s *Session) findMember(query string) (string, error) {
	var names []string
	for _, m := range stats.Members(s.data) {
		names = append(names, m.Name)
	}
	q := fold(query)

	matchers := []func(full, short string) bool{
		func(full, short string) bool { return full == q || short == q },
		func(full, short string) bool { return strings.HasPrefix(full, q) || strings.HasPrefix(short, q) },
		func(full, short string) bool { return strings.Contains(full, q) },
	}
	for _, match := range matchers {
		var found []string
		for _, n := range names {
			if match(fold(n), fold(s.name(n))) {
				found = append(found, n)
			}
		}
		switch len(found) {
		case 0:
			continue
		case 1:
			return found[0], nil
		default:
			slices.Sort(found)
			return "", fmt.Errorf("%q matches several people: %s", query, strings.Join(found, ", "))
		}
	}
	return "", fmt.Errorf("no member called %q (see 'who')", query)
}

// period describes the time range of the data: "Sep 14 → Nov 6 (53 days)".
func period(d stats.Data) string {
	first, last := d.Span()
	if first.IsZero() {
		return "no commits"
	}
	return fmt.Sprintf("%s → %s (%s)", render.Date(first), render.Date(last),
		render.Plural(stats.Days(first, last), "day"))
}

// prompt is "snitch> ", or "snitch [2w, -proto]> " when filters are on.
func (s *Session) prompt() string {
	p := render.Bold("snitch")
	if labels := s.filter.Labels(); len(labels) > 0 {
		p += " " + render.Yellow("["+strings.Join(labels, ", ")+"]")
	}
	return p + "> "
}

// ok and fail print the small ✓ / ✗ status lines.
func (s *Session) ok(format string, args ...any) {
	fmt.Fprintf(s.out, " %s %s\n", render.Green("✓"), fmt.Sprintf(format, args...))
}

func (s *Session) fail(err error) {
	fmt.Fprintf(s.out, " %s %v\n", render.Red("✗"), err)
}
