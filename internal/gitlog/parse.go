package gitlog

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	recordSep = "\x1e" // starts each commit (%x1e in logFormat)
	fieldSep  = "\x1f" // separates header fields (%x1f in logFormat)
)

// parseLog turns the raw text of `git log` (in logFormat, with --numstat)
// into commits. It does no I/O, so it can be tested with a hand-written string.
//
// Each record looks like:
//
//	<hash>␟<author>␟<email>␟<date>␟<subject>
//	(blank line)
//	<added>\t<deleted>\t<path>
//	...
func parseLog(raw string) ([]Commit, error) {
	var commits []Commit

	for _, record := range strings.Split(raw, recordSep) {
		if strings.TrimSpace(record) == "" {
			continue // the text starts with a separator, so the first piece is empty
		}

		c, err := parseCommit(record)
		if err != nil {
			return nil, err
		}
		commits = append(commits, c)
	}

	return commits, nil
}

// parseCommit parses one record: a header line followed by numstat lines.
func parseCommit(record string) (Commit, error) {
	header, body, _ := strings.Cut(record, "\n")

	fields := strings.Split(header, fieldSep)
	if len(fields) != 5 {
		return Commit{}, fmt.Errorf("bad commit header %q: want 5 fields, got %d", header, len(fields))
	}

	when, err := time.Parse(time.RFC3339, fields[3])
	if err != nil {
		return Commit{}, fmt.Errorf("commit %s: bad date: %w", fields[0], err)
	}

	c := Commit{
		Hash:    fields[0],
		Author:  fields[1],
		Email:   fields[2],
		When:    when,
		Subject: fields[4],
	}

	for _, line := range strings.Split(body, "\n") {
		if line == "" {
			continue // blank line between header and numstat
		}
		fc, err := parseNumstat(line)
		if err != nil {
			return Commit{}, fmt.Errorf("commit %s: %w", c.Hash, err)
		}
		c.Files = append(c.Files, fc)
	}

	return c, nil
}

// parseNumstat parses one line like "12\t3\tsrc/main.go".
// Binary files show "-\t-\tpath"; they count as 0 lines.
func parseNumstat(line string) (FileChange, error) {
	parts := strings.SplitN(line, "\t", 3)
	if len(parts) != 3 {
		return FileChange{}, fmt.Errorf("bad numstat line %q", line)
	}

	added, err := parseCount(parts[0])
	if err != nil {
		return FileChange{}, fmt.Errorf("bad numstat line %q: %w", line, err)
	}
	deleted, err := parseCount(parts[1])
	if err != nil {
		return FileChange{}, fmt.Errorf("bad numstat line %q: %w", line, err)
	}

	return FileChange{
		Path:    renamedPath(parts[2]),
		Added:   added,
		Deleted: deleted,
	}, nil
}

// parseCount reads a numstat number; "-" (binary file) becomes 0.
func parseCount(s string) (int, error) {
	if s == "-" {
		return 0, nil
	}
	return strconv.Atoi(s)
}

// renamedPath returns the new path of a renamed file. git writes renames as
// "old => new" or "dir/{old => new}/file"; any other path is returned as is.
func renamedPath(p string) string {
	if !strings.Contains(p, " => ") {
		return p
	}

	open := strings.Index(p, "{")
	close := strings.Index(p, "}")
	if open == -1 || close < open {
		_, newPath, _ := strings.Cut(p, " => ")
		return newPath
	}

	// "dir/{old => new}/file" → prefix "dir/", inside "old => new", suffix "/file"
	prefix, inside, suffix := p[:open], p[open+1:close], p[close+1:]
	_, newPart, _ := strings.Cut(inside, " => ")
	// "{old => }" means the file moved up a directory: avoid a double slash.
	return strings.ReplaceAll(prefix+newPart+suffix, "//", "/")
}
