#!/usr/bin/env bash
# Install wayfinder-maps from a GitHub release.
#   curl -fsSL https://raw.githubusercontent.com/RamsesKools/wayfinder-maps/main/scripts/install.sh | bash
#   curl -fsSL .../install.sh | bash -s -- --version v0.5.1
# Env: WAYFINDER_RELEASES_URL and WAYFINDER_API_URL override the GitHub endpoints (tests, mirrors).
set -euo pipefail

REPO="RamsesKools/wayfinder-maps"
RELEASES_URL="${WAYFINDER_RELEASES_URL:-https://github.com/$REPO/releases}"
API_URL="${WAYFINDER_API_URL:-https://api.github.com/repos/$REPO}"

usage() {
  cat <<EOF
Usage: install.sh [--version <tag>] [--prefix <dir>] [--bin-dir <dir>] [--force]

  --version <tag>  release to install (default: latest), e.g. v0.5.1
  --prefix <dir>   where the archive is extracted (default: \${XDG_DATA_HOME:-~/.local/share}/wayfinder-maps)
  --bin-dir <dir>  where the binary is symlinked (default: ~/.local/bin)
  --force          reinstall even if that version is already installed
EOF
}

say() { printf '%s\n' "$*"; }
die() { printf 'error: %s\n' "$*" >&2; exit 1; }

sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | cut -d' ' -f1
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | cut -d' ' -f1
  else
    die "need sha256sum or shasum to verify the download"
  fi
}

resolve_latest() {
  local url
  url=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "$RELEASES_URL/latest") \
    || die "could not reach $RELEASES_URL/latest"
  case "$url" in
    */releases/tag/*|*/tag/*) printf '%s\n' "${url##*/}" ;;
    *) die "no releases found at $RELEASES_URL" ;;
  esac
}

receipt_field() {
  [ -f "$1" ] || return 0
  sed -n "s/.*\"$2\": *\"\([^\"]*\)\".*/\1/p;/\"$2\"/q" "$1"
}

link_bin() {
  local src=$1 dest=$2
  if [ -e "$dest" ] && [ ! -L "$dest" ]; then
    die "$dest exists and is not a symlink; move it away or pick another --bin-dir"
  fi
  ln -sfn "$src" "$dest"
}

work=""
stage=""

main() {
  local version="" prefix="" bin_dir="" force=0
  while [ $# -gt 0 ]; do
    case "$1" in
      --version) [ $# -ge 2 ] || die "--version needs a value"; version=$2; shift 2 ;;
      --prefix) [ $# -ge 2 ] || die "--prefix needs a value"; prefix=$2; shift 2 ;;
      --bin-dir) [ $# -ge 2 ] || die "--bin-dir needs a value"; bin_dir=$2; shift 2 ;;
      --force) force=1; shift ;;
      -h|--help) usage; return 0 ;;
      *) usage >&2; die "unknown argument: $1" ;;
    esac
  done

  prefix="${prefix:-${XDG_DATA_HOME:-$HOME/.local/share}/wayfinder-maps}"
  bin_dir="${bin_dir:-$HOME/.local/bin}"
  prefix="${prefix%/}"
  bin_dir="${bin_dir%/}"
  [ -n "$prefix" ] || die "--prefix must not be /"

  local os arch
  case "$(uname -s)" in
    Darwin) os=darwin ;;
    Linux) os=linux ;;
    MINGW*|MSYS*|CYGWIN*) die "Windows is not supported by this script yet, see https://github.com/$REPO/issues/24" ;;
    *) die "unsupported OS: $(uname -s)" ;;
  esac
  case "$(uname -m)" in
    x86_64|amd64) arch=amd64 ;;
    arm64|aarch64) arch=arm64 ;;
    *) die "unsupported architecture: $(uname -m)" ;;
  esac

  command -v curl >/dev/null 2>&1 || die "curl is required"
  command -v tar >/dev/null 2>&1 || die "tar is required"

  if [ -z "$version" ]; then
    version=$(resolve_latest)
    say "Latest release: $version"
  fi
  version="v${version#v}"
  [[ "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]] || die "invalid version: $version"

  local receipt="$prefix/.install.json" installed=""
  installed=$(receipt_field "$receipt" version)
  if [ "$installed" = "$version" ] && [ -x "$prefix/wayfinder-maps" ] && [ "$force" -eq 0 ]; then
    say "wayfinder-maps $version is already installed in $prefix (use --force to reinstall)"
    return 0
  fi

  if [ -e "$prefix" ] && [ ! -f "$receipt" ] && [ -n "$(ls -A "$prefix" 2>/dev/null)" ]; then
    die "$prefix is not empty and has no .install.json; refusing to replace it (pick another --prefix)"
  fi

  local asset="wayfinder-maps_${version#v}_${os}_${arch}.tar.gz"
  work=$(mktemp -d "${TMPDIR:-/tmp}/wayfinder-install.XXXXXX")
  trap 'rm -rf "$work" ${stage:+"$stage"}' EXIT

  say "Downloading $asset"
  curl -fsSL -o "$work/$asset" "$RELEASES_URL/download/$version/$asset" \
    || die "download failed: $RELEASES_URL/download/$version/$asset"
  curl -fsSL -o "$work/checksums.txt" "$RELEASES_URL/download/$version/checksums.txt" \
    || die "download failed: $RELEASES_URL/download/$version/checksums.txt"

  local want got
  want=$(awk -v f="$asset" '$2 == f || $2 == "*" f { print $1; exit }' "$work/checksums.txt")
  [ -n "$want" ] || die "$asset not listed in checksums.txt"
  got=$(sha256_of "$work/$asset")
  if [ "$want" != "$got" ]; then
    die "checksum mismatch for $asset (expected $want, got $got); nothing was installed"
  fi
  say "Checksum OK ($got)"

  mkdir -p "$(dirname "$prefix")" "$bin_dir"
  stage=$(mktemp -d "$prefix.tmp.XXXXXX")
  tar -xzf "$work/$asset" -C "$stage"
  [ -x "$stage/wayfinder-maps" ] || die "archive has no wayfinder-maps binary"
  chmod 755 "$stage"
  if [ "$os" = darwin ]; then
    xattr -dr com.apple.quarantine "$stage" 2>/dev/null || true
  fi

  local commit=""
  commit=$(curl -fsSL -H 'Accept: application/vnd.github.sha' "$API_URL/commits/$version" 2>/dev/null || true)
  [[ "$commit" =~ ^[0-9a-f]{40}$ ]] || commit=""

  cat >"$stage/.install.json" <<EOF
{
  "version": "$version",
  "commit": "$commit",
  "installed_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "asset": "$asset",
  "sha256": "$got"
}
EOF

  # Swap whole directories with rename so a running process keeps its open files.
  if [ -e "$prefix" ]; then
    mv "$prefix" "$stage.old"
    mv "$stage" "$prefix"
    rm -rf "$stage.old"
  else
    mv "$stage" "$prefix"
  fi
  stage=""

  link_bin "$prefix/wayfinder-maps" "$bin_dir/wayfinder-maps"
  if [ -e "$prefix/wayfinder-maps-webview" ]; then
    link_bin "$prefix/wayfinder-maps-webview" "$bin_dir/wayfinder-maps-webview"
  fi

  say "Installed wayfinder-maps $version"
  say "  files:   $prefix"
  say "  binary:  $bin_dir/wayfinder-maps"
  say "  receipt: $receipt"
  say "Next: wayfinder-maps install-skill   # install the matching skills"

  case ":$PATH:" in
    *":$bin_dir:"*) ;;
    *) say "warning: $bin_dir is not on your PATH; add it, e.g. export PATH=\"$bin_dir:\$PATH\"" ;;
  esac
}

main "$@"
