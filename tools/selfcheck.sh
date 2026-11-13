#!/usr/bin/env bash
# Proves that each checker still catches the defect it was written for.
#
# A checker only proves something when it is fed a case it should have caught. Three ways a
# checker fails silently:
#
#   unread.py      a false positive when a field is read through `Rail(...).Field`
#   surfaces.py    reading the wrong path and reporting the index as missing
#   twins.py       name counting that cannot see two different names, or a Min/Max prefix moved
#                  to the end so the axis word is at the other end too
#
# This is that set made executable: for each checker, break the thing it watches, assert it
# says so, put it back.
#
# The cases are real properties of the shell:
#
#   unread      RailTokens.TagBg and .TagText are assigned in both themes and read by nothing
#   surfaces    a second bulk bar beside the first must be named in the surface index
#   twins       internal/ui clamps sidebar widths against 180..320 while settings persist
#               against 260..420
#   surfaces    the "cannot be photographed" list is a second surface property: a panel gated
#               by EntranceProgress cannot be captured, so the index must name it or nobody will
#               know to verify it through Tester instead
#   bigfiles    a file can hold every surface the window draws and still be unreadable: the
#               other checkers all ask whether something is connected, not whether the file has
#               grown past one job
#
#     bash tools/selfcheck.sh
#
# Each case edits one file, runs one checker, looks for a fragment, and restores from a copy.
# The restore is a copy rather than a reverse edit, so a wrong edit cannot leave the tree
# changed. Cases that skip are counted as failures: a case that no longer applies is a case
# that no longer proves anything.

set -u
cd "$(dirname "$0")/.."

pass=0
skipped=0
fail=0
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

# run <name> <checker> <file> <edit-script> <expected-fragment>
#
# <edit-script> is a Python file taking the path as argv[1] and editing it in place. Writing
# each edit as a file rather than a shell-quoted expression keeps the quoting out of the way:
# passing regexes through printf into python can lose a backslash on the way.
run() {
  local name="$1" checker="$2" file="$3" script="$4" want="$5"
  local backup="$tmp/$(basename "$file").bak"
  cp "$file" "$backup"

  if ! python3 "$script" "$file" 2>"$tmp/edit.err"; then
    echo "  SKIP $name: $(head -1 "$tmp/edit.err")"
    cp "$backup" "$file"
    # Counted separately as well as counted as a failure.
    #
    # A skip still fails the run -- the comment above says why -- but the summary used to read
    # "N passed, 1 failed" for a skip and for a real failure alike, so the one line anyone reads
    # could not tell them apart. The detail line always could; the summary now can too.
    skipped=$((skipped + 1))
    fail=$((fail + 1))
    return
  fi

  local out
  out=$(python3 "tools/$checker" 2>/dev/null | tr '\n' ' ')
  cp "$backup" "$file"

  case "$out" in
    *"$want"*)
      echo "  ok   $name"
      pass=$((pass + 1))
      ;;
    *)
      echo "  FAIL $name: tools/$checker did not report it"
      echo "       wanted: $want"
      echo "       got:    ${out:-<empty>}"
      fail=$((fail + 1))
      ;;
  esac
}

# --- the edits, one file each ------------------------------------------------------------

cat > "$tmp/unread.py" <<'PY'
import re, sys
path = sys.argv[1]
source = open(path, encoding="utf-8").read()
source, n = re.subn(r"\tCardTagBg   ui\.Color", "\tTagBg       ui.Color\n\tCardTagBg   ui.Color", source, count=1)
if not n:
    sys.exit("CardTagBg's declaration moved")
open(path, "w", encoding="utf-8").write(source)
PY

cat > "$tmp/surfaces.py" <<'PY'
import re, sys
path = sys.argv[1]
source = open(path, encoding="utf-8").read()
source, n = re.subn(r"^\| `batchBar`.*\n", "", source, count=1, flags=re.M)
if not n:
    sys.exit("the batchBar row moved")
open(path, "w", encoding="utf-8").write(source)
PY

cat > "$tmp/surfaces_animated.py" <<'PY'
import re, sys
path = sys.argv[1]
source = open(path, encoding="utf-8").read()
# The animated table's row. vocabularyReviewPanel is an animated surface, so the checker is
# exercised against it.
source, n = re.subn(r"^\| `vocabularyReviewPanel`\s*\| `ReviewOpen`.*\n", "", source, count=1, flags=re.M)
if not n:
    sys.exit("the vocabularyReviewPanel row in the animated table moved")
open(path, "w", encoding="utf-8").write(source)
PY

cat > "$tmp/surfaces_unverified.py" <<'PY'
import re, sys
path = sys.argv[1]
source = open(path, encoding="utf-8").read()
source, n = re.subn(r"^\| `vocabularyReviewPanel`\s*\| `ReviewOpen`.*\n",
                    "| `vocabularyReviewPanel` | `ReviewOpen` | **unverified** |\n",
                    source, count=1, flags=re.M)
if not n:
    sys.exit("the vocabularyReviewPanel row in the animated table moved")
open(path, "w", encoding="utf-8").write(source)
PY

cat > "$tmp/toolrefs.py" <<'PY'
import re, sys
path = sys.argv[1]
source = open(path, encoding="utf-8").read()
# The plan cites where Tidy vocabulary lives. The case follows the citation rather than a fixed
# string: it takes whatever file:line the plan names first and pushes the line number past the
# end.
import re as _re
m = _re.search(r"`([A-Za-z0-9_./-]+\.go):(\d+)`", source)
if not m:
    sys.exit("the plan cites no file:line")
source, n = _re.subn(_re.escape(m.group(0)), "`%s:99999`" % m.group(1), source, count=1)
if not n:
    sys.exit("the citation could not be rewritten")
open(path, "w", encoding="utf-8").write(source)
PY

cat > "$tmp/namesort.py" <<'PY'
import sys
path = sys.argv[1]
source = open(path, encoding="utf-8").read()
needle = "domain.CompareNaturalNames(summaries[i].Name, summaries[j].Name)"
if needle not in source:
    sys.exit("TagSummaries no longer re-sorts in Go")
open(path, "w", encoding="utf-8").write(source.replace(needle, "strings.Compare(summaries[i].Name, summaries[j].Name)", 1))
PY

cat > "$tmp/bigfiles.py" <<'PY'
import sys
path = sys.argv[1]
source = open(path, encoding="utf-8").read()
# Pad the file past the cap without touching what it does.
open(path, "w", encoding="utf-8").write(source + "\n" + "\n".join("// filler %d" % i for i in range(900)) + "\n")
PY

cat > "$tmp/hexcolors.py" <<'PY'
import sys
path = sys.argv[1]
source = open(path, encoding="utf-8").read()
needle = "theme.Text)"
if needle not in source:
    sys.exit("no theme.Text) to replace")
# A bare Hex where a token belongs: a colour written outside the token table.
open(path, "w", encoding="utf-8").write(source.replace(needle, 'ui.Hex("#b83d2e")', 1))
PY

cat > "$tmp/twins.py" <<'PY'
import sys
path = sys.argv[1]
source = open(path, encoding="utf-8").read()
needle = "WidthSidebarMin = 168"
if needle not in source:
    sys.exit("WidthSidebarMin no longer holds 168")
open(path, "w", encoding="utf-8").write(source.replace(needle, "WidthSidebarMin = 180", 1))
PY

# columns.py takes a picture, not a file to break, so its case is a synthetic one. Two images
# that differ in exactly the property it asserts: one where paint stops at the pane boundary,
# one where it runs across. Written as a picture because the defect it watches for -- the middle
# pane drawing over the inspector -- leaves no trace in the source code. The only evidence is the
# screenshot.
cat > "$tmp/columns_case.py" <<'PY'
import sys
from PIL import Image

def make(path, rule):
    # The shell's surfaces are all near-white -- the results pane is (244,244,245) and the
    # window is (255,255,255) -- so the case is built that way too. A picture with a big black
    # gap in it would pass the check for the wrong reason and prove nothing.
    width, height = 1600, 400
    image = Image.new("RGB", (width, height), (255, 255, 255))
    pixels = image.load()
    for x in range(0, 900):
        for y in range(0, height):
            pixels[x, y] = (244, 244, 245)
    if rule:
        for y in range(0, height):
            pixels[900, y] = (217, 217, 222)
    image.save(path)

boundary = int(sys.argv[2])
make(sys.argv[1] + "/ruled.png",  True)    # a separator stands at the boundary
make(sys.argv[1] + "/unruled.png", False)  # the two surfaces meet with nothing between
PY
python3 "$tmp/columns_case.py" "$tmp" 400

echo "checkers catch the defect each was written for:"

run "unread sees an unread field" \
    unread.py internal/ui/theme.go "$tmp/unread.py" "TagBg"
run "surfaces sees an unlisted surface" \
    surfaces.py docs/ui-surface-index.md "$tmp/surfaces.py" "batchBar"
run "surfaces sees an animated panel left out" \
    surfaces.py docs/ui-surface-index.md "$tmp/surfaces_animated.py" "ReviewOpen gates a surface"
run "surfaces refuses an unverified panel" \
    surfaces.py docs/ui-surface-index.md "$tmp/surfaces_unverified.py" "no test named in the index"
run "toolrefs sees a citation past the end" \
    toolrefs.py docs/architecture.md "$tmp/toolrefs.py" "past the end of the file"
run "namesort sees a name left in SQL" \
    namesort.py internal/storage/sqlite/tag_repo.go "$tmp/namesort.py" "and no CompareNaturalNames"
run "bigfiles sees a file past the cap" \
    bigfiles.py internal/ui/card.go "$tmp/bigfiles.py" "over the 900 cap"
run "twins sees two bounds that disagree" \
    twins.py internal/ui/metrics.go "$tmp/twins.py" "minsidebarwidth"

run "hexcolors sees a colour outside the token table" \
    hexcolors.py internal/ui/settingsui.go "$tmp/hexcolors.py" "ui.Hex(#b83d2e)"

# Every checker says what it counted, even when it found nothing.
#
# A checker whose summary sits behind --verbose prints nothing when clean, so "no output" then
# means both "nothing wrong" and "the check did not run". The failure mode is a path error and a
# missing separator looking the same.
silent=""
for checker in bigfiles hexcolors namesort surfaces toolrefs unread; do
  if [ -z "$(python3 tools/$checker.py 2>&1)" ]; then
    silent="$silent $checker"
  fi
done
if [ -z "$silent" ]; then
  echo "  ok   every checker says what it counted, even when clean"; pass=$((pass + 1))
else
  echo "  FAIL these checkers print nothing when clean:$silent"; fail=$((fail + 1))
fi

# A tool run from the wrong directory says so.
#
# These read their inputs relative to the repository root and used to answer from wherever they
# were run. A wrong directory still produced something that looked like a result.
root=$(pwd)
unwarned=""
for checker in twins bigfiles orphankeys; do
  out=$(cd .. && python3 "$root/tools/$checker.py" 2>&1)
  case "$out" in
    *"relative to the repository root"*) ;;
    *) unwarned="$unwarned $checker" ;;
  esac
done
if [ -z "$unwarned" ]; then
  echo "  ok   a checker run from the wrong directory warns about it"; pass=$((pass + 1))
else
  echo "  FAIL these checkers answered from the wrong directory in silence:$unwarned"; fail=$((fail + 1))
fi

# cardrows.py finds the two addresses it needs, on a picture built for the purpose.
#
# Scanning down "the card's left border" at a guessed x can hit the text or the sidebar's edge and
# learn nothing. This case feeds a synthetic list: a sidebar border that runs the full height, and
# two cards that do not, so a tool that confuses them fails here.
cat > "$tmp/cardrows_case.py" <<'PY'
import sys
from PIL import Image, ImageDraw
page = (244, 244, 245)
border = (217, 217, 222)
w, h = 1200, 800
im = Image.new("RGB", (w, h), page)
d = ImageDraw.Draw(im)
d.line([(280, 0), (280, h)], fill=border)          # the sidebar's edge: every row
for top, bottom in ((60, 260), (340, 540)):        # two cards: not every row
    d.rounded_rectangle([(300, top), (1150, bottom)], radius=16, outline=border)
    d.text((320, top + 20), "a card", fill=(30, 30, 34))
im.save(sys.argv[1])
PY
python3 "$tmp/cardrows_case.py" "$tmp/cards.png"
out=$(python3 tools/cardrows.py "$tmp/cards.png" 2>&1)
if printf '%s' "$out" | grep -q "border x=300"; then
  echo "  ok   cardrows finds the card edge past the sidebar's"; pass=$((pass + 1))
else
  echo "  FAIL cardrows did not find the card edge"; echo "       got: ${out:-<empty>}"; fail=$((fail + 1))
fi
if printf '%s' "$out" | grep -q "sidebar's own edge is at x=280"; then
  echo "  ok   cardrows tells the sidebar's edge from a card's"; pass=$((pass + 1))
else
  echo "  FAIL cardrows did not identify the sidebar's edge"; echo "       got: ${out:-<empty>}"; fail=$((fail + 1))
fi

# contrast.py finds text without being told where it is, and its tile size traces to the theme.
#
# The tile size is tied to the smallest Font token in internal/ui/metrics.go, which is the same
# move --boundary makes by taking its address from the layout. This case checks that the
# derivation runs and that the tool reports something.
out=$(python3 tools/contrast.py docs/screenshots/baseline/populated-grid-light.png 2>&1)
if printf '%s' "$out" | grep -q "from the smallest Font token"; then
  echo "  ok   contrast derives its tile size from the theme"; pass=$((pass + 1))
else
  echo "  FAIL contrast did not derive its tile size"; echo "       got: ${out:-<empty>}"; fail=$((fail + 1))
fi
if printf '%s' "$out" | grep -q "text-like tiles"; then
  echo "  ok   contrast finds text-like tiles in a populated capture"; pass=$((pass + 1))
else
  echo "  FAIL contrast found no text-like tiles"; echo "       got: ${out:-<empty>}"; fail=$((fail + 1))
fi

# A screen with no cards is a result, not a tool failure.
#
# An empty screen and a missing file must not share an exit code: over empty-dark the tool must
# report "no cards" (0), and over a missing path "cannot read the file" (2).
python3 tools/cardrows.py docs/screenshots/baseline/empty-dark.png >/dev/null 2>&1
nocards=$?
python3 tools/cardrows.py /nonexistent.png >/dev/null 2>&1
broken=$?
if [ "$nocards" -eq 0 ] && [ "$broken" -eq 2 ]; then
  echo "  ok   cardrows tells 'no cards here' from 'cannot read the file'"; pass=$((pass + 1))
else
  echo "  FAIL cardrows exit codes: no-cards=$nocards (want 0), cannot-open=$broken (want 2)"
  fail=$((fail + 1))
fi

echo "  -- columns, on synthetic pictures --"
out=$(python3 tools/columns.py "$tmp/unruled.png" --boundary 900 2>&1 | tr '\n' ' ')
case "$out" in
  *"no separator at x=900"*)
    echo "  ok   columns sees two surfaces meeting with no separator"; pass=$((pass + 1)) ;;
  *) echo "  FAIL columns did not report the missing separator"; echo "       got: ${out:-<empty>}"; fail=$((fail + 1)) ;;
esac

# The exit codes, which are a convention rather than a check.
#
# A path that does not open and a separator that is not there must not share an exit code: 2 means
# the tool could not run and 1 means it ran and the picture is wrong. This case keeps the two
# apart.
python3 tools/columns.py "$tmp/does-not-exist.png" --boundary 900 >/dev/null 2>&1
if [ $? -eq 2 ]; then
  echo "  ok   columns exits 2 for a path it cannot open"; pass=$((pass + 1))
else
  echo "  FAIL columns did not exit 2 for a missing path (got $?)"; fail=$((fail + 1))
fi

python3 tools/columns.py "$tmp/unruled.png" --boundary 900 >/dev/null 2>&1
if [ $? -eq 1 ]; then
  echo "  ok   columns exits 1 for a picture that is wrong"; pass=$((pass + 1))
else
  echo "  FAIL columns did not exit 1 for a missing separator (got $?)"; fail=$((fail + 1))
fi

python3 tools/columns.py "$tmp/ruled.png" --boundary 900 >/dev/null 2>&1
if [ $? -eq 0 ]; then
  echo "  ok   columns exits 0 for a picture that is right"; pass=$((pass + 1))
else
  echo "  FAIL columns did not exit 0 for a separator that is there (got $?)"; fail=$((fail + 1))
fi

out=$(python3 tools/columns.py "$tmp/ruled.png" --boundary 900 2>&1 | tr '\n' ' ')
if [ -z "$out" ]; then
  echo "  ok   columns accepts a separator on two near-white surfaces"; pass=$((pass + 1))
else
  echo "  FAIL columns reported a drawn separator"; echo "       got: $out"; fail=$((fail + 1))
fi

cat > "$tmp/renamecardaction.py" <<'PY'
import sys
path = sys.argv[1]
source = open(path, encoding="utf-8").read()
# Rename the method the audit's table names for the card's action row. A rename is the everyday
# way a render point disappears: the code still builds, the row is simply drawn by nothing now.
source, n = source.replace("func (s *Shell) cardActions(", "func (s *Shell) cardActionsGone("), 1
if not n:
    sys.exit("the cardActions method moved")
open(path, "w", encoding="utf-8").write(source)
PY

run "targetaudit: a renamed drawer" targetaudit.py internal/ui/card.go "$tmp/renamecardaction.py" "no render point"


# The summary goes last, after every case, and so does `[ "$fail" -eq 0 ]`.
#
# If the exit test sits earlier, the script's exit status becomes the *last* command's (a
# restore, effectively 0) and the gate reports "ok every checker still catches its defect" while
# a case is skipping. A stale needle in a case is enough to hide a real failure.
echo
if [ "$skipped" -gt 0 ]; then
  echo "$pass passed, $fail failed ($skipped of them skipped)"
else
  echo "$pass passed, $fail failed"
fi
[ "$fail" -eq 0 ]
