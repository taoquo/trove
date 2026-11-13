package ui

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/internal/i18n"
)

// TestABatchActionSaysWhatItDid pins the answer to "did that work?".
//
// The batch bar's three writes -- typed tags, the tag picker, move to list -- all reloaded
// the page and put the bar back exactly as it was: the same count, the same controls, no
// sentence. On the all-bookmarks scope a move changes nothing visible at all (cards do not
// carry their list), so a press looked like a miss and invited a second one. Clearing the
// selection after a bulk action is one acknowledgement; this app keeps the reader's selection
// and instead says what happened.
func TestABatchActionSaysWhatItDid(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "first", clockBase)
	seed(t, db, "2", "grace", "second", clockBase)
	shell.Reload(ctx)

	shell.state.SelectedIDs = map[string]bool{"1": true, "2": true}
	shell.state.TagInput = "rust"
	if _, err := shell.AddTagsToSelection(ctx); err != nil {
		t.Fatalf("AddTagsToSelection: %v", err)
	}

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	want := shell.t().Tf(i18n.BatchTagged, 2)
	if !tester.HasText(want) {
		t.Errorf("tagging two bookmarks said nothing; want %q, texts: %q", want, tester.Texts())
	}
	if got := shell.BatchSelection(); len(got) != 0 {
		t.Errorf("the selection survived the write: %v", got)
	}
}

// TestAMoveToListNamesTheDestination keeps the move's sentence honest: a count and the
// list's own name, because "moved 2 bookmarks" without the destination leaves the reader
// guessing which list they went to.
func TestAMoveToListNamesTheDestination(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "first", clockBase)
	seed(t, db, "2", "grace", "second", clockBase)
	if _, err := shell.service.CreateList(ctx, "Work"); err != nil {
		t.Fatalf("CreateList: %v", err)
	}
	shell.Reload(ctx)

	shell.state.SelectedIDs = map[string]bool{"1": true, "2": true}
	shell.state.MoveTargetLabel = "Work"
	if err := shell.MoveSelectionToList(ctx); err != nil {
		t.Fatalf("MoveSelectionToList: %v", err)
	}

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	want := shell.t().Tf(i18n.BatchMoved, 2, "Work")
	if !tester.HasText(want) {
		t.Errorf("moving two bookmarks said nothing; want %q, texts: %q", want, tester.Texts())
	}
	if got := shell.BatchSelection(); len(got) != 0 {
		t.Errorf("the selection survived the move: %v", got)
	}
}

// TestTheBatchBarSaysWhatEachPickerChooses pins the two selectors' resting state.
//
// Both pickers bind a value that starts empty, so a bare trigger draws nothing and a reader
// cannot tell which one tags and which one moves. Each picker carries a "Choose tag" option as
// its empty value, which is exactly the resting text a picker with no choice should show.
func TestTheBatchBarSaysWhatEachPickerChooses(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "first", clockBase)
	seed(t, db, "2", "grace", "second", clockBase)
	if _, err := shell.service.CreateList(ctx, "Work"); err != nil {
		t.Fatalf("CreateList: %v", err)
	}
	if _, err := shell.service.AddTags(ctx, []string{"1"}, []string{"rust"}); err != nil {
		t.Fatalf("seed the tag: %v", err)
	}
	shell.Reload(ctx)

	shell.state.SelectedIDs = map[string]bool{"1": true, "2": true}
	tester := ui.NewTester(shell.View, 1400, 880)
	tester.Frame()

	for _, want := range []string{shell.batchChooseTagLabel(), shell.batchChooseListLabel()} {
		if !tester.HasText(want) {
			t.Errorf("a batch picker drew no resting text; want %q, texts: %q", want, tester.Texts())
		}
	}
}

// TestAPressOnTheUnchosenPickerDoesNothing keeps the placeholder from acting like a choice.
//
// Apply is disabled until a tag is chosen. Without that state the list picker is worse than a
// no-op: an empty label falls through to the first list (listIDForLabel), so pressing "Move to
// list" without choosing moves the selection into Inbox.
func TestAPressOnTheUnchosenPickerDoesNothing(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "first", clockBase)
	if _, err := shell.service.CreateList(ctx, "Work"); err != nil {
		t.Fatalf("CreateList: %v", err)
	}
	if _, err := shell.service.AddTags(ctx, []string{"1"}, []string{"rust"}); err != nil {
		t.Fatalf("seed the tag: %v", err)
	}
	shell.Reload(ctx)

	shell.state.SelectedIDs = map[string]bool{"1": true}
	if err := shell.ApplyBulkTag(ctx); err != nil {
		t.Fatalf("ApplyBulkTag: %v", err)
	}
	if got := shell.BatchSelection(); len(got) != 1 {
		t.Errorf("an unchosen tag picker consumed the selection: %v", got)
	}

	if err := shell.MoveSelectionToList(ctx); err != nil {
		t.Fatalf("MoveSelectionToList: %v", err)
	}
	if got := shell.BatchSelection(); len(got) != 1 {
		t.Errorf("an unchosen list picker moved the selection into %q", shell.state.Data.Lists[0].Name)
	}
}

// TestTheTagPickerSaysWhatItDid covers the tag picker's path beside the typed one.
func TestTheTagPickerSaysWhatItDid(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "first", clockBase)
	if _, err := shell.service.AddTags(ctx, []string{"1"}, []string{"rust"}); err != nil {
		t.Fatalf("seed the tag: %v", err)
	}
	shell.Reload(ctx)

	shell.state.SelectedIDs = map[string]bool{"1": true}
	shell.state.BulkTagLabel = "rust"
	if err := shell.ApplyBulkTag(ctx); err != nil {
		t.Fatalf("ApplyBulkTag: %v", err)
	}

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	want := shell.t().T(i18n.BatchTaggedOne)
	if !tester.HasText(want) {
		t.Errorf("the picker said nothing; want %q, texts: %q", want, tester.Texts())
	}
	if got := shell.BatchSelection(); len(got) != 0 {
		t.Errorf("the selection survived the write: %v", got)
	}
}
