package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/RamsesKools/wayfinder-maps/internal/ghpull"
)

// pullSource is what a pull records in <effort>/.source.json.
type pullSource = ghpull.Source

// readPullSource reports whether the effort is a GitHub mirror made by
// wayfinder-maps pull, and where from.
func readPullSource(effort string) (pullSource, bool) { return ghpull.ReadSource(effort) }

var pullMu sync.Mutex

// newPuller is the seam for the gh runner and the environment.
func newPuller(gh ghpull.Runner, stdout, stderr io.Writer) *ghpull.Puller {
	return &ghpull.Puller{GH: gh, Getenv: os.Getenv, Stdout: stdout, Stderr: stderr}
}

// runPull re-pulls the effort from GitHub in-process, reusing the gh account
// recorded at the first pull.
var runPull = func(effort string) error {
	pullMu.Lock()
	defer pullMu.Unlock()
	return newPuller(ghpull.GH, io.Discard, io.Discard).Pull(ghpull.Options{Out: effort, Refresh: true})
}

// pullCmd is `wayfinder-maps pull`; it returns the exit code.
func pullCmd(args []string, gh ghpull.Runner, stdout, stderr io.Writer) int {
	var o ghpull.Options
	var pos []string
	for _, a := range args {
		switch {
		case a == "--refresh":
			o.Refresh = true
		case strings.HasPrefix(a, "-") && a != "-":
			fmt.Fprintf(stderr, "wayfinder-maps pull: unknown option %s\n", a)
			return 2
		default:
			pos = append(pos, a)
		}
	}
	if o.Refresh {
		if len(pos) > 0 {
			o.Out = pos[0]
		}
	} else {
		if len(pos) > 0 {
			o.Repo = pos[0]
		}
		if len(pos) > 1 {
			n, err := strconv.Atoi(pos[1])
			if err != nil || n <= 0 {
				fmt.Fprintf(stderr, "wayfinder-maps pull: map issue number must be a positive integer, got %q\n", pos[1])
				return 2
			}
			o.Map = n
		}
		if len(pos) > 2 {
			o.Out = pos[2]
		}
	}
	if err := newPuller(gh, stdout, stderr).Pull(o); err != nil {
		fmt.Fprintf(stderr, "wayfinder-maps pull: %v\n", err)
		var pe *ghpull.Error
		if errors.As(err, &pe) {
			return pe.Code
		}
		return 1
	}
	return 0
}
