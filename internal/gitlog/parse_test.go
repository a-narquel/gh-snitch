package gitlog

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

// sampleLog imitates the output of `git log` in logFormat with --numstat:
// two commits, the second one with a binary file ("-" instead of numbers).
// "\x1e" starts each commit and "\x1f" separates the header fields.
const sampleLog = "\x1e" +
	"aaa111\x1fandre\x1fandre@example.com\x1f2026-11-06T16:11:34Z\x1fAdd parser\n" +
	"\n" +
	"10\t2\tinternal/gitlog/parse.go\n" +
	"3\t0\tmain.go\n" +
	"\x1e" +
	"bbb222\x1fjoao\x1fjoao@example.com\x1f2026-11-05T09:00:00Z\x1fAdd logo\n" +
	"\n" +
	"-\t-\tdocs/logo.png\n"

func TestParseLog(t *testing.T) {
	commits, err := parseLog(sampleLog)
	if err != nil {
		t.Fatalf("parseLog: %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("got %d commits, want 2", len(commits))
	}

	c := commits[0]
	if c.Hash != "aaa111" {
		t.Errorf("Hash = %q, want %q", c.Hash, "aaa111")
	}
	if c.Author != "andre" {
		t.Errorf("Author = %q, want %q", c.Author, "andre")
	}
	if c.Subject != "Add parser" {
		t.Errorf("Subject = %q, want %q", c.Subject, "Add parser")
	}
	wantWhen := time.Date(2026, 11, 6, 16, 11, 34, 0, time.UTC)
	if !c.When.Equal(wantWhen) {
		t.Errorf("When = %v, want %v", c.When, wantWhen)
	}

	// reflect.DeepEqual compares slices and structs field by field, like
	// assertEquals on two lists in JUnit. (== doesn't work on slices.)
	wantFiles := []FileChange{
		{Path: "internal/gitlog/parse.go", Added: 10, Deleted: 2},
		{Path: "main.go", Added: 3, Deleted: 0},
	}
	if !reflect.DeepEqual(c.Files, wantFiles) {
		t.Errorf("Files = %+v, want %+v", c.Files, wantFiles)
	}

	bin := commits[1].Files[0]
	if bin.Added != 0 || bin.Deleted != 0 {
		t.Errorf("binary file = +%d/-%d, want +0/-0", bin.Added, bin.Deleted)
	}
}

func TestParseLogErrors(t *testing.T) {
	// A table-driven test: a list of cases, one loop. It's the most common
	// test style in Go, and t.Run gives each case its own name in the output.
	tests := []struct {
		name string
		raw  string
	}{
		{"too few header fields", "\x1eaaa\x1fandre\n"},
		{"bad date", "\x1eaaa\x1fandre\x1fa@b.c\x1fyesterday\x1fmsg\n"},
		{"bad numstat", "\x1eaaa\x1fandre\x1fa@b.c\x1f2026-11-06T16:11:34Z\x1fmsg\n\nten\t2\tx.go\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseLog(tt.raw); err == nil {
				t.Errorf("parseLog(%q) returned no error", tt.raw)
			}
		})
	}
}

func TestParseLogEmpty(t *testing.T) {
	commits, err := parseLog("")
	if err != nil || len(commits) != 0 {
		t.Errorf("parseLog(\"\") = %v, %v; want no commits and no error", commits, err)
	}
}

func TestRenamedPath(t *testing.T) {
	tests := map[string]string{ // input → want
		"plain.go":                "plain.go",
		"old.go => new.go":        "new.go",
		"src/{a => b}/f.go":       "src/b/f.go",
		"src/{old => }/f.go":      "src/f.go",
		"{ => pkg}/f.go":          "pkg/f.go",
		"docs/{intro.md => a.md}": "docs/a.md",
	}
	for in, want := range tests {
		if got := renamedPath(in); got != want {
			t.Errorf("renamedPath(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestLoadAndBlame builds a real (tiny) git repository in a temp folder and
// runs Load and Blame on it. It's slower than the tests above, but it checks
// that our git commands and parsing work together.
func TestLoadAndBlame(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir() // deleted automatically when the test ends

	run := func(env []string, args ...string) {
		t.Helper() // failures point at the caller's line, not this one
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), env...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	as := func(name, email, date string) []string {
		return []string{
			"GIT_AUTHOR_NAME=" + name, "GIT_AUTHOR_EMAIL=" + email, "GIT_AUTHOR_DATE=" + date,
			"GIT_COMMITTER_NAME=" + name, "GIT_COMMITTER_EMAIL=" + email, "GIT_COMMITTER_DATE=" + date,
		}
	}
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	run(nil, "init", "-q")
	write("a.txt", "one\ntwo\nthree\n")
	run(nil, "add", ".")
	run(as("Maria", "maria@example.com", "2026-10-01T10:00:00Z"), "commit", "-q", "-m", "first")

	write("a.txt", "one\nTWO\nthree\nfour\n")
	write(".mailmap", "Maria <maria@example.com> <maria@work.example.com>\n")
	run(nil, "add", ".")
	run(as("maria (work)", "maria@work.example.com", "2026-10-02T23:30:00Z"), "commit", "-q", "-m", "second")

	commits, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("got %d commits, want 2", len(commits))
	}
	// .mailmap maps the work email to "Maria", so both commits share an author.
	for _, c := range commits {
		if c.Author != "Maria" {
			t.Errorf("commit %q: Author = %q, want %q (via .mailmap)", c.Subject, c.Author, "Maria")
		}
	}
	if commits[0].Subject != "second" {
		t.Errorf("newest commit = %q, want %q", commits[0].Subject, "second")
	}

	blame, err := Blame(dir, nil)
	if err != nil {
		t.Fatalf("Blame: %v", err)
	}
	lines := 0
	for _, fb := range blame {
		lines += len(fb.Lines)
	}
	// a.txt has 4 lines and .mailmap has 1.
	if lines != 5 {
		t.Errorf("blamed %d lines, want 5", lines)
	}
}

func TestLoadNotARepo(t *testing.T) {
	if _, err := Load(t.TempDir()); err == nil {
		t.Error("Load on a folder that isn't a git repository returned no error")
	}
}
