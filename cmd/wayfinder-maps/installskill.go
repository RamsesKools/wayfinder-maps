package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/RamsesKools/wayfinder-maps/internal/selfupdate"
	"github.com/RamsesKools/wayfinder-maps/internal/skillinstall"
	"github.com/RamsesKools/wayfinder-maps/internal/version"
	"github.com/RamsesKools/wayfinder-maps/skills"
)

type installDeps struct {
	FS          fs.FS
	Home        string
	Stdin       io.Reader
	Interactive bool
	Version     string
	Commit      string
	Now         func() time.Time
}

type target struct {
	Dir     string
	Harness []string
}

type listFlag []string

func (l *listFlag) String() string { return strings.Join(*l, ",") }
func (l *listFlag) Set(v string) error {
	for _, s := range strings.Split(v, ",") {
		if s = strings.TrimSpace(s); s != "" {
			*l = append(*l, s)
		}
	}
	return nil
}

func installSkillCmd(args []string, stdout, stderr io.Writer) int {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(stderr, "wayfinder-maps: install-skill: %v\n", err)
		return 2
	}
	tty := isTerminal(os.Stdin)
	v, commit := "dev", ""
	if cur := version.Get(); cur != "dev" {
		v = "v" + cur
		commit = receiptCommit(v)
	}
	return installSkill(args, installDeps{
		FS: skills.FS, Home: home, Stdin: os.Stdin, Interactive: tty,
		Version: v, Commit: commit, Now: time.Now,
	}, stdout, stderr)
}

// isTerminal is a char-device check that excludes /dev/null, which is one too.
func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	null, err := os.Stat(os.DevNull)
	return err != nil || !os.SameFile(fi, null)
}

// receiptCommit reads the release commit from the install receipt next to the binary, if it is for this version.
func receiptCommit(v string) string {
	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe)
	}
	if err != nil {
		return ""
	}
	b, err := os.ReadFile(filepath.Join(filepath.Dir(exe), selfupdate.ReceiptName))
	if err != nil {
		return ""
	}
	var r selfupdate.Receipt
	if json.Unmarshal(b, &r) != nil || r.Version != v {
		return ""
	}
	return r.Commit
}

func installSkill(args []string, d installDeps, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("install-skill", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var dirs, only listFlag
	fs.Var(&dirs, "dir", "skills dir to install into (repeatable); default: the Claude Code, Copilot and Codex dirs")
	fs.Var(&only, "only", "install just these skills (comma separated or repeated)")
	optional := fs.Bool("with-optional", false, "also install the optional skills")
	dry := fs.Bool("dry-run", false, "list what would change, write nothing")
	force := fs.Bool("force", false, "replace wayfinder skill dirs that are unmanaged or locally edited")
	yes := fs.Bool("yes", false, "install into the default dirs without asking")
	fs.BoolVar(yes, "y", false, "same as --yes")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	fail := func(format string, a ...any) int {
		fmt.Fprintf(stderr, "wayfinder-maps: install-skill: "+format+"\n", a...)
		return 2
	}

	cat, err := skillinstall.Catalog(d.FS)
	if err != nil {
		return fail("%v", err)
	}
	selected, err := skillinstall.Select(cat, only, *optional)
	if err != nil {
		return fail("%v", err)
	}

	var targets []target
	switch {
	case len(dirs) > 0:
		for _, dir := range dirs {
			targets = append(targets, target{Dir: dir})
		}
	default:
		targets = defaultTargets(d.Home)
		if !*dry && !*yes {
			if !d.Interactive {
				return fail("not a terminal; pass --yes to install into the default dirs, or --dir <path>")
			}
			targets = confirm(targets, d, bufio.NewReader(d.Stdin), stdout)
			if len(targets) == 0 {
				fmt.Fprintln(stdout, "Nothing selected, nothing installed.")
				return 0
			}
		}
	}

	var plans []*skillinstall.Plan
	blocked := false
	for _, t := range targets {
		p, err := skillinstall.NewPlan(t.Dir, selected, *force)
		if err != nil {
			return fail("%v", err)
		}
		plans = append(plans, p)
		for _, c := range p.Conflicts() {
			fmt.Fprintf(stderr, "wayfinder-maps: install-skill: %s: %s %s; use --force to replace it\n", tilde(t.Dir, d.Home), c.Skill, c.Reason)
			blocked = true
		}
	}
	if blocked {
		return fail("nothing was changed")
	}

	for i, p := range plans {
		printPlan(stdout, p, targets[i], d.Home)
	}
	if *dry {
		fmt.Fprintln(stdout, "\nDry run, nothing written.")
		return 0
	}
	changed := false
	for _, p := range plans {
		if err := p.Apply(skillinstall.Meta{Version: d.Version, Commit: d.Commit, Now: d.Now()}); err != nil {
			return fail("%v", err)
		}
		changed = changed || p.Changed()
	}
	if !changed {
		fmt.Fprintf(stdout, "\nSkills %s already up to date.\n", d.Version)
		return 0
	}
	fmt.Fprintf(stdout, "\nInstalled wayfinder-maps skills %s.\n", d.Version)
	return 0
}

// defaultTargets lists each harness's personal skills dir, merging harnesses whose dirs resolve to the same place.
func defaultTargets(home string) []target {
	harnesses := []target{
		{filepath.Join(home, ".claude", "skills"), []string{"Claude Code"}},
		{filepath.Join(home, ".copilot", "skills"), []string{"GitHub Copilot"}},
		{filepath.Join(home, ".agents", "skills"), []string{"Codex"}},
	}
	var out []target
	seen := map[string]int{}
	for _, h := range harnesses {
		key := resolve(h.Dir)
		if i, ok := seen[key]; ok {
			out[i].Harness = append(out[i].Harness, h.Harness...)
			continue
		}
		seen[key] = len(out)
		out = append(out, h)
	}
	return out
}

// resolve follows symlinks through the deepest part of p that exists.
func resolve(p string) string {
	rest := ""
	for cur := p; ; cur = filepath.Dir(cur) {
		if r, err := filepath.EvalSymlinks(cur); err == nil {
			return filepath.Join(r, rest)
		}
		if filepath.Dir(cur) == cur {
			return p
		}
		rest = filepath.Join(filepath.Base(cur), rest)
	}
}

func confirm(targets []target, d installDeps, in *bufio.Reader, out io.Writer) []target {
	fmt.Fprintln(out, "Install the wayfinder-maps skills into:")
	var picked []target
	for _, t := range targets {
		fmt.Fprintf(out, "  %s (%s)? [Y/n] ", tilde(t.Dir, d.Home), strings.Join(t.Harness, ", "))
		line, err := in.ReadString('\n')
		switch a := strings.ToLower(strings.TrimSpace(line)); {
		case a == "" || a == "y" || a == "yes":
			if err == nil || a != "" {
				picked = append(picked, t)
			}
		}
		if err != nil {
			fmt.Fprintln(out)
			break
		}
	}
	return picked
}

func printPlan(w io.Writer, p *skillinstall.Plan, t target, home string) {
	label := tilde(t.Dir, home)
	if len(t.Harness) > 0 {
		label += " (" + strings.Join(t.Harness, ", ") + ")"
	}
	fmt.Fprintf(w, "%s\n", label)
	for _, c := range p.Changes {
		fmt.Fprintf(w, "  %-9s %s\n", c.Action, c.Skill)
	}
}

func tilde(p, home string) string {
	if rel, err := filepath.Rel(home, p); err == nil && !strings.HasPrefix(rel, "..") {
		return "~/" + filepath.ToSlash(rel)
	}
	return p
}
