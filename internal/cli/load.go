package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/a-narquel/gh-snitch/internal/github"
	"github.com/a-narquel/gh-snitch/internal/gitlog"
	"github.com/a-narquel/gh-snitch/internal/render"
)

// maxPRs caps how many pull requests we download (50 per API call).
const maxPRs = 1000

const banner = `   ┌─┐┌┐┌┬┌┬┐┌─┐┬ ┬
   └─┐││││ │ │  ├─┤   gh-snitch %s
   └─┘┘└┘┴ ┴ └─┘┴ ┴   who really carried the project?
`

// loadOptions controls what load fetches and how it reports progress.
type loadOptions struct {
	dir      string
	noGitHub bool
	progress io.Writer // where the spinner lines go (stderr)
	animate  bool      // spin, or just print the final lines (when not a terminal)
}

// load reads everything snitch needs into s. Only a git failure is fatal:
// without GitHub or blame, the commands that need them say so.
func (s *Session) load(opt loadOptions) error {
	err := step(opt, "Reading git history…", func(progress func(string)) (string, error) {
		commits, err := gitlog.Load(opt.dir)
		if err != nil {
			return "", err
		}
		s.data.Commits = commits
		return render.Plural(len(commits), "commit"), nil
	})
	if err != nil {
		return err
	}

	s.branch, _ = gitlog.CurrentBranch(opt.dir) // "_" ignores the error: no branch name is fine
	s.tags, _ = gitlog.Tags(opt.dir)

	repo, repoErr := github.CurrentRepo()
	if repoErr == nil {
		s.repo = repo.Owner + "/" + repo.Name
	} else if abs, err := filepath.Abs(opt.dir); err == nil {
		s.repo = filepath.Base(abs)
	}

	switch {
	case opt.noGitHub:
		s.ghNote = "GitHub data is off (--no-github)"
	case repoErr != nil:
		s.ghNote = "no GitHub remote found"
		skip(opt, "Fetching PRs/reviews…", s.ghNote)
	default:
		err := step(opt, "Fetching PRs/reviews…", func(progress func(string)) (string, error) {
			prs, err := github.FetchPRs(repo, maxPRs)
			if err != nil {
				return "", err
			}
			progress("linking authors…")
			names, err := s.linkLogins(repo)
			if err != nil {
				return "", err
			}
			s.data.PRs, s.data.HasPRs, s.data.Names = prs, true, names
			return render.Plural(len(prs), "PR"), nil
		})
		if err != nil {
			s.ghNote = "couldn't load GitHub data: " + firstLine(err.Error())
		}
	}

	_ = step(opt, "Running blame…", func(progress func(string)) (string, error) {
		blame, err := gitlog.Blame(opt.dir, func(done, total int) {
			progress(fmt.Sprintf("%d/%d files", done, total))
		})
		if err != nil {
			return "", err
		}
		s.data.Blame = blame
		return render.Plural(len(blame), "file"), nil
	})

	s.buildShortNames()
	return nil
}

// linkLogins maps GitHub logins to git author names, through the commit emails.
func (s *Session) linkLogins(repo github.Repo) (map[string]string, error) {
	commitFor := map[string]string{} // email → one commit made with it
	authorOf := map[string]string{}  // email → git author name
	for _, c := range s.data.Commits {
		if _, ok := commitFor[c.Email]; !ok {
			commitFor[c.Email] = c.Hash
			authorOf[c.Email] = c.Author
		}
	}

	logins, err := github.ResolveLogins(repo, commitFor)
	if err != nil {
		return nil, err
	}
	names := map[string]string{}
	for email, login := range logins {
		names[login] = authorOf[email]
	}
	return names, nil
}

// step runs fn while showing a spinner, then prints the result line:
//
//	⠿ Reading git history…   412 commits ✓
//
// fn gets a progress function it can call to update the text next to the
// spinner (e.g. "120/214 files").
func step(opt loadOptions, label string, fn func(progress func(string)) (string, error)) error {
	var status atomic.Value // holds a string; safe to read and write from two goroutines
	status.Store("")

	stop := make(chan struct{})
	stopped := make(chan struct{})

	if opt.animate {
		// A goroutine is a lightweight thread: `go f()` starts f and returns
		// immediately. This one redraws the spinner until we close stop.
		go func() {
			defer close(stopped)
			frames := []rune("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏")
			tick := time.NewTicker(80 * time.Millisecond)
			defer tick.Stop()
			for i := 0; ; i++ {
				select { // waits for whichever channel is ready first
				case <-stop:
					return
				case <-tick.C:
					// \r goes back to the start of the line; \x1b[K clears it.
					fmt.Fprintf(opt.progress, "\r\x1b[K %c %s %s", frames[i%len(frames)],
						render.Pad(label, 24), render.Dim(status.Load().(string)))
				}
			}
		}()
	} else {
		close(stopped)
	}

	detail, err := fn(func(s string) { status.Store(s) })
	close(stop)
	<-stopped // wait for the spinner goroutine to finish before printing

	if opt.animate {
		fmt.Fprint(opt.progress, "\r\x1b[K")
	}
	if err != nil {
		fmt.Fprintf(opt.progress, " %s %s %s\n", render.Red("✗"), render.Pad(label, 24),
			render.Red(render.Trunc(firstLine(err.Error()), 60)))
		return err
	}
	fmt.Fprintf(opt.progress, " ⠿ %s %s %s\n", render.Pad(label, 24), detail, render.Green("✓"))
	return nil
}

// skip prints a step that didn't run.
func skip(opt loadOptions, label, why string) {
	fmt.Fprintf(opt.progress, " %s %s %s\n", render.Dim("–"), render.Pad(label, 24), render.Dim("skipped: "+why))
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return line
}

// isTerminal reports whether f is an interactive terminal (not a pipe or file).
func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
