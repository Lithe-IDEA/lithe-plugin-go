# Go Support for Lithe

为 [Lithe](https://github.com/1lck/Lithe-IDEA) 提供 Go 支持：用你电脑上的 `gopls` 提供代码智能，
用声明式构建系统接口（`buildSystems`）识别 Go 模块，在构建窗口里列出包，运行 `go build`、`go vet`、
`go test` 和 `go run`，查看模块依赖。
这是一个**社区插件**，按插件契约 v0 编写。插件只有一份 `plugin.json`，不附带任何可执行文件，也没有后端进程。

> English: Go support for Lithe — code intelligence through your own `gopls`, and the Go Modules build system
> (package model, build, vet, test, run and the module list) through the declarative `buildSystems` contract.
> The plugin is a single `plugin.json` with no executables and no backend process.

> **预发布**：按插件契约 v0（`schemaVersion: 0`、`pluginAPIVersion: 0`）编写，只能装进新版 Lithe 桌面端
> （`desktop-v*` 预发布），旧版 macOS Lithe 无法安装。

## 功能

| 功能 | 说明 |
| --- | --- |
| 代码智能 | 补全、跳转定义、悬停、诊断、格式化，由 `gopls` 提供 |
| 识别模块 | 含 `go.mod` 的目录是构建根；`go list -json ./...` 的结果组成模块和包 |
| 构建 | Build（`go build ./...`）作用于模块；Vet（`go vet`）可作用于模块或单个包 |
| 运行 | 每个 `main` 包在构建窗口里有「运行 <包名>」按钮，执行 `go run <导入路径>`，输出显示在构建输出里 |
| 测试 | `go test` 作用于模块或单个包，可用 `-run` 只跑一个测试；输出显示在构建输出里，插件不提供测试报告文件 |
| 诊断 | 把 `文件:行:列: 消息` 形式的编译输出显示为问题 |
| 依赖 | `go list -m -json all` 列出模块依赖（含版本和是否间接依赖） |
| 同步 | `go.mod`、`go.sum`、`go.work` 变化后重新读取模块 |

## 设置和构建选项

**插件设置**（设置 → 插件 → Go Support →「设置」）：

| 设置 | 生效方式 | 说明 |
| --- | --- | --- |
| Go executable | 重新读取模块 | 读取模块、构建、检查、测试和运行用的 `go`。留空时使用 `PATH` 里的 `go`。也显示在「构建、执行、部署 › Go Modules」页 |
| Staticcheck analyses | 立即 | 让 gopls 额外报告 staticcheck 的检查结果；大模块上更耗 CPU 和内存 |
| Format with gofumpt | 立即 | 格式化时使用 gofumpt 的更严格规则 |
| Parameter placeholders in completion | 立即 | 补全函数调用时插入参数占位符 |
| Hover documentation | 立即 | 悬停显示多少文档：完整、第一句、一行或只有签名 |

后四项通过 `workspace/configuration` 发给正在运行的 gopls，不需要重启语言服务器；
它们也显示在「语言与框架 › Go」页。

**构建选项**（构建窗口 →「构建选项」），只对当前工作区生效：

| 选项 | 传给 go 的内容 |
| --- | --- |
| Build Tags | `-tags <值>` |
| Race Detector | `-race` |

选项会加到构建、检查、测试和运行的命令里。

## 依赖的程序

| 程序 | 用途 | 查找方式 |
| --- | --- | --- |
| `go` 1.21 或更新版本 | 读取模块、构建、检查、测试、运行、依赖 | 插件设置 Go executable；留空时找 `PATH` 里的 `go` |
| `gopls` | 代码智能 | `PATH` 里的 `gopls` |

安装插件时 Lithe 会列出这些程序和插件需要的权限（启动语言服务器、运行构建命令），确认后才会安装。
没有这两个程序的话可以先安装：

```bash
brew install go
go install golang.org/x/tools/gopls@latest
```

`go install` 把 gopls 装在 `$(go env GOPATH)/bin`，这个目录需要在 Lithe 能看到的 `PATH` 里。

## 安装

1. 从 [Releases](https://github.com/Lithe-IDEA/lithe-plugin-go/releases) 下载 `go-support-<版本>-darwin.zip`。
2. 在 Lithe 中打开 **设置 → 插件**，点击页面标题栏右侧的 **从文件安装…**，选中这个 zip，核对 SHA-256 和权限后确认。
3. 打开含 `go.mod` 的项目。构建窗口会出现 Go Modules，打开 `.go` 文件时启动 gopls。

一个包同时支持 Apple Silicon 和 Intel Mac。

## 工作方式

- 读取模块、构建、检查、测试、运行和依赖都是 `plugin.json` 里声明的 `go` 命令，由 Lithe 解析 `go` 并运行。
- 运行入口由 Lithe 从模型里的 `main` 包生成：每个带 `application` 标签的包一条，名称是导入路径的最后一段。
- gopls 由 Lithe 按 `languageServers` 的声明启动，插件设置通过 `workspaceConfiguration` 里的
  `${setting:<id>}` 传给它。
- 插件目录在运行时保持只读。

## 诊断日志

插件没有自己的进程，也不发送 `lithe/log`。gopls 由 Lithe 启动，它的启动结果、`window/logMessage`
和有界的 stderr 由 Lithe 记进语言服务的日志；`go` 命令的输出显示在构建输出里，不复制进诊断日志。
排障时使用 Lithe 的日志窗口和诊断导出，不要开启 gopls 的 RPC dump，也不要把源码、环境变量或完整命令行写进日志。

## 本地打包

需要 Node.js 22+ 和 `zip`：

```bash
node scripts/package.mjs
```

输出 `dist/go-support-<版本>-darwin.zip` 和它的 SHA-256。本地开发时，用 **设置 → 插件 → 从文件安装…**
选择这个 zip 安装。

`plugin.json` 的完整格式见 Rust Core 仓库里的
[插件契约](https://github.com/Lithe-IDEA/Lithe-core-rust/blob/main/shared/plugins/contract/v1/README.md)、
[构建工具接入指南](https://github.com/Lithe-IDEA/Lithe-core-rust/blob/main/shared/plugins/guides/build-tool-integration.md)
和[语言服务器插件生命周期](https://github.com/Lithe-IDEA/Lithe-core-rust/blob/main/shared/plugins/guides/lsp-plugin-lifecycle.md)。

## 宿主端到端测试

`e2e/host/` 里的测试通过 Lithe 桌面端自己的测试工具（宿主的 `desktop/internal/workbench` 包）安装真实插件包，
像用户一样操作构建窗口和编辑器。这些测试原来放在 Lithe 主仓库，按“宿主不依赖插件”的要求
（[1lck/Lithe-IDEA#1915](https://github.com/1lck/Lithe-IDEA/pull/1915)）移到这里：主仓库不读取本仓库，
`e2e/host/lithe-host.json` 记录测试最后一次通过时的宿主版本。

需要 Node.js 22+、Go、Rust 工具链（构建宿主固定的 Rust Core）、`git`，以及测试要用的 `go` 和 `gopls`：

```bash
node scripts/package.mjs
LITHE_TEST_GO_PLUGIN_ARCHIVE="$PWD/dist/go-support-0.0.1-darwin.zip" node scripts/e2e-host.mjs
```

插件包路径要写绝对路径。脚本把固定版本的宿主（只取 `desktop/` 和 `scripts/`）检出到 `.artifacts/lithe-host`，
复制 `e2e/host/desktop/` 下的测试，再在那里运行宿主的 `scripts/desktop/desktop-go.sh test`；第一次运行会克隆并构建
Rust Core，`LITHE_CORE_RUST_DIR` 可以指向本地的 Core 检出。`--ref <分支或 commit>` 换一个宿主版本试跑，
`--run <正则>` 只跑部分测试。测试用的是宿主内部的测试工具，升级 `lithe-host.json` 的 `revision` 时可能要同步修改测试。

## 功能矩阵

插件各项能力在 macOS 和 Windows 上的实现与验收状态见 [docs/feature-matrix.md](docs/feature-matrix.md)。
Lithe 主仓库的功能矩阵不收录插件能力，改动插件行为时在同一个 PR 里更新这一页。

## 发布新版本

1. 按语义化版本修改 `io.github.lithe-idea.go-support/plugin.json` 的 `version`：只修 Bug 升修订号，
   加功能升次版本号，有不兼容的变化升主版本号。合入 `main`。
2. 推送同名标签：

   ```bash
   git tag v0.0.1 && git push origin v0.0.1
   ```

3. `Release` 工作流会核对标签与版本一致、打包并发布 Release（说明里附 SHA-256 和
   `docs/releases/v<版本>.md`）。`release.yml` 里 `PRERELEASE_TAGS` 列出的版本会标为预发布版本：
   目前是只能装进新版桌面端的 `v0.0.1`。
4. Lithe Store 的自动追踪会发现稳定版本并开合并请求，合并后用户即可在 Lithe 中更新。自动追踪会跳过预发布版本；
   需要收录时在 Store 中手工提交条目，并使用已发布 ZIP 的实际 SHA-256。

## 许可

[Apache-2.0](LICENSE)
