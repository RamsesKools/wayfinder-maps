// Package updatecheck finds out whether a newer release exists, caching the
// answer so most runs make no network call.
package updatecheck

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/RamsesKools/wayfinder-maps/internal/installmethod"
	"github.com/RamsesKools/wayfinder-maps/internal/selfupdate"
)

const (
	// SilenceEnv turns off both the check and the notice.
	SilenceEnv = "WAYFINDER_NO_UPDATE_CHECK"

	successTTL = 24 * time.Hour
	// A failed lookup (offline, rate limit) is not retried on every run.
	failureTTL = time.Hour
	timeout    = 2 * time.Second
)

// Result is what the check found; Current is without the leading v, Latest has it.
type Result struct {
	Current  string `json:"current"`
	Latest   string `json:"latest"`
	Outdated bool   `json:"outdated"`
	Hint     string `json:"hint,omitempty"`
}

// Notice is the text printed when outdated, empty otherwise.
func (r Result) Notice() string {
	if !r.Outdated {
		return ""
	}
	s := "wayfinder-maps v" + r.Current + " is outdated, " + r.Latest + " is available.\n"
	if r.Hint != "" {
		s += "Update: " + r.Hint + "\n"
	}
	return s + "Silence: export " + SilenceEnv + "=1\n"
}

// Checker looks up the latest release behind a file cache shared by the CLI and the server.
type Checker struct {
	ReleasesURL string
	// CacheFile is empty to disable caching.
	CacheFile string
	Method    installmethod.Method
	// Current is the running version without the leading v.
	Current string
	Client  *http.Client
	Now     func() time.Time
}

type cacheEntry struct {
	URL       string    `json:"url"`
	CheckedAt time.Time `json:"checkedAt"`
	// Latest is empty for a failed lookup.
	Latest string `json:"latest"`
}

// New builds a Checker for the running binary, honouring WAYFINDER_RELEASES_URL like self-update.
func New(current string, m installmethod.Method) *Checker {
	c := &Checker{ReleasesURL: selfupdate.DefaultReleasesURL, Method: m, Current: current}
	if v := os.Getenv("WAYFINDER_RELEASES_URL"); v != "" {
		c.ReleasesURL = v
	}
	if dir, err := os.UserCacheDir(); err == nil {
		c.CacheFile = filepath.Join(dir, "wayfinder-maps", "latest-release.json")
	}
	return c
}

// Disabled reports whether the environment silences the check.
func Disabled(getenv func(string) string) bool {
	v := strings.ToLower(getenv(SilenceEnv))
	return v != "" && v != "0" && v != "false"
}

// ShouldNotify reports whether the CLI may print a notice: on a terminal, outside CI, and not silenced.
func ShouldNotify(getenv func(string) string, stderrIsTerminal bool) bool {
	return stderrIsTerminal && getenv("CI") == "" && !Disabled(getenv)
}

// Cached answers from the cache alone and never touches the network; ok is false when it holds nothing fresh.
func (c *Checker) Cached() (Result, bool) {
	e, ok := c.read()
	if !ok || e.Latest == "" || c.now().Sub(e.CheckedAt) >= successTTL {
		return Result{}, false
	}
	return c.result(e.Latest), true
}

// Check answers from a fresh cache entry, else asks the release server once and caches the outcome.
func (c *Checker) Check(ctx context.Context) (Result, error) {
	if c.Method == installmethod.Dev || c.Current == "" || c.Current == "dev" {
		return Result{}, errors.New("dev build")
	}
	if e, ok := c.read(); ok {
		age := c.now().Sub(e.CheckedAt)
		if e.Latest != "" && age < successTTL {
			return c.result(e.Latest), nil
		}
		if e.Latest == "" && age < failureTTL {
			return Result{}, errors.New("latest release lookup failed recently")
		}
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	client := c.Client
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	u := &selfupdate.Updater{ReleasesURL: c.ReleasesURL, Client: client}
	latest, err := u.Latest(ctx)
	c.write(cacheEntry{URL: c.ReleasesURL, CheckedAt: c.now(), Latest: latest})
	if err != nil {
		return Result{}, err
	}
	return c.result(latest), nil
}

func (c *Checker) result(latest string) Result {
	r := Result{Current: c.Current, Latest: latest, Outdated: selfupdate.Newer(latest, "v"+c.Current)}
	if r.Outdated {
		r.Hint = installmethod.Hint(c.Method)
	}
	return r
}

func (c *Checker) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

func (c *Checker) read() (cacheEntry, bool) {
	if c.CacheFile == "" {
		return cacheEntry{}, false
	}
	b, err := os.ReadFile(c.CacheFile)
	if err != nil {
		return cacheEntry{}, false
	}
	var e cacheEntry
	if json.Unmarshal(b, &e) != nil || e.URL != c.ReleasesURL {
		return cacheEntry{}, false
	}
	return e, true
}

// write is best effort: a read-only cache dir just means more lookups.
func (c *Checker) write(e cacheEntry) {
	if c.CacheFile == "" {
		return
	}
	b, err := json.Marshal(e)
	if err != nil {
		return
	}
	if os.MkdirAll(filepath.Dir(c.CacheFile), 0o755) != nil {
		return
	}
	tmp := c.CacheFile + ".tmp"
	if os.WriteFile(tmp, b, 0o644) == nil {
		os.Rename(tmp, c.CacheFile)
	}
}
