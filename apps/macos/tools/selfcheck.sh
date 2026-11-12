#!/usr/bin/env bash
# Proves that each checker still catches the defect it was written for.
#
# Every checker in tools/ had a first version that was wrong, and each of those only showed up
# when it was fed a case it should have caught:
#
#   unread.py      its first version, once pointed at RailTokens, reported a false positive:
#                  DisplayFace is read through `Rail(...).Field`, which its pattern missed
#   deadtokens.py  its first version read a comment that named a token in order to reject it,
#                  and counted that as a use
#   surfaces.py    its first version read the wrong path and said the index was missing
#   twins.py       its first two versions could not see the pair they were written for: name
#                  counting cannot see two different names, and moving a Min/Max prefix to the
#                  end cannot see that the axis word is at the other end too
#
# Those findings live in docs/implementation-log.md, which nothing runs. This is the same set
# made executable: for each checker, break the thing it watches, assert it says so, put it back.
#
# The cases are not invented. Each defect actually shipped:
#
#   unread      RailTokens.TagBg and .TagText were assigned in both themes and read by nothing
#               (round 204)
#   deadtokens  the results pane filled itself with --main-bg, which no rule in the extension
#               references (round 206)
#   surfaces    a second bulk bar existed for two rounds because the surface index did not say
#               the first one did (rounds 214-216)
#   twins       internal/ui clamped sidebar widths against 180..320 while settings persisted
#               against 260..420 (round 218)
#   surfaces    the "cannot be photographed" list is a second surface property, added in round
#               222: a panel gated by EntranceProgress cannot be captured, so the index must
#               name it or nobody will know to verify it through Tester instead
#   bigfiles    shell.go reached 5419 lines holding every surface the window draws, and none of
#               the other checkers could see it: they all ask whether something is connected,
#               and a file can be entirely connected and still be unreadable (round 238)
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
# the first version of this script passed regexes through printf into python and lost a
# backslash on the way.
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

cat > "$tmp/deadtokens.py" <<'PY'
import sys
path = sys.argv[1]
source = open(path, encoding="utf-8").read()
# The results pane's fill, whichever way it is derived. Round 238 re-rooted it on the platform
# theme; the derivation later became resultsSurfaceFor, and the needle stayed on the old line,
# so from then until round 503 this case skipped and the summary counted it as a failure the
# gate never saw (the exit status was computed above the last case). The needle now names the
# line as it is, and the case is about the mechanism -- a token the stylesheet defines but never
# uses, which deadtokens.py has to see -- not about which expression fills the pane.
needle = "Results:         resultsSurfaceFor(dark),"
if needle not in source:
    sys.exit("the results pane no longer takes resultsSurfaceFor")
open(path, "w", encoding="utf-8").write(
    source.replace(needle, "Results:         resultsSurfaceFor(dark), // --main-bg", 1))
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
# The animated table's row. It used settingsPanel until settings became a page and the
# row legitimately left the table (S37); vocabularyReviewPanel is still an animated
# surface, so the checker is exercised against one of those.
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
# The plan cites where Tidy vocabulary lives. Round 238 moved it, so the case follows the
# citation rather than a fixed string: it takes whatever file:line the plan names first and
# pushes the line number past the end.
import re as _re
m = _re.search(r"`([a-z_]+\.go):(\d+)`", source)
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
# A bare Hex where a token belongs, which is the shape both round-284 findings had.
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
# one where it runs across. Written as a picture because the defect it watches for -- eight
# rounds of width arithmetic during which the middle pane drew over the inspector -- left no
# trace in the source. The only evidence was the screenshot.
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
run "deadtokens sees a dead token in use" \
    deadtokens.py internal/ui/theme.go "$tmp/deadtokens.py" "main-bg"
run "surfaces sees an unlisted surface" \
    surfaces.py ../../docs/ui-surface-index.md "$tmp/surfaces.py" "batchBar"
run "surfaces sees an animated panel left out" \
    surfaces.py ../../docs/ui-surface-index.md "$tmp/surfaces_animated.py" "ReviewOpen gates a surface"
run "surfaces refuses an unverified panel" \
    surfaces.py ../../docs/ui-surface-index.md "$tmp/surfaces_unverified.py" "no test named in the index"
run "toolrefs sees a citation past the end" \
    toolrefs.py ../../docs/ui-implementation-plan.md "$tmp/toolrefs.py" "past the end of the file"
run "namesort sees a name left in SQL" \
    namesort.py internal/storage/sqlite/tag_repo.go "$tmp/namesort.py" "and no CompareNaturalNames"
run "bigfiles sees a file past the cap" \
    bigfiles.py internal/ui/card.go "$tmp/bigfiles.py" "over the 900 cap"
run "twins sees two bounds that disagree" \
    twins.py internal/ui/metrics.go "$tmp/twins.py" "minsidebarwidth"

run "hexcolors sees a colour outside the token table" \
    hexcolors.py internal/ui/settingsui.go "$tmp/hexcolors.py" "ui.Hex(#b83d2e)"

# A checker with a two-level result has to say both levels.
#
# Round 324 ran deadtokens.py looking for dead tokens, got no output, and read that as "none" --
# while the answer was six, all either allowlisted or not used by this build. Printing nothing and
# finding nothing were the same output, the same shape round 317 found in the exit codes.
deadtokens_out=$(python3 tools/deadtokens.py 2>&1)
case "$deadtokens_out" in
  *"dead,"*"without an allowlist entry"*)
    echo "  ok   deadtokens says how many it found, and how many are known"; pass=$((pass + 1)) ;;
  *) echo "  FAIL deadtokens printed no counts: ${deadtokens_out:-<empty>}"; fail=$((fail + 1)) ;;
esac

# Every checker says what it counted, even when it found nothing.
#
# Round 326 ran all of them and found five -- bigfiles, hexcolors, namesort, surfaces, toolrefs --
# printing nothing at all when clean, because their summaries sat behind --verbose. "No output"
# then meant both "nothing wrong" and "the check did not run", which is how round 287 lost a round
# to eighteen screenshots that all reported FAIL because a path error and a missing separator
# looked the same.
silent=""
for checker in bigfiles hexcolors namesort surfaces toolrefs deadtokens unread; do
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
# Eight of these read their inputs relative to apps/macos and used to answer from wherever they
# were run: twins.py reported nothing from the repo root and six findings from here, i18ncheck 4
# lines and 97. Round 331 checked the callers -- all eight shell scripts pin the directory -- so
# nothing was misreading, but a wrong directory still produced something that looked like a result.
unwarned=""
for checker in i18ncheck twins bigfiles orphankeys; do
  out=$(cd ../.. && python3 apps/macos/tools/$checker.py 2>&1)
  case "$out" in
    *"reads paths relative to apps/macos"*) ;;
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
# Round 340 scanned down "the card's left border" at a guessed x, hit the text at x=566 and the
# sidebar's edge at x=561, and learned nothing. This case feeds a synthetic list: a sidebar border
# that runs the full height, and two cards that do not, so a tool that confuses them fails here.
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

# copycheck.py reports a count, and the count has to be there to be a gauge.
out=$(python3 tools/copycheck.py 2>&1)
case "$out" in
  *"English strings in copy.ts"*"not present verbatim"*)
    echo "  ok   copycheck reports how much of the source copy is present"; pass=$((pass + 1)) ;;
  *) echo "  FAIL copycheck printed no counts: ${out:-<empty>}"; fail=$((fail + 1)) ;;
esac

# contrast.py finds text without being told where it is, and its tile size traces to the theme.
#
# Round 370's answer moved with --tile; round 371 tied the tile to the smallest Font token in
# internal/ui/metrics.go, which is the same move --boundary makes by taking its address from the
# layout. This case checks that the derivation runs and that the tool reports something.
out=$(python3 tools/contrast.py ../../docs/screenshots/baseline/populated-grid-light.png 2>&1)
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
# Round 376 ran cardrows.py over empty-dark, got exit 2, and could not tell that from a missing
# file -- the same shape round 317 fixed in columns.py and round 375 found in contrast.py, where
# the dark appearance was a domain the tool had never been run on.
python3 tools/cardrows.py ../../docs/screenshots/baseline/empty-dark.png >/dev/null 2>&1
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
# Round 287 ran columns.py over eighteen screenshots from the wrong directory, every path failed
# to open, and the loop read the "No such file" text as a failed check -- because a path that does
# not open and a separator that is not there both exited 1. The fix is that 2 means the tool could
# not run and 1 means it ran and the picture is wrong, and this case is what keeps the two apart.
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

# The board is the loop's memory, and a memory that can hold two currents is a memory that
# has stopped meaning anything: WIP=1 is the rule that keeps a session from becoming a wish
# list. The edit takes the first slice row and makes it doing, then duplicates it, so the
# case does not depend on which slice happens to be open -- an earlier version looked for a
# doing row and skipped (counted as a failure) whenever the board was between slices.
cat > "$tmp/statuscheck_dup.py" <<'PY'
import sys
path = sys.argv[1]
lines = open(path, encoding="utf-8").read().splitlines()
for i, line in enumerate(lines):
    if line.strip().startswith("| ID |"):
        header = [cell.strip() for cell in line.strip().strip("|").split("|")]
        try:
            status = header.index("Status") + 1
        except ValueError:
            sys.exit("the 切片 table has no Status column")
        first = i + 2  # header + separator
        cells = lines[first].split("|")
        cells[status] = " doing "
        lines[first] = "|".join(cells)
        duplicate = list(cells)
        duplicate[1] = " DUP "
        lines.insert(first + 1, "|".join(duplicate))
        break
else:
    sys.exit("no 切片 table on the board")
open(path, "w", encoding="utf-8").write("\n".join(lines) + "\n")
PY

run "statuscheck: a second doing row" statuscheck.py ../../docs/loop/board.md "$tmp/statuscheck_dup.py" "exactly one Now"

# The summary goes last, after every case.
#
# It used to sit above the targetaudit case, and `[ "$fail" -eq 0 ]` with it -- so the script's
# exit status was the *last* command's (a restore, effectively 0) and the gate reported "ok every
# checker still catches its defect" while a case was skipping. Found in round 503, when the
# deadtokens case had been skipping unnoticed: its needle had been stale since theme.go stopped
# writing `Results: theme.Background,`.
echo
if [ "$skipped" -gt 0 ]; then
  echo "$pass passed, $fail failed ($skipped of them skipped)"
else
  echo "$pass passed, $fail failed"
fi
[ "$fail" -eq 0 ]
