package main

import (
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/taoquo/trove/internal/idgen"
	"github.com/taoquo/trove/internal/ports"
	"github.com/taoquo/trove/internal/storage/sqlite"
	appui "github.com/taoquo/trove/internal/ui"
	"github.com/taoquo/trove/internal/usecase/workspace"
)

// TestCompositionRootRendersWithoutAWindow runs the same wiring start() builds,
// minus the window, so a change to the composition root is caught here rather
// than at launch.
//
// The view is rendered with MyGo's own tester: no display, no event loop.
func TestCompositionRootRendersWithoutAWindow(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "root.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	// The same repository set start() assembles.
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

	shell := appui.NewShell(
		workspace.New(repos, ports.SystemClock{}, idgen.New("test")),
		appui.AppearanceLight,
	)

	tester := ui.NewTester(shell.View, 1180, 760)
	if !tester.HasText("All bookmarks") {
		t.Errorf("the workspace shell did not render; texts: %q", tester.Texts())
	}
	// The chrome is the hero: the brand and, under it, the pair of facet tabs. "Library" was
	// a heading this shell invented.
	if !tester.HasText("Bookmarks") || !tester.HasText("Tags") || !tester.HasText("Authors") {
		t.Errorf("the shell is missing its chrome; texts: %q", tester.Texts())
	}
}

// TestShouldHideOnCloseTellsTheTwoPathsApart pins the decision the close handler makes.
//
// The handler needs a live window, so what is testable is the predicate. The two paths differ
// in one bit: the red button's close must be prevented (which hides the window), and the quit
// path's close must not be, because MyGo's prepareQuit walks the windows calling close() and
// treats a prevented one as a cancelled quit. Getting this wrong is not subtle -- an earlier
// version shipped with every close prevented, and Cmd-Q stopped working while the red button
// appeared to work.
func TestShouldHideOnCloseTellsTheTwoPathsApart(t *testing.T) {
	if !shouldHideOnClose(false) {
		t.Error("a close outside a quit must be prevented, or the red button quits the app")
	}
	if shouldHideOnClose(true) {
		t.Error("a close during a quit must not be prevented, or Quit is cancelled and does nothing")
	}
}
