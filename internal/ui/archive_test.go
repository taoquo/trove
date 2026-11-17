package ui

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

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

// TestTheBatchBarArchivesTheSelectionAndUndoPutsItBack is the bulk action, end to end on
// a rendered shell.
//
// The bar's archive used to be the row menu repeated once per card: a reader with twenty
// picked bookmarks had twenty context menus to open. The receipt's Undo is what makes the
// one click safe, and it restores the rows to the positions they had, because archiving
// only writes archived_at and the order never moved.
func TestTheBatchBarArchivesTheSelectionAndUndoPutsItBack(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	for i, id := range []string{"1", "2", "3"} {
		seed(t, db, id, "ada", "post "+id, clockBase.Add(time.Duration(i)*time.Minute))
	}
	shell.Reload(ctx)
	before := listedIDs(shell)
	if len(before) != 3 {
		t.Fatalf("the list holds %v, want three rows", before)
	}
	shell.state.SelectedIDs = map[string]bool{"1": true, "2": true}

	tester := ui.NewTester(shell.View, 1180, 900)
	tester.Frame()
	archiveLabel := shell.t().T(i18n.ActionArchive)
	if !tester.HasText(archiveLabel) {
		t.Fatalf("the batch bar does not offer %q", archiveLabel)
	}

	if err := tester.Click(archiveLabel); err != nil {
		t.Fatalf("Click(%q): %v", archiveLabel, err)
	}
	tester.Frame()

	if ids := listedIDs(shell); len(ids) != 1 || ids[0] != "3" {
		t.Fatalf("the list holds %v after archiving two, want [3]", ids)
	}
	if len(shell.state.SelectedIDs) != 0 {
		t.Errorf("the archived rows are still selected: %v", shell.state.SelectedIDs)
	}
	if want := shell.t().Tf(i18n.ToastArchivedMany, 2); !tester.HasText(want) {
		t.Errorf("the receipt %q is not on screen", want)
	}

	undoLabel := shell.t().T(i18n.ActionUndo)
	if err := tester.Click(undoLabel); err != nil {
		t.Fatalf("Click(%q): %v", undoLabel, err)
	}
	tester.Frame()

	if ids := listedIDs(shell); !slices.Equal(ids, before) {
		t.Errorf("undo left the list as %v, want the original %v", ids, before)
	}
	// The undo is itself a receipt, and it says what it did rather than repeating the
	// first sentence: the reader can press its own Undo to put them away again.
	if want := shell.t().Tf(i18n.ToastUnarchivedMany, 2); !tester.HasText(want) {
		t.Errorf("the undo's receipt %q is not on screen", want)
	}
}

// TestTheArchiveShortcutMovesTheOpenCardBothWays covers Shift-Cmd-E.
//
// One key, two directions: the card the outline is on leaves the list, and in the archive
// the same key is the way back. The reader never has to remember which view they are in.
func TestTheArchiveShortcutMovesTheOpenCardBothWays(t *testing.T) {
	shell, tester := gridShell(t)
	ctx := context.Background()

	tester.Key(0, ui.KeyDown)
	tester.Frame()
	card := shell.state.Selected
	if card == "" {
		t.Fatal("Down selected no card")
	}

	tester.Key(ui.Super|ui.Shift, ui.KeyE)
	tester.Frame()
	if ids := listedIDs(shell); slices.Contains(ids, card) {
		t.Fatalf("the shortcut left %q in the list: %v", card, ids)
	}

	shell.state.OnlyArchived = true
	shell.Reload(ctx)
	if ids := listedIDs(shell); len(ids) != 1 || ids[0] != card {
		t.Fatalf("the archive holds %v, want [%s]", ids, card)
	}

	tester = ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	tester.Key(0, ui.KeyDown)
	tester.Frame()
	// The one card in the archive is where the outline landed; the same key takes it out.
	tester.Key(ui.Super|ui.Shift, ui.KeyE)
	tester.Frame()
	if ids := listedIDs(shell); len(ids) != 0 {
		t.Errorf("the archive still holds %v after the way back", ids)
	}

	shell.state.OnlyArchived = false
	shell.Reload(ctx)
	if ids := listedIDs(shell); len(ids) != 4 {
		t.Errorf("the library holds %v, want all four bookmarks", ids)
	}
}

// TestTheArchiveRowNamesHowMuchIsInIt covers the count beside the archive row.
func TestTheArchiveRowNamesHowMuchIsInIt(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post", clockBase)
	seed(t, db, "2", "grace", "another post", clockBase.Add(time.Minute))
	shell.Reload(ctx)

	if err := shell.SetArchived(ctx, "1", true); err != nil {
		t.Fatalf("SetArchived: %v", err)
	}
	want := shell.t().Tf(i18n.FilterArchivedWithCount, 1)
	if got := shell.archiveFilterLabel(); got != want {
		t.Errorf("the archive row reads %q, want %q", got, want)
	}

	shell.state.FilterPopoverOpen = true
	tester := ui.NewTester(shell.View, 1180, 900)
	tester.Frame()
	if !tester.HasText(want) {
		t.Errorf("the rendered filter panel does not show %q", want)
	}
}

// TestAnEmptyArchiveSaysItIsEmpty keeps the archive from borrowing the filtered-miss copy.
//
// "No bookmarks match the current filters" blames the conditions for the empty page, and
// the archive is one of the conditions: the advice pointed at the thing the reader had
// just asked for.
func TestAnEmptyArchiveSaysItIsEmpty(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.state.OnlyArchived = true
	shell.Reload(context.Background())

	tester := ui.NewTester(shell.View, 1180, 900)
	tester.Frame()
	if want := shell.t().T(i18n.EmptyArchive); !tester.HasText(want) {
		t.Errorf("the empty archive does not say %q", want)
	}
	if wrong := shell.t().T(i18n.EmptyFilter); tester.HasText(wrong) {
		t.Errorf("the empty archive shows the filtered-miss copy %q", wrong)
	}
}

func listedIDs(shell *Shell) []string {
	ids := make([]string, 0, len(shell.state.Data.Bookmarks))
	for _, row := range shell.state.Data.Bookmarks {
		ids = append(ids, row.TweetID)
	}
	return ids
}
