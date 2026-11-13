# 界面表面索引（主区）

**用途**：在动手加一个新方法之前，先确认"这个表面是不是已经有了"。

**怎么用**：写一个新方法之前，在这张表里找一次它的名字（或它的近义词）。**不在这里，才动手。**

**维护**：这份表和代码会脱节 —— **清单没有被维护就等于没有**。**所以每次新增或删除一个绘制方法，同一次改动里改这张表。**

---

## 主区（结果列）

| 方法 | 文件:行 | 何时出现 |
|---|---|---|
| `viewEnds` | `viewends.go:51` | 网格/列表能滚动时挂载，读者越过一屏后在页面右下角淡入（`.view-ends`，`app.css:2255-2266`）。无右侧留白时在最后「托盘高 + 24」处让位；有留白（`inset >= 42`）时居中放进留白，不隐藏。两个 26×24 箭头分别 `ScrollTo(0)` / `ScrollTo(MaxFloat32)`；`MotionBase` 同时驱动透明度与 4pt 上浮 |
| `viewEndsButton` | `viewends.go:123` | `viewEnds` 的一半：一个 26×24 的圆角箭头，`toTop` 决定符号与目标端，禁用端用 `--faint`（`.view-ends button:disabled`，`app.css:2262-2263`）；两端的 14 点分隔线由 `viewEnds` 画 |
| `pageInset` | `results.go:41` | **不是表面**，是页面的横向内边距：`max(16, (窗格 − 1200)/2)`（`app.css:609`）。结果页、设置页、标签管理共用；因为第一个参数是 `*ui.Context`，`tools/surfaces.py` 要求在这里登记 |
| `addRow` | `toolbar.go:150` | 常驻。URL 输入 + 正文输入 + Add |
| `toolbar` | `toolbar.go:330` | 常驻。搜索 / Filters / Sort，一行；视图开关 + Graph + Manage tags，一行 |
| `conditionChip` | `chips.go:21` | 每一个生效条件的芯片：名称 + ×。**两处共用**——工具栏搜索行右侧（结果非空时）与空态的按条件列表（结果为空时）。名字永远来自 facet 的 `name`，不是存储值 |
| `filterPopover` | `sidebar.go:416` | `state.FilterPopoverOpen`。面板是 `.pop.proto-menu` 的静止形状：262 宽、11 圆角、4 内边距、`--shadow`（`popoverShadow`，`theme.go`）；入场见 `filterPopoverEntrance` |
| `filterPopoverEntrance` | `motion.go:219` | **不是表面**，是筛选浮层入场（`.pop` 的 `pop-fade` + `pop-in`）的时钟：读 `FilterPopoverOpenedAt` 与帧时间算透明度、上移量与缩放，未走完时 `c.After(1/60)` 要下一帧。因为第一个参数是 `*ui.Context`，`tools/surfaces.py` 要求在这里登记 |
| `batchBar` | **`toolbar.go:203`** | **`len(BatchSelection()) > 0`**。左侧是批量打标签（输入名字或标签下拉 + Apply）与移动到列表；右端四个动作：`选择当前结果` / `选择已加载` / `取消选择` / `删除` |
| `resultsPage` | `results.go:18` | 常驻。包住 `results` 的换页入场：240ms 内从 opacity 0、下移 5pt 到位；减少动态时只保留淡入。导航方法调 `markPageChange` 起动画 |
| `resultsEntrance` | `motion.go:131` | **不是表面**，是入场的时钟：读 `PageChangedAt` 与帧时间，算 opacity 与下移量，未走完时 `c.After(1/60)` 要下一帧。因为第一个参数是 `*ui.Context`，`tools/surfaces.py` 要求在这里登记 |
| `results` | `results.go:53` | 常驻。按 `TagMode`/`GraphMode`/`ViewMode` 分发 |
| `graphPage` | `graphpage.go:48` | `GraphMode`。整页关系图，页头与设置/标签管理同一语法：26 方形返回（`ToggleGraph`，Label/Tooltip「返回书签」）+ `关系图`；下面按「构建中 / 无关联 / 图本身」三态。返回方块是整页唯一的可见出口。图本身装在一张 `.options-tag-graph-shell` 卡片里（radius 16 / padding 14 / gap 12） |
| `graphPageHead` | `graphpage.go:95` | **不是表面**，是 `graphPage` 的页头（返回方块 + 页面名）。因为第一个参数是 `*ui.Context`，`tools/surfaces.py` 要求在这里登记 |
| `graphHead` | `graphpage.go:119` | 图的摘要行：`60 个标签 · 189 条关联`、隐藏数、作者节点数，右侧「显示作者」「取消聚焦」两颗 `IconChip`。`.options-tag-graph-head` 的一行、30 点最小高 |
| `graphFocusPanel` | `graphpage.go:153` | 有焦点标签时的卡片：标签名（13/600）、`N 书签 · M 位作者`、前三位作者、`查看书签`。选中底、26% accent 边、padding 12、gap 6。整卡带 Label「聚焦的标签」，读屏读作一块 |
| `loadingState` | `results.go:123` | `!state.Loaded`。骨架屏六卡；呼吸值来自 `pulse`，减少动态下恒为 1 |
| `emptyState` | `results.go:184` | 已加载且零结果 |
| `stateCard` | `results.go:244` | 空状态与错误态共用 |
| `bookmarkGrid` | `card.go:568` | `ViewMode == grid`。**`ui.GridView` 虚拟化**（规范 §6.12） |
| `bookmarkList` | `card.go:631` | `ViewMode == list`。**`ui.List` 虚拟化**（规范 §6.13） |
| `bookmarkCard` | `card.go:13` | 每条结果。**卡片本身**：圆角 16 + `Clip()`，媒体在 body 之前满宽，body padding 18，操作行在 body 内底部 |
| `cardSelectToggle` | `card.go:398` | `/`.options-bookmark-select-toggle`：绝对定位 18/18、20 方形、选中显示 13px check。**不接点击**（卡片整体已是选择目标，第二个目标会触发两次） |
| `cardActions` | `card.go:461` | `/`.options-card-actions`：栅格是 💬 ♡ ⤴ + 右侧 ↗；列表换成 reply/retweet/like 指标。**左侧一组是装饰性的**，只有右侧那个能点 |
| `cardSuggestionChip` | `card.go:366` | 待确认建议的虚线 chip，结果卡片与详情标签行共用；高度/字号/横向 padding 由调用方给（栅格与列表不同） |
| `pendingSuggestionsBar` | `toolbar.go:551` | `.options-bulk-actions-bar`：待确认标签计数 + `接受全部建议`。**与 `batchBar` 不同**：那个是选中后的批量操作，这个与选择无关（是应用在请你处理） |
| `listsAndViewsSettings` | `listsviews.go:199` | 设置 › **列表与视图**页签里的「列表」与「保存视图」两段（`state.SettingsOpen`）。侧栏只列标签与作者，这两项的入口在这里；用偏好行语法；Unclassified 与 Inbox 各带一句说明，其余列表只有名字；行内图标带对象名 |
| `settingsPage` | `settingsview.go:36` | `state.SettingsOpen`。**整页**：占据结果列，侧栏保留；页头 + 顶部标签 + 可滚动分区。分区内容是 magpie 的 `.list` / `.row.pref` 卡 |
| `settingsPageHeader` | `settingsview.go:78` | 页头一行：图标返回、页名、五个分区标签、右端 `Find a setting` 输入框；查找有字时标签让位、输入框占满（`.set-searching`） |
| `settingsTabs` | `settingsview.go:125` | 顶部五个分区标签：`.warm-tabs`，11/600/.05em 大写、22 高、6 圆角，选中用 --pill 底（`app.css:3632-3635`）；`Wrap()`，1180 点窗里一行放得下 |
| `warmTab`（`settingsTabs` / `tagLibraryTabs` 共用） | `settingsui.go:167` | 一片 `.warm-tabs` 按钮；`mark` 在字后带 6pt 圆点 |
| `settingsFilteredSections` | `settingsview.go:216` | `SettingsQuery` 非空时：命中分区名 + 该分区内命中的行，全不命中显示 `SettingsNoMatch` |
| `settingsCard` | `settingsui.go:60` | 一张 `.list`：--card 底、1px --line、12 圆角，行与行之间 1px --line-2；返回通过查找的行数，供分区决定是否画分组标题 |
| `settingsRow` | `settingsui.go:85` | 一条 `.row.pref`：48 高、内边距 12/12/12/14、间距 14；带 `body` 的行改成「名称行 + 控件」两段，因此高于一行 |
| `settingsRowLine` | `settingsui.go:111` | 行左侧：13/500 名称 + 11.5 --fg-2 说明（换行，不省略），控件在行尾 |
| `settingsGeneralSection` | `settingsview.go:255` | 通用：**语言、主题**两行，各带一句说明与 `.proto.pick` |
| `settingsSyncSection` | `settingsview.go:288` | 同步：启动同步、原始载荷、停止缓冲、首页上限，然后是 X 凭证卡（状态行、浏览器读取、连接测试、Cookie/cURL 粘贴） |
| `settingsAutoTagSection` | `settingsview.go:432` | 自动标签：**只剩一行指路牌**（名字是「管理标签」，避免与标签页同名）；控件在标签管理页，按钮打开 `TagMode` 的自动标签页 |
| `settingsDataSafetySection` | `settingsview.go:449` | 数据安全：DATA 标签 + 一句说明；导出、兼容备份、恢复、重置四行（各带一句说明，危险操作用 `.text.danger`）；卡片下方一行回执（`LastBundleOK` 为真时用成功色，进度与拒绝仍是 muted 的 `settingsNote`） |
| `settingsListsViewsSection` | `listsviews.go:188` | 列表与视图：`listsAndViewsSettings` 的两个卡片，只在 `SettingsSectionListsViews` 页签下画 |
| `settingsSectionLabel` / `settingsNote` | `settingsui.go:135` | `.label`：11px/600/.05em/**大写**/--muted 的分组标题（`app.css:2136`）；`settingsNote` 是它下面的一句说明（`.feat-say`：12px、--muted、在卡外）。设置页与自动标签页共用 |
| `settingsField` / `settingsInput` / `settingsQuiet` | `settingsui.go:458` | 偏好行里的文本框：27 高、7 圆角、0 9 内边距、12.5px；`settingsInput` 是可继续链式设置 placeholder / Bind 的底层字段，`settingsQuiet` 是行尾的只读值（11.5px --muted，不画按钮框） |
| `textAction` / `settingsAction` / `IconChip` | `settingsui.go:271` | 全壳唯一的带字按钮 `.text.action`：27 高、6 圆角、--card 底、12px --fg-2；primary / active / danger / disabled / fill / height 变体。`IconChip` 是带图标的同一按钮 |

| `runProgressBar` | `toolbar.go:19` | `view` 里带进度时（侧栏 hero 下方） |
| `tagLibraryText` | `taglibrary.go:96` | `state.TagMode`。页头之后按 `state.TagView` 二选一：标签页 = 工具面板 + 表格，自动标签页 = `tagAutoTagView`。两页互斥 |
| `tagLibraryHeader` | `taglibrary_header.go:26` | 标签库页头（两页共用），与设置页同一语法：34 高一行的图标返回 + `管理标签` 13.5/600 + `标签 / 自动标签` 两个 `.warm-tabs`，下面一行 --muted 统计 |
| `tagLibraryBack` | `taglibrary_header.go:50` | 页头左端的 26 方形返回：`ToggleTagMode` 退出标签模式；文字「返回书签」作 Label 与 Tooltip，与设置页返回一致 |
| `tagLibraryTabs` | `taglibrary_header.go:84` | 「标签 / 自动标签」两片 `.warm-tabs`（`warmTab`）。`自动标签` 未选中且有待办时带 6pt 圆点（运行中或有待确认建议）。**在页头里**，不占表格一行 |
| `tagAutoTagView` | `autotagview.go:32` | `TagView == TagViewAutoTag`：滚动区里一张运行卡 + 前缀说明的偏好行卡（基础规则 / 标签来源 / 关键词规则 / 请求节奏 / 大模型标签），与设置页同一 `.list` / `.row.pref` 语法 |
| `autoTagRunCard` | `autotagview.go:67` | 运行卡：立即分析（primary）/ 停止、实时进度、上次结果（标题 + 详情 + 行尾时间）、空状态、待确认建议、词表审阅 |
| `autoTagBasicsCard` | `autotagview.go:168` | 基础规则卡：三个触发开关（`.segs` Off/On） |
| `autoTagSourcesCard` | `autotagview.go:200` | 标签来源卡：话题标签、作者历史两个开关 |
| `autoTagKeywordRules` | `autotagview.go:225` | 关键词规则卡：每条一行（关键词 → 标签 + 行尾移除图标），最后一行是编辑器（两个字段 + 添加），空时一行说明 |
| `autoTagPacingCard` | `autotagview.go:274` | 请求节奏卡：每请求书签数 / 并行请求数，行尾 `.set-field` 数字框（72×24，右对齐），限流提示作说明行 |
| `autoTagModelCard` | `autotagview.go:306` | 模型连接卡：开关、服务商 / 模型 / 地址 / 部署名（仅 Azure）/ 每轮上限 / 词表模式、API key（字段 + 保存/清除）、连接测试（行尾动作 + 结果说明）。超过词表上限时卡下再加一句 |
| `tagLibraryToolbar` | `taglibrary.go:181` | 工具面板三行：搜索 + 排序 + 关系图；筛选芯片（窄化时右侧带 `N / M 个标签`）；动作行（创建在左、整理词表/清理孤儿在右）。有选中时第三行换成 `tagLibraryBulkControls`。空库不画工具面板 |
| `tagSortSelect` | `taglibrary.go:343` | 面板第一行的排序 `.proto.pick` |
| `tagLibraryChip` | `taglibrary.go:378` | 五个筛选芯片；`.chip-button-active` 是 accent 16%/22%。`待确认` 那颗带一个计数 |
| `tagLibraryBulkControls` | `taglibrary.go:420` | `TagSelectionCount() > 0` 时的整行：已选数、合并目标（`.proto.pick`）、合并、删除选中，右端「取消选择」 |
| `tagTableFitsAllColumns` | `taglibrary.go:470` | 窄面板（窄到放不下名称列 120pt）时，表头和行都去掉「更新时间」列；两处都问同一个 `*ui.Context`，所以不会各说各话 |
| `tagTableHead` | `taglibrary.go:486` | 表格列名 + 全选复选框。窄面板时去掉「更新时间」列（`tagTableFitsAllColumns`） |
| `tagTableHeadCell` | `taglibrary.go:530` | 一个数字列名，右对齐 |
| `tagTableHeadCellStart` | `taglibrary.go:539` | 一个文本列名（所属主题），左对齐 |
| `tagTableRow` | `taglibrary.go:559` | 每个标签一行：勾选、名称（点击查看书签；重命名时原地变成输入框）、别名/子标签（别名编辑时原地变成输入框 + 保存/取消）、主题（点击原地变成选择器）、两个计数、更新时间、四个行内动作（书签/重命名/别名/删除） |
| `tagTableCount` | `taglibrary.go:788` | 一个数字格 |
| `tagAliasEditor` | `taglibrary.go:767` | `TagEditField == TagEditAliases`：别名行原地变成输入框 + 保存 + 取消 |
| `dangerDialog` | `dialogs.go:34` | 四类危险确认（删标签、批量删标签、删列表、重置数据、清理孤儿）共用的对话框。**按钮顺序固定为「危险动作, 取消」**：MyGo 聚焦最后一个按钮，所以回车落在取消上 |
| `vocabularyReviewPanel` | `review.go:33` | `state.ReviewOpen`（`EntranceProgress` 驱动）。四种身体：请求中 / 请求失败（错误 + `ReviewRetry`）/ 部分失败（错误在计划列表之上，Apply 即重试剩余）/ 计划列表（`ReviewApplying` 时按钮忙碌、未勾选时禁用）|
| `suggestionsPanel` | `panels.go:548` | `state.SuggestionsOpen`（`EntranceProgress` 驱动） |
| `detailOverlay` | `inspector.go:21` | `state.DetailOpen`（`EntranceProgress` 驱动） |
| `inspector` | `inspector.go:70` | 详情的入口：固定 hero + 阅读区（单栏滚动）+ 底部一条。**滚动在阅读区里**（规范 §6.14 要它"自己拥有纵向滚动区域"）；长推文不会把按钮推出屏幕 |
| `inspectorEmpty` | `inspector.go:85` | 没有选中项时的两行。**在 `Scroll` 之外**——它太短，给它滚动区没有意义 |
| `inspectorBody` | `inspector.go:99` | 详情的骨架：`detailHeader`（作者 / 发布时间 / 标签 / 三个图标）+ 分隔线 + `detailPost` 的滚动区。没有右栏，也没有底栏 |
| `detailPost` | `detailpost.go:28` | 详情阅读列。**整条推文是一个对象**：正文 → 14pt → 媒体 → alt → 计数，没有「内容摘要」「媒体资源」两个小标题，也没有摘要卡片 |
| `detailMedia` | `detailpost.go:57` | 有媒体时。1 张满宽；2–4 张两列网格；奇数最后一个保持半宽 |
| `detailMediaItem` | `detailpost.go:89` | 每一张图：按图片自身比例给高度（比例夹在 0.8–3.2 之间），点击打开灯箱的**对应序号**；`IsVideoMediaType`（含 `animated_gif`）盖一个播放标记 |
| `detailPlayMark` | `detailpost.go:142` | 视频海报中央的 58pt 圆盘 + play glyph。是标记不是控件——真正的播放器在灯箱里 |
| `detailMetrics` | `detailpost.go:160` | 有 `row.Metrics` 时：回复 / 转发 / 喜欢三个计数，与卡片列表模式同一套图标 |
| `renderLightbox` | `lightbox.go:203` | `s.LightboxOpen()`。外壳 `min(94vw,1320)` + 三段（工具条 / 舞台 / 计数），舞台是 `72 / 1fr / 72` 三轨；不画类型与 URL 两行 |
| `detailHeader` | `inspector.go:117` | 卡片唯一的顶栏，一条 `ui.Row`：30pt 头像 / 名字 + handle + 发布时间 + 标签（同一个可换行行）/ 三个图标（复制链接、在 X 打开、关闭） |
| `detailTags` | `inspector.go:173` | 顶栏作者行里的尾巴：标签药丸 + 待确认建议芯片 + 新标签输入框，紧跟发布时间。没有标题，也没有"还没有标签"的句子 |
| `detailActions` | `inspector.go:222` | 三个 28pt 图标按钮：复制链接（就地变对勾）、在 X 打开、关闭（`AutoFocus()`，对话框的初始焦点） |
| `lightboxPlay` | `lightbox.go:286` | 灯箱里的**视频或 GIF**海报上，64pt 圆盘；标签按类型取 `MediaPlay` / `MediaPlayGif`。点击 `PlayVideo` 起原生播放器；播放中不画（播放器盖着它） |
| `lightboxClose` | `lightbox.go:310` | `.options-media-lightbox-close`：38 圆形、`color-mix(panel-bg 78%, rgba(25,21,20,0.46))` 底、白 glyph、静止 82% 不透明。**不是 `IconChip`**：那个带可见文字 |
| `lightboxNav` | `lightbox.go:329` | `.options-media-lightbox-nav`：72 轨道里的 46 圆形，glyph 是文字 ‹ ›（图标表里没有 chevron）。**只在多项时画**，单项时画同宽占位 |
| `mediaLightboxImage` | `mediaview.go:266` | `.options-media-lightbox-image`：满宽、`min(76vh,800)` 高、Contain、radius 18，带自己的 160ms 入场。**视频取 poster 不取文件** |

## 确认对话框（都在被问到时才出现）

| 方法 | 文件:行 | 何时出现 |
|---|---|---|
| `handleConfirmations` | `dialogs.go:142` | 分发器：`deleteBookmarks` 的确认答复 |
| `handleTagDeleteConfirmations` | `dialogs.go:40` | 删除标签 / 合并标签的确认 |
| `handleListDeleteConfirmation` | `dialogs.go:122` | 删除列表的确认 |
| `handleCleanupConfirmation` | `dialogs.go:176` | 清理孤儿标签的确认 |
| `handleResetConfirmation` | `dialogs.go:99` | 重置的确认 |

## 窗口标题栏

| 方法 | 文件:行 | 何时出现 |
|---|---|---|
| `titleBar` | `titlebar.go:27` | 常驻，窗口第一行。自己画的 50pt 标题栏：系统标题栏隐藏、红绿灯保留；左段是侧栏自己的顶带（同样的底色 + 侧栏边缘延伸到窗口顶端），右段是结果上方的头部。两段都能拖动窗口，双击缩放。开关的图形画在 `drawSidebarGlyph`：16 格上的圆角框 12.5×10.5 r2.25 stroke1.25，分隔线 6.75，左窗格 4×9 r1、展开时 32%（`app.css:525`） |
| `sidebarToggle` | `titlebar.go:108` | 常驻，标题栏里。侧栏开关：magpie 的方框 + 分隔线 + 左侧填充（展开时 32%），**两种布局都在窗口同一个 x**——展开时在左段，折叠时搬进右段 |
| `brandLockup` | `titlebar.go:150` | 常驻，紧接开关。标记 + 13.5/600 名称；宽度不足时只留标记（magpie 的 `fitTop` tight 态） |
| `pageCrumb` | `titlebar.go:168` | 常驻，品牌之后。当前页名 13.5/600：设置 / 管理标签 / 标签图 / 作用域名。**页名只在这里**（magpie 的 `.crumb`，app.css:522） |

## 侧栏

| 方法 | 文件:行 | 何时出现 |
|---|---|---|
| `sidebar` | `sidebar.go:48` | `SidebarCollapsed` 为假时。三段式：hero / 页签中段 / 页脚三控件。折叠是**隐藏**而不是图标轨，没有 rail |
| `bookmarkContextMenu` | `contextmenus.go:31` | 每张结果卡右键：打开、在 X 中打开、复制正文、复制链接、加入/移出选择 |
| `authorContextMenu` | `contextmenus.go:129` | 侧栏作者行右键：按此作者筛选、只看此作者、复制 @handle |
| `sidebarHero` | `sidebar.go:92` | **只在有运行时内容时画**：同步进行中（进度 + 取消）、同步失败（原因 + 重连）、可续传（句子 + 立即同步）、自动标签运行中。没有这些时整块和其下分隔线都不画 |
| `sidebarFooter` | `sidebar.go:232` | 常驻。`.side-foot`：状态行（7 点圆点 + 末次同步时间，11.5）+ 一排 26 图标（语言 / 主题 / 同步 / 设置），`8 10 10` 内边距、1 间距（`app.css:397`、`:513-517`）。导出/恢复/重置在设置的数据安全段与文件菜单里 |
| `sidebarFacetTabs` | `sidebartabs.go:50` | 常驻。分段控件的两片 + 当前页签自己的控件（标签页签上是新建与标签库） |
| `sidebarFacetTab` | `sidebartabs.go:123` | 分段控件的一片：名称 + 当前条件下的行数。**非当前页签有筛选时画一个小点**——一个槽位里显示两个维度的唯一办法。`Focusable`，Tab 可达、Enter/Space 切换 |
| `sidebarSearchToggle` | `sidebartabs.go:196` | 页签行里的放大镜：开/关搜索框。**两个页签共用同一个入口**，打开时底色是 accent 10% |
| `sidebarSearchField` | `sidebartabs.go:271` | `SidebarSearchOpen` 为真（或该页签的查询非空）时，页签行与列表之间的搜索框。`AutoFocus` 只在出现那一帧取焦 |
| `sidebarTagsBody` | `sidebartabs.go:307` | 标签页签：新建行 / 标签行，列表自己滚动。**搜索框默认不画**；**不画导航行** |
| `sidebarAuthorsBody` | `sidebartabs.go:386` | 作者页签：全部作者行，列表自己滚动。**不分页、不设上限**；搜索框同样默认不画 |
| `authorRow` | `sidebartabs.go:428` | 作者页签里的每一行：30 高（两行）、头像、名字 12.5/500 在 11 号 handle 上、条件生效下的 11 号计数。**行本身任何状态都不画底色**：选中是列表那一个滑块（`navHighlight`，`navhighlight.go`），行只在悬停/选中时把名字提到 `--fg`。**顺序永远来自服务，选中不会把行提到顶部** |
| `tagRow` | `tags.go:702` | 标签页签的每一行，也是标签库的行来源：26 高、`0 10`、6 圆角、6 间距、12.5/500 名称、11 号计数（`app.css:440-449`）。**行本身不画底色**；选中同样由列表的滑块标记（多个标签同时生效时标第一个），行只把名称提到 `--fg`（悬停 200ms 过渡） |

## ⚠️ 哪些表面能用截图验，哪些不能

**`TROVE_CAPTURE` 拍不到任何靠 `EntranceProgress` 决定可见性的表面。**

**原因**：面板的透明度来自 `root.Animate(key, target, duration)`，而那个值取自 `e.c.now` —— **一个由帧推进的时钟。`CapturePage` 只取一张快照，不泵帧，所以时钟停住，面板停在起始值 0，画出来不可见。**

**所以边界是**：

| 验证手段 | 能 | 不能 |
|---|---|---|
| **`TROVE_CAPTURE` 截图** | 拍动画之外的表面（结果区、侧栏、工具栏、中文界面） | **拍下面这四个** |
| **`ui.Tester`** | 读任何表面的文本内容 | **表达"可见"这个属性** —— 它收集文本时不看透明度，所以标题在 0 和 1 上读起来一样 |

**四个拍不到的表面**（由 `shell.go` 的 `EntranceProgress` 调用决定）：

| 表面 | 状态字段 | 内容由哪条测试验 |
|---|---|---|
| `detailOverlay` | `DetailOpen` | `TestTheDetailHeroDrawsItsParts` |
| `vocabularyReviewPanel` | `ReviewOpen` | `TestTheVocabularyReviewPanelDrawsItsText` |
| `suggestionsPanel` | `SuggestionsOpen` | `TestTheSuggestionsPanelDrawsItsTitle` |

**这一节由 `tools/surfaces.py` 核对三件事**：

1. **`shell.go` 里每个 `EntranceProgress` 的状态字段都在本节里**
2. **本节里每个字段都在 `shell.go` 里**
3. **每一行的第三列都点名了一条测试，而那条测试真的存在** —— **"未验"不是允许的取值**，**因为一个没人验过的表面与一个不存在的表面在验证上是同一件事**

## ⚠️ 这张表不覆盖什么

- **只列 `func (s *Shell) ...(c *ui.Context, ...)` 形态的绘制方法。** 内联在 `Children(func(){...})` 里的表面（比如卡片内部的作者块、标签行）不在表里。
- **不列 `ui` 包里的共用控件**（`IconChip`、`IconToggle`、`IconOnly`）—— 它们是零件，不是表面。
- **不列"何时出现"之外的语义** —— 为什么这样设计要读代码，这张表只回答"有没有"。
