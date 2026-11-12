#!/usr/bin/env bash
# Checks that user-visible text goes through the translator.
#
# A hardcoded string is invisible until someone switches language, which is exactly
# the kind of omission that survives every other test. This is a grep rather than a Go
# test because the property is about the source, not about behaviour.
#
#   bash apps/macos/tools/check-text.sh
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
#              (extension.css:3313-3328). It is not a word in either language.
allowed='\\u00d7|"@"\+|"“"'

if [ -n "$hits" ]; then
  remaining=$(printf '%s\n' "$hits" | grep -vE "$allowed" || true)
  if [ -n "$remaining" ]; then
    echo "Hardcoded user-visible text found. Route it through i18n (s.t()/s.t().Tf()):"
    echo "$remaining"
    exit 1
  fi
fi

echo "OK: every user-visible string goes through the translator"
