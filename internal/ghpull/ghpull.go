// Package ghpull mirrors a wayfinder map kept as GitHub issues into the .plan/
// layout. It is read-only against GitHub and shells out to gh for all HTTP.
package ghpull

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Runner runs `gh <args>` and returns stdout. A non-empty token is passed to
// gh as GH_TOKEN.
type Runner func(token string, args ...string) ([]byte, error)

// GH is the Runner that executes the real gh binary.
func GH(token string, args ...string) ([]byte, error) {
	cmd := exec.Command("gh", args...)
	if token != "" {
		cmd.Env = append(os.Environ(), "GH_TOKEN="+token)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, classify(args, err, stderr.String())
	}
	return out, nil
}

// Source is what a pull records in <out>/.source.json.
type Source struct {
	Repo    string `json:"repo"`
	Map     int    `json:"map"`
	Auth    string `json:"auth"`
	Account string `json:"account"`
}

// ReadSource reports whether dir is a GitHub mirror, and where from.
func ReadSource(dir string) (Source, bool) {
	var src Source
	if dir == "" {
		return src, false
	}
	b, err := os.ReadFile(filepath.Join(dir, ".source.json"))
	if err != nil || json.Unmarshal(b, &src) != nil || src.Repo == "" || src.Map == 0 {
		return src, false
	}
	return src, true
}

// Error carries the process exit code for failures the caller caused (usage,
// an unknown gh account, a map without sub-issues). Other errors exit 1.
type Error struct {
	Code int
	Msg  string
}

func (e *Error) Error() string { return e.Msg }

func userErr(format string, a ...any) error {
	return &Error{Code: 2, Msg: fmt.Sprintf(format, a...)}
}

// Options selects what to pull. With Refresh, only Out is read and the rest
// comes from <Out>/.source.json.
type Options struct {
	Repo    string
	Map     int
	Out     string
	Refresh bool
	// Account pulls as this gh account and records it in .source.json. Empty
	// keeps the recorded account, then $WAYFINDER_GH_ACCOUNT, then the active one.
	Account string
	// Progress, if set, gets (done, total) items as the pull advances. The
	// total moves: it is 1 until the sub-issues are listed.
	Progress func(done, total int)
}

// Puller holds the injectable pieces: the gh runner, the environment, and the
// writers for the per-ticket log and warnings.
type Puller struct {
	GH     Runner
	Getenv func(string) string
	Stdout io.Writer
	Stderr io.Writer
}

type issue struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	Body        string `json:"body"`
	State       string `json:"state"`
	StateReason string `json:"state_reason"`
	UpdatedAt   string `json:"updated_at"`
	Assignees   []struct {
		Login string `json:"login"`
	} `json:"assignees"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
	Deps struct {
		TotalBlockedBy int `json:"total_blocked_by"`
	} `json:"issue_dependencies_summary"`
}

type comment struct {
	Body string `json:"body"`
}

type linkedPR struct {
	URL    string `json:"url"`
	Merged bool   `json:"merged"`
}

const prQuery = `
query($owner: String!, $name: String!, $number: Int!) {
  repository(owner: $owner, name: $name) {
    issue(number: $number) {
      closedByPullRequestsReferences(first: 20, includeClosedPrs: true) {
        nodes { url merged }
      }
    }
  }
}`

// Pull mirrors the map into o.Out. The mirror is built in a staging dir and
// swapped in at the end, so a failed pull leaves the previous mirror intact.
func (p *Puller) Pull(o Options) error {
	var who string
	return explainWith(p.pull(o, &who), &who)
}

func explainWith(err error, who *string) error {
	if *who == "" {
		*who = "the active gh account"
	}
	return explain(err, *who)
}

func (p *Puller) pull(o Options, who *string) error {
	repo, mapNum, out := o.Repo, o.Map, o.Out
	var auth, account, token string
	progress := o.Progress
	if progress == nil {
		progress = func(int, int) {}
	}
	if o.Refresh {
		src, ok := ReadSource(out)
		if !ok {
			return userErr("--refresh needs a pulled map dir containing .source.json")
		}
		repo, mapNum, auth, account = src.Repo, src.Map, src.Auth, src.Account
	}
	if repo == "" || mapNum == 0 {
		return userErr("usage: wayfinder-maps pull [--account <gh-user>] <owner/repo> <map-issue-number> [out-dir]\n       wayfinder-maps pull [--account <gh-user>] --refresh <out-dir>")
	}
	switch {
	case o.Account != "":
		auth, account = "gh", o.Account
	case account == "" && p.Getenv("WAYFINDER_GH_ACCOUNT") != "":
		auth, account = "gh", p.Getenv("WAYFINDER_GH_ACCOUNT")
	}
	if auth == "gh" && account != "" {
		*who = fmt.Sprintf("gh account '%s'", account)
		var err error
		if token, err = AccountToken(p.GH, account); err != nil {
			return err
		}
	}

	if auth == "" {
		if p.Getenv("GH_TOKEN")+p.Getenv("GITHUB_TOKEN") != "" {
			auth = "env"
		} else {
			auth = "gh"
			if b, err := p.GH(token, "api", "user", "--jq", ".login"); err == nil {
				account = strings.TrimSpace(string(b))
			}
		}
	}

	switch {
	case auth == "env":
		*who = "the GH_TOKEN/GITHUB_TOKEN token"
	case account != "":
		*who = fmt.Sprintf("gh account '%s'", account)
	}

	progress(0, 1)
	var m issue
	if err := p.getJSON(token, &m, "api", fmt.Sprintf("repos/%s/issues/%d", repo, mapNum)); err != nil {
		return err
	}
	if out == "" {
		out = filepath.Join(".plan", Slugify(m.Title))
	}

	subs, err := getPaged[issue](p, token, fmt.Sprintf("repos/%s/issues/%d/sub_issues", repo, mapNum))
	if err != nil {
		return err
	}
	if len(subs) == 0 {
		return userErr("issue #%d has no sub-issues", mapNum)
	}
	total := len(subs) + 1
	progress(1, total)

	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(filepath.Dir(out), ".wayfinder-pull-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	if err := os.Chmod(stage, 0o755); err != nil {
		return err
	}
	if err := os.Mkdir(filepath.Join(stage, "tickets"), 0o755); err != nil {
		return err
	}

	var decisions, implemented, outOfScope strings.Builder
	var rewrites []*regexp.Regexp
	var rewriteTo []string
	for i, t := range subs {
		link, err := p.pullTicket(token, repo, stage, t, &decisions, &implemented, &outOfScope)
		if err != nil {
			return err
		}
		re := regexp.MustCompile(fmt.Sprintf(`(?m)https://github\.com/%s/issues/%d([^0-9]|$)`, regexp.QuoteMeta(repo), t.Number))
		rewrites = append(rewrites, re)
		rewriteTo = append(rewriteTo, link+"${1}")
		progress(i+2, total)
	}

	// The map body is already a wayfinder map; point its issue URLs at the
	// mirrored files. A section the body lacks is generated from ticket states.
	body := strings.TrimRight(m.Body, "\n")
	for i, re := range rewrites {
		body = re.ReplaceAllString(body, rewriteTo[i])
	}
	body = strings.TrimRight(body, "\n")

	var mb strings.Builder
	fmt.Fprintf(&mb, "# %s\n\n%s\n", m.Title, body)
	for _, s := range []struct {
		heading string
		list    string
	}{
		{"## Decisions so far", decisions.String()},
		{"## Implemented so far", implemented.String()},
		{"## Out of scope", outOfScope.String()},
	} {
		if s.list == "" || regexp.MustCompile(`(?m)^`+regexp.QuoteMeta(s.heading)).MatchString(body) {
			continue
		}
		fmt.Fprintf(&mb, "\n%s\n\n%s", s.heading, s.list)
	}
	if err := os.WriteFile(filepath.Join(stage, "map.md"), []byte(mb.String()), 0o644); err != nil {
		return err
	}

	var sb bytes.Buffer
	enc := json.NewEncoder(&sb)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(Source{Repo: repo, Map: mapNum, Auth: auth, Account: account}); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(stage, ".source.json"), sb.Bytes(), 0o644); err != nil {
		return err
	}
	if err := os.RemoveAll(out); err != nil {
		return err
	}
	if err := os.Rename(stage, out); err != nil {
		return err
	}

	fmt.Fprintf(p.Stdout, "wayfinder-maps pull: %s#%d -> %s (%d tickets)\n", repo, mapNum, out, len(subs))
	return nil
}

// pullTicket writes one ticket file and returns its relative link. It also
// files the ticket into the generated map sections.
func (p *Puller) pullTicket(token, repo, out string, t issue, decisions, implemented, outOfScope *strings.Builder) (string, error) {
	assignee := ""
	if len(t.Assignees) > 0 {
		assignee = t.Assignees[0].Login
	}
	typ, afk := ticketType(t)

	pr := ""
	if typ == "implement" {
		owner, name, _ := strings.Cut(repo, "/")
		var res struct {
			Data struct {
				Repository struct {
					Issue struct {
						Closed struct {
							Nodes []linkedPR `json:"nodes"`
						} `json:"closedByPullRequestsReferences"`
					} `json:"issue"`
				} `json:"repository"`
			} `json:"data"`
		}
		err := p.getJSON(token, &res, "api", "graphql", "-f", "query="+prQuery,
			"-f", "owner="+owner, "-f", "name="+name, "-F", "number="+strconv.Itoa(t.Number))
		if err != nil {
			return "", err
		}
		nodes := res.Data.Repository.Issue.Closed.Nodes
		for i, n := range nodes {
			if i == 0 {
				pr = n.URL
			}
			if n.Merged {
				pr = n.URL
				break
			}
		}
		if len(nodes) > 1 {
			fmt.Fprintf(p.Stderr, "wayfinder-maps pull: #%d links more than one PR; using %s\n", t.Number, pr)
		}
	}

	blockedBy := []int{}
	if t.Deps.TotalBlockedBy != 0 {
		var blockers []issue
		if err := p.getJSON(token, &blockers, "api", fmt.Sprintf("repos/%s/issues/%d/dependencies/blocked_by", repo, t.Number)); err != nil {
			return "", err
		}
		for _, b := range blockers {
			blockedBy = append(blockedBy, b.Number)
		}
	}
	blockedJSON, _ := json.Marshal(blockedBy)

	slug := Slugify(t.Title)
	name := fmt.Sprintf("%d-%s.md", t.Number, slug)
	link := "./tickets/" + name

	var f strings.Builder
	fmt.Fprintf(&f, "---\ntype: %s\nblocked_by: %s\n", typ, blockedJSON)
	if pr != "" {
		fmt.Fprintf(&f, "pr: %s\n", pr)
	}
	if afk {
		f.WriteString("afk: true\n")
	}
	if t.State == "open" && assignee != "" {
		fmt.Fprintf(&f, "claimed_by: %s\nclaimed_at: %s\n", assignee, t.UpdatedAt)
	}
	fmt.Fprintf(&f, "---\n\n# %s\n\n%s\n", t.Title, t.Body)

	if t.State == "closed" {
		answer, err := p.closedAnswer(token, repo, t, typ, pr)
		if err != nil {
			return "", err
		}
		item := fmt.Sprintf("- [%s](%s)\n", t.Title, link)
		switch {
		case t.StateReason == "not_planned":
			fmt.Fprintf(&f, "\n## Ruled out\n\n%s\n", answer)
			outOfScope.WriteString(item)
		case typ == "implement":
			fmt.Fprintf(&f, "\n## Answer\n\n%s\n", answer)
			implemented.WriteString(item)
		default:
			fmt.Fprintf(&f, "\n## Answer\n\n%s\n", answer)
			decisions.WriteString(item)
		}
	}
	if err := os.WriteFile(filepath.Join(out, "tickets", name), []byte(f.String()), 0o644); err != nil {
		return "", err
	}

	line := fmt.Sprintf("  #%d  %s  %s", t.Number, typ, t.State)
	if assignee != "" {
		line += "  @" + assignee
	}
	fmt.Fprintln(p.Stdout, line)
	return link, nil
}

var headingRE = regexp.MustCompile(`(?m)^(#{1,5}) `)

// closedAnswer is the ticket's comments, joined, with headings demoted one
// level so a comment's own heading cannot end the "## Answer" section.
func (p *Puller) closedAnswer(token, repo string, t issue, typ, pr string) (string, error) {
	comments, err := getPaged[comment](p, token, fmt.Sprintf("repos/%s/issues/%d/comments", repo, t.Number))
	if err != nil {
		return "", err
	}
	bodies := make([]string, len(comments))
	for i, c := range comments {
		bodies[i] = c.Body
	}
	answer := strings.TrimRight(headingRE.ReplaceAllString(strings.Join(bodies, "\n\n---\n\n"), "#$1 "), "\n")
	if answer != "" {
		return answer, nil
	}
	if typ == "implement" && t.StateReason != "not_planned" && pr != "" {
		return "Shipped in " + pr + ".", nil
	}
	return "_Closed on GitHub with no answer comment._", nil
}

// ticketType is the first wayfinder:<type> label other than afk; anything
// unknown is a grilling ticket.
func ticketType(t issue) (typ string, afk bool) {
	typ = "grilling"
	found := false
	for _, l := range t.Labels {
		if l.Name == "wayfinder:afk" {
			afk = true
			continue
		}
		if rest, ok := strings.CutPrefix(l.Name, "wayfinder:"); ok && !found {
			typ, found = rest, true
		}
	}
	switch typ {
	case "research", "prototype", "grilling", "task", "implement":
	default:
		typ = "grilling"
	}
	return typ, afk
}

func (p *Puller) getJSON(token string, v any, args ...string) error {
	b, err := p.GH(token, args...)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("gh %s: bad JSON: %v", endpointOf(args), err)
	}
	return nil
}

// getPaged fetches every page of a list endpoint and merges them.
func getPaged[T any](p *Puller, token, endpoint string) ([]T, error) {
	var pages [][]T
	if err := p.getJSON(token, &pages, "api", "--paginate", "--slurp", endpoint); err != nil {
		return nil, err
	}
	var all []T
	for _, pg := range pages {
		all = append(all, pg...)
	}
	return all, nil
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify lowercases ASCII, collapses other runs to "-", and caps at 50 chars.
func Slugify(s string) string {
	lower := strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' {
			return r + 'a' - 'A'
		}
		return r
	}, s)
	s = strings.Trim(nonSlug.ReplaceAllString(lower, "-"), "-")
	if len(s) > 50 {
		s = s[:50]
	}
	return strings.TrimRight(s, "-")
}
