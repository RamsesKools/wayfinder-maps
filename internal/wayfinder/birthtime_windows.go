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
	d, ok := fi.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		return fi.ModTime(), true
	}
	return time.Unix(0, d.CreationTime.Nanoseconds()), true
}
