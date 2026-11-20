package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/RamsesKools/wayfinder-maps/internal/installmethod"
)

func TestHelpHidesSelfUpdateOnHomebrew(t *testing.T) {
	for _, m := range []installmethod.Method{installmethod.Script, installmethod.Manual, installmethod.GoInstall, installmethod.Dev} {
		if h := helpText(m); !strings.Contains(h, "self-update") || !strings.Contains(h, "[--version vX.Y.Z] [--check] [--force]") {
			t.Errorf("%s: help should list self-update with its flags:\n%s", m, h)
		}
	}
	h := helpText(installmethod.Homebrew)
	if strings.Contains(h, "self-update") || strings.Contains(h, "--check") {
		t.Errorf("homebrew help must not mention self-update:\n%s", h)
	}
	for _, want := range []string{"status", "install-skill", "--version             print the version"} {
		if !strings.Contains(h, want) {
			t.Errorf("homebrew help lost %q:\n%s", want, h)
		}
	}
}

func TestSelfUpdateRefusedOnHomebrew(t *testing.T) {
	var out, errb bytes.Buffer
	for _, args := range [][]string{nil, {"--check"}, {"--force", "--version", "v0.1.0"}} {
		out.Reset()
		errb.Reset()
		if code := selfUpdateFor(installmethod.Homebrew, args, &out, &errb); code != 2 {
			t.Errorf("%v: code %d, want 2", args, code)
		}
		if !strings.Contains(errb.String(), "brew upgrade wayfinder-maps") || out.Len() != 0 {
			t.Errorf("%v: stderr %q, stdout %q", args, errb.String(), out.String())
		}
	}
}

func TestVersionLine(t *testing.T) {
	got := versionLine(installmethod.Install{Method: installmethod.Homebrew, Exe: "/b/wayfinder-maps"})
	if !strings.HasSuffix(got, " (homebrew, /b/wayfinder-maps)") || !strings.HasPrefix(got, "wayfinder-maps ") {
		t.Errorf("versionLine = %q", got)
	}
}
