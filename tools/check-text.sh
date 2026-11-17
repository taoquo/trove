#!/usr/bin/env bash
# Checks that user-visible text goes through the translator.
#
# A hardcoded string is invisible until someone switches language, which is exactly
# the kind of omission that survives every other test. This is a grep rather than a Go
# test because the property is about the source code, not about behaviour.
#
#   bash tools/check-text.sh
set -euo pipefail
cd "$(dirname "$0")/.."

# Text-bearing calls whose first string argument must be translated.
pattern='ui\.(Text|Button|PrimaryButton|Badge|Tooltip)\(c, "'
hits=$(grep -rnE "$pattern" internal/ui/*.go 2>/dev/null | grep -v '_test.go' || true)

# The forms that are data or a glyph rather than prose.
#
#   \u00d7, ×   the close control's multiplication sign (settings header)
#   "@"+       the handle prefix, assembled around a name
#   “          .options-detail-summary-quote's decorative glyph, 3.8rem at 11% accent
#              It is not a word in either language.
allowed='\\u00d7|"@"\+|"“"'

status=0
if [ -n "$hits" ]; then
  remaining=$(printf '%s\n' "$hits" | grep -vE "$allowed" || true)
  if [ -n "$remaining" ]; then
    echo "Hardcoded user-visible text found. Route it through i18n (s.t()/s.t().Tf()):"
    echo "$remaining"
    status=1
  fi
fi

# Status-line sentences are user-visible too, and the calls that show them are usually
# given an expression rather than the direct literal the pattern above looks for. The two
# shortcut forms are caught here: a bare English string, and a fmt.Sprintf whose format
# string has words in it. A message assembled elsewhere -- s.t().T(key), err.Error() -- is
# not this checker's business.
status_pattern='(ShowStatus|ShowToast|ShowBanner)\(.*("[^"]*[A-Za-z][^"]*"|fmt\.Sprintf\("[^"]*[A-Za-z])'
status_hits=$(grep -rnE "$status_pattern" internal/ui/*.go 2>/dev/null | grep -v '_test.go' || true)
if [ -n "$status_hits" ]; then
  echo "Hardcoded status text found. Status and toast sentences go through i18n:"
  echo "$status_hits"
  status=1
fi

if [ "$status" -ne 0 ]; then
  exit 1
fi

echo "OK: every user-visible string goes through the translator"
