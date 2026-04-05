# tmux-ghostty

[English](./README.md) | [中文](./README.zh-CN.md)

`tmux-ghostty` is a local macOS tool that uses Ghostty only as the visible terminal UI, while `tmux` handles the real text/data path and remains the shared text source that both the user and an agent can observe and control.

## What v1 Implements

- `tmux-ghostty` CLI plus auto-started `tmux-ghostty-broker`
- Unix domain socket JSON-RPC 2.0
- workspace / pane / action state persistence
- Ghostty AppleScript orchestration for windows, tabs, splits, focus, text input, and key events
- one logical pane = one local `tmux` session
- pane snapshot capture from local `tmux`
- explicit `claim` / `release` / `interrupt` / `observe`
- command risk classification with approval flow
- JumpServer attach adapter that reuses the local `tmux-jumpserver` runner
- broker idle auto-exit logic

## Repository Layout

```text
cmd/
  tmux-ghostty/
  tmux-ghostty-broker/
internal/
  app/
  broker/
  control/
  execx/
  ghostty/
  jump/
  logx/
  model/
  observe/
  risk/
  rpc/
  store/
  tmux/
```

## Agent Runbook

This repository includes a repo-local agent runbook at `skills/tmux-ghostty/SKILL.md`. It is written in a vendor-neutral format so different local coding agents can reuse the same workflow. `CLAUDE.md` points Claude Code to the same file.

## Build and Test

Use the following commands to run tests and build the binaries.

```bash
go test ./...
go build ./cmd/tmux-ghostty
go build ./cmd/tmux-ghostty-broker
```

This repository currently produces 2 binaries:

- `tmux-ghostty`
- `tmux-ghostty-broker`

For release builds and local packaging:

```bash
make release-binaries VERSION=v0.1.0
make package VERSION=v0.1.0
make install-tarball VERSION=v0.1.0
make homebrew-formula VERSION=v0.1.0
make publish-homebrew-tap VERSION=v0.1.0
```

`make package` creates:

- `dist/release/<version>/tmux-ghostty_<version>_darwin_universal.tar.gz`
- `dist/release/<version>/tmux-ghostty_<version>_darwin_universal.pkg`
- `dist/release/<version>/checksums.txt`
- `dist/release/<version>/homebrew/Formula/tmux-ghostty.rb`

## CLI

```text
tmux-ghostty version
tmux-ghostty self-update --check
tmux-ghostty self-update --version <tag>
tmux-ghostty uninstall

tmux-ghostty up
tmux-ghostty down --force
tmux-ghostty status

tmux-ghostty workspace create
tmux-ghostty workspace reconcile
tmux-ghostty workspace close <workspace-id>

tmux-ghostty pane list
tmux-ghostty pane focus <pane-id>
tmux-ghostty pane snapshot <pane-id>

tmux-ghostty host attach <pane-id> <query>

tmux-ghostty claim <pane-id> --actor agent
tmux-ghostty claim <pane-id> --actor user
tmux-ghostty release <pane-id>
tmux-ghostty interrupt <pane-id>
tmux-ghostty observe <pane-id>

tmux-ghostty actions
tmux-ghostty approve <action-id>
tmux-ghostty deny <action-id>

tmux-ghostty command preview <pane-id> <command...>
tmux-ghostty command send <pane-id> <command...>
tmux-ghostty help
```

`tmux-ghostty help` is the authoritative detailed command reference. The README keeps the high-level command tree; use the CLI for the per-command descriptions. `tmux-ghostty -h` and `tmux-ghostty --help` are equivalent aliases.

`tmux-ghostty version` prints build metadata. `tmux-ghostty self-update` installs a GitHub Release package over the current installation. `tmux-ghostty uninstall` removes both installed binaries and the current user's runtime data.

## Install, Update, Uninstall

`tmux-ghostty` currently supports 4 install paths:

- `Homebrew`: best for end users who want `brew upgrade` and `brew uninstall`
- macOS `.pkg`: best for direct GUI/package installation from GitHub Releases
- `tar.gz`: best for script-based installation without Homebrew
- source build: best for local development or custom packaging

### Install with Homebrew

End-user install flow:

```bash
brew tap Woo-kk/tmux-ghostty
brew install tmux-ghostty
brew upgrade tmux-ghostty
brew uninstall tmux-ghostty
```

When installed by Homebrew, `tmux-ghostty self-update` and `tmux-ghostty uninstall` are intentionally blocked. Users should use `brew upgrade tmux-ghostty` and `brew uninstall tmux-ghostty` instead.

### Install with macOS pkg

Download `tmux-ghostty_<version>_darwin_universal.pkg` from GitHub Releases, or install it directly:

```bash
sudo /usr/sbin/installer -pkg tmux-ghostty_<version>_darwin_universal.pkg -target /
```

The packaged macOS installer places both binaries in:

```text
/usr/local/bin/tmux-ghostty
/usr/local/bin/tmux-ghostty-broker
```

Typical lifecycle:

```bash
tmux-ghostty version
tmux-ghostty self-update --check
tmux-ghostty self-update
sudo tmux-ghostty uninstall
```

### Install from tarball

You can install from the release tarball without building or using a `.pkg`:

```bash
./scripts/install-tarball.sh --version v0.1.0
```

Or install from a locally built archive:

```bash
./scripts/install-tarball.sh --archive dist/release/v0.1.0/tmux-ghostty_v0.1.0_darwin_universal.tar.gz
```

### Build from source

For local development or custom packaging:

```bash
go build ./cmd/tmux-ghostty
go build ./cmd/tmux-ghostty-broker
```

If you want to install the locally built binaries into `/usr/local/bin`:

```bash
sudo install -m 0755 ./tmux-ghostty /usr/local/bin/tmux-ghostty
sudo install -m 0755 ./tmux-ghostty-broker /usr/local/bin/tmux-ghostty-broker
```

`tmux-ghostty uninstall` removes:

- `/usr/local/bin/tmux-ghostty`
- `/usr/local/bin/tmux-ghostty-broker`
- `~/Library/Application Support/tmux-ghostty/`

## Command Risk Levels

`command preview` classifies commands into 3 levels:

- `read`: read-only commands, sent directly without approval. Examples: `pwd`, `ls`, `cat`, `rg`, `ps`, `kubectl get ns`, `git status -sb`
- `nav`: shell/navigation setup commands, also sent directly without approval. Examples: `cd /tmp`, `export KUBECONFIG=...`, `source env.sh`
- `risky`: commands that may mutate state or that the classifier cannot safely recognize. These require `tmux-ghostty approve <action-id>` before `command send` can continue. Examples: `rm -rf ...`, `kubectl apply -f ...`, `kubectl delete ...`, `helm upgrade ...`, `echo hi > file.txt`

Current classification is prefix-based and intentionally conservative:

- shell combiners and redirections such as `&&`, `||`, `;`, `|`, `>`, `>>`, `<`, `<<`, command substitution, or multi-line input are always `risky`
- unknown commands also fall back to `risky`
- JumpServer menu-style inputs such as `/1201` or `1` are currently not special-cased, so they are also treated as `risky`

## Runtime Paths

By default the broker uses:

```text
~/Library/Application Support/tmux-ghostty/
```

with:

```text
broker.sock
broker.pid
state.json
actions.json
broker.log
```

Useful environment variables:

- `TMUX_GHOSTTY_HOME`
- `TMUX_GHOSTTY_BROKER_BIN`
- `TMUX_GHOSTTY_RELEASE_REPO`
- `TMUX_GHOSTTY_IDLE_TIMEOUT`
- `TMUX_GHOSTTY_JUMP_PROFILE`
- `TMUX_GHOSTTY_JUMP_RUNNER`
- `TMUX_GHOSTTY_REMOTE_TMUX_SESSION`

## GitHub Release Automation

This repository includes `.github/workflows/release.yml`. Pushing a tag like `v0.1.0` triggers the macOS release pipeline:

```bash
git tag v0.1.0
git push origin v0.1.0
```

The workflow builds both binaries for `darwin/amd64` and `darwin/arm64`, merges universal binaries, creates the `.pkg`, notarizes it when Apple signing secrets are configured, generates a Homebrew formula file, and uploads the `.pkg`, `.tar.gz`, `checksums.txt`, and formula file to GitHub Release.

If `HOMEBREW_TAP_TOKEN` is configured, the same workflow also commits the generated formula into `Woo-kk/homebrew-tmux-ghostty` automatically.

## Notes

- Ghostty is treated as the visible frontend only. `tmux` carries the actual text/data flow, so snapshot text comes from local `tmux`, not from Ghostty content APIs.
- The JumpServer adapter assumes the existing local runner at `/Users/guyuanshun/.codex/skills/tmux-jumpserver/scripts/run_jump_profile.sh` unless overridden by `TMUX_GHOSTTY_JUMP_RUNNER`.
- The current test suite uses real local `tmux` and fake Ghostty orchestration so it does not spawn GUI windows during automated runs.
