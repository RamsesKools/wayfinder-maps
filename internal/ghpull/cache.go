package ghpull

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// githubCache is where mirrors made from the viewer live, under the user cache dir.
func githubCache(cacheRoot string) string {
	return filepath.Join(cacheRoot, "wayfinder-maps", "github")
}

// MirrorDir is where the map is mirrored: <cache>/wayfinder-maps/github/<owner>-<repo>/<slug>.
// A slug already taken by a different map of the repo gets "-<number>" appended.
func MirrorDir(cacheRoot, repo, title string, num int) string {
	repoDir := filepath.Join(githubCache(cacheRoot), strings.ReplaceAll(repo, "/", "-"))
	slug := Slugify(title)
	if slug == "" {
		slug = "map-" + strconv.Itoa(num)
	}
	dir := filepath.Join(repoDir, slug)
	if src, ok := ReadSource(dir); ok && src.Map != num {
		dir += "-" + strconv.Itoa(num)
	}
	return dir
}

// FindMirror locates the cached mirror of a repo's map by issue number, so a
// map renamed on GitHub is still found under its old slug.
func FindMirror(cacheRoot, repo string, num int) (string, bool) {
	repoDir := filepath.Join(githubCache(cacheRoot), strings.ReplaceAll(repo, "/", "-"))
	entries, _ := os.ReadDir(repoDir)
	for _, e := range entries {
		dir := filepath.Join(repoDir, e.Name())
		if src, ok := ReadSource(dir); ok && src.Map == num && strings.EqualFold(src.Repo, repo) {
			return dir, true
		}
	}
	return "", false
}

// IsMirror reports whether path is exactly <cache>/.../<owner>-<repo>/<slug>
// holding a pull's .source.json, so cleanup can never reach anything else.
func IsMirror(cacheRoot, path string) bool {
	if path == "" {
		return false
	}
	rel, err := filepath.Rel(githubCache(cacheRoot), filepath.Clean(path))
	if err != nil || strings.HasPrefix(rel, "..") || len(strings.Split(rel, string(filepath.Separator))) != 2 {
		return false
	}
	if fi, err := os.Lstat(path); err != nil || !fi.IsDir() {
		return false
	}
	_, ok := ReadSource(path)
	return ok
}

// RemoveMirror deletes a cached mirror, and its repo dir once that is empty.
func RemoveMirror(cacheRoot, path string) error {
	if !IsMirror(cacheRoot, path) {
		return fmt.Errorf("%s is not a cached GitHub map", path)
	}
	path = filepath.Clean(path)
	if err := os.RemoveAll(path); err != nil {
		return err
	}
	os.Remove(filepath.Dir(path)) // fails, harmlessly, while sibling maps remain
	return nil
}
