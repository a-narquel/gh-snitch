// Package github fetches pull requests and reviews through the GitHub API,
// using the login that `gh` already has (no extra token needed).
package github

import (
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/cli/go-gh/v2/pkg/repository"
)

// PR is a pull request with its reviews. Author fields are GitHub logins.
type PR struct {
	Number    int
	Title     string
	State     string // OPEN, CLOSED or MERGED
	Author    string
	MergedBy  string
	CreatedAt time.Time
	MergedAt  time.Time // zero if not merged
	ClosedAt  time.Time // zero if still open
	Reviews   []Review
}

// Review is one submitted review on a PR.
type Review struct {
	Author      string
	State       string // APPROVED, CHANGES_REQUESTED, COMMENTED, DISMISSED
	SubmittedAt time.Time
}

// Repo is the GitHub repository of the current directory (from its git remotes).
type Repo = repository.Repository

// CurrentRepo finds the GitHub repository for the current directory.
func CurrentRepo() (Repo, error) {
	return repository.Current()
}

const prQuery = `
query($owner: String!, $name: String!, $cursor: String) {
  repository(owner: $owner, name: $name) {
    pullRequests(first: 50, after: $cursor, orderBy: {field: CREATED_AT, direction: DESC}) {
      pageInfo { hasNextPage endCursor }
      nodes {
        number title state createdAt mergedAt closedAt
        author { login __typename }
        mergedBy { login }
        reviews(first: 50) {
          nodes { author { login __typename } state submittedAt }
        }
      }
    }
  }
}`

// The response types mirror the JSON shape of the query. Go's JSON decoder
// matches field names case-insensitively, so "createdAt" fills CreatedAt.
// A JSON null leaves the field at its zero value (empty login, zero time).
type login struct {
	Login string
	// A struct tag tells the JSON decoder which key to read, for names that
	// can't be a Go field name (a leading "_" would also make it private).
	Typename string `json:"__typename"`
}

// name returns the login, with "[bot]" added for bots ("dependabot[bot]"),
// which is also how bots appear as git authors.
func (l login) name() string {
	if l.Typename == "Bot" && l.Login != "" {
		return l.Login + "[bot]"
	}
	return l.Login
}

type prNode struct {
	Number    int
	Title     string
	State     string
	CreatedAt time.Time
	MergedAt  time.Time
	ClosedAt  time.Time
	Author    login
	MergedBy  login
	Reviews   struct {
		Nodes []struct {
			Author      login
			State       string
			SubmittedAt time.Time
		}
	}
}

type prPage struct {
	Repository struct {
		PullRequests struct {
			PageInfo struct {
				HasNextPage bool
				EndCursor   string
			}
			Nodes []prNode
		}
	}
}

// FetchPRs returns up to max pull requests, newest first.
func FetchPRs(repo Repo, max int) ([]PR, error) {
	client, err := api.NewGraphQLClient(api.ClientOptions{Host: repo.Host})
	if err != nil {
		return nil, err
	}

	var prs []PR
	var cursor *string // nil is sent as JSON null, which means "first page"
	for len(prs) < max {
		var page prPage
		vars := map[string]any{"owner": repo.Owner, "name": repo.Name, "cursor": cursor}
		if err := client.Do(prQuery, vars, &page); err != nil {
			return nil, fmt.Errorf("fetching pull requests: %w", err)
		}

		conn := page.Repository.PullRequests
		for _, n := range conn.Nodes {
			pr := PR{
				Number:    n.Number,
				Title:     n.Title,
				State:     n.State,
				Author:    n.Author.name(),
				MergedBy:  n.MergedBy.Login,
				CreatedAt: n.CreatedAt,
				MergedAt:  n.MergedAt,
				ClosedAt:  n.ClosedAt,
			}
			for _, r := range n.Reviews.Nodes {
				pr.Reviews = append(pr.Reviews, Review{
					Author:      r.Author.name(),
					State:       r.State,
					SubmittedAt: r.SubmittedAt,
				})
			}
			prs = append(prs, pr)
		}

		if !conn.PageInfo.HasNextPage {
			break
		}
		cursor = &conn.PageInfo.EndCursor
	}

	if len(prs) > max {
		prs = prs[:max]
	}
	return prs, nil
}

// noreply matches GitHub's private emails: "12345+login@users.noreply.github.com".
var noreply = regexp.MustCompile(`^(?:\d+\+)?([^@]+)@users\.noreply\.github\.com$`)

// ResolveLogins finds the GitHub login behind each git email.
//
// commitFor maps each email to the hash of one commit made with it. For
// noreply emails the login is in the address; for the rest we ask the API
// who authored that commit. Emails that can't be resolved are left out.
func ResolveLogins(repo Repo, commitFor map[string]string) (map[string]string, error) {
	client, err := api.NewRESTClient(api.ClientOptions{Host: repo.Host})
	if err != nil {
		return nil, err
	}

	logins := map[string]string{}
	var mu sync.Mutex // guards logins: several goroutines write to it
	var wg sync.WaitGroup
	limit := make(chan struct{}, 4) // at most 4 requests at the same time

	for email, sha := range commitFor {
		if m := noreply.FindStringSubmatch(email); m != nil {
			logins[email] = m[1]
			continue
		}

		wg.Go(func() {
			limit <- struct{}{}        // take a slot (blocks if 4 are busy)
			defer func() { <-limit }() // give it back when done

			var resp struct{ Author *login } // pointer: null when GitHub doesn't know the email
			path := fmt.Sprintf("repos/%s/%s/commits/%s", repo.Owner, repo.Name, sha)
			if err := client.Get(path, &resp); err != nil || resp.Author == nil {
				return
			}
			mu.Lock()
			logins[email] = resp.Author.Login
			mu.Unlock()
		})
	}
	wg.Wait()
	return logins, nil
}
