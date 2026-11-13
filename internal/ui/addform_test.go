package ui

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/internal/storage/sqlite"
)

// TestTheAddFormNamesItsFieldsAndAddsOnEnter covers the one form this build adds
// (Cmd-N, because a standalone manager has no x.com page to save from).
//
// It drew two blank boxes and a button: nothing said which box took the link and which
// took the text, the first field did not take the cursor, and Enter -- the key every other
// field in this shell submits with -- did nothing.
func TestTheAddFormNamesItsFieldsAndAddsOnEnter(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	shell.SetLocale("en", "en")
	shell.Reload(ctx)
	shell.ToggleAddForm()

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	for _, want := range []string{"Paste a link to the post", "Post text (optional)"} {
		if !tester.HasText(want) {
			t.Errorf("the add form is missing the placeholder %q; texts: %q", want, tester.Texts())
		}
	}

	// Paste into the focused field and press Enter, as the other fields in the shell work.
	tester.Type("https://x.com/ada/status/123")
	tester.Key(0, ui.KeyEnter)
	tester.Frame()

	if _, found, err := sqlite.NewBookmarkRepository(db).Get(ctx, "123"); err != nil || !found {
		t.Fatalf("Enter in the add form did not add the post (found %v, err %v)", found, err)
	}
}
