package workspace

import (
	"context"
	"github.com/taoquo/trove/internal/domain"
	"sort"
)

// BookmarkRow is a bookmark with the relations the results list shows.
type BookmarkRow struct {
	domain.Bookmark
	Tags []domain.Tag
	List domain.List
}

// View is one rendered state of the workspace.
type View struct {
	// Generation identifies this result. The UI compares it against the newest
	// generation before displaying, so a slow earlier query cannot overwrite a
	// newer one after the user changed the filter again.
	Generation uint64

	Query     domain.LibraryQuery
	Bookmarks []BookmarkRow
	// Total is how many bookmarks match, ignoring the window, so the UI can say
	// "showing 100 of 4321" instead of silently truncating.
	Total   int
	HasMore bool

	Counts  domain.WorkspaceCounts
	Lists   []domain.ListSummary
	Tags    []domain.TagSummary
	Authors []domain.AuthorSummary
	// FacetCounts is what each tag and author row would match under Query's conditions.
	// It is loaded with the view rather than on demand so a sidebar row and the list it
	// opens are always computed from the same filter.
	FacetCounts domain.FacetCounts
}

// SidebarLimit caps the author list; tags and lists are all shown.
const SidebarLimit = 50

// MatchingIDs returns the ids of every bookmark the query matches, not just the page.
//
// The bulk bar's "select all results" needs the whole set, and Load returns a window. The
// window is widened to the match count rather than to MaxLibraryPageSize, so a small result
// set does not ask the database for a thousand rows.
//
// A match count above MaxLibraryPageSize cannot be returned in one query; the cap is
// reported rather than silently truncating, because a bulk action that quietly skipped the
// tail would delete or tag a set the user did not choose.
func (s *Service) MatchingIDs(ctx context.Context, query domain.LibraryQuery) ([]string, error) {
	if s.repos.Bookmarks == nil {
		return nil, nil
	}
	total := query
	total.Limit = domain.MaxLibraryPageSize
	total.Offset = 0
	rows, err := s.repos.Bookmarks.Query(ctx, total)
	if err != nil {
		return nil, err
	}
	if len(rows) == domain.MaxLibraryPageSize {
		return nil, ErrTooManyToSelect
	}
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.TweetID)
	}
	sort.Strings(ids)
	return ids, nil
}

// Load builds a view.
func (s *Service) Load(ctx context.Context, query domain.LibraryQuery) (View, error) {
	normalized, err := query.Normalize()
	if err != nil {
		return View{}, err
	}

	bookmarks, err := s.repos.Bookmarks.Query(ctx, normalized)
	if err != nil {
		return View{}, err
	}
	total, err := s.repos.Bookmarks.Count(ctx, normalized.Filter)
	if err != nil {
		return View{}, err
	}

	view := View{
		Generation: s.generation.Add(1),
		Query:      normalized,
		Total:      total,
		HasMore:    normalized.Offset+len(bookmarks) < total,
	}

	view.Bookmarks, err = s.decorate(ctx, bookmarks)
	if err != nil {
		return View{}, err
	}

	if err := s.loadSidebar(ctx, &view); err != nil {
		return View{}, err
	}

	return view, nil
}

// LoadPage reads one page of results without the sidebar.
//
// It is Load for a list that is already on screen: a second page changes which bookmarks
// are shown, not which tags, lists or authors exist. Those summaries and counts are the
// larger half of a read -- about 60% of a 50,000-bookmark Load -- and recomputing them for
// every page of one scrolling session is work whose
// answer the caller already holds.
//
// The Generation is the service's current one, not a new one: a page belongs to the query
// the reader is on, and IsCurrent still rejects it if a newer query started meanwhile.
func (s *Service) LoadPage(ctx context.Context, query domain.LibraryQuery) (View, error) {
	normalized, err := query.Normalize()
	if err != nil {
		return View{}, err
	}

	bookmarks, err := s.repos.Bookmarks.Query(ctx, normalized)
	if err != nil {
		return View{}, err
	}
	total, err := s.repos.Bookmarks.Count(ctx, normalized.Filter)
	if err != nil {
		return View{}, err
	}

	view := View{
		Generation: s.generation.Load(),
		Query:      normalized,
		Total:      total,
		HasMore:    normalized.Offset+len(bookmarks) < total,
	}
	view.Bookmarks, err = s.decorate(ctx, bookmarks)
	if err != nil {
		return View{}, err
	}
	return view, nil
}

// decorate attaches the tags and the list to each bookmark.
func (s *Service) decorate(ctx context.Context, bookmarks []domain.Bookmark) ([]BookmarkRow, error) {
	rows := make([]BookmarkRow, 0, len(bookmarks))
	if len(bookmarks) == 0 {
		return rows, nil
	}

	ids := make([]string, 0, len(bookmarks))
	for _, bookmark := range bookmarks {
		ids = append(ids, bookmark.TweetID)
	}

	tagsByBookmark, err := s.repos.Bookmarks.TagsByBookmark(ctx, ids)
	if err != nil {
		return nil, err
	}
	listsByBookmark, err := s.repos.Bookmarks.ListByBookmark(ctx, ids)
	if err != nil {
		return nil, err
	}

	for _, bookmark := range bookmarks {
		rows = append(rows, BookmarkRow{
			Bookmark: bookmark,
			Tags:     tagsByBookmark[bookmark.TweetID],
			List:     listsByBookmark[bookmark.TweetID],
		})
	}
	return rows, nil
}

// sidebarCache is the half of a view that changes only when the library does: the counts
// and the three sidebar sections. A change of filter does not move any of them, and they
// are about 60% of a 50,000-bookmark read.
type sidebarCache struct {
	valid    bool
	revision int64
	counts   domain.WorkspaceCounts
	lists    []domain.ListSummary
	tags     []domain.TagSummary
	authors  []domain.AuthorSummary
}

// loadSidebar fills the counts and the three sidebar sections.
//
// The aggregates are reused while the database's write revision is unchanged. The
// revision is read before they are, never after: a write that lands mid-read then leaves
// a cache stamped with an older number, which the next read rejects. The other order
// could stamp new data with an old revision, and that is the one mistake that would show
// stale numbers.
func (s *Service) loadSidebar(ctx context.Context, view *View) error {
	revision, hasRevision := s.libraryRevision(ctx)
	if hasRevision {
		s.sidebarMu.Lock()
		cached := s.sidebar.valid && s.sidebar.revision == revision
		if cached {
			view.Counts = s.sidebar.counts
			view.Lists = append([]domain.ListSummary(nil), s.sidebar.lists...)
			view.Tags = append([]domain.TagSummary(nil), s.sidebar.tags...)
			view.Authors = append([]domain.AuthorSummary(nil), s.sidebar.authors...)
		}
		s.sidebarMu.Unlock()
		if cached {
			return s.loadFacetCounts(ctx, view)
		}
	}

	counts, err := s.repos.Bookmarks.Counts(ctx)
	if err != nil {
		return err
	}
	view.Counts = counts

	if view.Lists, err = s.repos.Lists.Summaries(ctx); err != nil {
		return err
	}
	if view.Tags, err = s.repos.Tags.Summaries(ctx); err != nil {
		return err
	}
	if view.Authors, err = s.repos.Bookmarks.AuthorSummaries(ctx, SidebarLimit); err != nil {
		return err
	}

	if hasRevision {
		s.sidebarMu.Lock()
		s.sidebar = sidebarCache{
			valid:    true,
			revision: revision,
			counts:   view.Counts,
			lists:    append([]domain.ListSummary(nil), view.Lists...),
			tags:     append([]domain.TagSummary(nil), view.Tags...),
			authors:  append([]domain.AuthorSummary(nil), view.Authors...),
		}
		s.sidebarMu.Unlock()
	}

	return s.loadFacetCounts(ctx, view)
}

// libraryRevision reads the database's write counter, when the adapter has one.
func (s *Service) libraryRevision(ctx context.Context) (int64, bool) {
	if s.repos.Revision == nil {
		return 0, false
	}
	revision, err := s.repos.Revision.Revision(ctx)
	if err != nil {
		// An unreadable revision is not a reason to fail the view: read the aggregates
		// and cache nothing.
		return 0, false
	}
	return revision, true
}

// loadFacetCounts fills what each sidebar row would match under the view's conditions.
func (s *Service) loadFacetCounts(ctx context.Context, view *View) error {
	// With nothing narrowing the library, a row's number is its own summary's: counting
	// again would be two full scans to recompute what is already in hand, on the path that
	// runs every time the workspace opens or a sync finishes.
	if view.Query.Filter.IsZero() {
		view.FacetCounts = domain.FacetCounts{Ready: true, Tags: map[string]int{}, Authors: map[string]int{}}
		for _, tag := range view.Tags {
			view.FacetCounts.Tags[tag.ID] = tag.BookmarkCount
		}
		for _, author := range view.Authors {
			view.FacetCounts.Authors[author.AuthorHandle] = author.BookmarkCount
		}
		return nil
	}
	counts, err := s.repos.Bookmarks.FacetCounts(ctx, view.Query.Filter)
	if err != nil {
		return err
	}
	view.FacetCounts = counts
	return nil
}
