package github

import (
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/cli/go-gh/v2/pkg/auth"
)

// Profile is a GitHub user's all-time activity.
type Profile struct {
	Login     string
	Name      string
	CreatedAt time.Time
	Followers int
	Stars     int // stars on the repos they own
	Repos     int // repos they own (not forks)

	Years     []Year     // oldest first
	Days      []Day      // every day of every year, oldest first
	Languages []Language // bytes of code per language, across their own repos
}

// Year is one year of contributions.
type Year struct {
	Year     int
	Commits  int
	PRs      int
	Reviews  int
	Issues   int
	Private  int // contributions in private repos you can't see the details of
	TopRepos []RepoCount
}

// Total is everything they did that year.
func (y Year) Total() int {
	return y.Commits + y.PRs + y.Reviews + y.Issues + y.Private
}

// RepoCount is how many commits someone made to one repo.
type RepoCount struct {
	Name    string // owner/name
	Commits int
}

// Day is one square of the contribution calendar.
type Day struct {
	Date  time.Time // midnight, local time
	Count int
}

// Language is how much code a user has in one language.
type Language struct {
	Name  string
	Bytes int
}

// FetchProfile loads a user's profile. An empty login means "whoever is
// logged in to gh".
func FetchProfile(login string, progress func(string)) (Profile, error) {
	host, _ := auth.DefaultHost()
	client, err := api.NewGraphQLClient(api.ClientOptions{Host: host})
	if err != nil {
		return Profile{}, err
	}

	if login == "" {
		var resp struct{ Viewer struct{ Login string } }
		if err := client.Do(`query { viewer { login } }`, nil, &resp); err != nil {
			return Profile{}, fmt.Errorf("finding who is logged in: %w", err)
		}
		login = resp.Viewer.Login
	}

	p, years, err := fetchUser(client, login)
	if err != nil {
		return Profile{}, err
	}

	// One query per year, all at the same time: the API only accepts a
	// window of at most one year per contributionsCollection.
	progress(fmt.Sprintf("%d years…", len(years)))
	results := make([]yearResult, len(years))
	var wg sync.WaitGroup
	for i, y := range years {
		wg.Go(func() {
			// Each goroutine writes only its own slot of results, so no mutex is needed.
			results[i] = fetchYear(client, login, y)
		})
	}
	wg.Wait()

	for _, r := range results {
		if r.err != nil {
			return Profile{}, r.err
		}
		p.Years = append(p.Years, r.year)
		p.Days = append(p.Days, r.days...)
	}
	slices.SortFunc(p.Years, func(a, b Year) int { return a.Year - b.Year })
	slices.SortFunc(p.Days, func(a, b Day) int { return a.Date.Compare(b.Date) })

	progress("languages…")
	if err := fetchRepos(client, &p); err != nil {
		return Profile{}, err
	}
	return p, nil
}

const userQuery = `
query($login: String!) {
  user(login: $login) {
    login name createdAt
    followers { totalCount }
    contributionsCollection { contributionYears }
  }
}`

func fetchUser(client *api.GraphQLClient, login string) (Profile, []int, error) {
	var resp struct {
		User *struct { // pointer: null when the login doesn't exist
			Login, Name string
			CreatedAt   time.Time
			Followers   struct{ TotalCount int }

			ContributionsCollection struct{ ContributionYears []int }
		}
	}
	if err := client.Do(userQuery, map[string]any{"login": login}, &resp); err != nil {
		return Profile{}, nil, fmt.Errorf("fetching user %s: %w", login, err)
	}
	u := resp.User
	if u == nil {
		return Profile{}, nil, fmt.Errorf("no GitHub user called %q", login)
	}
	p := Profile{Login: u.Login, Name: u.Name, CreatedAt: u.CreatedAt, Followers: u.Followers.TotalCount}
	return p, u.ContributionsCollection.ContributionYears, nil
}

const yearQuery = `
query($login: String!, $from: DateTime!, $to: DateTime!) {
  user(login: $login) {
    contributionsCollection(from: $from, to: $to) {
      totalCommitContributions
      totalPullRequestContributions
      totalPullRequestReviewContributions
      totalIssueContributions
      restrictedContributionsCount
      contributionCalendar {
        weeks { contributionDays { date contributionCount } }
      }
      commitContributionsByRepository(maxRepositories: 5) {
        repository { nameWithOwner }
        contributions { totalCount }
      }
    }
  }
}`

type yearResult struct {
	year Year
	days []Day
	err  error
}

func fetchYear(client *api.GraphQLClient, login string, year int) yearResult {
	var resp struct {
		User struct {
			ContributionsCollection struct {
				TotalCommitContributions            int
				TotalPullRequestContributions       int
				TotalPullRequestReviewContributions int
				TotalIssueContributions             int
				RestrictedContributionsCount        int
				ContributionCalendar                struct {
					Weeks []struct {
						ContributionDays []struct {
							Date              string // "2026-11-07": not a full timestamp, so we parse it ourselves
							ContributionCount int
						}
					}
				}
				CommitContributionsByRepository []struct {
					Repository    struct{ NameWithOwner string }
					Contributions struct{ TotalCount int }
				}
			}
		}
	}
	vars := map[string]any{
		"login": login,
		"from":  time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
		"to":    time.Date(year, 12, 31, 23, 59, 59, 0, time.UTC).Format(time.RFC3339),
	}
	if err := client.Do(yearQuery, vars, &resp); err != nil {
		return yearResult{err: fmt.Errorf("fetching %d: %w", year, err)}
	}

	c := resp.User.ContributionsCollection
	r := yearResult{year: Year{
		Year:    year,
		Commits: c.TotalCommitContributions,
		PRs:     c.TotalPullRequestContributions,
		Reviews: c.TotalPullRequestReviewContributions,
		Issues:  c.TotalIssueContributions,
		Private: c.RestrictedContributionsCount,
	}}
	for _, repo := range c.CommitContributionsByRepository {
		r.year.TopRepos = append(r.year.TopRepos, RepoCount{repo.Repository.NameWithOwner, repo.Contributions.TotalCount})
	}
	for _, w := range c.ContributionCalendar.Weeks {
		for _, d := range w.ContributionDays {
			date, err := time.ParseInLocation(time.DateOnly, d.Date, time.Local)
			if err != nil {
				return yearResult{err: fmt.Errorf("bad date %q from GitHub", d.Date)}
			}
			r.days = append(r.days, Day{Date: date, Count: d.ContributionCount})
		}
	}
	return r
}

const reposQuery = `
query($login: String!, $cursor: String) {
  user(login: $login) {
    repositories(first: 100, after: $cursor, ownerAffiliations: OWNER, isFork: false) {
      pageInfo { hasNextPage endCursor }
      nodes {
        stargazerCount
        languages(first: 10) { edges { size node { name } } }
      }
    }
  }
}`

// fetchRepos adds up stars and language bytes over every repo the user owns.
func fetchRepos(client *api.GraphQLClient, p *Profile) error {
	bytes := map[string]int{}
	var cursor *string
	for {
		var resp struct {
			User struct {
				Repositories struct {
					PageInfo struct {
						HasNextPage bool
						EndCursor   string
					}
					Nodes []struct {
						StargazerCount int
						Languages      struct {
							Edges []struct {
								Size int
								Node struct{ Name string }
							}
						}
					}
				}
			}
		}
		vars := map[string]any{"login": p.Login, "cursor": cursor}
		if err := client.Do(reposQuery, vars, &resp); err != nil {
			return fmt.Errorf("fetching repositories: %w", err)
		}

		conn := resp.User.Repositories
		for _, r := range conn.Nodes {
			p.Repos++
			p.Stars += r.StargazerCount
			for _, e := range r.Languages.Edges {
				bytes[e.Node.Name] += e.Size
			}
		}
		if !conn.PageInfo.HasNextPage {
			break
		}
		cursor = &conn.PageInfo.EndCursor
	}

	for name, n := range bytes {
		p.Languages = append(p.Languages, Language{name, n})
	}
	return nil
}
