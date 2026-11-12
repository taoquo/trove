// Package domain holds trove's pure domain logic. It imports no database,
// network, filesystem or UI package, so it stays testable on its own.
package domain

import (
	"strings"
	"unicode/utf8"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/norm"
)

// Tag identity.
//
// This file is a faithful port of the TypeScript original in
// src/lib/tags/canonical.ts. It must stay behaviourally identical, because the
// canonical key decides which tags are "the same tag": it drives de-duplication,
// the blockedTagKeys list, backup-restore cleanup and suggestion ids. Drift
// silently changes which tags exist in a user's library.
//
// Equivalence is enforced by TestCanonicalTagKeyMatchesTypeScript, which replays
// a fixture generated from the TypeScript implementation itself
// (testdata/tagkey/, regenerate with `node
// apps/macos/testdata/tagkey/gentagkeys.mjs`).
//
// See docs/domain-migration-map.md §4.

// neverDepluralize are words ending in "s" that are not plural. Without this
// list the naive plural stripping below would turn "devops" into "devop" and
// "news" into "new". Mirrors NEVER_DEPLURALIZE in the TypeScript.
var neverDepluralize = map[string]struct{}{
	"news": {}, "devops": {}, "ops": {}, "ios": {}, "macos": {}, "aws": {},
	"gcs": {}, "kubernetes": {}, "series": {}, "redis": {}, "nextjs": {},
	"nodejs": {}, "nestjs": {}, "threejs": {}, "js": {}, "ts": {}, "os": {},
	"saas": {}, "paas": {}, "iaas": {}, "cms": {}, "dns": {}, "gis": {},
	"aigc": {},
}

// depluralize mirrors the TypeScript depluralize().
//
// The TypeScript tests `value.length < 4` on a UTF-16 length. Using the Go byte
// length here is equivalent: for a string of pure [a-z] the two counts agree,
// and for anything else the `isLowerASCII` guard returns the value unchanged
// whichever branch is taken.
func depluralize(value string) string {
	if len(value) < 4 || isNeverDepluralize(value) || !isLowerASCIILetters(value) {
		return value
	}

	if strings.HasSuffix(value, "ies") {
		return value[:len(value)-3] + "y"
	}

	// /(?:s|x|z|ch|sh)es$/
	for _, suffix := range []string{"ses", "xes", "zes", "ches", "shes"} {
		if strings.HasSuffix(value, suffix) {
			return value[:len(value)-2]
		}
	}

	// /s$/ but not /(?:ss|us|is|os)$/
	if strings.HasSuffix(value, "s") && !hasKeepSuffix(value) {
		return value[:len(value)-1]
	}

	return value
}

func isNeverDepluralize(value string) bool {
	_, ok := neverDepluralize[value]
	return ok
}

// hasKeepSuffix reports /(?:ss|us|is|os)$/.
func hasKeepSuffix(value string) bool {
	for _, suffix := range []string{"ss", "us", "is", "os"} {
		if strings.HasSuffix(value, suffix) {
			return true
		}
	}
	return false
}

// isLowerASCIILetters reports /^[a-z]+$/.
func isLowerASCIILetters(value string) bool {
	if value == "" {
		return false
	}
	for i := 0; i < len(value); i++ {
		if value[i] < 'a' || value[i] > 'z' {
			return false
		}
	}
	return true
}

// jsIsSpace reports whether r is whitespace to JavaScript.
//
// Go and JavaScript disagree here, so neither strings.TrimSpace nor the regexp
// \s class can be used directly:
//
//   - Go's unicode.IsSpace is true for U+0085 (NEL); JavaScript does not treat
//     NEL as whitespace, so "\u0085AI" must keep it.
//   - Go's unicode.IsSpace is false for U+FEFF (ZWNBSP/BOM); JavaScript does
//     treat it as whitespace, so "\uFEFFAI" must lose it.
//
// The set below is JavaScript's WhiteSpace + LineTerminator, which is what both
// String.prototype.trim() and the /\s/ class use.
func jsIsSpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ',
		'\u00a0', // NBSP
		'\u1680', // OGHAM SPACE MARK
		'\u2028', // LINE SEPARATOR
		'\u2029', // PARAGRAPH SEPARATOR
		'\u202f', // NARROW NO-BREAK SPACE
		'\u205f', // MEDIUM MATHEMATICAL SPACE
		'\u3000', // IDEOGRAPHIC SPACE
		'\ufeff': // ZWNBSP / BOM
		return true
	}
	// The remaining Zs code points: EN QUAD .. HAIR SPACE.
	return r >= '\u2000' && r <= '\u200a'
}

// jsTrim mirrors String.prototype.trim().
func jsTrim(s string) string {
	return strings.TrimFunc(s, jsIsSpace)
}

// collapseJSSpace mirrors s.replace(/\s+/g, " ") followed by trim(), which is
// how the TypeScript normalizer finishes. Collapsing and trimming at once avoids
// a second pass.
func collapseJSSpace(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	pendingSpace := false

	for _, r := range s {
		if jsIsSpace(r) {
			// Suppressed while nothing has been written yet, which trims the
			// leading run.
			if b.Len() > 0 {
				pendingSpace = true
			}
			continue
		}
		if pendingSpace {
			b.WriteByte(' ')
			pendingSpace = false
		}
		b.WriteRune(r)
	}

	return b.String()
}

// jsLower is JavaScript's String.prototype.toLowerCase().
//
// Go's strings.ToLower is a simple per-rune mapping, which diverges from
// JavaScript wherever Unicode has *conditional* casing. Two such cases were
// caught by the fixture from the real TypeScript:
//
//   - Final_Sigma: "ΣΣ" must become "σς" (a capital sigma at the end of a cased
//     word lowercases to the final form); strings.ToLower gives "σσ".
//   - U+0130 LATIN CAPITAL LETTER I WITH DOT ABOVE must become "i\u0307";
//     strings.ToLower drops the combining dot.
//
// x/text's full case mapper implements both under the root locale, so it is used
// instead of a hand-rolled special case. A Caser is safe for concurrent use.
var jsLower = cases.Lower(language.Und)

// jsToLower mirrors String.prototype.toLowerCase().
func jsToLower(s string) string {
	// Fast path: ASCII has no conditional casing, so the simple mapper is
	// equivalent and avoids the Caser's allocation.
	ascii := true
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			ascii = false
			break
		}
	}
	if ascii {
		return strings.ToLower(s)
	}
	return jsLower.String(s)
}

// stripLeadingHashes mirrors s.replace(/^#+/, "").
func stripLeadingHashes(s string) string {
	i := 0
	for i < len(s) && s[i] == '#' {
		i++
	}
	return s[i:]
}

// compactSeparators mirrors s.replace(/[\s\-_/.]+/g, "").
//
// Note this removes JavaScript whitespace as well as the four literal
// separators, so it must use jsIsSpace rather than the regexp \s class.
func compactSeparators(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if jsIsSpace(r) {
			continue
		}
		switch r {
		case '-', '_', '/', '.':
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// NormalizeTagDisplayName mirrors normalizeTagDisplayName: it strips the leading
// hash marks and collapses whitespace while keeping the human readable form
// (original casing and spacing) intact.
func NormalizeTagDisplayName(tagName string) string {
	s := norm.NFKC.String(tagName)
	s = jsTrim(s)
	s = stripLeadingHashes(s)
	s = collapseJSSpace(s)
	return s
}

// CanonicalTagKey mirrors canonicalTagKey: the canonical identity of a tag name,
// so "AI Agent", "ai-agents" and "AI_Agent" collapse into one concept.
func CanonicalTagKey(tagName string) string {
	display := jsToLower(NormalizeTagDisplayName(tagName))
	compact := compactSeparators(display)
	return depluralize(compact)
}

// IsSameTagName mirrors isSameTagName. An empty key never matches, so two names
// that normalize away to nothing are not "the same tag".
func IsSameTagName(left, right string) bool {
	leftKey := CanonicalTagKey(left)
	return leftKey != "" && leftKey == CanonicalTagKey(right)
}

// FindTagByName mirrors findTagByName: alias-aware lookup, so a tag absorbed by
// a merge keeps the old name as an alias and later suggestions of that name
// resolve to the surviving tag. It matches the display name first and only then
// the aliases, and an empty canonical key never matches.
func FindTagByName(tags []Tag, tagName string) (Tag, bool) {
	key := CanonicalTagKey(tagName)
	if key == "" {
		return Tag{}, false
	}

	for _, tag := range tags {
		if CanonicalTagKey(tag.Name) == key {
			return tag, true
		}
	}
	for _, tag := range tags {
		for _, alias := range tag.Aliases {
			if CanonicalTagKey(alias) == key {
				return tag, true
			}
		}
	}

	return Tag{}, false
}
