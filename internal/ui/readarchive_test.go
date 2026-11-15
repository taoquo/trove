package ui

import (
	"context"
	"testing"

	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/i18n"
	"github.com/taoquo/trove/internal/storage/sqlite"
)

// The two states a reader owns, as the shell sees them: the detail marks read, the row
// menu archives, and both reach the query through the filter table.

// TestTheUnreadAndArchiveConditionsReachTheQuery pins the mapping the panel and the chips
// share. The archive's zero value is left alone so a shell that never touched it keeps the
// default view.
func TestTheUnreadAndArchiveConditionsReachTheQuery(t *testing.T) {
	shell, _ := newShell(t)

	shell.state.OnlyUnread = true
	shell.state.OnlyArchived = true
	filter := shell.filter()
	if !filter.OnlyUnread {
		t.Error("the unread switch did not reach the filter")
	}
	if filter.Archived != domain.ArchivedOnly {
		t.Errorf("archive scope = %q, want %q", filter.Archived, domain.ArchivedOnly)
	}

	// Unticking the archive puts the shell back on the default, which is the empty value
	// Normalize reads as "hidden".
	shell.state.OnlyArchived = false
	if got := shell.filter().Archived; got != "" {
		t.Errorf("archive scope = %q after unticking, want the default", got)
	}
}

// TestTheUnreadAndArchiveHaveChips is the promise the disabled rows could not keep: a
// condition that narrows the list is named in the toolbar and can be removed there.
func TestTheUnreadAndArchiveHaveChips(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(ctx)

	shell.state.OnlyUnread = true
	shell.state.OnlyArchived = true

	conditions := shell.activeConditions()
	if len(conditions) != 2 {
		t.Fatalf("%d conditions, want unread and archived: %+v", len(conditions), conditions)
	}
	if want := shell.t().T(i18n.FilterUnread); conditions[0].Label != want {
		t.Errorf("first chip = %q, want %q", conditions[0].Label, want)
	}
	if want := shell.t().T(i18n.FilterArchived); conditions[1].Label != want {
		t.Errorf("second chip = %q, want %q", conditions[1].Label, want)
	}

	// A chip removes its own condition. The unread switch is the one checked here because it
	// is the one that stacks with the archive rather than replacing it.
	conditions[0].Remove()
	if shell.state.OnlyUnread {
		t.Error("removing the unread chip left the switch on")
	}
	if !shell.state.OnlyArchived {
		t.Error("removing the unread chip cleared the archive")
	}
}

// TestOpeningTheDetailMarksItRead is the read half of the feature, including the timing
// that keeps the panel open: the refresh waits for the detail to close.
func TestOpeningTheDetailMarksItRead(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(ctx)

	shell.state.OnlyUnread = true
	shell.Reload(ctx)
	if got := len(shell.state.Data.Bookmarks); got != 1 {
		t.Fatalf("the unread list holds %d rows, want 1", got)
	}

	shell.showDetail("1")

	stored, found, err := sqlite.NewBookmarkRepository(db).Get(ctx, "1")
	if err != nil || !found {
		t.Fatalf("Get = %v, %v", found, err)
	}
	if stored.Unread() {
		t.Error("opening the detail did not mark the bookmark read")
	}
	// The row the reader opened is still listed while the panel is open: refreshing under
	// them would close the panel they just opened.
	if got := len(shell.state.Data.Bookmarks); got != 1 {
		t.Errorf("the list lost the open row: %d rows", got)
	}
	if !shell.state.UnreadReloadPending {
		t.Error("the refresh was not held for the detail's close")
	}

	shell.CloseDetail()

	if shell.state.UnreadReloadPending {
		t.Error("closing the detail left the refresh pending")
	}
	if got := len(shell.state.Data.Bookmarks); got != 0 {
		t.Errorf("the unread list still holds %d rows after the read", got)
	}
}

// TestArchivingTakesTheRowOutOfTheListAndTheArchiveShowsIt exercises the action the row
// menu calls, on the same shell a reader uses.
func TestArchivingTakesTheRowOutOfTheListAndTheArchiveShowsIt(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post", clockBase)
	seed(t, db, "2", "grace", "another post", clockBase)
	shell.Reload(ctx)

	if err := shell.SetArchived(ctx, "1", true); err != nil {
		t.Fatalf("SetArchived: %v", err)
	}
	if ids := listedIDs(shell); len(ids) != 1 || ids[0] != "2" {
		t.Fatalf("the list holds %v after archiving, want [2]", ids)
	}

	// The archive is the same action's other view, not a second mechanism.
	shell.state.OnlyArchived = true
	shell.Reload(ctx)
	if ids := listedIDs(shell); len(ids) != 1 || ids[0] != "1" {
		t.Fatalf("the archive holds %v, want [1]", ids)
	}

	if err := shell.SetArchived(ctx, "1", false); err != nil {
		t.Fatalf("SetArchived(back): %v", err)
	}
	if ids := listedIDs(shell); len(ids) != 0 {
		t.Errorf("the archive still holds %v after the way back", ids)
	}

	shell.state.OnlyArchived = false
	shell.Reload(ctx)
	if ids := listedIDs(shell); len(ids) != 2 {
		t.Errorf("the default view holds %v, want both bookmarks", ids)
	}
}

func listedIDs(shell *Shell) []string {
	ids := make([]string, 0, len(shell.state.Data.Bookmarks))
	for _, row := range shell.state.Data.Bookmarks {
		ids = append(ids, row.TweetID)
	}
	return ids
}
