package wayfinder

import (
	"os"
	"syscall"
	"time"
)

func birthTime(path string) (time.Time, bool) {
	fi, err := os.Stat(path)
	if err != nil {
		return time.Time{}, false
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return fi.ModTime(), true
	}
	return time.Unix(st.Birthtimespec.Unix()), true
}
