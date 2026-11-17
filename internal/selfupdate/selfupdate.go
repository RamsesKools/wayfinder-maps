// Package selfupdate replaces an installed release of wayfinder-maps in place.
//
// It follows the install.sh contract: release archives named
// wayfinder-maps_<version>_<os>_<arch>.tar.gz plus checksums.txt, extracted into
// the install dir, which holds a .install.json receipt next to the binary.
package selfupdate

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// DefaultBaseURL is the GitHub repo releases are fetched from.
const DefaultBaseURL = "https://github.com/RamsesKools/wayfinder-maps"

// ReceiptName is the receipt file written next to the binary.
const ReceiptName = ".install.json"

const (
	maxChecksumBytes = 1 << 20
	maxFileBytes     = 512 << 20
)

var tagRe = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)$`)

// Receipt records what is installed; the schema is shared with install.sh.
type Receipt struct {
	Version     string `json:"version"`
	Commit      string `json:"commit,omitempty"`
	InstalledAt string `json:"installed_at"`
	Asset       string `json:"asset"`
	SHA256      string `json:"sha256"`
}

// RefusalError means this binary was not installed from a release, so it must not be replaced.
type RefusalError struct{ Reason string }

func (e *RefusalError) Error() string { return e.Reason }

// Updater holds everything an update needs, so tests can point it at a fake server.
type Updater struct {
	BaseURL string
	Client  *http.Client
	GOOS    string
	GOARCH  string
	// Current is the running version without the leading v, or "dev".
	Current string
	// Exe is the running binary with symlinks resolved.
	Exe string
	Now func() time.Time
}

// Status is the outcome of a check or an update.
type Status struct {
	Current  string
	Target   string
	Behind   bool
	Updated  bool
	Asset    string
	Receipt  Receipt
	NoChange bool
}

// Check reports what an update would do without touching disk. pin may be empty for latest.
func (u *Updater) Check(ctx context.Context, pin string) (Status, error) {
	if err := u.preflight(); err != nil {
		return Status{}, err
	}
	return u.plan(ctx, pin)
}

// Update installs pin (or latest) over the running install unless it is already current and force is unset.
func (u *Updater) Update(ctx context.Context, pin string, force bool) (Status, error) {
	if err := u.preflight(); err != nil {
		return Status{}, err
	}
	st, err := u.plan(ctx, pin)
	if err != nil {
		return st, err
	}
	if st.Target == "v"+u.Current && !force {
		st.NoChange = true
		return st, nil
	}
	rcpt, err := u.install(ctx, st.Target)
	if err != nil {
		return st, err
	}
	st.Updated, st.Receipt, st.Asset = true, rcpt, rcpt.Asset
	return st, nil
}

func (u *Updater) plan(ctx context.Context, pin string) (Status, error) {
	target, err := u.resolveTarget(ctx, pin)
	if err != nil {
		return Status{}, err
	}
	st := Status{Current: u.Current, Target: target, Asset: assetName(target, u.GOOS, u.GOARCH)}
	st.Behind = compare("v"+u.Current, target) < 0
	return st, nil
}

func (u *Updater) preflight() error {
	if u.Current == "" || u.Current == "dev" {
		return &RefusalError{"this is a dev build (from source or go install), not a release; install a release with install.sh to enable self-update"}
	}
	if u.GOOS != "darwin" && u.GOOS != "linux" {
		return &RefusalError{fmt.Sprintf("self-update does not support %s yet; download the archive from Releases", u.GOOS)}
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(u.Exe), ReceiptName))
	if errors.Is(err, os.ErrNotExist) {
		return &RefusalError{fmt.Sprintf("no %s next to %s, so it was not installed by install.sh (go install or a manual download?); reinstall with install.sh or update the way you installed it", ReceiptName, u.Exe)}
	}
	if err != nil {
		return err
	}
	var r Receipt
	if err := json.Unmarshal(data, &r); err != nil {
		return &RefusalError{fmt.Sprintf("%s next to %s is not valid JSON (%v); reinstall with install.sh", ReceiptName, u.Exe, err)}
	}
	return nil
}

// resolveTarget returns a v-prefixed tag: the pin, or latest via the /releases/latest redirect (no API, no token).
func (u *Updater) resolveTarget(ctx context.Context, pin string) (string, error) {
	if pin != "" {
		if !strings.HasPrefix(pin, "v") {
			pin = "v" + pin
		}
		if !tagRe.MatchString(pin) {
			return "", fmt.Errorf("invalid version %q, want vX.Y.Z", pin)
		}
		return pin, nil
	}
	client := *u.client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.BaseURL+"/releases/latest", nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("resolve latest release: %w", err)
	}
	defer resp.Body.Close()
	loc := resp.Header.Get("Location")
	if resp.StatusCode < 300 || resp.StatusCode >= 400 || loc == "" {
		return "", fmt.Errorf("resolve latest release: %s answered %s without a redirect", req.URL, resp.Status)
	}
	tag := path.Base(strings.TrimRight(strings.SplitN(loc, "?", 2)[0], "/"))
	if !tagRe.MatchString(tag) {
		return "", fmt.Errorf("resolve latest release: redirect %q does not end in a vX.Y.Z tag", loc)
	}
	return tag, nil
}

func (u *Updater) install(ctx context.Context, tag string) (Receipt, error) {
	dir := filepath.Dir(u.Exe)
	asset := assetName(tag, u.GOOS, u.GOARCH)
	// Staging inside the install dir keeps every rename on one filesystem.
	stage, err := os.MkdirTemp(dir, ".update-")
	if err != nil {
		return Receipt{}, fmt.Errorf("cannot write to %s: %w", dir, err)
	}
	defer os.RemoveAll(stage)

	want, err := u.expectedSum(ctx, tag, asset)
	if err != nil {
		return Receipt{}, err
	}
	archive := filepath.Join(stage, asset)
	got, err := u.download(ctx, u.releaseURL(tag, asset), archive)
	if err != nil {
		return Receipt{}, err
	}
	if got != want {
		return Receipt{}, fmt.Errorf("checksum mismatch for %s: checksums.txt says %s, downloaded file is %s", asset, want, got)
	}

	files := filepath.Join(stage, "files")
	names, err := extract(archive, files)
	if err != nil {
		return Receipt{}, fmt.Errorf("extract %s: %w", asset, err)
	}
	bin := filepath.Base(u.Exe)
	if !contains(names, bin) {
		return Receipt{}, fmt.Errorf("%s does not contain %s", asset, bin)
	}

	// Binary last, so a failure midway never leaves a new binary beside stale helpers.
	for _, n := range names {
		if n != bin {
			if err := os.Rename(filepath.Join(files, n), filepath.Join(dir, n)); err != nil {
				return Receipt{}, err
			}
		}
	}
	if err := os.Rename(filepath.Join(files, bin), u.Exe); err != nil {
		return Receipt{}, err
	}

	rcpt := Receipt{
		Version:     tag,
		InstalledAt: u.now().UTC().Format(time.RFC3339),
		Asset:       asset,
		SHA256:      got,
	}
	if err := writeReceipt(dir, stage, rcpt); err != nil {
		return Receipt{}, err
	}
	return rcpt, nil
}

func writeReceipt(dir, stage string, r Receipt) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(stage, ReceiptName)
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, ReceiptName))
}

func (u *Updater) expectedSum(ctx context.Context, tag, asset string) (string, error) {
	body, err := u.get(ctx, u.releaseURL(tag, "checksums.txt"))
	if err != nil {
		return "", err
	}
	defer body.Close()
	data, err := io.ReadAll(io.LimitReader(body, maxChecksumBytes))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		f := strings.Fields(line)
		if len(f) == 2 && strings.TrimPrefix(f[1], "*") == asset {
			return strings.ToLower(f[0]), nil
		}
	}
	return "", fmt.Errorf("checksums.txt for %s has no entry for %s (no build for %s/%s?)", tag, asset, u.GOOS, u.GOARCH)
}

// download writes url to dest and returns its sha256.
func (u *Updater) download(ctx context.Context, url, dest string) (string, error) {
	body, err := u.get(ctx, url)
	if err != nil {
		return "", err
	}
	defer body.Close()
	f, err := os.Create(dest)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(f, h), body); err != nil {
		return "", fmt.Errorf("download %s: %w", url, err)
	}
	return hex.EncodeToString(h.Sum(nil)), f.Close()
}

func (u *Updater) get(ctx context.Context, url string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := u.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("download %s: %s", url, resp.Status)
	}
	return resp.Body, nil
}

func (u *Updater) releaseURL(tag, name string) string {
	return u.BaseURL + "/releases/download/" + tag + "/" + name
}

func (u *Updater) client() *http.Client {
	if u.Client != nil {
		return u.Client
	}
	return &http.Client{Timeout: 5 * time.Minute}
}

func (u *Updater) now() time.Time {
	if u.Now != nil {
		return u.Now()
	}
	return time.Now()
}

func assetName(tag, goos, goarch string) string {
	return fmt.Sprintf("wayfinder-maps_%s_%s_%s.tar.gz", strings.TrimPrefix(tag, "v"), goos, goarch)
}

// extract unpacks a flat tar.gz into dir and returns the file names; anything nested or non-regular is rejected.
func extract(archive, dir string) ([]string, error) {
	f, err := os.Open(archive)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	if err := os.Mkdir(dir, 0o755); err != nil {
		return nil, err
	}
	var names []string
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return names, nil
		}
		if err != nil {
			return nil, err
		}
		name := strings.TrimPrefix(hdr.Name, "./")
		if hdr.Typeflag == tar.TypeDir && name == "" {
			continue
		}
		if hdr.Typeflag != tar.TypeReg {
			return nil, fmt.Errorf("unexpected entry %q (type %c)", hdr.Name, hdr.Typeflag)
		}
		if name == "" || name != filepath.Base(name) || name == ".." || name == ReceiptName {
			return nil, fmt.Errorf("unexpected path %q", hdr.Name)
		}
		out, err := os.OpenFile(filepath.Join(dir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, os.FileMode(hdr.Mode&0o755)|0o600)
		if err != nil {
			return nil, err
		}
		n, err := io.Copy(out, io.LimitReader(tr, maxFileBytes+1))
		if cerr := out.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return nil, err
		}
		if n > maxFileBytes {
			return nil, fmt.Errorf("%q is too large", hdr.Name)
		}
		names = append(names, name)
	}
}

// compare orders two vX.Y.Z tags; an unparsable side sorts as equal.
func compare(a, b string) int {
	pa, pb := tagRe.FindStringSubmatch(a), tagRe.FindStringSubmatch(b)
	if pa == nil || pb == nil {
		return 0
	}
	for i := 1; i <= 3; i++ {
		x, _ := strconv.Atoi(pa[i])
		y, _ := strconv.Atoi(pb[i])
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}
