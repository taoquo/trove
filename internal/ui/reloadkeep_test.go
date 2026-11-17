package ui

import (
	"context"
	"fmt"
	"testing"

	"github.com/taoquo/trove/internal/domain"
)

// loadAllPaged reads the whole fixture into the page one page at a time, the way scrolling
// does, and returns the ids now on screen.
func loadAllPaged(t *testing.T, shell *Shell, total int) []string {
	t.Helper()
	ctx := context.Background()
	shell.Reload(ctx)
	for len(shell.state.Data.Bookmarks) < total {
		shell.LoadMoreResults(ctx)
	}
	ids := make([]string, 0, total)
	for _, row := range shell.state.Data.Bookmarks {
		ids = append(ids, row.TweetID)
	}
	return ids
}

// A background refresh is not a first load: whatever the reader has paged in stays.
//
// The read used to ask for one page every time, so a sync finishing in the background --
// or any refresh at all -- cut a 500-row list back to 100 under the reader and left the
// scroll offset past the new end.
func TestABackgroundRefreshKeepsTheLoadedPage(t *testing.T) {
	shell, db := newShell(t)
	total := domain.DefaultLibraryPageSize * 5
	for i := 0; i < total; i++ {
		seed(t, db, fmt.Sprintf("%d", i), "ada", fmt.Sprintf("post %d", i), clockBase)
	}

	ids := loadAllPaged(t, shell, total)
	if len(ids) != total {
		t.Fatalf("%d bookmarks are loaded, want %d", len(ids), total)
	}

	const scrolledTo = float32(2400)
	shell.resultsScroll.Y = scrolledTo

	shell.Reload(context.Background())

	if got := len(shell.state.Data.Bookmarks); got != total {
		t.Errorf("a background refresh left %d rows on screen, want the %d that were loaded", got, total)
	}
	if shell.resultsScroll.Y != scrolledTo {
		t.Errorf("the refresh moved the list to %v, want it to stay at %v", shell.resultsScroll.Y, scrolledTo)
	}
	if shell.state.Data.Query.Limit < total {
		t.Errorf("the refresh asked for %d rows, want at least the %d already loaded",
			shell.state.Data.Query.Limit, total)
	}
}

// A new list starts at one page; the same list keeps what the reader picked.
//
// A refresh used to prune the batch selection against the page it read back. With a
// one-page refresh that meant a selection made across several pages silently collapsed to
// the first page's ids -- the reader's work disappearing without a click.
func TestABackgroundRefreshKeepsTheSelectionAndANewListDoesNot(t *testing.T) {
	shell, db := newShell(t)
	total := domain.DefaultLibraryPageSize * 5
	for i := 0; i < total; i++ {
		seed(t, db, fmt.Sprintf("%d", i), "ada", fmt.Sprintf("post %d", i), clockBase)
	}

	ids := loadAllPaged(t, shell, total)
	shell.state.SelectedIDs = make(map[string]bool, len(ids))
	for _, id := range ids {
		shell.state.SelectedIDs[id] = true
	}
	shell.state.Selected = ids[0]

	shell.Reload(context.Background())

	if got := len(shell.state.SelectedIDs); got != total {
		t.Errorf("a background refresh left %d selected, want all %d", got, total)
	}
	if shell.state.Selected != ids[0] {
		t.Errorf("the refresh closed the open card (%q), want %q", shell.state.Selected, ids[0])
	}

	// A changed condition is a new list, and ids that are not on it are no longer a
	// selection of anything.
	shell.state.Query = "post 1"
	shell.Reload(context.Background())

	page := make(map[string]bool, len(shell.state.Data.Bookmarks))
	for _, row := range shell.state.Data.Bookmarks {
		page[row.TweetID] = true
	}
	if len(page) == 0 {
		t.Fatal("the narrowed query matched nothing")
	}
	for id := range shell.state.SelectedIDs {
		if !page[id] {
			t.Errorf("a new list kept id %q, which is not on it", id)
		}
	}
	if len(shell.state.SelectedIDs) == total {
		t.Error("a new list kept the old selection whole")
	}
	if shell.state.Selected != "" {
		t.Errorf("the open card stayed open on a list that does not hold it: %q", shell.state.Selected)
	}
}
