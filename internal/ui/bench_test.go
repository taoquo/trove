package ui

import (
	"context"
	"fmt"
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
	shell.State().SystemLocale = "en"
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
	return samples[len(samples)/2], samples[(len(samples)*95)/100]
}

func sortDurations(values []time.Duration) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}

// TestUIRenderScalingReport measures how long a frame takes at each library size.
func TestUIRenderScalingReport(t *testing.T) {
	if testing.Short() {
		t.Skip("the render report builds large databases")
	}
	if raceEnabled {
		t.Skip("the render report is skipped under -race; the detector distorts it")
	}

	t.Logf("| bookmarks | frame | median | p95 |")
	t.Logf("|---|---|---|---|")

	for _, size := range []int{1000, 10000, 50000} {
		shell, _ := buildUILibrary(t, size)

		iterations := 30
		if size >= 50000 {
			iterations = 15
		}

		median, p95 := measureRender(t, shell, iterations)
		t.Logf("| %d | results list (100 cards) | %.2f ms | %.2f ms |", size,
			float64(median.Microseconds())/1000, float64(p95.Microseconds())/1000)

		// The inspector populated, which adds a text-heavy column.
		shell.State().Selected = shell.State().Data.Bookmarks[0].TweetID
		median, p95 = measureRender(t, shell, iterations)
		t.Logf("| %d | with inspector open | %.2f ms | %.2f ms |", size,
			float64(median.Microseconds())/1000, float64(p95.Microseconds())/1000)

		// The settings panel, which is a modal over the workspace.
		shell.State().SettingsOpen = true
		median, p95 = measureRender(t, shell, iterations)
		t.Logf("| %d | with settings panel | %.2f ms | %.2f ms |", size,
			float64(median.Microseconds())/1000, float64(p95.Microseconds())/1000)
	}
}
