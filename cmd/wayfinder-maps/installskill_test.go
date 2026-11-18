package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/RamsesKools/wayfinder-maps/internal/skillinstall"
)

var testFS = fstest.MapFS{
	"LICENSE":                    {Data: []byte("MIT")},
	"wayfinder-maps/SKILL.md":    {Data: []byte("---\nname: wayfinder-maps\n---\nuse `research`\n")},
	"grill-me/SKILL.md":          {Data: []byte("---\nname: grill-me\n---\n")},
	"research/SKILL.md":          {Data: []byte("---\nname: research\n---\n")},
	"prototype/SKILL.md":         {Data: []byte("---\nname: prototype\n---\n")},
	"domain-modeling/SKILL.md":   {Data: []byte("---\nname: domain-modeling\n---\n")},
	".optional/to-spec/SKILL.md": {Data: []byte("---\nname: to-spec\n---\n")},
}

type run struct {
	home, out, err string
	rc             int
}

func do(t *testing.T, home, stdin string, tty bool, args ...string) run {
	t.Helper()
	var out, errb bytes.Buffer
	d := installDeps{
		FS: testFS, Home: home, Stdin: strings.NewReader(stdin), Interactive: tty,
		Version: "v0.9.0", Commit: "abc123",
		Now: func() time.Time { return time.Date(2026, 11, 17, 9, 0, 0, 0, time.UTC) },
	}
	rc := installSkill(args, d, &out, &errb)
	return run{home, out.String(), errb.String(), rc}
}

func has(p ...string) bool {
	_, err := os.Lstat(filepath.Join(p...))
	return err == nil
}

func TestDirFlagInstallsWithoutPrompt(t *testing.T) {
	home, dir := t.TempDir(), filepath.Join(t.TempDir(), "skills")
	r := do(t, home, "", false, "--dir", dir)
	if r.rc != 0 {
		t.Fatalf("rc %d: %s%s", r.rc, r.out, r.err)
	}
	if !has(dir, "wayfinder-maps", "SKILL.md") || !has(dir, "wayfinder-grill-me", "SKILL.md") || !has(dir, skillinstall.LockName) {
		t.Fatal("skills not installed")
	}
	if has(dir, "wayfinder-to-spec") {
		t.Fatal("optional skill installed without --with-optional")
	}
	lock, _ := skillinstall.ReadLock(dir)
	if lock.Version != "v0.9.0" || lock.Commit != "abc123" {
		t.Fatalf("lock = %+v", lock)
	}
}

func TestWithOptionalAndOnly(t *testing.T) {
	dir := t.TempDir()
	do(t, t.TempDir(), "", false, "--dir", dir, "--with-optional")
	if !has(dir, "wayfinder-to-spec") {
		t.Fatal("--with-optional skipped to-spec")
	}
	dir2 := t.TempDir()
	do(t, t.TempDir(), "", false, "--dir", dir2, "--only", "grill-me,wayfinder-research")
	if !has(dir2, "wayfinder-grill-me") || !has(dir2, "wayfinder-research") || has(dir2, "wayfinder-maps") {
		t.Fatal("--only selection wrong")
	}
}

func TestUnknownOnlyFails(t *testing.T) {
	r := do(t, t.TempDir(), "", false, "--dir", t.TempDir(), "--only", "nope")
	if r.rc != 2 || !strings.Contains(r.err, "nope") {
		t.Fatalf("rc %d: %s", r.rc, r.err)
	}
}

func TestDryRunWritesNothingAndListsChanges(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "skills")
	r := do(t, t.TempDir(), "", false, "--dir", dir, "--dry-run")
	if r.rc != 0 || has(dir) {
		t.Fatalf("rc %d, dir exists %v", r.rc, has(dir))
	}
	if !strings.Contains(r.out, "create") || !strings.Contains(r.out, "wayfinder-grill-me") || !strings.Contains(r.out, "Dry run") {
		t.Fatalf("out:\n%s", r.out)
	}
}

func TestRerunIsANoOp(t *testing.T) {
	dir := t.TempDir()
	do(t, t.TempDir(), "", false, "--dir", dir)
	r := do(t, t.TempDir(), "", false, "--dir", dir)
	if r.rc != 0 || !strings.Contains(r.out, "up to date") {
		t.Fatalf("rc %d:\n%s", r.rc, r.out)
	}
}

func TestConflictStopsNonZeroWithoutPartialChanges(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "wayfinder-research"), 0o755)
	os.WriteFile(filepath.Join(dir, "wayfinder-research", "SKILL.md"), []byte("mine"), 0o644)
	r := do(t, t.TempDir(), "", false, "--dir", dir)
	if r.rc != 2 || !strings.Contains(r.err, "wayfinder-research") || !strings.Contains(r.err, "--force") {
		t.Fatalf("rc %d: %s", r.rc, r.err)
	}
	if has(dir, "wayfinder-maps") || has(dir, skillinstall.LockName) {
		t.Fatal("partial changes")
	}
	if r := do(t, t.TempDir(), "", false, "--dir", dir, "--force"); r.rc != 0 {
		t.Fatalf("force rc %d: %s", r.rc, r.err)
	}
}

func TestConflictInOneDirBlocksEveryDir(t *testing.T) {
	good, bad := t.TempDir(), t.TempDir()
	os.MkdirAll(filepath.Join(bad, "wayfinder-maps"), 0o755)
	r := do(t, t.TempDir(), "", false, "--dir", good, "--dir", bad)
	if r.rc != 2 || has(good, "wayfinder-maps") {
		t.Fatalf("rc %d, good dir written %v", r.rc, has(good, "wayfinder-maps"))
	}
}

func TestDefaultsNeedYesWithoutATerminal(t *testing.T) {
	home := t.TempDir()
	r := do(t, home, "", false)
	if r.rc != 2 || !strings.Contains(r.err, "--yes") || has(home, ".claude") {
		t.Fatalf("rc %d: %s", r.rc, r.err)
	}
}

func TestYesInstallsIntoAllThreeHarnessDirs(t *testing.T) {
	home := t.TempDir()
	r := do(t, home, "", false, "--yes")
	if r.rc != 0 {
		t.Fatalf("rc %d: %s%s", r.rc, r.out, r.err)
	}
	for _, d := range []string{".claude/skills", ".copilot/skills", ".agents/skills"} {
		if !has(home, d, "wayfinder-maps", "SKILL.md") {
			t.Errorf("%s not installed", d)
		}
	}
}

func TestSymlinkedHarnessDirsInstallOnce(t *testing.T) {
	home := t.TempDir()
	os.MkdirAll(filepath.Join(home, ".agents", "skills"), 0o755)
	os.MkdirAll(filepath.Join(home, ".claude"), 0o755)
	os.Symlink(filepath.Join(home, ".agents", "skills"), filepath.Join(home, ".claude", "skills"))

	r := do(t, home, "", false, "--yes")
	if r.rc != 0 {
		t.Fatalf("rc %d: %s%s", r.rc, r.out, r.err)
	}
	if !strings.Contains(r.out, "Claude Code, Codex") {
		t.Fatalf("shared dir not merged:\n%s", r.out)
	}
}

func TestInteractiveAsksPerDirAndHonoursNo(t *testing.T) {
	home := t.TempDir()
	r := do(t, home, "y\nn\n\n", true)
	if r.rc != 0 {
		t.Fatalf("rc %d: %s%s", r.rc, r.out, r.err)
	}
	if !has(home, ".claude/skills/wayfinder-maps") || has(home, ".copilot") || !has(home, ".agents/skills/wayfinder-maps") {
		t.Fatalf("answers y,n,<enter> not honoured:\n%s", r.out)
	}
}

func TestInteractiveDeclineAllWritesNothing(t *testing.T) {
	home := t.TempDir()
	r := do(t, home, "n\nn\nn\n", true)
	if r.rc != 0 || has(home, ".claude") || !strings.Contains(r.out, "Nothing selected") {
		t.Fatalf("rc %d:\n%s", r.rc, r.out)
	}
}

func TestDryRunDefaultsNeedNeitherPromptNorYes(t *testing.T) {
	home := t.TempDir()
	r := do(t, home, "", false, "--dry-run")
	if r.rc != 0 || has(home, ".claude") || !strings.Contains(r.out, ".copilot/skills") {
		t.Fatalf("rc %d:\n%s%s", r.rc, r.out, r.err)
	}
}

func TestNewerBinaryUpdatesCleanSkillsAndKeepsLockOwnership(t *testing.T) {
	dir := t.TempDir()
	do(t, t.TempDir(), "", false, "--dir", dir)

	newer := fstest.MapFS{}
	for k, v := range testFS {
		newer[k] = v
	}
	newer["research/SKILL.md"] = &fstest.MapFile{Data: []byte("---\nname: research\n---\nnew upstream\n")}

	var out, errb bytes.Buffer
	d := installDeps{FS: newer, Home: t.TempDir(), Stdin: strings.NewReader(""), Version: "v0.9.1", Now: time.Now}
	if rc := installSkill([]string{"--dir", dir}, d, &out, &errb); rc != 0 {
		t.Fatalf("rc %d: %s", rc, errb.String())
	}
	if !strings.Contains(out.String(), "update    wayfinder-research") || !strings.Contains(out.String(), "unchanged wayfinder-maps") {
		t.Fatalf("out:\n%s", out.String())
	}
	b, _ := os.ReadFile(filepath.Join(dir, "wayfinder-research", "SKILL.md"))
	if !strings.Contains(string(b), "new upstream") {
		t.Fatal("skill not updated")
	}
	if lock, _ := skillinstall.ReadLock(dir); lock.Version != "v0.9.1" {
		t.Fatalf("lock version %s", lock.Version)
	}
}
