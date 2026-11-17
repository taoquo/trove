package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/idgen"
	"github.com/taoquo/trove/internal/ports"
	"github.com/taoquo/trove/internal/storage/sqlite"
	"github.com/taoquo/trove/internal/usecase/workspace"
)

// The UI half of the performance report.
//
// The data-layer benchmark says how long a query takes; this says how long the
// frame takes, which is what a person actually perceives. MyGo renders without a
// display, so the measurement runs in a test.

type benchClock struct{ now time.Time }

func (c benchClock) Now() time.Time { return c.now }

// buildUILibrary fills a database and returns a shell ready to render.
func buildUILibrary(t *testing.T, n int) (*Shell, *sqlite.DB) {
	t.Helper()

	db, err := sqlite.Open(filepath.Join(t.TempDir(), fmt.Sprintf("ui-bench-%d.db", n)))
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

	ctx := context.Background()
	base := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)

	// Tags, so the sidebar has something to draw.
	for i := 0; i < 40; i++ {
		if err := repos.Tags.Create(ctx, domain.Tag{
			ID: fmt.Sprintf("tag-%d", i), Name: fmt.Sprintf("Tag %d", i),
			CanonicalKey: domain.CanonicalTagKey(fmt.Sprintf("Tag %d", i)),
			CreatedAt:    domain.FormatTimestamp(base), UpdatedAt: domain.FormatTimestamp(base),
		}); err != nil {
			t.Fatalf("Create tag: %v", err)
		}
	}

	const batch = 1000
	for start := 0; start < n; start += batch {
		end := start + batch
		if end > n {
			end = n
		}
		rows := make([]domain.Bookmark, 0, end-start)
		for i := start; i < end; i++ {
			rows = append(rows, domain.Bookmark{
				TweetID:      fmt.Sprintf("%d", 1_800_000_000_000_000_000+int64(i)),
				TweetURL:     fmt.Sprintf("https://x.com/author%d/status/%d", i%200, i),
				AuthorName:   fmt.Sprintf("Author %d", i%200),
				AuthorHandle: fmt.Sprintf("author%d", i%200),
				Text: fmt.Sprintf("Post %d. A reasonably long body so the card "+
					"renders several lines of text the way a real post does.", i),
				SavedAt:    base.Add(-time.Duration(i) * time.Minute),
				LastSeenAt: base,
				UpdatedAt:  base,
			})
		}
		if _, err := repos.Bookmarks.UpsertMany(ctx, rows); err != nil {
			t.Fatalf("UpsertMany: %v", err)
		}
	}

	shell := NewShell(workspace.New(repos, benchClock{now: base}, idgen.New("bench")), AppearanceLight)
	shell.Reload(ctx)
	return shell, db
}

// measureRender times a full frame at a given window size.
func measureRender(t *testing.T, shell *Shell, iterations int) (median, p95 time.Duration) {
	t.Helper()

	tester := ui.NewTester(shell.View, 1180, 760)
	samples := make([]time.Duration, 0, iterations)

	for i := 0; i < iterations; i++ {
		start := time.Now()
		tester.Frame()
		samples = append(samples, time.Since(start))
	}

	sortDurations(samples)
	// Nearest-rank p95 -- the same definition as the workspace budget's measure. The plain
	// (n*95)/100 index landed on the maximum for thirty samples as well, so one delayed frame
	// could redden the gate.
	rank := (95*len(samples) + 99) / 100
	return samples[len(samples)/2], samples[rank-1]
}

func sortDurations(values []time.Duration) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}

// frameBudget is the p95 ceiling for one frame, at 10,000 bookmarks.
//
// Twice the Wave 5 render report's measurements, the same starting rule as the
// workspace budget: a loaded machine passes, and a frame that starts rebuilding
// something expensive does not. The settings frame is the cheapest of the three
// because the panel replaces the list rather than adding to it.
const frameBudget = 5 * time.Millisecond

// settingsFrameBudget is the ceiling while the settings panel covers the workspace.
const settingsFrameBudget = 2 * time.Millisecond

// TestUIRenderBudget measures the frame a reader looks at and fails when it crosses its
// ceiling. A frame's cost is not visible in a report, and a view that starts doing work
// per card is exactly the regression a screenshot cannot show.
//
// A frame is wall clock, so it is measured alone. In the parallel `go test ./...` run the
// same frames crossed their ceiling while another package filled a database; the gate's
// budget step sets TROVE_BUDGETS=1 and runs this test in a process of its own.
func TestUIRenderBudget(t *testing.T) {
	if testing.Short() {
		t.Skip("the budget builds a ten-thousand-row database")
	}
	if raceEnabled {
		t.Skip("the render budget is skipped under -race; the detector distorts it")
	}
	if os.Getenv("TROVE_BUDGETS") != "1" {
		t.Skip("wall-clock ceilings run alone: set TROVE_BUDGETS=1 (the gate's budget step does)")
	}

	const size = 10000
	shell, _ := buildUILibrary(t, size)
	const iterations = 30

	check := func(label string, budget time.Duration) {
		t.Helper()
		median, p95 := measureRender(t, shell, iterations)
		t.Logf("| %d | %s | %.2f ms | %.2f ms | %.0f ms |", size, label,
			float64(median.Microseconds())/1000, float64(p95.Microseconds())/1000,
			float64(budget.Microseconds())/1000)
		if p95 > budget {
			t.Errorf("%s took %.2f ms at p95, over the %.0f ms budget",
				label, float64(p95.Microseconds())/1000, float64(budget.Microseconds())/1000)
		}
	}

	t.Logf("| bookmarks | frame | median | p95 | budget |")
	t.Logf("|---|---|---|---|---|")
	check("results list (100 cards)", frameBudget)

	// The inspector populated, which adds a text-heavy column.
	shell.State().Selected = shell.State().Data.Bookmarks[0].TweetID
	check("with inspector open", frameBudget)

	// The settings panel, which is a modal over the workspace.
	shell.State().SettingsOpen = true
	check("with settings panel", settingsFrameBudget)
}
