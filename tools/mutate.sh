#!/usr/bin/env bash
# A small mutation harness.
#
# It exists because a passing suite says nothing about whether the assertions bite: a
# test can reimplement the function under test and never fail. Reading the test does not
# reveal that; changing the function does.
#
#   bash tools/mutate.sh <file> <old-string> <new-string> <test-package> <test-run-regex>
#
# Prints CAUGHT, SURVIVED, ANCHOR-MISMATCH, MUTATION-NOT-APPLIED or BUILD-FAILED, and
# restores the file in every case.
set -uo pipefail
cd "$(dirname "$0")/.."
source .toolchain/env.sh >/dev/null 2>&1

file="$1"; old="$2"; new="$3"; pkg="$4"; run="$5"

backup=$(mktemp)
cp "$file" "$backup"
restore() { cp "$backup" "$file"; rm -f "$backup"; }
trap restore EXIT

# Refuse to judge a mutation against a filter that matches nothing, and do it before the
# mutation: after it the package may not compile, and `go test -list` on a package that
# does not build prints nothing -- which is indistinguishable from an empty filter. That
# turned a compile error into "NO-TESTS-MATCHED" once already.
matched=$(go test -list "$run" "$pkg" 2>/dev/null | grep -c '^Test')
if [ "$matched" -eq 0 ]; then
  echo "NO-TESTS-MATCHED (the filter selects nothing, or the package does not build)"
  exit 5
fi

python3 tools/apply_mutation.py "$file" "$old" "$new" || { echo "ANCHOR-MISMATCH"; exit 2; }

# Confirm the edit landed: an anchor that matched but produced identical text would
# report SURVIVED for the wrong reason.
if ! python3 tools/apply_mutation.py --check "$file" "$old" "$new"; then
  echo "MUTATION-NOT-APPLIED"
  exit 3
fi

output=$(go test -count=1 "$pkg" -run "$run" 2>&1)
status=$?

if [ $status -eq 0 ]; then
  echo "SURVIVED"
  exit 0
fi

# A build failure is not a caught mutation: the mutant never ran.
if printf '%s' "$output" | grep -qE "build failed|cannot use|undefined:|syntax error|setup failed"; then
  echo "BUILD-FAILED (not a real catch)"
  exit 4
fi

echo "CAUGHT"
