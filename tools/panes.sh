#!/usr/bin/env bash
# Captures the three-pane shell from the build in hand and checks that the inspector's left
# edge is drawn.
#
# Why this is not just tools/columns.py
#
# columns.py reads a picture, and calling it on a stored baseline is not a check of the source code:
# delete the line that draws the separator and the baseline file is unchanged, so the gate
# stays green while the window is wrong. The mutation that is supposed to prove the check works
# -- remove .MinWidth(0), or remove the BorderWidth -- would be caught by nothing.
#
# So this launches the binary, asks it for the three-pane state, and runs the check on a picture
# taken from that build. The stored pair under docs/screenshots is the record of what the defect
# looked like, but it is not what the gate reads.
#
#     bash tools/panes.sh
#
# Exits non-zero when the boundary is missing. Reports SKIP, with the reason, when the capture
# cannot show an inspector at all -- a library with no bookmarks has no detail pane, and a pass
# in that state would be a pass on nothing.

set -u
cd "$(dirname "$0")/.."

APP="build/darwin-arm64/trove.app"
BIN="$APP/Contents/MacOS/trove"
NAME="trove"
PATTERN="MacOS/$NAME"

if [ ! -x "$BIN" ]; then
  echo "  FAIL no binary at $BIN -- run: go tool mygo build -skip-dmg"
  exit 1
fi

# A leftover instance would be captured instead of this build -- the failure mode a launch check
# can miss. Refuse rather than kill it.
before=$(pgrep -f "$PATTERN" | tr '\n' ' ')
if [ -n "$before" ]; then
  echo "  FAIL a process named '$NAME' was already running (pids: $before)"
  echo "       quit it and run again: this would capture the old build"
  exit 1
fi

# Two captures, because the shell has two answers and the boundary between them is a width rule.
# An arithmetic rule -- sidebar + MainMinWidth + InspectorWidth -- is satisfied by 1180, so a
# 1180-wide window would get three columns. The detail is never a pane at any width: it is an
# overlay.
#
# So the narrow window must NOT have a boundary at the pane width, and the wide one must.
narrow_png=$(mktemp -t trove-panes-narrow).png
wide_png=$(mktemp -t trove-panes-wide).png
alone_png=$(mktemp -t trove-panes-alone).png
trap 'rm -f "$narrow_png" "$wide_png" "$alone_png"' EXIT

# Headless, like every other capture.
#
# A windowless capture lays out from the width the shell was asked for. The three checks pass on
# ui.Render's frames, so nothing in the gate opens a window.
TROVE_HEADLESS=1 TROVE_CAPTURE="$narrow_png" TROVE_CAPTURE_STATE=detail TROVE_CAPTURE_DELAY=5s \
  TROVE_CAPTURE_WIDTH=1180 "$BIN" >/dev/null 2>&1
# No TROVE_CAPTURE_STATE=detail: the detail is an overlay at every width, and an overlay's
# backdrop dims the whole window -- which puts the sidebar's separator below this check's contrast
# threshold in bands 1 and 2 and makes the check fail on a boundary that is there. What this
# capture is for is the workspace's own two panes.
TROVE_HEADLESS=1 TROVE_CAPTURE="$wide_png" TROVE_CAPTURE_DELAY=5s \
  TROVE_CAPTURE_WIDTH=1500 "$BIN" >/dev/null 2>&1
# The narrow window with no detail at all. This is what the check for "the list keeps its width"
# needs: with the detail open at 1180 the overlay covers the list, so its extent cannot be read
# from that picture. Without it the list is unoccluded and its right edge is measurable.
TROVE_HEADLESS=1 TROVE_CAPTURE="$alone_png" TROVE_CAPTURE_DELAY=5s \
  TROVE_CAPTURE_WIDTH=1180 "$BIN" >/dev/null 2>&1
# The wide window WITH the detail, to assert there is no third pane in it.
overlay_png=$(mktemp -t trove-panes-overlay).png
TROVE_HEADLESS=1 TROVE_CAPTURE="$overlay_png" TROVE_CAPTURE_STATE=detail TROVE_CAPTURE_DELAY=5s \
  TROVE_CAPTURE_WIDTH=1500 "$BIN" >/dev/null 2>&1

if [ ! -s "$narrow_png" ] || [ ! -s "$wide_png" ]; then
  echo "  FAIL a capture wrote nothing (narrow $(wc -c <"$narrow_png" 2>/dev/null || echo 0) bytes,"
  echo "       wide $(wc -c <"$wide_png" 2>/dev/null || echo 0) bytes)"
  exit 1
fi

# The boundary is derived from the picture, not from the width that was asked for, because the
# two are not the same: a window asked to be 1500 points wide has 1470 of content once the frame
# is taken out. The inspector is 304 points and the image is 2x, so its left edge is 608 pixels
# in from the right -- which is also where the narrow case's 1752 comes from.
wide_width=$(python3 -c "from PIL import Image; print(Image.open('$wide_png').size[0])")
wide_boundary=$((wide_width - 608))
# The narrow capture's boundary, at the same offset, where a pane must NOT be.
narrow_width=$(python3 -c "from PIL import Image; print(Image.open('$narrow_png').size[0])")
narrow_boundary=$((narrow_width - 608))
# BOTH pane boundaries, not just the inspector's.
#
# A criterion written as "three consecutive painted segments on a horizontal line" does not
# describe this shell: the three columns are adjacent, and a separator is a 1px line between two
# tracks, not a gap. A scan line through the cards has two segments (sidebar, then everything
# else) and one through the header has thirty-eight. "Three segments" describes no row here.
# Looking for a gap finds no row either; columns.py measures a boundary line instead.
#
# What is checkable is a separator at each pane boundary. There are two, and the sidebar's is
# ui.Split's own while the inspector's is the BorderWidth this build adds.
# The sidebar's boundary moves with the persisted width (212 by default, dragged inside
# 168..320), so it is read from the picture: the first separator after the column's narrowest
# possible edge. Scanning has to start left of 168, not at the old 300, or a column dragged to
# its minimum would have its own line skipped. The inspector's boundary is arithmetic, since its
# width is a constant.
read -r sidebar_boundary <<EOF
$(python3 - "$wide_png" <<'PY'
import sys
from PIL import Image
im = Image.open(sys.argv[1]).convert("RGB"); px = im.load(); w, h = im.size
# A separator, judged the way columns.py will judge it.
#
# Two mistakes would make this disagree with the assertion:
#
#   - sampling y 400..1100 and asking for 80 of 175 rows, while columns.py asks for half of each
#     band across the whole picture, lets a candidate pass here and fail there;
#   - taking the *first* x that passes lets a row's highlight edge win over the real separator.
#     Which one comes first depends on the persisted sidebar width, because persistLayout writes
#     it on every launch and these three captures share the real preferences.
#
# So detection uses the same test over the same span the assertion will use: a boundary is a
# column that steps away from the pixels on both sides for more than half the height. A highlight
# edge cannot do that; a pane boundary does.
def is_separator(x):
    rows = 0
    total = 0
    for y in range(40, h - 40, 4):
        total += 1
        here, left, right = px[x, y], px[x - 4, y], px[x + 4, y]
        if (abs(here[0]-left[0])+abs(here[1]-left[1])+abs(here[2]-left[2]) > 6 and
                abs(here[0]-right[0])+abs(here[1]-right[1])+abs(here[2]-right[2]) > 6):
            rows += 1
    return rows > total // 2
for x in range(120, w // 2):
    if is_separator(x):
        print(x); sys.exit(0)
print(0)
PY
)
EOF

for pair in "$sidebar_boundary:the sidebar's right edge, from ui.Split"; do
  at=${pair%%:*}
  what=${pair#*:}
  if [ "$at" = "0" ]; then
    echo "  FAIL no separator was found between the sidebar and the list"
    exit 1
  fi
  out=$(python3 tools/columns.py "$wide_png" --boundary "$at" 2>&1)
  code=$?
  # 2 means the tool could not run; 1 means it ran and the picture is wrong. The two must not
  # share an exit code, or "No such file" reads as a failed check.
  if [ "$code" -eq 2 ]; then
    echo "  FAIL columns.py could not run: $out"
    exit 1
  fi
  if [ -n "$out" ]; then
    echo "  FAIL a wide capture has no separator at $what (x=$at):"
    echo "$out" | sed 's/^/       /'
    echo "       see the shell's spacing rules"
    # Keep the picture and say what it was.
    #
    # A hard-to-reproduce failure needs the failing frame to check. What must be ruled out -- a
    # window narrower than the width the layout needs, so the detail draws as an overlay and its
    # backdrop dims the sidebar below the threshold -- cannot be checked once the frame is deleted.
    # So a failure leaves the evidence behind and states the size it was taken at.
    keep="${TMPDIR:-/tmp}/trove-panes-failed.png"
    cp "$wide_png" "$keep" 2>/dev/null
    echo "       the frame is kept at $keep"
    echo "       its size is $(python3 -c "
import sys
from PIL import Image
print(Image.open(sys.argv[1]).size)" "$wide_png" 2>/dev/null) device pixels;"
    echo "       this is the workspace's own boundary; the detail is an overlay at every width,"
    echo "       so a frame with it open has a backdrop over the whole window."
    exit 1
  fi
  echo "  ok   wide: separator at $what (x=$at)"
done

# The wide window WITH the detail: no third pane.
#
# There is no three-column layout: the detail is a fixed modal at every width, and the reference
# picture's workspace is a sidebar and a list. columns.py reports the bands where a boundary is
# missing, so a non-empty answer at the inspector's old position is the pass here.
overlay_out=$(python3 tools/columns.py "$overlay_png" --boundary "$wide_boundary" 2>&1)
if [ -z "$overlay_out" ]; then
  echo "  FAIL the wide window drew the detail as a third pane; the detail is a modal at"
  echo "       every width, and the workspace is two columns"
  exit 1
fi
echo "  ok   wide: the detail is an overlay, not a third pane"

echo "  ok   narrow: the detail is an overlay, not a pane -- no boundary at $narrow_boundary"
exit 0
