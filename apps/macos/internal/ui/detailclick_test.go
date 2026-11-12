package ui

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/x-master/apps/macos/internal/domain"
	"github.com/taoquo/x-master/apps/macos/internal/i18n"
	"github.com/taoquo/x-master/apps/macos/internal/storage/sqlite"
)

// TestTheDetailAnswersEveryClick walks the detail overlay the way a pointer meets it:
// the hero (author, tags, three actions), the media, the preview above it, and the way out.
//
// Rounds 35 and 37 found controls that looked pressable and were not, in the toolbar and on
// the card. This is the third surface of that census: the reader, which is the one place a
// user reaches every day. Round 38.
func TestTheDetailAnswersEveryClick(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	shell.SetLocale(i18n.EN, i18n.EN)

	// One post with two pictures: enough for the preview to have both arrows and a counter,
	// with no network picture needed for any of the controls to exist.
	bookmark := domain.Bookmark{
		TweetID: "detail-click", TweetURL: "https://x.com/ada/status/detail-click",
		AuthorName: "ada", AuthorHandle: "ada",
		Text:    "a post the census reads in full",
		SavedAt: clockBase, LastSeenAt: clockBase, UpdatedAt: clockBase,
		Media: []domain.Media{
			{Type: domain.MediaPhoto, URL: "https://example.test/one.png"},
			{Type: domain.MediaPhoto, URL: "https://example.test/two.png"},
		},
	}
	if _, err := sqlite.NewBookmarkRepository(db).UpsertMany(ctx, []domain.Bookmark{bookmark}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}
	seedTagOnly(t, db, "t-rust", "rust")
	if _, err := shell.service.AddTags(ctx, []string{bookmark.TweetID}, []string{"rust"}); err != nil {
		t.Fatalf("attach tag: %v", err)
	}
	shell.Reload(ctx)
	shell.state.Selected = bookmark.TweetID
	shell.state.DetailOpen = true
	shell.state.LeftSidebarWidth = WidthSidebarDef
	shell.state.RightSidebarWidth = WidthInspectorDef

	tester := ui.NewTester(shell.View, 1180, 900)
	tester.Frame()
	click := func(label string) {
		t.Helper()
		tester.Frame()
		if err := tester.Click(label); err != nil {
			t.Fatalf("clicking %q: %v (texts: %q)", label, err, tester.Texts())
		}
		tester.Frame()
	}
	tagsOf := func() []string {
		t.Helper()
		view, err := shell.service.Load(ctx, domain.LibraryQuery{})
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		for _, row := range view.Bookmarks {
			if row.TweetID != bookmark.TweetID {
				continue
			}
			names := make([]string, 0, len(row.Tags))
			for _, tag := range row.Tags {
				names = append(names, tag.Name)
			}
			return names
		}
		return nil
	}

	// The hero names the author: the avatar is a labelled picture, and the name and handle
	// are the text beside it.
	if _, found := tester.Find(DetailAvatarLabel); !found {
		t.Errorf("the detail's hero draws no author picture; texts: %q", tester.Texts())
	}
	if _, found := tester.Find("@ada"); !found {
		t.Errorf("the detail's hero does not show the handle; texts: %q", tester.Texts())
	}

	// The tag pill's × removes that tag. It is named after the tag it removes, so one press
	// is unambiguous where six pills used to read out as six "Remove"s.
	click(shell.t().Tf(i18n.ActionRemoveNamed, "rust"))
	if got := tagsOf(); len(got) != 0 {
		t.Errorf("the pill's × left the post tagged %v", got)
	}

	// The new-tag field creates and attaches in one press of Enter.
	click(DetailTagInputLabel)
	tester.Type("go")
	tester.Key(0, ui.KeyEnter)
	tester.Frame()
	if got := tagsOf(); len(got) != 1 || got[0] != "go" {
		t.Errorf("Enter in the tag field left the post tagged %v, want [go]", got)
	}

	// Copy link answers where the eye is: the toast, and the glyph becoming a tick.
	//
	// Its rectangle is read before the press because the press renames the control to
	// "Copied" for a moment; the detail's actions sit side by side in a known order, and
	// the card behind the modal has an "Open on X" of its own, so the open control is
	// found from its neighbour rather than by a name the list also carries.
	copyRect, found := tester.Find(shell.t().T(i18n.ActionCopyLink))
	if !found {
		t.Fatalf("the detail draws no copy control; texts: %q", tester.Texts())
	}
	openX := copyRect.X + copyRect.W + DetailActionGap + DetailIconButton/2
	openY := copyRect.Y + copyRect.H/2
	click(shell.t().T(i18n.ActionCopyLink))
	if shell.ToastText() == "" {
		t.Error("copying the link said nothing, so the reader cannot tell it worked")
	}

	// Open on X reaches the browser with this post's URL and leaves the reader open.
	opened := ""
	shell.UseExternalOpener(func(url string) error { opened = url; return nil })
	tester.ClickAt(openX, openY)
	tester.Frame()
	if opened != bookmark.TweetURL {
		t.Errorf("Open on X opened %q, want %q", opened, bookmark.TweetURL)
	}
	if !shell.state.DetailOpen {
		t.Error("Open on X closed the detail")
	}

	// A picture opens the preview; the second picture opens it at its own index.
	first, found := tester.Find(DetailMediaTriggerLabel)
	if !found {
		t.Fatalf("the detail draws no media; texts: %q", tester.Texts())
	}
	tester.ClickAt(first.X+first.W/2, first.Y+first.H/2)
	tester.Frame()
	if !shell.LightboxOpen() || shell.state.LightboxIndex != 0 {
		t.Fatalf("pressing the first picture opened %v at %d, want the preview at 0",
			shell.LightboxOpen(), shell.state.LightboxIndex)
	}
	click(shell.t().T(i18n.MediaNext))
	if shell.state.LightboxIndex != 1 {
		t.Errorf("the preview's next arrow left it at %d, want 1", shell.state.LightboxIndex)
	}
	click(shell.t().T(i18n.MediaPrevious))
	if shell.state.LightboxIndex != 0 {
		t.Errorf("the preview's previous arrow left it at %d, want 0", shell.state.LightboxIndex)
	}

	// The preview's own close control closes the preview, not the detail under it.
	click(shell.t().T(i18n.MediaClose))
	if shell.LightboxOpen() {
		t.Error("the preview's close control left the preview open")
	}
	if !shell.state.DetailOpen {
		t.Error("the preview's close control also closed the detail")
	}

	// The detail's close control is the way out. It is drawn as a glyph, so it has to be
	// named as well as tooltipped -- a tooltip is a hover hint, not a name (round 35).
	click(shell.t().T(i18n.InspectorTitle))
	if shell.state.DetailOpen {
		t.Errorf("clicking %q left the detail open", shell.t().T(i18n.InspectorTitle))
	}
}
