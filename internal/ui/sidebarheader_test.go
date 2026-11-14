package ui

import (
	"context"
	"math"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/i18n"
)

// The sidebar's header and rows, at the width the column opens at.
//
// Both tests are about the same thing on two rows: the column is 212 points wide, and a
// control or a number that sits a band away from the edge it belongs to is what makes it
// read as unfinished (the reader's report of 2026-11-10).

// TestTheFacetControlsSitBesideTheirTabs covers the tab row.
//
// The search, "+" and manage buttons were drawn on a row of their own under the tabs at
// every width, 26 points below the chips they act on, with a band of empty chrome between
// the two. A section's tabs and its controls sit on one line and wrap them only
// when the row runs out.
func TestTheFacetControlsSitBesideTheirTabs(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "a post", clockBase)
	// The column's default locale is the narrow one for these labels; the wide one is what
	// lets the row hold both ends at 212, and it is the case the report was made in.
	shell.translator = i18n.New("zh-CN", "zh-CN")
	shell.state.Data.Tags = []domain.TagSummary{{ID: "1", Name: "golang", BookmarkCount: 2}}
	shell.Reload(context.Background())

	tester := ui.NewTester(shell.View, 900, 760)
	tester.Frame()

	tabs, ok := tester.Find(shell.t().T(i18n.SidebarTags))
	if !ok {
		t.Fatalf("the tags tab is not drawn; texts: %q", tester.Texts())
	}
	search, ok := tester.Find(shell.t().T(i18n.SearchLabel))
	if !ok {
		t.Fatalf("the search toggle is not drawn; texts: %q", tester.Texts())
	}

	tabMiddle := tabs.Y + tabs.H/2
	controlMiddle := search.Y + search.H/2
	if diff := math.Abs(float64(tabMiddle - controlMiddle)); diff > 3 {
		t.Errorf("the tabs are centred at y=%.1f and their controls at y=%.1f: %.1f apart, so the buttons are not on the tab row",
			tabMiddle, controlMiddle, diff)
	}
	if search.X < tabs.X {
		t.Errorf("the search toggle is at x=%.1f, left of the tabs at %.1f", search.X, tabs.X)
	}
}

// TestATagRowsCountReachesTheRowsEdge covers the tag rows.
//
// The count sat before an empty 56-point band: the two hover actions kept their 22-point
// boxes whether or not anything was drawn in them, which at
// the 212-point column is a third of the row. Names were cut short beside a gap with
// nothing in it, and the count stopped well short of the edge the eye reads as the
// column's. The row puts the count at that edge with `margin-left: auto`.
func TestATagRowsCountReachesTheRowsEdge(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(context.Background())
	// After the reload, which is what fills the facet from the library. The per-result
	// counts are off so the row draws the tag's own 7, a number that appears nowhere else on
	// the frame and can be found by its text.
	shell.state.Data.FacetCounts.Ready = false
	shell.state.Data.Tags = []domain.TagSummary{{ID: "1", Name: "golang", BookmarkCount: 7}}

	tester := ui.NewTester(shell.View, 900, 760)
	tester.Frame()

	count, ok := tester.Find("7")
	if !ok {
		t.Fatalf("the row's count is not drawn; texts: %q", tester.Texts())
	}

	// The row fills the column's content box, and its last inset is SubRowPadX.
	edge := shell.state.LeftSidebarWidth - SidebarListsPadX - SubRowPadX
	if got := count.X + count.W; got < edge-1 {
		t.Errorf("the count ends at x=%.1f and the row's inset at %.1f: %.1f points of the row are empty",
			got, edge, edge-got)
	}
}
