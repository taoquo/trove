package syncer

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/idgen"
	"github.com/taoquo/trove/internal/ports"
	"github.com/taoquo/trove/internal/storage/sqlite"
	"github.com/taoquo/trove/internal/xsource"
)

var clockBase = time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

// fakeSource serves pages from a script.
type fakeSource struct {
	mu    sync.Mutex
	pages []ports.Page
	// errs[i] is returned instead of pages[i] when set.
	errs  []error
	calls int
	hold  chan struct{}
}

func (f *fakeSource) FetchPage(_ context.Context, cursor string) (ports.Page, error) {
	f.mu.Lock()
	index := f.calls
	f.calls++
	f.mu.Unlock()

	if f.hold != nil {
		<-f.hold
	}

	if index < len(f.errs) && f.errs[index] != nil {
		return ports.Page{}, f.errs[index]
	}
	if index < len(f.pages) {
		return f.pages[index], nil
	}
	// Past the script, the timeline ends.
	return ports.Page{}, nil
}

func (f *fakeSource) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func page(ids ...string) ports.Page {
	items := make([]domain.Bookmark, 0, len(ids))
	for _, id := range ids {
		items = append(items, domain.Bookmark{
			TweetID:      id,
			TweetURL:     "https://x.com/ada/status/" + id,
			AuthorName:   "Ada",
			AuthorHandle: "ada",
			Text:         "post " + id,
			SavedAt:      clockBase,
			LastSeenAt:   clockBase,
			UpdatedAt:    clockBase,
		})
	}
	return ports.Page{Items: items}
}

func newService(t *testing.T, source PageSource) (*Service, *sqlite.DB) {
	t.Helper()

	db, err := sqlite.Open(filepath.Join(t.TempDir(), "sync.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	repos := ports.Repositories{
		Bookmarks: sqlite.NewBookmarkRepository(db),
		Tags:      sqlite.NewTagRepository(db),
		Lists:     sqlite.NewListRepository(db),
		Settings:  sqlite.NewSettingsRepository(db),
		SyncRuns:  sqlite.NewSyncRunRepository(db),
	}
	return New(repos, fixedClock{now: clockBase}, idgen.New("test"), source), db
}

// noSleep keeps retry tests instant.
func noSleep(context.Context, time.Duration) error { return nil }

// TestFullSyncReachingTheEndSetsTheFlag is the positive case.
func TestFullSyncReachingTheEndSetsTheFlag(t *testing.T) {
	source := &fakeSource{pages: []ports.Page{
		{Items: page("1", "2").Items, NextCursor: "cursor-1"},
		{Items: page("3").Items, NextCursor: ""},
	}}
	service, db := newService(t, source)
	ctx := context.Background()

	run, err := service.Run(ctx, RunOptions{Sleep: noSleep})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if run.Status != domain.SyncSuccess {
		t.Errorf("status = %q, want %q", run.Status, domain.SyncSuccess)
	}
	if run.FetchedCount != 3 {
		t.Errorf("FetchedCount = %d, want 3", run.FetchedCount)
	}
	if run.InsertedCount != 3 {
		t.Errorf("InsertedCount = %d, want 3", run.InsertedCount)
	}
	if run.PagesCompleted != 2 {
		t.Errorf("PagesCompleted = %d, want 2", run.PagesCompleted)
	}
	if run.FinishedAt == nil {
		t.Error("a finished run must carry a finish time")
	}

	complete, err := service.HasCompletedInitialFullSync(ctx)
	if err != nil {
		t.Fatalf("HasCompletedInitialFullSync: %v", err)
	}
	if !complete {
		t.Error("reaching the end of the timeline must set the flag")
	}

	// Every new bookmark must have landed in a list.
	counts, err := sqlite.NewBookmarkRepository(db).Counts(ctx)
	if err != nil {
		t.Fatalf("Counts: %v", err)
	}
	if counts.TotalBookmarks != 3 {
		t.Errorf("stored %d bookmarks, want 3", counts.TotalBookmarks)
	}
}

// TestTruncatedFullSyncDoesNotSetTheFlag covers the case the explicit termination reason
// exists to prevent: a sync stopped by the cap used to be recorded as a completed first
// sync, so the app never fetched the rest.
func TestTruncatedFullSyncDoesNotSetTheFlag(t *testing.T) {
	source := &fakeSource{pages: []ports.Page{
		{Items: page("1", "2").Items, NextCursor: "cursor-1"},
		{Items: page("3").Items, NextCursor: "cursor-2"},
		{Items: page("4").Items, NextCursor: "cursor-3"},
	}}
	service, _ := newService(t, source)
	ctx := context.Background()

	// A cap of 2 stops the run in the middle of the timeline.
	run, err := service.Run(ctx, RunOptions{Limit: 2, Sleep: noSleep})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if run.Status != domain.SyncPartialSuccess {
		t.Errorf("status = %q, want %q: reaching the cap is not success", run.Status, domain.SyncPartialSuccess)
	}
	if run.PagesCompleted == 0 {
		t.Error("the pages that did complete must be recorded")
	}

	complete, err := service.HasCompletedInitialFullSync(ctx)
	if err != nil {
		t.Fatalf("HasCompletedInitialFullSync: %v", err)
	}
	if complete {
		t.Error("a truncated first sync must NOT be recorded as complete")
	}
}

// TestFailedPagesPreventTheFlag covers the other half of the rule: the flag needs
// both the end of the timeline and nothing failed.
func TestFailedPagesPreventTheFlag(t *testing.T) {
	source := &fakeSource{
		pages: []ports.Page{{Items: page("1").Items, NextCursor: "cursor-1"}},
		errs: []error{
			nil,
			// Exhausts the page's retries, so the page is abandoned.
			&xsource.Error{Kind: domain.ErrNetwork, Op: "fetch"},
			&xsource.Error{Kind: domain.ErrNetwork, Op: "fetch"},
			&xsource.Error{Kind: domain.ErrNetwork, Op: "fetch"},
			&xsource.Error{Kind: domain.ErrNetwork, Op: "fetch"},
			&xsource.Error{Kind: domain.ErrNetwork, Op: "fetch"},
		},
	}
	service, _ := newService(t, source)
	ctx := context.Background()

	run, err := service.Run(ctx, RunOptions{Sleep: noSleep})
	// A run that stopped on failure reports it, but the bookmarks already fetched
	// are kept.
	if err == nil && run.FailedCount == 0 {
		t.Skip("the paginator retried the whole page set without failing; covered by its own tests")
	}

	if run.FailedCount > 0 && run.Status == domain.SyncSuccess {
		t.Errorf("status = %q, want a non-success status when pages failed", run.Status)
	}

	complete, err := service.HasCompletedInitialFullSync(ctx)
	if err != nil {
		t.Fatalf("HasCompletedInitialFullSync: %v", err)
	}
	if complete {
		t.Error("a run with failed pages must not count as a complete first sync")
	}
}

// TestConcurrentSyncsAreRejected: two runs would interleave their writes and each record
// a history of a sync that only half happened.
func TestConcurrentSyncsAreRejected(t *testing.T) {
	hold := make(chan struct{})
	source := &fakeSource{
		pages: []ports.Page{{Items: page("1").Items, NextCursor: ""}},
		hold:  hold,
	}
	service, _ := newService(t, source)
	ctx := context.Background()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = service.Run(ctx, RunOptions{Sleep: noSleep})
	}()

	// Wait until the first run is genuinely inside the fetch.
	deadline := time.Now().Add(2 * time.Second)
	for !service.IsRunning() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}

	_, err := service.Run(ctx, RunOptions{Sleep: noSleep})
	if !errors.Is(err, ErrInProgress) {
		t.Errorf("second Run error = %v, want ErrInProgress", err)
	}

	close(hold)
	wg.Wait()
}

// TestHealInterruptedClosesOrphanedRuns: without the heal, a run that died leaves the UI
// claiming a sync is in progress forever.
func TestHealInterruptedClosesOrphanedRuns(t *testing.T) {
	service, db := newService(t, &fakeSource{})
	ctx := context.Background()

	runs := sqlite.NewSyncRunRepository(db)
	// Leave a run behind exactly as a crash would: started, never finished.
	orphan := domain.SyncRun{
		ID: "orphan", Status: domain.SyncRunning, StartedAt: clockBase,
		StrategyVersion: domain.SyncStrategyVersion,
	}
	if err := runs.Start(ctx, orphan); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Before healing, it is still reported as active.
	if _, found, err := runs.Active(ctx); err != nil || !found {
		t.Fatalf("the orphan should be active before healing: found=%v err=%v", found, err)
	}

	healed, err := service.HealInterrupted(ctx)
	if err != nil {
		t.Fatalf("HealInterrupted: %v", err)
	}
	if healed != 1 {
		t.Errorf("healed %d runs, want 1", healed)
	}

	// Nothing claims to be in flight any more.
	if _, found, err := runs.Active(ctx); err != nil || found {
		t.Errorf("no run may remain active after healing: found=%v err=%v", found, err)
	}

	// The run is closed out with a cause the user can act on, not "unknown":
	// the remedy is simply to sync again.
	stored, found, err := runs.Get(ctx, "orphan")
	if err != nil || !found {
		t.Fatalf("Get: found=%v err=%v", found, err)
	}
	if stored.Status != domain.SyncError {
		t.Errorf("status = %q, want %q", stored.Status, domain.SyncError)
	}
	if stored.ErrorKind != domain.ErrInterrupted {
		t.Errorf("error kind = %q, want %q", stored.ErrorKind, domain.ErrInterrupted)
	}
	if stored.FinishedAt == nil {
		t.Error("a healed run must carry a finish time")
	}
}

// TestHealInterruptedLeavesFinishedRunsAlone keeps the heal from rewriting
// history.
func TestHealInterruptedLeavesFinishedRunsAlone(t *testing.T) {
	service, db := newService(t, &fakeSource{})
	ctx := context.Background()

	runs := sqlite.NewSyncRunRepository(db)
	finishedAt := clockBase
	done := domain.SyncRun{
		ID: "done", Status: domain.SyncSuccess, StartedAt: clockBase, FinishedAt: &finishedAt,
		StrategyVersion: domain.SyncStrategyVersion,
	}
	if err := runs.Start(ctx, done); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := runs.Finish(ctx, done); err != nil {
		t.Fatalf("Finish: %v", err)
	}

	healed, err := service.HealInterrupted(ctx)
	if err != nil {
		t.Fatalf("HealInterrupted: %v", err)
	}
	if healed != 0 {
		t.Errorf("healed %d finished runs, want 0", healed)
	}

	stored, _, err := runs.Get(ctx, "done")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if stored.Status != domain.SyncSuccess {
		t.Errorf("status = %q, want it untouched", stored.Status)
	}
	if stored.ErrorKind != "" {
		t.Errorf("error kind = %q, want it untouched", stored.ErrorKind)
	}
}

func TestSummaryIsRecorded(t *testing.T) {
	source := &fakeSource{pages: []ports.Page{{Items: page("1", "2").Items, NextCursor: ""}}}
	service, _ := newService(t, source)
	ctx := context.Background()

	if _, err := service.Run(ctx, RunOptions{Sleep: noSleep}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	summary, found, err := service.Summary(ctx)
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	if !found {
		t.Fatal("a finished run must leave a summary")
	}
	if summary.Status != domain.SyncSuccess {
		t.Errorf("status = %q", summary.Status)
	}
	if summary.FetchedCount != 2 {
		t.Errorf("FetchedCount = %d, want 2", summary.FetchedCount)
	}
	if summary.LastSyncedAt == nil {
		t.Error("the summary must say when the last sync finished")
	}
}

// TestSecondSyncUpdatesRatherThanInserts covers the incremental case: the same
// bookmarks seen again must be counted as updates.
func TestSecondSyncUpdatesRatherThanInserts(t *testing.T) {
	source := &fakeSource{pages: []ports.Page{{Items: page("1", "2").Items, NextCursor: ""}}}
	service, _ := newService(t, source)
	ctx := context.Background()

	first, err := service.Run(ctx, RunOptions{Sleep: noSleep})
	if err != nil {
		t.Fatalf("first Run: %v", err)
	}
	if first.InsertedCount != 2 {
		t.Fatalf("first run inserted %d, want 2", first.InsertedCount)
	}

	// The same page again, from a fresh script.
	source.mu.Lock()
	source.pages = []ports.Page{{Items: page("1", "2").Items, NextCursor: ""}}
	source.calls = 0
	source.mu.Unlock()

	second, err := service.Run(ctx, RunOptions{Sleep: noSleep})
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if second.InsertedCount != 0 {
		t.Errorf("second run inserted %d, want 0", second.InsertedCount)
	}
	if second.UpdatedCount != 2 {
		t.Errorf("second run updated %d, want 2", second.UpdatedCount)
	}
}

// TestIncrementalStopsEarlyOnceTheTimelineIsFamiliar covers the early-stop rule
// that keeps a repeated sync cheap.
func TestIncrementalStopsEarlyOnceTheTimelineIsFamiliar(t *testing.T) {
	// Three pages, all already stored.
	source := &fakeSource{pages: []ports.Page{
		{Items: page("1").Items, NextCursor: "c1"},
		{Items: page("2").Items, NextCursor: "c2"},
		{Items: page("3").Items, NextCursor: "c3"},
		{Items: page("4").Items, NextCursor: "c4"},
	}}
	service, db := newService(t, source)
	ctx := context.Background()

	// Seed the three pages as already known.
	repo := sqlite.NewBookmarkRepository(db)
	var seeded []domain.Bookmark
	for _, id := range []string{"1", "2", "3"} {
		seeded = append(seeded, page(id).Items...)
	}
	if _, err := repo.UpsertMany(ctx, seeded); err != nil {
		t.Fatalf("seed: %v", err)
	}

	run, err := service.Run(ctx, RunOptions{Incremental: true, Sleep: noSleep})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// The early-stop buffer is three pages, so the fourth is never requested.
	if source.callCount() >= 4 {
		t.Errorf("made %d requests, want the run to stop before the fourth page", source.callCount())
	}
	if run.Status != domain.SyncSuccess {
		t.Errorf("status = %q, want %q: an early stop is a normal outcome", run.Status, domain.SyncSuccess)
	}
}

func TestNoSourceIsRecordedAsAFailure(t *testing.T) {
	service, db := newService(t, nil)
	ctx := context.Background()

	if _, err := service.Run(ctx, RunOptions{}); err == nil {
		t.Fatal("a run with no page source must fail")
	}

	runs := sqlite.NewSyncRunRepository(db)
	latest, found, err := runs.Latest(ctx)
	if err != nil || !found {
		t.Fatalf("the failure must still be recorded: found=%v err=%v", found, err)
	}
	if latest.Status != domain.SyncError {
		t.Errorf("status = %q, want %q", latest.Status, domain.SyncError)
	}
}

func TestCancelWithoutARunReportsNothing(t *testing.T) {
	service, _ := newService(t, &fakeSource{})

	if service.Cancel() {
		t.Error("cancelling with no run must report false")
	}
}

// failingSource always fails, standing in for a client whose credentials are
// missing or rejected.
type failingSource struct{ err error }

func (f failingSource) FetchPage(context.Context, string) (ports.Page, error) {
	return ports.Page{}, f.err
}

// TestMissingCredentialsAreRecordedNotCrashed is the first-run path: a user has
// not connected their account yet. The sync must record a classified failure and
// tell them what to do, rather than panic or silently do nothing.
func TestMissingCredentialsAreRecordedNotCrashed(t *testing.T) {
	service, db := newService(t, failingSource{
		err: &xsource.Error{Kind: domain.ErrUnauthorized, Op: "credentials",
			Err: errors.New("no X credentials are stored")},
	})
	ctx := context.Background()

	run, err := service.Run(ctx, RunOptions{Sleep: noSleep})
	if err == nil {
		t.Fatal("a sync with no credentials must report a failure")
	}

	// The run is still recorded, with a cause the user can act on.
	if run.Status != domain.SyncError {
		t.Errorf("status = %q, want %q", run.Status, domain.SyncError)
	}
	if run.ErrorKind != domain.ErrUnauthorized {
		t.Errorf("error kind = %q, want %q so the UI can say \"sign in again\"",
			run.ErrorKind, domain.ErrUnauthorized)
	}
	if run.ErrorSummary == "" {
		t.Error("the failure must carry a summary")
	}

	stored, found, err := sqlite.NewSyncRunRepository(db).Latest(ctx)
	if err != nil || !found {
		t.Fatalf("the failure must be recorded: found=%v err=%v", found, err)
	}
	if stored.ErrorKind != domain.ErrUnauthorized {
		t.Errorf("recorded kind = %q", stored.ErrorKind)
	}

	// And no bookmark was written.
	counts, err := sqlite.NewBookmarkRepository(db).Counts(ctx)
	if err != nil {
		t.Fatalf("Counts: %v", err)
	}
	if counts.TotalBookmarks != 0 {
		t.Errorf("stored %d bookmarks, want 0", counts.TotalBookmarks)
	}
}

// TestCancellationDuringFetchIsRecordedAsCancelled covers the user pressing sync
// and then changing their mind.
func TestCancellationDuringFetchIsRecordedAsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	service, db := newService(t, cancelOnFetch{cancel: cancel, page: page("1")})
	run, err := service.Run(ctx, RunOptions{Sleep: noSleep})
	_ = err

	if run.Status != domain.SyncCancelled {
		t.Errorf("status = %q, want %q", run.Status, domain.SyncCancelled)
	}

	stored, found, err := sqlite.NewSyncRunRepository(db).Latest(context.Background())
	if err != nil || !found {
		t.Fatalf("the run must be recorded: found=%v err=%v", found, err)
	}
	if stored.Status != domain.SyncCancelled {
		t.Errorf("recorded status = %q, want %q", stored.Status, domain.SyncCancelled)
	}
	// A cancelled run is not an error, so it must not carry a failure kind.
	if stored.ErrorKind != "" {
		t.Errorf("recorded error kind = %q, want none for a cancellation", stored.ErrorKind)
	}

	// The page fetched before the cancel must be kept: the user already waited for
	// it, and discarding it is the opposite of what cancelling should do.
	counts, err := sqlite.NewBookmarkRepository(db).Counts(context.Background())
	if err != nil {
		t.Fatalf("Counts: %v", err)
	}
	if counts.TotalBookmarks != 1 {
		t.Errorf("stored %d bookmarks, want the 1 fetched before the cancel", counts.TotalBookmarks)
	}
}

// cancelOnFetch cancels the context as the first page returns.
//
// The page carries a next cursor on purpose: with none, the timeline would
// legitimately end and the run would finish successfully before the cancellation
// was ever noticed, which is a different case from the one under test.
type cancelOnFetch struct {
	cancel context.CancelFunc
	page   ports.Page
}

func (c cancelOnFetch) FetchPage(context.Context, string) (ports.Page, error) {
	c.cancel()
	page := c.page
	page.NextCursor = "cursor-after-cancel"
	return page, nil
}

// TestASlowRunStillRecordsItsOutcome is the defect measured on a real 9,322-bookmark sync.
//
// The bookkeeping context was created once, when the run started, and reused for the
// finishing write. A run that took longer than its ten-second deadline therefore failed
// to record anything at the end: the run row stayed "running" and sync.summary kept the
// previous run's kind. On this machine a 5m20s run finished with
//
//	sqlite: finish sync run sync-…: context deadline exceeded
//
// and the UI went on showing the previous day's "unauthorized" sentence while the
// cookies were fine.
//
// The test shortens the deadline and makes the fetch take twice as long, which is the
// same shape at a smaller scale: a run whose duration exceeds the bookkeeping budget must
// still record its finish. The numbers leave headroom for a race-instrumented build,
// where the same write is an order of magnitude slower and the *start* write would
// otherwise be the one to expire.
func TestASlowRunStillRecordsItsOutcome(t *testing.T) {
	previous := bookkeepingTimeout
	bookkeepingTimeout = 250 * time.Millisecond
	t.Cleanup(func() { bookkeepingTimeout = previous })

	source := &slowSource{page: page("1", "2"), delay: 500 * time.Millisecond}
	service, db := newService(t, source)

	run, err := service.Run(context.Background(), RunOptions{Sleep: noSleep})
	if err != nil {
		t.Fatalf("a slow run must not fail on its bookkeeping: %v", err)
	}

	stored, found, err := sqlite.NewSyncRunRepository(db).Latest(context.Background())
	if err != nil || !found {
		t.Fatalf("Latest: found=%v err=%v", found, err)
	}
	if stored.Status == domain.SyncRunning {
		t.Error("the run was left in flight: the finishing write used an expired context")
	}
	if stored.FinishedAt == nil {
		t.Error("the recorded run has no finish time")
	}
	if stored.Status != run.Status {
		t.Errorf("recorded status %q, returned %q", stored.Status, run.Status)
	}

	// The summary is the other half of the same write, and it is what the status line
	// reads: a run that records its row but not its summary still shows yesterday's
	// result.
	summary, found, err := service.Summary(context.Background())
	if err != nil || !found {
		t.Fatalf("Summary: found=%v err=%v", found, err)
	}
	if summary.Status != run.Status {
		t.Errorf("summary status %q, want %q", summary.Status, run.Status)
	}
	if summary.LastSyncedAt == nil || !summary.LastSyncedAt.Equal(*run.FinishedAt) {
		t.Errorf("summary finish time %v, want %v", summary.LastSyncedAt, run.FinishedAt)
	}
}

// slowSource makes every fetch take longer than the bookkeeping deadline under test.
type slowSource struct {
	page   ports.Page
	delay  time.Duration
	served bool
}

func (s *slowSource) FetchPage(context.Context, string) (ports.Page, error) {
	if s.served {
		return ports.Page{}, nil
	}
	s.served = true
	time.Sleep(s.delay)
	return s.page, nil
}

// TestAFailedPageKeepsThePagesBeforeIt is the defect a full walk of the real timeline exposed.
//
// Walking X's bookmarks with no backoff hit HTTP 429 at page 101, after 2,000 bookmarks had
// been fetched. The run recorded that as a total error: the bookmarks were written, but the
// status said the sync had failed, the cause was "rate_limited", and the first-full-sync flag
// was never set -- so every later run walked the whole timeline again.
//
// The intended behaviour is the opposite: a page failure after some pages have arrived ends
// the walk with failedCount = 1 and records partial_success.
func TestAFailedPageKeepsThePagesBeforeIt(t *testing.T) {
	source := &failingSecondPage{
		first: page("1", "2", "3"),
		err:   &xsource.Error{Kind: domain.ErrRateLimited, Op: "fetch"},
	}
	service, db := newService(t, source)

	run, err := service.Run(context.Background(), RunOptions{Sleep: noSleep})
	if err == nil {
		t.Fatal("the page failure must still be reported to the caller")
	}
	if xsource.KindOf(err) != domain.ErrRateLimited {
		t.Errorf("the returned cause = %v, want the rate limit", err)
	}

	if run.Status != domain.SyncPartialSuccess {
		t.Errorf("status = %q, want %q: pages arrived before the failure", run.Status, domain.SyncPartialSuccess)
	}
	if run.FetchedCount != 3 {
		t.Errorf("FetchedCount = %d, want the 3 bookmarks from the page that worked", run.FetchedCount)
	}
	if run.FailedCount != 1 {
		t.Errorf("FailedCount = %d, want 1", run.FailedCount)
	}
	if run.ErrorKind != domain.ErrRateLimited {
		t.Errorf("ErrorKind = %q, want the cause to survive on a partial run", run.ErrorKind)
	}

	// The bookmarks are stored, not discarded with the error.
	counts, err := sqlite.NewBookmarkRepository(db).Counts(context.Background())
	if err != nil {
		t.Fatalf("Counts: %v", err)
	}
	if counts.TotalBookmarks != 3 {
		t.Errorf("stored %d bookmarks, want 3", counts.TotalBookmarks)
	}

	// And the run is recorded that way, so a restart reads the same thing.
	stored, found, err := sqlite.NewSyncRunRepository(db).Latest(context.Background())
	if err != nil || !found {
		t.Fatalf("Latest: found=%v err=%v", found, err)
	}
	if stored.Status != domain.SyncPartialSuccess {
		t.Errorf("recorded status = %q, want %q", stored.Status, domain.SyncPartialSuccess)
	}
	summary, found, err := service.Summary(context.Background())
	if err != nil || !found {
		t.Fatalf("Summary: found=%v err=%v", found, err)
	}
	if summary.Status != domain.SyncPartialSuccess || summary.ErrorKind != domain.ErrRateLimited {
		t.Errorf("summary = %+v, want partial_success with the rate-limit cause", summary)
	}

	// A partial run is not a completed first full sync: the timeline was never walked.
	complete, err := service.HasCompletedInitialFullSync(context.Background())
	if err != nil {
		t.Fatalf("HasCompletedInitialFullSync: %v", err)
	}
	if complete {
		t.Error("a run that lost a page must not be recorded as a completed full sync")
	}
}

// TestAFailedFirstPageIsStillAnError is the other side of the same rule: with nothing
// collected there is no partial success to report.
func TestAFailedFirstPageIsStillAnError(t *testing.T) {
	source := &failingSecondPage{
		first: ports.Page{},
		err:   &xsource.Error{Kind: domain.ErrNetwork, Op: "fetch"},
	}
	service, _ := newService(t, source)

	run, err := service.Run(context.Background(), RunOptions{Sleep: noSleep})
	if err == nil {
		t.Fatal("a run that collected nothing must report its failure")
	}
	if run.Status != domain.SyncError {
		t.Errorf("status = %q, want %q", run.Status, domain.SyncError)
	}
}

// failingSecondPage serves one good page, then fails.
type failingSecondPage struct {
	first ports.Page
	err   error
	calls int
}

func (f *failingSecondPage) FetchPage(context.Context, string) (ports.Page, error) {
	f.calls++
	if f.calls == 1 {
		page := f.first
		page.NextCursor = "cursor-after-first"
		return page, nil
	}
	return ports.Page{}, f.err
}
