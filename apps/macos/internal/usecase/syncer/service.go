// Package syncer runs the bookmark sync.
//
// It is the seam between the X client and storage: the client knows the wire
// protocol, the repositories know SQL, and only this package knows the order the
// two have to happen in.
//
// Two defects from docs/x-sync-protocol.md §3.2 are closed here rather than in
// either side: S6, two syncs running at once, and S7, a run left in flight by a
// crash.
package syncer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/taoquo/x-master/apps/macos/internal/domain"
	"github.com/taoquo/x-master/apps/macos/internal/ports"
	"github.com/taoquo/x-master/apps/macos/internal/xsource"
)

// Setting keys owned by this package.
const (
	summaryKey           = "sync.summary"
	completedFullSyncKey = "sync.hasCompletedInitialFullSync"
	// resumeCursorKey is where a full walk that reached the limit left off.
	// Without it the next run started at the top again, so a library larger
	// than the limit could never sync past it.
	resumeCursorKey = "sync.resumeCursor"
)

// ErrInProgress reports a second sync started while one is running.
//
// This is defect S6. Two runs would interleave their writes, fight over the same
// cursor, and each record a full history of a sync that only half happened.
var ErrInProgress = errors.New("syncer: a sync is already in progress")

// persistContext returns a context for writing what a run already fetched.
//
// A cancelled run has pages in hand that the user paid for with their time. Writing
// them through the run's own context fails, which silently discards them and turns
// a cancellation into a write error -- the opposite of what cancelling should do.
func persistContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), persistTimeout)
}

// persistTimeout bounds the write that saves a run's fetched pages.
const persistTimeout = 60 * time.Second

// bookkeepingContext returns a context for recording a run's outcome.
//
// The run's own context is already cancelled by the time a cancelled or
// timed-out run needs to be written down, so recording through it fails and the
// row stays "running" forever. That is precisely the symptom the startup
// self-heal exists to clean up, and leaving it to the next launch is not good
// enough: the user would see "syncing" until they quit and reopened.
//
// The write is short and local, so it gets its own bounded context that keeps the
// values but drops the cancellation.
//
// It must be called *per write*, immediately before the write. It used to be
// created once when the run started and reused for the finishing write minutes
// later; the ten-second deadline had expired by then, so every sync longer than
// ten seconds failed to record its outcome with "context deadline exceeded" and
// left the row in "running" forever. Measured on a real 9,322-bookmark sync: the
// run took 5m20s and the finish write failed.
func bookkeepingContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), bookkeepingTimeout)
}

// defaultPagePace is the minimum gap between two page requests.
//
// Measured on this machine: a full walk of 469 pages spends about 40 seconds in
// requests, and X answered HTTP 429 on page 101 of a separate walk made with no
// pause at all. 250ms over 469 pages adds under two minutes to a first full sync
// and keeps the request rate near what a person clicking through the app produces.
const defaultPagePace = 250 * time.Millisecond

// bookkeepingTimeout bounds a run's own bookkeeping write.
//
// It is a variable rather than a constant so a test can shorten it: the defect it
// bounds needs a run that outlives the bound, and a ten-second test would be the
// slowest in the suite by an order of magnitude. Nothing outside this package sets
// it.
var bookkeepingTimeout = 10 * time.Second

// PageSource fetches one page of the bookmark timeline.
//
// It is an interface so the orchestration can be tested without a network, and so
// the client stays replaceable.
type PageSource interface {
	FetchPage(ctx context.Context, cursor string) (ports.Page, error)
}

// CredentialRefresher re-reads the X session from wherever it can and reports
// whether the stored pair changed.
//
// A refresh that finds the same pair returns false: X would answer the retry
// exactly as it answered the first request, and a second doomed request is not a
// recovery.
type CredentialRefresher func(ctx context.Context) (bool, error)

// credentialRefreshTimeout bounds the refresh. The browser read has its own
// operations to do -- snapshot the profile, ask the Keychain, decrypt -- and a
// hung one must not hold the run open.
const credentialRefreshTimeout = 20 * time.Second

// UseCredentialRefresher installs the re-authentication step.
//
// Without one, an expired session is a run that fails and waits for the reader to
// go to Settings. With one, the run heals itself: the extension never sees this
// failure because chrome.cookies hands it a fresh pair on every request, and this
// is the desktop equivalent.
func (s *Service) UseCredentialRefresher(refresh CredentialRefresher) {
	s.refreshCredentials = refresh
}

// Service runs syncs.
type Service struct {
	repos  ports.Repositories
	clock  ports.Clock
	ids    ports.IDGenerator
	source PageSource

	mu      sync.Mutex
	running bool
	// progress is the live run, for the UI.
	progress *xsource.Progress
	// cancel stops the run in flight.
	cancel context.CancelFunc
	// refreshCredentials re-reads the session when a run dies before its first
	// page. Nil leaves the failure to the caller.
	refreshCredentials CredentialRefresher
}

// New builds a service.
func New(repos ports.Repositories, clock ports.Clock, ids ports.IDGenerator, source PageSource) *Service {
	if clock == nil {
		clock = ports.SystemClock{}
	}
	return &Service{repos: repos, clock: clock, ids: ids, source: source}
}

// RunOptions configures one sync.
type RunOptions struct {
	// Limit caps how many bookmarks a first full sync collects.
	Limit int
	// Incremental grows the early-stop window; a full sync disables it.
	Incremental bool
	// StopBufferPages is the early-stop window for an incremental run. Zero
	// uses the domain default.
	StopBufferPages int
	// StartCursor resumes a full walk that stopped at the limit.
	StartCursor string
	// OnProgress is called as pages complete.
	OnProgress func(xsource.Progress)
	// RawPayloads stores the untouched response bodies. It is opt-in because it
	// multiplies the database size for data the app does not read.
	RawPayloads bool
	// Retry overrides the per-page retry policy; the zero value uses the default.
	Retry xsource.RetryPolicy
	// Sleep waits between retries; tests replace it.
	Sleep func(ctx context.Context, d time.Duration) error
}

// Run performs one sync and records it.
func (s *Service) Run(ctx context.Context, options RunOptions) (domain.SyncRun, error) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return domain.SyncRun{}, ErrInProgress
	}
	s.running = true

	runCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.mu.Unlock()

	defer func() {
		cancel()
		s.mu.Lock()
		s.running = false
		s.cancel = nil
		s.progress = nil
		s.mu.Unlock()
	}()

	if s.source == nil {
		return s.recordFailure(ctx, options, errors.New("syncer: no page source configured"))
	}

	startedAt := s.clock.Now()
	run := domain.SyncRun{
		ID:        s.newID("sync"),
		Status:    domain.SyncRunning,
		StartedAt: startedAt,
		// The policy is recorded with the run, so a later change to the early-stop
		// rules is distinguishable in history.
		StrategyVersion: domain.SyncStrategyVersion,
	}
	if err := s.withBookkeeping(ctx, func(write context.Context) error {
		return s.repos.SyncRuns.Start(write, run)
	}); err != nil {
		return run, err
	}

	// Known ids drive the incremental early-stop rule. They must be read before
	// the run inserts anything, or every page looks new.
	existing, err := s.repos.Bookmarks.AllIDs(ctx)
	if err != nil {
		return s.finishFailure(ctx, run, err)
	}
	known := make(map[string]bool, len(existing))
	for _, id := range existing {
		known[id] = true
	}

	limit := options.Limit
	if limit <= 0 {
		limit = domain.DefaultSyncLimit
	}

	// A first full sync stops only at the end of the pages; an incremental one may
	// stop early once the timeline is entirely familiar.
	stopBuffer := 0
	if options.Incremental {
		stopBuffer = options.StopBufferPages
		if stopBuffer <= 0 {
			stopBuffer = domain.DefaultIncrementalStopBufferPages
		}
	}

	// Pages are written as they arrive, not once at the end.
	//
	// A first full sync of a large library is minutes of paced requests, and the
	// old single write at the end meant the window showed nothing and the database
	// changed only when the last page landed. Per-page writes also make a partial
	// run -- cancelled, failed, or killed -- cost only the page in flight rather
	// than the whole walk, and let the list grow while the run is still going.
	//
	// The counters are summed here rather than read from the final result, because
	// a page that could not be written is not in the library even though it is in
	// the result.
	upserted := ports.UpsertResult{}
	stored := 0
	writePage := func(_ int, pageBookmarks []domain.Bookmark) error {
		if len(pageBookmarks) == 0 {
			return nil
		}
		// Its own context: a cancelled run still writes the page it already has, the
		// same reason the closing write has one.
		write, cancelWrite := persistContext(ctx)
		defer cancelWrite()

		written, err := s.repos.Bookmarks.UpsertMany(write, pageBookmarks)
		if err != nil {
			return err
		}
		upserted.Inserted += written.Inserted
		upserted.Updated += written.Updated
		stored += len(pageBookmarks)
		return nil
	}

	paginate := func() (xsource.Result, error) {
		return xsource.Paginate(runCtx, xsource.Options{
			FetchPage:                      s.source.FetchPage,
			Limit:                          limit,
			StartCursor:                    options.StartCursor,
			StopAfterConsecutiveKnownPages: stopBuffer,
			IsExistingBookmark:             func(bookmark domain.Bookmark) bool { return known[bookmark.TweetID] },
			OnPage:                         writePage,
			OnProgress: func(progress xsource.Progress) {
				s.setProgress(progress)
				if options.OnProgress != nil {
					options.OnProgress(progress)
				}
			},
			Retry: options.Retry,
			Pace:  defaultPagePace,
			Sleep: options.Sleep,
			Now:   s.clock.Now,
		})
	}

	result, paginateErr := paginate()

	// An expired session is the one failure with a known, cheap repair: the
	// browser still has a live one. Attempt it only when nothing was fetched --
	// after a page has completed, restarting would repeat work and reorder the
	// run for a problem the remaining pages have already survived.
	if s.shouldRefreshCredentials(result, paginateErr, runCtx) {
		refreshCtx, cancelRefresh := context.WithTimeout(runCtx, credentialRefreshTimeout)
		refreshed, _ := s.refreshCredentials(refreshCtx)
		cancelRefresh()
		if refreshed {
			result, paginateErr = paginate()
		}
	}

	// The closing writes still use a context of their own, for the reason the page
	// writes do: a cancelled run's own context would fail them and throw away what
	// the pages already paid for.
	persist, cancelPersist := persistContext(ctx)
	defer cancelPersist()

	if stored > 0 {
		// Every new bookmark has to land in a list, or it is invisible in every
		// list view including the Inbox. This runs once over the whole library
		// rather than per page: the assignment is an UPDATE over the rows that
		// have no list, and doing it per page would rescan them on every page of
		// a first sync.
		if _, err := s.repos.Bookmarks.AssignMissingToInbox(persist, s.clock.Now()); err != nil {
			return s.finishFailure(ctx, run, err)
		}
	}

	if options.RawPayloads && len(result.RawPayloads) > 0 {
		// A failure here must not lose the sync: the payloads are diagnostic, and
		// the bookmarks are the point.
		_ = s.storeRawPayloads(persist, result.RawPayloads)
	}

	finishedAt := s.clock.Now()
	run.FinishedAt = &finishedAt
	run.FetchedCount = stored
	run.InsertedCount = upserted.Inserted
	run.UpdatedCount = upserted.Updated
	run.FailedCount = result.FailedCount
	run.PagesCompleted = result.PagesCompleted
	run.Status = statusFor(result.Termination, result.FailedCount, stored)

	// A cancellation is not a failure, so it records no cause. Setting one would
	// put an error reason beside a status the user chose themselves, and the UI
	// would report "cancelled: interrupted" for pressing stop.
	if paginateErr != nil && result.Termination != domain.TermCancelled {
		run.ErrorKind = errorKindFor(paginateErr)
		run.ErrorSummary = paginateErr.Error()
		if run.Status == domain.SyncSuccess {
			run.Status = domain.SyncError
		}
	}

	// Recorded through its own context: the run's may already be cancelled, and the
	// context this write needs must be created now rather than when the run began.
	if err := s.withBookkeeping(ctx, func(write context.Context) error {
		return s.repos.SyncRuns.Finish(write, run)
	}); err != nil {
		return run, err
	}

	s.recordSummary(ctx, run)
	s.recordResumeCursor(ctx, result)

	// The first-full-sync flag is set only by a run that genuinely reached the end
	// of the timeline with nothing failed. Setting it from a truncated run is the
	// defect the explicit termination reason exists to prevent.
	if !options.Incremental && domain.CompletesFullSync(result.Termination, result.FailedCount) {
		if err := s.markFullSyncComplete(ctx); err != nil {
			return run, err
		}
	}

	// A failed run is reported as a failure. Swallowing it would leave the caller
	// to infer the outcome from the status field, and a UI that only checks the
	// error would show a failed sync as a successful one.
	if paginateErr != nil && !errors.Is(paginateErr, xsource.ErrCancelled) {
		return run, paginateErr
	}
	return run, nil
}

// shouldRefreshCredentials reports whether a failed run is worth one re-auth
// attempt.
//
// Three conditions, each excluding a case where the attempt would be wrong:
// there must be a refresher at all; the run must not have been cancelled (the
// reader pressed Stop, and a browser excursion is not what they asked for); and
// no page may have completed, because a refresh restarts pagination from the
// start cursor.
func (s *Service) shouldRefreshCredentials(result xsource.Result, err error, ctx context.Context) bool {
	if s.refreshCredentials == nil || err == nil || ctx.Err() != nil {
		return false
	}
	if result.PagesCompleted > 0 {
		return false
	}
	switch xsource.KindOf(err) {
	case domain.ErrUnauthorized, domain.ErrAuthExpired, domain.ErrCSRFInvalid:
		return true
	default:
		return false
	}
}

// statusFor maps a termination reason onto a recorded status.
//
// fetched is how many bookmarks the run collected before it stopped, and it is what
// separates "a page failed and we lost the rest" from "nothing worked at all". The
// source makes the same distinction in one line:
//
//	status = failedCount > 0 ? "partial_success" : "success"   (src/background/syncBookmarks.ts:189)
//
// after a catch that only rethrows when nothing was collected
// (src/lib/x/paginateBookmarks.ts:60-65). Recording the first case as an error threw
// away the meaning of 9,322 stored bookmarks: the status line said the sync had failed
// outright, and the first-full-sync flag was never set, so every later run walked the
// whole timeline again.
func statusFor(reason domain.TerminationReason, failedCount, fetched int) domain.SyncStatus {
	switch reason {
	case domain.TermEndOfPages:
		if failedCount > 0 {
			// Pages were lost, so the library is incomplete even though the
			// timeline ended.
			return domain.SyncPartialSuccess
		}
		return domain.SyncSuccess

	case domain.TermStoppedEarly:
		if failedCount > 0 {
			return domain.SyncPartialSuccess
		}
		return domain.SyncSuccess

	case domain.TermHitLimit:
		// Reaching the cap is not success: there is more on X than we took.
		return domain.SyncPartialSuccess

	case domain.TermCancelled:
		return domain.SyncCancelled

	case domain.TermFailed:
		if fetched > 0 {
			// Some pages arrived before the failing one. The library grew, so the
			// run is a partial success with a cause, not a total failure.
			return domain.SyncPartialSuccess
		}
		return domain.SyncError

	default:
		return domain.SyncError
	}
}

// errorKindFor classifies a pagination failure.
//
// The client already speaks the domain's own vocabulary, so its kind is used
// directly rather than translated: a translation table is one more place for the
// two to drift apart.
func errorKindFor(err error) domain.SyncErrorKind {
	if err == nil {
		return ""
	}

	var classified *xsource.Error
	if errors.As(err, &classified) && classified.Kind != "" {
		return classified.Kind
	}

	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return domain.ErrInterrupted
	}
	// A cursor that repeated means the response shape is not what we expect.
	if errors.Is(err, xsource.ErrPaginationLoop) {
		return domain.ErrResponseSchemaChanged
	}
	if errors.Is(err, xsource.ErrCancelled) {
		return domain.ErrInterrupted
	}
	return domain.ErrUnknown
}

// finishFailure records a run that could not complete.
func (s *Service) finishFailure(ctx context.Context, run domain.SyncRun, cause error) (domain.SyncRun, error) {
	finishedAt := s.clock.Now()
	run.FinishedAt = &finishedAt
	run.Status = domain.SyncError
	run.ErrorKind = errorKindFor(cause)
	run.ErrorSummary = cause.Error()

	if err := s.withBookkeeping(ctx, func(write context.Context) error {
		return s.repos.SyncRuns.Finish(write, run)
	}); err != nil {
		return run, errors.Join(cause, err)
	}
	s.recordSummary(ctx, run)
	return run, cause
}

// recordFailure handles a failure that happened before a run record existed.
func (s *Service) recordFailure(ctx context.Context, _ RunOptions, cause error) (domain.SyncRun, error) {
	now := s.clock.Now()
	run := domain.SyncRun{
		ID:              s.newID("sync"),
		Status:          domain.SyncError,
		StartedAt:       now,
		FinishedAt:      &now,
		ErrorKind:       errorKindFor(cause),
		ErrorSummary:    cause.Error(),
		StrategyVersion: domain.SyncStrategyVersion,
	}
	if err := s.repos.SyncRuns.Start(ctx, run); err != nil {
		return run, errors.Join(cause, err)
	}
	if err := s.withBookkeeping(ctx, func(write context.Context) error {
		return s.repos.SyncRuns.Finish(write, run)
	}); err != nil {
		return run, errors.Join(cause, err)
	}
	s.recordSummary(ctx, run)
	return run, cause
}

// storeRawPayloads writes the untouched response bodies.
func (s *Service) storeRawPayloads(ctx context.Context, payloads map[string]json.RawMessage) error {
	if len(payloads) == 0 {
		return nil
	}
	capturedAt := domain.FormatTimestamp(s.clock.Now())
	list := make([]ports.RawPayload, 0, len(payloads))
	for tweetID, raw := range payloads {
		if tweetID == "" || len(raw) == 0 {
			continue
		}
		list = append(list, ports.RawPayload{TweetID: tweetID, PayloadJSON: string(raw), CapturedAt: capturedAt})
	}
	return s.repos.Bookmarks.SaveRawPayloads(ctx, list)
}

// recordResumeCursor keeps the continuation point in step with the walk.
//
// It is written only when the limit stopped the run, and cleared when the walk
// reached the end: a stale cursor would ask X for a page it has already given
// us, and the completed flag is what tells the next run to be incremental.
func (s *Service) recordResumeCursor(ctx context.Context, result xsource.Result) {
	if err := ctx.Err(); err != nil {
		return
	}
	switch {
	case result.Termination == domain.TermHitLimit && result.NextCursor != "":
		_ = s.repos.Settings.Set(ctx, resumeCursorKey, result.NextCursor, s.clock.Now())
	case result.Termination == domain.TermEndOfPages:
		_, _ = s.repos.Settings.Delete(ctx, resumeCursorKey)
	}
}

// ResumeCursor is where the next full sync should continue, empty when there is
// nothing outstanding.
//
// A full walk is capped so it cannot run unbounded. The cap used to be a dead
// end: the next sync restarted at the top, collected the same ten thousand
// bookmarks, and never reached the older ones.
func (s *Service) ResumeCursor(ctx context.Context) string {
	raw, found, err := s.repos.Settings.Get(ctx, resumeCursorKey)
	if err != nil || !found {
		return ""
	}
	return raw
}

// Cancel asks the run in flight to stop.
//
// The context is cancelled rather than a flag being set, because the client is
// blocked in an HTTP request: a flag would not be noticed until the request
// returned, which is exactly when the user has already given up.
func (s *Service) Cancel() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel == nil {
		return false
	}
	s.cancel()
	return true
}

// Progress returns the live progress, and whether a run is in flight.
func (s *Service) Progress() (xsource.Progress, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.progress == nil {
		return xsource.Progress{}, false
	}
	return *s.progress, true
}

// IsRunning reports whether a sync is in flight right now.
func (s *Service) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

func (s *Service) setProgress(progress xsource.Progress) {
	s.mu.Lock()
	defer s.mu.Unlock()
	copied := progress
	s.progress = &copied
}

// HealInterrupted closes out runs left in flight by a crash.
//
// This is defect S7. A run that died leaves the UI claiming a sync is in progress
// forever, and nothing else will ever clear it: the process that owned the run is
// gone. It must run at startup, before anything reads the sync state.
//
// It returns how many runs were closed out.
func (s *Service) HealInterrupted(ctx context.Context) (int, error) {
	if s.repos.SyncRuns == nil {
		return 0, nil
	}
	return s.repos.SyncRuns.MarkInterrupted(ctx, s.clock.Now())
}

// Summary returns the headline result of the most recent sync.
//
// It is read from its own key rather than derived from the run history: a user
// may prune history but must never lose "when did I last sync, and did it work".
func (s *Service) Summary(ctx context.Context) (domain.SyncSummary, bool, error) {
	if s.repos.Settings == nil {
		return domain.SyncSummary{}, false, nil
	}

	raw, found, err := s.repos.Settings.Get(ctx, summaryKey)
	if err != nil || !found || raw == "" {
		return domain.SyncSummary{}, false, err
	}

	var summary domain.SyncSummary
	if err := json.Unmarshal([]byte(raw), &summary); err != nil {
		// An unreadable summary must not fail the caller: it is a status line, and
		// the run history still has the truth.
		return domain.SyncSummary{}, false, nil
	}
	return summary, true, nil
}

// HasCompletedInitialFullSync reports whether a first full sync ever finished.
func (s *Service) HasCompletedInitialFullSync(ctx context.Context) (bool, error) {
	if s.repos.Settings == nil {
		return false, nil
	}
	raw, found, err := s.repos.Settings.Get(ctx, completedFullSyncKey)
	if err != nil || !found {
		return false, err
	}
	return raw == "true", nil
}

// markFullSyncComplete records that the timeline was walked to its end.
func (s *Service) markFullSyncComplete(ctx context.Context) error {
	if s.repos.Settings == nil {
		return nil
	}
	return s.withBookkeeping(ctx, func(write context.Context) error {
		return s.repos.Settings.Set(write, completedFullSyncKey, "true", s.clock.Now())
	})
}

// withBookkeeping runs one recording write on a fresh bounded context.
//
// Every write that records a run's state goes through here, so the deadline is
// created when the write happens rather than when the run started. See
// bookkeepingContext for the defect that made this necessary.
func (s *Service) withBookkeeping(ctx context.Context, write func(context.Context) error) error {
	bookkeeping, cancel := bookkeepingContext(ctx)
	defer cancel()
	return write(bookkeeping)
}

// recordSummary stores the headline result.
//
// The write gets its own bookkeeping context rather than the caller's: this runs
// after a run that may have been cancelled, and a cancelled context would drop
// the summary while leaving the run row behind -- the two would then disagree
// about what the last sync did.
func (s *Service) recordSummary(ctx context.Context, run domain.SyncRun) {
	if s.repos.Settings == nil {
		return
	}

	summary := domain.SyncSummary{
		Status:        run.Status,
		FetchedCount:  run.FetchedCount,
		InsertedCount: run.InsertedCount,
		UpdatedCount:  run.UpdatedCount,
		FailedCount:   run.FailedCount,
		LastSyncedAt:  run.FinishedAt,
		ErrorKind:     run.ErrorKind,
		ErrorSummary:  run.ErrorSummary,
	}
	encoded, err := json.Marshal(summary)
	if err != nil {
		return
	}
	_ = s.withBookkeeping(ctx, func(write context.Context) error {
		return s.repos.Settings.Set(write, summaryKey, string(encoded), s.clock.Now())
	})
}

// newID mints an identifier, falling back to a timestamp when no generator is
// configured so a run is never recorded with an empty id.
func (s *Service) newID(prefix string) string {
	if s.ids == nil {
		return fmt.Sprintf("%s-%d", prefix, s.clock.Now().UnixNano())
	}
	return s.ids.NewID()
}

// CleanupOrphanedRuns is the startup entry point: it heals interrupted runs and
// reports what it did, so the app can log it.
func (s *Service) CleanupOrphanedRuns(ctx context.Context) (healed int, err error) {
	healed, err = s.HealInterrupted(ctx)
	if err != nil {
		return 0, fmt.Errorf("syncer: heal interrupted runs: %w", err)
	}
	return healed, nil
}
