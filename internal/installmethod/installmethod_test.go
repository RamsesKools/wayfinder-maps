package installmethod

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RamsesKools/wayfinder-maps/internal/selfupdate"
)

func TestDetect(t *testing.T) {
	receipt := t.TempDir()
	os.WriteFile(filepath.Join(receipt, selfupdate.ReceiptName), []byte(`{}`), 0o644)

	tests := []struct {
		name string
		env  Env
		want Method
	}{
		{"dev build", Env{Version: "dev", Exe: "/x/wayfinder-maps"}, Dev},
		{"empty version is dev", Env{Exe: "/x/wayfinder-maps"}, Dev},
		{"install.sh receipt", Env{Version: "1.0.0", Exe: filepath.Join(receipt, "wayfinder-maps")}, Script},
		{"homebrew cellar", Env{Version: "1.0.0", Exe: "/opt/homebrew/Cellar/wayfinder-maps/1.0.0/bin/wayfinder-maps"}, Homebrew},
		{"homebrew caskroom", Env{Version: "1.0.0", Exe: "/opt/homebrew/Caskroom/wayfinder-maps/1.0.0/wayfinder-maps"}, Homebrew},
		{"linuxbrew", Env{Version: "1.0.0", Exe: "/home/linuxbrew/.linuxbrew/Cellar/wayfinder-maps/1.0.0/bin/wayfinder-maps"}, Homebrew},
		{"go install GOBIN", Env{Version: "1.0.0", Exe: "/gobin/wayfinder-maps", GOBIN: "/gobin"}, GoInstall},
		{"go install GOPATH", Env{Version: "1.0.0", Exe: "/gp/bin/wayfinder-maps", GOPATH: "/gp"}, GoInstall},
		{"go install default", Env{Version: "1.0.0", Exe: "/home/me/go/bin/wayfinder-maps", Home: "/home/me"}, GoInstall},
		{"manual", Env{Version: "1.0.0", Exe: "/usr/local/bin/wayfinder-maps", Home: "/home/me"}, Manual},
	}
	for _, tt := range tests {
		if got := Detect(tt.env); got != tt.want {
			t.Errorf("%s: Detect = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestHint(t *testing.T) {
	want := map[Method]string{
		Script:    "wayfinder-maps self-update",
		Homebrew:  "brew upgrade wayfinder-maps",
		GoInstall: "go install github.com/RamsesKools/wayfinder-maps/cmd/wayfinder-maps@latest",
		Manual:    "https://github.com/RamsesKools/wayfinder-maps/releases/latest",
		Dev:       "",
	}
	for m, w := range want {
		if got := Hint(m); got != w {
			t.Errorf("Hint(%q) = %q, want %q", m, got, w)
		}
	}
	if !strings.Contains(Hint(Manual), "releases") {
		t.Error("manual hint must link the releases page")
	}
}
