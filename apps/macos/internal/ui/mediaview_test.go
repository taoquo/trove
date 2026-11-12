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

	"github.com/taoquo/x-master/apps/macos/internal/media"
)

func testPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for x := 0; x < 4; x++ {
		for y := 0; y < 4; y++ {
			img.Set(x, y, color.RGBA{R: 10, G: 20, B: 30, A: 255})
		}
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	return buffer.Bytes()
}

// TestMediaPictureStartsALoadWithoutBlocking covers the draw path's contract: a miss
// starts a load and reports "not yet" rather than fetching inside the frame.
func TestMediaPictureStartsALoadWithoutBlocking(t *testing.T) {
	body := testPNG(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	defer server.Close()

	shell, _ := newShell(t)
	shell.UseMediaFetcher(media.NewFetcher())

	// Not ready, and the call must return promptly rather than waiting for the network.
	started := time.Now()
	bitmap, ready := shell.mediaPicture(server.URL + "/a.png")
	if ready || bitmap != nil {
		t.Error("an uncached picture must report not-ready on the frame it is requested")
	}
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Errorf("the draw path blocked for %s", elapsed)
	}

	// The load is now in flight or done; wait for the cache rather than assuming a
	// speed.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, ok := shell.media.Cached(server.URL + "/a.png"); ok {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if _, cached := shell.media.Cached(server.URL + "/a.png"); !cached {
		t.Fatal("the background load never landed")
	}
	// And now it is drawable.
	bitmap, ready = shell.mediaPicture(server.URL + "/a.png")
	if !ready || bitmap == nil {
		t.Error("a cached picture must be ready to draw")
	}
}

// TestNoFetcherDescribesRatherThanShows keeps an uninstalled loader from panicking.
func TestNoFetcherDescribesRatherThanShows(t *testing.T) {
	shell, _ := newShell(t)

	bitmap, ready := shell.mediaPicture("https://example.test/a.png")
	if ready || bitmap != nil {
		t.Error("without a fetcher there is nothing to draw")
	}
	if _, ready := shell.mediaPicture(""); ready {
		t.Error("an empty url has no picture")
	}
}

// TestMediaSectionRendersWithoutAFetcher covers the detail view with media present and
// no loader: it must describe the media rather than omit the section.
func TestMediaSectionRendersWithoutAFetcher(t *testing.T) {
	shell, db := newShell(t)
	seedMediaBookmark(t, db, "1", "photo")
	shell.Reload(context.Background())
	shell.state.Selected = "1"
	shell.OpenSelectedDetail()

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	if _, found := tester.Find(DetailMediaTriggerLabel); !found {
		t.Errorf("the post's picture is missing; texts: %q", tester.Texts())
	}
	// The section no longer captions the picture with its type: the source's detail media
	// section (BookmarkInspector.tsx:67-104) has a heading and the picture's shell, and names
	// the type nowhere -- the earlier "photo" caption was this build's own addition. What has
	// to survive without a loader is the section and its shell, which the heading asserts.
}
