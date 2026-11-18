package skillinstall

import (
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/RamsesKools/wayfinder-maps/skills"
)

func realCatalog(t *testing.T) []Skill {
	t.Helper()
	cat, err := Catalog(skills.FS)
	if err != nil {
		t.Fatal(err)
	}
	return cat
}

func find(t *testing.T, cat []Skill, name string) Skill {
	t.Helper()
	for _, s := range cat {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("no skill %q", name)
	return Skill{}
}

func TestCatalogPrefixesEverythingButWayfinderMaps(t *testing.T) {
	cat := realCatalog(t)
	want := map[string]string{
		"wayfinder-maps":  "wayfinder-maps",
		"grill-me":        "wayfinder-grill-me",
		"research":        "wayfinder-research",
		"prototype":       "wayfinder-prototype",
		"domain-modeling": "wayfinder-domain-modeling",
		"to-tickets":      "wayfinder-to-tickets",
	}
	for name, install := range want {
		if got := find(t, cat, name).InstallName; got != install {
			t.Errorf("%s installs as %q, want %q", name, got, install)
		}
	}
}

func TestCatalogMarksOptionalSkills(t *testing.T) {
	cat := realCatalog(t)
	if find(t, cat, "to-tickets").Optional != true || find(t, cat, "research").Optional {
		t.Fatal("optional flag wrong")
	}
}

func TestCatalogRewritesFrontmatterName(t *testing.T) {
	s := find(t, realCatalog(t), "grill-me")
	if !strings.HasPrefix(string(s.Files["SKILL.md"]), "---\nname: wayfinder-grill-me\n") {
		t.Fatalf("frontmatter not rewritten:\n%.80s", s.Files["SKILL.md"])
	}
}

func TestCatalogRewritesSkillReferences(t *testing.T) {
	body := string(find(t, realCatalog(t), "wayfinder-maps").Files["SKILL.md"])
	for _, want := range []string{"`wayfinder-research` skill", "`wayfinder-prototype` skill", "`wayfinder-domain-modeling`"} {
		if !strings.Contains(body, want) {
			t.Errorf("SKILL.md lacks %s", want)
		}
	}
	for _, stale := range []string{"`research` skill", "`prototype` skill", "`domain-modeling`"} {
		if strings.Contains(body, stale) {
			t.Errorf("SKILL.md still has %s", stale)
		}
	}
}

func TestCatalogLeavesNonSkillBackticksAlone(t *testing.T) {
	ui := string(find(t, realCatalog(t), "prototype").Files["UI.md"])
	if strings.Contains(ui, "`wayfinder-prototype` in the path") {
		t.Fatal("UI.md route hint was treated as a skill reference")
	}
}

func TestCatalogShipsLicenseInEverySkill(t *testing.T) {
	for _, s := range realCatalog(t) {
		if len(s.Files["LICENSE"]) == 0 {
			t.Errorf("%s has no LICENSE", s.Name)
		}
	}
}

func TestCatalogIgnoresNonSkillEntries(t *testing.T) {
	fsys := fstest.MapFS{
		"README.md":   {Data: []byte("x")},
		"a/SKILL.md":  {Data: []byte("---\nname: a\n---\n")},
		"empty/x.txt": {Data: []byte("x")},
		"embed.go":    {Data: []byte("x")},
	}
	cat, err := Catalog(fsys)
	if err != nil {
		t.Fatal(err)
	}
	if len(cat) != 1 || cat[0].Name != "a" {
		t.Fatalf("got %+v", cat)
	}
}

func TestSelectDefaultsToTheFiveCoreSkills(t *testing.T) {
	got, err := Select(realCatalog(t), nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if names := installNames(got); strings.Join(names, ",") != "wayfinder-domain-modeling,wayfinder-grill-me,wayfinder-maps,wayfinder-prototype,wayfinder-research" {
		t.Fatalf("got %v", names)
	}
}

func TestSelectWithOptionalAddsTheRest(t *testing.T) {
	cat := realCatalog(t)
	got, err := Select(cat, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(cat) {
		t.Fatalf("got %d of %d", len(got), len(cat))
	}
}

func TestSelectOnlyAcceptsUpstreamOrInstalledNames(t *testing.T) {
	got, err := Select(realCatalog(t), []string{"grill-me", "wayfinder-research", "to-tickets"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if names := installNames(got); strings.Join(names, ",") != "wayfinder-grill-me,wayfinder-research,wayfinder-to-tickets" {
		t.Fatalf("got %v", names)
	}
}

func TestSelectOnlyRejectsUnknownNames(t *testing.T) {
	if _, err := Select(realCatalog(t), []string{"nope"}, false); err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("err = %v", err)
	}
}

var _ fs.FS = skills.FS

func installNames(ss []Skill) []string {
	var n []string
	for _, s := range ss {
		n = append(n, s.InstallName)
	}
	return n
}
