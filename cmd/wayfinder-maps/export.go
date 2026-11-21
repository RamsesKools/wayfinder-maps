package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/RamsesKools/wayfinder-maps/internal/wayfinder"
)

const exportUsage = "usage: wayfinder-maps export <map-dir> <out-dir>"

// staticFlag is injected into the exported index.html; the viewer reads it to
// load graph.json instead of talking to a server.
const staticFlag = `<meta name="wayfinder-static" content="1">`

// exportCmd writes a map as a self-contained static site: the viewer assets plus
// a graph.json snapshot, servable by any plain file server.
func exportCmd(args []string, stdout, stderr io.Writer) int {
	var pos []string
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			fmt.Fprintf(stderr, "wayfinder-maps: unknown option %s\n%s\n", a, exportUsage)
			return 2
		}
		pos = append(pos, a)
	}
	if len(pos) != 2 {
		fmt.Fprintln(stderr, exportUsage)
		return 2
	}
	n, err := exportSite(pos[0], pos[1])
	if err != nil {
		fmt.Fprintf(stderr, "wayfinder-maps: %v\n", err)
		return 2
	}
	fmt.Fprintf(stdout, "exported %d tickets to %s\n", n, pos[1])
	return 0
}

// exportSite returns the number of tickets exported.
func exportSite(mapDir, outDir string) (int, error) {
	e, err := wayfinder.Load(mapDir)
	if err != nil {
		return 0, err
	}
	if inside(e.Dir, outDir) {
		return 0, fmt.Errorf("%s is inside the map %s", outDir, e.Dir)
	}
	g := buildGraph(e)

	web, err := embeddedWeb()
	if err != nil {
		return 0, err
	}
	err = fs.WalkDir(web, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		dst := filepath.Join(outDir, filepath.FromSlash(p))
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		b, err := fs.ReadFile(web, p)
		if err != nil {
			return err
		}
		if p == "index.html" {
			if b, err = flagStatic(b); err != nil {
				return err
			}
		}
		return os.WriteFile(dst, b, 0o644)
	})
	if err != nil {
		return 0, err
	}

	b, err := json.Marshal(g)
	if err != nil {
		return 0, err
	}
	return len(g.Nodes), os.WriteFile(filepath.Join(outDir, "graph.json"), b, 0o644)
}

func flagStatic(html []byte) ([]byte, error) {
	const head = "<head>"
	if strings.Count(string(html), head) != 1 {
		return nil, fmt.Errorf("index.html has no single %s to flag as static", head)
	}
	return []byte(strings.Replace(string(html), head, head+"\n"+staticFlag, 1)), nil
}

// inside reports whether path is dir itself or lies below it.
func inside(dir, path string) bool {
	d, err1 := filepath.Abs(dir)
	p, err2 := filepath.Abs(path)
	if err1 != nil || err2 != nil {
		return false
	}
	r, err := filepath.Rel(d, p)
	return err == nil && r != ".." && !strings.HasPrefix(r, ".."+string(filepath.Separator))
}
