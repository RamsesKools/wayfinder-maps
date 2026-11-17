package version

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolve(t *testing.T) {
	for _, tc := range []struct{ name, injected, build, want string }{
		{"injected wins", "1.2.3", "v9.9.9", "1.2.3"},
		{"module version from go install", "", "v0.4.1", "0.4.1"},
		{"devel build", "", "(devel)", "dev"},
		{"no build info", "", "", "dev"},
		{"pseudo-version is not a release", "", "v0.4.2-0.20260101000000-abcdef123456+dirty", "dev"},
	} {
		if got := resolve(tc.injected, tc.build); got != tc.want {
			t.Errorf("%s: resolve(%q, %q) = %q, want %q", tc.name, tc.injected, tc.build, got, tc.want)
		}
	}
}

func TestString(t *testing.T) {
	if got, want := String("tool"), "tool "+Get(); got != want {
		t.Errorf("String = %q, want %q", got, want)
	}
}

func TestBinariesReportVersion(t *testing.T) {
	if testing.Short() {
		t.Skip("builds binaries")
	}
	tools := map[string]string{
		"wayfinder-maps":         "../../cmd/wayfinder-maps",
		"wayfinder-maps-webview": "../../cmd/wayfinder-maps-webview",
	}
	for name, pkg := range tools {
		for _, tc := range []struct{ ldflags, want string }{
			{"", name + " dev"},
			{"-X github.com/RamsesKools/wayfinder-maps/internal/version.Version=1.2.3", name + " 1.2.3"},
		} {
			bin := filepath.Join(t.TempDir(), name)
			build := exec.Command("go", "build", "-ldflags", tc.ldflags, "-o", bin, pkg)
			if out, err := build.CombinedOutput(); err != nil {
				t.Fatalf("build %s: %v\n%s", name, err, out)
			}
			out, err := exec.Command(bin, "--version").Output()
			if err != nil {
				t.Fatalf("%s --version: %v", name, err)
			}
			if got := strings.TrimSpace(string(out)); got != tc.want {
				t.Errorf("%s --version = %q, want %q", name, got, tc.want)
			}
		}
	}
}
