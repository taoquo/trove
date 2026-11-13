package ui

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestTheScrollEndsTrayJumpsToTheEnd builds the real page and presses the tray's lower
// control: the list must land at its own end, which is what the capsule is for.
func TestTheScrollEndsTrayJumpsToTheEnd(t *testing.T) {
	shell, db := newShell(t)
	for i := 0; i < 24; i++ {
		seed(t, db, fmt.Sprintf("%d", i), "ada", fmt.Sprintf("post %d", i), clockBase)
	}
	shell.Reload(context.Background())

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	if shell.resultsScroll.MaxY <= 0 {
		t.Fatalf("the list does not scroll (MaxY=%v), so this cannot test the tray",
			shell.resultsScroll.MaxY)
	}
	// A screen down, where the tray shows.
	shell.resultsScroll.Y = shell.resultsScroll.MaxY / 2
	tester.Frame()

	// The tray's lower control: its box sits ViewEndsInset from the page's right edge and
	// ViewEndsBottomInset above the window's, and the button is the lower one in the tray.
	x := float32(1180 - ViewEndsInset - ViewEndsWidth/2)
	y := float32(760 - ViewEndsBottomInset - ViewEndsPad - ViewEndsButtonHeight/2)
	tester.ClickAt(x, y)

	// The jump is eased over MotionSlow, and the tester's frames advance with the clock, so
	// the list lands at its end rather than half way; 3 seconds is ten times the animation.
	for i := 0; i < 300 && shell.resultsScroll.Scrolling(); i++ {
		time.Sleep(10 * time.Millisecond)
		tester.Frame()
	}
	tester.Frame()

	if shell.resultsScroll.Y != shell.resultsScroll.MaxY {
		t.Errorf("pressing the tray's lower control left the list at %v of %v",
			shell.resultsScroll.Y, shell.resultsScroll.MaxY)
	}
}
