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

## Build and Test

Use the following commands to run tests and build the binaries.

```bash
go test ./...
go build ./cmd/tmux-ghostty
go build ./cmd/tmux-ghostty-broker
```

## CLI

```text
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

`tmux-ghostty help` prints the full command list. `tmux-ghostty -h` and `tmux-ghostty --help` are equivalent aliases.

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
- `TMUX_GHOSTTY_IDLE_TIMEOUT`
- `TMUX_GHOSTTY_JUMP_PROFILE`
- `TMUX_GHOSTTY_JUMP_RUNNER`
- `TMUX_GHOSTTY_REMOTE_TMUX_SESSION`

## Notes

- Ghostty is treated as the visible frontend only. `tmux` carries the actual text/data flow, so snapshot text comes from local `tmux`, not from Ghostty content APIs.
- The JumpServer adapter assumes the existing local runner at `/Users/guyuanshun/.codex/skills/tmux-jumpserver/scripts/run_jump_profile.sh` unless overridden by `TMUX_GHOSTTY_JUMP_RUNNER`.
- The current test suite uses real local `tmux` and fake Ghostty orchestration so it does not spawn GUI windows during automated runs.
