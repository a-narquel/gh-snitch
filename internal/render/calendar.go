package render

import (
	"fmt"
	"io"
	"slices"
	"strings"
	"time"
)

// Calendar draws a GitHub-style contribution grid: one column per week, one
// row per weekday (Monday on top), with month names above.
//
//	     Dec    Jan    Feb …
//	Mon  ·░·▒▓··░█…
//	Wed  …
//
// counts[i] is the number of contributions on start + i days.
func Calendar(w io.Writer, start time.Time, counts []int) {
	if len(counts) == 0 {
		return
	}
	// Go back to the Monday of the first week, so every column is Mon..Sun.
	// (int(time.Sunday) is 0, so this maps Mon→0 … Sun→6.)
	lead := (int(start.Weekday()) + 6) % 7
	first := start.AddDate(0, 0, -lead)
	weeks := (lead + len(counts) + 6) / 7

	// grid[row][col] = count, or -1 for days outside the range.
	var grid [7][]int
	for r := range grid {
		grid[r] = slices.Repeat([]int{-1}, weeks)
	}
	for i, n := range counts {
		pos := lead + i
		grid[pos%7][pos/7] = n
	}

	thresholds := heatThresholds(counts)

	// Month labels: put a name over the first week that starts in that month.
	const indent = 5 // "Mon  "
	labels := []rune(strings.Repeat(" ", weeks+4))
	nextFree := 0
	for col := range weeks {
		day := first.AddDate(0, 0, col*7)
		if col > 0 && day.Day() > 7 || col < nextFree {
			continue
		}
		name := day.Format("Jan")
		copy(labels[col:], []rune(name))
		nextFree = col + len(name) + 1
	}
	fmt.Fprintln(w, Dim(strings.Repeat(" ", indent)+strings.TrimRight(string(labels), " ")))

	rowNames := []string{"Mon", "", "Wed", "", "Fri", "", "Sun"}
	for r, row := range grid {
		var b strings.Builder
		b.WriteString(Dim(Pad(rowNames[r], indent)))
		for _, n := range row {
			b.WriteString(calCell(n, thresholds))
		}
		fmt.Fprintln(w, strings.TrimRight(b.String(), " "))
	}

	// Legend.
	var legend strings.Builder
	for lvl := range 5 {
		legend.WriteString(paintCell(lvl))
	}
	fmt.Fprintf(w, "%s%s %s %s\n", strings.Repeat(" ", indent), Dim("less"), legend.String(), Dim("more"))
}

// heatThresholds splits the non-zero counts into 4 groups of about the same
// size (quartiles), like GitHub does, so one crazy day doesn't make every
// other day look empty.
func heatThresholds(counts []int) [3]int {
	var nonzero []int
	for _, n := range counts {
		if n > 0 {
			nonzero = append(nonzero, n)
		}
	}
	if len(nonzero) == 0 {
		return [3]int{}
	}
	slices.Sort(nonzero)
	q := func(f float64) int { return nonzero[int(f*float64(len(nonzero)-1))] }
	return [3]int{q(0.25), q(0.5), q(0.75)}
}

func calCell(n int, t [3]int) string {
	switch {
	case n < 0:
		return " "
	case n == 0:
		return paintCell(0)
	case n <= t[0]:
		return paintCell(1)
	case n <= t[1]:
		return paintCell(2)
	case n <= t[2]:
		return paintCell(3)
	default:
		return paintCell(4)
	}
}

// paintCell draws one square at intensity 0 (none) … 4 (most).
func paintCell(level int) string {
	if level == 0 {
		return Dim("·")
	}
	return Green(heat[level+1]) // ░ ▒ ▓ █
}
