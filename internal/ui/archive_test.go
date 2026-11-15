package ui

import (
	"context"
	"testing"

	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/i18n"
)

// The archive as the shell sees it: the row menu sets it, the filter table names it, and
// the two views are the same query with one scope changed.

// TestTheArchiveConditionReachesTheQuery pins the mapping the panel and the chips share.
// The zero value is left alone so a shell that never touched it keeps the default view.
func TestTheArchiveConditionReachesTheQuery(t *testing.T) {
	shell, _ := newShell(t)

	shell.state.OnlyArchived = true
	if got := shell.filter().Archived; got != domain.ArchivedOnly {
		t.Errorf("archive scope = %q, want %q", got, domain.ArchivedOnly)
	}

	// Unticking it puts the shell back on the default, which is the empty value Normalize
	// reads as "hidden".
	shell.state.OnlyArchived = false
	if got := shell.filter().Archived; got != "" {
		t.Errorf("archive scope = %q after unticking, want the default", got)
	}
}

// TestTheArchiveHasAChip is the promise the disabled rows could not keep: a condition that
// changes the list is named in the toolbar and can be removed there.
func TestTheArchiveHasAChip(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(ctx)

	shell.state.OnlyArchived = true

	conditions := shell.activeConditions()
	if len(conditions) != 1 {
		t.Fatalf("%d conditions, want only the archive: %+v", len(conditions), conditions)
	}
	if want := shell.t().T(i18n.FilterArchived); conditions[0].Label != want {
		t.Errorf("the chip = %q, want %q", conditions[0].Label, want)
	}

	conditions[0].Remove()
	if shell.state.OnlyArchived {
		t.Error("removing the archive chip left the mode on")
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
