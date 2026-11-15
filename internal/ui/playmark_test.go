package ui

import (
	"context"
	"math"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/storage/sqlite"
)

// The video mark: one disc, three sizes, and nothing at all on a photo.
//
// A video's poster is a still frame, so without a mark a video card and a photo card are the
// same picture -- which is what the user saw: "图片和视频现在都是以截图的形式看出来的".
// The disc says "this one moves"; its absence is what a photo looks like. These tests hold the
// three surfaces apart, because the point of one mark is that it is the same drawing scaled,
// not three drawings that happen to look alike.

// cardsFor lays out a library in the given view mode and returns the shell and its tester.
//
// No media fetcher is installed, so every media frame is the placeholder: the mark belongs to
// the post's type, not to the picture having arrived, and a video whose poster is still loading
// is still a video.
func cardsFor(t *testing.T, mode string, bookmarks ...domain.Bookmark) (*Shell, *ui.Tester) {
	t.Helper()

	shell, db := newShell(t)
	if _, err := sqlite.NewBookmarkRepository(db).UpsertMany(context.Background(), bookmarks); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}
	shell.Reload(context.Background())
	shell.state.ViewMode = mode
	shell.state.LeftSidebarWidth = WidthSidebarDef
	shell.state.RightSidebarWidth = WidthInspectorDef

	tester := ui.NewTester(shell.View, 1180, 900)
	tester.Frame()
	return shell, tester
}

// cardBookmark is one post with one attachment.
func cardBookmark(id string, media domain.Media) domain.Bookmark {
	return domain.Bookmark{
		TweetID: id, TweetURL: "https://x.com/ada/status/" + id,
		AuthorName: "Ada Lovelace", AuthorHandle: "ada",
		Text:    "a post with one attachment",
		SavedAt: clockBase, LastSeenAt: clockBase, UpdatedAt: clockBase,
		Media: []domain.Media{media},
	}
}

// poster is a video's still frame: the picture the card draws in place of the video.
func poster(url string) *string { return &url }

// TestAVideoCardCarriesTheMarkCentredOnItsPoster: the grid card's video gets the disc, at the
// card's size and in the middle of the media band rather than in the middle of the card. The
// media is at the top of a card and the body is below it, so a mark centred on the card would
// sit over the words.
func TestAVideoCardCarriesTheMarkCentredOnItsPoster(t *testing.T) {
	// One card: Tester.Find returns the first match, and a second card in the grid would make
	// "the first media frame" and "the first mark" two different cards.
	_, tester := cardsFor(t, domain.ViewGrid,
		cardBookmark("video", domain.Media{
			Type: domain.MediaVideo, URL: "https://example.invalid/clip.mp4",
			PosterURL: poster("https://example.invalid/poster.png"),
		}),
	)

	mark, found := tester.Find(CardPlayMarkLabel)
	if !found {
		t.Fatalf("the video card has no play mark; texts: %q", tester.Texts())
	}
	if math.Abs(float64(mark.W-CardPlaySize)) > 0.5 || math.Abs(float64(mark.H-CardPlaySize)) > 0.5 {
		t.Errorf("the card's disc is %.0f x %.0f, want %d square", mark.W, mark.H, CardPlaySize)
	}

	frame, found := tester.Find(CardMediaFrameLabel)
	if !found {
		t.Fatalf("the video card has no media frame; texts: %q", tester.Texts())
	}
	left := mark.X - frame.X
	right := (frame.X + frame.W) - (mark.X + mark.W)
	top := mark.Y - frame.Y
	bottom := (frame.Y + frame.H) - (mark.Y + mark.H)
	if math.Abs(float64(left-right)) > 1 || math.Abs(float64(top-bottom)) > 1 {
		t.Errorf("the disc sits %.0f/%.0f from the frame's sides and %.0f/%.0f from its top "+
			"and bottom: not centred on the poster", left, right, top, bottom)
	}
}

// TestAPhotoCardCarriesNoMark: the mark is the whole difference between the two kinds, so a
// photo must not get one. Adding a "photo" mark of its own would put ink on most cards to say
// what a picture already says.
func TestAPhotoCardCarriesNoMark(t *testing.T) {
	_, tester := cardsFor(t, domain.ViewGrid,
		cardBookmark("photo", domain.Media{
			Type: domain.MediaPhoto, URL: "https://example.invalid/photo.png",
		}),
	)

	if mark, found := tester.Find(CardPlayMarkLabel); found {
		t.Errorf("the photo card carries a play mark at %v", mark)
	}
}

// TestAGifCardCarriesTheMarkToo: an animated GIF is served as an mp4 and drawn as a still
// poster, so it is a video to the eye in exactly the same way. IsVideoMediaType is what covers
// both; a comparison with MediaVideo alone would leave GIFs unmarked.
func TestAGifCardCarriesTheMarkToo(t *testing.T) {
	_, tester := cardsFor(t, domain.ViewGrid,
		cardBookmark("gif", domain.Media{
			Type: domain.MediaAnimatedGif, URL: "https://example.invalid/clip.mp4",
			PosterURL: poster("https://example.invalid/still.png"),
		}),
	)

	if _, found := tester.Find(CardPlayMarkLabel); !found {
		t.Errorf("the GIF card has no play mark; texts: %q", tester.Texts())
	}
}

// TestTheListThumbnailUsesASmallerDisc: the list's inline media is 148 wide and 108 tall at its
// shortest, so the grid's 40-point disc would cover a third of it. The mark is the same drawing
// one step down, which is why the sizes are named per surface rather than one number.
func TestTheListThumbnailUsesASmallerDisc(t *testing.T) {
	_, tester := cardsFor(t, domain.ViewList,
		cardBookmark("video", domain.Media{
			Type: domain.MediaVideo, URL: "https://example.invalid/clip.mp4",
			PosterURL: poster("https://example.invalid/poster.png"),
		}),
	)

	mark, found := tester.Find(CardPlayMarkLabel)
	if !found {
		t.Fatalf("the list card's video has no play mark; texts: %q", tester.Texts())
	}
	if math.Abs(float64(mark.W-CardListPlaySize)) > 0.5 {
		t.Errorf("the list card's disc is %.0f wide, want %d", mark.W, CardListPlaySize)
	}
	if mark.W >= CardPlaySize {
		t.Errorf("the list card's disc is %.0f wide: the grid's %d does not fit a %d-point "+
			"thumbnail", mark.W, CardPlaySize, CardListSideWidth)
	}

	// The mark is an overlay over the thumbnail, so the box it is centred in has to be the
	// thumbnail and not something that collapsed when the radius moved onto the wrapper.
	thumbnail, found := tester.Find(CardListMediaLabel)
	if !found {
		t.Fatalf("the list card has no media box; texts: %q", tester.Texts())
	}
	if math.Abs(float64(thumbnail.W-CardListSideWidth)) > 0.5 {
		t.Errorf("the thumbnail is %.0f wide, want the %d side track", thumbnail.W, CardListSideWidth)
	}
	if thumbnail.H < CardListSideMinHeight {
		t.Errorf("the thumbnail is %.0f tall, want at least %d", thumbnail.H, CardListSideMinHeight)
	}
	left := mark.X - thumbnail.X
	right := (thumbnail.X + thumbnail.W) - (mark.X + mark.W)
	if math.Abs(float64(left-right)) > 1 {
		t.Errorf("the disc sits %.0f from the thumbnail's left and %.0f from its right: not "+
			"centred", left, right)
	}
}

// TestTheDetailKeepsItsOwnDisc: the detail has the room for the 58-point disc the player's own
// control uses, so the mark there is the full size and the card's is the scaled one.
func TestTheDetailKeepsItsOwnDisc(t *testing.T) {
	server, _ := redPictureServer(t)
	_, tester := detailShell(t, domain.Bookmark{
		TweetID: "detail-mark", TweetURL: "https://x.com/ada/status/detail-mark",
		AuthorName: "Ada Lovelace", AuthorHandle: "ada",
		Text:    "a post with a video",
		SavedAt: clockBase, LastSeenAt: clockBase, UpdatedAt: clockBase,
		Media: []domain.Media{{
			Type: domain.MediaVideo, URL: server.URL + "/clip.mp4",
			PosterURL: poster(server.URL + "/poster.png"),
		}},
	})

	mark, found := tester.Find(DetailPlayMarkLabel)
	if !found {
		t.Fatalf("the detail's video has no play mark; texts: %q", tester.Texts())
	}
	if math.Abs(float64(mark.W-DetailPlaySize)) > 0.5 {
		t.Errorf("the detail's disc is %.0f wide, want %d", mark.W, DetailPlaySize)
	}
}
