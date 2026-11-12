"""Applies one textual mutation, refusing an ambiguous anchor.

Kept apart from the shell harness because a nested heredoc inside it was silently
mangled, which is how the harness first reported a caught mutation as SURVIVED.

    apply_mutation.py FILE OLD NEW      replace OLD with NEW, once
    apply_mutation.py --check FILE OLD NEW   report whether the edit is present

Keeping OLD as well as NEW is what lets the check handle a deletion: an empty NEW has no
first line to look for, and the check that searched for one raised instead.
"""
import sys

if len(sys.argv) == 5 and sys.argv[1] == "--check":
    _, _, path, old, new = sys.argv
    source = open(path, encoding="utf-8").read()
    if new == "":
        # A deletion is present when the anchor is gone.
        sys.exit(0 if old not in source else 1)
    sys.exit(0 if new.splitlines()[0] in source else 1)

if len(sys.argv) != 4:
    print(__doc__, file=sys.stderr)
    sys.exit(2)

path, old, new = sys.argv[1], sys.argv[2], sys.argv[3]
source = open(path, encoding="utf-8").read()
count = source.count(old)
if count != 1:
    # 2, not 1: this is the tool being unable to do its job, not an answer to a question.
    #
    # 1 in --check mode answers "has this edit been applied?", which is a result. Sharing the code
    # between that result and this failure is the shape round 317 fixed in columns.py and round
    # 376 fixed in cardrows.py; round 379's exit-code table found it here. It was latent rather
    # than live -- mutate.sh runs apply and --check on separate paths, so the two meanings never
    # reached one caller -- and the fix costs nothing, because mutate.sh maps any non-zero from
    # the apply path to ANCHOR-MISMATCH and exits 2 either way.
    print(f"the anchor appears {count} times, not once", file=sys.stderr)
    sys.exit(2)
open(path, "w", encoding="utf-8").write(source.replace(old, new))
