package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/apps/macos/internal/domain"
	"github.com/taoquo/trove/apps/macos/internal/i18n"
	"github.com/taoquo/trove/apps/macos/internal/idgen"
	"github.com/taoquo/trove/apps/macos/internal/ports"
	"github.com/taoquo/trove/apps/macos/internal/storage/sqlite"
	appui "github.com/taoquo/trove/apps/macos/internal/ui"
	"github.com/taoquo/trove/apps/macos/internal/usecase/workspace"
)

// menuShell is the composition root's wiring without a window, which is what buildMenu needs.
func menuShell(t *testing.T) (*appui.Shell, *sqlite.DB) {
	t.Helper()

	db, err := sqlite.Open(filepath.Join(t.TempDir(), "menu.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	repos := ports.Repositories{
		Bookmarks:   sqlite.NewBookmarkRepository(db),
		Tags:        sqlite.NewTagRepository(db),
		Lists:       sqlite.NewListRepository(db),
		Suggestions: sqlite.NewSuggestionRepository(db),
		Analysis:    sqlite.NewAnalysisRepository(db),
		Settings:    sqlite.NewSettingsRepository(db),
		SavedViews:  sqlite.NewSavedViewRepository(db),
		SyncRuns:    sqlite.NewSyncRunRepository(db),
	}
	return appui.NewShell(workspace.New(repos, ports.SystemClock{}, idgen.New("test")),
		appui.AppearanceLight), db
}

// walkMenu visits every item, depth first, with its path from the menu bar.
func walkMenu(items []*mygo.MenuItem, path []string, visit func(path []string, item *mygo.MenuItem)) {
	for _, item := range items {
		if item == nil {
			continue
		}
		here := append(append([]string{}, path...), item.Label)
		visit(here, item)
		walkMenu(item.Submenu, here, visit)
	}
}

// TestTheMenuBarIsCompleteAndLocalized walks the menu the app installs.
//
// The bar is the only way to reach reset, restore and the two file panels, so an item
// without a label, without an id or without a handler is an entry point that exists on
// screen and nowhere else. Both locales are checked: the bar is rebuilt on a language
// change, and an item whose label is copied once at build time would keep the old words.
func TestTheMenuBarIsCompleteAndLocalized(t *testing.T) {
	shell, _ := menuShell(t)

	for _, locale := range []struct{ tag, want string }{
		{"en", "Sync now"},
		{"zh-CN", "立即同步"},
	} {
		shell.SetLocale(locale.tag, "en")
		menu := buildMenu(shell)

		ids := map[string]string{}
		walkMenu(menu.Items(), nil, func(path []string, item *mygo.MenuItem) {
			where := ""
			for _, part := range path {
				if part != "" {
					where += part + " > "
				}
			}
			if item.Type == mygo.MenuItemSeparator {
				return
			}
			if item.Label == "" && item.Role == "" {
				t.Errorf("[%s] %sitem without a label or a role", locale.tag, where)
			}
			if item.Click != nil {
				if item.ID == "" {
					t.Errorf("[%s] %sthe clickable item %q has no id", locale.tag, where, item.Label)
					return
				}
				if other, seen := ids[item.ID]; seen {
					t.Errorf("[%s] id %q names both %q and %q", locale.tag, item.ID, other, item.Label)
				}
				ids[item.ID] = item.Label
			}
		})

		// The labels are the interface's language, not whatever was current at build time.
		if got := menu.ItemByID("sync").Label; got != locale.want {
			t.Errorf("[%s] the sync item reads %q, want %q", locale.tag, got, locale.want)
		}
	}
}

// TestEveryMenuCommandDoesSomething presses the items that act without a panel.
//
// Sync, auto-tag and the three file commands need a service or a native sheet, so their
// handlers are covered where those live (exportpanel_test.go, bundle tests). What is left
// is the state-only commands, and each has to change the state its name promises: a menu
// entry that reports a click and does nothing is the defect the whole round is looking for.
func TestEveryMenuCommandDoesSomething(t *testing.T) {
	shell, _ := menuShell(t)
	menu := buildMenu(shell)
	activate := func(id string) {
		t.Helper()
		item := menu.ItemByID(id)
		if item == nil {
			t.Fatalf("the menu has no item %q", id)
		}
		if item.Click == nil {
			t.Fatalf("the menu item %q has no handler", id)
		}
		item.Click(item, nil)
	}

	// View > appearance: each radio changes the theme, and the state is what the window
	// draws from.
	for _, item := range []struct {
		id   string
		want string
	}{
		{"appearance-dark", appui.AppearanceDark},
		{"appearance-light", appui.AppearanceLight},
		{"appearance-system", appui.AppearanceSystem},
	} {
		activate(item.id)
		if got := shell.State().Appearance; got != item.want {
			t.Errorf("%s left the appearance at %q, want %q", item.id, got, item.want)
		}
	}

	// View > settings and Review > suggestions: each opens its panel.
	activate("settings")
	if !shell.State().SettingsOpen {
		t.Error("View > Settings did not open the settings page")
	}
	shell.State().SettingsOpen = false
	activate("review")
	if !shell.State().SuggestionsOpen {
		t.Error("File > Review did not open the suggestions panel")
	}
	shell.State().SuggestionsOpen = false

	// File > add bookmark toggles the manual form.
	activate("add-bookmark")
	if !shell.State().AddOpen {
		t.Error("File > Add bookmark did not open the form")
	}

	// View > toggle sidebar records the reader's choice.
	activate("toggle-sidebar")
	if !shell.State().SidebarChosen {
		t.Error("View > Toggle sidebar did not record a choice")
	}

	// File > reset asks before it acts; the dialog itself is the confirmation.
	activate("reset")
	if !shell.State().ConfirmResetData {
		t.Error("File > Reset local data acted without asking")
	}
}

// TestTheMenuFocusSearchLandsInTheToolbar presses View > Find and checks where the caret is.
//
// The command exists so Cmd-F has a discoverable menu entry; the entry is only worth its
// key equivalent if the press lands in the list's own field.
func TestTheMenuFocusSearchLandsInTheToolbar(t *testing.T) {
	shell, db := menuShell(t)
	shell.SetLocale(i18n.EN, i18n.EN)
	seedMenuBookmark(t, db)
	shell.Reload(context.Background())
	menu := buildMenu(shell)

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	if _, found := tester.Find(shell.T(i18n.SearchPlaceholder)); !found {
		t.Fatalf("the toolbar's search field has no name; texts: %q", tester.Texts())
	}

	item := menu.ItemByID("focus-search")
	if item == nil || item.Click == nil {
		t.Fatal("the menu has no focus-search command")
	}
	item.Click(item, nil)
	tester.Frame()

	if !tester.Focused(shell.T(i18n.SearchPlaceholder)) {
		t.Errorf("Find left the caret outside the search field; texts: %q", tester.Texts())
	}
}

// seedMenuBookmark gives the shell a library, so the results page draws its toolbar.
func seedMenuBookmark(t *testing.T, db *sqlite.DB) {
	t.Helper()
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	bookmark := domain.Bookmark{
		TweetID: "1", TweetURL: "https://x.com/ada/status/1",
		AuthorName: "ada", AuthorHandle: "ada", Text: "a post about Go",
		SavedAt: now, LastSeenAt: now, UpdatedAt: now,
	}
	if _, err := sqlite.NewBookmarkRepository(db).UpsertMany(context.Background(),
		[]domain.Bookmark{bookmark}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}
}
