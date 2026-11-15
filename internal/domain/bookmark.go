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
// PosterURL is a pointer to distinguish "this is an image, so there is no poster"
// (the key is absent) from "this is a video whose poster URL is empty" (the key is
// present and empty). Collapsing both to "" makes the UI render images as videos
// with a missing poster.
type Media struct {
	// The JSON tags are the persisted field names, because this value is
	// serialized into a column that a backup round-trips.
	Type      MediaType `json:"type"`
	URL       string    `json:"url"`
	PosterURL *string   `json:"posterUrl,omitempty"`
	AltText   string    `json:"altText"`
}

// Metrics is a post's engagement counts.
//
// Every field is a pointer so "the API did not return this" stays distinct from
// "the API returned zero". Coercing a missing count to zero would make every post
// with missing metrics look like it had zero likes.
//
// A nil *Metrics means no metric was returned at all.
type Metrics struct {
	// The JSON tags are the persisted field names. omitempty keeps "the API did not
	// return this" distinct from "it returned zero".
	Likes    *int `json:"likes,omitempty"`
	Retweets *int `json:"retweets,omitempty"`
	Replies  *int `json:"replies,omitempty"`
}

// Bookmark is one saved post.
type Bookmark struct {
	// TweetID is the identity of the record, always a string: X ids exceed the
	// range a float64 represents exactly, so they are never parsed as numbers.
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

	// ArchivedAt is when the reader put this bookmark away, and the zero time means it
	// is in play.
	//
	// Archiving is not deleting: the row keeps its text, its tags and its place in the
	// backup. It leaves the lists -- every view hides archived rows unless the filter
	// asks for them -- which is what lets a library stay complete while the list a
	// reader works from stays short.
	ArchivedAt time.Time

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

// Archived reports whether the reader has put this bookmark away.
func (b Bookmark) Archived() bool { return !b.ArchivedAt.IsZero() }

// LongformThreshold is the character count above which a post is treated as
// longform: trimmed length > 280.
const LongformThreshold = 280

// IsLongform mirrors bookmarkIsLongform from the fixture.
//
// Two details matter for fidelity, and neither is the obvious Go spelling:
//
//   - trimming uses the tag whitespace set, not strings.TrimSpace (see isTagSpace).
//   - length is counted in UTF-16 code units, not runes. the fixture reports 2
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
	return utf16Length(trimTagSpace(text)) > LongformThreshold
}

// utf16Length returns the number of UTF-16 code units in s: one per BMP
// character, two per astral character. X's length rule counts the same way.
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
