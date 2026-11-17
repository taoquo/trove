package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/taoquo/trove/internal/domain"
)

// TestUpsertOverwritesWithTheNewValue is one half of a pair.
//
// domain.MergeDraft is the other half, and it does the opposite. The two exist together
// because the paths need different rules and a single test cannot say which one it
// protects: rewriting UpsertMany with COALESCE would leave MergeDraft's test green while
// sync silently stopped picking up an author's changed display name -- a defect whose
// only symptom is a stale name on a card, months later.
//
// The rule here is deliberate. A sync writes a complete API snapshot, so the newest value
// is the authoritative one.
func TestUpsertOverwritesWithTheNewValue(t *testing.T) {
	db := openTestDB(t)
	repo := NewBookmarkRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

	first := domain.Bookmark{
		TweetID: "1", TweetURL: "https://x.com/ada/status/1",
		AuthorName: "Ada Lovelace", AuthorHandle: "ada",
		Text: "as first seen", SavedAt: now, UpdatedAt: now,
	}
	if _, err := repo.UpsertMany(ctx, []domain.Bookmark{first}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}

	// The same post, as a later sync sees it: the author has changed their display name
	// and edited nothing else.
	later := now.Add(time.Hour)
	second := first
	second.AuthorName = "Ada L."
	second.UpdatedAt = later
	if _, err := repo.UpsertMany(ctx, []domain.Bookmark{second}); err != nil {
		t.Fatalf("UpsertMany again: %v", err)
	}

	got, found, err := repo.Get(ctx, "1")
	if err != nil || !found {
		t.Fatalf("Get: %v, found=%v", err, found)
	}
	if got.AuthorName != "Ada L." {
		t.Errorf("the author name is %q; a sync's snapshot is authoritative and must win",
			got.AuthorName)
	}
	// savedAt is the exception, and it is the user's own timeline.
	if !got.SavedAt.Equal(now) {
		t.Errorf("savedAt moved from %v to %v; a later sync must never move it", now, got.SavedAt)
	}
}

// TestKnownIDsCountsTheLocalDeletes pins the early-stop input.
//
// KnownIDs is what the incremental sync's early stop reads. A tombstoned id is
// one this device has seen, and X still returns it; counting it as new made the
// rule never fire. Unlike AllIDs it must include the tombstones.
func TestKnownIDsCountsTheLocalDeletes(t *testing.T) {
	db := openTestDB(t)
	repo := NewBookmarkRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

	if _, err := repo.UpsertMany(ctx, []domain.Bookmark{
		{TweetID: "1", TweetURL: "https://x.com/ada/status/1", AuthorName: "Ada", AuthorHandle: "ada",
			Text: "kept", SavedAt: now, UpdatedAt: now},
		{TweetID: "2", TweetURL: "https://x.com/ada/status/2", AuthorName: "Ada", AuthorHandle: "ada",
			Text: "deleted here", SavedAt: now, UpdatedAt: now},
	}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}
	if _, err := repo.DeleteMany(ctx, []string{"2"}); err != nil {
		t.Fatalf("DeleteMany: %v", err)
	}

	known, err := repo.KnownIDs(ctx)
	if err != nil {
		t.Fatalf("KnownIDs: %v", err)
	}
	seen := make(map[string]bool, len(known))
	for _, id := range known {
		seen[id] = true
	}
	if !seen["1"] || !seen["2"] {
		t.Errorf("KnownIDs = %v, want both 1 and the tombstoned 2", known)
	}

	// AllIDs is the other question -- what is stored -- and must not include the
	// tombstone, or a restore would think the row is still there.
	stored, err := repo.AllIDs(ctx)
	if err != nil {
		t.Fatalf("AllIDs: %v", err)
	}
	for _, id := range stored {
		if id == "2" {
			t.Errorf("AllIDs = %v, want the deleted row absent", stored)
		}
	}
}
