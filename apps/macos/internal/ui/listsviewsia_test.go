package ui

import (
	"context"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/apps/macos/internal/i18n"
	"github.com/taoquo/trove/apps/macos/internal/storage/sqlite"
)

// TestTheDefaultSavedViewNameSpeaksTheUiLanguage pins the fallback name's language.
//
// Saving a view with an empty name is allowed -- the source writes DEFAULT_SAVED_VIEW_NAME
// (OptionsApp.tsx:556) -- but this build wrote the literal English "Saved view" into the store
// instead of the key that already existed (i18n.DefaultSavedViewName). The row then read
// "Saved view" on a Chinese page, and the key was never drawn anywhere.
func TestTheDefaultSavedViewNameSpeaksTheUiLanguage(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	shell.SetLocale("zh-CN", "zh-CN")

	shell.state.SavedViewName = ""
	shell.SaveCurrentView(ctx)

	stored, err := sqlite.NewSavedViewRepository(db).List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(stored) != 1 {
		t.Fatalf("the store holds %d views", len(stored))
	}

	want := shell.t().T(i18n.DefaultSavedViewName)
	if stored[0].Name != want {
		t.Errorf("a nameless view is stored as %q; want %q", stored[0].Name, want)
	}
}

// TestApplyingASavedViewTakesYouToTheResults pins the end of the apply path.
//
// The saved views live in settings (round 472 moved them there because the sidebar follows the
// reference picture's two tabs). Pressing a row's play button restored the filters and left the
// settings page covering the results, so the one visible effect was that the page behind the
// sheet had changed -- the view looked like it had done nothing. OpenList already leaves the
// sheet; apply is the same promise.
func TestApplyingASavedViewTakesYouToTheResults(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "first", clockBase)

	shell.state.Query = "ada"
	shell.state.SavedViewName = "Ada only"
	shell.SaveCurrentView(ctx)
	shell.Reload(ctx)
	views := savedViewsOf(t, shell)
	if len(views) != 1 {
		t.Fatalf("the shell lists %d views", len(views))
	}
	view := views[0]
	shell.OpenSettings()
	shell.state.SettingsSection = SettingsSectionListsViews

	shell.ApplySavedView(ctx, view)

	if shell.state.SettingsOpen {
		t.Error("the settings page stayed over the results a saved view restored")
	}
	if shell.state.Query != "ada" {
		t.Errorf("the query is %q; want %q", shell.state.Query, "ada")
	}
}

// TestEachRowActionNamesItsOwnRow keeps the two management cards readable by ear.
//
// Every list row carries three icon buttons and every saved view two, and all of them shared one
// generic name ("Open bookmarks", "Rename list", "Delete", "Apply view"). A screen reader walking
// the card heard the same four words over and over with nothing saying which list or view each
// one acts on. The source's delete button carries the view's name (WorkspaceSidebar.tsx:88:
// `${copy.deleteSavedView} ${savedView.name}`).
func TestEachRowActionNamesItsOwnRow(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "first", clockBase)
	if _, err := shell.service.CreateList(ctx, "Work"); err != nil {
		t.Fatalf("CreateList: %v", err)
	}
	shell.state.Query = "ada"
	shell.state.SavedViewName = "Recent"
	shell.SaveCurrentView(ctx)
	shell.Reload(ctx)
	shell.OpenSettings()
	shell.state.SettingsSection = SettingsSectionListsViews

	tester := ui.NewTester(shell.View, 1200, 2000)
	tester.Frame()

	for _, want := range []string{
		shell.t().Tf(i18n.ActionOpenNamed, "Work"),
		shell.t().Tf(i18n.ActionRenameNamed, "Work"),
		shell.t().Tf(i18n.ActionDeleteNamed, "Work"),
		shell.t().Tf(i18n.SavedViewApplyNamed, "Recent"),
		shell.t().Tf(i18n.ActionDeleteNamed, "Recent"),
	} {
		if _, ok := tester.Find(want); !ok {
			t.Errorf("no control is named %q; texts: %q", want, tester.Texts())
		}
	}
}

// TestTheListsAndViewsHaveTheirOwnSettingsTab pins the place, not just the contents.
//
// Both cards used to close the 数据安全 page, under "backups and restores stay on this Mac" --
// a sentence that describes neither, on a tab a reader looking for a saved view has no reason
// to open. The extension puts them in the sidebar; round 472 moved management into settings for
// the reference picture's sake, which left the question of which page, and this is the answer.
func TestTheListsAndViewsHaveTheirOwnSettingsTab(t *testing.T) {
	shell, _ := newShell(t)
	shell.Reload(context.Background())
	shell.OpenSettings()

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	if !tester.HasText(strings.ToUpper(shell.t().T(i18n.SettingsSectionListsViews))) {
		t.Errorf("the tab row has no %q tab; texts: %q",
			shell.t().T(i18n.SettingsSectionListsViews), tester.Texts())
	}

	shell.state.SettingsSection = SettingsSectionDataSafe
	tester.Frame()
	if tester.HasText(shell.t().T(i18n.SidebarLists)) {
		t.Errorf("the data-safety page still draws the lists card; texts: %q", tester.Texts())
	}
}
