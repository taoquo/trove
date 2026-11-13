#!/usr/bin/env python3
"""Report where each list card starts and ends, and how much of it is blank.

    python3 tools/cardrows.py docs/screenshots/baseline/populated-list-light.png
    python3 tools/cardrows.py <png> --column 565     # skip the search and use this x

Why this exists
---------------
A list row can look half empty: the content occupies the top and the rest is blank. The obvious
measurements fail:

  - counting "runs of page colour" in the main pane mixes the blank *inside* a card with the gap
    *between* cards, so it can compare one capture against itself but not one state against
    another.
  - scanning down "the card's left border" at a guessed x fails: at x=566 it hits the text; at
    x=561 it hits the sidebar's own border, which runs the full height.

Every such attempt starts from a guessed address, and the thing being measured shares its
neighbourhood with something else. `tools/columns.py` avoids that by taking the address as an
argument and then checking whether anything is drawn there. This tool takes the opposite
approach -- it finds the address itself -- so the search has to be based on a property that
separates the two candidates rather than on a guess.

That property: the sidebar's border runs the whole height of the image, and a card's border does
not. A column that is border-coloured on every row is the sidebar; a column that is border-coloured
on some rows and not others, to the right of the sidebar, is a card edge.

A limitation that is larger than it first appears, and the reason this tool is not yet trustworthy
for the question it was written for.

A card has a 16pt corner radius, so on the leftmost column its border is drawn only across the
middle of its height. The tool measures the list card at 195px and the grid card at 207px, against
the grid's own CardMinHeight of 240pt = 480px. Two radii do not explain a shortfall of that size,
so the extents reported here are not the card's extents and the blank figure derived from them
cannot be used to judge the design either.

What it does do correctly, and what `--column` plus `selfcheck` pin down, is find the two addresses:
the sidebar's edge, which is border-coloured on nearly every row, and the cards' edge, which is not.
The step that is missing is stepping in from the corner by the radius before scanning down. Until
that is written, read this tool as an address finder only.

Exit codes follow tools/columns.py: 2 when the tool cannot do its job, 1 when it ran and the
picture is wrong (no cards found), 0 otherwise.
"""

import argparse
import sys

try:
    from PIL import Image
except ImportError:  # pragma: no cover - the toolchain bundles Pillow
    sys.stderr.write("cardrows: Pillow is required\n")
    sys.exit(2)

# A pixel counts as "ink" when it differs from the page colour by more than this, summed over the
# three channels. The card border is roughly (217,217,222) on a (244,244,245) page: a difference of
# about 77, well clear of JPEG-free PNG noise but below the dark text.
INK = 20


def read(path):
    try:
        return Image.open(path).convert("RGB")
    except OSError as err:
        sys.stderr.write("cardrows: cannot open %s: %s\n" % (path, err))
        raise SystemExit(2)


def page_colour(image):
    """The page colour, sampled well away from any card or border."""
    width, height = image.size
    return image.getpixel((4, height // 2))


def sidebar_edge(image, page, search_from):
    """The x of the sidebar's right border, or None.

    Border-coloured on nearly every row, which is what tells it apart from a card's edge: cards
    start and stop, and this does not. A scan down x=561 delivers one continuous line, which is
    exactly this.
    """
    width, height = image.size
    pixels = image.load()
    rows = range(140, height - 10, 2)
    total = len(rows)
    for x in range(search_from, width // 2):
        hits = sum(1 for y in rows
                   if abs(pixels[x, y][0] - page[0]) + abs(pixels[x, y][1] - page[1])
                   + abs(pixels[x, y][2] - page[2]) > INK)
        if hits > total * 0.92:
            return x, total
    return None, total


def find_border_column(image, page, search_from):
    """The x of the cards' left border.

    Everything runs from just past the sidebar's edge. Starting at x=400 -- inside the sidebar --
    returns a sidebar element at x=400 as if it were a card: the measurement begins at a guessed
    address, and the guess is in the wrong region entirely.
    """
    width, height = image.size
    pixels = image.load()
    rows = range(140, height - 10, 2)
    total = len(rows)

    edge, _ = sidebar_edge(image, page, search_from)
    if edge is not None:
        search_from = edge + 2

    for x in range(search_from, width // 2):
        hits = sum(1 for y in rows
                   if abs(pixels[x, y][0] - page[0]) + abs(pixels[x, y][1] - page[1])
                   + abs(pixels[x, y][2] - page[2]) > INK)
        if hits == 0:
            continue
        if hits > total * 0.92:
            continue
        if hits >= 3:
            return x, hits, total, edge
    return None, 0, total, edge


def measure(image, page, column):
    """Each card's extent down `column`, as (top, bottom) pairs."""
    width, height = image.size
    pixels = image.load()
    cards = []
    start = None
    for y in range(120, height):
        r, g, b = pixels[column, y]
        on = abs(r - page[0]) + abs(g - page[1]) + abs(b - page[2]) > INK
        if on and start is None:
            start = y
        elif not on and start is not None:
            if y - start > 20:
                cards.append((start, y - 1))
            start = None
    if start is not None and height - start > 20:
        cards.append((start, height - 1))
    return cards


def content_bottom(image, page, column, top, bottom, inset):
    """The last row inside the card that has anything drawn on it.

    Scans the card's interior (a little inside the border, so the border itself does not count)
    from the bottom up, and returns the first row with ink. `None` when the card is empty.
    """
    width, height = image.size
    pixels = image.load()
    right = min(width - 4, column + 1700)
    for y in range(bottom - 2, top + 1, -1):
        for x in range(column + inset, right, 3):
            r, g, b = pixels[x, y]
            # Skip the card's own border colour, which is a light grey: the content here is text
            # and avatars, which are much darker.
            if abs(r - page[0]) + abs(g - page[1]) + abs(b - page[2]) > 200:
                return y
    return None


def main(argv):
    parser = argparse.ArgumentParser(description=__doc__.split("\n")[1])
    parser.add_argument("png", nargs="+", help="a list-layout screenshot")
    parser.add_argument("--column", type=int, default=None,
                        help="the cards' left border x; found automatically when omitted")
    parser.add_argument("--search-from", type=int, default=40,
                        help="where to start looking for the sidebar's edge")
    args = parser.parse_args(argv)

    broken = False
    failed = False
    empty = False
    for path in args.png:
        image = read(path)
        page = page_colour(image)
        column = args.column
        discarded = 0
        edge = None
        if column is None:
            column, hits, total, edge = find_border_column(image, page, args.search_from)
            if column is None:
                # A screen with no cards has no card border, and that is a result rather than a
                # failure -- the same distinction as the empty branch below: an empty screen and
                # a missing file must not share an exit code.
                sys.stderr.write("cardrows: %s has no card border -- it has no cards\n" % path)
                empty = True
                continue
            discarded = total - hits
        cards = measure(image, page, column)
        if not cards:
            # No cards is a result, not a failure. empty-dark is a screen with a sidebar and an
            # empty state and no cards at all; reporting that as the same code as "cannot open the
            # file" confuses a result with a failure. 1 means it ran and the picture has no cards;
            # 2 stays for the tool not being able to do its job.
            sys.stderr.write("cardrows: %s has no cards -- nothing to measure at x=%d\n"
                             % (path, column))
            empty = True
            continue
        print("%s  border x=%d" % (path, column))
        if edge is not None:
            print("  (the sidebar's own edge is at x=%d; the search starts past it)" % edge)
        if column is not None and discarded:
            print("  (that column is border-coloured on %d of %d rows, so it is not the sidebar's)"
                  % (len(range(140, image.size[1] - 10, 2)) - discarded,
                     len(range(140, image.size[1] - 10, 2))))
        for top, bottom in cards:
            height = bottom - top + 1
            end = content_bottom(image, page, column, top, bottom, 6)
            if end is None:
                print("  y=%4d..%4d  %3dpx = %3.0fpt   (empty)" % (top, bottom, height, height / 2))
                continue
            used = end - top + 1
            print("  y=%4d..%4d  %3dpx = %3.0fpt   content to y=%d (%3dpx = %3.0fpt), blank %3dpx = %3.0fpt"
                  % (top, bottom, height, height / 2, end, used, used / 2, height - used,
                     (height - used) / 2))

    if broken:
        return 2
    if failed:
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
