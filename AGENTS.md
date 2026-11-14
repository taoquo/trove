# AGENTS.md — 怎么在这个仓库里干活

trove 是一个管理 X/Twitter 书签的桌面应用。Go 1.27 用 [MyGo](https://github.com/egoist/mygo)
绘制原生界面：没有 webview、HTML 或 JavaScript。数据在本地 SQLite；X 凭据和模型密钥在系统
安全存储里（macOS Keychain / Windows Credential Manager）。

先读：

- `docs/architecture.md` — 分层与组合根
- `docs/data.md` — 数据库、备份、凭据
- `docs/testing.md` — 测试、gate、截图
- `docs/design.md` — token、组件、状态与视觉参考
- `docs/ui-surface-index.md` — 绘制方法索引（改名或新增必须同步）

## 1. 仓库布局

```text
cmd/trove/           组合根、窗口、菜单、截图入口
cmd/mediaprobe/      开发用媒体诊断工具
cmd/seed/            用 testdata/site-fixture.json 造一份虚构书库，供站点截图
internal/domain      纯规则，不依赖其他层
internal/ports       usecase 需要的接口
internal/usecase     workspace / tagging / syncer / suggestions / settings
internal/storage     SQLite 仓库与迁移
internal/xsource     X GraphQL 客户端
internal/llm         模型提供方与适配器
internal/autotag     自动打标流水线
internal/credentials Keychain / Credential Manager 与浏览器 cookie 读取
internal/media       媒体下载与磁盘缓存
internal/bundle      .trovebundle 备份格式
internal/importexport 旧版 exportVersion: 2 JSON
internal/ui          MyGo 工作区与全部绘制
internal/i18n        中英文案目录
resources/           应用图标等随包资源
Casks/               Homebrew cask（跟随 latest release，版本无关）
site/                静态产品页（Cloudflare Worker，手动发布）
testdata/            冻结的等价性 fixture
tools/               检查器、截图工具、quality gate
docs/                当前架构与设计文档
```

根目录不放 Go 文件；应用入口是 `cmd/trove/main.go`。

## 2. 命令

`.toolchain/` 是固定的 Go 1.27.2 工具链：它禁止隐式切换（`GOTOOLCHAIN=local`）、强制
`CGO_ENABLED=0`，并把模块下载走镜像。`env.sh` 通过 `BASH_SOURCE` 定位自己，**必须从 bash
source，不能从 zsh**：

```bash
bash -c 'source .toolchain/env.sh && go test ./...'
```

交互式 bash 或 `bash -c` 内：

```bash
source .toolchain/env.sh

go test ./...
go test ./internal/domain -run TestCanonicalTagKeyMatchesFixture -v
go test -race ./...
find . -name '*.go' -not -path './.toolchain/*' -not -path './.git/*' -print0 | xargs -0 gofmt -l
go vet ./...
go tool mygo vet ./cmd/trove
CGO_ENABLED=0 go build -o /dev/null ./cmd/trove
go tool mygo build -skip-dmg          # -> build/darwin-arm64/trove.app
```

`gofmt -l .` 不能用：它会递归进 `.toolchain/go` 的 testdata。

完整 gate 是提交前唯一标准检查；某一步失败也会继续，最后汇总：

```bash
bash -c 'source .toolchain/env.sh && bash tools/gate.sh'
```

它跑文案/接线/表面/命名/引用检查、每个检查器的负样本、gofmt/vet/build、`go test` 与
`go test -race`、`mygo build`、启动/单实例/退出冒烟，以及两张真实截图上的边界与几何检查。
截图在本地 `docs/screenshots`，不提交。

截图与像素检查：

```bash
TROVE_DATA_DIR=<临时数据目录>
TROVE_HEADLESS=1
TROVE_CAPTURE=<输出 png>
TROVE_CAPTURE_STATE=<workspace|detail|settings|tag-autotag|...>
TROVE_CAPTURE_WIDTH=1297
TROVE_CAPTURE_LOCALE=<zh-CN|en>
TROVE_CAPTURE_APPEARANCE=<light|dark>
```

用法示例：

```bash
TROVE_HEADLESS=1 TROVE_CAPTURE=/tmp/trove.png \
  TROVE_CAPTURE_WIDTH=1400 TROVE_CAPTURE_APPEARANCE=light \
  build/darwin-arm64/trove.app/Contents/MacOS/trove
```

`tools/baselines.sh` 和 `tools/criteria.sh` 重新生成截图；`tools/panes.sh`、`tools/geometry.sh`
读取当前构建的截图做像素检查。

### 多平台

```bash
go tool mygo build -platform darwin/arm64,darwin/amd64
go tool mygo build -platform windows/amd64
```

macOS 的 `.dmg`、签名与公证需要 macOS；Windows 的 NSIS `Setup.exe` 需要 Windows。`mygo.json`
的 `main` 指向 `./cmd/trove`，`version` 是发布版本的唯一来源。

无窗口命令在 `cmd/trove/cli.go`：`trove sync`、`trove export <文件>`、`trove auth`、`trove version`。它们和窗口
共用同一份数据库与同一个同步服务（`library.syncer`），参数走 `flag`，输出是英文并写到 stdout。
新增子命令时，`runCommand` 里不认识的参数必须落回窗口——macOS 自己会传 `-psn_…`。

发布：推 `v<version>` 标签即触发 `.github/workflows/release.yml`，它构建三个安装包、生成更新包与
清单、改名成站点使用的固定文件名（`trove-macos-arm64.dmg`、`trove-macos-x64.dmg`、
`trove-windows-x64-setup.exe`）、为全部资产出构建证明与 `SHA256SUMS.txt` 并创建 Release。
文件名与 `site/public/app.js` 的 `FILES` 是同一份契约，改名要两边一起改。签名、公证与自更新由
secret 决定，缺哪个少哪层，见 `docs/testing.md`；发行说明取自 `CHANGELOG.md` 对应版本小节。
CI 在 `.github/workflows/ci.yml`，只跑 gate 里不需要窗口的部分。

## 3. 架构

```text
domain    纯规则，无依赖
ports     usecase 需要的接口（仓库、书签源、时钟）
usecase   workspace / tagging / syncer / suggestions / settings
adapter   storage/sqlite、xsource、llm、credentials、media、bundle、importexport
ui        MyGo 工作区，只和 usecase 说话
```

`cmd/trove/main.go` 是唯一的组合根：只有它选择具体实现并把对象接起来。其他包通过构造函数
接收接口，测试可以替换成内存实现。

存储是 `modernc.org/sqlite`（纯 Go，迁移在 `internal/storage/sqlite/migrations.go`）。X 凭据是
用户的 `auth_token` + `ct0` cookie：从已登录浏览器配置读取（`internal/credentials/browser.go`），
或由用户粘贴；写入 macOS Keychain（`/usr/bin/security`）或 Windows Credential Manager。
LLM 提供方在 `internal/llm/providers.go`，适配器在 `internal/llm/adapters.go`。导入/导出在
`internal/importexport`（旧版 JSON）和 `internal/bundle`（原生容器）。

## 4. MyGo 规则

`internal/ui.State` 只保存数据。一个 view 可能因为一次事件被重建多次，所以 `View` 必须无副作用：
不做 I/O，不保存元素或句柄。耗时工作走事件处理器或受保护的后台任务，完成后通过
`Window.Update` 发布回窗口线程。键盘命令用 `c.OnShortcut` / `handle.OnShortcut`。

UI 测试用 `ui.NewTester`（不开窗口），是复现布局和状态缺陷的常规方式。所有用户可见字符串走
`internal/i18n`；启动时 `AssertComplete()` 检查两种语言；缺 key 是构建缺陷。

新增或改名绘制方法必须在同一个改动里更新 `docs/ui-surface-index.md`。

## 5. 检查器

检查器是构建的一部分，不是可选工具：

- `tools/check-text.sh`：用户可见字符串必须走翻译目录；
- `tools/check-wiring.sh`：接口方法必须可达、字段必须被读取，并跑 surfaces / toolrefs /
  namesort / bigfiles / 颜色与几何检查；
- `tools/selfcheck.sh`：逐个破坏检查器看守的东西，证明它仍会变红；
- `tools/panes.sh`、`tools/geometry.sh`：在真实构建的截图上验证三栏边界与几何不变量。

新增检查必须同时加一个负样本；没有负样本的检查不进 gate。

## 6. 工作方式

- 动手前先读对应的 `docs/` 页面，把期望行为写成一句可断言的话。
- 改动后跑 gate；碰 X / LLM / Keychain 的改动要真跑一次，没跑就写 `not tried` 和原因。
- 同类问题先 grep；只有一个时在提交信息里写为什么。
- 提交信息是短的英文句子，前缀 `feat:`、`fix:`、`test:`、`refactor:`、`perf:`。
- 不提交生成物；`build/` 只是本地产物。

## 7. 红线

- **密钥、cookie 不进截图、日志或 commit。** 安全存储只查存在性，不读值、不打印。
- **不用 `open` 启动应用。** 直接运行 `build/darwin-arm64/trove.app/Contents/MacOS/trove`；
  有实例在跑时 gate 会拒绝。
- **不 push。** 提交即可，远端由用户决定。
- **不改生成物**（`build/`、`dist/`）。
- **不跳过 gate。** 时间不够只能缩范围。

## 8. 站点

`site/public/` 是静态产品页，通过 Cloudflare Worker `trove-site` 发布到
<https://trove.taoquo.com/>（配置在 `site/wrangler.jsonc`，安全响应头在
`site/public/_headers`）。发布是手动的一步：在 `site/` 里 `npm run check` 验证配置、
`npm run dev` 本地跑 Workers 运行时、`npm run deploy` 上线；CI 不发布站点。
改站点只改 `site/`，不要把构建产物或截图散落到根目录。
