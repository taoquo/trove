# 参考

实现期只读这里钉住的版本。产品环可以重钉，但必须在 `board.md` 的 `Decisions` 写日期和理由。

## Magpie（视觉与交互目标）

| 项 | 值 |
|---|---|
| 仓库 | `https://github.com/yetone/magpie` |
| 许可 | MIT（`LICENSE`，Copyright 2026 yetone） |
| 钉住的 commit | `dca338f3a348ac8e1a937ebde418e9957ac5c40e`（`dca338f`，2026-11-07） |
| 本地 clone | `/tmp/magpie-src` |
| 站点 | `https://usemagpie.ai/`（宣传页；行为以源码为准） |

抓取：

```bash
git clone https://github.com/yetone/magpie /tmp/magpie-src
git -C /tmp/magpie-src checkout dca338f3a348ac8e1a937ebde418e9957ac5c40e
```

**为什么不 vendored 整个站点**：CSS/JS/HTML 加起来很大，而且会随上游移动；需要的是"同一个 commit +
一条命令 + 哈希"，不是把产物塞进仓库。截图也只留在本地 `docs/screenshots`，证据记的是捕获命令。

已在这个 pin 上读过的设计记录（本地、未跟踪）：

- `docs/magpie-ui-audit.md`（第 532 轮）— 逐项差距与优先级；
- `docs/magpie-visual-spec.md`（第 531 轮）— token 与组件映射；
- `docs/magpie-p0-p1-plan.md`、`docs/magpie-p2-plan.md` — 已落地项。

## 扩展（文案与行为规格）

`src/` 是同一产品的参考实现：UI、文案、存储格式、同步协议、自动打标行为都以它为准
（见根目录 `CLAUDE.md`）。引用必须带 `src/...:line`。

## 捕获环境（视觉证据）

固定值，证据里写清楚：

```bash
XBM_DATA_DIR=<临时 fixture 目录>
XBM_HEADLESS=1
XBM_CAPTURE=<输出 png>
XBM_CAPTURE_STATE=<workspace|detail|settings|tag-autotag|...>
XBM_CAPTURE_WIDTH=1297
XBM_CAPTURE_LOCALE=<zh-CN|en>
```

构建后直接跑 bundle 二进制，不用 `open`。
