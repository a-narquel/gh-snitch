package render

// ColorEnabled turns colors on or off for the whole program. The cli package
// sets it at startup (off when output goes to a file or NO_COLOR is set).
var ColorEnabled = false

// ANSI color codes. The terminal reads "\x1b[36m" as "switch to cyan" and
// "\x1b[0m" as "back to normal".
const (
	reset  = "\x1b[0m"
	bold   = "\x1b[1m"
	dim    = "\x1b[2m"
	red    = "\x1b[31m"
	green  = "\x1b[32m"
	yellow = "\x1b[33m"
)

// palette gives each chart series its own color.
var palette = []string{"\x1b[36m", "\x1b[33m", "\x1b[35m", "\x1b[32m", "\x1b[34m", "\x1b[31m"}

func paint(code, s string) string {
	if !ColorEnabled || s == "" {
		return s
	}
	return code + s + reset
}

func Bold(s string) string   { return paint(bold, s) }
func Dim(s string) string    { return paint(dim, s) }
func Red(s string) string    { return paint(red, s) }
func Green(s string) string  { return paint(green, s) }
func Yellow(s string) string { return paint(yellow, s) }

// Tint paints s in the color of the i-th chart series.
func Tint(i int, s string) string {
	return paint(palette[i%len(palette)], s)
}
