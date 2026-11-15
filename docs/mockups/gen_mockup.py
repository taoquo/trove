#!/usr/bin/env python3
"""Generates the card-layout comparison mockups at true scale.

Not part of the build. It emits one HTML per variant, all sharing the same eight
bookmarks, so the only difference on the page is the card rule.
"""
import os, html

OUT = "/Users/shuke/WorkSpace/01Code/04Git2/trove/docs/mockups"
COL = 522          # card width at a 1297 window, two columns (measured)
GAP = 22

# ---------------------------------------------------------------- media art

def art(kind, ratio):
    """A stand-in for a real picture, drawn at its true aspect ratio."""
    w = 100
    if kind == "longshot":          # a long screenshot of a post: dense text, top matters
        lines = "".join(
            f'<p style="margin:0 0 14px">{t}</p>' for t in [
                "而且这样吃，你比全世界百分之 99.99 的人吃的都健康。比美国总统、和世界首富都健康，因为他们还在喝可乐吃油炸食品。",
                "而且也不是很花钱，我本人可以控制在每天 50 元左右。",
                "钢切燕麦和冲泡燕麦的 β-葡聚糖含量、吸收利用率生物效果差别具体多少？土豆、红薯的 β-葡聚糖含量多少？",
                "多数常见水果，糖醇糖分较高（每 100g 约 27g 碳水），抗氧化密度较低。深海小型鱼类生物放大效应强于大型捕食鱼类，适合补充 omega-3。",
                "胆固醇（LDL-C）的来源，主要是饱和脂肪与精制碳水，而不是食物里的胆固醇本身。",
            ])
        return f'''<div style="position:absolute;inset:0;overflow:hidden;background:#fbfbf9;display:flex;flex-direction:column;font:11px/1.7 -apple-system,PingFang SC">
          <div style="padding:18px 20px 10px;display:flex;gap:10px;align-items:center;border-bottom:1px solid #eee">
            <div style="width:34px;height:34px;border-radius:50%;background:#d8d2c8"></div>
            <div><div style="font-weight:600;font-size:13px">营养师顾中一</div><div style="color:#8a8a8a">@guzhongyi</div></div>
          </div>
          <div style="padding:16px 20px;color:#1a1a1a;overflow:hidden">{lines}{lines}</div>
        </div>'''
    if kind == "square":            # a dashboard / chart
        bars = "".join(f'<div style="width:14px;height:{h}px;background:#c97a63;border-radius:2px"></div>'
                       for h in (38, 62, 44, 80, 55, 96, 70))
        return f'''<div style="position:absolute;inset:0;background:#101a24;color:#e8eef4;display:flex;flex-direction:column;font:11px/1.5 -apple-system,PingFang SC">
          <div style="padding:20px 22px 8px;font-size:14px;font-weight:600">Claude Code 成本分析</div>
          <div style="padding:4px 22px;color:#9fb0c0">过去 30 天 · 按任务分桶</div>
          <div style="flex:1;display:flex;align-items:flex-end;gap:10px;padding:20px 22px">{bars}</div>
          <div style="padding:0 22px 18px;color:#9fb0c0">多 Agent 编排 · 上下文复用 · 缓存命中</div>
        </div>'''
    if kind == "wide":              # a game / photo still
        return f'''<div style="position:absolute;inset:0;display:flex;background:
            radial-gradient(120% 90% at 72% 18%, #cf7a3c 0%, #7a3b22 45%, #2b1a14 100%);
            display:flex;align-items:flex-end;font:11px/1.5 -apple-system,PingFang SC;color:#f4e7dc">
          <div style="padding:16px 20px">十天，一个人，我用 AI + Unreal 5.8 制作 3A 级游戏</div>
        </div>'''
    if kind == "video":
        return f'''<div style="position:absolute;inset:0;background:
            linear-gradient(150deg,#233 0%,#0e1418 60%,#000 100%)">
          <div style="position:absolute;inset:0;display:flex;align-items:center;justify-content:center">
            <div style="width:58px;height:58px;border-radius:50%;background:rgba(25,21,20,.55);display:flex;align-items:center;justify-content:center">
              <div style="width:0;height:0;border-left:16px solid #fff;border-top:10px solid transparent;border-bottom:10px solid transparent;margin-left:5px"></div>
            </div>
          </div>
          <div style="position:absolute;left:16px;bottom:12px;color:#cfd8de;font:11px -apple-system">2:14</div>
        </div>'''
    if kind == "tiles":             # four pictures, shown as one
        return f'''<div style="position:absolute;inset:0;display:grid;grid-template-columns:1fr 1fr;gap:2px;background:#efe7df">
          {''.join(f'<div style="background:hsl({h} 28% 72%)"></div>' for h in (18, 34, 8, 44))}
        </div>'''
    return f'<div style="position:absolute;inset:0;background:#efe7df"></div>'


def cover(inner, frame_ratio, ratio, pos="center", box=COL):
    """object-fit: cover, centre or top. Sizes are explicit because the art is absolute."""
    fw, fh = box, box / frame_ratio
    if ratio < frame_ratio:          # taller than the frame: fill width, crop height
        iw, ih = fw, fw / ratio
        x, y = 0, (0 if pos == "top" else (fh - ih) / 2)
    else:                            # wider: fill height, crop sides
        ih = fh; iw = fh * ratio
        x, y = (fw - iw) / 2, 0
    return (f'<div class="frame" style="aspect-ratio:{frame_ratio}">'
            f'<div style="position:absolute;width:{iw:.2f}px;height:{ih:.2f}px;'
            f'left:{x:.2f}px;top:{y:.2f}px">{inner}</div></div>')


# ---------------------------------------------------------------- content

BOOKMARKS = [
    dict(id="t1", name="章工", handle="435hz", time="11月11日 08:53",
         text="最好的关系，就是既不控制对方也不受对方的控制。", tags=[], media=None, lines=1),
    dict(id="t2", name="阿丘", handle="axiu", time="11月10日 21:40",
         text="设计系统不是一套组件，是一套约定。约定写在文档里没人读，写在组件里没人改，"
              "最后真正生效的是评审时那句“我们这里一直这么做”。所以系统的第一产物不是 token，"
              "是能让新人对齐的默认值：默认间距、默认字号、默认在什么情况下不新增。"
              "系统失效从来不是因为不够全，是因为默认值太多。",
         tags=[], media=None, lines=5),
    dict(id="t3", name="甘棠", handle="gantang", time="11月10日 18:02",
         text="标签词表是一座花园，每周都要剪一次。", tags=["design", "typography"], media=None, lines=1),
    dict(id="t4", name="木白", handle="mubai", time="11月9日 14:20",
         text="把成本拆到任务级别之后，才知道钱花在哪。", tags=["ui"], lines=1,
         media=dict(kind="square", ratio=1.0)),
    dict(id="t5", name="小奇说", handle="xingbugengming", time="11月9日 10:15",
         text="十天，一个人，我用 AI + Unreal 5.8 制作 3A 级游戏。",
         tags=[], media=dict(kind="wide", ratio=16 / 9), lines=1),
    dict(id="t6", name="营养师顾中一", handle="guzhongyi", time="11月8日 22:31",
         text="钢切燕麦和冲泡燕麦的 β-葡聚糖含量差别到底有多大？这篇把数据摊开了讲。",
         tags=["health"], media=dict(kind="longshot", ratio=0.62), lines=2),
    dict(id="t7", name="归藏", handle="guizang", time="11月8日 09:44",
         text="一套 Agent 工作台的设计稿，四个状态。", tags=["agent"], lines=1,
         media=dict(kind="tiles", ratio=4 / 3)),
    dict(id="t8", name="老周", handle="laozhou", time="11月7日 20:08",
         text="这段讲了为什么纯 Go 的 SQLite 在 macOS 上启动反而更快。", tags=[], lines=1,
         media=dict(kind="video", ratio=16 / 9)),
]

# ---------------------------------------------------------------- card markup

def icons():
    svg = lambda p: f'<svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="#756a63" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round">{p}</svg>'
    return dict(
        comment=svg('<path d="M14 7.6c0 2.6-2.7 4.7-6 4.7-.7 0-1.4-.1-2-.2L3 13.4l.7-2.2C2.6 10.4 2 9.1 2 7.6 2 5 4.7 3 8 3s6 2 6 4.6Z"/>'),
        heart=svg('<path d="M8 13S2.5 9.7 2.5 6.4A2.9 2.9 0 0 1 8 5a2.9 2.9 0 0 1 5.5 1.4C13.5 9.7 8 13 8 13Z"/>'),
        share=svg('<circle cx="4" cy="8" r="1.7"/><circle cx="12" cy="3.6" r="1.7"/><circle cx="12" cy="12.4" r="1.7"/><path d="M5.6 7.2 10.4 4.4M5.6 8.8l4.8 2.8"/>'),
        external=svg('<path d="M9.5 2.5H13.5V6.5M13 3 7.5 8.5"/><path d="M12 9.5v3a1 1 0 0 1-1 1H3.5a1 1 0 0 1-1-1V5a1 1 0 0 1 1-1h3"/>'),
    )

ICONS = icons()

def card(bm, variant):
    tags = bm["tags"]
    t = html.escape(bm["text"])
    media = bm["media"]
    frame = ""
    if media:
        r = media["ratio"]
        if variant == "A":
            frame = cover(art(media["kind"], r), 16 / 9, r, "center")
        elif variant in ("B", "C"):
            if variant == "C":
                pass
            else:
                cap = min(COL * 0.5, 240)
                frame = cover(art(media["kind"], r), COL / cap, r, "top")
        elif variant == "D":
            fr = min(max(r, 4 / 3), 16 / 9)
            frame = cover(art(media["kind"], r), fr, r, "center")

    clamp = {"A": 5, "B": 3, "C": 3, "D": 3}[variant]

    taghtml = "".join(f'<span class="tag">{html.escape(x)}</span>' for x in tags)
    if variant in ("B", "C", "D"):
        head_extra = f'<div class="tags">{taghtml}</div>' if taghtml else ""
    else:
        head_extra = ""

    body = f'''
      <div class="head">
        <div class="who">
          <div class="name">{html.escape(bm["name"])}</div>
          <div class="sub"><span class="handle">@{bm["handle"]}</span>{head_extra}</div>
        </div>
        <div class="time">{bm["time"]}</div>
      </div>
      <div class="copy" style="-webkit-line-clamp:{clamp}">{t}</div>
      {"" if variant in ("B", "C", "D") else (f'<div class="tags sep">{taghtml}</div>' if taghtml else "")}
      <div class="actions">
        <div class="lead">{ICONS["comment"]}{ICONS["heart"]}{ICONS["share"]}</div>
        {ICONS["external"]}
      </div>'''

    initial = bm["name"][:2]
    avatar = f'<div class="avatar">{initial}</div>'

    if variant == "C":
        thumb = ""
        if media:
            # draw the picture at card width, then shrink it, so a thumbnail of a
            # dense screenshot still reads as a screenshot instead of as body text
            inner = cover(art(media["kind"], media["ratio"]), 1.0, media["ratio"], "top", box=COL)
            thumb = (f'<div class="thumb"><div style="position:absolute;width:{COL}px;height:{COL}px;'
                     f'transform:scale({120 / COL:.5f});transform-origin:0 0">{inner}</div></div>')
        return f'''<article class="card C">
          <div class="toggle"></div>
          <div class="cbody">{thumb}<div class="ccontent">{body}</div></div>
        </article>'''
    if variant == "A":
        return f'''<article class="card A">
          <div class="toggle"></div>
          {frame}
          <div class="body"><div class="main">{avatar}<div class="col">{body}</div></div></div>
        </article>'''
    return f'''<article class="card {variant}">
      <div class="toggle"></div>
      {frame}
      <div class="body"><div class="main{'' if frame else ' inset'}">{avatar}<div class="col">{body}</div></div></div>
    </article>'''


# ---------------------------------------------------------------- heights

LH = 18.85                      # 13px * 1.45
HEAD = 43                      # name + handle
PAD = 28                       # body padding, top + bottom
ACTIONS = 12 + 32 + 1          # padding-top + icons + hairline

def height(bm, variant):
    lines = bm.get("lines", 1)
    m = bm["media"]
    if variant == "C":
        inner = HEAD + lines * LH + 14 + ACTIONS
        return max(160, PAD + max(120 if m else 0, inner))
    copy = max(lines, 2.6) * LH if variant == "A" else lines * LH
    h = PAD + HEAD + 14 + copy + ACTIONS
    if variant == "A":
        if bm["tags"]:
            h += 28
        h = max(200, h)
    if m:
        if variant == "A":
            h += COL * 9 / 16
        elif variant == "B":
            h += min(COL * 0.5, 240)
        elif variant == "D":
            fr = min(max(m["ratio"], 4 / 3), 16 / 9)
            h += COL / fr
    return h


def distribute(items, variant):
    if variant == "A":
        n = len(items)
        split = (n + 1) // 2
        return items[:split], items[split:]
    left, right, hl, hr = [], [], 0, 0
    for bm in items:
        h = height(bm, variant)
        if hl <= hr:
            left.append(bm); hl += h + GAP
        else:
            right.append(bm); hr += h + GAP
    return left, right


STYLE = '''
*{box-sizing:border-box}
body{margin:0;background:#f6f0ea;font-family:-apple-system,"PingFang SC","Helvetica Neue",sans-serif;
     -webkit-font-smoothing:antialiased;color:#191514}
.wrap{padding:34px 40px 60px}
.h{font-size:19px;font-weight:600;letter-spacing:-.01em}
.note{font-size:12.5px;color:#756a63;margin-top:6px;line-height:1.55}
.grid{display:flex;gap:22px;align-items:flex-start;margin-top:20px}
.coln{display:flex;flex-direction:column;gap:22px;width:522px}

.card{background:#fbf7f3;border:1px solid #e6dacd;border-radius:10px;overflow:hidden;
      display:flex;flex-direction:column;position:relative}
.card.A{min-height:200px}
.frame{overflow:hidden;position:relative;background:#efe7df;border-bottom:1px solid #eadfd5;flex:none}
.frame>div{display:block}
.toggle{position:absolute;left:18px;top:18px;width:20px;height:20px;border-radius:5px;
        border:1px solid rgba(25,21,20,.28);z-index:3}
.card.A .toggle,.card.B .toggle,.card.D .toggle{border-color:rgba(255,255,255,.55);
        background:rgba(251,247,243,.82);box-shadow:0 2px 7px rgba(25,21,20,.08)}
.body{padding:14px;display:flex;flex-direction:column;flex:1}
.main{display:flex;gap:10px;flex:1}
.main.inset{padding-left:30px}
.avatar{width:28px;height:28px;border-radius:50%;background:#efe7df;flex:none;
        display:flex;align-items:center;justify-content:center;font-size:10px;font-weight:700;color:#191514}
.col{display:flex;flex-direction:column;gap:14px;flex:1;min-width:0}
.head{display:flex;gap:12px;justify-content:space-between;align-items:flex-start}
.who{min-width:0}
.name{font-size:13px;font-weight:500;line-height:1.38}
.sub{display:flex;align-items:center;gap:8px;margin-top:3px;font-size:11.5px;color:#5a4a43}
.time{font-size:11.5px;color:#756a63;white-space:nowrap;line-height:1.38;flex:none}
.copy{font-size:13px;line-height:1.45;color:#191514;flex:1;min-height:64.5px;
      display:-webkit-box;-webkit-box-orient:vertical;overflow:hidden}
.B .copy,.C .copy,.D .copy{min-height:28px}
.tags{display:flex;gap:8px;flex-wrap:wrap}
.tags.sep{margin-top:0}
.tag{font-size:11.5px;font-weight:500;color:#5a4a43;background:#efe7df;border-radius:999px;
     padding:0 9px;height:24px;display:inline-flex;align-items:center}
.sub .tag{height:20px;font-size:11px;padding:0 8px}
.actions{border-top:1px solid #eadfd5;padding-top:12px;display:flex;justify-content:space-between;
         align-items:center;margin-top:auto}
.lead{display:flex;gap:12px;align-items:center}
/* C: thumbnail beside the text */
.card.C .cbody{display:flex;gap:14px;padding:14px 14px 14px 44px;flex:1}
.card.C .thumb{width:120px;height:120px;border-radius:8px;overflow:hidden;flex:none;background:#efe7df;
               position:relative}
.card.C .ccontent{flex:1;min-width:0;display:flex;flex-direction:column;gap:14px}
.card.C .main{padding:0;gap:0}
.card.C .avatar{display:none}
.card.C .copy{min-height:0}
.card.C .actions{padding-top:10px}
.card.C{min-height:160px}
'''

def page(title, sub, variant):
    left, right = distribute(BOOKMARKS, variant)
    hl = sum(height(b, variant) for b in left) + GAP * (len(left) - 1)
    hr = sum(height(b, variant) for b in right) + GAP * (len(right) - 1)
    body = "".join(card(bm, variant) for bm in left)
    body2 = "".join(card(bm, variant) for bm in right)
    return f'''<!doctype html><meta charset="utf-8"><style>{STYLE}</style>
<div class="wrap">
  <div class="h">{title}</div>
  <div class="note">{sub}</div>
  <div class="grid">
    <div class="coln">{body}</div>
    <div class="coln">{body2}</div>
  </div>
</div>''', hl, hr


VARIANTS = {
 "A": ("现状", "媒体 16:9 居中裁切 · 正文 5 行 · 标签单独一行 · 最小高度 200 · 两栏按数量切分"),
 "B": ("推荐:2:1 媒体带 + 3 行正文", "媒体带 2:1(上限 240)顶部对齐 · 正文 3 行 · 标签并入 handle 行 · 两栏按高度分配"),
 "C": ("备选:并排缩略图", "120 方形缩略图在左 · 正文 3 行 · 标签并入 handle 行 · 高度按文字走 · 两栏按高度分配"),
 "D": ("对照:媒体带按图片真实比例(夹在 4:3–16:9)", "媒体带跟图片走 · 正文 3 行 · 标签并入 handle 行 · 两栏按高度分配"),
}

CMP = ["t1", "t2", "t4", "t6"]

CMP_STYLE = '''
.cmp{display:flex;gap:24px;align-items:flex-start;margin-top:26px}
.cc{width:522px;flex:none;display:flex;flex-direction:column;gap:22px}
.ch{font:600 13px/1 ui-monospace,SFMono-Regular,Menlo,monospace;color:#b4553f;letter-spacing:.08em}
.ct{font-size:16px;font-weight:600;margin-top:9px;letter-spacing:-.01em}
.cn{font-size:12px;color:#756a63;line-height:1.5;margin-top:5px;height:38px}
.stats{font-size:12px;color:#5a4a43;line-height:1.7;border-top:1px solid #e6dacd;padding-top:10px;
       margin-top:auto}
.stats b{font-weight:600;color:#191514}
'''


def compare_page():
    byid = {b["id"]: b for b in BOOKMARKS}
    cols, heights = [], {}
    for k in "ABCD":
        items = [byid[i] for i in CMP]
        hs = [height(b, k) for b in items]
        heights[k] = hs
        cards = "".join(card(b, k) for b in items)
        title, sub = VARIANTS[k]
        cols.append(f'''<div class="cc">
          <div class="ch">{k}</div><div class="ct">{title}</div><div class="cn">{sub}</div>
          {cards}
          <div class="stats">{stat_line(k, hs)}</div>
        </div>''')
    return f'''<!doctype html><meta charset="utf-8"><style>{STYLE}{CMP_STYLE}</style>
<div class="wrap">
  <div class="h">同一组书签,四种卡片规则</div>
  <div class="note">四列同一个宽度(522)、同一份内容,只换卡片的规则。从上到下:一条短文字、一条长文字、一条方形图、一条长截图。<br>
  另外四张整页图是同样的规则排了 8 条书签,两栏。</div>
  <div class="cmp">{''.join(cols)}</div>
</div>'''


def stat_line(k, hs):
    l, r = distribute(BOOKMARKS, k)
    hl = sum(height(b, k) for b in l) + GAP * (len(l) - 1)
    hr = sum(height(b, k) for b in r) + GAP * (len(r) - 1)
    return (f'这四条:<b>{hs[0]} / {hs[1]} / {hs[2]} / {hs[3]}</b><br>'
            f'8 条两栏:<b>差 {abs(hl - hr):.0f}</b><br>'
            f'高/矮比 <b>{hs[2] / hs[0]:.1f}×</b>')


if __name__ == "__main__":
    os.makedirs(OUT, exist_ok=True)
    for k, (title, sub) in VARIANTS.items():
        doc, hl, hr = page(title, sub, k)
        open(f"{OUT}/variant-{k}.html", "w").write(doc)
        print(f"{k}: 左栏 {hl:.0f}  右栏 {hr:.0f}  差 {abs(hl-hr):.0f}")
    open(f"{OUT}/compare.html", "w").write(compare_page())
    print("compare written")
