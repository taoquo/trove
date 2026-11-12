package domain

import "time"

// SortOrder is how the results list is ordered.
type SortOrder = string

// The five orders the domain knows. Note that the old UI could only cycle
// through four of them, so "saved-desc" was reachable only by restoring a saved
// view or a backup. The native UI is meant to expose all five.
// See docs/baseline-audit.md §2.3 and docs/ui-spec.md §2.3.
const (
	SortTimeline    SortOrder = "timeline"
	SortSavedDesc   SortOrder = "saved-desc"
	SortSavedAsc    SortOrder = "saved-asc"
	SortCreatedDesc SortOrder = "created-desc"
	SortLikesDesc   SortOrder = "likes-desc"
)

// AllSortOrders is the whitelist. It must contain every value the old extension
// could persist, because a backup whose sortOrder falls outside it is rejected
// outright rather than defaulted.
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
// The earlier note here said the extension's saved-view UI "was switched off". That is
// wrong for src/, and the correction matters because the comment is what a reader would
// trust:
//
//	src/options/components/WorkspaceSidebar.tsx
//	  :44   data-testid="saved-views-section"
//	  :53   data-testid="saved-view-name-input"
//	  :70   data-testid="save-current-view"      <AppIcon name="bookmark" size={14} />
//	  :78   data-testid="saved-view-apply"
//
// The section renders, the create field and its button are wired to onSaveCurrentView,
// and each view has apply and delete controls. So the data must round-trip *and* the
// interface must offer the same three actions -- which it did not, which is the gap this
// comment used to hide.
type SavedView struct {
	ID   string
	Name string
	// Query is the free-text search box contents.
	Query               string
	ActiveTagIDs        []string
	ActiveAuthorHandles []string
	OnlyWithMedia       bool
	OnlyLongform        bool
	SortOrder           SortOrder
	ViewMode            ViewMode
	CreatedAt           time.Time
	UpdatedAt           time.Time
}
