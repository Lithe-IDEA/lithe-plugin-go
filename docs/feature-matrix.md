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
| `go-support-plugin` | gopls 代码智能、Go Modules 构建、运行与 `go test` | implemented / pending | missing / not-applicable | 2026-11-15 macOS 宿主端到端测试通过 |
| `go-debug-delve` | 用用户的 Delve 调试 Go 包 | implemented / pending | missing / not-applicable | 2026-11-15 macOS 宿主端到端测试通过 |
| `go-project-generator` | 「新建项目」里的 Go 模块，GOROOT 选择和 Go 版本默认值 | implemented / pending | missing / not-applicable | 2026-11-15 macOS 宿主集成测试通过 |

## go-support-plugin

**能力**：用用户自己的 `gopls` 提供代码智能；用声明式 `buildSystems` 识别 Go 模块，在构建窗口里列出包，
运行 `go build`、`go vet`、`go test` 和 `go run`，列出模块依赖；插件设置调整 `go` 路径和 gopls 选项。
没有设置 `go` 路径时，构建用 `toolDefinitions` 里的 `go` 工具定义解析 Go：项目固定的 Go、全局默认、`GOROOT`、
`PATH` 里的 `go`、在常见位置找到的最新安装。

| 平台 | 实现 | 验证 | 说明 |
| --- | --- | --- | --- |
| macOS | implemented | pending | 发布包同时支持 Apple Silicon 和 Intel Mac |
| Windows | missing | not-applicable | 没有 Windows 包：`scripts/package.mjs` 只打 `darwin` 包 |

**证据**：

- `io.github.lithe-idea.go-support/plugin.json`：`languageServers`、`buildSystems`、`settings`、`toolDefinitions` 的声明。
- `e2e/host/desktop/internal/projectgen/go_integration_test.go`：`TestGoPluginToolDefinitionFindsTheLocalGo`
  确认工具定义找到 `PATH` 里那个 go 的 GOROOT 和版本。
- `e2e/host/desktop/internal/workbench/official_go_plugin_e2e_test.go`：宿主端到端测试。

**依赖的宿主能力**：`languageServers`（宿主启动 gopls 并转发 `workspace/configuration`）、`buildSystems`
（构建窗口识别实例、读取模型、运行命令任务和运行入口、运行测试）、`settings`（插件设置页和设置生效方式）。
宿主 [1lck/Lithe-IDEA@059d878d4](https://github.com/1lck/Lithe-IDEA/blob/059d878d4284e1a1fd218aeaa56c29d9753d4dfa/desktop/internal/pluginhost/pluginhost.go)
的 `pluginhost.go` 开头列出了这些能力的服务范围；语言服务器的 code action、signature help 和 inlay hints
宿主还没有界面，插件声明了也不会显示。

**最近验证**：2026-11-15，macOS（Apple Silicon），插件 0.1.0 发布包，宿主 `cebbe0def`（1lck/Lithe-IDEA#1958 的分支提交）。
`node scripts/e2e-host.mjs` 通过 `TestOfficialGoPluginBuildsAndServesAModule`，覆盖识别 `go.mod`、`go list` 模型和
`application` 目标、Build、Vet、运行入口、Go executable 设置重新读取模型并用于后续构建、gopls 诊断、
gofumpt 设置立即生效，以及工作流结束后安装目录逐字节不变。没有覆盖 `go test`、依赖列表、`go.mod` 变化后重新读取、
构建选项（Build Tags、Race Detector）、缺少 gopls 时的提示，也没有做下面的手工验收。
这次验证早于构建加入 `go` 工具定义解析（插件 PR “GOROOT 选择”），之后没有重跑这个测试。

**验证方法**（macOS）：

1. 用“设置 → 插件 → 从文件安装…”安装 `dist/go-support-<版本>-darwin.zip`，启用后不需要重启。
2. 打开含 `go.mod` 的项目：构建窗口出现 Go Modules，列出模块和包，每个 `main` 包有“运行 <包名>”按钮。
3. 运行 Build、Vet、Test（整个模块、单个包、用 `-run` 只跑一个测试）和运行入口，输出显示在构建输出里；
   `文件:行:列: 消息` 形式的编译错误显示为问题。
4. 打开 `.go` 文件：gopls 来自用户的 `PATH`，补全、跳转定义、悬停、诊断和格式化可用；修改 gofumpt、
   Staticcheck、补全占位符、悬停文档设置后立即生效，不需要重启语言服务器。
5. 修改 Go executable 设置后模块重新读取，之后的构建使用新路径；Build Tags 和 Race Detector 构建选项出现在命令里。
   清空 Go executable 后，「构建、执行、部署 › 工具链」列出电脑上的 Go；为项目选另一个 Go 后，构建输出里的 `go version` 随之改变。
6. 查看依赖列表（含版本和间接依赖）；修改 `go.mod` 后模块重新读取。
7. `PATH` 里没有 gopls 时给出缺失提示，不崩溃。
8. 禁用插件后 gopls 和运行中的进程退出；插件安装目录在整个过程中不被写入。

**迁移来源**：主仓库 `shared/platform-feature-matrix/features/go-support-plugin.json`
（[1lck/Lithe-IDEA@b582e206a](https://github.com/1lck/Lithe-IDEA/blob/b582e206a/shared/platform-feature-matrix/features/go-support-plugin.json)），
在 #1915 中删除。原条目写的是旧 Swift 宿主上官方包 `dev.lithe.plugin.go-support` 迁移到社区版 0.1.0 的流程，
以及 Viva 宿主当时还没有运行和测试入口的状态。插件已按契约 v0 重写并从 0.0.1 重新发布，宿主也已经支持
`buildSystems` 的运行和测试，这两部分不再适用，本条按当前插件重写。

## go-debug-delve

**能力**：「调试」窗口里的 Go (Delve) 适配器。Lithe 启动用户 `PATH` 里的 `dlv dap`，读到 Delve 在标准输出打印的
`DAP server listening at: <地址>` 后连接这个本机地址；调试当前文件所在的包（`target.directory` 绑定到 `program`），
工作目录是项目根目录。支持断点、单步、调用栈、变量和表达式求值，程序输出显示在调试控制台，程序结束后会话自动结束。

| 平台 | 实现 | 验证 | 说明 |
| --- | --- | --- | --- |
| macOS | implemented | pending | 需要宿主支持 `tcpAnnounced` 调试适配器（1lck/Lithe-IDEA#1954）、Rust Core a58e07e 或更新版本的 `target.directory` 绑定，以及宿主按工作目录设置子进程 `PWD`（1lck/Lithe-IDEA#1958；没有它时，项目路径经过符号链接的话 Delve 编译会失败） |
| Windows | missing | not-applicable | 没有 Windows 包 |

**证据**：

- `io.github.lithe-idea.go-support/plugin.json`：`debugAdapters` 里的 `delve`，以及 `debug.launch` 权限和 `onDebug:delve` 激活事件。
- `e2e/host/desktop/internal/workbench/official_go_plugin_debug_e2e_test.go`：宿主端到端测试。

**依赖的宿主能力**：`debugAdapters` 的 `tcpAnnounced` 传输（宿主启动适配器进程，交给 Core 判断哪一行是地址公告，
再连接公告的本机端口）、`target.directory` 绑定（Core a58e07e，Lithe-core-rust#47）。

**限制**：只支持 launch，不支持 attach 和调试测试；适配器只在打开 `.go` 文件时可选（选择器是 `file` 加 `go` 语言），
从构建窗口的运行入口启动调试还不支持。Delve 在项目根目录生成 `__debug_bin…` 临时程序，会话结束时删除。
适配器名称 Go (Delve) 目前不能翻译：插件语言文件还不覆盖调试适配器。

**最近验证**：2026-11-15，macOS（Apple Silicon），插件 0.1.0 发布包，宿主 `cebbe0def`（1lck/Lithe-IDEA#1958 的分支提交），
Go 1.27.4、Delve 1.27.2。`node scripts/e2e-host.mjs` 通过 `TestOfficialGoPluginDebugsAModuleWithDelve`，覆盖下面验证方法的
第 2 步、第 3 步的断点停下、调用栈顶和局部变量、第 4 步和第 7 步，并确认 Delve 的地址公告没有显示在调试控制台里。
没有覆盖安装对话框里的权限（第 1 步）、单步和表达式求值、调试子包（第 5 步）、缺少 `dlv` 时的提示（第 6 步），也没有做手工验收。

**验证方法**（macOS）：

1. 安装插件包，确认安装对话框列出启动调试器权限；`PATH` 里有 `dlv`。
2. 打开含 `go.mod` 的项目，打开 `main.go`，在一行上设断点，在「调试」窗口选 Go (Delve) 后点「开始」。
3. 程序停在断点：调用栈顶是 `main.main` 和断点所在行，变量里能看到局部变量的值；单步、表达式求值可用。
4. 点「继续」：程序运行到结束，输出显示在调试控制台，会话自动结束，项目根目录里没有留下 `__debug_bin…`。
5. 调试 `cmd/<名字>` 这样的子包：打开该包里的文件后开始，调试的是这个包。
6. `PATH` 里没有 `dlv` 时，开始调试给出缺少程序的提示，不崩溃。
7. 整个过程中插件安装目录不被写入。

## go-project-generator

**能力**：「新建项目」对话框里的 Go。GOROOT 下拉列出 `go` 工具定义在电脑上找到的 Go，默认是版本最新的一个，
创建后记为项目固定，构建和运行使用它；没有找到 Go 时留空，不影响创建。生成 `go.mod`（`module` 行是模块路径，
默认是小写的项目名；`go` 行默认是所选 GOROOT 的版本，手动改过后不再跟随）和 `.gitignore`，可选生成打印
`Hello, Gopher!` 的示例 `main.go`。`go.mod` 由模板写出，不运行 `go mod init`，创建项目不需要联网，也不需要先装好 `go`。

| 平台 | 实现 | 验证 | 说明 |
| --- | --- | --- | --- |
| macOS | implemented | pending | |
| Windows | missing | not-applicable | 没有 Windows 包 |

**证据**：

- `io.github.lithe-idea.go-support/plugin.json`：`projectGenerators` 里的 `go-module`（`goroot` 是 `tool` 字段，
  `goVersion` 默认 `${field.goroot.version}`）、`toolDefinitions` 里的 `go`，以及 `project.generate` 权限。
- `io.github.lithe-idea.go-support/templates/go/`：模块和示例代码模板。
- `e2e/host/desktop/internal/projectgen/go_integration_test.go`：经 Core 的表单、生成计划和宿主执行器的集成测试。

**依赖的宿主能力**：`projectGenerators` 的 `tool` 字段和创建后的项目固定，`toolDefinitions` 的发现和解析；
以子命令开头的版本探针 `go version` 需要 Rust Core 6477e9a（Lithe-core-rust#52，宿主 [1lck/Lithe-IDEA#1964](https://github.com/1lck/Lithe-IDEA/pull/1964)）。

**限制**：字段名称和说明目前只有英文，插件语言文件还不覆盖新建项目。

**最近验证**：2026-11-15，macOS（Apple Silicon），插件源目录里的清单和模板，宿主 `b03dbf0c0`（1lck/Lithe-IDEA#1964 的分支提交：preview 1fc1e8b05 加上
Core 6477e9a），Homebrew 的 Go 1.27.4。在宿主检出里运行 `internal/projectgen` 的 `^TestGoPlugin`，通过五个集成测试，经 Core 的表单、
生成计划、宿主执行器和工具链解析：

- `TestGoPluginToolDefinitionFindsTheLocalGo`：插件的 `go` 工具定义找到 `PATH` 里那个 go 的 GOROOT
  （`/opt/homebrew/Cellar/go/1.27.4/libexec`），版本与 `go env GOVERSION` 一致。
- `TestGoPluginGeneratesAModuleWithSampleCode`：用找到的 Go 填 GOROOT，`Hello-Go` 生成 `module hello-go` 和
  `go 1.27.4`、`main.go`，`.gitignore` 忽略 `__debug_bin*`，打开 `main.go`；生成计划带 `goroot` 的项目固定，
  保存后工具链把项目里的 `go` 解析到这个 GOROOT，用它 `go vet` 通过、`go run` 输出 `Hello, Gopher!`。
- `TestGoPluginGeneratesAnEmptyModuleWithoutAGoroot`：没有 Go 时手填 Go 版本，关闭示例代码、改模块路径，
  只生成 `go.mod` 并打开它，没有项目固定。
- `TestGoPluginGoVersionFollowsTheGoroot`：没有 Go 或只有低于 1.21 的 Go 时 GOROOT 留空、Go 版本必填；
  Go 版本默认取最新的 GOROOT，换 GOROOT 后跟着变，手动改过后保留；手动选低于 1.21 的 GOROOT 时表单报错。
- `TestGoPluginFormRejectsAModulePathWithSpaces`：`My App` 默认得到的模块路径被表单拒绝，改成合法路径后通过。

覆盖验证方法第 2 步里的文件、打开的文件、项目固定和 `go run`，以及第 3、4、5、6 步的表单和生成结果；
没有覆盖新建项目对话框的界面和图标（第 1 步）、GOROOT 下拉的显示、创建后在构建窗口里的识别，也没有做手工验收。

**验证方法**（macOS）：

1. 安装插件包，打开「新建项目」：左侧出现 Go，图标在浅色和深色外观下都清楚。
2. GOROOT 下拉列出电脑上的 Go，默认选中版本最新的一个。名称填 `Hello-Go`，保持默认创建：项目里有 `go.mod`
   （`module hello-go`，`go` 行是所选 Go 的版本）、`.gitignore` 和 `main.go`，创建后打开 `main.go`；构建窗口识别出
   Go Modules，运行入口输出 `Hello, Gopher!`；「构建、执行、部署 › 工具链」显示项目固定了这个 Go。
3. 关闭“添加示例代码”再创建：没有 `main.go`，创建后打开 `go.mod`。
4. 名称里有空格（如 `My App`）时，模块路径显示问题，改成合法路径后才能创建。
5. 换一个 GOROOT，高级选项里的 Go 版本跟着变；手动修改 Go 版本后再换 GOROOT，Go 版本保持手动填写的值，
   `go.mod` 的 `go` 行是这个值。
6. 电脑上没有 Go（或只有低于 1.21 的 Go）时 GOROOT 留空，高级选项自动展开并提示填写 Go 版本，填写后可以创建。
