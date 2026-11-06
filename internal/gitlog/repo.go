package gitlog

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

// Tag is a git tag with the date it was created. Tags show up as markers
// (▲) on the activity chart, e.g. "▲ deadline-stage-1".
type Tag struct {
	Name string
	When time.Time
}

// CurrentBranch returns the name of the checked-out branch (e.g. "main").
func CurrentBranch(repoDir string) (string, error) {
	out, err := git(repoDir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// Tags returns the repository's tags, oldest first.
func Tags(repoDir string) ([]Tag, error) {
	// %09 is a tab in for-each-ref formats.
	out, err := git(repoDir, "for-each-ref", "--format=%(refname:short)%09%(creatordate:iso-strict)", "refs/tags")
	if err != nil {
		return nil, err
	}

	var tags []Tag
	for _, line := range strings.Split(out, "\n") {
		name, date, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		when, err := time.Parse(time.RFC3339, date)
		if err != nil {
			return nil, fmt.Errorf("tag %s: bad date: %w", name, err)
		}
		tags = append(tags, Tag{Name: name, When: when})
	}

	slices.SortFunc(tags, func(a, b Tag) int { return a.When.Compare(b.When) })
	return tags, nil
}
