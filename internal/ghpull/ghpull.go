// Package ghpull mirrors a wayfinder map kept as GitHub issues into the .plan/
// layout. It is read-only against GitHub and shells out to gh for all HTTP.
package ghpull

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/RamsesKools/wayfinder-maps/internal/wayfinder"
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

// Step is how far a pull has come, in mirrored items and fetched batches. Both
// totals move: they are 1 until the first batch is in.
type Step struct {
	Done    int `json:"done"`
	Total   int `json:"total"`
	Batch   int `json:"batch"`
	Batches int `json:"batches"`
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
	// Progress, if set, is called at the start and once per batch.
	Progress func(Step)
}

// Puller holds the injectable pieces: the gh runner, the environment, and the
// writers for the per-ticket log and warnings.
type Puller struct {
	GH     Runner
	Getenv func(string) string
	// DefaultAccount is the gh account used when nothing else picks one; the env var beats it.
	DefaultAccount string
	Stdout         io.Writer
	Stderr         io.Writer
}

type issue struct {
	ID          string `json:"id"`
	Number      int    `json:"number"`
	Title       string `json:"title"`
	Body        string `json:"body"`
	State       string `json:"state"`
	StateReason string `json:"stateReason"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
	Assignees   struct {
		Nodes []struct {
			Login string `json:"login"`
		} `json:"nodes"`
	} `json:"assignees"`
	Labels    conn[label]    `json:"labels"`
	Comments  conn[comment]  `json:"comments"`
	BlockedBy conn[blocker]  `json:"blockedBy"`
	PRs       conn[linkedPR] `json:"closedByPullRequestsReferences"`
}

type conn[T any] struct {
	PageInfo struct {
		HasNextPage bool   `json:"hasNextPage"`
		EndCursor   string `json:"endCursor"`
	} `json:"pageInfo"`
	Nodes []T `json:"nodes"`
}

type label struct {
	Name string `json:"name"`
}

type comment struct {
	Body string `json:"body"`
}

type blocker struct {
	Number int `json:"number"`
}

type linkedPR struct {
	URL    string `json:"url"`
	Merged bool   `json:"merged"`
}

type mapPage struct {
	Title     string `json:"title"`
	Body      string `json:"body"`
	SubIssues struct {
		TotalCount int `json:"totalCount"`
		conn[issue]
	} `json:"subIssues"`
}

// nested is a paged connection of a ticket.
type nested struct {
	name, args, fields string
	first              int
}

// A full batch asks for at most 6,100 of the 500,000 nodes GraphQL allows, and
// stays well clear of its query timeout.
const ticketBatch = 50

var (
	labelConn   = nested{name: "labels", fields: "name", first: 20}
	commentConn = nested{name: "comments", fields: "body", first: 30}
	// The default order is newest first; this one matches REST.
	blockerConn = nested{name: "blockedBy", args: ", orderBy: {field: DEPENDENCY_ADDED_AT, direction: ASC}", fields: "number", first: 50}
	prConn      = nested{name: "closedByPullRequestsReferences", args: ", includeClosedPrs: true", fields: "url merged", first: 20}
)

const pageInfoFields = "pageInfo { hasNextPage endCursor }"

func (n nested) selection() string {
	return fmt.Sprintf("%s(first: %d%s) { %s nodes { %s } }", n.name, n.first, n.args, pageInfoFields, n.fields)
}

var mapQuery = fmt.Sprintf(`query WayfinderMap($owner: String!, $name: String!, $number: Int!, $after: String) {
  repository(owner: $owner, name: $name) {
    issue(number: $number) {
      title body
      subIssues(first: %d, after: $after) {
        totalCount %s
        nodes {
          id number title body state stateReason(enableDuplicate: true) createdAt updatedAt
          assignees(first: 1) { nodes { login } }
          %s
          %s
          %s
          %s
        }
      }
    }
  }
}`, ticketBatch, pageInfoFields, labelConn.selection(), commentConn.selection(), blockerConn.selection(), prConn.selection())

// fetchMap reads one batch of the map's sub-issues, along with the map itself.
func (p *Puller) fetchMap(token, repo string, num int, after string) (*mapPage, error) {
	owner, name, _ := strings.Cut(repo, "/")
	vars := []string{"-f", "owner=" + owner, "-f", "name=" + name, "-F", "number=" + strconv.Itoa(num)}
	if after != "" {
		vars = append(vars, "-f", "after="+after)
	}
	var res struct {
		Repository struct {
			Issue *mapPage `json:"issue"`
		} `json:"repository"`
	}
	subject := fmt.Sprintf("%s#%d", repo, num)
	if err := graphql(p.GH, token, subject, &res, mapQuery, vars...); err != nil {
		return nil, err
	}
	m := res.Repository.Issue
	if m == nil {
		return nil, fmt.Errorf("gh %s: no issue in the response", subject)
	}
	// GraphQL enums are upper case; the mirror and the log use lower case.
	for i := range m.SubIssues.Nodes {
		t := &m.SubIssues.Nodes[i]
		t.State, t.StateReason = strings.ToLower(t.State), strings.ToLower(t.StateReason)
	}
	return m, nil
}

const morePageSize = 100

func (n nested) moreQuery() string {
	return fmt.Sprintf(`query WayfinderMore_%s($id: ID!, $after: String!) {
  node(id: $id) { ... on Issue { page: %s(first: %d, after: $after%s) { %s nodes { %s } } } }
}`, n.name, n.name, morePageSize, n.args, pageInfoFields, n.fields)
}

// drain appends the pages of c that did not fit in the batch.
func drain[T any](p *Puller, token, subject, id string, n nested, c *conn[T]) error {
	for c.PageInfo.HasNextPage {
		var res struct {
			Node struct {
				Page *conn[T] `json:"page"`
			} `json:"node"`
		}
		err := graphql(p.GH, token, subject, &res, n.moreQuery(), "-f", "id="+id, "-f", "after="+c.PageInfo.EndCursor)
		if err != nil {
			return err
		}
		if res.Node.Page == nil {
			return fmt.Errorf("gh %s: no %s page in the response", subject, n.name)
		}
		c.Nodes = append(c.Nodes, res.Node.Page.Nodes...)
		c.PageInfo = res.Node.Page.PageInfo
	}
	return nil
}

// complete fetches what the mirror needs of the ticket's overflowing connections.
func (p *Puller) complete(token, repo string, t *issue) error {
	subject := fmt.Sprintf("%s#%d", repo, t.Number)
	if err := drain(p, token, subject, t.ID, labelConn, &t.Labels); err != nil {
		return err
	}
	if err := drain(p, token, subject, t.ID, blockerConn, &t.BlockedBy); err != nil {
		return err
	}
	if typ, _ := ticketType(*t); typ == "implement" {
		if err := drain(p, token, subject, t.ID, prConn, &t.PRs); err != nil {
			return err
		}
	}
	if t.State == "closed" {
		return drain(p, token, subject, t.ID, commentConn, &t.Comments)
	}
	return nil
}

// graphql runs a query and decodes its data into v. subject names what is
// read in errors.
func graphql(gh Runner, token, subject string, v any, query string, vars ...string) error {
	b, err := gh(token, append([]string{"api", "graphql", "-f", "query=" + query}, vars...)...)
	if err != nil {
		var ge *GHError
		if errors.As(err, &ge) && ge.Endpoint == "graphql" {
			ge.Endpoint = subject
		}
		return err
	}
	res := struct {
		Data any `json:"data"`
	}{v}
	if err := json.Unmarshal(b, &res); err != nil {
		return fmt.Errorf("gh %s: bad JSON: %v", subject, err)
	}
	return nil
}

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
		progress = func(Step) {}
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
	case account == "" && p.DefaultAccount != "":
		auth, account = "gh", p.DefaultAccount
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

	progress(Step{Total: 1, Batches: 1})
	m, err := p.fetchMap(token, repo, mapNum, "")
	if err != nil {
		return err
	}
	if out == "" {
		out = filepath.Join(".plan", Slugify(m.Title))
	}
	if len(m.SubIssues.Nodes) == 0 && !m.SubIssues.PageInfo.HasNextPage {
		return userErr("issue #%d has no sub-issues", mapNum)
	}

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
	count := 0
	for page, batch := m, 1; ; batch++ {
		for _, t := range page.SubIssues.Nodes {
			if err := p.complete(token, repo, &t); err != nil {
				return err
			}
			link, err := p.writeTicket(repo, stage, t, &decisions, &implemented, &outOfScope)
			if err != nil {
				return err
			}
			re := regexp.MustCompile(fmt.Sprintf(`(?m)https://github\.com/%s/issues/%d([^0-9]|$)`, regexp.QuoteMeta(repo), t.Number))
			rewrites = append(rewrites, re)
			rewriteTo = append(rewriteTo, link+"${1}")
			count++
		}
		next := page.SubIssues.PageInfo
		if !next.HasNextPage {
			progress(Step{Done: count + 1, Total: count + 1, Batch: batch, Batches: batch})
			break
		}
		progress(Step{Done: count + 1, Total: max(page.SubIssues.TotalCount, count+1) + 1,
			Batch: batch, Batches: max((page.SubIssues.TotalCount+ticketBatch-1)/ticketBatch, batch+1)})
		if page, err = p.fetchMap(token, repo, mapNum, next.EndCursor); err != nil {
			return err
		}
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

	fmt.Fprintf(p.Stdout, "wayfinder-maps pull: %s#%d -> %s (%d tickets)\n", repo, mapNum, out, count)
	return nil
}

// writeTicket writes one ticket file and returns its relative link. It also
// files the ticket into the generated map sections.
func (p *Puller) writeTicket(repo, out string, t issue, decisions, implemented, outOfScope *strings.Builder) (string, error) {
	assignee := ""
	if len(t.Assignees.Nodes) > 0 {
		assignee = t.Assignees.Nodes[0].Login
	}
	typ, afk := ticketType(t)

	pr := ""
	if typ == "implement" {
		nodes := t.PRs.Nodes
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
	for _, b := range t.BlockedBy.Nodes {
		blockedBy = append(blockedBy, b.Number)
	}
	blockedJSON, _ := json.Marshal(blockedBy)

	slug := Slugify(t.Title)
	name := fmt.Sprintf("%d-%s.md", t.Number, slug)
	link := "./tickets/" + name

	var f strings.Builder
	fmt.Fprintf(&f, "---\ntype: %s\nblocked_by: %s\n", typ, blockedJSON)
	if t.CreatedAt != "" {
		fmt.Fprintf(&f, "created: %s\n", t.CreatedAt)
	}
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
		answer := closedAnswer(t, typ, pr)
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

var headingRE = regexp.MustCompile(`^#{1,6} `)

// demoteHeadings moves a comment's headings to level 3 or lower, keeping their
// hierarchy, since a "## " line ends the "## Answer" section. Code blocks stay
// as written.
func demoteHeadings(comment string) string {
	raw, scan := wayfinder.SplitScan(comment)
	by := 1
	for _, l := range scan {
		if strings.HasPrefix(l, "# ") {
			by = 2
		}
	}
	for i, l := range scan {
		if m := headingRE.FindString(l); m != "" {
			level := len(m) - 1
			raw[i] = strings.Repeat("#", min(level+by, 6)) + raw[i][level:]
		}
	}
	return strings.Join(raw, "\n")
}

// closedAnswer is the ticket's comments, joined, with their headings demoted.
func closedAnswer(t issue, typ, pr string) string {
	bodies := make([]string, len(t.Comments.Nodes))
	for i, c := range t.Comments.Nodes {
		bodies[i] = demoteHeadings(c.Body)
	}
	answer := strings.TrimRight(strings.Join(bodies, "\n\n---\n\n"), "\n")
	if answer != "" {
		return answer
	}
	if typ == "implement" && t.StateReason != "not_planned" && pr != "" {
		return "Shipped in " + pr + "."
	}
	return "_Closed on GitHub with no answer comment._"
}

// ticketType is the first wayfinder:<type> label other than afk; anything
// unknown is a grilling ticket.
func ticketType(t issue) (typ string, afk bool) {
	typ = "grilling"
	found := false
	for _, l := range t.Labels.Nodes {
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
