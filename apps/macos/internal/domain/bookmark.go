package domain

import "time"

// MediaType is the kind of attachment on a post.
type MediaType = string

const (
	MediaPhoto       MediaType = "photo"
	MediaVideo       MediaType = "video"
	MediaAnimatedGif MediaType = "animated_gif"
)

// SourceKind records which X payload shape a bookmark came from.
type SourceKind = string

const (
	SourceBookmark  SourceKind = "x-bookmark"
	SourceNoteTweet SourceKind = "x-note-tweet"
)

// Media is one attachment.
//
// PosterURL is a pointer because the old extension distinguishes "this is an
// image, so there is no poster" (the key is absent) from "this is a video whose
// poster URL is empty" (the key is present and empty). Collapsing both to ""
// makes the UI render images as videos with a missing poster.
//
// See docs/domain-migration-map.md §3.2.
type Media struct {
	// The JSON tags match the extension's field names, because this value is
	// serialized into a column that a backup round-trips.
	Type      MediaType `json:"type"`
	URL       string    `json:"url"`
	PosterURL *string   `json:"posterUrl,omitempty"`
	AltText   string    `json:"altText"`
}

// Metrics is a post's engagement counts.
//
// Every field is a pointer so "the API did not return this" stays distinct from
// "the API returned zero". The old parser wrote `legacy?.favorite_count ?? 0`,
// which made every post with missing metrics look like it had zero likes — the
// defect called out in docs/baseline-audit.md §2.5.
//
// A nil *Metrics means no metric was returned at all.
type Metrics struct {
	// The JSON tags match the extension's field names. omitempty keeps "the API
	// did not return this" distinct from "it returned zero", which is the fix for
	// the defect in docs/baseline-audit.md §2.5.
	Likes    *int `json:"likes,omitempty"`
	Retweets *int `json:"retweets,omitempty"`
	Replies  *int `json:"replies,omitempty"`
}

// Bookmark is one saved post.
//
// See docs/sqlite-schema.md §4.1.
type Bookmark struct {
	// TweetID is the identity of the record, always a string: X ids exceed the
	// range JavaScript integers represent exactly, so they are never parsed as
	// numbers.
	TweetID string

	TweetURL        string
	AuthorName      string
	AuthorHandle    string
	AuthorAvatarURL string
	Text            string

	// CreatedAtOnX is X's own rendering of the post time, kept verbatim because
	// it is not ISO 8601 and re-parsing it would lose information.
	CreatedAtOnX string

	// SavedAt is when this machine first stored the bookmark. A later sync must
	// never overwrite it: it is the user's own timeline.
	SavedAt time.Time
	// LastSeenAt is the most recent sync that saw the post still bookmarked.
	LastSeenAt time.Time

	// TimelineRank is the post's position in the bookmark timeline. Nil means
	// unknown; zero is a real rank (the first post).
	TimelineRank *int

	Hashtags []string
	// SourceKind is SourceBookmark or SourceNoteTweet.
	SourceKind SourceKind

	Media []Media
	// Metrics is nil when no engagement counts were returned.
	Metrics *Metrics

	UpdatedAt time.Time
}

// HasMedia reports whether the post has any attachment.
func (b Bookmark) HasMedia() bool { return len(b.Media) > 0 }

// LongformThreshold is the character count above which the old extension treats
// a post as longform: text.trim().length > 280.
//
// See docs/baseline-audit.md §2.3.
const LongformThreshold = 280

// IsLongform mirrors bookmarkIsLongform from the TypeScript.
//
// Two details matter for fidelity, and neither is the obvious Go spelling:
//
//   - trim() is JavaScript's, which is not strings.TrimSpace (see jsIsSpace).
//   - length is counted in UTF-16 code units, not runes. JavaScript reports 2
//     for an astral character such as an emoji, so a post made mostly of emoji
//     crosses the 280 threshold at half the apparent character count.
func (b Bookmark) IsLongform() bool {
	return IsLongformText(b.Text)
}

// IsLongformText reports whether post text counts as longform.
//
// It is exported separately because the storage layer denormalizes the answer
// into a column, and two implementations of this rule is exactly how a filter
// silently changes meaning: SQLite's length() counts code points, while this
// counts UTF-16 code units, so the two disagree on any post containing an
// astral character.
func IsLongformText(text string) bool {
	return utf16Length(jsTrim(text)) > LongformThreshold
}

// utf16Length returns the length JavaScript would report for s, i.e. the number
// of UTF-16 code units: one per BMP character, two per astral character.
func utf16Length(s string) int {
	n := 0
	for _, r := range s {
		if r > 0xFFFF {
			n += 2
			continue
		}
		n++
	}
	return n
}
