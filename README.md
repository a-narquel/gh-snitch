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
gh snitch
```

It opens a menu: move with the arrow keys, Enter to open, Esc to go back.

| Menu | What it shows |
|---|---|
| Leaderboard | commits, +/- lines, surviving code, active days, PRs, reviews, bus factor |
| Member | one person's profile: streaks, weekly sparkline, where they work, commit types |
| Activity | commits over time for the top 5 (git tags show as ▲) |
| Awards | the fun stuff |
| Filters | pick a period (last week, month…) and files to ignore (`*.lock`, `dist/`) |

Every screen also works straight from the shell, handy for scripts:

```sh
gh snitch who                       # or: show <member>, activity, awards
gh snitch --since 2w --exclude '*.lock' who
gh snitch me                        # your all-time GitHub stats (no repo needed)
```

Members are matched loosely: `show andre` finds "André Narquel". To merge one
person's several emails into one member, add a [`.mailmap`](https://git-scm.com/docs/gitmailmap).

Flags: `--since`, `--exclude` (repeatable), `--no-github`, `--version`.

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
