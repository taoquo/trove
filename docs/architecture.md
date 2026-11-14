# 架构

trove 是一个 macOS 与 Windows 桌面应用，用 Go 1.27 和 [MyGo](https://github.com/egoist/mygo)
绘制原生界面：没有 webview、HTML 或 JavaScript。数据在本地 SQLite，X 凭据和模型密钥在系统
安全存储里（macOS Keychain / Windows Credential Manager）。

## 分层

```
domain    纯规则，不依赖任何其他层
ports     usecase 需要的接口（仓库、书签源、时钟）
usecase   workspace / tagging / syncer / suggestions / settings
adapter   storage/sqlite、xsource、llm、credentials、media、bundle、importexport
ui        MyGo 工作区，只和 usecase 说话
```

`cmd/trove/main.go:1` 是唯一的组合根：只有它选择具体实现并把对象接起来。其他包通过构造函数接收接口，
测试可以替换成内存实现。

| 目录 | 职责 |
|---|---|
| `internal/domain` | 书签、标签、关系图、排序、分页、时间与文本规范化。无 I/O |
| `internal/ports` | 仓库与外部服务的接口定义 |
| `internal/usecase` | 用例编排：工作区读取、打标、同步、建议、设置 |
| `internal/storage/sqlite` | modernc.org/sqlite 仓库实现与迁移 |
| `internal/xsource` | X 的 GraphQL 客户端、查询目录、错误分类 |
| `internal/llm` | 模型提供方目录、适配器、错误与预算 |
| `internal/autotag` | 自动打标流水线、启发式、预算与进度 |
| `internal/credentials` | Keychain 存储、浏览器 cookie 读取、凭据规则 |
| `internal/media` | 媒体下载与磁盘缓存 |
| `internal/bundle` | `.trovebundle` 备份读写 |
| `internal/importexport` | 旧版 `exportVersion: 2` JSON 的导入与导出 |
| `internal/ui` | MyGo 界面、状态、事件处理与绘制 |
| `internal/i18n` | 中英文文案目录与完整性检查 |

## 一次读取

`internal/ui` 的状态只保存数据。`View` 可以因为一次事件被重建多次，因此它必须无副作用：
不做 I/O，不保存元素或句柄。耗时工作走事件处理器或受保护的后台任务，完成后通过
`Window.Update` 把结果发布回窗口线程。键盘命令用 `c.OnShortcut` / `handle.OnShortcut`。

界面测试用 `ui.NewTester`，不开窗口，是复现布局和状态缺陷的常规方式。所有用户可见字符串
走 `internal/i18n`；启动时 `AssertComplete()` 会检查两种语言是否都有对应文案。

## 一次同步

1. `internal/ui` 触发 `usecase/syncer`；
2. syncer 通过 `ports` 请求 `xsource` 拉取书签页；
3. 记录写入 `internal/storage/sqlite`，原始负载留在 `raw_payloads`；
4. 进度和结果通过 `Window.Update` 回到界面；
5. 失败的运行保留在 `sync_runs`，下次启动据此决定是否续跑。

同步不会把本地已删除的书签带回来：删除会写进 `deleted_bookmarks`，导入时按墓碑跳过。

## 设计契约

视觉与交互的契约在 `docs/design.md`：token、组件和状态都在那里，且都是本项目的决定。
新增或改名绘制方法必须在同一个改动里更新 `docs/ui-surface-index.md`。

## 检查器

检查器是构建的一部分，不是可选工具：

- `tools/check-text.sh`：用户可见字符串必须走翻译目录；
- `tools/check-wiring.sh`：接口方法必须可达、字段必须被读取，并跑 surfaces / toolrefs /
  namesort / bigfiles / 颜色与几何检查；
- `tools/selfcheck.sh`：逐个破坏检查器看守的东西，证明它仍会变红；
- `tools/panes.sh`、`tools/geometry.sh`：在真实构建的截图上验证三栏边界与几何不变量。

完整 gate 见 `docs/testing.md`。
