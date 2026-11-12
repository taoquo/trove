#!/usr/bin/env bash
# Prints the evidence block a slice's commit body must carry.
#
# The block was a template in the contract, and templates do not get filled: the fields
# everyone can type (tree, dirty, date) are exactly the ones that go missing. This prints
# those, and leaves the eight lines only the run can answer. Paste it into the commit, then
# replace each placeholder with what actually happened -- including `not tried`, which is
# never blank.
#
#     bash tools/evidence.sh F6-1 --gate "14/14 in 4m12s"
set -u
cd "$(dirname "$0")/.."

slice="${1:-<slice-id>}"
gate="<paste the last line of gate.sh>"
if [ "${2:-}" = "--gate" ] && [ -n "${3:-}" ]; then
  gate="$3"
fi

tree=$(git rev-parse --short HEAD 2>/dev/null || echo "<no commit>")
if [ -n "$(git status --porcelain 2>/dev/null)" ]; then
  dirty="dirty"
else
  dirty="clean"
fi

cat <<EOF
Slice: $slice
Tree: $tree ($dirty)
Gate: $gate
Red: <command> -> FAIL (<what failed on the old code>)
Green: <command> -> ok
Real run: <command + state + time> / not tried: <why>
Capture: <path> (build $tree)
Broken by hand: <what was reverted to make the test red> / none
Not tried: <what was not run, and why> / none
EOF
