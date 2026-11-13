package workspace_test

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/idgen"
	"github.com/taoquo/trove/internal/ports"
	"github.com/taoquo/trove/internal/storage/sqlite"
	"github.com/taoquo/trove/internal/usecase/workspace"
)

// The scaling benchmark for the workspace's read paths.
//
// It measures the paths a person actually waits on: opening the workspace, typing
// in the search box, filtering by tag, and switching sort order. The point is not
// a single number but where the cost starts to matter, which is why it runs at
// three sizes rather than one.

var benchSizes = []int{1000, 10000, 50000}

// benchClock is fixed, so timestamps do not vary between runs.
type benchClock struct{}

func (benchClock) Now() time.Time { return time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC) }

// benchLibrary is a database filled to a given size.
type benchLibrary struct {
	db      *sqlite.DB
	service *workspace.Service
	repos   ports.Repositories
}

// buildLibrary fills a database with n bookmarks, tags and relations.
func buildLibrary(t *testing.T, n int) *benchLibrary {
	t.Helper()

	db, err := sqlite.Open(filepath.Join(t.TempDir(), fmt.Sprintf("bench-%d.db", n)))
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
		// The revision source the app itself passes (main.go). Without it the sidebar
		// aggregates are recomputed on every Load, which is a configuration nothing ships:
		// the table would report the cost of a cache that is always cold. The cache's own
		// effect is measured separately by TestTheSidebarCacheIsWhatMakesAFilterCheap.
		Revision: db,
	}

	ctx := context.Background()
	base := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)

	// Tags: one per thirty bookmarks, which is the ratio a real library settles at.
	tagCount := n/30 + 1
	tags := make([]domain.Tag, 0, tagCount)
	for i := 0; i < tagCount; i++ {
		tags = append(tags, domain.Tag{
			ID: fmt.Sprintf("tag-%d", i), Name: fmt.Sprintf("Tag %d", i),
			CanonicalKey: domain.CanonicalTagKey(fmt.Sprintf("Tag %d", i)),
			CreatedAt:    domain.FormatTimestamp(base), UpdatedAt: domain.FormatTimestamp(base),
		})
	}
	for _, tag := range tags {
		if err := repos.Tags.Create(ctx, tag); err != nil {
			t.Fatalf("Create tag: %v", err)
		}
	}

	// Bookmarks, inserted in batches the way a sync writes them.
	const batch = 500
	for start := 0; start < n; start += batch {
		end := start + batch
		if end > n {
			end = n
		}

		rows := make([]domain.Bookmark, 0, end-start)
		for i := start; i < end; i++ {
			rank := i
			rows = append(rows, domain.Bookmark{
				TweetID:      fmt.Sprintf("%d", 1_800_000_000_000_000_000+int64(i)),
				TweetURL:     fmt.Sprintf("https://x.com/author%d/status/%d", i%500, i),
				AuthorName:   fmt.Sprintf("Author %d", i%500),
				AuthorHandle: fmt.Sprintf("author%d", i%500),
				Text:         fmt.Sprintf("Post %d about %s and %s", i, benchTopics[i%len(benchTopics)], benchTopics[(i*7)%len(benchTopics)]),
				SavedAt:      base.Add(-time.Duration(i) * time.Minute),
				LastSeenAt:   base,
				UpdatedAt:    base,
				TimelineRank: &rank,
				SourceKind:   domain.SourceBookmark,
			})
		}
		if _, err := repos.Bookmarks.UpsertMany(ctx, rows); err != nil {
			t.Fatalf("UpsertMany: %v", err)
		}
	}

	// Relations: most bookmarks carry one tag, some two.
	relations := make([]domain.BookmarkTag, 0, n)
	for i := 0; i < n; i++ {
		if i%5 == 0 {
			continue // leave a fifth unclassified, so that filter has work to do
		}
		relations = append(relations, domain.BookmarkTag{
			BookmarkID: fmt.Sprintf("%d", 1_800_000_000_000_000_000+int64(i)),
			TagID:      fmt.Sprintf("tag-%d", (i/30)%tagCount),
			CreatedAt:  base,
			Source:     domain.TagSourceManual,
		})
	}
	if err := repos.Tags.AttachMany(ctx, relations); err != nil {
		t.Fatalf("AttachMany: %v", err)
	}

	// Every bookmark needs a list, or it is invisible in every list view.
	if _, err := repos.Bookmarks.AssignMissingToInbox(ctx, base); err != nil {
		t.Fatalf("AssignMissingToInbox: %v", err)
	}

	service := workspace.New(repos, benchClock{}, idgen.New("bench"))
	return &benchLibrary{db: db, service: service, repos: repos}
}

// benchTopics make the search term realistic: a term that appears in a fraction
// of posts, not in all of them.
var benchTopics = []string{
	"compilers", "databases", "kubernetes", "rust", "typography",
	"coffee", "hiking", "jazz", "photography", "woodworking",
}

// measure runs fn repeatedly and returns the median and p95 durations.
func measure(iterations int, fn func()) (median, p95 time.Duration) {
	samples := make([]time.Duration, 0, iterations)
	for i := 0; i < iterations; i++ {
		start := time.Now()
		fn()
		samples = append(samples, time.Since(start))
	}

	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	return samples[len(samples)/2], samples[(len(samples)*95)/100]
}

// logRow prints one measurement as a markdown table row.
func logRow(t *testing.T, size int, label string, median, p95 time.Duration) {
	t.Helper()
	t.Logf("| %d | %s | %.1f ms | %.1f ms |", size,
		label, float64(median.Microseconds())/1000, float64(p95.Microseconds())/1000)
}

// TestScalingReport measures the paths a user waits on at three sizes.
func TestScalingReport(t *testing.T) {
	if testing.Short() {
		t.Skip("the scaling report builds large databases")
	}
	if raceEnabled {
		// Race instrumentation makes a fifty-thousand-row database take minutes.
		// The report is a measurement, and the numbers would be meaningless under
		// the detector anyway, so it is skipped rather than slowed.
		t.Skip("the scaling report is skipped under -race; run it without the detector")
	}

	var startMemory runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&startMemory)

	t.Logf("| bookmarks | operation | median | p95 |")
	t.Logf("|---|---|---|---|")

	for _, size := range benchSizes {
		library := buildLibrary(t, size)
		ctx := context.Background()

		iterations := 20
		if size >= 50000 {
			iterations = 10
		}

		// Opening the workspace: the query plus the sidebar aggregates.
		median, p95 := measure(iterations, func() {
			if _, err := library.service.Load(ctx, domain.LibraryQuery{}); err != nil {
				t.Fatalf("Load: %v", err)
			}
		})
		logRow(t, size, "open workspace (first page)", median, p95)

		// Typing in the search box. This is the path the FTS5 decision was about.
		median, p95 = measure(iterations, func() {
			if _, err := library.service.Load(ctx, domain.LibraryQuery{
				Filter: domain.LibraryFilter{Query: "kubernetes"},
			}); err != nil {
				t.Fatalf("Load search: %v", err)
			}
		})
		logRow(t, size, "search (LIKE, one term)", median, p95)

		// Filtering by tag.
		median, p95 = measure(iterations, func() {
			if _, err := library.service.Load(ctx, domain.LibraryQuery{
				Filter: domain.LibraryFilter{TagIDs: []string{"tag-0"}},
			}); err != nil {
				t.Fatalf("Load tag: %v", err)
			}
		})
		logRow(t, size, "filter by one tag", median, p95)

		// The Unclassified view, which is a NOT EXISTS over the relation table.
		median, p95 = measure(iterations, func() {
			if _, err := library.service.Load(ctx, domain.LibraryQuery{
				Filter: domain.LibraryFilter{OnlyUnclassified: true},
			}); err != nil {
				t.Fatalf("Load unclassified: %v", err)
			}
		})
		logRow(t, size, "unclassified", median, p95)

		// Each sort order, since each has its own index path.
		for _, order := range domain.AllSortOrders {
			order := order
			median, p95 = measure(iterations, func() {
				if _, err := library.service.Load(ctx, domain.LibraryQuery{Sort: order}); err != nil {
					t.Fatalf("Load sort %s: %v", order, err)
				}
			})
			logRow(t, size, "sort: "+order, median, p95)
		}

		// Scrolling: a deep page, which is where an offset scan shows up.
		median, p95 = measure(iterations, func() {
			if _, err := library.service.Load(ctx, domain.LibraryQuery{Offset: size / 2}); err != nil {
				t.Fatalf("Load deep page: %v", err)
			}
		})
		logRow(t, size, "deep page (offset n/2)", median, p95)

		// Writing: the batch size a sync uses.
		median, p95 = measure(iterations, func() {
			rows := make([]domain.Bookmark, 0, 500)
			for i := 0; i < 500; i++ {
				rows = append(rows, domain.Bookmark{
					TweetID: fmt.Sprintf("bench-write-%d-%d", size, i), Text: "written by the benchmark",
					SavedAt: time.Now(), UpdatedAt: time.Now(),
				})
			}
			if _, err := library.repos.Bookmarks.UpsertMany(ctx, rows); err != nil {
				t.Fatalf("UpsertMany: %v", err)
			}
		})
		logRow(t, size, "write 500 bookmarks (sync batch)", median, p95)
	}

	var endMemory runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&endMemory)

	t.Logf("")
	t.Logf("peak heap during the run: %.1f MiB", float64(endMemory.HeapSys)/1024/1024)
	t.Logf("total allocated during the run: %.1f MiB", float64(endMemory.TotalAlloc-startMemory.TotalAlloc)/1024/1024)
	t.Logf("goroutines at the end: %d", runtime.NumGoroutine())
}

// TestTheSidebarCacheIsWhatMakesAFilterCheap measures the revision cache against the same
// library with the cache switched off (Revision nil), so the difference is the cache and
// nothing else: same database, same query, same process.
//
// It exists because a cache that quietly stops being consulted looks exactly like a fast
// one from the outside. The assertion is deliberately loose -- at least 2x -- so a loaded
// machine cannot fail it, while a cache that stops working (both sides equal) fails at once.
func TestTheSidebarCacheIsWhatMakesAFilterCheap(t *testing.T) {
	if testing.Short() {
		t.Skip("the cache comparison builds large databases")
	}
	if raceEnabled {
		t.Skip("timings are meaningless under the race detector")
	}

	for _, size := range []int{10000, 50000} {
		library := buildLibrary(t, size)
		ctx := context.Background()

		// One service wired the way main.go wires the app, one without a revision source.
		// They share the database, so the only difference is whether the sidebar is reused.
		cached := workspace.New(library.repos, benchClock{}, idgen.New("bench"))
		uncachedRepos := library.repos
		uncachedRepos.Revision = nil
		uncached := workspace.New(uncachedRepos, benchClock{}, idgen.New("bench"))

		cached.Load(ctx, domain.LibraryQuery{})
		uncached.Load(ctx, domain.LibraryQuery{})

		iterations := 20
		if size >= 50000 {
			iterations = 10
		}
		withCache, withCacheP95 := measure(iterations, func() {
			cached.Load(ctx, domain.LibraryQuery{})
		})
		withoutCache, withoutCacheP95 := measure(iterations, func() {
			uncached.Load(ctx, domain.LibraryQuery{})
		})

		t.Logf("| %d | Load with the sidebar reused | %.1f ms | %.1f ms |",
			size, float64(withCache.Microseconds())/1000, float64(withCacheP95.Microseconds())/1000)
		t.Logf("| %d | Load recomputing the sidebar | %.1f ms | %.1f ms |",
			size, float64(withoutCache.Microseconds())/1000, float64(withoutCacheP95.Microseconds())/1000)

		if withCache*2 > withoutCache {
			t.Errorf("%d bookmarks: reusing the sidebar took %v, recomputing it %v -- the cache is not doing its work",
				size, withCache, withoutCache)
		}
	}
}
