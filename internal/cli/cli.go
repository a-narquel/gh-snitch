package cli

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/cli/go-gh/v2/pkg/term"

	"github.com/a-narquel/gh-snitch/internal/render"
)

// Version is shown in the banner and by --version.
const Version = "v0.1.0"

// Run is the whole program. It returns the exit code (0 = success), which
// main passes to os.Exit. Keeping os.Exit out of here makes Run testable.
//
//	gh snitch                 → interactive prompt
//	gh snitch who             → run one command and exit
//	gh snitch --since 2w who  → same, with a filter
func Run(args []string) int {
	flags := flag.NewFlagSet("gh snitch", flag.ContinueOnError)
	noGitHub := flags.Bool("no-github", false, "don't fetch PRs and reviews from GitHub")
	showVersion := flags.Bool("version", false, "print the version and exit")
	since := flags.String("since", "", "only look at commits since `WHEN` (2w, 3d, 1m, 2026-10-01)")
	until := flags.String("until", "", "only look at commits until `WHEN`")
	var excludes []string
	flags.Func("exclude", "ignore files matching `GLOB` (repeatable)", func(v string) error {
		excludes = append(excludes, v)
		return nil
	})
	flags.Usage = func() {
		fmt.Fprintf(flags.Output(), "Usage: gh snitch [flags] [command [args]]\n\n"+
			"Without a command, starts an interactive prompt (type 'help' there).\n\nFlags:\n")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if *showVersion {
		fmt.Println("gh-snitch", Version)
		return 0
	}

	t := term.FromEnv()
	render.ColorEnabled = t.IsColorEnabled()
	interactive := flags.NArg() == 0

	s := &Session{out: os.Stdout, now: time.Now}

	if interactive {
		fmt.Printf(banner+"\n", Version)
	}
	err := s.load(loadOptions{
		dir:      ".",
		noGitHub: *noGitHub,
		progress: os.Stderr,
		animate:  isTerminal(os.Stderr),
	})
	if err != nil {
		return 1
	}

	// Flags become filters, exactly as if the user had typed them in the prompt.
	if *since != "" {
		if err := s.setSince(*since, false); err != nil {
			s.fail(err)
			return 2
		}
	}
	if *until != "" {
		if err := s.setUntil(*until, false); err != nil {
			s.fail(err)
			return 2
		}
	}
	s.filter.Exclude = append(s.filter.Exclude, excludes...)

	if !interactive {
		fmt.Fprintln(os.Stderr)
		if err := s.exec(flags.Args()); err != nil {
			s.fail(err)
			return 1
		}
		return 0
	}

	s.header()
	s.repl(os.Stdin)
	return 0
}

// header is the line under the loading steps:
//
//	a-narquel/dida-trade · branch main · Sep 14 → Nov 6 (53 days)
func (s *Session) header() {
	parts := []string{render.Bold(s.repo)}
	if s.branch != "" {
		parts = append(parts, "branch "+s.branch)
	}
	parts = append(parts, period(s.data))
	fmt.Fprintf(s.out, "\n %s\n", strings.Join(parts, " · "))
	if s.ghNote != "" {
		fmt.Fprintf(s.out, " %s\n", render.Dim("("+s.ghNote+": PR and review numbers are off)"))
	}
	fmt.Fprintf(s.out, " Type %s for commands.\n\n", render.Bold("'help'"))
}

// repl is the Read-Eval-Print Loop: show the prompt, read a line, run it, repeat.
func (s *Session) repl(in io.Reader) {
	sc := bufio.NewScanner(in)
	for {
		fmt.Fprint(s.out, s.prompt())
		if !sc.Scan() { // false on Ctrl+D (end of input) or a read error
			fmt.Fprintln(s.out)
			return
		}

		args := strings.Fields(sc.Text())
		if len(args) == 0 {
			continue
		}
		if args[0] == "quit" || args[0] == "exit" || args[0] == "q" {
			return
		}

		fmt.Fprintln(s.out)
		if err := s.exec(args); err != nil {
			s.fail(err)
		}
		fmt.Fprintln(s.out)
	}
}

// exec runs one command line, already split into words.
func (s *Session) exec(args []string) error {
	cmd, ok := lookup(args[0])
	if !ok {
		return fmt.Errorf("unknown command %q (type 'help')", args[0])
	}
	return cmd.run(s, args[1:])
}
