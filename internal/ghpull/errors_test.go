package ghpull

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
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
		{"no access", &GHError{Kind: KindForbidden, Status: 404, Endpoint: "repos/acme/widgets"}, "cannot read repos/acme/widgets as gh account 'alice' (HTTP 404)"},
		{"forbidden", &GHError{Kind: KindForbidden, Status: 403, Endpoint: "repos/acme/widgets"}, "(HTTP 403)"},
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
	f := fixtures(t)
	p, _, _ := f.puller(nil)
	out := filepath.Join(t.TempDir(), "plan", "out")
	if err := p.Pull(Options{Repo: "acme/widgets", Map: 10, Out: out}); err != nil {
		t.Fatal(err)
	}
	before := readTree(t, out)

	// The second batch fails after the first was written to the staging dir.
	f.pageSubIssues(5)
	run := func(token string, args ...string) ([]byte, error) {
		if slices.Contains(args, "after=c1") {
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

// graphqlFailure is a gh run GraphQL answered with errors: HTTP 200, so
// stderr carries only GitHub's message.
func graphqlFailure(args []string, stderr string) error {
	return classify(args, errors.New("exit status 1"), stderr)
}

func TestGraphQLFailuresAreExplained(t *testing.T) {
	const check = " Check the repo name and issue number, and that this account can see the repo; check with: gh auth status"
	for _, tc := range []struct {
		name, stderr, want string
	}{
		{"unknown repo", "gh: Could not resolve to a Repository with the name 'acme/widgets'.",
			"cannot read acme/widgets#10 as gh account 'alice': Could not resolve to a Repository with the name 'acme/widgets'." + check},
		{"unknown issue", "gh: Could not resolve to an Issue with the number of 10.",
			"cannot read acme/widgets#10 as gh account 'alice': Could not resolve to an Issue with the number of 10." + check},
		{"integration", "gh: Resource not accessible by integration",
			"cannot read acme/widgets#10 as gh account 'alice': Resource not accessible by integration." + check},
		{"SAML", "gh: Resource protected by organization SAML enforcement. You must grant your OAuth token access to this organization.",
			"You must grant your OAuth token access to this organization." + check},
		{"scopes", "gh: Your token has not been granted the required scopes to execute this query.",
			"cannot read acme/widgets#10 as gh account 'alice': Your token has not been granted the required scopes"},
		{"primary rate limit", "gh: API rate limit exceeded for user ID 1.",
			"GitHub rate limited gh account 'alice': API rate limit exceeded for user ID 1. Wait and pull again; see the reset time with: gh api rate_limit"},
		{"secondary rate limit", "gh: You have exceeded a secondary rate limit. Please wait a few minutes before you try again. (HTTP 403)",
			"GitHub rate limited gh account 'alice': You have exceeded a secondary rate limit."},
		{"too many requests", "gh: HTTP 429 (HTTP 429)", "GitHub rate limited gh account 'alice'"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run := func(token string, args ...string) ([]byte, error) {
				if endpointOf(args) == "user" {
					return []byte("alice\n"), nil
				}
				return nil, graphqlFailure(args, tc.stderr)
			}
			pe := pullErr(t, newFakeGH(t, "full"), run, filepath.Join(t.TempDir(), "out"))
			if pe.Code != 2 || !strings.Contains(pe.Msg, tc.want) {
				t.Errorf("code %d: %q, want it to contain %q", pe.Code, pe.Msg, tc.want)
			}
		})
	}
}

func TestOtherGraphQLFailureNamesTheTicket(t *testing.T) {
	f := fixtures(t)
	f.pageConn(14, "blockedBy", 1)
	p, _, _ := f.puller(nil)
	p.GH = func(token string, args ...string) ([]byte, error) {
		if slices.Contains(args, "id=I_14") {
			return nil, graphqlFailure(args, "gh: Something went wrong while executing your query.")
		}
		return f.run(token, args...)
	}
	out := filepath.Join(t.TempDir(), "out")
	err := p.Pull(Options{Repo: "acme/widgets", Map: 10, Out: out})
	if want := "gh acme/widgets#14: gh: Something went wrong while executing your query."; err == nil || err.Error() != want {
		t.Errorf("got %v, want %q", err, want)
	}
	if _, err := os.Stat(out); err == nil {
		t.Error("a partial mirror was swapped in")
	}
}

func TestListMapsFailureNamesTheRepo(t *testing.T) {
	_, err := ListMaps(func(_ string, args ...string) ([]byte, error) {
		return nil, graphqlFailure(args, "gh: Could not resolve to a Repository with the name 'acme/secret'.")
	}, "", "acme/secret")
	if got, want := Explain(err, "").Error(), "cannot read acme/secret as the active gh account: Could not resolve"; !strings.Contains(got, want) {
		t.Errorf("got %q, want it to contain %q", got, want)
	}
}
