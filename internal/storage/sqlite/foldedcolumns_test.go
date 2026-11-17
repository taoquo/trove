package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/taoquo/trove/internal/domain"
)

// TestTheFoldedColumnsFollowEveryWrite covers the columns the search reads.
//
// The folded text is kept by triggers rather than by the query, so the one thing
// that can go wrong is a write path that forgets it: an import that inserts rows
// directly, a restore, a sync upserting a changed post. Every write goes through
// the same trigger, and this drives the two that exist -- an insert and an update
// that keeps the same tweet id -- and then asks the search, which is the reader.
func TestTheFoldedColumnsFollowEveryWrite(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	repo := NewBookmarkRepository(db)
	base := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)

	bookmark := domain.Bookmark{
		TweetID:      "1",
		TweetURL:     "https://x.com/Ada/status/1",
		AuthorName:   "Ada Lovelace",
		AuthorHandle: "Ada",
		Text:         "CAFÉ and Kubernetes",
		SavedAt:      base,
		LastSeenAt:   base,
		UpdatedAt:    base,
		SourceKind:   domain.SourceBookmark,
	}
	if _, err := repo.UpsertMany(ctx, []domain.Bookmark{bookmark}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}

	// A search folded in Go has to meet a column folded at write time.
	found, err := repo.Query(ctx, domain.LibraryQuery{
		Filter: domain.LibraryFilter{Query: "café"},
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(found) != 1 {
		t.Fatalf("searching for café found %d bookmarks, want 1", len(found))
	}

	// The same tweet id again is an update, not a second row: the trigger has to
	// follow the new text.
	bookmark.Text = "the new text about RUST"
	if _, err := repo.UpsertMany(ctx, []domain.Bookmark{bookmark}); err != nil {
		t.Fatalf("UpsertMany update: %v", err)
	}

	for _, term := range []string{"new text", "rust"} {
		found, err := repo.Query(ctx, domain.LibraryQuery{
			Filter: domain.LibraryFilter{Query: term},
		})
		if err != nil {
			t.Fatalf("Query %q: %v", term, err)
		}
		if len(found) != 1 {
			t.Errorf("after the update, searching for %q found %d bookmarks, want 1", term, len(found))
		}
	}
	if found, err := repo.Query(ctx, domain.LibraryQuery{
		Filter: domain.LibraryFilter{Query: "café"},
	}); err != nil {
		t.Fatalf("Query café: %v", err)
	} else if len(found) != 0 {
		t.Errorf("the old text is still searchable: %d rows", len(found))
	}

	// The author columns are the same story, and the sidebar's equality compares
	// them: a handle typed in another case still finds its bookmark.
	found, err = repo.Query(ctx, domain.LibraryQuery{
		Filter: domain.LibraryFilter{AuthorHandles: []string{"ADA"}},
	})
	if err != nil {
		t.Fatalf("Query author: %v", err)
	}
	if len(found) != 1 {
		t.Errorf("filtering by ADA found %d bookmarks, want 1", len(found))
	}
}

// TestAFoldedColumnStaysRightForARowWrittenOutsideTheRepository covers the paths
// the trigger exists for.
//
// Imports and restores write rows in bulk through their own statements, and the
// trigger is what keeps those rows searchable without every writer remembering the
// four columns. This writes a row the way such a path would -- plain SQL -- and
// reads the columns back.
func TestAFoldedColumnStaysRightForARowWrittenOutsideTheRepository(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	if _, err := db.ExecContext(ctx, `INSERT INTO bookmarks
		(tweet_id, tweet_url, author_name, author_handle, text, saved_at, updated_at)
		VALUES ('2', 'https://x.com/Grace/status/2', 'Grace Hopper', 'Grace', 'COMPILERS', ?, ?)`,
		domain.FormatTimestamp(time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)),
		domain.FormatTimestamp(time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC))); err != nil {
		t.Fatalf("insert: %v", err)
	}

	var text, handle string
	if err := db.QueryRowContext(ctx,
		`SELECT text_folded, author_handle_folded FROM bookmarks WHERE tweet_id = '2'`,
	).Scan(&text, &handle); err != nil {
		t.Fatalf("read folded columns: %v", err)
	}
	if text != "compilers" || handle != "grace" {
		t.Errorf("folded columns are %q / %q, want \"compilers\" / \"grace\"", text, handle)
	}
}

// TestTagSummariesCountPendingSuggestionsByFoldedName covers the sidebar's count.
//
// The count is matched by name, because a suggestion carries a name rather than a
// tag id: the tag may not exist yet, and two spellings of it are the same tag. The
// join now does that against a folded column, so this pins what the column has to
// mean -- trimmed, folded, and pending only.
func TestTagSummariesCountPendingSuggestionsByFoldedName(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	base := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)

	tags := NewTagRepository(db)
	if err := tags.Create(ctx, domain.Tag{
		ID: "t1", Name: "Go", CanonicalKey: domain.CanonicalTagKey("Go"),
		CreatedAt: domain.FormatTimestamp(base), UpdatedAt: domain.FormatTimestamp(base),
	}); err != nil {
		t.Fatalf("Create tag: %v", err)
	}

	repo := NewBookmarkRepository(db)
	bookmark := domain.Bookmark{
		TweetID: "1", TweetURL: "https://x.com/ada/status/1", AuthorHandle: "ada",
		Text: "a post", SavedAt: base, LastSeenAt: base, UpdatedAt: base,
		SourceKind: domain.SourceBookmark,
	}
	if _, err := repo.UpsertMany(ctx, []domain.Bookmark{bookmark}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}
	if err := tags.AttachMany(ctx, []domain.BookmarkTag{{
		BookmarkID: "1", TagID: "t1", CreatedAt: base, Source: domain.TagSourceManual,
	}}); err != nil {
		t.Fatalf("AttachMany: %v", err)
	}

	suggestions := NewSuggestionRepository(db)
	if _, err := suggestions.InsertMany(ctx, []domain.TagSuggestion{
		{ID: "s1", BookmarkID: "1", TagName: "  go ", Source: domain.SourceHashtag,
			Confidence: 0.9, Status: domain.StatusPending, CreatedAt: base},
		{ID: "s2", BookmarkID: "1", TagName: "GO", Source: domain.SourceHashtag,
			Confidence: 0.9, Status: domain.StatusAccepted, CreatedAt: base},
		{ID: "s3", BookmarkID: "1", TagName: "Rust", Source: domain.SourceHashtag,
			Confidence: 0.9, Status: domain.StatusPending, CreatedAt: base},
	}); err != nil {
		t.Fatalf("InsertMany: %v", err)
	}

	summaries, err := tags.Summaries(ctx)
	if err != nil {
		t.Fatalf("Summaries: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("got %d summaries, want 1", len(summaries))
	}
	if got := summaries[0].BookmarkCount; got != 1 {
		t.Errorf("bookmark count = %d, want 1", got)
	}
	// One pending "go" in another case and with spaces, not the accepted "GO", and
	// not the pending suggestion for a different tag.
	if got := summaries[0].PendingCount; got != 1 {
		t.Errorf("pending count = %d, want 1", got)
	}
}
