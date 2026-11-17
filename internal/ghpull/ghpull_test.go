package ghpull

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// fakeGH serves canned gh responses from testdata/<scenario>/gh, the same
// files the old bash script was run against (via a fake gh on PATH) to
// produce the golden trees.
type fakeGH struct {
	t     *testing.T
	dir   string
	calls []call
	// accounts are the logged-in gh accounts; "" means gh api user fails.
	accounts map[string]bool
	login    string
}

type call struct {
	token string
	args  []string
}

func newFakeGH(t *testing.T, scenario string) *fakeGH {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("testdata", scenario, "gh"))
	if err != nil {
		t.Fatal(err)
	}
	login, _ := os.ReadFile(filepath.Join(dir, "user.txt"))
	return &fakeGH{t: t, dir: dir, login: strings.TrimSpace(string(login)), accounts: map[string]bool{"alice": true}}
}

func (f *fakeGH) run(token string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, call{token, args})
	if len(args) >= 4 && args[0] == "auth" && args[1] == "token" {
		if !f.accounts[args[3]] {
			return nil, errors.New("no token found for " + args[3])
		}
		return []byte("tok-" + args[3] + "\n"), nil
	}
	var endpoint, number string
	slurp := false
	for i := 1; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--slurp":
			slurp = true
		case a == "--jq":
			i++
		case a == "-f" || a == "-F":
			i++
			if v, ok := strings.CutPrefix(args[i], "number="); ok {
				number = v
			}
		case !strings.HasPrefix(a, "-"):
			endpoint = a
		}
	}
	var file string
	switch endpoint {
	case "user":
		if f.login == "" {
			return nil, errors.New("not logged in")
		}
		return []byte(f.login + "\n"), nil
	case "graphql":
		file = "graphql__" + number + ".json"
	default:
		file = strings.ReplaceAll(endpoint, "/", "__") + ".json"
	}
	b, err := os.ReadFile(filepath.Join(f.dir, file))
	if err != nil {
		return nil, fmt.Errorf("HTTP 404: %s", endpoint)
	}
	if slurp {
		b = append(append([]byte("["), b...), ']')
	}
	return b, nil
}

func (f *fakeGH) puller(env map[string]string) (*Puller, *bytes.Buffer, *bytes.Buffer) {
	var out, errw bytes.Buffer
	return &Puller{GH: f.run, Getenv: func(k string) string { return env[k] }, Stdout: &out, Stderr: &errw}, &out, &errw
}

func readTree(t *testing.T, root string) map[string]string {
	t.Helper()
	tree := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		rel, _ := filepath.Rel(root, p)
		tree[filepath.ToSlash(rel)] = string(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

func assertTree(t *testing.T, got, want string) {
	t.Helper()
	g, w := readTree(t, got), readTree(t, want)
	for name, wb := range w {
		gb, ok := g[name]
		if !ok {
			t.Errorf("missing %s", name)
		} else if gb != wb {
			t.Errorf("%s differs\n--- got\n%s\n--- want\n%s", name, gb, wb)
		}
	}
	for name := range g {
		if _, ok := w[name]; !ok {
			t.Errorf("unexpected %s", name)
		}
	}
}

func golden(t *testing.T, scenario, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", scenario, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestPullMatchesBashGolden(t *testing.T) {
	for _, tc := range []struct {
		scenario string
		mapNum   int
	}{{"full", 10}, {"sections", 30}} {
		t.Run(tc.scenario, func(t *testing.T) {
			f := newFakeGH(t, tc.scenario)
			p, stdout, stderr := f.puller(nil)
			out := filepath.Join(t.TempDir(), "out")
			if err := p.Pull(Options{Repo: "acme/widgets", Map: tc.mapNum, Out: out}); err != nil {
				t.Fatal(err)
			}
			assertTree(t, out, filepath.Join("testdata", tc.scenario, "golden"))
			if got, want := strings.ReplaceAll(stdout.String(), out, "OUT"), golden(t, tc.scenario, "stdout.golden"); got != want {
				t.Errorf("stdout\n--- got\n%s\n--- want\n%s", got, want)
			}
			if got, want := stderr.String(), golden(t, tc.scenario, "stderr.golden"); got != want {
				t.Errorf("stderr\n--- got\n%s\n--- want\n%s", got, want)
			}
		})
	}
}

func TestRefreshUsesRecordedAccount(t *testing.T) {
	f := newFakeGH(t, "full")
	p, _, _ := f.puller(nil)
	out := filepath.Join(t.TempDir(), "out")
	if err := p.Pull(Options{Repo: "acme/widgets", Map: 10, Out: out}); err != nil {
		t.Fatal(err)
	}
	for _, c := range f.calls {
		if c.token != "" {
			t.Fatalf("first pull passed a token: %v", c)
		}
	}

	f.calls = nil
	if err := p.Pull(Options{Out: out, Refresh: true}); err != nil {
		t.Fatal(err)
	}
	if len(f.calls) < 2 || strings.Join(f.calls[0].args, " ") != "auth token --user alice" {
		t.Fatalf("refresh should look up alice's token first: %v", f.calls)
	}
	for _, c := range f.calls[1:] {
		if c.token != "tok-alice" {
			t.Errorf("call without alice's token: %v", c)
		}
	}
	assertTree(t, out, filepath.Join("testdata", "full", "golden"))
}

func TestRefreshAccountNotLoggedIn(t *testing.T) {
	f := newFakeGH(t, "full")
	p, _, _ := f.puller(nil)
	out := filepath.Join(t.TempDir(), "out")
	if err := p.Pull(Options{Repo: "acme/widgets", Map: 10, Out: out}); err != nil {
		t.Fatal(err)
	}
	f.accounts = map[string]bool{}
	f.calls = nil
	err := p.Pull(Options{Out: out, Refresh: true})
	var pe *Error
	if !errors.As(err, &pe) || pe.Code != 2 || pe.Msg != "gh account 'alice' is not logged in" {
		t.Fatalf("got %v", err)
	}
	if len(f.calls) != 1 {
		t.Errorf("should stop before touching GitHub: %v", f.calls)
	}
	if _, err := os.Stat(filepath.Join(out, "map.md")); err != nil {
		t.Errorf("mirror was touched: %v", err)
	}
}

func TestAuthKind(t *testing.T) {
	read := func(t *testing.T, env map[string]string) Source {
		f := newFakeGH(t, "full")
		p, _, _ := f.puller(env)
		out := filepath.Join(t.TempDir(), "out")
		if err := p.Pull(Options{Repo: "acme/widgets", Map: 10, Out: out}); err != nil {
			t.Fatal(err)
		}
		src, ok := ReadSource(out)
		if !ok {
			t.Fatal("no source")
		}
		return src
	}
	if s := read(t, map[string]string{"GITHUB_TOKEN": "x"}); s.Auth != "env" || s.Account != "" {
		t.Errorf("env token: %+v", s)
	}
	if s := read(t, nil); s.Auth != "gh" || s.Account != "alice" {
		t.Errorf("gh login: %+v", s)
	}
}

func TestRefreshWithoutSource(t *testing.T) {
	p, _, _ := newFakeGH(t, "full").puller(nil)
	for _, out := range []string{"", t.TempDir()} {
		var pe *Error
		if err := p.Pull(Options{Out: out, Refresh: true}); !errors.As(err, &pe) || pe.Code != 2 {
			t.Errorf("out=%q: %v", out, err)
		}
	}
}

func TestUsageAndEmptyMap(t *testing.T) {
	f := newFakeGH(t, "full")
	p, _, _ := f.puller(nil)
	var pe *Error
	if err := p.Pull(Options{}); !errors.As(err, &pe) || pe.Code != 2 {
		t.Errorf("usage: %v", err)
	}
	if len(f.calls) != 0 {
		t.Errorf("usage error should not call gh: %v", f.calls)
	}

	empty := newFakeGH(t, "full")
	empty.dir = t.TempDir()
	for name, body := range map[string]string{
		"repos__acme__widgets__issues__9.json":             `{"number":9,"title":"Empty","body":""}`,
		"repos__acme__widgets__issues__9__sub_issues.json": `[]`,
	} {
		if err := os.WriteFile(filepath.Join(empty.dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	p, _, _ = empty.puller(nil)
	out := filepath.Join(t.TempDir(), "out")
	err := p.Pull(Options{Repo: "acme/widgets", Map: 9, Out: out})
	if !errors.As(err, &pe) || pe.Code != 2 || !strings.Contains(pe.Msg, "no sub-issues") {
		t.Errorf("empty map: %v", err)
	}
	if _, err := os.Stat(out); err == nil {
		t.Error("out dir created for an empty map")
	}
}

func TestDefaultOutAndPagination(t *testing.T) {
	f := newFakeGH(t, "full")
	two := func(token string, args ...string) ([]byte, error) {
		if strings.HasSuffix(args[len(args)-1], "/issues/10/sub_issues") {
			return []byte(`[[{"number":1,"title":"A","state":"open"}],[{"number":2,"title":"B","state":"open"}]]`), nil
		}
		return f.run(token, args...)
	}
	p, _, _ := f.puller(nil)
	p.GH = two
	t.Chdir(t.TempDir())
	if err := p.Pull(Options{Repo: "acme/widgets", Map: 10}); err != nil {
		t.Fatal(err)
	}
	tree := readTree(t, filepath.Join(".plan", "billing-rewrite-phase-1"))
	for _, name := range []string{"tickets/1-a.md", "tickets/2-b.md", "map.md", ".source.json"} {
		if _, ok := tree[name]; !ok {
			t.Errorf("missing %s in %v", name, tree)
		}
	}
}

func TestSlugify(t *testing.T) {
	long := "A very long title that keeps going well past the fifty character cap"
	for in, want := range map[string]string{
		"Should invoices be immutable?": "should-invoices-be-immutable",
		"  --Hello,  World!!  ":         "hello-world",
		"Épique & more":                 "pique-more",
		long:                            "a-very-long-title-that-keeps-going-well-past-the-f",
		"":                              "",
	} {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
	if !regexp.MustCompile(`^[a-z0-9-]{0,50}$`).MatchString(slugify(strings.Repeat("ab ", 40))) {
		t.Error("slug not capped")
	}
}
