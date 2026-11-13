#!/usr/bin/env bash
# Regenerates the captures under docs/screenshots/criteria, and proves they are reproducible.
#
# tools/baselines.sh does this for the eight baseline pictures. This does it for the two that
# carry the geometry invariants, so both are evidence anyone can reproduce: a capture typed by
# hand, or one scattered across ad-hoc directories, is findable only by whoever remembers it.
#
# The rule this script enforces is cmp: capture, compare with what is on disk, and say so. A
# capture that no longer matches is not a failure -- the build may legitimately have changed -- but
# it must be reported, because "this picture is reproducible" is the whole claim a criterion makes.
#
#   bash tools/criteria.sh           regenerate and compare
#   bash tools/criteria.sh --update  regenerate and replace
#
# The pairs are the same shape as baselines.sh's table: name  width  state.
set -uo pipefail
cd "$(dirname "$0")/.."
source .toolchain/env.sh >/dev/null 2>&1

APP="build/darwin-arm64/trove.app"
BIN="$APP/Contents/MacOS/trove"
OUT=docs/screenshots/criteria
UPDATE=0
[ "${1:-}" = "--update" ] && UPDATE=1

if [ ! -x "$BIN" ]; then
  echo "  FAIL no binary at $BIN -- run: go tool mygo build -skip-dmg"; exit 1
fi
mkdir -p "$OUT"

if pgrep -f "MacOS/trove" >/dev/null 2>&1; then
  echo "  FAIL a build is already running; quit it first (the capture needs a single instance)"
  exit 1
fi

# name  width  state
CASES=(
  "wide-detail-1500 1500 detail"
)

pass=0
fail=0
for row in "${CASES[@]}"; do
  read -r name width state <<<"$row"
  target="$OUT/$name.png"
  tmp=$(mktemp -t criteria-XXXXXX).png

  TROVE_HEADLESS=1 TROVE_CAPTURE="$tmp" TROVE_CAPTURE_STATE="$state" TROVE_CAPTURE_DELAY=6s TROVE_CAPTURE_WIDTH="$width" \
    "$BIN" >/dev/null 2>&1
  if [ ! -s "$tmp" ]; then
    echo "  FAIL $name -- no picture was written"; fail=$((fail + 1)); rm -f "$tmp"; continue
  fi

  if [ ! -f "$target" ]; then
    mv "$tmp" "$target"
    echo "  new  $name  ($(wc -c <"$target" | tr -d ' ') bytes)"; pass=$((pass + 1))
  elif cmp -s "$tmp" "$target"; then
    echo "  ok   $name -- reproduces byte for byte"; pass=$((pass + 1)); rm -f "$tmp"
  else
    if [ "$UPDATE" -eq 1 ]; then
      mv "$tmp" "$target"
      echo "  upd  $name -- replaced"; pass=$((pass + 1))
    else
      echo "  DIFF $name -- the build has changed since this capture; rerun with --update if that is expected"
      fail=$((fail + 1)); rm -f "$tmp"
    fi
  fi
done

echo
echo "$pass passed, $fail failed"
[ "$fail" -eq 0 ]
