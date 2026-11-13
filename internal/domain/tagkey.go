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
// This file implements the canonical tag key. It must stay behaviourally
// identical, because the canonical key decides which tags are "the same tag": it
// drives de-duplication, the blockedTagKeys list, backup-restore cleanup and
// suggestion ids. Drift silently changes which tags exist in a user's library.
//
// Equivalence is enforced by TestCanonicalTagKeyMatchesFixture, which replays the
// frozen fixture under testdata/tagkey/.

// neverDepluralize are words ending in "s" that are not plural. Without this
// list the naive plural stripping below would turn "devops" into "devop" and
// "news" into "new". Mirrors NEVER_DEPLURALIZE in the fixture.
var neverDepluralize = map[string]struct{}{
	"news": {}, "devops": {}, "ops": {}, "ios": {}, "macos": {}, "aws": {},
	"gcs": {}, "kubernetes": {}, "series": {}, "redis": {}, "nextjs": {},
	"nodejs": {}, "nestjs": {}, "threejs": {}, "js": {}, "ts": {}, "os": {},
	"saas": {}, "paas": {}, "iaas": {}, "cms": {}, "dns": {}, "gis": {},
	"aigc": {},
}

// depluralize mirrors the fixture depluralize().
//
// The fixture applies `value.length < 4` to a UTF-16 length. Using the Go byte
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

// isTagSpace reports whether r is whitespace for tag normalization.
//
// The set is deliberately not Go's unicode.IsSpace, which differs in two places
// pinned by the frozen fixture:
//
//   - Go's unicode.IsSpace is true for U+0085 (NEL); the fixture keeps it, so
//     "\u0085AI" must not be trimmed.
//   - Go's unicode.IsSpace is false for U+FEFF (ZWNBSP/BOM); the fixture treats
//     it as whitespace, so "\uFEFFAI" must lose it.
//
// The set below is the WhiteSpace + LineTerminator set used by the reference
// normalizer.
func isTagSpace(r rune) bool {
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

// trimTagSpace trims the whitespace set used by tag normalization.
func trimTagSpace(s string) string {
	return strings.TrimFunc(s, isTagSpace)
}

// collapseTagSpace collapses runs of tag whitespace to one space and trims the
// ends in a single pass.
func collapseTagSpace(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	pendingSpace := false

	for _, r := range s {
		if isTagSpace(r) {
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

// tagLowerCaser is the full Unicode lower-case mapper used for tag text.
//
// Go's strings.ToLower is a simple per-rune mapping, which diverges wherever
// Unicode has *conditional* casing. Two such cases are pinned by the fixture:
//
//   - Final_Sigma: "ΣΣ" must become "σς" (a capital sigma at the end of a cased
//     word lowercases to the final form); strings.ToLower gives "σσ".
//   - U+0130 LATIN CAPITAL LETTER I WITH DOT ABOVE must become "i\u0307";
//     strings.ToLower drops the combining dot.
//
// x/text's full case mapper implements both under the root locale, so it is used
// instead of a hand-rolled special case. A Caser is safe for concurrent use.
var tagLowerCaser = cases.Lower(language.Und)

// lowerTagText lower-cases tag text, including conditional casing.
func lowerTagText(s string) string {
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
	return tagLowerCaser.String(s)
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
// Note this removes tag whitespace as well as the four literal separators, so
// it must use isTagSpace rather than the regexp \s class.
func compactSeparators(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if isTagSpace(r) {
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
	s = trimTagSpace(s)
	s = stripLeadingHashes(s)
	s = collapseTagSpace(s)
	return s
}

// CanonicalTagKey mirrors canonicalTagKey: the canonical identity of a tag name,
// so "AI Agent", "ai-agents" and "AI_Agent" collapse into one concept.
func CanonicalTagKey(tagName string) string {
	display := lowerTagText(NormalizeTagDisplayName(tagName))
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
