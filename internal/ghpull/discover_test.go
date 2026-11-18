package ghpull

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

func TestParseRepo(t *testing.T) {
	for _, tc := range []struct {
		in, want string
		ok       bool
	}{
		{"acme/widgets", "acme/widgets", true},
		{"  acme/widgets  ", "acme/widgets", true},
		{"https://github.com/acme/widgets", "acme/widgets", true},
		{"http://github.com/acme/widgets/", "acme/widgets", true},
		{"https://github.com/acme/widgets.git", "acme/widgets", true},
		{"github.com/acme/widgets", "acme/widgets", true},
		{"https://github.com/acme/widgets/issues/12", "acme/widgets", true},
		{"https://www.github.com/acme/widgets/tree/main", "acme/widgets", true},
		{"git@github.com:acme/widgets.git", "acme/widgets", true},
		{"my-org/my.repo_1", "my-org/my.repo_1", true},
		{"", "", false},
		{"acme", "", false},
		{"acme/", "", false},
		{"/widgets", "", false},
		{"acme/widgets/extra", "", false},
		{"https://example.com/acme/widgets", "", false},
		{"https://github.com/acme", "", false},
		{"acme/wid gets", "", false},
		{"../widgets", "", false},
		{"acme/..", "", false},
	} {
		got, err := ParseRepo(tc.in)
		if (err == nil) != tc.ok || got != tc.want {
			t.Errorf("ParseRepo(%q) = %q, %v; want %q ok=%v", tc.in, got, err, tc.want, tc.ok)
		}
	}
}

const mapsJSON = `{"data":{"repository":{"issues":{"nodes":[
 {"number":7,"title":"Old closed map","state":"CLOSED","subIssuesSummary":{"total":2,"completed":2,"percentCompleted":100},
  "subIssues":{"nodes":[{"labels":{"nodes":[{"name":"wayfinder:grilling"}]}}]}},
 {"number":5,"title":"Sub-issues but no wayfinder labels","state":"OPEN","subIssuesSummary":{"total":2,"completed":0,"percentCompleted":0},
  "subIssues":{"nodes":[{"labels":{"nodes":[{"name":"bug"}]}}]}},
 {"number":4,"title":"Plain issue","state":"OPEN","subIssuesSummary":{"total":0,"completed":0,"percentCompleted":0},
  "subIssues":{"nodes":[]}},
 {"number":3,"title":"Only a modifier label","state":"OPEN","subIssuesSummary":{"total":1,"completed":0,"percentCompleted":0},
  "subIssues":{"nodes":[{"labels":{"nodes":[{"name":"wayfinder:afk"}]}}]}},
 {"number":2,"title":"Live map","state":"OPEN","subIssuesSummary":{"total":4,"completed":3,"percentCompleted":75},
  "subIssues":{"nodes":[{"labels":{"nodes":[]}},{"labels":{"nodes":[{"name":"wayfinder:research"},{"name":"wayfinder:afk"}]}}]}}
]}}}}`

func TestListMapsKeepsIssuesWhoseChildCarriesAWayfinderLabel(t *testing.T) {
	var gotArgs []string
	gh := func(token string, args ...string) ([]byte, error) {
		gotArgs = args
		return []byte(mapsJSON), nil
	}

	maps, err := ListMaps(gh, "tok", "acme/widgets")
	if err != nil {
		t.Fatal(err)
	}

	want := []MapInfo{
		{Number: 2, Title: "Live map", Closed: false, Completed: 3, Total: 4, Percent: 75},
		{Number: 7, Title: "Old closed map", Closed: true, Completed: 2, Total: 2, Percent: 100},
	}
	if len(maps) != len(want) || maps[0] != want[0] || maps[1] != want[1] {
		t.Errorf("maps = %+v, want %+v (open first, closed after)", maps, want)
	}
	if !strings.Contains(strings.Join(gotArgs, " "), "owner=acme") || !strings.Contains(strings.Join(gotArgs, " "), "name=widgets") {
		t.Errorf("graphql args = %v", gotArgs)
	}
}

func TestListMapsSurfacesGHFailure(t *testing.T) {
	gh := func(string, ...string) ([]byte, error) { return nil, errors.New("boom") }
	if _, err := ListMaps(gh, "", "acme/widgets"); err == nil {
		t.Error("want an error")
	}
}

func TestStatusReportsAccountsOfTheDefaultHostAndTheActiveOne(t *testing.T) {
	gh := func(token string, args ...string) ([]byte, error) {
		if strings.Join(args, " ") != "auth status --json hosts" {
			t.Errorf("args = %v", args)
		}
		return []byte(`{"hosts":{
		  "ghe.example.com":[{"state":"success","active":true,"host":"ghe.example.com","login":"corp"}],
		  "github.com":[
		    {"state":"success","active":false,"host":"github.com","login":"bob"},
		    {"state":"error","active":false,"host":"github.com","login":"stale"},
		    {"state":"success","active":true,"host":"github.com","login":"alice"}]}}`), nil
	}

	st := GetStatus(gh)

	if !st.Installed || !st.LoggedIn || st.Account != "alice" {
		t.Errorf("status = %+v", st)
	}
	if len(st.Accounts) != 2 || st.Accounts[0] != "alice" || st.Accounts[1] != "bob" {
		t.Errorf("accounts = %v, want active first, then the rest, no stale or other-host logins", st.Accounts)
	}
}

func TestStatusWithoutGHOrLogin(t *testing.T) {
	missing := func(string, ...string) ([]byte, error) {
		return nil, classify([]string{"auth", "status"}, exec.ErrNotFound, "")
	}
	if st := GetStatus(missing); st.Installed || st.LoggedIn {
		t.Errorf("missing gh: %+v", st)
	}

	empty := func(string, ...string) ([]byte, error) { return []byte(`{"hosts":{}}`), nil }
	if st := GetStatus(empty); !st.Installed || st.LoggedIn || st.Account != "" {
		t.Errorf("no login: %+v", st)
	}
}

func TestStatusFallsBackToTheAPIUserWhenGHLacksJSONStatus(t *testing.T) {
	gh := func(token string, args ...string) ([]byte, error) {
		if args[0] == "auth" {
			return nil, classify(args, errors.New("exit status 1"), "unknown flag: --json")
		}
		return []byte("carol\n"), nil
	}

	if st := GetStatus(gh); !st.LoggedIn || st.Account != "carol" || len(st.Accounts) != 1 {
		t.Errorf("status = %+v", st)
	}
}

func TestListReposReturnsReposTheAccountWasActiveIn(t *testing.T) {
	var calls []string
	gh := func(token string, args ...string) ([]byte, error) {
		calls = append(calls, token+"|"+strings.Join(args, " "))
		if args[1] == "user" {
			return []byte("alice\n"), nil
		}
		return []byte(`[
		 {"type":"PushEvent","repo":{"name":"acme/widgets"}},
		 {"type":"WatchEvent","repo":{"name":"other/starred"}},
		 {"type":"IssueCommentEvent","repo":{"name":"acme/gadgets"}},
		 {"type":"PullRequestEvent","repo":{"name":"acme/widgets"}}]`), nil
	}

	repos, err := ListRepos(gh, "tok")

	if err != nil || strings.Join(repos, ",") != "acme/widgets,acme/gadgets" {
		t.Errorf("repos = %v, err = %v; want the repos of her events, newest first, no stars, no duplicates", repos, err)
	}
	want := []string{"tok|api user --jq .login", "tok|api users/alice/events?per_page=100"}
	if strings.Join(calls, "\n") != strings.Join(want, "\n") {
		t.Errorf("calls = %q, want %q", calls, want)
	}
}

func TestListReposStopsAtTen(t *testing.T) {
	gh := func(token string, args ...string) ([]byte, error) {
		if args[1] == "user" {
			return []byte("alice"), nil
		}
		var evs []string
		for i := 0; i < 15; i++ {
			evs = append(evs, fmt.Sprintf(`{"type":"PushEvent","repo":{"name":"acme/r%d"}}`, i))
		}
		return []byte("[" + strings.Join(evs, ",") + "]"), nil
	}

	if repos, err := ListRepos(gh, ""); err != nil || len(repos) != 10 {
		t.Errorf("len = %d, err = %v", len(repos), err)
	}
}

func TestListReposSurfacesGHFailure(t *testing.T) {
	gh := func(string, ...string) ([]byte, error) { return nil, errors.New("boom") }
	if _, err := ListRepos(gh, ""); err == nil {
		t.Error("want an error")
	}
}

func TestAccountTokenAsksGHForThatAccount(t *testing.T) {
	gh := func(token string, args ...string) ([]byte, error) {
		if strings.Join(args, " ") != "auth token --user bob" {
			t.Errorf("args = %v", args)
		}
		return []byte("tok-bob\n"), nil
	}

	if tok, err := AccountToken(gh, "bob"); err != nil || tok != "tok-bob" {
		t.Errorf("token = %q, err = %v", tok, err)
	}
	if tok, err := AccountToken(gh, ""); err != nil || tok != "" {
		t.Errorf("no account should mean the active one: %q, %v", tok, err)
	}
}
