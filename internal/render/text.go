// Package render draws things in the terminal: tables, bars, charts.
// It knows nothing about git or GitHub; it only gets numbers and strings.
package render

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rivo/uniseg"
)

// ansi matches color escape codes like "\x1b[36m", which take no space on screen.
var ansi = regexp.MustCompile("\x1b\\[[0-9;]*m")

// Width is how many terminal columns s takes.
//
// len(s) counts bytes, which is wrong for "█" (3 bytes, 1 column) and
// "🦉" (4 bytes, 2 columns). uniseg knows the real width of each character.
func Width(s string) int {
	return uniseg.StringWidth(ansi.ReplaceAllString(s, ""))
}

// Pad adds spaces on the right until s is w columns wide.
func Pad(s string, w int) string {
	return s + strings.Repeat(" ", max(0, w-Width(s)))
}

// PadLeft adds spaces on the left until s is w columns wide (right-aligns it).
func PadLeft(s string, w int) string {
	return strings.Repeat(" ", max(0, w-Width(s))) + s
}

// TruncLeft cuts the start of s so it fits in w columns: "…/paxos/Acceptor.java".
// Good for paths, where the end is the interesting part.
func TruncLeft(s string, w int) string {
	if Width(s) <= w {
		return s
	}
	r := []rune(s) // a rune is one Unicode character, like a Java char (but 32 bits)
	for len(r) > 0 && uniseg.StringWidth(string(r))+1 > w {
		r = r[1:]
	}
	return "…" + string(r)
}

// Trunc cuts the end of s so it fits in w columns: "André Narq…".
func Trunc(s string, w int) string {
	if Width(s) <= w {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && uniseg.StringWidth(string(r))+1 > w {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}

// Int formats n with thousands separators: 14203 → "14,203".
func Int(n int) string {
	s := strconv.Itoa(n)
	if n < 0 {
		return "-" + Int(-n)
	}
	var b strings.Builder // like Java's StringBuilder
	for i, ch := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(ch)
	}
	return b.String()
}

// Short formats big numbers compactly: 198 → "198", 14203 → "14.2k".
func Short(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	default:
		return strconv.Itoa(n)
	}
}

// Pct formats a 0..100 percentage: 48.3 → "48%".
func Pct(p float64) string {
	return fmt.Sprintf("%.0f%%", p) // %% prints a literal %
}

// Date formats a date the short way: "Sep 14", or "Sep 14 2024" when it
// isn't this year.
func Date(t time.Time) string {
	if t.Year() != time.Now().Year() {
		return t.Format("Jan 2 2006")
	}
	return t.Format("Jan 2")
}

// Duration formats a duration for humans: "45m", "3h 20m", "2d 4h".
func Duration(d time.Duration) string {
	d = d.Round(time.Minute)
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	switch {
	case days > 0:
		return fmt.Sprintf("%dd %dh", days, hours)
	case hours > 0:
		return fmt.Sprintf("%dh %02dm", hours, mins)
	default:
		return fmt.Sprintf("%dm", mins)
	}
}

// Plural returns "1 day" / "3 days".
func Plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}
