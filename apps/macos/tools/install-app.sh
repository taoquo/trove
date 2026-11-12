#!/usr/bin/env bash
# Builds the app and installs it over the copy in /Applications, so the next run is a click on the
# icon rather than a terminal launch.
#
#     bash tools/install-app.sh          # build, install, leave it closed
#     bash tools/install-app.sh --run    # build, install, open it
#
# Why this exists at all: a TCC grant (Full Disk Access, which the browser-cookie import needs) is
# attached to a binary at a path. Running the built binary out of build/darwin-arm64/ and running
# the installed bundle are two different subjects as far as macOS is concerned, so a grant made to
# one does nothing for the other. Installing to one stable path is what makes a single grant cover
# every rebuild.
set -uo pipefail
cd "$(dirname "$0")/.."

DEST="/Applications"
NAME="X Bookmark Manager"
BUNDLE_ID="com.taoquo.xmaster"
BUILT="build/darwin-arm64/$NAME.app"
INSTALLED="$DEST/$NAME.app"

run_after=0
for arg in "$@"; do
  case "$arg" in
    --run) run_after=1 ;;
    -h|--help) sed -n '2,12p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "unknown argument: $arg" >&2; exit 2 ;;
  esac
done

# Replacing a bundle that is running leaves the old process alive on deleted files, and the next
# launch can come up as the old build. Quit it first, through the app so its own quit path runs.
if pgrep -f "$NAME" >/dev/null 2>&1; then
  echo "• quitting the running instance"
  osascript -e "tell application \"$NAME\" to quit" >/dev/null 2>&1 || true
  for _ in $(seq 1 20); do
    pgrep -f "$NAME" >/dev/null 2>&1 || break
    sleep 0.25
  done
  if pgrep -f "$NAME" >/dev/null 2>&1; then
    echo "  it did not quit; stopping it" >&2
    pkill -f "$NAME" >/dev/null 2>&1 || true
    sleep 1
  fi
fi

echo "• building"
# The toolchain lives in the repository, not on PATH.
# shellcheck disable=SC1091
source .toolchain/env.sh 2>/dev/null || source ../../.toolchain/env.sh
if ! go tool mygo build -skip-dmg >/tmp/xbm-build.log 2>&1; then
  echo "  build failed:" >&2
  sed 's/^/    /' /tmp/xbm-build.log >&2
  exit 1
fi
if [ ! -d "$BUILT" ]; then
  echo "  no bundle at $BUILT" >&2
  exit 1
fi

built_id=$(/usr/libexec/PlistBuddy -c "Print :CFBundleIdentifier" "$BUILT/Contents/Info.plist" 2>/dev/null)
if [ "$built_id" != "$BUNDLE_ID" ]; then
  echo "  the bundle's identifier is '$built_id', expected '$BUNDLE_ID'" >&2
  exit 1
fi

echo "• installing to $INSTALLED"
rm -rf "$INSTALLED"
if ! cp -R "$BUILT" "$INSTALLED"; then
  echo "  copy failed" >&2
  exit 1
fi

# A quarantined bundle is refused by Gatekeeper on first launch; this one was built here, so the
# attribute is removed rather than asking the user to click through a warning about a file they
# just compiled.
# Read back what was copied, so a stale bundle cannot pass as the new one. This has to happen
# before signing: signing rewrites the Mach-O, so after it the bytes legitimately differ.
installed_id=$(/usr/libexec/PlistBuddy -c "Print :CFBundleIdentifier" "$INSTALLED/Contents/Info.plist" 2>/dev/null)
built_hash=$(shasum -a 256 "$BUILT/Contents/MacOS/$NAME" | cut -d' ' -f1)
installed_hash=$(shasum -a 256 "$INSTALLED/Contents/MacOS/$NAME" | cut -d' ' -f1)
if [ "$installed_id" != "$BUNDLE_ID" ] || [ "$built_hash" != "$installed_hash" ]; then
  echo "  the installed copy does not match the build" >&2
  exit 1
fi

xattr -dr com.apple.quarantine "$INSTALLED" 2>/dev/null || true

# Sign with the stable dev certificate when it exists. This is not cosmetic: mygo signs ad-hoc, and
# an ad-hoc designated requirement is `cdhash H"..."` -- the binary's hash -- so a Full Disk Access
# grant (which the browser-cookie import needs) would be invalidated by every install. Signed with
# the certificate, the requirement is `identifier ... and certificate root = H"..."`, which is the
# same for every rebuild, so one grant keeps working.
IDENTITY="X Bookmark Manager Dev"
if security find-identity -v -p codesigning 2>/dev/null | grep -qF "$IDENTITY"; then
  echo "• signing as '$IDENTITY'"
  if ! codesign --force --deep --sign "$IDENTITY" "$INSTALLED" >/dev/null 2>&1; then
    echo "  signing failed; the installed copy stays ad-hoc and a TCC grant will not survive the" >&2
    echo "  next install" >&2
  fi
else
  echo "• no '$IDENTITY' certificate: signed ad-hoc (run tools/make-dev-cert.sh to make one)"
  echo "  a Full Disk Access grant will have to be re-made after each install"
fi

# The signature is what TCC records. An ad-hoc signature's hash changes with every build, so
# whether a Full Disk Access grant survives is a property of macOS rather than of this script --
# it is reported here rather than assumed, because the answer decides whether the grant has to be
# repeated after each install.
if ! codesign --verify "$INSTALLED" >/dev/null 2>&1; then
  echo "  the installed bundle's signature does not verify" >&2
  exit 1
fi

# The authority, not the signature's size: `Signature size=1701` also starts with "Signature", and
# it says nothing about who signed. `Authority` is present only for a real certificate.
sig=$(codesign -dv --verbose=2 "$INSTALLED" 2>&1 | sed -n 's/^Authority=//p' | head -1)
# The requirement line may or may not carry a leading "# ", depending on the codesign version.
requirement=$(codesign -d -r- "$INSTALLED" 2>&1 | sed -n 's/^#\{0,1\} *designated => //p')
echo "  installed $BUNDLE_ID, signed by: ${sig:-ad-hoc}"
echo "  designated: ${requirement:-unknown}"

echo "• done: $INSTALLED"
if [ "$run_after" = "1" ]; then
  echo "• opening"
  open "$INSTALLED"
fi
