package ui

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/storage/sqlite"
)

// TestTheCardMediaFrameIsSixteenByNine measures the one number the card's height depends on.
//
// The frame is aspect-video, and at the 1297 reference width it is 458 by 257. A media card's
// whole height follows from
// it: every card with a picture is the picture plus the body, so a frame that takes the
// wrong height moves everything under it and makes a two-column grid look ragged.
//
// The capture that prompted this check showed a blank media frame about a fifth taller
// than it was wide-to-height, on a card whose picture had not arrived -- so the placeholder
// is measured too. A placeholder of a different shape is a layout that jumps when the
// picture lands.
func TestTheCardMediaFrameIsSixteenByNine(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()

	bookmark := domain.Bookmark{
		TweetID: "media-1", TweetURL: "https://x.com/ada/status/media-1",
		AuthorName: "ada", AuthorHandle: "ada", Text: "a post with a picture",
		SavedAt: clockBase, LastSeenAt: clockBase, UpdatedAt: clockBase,
		Media: []domain.Media{{Type: domain.MediaPhoto, URL: "https://example.invalid/p.png"}},
	}
	if _, err := sqlite.NewBookmarkRepository(db).UpsertMany(ctx, []domain.Bookmark{bookmark}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}
	shell.Reload(ctx)
	shell.state.LeftSidebarWidth = WidthSidebarDef
	shell.state.RightSidebarWidth = WidthInspectorDef

	// No media fetcher is installed, so the frame is the placeholder -- which must have the
	// picture's shape.
	tester := ui.NewTester(shell.View, 1180, 900)
	tester.Frame()

	rect, found := tester.Find(CardMediaFrameLabel)
	if !found {
		t.Fatalf("the card has no media frame; texts: %q", tester.Texts())
	}
	if rect.W <= 0 || rect.H <= 0 {
		t.Fatalf("the media frame measured %v", rect)
	}
	got := rect.W / rect.H
	if diff := got - CardMediaAspect; diff > 0.02 || diff < -0.02 {
		t.Errorf("the media frame is %.0f by %.0f (ratio %.2f), want 16:9 (%.2f)",
			rect.W, rect.H, got, CardMediaAspect)
	}
}
