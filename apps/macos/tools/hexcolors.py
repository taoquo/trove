"""Checks that cinnabar and every other bare colour live in one file.

The goal's colour rule has two halves: interactive colours -- selection, focus, the primary
button, progress -- follow the system accent, and cinnabar lives in exactly one place,
Tokens.BrandCinnabar. Both halves are about where a colour is written down, and neither is
visible in a screenshot: a hardcoded cinnabar and the theme's accent are the same pixels until
the system accent changes.

Round 284 found two violations by reading, not by a tool. StatusBadge drew StatusRunning with
lightAccent/darkAccent -- a second copy of cinnabar -- and railTagAccent in taggraph.go drew the
graph's focused node with it too. Both took a bool and picked between two hardcoded values, so
neither could follow the system. Both were removed and both now take the accent the caller got
from c.Theme().

What this checks
----------------

Every ui.Hex(...) in internal/ui lives in theme.go. That file is the token table: it mirrors
extension.css's variables one for one, each with the variable's name in a trailing comment, and
it is where the theme is assembled from ui.LightTheme()/DarkTheme(). A Hex anywhere else is a
colour that no theme can reach.

    python3 tools/hexcolors.py            # bare colours outside theme.go, one per line
    python3 tools/hexcolors.py --all      # every file's count, including theme.go

Exit code 0 when theme.go is the only file with one, 1 otherwise.
"""
import os
import re
import sys

UI = "internal/ui"
TOKEN_FILE = "theme.go"
HEX = re.compile(r'\bui\.Hex\(\s*"(#[0-9a-fA-F]{3,8})"')


def go_files(root):
    for name in sorted(os.listdir(root)):
        if name.endswith(".go"):
            yield os.path.join(root, name)


def main(argv):
    here = os.path.dirname(os.path.abspath(__file__))
    root = os.path.join(os.path.dirname(here), UI)

    offenders = []
    counts = {}
    for path in go_files(root):
        name = os.path.basename(path)
        found = [(number, match.group(1))
                 for number, line in enumerate(open(path, encoding="utf-8").read().split("\n"), 1)
                 for match in [HEX.search(line)] if match]
        if found:
            counts[name] = len(found)
        if name == TOKEN_FILE or name.endswith("_test.go"):
            continue
        for number, value in found:
            offenders.append("%s:%d: ui.Hex(%s)" % (name, number, value))

    if "--all" in sys.argv:
        for name in sorted(counts):
            print("%-28s %d" % (name, counts[name]))
        return 0

    for line in offenders:
        print(line)
    # A count even when clean, for the reason round 326 gives the other four: otherwise "nothing
    # printed" and "the check did not run" are the same output.
    print("hexcolors: %d colours outside the token table" % len(offenders), file=sys.stderr)
    return 1 if offenders else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
