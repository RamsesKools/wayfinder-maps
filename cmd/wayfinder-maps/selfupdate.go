package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/RamsesKools/wayfinder-maps/internal/installmethod"
	"github.com/RamsesKools/wayfinder-maps/internal/selfupdate"
	"github.com/RamsesKools/wayfinder-maps/internal/version"
)

// Same overrides as install.sh, for tests and mirrors.
const (
	releasesURLEnv = "WAYFINDER_RELEASES_URL"
	apiURLEnv      = "WAYFINDER_API_URL"
)

// selfUpdateFor refuses on a Homebrew install before touching anything, since brew owns those files.
func selfUpdateFor(m installmethod.Method, args []string, stdout, stderr io.Writer) int {
	if m == installmethod.Homebrew {
		fmt.Fprintf(stderr, "wayfinder-maps: self-update refused: this install is managed by Homebrew; run %s\n", installmethod.Hint(m))
		return 2
	}
	return selfUpdate(args, stdout, stderr)
}

func selfUpdate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("self-update", flag.ContinueOnError)
	fs.SetOutput(stderr)
	pin := fs.String("version", "", "install this release (vX.Y.Z) instead of the latest")
	check := fs.Bool("check", false, "report only; exit 1 when behind")
	force := fs.Bool("force", false, "reinstall even when already at the target version")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe)
	}
	if err != nil {
		fmt.Fprintf(stderr, "wayfinder-maps: self-update: %v\n", err)
		return 2
	}
	u := &selfupdate.Updater{
		ReleasesURL: selfupdate.DefaultReleasesURL,
		APIURL:      selfupdate.DefaultAPIURL,
		GOOS:        runtime.GOOS,
		GOARCH:      runtime.GOARCH,
		Current:     version.Get(),
		Exe:         exe,
	}
	if v := os.Getenv(releasesURLEnv); v != "" {
		u.ReleasesURL = v
	}
	if v := os.Getenv(apiURLEnv); v != "" {
		u.APIURL = v
	}
	return runSelfUpdate(u, *pin, *check, *force, stdout, stderr)
}

func runSelfUpdate(u *selfupdate.Updater, pin string, check, force bool, stdout, stderr io.Writer) int {
	ctx := context.Background()
	if check {
		st, err := u.Check(ctx, pin)
		if err != nil {
			return updateFailed(stderr, err)
		}
		if st.Behind {
			fmt.Fprintf(stdout, "wayfinder-maps %s is behind %s; run wayfinder-maps self-update\n", u.Current, st.Target)
			return 1
		}
		fmt.Fprintf(stdout, "wayfinder-maps %s is up to date (target %s)\n", u.Current, st.Target)
		return 0
	}
	st, err := u.Update(ctx, pin, force)
	if err != nil {
		return updateFailed(stderr, err)
	}
	if st.NoChange {
		fmt.Fprintf(stdout, "wayfinder-maps %s is already installed; use --force to reinstall\n", u.Current)
		return 0
	}
	fmt.Fprintf(stdout, "wayfinder-maps %s -> %s (%s, sha256 verified)\n", u.Current, st.Target, st.Asset)
	return 0
}

func updateFailed(stderr io.Writer, err error) int {
	var refusal *selfupdate.RefusalError
	if errors.As(err, &refusal) {
		fmt.Fprintf(stderr, "wayfinder-maps: self-update refused: %v\n", err)
	} else {
		fmt.Fprintf(stderr, "wayfinder-maps: self-update failed: %v\n", err)
	}
	return 2
}
