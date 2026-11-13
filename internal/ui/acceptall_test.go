package ui

import (
	"context"
	"testing"

	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/storage/sqlite"
)

// TestAcceptingTheWholeBatchRefreshesWhatTheFeedCounts pins the difference between the two
// accept paths.
//
// Accepting one suggestion reloads the workspace, so the card, the banner and the tag
// page's counts all move. The batch in the review panel wrote through the service and
// then refreshed only the panel's own list: the feed behind it went on saying
// "N pending tags" for suggestions that no longer existed, and the reader had to do
// something else to see that the batch had landed.
func TestAcceptingTheWholeBatchRefreshesWhatTheFeedCounts(t *testing.T) {
	shell, db := newBatchShell(t)
	seed(t, db, "1", "ada", "the first bookmark", clockBase)

	suggestion := domain.TagSuggestion{
		ID:         domain.SuggestionID("1", "rust"),
		BookmarkID: "1",
		TagName:    "rust",
		Source:     domain.SourceLLM,
		Confidence: 0.9,
		Status:     domain.StatusPending,
		CreatedAt:  clockBase,
	}
	if _, err := sqlite.NewSuggestionRepository(db).InsertMany(
		context.Background(), []domain.TagSuggestion{suggestion}); err != nil {
		t.Fatalf("seed the suggestion: %v", err)
	}

	shell.Reload(context.Background())
	shell.openSuggestions(context.Background())
	if got := len(shell.State().PendingSuggestions); got != 1 {
		t.Fatalf("the feed banner counts %d suggestions, want 1", got)
	}

	if applied := shell.AcceptVisibleSuggestions(context.Background()); applied != 1 {
		t.Fatalf("applied %d suggestions, want 1", applied)
	}

	if got := len(shell.State().PendingSuggestions); got != 0 {
		t.Errorf("the feed banner still counts %d suggestions after the batch was applied", got)
	}
}
