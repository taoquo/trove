package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/taoquo/trove/internal/domain"
)

// TestDeleteAllClearsRowsAndReportsTheCount covers the store's "reset local data" half.
//
// DeleteAll short-circuits when the table is already empty, and mutation testing found
// the inversion of that check survives the whole suite: returning zero without deleting
// when rows ARE present. That is the shape of a reset that silently leaves data behind,
// and the reset is reached from the data-safety panel, where the user has just agreed to
// lose everything.
func TestDeleteAllClearsRowsAndReportsTheCount(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	repository := NewSuggestionRepository(db)
	created := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

	suggestions := make([]domain.TagSuggestion, 0, 3)
	for _, id := range []string{"s1", "s2", "s3"} {
		suggestions = append(suggestions, domain.TagSuggestion{
			ID: id, BookmarkID: "b1", TagName: "Go", Source: domain.SourceLLM,
			Confidence: 0.9, Status: domain.StatusPending, CreatedAt: created,
		})
	}
	inserted, err := repository.InsertMany(ctx, suggestions)
	if err != nil {
		t.Fatalf("InsertMany: %v", err)
	}
	if inserted != 3 {
		t.Fatalf("inserted %d, want 3", inserted)
	}

	removed, err := repository.DeleteAll(ctx)
	if err != nil {
		t.Fatalf("DeleteAll: %v", err)
	}
	if removed != 3 {
		t.Errorf("DeleteAll reported %d removed, want 3", removed)
	}

	// The rows must actually be gone, which the count alone does not prove.
	remaining, err := repository.All(ctx)
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if len(remaining) != 0 {
		t.Errorf("%d suggestions survived the delete", len(remaining))
	}

	// And an already-empty table reports zero rather than an error, which is the
	// short-circuit the mutation inverted.
	again, err := repository.DeleteAll(ctx)
	if err != nil {
		t.Fatalf("DeleteAll on an empty table: %v", err)
	}
	if again != 0 {
		t.Errorf("an empty table reported %d removed, want 0", again)
	}
}
