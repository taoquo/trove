#!/usr/bin/env python3
"""Draws the detail dialog at true size, under four layouts.

Not part of the build. Every number here comes from internal/ui (detailmetrics.go,
metrics.go): a 780-wide dialog, a 22/26/18 header, 24/26 post padding, 13px/1.6 body,
6px media grid, 12px media radius.

    python3 gen_detail_mockup.py     ->  docs/mockups/detail-*.html
"""
import html
import os

OUT = os.path.dirname(os.path.abspath(__file__))

W = 780                    # DetailCardMaxWidth
MAXH = 720                 # DetailCardMaxHeight
PADX = 26                  # DetailHeroPadSide / post padding
CW = W - 2 * PADX          # 728, the reading width
HDR = 22 + 30 + 18         # header padding + avatar + divider line above
BODY = 13
BODY_LH = 20.8             # 13 * 1.6
MEDIA_GAP = 14             # DetailPostGap
GRID_GAP = 6               # DetailMediaGridGap
BAND = 420                 # the capped media band (proposal)
MEASURE = 600              # the capped reading measure (proposal)

# ---------------------------------------------------------------- pictures

def art(kind):
    """A stand-in for a real picture, drawn to fill its box."""
    if kind == "phone":             # a portrait screenshot: header plus rows
        rows = "".join(
            f'<div style="display:flex;justify-content:space-between;align-items:center;'
            f'padding:13px 16px;border-bottom:1px solid #ece7e1">'
            f'<span style="font-size:12px">{t}</span>'
            f'<span style="font-size:11px;color:#9a8f88">{v}</span></div>'
            for t, v in [("通知", "开"), ("专注模式", "关"), ("屏幕使用时间", "3 小时 12 分"),
                         ("外观", "浅色"), ("语言与地区", "简体中文"), ("存储", "18.4 GB 可用"),
                         ("蜂窝网络", "开"), ("蓝牙", "开"), ("个人热点", "关"), ("通用", "›"),
                         ("辅助功能", "›"), ("电池", "78%"), ("相机", "›"), ("邮件", "2 个账户"),
                         ("日历", "›"), ("天气", "青岛"), ("地图", "›"), ("隐私与安全", "›"),
                         ("关于本机", "›")])
        return f'''<div class="art" style="background:#fbfbf9;color:#1a1a1a;
            font:12px/1.5 -apple-system,'PingFang SC'">
          <div style="padding:16px 16px 12px;border-bottom:1px solid #ece7e1">
            <div style="font-weight:600;font-size:15px">设置</div>
            <div style="color:#9a8f88;font-size:11px;margin-top:3px">iPhone · iOS 19.2</div>
          </div>{rows}</div>'''
    if kind == "dash":              # a square dashboard
        bars = "".join(f'<div style="width:16px;height:{h}px;background:#c97a63;border-radius:2px"></div>'
                       for h in (44, 70, 52, 92, 63, 110, 80))
        return f'''<div class="art" style="background:#101a24;color:#e8eef4;
            display:flex;flex-direction:column;font:11px/1.5 -apple-system,'PingFang SC'">
          <div style="padding:18px 20px 6px;font-size:14px;font-weight:600">Claude Code 成本分析</div>
          <div style="padding:0 20px;color:#9fb0c0">过去 30 天 · 按任务分桶</div>
          <div style="flex:1;display:flex;align-items:flex-end;gap:10px;padding:14px 20px">{bars}</div>
          <div style="padding:0 20px 16px;color:#9fb0c0">多 Agent 编排 · 上下文复用</div>
        </div>'''
    if kind == "game":              # a wide still
        return f'''<div class="art" style="display:flex;align-items:flex-end;background:
            radial-gradient(120% 90% at 72% 18%,#cf7a3c 0%,#7a3b22 45%,#2b1a14 100%);
            font:11px/1.5 -apple-system,'PingFang SC';color:#f4e7dc">
          <div style="padding:14px 18px">十天，一个人，我用 AI + Unreal 5.8 制作 3A 级游戏</div>
        </div>'''
    if kind == "photo":             # a portrait photo: sky, ridge, sun
        return f'''<div class="art" style="background:
            linear-gradient(180deg,#d3dee3 0%,#aec2c8 34%,#7b939b 56%,#4a606b 100%)">
          <div style="position:absolute;left:0;right:0;bottom:0;height:52%;background:#33474f;
            clip-path:polygon(0 34%,18% 18%,33% 40%,52% 10%,72% 36%,88% 22%,100% 32%,100% 100%,0 100%)"></div>
          <div style="position:absolute;right:16%;top:13%;width:64px;height:64px;border-radius:50%;
            background:rgba(255,246,232,.85)"></div>
          <div style="position:absolute;left:14px;bottom:12px;color:#eef3f5;font:11px -apple-system">
            青岛 · 11月</div></div>'''
    if kind == "film":              # a video poster: dark, one warm source
        return f'''<div class="art" style="background:
            linear-gradient(158deg,#22333c 0%,#0d1418 58%,#04070a 100%)">
          <div style="position:absolute;left:8%;top:10%;width:44%;height:58%;background:
            radial-gradient(closest-side,rgba(233,192,142,.5),rgba(233,192,142,0))"></div>
          <div style="position:absolute;left:0;right:0;bottom:0;height:24%;background:
            linear-gradient(to top,rgba(0,0,0,.55),transparent)"></div>
        </div>'''
    if kind == "chart":             # a 4:3 chart
        bars = "".join(f'<div style="flex:1;height:{h}%;background:#7d9a8b;border-radius:3px 3px 0 0"></div>'
                       for h in (44, 61, 38, 78, 55, 88))
        return f'''<div class="art" style="background:#f4f1ea;display:flex;flex-direction:column;
            font:11px/1.5 -apple-system,'PingFang SC';color:#3a332e">
          <div style="padding:14px 16px 4px;font-size:12px;font-weight:600">每周新增书签</div>
          <div style="flex:1;display:flex;align-items:flex-end;gap:9px;padding:10px 16px 14px">{bars}</div>
        </div>'''
    return '<div class="art" style="background:#efe7df"></div>'


DESIGN = 728       # pictures are drawn at the reading width, then scaled to their box


PLAY = 58          # DetailPlaySize
GLYPH = 22         # the triangle inside it


def play_mark():
    """detailPlayMark: a 58-point disc, fixed, never scaled with the picture."""
    return (f'<div style="position:absolute;inset:0;display:flex;align-items:center;'
            f'justify-content:center"><div style="width:{PLAY}px;height:{PLAY}px;'
            f'border-radius:50%;background:rgba(25,21,20,.55);display:flex;'
            f'align-items:center;justify-content:center">'
            f'<div style="width:0;height:0;margin-left:{GLYPH * 0.18:.1f}px;'
            f'border-left:{GLYPH * 0.5:.1f}px solid #fff;'
            f'border-top:{GLYPH * 0.5:.1f}px solid transparent;'
            f'border-bottom:{GLYPH * 0.5:.1f}px solid transparent"></div></div></div>')


def cover(kind, boxw, boxh, ratio, pos="center", play=False):
    """object-fit: cover, centred or top, in explicit pixels.

    The art is always laid out at DESIGN wide and scaled down to the box, so a 129-point
    tile holds a small picture of the same screen rather than a full-size screen in a hole.
    """
    s = boxw / DESIGN
    fw, fh = DESIGN, DESIGN * boxh / boxw       # the frame, at drawing size
    if ratio < fw / fh:                         # taller than the frame: crop the height
        iw, ih = fw, fw / ratio
        x, y = 0, (0 if pos == "top" else (fh - ih) / 2)
    else:                                       # wider: crop the sides
        iw, ih = fh * ratio, fh
        x, y = (fw - iw) / 2, 0
    inner = (f'<div style="position:absolute;width:{iw:.2f}px;height:{ih:.2f}px;'
             f'left:{x:.2f}px;top:{y:.2f}px">{art(kind)}</div>')
    if abs(s - 1) < 1e-6:
        scaled = inner
    else:
        scaled = (f'<div style="position:absolute;width:{fw}px;height:{fh:.2f}px;'
                  f'transform:scale({s:.5f});transform-origin:0 0">{inner}</div>')
    return (f'<div class="frame" style="width:{boxw:.1f}px;height:{boxh:.1f}px">'
            f'{scaled}{play_mark() if play else ""}</div>')


# ---------------------------------------------------------------- posts

POSTS = {
    "single": dict(
        name="Mira Okonkwo", handle="mirabuilds", date="Nov 12, 5:12 PM", tags=["design"],
        text="The best interfaces assume you are competent and tired. Every confirmation "
             "dialog is a small bet that you are neither. A dialog that asks again what the "
             "last screen already knew is the tax a product charges for its own doubt.",
        media=[dict(kind="photo", ratio=0.75)],
        metrics=[("comment", 12), ("share", 38), ("heart", 214)],
    ),
    "grid": dict(
        name="Priya Raman", handle="priya_writes", date="Nov 10, 4:30 PM", tags=["notes"],
        text="Notes are not storage. They are a conversation with the person you will be in "
             "six months, who will not remember why this mattered.",
        media=[dict(kind="dash", ratio=1.0), dict(kind="game", ratio=16 / 9),
               dict(kind="photo", ratio=0.75), dict(kind="chart", ratio=4 / 3)],
        metrics=[("comment", 4), ("share", 9), ("heart", 61)],
    ),
}


def esc(s):
    return html.escape(s, quote=False)


def copy_lines(text, width):
    """Chinese-aware line count at the given width, 13px."""
    per = int(width / BODY)                     # one CJK glyph is ~1em wide
    lines, cur = 1, 0
    for word in text.split(" "):
        n = sum(1 if ord(ch) > 0x2E80 else 0.55 for ch in word) + (1 if cur else 0)
        if cur + n > per:
            lines += 1
            cur = 0
        cur += n
    return lines


ICON = {
    "comment": '<path d="M14 7.6c0 2.6-2.7 4.7-6 4.7-.7 0-1.4-.1-2-.2L3 13.4l.7-2.2C2.6 10.4 2 9.1 2 7.6 2 5 4.7 3 8 3s6 2 6 4.6Z"/>',
    "share": '<circle cx="4" cy="8" r="1.7"/><circle cx="12" cy="3.6" r="1.7"/><circle cx="12" cy="12.4" r="1.7"/><path d="M5.6 7.2 10.4 4.4M5.6 8.8l4.8 2.8"/>',
    "heart": '<path d="M8 13S2.5 9.7 2.5 6.4A2.9 2.9 0 0 1 8 5a2.9 2.9 0 0 1 5.5 1.4C13.5 9.7 8 13 8 13Z"/>',
    "copy": '<rect x="5.5" y="5.5" width="8" height="8" rx="1.6"/><path d="M10.5 5.5v-1a1 1 0 0 0-1-1h-6a1 1 0 0 0-1 1v6a1 1 0 0 0 1 1h1"/>',
    "external": '<path d="M9.5 2.5H13.5V6.5M13 3 7.5 8.5"/><path d="M12 9.5v3a1 1 0 0 1-1 1H3.5a1 1 0 0 1-1-1V5a1 1 0 0 1 1-1h3"/>',
    "globe": '<circle cx="8" cy="8" r="5.4"/><path d="M2.6 8h10.8M8 2.6c3.2 3 3.2 7.8 0 10.8M8 2.6c-3.2 3-3.2 7.8 0 10.8"/>',
}


def svg(path, size=16, ink="#756a63", stroke=1.3):
    return (f'<svg width="{size}" height="{size}" viewBox="0 0 16 16" fill="none" '
            f'stroke="{ink}" stroke-width="{stroke}" stroke-linecap="round" '
            f'stroke-linejoin="round">{path}</svg>')


# ---------------------------------------------------------------- the dialog

def header(post):
    pills = "".join(
        f'<span class="pill">{esc(t)}<span class="px">×</span></span>' for t in post["tags"])
    return f'''
      <div class="hd">
        <div class="ava">{esc(post["name"][:2])}</div>
        <div class="who">
          <span class="nm">{esc(post["name"])}</span>
          <span class="hd2">@{post["handle"]}</span>
          <span class="dt">{svg(ICON["globe"], 12, "#756a63")}<span>{post["date"]}</span></span>
          {pills}<span class="inp">Enter a new tag name</span>
        </div>
        <div class="acts">
          <span class="act">{svg(ICON["copy"])}</span>
          <span class="act">{svg(ICON["external"], 16, "#b4553f")}</span>
          <span class="act">{svg('<path d="M4 4l8 8M12 4l-8 8"/>')}</span>
        </div>
      </div>
      <div class="div"></div>'''


def metrics(post, width=None):
    style = f'style="width:{width}px"' if width else ""
    cells = "".join(
        f'<span class="m">{svg(ICON[k], 14, "#756a63")}<b>{v}</b></span>' for k, v in post["metrics"])
    return f'<div class="mtr" {style}>{cells}</div>'


def body_text(post, width):
    return f'<div class="txt" style="width:{width}px">{esc(post["text"])}</div>'


def media_single(post, variant):
    m = post["media"][0]
    if variant == "A":
        r = min(max(m["ratio"], 0.8), 3.2)
        return cover(m["kind"], CW, CW / r, r, "center")
    if variant == "B":                      # B2: the whole picture, height capped
        r = min(max(m["ratio"], 0.8), 3.2)
        h = min(CW / r, BAND)
        frame = cover(m["kind"], h * r, h, m["ratio"], "center")
        return (f'<div style="display:flex;justify-content:center">{frame}</div>'
                if h * r < CW - 0.5 else frame)
    if variant == "D":
        return cover(m["kind"], CW, CW / (16 / 9), 16 / 9, "center")
    return ""            # C draws it in the rail


def media_grid(post, variant):
    items = post["media"]
    if variant == "A":                      # every cell follows its own picture
        rows, hs = "", []
        for i in range(0, len(items), 2):
            cells = "".join(
                f'<div class="cell">{cover(items[j]["kind"], (CW - GRID_GAP) / 2, ((CW - GRID_GAP) / 2) / min(max(items[j]["ratio"], 0.8), 3.2), min(max(items[j]["ratio"], 0.8), 3.2))}</div>'
                for j in range(i, min(i + 2, len(items))))
            rows += f'<div class="grow" style="align-items:flex-start">{cells}</div>'
            hs.append(max(((CW - GRID_GAP) / 2) / min(max(items[j]["ratio"], 0.8), 3.2)
                          for j in range(i, min(i + 2, len(items)))))
        return rows, sum(hs) + GRID_GAP * (len(hs) - 1)
    cell = (CW - GRID_GAP) / 2
    h = cell * 2 / 3                        # a 3:2 tile: the band's shape, half size
    rows = ""
    for i in range(0, len(items), 2):
        cells = "".join(f'<div class="cell">{cover(items[j]["kind"], cell, h, items[j]["ratio"])}</div>'
                        for j in range(i, min(i + 2, len(items))))
        rows += f'<div class="grow">{cells}</div>'
    return rows, h * 2 + GRID_GAP


def post_html(post, variant):
    has_media = bool(post["media"])
    txt_w = MEASURE if variant in ("B", "D") else CW
    txt = body_text(post, txt_w)
    mt = metrics(post, txt_w if variant in ("B", "D") else None)

    if not has_media:
        return f'<div class="post">{txt}{mt}</div>'

    single = len(post["media"]) == 1
    if variant == "C":
        rail_w = 264
        if single:
            m = post["media"][0]
            r = min(max(m["ratio"], 0.8), 3.2)
            box = cover(m["kind"], rail_w, rail_w / r, r, "top")
        else:
            box, _ = "", 0
            cells = "".join(f'<div class="cell">{cover(x["kind"], (rail_w - GRID_GAP) / 2, (rail_w - GRID_GAP) / 2, x["ratio"])}</div>'
                            for x in post["media"])
            box = f'<div class="gcol" style="width:{rail_w}px">{cells}</div>'
        return (f'<div class="post row">'
                f'<div class="main"><div class="txt" style="width:440px">{esc(post["text"])}</div>{metrics(post, 440)}</div>'
                f'<div class="rail">{box}</div></div>')

    if variant == "D":
        cover_h = CW / (16 / 9)
        if single:
            media_html = media_single(post, "D")
        else:
            g, _ = media_grid(post, "B")
            media_html = g
        return (f'<div class="post">'
                f'<div class="media first">{media_html}</div>'
                f'<div class="txt" style="width:{MEASURE}px;margin-top:{MEDIA_GAP}px">{esc(post["text"])}</div>'
                f'<div class="txt" style="width:{MEASURE}px;margin-top:{MEDIA_GAP}px"></div>'
                f'{metrics(post, MEASURE)}</div>')

    if single:
        media_html = media_single(post, variant)
        return f'<div class="post">{txt}<div class="media">{media_html}</div>{mt}</div>'
    g, _ = media_grid(post, variant)
    return f'<div class="post">{txt}<div class="media">{g}</div>{mt}</div>'


# ---------------------------------------------------------------- heights

def post_height(post, variant):
    has = bool(post["media"])
    txt_w = MEASURE if variant in ("B", "D") else (440 if variant == "C" else CW)
    th = copy_lines(post["text"], txt_w) * BODY_LH
    mt = 16 + 20
    if not has:
        return th + mt
    if variant == "C":
        rail_w = 264
        if len(post["media"]) == 1:
            m = post["media"][0]
            r = min(max(m["ratio"], 0.8), 3.2)
            mh = rail_w / r
        else:
            mh = ((rail_w - GRID_GAP) / 2) * 2 + GRID_GAP
        return max(th + mt, mh)
    if variant == "A":
        if len(post["media"]) == 1:
            m = post["media"][0]
            return th + MEDIA_GAP + CW / min(max(m["ratio"], 0.8), 3.2) + mt
        _, gh = media_grid(post, "A")
        return th + MEDIA_GAP + gh + mt
    if variant == "B":
        if len(post["media"]) == 1:
            m = post["media"][0]
            r = min(max(m["ratio"], 0.8), 3.2)
            return th + MEDIA_GAP + min(CW / r, BAND) + mt
        _, gh = media_grid(post, "B")
        return th + MEDIA_GAP + gh + mt
    # D: the picture comes first
    if len(post["media"]) == 1:
        return CW / (16 / 9) + MEDIA_GAP + th + mt
    _, gh = media_grid(post, "B")
    return gh + MEDIA_GAP + th + mt


# ---------------------------------------------------------------- style

STYLE = '''
*{box-sizing:border-box}
body{margin:0;background:#e7ded4;font-family:-apple-system,"PingFang SC","Helvetica Neue",sans-serif;
     -webkit-font-smoothing:antialiased;color:#191514}
.wrap{padding:40px 44px 56px}
.h{font-size:19px;font-weight:600;letter-spacing:-.01em}
.note{font-size:12.5px;color:#5a4a43;margin-top:6px;line-height:1.6}
.grid{display:grid;grid-template-columns:780px 780px;gap:44px 40px;margin-top:26px;align-items:start}
.cellwrap{}
.lab{font:600 13px/1 ui-monospace,Menlo,monospace;color:#b4553f;letter-spacing:.08em;margin-bottom:4px}
.labt{font-size:14px;font-weight:600;margin-bottom:2px}
.labs{font-size:12px;color:#6b5b52;line-height:1.5;height:34px}
.stat{font-size:11.5px;color:#5a4a43;margin-top:8px}
.stat b{color:#191514}

.dlg{width:780px;background:#fbf7f3;border:1px solid #e6dacd;border-radius:12px;overflow:hidden;
     box-shadow:0 24px 60px rgba(25,21,20,.22);display:flex;flex-direction:column;max-height:720px;
     position:relative}
.dlg.clip:after{content:"";position:absolute;left:0;right:0;bottom:0;height:60px;pointer-events:none;
     background:linear-gradient(to bottom,rgba(251,247,243,0),rgba(251,247,243,.96));border-radius:0 0 12px 12px}
.hd{display:flex;gap:16px;align-items:flex-start;padding:22px 26px 18px}
.ava{width:30px;height:30px;border-radius:50%;background:#efe7df;flex:none;display:flex;
     align-items:center;justify-content:center;font-size:10px;font-weight:700;color:#191514}
.who{display:flex;align-items:center;gap:10px;flex-wrap:wrap;min-width:0;flex:1;height:28px;overflow:hidden}
.nm{font-family:"Source Serif 4",Charter,Georgia,"Songti SC",serif;font-size:16px;font-weight:600;
    white-space:nowrap}
.hd2{font-size:12px;color:#756a63;white-space:nowrap}
.dt{display:inline-flex;align-items:center;gap:4px;font-size:12px;color:#756a63;white-space:nowrap}
.pill{display:inline-flex;align-items:center;gap:4px;height:22px;padding:0 6px 0 10px;border-radius:999px;
      background:#efe7df;border:1px solid #e6dacd;font-size:12px;color:#5a4a43}
.px{font-size:12px;color:#8d8078}
.inp{display:inline-flex;align-items:center;height:28px;width:116px;border-radius:999px;
     border:1px solid #ded2c6;padding:0 12px;font-size:12px;color:#a2948c;white-space:nowrap;background:#fffdfa}
.acts{display:flex;gap:4px;flex:none;margin-left:10px}
.act{width:26px;height:26px;border-radius:6px;display:flex;align-items:center;justify-content:center}
.div{height:1px;background:#eadfd5}
.post{padding:24px 26px 26px;overflow:hidden}
.post.row{display:flex;gap:24px;align-items:flex-start}
.main{flex:1;min-width:0}
.rail{flex:none}
.txt{font-size:13px;line-height:20.8px;color:#191514}
.media{margin-top:14px}
.media.first{margin-top:0}
.frame{position:relative;overflow:hidden;border-radius:12px;background:#efe7df}
.art{position:absolute;inset:0;overflow:hidden}
.grow{display:flex;gap:6px}
.grow+.grow{margin-top:6px}
.cell{flex:1;min-width:0}
.gcol{display:flex;flex-wrap:wrap;gap:6px}
.gcol .cell{flex:0 0 calc(50% - 3px)}
.mtr{display:flex;gap:18px;align-items:center;margin-top:16px}
.m{display:inline-flex;align-items:center;gap:5px;font-size:11px;font-weight:600;color:#756a63}
.m b{font-weight:600}
'''


def page(title, note, postkey):
    post = POSTS[postkey]
    cells = []
    for k in "ABCD":
        title_k, sub = VARIANTS[k]
        ph = post_height(post, k)
        total = HDR + 1 + ph
        clipped = total > MAXH
        cells.append(f'''<div class="cellwrap">
          <div class="lab">{k}</div><div class="labt">{title_k}</div><div class="labs">{sub}</div>
          <div class="dlg{' clip' if clipped else ''}">{header(post)}{post_html(post, k)}</div>
          <div class="stat">对话框 <b>{min(total, MAXH):.0f}</b> 高 · 内容 <b>{total:.0f}</b>
            {'· <b>超出一屏,要滚</b>' if clipped else '· 一屏放得下'}</div>
        </div>''')
    return f'''<!doctype html><meta charset="utf-8"><style>{STYLE}</style>
<div class="wrap">
  <div class="h">{title}</div><div class="note">{note}</div>
  <div class="grid">{''.join(cells)}</div>
</div>'''


VARIANTS = {
    "A": ("现状:图片按自己的比例,通栏",
          "单图 = 728 × 比例(夹 0.8–3.2);多图 2 列,每格自己一个比例"),
    "B": ("正文收窄,媒体封顶 420(B2)",
          "正文 600;单图最高 420、整张都在、居中;多图等高"),
    "C": ("图文并排 440 + 264",
          "有图时文字在左 440,图片在右 264,对话框高度由两者较高的一边决定"),
    "D": ("图片置顶 16:9,正文在下",
          "先图后文,图片固定 16:9(410 高),正文 600"),
}

# ---------------------------------------------------------------- case matrix
#
# Every content mix a bookmark can have, drawn under B. Two rules are on trial:
#
#   B1  正文 600,单图裁成 420 的带(cover)
#   B2  正文 600,单图高度封顶 420、整张都在(contain),多图等高 3:2 格
#
# Only B1 still makes a crop decision; B2 crops nothing unless the picture is
# outside 4:5–3.2:1, where detailMediaRatio already clamps.

import math

def text_height(text, width):
    """One flowing paragraph of 13px text: CJK counts 1em, everything else ~0.55em."""
    units = sum(1 if ord(ch) > 0x2E80 else 0.55 for ch in text)
    return max(1, math.ceil(units / (width / BODY))) * BODY_LH


MED = ("设计系统不是一套组件,是一套约定。约定写在文档里没人读,写在组件里没人改,"
        "最后真正生效的是评审时那句「我们这里一直这么做」。所以系统的第一产物不是 token,"
        "是能让新人对齐的默认值:默认间距、默认字号、默认在什么情况下不新增。"
        "系统失效从来不是因为不够全,是因为默认值太多。"
        "第二个产物是拒绝的能力:有人提了一个新组件,你能在一分钟内说清楚为什么不用加,"
        "而不是先说「我看看」。这需要一条能指出去的线:这个需求已经由哪个组件覆盖,"
        "或者它根本不是组件的问题,是流程的问题。"
        "第三个产物是度量。没有度量的系统只能靠感觉维护,而有感觉的人通常已经升职离开了一线。"
        "度量不必复杂:一个季度里新增了几个组件、删掉了几个、有多少界面在没有新增组件的情况下"
        "被搭出来。数字本身不重要,重要的是它每周都会出现在某个人的屏幕上,逼着人回答「为什么」。"
        "第四个产物是迁移路径。任何一次改动都会让旧界面变成库存,库存不清,新规则就只是又一种做法。"
        "最省事的迁移不是发一封通知,是把旧写法加进 lint,让它在提交时自己变红。"
        "第五个产物是放弃。一个活着的系统一定删过东西:三年没被用过一次的变体、"
        "为了一个活动加的分支、某个已经下线的业务留下的颜色,每一样都在向后来的人收费。"
        "删掉它们不需要理由,留下它们才需要。")

LONG = MED + (
        "第六个产物是口径。同一个词在两个团队里差一点意思,拼起来就是一次返工:「列表」是带分页的还是"
        "整段滚动的,「空状态」是还没加载还是确实没有。把这些词写进同一页词汇表,比多写十页规范有用。"
        "第七个产物是节奏。系统按季度改一次版本,而不是按需求改;两次版本之间只修错误,不加概念。"
        "这条最不受欢迎,因为它意味着有人要等,但它是唯一能让下游排期的办法。"
        "第八个产物是证据。每个决定留一句当时的约束:为什么这个控件必须有上限,为什么那张表不用虚拟滚动。"
        "半年后有人想改掉它,至少能看到当时在防什么,而不是从「谁写的」开始考古。"
        "第九个产物是边界。系统管颜色、间距、可访问性,不管业务文案,也不管页面该有几个区块。"
        "边界写清楚,评审才有地方停;边界不清,每一次讨论都会滑到「这个产品应该怎么做」。"
        "第十个产物是沉默。大部分时候没人提系统,界面照样被搭出来,这才是它活着的样子。"
        "如果每个月都要开会推动一次,那说明它还没成为默认值,只是又一个需要维护的项目。"
        "还有一件常被忽略的事:文档写给三个月后的自己看,不是写给评审看。"
        "所以它应该回答「为什么不是另一种做法」,而不是罗列「我们支持哪些用法」;"
        "前者能拦住一次错误的复用,后者只能证明作者很勤奋。"
        "同样的道理,组件的默认值比可配置项重要:每多一个开关,就多一种没人测过的组合。"
        "一个系统能不能活过两任维护者,取决于新来的人读完一页纸之后,能不能直接改对一处间距。"
        "所以最后一条还是回到开头:系统不是组件库,是一组被默认遵守的约定,"
        "而默认值只会被两件事改变,一是有人持续地删,二是有人持续地解释为什么删。"
        "这也是为什么很多系统死在第二版:第一版回答「有没有」,第二版要回答「为什么是它、"
        "为什么不是那个已经被用惯的老写法」,后者没有现成的话可说,只能一条一条讲清楚。"
        "讲不清的时候就把规则缩小:先守住一个组件、一处间距、一种状态,守住了再谈下一页,"
        "而不是先画一张覆盖全部场景的图。守住的东西会自己长大,守不住的东西写得再全"
        "也只是一份文档,三个月后没人记得它存在过。所以每次复盘我都只问一句:"
        "上个季度有哪一条约定,是靠某个人记得才生效的?如果答案是「有」,那一条就还没进系统,"
        "它只是这个人的个人习惯,而人总会走。把它变成默认值、变成 lint、变成模板里删除不掉的"
        "那一行,才算真的存下来了。剩下的都可以慢慢来,因为剩下的本来就会自己长出来。")

CASES = {
    "short": dict(
        name="林岚", handle="lanwrites", date="11月12日 17:12", tags=[],
        text="阅读是把别人的时间折叠成自己的。",
        media=[], metrics=[("comment", 4), ("share", 2), ("heart", 86)]),
    "three": dict(
        name="陈默", handle="chenmo", date="11月11日 09:20", tags=["design"],
        text="好用的默认值是设计里最便宜的东西:它不需要新界面、不需要解释,"
             "只需要有人先说一次「就这样吧」,而且说得让人愿意跟着。"
             "默认值定错了,后面每一次评审都在替它还债;而定对了的那天没人会注意到,"
             "这大概是它唯一的下场。",
        media=[], metrics=[("comment", 6), ("share", 11), ("heart", 132)]),
    "long": dict(
        name="周予", handle="zhouyu_", date="11月10日 22:40", tags=["design", "systems"],
        text=MED, media=[], metrics=[("comment", 9), ("share", 31), ("heart", 208)]),
    "huge": dict(
        name="周予", handle="zhouyu_", date="11月10日 22:40", tags=["design", "systems"],
        text=LONG, media=[], metrics=[("comment", 18), ("share", 64), ("heart", 402)]),
    "notext": dict(
        name="Mira Okonkwo", handle="mirabuilds", date="11月9日 14:02", tags=[],
        text="", media=[dict(kind="dash", ratio=1.0)],
        metrics=[("comment", 3), ("share", 5), ("heart", 58)]),
    "notext4": dict(
        name="Priya Raman", handle="priya_writes", date="11月9日 11:26", tags=[],
        text="", media=[dict(kind="photo", ratio=0.75), dict(kind="game", ratio=16 / 9),
                        dict(kind="chart", ratio=4 / 3), dict(kind="dash", ratio=1.0)],
        metrics=[("comment", 7), ("share", 15), ("heart", 133)]),
    "wide": dict(
        name="小奇说", handle="xingbugengming", date="11月9日 10:15", tags=[],
        text="十天,一个人,我用 AI 加 Unreal 5.8 做完了这段关卡。",
        media=[dict(kind="game", ratio=16 / 9)],
        metrics=[("comment", 22), ("share", 91), ("heart", 340)]),
    "tall": dict(
        name="林岚", handle="lanwrites", date="11月8日 08:40", tags=["travel"],
        text="十一月的青岛,海是灰的,岸上的人比夏天少一半,反而像它本来的样子。",
        media=[dict(kind="photo", ratio=0.75)],
        metrics=[("comment", 2), ("share", 6), ("heart", 74)]),
    "video": dict(
        name="老周", handle="laozhou", date="11月8日 20:08", tags=[],
        text="两分钟讲清楚纯 Go 的 SQLite 在 macOS 上为什么启动更快。",
        media=[dict(kind="film", ratio=16 / 9, video=True)],
        metrics=[("comment", 14), ("share", 33), ("heart", 188)]),
    "vvideo": dict(
        name="阿丘", handle="axiu", date="11月7日 19:30", tags=[],
        text="在电梯里拍的,竖屏反而比横屏更近。",
        media=[dict(kind="film", ratio=9 / 16, video=True)],
        metrics=[("comment", 1), ("share", 3), ("heart", 29)]),
    "threeimg": dict(
        name="归藏", handle="guizang", date="11月7日 09:44", tags=["agent"],
        text="一套 Agent 工作台的三个状态:空闲、执行、等待确认。",
        media=[dict(kind="dash", ratio=1.0), dict(kind="photo", ratio=0.75),
               dict(kind="game", ratio=16 / 9)],
        metrics=[("comment", 9), ("share", 21), ("heart", 145)]),
    "four": dict(
        name="木白", handle="mubai", date="11月6日 17:05", tags=["ui"],
        text="四个来源:仪表盘、现场照片、一段录屏、一张周报。",
        media=[dict(kind="dash", ratio=1.0), dict(kind="photo", ratio=0.75),
               dict(kind="film", ratio=16 / 9, video=True), dict(kind="chart", ratio=4 / 3)],
        metrics=[("comment", 5), ("share", 12), ("heart", 97)]),
    "pano": dict(
        name="甘棠", handle="gantang", date="11月6日 11:18", tags=[],
        text="同一段海岸,从东头走到西头。",
        media=[dict(kind="game", ratio=3.2)],
        metrics=[("comment", 2), ("share", 4), ("heart", 51)]),
}


def clamp_ratio(r):
    return min(max(r, 0.8), 3.2)


def b_single(m, band):
    r = clamp_ratio(m["ratio"])
    play = m.get("video", False)
    if band:                                    # B1: fill 728, crop to 420
        return cover(m["kind"], CW, BAND, m["ratio"], "center", play)
    h = min(CW / r, BAND)                       # B2: whole picture, height capped
    return cover(m["kind"], h * r, h, m["ratio"], "center", play)


def b_grid(items):
    cell = (CW - GRID_GAP) / 2
    th = cell * 2 / 3
    rows = ""
    for i in range(0, len(items), 2):
        pair = items[i:i + 2]
        cells = "".join(
            f'<div class="ucell">{cover(x["kind"], cell, th, x["ratio"], "center", x.get("video", False))}</div>'
            for x in pair)
        if len(pair) == 1:
            cells += '<div class="ucell"></div>'
        rows += f'<div class="ugrow">{cells}</div>'
    return rows


def b_post(post, band=False):
    parts = []
    if post["text"]:
        parts.append(f'<div class="txt" style="width:{MEASURE}px">{esc(post["text"])}</div>')
    if post["media"]:
        mt = MEDIA_GAP if post["text"] else 0
        if len(post["media"]) == 1:
            box = (f'<div style="display:flex;justify-content:center">'
                   f'{b_single(post["media"][0], band)}</div>')
        else:
            box = b_grid(post["media"])
        parts.append(f'<div style="margin-top:{mt}px">{box}</div>')
    parts.append(metrics(post, MEASURE))
    return f'<div class="post">{"".join(parts)}</div>'


def b_height(post, band=False):
    th = text_height(post["text"], MEASURE) if post["text"] else 0
    mt = 36
    if not post["media"]:
        return th + mt
    if len(post["media"]) == 1:
        mh = BAND if band else min(CW / clamp_ratio(post["media"][0]["ratio"]), BAND)
    else:
        rows = (len(post["media"]) + 1) // 2
        mh = ((CW - GRID_GAP) / 2) * 2 / 3 * rows + GRID_GAP * (rows - 1)
    return th + (MEDIA_GAP if post["text"] else 0) + mh + mt


CASE_STYLE = '''
.ugrow{display:flex;gap:6px}
.ugrow+.ugrow{margin-top:6px}
.ucell{flex:0 0 361px;min-width:0}
'''


def case_page(title, note, items):
    cells = []
    for i, (key, label, sub, band) in enumerate(items, 1):
        post = CASES[key]
        ph = b_height(post, band)
        total = HDR + 1 + ph
        clipped = total > MAXH
        cells.append(f'''<div class="cellwrap">
          <div class="lab">{i:02d}</div><div class="labt">{label}</div><div class="labs">{sub}</div>
          <div class="dlg{' clip' if clipped else ''}">{header(post)}{b_post(post, band)}</div>
          <div class="stat">对话框 <b>{min(total, MAXH):.0f}</b> 高 · 内容 <b>{total:.0f}</b>
            {'· <b>超出一屏,要滚</b>' if clipped else '· 一屏放得下'}</div>
        </div>''')
    return f'''<!doctype html><meta charset="utf-8"><style>{STYLE}{CASE_STYLE}</style>
<div class="wrap">
  <div class="h">{title}</div><div class="note">{note}</div>
  <div class="grid">{''.join(cells)}</div>
</div>'''


if __name__ == "__main__":
    for key, title, note in [
        ("single", "详情页:一张竖图",
         "同一个弹窗(780 宽,最高 720),同一条书签:五句话加一张 3:4 的竖图。"),
        ("grid", "详情页:四张图",
         "同一个弹窗(780 宽,最高 720),同一条书签:两句话加四张图(1:1、16:9、3:4、4:3)。"),
    ]:
        open(os.path.join(OUT, f"detail-{key}.html"), "w").write(page(title, note, key))
        for k in "ABCD":
            print(key, k, f"内容高 {HDR + 1 + post_height(POSTS[key], k):.0f}")

    text_cases = [
        ("short", "一句文字 · 无图", "正文一行;弹窗跟着内容缩,不留空白", False),
        ("three", "三行文字 · 一个标签", "库里最常见的一档(中位 143 字)", False),
        ("long", "长文 · 536 字", "p90 是 476 字;这个长度仍然一屏放得下", False),
        ("huge", "超长文 · 1507 字", "只有这一档才真的滚;正文仍是最宽 600", False),
        ("notext", "没有正文 · 一张图", "正文块整个不存在,图片从内边距开始", False),
        ("notext4", "没有正文 · 四张图", "同上,图片直接进等高网格", False),
    ]
    media_cases = [
        ("wide", "一张 16:9 横图", "728 × 410,封顶 420 没有碰到", False),
        ("tall", "一张 3:4 竖图 · B1 裁切", "728 × 420,只露出中间 43%", True),
        ("tall", "一张 3:4 竖图 · B2 整张", "315 × 420 居中,一点没裁", False),
        ("video", "一段 16:9 视频", "和横图同一个盒子,加 58 点播放盘", False),
        ("vvideo", "一段 9:16 竖视频", "236 × 420 居中;播放盘不跟着缩", False),
        ("threeimg", "三张图", "两列等高 361 × 240;第三张留半格", False),
        ("four", "四张图,其中一张是 GIF", "四格等高;视频格带播放盘", False),
        ("pano", "一张 3.2:1 全景", "728 × 227,封顶用不上,宽度先到顶", False),
    ]
    open(os.path.join(OUT, "detail-cases-text.html"), "w").write(case_page(
        "详情页:正文的六种样子",
        "同一个弹窗(B:正文 600)。正文从一句到一千五百字,弹窗高度跟着内容走,"
        "只有超过 720 之后才滚;没有正文的帖子,正文块整个不存在。",
        text_cases))
    open(os.path.join(OUT, "detail-cases-media.html"), "w").write(case_page(
        "详情页:媒体的每一种组合(B)",
        "正文 600 不动,变的是媒体:单图高度封顶 420、整张都在(B2);多图两列等高 3:2 格;"
        "视频只是加一个 58 点的播放盘,不改变盒子。第二、三格是同一条书签,只有裁不裁的区别。"
        "X 的一条帖子要么一个视频(含 GIF)、要么最多四张图,不存在视频加图片的混排。",
        media_cases))
    print("cases written")
