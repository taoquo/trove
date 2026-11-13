package xsource

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/ports"
)

// TerminationReason explains why pagination stopped. It is the domain type, so
// the sync service can decide whether a first full sync may be recorded as
// complete without translating between vocabularies.
type TerminationReason = domain.TerminationReason

const (
	// TermEndOfPages: X reported no further cursor. The only clean ending.
	TermEndOfPages = domain.TermEndOfPages
	// TermHitLimit: the configured cap stopped the run early.
	TermHitLimit = domain.TermHitLimit
	// TermStoppedEarly: the incremental early-stop rule fired.
	TermStoppedEarly = domain.TermStoppedEarly
	// TermCancelled: the user cancelled.
	TermCancelled = domain.TermCancelled
	// TermFailed: a page failed and the run stopped.
	TermFailed = domain.TermFailed
)

// PageFetcher fetches one page. It is a function so pagination can be tested
// without a network.
type PageFetcher func(ctx context.Context, cursor string) (ports.Page, error)

// Progress is a snapshot of a run in flight.
//
// It carries what the UI needs to explain a run: how many pages completed, how many
// items arrived, and when a rate limit has us waiting.
type Progress struct {
	Phase          string
	Page           int
	Fetched        int
	PagesCompleted int
	Failed         int
	// WaitingUntil is set while backing off, so the UI can show a countdown
	// instead of a frozen bar.
	WaitingUntil time.Time
}

// RetryPolicy bounds how hard a single page is retried.
type RetryPolicy struct {
	// MaxAttempts is the total number of tries per page, including the first.
	MaxAttempts int
	// BaseDelay is the first backoff step.
	BaseDelay time.Duration
	// MaxDelay caps the exponential growth.
	MaxDelay time.Duration
	// MaxRetryAfter caps how long a server-requested wait may be honoured. A
	// larger value is reported rather than slept through, so an absurd header
	// cannot park the app.
	MaxRetryAfter time.Duration
	// JitterFraction spreads retries (0 disables it, for deterministic tests).
	JitterFraction float64
}

// DefaultRetryPolicy is the policy a real sync uses.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts:    5,
		BaseDelay:      2 * time.Second,
		MaxDelay:       30 * time.Second,
		MaxRetryAfter:  30 * time.Second,
		JitterFraction: 0.2,
	}
}

// Options configures a paginated run.
type Options struct {
	// FetchPage performs one request. Required.
	FetchPage PageFetcher
	// Limit caps how many bookmarks are collected; zero uses DefaultSyncLimit.
	// Reaching it is reported as TermHitLimit, never as a completed sync.
	Limit int
	// StartCursor resumes a walk that stopped at the limit. Empty starts at the
	// top. A full sync capped at DefaultSyncLimit would otherwise re-walk the
	// same first ten thousand bookmarks on every run and never reach the older
	// ones.
	StartCursor string
	// StopAfterConsecutiveKnownPages ends an incremental sync early once this
	// many consecutive pages contain nothing new. Zero disables the rule.
	StopAfterConsecutiveKnownPages int
	// IsExistingBookmark reports whether a bookmark is already stored. When the
	// early-stop rule is enabled it is required, because the rule cannot work
	// without it.
	IsExistingBookmark func(domain.Bookmark) bool
	// OnProgress is called as pages complete.
	OnProgress func(Progress)
	// OnPage, when set, receives each page's bookmarks as they arrive, already cut
	// to the run's limit and carrying their timeline rank. It is how a run's pages
	// reach storage during the walk instead of all at the end: a first full sync is
	// minutes of requests, and a caller that waits for Result holds the library
	// still for the whole of it.
	//
	// An error ends the run as a failure. The pages already handed over stay
	// written, which is the point of handing them over at all -- the caller records
	// the run as a partial success with the cause.
	OnPage func(page int, bookmarks []domain.Bookmark) error
	// Retry bounds per-page retries; the zero value uses DefaultRetryPolicy.
	Retry RetryPolicy
	// Pace is the minimum delay between two page requests. Zero means no delay.
	//
	// A full walk of a large library is hundreds of requests in a few minutes:
	// measured here, 469 pages took 40 seconds of request time and X answered HTTP
	// 429 on page 101 of a separate walk. Spacing the requests is what keeps a
	// first full sync inside the allowance.
	Pace time.Duration
	// Sleep waits between attempts; tests replace it.
	Sleep func(ctx context.Context, d time.Duration) error
	// Now is the clock, for tests.
	Now func() time.Time
}

// Result is the outcome of a paginated run.
type Result struct {
	Bookmarks []domain.Bookmark
	// Termination says why the loop ended. Only TermEndOfPages means X had
	// nothing more to give.
	Termination TerminationReason
	// FailedCount counts pages that were abandoned after their retries.
	FailedCount int
	// PagesCompleted counts pages that were fetched successfully.
	PagesCompleted int
	// PageError is the failure of the page that ended the run, when Termination
	// is TermFailed. It is carried on the Result rather than only returned so a
	// caller can record a partial run's cause without discarding its bookmarks.
	PageError error
	// RawPayloads is each tweet's untouched JSON, when raw capture is enabled.
	RawPayloads map[string]json.RawMessage
	// NextCursor is where a later run resumes when Termination is TermHitLimit.
	// Empty means there is nothing to continue from.
	NextCursor string
}

// ErrPaginationLoop reports a cursor that repeated. Comparing only against the
// immediately previous cursor would let a cycle through three or more cursors request
// forever.
var ErrPaginationLoop = errors.New("xsource: the timeline cursor repeated")

// Paginate walks the timeline until it ends, the limit is reached, the caller
// cancels, or a page fails.
func Paginate(ctx context.Context, opts Options) (Result, error) {
	if opts.FetchPage == nil {
		return Result{}, errors.New("xsource: Paginate needs a FetchPage")
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = domain.DefaultSyncLimit
	}
	if opts.StopAfterConsecutiveKnownPages > 0 && opts.IsExistingBookmark == nil {
		// Without the predicate the rule would treat every page as containing
		// something new and never fire, so refuse rather than silently not
		// stopping.
		return Result{}, errors.New("xsource: StopAfterConsecutiveKnownPages needs IsExistingBookmark")
	}

	retry := opts.Retry
	if retry.MaxAttempts <= 0 {
		retry = DefaultRetryPolicy()
	}

	collector := &collector{
		limit:            limit,
		earlyStopPages:   opts.StopAfterConsecutiveKnownPages,
		isExisting:       opts.IsExistingBookmark,
		onProgress:       opts.OnProgress,
		onPage:           opts.OnPage,
		retry:            retry,
		pace:             opts.Pace,
		sleep:            opts.Sleep,
		now:              opts.Now,
		cursor:           opts.StartCursor,
		rawPayloads:      map[string]json.RawMessage{},
		seenCursors:      map[string]bool{},
		consecutiveKnown: 0,
	}

	return collector.run(ctx, opts.FetchPage)
}

type collector struct {
	limit          int
	earlyStopPages int
	isExisting     func(domain.Bookmark) bool
	onProgress     func(Progress)
	onPage         func(page int, bookmarks []domain.Bookmark) error

	retry RetryPolicy
	pace  time.Duration
	sleep func(ctx context.Context, d time.Duration) error
	now   func() time.Time

	bookmarks        []domain.Bookmark
	seenCursors      map[string]bool
	consecutiveKnown int
	pagesCompleted   int
	failedCount      int
	cursor           string
	nextCursor       string
	rawPayloads      map[string]json.RawMessage
}

func (c *collector) run(ctx context.Context, fetch PageFetcher) (Result, error) {
	for {
		if err := ctx.Err(); err != nil {
			return c.result(domain.TermCancelled), ErrCancelled
		}

		page, err := c.fetchWithRetry(ctx, fetch)
		if err != nil {
			if IsCancelled(err) {
				return c.result(domain.TermCancelled), ErrCancelled
			}
			// A page that failed after its retries ends the run, but it does not
			// invalidate the pages before it. Keep them, count the failure, and
			// report the cause on the Result.
			//
			// The run is recorded as partial_success, not error. Returning the
			// error instead would record a run that had collected 9,322 bookmarks and
			// then hit a single bad page -- measured here as HTTP 429 at page 101 when
			// the timeline is walked without backoff -- as a total failure, and the
			// first-full-sync flag would never be set.
			c.failedCount++
			c.report()
			result := c.result(domain.TermFailed)
			result.PageError = err
			return result, err
		}

		remaining := c.limit - len(c.bookmarks)
		pageBookmarks := page.Items
		truncated := false
		if len(pageBookmarks) > remaining {
			pageBookmarks = pageBookmarks[:remaining]
			truncated = true
		}

		// The rank is the post's position in the walk, not its index inside the page,
		// and it is assigned here because a page handed to OnPage is written before
		// the walk ends -- result() still recomputes the whole collection at the end,
		// so the two agree.
		offset := len(c.bookmarks)
		for i := range pageBookmarks {
			rank := offset + i
			pageBookmarks[i].TimelineRank = &rank
		}

		c.bookmarks = append(c.bookmarks, pageBookmarks...)
		c.pagesCompleted++
		c.nextCursor = page.NextCursor
		for tweetID, raw := range page.RawPayloads {
			if tweetID != "" && len(raw) > 0 {
				c.rawPayloads[tweetID] = raw
			}
		}

		if c.earlyStopPages > 0 {
			// The predicate may record ids as it goes, which keeps a page of
			// duplicates from looking entirely known.
			hasNew := false
			for _, bookmark := range pageBookmarks {
				if !c.isExisting(bookmark) {
					hasNew = true
				}
			}
			if hasNew {
				c.consecutiveKnown = 0
			} else {
				c.consecutiveKnown++
			}
		}

		// Before the progress event, not after: a caller that reloads on progress is
		// reading the rows this page just wrote.
		if c.onPage != nil {
			if err := c.onPage(c.pagesCompleted, pageBookmarks); err != nil {
				result := c.result(domain.TermFailed)
				result.PageError = err
				return result, err
			}
		}

		c.report()

		switch {
		case len(c.bookmarks) >= c.limit:
			return c.result(domain.TermHitLimit), nil

		case truncated:
			return c.result(domain.TermHitLimit), nil

		case page.NextCursor == "":
			return c.result(domain.TermEndOfPages), nil

		case page.NextCursor == c.cursor:
			// The timeline handed back the cursor it was given, so there is nothing
			// further to ask for. "Same cursor again" is X's own end-of-timeline
			// signal, so there is nothing to resume from even though the cursor is
			// non-empty. Recording it as a schema change was wrong in a way that
			// mattered: measured on a real 9,344-bookmark walk, this is exactly where X
			// ends the timeline, so every full sync would be filed as a failure and the
			// first-full-sync flag would never be set.
			c.nextCursor = ""
			return c.result(domain.TermEndOfPages), nil

		case c.seenCursors[page.NextCursor]:
			// A cursor from earlier in the same walk is a different thing: the loop is
			// cycling and would never end. Detecting it and reporting a failure is
			// stricter than checking only the immediately previous cursor.
			return c.result(domain.TermFailed), fmt.Errorf("%w: cursor %q came back around", ErrPaginationLoop, page.NextCursor)

		case c.earlyStopPages > 0 && c.consecutiveKnown >= c.earlyStopPages:
			return c.result(domain.TermStoppedEarly), nil
		}

		c.seenCursors[c.cursor] = true
		c.cursor = page.NextCursor

		if c.pace > 0 {
			if err := c.sleepFor(ctx, c.pace); err != nil {
				return c.result(domain.TermCancelled), ErrCancelled
			}
		}
	}
}

// sleepFor waits, reporting the wait so the UI can count down.
func (c *collector) sleepFor(ctx context.Context, delay time.Duration) error {
	if c.onProgress != nil {
		progress := c.progress()
		progress.WaitingUntil = c.clock().Add(delay)
		c.onProgress(progress)
	}
	if c.sleep != nil {
		return c.sleep(ctx, delay)
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// fetchWithRetry attempts one page, honouring Retry-After and a bounded
// exponential backoff.
func (c *collector) fetchWithRetry(ctx context.Context, fetch PageFetcher) (ports.Page, error) {
	var lastErr error

	for attempt := 1; attempt <= c.retry.MaxAttempts; attempt++ {
		page, err := fetch(ctx, c.cursor)
		if err == nil {
			return page, nil
		}

		lastErr = err

		if IsCancelled(err) {
			return ports.Page{}, ErrCancelled
		}
		if !IsRetryable(err) {
			// An invalid credential can never succeed on retry, and looping on
			// it is what the spec forbids.
			return ports.Page{}, err
		}
		if attempt == c.retry.MaxAttempts {
			break
		}

		delay := c.backoff(attempt)

		// A server-requested wait wins, but only up to the cap. Beyond it we
		// stop and report, rather than parking the app on a bogus header.
		if requested := RetryAfter(err); requested > 0 {
			if requested > c.retry.MaxRetryAfter {
				return ports.Page{}, err
			}
			delay = requested
		}

		c.waitFor(ctx, delay)

		if err := ctx.Err(); err != nil {
			return ports.Page{}, ErrCancelled
		}
	}

	return ports.Page{}, lastErr
}

// backoff is an exponential delay with optional jitter.
func (c *collector) backoff(attempt int) time.Duration {
	delay := c.retry.BaseDelay << (attempt - 1)
	if delay > c.retry.MaxDelay || delay <= 0 {
		delay = c.retry.MaxDelay
	}
	if c.retry.JitterFraction > 0 {
		spread := float64(delay) * c.retry.JitterFraction
		delay += time.Duration((rand.Float64()*2 - 1) * spread)
		if delay < 0 {
			delay = 0
		}
	}
	return delay
}

// waitFor sleeps, reporting the wait so the UI can count down.
func (c *collector) waitFor(ctx context.Context, delay time.Duration) {
	if c.onProgress != nil {
		progress := c.progress()
		progress.WaitingUntil = c.clock().Add(delay)
		c.onProgress(progress)
	}

	sleep := c.sleep
	if sleep == nil {
		sleep = func(ctx context.Context, d time.Duration) error {
			timer := time.NewTimer(d)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		}
	}
	_ = sleep(ctx, delay)
}

func (c *collector) report() {
	if c.onProgress != nil {
		c.onProgress(c.progress())
	}
}

func (c *collector) progress() Progress {
	return Progress{
		Phase:          "fetching",
		Page:           c.pagesCompleted,
		Fetched:        len(c.bookmarks),
		PagesCompleted: c.pagesCompleted,
		Failed:         c.failedCount,
	}
}

func (c *collector) clock() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}

// result finalises the run, assigning timeline ranks over the whole collection.
//
// Ranks are global and zero-based: they are the post's position in the bookmark
// timeline as of this run, and are recomputed on every sync.
func (c *collector) result(termination TerminationReason) Result {
	ranked := make([]domain.Bookmark, len(c.bookmarks))
	copy(ranked, c.bookmarks)
	for i := range ranked {
		rank := i
		ranked[i].TimelineRank = &rank
	}

	var raw map[string]json.RawMessage
	if len(c.rawPayloads) > 0 {
		raw = c.rawPayloads
	}
	next := ""
	if termination == domain.TermHitLimit {
		next = c.nextCursor
	}
	return Result{
		Bookmarks:      ranked,
		Termination:    termination,
		FailedCount:    c.failedCount,
		PagesCompleted: c.pagesCompleted,
		RawPayloads:    raw,
		NextCursor:     next,
	}
}
