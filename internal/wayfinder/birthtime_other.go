//go:build !darwin && !linux && !windows

package wayfinder

import "time"

func birthTime(path string) (time.Time, bool) { return modTime(path) }
