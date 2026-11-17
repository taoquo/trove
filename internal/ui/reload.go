package ui

import (
	"context"
	"slices"
	"time"

	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/i18n"
	"github.com/taoquo/trove/internal/usecase/suggestions"
	"github.com/taoquo/trove/internal/usecase/workspace"
)

// A workspace refresh is one read of everything the page draws: the result page, the
// sidebar's counts, the stored sync result, the saved views and the pending
// suggestions.
//
// It used to run inside the click handler that asked for it. Every filter click, tag
// click, scope switch and post-write refresh therefore stopped the window from drawing
// for as long as SQLite took: on a 50,000-bookmark library, 68 ms to open it and 124 ms
// for a tag filter, multiplied by however many clicks the reader made in a row. Typing in
// the search field was the worst of it: every keystroke ran a query that briefly owns the
// only database connection.
//
// It now runs off the window's thread, and its result is applied through
// Window.Update. Three rules keep that honest:
//
//   - The previous page stays on screen while the next one is computed. A refresh is not
//     a first load: blanking to the skeleton on every keystroke is worse than a page that
//     is 100 ms old.
//   - Only the newest request is applied. Requests coalesce: while a read is in flight,
//     further requests collapse into one "what the reader wants now", so typing eight
//     characters cannot queue eight queries behind each other.
//   - Reload stays synchronous, because a caller that has just written to the database
//     and reads the view back is a statement about the write, not a redraw. Tests use it
//     for the same reason.

// reloadData is one read of the workspace. It holds data only: gathering it touches no
// state, and applying it touches no service, so the two halves can run on different
// threads.
type reloadData struct {
	view workspace.View
	err  error

	summary    domain.SyncSummary
	hasSummary bool
	resume     bool

	savedViews    []domain.SavedView
	savedViewsErr error

	pending []suggestions.PendingRow
}

// reloadQuery is the read under the current filters.
//
// A background refresh is not a first load. When the reader has paged past the first
// window, re-reading only the first page would shorten the list under them and leave the
// scroll offset pointing past its new end, which reads as "the library lost rows". The
// window therefore stays at least as large as what is on screen, up to a single read's
// cap; past that the top of the list stays and scrolling continues the list from there.
//
// A changed condition is the exception: that is a new list, and it starts at the first
// page.
func (s *Shell) reloadQuery() domain.LibraryQuery {
	query := domain.LibraryQuery{Filter: s.filter(), Sort: s.state.Sort}
	// The stored query has been through Normalize, which fills the match modes; comparing
	// it with raw state would say "changed" on every refresh.
	if normalized, err := query.Normalize(); err == nil {
		query = normalized
	}

	limit := domain.DefaultLibraryPageSize
	// sameLibraryQuery ignores the window, so it answers "is this the same list" here.
	if s.state.Loaded && sameLibraryQuery(s.state.Data.Query, query) {
		if loaded := len(s.state.Data.Bookmarks); loaded > limit {
			limit = min(loaded, domain.MaxLibraryPageSize)
		}
	}
	query.Limit = limit
	return query
}

// gatherReload reads the workspace without touching state.
func (s *Shell) gatherReload(ctx context.Context, query domain.LibraryQuery) reloadData {
	var data reloadData

	// Each read is skipped when its service is absent, which is how the render-only
	// tests build a shell.
	if s.service != nil {
		data.view, data.err = s.service.Load(ctx, query)
		data.savedViews, data.savedViewsErr = s.service.SavedViews(ctx)
	}
	if s.syncer != nil {
		data.summary, data.hasSummary, _ = s.syncer.Summary(ctx)
		data.resume = s.syncer.ResumeCursor(ctx) != ""
	}
	if s.suggestions != nil {
		data.pending, _ = s.suggestions.Pending(ctx)
	}
	return data
}

// applyReload installs a gathered read. It runs on the window's thread.
func (s *Shell) applyReload(data reloadData) {
	s.state.Refreshing = false

	// The last sync is read from storage on every load, which is what makes it survive
	// a restart.
	if data.hasSummary {
		s.applySyncSummary(data.summary)
	}
	if s.syncer != nil {
		s.state.SyncResumePending = data.resume
	}
	if data.savedViewsErr != nil {
		s.state.Err = data.savedViewsErr.Error()
	} else if s.service != nil {
		s.state.SavedViews = data.savedViews
	}
	if s.suggestions == nil {
		s.state.PendingSuggestions = nil
	} else {
		s.state.PendingSuggestions = data.pending
	}

	if data.err != nil {
		s.state.Err = data.err.Error()
		// A failed first read must leave the loading skeleton, not keep it: Loaded gated
		// the error card, so the window said "loading" forever with no way to retry.
		s.state.Loaded = true
		// With rows on screen the empty state is not drawn, so it cannot speak for this
		// failure; the status line does, and the last good page stays up rather
		// than blanking to an error card the reader cannot use.
		if len(s.state.Data.Bookmarks) > 0 {
			s.ShowStatus(ToastError, s.t().T(i18n.LoadFailedTitle))
		}
		return
	}
	if s.service == nil {
		s.state.Loaded = true
		return
	}

	// A result that is no longer the newest is discarded rather than shown. The async
	// path guards this with the request sequence; this second check also covers the
	// synchronous callers, whose reload can follow a write that bumped the generation.
	if !s.service.IsCurrent(data.view.Generation) {
		return
	}

	// A new set of conditions is a new list: it starts at the first page, at the top, and
	// with the grid's automatic page anchors cleared. Keeping the previous window's limit
	// drew 400 cards for a filter that had never been scrolled, and keeping the offset
	// showed the middle of a list the reader had not seen the start of.
	queryChanged := s.state.Loaded && !sameLibraryQuery(s.state.Data.Query, data.view.Query)
	if queryChanged {
		s.state.RenderLimit = ResultsRenderLimit
		s.state.autoLoadY, s.state.renderGrewY = -1, -1
		s.resultsScroll.Y = 0
	}

	s.state.Data = data.view
	// Only a new list re-answers "what is selected". A refresh of the same list that comes
	// back shorter -- a row removed elsewhere, a page that did not refill -- must not
	// silently drop ids the reader picked: they are still their selection, and the batch
	// actions still name them.
	s.keepSelection(queryChanged)
	s.state.Loaded = true
	s.state.Err = ""

	// A condition naming a tag this read no longer finds has to go -- the chip would print a
	// uuid and the filter would match nothing (filterTagConditions). The delete paths drop it
	// where they know what they deleted; this catches the tags removed by another route: the
	// orphan sweep, an import that replaces the library, or a saved view restored after its
	// tag was deleted. The view in hand was read *with* the stale condition, so one more read
	// is queued; that read finds nothing to drop, so it cannot loop.
	tags := make(map[string]bool, len(data.view.Tags))
	for _, tag := range data.view.Tags {
		tags[tag.ID] = true
	}
	if s.filterTagConditions(func(id string) bool { return tags[id] }) {
		s.ReloadAsync(context.Background())
	}
}

// sameLibraryQuery reports whether two reads answer the same question. Only the conditions,
// the order and the sort matter: the window changes as pages are read, and a page is not a
// new list.
func sameLibraryQuery(a, b domain.LibraryQuery) bool {
	if a.Sort != b.Sort {
		return false
	}
	af, bf := a.Filter, b.Filter
	return af.Query == bf.Query &&
		af.TagMatchMode == bf.TagMatchMode &&
		slices.Equal(af.TagIDs, bf.TagIDs) &&
		af.AuthorMatchMode == bf.AuthorMatchMode &&
		slices.Equal(af.AuthorHandles, bf.AuthorHandles) &&
		af.ListID == bf.ListID &&
		af.OnlyWithPendingSuggestions == bf.OnlyWithPendingSuggestions &&
		af.OnlyUnclassified == bf.OnlyUnclassified &&
		af.OnlyWithMedia == bf.OnlyWithMedia &&
		af.OnlyWithVideo == bf.OnlyWithVideo &&
		af.OnlyLongform == bf.OnlyLongform &&
		af.Archived == bf.Archived
}

// SearchDebounce is how long the search field waits after a keystroke.
//
// A keystroke is not a question. Typing "kubernetes" used to read the library ten times,
// once per character, and each read owns the only database connection for as long as it
// runs -- so the list lagged behind the field it was answering. One short pause per burst
// asks once, for what the reader actually meant.
const SearchDebounce = 150 * time.Millisecond

// searchChanged schedules the read for what the search field says now.
//
// Every keystroke takes a new sequence number; a timer whose number is no longer the
// newest does nothing. That is what makes the debounce one query rather than a queue of
// them, and it also covers the stale case: a read already running for an older query is
// dropped by ReloadAsync's own sequence check.
func (s *Shell) searchChanged() {
	if s.service == nil {
		return
	}
	seq := s.searchSeq.Add(1)
	if !s.state.Searching {
		s.state.Searching = true
		s.invalidate()
	}
	after := s.after
	if after == nil {
		after = time.AfterFunc
	}
	after(SearchDebounce, func() {
		s.publish(func() {
			if s.searchSeq.Load() != seq {
				return
			}
			s.state.Searching = false
			s.ReloadAsync(context.Background())
		})
	})
}

// reloadRequest is one queued refresh: the sequence that identifies it and the query
// captured on the window's thread (state must not be read off it).
type reloadRequest struct {
	seq   uint64
	query domain.LibraryQuery
}

// ReloadAsync refreshes the workspace without stopping the window.
//
// The call itself is cheap: it records what the reader wants now and returns. If a read
// is already in flight its result is still applied — the reader sees the page move — and
// the newest query runs immediately after it; if several requests arrive meanwhile they
// collapse into the last one.
func (s *Shell) ReloadAsync(ctx context.Context) {
	if s.service == nil {
		s.state.Loaded = true
		return
	}

	request := reloadRequest{seq: s.reloadSeq.Add(1), query: s.reloadQuery()}

	s.reloadMu.Lock()
	s.reloadPending = &request
	start := !s.reloadRunning
	s.reloadRunning = true
	s.reloadMu.Unlock()

	if !s.state.Refreshing {
		s.state.Refreshing = true
		s.invalidate()
	}
	if !start {
		return
	}
	s.runInBackground(func() { s.drainReloads(ctx) })
}

// drainReloads runs queued refreshes one at a time until the queue is empty.
func (s *Shell) drainReloads(ctx context.Context) {
	for {
		s.reloadMu.Lock()
		request := s.reloadPending
		s.reloadPending = nil
		if request == nil {
			s.reloadRunning = false
			s.reloadMu.Unlock()
			s.publish(func() {
				// A request can arrive between the loop ending and this frame: it sets
				// the note again itself, and clearing it here would contradict it.
				if s.refreshIdle() {
					s.state.Refreshing = false
					s.invalidate()
				}
			})
			return
		}
		s.reloadMu.Unlock()

		data := s.gatherReload(ctx, request.query)
		s.publish(func() {
			// A newer request is already queued: showing this page would flash a result
			// the reader has moved past, then replace it a frame later.
			if request.seq != s.reloadSeq.Load() {
				return
			}
			s.applyReload(data)
			// applyReload clears the note; with another page queued it is not the end
			// of the refresh, and the note must not blink off between pages.
			if !s.refreshIdle() {
				s.state.Refreshing = true
			}
			s.invalidate()
		})
	}
}

// refreshIdle reports whether no refresh is queued or running.
func (s *Shell) refreshIdle() bool {
	s.reloadMu.Lock()
	defer s.reloadMu.Unlock()
	return !s.reloadRunning && s.reloadPending == nil
}
