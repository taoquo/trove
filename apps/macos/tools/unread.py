"""Lists fields on State and View that the interface never reads.

Round 98 found `View.Authors`: the service ran a SQL query to build the list on every
load, and no file in internal/ui ever read it. It was found by eye, which is how that
kind of field survives -- the compiler is happy, the tests pass, and the data is thrown
away.

    python3 tools/unread.py

A field counts as read when the accessor the interface actually uses reaches it -- for
State fields `<x>.state.<Field>`, for View fields `<x>.Data.<Field>` -- outside a line
that assigns to it.

Matching on the bare name was tried first and was useless: `View.Authors` looked read
because the *tag graph* has a field of the same name, so the tool reported no gap at all
even with every read of the real one deleted. Validating a tool against a mutation that
should trip it is the only way that shows up.

It reports candidates, not findings.

WHAT THIS DOES NOT COVER, because a green gate reads as "there are no orphans":

  * Only the three types in SOURCES -- State, View and RailTokens. Every other DTO that
    crosses a layer boundary is unchecked. There are nineteen structs in three files alone
    (usecase/workspace, domain/libraryquery, autotag/pipeline), and adding them wholesale
    would be broad and shallow: the pattern needs the accessor each one is reached through,
    and a guessed accessor produces false positives, which is how an allowlist stops meaning
    anything.
  * Fields read only through a map, reflection or a struct copy are reported as unread.
  * Fields read only by a test are reported as unread. That is deliberate -- a test is not
    the interface -- but it means a field kept for a test seam needs an allowlist entry.

The wider family this belongs to -- a value produced at one end and consumed at neither --
has been found seven times in this project (graphModel, vocabularyReview, tagLibrary,
mediaPicture, ShouldContinue, ValidVocabularyMode, CompareNaturalNames), and every one of
those was found by reading a module, not by a tool. This check closes one shape of it.
"""
import os
import re

UI = "internal/ui"
# type -> (defining file, the accessor the interface reaches it through)
# Round 238 split internal/ui/shell.go into thirty files. A type is no longer in a file whose
# name can be written down, so the file is found by searching the package for the declaration.
# Three checkers assumed one file and each had to be told the same thing; a type is a type
# wherever it is declared.
TYPE_DIRS = {
    'State': 'internal/ui',
    'RailTokens': 'internal/ui',
    'View': 'internal/usecase/workspace',
}


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

def file_declaring(type_name):
    """The file inside TYPE_DIRS that declares `type <name> struct`, or None."""
    needle = "type %s struct {" % type_name
    root = TYPE_DIRS[type_name]
    for path in go_files(root):
        if needle in open(path, encoding="utf-8").read():
            return path
    return None


SOURCES = {
    'State': (None, r"\.state\.%s\b"),  # one %s
    # A View also travels through locals -- `view.Generation` at the staleness guard --
    # so a Data-only pattern reported Generation as unread when it is read. The extra
    # requirement that the line mentions a view keeps the tag graph's own `Authors` field
    # from masking the real one.
    'View': (None, r"(\.Data\.%s\b)|(\bview\w*\.%s\b)"),
    # RailTokens is read as `rail.FieldName` in the shell, and as `tokens.FieldName` or
    # `light.FieldName`/`dark.FieldName` where a token set is built directly. Round 204 found
    # TagBg and TagText by hand: two fields assigned in both the light and the dark literal
    # and read by nothing.
    # The accessor is `Rail(...)` too: `return Rail(...).DisplayFace` reads a field with no
    # variable name in front of the dot, and a variable-only pattern missed it -- that false
    # positive is why the pattern carries the call form.
    'RailTokens': (None, r"(\b(rail|tokens|light|dark)\.%s\b)|(\)\.%s\b)"),
}


def go_files(root):
    found = []
    for base, _, names in os.walk(root):
        for name in names:
            if name.endswith(".go") and not name.endswith("_test.go"):
                found.append(os.path.join(base, name))
    return sorted(found)


def fields_of(path, type_name):
    source = open(path, encoding="utf-8").read()
    start = source.index("type %s struct {" % type_name)
    depth = 0
    names = []
    for line in source[start:].split("\n"):
        depth += line.count("{") - line.count("}")
        if depth <= 0 and names:
            break
        # A field line is `Name Type`, possibly with a comment above it.
        match = re.match(r"^\s*([A-Z]\w*)\s+[\[\*\w\.]", line)
        if match:
            names.append(match.group(1))
    return names


ui_lines = []
for path in go_files(UI):
    for line in open(path, encoding="utf-8").read().split("\n"):
        ui_lines.append((path, line))

assignment = re.compile(r"\.(\w+)\s*(?:=[^=]|\+=|-=|\*=|/=|\|=\s*|:=)")

import sys

every_unread = []

for type_name, (_named, pattern) in SOURCES.items():
    path = file_declaring(type_name)
    if path is None:
        print("%s is declared nowhere under %s" % (type_name, TYPE_DIRS[type_name]))
        continue
    names = fields_of(path, type_name)
    unread = []
    for name in names:
        if pattern.count('%s') == 1:
            use = re.compile(pattern % re.escape(name))
        else:
            use = re.compile(pattern % ((re.escape(name),) * pattern.count('%s')))
        read = False
        for _, line in ui_lines:
            if not use.search(line):
                continue
            # An assignment is not a read.
            match = assignment.search(line)
            if match and match.group(1) == name:
                continue
            read = True
            break
        if not read:
            unread.append(name)

    every_unread.extend(unread)

    if "--names" not in sys.argv:
        print(f"{type_name}: {len(names)} fields, {len(unread)} never read in {UI}")
        for name in unread:
            print(f"  {name}")
        print()

if "--names" in sys.argv:
    # One name per line, for the gate that watches the set rather than reading it.
    for name in every_unread:
        print(name)
