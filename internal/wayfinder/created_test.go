package wayfinder

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestCreatedTimesPrefersFrontmatterThenGitThenBirthTime(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	tickets := filepath.Join(dir, "tickets")
	if err := os.MkdirAll(tickets, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(tickets, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(filepath.Join(dir, "map.md"), []byte("# M\n"), 0o644)
	git := func(env []string, args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false"}, args...)...)
		cmd.Env = append(os.Environ(), env...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	at := func(s string) []string { return []string{"GIT_COMMITTER_DATE=" + s, "GIT_AUTHOR_DATE=" + s} }

	git(nil, "init", "-q")
	write("01-a.md", "---\ntype: grilling\n---\n# A\n")
	write("02-b.md", "---\ntype: grilling\ncreated: 2026-03-04T05:06:07Z\n---\n# B\n")
	git(at("2026-01-01T10:00:00Z"), "add", ".")
	git(at("2026-01-01T10:00:00Z"), "commit", "-qm", "first")
	write("01-a.md", "---\ntype: grilling\n---\n# A\n\n## Answer\n\nYes.\n")
	git(at("2026-02-01T10:00:00Z"), "commit", "-qam", "edit")
	write("03-c.md", "---\ntype: grilling\n---\n# C\n")

	e, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := e.CreatedTimes()
	if want := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC); !got[1].Equal(want) {
		t.Errorf("01 = %v, want the commit that added it, %v", got[1], want)
	}
	if want := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC); !got[2].Equal(want) {
		t.Errorf("02 = %v, want its created key, %v", got[2], want)
	}
	if c := got[3]; c.IsZero() || time.Since(c) > time.Hour {
		t.Errorf("03 = %v, want the untracked file's birth time (just now)", c)
	}
}

func TestCreatedTimesOutsideGit(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "tickets"), 0o755)
	os.WriteFile(filepath.Join(dir, "map.md"), []byte("# M\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "tickets", "01-a.md"), []byte("---\ntype: grilling\n---\n# A\n"), 0o644)
	e, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if c := e.CreatedTimes()[1]; c.IsZero() || time.Since(c) > time.Hour {
		t.Errorf("01 = %v, want its birth time", c)
	}
}
