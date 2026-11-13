package ui

import (
	"context"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/internal/i18n"
)

// TestToastExpiresAfterItsOwnTimeout covers the auto-dismiss.
//
// The message clears on a 2200ms timer. This shell stores the deadline and
// checks it as the overlay is built instead of repainting on a timer, so the behaviour
// under test is the deadline, not the scheduling.
func TestToastExpiresAfterItsOwnTimeout(t *testing.T) {
	shell, _ := newShell(t)

	if got := shell.ToastText(); got != "" {
		t.Errorf("a fresh shell shows a toast: %q", got)
	}

	shell.ShowToast("No link available")
	if got := shell.ToastText(); got != "No link available" {
		t.Errorf("the toast reads %q", got)
	}

	// Still there just before the deadline.
	shell.state.ToastUntil = time.Now().Add(time.Millisecond)
	if got := shell.ToastText(); got == "" {
		t.Error("a toast must survive until its deadline")
	}

	// Gone after it, without anything else happening.
	shell.state.ToastUntil = time.Now().Add(-time.Millisecond)
	if got := shell.ToastText(); got != "" {
		t.Errorf("an expired toast is still shown: %q", got)
	}
}

// TestCopyWithoutALinkShowsTheToast covers the only toast trigger a user can
// reach without a clipboard.
func TestCopyWithoutALinkShowsTheToast(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(context.Background())

	// A selected bookmark with no URL.
	shell.state.Selected = "1"
	shell.state.Data.Bookmarks[0].TweetURL = ""

	var captured string
	shell.CopySourceLink(&clipboardContext{written: &captured})

	if got := shell.ToastText(); got != shell.t().T(i18n.ToastNoLink) {
		t.Errorf("toast = %q, want the no-link message", got)
	}
	if captured != "" {
		t.Errorf("nothing must be written to the clipboard, got %q", captured)
	}
}

// TestCopyWritesTheLinkAndConfirms covers the success path.
func TestCopyWritesTheLinkAndConfirms(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(context.Background())
	shell.state.Selected = "1"

	var captured string
	shell.CopySourceLink(&clipboardContext{written: &captured})

	want := shell.state.Data.Bookmarks[0].TweetURL
	if captured != want {
		t.Errorf("clipboard = %q, want %q", captured, want)
	}
	if got := shell.ToastText(); got != shell.t().T(i18n.ToastDone) {
		t.Errorf("toast = %q, want the confirmation", got)
	}
}

// TestToastAppearsInTheOverlay keeps the message from being state nobody draws.
func TestToastAppearsInTheOverlay(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(context.Background())
	shell.state.Selected = "1"
	shell.OpenSelectedDetail()
	shell.ShowToast("No link available")

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	if !tester.HasText("No link available") {
		t.Errorf("the toast is not drawn; texts: %q", tester.Texts())
	}
}

// clipboardContext captures what the copy action writes.
type clipboardContext struct{ written *string }

func (c *clipboardContext) WriteClipboard(s string) { *c.written = s }
