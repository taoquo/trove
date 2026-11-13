"""Lists i18n keys this catalog declares that nothing renders.

A key can be replaced by a new label and left defined and unused. None of the three existing
checkers covers it: uncalled.py walks methods, unread.py walks State and View fields,
earlyreturn.py walks control flow. A catalog is a fourth place a gap can hide.

    python3 tools/orphankeys.py

A key counts as used when its Go name appears anywhere outside the catalog file. Names
used only in tests count as used, which is deliberate: some keys exist for tests to assert
the catalog itself.
"""
import os
import re
import sys

CATALOG = "internal/i18n/i18n.go"
SEARCH = ["internal", "cmd/trove", "tools"]


# The tools here read their inputs relative to the repository root. A tool run from anywhere else
# used to answer anyway, and a wrong directory produced something that looked like a result. The
# shell callers pin the directory; this guard is for the next caller, who will not know that.
import os as _os, sys as _sys
if _os.path.abspath(_os.getcwd()) != _os.path.dirname(_os.path.dirname(_os.path.abspath(__file__))):
    _sys.stderr.write(
        "warning: %s is being run from %s; it reads paths relative to the repository root.\n"
        "         cd to the repository root first, or the numbers below will be about something else.\n"
        % (_os.path.basename(__file__), _os.getcwd()))

def go_files(root):
    if os.path.isfile(root):
        return [root]
    found = []
    for base, _, names in os.walk(root):
        for name in names:
            if name.endswith(".go"):
                found.append(os.path.join(base, name))
    return sorted(found)


catalog = open(CATALOG, encoding="utf-8").read()
declared = set(re.findall(r"^\t([A-Za-z][A-Za-z0-9]*)\s+Key = ", catalog, re.M))

others = []
for root in SEARCH:
    for path in go_files(root):
        if path == CATALOG:
            continue
        others.append(open(path, encoding="utf-8").read())
haystack = "\n".join(others)

unused = sorted(name for name in declared if not re.search(r"\b%s\b" % re.escape(name), haystack))

if "--names" in sys.argv:
    for name in unused:
        print(name)
    raise SystemExit(0)

print(f"{len(declared)} keys declared; {len(unused)} rendered nowhere\n")
for name in unused:
    print(f"  {name}")
