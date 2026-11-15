#!/usr/bin/env python3
"""Generates the photo/video marking mockups at true scale.

Not part of the build. Five variants of the same content -- a grid card, a detail's single
video and a detail grid holding a video -- so the only difference on each page is the mark.
Sizes come from the app: card 458 wide with a 16:9 media band, detail pane 726, grid tile
360 x 240, play disc 58 in the detail.

    python3 gen_mediamark_mockup.py
    chrome --headless=new --force-device-scale-factor=2 --window-size=1240,1500 \
           --screenshot=mediamark-A.png mediamark-A.html
"""
import os

OUT = os.path.dirname(os.path.abspath(__file__))

CARD = 458
TILE_W, TILE_H = 360, 240
PANE = 726
DISC = 58

FONT = "-apple-system, 'PingFang SC', 'Helvetica Neue', sans-serif"


# ---------------------------------------------------------------- posters

def poster(kind):
    """A stand-in for a real poster; the frame is the poster's size."""
    if kind == "photo":
        return ('<div style="position:absolute;inset:0;background:'
                'radial-gradient(120% 90% at 70% 20%, #d9b48f 0%, #a8764f 45%, #4b3323 100%)">'
                f'<div style="position:absolute;right:18px;top:14px;color:rgba(255,255,255,.85);'
                f'font:11px {FONT}">一张照片</div></div>')
    if kind == "video":
        return ('<div style="position:absolute;inset:0;background:'
                'linear-gradient(150deg,#2b3a44 0%,#141c22 55%,#07090b 100%)">'
                f'<div style="position:absolute;right:18px;top:14px;color:rgba(255,255,255,.75);'
                f'font:11px {FONT}">一段视频</div></div>')
    return ('<div style="position:absolute;inset:0;background:'
            'linear-gradient(140deg,#3a2f4b 0%,#1d1830 60%,#0c0a14 100%)">'
            f'<div style="position:absolute;right:18px;top:14px;color:rgba(255,255,255,.75);'
            f'font:11px {FONT}">一个 GIF</div></div>')


def disc(size):
    glyph = size * 0.28
    return (f'<div style="position:absolute;left:50%;top:50%;transform:translate(-50%,-50%);'
            f'width:{size}px;height:{size}px;border-radius:50%;background:rgba(25,21,20,.55);'
            f'display:flex;align-items:center;justify-content:center">'
            f'<div style="width:0;height:0;border-left:{glyph:.0f}px solid #fff;'
            f'border-top:{glyph*0.62:.0f}px solid transparent;'
            f'border-bottom:{glyph*0.62:.0f}px solid transparent;margin-left:{glyph*0.3:.0f}px">'
            f'</div></div>')


def chip(label, size=24):
    glyph = size * 0.40
    return (f'<div style="position:absolute;left:10px;bottom:10px;height:{size}px;'
            f'padding:0 {size*0.42:.0f}px;border-radius:{size/2:.0f}px;'
            f'background:rgba(25,21,20,.62);color:#fff;display:flex;align-items:center;gap:5px;'
            f'font:600 {size*0.46:.0f}px/1 {FONT}">'
            f'<div style="width:0;height:0;border-left:{glyph:.0f}px solid #fff;'
            f'border-top:{glyph*0.62:.0f}px solid transparent;'
            f'border-bottom:{glyph*0.62:.0f}px solid transparent;margin-left:1px"></div>'
            f'{label}</div>')


def bare_play(size=DISC):
    glyph = size * 0.5
    return (f'<div style="position:absolute;left:50%;top:50%;transform:translate(-50%,-50%);'
            f'width:0;height:0;border-left:{glyph:.0f}px solid rgba(255,255,255,.94);'
            f'border-top:{glyph*0.62:.0f}px solid transparent;'
            f'border-bottom:{glyph*0.62:.0f}px solid transparent;margin-left:{glyph*0.22:.0f}px;'
            f'filter:drop-shadow(0 1px 5px rgba(0,0,0,.65))"></div>')


# ---------------------------------------------------------------- the mark

def marked(kind, variant, card=False):
    """The poster plus whatever the variant draws over a video (or a GIF)."""
    art = poster(kind)
    if kind == "photo":
        return art
    label = "视频" if kind == "video" else "GIF"
    size = 40 if card else DISC
    if variant == "A":
        # Today: the detail draws a 58-point disc; a card draws nothing at all.
        return art if card else art + disc(DISC)
    if variant == "B":
        # A disc everywhere, scaled to what it sits on.
        return art + disc(size)
    if variant == "C":
        # A corner chip naming the type; photos stay unmarked.
        return art + chip(label, 20 if card else 24)
    if variant == "D":
        # Disc and chip: the disc says "press", the chip says what it is.
        return art + disc(size) + chip(label, 20 if card else 24)
    if variant == "E":
        # A glyph with no disc, so the poster keeps its picture.
        return art + bare_play(size)
    return art


# ---------------------------------------------------------------- pieces

def card(kind, variant):
    w, h = CARD, CARD * 9 / 16
    return f'''<div style="width:{w}px;border:1px solid #e9e2da;border-radius:12px;overflow:hidden;background:#fff">
      <div style="position:relative;width:{w}px;height:{h:.0f}px;overflow:hidden;background:#efe7df">
        {marked(kind, variant, card=True)}
      </div>
      <div style="padding:14px 16px 16px;font:12px/1.6 {FONT};color:#1d1a17">
        <div style="display:flex;align-items:center;gap:8px;margin-bottom:8px">
          <div style="width:24px;height:24px;border-radius:50%;background:#e6ddd3"></div>
          <div><div style="font-weight:600;font-size:12px">Ada Lovelace</div>
          <div style="color:#8d857c;font-size:11px">@ada</div></div>
        </div>
        <div>一条书签的正文,两行左右,然后是它带的那一张媒体。</div>
      </div>
    </div>'''


def single(variant):
    return f'''<div style="width:{PANE}px">
      <div style="font:600 12px {FONT};color:#7c736a;margin-bottom:8px">详情 · 一张 16:9 视频</div>
      <div style="position:relative;width:{PANE}px;height:{PANE*9/16:.0f}px;border-radius:12px;overflow:hidden;background:#efe7df">
        {marked("video", variant)}
      </div>
    </div>'''


def tile(kind, variant):
    return f'''<div style="position:relative;width:{TILE_W}px;height:{TILE_H}px;border-radius:12px;overflow:hidden;background:#efe7df">
      {marked(kind, variant)}
    </div>'''


def grid(variant):
    tiles = [tile("photo", variant), tile("video", variant),
             tile("gif", variant), tile("photo", variant)]
    return f'''<div style="width:{PANE}px">
      <div style="font:600 12px {FONT};color:#7c736a;margin-bottom:8px">详情 · 四格,其中一格是视频</div>
      <div style="display:grid;grid-template-columns:{TILE_W}px {TILE_W}px;gap:6px">{''.join(tiles)}</div>
    </div>'''


NOTES = {
    "A": ("A 现状", "详情里是一枚 58 点播放盘;网格卡片上的视频和照片完全一样,没有任何标记。"),
    "B": ("B 统一播放盘", "卡片 40 点、详情 58 点,同一枚盘按所在位置缩放;照片不加东西。"),
    "C": ("C 类型角标", "视频和 GIF 在左下角挂一枚「视频」/「GIF」芯片;中心不加盘,海报完整。"),
    "D": ("D 盘 + 角标", "中心一枚播放盘说「点了会播」,左下角一枚芯片说「这是视频/GIF」。"),
    "E": ("E 纯图形", "只有一枚没有底盘的高亮播放三角,任何尺寸下都不抢海报。"),
}


def page(variant):
    title, note = NOTES[variant]
    return f'''<!doctype html><html lang="zh-CN"><head><meta charset="utf-8">
<style>
  body {{ margin:0; padding:36px 40px 44px; background:#f8f5f1; color:#1d1a17; font:13px/1.5 {FONT}; }}
  h1 {{ font-size:19px; margin:0 0 6px; font-weight:650; }}
  p.note {{ margin:0 0 26px; color:#6f665d; max-width:760px; }}
  .row {{ display:flex; gap:22px; align-items:flex-start; margin-bottom:34px; }}
  .label {{ font:600 12px {FONT}; color:#7c736a; margin:0 0 10px; }}
</style></head><body>
  <h1>{title}</h1>
  <p class="note">{note}</p>
  <div class="label">网格卡片</div>
  <div class="row">{card("photo", variant)}{card("video", variant)}{card("gif", variant)}</div>
  <div class="row">{single(variant)}</div>
  <div class="row">{grid(variant)}</div>
</body></html>'''


for key in NOTES:
    open(os.path.join(OUT, f"mediamark-{key}.html"), "w").write(page(key))
    print("wrote", f"mediamark-{key}.html")
