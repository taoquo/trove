"""Lists stylesheet tokens that nothing in the extension uses but this build does.

Round 206 found --main-bg by hand: both blocks define it, nothing in src/ references it, and
the results pane was drawing it. The rule it should have used -- .options-main-shell names
--options-page-bg -- was two hex steps away, which is why no amount of looking at the
interface shows it.

The check runs in the other direction from every other tool here. The others ask "does my
side match the source"; this one asks "does the source use what my side does". That needs no
component reading: one pass over the two blocks gives the candidates, and only the ones this
build also references are interesting.

    python3 tools/deadtokens.py            # dead tokens this build also uses
    python3 tools/deadtokens.py --all      # every dead token, with whether this build uses it

A token counts as used when `var(--name)` appears anywhere in src/, or when the bare name
appears in a .ts/.tsx file -- the overlay backdrop and accent-text are referenced from
TypeScript and never from CSS, and a CSS-only scan reported both as dead.

It reports candidates, not findings: a token can be dead in the extension and still be the
right thing for a native surface that has no counterpart, which is an allowlist entry.
"""
import os
import re
import sys

# The tools run from apps/macos and the stylesheet lives at the repository root, so both are
# found by walking up rather than by assuming a working directory.
def repo_root():
    here = os.path.dirname(os.path.abspath(__file__))
    while here != "/":
        if os.path.isfile(os.path.join(here, "src", "styles", "extension.css")):
            return here
        here = os.path.dirname(here)
    raise SystemExit("deadtokens: could not find src/styles/extension.css above this file")


ROOT = repo_root()
CSS = os.path.join(ROOT, "src", "styles", "extension.css")
SRC = os.path.join(ROOT, "src")


def blocks():
    """The two token blocks, by line range.

    Not by index(':root {'): the file has more than one, and matching the wrong one made the
    light values read as the dark ones -- the mistake that nearly wrote a "the two themes
    agree" assertion in round 198.
    """
    lines = open(CSS, encoding="utf-8").read().split("\n")
    light_at = next(i for i, l in enumerate(lines) if l.strip() == ":root {")
    dark_at = next(i for i, l in enumerate(lines) if l.strip().startswith(':root[data-theme="dark"]'))

    def end_of(start):
        return next(i for i in range(start, len(lines)) if lines[i].strip() == "}")

    def parse(start):
        body = "\n".join(lines[start:end_of(start)])
        return dict(re.findall(r"--([a-z0-9-]+):\s*([^;]+);", body))

    return parse(light_at), parse(dark_at)


def native_tokens():
    """The token names this build names in a comment or a field, i.e. the ones it means to use.

    theme.go carries the source token beside every value -- `// --options-page-bg` -- which is
    the only machine-readable statement of which tokens this side intends to render.
    """
    names = set()
    for base, _, file_names in os.walk("internal/ui"):
        for name in file_names:
            if not name.endswith(".go") or name.endswith("_test.go"):
                continue
            text = open(os.path.join(base, name), encoding="utf-8").read()
            # Only a comment that IS the token name, which is how theme.go labels each value:
            # `// --panel-bg-strong`, nothing after it. A looser pattern -- `//\s*--name` --
            # was tried and reported four tokens that appear in prose instead: the comment
            # explaining that --main-bg is NOT used names it, and so does the one explaining
            # what theme.Selection was wrongly set to.
            names.update(re.findall(r"//\s*--([a-z0-9-]+)\s*$", text, re.M))
    return names


def referenced():
    """Every token name the extension's own code references."""
    names = set()
    for base, _, file_names in os.walk(SRC):
        for name in file_names:
            if not name.endswith((".css", ".ts", ".tsx")):
                continue
            text = open(os.path.join(base, name), encoding="utf-8").read()
            names.update(re.findall(r"var\(--([a-z0-9-]+)\)", text))
            # A bare mention in TypeScript counts: those are the tokens set through inline
            # styles, which never appear as var() anywhere.
            if name.endswith((".ts", ".tsx")):
                names.update(re.findall(r"--([a-z0-9-]+)", text))
    return names


# Tokens that are dead in the extension and deliberately kept here. Each needs a reason, and
# the list is expected to be uncomfortable to add to.
ALLOWED = {
    # The extension defines both and uses neither. this build did too, until round 210: the
    # only references were the two fields deleted in round 204 and a constant feeding MyGo's
    # text-selection highlight, which should keep MyGo's own default.
}


def main():
    show_all = "--all" in sys.argv
    light, dark = blocks()
    dead = sorted(set(light) - referenced() | (set(dark) - referenced()))
    if not dead:
        print("deadtokens: no dead tokens -- has the stylesheet moved?", file=sys.stderr)
        return 0

    mine = native_tokens()
    interesting = []
    for name in dead:
        used_here = name in mine
        if show_all:
            print("--%s\tused here=%s\tlight=%s\tdark=%s"
                  % (name, used_here, light.get(name, "-").strip(), dark.get(name, "-").strip()))
        # Only the ones this build means to render: a dead token it never mentions is the
        # extension's business, not a mismatch.
        if used_here and name not in ALLOWED:
            interesting.append(name)

    if not show_all:
        for name in interesting:
            print(name)
    # Always say how many were found, and how many of them are known.
    #
    # Round 324 ran this looking for dead tokens, got no output, and read that as "none" -- while
    # the answer was six, all either allowlisted or not used by this build. Printing nothing and
    # finding nothing are the same output, which is the shape round 317 found in the exit codes:
    # a checker with a two-level result has to say both levels.
    print("deadtokens: %d dead, %d without an allowlist entry" % (len(dead), len(interesting)),
          file=sys.stderr)
    if show_all:
        print("", file=sys.stderr)
    return 0


if __name__ == "__main__":
    sys.exit(main())
