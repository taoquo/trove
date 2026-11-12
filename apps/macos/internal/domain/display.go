package domain

import (
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Display formatting, ported from src/options/utils.ts.
//
// These decide what a count, a long body and a list of tags look like, which is the
// kind of thing that is invisible until it is wrong: a card showing twelve tags instead
// of two does not fail any test, it just stops being readable.

// compactUnit is one step of a locale's compact notation.
type compactUnit struct {
	scale  float64
	suffix string
}

// The units each locale uses, largest last.
//
// English has no thousands step below K and stops at T. Chinese abbreviates at 万 (ten
// thousand), then 亿 (a hundred million), then 万亿 -- it has no thousands step at all,
// which is why 1234 stays "1234" in Chinese and becomes "1.2K" in English.
var (
	compactUnitsEnglish = []compactUnit{
		{1e3, "K"}, {1e6, "M"}, {1e9, "B"}, {1e12, "T"},
	}
	compactUnitsChinese = []compactUnit{
		{1e4, "万"}, {1e8, "亿"}, {1e12, "万亿"},
	}
)

// FormatCompactCount renders a count the way the locale's compact notation does.
//
// The source delegates to Intl.NumberFormat with notation "compact" and at most one
// fraction digit above a thousand. That is not a k-suffix rule: the units differ per
// locale, and the value *rolls over* -- 999999 formats as "1M", not "1000K", because
// the rounded value reaching a thousand promotes it to the next unit. Both behaviours
// are checked against the real formatter by the fixture test.
func FormatCompactCount(value int, locale string) string {
	units := compactUnitsEnglish
	if locale == "zh-CN" {
		units = compactUnitsChinese
	}

	// The largest unit that does not exceed the value; below the smallest, the plain
	// number.
	chosen := -1
	for index, unit := range units {
		if float64(value) >= unit.scale {
			chosen = index
		}
	}
	if chosen < 0 {
		return strconv.Itoa(value)
	}

	scaled := math.Round(float64(value)/units[chosen].scale*10) / 10

	// Rounding up to the *next* unit's ratio means that unit describes the value
	// better, which is what keeps 999999 from reading as 1000K.
	//
	// The threshold is the ratio between the two units, not a fixed thousand: English
	// steps by a thousand, Chinese by ten thousand. Using a fixed thousand would make
	// 12345678 read as "0.1亿" where the formatter says "1234.6万".
	if chosen+1 < len(units) {
		rollover := units[chosen+1].scale / units[chosen].scale
		if scaled >= rollover {
			chosen++
			scaled = math.Round(float64(value)/units[chosen].scale*10) / 10
		}
	}

	return compactNumber(scaled, units[chosen].suffix)
}

// compactNumber renders an already-rounded scaled value with its suffix.
//
// "1.0K" is not what the formatter produces: it emits the shortest form that keeps the
// value, so a whole number loses its decimal.
func compactNumber(scaled float64, suffix string) string {
	text := strconv.FormatFloat(scaled, 'f', 1, 64)
	text = strings.TrimSuffix(text, ".0")
	return text + suffix
}

// TruncateText shortens a long body for a card.
//
// The length is the source's, including its arithmetic: it keeps maxLength-1 characters
// and appends three dots, so the result is two characters past the limit. That is the
// source's behaviour and it is reproduced rather than corrected -- a "fixed" version
// would produce different text from the other product for the same post.
func TruncateText(value string, maxLength int) string {
	if maxLength <= 0 {
		maxLength = 120
	}
	runes := []rune(value)
	if len(runes) <= maxLength {
		return value
	}
	return string(runes[:maxLength-1]) + "..."
}

// DefaultTruncateLength is the source's default.
const DefaultTruncateLength = 120

// MaxCardTagNames is how many tags a list card shows before collapsing the rest.
const MaxCardTagNames = 2

// VisibleCardTagNames collapses a long tag list on a card.
//
// In list mode a card has one line for tags, so past two the rest become a count. Grid
// mode has the width to show them all, which is why the view mode is a parameter.
func VisibleCardTagNames(tagNames []string, viewMode string) []string {
	if viewMode != ViewList || len(tagNames) <= MaxCardTagNames {
		return tagNames
	}

	visible := make([]string, 0, MaxCardTagNames+1)
	visible = append(visible, tagNames[:MaxCardTagNames]...)
	visible = append(visible, "+"+strconv.Itoa(len(tagNames)-MaxCardTagNames))
	return visible
}

// IsVideoMediaType reports whether a media type plays rather than shows.
//
// An animated GIF is treated as video because it is rendered as one, which is what
// makes it get a play affordance rather than being drawn inline.
func IsVideoMediaType(mediaType string) bool {
	return mediaType == "video" || mediaType == "animated_gif"
}

// NotSyncedYet is the timestamp shown when a sync has never run.
func NotSyncedYet(locale string) string {
	if locale == "zh-CN" {
		return "尚未同步"
	}
	return "Not synced yet"
}

// Card density, from the source's getListCopyDensityClass and
// getListCardTemplateClass.
//
// The source expresses these as CSS classes. The classification is what matters -- how
// many lines a card reserves and which of four layouts it uses -- so it is modelled as
// values rather than as class names, which have no meaning without a stylesheet.
const (
	// CardOneLine is a short post with no tags and no media.
	CardOneLine = 1
	// CardTwoLines is anything with media or tags, on the source's reasoning that both
	// claim a row of their own.
	CardTwoLines = 2
	// CardThreeLines is a long untagged text post.
	CardThreeLines = 3
)

// ShortCopyLength is the length below which an untagged post fits one line.
const ShortCopyLength = 48

// LongCopyLength is the length at which an untagged post earns a third line.
const LongCopyLength = 240

// CardLineCount is how many lines a list card reserves for the body.
//
// Media wins over everything: a card showing a picture has no room for three lines of
// text, so it takes two however long the text is.
func CardLineCount(text string, hasMedia bool, hasTags bool) int {
	textLength := len([]rune(strings.TrimSpace(text)))

	if hasMedia {
		return CardTwoLines
	}
	if !hasTags && textLength <= ShortCopyLength {
		return CardOneLine
	}
	if hasTags || textLength >= LongCopyLength {
		return CardTwoLines
	}
	return CardThreeLines
}

// The four list-card layouts the source chooses between.
const (
	CardTemplateMedia   = "options-result-card-list-template-media"
	CardTemplateTagged  = "options-result-card-list-template-tagged"
	CardTemplateCompact = "options-result-card-list-template-compact"
	CardTemplateDefault = "options-result-card-list-template-default"
)

// CardTemplate names the layout a list card uses.
//
// The names are the source's class names, kept verbatim so the two products' card
// layouts can be compared by the same key rather than by description.
func CardTemplate(hasMedia bool, hasTags bool, lineCount int) string {
	if hasMedia {
		return CardTemplateMedia
	}
	if hasTags {
		return CardTemplateTagged
	}
	if lineCount == CardOneLine {
		return CardTemplateCompact
	}
	return CardTemplateDefault
}

// ListCardMinHeight is the floor for a card in the list layout.
//
// The source picks one of four classes from three facts, and each class carries its own
// height (extension.css: the .options-result-card-list-template-* rules):
//
//	media        -> 188   a picture needs the room
//	has tags     -> 176   the tag row needs a line
//	one line     -> 132   the compact case
//	otherwise    -> 154   the default
//
// A card with no floor collapses to its text, so a one-line post and a long one sit at
// different heights and the list stops reading as a list.
func ListCardMinHeight(hasMedia bool, hasTags bool, lines int) float32 {
	switch {
	case hasMedia:
		return 188
	case hasTags:
		return 176
	case lines <= 1:
		return 132
	default:
		return 154
	}
}

// MediaThumbnailURL is the picture a card draws for a bookmark's media.
//
// The source takes media[0] and nothing else (BookmarkCard.tsx:25-28, and the same three
// lines in BookmarkInspector.tsx:60):
//
//	mediaUrl   = bookmark.media[0].url
//	isVideo    = type is "video" or "animated_gif"
//	previewUrl = isVideo ? poster(0) ?? mediaUrl : mediaUrl
//
// So a video has no image of its own and its poster stands in. A video without a poster
// has nothing to draw: the source would put the .mp4 in an <img> and get a broken frame,
// which this build does not reproduce -- there is no picture, so none is drawn.
//
// Scanning every item for the first photo was wrong: for a post whose first attachment is
// a video and whose second is a photo, the source shows the video's poster and this showed
// the later photo.
func MediaThumbnailURL(media []Media) string {
	if len(media) == 0 {
		return ""
	}
	first := media[0]
	if first.Type == MediaVideo || first.Type == MediaAnimatedGif {
		if first.PosterURL != nil && *first.PosterURL != "" {
			return *first.PosterURL
		}
		return ""
	}
	return first.URL
}

// TweetIDFromURL reads the post id out of an X or Twitter status link.
//
// The extension never does this: its content script reads the id off the page it is
// injected into. A standalone manager has no page, so the user's paste is the only source
// and the link is the only thing to read it from.
//
// Both hosts are accepted, as is a mobile subdomain and a link with a handle or without
// one. The query and fragment are ignored, because a shared link carries tracking there.
func TweetIDFromURL(rawURL string) (string, bool) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return "", false
	}
	// A link pasted without a scheme is still a link.
	if !strings.Contains(trimmed, "://") {
		trimmed = "https://" + trimmed
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", false
	}
	host := strings.ToLower(parsed.Hostname())
	host = strings.TrimPrefix(host, "www.")
	host = strings.TrimPrefix(host, "mobile.")
	if host != "x.com" && host != "twitter.com" {
		return "", false
	}

	// The path is either /<handle>/status/<id> or /i/web/status/<id>. Everything after the
	// id is a suffix, not part of it.
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	for i := len(segments) - 1; i >= 1; i-- {
		if segments[i-1] != "status" && segments[i-1] != "statuses" {
			continue
		}
		id := segments[i]
		if id == "" || !isDigits(id) {
			return "", false
		}
		return id, true
	}
	return "", false
}

// isDigits reports whether every rune is an ASCII digit.
func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// MergeDraft folds a hand-entered draft into the record that is already stored.
//
// This is bookmarksStore.ts's upsertBookmarkSnapshot, which a standalone manager needs for
// the same reason the extension did: a draft is read from a link, so it carries only what
// the user typed, while the stored row may have come from the API and be complete. The
// stored value therefore wins wherever it exists, and the draft only fills what is empty.
//
// The reverse rule is not an option: the repository's UpsertMany uses `excluded.X` for
// every column, so writing a partly-empty draft through it would blank the fields the
// draft does not carry. Sync wants that rule (an API snapshot is authoritative);
// this path wants this one.
//
// savedAt is the user's own timeline and is never moved. lastSeenAt is the one field
// the draft does move. A first-time add gets a negative rank so it appears at the top
// until a real sync assigns a position.
func MergeDraft(existing, draft Bookmark, now time.Time) Bookmark {
	merged := existing
	if merged.TweetID == "" {
		merged.TweetID = draft.TweetID
	}
	if merged.TweetURL == "" {
		merged.TweetURL = draft.TweetURL
	}
	if merged.AuthorName == "" {
		merged.AuthorName = draft.AuthorName
	}
	if merged.AuthorHandle == "" {
		merged.AuthorHandle = draft.AuthorHandle
	}
	if merged.AuthorAvatarURL == "" {
		merged.AuthorAvatarURL = draft.AuthorAvatarURL
	}
	if merged.Text == "" {
		merged.Text = draft.Text
	}
	if merged.CreatedAtOnX == "" {
		merged.CreatedAtOnX = draft.CreatedAtOnX
	}
	if merged.SavedAt.IsZero() {
		merged.SavedAt = now
	}
	merged.LastSeenAt = now
	if merged.TimelineRank == nil {
		// Negative, so a hand-added post sorts above the synced timeline. -now is the
		// extension's temporaryTimelineRank.
		rank := -int(now.UnixNano())
		merged.TimelineRank = &rank
	}
	// Media and metrics are not read off a link, so a draft never supplies them; leaving
	// them as the stored row has them is the same as the extension's `existing?.media`.
	merged.UpdatedAt = now
	return merged
}
