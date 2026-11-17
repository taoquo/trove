package sqlite

import (
	"strings"
	"testing"
	"time"

	"github.com/taoquo/trove/internal/domain"
)

// explainDetails runs EXPLAIN QUERY PLAN and returns each step's detail line.
func explainDetails(t *testing.T, db *DB, query string, args ...any) []string {
	t.Helper()
	rows, err := db.Query(query, args...)
	if err != nil {
		t.Fatalf("EXPLAIN QUERY PLAN: %v\n%s", err, query)
	}
	defer rows.Close()

	var details []string
	for rows.Next() {
		var id, parent, notUsed int
		var detail string
		if err := rows.Scan(&id, &parent, &notUsed, &detail); err != nil {
			t.Fatalf("scan plan: %v", err)
		}
		details = append(details, detail)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("plan rows: %v", err)
	}
	return details
}

// TestNoSortNeedsATempBTree covers the first page of every sort.
//
// The list asks for 80 rows in one of five orders. Without an index that already
// carries the order, SQLite reads the whole library, sorts it into a temporary
// b-tree and then throws all but 80 rows away -- on a 10k library that is every
// keystroke's reload paying for a full sort. Each sort therefore has an index,
// one per archive state, and this is the assertion that they are the ones the
// planner picks.
func TestNoSortNeedsATempBTree(t *testing.T) {
	db := openTestDB(t)

	for _, sort := range []domain.SortOrder{
		domain.SortTimeline, domain.SortSavedDesc, domain.SortSavedAsc,
		domain.SortCreatedDesc, domain.SortLikesDesc,
	} {
		order, ok := bookmarkOrderSQL[sort]
		if !ok {
			t.Fatalf("no ordering for sort %q", sort)
		}
		for _, scope := range domain.AllArchiveScopes {
			where := " WHERE b.archived_at IS NULL"
			if scope == domain.ArchivedOnly {
				where = " WHERE b.archived_at IS NOT NULL"
			}
			query := `EXPLAIN QUERY PLAN SELECT ` + bookmarkColumns + ` FROM bookmarks AS b` +
				where + ` ORDER BY ` + order + ` LIMIT 80`

			details := explainDetails(t, db, query)
			for _, detail := range details {
				if strings.Contains(strings.ToUpper(detail), "TEMP B-TREE") {
					t.Errorf("%s (%s) sorts through a temp b-tree: %v", sort, scope, details)
					break
				}
			}
		}
	}
}

// TestTheAuthorFilterUsesItsIndex covers the sidebar's author condition.
//
// Choosing an author used to compare trove_fold(author_handle), a function call
// the index on author_handle cannot answer, so the plan fell back to a scan. The
// folded handle has a column of its own now, and the equality reads it.
func TestTheAuthorFilterUsesItsIndex(t *testing.T) {
	db := openTestDB(t)
	filter := domain.LibraryFilter{AuthorHandles: []string{"ada"}}
	where, args := bookmarkFilterSQL(filter, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	query := `EXPLAIN QUERY PLAN SELECT ` + bookmarkColumns + ` FROM bookmarks AS b` + where +
		` ORDER BY b.saved_at DESC, b.tweet_id ASC LIMIT 80`

	details := explainDetails(t, db, query, args...)
	joined := strings.Join(details, "\n")
	if !strings.Contains(joined, "bookmarks_author_folded_idx") {
		t.Errorf("the author condition does not use its index: %v", details)
	}
	for _, detail := range details {
		if strings.Contains(detail, "SCAN b") {
			t.Errorf("the author condition scans the whole table: %v", details)
			break
		}
	}
}
