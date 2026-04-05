---
name: tmux-ghostty
description: Use tmux-ghostty to manage local Ghostty/tmux workspaces, panes, control handoff, approvals, and JumpServer-backed host attachment through the local CLI and broker.
---

# Tmux Ghostty

Use this runbook when an agent should operate the local `tmux-ghostty` CLI instead of manipulating Ghostty or tmux state ad hoc.

This file is intentionally vendor-neutral. It should be usable from any local coding agent that can read repository files and run local shell commands, including Claude Code.

## User Help

If the active agent is using this runbook and the user says `help`, return a short Chinese usage note.

Do not dump internal package names, JSON-RPC method names, or repository implementation details unless the user explicitly asks for them.

The help response should cover these points:

- 这个 skill 用来通过 `tmux-ghostty` 管理本地 Ghostty + tmux 工作区、pane 控制权、命令发送和审批，以及通过 JumpServer 挂接远端主机。
- Ghostty 只是可见终端界面，真正共享的文本状态在 tmux 里，所以用户和 agent 可以围绕同一 pane 协作。
- 多数操作会自动拉起本地 broker，不需要用户先做复杂初始化。
- 建议先用 `tmux-ghostty help` 看完整命令说明；如果要发命令，先 `command preview`，再决定是否 `command send`。
- 如果不知道 pane 或 action ID，先执行 `tmux-ghostty pane list` 或 `tmux-ghostty actions`。

The help response should also give a few short example requests such as:

- `启动 broker，然后创建一个 workspace`
- `列出当前 pane，并把 pane-1 聚焦`
- `把 pane-1 的控制权切给 agent，然后预览一条 kubectl 命令`
- `把 pane-1 挂到 test4 这台远端主机`
- `查看待审批动作，并批准 action-123`

Keep that response concise and task-oriented.

## Quick Start

1. Check the command reference:
   - `tmux-ghostty help`
2. Start or confirm the broker:
   - `tmux-ghostty up`
   - `tmux-ghostty status`
3. Create and inspect workspaces or panes:
   - `tmux-ghostty workspace create`
   - `tmux-ghostty pane list`
   - `tmux-ghostty pane snapshot <pane-id>`
4. Control and send commands safely:
   - `tmux-ghostty claim <pane-id> --actor agent`
   - `tmux-ghostty command preview <pane-id> <command...>`
   - `tmux-ghostty command send <pane-id> <command...>`
   - `tmux-ghostty actions`
   - `tmux-ghostty approve <action-id>`

## Workflow

### 1. Broker lifecycle

- Use `tmux-ghostty up` to ensure the local broker is running.
- Use `tmux-ghostty status` to inspect broker state as JSON.
- Use `tmux-ghostty down` when shutting down normally.
- Use `tmux-ghostty down --force` only when active workspaces must be torn down.

### 2. Workspace lifecycle

- Create a new shared terminal workspace with `tmux-ghostty workspace create`.
- Re-sync state from the current Ghostty/tmux view with `tmux-ghostty workspace reconcile`.
- Close a workspace and all of its panes with `tmux-ghostty workspace close <workspace-id>`.

### 3. Pane inspection and focus

- Discover pane IDs with `tmux-ghostty pane list`.
- Bring a pane to the front with `tmux-ghostty pane focus <pane-id>`.
- Read pane text and metadata with `tmux-ghostty pane snapshot <pane-id>`.

### 4. Remote host attach

- Use `tmux-ghostty host attach <pane-id> <query>` to attach a pane to a JumpServer-backed host session.
- Treat this as pane-level routing inside the shared terminal model, not as a separate remote shell protocol.
- If the user only needs remote shell automation and not the local shared Ghostty/tmux UI model, the standalone `tmux-jumpserver` skill may be a better fit.

### 5. Control handoff

- Use `tmux-ghostty claim <pane-id> --actor agent` when the agent should actively drive the pane.
- Use `tmux-ghostty claim <pane-id> --actor user` when the user should take over explicitly.
- Use `tmux-ghostty release <pane-id>` to clear control ownership.
- Use `tmux-ghostty observe <pane-id>` when the pane should remain read-only from the agent side.
- Use `tmux-ghostty interrupt <pane-id>` to stop the running foreground command.

### 6. Commands and approvals

- Prefer `tmux-ghostty command preview <pane-id> <command...>` before `command send` unless the risk is already obvious.
- Use `tmux-ghostty command send <pane-id> <command...>` to execute the command.
- If the broker marks the command as risky, inspect `tmux-ghostty actions`.
- Approve with `tmux-ghostty approve <action-id>` or reject with `tmux-ghostty deny <action-id>`.

### 7. Version, updates, and removal

- Use `tmux-ghostty version` to inspect version, build, and install metadata.
- Use `tmux-ghostty self-update --check` to check for a newer release.
- Use `tmux-ghostty self-update` or `tmux-ghostty self-update --version <tag>` for direct macOS installs.
- Use `tmux-ghostty uninstall` only for direct installs. Homebrew installs should use `brew upgrade` or `brew uninstall`.

## Operational Rules

- `tmux-ghostty help` is the authoritative command reference. Prefer it over restating the entire command tree from memory.
- Do not invent pane IDs, workspace IDs, or action IDs. Query them first with `pane list`, workspace commands, or `actions`.
- Most query-style commands print JSON. Parse the returned structure instead of scraping prose.
- Most operational subcommands auto-start the broker. Use `up` when you want explicit broker visibility or troubleshooting.
- Prefer the control-safe sequence `claim` -> `command preview` -> `command send` -> `actions` -> `approve` or `deny`.
- Do not bypass the approval flow for risky commands.
- Use `pane snapshot` before and after important transitions when you need verifiable terminal state.
- When a request is purely about CLI syntax or command discovery, answer from `tmux-ghostty help` behavior rather than internal package structure.
- If the agent runtime does not auto-load `skills/`, open this file manually or point the runtime's repo instructions to it.

## Example Requests

- `启动一个新 workspace，然后把 agent 控制权切到新 pane`
- `列出当前 panes，并告诉我哪个 pane 处于 observe 模式`
- `对 pane-2 预览 kubectl apply -f app.yaml，会不会触发审批？`
- `把 pane-3 挂到 test4，然后抓一份 snapshot 给我`
- `查看当前待审批动作，并拒绝最危险的那个`
- `检查当前安装方式，如果不是 Homebrew 就执行 self-update --check`
