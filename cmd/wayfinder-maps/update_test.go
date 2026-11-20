package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/RamsesKools/wayfinder-maps/internal/installmethod"
	"github.com/RamsesKools/wayfinder-maps/internal/updatecheck"
)

func releaseServer(t *testing.T, latest string, hits *atomic.Int32) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if latest == "" {
			http.Error(w, "rate limited", http.StatusForbidden)
			return
		}
		http.Redirect(w, r, "/releases/tag/"+latest, http.StatusFound)
	}))
	t.Cleanup(srv.Close)
	return srv.URL + "/releases"
}

func testChecker(t *testing.T, url string) *updatecheck.Checker {
	return &updatecheck.Checker{ReleasesURL: url, CacheFile: filepath.Join(t.TempDir(), "c.json"), Method: installmethod.Homebrew, Current: "0.13.0"}
}

func TestNotice(t *testing.T) {
	on := settings{UpdateCheck: true}
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

	tests := []struct {
		name   string
		getenv map[string]string
		tty    bool
		set    settings
		latest string
		want   bool
		hits   int32
	}{
		{"outdated on a terminal", nil, true, on, "v0.14.0", true, 1},
		{"up to date", nil, true, on, "v0.13.0", false, 1},
		{"offline or rate limited", nil, true, on, "", false, 1},
		{"stderr piped", nil, false, on, "v0.14.0", false, 0},
		{"CI", map[string]string{"CI": "true"}, true, on, "v0.14.0", false, 0},
		{"env silenced", map[string]string{"WAYFINDER_NO_UPDATE_CHECK": "1"}, true, on, "v0.14.0", false, 0},
		{"settings opt-out", nil, true, settings{}, "v0.14.0", false, 0},
	}
	for _, tt := range tests {
		var hits atomic.Int32
		var out bytes.Buffer
		n := notifier{getenv: env(tt.getenv), tty: tt.tty, settings: tt.set, checker: testChecker(t, releaseServer(t, tt.latest, &hits)), out: &out}
		n.start()()
		if got := strings.Contains(out.String(), "is outdated, v0.14.0 is available."); got != tt.want {
			t.Errorf("%s: printed %q", tt.name, out.String())
		}
		if tt.want && !strings.Contains(out.String(), "Update: brew upgrade wayfinder-maps\nSilence: export WAYFINDER_NO_UPDATE_CHECK=1\n") {
			t.Errorf("%s: notice lacks hint lines: %q", tt.name, out.String())
		}
		if hits.Load() != tt.hits {
			t.Errorf("%s: hits = %d, want %d", tt.name, hits.Load(), tt.hits)
		}
	}
}

func TestNoticeUsesCache(t *testing.T) {
	var hits atomic.Int32
	c := testChecker(t, releaseServer(t, "v0.14.0", &hits))
	for i := 0; i < 3; i++ {
		var out bytes.Buffer
		(notifier{getenv: func(string) string { return "" }, tty: true, settings: settings{UpdateCheck: true}, checker: c, out: &out}).start()()
		if out.Len() == 0 {
			t.Fatalf("run %d printed nothing", i)
		}
	}
	if hits.Load() != 1 {
		t.Errorf("hits = %d, want 1", hits.Load())
	}
}

func TestUpdateEndpoint(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("HOME", cfg)
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("AppData", cfg)
	t.Setenv("WAYFINDER_NO_UPDATE_CHECK", "")

	var hits atomic.Int32
	c := testChecker(t, releaseServer(t, "v0.14.0", &hits))
	old := updateChecker
	updateChecker = func() *updatecheck.Checker { return c }
	t.Cleanup(func() { updateChecker = old })
	h := newServer("")

	get := func() string {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/update", nil))
		return strings.TrimSpace(rec.Body.String())
	}
	want := `{"enabled":true,"envOff":false,"current":"0.13.0","latest":"v0.14.0","outdated":true,"hint":"brew upgrade wayfinder-maps"}`
	for i := 0; i < 3; i++ {
		if b := get(); b != want {
			t.Fatalf("get %d = %s", i, b)
		}
	}
	if hits.Load() != 1 {
		t.Errorf("hits = %d, reloads must not hit the release server", hits.Load())
	}

	s := loadSettings()
	s.UpdateCheck = false
	saveSettings(s)
	if b := get(); b != `{"enabled":false,"envOff":false,"current":"0.13.0"}` {
		t.Errorf("opted out = %s", b)
	}
	s.UpdateCheck = true
	saveSettings(s)
	t.Setenv("WAYFINDER_NO_UPDATE_CHECK", "1")
	if b := get(); b != `{"enabled":false,"envOff":true,"current":"0.13.0"}` {
		t.Errorf("env silenced = %s", b)
	}
	if hits.Load() != 1 {
		t.Errorf("hits = %d after toggling, want 1", hits.Load())
	}
}
