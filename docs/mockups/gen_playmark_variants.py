#!/usr/bin/env python3
"""The play mark, five sizes and grounds, at true scale.

Not part of the build. Same content on every page: a detail picture, two grid cards (one dark
poster, one light one, so the mark's contrast can be judged against both) and a list thumbnail.
Only the disc changes.

Sizes come from the app: detail pane 726, grid card 458 (the reference width; the user's window
draws them ~785), list thumbnail 148 x 108.

    python3 gen_playmark_variants.py
    chrome --headless=new --force-device-scale-factor=2 --window-size=1000,3560 \
           --screenshot=playmark-A.png playmark-A.html
"""
import os

OUT = os.path.dirname(os.path.abspath(__file__))

CARD = 458
PANE = 726
THUMB_W, THUMB_H = 148, 108

FONT = "-apple-system, 'PingFang SC', 'Helvetica Neue', sans-serif"
INK = "#191514"
RED = "#B83D2E"      # the brand's 朱红, design.md §4
PAPER = "#F8F1EA"


# ---------------------------------------------------------------- posters

def poster(kind):
    """A video's still frame, one dark and one light, so a mark can fail on either."""
    if kind == "dark":
        return ('<div style="position:absolute;inset:0;background:'
                'linear-gradient(150deg,#2b3a44 0%,#141c22 55%,#07090b 100%)"></div>')
    return ('<div style="position:absolute;inset:0;background:'
            'linear-gradient(150deg,#efe4d8 0%,#d9c8b6 60%,#c2ad99 100%)"></div>')


def disc(size, ground, ink, ring=None):
    glyph = round(size * 0.42)
    border = f"box-shadow:0 0 0 1px {ring};" if ring else ""
    return (f'<div style="position:absolute;left:50%;top:50%;transform:translate(-50%,-50%);'
            f'width:{size}px;height:{size}px;border-radius:50%;background:{ground};{border}'
            f'display:flex;align-items:center;justify-content:center">'
            f'<div style="width:0;height:0;border-left:{glyph}px solid {ink};'
            f'border-top:{round(glyph*0.62)}px solid transparent;'
            f'border-bottom:{round(glyph*0.62)}px solid transparent;margin-left:{round(glyph*0.3)}px">'
            f'</div></div>')


# ---------------------------------------------------------------- the variants

VARIANTS = {
    # key: (card, detail, thumb, ground, ink, ring)
    "A": (40, 58, 28, "rgba(25,21,20,.55)", "#fff", None),
    "B": (52, 72, 34, "rgba(25,21,20,.55)", "#fff", None),
    "C": (46, 64, 30, RED, "#fff", None),
    "D": (46, 64, 30, "rgba(248,241,234,.94)", INK, None),
    "E": (48, 68, 32, "rgba(25,21,20,.86)", "#fff", "rgba(255,255,255,.28)"),
}

NOTES = {
    "A": ("A 现状", "卡片 40 / 详情 58 / 列表 28，深色半透明盘，白三角。"),
    "B": ("B 只放大", "同一套颜色，卡片 52 / 详情 72 / 列表 34，三角也跟着放大。"),
    "C": ("C 朱红盘", "品牌朱红 #B83D2E 实心盘，白三角。最醒目，也是最响的。"),
    "D": ("D 纸色盘", "纸色 #F8F1EA 实心盘，墨色三角。深海报上亮、浅海报上靠盘底压住。"),
    "E": ("E 实心墨盘", "近黑 86% 盘 + 一圈 28% 白边，白三角。比现在重，但不引入新颜色。"),
}


def art(sizes, kind, size_index):
    card, detail, thumb = sizes[:3]
    size = (card, detail, thumb)[size_index]
    ground, ink, ring = sizes[3], sizes[4], sizes[5]
    return poster(kind) + disc(size, ground, ink, ring)


def detail_block(v):
    sizes = VARIANTS[v]
    return f'''<div style="margin-bottom:22px">
      <div style="font:600 11px {FONT};color:#8d857c;margin-bottom:6px">详情 · 726×408</div>
      <div style="position:relative;width:{PANE}px;height:{round(PANE*9/16)}px;border-radius:12px;overflow:hidden">
        {art(sizes, "dark", 1)}
      </div>
    </div>'''


def card(kind, v, width=CARD):
    sizes = VARIANTS[v]
    return f'''<div style="width:{width}px">
      <div style="font:600 11px {FONT};color:#8d857c;margin-bottom:6px">卡片 · {kind == "dark" and "深色海报" or "浅色海报"} · {width}×{round(width*9/16)}</div>
      <div style="position:relative;width:{width}px;height:{round(width*9/16)}px;overflow:hidden;background:#efe7df">
        {art(sizes, kind, 0)}
      </div>
    </div>'''


def thumb(v):
    sizes = VARIANTS[v]
    return f'''<div style="width:{THUMB_W}px">
      <div style="font:600 11px {FONT};color:#8d857c;margin-bottom:6px">列表 · 148×108</div>
      <div style="position:relative;width:{THUMB_W}px;height:{THUMB_H}px;border-radius:12px;overflow:hidden">
        {art(sizes, "dark", 2)}
      </div>
    </div>'''


def page(v):
    title, note = NOTES[v]
    return f'''<!doctype html><html lang="zh-CN"><head><meta charset="utf-8">
<style>
  body {{ margin:0; padding:34px 38px 40px; background:{PAPER}; color:{INK}; font:13px/1.5 {FONT}; }}
  h1 {{ font-size:18px; margin:0 0 6px; font-weight:650; }}
  p.note {{ margin:0 0 24px; color:#6f665d; }}
  .row {{ display:flex; gap:20px; align-items:flex-start; }}
  .cols {{ display:flex; gap:20px; align-items:flex-end; }}
</style></head><body>
  <h1>{title}</h1>
  <p class="note">{note}</p>
  {detail_block(v)}
  <div class="cols">{card("dark", v)}{card("light", v)}{thumb(v)}</div>
  <div class="cols" style="margin-top:20px">{card("dark", v, 785)}</div>
</body></html>'''


for key in VARIANTS:
    open(os.path.join(OUT, f"playmark-{key}.html"), "w").write(page(key))
    print("wrote", f"playmark-{key}.html")
