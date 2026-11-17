package ui

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/storage/sqlite"
)

// TestASearchForATagNameShowsThePostAndItsChip closes the loop on search: the name
// that found the post is drawn on the card, so the result explains itself.
func TestASearchForATagNameShowsThePostAndItsChip(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post about compilers", clockBase)

	tags := sqlite.NewTagRepository(db)
	if err := tags.Create(ctx, domain.Tag{
		ID: "t1", Name: "Kubernetes", CanonicalKey: domain.CanonicalTagKey("Kubernetes"),
		CreatedAt: domain.FormatTimestamp(clockBase), UpdatedAt: domain.FormatTimestamp(clockBase),
	}); err != nil {
		t.Fatalf("Create tag: %v", err)
	}
	if err := tags.Attach(ctx, domain.BookmarkTag{
		BookmarkID: "1", TagID: "t1", CreatedAt: clockBase, Source: domain.TagSourceManual,
	}); err != nil {
		t.Fatalf("Attach: %v", err)
	}

	// The tag's name is nowhere in the post; only the filter can find it.
	shell.state.Query = "kubernetes"
	shell.Reload(ctx)

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	if !tester.HasText("a post about compilers") {
		t.Fatalf("searching a tag name did not find the post filed under it; texts: %q", tester.Texts())
	}
	if !tester.HasText("Kubernetes") {
		t.Errorf("the card does not say why it matched; texts: %q", tester.Texts())
	}
}
