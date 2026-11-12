package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/taoquo/trove/apps/macos/internal/domain"
)

// TestBookmarkMediaSurvivesTheRoundTrip covers the columns no test ever filled.
//
// Mutation testing found it: making encodeMedia return "[]" for a non-empty list left
// the suite green, because no test ever stored a bookmark with media. The detail view
// renders that media, so losing it is losing a whole section of the interface -- and it
// would have looked like a rendering bug rather than a storage one.
func TestBookmarkMediaSurvivesTheRoundTrip(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	repository := NewBookmarkRepository(db)

	poster := "https://example.test/poster.jpg"
	saved := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

	bookmark := domain.Bookmark{
		TweetID: "media-1", TweetURL: "https://x.com/ada/status/media-1",
		AuthorName: "ada", AuthorHandle: "ada", Text: "a post with media",
		SavedAt: saved, LastSeenAt: saved, UpdatedAt: saved,
		Hashtags: []string{"golang", "sqlite"},
		Media: []domain.Media{
			{Type: "photo", URL: "https://example.test/a.jpg", AltText: "a diagram"},
			{Type: "video", URL: "https://example.test/b.mp4", PosterURL: &poster},
			{Type: "animated_gif", URL: "https://example.test/c.gif"},
		},
		Metrics: &domain.Metrics{Likes: intPointer(12), Retweets: intPointer(3)},
	}

	if _, err := repository.UpsertMany(ctx, []domain.Bookmark{bookmark}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}

	loaded, found, err := repository.Get(ctx, "media-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !found {
		t.Fatal("the bookmark was not stored")
	}

	// Media, field by field: a type, a URL, an alt text and an optional poster.
	if len(loaded.Media) != 3 {
		t.Fatalf("%d media items, want 3 (raw %+v)", len(loaded.Media), loaded.Media)
	}
	if loaded.Media[0].Type != "photo" || loaded.Media[0].URL != "https://example.test/a.jpg" {
		t.Errorf("media[0] = %+v", loaded.Media[0])
	}
	if loaded.Media[0].AltText != "a diagram" {
		t.Errorf("media[0].AltText = %q, want the alt text", loaded.Media[0].AltText)
	}
	if loaded.Media[1].PosterURL == nil || *loaded.Media[1].PosterURL != poster {
		t.Errorf("media[1].PosterURL = %v, want the poster URL", loaded.Media[1].PosterURL)
	}
	if loaded.Media[2].Type != "animated_gif" {
		t.Errorf("media[2].Type = %q", loaded.Media[2].Type)
	}

	// The other JSON columns, which share the same encode/decode path.
	if len(loaded.Hashtags) != 2 || loaded.Hashtags[0] != "golang" {
		t.Errorf("hashtags = %v, want the two stored", loaded.Hashtags)
	}
	if loaded.Metrics == nil || loaded.Metrics.Likes == nil || *loaded.Metrics.Likes != 12 {
		t.Errorf("metrics = %+v, want likes 12", loaded.Metrics)
	}
}

// TestBookmarkWithoutMediaStoresAnEmptyArray keeps the empty case from becoming NULL:
// the two are read back differently, and a NULL would fail to decode.
func TestBookmarkWithoutMediaStoresAnEmptyArray(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	repository := NewBookmarkRepository(db)

	saved := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	if _, err := repository.UpsertMany(ctx, []domain.Bookmark{{
		TweetID: "plain-1", TweetURL: "https://x.com/ada/status/plain-1",
		AuthorName: "ada", AuthorHandle: "ada", Text: "no media",
		SavedAt: saved, LastSeenAt: saved, UpdatedAt: saved,
	}}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}

	loaded, found, err := repository.Get(ctx, "plain-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !found {
		t.Fatal("the bookmark was not stored")
	}
	if len(loaded.Media) != 0 {
		t.Errorf("media = %+v, want none", loaded.Media)
	}
}

func intPointer(value int) *int { return &value }
