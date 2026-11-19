package wayfinder

import (
	"time"

	"golang.org/x/sys/unix"
)

// Older kernels and some filesystems keep no birth time; mtime stands in.
func birthTime(path string) (time.Time, bool) {
	var st unix.Statx_t
	if unix.Statx(unix.AT_FDCWD, path, 0, unix.STATX_BTIME, &st) == nil && st.Mask&unix.STATX_BTIME != 0 {
		return time.Unix(st.Btime.Sec, int64(st.Btime.Nsec)), true
	}
	return modTime(path)
}
