package domain

import "time"

// SuggestionSource records which mechanism proposed a tag.
type SuggestionSource = string

const (
	SourceHashtag SuggestionSource = "hashtag"
	SourceAuthor  SuggestionSource = "author"
	SourceKeyword SuggestionSource = "keyword"
	SourceLLM     SuggestionSource = "llm"
)

// SuggestionStatus is a suggestion's lifecycle state.
type SuggestionStatus = string

const (
	StatusPending  SuggestionStatus = "pending"
	StatusAccepted SuggestionStatus = "accepted"
	StatusRejected SuggestionStatus = "rejected"
)

// TagSuggestion is a proposed tag for a bookmark, awaiting the user's decision.
//
// Accepted and rejected rows are kept, not deleted: they are the negative cache
// that stops the user being re-offered a tag they already declined.
type TagSuggestion struct {
	// ID is BookmarkID + "::" + canonical key of TagName, so creating the same
	// suggestion twice is a no-op by primary key rather than a duplicate.
	ID         string
	BookmarkID string
	TagName    string
	Source     SuggestionSource
	Confidence float64
	Status     SuggestionStatus
	CreatedAt  time.Time
	// ResolvedAt is set when the status leaves pending.
	ResolvedAt *time.Time
}

// SuggestionID builds the composite primary key.
func SuggestionID(bookmarkID, tagName string) string {
	return bookmarkID + "::" + CanonicalTagKey(tagName)
}

// IsPending reports whether the suggestion still needs a decision.
func (s TagSuggestion) IsPending() bool { return s.Status == StatusPending }

// IsResolved reports whether the suggestion has been accepted or rejected. These
// rows must survive deletion of their bookmark, because they encode the user's
// answer.
func (s TagSuggestion) IsResolved() bool {
	return s.Status == StatusAccepted || s.Status == StatusRejected
}

// AllSuggestionSources is every source a suggestion may have, matching the
// schema's CHECK constraint.
var AllSuggestionSources = []SuggestionSource{SourceHashtag, SourceAuthor, SourceKeyword, SourceLLM}

// AllSuggestionStatuses is every status, matching the schema's CHECK constraint.
var AllSuggestionStatuses = []SuggestionStatus{StatusPending, StatusAccepted, StatusRejected}

// ValidSuggestionSource reports whether a stored source is known.
//
// An imported row carrying an unknown value must be refused rather than guessed
// at, because the stored source decides whether a later prune may remove the tag.
func ValidSuggestionSource(source string) bool {
	for _, candidate := range AllSuggestionSources {
		if source == candidate {
			return true
		}
	}
	return false
}

// ValidSuggestionStatus reports whether a stored status is known.
//
// Status is what stops a resolved suggestion being offered again, so an unknown
// value cannot be defaulted to pending.
func ValidSuggestionStatus(status string) bool {
	for _, candidate := range AllSuggestionStatuses {
		if status == candidate {
			return true
		}
	}
	return false
}
