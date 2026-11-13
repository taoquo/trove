package ui

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestTheSearchFocusIsTakenOnce covers Cmd-F on the results page.
//
// The request lived in a state flag that only a reload cleared, and Cmd-F does not
// reload. So after one Cmd-F the field took the cursor back on every frame: Tab could
// not leave it, and the next keystroke went into the search box even after the reader
// had moved on.
func TestTheSearchFocusIsTakenOnce(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(ctx)

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	// Cmd-F asks for the search field.
	shell.FocusSearch()
	tester.Frame()

	// Tab moves the keyboard elsewhere; the field must let it go.
	tester.Key(0, ui.KeyTab)
	tester.Frame()
	tester.Type("x")
	tester.Frame()

	if got := shell.State().Query; got != "" {
		t.Errorf("the search field took the cursor back after Tab: query %q", got)
	}
}
