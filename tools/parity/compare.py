#!/usr/bin/env python3
"""Compares the source render against the native render.

    python3 tools/parity/compare.py --state workspace --width 1297

Both pictures come from the same fixture at the same size: the source side from
render-source.mjs, the native side from render-native.sh. The report is made of
numbers a person can check -- where a label sits, what a surface's colour is --
because "looks different" is what produced six rounds of drift.

Labels are matched by text and, for repeated text (an author name appears in the
sidebar and on the card), by their order down the page. That pairing is the only
guess in the file, and it is stable for a fixed fixture.
"""

import argparse
import json
from collections import Counter, defaultdict
from pathlib import Path

from PIL import Image

ROOT = Path(__file__).resolve().parents[2]
PARITY = ROOT / "docs/parity"


def dominant(im, box):
    crop = im.crop(box).convert("RGB")
    return Counter(crop.getdata()).most_common(1)[0][0]


def near(a, b, tolerance=4):
    return all(abs(a[i] - b[i]) <= tolerance for i in range(3))


def split_line(im, scale):
    """Where the sidebar's ground gives way to the results ground.

    Two dominant colours are sampled from the two halves, then the scan looks for
    the first column after x=300 that belongs to the right-hand colour and stays
    there. The split handle is 12px wide, so "stays there" is what skips it.
    """
    width, height = im.size
    left = dominant(im, (0, int(200 * scale), int(180 * scale), int(500 * scale)))
    right = dominant(im, (width - int(120 * scale), int(40 * scale), width, int(120 * scale)))
    if near(left, right, 6):
        return None
    run = 0
    for x in range(int(300 * scale), width):
        if near(im.getpixel((x, int(400 * scale))), right, 4):
            run += 1
            if run >= int(20 * scale):
                return round((x - int(20 * scale) + 1) / scale, 1)
        else:
            run = 0
    return None


def source_rects(state, width):
    path = PARITY / f"source-{state}-{width}.json"
    if not path.exists():
        return {}
    boxes = json.loads(path.read_text())
    wanted = {
        "sidebar": lambda e: e.get("tid") == "lists-sidebar",
        "main": lambda e: e.get("tid") == "library-workspace",
        "toolbar shell": lambda e: "options-toolbar-shell" in (e.get("cls") or ""),
        "header": lambda e: e.get("tid") == "library-header-section",
        "first card": lambda e: "options-result-card" in (e.get("cls") or ""),
        "pending bar": lambda e: e.get("tid") == "suggestions-actions-bar",
    }
    out = {}
    for name, match in wanted.items():
        for element in boxes:
            if match(element):
                out[name] = (element["x"], element["y"], element["w"], element["h"])
                break
    return out


def text_box(box):
    """The rect a label is drawn in, not the rect of the element that holds it.

    The native side reports the box MyGo drew the text in. The source side reports
    elements, and an element's box includes its padding, so a padded badge or button
    looked offset by exactly that padding (the status badge read +22/+8 while both
    sides drew it in the same place). The harness adds `tr` -- the range rect of the
    element's own text -- for exactly this comparison; fall back to the element box
    for nodes that only hold other elements.
    """
    tr = box.get("tr")
    if tr:
        return {**box, "x": tr["x"], "y": tr["y"], "w": tr["w"], "h": tr["h"]}
    return box


def paired(boxes):
    """text -> list of rects, ordered down the page then across."""
    groups = defaultdict(list)
    for box in boxes:
        text = (box.get("t") or "").strip()
        if not text or len(text) > 60:
            continue
        groups[text].append(text_box(box))
    for text, items in groups.items():
        # An ancestor's textContent is the same string as its child's, so the dump holds
        # both. Keep the leaves when there are any: they are the elements that actually
        # draw the text, and the ancestors would otherwise count as repeats.
        leaves = [b for b in items if b.get("tr")]
        if leaves:
            items = leaves
        items.sort(key=lambda b: (round(b["y"] / 8), b["x"]))
        groups[text] = items
    return groups


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--state", default="workspace")
    parser.add_argument("--width", type=int, default=1297)
    args = parser.parse_args()

    source_png = PARITY / f"source-{args.state}-{args.width}.png"
    native_png = PARITY / f"native-{args.state}-{args.width}.png"
    if not source_png.exists():
        raise SystemExit(f"missing {source_png}; run bash tools/parity/render.sh")
    if not native_png.exists():
        raise SystemExit(f"missing {native_png}; run bash tools/parity/render-native.sh")
    source_json = PARITY / f"source-{args.state}-{args.width}.json"
    native_json = PARITY / f"native-{args.state}-{args.width}.json"

    source = Image.open(source_png).convert("RGB")
    native = Image.open(native_png).convert("RGB")
    scale = source.size[0] / args.width
    native_scale = native.size[0] / args.width

    rects = source_rects(args.state, args.width)

    print(f"# {args.state} @ {args.width}px\n")
    print("| measurement | source | native | Δ |")
    print("|---|---|---|---|")

    def row(label, a, b):
        delta = "—" if a is None or b is None else f"{b - a:+.1f}"
        print(f"| {label} | {a if a is not None else '?'} | {b if b is not None else '?'} | {delta} |")

    if "main" in rects:
        row("main content x (source box tree)", rects["main"][0] + 32, None)

    print("\n## surfaces\n")
    print("| region | source | native |")
    print("|---|---|---|")
    regions = {
        "results ground": (int(args.width * 0.62), 60, int(args.width * 0.72), 100),
        "pending bar": (int(args.width * 0.45), 215, int(args.width * 0.55), 240),
        "card body": (int(args.width * 0.30), 430, int(args.width * 0.34), 470),
        "sidebar footer": (int(args.width * 0.02), int(args.width * 0.0) + 750,
                           int(args.width * 0.08), 790),
    }
    for name, box in regions.items():
        sb = tuple(int(v * scale) for v in box)
        nb = tuple(int(v * native_scale) for v in box)
        if sb[2] < source.size[0] and sb[3] < source.size[1] and nb[2] < native.size[0] and nb[3] < native.size[1]:
            print(f"| {name} | {dominant(source, sb)} | {dominant(native, nb)} |")

    if rects:
        print("\n## source geometry\n")
        for name, rect in rects.items():
            print(f"- {name}: x={rect[0]} y={rect[1]} w={rect[2]} h={rect[3]}")

    if not source_json.exists() or not native_json.exists():
        return

    source_groups = paired(json.loads(source_json.read_text()))
    native_groups = paired(json.loads(native_json.read_text()))

    rows = []
    skipped = 0
    for text, natives in native_groups.items():
        sources = source_groups.get(text)
        if not sources:
            continue
        # MyGo's Tester.Find returns the first label with a text, so a repeated
        # label cannot be addressed by position. Pairing only what both sides
        # render once keeps the table honest; the cards are compared by picture.
        if len(natives) != 1 or len(sources) != 1:
            skipped += 1
            continue
        rows.append((text, sources[0], natives[0]))

    rows.sort(key=lambda r: (r[1]["y"], r[1]["x"]))
    print("\n## text anchors (same fixture, same size)\n")
    print("| label | src x,y | native x,y | Δx | Δy |")
    print("|---|---|---|---|---|")
    worst = []
    for text, src, item in rows:
        dx = item.get("x", 0) - src["x"]
        dy = item.get("y", 0) - src["y"]
        if abs(dx) > 2 or abs(dy) > 2:
            worst.append((abs(dx) + abs(dy), text, dx, dy))
        print(
            f"| {text.replace(chr(10), ' ')[:26]} | {src['x']},{src['y']} | "
            f"{round(item.get('x', 0), 1)},{round(item.get('y', 0), 1)} | {dx:+.1f} | {dy:+.1f} |"
        )

    worst.sort(reverse=True)
    print(f"\n{len(rows)} unique anchors, {len(worst)} off by more than 2px "
          f"({skipped} repeated labels skipped).")
    if worst:
        print("\nlargest offsets:")
        for total, text, dx, dy in worst[:10]:
            print(f"- {text.replace(chr(10), ' ')[:40]}: Δx={dx:+.1f} Δy={dy:+.1f}")


if __name__ == "__main__":
    main()
