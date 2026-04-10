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
- provider-based remote attach layer, with a built-in JumpServer provider that reuses the local `tmux-jumpserver` runner
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
  remote/
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
tmux-ghostty workspace inspect-current
tmux-ghostty workspace bootstrap-current
tmux-ghostty workspace adopt-current
tmux-ghostty workspace reconcile
tmux-ghostty workspace close <workspace-id>
tmux-ghostty workspace clear <workspace-id>
tmux-ghostty workspace delete <workspace-id>

tmux-ghostty pane list
tmux-ghostty pane focus <pane-id>
tmux-ghostty pane clear <pane-id>
tmux-ghostty pane delete <pane-id>
tmux-ghostty pane snapshot <pane-id>
tmux-ghostty pane split <pane-id> --direction up|down|left|right [--claim agent|user]

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

`tmux-ghostty workspace inspect-current` reports whether the currently focused Ghostty terminal is directly adoptable or first needs bootstrapping. If the terminal is already inside a local tmux pane, `tmux-ghostty workspace adopt-current` keeps working in the current Ghostty window instead of opening a new one. If the terminal is a local idle shell outside tmux, `tmux-ghostty workspace bootstrap-current` starts a broker-owned tmux session in place and adopts it into a current-window workspace. In current-window mode, the CLI does not silently launch or rebuild a replacement Ghostty window; if the front window, focused terminal, or tmux context is unsuitable, it fails explicitly. `tmux-ghostty pane split` is the formal way to grow an existing workspace in-place.

`tmux-ghostty pane clear` and `tmux-ghostty workspace clear` clear the pane screen state that tmux-ghostty snapshots, and clear tmux scrollback for the affected pane or workspace. `tmux-ghostty pane delete` and `tmux-ghostty workspace delete` permanently remove broker state for the selected pane or workspace and terminate any broker-owned local tmux sessions that belong to it.

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
- when a pane is in a remote provider navigation stage such as `menu`, `target_search`, or `selection`, inputs such as `2801`, `/2801`, `1`, or `h` are treated as `nav` instead of `risky`

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
- `TMUX_GHOSTTY_REMOTE_PROVIDER`
- `TMUX_GHOSTTY_REMOTE_TMUX_SESSION`

JumpServer-provider-specific variables:
`TMUX_GHOSTTY_JUMP_PROFILE`, `TMUX_GHOSTTY_JUMP_RUNNER`

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
- `host attach` is wired through `internal/remote`, so additional remote providers such as direct SSH can be added without rewriting the broker/workspace core.
- The built-in `jumpserver` provider now materializes its bundled runner and expect helper under the tmux-ghostty runtime directory automatically; `TMUX_GHOSTTY_JUMP_RUNNER` still overrides that default when needed.
- `host attach` succeeds once the remote shell is ready. Remote tmux attach is best-effort and surfaces its outcome through `remote_tmux_status` and `remote_tmux_detail`.
- The current test suite uses real local `tmux` and fake Ghostty orchestration so it does not spawn GUI windows during automated runs.
