package selfupdate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

type fakeRelease struct {
	latest string
	// files per tag, name -> content
	files map[string]map[string][]byte
	// tamper serves a different archive than the one checksums.txt describes.
	tamper bool
}

func tarball(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, data := range files {
		mode := int64(0o644)
		if name == "wayfinder-maps" {
			mode = 0o755
		}
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: mode, Size: int64(len(data)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		tw.Write(data)
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func newFake(t *testing.T, latest string, tags ...string) (*fakeRelease, *httptest.Server) {
	t.Helper()
	f := &fakeRelease{latest: latest, files: map[string]map[string][]byte{}}
	for _, tag := range tags {
		f.addTag(t, tag, map[string][]byte{
			"wayfinder-maps": []byte("binary " + tag),
			"wayfinder-app":  []byte("launcher " + tag),
			"wayfinder-web":  []byte("launcher " + tag),
		})
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/releases/tag/"+f.latest, http.StatusFound)
	})
	mux.HandleFunc("/releases/download/", func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/releases/download/"), "/")
		data, ok := f.files[parts[0]][parts[len(parts)-1]]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if f.tamper && strings.HasSuffix(parts[1], ".tar.gz") {
			data = tarball(t, map[string][]byte{"wayfinder-maps": []byte("evil")})
		}
		w.Write(data)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return f, srv
}

func (f *fakeRelease) addTag(t *testing.T, tag string, contents map[string][]byte) {
	asset := assetName(tag, runtime.GOOS, runtime.GOARCH)
	archive := tarball(t, contents)
	sum := sha256.Sum256(archive)
	f.files[tag] = map[string][]byte{
		asset:           archive,
		"checksums.txt": []byte(fmt.Sprintf("%s  %s\n%s  other_asset.zip\n", hex.EncodeToString(sum[:]), asset, strings.Repeat("0", 64))),
	}
}

// install lays out a fake install dir with a binary and a receipt, like install.sh does.
func install(t *testing.T, version string, receipt bool) string {
	t.Helper()
	dir := t.TempDir()
	exe := filepath.Join(dir, "wayfinder-maps")
	if err := os.WriteFile(exe, []byte("binary v"+version), 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "wayfinder-app"), []byte("launcher v"+version), 0o755)
	if receipt {
		data, _ := json.Marshal(Receipt{Version: "v" + version, InstalledAt: "2026-01-01T00:00:00Z", Asset: "old.tar.gz", SHA256: "x"})
		os.WriteFile(filepath.Join(dir, ReceiptName), data, 0o644)
	}
	return exe
}

func updater(srv *httptest.Server, exe, current string) *Updater {
	return &Updater{
		BaseURL: srv.URL, GOOS: runtime.GOOS, GOARCH: runtime.GOARCH,
		Current: current, Exe: exe,
		Now: func() time.Time { return time.Date(2026, 11, 17, 9, 0, 0, 0, time.UTC) },
	}
}

func read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestCheckBehindAndUpToDate(t *testing.T) {
	_, srv := newFake(t, "v0.6.0", "v0.6.0")
	exe := install(t, "0.5.1", true)

	st, err := updater(srv, exe, "0.5.1").Check(context.Background(), "")
	if err != nil || !st.Behind || st.Target != "v0.6.0" {
		t.Fatalf("behind: %+v %v", st, err)
	}
	st, err = updater(srv, exe, "0.6.0").Check(context.Background(), "")
	if err != nil || st.Behind {
		t.Fatalf("up to date: %+v %v", st, err)
	}
	if got := read(t, exe); got != "binary v0.5.1" {
		t.Fatalf("check modified the binary: %q", got)
	}
}

func TestUpdateLatestRewritesReceipt(t *testing.T) {
	_, srv := newFake(t, "v0.6.0", "v0.6.0")
	exe := install(t, "0.5.1", true)
	dir := filepath.Dir(exe)

	st, err := updater(srv, exe, "0.5.1").Update(context.Background(), "", false)
	if err != nil || !st.Updated {
		t.Fatalf("update: %+v %v", st, err)
	}
	if got := read(t, exe); got != "binary v0.6.0" {
		t.Fatalf("binary = %q", got)
	}
	if got := read(t, filepath.Join(dir, "wayfinder-web")); got != "launcher v0.6.0" {
		t.Fatalf("new launcher missing: %q", got)
	}
	fi, _ := os.Stat(exe)
	if fi.Mode().Perm()&0o111 == 0 {
		t.Fatalf("binary lost its exec bit: %v", fi.Mode())
	}
	var r Receipt
	if err := json.Unmarshal([]byte(read(t, filepath.Join(dir, ReceiptName))), &r); err != nil {
		t.Fatal(err)
	}
	asset := assetName("v0.6.0", runtime.GOOS, runtime.GOARCH)
	if r.Version != "v0.6.0" || r.Asset != asset || r.InstalledAt != "2026-11-17T09:00:00Z" || len(r.SHA256) != 64 {
		t.Fatalf("receipt = %+v", r)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".update-") {
			t.Fatalf("staging dir left behind: %s", e.Name())
		}
	}
}

func TestPinnedVersionAndDowngrade(t *testing.T) {
	_, srv := newFake(t, "v0.6.0", "v0.5.0", "v0.6.0")
	exe := install(t, "0.6.0", true)

	st, err := updater(srv, exe, "0.6.0").Update(context.Background(), "v0.5.0", false)
	if err != nil || !st.Updated || st.Target != "v0.5.0" {
		t.Fatalf("pin: %+v %v", st, err)
	}
	if got := read(t, exe); got != "binary v0.5.0" {
		t.Fatalf("binary = %q", got)
	}
	// A bare number is accepted too.
	if _, err := updater(srv, exe, "0.5.0").Update(context.Background(), "0.6.0", false); err != nil {
		t.Fatal(err)
	}
	if got := read(t, exe); got != "binary v0.6.0" {
		t.Fatalf("binary = %q", got)
	}
	if _, err := updater(srv, exe, "0.6.0").Update(context.Background(), "latest", false); err == nil {
		t.Fatal("expected invalid version error")
	}
}

func TestAlreadyCurrentIsNoopUnlessForced(t *testing.T) {
	_, srv := newFake(t, "v0.6.0", "v0.6.0")
	exe := install(t, "0.6.0", true)

	st, err := updater(srv, exe, "0.6.0").Update(context.Background(), "", false)
	if err != nil || !st.NoChange || st.Updated {
		t.Fatalf("noop: %+v %v", st, err)
	}
	if got := read(t, exe); got != "binary v0.6.0" {
		t.Fatalf("noop rewrote binary")
	}
	st, err = updater(srv, exe, "0.6.0").Update(context.Background(), "", true)
	if err != nil || !st.Updated {
		t.Fatalf("force: %+v %v", st, err)
	}
}

func TestRefusals(t *testing.T) {
	_, srv := newFake(t, "v0.6.0", "v0.6.0")
	cases := []struct {
		name    string
		current string
		receipt bool
		goos    string
		want    string
	}{
		{"dev build", "dev", true, runtime.GOOS, "dev build"},
		{"go install or manual download", "0.5.1", false, runtime.GOOS, "install.sh"},
		{"windows", "0.5.1", true, "windows", "windows"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			exe := install(t, "0.5.1", c.receipt)
			u := updater(srv, exe, c.current)
			u.GOOS = c.goos
			for _, run := range []func() error{
				func() error { _, err := u.Check(context.Background(), ""); return err },
				func() error { _, err := u.Update(context.Background(), "", false); return err },
			} {
				err := run()
				var ref *RefusalError
				if !errors.As(err, &ref) || !strings.Contains(err.Error(), c.want) {
					t.Fatalf("err = %v, want refusal mentioning %q", err, c.want)
				}
			}
			if got := read(t, exe); got != "binary v0.5.1" {
				t.Fatalf("refusal modified the binary: %q", got)
			}
		})
	}
}

func TestChecksumMismatchLeavesInstallUntouched(t *testing.T) {
	f, srv := newFake(t, "v0.6.0", "v0.6.0")
	f.tamper = true
	exe := install(t, "0.5.1", true)
	before := read(t, filepath.Join(filepath.Dir(exe), ReceiptName))

	_, err := updater(srv, exe, "0.5.1").Update(context.Background(), "", false)
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("err = %v", err)
	}
	if read(t, exe) != "binary v0.5.1" || read(t, filepath.Join(filepath.Dir(exe), ReceiptName)) != before {
		t.Fatal("install changed after a failed verification")
	}
	entries, _ := os.ReadDir(filepath.Dir(exe))
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".update-") {
			t.Fatalf("staging dir left behind: %s", e.Name())
		}
	}
}

func TestMissingReleaseAndMissingAsset(t *testing.T) {
	_, srv := newFake(t, "v0.6.0", "v0.6.0")
	exe := install(t, "0.5.1", true)
	if _, err := updater(srv, exe, "0.5.1").Update(context.Background(), "v9.9.9", false); err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("err = %v", err)
	}
	u := updater(srv, exe, "0.5.1")
	u.GOARCH = "riscv64"
	if _, err := u.Update(context.Background(), "", false); err == nil || !strings.Contains(err.Error(), "no entry") {
		t.Fatalf("err = %v", err)
	}
}

func TestRejectsUnsafeArchive(t *testing.T) {
	f, srv := newFake(t, "v0.6.0")
	f.addTag(t, "v0.6.0", map[string][]byte{"wayfinder-maps": []byte("x"), "../escape": []byte("y")})
	exe := install(t, "0.5.1", true)
	_, err := updater(srv, exe, "0.5.1").Update(context.Background(), "", false)
	if err == nil || !strings.Contains(err.Error(), "unexpected path") {
		t.Fatalf("err = %v", err)
	}
	if _, serr := os.Stat(filepath.Join(filepath.Dir(filepath.Dir(exe)), "escape")); serr == nil {
		t.Fatal("archive escaped the install dir")
	}
}

func TestLatestWithoutRedirectFails(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	exe := install(t, "0.5.1", true)
	if _, err := updater(srv, exe, "0.5.1").Check(context.Background(), ""); err == nil || !strings.Contains(err.Error(), "without a redirect") {
		t.Fatalf("err = %v", err)
	}
}

func TestCompare(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want int
	}{
		{"v0.5.1", "v0.5.1", 0}, {"v0.5.1", "v0.6.0", -1}, {"v0.10.0", "v0.9.9", 1}, {"v1.0.0", "v0.99.99", 1}, {"vdev", "v1.0.0", 0},
	} {
		if got := compare(c.a, c.b); got != c.want {
			t.Errorf("compare(%s,%s) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

// Replacing by rename must work while another process executes the old binary,
// and leave that process untouched (writing in place fails with ETXTBSY on Linux).
func TestReplaceWhileOldBinaryRuns(t *testing.T) {
	if os.Getenv("SELFUPDATE_SLEEPER") != "" {
		time.Sleep(time.Minute)
		return
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	old, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	exe := filepath.Join(dir, "wayfinder-maps")
	if err := os.WriteFile(exe, old, 0o755); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(Receipt{Version: "v0.5.1"})
	os.WriteFile(filepath.Join(dir, ReceiptName), data, 0o644)

	cmd := exec.Command(exe, "-test.run=^TestReplaceWhileOldBinaryRuns$")
	cmd.Env = append(os.Environ(), "SELFUPDATE_SLEEPER=1")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { cmd.Process.Kill(); cmd.Wait() }()
	time.Sleep(200 * time.Millisecond)

	f, srv := newFake(t, "v0.6.0")
	f.addTag(t, "v0.6.0", map[string][]byte{"wayfinder-maps": []byte("new binary")})
	if _, err := updater(srv, exe, "0.5.1").Update(context.Background(), "", false); err != nil {
		t.Fatalf("update while running: %v", err)
	}
	if got := read(t, exe); got != "new binary" {
		t.Fatalf("binary not replaced: %q", got)
	}
	if err := cmd.Process.Signal(syscall.Signal(0)); err != nil {
		t.Fatalf("old process died: %v", err)
	}
}
