package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/taoquo/trove/internal/domain"
)

// The two states the reader owns: opening a bookmark marks it read, and putting it away in
// the archive takes it out of every list. Both are local, both survive a sync, and both are
// what the unread and archive conditions read.

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

func TestMarkReadOnlyWritesTheFirstOpen(t *testing.T) {
	db := openTestDB(t)
	repo := NewBookmarkRepository(db)
	ctx := context.Background()

	rows := []domain.Bookmark{
		bookmarkFixture("one", clockBase),
		bookmarkFixture("two", clockBase),
	}
	if _, err := repo.UpsertMany(ctx, rows); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}

	marked, err := repo.MarkRead(ctx, []string{"one", "two"}, clockBase.Add(time.Minute))
	if err != nil {
		t.Fatalf("MarkRead: %v", err)
	}
	if marked != 2 {
		t.Errorf("MarkRead marked %d rows, want 2", marked)
	}

	// Opening the detail again is not a write, and the stored time stays the first open.
	marked, err = repo.MarkRead(ctx, []string{"one"}, clockBase.Add(time.Hour))
	if err != nil {
		t.Fatalf("MarkRead(again): %v", err)
	}
	if marked != 0 {
		t.Errorf("re-opening marked %d more rows, want 0", marked)
	}

	stored, found, err := repo.Get(ctx, "one")
	if err != nil || !found {
		t.Fatalf("Get(one) = %v, %v", found, err)
	}
	if want := clockBase.Add(time.Minute); !stored.ReadAt.Equal(want) {
		t.Errorf("ReadAt = %v, want the first open %v", stored.ReadAt, want)
	}

	if count, err := repo.Count(ctx, domain.LibraryFilter{OnlyUnread: true}); err != nil {
		t.Fatalf("Count(unread): %v", err)
	} else if count != 0 {
		t.Errorf("the unread condition matched %d rows, want 0", count)
	}
}

func TestUnreadCountsAmongUnarchivedRowsOnly(t *testing.T) {
	db := openTestDB(t)
	repo := NewBookmarkRepository(db)
	ctx := context.Background()

	rows := []domain.Bookmark{
		bookmarkFixture("unread", clockBase),
		bookmarkFixture("read", clockBase),
		bookmarkFixture("unread-but-archived", clockBase),
	}
	if _, err := repo.UpsertMany(ctx, rows); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}
	if _, err := repo.MarkRead(ctx, []string{"read", "unread-but-archived"}, clockBase); err != nil {
		t.Fatalf("MarkRead: %v", err)
	}
	if err := repo.SetArchived(ctx, "unread-but-archived", clockBase); err != nil {
		t.Fatalf("SetArchived: %v", err)
	}

	// The two conditions stack: unread is a condition inside the scope the archive decides.
	rows2, err := repo.Query(ctx, domain.LibraryQuery{
		Filter: domain.LibraryFilter{OnlyUnread: true},
	})
	if err != nil {
		t.Fatalf("Query(unread): %v", err)
	}
	if got := idsOf(rows2); len(got) != 1 || got[0] != "unread" {
		t.Fatalf("unread matched %v, want [unread]", got)
	}

	// The archive's own view asks its own question, and an archived unread bookmark is
	// still in it.
	if count, err := repo.Count(ctx, domain.LibraryFilter{Archived: domain.ArchivedOnly}); err != nil {
		t.Fatalf("Count(archived): %v", err)
	} else if count != 1 {
		t.Errorf("the archive matched %d rows, want 1", count)
	}
}

// TestUpsertKeepsTheReadersState pins the rule that makes the two states safe: a sync
// rewrites every payload field, and must never clear them.
func TestUpsertKeepsTheReadersState(t *testing.T) {
	db := openTestDB(t)
	repo := NewBookmarkRepository(db)
	ctx := context.Background()

	original := bookmarkFixture("kept", clockBase)
	original.ReadAt = clockBase.Add(time.Minute)
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
	if stored.ReadAt.IsZero() {
		t.Error("the sync cleared read_at")
	}
	if stored.ArchivedAt.IsZero() {
		t.Error("the sync pulled the bookmark out of the archive")
	}
}

// TestRestoreIntoAnEmptyLibraryBringsTheStatesBack is the other half of the upsert rule:
// written on insert, so an import into a fresh library carries them.
func TestRestoreIntoAnEmptyLibraryBringsTheStatesBack(t *testing.T) {
	db := openTestDB(t)
	repo := NewBookmarkRepository(db)

	restored := bookmarkFixture("from-a-backup", clockBase)
	restored.ReadAt = clockBase.Add(time.Minute)
	restored.ArchivedAt = clockBase.Add(2 * time.Minute)
	if _, err := repo.UpsertMany(context.Background(), []domain.Bookmark{restored}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}

	stored, found, err := repo.Get(context.Background(), "from-a-backup")
	if err != nil || !found {
		t.Fatalf("Get = %v, %v", found, err)
	}
	if !stored.ReadAt.Equal(restored.ReadAt) || !stored.ArchivedAt.Equal(restored.ArchivedAt) {
		t.Errorf("restored state = read %v archived %v, want %v and %v",
			stored.ReadAt, stored.ArchivedAt, restored.ReadAt, restored.ArchivedAt)
	}
}

func idsOf(bookmarks []domain.Bookmark) []string {
	ids := make([]string, 0, len(bookmarks))
	for _, bookmark := range bookmarks {
		ids = append(ids, bookmark.TweetID)
	}
	return ids
}
