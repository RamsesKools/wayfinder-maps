package ghpull

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

// IsTerminal reports whether w is a character device such as a terminal.
func IsTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// Progress renders pull progress. On a terminal it keeps a bar on the last
// line, and as an io.Writer it lets other output scroll above the bar. Off a
// terminal it prints one "pulled N/M items" line per update.
type Progress struct {
	mu          sync.Mutex
	w           io.Writer
	tty         bool
	done, total int
	drawn       bool
}

func NewProgress(w io.Writer, tty bool) *Progress { return &Progress{w: w, tty: tty} }

const barWidth = 24

func (p *Progress) bar() string {
	filled := 0
	if p.total > 0 {
		filled = barWidth * p.done / p.total
	}
	head := ""
	if filled < barWidth && filled > 0 {
		head, filled = ">", filled-1
	}
	pad := barWidth - filled - len(head)
	return fmt.Sprintf("[%s%s%s] %d/%d items", strings.Repeat("=", filled), head, strings.Repeat(" ", pad), p.done, p.total)
}

func (p *Progress) draw() {
	fmt.Fprint(p.w, "\r\x1b[2K"+p.bar())
	p.drawn = true
}

// Update records the latest (done, total); the total may move.
func (p *Progress) Update(done, total int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.done, p.total = done, total
	if p.tty {
		p.draw()
	} else if done > 0 && total > 1 {
		fmt.Fprintf(p.w, "pulled %d/%d items\n", done, total)
	}
}

// Write prints other output above the bar.
func (p *Progress) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.drawn {
		fmt.Fprint(p.w, "\r\x1b[2K")
	}
	n, err := p.w.Write(b)
	if p.drawn {
		p.draw()
	}
	return n, err
}

// Close removes the bar.
func (p *Progress) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.drawn {
		fmt.Fprint(p.w, "\r\x1b[2K")
		p.drawn = false
	}
}
