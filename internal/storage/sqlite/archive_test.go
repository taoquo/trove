package sqlite

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/taoquo/trove/internal/domain"
)

// The archive as storage sees it: putting a bookmark away takes it out of every list, the
// zero time is the way back, and the state is local -- it survives a sync and travels with
// a restore.

func TestArchiveLeavesTheListAndTheArchiveShowsIt(t *testing.T) {
	db := openTestDB(t)
	repo := NewBookmarkRepository(db)
	ctx := context.Background()

	rows := []domain.Bookmark{
		bookmarkFixture("live", clockBase),
		bookmarkFixture("put-away", clockBase),
	}
	if _, err := repo.UpsertMany(ctx, rows); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}

	if err := repo.SetArchived(ctx, "put-away", clockBase.Add(time.Hour)); err != nil {
		t.Fatalf("SetArchived: %v", err)
	}

	// The default hides archived rows: every view built before the archive existed must
	// keep showing what it showed.
	count, err := repo.Count(ctx, domain.LibraryFilter{})
	if err != nil {
		t.Fatalf("Count(default): %v", err)
	}
	if count != 1 {
		t.Errorf("the default view matched %d rows, want 1", count)
	}

	// The archive is a different set, not a wider one.
	archived, err := repo.Query(ctx, domain.LibraryQuery{
		Filter: domain.LibraryFilter{Archived: domain.ArchivedOnly},
	})
	if err != nil {
		t.Fatalf("Query(archived): %v", err)
	}
	if len(archived) != 1 || archived[0].TweetID != "put-away" {
		t.Fatalf("the archive returned %v, want only put-away", idsOf(archived))
	}

	// The zero time is the way back out.
	if err := repo.SetArchived(ctx, "put-away", time.Time{}); err != nil {
		t.Fatalf("SetArchived(back): %v", err)
	}
	if count, err = repo.Count(ctx, domain.LibraryFilter{}); err != nil {
		t.Fatalf("Count(back): %v", err)
	}
	if count != 2 {
		t.Errorf("after the way back the default view matched %d rows, want 2", count)
	}
}

// TestUpsertKeepsTheArchive pins the rule that makes the state safe: a sync rewrites every
// payload field, and must never pull a bookmark out of the archive.
func TestUpsertKeepsTheArchive(t *testing.T) {
	db := openTestDB(t)
	repo := NewBookmarkRepository(db)
	ctx := context.Background()

	original := bookmarkFixture("kept", clockBase)
	original.ArchivedAt = clockBase.Add(2 * time.Minute)
	if _, err := repo.UpsertMany(ctx, []domain.Bookmark{original}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}

	// The next sync's payload holds the post's fields and nothing about the reader.
	synced := bookmarkFixture("kept", clockBase)
	synced.Text = "about kept, edited on X"
	synced.LastSeenAt = clockBase.Add(time.Hour)
	if _, err := repo.UpsertMany(ctx, []domain.Bookmark{synced}); err != nil {
		t.Fatalf("UpsertMany(sync): %v", err)
	}

	stored, found, err := repo.Get(ctx, "kept")
	if err != nil || !found {
		t.Fatalf("Get = %v, %v", found, err)
	}
	if stored.Text != synced.Text {
		t.Errorf("text = %q, want the payload's %q", stored.Text, synced.Text)
	}
	if stored.ArchivedAt.IsZero() {
		t.Error("the sync pulled the bookmark out of the archive")
	}
}

// TestRestoreIntoAnEmptyLibraryBringsTheArchiveBack is the other half of the upsert rule:
// written on insert, so an import into a fresh library carries it.
func TestRestoreIntoAnEmptyLibraryBringsTheArchiveBack(t *testing.T) {
	db := openTestDB(t)
	repo := NewBookmarkRepository(db)

	restored := bookmarkFixture("from-a-backup", clockBase)
	restored.ArchivedAt = clockBase.Add(2 * time.Minute)
	if _, err := repo.UpsertMany(context.Background(), []domain.Bookmark{restored}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}

	stored, found, err := repo.Get(context.Background(), "from-a-backup")
	if err != nil || !found {
		t.Fatalf("Get = %v, %v", found, err)
	}
	if !stored.ArchivedAt.Equal(restored.ArchivedAt) {
		t.Errorf("restored archive = %v, want %v", stored.ArchivedAt, restored.ArchivedAt)
	}
}

// TestSetArchivedManyMovesTheWholeBatch covers the batch the list writes and the count the
// sidebar reads.
//
// The id set crosses the 500-row chunk boundary on purpose: a single UPDATE with one
// placeholder list would fail there, and a loop that stopped at the first chunk would
// archive part of a selection and report success.
func TestSetArchivedManyMovesTheWholeBatch(t *testing.T) {
	db := openTestDB(t)
	repo := NewBookmarkRepository(db)
	ctx := context.Background()

	const total = 1100
	rows := make([]domain.Bookmark, 0, total)
	ids := make([]string, 0, total)
	for i := 0; i < total; i++ {
		id := fmt.Sprintf("batch-%04d", i)
		rows = append(rows, bookmarkFixture(id, clockBase))
		ids = append(ids, id)
	}
	if _, err := repo.UpsertMany(ctx, rows); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}

	// An id the library does not hold is a selection that changed under the click, not
	// a failed write: the rest of the batch still lands.
	if err := repo.SetArchivedMany(ctx, append(ids, "gone"), clockBase.Add(time.Hour)); err != nil {
		t.Fatalf("SetArchivedMany: %v", err)
	}
	if count, err := repo.Count(ctx, domain.LibraryFilter{}); err != nil {
		t.Fatalf("Count: %v", err)
	} else if count != 0 {
		t.Errorf("the default view matched %d rows, want none", count)
	}
	if count, err := repo.Count(ctx, domain.LibraryFilter{Archived: domain.ArchivedOnly}); err != nil {
		t.Fatalf("Count(archived): %v", err)
	} else if count != total {
		t.Errorf("the archive matched %d rows, want %d", count, total)
	}
	counts, err := repo.Counts(ctx)
	if err != nil {
		t.Fatalf("Counts: %v", err)
	}
	if counts.ArchivedCount != total {
		t.Errorf("the sidebar says %d archived, want %d", counts.ArchivedCount, total)
	}

	// The zero time is the way back for the batch too.
	if err := repo.SetArchivedMany(ctx, ids[:2], time.Time{}); err != nil {
		t.Fatalf("SetArchivedMany(back): %v", err)
	}
	if count, err := repo.Count(ctx, domain.LibraryFilter{}); err != nil {
		t.Fatalf("Count(back): %v", err)
	} else if count != 2 {
		t.Errorf("after the way back the default view matched %d rows, want 2", count)
	}
}

func idsOf(bookmarks []domain.Bookmark) []string {
	ids := make([]string, 0, len(bookmarks))
	for _, bookmark := range bookmarks {
		ids = append(ids, bookmark.TweetID)
	}
	return ids
}
