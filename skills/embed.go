// Package skills embeds the skills shipped with the binary.
package skills

import "embed"

// FS holds every skill dir, including the dot-prefixed .optional/ one.
//
//go:embed all:*
var FS embed.FS
