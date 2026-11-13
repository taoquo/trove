#!/usr/bin/env bash
# Prints the numbers the plan's baseline table quotes, so the table can be regenerated instead of
# hand-maintained.
#
# A baseline table is worth having, because "where we started" is what makes "how far we came"
# legible, but a hand-written one is read as "where we are" by whoever finds it next, which is the
# wrong reading.
#
# So this prints the current values and nothing else. Run it when the table needs updating:
#
#     bash tools/baseline.sh
#
# The gate and selfcheck lines are not run here -- they take minutes and this is meant to be cheap.
# Run `bash tools/gate.sh` and `bash tools/selfcheck.sh` for those and read their last line.
set -euo pipefail

cd "$(dirname "$0")/.."

packages=$(go test -count=1 ./... 2>&1 | grep -cE '^(ok|FAIL)' || true)
tests=$(go test -count=1 ./... -v 2>&1 | grep -cE '^(    )?--- PASS' || true)
files=$(ls internal/ui/*.go | wc -l | tr -d ' ')
nonTest=$(ls internal/ui/*.go | grep -v _test.go | wc -l | tr -d ' ')
biggest=$(wc -l internal/ui/*.go | grep -v _test.go | grep -v ' total$' | sort -rn | head -1 | awk '{print $1}')
biggestName=$(wc -l internal/ui/*.go | grep -v _test.go | grep -v ' total$' | sort -rn | head -1 | awk '{print $2}')
checkers=$(ls tools/*.py tools/*.sh 2>/dev/null | wc -l | tr -d ' ')

# How many rows the plan's criteria table has, and how they are marked. Counting them here means
# the table cannot drift without this line changing.
#
# The rows have to be taken from section 11 only. Other tables in the plan (17.x, 10b.x) also
# start a row with a bold number. So the count would be scoped: from the "## 11." heading to the
# next "## " heading.
#
# Section 11 itself holds three tables, and the criteria one stops at row 13 -- row 14 lives
# further down in its own paragraph, and there is a separate table of already-fixed defects that
# also starts rows with a bold number. So the range would be the criteria table alone: from its
# header to the first blank line after it.
#
# The criteria-table row count is NOT computed here. The table's boundaries are not regular
# enough for a one-line awk to find, and a wrong count is worse than no count because it looks
# authoritative. The figures in the plan's prose stay hand-measured and dated. What IS computed
# here is the section count below, which is a plain grep and cannot drift.


cat <<EOF
| item | value |
|---|---|

| packages (go test ./...) | $packages |
| tests | $tests |
| internal/ui files (all) | $files |
| internal/ui files (non-test) | $nonTest |
| largest non-test file | $(basename "$biggestName") at $biggest lines |
| tools/ | $checkers |
| design docs | $(find docs -maxdepth 1 -name '*.md' | wc -l | tr -d ' ') |
EOF

# The gate and the selfcheck are opt-in because they take minutes and this script exists to be run
# casually. A number nobody can produce cheaply is a number that goes stale quietly, so --with-gate
# produces it on request and says how long it took.
if [ "${1:-}" = "--with-gate" ]; then
  echo
  for script in gate selfcheck; do
    started=$(date +%s)
    line=$(bash "tools/$script.sh" 2>&1 | tail -1)
    echo "| $script.sh | $line ($(($(date +%s) - started))s) |"
  done
else
  echo "gate and selfcheck are not counted here; pass --with-gate to run them, or run them and read"
  echo "their last line. A number that is expensive to produce is a number that quietly goes stale."
fi
