package ui

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/i18n"
	"github.com/taoquo/trove/internal/media"
	"github.com/taoquo/trove/internal/storage/sqlite"
)

// TestMediaChainEndToEnd is the whole path in one test: a URL on a real server, through
// the fetcher, into a bitmap, drawn by the interface.
//
// Every piece had its own test and the chain had none, which is the shape of defect this
// goal keeps producing: each end passes alone and nothing checks they meet. The one
// thing it still cannot check is that the pixels look right, which needs a screen.
func TestMediaChainEndToEnd(t *testing.T) {
	// A real picture on a real server.
	picture := image.NewRGBA(image.Rect(0, 0, 12, 9))
	for x := 0; x < 12; x++ {
		for y := 0; y < 9; y++ {
			picture.Set(x, y, color.RGBA{R: 40, G: 90, B: 160, A: 255})
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, picture); err != nil {
		t.Fatalf("Encode: %v", err)
	}

	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(encoded.Bytes())
	}))
	defer server.Close()

	shell, db := newShell(t)
	shell.UseMediaFetcher(media.NewFetcher())

	// A bookmark pointing at it.
	bookmark := domain.Bookmark{
		TweetID: "chain-1", TweetURL: "https://x.com/ada/status/chain-1",
		AuthorName: "ada", AuthorHandle: "ada", Text: "a post with a picture",
		SavedAt: clockBase, LastSeenAt: clockBase, UpdatedAt: clockBase,
		Media: []domain.Media{{Type: "photo", URL: server.URL + "/picture.png"}},
	}
	if _, err := sqlite.NewBookmarkRepository(db).UpsertMany(context.Background(), []domain.Bookmark{bookmark}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}

	shell.Reload(context.Background())
	shell.state.Selected = "chain-1"
	shell.OpenSelectedDetail()

	// The frame that asks for it must not block, and must not have the picture yet.
	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	if _, ready := shell.mediaPicture(server.URL + "/picture.png"); !ready {
		// Expected on the first frame: the load was started, not awaited.
		t.Log("the first frame started the load rather than waiting for it")
	}

	// Wait for the fetch to land.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, cached := shell.media.Cached(server.URL + "/picture.png"); cached {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, cached := shell.media.Cached(server.URL + "/picture.png"); !cached {
		t.Fatal("the picture never arrived")
	}

	// It is drawable, at the size the server sent.
	bitmap, ready := shell.mediaPicture(server.URL + "/picture.png")
	if !ready || bitmap == nil {
		t.Fatal("a fetched picture must be drawable")
	}

	// And the interface renders with it without panicking, which is what "drawn" means
	// without a screen.
	tester.Frame()
	if requests != 1 {
		t.Errorf("the server was asked %d times, want 1", requests)
	}
	if _, found := tester.Find(DetailMediaTriggerLabel); !found {
		t.Errorf("the post's picture is missing; texts: %q", tester.Texts())
	}

	// The lightbox draws the same picture at its own size.
	shell.OpenLightbox(0)
	tester.Frame()
	if !tester.HasText(shell.t().T(i18n.MediaClose)) {
		t.Errorf("the lightbox is not showing the media; texts: %q", tester.Texts())
	}
	// One item, so no counter: the counter is drawn only when there is more than one item
	// (`hasMultipleMedia ? <footer>`).
	if tester.HasText("1 / 1") {
		t.Error("a one-item preview counts itself")
	}
}

// TestMediaIsFetchedOnceAcrossFrames keeps a redraw from re-requesting.
func TestMediaIsFetchedOnceAcrossFrames(t *testing.T) {
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 5, 5))); err != nil {
		t.Fatalf("Encode: %v", err)
	}

	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_, _ = w.Write(encoded.Bytes())
	}))
	defer server.Close()

	shell, _ := newShell(t)
	shell.UseMediaFetcher(media.NewFetcher())

	// Several frames asking for the same picture before it lands.
	for i := 0; i < 5; i++ {
		shell.mediaPicture(server.URL + "/a.png")
		time.Sleep(5 * time.Millisecond)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, cached := shell.media.Cached(server.URL + "/a.png"); cached {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if requests > 1 {
		t.Errorf("five frames asked the server %d times, want 1", requests)
	}
}
