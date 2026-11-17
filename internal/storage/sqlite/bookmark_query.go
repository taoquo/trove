package sqlite

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/taoquo/trove/internal/domain"
)

// Count returns how many bookmarks match a filter.
func (r *BookmarkRepository) Count(ctx context.Context, filter domain.LibraryFilter) (int, error) {
	where, args := bookmarkFilterSQL(filter, r.now())
	query := `SELECT COUNT(*) FROM bookmarks AS b` + where

	var count int
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("sqlite: count bookmarks: %w", err)
	}
	return count, nil
}

// Query returns the bookmarks matching a filter, in the requested order.
func (r *BookmarkRepository) Query(ctx context.Context, query domain.LibraryQuery) ([]domain.Bookmark, error) {
	normalized, err := query.Normalize()
	if err != nil {
		return nil, err
	}

	where, args := bookmarkFilterSQL(normalized.Filter, r.now())
	// The order is chosen from a fixed map, never built from the caller's string:
	// an ORDER BY assembled from input is an injection point, and this one is
	// reachable from a restored saved view.
	orderBy, ok := bookmarkOrderSQL[normalized.Sort]
	if !ok {
		return nil, fmt.Errorf("sqlite: no ordering for sort %q", normalized.Sort)
	}

	statement := `SELECT ` + bookmarkColumns + ` FROM bookmarks AS b` + where +
		` ORDER BY ` + orderBy + ` LIMIT ? OFFSET ?`
	args = append(args, normalized.Limit, normalized.Offset)

	rows, err := r.db.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, fmt.Errorf("sqlite: query bookmarks: %w", err)
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

// bookmarkOrderSQL maps a sort order to its ORDER BY clause.
//
// Every clause ends with the tweet id so paging is stable: without a total order,
// two rows that compare equal can swap between pages and a bookmark is shown
// twice while another is never shown.
var bookmarkOrderSQL = map[domain.SortOrder]string{
	// Rank ascending, but a known rank always beats an unknown one. The sentinel is
	// the largest integer, which no rank reaches, so COALESCE sorts nulls last; the
	// same order the older "rank IS NULL, rank" produced, written so the expression
	// index in schemaV10 can match it. The tie-break is saved_at descending,
	// matching that order.
	domain.SortTimeline:  `COALESCE(b.timeline_rank, 9223372036854775807) ASC, b.saved_at DESC, b.tweet_id ASC`,
	domain.SortSavedDesc: `b.saved_at DESC, b.tweet_id ASC`,
	domain.SortSavedAsc:  `b.saved_at ASC, b.tweet_id ASC`,
	// Sorted on the parsed copy, never on the verbatim X rendering: that text
	// begins with a weekday name, so sorting it orders by weekday.
	domain.SortCreatedDesc: `b.created_at_on_x_ts DESC, b.tweet_id ASC`,
	// A post with no metrics counts as zero likes.
	domain.SortLikesDesc: `COALESCE(json_extract(b.metrics_json, '$.likes'), 0) DESC, b.tweet_id ASC`,
}

// bookmarkFilterSQL renders the WHERE clause for a filter.
//
// Every value is bound, never interpolated. The only text spliced in is a run of
// placeholders, whose length comes from the number of values.
func bookmarkFilterSQL(filter domain.LibraryFilter, now time.Time) (string, []any) {
	normalized := filter.Normalize()

	var clauses []string
	var args []any

	if normalized.Query != "" {
		// Search covers text, author name, author handle, the post URL, the names of
		// the tags on a bookmark and the name of its list, so a pasted link finds its
		// own bookmark and a tag name finds what the user filed under it.
		//
		// The bookmark columns are the folded copies the schema keeps, not trove_fold of
		// the originals: folding in the WHERE is a Go call per row per column, and a scan
		// touches every row. The tag and list names are folded in place: those tables are
		// small, and the EXISTS only walks the joins for the rows the outer query kept.
		like := "%" + escapeLike(strings.ToLower(normalized.Query)) + "%"
		clauses = append(clauses, `(b.text_folded LIKE ? ESCAPE '\'`+
			` OR b.author_name_folded LIKE ? ESCAPE '\'`+
			` OR b.author_handle_folded LIKE ? ESCAPE '\'`+
			` OR b.tweet_url_folded LIKE ? ESCAPE '\'`+
			` OR EXISTS (SELECT 1 FROM bookmark_tags bt JOIN tags t ON t.id = bt.tag_id`+
			` WHERE bt.bookmark_id = b.tweet_id AND trove_fold(t.name) LIKE ? ESCAPE '\')`+
			` OR EXISTS (SELECT 1 FROM bookmark_lists bl JOIN lists l ON l.id = bl.list_id`+
			` WHERE bl.bookmark_id = b.tweet_id AND trove_fold(l.name) LIKE ? ESCAPE '\'))`)
		args = append(args, like, like, like, like, like, like)
	}

	if normalized.OnlyWithPendingSuggestions {
		clauses = append(clauses, `EXISTS (SELECT 1 FROM tag_suggestions ts
			WHERE ts.bookmark_id = b.tweet_id AND ts.status = 'pending')`)
	}

	if len(normalized.TagIDs) > 0 {
		if normalized.TagMatchMode == domain.MatchAny {
			clauses = append(clauses, `EXISTS (SELECT 1 FROM bookmark_tags bt
				WHERE bt.bookmark_id = b.tweet_id AND bt.tag_id IN (`+placeholders(len(normalized.TagIDs))+`))`)
			args = append(args, argsFor(normalized.TagIDs)...)
		} else {
			// A separate EXISTS per tag is what makes "all" mean all: a single IN
			// would match a bookmark carrying any one of them.
			for _, tagID := range normalized.TagIDs {
				clauses = append(clauses, `EXISTS (SELECT 1 FROM bookmark_tags bt
					WHERE bt.bookmark_id = b.tweet_id AND bt.tag_id = ?)`)
				args = append(args, tagID)
			}
		}
	}

	if len(normalized.AuthorHandles) > 0 {
		// Matched against the handle and the display name together, so either
		// one resolves an author.
		if normalized.AuthorMatchMode == domain.MatchAll {
			for _, handle := range normalized.AuthorHandles {
				clauses = append(clauses, `(b.author_handle_folded = ? OR b.author_name_folded = ?)`)
				args = append(args, foldedAuthor(handle), foldedAuthor(handle))
			}
		} else {
			clauses = append(clauses, `(b.author_handle_folded IN (`+placeholders(len(normalized.AuthorHandles))+`)`+
				` OR b.author_name_folded IN (`+placeholders(len(normalized.AuthorHandles))+`))`)
			for _, handle := range normalized.AuthorHandles {
				args = append(args, foldedAuthor(handle))
			}
			for _, handle := range normalized.AuthorHandles {
				args = append(args, foldedAuthor(handle))
			}
		}
	}

	if normalized.ListID != "" {
		clauses = append(clauses, `EXISTS (SELECT 1 FROM bookmark_lists bl
			WHERE bl.bookmark_id = b.tweet_id AND bl.list_id = ?)`)
		args = append(args, normalized.ListID)
	}

	if normalized.OnlyUnclassified {
		clauses = append(clauses, `NOT EXISTS (SELECT 1 FROM bookmark_tags bt
			WHERE bt.bookmark_id = b.tweet_id)`)
	}

	if normalized.OnlyWithMedia {
		// The denormalized kind, not media_json: the summary is written on every
		// upsert and backfilled once, so the condition stays a column test.
		clauses = append(clauses, `b.media_kind <> ''`)
	}

	if normalized.OnlyWithVideo {
		// A video filter is a subset of "has media", so it is applied whether or not
		// the media toggle is also on. An animated GIF plays, so it belongs here.
		clauses = append(clauses, `b.media_kind IN ('video', 'animated_gif')`)
	}

	if normalized.OnlyLongform {
		// The denormalized flag, not length(text): SQLite counts code points while
		// the rule counts UTF-16 code units, and the two disagree on any post with
		// an emoji.
		clauses = append(clauses, `b.is_longform = 1`)
	}

	// The archive is a mode of the view rather than a toggle, but it is applied here
	// with every other condition so no caller can forget it: a list that quietly kept
	// showing archived rows would make the archive unreachable rather than merely
	// optional. Hidden is unconditional rather than spliced out of the clause list,
	// because "everything except the archive" is what the app shows by default.
	if normalized.Archived == domain.ArchivedOnly {
		clauses = append(clauses, `b.archived_at IS NOT NULL`)
	} else {
		clauses = append(clauses, `b.archived_at IS NULL`)
	}

	// The cutoff comes from the domain, so the filter's rule exists once. Timestamps
	// are fixed-width UTC ISO strings, so TEXT comparison is chronological, and
	// passing the cutoff as a bound value keeps the comparison index-friendly.
	if cutoff := normalized.SavedAfter(now); !cutoff.IsZero() {
		clauses = append(clauses, `b.saved_at >= ?`)
		args = append(args, domain.FormatTimestamp(cutoff))
	}

	// There is never an empty clause list now that the archive decides every view, so
	// the result is always a WHERE. Callers that append a condition of their own can
	// therefore append it with AND.
	return "\n WHERE " + strings.Join(clauses, "\n   AND "), args
}
