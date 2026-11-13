package ui

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/internal/i18n"
)

// tabUntil focuses the next control on each press until want is focused.
//
// It returns whether the control was reached. The number of stops is not the contract;
// being reachable at all is.
func tabUntil(tester *ui.Tester, want string) bool {
	for i := 0; i < 40; i++ {
		tester.Key(0, ui.KeyTab)
		tester.Frame()
		if tester.Focused(want) {
			return true
		}
	}
	return false
}

// TestTheTagLibrariesRowControlsTakeTheKeyboard covers every target in a library row.
//
// The tag's name, the parent cell and the row's rename/delete icons were clickable boxes
// with no focus stop: a keyboard reader could see them and never use them. They now carry
// a Label and Focusable(), like the table's own action icons.
func TestTheTagLibrariesRowControlsTakeTheKeyboard(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	shell.SetLocale(i18n.EN, i18n.EN)
	seedTagOnly(t, db, "a", "rust")
	shell.ReloadTagLibrary(ctx)
	shell.ToggleTagMode(ctx)

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	for _, want := range []string{
		"rust",
		shell.t().T(i18n.TagParentNone),
		shell.t().Tf(i18n.TagRenameFor, "rust"),
		shell.t().Tf(i18n.TagDeleteFor, "rust"),
	} {
		if !tabUntil(tester, want) {
			t.Errorf("Tab never reached %q; texts: %q", want, tester.Texts())
		}
	}
}

// TestTheDetailTagPillCanBeRemovedFromTheKeyboard covers the detail's tag row.
func TestTheDetailTagPillCanBeRemovedFromTheKeyboard(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	shell.SetLocale(i18n.EN, i18n.EN)
	seed(t, db, "1", "ada", "a post", clockBase)
	seedTagOnly(t, db, "a", "rust")
	if _, err := shell.service.AddTags(ctx, []string{"1"}, []string{"rust"}); err != nil {
		t.Fatalf("attach tag: %v", err)
	}
	shell.Reload(ctx)
	shell.state.Selected = "1"
	shell.OpenSelectedDetail()

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	// The pill names the tag it removes, so that -- not the bare verb -- is what the focus
	// stop answers to.
	if !tabUntil(tester, shell.t().Tf(i18n.ActionRemoveNamed, "rust")) {
		t.Fatalf("Tab never reached the tag's remove control; texts: %q", tester.Texts())
	}
	tester.Key(0, ui.KeyEnter)
	tester.Frame()

	rows := shell.State().Data.Bookmarks
	if len(rows) != 1 {
		t.Fatalf("the library holds %d rows, want the one bookmark", len(rows))
	}
	if len(rows[0].Tags) != 0 {
		t.Errorf("Enter on the focused remove left the tag attached: %+v", rows[0].Tags)
	}
}

// TestTheSidebarsChromeTakesTheKeyboard covers the search toggle and the locale chip.
//
// Both were pointer-only, and the locale chip had no readable name at all: the one control
// that changes every word on screen could not be reached or announced.
func TestTheSidebarsChromeTakesTheKeyboard(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	shell.SetLocale(i18n.EN, i18n.EN)
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(ctx)

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	if !tabUntil(tester, shell.t().T(i18n.SearchLabel)) {
		t.Fatalf("Tab never reached the sidebar's search toggle; texts: %q", tester.Texts())
	}
	tester.Key(0, ui.KeyEnter)
	tester.Frame()
	if !shell.State().SidebarSearchOpen {
		t.Error("Enter on the focused search toggle did not open the field")
	}

	if !tabUntil(tester, shell.t().T(i18n.LanguageSwitch)) {
		t.Fatalf("Tab never reached the locale chip; texts: %q", tester.Texts())
	}
	tester.Key(0, ui.KeyEnter)
	tester.Frame()
	if got := shell.State().Locale; got != i18n.ZHCN {
		t.Errorf("Enter on the locale chip left the locale at %q", got)
	}
}
