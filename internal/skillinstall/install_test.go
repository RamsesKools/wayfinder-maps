package skillinstall

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func skill(name string, files map[string]string) Skill {
	s := Skill{Name: name, InstallName: name, Files: map[string][]byte{}}
	for p, c := range files {
		s.Files[p] = []byte(c)
	}
	return s
}

func meta(v string) Meta {
	return Meta{Version: v, Commit: "abc", Now: time.Date(2026, 11, 17, 9, 0, 0, 0, time.UTC)}
}

func install(t *testing.T, dir string, ss []Skill, m Meta, force bool) *Plan {
	t.Helper()
	p, err := NewPlan(dir, ss, force)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Apply(m); err != nil {
		t.Fatal(err)
	}
	return p
}

func read(t *testing.T, p ...string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(p...))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func exists(p ...string) bool {
	_, err := os.Lstat(filepath.Join(p...))
	return err == nil
}

func write(t *testing.T, content string, p ...string) {
	t.Helper()
	path := filepath.Join(p...)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

var two = []Skill{
	skill("wayfinder-maps", map[string]string{"SKILL.md": "maps v1", "sub/NOTE.md": "note"}),
	skill("wayfinder-research", map[string]string{"SKILL.md": "research v1"}),
}

func TestFreshInstallWritesFilesAndLock(t *testing.T) {
	dir := t.TempDir()
	p := install(t, dir, two, meta("v0.9.0"), false)

	if got := read(t, dir, "wayfinder-maps", "sub", "NOTE.md"); got != "note" {
		t.Fatalf("nested file = %q", got)
	}
	lock, err := ReadLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	if lock.Version != "v0.9.0" || lock.Commit != "abc" || lock.InstalledAt != "2026-11-17T09:00:00Z" {
		t.Fatalf("lock header = %+v", lock)
	}
	if h := lock.Skills["wayfinder-maps"].Files["SKILL.md"]; h != hash([]byte("maps v1")) || h[:7] != "sha256:" {
		t.Fatalf("hash = %q", h)
	}
	if !p.Changed() {
		t.Fatal("fresh install should report changes")
	}
}

func TestRerunAtSameContentIsANoOp(t *testing.T) {
	dir := t.TempDir()
	install(t, dir, two, meta("v0.9.0"), false)
	before := read(t, dir, LockName)

	later := meta("v0.9.0")
	later.Now = later.Now.Add(time.Hour)
	p := install(t, dir, two, later, false)

	if p.Changed() {
		t.Fatal("rerun should not change anything")
	}
	if read(t, dir, LockName) != before {
		t.Fatal("lock was rewritten on a no-op")
	}
}

func TestOtherSkillsAreLeftAlone(t *testing.T) {
	dir := t.TempDir()
	write(t, "mine", dir, "grill-me", "SKILL.md")
	write(t, "stray", dir, "notes.txt")
	install(t, dir, two, meta("v0.9.0"), false)
	if read(t, dir, "grill-me", "SKILL.md") != "mine" || read(t, dir, "notes.txt") != "stray" {
		t.Fatal("foreign files touched")
	}
}

func TestExistingUnlockedDirConflictsAndNothingIsWritten(t *testing.T) {
	dir := t.TempDir()
	write(t, "mine", dir, "wayfinder-research", "SKILL.md")

	p, err := NewPlan(dir, two, false)
	if err != nil {
		t.Fatal(err)
	}
	if c := p.Conflicts(); len(c) != 1 || c[0].Skill != "wayfinder-research" {
		t.Fatalf("conflicts = %+v", c)
	}
	if err := p.Apply(meta("v0.9.0")); err == nil {
		t.Fatal("Apply must refuse a plan with conflicts")
	}
	if exists(dir, "wayfinder-maps") || exists(dir, LockName) {
		t.Fatal("partial changes written")
	}
	if read(t, dir, "wayfinder-research", "SKILL.md") != "mine" {
		t.Fatal("conflicting dir was modified")
	}
}

func TestForceReplacesOnlyTheConflictingWayfinderDir(t *testing.T) {
	dir := t.TempDir()
	write(t, "mine", dir, "wayfinder-research", "SKILL.md")
	write(t, "x", dir, "wayfinder-research", "extra.md")
	write(t, "other", dir, "unrelated", "SKILL.md")

	install(t, dir, two, meta("v0.9.0"), true)

	if read(t, dir, "wayfinder-research", "SKILL.md") != "research v1" || exists(dir, "wayfinder-research", "extra.md") {
		t.Fatal("dir not replaced cleanly")
	}
	if read(t, dir, "unrelated", "SKILL.md") != "other" {
		t.Fatal("unrelated skill touched")
	}
}

func TestLocalEditIsDriftAndConflicts(t *testing.T) {
	dir := t.TempDir()
	install(t, dir, two, meta("v0.9.0"), false)
	write(t, "edited", dir, "wayfinder-maps", "SKILL.md")

	next := []Skill{skill("wayfinder-maps", map[string]string{"SKILL.md": "maps v2", "sub/NOTE.md": "note"}), two[1]}
	p, _ := NewPlan(dir, next, false)
	if c := p.Conflicts(); len(c) != 1 || c[0].Skill != "wayfinder-maps" {
		t.Fatalf("conflicts = %+v", c)
	}
}

func TestExtraFileInManagedDirIsDriftAndConflicts(t *testing.T) {
	dir := t.TempDir()
	install(t, dir, two, meta("v0.9.0"), false)
	write(t, "mine", dir, "wayfinder-maps", "my-notes.md")

	p, _ := NewPlan(dir, two, false)
	if len(p.Conflicts()) != 1 {
		t.Fatalf("conflicts = %+v", p.Conflicts())
	}
}

func TestDeletedFileInManagedDirIsDrift(t *testing.T) {
	dir := t.TempDir()
	install(t, dir, two, meta("v0.9.0"), false)
	os.Remove(filepath.Join(dir, "wayfinder-maps", "sub", "NOTE.md"))

	p, _ := NewPlan(dir, two, false)
	if len(p.Conflicts()) != 1 {
		t.Fatalf("conflicts = %+v", p.Conflicts())
	}
}

func TestCleanUpgradeReplacesDirAndDropsRemovedFiles(t *testing.T) {
	dir := t.TempDir()
	install(t, dir, two, meta("v0.9.0"), false)

	next := []Skill{skill("wayfinder-maps", map[string]string{"SKILL.md": "maps v2"}), two[1]}
	p := install(t, dir, next, meta("v0.10.0"), false)

	if read(t, dir, "wayfinder-maps", "SKILL.md") != "maps v2" || exists(dir, "wayfinder-maps", "sub") {
		t.Fatal("upgrade did not replace the dir")
	}
	if lock, _ := ReadLock(dir); lock.Version != "v0.10.0" {
		t.Fatalf("lock version = %s", lock.Version)
	}
	actions := map[string]Action{}
	for _, c := range p.Changes {
		actions[c.Skill] = c.Action
	}
	if actions["wayfinder-maps"] != Update || actions["wayfinder-research"] != Unchanged {
		t.Fatalf("actions = %v", actions)
	}
}

func TestPartialInstallKeepsPreviousLockEntries(t *testing.T) {
	dir := t.TempDir()
	install(t, dir, two, meta("v0.9.0"), false)
	install(t, dir, two[:1], meta("v0.9.0"), false)

	lock, _ := ReadLock(dir)
	if _, ok := lock.Skills["wayfinder-research"]; !ok || len(lock.Skills) != 2 {
		t.Fatalf("lock skills = %v", lock.Skills)
	}
}

func TestPlanHasNoSideEffects(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "not", "yet")
	if _, err := NewPlan(dir, two, false); err != nil {
		t.Fatal(err)
	}
	if exists(dir) {
		t.Fatal("NewPlan created the target dir")
	}
}

func TestCorruptLockIsAnError(t *testing.T) {
	dir := t.TempDir()
	write(t, "{nope", dir, LockName)
	if _, err := NewPlan(dir, two, false); err == nil {
		t.Fatal("expected an error")
	}
}

func TestNoStagingLeftovers(t *testing.T) {
	dir := t.TempDir()
	install(t, dir, two, meta("v0.9.0"), false)
	install(t, dir, []Skill{skill("wayfinder-maps", map[string]string{"SKILL.md": "v2"}), two[1]}, meta("v0.10.0"), false)
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != LockName && e.Name() != "wayfinder-maps" && e.Name() != "wayfinder-research" {
			t.Fatalf("leftover %s", e.Name())
		}
	}
}
