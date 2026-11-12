# 教训

一条规则 = 一句话 + 为什么 + 证据 + seen 日期 + 它对应的检查。**同一规则在 3 个不同日期出现，
升级成检查（测试或 gate），然后这里只留一行指针。** 最多 20 条活跃；30 天没人引用的删掉。

检查存在时，规则的价值已经转移到检查里；这里只解释"为什么有这个检查"。

## 活跃

### L1 状态位只在写入路径更新，读取路径就会忘记它

- 为什么：`APIKeyStored` 只在保存/清除时写，页面重新加载时从不问 Keychain，于是"存了却显示没存"。
- 证据：`TestAStoredKeyStillReadsAsStoredAfterReopening`、`TestSwitchingProvidersForgetsTheOtherKeysPresence`。
- Seen：2026-11-09。
- Check：两个测试 + 页面加载时 `refreshAPIKeyState`。

### L2 输入框的光标位置会吃掉用户输入

- 为什么：MyGo 点击后光标在末尾；在 `25` 后输入 `50` 得到 `2550`，范围检查拒绝，失焦还原。
  用户看到的是"这个数改不了"。
- 证据：`TestTypingIntoANumberFieldReplacesTheValue`。
- Seen：2026-11-09。
- Check：`settingsNumber` 获得焦点时清空绑定文本，越界夹取。

### L3 进度的分母必须是本轮工作量，不是库的大小

- 为什么：模型阶段曾拿整库做分母，进度条永远填不满，ETA 和预算秒数全部失真。
- 证据：`TestTheModelPhaseMeasuresItsOwnWorkload`。
- Seen：2026-11-09。
- Check：`beginLLMPhase` 发布 `min(候选, 单次上限)`；启发式阶段收在 `Total/Total`。

### L4 文案里的范围必须来自引擎常量

- 为什么：帮助文字写过"0 = 一次一个"和"0 = 不限制"，而引擎把 0 归一成 3 和 200；
  UI 的范围也曾是 1..100 / 0..100000，引擎是 1..50 / 1..2000。
- 证据：`docs/loop/board.md` R1–R3 同一轮的修复；引擎常量在 `internal/autotag/budget.go`。
- Seen：2026-11-09。
- Check：待升级。下一步用参数化测试把每个数字行的范围与引擎常量对齐。

### L5 验证必须在要交付的构建上

- 为什么：多次出现"应用没有更新"，验证的是旧二进制；`open` 复用实例让它更隐蔽。
- 证据：gate 的 smoke 步骤从 `open` 改为直接跑 bundle 二进制（round 237–238）。
- Seen：2026-11-09。
- Check：`tools/smoke.sh`；freshness 门禁仍未建（R4）。

### L6 修一个缺陷之前先 grep 同类

- 为什么：同一个根因往往在兄弟控件上还有第二份；只修截图里那一处，下一张截图还会回来。
- 证据：API Key 的读取路径与 provider 切换；数字字段三个调用点。
- Seen：2026-11-09。
- Check：人工。证据块的 `Broken by hand` 和"同类已 grep"两项是它的执行点。

### L7 从旧表面搬来的页面会带着旧表面的内边距

- 为什么：设置页是从扩展的对话框改成页面的，12/24 的内边距属于对话框；页面下沿因此多出
  一段别的页面没有的空白。
- 证据：`TestTheSettingsPageKeepsMagpiesPagePadding`、F6-1。
- Seen：2026-11-09。
- Check：该测试钉住 8/8；`grammar.md` 的 token 表带出处。

### L8 文案承诺的边界必须在实现里兑现

- 为什么：状态栏说"当前批次结束后会停下来"，但停止按钮取消的是运行上下文，正在飞的请求被
  中断，那一批的结果也没了。引擎本来就有协作式取消（波次边界），外壳没有用它。
- 证据：`TestAutoTagStopKeepsTheBatchItIsOn`（一个请求在途 → 停止 → 该请求完成、建议入库、
  不再发第二批判）；三个停止入口统一走 `StopAutoTag`。
- Seen：2026-11-09。
- Check：该测试 + `TestAutoTagStoppingOffersNoSecondStopButton`。

### L9 恢复必须先读失败状态里的用户决定

- 为什么：`HealStaleRun` 把持久化的运行一律记成 `interrupted`，即使状态里写着
  `CancelRequested`；计数又抄了上一次的 summary。用户主动停止的运行因此会被自动续跑，
  面板还会报告一个从未发生的进度。
- 证据：`TestHealingACancelledRunKeepsItCancelled`、`TestHealStaleRun` 的计数断言。
- Seen：2026-11-09。
- Check：两个测试 + `shouldResumeInterrupted` 的判定表。

### L10 以“像不像参考”为尺子的审计，看不见参考自带的缺陷

- 为什么：十轮走查的判定标准是“与扩展一致”。侧栏保留的两个空操作位来自
  `extension.css:3007-3010`，关系图的环来自 `graphModel.ts`，parity 测试甚至把环钉住了；
  复刻得越准，这两处越不可能被标成缺陷。审计的尺子同时就是它的盲区。
- 证据：第 11 轮两处“与源完全一致”的缺陷；`TestGraphViewModelMatchesTypeScript` 钉住的正是
  用户说“不能用”的那个布局。
- Seen：2026-11-10。
- Check：待升级。下一步在 `tools/criteria.sh` 里加一条：每轮至少一处判断写成“不参照源，只
  问读者能不能用”，并记录结论。

### L11 走查必须在真实体量下进行

- 为什么：六个维度（信息架构 / 流程 / 效率 / 反馈 / 层级 / 一致性）问的都是部件之间的关系，
  没有一条问“这个功能在真实数据量下能不能用”。capture fixture 只有几条种子：60 个节点、76 个
  标签、9509 条书签、212 点侧栏这些量级只在用户机器上出现，于是环上的标题叠成灰、标签行空出
  三分之一，十轮都拍不到。
- 证据：第 11 轮三处全部由用户在真实库上发现；关系图在 60 节点下只有个位数标题放得下。
- Seen：2026-11-10。
- Check：待升级（board Inbox “真实体量走查”）。在生成器落地前，人工：每轮至少一次在真实库上
  逐页走。

### L12 抓图的状态必须走真实入口的构建路径

- 为什么：`capturestate.go` 的 `graph` 状态只置了 `GraphMode`，没建模型，拍出来是空态。第 10
  轮据此写下“未验：有关系节点的图”，把取证缺陷记成了未覆盖范围——于是最该被看的那一页，
  恰好没有任何一轮真正看过。
- 证据：第 11 轮修 `capturestate.go` 后同一状态拍出了真实图；第 10 轮结论因此作废。
- Seen：2026-11-10。
- Check：人工。`capturestate.go` 每个 case 都必须调用与点击相同的入口函数，不能只改状态位。

### L13 数字与后缀是两段字符串，不是一条格式串

- 为什么：关系图的作者层把“数量 + 后缀”当成一条翻译文案交给 `Tf`。`GraphAuthorNodesSuffix`
  在两种语言里都没有动词，`fmt` 于是把多余参数原样打进界面：`位作者节点 %!(EXTRA int=18)`。
  这不是翻译缺失（`AssertComplete` 会拦住那种），是**调用形状**错了——源自己也是
  `{n} {suffix}` 两段拼的（`TagGraphView.tsx:99`）。凡是“数 + 量词 / 数 + 后缀”的文案，
  两段分开取，数字不进翻译。
- 证据：`TestTheAuthorLayerIsCountedInWords`；复原实现上读到 `author nodes%!(EXTRA int=3)`。
- Seen：2026-11-11。
- Check：现有测试。全库只有这一处这种形状（已改成 `Sprintf`）；`check-text.sh` 不比对 verb 与
  参数的数量，缺口记在 board Inbox。

### L14 改了名字，要跑整包，不只跑新测试

- 为什么：第 16 轮把详情标签贴纸的无障碍名从 `Remove` 改成 `Remove <tag>`，提交时只跑了那一轮
  新增测试所在的 `-run`。`TestTheDetailTagPillCanBeRemovedFromTheKeyboard` 仍在 `tabUntil` 里找
  裸动词，于是一整片键盘可达性检查在提交后是红的——直到第 17 轮跑整包才照出来。改名是一个
  跨测试的契约变更，新测试绿不能证明旧测试还绿。
- 证据：第 17 轮 `go test ./internal/ui` 报 `Tab never reached the tag's remove control`，
  同一份构建里 `Remove rust` 已经出现在文本里；改成找带标签的名字后整包绿。
- Seen：2026-11-11。
- Check：现有流程。每次提交前跑 `go test ./internal/...`（gate 的 tests 步骤也会跑）；只跑
  `-run` 只适用于修复过程中的快速回路，不适用于收尾。

## 已升级（只留检查）

（暂无。规则升级后从上面移到此处，只保留一行：规则 → 检查。）

### L15 抓图状态要选中“画出来的那一行”，不是“存起来的第一行”

- 为什么：`tags-editor` 状态把编辑器开在 `TagItems[0]` 上。仓库按名称返回标签，表格按书签数
  绘制；名称上的第一个标签恰好排在几十行之外，于是四份“内联编辑器”截图全是普通表格。这不是
  渲染缺陷，是取证缺陷——和 L12 同一类：状态没有站在读者会看到的位置。
- 证据：第 23 轮深/浅 × 中/英 四张 `tags-editor` 全等于 `tags`；红测试读到焦点在 `t-alpha`、
  首行是 `t-zebra`。
- Seen：2026-11-11。
- Check：现有测试。抓图状态凡是选“第一个”的，都问一句：视图排序与仓库排序一致吗？不一致就
  选 `Visible*`。

### L16 放进 state 的翻译值是状态，语言一换就得重算

- 为什么：设置页的两个选择器绑定的是 `AppearanceLabel` / `LocaleLabel`，值是翻译后的字符串。
  设置页自己的语言处理重算了它们，页脚的语言按钮没有，于是中文页面里留下一行 “Light”。只要
  一个翻译值是“上一刻读出来的”，它的每个入口就都要重算；靠其中一个入口顺手调用，迟早漏掉另一
  个。抓图路径之所以没暴露，是因为 `XBM_CAPTURE_APPEARANCE` 恰好在语言之后又重算了一次。
- 证据：`TestTheSidebarLanguageChipRelabelsTheSettingsPage`；红时读到 `Light`。
- Seen：2026-11-11。
- Check：现有测试。第 459 轮曾把所有 `state` 字段里的翻译值列成两类（状态 vs 一次性回执）；
  这条是第三类：绑定到控件的选项值。新增这类字段时，把重算放进 `SetLocale` 而不是某个入口。

### L17 藏在交互后的条形 / 面板，要有一个抓图状态，否则永远不算看过

- 为什么：批量条只在勾选卡片角上的方框之后出现。前 24 轮拍了 20 多个状态，没有一张包含它，
  于是“批量操作”这一页在记录里一直是“没发现问题”。第 25 轮补了 `batch-selected`，第一张图就
  同时暴露了两类缺陷：写入没有回执、两个选择器只有一个空箭头。缺陷不是这一轮才出现的，是这
  一轮才被看见的——没拍到的状态等于没审过。
- 证据：`docs/screenshots/batch-selected-light-zh.png` 与修复前同名图；红测试
  `TestABatchActionSaysWhatItDid`、`TestTheBatchBarSaysWhatEachPickerChooses`。
- Seen：2026-11-12。
- Check：现有清单。`ApplyCaptureState` 的每个 case 对应一个“有独立画面的状态”；审一页之前先
  数一遍：这一页有没有只在某个动作之后才画出来的条 / 面板 / 横幅？有就补状态再开始看。

### L18 需要读者做选择的破坏性动作，默认值必须是空，而不是“列表里的第一个”

- 为什么：同一个形状两轮各抓一次。第 25 轮是“移动到列表”：选择器只有一个箭头，未选时静默
  移进第一个列表；第 28 轮是“合并到”：勾任何一行就把目标写成当前列表的第一个标签，按一下
  合并就把它折进去。两处的写法都出于同一个想法——“总得有个值”——而破坏性动作里，这个值就是
  替读者做的决定。正确的形状是：占位项 + 按钮禁用 + 空值时一句指名缺什么的提示（选目标 /
  再选一个标签），源在 `TagResultsPane.tsx:929-943` 与 `BookmarkResultsPane.tsx:69` 都是这么
  写的。
- 证据：`TestAPressOnTheUnchosenPickerDoesNothing`、`TestMergeNeedsATargetTheReaderChose`；
  红时分别读到选择被移进 Inbox、目标被写成 `"agentic ai"`。
- Seen：2026-11-12。
- Check：现有测试。看到任何“选择器 + 确认按钮”的组合，先问：没选时按钮能不能按？按了会
  不会落到列表里某个“第一个”上？再把答案写进测试。

### L19 一条测试可以把缺陷写成规格，修根因时要先删掉它

- 为什么：`TestTheMergePickerCommitsTheTargetItShows` 断言“选择器显示第一个标签时，
  `TagMergeTarget` 也必须写上它”。它描述的是选择器与状态不一致这个*症状*，而症状的根因正是
  “替读者选了第一个标签”。保留它就只能修症状；第 28 轮删掉它并换成“目标从空开始”，根因才
  消失。测试数量不该是目标，测试描述的契约才是。
- 证据：删除 `tagmergepicker_test.go`（77 行）；替换为 `tagmergesafety_test.go` 四项。
- Seen：2026-11-12。
- Check：现有测试。红测试写不下去、或必须绕着实现写时，先读它断言的到底是什么；如果它断言
  的是一个由缺陷产生的中间状态，改的是测试。

### L20 慢动作不要在按下时销毁触发它的界面；没落地的那部分要留在原处

- 为什么：词表审阅把 `ReviewOpen = false` 写在 `runLibraryWrite` 之前，于是按下“应用所选”的
  瞬间面板消失、全库关系重写在无声进行；一个计划失败就 `return`，后面的好计划连尝试都没有，
  而面板已经关了，谁也看不到哪些生效、哪些没生效。源的做法是三个字：留着它——面板不关、按钮
  变 busy、失败逐条收集并显示在列表之上，剩下没做的还勾着，下一次按就是重试
  （`OptionsApp.tsx:777-819`、`TagResultsPane.tsx:820-905`）。
- 证据：`TestThePanelStaysUpWhileItApplies`（红：面板已关、无忙碌标记）、
  `TestOneFailedPlanDoesNotDiscardTheRest`（红：好计划未执行、面板关闭、错误为空）。
- Seen：2026-11-12。
- Check：现有测试。任何“按下 → 关闭自己 → 后台干活”的路径，问三句：干到一半屏幕上有什么？
  失败了还能不能重试没做完的那部分？完成后回执画在哪一页、读者在不在那一页？

### L21 同一个状态有两个读者时，两处判定会各自漂移

- 为什么：一次同步的状态有四个读者——侧栏的点（`workspaceStatus`）、状态行的 tooltip
  （`workspaceStatusLabel`）、英雄区的句子（`syncStatusLabel`）、以及 To be 的页脚。前三个各自
  写了一个 switch。第 30 轮补取消状态时只改了前两个，第三个早就有了，第二个漏了 case，于是
  点变色、英雄区写“已取消”、tooltip 写“空闲”。读者不会知道那是两个函数；他看到的是同一个
  运行被叫了两个名字。让 label 从 status 派生也不是免费方案：返回类型不同，仍要一张全表；有效
  的做法是让测试同时断言两者，任何一个 switch 漏 case 就红。
- 证据：`TestACancelledSyncSaysCancelled`（红：`a stopped sync is labelled "status.idle", want
  "status.cancelled"`）。
- Seen：2026-11-12。
- Check：现有测试。看到对同一枚举的多个 switch，逐个比较 case 集合；有 `default` 兜底的尤其
  危险——漏掉的分支不会报错，只会说错话。给这类函数写一条“两个读者同问同答”的测试。

### L22 一个字段装三种语义时，界面只能靠猜；语义要显式，样式才能确定

- 为什么：`LastBundleReport` 同时装进度（“正在导出…”）、结果（“备份已导出”“本地数据已重置”）
  和拒绝（“选中的行太多”），页面用同一个 `settingsNote` 画。于是刚完成的操作和卡片下面的说明
  长得一样，读者没有理由把它当成“发生了”。颜色不是装饰：它是“这句话属于哪一类”的唯一线索。
  第 31 轮加了 `LastBundleOK` 来回答“这是结果吗”，样式才第一次有依据。抓图时还发现另一个
  教训的同款：钩子只写了句子、没写标记，画面就诚实地画成了提示色。
- 证据：`TestResetSaysTheLibraryIsGone`（断言 `LastBundleOK`）；`settings-reset-light-zh.png`
  第一次拍出来是灰色，补上标记后才是绿色。
- Seen：2026-11-12。
- Check：现有测试。一个字符串字段被三种语义写入时，先列出写入点，再问：界面怎么知道现在是
  哪一种？答不上来就不要靠文案猜，加一个布尔或枚举。

### L23 框架的“减少动态”语义要读实现，不要读注释

- 为什么：`pulse` 上方的注释写着“帧结束在 1，所以在减少动态下看到的是稳定全亮”。听起来完全
  合理，因为最后一帧确实是 1。但 MyGo 对减少动态的处理是“把序列取到 `final()`”，而
  `final()` 对**永不结束的往返**（`RepeatForever + Alternate`）返回的是**第一帧**——plays()
  为 0，0 是偶数。于是注释描述的是一厢情愿，实际画面是 55% 透明度。红测试把这句话变成了可
  执行的问题：`a pulse under reduced motion shows at opacity 0.55`。
- 证据：`TestAReducedMotionPulseStaysFullyVisible`；`mygo@v0.4.6/ui/motion.go:646-651`。
- Seen：2026-11-12。
- Check：现有测试。任何依赖框架“在某个偏好下会怎样”的代码，先读那一分支的实现并写一条断言
  它的测试；注释里的因果链不算证据。

### L24 上限拒绝必须和“另一条路”一起出现

- 为什么：`MatchingIDs` 在 9,562 条时超过 1000 上限，批量条回一句“一次选择的条数过多”——
  这句话诚实，但它把读者留在原地：界面上再没有第二个能选中的控件，只能一行行手点。源在同一
  位置并排放了两个按钮（“选择当前结果” / “选择已加载”，`BookmarkResultsPane.tsx:45-52`），
  我们只搬了会拒绝的那个。小样本 fixture 永远碰不到上限，所以 25 轮“批量操作”巡检时它看起来
  完全正常；只有真实库让“按下去会发生什么”变成一个能回答的问题。
- 证据：`TestARealScaleLibraryCanStillBeBulkSelected`（红时文本只有 `Select all results`、
  `Too many results to select at once`、`100 / 1200 results`）；`batch-selected-scale-light-zh.png`。
- Seen：2026-11-12。
- Check：现有测试。凡是“超过 X 就拒绝”的分支，先问：同屏有没有一个不超限的等价动作？没有
  就在源里找那个按钮（源通常已经把两种情况拆开），别把它改小上限了事。

### L25 在点击处理器里等一个“需要事件循环才能完成”的东西，就是让窗口等自己

- 为什么：`mygo.Dialog.Save` 是阻塞的，而且允许从任何 goroutine 调用——它在内部把真正
  的弹窗 post 到主线程，然后等回调。放在点击处理器里，处理器就不返回；处理器正在主线程的
  事件派发里，派发不结束，窗口就不能重画。面板如果因为任何原因没出现，屏幕上就没有一丝
  变化，读者得到的是“这个按钮坏了”。用户报的“导出数据没有任何响应”正是这个形状。
- 证据：`TestClickingExportAsksForAPathOffTheWindowThread`（红：`the save panel was opened
  inside the click handler`）；`settings-export-light-zh.png`（点击后的第一帧：按钮变灰 +
  “正在选择保存位置…”）。
- Seen：2026-11-12。
- Check：现有测试。凡是处理器里调用的 API 文档写着“blocks”“waits”“modal”“shows a
  panel”，先问它靠谁推进；靠事件循环的，就把它放进工作里，让处理器先返回，并在返回前
  把“我在等”写进状态。

### L26 可点的反馈画在整块上，可点的范围却只在角落

- 为什么：条件 chip 悬停时整块换底色、整块变字色，指针走过去得到的回答是“这一块都能按”；
  但真正的处理器只挂在 16 点的 × 上，按在字上什么都没有。第 34 轮的教训是“按下去要留下
  痕迹”，这一轮补上更早的一步：痕迹要覆盖控件画出来的范围。源里整个 chip 就是一个
  `<button>`，这条不需要猜。
- 证据：`TestTheConditionChipIsOneButton`（红：`pressing the condition's own word did
  nothing: only its x removes it`）。
- Seen：2026-11-12。
- Check：现有测试。给一个控件加了 hover / active 的整块反馈时，回头确认处理器也挂在同一块
  上；一个控制里有两个可点区域时，先想清楚它们是不是同一个动作（本例 facet 的移除是
  toggle，挂两次会把条件又打开）。

### L27 tooltip 不是可访问名称

- 为什么：`IconOnlySized` 的注释写着“The tooltip is the accessible name”，于是所有纯图标
  按钮只设了 `Tooltip`。测试器的 `Find`（“anything that carries a text or an accessibility
  label”）一个都找不到——搜索框的清除按钮因此连测试都点不到，读屏器同理。真正给名字的是
  `Label`；设置行里的 `settingsIcon` 两个都设，反而是对的。
- 证据：`TestTheToolbarAnswersEveryClick` 的清除一步（红时 `ui: no element shows "Clear"`）；
  `IconOnlySized` 加 `Label` 后同一步转绿。
- Seen：2026-11-12。
- Check：现有测试。任何“只有图标”的控件，断言 `tester.Find(名字)` 能找到它；找得到才算
  有名字，tooltip 只证明鼠标悬停时有提示。

### L28 重复出现的控件，名字里要带“是哪一个”

- 为什么：结果列表里每张卡片都有一个选择框，全都叫 “Select bookmark”。单个看没问题，
  在一列同名控件里它就无法回答“我这一下选的是哪条”。源早就在名字里拼了作者
  (`BookmarkCard.tsx:142`)，反而把信息补在重复的地方。第 35 轮修的是“没有名字”，这一轮
  修的是“名字没有区分度”。
- 证据：`TestTheCardAnswersEveryClick`（红：文本里两处一模一样的 `Select bookmark`；
  `the card's box is not named after its author`）。
- Seen：2026-11-12。
- Check：现有测试。同一个控件在列表里重复出现时，问它的可访问名字能不能唯一指向一行；
  不能就用带标识的格式串。
- 第 40 轮补记：同一条规则在标签表格上漏了一整张表——四个动词、选择框、主题格全都
  不带标签名（源逐行拼了，`TagResultsPane.tsx:118`、`:148-199`）。也就是说 L28 修的是
  “卡片”这一处；凡是“一列里重复出现的控件”都要按表面清点：卡片、表格行、侧栏行、
  搜索结果。改完记得再 grep 一遍同形状的 Label 常量。

### L29 修好公共构造函数，不等于修好了所有同类控件

- 为什么：第 35 轮把“纯图标按钮只设 tooltip”的缺陷修在 `IconOnlySized` 里，所有走这个
  构造函数的工具栏按钮同时有了名字。但详情右上角的关闭控件是手写的一个 Box——同样的
  形状、同样的缺陷，因为没走那个函数而原样留下。它还是覆盖层里唯一一条用指针退出的
  路，所以这个漏网比工具栏里的任何一个都重。教训不是“再修一次”，而是：共享构造函数的
  修复只覆盖它的调用者，修完要按“形状”再普查一遍手写的实例。
- 证据：`TestTheDetailAnswersEveryClick`（红：`clicking "Details": ui: no element shows
  "Details"`；绿：同一步关掉覆盖层）。源的名字本来就在
  `BookmarkInspector.tsx:174-177`，`shell_test.go:177` 的注释也写明了，只差实现。
- Seen：2026-11-12。
- Check：现有测试。把名字/命中区一类的修复放进公共构造函数后，`grep` 一遍同形状的
  手写控件（本仓库用 `.Tooltip(` 找只有 tooltip 的盒子），逐个确认是否也设了 `Label`。

### L30 相邻的两个控件同名，等于两个都没名字

- 为什么：批量条里标签选择器和写标签的图标按钮都叫「Add tag」，列表选择器和「Move to
  list」按钮同名。单独看每个都有名字，放在一起，读屏器念出两遍同一个词，读者不知道
  哪一个会弹出列表、哪一个是按了就写。源的选择器叫 `chooseTag`，按钮叫 `applyTag`——
  一个说“选什么”，一个说“按了做什么”。L28 说的是同一个控件重复出现时的区分，这一条是
  不同控件撞名。
- 证据：`TestTheBatchBarAnswersEveryClick`（红时选择器只能按旧名找到、移动那步菜单
  打不开；改后“Choose tag”/“Choose list”各自唯一）。
- Seen：2026-11-13。
- Check：现有测试。给一个触发器起名时，先 grep 同一行（`strings.ToLower`）里还有没有
  另一个控件用同一个 key；有就按角色分开命名（选择器 = 选什么，按钮 = 做什么）。

### L31 同一个控件有两个绘制点，等于每次输入都换了一个控件

- 为什么：设置页的查找框在“没筛选”和“正在筛选”时由两条不同的调用画出——一条画固定
  宽度，一条画 `Grow(1)`。长得一模一样，但对 MyGo 来说是两个元素：第一帧是 A，读者打一个
  字母，状态变成“正在筛选”，下一帧画的是 B，A 连同它的焦点一起消失。手快的读者看到的是
  “只能打一个字”；测试看到的是查询停在 `"b"`、退格也不生效。布局上的“同一个位置”不等于
  元素上的“同一个元素”。
- 证据：`TestTheSettingsPageAnswersEveryClick` 的查找框一步（红：`clearing the find field
  left the page filtered`；绿：打五个字母、退五次格都作用在同一个框上）。定位时用临时
  测试打出 `typed b: query="b" focused=false`，确认是失焦而不是按键没到。
- Seen：2026-11-13。
- Check：现有测试。一个输入框（或任何带焦点、滚动位置、拖拽状态的控件）如果按状态在
  不同分支里被画出，先问“换状态时它还是同一个元素吗”；要保留身份，就在一个调用点画，
  只改属性（宽度、`Grow`），旁边按条件出现/消失的兄弟用 `Invisible()` 保住槽位。
- 第 42 轮补记：L30 的同名还有一种更难看见的形状——**行名与行里的按钮同名**。规则编辑器
  的 `settingsItem` 把「Add rule」画成行名，又在行尾的按钮上画一次；行名占满整行宽度，
  点击落在名字上，按钮永远按不到（测试加了两条规则，存下来 0 条）。形状上不是“相邻两个
  控件”，而是“一个控件被画了两遍”。给行起名之前先看这一行自己有没有带文字的按钮；有就
  让行留空（`body`），把名字交给那个按钮。
- 第 43 轮补记：同一个坑在“自建控件”上又出现了一次——为了能翻译占位符，工具栏把
  `ui.SearchField` 换成了 `TextInput` + `Placeholder`，名字随之消失。换掉平台控件时，
  要按 L27 把它的可访问名称一起搬过来；一个 `searchField` 助手修的是三个页面的搜索框。
