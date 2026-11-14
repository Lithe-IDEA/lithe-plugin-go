# 功能矩阵

本页记录 Go Support 插件自己的跨平台功能状态。Lithe 主仓库的功能矩阵（`shared/platform-feature-matrix/`）
只记录宿主能力，不再收录插件能力（[1lck/Lithe-IDEA#1915](https://github.com/1lck/Lithe-IDEA/pull/1915)）。
插件能力的状态、证据和验证方法在这里维护，改动插件行为的 PR 同时更新本页。

## 状态取值

沿用主仓库功能矩阵的取值：

| 字段 | 取值 |
| --- | --- |
| 实现 | `implemented` 已实现；`partial` 部分实现；`missing` 未实现；`not-applicable` 该平台不适用 |
| 验证 | `verified` 已按“验证方法”完整验收；`pending` 尚未完整验收；`not-applicable` 无需验收 |

只有按“验证方法”完整走过一遍，并写明日期、插件版本和宿主版本，才能标 `verified`。
自动化测试的结果写在“最近验证”里，不能代替验收。

## 总览

| ID | 能力 | macOS | Windows | 最近验证 |
| --- | --- | --- | --- | --- |
| `go-support-plugin` | gopls 代码智能、Go Modules 构建、运行与 `go test` | implemented / pending | missing / not-applicable | 2026-11-14 macOS 宿主端到端测试通过 |

## go-support-plugin

**能力**：用用户自己的 `gopls` 提供代码智能；用声明式 `buildSystems` 识别 Go 模块，在构建窗口里列出包，
运行 `go build`、`go vet`、`go test` 和 `go run`，列出模块依赖；插件设置调整 `go` 路径和 gopls 选项。

| 平台 | 实现 | 验证 | 说明 |
| --- | --- | --- | --- |
| macOS | implemented | pending | 发布包同时支持 Apple Silicon 和 Intel Mac |
| Windows | missing | not-applicable | 没有 Windows 包：`scripts/package.mjs` 只打 `darwin` 包 |

**证据**：

- `io.github.lithe-idea.go-support/plugin.json`：`languageServers`、`buildSystems`、`settings` 的声明。
- `e2e/host/desktop/internal/workbench/official_go_plugin_e2e_test.go`：宿主端到端测试。

**依赖的宿主能力**：`languageServers`（宿主启动 gopls 并转发 `workspace/configuration`）、`buildSystems`
（构建窗口识别实例、读取模型、运行命令任务和运行入口、运行测试）、`settings`（插件设置页和设置生效方式）。
宿主 [1lck/Lithe-IDEA@059d878d4](https://github.com/1lck/Lithe-IDEA/blob/059d878d4284e1a1fd218aeaa56c29d9753d4dfa/desktop/internal/pluginhost/pluginhost.go)
的 `pluginhost.go` 开头列出了这些能力的服务范围；语言服务器的 code action、signature help 和 inlay hints
宿主还没有界面，插件声明了也不会显示。

**最近验证**：2026-11-14，macOS（Apple Silicon），插件 0.0.1 发布包，宿主 `059d878d4`。
`node scripts/e2e-host.mjs` 通过 `TestOfficialGoPluginBuildsAndServesAModule`，覆盖识别 `go.mod`、`go list` 模型和
`application` 目标、Build、Vet、运行入口、Go executable 设置重新读取模型并用于后续构建、gopls 诊断、
gofumpt 设置立即生效，以及工作流结束后安装目录逐字节不变。没有覆盖 `go test`、依赖列表、`go.mod` 变化后重新读取、
构建选项（Build Tags、Race Detector）、缺少 gopls 时的提示，也没有做下面的手工验收。

**验证方法**（macOS）：

1. 用“设置 → 插件 → 从文件安装…”安装 `dist/go-support-<版本>-darwin.zip`，启用后不需要重启。
2. 打开含 `go.mod` 的项目：构建窗口出现 Go Modules，列出模块和包，每个 `main` 包有“运行 <包名>”按钮。
3. 运行 Build、Vet、Test（整个模块、单个包、用 `-run` 只跑一个测试）和运行入口，输出显示在构建输出里；
   `文件:行:列: 消息` 形式的编译错误显示为问题。
4. 打开 `.go` 文件：gopls 来自用户的 `PATH`，补全、跳转定义、悬停、诊断和格式化可用；修改 gofumpt、
   Staticcheck、补全占位符、悬停文档设置后立即生效，不需要重启语言服务器。
5. 修改 Go executable 设置后模块重新读取，之后的构建使用新路径；Build Tags 和 Race Detector 构建选项出现在命令里。
6. 查看依赖列表（含版本和间接依赖）；修改 `go.mod` 后模块重新读取。
7. `PATH` 里没有 gopls 时给出缺失提示，不崩溃。
8. 禁用插件后 gopls 和运行中的进程退出；插件安装目录在整个过程中不被写入。

**迁移来源**：主仓库 `shared/platform-feature-matrix/features/go-support-plugin.json`
（[1lck/Lithe-IDEA@b582e206a](https://github.com/1lck/Lithe-IDEA/blob/b582e206a/shared/platform-feature-matrix/features/go-support-plugin.json)），
在 #1915 中删除。原条目写的是旧 Swift 宿主上官方包 `dev.lithe.plugin.go-support` 迁移到社区版 0.1.0 的流程，
以及 Viva 宿主当时还没有运行和测试入口的状态。插件已按契约 v0 重写并从 0.0.1 重新发布，宿主也已经支持
`buildSystems` 的运行和测试，这两部分不再适用，本条按当前插件重写。
