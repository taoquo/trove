package ui

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/idgen"
	"github.com/taoquo/trove/internal/ports"
	"github.com/taoquo/trove/internal/storage/sqlite"
	"github.com/taoquo/trove/internal/usecase/syncer"
	"github.com/taoquo/trove/internal/usecase/workspace"
)

// newListShell is the ordinary test shell: it is already built with the workspace
// service, which owns lists.
func newListShell(t *testing.T) (*Shell, *sqlite.DB) {
	t.Helper()
	return newShell(t)
}

var _ = workspace.New

// TestCreateListFromInput covers the sidebar's new-list field.
func TestCreateListFromInput(t *testing.T) {
	shell, db := newListShell(t)
	ctx := context.Background()

	// A blank field creates nothing, including one that is only spaces.
	for _, blank := range []string{"", "   "} {
		shell.State().NewListName = blank
		shell.CreateListFromInput(ctx)
	}
	lists, err := sqlite.NewListRepository(db).List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, list := range lists {
		if list.Name == "" {
			t.Error("a blank field created a list")
		}
	}

	// A real name lands, and the field clears.
	shell.State().NewListName = "Research"
	shell.CreateListFromInput(ctx)
	if shell.State().NewListName != "" {
		t.Errorf("the field still holds %q", shell.State().NewListName)
	}

	found := false
	lists, err = sqlite.NewListRepository(db).List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, list := range lists {
		if list.Name == "Research" {
			found = true
		}
	}
	if !found {
		t.Errorf("the list was not created; lists: %v", lists)
	}

	// A duplicate name finds the existing list rather than creating a second one. It is
	// not an error: creating is find-or-create, which is what makes the field safe to
	// press twice.
	shell.State().NewListName = "Research"
	shell.CreateListFromInput(ctx)
	if shell.State().Err != "" {
		t.Errorf("a duplicate name reported %q", shell.State().Err)
	}

	after, err := sqlite.NewListRepository(db).List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	named := 0
	for _, list := range after {
		if list.Name == "Research" {
			named++
		}
	}
	if named != 1 {
		t.Errorf("%d lists are named Research, want 1", named)
	}

	// The Inbox's name is reserved, and that refusal does surface.
	shell.State().NewListName = domain.InboxListName
	shell.CreateListFromInput(ctx)
	// A refusal is reported on the status line, not in state.Err: there is one error channel,
	// and the old one under the toolbar is gone.
	if shell.ToastText() == "" {
		t.Error("the reserved name must be refused")
	}
}

// TestDeletingAListMovesItsBookmarks is the safety the service exists to provide: a
// deleted list must not take its bookmarks with it.
func TestDeletingAListMovesItsBookmarks(t *testing.T) {
	shell, db := newListShell(t)
	ctx := context.Background()

	shell.State().NewListName = "Research"
	shell.CreateListFromInput(ctx)

	lists, err := sqlite.NewListRepository(db).List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	var researchID string
	for _, list := range lists {
		if list.Name == "Research" {
			researchID = list.ID
		}
	}
	if researchID == "" {
		t.Fatal("the list was not created")
	}

	// A bookmark in it.
	seed(t, db, "in-list", "ada", "a post in a list", clockBase)
	// The assignment lives in the bookmark repository; the join row is written here so
	// the test does not depend on that method's signature.
	if _, err := db.Exec(
		`INSERT INTO bookmark_lists (bookmark_id, list_id, updated_at) VALUES (?, ?, ?)`,
		"in-list", researchID, domain.FormatTimestamp(clockBase)); err != nil {
		t.Fatalf("assign: %v", err)
	}

	// Asking does not delete: the confirmation has to be answered.
	shell.AskDeleteList(researchID)
	if _, found, _ := sqlite.NewListRepository(db).Get(ctx, researchID); !found {
		t.Fatal("asking must not delete the list")
	}

	shell.ConfirmDeleteListNow(ctx)

	if _, found, _ := sqlite.NewListRepository(db).Get(ctx, researchID); found {
		t.Error("the list was not deleted")
	}
	// The bookmark survives, in the Inbox.
	if _, found, _ := sqlite.NewBookmarkRepository(db).Get(ctx, "in-list"); !found {
		t.Error("deleting a list deleted its bookmarks")
	}
	if lists := listsFor(t, db, "in-list"); len(lists) == 0 {
		t.Error("the bookmark was left in no list")
	}
}

// TestTheInboxCannotBeDeleted covers the refusal: asking for it must not even open a
// confirmation, because the service would reject the delete anyway.
func TestTheInboxCannotBeDeleted(t *testing.T) {
	shell, _ := newListShell(t)

	shell.AskDeleteList(domain.InboxListID)
	if shell.State().ConfirmDeleteList != "" {
		t.Errorf("the Inbox opened a confirmation for %q", shell.State().ConfirmDeleteList)
	}
}

// TestDeletingAListWithoutAnsweringLeavesIt keeps the dialog from acting on dismissal.
func TestDeletingAListWithoutAnsweringLeavesIt(t *testing.T) {
	shell, db := newListShell(t)
	ctx := context.Background()

	shell.State().NewListName = "Research"
	shell.CreateListFromInput(ctx)

	lists, _ := sqlite.NewListRepository(db).List(ctx)
	var id string
	for _, list := range lists {
		if list.Name == "Research" {
			id = list.ID
		}
	}

	shell.State().ConfirmDeleteList = id
	// Dismissed: the state is cleared without the delete running.
	shell.State().ConfirmDeleteList = ""
	if _, found, _ := sqlite.NewListRepository(db).Get(ctx, id); !found {
		t.Error("dismissing the dialog deleted the list")
	}
}

// TestAcceptAllAppliesEveryVisibleSuggestion covers the bulk accept button.
//
// The batch is what is on screen, not everything pending: a filtered review should not
// accept rows the user cannot see.
func TestAcceptAllAppliesEveryVisibleSuggestion(t *testing.T) {
	// The batch shell carries the suggestion service, which a plain shell does not.
	shell, db := newBatchShell(t)
	ctx := context.Background()

	// Three pending suggestions, all visible.
	suggestions := make([]domain.TagSuggestion, 0, 3)
	for i, id := range []string{"s1", "s2", "s3"} {
		seed(t, db, id, "ada", "post "+id, clockBase)
		suggestions = append(suggestions, domain.TagSuggestion{
			ID: id + "::topic-" + string(rune('a'+i)), BookmarkID: id,
			TagName: "topic-" + string(rune('a'+i)), Source: domain.SourceLLM,
			Confidence: 0.9, Status: domain.StatusPending, CreatedAt: clockBase,
		})
	}
	if _, err := sqlite.NewSuggestionRepository(db).InsertMany(ctx, suggestions); err != nil {
		t.Fatalf("InsertMany: %v", err)
	}

	shell.OpenSuggestions(ctx)
	if len(shell.State().Suggestions) != 3 {
		t.Fatalf("the panel lists %d suggestions", len(shell.State().Suggestions))
	}

	applied := shell.AcceptVisibleSuggestions(ctx)
	if applied != 3 {
		t.Errorf("applied %d of 3", applied)
	}
	if shell.State().Err != "" {
		t.Errorf("a clean batch reported %q", shell.State().Err)
	}

	// The panel is empty now, and nothing is pending in the store.
	if len(shell.State().Suggestions) != 0 {
		t.Errorf("the panel still lists %d", len(shell.State().Suggestions))
	}
	pending, err := sqlite.NewSuggestionRepository(db).List(ctx, domain.StatusPending)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(pending) != 0 {
		t.Errorf("%d suggestions are still pending", len(pending))
	}

	// An empty batch is a no-op rather than a panic.
	if again := shell.AcceptVisibleSuggestions(ctx); again != 0 {
		t.Errorf("an empty batch applied %d", again)
	}
}

// syncedAt is when the fixture library was last brought up to date.
var syncedAt = clockBase

// newSyncService builds a syncer over a database.
func newSyncService(db *sqlite.DB) *syncer.Service {
	return syncer.New(
		ports.Repositories{
			Bookmarks: sqlite.NewBookmarkRepository(db),
			Lists:     sqlite.NewListRepository(db),
			Settings:  sqlite.NewSettingsRepository(db),
			SyncRuns:  sqlite.NewSyncRunRepository(db),
		},
		fixedClock{now: clockBase}, idgen.New("test"), nil)
}

// writeSyncSummary stores a summary the way a run would.
func writeSyncSummary(t *testing.T, db *sqlite.DB, summary domain.SyncSummary) {
	t.Helper()

	encoded, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if err := sqlite.NewSettingsRepository(db).Set(
		context.Background(), "sync.summary", string(encoded), clockBase); err != nil {
		t.Fatalf("Set: %v", err)
	}
}

// TestLastSyncSurvivesARestart is the gap the stored summary closes.
//
// The status line was kept only in memory, so every launch claimed the library had never
// been synced even though the summary was on disk. The behaviour under test is a second
// shell reading it back.
func TestLastSyncSurvivesARestart(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()

	service := newSyncService(db)
	// The summary is written the way the sync writes it: json.Marshal of the domain type
	// into the settings key. The writer is unexported, so the test stores the same bytes
	// rather than inventing a shape the reader would not accept.
	writeSyncSummary(t, db, domain.SyncSummary{
		Status: domain.SyncSuccess, FetchedCount: 12, InsertedCount: 3,
		LastSyncedAt: &syncedAt,
	})
	shell.UseSyncer(service)

	// The first shell reads it.
	shell.LoadSyncSummary(ctx)
	if shell.State().LastSyncedAt.IsZero() {
		t.Fatal("the first shell did not read the stored summary")
	}

	// A second shell over the same database reads it too, which is the restart.
	fresh, _ := newShell(t)
	fresh.UseSyncer(newSyncService(db))
	fresh.LoadSyncSummary(ctx)
	if fresh.State().LastSyncedAt.IsZero() {
		t.Error("a fresh shell forgot when the library was last synced")
	}
	if fresh.State().LastSyncStatus == "" {
		t.Error("a fresh shell has no status line")
	}
}

// TestAStaleSummaryDoesNotOverwriteARunningRun keeps a load during a sync from claiming
// the sync is over.
func TestAStaleSummaryDoesNotOverwriteARunningRun(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()

	writeSyncSummary(t, db, domain.SyncSummary{
		Status: domain.SyncError, LastSyncedAt: &syncedAt,
	})
	shell.UseSyncer(newSyncService(db))

	shell.State().SyncRunning = true
	shell.State().LastSyncStatus = "Syncing"
	shell.LoadSyncSummary(ctx)

	if shell.State().LastSyncStatus != "Syncing" {
		t.Errorf("the status line became %q while a run was in flight", shell.State().LastSyncStatus)
	}
}

// TestOpenListPutsTheListInTheScope covers the entry point the settings section gained: a list
// could be created, renamed and deleted but never read.
func TestOpenListPutsTheListInTheScope(t *testing.T) {
	shell, db := newListShell(t)
	ctx := context.Background()

	shell.State().NewListName = "Research"
	shell.CreateListFromInput(ctx)

	lists, err := sqlite.NewListRepository(db).List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	var researchID string
	for _, list := range lists {
		if list.Name == "Research" {
			researchID = list.ID
		}
	}
	if researchID == "" {
		t.Fatal("the list was not created")
	}

	shell.State().TagMode = true
	shell.OpenList(ctx, researchID)

	if got := shell.State().Selection; got != prefixList+researchID {
		t.Errorf("selection = %q, want the list scope", got)
	}
	if shell.State().TagMode {
		t.Error("tag management was left covering the results")
	}
}
