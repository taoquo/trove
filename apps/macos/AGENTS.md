# AGENTS.md — 怎么在这个仓库里干活

这是一页契约，不是设计文档。仓库架构看根目录 `CLAUDE.md`；视觉与行为的参考看
`docs/loop/reference.md`；当前做什么看 `docs/loop/board.md`。

**每次会话的开场顺序**（不要从最后一条消息开工）：

1. `docs/loop/board.md` — 找到唯一一行 `doing`，或从 `Next` 取第一行；
2. `docs/loop/grammar.md` — 读这个切片涉及的那一层；
3. `docs/loop/lessons.md` — 读标着 `Check` 的规则；
4. 本文件 — 车道、DoR、DoD、证据。

---

## 1. 两条车道

| 车道 | 用于 | 流程 | 上限 |
|---|---|---|---|
| **slice** | 一条用户链路的一个状态 | DoR → 红 → 绿 → gate → 证据 → 人审 → 更板 | 一个状态 1 天；一条链路一片 3 天 |
| **fix** | 单点缺陷、文案、一个 token | 红测试 → gate → 三行证据 | 半天 |

超时只拆小或缩范围，不延期；范围写回板子。

**P0**（崩溃、数据损坏、泄密）可以打断任何切片，但必须在板上记录"从哪一片切走、切回哪一片"。
其他一切进 `Inbox`，当前切片收尾后再取。用户明确说"先做这个"时，按 P0 处理并记录。

一个 session 可以跨多个切片，但**每个切片必须在下一个开始前 commit 并更板**。压缩或交班前，
先把板子的当前行、证据、剩余风险写清。

## 2. 开始前（DoR）

- [ ] 链路和状态写明；Profile 已选：`behavior` / `visual` / `data`
- [ ] 期望行为写成一句可断言的话（用户看到什么，而不是代码里改了什么）
- [ ] 验收命令存在，哪怕它现在会红
- [ ] visual：参考值和出处（`src/...:line` 或 `app.css:line`）已写进 `grammar.md`
- [ ] data/sync：fixture 或真实运行的方式已确定

缺一项不开工。

## 3. 收尾（DoD）

- [ ] 红输出已记录：旧代码上失败的那条命令和失败信息
- [ ] 新代码上绿
- [ ] 同类已 grep；只有一个时，在证据里写为什么
- [ ] `gate.sh` 全绿，且跑在要提交的那棵树上；跑完再改代码必须重跑
- [ ] 碰 X / LLM / Keychain 的切片真跑一次；没跑就写 `not tried` 和原因
- [ ] visual：固定环境捕获（`XBM_*`）+ 人审
- [ ] commit body 带证据块
- [ ] `board.md` 已更新；`done` 必须有证据
- [ ] slice 车道写复盘三问；fix 车道可跳过

## 4. 证据块

```bash
source ../../.toolchain/env.sh
cd apps/macos
bash tools/evidence.sh F6-1 --gate "14/14 in 4m12s"
```

它填好 tree、dirty、时间，其余按实情补：

```text
Slice: F6-1
Tree: abc1234 (clean)
Gate: 14/14 in 4m12s
Red: go test ./internal/ui -run TestX -> FAIL (旧代码上的失败)
Green: same -> ok
Real run: <命令 + 状态 + 时间> / not tried: <原因>
Capture: <路径> (build <sha>)
Broken by hand: <改了哪一行让测试变红> / none
Not tried: <没跑什么、为什么> / none
```

## 5. 红线

- **密钥、cookie 不进截图、日志、commit。** Keychain 只查存在性，不读值、不打印。
- **不用 `open` 启动应用。** 直接跑 `build/darwin-arm64/X Bookmark Manager.app/Contents/MacOS/X Bookmark Manager`；
  有实例在跑时 gate 会拒绝。
- **不 push。** 提交即可，远端由用户决定。
- **不改生成物**（`build/`、`build/chrome-mv3`）。
- **不跳过 gate。** 时间不够只能缩范围。

## 6. 流程自身

- 每条教训进 `docs/loop/lessons.md`；同一规则在 3 个不同日期出现 → 升级成检查或删掉。
- 每条检查必须能变红；没有负样本的检查不进 gate。
- 三片之后回答三个问题：返工轮次降了吗？逃逸缺陷降了吗？新 session 只读文档能完成一片吗？
  有一个"否"就砍流程，不加流程。
- 新增产物必须有消费者和检查；没有就删。
