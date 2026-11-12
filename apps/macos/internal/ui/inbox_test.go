package ui

import (
	"context"
	"testing"

	"github.com/taoquo/trove/apps/macos/internal/domain"
	"github.com/taoquo/trove/apps/macos/internal/ports"
	"github.com/taoquo/trove/apps/macos/internal/storage/sqlite"
	"github.com/taoquo/trove/apps/macos/internal/usecase/tagging"
)

// newInboxShell builds a shell with a tagging service, which is what carries the
// post-sync step.
func newInboxShell(t *testing.T) (*Shell, *sqlite.DB) {
	t.Helper()

	shell, db := newShell(t)
	repos := ports.Repositories{
		Bookmarks:   sqlite.NewBookmarkRepository(db),
		Tags:        sqlite.NewTagRepository(db),
		Lists:       sqlite.NewListRepository(db),
		Suggestions: sqlite.NewSuggestionRepository(db),
		Analysis:    sqlite.NewAnalysisRepository(db),
		Settings:    sqlite.NewSettingsRepository(db),
	}
	// The service is built with the full set of repositories: the post-sync step writes
	// to both the list and the bookmark tables.
	shell.UseTagging(tagging.New(repos, fixedClock{now: clockBase}, nil))
	return shell, db
}

// TestSyncedBookmarksReachTheInbox covers the step that was written and never called.
//
// Its own comment says a bookmark that is in no list is invisible in every list view.
// The behaviour under test is therefore that a bookmark with no list ends up in the
// Inbox once the post-sync step runs -- and that the step is reachable from the shell,
// which is what was missing.
func TestSyncedBookmarksReachTheInbox(t *testing.T) {
	shell, db := newInboxShell(t)
	ctx := context.Background()

	// A bookmark stored the way a sync stores it: with no list of its own.
	bookmark := domain.Bookmark{
		TweetID: "synced-1", TweetURL: "https://x.com/ada/status/synced-1",
		AuthorName: "ada", AuthorHandle: "ada", Text: "a synced post",
		SavedAt: clockBase, LastSeenAt: clockBase, UpdatedAt: clockBase,
	}
	if _, err := sqlite.NewBookmarkRepository(db).UpsertMany(ctx, []domain.Bookmark{bookmark}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}

	// Before the step, it belongs to no list. The join table is read directly: the
	// repository exposes the assignment, not the query.
	if lists := listsFor(t, db, "synced-1"); len(lists) != 0 {
		t.Fatalf("the fixture bookmark already has a list: %v", lists)
	}

	assigned := shell.AssignSyncedBookmarksToInbox(ctx)
	if assigned == 0 {
		t.Fatal("the post-sync step assigned nothing")
	}

	// After it, the Inbox holds it.
	lists := listsFor(t, db, "synced-1")
	foundInbox := false
	for _, id := range lists {
		if id == domain.InboxListID {
			foundInbox = true
		}
	}
	if !foundInbox {
		t.Errorf("the synced bookmark is in %v, not the Inbox", lists)
	}

	// The Inbox itself now exists, which is what the sidebar reads.
	if _, found, err := sqlite.NewListRepository(db).Get(ctx, domain.InboxListID); err != nil || !found {
		t.Errorf("the Inbox was not created: found=%v err=%v", found, err)
	}

	// Running it again is a no-op rather than a duplicate assignment.
	if again := shell.AssignSyncedBookmarksToInbox(ctx); again != 0 {
		t.Errorf("a second run assigned %d more", again)
	}
}

// TestAssignWithoutATaggingServiceDoesNothing keeps the shell usable without one, which
// is how several render tests build it.
// listsFor reads the lists a bookmark belongs to.
func listsFor(t *testing.T, db *sqlite.DB, bookmarkID string) []string {
	t.Helper()

	rows, err := db.Query(`SELECT list_id FROM bookmark_lists WHERE bookmark_id = ?`, bookmarkID)
	if err != nil {
		t.Fatalf("query lists: %v", err)
	}
	defer rows.Close()

	var lists []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan list id: %v", err)
		}
		lists = append(lists, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate lists: %v", err)
	}
	return lists
}

func TestAssignWithoutATaggingServiceDoesNothing(t *testing.T) {
	shell, _ := newShell(t)
	if got := shell.AssignSyncedBookmarksToInbox(context.Background()); got != 0 {
		t.Errorf("without a tagging service the step assigned %d", got)
	}
}
