package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/RamsesKools/wayfinder-maps/internal/selfupdate"
)

func TestRunSelfUpdateExitCodes(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "wayfinder-maps")
	os.WriteFile(exe, []byte("x"), 0o755)
	u := &selfupdate.Updater{BaseURL: "http://127.0.0.1:1", GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Exe: exe}

	var out, errb bytes.Buffer
	u.Current = "dev"
	if code := runSelfUpdate(u, "", false, false, &out, &errb); code != 2 || !strings.Contains(errb.String(), "refused") {
		t.Fatalf("dev: code %d, stderr %q", code, errb.String())
	}

	errb.Reset()
	u.Current = "0.5.1"
	if code := runSelfUpdate(u, "", true, false, &out, &errb); code != 2 || !strings.Contains(errb.String(), ".install.json") {
		t.Fatalf("no receipt: code %d, stderr %q", code, errb.String())
	}

	os.WriteFile(filepath.Join(dir, selfupdate.ReceiptName), []byte(`{"version":"v0.5.1"}`), 0o644)
	out.Reset()
	if code := runSelfUpdate(u, "v0.6.0", true, false, &out, &errb); code != 1 || !strings.Contains(out.String(), "behind") {
		t.Fatalf("behind: code %d, stdout %q", code, out.String())
	}
	out.Reset()
	if code := runSelfUpdate(u, "v0.5.1", true, false, &out, &errb); code != 0 || !strings.Contains(out.String(), "up to date") {
		t.Fatalf("up to date: code %d, stdout %q", code, out.String())
	}
}
