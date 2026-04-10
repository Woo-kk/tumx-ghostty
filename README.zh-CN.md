# tmux-ghostty

[English](./README.md) | [中文](./README.zh-CN.md)

`tmux-ghostty` 是一个本地 macOS 工具：Ghostty 只负责用户可见的终端 UI，`tmux` 负责真实的文本/数据传递，并作为用户与 agent 共同读取和控制的共享文本事实源。

## v1 包含内容

- `tmux-ghostty` CLI，以及自动拉起的 `tmux-ghostty-broker`
- 基于 Unix domain socket 的 JSON-RPC 2.0
- workspace / pane / action 状态持久化
- 通过 Ghostty AppleScript 编排 window、tab、split、focus、文本输入和按键事件
- 一个逻辑 pane 对应一个本地 `tmux` session
- 从本地 `tmux` 抓取 pane 快照
- 显式控制权切换：`claim` / `release` / `interrupt` / `observe`
- 命令风险分类和审批流
- 基于 provider 的远端挂接层，当前内置 JumpServer provider，并复用本地 `tmux-jumpserver` runner
- broker 空闲自动退出逻辑

## 仓库结构

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

这个仓库内置了一份 repo-local agent runbook，位于 `skills/tmux-ghostty/SKILL.md`。它使用中性的工作流写法，便于不同的本地 coding agent 复用；根目录下的 `CLAUDE.md` 也会把 Claude Code 指向同一份文件。

## 构建与测试

使用下面的命令运行测试并构建二进制。

```bash
go test ./...
go build ./cmd/tmux-ghostty
go build ./cmd/tmux-ghostty-broker
```

当前仓库会产出 2 个二进制：

- `tmux-ghostty`
- `tmux-ghostty-broker`

如果要做 release 构建或本地打包，可以执行：

```bash
make release-binaries VERSION=v0.1.0
make package VERSION=v0.1.0
make install-tarball VERSION=v0.1.0
make homebrew-formula VERSION=v0.1.0
make publish-homebrew-tap VERSION=v0.1.0
```

`make package` 会生成：

- `dist/release/<version>/tmux-ghostty_<version>_darwin_universal.tar.gz`
- `dist/release/<version>/tmux-ghostty_<version>_darwin_universal.pkg`
- `dist/release/<version>/checksums.txt`
- `dist/release/<version>/homebrew/Formula/tmux-ghostty.rb`

## 命令行

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

`tmux-ghostty help` 是权威的详细命令参考。README 只保留高层级命令树，具体命令说明请以 CLI 输出为准。`tmux-ghostty -h` 和 `tmux-ghostty --help` 是等价别名。

`tmux-ghostty workspace inspect-current` 会报告当前焦点 Ghostty terminal 是否可被接管，还是需要先 bootstrap。若当前 terminal 只是本地空闲 shell 且尚未进入 tmux，可用 `tmux-ghostty workspace bootstrap-current` 原地拉起 broker 管理的 tmux session 并接管；若已经在本地 tmux pane 中，则用 `tmux-ghostty workspace adopt-current` 在当前 Ghostty 窗口内继续工作，而不是新开窗口。当前窗口模式下，CLI 不会再隐式拉起替代用的 Ghostty window；如果前台窗口、焦点 terminal 或 tmux 上下文不满足要求，它会明确失败。`tmux-ghostty pane split` 是在已有 workspace 内正式扩 pane 的入口。

`tmux-ghostty pane clear` 和 `tmux-ghostty workspace clear` 会清理 tmux-ghostty 记录的 pane 屏幕快照，并清空对应 pane 或 workspace 的 tmux scrollback。`tmux-ghostty pane delete` 和 `tmux-ghostty workspace delete` 会永久删除对应 pane 或 workspace 的 broker 状态，并终止其归 broker 所有的本地 tmux session。

`tmux-ghostty version` 会输出构建元信息。`tmux-ghostty self-update` 会用 GitHub Release 中的安装包覆盖当前安装。`tmux-ghostty uninstall` 会同时删除两个已安装二进制和当前用户的运行时数据。

## 安装、升级、卸载

`tmux-ghostty` 目前支持 4 种安装方式：

- `Homebrew`：最适合终端用户，后续可以直接 `brew upgrade` 和 `brew uninstall`
- macOS `.pkg`：最适合从 GitHub Releases 直接下载安装包
- `tar.gz`：最适合不用 Homebrew 的脚本化安装
- 源码构建：最适合本地开发或自定义打包

### 用 Homebrew 安装

终端用户安装方式：

```bash
brew tap Woo-kk/tmux-ghostty
brew install tmux-ghostty
brew upgrade tmux-ghostty
brew uninstall tmux-ghostty
```

如果是通过 Homebrew 安装，`tmux-ghostty self-update` 和 `tmux-ghostty uninstall` 会被主动拦住。此时应使用 `brew upgrade tmux-ghostty` 和 `brew uninstall tmux-ghostty`。

### 用 macOS pkg 安装

从 GitHub Releases 下载 `tmux-ghostty_<version>_darwin_universal.pkg`，或者直接执行：

```bash
sudo /usr/sbin/installer -pkg tmux-ghostty_<version>_darwin_universal.pkg -target /
```

macOS 安装包会把两个二进制放到：

```text
/usr/local/bin/tmux-ghostty
/usr/local/bin/tmux-ghostty-broker
```

典型使用流程：

```bash
tmux-ghostty version
tmux-ghostty self-update --check
tmux-ghostty self-update
sudo tmux-ghostty uninstall
```

### 用 tar.gz 安装

你可以不走 `.pkg`，直接通过 release tarball 安装：

```bash
./scripts/install-tarball.sh --version v0.1.0
```

也可以直接安装本地构建出的归档包：

```bash
./scripts/install-tarball.sh --archive dist/release/v0.1.0/tmux-ghostty_v0.1.0_darwin_universal.tar.gz
```

### 用源码构建安装

适合本地开发或自定义打包：

```bash
go build ./cmd/tmux-ghostty
go build ./cmd/tmux-ghostty-broker
```

如果你要把本地构建出的二进制安装到 `/usr/local/bin`：

```bash
sudo install -m 0755 ./tmux-ghostty /usr/local/bin/tmux-ghostty
sudo install -m 0755 ./tmux-ghostty-broker /usr/local/bin/tmux-ghostty-broker
```

`tmux-ghostty uninstall` 会删除：

- `/usr/local/bin/tmux-ghostty`
- `/usr/local/bin/tmux-ghostty-broker`
- `~/Library/Application Support/tmux-ghostty/`

## 命令分级

`command preview` 会把命令分成 3 个级别：

- `read`：只读命令，直接发送，不需要审批。示例：`pwd`、`ls`、`cat`、`rg`、`ps`、`kubectl get ns`、`git status -sb`
- `nav`：导航或环境准备类命令，也会直接发送，不需要审批。示例：`cd /tmp`、`export KUBECONFIG=...`、`source env.sh`
- `risky`：可能修改状态，或者分类器无法安全识别的命令。此类命令需要先执行 `tmux-ghostty approve <action-id>`，然后才能继续 `command send`。示例：`rm -rf ...`、`kubectl apply -f ...`、`kubectl delete ...`、`helm upgrade ...`、`echo hi > file.txt`

当前分级规则是基于前缀匹配的保守策略：

- 包含 `&&`、`||`、`;`、`|`、`>`、`>>`、`<`、`<<`、命令替换或多行输入的命令，都会直接归类为 `risky`
- 未识别的命令默认也归类为 `risky`
- 当 pane 处于远端 provider 的导航阶段，例如 `menu`、`target_search`、`selection` 时，`2801`、`/2801`、`1`、`h` 这类输入会按 `nav` 处理，不再默认走 `risky`

## 运行时路径

默认情况下，broker 使用：

```text
~/Library/Application Support/tmux-ghostty/
```

目录内容如下：

```text
broker.sock
broker.pid
state.json
actions.json
broker.log
```

常用环境变量：

- `TMUX_GHOSTTY_HOME`
- `TMUX_GHOSTTY_BROKER_BIN`
- `TMUX_GHOSTTY_RELEASE_REPO`
- `TMUX_GHOSTTY_IDLE_TIMEOUT`
- `TMUX_GHOSTTY_REMOTE_PROVIDER`
- `TMUX_GHOSTTY_REMOTE_TMUX_SESSION`

JumpServer provider 专用变量：
`TMUX_GHOSTTY_JUMP_PROFILE`、`TMUX_GHOSTTY_JUMP_RUNNER`

## GitHub Release 自动发布

仓库已经包含 `.github/workflows/release.yml`。推送类似 `v0.1.0` 的 tag 后，会触发 macOS release 流程：

```bash
git tag v0.1.0
git push origin v0.1.0
```

这个 workflow 会分别构建 `darwin/amd64` 和 `darwin/arm64` 的两个二进制，合并 universal binary，生成 `.pkg`，在 Apple 签名 secrets 配好时执行公证，额外生成 Homebrew formula 文件，并把 `.pkg`、`.tar.gz`、`checksums.txt` 和 formula 文件一起上传到 GitHub Release。

如果你已经配置了 `HOMEBREW_TAP_TOKEN`，同一个 workflow 还会自动把 formula 提交到 `Woo-kk/homebrew-tmux-ghostty`。

## 说明

- Ghostty 只被当作可见前端使用。真正的文本/数据传递由 `tmux` 负责，所以快照文本来自本地 `tmux`，而不是 Ghostty 的内容 API。
- `host attach` 现在通过 `internal/remote` 挂接 provider，后续可以扩到 SSH 直连或其他跳板机类型，而不用重写 broker/workspace 核心。
- 当前内置的 `jumpserver` provider 会自动把仓库内置的 runner 和 expect helper 落到 tmux-ghostty 运行时目录；如果需要，仍然可以通过 `TMUX_GHOSTTY_JUMP_RUNNER` 覆盖默认值。
- `host attach` 现在以远端 shell 就绪为成功条件；远端 tmux 只是 best-effort，其结果会通过 `remote_tmux_status` 和 `remote_tmux_detail` 暴露出来。
- 当前测试套件使用真实本地 `tmux` 加 fake Ghostty 编排，因此自动化测试时不会真的弹出 GUI 窗口。
