package ghpull

import (
	"bytes"
	"strings"
	"testing"
)

func TestProgressLines(t *testing.T) {
	var b bytes.Buffer
	p := NewProgress(&b, false)
	p.Update(0, 1)
	p.Update(1, 15)
	p.Update(5, 15)
	p.Update(6, 20)
	p.Close()
	if got, want := b.String(), "pulled 1/15 items\npulled 5/15 items\npulled 6/20 items\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestProgressBar(t *testing.T) {
	var b bytes.Buffer
	p := NewProgress(&b, true)
	p.Update(5, 15)
	if !strings.Contains(b.String(), "[=======>                ] 5/15 items") {
		t.Errorf("bar: %q", b.String())
	}
	b.Reset()
	p.Write([]byte("log line\n"))
	if got := b.String(); !strings.HasPrefix(got, "\r\x1b[2K") || !strings.Contains(got, "log line\n\r\x1b[2K[") {
		t.Errorf("output should scroll above the bar: %q", got)
	}
	b.Reset()
	p.Update(20, 20)
	if !strings.Contains(b.String(), "[========================] 20/20 items") {
		t.Errorf("full bar: %q", b.String())
	}
	b.Reset()
	p.Close()
	if b.String() != "\r\x1b[2K" {
		t.Errorf("close: %q", b.String())
	}
}
