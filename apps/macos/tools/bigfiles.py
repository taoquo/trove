"""Reports non-test Go files that have grown past a single responsibility.

Round 238 found internal/ui/shell.go at 5419 lines holding every surface the window draws:
twenty-five painting methods, the state, the sync actions, the bundle actions and the settings
form, in one file. Spec 15.1 forbids that shape, and nothing in this repository was measuring it
-- the eight other checkers all ask whether something is *connected*, and a file can be entirely
connected and still be unreadable.

The split brought the largest file to 808 lines across thirty files, and this keeps it there.

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
# Three backend files were already over the cap when this check was written, and each is one
# job rather than a bundled pair: the cap catches "a file doing several things", and these are
# long because a job is long. The UI package, which is where the round-238 problem was, has
# nothing over 808.
#
# They are listed rather than exempted by path prefix so that the list is visible and so that a
# file that grows past the cap later still has to be argued for.
ALLOWED = {
    # The catalogue itself: nine functions over a thousand lines of message pairs.
    "internal/i18n/i18n.go": "the message catalogue, one entry per string",
    # One pipeline, twenty-four steps of it, read top to bottom in the order they run.
    "internal/autotag/pipeline.go": "the auto-tag pipeline, in the order its steps run",
    # The read model for the whole workspace: every query the three columns ask for.
    "internal/usecase/workspace/service.go": "the workspace read model, one method per query",
}


def go_files():
    seen = set()
    for root in ROOTS:
        for base, dirs, files in os.walk(root):
            dirs[:] = [d for d in dirs if d not in ("node_modules", ".git", "build")]
            for name in sorted(files):
                if not name.endswith(".go") or name.endswith("_test.go"):
                    continue
                path = os.path.normpath(os.path.join(base, name))
                if path in seen:
                    continue
                seen.add(path)
                yield path


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
    # Round 326 ran every checker and found five -- this one among them -- printing nothing when
    # clean, because the summary sat behind --verbose. "No output" then means both "nothing wrong"
    # and "the check did not run", which is the shape round 287 lost a round to: eighteen
    # screenshots all reported FAIL because a wrong path and a missing separator looked the same.
    # The count costs one line and tells the two apart.
    print("bigfiles: %d files measured, %d over the cap" % (measured, over), file=sys.stderr)
    return 0


if __name__ == "__main__":
    sys.exit(main())
