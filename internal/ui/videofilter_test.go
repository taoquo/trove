package ui

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/i18n"
	"github.com/taoquo/trove/internal/storage/sqlite"
)

// TestTheVideoRowInTheFilterPanelKeepsOnlyPostsThatPlay walks the condition the way a
// reader does: open the panel, press the row, and the list is the posts that play.
//
// The row is the one control the video filter has, so the test asserts both halves:
// that the panel draws it at all, and that pressing it narrows the list rather than
// merely flipping a flag.
func TestTheVideoRowInTheFilterPanelKeepsOnlyPostsThatPlay(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()

	post := func(id, text string, media ...domain.Media) domain.Bookmark {
		return domain.Bookmark{
			TweetID: id, TweetURL: "https://x.com/ada/status/" + id,
			AuthorName: "ada", AuthorHandle: "ada", Text: text,
			SavedAt: clockBase, LastSeenAt: clockBase, UpdatedAt: clockBase,
			Media: media,
		}
	}
	if _, err := sqlite.NewBookmarkRepository(db).UpsertMany(ctx, []domain.Bookmark{
		post("1", "a post with a picture", domain.Media{Type: domain.MediaPhoto, URL: "p"}),
		post("2", "a post with a video", domain.Media{Type: domain.MediaVideo, URL: "v"}),
		post("3", "a post with an animation", domain.Media{Type: domain.MediaAnimatedGif, URL: "g"}),
		post("4", "a post with nothing"),
	}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}
	shell.Reload(ctx)

	tester := ui.NewTester(shell.View, 1180, 760)
	click := func(label string) {
		t.Helper()
		tester.Frame()
		if err := tester.Click(label); err != nil {
			t.Fatalf("clicking %q: %v (texts: %q)", label, err, tester.Texts())
		}
		tester.Frame()
	}

	click(shell.t().T(i18n.ToolbarFilters))
	if !tester.HasText(shell.t().T(i18n.FilterHasVideo)) {
		t.Fatalf("the filter panel has no video row; texts: %q", tester.Texts())
	}
	click(shell.t().T(i18n.FilterHasVideo))
	if !shell.state.OnlyWithVideo {
		t.Fatalf("pressing %q did not set the condition", shell.t().T(i18n.FilterHasVideo))
	}

	// The click reloads in the background; wait for the read the test is about.
	shell.Reload(ctx)
	tester.Frame()

	for _, text := range []string{"a post with a video", "a post with an animation"} {
		if !tester.HasText(text) {
			t.Errorf("the video filter left out %q; texts: %q", text, tester.Texts())
		}
	}
	for _, text := range []string{"a post with a picture", "a post with nothing"} {
		if tester.HasText(text) {
			t.Errorf("the video filter kept %q; texts: %q", text, tester.Texts())
		}
	}
}
