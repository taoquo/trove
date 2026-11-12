# 板

唯一任务板。**现在 = `Status` 为 `doing` 的那一行（至多一行）。** 规则见 `apps/macos/AGENTS.md`；
`tools/statuscheck.py` 在 gate 里检查这张表：状态值、Profile、DoR、验收命令、证据、过期。

## 切片

| ID | 链路 | 切片 | Profile | Status | DoR | 验收命令 | 证据 | Updated |
|---|---|---|---|---|---|---|---|---|
| F6-1 | 设置 | 页面内边距 8/8 | visual | done | ✅ | `go test ./internal/ui -run TestTheSettingsPageKeepsMagpiesPagePadding` | commit body；`docs/screenshots/F6-1-settings.png` | 2026-11-09 |
| F5-1 | 自动打标 | 取消与恢复 | behavior | done | ✅ | `go test ./internal/ui -run TestAutoTag; go test ./internal/usecase/tagging -run TestHeal` | 红：旧实现 heal 记为 interrupted、外层 Stop 取消上下文；绿：本行命令 + gate（见 commit body） | 2026-11-09 |
| F1-1 | 连接与同步 | 未配置凭据 | behavior | todo | — | `go test ./internal/ui -run TestSyncNeedsCredentials` | — | 2026-11-09 |
| F3-1 | 详情 | 媒体缺失 | visual | todo | — | `go test ./internal/ui -run TestDetailMissingMedia` | — | 2026-11-09 |
| F2-1 | 浏览与筛选 | 无匹配 | visual | todo | — | `go test ./internal/ui -run TestFilterNoMatch` | — | 2026-11-09 |
| F4-1 | 浏览与筛选 | 列表范围：标题正确 + 可回到全部 | behavior | todo | — | `go test ./internal/ui -run TestAListScopeNamesItselfAndCanBeLeft` | — | 2026-11-09 |
| F7-1 | 连接与同步 | 删除要留住：本地删掉的书签不被下次同步带回 | data | done | ✅ | `go test ./internal/usecase/syncer -run TestALocallyDeletedBookmarkStaysDeleted` | 红：`bookmark 2 was re-imported by the sync after the user deleted it`；绿：本行命令 + `importexport` 两条墓碑测试 + gate 14/14 | 2026-11-09 |

## Inbox

新观察进这里，不直接开工；P0 例外。每条要有日期和一句可复现的话。

- 2026-11-12 — 第 34 轮把三个文件命令的面板移到后台工作里，测试只能证明“处理器先返回、
  面板调用在工作里”，弹不弹得出 NSSavePanel 需要一个真人在应用里点一次“导出数据”确认。
  若仍无面板，下一步是在 `runPanel` 的 sheet 分支加日志（`parent.IsVisible()` 为假时会退回
  应用模态）。`bundle.go:58-110`、`mygo internal/darwin/dialog.go:31-45`。
- 2026-11-12 — 抓图的 `XBM_CAPTURE_LOCALE` 只认 `zh-CN`；写 `zh` 会被 `i18n.ValidLocale`
  判为无效并**静默**回落英文，第 35 轮第一次抓出的“中文”图其实是英文。要不要让
  `ResolveLocale` 按语言前缀归一（系统报 `zh-Hans-CN` 或 `zh` 时也能落到中文）？这是产品
  级改动，先记着。`i18n.go:34-52`。

- 2026-11-09 — API Key 已存时，聚焦输入框会暂时变空白（占位符消失）。要不要改成固定掩码？待定。
- 2026-11-09 — 设置页底部从 24 收到 8 后，长列表末尾是否太贴边？F6-1 人审时回答。
- 2026-11-09 — Magpie 的 HEAD 已到 `8fa3c60`，参考仍钉在 `dca338f`。下次产品环再决定是否重钉。
- 2026-11-09 — 取消改为协作式后，若 provider 请求本身挂死，再按停止不会升级为硬取消，只能等
  传输层超时。要不要"第二次按下 = 硬停"？待定。
- 2026-11-09 — 从设置里的列表打开一个列表后没有回到“全部书签”的入口，标题栏仍写“全部书签”。
  已建切片 F4-1；`results.go:397-405`、`listsviews.go:157-166`。
- 2026-11-09 — 动态 GIF 没有播放入口也永远不动：两处只比较 `MediaVideo`，没用
  `domain.IsVideoMediaType`（它包含 `animated_gif`）。`detailpost.go:108`、`lightbox.go:263`。
  2026-11-12 已修：第 27 轮两处改用 `IsVideoMediaType`，按钮按类型命名（R32/R33）。
- 2026-11-09 — 词表审阅“应用所选”先关面板再干活：没有忙碌态，完成提示只画在结果页工具栏，
  标签模式下永远看不到。`tags.go:229`、`work.go:95`、`taglibrary.go:113-115`。
  2026-11-12 已修：第 29 轮（R37/R38/R39）。
- 2026-11-09 — 计数的三个图标无可读名称；多图帖只画第一张的替代文本。`detailpost.go:174`、`:46`。
  2026-11-11 修正：源把这三个计数整块标了 `aria-hidden="true"`（`BookmarkCard.tsx:230-243`），
  它们本来就不该被朗读；真正的缺口是 MyGo 没有“对辅助技术隐藏”的开关，名字加不加都不对。
  替代文本那条仍在：源里图片是 `alt=""`，我们是自己加的，多图只画第一张。
- 2026-11-09 — 词表审阅失败后只有“关闭”，没有“重试”，重来要关掉再从工具栏打开。`review.go:28-37`。
  2026-11-12 已修：第 29 轮（R40）。
- 2026-11-09 — 标签库的“孤儿标签 / 长尾标签 / 未归主题”是行话，逐字来自扩展 `copy.ts:102-152`；
  要改是两边一起改的产品决策。`i18n.go:1166-1177`。
- 2026-11-09 — 设置的“自动标签”页签只剩一张指路卡，整页留白（S26 的既定决策）。点进去
  看不到任何自动标签设置，是否干脆去掉这个页签？`settingsview.go:403-422`。
- 2026-11-10 — 骨架屏是 220px auto-fit 四列，结果网格是两列 masonry，读数落地时会重排；
  骨架注释却写“不会跳”。数值来自扩展，属源里已有的缺陷。`results.go:227-248`、`card.go:601-626`。
- 2026-11-10 — “清空屏蔽名单”放在设置·数据安全，扩展在自动标签页。第 18 轮补了该行的说明
  与清空回执，位置仍是产品决策：`settingsview.go:478-501`、`SettingsPanel.tsx:912-922`。
- 2026-11-10 — “上次分析结果”是一整段无层级的句子，且把“本次新增”与“当前待确认”混在一句。
  文案在扩展 `copy.ts:225` 也有，属两边一起改的产品决策。`errors.go:106-122`。
- 2026-11-10 — 结果页待确认横幅用全库数并接受全部；扩展只算当前可见并只接受可见的。
  `toolbar.go:534-581`、`OptionsApp.tsx:984`、`BookmarkResultsPane.tsx:215-234`。
- 2026-11-10 — 真实体量走查（第 11 轮的教训 L11）：每轮打开应用前先灌一份 76 标签 / 9.5k
  书签的库，在 212 点侧栏、2000 条结果下逐页看一遍，再拍图。现在所有 capture fixture 都是
  小样本，量级缺陷拍不出来。要不要做一个 `XBM_CAPTURE_SCALE` 生成器？待定。
- 2026-11-11 — 搜索框占位符承诺“搜索书签、作者和备注…”，但两边都没有备注字段，搜索也不搜
  备注（`copy.ts:39/353`、`searchBookmarks.ts:34-46`、`bookmark_repo.go:414-423`）。要改是
  两边一起改的产品决策：去掉这个词，或真的加一个备注字段。
- 2026-11-11 — 详情的新标签输入框里，英文占位符 `Enter a new tag name` 被截成 `Enter a new
  tag na`（中文 `输入新标签名称` 正好放得下）。源的上限同样是 116 点
  （`extension.css:3428` `clamp(82px, 10vw, 116px)`），placeholder 同样无省略号截断——要改是
  两边一起改的产品决策：缩短英文文案，还是让输入框按内容变宽。`inspector.go:211-215`。
- 2026-11-11 — 格式串与参数不匹配没有检查：第 13 轮的 `%!(EXTRA int=18)` 是唯一一处把无动词
  文案交给 `Tf` 的调用点，而 `check-text.sh` 只查“文案是否都走翻译”。要不要扫 `Tf` 调用点，
  比对文案里的 verb 数与参数数？待定。
- 2026-11-12 — 批量条的“选择全部结果”一次选中全部匹配（9,562 条，`MatchingIDs`），源把它拆成
  “选择已加载”（当前 80 条）与“选择当前结果”两个按钮（`BookmarkResultsPane.tsx:45-52`、
  `copy.ts:58-59/372-373`）；原生只有后者且叫“全部结果”。要不要补“选择已加载”，或把默认
  一次选中的量收紧？待定。
  2026-11-12 已修：第 33 轮按源补齐两个按钮并改名（R48）；超限时仍整体拒绝，不选部分集合。
- 2026-11-12 — 批量条的标签图标按钮与输入框的 Enter，在输入为空时什么都不做也不说
  （`AddTagsToSelection` 遇空输入直接返回）。禁用还是给一句话？待定。`selection.go:127-136`。
- 2026-11-12 — 保存视图行只写名字，不写它恢复了什么（搜索词 / 标签 / 作者 / 媒体）。源也只有
  名字；要不要给每个视图一行副标题？待定。`listsviews.go:307-330`。
- 2026-11-12 — 一次性回执（`TagActionMsg`、`LastBundleReport` 等）拍不到：`ApplyCaptureState`
  先跑，`ApplyCaptureLocale` 随后按设计清空它们，截图里只剩空行。第 28 轮写过
  `tags-rename-taken` 又删掉，改为测试断言。第 30 轮给同步回执接了这样一个“语言之后”的钩子
  （`applyCaptureState` 末尾调 `ApplyCaptureReceipt`，`capture.go:146-153`）；其余一次性回执
  要不要同样处理，仍待定。`preferences.go:115-133`。
- 2026-11-12 — 合并选择器每帧重建全库标签名；源在超过阈值时惰性生成选项
  （`TagResultsPane.tsx:410-419`）。76 标签无感，500 标签时再看。`taglibrary.go:431-447`。
- 2026-11-12 — 引擎/模型错误在中文面板里是英文原文（超时、未配置、鉴权），词表审阅的失败行
  也是。源同样不译；要不要按类别译成句子，是两边一起改的产品决策。`review.go:45-66`。
- 2026-11-12 — 导入 / 恢复的回执同样是一次性句子，抓图钩子目前只接了同步与重置（第 30、31
  轮）。要不要把导入也接上，或在 Inbox 里继续用测试断言？`capture.go:147-157`。
- 2026-11-12 — 自动标签进度条的宽度是跳变的：源有 `transition: width 200ms ease-out`
  （`extension.css:4658`，令牌 `MotionWidthMs`），要逐帧驱动才能断言中间值；等有动画测试夹具
  再做。`toolbar.go:66-90`。


## Watch

已知、未排期、不要反复重新讨论的项。

- View Transition、`.navlamp` 指针灯、原生菜单宽圆角：平台能力，不做（audit §12）。
- 顶栏页签条：本应用没有页面页签可搬（audit §12）。
- freshness 门禁：未建；回放 R4 待覆盖。

## Decisions

- 2026-11-09 — 状态入库：`.gitignore` 用 `docs/*` + `docs/loop/` 白名单。理由：不入库的板子换机即失忆。
- 2026-11-09 — 两条车道：slice 走完整 DoR/DoD，fix 只要求红测试 + gate + 三行证据。
- 2026-11-09 — 第一片选视觉 F6-1，理由：当前主目标是 Magpie 视觉，验收 profile 清楚。
- 2026-11-09 — 签收节奏：每片一次 10 分钟；批量巡检留到周环。
- 2026-11-09 — 真实运行边界：LLM / Keychain 每个碰到的切片真跑一次；X 同步允许 fixture + 周级真跑。
- 2026-11-09 — 暂不装 commit 钩子；三片后看证据缺失率再决定。
- 2026-11-09 — 参考钉 `dca338f`；实现期只读本地 clone，不读上游。

## Replay

每出现一个逃逸缺陷，加一行；回放必须能在坏版本上变红。

| ID | 缺陷 | 现在的检查 | 状态 |
|---|---|---|---|
| R1 | 模型阶段的分母是书签库大小，进度永远填不满 | `TestTheModelPhaseMeasuresItsOwnWorkload` | 已覆盖 |
| R2 | 已存 Key 重新打开页面读成"尚未存密钥" | `TestAStoredKeyStillReadsAsStoredAfterReopening` | 已覆盖 |
| R3 | 数字输入框像"只能设 25"（光标在末尾，输入被拼在前面） | `TestTypingIntoANumberFieldReplacesTheValue` | 已覆盖 |
| R4 | 构建后应用没更新，验证的是旧二进制 | freshness（待建） | 待覆盖 |
| R5 | 侧栏标签名过长时换行，计数被挤到第一行、行高翻倍 | `TestATagNameThatDoesNotFitEndsInOneLine` | 已覆盖 |
| R6 | 本地删除的书签被下一次同步重新导入 | `TestALocallyDeletedBookmarkStaysDeleted` | 已覆盖 |
| R7 | 已有结果时刷新失败完全静默，页面看起来是新的 | `TestAFailedRefreshKeepsTheRowsAndSaysSo` | 已覆盖 |
| R8 | 批量接受建议后，横幅与标签页计数停在旧数字 | `TestAcceptingTheWholeBatchRefreshesWhatTheFeedCounts` | 已覆盖 |
| R9 | 关系图整页没有可见的返回入口，只有 Escape | `TestTheGraphPageHasAWayBackToTheBookmarks` | 已覆盖 |
| R10 | 侧栏标签行右侧空 56 点：保留了两个空的 22 点操作位 | `TestATagRowsCountReachesTheRowsEdge` | 已覆盖 |
| R11 | 搜索 / 加号 / 管理画在页签下面另起一行，低 38 点 | `TestTheFacetControlsSitBesideTheirTabs` | 已覆盖 |
| R12 | 关系图在真实体量下标题叠成一片灰，且不可交互 | `TestTheGraphFindsRoomForItsCaptions`、`TestClickingAGraphNodeFiltersByThatTag`、`TestTheGraphUsesTheWholeCanvas`、`TestAFocusedGraphNamesOnlyWhatItLights`、`TestTheFocusPanelHasRoom` | 已覆盖 |
| R13 | 指针一进侧栏就浮出滚动条，压在计数上 | `TestTheSidebarListDrawsNoScrollbar`、`TestTheSidebarListStillScrolls` | 已覆盖 |
| R14 | 打开作者层，头部画成 `位作者节点 %!(EXTRA int=18)`，环没有名字 | `TestTheAuthorLayerIsCountedInWords`、`TestAnAuthorRingCarriesItsHandle` | 已覆盖 |
| R15 | 筛到一条结果时，那张卡片占满整行（1057 点 vs 468 点） | `TestOneResultKeepsItsColumn` | 已覆盖 |
| R16 | 详情标签行里六颗“移除”同名，朗读时分不清属于哪个标签 | `TestEachTagPillNamesTheTagItRemoves` | 已覆盖 |
| R17 | “添加书签”行只能从菜单栏再执行一次命令才收得起来 | `TestTheAddRowHasAWayToCloseItself` | 已覆盖 |
| R18 | 侧栏标签搜索只认名称、不认别名，且无命中时整列空白 | `TestTheSidebarTagSearchFindsAliases`、`TestTheSidebarSaysWhenASearchMatchesNoTag` | 已覆盖 |
| R19 | 设置·同步把“X 凭证”写了两遍；屏蔽名单行无说明、清空无回执 | `TestTheSyncSectionNamesTheCredentialCardOnce`、`TestTheBlockedTagRowSaysWhatItIsAndConfirmsTheClear` | 已覆盖 |
| R20 | 换服务商后旧端点随行，密钥会被发到上一家的地址 | `TestSwitchingProviderDoesNotKeepTheOldEndpoint` | 已覆盖 |
| R21 | 同步中侧栏把“同步中”写两遍；同步失败只说“请重试”而无按钮 | `TestTheSyncingSidebarSaysItOnce`、`TestAFailedSyncOffersTheWayToTryAgain` | 已覆盖 |
| R22 | 网格/列表切换与筛选 pill 有焦点位却没有名字 | `TestTheViewTogglesTellTheKeyboardWhichIsWhich`、`TestThePendingFilterChipNamesItsCount` | 已覆盖 |
| R23 | 从侧栏底部切语言后，设置页的主题选择器仍显示旧语言 | `TestTheSidebarLanguageChipRelabelsTheSettingsPage` | 已覆盖 |
| R24 | `tags-editor` 截图从未拍到编辑器：焦点落在屏幕外的那一行 | `TestTheTagEditorCaptureOpensOnTheRowsAtTheTop` | 已覆盖 |
| R25 | 条件 chip 的 x 用条件本身命名，读屏在同一 chip 上听到两个同名元素 | `TestEachActiveFilterChipNamesWhatItRemoves` | 已覆盖 |
| R26 | 同一个“只含媒体”条件：弹层叫“包含媒体”，chip 叫“仅含媒体” | `TestTheMediaChipUsesTheSameWordAsTheFilterItRepresents` | 已覆盖 |
| R27 | 批量打标 / 移动完成后没有回执、选择不清，成功的写入看起来像没按到 | `TestABatchActionSaysWhatItDid`、`TestAMoveToListNamesTheDestination`、`TestTheTagPickerSaysWhatItDid` | 已覆盖 |
| R28 | 批量条的两个选择器静止时只有一个箭头；未选时“移动到列表”静默移进第一个列表 | `TestTheBatchBarSaysWhatEachPickerChooses`、`TestAPressOnTheUnchosenPickerDoesNothing` | 已覆盖 |
| R29 | 空名字保存视图写死英文 “Saved view”，中文页面出现英文行 | `TestTheDefaultSavedViewNameSpeaksTheUiLanguage` | 已覆盖 |
| R30 | 应用保存视图后设置页不关，按下去像没反应 | `TestApplyingASavedViewTakesYouToTheResults` | 已覆盖 |
| R31 | 列表与保存视图埋在数据安全页末尾；两卡片的行内图标共用一个通用名 | `TestTheListsAndViewsHaveTheirOwnSettingsTab`、`TestEachRowActionNamesItsOwnRow` | 已覆盖 |
| R32 | 动态 GIF 被当照片：详情没有播放徽标，预览没有播放按钮 | `TestAGifOffersPlayInThePreview` | 已覆盖 |
| R33 | 预览层是模态的却没有名称；GIF 的播放按钮叫“播放视频” | `TestThePreviewNamesItself`、`TestAGifOffersPlayInThePreview` | 已覆盖 |
| R34 | 勾选标签即静默选定合并目标，一按合并就折进排序第一的标签；且没有确认 | `TestMergeNeedsATargetTheReaderChose`、`TestMergeAsksBeforeItFolds`、`TestTheConfirmationNamesTheSurvivor` | 已覆盖 |
| R35 | 只勾目标本身仍执行合并，回执“已合并 0 个标签” | `TestMergingOnlyTheTargetSaysThereIsNothingToMerge` | 已覆盖 |
| R36 | 重命名撞名时中文界面显示 `workspace: cannot rename ...` | `TestARefusedRenameSaysWhyInTheUiLanguage` | 已覆盖 |
| R37 | 一个计划失败即中止其余计划，面板同时关掉，剩余提议无处重试 | `TestOneFailedPlanDoesNotDiscardTheRest` | 已覆盖 |
| R38 | 审阅面板在写入前关闭、无忙碌态；空勾选也能按 Apply 并回“已应用 0 项” | `TestThePanelStaysUpWhileItApplies`、`TestApplyWaitsForASelection` | 已覆盖 |
| R39 | 完成回执只画在结果页工具栏，从标签页发起的审阅永远看不到 | `TestTheTagPageSaysTheReviewLanded` | 已覆盖 |
| R40 | 请求失败只有“关闭建议”，模态盖住工具栏，重来要关掉再找 | `TestAFailedReviewOffersAnotherTry` | 已覆盖 |
| R41 | 成功的同步结束只清空进度、更新一个 11px 时间戳；存了几条无人告知 | `TestASuccessfulSyncSaysWhatItStored` | 已覆盖 |
| R42 | 续传句说“再次同步即可继续”，下面没有任何可按的控件 | `TestTheResumeLineOffersTheSyncItAsksFor` | 已覆盖 |
| R43 | 被取消的同步在状态行 tooltip 里写“空闲”，同一次运行有两个说法 | `TestACancelledSyncSaysCancelled` | 已覆盖 |
| R44 | 重置清空整个库之后没有任何回执，空库与加载失败无法区分 | `TestResetSaysTheLibraryIsGone` | 已覆盖 |
| R45 | 重置删掉了设置与 Keychain 账号，窗口仍显示“cookies 已保存”“启动时同步” | `TestResetForgetsTheSettingsItDeleted` | 已覆盖 |
| R46 | 结果、进度与拒绝共用一行 --muted 文字，完成的动作读起来像提示 | `TestResetSaysTheLibraryIsGone`（`LastBundleOK`） | 已覆盖 |
| R47 | 减少动态下骨架屏与同步圆点停在 55% 透明度，注释却承诺“稳定的全亮” | `TestAReducedMotionPulseStaysFullyVisible` | 已覆盖 |
| R48 | 9.5k 条时批量条唯一的选择按钮永远拒绝，没有“选择已加载”这条源里的出路 | `TestARealScaleLibraryCanStillBeBulkSelected` | 已覆盖 |
| R49 | 导出/导入的文件面板开在点击处理器里，处理器不返回，点击看起来毫无响应 | `TestClickingExportAsksForAPathOffTheWindowThread` | 已覆盖（面板弹出待真人实测） |
| R50 | 条件 chip 只有 16 点的 × 能按，chip 上的字与整块悬停反馈都在承诺可点 | `TestTheConditionChipIsOneButton` | 已覆盖 |
| R51 | 所有纯图标控件只有 tooltip、没有可访问名称；清除按钮因此像不存在 | `TestTheToolbarAnswersEveryClick`（清除一步） | 已覆盖 |
| R52 | 每张卡片的选择框同名 “Select bookmark”，一列 2000 个名字区分不了任何一条 | `TestTheCardAnswersEveryClick` | 已覆盖 |
| R53 | 详情右上角的关闭控件手写的，只设了 tooltip；覆盖层唯一一条指针退路没有可访问名称 | `TestTheDetailAnswersEveryClick` | 已覆盖 |
| R54 | 批量条的输入框无占位符无名称；两个选择器与旁边按钮同名（“Add tag” ×2、“Move to list” ×2） | `TestTheBatchBarAnswersEveryClick` | 已覆盖 |
| R55 | 标签表格每行的四个动词、选择框、主题格都不带标签名；新建行输入框无名 | `TestTheTagLibraryAnswersEveryClick` | 已覆盖 |
| R56 | 设置页查找框按状态在两个绘制点之间切换，打一个字就失焦，之后输入全部丢失；返回控件只有 tooltip | `TestTheSettingsPageAnswersEveryClick` | 已覆盖 |
| R57 | 关键词规则每行的删除都叫「Remove」、建议面板每行的接受/拒绝都不带标签名；规则编辑器的行名盖住同名按钮 | `TestTheAutoTagPageAnswersEveryClick`、`TestTheRunAndItsSuggestionsAnswerEveryClick` | 已覆盖 |
| R58 | 工具栏搜索框在 magpie 改版后只剩占位符、没有可访问名称，而它是 Cmd-F 的落点 | `TestTheMenuFocusSearchLandsInTheToolbar` | 已覆盖 |

## Metrics

每片收尾加一行；`gate` 单位是秒。

| 切片 | 计划 | 实际 | 返工轮次 | 逃逸 | 用户插话 | gate | 日期 |
|---|---|---|---|---|---|---|---|
| F6-1 | 0.5 天 | 0.5 天 | 0 | 0 | 0 | 275 | 2026-11-09 |
| F5-1 | 0.5 天 | 0.5 天 | 0 | 0 | 0 | 267 | 2026-11-09 |

### 基线（2026-11-09，loop 之前）

- 最近 20 个 commit：7 refactor / 6 feat / 5 fix / 2 test；11 月 1 日以来 57 个。
- 被反复改的文件（最近 20 个 commit）：`metrics.go` 9 次、`i18n.go` 7 次、`toolbar.go` 6 次、
  `spec_test.go` 6 次。
- gate：14/14，275 秒（2026-11-09，含新增的 statuscheck 步骤）。
- 目的：三片之后用同一口径对比；口径不变才算数。
