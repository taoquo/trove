package ui

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/x-master/apps/macos/internal/domain"
	"github.com/taoquo/x-master/apps/macos/internal/i18n"
	"github.com/taoquo/x-master/apps/macos/internal/storage/sqlite"
)

// seedMediaBookmark stores one bookmark carrying the given media, in order.
func seedMediaBookmark(t *testing.T, db *sqlite.DB, id string, types ...domain.MediaType) {
	t.Helper()

	media := make([]domain.Media, 0, len(types))
	for i, mediaType := range types {
		media = append(media, domain.Media{
			Type: mediaType,
			URL:  "https://example.test/" + id + "-" + string(rune('a'+i)) + ".bin",
		})
	}

	bookmark := domain.Bookmark{
		TweetID: id, TweetURL: "https://x.com/ada/status/" + id,
		AuthorName: "ada", AuthorHandle: "ada", Text: "a post with media",
		SavedAt: clockBase, LastSeenAt: clockBase, UpdatedAt: clockBase,
		Media: media,
	}
	if _, err := sqlite.NewBookmarkRepository(db).UpsertMany(context.Background(), []domain.Bookmark{bookmark}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}
}

// TestLightboxStartsClosed keeps an index from looking open before anything is previewed.
func TestLightboxStartsClosed(t *testing.T) {
	shell, _ := newShell(t)
	if shell.LightboxOpen() {
		t.Fatalf("a fresh shell has the lightbox open at %d", shell.state.LightboxIndex)
	}
}

// TestLightboxNavigationWraps covers the navigation the stylesheet's two controls drive.
//
// It wraps rather than stopping: with two pictures, a "next" that does nothing looks
// like a broken button.
func TestLightboxNavigationWraps(t *testing.T) {
	shell, db := newShell(t)
	seedMediaBookmark(t, db, "1", "photo", "video", "animated_gif")
	shell.Reload(context.Background())
	shell.state.Selected = "1"

	// Opening with no index starts at the first item.
	shell.OpenLightbox(0)
	if !shell.LightboxOpen() {
		t.Fatal("the lightbox must open")
	}
	if got, _ := shell.lightboxIndex(); got != 0 {
		t.Errorf("opened at %d, want 0", got)
	}

	shell.NextMedia()
	if got, _ := shell.lightboxIndex(); got != 1 {
		t.Errorf("next went to %d, want 1", got)
	}
	shell.NextMedia()
	shell.NextMedia() // wraps
	if got, _ := shell.lightboxIndex(); got != 0 {
		t.Errorf("next did not wrap: %d", got)
	}

	shell.PrevMedia() // wraps backwards
	if got, _ := shell.lightboxIndex(); got != 2 {
		t.Errorf("previous did not wrap: %d", got)
	}

	shell.CloseLightbox()
	if shell.LightboxOpen() {
		t.Error("closing must hide the lightbox")
	}
	// Navigation on a closed lightbox must not reopen it.
	shell.NextMedia()
	if shell.LightboxOpen() {
		t.Error("navigation must not open a closed lightbox")
	}
}

// TestLightboxOpenClampsToTheMedia keeps an out-of-range index from pointing past the end.
func TestLightboxOpenClampsToTheMedia(t *testing.T) {
	shell, db := newShell(t)
	seedMediaBookmark(t, db, "1", "photo", "video")
	shell.Reload(context.Background())
	shell.state.Selected = "1"

	shell.OpenLightbox(99)
	if got, _ := shell.lightboxIndex(); got != 1 {
		t.Errorf("a too-large index gave %d, want the last item", got)
	}
	shell.OpenLightbox(-5)
	if got, _ := shell.lightboxIndex(); got != 0 {
		t.Errorf("a negative index gave %d, want the first item", got)
	}
}

// TestLightboxDoesNotOpenWithoutMedia keeps an empty preview off the screen.
func TestLightboxDoesNotOpenWithoutMedia(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "no media here", clockBase)
	shell.Reload(context.Background())
	shell.state.Selected = "1"

	shell.OpenLightbox(0)
	if shell.LightboxOpen() {
		t.Error("a bookmark with no media has nothing to preview")
	}
}

// TestLightboxClampsWhenTheBookmarkChanges covers an index held across a change: a sync
// can replace the row under an open lightbox.
func TestLightboxClampsWhenTheBookmarkChanges(t *testing.T) {
	shell, db := newShell(t)
	seedMediaBookmark(t, db, "1", "photo", "video", "animated_gif")
	seedMediaBookmark(t, db, "2", "photo")
	shell.Reload(context.Background())

	shell.state.Selected = "1"
	shell.OpenLightbox(2)
	if got, _ := shell.lightboxIndex(); got != 2 {
		t.Fatalf("opened at %d", got)
	}

	// The selection moves to a bookmark with one item.
	shell.state.Selected = "2"
	got, ok := shell.lightboxIndex()
	if !ok {
		t.Fatal("the lightbox is still open on the new bookmark")
	}
	if got != 0 {
		t.Errorf("index %d points past a one-item bookmark", got)
	}
}

// TestLightboxEntranceIsItsOwn checks the animation: the lightbox pops and lifts 10px,
// where the detail card scales and does not lift.
func TestLightboxEntranceIsItsOwn(t *testing.T) {
	if EntranceLightbox.OffsetYFrom != 10 {
		t.Errorf("the lightbox lifts %vpx, want the stylesheet's 10", EntranceLightbox.OffsetYFrom)
	}
	if EntranceLightbox.DurationMs != AnimateLightboxContentMs {
		t.Errorf("the lightbox runs %dms, want %d", EntranceLightbox.DurationMs, AnimateLightboxContentMs)
	}
	if EntranceLightbox.Keyframe == EntranceDetailCard.Keyframe {
		t.Error("the lightbox must not reuse the detail card's keyframe")
	}
}

var _ = domain.Media{}

// TestLightboxIsDrawn keeps the overlay from being state nobody renders.
func TestLightboxIsDrawn(t *testing.T) {
	shell, db := newShell(t)
	seedMediaBookmark(t, db, "1", "photo", "video")
	shell.Reload(context.Background())
	shell.state.Selected = "1"
	shell.OpenSelectedDetail()

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	if tester.HasText("1 / 2") {
		t.Error("the lightbox must not be drawn before it is opened")
	}

	shell.OpenLightbox(0)
	tester.Frame()
	if !tester.HasText("1 / 2") {
		t.Errorf("the lightbox is not drawn; texts: %q", tester.Texts())
	}
	// No type or URL text: the source's lightbox shows the picture, the arrows and the
	// counter, and nothing else (BookmarkInspector.tsx:500-580). An earlier version of this
	// overlay printed the media's type and URL because it could not show the picture at
	// all; the fetcher exists now, so that text is gone.
	if tester.HasText("photo") {
		t.Errorf("the lightbox prints the media's type; texts: %q", tester.Texts())
	}
	if !tester.HasText(shell.t().T(i18n.MediaClose)) {
		t.Error("the lightbox must offer a close control")
	}
	if !tester.HasText(shell.t().T(i18n.MediaPrevious)) || !tester.HasText(shell.t().T(i18n.MediaNext)) {
		t.Errorf("two items need both arrows; texts: %q", tester.Texts())
	}

	// Stepping shows the next item.
	shell.NextMedia()
	tester.Frame()
	if !tester.HasText("2 / 2") {
		t.Errorf("stepping did not advance; texts: %q", tester.Texts())
	}
}
