#!/usr/bin/env python3
"""Check the select checkbox reads as a control when it sits on a photograph.

    python3 tools/selecttoggle.py [--shot docs/screenshots/criteria/card-media-1180.png]

The select toggle over a card with top media gets a translucent panel, a white ring and a drop
shadow, because an outline drawn on a picture disappears into it. This is the repeatable check for
that treatment, so a later change to the toggle cannot quietly drop the parts that are hardest to
see.

What it measures, and why each is a real property rather than a pixel count:

  - fill   the box interior is lighter than the picture below it, by a wide margin: that is the
           82% panel over the ring. An empty box on a photo cannot pass.
  - ring   the pixel just inside the box's edge is lighter than the fill, i.e. a lighter hairline
           sits between the picture and the panel.
  - shadow the rows under the box are darker than the same picture far to the right at the same
           height, and the difference shrinks with distance. Measuring against the picture rather
           than against an absolute value is what keeps this independent of the seed gradient.

Exit 0 pass, 1 fail, 2 cannot run.
"""

import argparse
import sys
from pathlib import Path

try:
    from PIL import Image
except ImportError:
    print("cannot run: Pillow is missing", file=sys.stderr)
    sys.exit(2)

ROOT = Path(__file__).resolve().parent.parent
DEFAULT = ROOT / "docs/screenshots/criteria/card-media-1180.png"

# The first card's checkbox, in the PNG's 2x pixels.
#
# Measured rather than derived: arithmetic from the card's padding gives 604..652, which is
# outside the box, and reports both the ring and the shadow missing. Scanning for the bright run
# gives 605..642 by 435..476.
BOX = (604, 435, 643, 477)


def luminance(px):
    return 0.299 * px[0] + 0.587 * px[1] + 0.114 * px[2]


def main(argv):
    parser = argparse.ArgumentParser()
    parser.add_argument("--shot", default=str(DEFAULT))
    args = parser.parse_args(argv)

    path = Path(args.shot)
    if not path.exists():
        print(f"cannot run: {path} is missing -- run: python3 tools/cardmedia.py", file=sys.stderr)
        return 2

    im = Image.open(path).convert("RGB")
    x0, y0, x1, y1 = BOX
    cx = (x0 + x1) // 2
    cy = (y0 + y1) // 2
    far_x = x1 + 300  # same height, well clear of the box

    picture = luminance(im.getpixel((cx, y1 + 60)))
    # The ring, measured rather than assumed: scanning across the left edge at y=456 gives
    # (91,87,107) for the picture at 603, (226,225,228) for the ring at 604-605, and (217,216,221)
    # for the panel from 606 inward. So the ring is the first two pixels of the box and the fill
    # starts at the third, not across the whole first few pixels.
    ring = luminance(im.getpixel((x0, cy)))
    fill = luminance(im.getpixel((x0 + 8, cy)))

    fail = 0
    if fill - picture < 40:
        print(f"FAIL the box interior ({fill:.0f}) is not clearly lighter than the picture "
              f"({picture:.0f}); the panel fill is missing")
        fail += 1
    if ring - fill < 6:
        print(f"FAIL no lighter hairline at the box edge (edge {ring:.0f} vs fill {fill:.0f}); "
              f"the white ring is missing")
        fail += 1

    # The shadow: darker under the box than the picture at the same height, fading with distance.
    deltas = []
    for dy in (1, 4, 8):
        under = luminance(im.getpixel((cx, y1 + dy)))
        aside = luminance(im.getpixel((far_x, y1 + dy)))
        deltas.append(aside - under)
    if deltas[0] < 2:
        print(f"FAIL no shadow under the box (under is {deltas[0]:.1f} darker than the picture)")
        fail += 1
    elif not (deltas[0] >= deltas[1] >= deltas[2] >= 0):
        print(f"FAIL the shadow does not fade with distance: deltas {deltas}")
        fail += 1

    if fail:
        return 1
    print(f"  ok   fill +{fill - picture:.0f} over the picture, ring +{ring - fill:.0f}, "
          f"shadow {deltas[0]:.1f}/{deltas[1]:.1f}/{deltas[2]:.1f} away from the box")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
