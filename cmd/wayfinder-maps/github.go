package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/RamsesKools/wayfinder-maps/internal/ghpull"
)

// Seams for tests: the gh runner and the user cache dir.
var (
	ghRunner     ghpull.Runner = ghpull.GH
	userCacheDir               = os.UserCacheDir
)

// runOpen pulls a map into o.Out in-process, serialised with refresh pulls.
var runOpen = func(o ghpull.Options, progress func(done, total int)) error {
	pullMu.Lock()
	defer pullMu.Unlock()
	o.Progress = progress
	return newPuller(ghRunner, io.Discard, io.Discard).Pull(o)
}

// maxRememberedRepos caps the repos kept for the picker's dropdown.
const maxRememberedRepos = 10

func reposFile() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "wayfinder-maps", "github-repos.json"), nil
}

// rememberedRepos are the repos maps were opened from, newest first.
func rememberedRepos() []string {
	f, err := reposFile()
	if err != nil {
		return nil
	}
	b, err := os.ReadFile(f)
	if err != nil {
		return nil
	}
	var repos []string
	json.Unmarshal(b, &repos)
	return repos
}

func rememberRepo(repo string) {
	f, err := reposFile()
	if err != nil {
		return
	}
	out := []string{repo}
	for _, r := range rememberedRepos() {
		if r != repo {
			out = append(out, r)
		}
	}
	if len(out) > maxRememberedRepos {
		out = out[:maxRememberedRepos]
	}
	os.MkdirAll(filepath.Dir(f), 0o755)
	if b, err := json.Marshal(out); err == nil {
		os.WriteFile(f, b, 0o644)
	}
}

// isCachedMirror reports whether path is a mirror the viewer pulled into the user cache dir.
func isCachedMirror(path string) bool {
	cache, err := userCacheDir()
	return err == nil && ghpull.IsMirror(cache, path)
}

// needsHeader answers 400 unless the request has the method and the custom
// header, which forces a CORS preflight so a foreign page cannot trigger it.
func needsHeader(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method || r.Header.Get("X-Wayfinder") == "" {
		http.Error(w, method+" with X-Wayfinder required", http.StatusBadRequest)
		return false
	}
	return true
}

func ghError(w http.ResponseWriter, err error, account string) {
	who := ""
	if account != "" {
		who = fmt.Sprintf("gh account '%s'", account)
	}
	err = ghpull.Explain(err, who)
	code := http.StatusBadGateway
	var pe *ghpull.Error
	if errors.As(err, &pe) {
		code = http.StatusBadRequest
	}
	http.Error(w, err.Error(), code)
}

// streamSSE runs fn and streams its progress as SSE: "progress" {done,total},
// then "done" with the data or "error" {message}.
func streamSSE(w http.ResponseWriter, fn func(progress func(done, total int)) (any, error)) {
	fl, _ := w.(http.Flusher)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	send := func(event string, v any) {
		b, _ := json.Marshal(v)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b)
		if fl != nil {
			fl.Flush()
		}
	}
	send("progress", map[string]int{"done": 0, "total": 0})
	data, err := fn(func(done, total int) { send("progress", map[string]int{"done": done, "total": total}) })
	if err != nil {
		send("error", map[string]string{"message": err.Error()})
		return
	}
	send("done", data)
}

// registerGitHub wires the endpoints that find and open maps kept as GitHub
// issues, and the cleanup of the mirrors they leave in the user cache dir.
func registerGitHub(mux *http.ServeMux, writeJSON func(http.ResponseWriter, any)) {
	mux.HandleFunc("/api/github/status", func(w http.ResponseWriter, r *http.Request) {
		if !needsHeader(w, r, http.MethodGet) {
			return
		}
		writeJSON(w, ghpull.GetStatus(ghRunner))
	})

	// Remembered repos first, then the account's most recently pushed ones.
	// Failing to reach GitHub still yields the remembered ones.
	mux.HandleFunc("/api/github/repos", func(w http.ResponseWriter, r *http.Request) {
		if !needsHeader(w, r, http.MethodGet) {
			return
		}
		repos := rememberedRepos()
		seen := map[string]bool{}
		for _, r := range repos {
			seen[strings.ToLower(r)] = true
		}
		if token, err := ghpull.AccountToken(ghRunner, r.URL.Query().Get("account")); err == nil {
			if recent, err := ghpull.ListRepos(ghRunner, token); err == nil {
				for _, name := range recent {
					if !seen[strings.ToLower(name)] {
						repos = append(repos, name)
					}
				}
			}
		}
		if repos == nil {
			repos = []string{}
		}
		writeJSON(w, repos)
	})

	mux.HandleFunc("/api/github/maps", func(w http.ResponseWriter, r *http.Request) {
		if !needsHeader(w, r, http.MethodGet) {
			return
		}
		repo, err := ghpull.ParseRepo(r.URL.Query().Get("repo"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		account := r.URL.Query().Get("account")
		token, err := ghpull.AccountToken(ghRunner, account)
		if err != nil {
			ghError(w, err, account)
			return
		}
		maps, err := ghpull.ListMaps(ghRunner, token, repo)
		if err != nil {
			ghError(w, err, account)
			return
		}
		if maps == nil {
			maps = []ghpull.MapInfo{}
		}
		writeJSON(w, map[string]any{"repo": repo, "maps": maps})
	})

	// Pull a map into the user cache dir, streaming progress as SSE. The
	// "done" event carries the mirror's path, which the client then opens.
	mux.HandleFunc("/api/github/open", func(w http.ResponseWriter, r *http.Request) {
		if !needsHeader(w, r, http.MethodPost) {
			return
		}
		q := r.URL.Query()
		repo, err := ghpull.ParseRepo(q.Get("repo"))
		num, nerr := strconv.Atoi(q.Get("map"))
		cache, cerr := userCacheDir()
		switch {
		case err != nil:
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		case nerr != nil || num <= 0:
			http.Error(w, "map must be a positive issue number", http.StatusBadRequest)
			return
		case cerr != nil:
			http.Error(w, cerr.Error(), http.StatusInternalServerError)
			return
		}
		out := ghpull.MirrorDir(cache, repo, q.Get("title"), num)
		streamSSE(w, func(progress func(done, total int)) (any, error) {
			if err := runOpen(ghpull.Options{Repo: repo, Map: num, Out: out, Account: q.Get("account")}, progress); err != nil {
				return nil, err
			}
			rememberRepo(repo)
			return map[string]string{"path": out}, nil
		})
	})

	// Delete a cached GitHub mirror and forget its recents entry. Unlike
	// /api/recents/remove this touches files, so it only ever accepts a mirror
	// inside the user cache dir.
	mux.HandleFunc("/api/recents/cleanup", func(w http.ResponseWriter, r *http.Request) {
		if !needsHeader(w, r, http.MethodPost) {
			return
		}
		path := r.URL.Query().Get("project")
		cache, err := userCacheDir()
		if err == nil {
			err = ghpull.RemoveMirror(cache, path)
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		removeRecent(path)
		writeJSON(w, loadRecents())
	})
}
