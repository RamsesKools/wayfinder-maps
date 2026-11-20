package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const demoDir = "../../internal/wayfinder/testdata/implement-demo"

func runExport(t *testing.T, args ...string) (code int, stderr string) {
	t.Helper()
	var out, errw bytes.Buffer
	code = exportCmd(args, &out, &errw)
	return code, errw.String()
}

func TestExportWritesStaticSite(t *testing.T) {
	out := filepath.Join(t.TempDir(), "site")
	if code, stderr := runExport(t, demoDir, out); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}

	for _, f := range []string{"index.html", "style.css", "favicon.svg", "js/main.js", "js/static.js", "graph.json"} {
		if _, err := os.Stat(filepath.Join(out, f)); err != nil {
			t.Errorf("missing %s: %v", f, err)
		}
	}

	html, err := os.ReadFile(filepath.Join(out, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(html), `<meta name="wayfinder-static"`) {
		t.Error("index.html does not flag static mode")
	}

	raw, err := os.ReadFile(filepath.Join(out, "graph.json"))
	if err != nil {
		t.Fatal(err)
	}
	var g graphDoc
	if err := json.Unmarshal(raw, &g); err != nil {
		t.Fatal(err)
	}
	want := buildGraph(loadImplementDemo(t))
	if g.Name != want.Name || len(g.Nodes) != 20 || len(g.Edges) != len(want.Edges) || len(g.Fog) != 3 || g.Counts != want.Counts {
		t.Errorf("graph.json = %s %d nodes %d edges %d fog %+v", g.Name, len(g.Nodes), len(g.Edges), len(g.Fog), g.Counts)
	}
	if g.Nodes[6].Body == "" {
		t.Error("ticket bodies are missing from graph.json")
	}
}

func TestExportKeepsTheSourceIndexUnflagged(t *testing.T) {
	if strings.Contains(string(mustRead(t, "web/index.html")), "wayfinder-static") {
		t.Error("the served index.html must stay unflagged: server mode is unchanged")
	}
}

func TestExportArgs(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		msg  string
	}{
		{"no args", nil, "usage:"},
		{"one arg", []string{demoDir}, "usage:"},
		{"too many", []string{demoDir, "a", "b"}, "usage:"},
		{"unknown flag", []string{"--nope", demoDir, "a"}, "unknown option --nope"},
		{"not a map", []string{t.TempDir(), filepath.Join(t.TempDir(), "o")}, "wayfinder-maps:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if code, stderr := runExport(t, tc.args...); code != 2 || !strings.Contains(stderr, tc.msg) {
				t.Errorf("code %d, stderr %q; want 2 containing %q", code, stderr, tc.msg)
			}
		})
	}
}

func TestExportRefusesToWriteIntoTheMap(t *testing.T) {
	if code, stderr := runExport(t, demoDir, filepath.Join(demoDir, "tickets")); code != 2 || !strings.Contains(stderr, "inside the map") {
		t.Errorf("code %d, stderr %q", code, stderr)
	}
}

func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
