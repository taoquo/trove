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

	"github.com/taoquo/x-master/apps/macos/internal/domain"
	"github.com/taoquo/x-master/apps/macos/internal/i18n"
	"github.com/taoquo/x-master/apps/macos/internal/media"
	"github.com/taoquo/x-master/apps/macos/internal/storage/sqlite"
)

// pictureServer serves one PNG, after `release` is closed, so a frame can be drawn while
// the picture is still in flight.
func pictureServer(t *testing.T, release <-chan struct{}) (*httptest.Server, *int) {
	t.Helper()

	picture := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for x := 0; x < 8; x++ {
		for y := 0; y < 8; y++ {
			picture.Set(x, y, color.RGBA{R: 200, G: 60, B: 60, A: 255})
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, picture); err != nil {
		t.Fatalf("Encode: %v", err)
	}

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if release != nil {
			<-release
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(encoded.Bytes())
	}))
	t.Cleanup(server.Close)
	return server, &requests
}

// TestTheCardShowsTheAuthorsPicture is the user's second report: "有些图片的资源其实没有应用到这个
// icon 上" -- the avatar resource was stored and never drawn.
//
// The source decides this in three lines (BookmarkCard.tsx:51-66): an <img> when there is
// a URL, the initials when there is not. This card always drew the initials.
func TestTheCardShowsTheAuthorsPicture(t *testing.T) {
	release := make(chan struct{})
	server, _ := pictureServer(t, release)
	shell, db := newShell(t)
	ctx := context.Background()
	shell.UseMediaFetcher(media.NewFetcher())

	bookmark := domain.Bookmark{
		TweetID: "avatar-1", TweetURL: "https://x.com/ada/status/avatar-1",
		AuthorName: "Ada Lovelace", AuthorHandle: "ada",
		AuthorAvatarURL: server.URL + "/ada.png",
		Text:            "a post by an author with a picture",
		SavedAt:         clockBase, LastSeenAt: clockBase, UpdatedAt: clockBase,
	}
	if _, err := sqlite.NewBookmarkRepository(db).UpsertMany(ctx, []domain.Bookmark{bookmark}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}
	shell.Reload(ctx)
	shell.state.LeftSidebarWidth = WidthSidebarDef
	shell.state.RightSidebarWidth = WidthInspectorDef

	tester := ui.NewTester(shell.View, 1180, 900)
	tester.Frame()

	// The first frame starts the load and draws the fallback, which is what the source's
	// <img> loading shows until the picture arrives. The server blocks, so this is
	// deterministic rather than a race with a fast local fetch.
	if !tester.HasText("AL") {
		t.Errorf("the frame before the picture is missing the initials; texts: %q", tester.Texts())
	}
	close(release)

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, cached := shell.media.Cached(bookmark.AuthorAvatarURL); cached {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, cached := shell.media.Cached(bookmark.AuthorAvatarURL); !cached {
		t.Fatal("the avatar never loaded")
	}

	tester.Frame()
	if tester.HasText("AL") {
		t.Error("the card still draws the initials over a picture that has loaded")
	}
	rect, found := tester.Find(CardAvatarLabel)
	if !found {
		t.Fatal("the card has no avatar at all")
	}

	// The node existing is not the same as the picture being painted: the test server's
	// pixels are pure red, and the fallback disc is the theme's grey, so sampling the
	// middle of the avatar tells the two apart.
	img := tester.Image()
	scale := float32(img.Rect.Dx()) / 1180
	px := img.RGBAAt(int((rect.X+rect.W/2)*scale), int((rect.Y+rect.H/2)*scale))
	if px.R < 150 || px.G > 120 || px.B > 120 {
		t.Errorf("the avatar centre is %v, want the picture's red", px)
	}
}

// TestACardWithoutAnAvatarKeepsTheInitial is the other branch: a bookmark whose author
// picture is unknown must still show something.
func TestACardWithoutAnAvatarKeepsTheInitial(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()

	bookmark := domain.Bookmark{
		TweetID: "noavatar-1", TweetURL: "https://x.com/grace/status/noavatar-1",
		AuthorName: "Grace Hopper", AuthorHandle: "grace",
		Text:    "a post by an author without a picture",
		SavedAt: clockBase, LastSeenAt: clockBase, UpdatedAt: clockBase,
	}
	if _, err := sqlite.NewBookmarkRepository(db).UpsertMany(ctx, []domain.Bookmark{bookmark}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}
	shell.Reload(ctx)
	shell.state.LeftSidebarWidth = WidthSidebarDef
	shell.state.RightSidebarWidth = WidthInspectorDef

	tester := ui.NewTester(shell.View, 1180, 900)
	tester.Frame()

	if !tester.HasText("GH") {
		t.Errorf("the fallback initials are missing; texts: %q", tester.Texts())
	}
}

// TestTheSidebarAvatarIsOneStableSlot is the sidebar's half of the same rule.
//
// The source has one 24-point span per row (WorkspaceSidebar.tsx:744-750): the picture when
// the author has a URL, the "@" when they do not. The row here built the disc first and then
// drew the picture beside it, so an author with a face also got an empty 24-point box: every
// row with a picture sat a slot to the right of every row without one, and a picture still in
// flight looked like an author who had none.
func TestTheSidebarAvatarIsOneStableSlot(t *testing.T) {
	server, _ := pictureServer(t, nil)
	release := make(chan struct{})
	defer close(release)
	slow, _ := pictureServer(t, release)

	shell, db := newShell(t)
	ctx := context.Background()
	shell.UseMediaFetcher(media.NewFetcher())

	seeded := []domain.Bookmark{
		{
			TweetID: "sidebar-ada", TweetURL: "https://x.com/ada/status/sidebar-ada",
			AuthorName: "Ada Figure", AuthorHandle: "ada",
			AuthorAvatarURL: server.URL + "/ada.png", Text: "a ready picture",
			SavedAt: clockBase, LastSeenAt: clockBase, UpdatedAt: clockBase,
		},
		{
			TweetID: "sidebar-grace", TweetURL: "https://x.com/grace/status/sidebar-grace",
			AuthorName: "Grace Figure", AuthorHandle: "grace",
			AuthorAvatarURL: slow.URL + "/grace.png", Text: "a picture still on the wire",
			SavedAt: clockBase, LastSeenAt: clockBase, UpdatedAt: clockBase,
		},
		{
			TweetID: "sidebar-ken", TweetURL: "https://x.com/ken/status/sidebar-ken",
			AuthorName: "Ken Figure", AuthorHandle: "ken",
			Text:    "no picture at all",
			SavedAt: clockBase, LastSeenAt: clockBase, UpdatedAt: clockBase,
		},
	}
	if _, err := sqlite.NewBookmarkRepository(db).UpsertMany(ctx, seeded); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}
	shell.Reload(ctx)
	shell.state.SidebarFacetTab = sidebarTabAuthors

	tester := ui.NewTester(shell.View, 1180, 900)
	tester.Frame()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, cached := shell.media.Cached(seeded[0].AuthorAvatarURL); cached {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, cached := shell.media.Cached(seeded[0].AuthorAvatarURL); !cached {
		t.Fatal("the ready picture never loaded")
	}
	tester.Frame()

	// The "@" stands for an author with no URL, nothing else: the picture is still in flight,
	// and the source draws its <img> blank until it lands.
	fallbacks := 0
	for _, text := range tester.Texts() {
		if text == shell.t().T(i18n.AuthorAvatarFallback) {
			fallbacks++
		}
	}
	if fallbacks != 1 {
		t.Errorf("%d rows draw the fallback, want only the author without a URL; texts: %q",
			fallbacks, tester.Texts())
	}

	// And the slot is the same width in every row, so the handles line up.
	positions := map[string]float32{}
	for _, handle := range []string{"@ada", "@grace", "@ken"} {
		rect, found := tester.Find(handle)
		if !found {
			t.Fatalf("the row for %s is missing; texts: %q", handle, tester.Texts())
		}
		positions[handle] = rect.X
	}
	for _, handle := range []string{"@grace", "@ken"} {
		if positions[handle] != positions["@ada"] {
			t.Errorf("%s starts at %v, %s at %v: the avatar slot is not one width",
				handle, positions[handle], "@ada", positions["@ada"])
		}
	}
}

// TestAFailedPictureIsRetried pins the retry policy. A dropped connection used to leave the
// placeholder for good: nothing scheduled another frame after a failure, so the picture only
// reappeared if the user happened to redraw. Measured on this machine, 3 of 57 avatar
// requests timed out over the proxy and all three succeeded on a second try.
func TestAFailedPictureIsRetried(t *testing.T) {
	want := []time.Duration{2 * time.Second, 8 * time.Second, 20 * time.Second}
	for i, expected := range want {
		delay, again := mediaRetryDelay(i + 1)
		if !again {
			t.Fatalf("attempt %d is not retried", i+1)
		}
		if delay != expected {
			t.Errorf("attempt %d waits %s, want %s", i+1, delay, expected)
		}
	}
	if _, again := mediaRetryDelay(len(want) + 1); again {
		t.Error("a dead picture is retried forever")
	}
	if _, again := mediaRetryDelay(0); again {
		t.Error("attempt zero must not schedule a retry")
	}
}
