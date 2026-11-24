package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// settings are the viewer preferences kept across restarts. They live in a file
// rather than localStorage, which is per origin and app mode picks a random port.
type settings struct {
	Direction   string `json:"direction"`
	FreezeNodes bool   `json:"freezeNodes"`
	UpdateCheck bool   `json:"updateCheck"`
	// GHAccount is the default gh account for pull and Open from GitHub; empty means the active one.
	GHAccount string `json:"ghAccount"`
	// ZoomButtons zooms with the +/- buttons instead of the wheel.
	ZoomButtons bool `json:"zoomButtons"`
}

// settingsPatch is a partial update: absent fields keep their stored value.
type settingsPatch struct {
	Direction   *string `json:"direction"`
	FreezeNodes *bool   `json:"freezeNodes"`
	UpdateCheck *bool   `json:"updateCheck"`
	GHAccount   *string `json:"ghAccount"`
	ZoomButtons *bool   `json:"zoomButtons"`
}

// apply merges the patch into s, reporting false for an empty or invalid one.
func (p settingsPatch) apply(s *settings) bool {
	if p.Direction == nil && p.FreezeNodes == nil && p.UpdateCheck == nil && p.GHAccount == nil && p.ZoomButtons == nil {
		return false
	}
	if p.Direction != nil {
		if !validDirection(*p.Direction) {
			return false
		}
		s.Direction = *p.Direction
	}
	if p.FreezeNodes != nil {
		s.FreezeNodes = *p.FreezeNodes
	}
	if p.UpdateCheck != nil {
		s.UpdateCheck = *p.UpdateCheck
	}
	if p.ZoomButtons != nil {
		s.ZoomButtons = *p.ZoomButtons
	}
	if p.GHAccount != nil {
		a := strings.TrimSpace(*p.GHAccount)
		if !validAccount(a) {
			return false
		}
		s.GHAccount = a
	}
	return true
}

var accountRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,38}$`)

// validAccount reports whether a is empty (the active gh account) or a GitHub login.
func validAccount(a string) bool { return a == "" || accountRe.MatchString(a) }

const defaultDirection = "E"

// validDirection reports whether d is a compass point or "none" (radial layout).
func validDirection(d string) bool {
	switch d {
	case "N", "NE", "E", "SE", "S", "SW", "W", "NW", "none":
		return true
	}
	return false
}

func settingsFile() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "wayfinder-maps", "settings.json"), nil
}

// loadSettings reads the file, falling back to defaults for a missing,
// unreadable or invalid one.
func loadSettings() settings {
	s := settings{Direction: defaultDirection, UpdateCheck: true}
	f, err := settingsFile()
	if err != nil {
		return s
	}
	b, err := os.ReadFile(f)
	if err != nil {
		return s
	}
	got := s
	if json.Unmarshal(b, &got) == nil && validDirection(got.Direction) {
		s.Direction = got.Direction
		s.FreezeNodes = got.FreezeNodes
		s.UpdateCheck = got.UpdateCheck
		s.ZoomButtons = got.ZoomButtons
		if validAccount(got.GHAccount) {
			s.GHAccount = got.GHAccount
		}
	}
	return s
}

func saveSettings(s settings) error {
	f, err := settingsFile()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(f, b, 0o644)
}
