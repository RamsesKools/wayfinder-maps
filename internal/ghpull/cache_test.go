package ghpull

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func writeMirror(t *testing.T, dir, repo string, num int) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	src := `{"repo":"` + repo + `","map":` + strconv.Itoa(num) + `}`
	if err := os.WriteFile(filepath.Join(dir, ".source.json"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "map.md"), []byte("# M\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestMirrorDirLivesUnderTheUserCacheDir(t *testing.T) {
	root := t.TempDir()

	got := MirrorDir(root, "Acme/Widgets", "Live Map!", 2)

	if want := filepath.Join(root, "wayfinder-maps", "github", "Acme-Widgets", "live-map"); got != want {
		t.Errorf("MirrorDir = %q, want %q", got, want)
	}
}

func TestMirrorDirFallsBackToTheIssueNumberForAnUnsluggableTitle(t *testing.T) {
	got := MirrorDir(t.TempDir(), "acme/widgets", "???", 7)

	if filepath.Base(got) != "map-7" {
		t.Errorf("MirrorDir = %q", got)
	}
}

func TestMirrorDirKeepsTwoMapsWithOneTitleApart(t *testing.T) {
	root := t.TempDir()
	first := MirrorDir(root, "acme/widgets", "Same", 2)
	writeMirror(t, first, "acme/widgets", 2)

	if again := MirrorDir(root, "acme/widgets", "Same", 2); again != first {
		t.Errorf("same map moved: %q vs %q", again, first)
	}
	other := MirrorDir(root, "acme/widgets", "Same", 9)
	if other == first || filepath.Base(other) != "same-9" {
		t.Errorf("a different map with the same title got %q (first %q)", other, first)
	}
}

func TestRemoveMirrorDeletesTheMirrorAndItsEmptyRepoDir(t *testing.T) {
	root := t.TempDir()
	dir := MirrorDir(root, "acme/widgets", "Live Map", 2)
	writeMirror(t, dir, "acme/widgets", 2)

	if !IsMirror(root, dir) {
		t.Fatal("IsMirror = false for a cached mirror")
	}
	if err := RemoveMirror(root, dir); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("mirror still there: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(dir)); !os.IsNotExist(err) {
		t.Errorf("empty repo dir still there: %v", err)
	}
}

func TestRemoveMirrorKeepsSiblingMaps(t *testing.T) {
	root := t.TempDir()
	a, b := MirrorDir(root, "acme/widgets", "A", 1), MirrorDir(root, "acme/widgets", "B", 2)
	writeMirror(t, a, "acme/widgets", 1)
	writeMirror(t, b, "acme/widgets", 2)

	if err := RemoveMirror(root, a); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(b); err != nil {
		t.Errorf("sibling gone: %v", err)
	}
}

func TestRemoveMirrorRefusesAnythingThatIsNotACachedMirror(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	writeMirror(t, outside, "acme/widgets", 1) // has .source.json, but not in the cache
	noSource := filepath.Join(root, "wayfinder-maps", "github", "acme-widgets", "x")
	if err := os.MkdirAll(noSource, 0o755); err != nil {
		t.Fatal(err)
	}
	cacheTop := filepath.Join(root, "wayfinder-maps", "github")
	repoDir := filepath.Join(cacheTop, "acme-widgets")

	for _, p := range []string{outside, noSource, cacheTop, repoDir, root, "", filepath.Join(repoDir, "x", "..", "..", "..", "..")} {
		if IsMirror(root, p) {
			t.Errorf("IsMirror(%q) = true", p)
		}
		if err := RemoveMirror(root, p); err == nil {
			t.Errorf("RemoveMirror(%q) succeeded", p)
		}
	}
	if _, err := os.Stat(outside); err != nil {
		t.Errorf("outside dir touched: %v", err)
	}
}

func TestFindMirrorLocatesAMirrorByMapNumberEvenAfterTheTitleChanged(t *testing.T) {
	root := t.TempDir()
	old := MirrorDir(root, "acme/widgets", "Old title", 2)
	writeMirror(t, old, "acme/widgets", 2)
	writeMirror(t, MirrorDir(root, "acme/widgets", "Other", 3), "acme/widgets", 3)

	if got, ok := FindMirror(root, "acme/widgets", 2); !ok || got != old {
		t.Errorf("FindMirror = %q, %v; want %q", got, ok, old)
	}
	if _, ok := FindMirror(root, "acme/widgets", 9); ok {
		t.Error("map 9 was never pulled")
	}
	if _, ok := FindMirror(root, "acme/gadgets", 2); ok {
		t.Error("map 2 of another repo was never pulled")
	}
}
