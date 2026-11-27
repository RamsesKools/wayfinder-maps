package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RamsesKools/wayfinder-maps/internal/ghpull"
)

// isolateGitHub points the cache and config dirs at temp dirs and swaps gh for
// a fake that answers by the first arguments it sees.
func isolateGitHub(t *testing.T, gh ghpull.Runner) (cache string) {
	t.Helper()
	isolateConfig(t)
	cache = t.TempDir()
	oldCache, oldGH := userCacheDir, ghRunner
	userCacheDir, ghRunner = func() (string, error) { return cache, nil }, gh
	t.Cleanup(func() { userCacheDir, ghRunner = oldCache, oldGH })
	return cache
}

func call(h http.Handler, method, target string, header bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	if header {
		req.Header.Set("X-Wayfinder", "1")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func fakeMirror(t *testing.T, cache, repo, title string, num int) string {
	t.Helper()
	dir := ghpull.MirrorDir(cache, repo, title, num)
	if err := os.MkdirAll(filepath.Join(dir, "tickets"), 0o755); err != nil {
		t.Fatal(err)
	}
	src, _ := json.Marshal(ghpull.Source{Repo: repo, Map: num, Auth: "gh", Account: "alice"})
	if err := os.WriteFile(filepath.Join(dir, ".source.json"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "map.md"), []byte("# "+title+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestGitHubEndpointsNeedTheCustomHeader(t *testing.T) {
	isolateGitHub(t, func(string, ...string) ([]byte, error) { return []byte("{}"), nil })
	h := newServer("")

	for _, c := range []struct{ method, target string }{
		{"GET", "/api/github/status"},
		{"GET", "/api/github/repos"},
		{"GET", "/api/github/maps?repo=acme/widgets"},
		{"GET", "/api/github/cached?repo=acme/widgets&map=1"},
		{"POST", "/api/github/open?repo=acme/widgets&map=1&title=x"},
		{"POST", "/api/recents/cleanup?project=/x"},
	} {
		if rec := call(h, c.method, c.target, false); rec.Code != http.StatusBadRequest {
			t.Errorf("%s %s without header = %d, want 400", c.method, c.target, rec.Code)
		}
	}
}

func TestGitHubStatusEndpoint(t *testing.T) {
	isolateGitHub(t, func(_ string, args ...string) ([]byte, error) {
		return []byte(`{"hosts":{"github.com":[{"state":"success","active":true,"host":"github.com","login":"alice"}]}}`), nil
	})

	rec := call(newServer(""), "GET", "/api/github/status", true)

	var st ghpull.Status
	if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil || !st.LoggedIn || st.Account != "alice" {
		t.Errorf("status = %+v, err = %v, body %s", st, err, rec.Body)
	}
}

func TestGitHubReposListsRememberedReposFirstThenRecentOnesWithoutDuplicates(t *testing.T) {
	var gotToken string
	isolateGitHub(t, func(token string, args ...string) ([]byte, error) {
		if args[0] == "auth" {
			return []byte("tok-bob\n"), nil
		}
		gotToken = token
		if args[1] == "user" {
			return []byte("bob\n"), nil
		}
		return []byte(`[{"type":"PushEvent","repo":{"name":"acme/widgets"}},{"type":"PushEvent","repo":{"name":"acme/gadgets"}}]`), nil
	})
	rememberRepo("zed/old")
	rememberRepo("acme/gadgets")

	rec := call(newServer(""), "GET", "/api/github/repos?account=bob", true)

	var repos []string
	if err := json.Unmarshal(rec.Body.Bytes(), &repos); err != nil {
		t.Fatal(err)
	}
	if strings.Join(repos, ",") != "acme/gadgets,zed/old,acme/widgets" {
		t.Errorf("repos = %v", repos)
	}
	if gotToken != "tok-bob" {
		t.Errorf("listed as token %q, want bob's", gotToken)
	}
}

func TestGitHubReposStillAnswersWithRememberedReposWhenGitHubFails(t *testing.T) {
	isolateGitHub(t, func(string, ...string) ([]byte, error) { return nil, errBoom })
	rememberRepo("zed/old")

	rec := call(newServer(""), "GET", "/api/github/repos", true)

	var repos []string
	if err := json.Unmarshal(rec.Body.Bytes(), &repos); err != nil || len(repos) != 1 || repos[0] != "zed/old" {
		t.Errorf("repos = %v, err = %v, code %d", repos, err, rec.Code)
	}
}

func TestGitHubMapsEndpointValidatesTheRepoAndListsMaps(t *testing.T) {
	isolateGitHub(t, func(_ string, args ...string) ([]byte, error) {
		return []byte(`{"data":{"repository":{"issues":{"nodes":[{"number":2,"title":"Live map","state":"OPEN",
		  "subIssuesSummary":{"total":4,"completed":3,"percentCompleted":75},
		  "subIssues":{"nodes":[{"labels":{"nodes":[{"name":"wayfinder:research"}]}}]}}]}}}}`), nil
	})
	h := newServer("")

	rec := call(h, "GET", "/api/github/maps?repo="+url.QueryEscape("https://github.com/acme/widgets.git"), true)
	var res struct {
		Repo string
		Maps []ghpull.MapInfo
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil || res.Repo != "acme/widgets" || len(res.Maps) != 1 || res.Maps[0].Percent != 75 || res.Maps[0].Number != 2 {
		t.Errorf("res = %+v, err = %v, body %s", res, err, rec.Body)
	}

	if rec := call(h, "GET", "/api/github/maps?repo=nonsense", true); rec.Code != http.StatusBadRequest {
		t.Errorf("bad repo = %d, want 400", rec.Code)
	}
}

func TestOpeningFromGitHubPullsIntoTheCacheRemembersTheRepoAndStreamsTheResult(t *testing.T) {
	cache := isolateGitHub(t, func(string, ...string) ([]byte, error) { return nil, errBoom })
	var got ghpull.Options
	oldOpen := runOpen
	runOpen = func(o ghpull.Options, progress func(ghpull.Step)) error {
		got = o
		progress(ghpull.Step{Done: 1, Total: 2, Batches: 1})
		fakeMirror(t, cache, "acme/widgets", "Live map", 2)
		return nil
	}
	t.Cleanup(func() { runOpen = oldOpen })

	rec := call(newServer(""), "POST", "/api/github/open?repo=acme/widgets&map=2&title=Live+map&account=alice", true)

	want := ghpull.MirrorDir(cache, "acme/widgets", "Live map", 2)
	if got.Repo != "acme/widgets" || got.Map != 2 || got.Out != want || got.Account != "alice" || got.Refresh {
		t.Errorf("pull options = %+v, want out %q", got, want)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "event: progress") || !strings.Contains(body, "event: done") || !strings.Contains(body, strings.ReplaceAll(want, `\`, `\\`)) {
		t.Errorf("stream = %q", body)
	}
	if repos := rememberedRepos(); len(repos) != 1 || repos[0] != "acme/widgets" {
		t.Errorf("remembered = %v", repos)
	}
}

func TestOpeningFromGitHubReportsAFailedPullAsAnErrorEventAndRemembersNothing(t *testing.T) {
	isolateGitHub(t, func(string, ...string) ([]byte, error) { return nil, errBoom })
	oldOpen := runOpen
	runOpen = func(ghpull.Options, func(ghpull.Step)) error { return errBoom }
	t.Cleanup(func() { runOpen = oldOpen })

	rec := call(newServer(""), "POST", "/api/github/open?repo=acme/widgets&map=2&title=x", true)

	if body := rec.Body.String(); !strings.Contains(body, "event: error") || strings.Contains(body, "event: done") {
		t.Errorf("stream = %q", body)
	}
	if repos := rememberedRepos(); len(repos) != 0 {
		t.Errorf("remembered = %v", repos)
	}
}

func TestOpeningFromGitHubRejectsBadInput(t *testing.T) {
	isolateGitHub(t, func(string, ...string) ([]byte, error) { return nil, errBoom })
	h := newServer("")

	for _, target := range []string{
		"/api/github/open?repo=nonsense&map=2&title=x",
		"/api/github/open?repo=acme/widgets&map=zero&title=x",
		"/api/github/open?repo=acme/widgets&map=-1&title=x",
	} {
		if rec := call(h, "POST", target, true); rec.Code != http.StatusBadRequest {
			t.Errorf("%s = %d, want 400", target, rec.Code)
		}
	}
}

func TestACachedMirrorShowsInRecentsAsCachedAndCleanupDeletesItAndTheEntry(t *testing.T) {
	cache := isolateGitHub(t, func(string, ...string) ([]byte, error) { return nil, errBoom })
	h := newServer("")
	mirror := fakeMirror(t, cache, "acme/widgets", "Live map", 2)
	plain := demoMap(t)
	call(h, "GET", "/api/graph?open=1&effort="+url.QueryEscape(plain), false)
	call(h, "GET", "/api/graph?open=1&effort="+url.QueryEscape(mirror), false)

	var recents []recentEntry
	json.Unmarshal(call(h, "GET", "/api/recents", false).Body.Bytes(), &recents)
	if len(recents) != 2 || recents[0].Path != mirror || !recents[0].Cached || recents[1].Cached {
		t.Fatalf("recents = %+v, want the mirror first and flagged cached", recents)
	}

	rec := call(h, "POST", "/api/recents/cleanup?project="+url.QueryEscape(mirror), true)

	if rec.Code != http.StatusOK {
		t.Fatalf("cleanup = %d %s", rec.Code, rec.Body)
	}
	if _, err := os.Stat(mirror); !os.IsNotExist(err) {
		t.Errorf("mirror still on disk: %v", err)
	}
	var left []recentEntry
	json.Unmarshal(rec.Body.Bytes(), &left)
	if len(left) != 1 || left[0].Path != plain {
		t.Errorf("recents after cleanup = %+v", left)
	}
}

func TestCleanupRefusesAPathThatIsNotACachedMirror(t *testing.T) {
	isolateGitHub(t, func(string, ...string) ([]byte, error) { return nil, errBoom })
	h := newServer("")
	plain := demoMap(t)
	call(h, "GET", "/api/graph?open=1&effort="+url.QueryEscape(plain), false)

	rec := call(h, "POST", "/api/recents/cleanup?project="+url.QueryEscape(plain), true)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("cleanup of a user's own map = %d, want 400", rec.Code)
	}
	if _, err := os.Stat(filepath.Join(plain, "map.md")); err != nil {
		t.Errorf("a non-cached map was touched: %v", err)
	}
	if got := loadRecents(); len(got) != 1 {
		t.Errorf("recents = %+v", got)
	}
}

func TestForgettingARecentNeverDeletesTheCachedFiles(t *testing.T) {
	cache := isolateGitHub(t, func(string, ...string) ([]byte, error) { return nil, errBoom })
	h := newServer("")
	mirror := fakeMirror(t, cache, "acme/widgets", "Live map", 2)
	call(h, "GET", "/api/graph?open=1&effort="+url.QueryEscape(mirror), false)

	call(h, "POST", "/api/recents/remove?project="+url.QueryEscape(mirror), false)

	if _, err := os.Stat(filepath.Join(mirror, "map.md")); err != nil {
		t.Errorf("forget touched the files: %v", err)
	}
	if got := loadRecents(); len(got) != 0 {
		t.Errorf("recents = %+v", got)
	}
}

var errBoom = errors.New("boom")

func TestGitHubCachedReportsAnExistingMirrorOfTheMap(t *testing.T) {
	isolateGitHub(t, func(string, ...string) ([]byte, error) { return nil, errBoom })
	cache, _ := userCacheDir()
	dir := fakeMirror(t, cache, "acme/widgets", "Live map", 2)
	get := func(target string) (res struct {
		Cached bool   `json:"cached"`
		Path   string `json:"path"`
	}) {
		rec := call(newServer(""), "GET", target, true)
		if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil || rec.Code != 200 {
			t.Fatalf("%s: %d %s", target, rec.Code, rec.Body)
		}
		return res
	}

	if r := get("/api/github/cached?repo=https://github.com/acme/widgets&map=2"); !r.Cached || r.Path != dir {
		t.Errorf("cached = %+v, want %q", r, dir)
	}
	if r := get("/api/github/cached?repo=acme/widgets&map=5"); r.Cached || r.Path != "" {
		t.Errorf("an unpulled map reported as cached: %+v", r)
	}
}
