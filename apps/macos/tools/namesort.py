"""Lists SQL sorts by name that no Go-side natural comparison follows.

Rounds 186, 227 and 228 each found the same defect in a different repository: a name ordered
with SQL's `ORDER BY`, while the extension orders names through Intl.Collator. SQL compares
bytes, so it puts "Zebra" before "apple" and orders CJK by code point -- and a CJK library is
exactly where that shows.

    tag list      ORDER BY name ASC, and List re-sorted in Go while Summaries did not
    saved views   ORDER BY name ASC, with no Go sort at all
    author list   ORDER BY ... author_handle ASC, which also put the LIMIT on the wrong side

The rule the three fixes settled on is: `ORDER BY` may order by counts and clusters, and a
name order belongs in Go through domain.CompareNaturalNames.

The check is per function rather than per query, because the remedy is a line of Go in the same
function. A function that mentions a name column in an ORDER BY and never calls
CompareNaturalNames is a candidate.

    python3 tools/namesort.py            # candidates
    python3 tools/namesort.py --verbose  # every ORDER BY it looked at

It reports candidates, not findings: `ORDER BY bt.bookmark_id, t.name` is a deterministic base
for a group-by, not a user-visible order, and sorting that in Go would be work for nothing.
"""
import os
import re
import sys

ROOT = "internal/storage/sqlite"
# A column whose order a person reads. `t.name` is included; `started_at` and `id` are not.
NAME_COLUMN = re.compile(r"\b(name|handle|author_handle|canonical_key|normalized_name)\b", re.I)
ORDER_BY = re.compile(r"ORDER BY[^`'\"]*", re.I)
FUNC = re.compile(r"^func \([^)]*\) (\w+)\(", re.M)
NATURAL = "CompareNaturalNames"


def functions(path):
    """(name, body) for each method in a file."""
    source = open(path, encoding="utf-8").read()
    marks = [(m.start(), m.group(1)) for m in FUNC.finditer(source)]
    for i, (start, name) in enumerate(marks):
        end = marks[i + 1][0] if i + 1 < len(marks) else len(source)
        yield name, source[start:end]


# Candidates whose ORDER BY is not a user-visible order. Each needs a reason.
ALLOWED = {
    # A deterministic base for the rows of one bookmark's tag join. The caller groups them by
    # bookmark, so the name column is a tie-break inside a group rather than a list a person
    # reads down.
    "TagsByBookmark": "grouping base inside one bookmark's tag join, not a displayed order",
    # `ORDER BY CASE WHEN canonical_key = ? THEN 0 ELSE 1 END LIMIT 1` picks the row whose key
    # matches; it is a priority, and there is no second row to order against.
    "FindByName": "a priority that picks one row, not an order",
}


def main():
    verbose = "--verbose" in sys.argv
    candidates = 0
    for base, _, files in os.walk(ROOT):
        for name in sorted(files):
            if not name.endswith(".go") or name.endswith("_test.go"):
                continue
            path = os.path.join(base, name)
            for func, body in functions(path):
                hits = [m.group(0) for m in ORDER_BY.finditer(body)]
                named = [h for h in hits if NAME_COLUMN.search(h)]
                if not named:
                    if verbose and hits:
                        print("%s.%s: %d ORDER BY, none by name" % (name, func, len(hits)))
                    continue
                if NATURAL in body:
                    if verbose:
                        print("%s.%s: orders by name and re-sorts in Go" % (name, func))
                    continue
                if func in ALLOWED:
                    if verbose:
                        print("%s.%s: allowed (%s)" % (name, func, ALLOWED[func]))
                    continue
                candidates += 1
                print("%s.%s: %s -- and no %s in the same function"
                      % (name, func, named[0].strip(), NATURAL))
    # Unconditional, unlike the per-item lines above.
    #
    # Round 326 ran every checker and found five -- this one among them -- printing nothing when
    # clean, because the summary sat behind --verbose. "No output" then means both "nothing wrong"
    # and "the check did not run", which is the shape round 287 lost a round to: eighteen
    # screenshots all reported FAIL because a wrong path and a missing separator looked the same.
    # The count costs one line and tells the two apart.
    print("namesort: %d candidates" % candidates, file=sys.stderr)
    return 0


if __name__ == "__main__":
    sys.exit(main())
