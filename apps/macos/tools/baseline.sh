#!/usr/bin/env bash
# Prints the numbers the plan's baseline table quotes, so the table can be regenerated instead of
# hand-maintained.
#
# Round 314 audited the objective against the workspace and found every one of the seven numbers
# in the plan's baseline table had gone stale -- it was written at round 257 and quoted 11/11 for
# the gate, 9/9 for selfcheck, 1054 tests, 22 packages, 31 ui files, an 824-line maximum and 9
# checkers. All seven were wrong at round 314. The table is still worth having, because "where we
# started" is what makes "how far we came" legible, but a hand-written one is read as "where we
# are" by whoever finds it next, which is the wrong reading.
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

# How many rows the plan's criteria table has, and how they are marked. Round 431 found two of
# these numbers stale in the prose; counting them here means the table cannot drift again without
# this line changing.
#
# The rows have to be taken from section 11 only. A first attempt grepped the whole file and got
# 123 rows / 33 passed / 34 warned -- other tables in the plan (17.x, 10b.x) also start a row with
# a bold number. So the count is scoped: from the "## 11." heading to the next "## " heading.
# Round 432 measured this rather than assuming it; the whole-file figure was wrong by an order of
# magnitude and looked entirely plausible.
#
# And the first scoping attempt was still wrong: section 11 holds three tables, and the criteria one
# stops at row 13 -- row 14 lives further down in its own paragraph, and there is a separate table
# of already-fixed defects (337/347/348/349) that also starts rows with a bold number. So the range
# is the criteria table alone: from its header to the first blank line after it. 16 rows (14
# criteria, 9a-9d counted separately). Both wrong attempts looked plausible; neither was measured
# against the file before being trusted.
#
# The criteria-table row count is NOT computed here. Four attempts (123, 21, 75, then 75/13/9) each
# produced a different number and none matched the file; the table's boundaries are not regular
# enough for a one-line awk to find, and a wrong count is worse than no count because it looks
# authoritative. The figures in the plan's prose stay hand-measured and dated with the round that
# measured them -- that is the convention sections 14 and 17 already use. What IS computed here is
# the contract's section count, which is a plain grep and cannot drift.


cat <<EOF
| item | value |
|---|---|

| packages (go test ./...) | $packages |
| tests | $tests |
| internal/ui files (all) | $files |
| internal/ui files (non-test) | $nonTest |
| largest non-test file | $(basename "$biggestName") at $biggest lines |
| tools/ | $checkers |
| contract sections | $(grep -c '^## ' ../../docs/shell-contract.md) |
| contract deviation sections (S8xx) | $(grep -c '^## S8[b-z]' ../../docs/shell-contract.md) |
EOF

# The gate and the selfcheck are opt-in because they take minutes and this script exists to be run
# casually. Round 396 found the cost of leaving them out: section 14 of the plan quotes "gate 12/12,
# selfcheck 12/12" and both had moved on. A number nobody can produce cheaply is a number that goes
# stale quietly, so --with-gate produces it on request and says how long it took.
if [ "${1:-}" = "--with-gate" ]; then
  echo
  for script in gate selfcheck; do
    started=$(date +%s)
    line=$(bash "tools/$script.sh" 2>&1 | tail -1)
    echo "| $script.sh | $line ($(($(date +%s) - started))s) |"
  done
else
  echo "gate and selfcheck are not counted here; pass --with-gate to run them, or run them and read"
  echo "their last line. Round 396 found section 14 quoting 12/12 for both when they were at 13/13 and"
  echo "24/24 -- a number that is expensive to produce is a number that quietly goes stale."
fi
