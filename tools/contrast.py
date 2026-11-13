#!/usr/bin/env python3
"""Measures the WCAG contrast of text drawn in a capture, without being told where the text is.

    python3 tools/contrast.py docs/screenshots/baseline/populated-grid-light.png
    python3 tools/contrast.py <png> --min-ratio 4.5

Why this exists
---------------
Contrast is the part of visual work that nothing else measures:

    $ grep -rn "contrast" tools/ internal/ui/
    (nothing)

`tools/hexcolors.py` checks that colours come from the token table rather than bare hex, which is a
necessary condition for contrast and not a sufficient one: a token can hold a colour that reads
badly on its own background. So this measures the ratio itself, and reports the worst one it finds.

Finding the text instead of being told where it is
-------------------------------------------------
Scanning a guessed address is a trap: the card's left border sits at x=564 and the sidebar's edge
at x=560, one column away. The same trap is here in a worse form, because text has no fixed
address at all.

What separates text from background is variance. A patch of background is nearly one colour; a
patch with glyphs in it holds two populations -- the ground and the ink -- so it is bimodal and its
spread is large. So this tiles the picture, keeps the tiles whose darkest pixel is far enough from
their lightest, and for each takes the ground to be the most common colour and the ink to be the
darkest decile. It reports per tile, worst first, and exits 1 when anything is below the threshold.

Exit codes follow tools/columns.py: 2 when the tool cannot do its job, 1 when it ran and found
something below the threshold, 0 otherwise.

Known limits, stated rather than discovered later
-------------------------------------------------
* The threshold defaults to a computed rather than rounded figure: #85776f on #f4f4f5 is
  3.926646:1. Rounding it to 3.93 makes the build it was derived from report as "below" against a
  threshold of 3.93, because 3.9266 is below 3.93. A rounded threshold makes the build it was
  derived from fail.
  The muted text -- #85776f on the page -- measures 3.926646:1 against the page and 3.562067:1
  against a card. WCAG AA asks 4.5 for small text, so the design target does not meet it;
  measuring this build against AA would fail it for being faithful.
  This build's timestamp measures 4.40:1, which is better than the reference rather than worse.
  A difference in the other direction is the one worth reporting, and the threshold is set so that
  regressions below the reference show up.
* The ink is a single darkest sample, so the figure is a lower bound on the real ratio: antialiased
  edge pixels are lighter than the glyph core.
* An arbitrary tile size moves the verdict: 36 tiles below threshold at --tile 30, none at 40, none
  at 60, on the same picture. It comes from the smallest Font token in internal/ui/metrics.go --
  11pt doubled for the 2x capture and two lines tall, which is 32px at the time of writing. The
  tool reports the muted timestamp at 4.40:1, three tiles with ink (113,113,122) on ground
  (244,244,245). A parameter that changes the verdict is not a detail, so this one is derived
  rather than passed, the way columns.py takes its address from the layout.
* A spread test alone is too permissive: the sidebar's selected-row background (233,233,236)
  against the page (244,244,245) is a background boundary, not a glyph, yet it measures as text at
  1.10:1. Any two-tone tile passes the spread test, so a card edge, a selected row and a divider
  all qualify. What separates them is shape: glyphs are thin and scattered, a boundary is a
  straight run. Counting the tile's transitions along each row tells them apart.
* The ink estimator must stay on the glyphs: a tenth percentile and a mean of the darkest 3% both
  report ink == ground, because glyphs cover only a few percent of a tile; the minimum is the one
  that stays on them.
* It does no letter recognition. It says where the contrast is poor, not which string is there;
  pairing a tile with a string is still a reading a person does.
"""

import argparse
import os
import re
import sys
from collections import Counter

try:
    from PIL import Image
except ImportError:  # pragma: no cover - the toolchain bundles Pillow
    sys.stderr.write("contrast: Pillow is required\n")
    sys.exit(2)


def relative_luminance(rgb):
    def channel(v):
        v = v / 255.0
        return v / 12.92 if v <= 0.03928 else ((v + 0.055) / 1.055) ** 2.4
    r, g, b = rgb
    return 0.2126 * channel(r) + 0.7152 * channel(g) + 0.0722 * channel(b)


def ratio(a, b):
    la, lb = relative_luminance(a), relative_luminance(b)
    hi, lo = max(la, lb), min(la, lb)
    return (hi + 0.05) / (lo + 0.05)


def transitions(image, left, top, size, ground, tolerance=24):
    """The median number of ground<->ink changes along the tile's rows.

    This is what separates a glyph from a boundary: any two-tone tile passes a spread test, so the
    sidebar's selected row -- one straight edge between two greys -- is reported as text at 1.10:1. Along a row through glyphs the colour
    leaves and returns to the ground repeatedly; along a row across an edge it does so once.
    """
    pixels = image.load()
    counts = []
    for y in range(top, top + size, 3):
        inside = False
        changes = 0
        for x in range(left, left + size):
            c = pixels[x, y]
            off = abs(c[0] - ground[0]) + abs(c[1] - ground[1]) + abs(c[2] - ground[2]) > tolerance
            if off and not inside:
                changes += 1
            inside = off
        counts.append(changes)
    if not counts:
        return 0
    counts.sort()
    return counts[len(counts) // 2]


def tiles(image, size):
    """Every size x size tile whose content is dark enough and varied enough to be text."""
    width, height = image.size
    pixels = image.load()
    out = []
    for top in range(0, height - size, size):
        for left in range(0, width - size, size):
            counts = Counter()
            darkest = None
            lightest = None
            darkest_lum = 2.0
            lightest_lum = -1.0
            for y in range(top, top + size, 2):
                for x in range(left, left + size, 2):
                    c = pixels[x, y]
                    counts[c] += 1
                    lum = relative_luminance(c)
                    if lum < darkest_lum:
                        darkest_lum, darkest = lum, c
                    if lum > lightest_lum:
                        lightest_lum, lightest = lum, c
            if not counts:
                continue
            # Text needs two populations. A flat tile has one, and its spread is near zero.
            if lightest_lum - darkest_lum < 0.05:
                continue
            ground, ground_count = counts.most_common(1)[0]
            ordered = sorted(counts.elements(), key=relative_luminance)
            # The ground has to actually be a ground: most of the tile.
            if ground_count < len(ordered) * 0.5:
                continue
            # The ink is the sample furthest from the ground, not the darkest one.
            #
            # Darkest is the obvious choice and it is wrong half the time. On a light appearance
            # the glyphs are darker than their ground, so the minimum is the ink; on a dark
            # appearance they are lighter, so the minimum is the ground. Over empty-dark.png the
            # darkest choice reports "no text-like tile -- is it blank?" on a picture full of text,
            # because every tile compares the ground with itself in the other direction.
            #
            # Distance from the ground is the property that holds in both appearances: the tool
            # has to distinguish light from dark, and picking an end rather than a distance does
            # not.
            ink = max(ordered, key=lambda c: abs(relative_luminance(c) - relative_luminance(ground)))
            if ink == ground:
                continue
            # And it has to look like glyphs rather than one straight edge.
            if transitions(image, left, top, size, ground) < 3:
                continue
            out.append((ratio(ink, ground), ink, ground, left, top))
    return out


def main(argv):
    parser = argparse.ArgumentParser(description=__doc__.split("\n")[1])
    parser.add_argument("png", nargs="+", help="a capture")
    parser.add_argument("--tile", type=int, default=None,
                        help="tile size in pixels; defaults to a size derived from the smallest "
                             "font in internal/ui/metrics.go")
    parser.add_argument("--min-ratio", type=float, default=3.926646,
                        help="the threshold to report against. The default is the reference "
                             "design's muted text: --text-tertiary #85776f on the page #f4f4f5 is "
                             "3.926646:1. WCAG AA's 4.5 is not this project's target -- the "
                             "reference is, and it does not meet AA either.")
    parser.add_argument("--top", type=int, default=12, help="how many tiles to list")
    args = parser.parse_args(argv)

    if args.tile is None:
        args.tile = tile_from_metrics()
        print("contrast: tile %d px, from the smallest Font token in internal/ui/metrics.go"
              % args.tile)

    broken = False
    failed = False
    for path in args.png:
        try:
            image = Image.open(path).convert("RGB")
        except OSError as err:
            sys.stderr.write("contrast: cannot open %s: %s\n" % (path, err))
            broken = True
            continue

        found = tiles(image, args.tile)
        if not found:
            sys.stderr.write("contrast: no text-like tile in %s -- is it blank?\n" % path)
            failed = True
            continue

        found.sort()
        # Equality passes. The default threshold *is* the reference design's ratio, so a build that
        # reproduces it lands exactly on the boundary. Without the epsilon the faithful build
        # reports 3.93 against a threshold of 3.93 as "below", because the arithmetic gives
        # 3.92999...  "At the reference value" and "below the reference value" are the two
        # situations this reports, and they must not share an outcome. The epsilon is one part in a
        # thousand of a ratio, far below any real change.
        epsilon = 1e-3
        below = [f for f in found if f[0] < args.min_ratio - epsilon]
        print("%s  %d text-like tiles, %d below %.1f:1" % (path, len(found), len(below), args.min_ratio))
        for value, ink, ground, left, top in found[:args.top]:
            mark = "  <-- below" if value < args.min_ratio - epsilon else ""
            print("  %.2f:1  ink=%s ground=%s  at (%d,%d)%s"
                  % (value, ink, ground, left, top, mark))
        if below:
            failed = True

    if broken:
        return 2
    return 1 if failed else 0


HERE = os.path.dirname(os.path.abspath(__file__))
METRICS = os.path.join(HERE, "..", "internal", "ui", "metrics.go")


def tile_from_metrics():
    """The tile size, taken from the smallest font this build draws rather than chosen.

    An arbitrary tile size moves the verdict -- 36 tiles below threshold at 30, none at 40 and
    none at 60, on the same picture -- and a parameter that changes the verdict means the verdict
    is not yet a property of the picture. The tile is sized to the text, and the text's size is in
    internal/ui/metrics.go: a tile two lines tall at the smallest font covers a few glyphs and
    little else. The capture is 2x, which is why the point size is doubled.
    """
    try:
        text = open(METRICS, encoding="utf-8").read()
    except OSError as err:
        sys.stderr.write("contrast: cannot read %s: %s\n" % (METRICS, err))
        raise SystemExit(2)
    sizes = [int(m) for m in re.findall(r'^\tFont[A-Za-z]+\s*=\s*(\d+)', text, re.M)]
    if not sizes:
        sys.stderr.write("contrast: no Font token in %s -- has it moved?\n" % METRICS)
        raise SystemExit(2)
    smallest = min(sizes)
    # One line of the smallest text at 2x, times two lines, plus a little for ascenders.
    return int(round(smallest * 2 * 1.6))


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
