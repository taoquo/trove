#!/usr/bin/env python3
"""Reports how much of the extension's English copy is present verbatim in this build.

    python3 tools/copycheck.py
    python3 tools/copycheck.py --list      # the strings that are not present, with near misses

Why this exists, and what it cannot say
---------------------------------------
Round 337 read the eight baselines against src/ and found four defects no completion criterion
covered: `Published` showed a raw ISO stamp, the sync-error line showed only the word "Error", the
sidebar's empty-tags line was a different sentence with no full stop, and the inspector's
"no bookmark selected" line had been rewritten in both languages. All four are about what the
pixels *say*, and every one of the thirteen criteria is about where they *are* -- structure,
separators, tokens, weights. Section 11.1 records that gap.

This prints the number behind criterion 14, which is deliberately weak: "the count must not grow".
Round 350 measured 110 of 301 and then read the 110, and they fall into three groups that this
cannot separate:

  * the same sentence with a different placeholder syntax -- "{blocked} tag names" here against
    "%d tag names" there, which is correct because the Go side formats with %d;
  * copy for features this build does not have -- the tag relation graph, the popup -- which is
    absent rather than wrong;
  * genuine rewrites, which are the defects.

How many of them are each kind -- measured in round 394, at the count of 108:

    16   carry a {placeholder}: "Elapsed {seconds}s", "Syncing... {pages} pages". These are present
         in this build, reformatted by its own formatter rather than stored as the literal template,
         so they are not differences in wording at all.
    92   do not. This is the population that still has to be read screen by screen. How much of it
         is unbuilt features and how much is genuine rewrites is NOT MEASURED -- separating those
         needs a judgement per string about whether this build has the feature, and there is no
         cheap test for it. Round 393 called that out as the one limitation in tools/ that stated a
         classification without giving the count behind it.

So a count of 110 does not mean 110 defects, and a criterion demanding zero would fail on the
first two groups. Read this as a gauge: it is cheap to re-run, and a jump means something changed
in the copy layer. For the third group, the check that works is one screen at a time -- read a
capture, find the strings it shows in src/options/i18n/copy.ts, compare them word for word. That
is a procedure, not a gate, because the set of rendered screens grows.

Two corrections from rounds 363-365, kept because both were wrong turns worth not repeating:

Reading.copy.ts is not flat. It holds plain keys, nested objects (statusLabels is one, read back
as copy.statusLabels[status]), and strings that reach a component as a prop rather than through
this table at all (phaseHeuristics lives in src/ui/components.tsx and is passed in). Three cheap
textual checks in a row gave three wrong answers about which strings are dead -- 9, then 1, then
0 -- because each assumed the flat shape after looking at the top of the file. Before writing a
cross-file check, print the structure.

A near match is a candidate and never a conclusion. Round 363 paired this build's strings against
the source's by edit distance and concluded two of them should change; both pairings were false,
and acting on them would have moved the interface further from the source, not closer.

Exit codes follow tools/columns.py: 2 when the tool cannot do its job, 0 otherwise. There is no
failure code, because no count here is a failure.
"""

import argparse
import os
import re
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.dirname(os.path.dirname(os.path.dirname(HERE)))   # the repository root
COPY = os.path.join(ROOT, "src", "options", "i18n", "copy.ts")
MINE = os.path.join(HERE, "..", "internal", "i18n", "i18n.go")


def cjk(text):
    return any("\u4e00" <= c <= "\u9fff" for c in text)


def english_strings(path):
    """Every `key: "value"` in copy.ts whose value has no CJK characters, last one per key.

    The file holds the zh-CN table first and the en table after it, so keeping the last occurrence
    of each key keeps the English one.
    """
    try:
        text = open(path, encoding="utf-8").read()
    except OSError as err:
        sys.stderr.write("copycheck: cannot read %s: %s\n" % (path, err))
        raise SystemExit(2)

    pairs = re.findall(r'(\w+):\s*"((?:[^"\\]|\\.)*)"', text)
    by_key = {}
    for key, value in pairs:
        if not cjk(value) and len(value) > 2:
            by_key[key] = value
    return by_key


def main(argv):
    parser = argparse.ArgumentParser(description=__doc__.split("\n")[1])
    parser.add_argument("--copy", default=COPY, help="the extension's copy table")
    parser.add_argument("--mine", default=MINE, help="this build's string table")
    parser.add_argument("--split", action="store_true",
                        help="also report how many of the missing strings carry a {placeholder}")
    parser.add_argument("--list", action="store_true", help="name the strings that are not present")
    args = parser.parse_args(argv)

    source = english_strings(args.copy)
    if not source:
        sys.stderr.write("copycheck: no English strings found in %s -- has it moved?\n" % args.copy)
        return 2

    mine = open(args.mine, encoding="utf-8").read()
    missing = sorted((k, v) for k, v in source.items() if ('"%s"' % v) not in mine)

    print("copycheck: %d English strings in copy.ts, %d not present verbatim in this build"
          % (len(source), len(missing)))
    if args.split:
        # Two populations, and only one of them is measurable here.
        #
        # A string carrying a {placeholder} cannot be compared literally: this build stores it as a
        # format string and its formatter produces the same visible text. The rest have to be read
        # one screen at a time -- telling an unbuilt feature from a rewrite is a judgement about
        # whether the feature exists, and nothing cheap answers it. Round 394 measured the first
        # number and wrote the second as unmeasured rather than guessing a ratio.
        templated = [k for k, v in missing if "{" in v]
        print("           of those, %d carry a {placeholder} and %d do not"
              % (len(templated), len(missing) - len(templated)))
        print("           the %d are NOT split into unbuilt features and rewrites: that needs a"
              % (len(missing) - len(templated)))
        print("           judgement per string and there is no cheap test for it")
    if not args.split:
        # This line said the same thing as the two above, and round 394 left both printing: adding
        # a section next to an existing one makes the old one redundant, and two places saying the
        # same thing drift apart with one of them ending up wrong.
        print("           (of those, some are placeholder syntax, some are unbuilt features, some"
              " are rewrites -- see this tool's docstring, or pass --split for the count)")

    if args.list:
        import difflib
        mine_strings = set(re.findall(r'"((?:[^"\\]|\\.)*)"', mine))
        for key, value in missing:
            near = difflib.get_close_matches(value, mine_strings, n=1, cutoff=0.72)
            print("  %-38s %s" % (key, value[:60]))
            if near:
                print("  %-38s   nearest here: %s" % ("", near[0][:60]))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
