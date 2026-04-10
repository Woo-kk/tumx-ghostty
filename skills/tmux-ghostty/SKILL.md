---
name: tmux-ghostty
description: Use tmux-ghostty to manage local Ghostty/tmux workspaces, current-window bootstrap/adoption, pane control handoff, approvals, and provider-based remote attachment through the local CLI and broker.
---

# Tmux Ghostty

Use this runbook when an agent should operate the local `tmux-ghostty` CLI instead of manipulating Ghostty or tmux state ad hoc.

This file is intentionally vendor-neutral. It should be usable from any local coding agent that can read repository files and run local shell commands, including Claude Code.

## User Help

If the active agent is using this runbook and the user says `help`, return a short Chinese usage note.

Do not dump internal package names, JSON-RPC method names, or repository implementation details unless the user explicitly asks for them.

The help response should cover these points:

- 这个 skill 用来通过 `tmux-ghostty` 管理本地 Ghostty + tmux 工作区、pane 控制权、命令发送和审批，以及通过远端 provider 挂接远端主机。
- Ghostty 只是可见终端界面，真正共享的文本状态在 tmux 里，所以用户和 agent 可以围绕同一 pane 协作。
- 多数操作会自动拉起本地 broker，不需要用户先做复杂初始化。
- 如果用户要求“就在这个窗口里继续”，先走 `workspace inspect-current`；若已经在 tmux 中，再走 `workspace adopt-current`，若只是本地空闲 shell，则走 `workspace bootstrap-current`；这条路径失败时要明确报错，不能假装等价成功。
- 如果不知道 pane 或 action ID，先执行 `tmux-ghostty pane list` 或 `tmux-ghostty actions`。

The help response should also give a few short example requests such as:

- `接管当前窗口，然后在上方加一个新分屏`
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
3. If the user wants to stay in the current Ghostty window:
   - `tmux-ghostty workspace inspect-current`
   - `tmux-ghostty workspace bootstrap-current` or `tmux-ghostty workspace adopt-current`
   - `tmux-ghostty pane split <pane-id> --direction up|down|left|right`
4. Otherwise create a new workspace:
   - `tmux-ghostty workspace create`
5. Control and send commands safely:
   - `tmux-ghostty claim <pane-id> --actor agent`
   - `tmux-ghostty command preview <pane-id> <command...>`
   - `tmux-ghostty command send <pane-id> <command...>`
   - `tmux-ghostty actions`
   - `tmux-ghostty approve <action-id>`

## Capability Boundaries

- `workspace create`: stable path, opens a new Ghostty window.
- `workspace inspect-current`: read-only check of the currently focused Ghostty terminal.
- `workspace bootstrap-current`: formal path for turning a local idle shell in the current Ghostty terminal into a broker-managed tmux workspace without opening a new window.
- `workspace adopt-current`: formal path for taking over the current Ghostty window without opening a new one.
- `workspace reconcile`: restore already known workspaces. It does not import an unmanaged current window, and it does not silently reopen a replacement window for current-window workspaces.
- `pane split`: formal path for adding panes inside an existing workspace.
- `host attach`: formal remote-provider attach entrypoint. The current built-in provider is JumpServer.
- If the CLI still lacks a required capability, say so explicitly. Do not silently fall back to ad hoc `tmux` or `osascript` layout surgery.

## Workflow

### 1. Broker lifecycle

- Use `tmux-ghostty up` to ensure the local broker is running.
- Use `tmux-ghostty status` to inspect broker state as JSON.
- Use `tmux-ghostty down` when shutting down normally.
- Use `tmux-ghostty down --force` only when active workspaces must be torn down.

### 2. Current-window-first strategy

- If the user says `当前窗口`、`这个窗口`、`不要新开窗口`、`在这里分屏`, prefer this sequence:
  - `tmux-ghostty up`
  - `tmux-ghostty workspace inspect-current`
  - If `adoptable=true`, run `tmux-ghostty workspace adopt-current`
  - If `bootstrappable=true`, run `tmux-ghostty workspace bootstrap-current`
  - `tmux-ghostty pane split <pane-id> --direction ...`
- After `inspect-current`, tell the user whether the focused terminal is directly adoptable or needs bootstrap first.
- After `bootstrap-current` or `adopt-current`, explicitly state that this run is using the current Ghostty window.
- If `bootstrap-current` or `adopt-current` fails, explain the reason and stop the current-window flow.
- Only switch to `workspace create` if the user explicitly accepts opening a new Ghostty window.

### 3. Workspace lifecycle

- Create a new shared terminal workspace with `tmux-ghostty workspace create`.
- Re-sync state from the current Ghostty/tmux view with `tmux-ghostty workspace reconcile`.
- Close a workspace and all of its panes with `tmux-ghostty workspace close <workspace-id>`.
- Clear tmux-backed screen state for every pane in a workspace with `tmux-ghostty workspace clear <workspace-id>`.
- Permanently remove a workspace and all of its panes from broker state with `tmux-ghostty workspace delete <workspace-id>`.

### 4. Pane inspection, split, and focus

- Discover pane IDs with `tmux-ghostty pane list`.
- Bring a pane to the front with `tmux-ghostty pane focus <pane-id>`.
- Clear a pane's tmux-backed screen state with `tmux-ghostty pane clear <pane-id>`.
- Permanently remove a pane from broker state with `tmux-ghostty pane delete <pane-id>`.
- Read pane text and metadata with `tmux-ghostty pane snapshot <pane-id>`.
- Expand an existing workspace in-place with `tmux-ghostty pane split <pane-id> --direction up|down|left|right`.
- Use `--claim agent` or `--claim user` on `pane split` when ownership should be set immediately.
- When reporting results, describe the pane topology and which host each pane is attached to.

### 5. Remote attach

- Use `tmux-ghostty host attach <pane-id> <query>` to attach a pane through the configured remote provider.
- Treat this as pane-level routing inside the shared terminal model, not as a separate remote shell protocol.
- If the current provider is JumpServer and the user only needs remote shell automation rather than the shared Ghostty/tmux UI model, the standalone `tmux-jumpserver` skill may be a better fit.

### 6. Control handoff

- Use `tmux-ghostty claim <pane-id> --actor agent` when the agent should actively drive the pane.
- Use `tmux-ghostty claim <pane-id> --actor user` when the user should take over explicitly.
- Use `tmux-ghostty release <pane-id>` to clear control ownership.
- Use `tmux-ghostty observe <pane-id>` when the pane should remain read-only from the agent side.
- Use `tmux-ghostty interrupt <pane-id>` to stop the running foreground command.

### 7. Commands, stages, and approvals

- Prefer `tmux-ghostty command preview <pane-id> <command...>` before `command send` unless the risk is already obvious.
- Use `tmux-ghostty command send <pane-id> <command...>` to execute the command.
- If the broker marks the command as risky, inspect `tmux-ghostty actions`.
- Approve with `tmux-ghostty approve <action-id>` or reject with `tmux-ghostty deny <action-id>`.
- Use the `stage` field in `pane snapshot` or `status` output to decide whether the pane is in `shell`, `menu`, `target_search`, `selection`, `auth_prompt`, or `remote_shell`.
- If a pending approval already exists, stop and resolve it before sending more commands into the same pane.

### 8. Provider menu runbook

- For the built-in JumpServer provider, treat `Opt>` as `menu`.
- Treat `[Host]>` or `Search:` as `target_search`.
- Treat `ID>` or parsed account rows as `selection`.
- Treat a normal prompt after attach as `remote_shell`.
- In provider navigation stages, single-token inputs such as `2801`, `/2801`, `1`, or `h` should usually classify as `nav`. If they still come back as `risky`, treat that as a product gap, not as a normal success path.
- If `host attach` fails and the pane is still in `menu`, `target_search`, or `selection`, inspect `pane snapshot`, read the current `stage`, and continue the menu flow with `command send` instead of abandoning the attach.
- Use `actions` only when the CLI still requests approval. Do not normalize that as the preferred path for provider navigation menus.

### 9. Version, updates, and removal

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
- For current-window requests, prefer `inspect-current` -> (`bootstrap-current` or `adopt-current`) -> `pane split` before considering `workspace create`.
- When the user explicitly requires the current window, do not present `workspace create` as an equivalent success path.
- Do not bypass the approval flow for risky commands.
- Use `pane snapshot` before and after important transitions when you need verifiable terminal state.
- Always tell the user whether the result is using the current window or a newly created workspace.
- If the user later agrees to open a new workspace, state clearly that this is a strategy change rather than a silent downgrade.
- If the CLI still lacks a needed capability, say it is a product limitation instead of pretending the fallback is equivalent.
- When a request is purely about CLI syntax or command discovery, answer from `tmux-ghostty help` behavior rather than internal package structure.
- If the agent runtime does not auto-load `skills/`, open this file manually or point the runtime's repo instructions to it.

## Downgrade Matrix

- `workspace bootstrap-current` or `workspace adopt-current` fails:
  Explain the reason.
  Stop the current-window flow unless the user explicitly asks to switch to `workspace create`.
- `host attach` fails while `stage` is `menu`, `target_search`, or `selection`:
  Read `pane snapshot`, inspect `stage`, and continue with the menu flow instead of improvising unrelated shell commands or giving up.
- Query or account selection is ambiguous:
  Report the concrete candidates instead of claiming attach succeeded.
- A pending approval already exists:
  Resolve the action first and do not continue injecting commands into that pane.

## Example Requests

- `接管这个窗口，然后在上方再加一个 pane`
- `如果当前窗口不能接管，就明确告诉我原因，然后新开 workspace`
- `列出当前 panes，并告诉我每个 pane 现在连的是哪台机器`
- `对 pane-2 预览 kubectl apply -f app.yaml，会不会触发审批？`
- `把 pane-3 挂到 test4，然后抓一份带 stage 的 snapshot 给我`
- `现在 pane 里还是 Opt>，继续把它推进到远端 shell`
