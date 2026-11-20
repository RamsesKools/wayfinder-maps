package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/RamsesKools/wayfinder-maps/internal/installmethod"
	"github.com/RamsesKools/wayfinder-maps/internal/updatecheck"
	"github.com/RamsesKools/wayfinder-maps/internal/version"
)

// updateChecker is a seam for tests; production builds it for the running binary.
var updateChecker = func() *updatecheck.Checker {
	m, _ := installmethod.Current()
	return updatecheck.New(version.Get(), m)
}

// notifier prints the outdated notice after a command, off the command's critical path.
type notifier struct {
	getenv   func(string) string
	tty      bool
	settings settings
	checker  *updatecheck.Checker
	out      io.Writer
	// settingsFile is named in the notice as the place to switch it off.
	settingsFile string
}

// start looks up the latest release in the background and returns a func that
// waits for it and prints the notice. It does nothing when notices are off.
func (n notifier) start() (wait func()) {
	if !n.settings.UpdateCheck || !updatecheck.ShouldNotify(n.getenv, n.tty) {
		return func() {}
	}
	done := make(chan string, 1)
	go func() {
		r, err := n.checker.Check(context.Background())
		if err != nil {
			done <- ""
			return
		}
		done <- r.Notice(n.settingsFile)
	}()
	return func() {
		if s := <-done; s != "" {
			fmt.Fprint(n.out, s)
		}
	}
}

// startUpdateNotice wires the notifier to the real environment and stderr.
func startUpdateNotice() (wait func()) {
	fi, err := os.Stderr.Stat()
	tty := err == nil && fi.Mode()&os.ModeCharDevice != 0
	f, _ := settingsFile()
	if home, err := os.UserHomeDir(); err == nil && home != "" && strings.HasPrefix(f, home+string(filepath.Separator)) {
		f = "~" + f[len(home):]
	}
	return notifier{getenv: os.Getenv, tty: tty, settings: loadSettings(), checker: updateChecker(), out: os.Stderr, settingsFile: f}.start()
}

// updateInfo is the viewer's view of the check: only the current version while it is off.
type updateInfo struct {
	Enabled  bool   `json:"enabled"`
	Current  string `json:"current"`
	Latest   string `json:"latest,omitempty"`
	Outdated bool   `json:"outdated,omitempty"`
	Hint     string `json:"hint,omitempty"`
}

var updateMu sync.Mutex

// currentUpdateInfo answers from the shared cache; it only reaches GitHub when that is stale.
func currentUpdateInfo() updateInfo {
	c := updateChecker()
	info := updateInfo{Current: c.Current}
	if !loadSettings().UpdateCheck || c.Method == installmethod.Dev {
		return info
	}
	info.Enabled = true
	updateMu.Lock()
	defer updateMu.Unlock()
	if r, err := c.Check(context.Background()); err == nil {
		info.Latest, info.Outdated, info.Hint = r.Latest, r.Outdated, r.Hint
	}
	return info
}
