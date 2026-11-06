package render

import (
	"fmt"
	"io"
	"math"
	"strings"
)

// Series is one line on a chart.
type Series struct {
	Name   string
	Values []int
}

// Marker labels a point on the x axis, e.g. a git tag: "▲ v1.0".
type Marker struct {
	Index int // which point it belongs to
	Label string
}

// cell is one character of the chart grid and which series drew it.
type cell struct {
	ch     rune
	series int // -1 = empty or axis
}

// LineChart draws one or more series as a line chart with box characters:
//
//	40 ┤        ╭╮
//	20 ┤   ╭───╯╰╮      ── andre
//	 0 ┼───╯     ╰──    ── joao
//	   Sep 14   Sep 28
//
// All series must have the same length. xLabels has one label per point
// (some are skipped if there's no room).
func LineChart(w io.Writer, series []Series, xLabels []string, markers []Marker) {
	if len(series) == 0 || len(series[0].Values) == 0 {
		return
	}
	n := len(series[0].Values)

	// Each point gets a few columns, so short timelines aren't squashed.
	colsPer := min(max(60/n, 1), 6)
	width := n * colsPer

	top := 0
	for _, s := range series {
		for _, v := range s.Values {
			top = max(top, v)
		}
	}
	if top == 0 {
		top = 1
	}
	height := min(max(top, 4), 10)
	yOf := func(v int) int { return int(math.Round(float64(v) / float64(top) * float64(height))) }

	// grid[row][col]; row 0 is the top of the chart.
	grid := make([][]cell, height+1)
	for r := range grid {
		grid[r] = make([]cell, width)
		for c := range grid[r] {
			grid[r][c] = cell{' ', -1}
		}
	}
	set := func(y, x int, ch rune, s int) { grid[height-y][x] = cell{ch, s} }

	// Draw the last series first, so the first (the top contributor) ends up on top.
	for si := len(series) - 1; si >= 0; si-- {
		vals := series[si].Values
		at := func(x int) int { return yOf(vals[min(x/colsPer, n-1)]) }

		for x := 0; x < width-1; x++ {
			y0, y1 := at(x), at(x+1)
			switch {
			case y0 == y1:
				set(y0, x, '─', si)
			case y0 < y1: // going up
				set(y0, x, '╯', si)
				set(y1, x, '╭', si)
			default: // going down
				set(y0, x, '╮', si)
				set(y1, x, '╰', si)
			}
			for y := min(y0, y1) + 1; y < max(y0, y1); y++ {
				set(y, x, '│', si)
			}
		}
		set(at(width-1), width-1, '─', si)
	}

	labelW := Width(Int(top))
	lastLabel := ""
	for r := 0; r <= height; r++ {
		value := int(math.Round(float64(top) * float64(height-r) / float64(height)))
		label, axis := "", "│"
		if l := Int(value); l != lastLabel || r == height {
			label, axis, lastLabel = l, "┤", l
		}
		if r == height {
			axis = "┼"
		}

		var b strings.Builder
		b.WriteString(" " + PadLeft(label, labelW) + " " + Dim(axis))
		for _, c := range grid[r] {
			if c.series < 0 {
				b.WriteRune(c.ch)
			} else {
				b.WriteString(Tint(c.series, string(c.ch)))
			}
		}
		if r < len(series) {
			b.WriteString("    " + Tint(r, "──") + " " + series[r].Name)
		}
		fmt.Fprintln(w, strings.TrimRight(b.String(), " "))
	}

	// x labels: place them left to right, skipping any that would overlap.
	indent := 1 + labelW + 1 + 1 // " " + label + " " + axis: where the chart's first column starts
	line := []rune(strings.Repeat(" ", indent+width+20))
	nextFree := 0
	for i, label := range xLabels {
		col := i * colsPer
		if col < nextFree || label == "" {
			continue
		}
		copy(line[indent+col:], []rune(label))
		nextFree = col + Width(label) + 3
	}
	fmt.Fprintln(w, Dim(strings.TrimRight(string(line), " ")))

	for _, m := range markers {
		if m.Index < 0 || m.Index >= n {
			continue
		}
		fmt.Fprintln(w, strings.Repeat(" ", indent+m.Index*colsPer)+Yellow("▲ "+m.Label))
	}
}

// heat are the punch card intensity characters, from "nothing" to "a lot".
var heat = []string{" ", "·", "░", "▒", "▓", "█"}

// HeatLevels is how many non-empty intensity levels Heat has.
const HeatLevels = 5

// Heat returns the punch card character for level 0 (none) … HeatLevels (max).
func Heat(level int) string {
	return heat[min(max(level, 0), HeatLevels)]
}
