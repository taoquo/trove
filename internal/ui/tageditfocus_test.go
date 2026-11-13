package ui

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/internal/i18n"
)

// TestTheOpenTagEditorGivesTheCursorUp covers the tag library's in-place editors.
//
// The name, alias and theme editors each called Focus() inside the view, so every frame put
// the cursor back in the open field. Tab could not leave it: the next frame took the keyboard
// back, and a keyboard reader never reached the row's other controls. Focus belongs to the
// frame that opens the editor, not to every frame that redraws it.
func TestTheOpenTagEditorGivesTheCursorUp(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seedTagOnly(t, db, "a", "rust")
	shell.ReloadTagLibrary(ctx)
	shell.ToggleTagMode(ctx)

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	if err := tester.Click(shell.t().Tf(i18n.TagRenameFor, "rust")); err != nil {
		t.Fatalf("opening the rename: %v", err)
	}
	tester.Frame()

	// The editor opens with the cursor in it, so typing edits the tag.
	tester.Type("!")
	tester.Frame()
	if got := shell.State().TagNameEdit; got != "rust!" {
		t.Fatalf("the editor did not start with the cursor: draft %q", got)
	}

	// Tab is how the keyboard reaches the rest of the page. The field must not take the
	// cursor back on the frame after.
	tester.Key(0, ui.KeyTab)
	tester.Frame()
	before := shell.State().TagNameEdit
	tester.Type("x")
	tester.Frame()
	if got := shell.State().TagNameEdit; got != before {
		t.Errorf("the open field took the cursor back after Tab: draft %q, want %q", got, before)
	}
}
