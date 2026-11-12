#!/usr/bin/env bash
# Fails when a method or a field stops being reachable from the interface.
#
# Hundreds of rounds of this build produced the same defect: both ends implemented and
# nothing between them. Keyword rules, the LLM client, the tag library, the progress
# renderer, the icon renderer, saved views, multi-tag filtering, authors. Each was found
# by reading, which is why one survived fifty rounds without a UI.
#
# The two tools below report candidates rather than findings, so this does not demand
# zero: it demands that the known set does not grow. Removing a name from an allowlist is
# part of fixing the gap it names.
set -u

cd "$(dirname "$0")/.."

fail=0

# Every method here was looked at and judged: either the source has no such action
# (RejectAll), or the interface reaches the service by another route (IsRunning,
# Progress, Summary are kept in the shell's own state because a sync is always started
# in this process), or it is not a user action at all (Endpoints, SaveEndpoints,
# Withheld, ClientError). RenameList
# are still open gaps and are listed here so the count cannot grow while they wait.
# Two more, added when the sweep was widened from internal/usecase to internal/ui:
#
#   Denied, LiveRunProgress,, RemoveTagFromSelection
#       called from tests only. They are not gaps: each is either a pure mapping checked
#       against the source's keyframes, or an action whose call site is in a test.
#
#   UseAutoTagRunner
#       the seam the auto-continue chain is tested through. The engine call is replaced so
#       a test can choose the summary the continuation decides on; without it the chain can
#       only be asserted piecewise, which is exactly the state that hid a switch wired to
#       nothing for want of a caller.
# IconNames / IconPaths are read by icons_test.go's drift checks -- every alias has a path,
# every path parses -- which is what keeps tools/iconnames.py honest. Test-only seams go on
# this list with a reason; that is the mechanism, not a weakening of it.
#   SetTestedViewportHeight
#       called by the parity dump in package main (parity_geometry_test.go). A tester has no
#       window, so the detail's 86dvh cap and its media's 35dvh need the height the dump draws
#       at; this checker only follows callers inside internal/ui, so it is listed here.
#   SetTestedViewportWidth
#       called from tests inside this package (TestANarrowWindowGivesTheSidebarUp and the
#       layout-choice tests). A tester has no window, so without it the width rule that gives
#       the sidebar up below NavSideWidth could never be exercised; the checker does not count
#       test callers.
#   LoadSyncSummary
#       called from tests only. It is the read-and-install form of the summary; the
#       reload engine reads the summary through gatherReload and installs it through
#       applySyncSummary, both in reload.go. The tests call this one directly to drive
#       the adoption rules in isolation (stalesummary_test.go, lists_test.go).
uncalled_allowed="IconNames IconPaths ClientError Denied Endpoints IsRunning LiveRunProgress LoadSyncSummary Progress RejectAll RemoveTagFromSelection SaveEndpoints SetTestedViewportHeight SetTestedViewportWidth UseAutoTagRunner Withheld"

# The tools print one name per line; the case patterns below compare space-separated
# words, so the newlines are folded first. Comparing " $list " against a newline-separated
# list silently matched nothing.
uncalled=$(python3 tools/uncalled.py --names | tr '\n' ' ')
uncalled=$(python3 tools/uncalled.py --names | tr '\n' ' ')
# Fields that are assigned and never read. unread.py covers State, View and RailTokens; the
# list below is every RailTokens field it reports, with the reason it stays.
#
#   DisplayFont    the full stylesheet stack, kept for the record beside DisplayFace, which
#                  is the single family MyGo is asked for. The comment on the field says so.
#   BadgeText      the status badge's label colour. The badge draws its own text through
#                  StatusBadgeTokens, so this field has no reader; it is left rather than
#                  deleted because the token is real and the next badge will want it.
#   FieldFocus     the search field's ground once it has the cursor. MyGo's SearchField
#                  exposes no focus state, so there is nowhere to apply it.
#   Counts         the workspace's totals (bookmarks, unclassified, tags, lists). Its last
#                  reader was the sidebar's 全部书签 row, which round 508 removed at the
#                  user's request; the results header now carries the only number that row
#                  showed, and it counts results rather than the workspace. The field stays
#                  because the load already computes it and the workspace view is its only
#                  home -- delete it, and the query behind it, when no surface claims it.
#
# Adding a name here is the intended way to silence the check, and it is meant to be
# uncomfortable: round 204 deleted two fields (TagBg, TagText) that were on no list.
unread_allowed="BadgeText FieldFocus DisplayFont Counts"
unread=$(python3 tools/unread.py --names | tr '\n' ' ')

unread_unexpected=""
for name in $unread; do
  case " $unread_allowed " in
    *" $name "*) ;;
    *) unread_unexpected="$unread_unexpected $name" ;;
  esac
done

if [ -n "$unread_unexpected" ]; then
  echo "FAIL: fields that are assigned and never read:$unread_unexpected"
  echo "      each is either a gap to wire up or a name to add to the allowlist with a reason"
  python3 tools/unread.py
  exit 1
fi

unexpected=""
for name in $uncalled; do
  case " $uncalled_allowed " in
    *" $name "*) ;;
    *) unexpected="$unexpected $name" ;;
  esac
done

if [ -n "$unexpected" ]; then
  echo "FAIL: usecase methods nothing calls:$unexpected"
  echo "      each is either a gap to wire up or a name to add to the allowlist with a reason"
  fail=1
fi

# Strings the extension shows that this catalog cannot render. The count is a known debt,
# not a pass: it may not grow, and every fix should lower it. Measured against the source
# tree, so it does not drift when nothing changes here.
strings_missing=$(python3 tools/i18ncheck.py --names | wc -l | tr -d ' ')
strings_baseline=95
if [ "$strings_missing" -gt "$strings_baseline" ]; then
  echo "FAIL: $strings_missing source strings are unrenderable, up from $strings_baseline"
  echo "      run python3 tools/i18ncheck.py to see which"
  fail=1
fi

# Catalog keys nothing renders. Like the unrenderable-string count, this is a known debt
# that may not grow: the first run found 43.
orphans=$(python3 tools/orphankeys.py --names | tr '\n' ' ')
orphans_count=$(echo $orphans | wc -w | tr -d ' ')
# A key that holds a source string has to exist to satisfy the other count even while
# nothing renders it, so this number went from 40 to 41 when settingsSectionAutoTag was
# restored. The two metrics pull in opposite directions and both are real.
#
# Round 509 took it from 6 to 7: sidebar.library ("资料库") was the results header's overline,
# and the user asked for that header to be compacted (shell-contract.md S24). The key stays
# because copy.ts:37 holds the string, and nothing draws it until a surface claims the word
# again -- deleting it would raise the unrenderable-string count instead.
#
# Round 530 took it from 7 to 9: action.loadMore and action.showMore lost their controls.
# The results are read by scrolling now (pagination.go), so neither "加载更多" nor "显示更多"
# is drawn. Both strings are in copy.ts, so they stay in the catalog -- the same trade as
# sidebar.library above -- and the two extra entries are the price of removing a button the
# product does not want.
orphans_baseline=9
if [ "$orphans_count" -gt "$orphans_baseline" ]; then
  echo "FAIL: $orphans_count catalog keys are rendered nowhere, up from $orphans_baseline"
  fail=1
fi

# An icon name the set does not have draws nothing, and an empty box reads as a rendering
# fault rather than a typo. The footer's three buttons shipped blank this way.
if ! python3 tools/iconnames.py > /dev/null; then
  echo "FAIL: an icon name is not in the icon set"
  python3 tools/iconnames.py
  fail=1
fi

# Tokens the stylesheet defines, never uses, and this build renders by name. Round 206 found
# --main-bg this way: the results pane was filling itself with a colour no rule in the
# extension references. The direction is the reverse of every other tool here -- it asks
# whether the source uses what this side does -- and it needs no component reading.
deadtokens=$(python3 tools/deadtokens.py | tr '\n' ' ')
if [ -n "$deadtokens" ]; then
  echo "FAIL: this build renders tokens the stylesheet never uses:$deadtokens"
  echo "      each is either a surface to re-source or a name to allowlist in tools/deadtokens.py"
  python3 tools/deadtokens.py --all
  fail=1
fi

# docs/ui-surface-index.md must list every surface the shell draws. It exists because round
# 214 built a second bulk bar without noticing the first; a hand-written list goes stale, and
# rounds 190/191 showed what that costs. Both directions are checked: a surface that is not
# listed, and a name that is listed but draws nothing.
surfaces=$(python3 tools/surfaces.py | tr '\n' ' ')
if [ -n "$surfaces" ]; then
  echo "FAIL: docs/ui-surface-index.md and the shell disagree:$surfaces"
  echo "      add the surface, or remove the name -- the index is what stops a second one"
  fail=1
fi

# Every element of the reference picture must have somewhere that draws it.
#
# Round 466 added this because the objective makes "does the reference's every surface have a
# render point" its own criterion, and a prose answer rots: a part can be deleted from the shell
# while the document still says it is drawn. tools/targetaudit.py names, for each visible element,
# the Go symbol that draws it and the metric its size comes from; both are looked up in the tree,
# so a renamed method or a deleted constant fails here rather than in someone's memory.
if ! python3 tools/targetaudit.py --strict > /dev/null; then
  echo "FAIL: a visible element of the reference picture has no render point"
  python3 tools/targetaudit.py --strict | grep -E "MISSING|no render point|name a size" | head -12
  fail=1
fi

# Two names for one number is the defect round 218 found: internal/ui clamped sidebar widths
# against 180..320 while internal/usecase/settings persisted against 260..420, and a stored 400
# was accepted by one and shrunk by the other. Neither name appears twice, so only a check
# that compares names by their words can see it.
twins=$(python3 tools/twins.py | tr '\n' ' ')
if [ -n "$twins" ]; then
  echo "FAIL: constants that mean the same thing hold different values:$twins"
  echo "      one of the two is the source of truth; run python3 tools/twins.py --all"
  fail=1
fi

# `path:line` citations in docs/ must land inside the file they name. Four rounds of clearing
# stale tables (190/191/217/218) showed that a table goes out of date; a citation is the part
# of that prose a script can check, and the numbers move whenever the code above them does.
toolrefs=$(python3 tools/toolrefs.py | tr '\n' ' ')
if [ -n "$toolrefs" ]; then
  echo "FAIL: a citation in docs/ does not land where it says:$toolrefs"
  echo "      either the line moved or the file is gone; run python3 tools/toolrefs.py --verbose"
  fail=1
fi

# Rounds 186, 227, 228 and 229 each found a name ordered by SQL's byte comparison while the
# extension orders names through Intl.Collator -- and two of the four were a second query in a
# package whose first query already did it right. The rule is one comparator for one question:
# ORDER BY may order counts and clusters, and a name order belongs in Go.
namesort=$(python3 tools/namesort.py | tr '\n' ' ')
if [ -n "$namesort" ]; then
  echo "FAIL: a name is ordered in SQL with no natural comparison in the same function:$namesort"
  echo "      re-sort in Go with domain.CompareNaturalNames, or allowlist it in tools/namesort.py"
  fail=1
fi

# A file can be entirely connected and still be unreadable. Round 238 found shell.go at 5419
# lines; the other checkers all ask whether something is wired, and none asked whether a file
# had grown past one job.
bigfiles=$(python3 tools/bigfiles.py | tr '\n' ' ')
if [ -n "$bigfiles" ]; then
  echo "FAIL: a Go file has grown past one responsibility:$bigfiles"
  echo "      split it by what it does, or allowlist it in tools/bigfiles.py with a reason"
  fail=1
fi

# Three panes, two boundaries, and nothing that could see one of them missing.
#
# Rounds 246-255 spent eight rounds on widths -- Width(304), Fill, FillHeight, Spacer,
# Row.FillWidth, then arithmetic -- and none of them moved the picture. Scanning a horizontal
# line across docs/screenshots/255-inspector-fixed.png showed paint running unbroken from x=560
# to x=2359 while the inspector's declared boundary is at x=1752: the middle pane was drawing
# over the pane beside it. 1054 tests and nine checkers verify values and wiring, and not one
# of them can say what the shell's space is supposed to look like.
#
# Both pictures are kept, because the pair is the evidence: 259-inspector-unbordered.png is the
# build before the boundary was drawn and 259-inspector-bordered.png is after, and the check has
# to tell them apart. A checker that only ever saw one of them could pass by accident.
columns=$(python3 tools/columns.py \
    ../../docs/screenshots/259-inspector-bordered.png --boundary 1752 2>&1 | tr '\n' ' ')
if [ -n "$columns" ]; then
  echo "FAIL: the inspector pane has no separator at its left edge:$columns"
  echo "      the panes read as one surface; see docs/shell-contract.md S1 and S3"
  fail=1
fi

# And the pair must still differ in the way the check watches for. If a later change makes the
# unbordered picture pass too -- the boundary moving, or the tolerance widening until anything
# counts -- the case above stops proving anything. This is the mutation, run every time.
columns_before=$(python3 tools/columns.py \
    ../../docs/screenshots/259-inspector-unbordered.png --boundary 1752 2>&1 | tr '\n' ' ')
if [ -z "$columns_before" ]; then
  echo "FAIL: the unbordered baseline now passes, so the separator check proves nothing"
  echo "      it must fail on 259-inspector-unbordered.png and pass on the bordered one"
  fail=1
fi

# Cinnabar in one place, and no colour a theme cannot reach.
#
# Round 284 found two violations by reading: StatusBadge drew StatusRunning with a second copy of
# cinnabar, and railTagAccent drew the graph's focused node with it. Both took a bool and chose
# between two hardcoded values, so neither could follow the system accent -- and neither shows up
# in a screenshot, because a hardcoded cinnabar and the theme's accent are the same pixels until
# the system accent changes.
hexcolors=$(python3 tools/hexcolors.py | tr '\n' ' ')
if [ -n "$hexcolors" ]; then
  echo "FAIL: a colour is written outside the token table:$hexcolors"
  echo "      theme.go is the one file that mirrors the stylesheet's variables;"
  echo "      a Hex anywhere else is a colour no theme can reach"
  fail=1
fi

# A dialog's buttons must not both be "Cancel", because choosing the first one acts. Three of
# the four AlertDialogs in dialogs.go had that until round 304: the label said Cancel and the
# choice deleted. The fourth passed two different strings and was the shape all four wanted.
twincancel=$(grep -n "ActionCancel), s.t().T(i18n.ActionCancel))" internal/ui/dialogs.go | tr '\n' ' ')
if [ -n "$twincancel" ]; then
  echo "FAIL: two dialog buttons carry the same label:$twincancel"
  echo "      a button's label and what it does are two things; check the pair"
  fail=1
fi

# Cancel comes first in every confirmation, so the destructive button is never the one under the
# cursor when a dialog appears by reflex. confirm.go states that rule for Confirmation and its
# comment gives the reason; the hand-written AlertDialogs in dialogs.go did the opposite until
# round 306, passing [action, cancel] and treating index 0 as the action.
#
# The check reads the buttons line, which is the one carrying two labels -- the lines above it are
# the title and the body, and the title is often the same word as the action.
notcancel=$(grep -nE "^\s+s\.t\(\)\.T\(i18n\.[A-Za-z]+\), s\.t\(\)\.T\(i18n\.ActionCancel\)\)" internal/ui/dialogs.go | tr '\n' ' ')
if [ -n "$notcancel" ]; then
  echo "FAIL: a dialog's first button is not the cancel label:$notcancel"
  echo "      confirm.go's Buttons() puts cancel first; see its comment for why"
  fail=1
fi

early=$(python3 tools/earlyreturn.py --names | tr '\n' ' ')
if [ -n "$early" ]; then
  echo "FAIL: a return escapes a Children closure from inside a loop:$early"
  echo "      the rest of the loop does not run; use if/else or continue"
  fail=1
fi

# RailTokens fields that are assigned and never read, with the reason each stays. unread.py
# covered State and View until this round; adding RailTokens found three, all deliberate:
#
#   DisplayFont    the full stylesheet stack, kept for the record beside DisplayFace, which
#                  is the single family MyGo is asked for. The field's own comment says so.
#   BadgeText      the status badge's label colour. The badge draws its text through
#                  StatusBadgeTokens, so this has no reader; the token is real, so it stays.
#   FieldFocus     the search field's ground once it has the cursor. MyGo's SearchField
#                  exposes no focus state, so there is nowhere to apply it.
#
#   Counts         the workspace's totals. Its last reader was the sidebar's 全部书签 row,
#                  removed in round 508 at the user's request; the results header counts what
#                  is shown instead. The workspace view is the field's only home, so it stays
#                  until a surface claims it or the load stops producing it.
#
# Round 204 deleted two fields (TagBg, TagText) that were on no list at all, which is the
# state this check exists to prevent.
unread_allowed="BadgeText FieldFocus DisplayFont Counts"
unread=$(python3 tools/unread.py --names | tr '\n' ' ')
unread_unexpected=""
for name in $unread; do
  case " $unread_allowed " in
    *" $name "*) ;;
    *) unread_unexpected="$unread_unexpected $name" ;;
  esac
done
if [ -n "$unread_unexpected" ]; then
  echo "FAIL: fields the interface never reads:$unread_unexpected"
  echo "      each is either a gap to wire up or a name to add to the allowlist with a reason"
  python3 tools/unread.py
  fail=1
fi

# A name that has been wired must leave the allowlist, or the list stops meaning
# anything.
for name in $uncalled_allowed; do
  case " $uncalled " in
    *" $name "*) ;;
    *) echo "FAIL: $name is on the allowlist but is no longer uncalled -- remove it"
       fail=1 ;;
  esac
done

if [ "$fail" -eq 0 ]; then
  echo "OK: methods and fields are reached, no return escapes a Children loop, and"
echo "    the unrenderable-string count is still $strings_missing (a known debt: $strings_baseline),"
echo "    and $orphans_count catalog keys are rendered nowhere (a known debt: $orphans_baseline)"
fi
exit "$fail"
