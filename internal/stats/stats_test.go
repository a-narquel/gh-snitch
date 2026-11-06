package stats

import (
	"testing"
	"time"

	"github.com/a-narquel/gh-snitch/internal/github"
	"github.com/a-narquel/gh-snitch/internal/gitlog"
)

// at makes a time quickly: at(2026, 10, 5, 23) = Oct 5 2026, 23:00 local.
func at(y int, m time.Month, d, h int) time.Time {
	return time.Date(y, m, d, h, 0, 0, 0, time.Local)
}

func commit(author string, when time.Time, files ...gitlog.FileChange) gitlog.Commit {
	return gitlog.Commit{Hash: author + when.Format("0102150405"), Author: author, When: when, Files: files}
}

func file(path string, added, deleted int) gitlog.FileChange {
	return gitlog.FileChange{Path: path, Added: added, Deleted: deleted}
}

// sample is a small made-up project used by several tests.
func sample() Data {
	return Data{
		Commits: []gitlog.Commit{
			commit("andre", at(2026, 10, 7, 14), file("src/server/a.go", 50, 10)),
			commit("andre", at(2026, 10, 6, 23), file("src/server/a.go", 20, 5), file("api.proto", 5, 0)),
			commit("andre", at(2026, 10, 5, 10), file("src/server/b.go", 30, 0)), // Monday
			commit("joao", at(2026, 10, 3, 2), file("docs/readme.md", 10, 2)),    // Saturday, 2am
			commit("dependabot[bot]", at(2026, 10, 2, 9), file("go.sum", 3, 3)),
		},
		Blame: []gitlog.FileBlame{
			{Path: "src/server/a.go", Lines: lines("andre", at(2026, 10, 7, 14), 40)},
			{Path: "docs/readme.md", Lines: lines("joao", at(2026, 10, 3, 2), 8)},
		},
		HasPRs: true,
		Names:  map[string]string{"a-narquel": "andre", "jsilva": "joao"},
		PRs: []github.PR{{
			Number: 1, Author: "a-narquel", MergedBy: "a-narquel", State: "MERGED",
			CreatedAt: at(2026, 10, 6, 9), MergedAt: at(2026, 10, 6, 12),
			Reviews: []github.Review{
				{Author: "jsilva", State: "APPROVED", SubmittedAt: at(2026, 10, 6, 10)},
				{Author: "jsilva", State: "COMMENTED", SubmittedAt: at(2026, 10, 6, 11)},
				{Author: "a-narquel", State: "COMMENTED", SubmittedAt: at(2026, 10, 6, 11)}, // self: ignored
			},
		}},
	}
}

func lines(author string, when time.Time, n int) []gitlog.BlameLine {
	out := make([]gitlog.BlameLine, n)
	for i := range out {
		out[i] = gitlog.BlameLine{Author: author, When: when}
	}
	return out
}

func TestMembers(t *testing.T) {
	members := Members(Filter{}.Apply(sample())) // the default filter hides bots
	if len(members) != 2 {
		t.Fatalf("got %d members, want 2 (bot hidden): %+v", len(members), members)
	}

	a := members[0]
	if a.Name != "andre" {
		t.Fatalf("top member = %q, want andre", a.Name)
	}
	checks := []struct {
		field     string
		got, want int
	}{
		{"Commits", a.Commits, 3},
		{"Added", a.Added, 105},
		{"Deleted", a.Deleted, 15},
		{"Surviving", a.Surviving, 40},
		{"ActiveDays", a.ActiveDays, 3},
		{"LongestStreak", a.LongestStreak, 3},
		{"Night", a.Night, 1},
		{"Biggest", a.Biggest, 60},
		{"PRs", a.PRs, 1},
		{"Reviews", a.Reviews, 0},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("andre.%s = %d, want %d", c.field, c.got, c.want)
		}
	}

	j := members[1]
	if j.Reviews != 1 {
		t.Errorf("joao.Reviews = %d, want 1 (two reviews on the same PR count once)", j.Reviews)
	}
	if j.Night != 1 || j.Weekend != 1 {
		t.Errorf("joao night/weekend = %d/%d, want 1/1", j.Night, j.Weekend)
	}
}

func TestFilter(t *testing.T) {
	d := sample()

	f := Filter{Since: at(2026, 10, 5, 0)}
	if got := len(f.Apply(d).Commits); got != 3 {
		t.Errorf("since Oct 5: %d commits, want 3", got)
	}

	f = Filter{Exclude: []string{"*.proto", "docs/"}}
	v := f.Apply(d)
	for _, c := range v.Commits {
		for _, fc := range c.Files {
			if fc.Path == "api.proto" || fc.Path == "docs/readme.md" {
				t.Errorf("excluded file %s is still there", fc.Path)
			}
		}
		if c.Author == "joao" {
			t.Error("joao's only commit touched an excluded file, so it should be gone")
		}
	}

	f = Filter{ShowBots: true}
	if got := len(f.Apply(d).Commits); got != 5 {
		t.Errorf("with bots: %d commits, want 5", got)
	}

	// Apply must not change the original data.
	if len(d.Commits[1].Files) != 2 {
		t.Error("Apply modified the original commit's files")
	}
}

func TestParseWhen(t *testing.T) {
	now := at(2026, 11, 6, 12)
	tests := []struct {
		in   string
		want time.Time
	}{
		{"2w", now.AddDate(0, 0, -14)},
		{"3d", now.AddDate(0, 0, -3)},
		{"1m", now.AddDate(0, -1, 0)},
		{"1y", now.AddDate(-1, 0, 0)},
		{"12h", now.Add(-12 * time.Hour)},
		{"2026-10-01", at(2026, 10, 1, 0)},
	}
	for _, tt := range tests {
		got, err := ParseWhen(tt.in, now)
		if err != nil || !got.Equal(tt.want) {
			t.Errorf("ParseWhen(%q) = %v, %v; want %v", tt.in, got, err, tt.want)
		}
	}
	for _, bad := range []string{"", "w", "2x", "soon", "-3d"} {
		if _, err := ParseWhen(bad, now); err == nil {
			t.Errorf("ParseWhen(%q) returned no error", bad)
		}
	}
}

func TestBusFactor(t *testing.T) {
	names, share := BusFactor(sample())
	if len(names) != 1 || names[0] != "andre" {
		t.Errorf("BusFactor names = %v, want [andre]", names)
	}
	if share < 83 || share > 84 { // 40 of 48 lines
		t.Errorf("BusFactor share = %.1f, want about 83.3", share)
	}
}

func TestReviews(t *testing.T) {
	rs := Reviews(sample())
	if got := rs.Matrix["joao"]["andre"]; got != 1 {
		t.Errorf("joao reviewed andre %d times, want 1", got)
	}
	if rs.MedianFirstReview != time.Hour {
		t.Errorf("MedianFirstReview = %v, want 1h", rs.MedianFirstReview)
	}
	if rs.MedianMerge != 3*time.Hour {
		t.Errorf("MedianMerge = %v, want 3h", rs.MedianMerge)
	}
	if len(rs.SelfMerged) != 0 {
		t.Errorf("SelfMerged = %v, want none (joao reviewed it)", rs.SelfMerged)
	}
}

func TestActivityKeepsEmptyWeeks(t *testing.T) {
	d := Data{Commits: []gitlog.Commit{
		commit("a", at(2026, 10, 26, 10)),
		commit("a", at(2026, 10, 5, 10)),
	}}
	tl := Activity(d, Week)
	if len(tl.Starts) != 4 {
		t.Fatalf("got %d weeks, want 4 (Oct 5 → Oct 26)", len(tl.Starts))
	}
	want := []int{1, 0, 0, 1}
	for i, n := range want {
		if tl.Total[i] != n {
			t.Errorf("week %d: %d commits, want %d", i, tl.Total[i], n)
		}
	}
	if tl.Starts[0].Weekday() != time.Monday {
		t.Errorf("weeks start on %v, want Monday", tl.Starts[0].Weekday())
	}
}

func TestCommitTypes(t *testing.T) {
	d := Data{Commits: []gitlog.Commit{
		{Author: "a", Subject: "feat(ui): add button"},
		{Author: "a", Subject: "fix!: crash on start"},
		{Author: "a", Subject: "Feature: big thing"},
		{Author: "a", Subject: "update readme"},
	}}
	got := map[string]int{}
	for _, s := range CommitTypes(d, "a") {
		got[s.Name] = s.Value
	}
	if got["feat"] != 2 || got["fix"] != 1 || got["other"] != 1 {
		t.Errorf("CommitTypes = %v, want feat:2 fix:1 other:1", got)
	}
}
