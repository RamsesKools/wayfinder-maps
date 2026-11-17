//go:build !(linux && cgo)

// The helper only has a job on Linux, where the main binary stays pure Go and
// the webkit linkage lives here. Other platforms embed their webview directly.
package main

import (
	"fmt"
	"os"

	"github.com/RamsesKools/wayfinder-maps/internal/version"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println(version.String("wayfinder-maps-webview"))
		return
	}
	fmt.Fprintln(os.Stderr, "wayfinder-maps-webview: Linux native-window helper; nothing to do on this platform")
	os.Exit(2)
}
