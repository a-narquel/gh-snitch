package stats

import (
	"cmp"
	"slices"
	"strings"
	"time"
)

// ReviewStats is the "who reviews whom" picture.
type ReviewStats struct {
	// Matrix[reviewer][author] = PRs by author that reviewer reviewed.
	Matrix map[string]map[string]int
	People []string // everyone who gave or got a review, most involved first

	MedianFirstReview time.Duration // PR opened → first review by someone else
	MedianMerge       time.Duration // PR opened → merged
	HasFirstReview    bool
	HasMerge          bool

	// SelfMerged[author] = PRs they merged themselves with no review from anyone else.
	SelfMerged map[string]int
}

// Reviews computes review stats. Names are git author names when the login
// could be linked to a commit author, otherwise GitHub logins.
func Reviews(d Data) ReviewStats {
	rs := ReviewStats{Matrix: map[string]map[string]int{}, SelfMerged: map[string]int{}}
	seen := map[string]bool{}
	var firstReview, toMerge []time.Duration

	for _, pr := range d.PRs {
		author := d.NameOf(pr.Author)
		seen[author] = true

		revs := reviewers(pr)
		for _, login := range revs {
			reviewer := d.NameOf(login)
			seen[reviewer] = true
			if rs.Matrix[reviewer] == nil {
				rs.Matrix[reviewer] = map[string]int{}
			}
			rs.Matrix[reviewer][author]++
		}

		var first time.Time
		for _, r := range pr.Reviews {
			if r.Author != pr.Author && !r.SubmittedAt.IsZero() &&
				(first.IsZero() || r.SubmittedAt.Before(first)) {
				first = r.SubmittedAt
			}
		}
		if !first.IsZero() {
			firstReview = append(firstReview, first.Sub(pr.CreatedAt))
		}

		if !pr.MergedAt.IsZero() {
			toMerge = append(toMerge, pr.MergedAt.Sub(pr.CreatedAt))
			if len(revs) == 0 && pr.MergedBy == pr.Author {
				rs.SelfMerged[author]++
			}
		}
	}

	rs.MedianFirstReview, rs.HasFirstReview = median(firstReview)
	rs.MedianMerge, rs.HasMerge = median(toMerge)

	// Order people by how involved they are in reviews (given + received),
	// leaving out anyone who neither gave nor got a review.
	involvement := map[string]int{}
	for reviewer, row := range rs.Matrix {
		for author, n := range row {
			involvement[reviewer] += n
			involvement[author] += n
		}
	}
	for name := range seen {
		if involvement[name] > 0 {
			rs.People = append(rs.People, name)
		}
	}
	slices.SortFunc(rs.People, func(a, b string) int {
		if c := cmp.Compare(involvement[b], involvement[a]); c != 0 {
			return c
		}
		return strings.Compare(a, b)
	})
	return rs
}

// median returns the middle value, and false if there are no values.
func median(ds []time.Duration) (time.Duration, bool) {
	if len(ds) == 0 {
		return 0, false
	}
	s := slices.Clone(ds) // sort a copy, so the caller's slice isn't reordered
	slices.Sort(s)
	mid := len(s) / 2
	if len(s)%2 == 0 {
		return (s[mid-1] + s[mid]) / 2, true
	}
	return s[mid], true
}
