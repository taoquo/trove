# 对照装置（parity rig）

**为什么有它**：六周的 UI 工作用的是"手拍截图"。手拍的图不可复现、不可测量，
而且早就和 `src/` 漂移了（侧栏 256 vs 280、底色 #fbf7f4 vs #f3ebe4）。
这套装置把"源长什么样"变成一条命令的输出：同尺寸、同数据、截图 + 每个节点的盒模型。

## 用法

```bash
npm run build                                  # 需要 build/chrome-mv3
bash tools/parity/render.sh                    # 五个状态，1297×808，2x
bash tools/parity/render.sh workspace detail   # 指定状态
```

输出在 `docs/parity/`（随 `docs/` 一起 gitignore）：

- `source-<state>-<width>.png` —— 源的真机渲染
- `source-<state>-<width>.json` —— 每个可见节点的 `tag/cls/tid/x/y/w/h`

## 它怎么工作

`render-source.mjs` 起一个静态服务，用 headless Chrome 打开 `harness.html`，
`harness.html` 做三件事：

1. 用 `fixture.json` 播种 IndexedDB（书签、标签、建议、同步记录）；
2. 用内存桩提供 `chrome.storage.local`（locale / 主题 / 侧栏宽），
   `chrome.runtime.sendMessage` 返回空 —— 渲染路径不需要别的扩展 API；
3. `import("/build/chrome-mv3/options.js")`，等结果网格出现，按 `?state=` 触发
   详情/设置/列表，再把整棵树的 `getBoundingClientRect()` 写进 `#__boxes`。

没有 npm 依赖：Node 24 的全局 `WebSocket` 直接驱动 DevTools 协议。

## 改基准数据

只改 `fixture.json`。两侧必须用**同一份**：原生侧（待做）通过 `XBM_SEED` 读同一个文件，
否则对照又变成"两张不同的图比谁好看"。

## 已知边界

- 只比几何与令牌色，不比字形像素（`Iowan Old Style` 与系统字体渲染不同）。
- 状态钩子是点击驱动的：详情点第一张卡片，设置点页脚齿轮，列表点视图切换。
  新状态加在 `harness.html` 的 `state ===` 分支里。
