#!/usr/bin/env bash
# Runs the whole gate, in the order the checks depend on each other.
#
# Until round 238 this was a command typed by hand each time, and the launch assertion inside it
# was the one step nobody could run twice the same way: it used `open`, which reuses a running
# instance, so it reported success while testing an old binary (round 237). A gate that lives in
# a shell history cannot be reviewed, and a step whose command differs each time is a step whose
# result differs.
#
#     source ../../.toolchain/env.sh && bash tools/gate.sh
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

echo "checks:"
step "translator covers every string" bash tools/check-text.sh
step "wiring, surfaces, tokens, names, citations" bash tools/check-wiring.sh
step "the board names one slice with its evidence" python3 tools/statuscheck.py
step "every checker still catches its defect" bash tools/selfcheck.sh

echo "build:"
quiet "gofmt" gofmt -l .
step "go vet" go vet ./...
step "mygo vet" go tool mygo vet .
quiet "CGO_ENABLED=0 build" sh -c 'CGO_ENABLED=0 go build -o /dev/null .'

echo "tests:"
step "go test" go test -count=1 -timeout 20m ./...
step "go test -race" go test -race -count=1 -timeout 20m ./...

echo "app:"
step "mygo build" go tool mygo build -skip-dmg
step "launch, single instance, quit" bash tools/smoke.sh
# The panes check captures this build rather than reading a stored picture. Eight rounds
# (246-255) went into widths while the inspector had no left edge at all, and nothing in the
# suite could see it: 1054 tests and nine checkers ask about values and wiring, and none of them
# asks what the shell's space looks like. Reading a stored baseline would not have helped
# either -- deleting the line that draws the edge leaves the baseline unchanged.
step "the panes have a boundary" bash tools/panes.sh
step "the two geometry invariants" bash tools/geometry.sh

echo
echo "$pass passed, $fail failed"
[ "$fail" -eq 0 ]
