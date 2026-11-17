package xsource

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/ports"
)

func bm(id string) domain.Bookmark {
	return domain.Bookmark{TweetID: id, TweetURL: "https://x.com/i/status/" + id}
}

func ids(bookmarks []domain.Bookmark) []string {
	out := make([]string, len(bookmarks))
	for i, b := range bookmarks {
		out[i] = b.TweetID
	}
	return out
}

type step struct {
	expectCursor string
	page         ports.Page
	err          error
}

// queueFetcher replays scripted pages and asserts the cursor each call receives,
// so a pagination bug shows up as a wrong cursor rather than a wrong count.
func queueFetcher(t *testing.T, steps []step) PageFetcher {
	t.Helper()
	index := 0
	return func(_ context.Context, cursor string) (ports.Page, error) {
		if index >= len(steps) {
			t.Fatalf("unexpected extra fetch with cursor %q (only %d steps scripted)", cursor, len(steps))
		}
		current := steps[index]
		index++
		if current.expectCursor != cursor {
			t.Errorf("fetch %d received cursor %q, want %q", index, cursor, current.expectCursor)
		}
		return current.page, current.err
	}
}

// instantSleep records the delays instead of waiting, so retry timing is
// asserted without slowing the suite.
func instantSleep(record *[]time.Duration) func(context.Context, time.Duration) error {
	return func(_ context.Context, d time.Duration) error {
		*record = append(*record, d)
		return nil
	}
}

func TestPaginateWalksToTheEnd(t *testing.T) {
	fetch := queueFetcher(t, []step{
		{expectCursor: "", page: ports.Page{Items: []domain.Bookmark{bm("1"), bm("2")}, NextCursor: "c2"}},
		{expectCursor: "c2", page: ports.Page{Items: []domain.Bookmark{bm("3")}, NextCursor: ""}},
	})

	result, err := Paginate(context.Background(), Options{FetchPage: fetch})
	if err != nil {
		t.Fatalf("Paginate: %v", err)
	}

	if result.Termination != TermEndOfPages {
		t.Errorf("Termination = %q, want %q", result.Termination, TermEndOfPages)
	}
	if got := ids(result.Bookmarks); len(got) != 3 || got[0] != "1" || got[2] != "3" {
		t.Errorf("bookmarks = %v, want 1,2,3", got)
	}
	if result.PagesCompleted != 2 {
		t.Errorf("PagesCompleted = %d, want 2", result.PagesCompleted)
	}

	// Ranks are global and zero-based.
	for i, bookmark := range result.Bookmarks {
		if bookmark.TimelineRank == nil || *bookmark.TimelineRank != i {
			t.Errorf("bookmark %d rank = %v, want %d", i, bookmark.TimelineRank, i)
		}
	}
}

// TestPaginateReachingTheLimitIsNotCompletion is the regression test for a live
// defect: breaking on the cap without saying so records a truncated first sync as
// complete, and the remaining history is never fetched.
func TestPaginateReachingTheLimitIsNotCompletion(t *testing.T) {
	fetch := queueFetcher(t, []step{
		{expectCursor: "", page: ports.Page{Items: []domain.Bookmark{bm("1"), bm("2"), bm("3")}, NextCursor: "c2"}},
	})

	result, err := Paginate(context.Background(), Options{FetchPage: fetch, Limit: 3})
	if err != nil {
		t.Fatalf("Paginate: %v", err)
	}

	if result.Termination != TermHitLimit {
		t.Errorf("Termination = %q, want %q so the caller does not mark a full sync complete",
			result.Termination, TermHitLimit)
	}
	if domain.CompletesFullSync(result.Termination, result.FailedCount, result.Unrecognized) {
		t.Error("hitting the cap must never count as a completed full sync")
	}
}

func TestPaginateLimitTruncatesTheFinalPage(t *testing.T) {
	fetch := queueFetcher(t, []step{
		{expectCursor: "", page: ports.Page{Items: []domain.Bookmark{bm("1"), bm("2"), bm("3")}, NextCursor: "c2"}},
	})

	result, err := Paginate(context.Background(), Options{FetchPage: fetch, Limit: 2})
	if err != nil {
		t.Fatalf("Paginate: %v", err)
	}
	if len(result.Bookmarks) != 2 {
		t.Errorf("got %d bookmarks, want exactly the limit of 2", len(result.Bookmarks))
	}
	if result.Termination != TermHitLimit {
		t.Errorf("Termination = %q, want %q", result.Termination, TermHitLimit)
	}
}

// TestPaginateDetectsCursorLoops covers a request that could repeat forever:
// comparing only against the immediately previous cursor would let a cycle of three
// or more cursors run without terminating.
func TestPaginateDetectsCursorLoops(t *testing.T) {
	// The same cursor back is the end of the walk, not a loop.
	//
	// A repeated cursor breaks the walk with no error. Measured on a real walk: this is
	// where X ends a 9,344-bookmark timeline, so calling it a schema change filed every
	// complete full sync as a failure and left the first-full-sync flag unset.
	t.Run("the same cursor twice ends the walk", func(t *testing.T) {
		fetch := queueFetcher(t, []step{
			{expectCursor: "", page: ports.Page{Items: []domain.Bookmark{bm("1")}, NextCursor: "same"}},
			{expectCursor: "same", page: ports.Page{Items: []domain.Bookmark{bm("2")}, NextCursor: "same"}},
		})

		result, err := Paginate(context.Background(), Options{FetchPage: fetch})
		if err != nil {
			t.Fatalf("err = %v, want the walk to end cleanly", err)
		}
		if result.Termination != TermEndOfPages {
			t.Errorf("Termination = %q, want %q", result.Termination, TermEndOfPages)
		}
		if len(result.Bookmarks) != 2 {
			t.Errorf("got %d bookmarks, want both pages", len(result.Bookmarks))
		}
		if result.FailedCount != 0 {
			t.Errorf("FailedCount = %d, want 0: nothing failed", result.FailedCount)
		}
	})

	t.Run("a cycle through several cursors", func(t *testing.T) {
		fetch := queueFetcher(t, []step{
			{expectCursor: "", page: ports.Page{Items: []domain.Bookmark{bm("1")}, NextCursor: "a"}},
			{expectCursor: "a", page: ports.Page{Items: []domain.Bookmark{bm("2")}, NextCursor: "b"}},
			{expectCursor: "b", page: ports.Page{Items: []domain.Bookmark{bm("3")}, NextCursor: "a"}},
		})

		result, err := Paginate(context.Background(), Options{FetchPage: fetch})
		if !errors.Is(err, ErrPaginationLoop) {
			t.Fatalf("err = %v, want ErrPaginationLoop for an A->B->A cycle", err)
		}
		if result.Termination != TermFailed {
			t.Errorf("Termination = %q, want %q", result.Termination, TermFailed)
		}
		// The pages already read are kept, so a retry does not start over.
		if len(result.Bookmarks) != 3 {
			t.Errorf("got %d bookmarks, want the 3 already fetched", len(result.Bookmarks))
		}
	})
}

func TestPaginateEarlyStopsAfterConsecutiveKnownPages(t *testing.T) {
	known := map[string]bool{"1": true, "2": true, "3": true}

	fetch := queueFetcher(t, []step{
		{expectCursor: "", page: ports.Page{Items: []domain.Bookmark{bm("1")}, NextCursor: "c1"}},
		{expectCursor: "c1", page: ports.Page{Items: []domain.Bookmark{bm("2")}, NextCursor: "c2"}},
		{expectCursor: "c2", page: ports.Page{Items: []domain.Bookmark{bm("3")}, NextCursor: "c3"}},
	})

	result, err := Paginate(context.Background(), Options{
		FetchPage:                      fetch,
		StopAfterConsecutiveKnownPages: 3,
		IsExistingBookmark:             func(b domain.Bookmark) bool { return known[b.TweetID] },
	})
	if err != nil {
		t.Fatalf("Paginate: %v", err)
	}
	if result.Termination != TermStoppedEarly {
		t.Errorf("Termination = %q, want %q", result.Termination, TermStoppedEarly)
	}
	if result.PagesCompleted != 3 {
		t.Errorf("PagesCompleted = %d, want 3", result.PagesCompleted)
	}
}

func TestPaginateResetsTheKnownPageStreakOnNewData(t *testing.T) {
	known := map[string]bool{"1": true, "3": true, "4": true, "5": true}

	fetch := queueFetcher(t, []step{
		{expectCursor: "", page: ports.Page{Items: []domain.Bookmark{bm("1")}, NextCursor: "c1"}},
		{expectCursor: "c1", page: ports.Page{Items: []domain.Bookmark{bm("2")}, NextCursor: "c2"}}, // new: resets
		{expectCursor: "c2", page: ports.Page{Items: []domain.Bookmark{bm("3")}, NextCursor: "c3"}},
		{expectCursor: "c3", page: ports.Page{Items: []domain.Bookmark{bm("4")}, NextCursor: "c4"}},
		{expectCursor: "c4", page: ports.Page{Items: []domain.Bookmark{bm("5")}, NextCursor: "c5"}},
	})

	result, err := Paginate(context.Background(), Options{
		FetchPage:                      fetch,
		StopAfterConsecutiveKnownPages: 3,
		IsExistingBookmark:             func(b domain.Bookmark) bool { return known[b.TweetID] },
	})
	if err != nil {
		t.Fatalf("Paginate: %v", err)
	}
	if result.Termination != TermStoppedEarly {
		t.Errorf("Termination = %q, want %q: the streak must restart after new data",
			result.Termination, TermStoppedEarly)
	}
	if result.PagesCompleted != 5 {
		t.Errorf("PagesCompleted = %d, want 5", result.PagesCompleted)
	}
}

func TestPaginateRefusesEarlyStopWithoutPredicate(t *testing.T) {
	_, err := Paginate(context.Background(), Options{
		FetchPage:                      queueFetcher(t, nil),
		StopAfterConsecutiveKnownPages: 3,
	})
	if err == nil {
		t.Fatal("the early-stop rule is unusable without a predicate, so it must be refused rather than silently inert")
	}
}

// TestPaginateCancellation covers stopping a run: the user can cancel, and the
// walk ends at a terminal state.
func TestPaginateCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	fetch := func(_ context.Context, _ string) (ports.Page, error) {
		cancel() // the user cancels while the first page is in flight
		return ports.Page{Items: []domain.Bookmark{bm("1")}, NextCursor: "c2"}, nil
	}

	result, err := Paginate(ctx, Options{FetchPage: fetch})
	if !errors.Is(err, ErrCancelled) {
		t.Fatalf("err = %v, want ErrCancelled", err)
	}
	if result.Termination != TermCancelled {
		t.Errorf("Termination = %q, want %q", result.Termination, TermCancelled)
	}
	if !domain.CompletesFullSync(TermEndOfPages, 0, 0) || domain.CompletesFullSync(result.Termination, 0, 0) {
		t.Error("a cancelled run must not mark a full sync complete")
	}
	// The page that arrived before the cancel leaves a place to resume. A cancel
	// that threw the cursor away restarted the whole walk next time.
	if result.NextCursor != "c2" {
		t.Errorf("NextCursor = %q, want c2 so the next run continues where this one stopped", result.NextCursor)
	}
}

func TestPaginateRetriesTransientFailures(t *testing.T) {
	var slept []time.Duration

	fetch := queueFetcher(t, []step{
		{expectCursor: "", err: newError(domain.ErrNetwork, "fetch", 503, nil)},
		{expectCursor: "", err: newError(domain.ErrRateLimited, "fetch", 429, nil)},
		{expectCursor: "", page: ports.Page{Items: []domain.Bookmark{bm("1")}, NextCursor: ""}},
	})

	result, err := Paginate(context.Background(), Options{
		FetchPage: fetch,
		Sleep:     instantSleep(&slept),
		Retry: RetryPolicy{
			MaxAttempts: 4, BaseDelay: time.Second, MaxDelay: 10 * time.Second,
			MaxRetryAfter: time.Minute,
		},
	})
	if err != nil {
		t.Fatalf("Paginate: %v", err)
	}
	if len(result.Bookmarks) != 1 {
		t.Errorf("got %d bookmarks, want 1 after the retries succeeded", len(result.Bookmarks))
	}
	if len(slept) != 2 {
		t.Errorf("slept %d times, want 2", len(slept))
	}
	// Exponential: 1s then 2s.
	if len(slept) == 2 && (slept[0] != time.Second || slept[1] != 2*time.Second) {
		t.Errorf("delays = %v, want [1s 2s]", slept)
	}
}

// TestPaginateDoesNotRetryCredentialFailures states the rule the spec is explicit
// about: an invalid credential can never succeed, so retrying it is pointless and
// looping on it is forbidden.
func TestPaginateDoesNotRetryCredentialFailures(t *testing.T) {
	var slept []time.Duration
	calls := 0

	fetch := func(_ context.Context, _ string) (ports.Page, error) {
		calls++
		return ports.Page{}, newError(domain.ErrUnauthorized, "fetch", 401, nil)
	}

	result, err := Paginate(context.Background(), Options{
		FetchPage: fetch,
		Sleep:     instantSleep(&slept),
	})
	if err == nil {
		t.Fatal("expected a failure")
	}
	if calls != 1 {
		t.Errorf("fetch was called %d times, want exactly 1", calls)
	}
	if len(slept) != 0 {
		t.Errorf("slept %v, want no waiting before giving up", slept)
	}
	if result.Termination != TermFailed {
		t.Errorf("Termination = %q, want %q", result.Termination, TermFailed)
	}
}

func TestPaginateHonoursRetryAfter(t *testing.T) {
	var slept []time.Duration

	fetch := queueFetcher(t, []step{
		{expectCursor: "", err: &Error{Kind: domain.ErrRateLimited, Status: 429, RetryAfter: 9 * time.Second}},
		{expectCursor: "", page: ports.Page{Items: []domain.Bookmark{bm("1")}}},
	})

	_, err := Paginate(context.Background(), Options{
		FetchPage: fetch,
		Sleep:     instantSleep(&slept),
		Retry: RetryPolicy{
			MaxAttempts: 3, BaseDelay: time.Second, MaxDelay: 30 * time.Second,
			MaxRetryAfter: time.Minute,
		},
	})
	if err != nil {
		t.Fatalf("Paginate: %v", err)
	}
	if len(slept) != 1 || slept[0] != 9*time.Second {
		t.Errorf("delays = %v, want the server-requested 9s rather than the backoff", slept)
	}
}

// TestPaginateReportsAnAbsurdRetryAfter covers the deliberate choice in the spec:
// a header asking us to wait longer than the cap is reported rather than slept
// through, so it cannot park the app.
func TestPaginateReportsAnAbsurdRetryAfter(t *testing.T) {
	var slept []time.Duration

	fetch := queueFetcher(t, []step{
		{expectCursor: "", err: &Error{Kind: domain.ErrRateLimited, Status: 429, RetryAfter: time.Hour}},
	})

	_, err := Paginate(context.Background(), Options{
		FetchPage: fetch,
		Sleep:     instantSleep(&slept),
		Retry: RetryPolicy{
			MaxAttempts: 5, BaseDelay: time.Second, MaxDelay: 30 * time.Second,
			MaxRetryAfter: 30 * time.Second,
		},
	})
	if err == nil {
		t.Fatal("expected the absurd wait to be reported")
	}
	if len(slept) != 0 {
		t.Errorf("slept %v, want none: an hour-long wait must be reported, not slept", slept)
	}
	if got := RetryAfter(err); got != time.Hour {
		t.Errorf("RetryAfter = %v, want the value surfaced to the caller", got)
	}
}

func TestPaginateKeepsEarlierPagesWhenRetriesAreExhausted(t *testing.T) {
	var slept []time.Duration

	fetch := queueFetcher(t, []step{
		{expectCursor: "", page: ports.Page{Items: []domain.Bookmark{bm("1"), bm("2")}, NextCursor: "c2"}},
		{expectCursor: "c2", err: newError(domain.ErrNetwork, "fetch", 500, nil)},
		{expectCursor: "c2", err: newError(domain.ErrNetwork, "fetch", 500, nil)},
	})

	result, err := Paginate(context.Background(), Options{
		FetchPage: fetch,
		Sleep:     instantSleep(&slept),
		Retry: RetryPolicy{
			MaxAttempts: 2, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond,
			MaxRetryAfter: time.Minute,
		},
	})
	if err == nil {
		t.Fatal("expected a failure once the retries ran out")
	}
	if result.Termination != TermFailed {
		t.Errorf("Termination = %q, want %q", result.Termination, TermFailed)
	}
	if len(result.Bookmarks) != 2 {
		t.Errorf("got %d bookmarks, want the 2 already fetched so a retry resumes", len(result.Bookmarks))
	}
	if result.FailedCount != 1 {
		t.Errorf("FailedCount = %d, want 1", result.FailedCount)
	}
	// The page after the good one is where the walk stopped, and it is kept: a
	// 9,000-bookmark walk that died at page 450 used to restart from the top and
	// fail at the same page again.
	if result.NextCursor != "c2" {
		t.Errorf("NextCursor = %q, want c2 so the next run continues rather than restarts", result.NextCursor)
	}
}

// TestPaginateDropsTheCursorItWasLoopingOn is the exception to the rule above: a
// repeated cursor is a cycle, and resuming from it would repeat the cycle.
func TestPaginateDropsTheCursorItWasLoopingOn(t *testing.T) {
	fetch := queueFetcher(t, []step{
		{expectCursor: "", page: ports.Page{Items: []domain.Bookmark{bm("1")}, NextCursor: "c2"}},
		{expectCursor: "c2", page: ports.Page{Items: []domain.Bookmark{bm("2")}, NextCursor: "c3"}},
		{expectCursor: "c3", page: ports.Page{Items: []domain.Bookmark{bm("3")}, NextCursor: "c2"}},
	})

	result, err := Paginate(context.Background(), Options{FetchPage: fetch})
	if err == nil {
		t.Fatal("expected the repeated cursor to be reported")
	}
	if result.NextCursor != "" {
		t.Errorf("NextCursor = %q, want empty: resuming from a cycling cursor loops again", result.NextCursor)
	}
}

// TestPaginateCountsUnrecognizedEntries checks that what a page could not read
// reaches the caller, so a run with unreadable posts is not recorded as clean.
func TestPaginateCountsUnrecognizedEntries(t *testing.T) {
	fetch := queueFetcher(t, []step{
		{expectCursor: "", page: ports.Page{Items: []domain.Bookmark{bm("1")}, NextCursor: "c2", Unrecognized: 2}},
		{expectCursor: "c2", page: ports.Page{Items: []domain.Bookmark{bm("2")}, Unrecognized: 1}},
	})

	result, err := Paginate(context.Background(), Options{FetchPage: fetch})
	if err != nil {
		t.Fatalf("Paginate: %v", err)
	}
	if result.Unrecognized != 3 {
		t.Errorf("Unrecognized = %d, want 3 summed over the walk", result.Unrecognized)
	}
}

// TestPaginateWaitsOutARateLimitWindow covers the long wait: X states its reset
// in minutes, which is far past an ordinary Retry-After cap. Refusing the wait
// kills the run and the next one starts over; the rate-limit budget is what
// makes the wait possible without letting a bogus header park the app.
func TestPaginateWaitsOutARateLimitWindow(t *testing.T) {
	var slept []time.Duration

	fetch := queueFetcher(t, []step{
		{expectCursor: "", err: &Error{Kind: domain.ErrRateLimited, Status: 429, RetryAfter: 5 * time.Minute}},
		{expectCursor: "", page: ports.Page{Items: []domain.Bookmark{bm("1")}}},
	})

	result, err := Paginate(context.Background(), Options{
		FetchPage: fetch,
		Sleep:     instantSleep(&slept),
		Retry: RetryPolicy{
			MaxAttempts: 3, BaseDelay: time.Second, MaxDelay: 30 * time.Second,
			MaxRetryAfter: 30 * time.Second, MaxRateLimitWait: 15 * time.Minute,
		},
	})
	if err != nil {
		t.Fatalf("Paginate: %v", err)
	}
	if len(result.Bookmarks) != 1 {
		t.Errorf("got %d bookmarks, want the page fetched after the window", len(result.Bookmarks))
	}
	if len(slept) != 1 || slept[0] != 5*time.Minute {
		t.Errorf("delays = %v, want the 5m window to be waited out", slept)
	}
}

// TestPaginateStopsWhenTheRateLimitBudgetRunsOut bounds that wait.
func TestPaginateStopsWhenTheRateLimitBudgetRunsOut(t *testing.T) {
	var slept []time.Duration

	fetch := queueFetcher(t, []step{
		{expectCursor: "", err: &Error{Kind: domain.ErrRateLimited, Status: 429, RetryAfter: 10 * time.Minute}},
		{expectCursor: "", err: &Error{Kind: domain.ErrRateLimited, Status: 429, RetryAfter: 10 * time.Minute}},
	})

	_, err := Paginate(context.Background(), Options{
		FetchPage: fetch,
		Sleep:     instantSleep(&slept),
		Retry: RetryPolicy{
			MaxAttempts: 4, BaseDelay: time.Second, MaxDelay: 30 * time.Second,
			MaxRetryAfter: 30 * time.Second, MaxRateLimitWait: 12 * time.Minute,
		},
	})
	if err == nil {
		t.Fatal("expected the run to stop once the window exceeded the budget")
	}
	if len(slept) != 1 || slept[0] != 10*time.Minute {
		t.Errorf("delays = %v, want one 10m wait and then the error", slept)
	}
}

func TestPaginateReportsProgress(t *testing.T) {
	var progress []Progress

	fetch := queueFetcher(t, []step{
		{expectCursor: "", page: ports.Page{Items: []domain.Bookmark{bm("1")}, NextCursor: "c2"}},
		{expectCursor: "c2", page: ports.Page{Items: []domain.Bookmark{bm("2")}}},
	})

	_, err := Paginate(context.Background(), Options{
		FetchPage:  fetch,
		OnProgress: func(p Progress) { progress = append(progress, p) },
	})
	if err != nil {
		t.Fatalf("Paginate: %v", err)
	}
	if len(progress) != 2 {
		t.Fatalf("got %d progress reports, want 2", len(progress))
	}
	// The counts a UI needs to explain a run are all present.
	last := progress[len(progress)-1]
	if last.Fetched != 2 || last.PagesCompleted != 2 || last.Page != 2 {
		t.Errorf("final progress = %+v, want 2 fetched over 2 pages", last)
	}
}

func TestPaginateRequiresAFetcher(t *testing.T) {
	if _, err := Paginate(context.Background(), Options{}); err == nil {
		t.Fatal("Paginate without a fetcher must be refused")
	}
}

func TestPaginateEmptyFirstPageEndsCleanly(t *testing.T) {
	fetch := queueFetcher(t, []step{
		{expectCursor: "", page: ports.Page{Items: nil, NextCursor: ""}},
	})

	result, err := Paginate(context.Background(), Options{FetchPage: fetch})
	if err != nil {
		t.Fatalf("Paginate: %v", err)
	}
	if result.Termination != TermEndOfPages {
		t.Errorf("Termination = %q, want %q", result.Termination, TermEndOfPages)
	}
	if len(result.Bookmarks) != 0 {
		t.Errorf("got %d bookmarks, want none", len(result.Bookmarks))
	}
	// An empty timeline that ran to the end is a legitimate completed full sync.
	if !domain.CompletesFullSync(result.Termination, result.FailedCount, result.Unrecognized) {
		t.Error("an empty library that paginated cleanly is a completed full sync")
	}
}

// TestPaceSpacesTheRequests is the rate-limit guard: a long walk must not send hundreds
// of requests back to back.
//
// Measured here: a walk with no pause hit HTTP 429 at page 101. The wait is reported as
// a WaitingUntil so the UI can show a countdown rather than a frozen bar.
func TestPaceSpacesTheRequests(t *testing.T) {
	var waits []time.Duration
	var waitingReported int

	fetch := queueFetcher(t, []step{
		{expectCursor: "", page: ports.Page{Items: []domain.Bookmark{bm("1")}, NextCursor: "a"}},
		{expectCursor: "a", page: ports.Page{Items: []domain.Bookmark{bm("2")}, NextCursor: "b"}},
		{expectCursor: "b", page: ports.Page{Items: []domain.Bookmark{bm("3")}}},
	})

	result, err := Paginate(context.Background(), Options{
		FetchPage: fetch,
		Pace:      250 * time.Millisecond,
		Sleep: func(_ context.Context, d time.Duration) error {
			waits = append(waits, d)
			return nil
		},
		OnProgress: func(p Progress) {
			if !p.WaitingUntil.IsZero() {
				waitingReported++
			}
		},
	})
	if err != nil {
		t.Fatalf("Paginate: %v", err)
	}
	if result.Termination != TermEndOfPages {
		t.Errorf("Termination = %q, want %q", result.Termination, TermEndOfPages)
	}

	// Three pages, so two gaps: there is nothing to wait for after the last one.
	if len(waits) != 2 {
		t.Fatalf("waited %d times (%v), want 2", len(waits), waits)
	}
	for _, d := range waits {
		if d != 250*time.Millisecond {
			t.Errorf("wait = %s, want the configured pace", d)
		}
	}
	if waitingReported < 2 {
		t.Errorf("the wait was reported %d times, want one per gap", waitingReported)
	}
}

// TestNoPaceMeansNoWait keeps the default honest: a caller that did not ask for pacing
// must not get a sleep, or every existing test and every retry becomes slower for no
// reason.
func TestNoPaceMeansNoWait(t *testing.T) {
	waits := 0
	fetch := queueFetcher(t, []step{
		{expectCursor: "", page: ports.Page{Items: []domain.Bookmark{bm("1")}, NextCursor: "a"}},
		{expectCursor: "a", page: ports.Page{Items: []domain.Bookmark{bm("2")}}},
	})

	if _, err := Paginate(context.Background(), Options{
		FetchPage: fetch,
		Sleep: func(context.Context, time.Duration) error {
			waits++
			return nil
		},
	}); err != nil {
		t.Fatalf("Paginate: %v", err)
	}
	if waits != 0 {
		t.Errorf("slept %d times without a pace", waits)
	}
}

// TestOnPageHandsOverEachPageAsItArrives covers the streaming seam.
//
// The sync service used to receive the whole walk in Result and write it once at
// the end, which is why a first full sync showed an empty window for minutes. The
// pages have to be handed over during the walk, in order, each carrying its place
// in the timeline -- the rank is written with the page and cannot be recomputed
// later without rewriting the library.
func TestOnPageHandsOverEachPageAsItArrives(t *testing.T) {
	type handed struct {
		page  int
		ids   []string
		ranks []int
	}
	var got []handed

	fetch := queueFetcher(t, []step{
		{expectCursor: "", page: ports.Page{Items: []domain.Bookmark{bm("1"), bm("2")}, NextCursor: "c2"}},
		{expectCursor: "c2", page: ports.Page{Items: []domain.Bookmark{bm("3")}}},
	})

	if _, err := Paginate(context.Background(), Options{
		FetchPage: fetch,
		OnPage: func(page int, bookmarks []domain.Bookmark) error {
			ranks := make([]int, len(bookmarks))
			for i, bookmark := range bookmarks {
				if bookmark.TimelineRank != nil {
					ranks[i] = *bookmark.TimelineRank
				} else {
					ranks[i] = -1
				}
			}
			got = append(got, handed{page: page, ids: ids(bookmarks), ranks: ranks})
			return nil
		},
	}); err != nil {
		t.Fatalf("Paginate: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("OnPage saw %d pages, want 2", len(got))
	}
	if got[0].page != 1 || strings.Join(got[0].ids, ",") != "1,2" {
		t.Errorf("first page = %+v, want page 1 with 1,2", got[0])
	}
	if got[1].page != 2 || strings.Join(got[1].ids, ",") != "3" {
		t.Errorf("second page = %+v, want page 2 with 3", got[1])
	}
	if fmt.Sprint(got[0].ranks) != "[0 1]" || fmt.Sprint(got[1].ranks) != "[2]" {
		t.Errorf("ranks = %v then %v, want [0 1] then [2]", got[0].ranks, got[1].ranks)
	}
}

// TestAnOnPageFailureEndsTheRunAndKeepsTheEarlierPages is the storage-failure case.
//
// A page that cannot be written stops the run, but the pages before it were already
// handed over -- which is the reason to hand them over at all -- and the result says
// why it stopped rather than pretending the walk reached the end.
func TestAnOnPageFailureEndsTheRunAndKeepsTheEarlierPages(t *testing.T) {
	storeErr := errors.New("disk is full")
	written := 0

	fetch := queueFetcher(t, []step{
		{expectCursor: "", page: ports.Page{Items: []domain.Bookmark{bm("1")}, NextCursor: "c2"}},
		{expectCursor: "c2", page: ports.Page{Items: []domain.Bookmark{bm("2")}}},
	})

	result, err := Paginate(context.Background(), Options{
		FetchPage: fetch,
		OnPage: func(page int, bookmarks []domain.Bookmark) error {
			if page == 2 {
				return storeErr
			}
			written += len(bookmarks)
			return nil
		},
	})
	if !errors.Is(err, storeErr) {
		t.Fatalf("err = %v, want the store error", err)
	}
	if result.Termination != TermFailed {
		t.Errorf("Termination = %q, want %q", result.Termination, TermFailed)
	}
	if !errors.Is(result.PageError, storeErr) {
		t.Errorf("PageError = %v, want the store error", result.PageError)
	}
	if written != 1 {
		t.Errorf("wrote %d bookmarks, want the 1 from the page before the failure", written)
	}
}
