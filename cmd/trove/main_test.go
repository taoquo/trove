package main

import (
	"path/filepath"
	"testing"
	"time"

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

// The close button's handler is not tested here.
//
// It needs a live window, and the decision it makes -- a close outside a quit is postponed into
// the quit sequence, a close inside one is let through -- only shows up as a process that
// either ends or does not. tools/smoke.sh is where that is observed: the app closes its own
// window (Window.Close fires the same event the traffic light does) and the script fails if the
// pid survives.

// TestSmokeCloseDelay pins the hook's conditions: it needs the smoke run, and a duration it can
// parse and use, so a typo cannot take the window away from a real user.
func TestSmokeCloseDelay(t *testing.T) {
	t.Setenv("TROVE_SMOKE", "1")
	t.Setenv("TROVE_SMOKE_CLOSE", "10s")
	if got := smokeCloseDelay(); got != 10*time.Second {
		t.Errorf("smokeCloseDelay = %v, want 10s", got)
	}

	t.Setenv("TROVE_SMOKE", "")
	if got := smokeCloseDelay(); got != 0 {
		t.Errorf("without the smoke run smokeCloseDelay = %v, want 0", got)
	}

	t.Setenv("TROVE_SMOKE", "1")
	for _, raw := range []string{"", "soon", "0s", "-5s"} {
		t.Setenv("TROVE_SMOKE_CLOSE", raw)
		if got := smokeCloseDelay(); got != 0 {
			t.Errorf("smokeCloseDelay with %q = %v, want 0", raw, got)
		}
	}
}
