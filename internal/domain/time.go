package domain

import (
	"strings"
	"time"
)

// TimestampLayout is the ISO 8601 storage form: always UTC, always "Z", always
// exactly three fractional digits.
//
// Every stored timestamp uses this exact shape, which keeps new rows byte-identical
// to existing ones and keeps TEXT comparison in SQLite equivalent to chronological
// order.
const TimestampLayout = "2006-01-02T15:04:05.000Z"

// FormatTimestamp renders t in the storage layout.
func FormatTimestamp(t time.Time) string {
	return t.UTC().Format(TimestampLayout)
}

// ParseTimestamp reads a stored timestamp.
//
// Existing data may carry fewer fractional digits than TimestampLayout, so this
// accepts the general RFC 3339 form rather than the fixed layout.
func ParseTimestamp(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339Nano, s)
}

// MustParseTimestamp is ParseTimestamp for values already known to be valid.
// It returns the zero time for an empty string, which is how "unset" is spelled.
func MustParseTimestamp(s string) time.Time {
	t, err := ParseTimestamp(s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// XTimestampLayout is the classic Twitter API rendering of a post's creation
// time, which is what the payload's `created_at` field contains.
//
// It is stored verbatim because re-rendering it would lose information, and it is
// NOT sortable as text: sorting these strings alphabetically orders them by
// weekday name first, which is why the storage layer keeps a parsed copy.
const XTimestampLayout = "Mon Jan 02 15:04:05 -0700 2006"

// ParseXTimestamp reads a post's creation time from either rendering seen in
// practice.
//
// X's own layout comes first because that is what the API returns. RFC 3339 is
// accepted as well because a restored backup or a future payload may already be
// ISO, and refusing it would drop the time entirely.
func ParseXTimestamp(s string) (time.Time, bool) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return time.Time{}, false
	}

	for _, layout := range []string{XTimestampLayout, time.RFC3339Nano, TimestampLayout} {
		if parsed, err := time.Parse(layout, trimmed); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

// FormatXTimestampOrEmpty renders a parsed creation time for storage, or an empty
// string when it could not be parsed.
//
// Empty is the "unknown" spelling and sorts last in a descending order.
func FormatXTimestampOrEmpty(s string) string {
	parsed, ok := ParseXTimestamp(s)
	if !ok {
		return ""
	}
	return FormatTimestamp(parsed)
}

// FormatSavedAt renders a stored timestamp for display.
//
// This is not FormatTimestamp: that one is the storage form, an RFC 3339 string, and it is
// what a card was displaying until this was added -- the raw "2026-10-15T00:00:00.000Z" a
// user is not meant to read.
//
// The rules are:
//   - an empty value is the "not synced yet" sentence, which is not a missing value
//   - a value that will not parse is returned unchanged, because replacing something the
//     user can inspect with a placeholder loses information
//   - otherwise it renders a short month, a numeric day, a numeric hour and a
//     two-digit minute
//
// It accepts X's own rendering as well as RFC 3339, because createdAtOnX is stored
// verbatim -- "Mon Nov 02 06:40:09 +0000 2026" -- and the inspector shows it through this
// function. Parsing RFC 3339 alone silently took the "will not parse" branch, so the
// published node printed the wire format while the saved node beside it printed a date.
func FormatSavedAt(value string, locale string) string {
	if value == "" {
		if locale == LocaleChinese {
			return "尚未同步"
		}
		return "Not synced yet"
	}
	parsed, ok := ParseXTimestamp(value)
	if !ok {
		return value
	}
	if parsed.IsZero() {
		// A malformed stored timestamp becomes MustParseTimestamp's zero time, and this
		// used to render it as "Jan 1, 0001": a date the user never saved reads as data.
		// The em dash says the value is absent without hiding that something was stored.
		return "\u2014"
	}
	if locale == LocaleChinese {
		return parsed.Local().Format("1月2日 15:04")
	}
	return parsed.Local().Format("Jan 2, 3:04 PM")
}

// LocaleChinese is the one non-default locale, named here so this file does not depend on
// the i18n package.
const LocaleChinese = "zh-CN"
