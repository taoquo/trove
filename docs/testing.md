# 测试与验收

## 完整 gate

提交前跑完整 gate。它会逐步执行，某一步失败也继续，最后汇总；退出码非零表示至少一步失败。

```bash
bash -c 'source .toolchain/env.sh && bash tools/gate.sh'
```

步骤：

1. 文案、接线、表面索引、token、命名与引用检查；
2. 每个检查器的负样本（`tools/selfcheck.sh`）；
3. gofmt、`go vet`、`go tool mygo vet ./cmd/trove`、`CGO_ENABLED=0` 构建；
4. `go test` 与 `go test -race`；
5. `go tool mygo build -skip-dmg`；
6. 启动、单实例、退出冒烟测试；
7. 在真实截图上检查三栏边界与两条几何不变量。

必须在将要提交的那棵树上跑；跑完再改代码就要重跑。

## CI 与发布

`.github/workflows/ci.yml` 在每次 push 和 PR 上跑 gate 里不需要窗口的部分：文案检查、
`check-wiring.sh --source-only`（跳过读 `docs/screenshots` 的一对列检查）、gofmt、vet、
`mygo vet`、`go test` 与 `go test -race`；另有一个 ubuntu 任务只做 `GOOS=windows`
交叉编译，防止 Windows 一侧悄悄坏掉。`selfcheck.sh` 与截图检查需要 Pillow、真实截取和窗口，
只在本地 gate 跑。

`.github/workflows/release.yml` 由 `v*` 标签触发：macOS 上构建两种架构的 dmg，Windows 上构建
NSIS 安装包，生成更新包与清单，改名成站点使用的固定文件名，为全部资产出构建证明与校验和，
然后创建 GitHub Release。

| 文件 | 平台 |
|---|---|
| `trove-macos-arm64.dmg` | macOS Apple 芯片 |
| `trove-macos-x64.dmg` | macOS Intel |
| `trove-windows-x64-setup.exe` | Windows x64 |
| `SHA256SUMS.txt` | 三个安装包的校验和 |
| `trove-<version>-<platform>.tar.gz` | 自更新下载的应用归档 |
| `update-<platform>.json` | 自更新清单（版本、发行说明、归档的 URL/大小/签名） |

版本号：`mygo.json` 的 `version` 是唯一来源，标签必须等于 `v<version>`。1.0 之前不因为加了功能
就动 minor，每次发布只加 patch（0.3.1 → 0.3.2 → 0.3.3），采用 `0.1.<构建号>` 的递增方式：
自更新只比较版本大小，单调递增就够用，提前把 minor 用掉会让 1.0 变得没有意义。minor 留给
「用户能感知的形态变了」——数据格式、界面结构、支持范围，而不是一次功能批次。

标签必须等于 `v$(mygo.json 里的 version)`，否则发布任务直接失败。站点下载走
`https://github.com/taoquo/trove/releases/latest/download/<文件名>`，所以文件名是公开契约：
改名必须同时改 `site/public/app.js` 的 `FILES`。更新清单引用的是 MyGo 自己的归档名，不要改。

发布任务的 secret，缺一个就少一层，流水线会在运行摘要里说明缺的是哪个：

| Secret | 作用 |
|---|---|
| `MACOS_CERTIFICATE`、`MACOS_CERTIFICATE_PASSWORD` | base64 的 Developer ID `.p12` 与它的密码；导入临时 keychain 后 `mygo build -sign` 用它签名 |
| `MACOS_SIGNING_IDENTITY` | 例如 `Developer ID Application: Name (TEAMID)` |
| `APPLE_ID`、`APPLE_TEAM_ID`、`APPLE_APP_PASSWORD` | `notarytool store-credentials` 的凭据；公证 dmg 并 stapling |
| `MYGO_UPDATER_PRIVATE_KEY` | `mygo keygen` 生成的私钥内容；没有它就不生成更新包，应用也无法自更新 |

更新签名密钥是 Ed25519 的一对：私钥只放在密码管理器和这个 secret 里，公钥写在 `mygo.json` 的
`updates.publicKey`，并随应用编译进去。**私钥丢了，所有已安装的应用就再也收不到更新**，所以换
密钥要连同 `updates.publicKey` 一起发新版，否则旧版只会拒绝新更新。

发行说明取自 `CHANGELOG.md` 中与 `mygo.json` 版本对应的 `## <version>` 小节；`mygo build` 在
文件存在但缺少该小节时直接失败，这是有意的：没有发行说明的更新窗口是空白的。Windows 的代码
签名还没接（MyGo 侧是 `windows.certificate`）。

## 单元测试

- 每个包的标准 `go test`；域规则有表驱动测试。
- `testdata/` 下的 JSON 是冻结的等价性 fixture，测试名以 `MatchFixture` 结尾。它们断言 Go 实现
  与冻结结果一致；不要为了通过测试去改 fixture 的期望值，除非行为本身是有意变更。
- `internal/ui` 用 `ui.NewTester`：不开窗口，能断言布局、状态和事件后的重建。
- MyGo 的生命周期与 goroutine 规则由 `go tool mygo vet ./cmd/trove` 检查。
- Windows 上先跑 `go test ./...`；依赖 macOS Keychain 的集成测试只在 darwin 上编译和运行。

## 截图与像素检查

应用支持抓图模式，由环境变量控制：

```bash
TROVE_DATA_DIR=<临时数据目录>
TROVE_HEADLESS=1
TROVE_CAPTURE=<输出 png>
TROVE_CAPTURE_STATE=<workspace|detail|settings|tag-autotag|...>
TROVE_CAPTURE_WIDTH=1297
TROVE_CAPTURE_LOCALE=<zh-CN|en>
TROVE_CAPTURE_APPEARANCE=<light|dark>
```

构建后直接运行 `build/darwin-arm64/trove.app/Contents/MacOS/trove`，不要用 `open`：LaunchServices
会复用正在运行的实例，产生假通过。`build/` 是生成物，不提交。

`tools/baselines.sh` 和 `tools/criteria.sh` 重新生成 `docs/screenshots` 下的截图；这些图只留在
本地。`tools/panes.sh` 抓当前构建并检查三栏边界，`tools/geometry.sh` 检查两条几何不变量。

## 新增检查

一个检查只有在能变红时才有意义。新增检查时，同时在 `tools/selfcheck.sh` 里加一个负样本：
破坏它看守的东西，确认它报错，再恢复。没有负样本的检查不进 gate。
