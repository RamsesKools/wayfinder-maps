#!/usr/bin/env bash
# Tests for scripts/install.sh against a fake release served from local files.
set -uo pipefail

here=$(cd "$(dirname "$0")" && pwd)
script="$here/../install.sh"
root=$(mktemp -d)
trap 'rm -rf "$root"; [ -n "${srv_pid:-}" ] && kill "$srv_pid" 2>/dev/null' EXIT

case "$(uname -s)" in Darwin) os=darwin ;; *) os=linux ;; esac
case "$(uname -m)" in arm64|aarch64) arch=arm64 ;; *) arch=amd64 ;; esac

sha() { if command -v sha256sum >/dev/null; then sha256sum "$1"; else shasum -a 256 "$1"; fi | cut -d' ' -f1; }

make_release() { # tag
  local tag=$1 v=${1#v} d="$root/releases/download/$1" pkg="$root/pkg-$1"
  mkdir -p "$d" "$pkg"
  printf '#!/bin/sh\necho wayfinder-maps %s\n' "$tag" >"$pkg/wayfinder-maps"
  printf '#!/bin/sh\necho helper\n' >"$pkg/wayfinder-maps-webview"
  chmod +x "$pkg"/*
  tar -czf "$d/wayfinder-maps_${v}_${os}_${arch}.tar.gz" -C "$pkg" .
  printf '%s  %s\n' "$(sha "$d/wayfinder-maps_${v}_${os}_${arch}.tar.gz")" "wayfinder-maps_${v}_${os}_${arch}.tar.gz" >"$d/checksums.txt"
}
make_release v1.0.0
make_release v1.1.0

export WAYFINDER_RELEASES_URL="file://$root/releases"
export WAYFINDER_API_URL="file://$root/api"
fails=0
ok() { printf 'ok   %s\n' "$1"; }
bad() { printf 'FAIL %s\n' "$1"; fails=$((fails + 1)); }
check() { local name=$1; shift; if "$@" >/dev/null 2>&1; then ok "$name"; else bad "$name"; fi; }
lacks() { ! grep -q "$1" <<<"$2"; }
run() { P="$root/p$RANDOM"; B="$root/b$RANDOM"; out=$(bash "$script" --prefix "$P" --bin-dir "$B" "$@" 2>&1); rc=$?; }
again() { out=$(bash "$script" --prefix "$P" --bin-dir "$B" "$@" 2>&1); rc=$?; }

run --version v1.0.0
check "install exits 0" test $rc -eq 0
check "reports checksum OK" grep -q "Checksum OK" <<<"$out"
check "hints at install-skill" grep -q "wayfinder-maps install-skill" <<<"$out"
check "binary runs through symlink" test "$("$B/wayfinder-maps")" = "wayfinder-maps v1.0.0"
check "receipt has version" grep -q '"version": "v1.0.0"' "$P/.install.json"
check "receipt has asset" grep -q "\"asset\": \"wayfinder-maps_1.0.0_${os}_${arch}.tar.gz\"" "$P/.install.json"
check "receipt has sha256" grep -q "\"sha256\": \"$(sha "$root/releases/download/v1.0.0/wayfinder-maps_1.0.0_${os}_${arch}.tar.gz")\"" "$P/.install.json"
check "receipt is valid json" python3 -c "import json,sys; json.load(open(sys.argv[1]))" "$P/.install.json"
check "warns when bin dir not on PATH" grep -q "not on your PATH" <<<"$out"
check "helper linked" test -L "$B/wayfinder-maps-webview"

again --version v1.0.0
check "rerun is a no-op" grep -q "already installed" <<<"$out"
check "rerun did not download" lacks Downloading "$out"

again --version 1.0.0 --force
check "--force reinstalls (v prefix optional)" grep -q "Installed wayfinder-maps v1.0.0" <<<"$out"

again --version v1.1.0
check "newer version replaces old" test "$("$B/wayfinder-maps")" = "wayfinder-maps v1.1.0"
check "no leftover staging dirs" lacks "\.tmp\." "$(ls -A "$(dirname "$P")")"

PATH="$B:$PATH" again --version v1.1.0 --force
check "no PATH warning when bin dir on PATH" lacks "not on your PATH" "$out"

# Checksum mismatch must fail and leave the install untouched.
make_release v2.0.0
sed -i.bak "s/^[0-9a-f]*/$(printf '0%.0s' {1..64})/" "$root/releases/download/v2.0.0/checksums.txt"
again --version v2.0.0
check "checksum mismatch fails" test $rc -ne 0
check "mismatch is reported" grep -q "checksum mismatch" <<<"$out"
check "mismatch leaves old install" test "$("$B/wayfinder-maps")" = "wayfinder-maps v1.1.0"

# Refuses to clobber a foreign directory.
mkdir -p "$root/foreign" && touch "$root/foreign/keep"
out=$(bash "$script" --prefix "$root/foreign" --bin-dir "$B" --version v1.0.0 2>&1); rc=$?
check "refuses non-empty foreign prefix" test $rc -ne 0
check "foreign prefix untouched" test -f "$root/foreign/keep"

# Refuses to replace a real file in the bin dir.
run --version v1.0.0
rm "$B/wayfinder-maps" && echo real >"$B/wayfinder-maps"
again --version v1.0.0 --force
check "refuses to overwrite a regular file in bin dir" test $rc -ne 0

out=$(bash "$script" --version nonsense 2>&1); rc=$?
check "rejects invalid version" test $rc -ne 0
out=$(bash "$script" --bogus 2>&1); rc=$?
check "rejects unknown flag" test $rc -ne 0

# "latest" follows the /releases/latest redirect.
if command -v python3 >/dev/null; then
  python3 -u - <<PY >"$root/port" &
import http.server, functools
class H(http.server.SimpleHTTPRequestHandler):
    def do_HEAD(self): self.do_GET()
    def do_GET(self):
        if self.path == "/releases/latest":
            self.send_response(302); self.send_header("Location", "/releases/tag/v1.1.0"); self.end_headers()
        elif self.path == "/releases/tag/v1.1.0":
            self.send_response(200); self.end_headers()
        else:
            super().do_GET()
    def log_message(self, *a): pass
s = http.server.HTTPServer(("127.0.0.1", 0), functools.partial(H, directory="$root"))
print(s.server_address[1], flush=True)
s.serve_forever()
PY
  srv_pid=$!
  for _ in 1 2 3 4 5 6 7 8 9 10; do [ -s "$root/port" ] && break; sleep 0.3; done
  port=$(cat "$root/port")
  WAYFINDER_RELEASES_URL="http://127.0.0.1:$port/releases" run
  check "latest resolves via redirect" grep -q "Latest release: v1.1.0" <<<"$out"
  check "latest installs that tag" test "$("$B/wayfinder-maps")" = "wayfinder-maps v1.1.0"
fi

echo
if [ "$fails" -eq 0 ]; then echo "all passed"; else echo "$fails failed"; exit 1; fi
