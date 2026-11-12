"""Lists constants that mean the same thing but are spelled differently.

Round 218 found two sets of width bounds: internal/usecase/settings clamped and persisted
against 260..420, internal/ui against 180..320, and a stored 400 was accepted by one layer and
silently shrunk by the other. Neither name appears twice, so a name-based check would not have
seen it -- the pair is `WidthSidebarMin` and `MinSidebarWidth`.

The check therefore compares names by their words rather than their spelling: split the
CamelCase, lowercase, sort the parts. `WidthSidebarMin` and `MinSidebarWidth` both become
minsidebarwidth, which is what makes them findable.

It reports candidates, not findings. Two constants can share a word set and mean different
things -- `OutcomeOK` ("ok") and the i18n key for the same word are the standing example --
so a group whose members hold different VALUES is marked, and one whose members agree is not:
two names for one number is the defect, and two names that disagree is the evidence.

    python3 tools/twins.py            # groups whose members disagree
    python3 tools/twins.py --all      # every group
"""
import os
import re
import sys

ROOT = "internal"
# `Name = value` and `Name Type = value`.
DECL = re.compile(r"^\s+([A-Z]\w*)\s+(?:[\w\.\[\]\*]+)\s*=\s*([^/\n]+)", re.M)
PLAIN = re.compile(r"^\s+([A-Z]\w*)\s*=\s*([^/\n]+)", re.M)
WORDS = re.compile(r"[A-Z]+(?![a-z])|[A-Z][a-z0-9]*")


def key_of(name):
    """A name reduced to its words, so spelling order does not matter."""
    return "".join(sorted(w.lower() for w in WORDS.findall(name)))


def declarations():
    for base, _, names in os.walk(ROOT):
        for name in names:
            if not name.endswith(".go") or name.endswith("_test.go"):
                continue
            path = os.path.join(base, name)
            text = open(path, encoding="utf-8").read()
            for match in list(DECL.finditer(text)) + list(PLAIN.finditer(text)):
                yield match.group(1), match.group(2).strip().rstrip(","), path


# Groups whose members hold different values and are not the same thing. Each needs a reason;
# the list is meant to be uncomfortable to add to.
ALLOWED = {
    # The engine's outcome value and the i18n key for the sentence shown about it. Same words,
    # different kinds of thing -- one is a stored string, the other a catalogue key.
    "okoutcome": "engine outcome \"ok\" against the i18n key for its message",
    "disabledllmoutcome": "the same pair, for the disabled-by-setting outcome",
}


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
    show_all = "--all" in sys.argv
    groups = {}
    for name, value, path in declarations():
        groups.setdefault(key_of(name), []).append((name, value, path))

    disagreeing = []
    for key, members in sorted(groups.items()):
        if len({m[0] for m in members}) < 2:
            continue
        # A group where one member is an i18n key string or a colour literal is not the same
        # kind of thing twice; comparing the values is what separates them.
        values = {m[1] for m in members}
        # The detail goes to stderr so stdout carries only the keys the gate compares.
        if show_all or len(values) > 1:
            print(key, file=sys.stderr)
            for name, value, path in members:
                mark = " <-- differs" if len(values) > 1 else ""
                print("    %s = %s   (%s)%s" % (name, value, path, mark), file=sys.stderr)
        if len(values) > 1 and key not in ALLOWED:
            disagreeing.append(key)

    if show_all:
        print("\n%d groups, %d whose members disagree" % (len(groups), len(disagreeing)),
              file=sys.stderr)
    # stdout is the gate's input: the keys that differ and are not allowlisted.
    for key in disagreeing:
        print(key)
    return 0


if __name__ == "__main__":
    sys.exit(main())
