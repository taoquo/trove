package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/taoquo/trove/apps/macos/internal/domain"
	"github.com/taoquo/trove/apps/macos/internal/ports"
)

// TagRepository stores tags and the tags on a bookmark.
type TagRepository struct {
	db *DB
}

// NewTagRepository builds a repository over an open database.
func NewTagRepository(db *DB) *TagRepository { return &TagRepository{db: db} }

var _ ports.TagRepository = (*TagRepository)(nil)

const tagColumns = `id, name, canonical_key, aliases_json, parent_id, created_at, updated_at`

// scanTag reads one tag row.
func scanTag(scanner interface{ Scan(...any) error }) (domain.Tag, error) {
	var (
		tag        domain.Tag
		aliasesRaw sql.NullString
		parentID   sql.NullString
		createdAt  string
		updatedAt  string
	)
	err := scanner.Scan(&tag.ID, &tag.Name, &tag.CanonicalKey, &aliasesRaw, &parentID, &createdAt, &updatedAt)
	if err != nil {
		return domain.Tag{}, err
	}
	tag.Aliases, _ = decodeStrings(aliasesRaw)
	tag.ParentID = parentID.String
	tag.CreatedAt = createdAt
	tag.UpdatedAt = updatedAt
	return tag, nil
}

// parentValue renders an empty parent as SQL NULL.
//
// An empty string would violate the self-referencing foreign key, so "no parent"
// has to be NULL rather than "".
func parentValue(parentID string) any {
	if strings.TrimSpace(parentID) == "" {
		return nil
	}
	return parentID
}

// List returns every tag, ordered by name.
func (r *TagRepository) List(ctx context.Context) ([]domain.Tag, error) {
	// The rows come back in name order so the result is stable, but the order is then
	// applied in Go: the extension sorts this list through Intl.Collator with numeric
	// ordering and base sensitivity, and SQL's ORDER BY is a byte comparison, which
	// differs on case ("Zebra" before "apple") and on non-ASCII names.
	rows, err := r.db.QueryContext(ctx, `SELECT `+tagColumns+` FROM tags ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list tags: %w", err)
	}
	defer rows.Close()

	var tags []domain.Tag
	for rows.Next() {
		tag, err := scanTag(rows)
		if err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.SliceStable(tags, func(i, j int) bool {
		return domain.CompareNaturalNames(tags[i].Name, tags[j].Name) < 0
	})
	return tags, nil
}

// Get returns one tag.
func (r *TagRepository) Get(ctx context.Context, id string) (domain.Tag, bool, error) {
	tag, err := scanTag(r.db.QueryRowContext(ctx, `SELECT `+tagColumns+` FROM tags WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Tag{}, false, nil
	}
	if err != nil {
		return domain.Tag{}, false, fmt.Errorf("sqlite: get tag %s: %w", id, err)
	}
	return tag, true, nil
}

// FindByName resolves a tag by its canonical key or by any alias.
//
// The canonical key alone is not enough: a tag that absorbed another name records
// that name as an alias, and resolving only by key would mint a near-duplicate of
// the tag that absorbed it.
func (r *TagRepository) FindByName(ctx context.Context, name string) (domain.Tag, bool, error) {
	key := domain.CanonicalTagKey(name)
	if key == "" {
		return domain.Tag{}, false, nil
	}

	query := `SELECT ` + tagColumns + ` FROM tags
	          WHERE canonical_key = ?
	             OR EXISTS (SELECT 1 FROM json_each(tags.aliases_json)
	                        WHERE xbm_fold(json_each.value) = xbm_fold(?))
	          ORDER BY CASE WHEN canonical_key = ? THEN 0 ELSE 1 END
	          LIMIT 1`

	tag, err := scanTag(r.db.QueryRowContext(ctx, query, key, strings.TrimSpace(name), key))
	if errors.Is(err, sql.ErrNoRows) {
		return r.findByAliasKey(ctx, key)
	}
	if err != nil {
		return domain.Tag{}, false, fmt.Errorf("sqlite: find tag %q: %w", name, err)
	}
	return tag, true, nil
}

// findByAliasKey compares each alias by its canonical key.
//
// SQL cannot do it: canonicalisation removes separators and folds the case the
// extension's way, and json_each only exposes the raw strings. Comparing
// lower(alias) with lower(name) missed "Golang" against "go-lang", so accepting
// that suggestion minted a second tag whose canonical key already existed.
func (r *TagRepository) findByAliasKey(ctx context.Context, key string) (domain.Tag, bool, error) {
	tags, err := r.List(ctx)
	if err != nil {
		return domain.Tag{}, false, err
	}
	for _, tag := range tags {
		for _, alias := range tag.Aliases {
			if domain.CanonicalTagKey(alias) == key {
				return tag, true, nil
			}
		}
	}
	return domain.Tag{}, false, nil
}

// Create inserts a tag.
func (r *TagRepository) Create(ctx context.Context, tag domain.Tag) error {
	aliasesJSON, err := encodeStrings(tag.Aliases)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx,
		`INSERT INTO tags (`+tagColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		tag.ID, tag.Name, tag.CanonicalKey, aliasesJSON, parentValue(tag.ParentID),
		tag.CreatedAt, tag.UpdatedAt)
	if err != nil {
		return fmt.Errorf("sqlite: create tag %s: %w", tag.ID, err)
	}
	return nil
}

// Update replaces a tag's mutable fields.
func (r *TagRepository) Update(ctx context.Context, tag domain.Tag) error {
	aliasesJSON, err := encodeStrings(tag.Aliases)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx,
		`UPDATE tags SET name = ?, canonical_key = ?, aliases_json = ?, parent_id = ?, updated_at = ?
		 WHERE id = ?`,
		tag.Name, tag.CanonicalKey, aliasesJSON, parentValue(tag.ParentID), tag.UpdatedAt, tag.ID)
	if err != nil {
		return fmt.Errorf("sqlite: update tag %s: %w", tag.ID, err)
	}
	return nil
}

// Delete removes a tag and every relation to it.
func (r *TagRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM tags WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("sqlite: delete tag %s: %w", id, err)
	}
	return nil
}

// DeleteMany removes several tags and returns how many existed.
func (r *TagRepository) DeleteMany(ctx context.Context, ids []string) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}

	total := 0
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("sqlite: begin delete tags: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op once committed

	for _, batch := range chunkStrings(ids, 500) {
		result, err := tx.ExecContext(ctx,
			`DELETE FROM tags WHERE id IN (`+placeholders(len(batch))+`)`, argsFor(batch)...)
		if err != nil {
			return 0, fmt.Errorf("sqlite: delete tags: %w", err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}
		total += int(affected)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("sqlite: commit delete tags: %w", err)
	}
	return total, nil
}

// Summaries returns each tag with its bookmark and pending-suggestion counts.
func (r *TagRepository) Summaries(ctx context.Context) ([]domain.TagSummary, error) {
	// Pending suggestions are matched by canonical key, because a suggestion holds
	// a name rather than a tag id: the tag may not exist yet.
	query := `
		SELECT ` + tagColumns + `,
		  (SELECT COUNT(*) FROM bookmark_tags bt WHERE bt.tag_id = tags.id),
		  (SELECT COUNT(*) FROM tag_suggestions ts
		     WHERE ts.status = 'pending'
		       AND xbm_fold(trim(ts.tag_name)) = xbm_fold(trim(tags.name)))
		FROM tags
		ORDER BY name ASC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("sqlite: tag summaries: %w", err)
	}
	defer rows.Close()

	var summaries []domain.TagSummary
	for rows.Next() {
		var (
			summary    domain.TagSummary
			aliasesRaw sql.NullString
			parentID   sql.NullString
			createdAt  string
			updatedAt  string
		)
		if err := rows.Scan(&summary.ID, &summary.Name, &summary.CanonicalKey, &aliasesRaw,
			&parentID, &createdAt, &updatedAt, &summary.BookmarkCount, &summary.PendingCount); err != nil {
			return nil, fmt.Errorf("sqlite: scan tag summary: %w", err)
		}
		summary.Aliases, _ = decodeStrings(aliasesRaw)
		summary.ParentID = parentID.String
		summary.CreatedAt = createdAt
		summary.UpdatedAt = updatedAt
		summaries = append(summaries, summary)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// The same order List applies. Both queries select tags by name, and only one of them
	// re-sorted in Go -- so the summary list and the tag list disagreed about the order of
	// the same names, which is the round-227 shape: two comparators for one question.
	sort.SliceStable(summaries, func(i, j int) bool {
		return domain.CompareNaturalNames(summaries[i].Name, summaries[j].Name) < 0
	})
	return summaries, nil
}

// AllRelations returns every bookmark-to-tag relation.
func (r *TagRepository) AllRelations(ctx context.Context) ([]domain.BookmarkTag, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT bookmark_id, tag_id, created_at, source FROM bookmark_tags`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list relations: %w", err)
	}
	defer rows.Close()

	var relations []domain.BookmarkTag
	for rows.Next() {
		var (
			relation domain.BookmarkTag
			created  string
		)
		if err := rows.Scan(&relation.BookmarkID, &relation.TagID, &created, &relation.Source); err != nil {
			return nil, fmt.Errorf("sqlite: scan relation: %w", err)
		}
		relation.CreatedAt = domain.MustParseTimestamp(created)
		relations = append(relations, relation)
	}
	return relations, rows.Err()
}

// Attach adds a relation, leaving an existing one untouched.
//
// DO NOTHING rather than DO UPDATE is the whole point: a manual tag must not be
// relabelled as automatic by a later automatic pass, because the source column is
// what a prune filters on.
func (r *TagRepository) Attach(ctx context.Context, relation domain.BookmarkTag) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO bookmark_tags (bookmark_id, tag_id, created_at, source) VALUES (?, ?, ?, ?)
		 ON CONFLICT(bookmark_id, tag_id) DO NOTHING`,
		relation.BookmarkID, relation.TagID, domain.FormatTimestamp(relation.CreatedAt), relation.Source)
	if err != nil {
		return fmt.Errorf("sqlite: attach tag %s to %s: %w", relation.TagID, relation.BookmarkID, err)
	}
	return nil
}

// AttachMany adds several relations in one transaction.
func (r *TagRepository) AttachMany(ctx context.Context, relations []domain.BookmarkTag) error {
	if len(relations) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlite: begin attach tags: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op once committed

	statement, err := tx.PrepareContext(ctx,
		`INSERT INTO bookmark_tags (bookmark_id, tag_id, created_at, source) VALUES (?, ?, ?, ?)
		 ON CONFLICT(bookmark_id, tag_id) DO NOTHING`)
	if err != nil {
		return fmt.Errorf("sqlite: prepare attach tags: %w", err)
	}
	defer statement.Close()

	for _, relation := range relations {
		if _, err := statement.ExecContext(ctx, relation.BookmarkID, relation.TagID,
			domain.FormatTimestamp(relation.CreatedAt), relation.Source); err != nil {
			return fmt.Errorf("sqlite: attach tag %s to %s: %w", relation.TagID, relation.BookmarkID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("sqlite: commit attach tags: %w", err)
	}
	return nil
}

// Detach removes one relation.
func (r *TagRepository) Detach(ctx context.Context, bookmarkID, tagID string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM bookmark_tags WHERE bookmark_id = ? AND tag_id = ?`, bookmarkID, tagID)
	if err != nil {
		return fmt.Errorf("sqlite: detach tag %s from %s: %w", tagID, bookmarkID, err)
	}
	return nil
}

// DetachMany removes several relations.
func (r *TagRepository) DetachMany(ctx context.Context, pairs []ports.BookmarkTagPair) (int, error) {
	if len(pairs) == 0 {
		return 0, nil
	}

	total := 0
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("sqlite: begin detach tags: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op once committed

	statement, err := tx.PrepareContext(ctx,
		`DELETE FROM bookmark_tags WHERE bookmark_id = ? AND tag_id = ?`)
	if err != nil {
		return 0, fmt.Errorf("sqlite: prepare detach tags: %w", err)
	}
	defer statement.Close()

	for _, pair := range pairs {
		result, err := statement.ExecContext(ctx, pair.BookmarkID, pair.TagID)
		if err != nil {
			return 0, fmt.Errorf("sqlite: detach tag %s from %s: %w", pair.TagID, pair.BookmarkID, err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}
		total += int(affected)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("sqlite: commit detach tags: %w", err)
	}
	return total, nil
}

// DetachAutomatic removes only relations an automatic pass created.
//
// Filtering on the source is what makes a prune safe: a tag the user applied by
// hand is never removed, however the prune was triggered.
func (r *TagRepository) DetachAutomatic(ctx context.Context, bookmarkIDs []string) (int, error) {
	sources := domain.AutomaticSources()
	if len(sources) == 0 {
		return 0, nil
	}

	sourceArgs := argsFor(sources)
	total := 0

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("sqlite: begin detach automatic: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op once committed

	if len(bookmarkIDs) == 0 {
		result, err := tx.ExecContext(ctx,
			`DELETE FROM bookmark_tags WHERE source IN (`+placeholders(len(sources))+`)`, sourceArgs...)
		if err != nil {
			return 0, fmt.Errorf("sqlite: detach automatic: %w", err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}
		total = int(affected)
	} else {
		for _, batch := range chunkStrings(bookmarkIDs, 400) {
			args := append(argsFor(batch), sourceArgs...)
			result, err := tx.ExecContext(ctx,
				`DELETE FROM bookmark_tags
				 WHERE bookmark_id IN (`+placeholders(len(batch))+`)
				   AND source IN (`+placeholders(len(sources))+`)`, args...)
			if err != nil {
				return 0, fmt.Errorf("sqlite: detach automatic: %w", err)
			}
			affected, err := result.RowsAffected()
			if err != nil {
				return 0, err
			}
			total += int(affected)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("sqlite: commit detach automatic: %w", err)
	}
	return total, nil
}

// readTagForMerge reads the surviving tag's name and aliases.
func readTagForMerge(ctx context.Context, tx *sql.Tx, tagID string) (string, []string, error) {
	var (
		name       string
		aliasesRaw sql.NullString
	)
	err := tx.QueryRowContext(ctx, `SELECT name, aliases_json FROM tags WHERE id = ?`, tagID).
		Scan(&name, &aliasesRaw)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil, nil
	}
	if err != nil {
		return "", nil, fmt.Errorf("sqlite: read the surviving tag: %w", err)
	}
	aliases, _ := decodeStrings(aliasesRaw)
	return name, aliases, nil
}

// dedupeAliases removes duplicates and blanks, keeping the first spelling.
func dedupeAliases(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		out = append(out, trimmed)
	}
	return out
}

// DeleteAll removes every tag and returns how many went.
//
// It exists for the reset action, whose confirmation promises that local tags are
// cleared. Without it the promise was false: the cascade emptied bookmarks and left the
// tag library standing.
func (r *TagRepository) DeleteAll(ctx context.Context) (int, error) {
	result, err := r.db.ExecContext(ctx, `DELETE FROM tags`)
	if err != nil {
		return 0, fmt.Errorf("sqlite: delete all tags: %w", err)
	}
	removed, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("sqlite: delete all tags: %w", err)
	}
	return int(removed), nil
}
