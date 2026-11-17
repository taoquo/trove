package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/ports"
)

// BookmarkRepository stores bookmarks in SQLite.
type BookmarkRepository struct {
	db *DB
	// now is injected so the relative time-range filter is testable; a filter
	// whose cutoff comes from the wall clock cannot be asserted against.
	now func() time.Time
}

// NewBookmarkRepository builds a repository over an open database.
func NewBookmarkRepository(db *DB) *BookmarkRepository {
	return &BookmarkRepository{db: db, now: time.Now}
}

// SetClock replaces the time source, for tests.
func (r *BookmarkRepository) SetClock(now func() time.Time) { r.now = now }

// Compile-time proof that the adapter satisfies the port.
var _ ports.BookmarkRepository = (*BookmarkRepository)(nil)

// upsertSQL inserts a bookmark or updates it in place.
//
// saved_at is deliberately absent from the update list: it is when this machine
// first saw the post, and a later sync must never move it. Every other field is
// refreshed from the newest payload.
//
// archived_at is absent for the same reason, one step further: it is the reader's own
// state, not the payload's. It is written on insert -- so a restore into an empty library
// brings it back -- and never on conflict, so a sync or an import into a library that
// already holds the post cannot pull it out of the archive.
const upsertSQL = `
INSERT INTO bookmarks (
  tweet_id, tweet_url, author_name, author_handle, author_avatar_url, text,
  created_at_on_x, created_at_on_x_ts, saved_at, last_seen_at, timeline_rank,
  archived_at,
  hashtags_json, source_kind, media_json, media_kind, metrics_json, updated_at,
  is_longform
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(tweet_id) DO UPDATE SET
  tweet_url          = excluded.tweet_url,
  author_name        = excluded.author_name,
  author_handle      = excluded.author_handle,
  author_avatar_url  = excluded.author_avatar_url,
  text               = excluded.text,
  created_at_on_x    = excluded.created_at_on_x,
  created_at_on_x_ts = excluded.created_at_on_x_ts,
  last_seen_at       = excluded.last_seen_at,
  timeline_rank      = excluded.timeline_rank,
  hashtags_json      = excluded.hashtags_json,
  source_kind        = excluded.source_kind,
  media_json         = excluded.media_json,
  media_kind         = excluded.media_kind,
  metrics_json       = excluded.metrics_json,
  updated_at         = excluded.updated_at,
  is_longform        = excluded.is_longform
`

// UpsertMany writes bookmarks, preserving saved_at on rows that already exist.
func (r *BookmarkRepository) UpsertMany(ctx context.Context, bookmarks []domain.Bookmark) (ports.UpsertResult, error) {
	result := ports.UpsertResult{}
	if len(bookmarks) == 0 {
		return result, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return result, fmt.Errorf("sqlite: begin bookmark upsert: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op once committed

	ids := make([]string, 0, len(bookmarks))
	for _, bookmark := range bookmarks {
		ids = append(ids, bookmark.TweetID)
	}

	existing, err := existingIDs(ctx, tx, ids)
	if err != nil {
		return result, err
	}

	// A tombstoned id is a bookmark the user deleted here. Re-importing it is the
	// F7-1 defect, so the row is neither inserted nor updated, and the sync's own
	// counts do not claim it was.
	deleted, err := tombstonedIDs(ctx, tx, ids)
	if err != nil {
		return result, err
	}

	statement, err := tx.PrepareContext(ctx, upsertSQL)
	if err != nil {
		return result, fmt.Errorf("sqlite: prepare bookmark upsert: %w", err)
	}
	defer statement.Close()

	for _, bookmark := range bookmarks {
		if deleted[bookmark.TweetID] {
			continue
		}
		args, err := bookmarkArgs(bookmark)
		if err != nil {
			return result, err
		}
		if _, err := statement.ExecContext(ctx, args...); err != nil {
			return result, fmt.Errorf("sqlite: upsert bookmark %s: %w", bookmark.TweetID, err)
		}
		if existing[bookmark.TweetID] {
			result.Updated++
		} else {
			result.Inserted++
		}
	}

	if err := tx.Commit(); err != nil {
		return result, fmt.Errorf("sqlite: commit bookmark upsert: %w", err)
	}
	return result, nil
}

// bookmarkArgs renders one bookmark as bound parameters.
func bookmarkArgs(bookmark domain.Bookmark) ([]any, error) {
	hashtagsJSON, err := encodeStrings(bookmark.Hashtags)
	if err != nil {
		return nil, err
	}
	mediaJSON, err := encodeMedia(bookmark.Media)
	if err != nil {
		return nil, err
	}
	metricsJSON, err := encodeMetrics(bookmark.Metrics)
	if err != nil {
		return nil, err
	}

	lastSeenAt := sql.NullString{}
	if !bookmark.LastSeenAt.IsZero() {
		lastSeenAt = sql.NullString{String: domain.FormatTimestamp(bookmark.LastSeenAt), Valid: true}
	}

	// Nil means "unknown rank", which is not the same as rank zero.
	timelineRank := sql.NullInt64{}
	if bookmark.TimelineRank != nil {
		timelineRank = sql.NullInt64{Int64: int64(*bookmark.TimelineRank), Valid: true}
	}

	// The reader-owned state: the zero time is NULL, which is in play.
	archivedAt := nullTimestamp(bookmark.ArchivedAt)

	sourceKind := bookmark.SourceKind
	if sourceKind == "" {
		sourceKind = domain.SourceBookmark
	}

	isLongform := 0
	if domain.IsLongformText(bookmark.Text) {
		isLongform = 1
	}

	updatedAt := bookmark.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = bookmark.SavedAt
	}

	return []any{
		bookmark.TweetID, bookmark.TweetURL, bookmark.AuthorName, bookmark.AuthorHandle,
		nullString(bookmark.AuthorAvatarURL), bookmark.Text,
		bookmark.CreatedAtOnX, domain.FormatXTimestampOrEmpty(bookmark.CreatedAtOnX),
		domain.FormatTimestamp(bookmark.SavedAt), lastSeenAt, timelineRank,
		archivedAt,
		hashtagsJSON, sourceKind, mediaJSON, domain.SummarizeMediaKind(bookmark.Media),
		metricsJSON,
		domain.FormatTimestamp(updatedAt), isLongform,
	}, nil
}

// existingIDs returns which of the given ids are already stored.
func existingIDs(ctx context.Context, tx *sql.Tx, ids []string) (map[string]bool, error) {
	found := make(map[string]bool, len(ids))
	for _, batch := range chunkStrings(ids, 500) {
		query := `SELECT tweet_id FROM bookmarks WHERE tweet_id IN (` + placeholders(len(batch)) + `)`
		rows, err := tx.QueryContext(ctx, query, argsFor(batch)...)
		if err != nil {
			return nil, fmt.Errorf("sqlite: check existing bookmarks: %w", err)
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return nil, err
			}
			found[id] = true
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}
	return found, nil
}

// Get returns one bookmark.
func (r *BookmarkRepository) Get(ctx context.Context, tweetID string) (domain.Bookmark, bool, error) {
	query := `SELECT ` + bookmarkColumns + ` FROM bookmarks AS b WHERE b.tweet_id = ?`
	bookmark, err := scanBookmark(r.db.QueryRowContext(ctx, query, tweetID))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Bookmark{}, false, nil
	}
	if err != nil {
		return domain.Bookmark{}, false, fmt.Errorf("sqlite: get bookmark %s: %w", tweetID, err)
	}
	return bookmark, true, nil
}

// GetMany returns the bookmarks with any of the given ids, keyed by id.
//
// One statement per batch of 400 ids, the same bound the tag read uses: SQLite's
// variable limit is per statement, and a page of suggestions can be longer than one
// statement may bind.
func (r *BookmarkRepository) GetMany(ctx context.Context, tweetIDs []string) (map[string]domain.Bookmark, error) {
	bookmarks := make(map[string]domain.Bookmark, len(tweetIDs))
	if len(tweetIDs) == 0 {
		return bookmarks, nil
	}

	for _, batch := range chunkStrings(tweetIDs, 400) {
		query := `SELECT ` + bookmarkColumns + ` FROM bookmarks AS b WHERE b.tweet_id IN (` +
			placeholders(len(batch)) + `)`
		rows, err := r.db.QueryContext(ctx, query, argsFor(batch)...)
		if err != nil {
			return nil, fmt.Errorf("sqlite: get bookmarks: %w", err)
		}
		for rows.Next() {
			bookmark, err := scanBookmark(rows)
			if err != nil {
				rows.Close()
				return nil, fmt.Errorf("sqlite: scan bookmark: %w", err)
			}
			bookmarks[bookmark.TweetID] = bookmark
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}
	return bookmarks, nil
}

// AllIDs returns every stored tweet id.
func (r *BookmarkRepository) AllIDs(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT tweet_id FROM bookmarks`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list bookmark ids: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// KnownIDs returns every id this device has seen, tombstones included.
//
// The sync's early stop asks "have I seen this post before". A locally deleted
// post was seen: X still returns it, upsert skips it, and treating it as new
// would keep the walk from stopping for as many pages as there are tombstones
// near the top of the timeline.
func (r *BookmarkRepository) KnownIDs(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT tweet_id FROM bookmarks UNION SELECT tweet_id FROM deleted_bookmarks`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list known bookmark ids: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// All returns every bookmark, newest first.
func (r *BookmarkRepository) All(ctx context.Context) ([]domain.Bookmark, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+bookmarkColumns+` FROM bookmarks AS b ORDER BY b.saved_at DESC, b.tweet_id ASC`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list all bookmarks: %w", err)
	}
	defer rows.Close()

	var bookmarks []domain.Bookmark
	for rows.Next() {
		bookmark, err := scanBookmark(rows)
		if err != nil {
			return nil, fmt.Errorf("sqlite: scan bookmark: %w", err)
		}
		bookmarks = append(bookmarks, bookmark)
	}
	return bookmarks, rows.Err()
}

// RawPayloads returns the stored response bodies for the given bookmarks.
func (r *BookmarkRepository) RawPayloads(ctx context.Context, tweetIDs []string) ([]ports.RawPayload, error) {
	if len(tweetIDs) == 0 {
		return nil, nil
	}

	var payloads []ports.RawPayload
	for _, batch := range chunkStrings(tweetIDs, 400) {
		query := `SELECT tweet_id, payload_json, captured_at FROM raw_payloads
		          WHERE tweet_id IN (` + placeholders(len(batch)) + `)`
		rows, err := r.db.QueryContext(ctx, query, argsFor(batch)...)
		if err != nil {
			return nil, fmt.Errorf("sqlite: raw payloads: %w", err)
		}
		for rows.Next() {
			var payload ports.RawPayload
			if err := rows.Scan(&payload.TweetID, &payload.PayloadJSON, &payload.CapturedAt); err != nil {
				rows.Close()
				return nil, fmt.Errorf("sqlite: scan raw payload: %w", err)
			}
			payloads = append(payloads, payload)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}
	return payloads, nil
}

// SaveRawPayloads stores untouched response bodies.
func (r *BookmarkRepository) SaveRawPayloads(ctx context.Context, payloads []ports.RawPayload) error {
	if len(payloads) == 0 {
		return nil
	}
	now := domain.FormatTimestamp(r.now())
	for _, batch := range chunkRawPayloads(payloads, 200) {
		tx, err := r.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("sqlite: begin raw payloads: %w", err)
		}
		statement, err := tx.PrepareContext(ctx, `INSERT INTO raw_payloads (tweet_id, payload_json, captured_at)
			VALUES (?, ?, ?)
			ON CONFLICT(tweet_id) DO UPDATE SET payload_json = excluded.payload_json, captured_at = excluded.captured_at`)
		if err != nil {
			tx.Rollback() //nolint:errcheck // the prepare error is the one to report
			return fmt.Errorf("sqlite: prepare raw payload: %w", err)
		}
		for _, payload := range batch {
			captured := payload.CapturedAt
			if captured == "" {
				captured = now
			}
			if _, err := statement.ExecContext(ctx, payload.TweetID, payload.PayloadJSON, captured); err != nil {
				statement.Close()
				tx.Rollback() //nolint:errcheck // the exec error is the one to report
				return fmt.Errorf("sqlite: store raw payload %s: %w", payload.TweetID, err)
			}
		}
		statement.Close()
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("sqlite: commit raw payloads: %w", err)
		}
	}
	return nil
}

func chunkRawPayloads(payloads []ports.RawPayload, size int) [][]ports.RawPayload {
	var chunks [][]ports.RawPayload
	for len(payloads) > size {
		chunks = append(chunks, payloads[:size])
		payloads = payloads[size:]
	}
	if len(payloads) > 0 {
		chunks = append(chunks, payloads)
	}
	return chunks
}

// TagsByBookmark returns the tags on each of the given bookmarks.
func (r *BookmarkRepository) TagsByBookmark(ctx context.Context, tweetIDs []string) (map[string][]domain.Tag, error) {
	result := make(map[string][]domain.Tag)
	if len(tweetIDs) == 0 {
		return result, nil
	}

	for _, batch := range chunkStrings(tweetIDs, 400) {
		query := `SELECT bt.bookmark_id, t.id, t.name, t.canonical_key, t.aliases_json,
		                 t.parent_id, t.created_at, t.updated_at
		          FROM bookmark_tags bt
		          JOIN tags t ON t.id = bt.tag_id
		          WHERE bt.bookmark_id IN (` + placeholders(len(batch)) + `)
		          ORDER BY bt.bookmark_id, t.name`

		rows, err := r.db.QueryContext(ctx, query, argsFor(batch)...)
		if err != nil {
			return nil, fmt.Errorf("sqlite: tags for bookmarks: %w", err)
		}
		for rows.Next() {
			var (
				bookmarkID string
				tag        domain.Tag
				aliasesRaw sql.NullString
				parentID   sql.NullString
				createdAt  string
				updatedAt  string
			)
			if err := rows.Scan(&bookmarkID, &tag.ID, &tag.Name, &tag.CanonicalKey, &aliasesRaw,
				&parentID, &createdAt, &updatedAt); err != nil {
				rows.Close()
				return nil, fmt.Errorf("sqlite: scan tag: %w", err)
			}

			tag.Aliases, _ = decodeStrings(aliasesRaw)
			// domain.Tag keeps these as the raw stored strings, so nothing is
			// re-rendered on the way out.
			tag.ParentID = parentID.String
			tag.CreatedAt = createdAt
			tag.UpdatedAt = updatedAt

			result[bookmarkID] = append(result[bookmarkID], tag)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}

	return result, nil
}

// ListByBookmark returns the list each of the given bookmarks belongs to.
func (r *BookmarkRepository) ListByBookmark(ctx context.Context, tweetIDs []string) (map[string]domain.List, error) {
	result := make(map[string]domain.List)
	if len(tweetIDs) == 0 {
		return result, nil
	}

	for _, batch := range chunkStrings(tweetIDs, 500) {
		query := `SELECT bl.bookmark_id, l.id, l.name, l.normalized_name, l.created_at, l.updated_at
		          FROM bookmark_lists bl
		          JOIN lists l ON l.id = bl.list_id
		          WHERE bl.bookmark_id IN (` + placeholders(len(batch)) + `)`

		rows, err := r.db.QueryContext(ctx, query, argsFor(batch)...)
		if err != nil {
			return nil, fmt.Errorf("sqlite: lists for bookmarks: %w", err)
		}
		for rows.Next() {
			var (
				bookmarkID string
				list       domain.List
				createdAt  string
				updatedAt  string
			)
			if err := rows.Scan(&bookmarkID, &list.ID, &list.Name, &list.Normalized, &createdAt, &updatedAt); err != nil {
				rows.Close()
				return nil, fmt.Errorf("sqlite: scan list: %w", err)
			}
			list.CreatedAt = domain.MustParseTimestamp(createdAt)
			list.UpdatedAt = domain.MustParseTimestamp(updatedAt)
			result[bookmarkID] = list
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}

	return result, nil
}

// AssignToList moves bookmarks into a list, replacing any current list.
func (r *BookmarkRepository) AssignToList(ctx context.Context, tweetIDs []string, listID string, now time.Time) error {
	if len(tweetIDs) == 0 {
		return nil
	}
	if strings.TrimSpace(listID) == "" {
		return errors.New("sqlite: a list id is required")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlite: begin assign to list: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op once committed

	timestamp := domain.FormatTimestamp(now)
	statement := `INSERT INTO bookmark_lists (bookmark_id, list_id, updated_at) VALUES (?, ?, ?)
	              ON CONFLICT(bookmark_id) DO UPDATE SET
	                list_id = excluded.list_id, updated_at = excluded.updated_at`

	for _, tweetID := range tweetIDs {
		if _, err := tx.ExecContext(ctx, statement, tweetID, listID, timestamp); err != nil {
			return fmt.Errorf("sqlite: assign bookmark %s to list %s: %w", tweetID, listID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("sqlite: commit assign to list: %w", err)
	}
	return nil
}

// AssignMissingToInbox gives every bookmark without a list the Inbox.
//
// It is what keeps "every bookmark belongs to exactly one list" true, and it must
// run after a sync inserts rows: a bookmark created without a list would
// otherwise be invisible in every list view, including the Inbox.
func (r *BookmarkRepository) AssignMissingToInbox(ctx context.Context, now time.Time) (int, error) {
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO bookmark_lists (bookmark_id, list_id, updated_at)
		 SELECT b.tweet_id, ?, ?
		 FROM bookmarks b
		 WHERE NOT EXISTS (SELECT 1 FROM bookmark_lists bl WHERE bl.bookmark_id = b.tweet_id)`,
		domain.InboxListID, domain.FormatTimestamp(now))
	if err != nil {
		return 0, fmt.Errorf("sqlite: assign missing bookmarks to Inbox: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(affected), nil
}

// SetArchived moves one bookmark into or out of the archive.
//
// A zero time is the way back: archived_at NULL is "in play", the same value every row
// carried before the archive existed. Archiving keeps the row and everything attached to
// it -- a restore into an empty library brings the state back with it.
func (r *BookmarkRepository) SetArchived(ctx context.Context, tweetID string, at time.Time) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE bookmarks SET archived_at = ? WHERE tweet_id = ?`, nullTimestamp(at), tweetID)
	if err != nil {
		return fmt.Errorf("sqlite: set archived for %s: %w", tweetID, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return fmt.Errorf("sqlite: no bookmark %s", tweetID)
	}
	return nil
}

// SetArchivedMany moves a batch into or out of the archive in one write per chunk.
//
// One UPDATE over the chunked id set rather than a call per row: the batch bar can name
// a few thousand ids, and a transaction per bookmark would take the only connection for
// as long as the reader's click takes to become a list.
func (r *BookmarkRepository) SetArchivedMany(ctx context.Context, tweetIDs []string, at time.Time) error {
	if len(tweetIDs) == 0 {
		return nil
	}
	archivedAt := nullTimestamp(at)
	for _, batch := range chunkStrings(tweetIDs, 500) {
		if _, err := r.db.ExecContext(ctx,
			`UPDATE bookmarks SET archived_at = ? WHERE tweet_id IN (`+placeholders(len(batch))+`)`,
			append([]any{archivedAt}, argsFor(batch)...)...); err != nil {
			return fmt.Errorf("sqlite: set archived for %d bookmarks: %w", len(batch), err)
		}
	}
	return nil
}

// Counts returns the sidebar aggregates in one round trip.
func (r *BookmarkRepository) Counts(ctx context.Context) (domain.WorkspaceCounts, error) {
	var counts domain.WorkspaceCounts

	// Unclassified is "carries no tag", independent of list membership.
	query := `
		SELECT
		  (SELECT COUNT(*) FROM bookmarks),
		  (SELECT COUNT(*) FROM bookmarks b
		     WHERE NOT EXISTS (SELECT 1 FROM bookmark_tags bt WHERE bt.bookmark_id = b.tweet_id)),
		  (SELECT COUNT(*) FROM tag_suggestions WHERE status = 'pending'),
		  (SELECT COUNT(*) FROM tags),
		  (SELECT COUNT(*) FROM lists),
		  (SELECT COUNT(*) FROM bookmarks WHERE archived_at IS NOT NULL)`

	if err := r.db.QueryRowContext(ctx, query).Scan(
		&counts.TotalBookmarks,
		&counts.UnclassifiedCount,
		&counts.PendingSuggestions,
		&counts.TagCount,
		&counts.ListCount,
		&counts.ArchivedCount,
	); err != nil {
		return domain.WorkspaceCounts{}, fmt.Errorf("sqlite: workspace counts: %w", err)
	}
	return counts, nil
}

// FacetCounts counts, for each tag and each author, how many bookmarks under a filter
// carry that value.
//
// Two aggregate queries, not one per row: the sidebar shows every tag and every author,
// and a count behind each of them would be a hundred queries per keystroke. The filter is
// the same bookmarkFilterSQL the result list uses, so a row's number and the list it opens
// cannot disagree about what the conditions mean -- with the default all-of tag reading,
// the number is exactly what clicking the row produces.
func (r *BookmarkRepository) FacetCounts(ctx context.Context, filter domain.LibraryFilter) (domain.FacetCounts, error) {
	normalized := filter.Normalize()
	counts := domain.FacetCounts{
		Ready:   true,
		Tags:    map[string]int{},
		Authors: map[string]int{},
	}

	// The facet's own value is dropped: a row's number is what clicking it shows, and a
	// click replaces the value already in effect rather than intersecting with it.
	tagFilter := normalized
	tagFilter.TagIDs = nil
	where, args := bookmarkFilterSQL(tagFilter, r.now())
	// The filter is applied in a subquery rather than as a join. Same rows, but SQLite
	// builds the matching id set once and walks the tag index from there, where a join
	// probed the primary key per relation: measured on a 10k library with one tag, 6.6ms
	// against 12.1ms.
	tagQuery := `SELECT tag_id, COUNT(*) FROM bookmark_tags
		WHERE bookmark_id IN (SELECT b.tweet_id FROM bookmarks AS b` + where + `)
		GROUP BY tag_id`
	rows, err := r.db.QueryContext(ctx, tagQuery, args...)
	if err != nil {
		return domain.FacetCounts{}, fmt.Errorf("sqlite: tag facet counts: %w", err)
	}
	for rows.Next() {
		var id string
		var count int
		if err := rows.Scan(&id, &count); err != nil {
			rows.Close()
			return domain.FacetCounts{}, err
		}
		counts.Tags[id] = count
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return domain.FacetCounts{}, err
	}
	rows.Close()

	// Authors, on the same rule: the author already in effect does not narrow the count of
	// the author a click would replace it with. Grouped on the stored handle, which is what
	// the sidebar row is keyed by, so the lookup cannot miss on case.
	authorFilter := normalized
	authorFilter.AuthorHandles = nil
	where, args = bookmarkFilterSQL(authorFilter, r.now())
	// A bookmark with no handle has no sidebar row to count for (AuthorSummaries skips
	// them too), so it is left out of the groups rather than folded into a nameless one.
	where += "\n   AND b.author_handle <> ''"
	rows, err = r.db.QueryContext(ctx,
		`SELECT b.author_handle, COUNT(*) FROM bookmarks AS b`+where+`
		 GROUP BY b.author_handle`, args...)
	if err != nil {
		return domain.FacetCounts{}, fmt.Errorf("sqlite: author facet counts: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var handle string
		var count int
		if err := rows.Scan(&handle, &count); err != nil {
			return domain.FacetCounts{}, err
		}
		counts.Authors[handle] = count
	}
	if err := rows.Err(); err != nil {
		return domain.FacetCounts{}, err
	}
	return counts, nil
}

// AuthorSummaries returns the most-bookmarked authors.
func (r *BookmarkRepository) AuthorSummaries(ctx context.Context, limit int) ([]domain.AuthorSummary, error) {
	if limit <= 0 {
		limit = 50
	}

	// Grouped on the handle alone, and the name comes from an arbitrary row: taking
	// whatever the first record held keeps which name a user sees stable, and a
	// "most recent" rule here would change it.
	// Ordered by count only. The handle order is applied in Go, for the reason tag_repo.go
	// gives: natural comparison breaks the tie, and SQL's ORDER BY is a byte
	// comparison. The limit cannot stay in the query either -- a LIMIT after a byte-order
	// sort keeps the wrong rows, so the rows are all fetched, ordered, and then cut.
	query := `
		SELECT author_handle, MAX(author_name) AS name,
		       COALESCE(MAX(author_avatar_url), '') AS avatar, COUNT(*) AS total
		FROM bookmarks
		WHERE author_handle <> ''
		GROUP BY author_handle
		ORDER BY total DESC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("sqlite: author summaries: %w", err)
	}
	defer rows.Close()

	var summaries []domain.AuthorSummary
	for rows.Next() {
		var summary domain.AuthorSummary
		if err := rows.Scan(&summary.AuthorHandle, &summary.AuthorName,
			&summary.AuthorAvatarURL, &summary.BookmarkCount); err != nil {
			return nil, err
		}
		summaries = append(summaries, summary)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Stable, so authors with equal counts keep the order the database gave them.
	sort.SliceStable(summaries, func(i, j int) bool {
		if summaries[i].BookmarkCount != summaries[j].BookmarkCount {
			return summaries[i].BookmarkCount > summaries[j].BookmarkCount
		}
		return domain.CompareNaturalNames(summaries[i].AuthorHandle, summaries[j].AuthorHandle) < 0
	})
	if limit > 0 && len(summaries) > limit {
		summaries = summaries[:limit]
	}
	return summaries, nil
}
