package updatecheck

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/RamsesKools/wayfinder-maps/internal/installmethod"
)

func fakeReleases(t *testing.T, latest string, hits *atomic.Int32) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if latest == "" {
			http.Error(w, "nope", http.StatusForbidden)
			return
		}
		http.Redirect(w, r, "/releases/tag/"+latest, http.StatusFound)
	}))
	t.Cleanup(srv.Close)
	return srv.URL + "/releases"
}

func newChecker(t *testing.T, url string, now *time.Time) *Checker {
	return &Checker{
		ReleasesURL: url,
		CacheFile:   filepath.Join(t.TempDir(), "latest.json"),
		Method:      installmethod.Script,
		Current:     "0.13.0",
		Now:         func() time.Time { return *now },
	}
}

func TestCheckCachesWithinTTL(t *testing.T) {
	var hits atomic.Int32
	now := time.Now()
	c := newChecker(t, fakeReleases(t, "v0.14.0", &hits), &now)

	for i := 0; i < 3; i++ {
		r, err := c.Check(context.Background())
		if err != nil || r.Latest != "v0.14.0" || !r.Outdated {
			t.Fatalf("check %d = %+v, %v", i, r, err)
		}
	}
	if hits.Load() != 1 {
		t.Errorf("hits = %d, want 1 within the cache window", hits.Load())
	}

	now = now.Add(25 * time.Hour)
	c.Check(context.Background())
	if hits.Load() != 2 {
		t.Errorf("hits = %d after TTL, want 2", hits.Load())
	}
}

func TestCachedNeverFetches(t *testing.T) {
	var hits atomic.Int32
	now := time.Now()
	c := newChecker(t, fakeReleases(t, "v0.14.0", &hits), &now)
	if _, ok := c.Cached(); ok {
		t.Error("empty cache reported a result")
	}
	c.Check(context.Background())
	r, ok := c.Cached()
	if !ok || r.Latest != "v0.14.0" || hits.Load() != 1 {
		t.Errorf("Cached = %+v %v, hits %d", r, ok, hits.Load())
	}
}

func TestCheckUpToDateAndAhead(t *testing.T) {
	var hits atomic.Int32
	now := time.Now()
	c := newChecker(t, fakeReleases(t, "v0.13.0", &hits), &now)
	if r, _ := c.Check(context.Background()); r.Outdated || r.Latest != "v0.13.0" {
		t.Errorf("same version = %+v", r)
	}
	c.Current = "0.14.0"
	if r, _ := c.Check(context.Background()); r.Outdated {
		t.Errorf("newer than latest = %+v", r)
	}
}

func TestFailureIsCachedBriefly(t *testing.T) {
	var hits atomic.Int32
	now := time.Now()
	c := newChecker(t, fakeReleases(t, "", &hits), &now)
	if _, err := c.Check(context.Background()); err == nil {
		t.Fatal("want an error")
	}
	c.Check(context.Background())
	if hits.Load() != 1 {
		t.Errorf("hits = %d, a failure should not be retried at once", hits.Load())
	}
	now = now.Add(2 * time.Hour)
	c.Check(context.Background())
	if hits.Load() != 2 {
		t.Errorf("hits = %d, a failure should be retried after an hour", hits.Load())
	}
}

func TestCacheKeyedByURL(t *testing.T) {
	var hits atomic.Int32
	now := time.Now()
	c := newChecker(t, fakeReleases(t, "v0.14.0", &hits), &now)
	c.Check(context.Background())
	c.ReleasesURL = fakeReleases(t, "v0.20.0", &hits)
	if r, _ := c.Check(context.Background()); r.Latest != "v0.20.0" {
		t.Errorf("other mirror served from cache: %+v", r)
	}
}

func TestUnreachableIsAnError(t *testing.T) {
	now := time.Now()
	c := newChecker(t, "http://127.0.0.1:1/releases", &now)
	if _, err := c.Check(context.Background()); err == nil {
		t.Error("want an error")
	}
}

func TestDevBuildNeverChecks(t *testing.T) {
	var hits atomic.Int32
	now := time.Now()
	c := newChecker(t, fakeReleases(t, "v9.9.9", &hits), &now)
	c.Method = installmethod.Dev
	if _, err := c.Check(context.Background()); err == nil || hits.Load() != 0 {
		t.Errorf("dev check: err %v, hits %d", err, hits.Load())
	}
}

func TestNotice(t *testing.T) {
	r := Result{Current: "0.13.0", Latest: "v0.14.0", Outdated: true, Hint: "brew upgrade wayfinder-maps"}
	want := "wayfinder-maps v0.13.0 is outdated, v0.14.0 is available.\nUpdate: brew upgrade wayfinder-maps\nSilence: set \"updateCheck\": false in /cfg/settings.json\n"
	if got := r.Notice("/cfg/settings.json"); got != want {
		t.Errorf("Notice = %q", got)
	}
	if (Result{Current: "0.14.0", Latest: "v0.14.0"}).Notice("/cfg/settings.json") != "" {
		t.Error("up to date result has a notice")
	}
}

func TestShouldNotify(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	tests := []struct {
		name string
		env  map[string]string
		tty  bool
		want bool
	}{
		{"terminal", nil, true, true},
		{"piped", nil, false, false},
		{"ci", map[string]string{"CI": "true"}, true, false},
	}
	for _, tt := range tests {
		if got := ShouldNotify(env(tt.env), tt.tty); got != tt.want {
			t.Errorf("%s: %v, want %v", tt.name, got, tt.want)
		}
	}
}
