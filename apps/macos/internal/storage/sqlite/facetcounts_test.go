package sqlite

import (
	"context"
	"testing"

	"github.com/taoquo/trove/apps/macos/internal/domain"
)

// P6 of docs/filter-interaction-plan.md: the sidebar's numbers come from the same filter
// as the list, so a row cannot promise a result set the list will not show.

// TestFacetCountsReadTheFilter covers both directions: an unfiltered library counts its
// own totals, and a scope plus a tag narrows every row the same way the list is narrowed.
func TestFacetCountsReadTheFilter(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	repo := NewBookmarkRepository(db)
	tags := NewTagRepository(db)
	lists := NewListRepository(db)

	bookmark := func(id, handle string) domain.Bookmark {
		return domain.Bookmark{
			TweetID: id, TweetURL: "https://x.com/" + handle + "/status/" + id,
			AuthorName: handle, AuthorHandle: handle, Text: "post " + id,
			SavedAt: clockBase, LastSeenAt: clockBase, UpdatedAt: clockBase,
		}
	}
	if _, err := repo.UpsertMany(ctx, []domain.Bookmark{
		bookmark("1", "ada"), bookmark("2", "grace"), bookmark("3", "ada"),
		bookmark("4", "lin"), bookmark("5", "lin"),
	}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}
	if _, err := repo.AssignMissingToInbox(ctx, clockBase); err != nil {
		t.Fatalf("AssignMissingToInbox: %v", err)
	}
	// A second list, so the scope has something to exclude.
	if err := lists.Create(ctx, domain.List{
		ID: "list-work", Name: "Work", Normalized: domain.NormalizeListName("Work"),
		CreatedAt: clockBase, UpdatedAt: clockBase,
	}); err != nil {
		t.Fatalf("Create list: %v", err)
	}
	if err := repo.AssignToList(ctx, []string{"4"}, "list-work", clockBase); err != nil {
		t.Fatalf("AssignToList: %v", err)
	}

	for _, tag := range []domain.Tag{
		{ID: "go", Name: "Go", CanonicalKey: domain.CanonicalTagKey("Go"), CreatedAt: domain.FormatTimestamp(clockBase), UpdatedAt: domain.FormatTimestamp(clockBase)},
		{ID: "rust", Name: "Rust", CanonicalKey: domain.CanonicalTagKey("Rust"), CreatedAt: domain.FormatTimestamp(clockBase), UpdatedAt: domain.FormatTimestamp(clockBase)},
	} {
		if err := tags.Create(ctx, tag); err != nil {
			t.Fatalf("Create tag: %v", err)
		}
	}
	relations := []domain.BookmarkTag{
		{BookmarkID: "1", TagID: "go", CreatedAt: clockBase, Source: domain.TagSourceManual},
		{BookmarkID: "2", TagID: "go", CreatedAt: clockBase, Source: domain.TagSourceManual},
		{BookmarkID: "2", TagID: "rust", CreatedAt: clockBase, Source: domain.TagSourceManual},
		{BookmarkID: "4", TagID: "go", CreatedAt: clockBase, Source: domain.TagSourceManual},
		{BookmarkID: "5", TagID: "rust", CreatedAt: clockBase, Source: domain.TagSourceManual},
	}
	if err := tags.AttachMany(ctx, relations); err != nil {
		t.Fatalf("AttachMany: %v", err)
	}

	// Unfiltered: the library's own totals, which is what the sidebar showed before.
	all, err := repo.FacetCounts(ctx, domain.LibraryFilter{})
	if err != nil {
		t.Fatalf("FacetCounts: %v", err)
	}
	if !all.Ready {
		t.Error("a loaded count must say so")
	}
	for tagID, want := range map[string]int{"go": 3, "rust": 2} {
		if got := all.Tags[tagID]; got != want {
			t.Errorf("unfiltered tag %q = %d, want %d", tagID, got, want)
		}
	}
	for handle, want := range map[string]int{"ada": 2, "grace": 1, "lin": 2} {
		if got := all.Authors[handle]; got != want {
			t.Errorf("unfiltered author %q = %d, want %d", handle, got, want)
		}
	}

	// The Inbox plus #go. A tag row is counted with the tag condition dropped, because a
	// click on it replaces #go rather than intersecting with it: #rust is 2, the two inbox
	// bookmarks that carry it, not the 1 that also carry #go.
	filtered, err := repo.FacetCounts(ctx, domain.LibraryFilter{
		ListID: domain.InboxListID, TagIDs: []string{"go"},
	})
	if err != nil {
		t.Fatalf("FacetCounts: %v", err)
	}
	for tagID, want := range map[string]int{"go": 2, "rust": 2} {
		if got := filtered.Tags[tagID]; got != want {
			t.Errorf("inbox #go tag %q = %d, want %d", tagID, got, want)
		}
	}
	// Author rows keep the tag condition (different facets still intersect): only b1 and
	// b2 are in the inbox carrying #go, one each by ada and grace.
	for handle, want := range map[string]int{"ada": 1, "grace": 1, "lin": 0} {
		if got := filtered.Authors[handle]; got != want {
			t.Errorf("inbox #go author %q = %d, want %d", handle, got, want)
		}
	}

	// The mirror case: an author condition does not narrow the author rows, but does narrow
	// the tag rows. @ada in the inbox is b1 and b3: one #go and no #rust.
	byAuthor, err := repo.FacetCounts(ctx, domain.LibraryFilter{
		ListID: domain.InboxListID, AuthorHandles: []string{"ada"},
	})
	if err != nil {
		t.Fatalf("FacetCounts: %v", err)
	}
	for tagID, want := range map[string]int{"go": 1, "rust": 0} {
		if got := byAuthor.Tags[tagID]; got != want {
			t.Errorf("inbox @ada tag %q = %d, want %d", tagID, got, want)
		}
	}
	for handle, want := range map[string]int{"ada": 2, "grace": 1, "lin": 1} {
		if got := byAuthor.Authors[handle]; got != want {
			t.Errorf("inbox @ada author %q = %d, want %d", handle, got, want)
		}
	}
}
