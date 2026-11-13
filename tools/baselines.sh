#!/usr/bin/env bash
# Regenerates the eight baseline screenshots, from the build in hand.
#
#     bash tools/baselines.sh
#
# Why a script and not a list in a document
#
# TROVE_DATA_DIR lets the two empty baselines be taken against a database with no bookmarks in it.
# A hand-written list goes stale as soon as a string shown in a state changes, because nothing
# connects "a string changed" to "a baseline is now stale". So the eight live here, as commands,
# and regenerating them is one command rather than something to remember.
#
# It also asserts what the completion criterion asserts -- that the eight differ pairwise. That is
# not automatic after a retake: two captures could come out identical.
#
# Exits non-zero when a capture fails or two baselines are byte-identical.

set -u
cd "$(dirname "$0")/.."

APP="build/darwin-arm64/trove.app"
BIN="$APP/Contents/MacOS/trove"
NAME="trove"
PATTERN="MacOS/$NAME"
OUT="docs/screenshots/baseline"
WIDTH=1180

if [ ! -x "$BIN" ]; then
  echo "  FAIL no binary at $BIN -- run: go tool mygo build -skip-dmg"
  exit 1
fi

# A leftover instance would be captured instead of this build. Refuse rather than kill it, for the
# reason tools/panes.sh gives: a leftover instance is the failure a launch check can miss.
before=$(pgrep -f "$PATTERN" | tr '\n' ' ')
if [ -n "$before" ]; then
  echo "  FAIL a process named '$NAME' was already running (pids: $before)"
  echo "       quit it and run again: this would capture the old build"
  exit 1
fi

mkdir -p "$OUT"

# name and capture state; the two empty ones also need a data directory of their own, because the
# seeded database has eight bookmarks and an empty state needs zero.
capture() {
  local name="$1" state="$2" empty="$3"
  local png="$OUT/$name.png"
  rm -f "$png"

  if [ "$empty" = "empty" ]; then
    local dir="/tmp/trove-baseline-$name"
    rm -rf "$dir"
    if [ -n "$state" ]; then
      TROVE_DATA_DIR="$dir" TROVE_HEADLESS=1 TROVE_CAPTURE="$png" TROVE_CAPTURE_STATE="$state" \
        TROVE_CAPTURE_DELAY=5s TROVE_CAPTURE_WIDTH="$WIDTH" "$BIN" >/dev/null 2>&1
    else
      TROVE_DATA_DIR="$dir" TROVE_HEADLESS=1 TROVE_CAPTURE="$png" \
        TROVE_CAPTURE_DELAY=5s TROVE_CAPTURE_WIDTH="$WIDTH" "$BIN" >/dev/null 2>&1
    fi
  else
    TROVE_HEADLESS=1 TROVE_CAPTURE="$png" TROVE_CAPTURE_STATE="$state" \
      TROVE_CAPTURE_DELAY=5s TROVE_CAPTURE_WIDTH="$WIDTH" "$BIN" >/dev/null 2>&1
  fi
  pkill -f "$PATTERN" >/dev/null 2>&1
  sleep 1

  if [ ! -s "$png" ]; then
    echo "  FAIL $name was not written"
    return 1
  fi
  echo "  ok   $name  $(wc -c < "$png" | tr -d ' ') bytes"
  return 0
}

fail=0
names=""

while read -r name state empty; do
  [ -z "$name" ] && continue
  capture "$name" "${state:--}" "$empty" || fail=$((fail + 1))
  names="$names $name"
done <<'ROWS'
empty-light - empty
empty-dark dark empty
populated-grid-light - -
populated-list-light list -
selected-inspector-light detail -
search-no-results-light search-empty -
sync-running-light sync-running -
sync-error-light sync-error -
ROWS

count=$(ls "$OUT"/*.png 2>/dev/null | wc -l | tr -d ' ')
if [ "$count" != "8" ]; then
  echo "  FAIL $count baselines in $OUT, expected 8"
  fail=$((fail + 1))
fi

# Pairwise, because the criterion says the eight differ and a retake can break that.
identical=0
for a in "$OUT"/*.png; do
  for b in "$OUT"/*.png; do
    if [ "$a" \< "$b" ] && cmp -s "$a" "$b"; then
      echo "  FAIL $(basename "$a") and $(basename "$b") are byte-identical"
      identical=$((identical + 1))
    fi
  done
done
if [ "$identical" -eq 0 ]; then
  echo "  ok   the eight differ pairwise"
else
  fail=$((fail + identical))
fi

if [ "$fail" -eq 0 ]; then
  echo "  ok   the eight baselines are regenerated from this build"
else
  echo "  FAIL $fail problem(s)"
fi
exit $([ "$fail" -eq 0 ] && echo 0 || echo 1)
