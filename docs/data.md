# 数据、备份与凭据

## SQLite

数据库在应用数据目录，打开时按顺序应用 `internal/storage/sqlite/migrations.go` 中的迁移。每个
迁移在一个事务里执行并记录版本，失败不会留下半个 schema。

| 表 | 内容 |
|---|---|
| `bookmarks` | 书签本体与在 X 上的时间、作者、媒体、长文标记，以及读者自己的两个状态 `read_at` / `archived_at` |
| `raw_payloads` | 最近一次同步拿到的原始 JSON，用于重放与排查 |
| `deleted_bookmarks` | 本地删除的墓碑，同步导入时据此跳过 |
| `tags` / `bookmark_tags` | 标签树与书签-标签关系；关系的 `source` 记录人工、规则或模型 |
| `tag_suggestions` | 待确认的标签建议及其状态 |
| `llm_analysis` | 每条书签的模型分析结果与哈希 |
| `saved_views` | 保存的筛选视图 |
| `lists` / `bookmark_lists` | 列表与书签归属；Inbox 是系统行 |
| `sync_runs` | 每次同步的状态、计数与错误 |
| `app_settings` | 界面与自动打标的本地设置 |

`read_at` 是第一次在这台机器上打开详情的时刻，`archived_at` 是放进归档的时刻；NULL 分别是
"未读"和"在用"。这两列只属于读者，不属于 X 的负载：它们在插入时写入、在
`ON CONFLICT DO UPDATE` 的更新列表里刻意缺席，所以一次同步不会把已归档的书签拉回来，也不会
把读过的重新标成未读；而恢复进一个空库时，备份里的状态会跟着回来。旧版 JSON 导出把它们作为
可选的 `readAt` / `archivedAt` 携带，旧文件没有这两个字段，读出来就是未读、未归档。

名称排序在 Go 里用 `domain.CompareNaturalNames`，不要交给 SQL 的字节比较；`tools/namesort.py`
会检查这一点。

## 备份

### `.trovebundle`（原生）

ZIP 容器，`manifest.json` 声明格式与版本，其余文件按白名单读取和写入：书签、标签、关系、
列表、建议、分析、保存视图、分类规则、设置、同步记录与原始负载。写入是白名单：新表默认不
导出。读取按敌意输入处理：条目名、声明大小、压缩比和总量都在读取字节之前校验，任何失败都
不会触碰已有数据。

### 旧版 `exportVersion: 2` JSON（兼容）

`internal/importexport/legacyformat.go` 负责这种扁平 JSON 的导入与导出。它保留是因为已有用户
手里有这种备份；导入会先完整校验再写入，导出只写当前应用真正拥有的字段，端点与任何凭据都
不离开本机。这条边界是刻意保留的，不要和新格式混在一起。

## 凭据

X 会话是 `auth_token` + `ct0`。来源有两个：

- 从已登录的浏览器配置读取（`internal/credentials/browser.go`，sweetcookie）；
- 用户粘贴 Cookie header。

`trove auth` 是第一条的命令行版本（`credentials.Store.SignInFromBrowser`）：先打开 X 的登录页，
再轮询浏览器直到出现会话，然后写进同一个安全存储。它读写的是同一组账户，所以窗口与命令行
互相看得见对方的登录结果；系统拒绝读取浏览器配置目录时立刻失败，不空等。

两条路都写进系统安全存储的同一组账户：macOS 上是 Keychain，Windows 上是 Credential Manager。
服务名/目标前缀是 `com.taoquo.trove`，账户按
`keychain.Account*` 命名；`credentials.Accounts` 是重置本地数据时清理全部密钥的唯一清单。

模型密钥按 provider 分账户保存；旧的单 provider 密钥有单独的 legacy 账户，只对当前 provider
生效。

## 红线

密钥和 cookie 不进入截图、日志、导出或诊断包。安全存储只查存在性，不把值读进输出。
