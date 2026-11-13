"""Checks that docs/ui-surface-index.md lists every drawing method in the shell.

The index exists because a second bulk bar can be built without noticing the first one; a
hand-maintained list is exactly the kind of thing that goes stale, and a stale list costs -- a
table saying "not started" about work that was finished sends the next reader to redo it.

So this checks the one property that can be checked: every method that draws a surface is in
the table, and every name in the table is a method that exists. It does not check whether the
"when it appears" column is right -- that is prose, and a script that tried would be guessing.

    python3 tools/surfaces.py            # missing and unknown names, one per line
    python3 tools/surfaces.py --verbose  # a summary

A method draws a surface when it takes a *ui.Context first, which is the shape every drawing
method in shell.go has and the shape no other method has.
"""
import os
import re
import sys

# The package, not one file. Drawing methods move between files; reading one file reports them as
# "in the index but draws nothing", correctly, because that file no longer holds them. A surface
# is a method on Shell; which file it sits in is not something the index should have to track.
UI_DIR = "internal/ui"


def repo_root():
    """Where docs/ lives. The tools run from the repository root and the index is at the root."""
    here = os.path.dirname(os.path.abspath(__file__))
    while here != "/":
        if os.path.isfile(os.path.join(here, "go.mod")) and os.path.isdir(os.path.join(here, "docs")):
            return here
        here = os.path.dirname(here)
    raise SystemExit("surfaces: could not find the repository root above this file")


INDEX = os.path.join(repo_root(), "docs", "ui-surface-index.md")


def drawing_methods():
    source = package_source()
    pattern = re.compile(r"^func \(s \*Shell\) ([a-z]\w*)\(c \*ui\.Context", re.M)
    return sorted(set(pattern.findall(source)))


def listed():
    if not os.path.isfile(INDEX):
        return None
    names = set()
    for line in open(INDEX, encoding="utf-8"):
        # A table row whose first cell is a code span: | `name` | ...
        match = re.match(r"^\|\s*`(\w+)`\s*\|", line)
        if match:
            names.add(match.group(1))
    return names


def animated_states():
    """The State fields that gate a surface through EntranceProgress.

    A panel's opacity comes from root.Animate, whose clock is advanced by frames.
    CapturePage takes a snapshot without pumping them, so these surfaces cannot be
    photographed -- which is why the index names them and this compares the two.
    """
    source = package_source()
    # EntranceProgress(root, "key", then s.state.Field on the next line.
    return sorted(set(re.findall(r"EntranceProgress\(root,[^\n]*\n\s*s\.state\.(\w+)", source)))


def documented_animated():
    """The same fields as the index lists them, from its "cannot be photographed" table."""
    if not os.path.isfile(INDEX):
        return set()
    text = open(INDEX, encoding="utf-8").read()
    marker = "四个拍不到的表面"
    if marker not in text:
        return None
    section = text[text.index(marker):]
    section = section[:section.index("\n## ")] if "\n## " in section else section
    # Rows of the form: | `method` | `StateField` | ... |
    return set(re.findall(r"^\|\s*`\w+`\s*\|\s*`(\w+)`", section, re.M))


def test_functions():
    """Every test function name in the ui package."""
    names = set()
    for base, _, files in os.walk("internal/ui"):
        for name in files:
            if not name.endswith("_test.go"):
                continue
            text = open(os.path.join(base, name), encoding="utf-8").read()
            names.update(re.findall(r"^func (Test\w+)\(", text, re.M))
    return names


def documented_tests():
    """The test each animated surface's row names, by field."""
    if not os.path.isfile(INDEX):
        return {}
    text = open(INDEX, encoding="utf-8").read()
    marker = "四个拍不到的表面"
    if marker not in text:
        return {}
    section = text[text.index(marker):]
    section = section[:section.index("\n## ")] if "\n## " in section else section
    found = {}
    for line in section.split("\n"):
        # | `panel` | `Field` | `Test Name` |
        match = re.match(r"^\|\s*`\w+`\s*\|\s*`(\w+)`\s*\|\s*(.+?)\s*\|", line)
        if match:
            found[match.group(1)] = match.group(2)
    return found


def package_source():
    """Every non-test .go file in the ui package, concatenated."""
    parts = []
    for base, _, files in os.walk(UI_DIR):
        for name in sorted(files):
            if name.endswith(".go") and not name.endswith("_test.go"):
                parts.append(open(os.path.join(base, name), encoding="utf-8").read())
    return "\n".join(parts)


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
    methods = set(drawing_methods())
    index = listed()
    if index is None:
        print("surfaces: docs/ui-surface-index.md is missing", file=sys.stderr)
        return 1

    missing = sorted(methods - index)
    unknown = sorted(index - methods)

    for name in missing:
        print("%s is not in the index" % name)
    for name in unknown:
        print("%s is in the index but draws nothing" % name)

    # A surface gated by EntranceProgress cannot be photographed, and the index has to say
    # which ones those are. A new one that is not written down is a surface nobody will know
    # to verify through Tester instead.
    animated = set(animated_states())
    documented = documented_animated()
    if documented is None:
        print("the index has no 'cannot be photographed' section")
    else:
        for field in sorted(animated - documented):
            print("%s gates a surface through EntranceProgress but the index does not say so"
                  % field)
        for field in sorted(documented - animated):
            print("%s is listed as animated but nothing gates it that way" % field)

        # And every row must name a test that exists. "未验" is not a value: a surface nobody
        # has verified and a surface that does not exist are the same thing as far as
        # verification goes.
        tests = test_functions()
        for field, cell in sorted(documented_tests().items()):
            named = re.findall(r"(Test\w+)", cell)
            if not named:
                print("%s has no test named in the index; 'unverified' is not a value" % field)
                continue
            for name in named:
                if name not in tests:
                    print("%s names %s, which is not a test in internal/ui" % (field, name))

    # Unconditional, unlike the per-item lines above: "no output" must not mean both "nothing
    # wrong" and "the check did not run".
    print("surfaces: %d drawing methods, %d listed, %d missing, %d unknown"
          % (len(methods), len(index), len(missing), len(unknown)), file=sys.stderr)
    return 0


if __name__ == "__main__":
    sys.exit(main())
