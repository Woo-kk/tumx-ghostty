# Homebrew Packaging

This repository can generate a Homebrew formula file from a tagged release tarball:

```bash
make release-binaries VERSION=v0.1.0
make homebrew-formula VERSION=v0.1.0
```

The generated formula is written to:

```text
dist/release/<version>/homebrew/Formula/tmux-ghostty.rb
```

## What still needs to exist outside this repository

To publish via Homebrew, you still need a separate tap repository. The common layout is:

```text
Woo-kk/homebrew-tmux-ghostty
  Formula/
    tmux-ghostty.rb
```

Recommended external setup:

1. Use the public tap repo `Woo-kk/homebrew-tmux-ghostty`.
2. Keep the tap repo default branch as `main`.
3. Make sure GitHub Releases for this repo publish `tmux-ghostty_<version>_darwin_universal.tar.gz` and `checksums.txt`.
4. Add a fine-grained PAT with `contents:write` permission to the tap repository.
5. Store that PAT in this repository as the Actions secret `HOMEBREW_TAP_TOKEN`.

Optional workflow variables:

- `HOMEBREW_TAP_BRANCH=main`
- `HOMEBREW_TAP_FORMULA_PATH=Formula/tmux-ghostty.rb`
- `TMUX_GHOSTTY_HOMEBREW_FORMULA=tmux-ghostty`
- `TMUX_GHOSTTY_HOMEBREW_CLASS=TmuxGhostty`
- `TMUX_GHOSTTY_HOMEBREW_HOMEPAGE=https://github.com/Woo-kk/tumx-ghostty`
- `TMUX_GHOSTTY_HOMEBREW_DESC=Shared terminal broker for Ghostty powered by tmux`

After that, the existing tag-based release workflow in this repository can publish the generated formula into the tap repo automatically.

## Manual tap publishing

If you want to push the formula manually after generating it:

```bash
HOMEBREW_TAP_REPO=Woo-kk/homebrew-tmux-ghostty \
HOMEBREW_TAP_TOKEN=<token> \
make publish-homebrew-tap VERSION=v0.1.0
```

## Typical Homebrew usage

After the tap repo exists:

```bash
brew tap Woo-kk/tmux-ghostty
brew install tmux-ghostty
brew upgrade tmux-ghostty
brew uninstall tmux-ghostty
```

## Notes

- This project is macOS-only, but the formula currently installs from the universal macOS tarball rather than from source.
- `tmux-ghostty self-update` is intentionally blocked for Homebrew-managed installs. Users should use `brew upgrade`.
