package domain

import (
	"errors"
	"strings"
	"time"
)

// MultiValueMatchMode decides how a multi-select filter combines.
//
// Both modes exist with a different default per filter: tags default to "all"
// and authors to "any". Collapsing them to one behaviour would change what a
// user's existing selection returns.
type MultiValueMatchMode = string

const (
	MatchAny MultiValueMatchMode = "any"
	MatchAll MultiValueMatchMode = "all"
)

// ValidMatchMode reports whether mode is known.
func ValidMatchMode(mode string) bool { return mode == MatchAny || mode == MatchAll }

// SavedTimeRange limits results to bookmarks saved recently.
type SavedTimeRange = string

const (
	RangeAll SavedTimeRange = "all"
	Range7d  SavedTimeRange = "7d"
	Range30d SavedTimeRange = "30d"
	Range90d SavedTimeRange = "90d"
)

// AllSavedTimeRanges is the whitelist of supported ranges.
var AllSavedTimeRanges = []SavedTimeRange{RangeAll, Range7d, Range30d, Range90d}

// ValidSavedTimeRange reports whether r is known.
func ValidSavedTimeRange(r string) bool {
	for _, candidate := range AllSavedTimeRanges {
		if r == candidate {
			return true
		}
	}
	return false
}

// SavedTimeRangeDays returns the window in days, or 0 for "all".
//
// It is a function rather than a method because SavedTimeRange is an alias for
// string, and Go does not allow methods on a non-local type.
func SavedTimeRangeDays(r SavedTimeRange) int {
	switch r {
	case Range7d:
		return 7
	case Range30d:
		return 30
	case Range90d:
		return 90
	default:
		return 0
	}
}

// LibraryFilter is the filter set a library view applies.
//
// The semantics are fixed rather than incidental because a filter that selects
// a different set makes a user's model of their own library wrong.
type LibraryFilter struct {
	// Query matches text, author name, author handle and the post URL,
	// case-insensitively, as a substring. Those four fields are searched;
	// omitting the URL would stop a pasted link from finding the bookmark it
	// points at.
	Query string

	// TagIDs are combined according to TagMatchMode.
	TagIDs []string
	// TagMatchMode defaults to MatchAll.
	TagMatchMode MultiValueMatchMode

	// AuthorHandles are matched against both the handle and the display name,
	// because both are put in one set and membership is tested against it. A user
	// filtering by a display name expects it to work.
	AuthorHandles []string
	// AuthorMatchMode defaults to MatchAny.
	AuthorMatchMode MultiValueMatchMode

	// ListID limits the results to one list. Empty means every list.
	//
	// There is deliberately no "in no list" mode: every bookmark is kept in a
	// list, defaulting to the Inbox, so a third state does not exist to filter on.
	ListID string

	// OnlyWithPendingSuggestions keeps bookmarks that have at least one suggestion
	// waiting for an answer.
	//
	// The sidebar has a row for this, beside the tag list: it is how a user
	// finds the bookmarks a review pass wants them to look at.
	OnlyWithPendingSuggestions bool

	// OnlyUnclassified keeps bookmarks that carry no tag at all, whatever list
	// the bookmark is in.
	OnlyUnclassified bool

	OnlyWithMedia bool
	OnlyLongform  bool

	// TimeRange limits by saved_at and defaults to RangeAll.
	TimeRange SavedTimeRange
}

// Normalize trims, lowercases handles and the query, drops blanks, removes
// duplicates and fills the mode defaults.
func (f LibraryFilter) Normalize() LibraryFilter {
	normalized := f
	normalized.Query = strings.TrimSpace(f.Query)
	normalized.ListID = strings.TrimSpace(f.ListID)
	normalized.TagIDs = dedupeStrings(f.TagIDs, false)
	normalized.AuthorHandles = dedupeStrings(f.AuthorHandles, true)

	if normalized.TagMatchMode == "" {
		normalized.TagMatchMode = MatchAll
	}
	if normalized.AuthorMatchMode == "" {
		normalized.AuthorMatchMode = MatchAny
	}
	if normalized.TimeRange == "" {
		normalized.TimeRange = RangeAll
	}
	return normalized
}

// Validate reports an unknown mode or range rather than silently defaulting it.
//
// A stored saved view holding a value this build does not know must be reported:
// quietly substituting "all" would show a wider result set than the view the user
// saved.
func (f LibraryFilter) Validate() error {
	normalized := f.Normalize()
	if !ValidMatchMode(normalized.TagMatchMode) {
		return errors.New("domain: unknown tag match mode " + normalized.TagMatchMode)
	}
	if !ValidMatchMode(normalized.AuthorMatchMode) {
		return errors.New("domain: unknown author match mode " + normalized.AuthorMatchMode)
	}
	if !ValidSavedTimeRange(normalized.TimeRange) {
		return errors.New("domain: unknown time range " + normalized.TimeRange)
	}
	return nil
}

// IsZero reports whether the filter would select everything.
func (f LibraryFilter) IsZero() bool {
	normalized := f.Normalize()
	return normalized.Query == "" &&
		len(normalized.TagIDs) == 0 &&
		len(normalized.AuthorHandles) == 0 &&
		normalized.ListID == "" &&
		!normalized.OnlyUnclassified &&
		!normalized.OnlyWithPendingSuggestions &&
		!normalized.OnlyWithMedia &&
		!normalized.OnlyLongform &&
		normalized.TimeRange == RangeAll
}

// ActiveCount is the number of active toggles, for the filter button's badge.
//
// Only the boolean toggles are counted: text, tags, authors and the time range
// are visible in the toolbar, so the badge means "hidden filters are on", and
// inflating it would make it useless as a warning.
func (f LibraryFilter) ActiveCount() int {
	// The suggestion row is one of the toggles, so it counts toward the badge.
	count := 0
	if f.OnlyWithMedia {
		count++
	}
	if f.OnlyLongform {
		count++
	}
	return count
}

// SavedAfter returns the cutoff for the time range, or the zero time for "all".
//
// It is the only implementation of this rule. The storage layer used to compute
// the same cutoff itself, and the two had already drifted apart -- one normalised
// to UTC and the other did not -- which is exactly how a filter silently starts
// selecting a different set. The normalisation lives here so there is one answer.
func (f LibraryFilter) SavedAfter(now time.Time) time.Time {
	days := SavedTimeRangeDays(f.Normalize().TimeRange)
	if days == 0 {
		return time.Time{}
	}
	return now.UTC().AddDate(0, 0, -days)
}

// Page size bounds for the library list.
const (
	DefaultLibraryPageSize = 100
	MaxLibraryPageSize     = 1000
)

// ErrInvalidPageSize reports a page size outside the allowed range.
var ErrInvalidPageSize = errors.New("domain: page size out of range")

// LibraryQuery is a filter, an order and a window.
type LibraryQuery struct {
	Filter LibraryFilter
	Sort   SortOrder
	Limit  int
	Offset int
}

// Normalize fills defaults and clamps the window.
func (q LibraryQuery) Normalize() (LibraryQuery, error) {
	normalized := q
	normalized.Filter = q.Filter.Normalize()

	if err := normalized.Filter.Validate(); err != nil {
		return LibraryQuery{}, err
	}

	if normalized.Sort == "" {
		normalized.Sort = SortTimeline
	}
	if !ValidSortOrder(normalized.Sort) {
		return LibraryQuery{}, errors.New("domain: unknown sort order " + normalized.Sort)
	}

	if normalized.Limit == 0 {
		normalized.Limit = DefaultLibraryPageSize
	}
	if normalized.Limit < 0 || normalized.Limit > MaxLibraryPageSize {
		return LibraryQuery{}, ErrInvalidPageSize
	}
	if normalized.Offset < 0 {
		normalized.Offset = 0
	}

	return normalized, nil
}

// dedupeStrings removes blanks and duplicates, optionally lowercasing.
//
// Author handles and search text are compared case-insensitively because X does,
// while tag ids are opaque and must keep their case.
func dedupeStrings(values []string, lower bool) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if lower {
			trimmed = strings.ToLower(trimmed)
		}
		if seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		out = append(out, trimmed)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// WorkspaceCounts is the sidebar's numbers.
//
// These are SQL aggregates, computed once, rather than a walk over every
// bookmark on each render.
type WorkspaceCounts struct {
	TotalBookmarks int
	// UnclassifiedCount is bookmarks with no tags at all.
	UnclassifiedCount  int
	PendingSuggestions int
	TagCount           int
	ListCount          int
}

// FacetCounts is what each sidebar row would show if it were clicked.
//
// A facet holds one value, and a click replaces it, so the count for a value is taken
// with that facet's own value left out and every other condition applied: the row says 3
// and clicking it shows 3. The value already in effect is counted the same way, which
// makes its number the current result count rather than something to work out.
//
// The maps are keyed by the stored value (a tag id, an author handle). A value with no
// entry has no bookmarks under the other conditions, which is a real answer, not a
// missing one: Ready says whether the counts were loaded at all, so a view built without
// them can fall back to its global summaries rather than claiming zero.
type FacetCounts struct {
	Ready bool
	// Tags is keyed by tag id.
	Tags map[string]int
	// Authors is keyed by author handle, as stored on the bookmark.
	Authors map[string]int
}

// ListSummary is a list and how many bookmarks it holds.
type ListSummary struct {
	List
	BookmarkCount int
}

// TagSummary is a tag and how many bookmarks carry it.
type TagSummary struct {
	Tag
	BookmarkCount int
	// PendingCount is how many suggestions are waiting for this tag name.
	PendingCount int
}

// AuthorSummary is an author and how many bookmarks they have.
type AuthorSummary struct {
	AuthorHandle  string
	AuthorName    string
	BookmarkCount int
	// AuthorAvatarURL is the picture the sidebar row shows at 24 points, the image
	// or a fallback. Empty when the bookmark that carried this handle held none.
	AuthorAvatarURL string
}
