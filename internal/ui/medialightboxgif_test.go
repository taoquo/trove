package ui

import (
	"testing"

	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/i18n"
)

// TestAGifOffersPlayInThePreview closes the oldest open Inbox item: a GIF never moved.
//
// X serves an animated GIF as an mp4 (parse.go keeps the poster and takes the best mp4 for
// every video *and* animated_gif), and both are treated alike everywhere the code asks "does
// this play". Comparing the type to MediaVideo alone draws a GIF as a still with no play control
// on either surface -- the one control that would have started the file.
func TestAGifOffersPlayInThePreview(t *testing.T) {
	server, _ := redPictureServer(t)
	poster := server.URL + "/poster.png"
	clip := server.URL + "/loop.mp4"
	shell, tester := detailShell(t, domain.Bookmark{
		TweetID: "detail-gif", TweetURL: "https://x.com/ada/status/detail-gif",
		AuthorName: "Ada Lovelace", AuthorHandle: "ada",
		Text:    "a post with a GIF",
		SavedAt: clockBase, LastSeenAt: clockBase, UpdatedAt: clockBase,
		Media: []domain.Media{{Type: domain.MediaAnimatedGif, URL: clip, PosterURL: &poster}},
	})

	shell.OpenLightbox(0)
	tester.Frame()

	play := i18n.New("en").T(i18n.MediaPlayGif)
	if !tester.HasText(play) {
		t.Fatalf("the preview of a GIF offers no play control; texts: %q", tester.Texts())
	}
	// The control is over a GIF, so it must not call the thing a video.
	if tester.HasText(i18n.New("en").T(i18n.MediaPlay)) {
		t.Errorf("a GIF's play control is named after a video; texts: %q", tester.Texts())
	}

	var opened string
	shell.UseExternalOpener(func(rawURL string) error {
		opened = rawURL
		return nil
	})
	if err := tester.Click(play); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if opened != clip {
		t.Errorf("play handed %q to the player, want the GIF's file %q", opened, clip)
	}
}

// TestThePreviewNamesItself gives the overlay a name.
//
// The lightbox is a modal dialog, `role="dialog" aria-modal="true"` with aria-label=copy.mediaTitle.
// A modal with no name is announced as an unlabelled dialog over the post.
func TestThePreviewNamesItself(t *testing.T) {
	server, _ := redPictureServer(t)
	picture := server.URL + "/one.png"
	shell, tester := detailShell(t, domain.Bookmark{
		TweetID: "detail-name", TweetURL: "https://x.com/ada/status/detail-name",
		AuthorName: "Ada Lovelace", AuthorHandle: "ada",
		Text:    "a post with a picture",
		SavedAt: clockBase, LastSeenAt: clockBase, UpdatedAt: clockBase,
		Media: []domain.Media{{Type: domain.MediaPhoto, URL: picture}},
	})

	shell.OpenLightbox(0)
	tester.Frame()

	want := i18n.New("en").T(i18n.MediaTitle)
	if _, ok := tester.Find(want); !ok {
		t.Errorf("the preview dialog has no name; want %q, texts: %q", want, tester.Texts())
	}
}
