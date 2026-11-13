package ui

import (
	"context"
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/i18n"
	"github.com/taoquo/trove/internal/storage/sqlite"
)

// TestARealScaleLibraryCanStillBeBulkSelected is the bulk bar at 1,200 rows.
//
// "Select all results" runs MatchingIDs, which refuses above MaxLibraryPageSize and
// explains why -- the refusal is right, because a bulk action must not run on a partial
// set. The second button, "select loaded", selects the rows the list actually holds. Without it,
// a reader at this size has no one-press bulk selection at all and is left ticking boxes by hand.
func TestARealScaleLibraryCanStillBeBulkSelected(t *testing.T) {
	shell, db := newBatchShell(t)
	ctx := context.Background()

	rows := make([]domain.Bookmark, 0, 1200)
	for i := 1; i <= 1200; i++ {
		id := fmt.Sprintf("%d", i)
		rows = append(rows, domain.Bookmark{
			TweetID: id, TweetURL: "https://x.com/ada/status/" + id,
			AuthorName: "ada", AuthorHandle: "ada", Text: "post " + id,
			SavedAt: clockBase, LastSeenAt: clockBase, UpdatedAt: clockBase,
		})
	}
	if _, err := sqlite.NewBookmarkRepository(db).UpsertMany(ctx, rows); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}
	shell.Reload(ctx)

	// The current-results button refuses, and says so rather than selecting a prefix.
	shell.SelectAllResults(ctx)
	if got := len(shell.State().SelectedIDs); got != 0 {
		t.Fatalf("the refused select-all still selected %d rows", got)
	}
	if got, want := shell.State().LastBundleReport, shell.t().T(i18n.BatchTooMany); got != want {
		t.Fatalf("the refusal reads %q, want %q", got, want)
	}

	// The loaded button is the way out, and it is on screen beside it.
	shell.state.SelectedIDs = map[string]bool{"1": true}
	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	label := shell.t().T(i18n.BatchSelectLoaded)
	if !tester.HasText(label) {
		t.Fatalf("the bulk bar offers no way to select what is loaded; want %q, texts: %q",
			label, tester.Texts())
	}
	if err := tester.Click(label); err != nil {
		t.Fatalf("clicking %q: %v", label, err)
	}
	tester.Frame()

	loaded := len(shell.State().Data.Bookmarks)
	if loaded == 0 {
		t.Fatal("nothing is loaded, so the test proves nothing")
	}
	if got := len(shell.State().SelectedIDs); got != loaded {
		t.Errorf("%d rows selected after select-loaded, want the %d loaded", got, loaded)
	}
}
