package ghpull

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func pullErr(t *testing.T, f *fakeGH, run Runner, out string) *Error {
	t.Helper()
	p, _, _ := f.puller(nil)
	if run != nil {
		p.GH = run
	}
	err := p.Pull(Options{Repo: "acme/widgets", Map: 10, Out: out})
	var pe *Error
	if !errors.As(err, &pe) {
		t.Fatalf("want a user-facing *Error, got %T: %v", err, err)
	}
	return pe
}

func TestGHFailuresAreExplained(t *testing.T) {
	f := newFakeGH(t, "full")
	for _, tc := range []struct {
		name string
		err  *GHError
		want string
	}{
		{"gh missing", &GHError{Kind: KindMissing, Endpoint: "user"}, "gh is not installed or not on PATH. Install it from https://cli.github.com"},
		{"not logged in", &GHError{Kind: KindNoAuth, Endpoint: "user"}, "gh is not logged in. Run: gh auth login"},
		{"expired", &GHError{Kind: KindForbidden, Status: 401, Endpoint: "x"}, "GitHub rejected gh account 'alice' (HTTP 401)"},
		{"no access", &GHError{Kind: KindForbidden, Status: 404, Endpoint: "repos/acme/widgets/issues/10"}, "cannot read repos/acme/widgets/issues/10 as gh account 'alice' (HTTP 404)"},
		{"forbidden", &GHError{Kind: KindForbidden, Status: 403, Endpoint: "repos/acme/widgets/issues/10"}, "(HTTP 403)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run := func(token string, args ...string) ([]byte, error) {
				if endpointOf(args) == "user" {
					return []byte("alice\n"), nil
				}
				return nil, tc.err
			}
			pe := pullErr(t, f, run, filepath.Join(t.TempDir(), "out"))
			if pe.Code != 2 || !strings.Contains(pe.Msg, tc.want) {
				t.Errorf("code %d: %q, want it to contain %q", pe.Code, pe.Msg, tc.want)
			}
		})
	}
}

func TestGHMissingOnRefresh(t *testing.T) {
	f := newFakeGH(t, "full")
	p, _, _ := f.puller(nil)
	out := filepath.Join(t.TempDir(), "out")
	if err := p.Pull(Options{Repo: "acme/widgets", Map: 10, Out: out}); err != nil {
		t.Fatal(err)
	}
	p.GH = func(string, ...string) ([]byte, error) { return nil, &GHError{Kind: KindMissing} }
	err := p.Pull(Options{Out: out, Refresh: true})
	if err == nil || !strings.Contains(err.Error(), "gh is not installed") {
		t.Errorf("got %v", err)
	}
}

func TestFailedPullKeepsPreviousMirror(t *testing.T) {
	f := newFakeGH(t, "full")
	p, _, _ := f.puller(nil)
	out := filepath.Join(t.TempDir(), "plan", "out")
	if err := p.Pull(Options{Repo: "acme/widgets", Map: 10, Out: out}); err != nil {
		t.Fatal(err)
	}
	before := readTree(t, out)

	run := func(token string, args ...string) ([]byte, error) {
		if strings.HasSuffix(endpointOf(args), "/issues/20/comments") {
			return nil, classify(args, errors.New("exit status 1"), "gh: Resource not accessible (HTTP 403)")
		}
		return f.run(token, args...)
	}
	pe := pullErr(t, f, run, out)
	if !strings.Contains(pe.Msg, "(HTTP 403)") {
		t.Errorf("message: %q", pe.Msg)
	}
	after := readTree(t, out)
	if len(after) != len(before) {
		t.Errorf("mirror changed: %d files before, %d after", len(before), len(after))
	}
	for name, b := range before {
		if after[name] != b {
			t.Errorf("%s changed", name)
		}
	}
	entries, _ := os.ReadDir(filepath.Dir(out))
	if len(entries) != 1 {
		t.Errorf("staging dir left behind: %v", entries)
	}
}

func TestClassifyGhStderr(t *testing.T) {
	for _, tc := range []struct {
		stderr string
		kind   Kind
		status int
	}{
		{"To get started with GitHub CLI, please run:  gh auth login\nAlternatively, populate the GH_TOKEN environment variable", KindNoAuth, 0},
		{"gh: Not Found (HTTP 404)", KindForbidden, 404},
		{"gh: Bad credentials (HTTP 401)", KindForbidden, 401},
		{"gh: Resource not accessible by personal access token (HTTP 403)", KindForbidden, 403},
		{"gh: Server Error (HTTP 502)", KindOther, 502},
		{"dial tcp: lookup api.github.com: no such host", KindOther, 0},
	} {
		e := classify([]string{"api", "repos/a/b/issues/1"}, errors.New("exit status 1"), tc.stderr)
		if e.Kind != tc.kind || e.Status != tc.status {
			t.Errorf("%q: kind %d status %d, want %d %d", tc.stderr, e.Kind, e.Status, tc.kind, tc.status)
		}
	}
}

func TestRealGHRunner(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a shell script as gh")
	}
	t.Setenv("PATH", t.TempDir())
	if _, err := GH("", "api", "user"); err == nil || err.(*GHError).Kind != KindMissing {
		t.Errorf("no gh on PATH: %v", err)
	}

	dir := t.TempDir()
	script := "#!/bin/sh\necho 'To get started with GitHub CLI, please run:  gh auth login' >&2\nexit 4\n"
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	if _, err := GH("", "api", "user"); err == nil || err.(*GHError).Kind != KindNoAuth {
		t.Errorf("exit 4: %v", err)
	}
}
