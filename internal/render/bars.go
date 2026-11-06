package render

import (
	"math"
	"strings"
)

// eighths are block characters from 1/8 to 8/8 wide. Using them lets a bar
// grow in steps of 1/8 of a column instead of whole columns: "████▋".
var eighths = []rune{'▏', '▎', '▍', '▌', '▋', '▊', '▉', '█'}

// Bar draws value as a horizontal bar, where max fills width columns.
func Bar(value, max float64, width int) string {
	if max <= 0 || value <= 0 {
		return ""
	}
	n := int(math.Round(value / max * float64(width) * 8)) // size in eighths
	n = min(n, width*8)
	if n == 0 {
		n = 1 // anything above zero gets at least a sliver
	}
	bar := strings.Repeat("█", n/8)
	if rem := n % 8; rem > 0 {
		bar += string(eighths[rem-1])
	}
	return bar
}

// levels are the heights a sparkline can use, lowest first.
var levels = []rune("▁▂▃▄▅▆▇█")

// Spark draws a tiny chart, one character per value: ▁▁▂▂▃▅▂▁▃▄▇
// Zero is always the lowest bar; any value above zero is at least one step up,
// so you can tell "nothing" from "a little".
func Spark(values []int) string {
	top := 0
	for _, v := range values {
		top = max(top, v)
	}
	var b strings.Builder
	for _, v := range values {
		if v <= 0 || top == 0 {
			b.WriteRune(levels[0])
			continue
		}
		i := 1 + int(math.Round(float64(v)/float64(top)*float64(len(levels)-2)))
		b.WriteRune(levels[min(i, len(levels)-1)])
	}
	return b.String()
}
