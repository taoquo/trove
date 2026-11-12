package ui

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/x-master/apps/macos/internal/i18n"
)

// TestTheSidebarAnswersEveryClick walks the column's controls by their visible names.
//
// Round 35 found two controls that looked pressable and were not, so this round takes the
// left column in the same order a pointer meets it: the facet tabs, the search toggle, the
// create row, the table glyph, a tag row, the footer's theme and settings controls, and
// the locale chip. Each click has to leave the state its name promises. Round 36.
func TestTheSidebarAnswersEveryClick(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	shell.SetLocale(i18n.EN, i18n.EN)
	seed(t, db, "1", "ada", "a post", clockBase)
	seedTagOnly(t, db, "t-rust", "rust")
	if _, err := shell.service.AddTags(ctx, []string{"1"}, []string{"rust"}); err != nil {
		t.Fatalf("attach tag: %v", err)
	}
	shell.Reload(ctx)

	tester := ui.NewTester(shell.View, 1180, 760)
	click := func(label string) {
		t.Helper()
		tester.Frame()
		if err := tester.Click(label); err != nil {
			t.Fatalf("clicking %q: %v (texts: %q)", label, err, tester.Texts())
		}
		tester.Frame()
	}

	// The facet tabs switch the list in the same slot.
	click(shell.t().T(i18n.AuthorsTitle))
	if got, want := shell.sidebarTab(), sidebarTabAuthors; got != want {
		t.Errorf("clicking %q showed the %q tab", shell.t().T(i18n.AuthorsTitle), got)
	}
	// An author row narrows the list the same way a tag row does.
	author := shell.authorLabel("ada")
	click(author)
	if len(shell.state.ActiveAuthors) != 1 || shell.state.ActiveAuthors[0] != "ada" {
		t.Errorf("clicking the author row left the filter %v", shell.state.ActiveAuthors)
	}

	click(shell.t().T(i18n.SidebarTags))
	if got, want := shell.sidebarTab(), sidebarTabTags; got != want {
		t.Errorf("clicking %q did not come back to the tags tab", shell.t().T(i18n.SidebarTags))
	}

	// The magnifier opens the column's own field.
	click(shell.t().T(i18n.SearchLabel))
	if !shell.state.SidebarSearchOpen {
		t.Errorf("clicking %q did not open the sidebar search", shell.t().T(i18n.SearchLabel))
	}

	// "+" opens the inline draft row.
	click(shell.t().T(i18n.CreateTagSymbol))
	if !shell.state.CreatingTag {
		t.Errorf("clicking %q did not open the tag draft", shell.t().T(i18n.CreateTagSymbol))
	}

	// A tag row narrows the list, and the condition appears in the toolbar.
	click("rust")
	if len(shell.state.ActiveTagIDs) != 1 || shell.state.ActiveTagIDs[0] != "t-rust" {
		t.Errorf("clicking the tag row left the filter %v", shell.state.ActiveTagIDs)
	}

	// The table glyph opens the tag library.
	click(shell.t().T(i18n.TagManageLabel))
	if !shell.state.TagMode {
		t.Errorf("clicking %q did not open the tag library", shell.t().T(i18n.TagManageLabel))
	}
	click(shell.t().T(i18n.TagExitManagement))
	if shell.state.TagMode {
		t.Errorf("clicking %q did not come back to the workspace", shell.t().T(i18n.TagExitManagement))
	}

	// The footer's theme control cycles the appearance.
	themeLabel := shell.t().T(i18n.ThemeLabel) + ": " + shell.appearanceLabel()
	before := shell.state.Appearance
	click(themeLabel)
	if shell.state.Appearance == before {
		t.Errorf("clicking %q did not change the appearance", themeLabel)
	}

	// The gear opens settings.
	click(shell.t().T(i18n.SettingsTitle))
	if !shell.state.SettingsOpen {
		t.Errorf("clicking %q did not open settings", shell.t().T(i18n.SettingsTitle))
	}

	// The locale chip switches the catalog. Last, because it renames every other control.
	click(shell.t().T(i18n.LanguageSwitch))
	if got, want := shell.state.Locale, i18n.ZHCN; got != want {
		t.Errorf("the locale chip left the language at %q, want %q", got, want)
	}
}
