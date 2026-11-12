#!/usr/bin/env bash
# The two geometry invariants, checked against the captures in docs/screenshots.
#
# Criterion 1 of the plan used to read "columns.py fails on the picture from before the fix and
# passes on the picture after it". Round 389 established that this cannot be reproduced: the defect
# it names -- paint running from x=560 to x=2359 with no boundary -- is gone, and a picture taken
# now has the boundary in it at both widths. A criterion that requires building something broken
# first only holds while the broken thing exists.
#
# So it is replaced by two invariants that hold for any build:
#
#   1. narrow (1180): the middle pane is the last pane, so its content must stop at the edge the
#      contract gives -- W - 12 points, which is 2*(W-12) pixels. contentedge.py measures that.
#   2. wide (1500, detail open): the inspector's boundary must be drawn at W - 608 pixels, and
#      nothing may be painted across it. columns.py measures that.
#
# Both are true today, which is the point: this is a step that stops a future change, not one that
# documents a past fix. A step that can only pass is worth nothing, so the last thing this script
# does is feed a deliberately overrunning picture to contentedge.py and confirm it fails -- the same
# move selfcheck.sh makes for every other checker.
set -uo pipefail
cd "$(dirname "$0")/.."

SHOTS=../../docs/screenshots
pass=0; fail=0
ok()   { echo "  ok   $1"; pass=$((pass + 1)); }
bad()  { echo "  FAIL $1"; echo "       $2"; fail=$((fail + 1)); }

NARROW="$SHOTS/baseline/populated-grid-light.png"
WIDE="$SHOTS/criteria/wide-detail-1500.png"

if [ ! -f "$NARROW" ]; then
  echo "  FAIL no capture at $NARROW -- run: bash tools/baselines.sh"; exit 1
fi

# 1. Narrow: the content stops at the contract's right edge (1180 points -> 2352 pixels).
out=$(python3 tools/contentedge.py "$NARROW" --right-edge 2352 2>&1)
if [ $? -eq 0 ]; then
  ok "narrow: the content stops at the contract's right edge ($(printf '%s' "$out" | sed -n 's/.*content right = \([0-9]*\).*/\1/p'))"
else
  bad "narrow: the content runs past the contract's right edge" "$out"
fi

# 2. Wide: the inspector's boundary is drawn and not crossed.
if [ ! -f "$WIDE" ]; then
  bad "wide: no capture at $WIDE" "see docs/screenshots/criteria/README.md for how it was taken"
else
  width=$(python3 -c "from PIL import Image; print(Image.open('$WIDE').size[0])")
  boundary=$((width - 608))
  out=$(python3 tools/columns.py "$WIDE" --boundary "$boundary" 2>&1)
  if [ $? -eq 0 ]; then
    ok "wide: the inspector's boundary is drawn at $boundary and not crossed"
  else
    bad "wide: no boundary at $boundary, or paint crosses it" "$out"
  fi
fi

# 3. And the check can fail: a synthetic picture with content past the edge must be rejected.
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
python3 - "$NARROW" "$tmp/over.png" <<'PY'
import sys
from PIL import Image, ImageDraw
src, out = sys.argv[1], sys.argv[2]
im = Image.open(src).convert("RGB")
d = ImageDraw.Draw(im)
# Paint a solid bar well past the declared edge, covering every row.
#
# The first version painted one pixel every third row, and contentedge.py did not notice: it takes
# the median over rows sampled every ninth pixel, so a bar covering a third of those rows leaves
# the median at the ordinary content edge. A synthetic input has to be unambiguous -- the point of
# this branch is to show the checker can fail, and an input it can argue past does not show that.
#
# The bar goes just past the declared edge but inside the search window: contentedge.py ignores
# anything within --frame (6) of the right, because the window's own border lives there. 2353 is
# inside the picture and past 2352, which is the only band this checker can see.
for y in range(0, im.size[1]):
    for x in range(2353, 2355):
        d.point((x, y), fill=(20, 20, 20))
im.save(out)
PY
python3 tools/contentedge.py "$tmp/over.png" --right-edge 2352 >/dev/null 2>&1
if [ $? -eq 1 ]; then
  ok "a picture with content past the edge is rejected"
else
  bad "contentedge.py accepted a picture with content past the edge" "the invariant cannot fail"
fi

echo
echo "$pass passed, $fail failed"
[ "$fail" -eq 0 ]
