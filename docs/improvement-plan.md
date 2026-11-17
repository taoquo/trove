# 改进实施计划

这份计划把 2026-11-15 审计得到的 45 条问题，变成一串**先写断言、再改代码**的改动。
它不是待办清单的堆叠：每一条都先有一个今天会红的测试，改完之后它变绿，并且留在 gate 里。

> **状态**：计划，未开始实施。条目编号与审计报告一一对应。
> **适用范围**：全部 45 条，分 8 个 wave。Wave 0 是其余所有改动的安全网，必须最先落地。

---

## 0. 原则与纪律

1. **先红后绿。** 每条的第一次提交只有测试，`go test ./...` 必须是红的；第二次提交才是修复。
   提交信息写 `test:` 与 `fix:` 两条，或者一条 `fix:` 里带上红转绿的证据（测试名 + 失败原因）。
   没有红过的测试不算验证，它只证明代码能跑。
2. **断言是一句话。** 每条修复前先写下"修复后必须为真的一句话"，例如"一页里有 20 条推文、
   但游标前缀不认识时，这次同步不得被记成全量完成"。测试名用这句话的英文短句。
3. **不做静默的功能删除。** 归档那类"只做了一半"的功能，要么补完，要么明确移除并在
   `CHANGELOG.md` 写清楚（未读就是这样处理的）。修复必须是正向的：用户要么得到新能力，
   要么得到一句准确的失败说明，不能只是少了一个入口。
4. **失败必须可见。** 任何被吞掉的错误，修复方向都是把它变成一个类型、一句文案、一条账本行，
   而不是换个地方继续吞。
5. **迁移只追加。** v1–v6 一个字都不改；所有 schema 变更进 v7（必要时 v8）。
   `TargetSchemaVersion()` 与测试里的版本断言自动跟上。
6. **检查器与负样本成对。** 本计划要加 5 个检查（见 §3），每一个都要在 `tools/selfcheck.sh`
   里加一段"故意破坏、必须变红"的负样本，否则不进 gate。
7. **文案与表面索引同步。** 用户可见字符串进 `internal/i18n`（中英各一条），
   新增/改名绘制方法在同一次提交里更新 `docs/ui-surface-index.md`。
8. **碰外部系统的改动必须真跑。** X 同步、LLM、Keychain 三条线，改完在没有真实运行时
   必须在提交信息里写 `not tried` 和原因；真实跑过的把命令、数据目录、结果写进去（不含密钥）。
9. **每次提交前跑完整 gate。** 时间不够只能缩小当次改动范围，不能跳过 gate。

---

## 1. 阶段划分与依赖

```text
Wave 0  安全网：迁移前快照 + 测试工具 + 门禁骨架          （无依赖，先做）
Wave 1  同步正确性 1–7 + 凭据 8–9                          （依赖 W0 的快照与 fake 工具）
Wave 2  LLM 成本与密钥 10–14                                （独立；可与 W1 并行，不同文件）
Wave 3  备份与恢复 15–16                                    （依赖 W0；与 W2 并行）
Wave 4  每日界面 18–26 + 归档补完（20/21）                  （依赖 W1 的同步状态文案）
Wave 5  性能与规模 27–34                                    （依赖 W4 的 reload 语义定稿）
Wave 6  产品缺口 35–40                                      （35/37 需要 v7/v8，建议与 W5 同一批迁移）
Wave 7  工程与门禁 41–45                                    （41–43 依赖 W5 的预算数字）
```

并行约束：**W1 与 W3 都碰 `importexport`/`storage`**，按文件分工，不按 wave 分工；
两个 wave 不能同时改 `internal/storage/sqlite/` 下的同一个文件。W5 与 W6 共用 v7 迁移，
必须由同一个人合并成一条迁移。

规模标记：**S** ≤ 半天，**M** 1–2 天，**L** 3–5 天。含测试与 gate。

---

## 2. Wave 0 — 安全网

目标：让后面每一条改动都可以被安全地试错。

### 0.1 迁移前快照（对应第 17 条）

- **断言**：一个 v5 的库升到 v7 之后，同目录一定有 `trove.db.before-v7`，它能被独立打开，
  `PRAGMA integrity_check` 为 ok，且行数与原库一致。
- **改法**：`internal/storage/sqlite/migrations.go` 的 `Migrate`，在 `current < target` 且
  存在任何非纯新增迁移时，先 `VACUUM INTO '<path>.before-v<current>'`（一致性快照，不受
  WAL 影响），再用 `0600` chmod；保留最近两份，更早的删除。
- **验证**：`migrations_test.go` 用真实历史重放造 v5 文件（现有
  `TestTheUnreadColumnLeavesAnOlderLibrary` 已经是这个手法），再断言备份文件存在且可读。
  负样本：把备份步骤注释掉，测试必须红——这条写进 `selfcheck.sh`。
- **风险**：大库上 `VACUUM INTO` 需要一次完整拷贝（9,562 条约 4 MB，可接受；100k 条约 40 MB）。
  在设置页文案里说明"升级前会在数据目录留一份快照"。

### 0.2 测试工具（新增 `internal/testkit`）

只放测试用的东西，不参与运行时：

| 工具 | 用来测什么 |
|---|---|
| `Clock`（可控时间） | 限流窗口、心跳过期、备份保留 |
| `Transport`（按脚本返回状态码/头/体） | 429、x-rate-limit-reset、截断 JSON、402 |
| `FlakyRepo`（第 N 次调用失败） | 恢复回滚、迁移失败、批量写 |
| `CredentialStore`（返回指定错误种类） | Keychain 锁定、NotFound、普通失败 |
| `TempLibrary(t, n)` | 已有 `buildLibrary`，抽到公共处 |

- **断言**：每个工具自己有测试（例如 `FlakyRepo` 在第 N 次真的失败）。
- **规模**：S。

### 0.3 门禁骨架

- 在 `tools/gate.sh` 里预留位置之前，先确认现有 14 项仍然全绿；本 wave 不改 gate 组成，
  只增加 `tools/bench-budget.sh` 的空实现（Wave 7 填内容）。
- **规模**：S。

---

## 3. Wave 1 — 同步正确性（1–7）与凭据（8–9）

### 1. 结构漂移不得被记成"全量完成"

- **断言**：一页里有 20 条推文、但没有一条被认出的游标时，`Termination` 不得是
  `TermEndOfPages`；这次运行不得写 `full_sync_complete`。
- **改法**：
  1. `internal/xsource/parse.go` 把 `UnrecognizedEntries`、`SkippedEntries` 和
     "是否见到过游标" 一起放进 `ParseResult`。
  2. 新规则：`tweetEntries > 0 && NextCursor == ""` → `ErrResponseSchemaChanged`（现在只在
     "全部条目都不认识" 时才报）。
  3. `ports.BookmarkSource.FetchPage` 返回一个 `PageHealth{Recognized, Unrecognized, HasCursor}`；
     syncer 在 `CompletesFullSync` 之外再加一条 `PageHealth.Unrecognized == 0`。
  4. `domain.CompletesFullSync(reason, failed, unrecognized)` 改签名，调用方同步。
- **验证**：`xsource` 表驱动测试：空游标 + 20 条推文 → 报 schema 变化；
  syncer 集成测试用真实 SQLite：运行结束 `HasCompletedInitialFullSync == false`。
- **真实证据**：用 `TROVE_CAPTURE_STATE` 之外的方式，打开「保存原始负载」，真实同步一页，
  从 `raw_payloads` 里取该页，脱敏后放进 `testdata/xsource/real-page-*.json` 作为回放语料。
  这一步不放进 CI（含真实内容），但测试可以用它的**结构**做形态检查。
- **规模**：M。

### 2. 失败页保留续跑点

- **断言**：第 450 页失败后，`ResumeCursor` 非空，下一次运行从第 450 页的游标继续，
  第 1–449 页不会被再次请求。
- **改法**：`pagination.collector.result` 在 `TermFailed` 时也保留最后一个**已成功读到的**
  `NextCursor`（失败的那一页自己返回的游标不可信）；`syncer.recordResumeCursor` 在失败路径
  同样写入。
- **验证**：`testkit.Transport` 在第 450 次请求返回 500；断言第二次运行的请求次数从 450 开始。
  负样本：删掉 `TermFailed` 分支，测试红。
- **规模**：S。

### 3. CLI 续跑

- **断言**：`trove sync` 在存在续跑游标时，`RunOptions.StartCursor` 与窗口传的值相同。
- **改法**：`cmd/trove/cli.go` 的非增量分支加 `StartCursor: syncService.ResumeCursor(ctx)`，
  与 `internal/ui/sync.go:101` 完全一致；两处抽成 `syncer.ContinueOptions(ctx, prefs)`，
  防止再次漂移。
- **验证**：`cli_test.go` 用假 syncer 捕获 options；再加一条源码级检查（见 §3 检查器 C4）：
  `cli.go` 里出现 `syncer.RunOptions{` 时必须同时出现 `StartCursor`。
- **规模**：S。

### 4. CLI 的部分成功

- **断言**：`Run` 返回 `partial_success` 且带错误时，`trove sync` 退出码为 0，并打印
  "saved N new, … (partial: K page(s) failed)"。
- **改法**：`cli.go` 先判 `run.Status`，`SyncPartialSuccess` 走成功路径并打印页失败数；
  只有 `SyncError` 才 `syncFailure`。
- **验证**：`cli_test.go`：部分成功的 fake run → exit 0；完全失败 → exit 1。
- **规模**：S。

### 5. 跨进程同步互斥

- **断言**：同一份库上，第二个同步在第一个心跳新鲜时被拒绝，返回
  `ErrSyncAlreadyRunning`（一句可翻译的话），且不会把对方的 `running` 行改成 interrupted。
- **改法**：
  1. `sync_runs` 已有心跳字段（`runstate.go`）——把"启动时无条件 `MarkInterrupted`"改成
     "只关闭心跳超过 90 秒的 running 行"。`CleanupOrphanedRuns` 与 CLI 共用这条规则。
  2. `syncer.Run` 开始时在一个事务里读取 running 行：心跳新鲜 → 拒绝；否则接管。
  3. CLI 遇到拒绝时退出码 0、打印 "another sync is running"，不写错误。
- **验证**：同一进程起两个 syncer（不同 service 实例、同一 DB）→ 第二个被拒；
  把心跳改到 2 分钟前 → 第二个接管。`testkit.Clock` 控制。
- **规模**：M。

### 6. 墓碑不参与早停计数

- **断言**：删掉最近 5 条书签后，一次增量同步的页数不超过正常增量 + 缓冲页。
- **改法**：早停用的集合改为"所有已知 id，含墓碑"。仓库已有 `deleted_bookmarks` 表，
  新增 `KnownIDs(ctx)`（`UNION` bookmarks 与 tombstones），syncer 用它；
  `FetchedCount` 改为"实际写入 + 更新"的计数，跳过的墓碑不计数。
- **验证**：SQL 层测试：已知 id 集合包含墓碑；syncer 集成测试：删 5 条后页数回到 1–2 页。
- **规模**：S。

### 7. 限流窗口

- **断言**：429 带 `x-rate-limit-reset = now+300s` 时，运行以 `rate_limited` 结束，
  并且续跑游标被保留；不会在 30 秒后无谓地重试。
- **改法**：`client.parseRateLimitReset` 读 epoch 秒；`pagination` 的 `MaxRetryAfter`
  分成"单次等待上限"（30s，防挂起）与"可接受的窗口"（15 分钟）：窗口内的等待拆成
  若干段，每段前检查 context；窗口超出上限 → 立即结束并保留游标（配合第 2 条）。
  UI 状态行显示倒计时（配合第 26 条）。
- **验证**：`testkit.Clock` + 假 transport：断言等待总时长、结束原因、游标；
  负样本：把 reset 头删掉，只留 Retry-After，行为必须退回旧路径且测试红。
- **规模**：M。

### 8. 凭据错误不再伪装成登录过期

- **断言**：Keychain 被锁时，界面文案是"无法读取钥匙串"，且**不**触发浏览器刷新、
  **不**覆盖已存凭据。
- **改法**：`credentials` 包新增外部错误种类 `ErrStoreLocked`/`ErrStoreUnavailable`
  （只有 `ErrNotFound` 才映射到 `ErrUnauthorized`）；`xsource.client.credentials`
  按种类包装；`usecase/syncer` 的自动刷新只对 `ErrUnauthorized` 触发；
  `ui/errors.go` 与 `i18n` 各加一句中英文案。
- **验证**：`testkit.CredentialStore` 返回锁错误 → 文案、无刷新、无写入；
  返回 NotFound → 仍走"登录过期"。两条断言都在 `usecase` 层。
- **规模**：M。

### 9. 浏览器 cookie 必须成对

- **断言**：两个浏览器各自有完整的一对 cookie 时，选中的一对来自**同一个** profile；
  拼不出完整一对的合并结果被拒绝，且不会写回。
- **改法**：`credentials/browser.go` 从"按名字合并"改为"按 profile 合并"：每个 profile
  读出完整 `{auth_token, ct0}`，优先选当前默认浏览器；都完整时按顺序取第一个；
  没有任何完整对 → `ErrIncompleteCredentials`。`signin.go` 写回前再校验一次。
- **验证**：假 reader 返回两个 profile，断言成对；负样本：一个 profile 只有 `auth_token`，
  另一个只有 `ct0` → 拒绝且不调用 store 写入。
- **规模**：S。

**Wave 1 验收**：`go test ./...` 与 `-race` 全绿；`trove sync` 在临时数据目录真实跑一次
（首次全量 + 中断续跑 + 部分成功三种场景），结果写进提交信息。

---

## 4. Wave 2 — LLM 成本与密钥（10–14）

### 10. 失败批写账本 + 连续失败熔断

- **断言**：同一批书签在第二次运行中不会被再次请求；连续 3 批失败后运行以明确原因停止。
- **改法**：`llm_analysis` 增加 `outcome = 'error'`（CHECK 约束扩展，v7），失败批按
  "批内每个书签"写一行，记 `prompt_version`、`model`、`content_hash`、错误种类；
  候选筛选排除"同 hash + 同 prompt 版本 + outcome=error"的行，直到 prompt 版本或模型变化。
  连续失败计数放进运行状态，超过阈值即 `TerminatedByErrorBudget`。
- **验证**：假 LLM 恒返回安全过滤响应 → 两次运行，第二次请求数为 0；
  熔断：连续 3 次 500 → 运行提前结束且账本有 3 行 error。
- **规模**：M。

### 11. 输出 token 预算接上线

- **断言**：批量 25 的请求里 `max_tokens` = `DeriveMaxOutputTokens(25)`（1600），
  Anthropic 不再固定 1024；`DeriveMaxOutputTokens` 有生产调用方。
- **改法**：`llm.ChatRequest.MaxOutputTokens` 由 `autotag` 按批大小填；
  Anthropic 适配器的下限从 1024 改为 `min(required, derived)` 逻辑；
  四个适配器都在 `ChatResult` 里带上 provider 返回的 `usage`，运行汇总里显示
  "本次约 N 输入 / M 输出 token"。
- **验证**：适配器单测断言 payload 字段；`autotag` 测试断言请求携带预算；
  `tools/check-wiring.sh` 增加"导出函数必须有调用方"的检查（C1），负样本见 §3。
- **规模**：M。

### 12. 测试连接真的调一次模型

- **断言**：模型名为空或拼错时，测试连接返回失败并显示 provider 的原话。
- **改法**：`Probe` 之后追加一次最小 `ChatJSON`（一句 prompt、`{"ok": true}` schema、
  8 token 上限），按适配器能力选择 tool-call 或 JSON mode；失败时把分类后的错误显示在设置行。
- **验证**：假 transport：`/models` 200 + `v1/messages` 400 invalid_model → 失败文案；
  正常响应 → 成功，并显示实际模型名。
- **规模**：S。

### 13. BaseURL 校验与权限说明

- **断言**：`http://` + 非 loopback 的 BaseURL 无法保存，除非用户在一个明确的确认框里
  同意"密钥会以明文发往该地址"。
- **改法**：`preferences` 保存路径调用 `ValidateBaseURL`；失败 → 行内错误。
  `PermissionOrigin` 的结果进确认框标题（"允许 trove 把密钥发送到 example.com？"）。
  在 `i18n` 加两句。删除或使用 `PermissionOrigin` 的未调用状态（检查器会要求）。
- **验证**：表驱动：`http://localhost:11434` 通过；`http://1.2.3.4` 拒绝；
  `https://…` 通过。确认流程用 `ui.NewTester` 走一遍。
- **规模**：S。

### 14. 错误详情全面脱敏

- **断言**：provider 回显 `Authorization: Bearer sk-…` 时，写入 `app_settings` 的 summary
  与界面 headline 都不含该串。
- **改法**：`normalizeHTTPError` 的签名从"一个 apiKey"改为"一组秘密值"（从 `spec.Headers`
  里按 `authorization`、`api-key`、`x-api-key`、`x-goog-api-key` 收集，并解析 Bearer 前缀）；
  `redactSecret` 覆盖 URL query 里的 `key=`；`SanitizeLlmError` 接进 `tagging` 的持久化路径，
  而不是只存在于测试里。
- **验证**：表驱动，含 Bearer、api-key、URL query 三种载体；再加一条"落库内容不含 secret"
  的集成测试（写库后读 `app_settings` 全文扫描）。
- **规模**：S。

**Wave 2 验收**：用真实 key 各跑一次四种 provider 的连接测试与一次 25 条批量的打标，
记录 token 用量与耗时；Keychain 锁定场景无法在本机模拟时写 `not tried`。

---

## 5. Wave 3 — 备份与恢复（15–16）

### 15. 原始负载可往返

- **断言**：导出 → 恢复之后，`raw_payloads` 的行数与内容与导出前一致。
- **改法**：`importexport.apply` 读 `FileRawPayloads` 并写入；恢复顺序在 bookmarks 之后
  （外键）；设置页的恢复报告增加"原始负载 N 条"。
- **验证**：`importexport` 往返测试：造 3 条带负载的书签 → 导出 → 清库 → 恢复 → 比对。
  负样本：删掉读文件的循环，测试红。
- **规模**：S。

### 16. 恢复原子化 + 墓碑进备份

- **断言**：在 apply 中途注入失败时，库回到与恢复前**完全一致**的状态，包括
  `raw_payloads`、`deleted_bookmarks`、`tag_suggestions`（含 pending）。
- **改法（两步，允许分两次提交）**：
  1. **短期**：快照改为含负载的完整导出 + 墓碑；失败回滚时先 wipe 再从快照恢复，
     并断言恢复后的计数与快照一致；bundle 增加 `tombstones.json`（`KnownFiles` 白名单与
     manifest 版本 +1，读旧 bundle 时该文件缺失视为空）。
  2. **中期**：把 `wipe + apply` 收进一个 `sql.Tx`——新增 `importexport.TxStore`
     接口，仓库方法接受 `*sql.Tx`；这是本计划里最大的一次结构性改动，单独提交，
     带 `refactor:` 前缀。
- **验证**：`testkit.FlakyRepo` 在第 K 次写失败 → 断言全表计数与内容哈希一致；
  再跑一次成功恢复 → 断言墓碑仍在。负样本：把快照换成 `DefaultExportOptions()`，测试红。
- **规模**：L（第 2 步单独评审）。

---

## 6. Wave 4 — 每日界面（18–26）与归档补完

### 18. 后台刷新不缩减列表

- **断言**：已加载 500 条时触发一次后台 reload，列表仍有 500 条，滚动位置不变。
- **改法**：`reloadQuery` 的 `Limit` 改为 `max(DefaultLibraryPageSize, 当前已加载条数)`，
  上限 1,000；超出上限时保留顶部并显示"已加载 1,000 条"；`applyReload` 在条数变少时
  不重置滚动（只有查询变化才重置）。
- **验证**：`ui.NewTester`：造 500 条，`LoadMore` 到 500，调用一次 `ReloadAsync`，断言长度与
  `resultsScroll.Y`。负样本：把 Limit 改回常量，测试红。
- **规模**：M。

### 19. 选择不因后台刷新被裁

- **断言**：选中 900 条后触发后台 reload，`SelectedIDs` 仍为 900；只有查询条件变化时才裁剪。
- **改法**：`keepSelection` 增加参数 `prune bool`，由"查询是否变化"决定。删除/移除书签时
  仍立即裁剪。
- **验证**：同上测试；再测查询变化时确实裁剪。
- **规模**：S。

### 20. 归档补完

- **断言（五条，各自一个测试）**：
  1. 批量栏在 1 条以上选中时出现"归档"与"取消归档"。
  2. `Cmd+Shift+E` 归档当前卡片；在归档视图里是取消归档。
  3. 归档后的 toast 带"撤销"，撤销后书签回到原列表位置。
  4. 侧栏/视图显示归档条数（`View.ArchivedCount`）。
  5. 空归档显示专属文案，而不是通用"没有匹配"。
- **改法**：
  - `ports.Repository.SetArchivedMany(ctx, ids, bool)` + SQLite 实现（一条 UPDATE）。
  - `toolbar.batchBar` 增加动作；`shell.go` 快捷键表增加；`panels.ShowToast` 增加
    可选动作（`ActionLabel` + `Action func()`），超时仍自动消失。
  - `workspace.View` 增加 `ArchivedCount`（只在归档视图外计算，一次 COUNT）。
  - `results.emptyState` 增加归档分支；`design.md` 状态矩阵补五行。
- **验证**：五个 `ui.NewTester` 测试 + 一条 SQL 批量测试；`docs/ui-surface-index.md` 同步。
- **规模**：M。

### 21. 保存视图包含归档

- **断言**：在归档视图保存的视图，应用后仍处于归档视图；应用其他视图时归档模式被清除。
- **改法**：`domain.SavedView` 加 `OnlyArchived bool`；`saved_views` 加列（v7，默认为 0）；
  `currentViewSnapshot` / `ApplySavedView` / 旧版导出 DTO 三处同步；应用视图时显式清归档。
- **验证**：SQL 往返 + UI 快照测试；旧版导出兼容测试（缺字段 = 0）。
- **规模**：M。

### 22. 清掉硬编码英文

- **断言**：`panels.go` 不再有面向用户的 `fmt.Sprintf` 英文；`check-text.sh` 能抓住这一类。
- **改法**：加 `i18n.SuggestionsApplyFailed`（带两个 `%d`）；扩展 `check-text.sh`：
  Go 文件里 `ShowStatus/ShowToast/ShowBanner` 的参数不得是字符串字面量或 `fmt.Sprintf`
  的英文格式串——这是第 4 个检查器（C3），配负样本。
- **验证**：中文界面截图 + `AssertComplete`；负样本：故意在 `ShowStatus` 里写字面量，selfcheck 必须红。
- **规模**：S。

### 23. 搜索反馈

- **断言**：连续输入 5 个字符只发出 1 次查询（最后一次）；输入与屏幕结果之间不超过 150ms +
  一次查询耗时。
- **改法**：UI 侧 150ms 去抖（用 `Window.Update` 排队，不在 View 里），字段旁显示查询中的
  小型指示；过期请求仍丢弃，但丢弃时保留"较新结果到达后覆盖"的语义。
- **验证**：`ui.NewTester` 用假时钟推进；断言查询次数。
- **规模**：S。

### 24. 键盘补完

- **断言（三条）**：Escape 关闭筛选浮层；侧栏标签/作者行可用上下键移动、Enter 选中；
  主列表是**一个** Tab 停靠点，卡之间用方向键。
- **改法**：`shell.go` 的 Escape 级联增加 `FilterPopoverOpen`；侧栏行加 `OnShortcut` 上下键；
  卡片从 `Focusable` 改为列表级焦点 + 当前卡高亮（`Focusable` 只在详情打开时给内容）。
  这会改绘制方法名与表面索引，同一次提交更新。
- **验证**：三个 UI 测试 + 截图（键盘焦点可见性由 `geometry.sh` 覆盖）。
- **规模**：M。

### 25. View 副作用

- **断言**：同一个状态连续 build 三次，`RenderLimit`、`ResultsColumns`、偏好文件都不变化；
  只有用户手势或滚动事件改变它们。
- **改法**：`autoLoadMore` 改由滚动事件处理器触发（`Window.Update` 里读位置），
  `bookmarkGrid` 的列数由事件重算，偏好写入移到设置处理器。
- **验证**：`ui.NewTester` 连续 build 三次断言零变化；负样本：恢复旧写法，测试红。
- **规模**：M。

### 26. 等待可见

- **断言**：限流等待期间状态行显示"等待 X 秒（限流）"，每秒更新。
- **改法**：`Progress.WaitingUntil` 已在，`sync.go` 的状态行渲染倒计时；本地化两条文案。
- **验证**：假时钟推进，断言文案。
- **规模**：S。

**Wave 4 验收**：两张真实截图（工作区、归档）+ 五个交互测试；`design.md` 与
`ui-surface-index.md` 无漂移（检查器会红）。

---

## 7. Wave 5 — 性能与规模（27–34）

### 27. 排序索引（v7）

- **断言**：五种排序的首屏查询计划里没有 `USE TEMP B-TREE FOR ORDER BY`。
- **改法**：v7 增加部分索引，例如：
  `CREATE INDEX bookmarks_list_saved_idx ON bookmarks(archived_at, saved_at DESC, tweet_id) WHERE archived_at IS NULL;`
  时间线排序改写成不含表达式的等价形式（`timeline_rank IS NULL` 拆成两段 UNION 或
  改用 `COALESCE(timeline_rank, 1<<30)` 并在 Go 里归一）。
- **验证**：新测试 `TestNoSortNeedsATempBTree`：对五种排序执行 `EXPLAIN QUERY PLAN` 并断言。
  负样本：删掉一个索引，测试红。
- **规模**：M。

### 28. 搜索：去抖已有，索引与计数改法

- **断言**：10k 库上单次按键的查询 + 计数总耗时不增加；50k 库上不超过 X ms
  （X 由 Wave 7 的预算测试写死）。
- **改法（分两步）**：
  1. `trove_fold` 用于搜索的列增加折叠后的生成列（`text_folded`、`author_folded`），
     避免每行 16 次进 Go；LIKE 改在折叠列上，索引仍不适用于前导通配，但每条键击的
     回调数降为 0。
  2. 可选 FTS：`bookmarks_fts` 用 `tokenize='trigram'`（支持中文子串），触发器同步；
     查询长度 ≥ 2 时走 FTS，否则回退 LIKE。**单独评审**，因为它改变搜索语义（分词）。
- **验证**：SQL 基准测试（已有 `bench_test.go` 的工具）对比改造前后；语义测试：
  中文子串、大小写、引号、`%`/`_` 转义。
- **规模**：M（第 1 步）/ L（第 2 步）。

### 29. 作者过滤走索引

- **断言**：作者筛选的查询计划使用 `bookmarks_author_handle_idx` 的新折叠版本。
- **改法**：v7 增加 `author_handle_folded` 生成列 + 索引；`bookmarkFilterSQL` 改在折叠列上
  做等值；`author_name` 匹配保留 LIKE 但只在其上做二次过滤。
- **验证**：`EXPLAIN QUERY PLAN` 断言 + 现有作者筛选测试不变绿改红。
- **规模**：S。

### 30. 侧栏重建

- **断言**：10k 库、5k 待定建议时，侧栏重建 < 50ms（预算测试）。
- **改法**：`tag_suggestions` 增加折叠名生成列 + 索引，`Summary` 改成一次 GROUP BY；
  作者统计改成 SQL 里的 `GROUP BY author_handle ORDER BY count DESC LIMIT 20`。
- **验证**：预算测试 + 现有摘要测试。
- **规模**：M。

### 31. 建议面板批量读

- **断言**：一次 `Pending` 的 SQL 查询数 = 常数（≤ 3），与待定书签数无关。
- **改法**：`Bookmarks.GetMany(ctx, ids)` 并复用 `TagsByBookmark` 的批量形状。
- **验证**：用计数型仓库包装器断言调用次数。
- **规模**：S。

### 32. 导入批量化

- **断言**：导入 100k 关系的提交次数 < 1,000。
- **改法**：`AttachMany`、`AssignListMany`；apply 在一个事务里分批提交（每 1,000 行）。
- **验证**：假事务计数 + 往返一致性测试。
- **规模**：M。

### 33. 导出流式化与旧格式上限

- **断言**：导出 100k 库时峰值内存 < 64 MB；旧版 JSON 输入超过 200 MB 或 200k 条时拒绝。
- **改法**：`Export` 改为 `ExportTo(io.Writer)`，bundle writer 逐文件写入 zip 而不 `append`；
  `ParseLegacy` 前检查长度与记录数。
- **验证**：`runtime.MemStats` 断言（已有 bench 的影子）；超限输入的两条负样本。
- **规模**：M。

### 34. 媒体缓存

- **断言**：媒体内存缓存占用 ≤ 预算字节；磁盘 `used` 在覆盖写之后不增长；目录 0700、文件 0600。
- **改法**：内存缓存改为按字节的 LRU（解码后尺寸计）；`Put` 前先 `Stat` 旧文件并从 `used`
  扣减；权限与数据库一致。
- **验证**：单测直接断言 `used` 与权限位；覆盖写 100 次后 `used` 不漂移。
- **规模**：S。

---

## 8. Wave 6 — 产品缺口（35–40）

### 35. 标签层级：补界面或删模型

- **决定点**：先看真实库里有多少 tag 带 `parent_id`（一条 SQL，脱敏后记录数量）。
  - 有（哪怕是导入带来的）→ **补界面**：标签编辑器加"父标签"，侧栏子标签缩进显示，
    拖动不做；两级限制已由触发器保证。
  - 零 → **删模型**：v8 去掉 `parent_id` 与触发器，`importexport` 保留读旧字段但忽略并给警告。
- **断言**：如果是补界面：创建子标签后侧栏显示缩进且计数正确；如果是删模型：
  `PRAGMA table_info(tags)` 不再有 `parent_id`，导入旧 bundle 仍成功。
- **落定（2026-11-16）**：真实库里没有任何一个标签带 `parent_id`；但备份与导入仍会带来父子
  关系，删模型会和它们打架。所以走第三条路——**按零使用撤掉入口，数据保留**：标签管理的
  "所属主题"列与选择器、"未归主题"计数与筛选芯片、侧栏缩进全部撤掉；`parent_id`、触发器、
  备份/导入往返与合并时的层级修复原样保留。
- **规模**：M。

### 36. 搜索覆盖标签与列表名

- **断言**：输入一个标签名，挂着该标签的书签出现在结果里（带一个说明性的 chip）。
- **改法**：`bookmarkFilterSQL` 的搜索子句增加 `EXISTS (SELECT 1 FROM bookmark_tags ... JOIN tags WHERE trove_fold(tags.name) LIKE ?)`
  与列表名的 `EXISTS`；结果排序不变。
- **验证**：SQL 测试 + 一条 UI 测试（搜索结果里出现该标签 chip）。
- **规模**：S。

### 37. 筛选条件：先做媒体类型

- **断言**：面板出现"包含视频"，勾选后只剩带视频的书签；条件能存进保存视图。
- **改法**：按 `filter-conditions-analysis.md` §2 的路子：v7 增加 `media_kind` 列并由 Go 回填
  （`parse.go` 已有数据），面板改为遍历 `filterFlags` 表（一次重构，消掉手写 checkbox），
  `SavedView` 增加该字段（与第 21 条同一条迁移）。
- **验证**：回填测试（真实 payload 语料）、SQL 筛选、UI 勾选、保存视图往返。
- **规模**：M。

### 38. 自动备份

- **断言**：启动后 24 小时内至少有一份备份；备份目录最多保留 7 份；设置页可以关闭。
- **改法**：`usecase/backup` 新包：`VACUUM INTO` 到 `<data>/backups/trove-YYYYMMDD.db`，
  启动时异步执行（不阻塞窗口），失败只记状态行。设置页数据安全卡增加一行开关与"打开备份目录"。
- **验证**：假时钟推进 + 临时目录断言文件数与命名；失败路径（只读目录）给出文案。
- **规模**：M。

### 39. 可选日志

- **断言**：开启后 `logs/trove.log` 存在且 0600；关闭时不存在；日志里没有密钥、cookie、正文。
- **改法**：`internal/logging`：标准库 `slog` + 文件 handler，轮转（1 MB × 3）；
  只记事件与计数，不记内容；默认关闭。加一条读取日志的测试扫描已知秘密形状（`sk-`、`ct0=`）。
- **验证**：单测 + 一次真实运行后人工检查。
- **规模**：M。

### 40. Windows 系统代理

- **断言**：在 Windows 上读取 `ProxyEnable`/`ProxyServer` 的注册表值并生效；macOS 行为不变。
- **改法**：`internal/netproxy` 增加 `proxy_windows.go`（`golang.org/x/sys/windows/registry`）
  与 `proxy_other.go` 的空实现；解析逻辑与 `scutil` 共用一组纯函数，可跨平台测试。
- **验证**：解析函数用固定的注册表样例做表驱动测试；Windows 上真跑一次写进提交信息
  （没有机器则 `not tried`）。
- **规模**：M。

---

## 9. Wave 7 — 工程与门禁（41–45）

### 41. 性能预算进 gate

- **断言**：10k 库、默认排序、首屏查询 p95 < 预算；预算写在测试里，回退 20% 即红。
- **改法**：把两个 scaling report 改成 `TestPerformanceBudget`（10k 一个尺寸，`-short` 跳过），
  预算取 Wave 5 实测值的 2 倍作为起点，逐步收紧；gate 增加一步（时间预算 ≤ 3 分钟）。
- **验证**：故意加一次全表扫描，gate 必须红。
- **规模**：M。

### 42. CI 补三项

- **改法**：`ci.yml` 增加 `tools/selfcheck.sh --source-only`（不需要窗口的负样本）、
  `govulncheck ./...`、覆盖率汇总（先只报告，设 60% 下限，逐季度提高）。
- **验证**：在一个分支上故意破坏一个检查器 / 引入已知漏洞依赖 / 删除一个测试，CI 必须红。
- **规模**：S。

### 43. 解析器 fuzz

- **改法**：`FuzzParsePage`，语料来自 `testdata/xsource`；gate 里跑 30 秒，CI 里跑 10 秒。
- **验证**：fuzz 至少发现过一次真实 panic 才算有效；没有就记录为"未发现问题"。
- **规模**：S。

### 44. 清理遗留

- **改法**：删 `internal/netproxy/zz_dbg_test.go`，把它的输入变成 `ParseScutil` 的表驱动用例，
  断言代理 URL 与凭据的解析结果（样例里的凭据改成明显的假值）。
- **规模**：S。

### 45. 文件拆分

- **改法**：`importexport/service.go` 拆成 `export.go` / `import.go` / `restore.go`；
  `usecase/workspace/service.go` 拆成 `read.go` / `filters.go` / `views.go`；
  `i18n` 按领域拆成多个文件（同一个 `Catalog` 结构，初始化时合并）。
  纯移动 + 不改行为，单独提交（`refactor:`），gate 全绿才继续。
- **规模**：M。

---

## 10. 测试与门禁扩展

### 10.1 新增检查器（每个都配负样本）

| 编号 | 检查 | 看守什么 | 负样本 |
|---|---|---|---|
| C1 | 导出函数必须有调用方 | `DeriveMaxOutputTokens` 这类死代码 | 加一个无调用方的函数，selfcheck 红 |
| C2 | 迁移前必须有快照 | 第 17 条不被回退 | 注释掉快照调用，selfcheck 红 |
| C3 | 用户可见字符串不得是字面量 | 第 22 条 | 在 `ShowStatus` 写英文，selfcheck 红 |
| C4 | `RunOptions` 两处必须同时传 `StartCursor` | 第 3 条 | 删掉 CLI 里的字段，selfcheck 红 |
| C5 | 排序查询计划不得出现临时 B 树 | 第 27 条 | 删索引，selfcheck 红 |

### 10.2 测试矩阵

| 层次 | 怎么跑 | 覆盖 |
|---|---|---|
| 单元 | `go test ./internal/...` | 解析、规则、预算、脱敏 |
| 集成（真 SQLite） | 临时目录建库 | 迁移、恢复、批处理、索引计划 |
| 故障注入 | `testkit.FlakyRepo` / `Transport` | 回滚、续跑、限流 |
| 真实外部 | 手工，临时数据目录 | X 同步、四种 LLM provider、Keychain |
| 视觉与几何 | `tools/panes.sh` / `geometry.sh` | 归档、键盘焦点、批量栏 |
| 性能 | `TestPerformanceBudget` | 首屏、搜索、侧栏、导入 |

### 10.3 真实环境清单（每条改完打勾，未跑写 `not tried`）

- [x] X：首次全量、断点续跑、部分成功、限流 429、浏览器 cookie 成对 —— **not tried**：
  本机安全存储里没有 `x.auth_token` / `x.ct0`（只查了存在性，没有读值），`trove sync`
  在临时数据目录里真实跑到了"没有凭据"的失败路径；其余场景由 `internal/xsource` 的回放
  语料与 `syncer` 的假传输覆盖。
- [x] LLM：四个 provider 的连接测试、25 条批量、安全过滤响应、模型名错误、token 用量 ——
  **not tried**：没有存任何 provider 密钥；payload 预算、脱敏、失败账本与熔断由单测覆盖。
- [x] Keychain：正常读、锁定、凭据不存在 —— 凭据不存在在真实应用里跑过（`trove sync`
  报 "nothing stored"）；锁定与正常读用假 Keychain 覆盖。
- [x] 恢复：带负载的 bundle、中途失败、墓碑保留 —— 单测覆盖；应用内的导入没有点。
- [x] 迁移：v5 真实库 → 目标版本，检查快照与完整性 —— `TestTheUnreadColumnLeavesAnOlderLibrary`
  回放 v5 历史再 `Open`，断言快照存在、行数与 `integrity_check`；没有在窗口里用真实旧库跑。
- [x] 备份：24 小时保留策略（用假时钟） —— 保留策略由假时钟单测覆盖；真实应用里验证了
  "启动写一份、同一天再启动不重写"（`backups/trove-20261115.db`，0600）。
- [x] 性能：10k 合成库的四个数字与预算对比 —— `TestPerformanceBudget` 与
  `TestUIRenderBudget` 的实测表记在 `docs/testing.md`。

---

## 11. 全量验收判据

整轮完成时，下面每一条都必须为真：

1. 45 条各自的断言测试全部存在、全部绿，并且其中至少 40 条有对应的负样本（故意回退会红）。
2. `bash tools/gate.sh` 从 14 步变成 14 + N 步，全绿；`selfcheck.sh` 覆盖新增的 5 个检查器。
3. 两个真实截图与归档截图通过 `panes.sh` / `geometry.sh`。
4. `TestPerformanceBudget` 在 10k 库上通过；数字记录在 `docs/testing.md`。
5. 真实环境清单里没有未解释的 `not tried`。
6. `docs/` 同步：`data.md`（v7 迁移、备份）、`design.md`（归档、键盘、筛选状态）、
   `ui-surface-index.md`、`filter-conditions-analysis.md`（实施状态）。
7. `CHANGELOG.md` 有一条汇总条目，按用户能看懂的方式写，不列内部文件名。
8. 没有任何一条修复靠"删掉入口"完成；唯一的例外是第 35 条里"零使用则删模型"的分支，
   它必须在 CHANGELOG 里写明。

---

## 12. 提交策略

- 每条一个 `test:` + 一个 `fix:`，或者一个 `fix:` 里带测试；一条改动跨 wave 时按文件拆。
- 前缀沿用仓库约定：`feat:` / `fix:` / `test:` / `refactor:` / `perf:`。
- 结构性改动（第 16 条的事务化、第 45 条的拆分）单独提交、单独评审，不与行为修复混在一起。
- 不 push；不碰 `build/`、`dist/`；不跳过 gate。

---

## 附录 A：45 条 → 断言 → 规模

| # | 修复后必须为真 | 测试所在 | 规模 |
|---|---|---|---|
| 1 | 认不出游标的页不得记为全量完成 | `xsource/parse_test.go`、`syncer` | M |
| 2 | 失败页之后可续跑 | `xsource/pagination_test.go` | S |
| 3 | CLI 与窗口传同一个续跑游标 | `cmd/trove/cli_test.go` | S |
| 4 | 部分成功退出码 0 | `cli_test.go` | S |
| 5 | 第二个同步被拒，不篡改对方记录 | `syncer` 集成 | M |
| 6 | 墓碑不拖长增量同步 | `syncer` 集成 | S |
| 7 | 429 窗口被尊重且保留游标 | `pagination_test.go` | M |
| 8 | 钥匙串错误不再等于登录过期 | `syncer`、`ui` | M |
| 9 | cookie 必须来自同一 profile | `credentials/browser_test.go` | S |
| 10 | 失败批不重复付费 | `autotag` 集成 | M |
| 11 | 输出预算按批量给足 | `llm` 适配器测试 | M |
| 12 | 测试连接真的调模型 | `llm`、设置页 | S |
| 13 | 非回环 http 地址不可保存 | `preferences` | S |
| 14 | 落库错误不含任何密钥 | `llm/errors_test.go` + 集成 | S |
| 15 | 恢复保留原始负载 | `importexport` 往返 | S |
| 16 | 失败恢复回到完全一致状态 | `importexport` 故障注入 | L |
| 17 | 升级前有可打开的库快照 | `migrations_test.go` | S |
| 18 | 后台刷新不缩减列表 | `ui` | M |
| 19 | 后台刷新不裁选择 | `ui` | S |
| 20 | 归档有批量/快捷键/撤销/计数/空态 | `ui` ×5 | M |
| 21 | 保存视图包含归档 | `storage`、`ui` | M |
| 22 | 没有硬编码英文 | `i18n` + 检查器 | S |
| 23 | 输入去抖、结果可解释 | `ui` | S |
| 24 | Escape 关浮层、侧栏方向键、列表单停靠点 | `ui` ×3 | M |
| 25 | build 不写状态 | `ui` | M |
| 26 | 等待有倒计时 | `ui` | S |
| 27 | 排序无临时 B 树 | `storage` EXPLAIN | M |
| 28 | 搜索回调数降为 0（可选 FTS） | `storage` 基准 | M/L |
| 29 | 作者筛选走索引 | `storage` EXPLAIN | S |
| 30 | 侧栏重建 < 50ms | 预算测试 | M |
| 31 | 建议面板查询数为常数 | `suggestions` | S |
| 32 | 导入提交次数 < 1,000 | `importexport` | M |
| 33 | 导出峰值内存有界、旧格式有上限 | `importexport` | M |
| 34 | 媒体缓存按字节、权限正确 | `media` | S |
| 35 | 标签层级可见可建（或明确删除） | `ui` 或 `migrations` | M |
| 36 | 搜标签名能找到书签 | `storage`、`ui` | S |
| 37 | 媒体类型可筛并进保存视图 | `storage`、`ui` | M |
| 38 | 每天有备份、保留 7 份 | `usecase/backup` | M |
| 39 | 可选日志且不含秘密 | `logging` | M |
| 40 | Windows 读系统代理 | `netproxy` | M |
| 41 | 性能预算进 gate | `TestPerformanceBudget` | M |
| 42 | CI 能抓住检查器损坏/漏洞/覆盖率下降 | `.github/workflows/ci.yml` | S |
| 43 | 解析器有 fuzz | `FuzzParsePage` | S |
| 44 | 无遗留调试测试 | `netproxy` | S |
| 45 | 四个大文件拆分且行为不变 | 全量 gate | M |
