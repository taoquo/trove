"""Lists i18n keys this catalog declares that nothing renders.

Round 116 replaced an invented label with the source's, leaving the old key defined and
unused. None of the three existing checkers covers it: uncalled.py walks methods,
unread.py walks State and View fields, earlyreturn.py walks control flow. A catalog is a
fourth place a gap can hide.

    python3 tools/orphankeys.py

A key counts as used when its Go name appears anywhere outside the catalog file. Names
used only in tests count as used, which is deliberate: some keys exist for tests to assert
the catalog itself.
"""
import os
import re
import sys

CATALOG = "internal/i18n/i18n.go"
SEARCH = ["internal", "main.go", "tools"]


# The tools here read their inputs relative to apps/macos. Eight of them used to answer from
# wherever they were run -- `i18ncheck` reported 4 lines from the repo root and 97 from here,
# `orphankeys` 4 and 7 -- and a wrong directory produced something that looked like a result.
# Round 331 checked the callers: all eight shell scripts do `cd "$(dirname "$0")/.."`, so nothing
# was misreading. This is for the next caller, who will not know that.
import os as _os, sys as _sys
if _os.path.basename(_os.getcwd()) != "macos":
    _sys.stderr.write(
        "warning: %s is being run from %s; it reads paths relative to apps/macos.\n"
        "         cd apps/macos first, or the numbers below will be about something else.\n"
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
