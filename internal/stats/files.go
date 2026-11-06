package stats

import (
	"cmp"
	"slices"
	"strings"
)

// FileStat describes how much a file was worked on.
type FileStat struct {
	Path     string
	Changes  int // commits that touched it
	Churn    int // lines added + deleted, over all those commits
	Authors  int // distinct people who touched it
	Owner    string
	OwnerPct float64 // owner's share of the current lines (or of the churn if the file is gone)
}

// Files ranks files by how often they changed (the "hotspots").
func Files(d Data) []FileStat {
	type acc struct {
		changes, churn int
		churnBy        map[string]int
	}
	byPath := map[string]*acc{}

	for _, c := range d.Commits {
		for _, f := range c.Files {
			a := byPath[f.Path]
			if a == nil {
				a = &acc{churnBy: map[string]int{}}
				byPath[f.Path] = a
			}
			a.changes++
			a.churn += f.Added + f.Deleted
			a.churnBy[c.Author] += f.Added + f.Deleted
		}
	}

	// Who owns each file's current lines, from blame.
	ownedBy := map[string]map[string]int{}
	for _, fb := range d.Blame {
		counts := map[string]int{}
		for _, l := range fb.Lines {
			counts[l.Author]++
		}
		ownedBy[fb.Path] = counts
	}

	files := make([]FileStat, 0, len(byPath))
	for path, a := range byPath {
		fs := FileStat{Path: path, Changes: a.changes, Churn: a.churn, Authors: len(a.churnBy)}

		// Prefer blame (who owns the code now). For deleted files, fall back
		// to who changed it the most.
		counts := ownedBy[path]
		if len(counts) == 0 {
			counts = a.churnBy
		}
		fs.Owner, fs.OwnerPct = top(counts)
		files = append(files, fs)
	}

	slices.SortFunc(files, func(a, b FileStat) int {
		if c := cmp.Compare(b.Changes, a.Changes); c != 0 {
			return c
		}
		if c := cmp.Compare(b.Churn, a.Churn); c != 0 {
			return c
		}
		return strings.Compare(a.Path, b.Path)
	})
	return files
}

// top returns the key with the biggest count and its share of the total.
func top(counts map[string]int) (string, float64) {
	best, bestN, total := "", -1, 0
	for k, n := range counts {
		total += n
		// Ties go to the alphabetically first name, so results are stable
		// (map iteration order in Go is random on purpose).
		if n > bestN || (n == bestN && k < best) {
			best, bestN = k, n
		}
	}
	return best, pct(bestN, total)
}
