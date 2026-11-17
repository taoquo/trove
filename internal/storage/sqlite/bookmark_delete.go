package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/ports"
)

// tombstonedIDs returns which of the given ids the user deleted locally.
func tombstonedIDs(ctx context.Context, tx *sql.Tx, ids []string) (map[string]bool, error) {
	found := make(map[string]bool, len(ids))
	for _, batch := range chunkStrings(ids, 500) {
		query := `SELECT tweet_id FROM deleted_bookmarks WHERE tweet_id IN (` + placeholders(len(batch)) + `)`
		rows, err := tx.QueryContext(ctx, query, argsFor(batch)...)
		if err != nil {
			return nil, fmt.Errorf("sqlite: check bookmark tombstones: %w", err)
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

// Delete removes one bookmark and records its tombstone.
func (r *BookmarkRepository) Delete(ctx context.Context, tweetID string) error {
	if _, err := r.deleteBookmarks(ctx, []string{tweetID}); err != nil {
		return fmt.Errorf("sqlite: delete bookmark %s: %w", tweetID, err)
	}
	return nil
}

// DeleteMany removes several bookmarks and returns how many existed.
func (r *BookmarkRepository) DeleteMany(ctx context.Context, tweetIDs []string) (int, error) {
	return r.deleteBookmarks(ctx, tweetIDs)
}

// deleteBookmarks removes rows and writes a tombstone for every id, in one
// transaction.
//
// Both halves must land together. A delete without a tombstone is the F7-1
// defect: the next sync upserts the same tweet and the bookmark returns. A
// tombstone without a delete would silently hide a row the user still has.
func (r *BookmarkRepository) deleteBookmarks(ctx context.Context, tweetIDs []string) (int, error) {
	if len(tweetIDs) == 0 {
		return 0, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("sqlite: begin delete bookmarks: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op once committed

	tombstone, err := tx.PrepareContext(ctx,
		`INSERT INTO deleted_bookmarks (tweet_id, deleted_at) VALUES (?, ?)
		 ON CONFLICT(tweet_id) DO UPDATE SET deleted_at = excluded.deleted_at`)
	if err != nil {
		return 0, fmt.Errorf("sqlite: prepare bookmark tombstone: %w", err)
	}
	defer tombstone.Close()

	deletedAt := domain.FormatTimestamp(r.now())
	total := 0
	for _, batch := range chunkStrings(tweetIDs, 500) {
		result, err := tx.ExecContext(ctx,
			`DELETE FROM bookmarks WHERE tweet_id IN (`+placeholders(len(batch))+`)`, argsFor(batch)...)
		if err != nil {
			return 0, fmt.Errorf("sqlite: delete bookmarks: %w", err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}
		total += int(affected)

		// Every requested id is tombstoned, not just the rows that existed: the
		// user's intent is what must survive, and an id that had already gone
		// would otherwise be free for the next sync to bring back.
		for _, tweetID := range batch {
			if _, err := tombstone.ExecContext(ctx, tweetID, deletedAt); err != nil {
				return 0, fmt.Errorf("sqlite: record tombstone for %s: %w", tweetID, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("sqlite: commit delete bookmarks: %w", err)
	}
	return total, nil
}

// DeleteAll empties the library and returns how many rows went.
//
// The dependent rows go with it: bookmark_tags, bookmark_lists, raw_payloads and
// llm_analysis all cascade. tag_suggestions does not, because resolved rows are
// the negative cache and must outlive the bookmarks they were made for.
func (r *BookmarkRepository) DeleteAll(ctx context.Context) (int, error) {
	var before int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM bookmarks`).Scan(&before); err != nil {
		return 0, fmt.Errorf("sqlite: count bookmarks: %w", err)
	}

	// Emptying the library is a fresh start, so the tombstones go too: they only
	// exist to keep a sync from re-importing a local delete, and a library the
	// user deliberately emptied must be able to fill again. This runs before the
	// empty check because tombstones can outlive every bookmark row, and a
	// restore must clear them even when it is restoring into an empty library.
	if _, err := r.db.ExecContext(ctx, `DELETE FROM deleted_bookmarks`); err != nil {
		return 0, fmt.Errorf("sqlite: clear bookmark tombstones: %w", err)
	}
	if before == 0 {
		return 0, nil
	}

	// Pending suggestions are dropped explicitly: the schema gives them no foreign
	// key on purpose, so nothing else will.
	if _, err := r.db.ExecContext(ctx,
		`DELETE FROM tag_suggestions WHERE status = 'pending'`); err != nil {
		return 0, fmt.Errorf("sqlite: drop pending suggestions: %w", err)
	}
	if _, err := r.db.ExecContext(ctx, `DELETE FROM bookmarks`); err != nil {
		return 0, fmt.Errorf("sqlite: delete bookmarks: %w", err)
	}
	return before, nil
}

// ClearTombstones forgets the local-delete record for the given ids.
//
// An import carries bookmarks the user explicitly asked for, so for those ids
// the file is authoritative and a remembered delete must not silently drop
// them. Ids outside the file keep their tombstones.
func (r *BookmarkRepository) ClearTombstones(ctx context.Context, tweetIDs []string) error {
	if len(tweetIDs) == 0 {
		return nil
	}

	for _, batch := range chunkStrings(tweetIDs, 500) {
		if _, err := r.db.ExecContext(ctx,
			`DELETE FROM deleted_bookmarks WHERE tweet_id IN (`+placeholders(len(batch))+`)`,
			argsFor(batch)...); err != nil {
			return fmt.Errorf("sqlite: clear bookmark tombstones: %w", err)
		}
	}
	return nil
}

// Tombstones returns the record of posts deleted locally, oldest id first.
//
// The order is stable so an export is reproducible: two backups of an unchanged
// library must differ only in the timestamp they record.
func (r *BookmarkRepository) Tombstones(ctx context.Context) ([]ports.Tombstone, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT tweet_id, deleted_at FROM deleted_bookmarks ORDER BY tweet_id`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list bookmark tombstones: %w", err)
	}
	defer rows.Close()

	var tombstones []ports.Tombstone
	for rows.Next() {
		var tombstone ports.Tombstone
		if err := rows.Scan(&tombstone.TweetID, &tombstone.DeletedAt); err != nil {
			return nil, fmt.Errorf("sqlite: scan bookmark tombstone: %w", err)
		}
		tombstones = append(tombstones, tombstone)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: list bookmark tombstones: %w", err)
	}
	return tombstones, nil
}

// AddTombstones records deletes that happened elsewhere.
//
// It is the write half of a restore: the bundle carries the record of what its
// device deleted, and without it the next sync brings those posts back. Nothing is
// deleted here -- the bookmarks themselves are the bundle's business -- and an
// existing tombstone keeps the earlier date, because the delete already happened
// once and a later copy of the record is not a new one.
func (r *BookmarkRepository) AddTombstones(ctx context.Context, tombstones []ports.Tombstone) error {
	if len(tombstones) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlite: begin add bookmark tombstones: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op once committed

	statement, err := tx.PrepareContext(ctx,
		`INSERT INTO deleted_bookmarks (tweet_id, deleted_at) VALUES (?, ?)
		 ON CONFLICT(tweet_id) DO NOTHING`)
	if err != nil {
		return fmt.Errorf("sqlite: prepare bookmark tombstone: %w", err)
	}
	defer statement.Close()

	for _, tombstone := range tombstones {
		if tombstone.TweetID == "" {
			continue
		}
		deletedAt := tombstone.DeletedAt
		if deletedAt == "" {
			deletedAt = domain.FormatTimestamp(r.now())
		}
		if _, err := statement.ExecContext(ctx, tombstone.TweetID, deletedAt); err != nil {
			return fmt.Errorf("sqlite: record tombstone for %s: %w", tombstone.TweetID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("sqlite: commit add bookmark tombstones: %w", err)
	}
	return nil
}
