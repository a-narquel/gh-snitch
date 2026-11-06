package render

import (
	"fmt"
	"io"
	"strings"
)

// Align says how a column lines up.
type Align int

const (
	Left Align = iota
	Right
)

// Table is a simple text table:
//
//	MEMBER   COMMITS   +LINES
//	───────  ────────  ──────
//	andre    198       14,203
//	───────  ────────  ──────
//	total    412       28,592
type Table struct {
	Headers []string
	Align   []Align // one per column; missing ones default to Left
	Rows    [][]string
	Footer  [][]string // optional rows under a second rule (e.g. totals)
}

// Render writes the table to w, indented by one space.
func (t Table) Render(w io.Writer) {
	widths := make([]int, len(t.Headers))
	measure := func(rows [][]string) {
		for _, row := range rows {
			for i, cell := range row {
				if i < len(widths) {
					widths[i] = max(widths[i], Width(cell))
				}
			}
		}
	}
	measure([][]string{t.Headers})
	measure(t.Rows)
	measure(t.Footer)

	line := func(cells []string, style func(string) string) {
		parts := make([]string, len(widths))
		for i := range widths {
			cell := ""
			if i < len(cells) {
				cell = cells[i]
			}
			if i < len(t.Align) && t.Align[i] == Right {
				parts[i] = PadLeft(cell, widths[i])
			} else {
				parts[i] = Pad(cell, widths[i])
			}
		}
		fmt.Fprintln(w, " "+style(strings.TrimRight(strings.Join(parts, "  "), " ")))
	}
	rule := func() {
		parts := make([]string, len(widths))
		for i, wd := range widths {
			parts[i] = strings.Repeat("─", wd)
		}
		fmt.Fprintln(w, " "+Dim(strings.Join(parts, "  ")))
	}
	plain := func(s string) string { return s }

	line(t.Headers, Bold)
	rule()
	for _, row := range t.Rows {
		line(row, plain)
	}
	if len(t.Footer) > 0 {
		rule()
		for _, row := range t.Footer {
			line(row, plain)
		}
	}
}
