package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/RamsesKools/wayfinder-maps/internal/ghpull"
	"github.com/RamsesKools/wayfinder-maps/internal/installmethod"
	"github.com/RamsesKools/wayfinder-maps/internal/version"
	"github.com/RamsesKools/wayfinder-maps/internal/wayfinder"
)

const usageText = `wayfinder-maps — inspect a wayfinder map

usage:
  wayfinder-maps status <effort-dir>   print the frontier and the state of the map
  wayfinder-maps lint   <effort-dir>   check the map's format invariants
  wayfinder-maps pull [flags] <owner/repo> <map-issue> [out-dir]
                                       mirror a map kept as GitHub issues into .plan/
  wayfinder-maps pull [flags] --refresh <out-dir>
                                       re-pull a mirrored map from its .source.json
      --account <gh-user>              pull as this gh account (default: WAYFINDER_GH_ACCOUNT, or ghAccount in settings.json)
      -q, --quiet                      no progress bar or "pulled N/M items" lines
  wayfinder-maps serve  [path]         open in a browser (local web server)
  wayfinder-maps app    [path]         open in a native window
  wayfinder-maps self-update           upgrade a release install in place
                 [--version vX.Y.Z] [--check] [--force]
  wayfinder-maps install-skill         write the matching skills into your agent skills dirs
                 [--dir <path>]... [--only a,b] [--with-optional] [--dry-run] [--force] [--yes]
  wayfinder-maps --version             print the version

<effort-dir> holds map.md and tickets/; status/lint default to the working dir.
pull needs gh (logged in); the account is stored in .source.json; out-dir defaults to .plan/<slug of the map title>.
For serve/app, [path] is optional: an effort opens straight into its map, a
project (a folder with a .plan/) into its map list, and no path shows a splash
with an Open Folder button and recent projects.

serve listens on :7777 (override with PORT) and re-reads the map on each request.
app wraps the same server in a native window (requires a cgo build).

install-skill proposes ~/.claude/skills, ~/.copilot/skills and ~/.agents/skills (Claude Code, GitHub
Copilot, Codex) and asks per dir; --yes (or --dir) skips the questions, for automation. Skills install
as wayfinder-<name> and are tracked in .wayfinder-maps-skills.lock.json in each dir.

exit: 0 clean, 1 lint errors found, 2 could not read the map`

// helpText hides self-update for Homebrew installs, which brew owns.
func helpText(m installmethod.Method) string {
	if m != installmethod.Homebrew {
		return usageText
	}
	var keep []string
	lines := strings.Split(usageText, "\n")
	for i := 0; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "  wayfinder-maps self-update") {
			i++
			continue
		}
		keep = append(keep, lines[i])
	}
	return strings.Join(keep, "\n")
}

func versionLine(i installmethod.Install) string {
	return fmt.Sprintf("%s (%s)", version.String("wayfinder-maps"), i)
}

func main() {
	args := os.Args[1:]
	wait := func() {}
	// --version and self-update cover the version themselves; the rest end with the outdated notice.
	if len(args) > 0 && args[0] != "self-update" && args[0] != "--version" && args[0] != "-h" && args[0] != "--help" {
		wait = startUpdateNotice()
		// serve and app block until interrupted, so their notice cannot wait for the end.
		if args[0] == "serve" || args[0] == "app" {
			go wait()
			wait = func() {}
		}
	}
	code := dispatch(args)
	wait()
	os.Exit(code)
}

func dispatch(args []string) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		fmt.Println(helpText(installmethod.Current().Method))
		return 0
	}

	if args[0] == "--version" {
		fmt.Println(versionLine(installmethod.Current()))
		return 0
	}

	if args[0] == "pull" {
		return pullCmd(args[1:], ghpull.GH, os.Stdout, os.Stderr)
	}

	if args[0] == "self-update" {
		return selfUpdateFor(installmethod.Current().Method, args[1:], os.Stdout, os.Stderr)
	}

	if args[0] == "install-skill" {
		return installSkillCmd(args[1:], os.Stdout, os.Stderr)
	}

	cmd := args[0]
	dir := ""
	if len(args) > 1 {
		dir = args[1]
	}

	switch cmd {
	case "status", "lint":
		// These need a single effort; default to the working directory.
		d := dir
		if d == "" {
			d = "."
		}
		e, err := wayfinder.Load(d)
		if err != nil {
			fmt.Fprintf(os.Stderr, "wayfinder-maps: %v\n", err)
			return 2
		}
		if cmd == "status" {
			return status(os.Stdout, e)
		}
		return lint(e)
	case "serve":
		return serve(dir)
	case "app":
		return app(dir)
	default:
		fmt.Fprintf(os.Stderr, "wayfinder-maps: unknown command %q\n\n%s\n", cmd, helpText(installmethod.Current().Method))
		return 2
	}
}

func status(w io.Writer, e *wayfinder.Effort) int {
	name := e.Map.Name
	if name == "" {
		name = e.Name
	}
	fmt.Fprintf(w, "%s\n", name)

	var impl, decisions []*wayfinder.Ticket
	for _, t := range e.Tickets {
		if t.Type == wayfinder.TypeImplement {
			impl = append(impl, t)
		} else {
			decisions = append(decisions, t)
		}
	}
	if len(impl) == 0 {
		fmt.Fprintf(w, "%s\n\n", counts(e.Tickets))
	} else {
		fmt.Fprintf(w, "Decisions:      %s\nImplementation: %s\n\n", counts(decisions), counts(impl))
	}

	frontier := e.Frontier()
	if len(frontier) == 0 {
		fmt.Fprintln(w, "Frontier: empty — no ticket is ready to claim.")
	} else {
		afk := 0
		for _, t := range frontier {
			if t.AFK() {
				afk++
			}
		}
		fmt.Fprintf(w, "Frontier — ready to claim, first by number wins (%d AFK, %d HITL):\n", afk, len(frontier)-afk)
		for _, t := range frontier {
			mark := "HITL"
			if t.AFK() {
				mark = "AFK"
			}
			fmt.Fprintf(w, "  %02d  %-46s  %-9s  %s\n", t.Num, truncate(t.Title, 46), t.Type, mark)
		}
	}

	var blocked, claimed []*wayfinder.Ticket
	for _, t := range e.Tickets {
		switch t.Status {
		case wayfinder.StatusClaimed:
			if t.Type != wayfinder.TypeImplement {
				claimed = append(claimed, t)
			}
		case wayfinder.StatusOpen:
			if !contains(frontier, t) {
				blocked = append(blocked, t)
			}
		}
	}

	if len(claimed) > 0 {
		fmt.Fprintln(w, "\nClaimed — in flight:")
		for _, t := range claimed {
			fmt.Fprintf(w, "  %02d  %-46s  by %s\n", t.Num, truncate(t.Title, 46), claimant(t))
		}
	}

	var shipping []*wayfinder.Ticket
	for _, t := range impl {
		if t.Status == wayfinder.StatusClaimed || t.Status == wayfinder.StatusResolved {
			shipping = append(shipping, t)
		}
	}
	if len(shipping) > 0 {
		fmt.Fprintln(w, "\nImplementation — in flight and shipped:")
		for _, t := range shipping {
			detail := "by " + claimant(t)
			if t.Status == wayfinder.StatusResolved {
				detail = "shipped without a PR"
				if t.PR != "" {
					detail = t.PR
				}
			}
			fmt.Fprintf(w, "  %02d  %-46s  %s\n", t.Num, truncate(t.Title, 46), detail)
		}
	}

	if len(blocked) > 0 {
		fmt.Fprintln(w, "\nBlocked:")
		for _, t := range blocked {
			fmt.Fprintf(w, "  %02d  %-46s  waits on %s\n", t.Num, truncate(t.Title, 46), nums(t.BlockedBy))
		}
	}

	var undermined []*wayfinder.Ticket
	for _, t := range e.Tickets {
		if len(t.UnderminedBy) > 0 {
			undermined = append(undermined, t)
		}
	}
	if len(undermined) > 0 {
		fmt.Fprintln(w, "\nUndermined — resolved on a premise that later changed:")
		for _, t := range undermined {
			fmt.Fprintf(w, "  %02d  %-46s  broken by %s\n", t.Num, truncate(t.Title, 46), nums(t.UnderminedBy))
		}
	}

	anchored := 0
	for _, f := range e.Map.Fog {
		if f.ClearsWith != 0 {
			anchored++
		}
	}
	fmt.Fprintf(w, "\nFog: %d patches, %d anchored to a ticket\n", len(e.Map.Fog), anchored)
	return 0
}

func counts(ts []*wayfinder.Ticket) string {
	var n [4]int
	for _, t := range ts {
		switch t.Status {
		case wayfinder.StatusResolved:
			n[0]++
		case wayfinder.StatusClaimed:
			n[1]++
		case wayfinder.StatusOpen:
			n[2]++
		case wayfinder.StatusOutOfScope:
			n[3]++
		}
	}
	return fmt.Sprintf("%d resolved · %d claimed · %d open · %d out of scope", n[0], n[1], n[2], n[3])
}

func claimant(t *wayfinder.Ticket) string {
	if t.ClaimedBy == "" {
		return "unknown"
	}
	return t.ClaimedBy
}

func lint(e *wayfinder.Effort) int {
	diags := wayfinder.Lint(e, wayfinder.DefaultOptions())
	if len(diags) == 0 {
		fmt.Printf("%s: clean — %d tickets, no drift\n", e.Name, len(e.Tickets))
		return 0
	}

	sort.SliceStable(diags, func(i, j int) bool {
		if diags[i].Level != diags[j].Level {
			return diags[i].Level > diags[j].Level
		}
		return diags[i].File < diags[j].File
	})

	errs := 0
	for _, d := range diags {
		if d.Level == wayfinder.Error {
			errs++
		}
		loc := rel(d.File)
		if d.Line > 0 {
			loc = fmt.Sprintf("%s:%d", loc, d.Line)
		}
		fmt.Printf("%s: %s: %s\n", loc, d.Level, d.Msg)
	}

	fmt.Printf("\n%d error(s), %d warning(s)\n", errs, len(diags)-errs)
	if errs > 0 {
		return 1
	}
	return 0
}

func rel(p string) string {
	if wd, err := os.Getwd(); err == nil {
		if r, err := filepath.Rel(wd, p); err == nil && !strings.HasPrefix(r, "..") {
			return r
		}
	}
	return p
}

func contains(ts []*wayfinder.Ticket, want *wayfinder.Ticket) bool {
	for _, t := range ts {
		if t == want {
			return true
		}
	}
	return false
}

func nums(ns []int) string {
	var s []string
	for _, n := range ns {
		s = append(s, fmt.Sprintf("%02d", n))
	}
	return strings.Join(s, ", ")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
