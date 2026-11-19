package wayfinder

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// CreatedTimes reports when each ticket was created: its `created:` key, else
// the commit that first added its file, else the file's birth time. Tickets
// with none of these are left out.
func (e *Effort) CreatedTimes() map[int]time.Time {
	out := map[int]time.Time{}
	var added map[string]time.Time
	for _, t := range e.Tickets {
		if !t.Created.IsZero() {
			out[t.Num] = t.Created
			continue
		}
		if added == nil {
			added = gitAdded(filepath.Dir(t.Path))
		}
		if at, ok := added[filepath.Base(t.Path)]; ok {
			out[t.Num] = at
		} else if at, ok := birthTime(t.Path); ok {
			out[t.Num] = at
		}
	}
	return out
}

// gitAdded maps each file name in dir to the time of the oldest commit that
// added it. Outside a git repo, or without git, it is empty.
func gitAdded(dir string) map[string]time.Time {
	out := map[string]time.Time{}
	cmd := exec.Command("git", "-C", dir, "log", "--no-renames", "--diff-filter=A", "--format=@%ct", "--name-only", "--relative", "--", ".")
	b, err := cmd.Output()
	if err != nil {
		return out
	}
	var at time.Time
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case line == "":
		case strings.HasPrefix(line, "@"):
			sec, _ := strconv.ParseInt(line[1:], 10, 64)
			at = time.Unix(sec, 0)
		case !strings.Contains(line, "/"):
			out[line] = at // log runs newest first, so the oldest add wins
		}
	}
	return out
}

func modTime(path string) (time.Time, bool) {
	fi, err := os.Stat(path)
	if err != nil {
		return time.Time{}, false
	}
	return fi.ModTime(), true
}
