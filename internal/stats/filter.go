package stats

import (
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/a-narquel/gh-snitch/internal/github"
	"github.com/a-narquel/gh-snitch/internal/gitlog"
)

// Filter narrows what every command looks at. It persists in the REPL until
// `reset`, and shows up in the prompt: snitch [2w, -proto]>
type Filter struct {
	Since, Until           time.Time // zero = no limit
	SinceLabel, UntilLabel string    // what the user typed, for the prompt
	Exclude                []string  // glob patterns of paths to ignore
	ShowBots               bool      // bots (dependabot[bot]…) are hidden unless this is on
}

// IsBot reports whether a name belongs to a bot account.
func IsBot(name string) bool {
	return strings.HasSuffix(name, "[bot]")
}

// IsZero reports whether no filter is active.
func (f Filter) IsZero() bool {
	return f.Since.IsZero() && f.Until.IsZero() && len(f.Exclude) == 0
}

// Labels are the short descriptions shown in the prompt.
func (f Filter) Labels() []string {
	var labels []string
	if f.SinceLabel != "" {
		labels = append(labels, f.SinceLabel)
	}
	if f.UntilLabel != "" {
		labels = append(labels, "until "+f.UntilLabel)
	}
	for _, p := range f.Exclude {
		labels = append(labels, "-"+strings.TrimPrefix(p, "*."))
	}
	if f.ShowBots {
		labels = append(labels, "+bots")
	}
	return labels
}

// inWindow reports whether t is inside [Since, Until].
func (f Filter) inWindow(t time.Time) bool {
	if !f.Since.IsZero() && t.Before(f.Since) {
		return false
	}
	if !f.Until.IsZero() && t.After(f.Until) {
		return false
	}
	return true
}

// Excludes reports whether a path matches one of the exclude patterns.
//
// A pattern matches the full path ("src/gen/*.go"), the file name ("*.proto")
// or, when it ends with "/", a directory prefix ("vendor/").
func (f Filter) Excludes(p string) bool {
	for _, pat := range f.Exclude {
		if strings.HasSuffix(pat, "/") && strings.HasPrefix(p, pat) {
			return true
		}
		if ok, _ := path.Match(pat, p); ok {
			return true
		}
		if ok, _ := path.Match(pat, path.Base(p)); ok {
			return true
		}
	}
	return false
}

// Apply returns a copy of d with only what passes the filter.
//
// Excluded files are removed from each commit; a commit left with no files
// is dropped (it only touched excluded files).
func (f Filter) Apply(d Data) Data {
	if f.IsZero() && f.ShowBots {
		return d
	}
	hide := func(name string) bool { return !f.ShowBots && IsBot(name) }

	out := d // copies the struct; we then replace the slices below
	out.Commits = nil
	for _, c := range d.Commits {
		if !f.inWindow(c.When) || hide(c.Author) {
			continue
		}
		if len(f.Exclude) > 0 && len(c.Files) > 0 {
			var kept []gitlog.FileChange
			for _, fc := range c.Files {
				if !f.Excludes(fc.Path) {
					kept = append(kept, fc)
				}
			}
			if len(kept) == 0 {
				continue
			}
			c.Files = kept // c is a copy, so the original commit is untouched
		}
		out.Commits = append(out.Commits, c)
	}

	out.Blame = nil
	for _, fb := range d.Blame {
		if f.Excludes(fb.Path) {
			continue
		}
		kept := gitlog.FileBlame{Path: fb.Path}
		for _, l := range fb.Lines {
			if f.inWindow(l.When) && !hide(l.Author) {
				kept.Lines = append(kept.Lines, l)
			}
		}
		out.Blame = append(out.Blame, kept)
	}

	out.PRs = nil
	for _, pr := range d.PRs {
		if !f.inWindow(pr.CreatedAt) || hide(pr.Author) {
			continue
		}
		var reviews []github.Review
		for _, r := range pr.Reviews {
			if !hide(r.Author) {
				reviews = append(reviews, r)
			}
		}
		pr.Reviews = reviews // pr is a copy, so this doesn't change d
		out.PRs = append(out.PRs, pr)
	}
	return out
}

// ParseWhen reads a point in time typed by the user:
//
//	2w, 3d, 12h, 1m (month), 1y  → that long before now
//	2026-10-01                   → that date (local midnight)
func ParseWhen(s string, now time.Time) (time.Time, error) {
	if t, err := time.ParseInLocation(time.DateOnly, s, time.Local); err == nil {
		return t, nil
	}

	if len(s) < 2 {
		return time.Time{}, fmt.Errorf("can't read %q: use 2w, 3d, 1m, 1y or 2026-10-01", s)
	}
	n, err := strconv.Atoi(s[:len(s)-1])
	if err != nil || n < 0 {
		return time.Time{}, fmt.Errorf("can't read %q: use 2w, 3d, 1m, 1y or 2026-10-01", s)
	}

	switch s[len(s)-1] {
	case 'h':
		return now.Add(-time.Duration(n) * time.Hour), nil
	case 'd':
		return now.AddDate(0, 0, -n), nil
	case 'w':
		return now.AddDate(0, 0, -7*n), nil
	case 'm':
		return now.AddDate(0, -n, 0), nil
	case 'y':
		return now.AddDate(-n, 0, 0), nil
	}
	return time.Time{}, fmt.Errorf("can't read %q: unit must be h, d, w, m or y", s)
}

// DescribeWhen turns a relative label into words: "2w" → "last 2 weeks".
// Dates and unknown labels come back unchanged.
func DescribeWhen(label string) string {
	if len(label) < 2 {
		return label
	}
	n, err := strconv.Atoi(label[:len(label)-1])
	if err != nil {
		return label
	}
	units := map[byte]string{'h': "hour", 'd': "day", 'w': "week", 'm': "month", 'y': "year"}
	unit, ok := units[label[len(label)-1]]
	if !ok {
		return label
	}
	if n == 1 {
		return "last " + unit
	}
	return fmt.Sprintf("last %d %ss", n, unit)
}
