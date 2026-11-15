package cli

import "strings"

// command is something that can be run straight from the shell:
// gh snitch who, gh snitch show andre…
// The interactive menu (tui.go) calls the same functions.
type command struct {
	name  string
	usage string // arguments, for --help: "<member>"
	help  string
	run   func(s *Session, args []string) error
}

var commands = []command{
	{name: "who", help: "leaderboard: commits, lines, surviving code, PRs, reviews", run: cmdWho},
	{name: "show", usage: "<member>", help: "one member's profile", run: cmdShow},
	{name: "activity", help: "commits over time", run: cmdActivity},
	{name: "awards", help: "the fun stuff", run: cmdAwards},
	{name: "me", usage: "[login]", help: "your all-time GitHub stats (works outside a repo)", run: cmdMe},
}

func lookup(name string) (command, bool) {
	name = strings.ToLower(name)
	for _, c := range commands {
		if c.name == name {
			return c, true
		}
	}
	return command{}, false
}
