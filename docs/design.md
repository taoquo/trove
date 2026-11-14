# 设计契约

视觉与交互的契约：token、组件、状态三张表。这些 token、组件和状态都是本项目的决定；
改行为或改值，必须在同一个改动里更新这里。每行带出处，出处一律指向本仓库的实现。

## 1. Token

| Token | 值 | 说明 | 我们的常量 |
|---|---|---|---|
| 设置页内边距 | 顶 8 / 底 8 | 页面顶部与底部的呼吸空间 | `SettingsPagePadTop/Bottom`（`metrics_interaction.go:86`） |
| 设置行 | 48 高 / `12 12 12 14` / gap 14 | 一行偏好的高度、内边距与间距 | `SettingsRowHeight/PadY/PadLeft/PadRight/Gap`（`metrics_interaction.go:109`） |
| 设置卡间距 | 8 | 相邻卡片之间 | `SettingsCardGap`（`metrics_interaction.go:117`） |
| 数字字段 | 72×24 / 右对齐 | 数字输入的宽度、高度与对齐 | `SettingsNumberWidth/Height`（`metrics_interaction.go:151`） |
| 字段 | 24 高 / `0 8` / 圆角 7 / 1px `--line` | 小字段的盒子 | `SettingsNumberPadX/FieldRadius`（`metrics_interaction.go:150`） |
| 行内动作 | 27 高 / `0 10` / 圆角 6 / 1px `--line` | 行内按钮 | `SettingsAction*`（`metrics_interaction.go:135`） |
| 下拉 | 24 高 / `0 6 0 9` / 圆角 7 / `--pill` | 选择器 | `SettingsPick*`（`metrics_interaction.go:121`） |
| 页签 | 22 高 / `0 7` / 圆角 6 / 11 600 `.12em` 大写 | 分区页签 | `SettingsTab*`（`metrics_interaction.go:93`） |
| 查找框 | `0 1 180`，最小 120 | 搜索字段的宽度 | `SettingsFindWidth`（`metrics_interaction.go:104`） |
| 行名 / 副行 | 13 / 500；11.5 `--fg-2` | 名称与说明的字号 | `SettingsRowNameFont/SubFont`（`metrics_interaction.go:114`） |
| 列表卡 | 圆角 10，1px `--line`，无阴影 | 卡片与列表的表面 | `RadiusPanel` + `rail.Line` |
| 页面容器 | `2 / 16 / 8 / 16`，内容 1200 居中 | 页面的内边距与阅读宽度 | `ResultsPadTop/Bottom` + `pageInset` |

### 1.1 色板

整套配色只有一处来源：`theme.go`。两个外观都以家族语言为准——暖纸当底盘、米白卡片、
一道朱红。暗色把纸反过来用墨：底盘是墨色，卡片是暖炭，朱红提亮三级。

| 角色 | 浅色 | 深色 |
|---|---|---|
| 底盘 | `#F6F0EA` | `#191514` |
| 卡片 | `#FBF7F3` | `#221C19` |
| 抬升行 | `#F9F3EC` | `#1E1815` |
| 托盘 / 悬停 | `#EFE7DF` | `#2B2421` |
| 按下 | `#EDE3DA` | `#332B27` |
| 细线 | `#E6DACD` | `#3A2F2A` |
| 次细线 | `#EADFD5` | `#322925` |
| 正文墨 | `#191514` | `#F6F0EA` |
| 弱化墨 | `#5A4A43` | `#D7CBC2` |
| 三级墨 | `#756A63` | `#B9ACA3` |
| 四级墨 | `#8B8078` | `#7F736B` |
| 朱红 | `#B83D2E` | `#C97A63` |
| 朱红按下 | `#7E241C` | `#E3A595` |
| 朱红浅底 | `#F7E6E1` | `#3A2521` |
| 选中底 / 边 | `#F7E6E1` / `#C97A63` | `#3A2521` / `#A35B47` |
| 滚动条拇指 | `#C9B9AC` | `#6B5B52` |
| 成功 | `#4A6B52` | `#8FB394` |
| 提醒 | `#906327` | `#E0B070` |
| 危险 | `#7E241C` | `#F2A19A` |

对比度：正文墨、弱化墨、三级墨与控制墨在页面和卡片上都 ≥ 4.5:1。四级墨（时间戳这类可略过的元信息）
是 3.4–3.9:1，浅色提醒字在提醒浅底上是 4.45:1，暗色朱红字在朱红浅底上是 4.40:1；这三处都直接取自
家族色板，不再单独压暗。

### 1.2 字体

正文、控件与设置行用系统无衬线。声音由显示层承担：品牌名、页名、弹窗标题、详情里的作者名，
都用家族衬线，按字形逐级回退。

| 角色 | 字体 |
|---|---|
| 显示衬线 | `"Source Serif 4", Charter, Georgia, "Songti SC", "Noto Serif CJK SC", "STSong", SimSun, Palatino, serif`（`displayFace`，与站点同一份列表） |
| 正文无衬线 | 系统 UI 字体（工具包的默认） |

标题栏的品牌位写产品名 `trove`，两种语言同一个词；它不描述当前页面。页名只写一次：工作区写在标题栏的
crumb，设置、标签库、标签图写在自己的内容页头（带返回键的那一行）。同一屏不出现两个页名。

## 2. 组件

| 组件 | 解剖 | 状态 | 实现 |
|---|---|---|---|
| 设置行 | 名称 13/500 + 说明 11.5 muted；右侧控件；行间 `--line-2` | 默认 / 说明为错误色 / 控件 busy | `settingsui.go` |
| 设置页头 | 返回 26 + 页名 13.5/600 衬线 + warm tabs 22 + 查找 26 | 普通 / 搜索中（页签让位） | `settingsview.go` |
| 数字字段 | 72×24 右对齐；无效时红边 | 默认 / 聚焦（清空重输）/ 越界（夹取） | `settingsui.go` |
| API Key 行 | 字段 + 保存 + 清除（27 高动作） | 未存（“尚未存密钥”）/ 已存（掩码占位 + “已保存”）/ 保存后清空输入 | `autotagview.go:403`、`syncsettings.go:185` |
| 开关 / 分段 | Off/On 22 高；轨道 `--pill`，选项 22/圆角 5 | 开 / 关 / 进行中 | `settingsui.go` |
| 侧栏子行（标签/作者） | 26 高；名称单行、超出省略，计数贴右 | 默认 / 悬停 / 当前（500，无底色） | `tags.go:tagRow` |
| 确认卡片 | 380 宽 / 内边距 18 / 圆角 12 / 纸色底；衬线标题 16/600 + 正文 13 muted；动作行右对齐 | 默认（确认动作用危险墨 + 危险浅底）/ 悬停（填实危险色）/ 键盘（焦点在取消，回车即拒绝） | `dialogs.go:confirmDialog` |

## 3. 状态矩阵

| 链路 | 状态 | 期望 | 检查 |
|---|---|---|---|
| 设置 | 默认 | 通用页签、语言/主题两行 | `TestTheSettingsPanelDrawsItsSections` |
| 设置 | 页面内边距 | 顶/底 8，页面下沿不留额外空白 | `TestTheSettingsPageKeepsItsPagePadding` |
| 设置 | 搜索无结果 | 页签让位，跨分区结果带分区名，空态在流内 | 待补 |
| 设置 | API Key 未存 | 行说明“尚未存密钥”，字段为空、无掩码 | `TestAStoredKeyStillReadsAsStoredAfterReopening` |
| 设置 | API Key 已存 | 行说明“已保存”，字段空但显示掩码占位 | `TestAStoredKeyStillReadsAsStoredAfterReopening` |
| 自动打标 | 运行中 | 进度分母是本轮工作量；预算秒数与在途数可见 | `TestTheModelPhaseMeasuresItsOwnWorkload` |
| 自动打标 | 取消 | 进度条显示“正在停止”并禁用；页面与侧栏不再有第二个停止按钮；在途批次保留 | `TestAutoTagStoppingOffersNoSecondStopButton`、`TestAutoTagStopKeepsTheBatchItIsOn` |
| 自动打标 | 恢复 | 重启后按状态记为 interrupted / cancelled；有剩余且开关允许时续跑，预算跨重启累计 | `TestHealStaleRun`、`TestHealingACancelledRunKeepsItCancelled`、`TestAutoTagPanelReportsTheInterruptedRun`、`TestAutoTagResumeDecision` |
| 确认 | 键盘 | 焦点在取消，回车与 Esc 都拒绝；选中确认动作才执行 | `TestTheConfirmationCardPutsReturnOnCancel` |
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
