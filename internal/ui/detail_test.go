package ui

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/i18n"
	"github.com/taoquo/trove/internal/media"
	"github.com/taoquo/trove/internal/storage/sqlite"
	"github.com/taoquo/trove/internal/usecase/suggestions"
)

// detailShell opens the detail on one bookmark, with a media fetcher installed.
func detailShell(t *testing.T, bookmark domain.Bookmark) (*Shell, *ui.Tester) {
	t.Helper()

	shell, db := newShell(t)
	shell.UseMediaFetcher(media.NewFetcher())
	if _, err := sqlite.NewBookmarkRepository(db).UpsertMany(context.Background(), []domain.Bookmark{bookmark}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}
	shell.Reload(context.Background())
	shell.state.Selected = bookmark.TweetID
	shell.state.DetailOpen = true
	shell.state.LeftSidebarWidth = WidthSidebarDef
	shell.state.RightSidebarWidth = WidthInspectorDef

	tester := ui.NewTester(shell.View, 1180, 900)
	tester.Frame()
	return shell, tester
}

// redPictureServer serves one red PNG at any path and counts the requests per path.
func redPictureServer(t *testing.T) (*httptest.Server, func(path string) int) {
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

	return server, func(path string) int {
		mu.Lock()
		defer mu.Unlock()
		return served[path]
	}
}

// TestTheDetailShowsTheAuthorsPicture is the user's report: "详情页中，这个头像的 icon 也没有换掉".
//
// The hero draws the author's picture when there is one, the same rule as the card. Drawing
// the initials either way would give the same author a picture on the card and a letter in the
// detail.
func TestTheDetailShowsTheAuthorsPicture(t *testing.T) {
	server, _ := redPictureServer(t)
	avatar := server.URL + "/ada.png"

	shell, tester := detailShell(t, domain.Bookmark{
		TweetID: "detail-avatar", TweetURL: "https://x.com/ada/status/detail-avatar",
		AuthorName: "Ada Lovelace", AuthorHandle: "ada", AuthorAvatarURL: avatar,
		Text:    "a post by an author with a picture",
		SavedAt: clockBase, LastSeenAt: clockBase, UpdatedAt: clockBase,
	})

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, cached := shell.media.Cached(avatar); cached {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, cached := shell.media.Cached(avatar); !cached {
		t.Fatal("the avatar never loaded")
	}

	tester.Frame()
	rect, found := tester.Find(DetailAvatarLabel)
	if !found {
		t.Fatal("the detail has no avatar at all")
	}

	// The picture is pure red and the fallback disc is the theme's grey, so the middle of
	// the avatar tells the two apart.
	img := tester.Image()
	scale := float32(img.Rect.Dx()) / 1180
	px := img.RGBAAt(int((rect.X+rect.W/2)*scale), int((rect.Y+rect.H/2)*scale))
	if px.R < 150 || px.G > 120 || px.B > 120 {
		t.Errorf("the detail avatar centre is %v, want the picture's red", px)
	}
	if tester.HasText("AL") {
		t.Error("the detail still draws the initials over a picture that has loaded")
	}
}

// TestTheTagFieldDoesNotTakeTheFocus is the user's second report: "标签的输入框一直是激活状态，
// 不能去激活".
//
// MyGo moves the focus into a dialog as it opens (ui/scope.go, arrangeFocus). The tag field
// was the only focusable in the detail, so it took that focus and drew a focus ring and a
// caret as if the user had clicked it. The dialog's close control now takes it instead, and
// clicking elsewhere inside the detail lets it go.
func TestTheTagFieldDoesNotTakeTheFocus(t *testing.T) {
	_, tester := detailShell(t, domain.Bookmark{
		TweetID: "detail-focus", TweetURL: "https://x.com/ada/status/detail-focus",
		AuthorName: "Ada Lovelace", AuthorHandle: "ada",
		Text:    "a post whose detail can be clicked",
		SavedAt: clockBase, LastSeenAt: clockBase, UpdatedAt: clockBase,
	})

	if tester.Focused(DetailTagInputLabel) {
		t.Fatal("the new-tag field has the focus as the detail opens")
	}
	if _, ok := tester.TextCaret(); ok {
		t.Error("a text caret is showing although nothing was clicked")
	}

	if err := tester.Click(DetailTagInputLabel); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if !tester.Focused(DetailTagInputLabel) {
		t.Fatal("clicking the field does not focus it")
	}

	// The post's own text is inside the dialog and is not a control: the click has to take the
	// focus away without closing the detail.
	if err := tester.Click("a post whose detail can be clicked"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if tester.Focused(DetailTagInputLabel) {
		t.Error("the field keeps the focus after a click elsewhere in the detail")
	}
}

// TestTheDetailMediaOpensThePreview is the user's third report: "这个媒体资源也不能预览，不能点击".
//
// The media section is a button that opens the lightbox. The picture here is wrapped in a
// clickable box; the test presses it.
func TestTheDetailMediaOpensThePreview(t *testing.T) {
	server, _ := redPictureServer(t)
	shell, tester := detailShell(t, domain.Bookmark{
		TweetID: "detail-media", TweetURL: "https://x.com/ada/status/detail-media",
		AuthorName: "Ada Lovelace", AuthorHandle: "ada",
		Text:    "a post with one picture",
		SavedAt: clockBase, LastSeenAt: clockBase, UpdatedAt: clockBase,
		Media: []domain.Media{{Type: domain.MediaPhoto, URL: server.URL + "/one.png"}},
	})

	if err := tester.Click(DetailMediaTriggerLabel); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if !shell.LightboxOpen() {
		t.Fatal("pressing the detail's media did not open the preview")
	}
	if shell.state.LightboxIndex != 0 {
		t.Errorf("the preview opened at %d, want the first item", shell.state.LightboxIndex)
	}
}

// TestThePreviewSitsAboveTheDetail is the third report's other half: the preview opened, but
// it was drawn behind the detail card, so its controls could not be pressed.
//
// The detail is a DialogBase and the lightbox a ui.Overlay; inside MyGo's overlay layer the
// in-flow children paint first and the absolute ones after, and the dialog's backdrop is
// absolute -- so the lightbox, which was in flow, painted underneath it. Both layers are
// absolute now (shell.go, the lightbox block). The check is the consequence rather than the
// tree: the preview's own close control closes the preview and leaves the detail open.
func TestThePreviewSitsAboveTheDetail(t *testing.T) {
	server, _ := redPictureServer(t)
	shell, tester := detailShell(t, domain.Bookmark{
		TweetID: "detail-stack", TweetURL: "https://x.com/ada/status/detail-stack",
		AuthorName: "Ada Lovelace", AuthorHandle: "ada",
		Text:    "a post whose preview sits above it",
		SavedAt: clockBase, LastSeenAt: clockBase, UpdatedAt: clockBase,
		Media: []domain.Media{{Type: domain.MediaPhoto, URL: server.URL + "/stack.png"}},
	})

	shell.OpenLightbox(0)
	tester.Frame()
	if !shell.LightboxOpen() {
		t.Fatal("the preview did not open")
	}

	// The backdrop is the lightbox's own dim; with the lightbox under the detail, the press
	// lands on the detail's backdrop and closes the detail instead.
	if err := tester.Click(i18n.New("en", "en").T(i18n.MediaClose)); err != nil {
		t.Fatalf("Click: %v", err)
	}
	tester.Frame()
	if shell.LightboxOpen() {
		t.Error("the preview's close control did not close it: the preview is under the detail")
	}
	if !shell.state.DetailOpen {
		t.Error("closing the preview closed the detail as well")
	}
}

// TestAVideoShowsItsPosterNotTheFile pins which URL the detail asks the image fetcher for.
//
// A video shows its poster and never the video file itself (`getMediaPosterUrl(bookmark, 0) ??
// mediaUrl`). Putting media[0].URL into the image fetcher cannot decode an .mp4: the detail's
// media stays an empty frame and the file is requested on every retry.
func TestAVideoShowsItsPosterNotTheFile(t *testing.T) {
	server, requests := redPictureServer(t)
	poster := server.URL + "/poster.png"
	video := server.URL + "/clip.mp4"

	shell, tester := detailShell(t, domain.Bookmark{
		TweetID: "detail-video", TweetURL: "https://x.com/ada/status/detail-video",
		AuthorName: "Ada Lovelace", AuthorHandle: "ada",
		Text:    "a post with a video",
		SavedAt: clockBase, LastSeenAt: clockBase, UpdatedAt: clockBase,
		Media: []domain.Media{{Type: domain.MediaVideo, URL: video, PosterURL: &poster}},
	})

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, cached := shell.media.Cached(poster); cached {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	tester.Frame()

	if _, cached := shell.media.Cached(poster); !cached {
		t.Fatal("the video's poster never loaded")
	}
	if n := requests("/clip.mp4"); n != 0 {
		t.Errorf("the video file was requested %d times; an image fetcher cannot decode it", n)
	}
	if _, found := tester.Find(DetailMediaTriggerLabel); !found {
		t.Error("the detail has no media picture")
	}
	// The lightbox draws the same picture, so it is not asked for the file either.
	shell.OpenLightbox(0)
	tester.Frame()
	if n := requests("/clip.mp4"); n != 0 {
		t.Errorf("opening the preview requested the video file %d times", n)
	}
}

// TestEachPictureOpensItsOwnPreview pins the post's media grid: two pictures draw two
// triggers, and the second one opens the second picture rather than the first.
//
// Drawing media[0] alone would show one picture of four and a count, with no way to reach the
// others.
func TestEachPictureOpensItsOwnPreview(t *testing.T) {
	server, _ := redPictureServer(t)
	shell, tester := detailShell(t, domain.Bookmark{
		TweetID: "detail-pair", TweetURL: "https://x.com/ada/status/detail-pair",
		AuthorName: "Ada Lovelace", AuthorHandle: "ada",
		Text:    "a post with two pictures",
		SavedAt: clockBase, LastSeenAt: clockBase, UpdatedAt: clockBase,
		Media: []domain.Media{
			{Type: domain.MediaPhoto, URL: server.URL + "/one.png"},
			{Type: domain.MediaPhoto, URL: server.URL + "/two.png"},
		},
	})

	first, found := tester.Find(DetailMediaTriggerLabel)
	if !found {
		t.Fatalf("the detail draws no media; texts: %q", tester.Texts())
	}

	tester.ClickAt(first.X+first.W/2, first.Y+first.H/2)
	if !shell.LightboxOpen() || shell.state.LightboxIndex != 0 {
		t.Fatalf("pressing the first picture opened %v at %d, want the preview at 0",
			shell.LightboxOpen(), shell.state.LightboxIndex)
	}
	shell.CloseLightbox()
	tester.Frame()

	// The two share a row, so the second is one picture and one gap to the right.
	tester.ClickAt(first.X+first.W+DetailMediaGridGap+first.W/2, first.Y+first.H/2)
	if !shell.LightboxOpen() || shell.state.LightboxIndex != 1 {
		t.Errorf("pressing the second picture opened %v at %d, want the preview at 1",
			shell.LightboxOpen(), shell.state.LightboxIndex)
	}
}

// TestAVideoOffersPlayInThePreview is the gap the user reported: "如果是视频的话，其实是
// 不能播放的". The poster is a still picture; the preview puts the one control a still cannot
// have over it, and that control hands the file to a player.
//
// With no native player installed -- which is what a test has, since the AVKit view needs a
// real window -- the shell falls back to the system's player rather than doing nothing.
func TestAVideoOffersPlayInThePreview(t *testing.T) {
	server, _ := redPictureServer(t)
	poster := server.URL + "/poster.png"
	video := server.URL + "/clip.mp4"
	shell, tester := detailShell(t, domain.Bookmark{
		TweetID: "detail-play", TweetURL: "https://x.com/ada/status/detail-play",
		AuthorName: "Ada Lovelace", AuthorHandle: "ada",
		Text:    "a post with a video",
		SavedAt: clockBase, LastSeenAt: clockBase, UpdatedAt: clockBase,
		Media: []domain.Media{{Type: domain.MediaVideo, URL: video, PosterURL: &poster}},
	})

	shell.OpenLightbox(0)
	tester.Frame()
	play := i18n.New("en", "en").T(i18n.MediaPlay)
	if !tester.HasText(play) {
		t.Fatalf("the preview of a video offers no play control; texts: %q", tester.Texts())
	}

	var opened string
	shell.UseExternalOpener(func(rawURL string) error {
		opened = rawURL
		return nil
	})
	if err := tester.Click(play); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if opened != video {
		t.Errorf("play handed %q to the player, want the video file %q", opened, video)
	}
}

// TestClosingThePreviewTakesThePlayerAway: the player is a view over the window, not a child
// of the overlay, so nothing about closing the preview would remove it unless this does.
func TestClosingThePreviewTakesThePlayerAway(t *testing.T) {
	shell, _ := newShell(t)
	player := &recordingPlayer{}
	shell.UseVideoPlayer(player)
	shell.state.LightboxIndex = 0
	shell.state.VideoURL = "https://video.twimg.com/clip.mp4"

	shell.CloseLightbox()

	if shell.VideoPlaying() {
		t.Error("the preview closed but the shell still says a video is playing")
	}
	if !player.hidden {
		t.Error("the native player was never told to hide")
	}
}

// recordingPlayer is a VideoPlayer that records what it was asked to do.
type recordingPlayer struct {
	shown  string
	hidden bool
}

func (p *recordingPlayer) Show(_ *mygo.Window, url string) error {
	p.shown = url
	return nil
}

func (p *recordingPlayer) Hide() { p.hidden = true }

// TestTheDetailDrawsThePostAndNothingElse pins the rail's absence: its properties (kind, list)
// and its timeline (published, saved, last sync) are not drawn.
//
// The reader's report: "属性信息其实完全没有必要……都差不多，都一样的" and "保存和同步的时间其实
// 没有那么大的意义". What survives is the published date as a clock on the author's line, and the
// tags on the card's floor.
func TestTheDetailDrawsThePostAndNothingElse(t *testing.T) {
	shell, tester := detailShell(t, domain.Bookmark{
		TweetID: "detail-kind", TweetURL: "https://x.com/ada/status/detail-kind",
		AuthorName: "Ada Lovelace", AuthorHandle: "ada",
		Text: "a long form note", SourceKind: domain.SourceNoteTweet,
		CreatedAtOnX: "Mar 3, 2024",
		SavedAt:      clockBase, LastSeenAt: clockBase, UpdatedAt: clockBase,
	})
	for i := range shell.state.Data.Bookmarks {
		shell.state.Data.Bookmarks[i].List = domain.List{ID: "list-inbox", Name: "Inbox"}
	}
	tester.Frame()

	// The published date is on the author's line, beside the avatar.
	if !tester.HasText("Mar 3, 2024") {
		t.Fatalf("the published date is missing; texts: %q", tester.Texts())
	}
	// The detail's avatar is the only one with this label; the card behind the overlay has
	// its own, so this is the hero's line and not the results list's.
	avatar, found := tester.Find(DetailAvatarLabel)
	if !found {
		t.Fatal("the detail has no hero")
	}
	date, found := tester.Find("Mar 3, 2024")
	if !found {
		t.Fatal("the published date is not drawn at all")
	}
	if date.Y < avatar.Y || date.Y > avatar.Y+avatar.H {
		t.Errorf("the date at y=%.0f is outside the hero's line at y=%.0f", date.Y, avatar.Y)
	}

	// None of the rail's facts and none of its headings are drawn.
	for _, gone := range []string{"Inbox", "Properties", "Timeline", "Saved", "Last sync", "Long post"} {
		for _, text := range tester.Texts() {
			if text == gone {
				t.Errorf("%q is still drawn; the rail was removed", gone)
			}
		}
	}
}

// TestTheDetailShowsPendingSuggestions pins that the detail shows the dashed suggestion chips
// the cards show, in the tag row.
func TestTheDetailShowsPendingSuggestions(t *testing.T) {
	shell, tester := detailShell(t, domain.Bookmark{
		TweetID: "detail-suggest", TweetURL: "https://x.com/ada/status/detail-suggest",
		AuthorName: "Ada Lovelace", AuthorHandle: "ada",
		Text:    "a post with a suggestion waiting",
		SavedAt: clockBase, LastSeenAt: clockBase, UpdatedAt: clockBase,
	})
	shell.state.PendingSuggestions = []suggestions.PendingRow{{
		Suggestion: domain.TagSuggestion{
			ID: "s1", BookmarkID: "detail-suggest", TagName: "screencast",
		},
	}}
	tester.Frame()

	if !tester.HasText("screencast") {
		t.Fatalf("the detail draws no suggestion chip; texts: %q", tester.Texts())
	}
}

// TestTheHeaderStaysAboveALongPost is the reader's complaint made into a check: a long post used
// to carry the card's controls off the bottom of the card, because the whole card was one scroll
// region. The header is outside the scroll now, so it is on screen whatever the post's length.
func TestTheHeaderStaysAboveALongPost(t *testing.T) {
	body := ""
	for i := 0; i < 40; i++ {
		body += "a paragraph long enough to fill the reading pane several times over.\n\n"
	}
	_, tester := detailShell(t, domain.Bookmark{
		TweetID: "detail-long", TweetURL: "https://x.com/ada/status/detail-long",
		AuthorName: "Ada Lovelace", AuthorHandle: "ada",
		Text:    body,
		SavedAt: clockBase, LastSeenAt: clockBase, UpdatedAt: clockBase,
	})

	// The detail's own avatar is the anchor: the card behind the overlay has its own label.
	avatar, found := tester.Find(DetailAvatarLabel)
	if !found {
		t.Fatal("the detail has no header at all")
	}
	if avatar.Y+avatar.H > 200 {
		t.Errorf("the header is at y=%.0f in a 900-point window: the post pushed it out",
			avatar.Y)
	}
}
