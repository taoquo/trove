package ui

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/x-master/apps/macos/internal/idgen"
	"github.com/taoquo/x-master/apps/macos/internal/media"
	"github.com/taoquo/x-master/apps/macos/internal/ports"
	"github.com/taoquo/x-master/apps/macos/internal/storage/sqlite"
	"github.com/taoquo/x-master/apps/macos/internal/usecase/workspace"
)

type failingTransport struct{}

func (failingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, context.DeadlineExceeded
}

// TestRealLibraryPicturesStayDrawable measures the flicker report against the user's own
// database and media cache. It is a diagnostic, not a CI test: it skips without
// XBM_REAL_DB.
func TestRealLibraryPicturesStayDrawable(t *testing.T) {
	dbPath := os.Getenv("XBM_REAL_DB")
	if dbPath == "" {
		t.Skip("set XBM_REAL_DB to a copy of x-master.db and XBM_REAL_MEDIA to its media-cache")
	}

	db, err := sqlite.Open(dbPath)
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
	}
	service := workspace.New(repos, clockOnly{}, idgen.New("measure"))
	shell := NewShell(service, AppearanceLight)
	shell.state.LeftSidebarWidth = WidthSidebarDef
	shell.state.RightSidebarWidth = WidthInspectorDef

	fetcher := media.NewFetcherWithClient(&http.Client{Transport: failingTransport{}})
	fetcher.UseDiskCache(media.NewDiskCache(os.Getenv("XBM_REAL_MEDIA"), media.DefaultDiskBudget))
	shell.UseMediaFetcher(fetcher)

	shell.Reload(context.Background())

	tester := ui.NewTester(shell.View, 1180, 900)
	tester.Frame()

	// Every picture the books point at, deduplicated the way the caches key them.
	urls := map[string]bool{}
	for _, row := range shell.state.Data.Bookmarks {
		if row.AuthorAvatarURL != "" {
			urls[row.AuthorAvatarURL] = true
		}
		for _, m := range row.Media {
			if m.URL != "" {
				urls[m.URL] = true
			}
		}
	}
	all := make([]string, 0, len(urls))
	for url := range urls {
		all = append(all, url)
	}

	readyCount := func() int {
		n := 0
		for _, url := range all {
			if _, ok := shell.draw.get(url); ok {
				n++
			}
		}
		return n
	}

	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) && readyCount() < len(all) {
		time.Sleep(50 * time.Millisecond)
	}

	first := readyCount()
	for i := 0; i < 5; i++ {
		tester.Frame()
		time.Sleep(100 * time.Millisecond)
	}
	last := readyCount()

	// What the old draw path would have reported: it asked this cache, and this cache is the
	// reason the screen blinked.
	cached := 0
	for _, url := range all {
		if _, ok := shell.media.Cached(url); ok {
			cached++
		}
	}

	t.Logf("%d bookmarks, %d distinct picture urls: fetcher holds %d, drawable %d then %d after five frames",
		len(shell.state.Data.Bookmarks), len(all), cached, first, last)
	if last < first {
		t.Errorf("frames lost pictures: %d, then %d", first, last)
	}
	// Not an error: this harness has no network, so a picture the disk cache does not hold
	// cannot arrive. The report that matters is the one above -- what the screen keeps.
	if last < len(all) {
		t.Logf("%d of %d pictures are not on disk and cannot be measured here", len(all)-last, len(all))
	}
}

type clockOnly struct{}

func (clockOnly) Now() time.Time { return time.Now() }
