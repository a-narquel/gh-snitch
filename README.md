# gh-snitch

A `gh` extension that shows who really carried the project: per-member commits,
lines, surviving code, PRs and reviews, drawn in ASCII in your terminal.

It reads history from your local clone (`git log`, `git blame`) and PRs/reviews
from the GitHub API using your existing `gh` login, so it works on private
repos without a paid plan.

## Install

```sh
gh extension install a-narquel/gh-snitch
# or, from a clone of this repo:
go build && gh extension install .
```

## Use

Run it inside any git repository:

```sh
gh snitch                          # interactive prompt
gh snitch who                      # one command, then exit
gh snitch --since 2w --exclude '*.proto' who
```

| Command | What it shows |
|---|---|
| `who [--all]` | leaderboard: commits, +/- lines, surviving code, active days, PRs, reviews, bus factor |
| `show <member>` | one member's profile: streaks, weekly sparkline, where they work, commit types |
| `compare <a> <b>` | two members side by side |
| `activity [--daily\|--weekly\|--monthly] [member…]` | commits over time (git tags show as ▲) |
| `when [member]` | punch card of commit days and hours |
| `files [--top N]` | hotspots: most-changed files and who owns them |
| `reviews` | who reviews whom, time to first review and to merge, self-merges |
| `awards` | the fun stuff |
| `since`, `until`, `exclude`, `include`, `bots`, `reset` | filters that persist in the prompt |

Members are matched loosely: `show andre` finds "André Narquel". To merge one
person's several emails into one member, add a [`.mailmap`](https://git-scm.com/docs/gitmailmap).

Flags: `--since`, `--until`, `--exclude` (repeatable), `--no-github`, `--version`.

## Layout

```
main.go              entry point: calls cli.Run
internal/gitlog/     runs git log / git blame and parses the output
internal/github/     PRs and reviews (GraphQL), git email → GitHub login (REST)
internal/stats/      pure functions: data in, numbers out
internal/render/     tables, bars, sparklines, line chart, colors
internal/cli/        loading spinners, the prompt, every command
```

Run the tests with `go test ./...`.
