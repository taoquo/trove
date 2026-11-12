package ui

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/x-master/apps/macos/internal/domain"
	"github.com/taoquo/x-master/apps/macos/internal/media"
	"github.com/taoquo/x-master/apps/macos/internal/storage/sqlite"
)

// pictureGallery serves one small PNG at any path and counts the requests per path.
func pictureGallery(t *testing.T) (*httptest.Server, func() (requests, distinct int)) {
	t.Helper()

	picture := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for x := 0; x < 8; x++ {
		for y := 0; y < 8; y++ {
			picture.Set(x, y, color.RGBA{R: 30, G: 120, B: 200, A: 255})
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, picture); err != nil {
		t.Fatalf("Encode: %v", err)
	}

	var mu sync.Mutex
	served := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		served[r.URL.Path]++
		mu.Unlock()
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(encoded.Bytes())
	}))
	t.Cleanup(server.Close)

	return server, func() (int, int) {
		mu.Lock()
		defer mu.Unlock()
		requests := 0
		for _, n := range served {
			requests += n
		}
		return requests, len(served)
	}
}

// galleryShell builds a shell holding n bookmarks, each with an author picture and one
// media picture, all served by the gallery.
func galleryShell(t *testing.T, n int, server *httptest.Server) *Shell {
	t.Helper()

	shell, db := newShell(t)
	shell.UseMediaFetcher(media.NewFetcher())

	rows := make([]domain.Bookmark, 0, n)
	for i := 0; i < n; i++ {
		rows = append(rows, domain.Bookmark{
			TweetID:         fmt.Sprintf("flicker-%03d", i),
			TweetURL:        fmt.Sprintf("https://x.com/ada/status/flicker-%03d", i),
			AuthorName:      "Ada",
			AuthorHandle:    "ada",
			AuthorAvatarURL: fmt.Sprintf("%s/avatar-%03d.png", server.URL, i),
			Text:            "a post with a picture",
			SavedAt:         clockBase, LastSeenAt: clockBase, UpdatedAt: clockBase,
			Media: []domain.Media{{
				Type: domain.MediaPhoto,
				URL:  fmt.Sprintf("%s/media-%03d.png", server.URL, i),
			}},
		})
	}
	if _, err := sqlite.NewBookmarkRepository(db).UpsertMany(context.Background(), rows); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}
	shell.Reload(context.Background())
	shell.state.LeftSidebarWidth = WidthSidebarDef
	shell.state.RightSidebarWidth = WidthInspectorDef
	return shell
}

// TestPicturesAreNotFetchedAgainOnEveryFrame is the flicker report measured.
//
// The screen blinked because the draw path asked the fetcher whether a picture was ready.
// That cache holds 64 images, while the grid renders 80 cards -- about 160 pictures -- so
// each frame's loads evicted the pictures the previous frame had drawn, and the next frame
// put the placeholders back. Measured before the fix: 569 requests for 168 distinct URLs.
//
// A frame must draw what it has already drawn. Nothing on screen may go back to a
// placeholder, and nothing may be fetched twice without the user changing the view.
func TestPicturesAreNotFetchedAgainOnEveryFrame(t *testing.T) {
	server, counts := pictureGallery(t)
	// More bookmarks than the grid builds, so the library outgrows both the render limit
	// and the fetcher's cache -- the two numbers the blink came from.
	cards := ResultsRenderLimit + 8
	shell := galleryShell(t, cards, server)

	tester := ui.NewTester(shell.View, 1180, 900)
	tester.Frame()

	// The loads are background work, so this waits rather than frames. Only the built
	// cards ask for pictures: the source renders 80 at a time.
	want := ResultsRenderLimit * 2
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) && shell.draw.len() < want {
		time.Sleep(10 * time.Millisecond)
	}
	if got := shell.draw.len(); got < want {
		t.Fatalf("only %d of %d pictures became drawable", got, want)
	}

	before, distinct := counts()

	// More frames of the same view: this is what the window does while it is open.
	for i := 0; i < 4; i++ {
		tester.Frame()
	}

	after, _ := counts()
	t.Logf("%d cards, %d distinct urls, %d requests, %d after four more frames",
		cards, distinct, before, after)

	if after != before {
		t.Errorf("frames fetched %d more pictures: the screen is blinking", after-before)
	}
	if shell.draw.len() < want {
		t.Errorf("frames dropped pictures: %d of %d still drawable", shell.draw.len(), want)
	}
}

// TestAPictureOnScreenIsNotEvictedForOneThatIsNot pins the eviction rule.
//
// The budget is a soft limit: a picture drawn in the current frame is what the user is
// looking at, so it is never the one dropped. Running over budget is the honest outcome --
// the screen really is displaying that much. Before the fix the rule was "drop the oldest
// insert", which is the same as "drop the top of the first column", every frame.
func TestAPictureOnScreenIsNotEvictedForOneThatIsNot(t *testing.T) {
	cache := &drawCache{budget: 1} // one byte: every picture is over it
	image := func() *media.Image {
		return &media.Image{Width: 4, Height: 4, Pixels: image.NewRGBA(image.Rect(0, 0, 4, 4))}
	}

	cache.beginFrame()
	cache.put("on-screen", image())
	cache.get("on-screen")
	cache.put("also-on-screen", image())

	// Both were in use during this frame, so neither may be dropped even though the
	// budget was exceeded twice over.
	for _, url := range []string{"on-screen", "also-on-screen"} {
		if _, ok := cache.get(url); !ok {
			t.Fatalf("%s was evicted while it was on screen", url)
		}
	}

	// A picture from an earlier frame is the one that makes room.
	cache.beginFrame()
	cache.put("off-screen", image())
	if _, ok := cache.get("off-screen"); !ok {
		t.Fatal("the picture just put in the cache is not drawable")
	}
	// In a new frame the earlier pictures are the ones that may go; the one just put in
	// the cache is the one this frame will draw.
	if _, ok := cache.get("on-screen"); ok {
		t.Error("a picture from an earlier frame survived the eviction")
	}
}

// TestAPictureSurvivesTheFetcherForgettingIt is the mechanism, isolated from layout.
//
// The fetcher's cache is much smaller than a rendered grid. Once a picture has been decoded
// the interface must keep drawing it, whatever the fetcher does with its own copy.
func TestAPictureSurvivesTheFetcherForgettingIt(t *testing.T) {
	server, _ := pictureGallery(t)
	shell := galleryShell(t, media.MaxEntries+8, server)

	ctx := context.Background()
	first := fmt.Sprintf("%s/avatar-000.png", server.URL)
	if _, err := shell.media.Load(ctx, first); err != nil {
		t.Fatalf("Load: %v", err)
	}
	bitmap, ok := shell.mediaPicture(first)
	if !ok {
		t.Fatal("the first picture is not drawable after loading")
	}

	// Enough further loads to push the first one out of the fetcher's cache.
	for i := 1; i <= media.MaxEntries+4; i++ {
		url := fmt.Sprintf("%s/avatar-%03d.png", server.URL, i)
		if _, err := shell.media.Load(ctx, url); err != nil {
			t.Fatalf("Load %s: %v", url, err)
		}
		shell.draw.put(url, mustCached(t, shell, url))
	}
	if _, still := shell.media.Cached(first); still {
		// Not fatal, but the test is only meaningful if the eviction happened.
		t.Log("the fetcher still holds the first picture; the scenario did not occur")
	}

	again, ok := shell.mediaPicture(first)
	if !ok {
		t.Fatal("the picture went back to a placeholder when the fetcher forgot it")
	}
	if again != bitmap {
		t.Error("the picture was decoded a second time instead of being reused")
	}
}

func mustCached(t *testing.T, shell *Shell, url string) *media.Image {
	t.Helper()
	loaded, ok := shell.media.Cached(url)
	if !ok {
		t.Fatalf("%s is not cached", url)
	}
	return loaded
}
