package workspace_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
//
// p95 is the nearest rank: ceil(0.95*n), one-based, so twenty samples report the nineteenth.
// The plain (n*95)/100 indexing this replaced reported the twentieth -- the maximum -- and the
// gate runs the budgets right after the race tests, when one sample can be delayed by a
// second's worth of cache and page work. A single spike is not a regression; a regression moves
// every sample and still fails, because the ceiling is compared against the twenty-first
// percentile of the slow end rather than the median alone.
func measure(iterations int, fn func()) (median, p95 time.Duration) {
	samples := make([]time.Duration, 0, iterations)
	for i := 0; i < iterations; i++ {
		start := time.Now()
		fn()
		samples = append(samples, time.Since(start))
	}

	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	rank := (95*len(samples) + 99) / 100
	return samples[len(samples)/2], samples[rank-1]
}

// logRow prints one measurement as a markdown table row.
func logRow(t *testing.T, size int, label string, median, p95 time.Duration) {
	t.Helper()
	t.Logf("| %d | %s | %.1f ms | %.1f ms |", size,
		label, float64(median.Microseconds())/1000, float64(p95.Microseconds())/1000)
}

// budgetLibrarySize is the library the budget is measured on: one size, big enough
// that a per-row cost shows up and small enough that the gate pays for it once.
const budgetLibrarySize = 10000

// performanceBudget is the p95 ceiling for each read path at 10k bookmarks.
//
// The numbers are twice the measurements the Wave 5 scaling report took on the machine
// the plan was written on, which leaves a loaded or slower machine room to pass while
// still failing a change that makes a path much more expensive. They are meant to
// tighten as the paths are measured on more machines; a report is not a gate, and this is.
var performanceBudget = []struct {
	name   string
	query  domain.LibraryQuery
	budget time.Duration
}{
	{"open workspace (first page)", domain.LibraryQuery{}, 75 * time.Millisecond},
	{"search (LIKE, one term)",
		domain.LibraryQuery{Filter: domain.LibraryFilter{Query: "kubernetes"}},
		320 * time.Millisecond},
	{"filter by one tag",
		domain.LibraryQuery{Filter: domain.LibraryFilter{TagIDs: []string{"tag-0"}}},
		100 * time.Millisecond},
	{"unclassified",
		domain.LibraryQuery{Filter: domain.LibraryFilter{OnlyUnclassified: true}},
		80 * time.Millisecond},
	{"deep page (offset n/2)",
		domain.LibraryQuery{Offset: budgetLibrarySize / 2},
		5 * time.Millisecond},
}

// budgetsRequested tells a wall-clock test whether it was asked for.
//
// `go test ./...` runs packages in parallel, and a query timed while another package is
// filling a ten-thousand-row database reads that contention, not the code. The gate run
// that showed this had search at 366 ms against its 320 ms ceiling and a deep page at
// 5.2 ms against 5.0 -- both under budget when measured in a process of their own. So the
// ceilings are measured where they mean something: the gate's budget step sets
// TROVE_BUDGETS=1 and names each test, and the full-suite run skips them on purpose.
func budgetsRequested() bool { return os.Getenv("TROVE_BUDGETS") == "1" }

// writeBudget is the ceiling for a sync's batch write.
const writeBudget = 140 * time.Millisecond

// TestPerformanceBudget measures the paths a reader waits on at 10,000 bookmarks and
// fails when one of them crosses its ceiling.
//
// A regression used to be something a report showed and a reader had to notice. The
// numbers below are the same paths at one size, with the ceiling written down beside
// them, so a change that scans the table for a filtered page turns the gate red.
func TestPerformanceBudget(t *testing.T) {
	if testing.Short() {
		t.Skip("the budget builds a ten-thousand-row database")
	}
	if raceEnabled {
		t.Skip("timings are meaningless under the race detector")
	}
	if !budgetsRequested() {
		t.Skip("wall-clock ceilings run alone: set TROVE_BUDGETS=1 (the gate's budget step does)")
	}

	library := buildLibrary(t, budgetLibrarySize)
	ctx := context.Background()

	paths := make([]struct {
		name   string
		query  domain.LibraryQuery
		budget time.Duration
	}, 0, len(performanceBudget)+len(domain.AllSortOrders))
	for _, order := range domain.AllSortOrders {
		paths = append(paths, struct {
			name   string
			query  domain.LibraryQuery
			budget time.Duration
		}{"sort: " + string(order), domain.LibraryQuery{Sort: order}, 5 * time.Millisecond})
	}
	paths = append(paths, performanceBudget...)

	t.Logf("| bookmarks | operation | median | p95 | budget |")
	t.Logf("|---|---|---|---|---|")
	const iterations = 20
	for _, path := range paths {
		query := path.query
		// One read before measuring: the first page pays for opening the database and
		// filling the sidebar cache, and that is not what a steady-state frame costs.
		if _, err := library.service.Load(ctx, query); err != nil {
			t.Fatalf("warm %s: %v", path.name, err)
		}
		median, p95 := measure(iterations, func() {
			if _, err := library.service.Load(ctx, query); err != nil {
				t.Fatalf("Load %s: %v", path.name, err)
			}
		})
		t.Logf("| %d | %s | %.1f ms | %.1f ms | %.0f ms |", budgetLibrarySize, path.name,
			float64(median.Microseconds())/1000, float64(p95.Microseconds())/1000,
			float64(path.budget.Microseconds())/1000)
		if p95 > path.budget {
			t.Errorf("%s took %.1f ms at p95, over the %.1f ms budget",
				path.name, float64(p95.Microseconds())/1000,
				float64(path.budget.Microseconds())/1000)
		}
	}

	batch := make([]domain.Bookmark, 0, 500)
	for i := 0; i < 500; i++ {
		batch = append(batch, domain.Bookmark{
			TweetID: fmt.Sprintf("budget-write-%d", i), Text: "written by the budget",
			SavedAt: time.Now(), UpdatedAt: time.Now(),
		})
	}
	median, p95 := measure(iterations, func() {
		if _, err := library.repos.Bookmarks.UpsertMany(ctx, batch); err != nil {
			t.Fatalf("UpsertMany: %v", err)
		}
	})
	t.Logf("| %d | write 500 bookmarks (sync batch) | %.1f ms | %.1f ms | %.0f ms |",
		budgetLibrarySize, float64(median.Microseconds())/1000,
		float64(p95.Microseconds())/1000, float64(writeBudget.Microseconds())/1000)
	if p95 > writeBudget {
		t.Errorf("the sync batch took %.1f ms at p95, over the %.1f ms budget",
			float64(p95.Microseconds())/1000, float64(writeBudget.Microseconds())/1000)
	}
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
	if !budgetsRequested() {
		t.Skip("wall-clock comparisons run alone: set TROVE_BUDGETS=1 (the gate's budget step does)")
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
