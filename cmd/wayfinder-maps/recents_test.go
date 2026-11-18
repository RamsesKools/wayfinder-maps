package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func isolateConfig(t *testing.T) {
	t.Helper()
	cfg := t.TempDir()
	t.Setenv("HOME", cfg)
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("AppData", cfg)
}

func demoMap(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("..", "..", "internal", "wayfinder", "testdata", "implement-demo"))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestServingAMapRecordsItAsRecent(t *testing.T) {
	isolateConfig(t)
	m := demoMap(t)

	if _, _, err := startAppServer(m); err != nil {
		t.Fatal(err)
	}

	got := loadRecents()
	if len(got) != 1 || got[0].Path != m || got[0].Kind != "map" || got[0].Name != "Release notes generator" || got[0].Maps != 1 {
		t.Errorf("recents = %+v", got)
	}
}

func TestUnloadableOrMissingPathIsNotRecorded(t *testing.T) {
	isolateConfig(t)
	broken := t.TempDir()
	if err := os.WriteFile(filepath.Join(broken, "map.md"), []byte("# Broken\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, p := range []string{broken, filepath.Join(t.TempDir(), "nope")} {
		if _, _, err := startAppServer(p); err != nil {
			t.Fatal(err)
		}
	}

	if got := loadRecents(); len(got) != 0 {
		t.Errorf("recents = %+v, want none", got)
	}
}

func TestFailedListenIsNotRecorded(t *testing.T) {
	isolateConfig(t)
	busy, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()

	if _, err := listenAndRecord(busy.Addr().String(), demoMap(t)); err == nil {
		t.Fatal("want a listen error on a busy port")
	}

	if got := loadRecents(); len(got) != 0 {
		t.Errorf("recents = %+v, want none", got)
	}
}

func get(h http.Handler, target string) int {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec.Code
}

func TestOpeningAMapInTheViewerRecordsItButPollingDoesNot(t *testing.T) {
	isolateConfig(t)
	h := newServer("")
	m := demoMap(t)

	if c := get(h, "/api/graph?effort="+url.QueryEscape(m)); c != http.StatusOK {
		t.Fatalf("graph = %d", c)
	}
	if got := loadRecents(); len(got) != 0 {
		t.Fatalf("a plain graph fetch (the poller) recorded %+v", got)
	}

	if c := get(h, "/api/graph?open=1&effort="+url.QueryEscape(m)); c != http.StatusOK {
		t.Fatalf("graph = %d", c)
	}
	if got := loadRecents(); len(got) != 1 || got[0].Path != m || got[0].Kind != "map" {
		t.Errorf("recents = %+v", got)
	}
}

func TestViewerOpenOfUnloadableMapIsNotRecorded(t *testing.T) {
	isolateConfig(t)
	h := newServer("")

	if c := get(h, "/api/graph?open=1&effort="+url.QueryEscape(t.TempDir())); c == http.StatusOK {
		t.Fatalf("graph of an empty dir = %d", c)
	}

	if got := loadRecents(); len(got) != 0 {
		t.Errorf("recents = %+v, want none", got)
	}
}

func TestRecentsKeepProjectsAndMapsDedupedNewestFirstAndDropGoneFolders(t *testing.T) {
	isolateConfig(t)
	m := demoMap(t)
	project := t.TempDir()
	gone := filepath.Join(project, ".plan", "gone")
	if err := os.MkdirAll(filepath.Join(gone, "tickets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gone, "map.md"), []byte("# Gone\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	addRecent(gone)
	addRecent(project + "/")
	addRecent(m)
	addRecent(m + "/")
	if err := os.RemoveAll(gone); err != nil {
		t.Fatal(err)
	}

	got := loadRecents()
	if len(got) != 2 || got[0].Path != m || got[0].Kind != "map" || got[1].Path != project || got[1].Kind != "project" {
		t.Errorf("recents = %+v", got)
	}
}

func TestRecentsAreCappedAtTwenty(t *testing.T) {
	isolateConfig(t)
	root := t.TempDir()
	for i := 0; i < 25; i++ {
		p := filepath.Join(root, string(rune('a'+i)))
		if err := os.Mkdir(p, 0o755); err != nil {
			t.Fatal(err)
		}
		addRecent(p)
	}

	if got := loadRecents(); len(got) != 20 || got[0].Name != "y" {
		t.Errorf("len = %d, first = %+v", len(got), got[0])
	}
}

func TestRecentMapWhoseMapFileVanishedIsDropped(t *testing.T) {
	isolateConfig(t)
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "tickets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "map.md"), []byte("# M\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	addRecent(dir)
	if err := os.Remove(filepath.Join(dir, "map.md")); err != nil {
		t.Fatal(err)
	}

	if got := loadRecents(); len(got) != 0 {
		t.Errorf("recents = %+v, want none", got)
	}
}
