"""Lists exported methods on the usecase layer that nothing calls.

A command can exist with no way to reach it. Finding those one at a time is what lets one
survive, so this walks the source code.

    python3 tools/uncalled.py

The check is deliberately simple, because cleverer versions are wrong:

  * counting only internal/ui and cmd/trove reports CreateTag unreachable while
    AddTagsToSelection calls it;
  * excluding the whole defining file reports HealInterrupted unreachable while
    CleanupOrphanedRuns calls it in the same file;
  * matching uses per line reports MergeTags and RenameTag unreachable, which are
    plainly called.

So: a method counts as reached when the text `.Name(` appears on any line that is not the
line declaring it. That is crude and it is right, which is the trade this needs.

It reports candidates, not findings.
"""
import os
import re
import sys

# "." rather than "cmd/trove/main.go": the app package has more than one file, and naming
# only main.go leaves the others unscanned. A method called from capture.go is reported as
# having no caller for exactly that reason. The walker prunes the pinned toolchain and the
# data directories, which are inside the module root.
ROOTS = ["."]
# The layers whose exported methods must be reachable. internal/ui is here because a
# method on Shell with no call site is invisible otherwise: two checkers report a clean
# tree while Export, AskBundleImport and three other bundle actions have no button -- they
# are outside the only layer being swept.
LAYERS = ["internal/usecase", "internal/ui"]
WALK_SKIP = {".git", ".toolchain", ".claude", ".codex", ".superpowers", "docs", "testdata", "resources", "assets", "build", "dist"}


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
    for base, dirs, names in os.walk(root):
        dirs[:] = [d for d in dirs if d not in WALK_SKIP]
        for name in names:
            if name.endswith(".go") and not name.endswith("_test.go"):
                found.append(os.path.join(base, name))
    return sorted(found)


# Both forms. A method-only check misses a plain function: `func Options(...)` is not a method,
# so a whole type and its constructor can sit unreachable while the gate is green. A function with
# no receiver is exported from the same layers and is reachable the same way, so it belongs in the
# same sweep.
definition = re.compile(r"^func (?:\([^)]*\) )?([A-Z]\w*)\(", re.M)

defined = {}
for layer in LAYERS:
    for path in go_files(layer):
        source = open(path, encoding="utf-8").read()
        for match in definition.finditer(source):
            defined.setdefault(match.group(1), []).append(path)

lines_by_file = {path: open(path, encoding="utf-8").read().split("\n") for path in go_files(".")}


# A one-line function puts its call on the same line as its declaration --
# `func (s *Shell) OpenFirstMediaInDetail() { s.OpenLightbox(0) }` -- so skipping every
# line that starts with "func" reports OpenLightbox as unreachable. Only the declaration
# is skipped, not the body beside it.
# The receiver group is optional: a plain function has the same shape without it, and a
# method-only form would leave `func Options(...)` to match its own declaration.
declaration = re.compile(r"^func (?:\([^)]*\)\s*)?[A-Z]\w*\([^)]*\)\s*")


def reached(name):
    # A call is `.Name(` for a method and `Name(` for a function in the same package. Requiring
    # the dot reports Rail, IsDark and EntranceProgress -- all plainly called -- as
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
    if reached(name) and not reached(name).startswith("internal/ui") and not reached(name).startswith("cmd/trove/")
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
