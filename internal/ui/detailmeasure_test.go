package ui

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/taoquo/trove/internal/domain"
)

// The detail's measure and its media cap: the post reads in a column narrower than the pane,
// and a picture taller than the cap is scaled down whole rather than cropped to the pane.
//
// The numbers are measured off the real layout rather than compared with the constants alone,
// because the point is the shape the reader gets: a text column that stops short of the pane's
// right edge, a portrait picture that keeps its own frame and is centred, and a gallery whose
// tiles share one shape.

// sizedPictureServer serves one PNG of the given size at any path.
func sizedPictureServer(t *testing.T, width, height int) *httptest.Server {
	t.Helper()

	picture := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			picture.Set(x, y, color.RGBA{R: 200, G: 60, B: 60, A: 255})
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, picture); err != nil {
		t.Fatalf("Encode: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(encoded.Bytes())
	}))
	t.Cleanup(server.Close)
	return server
}

// waitForPicture blocks until the fetcher has decoded the picture, which is when the layout
// knows its shape; until then the frame is the 16:9 placeholder.
func waitForPicture(t *testing.T, shell *Shell, url string) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, cached := shell.media.Cached(url); cached {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("the picture at %s never loaded", url)
}

// bodyText is long enough to wrap several times at the reading measure, so its box is the
// measure itself rather than the width of its longest line.
const bodyText = "The best interfaces assume you are competent and tired. Every confirmation " +
	"dialog is a small bet that you are neither. A dialog that asks again what the last screen " +
	"already knew is the tax a product charges for its own doubt, and the readers pay it every " +
	"time they open a bookmark that only wanted to be read once."

// longBody wraps past the measure's width, so its box is the measure and not the text.
const longBody = bodyText + " The measure is a cap, so a short post is still a short line " +
	"rather than a centred one, and the pictures may still use the whole pane."

// TestTheDetailBodyKeepsItsOwnMeasure is the reading column: 728 wide is about 56 CJK
// characters a line, which is past the point where the eye loses the start of the line it is
// returning to. The column is a cap, so only a post long enough to wrap shows it.
func TestTheDetailBodyKeepsItsOwnMeasure(t *testing.T) {
	server := sizedPictureServer(t, 800, 450)
	shell, tester := detailShell(t, domain.Bookmark{
		TweetID: "measure", TweetURL: "https://x.com/ada/status/measure",
		AuthorName: "Ada Lovelace", AuthorHandle: "ada",
		Text:    longBody,
		SavedAt: clockBase, LastSeenAt: clockBase, UpdatedAt: clockBase,
		Media: []domain.Media{{Type: domain.MediaPhoto, URL: server.URL + "/wide.png"}},
	})
	waitForPicture(t, shell, server.URL+"/wide.png")
	tester.Frame()

	text, found := tester.Find(DetailBodyLabel)
	if !found {
		t.Fatalf("the post's words are not drawn; texts: %q", tester.Texts())
	}
	if pane := shell.detailPaneWidth(); text.W >= pane {
		t.Errorf("the body is %.0f wide, the pane is %.0f: it still runs the full width",
			text.W, pane)
	}
	if math.Abs(float64(text.W-DetailReadWidth)) > 0.5 {
		t.Errorf("the body is %.0f wide, want the %d measure", text.W, DetailReadWidth)
	}
}

// TestATallPictureIsShownWholeAtTheCap is B2: the height is capped, the width follows the
// picture's own ratio, so a portrait photo is scaled down whole and centred instead of being
// cropped to a band. The pane shows the frame the lightbox will open, only smaller.
func TestATallPictureIsShownWholeAtTheCap(t *testing.T) {
	server := sizedPictureServer(t, 800, 1000) // 4:5, the clamp's tall end
	shell, tester := detailShell(t, domain.Bookmark{
		TweetID: "tall", TweetURL: "https://x.com/ada/status/tall",
		AuthorName: "Ada Lovelace", AuthorHandle: "ada",
		Text:    bodyText,
		SavedAt: clockBase, LastSeenAt: clockBase, UpdatedAt: clockBase,
		Media: []domain.Media{{Type: domain.MediaPhoto, URL: server.URL + "/tall.png"}},
	})
	waitForPicture(t, shell, server.URL+"/tall.png")
	tester.Frame()

	frame, found := tester.Find(DetailMediaTriggerLabel)
	if !found {
		t.Fatalf("the picture is not drawn; texts: %q", tester.Texts())
	}
	if math.Abs(float64(frame.H-DetailMediaCap)) > 0.5 {
		t.Errorf("the portrait frame is %.0f high, want the %d cap", frame.H, DetailMediaCap)
	}
	if want := float32(DetailMediaCap) * 800 / 1000; math.Abs(float64(frame.W-want)) > 0.5 {
		t.Errorf("the portrait frame is %.0f wide, want %.0f: it was cropped, not scaled",
			frame.W, want)
	}

	// Scaled down, so it is narrower than the pane and has to be centred in it. The body is
	// the anchor: it starts at the pane's left edge.
	text, found := tester.Find(DetailBodyLabel)
	if !found {
		t.Fatalf("the post's words are not drawn; texts: %q", tester.Texts())
	}
	left := frame.X - text.X
	right := (text.X + shell.detailPaneWidth()) - (frame.X + frame.W)
	if math.Abs(float64(left-right)) > 1 {
		t.Errorf("the portrait sits %.0f from the pane's left and %.0f from its right: not centred",
			left, right)
	}
}

// TestAWidePictureFillsThePane: a picture that is not tall enough to reach the cap still uses
// the whole column, so the 600-point measure narrows the words and nothing else.
func TestAWidePictureFillsThePane(t *testing.T) {
	server := sizedPictureServer(t, 1600, 900)
	shell, tester := detailShell(t, domain.Bookmark{
		TweetID: "wide", TweetURL: "https://x.com/ada/status/wide",
		AuthorName: "Ada Lovelace", AuthorHandle: "ada",
		Text:    bodyText,
		SavedAt: clockBase, LastSeenAt: clockBase, UpdatedAt: clockBase,
		Media: []domain.Media{{Type: domain.MediaPhoto, URL: server.URL + "/wide.png"}},
	})
	waitForPicture(t, shell, server.URL+"/wide.png")
	tester.Frame()

	frame, found := tester.Find(DetailMediaTriggerLabel)
	if !found {
		t.Fatalf("the picture is not drawn; texts: %q", tester.Texts())
	}
	pane := shell.detailPaneWidth()
	if math.Abs(float64(frame.W-pane)) > 0.5 {
		t.Errorf("the wide picture is %.0f wide, want the pane's %.0f", frame.W, pane)
	}
	if want := pane * 900 / 1600; math.Abs(float64(frame.H-want)) > 0.5 {
		t.Errorf("the wide picture is %.0f high, want %.0f", frame.H, want)
	}
}

// TestTheGalleryTilesShareOneShape: a square and a 16:9 picture in one row are drawn at the
// same size, so the row reads as a set rather than as one hole beside a picture. Their own
// ratios are what the lightbox is for.
func TestTheGalleryTilesShareOneShape(t *testing.T) {
	square := sizedPictureServer(t, 600, 600)
	wide := sizedPictureServer(t, 1600, 900)
	shell, tester := detailShell(t, domain.Bookmark{
		TweetID: "pair", TweetURL: "https://x.com/ada/status/pair",
		AuthorName: "Ada Lovelace", AuthorHandle: "ada",
		Text:    "two pictures of different shapes",
		SavedAt: clockBase, LastSeenAt: clockBase, UpdatedAt: clockBase,
		Media: []domain.Media{
			{Type: domain.MediaPhoto, URL: square.URL + "/one.png"},
			{Type: domain.MediaPhoto, URL: wide.URL + "/two.png"},
		},
	})
	waitForPicture(t, shell, square.URL+"/one.png")
	waitForPicture(t, shell, wide.URL+"/two.png")
	tester.Frame()

	first, found := tester.Find(DetailMediaTriggerLabel)
	if !found {
		t.Fatalf("the first picture is not drawn; texts: %q", tester.Texts())
	}
	if want := first.W / DetailMediaTileRatio; math.Abs(float64(first.H-want)) > 0.5 {
		t.Errorf("the tile is %.0f x %.0f, want the 3:2 tile's %.0f height",
			first.W, first.H, want)
	}
	// The two tiles and the gap between them fill the pane, so neither picture's own shape
	// has a say in the size it is drawn at.
	if want := (shell.detailPaneWidth() - DetailMediaGridGap) / 2; math.Abs(float64(first.W-want)) > 0.5 {
		t.Errorf("the tile is %.0f wide, want %.0f", first.W, want)
	}
}

// TestAPostWithNoWordsStartsWithItsPicture: the 14-point gap is a paragraph's, and a post with
// no words has no paragraph for the picture to follow. The block goes with the text rather
// than leaving a blank band under the header.
func TestAPostWithNoWordsStartsWithItsPicture(t *testing.T) {
	const name = "Ada Lovelace"
	server := sizedPictureServer(t, 600, 600)
	// open measures the picture's start against the header's avatar, which sits at a fixed
	// offset from the dialog's top: the dialog is centred, so a taller post moves everything
	// down together and the header is what stays put relative to it.
	open := func(text string) (frameGap, textHeight float32) {
		t.Helper()
		shell, tester := detailShell(t, domain.Bookmark{
			TweetID: "notext", TweetURL: "https://x.com/ada/status/notext",
			AuthorName: name, AuthorHandle: "ada",
			Text:    text,
			SavedAt: clockBase, LastSeenAt: clockBase, UpdatedAt: clockBase,
			Media: []domain.Media{{Type: domain.MediaPhoto, URL: server.URL + "/one.png"}},
		})
		waitForPicture(t, shell, server.URL+"/one.png")
		tester.Frame()
		frame, found := tester.Find(DetailMediaTriggerLabel)
		if !found {
			t.Fatalf("the picture is not drawn; texts: %q", tester.Texts())
		}
		avatar, found := tester.Find(DetailAvatarLabel)
		if !found {
			t.Fatalf("the detail's avatar is not drawn; texts: %q", tester.Texts())
		}
		if text != "" {
			body, found := tester.Find(DetailBodyLabel)
			if !found {
				t.Fatalf("the post's words are not drawn; texts: %q", tester.Texts())
			}
			textHeight = body.H
		}
		return frame.Y - avatar.Y, textHeight
	}

	blankGap, _ := open("")
	textGap, textHeight := open(bodyText)

	if want := textHeight + DetailPostGap; math.Abs(float64(textGap-blankGap-want)) > 0.5 {
		t.Errorf("the picture starts %.0f below the byline with words and %.0f without; "+
			"want %.0f apart (the words plus the paragraph gap)",
			textGap, blankGap, want)
	}
}
