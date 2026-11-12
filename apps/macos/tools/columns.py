"""Checks that the pane boundary the shell contract declares is actually drawn.

Written in round 258, after eight rounds (246-255) of adjusting widths -- Width(304), Fill,
FillHeight, Spacer, Row.FillWidth, then arithmetic -- none of which moved the picture, because
the problem was never a width. Scanning a horizontal line across docs/screenshots/
255-inspector-fixed.png showed paint running unbroken from x=560 to x=2359 while the inspector's
declared boundary is at x=1752. The panes read as one surface, and the inspector's header looked
like it was overflowing the toolbar when in fact there was no boundary to overflow.

    "The inspector's header is not overflowing. It has no boundary."

Nothing in the suite could see this. 1054 tests and nine checkers verify values (is the token
256?) and wiring (is the function called?), and none of them can say what the shell's space is
supposed to look like.

What this checks
----------------

docs/shell-contract.md S3: the separator is a single 1px line at the track boundary. So at the
declared boundary x there must be a column that differs from the surface on both sides of it.

Two earlier criteria, both wrong, and both worth recording because the self-check is what found
them:

*The first took the background as the colour at x=1*, on the reasoning that the shell always
paints the sidebar's left edge. True of a screenshot and false of anything else: fed a picture
whose paint starts at x=0 it read the paint as the background and reported every band.

*The second looked for a gap* -- a stretch of background between the two panes. That is the
intuitive reading of "the panes are separated", and it cannot work here: every surface in this
shell is near-white. The results pane's ground is (244,244,245) and the window is (255,255,255),
a difference of 11. There is no gap to find, at any tolerance that does not also call text
background. That criterion was in place while the boundary *was* correctly drawn at x=1752 and
still reported failure -- the checker was wrong and the shell was right, which is the more
expensive of the two ways to be wrong.

So the check is for the rule itself. That is what the contract promises, it needs no guess about
what counts as background, and it is the same assertion whether the shell shows two panes or
three.

    python3 tools/columns.py docs/screenshots/<png> --boundary 1752
    python3 tools/columns.py <png> --boundary <x> [--bands 3] [--tolerance 6]

Exit code 0 when every band has a rule at the boundary, 1 when any band does not.
"""
import argparse
import sys

try:
    from PIL import Image
except ImportError:  # pragma: no cover - the toolchain bundles Pillow
    sys.stderr.write("columns: Pillow is not available\n")
    sys.exit(2)


def differs(a, b, tolerance):
    return abs(a[0] - b[0]) + abs(a[1] - b[1]) + abs(a[2] - b[2]) > tolerance


def rule_at(pixels, width, y0, y1, boundary, reach, tolerance):
    """Whether a separator stands at the boundary in this band.

    A separator is a column that differs from the surface a little way out on each side. Both
    sides are sampled at `reach` columns away rather than immediately adjacent, so the check
    survives an anti-aliased edge: the pixel next to a 1px rule is often a blend of the rule and
    the surface, and a blend beside a blend looks like no rule at all.

    It returns the number of rows in the band where such a column exists, so a band that is
    mostly the rule passes and a band where the rule is only one row of an icon does not.
    """
    rows = 0
    for y in range(y0, y1):
        left = pixels[max(0, boundary - reach), y][:3]
        here = pixels[boundary, y][:3]
        right = pixels[min(width - 1, boundary + reach), y][:3]
        if differs(here, left, tolerance) and differs(here, right, tolerance):
            rows += 1
    return rows


def main(argv):
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("png", nargs="+", help="a screenshot to scan")
    parser.add_argument("--boundary", type=int, required=True,
                        help="the x the pane boundary should be at, in image pixels")
    parser.add_argument("--bands", type=int, default=3,
                        help="how many horizontal bands to check (default 3)")
    parser.add_argument("--reach", type=int, default=4,
                        help="how far out to sample the surface on each side (default 4)")
    parser.add_argument("--tolerance", type=int, default=6,
                        help="how far a pixel must differ to count as a rule (default 6)")
    parser.add_argument("--min-rows", type=float, default=0.5,
                        help="the fraction of a band's rows that must show the rule (default 0.5)")
    parser.add_argument("--verbose", action="store_true")
    args = parser.parse_args(argv)

    # Two different kinds of trouble, and they must not share an exit code.
    #
    # Round 287 ran this over eighteen screenshots from the wrong directory. Every path was
    # missing, every run wrote "No such file" to stderr, the loop tested `[ -z "$out" ]` and so
    # reported all eighteen as FAIL -- and a missing path and a missing separator both exited 1,
    # so the caller had no way to tell them apart. The separator check was right the whole time.
    #
    # So: 2 means the tool could not do its job (a path that does not open, a boundary outside the
    # picture), and 1 means it did its job and the picture is wrong. A caller can now say
    # "if [ $? -eq 2 ]; then the tool is broken; else the check failed; fi".
    broken = False
    failed = False
    for path in args.png:
        try:
            image = Image.open(path).convert("RGB")
        except OSError as err:
            sys.stderr.write("columns: cannot open %s: %s\n" % (path, err))
            broken = True
            continue

        width, height = image.size
        if not 0 < args.boundary < width:
            sys.stderr.write("columns: cannot check %s: boundary %d is outside 0..%d\n"
                             % (path, args.boundary, width))
            broken = True
            continue

        pixels = image.load()
        band_height = max(1, height // (args.bands + 1))

        for index in range(args.bands):
            y0 = band_height * (index + 1)
            y1 = min(height, y0 + band_height)
            if y1 - y0 < 2:
                continue
            rows = rule_at(pixels, width, y0, y1, args.boundary, args.reach, args.tolerance)
            need = int((y1 - y0) * args.min_rows)
            if rows < need:
                sys.stderr.write(
                    "FAIL: %s band %d (y %d..%d): no separator at x=%d -- %d of %d rows show "
                    "one, %d needed; the panes read as one surface\n"
                    % (path, index, y0, y1, args.boundary, rows, y1 - y0, need))
                failed = True
            elif args.verbose:
                print("%s band %d (y %d..%d): separator at %d spans %d of %d rows"
                      % (path, index, y0, y1, args.boundary, rows, y1 - y0))

    if broken:
        return 2
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
