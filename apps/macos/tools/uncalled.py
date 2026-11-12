"""Lists exported methods on the usecase layer that nothing calls.

Written after round 89 found a command that had existed for fifty rounds with no way to
reach it. Finding those one at a time is what lets one survive, so this walks the source.

    python3 tools/uncalled.py

The check is deliberately simple, because two cleverer versions were both wrong:

  * counting only internal/ui and main.go reported CreateTag unreachable while
    AddTagsToSelection calls it;
  * excluding the whole defining file reported HealInterrupted unreachable while
    CleanupOrphanedRuns calls it in the same file;
  * matching uses per line then reported MergeTags and RenameTag unreachable, which are
    plainly called.

So: a method counts as reached when the text `.Name(` appears on any line that is not the
line declaring it. That is crude and it is right, which is the trade this needs.

It reports candidates, not findings.
"""
import os
import re
import sys

# "." rather than "main.go": the root package has more than one file, and naming only
# main.go left the others unscanned. A method called from capture.go was reported as having
# no caller for exactly that reason.
ROOTS = ["internal", "."]
# The layers whose exported methods must be reachable. internal/ui is here because a
# method on Shell with no call site is invisible otherwise: two checkers reported a clean
# tree while Export, AskBundleImport and three other bundle actions had no button -- they
# were outside the only layer being swept.
LAYERS = ["internal/usecase", "internal/ui"]


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
            if name.endswith(".go") and not name.endswith("_test.go"):
                found.append(os.path.join(base, name))
    return sorted(found)


# Both forms. The method-only version missed round 208's orphan: `func Options(...)` is a
# plain function, so a whole type and its constructor sat unreachable and the gate was green.
# A function with no receiver is exported from the same layers and is reachable the same way,
# so it belongs in the same sweep.
definition = re.compile(r"^func (?:\([^)]*\) )?([A-Z]\w*)\(", re.M)

defined = {}
for layer in LAYERS:
    for path in go_files(layer):
        source = open(path, encoding="utf-8").read()
        for match in definition.finditer(source):
            defined.setdefault(match.group(1), []).append(path)

lines_by_file = {path: open(path, encoding="utf-8").read().split("\n") for path in go_files(ROOTS[0]) + go_files(ROOTS[1])}


# A one-line function puts its call on the same line as its declaration --
# `func (s *Shell) OpenFirstMediaInDetail() { s.OpenLightbox(0) }` -- so skipping every
# line that starts with "func" reported OpenLightbox as unreachable. Only the declaration
# is skipped, not the body beside it.
# The receiver group is optional: a plain function has the same shape without it, and the
# method-only version left `func Options(...)` in place to match its own declaration.
declaration = re.compile(r"^func (?:\([^)]*\)\s*)?[A-Z]\w*\([^)]*\)\s*")


def reached(name):
    # A call is `.Name(` for a method and `Name(` for a function in the same package. Requiring
    # the dot reported Rail, IsDark and EntranceProgress -- all plainly called -- as
    # unreachable, because a package-level call carries no receiver.
    pattern = re.compile(r"(?:\.|\b)%s\(" % re.escape(name))
    for path, lines in lines_by_file.items():
        for line in lines:
            stripped = line.lstrip()
            if stripped.startswith("func "):
                stripped = declaration.sub("", stripped)
            if pattern.search(stripped):
                return path
    return ""


uncalled = sorted(name for name in defined if not reached(name))
only_outside_ui = sorted(
    name for name in defined
    if reached(name) and not reached(name).startswith("internal/ui") and reached(name) != "main.go"
)

if "--names" in sys.argv:
    # One name per line, for the gate that watches the set rather than reading it.
    for name in uncalled:
        print(name)
    raise SystemExit(0)

print(f"{len(defined)} exported methods across {', '.join(LAYERS)}")
print(f"{len(uncalled)} are called from nowhere in {', '.join(ROOTS)}\n")
for name in uncalled:
    print(f"  {name:28s} {defined[name][0]}")
print(f"\n{len(only_outside_ui)} are reached only from outside the interface or main:")
for name in only_outside_ui:
    print(f"  {name:28s} from {reached(name)}")
