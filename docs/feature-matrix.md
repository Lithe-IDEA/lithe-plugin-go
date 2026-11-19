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
| `go-support-plugin` | gopls 代码智能、Go Modules 构建、运行与 `go test` | implemented / pending | implemented / pending | 2026-11-15 macOS 宿主端到端测试通过；Windows ARM64、x64 [原生 CI 通过](https://github.com/Lithe-IDEA/lithe-plugin-go/actions/runs/43728078241) |
| `go-debug-delve` | 用用户的 Delve 调试 Go 包 | implemented / pending | partial / pending | 2026-11-15 macOS 宿主端到端测试通过；Windows x64 [原生 CI 通过](https://github.com/Lithe-IDEA/lithe-plugin-go/actions/runs/43728078241)；Windows ARM64 上 Delve 不支持 |
| `go-run-configuration-types` | 「运行/调试配置」里的 Go 构建和 Go 测试 | implemented / pending | implemented / pending | 2026-11-19 macOS 宿主端到端测试通过（桌面端 0.8.8，宿主 d87f114）；Windows 只有命令层面的 `scripts/test.mjs` |
| `go-settings-previews` | Go 语言设置页和 Go 构建系统页里插件自己画的预览 | implemented / pending | implemented / pending | 2026-11-19 macOS 宿主端到端测试通过（本地 1.4.0 包，宿主 312a9c63a，即 1lck/Lithe-IDEA#2191 的分支提交）；Windows 后端只在 CI 构建，没有在 Windows 版 Lithe 里运行过 |
| `go-project-generator` | 「新建项目」里的 Go 模块，GOROOT 选择和 Go 版本默认值 | implemented / pending | implemented / pending | 2026-11-15 macOS 宿主集成测试通过；Windows ARM64、x64 [原生 CI 通过](https://github.com/Lithe-IDEA/lithe-plugin-go/actions/runs/43728078241)（模板生成的模块能构建和测试） |

## go-support-plugin

**能力**：用用户自己的 `gopls` 提供代码智能，缺少 gopls 时可以在插件详情的能力卡片上安装；用声明式 `buildSystems` 识别 Go 模块，在构建窗口里列出包，
运行 `go build`、`go vet`、`go test` 和 `go run`，列出模块依赖；插件设置调整 `go` 路径和 gopls 选项；
`indexingExclusions` 让宿主把排除在索引之外的目录写进 gopls 的 `directoryFilters`。
没有设置 `go` 路径时，构建用 `toolDefinitions` 里的 `go` 工具定义解析 Go：项目固定的 Go、全局默认、`GOROOT`、
`PATH` 里的 `go`、在常见位置找到的最新安装。

| 平台 | 实现 | 验证 | 说明 |
| --- | --- | --- | --- |
| macOS | implemented | pending | 发布包同时支持 Apple Silicon 和 Intel Mac |
| Windows | implemented | pending | `-windows-x64.zip` 和 `-windows-arm64.zip` 只在设置预览后端上与 darwin 包不同：GOROOT 里解析 `bin/go.exe`，gopls 按 PATHEXT 找到 `gopls.exe`。见下文“Windows 安装包” |

**证据**：

- `io.github.lithe-idea.go-support/plugin.json`：`languageServers`、`buildSystems`、`settings`、`toolDefinitions` 的声明，
  以及 `commandInstallers` 里的 `gopls`（`requiresTools` 为 `go`，`commands.install` 权限）。
- `e2e/host/desktop/internal/projectgen/go_integration_test.go`：`TestGoPluginToolDefinitionFindsTheLocalGo`
  确认工具定义找到 `PATH` 里那个 go 的 GOROOT 和版本。
- `e2e/host/desktop/internal/workbench/official_go_plugin_e2e_test.go`：宿主端到端测试。

**依赖的宿主能力**：`languageServers`（宿主启动 gopls 并转发 `workspace/configuration`；`indexingExclusions`
需要固定了 Lithe-core-rust#68 的桌面端，之前 gopls 的排除目录写死在 Core 里）、`buildSystems`
（构建窗口识别实例、读取模型、运行命令任务和运行入口、运行测试）、`settings`（插件设置页和设置生效方式）、
`commandInstallers`（Lithe-core-rust#53，Core c550c2b；宿主在 1lck/Lithe-IDEA#1971 检测缺少的命令并提供安装按钮）。
宿主 [1lck/Lithe-IDEA@059d878d4](https://github.com/1lck/Lithe-IDEA/blob/059d878d4284e1a1fd218aeaa56c29d9753d4dfa/desktop/internal/pluginhost/pluginhost.go)
的 `pluginhost.go` 开头列出了这些能力的服务范围；语言服务器的 code action、signature help 和 inlay hints
宿主还没有界面，插件声明了也不会显示。

**最近验证**：2026-11-15，macOS（Apple Silicon），插件 0.1.0 发布包，宿主 `cebbe0def`（1lck/Lithe-IDEA#1958 的分支提交）。
`node scripts/e2e-host.mjs` 通过 `TestOfficialGoPluginBuildsAndServesAModule`，覆盖识别 `go.mod`、`go list` 模型和
`application` 目标、Build、Vet、运行入口、Go executable 设置重新读取模型并用于后续构建、gopls 诊断、
gofumpt 设置立即生效，以及工作流结束后安装目录逐字节不变。没有覆盖 `go test`、依赖列表、`go.mod` 变化后重新读取、
构建选项（Build Tags、Race Detector）、缺少 gopls 时的提示和安装（第 7 步，`commandInstallers` 声明晚于这次验证），
也没有做下面的手工验收。
这次验证早于构建加入 `go` 工具定义解析（插件 PR “GOROOT 选择”），之后没有重跑这个测试。

**验证方法**（macOS）：

1. 用“设置 → 插件 → 从文件安装…”安装 `dist/go-support-<版本>-darwin.zip`，启用后不需要重启。
2. 打开含 `go.mod` 的项目：构建窗口出现 Go Modules，列出模块和包，每个 `main` 包有“运行 <包名>”按钮。
3. 运行 Build、Vet、Test（整个模块、单个包、用 `-run` 只跑一个测试）和运行入口，输出显示在构建输出里；
   `文件:行:列: 消息` 形式的编译错误显示为问题。
4. 打开 `.go` 文件：gopls 来自在 Lithe 里安装时记下的目录或用户的 `PATH`，补全、跳转定义、悬停、诊断和格式化可用；修改 gofumpt、
   Staticcheck、补全占位符、悬停文档设置后立即生效，不需要重启语言服务器。
5. 修改 Go executable 设置后模块重新读取，之后的构建使用新路径；Build Tags 和 Race Detector 构建选项出现在命令里。
   清空 Go executable 后，「构建、执行、部署 › 工具链」列出电脑上的 Go；为项目选另一个 Go 后，构建输出里的 `go version` 随之改变。
6. 查看依赖列表（含版本和间接依赖）；修改 `go.mod` 后模块重新读取。
7. 没有 gopls 时，插件详情「功能」页的语言服务器卡片显示“未找到 gopls”和安装按钮。分别选 Lithe 管理的目录和
   Go 的默认位置安装一次：对话框列出 `go install golang.org/x/tools/gopls@latest`，装好后卡片不再提示，
   重新打开 `.go` 文件时 gopls 启动；装进 Lithe 管理的目录时 `go env GOPATH` 下的 `bin` 没有新增文件。
8. 禁用插件后 gopls 和运行中的进程退出；插件安装目录在整个过程中不被写入。

**迁移来源**：主仓库 `shared/platform-feature-matrix/features/go-support-plugin.json`
（[1lck/Lithe-IDEA@b582e206a](https://github.com/1lck/Lithe-IDEA/blob/b582e206a/shared/platform-feature-matrix/features/go-support-plugin.json)），
在 #1915 中删除。原条目写的是旧 Swift 宿主上官方包 `dev.lithe.plugin.go-support` 迁移到社区版 0.1.0 的流程，
以及 Viva 宿主当时还没有运行和测试入口的状态。插件已按契约 v0 重写并从 0.0.1 重新发布，宿主也已经支持
`buildSystems` 的运行和测试，这两部分不再适用，本条按当前插件重写。

## go-debug-delve

**能力**：「调试」窗口里的 Go (Delve) 适配器。Lithe 启动在 Lithe 里安装时记下的或用户 `PATH` 里的 `dlv dap`，读到 Delve 在标准输出打印的
`DAP server listening at: <地址>` 后连接这个本机地址；在当前文件所在的包目录里构建并调试这个包（`dlvCwd` 和 `env.PWD` 绑定到 `target.directory`，`program` 是 `.`），
程序的工作目录也是包目录（`cwd` 绑定到 `target.directory`，不再是调试引擎默认的工作区根目录）。打开的文件夹根目录不需要 `go.mod`，可以并排放多个模块；没有 `go.mod` 的目录以
`GO111MODULE=auto` 构建目录里的全部文件。支持断点、单步、调用栈、变量和表达式求值，程序输出显示在调试控制台，程序结束后会话自动结束。
缺少 `dlv` 时可以在插件详情的能力卡片上安装。

| 平台 | 实现 | 验证 | 说明 |
| --- | --- | --- | --- |
| macOS | implemented | pending | 需要宿主支持 `tcpAnnounced` 调试适配器（1lck/Lithe-IDEA#1954），以及带 `targetBindings.adapterArguments`、且 `targetBindings.cwd` 可绑到 `target.directory` 的 Rust Core（Lithe-core-rust#55，宿主固定版本在 1lck/Lithe-IDEA 的跟进 PR 里）。旧宿主不认识这个字段，会拒绝安装本版本 |
| Windows | partial | pending | x64：同 macOS 的宿主要求；`dlv` 按 PATHEXT 找到 `dlv.exe`，托管安装写到 `GOBIN`。ARM64：Delve 本身不支持 windows/arm64（1.27.2 构建时报 `your_windows_architecture_is_not_supported_by_delve`），能力卡片上的安装会以这个构建错误失败，插件清单无法按架构隐藏它。见下文“Windows 安装包” |

**证据**：

- `io.github.lithe-idea.go-support/plugin.json`：`debugAdapters` 里的 `delve`，以及 `debug.launch` 权限和 `onDebug:delve` 激活事件；
  `commandInstallers` 里的 `dlv`。
- `e2e/host/desktop/internal/workbench/official_go_plugin_debug_e2e_test.go`：宿主端到端测试。

**依赖的宿主能力**：`debugAdapters` 的 `tcpAnnounced` 传输（宿主启动适配器进程，交给 Core 判断哪一行是地址公告，
再连接公告的本机端口）、`targetBindings.adapterArguments` 的点分路径绑定（Lithe-core-rust#55）、
`commandInstallers`（Core c550c2b，宿主 1lck/Lithe-IDEA#1971）。

**限制**：只支持 launch，不支持 attach 和调试测试；适配器只在打开 `.go` 文件时可选（选择器是 `file` 加 `go` 语言），
从构建窗口的运行入口启动调试还不支持。Delve 在包目录生成 `__debug_bin…` 临时程序，会话结束时删除。
没有 `go.mod` 的目录以 GOPATH 模式构建，只能导入标准库和 `GOPATH/src` 里的包。
适配器名称 Go (Delve) 目前不能翻译：插件语言文件还不覆盖调试适配器。

**最近验证**：2026-11-15，macOS（Apple Silicon），插件 0.1.0 发布包，宿主 `cebbe0def`（1lck/Lithe-IDEA#1958 的分支提交），
Go 1.27.4、Delve 1.27.2。`node scripts/e2e-host.mjs` 通过 `TestOfficialGoPluginDebugsAModuleWithDelve`，覆盖下面验证方法的
第 2 步、第 3 步的断点停下、调用栈顶和局部变量、第 4 步和第 7 步，并确认 Delve 的地址公告没有显示在调试控制台里。
没有覆盖安装对话框里的权限（第 1 步）、单步和表达式求值、调试子包（第 5 步）、缺少 `dlv` 时的提示（第 6 步），也没有做手工验收。

**验证方法**（macOS）：

1. 安装插件包，确认安装对话框列出启动调试器权限；`PATH` 里有 `dlv`。
2. 打开根目录没有 `go.mod`、里面并排放了几个模块的文件夹，打开其中一个模块的 `main.go`，在一行上设断点，
   在「调试」窗口选 Go (Delve) 后点「开始」。
3. 程序停在断点：调用栈顶是 `main.main` 和断点所在行，变量里能看到局部变量的值；单步、表达式求值可用。
4. 点「继续」：程序运行到结束，按相对路径读到包目录里的文件，输出显示在调试控制台，会话自动结束，
   包目录和文件夹根目录里都没有留下 `__debug_bin…`。
5. 调试 `cmd/<名字>` 这样的子包：打开该包里的文件后开始，调试的是这个包；再调试一个没有 `go.mod`、
   由两个文件组成的目录，两个文件一起构建。
6. 没有 `dlv` 时，插件详情「功能」页的调试器卡片显示“未找到 dlv”和安装按钮；安装后开始调试能停在断点。
7. 整个过程中插件安装目录不被写入。

## go-run-configuration-types

**能力**：Go 构建和 Go 测试两种运行配置类型，出现在「运行/调试配置」的「+」菜单里，名称、说明、字段标题和选项来自插件的中文翻译，
图标是插件包里的 Go 地鼠加构建或测试标记（浅色和深色各一份）。
Go 构建按“运行种类”执行 `go run` 一个软件包（导入路径或相对路径）、一个 `.go` 文件或一个目录；Go 测试执行 `go test`
一个软件包模式（默认 `./...`）或一个目录下的所有软件包（`<目录>/...`），可用“模式”作为 `-run` 过滤。两种都有“Go 工具实参”，
程序参数、工作目录、环境变量、启动前步骤、存为项目文件和允许多个实例由宿主统一提供。存为项目文件时写进
`.lithe/run/plugin/io.github.lithe-idea.go-support.toml`。

| 平台 | 实现 | 验证 | 说明 |
| --- | --- | --- | --- |
| macOS | implemented | pending | 需要支持 `runConfigurationTypes` 的桌面端 0.8.6 或更新版本（1lck/Lithe-IDEA#2148，Core Lithe-core-rust#94）；Store 给更早的桌面端继续提供 1.0.1 |
| Windows | implemented | pending | 命令与平台无关，文件和目录以绝对路径传给 `go.exe`；还没有在 Windows 版 Lithe 里跑过 |

**证据**：

- `io.github.lithe-idea.go-support/plugin.json`：`buildSystems` 里 `go` 的 `runConfigurationTypes`（`build`、`test`）和
  `requires.runConfigurationTypes`；`l10n/zh-Hans.json` 的 `runConfigurationTypes.go`；两种类型的 `icon`
  指向 `icons/go-build.svg`、`icons/go-test.svg` 和各自的 `-dark` 文件，出处见 `icons/NOTICE`。
- `scripts/test.mjs`：按契约的展开规则把两种类型的字段拼成命令，用真实的 go 运行软件包、文件、目录的 Go 构建，
  以及软件包、目录、模式过滤的 Go 测试。
- `e2e/host/desktop/internal/workbench/official_go_plugin_run_types_e2e_test.go`：宿主端到端测试，包括两种类型都读到包里的浅色和深色图标。

**依赖的宿主能力**：`runConfigurationTypes`（API-29，Core Lithe-core-rust#94 的表单、保存和 `build.runConfigurationPlan`；
桌面端 1lck/Lithe-IDEA#2148 的「+」菜单、插件字段和翻译、`.lithe/run/plugin/<ID>.toml`、“允许多个实例”）；
类型图标需要桌面端 0.8.7 或更新版本（1lck/Lithe-IDEA#2169）读取插件包里的图标，桌面端 0.8.6 显示按执行类别的通用图标。

**限制**：

- 不能调试：宿主拒绝调试插件声明的运行类型，调试用「调试」窗口的 Go (Delve)。
- 字段排在宿主的工作目录和环境变量前面，选项是分段按钮而不是下拉框；目录字段不能有默认值，Go 测试的目录不会默认填项目根目录；
  名称不会按所选软件包自动生成。
- 还没有的字段：输出目录、构建后运行、模拟终端、重定向输入、以 sudo 运行、模块、测试框架、使用所有自定义构建标记。

**最近验证**：2026-11-18，macOS（Apple Silicon），插件 1.1.0 发布包，宿主 `a8797ae`（1lck/Lithe-IDEA main，包含 #2148），
Go 1.27.4。`node scripts/e2e-host.mjs` 通过 `TestOfficialGoPluginRunsGoBuildAndGoTestConfigurations`：两种类型以“Go 构建”
“Go 测试”列出；在对话框里新建 Go 构建，运行种类默认软件包、只显示软件包路径，填 `./cmd/echo` 和程序参数 `one two`
后运行，输出 `echo: one|two`；新建 Go 测试，切到目录后显示目录、隐藏软件包路径，填项目根目录、模式 `^TestGreet$` 和
`-v -count=1` 后运行，只跑了 TestGreet；两条配置都写进插件的运行文档，安装目录逐字节不变。`scripts/test.mjs` 另外用真实的 go
覆盖文件和目录的 Go 构建、软件包模式的 Go 测试。没有覆盖存为项目文件后重新打开项目、允许多个实例、启动前步骤，
也没有做下面的手工验收。

类型图标：2026-11-18，macOS（Apple Silicon），本地打包的插件 1.2.0，宿主 `0ffbbd6`（1lck/Lithe-IDEA#2169 分支，
Core `0a15a98`），Go 1.27.4。`node scripts/e2e-host.mjs` 的三个 workbench 测试和五个 projectgen 测试全部通过；
`TestOfficialGoPluginRunsGoBuildAndGoTestConfigurations` 另外检查两种类型都读到包里的浅色和深色图标、两套配色不同。
2026-11-19 `e2e/host/lithe-host.json` 改固定到桌面端 0.8.8 的 `d87f114` 后，用 1.3.0 发布包重跑，四个 workbench 测试和五个 projectgen
测试全部通过，图标检查也在其中。

**验证方法**（macOS）：

1. 安装插件包，打开含 `go.mod` 的项目，打开「运行/调试配置」点「+」：内置类型之后有 Go 构建和 Go 测试，说明清楚；
   图标是 Go 地鼠加锤子和 Go 地鼠加测试标记，在浅色和深色外观下都清楚，配置树和标题栏的运行配置列表里也是这两个图标。
2. 新建 Go 构建：运行种类在软件包、文件、目录间切换时只显示对应的一项；三种分别选 `main` 包、`main.go` 和包目录后运行，
   程序输出显示在 Run 窗口里，程序参数传给程序；Go 工具实参填 `-race` 时命令里带上它。
3. 新建 Go 测试：默认 `./...` 运行全部测试；改成某个包并填模式后只跑匹配的测试；切到目录后测试这个目录下的所有包；
   测试失败时 Run 窗口显示失败和非零退出码。
4. 勾选存为项目文件后，`.lithe/run/plugin/io.github.lithe-idea.go-support.toml` 里有这条配置；关闭并重新打开项目后配置还在，
   文件和目录字段仍指向原来的路径。
5. 勾选允许多个实例后连续运行两次，Run 窗口有两个标签；不勾选时第二次运行先停掉第一次。
6. 在构建选项里设 Build Tags 后运行两种配置，命令带上 `-tags`。
7. 整个过程中插件安装目录不被写入。

## go-project-generator

**能力**：「新建项目」对话框里的 Go。GOROOT 下拉列出 `go` 工具定义在电脑上找到的 Go，默认是版本最新的一个，
创建后记为项目固定，构建和运行使用它；没有找到 Go 时留空，不影响创建。生成 `go.mod`（`module` 行是模块路径，
默认是小写的项目名；`go` 行默认是所选 GOROOT 的版本，手动改过后不再跟随）和 `.gitignore`，可选生成打印
`Hello, Gopher!` 的示例 `main.go`。`go.mod` 由模板写出，不运行 `go mod init`，创建项目不需要联网，也不需要先装好 `go`。

| 平台 | 实现 | 验证 | 说明 |
| --- | --- | --- | --- |
| macOS | implemented | pending | |
| Windows | implemented | pending | 模板与平台无关；GOROOT 下拉靠 `bin/go.exe` 识别 Go |

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

## go-settings-previews

**能力**：插件后端（`bin/backend`，Rust）实现 `settings.view` 视图函数，给两个 `settingRegions` 画画布：

- `language`（Go 语言设置页）：四张卡片分别画悬停文档（按 `hoverKind` 显示完整、摘要、单行或只有签名）、参数占位符（`usePlaceholders`）、
  gofumpt 格式化（`gofumpt` 开关前后的代码对比）和 Staticcheck 分析（`staticcheck` 开启后多出的问题）在编辑器里的样子。
- `toolchain`（Go 构建系统页）：Lithe 查找 go 的顺序（设置的 `go` 路径、项目固定、全局默认、`GOROOT`、`PATH`、常见位置），
  设置了 `go` 路径时只有这一步“使用中”，其余“不使用”。

两个区域都读设置窗口里还没应用的草稿，草稿和已保存的值不同时预览先变、并标出“含未应用的修改”。画面中英文跟随设置窗口的语言，
浅色和深色外观用宿主的颜色标记或成对颜色；点亮的标签（“开”“一行”等）在选中底色上用 `$text` 写字，深色外观下也清楚。
后端只读设置，不启动 gopls、go 或 dlv，也不写插件目录。

1.4.0 起场景按视图事件里的 `width` 排版：语言页放得下两张最窄的卡片（宽度至少 594）时两张一行，否则一张一行，卡片随宽度变宽，
文字和代码保持原来的字号；构建系统页的步骤卡片随宽度变宽。画布声明 `on: ["resize"]`，窗口宽度稳定后宿主发 `resize`，
后端按新宽度重画。事件里没有 `width` 时按最窄设置窗口的区域宽度画（语言页 610、构建系统页 340），也不声明 `resize`，和 1.3.0 相同。

**证据**：

- `io.github.lithe-idea.go-support/plugin.json`：`settingRegions` 的 `language`、`toolchain`，`backend.launch`，`requires` 的 `backend: 0`、`ui: 0`，
  权限 `backend.launch`、`ui.contribute`。
- `backend/src/`：视图函数和场景；`cargo test --locked` 检查每个场景在两种语言、各个宽度下都在契约限制内、viewBox 宽度等于事件宽度、
  卡片内容不超出卡片，两张一行只在放得下两张最窄卡片时出现，没有宽度时退回最窄窗口的排版且不声明 `resize`，
  以及各设置对预览的影响。CI 不跑 `cargo test`，只构建后端。
- `e2e/host/desktop/internal/workbench/official_go_plugin_settings_view_e2e_test.go`：经设置窗口自己的设置来源启动安装包里的后端，
  用中英文打开两个区域并发草稿事件，事件带区域宽度，由 Core 校验每次回答；检查 viewBox 宽度等于区域宽度、画布高度放得下 viewBox、
  不写 `pan` 并声明 `resize`；再以 760 的宽度发 `resize`，检查按新宽度重画且草稿标记仍在。

**依赖的宿主能力**：插件视图和设置区域（1lck/Lithe-IDEA#2183，Core Lithe-core-rust#104），桌面端 0.8.7 起包含。桌面端 0.8.6 同样支持 `ui` 0 和 `backend` 0，Store 按 `requires` 挡不住它，但它不认识 `settingRegions`，装 1.3.0 及更新版本会失败。
按宽度排版和 `resize` 需要视图事件带 `width` 的宿主（1lck/Lithe-IDEA#2191，固定 Core Lithe-core-rust#109），还没有桌面端版本包含：
这种宿主量好区域宽度才发 `open`，宽度稳定 400 ms 后给声明了 `resize` 的画布发 `resize`，画布不写 `pan` 时按
`min(宽/viewBox 宽, 高/viewBox 高)` 从左上角显示整个 viewBox。桌面端 0.8.7、0.8.8 不发宽度，并按“区域宽/viewBox 宽”缩放画布，
宽窗口里画面跟着放大、下方被画布高度裁掉；1.4.0 在这两个版本上和 1.3.0 相同。

**限制**：代码按 SF Mono 的 7pt 字宽一个字符一个节点排版，选区、光标、输入框和波浪线才能对准字符；Windows 上 Consolas 的字更窄，
代码间距会比宿主自己画的略宽。文字宽度按字号估算，不是宿主量出来的；卡片窄时较长的说明和诊断用省略号截断。

**最近验证**：2026-11-19，macOS（Apple Silicon），本地构建的 `go-support-1.4.0-darwin.zip`（未发布），宿主 `312a9c63a`
（1lck/Lithe-IDEA#2191 的分支提交，固定 Core `0b6da27`）。`TestOfficialGoPluginDrawsItsSettingsRegions` 通过：后端由宿主启动，
两个区域在中英文下按宿主给的区域宽度各画出一张画布，声明 `resize`、不写 `pan`，标题和标签是对应语言，草稿让预览标出未应用的修改、
让构建系统页的 `go` 路径一步变成“使用中”；以 760 的宽度发 `resize` 后按新宽度重画，草稿标记仍在；安装目录的文件不变。
同一次运行里另外三个 workbench 测试和五个 projectgen 测试也通过。另把后端画出的场景放进宿主无头界面（设置窗口的浅色和深色配色）截图：
语言页 330、610、900，构建系统页 344、700，中英文、默认值和全部开启，共 40 张，没有裁切和重叠，深色下点亮的标签文字清楚。
此前同一天 1.3.0 发布包在桌面端 0.8.8（`d87f114`）上通过同一测试。没有做手工验收。

**验证方法**（macOS）：

1. 用包含 #2191 的桌面端安装插件包，打开「设置 › 语言 › Go」：顶部出现四张预览卡片，浅色和深色外观下文字、代码和点亮的标签都清楚。
   拖宽、拖窄设置窗口，停下后预览按新宽度重新排版：放得下时两张一行，窄了一张一行，字号不变，不放大也不被裁掉。
2. 改悬停文档、参数占位符、gofumpt、Staticcheck，先不点应用：对应卡片立刻变化，标出“含未应用的修改”；点应用后标记消失。
3. 打开「设置 › 构建、执行、部署 › 构建工具 › Go」：预览列出查找 go 的顺序；填 `go` 路径后这一步变成“使用中”，其余“不使用”。
   拖宽窗口后步骤卡片跟着变宽，较长的 `go` 路径显示得更多。
4. 界面语言换成中文，两个预览都显示中文。

## Windows 安装包

| 能力 | Windows ARM64 | Windows x64 | 验证入口 |
| --- | --- | --- | --- |
| 安装包 | 已实现（`-windows-arm64.zip`，带 ARM64 的 `bin/backend.exe`） | 已实现（`-windows-x64.zip`，带 x64 的 `bin/backend.exe`） | `scripts/package/archives.test.mjs`：清单只改了后端启动路径，后端是该架构的 PE 程序并带许可证清单，`go.exe` 布局，条目名在 Windows 上可解压 |
| 设置预览后端 | CI 在 `windows-latest` 上用 MSVC ARM64 工具交叉编译 | CI 在 `windows-latest` 上编译 | `.github/workflows/ci.yml` 的 `backend` 任务；还没有在 Windows 版 Lithe 里打开过设置预览 |
| 真实 go、gopls 流程 | [已通过原生 CI](https://github.com/Lithe-IDEA/lithe-plugin-go/actions/runs/43728078241)（`windows-11-arm`） | [已通过原生 CI](https://github.com/Lithe-IDEA/lithe-plugin-go/actions/runs/43728078241)（`windows-latest`） | `.github/workflows/ci.yml` 的 `commands` 任务跑 `scripts/test.mjs`，`LITHE_TEST_PLUGIN_DIR` 指向解压到带空格路径的 Windows 包 |
| Delve 调试 | 不支持：Delve 上游没有 windows/arm64 后端，`scripts/test.mjs` 把 dlv 检查记为跳过 | [已通过原生 CI](https://github.com/Lithe-IDEA/lithe-plugin-go/actions/runs/43728078241)（托管安装和 `dlv dap` 端点公告） | 同上 |

包里除了声明资源（清单、模板、图标），只有画设置预览的后端，所以 ARM64 与 x64 各有一份包，只有 `bin/backend.exe` 不同；Go、gopls 和 dlv 仍是用户自己的。
Delve 目前不能在 Windows ARM64 上构建，所以那里没有调试；Delve 加入支持后，同一份包不改就能用，测试也会自动改为实际检查。
`scripts/test.mjs` 按清单逐条运行版本探测、模型和依赖命令、构建、vet、过滤后的测试、`go env` 目录、gopls 和 dlv 的托管安装、
gopls 的校验参数和 `dlv dap` 的端点公告。这些只是命令层面的自动化测试，不能代替 Lithe 里的验收，验证列保持 `pending`。
