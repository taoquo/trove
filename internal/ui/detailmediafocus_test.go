package ui

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestEveryPictureInTheDetailIsReachableFromTheKeyboard covers the detail's media.
//
// Each picture is the lightbox's trigger, but the box was pointer-only: no Tab stop, no
// Enter, and no accessible name. A reader who works from the keyboard could open the detail
// and reach nothing in the post it exists to show.
func TestEveryPictureInTheDetailIsReachableFromTheKeyboard(t *testing.T) {
	shell, db := newShell(t)
	seedMediaBookmark(t, db, "1", "photo", "video")
	shell.Reload(context.Background())
	shell.state.Selected = "1"
	shell.OpenSelectedDetail()

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	// Walk the dialog's focus order until the picture takes it. The exact number of stops
	// is not the contract; reaching it at all is.
	reached := false
	for i := 0; i < 40 && !reached; i++ {
		tester.Key(0, ui.KeyTab)
		tester.Frame()
		reached = tester.Focused(DetailMediaTriggerLabel)
	}
	if !reached {
		t.Fatalf("Tab never reached the post's pictures; texts: %q", tester.Texts())
	}

	tester.Key(0, ui.KeyEnter)
	tester.Frame()
	if !shell.LightboxOpen() {
		t.Error("Enter on the focused picture did not open the preview")
	}
}
