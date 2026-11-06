// Package gitlog reads commit history from a local git repository.
package gitlog

import "time"

// FileChange is one file touched by a commit (one line of `git log --numstat`).
type FileChange struct {
	Path    string
	Added   int
	Deleted int
}

// Commit is a single commit with the files it changed.
type Commit struct {
	Hash    string
	Author  string
	Email   string
	When    time.Time
	Subject string
	Files   []FileChange
}
