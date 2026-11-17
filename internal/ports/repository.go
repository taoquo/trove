// Package ports declares what the usecase layer needs from the outside world.
//
// The interfaces live here rather than next to their implementations so that
// domain and usecase never import an adapter. That is what keeps the storage
// engine, the sync client and the model provider replaceable.
package ports

import (
	"context"
	"time"

	"github.com/taoquo/trove/internal/domain"
)

// RawPayload is one untouched response body.
//
// It is stored as the JSON text the API returned, so a parser change can be
// replayed against real data without re-fetching it.
type RawPayload struct {
	TweetID     string
	PayloadJSON string
	CapturedAt  string
}

// Tombstone is one recorded local delete.
//
// It exists so a sync cannot resurrect a post the user removed: the upsert skips a
// tombstoned id, and a backup carries the record so a restore does not lose it.
type Tombstone struct {
	TweetID   string
	DeletedAt string
}

// UpsertResult reports what a bulk write changed.
//
// Inserted and Updated are counted separately because the sync report shows them
// separately, and a run that reports "1234 updated" when it inserted 1234 rows
// tells the user nothing about whether sync is working.
type UpsertResult struct {
	Inserted int
	Updated  int
}

// BookmarkRepository stores bookmarks.
type BookmarkRepository interface {
	// UpsertMany inserts or updates bookmarks, preserving saved_at on update:
	// it is the user's own timeline and a later sync must never move it.
	UpsertMany(ctx context.Context, bookmarks []domain.Bookmark) (UpsertResult, error)

	// Get returns a bookmark and whether it exists.
	Get(ctx context.Context, tweetID string) (domain.Bookmark, bool, error)

	// GetMany returns the bookmarks with any of the given ids, keyed by id. An id
	// that no longer exists is simply absent. It is the read the suggestion panel
	// needs: one statement for the page rather than one per row, and the count of
	// statements no longer grows with the number of suggestions.
	GetMany(ctx context.Context, tweetIDs []string) (map[string]domain.Bookmark, error)

	// Query returns the bookmarks matching a filter, in the requested order.
	Query(ctx context.Context, query domain.LibraryQuery) ([]domain.Bookmark, error)

	// Count returns how many bookmarks match, ignoring the window.
	Count(ctx context.Context, filter domain.LibraryFilter) (int, error)

	// AllIDs returns every stored tweet id, for detecting deletions.
	AllIDs(ctx context.Context) ([]string, error)
	// KnownIDs returns every id this device has already seen, including the
	// tombstones of locally deleted posts. The sync's early stop must treat a
	// tombstoned id as known: X still returns it, but upsert skips it, so
	// counting it as new would walk the whole timeline every run.
	KnownIDs(ctx context.Context) ([]string, error)

	// All returns every bookmark.
	//
	// The auto-tagger works over the whole library rather than a window, so it
	// needs the whole set: filtering it through a paged query would silently cap
	// a large library at the page size and skip the rest.
	All(ctx context.Context) ([]domain.Bookmark, error)

	Delete(ctx context.Context, tweetID string) error

	// DeleteMany removes bookmarks and returns how many existed.
	DeleteMany(ctx context.Context, tweetIDs []string) (int, error)

	// DeleteAll empties the library and returns how many rows went. It also
	// forgets the local deletes, because emptying the library is a fresh start.
	DeleteAll(ctx context.Context) (int, error)

	// ClearTombstones forgets the local-delete record for the given ids, so they
	// can be written again. An import calls it with the ids its file carries: the
	// user asked for that file's contents, and for those bookmarks it is
	// authoritative. A restore does not need it; emptying the library clears
	// every tombstone first.
	ClearTombstones(ctx context.Context, tweetIDs []string) error

	// Tombstones returns the record of posts deleted locally.
	Tombstones(ctx context.Context) ([]Tombstone, error)

	// AddTombstones records local deletes without deleting anything. A restore
	// calls it with the bundle's record: the file is the new library, so the posts
	// the exporting device had deleted must stay deleted here too.
	AddTombstones(ctx context.Context, tombstones []Tombstone) error

	// RawPayloads returns the stored response bodies for the given bookmarks.
	RawPayloads(ctx context.Context, tweetIDs []string) ([]RawPayload, error)

	// SaveRawPayloads stores untouched response bodies, replacing an earlier
	// capture of the same tweet. It is the write half of the diagnostic
	// setting; without it "keep raw responses" was a checkbox with no effect.
	SaveRawPayloads(ctx context.Context, payloads []RawPayload) error

	// TagsByBookmark returns the tags for each given bookmark. Bookmarks with no
	// tags are absent from the map rather than present with an empty slice.
	TagsByBookmark(ctx context.Context, tweetIDs []string) (map[string][]domain.Tag, error)

	// ListByBookmark returns the list each given bookmark belongs to.
	ListByBookmark(ctx context.Context, tweetIDs []string) (map[string]domain.List, error)

	// AssignToList moves bookmarks into a list, replacing any current list.
	AssignToList(ctx context.Context, tweetIDs []string, listID string, now time.Time) error

	// AssignMissingToInbox gives every bookmark without a list the Inbox, which
	// is what keeps "every bookmark is in exactly one list" true. It returns how
	// many rows it added.
	AssignMissingToInbox(ctx context.Context, now time.Time) (int, error)

	// SetArchived moves one bookmark into or out of the archive. A zero time
	// takes it back out.
	SetArchived(ctx context.Context, tweetID string, at time.Time) error

	// SetArchivedMany moves a batch into or out of the archive in one write. A zero
	// time takes them back out. Ids the library does not hold are ignored: a batch is
	// a selection, and a row that left between the click and the write is not an error.
	SetArchivedMany(ctx context.Context, tweetIDs []string, at time.Time) error

	// Counts returns the sidebar aggregates.
	Counts(ctx context.Context) (domain.WorkspaceCounts, error)

	// FacetCounts counts, for each tag and each author, how many bookmarks the given
	// filter selects with that facet's own values removed.
	//
	// One pair of aggregates rather than a query per row: the sidebar redraws on every
	// filter change, and a count per tag would put a query behind each of them.
	FacetCounts(ctx context.Context, filter domain.LibraryFilter) (domain.FacetCounts, error)

	// AuthorSummaries returns the most-bookmarked authors.
	AuthorSummaries(ctx context.Context, limit int) ([]domain.AuthorSummary, error)
}

// TagRepository stores tags and the tags on a bookmark.
type TagRepository interface {
	List(ctx context.Context) ([]domain.Tag, error)
	Get(ctx context.Context, id string) (domain.Tag, bool, error)
	// FindByName resolves a tag by its canonical key or by one of its aliases,
	// which is what stops an accepted suggestion from minting a near-duplicate.
	FindByName(ctx context.Context, name string) (domain.Tag, bool, error)
	Create(ctx context.Context, tag domain.Tag) error
	Update(ctx context.Context, tag domain.Tag) error
	Delete(ctx context.Context, id string) error
	// DeleteMany removes tags and returns how many existed.
	DeleteMany(ctx context.Context, ids []string) (int, error)
	// DeleteAll removes every tag and returns how many went. It is for the reset
	// action, whose confirmation promises that local tags are cleared.
	DeleteAll(ctx context.Context) (int, error)

	// Summaries returns each tag with its bookmark and pending counts.
	Summaries(ctx context.Context) ([]domain.TagSummary, error)

	// Attach adds a relation, leaving an existing one alone: a manual tag must
	// not be downgraded to an automatic source by a later automatic pass.
	Attach(ctx context.Context, relation domain.BookmarkTag) error
	// AttachMany adds several relations in one transaction.
	AttachMany(ctx context.Context, relations []domain.BookmarkTag) error
	// Detach removes a relation.
	Detach(ctx context.Context, bookmarkID, tagID string) error
	// DetachMany removes several relations.
	DetachMany(ctx context.Context, pairs []BookmarkTagPair) (int, error)
	// DetachAutomatic removes only automatic relations, so a prune can never
	// delete a tag a human applied.
	DetachAutomatic(ctx context.Context, bookmarkIDs []string) (int, error)

	// AllRelations returns every bookmark-to-tag relation.
	//
	// The author heuristic needs the whole relation set to measure how strongly an
	// author leans toward a tag, and measuring it from a page would make the
	// threshold depend on which page was loaded.
	AllRelations(ctx context.Context) ([]domain.BookmarkTag, error)
}

// BookmarkTagPair identifies a relation.
type BookmarkTagPair struct {
	BookmarkID string
	TagID      string
}

// ListRepository stores lists.
type ListRepository interface {
	List(ctx context.Context) ([]domain.List, error)
	// Summaries returns each list with its bookmark count.
	Summaries(ctx context.Context) ([]domain.ListSummary, error)
	Get(ctx context.Context, id string) (domain.List, bool, error)
	FindByNormalizedName(ctx context.Context, normalized string) (domain.List, bool, error)
	Create(ctx context.Context, list domain.List) error
	Update(ctx context.Context, list domain.List) error
	// Delete removes a list. It must fail if any bookmark still points at it, so
	// the caller has to decide where those bookmarks go first.
	Delete(ctx context.Context, id string) error
	// EnsureInbox creates the Inbox list if it is missing.
	EnsureInbox(ctx context.Context, now time.Time) error
}

// SuggestionRepository stores tag suggestions.
type SuggestionRepository interface {
	List(ctx context.Context, status domain.SuggestionStatus) ([]domain.TagSuggestion, error)
	// All returns every suggestion, whatever its status.
	//
	// An export needs the resolved rows too: they are the negative cache, and a
	// backup that dropped them would let a restored library re-offer tags the user
	// already answered.
	All(ctx context.Context) ([]domain.TagSuggestion, error)
	// DeleteAll removes every suggestion and returns how many went.
	DeleteAll(ctx context.Context) (int, error)
	// Get returns one suggestion by id, whatever its status.
	//
	// It has to span every status: a caller that only looked among pending rows
	// could not tell "no such suggestion" from "the user already answered this",
	// and would report the wrong thing for a double tap.
	Get(ctx context.Context, id string) (domain.TagSuggestion, bool, error)
	ListForBookmark(ctx context.Context, tweetID string) ([]domain.TagSuggestion, error)
	// AllIDs returns every suggestion id, which is the auto-tagger's negative
	// cache: a pair already offered must never be offered again.
	AllIDs(ctx context.Context) (map[string]bool, error)
	InsertMany(ctx context.Context, suggestions []domain.TagSuggestion) (int, error)
	// Resolve stamps an answer on a suggestion that is still pending.
	Resolve(ctx context.Context, id string, status domain.SuggestionStatus, now time.Time) error
	// RejectByTagNames rejects every pending suggestion for the given names,
	// which deleting a tag must do or accepting one would recreate it.
	RejectByTagNames(ctx context.Context, names []string, now time.Time) (int, error)
	// DropPendingForBookmarks removes only pending rows, keeping the resolved
	// ones that form the negative cache.
	DropPendingForBookmarks(ctx context.Context, tweetIDs []string) (int, error)
	PendingCount(ctx context.Context) (int, error)
}

// AnalysisRepository stores the auto-tagger's idempotency ledger.
type AnalysisRepository interface {
	List(ctx context.Context) ([]domain.LlmAnalysis, error)
	// UpsertMany records one row per analysed bookmark, including the ones the
	// model skipped: a missing row is what makes a bookmark be re-sent forever.
	UpsertMany(ctx context.Context, records []domain.LlmAnalysis) (int, error)
	// ExistingFor returns the ledger entries for the given model and prompt
	// version, so a run can skip what it has already answered.
	ExistingFor(ctx context.Context, model string, promptVersion int, contentHashes map[string]string) (map[string]string, error)
	DeleteAll(ctx context.Context) (int, error)
}

// SettingsRepository stores the key-value configuration.
type SettingsRepository interface {
	Get(ctx context.Context, key string) (string, bool, error)
	Set(ctx context.Context, key, valueJSON string, now time.Time) error
	All(ctx context.Context) (map[string]string, error)
	Delete(ctx context.Context, keys ...string) (int, error)
	// DeleteAll removes every stored setting. It is for the reset action, whose
	// confirmation promises that local settings are cleared.
	DeleteAll(ctx context.Context) (int, error)
}

// SavedViewRepository stores named filter sets.
type SavedViewRepository interface {
	List(ctx context.Context) ([]domain.SavedView, error)
	Get(ctx context.Context, id string) (domain.SavedView, bool, error)
	Upsert(ctx context.Context, view domain.SavedView) error
	Delete(ctx context.Context, id string) error
}

// SyncRunRepository stores the sync history.
type SyncRunRepository interface {
	Start(ctx context.Context, run domain.SyncRun) error
	// ClaimStart records a run as running only if no run still owns the library.
	//
	// "Owns" means a running row whose heartbeat is newer than staleBefore. Older
	// ones are closed out as interrupted first, so a crash does not lock the
	// library forever. It returns false when another live run owns it, which is how
	// the window and a scheduled `trove sync` refuse to write the same database at
	// the same time.
	ClaimStart(ctx context.Context, run domain.SyncRun, now, staleBefore time.Time) (bool, error)
	// TouchHeartbeat moves a running run's heartbeat.
	//
	// The heartbeat is what tells another process the run is alive. A run that
	// dies stops moving it, and only then may the next one take the library over.
	TouchHeartbeat(ctx context.Context, runID string, now time.Time) error
	Finish(ctx context.Context, run domain.SyncRun) error
	Get(ctx context.Context, id string) (domain.SyncRun, bool, error)
	Latest(ctx context.Context) (domain.SyncRun, bool, error)
	// Recent returns the newest runs, newest first.
	//
	// A backup carries a bounded slice of history rather than everything: a
	// long-lived install accumulates thousands of runs.
	Recent(ctx context.Context, limit int) ([]domain.SyncRun, error)
	// MarkInterrupted closes out running rows whose heartbeat stopped before
	// cutoff.
	//
	// Without it a killed run leaves the UI claiming a sync is in flight forever.
	// The cutoff is what keeps it from closing a run another process is still
	// making progress on.
	MarkInterrupted(ctx context.Context, cutoff, now time.Time) (int, error)
	Active(ctx context.Context) (domain.SyncRun, bool, error)
}

// RevisionSource reports a number that changes whenever the library is written.
//
// It exists so a reader can reuse an aggregate without trusting a call site to remember
// that it wrote: the number comes from the database itself, and any write moves it. The
// SQLite adapter answers with total_changes(), which counts every row this connection has
// changed since it opened -- and the app keeps exactly one connection, so it is the whole
// truth about writes the app made.
type RevisionSource interface {
	Revision(ctx context.Context) (int64, error)
}

// Repositories bundles the stores a usecase needs.
//
// It is a struct rather than eight constructor parameters because most usecases
// need more than one store, and threading them individually made every new store
// a change to every call site.
type Repositories struct {
	Bookmarks   BookmarkRepository
	Tags        TagRepository
	Lists       ListRepository
	Suggestions SuggestionRepository
	Analysis    AnalysisRepository
	Settings    SettingsRepository
	SavedViews  SavedViewRepository
	SyncRuns    SyncRunRepository
	// Revision is optional: when it is nil, nothing is cached.
	Revision RevisionSource
}

// Clock is the time source, injected so time-dependent behaviour is testable.
type Clock interface {
	Now() time.Time
}

// SystemClock is the real clock.
type SystemClock struct{}

// Now returns the current time.
func (SystemClock) Now() time.Time { return time.Now() }

// IDGenerator mints identifiers.
//
// It is an interface so tests can supply a deterministic generator.
type IDGenerator interface {
	NewID() string
}
