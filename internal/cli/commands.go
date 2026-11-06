package cli

import (
	"fmt"
	"strings"

	"github.com/a-narquel/gh-snitch/internal/render"
)

// command is one thing the user can type at the prompt.
type command struct {
	name    string
	aliases []string
	usage   string // arguments, for help: "<member>"
	help    string
	run     func(s *Session, args []string) error
}

// commands is filled in init() rather than directly, because `help` needs
// to read this list: a variable can't refer to itself in its own initializer.
var commands []command

func init() {
	commands = []command{
		{name: "who", usage: "[--all]", help: "leaderboard: commits, lines, surviving code, PRs, reviews", run: cmdWho},
		{name: "show", usage: "<member>", help: "one member's profile", run: cmdShow},
		{name: "compare", usage: "<member> <member>", help: "two members side by side", run: cmdCompare},
		{name: "activity", usage: "[--daily|--weekly|--monthly] [member…]", help: "commits over time", run: cmdActivity},
		{name: "when", usage: "[member]", help: "punch card: which days and hours people commit", run: cmdWhen},
		{name: "files", usage: "[--top N]", help: "hotspots: the files that change the most", run: cmdFiles},
		{name: "reviews", help: "who reviews whom, review and merge times", run: cmdReviews},
		{name: "awards", help: "the fun stuff", run: cmdAwards},
		{name: "since", usage: "<when>", help: "filter: only from then on (2w, 3d, 1m, 1y, 2026-10-01)", run: cmdSince},
		{name: "until", usage: "<when>", help: "filter: only up to then", run: cmdUntil},
		{name: "exclude", usage: "<glob>…", help: "filter: ignore matching files (*.proto, vendor/, go.sum)", run: cmdExclude},
		{name: "include", usage: "<glob>", help: "undo an exclude", run: cmdInclude},
		{name: "bots", usage: "[on|off]", help: "filter: show or hide bots like dependabot[bot] (hidden by default)", run: cmdBots},
		{name: "reset", help: "clear all filters", run: cmdReset},
		{name: "help", aliases: []string{"?"}, help: "this list", run: cmdHelp},
		{name: "clear", help: "clear the screen", run: cmdClear},
	}
}

func lookup(name string) (command, bool) {
	name = strings.ToLower(name)
	for _, c := range commands {
		if c.name == name {
			return c, true
		}
		for _, a := range c.aliases {
			if a == name {
				return c, true
			}
		}
	}
	return command{}, false
}

func cmdHelp(s *Session, args []string) error {
	width := 0
	for _, c := range commands {
		width = max(width, render.Width(c.name+" "+c.usage))
	}
	fmt.Fprintln(s.out, render.Bold(" Commands"))
	for _, c := range commands {
		fmt.Fprintf(s.out, "   %s  %s\n", render.Pad(c.name+" "+render.Dim(c.usage), width), c.help)
	}
	fmt.Fprintf(s.out, "   %s  %s\n", render.Pad("quit", width), "leave (or Ctrl+D)")
	fmt.Fprintln(s.out)
	fmt.Fprintln(s.out, render.Dim(" Filters stick until 'reset' and show in the prompt: snitch [2w, -proto]>"))
	fmt.Fprintln(s.out, render.Dim(" Any command also works from the shell: gh snitch --since 2w who"))
	return nil
}

func cmdClear(s *Session, args []string) error {
	fmt.Fprint(s.out, "\x1b[H\x1b[2J") // move the cursor home, clear the screen
	return nil
}
