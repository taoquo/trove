#!/usr/bin/env bash
# Creates the self-signed certificate tools/install-app.sh signs with, once.
#
# Why a certificate at all: the app reads browser cookie databases for the X credential import,
# and macOS guards those directories with TCC. A grant is recorded against the app's designated
# requirement, and an ad-hoc signature's requirement is `cdhash H"..."` -- the hash of the binary --
# so every rebuild would invalidate a Full Disk Access grant and the user would re-grant after each
# build. A certificate moves the requirement to `identifier "com.taoquo.trove" and certificate
# root = H"..."`, which no rebuild changes.
#
# The certificate is self-signed and local: it identifies this machine's builds to this machine's
# TCC and nothing else. It is not a Developer ID and cannot notarise anything.
#
#     bash tools/make-dev-cert.sh
#
# Safe to re-run: it does nothing when the identity already exists.
set -uo pipefail
cd "$(dirname "$0")/.."

IDENTITY="trove Dev"
KEYCHAIN="$HOME/Library/Keychains/login.keychain-db"

if security find-identity -v -p codesigning 2>/dev/null | grep -qF "$IDENTITY"; then
  echo "• the identity '$IDENTITY' already exists"
  exit 0
fi

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

echo "• generating the certificate"
openssl req -x509 -newkey rsa:2048 \
  -keyout "$work/key.pem" -out "$work/cert.pem" \
  -days 3650 -nodes \
  -subj "/CN=$IDENTITY/O=X Master Dev" \
  -addext "keyUsage=digitalSignature" \
  -addext "extendedKeyUsage=codeSigning" >/dev/null 2>&1 || { echo "  openssl failed" >&2; exit 1; }

openssl pkcs12 -export \
  -inkey "$work/key.pem" -in "$work/cert.pem" \
  -out "$work/id.p12" -passout pass:xmdev \
  -name "$IDENTITY" >/dev/null 2>&1 || { echo "  pkcs12 failed" >&2; exit 1; }

echo "• importing into the login keychain"
security import "$work/id.p12" -k "$KEYCHAIN" -P xmdev \
  -T /usr/bin/codesign -T /usr/bin/security >/dev/null 2>&1 || { echo "  import failed" >&2; exit 1; }

# Trusted for code signing only, in the user's own trust settings -- no admin authorisation and no
# system-wide trust change.
echo "• trusting it for code signing"
security add-trusted-cert -r trustRoot -p codeSign -k "$KEYCHAIN" "$work/cert.pem" >/dev/null 2>&1 || {
  echo "  trusting failed; the app will be signed ad-hoc instead" >&2
  exit 1
}

if security find-identity -v -p codesigning | grep -qF "$IDENTITY"; then
  echo "• done: '$IDENTITY' is available"
else
  echo "  the identity was imported but is not usable for code signing" >&2
  exit 1
fi
