# Go Support for Lithe

为 [Lithe](https://github.com/1lck/Lithe-IDEA) 提供 Go 支持：用你电脑上的 `gopls` 提供代码智能，
用声明式构建系统接口（`buildSystems`）识别 Go 模块，在构建窗口里列出包，运行 `go build`、`go vet`、
`go test` 和 `go run`，查看模块依赖；在「运行/调试配置」里新建 Go 构建和 Go 测试配置；用你电脑上的 Delve（`dlv`）调试，
在「新建项目」里创建 Go 模块。
这是一个**社区插件**，按插件契约 v0 编写。插件包含一份 `plugin.json`、几份项目模板，和一个只画设置页预览的小后端（Rust 编写）；
gopls、go 和 dlv 仍然用你电脑上的。

> English: Go support for Lithe — code intelligence through your own `gopls`, the Go Modules build system
> (package model, build, vet, test, run and the module list) through the declarative `buildSystems` contract,
> Go Build and Go Test run configurations, debugging through your own Delve, and a Go module entry in New Project.
> The plugin is a `plugin.json`, a few project templates and a small Rust backend that only draws the settings
> previews; gopls, go and dlv stay your own.

> **兼容性**：按插件契约 v0（`schemaVersion: 0`、`pluginAPIVersion: 0`）编写，只能装进新版 Lithe 桌面端
> （`desktop-v*` 预发布），旧版 macOS Lithe 无法安装。1.0.0 的调试需要宿主支持 `tcpAnnounced` 调试适配器
> （[1lck/Lithe-IDEA#1954](https://github.com/1lck/Lithe-IDEA/pull/1954)），`go` 工具定义需要固定了 Rust Core
> 6477e9a 或更新版本的桌面端（[1lck/Lithe-IDEA#1964](https://github.com/1lck/Lithe-IDEA/pull/1964)），
> 在插件详情里安装 gopls 和 dlv 需要固定了 Core c550c2b 或更新版本、支持 `commandInstallers` 的桌面端
> （[1lck/Lithe-IDEA#1971](https://github.com/1lck/Lithe-IDEA/pull/1971)）。1.0.1 起作为正式版本发布并收录到 Lithe Store；
> 没有这些能力的桌面端会从 Store 装到 0.0.1。语言服务器的 `indexingExclusions` 声明需要固定了包含
> [Lithe-core-rust#68](https://github.com/Lithe-IDEA/Lithe-core-rust/pull/68) 的 Core 的桌面端，
> 更早的桌面端不认识这个字段，会拒绝整份清单。1.1.0 的 Go 构建和 Go 测试运行配置需要支持插件声明运行配置类型的桌面端
> （[1lck/Lithe-IDEA#2148](https://github.com/1lck/Lithe-IDEA/pull/2148)，固定了
> [Lithe-core-rust#94](https://github.com/Lithe-IDEA/Lithe-core-rust/pull/94) 的 Core），桌面端 0.8.6 起包含它。
> 1.1.0 作为正式版本发布并收录到 Lithe Store；Store 按版本的 `requires` 给更早的桌面端继续提供 1.0.1。
> 1.2.0 的 Go 构建和 Go 测试类型图标需要能读取插件包图标的桌面端（[1lck/Lithe-IDEA#2169](https://github.com/1lck/Lithe-IDEA/pull/2169)），
> 桌面端 0.8.7 起包含它；桌面端 0.8.6 也能安装 1.2.0，只是显示通用图标。
> 1.3.0 在 Go 语言设置页和 Go 构建系统页加了插件自己画的预览区域（`settingRegions`），需要支持插件视图的桌面端
> （[1lck/Lithe-IDEA#2183](https://github.com/1lck/Lithe-IDEA/pull/2183)，固定了
> [Lithe-core-rust#104](https://github.com/Lithe-IDEA/Lithe-core-rust/pull/104) 的 Core），桌面端 0.8.7 起包含它。
> 1.2.0 和 1.3.0 都作为正式版本发布并收录到 Lithe Store。桌面端 0.8.6 也支持 `ui` 0 和 `backend` 0，
> Store 按 `requires` 挡不住它，但它不认识 `settingRegions`，安装或更新到 1.3.0 会失败，需要先把桌面端更新到 0.8.7 或更新版本。
> 1.3.1 让这两个预览按设置窗口的实际宽度重新排版，窗口宽度变化后跟着重排；这要靠告诉插件视图宽度的桌面端
> （[1lck/Lithe-IDEA#2191](https://github.com/1lck/Lithe-IDEA/pull/2191)，固定了
> [Lithe-core-rust#109](https://github.com/Lithe-IDEA/Lithe-core-rust/pull/109) 的 Core），还没有桌面端版本包含它。
> 桌面端 0.8.7、0.8.8 不发送宽度，1.3.1 在上面按最小设置窗口排版，和 1.3.0 相同，所以 1.3.1 作为正式版本发布。

## 功能

| 功能 | 说明 |
| --- | --- |
| 代码智能 | 补全、跳转定义、悬停、诊断、格式化，由 `gopls` 提供 |
| 识别模块 | 含 `go.mod` 的目录是构建根；`go list -json ./...` 的结果组成模块和包 |
| 构建 | Build（`go build ./...`）作用于模块；Vet（`go vet`）可作用于模块或单个包 |
| 运行 | 每个 `main` 包在构建窗口里有「运行 <包名>」按钮，执行 `go run <导入路径>`，输出显示在构建输出里 |
| 测试 | `go test` 作用于模块或单个包，可用 `-run` 只跑一个测试；输出显示在构建输出里，插件不提供测试报告文件 |
| 运行配置 | 「运行/调试配置」的「+」里有 Go 构建（`go run` 一个软件包、文件或目录）和 Go 测试（`go test` 一个软件包或目录，可按模式过滤），可存为项目文件、允许多个实例，见下文“运行配置” |
| 诊断 | 把 `文件:行:列: 消息` 形式的编译输出显示为问题 |
| 依赖 | `go list -m -json all` 列出模块依赖（含版本和是否间接依赖） |
| 同步 | `go.mod`、`go.sum`、`go.work` 变化后重新读取模块 |
| 调试 | 「调试」窗口里选 Go (Delve)，调试当前打开的 `.go` 文件所在的包：断点、单步、调用栈、变量和表达式求值，程序输出显示在调试控制台 |
| 新建项目 | 「新建项目」里的 Go：选择 GOROOT，生成 `go.mod`、`.gitignore`，可选生成示例 `main.go`；模块路径默认是小写的项目名，Go 版本默认是所选 Go 的版本 |
| 工具链 | 在常见位置找到电脑上的 Go 安装；项目可以固定用其中一个构建和运行 |
| 设置预览 | Go 语言设置页顶部的预览画出悬停文档、参数占位符、gofumpt 格式化和 Staticcheck 分析在编辑器里的效果；Go 构建系统页画出 Lithe 查找 go 的顺序和当前用的是哪一步。改了还没应用的选项会先在预览里显示，并标出“含未应用的修改” |

## 设置和构建选项

**插件设置**（设置 → 插件 → Go Support →「设置」）：

| 设置 | 生效方式 | 说明 |
| --- | --- | --- |
| Go executable | 重新读取模块 | 读取模块、构建、检查、测试和运行用的 `go`。留空时使用「工具链」选出的 Go（见下）。也显示在「构建、执行、部署 › Go Modules」页 |
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

**工具链**（设置 →「构建、执行、部署」→「工具链」）：插件声明了 `go` 工具，Lithe 在下面这些位置查找 Go，
用 `go version` 读出版本，列在这里：

| 平台 | 位置 |
| --- | --- |
| macOS | `/usr/local/go`（官方安装包），Homebrew 的 `go` 和 `go@1.N`（`/opt/homebrew/Cellar`、`/usr/local/Cellar`） |
| Linux | `/usr/local/go`，`/usr/lib/go`、`/usr/lib/go-1.N`，Linuxbrew |
| Windows | `C:\Program Files\Go`，Scoop |
| 所有平台 | `~/sdk/go*`（`golang.org/dl` 下载的版本），`~/go/go*` |

没有设置 Go executable 时，构建、检查、测试和运行按顺序使用第一个可用的 Go：项目固定的 Go、全局默认的 Go、
`GOROOT` 环境变量指向的 Go、`PATH` 里的 `go`、找到的最新安装；都没有时使用 `PATH` 里的 `go`。
选中的 Go 的 `bin` 目录加到命令的 `PATH` 前面，`GOROOT` 设为它的安装目录。

## 调试

打开「调试」窗口，适配器选 **Go (Delve)**，打开要调试的 `.go` 文件，在行号旁设好断点后点「开始」。

- 调试的是当前文件所在的包：Delve 在这个包的目录里构建它，再在断点处停下。所以要调试 `cmd/server` 的程序，
  打开 `cmd/server` 里的任意文件即可。
- 打开的文件夹里可以并排放好几个项目，根目录不需要 `go.mod`：Delve 从包所在的目录往上找它所属的模块。
- 没有 `go.mod` 的目录也能调试，目录里的所有 `.go` 文件一起构建；这时 Delve 以 `GO111MODULE=auto` 构建，
  只能导入标准库和 `GOPATH/src` 里的包。
- 程序的工作目录是包所在的目录，所以 `os.ReadFile("examples/input.txt")` 这样的相对路径从包目录算起。
- 程序结束后会话自动结束，程序的输出留在调试控制台里。
- Delve 在包所在的目录生成名为 `__debug_bin…` 的临时程序，会话结束时自己删除；新建项目生成的 `.gitignore`
  已经忽略它。
- 目前只支持启动调试（launch），不支持附加到运行中的进程，也不支持调试测试。

## 运行配置

打开「运行/调试配置」，点「+」，选 **Go 构建** 或 **Go 测试**。名称、存为项目文件、允许多个实例、程序参数、
工作目录、环境变量和启动前步骤由 Lithe 统一提供，下面是插件自己的字段：

**Go 构建**：执行 `go run [Go 工具实参] <要运行的内容> [程序参数]`，程序输出显示在 Run 窗口里。

| 字段 | 说明 |
| --- | --- |
| 运行种类 | 软件包、文件或目录，决定下面显示哪一项，默认是软件包 |
| 软件包路径 | main 软件包的导入路径（如 `example.com/app/cmd/server`），或相对工作目录的路径（如 `./cmd/server`） |
| 文件 | main 软件包里的一个 `.go` 文件，`go run` 只构建这个文件 |
| 目录 | main 软件包所在的目录 |
| Go 工具实参 | 放在要运行的内容前面传给 `go run`，例如 `-mod=vendor`、`-race` |

**Go 测试**：执行 `go test [Go 工具实参] [-run <模式>] <软件包或目录> [程序参数]`。

| 字段 | 说明 |
| --- | --- |
| 测试种类 | 目录或软件包，默认是软件包 |
| 软件包路径 | 导入路径，或相对工作目录的软件包模式，默认 `./...`（工作目录下的所有软件包） |
| 目录 | 测试这个目录及其子目录里的所有软件包（传给 go 的是 `<目录>/...`） |
| 模式 | 要运行的测试的正则表达式，作为 `-run` 传入，例如 `^TestName$`；留空运行所有测试 |
| Go 工具实参 | 放在软件包前面传给 `go test`，例如 `-v`、`-count=1` |

文件和目录在项目里存为相对项目根目录的路径，运行时传给 go 的是绝对路径。存为项目文件的配置写在
`.lithe/run/plugin/io.github.lithe-idea.go-support.toml`，可以提交到仓库。两种配置用的 `go` 和构建窗口相同
（见上文“工具链”），构建选项里的 Build Tags 和 Race Detector 也会加进命令。目前还不能调试这两种配置，
调试请用「调试」窗口。

## 新建项目

「新建项目」对话框左侧有 **Go**。可以填写：

| 字段 | 说明 |
| --- | --- |
| GOROOT | 项目构建和运行用的 Go，从「工具链」找到的安装里选，默认是版本最新的一个。创建后项目固定使用它（记在项目的 `.lithe/toolchains/local.json`），以后可以在「工具链」里修改。没有找到 Go 时留空，不影响创建 |
| 添加示例代码 | 默认开启，生成打印 `Hello, Gopher!` 的 `main.go`，创建后打开它；关闭时只生成模块文件，创建后打开 `go.mod` |
| 模块路径 | 写进 `go.mod` 的 `module` 行，别的代码用它导入本模块，例如 `example.com/hello` 或 `github.com/you/hello`。默认是小写的项目名；项目名里有空格时需要改掉 |
| Go 版本（高级） | 写进 `go.mod` 的 `go` 行，表示模块至少需要的 Go 版本，默认是所选 GOROOT 的版本，换 GOROOT 时跟着变，手动改过后不再变；更新的 Go 也能构建。没有选 GOROOT 时需要手动填写，高级选项会自动展开 |

`go.mod` 由模板直接写出，不运行 `go mod init`，所以创建项目不需要联网，也不需要先装好 `go`。
GOROOT 只能选 Go 1.21 或更新的版本；选了更旧的版本时表单会提示，不能创建。
这些字段目前只有英文名称：插件语言文件还不能翻译新建项目的字段。

## 依赖的程序

| 程序 | 用途 | 查找方式 |
| --- | --- | --- |
| `go` 1.21 或更新版本 | 读取模块、构建、检查、测试、运行、依赖 | 插件设置 Go executable；留空时用「工具链」选出的 Go，最后找 `PATH` 里的 `go` |
| `gopls` | 代码智能 | 先找在 Lithe 里安装时记下的目录，再找 `PATH` 里的 `gopls` |
| `dlv`（Delve，需要有 `dlv dap` 命令，已用 1.27.2 验证） | 调试 | 先找在 Lithe 里安装时记下的目录，再找 `PATH` 里的 `dlv`；Delve 构建程序时用 `PATH` 里的 `go` |

安装插件时 Lithe 会列出这些程序和插件需要的权限（启动语言服务器、运行构建命令、启动调试器、创建项目、
安装插件所需的命令行工具），确认后才会安装。

`go` 需要自己安装，例如 `brew install go`。gopls 和 dlv 可以在 Lithe 里装：

1. 打开 **设置 → 插件 → 已安装**，选中 Go Support，切到 **功能**。
2. 缺少的程序会在对应能力卡片上显示“未找到 gopls”或“未找到 dlv”，旁边有 **安装** 按钮。
3. 点击后选择安装位置，核对要运行的命令，确认后开始安装，输出显示在对话框里：
   - **Lithe 管理的目录**：运行 `go install <包>@latest` 时把 `GOBIN` 设为插件自己的状态目录。
     不改动 Go 的默认目录，卸载插件时一起删除。
   - **Go 的默认位置**：不设 `GOBIN`，装进 `go env GOBIN`；`GOBIN` 为空时装进 `$(go env GOPATH)/bin`。
     卸载插件时不删除这些文件。

两种位置装好的命令都会被记下，之后启动 gopls 和 Delve 时直接用，不需要再改 `PATH`。安装需要联网，
用的是「构建、执行、部署 › 工具链」选出的 Go（打开的项目固定了 Go 时用那一个），需要 1.21 或更新版本。也可以自己在终端安装：

```bash
go install golang.org/x/tools/gopls@latest
go install github.com/go-delve/delve/cmd/dlv@latest
```

在终端安装时，`$(go env GOPATH)/bin` 需要在 Lithe 能看到的 `PATH` 里。不调试的话可以不装 dlv。

## 安装

1. 从 [Releases](https://github.com/Lithe-IDEA/lithe-plugin-go/releases) 下载对应平台的包：macOS 用 `go-support-<版本>-darwin.zip`，
   Windows x64 用 `go-support-<版本>-windows-x64.zip`，Windows ARM64 用 `go-support-<版本>-windows-arm64.zip`。
2. 在 Lithe 中打开 **设置 → 插件**，点击页面标题栏右侧的 **从文件安装…**，选中这个 zip，核对 SHA-256 和权限后确认。
3. 打开含 `go.mod` 的项目。构建窗口会出现 Go Modules，打开 `.go` 文件时启动 gopls。

darwin 包同时支持 Apple Silicon 和 Intel Mac，windows 包同时支持 Windows ARM64 和 x64。两个包字节相同：
插件只声明 Lithe 怎样启动用户自己的 go、gopls 和 dlv，Windows 上从 GOROOT 解析 `bin/go.exe`，
`gopls` 和 `dlv` 按 PATHEXT 找到 `.exe`。Delve 本身还不支持 Windows ARM64，那里没有调试，其他功能可用。

## 工作方式

- 读取模块、构建、检查、测试、运行和依赖都是 `plugin.json` 里声明的 `go` 命令，由 Lithe 解析 `go` 并运行。
- 运行入口由 Lithe 从模型里的 `main` 包生成：每个带 `application` 标签的包一条，名称是导入路径的最后一段。
- gopls 由 Lithe 按 `languageServers` 的声明启动，插件设置通过 `workspaceConfiguration` 里的
  `${setting:<id>}` 传给它。
- 用户排除在索引之外的目录（以及被排除的 Git 忽略目录），由 Lithe 按 `indexingExclusions` 的声明追加到 gopls
  读到的 `gopls.directoryFilters` 末尾：每个目录写成 `-<相对路径>`，工作区根目录写成 `-`；配置里没有
  `directoryFilters` 时从 gopls 的默认值 `-**/node_modules` 开始。目录名含 `*`、`?`、`[`、`]`、`{`、`}`
  的不写入，以免误排除别的目录。排除的目录变化时，正在运行的 gopls 会收到新的配置。
- Delve 由 Lithe 按 `debugAdapters` 的声明启动为 `dlv dap`。Delve 在标准输出打印
  `DAP server listening at: <地址>`，Lithe 读到这一行后连接这个本机地址（`tcpAnnounced` 传输）。
  调试的包由当前文件所在目录决定（`target.directory` 绑定到 Delve 的 `program`）。
- Go 的查找方式由 `toolDefinitions` 里的 `go` 声明：查找位置、`bin/go` 的位置、用 `go version` 读版本、
  `GOROOT` 环境变量。构建系统的 `go` 先看 Go executable 设置，再交给这个工具定义解析。
- 新建项目由 `projectGenerators` 声明，Lithe 把 `templates/go/` 里的模板展开到新目录；
  模板名以 `dot-` 开头的文件生成为点文件。GOROOT 是 `tool` 类型的字段，Go 版本的默认值是
  `${field.goroot.version}`；创建后 Lithe 把所选 GOROOT 记为项目固定（`pinAs: "project"`）。
- 插件目录在运行时保持只读。

## 诊断日志

插件没有自己的进程，也不发送 `lithe/log`。gopls 由 Lithe 启动，它的启动结果、`window/logMessage`
和有界的 stderr 由 Lithe 记进语言服务的日志；`go` 命令的输出显示在构建输出里，不复制进诊断日志。
Delve 也由 Lithe 启动：每次调试的适配器启动结果（解析、启动、超时、退出、initialize 失败或成功）由 Lithe
记在插件的日志通道里；Delve 和被调试程序的输出显示在调试控制台里，不复制进诊断日志。
排障时使用 Lithe 的日志窗口和诊断导出，不要开启 gopls 的 RPC dump 或 Delve 的 `--log`，
也不要把源码、环境变量或完整命令行写进日志。

## 本地打包

先构建画设置预览的后端，需要 Rust 工具链和对应的 target（`rustup target add`）：macOS 上构建 darwin（Apple Silicon 和 Intel
合成一个通用二进制并做 ad-hoc 签名），Windows 上构建 windows-x64 和 windows-arm64。再用 Node.js 22+ 和 `zip`（macOS 或 Linux）打包：

```bash
node scripts/build-backend.mjs darwin          # 写到 .artifacts/backend-darwin/
node scripts/package.mjs --platforms darwin    # CI 和发布时三个平台都打
```

输出 `dist/go-support-<版本>-<平台>.zip` 和它们的 SHA-256。每个包带 `bin/backend`（Windows 是 `bin/backend.exe`，
包里的 `plugin.json` 相应改写启动路径）和列出后端所用 crate 许可证的 `THIRD_PARTY_LICENSES.txt`。本地开发时，用 **设置 → 插件 → 从文件安装…**
选择这个 zip 安装。

```bash
node --test scripts/package/*.test.mjs   # 核对 dist/ 里的每个包
node scripts/test.mjs                     # 用真实的 go、gopls、dlv 跑清单里的命令
```

`scripts/test.mjs` 在带空格的临时目录里按 Go 项目模板生成模块，逐条运行版本探测、模型、依赖、构建、vet、
过滤后的测试和 `go env` 目录，再把 gopls 和 dlv 托管安装到独立的 `GOBIN`，检查 gopls 的校验参数和
`dlv dap` 的端点公告。`LITHE_TEST_PLUGIN_DIR` 指向解压后的发布包时测发布包，`LITHE_TEST_INSTALL=0` 跳过需要联网的安装。
CI 在 macOS、Windows x64 和 Windows ARM64 上跑这个脚本，Windows 上测的是解压到带空格路径的 windows 包。

`plugin.json` 的完整格式见 Rust Core 仓库里的
[插件契约](https://github.com/Lithe-IDEA/Lithe-core-rust/blob/main/shared/plugins/contract/v1/README.md)、
[构建工具接入指南](https://github.com/Lithe-IDEA/Lithe-core-rust/blob/main/shared/plugins/guides/build-tool-integration.md)
和[语言服务器插件生命周期](https://github.com/Lithe-IDEA/Lithe-core-rust/blob/main/shared/plugins/guides/lsp-plugin-lifecycle.md)。

## 宿主端到端测试

`e2e/host/` 里的测试通过 Lithe 桌面端自己的测试工具（宿主的 `desktop/internal/workbench` 包）安装真实插件包，
像用户一样操作构建窗口和编辑器。这些测试原来放在 Lithe 主仓库，按“宿主不依赖插件”的要求
（[1lck/Lithe-IDEA#1915](https://github.com/1lck/Lithe-IDEA/pull/1915)）移到这里：主仓库不读取本仓库，
`e2e/host/lithe-host.json` 记录测试最后一次通过时的宿主版本。

需要 Node.js 22+、Go、Rust 工具链（构建宿主固定的 Rust Core）、`git`，以及测试要用的 `go`、`gopls` 和 `dlv`：

```bash
node scripts/build-backend.mjs darwin
node scripts/package.mjs --platforms darwin
LITHE_TEST_GO_PLUGIN_ARCHIVE="$PWD/dist/go-support-1.3.1-darwin.zip" node scripts/e2e-host.mjs
```

插件包路径要写绝对路径。脚本把固定版本的宿主（只取 `desktop/` 和 `scripts/`）检出到 `.artifacts/lithe-host`，
通过 Go overlay 加载 `e2e/host/desktop/` 下的测试，再运行宿主的 `scripts/desktop/desktop-go.sh test`；第一次运行会克隆并构建
Rust Core，`LITHE_CORE_RUST_DIR` 可以指向本地的 Core 检出。`--ref <分支或 commit>` 换一个宿主版本试跑，
`--run <正则>` 只跑部分测试。测试用的是宿主内部的测试工具，升级 `lithe-host.json` 的 `revision` 时可能要同步修改测试。

远程测试需要支持远程插件的宿主版本，以及登录环境里有 `go`、`gopls` 和 `dlv` 的一次性 Linux SSH 目标。使用与客户端平台匹配的插件包和该宿主的 `scripts/desktop/build-remote-server.sh` 生成的服务产物。Windows 上用 Git Bash 启动测试，插件包和路径按 Windows 客户端填写。

```bash
LITHE_TEST_GO_SSH_HOST=my-linux-test-host \
  LITHE_REMOTE_SERVER_STAGE=/absolute/path/to/remote-server \
  node scripts/e2e-host.mjs --remote --ref "$LITHE_HOST_REF"
```

`LITHE_HOST_REF` 是要验证的宿主分支或提交。也可用 `--worktree /absolute/path/to/Lithe-IDEA` 替换 `--ref "$LITHE_HOST_REF"`，直接测试该检出的当前文件。测试入口在 `.artifacts/` 生成一次性的 Go overlay，结束后移除，不向宿主检出复制测试源码。

用例覆盖远端构建、运行和测试，gopls 诊断与保存，以及 Delve 断点、栈帧跳转和调试资源清理。

## 功能矩阵

插件各项能力在 macOS 和 Windows 上的实现与验收状态见 [docs/feature-matrix.md](docs/feature-matrix.md)。
Lithe 主仓库的功能矩阵不收录插件能力，改动插件行为时在同一个 PR 里更新这一页。

## 发布新版本

1. 修改 `io.github.lithe-idea.go-support/plugin.json` 的 `version`：修复升修订号（1.1.0 → 1.1.1），
   加功能升次版本号（1.1.0 → 1.2.0）；只有特别大的革新才升主版本号，由维护者决定。新功能要求还没有发布的
   宿主能力或 Core 版本时，照样升次版本号，并作为预发布版本发布（见第 3 步）。合入 `main`。
2. 推送同名标签：

   ```bash
   git tag v1.0.0 && git push origin v1.0.0
   ```

3. `Release` 工作流会核对标签与版本一致、打包并发布 Release（说明里附 SHA-256 和
   `docs/releases/v<版本>.md`）。`release.yml` 里 `PRERELEASE_TAGS` 列出的版本会标为预发布版本：
   目前是只能装进新版桌面端的 `v0.0.1` 和依赖宿主 #1954、#1964、#1971 的 `v1.0.0`；
   `v1.0.1`、`v1.1.0`、`v1.2.0`、`v1.3.0` 和 `v1.3.1` 是正式版本。
   `v1.2.0` 和 `v1.3.0` 先以预发布发布，桌面端 0.8.7 带上它们依赖的 #2169 和 #2183 后改为正式版本；
   `v1.3.1` 的宽度排版要靠还没发布的 #2191，但不发宽度的桌面端上退回 1.3.0 的排版，所以直接作为正式版本发布。
4. Lithe Store 的自动追踪会发现稳定版本并开合并请求，合并后用户即可在 Lithe 中更新。自动追踪会跳过预发布版本；
   需要收录时在 Store 中手工提交条目，并使用已发布 ZIP 的实际 SHA-256。

## 许可

[Apache-2.0](LICENSE)
