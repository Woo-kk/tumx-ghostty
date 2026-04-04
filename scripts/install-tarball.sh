#!/bin/bash
set -euo pipefail

RELEASE_REPO="${TMUX_GHOSTTY_RELEASE_REPO:-Woo-kk/tumx-ghostty}"
INSTALL_DIR="${TMUX_GHOSTTY_INSTALL_DIR:-/usr/local/bin}"
VERSION=""
ARCHIVE_PATH=""
CHECKSUMS_PATH=""

usage() {
  cat <<'EOF'
Usage:
  scripts/install-tarball.sh --version <tag> [--install-dir <dir>]
  scripts/install-tarball.sh --archive <path> [--checksums <path>] [--install-dir <dir>]

Examples:
  ./scripts/install-tarball.sh --version v0.1.0
  ./scripts/install-tarball.sh --archive dist/release/v0.1.0/tmux-ghostty_v0.1.0_darwin_universal.tar.gz
EOF
}

die() {
  echo "error: $*" >&2
  exit 1
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version)
      VERSION="${2:-}"
      shift 2
      ;;
    --archive)
      ARCHIVE_PATH="${2:-}"
      shift 2
      ;;
    --checksums)
      CHECKSUMS_PATH="${2:-}"
      shift 2
      ;;
    --install-dir)
      INSTALL_DIR="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      die "unknown argument: $1"
      ;;
  esac
done

if [[ -n "$VERSION" && -n "$ARCHIVE_PATH" ]]; then
  die "--version and --archive are mutually exclusive"
fi
if [[ -z "$VERSION" && -z "$ARCHIVE_PATH" ]]; then
  die "either --version or --archive is required"
fi

WORK_DIR="$(mktemp -d)"
trap 'rm -rf "$WORK_DIR"' EXIT

compute_checksum() {
  local path="$1"
  shasum -a 256 "$path" | awk '{print tolower($1)}'
}

expected_checksum_from_file() {
  local target_name="$1"
  local checksums_file="$2"
  awk -v target="$target_name" '
    {
      name=$2
      sub(/^\*/, "", name)
      if (name == target) {
        print tolower($1)
        exit
      }
    }
  ' "$checksums_file"
}

download_release_archive() {
  local version="$1"
  local archive_name="tmux-ghostty_${version}_darwin_universal.tar.gz"
  local checksums_name="checksums.txt"
  ARCHIVE_PATH="$WORK_DIR/$archive_name"
  CHECKSUMS_PATH="$WORK_DIR/$checksums_name"
  local base_url="https://github.com/${RELEASE_REPO}/releases/download/${version}"
  curl -fsSL -o "$ARCHIVE_PATH" "$base_url/$archive_name"
  curl -fsSL -o "$CHECKSUMS_PATH" "$base_url/$checksums_name"
}

verify_archive_checksum() {
  local archive_path="$1"
  local checksums_path="$2"
  [[ -f "$checksums_path" ]] || return 0

  local archive_name
  archive_name="$(basename "$archive_path")"
  local expected
  expected="$(expected_checksum_from_file "$archive_name" "$checksums_path")"
  [[ -n "$expected" ]] || die "checksums file does not contain $archive_name"

  local actual
  actual="$(compute_checksum "$archive_path")"
  if [[ "$actual" != "$expected" ]]; then
    die "checksum mismatch for $archive_name: got $actual want $expected"
  fi
}

install_binary() {
  local src="$1"
  local dst_dir="$2"
  mkdir -p "$dst_dir"
  if [[ -w "$dst_dir" ]]; then
    install -m 0755 "$src" "$dst_dir/"
  else
    sudo install -m 0755 "$src" "$dst_dir/"
  fi
}

if [[ -n "$VERSION" ]]; then
  download_release_archive "$VERSION"
fi

[[ -f "$ARCHIVE_PATH" ]] || die "archive not found: $ARCHIVE_PATH"
if [[ -z "$CHECKSUMS_PATH" ]]; then
  candidate="$(dirname "$ARCHIVE_PATH")/checksums.txt"
  if [[ -f "$candidate" ]]; then
    CHECKSUMS_PATH="$candidate"
  fi
fi
if [[ -n "$CHECKSUMS_PATH" ]]; then
  verify_archive_checksum "$ARCHIVE_PATH" "$CHECKSUMS_PATH"
fi

tar -C "$WORK_DIR" -xzf "$ARCHIVE_PATH"
[[ -f "$WORK_DIR/tmux-ghostty" ]] || die "archive does not contain tmux-ghostty"
[[ -f "$WORK_DIR/tmux-ghostty-broker" ]] || die "archive does not contain tmux-ghostty-broker"

install_binary "$WORK_DIR/tmux-ghostty" "$INSTALL_DIR"
install_binary "$WORK_DIR/tmux-ghostty-broker" "$INSTALL_DIR"

echo "installed:"
echo "  $INSTALL_DIR/tmux-ghostty"
echo "  $INSTALL_DIR/tmux-ghostty-broker"
