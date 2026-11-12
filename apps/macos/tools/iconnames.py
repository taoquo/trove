"""Lists icon names the shell asks for that the icon set does not have.

An unknown name draws nothing, and an empty box looks like a rendering problem rather
than a typo, so this failed silently until the footer's three buttons appeared blank.

    python3 tools/iconnames.py [--names]

The icon set is internal/ui/testdata/icons.json, generated from src/ so it matches the
source's Phosphor set.
"""
import json
import os
import re
import sys

ICONS = "internal/ui/testdata/icons.json"
CALLS = ("IconChip", "IconOnly", "IconToggle", "IconPrimaryChip", "drawIcon")


def known_names():
    data = json.load(open(ICONS, encoding="utf-8"))
    names = data["icons"] if isinstance(data, dict) and "icons" in data else data
    return set(names) if isinstance(names, dict) else set(names)


# The tools in this directory resolve their inputs relative to apps/macos, and eight of them used
# to answer from wherever they were run: `twins.py` reported nothing from the repo root and six
# findings from here, `bigfiles.py` measured 1176 files from the root and 1 from here, `i18ncheck`
# reported 4 lines from the root and 97 from here. A wrong directory produced something that looked
# like a result, which is the shape this project keeps meeting: "no output" and "nothing to report"
# read the same.
#
# Round 331 checked every caller -- all eight shell scripts here do `cd "$(dirname "$0")/.."`, so
# nothing was actually misreading. This guard is for the next caller, who will not know that.
_here = os.path.dirname(os.path.abspath(__file__))
if os.path.basename(os.getcwd()) != "macos":
    sys.stderr.write(
        "warning: %s is being run from %s; it reads paths relative to apps/macos.\n"
        "         cd apps/macos first, or the numbers below will be about something else.\n"
        % (os.path.basename(__file__), os.getcwd()))


def main():
    known = known_names()
    pattern = re.compile(
        r"(?:%s)\([^)]*?\"([a-z][a-zA-Z]*)\"" % "|".join(CALLS))
    missing = {}
    for base, _, files in os.walk("internal/ui"):
        for name in files:
            if not name.endswith(".go") or name.endswith("_test.go"):
                continue
            path = os.path.join(base, name)
            for number, line in enumerate(open(path, encoding="utf-8"), 1):
                for match in pattern.finditer(line):
                    if match.group(1) not in known:
                        missing.setdefault(match.group(1), []).append(f"{path}:{number}")

    if "--names" in sys.argv:
        for name in sorted(missing):
            print(name)
    else:
        print(f"{len(known)} icons known; {len(missing)} names asked for that do not exist")
        for name, where in sorted(missing.items()):
            print(f"  {name!r} at {', '.join(where[:3])}")
    return 1 if missing else 0


if __name__ == "__main__":
    raise SystemExit(main())
