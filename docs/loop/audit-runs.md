# 主动走查记录

主动走查的记录：十轮自走查，加上第 11、12 轮由用户实测触发的复查。**行动入口仍是 `board.md` 的 Inbox 与切片表**：这里只回答
“第几轮、看了哪里、发现了什么、哪一条当场修了、其余去了哪”。

每轮的固定格式：

```text
## 第 N 轮 — <日期> — <表面>
- 维度：信息架构 / 流程 / 效率 / 反馈 / 层级 / 一致性（本轮到重点）
- 发现：P0/P1/P2 + 一句话症状 + file:line + 复现
- 当场修：红 → 绿 → gate → commit（没有就写“无”）
- 已入板：Inbox / 新切片 ID
- 未验：没跑什么、为什么
```

规则：每轮最多修一个点，避免把走查变成一次大改；其余必须落到板上，不能只留在这里。
每轮的“验收”是它点名的检查：修了的给红／绿命令，没修的给一条已存在的测试或复现路径。

## 轮次

| 轮 | 日期 | 表面 | 当场修 | 入板 | 输出 |
|---|---|---|---|---|---|
| 1 | 2026-11-09 | 工作区 / 侧栏 / 结果 / 手工添加 / 设置 / 灯箱 | `ed3c0f4`、`e7b8665`、`cf02a53`、`2673c04`、`6769b38` | 10 条 Inbox | 5 条 P1/P2 |
| 2 | 2026-11-09 | 详情页：图片、标签、计数 | `77080aa` | 4 条 Inbox | 1 条 P2 修，4 条 P2 |
| 3 | 2026-11-09 | 标签库：行内编辑、批量栏、筛选 | `2f909fc` | 2 条 Inbox | 1 条 P2 修，1 条 P2，2 条 P3 |
| 4 | 2026-11-09 | 删除的持久性、同步回填 | 无（F7-1 切片） | F7-1 + 1 条 Inbox | 1 条 P1，1 条 P3 |
| 5 | 2026-11-09 | 键盘可达性：剩余纯指针控件 | 本轮 6 处 + `keyboardreach_test.go` | 1 条 P3 | 6 条 P2 修，1 条 P3 |
| 6 | 2026-11-09 | 结果页：搜索焦点、逐帧状态 | `1836fea` | 1 条 P3 | 1 条 P2 修，1 条 P3 |
| 7 | 2026-11-09 | 视觉巡检：工作区、标签库、详情、设置 | 无 | 3 条 Inbox | 1 条 P3，2 条产品问题 |
| 8 | 2026-11-10 | 空态 / 加载态 / 数据安全 | `78218a3` | 3 条 Inbox | 1 条 P2 修，2 条 P3 |
| 9 | 2026-11-10 | 自动打标：运行中、结果、建议 | `884c5b8` | 3 条 Inbox | 1 条 P2 修，3 条 P3 |
| 10 | 2026-11-10 | 同步状态、设置·同步、关系图 | `49a19e9` | 2 条 Inbox | 1 条 P2 修，2 条 P3 |
| 11 | 2026-11-10 | 用户实测：侧栏标签行、标签页头、关系图 | `8f9a21f`、`0210c58` | 1 条 Inbox（删 1 条已修） | 2 条 P2 修，1 条 P1 修，3 条教训 |
| 12 | 2026-11-11 | 用户实测（第二轮）：侧栏滚动条、关系图第二遍 | `17855ab`、`8e838bd` | 0 条 Inbox | 1 条 P2 修，1 条 P1 修 |
| 13 | 2026-11-11 | 用户实测（第三轮）：关系图的作者层 | `24f0894` | 0 条 Inbox | 1 条 P1 修，1 条 P2 修 |
| 14 | 2026-11-11 | 新十轮第 1 遍：结果页（单结果占满整行） | `9ee1d0a` | 0 条 Inbox | 1 条 P1 修 |
| 15 | 2026-11-11 | 新十轮第 2 遍：详情覆盖层（每颗贴纸的移除都叫 Remove） | `aa34e8e` | 0 条 Inbox | 1 条 P2 修，5 处量过不改 |
| 16 | 2026-11-11 | 新十轮第 3 遍：添加行关不掉 | `37fc61c` | 0 条 Inbox | 1 条 P2 修，2 处量过不改 |
| 17 | 2026-11-11 | 新十轮第 4 遍：侧栏搜索只认名字 | `d6e9bbd` | 0 条 Inbox | 2 条 P2 修，1 处量过不改 |
| 18 | 2026-11-11 | 新十轮第 5 遍：设置·同步与数据安全 | `da54601` | 0 条 Inbox | 2 条 P2 修，1 处收窄 |
| 19 | 2026-11-11 | 新十轮第 6 遍：自动打标（供应商、密钥、运行中） | `9c45103` | 0 条 Inbox | 1 条 P1 修，2 条 P2 修 |
| 20 | 2026-11-11 | 新十轮第 7 遍：加载 / 空 / 错误态 | `e1067f0` | 0 条 Inbox | 2 条 P2 修，2 处量过不改 |
| 21 | 2026-11-11 | 新十轮第 8 遍：键盘与焦点路径 | `d7bf4ab` | 0 条 Inbox | 2 条 P2 修 |
| 22 | 2026-11-11 | 新十轮第 9 遍：窄窗与宽窗三档 | `30e6847` | 0 条 Inbox | 无缺陷；两条宽度规则补测试 |
| 23 | 2026-11-11 | 新十轮第 10 遍：深色 × 语言 | `e537019`、`fffc7a2` | 0 条 Inbox | 1 条 P2 修，1 条 P3 修（取证） |
| 24 | 2026-11-11 | 续十轮第 1 遍：搜索与筛选链路 | `b9e3e94`、`3f449ed` | 1 条 Inbox | 1 条 P2 修，1 条 P3 修 |
| 25 | 2026-11-12 | 续十轮第 2 遍：批量操作 | `5333f2b` | 2 条 Inbox | 2 条 P2 修 |
| 26 | 2026-11-12 | 续十轮第 3 遍：列表与保存视图 | `46928ae` | 1 条 Inbox | 3 条 P2 修，1 条 P3 修 |
| 27 | 2026-11-12 | 续十轮第 4 遍：详情媒体与灯箱 | `6a01a6d` | 闭 1 条 Inbox | 1 条 P2 修，2 条 P3 修 |
| 28 | 2026-11-12 | 续十轮第 5 遍：标签库的编辑、别名与合并 | `b98c3ff` | 2 条 Inbox | 1 条 P1 修，2 条 P2 修，2 条 P3 修 |
| 29 | 2026-11-12 | 续十轮第 6 遍：词表审阅与建议面板 | `5e076df` | 1 条 Inbox | 3 条 P2 修，2 条 P3 修 |
| 30 | 2026-11-12 | 续十轮第 7 遍：同步全链路 | `76de3ac` | 更新 1 条 Inbox | 2 条 P2 修，1 条 P3 修 |
| 31 | 2026-11-12 | 续十轮第 8 遍：导出 / 导入 / 重置 | `dab291f` | 1 条 Inbox | 2 条 P2 修，2 条 P3 修 |
| 32 | 2026-11-12 | 续十轮第 9 遍：动效、过渡与减少动态 | `bdb8a59` | 1 条 Inbox | 1 条 P2 修，1 条 P3 修 |
| 33 | 2026-11-12 | 续十轮第 10 遍：真实体量收尾（76 标签 / 9.5k 书签） | `f6d216c` | 闭 1 条 Inbox | 1 条 P2 修，1 处量过不改 |
| 34 | 2026-11-12 | 点击走查第 1 遍：数据备份 / 导出 / 恢复 / 重置 | `3471797` | 1 条 Inbox | 1 条 P1 修 |
| 35 | 2026-11-12 | 点击走查第 2 遍：工具栏与搜索（含条件 chip） | `f1e3dac` | 1 条 Inbox | 2 条 P2 修 |
| 36 | 2026-11-12 | 点击走查第 3 遍：侧栏 | `d168c2d` | 0 条 Inbox | 无缺陷；点击普查入测试 |
| 37 | 2026-11-12 | 点击走查第 4 遍：结果卡片与右键菜单 | `c381e12` | 0 条 Inbox | 1 条 P2 修 |
| 38 | 2026-11-12 | 点击走查第 5 遍：详情覆盖层 | `5f6cde8` | 0 条 Inbox | 1 条 P2 修 |
| 39 | 2026-11-13 | 点击走查第 6 遍：批量条 | `411ad0e` | 0 条 Inbox | 3 处命名修 |
| 40 | 2026-11-13 | 点击走查第 7 遍：标签库 | `b2bcb2b` | 0 条 Inbox | 6 处命名修 |
| 41 | 2026-11-13 | 点击走查第 8 遍：设置页 | `70a3420` | 闭 1 条 Inbox | 1 条 P1 修，1 处命名修 |
| 42 | 2026-11-13 | 点击走查第 9 遍：自动标签页与建议面板 | `dcc03ce` | 0 条 Inbox | 2 条 P2 修 |
| 43 | 2026-11-13 | 点击走查第 10 遍：菜单与危险确认 | `c1d7e6e` | 0 条 Inbox | 1 条 P2 修 |

## 第 1 轮 — 2026-11-09 — 工作区 / 侧栏 / 结果 / 手工添加 / 设置 / 灯箱

- 维度：反馈机制、操作效率（本轮到重点），兼看信息架构与一致性。
- 发现：
  - **P2（当场修）** 手工添加表单（Cmd-N）是两个没有说明的空输入框：链接框不取焦、Enter 不提交、
    校验错误用 accent 而不是错误色。`internal/ui/toolbar.go:156`。复现：Cmd-N → 看到两个空框，
    不知道该填哪个 → 粘贴后按 Enter 没有反应。修：`TestTheAddFormNamesItsFieldsAndAddsOnEnter`
    （红：两个占位符都不存在、书签未写入；绿：通过）。提交 `ed3c0f4`。
  - **P1（当场修）** 卡片的“在 X 打开”读的是选中项而不是所在卡片：没有选中时说“无链接”，
    看过一条详情后，点任何卡片都打开那一条。`internal/ui/card.go:532`、`bundle.go:174`。
    修：`TestTheCardOpensItsOwnBookmarkOnX`（红：`opened []`；绿：打开卡片自己的 URL）。
    提交 `e7b8665`。
  - **P1（当场修）** 批量合并的目标可能和选择器显示的不一致：显示第一个标签、状态却是空，
    Merge 反过来拒绝屏幕上写着的那一个。`internal/ui/taglibrary.go:418`。修：
    `TestTheMergePickerCommitsTheTargetItShows`（红：合并拒绝显示的目标；绿：合并折叠的是
    没显示的那个）。提交 `cf02a53`。
  - **P1（当场修）** 设置页 Cmd-F 把光标交给查找框一次，却把请求留给之后每一帧：点数字框
    再输入，字符进了搜索框。`internal/ui/settingsview.go:93`、`settingsui.go:497`。修：
    `TestSettingsFindFocusIsSpentByAFrame`（红：数字框拿不住光标；绿：输入 7 落在
    StopBufferPages）。提交 `2673c04`。
  - **P1（当场修）** 预览打开时按 Escape 关掉的是下面的详情：灯箱注册的是窗口快捷键，
    详情的 DialogBase 以 overlay 抢走 Escape。灯箱控件也全是纯指针的。`shell.go:387`、
    `lightbox.go:293-346`。修：`TestLightboxEscapeClosesTheLightboxBeforeTheDetail`、
    `TestLightboxOwnsTheKeyboard`（红：第一次 Escape 关掉详情、关闭键无焦点、右箭头与 Tab
    无效；绿：两层依次关闭、Modal 焦点、箭头翻页）。提交 `6769b38`。
- gate：合并树一次通过 14/14（含 `go test -race`、单实例冒烟、两处几何断言）。
- 已入板：10 条 Inbox（见 `board.md`），其中列表范围另建切片 F4-1。
- 未验：真实 X 同步与真实 LLM 运行（本轮不涉及）；截图（本轮是行为修正，尺寸由测试钉住）。

## 第 2 轮 — 2026-11-09 — 详情页：图片、标签、计数

- 维度：操作效率、组件一致性（本轮到重点），兼看反馈与层级。
- 发现：
  - **P2（当场修）** 详情里的每张图片/视频缩略图是灯箱的唯一入口，却是纯指针控件：Tab
    到不了、Enter 不能打开、也没有可读的名称。`internal/ui/detailpost.go:97`。复现：打开一条
    带图书签 → 只用键盘 → 永远打不开预览。
  - **P2** 详情标签行每个标签上的移除叉同样是纯指针控件。`internal/ui/inspector.go:197`。
  - **P2** 标签库里的标签名（打开该标签书签）与“父标签”单元格只能点。
    `internal/ui/taglibrary.go:613`、`:699`。
  - **P2** 侧栏底部的语言切换 chip 只能点，且没有可读名称（只有“中/EN”字形）。
    `internal/ui/sidebar.go:245`。这是设置里也能改的同一件事，但侧栏这条路径对键盘用户关闭。
  - **P3** 计数的三个图标没有可读名称，读屏只念三个数字。`internal/ui/detailpost.go:174`。
  - **P3** 多图帖只画 `Media[0]` 的替代文本：第一张没有描述时，后面几张的描述完全不出现。
    `internal/ui/detailpost.go:46`。
- 当场修：`TestEveryPictureInTheDetailIsReachableFromTheKeyboard`（红：Tab 40 次仍到不了
  `detail-media-trigger`；绿：Tab 可达，Enter 打开灯箱）。实现是 `.Focusable()`，与卡片工具栏
  图标同一修法。提交 `77080aa`；gate 与第 1 轮的修复同一次跑，14/14。
- 已入板：上列 4 条 P2 进 Inbox；2 条 P3 随同一行记录。
- 未验：截图（本轮为焦点行为，尺寸由测试钉住）。

## 第 3 轮 — 2026-11-09 — 标签库：行内编辑、批量栏、筛选

- 维度：操作效率、反馈机制（本轮到重点），兼看一致性与层级。
- 发现：
  - **P2（当场修）** 标签库的名称/别名/主题三个行内编辑器都在 View 里调用 `Focus()`：
    每一帧都把光标抢回打开的字段，Tab 出不去，键盘用户到不了同一行的其它控件。
    `internal/ui/taglibrary.go:604`、`:664`、`:758`。
  - **P2** 别名编辑器的取消叉（✕）没有 `Focusable`，Save 可按、取消不可按。
    `internal/ui/taglibrary.go:764`。复现：打开别名编辑 → Tab 只停在 Save。
  - **P3** 筛选 pill 可聚焦但没有可读名称（`Label`），读屏只能念出子文本。
    `internal/ui/taglibrary.go:377`。
  - **P3** 批量合并的注释说目标“来自行显示的那份列表”，实际取的是全部标签
    （`s.state.TagItems`，不受搜索/筛选影响）。行为合理，注释需要改。`taglibrary.go:416`。
- 当场修：`TestTheOpenTagEditorGivesTheCursorUp`（红：Tab 后下一帧输入的 `x` 仍进了名称草稿
  `rust!x`；绿：Tab 后草稿不变）。实现：三个编辑器共用一个 `tagEditField` Handle，
  `beginTagEdit` 聚焦一次。提交 `2f909fc`；gate 14/14。
- 已入板：2 条 P2/P3 进 Inbox；1 条为注释级，随本轮记录。
- 未验：截图（行为修正）；真实数据下上百标签的滚动性能（本轮不涉及）。

## 第 4 轮 — 2026-11-09 — 删除的持久性与同步

- 维度：反馈机制、用户流程（本轮到重点），兼看信息架构。
- 发现：
  - **P1** 删除书签只删本地行；下一次同步把同一条推文重新 upsert 回来，删除不生效。
    `internal/storage/sqlite/bookmark_repo.go:498`、`internal/usecase/syncer/service.go:258`。
    复现（探针，跑完即删）：预置 1/2/3 → `DeleteMany(["2"])` → `Service.Run` → 书签 2 又存在
    （测试输出 `after the next sync, the deleted bookmark exists: true`）。确认框只写
    “从书库移除、X 账号不受影响”（`i18n.go:686`），没有说它还会回来。
  - **P3** 词表审阅失败后只有“关闭”，没有“重试”；要重来必须关掉再从工具栏打开。
    `internal/ui/review.go:28-37`。
- 当场修：无。P1 交给子 Agent 作为切片 **F7-1**（墓碑表 + UpsertMany 跳过 + restore/import
  清墓碑），验收命令已写在板上；本轮只给出红探针与设计边界。
- 已入板：F7-1（doing）；P3 进 Inbox。
- 未验：子 Agent 的实现与 gate 在本轮之后；真实 X 同步仍用 fixture。

## 第 5 轮 — 2026-11-09 — 键盘可达性：剩余纯指针控件

- 维度：操作效率、组件一致性（本轮到重点）。
- 发现（同一类，逐个数过）：
  - **P2（当场修）** 侧栏搜索开关（`sidebartabs.go:174`）、侧栏标签行的重命名/删除
    （`tags.go:756`）、详情标签的移除叉（`inspector.go:197`）、标签库的标签名与“父标签”
    单元格（`taglibrary.go:609`、`:697`）、别名编辑器的取消叉（`taglibrary.go:764`）、
    侧栏语言 chip（`sidebar.go:245`，且没有可读名称）——全部只能点。
  - **P3** 标签库筛选 pill 可聚焦但仍无可读名称。`taglibrary.go:377`。
- 当场修：`keyboardreach_test.go` 三条测试（红：Tab 40 次到不了目标控件；绿：可达，
  详情移除叉按 Enter 真的把标签摘掉，语言 chip 按 Enter 真的换语言）。实现统一为
  `Focusable()` + `Label()`；语言 chip 新增 `LanguageSwitch`（“Switch language/切换语言”），
  避免与设置页“Language”一行在测试里重名。
- 已入板：P3 一条；其余 5 条 Inbox 已清（本轮修完）。
- 未验：真实 VoiceOver 朗读（MyGo 的可读名称已由 Label 提供，但没有做真机读屏验收）。

## 第 6 轮 — 2026-11-09 — 结果页：搜索焦点与逐帧状态

- 维度：操作效率、组件一致性（本轮到重点）。
- 发现：
  - **P2（当场修）** Cmd-F 把焦点请求写进 `state.FocusSearch`，而工具栏每一帧都读它；
    只有一次 reload 才会清掉，Cmd-F 本身并不触发 reload。于是按过一次 Cmd-F 之后，搜索框
    每帧都把光标抢回去：Tab 出不去，点别处后下一次按键仍落进搜索框。
    `internal/ui/toolbar.go:310`、`reload.go:86-88`。复现：Cmd-F → Tab → 输入 → 字符进了
    搜索框（`TestTheSearchFocusIsTakenOnce` 红：query `"x"`）。
  - **P3** 关闭详情后焦点回到卡片的实现仍是“视图里 `Focus()` + 120ms 定时器清除”
    （`card.go:80-91`）。窗口只有 120ms，暂不构成可见问题，但它是同类里唯一残留的
    逐帧写法；等卡片行有稳定 Handle 时一并收掉。
- 当场修：`TestTheSearchFocusIsTakenOnce`（红：Tab 后输入仍写进 query；绿：query 保持空）。
  实现：搜索框绑定 `searchField` Handle，`FocusSearch` 直接聚焦一次；删除
  `state.FocusSearch` 与 reload 里的清除副作用。提交 `1836fea`；gate 14/14。
- 已入板：P3 一条。
- 未验：真实输入法的连续输入（OnChange 每次触发一次合并后的 reload，本轮未做输入去抖评估）。

## 第 7 轮 — 2026-11-09 — 视觉巡检：工作区、标签库、详情、设置

- 维度：视觉层级、组件一致性（本轮到重点），兼看信息架构。
- 方法：直接启动 bundle 二进制，`XBM_HEADLESS=1 XBM_CAPTURE_STATE=… XBM_CAPTURE_WIDTH=1300`
  取工作区、标签库、详情、设置（自动标签）四张图，与 Magpie 参考和 `docs/ui-spec.md` 对照。
- 发现：
  - **P3** 侧栏标签名超长时省略（`tags.go:723-724`），但整行没有 `Tooltip`；标签库表格
    里的同一个名字有（`taglibrary.go:757`）。复现：1366 宽窗口，标签 `ai image
    generation` → 侧栏读“ai image genera…”，悬停没有全文，只能去标签库看。
  - **P3（产品问题，非实现偏差）** 标签库筛选 chip 与摘要用“孤儿标签 / 长尾标签 / 未归
    主题”（`i18n.go:1166-1177`）。这些词逐字来自扩展的 `copy.ts:102-152`，不是替身自造；
    但它们对第一次使用的人是行话。要改就得两边一起改，属产品决策。
  - **P3（已记录的设计决策）** 设置的“自动标签”页签只有一张指路卡，下面整页留白
    （`settingsview.go:403-422`，shell-contract S26）。决策有理由，但对读者来说点进
    “自动标签”却看不到任何自动标签设置，是不小的意外。列入 Inbox 待产品环判断是否
    干脆去掉这个页签。
- 当场修：无。本轮是视觉核对，找到的都不是实现偏差；改任何一个都会与已记录的决策冲突，
  所以只入板。
- 已入板：3 条 Inbox。
- 未验：深色主题（本轮四张都是浅色）；1280 与 1440 两个宽度的断点；VoiceOver 朗读。

## 第 8 轮 — 2026-11-10 — 空态 / 加载态 / 数据安全

- 维度：反馈机制、信息架构（本轮到重点），兼看一致性。
- 方法：同上，取“无匹配搜索结果”“首读加载”“设置·数据安全”三张图，并核对
  `src/options/settings/SettingsPanel.tsx` 的同一页。
- 发现：
  - **P2（当场修）** 已有结果时刷新失败完全静默：`applyReload` 把错误写进 `state.Err`
    （`reload.go:105-111`），而 `state.Err` 只由空态绘制（`results.go:281`）。于是刷新
    失败时旧列表原样留着，没有 toast、没有横幅，读者以为看到的是刚读到的数据。
    复现：`newShell` 读一次 → 关掉数据库 → `Reload` → 行还在，`ToastText()` 为空。
  - **P3** 骨架屏是 220px 的 auto-fit 网格，结果网格却是两列 masonry（`results.go:227-248`
    对 `card.go:601-626`）：读数落地时第一行的四张卡片会重排成两张。骨架的注释写着
    “the same grid the real cards use, so the page does not jump”，这句与实际不符。
    数值本身来自扩展的 `.options-feed-skeleton-grid`（`spec_test.go` 钉住），所以是
    “照抄了源里本来就有的跳动”，不是替身新引入的。
  - **P3** “清空屏蔽名单”在设置·数据安全里，而扩展把它放在自动标签页
    （`SettingsPanel.tsx:912-922`）；它也是该组唯一没有说明副文案的行，70 这个数字
    没有上下文。
  - 已核对、不是缺陷：搜索结果为空时侧栏每个标签都显示 0，是 `tagCount` 的有意语义
    （`filterfacets.go:206-217`，计数是“点它会得到什么”）；空态贴着工具栏而不是垂直
    居中，是 magpie `.empty-state` 的页内流式布局（`results.go:338-346`）；数据安全页的
    破坏性按钮静止时不着色，是 `.text.danger:hover` 的既定写法（`shell-contract.md`）。
- 当场修：`TestAFailedRefreshKeepsTheRowsAndSaysSo`（红：`a failed refresh shows toast ""`;
  绿：行保持 1 条，toast 读“书签加载失败”）。实现：失败时若有行，额外走
  `s.ShowStatus(ToastError, LoadingFailedTitle)`，与其它失败动作同一条状态线（S37）。
  提交 `78218a3`；gate 14/14。
- 已入板：2 条 P3；1 条 Inbox（静默刷新）随本轮修复删除。
- 未验：真实磁盘 I/O 失败（测试用关闭数据库模拟）；数据安全页的导入/导出真跑
  （本轮只读代码与既有测试，未动真文件）。

## 第 9 轮 — 2026-11-10 — 自动打标：运行中、结果、建议

- 维度：反馈机制、信息架构（本轮到重点），兼看操作效率。
- 方法：同上，取“运行中侧栏”“上次分析结果”“建议待确认横幅”三张图，并对照扩展的
  `OptionsApp.tsx` / `BookmarkResultsPane.tsx`。
- 发现：
  - **P2（当场修）** 在审阅面板里“接受全部建议”之后，只有面板自己的列表刷新；结果页
    横幅继续数着已经不存在的建议，标签页的“有待确认建议”芯片、行内待确认列、自动打标
    页的待确认行都停在旧数字。单条接受会 `ReloadAsync`，批量路径漏了。`panels.go:747-753`。
    复现：预置 1 条待确认 → `Reload` → 打开面板 → 接受全部 → 横幅仍是 1。
  - **P3** “上次分析结果”是一整段没有层级的句子：本次新增、全库待确认、候选、批次数、
    失败批次数、词表大小、新标签额度、两种丢弃数挤在一行（`errors.go:106-122`）。其中
    “新增 1605 条建议，待确认 1550 条”把“本次”与“当前”混在一句，读起来像本次产出
    1550 条；这正是用户先前两次指出的“统计不一致”的残留。文案在扩展
    `copy.ts:225` 也有，要改是两边一起改的产品决策。
  - **P3** 结果页待确认横幅用全库待确认数并接受全部（`toolbar.go:534-581`），扩展用的是
    “当前可见”的建议并只接受可见的（`OptionsApp.tsx:984`、`BookmarkResultsPane.tsx:215-234`）。
    标签写的是“接受全部建议”，所以不算撒谎，但 1144 这个数与列表里看得见的 100 条无关。
  - **P3** `autotag-running` 截图不是生产可达状态：它设了 `captureRun` 却没设
    `state.AutoTagRunning`（`capturestate.go:193-209`），于是侧栏同时画出“停止本次分析”
    和“立即分析”。生产里两者由同一个标志互斥。这张图不能当作运行中状态的证据。
- 当场修：`TestAcceptingTheWholeBatchRefreshesWhatTheFeedCounts`（红：批量后横幅仍为 1；
  绿：0）。实现：批量路径补 `ReloadAsync`，标签页打开时再补 `ReloadTagLibraryAsync`，
  与单条路径一致。提交 `884c5b8`；gate 14/14。
- 已入板：3 条 P3。
- 未验：真实 provider 的 120/2000 运行（截图用引擎状态构造）；接受 1000+ 条的耗时。

## 第 10 轮 — 2026-11-10 — 同步状态、设置·同步、关系图

- 维度：用户流程、导航（本轮到重点），兼看一致性与信息架构。
- 方法：同上，取“同步中”“同步失败”“设置·同步”“关系图”四张图；设置页的开关按原图像素
  放大核对（下拉缩略图会把选中态看反）。
- 发现：
  - **P2（当场修）** 关系图整页没有可见出口：`GraphMode` 替换结果列，也带走了工具栏里
    那颗“关系图”开关，页面上只剩标题和“暂无可展示的关联”；退出只有 Escape，而页面上
    没有任何字提到它。设置页与标签管理页都在页头放了同一颗 26 方形返回，关系图没有。
    `results.go:65-146`。复现：从结果页打开关系图 → 只用鼠标 → 找不到回书签的入口。
  - **P3** 设置·同步的组标题是“X 凭证”，卡片第一行也叫“X 凭证”
    （`settingsview.go:326`、`:357`）：组标题与行名同名，别的组标题都是类别
    （数据 / 列表 / 基础规则）。要么去掉组标题，要么把行名改成“保存状态”。
  - **P3** 同步中的侧栏同时写“同步中”和“同步中… 3 页 · 60 条”（`sidebar.go:157-165`
    与进度行），两处同一句话。属措辞级。
  - 已核对、不是缺陷：设置页两个开关静止时缩略图都在“关闭”上（放大到原分辨率确认），
    与 `SaveRawPayloads` 默认 false 一致；侧栏同步失败句用 accent 而不是红色，是扩展
    `.folio-inline-message.is-error` 的原样（`extension.css:670-672`）；状态线“错误”用
    红色是 magpie 的 `err` 语气，两条通道各自有据。
- 当场修：`TestTheGraphPageHasAWayBackToTheBookmarks`（红：页面没有返回控件；绿：Tab 可达，
  Enter 退出 `GraphMode`）。实现：`graphPage` 用设置/标签页的页头语法包住三态，返回方块
  Label/Tooltip「返回书签」，调用 `ToggleGraph`。提交 `49a19e9`；gate 14/14；截图
  `/tmp/audit10-graph-after.png`（本地）确认页头位置与两页一致。
- 已入板：2 条 P3。
- 未验：有关系节点的图（截图状态只有空态）；真机读屏朗读关系图节点。

## 第 11 轮 — 2026-11-10 — 用户实测：侧栏标签行、标签页头、关系图

- 来源：不是自走查。用户在自己的库上（76 个标签、60 个节点、9509 条书签、212 点侧栏）
  用了两天，一条消息里指出三处，并问了一句更难回答的：“为什么前面十轮没发现，要我自己找？”
- 三条发现与修复（都当场修，都有红测试）：
  - **P2** 侧栏标签行右侧空 56 点。行尾保留了扩展 `opacity: 0` 的两个 22 点操作框
    （`extension.css:3007-3010`），在 280 宽的扩展里占五分之一，在 212 点这里占三分之一：
    名字被截成“ai image genera…”而旁边空着，计数也停在离行右缘 56 点的地方。改：计数贴行
    右缘（magpie `em { margin-left: auto }`，app.css:446），两个操作只在悬停时进入布局，
    名字补 Tooltip。`TestATagRowsCountReachesTheRowsEdge`（红：计数止于 x=135，行右缩进
    192；绿：192）。
  - **P2** 搜索 / “+” / 管理三个按钮画在页签之下另起一行，比页签低 38 点（用户：“偏下了，
    应该和标签作者平行”）。改：一行，放不下时按 magpie `.gw-share` 的 wrap 折行
    （`row-gap: 4`，app.css:2478-2480）；低于 260 点隐藏页签计数（扩展加的数字，列表行
    自带计数）。`TestTheFacetControlsSitBesideTheirTabs`（红：中心相差 38.0；绿：≤3）。
    顺带修掉一个 MyGo 陷阱：给 spacer 用 `FillWidth` 等于把整行占满，控件永远折行。
  - **P1** 关系图在真实体量下不可用（用户：“几乎不能用”）。源布局把每个标签放在半径 0.21
    的环上（`graphModel.ts`），60 个 60–120 点宽的标题挤在 923 点周长的环上，标签叠成一片
    灰，外面一圈点。改：native 力导向布局（`graphlayout.go`），与模型一起离线算、确定性；
    只给排名前 20 画标题（带底板，可压在点上）；节点可点——点一个标签就按它筛选，焦点面板
    给出计数、作者和“查看书签”。
    `TestTheGraphFindsRoomForItsCaptions`（红：源环上 60 个标题只有个位数放得下）、
    `TestClickingAGraphNodeFiltersByThatTag`。
- 顺带修掉的取证缺陷：`capturestate.go` 的 `graph` 状态只设了 `GraphMode`，没建模型，
  拍出来是空态——第 10 轮“未验：有关系节点的图”就是这个原因。现在走 `RebuildGraph`，
  headless 路径下它同步完成。
- 元反思（写进 lessons L10–L12）：
  - 十轮里有两处与参考**完全一致**：侧栏的保留操作位、环上的布局，parity 测试还专门钉住了
    后者。审计的尺子是“像不像参考”，复刻得越准越不可能被标出来。
  - 走查用的是 capture fixture（少量种子），不是真实体量；60 个节点、76 个标签只在真机上
    出现。
  - 六个维度都是“部件之间的关系”，没有一条问“这个功能在真实数据下能不能用”。
  - 唯一一次逼近关系图的第 10 轮，取证路径本身是坏的（截图状态没建模型），于是把未覆盖
    写成了“未验”，而不是“没拍到”。
- 已入板：1 条 Inbox（真实体量的走查清单）；删除已修的“侧栏标签名没有 Tooltip”。
- 未验：真机上重新拍的关系图（用户机器上应用正在运行，单实例锁不允许再起一个 capture）；
  76 个标签下的悬停操作（截图路径不能模拟指针停留）。

## 第 12 轮 — 2026-11-11 — 用户实测（第二轮）：侧栏滚动条、关系图第二遍

- 来源：仍是用户实测。第 11 轮的三处修好后，读者在自己的库上又指出两处，并说"我们按照上面
  修改的自动化的流程再跑一遍，让我看看你的改进有没有生效"——即：红测试 → 绿 → 取证 → 记录。
- 发现与修复：
  - **P2** 侧栏列表只要指针在上面就浮出一条滚动条，因为列表与列同宽，它压在计数上（截图里
    "93" 被盖住）。读者："感觉没有必要，把它移除掉吧。" magpie 的侧栏本来就没有
    （`scrollbar-width: none`，app.css:402-403）。改：`sidebarListScroll` 用负的右内边距把
    拇指推出窗口，滚动本身不变。`TestTheSidebarListDrawsNoScrollbar`（红：1422 个像素在悬停
    帧里变化）、`TestTheSidebarListStillScrolls`。
  - **P1** 关系图第二遍。读者："是不是很乱？字也小，不知道要体现什么内容，下面的区域也被
    严重的挤压。" 四处对应：地图居中正方形（宽窗左右各空三分之一）、有焦点时二十个 24%
    透明的幽灵标题、没有一句话说明点与线的含义、焦点卡 8 点内边距 + FontTiny 的 90 点条。
    改：源的整张 `.options-tag-graph-shell` 卡片 + `.options-tag-graph-focus` 焦点卡；
    一行图例；画布双轴铺满；只给排名前 20（有焦点时只给焦点与邻居）起名，标题移到点上、
    15→16 号；长尾圆点 38%、无焦点连线 ×0.5。
    `TestTheGraphUsesTheWholeCanvas`、`TestAFocusedGraphNamesOnlyWhatItLights`、
    `TestTheFocusPanelHasRoom`、`TestTheGraphSaysWhatItsMarksMean`。
- 过程中的两次自我纠错，留档：
  - "标题不许压在圆点上"试过，20 个里只剩 4 个能画——密图里几乎每个标题都压着某颗点。
    结论：底板保留（压在点上是可读的），标题之间不许相撞，点与标题的关系交给阅读顺序。
  - 标题宽松化（8 点呼吸）必须在 `fillGraph` **之后**做：标题尺寸以画布点为单位，先铺开
    再缩放会把刚分开的标题重新挤到一起。
- 取证：`XBM_CAPTURE_STATE=graph XBM_HEADLESS=1 XBM_CAPTURE_WIDTH=1400`，20 个标题里 19 个
  可读且互不覆盖（本地 `/tmp/graph-new5.png`；上一版同一条命令拍出的是中间一团、四周散点）。
- 未验：真机上的滚动条（无头截图不模拟指针停留于列表）；焦点状态下的整页截图（截图状态
  不能设标签条件），由 `TestTheFocusPanelHasRoom` 覆盖几何。

## 第 13 轮 — 2026-11-11 — 用户实测（第三轮）：关系图的作者层

- 来源：用户截图一张——关系图的头部写着“18 位作者节点 `%!(EXTRA int=18)`”。
- 发现与修复：
  - **P1** 格式串逃逸。头部把“数量 + 后缀”当成一个格式串交给 `Tf`，而
    `GraphAuthorNodesSuffix`（“位作者节点”/“author nodes”）两种语言里都没有动词，`fmt` 于是把
    多余的参数原样打成 `%!(EXTRA int=…)`。源是把数字和后缀当两段字符串拼的
    （`TagGraphView.tsx:99`）。改：`fmt.Sprintf("%d %s", …)`。
    `TestTheAuthorLayerIsCountedInWords`（红：复原后的实现读到
    `author nodes%!(EXTRA int=3)`；绿：“3 author nodes”，且页面上没有 `%!`）。
  - **P2** 同一张截图里的第二处：打开作者层之后，十八个空心圆**一个名字都没有**。读者打开
    这一层是为了看“谁在写这些主题”，匿名圆环只增加了噪声。源给每个 handle 画 `@handle`，
    位置在圆心下 `r + 12`、12px（`TagGraphView.tsx:210-215`）。改：`graphAuthorLabels` 把每个
    handle 排在自己的环下面，被画布边缘切掉、或落在已接受的标签标题上的跳过（宁可少一个名字，
    也不把两个名字画在一起）。`TestAnAuthorRingCarriesItsHandle`（红：“0 of the model's 3
    rings were named”；绿：三个都在，位置在环下 6 点，撞车的那个被跳过，出界的那个被跳过）。
- 为什么会逃逸：
  - 这是全库唯一一处把**没有动词的文案**当格式串用的地方；`check-text.sh` 只管“文案是否都
    走翻译”，不读格式串与参数的对应关系。
  - 没有任何 capture state 打开过作者层（`graph` 状态只建标签图）。这与 L12 是同一个洞的
    另一面：没拍过的状态等于没看过的状态。
- 取证：新增 `XBM_CAPTURE_STATE=graph-authors`，走 `RebuildGraph` 这条真实入口。1400×880 下拍到
  头部“60 个标签 · 189 条关联 · 已隐藏 16 个 · **18 位作者节点**”，以及 18 个 `@handle`
  （本地 `/tmp/graph-authors-en.png`）。
- 顺带：删掉 `ToggleGraphAuthors` 上重复的一段文档注释。
- 未验：真机上的作者层（用户的应用仍在运行，单实例锁不允许再起一个 capture）。

## 新的十轮（第 14–23 轮，2026-11-11 起）— 计划

用户在同一句里给了任务与规范：“主动走查每一个页面与交互细节，自行决定顺序与优先级，六个维度
同步审视，高优先级立即修，其余整理成可执行方案。” 前十三轮的顺序是被用户实测牵着走的；
这一轮起先把覆盖面排出来，每轮一个面、真实体量、红测试后再改：

| 轮 | 面 | 六个维度里先问的 |
|---|---|---|
| 14 | 结果页：单结果、双列、分页、筛选后的形态 | 视觉层级、反馈 |
| 15 | 详情覆盖层：媒体、作者、标签、上下文 | 信息架构、操作效率 |
| 16 | 工具栏与筛选面板：条件 chip、计数、清除 | 反馈机制、操作效率 |
| 17 | 侧栏：作者页签、搜索、折叠 | 一致性、信息架构 |
| 18 | 设置·同步与数据安全：凭据、危险操作 | 反馈、认知负担 |
| 19 | 自动打标：运行、结果、建议、词表 | 流程、反馈 |
| 20 | 空态 / 加载态 / 错误态（全站） | 反馈机制 |
| 21 | 键盘与焦点路径（全站） | 操作效率、可达性 |
| 22 | 窄窗与宽窗（1024 / 1400 / 1800 三档） | 视觉层级、一致性 |
| 23 | 深色与语言的交叉走查 | 一致性 |
| 24 | 搜索与筛选的完整链路 | 流程、反馈 |
| 25 | 批量操作：多选、批量打标、批量接受 | 操作效率 |
| 26 | 列表与保存视图 | 信息架构 |
| 27 | 详情媒体与灯箱 | 视觉层级、操作 |
| 28 | 标签库的编辑、别名与合并 | 反馈、数据 |
| 29 | 词表审阅与建议面板 | 反馈机制 |
| 30 | 同步全链路第二遍 | 流程、一致性 |
| 31 | 导出 / 导入 / 重置 | 数据安全、认知负担 |
| 32 | 动效、过渡与减少动态 | 视觉一致性 |
| 33 | 真实体量收尾：76 标签 / 9.5k 书签 | 认知负担、性能 |

每一轮的收尾固定：一条红测试 → 绿 → 真实体量截图 → 本文件一段 → 板上的行。

## 点击走查十轮（第 34–43 轮，2026-11-12 起）— 计划

用户的原话：“重点在你需要模拟点击所有可以点击的入口、按钮等等，判断功能是否正常\性能如何。
例如我点击数据备份的时候，导出数据的时候，它就没有任何响应了。” 所以这一段的验收口径从
“看得对不对”换成“按下去有没有事发生”：每个控件至少被点一次，断言它改变了什么（状态、
文案、导航、文件、对话框）；什么都不改的控件就是缺陷。

| 轮 | 面 | 点哪些 |
|---|---|---|
| 34 | 数据备份 / 导出 / 恢复 / 重置 | 导出数据、导出扩展备份、导入恢复（取消/确认）、重置（取消/确认） |
| 35 | 工具栏与搜索 | 搜索框、Filters 及其每个条件、排序、视图切换、图谱、管理标签 |
| 36 | 侧栏 | 折叠、标签行、作者页签、搜索、新建标签、底部的同步/语言/主题 |
| 37 | 结果卡片与分页 | 勾选、打开详情、外链、复制、右键菜单、加载更多 |
| 38 | 详情覆盖层 | 标签增删、作者、媒体与灯箱、打开原文、关闭 |
| 39 | 批量条 | 选择当前/已加载、打标、移动、删除、取消 |
| 40 | 标签库 | 行内编辑、别名、主题、批量合并、清理 |
| 41 | 设置各页签 | 通用 / 同步 / 自动标签 / 数据安全 / 列表与视图 / 搜索里的每个控件 |
| 42 | 自动标签与词表审阅 | 分析、取消、建议面板、应用、重试 |
| 43 | 键盘与菜单 | 快捷键、菜单项、四类危险确认的确认与取消 |

## 第 14 轮 — 2026-11-11 — 第 1 遍：结果页

- 维度：视觉层级（卡片在真实体量下的一致尺寸）、反馈（筛选后的形态）。
- 发现与修复：
  - **P1** 筛到只剩一条结果时，那张卡片占满整个结果区：正文 1057 点宽，而两列时是 468 点，
    图片也跟着放大一倍。原因：masonry 按数量对半分列，分列点落到末尾时第二列被整个跳过，
    剩下的一列 `Grow(1)` 吃掉了整行。源的 `lg:columns-2` 不管有没有内容都保有两列
    （extension.css，`.options-results-grid`）。改：`columns == 2` 时第二列照画（可以为空）。
    `TestOneResultKeepsItsColumn`（红：1057 vs 468；绿：两者相差 <5%）。
- 取证：`XBM_CAPTURE_STATE=filtered` 1400×880，单结果卡片与双列时同宽，右半区按源留空。
- gate：14/14。前一轮因读者的实例在运行而挂起的两步（launch/smoke、panes）这一轮跑通了，
  说明第 13 轮的 bundle 也是可启动、几何合规的。
- 未验：真机上的筛选交互（截图状态直接置了条件）。

## 第 15 轮 — 2026-11-11 — 第 2 遍：详情覆盖层

- 维度：六个维度在这一页各问一遍，重点是操作效率（头部一行里的控件）与反馈（遮罩、滚动）。
- 发现与修复：
  - **P2** 标签行里每一颗“移除”都叫 `Remove`：一颗贴纸一个 12 点控件，六个标签读出来就是
    “ai agent、Remove、coding agent、Remove……”，戴耳机的读者只能靠数顺序分辨哪一个属于谁。
    源里一颗贴纸就是一个按钮，按钮文字就是标签名（`BookmarkInspector.tsx:414-420`），所以它
    的无障碍名自带标签。改：新增 `action.removeNamed`（”Remove %s” / “移除 %s”），控件命名为
    “Remove ai agent”。`TestEachTagPillNamesTheTagItRemoves`（红：文本里是 “ai agent”
    “Remove” “design” “Remove”；绿：两个带标签的名字都在）。
- 量过但判定**不改**的五处（记下来，免得下一轮再查一遍）：
  1. 新标签输入框“像一行字，不像输入框”——取样它的上边缘是 227、卡片底 255：1 点描边本来就在
     （`TextInput` 自带，和 `theme.Border` 同色）。我一开始按截图判断“没有边框”，是缩放看错了。
  2. 深色下遮罩太弱——量得 26 → 19；源在深色下是 `rgba(25,21,20,.72)` + `brightness(.68)`
     ≈ 18。一样。浅色下 244 → 176，也一致。
  3. 卡片高度被写死 720——实测 700/880/1000/1200 高的窗里，卡片是 601/755/859/1031，正好跟随
     86dvh；`DetailCardMaxHeight` 是个没人用的旧常量。
  4. 卡片按整窗居中而不是按结果区居中——源的 `.options-detail-backdrop` 是 `inset: 0` 的整页
     遮罩，本来就按视口居中。
  5. 头部三个计数（评论/转发/喜欢）没有可读名称——源把它们整块标了 `aria-hidden="true"`
     （`BookmarkCard.tsx:230-243`），也就是故意不进朗读树。我们“没名字”其实更接近源，真正的
     缺口是 MyGo 没有“对辅助技术隐藏”的开关。Inbox 那条改写成这个缺口。
- 取证：本轮没有像素变化（只改了无障碍名）；证据是上面红测试的两段文本。

## 第 16 轮 — 2026-11-11 — 第 3 遍：工具栏、筛选面板与添加行

- 维度：反馈机制（控件按下去有没有回应）、操作效率（一行里能不能退出来）。
- 发现与修复：
  - **P2** “添加书签”这一行从窗口里关不掉。它由菜单栏的“添加书签…”打开，窗口里没有任何
    控件能收起它——按错了，或者链接打到一半改主意，只能再回菜单里找同一条命令。本构建里
    其它可以展开的行（侧栏的草稿标签、批量操作条）都能就地收起。改：行尾补一颗 28 点方块，
    close 字形，名称为“取消”，按下即收起；顺手把从 `addRow` 漂到 `ToggleAddForm` 上的
    那段文档注释归位。`TestTheAddRowHasAWayToCloseItself`（红：整行文本里只有两个输入框和
    “添加书签”；绿：点“取消”后 `AddOpen` 为假、字段不再画）。
- 量过但判定**不改**的两处：
  1. 筛选面板里“未读 / 已归档”两行——源里它们就是 `disabled`（`WorkspaceToolbar.tsx:112-121`），
     我们也是 `Disabled(true)`；取样文字最暗值，可用的两行是 28，禁用两行是 141，已经分开。
  2. 筛选面板的文案“选择要应用的过滤条件。”——逐字来自扩展的 `copy.filterDescription`，
     句号也一致；要改是两边一起改的产品决策。
- 取证：`XBM_CAPTURE_STATE=add` 1400×880，行尾出现关闭方块；筛选面板截图与源的
  `disabled` 行对照见上。
- 未验：菜单栏命令本身（headless 路径不点菜单）。

## 第 17 轮 — 2026-11-11 — 第 4 遍：侧栏的作者页签、搜索与折叠

- 维度：一致性（同一个问题在侧栏和管理标签里答案不同）、信息架构（空的列表说不说为什么）。
- 发现与修复：
  - **P2** 侧栏的标签搜索只匹配名称。输入框的占位符是扩展自己的“名称或别名”
    （`copy.ts:92`），管理标签里的搜索框从建起就按名称、别名、父标签匹配；侧栏却自己写了一
    个只比名称的循环。同一个查询在两处得到两个答案，用户无从判断是标签没有别名，还是这个框
    坏了。改：`visibleSidebarTags` 直接调用 `domain.FilterSidebarTags`，再按 ID 映回摘要行，
    一个匹配器两处用。`TestTheSidebarTagSearchFindsAliases`（红：搜 `rs` 得 `[]`；绿：得 `rust`，
    并且别名中间的词 `systems` 也命中）。
  - **P2** 搜索把全部标签排除时，侧栏什么都不画：没有行、没有句子。于是“这个库还没有标签”
    与“你的搜索把它们都藏了”看起来一模一样，而“+”就在同一行、用户会以为要新建。作者页签
    早就有 `sidebar.noAuthorsMatch` 回答这件事。改：新增 `sidebar.noTagsMatch`（“没有匹配的
    标签。”/“No tags match your search.”），标签页签在“库里有标签但一个都没命中”时画它，
    库真的为空时仍画原来的 `sidebar.noTagsCreated`。`TestTheSidebarSaysWhenASearchMatchesNoTag`
    （红：标签页签文本里没有任何说明；绿：出现匹配句，且不出现“还没有创建任何标签”）。
- 量过但判定**不改**的一处：作者页签里不应该有标签的“+ / 管理”控件——裁图确认只有放大镜，
  `sidebarTab() == sidebarTabTags` 的判断是生效的（这一条上一轮在缩放图上误判过一次）。
- 顺带发现（第 16 轮的测试债）：`TestTheDetailTagPillCanBeRemovedFromTheKeyboard` 还在找
  `Remove` 这个裸动词，而第 16 轮已经把控件命名为“Remove rust”。第 16 轮的收尾只跑了新测试
  所在的那一个 `-run`，整包 `go test` 才把它照出来。已改为找带标签的名字；教训见 L14。
- 取证：新增 `XBM_CAPTURE_STATE=sidebar-search-empty`，1400×880，搜索框下画出“没有匹配的
  标签。”——空列表的状态以前没有任何截图，这正是它一直空着没人发现的原因。
  `docs/screenshots/sidebar-search-empty.png`。
- gate：12/14。launch/smoke 与 panes 两步拒绝在读者的实例运行时开跑（pid 70669），其余全绿，
  含 `go test -race` 与 `mygo build`。第 14 轮的 14/14 是同一批步骤在没有实例时的基线。

## 第 18 轮 — 2026-11-11 — 第 5 遍：设置·同步与设置·数据安全

- 维度：反馈机制（按下之后有没有回执）、视觉层级（组的名字与卡片的名字各管什么）。
- 发现与修复：
  - **P2** 设置·同步把凭证卡片的名字写了两遍：上面一行 11 点的组标签“X 凭证”，紧接着卡片
    第一行又叫“X 凭证”，中间什么都没有。组标签是本构建自己加的；源里同步段就是一串行，没有
    组标题（`SettingsPanel.tsx:726-744`），同一页的前半段（同步开关）也没有。改：删掉该组标签，
    由卡片标题单独命名。`TestTheSyncSectionNamesTheCredentialCardOnce`（红：同一标题在文本里
    出现 2 次，一次大写；绿：1 次）。
  - **P2** 数据安全的“被屏蔽的标签”是这张卡片五行里唯一没有副文案的一行：只有名字和一个红色
    按钮，不说为什么会有标签名被记住。清空之后也更安静——行和按钮在计数归零时一起消失，页面
    没有任何回执，读者无法区分“清空了”和“点空了”。源的同一动作会写“屏蔽名单已清空。”
    （`SettingsPanel.tsx:237-243`、`copy.ts:250`）。改：新增 `settingsBlockedTagsHint` 副文案，
    清空时把 `settingsClearBlockedTagsDone` 写进页面自己的回执行（与保存 Cookie 成功同一条）。
    `TestTheBlockedTagRowSaysWhatItIsAndConfirmsTheClear`（红：文本里既没有说明也没有回执；
    绿：两者都在，且 `LoadSettings` 读回空列表）。
- 量过但**收窄**的一条（原 Inbox 项）：清空屏蔽名单在源里位于自动标签卡片内，本构建放在数据
  安全。放哪里仍是产品决策，本轮不改；“该行没有副文案”这一半已修，Inbox 那行改写成只剩位置
  问题。
- 未动：`X 凭证` 卡片里的“Cookie 或 cURL”多行框在空库截图里占了三行高度，但它是贴一个 Cookie
  头/curl 命令/导出 cookie 列表的入口，长度不可预知；源里也是 textarea。量过，不改。
- 取证：`XBM_CAPTURE_STATE=settings-sync`、`settings-data`，1400×880。同步页现在只剩一处
  “X 凭证”；数据页五行齐平，每行都有副文案。`docs/screenshots/settings-sync.png`、
  `settings-data.png`（首次入库）。
- gate：12/14，同第 17 轮：launch/smoke 与 panes 拒绝在实例运行时开跑（pid 70669），其余全绿。

## 第 19 轮 — 2026-11-11 — 第 6 遍：自动打标（供应商、密钥、运行中）

- 维度：反馈机制（按钮按下去有没有可能发生的事）、流程（换服务商时要带过去什么、不能带过去
  什么）。
- 发现与修复：
  - **P1** 切换 LLM 服务商时，端点输入框原样留下上一个服务商的自定义地址。密钥就是发往这个
    地址的：读者把 Custom 指向自建网关、再改选 OpenAI，OpenAI 的 key 会被发到旧主机。源的
    选择器在换服务商时重置——需要端点的服务商清空，其余回到文档默认值，草稿 key 一并丢弃
    （`SettingsPanel.tsx:345-351`）。改：`saveAutoTagSettings` 在 provider 变化时做同样的重置，
    并清掉不属于新服务商的 deployment。`TestSwitchingProviderDoesNotKeepTheOldEndpoint`
    （红：切到 OpenAI 后仍存着 `https://old.example/v1`，草稿 key 还在；切到 Custom 也留着旧
    地址。绿：OpenAI 得默认地址，Custom 得空，草稿 key 清空）。
  - **P2** API Key 的“保存 / 清除”无论什么状态都画成可用：空输入框按保存、没有 key 按清除，
    都什么也不发生也不说明。源里保存按钮在草稿为空时 `disabled`，清除按钮在没有已存 key 时
    根本不渲染（`SettingsPanel.tsx:1032-1048`）。改：同样处理——保存按草稿是否为空禁用，
    清除只在已存 key 时画出。`TestTheClearKeyButtonAppearsOnlyWithAStoredKey`（红：无 key 时
    文本里有 “Clear”；绿：无 key 无清除、有 key 有）。
  - **P2（工具）** `autotag-running` 截图状态只造了进度条读的运行态，没置 shell 读的
    `AutoTagRunning`。于是一张“正在分析”的图里，进度条下方还画着“立即分析”——真实窗口不可能
    出现的组合，第 453 轮以来的该状态截图全都如此。改：状态里置位；`TestTheRunningCapture
    DoesNotAlsoOfferAnalyzeNow`（红：文本里同时有 “Stop this run” 和 “Analyze now”；绿：只剩
    前者）。
- 未决（仍在 Inbox）：已存 key 聚焦输入框时占位掩码消失，要不要改固定掩码；词表审阅“应用所选”
  先关面板再干活、失败后只有“关闭”。这两条是下一轮（词表与建议）的入口。
- 取证：`XBM_CAPTURE_STATE=autotag-running`、`tag-autotag`，1400×880。侧栏现在只有进度条和
  “停止本次分析”。`docs/screenshots/autotag-running.png`、
  `autotag-running-sidebar.png`（首次入库）。
- gate：12/14，同前几轮：launch/smoke 与 panes 拒绝在实例运行时开跑（pid 70669），其余全绿。

## 第 20 轮 — 2026-11-11 — 第 7 遍：加载 / 空 / 错误态

- 维度：反馈机制（读者能不能一眼看出“正在发生”“什么也没发生”“出错了，下一步按哪里”）。
- 走查范围：`loading`、`search-empty`、`sync-error`、`sync-running`、`suggestions-pending` 五个状态
  各拍一张 1400×880。
- 发现与修复：
  - **P2** 同步中的侧栏把同一句话说了两遍：“同步中”一行、其下“同步中… 3 页 · 60 条”又一行。
    进度句本来就以“同步中…”开头（`copy.ts:20`），源里是一个按钮标签在两种文案间切换
    （`WorkspaceSidebar.tsx:393-401`）。改：有进度句就只画它，没有才画裸状态；取消按钮在同一行。
    `TestTheSyncingSidebarSaysItOnce`（红：文本里 “Syncing” 与进度句各一条；绿：一条）。
  - **P2** 同步失败时侧栏写“同步失败，请重试。”，但没有可按的东西：同步命令是页脚那颗图标，
    句子和它之间没有任何联系；cookie 失效那条有“重新连接 X”和“打开同步设置”，其它失败没有。
    改：非凭据类失败在句下补一个带文字的“立即同步”；凭据类失败保持原有的重连对，不画同步按钮
    （同样的 cookie 再跑一次只会再败一次）。`TestAFailedSyncOffersTheWayToTryAgain`
    （红：失败块的文本里没有 “Sync now”；绿：网络失败有、未授权失败没有且仍有 “Reconnect X”）。
- 量过但判定**不改**的两处：
  1. 骨架屏四列、结果两列（Inbox 已记，数值来自扩展自己的 220px auto-fit 骨架；要改是两边一起改
     的产品决策）。
  2. 加载态右上角仍写着上次的“100 / 9562 结果”，与骨架并置。这是“正在重读、旧数还在”的常见
     做法，且源同样保留计数；不作为缺陷处理。
- 顺带记下：`syncProgressLabel` 用 `{pages}` 花括号替换，不走 `Tf`。本轮测试最初用 `Tf` 填充它，
  读到了 `%!(EXTRA int=3, int=60)`——同一个格式串缺口（Inbox 2026-11-11 那条）在测试里也能咬人。
- 取证：`XBM_CAPTURE_STATE=sync-running`、`sync-error`，1400×880。侧栏现在各只有一行/一句话加
  一个按钮。`docs/screenshots/sync-running.png`、`sync-error.png` 及两张裁图（首次入库）。
- gate：12/14，同前：launch/smoke 与 panes 拒绝在实例运行时开跑（pid 70669），其余全绿。

## 第 21 轮 — 2026-11-11 — 第 8 遍：键盘与焦点路径

- 维度：操作效率（只用键盘时每一步有没有名字、有没有去处）、可达性。
- 方法：把 `internal/ui` 里 32 处 `Focusable()` 逐个对照其后是否带 `Label`；再对没有文字的纯图形
  控件做一次渲染级检查（`Tester.Find` 只认文字与无障碍名，正是读屏能拿到的东西）。
- 发现与修复：
  - **P2** 结果工具栏的网格 / 列表两个切换是纯图标方块，有焦点位、没有任何名字。源也一样
    （`WorkspaceToolbar.tsx:140-158`，只有 `aria-pressed`），所以这一条是**本构建的增补**而不是
    复刻：两个没有名字的方块排在 Tab 序里，读者只能靠试。改：`IconToggle` 增加 label 参数，
    两处调用传新增的 `view.grid` / `view.list`（“Grid view / List view”“网格视图 / 列表视图”），
    同时作为 Label 与 Tooltip。`TestTheViewTogglesTellTheKeyboardWhichIsWhich`（红：`Find` 两个
    名字都找不到；绿：都在）。
  - **P2** 标签库筛选 pill（Inbox 那条）可聚焦却没有自己的名字：计数是 pill 里的第二段文字，
    控件本名只有筛选词，读出来是“With pending suggestions”再接一个孤零零的“2”。改：控件的
    Label 带上计数（“With pending suggestions 2”），一段话就能说完它做什么、会筛出什么。
    `TestThePendingFilterChipNamesItsCount`（红：`Find("… 2")` 找不到；绿：找到）。
- 过程中撞到的一次断言冲突，记下来：`TestShellRendersInChinese` 用精确相等断言中文里不再出现
  “列表”（第 472 轮删掉侧栏那一组时的守卫）。格子/列表标签最初写成“列表”，正好踩中——改成
  “列表视图”既保持了那条守卫，也更像一颗开关的名字。中文标签的措辞不是随便挑的。
- 未动：菜单栏快捷键（headless 路径点不到菜单）不在本轮范围。逐处对照后，其余没有可见文字的
  焦点位——详情覆盖层与设置页的返回方块、灯箱的关闭 / 上一张 / 下一张 / 播放、侧栏的放大镜、
  行内的重命名 / 别名 / 删除图标——都已带 Label；结果卡片本身与它的复选框从子文本和
  “Select bookmark” 得到名字。
- 取证：本轮只改无障碍名与工具提示，像素没有变化；证据是两段红测试文本。
- gate：12/14，同前：launch/smoke 与 panes 拒绝在实例运行时开跑（pid 70669），其余全绿。

## 第 22 轮 — 2026-11-11 — 第 9 遍：窄窗与宽窗（900 / 1024 / 1400 / 1800）

- 维度：视觉层级、组件一致性（同一套规则换一个窗口宽度还成不成立）。
- 走查范围：工作区、设置、详情覆盖层、关系图、标签库，四个宽度各拍一张；关键处按 1:1 裁图
  取样，不看缩放图。
- 结论：**没有找到缺陷**。两处宽度规则都已实现，且都按源的数值成立：
  1. 结果网格在 1024 点换两列。源是 `columns-1 lg:columns-2`（`extension.css:2020-2037`），
     `lg` 是视口断点：1023 点一列、1024 点两列，与侧栏是否展开无关。1800 点仍两列。
  2. 正文栏在 1200 点封顶。源是 `padding-left: max(16px, calc(50% - 600px))`（`app.css:609`）。
     1800 点宽的 1:1 截图上，卡片边缘在图像 x=846 与 x=3242 之间，2× retina 下折合 1198 点。
     2400 点同样宽，不再长。
- 其余量过、成立的点：900 点下侧栏仍在，是因为 `SidebarChosen` 记住了用户的选择（不是断点
  失效）；标签表在 900 点丢掉“更新时间”列，保留名称 / 计数 / 操作；详情卡片在 1024 与 1800
  下都停在约 740–750 点；关系图在 1024 的 1:1 裁图里标签不互相重叠，只有连线从标签下穿过。
- 本轮的输出因此是**覆盖**而不是修复：这两条规则此前只有 `TestOneResultKeepsItsColumn` 间接
  照看，而它只跑 1400 宽，断点一动没人知道。新增
  - `TestTheColumnsSwitchAtMagpiesBreakpoint`：1023→1、1024→2、1800→2，用字面量而非常量
    （读 `ResultsTwoColumnWidth` 的测试会跟着常量一起移动，永远不红）；
  - `TestTheReadingColumnStopsGrowingAtTwelveHundred`：1800 与 2400 两点量正文宽度，不允许
    超过 2%。
  变异验证：把 `ResultsTwoColumnWidth` 改成 1100，第一条红——“at 1024 points the feed drew 1
  columns, want 2”；把 `ContentMaxWidth` 改成 2000，第二条红——“the body is 668 points wide at
  1800 and 890 at 2400”。`metrics.go` 已复原。
- 取证：`docs/screenshots/width-900.png`、`width-1024.png`、`width-1800.png`、
  `width-1024-tags.png`、`width-1800-detail.png`（首次入库）。
- gate：12/14，同前几轮：launch/smoke 与 panes 拒绝在实例运行时开跑（pid 70669），其余全绿。

## 第 23 轮 — 2026-11-11 — 第 10 遍：深色与语言的交叉走查

- 维度：一致性（同一份界面在 浅/深 × 中/英 四种组合下是否还是同一个东西）、视觉层级（深色下
  每个面是否仍然分层）。
- 方法：21 个抓图状态全部在深色下各拍一张；工作区、详情、标签库、关系图、设置、自动打标、建议、
  同步失败、筛选、搜索空态这 11 个再拍中文浅色、中文深色、英文深色三份逐一对照。局部按 1:1
  裁图、取样像素，不看缩放图。
- 结论：主题本身没有缺口。取样确认了深色的三层：页面 `#1a1a1e`、卡片 `#232327`、pill
  `#2d2d33`；筛选弹层是卡片色（与页面分得开），侧栏进度条的轨道与填充在深浅两套里是同一组
  关系（轨道 accent 低透明、填充实 accent）。图表、表格、表单、危险按钮的悬停底色都有深色
  分支（`dangerSoftFor`、`GraphCanvasTokensFor`）。
- 发现与修复：
  - **P2** 语言与主题的交叉状态：从侧栏底部的语言按钮切到中文时，设置·通用的两个选择器不
    重算。主题行仍写 “Light”，语言行仍写 “English”，而整页其它文字已经是中文。设置页内部的
    语言处理自己调了 `syncPreferenceLabels`，页脚这个入口没有。改：`SetLocale` 自己重算，
    两个入口一致。`TestTheSidebarLanguageChipRelabelsTheSettingsPage`（红：切到中文后主题
    选择器仍持有 “Light”；绿：持有 “浅色”）。
  - **P3（取证）** `tags-editor` / `tags-aliases` / `tags-theme` 三个状态的截图与 `tags`
    一模一样：编辑器开在 `TagItems[0]` 上，而仓库按名称排序返回、表格按书签数排序绘制，
    名称上的第一个标签（13 条）在屏幕外。S28 的三张“证据”其实什么都没拍到。改：状态改为打开
    `VisibleTagItems()[0]`。`TestTheTagEditorCaptureOpensOnTheRowsAtTheTop`（红：焦点在
    `t-alpha`，首行是 `t-zebra`；绿：焦点在首行）。重拍后深色与浅色都能看到第一行的输入框、
    左侧 accent 条与聚焦环。
- 量过、判定**不改**的三处：
  1. 英文占位符 `Enter a new tag name` 在详情的新标签输入框里被截成 `Enter a new tag na`。
     源的上限同样是 116 点（`extension.css:3428` `clamp(82px, 10vw, 116px)`），HTML input 的
     placeholder 同样不带省略号地截断——这是两边一起改的产品决策，不是复刻偏差。
  2. “重置本地数据”“清空屏蔽名单”静止时与普通按钮同样式，只有悬停才变红。这是 recorded
     决策：本构建的按钮语法是 magpie 的 `.text.action`（`shell-contract.md:3672`），危险色是
     `.text.danger:hover`；扩展的常驻染色行属于另一套语法。
  3. 深色下帖子自带的浅色截图（媒体）很刺眼；那是内容，不是界面。
- 取证：`docs/screenshots/dark-workspace-zh.png`、`dark-settings-zh.png`、
  `dark-settings-data-zh.png`、`dark-tags-zh.png`、`dark-detail-zh.png`、`dark-graph-zh.png`、
  `dark-autotag-running-en.png`；修复前后的编辑器对照
  `tags-editor-dark-zh.png`、`tags-editor-light-zh.png`（首次入库）。
- gate：12/14，同前几轮：launch/smoke 与 panes 拒绝在实例运行时开跑（pid 70669），其余全绿。

## 第 24 轮 — 2026-11-11 — 续十轮第 1 遍：搜索与筛选链路

- 维度：流程（输入 → 加条件 → 无结果 → 逐个/全部清除）、一致性（同一个条件在几个地方
  出现几个名字）。
- 方法：把扩展的 `searchBookmarks.ts`、`WorkspaceToolbar.tsx`、`OptionsApp.tsx` 与原生
  `toolbar.go`、`filterfacets.go`、`chips.go` 逐项对照；用 `ui.Tester` 复现条件 chip 的
  焦点路径与空结果卡。
- 发现与修复：
  - **P2** 工具栏与空结果卡上的条件 chip，右侧那颗 x 的无障碍名就是条件本身（“包含媒体”
    “rust”），于是读屏在同一条 chip 上遇到两个同名元素，按钮那一个还没有动词。详情标签贴纸
    （第 16 轮）与标签库筛选 pill（第 21 轮）都已改成“移除 X”，这是第三处行内删除，因为
    “有名字”而漏过。改：按钮读“移除 <条件>”，裸动词作工具提示。
    `TestEachActiveFilterChipNamesWhatItRemoves`（红：找不到 “Remove rust”；绿：找到）。
  - **P3** 同一个“只含媒体”条件在同一屏上有两个名字：筛选弹层里的行写“包含媒体 / Has
    media”（`copy.ts:363`），选上之后出现的 chip 写“仅含媒体 / Media only”——原生自己加的
    第二个 key。源只有一个 key，行与 chip 共用（`OptionsApp.tsx:414-417`）。改：chip 改用
    `filter.hasMedia`，删掉 `filter.withMedia` 与两份译文。
    `TestTheMediaChipUsesTheSameWordAsTheFilterItRepresents`（红：chip 不出现 “Has media”；
    绿：出现）。
- 量过、判定**不改**的四处：
  1. 搜索匹配的字段与源一致：正文、作者名、句柄、帖子 URL（`searchBookmarks.ts:34-46` ↔
     `bookmark_repo.go:414-423`），不搜 hashtag，不搜标签名（标签从侧栏筛）。
  2. 排序按钮的循环是源的 4 档，`saved-desc` 不在循环里（`utils.ts:169`），已有测试钉住。
  3. 筛选弹层里“未读 / 已归档”两项在源里是 `disabled` 的占位，原生同样禁用并压暗。
  4. “清除全部”只清条件、不清搜索框（S25 的决策）；空结果卡把条件 chip 再画一遍，并给
     “清除全部”。
- 新 Inbox：搜索框占位符承诺“书签、作者和备注”，而两边的数据模型都没有备注字段，也都不搜
  备注（`copy.ts:39/353`、`searchBookmarks.ts:34-46`）。要改是两边一起改的产品决策。
- 取证：本轮的修复一个在无障碍名里、一个是同义词，像素没有变化；证据是两段红测试文本与
  `docs/loop/board.md` 的 R25、R26。
- gate：12/14，同前：launch/smoke 与 panes 拒绝在实例运行时开跑（pid 70669），其余全绿。

## 第 25 轮 — 2026-11-12 — 续十轮第 2 遍：批量操作

- 维度：反馈（做完的动作有没有回执）、操作效率（从多选到写库的路径）、识别（同一个条上两
  个选择器分不分得清）。
- 方法：把源的批量条 `BookmarkResultsPane.tsx:40-75` 与 `OptionsApp.tsx:500-519` 的
  `handleApplyBulkTag`，和原生 `toolbar.go` 的 `batchBar`、`selection.go`、`inspector.go`
  逐条对照；新增 `batch-selected` 抓图状态。批量条只在勾选卡片角上的方框之后才出现，前 24
  轮没有一张截图拍到过它——这次第一次拍到（浅 / 深 / 英三张），而它一进画面就暴露了第二个
  缺陷。
- 发现与修复：
  - **P2** 三条批量写入都没有回执，选择也不清。源在一次 bulk apply 之后调
    `handleClearBulkSelection`（`OptionsApp.tsx:517-518`），条形消失就是确认；原生保留选择、
    reload 后把条原样重画：计数一样、控件一样、没有一句话。在“全部书签”范围下移动列表屏幕
    完全不变（卡片不显示所属列表），一次成功的移动看起来像没按到，引诱人再按一次。改：输入
    框打标、选择器打标、移动到列表三条写入成功后都给回执（`batch.tagged` /
    `batch.moved`，各带单数文案）并清空选择。`TestABatchActionSaysWhatItDid`、
    `TestAMoveToListNamesTheDestination`、`TestTheTagPickerSaysWhatItDid`（红：写入后没有
    任何一句话 / 选择仍在；绿：“Tagged 2 bookmarks”“Moved 2 bookmarks to Work”
    “Tagged 1 bookmark”，选择为空）。文案在红跑时先出过 “Tagged 1 bookmarks”，据此补了
    两条单数 key。
  - **P2** 两个选择器静止时只画一个空胶囊加箭头。第一次拍到的批量条上，“选择标签”和“选择
    列表”两处都只有一个箭头，分不清哪个打标、哪个移动；源的 `<select>` 有一个空值的
    “Choose tag” 选项（`BookmarkResultsPane.tsx:54-60`、`copy.ts:374`），未选时 Apply 是
    禁用的（`:69`）。更糟的是列表选择器未选时走 `listIDForLabel` 的“首个列表”兜底：按下
    “移动到列表”会把选择静默移进第一个列表，回执还写着空目标。改：两个选择器把占位词作为
    第一个选项（`batch.chooseTag` / `batch.chooseList`），占位时两个动作禁用，`listIDForLabel`
    去掉兜底；占位词是翻译值，随每个语言入口重算（L16）。`TestTheBatchBarSaysWhatEachPickerChooses`、
    `TestAPressOnTheUnchosenPickerDoesNothing`（红：选择器没有任何静止文字 / 未选时按下会消
    耗选择；绿：两个占位词都在，选择原样）。
- 量过、判定**不改**的四处：
  1. 源只有“选一个标签并应用”这一条批量写入；“输入框打标”“移动到列表”“删除”是本构建加
     的，回执文案因此也是本构建自己的词，目的与源的清空选择相同。
  2. 批量条的位置在工具栏之下、结果之上，与源一致（`options-bulk-actions-bar`）。
  3. “删除”走确认框，源同样是确认后删除。
  4. 打标后工具栏的标签计数与侧栏计数由同一次 reload 更新，没有第 8 轮那种停在旧数字的问题。
- 新 Inbox 两条：① “选择全部结果”一次选中全部 9,562 条（`MatchingIDs`），源把它拆成
  “选择已加载”（当前 80 条）与“选择当前结果”两个按钮（`BookmarkResultsPane.tsx:45-52`、
  `copy.ts:58-59/372-373`）；原生只有后者且叫“全部结果”。② 批量条的标签图标按钮与输入框的
  Enter，在输入为空时什么都不做也不说（`AddTagsToSelection` 遇空输入直接返回）。
- 取证：`docs/screenshots/batch-selected-light-zh.png`、`batch-selected-dark-zh.png`、
  `batch-selected-light-en.png`（首次入库）。英文下 “Choose tag” / “Choose list” 在两颗
  pill 里放得下，两个动作压暗。
- gate：12/14，同前：launch/smoke 与 panes 拒绝在实例运行时开跑（pid 31684），其余全绿。

## 第 26 轮 — 2026-11-12 — 续十轮第 3 遍：列表与保存视图

- 维度：信息架构（这两块东西放在哪、找不找得到）、反馈（按下去之后发生了什么）、组件一致性
  （一排同样的图标各叫什么）。
- 方法：把源的 `WorkspaceSidebar.tsx:27-110`（保存视图面板）与 `OptionsApp.tsx:556-583`
  （保存 / 应用 / 删除）逐条对照原生 `listsviews.go`、`settingsview.go` 的入口与状态；新增
  `settings-lists-views` 抓图状态（行只注入内存，不写库），在 1400×1100 与 1180×900 下拍
  中文浅色、中文深色、英文浅色、1180 四张。
- 发现与修复：
  - **P2 默认视图名不随语言**：空名字保存时写死英文 `"Saved view"`，而 `defaultSavedViewName`
    这个 key 早就存在、两边译文都有——中文界面里出现一行英文，且该 key 从未被画出来。改：
    走 `s.t().T(i18n.DefaultSavedViewName)`。`TestTheDefaultSavedViewNameSpeaksTheUiLanguage`
    （红：存成 `"Saved view"`，要 `"保存视图"`；绿）。
  - **P2 应用视图后设置页不关**：两块的入口在设置里（第 472 轮的决定），按下“应用视图”恢复
    了全部条件，却把设置页留在结果之上——唯一可见的变化是背后那一页变了，按下去像没反应。
    改：`ApplySavedView` 与 `OpenList` 走同一条离场路径（`closePanels`，退出标签页 / 关系图）。
    `TestApplyingASavedViewTakesYouToTheResults`（红：设置页还盖在结果上；绿）。
  - **P2 两块东西埋在“数据安全”**：两张卡片画在数据安全页末尾，页首那句是“备份与恢复全部在
    本地完成，不会上传任何数据”，既不是列表也不是视图；要找保存视图的读者没有理由打开这一页；
    搜索“列表”也落到数据安全。改：设置新增第五个页签 **列表与视图**，两张卡片整体搬过去，
    关键词表从数据安全移到新页签（shell-contract S58）。
    `TestTheListsAndViewsHaveTheirOwnSettingsTab`（红：页签行没有“列表与视图”；绿，且数据
    安全页不再画列表卡片）；`TestTheSettingsPanelDrawsItsSections` 补第五个页签。
  - **P3 一排同样的图标没有对象名**：列表每行三个图标、视图每行两个，全部共用一个通用名
    （“打开书签”“重命名列表”“删除”“应用视图”），读屏在卡片里听到同一组词一遍又一遍，没有
    一个说它作用在哪一行。源的应用按钮就是视图名本身，删除按钮是
    `${copy.deleteSavedView} ${name}`（`WorkspaceSidebar.tsx:66-75/88`）。改：`settingsIcon`
    同时给 `Label` 与 `Tooltip`，两处调用点传 `打开/重命名/删除 %s`、`应用视图 %s`。
    `TestEachRowActionNamesItsOwnRow`（红：找不到“Open Work”等五个名字；绿）。
- 量过、判定**不改**的四处：
  1. 保存视图仍按单选模型只恢复每个 facet 的第一个值（S18.6/S29.5 的既定用户决定）。
  2. 重名视图允许存在，删除不确认：源同样如此（`handleDeleteSavedView` 直接过滤）。
  3. 恢复视图不动列表范围：源只恢复搜索与筛选，范围由侧栏决定。
  4. 行的顺序、间距、卡片语法与 magpie 的 `.list`/`.warm-card` 一致，深色下三层照旧。
- 新 Inbox：视图行只写名字，不写它恢复了什么（搜索词 / 标签 / 作者 / 媒体）。源也只有名字；
  这是“要不要给每个视图一行副标题”的产品问题。
- 取证：`docs/screenshots/lists-views-light-zh.png`、`lists-views-dark-zh.png`、
  `lists-views-light-en.png`、`lists-views-1180-zh.png`（首次入库）。
- gate：12/14，同前：launch/smoke 与 panes 拒绝在实例运行时开跑（pid 31684），其余全绿。
  第一次跑是 11/15：多出来的一项是 `surfaces` 抓住了新绘画方法 `settingsListsViewsSection` 没
  进 `docs/ui-surface-index.md`——检查器按规矩工作，补上索引（并把页头/页签里的“四个”改成
  “五个”、数据安全的说明去掉“再往下是列表与保存视图”）后通过；`check-wiring` 的孤儿 key
  也从 9 条降到 8 条（`defaultSavedViewName` 终于被画出来）。

## 第 27 轮 — 2026-11-12 — 续十轮第 4 遍：详情媒体与灯箱

- 维度：反馈（一张会动的东西有没有播放入口）、一致性（“会不会动”这件事在几处各判一次）、
  无障碍（预览层叫什么）。
- 方法：把源的 `isVideoMediaType` 全部调用点（`utils.ts:112`、`BookmarkInspector.tsx:20-22`
  详情徽标、`:59/752/764` 预览分支）与原生两处类型比较对照；核对 X 的 `animated_gif` 被
  `parse.go:309` 解析成什么（保留海报 + 取最高码率 mp4）。新增 `detail-media-gif` 抓图状态：
  首屏 100 条里没有 GIF（第一次拍成了普通工作区），改为读一页 1000 条再扫描，深浅各拍一张。
- 发现与修复：
  - **P2 动态 GIF 永远不动**（2026-11-09 记入 Inbox 的那条）：详情与预览都只比较
    `MediaVideo`，GIF 被当成照片，画一张静止海报、没有任何播放入口；而 X 的 GIF 就是 mp4，
    源在两处都按“会动”处理。改：两处改用 `domain.IsVideoMediaType`（它包含 `animated_gif`），
    详情出播放徽标、预览出播放按钮，按钮把 mp4 交给原生播放器。
    `TestAGifOffersPlayInThePreview`（红：GIF 的预览没有播放控件；绿：有，且交付的是 GIF 的
    mp4）。
  - **P3 播放按钮对 GIF 说“播放视频”**：同一个控件现在服务两种媒体，名字要说出是哪一种。改：
    `MediaPlayGif`（“Play GIF” / “播放 GIF”），按钮按条目类型取名。
  - **P3 预览层没有名字**：源是 `role="dialog" aria-modal="true"` + `aria-label=媒体资源`
    （`BookmarkInspector.tsx:499-503`），原生是模态层但没有名称，读屏只报“对话框”。
    改：新增 `mediaTitle`，模态容器带名字。`TestThePreviewNamesItself`（红：找不到“Media”；
    绿）。
- 量过、判定**不改**的四处：
  1. 点遮罩关闭、Esc 关闭、左右键换图、多图才画箭头与计数、关闭后焦点回详情——源的行为，
     原生已有测试（`lightbox_keyboard_test.go`、`TestClosingThePreviewTakesThePlayerAway`）。
  2. 图片本身在源里是 `alt=""`（装饰），原生同样不加替代文本。
  3. 视频没有海报时不画图，只留播放按钮：源会用 mp4 当 poster 得到一个破图框，这里不复刻。
  4. 灯箱的舞台、卡片、计数 pill 尺寸照 `.options-media-lightbox-*` 的数值，深色下照旧分层。
- 取证：`docs/screenshots/gif-preview-light-zh.png`、`gif-preview-dark-zh.png`（首次入库）。
  整页缩放下播放圆盘压在深色海报块上几乎看不见，1:1 裁图确认了圆盘与白色三角。
- gate：12/14，同前：launch/smoke 与 panes 拒绝在实例运行时开跑（pid 31684），其余全绿。

## 第 28 轮 — 2026-11-12 — 续十轮第 5 遍：标签库的编辑、别名与合并

- 维度：数据安全（合并是不可逆的写入）、反馈（按下之后发生了什么、失败为什么）、一致性
  （源在哪些地方先问再做）。
- 方法：把源的合并链路逐行对照——`OptionsApp.tsx:636-658`（空目标直接返回、源里排除目标、
  `window.confirm` 点名幸存者、完成后再清选择）、`TagResultsPane.tsx:912-944`（选择器带占位
  option、按钮 `disabled={!mergeTargetTagId || isSavingTags}`）、`tagsStore.ts:49-110`（重命名
  撞名拒绝并说明原因）。再看原生的 `MergeSelectedTags`、`commitTagMergeTarget`、重命名的两条
  入口。新增 `tags-merge` / `tags-merge-confirm` 抓图状态：批量条只在有勾选时存在，此前没有
  任何一张图拍到过它（L17）。
- 发现与修复：
  - **P1 勾选即替你选好目标，一按就合并**：`commitTagMergeTarget` 在任何一行被勾选时把
    `TagMergeTarget` 写成当前列表的第一个标签，批量条再以 `labels[0]` 兜底，于是“勾三行 →
    按合并”会把它们折进排序第一的那个标签里，删掉三个标签、改写全部关系，全程没有一次是
    读者做的选择。源的目标从空开始，按钮等到选择才可用（`TagResultsPane.tsx:929-943`），
    handler 见空目标直接返回（`OptionsApp.tsx:637`）。改：选择器加占位项“选择标签”、
    `SecondaryButtonWhen` 让合并按钮在选择前禁用、删除 `commitTagMergeTarget` 与它的两个
    调用点；`AskMergeSelectedTags` 在没有目标时给出“请选择合并到的目标标签”。
    `TestMergeNeedsATargetTheReaderChose`（红：`ticking rows chose the merge target "agentic
    ai"`；绿：目标为空、占位符可见、两枚标签都还在）。
  - **P2 合并没有确认**：源在合并前 `window.confirm` 并点名“合并到 X”（`OptionsApp.tsx:646-654`），
    原生按下即写。改：走既有的 `dangerDialog`（确认在前、取消在后并有焦点），正文写数量与
    幸存者：“把 1 个标签合并到“ai agent”？被合并的标签会被删除。”；取消保留勾选。
    `TestMergeAsksBeforeItFolds`（红：`tag "tag-a" was merged before the reader answered`、
    选择被清空；绿：对话框期间两枚标签与勾选都在）、`TestTheConfirmationNamesTheSurvivor`
    （正文含目标名与数量；确认后才删除源、清勾选、清目标、回执计数）。
  - **P3 只勾目标本身也照写，回一句“已合并 0 个标签”**：源把目标从源集合里滤掉后为空即返回
    （`OptionsApp.tsx:641-644`）。改：`mergeSources` 先排除目标，为空时说“请再选一个要合并的
    标签”，不弹确认、不写库。`TestMergingOnlyTheTargetSaysThereIsNothingToMerge`（红：写入
    发生了且回执是 0；绿）。
  - **P2 重命名撞名，中文界面回英文内部句**：服务层早就会拒绝撞名（`service.go:560-574`，
    与源一致），但 UI 直接把 `err.Error()` 画出来，读者看到
    `workspace: cannot rename "rustlang" to "Rust": "rust" already uses that name`。改：
    `tagWriteMessage` 用 `errors.As` 取出 `TagNameTakenError`，译成““Rust”已被另一个标签
    使用。”；面板与侧栏两条重命名入口共用。顺带删掉两处过时注释（还写着“撞名会合并”）。
    `TestARefusedRenameSaysWhyInTheUiLanguage`（红：原句带 `workspace:` 前缀；绿）。
  - **P3 失败的合并会吞掉勾选**：旧代码在写入前清空 `TagSelectedIDs`，失败时读者要重新找回
    那几行。改：清空移进成功回调，失败保留勾选（与源的“合并成功后才清”一致）。
- 删除并替换了一条**把缺陷写成规格**的测试：`TestTheMergePickerCommitsTheTargetItShows`
  断言“选择器显示第一个标签时状态也必须写上它”，正是 P1 的成因；它修的是症状（选择器与状态
  不一致），而不是“为什么替读者选”。新测试改为断言目标从空开始。
- 量过、判定**不改**的四处：
  1. 重命名撞名是**拒绝**而不是合并：服务层与源 `tagsStore.ts:76-90` 一致，本轮只修回执。
  2. 选择器的选项来自全库标签（`TagItems`），源的 `tagItems` 同样是全库（`OptionsApp.tsx:915`），
     过滤只影响行、不影响可合并的对象。
  3. 别名编辑、主题选择仍在原位保存（S28），行的四个动作各有名字、删除仍先问（S29）。
  4. “全选已列出”用表头复选框而不是源的那只按钮：既有决定，行为一致。
- 新 Inbox 两条：① 一次性回执（`TagActionMsg`、`LastBundleReport` 等）无法被任何 capture 状态
  拍到——`ApplyCaptureState` 先跑、`ApplyCaptureLocale` 随后按设计清空它们
  （`capture.go:126-140`、`preferences.go:115-133`）；本轮写过一个 `tags-rename-taken` 状态，
  拍出来是空行，已删除，改为测试断言。② 合并选择器每帧重建全库标签名，源在超过阈值时惰性
  生成选项（`TagResultsPane.tsx:410-419`）；76 标签无感，500 时再看。
- 取证：`docs/screenshots/tag-merge-bar-light-zh.png`、`tag-merge-bar-light-en.png`、
  `tag-merge-confirm-light-zh.png`、`tag-merge-confirm-dark-zh.png`（76 标签真实库首次入库）。
  1:1 裁图核对了占位项、禁用态的合并按钮、以及对话框的按钮次序（确认在左、取消在右并带焦点）。
- gate：12/14，同前：launch/smoke 与 panes 拒绝在实例运行时开跑（pid 31684），其余全绿。
  第一次跑 11/14：`bigfiles` 抓住 `panels.go` 939 行超过 900——合并动作被搬进
  `internal/ui/tagmerge.go`（103 行），`panels.go` 回到 843 行；重跑 12/14。

## 第 29 轮 — 2026-11-12 — 续十轮第 6 遍：词表审阅与建议面板

- 维度：反馈机制（慢动作期间、失败之后、完成之后各看到什么）、操作效率（一次失败要不要重来
  整个审阅）、一致性（源的隔离与重试语义）。
- 方法：读源的内联面板与 apply 循环——`TagResultsPane.tsx:820-905`（错误显示在列表之上而不是
  取代列表、Apply `disabled={!selectedReviewKeys.size || isSavingTags}`）、
  `OptionsApp.tsx:777-819`（每个计划独立 try/catch、失败收集、面板保持、先刷新再报错）。再看
  原生：面板是模态（`shell.go:329`）、apply 在 `tags.go`。**这页此前从未被拍过**：面板在工具栏
  按钮 + 一次 LLM 请求之后，没有任何 capture 状态（L17 的又一例）。本轮补了
  `review-proposals` / `review-partial-error` / `review-failed` 三个状态。
- 发现与修复：
  - **P2 一个计划失败，后面的好计划全被丢掉**：apply 循环遇到第一个错误就 `return`，而
    `runLibraryWrite` 的错误路径只弹一条 toast；面板已经关了，读者既不知道哪些生效、也没有
    地方重试。源逐个隔离并收集失败（`OptionsApp.tsx:783-815`）。改：逐计划执行、失败进
    `leftover`，成功的计数；`done` 里把未落地的计划（连同勾选）放回面板，错误用
    `tagWriteMessage` 逐条译出后以 `; ` 连接。`TestOneFailedPlanDoesNotDiscardTheRest`（红：
    好合并没执行、改名没执行、面板关了、错误为空、列表未收敛；绿：两项落地、面板开着、只剩
    失败的那一条）。
  - **P2 面板在写入前就关了，没有忙碌态**：`ReviewOpen = false` 写在 `runLibraryWrite` 之前，
    一次全库关系重写期间屏幕上什么都没有。源保留面板并禁用 Apply。改：新增
    `state.ReviewApplying`，按下时面板留下、Apply 变 busy（spinner + 禁用）、面板里写“正在
    应用整理建议…”；成功才关闭。`TestThePanelStaysUpWhileItApplies`（红：面板已关、无忙碌
    标记、无句子；绿）。
  - **P2 完成回执画在读者不在的页面上**：`ReviewApplied` 只在结果页工具栏的状态行
    （`toolbar.go:467`），而审阅是从标签页打开的，标签页没有那行工具栏——句子写了，永远看不
    见。改：标签页在 `TagActionMsg` 下面同位置画 `ReviewApplied`（成功色）。
    `TestTheTagPageSaysTheReviewLanded`（红：标签页读不到“已应用 1 项变更。”；绿）。
  - **P2 请求失败没有重试**：失败分支只有“关闭建议”，而模态盖住了工具栏的“整理词表”，重来
    要先关再找。源没有这个问题，因为面板是内联的、按钮一直在上面。改：请求失败（没有任何计
    划）时给 `ReviewRetry`（“重试”/“Try again”）主按钮，直接重新发起；部分失败时错误显示在
    计划之上，Apply 本身就是重试。
    `TestAFailedReviewOffersAnotherTry`（红：唯一的“Try again”来自页面底下、面板里没有；
    绿）。
  - **P3 没勾任何计划也能按 Apply**：按下即关面板并回“已应用 0 项变更”，读起来像成功。
    源在 `!selectedReviewKeys.size` 时禁用。改：`ReviewSelectionCount()` 为 0 时按钮禁用，
    处理函数同样早返回。`TestApplyWaitsForASelection`（红：面板关了且回了 0；绿）。
  - **P3 失败原因的语言**：失败列表原先在 worker 里拼 `err.Error()`，中文界面会夹 `workspace:
    …`。改为在窗口线程用 `tagWriteMessage` 翻译（改名撞名一句已是人话），其余仍保留原文
    （与源一致）。
- 量过、判定**不改**的三处：
  1. 审阅是模态而源是内联：既有结构决定（模态负责焦点与 Escape），本轮把源的三件事（错误在
     列表之上、忙碌、重试）搬进模态，而不是改回内联。
  2. 模型/传输错误原文仍是英文（源同样如此）：本轮只在有类型的地方翻译；要不要把引擎错误按
     类别译成句子是两边一起改的产品决策——记入 Inbox。
  3. 合并先于改名、以及“完成后清空勾选”的顺序保持不变（源同序）。
- 新 Inbox：引擎/模型错误在中文面板里是英文原文（超时、未配置、鉴权）；源也不译，属两边一起
  改的产品决策。
- 取证：`docs/screenshots/review-proposals-light-zh.png`、`review-proposals-dark-zh.png`、
  `review-proposals-light-en.png`、`review-partial-error-light-zh.png`、
  `review-failed-light-zh.png`（全部首次入库）。1:1 裁图核对了复选框与行文的对齐、Apply 的
  主按钮次序，以及部分失败时红色错误行与保留计划的关系。
- gate：12/14，同前：launch/smoke 与 panes 拒绝在实例运行时开跑（pid 31684），其余全绿。

## 第 30 轮 — 2026-11-12 — 续十轮第 7 遍：同步全链路

- 维度：反馈机制（一次同步的三种结局各留下什么）、操作效率（句子里点名的动作要不要自己带控件）、
  一致性（点、状态行、英雄区三处对同一次运行的描述）。
- 方法：把源里与同步有关的表面全部读一遍——`src/popup/components/SyncPanel.tsx`（最近同步时间 +
  状态词 + 立即同步）、`src/lib/runtime/syncStatus.ts:21`（状态或时间变了才重绘）、
  `src/options/OptionsApp.tsx:394`（结果页只画 `lastSyncLabel`）。再看原生：`StartSync` 的
  publish 收尾（`sync.go:125-165`）、英雄区（`sidebar.go:95-140`，S48）、状态点的两个 switch
  （`workspaceStatus` / `workspaceStatusLabel`）、以及**第三个** switch `syncStatusLabel`。
- 发现与修复：
  - **P2 成功的同步没有回执**：一次跑完，屏幕上发生的事是进度行被清空、页脚一个 11px 的日期变了。
    跑了一分钟、存下多少条，读者无从得知。运行本身知道 `InsertedCount`，而状态行浮在所有页面
    之上。改：`SyncSuccess` 时写 `SyncStored`（“已同步 %d 条新书签。”/“Synced %d new
    bookmark(s).”），为 0 时写 `SyncNothingNew`（“同步完成，没有新书签。”）。部分成功与失败
    不弹 toast，因为英雄区已经有自己的句子，两条通道同时说话会互相打脸。
    `TestASuccessfulSyncSaysWhatItStored`（红：`a successful sync said ""; want "Synced 2
    bookmark(s)."`；绿）。
  - **P2 续传句命令读者做一件它不提供的事**：“还有更早的书签，再次同步即可继续。”下面什么
    都没有；启动同步是页脚一个只有 tooltip 的图标。第 21 轮给失败句配了带文字的按钮，正是同
    一个理由，而续传句没有。改：句子下面加 `settingsAction(…, ActionSync, …)`，与失败句的按钮
    同一形状、同一次序。`TestTheResumeLineOffersTheSyncItAsksFor`（红：页面读得到句子、读不到
    “立即同步”；绿）。
  - **P3 取消的同步在状态行里叫“空闲”**：`workspaceStatus` 认 `SyncCancelled`，点也变了色，
    英雄区也会写“已取消”，唯独 `workspaceStatusLabel` 的 switch 漏了这个 case，落到 default
    “空闲”。读者刚亲手停下，同一次运行却被两个词描述。改：补 case。为什么不是“让 label 从
    status 派生”：两者返回的类型不同（一个 domain 枚举、一个 i18n key），派生要给枚举写一张
    完整的 key 表，那和现在一样是两处；本轮的做法是让测试同时断言两者，漏一处就红。
    `TestACancelledSyncSaysCancelled`（红：`a stopped sync is labelled "status.idle"`；绿）。
- 量过、判定**不改**的三处：
  1. 回执文案是新写的：源没有“同步了 N 条”这句话，popup 只画最近同步时间与状态词。措辞按
     `copy.ts` 的语气（“已同步”“同步完成”），中文用全角句号；这是原生独有的一层反馈，不是
     偏离源。
  2. 取消不弹回执：是读者自己按的停止，英雄区与状态点已经写着“已取消”；再弹一条只是复述。
  3. 部分成功不弹回执：`SyncPartial` 的英雄区画的是失败原因与重连按钮（第 21 轮），toast 会让
     同一屏出现两套结论；插入条数已写在同步历史里。
- Inbox：更新“一次性回执拍不到”一条——本轮找到的钩子是“在语言之后写”：`applyCaptureState`
  末尾、`headlessCapture` 之前调用 `ApplyCaptureReceipt`，画面里就有翻译正确的回执。这个钩子
  目前只对同步回执接线；`TagActionMsg`、`LastBundleReport` 等要不要同样处理，仍待定。顺带把
  第 29 轮修掉的两条 Inbox（审阅面板、失败重试）补上“已修”标注。
- 取证：`docs/screenshots/sync-resume-light-zh.png`、`sync-resume-dark-zh.png`、
  `sync-resume-light-en.png`、`sync-done-light-zh.png`、`sync-done-dark-zh.png`（全部首次入库，
  1400×880，真实 9.5k 书签库）。1:1 裁图核对了：续传句与按钮的间距、按钮与第 21 轮失败句按钮
  的同形同序、深色下按钮描边与文字、以及回执在深色里的绿底可读性。
- gate：12/14，同前：launch/smoke 与 panes 拒绝在实例运行时开跑（pid 31684），其余全绿
  （翻译、wiring、board、selfcheck、gofmt、vet、mygo vet、build、test、race、mygo build、几何）。

## 第 31 轮 — 2026-11-12 — 续十轮第 8 遍：导出 / 导入 / 重置

- 维度：数据安全（破坏性动作的回执与承诺）、认知负担（一条灰色句子到底是提示、进度还是结果）。
- 方法：读源的数据安全表面——`SettingsPanel.tsx:1150-1197`（三行：导出、恢复、重置，重置用
  `options-settings-action-row-danger` + `chip-button-danger`）、`DataSafetyPanel.tsx:20-58`
  （`statusText` 三分支：恢复条数 / `localDataReset` / 有效备份的条数与日期，装在
  `InlineMessage tone="info"` 里）、`useDataSafetyActions.ts:56-66`（confirm 之后
  `reloadOptionsPage()`）、`useWorkspaceCommands.ts:180-194`（`setResetResult({success:true})`）、
  `resetLocalData.ts`（清 store、清 settings、清 secrets）。再看原生：数据安全四行与
  `bundle.go` 的四条路径。
- 发现与修复：
  - **P2 重置没有回执**：导出说“备份已导出”、导入说写了多少条，唯独清空一切的重置在完成时把
    进度句清掉、什么也不说。读者看到的是一座空库——而这和一次加载失败长得一模一样。源重置后
    在数据安全面板画 `copy.localDataReset`。改：新增 `LocalDataResetDone`（“本地数据已重置”/
    “Local data reset”，与源 `copy.ts:31/345` 逐字一致），成功后在卡片下方写出。
    `TestResetSaysTheLibraryIsGone`（红：报告为空、页面上读不到这句话；绿）。
  - **P2 窗口记着它刚删掉的设置**：`ResetLocalData` 删掉 settings 表与 Keychain 里的账号，而
    窗口把两者都留在内存里——页面继续写“cookies 已保存”，同步区继续提供“启动时同步”，全是
    已经不存在的值。源的做法是整页重载。改：成功后跑 `reloadAfterReset`——`ApplyPreferences`
    （外观、语言、布局、同步偏好）、`loadAutoTagSettings`、`loadSyncPreferences`、
    `RefreshXCookieStatus`，即设置页打开时读的那一组；密钥值依旧不读进窗口，只读存在性。
    `TestResetForgetsTheSettingsItDeleted`（红：启动时同步仍为真、cookies 仍显示已保存；绿）。
  - **P3 结果与提示同色**：`LastBundleReport` 同时装着进度（“正在导出…”）、结果
    （“备份已导出”）和拒绝（批量过多），而页面统一用 `settingsNote` 的 --muted 12px 画，于是
    刚完成的操作读起来像卡片下面又一段说明。改：新增 `state.LastBundleOK`，结果置真、进度与
    拒绝置假；数据安全页在结果为真时用 `theme.Success` 画。第一次拍图时画面仍是灰的——抓图
    钩子只写了句子没写标记，正好证明这条规则真的由标记决定；补上后才是绿色。
  - **P3 代码注释与代码相反**：`importexport/service.go` 的 `ResetLocalData` 先调用
    `clearSecrets`，紧接着一段注释却说“密钥不在这里清除，这是一个已知缺口”。改写为事实：
    先清密钥、失败即中止，`clearer` 由 `main.go` 注入，因为账号属于 credentials 包。
- 量过、判定**不改**的三处：
  1. 原生比源多一行“扩展兼容备份”：扩展打不开 `.xmbundle`，而这正是扩展能导入的
     `exportVersion 2` 格式；它是桥，不是重复。
  2. 重置在这里连 X cookies 一起清（`credentials.Store.DeleteAll`），源做不到——它的 cookie
     属于浏览器而不是扩展存储。确认句写的是“本地设置”，原生覆盖得更多，方向是安全的一侧。
  3. 回执是卡片下方的一行文字，源是 `InlineMessage` 的浅色盒子：沿用设置页自己的说明面，
     只把语气（成功色）补上，结构差异记入契约。
- Inbox：导入 / 恢复的回执与导出一样是一次性句子，抓图钩子（`ApplyCaptureReceipt` / `ApplyCaptureResetReport`
  这种“语言之后写”）目前只接了同步与重置两条；要不要把导入也接上，待定。
- 取证：`docs/screenshots/settings-reset-light-zh.png`、`settings-reset-dark-zh.png`、
  `settings-reset-light-en.png`（首次入库，1400×880，真实 9.5k 书签库）。1:1 裁图核对了回执与
  卡片的间距、浅色 `#189a58` 与深色 `#6fd99b` 的可读性，以及中文与英文的同一行高。
- gate：12/14，同前：launch/smoke 与 panes 拒绝在实例运行时开跑（pid 31684），其余全绿。
  第一次跑 11/14：`twins` 抓住 `ResetLocalData` 与新增的 `LocalDataReset` 归一化后同名不同值
  ——动作名与回执名确实是两件事，把新键改名为 `LocalDataResetDone`（值仍是源的
  `localDataReset`），重跑 12/14。

## 第 32 轮 — 2026-11-12 — 续十轮第 9 遍：动效、过渡与减少动态

- 维度：视觉一致性（同一套动效语言在整站是否一致）、可达性（系统要求减少动态之后还剩什么）。
- 方法：把每个动效按“谁在驱动”分类，再逐个问它是否读 `ReduceMotion`：
  1. 入场（`entrance.go` 六个 keyframe）——`EntranceProgress` 在减少动态时直接返回目标值。
  2. 颜色过渡（`animateColor` / `hoverColor`）——MyGo 的 `AnimateColor` 按设计不读这个设置，
     所以本仓库包了一层。
  3. 位移/缩放（`AnimateTo`：按钮按压、导航高亮、选择打勾）——MyGo 的 `AnimateTo` 自己会读。
  4. 页面与浮层（`pageEntrance`、`popoverEntrance`）——按源的 media query 只去位移、留淡入。
  5. 等待态（`pulse`、`RunProgressView` 的不定长条纹）——条纹已按源的
     `@media (prefers-reduced-motion)` 改成静止满宽 0.6 透明度；`pulse` 没有。
- 发现与修复：
  - **P2 减少动态下骨架屏与同步圆点停在 55% 透明度**：`pulse` 是 0.55↔1 的
    `RepeatForever + Alternate` 关键帧。MyGo 在减少动态时把序列取到 `final()`，而
    `final()` 对“永不结束的往返”返回**第一帧**（`motion.go:646-651`：RepeatForever 的
    plays() 为 0，偶数 → `frames[0]`）。于是要求减少动态的读者看到的是比周围更淡的骨架条
    和一个像“停用”的圆点，而函数上方的注释恰恰写着“帧结束在 1，所以会看到稳定的全亮状态”。
    改：`pulse` 在减少动态下直接返回 1，不启动循环。`TestAReducedMotionPulseStaysFullyVisible`
    （红：`a pulse under reduced motion shows at opacity 0.55`；绿）。
  - **P3 令牌注释指错了元素**：`MotionWidthMs` 写着“驱动侧栏折叠，200ms ease-out”，而侧栏
    折叠是 `.st-pane` 的 0.3s 淡出（`MotionSlow`），200ms 的宽度过渡属于
    `.options-autotag-progress-fill`（`extension.css:4658`）。注释改成事实，并注明这条过渡
    尚未实现（进度条按精确比例落笔，一次批量是一次跳变，不是滑动）。整段注释也从“三种动效”
    改为“过渡令牌”的说法，避免再被读成全集。
- 量过、判定**不改**的三处：
  1. 浮层入场在减少动态下仍保留淡入：源的两条 media query 只关三处 `animation` 与不定长
     条纹，淡入是保留下来的；本仓库对灯箱的处理比源更严格（源漏掉了灯箱），是记录在案的
     分歧，不收回。
  2. 按压的缩放仍会发生（瞬时）：源的 media query 只作用于 `animation` 与若干 `transition`，
     `:active` 的 transform 不在其中；MyGo 在减少动态下即时到目标值，语义一致。
  3. 不定长进度条纹已按源改成“静止、满宽、0.6”，本轮未动。
- Inbox：进度条宽度没有 200ms ease-out 过渡（源 `extension.css:4658`）。要做得逐帧驱动，
  而仓库里断言动画中间值的手段只有“按真实时间 sleep 后取帧”，不可靠；记在板上，等有动画
  测试夹具时再做。
- 取证：`docs/screenshots/sync-running-light-zh.png`、`loading-light-zh.png`（1400×880，真实
  9.5k 书签库）。抓图路径本来就把减少动态强制为真，所以这两张恰好是“要求减少动态的读者”看到
  的状态：骨架条与圆点都是全亮。旧的 `loading.png` 已不是有效 PNG、旧的 `sync-running.png`
  早于 `pulse` 引入，做不出同状态的前后对照——本轮的验收是那条红测试，截图只证明现在的状态。
- gate：12/14，同前：launch/smoke 与 panes 拒绝在实例运行时开跑（pid 31684），其余全绿。

## 第 33 轮 — 2026-11-12 — 续十轮第 10 遍：真实体量收尾（76 标签 / 9.5k 书签）

- 维度：认知负担、操作效率（真实库下每个动作是否仍然可用）、性能。
- 方法：用真实库（9,562 条书签、76 个标签）逐页走一遍，对每个高频动作只问一句：“按下去
  会发生什么？”——不看小样本 fixture 的断言，看真实数量下它落在哪。
- 发现与修复：
  - **P2 真实体量下批量选择无路可走**：批量条上唯一的“选择全部结果”走
    `MatchingIDs`，在 9,562 条时超过 `MaxLibraryPageSize`（1000，`libraryquery.go:203`），
    于是每次都只回一句“一次选择的条数过多”，一条也不选。拒绝本身是对的（批量动作不能跑在
    部分集合上），缺的是源里的**第二个按钮**：`BookmarkResultsPane.tsx:45-52` 并排给出
    “选择当前结果”（全部匹配）与“选择已加载”（列表当前持有的行），文案在
    `copy.ts:58-59/372-373`。原生只有前者，且叫“选择全部结果”——名字比行为大。
    改：
    1. 新增 `SelectLoadedResults()`（`inspector.go`，紧随 `SelectAllResults`）：选中
       `state.Data.Bookmarks`，同时清掉上一次的拒绝句，因为那句话已经不再成立。
    2. 批量条按源顺序并排两个按钮（`toolbar.go` `batchBar`）：`选择当前结果` 在前、
       `选择已加载` 在后，然后才是 `取消选择` / `删除`。
    3. i18n：`BatchSelectAll` → `BatchSelectCurrent`（“Select current results” /
       “选择当前结果”），新增 `BatchSelectLoaded`（“Select loaded” / “选择已加载”）。
- 量过、判定**不改**的一处：`SelectAllResults` 在超限时仍然整体拒绝，不退化成“选中前 1000
  条”。批量打标/移动会作用在集合上，偷选一个前缀等于让读者在没有告知的情况下漏掉 8,562 条。
  拒绝加第二个按钮，比“贴心的部分选择”诚实。
- 红/绿：`TestARealScaleLibraryCanStillBeBulkSelected`（1,200 条种子库）。红时界面文本只有
  `Select all results`、`Too many results to select at once`、`100 / 1200 results`——拒绝句
  在那儿，出路不在；绿后同一屏出现 `Select loaded`，点一下选中列表持有的 100 行。
- 取证：`docs/screenshots/batch-selected-scale-light-zh.png`、
  `batch-selected-scale-dark-zh.png`、`batch-selected-scale-light-en.png`（1400×880，真实
  9,562 条库）。按 1:1 裁剪（`/tmp/b1.png`，0,150 2800×360）确认：`选择当前结果`、
  `选择已加载`、`取消选择`、`删除` 自左向右排列，右对齐于批量条同一基线，与源的两个选择按钮
  同序；中英文两张都在。
- Inbox：关闭 2026-11-12 那条“选择全部结果一次选中 9,562 条 / 源拆成两个按钮”——本轮按源
  补齐，并保留拒绝。
- gate：12/14，同前：launch/smoke 与 panes 拒绝在实例运行时开跑（pid 31684），其余全绿。
- 十轮收尾：第 24–33 轮全部完成，代码提交 `b9e3e94`…`f6d216c`，每轮都有红测试、真实体量
  截图与板上的一行。下一段工作的入口仍是 `board.md` 的 Inbox 与 Watch。

## 第 34 轮 — 2026-11-12 — 点击走查第 1 遍：数据备份 / 导出 / 恢复 / 重置

- 维度：操作效率、反馈机制。用户报告：“点击数据备份，导出数据的时候，它就没有任何响应。”
- 方法：先证明按钮是接上的，再看它接去了哪里。用 `ui.NewTester` 打开设置·数据安全，点
  “导出数据”：处理器确实跑了（`saveFile` 被调用一次），所以问题不在“没接”而在“接上之后
  卡住”。读 mygo 的实现：`Dialog.Save` 在**当前 goroutine 上阻塞**直到面板被关掉
  （`mygo dialog.go:9-11`）；而这条命令是在点击处理器里直接调它的。
- 发现与修复：
  - **P1 文件面板开在点击处理器里，点击等于把窗口挂起**：处理器不返回 → 窗口不能重画；
    面板若没有出现（或出现在别处），点击就没有留下任何状态变化，读者看到的是“死按钮”。
    这正是“导出没有任何响应”的形状。修：
    1. 预览用的文件面板（`Dialog.Save`/`Dialog.Open`）移进 `runInBackground` 的工作里。
       mygo 明确写着对话框“can be called from any goroutine”，面板由主线程从 post 里呈现，
       处理器立刻返回。涉及 `ExportBundle`、`ExportExtensionBackup`、`AskBundleImport`。
    2. 点下去的第一帧就有话：`BundleBusy` 置位、数据安全页的四个文件按钮变灰、状态行写
       “正在选择保存位置…”。取消面板则清掉这句话与忙碌位（`finishBundleWork`），不留下
       “命令还在跑”的假象。
    3. 导出本身仍然在窗口线程之外（原 `TestExportRunsOffTheWindowThread` 的理由不变）。
  - 顺带把四个文件命令的结束路径收成一个 `finishBundleWork(err)`：成功以外的结局（取消、
    失败）只有一种画法，不再各写一遍。
- 红/绿：`TestClickingExportAsksForAPathOffTheWindowThread`（红：
  `the save panel was opened inside the click handler: the window cannot draw until it is dismissed`；
  绿），`TestAnExportFinishesAfterThePanelAnswers`；另外三条既有测试（`TestExportRunsOffTheWindowThread`、
  `TestAFileCommandDoesNotStack`、`TestACancelledDialogDoesNothing`）按新契约改写——它们原来
  断言的正是“面板在处理器里同步打开”。
- 取证：`docs/screenshots/settings-export-light-zh.png`、`settings-export-dark-zh.png`、
  `settings-export-light-en.png`（1400×880，真实 9.5k 书签库）。画面：四个文件按钮全部变灰，
  卡片下方一行“正在选择保存位置…”。1:1 裁剪确认英文为 “Choosing where to save…”。
- 未能证明的一半：测试能证明“处理器先返回、工作里有面板调用”，但**没有**在测试里真的弹出
  一个 NSSavePanel——那需要一个真人。面板现在由主线程从 post 呈现，路径与其它后台工作一致；
  板上的 Inbox 留了一条，等待用户实测确认。
- gate：12/14。除既有的 launch/smoke 与 panes 拒绝（实例 pid 82094 在运行）外，本轮还先
  撞到一次 gofmt（新增的 i18n 键名变长，对齐变了），格式化后恢复。

## 第 35 轮 — 2026-11-12 — 点击走查第 2 遍：工具栏与搜索（含条件 chip）

- 维度：操作效率、可达性。口径同第 34 轮：每个控件都要被“按着它画出来的字”点一次。
- 方法：`ui.NewTester` 里按标签点排序、视图切换、筛选按钮、筛选条件、搜索清除；每一步断言
  它命名的那个状态。点击用 `tester.Click(标签)`，所以“标签所在的元素不是可点元素”会直接
  暴露成状态不变。
- 发现与修复：
  - **P2 条件 chip 只有 16 点的 × 能按，chip 上的字什么都不做**：源里整个 chip 就是一个
    `<button onClick={() => clearRefinement(key)}>`，字和 × 都在里面
    （`WorkspaceToolbar.tsx:169-177`）。原生把 × 做成了独立控件，字是死的，而 chip 悬停时
    整块变色的反馈写着“这里可以按”。改：`conditionChip` 的整行成为控件
    （`Focusable().Label("Remove <条件>").Tooltip("Remove").OnClick(chip.Remove)`），× 退回
    纯绘制。**只能有一个处理器**：facet 的 Remove 是 toggle，chip 与 × 各挂一次会把条件又
    打开。`TestTheConditionChipIsOneButton`（红：`pressing the condition's own word did
    nothing: only its x removes it`；绿，并断言 × 那条路径没有二次触发）。
  - **P2 所有纯图标控件没有可访问名称**：`IconOnlySized` 只设 `Tooltip`，而注释断言
    “tooltip 就是可访问名称”。测试器的 `Find`（“anything that carries a text or an
    accessibility label”）一个都找不到——搜索框的清除按钮因此像是不存在，读屏器也拿不到
    名字。改：`Label(tooltip)` 与 `Tooltip(tooltip)` 并列，和设置行里的 `settingsIcon` 一致。
    连带修正 `TestAFailedSyncOffersTheWayToTryAgain`：footer 的同步图标现在有名字，原来的
    “屏幕上有没有 Sync 这个词”不再能区分块内重试与全局命令，改为数个数（死会话时只剩 1 个）。
- 新测试：`TestTheToolbarAnswersEveryClick`（排序循环三种顺序且标签跟随、视图切换、筛选面板
  开关、条件 chip 整块可移除、搜索清除按钮清空查询）。
- 取证：`docs/screenshots/filter-chips-scale-light-zh.png`（1400×880，真实 9.5k 书签库，标签
  `3d modeling` 与作者 `@Geek` 两个条件在列）。chip 从“只有 × 能按”到“整块能按”是交互
  变化，静止画面看不出差别，本轮的验收是那两条测试；图证明 chip 在真实体量下的排布。
- 工具账：抓图的语言参数只认 `zh-CN`；写 `zh` 会被 `i18n.ValidLocale` 判为无效并静默回落
  英文（本轮第一次抓出来的“中文”图其实是英文）。已在 Inbox 记一条，重拍的两张
  `settings-export-*-zh.png` 用 `zh-CN`。
- gate：12/14，同前（实例 pid 82094 在运行，launch/smoke 与 panes 拒绝）。

## 第 36 轮 — 2026-11-12 — 点击走查第 3 遍：侧栏

- 维度：操作效率、可达性（口径同 34/35：按控件画出来的字点，断言状态）。
- 方法：`TestTheSidebarAnswersEveryClick` 从侧栏顶部走到页脚：标签/作者页签、搜索开关、
  “+” 新建标签、标签行、作者行、表格图标进标签库、标签库的“返回书签”、主题循环、设置齿轮、
  语言 chip。每一步断言它命名的状态。
- 结果：**全部通过，无缺陷**。十一个控件都留下了它们承诺的状态变化；标签行与作者行都真的
  加入了筛选集合。
- 记下的一处测试歧义（不是产品缺陷）：进入标签库后，“Manage tags”同时是侧栏图标、页面标题
  与页签名的名字，`tester.Click` 取第一个匹配，无法用它验证“再点一次表格图标返回工作区”。
  返回路径改用页面自己的“返回书签”（唯一名字）来断言；侧栏图标的返回路径仍由人工点击覆盖。
- 产出：`TestTheSidebarAnswersEveryClick` 成为该列点击行为的回归测试（第 35 轮的两个缺陷
  都是“看着能点、其实点不到”，这一类只有真点一次才会暴露）。
- 取证：`docs/screenshots/sidebar-authors-scale-light-zh.png`（1400×880，真实 9.5k 书签库，
  作者页签：20 位作者连头像、显示名、handle 与计数；页脚 中/主题/同步/设置）。
- gate：12/14，同前（实例 pid 82094 在运行，launch/smoke 与 panes 拒绝）。

## 第 37 轮 — 2026-11-12 — 点击走查第 4 遍：结果卡片与右键菜单

- 维度：操作效率、可达性。口径同前：按控件画出来的字点，断言状态。
- 方法：`TestTheCardAnswersEveryClick` 走一张卡片的三层交互：角上的选择框、卡片本身、
  右键菜单；每一步都检查“有没有误触下一层”。
- 发现与修复：
  - **P2 每张卡片的选择框都叫同一个名字**：`Select bookmark` 在 2000 条结果里重复 2000 次。
    源把它命名成 `selectBookmarkForBulk` + 作者（`BookmarkCard.tsx:142`），读屏器因此能听出
    这一格属于哪条。原生只有单词，一列同名按钮等于都没有名字。改：新增
    `CardSelectBookmarkFor`（“Select bookmark by %s” / “选择 %s 的书签”），用
    `AuthorName`（空则 handle）；可见 tooltip 仍是单词——它提示的是控件，不是这条书签。
    红：`the card's box is not named after its author`，文本里两处一模一样的
    `Select bookmark`；绿后按作者名可找到。
- 量过生效的三处（无缺陷）：
  1. 点选择框只改选择集合，不打开详情（`stopPropagation` 等价成立）；
  2. 卡片的 “Open on X” 只调浏览器，不打开详情；
  3. 右键菜单五项齐全（打开、在 X 打开、复制文字、复制链接、加入选择），选“加入选择”
     与“复制链接”都留下了可观察的结果。
- 取证：`docs/screenshots/results-cards-scale-light-zh.png`（1400×880，真实 9.5k 库）。
- gate：12/14，同前（实例 pid 82094 在运行，launch/smoke 与 panes 拒绝）。

## 第 38 轮 — 2026-11-12 — 点击走查第 5 遍：详情覆盖层

- 维度：可达性、操作效率。口径同前：按控件画出来的名字点，断言它改变了什么。
- 方法：`TestTheDetailAnswersEveryClick` 按指针的走法走一遍详情：作者的画面与 handle、
  标签 pill 的 ×、新标签输入框回车、复制链接、在 X 中打开、两张图片与灯箱的左右箭头和
  关闭，最后是详情自己的关闭。每一步都检查“有没有误触下一层”（点标签不关详情、点外链
  不关详情、关灯箱不关详情）。
- 发现与修复：
  - **P2 详情右上角的关闭控件没有可访问名称**：它是手写的 26 点图标盒，只设了
    `Tooltip(InspectorTitle)`，没有 `Label`。第 35 轮把 `IconOnlySized` 补上名字时，
    这个控件因为不走那个公共构造函数而漏掉了——它是覆盖层里唯一一条用指针退出的路，
    读屏器读不出来，测试器也找不到。源给它的 `aria-label` 就是 `detailsTitle`
    （`BookmarkInspector.tsx:174-177`），`shell_test.go:177` 的注释也早写明了这一点，
    只差实现。改：加 `Label(s.t().T(i18n.InspectorTitle))`，tooltip 不变。
    `TestTheDetailHeroShowsTheAuthorRatherThanATitle` 原先按 `Texts()` 里出现 “Details”
    判红，但 `Texts()` 同时包含可访问名称；改为“这个词只允许出现在卡片右缘”（标题会在
    作者行的左半边），可见标题仍然被守住。
    红：`clicking "Details": ui: no element shows "Details"`；绿后同一步关闭覆盖层。
- 量过生效的八处（无缺陷）：
  1. 作者：头像有名字（`detail-avatar`），handle `@ada` 在页头；
  2. 标签 ×：按 “移除 rust” 找到，按下后该条不再带 rust；
  3. 标签输入框：点入、输入、回车，标签建好并贴上，输入框清空；
  4. 复制链接：原地变勾 + toast；
  5. 在 X 中打开：浏览器收到这条的 URL，详情不关；
  6. 两张图片：第一张按下去在 0 号打开预览；
  7. 预览的左右箭头：1 ↔ 0（与源一致，可循环）；
  8. 预览关闭：只关预览，详情还在。
- 取证：`docs/screenshots/detail-overlay-scale-light-zh.png`（1400×880，真实 9.5k 库，
  `XBM_CAPTURE_STATE=detail`，zh-CN）。
- gate：12/14，同前（实例 pid 82094 在运行，launch/smoke 与 panes 拒绝）。

## 第 39 轮 — 2026-11-13 — 点击走查第 6 遍：批量条

- 维度：可达性、操作效率。口径同前：按控件画出来/被命名的样子点，断言状态。
- 方法：`TestTheBatchBarAnswersEveryClick` 勾两行后把整条走一遍：计数、输入框回车、
  标签选择器 + 应用、列表选择器 + 移动、选择已加载、选择当前结果、取消选择、
  删除（确认框弹出后取消）。
- 发现与修复（同一类：名字）：
  - **P2 打标签输入框没有任何名字**：`ui.TextInput` 既无占位符也无 `Label`，读者看到一条
    空白输入框，读屏器只念「文本栏」。它是本应用自己加的（源的批量条只有选择器），所以
    源里没有可抄的名字。改：`Placeholder` + `Label` 都用 `createTagPrompt`
    （“Enter a new tag name” / “输入新标签名称”），问句与名字是同一句。
  - **P2 两个选择器与旁边的按钮同名**：标签选择器叫「Add tag」而旁边的图标按钮也叫
    「Add tag」；列表选择器与「Move to list」按钮同名。相邻两个同名控件，读屏器分不出
    哪个会弹列表。源的选择器本来就叫 `chooseTag`（`BookmarkResultsPane.tsx:50-53`）。
    改：两个触发器的 `Label` 改为各自占位文案（“Choose tag” / “Choose list”），
    按钮保持「Add tag」「Move to list」——一个说选什么，一个说按了做什么。
    红：`clicking "Enter a new tag name": ui: no element shows ...`；三处改名后同一步
    全部可点。
- 量过生效的五处（无缺陷）：
  1. 输入框回车把两行都打上标签，toast 说“Tagged 2 bookmarks”，选择清空；
  2. 标签选择器选 rust + Apply：两行都带上 rust；
  3. 列表选择器选 Work + Move：toast 写明去向“Moved 2 bookmarks to Work”；
  4. 「选择已加载」选中列表里的 3 行；「选择当前结果」在有小样本时同样选中 3 行
     （9.5k 时会拒绝，第 33 轮已有“选择已加载”这条出路）；
  5. 删除先弹确认；取消后库仍是 3 条。
- 取证：`docs/screenshots/batch-bar-scale-light-zh.png`（1400×880，真实 9.5k 库，
  `XBM_CAPTURE_STATE=batch-selected`，zh-CN）。
- gate：12/14，同前（实例 pid 82094 在运行，launch/smoke 与 panes 拒绝）。

## 第 40 轮 — 2026-11-13 — 点击走查第 7 遍：标签库

- 维度：可达性、操作效率。口径同前：按控件被命名的样子点，断言状态。
- 方法：`TestTheTagLibraryAnswersEveryClick` 在真实表格里走一遍：六个行内控件、主题格、
  两个行复选框、批量合并（含确认）、清理孤儿标签（含确认）、以及页面自己的新建行。
- 发现与修复（同一类：行身份）：
  - **P2 表格里每行的动词与复选框都不带行名**：几十行里几十个「Rename tag」「Edit aliases」
    「View bookmarks」「Delete」，选择列的复选框完全没有名字，主题格在每一行未设主题时
    都念「No theme」。读屏器分不出是哪一行，测试也只能点到第一行。源本来就逐行拼名字
    （`TagResultsPane.tsx:118` 主题格、`:148-199` 四个动词与复选框）。改：新增
    `TagSelectRowFor` / `TagBookmarksFor` / `TagRenameFor` / `TagAliasesFor` /
    `TagThemeFor` / `TagDeleteFor` 六个格式串，行内全部带上标签名；主题格的名字从「值」
    改成「哪个标签的主题」。
  - **P2 新建行的输入框没有名字**：占位符是画上去的，不是名称，读屏器只念「文本栏」。
    改：`settingsInput` 加 `Label(createTagPrompt)`（与第 39 轮批量条同一条规则）。
    红：`the row for go has no "Select tag go"`（临时还原复选框命名取证）、
    `clicking "Enter a new tag name": ui: no element shows ...`；绿后同一批步骤全通。
- 量过生效的五处（无缺陷）：
  1. 铅笔把该行名字变成输入框，输入 `lang` + 回车 → 标签名成为 `golang`；
  2. 标签图标在别名格打开编辑器，输入 `gopher` + Save → 别名写入；
  3. 主题格打开自己的选择器，Escape 不写入；
  4. 两个复选框选中两行；合并选择器 `End` 选中 `rust`，Merge 先弹确认（随后取消），
     来源只剩被折进去的那一个；
  5. 有孤儿时出现清理按钮，按下先确认；新建行 `Create tag` → 输入 `zeta` → Add 建出，
     再开一次 Cancel 收起。
- 取证：`XBM_CAPTURE_STATE=tags` 重新抓图后与既有 `docs/screenshots/tags-scale-light-zh.png`
  逐字节相同（本轮改动只涉及名字，像素不变），沿用该文件（1400×880，真实 9.5k 库/76 标签）。
- gate：12/14，同前（实例 pid 82094 在运行，launch/smoke 与 panes 拒绝）。

## 第 41 轮 — 2026-11-13 — 点击走查第 8 遍：设置页

- 维度：可达性、功能正确性。口径同前：按控件画出来/被命名的样子点，断言状态。
- 方法：`TestTheSettingsPageAnswersEveryClick` 把设置页整页走一遍：返回、五个页签、
  通用页的语言与主题选择器、同步页的第一个开关、自动标签页的跳转、查找框（输入、
  筛选、退格清空）、再返回。
- 发现与修复：
  - **P1 查找框只收得下一个字符**：点进“搜索设置”，打第一个字母，输入框立刻失焦；
    再打的字全丢，退格也不生效，页面停在只按一个字母筛过的状态。原因是同一个框有两个
    绘制点——筛选时用 `Grow(1)` 的那一个，不筛选时用固定宽度的另一个——MyGo 看到子元素
    换了，就把旧输入框连同焦点一起丢掉，新框接不上键盘。改：框只在一个地方画，宽度
    由调用方在同一个元素上调整；页签和弹簧不再被跳过，改用 `Invisible()` 占位，元素的
    身份逐帧不变。`settingsFindField` 去掉 `grow` 参数并返回元素（`settingsui.go:486-501`），
    页头只剩一个调用点（`settingsview.go:102-121`）。红：`clearing the find field left the
    page filtered`、`the tabs did not come back after the query was cleared`；调试测试打出
    `typed b: query="b" focused=false` 定位到失焦。绿：同一批步骤全通。
  - **P2 返回控件只有 tooltip、没有名字**：这是设置页唯一一条用指针退出的路，与第 38 轮
    修掉的详情关闭是同一个遗漏（第 38 轮已记入 Inbox）。改：`Label(SettingsBack)` +
    `Tooltip(SettingsBack)`，与图页、标签库的返回一致（`settingsview.go:86-92`）。红：
    `the settings page's back control has no name`；绿：同一步关掉设置页。
- 量过生效的五处（无缺陷）：
  1. 五个页签各自切换 `SettingsSection`，通用页签回来时查找框清空；
  2. 主题选择器 `End` + `Enter` 改变外观；语言选择器同理（语言一项由固定截图核对）；
  3. 同步页第一个开关翻转“启动时同步”并落盘；
  4. 自动标签页的“打开标签管理”离开设置、落到自动标签页；
  5. 返回关掉设置页、回到结果。
- 取证：`docs/screenshots/settings-scale-light-zh.png`（1400×880，真实 9.5k 库/76 标签，
  `XBM_CAPTURE_STATE=settings`，zh-CN，light）。页头在 1:1 裁切下核对：返回箭头、设置、
  五个页签（通用在 pill 上）、右端查找框。
- gate：12/14，同前（实例 pid 82094 在运行，launch/smoke 与 panes 拒绝）。

## 第 42 轮 — 2026-11-13 — 点击走查第 9 遍：自动标签页与建议面板

- 维度：可达性、功能正确性。口径同前：按控件画出来/被命名的样子点，断言状态。
- 方法：两条点击普查。
  `TestTheAutoTagPageAnswersEveryClick` 从标签页的页签进入自动标签页，点第一个开关的
  两半、加两条关键词规则并按名字删掉、最后按「返回书签」离开；
  `TestTheRunAndItsSuggestionsAnswerEveryClick` 用假 provider（HTTP 服务器）按「立即分析」
  启动、在途时按「停止」、等这一批落地后按「1 Pending tags」开面板、按该行的接受按钮。
- 发现与修复（同一类：一列里重复出现，名字只说了动词）：
  - **P2 关键词规则的删除控件每行都叫「Remove」**：一行一个 ×，两行就有两个同名控件，
    读屏器和测试都只能落到第一行。改：`SettingsRemoveKeywordRuleFor`（“Remove keyword
    rule %s” / “移除关键词规则 %s”），名字带关键词。源本身用的是同一个通用标签
    （`SettingsPanel.tsx:788-793`），这里按 L28 偏离源，代码注释写明了原因。
  - **P2 建议面板每行的接受/拒绝只叫「Accept suggestion」「Reject suggestion」**：十条建议
    就是十个同名按钮。源本来就把标签名拼进名字（`BookmarkCard.tsx:201`、
    `BookmarkInspector.tsx:438`）。改：`SuggestionsAcceptFor` / `SuggestionsRejectFor`
    两个格式串。红：`clicking "Accept suggestion go": ui: no element shows ...`。
  - **P2 规则编辑器把「Add rule」画了两遍**：行名和按钮用同一句话，行名的宽度覆盖整行，
    点击落在名字上，按钮永远按不到（测试里加了两条规则，`KeywordRules` 仍是 0）。改：
    编辑器行不再设 `name`，只留按钮这一个同名节点（L30 的又一例）。绿后同一步加到两条。
- 量过生效的四处（无缺陷）：
  1. 页签进入自动标签页，返回书签退出管理模式；
  2. 第一个开关「开启」「关闭」两半各自翻转并落盘；
  3. 「立即分析」启动一次真实链路（假 provider 收到请求），在途时按钮变成「停止」；
  4. 按「停止」置取消位，这一批不被中断、落地后仍存下 1 条建议；按「1 Pending tags」打开
     面板，按该行的接受按钮后书签打上 go、待审建议清零；Escape 关掉面板。
- 取证：`docs/screenshots/autotag-scale-light-zh.png`（1400×880，真实 9.5k 库/76 标签，
  `XBM_CAPTURE_STATE=tag-autotag`，zh-CN，light）。规则编辑器在 1:1 裁切下核对：关键词
  输入框、→、标签输入框、添加规则，左侧不再有重复的行名。
- gate：12/14，同前（实例 pid 82094 在运行，launch/smoke 与 panes 拒绝）。

## 第 43 轮 — 2026-11-13 — 点击走查第 10 遍：菜单与危险确认

- 维度：可达性、操作效率（菜单是导出/导入/恢复/重置的唯一入口）。口径同前。
- 方法：三条测试覆盖菜单栏与它的命令。
  `TestTheMenuBarIsCompleteAndLocalized` 深度遍历 `buildMenu`，在两种语言下各重建一次，
  检查每个可点项都有 ID 与非空标签、ID 不重复、标签随语言变化；
  `TestEveryMenuCommandDoesSomething` 按下所有不需要面板的服务态命令（外观三项、设置、
  建议、添加书签、侧栏、重置）；
  `TestTheMenuFocusSearchLandsInTheToolbar` 按「显示 > 查找」断言光标落在工具栏搜索框里。
- 发现与修复：
  - **P2 工具栏搜索框没有可访问名称**：magpie 改版把 `ui.SearchField` 换成了自建的
    `TextInput`（为了能翻译占位符），重建时只留了 `Placeholder`，没设 `Label`——占位符是
    画上去的字，不是名字（L27）。这是全应用使用最多的输入框，也是 Cmd-F 落点；读屏器
    念不出它是什么。红：`the toolbar's search field has no name`。改：`Label(placeholder)`；
    同一个 `searchField` 助手同时修好结果页、标签页与侧栏的搜索框（`icons.go:168-175`）。
- 量过生效的七处（无缺陷）：
  1. 「显示 > 外观」三项各自改变主题（深色 / 浅色 / 跟随系统）；
  2. 「显示 > 设置」打开设置页；
  3. 「文件 > 审阅」打开建议面板；
  4. 「文件 > 添加书签」展开手工添加表单；
  5. 「显示 > 切换侧栏」记下读者的选择；
  6. 「文件 > 重置本地数据」先进入确认态，不直接动手；
  7. 「显示 > 查找」把光标放进搜索框（修好名字后才能这样断言）。
- 危险确认：删除书签（第 39 轮）、合并标签与清理孤儿（第 40 轮）都以「先问、取消后不动库」
  覆盖；重置本轮以菜单项进入确认态。确认框是原生 sheet，测试用 `Answer` 驱动它的两路，
  取消即拒绝由 `confirm.go` 的 fail-closed 规则保证。
- 取证：重建后重抓 `XBM_CAPTURE_APPEARANCE=light`、zh-CN、1400×880 的真实体量工作区，与
  `docs/screenshots/workspace-scale-light-zh.png` 逐字节相同（本轮只改名字，像素不变），
  沿用该文件。
- gate：12/14，同前（实例 pid 82094 在运行，launch/smoke 与 panes 拒绝）。
