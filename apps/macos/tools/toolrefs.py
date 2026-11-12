"""Checks that `path:line` references in docs/ point at lines that exist.

Written after adding two rows to the plan's form-differences table. Every other table in this
project has drifted at least once -- rounds 190, 191, 218 and 223 each found one saying "not
started" about finished work, or naming a test that did not exist -- and the reference in a
citation is the part of prose a script can check.

It does not check whether the citation is *about* the right thing. A line number that exists
and points at the wrong code is exactly as invisible to this as it is to a reader, and only
reading the source settles that.

    python3 tools/toolrefs.py            # references that do not resolve
    python3 tools/toolrefs.py --verbose  # every reference it looked at
"""
import os
import re
import sys

DOCS = "docs"
# A dated log, not a description of the current tree. A citation in it means "this is where the
# code was when I wrote this", so a later move invalidates it as history, not as a claim. The
# check that matters is on the documents that describe the tree as it is now.
HISTORICAL = {"implementation-log.md"}
# Files this repository does not contain but a doc legitimately cites: MyGo's own sources,
# which live in the module cache, and magpie's shell, which is the visual and interaction
# authority for rounds 531-532 (docs/magpie-visual-spec.md, docs/interaction-parity-plan.md)
# and lives only in github.com/yetone/magpie. An unresolved name that is not here is reported,
# because otherwise "cite a file that does not exist" would pass silently -- which the first
# version of the mutation check showed is exactly what a silent skip buys. For these the trade
# runs the other way: the line numbers cannot be checked here, so each doc pins the commit
# instead, and every citation is one line rather than a range the pattern below would silently
# drop. A `.js`/`.html` citation is admitted for the same reason a `.css` one is: magpie's
# behaviour is recorded in app.js and its surfaces in index.html, and a doc that could not name
# them would be citing from memory.
EXTERNAL = {"api_gen.go", "app.css", "app.js", "base.go", "client.js", "collapsible.go", "element.go", "feedback.go", "index.html", "menu.go", "paint.go", "runtime.go", "scope.go", "window.go"}
# `shell.go:4971`, `BookmarkResultsPane.tsx:40`, `copy.ts:159/473`.
REF = re.compile(r"`([A-Za-z0-9_./-]+\.(?:go|ts|tsx|css|js|html)):(\d+)(?:/(\d+))?`")
# Where a cited source file might live, since a doc names it by basename. These are resolved
# against ROOT, not against the working directory: the first version used "../src", which from
# apps/macos is apps/src -- so nothing in the extension resolved, and the search fell through
# to unrelated files with the same basename (a 142-line shell.go stood in for a 5,400-line
# one, and every reference to the real file was reported as past its end).
SEARCH_ROOTS = ["src", "apps/macos/internal", "apps/macos"]


def repo_root():
    here = os.path.dirname(os.path.abspath(__file__))
    while here != "/":
        if os.path.isdir(os.path.join(here, "src")) and os.path.isdir(os.path.join(here, "docs")):
            return here
        here = os.path.dirname(here)
    raise SystemExit("toolrefs: could not find the repository root above this file")


ROOT = repo_root()


def resolve(name):
    """The real path behind a citation, or None.

    A doc may cite a path (`src/options/utils.ts`) or a bare basename (`utils.ts`). The path
    form is resolved directly; the basename is searched for, and the longest match wins --
    preferring src/lib/storage/settings.ts over any other settings.ts is what keeps a citation
    from being checked against the wrong file.
    """
    if os.path.isabs(name):
        return name if os.path.isfile(name) else None
    direct = os.path.join(ROOT, name)
    if os.path.isfile(direct):
        return direct
    matches = []
    for root in SEARCH_ROOTS:
        for base, dirs, files in os.walk(os.path.join(ROOT, root)):
            dirs[:] = [d for d in dirs if d != "node_modules"]
            if os.path.basename(name) in files:
                matches.append(os.path.join(base, os.path.basename(name)))
    return matches or None


def lines_in(path):
    return len(open(path, encoding="utf-8").read().split("\n"))


def resolves(name, line):
    """Whether the citation points into a real file.

    A bare basename can match several files: `index.ts` lives in a dozen directories. Picking
    one and calling the rest wrong is what the second version did, and it reported 11 findings
    that were the resolver's fault rather than the document's. The question a reader asks is
    "does this citation land somewhere real", so any match having the line is a yes.
    """
    found = resolve(name)
    if found is None:
        return None
    if isinstance(found, str):
        found = [found]
    for path in found:
        if line <= lines_in(path):
            return path
    # None of them has the line; report against the longest one, which is the best guess.
    return max(found, key=lines_in)


def main():
    verbose = "--verbose" in sys.argv
    checked = 0
    bad = 0
    for base, _, files in os.walk(os.path.join(ROOT, DOCS)):
        for name in sorted(files):
            if not name.endswith(".md") or name in HISTORICAL:
                continue
            path = os.path.join(base, name)
            text = open(path, encoding="utf-8").read()
            for match in REF.finditer(text):
                cited, line = match.group(1), int(match.group(2))
                checked += 1
                real = resolves(cited, line)
                if real is None:
                    if os.path.basename(cited) in EXTERNAL:
                        if verbose:
                            print("%s: %s is outside this repository" % (name, cited))
                        continue
                    print("%s: %s does not resolve" % (name, cited))
                    bad += 1
                    continue
                if line > lines_in(real):
                    print("%s: %s:%d is past the end of the file (%d lines)"
                          % (name, cited, line, lines_in(real)))
                    bad += 1
                elif verbose:
                    print("%s: %s:%d ok" % (name, cited, line))
    # Unconditional, unlike the per-item lines above.
    #
    # Round 326 ran every checker and found five -- this one among them -- printing nothing when
    # clean, because the summary sat behind --verbose. "No output" then means both "nothing wrong"
    # and "the check did not run", which is the shape round 287 lost a round to: eighteen
    # screenshots all reported FAIL because a wrong path and a missing separator looked the same.
    # The count costs one line and tells the two apart.
    print("toolrefs: %d references, %d bad" % (checked, bad), file=sys.stderr)
    return 0


if __name__ == "__main__":
    sys.exit(main())
