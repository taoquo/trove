package ui

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/x-master/apps/macos/internal/domain"
)

// TestTheSidebarTagSearchNarrowsTheTagList covers the field WorkspaceSidebar puts above the tag
// tree.
//
// The field narrows the tag rows. Round 508 removed the two navigation rows the source keeps
// above them, so the tags tab now has nothing else in the list that the query could leave alone.
func TestTheSidebarTagSearchNarrowsTheTagList(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(context.Background())

	// Two tags, set directly: this is about what the view does with them.
	shell.state.Data.Tags = []domain.TagSummary{
		{ID: "1", Name: "golang", BookmarkCount: 2},
		{ID: "2", Name: "rust", BookmarkCount: 1},
	}

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	for _, name := range []string{"golang", "rust"} {
		if !tester.HasText(name) {
			t.Fatalf("tag %q is not in the tree to begin with; texts: %q", name, tester.Texts())
		}
	}

	shell.state.SidebarTagQuery = "go"
	tester.Frame()

	if !tester.HasText("golang") {
		t.Errorf("the matching tag is gone; texts: %q", tester.Texts())
	}
	if tester.HasText("rust") {
		t.Errorf("a tag that does not match the query is still shown; texts: %q", tester.Texts())
	}
	// Clearing brings the other tag back, so this is a filter and not a deletion.
	shell.state.SidebarTagQuery = ""
	tester.Frame()
	if !tester.HasText("rust") {
		t.Errorf("clearing the query did not restore the tree; texts: %q", tester.Texts())
	}
}
