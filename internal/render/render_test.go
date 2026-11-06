package render

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestInt(t *testing.T) {
	tests := map[int]string{0: "0", 7: "7", 999: "999", 1000: "1,000", 14203: "14,203", 1234567: "1,234,567", -4512: "-4,512"}
	for in, want := range tests {
		if got := Int(in); got != want {
			t.Errorf("Int(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestWidth(t *testing.T) {
	tests := map[string]int{
		"abc":                 3,
		"█▋":                  2, // 6 bytes, 2 columns
		"André":               5,
		"🦉":                   2, // emoji take two columns
		"\x1b[36mcyan\x1b[0m": 4, // color codes take no space
	}
	for in, want := range tests {
		if got := Width(in); got != want {
			t.Errorf("Width(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestBar(t *testing.T) {
	tests := []struct {
		value, max float64
		width      int
		want       string
	}{
		{10, 10, 4, "████"},
		{5, 10, 4, "██"},
		{1, 10, 4, "▍"}, // 0.4 columns ≈ 3 eighths
		{0, 10, 4, ""},
		{20, 10, 4, "████"}, // never wider than width
	}
	for _, tt := range tests {
		if got := Bar(tt.value, tt.max, tt.width); got != tt.want {
			t.Errorf("Bar(%v, %v, %d) = %q, want %q", tt.value, tt.max, tt.width, got, tt.want)
		}
	}
}

func TestSpark(t *testing.T) {
	if got, want := Spark([]int{0, 1, 4, 8}), "▁▃▅█"; got != want {
		t.Errorf("Spark = %q, want %q", got, want)
	}
}

func TestTrunc(t *testing.T) {
	if got := TruncLeft("src/server/paxos/Acceptor.java", 16); got != "…s/Acceptor.java" {
		t.Errorf("TruncLeft = %q", got)
	}
	if got := Trunc("André Narquel", 6); got != "André…" {
		t.Errorf("Trunc = %q", got)
	}
}

func TestDuration(t *testing.T) {
	tests := map[time.Duration]string{
		45 * time.Minute:              "45m",
		3*time.Hour + 20*time.Minute:  "3h 20m",
		52*time.Hour + 10*time.Minute: "2d 4h",
	}
	for in, want := range tests {
		if got := Duration(in); got != want {
			t.Errorf("Duration(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestTableAligns(t *testing.T) {
	var buf bytes.Buffer // a bytes.Buffer is an in-memory io.Writer: perfect for tests
	Table{
		Headers: []string{"NAME", "N"},
		Align:   []Align{Left, Right},
		Rows:    [][]string{{"André", "5"}, {"jo", "120"}},
	}.Render(&buf)

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("got %d lines, want 4:\n%s", len(lines), buf.String())
	}
	for _, l := range lines {
		if Width(l) != Width(lines[0]) && !strings.HasPrefix(l, " NAME") {
			t.Errorf("line %q has width %d, want %d", l, Width(l), Width(lines[0]))
		}
	}
	if !strings.HasSuffix(lines[2], "    5") {
		t.Errorf("numbers aren't right-aligned: %q", lines[2])
	}
}
