# 语法

视觉与交互的契约：**token、组件、状态三张表**。每行带出处；Magpie 的行号来自
`docs/loop/reference.md` 钉住的 commit。改行为或改值，必须在同一个 commit 更新这里。

实现期只读本地 clone（`/tmp/magpie-src`，pin 见 reference）；不读上游 HEAD。

## 1. Token

| Token | 值 | 出处（目标） | 我们的常量 |
|---|---|---|---|
| 设置页内边距 | 顶 8 / 底 8 | `app.css:511`（side-nav view top）、`app.css:602`（view bottom） | `SettingsPagePadTop/Bottom`（`metrics_interaction.go:92`） |
| 设置行 | 48 高 / `12 12 12 14` / gap 14 | `app.css:3785` | `SettingsRowHeight/PadY/PadLeft/PadRight/Gap`（`metrics_interaction.go:115`） |
| 设置卡间距 | 8 | `app.css:3628` | `SettingsCardGap`（`metrics_interaction.go:124`） |
| 数字字段 | 72×24 / 右对齐 | `app.css:3751`（`.set-field[type=number]`） | `SettingsNumberWidth/Height`（`metrics_interaction.go:158`） |
| 字段 | 24 高 / `0 8` / 圆角 7 / 1px `--line` | `app.css:3750` | `SettingsNumberPadX/FieldRadius`（`metrics_interaction.go:157`） |
| 行内动作 | 27 高 / `0 10` / 圆角 6 / 1px `--line` | `app.css:2163` | `SettingsAction*`（`metrics_interaction.go:142`） |
| 下拉 | 24 高 / `0 6 0 9` / 圆角 7 / `--pill` | `app.css:3016` | `SettingsPick*`（`metrics_interaction.go:128`） |
| 页签 | 22 高 / `0 7` / 圆角 6 / 11 600 `.05em` 大写 | `app.css:3633` | `SettingsTab*`（`metrics_interaction.go:99`） |
| 查找框 | `0 1 180`，最小 120 | `app.css:3669` | `SettingsFindWidth`（`metrics_interaction.go:110`） |
| 行名 / 副行 | 13 / 500；11.5 `--fg-2` | `app.css:3785` | `SettingsRowNameFont/SubFont`（`metrics_interaction.go:120`） |
| 列表卡 | 圆角 12，1px `--line`，无阴影 | `app.css:637` | `RadiusPanel` + `rail.Line` |
| 页面容器 | `2 / 16 / 8 / 16`，内容 1200 居中 | `app.css:602`、`app.css:609` | `ResultsPadTop/Bottom` + `pageInset` |

## 2. 组件

| 组件 | 解剖 | 状态 | 出处 |
|---|---|---|---|
| 设置行 | 名称 13/500 + 说明 11.5 muted；右侧控件；行间 `--line-2` | 默认 / 说明为错误色 / 控件 busy | `app.css:3785`、`settingsui.go` |
| 设置页头 | 返回 26 + 页名 13.5/600 + warm tabs 22 + 查找 26 | 普通 / 搜索中（页签让位） | `app.css:3632`、`settingsview.go` |
| 数字字段 | 72×24 右对齐；无效时红边 | 默认 / 聚焦（清空重输）/ 越界（夹取） | `app.css:3751`、`settingsui.go` |
| API Key 行 | 字段 + 保存 + 清除（27 高动作） | 未存（"尚未存密钥"）/ 已存（掩码占位 + "已保存"）/ 保存后清空输入 | `autotagview.go:407`、`syncsettings.go:208` |
| 开关 / 分段 | Off/On 22 高；轨道 `--pill`，选项 22/圆角 5 | 开 / 关 / 进行中 | `app.css:3370`、`settingsui.go` |
| 侧栏子行（标签/作者） | 26 高；名称单行、超出省略，计数贴右 | 默认 / 悬停 / 当前（500，无底色） | `app.css:444-446`、`tags.go:tagRow` |

## 3. 状态矩阵

| 链路 | 状态 | 期望 | 检查 |
|---|---|---|---|
| F6 设置 | 默认 | 通用页签、语言/主题两行 | `TestTheSettingsPanelDrawsItsSections` |
| F6 设置 | 页面内边距 | 顶/底 8，页面下沿不留额外空白 | `TestTheSettingsPageKeepsMagpiesPagePadding` |
| F6 设置 | 搜索无结果 | 页签让位，跨分区结果带分区名，空态在流内 | 待补 |
| F6 设置 | API Key 未存 | 行说明"尚未存密钥"，字段为空、无掩码 | `TestAStoredKeyStillReadsAsStoredAfterReopening` |
| F6 设置 | API Key 已存 | 行说明"已保存"，字段空但显示掩码占位 | `TestAStoredKeyStillReadsAsStoredAfterReopening` |
| F5 自动打标 | 运行中 | 进度分母是本轮工作量；预算秒数与在途数可见 | `TestTheModelPhaseMeasuresItsOwnWorkload` |
| F5 自动打标 | 取消 | 进度条显示"正在停止"并禁用；页面与侧栏不再有第二个停止按钮；在途批次保留 | `TestAutoTagStoppingOffersNoSecondStopButton`、`TestAutoTagStopKeepsTheBatchItIsOn` |
| F5 自动打标 | 恢复 | 重启后按状态记为 interrupted / cancelled；有剩余且开关允许时续跑，预算跨重启累计 | `TestHealStaleRun`、`TestHealingACancelledRunKeepsItCancelled`、`TestAutoTagPanelReportsTheInterruptedRun`、`TestAutoTagResumeDecision` |
| F1 同步 | 未配置凭据 | 失败原因可读，不显示空列表 | 待补（F1-1） |
| F3 详情 | 媒体缺失 | 占位而不是空白 | 待补（F3-1） |
| F2 筛选 | 无匹配 | 空态在流内，说明当前筛选 | 待补（F2-1） |

不适用某条链路的状态，在板子的那一行写明原因，不留空。
