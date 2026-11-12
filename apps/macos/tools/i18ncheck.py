"""Lists strings the extension shows that this shell's catalog does not carry.

The inventory has been wrong twice in ways nothing checked: it treated lists as a source
feature that does not exist, and it recorded saved views as having no UI when the source
renders one. Both were found by reading. A string the source shows and this shell cannot
render is the same kind of gap, and it is enumerable.

    python3 tools/i18ncheck.py
    python3 tools/i18ncheck.py --names

Matching is by the English string, not by key name. The first version compared key names
and reported 180 missing keys -- every one of them a false positive, because this catalog
namespaces its keys ("action.cancel") while the source uses flat ones ("cancelLabel").
Comparing names measured the naming scheme, not whether a string can be shown.

The source tree is read as the authority; popup/ and content/ are excluded because the
goal's surface does not include them.
"""
import os
import re
import sys

SOURCE = "../../src"
EXCLUDED = ("content/", "popup/")  # the goal's surface excludes both
CATALOG = "internal/i18n/i18n.go"


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

def source_files():
    found = []
    for base, _, names in os.walk(SOURCE):
        for name in names:
            if not (name.endswith(".ts") or name.endswith(".tsx")):
                continue
            if ".test." in name:
                continue
            path = os.path.join(base, name)
            # Relative to src/, so "components/" cannot match "content/".
            relative = path.replace(os.sep, "/").split("src/", 1)[-1]
            if relative.startswith(EXCLUDED):
                continue
            found.append(path)
    return found


# `copy.foo` in the source, which is how every user-visible string is reached.
used = set()
for path in source_files():
    for match in re.finditer(r"\bcopy\.([a-zA-Z][a-zA-Z0-9]*)", open(path, encoding="utf-8").read()):
        used.add(match.group(1))

# The source's English values for those keys.
copy_source = open(os.path.join(SOURCE, "options/i18n/copy.ts"), encoding="utf-8").read()
first = copy_source.index('  return {', copy_source.index('if (locale === "zh-CN")'))
second = copy_source.index('  return {', first + 1)
english = dict(re.findall(r'^\s+([a-zA-Z][a-zA-Z0-9]*):\s*"((?:[^"\\]|\\.)*)"', copy_source[second:], re.M))

# Every value this catalog can render, in either locale.
#
# Placeholders are normalised before comparing. The source writes {suggested} and this
# catalog writes %d, so the same sentence looked different and every count-bearing string
# was reported as missing -- which is a large part of the 146 found before this was fixed.
def normalize(text):
    text = re.sub(r"\{[a-zA-Z][a-zA-Z0-9]*\}", "\x00", text)
    text = re.sub(r"%[dsv]", "\x00", text)
    return text


declared_values = {
    normalize(value)
    for value in re.findall(r':\s*"((?:[^"\\]|\\.)*)"', open(CATALOG, encoding="utf-8").read())
}

# A key the source reaches but whose string this catalog never holds.
missing = sorted(
    key for key in used
    if key in english and english[key] and normalize(english[key]) not in declared_values
)

if "--names" in sys.argv:
    for key in missing:
        print(key)
    raise SystemExit(0)

print(f"{len(used)} keys used in {SOURCE} (popup/ and content/ excluded)")
print(f"{len(declared_values)} distinct strings in {CATALOG}")
print(f"{len(missing)} used whose English string the catalog does not hold\n")
for key in missing:
    print(f"  {key:36s} {english[key]!r}")
