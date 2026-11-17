#!/usr/bin/env bash
# Runs the whole gate, in the order the checks depend on each other.
#
# The gate lives in one place so every run performs the same steps in the same way. A gate that
# lives in a shell history cannot be reviewed, and a step whose command differs each time is a
# step whose result differs.
#
#     source .toolchain/env.sh && bash tools/gate.sh
#
# Every step runs even after one fails, so a single run reports everything that is wrong rather
# than the first thing. The exit code is non-zero if any step failed.

set -u
cd "$(dirname "$0")/.."

pass=0
fail=0

step() {
  local name="$1"
  shift
  local out
  if out=$("$@" 2>&1); then
    echo "  ok   $name"
    pass=$((pass + 1))
  else
    echo "  FAIL $name"
    printf '%s\n' "$out" | sed 's/^/       /' | head -20
    fail=$((fail + 1))
  fi
}

# A step that must produce no output to pass, where the command's own exit status is always 0.
quiet() {
  local name="$1"
  shift
  local out
  out=$("$@" 2>/dev/null)
  if [ -z "$out" ]; then
    echo "  ok   $name"
    pass=$((pass + 1))
  else
    echo "  FAIL $name"
    printf '%s\n' "$out" | sed 's/^/       /' | head -20
    fail=$((fail + 1))
  fi
}

# The family audit lives outside this repository, in the taoque skill. It is the only check
# that compares this repo's palette with the other surfaces that copy it. When the skill is
# not on this machine the step fails rather than skipping: a gate that skips is not a gate,
# and the fix is one environment variable.
family_audit() {
  local audit="${TAOQUE_AUDIT:-}"
  if [ -z "$audit" ]; then
    for candidate in ../taoque/scripts/audit.sh "$HOME/.claude/skills/taoque/scripts/audit.sh"; do
      if [ -f "$candidate" ]; then
        audit="$candidate"
        break
      fi
    done
  fi
  if [ -z "$audit" ]; then
    echo "taoque audit not found; set TAOQUE_AUDIT to its scripts/audit.sh"
    return 1
  fi
  bash "$audit"
}

echo "checks:"
step "family design tokens" family_audit
step "translator covers every string" bash tools/check-text.sh
step "wiring, surfaces, tokens, names, citations" bash tools/check-wiring.sh
step "every checker still catches its defect" bash tools/selfcheck.sh

# The module root is the repository root, and the pinned toolchain lives inside it. gofmt
# recurses into a directory argument, so it must be given files -- naming the root package's
# directory would walk .toolchain/go as well.
gofmt_project() {
  find . -name '*.go' -not -path './.toolchain/*' -not -path './.git/*' -not -path './build/*' -print0 |
    xargs -0 gofmt -l
}

echo "build:"
quiet "gofmt" gofmt_project
step "go vet" go vet ./...
step "mygo vet" go tool mygo vet ./cmd/trove
quiet "CGO_ENABLED=0 build" sh -c 'CGO_ENABLED=0 go build -o /dev/null ./cmd/trove'

echo "tests:"
step "go test" go test -count=1 -timeout 20m ./...

# The budgets are measured here and only here. "go test ./..." above runs packages in
# parallel, and a query or a frame timed while another package fills a database reads that
# contention, not the code: a run with both steps showed search at 366 ms against its 320 ms
# ceiling and a frame over its own, while both were under budget in a quiet process. So the
# wall-clock tests skip unless TROVE_BUDGETS=1, which this step sets -- and they are named,
# so a skip and a pass stay distinguishable in the step's single line. -p 1 keeps the query
# budget and the frame budget from being measured beside each other.
#
# Before the race run on purpose: wall-clock ceilings measured while the detector's process is
# still being reaped read the machine's flush rather than the code, and the first gate with this
# step failed on a 7.4 ms sample of a path whose median is 1.9 ms.
step "the time budgets still hold" env TROVE_BUDGETS=1 go test -p 1 -count=1 -timeout 5m -run 'TestPerformanceBudget|TestUIRenderBudget|TestTheSidebarCacheIsWhatMakesAFilterCheap' ./internal/usecase/workspace ./internal/ui

step "go test -race" go test -race -count=1 -timeout 20m ./...

# The page parser reads bytes from the network. Thirty seconds of mutation finds the panics a
# table of hand-written payloads cannot; the window is short on purpose, so this is a smoke
# screen rather than a proof. Only a failing input is written to testdata (the rest stays in the
# build cache), so a green run leaves the tree alone.
step "the parser survives arbitrary bytes for 30s" go test -count=1 -timeout 2m ./internal/xsource -run FuzzParsePage -fuzz FuzzParsePage -fuzztime 30s

echo "app:"
step "mygo build" go tool mygo build -skip-dmg
step "launch, single instance, quit" bash tools/smoke.sh
# The panes check captures this build rather than reading a stored picture. The unit tests and
# checkers ask about values and wiring, and none of them asks what the shell's space looks like.
# Reading a stored baseline would not help either -- deleting the line that draws the edge leaves
# the baseline unchanged.
step "the panes have a boundary" bash tools/panes.sh
step "the two geometry invariants" bash tools/geometry.sh

echo
echo "$pass passed, $fail failed"
[ "$fail" -eq 0 ]
