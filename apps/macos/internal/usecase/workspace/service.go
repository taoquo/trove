// Package workspace is the read model the three-column UI renders.
//
// It exists so the UI never assembles a view out of several stores itself: the
// sidebar counts, the list of results and the per-bookmark tags and list all have
// to agree with each other, and the only way to guarantee that is to build them
// from one snapshot.
package workspace

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/taoquo/trove/apps/macos/internal/domain"
	"github.com/taoquo/trove/apps/macos/internal/ports"
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

// Service builds views and applies user edits.
type Service struct {
	repos ports.Repositories
	clock ports.Clock
	ids   ports.IDGenerator

	// generation is monotonic, so the UI can discard a stale result.
	generation atomic.Uint64
	// mu serialises writes. Two edits to the same bookmark must not interleave.
	mu sync.Mutex
	// sidebarMu guards the sidebar cache; a read and a filter change can overlap.
	sidebarMu sync.Mutex
	sidebar   sidebarCache
}

// New builds a service.
func New(repos ports.Repositories, clock ports.Clock, ids ports.IDGenerator) *Service {
	if clock == nil {
		clock = ports.SystemClock{}
	}
	return &Service{repos: repos, clock: clock, ids: ids}
}

// Load builds a view.
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

// ErrTooManyToSelect is returned when a match set is larger than one query can carry.
var ErrTooManyToSelect = errors.New("workspace: too many bookmarks to select at once")

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
// larger half of a read -- about 60% of a 50,000-bookmark Load (docs/benchmark-report.md
// §4.2) -- and recomputing them for every page of one scrolling session is work whose
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
// are about 60% of a 50,000-bookmark read (docs/benchmark-report.md §4.2).
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

// IsCurrent reports whether a view is still the newest one.
//
// The UI calls this before rendering: a query that started before the user's last
// keystroke must not replace the results of one that started after.
func (s *Service) IsCurrent(generation uint64) bool {
	return generation == s.generation.Load()
}

// ErrUnchanged reports an edit that would change nothing.
var ErrUnchanged = errors.New("workspace: nothing to change")

// AddTags applies tags to bookmarks.
//
// A tag name that does not exist yet is created, and an existing one is reused by
// its canonical key, so "AI Agent" does not become a second tag beside
// "ai-agents".
func (s *Service) AddTags(ctx context.Context, tweetIDs []string, tagNames []string) (int, error) {
	if len(tweetIDs) == 0 || len(tagNames) == 0 {
		return 0, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.clock.Now()
	resolved := make([]domain.Tag, 0, len(tagNames))

	for _, name := range tagNames {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}

		existing, found, err := s.repos.Tags.FindByName(ctx, trimmed)
		if err != nil {
			return 0, err
		}
		if found {
			resolved = append(resolved, existing)
			continue
		}

		display := domain.NormalizeTagDisplayName(trimmed)
		key := domain.CanonicalTagKey(display)
		if key == "" {
			// A name that normalizes away would create an unnamed tag.
			continue
		}

		tag := domain.Tag{
			ID:           s.newID("tag"),
			Name:         display,
			CanonicalKey: key,
			CreatedAt:    domain.FormatTimestamp(now),
			UpdatedAt:    domain.FormatTimestamp(now),
		}
		if err := s.repos.Tags.Create(ctx, tag); err != nil {
			return 0, err
		}
		resolved = append(resolved, tag)
	}

	if len(resolved) == 0 {
		return 0, nil
	}

	relations := make([]domain.BookmarkTag, 0, len(tweetIDs)*len(resolved))
	for _, tweetID := range tweetIDs {
		for _, tag := range resolved {
			relations = append(relations, domain.BookmarkTag{
				BookmarkID: tweetID,
				TagID:      tag.ID,
				CreatedAt:  now,
				// A tag a human applied by hand is manual, so no later automatic
				// pass may remove it.
				Source: domain.TagSourceManual,
			})
		}
	}

	if err := s.repos.Tags.AttachMany(ctx, relations); err != nil {
		return 0, err
	}

	if err := s.bumpGeneration(); err != nil {
		// A stale-version read is recoverable; the write already succeeded.
		_ = err
	}
	return len(relations), nil
}

// RemoveTags removes tags from bookmarks.
func (s *Service) RemoveTags(ctx context.Context, tweetIDs []string, tagIDs []string) (int, error) {
	if len(tweetIDs) == 0 || len(tagIDs) == 0 {
		return 0, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	pairs := make([]ports.BookmarkTagPair, 0, len(tweetIDs)*len(tagIDs))
	for _, tweetID := range tweetIDs {
		for _, tagID := range tagIDs {
			pairs = append(pairs, ports.BookmarkTagPair{BookmarkID: tweetID, TagID: tagID})
		}
	}

	removed, err := s.repos.Tags.DetachMany(ctx, pairs)
	if err != nil {
		return 0, err
	}
	_ = s.bumpGeneration()
	return removed, nil
}

// MoveToList moves bookmarks into a list.
func (s *Service) MoveToList(ctx context.Context, tweetIDs []string, listID string) error {
	if len(tweetIDs) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, found, err := s.repos.Lists.Get(ctx, listID); err != nil {
		return err
	} else if !found {
		return fmt.Errorf("workspace: no list %q", listID)
	}

	if err := s.repos.Bookmarks.AssignToList(ctx, tweetIDs, listID, s.clock.Now()); err != nil {
		return err
	}
	_ = s.bumpGeneration()
	return nil
}

// DeleteBookmarks removes bookmarks.
//
// Pending suggestions go with them; resolved ones stay, because they are the
// negative cache that stops the same tag being offered again.
func (s *Service) DeleteBookmarks(ctx context.Context, tweetIDs []string) (int, error) {
	if len(tweetIDs) == 0 {
		return 0, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.repos.Suggestions.DropPendingForBookmarks(ctx, tweetIDs); err != nil {
		return 0, err
	}

	removed, err := s.repos.Bookmarks.DeleteMany(ctx, tweetIDs)
	if err != nil {
		return 0, err
	}
	_ = s.bumpGeneration()
	return removed, nil
}

// CreateTag creates a tag.
func (s *Service) CreateTag(ctx context.Context, name string) (domain.Tag, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	display := domain.NormalizeTagDisplayName(name)
	key := domain.CanonicalTagKey(display)
	if key == "" {
		return domain.Tag{}, errors.New("workspace: a tag needs a name")
	}

	// Reusing an existing tag is the right answer here: the user asked for a tag
	// by that name, and one already exists.
	if existing, found, err := s.repos.Tags.FindByName(ctx, display); err != nil {
		return domain.Tag{}, err
	} else if found {
		return existing, nil
	}

	now := s.clock.Now()
	tag := domain.Tag{
		ID:           s.newID("tag"),
		Name:         display,
		CanonicalKey: key,
		CreatedAt:    domain.FormatTimestamp(now),
		UpdatedAt:    domain.FormatTimestamp(now),
	}
	if err := s.repos.Tags.Create(ctx, tag); err != nil {
		return domain.Tag{}, err
	}
	_ = s.bumpGeneration()
	return tag, nil
}

// TagNameTakenError is a rename refused because another tag already answers to the name.
//
// It names the tag that got in the way, not the wanted name twice: renaming "Rustlang" to
// "Rust" would otherwise read as `"Rust" is already used by "Rust"`, which says nothing.
type TagNameTakenError struct {
	// Renaming is the tag the caller tried to rename.
	Renaming string
	// Wanted is the name that was asked for.
	Wanted string
	// TakenBy is the tag that already answers to it.
	TakenBy string
}

func (e *TagNameTakenError) Error() string {
	return fmt.Sprintf("workspace: cannot rename %q to %q: %q already uses that name",
		e.Renaming, e.Wanted, e.TakenBy)
}

// RenameTag renames a tag.
//
// A name another tag already answers to is refused rather than merged into it: merging
// would move a second tag's relations and delete that tag, which is not what a rename
// asks for. Merging has its own entry point.
func (s *Service) RenameTag(ctx context.Context, tagID, newName string) (domain.Tag, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tag, found, err := s.repos.Tags.Get(ctx, tagID)
	if err != nil {
		return domain.Tag{}, err
	}
	if !found {
		return domain.Tag{}, fmt.Errorf("workspace: no tag %q", tagID)
	}

	display := domain.NormalizeTagDisplayName(newName)
	key := domain.CanonicalTagKey(display)
	if key == "" {
		return domain.Tag{}, errors.New("workspace: a tag needs a name")
	}

	// A rename onto a name another tag already answers to is refused, not merged.
	//
	// This used to merge, on the reasoning that the relations then move instead of being
	// orphaned. Refusing avoids that too, and does it without moving anything: the user
	// asked to rename one tag, and merging would also have absorbed a second tag's
	// relations and deleted it. Merging is a separate action with its own entry point
	// (MergeTags), and the extension keeps the two apart for the same reason.
	//
	// The lookup covers aliases as well as names, because FindByName does -- an alias
	// another tag owns is a collision for the same reason a name is.
	if other, exists, err := s.repos.Tags.FindByName(ctx, display); err != nil {
		return domain.Tag{}, err
	} else if exists && other.ID != tagID {
		return domain.Tag{}, &TagNameTakenError{Renaming: tag.Name, Wanted: display, TakenBy: other.Name}
	}

	tag.Name = display
	tag.CanonicalKey = key
	tag.UpdatedAt = domain.FormatTimestamp(s.clock.Now())
	if err := s.repos.Tags.Update(ctx, tag); err != nil {
		return domain.Tag{}, err
	}
	_ = s.bumpGeneration()
	return tag, nil
}

// DeleteTag removes a tag and rejects its pending suggestions.
//
// Rejecting them is not optional: a leftover pending suggestion would recreate
// the tag the user just deleted on the next accept.
func (s *Service) DeleteTag(ctx context.Context, tagID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tag, found, err := s.repos.Tags.Get(ctx, tagID)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}

	now := s.clock.Now()
	if _, err := s.repos.Suggestions.RejectByTagNames(ctx, []string{tag.Name}, now); err != nil {
		return err
	}
	if err := s.repos.Tags.Delete(ctx, tagID); err != nil {
		return err
	}
	_ = s.bumpGeneration()
	return nil
}

// CreateList creates a list.
func (s *Service) CreateList(ctx context.Context, name string) (domain.List, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	trimmed := strings.TrimSpace(name)
	normalized := domain.NormalizeListName(trimmed)
	if normalized == "" {
		return domain.List{}, errors.New("workspace: a list needs a name")
	}
	if normalized == domain.NormalizeListName(domain.InboxListName) {
		return domain.List{}, &domain.ListError{Reason: `"Inbox" is reserved for the system list`}
	}

	if existing, found, err := s.repos.Lists.FindByNormalizedName(ctx, normalized); err != nil {
		return domain.List{}, err
	} else if found {
		return existing, nil
	}

	now := s.clock.Now()
	list := domain.List{
		ID:         s.newID("list"),
		Name:       trimmed,
		Normalized: normalized,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := s.repos.Lists.Create(ctx, list); err != nil {
		return domain.List{}, err
	}
	_ = s.bumpGeneration()
	return list, nil
}

// SavedViews lists the stored views, newest first.
func (s *Service) SavedViews(ctx context.Context) ([]domain.SavedView, error) {
	if s.repos.SavedViews == nil {
		return nil, nil
	}
	views, err := s.repos.SavedViews.List(ctx)
	if err != nil {
		return nil, err
	}
	// The source keeps the newest first, which is the order the sidebar shows.
	sort.SliceStable(views, func(i, j int) bool { return views[i].CreatedAt.After(views[j].CreatedAt) })
	return views, nil
}

// AddSavedView stores a view, keeping the newest first.
//
// The source writes the whole list on every change, so the ordering is part of the
// operation rather than something the reader has to impose.
func (s *Service) AddSavedView(ctx context.Context, view domain.SavedView) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.repos.SavedViews == nil {
		return nil
	}
	return s.repos.SavedViews.Upsert(ctx, view)
}

// RemoveSavedView deletes a view.
func (s *Service) RemoveSavedView(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.repos.SavedViews == nil {
		return nil
	}
	return s.repos.SavedViews.Delete(ctx, id)
}

// NewSavedView builds a view from the current filters.
//
// The id is generated here for the same reason the source generates one: two views made
// in the same second must not collide.
func (s *Service) NewSavedView(name string, snapshot domain.SavedView) domain.SavedView {
	now := s.clock.Now()
	snapshot.ID = s.newID("view")
	snapshot.Name = name
	snapshot.CreatedAt = now
	snapshot.UpdatedAt = now
	return snapshot
}

// RenameList renames a list.
func (s *Service) RenameList(ctx context.Context, listID, newName string) (domain.List, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	list, found, err := s.repos.Lists.Get(ctx, listID)
	if err != nil {
		return domain.List{}, err
	}
	if !found {
		return domain.List{}, fmt.Errorf("workspace: no list %q", listID)
	}
	if err := domain.ValidateListRename(list, newName); err != nil {
		return domain.List{}, err
	}

	trimmed := strings.TrimSpace(newName)
	normalized := domain.NormalizeListName(trimmed)
	if existing, exists, err := s.repos.Lists.FindByNormalizedName(ctx, normalized); err != nil {
		return domain.List{}, err
	} else if exists && existing.ID != listID {
		return domain.List{}, fmt.Errorf("workspace: a list named %q already exists", trimmed)
	}

	list.Name = trimmed
	list.Normalized = normalized
	list.UpdatedAt = s.clock.Now()
	if err := s.repos.Lists.Update(ctx, list); err != nil {
		return domain.List{}, err
	}
	_ = s.bumpGeneration()
	return list, nil
}

// DeleteList removes a list, first moving its bookmarks to a destination.
//
// The destination is required rather than defaulted inside the store: decision D3
// is that the user chooses, and the store refuses an empty list precisely so this
// decision cannot be skipped.
func (s *Service) DeleteList(ctx context.Context, listID, moveTo string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	list, found, err := s.repos.Lists.Get(ctx, listID)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	if err := domain.ValidateListDelete(list); err != nil {
		return err
	}

	if strings.TrimSpace(moveTo) == "" {
		return errors.New("workspace: a destination list is required")
	}
	if moveTo == listID {
		return errors.New("workspace: the destination must be a different list")
	}

	// Move the bookmarks out first: the store refuses to delete a list that still
	// holds any, which is what makes "silently lost their home" impossible.
	now := s.clock.Now()
	ids, err := s.bookmarksInList(ctx, listID)
	if err != nil {
		return err
	}
	if len(ids) > 0 {
		if err := s.repos.Bookmarks.AssignToList(ctx, ids, moveTo, now); err != nil {
			return err
		}
	}

	if err := s.repos.Lists.Delete(ctx, listID); err != nil {
		return err
	}
	_ = s.bumpGeneration()
	return nil
}

// bookmarksInList lists the bookmarks in a list.
func (s *Service) bookmarksInList(ctx context.Context, listID string) ([]string, error) {
	page, err := s.repos.Bookmarks.Query(ctx, domain.LibraryQuery{
		Filter: domain.LibraryFilter{ListID: listID},
		Sort:   domain.SortSavedDesc,
		Limit:  domain.MaxLibraryPageSize,
	})
	if err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(page))
	for _, bookmark := range page {
		ids = append(ids, bookmark.TweetID)
	}
	return ids, nil
}

// bumpGeneration marks every previously built view stale.
func (s *Service) bumpGeneration() error {
	s.generation.Add(1)
	return nil
}

// newID mints an identifier, falling back to a timestamp when no generator is
// configured so a caller cannot accidentally write empty ids.
func (s *Service) newID(prefix string) string {
	if s.ids == nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}
	return s.ids.NewID()
}

// LoadGraphInput reads everything the tag relation graph needs.
//
// It is a separate read rather than a field on View because the graph needs the whole
// library, not a page of it, and a caller that only wants the list should not pay for
// that. It lives here rather than in the UI so the UI keeps depending on usecases
// rather than on repositories.
func (s *Service) LoadGraphInput(ctx context.Context) (domain.TagRelationInput, error) {
	input := domain.TagRelationInput{Tags: []domain.Tag{}, Bookmarks: []domain.Bookmark{}, BookmarkTags: []domain.BookmarkTag{}}

	if s.repos.Tags != nil {
		tags, err := s.repos.Tags.List(ctx)
		if err != nil {
			return input, fmt.Errorf("workspace: read tags for the graph: %w", err)
		}
		input.Tags = tags

		relations, err := s.repos.Tags.AllRelations(ctx)
		if err != nil {
			return input, fmt.Errorf("workspace: read tag relations for the graph: %w", err)
		}
		input.BookmarkTags = relations
	}

	if s.repos.Bookmarks != nil {
		bookmarks, err := s.repos.Bookmarks.All(ctx)
		if err != nil {
			return input, fmt.Errorf("workspace: read bookmarks for the graph: %w", err)
		}
		input.Bookmarks = bookmarks
	}

	return input, nil
}

// MergeReport is what a merge changed.
type MergeReport struct {
	// Target is the surviving tag, with its absorbed aliases.
	Target domain.Tag
	// MergedTagIDs are the tags that no longer exist.
	MergedTagIDs []string
	// RemappedCount counts the relations that moved. A relation that already existed
	// on the target is not counted: nothing moved, it was already there.
	RemappedCount int
	// ReparentedCount counts the children that followed the survivor.
	ReparentedCount int
}

// MergeTags folds several tags into one.
//
// This is the other half of the vocabulary review: the model proposes merges and the
// user applies them, and without this the proposals could never take effect.
//
// The order matters for safety. Relations are added to the target before anything is
// removed, so an interruption leaves the library with too many tags rather than with
// bookmarks that lost a tag. The sources are only deleted once their relations have
// been moved.
func (s *Service) MergeTags(ctx context.Context, sourceTagIDs []string, targetTagID string) (MergeReport, error) {
	report := MergeReport{MergedTagIDs: []string{}}

	if targetTagID == "" {
		return report, errors.New("workspace: merge needs a target tag")
	}

	target, found, err := s.repos.Tags.Get(ctx, targetTagID)
	if err != nil {
		return report, err
	}
	if !found {
		return report, errors.New("workspace: the target tag does not exist")
	}

	// The target is never its own source.
	sources := make([]string, 0, len(sourceTagIDs))
	seen := make(map[string]bool, len(sourceTagIDs))
	for _, id := range sourceTagIDs {
		if id == "" || id == targetTagID || seen[id] {
			continue
		}
		seen[id] = true
		sources = append(sources, id)
	}
	if len(sources) == 0 {
		return report, nil
	}

	allTags, err := s.repos.Tags.List(ctx)
	if err != nil {
		return report, err
	}
	tagByID := make(map[string]domain.Tag, len(allTags))
	for _, tag := range allTags {
		tagByID[tag.ID] = tag
	}

	relations, err := s.repos.Tags.AllRelations(ctx)
	if err != nil {
		return report, err
	}

	// A bookmark that already carries the target must not gain a second identical
	// relation, so the existing pair is recorded first.
	alreadyOnTarget := make(map[string]bool, len(relations))
	for _, relation := range relations {
		if relation.TagID == targetTagID {
			alreadyOnTarget[relation.BookmarkID] = true
		}
	}

	aliases := append([]string{}, target.Aliases...)
	aliasKeys := make(map[string]bool, len(aliases)+1)
	aliasKeys[domain.CanonicalTagKey(target.Name)] = true
	for _, alias := range aliases {
		aliasKeys[domain.CanonicalTagKey(alias)] = true
	}

	merged := make(map[string]bool, len(sources))
	now := s.clock.Now()

	for _, sourceID := range sources {
		source, known := tagByID[sourceID]
		if !known {
			continue
		}

		for _, relation := range relations {
			if relation.TagID != sourceID {
				continue
			}

			if !alreadyOnTarget[relation.BookmarkID] {
				if err := s.repos.Tags.Attach(ctx, domain.BookmarkTag{
					BookmarkID: relation.BookmarkID,
					TagID:      targetTagID,
					CreatedAt:  relation.CreatedAt,
					Source:     relation.Source,
				}); err != nil {
					return report, fmt.Errorf("workspace: move a relation during merge: %w", err)
				}
				alreadyOnTarget[relation.BookmarkID] = true
				report.RemappedCount++
			}

			if err := s.repos.Tags.Detach(ctx, relation.BookmarkID, sourceID); err != nil {
				return report, fmt.Errorf("workspace: drop a merged relation: %w", err)
			}
		}

		// The absorbed names become aliases, so the tag can still be found by the name
		// it used to have and a later suggestion of that name resolves here.
		for _, name := range append([]string{source.Name}, source.Aliases...) {
			key := domain.CanonicalTagKey(name)
			if key == "" || aliasKeys[key] {
				continue
			}
			aliasKeys[key] = true
			aliases = append(aliases, name)
		}

		if err := s.repos.Tags.Delete(ctx, sourceID); err != nil {
			return report, fmt.Errorf("workspace: delete a merged tag: %w", err)
		}
		merged[sourceID] = true
		report.MergedTagIDs = append(report.MergedTagIDs, sourceID)
	}

	// The survivor keeps the absorbed names.
	target.Aliases = aliases
	// Its own parent may be one of the tags just deleted, in which case it becomes a
	// root. Writing the stale parentId would fail the foreign key, and writing it
	// successfully would leave a tag pointing at nothing.
	survivorWasChild := target.ParentID != ""
	if survivorWasChild && merged[target.ParentID] {
		target.ParentID = ""
	}
	target.UpdatedAt = domain.FormatTimestamp(now)
	if err := s.repos.Tags.Update(ctx, target); err != nil {
		return report, fmt.Errorf("workspace: save the surviving tag: %w", err)
	}

	// Children of a merged tag follow the survivor, unless the survivor is itself a
	// child -- the taxonomy stays two levels deep, so in that case they become roots.
	if len(report.MergedTagIDs) > 0 {
		// The survivor's parent may itself be one of the merged tags, in which case the
		// survivor is about to become a root. Deciding from the stale parentId would
		// send its children to the root level even though the survivor is a root they
		// could have followed.
		survivorEndsUpRoot := target.ParentID == ""

		for _, child := range allTags {
			if child.ParentID == "" || !merged[child.ParentID] {
				continue
			}
			// A merged source was just deleted; updating it would resurrect it.
			if child.ID == targetTagID || merged[child.ID] {
				continue
			}

			child.ParentID = ""
			if survivorEndsUpRoot {
				child.ParentID = targetTagID
			}
			child.UpdatedAt = domain.FormatTimestamp(now)
			if err := s.repos.Tags.Update(ctx, child); err != nil {
				return report, fmt.Errorf("workspace: reparent a merged tag's child: %w", err)
			}
			report.ReparentedCount++
		}
	}

	if err := s.bumpGeneration(); err != nil {
		return report, err
	}

	updated, _, err := s.repos.Tags.Get(ctx, targetTagID)
	if err != nil {
		return report, err
	}
	report.Target = updated

	return report, nil
}

// SetTagParent nests a tag under a root, or clears its parent when parentID is empty.
//
// The two-level limit is enforced here as well as by the schema's trigger, so the
// caller gets a sentence rather than a constraint violation.
func (s *Service) SetTagParent(ctx context.Context, tagID, parentID string) (domain.Tag, error) {
	var empty domain.Tag

	tag, found, err := s.repos.Tags.Get(ctx, tagID)
	if err != nil {
		return empty, err
	}
	if !found {
		return empty, errors.New("workspace: the tag does not exist")
	}

	if parentID == "" {
		tag.ParentID = ""
		tag.UpdatedAt = domain.FormatTimestamp(s.clock.Now())
		if err := s.repos.Tags.Update(ctx, tag); err != nil {
			return empty, err
		}
		return tag, nil
	}

	// The domain owns the rule, including the case the schema's triggers cannot see:
	// giving a parent to a tag that already has children would push those children to
	// a third level.
	allTags, err := s.repos.Tags.List(ctx)
	if err != nil {
		return empty, err
	}
	if err := domain.ValidateTagHierarchy(allTags, tagID, parentID); err != nil {
		return empty, err
	}

	tag.ParentID = parentID
	tag.UpdatedAt = domain.FormatTimestamp(s.clock.Now())
	if err := s.repos.Tags.Update(ctx, tag); err != nil {
		return empty, err
	}
	return tag, nil
}

// UpdateTagAliases replaces a tag's aliases.
//
// Blank aliases and an alias that duplicates the tag's own name are dropped: either
// would make the tag findable under a name it already answers to, which is noise
// rather than information.
//
// An alias that another tag already claims is dropped too, and "claims" means its name
// as well as its own aliases. FindByName matches a canonical key or any alias, so an
// alias equal to another tag's name makes one lookup answer with two tags. Blocking only
// names is what let two tags advertise the same alias in the extension, which is why it
// collects both there.
//
// Nothing is reported when an alias is dropped: a paste-heavy edit should still save the
// part that is usable.
func (s *Service) UpdateTagAliases(ctx context.Context, tagID string, aliases []string) (domain.Tag, error) {
	var empty domain.Tag

	tag, found, err := s.repos.Tags.Get(ctx, tagID)
	if err != nil {
		return empty, err
	}
	if !found {
		return empty, errors.New("workspace: the tag does not exist")
	}

	// Every canonical identity another tag already answers to.
	seen := map[string]bool{domain.CanonicalTagKey(tag.Name): true}
	all, err := s.repos.Tags.List(ctx)
	if err != nil {
		return empty, err
	}
	for _, other := range all {
		if other.ID == tag.ID {
			continue
		}
		seen[domain.CanonicalTagKey(other.Name)] = true
		for _, alias := range other.Aliases {
			seen[domain.CanonicalTagKey(alias)] = true
		}
	}

	cleaned := make([]string, 0, len(aliases))
	for _, alias := range aliases {
		trimmed := strings.TrimSpace(alias)
		key := domain.CanonicalTagKey(trimmed)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		cleaned = append(cleaned, trimmed)
	}

	tag.Aliases = cleaned
	tag.UpdatedAt = domain.FormatTimestamp(s.clock.Now())
	if err := s.repos.Tags.Update(ctx, tag); err != nil {
		return empty, err
	}
	return tag, nil
}

// LoadTagLibrary reads the tag management view.
//
// It is a separate read from Load because it needs every tag and every suggestion,
// not a page of bookmarks, and the workspace list does not carry suggestions at all.
func (s *Service) LoadTagLibrary(ctx context.Context) ([]domain.TagLibraryItem, error) {
	tags, err := s.repos.Tags.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("workspace: read tags for the library: %w", err)
	}
	relations, err := s.repos.Tags.AllRelations(ctx)
	if err != nil {
		return nil, fmt.Errorf("workspace: read tag relations for the library: %w", err)
	}

	var suggestions []domain.TagSuggestion
	if s.repos.Suggestions != nil {
		suggestions, err = s.repos.Suggestions.All(ctx)
		if err != nil {
			return nil, fmt.Errorf("workspace: read suggestions for the library: %w", err)
		}
	}

	return domain.BuildTagLibraryItems(tags, relations, suggestions), nil
}

// CleanupOrphanTags deletes the tags safe to sweep.
//
// It goes through the domain rule rather than filtering here, so the
// pending-suggestion guard cannot be forgotten at the call site: without it a sweep
// deletes a tag the user is still reviewing, rejects those suggestions, and blocks the
// name from ever being suggested again.
func (s *Service) CleanupOrphanTags(ctx context.Context) (int, error) {
	items, err := s.LoadTagLibrary(ctx)
	if err != nil {
		return 0, err
	}

	ids := domain.GetRemovableOrphanTagIDs(items)
	if len(ids) == 0 {
		return 0, nil
	}

	removed, err := s.repos.Tags.DeleteMany(ctx, ids)
	if err != nil {
		return 0, fmt.Errorf("workspace: clean up empty tags: %w", err)
	}
	if err := s.bumpGeneration(); err != nil {
		return removed, err
	}
	return removed, nil
}

// AddBookmark stores a post the user entered by hand.
//
// The extension has no such call: its content script sends the whole tweet it read off the
// page. A standalone manager has no page, so the link is the input and the record is built
// from what can be read out of it.
//
// The record is merged into whatever is already stored rather than written over it --
// see domain.MergeDraft for why the rule is the opposite of the sync path's.
func (s *Service) AddBookmark(ctx context.Context, rawURL, text string) (domain.Bookmark, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tweetID, ok := domain.TweetIDFromURL(rawURL)
	if !ok {
		return domain.Bookmark{}, ErrNotAPostLink
	}
	handle := handleFromTweetURL(rawURL)
	now := s.clock.Now()

	draft := domain.Bookmark{
		TweetID:      tweetID,
		TweetURL:     postURL(tweetID, handle),
		AuthorHandle: handle,
		Text:         strings.TrimSpace(text),
		// sourceKind is left alone: the extension's upsertBookmarkSnapshot does not set
		// it for a site-tweet save either, so a hand-entered post is not a third shape.
		CreatedAtOnX: "",
		LastSeenAt:   now,
		SavedAt:      now,
		UpdatedAt:    now,
	}

	stored, found, err := s.repos.Bookmarks.Get(ctx, tweetID)
	if err != nil {
		return domain.Bookmark{}, err
	}
	merged := draft
	if found {
		merged = domain.MergeDraft(stored, draft, now)
	} else {
		merged = domain.MergeDraft(domain.Bookmark{}, draft, now)
	}

	if _, err := s.repos.Bookmarks.UpsertMany(ctx, []domain.Bookmark{merged}); err != nil {
		return domain.Bookmark{}, err
	}
	if err := s.bumpGeneration(); err != nil {
		return domain.Bookmark{}, err
	}
	return merged, nil
}

// ErrNotAPostLink is returned for input that is not a link to an X post.
var ErrNotAPostLink = errors.New("workspace: that is not a link to a post")

// postURL is the canonical link for a post, with the handle when one is known.
//
// The extension stores whatever tweetUrl the page carried. Rebuilding it here keeps one
// shape in the library, so two adds of the same post cannot differ by tracking parameters.
func postURL(tweetID, handle string) string {
	if handle == "" {
		return "https://x.com/i/web/status/" + tweetID
	}
	return "https://x.com/" + handle + "/status/" + tweetID
}

// handleFromTweetURL reads the handle out of a post link, if it has one.
func handleFromTweetURL(rawURL string) string {
	trimmed := strings.TrimSpace(rawURL)
	if !strings.Contains(trimmed, "://") {
		trimmed = "https://" + trimmed
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return ""
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	for i := 1; i < len(segments); i++ {
		if segments[i-1] != "status" && segments[i-1] != "statuses" {
			continue
		}
		// The handle is the segment before "status"; /i/web/status has none.
		if i-2 >= 0 {
			candidate := segments[i-2]
			if candidate == "i" || candidate == "web" {
				return ""
			}
			return candidate
		}
	}
	return ""
}
