// Package installmethod tells how the running binary was installed, so callers
// can pick the matching update command.
package installmethod

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/RamsesKools/wayfinder-maps/internal/selfupdate"
	"github.com/RamsesKools/wayfinder-maps/internal/version"
)

// Method is how a binary got onto the machine.
type Method string

const (
	Dev       Method = "dev"
	Script    Method = "install.sh"
	Homebrew  Method = "homebrew"
	GoInstall Method = "go install"
	Manual    Method = "manual"
)

// Env is everything Detect looks at, so tests need no real installs.
type Env struct {
	// Version is the build version, "dev" for a source build.
	Version string
	// Exe is the running binary with symlinks resolved.
	Exe    string
	GOBIN  string
	GOPATH string
	Home   string
}

// Install is the detected method and the binary path it came from; Exe is empty when unknown.
type Install struct {
	Method Method
	Exe    string
}

func (i Install) String() string {
	if i.Exe == "" {
		return string(i.Method)
	}
	return string(i.Method) + ", " + i.Exe
}

// Current detects how the running binary was installed.
func Current() Install {
	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe)
	}
	home, _ := os.UserHomeDir()
	env := Env{Version: version.Get(), Exe: exe, GOBIN: os.Getenv("GOBIN"), GOPATH: os.Getenv("GOPATH"), Home: home}
	if err != nil {
		if env.Version == "dev" {
			return Install{Method: Dev}
		}
		return Install{Method: Manual}
	}
	return Install{Method: Detect(env), Exe: exe}
}

// Detect classifies the install. The receipt wins over the path, because install.sh may write anywhere.
func Detect(e Env) Method {
	if e.Version == "" || e.Version == "dev" {
		return Dev
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(e.Exe), selfupdate.ReceiptName)); err == nil {
		return Script
	}
	if inBrew(e.Exe) {
		return Homebrew
	}
	if isGoBin(e) {
		return GoInstall
	}
	return Manual
}

func inBrew(exe string) bool {
	for _, p := range strings.Split(filepath.ToSlash(exe), "/") {
		if p == "Cellar" || p == "Caskroom" {
			return true
		}
	}
	return false
}

// isGoBin follows go install's destination: GOBIN, else the first GOPATH entry's bin, else ~/go/bin.
func isGoBin(e Env) bool {
	dir := filepath.Dir(e.Exe)
	var bins []string
	if e.GOBIN != "" {
		bins = append(bins, e.GOBIN)
	}
	for _, p := range filepath.SplitList(e.GOPATH) {
		bins = append(bins, filepath.Join(p, "bin"))
	}
	if e.Home != "" {
		bins = append(bins, filepath.Join(e.Home, "go", "bin"))
	}
	for _, b := range bins {
		if filepath.Clean(b) == dir {
			return true
		}
	}
	return false
}

// Hint is the command or page that updates this kind of install; empty for a dev build.
func Hint(m Method) string {
	switch m {
	case Script:
		return "wayfinder-maps self-update"
	case Homebrew:
		return "brew upgrade wayfinder-maps"
	case GoInstall:
		return "go install github.com/RamsesKools/wayfinder-maps/cmd/wayfinder-maps@latest"
	case Manual:
		return selfupdate.DefaultReleasesURL + "/latest"
	}
	return ""
}
