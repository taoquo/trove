package ui

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestATagNameThatDoesNotFitEndsInOneLine covers the sidebar's tag names.
//
// The grouped row is `white-space: nowrap` with `overflow: hidden; text-overflow:
// ellipsis` on the name, so a name that does not fit is cut, not wrapped.
// This build let the text take a second line, which moved the count to the first line and
// made one row twice as tall as its neighbours.
func TestATagNameThatDoesNotFitEndsInOneLine(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seedTagOnly(t, db, "long", "artificial intelligence")
	seedTagOnly(t, db, "short", "ai agent")
	shell.Reload(ctx)
	shell.ReloadTagLibrary(ctx)

	// Narrow enough that the long name has to give; the sidebar keeps its own width and
	// the window only decides how much room the results pane has.
	tester := ui.NewTester(shell.View, 900, 760)
	tester.Frame()

	long, ok := tester.Find("artificial intelligence")
	if !ok {
		t.Fatalf("the long tag is not drawn; texts: %q", tester.Texts())
	}
	short, ok := tester.Find("ai agent")
	if !ok {
		t.Fatalf("the short tag is not drawn; texts: %q", tester.Texts())
	}
	if long.H > short.H {
		t.Errorf("the long tag is %.1f high and the short one %.1f: the name wrapped instead of being cut",
			long.H, short.H)
	}
}
