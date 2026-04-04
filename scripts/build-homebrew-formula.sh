#!/bin/bash
set -euo pipefail

VERSION="${1:?version is required}"
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST_DIR="${DIST_DIR:-$ROOT_DIR/dist/release/$VERSION}"
RELEASE_REPO="${TMUX_GHOSTTY_RELEASE_REPO:-Woo-kk/tumx-ghostty}"
FORMULA_NAME="${TMUX_GHOSTTY_HOMEBREW_FORMULA:-tmux-ghostty}"
FORMULA_CLASS="${TMUX_GHOSTTY_HOMEBREW_CLASS:-}"
HOMEPAGE="${TMUX_GHOSTTY_HOMEBREW_HOMEPAGE:-https://github.com/$RELEASE_REPO}"
DESCRIPTION="${TMUX_GHOSTTY_HOMEBREW_DESC:-Shared terminal broker for Ghostty powered by tmux}"
ARCHIVE_NAME="tmux-ghostty_${VERSION}_darwin_universal.tar.gz"
ARCHIVE_PATH="$DIST_DIR/$ARCHIVE_NAME"
CHECKSUMS_PATH="$DIST_DIR/checksums.txt"
OUTPUT_DIR="$DIST_DIR/homebrew/Formula"
OUTPUT_PATH="$OUTPUT_DIR/$FORMULA_NAME.rb"

die() {
  echo "error: $*" >&2
  exit 1
}

formula_class_name() {
  local formula_name="$1"
  local class_name=""
  IFS='-' read -r -a parts <<<"$formula_name"
  for part in "${parts[@]}"; do
    [[ -n "$part" ]] || continue
    first_char="$(printf '%s' "$part" | cut -c1 | tr '[:lower:]' '[:upper:]')"
    rest_chars="$(printf '%s' "$part" | cut -c2-)"
    class_name+="${first_char}${rest_chars}"
  done
  printf '%s\n' "$class_name"
}

checksum_for() {
  local archive_name="$1"
  local checksums_file="$2"
  awk -v target="$archive_name" '
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

[[ -f "$ARCHIVE_PATH" ]] || die "archive not found: $ARCHIVE_PATH"
if [[ -f "$CHECKSUMS_PATH" ]]; then
  SHA256="$(checksum_for "$ARCHIVE_NAME" "$CHECKSUMS_PATH")"
fi
if [[ -z "${SHA256:-}" ]]; then
  SHA256="$(shasum -a 256 "$ARCHIVE_PATH" | awk '{print tolower($1)}')"
fi
[[ -n "$SHA256" ]] || die "failed to determine sha256 for $ARCHIVE_NAME"

if [[ -z "$FORMULA_CLASS" ]]; then
  FORMULA_CLASS="$(formula_class_name "$FORMULA_NAME")"
fi

mkdir -p "$OUTPUT_DIR"

cat >"$OUTPUT_PATH" <<EOF
class ${FORMULA_CLASS} < Formula
  desc "${DESCRIPTION}"
  homepage "${HOMEPAGE}"
  url "https://github.com/${RELEASE_REPO}/releases/download/${VERSION}/${ARCHIVE_NAME}"
  sha256 "${SHA256}"
  version "${VERSION#v}"

  def install
    bin.install "tmux-ghostty", "tmux-ghostty-broker"
  end

  test do
    assert_match "${VERSION}", shell_output("#{bin}/tmux-ghostty version")
  end
end
EOF

echo "generated Homebrew formula: $OUTPUT_PATH"
