package ui

import (
	"context"
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/x-master/apps/macos/internal/domain"
	"github.com/taoquo/x-master/apps/macos/internal/i18n"
)

// TestTheFacetTabsSwitchTheList covers the control that replaced the source's two stacked,
// separately folded sections.
//
// Both halves are asserted: the list that goes away and the one that arrives. "The tags body
// is absent" alone would also pass if the sidebar had failed to draw at all, which is the
// same trap the fold test this replaces was written around.
//
// The result list is narrowed to nothing on purpose: with a card on screen its author line
// would draw the same name as the sidebar row, and the assertion could not tell them apart.
func TestTheFacetTabsSwitchTheList(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(ctx)
	shell.state.Query = "nothing matches this"
	shell.Reload(ctx)

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	tagsTab := shell.t().T(i18n.SidebarTags)
	authorsTab := shell.t().T(i18n.AuthorsTitle)
	emptyTags := shell.t().T(i18n.SidebarNoTagsCreated)

	if !tester.HasText(emptyTags) {
		t.Fatalf("the tags tab is not the one drawn first; texts: %q", tester.Texts())
	}
	if containsText(tester.Texts(), "ada") {
		t.Fatalf("author rows are drawn while the tags tab is open; texts: %q", tester.Texts())
	}

	if err := tester.Click(authorsTab); err != nil {
		t.Fatalf("there is no authors tab to click: %v", err)
	}
	tester.Frame()

	if tester.HasText(emptyTags) {
		t.Errorf("the tags body survived the switch to the authors tab")
	}
	if !containsText(tester.Texts(), "ada") {
		t.Errorf("the authors tab draws no rows; texts: %q", tester.Texts())
	}
	// Both labels stay put: they are the way back.
	for _, label := range []string{tagsTab, authorsTab} {
		if !tester.HasText(label) {
			t.Errorf("the %q tab label is gone; texts: %q", label, tester.Texts())
		}
	}

	if err := tester.Click(tagsTab); err != nil {
		t.Fatalf("there is no tags tab to click back to: %v", err)
	}
	tester.Frame()
	if !tester.HasText(emptyTags) {
		t.Errorf("switching back did not bring the tags list back; texts: %q", tester.Texts())
	}
}

// TestTheOtherTabShowsThatItIsFiltering covers the one thing a single-slot sidebar owes a
// filter it is not showing: a mark on the tab it belongs to.
//
// The chip in the results toolbar names the condition already; this is the sidebar's own
// answer to "why is my list shorter than the tab I am looking at says".
func TestTheOtherTabShowsThatItIsFiltering(t *testing.T) {
	shell, _ := newShell(t)
	shell.Reload(context.Background())

	shell.state.ActiveTagIDs = []string{"some-tag"}
	shell.state.SidebarFacetTab = sidebarTabAuthors

	if !shell.facetTabFiltered(sidebarTabTags) {
		t.Error("a tag condition does not mark the tags tab")
	}
	if shell.facetTabFiltered(sidebarTabAuthors) {
		t.Error("the authors tab claims a condition it does not have")
	}

	shell.state.ActiveTagIDs = nil
	shell.state.ActiveAuthors = []string{"ada"}
	if shell.facetTabFiltered(sidebarTabTags) {
		t.Error("the tags tab keeps its mark after the tag condition was cleared")
	}
	if !shell.facetTabFiltered(sidebarTabAuthors) {
		t.Error("an author condition does not mark the authors tab")
	}
}

// TestTheTagsTabDrawsNothingAboveTheTags covers the row removal round 508 was asked for.
//
// The source opens its tag tree with 全部书签 and 待确认标签 (WorkspaceSidebar.tsx:478 and :501),
// and both repeated something already on screen: the results header names 全部书签 as the scope
// and counts it, and the pending work belongs with the library's other tag filters. The tab is
// the tag rows and nothing else.
func TestTheTagsTabDrawsNothingAboveTheTags(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(context.Background())
	shell.state.LeftSidebarWidth = WidthSidebarDef
	shell.state.RightSidebarWidth = WidthInspectorDef
	shell.state.Data.Tags = []domain.TagSummary{{ID: "1", Name: "golang", BookmarkCount: 1}}

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	// No pending suggestions are seeded, so this string can only come from the removed row.
	if tester.HasText(shell.t().T(i18n.SuggestionsPendingTitle)) {
		t.Errorf("the tags tab still draws the pending row; texts: %q", tester.Texts())
	}

	// 全部书签 is still the results scope title, so the question is where it is drawn: inside the
	// sidebar's column it would be the row that was removed.
	all, found := tester.Find(shell.t().T(i18n.SidebarAllBookmarks))
	if found && all.X < WidthSidebarDef {
		t.Errorf("the tags tab still draws the all-bookmarks row at x=%v; texts: %q", all.X, tester.Texts())
	}

	// And the first row of the tab is the tag itself.
	if _, found := tester.Find("golang"); !found {
		t.Errorf("the tag row is missing; texts: %q", tester.Texts())
	}
}

// TestTheFacetTabsKeepTheirLabelsOnOneLine is the reader's screenshot: 标 stacked over 签 and
// 作 over 者, the segmented control twice its height.
//
// The row carried an 8-point gap on every child, including both sides of its zero-width
// spacer. With an author condition on, the authors chip carries a count as well as the dot,
// and in the Chinese locale -- two full-width characters per label -- the row's content came
// to more than the 260-point minimum column allows. MyGo then shrank the labels, and a text
// with no line limit wraps when it is squeezed: the labels were drawn 30 points tall instead
// of 15.
//
// The assertions are the label's height and its natural width, both taken at the minimum
// column and compared with a comfortable one. Height alone would pass on a layout that
// ellipsised the labels; width alone would pass on one that stacked them.
func TestTheFacetTabsKeepTheirLabelsOnOneLine(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	// A full author list, so the authors tab carries the two-digit count (and the cap's "+")
	// that a small library does not: this is the state the reader's window was in.
	for i := 0; i < 60; i++ {
		seed(t, db, fmt.Sprintf("b%d", i), fmt.Sprintf("author%d", i), "a post", clockBase)
	}
	shell.SetLocale("zh-CN", "en")
	shell.Reload(ctx)

	// An author condition adds the dot to the authors tab.
	shell.state.ActiveAuthors = []string{"author0"}
	shell.Reload(ctx)

	measure := func(width float32) map[string]ui.Rect {
		shell.state.LeftSidebarWidth = width
		tester := ui.NewTester(shell.View, 1180, 760)
		tester.Frame()
		rects := map[string]ui.Rect{}
		for _, label := range []string{"标签", "作者"} {
			rect, found := tester.Find(label)
			if !found {
				t.Fatalf("the %q tab is not drawn; texts: %q", label, tester.Texts())
			}
			rects[label] = rect
		}
		return rects
	}

	roomy := measure(WidthSidebarMax)
	narrow := measure(WidthSidebarMin)

	for _, label := range []string{"标签", "作者"} {
		if narrow[label].H > roomy[label].H*1.2 {
			t.Errorf("the %q label is %.0f high at the minimum column and %.0f in a roomy one: it wrapped",
				label, narrow[label].H, roomy[label].H)
		}
		// Half a point of tolerance: a wrapped line is the defect; a hundredth of a point is
		// the layout engine rounding.
		if narrow[label].W < roomy[label].W-0.5 {
			t.Errorf("the %q label is squeezed to %.1f from %.1f at the minimum column",
				label, narrow[label].W, roomy[label].W)
		}
	}
}
