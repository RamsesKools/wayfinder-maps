package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestPullCmdArgs(t *testing.T) {
	failGH := func(string, ...string) ([]byte, error) { return nil, errors.New("boom") }
	for _, tc := range []struct {
		name string
		args []string
		code int
		msg  string
	}{
		{"no args", nil, 2, "usage:"},
		{"missing map", []string{"acme/widgets"}, 2, "usage:"},
		{"bad map", []string{"acme/widgets", "x"}, 2, "positive integer"},
		{"unknown flag", []string{"--nope"}, 2, "unknown option --nope"},
		{"refresh without dir", []string{"--refresh"}, 2, "--refresh needs"},
		{"refresh without source", []string{"--refresh", t.TempDir()}, 2, ".source.json"},
		{"gh failure", []string{"acme/widgets", "1", t.TempDir() + "/o"}, 1, "boom"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, errw bytes.Buffer
			if code := pullCmd(tc.args, failGH, &out, &errw); code != tc.code || !strings.Contains(errw.String(), tc.msg) {
				t.Errorf("code %d, stderr %q; want %d containing %q", code, errw.String(), tc.code, tc.msg)
			}
		})
	}
}
