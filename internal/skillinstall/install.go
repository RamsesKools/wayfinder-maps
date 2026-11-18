package skillinstall

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// LockName is written in the target dir; skills listed in it are the ones this tool manages.
const LockName = ".wayfinder-maps-skills.lock.json"

// Lock records what is installed in one skills dir.
type Lock struct {
	Version     string                 `json:"version"`
	Commit      string                 `json:"commit"`
	InstalledAt string                 `json:"installed_at"`
	Skills      map[string]LockedSkill `json:"skills"`
}

// LockedSkill maps each file of an installed skill to its sha256.
type LockedSkill struct {
	Files map[string]string `json:"files"`
}

// Meta is stamped into the lock when something changes.
type Meta struct {
	Version string
	Commit  string
	Now     time.Time
}

// Action is what a plan does to one skill dir.
type Action int

const (
	Unchanged Action = iota
	Create
	Update   // managed and unedited, content differs
	Replace  // forced over a conflict
	Conflict // unmanaged dir or local edits, needs force
)

func (a Action) String() string {
	return [...]string{"unchanged", "create", "update", "replace", "conflict"}[a]
}

// Change is the planned outcome for one skill.
type Change struct {
	Skill  string
	Action Action
	Reason string
}

// Plan is the set of changes for one target dir, computed without touching disk.
type Plan struct {
	Dir     string
	Changes []Change
	skills  []Skill
	lock    Lock
}

func hash(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// ReadLock returns the lock in dir, or an empty one when there is none.
func ReadLock(dir string) (Lock, error) {
	b, err := os.ReadFile(filepath.Join(dir, LockName))
	if errors.Is(err, fs.ErrNotExist) {
		return Lock{Skills: map[string]LockedSkill{}}, nil
	}
	if err != nil {
		return Lock{}, err
	}
	var l Lock
	if err := json.Unmarshal(b, &l); err != nil {
		return Lock{}, fmt.Errorf("%s: %w", filepath.Join(dir, LockName), err)
	}
	if l.Skills == nil {
		l.Skills = map[string]LockedSkill{}
	}
	return l, nil
}

// NewPlan compares skills against dir. With force, conflicts become Replace.
func NewPlan(dir string, skills []Skill, force bool) (*Plan, error) {
	lock, err := ReadLock(dir)
	if err != nil {
		return nil, err
	}
	p := &Plan{Dir: dir, skills: skills, lock: lock}
	for _, s := range skills {
		c, err := classify(dir, s, lock)
		if err != nil {
			return nil, err
		}
		if c.Action == Conflict && force {
			c.Action = Replace
		}
		p.Changes = append(p.Changes, c)
	}
	return p, nil
}

func classify(dir string, s Skill, lock Lock) (Change, error) {
	c := Change{Skill: s.InstallName}
	target := filepath.Join(dir, s.InstallName)
	info, err := os.Lstat(target)
	if errors.Is(err, fs.ErrNotExist) {
		c.Action = Create
		return c, nil
	}
	if err != nil {
		return c, err
	}
	locked, ok := lock.Skills[s.InstallName]
	switch {
	case !ok:
		c.Action, c.Reason = Conflict, "exists but is not in the lock"
		return c, nil
	case !info.IsDir():
		c.Action, c.Reason = Conflict, "is not a directory"
		return c, nil
	}
	have, err := hashDir(target)
	if err != nil {
		return c, err
	}
	if drift := diff(locked.Files, have); len(drift) > 0 {
		c.Action, c.Reason = Conflict, "differs from the lock: "+strings.Join(drift, ", ")
		return c, nil
	}
	if len(diff(have, hashFiles(s))) == 0 {
		c.Action = Unchanged
	} else {
		c.Action = Update
	}
	return c, nil
}

func hashFiles(s Skill) map[string]string {
	m := make(map[string]string, len(s.Files))
	for p, b := range s.Files {
		m[p] = hash(b)
	}
	return m
}

func hashDir(dir string) (map[string]string, error) {
	m := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		m[filepath.ToSlash(rel)] = hash(b)
		return nil
	})
	return m, err
}

// diff lists the paths whose hash differs between a and b, or that exist in only one.
func diff(a, b map[string]string) []string {
	var out []string
	for p, h := range a {
		if b[p] != h {
			out = append(out, p)
		}
	}
	for p := range b {
		if _, ok := a[p]; !ok {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

// Conflicts lists the changes that need --force.
func (p *Plan) Conflicts() []Change { return p.byAction(Conflict) }

// Changed reports whether Apply would write anything.
func (p *Plan) Changed() bool {
	for _, c := range p.Changes {
		if c.Action != Unchanged {
			return true
		}
	}
	return false
}

func (p *Plan) byAction(a Action) []Change {
	var out []Change
	for _, c := range p.Changes {
		if c.Action == a {
			out = append(out, c)
		}
	}
	return out
}

// Apply writes the planned skills and the lock. It refuses a plan with conflicts and does nothing when nothing changed.
func (p *Plan) Apply(m Meta) error {
	if len(p.Conflicts()) > 0 {
		return errors.New("plan has conflicts; use force to replace them")
	}
	if !p.Changed() {
		return nil
	}
	if err := os.MkdirAll(p.Dir, 0o755); err != nil {
		return err
	}
	lock := p.lock
	for i, c := range p.Changes {
		if c.Action == Unchanged {
			continue
		}
		s := p.skills[i]
		if err := writeSkill(p.Dir, s); err != nil {
			return err
		}
		lock.Skills[s.InstallName] = LockedSkill{Files: hashFiles(s)}
	}
	lock.Version, lock.Commit, lock.InstalledAt = m.Version, m.Commit, m.Now.UTC().Format(time.RFC3339)
	b, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(filepath.Join(p.Dir, LockName), append(b, '\n'))
}

// writeSkill stages the skill beside its target, then swaps it in by rename.
func writeSkill(dir string, s Skill) error {
	target := filepath.Join(dir, s.InstallName)
	stage, old := target+".wayfinder-new", target+".wayfinder-old"
	for _, p := range []string{stage, old} {
		if err := os.RemoveAll(p); err != nil {
			return err
		}
	}
	for rel, b := range s.Files {
		dst := filepath.Join(stage, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dst, b, 0o644); err != nil {
			return err
		}
	}
	if _, err := os.Lstat(target); err == nil {
		if err := os.Rename(target, old); err != nil {
			return err
		}
	}
	if err := os.Rename(stage, target); err != nil {
		return err
	}
	return os.RemoveAll(old)
}

func writeAtomic(path string, b []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
