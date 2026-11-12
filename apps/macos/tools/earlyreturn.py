"""Finds a `return` inside a loop inside a Children closure.

Round 104 nearly shipped one: `if renaming { ...; return }` inside
`ui.Column(...).Children(func() { for _, list := range ... { ... } })`. The return leaves
the *closure*, so every list after the one being renamed was not drawn. It compiles, and
the loop looks like it uses `continue`.

    python3 tools/earlyreturn.py

The rule is structural and approximate: from a `Children(func() {` line, walk to the brace
that closes it, and report a `return` whose nesting is deeper than a `for` in the same
closure. A return after the loop, or one in a helper, does not match.

It reports candidates, not findings.
"""
import os
import re
import sys


def go_files(root):
    found = []
    for base, _, names in os.walk(root):
        for name in names:
            if name.endswith(".go") and not name.endswith("_test.go"):
                found.append(os.path.join(base, name))
    return sorted(found)


opener = re.compile(r"Children\(func\(\)\s*\{")
for_loop = re.compile(r"^\s*for\b")
ret = re.compile(r"^\s*return\b")

found = []
for path in go_files("internal/ui"):
    lines = open(path, encoding="utf-8").read().split("\n")
    for index, line in enumerate(lines):
        if not opener.search(line):
            continue

        # Depth relative to the closure body.
        depth = 0
        loop_depths = []
        for offset in range(index, len(lines)):
            text = lines[offset]
            body = text.split("//")[0]

            if offset > index and for_loop.match(body):
                loop_depths.append(depth)

            if ret.match(body) and loop_depths and depth > max(loop_depths):
                found.append((path, offset + 1, text.strip()))

            depth += body.count("{") - body.count("}")
            if depth <= 0 and offset > index:
                break
            # A loop that has closed no longer owns what follows.
            loop_depths = [d for d in loop_depths if d < depth or depth == 0]

if "--names" in sys.argv:
    for path, line, _ in found:
        print(f"{path}:{line}")
    raise SystemExit(0)

if not found:
    print("OK: no return escapes a Children closure from inside a loop")
else:
    for path, line, text in found:
        print(f"  {path}:{line}  {text}")
    print(f"\n{len(found)} candidate(s): a return here leaves the closure, so the rest of")
    print("the loop does not run. Use if/else, or continue.")
