#!/bin/bash
set -euo pipefail

VERSION="${1:?version is required}"
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST_DIR="${DIST_DIR:-$ROOT_DIR/dist/release/$VERSION}"
FORMULA_NAME="${TMUX_GHOSTTY_HOMEBREW_FORMULA:-tmux-ghostty}"
SOURCE_FORMULA="${HOMEBREW_SOURCE_FORMULA:-$DIST_DIR/homebrew/Formula/$FORMULA_NAME.rb}"
TAP_REPO="${HOMEBREW_TAP_REPO:-${TMUX_GHOSTTY_HOMEBREW_TAP_REPO:-}}"
TAP_TOKEN="${HOMEBREW_TAP_TOKEN:-${TMUX_GHOSTTY_HOMEBREW_TAP_TOKEN:-}}"
TAP_BRANCH="${HOMEBREW_TAP_BRANCH:-${TMUX_GHOSTTY_HOMEBREW_TAP_BRANCH:-main}}"
TAP_FORMULA_PATH="${HOMEBREW_TAP_FORMULA_PATH:-${TMUX_GHOSTTY_HOMEBREW_TAP_FORMULA_PATH:-Formula/${FORMULA_NAME}.rb}}"
TAP_REMOTE="${HOMEBREW_TAP_REMOTE:-https://github.com/${TAP_REPO}.git}"
GIT_NAME="${HOMEBREW_TAP_GIT_NAME:-github-actions[bot]}"
GIT_EMAIL="${HOMEBREW_TAP_GIT_EMAIL:-41898282+github-actions[bot]@users.noreply.github.com}"
COMMIT_MESSAGE="${HOMEBREW_TAP_COMMIT_MESSAGE:-brew: update ${FORMULA_NAME} to ${VERSION}}"
TMP_DIR=""

die() {
  echo "error: $*" >&2
  exit 1
}

cleanup() {
  if [[ -n "$TMP_DIR" && -d "$TMP_DIR" ]]; then
    rm -rf "$TMP_DIR"
  fi
}

git_clone() {
  local auth_header="$1"
  if [[ -n "$auth_header" ]]; then
    git -c "http.https://github.com/.extraheader=AUTHORIZATION: basic ${auth_header}" \
      clone --depth 1 --branch "$TAP_BRANCH" "$TAP_REMOTE" "$TMP_DIR"
    return
  fi
  git clone --depth 1 --branch "$TAP_BRANCH" "$TAP_REMOTE" "$TMP_DIR"
}

git_push() {
  local auth_header="$1"
  if [[ -n "$auth_header" ]]; then
    git -C "$TMP_DIR" -c "http.https://github.com/.extraheader=AUTHORIZATION: basic ${auth_header}" \
      push origin "$TAP_BRANCH"
    return
  fi
  git -C "$TMP_DIR" push origin "$TAP_BRANCH"
}

trap cleanup EXIT

[[ -f "$SOURCE_FORMULA" ]] || die "formula not found: $SOURCE_FORMULA"
[[ -n "$TAP_REPO" ]] || die "HOMEBREW_TAP_REPO (or TMUX_GHOSTTY_HOMEBREW_TAP_REPO) is required"

AUTH_HEADER=""
if [[ "$TAP_REMOTE" == https://github.com/* ]]; then
  [[ -n "$TAP_TOKEN" ]] || die "HOMEBREW_TAP_TOKEN is required for GitHub tap publishing"
  AUTH_HEADER="$(printf 'x-access-token:%s' "$TAP_TOKEN" | base64 | tr -d '\n')"
fi

TMP_DIR="$(mktemp -d)"
git_clone "$AUTH_HEADER"

mkdir -p "$TMP_DIR/$(dirname "$TAP_FORMULA_PATH")"
if [[ -f "$TMP_DIR/$TAP_FORMULA_PATH" ]] && cmp -s "$SOURCE_FORMULA" "$TMP_DIR/$TAP_FORMULA_PATH"; then
  echo "tap formula is already up to date: $TAP_REPO:$TAP_FORMULA_PATH"
  exit 0
fi

install -m 0644 "$SOURCE_FORMULA" "$TMP_DIR/$TAP_FORMULA_PATH"

git -C "$TMP_DIR" config user.name "$GIT_NAME"
git -C "$TMP_DIR" config user.email "$GIT_EMAIL"
git -C "$TMP_DIR" add "$TAP_FORMULA_PATH"

if git -C "$TMP_DIR" diff --cached --quiet; then
  echo "no Homebrew tap changes to publish"
  exit 0
fi

git -C "$TMP_DIR" commit -m "$COMMIT_MESSAGE"
git_push "$AUTH_HEADER"

echo "published Homebrew formula to $TAP_REPO:$TAP_FORMULA_PATH"
