// Package skillinstall copies the embedded skills into a skills dir and tracks them in a lock file.
package skillinstall

import (
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"
)

// Prefix is put in front of every installed skill name except Unprefixed.
const (
	Prefix      = "wayfinder-"
	Unprefixed  = "wayfinder-maps"
	optionalDir = ".optional"
)

var core = map[string]bool{
	"wayfinder-maps": true, "grill-me": true, "research": true, "prototype": true, "domain-modeling": true,
}

// Skill is one skill as it will be written to disk.
type Skill struct {
	Name        string // upstream name, the dir under skills/
	InstallName string // dir name and frontmatter name once installed
	Optional    bool
	Files       map[string][]byte // slash-separated relative path to content
}

func installName(name string) string {
	if name == Unprefixed || strings.HasPrefix(name, Prefix) {
		return name
	}
	return Prefix + name
}

// Catalog reads every skill under fsys (and fsys/.optional) and applies the install naming.
func Catalog(fsys fs.FS) ([]Skill, error) {
	var cat []Skill
	for _, root := range []string{".", optionalDir} {
		entries, err := fs.ReadDir(fsys, root)
		if err != nil {
			if root == optionalDir {
				continue
			}
			return nil, err
		}
		for _, e := range entries {
			if !e.IsDir() || e.Name() == optionalDir {
				continue
			}
			dir := path.Join(root, e.Name())
			if _, err := fs.Stat(fsys, path.Join(dir, "SKILL.md")); err != nil {
				continue
			}
			files, err := readTree(fsys, dir)
			if err != nil {
				return nil, err
			}
			cat = append(cat, Skill{Name: e.Name(), InstallName: installName(e.Name()), Optional: root == optionalDir, Files: files})
		}
	}
	license, _ := fs.ReadFile(fsys, "LICENSE")
	for i := range cat {
		if _, ok := cat[i].Files["LICENSE"]; !ok && license != nil {
			cat[i].Files["LICENSE"] = license
		}
	}
	rename(cat)
	sort.Slice(cat, func(i, j int) bool { return cat[i].InstallName < cat[j].InstallName })
	return cat, nil
}

func readTree(fsys fs.FS, dir string) (map[string][]byte, error) {
	files := map[string][]byte{}
	err := fs.WalkDir(fsys, dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		files[strings.TrimPrefix(p, dir+"/")] = b
		return nil
	})
	return files, err
}

var nameLine = regexp.MustCompile(`(?m)\A---\nname: *([^\n]+)\n`)

// rename rewrites the frontmatter name and every backticked reference to a known skill.
func rename(cat []Skill) {
	byName := map[string]string{}
	for _, s := range cat {
		byName[s.Name] = s.InstallName
	}
	ref := regexp.MustCompile("`([a-z][a-z0-9-]*)`")
	for _, s := range cat {
		for p, b := range s.Files {
			if !strings.HasSuffix(p, ".md") {
				continue
			}
			out := ref.ReplaceAllFunc(b, func(m []byte) []byte {
				if to, ok := byName[string(m[1:len(m)-1])]; ok {
					return []byte("`" + to + "`")
				}
				return m
			})
			if p == "SKILL.md" {
				out = nameLine.ReplaceAll(out, []byte("---\nname: "+s.InstallName+"\n"))
			}
			s.Files[p] = out
		}
	}
}

// Select picks the skills to install: only (upstream or installed names) when given, else the core five, plus the optional ones with withOptional.
func Select(cat []Skill, only []string, withOptional bool) ([]Skill, error) {
	var out []Skill
	if len(only) > 0 {
		for _, want := range only {
			var hit *Skill
			for i := range cat {
				if cat[i].Name == want || cat[i].InstallName == want {
					hit = &cat[i]
				}
			}
			if hit == nil {
				return nil, fmt.Errorf("unknown skill %q (known: %s)", want, strings.Join(names(cat), ", "))
			}
			out = append(out, *hit)
		}
	} else {
		for _, s := range cat {
			if core[s.Name] || (s.Optional && withOptional) {
				out = append(out, s)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].InstallName < out[j].InstallName })
	return dedupe(out), nil
}

func dedupe(ss []Skill) []Skill {
	var out []Skill
	for i, s := range ss {
		if i == 0 || ss[i-1].InstallName != s.InstallName {
			out = append(out, s)
		}
	}
	return out
}

func names(cat []Skill) []string {
	var n []string
	for _, s := range cat {
		n = append(n, s.Name)
	}
	return n
}
