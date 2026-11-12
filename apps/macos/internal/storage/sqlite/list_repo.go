package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/taoquo/trove/apps/macos/internal/domain"
	"github.com/taoquo/trove/apps/macos/internal/ports"
)

// ListRepository stores lists.
type ListRepository struct {
	db *DB
}

// NewListRepository builds a repository over an open database.
func NewListRepository(db *DB) *ListRepository { return &ListRepository{db: db} }

var _ ports.ListRepository = (*ListRepository)(nil)

const listColumns = `id, name, normalized_name, created_at, updated_at`

// scanList reads one list row.
func scanList(scanner interface{ Scan(...any) error }) (domain.List, error) {
	var (
		list      domain.List
		createdAt string
		updatedAt string
	)
	if err := scanner.Scan(&list.ID, &list.Name, &list.Normalized, &createdAt, &updatedAt); err != nil {
		return domain.List{}, err
	}
	list.CreatedAt = domain.MustParseTimestamp(createdAt)
	list.UpdatedAt = domain.MustParseTimestamp(updatedAt)
	return list, nil
}

// List returns every list, Inbox first and the rest by name.
func (r *ListRepository) List(ctx context.Context) ([]domain.List, error) {
	// The Inbox is pinned to the top: it is the default home for every new
	// bookmark, so it is the one list a user needs to reach without looking.
	//
	// The name order is then re-applied in Go, for the reason tag_repo.go gives: SQL's
	// ORDER BY is a byte comparison, and the names a person sorts are compared the way the
	// extension compares them. The extension does not order its lists at all, so this is an
	// addition -- and an addition that sorted differently from the tags beside it would be a
	// second set of rules for the same question.
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+listColumns+` FROM lists ORDER BY (id = ?) DESC, name ASC`, domain.InboxListID)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list lists: %w", err)
	}
	defer rows.Close()

	var lists []domain.List
	for rows.Next() {
		list, err := scanList(rows)
		if err != nil {
			return nil, err
		}
		lists = append(lists, list)
	}
	sort.SliceStable(lists, func(i, j int) bool {
		// The Inbox keeps its pin whatever the names are.
		if (lists[i].ID == domain.InboxListID) != (lists[j].ID == domain.InboxListID) {
			return lists[i].ID == domain.InboxListID
		}
		return domain.CompareNaturalNames(lists[i].Name, lists[j].Name) < 0
	})
	return lists, rows.Err()
}

// Summaries returns each list with its bookmark count.
func (r *ListRepository) Summaries(ctx context.Context) ([]domain.ListSummary, error) {
	query := `SELECT ` + listColumns + `,
	            (SELECT COUNT(*) FROM bookmark_lists bl WHERE bl.list_id = lists.id)
	          FROM lists
	          ORDER BY (id = ?) DESC, name ASC`

	rows, err := r.db.QueryContext(ctx, query, domain.InboxListID)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list summaries: %w", err)
	}
	defer rows.Close()

	var summaries []domain.ListSummary
	for rows.Next() {
		var (
			summary   domain.ListSummary
			createdAt string
			updatedAt string
		)
		if err := rows.Scan(&summary.ID, &summary.Name, &summary.Normalized,
			&createdAt, &updatedAt, &summary.BookmarkCount); err != nil {
			return nil, fmt.Errorf("sqlite: scan list summary: %w", err)
		}
		summary.CreatedAt = domain.MustParseTimestamp(createdAt)
		summary.UpdatedAt = domain.MustParseTimestamp(updatedAt)
		summaries = append(summaries, summary)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// The same order List applies, for the reason given there: both queries select lists by
	// name, and a second one that skipped the Go sort would disagree with the first about the
	// order of the same names. tag_repo.go had the identical split until round 229.
	sort.SliceStable(summaries, func(i, j int) bool {
		if (summaries[i].ID == domain.InboxListID) != (summaries[j].ID == domain.InboxListID) {
			return summaries[i].ID == domain.InboxListID
		}
		return domain.CompareNaturalNames(summaries[i].Name, summaries[j].Name) < 0
	})
	return summaries, nil
}

// Get returns one list.
func (r *ListRepository) Get(ctx context.Context, id string) (domain.List, bool, error) {
	list, err := scanList(r.db.QueryRowContext(ctx, `SELECT `+listColumns+` FROM lists WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.List{}, false, nil
	}
	if err != nil {
		return domain.List{}, false, fmt.Errorf("sqlite: get list %s: %w", id, err)
	}
	return list, true, nil
}

// FindByNormalizedName resolves a list by its normalized name.
func (r *ListRepository) FindByNormalizedName(ctx context.Context, normalized string) (domain.List, bool, error) {
	list, err := scanList(r.db.QueryRowContext(ctx,
		`SELECT `+listColumns+` FROM lists WHERE normalized_name = ?`, normalized))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.List{}, false, nil
	}
	if err != nil {
		return domain.List{}, false, fmt.Errorf("sqlite: find list %q: %w", normalized, err)
	}
	return list, true, nil
}

// Create inserts a list.
func (r *ListRepository) Create(ctx context.Context, list domain.List) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO lists (`+listColumns+`) VALUES (?, ?, ?, ?, ?)`,
		list.ID, list.Name, list.Normalized,
		domain.FormatTimestamp(list.CreatedAt), domain.FormatTimestamp(list.UpdatedAt))
	if err != nil {
		return fmt.Errorf("sqlite: create list %s: %w", list.ID, err)
	}
	return nil
}

// Update renames a list.
//
// The Inbox is refused here as well as in the domain, because this is the last
// point before the write and a rename would break the promise that the Inbox is
// always recognizable by its fixed id and name.
func (r *ListRepository) Update(ctx context.Context, list domain.List) error {
	if list.ID == domain.InboxListID {
		return &domain.ListError{Reason: "Inbox cannot be renamed"}
	}

	_, err := r.db.ExecContext(ctx,
		`UPDATE lists SET name = ?, normalized_name = ?, updated_at = ? WHERE id = ?`,
		list.Name, list.Normalized, domain.FormatTimestamp(list.UpdatedAt), list.ID)
	if err != nil {
		return fmt.Errorf("sqlite: update list %s: %w", list.ID, err)
	}
	return nil
}

// Delete removes a list.
//
// It refuses while any bookmark still points at the list. That is deliberate: a
// cascade would silently lose which list those bookmarks were in, so the caller
// has to move them first, which is decision D3.
func (r *ListRepository) Delete(ctx context.Context, id string) error {
	if id == domain.InboxListID {
		return &domain.ListError{Reason: "Inbox cannot be deleted"}
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlite: begin delete list: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op once committed

	var remaining int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM bookmark_lists WHERE list_id = ?`, id).Scan(&remaining); err != nil {
		return fmt.Errorf("sqlite: count bookmarks in list: %w", err)
	}
	if remaining > 0 {
		return fmt.Errorf("%w: %d bookmark(s) still in list %s", domain.ErrListNotEmpty, remaining, id)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM lists WHERE id = ?`, id); err != nil {
		return fmt.Errorf("sqlite: delete list %s: %w", id, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("sqlite: commit delete list: %w", err)
	}
	return nil
}

// EnsureInbox creates the Inbox list if it is missing.
func (r *ListRepository) EnsureInbox(ctx context.Context, now time.Time) error {
	return r.db.EnsureInbox(now)
}
