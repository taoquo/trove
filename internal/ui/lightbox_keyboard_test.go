package ui

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/internal/i18n"
)

// openLightboxOverDetail opens the detail on one bookmark with two media and then the
// lightbox, which is the way the shell reaches it: the reader clicks a picture in the
// detail. The tester's first frame builds the detail and the lightbox together.
func openLightboxOverDetail(t *testing.T) (*Shell, *ui.Tester) {
	t.Helper()

	shell, db := newShell(t)
	seedMediaBookmark(t, db, "1", "photo", "video")
	shell.Reload(context.Background())
	shell.state.Selected = "1"
	shell.OpenSelectedDetail()
	shell.OpenLightbox(0)
	if !shell.LightboxOpen() || !shell.State().DetailOpen {
		t.Fatalf("precondition: lightbox %v, detail %v", shell.LightboxOpen(), shell.State().DetailOpen)
	}

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	return shell, tester
}

// TestLightboxEscapeClosesTheLightboxBeforeTheDetail is the reported defect: Escape with
// the preview open closed the detail underneath it.
//
// The detail is a DialogBase, whose backdrop registers Escape as an overlay handler. The
// lightbox used to register Escape as a window shortcut, and MyGo lets the overlay win, so
// one press took the detail away and left the preview floating over the list. Escape must
// close the top layer only: the lightbox first, the detail on the next press.
func TestLightboxEscapeClosesTheLightboxBeforeTheDetail(t *testing.T) {
	shell, tester := openLightboxOverDetail(t)

	tester.Key(0, ui.KeyEscape)
	tester.Frame()

	if shell.LightboxOpen() {
		t.Error("the first Escape did not close the lightbox")
	}
	if !shell.State().DetailOpen {
		t.Error("the first Escape closed the detail under the lightbox; it must close only the lightbox")
	}

	tester.Key(0, ui.KeyEscape)
	tester.Frame()

	if shell.State().DetailOpen {
		t.Error("the second Escape did not close the detail")
	}
}

// TestLightboxOwnsTheKeyboard covers the preview's keyboard surface.
//
// The lightbox traps the focus, focuses its close control as it opens, moves with the arrows and
// lets Enter press the focused control. Its controls are real controls in a modal focus scope,
// not pointer-only boxes, and the arrows do something.
func TestLightboxOwnsTheKeyboard(t *testing.T) {
	shell, tester := openLightboxOverDetail(t)

	closeLabel := shell.t().T(i18n.MediaClose)
	prevLabel := shell.t().T(i18n.MediaPrevious)

	if !tester.Focused(closeLabel) {
		t.Errorf("the lightbox's close control does not take the focus as it opens; texts: %q", tester.Texts())
	}

	// Right and Left step the preview, and it wraps.
	tester.Key(0, ui.KeyRight)
	tester.Frame()
	if !tester.HasText("2 / 2") {
		t.Errorf("Right did not show the second item; texts: %q", tester.Texts())
	}
	tester.Key(0, ui.KeyLeft)
	tester.Frame()
	if !tester.HasText("1 / 2") {
		t.Errorf("Left did not return to the first item; texts: %q", tester.Texts())
	}

	// Tab stays in the lightbox: from the close control to the previous-arrow control.
	tester.Key(0, ui.KeyTab)
	tester.Frame()
	if !tester.Focused(prevLabel) {
		t.Errorf("Tab left the lightbox or did not move within it; texts: %q", tester.Texts())
	}

	// Enter presses the focused preview control rather than a hidden detail control: the
	// previous arrow wraps from the first item to the second, and the detail stays open.
	tester.Key(0, ui.KeyEnter)
	tester.Frame()
	if !shell.State().DetailOpen {
		t.Error("Enter operated a control of the hidden detail")
	}
	if !shell.LightboxOpen() {
		t.Error("Enter closed the lightbox instead of pressing its focused control")
	}
	if got, _ := shell.lightboxIndex(); got != 1 {
		t.Errorf("Enter on the previous arrow gave index %d, want 1", got)
	}
}
