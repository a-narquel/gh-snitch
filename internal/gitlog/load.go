package gitlog

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// logFormat tells git how to print each commit header:
//
//	%x1e = record separator (marks the start of a commit)
//	%x1f = field separator (between fields)
//	%H hash · %aN author · %aE email · %aI date (ISO 8601) · %s subject
//
// %aN/%aE (capital N/E) apply .mailmap, so one person with several emails
// shows up as a single author.
const logFormat = "--format=%x1e%H%x1f%aN%x1f%aE%x1f%aI%x1f%s"

// Load reads the commit history of the git repository in repoDir.
func Load(repoDir string) ([]Commit, error) {
	cmd := exec.Command("git", "log", "--no-merges", "--numstat", logFormat)
	cmd.Dir = repoDir

	out, err := cmd.Output()
	if err != nil {
		// When git itself fails (e.g. "not a git repository"), its message is in
		// the ExitError's Stderr. Put it in our error so the user can read it.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			msg := strings.TrimSpace(string(exitErr.Stderr))
			return nil, fmt.Errorf("running git log: %s", msg)
		}
		return nil, fmt.Errorf("running git log: %w", err)
	}

	return parseLog(string(out))
}
