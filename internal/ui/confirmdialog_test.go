package ui

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/taoquo/trove/internal/i18n"
)

// TestTheConfirmationCardPutsReturnOnCancel pins the card's keyboard contract.
//
// A confirmation exists to survive the reflexive Return, so the button that takes the
// focus must be the one that refuses. The old platform alert put the accent, the focus and
// Enter on its last button too, but its last button was the destructive one in the call
// that drew it first; this test asks the question of the card the shell draws now.
//
// The second half is the other direction: the destructive button still acts when it is
// chosen, so the safe default has not been bought by making the dialog inert.
func TestTheConfirmationCardPutsReturnOnCancel(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "a", "ada", "a post about compilers", clockBase)
	shell.Reload(ctx)
	if _, err := shell.service.AddTags(ctx, []string{"a"}, []string{"Rust"}); err != nil {
		t.Fatalf("AddTags: %v", err)
	}
	shell.Reload(ctx)
	if len(shell.state.Data.Tags) != 1 {
		t.Fatalf("%d tags, want the one just filed", len(shell.state.Data.Tags))
	}
	tag := shell.state.Data.Tags[0]

	tester := ui.NewTester(shell.View, 1180, 900)
	cancel := shell.t().T(i18n.ActionCancel)
	confirm := shell.t().T(i18n.ActionDelete)

	// The dialog opens with the focus already on Cancel.
	shell.state.ConfirmDeleteTag = tag.ID
	tester.Frame()
	if !tester.HasText(shell.t().T(i18n.DeleteTagConfirm)) {
		t.Fatalf("the question is not drawn; texts: %q", tester.Texts())
	}
	if !tester.Focused(cancel) {
		t.Fatalf("the confirmation did not open on %q", cancel)
	}

	// Return refuses: the question closes, and the tag is still there.
	tester.Key(0, ui.KeyEnter)
	tester.Frame()
	if shell.state.ConfirmDeleteTag != "" {
		t.Error("Enter on Cancel left the question on screen")
	}
	if got := len(shell.state.Data.Tags); got != 1 {
		t.Fatalf("%d tags after Return on Cancel, want the tag kept", got)
	}

	// Escape refuses the same way.
	shell.state.ConfirmDeleteTag = tag.ID
	tester.Frame()
	tester.Key(0, ui.KeyEscape)
	tester.Frame()
	if shell.state.ConfirmDeleteTag != "" {
		t.Error("Escape left the question on screen")
	}
	if got := len(shell.state.Data.Tags); got != 1 {
		t.Fatalf("%d tags after Escape, want the tag kept", got)
	}

	// The other button still ends the tag when it is chosen. A step back from Cancel
	// reaches it; the page's own controls that also read "Delete" are not in this dialog.
	shell.state.ConfirmDeleteTag = tag.ID
	tester.Frame()
	tester.Key(ui.Shift, ui.KeyTab)
	if !tester.Focused(confirm) {
		t.Fatalf("a step back from %q did not reach the confirming button", cancel)
	}
	tester.Key(0, ui.KeyEnter)
	tester.Frame()
	if got := len(shell.state.Data.Tags); got != 0 {
		t.Errorf("%d tags after choosing %q, want none", got, confirm)
	}
}

// TestTheBundleQuestionReachesTheScreen keeps the payload path wired.
//
// A bundle import is armed on its own PendingAction, not through the tag library's state
// flags. The dispatcher used to test the *deletion's* confirmation first and return when it
// was not asked, so an import alone drew no question -- and pressing Import did nothing.
// Arming a bundle and asking for the frame is the whole defect.
func TestTheBundleQuestionReachesTheScreen(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "a", "ada", "a post about compilers", clockBase)
	shell.Reload(context.Background())

	body := "Adds 3 bookmarks."
	shell.bundle.Arm(BundleRequest{Path: "/nowhere.trovebundle"}, "Import?", body, "Import", "Cancel")

	tester := ui.NewTester(shell.View, 1180, 900)
	tester.Frame()
	if !tester.HasText(body) {
		t.Fatalf("the bundle question is not on screen; texts: %q", tester.Texts())
	}
	if !tester.Focused(shell.t().T(i18n.ActionCancel)) {
		t.Fatal("the bundle question did not open on Cancel")
	}

	tester.Key(0, ui.KeyEnter)
	tester.Frame()
	if shell.bundle.Confirmation.Asked() {
		t.Error("Return on Cancel left the bundle question on screen")
	}
	if got := len(shell.state.Data.Bookmarks); got != 1 {
		t.Errorf("%d bookmarks after refusing the import, want the library untouched", got)
	}
}
