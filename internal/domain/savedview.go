package domain

import "time"

// SortOrder is how the results list is ordered.
type SortOrder = string

// The five orders the domain knows. All five are persisted, and the UI exposes
// all five.
const (
	SortTimeline    SortOrder = "timeline"
	SortSavedDesc   SortOrder = "saved-desc"
	SortSavedAsc    SortOrder = "saved-asc"
	SortCreatedDesc SortOrder = "created-desc"
	SortLikesDesc   SortOrder = "likes-desc"
)

// AllSortOrders is the whitelist. A backup whose sortOrder falls outside it is
// rejected outright rather than defaulted.
var AllSortOrders = []SortOrder{
	SortTimeline, SortSavedDesc, SortSavedAsc, SortCreatedDesc, SortLikesDesc,
}

// ValidSortOrder reports whether s is one of AllSortOrders.
func ValidSortOrder(s string) bool {
	for _, candidate := range AllSortOrders {
		if s == candidate {
			return true
		}
	}
	return false
}

// ViewMode is the results layout.
type ViewMode = string

const (
	ViewGrid ViewMode = "grid"
	ViewList ViewMode = "list"
)

// ValidViewMode reports whether m is a known layout.
func ValidViewMode(m string) bool { return m == ViewGrid || m == ViewList }

// SavedView is a named, reusable set of filters.
//
// A saved view round-trips: the filter set is stored, and the interface offers
// create, apply and delete, so the data and the controls stay in step.
type SavedView struct {
	ID   string
	Name string
	// Query is the free-text search box contents.
	Query               string
	ActiveTagIDs        []string
	ActiveAuthorHandles []string
	OnlyWithMedia       bool
	OnlyWithVideo       bool
	OnlyLongform        bool
	// OnlyArchived stores the view's archive mode. A view saved in the archive is a
	// view of the archive: the reader who returns to it meant that list, not the
	// library the archive hides. Applying any other view returns to the library,
	// because that is the state those views were saved in.
	OnlyArchived bool
	SortOrder    SortOrder
	ViewMode     ViewMode
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
