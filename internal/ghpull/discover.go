package ghpull

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// DefaultHost is the GitHub host everything talks to; the one place to widen
// when Enterprise support arrives.
const DefaultHost = "github.com"

var (
	repoPartRE = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	scpURLRE   = regexp.MustCompile(`^git@([^:]+):`)
)

// ParseRepo normalises "owner/repo" or a GitHub URL (https, scp-style ssh, with
// or without .git or trailing path) to "owner/repo".
func ParseRepo(s string) (string, error) {
	s = strings.TrimSpace(s)
	bad := fmt.Errorf("%q is not an owner/repo or a %s URL", s, DefaultHost)
	if m := scpURLRE.FindStringSubmatch(s); m != nil {
		s = m[1] + "/" + s[len(m[0]):]
	}
	for _, p := range []string{"https://", "http://"} {
		s = strings.TrimPrefix(s, p)
	}
	parts := strings.Split(strings.Trim(s, "/"), "/")
	if len(parts) > 0 && strings.Contains(parts[0], ".") {
		if strings.TrimPrefix(parts[0], "www.") != DefaultHost {
			return "", bad
		}
		parts = parts[1:]
		if len(parts) > 2 && parts[2] != "issues" && parts[2] != "tree" && parts[2] != "pull" && parts[2] != "blob" {
			return "", bad
		}
		parts = parts[:min(len(parts), 2)]
	}
	if len(parts) != 2 {
		return "", bad
	}
	parts[1] = strings.TrimSuffix(parts[1], ".git")
	for _, p := range parts {
		if !repoPartRE.MatchString(p) || strings.Trim(p, ".") == "" {
			return "", bad
		}
	}
	return parts[0] + "/" + parts[1], nil
}

// MapInfo is a map found in a repo: a parent issue whose sub-issues carry
// wayfinder labels.
type MapInfo struct {
	Number    int    `json:"number"`
	Title     string `json:"title"`
	Closed    bool   `json:"closed"`
	Completed int    `json:"completed"`
	Total     int    `json:"total"`
	Percent   int    `json:"percent"`
}

// mapsQuery scans the 100 most recently updated issues in one request. Labels
// are read from the first 30 sub-issues, which is plenty to spot a map.
const mapsQuery = `
query($owner: String!, $name: String!) {
  repository(owner: $owner, name: $name) {
    issues(first: 100, states: [OPEN, CLOSED], orderBy: {field: UPDATED_AT, direction: DESC}) {
      nodes {
        number title state
        subIssuesSummary { total completed percentCompleted }
        subIssues(first: 30) { nodes { labels(first: 10) { nodes { name } } } }
      }
    }
  }
}`

// ListMaps finds the maps in repo, open ones first (most recently updated
// first within each group), closed ones after.
func ListMaps(gh Runner, token, repo string) ([]MapInfo, error) {
	owner, name, _ := strings.Cut(repo, "/")
	var res struct {
		Repository struct {
			Issues struct {
				Nodes []struct {
					Number  int    `json:"number"`
					Title   string `json:"title"`
					State   string `json:"state"`
					Summary struct {
						Total, Completed, PercentCompleted int
					} `json:"subIssuesSummary"`
					SubIssues struct {
						Nodes []labelled `json:"nodes"`
					} `json:"subIssues"`
				} `json:"nodes"`
			} `json:"issues"`
		} `json:"repository"`
	}
	if err := graphql(gh, token, repo, &res, mapsQuery, "-f", "owner="+owner, "-f", "name="+name); err != nil {
		return nil, err
	}
	var open, closed []MapInfo
	for _, n := range res.Repository.Issues.Nodes {
		if n.Summary.Total == 0 || !hasWayfinderChild(n.SubIssues.Nodes) {
			continue
		}
		m := MapInfo{Number: n.Number, Title: n.Title, Closed: n.State == "CLOSED",
			Completed: n.Summary.Completed, Total: n.Summary.Total, Percent: n.Summary.PercentCompleted}
		if m.Closed {
			closed = append(closed, m)
		} else {
			open = append(open, m)
		}
	}
	return append(open, closed...), nil
}

type labelled struct {
	Labels struct {
		Nodes []struct {
			Name string `json:"name"`
		} `json:"nodes"`
	} `json:"labels"`
}

// hasWayfinderChild reports whether any child carries a wayfinder:<type> label
// (wayfinder:afk is a modifier, not a type).
func hasWayfinderChild(children []labelled) bool {
	for _, c := range children {
		for _, l := range c.Labels.Nodes {
			if rest, ok := strings.CutPrefix(l.Name, "wayfinder:"); ok && rest != "" && rest != "afk" {
				return true
			}
		}
	}
	return false
}

// Status is the gh state the splash shows: whether gh is installed, which
// accounts of the default host are logged in, and which one is active.
type Status struct {
	Installed bool     `json:"installed"`
	LoggedIn  bool     `json:"loggedIn"`
	Account   string   `json:"account"`
	Accounts  []string `json:"accounts"`
}

// GetStatus reads the logged-in accounts, the active one first.
func GetStatus(gh Runner) Status {
	b, err := gh("", "auth", "status", "--json", "hosts")
	var ge *GHError
	if errors.As(err, &ge) && ge.Kind == KindMissing {
		return Status{}
	}
	st := Status{Installed: true}
	if err != nil {
		// gh without `auth status --json`: all it can tell is the active login.
		if b, err := gh("", "api", "user", "--jq", ".login"); err == nil && strings.TrimSpace(string(b)) != "" {
			st.Account = strings.TrimSpace(string(b))
			st.Accounts, st.LoggedIn = []string{st.Account}, true
		}
		return st
	}
	var res struct {
		Hosts map[string][]struct {
			State  string `json:"state"`
			Active bool   `json:"active"`
			Login  string `json:"login"`
		} `json:"hosts"`
	}
	if json.Unmarshal(b, &res) != nil {
		return st
	}
	for _, a := range res.Hosts[DefaultHost] {
		switch {
		case a.State != "success":
		case a.Active:
			st.Account = a.Login
			st.Accounts = append([]string{a.Login}, st.Accounts...)
		default:
			st.Accounts = append(st.Accounts, a.Login)
		}
	}
	st.LoggedIn = len(st.Accounts) > 0
	return st
}

// ListRepos returns the 10 repos the account was most recently active in
// (pushes, pull requests, issues, comments), from its event feed. Stars do not
// count.
func ListRepos(gh Runner, token string) ([]string, error) {
	b, err := gh(token, "api", "user", "--jq", ".login")
	if err != nil {
		return nil, err
	}
	b, err = gh(token, "api", "users/"+strings.TrimSpace(string(b))+"/events?per_page=100")
	if err != nil {
		return nil, err
	}
	var evs []struct {
		Type string `json:"type"`
		Repo struct {
			Name string `json:"name"`
		} `json:"repo"`
	}
	if err := json.Unmarshal(b, &evs); err != nil {
		return nil, fmt.Errorf("gh events: bad JSON: %v", err)
	}
	names, seen := []string{}, map[string]bool{}
	for _, e := range evs {
		if e.Type == "WatchEvent" || e.Repo.Name == "" || seen[e.Repo.Name] {
			continue
		}
		seen[e.Repo.Name] = true
		if names = append(names, e.Repo.Name); len(names) == 10 {
			break
		}
	}
	return names, nil
}

// AccountToken returns the token of a logged-in gh account; "" means the
// active one, which needs no token.
func AccountToken(gh Runner, account string) (string, error) {
	if account == "" {
		return "", nil
	}
	b, err := gh("", "auth", "token", "--user", account)
	if err != nil {
		var ge *GHError
		if errors.As(err, &ge) && ge.Kind == KindMissing {
			return "", err
		}
		return "", userErr("gh account '%s' is not logged in. Run: gh auth login", account)
	}
	return strings.TrimSpace(string(b)), nil
}
