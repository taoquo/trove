#!/usr/bin/env bash
# Produces the native half of the parity rig: a box dump from the same fixture
# render.sh draws the source from, and a picture at the same size.
#
#   bash tools/parity/render-native.sh                 # workspace detail list
#   bash tools/parity/render-native.sh list dark       # just these states
#
# Requires the pinned toolchain (`source .toolchain/env.sh` happens inside, from
# bash) and the built app bundle (`go tool mygo build -skip-dmg`). Output lands in
# docs/parity/ next to the source's: native-<state>-<width>.json and .png.
#
# Why a script: the native dump needs four things to agree -- a freshly seeded
# database, the media server the fixture's pictures come from, the same width as
# the source render, and the same locale -- and getting one wrong produces a
# picture that looks plausible and compares against nothing. The fixture's
# preferences already carry zh-CN, but ApplyCaptureState resets the locale for
# screenshots, so XBM_CAPTURE_LOCALE is what actually keeps the footer and the
# strings in the language the source render used.
set -euo pipefail
cd "$(dirname "$0")/../.."

REPO="$PWD"
APPS="apps/macos"
WIDTH=${XBM_PARITY_WIDTH:-1297}
HEIGHT=${XBM_PARITY_HEIGHT:-808}
DATA_DIR=${XBM_PARITY_DATA_DIR:-/tmp/xbm-parity-native}
MEDIA_PORT=${XBM_PARITY_MEDIA_PORT:-8799}
BIN="$APPS/build/darwin-arm64/X Bookmark Manager.app/Contents/MacOS/X Bookmark Manager"

STATES=("$@")
if [ ${#STATES[@]} -eq 0 ]; then
  STATES=(workspace detail list)
fi

if [ ! -x "$BIN" ]; then
  echo "FAIL no app binary at $APPS/$BIN"
  echo "     build it first: bash -c 'source .toolchain/env.sh && cd $APPS && go tool mygo build -skip-dmg'"
  exit 1
fi

# LaunchServices would hand a capture to an already running copy of the app, so the
# picture would be of the old build. Refuse rather than kill, as tools/smoke.sh does.
if pgrep -f "X Bookmark Manager" >/dev/null 2>&1; then
  echo "FAIL an instance called 'X Bookmark Manager' is already running"
  echo "     quit it: this would capture that build, not the one just built"
  exit 1
fi

# The fixture's pictures are URLs under the repository root. render-source.mjs
# serves the same tree on the same port; reuse a server that is already up.
started_media=0
if ! nc -z 127.0.0.1 "$MEDIA_PORT" >/dev/null 2>&1; then
  python3 -m http.server "$MEDIA_PORT" --bind 127.0.0.1 --directory "$REPO" >/tmp/xbm-parity-media.log 2>&1 &
  started_media=$!
  trap 'kill "$started_media" 2>/dev/null || true' EXIT
  for _ in $(seq 1 40); do
    nc -z 127.0.0.1 "$MEDIA_PORT" >/dev/null 2>&1 && break
    sleep 0.1
  done
fi

# A fresh database every run: the dump exists to measure this build against the
# fixture, and a reused directory could carry another build's rows.
rm -rf "$DATA_DIR"
mkdir -p "$DATA_DIR"

echo "seeding $DATA_DIR"
bash -c "source .toolchain/env.sh && cd '$APPS' && go run ./cmd/seed -data-dir '$DATA_DIR' -warm-media" >/dev/null

for state in "${STATES[@]}"; do
  json="$REPO/docs/parity/native-$state-$WIDTH.json"
  png="$REPO/docs/parity/native-$state-$WIDTH.png"
  rm -f "$json" "$png"

  # The dark parity rendition is the one state whose appearance is the subject. Pinning it to
  # light, as every other state needs (the source's other renditions are light), would capture
  # the "dark" state in light and compare it against a dark picture.
  appearance=light
  [ "$state" = "dark" ] && appearance=dark

  echo "dumping $state at ${WIDTH}x${HEIGHT}"
  bash -c "source .toolchain/env.sh && cd '$APPS' && \
    XBM_PARITY_DB='$DATA_DIR' XBM_PARITY_OUT='$json' XBM_PARITY_WIDTH='$WIDTH' \
    XBM_CAPTURE_STATE='$state' XBM_CAPTURE_LOCALE='zh-CN' XBM_CAPTURE_APPEARANCE='$appearance' \
    go test -run TestDumpParityGeometry . -count=1" >/dev/null

  echo "capturing $state"
  XBM_DATA_DIR="$DATA_DIR" XBM_HEADLESS=1 XBM_CAPTURE="$png" \
    XBM_CAPTURE_STATE="$state" XBM_CAPTURE_LOCALE=zh-CN XBM_CAPTURE_APPEARANCE="$appearance" \
    XBM_CAPTURE_WIDTH="$WIDTH" XBM_CAPTURE_HEIGHT="$HEIGHT" XBM_CAPTURE_DELAY=4s \
    "$BIN" >/tmp/xbm-parity-capture.log 2>&1

  if [ ! -s "$json" ] || [ ! -s "$png" ]; then
    echo "FAIL $state produced no artifacts"
    exit 1
  fi
  echo "  ok   docs/parity/native-$state-$WIDTH.{json,png}"
done

echo "compare: /usr/bin/python3 tools/parity/compare.py --state <state> --width $WIDTH"
