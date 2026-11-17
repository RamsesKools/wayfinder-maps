// Package version holds the release version shared by every binary.
package version

import (
	"regexp"
	"runtime/debug"
	"strings"
)

// Version is set at build time: -ldflags "-X github.com/RamsesKools/wayfinder-maps/internal/version.Version=1.2.3".
var Version string

var release = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)

// Get returns the injected version, else the tagged module version (go install), else "dev".
func Get() string {
	build := ""
	if info, ok := debug.ReadBuildInfo(); ok {
		build = info.Main.Version
	}
	return resolve(Version, build)
}

// String formats the --version line for a tool.
func String(tool string) string { return tool + " " + Get() }

func resolve(injected, build string) string {
	if injected != "" {
		return injected
	}
	if release.MatchString(build) {
		return strings.TrimPrefix(build, "v")
	}
	return "dev"
}
