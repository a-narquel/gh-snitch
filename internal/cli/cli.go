package cli

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/cli/go-gh/v2/pkg/term"

	"github.com/a-narquel/gh-snitch/internal/render"
)

// Version is shown in the banner and by --version.
const Version = "v0.2.1"

// Run is the whole program. It returns the exit code (0 = success), which
// main passes to os.Exit. Keeping os.Exit out of here makes Run testable.
//
//	gh snitch                 → arrow-key menu
//	gh snitch who             → run one command and exit
//	gh snitch --since 2w who  → same, with a filter
func Run(args []string) int {
	flags := flag.NewFlagSet("gh snitch", flag.ContinueOnError)
	noGitHub := flags.Bool("no-github", false, "don't fetch PRs and reviews from GitHub")
	showVersion := flags.Bool("version", false, "print the version and exit")
	since := flags.String("since", "", "only look at commits since `WHEN` (2w, 3d, 1m, 2026-10-01)")
	var excludes []string
	flags.Func("exclude", "ignore files matching `GLOB` (repeatable)", func(v string) error {
		excludes = append(excludes, v)
		return nil
	})
	flags.Usage = func() {
		out := flags.Output()
		fmt.Fprintf(out, "Usage: gh snitch [flags] [command]\n\n"+
			"Without a command, opens a menu you move around with the arrow keys.\n\nCommands:\n")
		for _, c := range commands {
			fmt.Fprintf(out, "  %-18s %s\n", c.name+" "+c.usage, c.help)
		}
		fmt.Fprintf(out, "\nFlags:\n")
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

	s := &Session{out: os.Stdout, now: time.Now}

	// `gh snitch me` is about the user, not a repo: skip loading git history,
	// so it also works outside a git repository.
	if flags.Arg(0) == "me" {
		if err := s.exec(flags.Args()); err != nil {
			s.fail(err)
			return 1
		}
		return 0
	}

	// The menu needs a real terminal. When output goes to a pipe or a file
	// (gh snitch > out.txt), print the leaderboard instead.
	menu := flags.NArg() == 0 && isTerminal(os.Stdin) && isTerminal(os.Stdout)
	cmdArgs := flags.Args()
	if flags.NArg() == 0 && !menu {
		cmdArgs = []string{"who"}
	}

	if menu {
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

	// Flags become filters, exactly as if they had been picked in the menu.
	if *since != "" {
		if err := s.setSince(*since); err != nil {
			s.fail(err)
			return 2
		}
	}
	if len(excludes) > 0 {
		s.addExcludes(excludes)
	}

	if menu {
		if err := runTUI(s); err != nil {
			s.fail(err)
			return 1
		}
		return 0
	}

	fmt.Fprintln(os.Stderr)
	if err := s.exec(cmdArgs); err != nil {
		s.fail(err)
		return 1
	}
	return 0
}

// exec runs one command line, already split into words.
func (s *Session) exec(args []string) error {
	cmd, ok := lookup(args[0])
	if !ok {
		return fmt.Errorf("unknown command %q (see gh snitch --help)", args[0])
	}
	return cmd.run(s, args[1:])
}
