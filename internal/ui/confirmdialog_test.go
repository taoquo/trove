package ui

import (
	"context"
	"image/color"
	"testing"
	"time"

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

// TestTheDestructiveAnswerFillsUnderThePointer pins the confirmation card's hover state.
//
// At rest the confirming action is the danger ink on the danger-soft ground: readable, but
// plainly a warning rather than the path of least resistance. Under the pointer the ground
// fills with the danger ink and the label turns to the accent's foreground, so the press the
// reader is about to make is the one they see. The ground is sampled from the frame because
// the wiring -- which token reaches this one button -- is what the palette change touched.
func TestTheDestructiveAnswerFillsUnderThePointer(t *testing.T) {
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

	tester := ui.NewTester(shell.View, 1180, 900)
	shell.state.ConfirmDeleteTag = shell.state.Data.Tags[0].ID
	tester.Frame()

	cancelRect, found := tester.Find(shell.t().T(i18n.ActionCancel))
	if !found {
		t.Fatalf("the dialog's Cancel is not drawn; texts: %q", tester.Texts())
	}

	theme := FolioTheme(shell.state.Appearance, shell.state.SystemDark)
	rest := dangerSoftFor(theme.Dark)
	same := func(c color.RGBA, want ui.Color) bool {
		return c.R == want.R && c.G == want.G && c.B == want.B
	}

	// The confirming button is the sibling left of Cancel. Find(confirm) cannot locate it:
	// the dialog panel answers to the same name as the button's words, and comes first. The
	// ground is found by its colour instead -- the run of danger-soft pixels on the button
	// row, left of Cancel, sampled two points in from its left edge where only padding shows.
	img := tester.Image()
	scale := float32(img.Rect.Dx()) / 1180
	row := int((cancelRect.Y + cancelRect.H/2) * scale)
	sample := -1
	for x := int((cancelRect.X - 120) * scale); x+3 < int(cancelRect.X*scale); x++ {
		if same(img.RGBAAt(x, row), rest) &&
			same(img.RGBAAt(x+1, row), rest) &&
			same(img.RGBAAt(x+2, row), rest) &&
			same(img.RGBAAt(x+3, row), rest) {
			sample = x + 2
			break
		}
	}
	if sample < 0 {
		t.Fatalf("no danger-soft ground on the confirming button's row; texts: %q", tester.Texts())
	}

	tester.Move(float32(sample)/scale, float32(row)/scale)
	time.Sleep(2 * time.Duration(MotionFastMs) * time.Millisecond)
	tester.Frame()
	if got := tester.Image().RGBAAt(sample, row); !same(got, theme.Danger) {
		t.Errorf("under the pointer the confirming action is %v, want the danger fill %v",
			got, theme.Danger)
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
