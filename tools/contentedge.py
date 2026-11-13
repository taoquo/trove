#!/usr/bin/env python3
"""Checks that the middle pane's content stops at the right edge the shell declares.

    python3 tools/contentedge.py docs/screenshots/baseline/populated-grid-light.png --right-edge 1776

Why this is not tools/columns.py
--------------------------------
columns.py asks whether a boundary is *drawn*: it looks for a rule at a given x and fails when paint
crosses it. This asks a different question -- whether content runs *past* the edge it should stop
at -- and the two are not interchangeable. Running columns.py with the content's right edge (1776)
as the address fails on both a healthy capture and a mutated one, because neither has a rule there,
and a rule is what columns.py looks for. One measurement must not be used for another.

Where the address comes from
----------------------------
The right edge is not chosen. It follows from the shell's layout: at a window of W points the
middle pane ends at W - 12, and at 1180 points that is 1176 points, which is 2352 pixels on a 2x
capture. Below 1400 points the detail is an overlay rather than a pane, so the middle pane ends
earlier than the window does. The caller supplies the number for the capture's own width, and
--column-layout says so explicitly rather than leaving it implied.

How the content edge is taken
-----------------------------
Per row, the ground is that row's most common colour, and content is the last pixel that differs
from it. Two exclusions matter, both established by measuring rather than by reasoning:

* The window's own frame sits at the right of every row -- a (217,217,222) rule then white outside
  the window -- so an unclipped search returns W-1 on every picture. --frame is how many pixels to
  ignore at the right.
* The ground differs by row: the toolbar's ground is white, the card area's is (244,244,245). A
  single hard-coded reference makes the toolbar's background read as content. Taking each row's
  own mode avoids that, and it is the same move contrast.py makes.

The blind band, measured rather than guessed
--------------------------------------------
The search stops --frame pixels short of the right edge, because the window's own border is there.
At 1180 points the shell puts the edge at 2352 and the picture is 2360 wide, so the search runs
to x=2353 and the last six columns are never read. A bar painted at each of
2300 / 2340 / 2353 / 2354 / 2355 / 2358 is seen as follows:

    2300, 2340   inside -- correctly, both are left of 2352
    2353         over by 1, exit 1
    2354, 2355, 2358   not seen at all

So exactly one column past the edge is visible and the six nearest the border are not. That is
smaller than it sounds for the question this asks: an overrun that reaches the window's edge passes
through 2353 on its way and is caught there. A bar at 2500 is outside a 2360-wide picture, so there
is no content in the search window to find. What is genuinely invisible is content confined to the
last six columns without touching the one before them, and nothing is painted there.

The limit is "visible within --frame", not "no blind spot"; this is the measurement.

Exit codes -- four situations, and they must not share one
----------------------------------------------------------
    0  the content stops at or before the edge
    1  the content runs past it
    2  the tool could not do its job (unreadable file, no Pillow)
    3  the picture has no content in the pane to measure

3 rather than 0 because a capture with nothing in the pane has not been shown to be within the
edge; and not 2, because the file was fine. cardrows.py draws the same distinction, for the same
reason.
"""

import argparse
import sys
from collections import Counter

try:
    from PIL import Image
except ImportError:  # pragma: no cover - the toolchain bundles Pillow
    sys.stderr.write("contentedge: Pillow is required\n")
    sys.exit(2)


def content_right(image, left, frame, threshold):
    """The median over rows of the last pixel that differs from that row's own ground."""
    width, height = image.size
    pixels = image.load()
    rights = []
    for y in range(0, height, 9):
        row = [pixels[x, y] for x in range(left, width - frame)]
        if not row:
            continue
        ground = Counter(row).most_common(1)[0][0]
        for i in range(len(row) - 1, -1, -1):
            c = row[i]
            if abs(c[0] - ground[0]) + abs(c[1] - ground[1]) + abs(c[2] - ground[2]) > threshold:
                rights.append(left + i)
                break
    if not rights:
        return None
    rights.sort()
    return rights[len(rights) // 2]


def main(argv):
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("png", nargs="+", help="a capture")
    parser.add_argument("--right-edge", type=int, required=True,
                        help="the x, in image pixels, where the middle pane should end")
    parser.add_argument("--left", type=int, default=560,
                        help="the x where the pane starts (the sidebar's 280pt at 2x)")
    parser.add_argument("--frame", type=int, default=6,
                        help="pixels to ignore at the right, where the window's own frame is")
    parser.add_argument("--threshold", type=int, default=30,
                        help="how far a pixel must differ from the row's ground to count")
    parser.add_argument("--tolerance", type=int, default=0,
                        help="how far past the edge still counts as inside (default: exact)")
    args = parser.parse_args(argv)

    broken = False
    overrun = False
    empty = False
    for path in args.png:
        try:
            image = Image.open(path).convert("RGB")
        except OSError as err:
            sys.stderr.write("contentedge: cannot open %s: %s\n" % (path, err))
            broken = True
            continue

        right = content_right(image, args.left, args.frame, args.threshold)
        if right is None:
            sys.stderr.write("contentedge: %s has no content between x=%d and x=%d\n"
                             % (path, args.left, image.size[0] - args.frame))
            empty = True
            continue

        past = right - args.right_edge
        verdict = "over" if past > args.tolerance else "inside"
        print("%s  content right = %d  declared = %d  %s by %d"
              % (path, right, args.right_edge, verdict, abs(past)))
        if past > args.tolerance:
            overrun = True

    if broken:
        return 2
    if empty:
        return 3
    return 1 if overrun else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
