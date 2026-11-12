package ui

import (
	"context"
	"testing"

	"github.com/taoquo/trove/apps/macos/internal/domain"
	"github.com/taoquo/trove/apps/macos/internal/storage/sqlite"
)

// TestTheTagEditorCaptureOpensOnTheRowsAtTheTop pins the picture the `tags-editor` state is
// named after.
//
// Round 23 compared the dark and light renditions and found all four identical to `tags`: no
// row was in edit mode in any of them. The state called beginTagEdit with TagItems[0], which
// the repository returns in name order (tag_repo.go:59-84), while the table draws the library
// in bookmark-count order (taglibrary.go compareTagItems). In the capture fixture the
// alphabetically first tag has 13 bookmarks, so the editor opened on a row far below the fold
// and the screenshot published as the inline editor's evidence showed nothing at all. The
// state now opens the first row the table actually draws.
func TestTheTagEditorCaptureOpensOnTheRowsAtTheTop(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	for _, id := range []string{"1", "2", "3"} {
		seed(t, db, id, "ada", "post "+id, clockBase)
	}

	repo := sqlite.NewTagRepository(db)
	for _, tag := range []domain.Tag{
		{ID: "t-alpha", Name: "alpha", CanonicalKey: domain.CanonicalTagKey("alpha")},
		{ID: "t-zebra", Name: "zebra", CanonicalKey: domain.CanonicalTagKey("zebra")},
	} {
		tag.CreatedAt = domain.FormatTimestamp(clockBase)
		tag.UpdatedAt = domain.FormatTimestamp(clockBase)
		if err := repo.Create(ctx, tag); err != nil {
			t.Fatalf("Create tag %q: %v", tag.Name, err)
		}
	}
	// zebra carries more bookmarks, so it is the table's first row; alpha is first by name --
	// the two orders the state used to confuse.
	for _, bookmark := range []struct{ id, tag string }{
		{"1", "t-zebra"}, {"2", "t-zebra"}, {"3", "t-alpha"},
	} {
		if err := repo.Attach(ctx, domain.BookmarkTag{
			BookmarkID: bookmark.id, TagID: bookmark.tag,
			CreatedAt: clockBase, Source: domain.TagSourceManual,
		}); err != nil {
			t.Fatalf("Attach %s: %v", bookmark.id, err)
		}
	}
	shell.Reload(ctx)

	if !shell.ApplyCaptureState("tags-editor") {
		t.Fatal("the capture state is unknown")
	}

	visible := shell.VisibleTagItems()
	if len(visible) < 2 || visible[0].TagID != "t-zebra" {
		t.Fatalf("the fixture no longer puts zebra at the top; visible: %+v", visible)
	}
	if shell.State().TagFocus != visible[0].TagID {
		t.Errorf("the editor opened on %q, not on the first row (%q): the picture shows a table "+
			"that looks untouched", shell.State().TagFocus, visible[0].TagID)
	}
	if shell.State().TagEditField != TagEditName {
		t.Errorf("the open field is %q, want the name field", shell.State().TagEditField)
	}
}
