"""Reports non-test Go files that have grown past a single responsibility.

A file can hold every surface the window draws -- the painting methods, the state, the sync
actions, the bundle actions and the settings form -- in one place. That shape is forbidden, and
the other checkers all ask whether something is *connected*; a file can be entirely connected and
still be unreadable.

The cap keeps the largest file at a readable size.

    python3 tools/bigfiles.py            # files over the cap
    python3 tools/bigfiles.py --verbose  # every file it measured

The cap is a threshold for "look at this", not a rule about style. A file that legitimately
needs to be longer can be listed in ALLOWED with a reason -- but the reason has to be about the
file, not about the deadline.
"""
import os
import sys

ROOTS = ["internal", "."]
CAP = 900
# The files below are over the cap because each is one job rather than a bundled pair: the cap
# catches "a file doing several things", and these are long because a job is long. The UI package
# has nothing over 808.
#
# They are listed rather than exempted by path prefix so that the list is visible and so that a
# file that grows past the cap later still has to be argued for.
ALLOWED = {
    # One pipeline, twenty-four steps of it, read top to bottom in the order they run.
    "internal/autotag/pipeline.go": "the auto-tag pipeline, in the order its steps run",
}


def go_files():
    seen = set()
    for root in ROOTS:
        for base, dirs, files in os.walk(root):
            dirs[:] = [d for d in dirs if d not in
                       (".git", ".toolchain", ".claude", ".codex", ".superpowers",
                        "docs", "testdata", "resources", "assets", "build", "dist")]
            for name in sorted(files):
                if not name.endswith(".go") or name.endswith("_test.go"):
                    continue
                path = os.path.normpath(os.path.join(base, name))
                if path in seen:
                    continue
                seen.add(path)
                yield path


# The tools in this directory resolve their inputs relative to the repository root. A wrong
# directory produces something that looks like a result, which is the shape this project keeps
# meeting: "no output" and "nothing to report" read the same.
#
# All the shell scripts here do `cd "$(dirname "$0")/.."`; this guard is for the next caller, who
# will not know that.
_here = os.path.dirname(os.path.abspath(__file__))
if os.path.abspath(os.getcwd()) != os.path.dirname(os.path.dirname(os.path.abspath(__file__))):
    sys.stderr.write(
        "warning: %s is being run from %s; it reads paths relative to the repository root.\n"
        "         cd to the repository root first, or the numbers below will be about something else.\n"
        % (os.path.basename(__file__), os.getcwd()))


def main():
    verbose = "--verbose" in sys.argv
    over = 0
    measured = 0
    for path in sorted(go_files()):
        length = len(open(path, encoding="utf-8").read().split("\n"))
        measured += 1
        if path in ALLOWED:
            if verbose:
                print("%-46s %5d  allowed (%s)" % (path, length, ALLOWED[path]))
            continue
        if length > CAP:
            print("%s is %d lines, over the %d cap: %s"
                  % (path, length, CAP, "it is doing more than one job"))
            over += 1
        elif verbose:
            print("%-46s %5d" % (path, length))
    # Unconditional, unlike the per-item lines above.
    #
    # A checker whose summary sits behind --verbose prints nothing when clean, so "no output"
    # means both "nothing wrong" and "the check did not run". The count costs one line and tells
    # the two apart.
    print("bigfiles: %d files measured, %d over the cap" % (measured, over), file=sys.stderr)
    return 0


if __name__ == "__main__":
    sys.exit(main())
