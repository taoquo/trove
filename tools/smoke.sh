#!/usr/bin/env bash
# Launches the built app, asserts it is alive, asserts only one of it exists, quits it, and
# asserts it is gone.
#
# The assertion it replaces was run by hand and read:
#
#     open "$APP"; sleep 6; pgrep -f "MacOS/trove" && echo alive
#
# That passes whenever *a* process of that name is alive -- including one left over from an
# earlier run. `open` on a rebuilt .app is answered by LaunchServices, which reuses the running
# instance rather than starting the new binary, so the test can be green while the binary is
# old.
#
# So this script makes the three facts it depends on explicit:
#
#   1. no process of that name is running before the launch, and this is an error rather than
#      something to clean up silently -- a leftover is exactly what made the old assertion lie
#   2. the process it then finds is the one it started, by pid
#   3. the pid it started is gone after the quit
#
# It runs the binary inside the bundle rather than `open`ing the bundle, for the same reason.
#
#     bash tools/smoke.sh
#
# Exits non-zero on any of the four assertions. Leaves no process behind on the way out.

set -u
cd "$(dirname "$0")/.."

APP="build/darwin-arm64/trove.app"
BIN="$APP/Contents/MacOS/trove"
NAME="trove"
# Match the executable, not the word: "trove" also appears in this checkout's own
# path, and pgrep -f would then find the shell running this script.
PATTERN="MacOS/$NAME"
LOG=$(mktemp)

pass=0
fail=0
note() {
  if [ "$1" = ok ]; then pass=$((pass + 1)); else fail=$((fail + 1)); fi
  echo "  $1   $2"
}

if [ ! -x "$BIN" ]; then
  echo "  FAIL no binary at $BIN -- run: go tool mygo build -skip-dmg"
  exit 1
fi

# 1. Nothing of that name is running. A leftover would make every later assertion meaningless,
#    so it is reported rather than killed: whatever it is, this script did not start it.
before=$(pgrep -f "$PATTERN" | tr '\n' ' ')
if [ -n "$before" ]; then
  note FAIL "a process named '$NAME' was already running (pids: $before)"
  echo "       a leftover instance is what lets a launch check pass while testing an old binary"
  echo "       quit it and run again"
  exit 1
fi
note ok "no '$NAME' process before the launch"

# 2. Launch the binary itself, so the pid is this build's.
#
# TROVE_SMOKE asks for the window to be created but not shown. The three things below all still hold:
# the process exists, it is single-instance, and quitting runs the quit path. What stops happening
# is the window appearing in front of whoever is using the machine on every gate run.
TROVE_SMOKE=1 "$BIN" >"$LOG" 2>&1 &
started=$!
sleep 7

if kill -0 "$started" 2>/dev/null; then
  note ok "alive 7s after launch (pid $started)"
else
  note FAIL "exited within 7s; log follows"
  sed 's/^/       /' "$LOG"
fi

# 3. Exactly one. The menu bar and the sync runner are single-instance by design, and a second
#    copy of the binary is the simplest way to see whether that is enforced or assumed.
count=$(pgrep -f "$PATTERN" | wc -l | tr -d ' ')
if [ "$count" = "1" ]; then
  note ok "exactly one process"
else
  note FAIL "$count processes named '$NAME'"
fi

# 4. Quit through the app, not through a signal: the quit path runs app code, and SIGTERM would
#    skip it.
osascript -e "tell application \"$NAME\" to quit" >/dev/null 2>&1
sleep 3

if kill -0 "$started" 2>/dev/null; then
  note FAIL "still running after quit (pid $started)"
  kill "$started" 2>/dev/null
else
  note ok "gone after quit"
fi

if [ -s "$LOG" ]; then
  echo "  ---- output ----"
  sed 's/^/       /' "$LOG"
fi
rm -f "$LOG"

echo
echo "$pass passed, $fail failed"
[ "$fail" -eq 0 ]
