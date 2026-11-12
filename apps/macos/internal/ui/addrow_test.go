package ui

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/x-master/apps/macos/internal/i18n"
)

// TestTheAddRowHasAWayToCloseItself covers a row that took the top of the feed and had no exit.
//
// The row is this build's own surface -- the extension saves from x.com's page -- and it opens
// from the menu bar's "Add bookmark" command. Nothing in the window closed it, so a reader who
// opened it by mistake, or changed their mind with the link half-typed, had to go back to the
// menu for the same command. Every other openable row here is dismissable where it stands.
func TestTheAddRowHasAWayToCloseItself(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "post 1", clockBase)
	shell.SetLocale(i18n.EN, i18n.EN)
	shell.Reload(context.Background())

	shell.ToggleAddForm()
	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	if _, ok := tester.Find(shell.t().T(i18n.AddLinkPlaceholder)); !ok {
		t.Fatalf("the add row is not open; texts: %q", tester.Texts())
	}

	cancel := shell.t().T(i18n.ActionCancel)
	if _, ok := tester.Find(cancel); !ok {
		t.Fatalf("the add row has no control that closes it; texts: %q", tester.Texts())
	}
	if err := tester.Click(cancel); err != nil {
		t.Fatalf("cannot press the row's close control: %v", err)
	}
	tester.Frame()

	if shell.State().AddOpen {
		t.Error("the row is still open after its own close control was pressed")
	}
	if _, ok := tester.Find(shell.t().T(i18n.AddLinkPlaceholder)); ok {
		t.Error("the row's fields are still drawn after it was closed")
	}
}
