package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/ports"
)

// SuggestionRepository stores tag suggestions.
type SuggestionRepository struct {
	db *DB
}

// NewSuggestionRepository builds a repository over an open database.
func NewSuggestionRepository(db *DB) *SuggestionRepository { return &SuggestionRepository{db: db} }

var _ ports.SuggestionRepository = (*SuggestionRepository)(nil)

const suggestionColumns = `id, bookmark_id, tag_name, source, confidence, status, created_at, resolved_at`

// scanSuggestion reads one suggestion row.
func scanSuggestion(scanner interface{ Scan(...any) error }) (domain.TagSuggestion, error) {
	var (
		suggestion domain.TagSuggestion
		createdAt  string
		resolvedAt sql.NullString
	)
	err := scanner.Scan(&suggestion.ID, &suggestion.BookmarkID, &suggestion.TagName,
		&suggestion.Source, &suggestion.Confidence, &suggestion.Status, &createdAt, &resolvedAt)
	if err != nil {
		return domain.TagSuggestion{}, err
	}

	suggestion.CreatedAt = domain.MustParseTimestamp(createdAt)
	if resolvedAt.Valid && resolvedAt.String != "" {
		resolved := domain.MustParseTimestamp(resolvedAt.String)
		suggestion.ResolvedAt = &resolved
	}
	return suggestion, nil
}

// List returns the suggestions in a given status, newest first.
func (r *SuggestionRepository) List(ctx context.Context, status domain.SuggestionStatus) ([]domain.TagSuggestion, error) {
	query := `SELECT ` + suggestionColumns + ` FROM tag_suggestions WHERE status = ? ORDER BY created_at DESC, id ASC`
	rows, err := r.db.QueryContext(ctx, query, status)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list suggestions: %w", err)
	}
	return collectSuggestions(rows)
}

// All returns every suggestion.
func (r *SuggestionRepository) All(ctx context.Context) ([]domain.TagSuggestion, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+suggestionColumns+` FROM tag_suggestions ORDER BY created_at ASC, id ASC`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list all suggestions: %w", err)
	}
	return collectSuggestions(rows)
}

// DeleteAll removes every suggestion and returns how many went.
func (r *SuggestionRepository) DeleteAll(ctx context.Context) (int, error) {
	var before int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tag_suggestions`).Scan(&before); err != nil {
		return 0, fmt.Errorf("sqlite: count suggestions: %w", err)
	}
	if before == 0 {
		return 0, nil
	}
	if _, err := r.db.ExecContext(ctx, `DELETE FROM tag_suggestions`); err != nil {
		return 0, fmt.Errorf("sqlite: delete suggestions: %w", err)
	}
	return before, nil
}

// Get returns one suggestion by id.
func (r *SuggestionRepository) Get(ctx context.Context, id string) (domain.TagSuggestion, bool, error) {
	suggestion, err := scanSuggestion(r.db.QueryRowContext(ctx,
		`SELECT `+suggestionColumns+` FROM tag_suggestions WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.TagSuggestion{}, false, nil
	}
	if err != nil {
		return domain.TagSuggestion{}, false, fmt.Errorf("sqlite: get suggestion %s: %w", id, err)
	}
	return suggestion, true, nil
}

// ListForBookmark returns every suggestion made for one bookmark.
//
// Resolved rows are included: that is what makes the pair a negative cache.
func (r *SuggestionRepository) ListForBookmark(ctx context.Context, tweetID string) ([]domain.TagSuggestion, error) {
	query := `SELECT ` + suggestionColumns + ` FROM tag_suggestions WHERE bookmark_id = ? ORDER BY created_at DESC, id ASC`
	rows, err := r.db.QueryContext(ctx, query, tweetID)
	if err != nil {
		return nil, fmt.Errorf("sqlite: suggestions for bookmark: %w", err)
	}
	return collectSuggestions(rows)
}

func collectSuggestions(rows *sql.Rows) ([]domain.TagSuggestion, error) {
	defer rows.Close()

	var suggestions []domain.TagSuggestion
	for rows.Next() {
		suggestion, err := scanSuggestion(rows)
		if err != nil {
			return nil, err
		}
		suggestions = append(suggestions, suggestion)
	}
	return suggestions, rows.Err()
}

// AllIDs returns every suggestion id.
//
// This is the auto-tagger's negative cache. It covers every status on purpose: a
// suggestion the user already accepted or rejected must never be offered again.
func (r *SuggestionRepository) AllIDs(ctx context.Context) (map[string]bool, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id FROM tag_suggestions`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list suggestion ids: %w", err)
	}
	defer rows.Close()

	ids := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids[id] = true
	}
	return ids, rows.Err()
}

// InsertMany adds suggestions, skipping ones that already exist.
//
// DO NOTHING rather than an error: two rules can propose the same tag for the
// same post, and the composite id makes them the same suggestion, so a duplicate
// is expected rather than exceptional.
func (r *SuggestionRepository) InsertMany(ctx context.Context, suggestions []domain.TagSuggestion) (int, error) {
	if len(suggestions) == 0 {
		return 0, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("sqlite: begin insert suggestions: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op once committed

	statement, err := tx.PrepareContext(ctx,
		`INSERT INTO tag_suggestions (`+suggestionColumns+`)
		 VALUES (?, ?, ?, ?, ?, ?, ?, NULL)
		 ON CONFLICT(id) DO NOTHING`)
	if err != nil {
		return 0, fmt.Errorf("sqlite: prepare insert suggestions: %w", err)
	}
	defer statement.Close()

	inserted := 0
	for _, suggestion := range suggestions {
		result, err := statement.ExecContext(ctx,
			suggestion.ID, suggestion.BookmarkID, suggestion.TagName, suggestion.Source,
			suggestion.Confidence, suggestion.Status, domain.FormatTimestamp(suggestion.CreatedAt))
		if err != nil {
			return 0, fmt.Errorf("sqlite: insert suggestion %s: %w", suggestion.ID, err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}
		inserted += int(affected)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("sqlite: commit insert suggestions: %w", err)
	}
	return inserted, nil
}

// Resolve stamps an answer on a suggestion that is still pending.
//
// The pending guard is in the WHERE clause, so a double accept cannot rewrite an
// answer the user already gave.
func (r *SuggestionRepository) Resolve(ctx context.Context, id string, status domain.SuggestionStatus, now time.Time) error {
	if status != domain.StatusAccepted && status != domain.StatusRejected {
		return errors.New("sqlite: a suggestion resolves to accepted or rejected")
	}

	_, err := r.db.ExecContext(ctx,
		`UPDATE tag_suggestions SET status = ?, resolved_at = ? WHERE id = ? AND status = 'pending'`,
		status, domain.FormatTimestamp(now), id)
	if err != nil {
		return fmt.Errorf("sqlite: resolve suggestion %s: %w", id, err)
	}
	return nil
}

// RejectByTagNames rejects every pending suggestion for the given names.
//
// Matching is on the canonical key, so "AI Agent" and "ai-agents" are the same
// name here as they are everywhere else.
func (r *SuggestionRepository) RejectByTagNames(ctx context.Context, names []string, now time.Time) (int, error) {
	keys := make([]string, 0, len(names))
	for _, name := range names {
		if key := domain.CanonicalTagKey(name); key != "" {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		return 0, nil
	}

	// The rows hold display names, so they have to be read and compared through
	// the same canonical function rather than matched with SQL string equality.
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, tag_name FROM tag_suggestions WHERE status = 'pending'`)
	if err != nil {
		return 0, fmt.Errorf("sqlite: scan pending suggestions: %w", err)
	}

	type victim struct{ id string }
	var victims []victim
	wanted := make(map[string]bool, len(keys))
	for _, key := range keys {
		wanted[key] = true
	}

	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			rows.Close()
			return 0, err
		}
		if wanted[domain.CanonicalTagKey(name)] {
			victims = append(victims, victim{id: id})
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()

	if len(victims) == 0 {
		return 0, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("sqlite: begin reject suggestions: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op once committed

	statement, err := tx.PrepareContext(ctx,
		`UPDATE tag_suggestions SET status = 'rejected', resolved_at = ? WHERE id = ? AND status = 'pending'`)
	if err != nil {
		return 0, fmt.Errorf("sqlite: prepare reject suggestions: %w", err)
	}
	defer statement.Close()

	rejected := 0
	for _, item := range victims {
		result, err := statement.ExecContext(ctx, domain.FormatTimestamp(now), item.id)
		if err != nil {
			return 0, fmt.Errorf("sqlite: reject suggestion %s: %w", item.id, err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}
		rejected += int(affected)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("sqlite: commit reject suggestions: %w", err)
	}
	return rejected, nil
}

// DropPendingForBookmarks removes pending suggestions for the given bookmarks.
//
// Only pending rows go: resolved ones are the user's answer and must outlive the
// bookmark, which is why the schema gives this table no foreign key.
func (r *SuggestionRepository) DropPendingForBookmarks(ctx context.Context, tweetIDs []string) (int, error) {
	if len(tweetIDs) == 0 {
		return 0, nil
	}

	total := 0
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("sqlite: begin drop suggestions: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op once committed

	for _, batch := range chunkStrings(tweetIDs, 500) {
		result, err := tx.ExecContext(ctx,
			`DELETE FROM tag_suggestions
			 WHERE status = 'pending' AND bookmark_id IN (`+placeholders(len(batch))+`)`,
			argsFor(batch)...)
		if err != nil {
			return 0, fmt.Errorf("sqlite: drop pending suggestions: %w", err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}
		total += int(affected)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("sqlite: commit drop suggestions: %w", err)
	}
	return total, nil
}

// PendingCount returns how many suggestions are waiting.
func (r *SuggestionRepository) PendingCount(ctx context.Context) (int, error) {
	var count int
	if err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM tag_suggestions WHERE status = 'pending'`).Scan(&count); err != nil {
		return 0, fmt.Errorf("sqlite: count pending suggestions: %w", err)
	}
	return count, nil
}
