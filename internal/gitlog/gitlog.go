package gitlog

import "time"

type FileChange struct {
	Path    string
	Added   int
	Deleted int
}

type Commit struct {
	Hash    string
	Author  string
	Email   string
	When    time.Time
	Subject string
	Files   []FileChange
}
