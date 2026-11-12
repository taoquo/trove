package ui

import (
	"context"

	"github.com/egoist/mygo/ui"
	"github.com/taoquo/trove/apps/macos/internal/domain"
)

// Reading the next part of a result list.
//
// A page of results is 100 rows and a single read is capped at 1000, so a larger library is
// appended a page at a time. There is no "load more" control: the scroll asks, as the
// reference does, and the two rules below keep that from becoming a frame-by-frame read.
//
//   - one page per reach. The offset a page was asked at is remembered; appending rows
//     does not move it, so the next frame compares equal and waits for the reader to
//     scroll further. A build pass can run several times per frame, and a synchronous
//     read (a test, or an update delivered inline) would otherwise chain through the
//     whole library in one pass.
//   - the drawn rows first. The grid builds 80 cards at a time; the next screenful of
//     rows already in memory comes before another database read.

// autoLoadDistance is how close to the end of the drawn cards the scroll has to be before
// the next screenful is built or the next page read, in DIPs. The source's own value is 320
// (OptionsApp.tsx:240-243).
const autoLoadDistance = 320

// autoLoadMore continues the list when the reader reaches its end.
//
// It is called from the two result views, before they build. That is a write during a build
// pass, which the project avoids -- but MyGo gives a scroll container no event, and the read
// this replaces (a button) is the interaction the product does not want. The rules that make
// it safe are the anchors: every branch fires at most once per scroll offset, and the guards
// (LoadingMore, Refreshing, the render limit) are state the next build in the same pass sees.
// Nothing here runs twice for one offset, whatever the number of builds.
func (s *Shell) autoLoadMore(ctx context.Context) {
	if s.service == nil || !s.state.Loaded || s.state.LoadingMore || s.state.Refreshing {
		return
	}
	// The drawn cards come first: while the reader has not seen the rows already in
	// memory there is no reason to read the database. The list virtualises, so this is
	// the grid's rule only.
	if s.state.ViewMode == domain.ViewGrid &&
		s.state.RenderLimit > 0 && s.state.RenderLimit < len(s.state.Data.Bookmarks) {
		s.growRenderLimit()
		// The screenful that just appeared may not reach the rows already loaded; with
		// none left to draw, the reader is at the end of this page and the next one is
		// read in the same gesture.
		if s.state.RenderLimit < len(s.state.Data.Bookmarks) {
			return
		}
	}
	scroll := s.resultsScrollState()
	if !s.state.Data.HasMore || !scrollNearEnd(scroll) {
		return
	}
	if s.state.autoLoadY == scroll.Y {
		return
	}
	s.state.autoLoadY = scroll.Y
	s.LoadMoreResults(ctx)
}

// resultsScrollState is the scroll container of the result view in use. The grid and the
// list are two containers, so each keeps its own offset; only one of them is on screen.
//
// The list is tracked with TrackScroll like any scroll container. Reading its ListState
// instead (Visible/AtEnd) marks the frame as one that read the list, and MyGo then builds
// another while the visible range is still converging -- which, with row heights being
// measured, is every frame: the tester settled on a page revealed one row at a time.
func (s *Shell) resultsScrollState() *ui.ScrollState {
	if s.state.ViewMode == domain.ViewList {
		return &s.listScroll
	}
	return &s.resultsScroll
}

func scrollNearEnd(scroll *ui.ScrollState) bool {
	return scroll.MaxY > 0 && scroll.Y >= scroll.MaxY-autoLoadDistance
}

// gridNearEnd reports whether the grid's scroll is within autoLoadDistance of its end.
//
// MaxY is set by the previous layout, which is exactly the measurement needed: at the end of
// the content, Y and MaxY are the same. A grid shorter than its pane has MaxY 0 and never
// asks for another page, because there is no "end" the reader can reach.
func (s *Shell) gridNearEnd() bool {
	return scrollNearEnd(&s.resultsScroll)
}

// growRenderLimit builds the next screenful of rows already in memory, once per reach of
// the grid's end. The anchor is separate from autoLoadY's: growing the limit and reading a
// page are different costs and the reader should not need two gestures for the first page.
func (s *Shell) growRenderLimit() {
	if !s.gridNearEnd() || s.state.renderGrewY == s.resultsScroll.Y {
		return
	}
	s.state.renderGrewY = s.resultsScroll.Y
	s.ShowMoreCards()
}

// ShowMoreCards builds the next screenful of an already-loaded result window.
//
// It is separate from LoadMoreResults: that one reads another page from the database, this
// one only draws rows already in memory.
func (s *Shell) ShowMoreCards() {
	total := len(s.state.Data.Bookmarks)
	if s.state.RenderLimit >= total {
		return
	}
	s.state.RenderLimit = min(s.state.RenderLimit+ResultsRenderIncrement, total)
	s.invalidate()
}

// LoadMoreResults extends the result window by one page.
//
// The query layer caps a single read at MaxLibraryPageSize, so a library past that is
// read in pages and appended. A refresh always starts again at the first page: a filter
// change must not keep a window that belonged to another query.
func (s *Shell) LoadMoreResults(ctx context.Context) {
	if s.service == nil || s.state.LoadingMore || !s.state.Data.HasMore {
		return
	}
	current := len(s.state.Data.Bookmarks)
	query := domain.LibraryQuery{Filter: s.filter(), Sort: s.state.Sort}

	// Widening the first window and paging past the cap are the same read with a
	// different slice of it.
	widen := current < domain.MaxLibraryPageSize
	if widen {
		limit := current + domain.DefaultLibraryPageSize
		if limit > domain.MaxLibraryPageSize {
			limit = domain.MaxLibraryPageSize
		}
		query.Limit = limit
	} else {
		query.Limit = domain.DefaultLibraryPageSize
		query.Offset = current
	}

	// A deep page is ~95 ms at 50,000 bookmarks (docs/benchmark-report.md), and it is a
	// click: it runs off the window's thread with the button disabled, so a second click
	// cannot start a second page for the same list.
	s.state.LoadingMore = true
	s.invalidate()
	s.runInBackground(func() {
		// LoadPage, not Load: the sidebar's counts and summaries do not change because
		// the list grew by one page, and they are the larger half of a read.
		next, err := s.service.LoadPage(ctx, query)
		s.publish(func() {
			s.state.LoadingMore = false
			if err != nil {
				s.reportErr(err)
				s.invalidate()
				return
			}
			// The filter changed while this page was being read: appending it would mix
			// two queries in one list.
			if !s.service.IsCurrent(next.Generation) {
				s.invalidate()
				return
			}
			// Merged into the view already on screen, never installed whole: the page has
			// no sidebar in it, and replacing the view would blank the sidebar's counts
			// until the next full read.
			merged := s.state.Data
			if widen {
				merged.Bookmarks = next.Bookmarks
			} else {
				merged.Bookmarks = append(merged.Bookmarks, next.Bookmarks...)
			}
			merged.Query = next.Query
			merged.Generation = next.Generation
			merged.Total = next.Total
			merged.HasMore = len(merged.Bookmarks) < next.Total
			s.state.Data = merged
			// The reader is at the end of what was drawn: the rows that just arrived have
			// to be drawn too, or the page would appear to stop until another gesture.
			s.state.RenderLimit += len(next.Bookmarks)
			s.invalidate()
		})
	})
}
