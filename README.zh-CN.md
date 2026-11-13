# trove

[English README](./README.md)

trove 是一个原生 macOS 与 Windows 应用，面向在 X 上收藏过多、却仍然想回头找到某一帖的人。它把 X 书签同步到
本地 SQLite，并提供一个真正的工作区：浏览、搜索、筛选、打标签、审核建议——全程不依赖托管后端。

界面由 [MyGo](https://github.com/egoist/mygo) 直接绘制：没有 webview，没有 HTML，也没有 JavaScript。

## 功能

- 使用你自己的 `auth_token` + `ct0` cookie 从 X 同步书签（保存在 macOS Keychain 或 Windows Credential Manager）。
- 所有书签保存在本地 SQLite。
- 按标签、作者、媒体和已保存视图浏览、搜索、筛选。
- 用标签库整理书签，支持别名与父标签。
- 用启发式规则或 LLM 供应商生成标签建议，确认后再应用。
- 展示标签关系图，并对选区执行批量操作。
- 导入旧版 `exportVersion: 2` JSON 备份，并导出原生 bundle。
- 本地优先，中英双语。

## 环境要求

- macOS 12 或更高（Apple silicon 或 Intel），或 Windows 10/11 x64。
- macOS 发布包需要 Developer ID 做签名与公证；Windows 安装包由 NSIS 生成。
- `.toolchain/` 下固定的 Go 工具链（Go 1.27.2）。这是强制要求：它固定版本、禁止隐式切换
  toolchain，并把模块下载指向可用的镜像。
- Python 3 与 Pillow（截图类检查需要）。

## 构建与运行

从 bash 载入工具链（不要在 zsh 里 source——`env.sh` 通过 `BASH_SOURCE` 定位自身）：

```bash
bash -c 'source .toolchain/env.sh && go tool mygo build -skip-dmg'
```

产物在 `build/darwin-arm64/trove.app`。请直接运行 bundle 里的二进制，不要用 `open`，否则
LaunchServices 可能复用已在运行的实例：

```bash
build/darwin-arm64/trove.app/Contents/MacOS/trove
```

要同时构建两种 Mac 架构和 Windows 便携版，给 MyGo 传平台列表：

```bash
go tool mygo build -platform darwin/arm64,darwin/amd64
go tool mygo build -platform windows/amd64
```

Windows 上运行 `build/windows-amd64` 里的 `trove.exe` 或 NSIS `Setup.exe`。

## 测试与验收

```bash
bash -c 'source .toolchain/env.sh && go test ./...'
```

完整门禁是提交前的标准检查，包含源码检查器、`gofmt`/`go vet`/构建、带与不带 race detector 的
测试、MyGo 构建、启动/单实例/退出冒烟测试，以及两项基于实时截图的像素检查：

```bash
bash -c 'source .toolchain/env.sh && bash tools/gate.sh'
```

像素检查读取 `docs/screenshots` 下的截图，该目录只在本地保留。

## 仓库结构

```text
cmd/trove             组合根、窗口、菜单与截图入口
internal/domain       纯规则（无依赖）
internal/ports        用例所需的接口
internal/usecase      工作区、标签、同步、建议、设置
internal/storage      SQLite 仓库与迁移
internal/xsource      X GraphQL 客户端与查询目录
internal/llm          模型供应商、适配器与错误
internal/autotag      自动打标流水线
internal/credentials  Keychain / Credential Manager 与浏览器 cookie 读取
internal/media        媒体下载与磁盘缓存
internal/bundle       原生 .trovebundle 格式
internal/importexport 旧版 JSON 导入与导出
internal/ui           MyGo 工作区与全部绘制
cmd/mediaprobe        开发用媒体诊断命令
site/                 本地静态产品页（不自动部署）
tools/                源码检查器、截图工具、质量门禁
testdata/             冻结的等价性 fixture
docs/                 架构、数据、测试与设计
```

`docs/screenshots/` 下的截图只在本地生成，不纳入版本控制。

## 数据与隐私

书签、标签与设置保存在本地 SQLite 文件。X 凭据与 LLM API key 保存在 macOS Keychain 或 Windows Credential Manager。没有账号、遥测或
托管服务。
