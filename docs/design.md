# 设计参考

视觉与交互的契约：token、组件、状态三张表，加上钉住的上游参考。每行带出处；Magpie 的
行号来自下面钉住的 commit。改行为或改值，必须在同一个改动里更新这里。

实现期只读本地 clone（`/tmp/magpie-src`，pin 见下）；不读上游 HEAD。

## 参考

| 项 | 值 |
|---|---|
| 仓库 | `https://github.com/yetone/magpie` |
| 许可 | MIT（`LICENSE`，Copyright 2026 yetone） |
| 钉住的 commit | `dca338f3a348ac8e1a937ebde418e9957ac5c40e`（`dca338f`，2026-11-07） |
| 本地 clone | `/tmp/magpie-src` |
| 站点 | `https://usemagpie.ai/`（宣传页；行为以源码为准） |

抓取：

```bash
git clone https://github.com/yetone/magpie /tmp/magpie-src
git -C /tmp/magpie-src checkout dca338f3a348ac8e1a937ebde418e9957ac5c40e
```

**为什么不 vendored 整个站点**：CSS/JS/HTML 加起来很大，而且会随上游移动；需要的是“同一个
commit + 一条命令 + 哈希”，不是把产物塞进仓库。截图也只留在本地 `docs/screenshots`，证据记的
是捕获命令。

## 1. Token

| Token | 值 | 出处（目标） | 我们的常量 |
|---|---|---|---|
| 设置页内边距 | 顶 8 / 底 8 | `app.css:511`（side-nav view top）、`app.css:602`（view bottom） | `SettingsPagePadTop/Bottom`（`metrics_interaction.go:90`） |
| 设置行 | 48 高 / `12 12 12 14` / gap 14 | `app.css:3785` | `SettingsRowHeight/PadY/PadLeft/PadRight/Gap`（`metrics_interaction.go:113`） |
| 设置卡间距 | 8 | `app.css:3628` | `SettingsCardGap`（`metrics_interaction.go:122`） |
| 数字字段 | 72×24 / 右对齐 | `app.css:3751`（`.set-field[type=number]`） | `SettingsNumberWidth/Height`（`metrics_interaction.go:156`） |
| 字段 | 24 高 / `0 8` / 圆角 7 / 1px `--line` | `app.css:3750` | `SettingsNumberPadX/FieldRadius`（`metrics_interaction.go:155`） |
| 行内动作 | 27 高 / `0 10` / 圆角 6 / 1px `--line` | `app.css:2163` | `SettingsAction*`（`metrics_interaction.go:140`） |
| 下拉 | 24 高 / `0 6 0 9` / 圆角 7 / `--pill` | `app.css:3016` | `SettingsPick*`（`metrics_interaction.go:126`） |
| 页签 | 22 高 / `0 7` / 圆角 6 / 11 600 `.05em` 大写 | `app.css:3633` | `SettingsTab*`（`metrics_interaction.go:97`） |
| 查找框 | `0 1 180`，最小 120 | `app.css:3669` | `SettingsFindWidth`（`metrics_interaction.go:108`） |
| 行名 / 副行 | 13 / 500；11.5 `--fg-2` | `app.css:3785` | `SettingsRowNameFont/SubFont`（`metrics_interaction.go:118`） |
| 列表卡 | 圆角 12，1px `--line`，无阴影 | `app.css:637` | `RadiusPanel` + `rail.Line` |
| 页面容器 | `2 / 16 / 8 / 16`，内容 1200 居中 | `app.css:602`、`app.css:609` | `ResultsPadTop/Bottom` + `pageInset` |

## 2. 组件

| 组件 | 解剖 | 状态 | 出处 |
|---|---|---|---|
| 设置行 | 名称 13/500 + 说明 11.5 muted；右侧控件；行间 `--line-2` | 默认 / 说明为错误色 / 控件 busy | `app.css:3785`、`settingsui.go` |
| 设置页头 | 返回 26 + 页名 13.5/600 + warm tabs 22 + 查找 26 | 普通 / 搜索中（页签让位） | `app.css:3632`、`settingsview.go` |
| 数字字段 | 72×24 右对齐；无效时红边 | 默认 / 聚焦（清空重输）/ 越界（夹取） | `app.css:3751`、`settingsui.go` |
| API Key 行 | 字段 + 保存 + 清除（27 高动作） | 未存（“尚未存密钥”）/ 已存（掩码占位 + “已保存”）/ 保存后清空输入 | `autotagview.go:403`、`syncsettings.go:185` |
| 开关 / 分段 | Off/On 22 高；轨道 `--pill`，选项 22/圆角 5 | 开 / 关 / 进行中 | `app.css:3370`、`settingsui.go` |
| 侧栏子行（标签/作者） | 26 高；名称单行、超出省略，计数贴右 | 默认 / 悬停 / 当前（500，无底色） | `app.css:444-446`、`tags.go:tagRow` |

## 3. 状态矩阵

| 链路 | 状态 | 期望 | 检查 |
|---|---|---|---|
| 设置 | 默认 | 通用页签、语言/主题两行 | `TestTheSettingsPanelDrawsItsSections` |
| 设置 | 页面内边距 | 顶/底 8，页面下沿不留额外空白 | `TestTheSettingsPageKeepsMagpiesPagePadding` |
| 设置 | 搜索无结果 | 页签让位，跨分区结果带分区名，空态在流内 | 待补 |
| 设置 | API Key 未存 | 行说明“尚未存密钥”，字段为空、无掩码 | `TestAStoredKeyStillReadsAsStoredAfterReopening` |
| 设置 | API Key 已存 | 行说明“已保存”，字段空但显示掩码占位 | `TestAStoredKeyStillReadsAsStoredAfterReopening` |
| 自动打标 | 运行中 | 进度分母是本轮工作量；预算秒数与在途数可见 | `TestTheModelPhaseMeasuresItsOwnWorkload` |
| 自动打标 | 取消 | 进度条显示“正在停止”并禁用；页面与侧栏不再有第二个停止按钮；在途批次保留 | `TestAutoTagStoppingOffersNoSecondStopButton`、`TestAutoTagStopKeepsTheBatchItIsOn` |
| 自动打标 | 恢复 | 重启后按状态记为 interrupted / cancelled；有剩余且开关允许时续跑，预算跨重启累计 | `TestHealStaleRun`、`TestHealingACancelledRunKeepsItCancelled`、`TestAutoTagPanelReportsTheInterruptedRun`、`TestAutoTagResumeDecision` |
| 同步 | 未配置凭据 | 失败原因可读，不显示空列表 | 待补 |
| 详情 | 媒体缺失 | 占位而不是空白 | 待补 |
| 筛选 | 无匹配 | 空态在流内，说明当前筛选 | 待补 |

## 4. 品牌标记

图标与 folio（`github.com/taoquo/folio` 的 `assets/images/folio-mark.svg`）共用一套语言，
两边改动要一起看：纸色圆角方块、朱红细边、米色内框、近黑单线符号、底部一枚朱红圆点。

| 项 | 值 |
|---|---|
| 纸色 | `#F8F1EA` |
| 墨色 | `#191514` |
| 朱红 | `#B83D2E` |
| 内框 | `#D6C2B5` |
| 画布 | 128 × 128，无阴影、无渐变 |
| 外框 | `11,11` 起 `106 × 106`，圆角 24，描边 2 |
| 内框 | `26,26` 起 `76 × 76`，圆角 15，描边 1.5 |
| 符号线宽 | 2.5，圆头圆角 |
| 圆点 | `64,102`，半径 3.5 |
| 符号 | 一本书，书脊是一条竖线，书签带从下沿垂出（朱红实心、无描边） |

**小尺寸切图**：内框与圆点在 32 点以下会糊成噪点，所以侧栏 22 点的标记只留方块、书和书签带，
线宽加到 6。它是同一符号的简化版，不是第二个标志。

| 资产 | 源 | 用途 |
|---|---|---|
| `resources/icon.png`（1024） | `resources/icon.svg` | Dock、Finder、Windows 可执行文件 |
| `site/public/img/icon.png`（256） | 同上 | favicon |
| `site/public/img/mark.png`（256） | `internal/ui/testdata/branding/logo.svg` | 站点页头与页脚 24–30 点标记 |
| `site/public/img/og.png`（1200 × 630） | 站点 token + 同一标记 | 分享卡 |
| `internal/ui/testdata/branding/logo-128.png` | `.../branding/logo.svg` | 侧栏 22 点标记（内嵌） |

改图标要同时替换 SVG 源和导出的 PNG，再跑一次 gate 的截图检查。

## 捕获环境（视觉证据）

固定值，证据里写清楚：

```bash
TROVE_DATA_DIR=<临时 fixture 目录>
TROVE_HEADLESS=1
TROVE_CAPTURE=<输出 png>
TROVE_CAPTURE_STATE=<workspace|detail|settings|tag-autotag|...>
TROVE_CAPTURE_WIDTH=1297
TROVE_CAPTURE_LOCALE=<zh-CN|en>
```

构建后直接跑 bundle 二进制，不用 `open`。
