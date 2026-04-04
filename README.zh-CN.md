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
- 复用本地 `tmux-jumpserver` runner 的 JumpServer attach 适配层
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
  jump/
  logx/
  model/
  observe/
  risk/
  rpc/
  store/
  tmux/
```

## Codex 适配 Skill

这个仓库内置了一个 repo-local Codex skill，位于 `skills/tmux-ghostty/SKILL.md`。当 Codex 需要把 `tmux-ghostty` 当作主要的 Ghostty/tmux 共享控制面来操作时，应优先使用它。

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

`tmux-ghostty help` 是权威的详细命令参考。README 只保留高层级命令树，具体命令说明请以 CLI 输出为准。`tmux-ghostty -h` 和 `tmux-ghostty --help` 是等价别名。

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

## 维护者发布

### 用 Homebrew 发布

这个仓库现在可以生成一个可发布的 Homebrew formula 文件：

```bash
make homebrew-formula VERSION=v0.1.0
```

生成结果位于：

```text
dist/release/<version>/homebrew/Formula/tmux-ghostty.rb
```

但要真正通过 Homebrew 发布，你仍然需要一个单独的 tap 仓库，并在其中放置：

```text
Formula/tmux-ghostty.rb
```

Homebrew 发布需要额外配置这些内容：

- 公开 tap 仓库已经固定为 `Woo-kk/homebrew-tmux-ghostty`
- 在当前仓库里添加 Actions secret：`HOMEBREW_TAP_TOKEN=<fine-grained PAT>`。这个 token 只需要对 tap 仓库有 `contents:write` 权限。

可选配置：

- `HOMEBREW_TAP_BRANCH=main`
- `HOMEBREW_TAP_FORMULA_PATH=Formula/tmux-ghostty.rb`
- `TMUX_GHOSTTY_HOMEBREW_FORMULA=tmux-ghostty`
- `TMUX_GHOSTTY_HOMEBREW_CLASS=TmuxGhostty`
- `TMUX_GHOSTTY_HOMEBREW_HOMEPAGE=https://github.com/Woo-kk/tmux-ghostty`
- `TMUX_GHOSTTY_HOMEBREW_DESC=Shared terminal broker for Ghostty powered by tmux`

这些配好后，现有的 release workflow 会在每次打 tag 发布后，自动把生成出的 formula 同步到 tap 仓库。若你要在本地手动推送，可以执行：

```bash
HOMEBREW_TAP_REPO=Woo-kk/homebrew-tmux-ghostty \
HOMEBREW_TAP_TOKEN=<token> \
make publish-homebrew-tap VERSION=v0.1.0
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
- JumpServer 菜单式输入，例如 `/1201` 或 `1`，当前没有做特殊放行，因此也会被归类为 `risky`

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
- `TMUX_GHOSTTY_JUMP_PROFILE`
- `TMUX_GHOSTTY_JUMP_RUNNER`
- `TMUX_GHOSTTY_REMOTE_TMUX_SESSION`

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
- JumpServer 适配层默认假设本机已有 `/Users/guyuanshun/.codex/skills/tmux-jumpserver/scripts/run_jump_profile.sh`；如果需要，可通过 `TMUX_GHOSTTY_JUMP_RUNNER` 覆盖。
- 当前测试套件使用真实本地 `tmux` 加 fake Ghostty 编排，因此自动化测试时不会真的弹出 GUI 窗口。
