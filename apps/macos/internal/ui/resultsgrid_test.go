package ui

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/x-master/apps/macos/internal/domain"
)

// The feed's two columns are a property of the pane, not of how many cards are in it.
//
// TestOneResultKeepsItsColumn covers a card that changed size when the list got short: the
// masonry split the cards in two and skipped the second column when the split landed on its
// end, so a single result -- which is what a filter often leaves -- grew to the width of the
// pane and its picture with it. The source's `lg:columns-2` keeps the column whether or not an
// item has reached it (extension.css, `lg:columns-2` on .options-results-grid), so the card a
// reader lands on after filtering looks like the cards they were just looking at.

// bodyWidth renders n bookmarks at 1400 points wide and returns the width of the post's body
// text, which wraps to its card's column.
func bodyWidth(t *testing.T, n int) float32 {
	t.Helper()
	shell, db := newShell(t)
	for _, id := range []string{"1", "2"}[:(n)] {
		seed(t, db, id, "ada", gridBody, clockBase)
	}
	shell.Reload(context.Background())
	shell.state.ViewMode = domain.ViewGrid

	tester := ui.NewTester(shell.View, 1400, 880)
	tester.Frame()

	if got := shell.state.ResultsColumns; got != 2 {
		t.Fatalf("%d bookmarks drew %d columns at 1400; the test needs the two-column feed", n, got)
	}
	rect, ok := tester.Find(gridBody)
	if !ok {
		t.Fatalf("%d bookmarks: the post's body is not drawn; texts: %q", n, tester.Texts())
	}
	return rect.W
}

// gridBody is long enough to wrap, so its text node takes the column's width rather than the
// width of a few words.
const gridBody = "a long post about compilers, the people who write them and the machines " +
	"they write them for, which is more words than one line of this card can hold."

func TestOneResultKeepsItsColumn(t *testing.T) {
	two := bodyWidth(t, 2)
	one := bodyWidth(t, 1)

	if one > two*1.05 {
		t.Errorf("a single result's body is %.0f points wide against %.0f with two results: "+
			"the one card the reader filtered down to fills the pane", one, two)
	}
	if one < two*0.95 {
		t.Errorf("a single result's body is %.0f points wide against %.0f with two results: "+
			"the card shrank instead of keeping its column", one, two)
	}
}

// TestTheColumnsSwitchAtMagpiesBreakpoint pins the column count to the window.
//
// The source's class is `columns-1 lg:columns-2`, and `lg` is a viewport breakpoint
// (extension.css:2020-2037): 1024 points of window, whatever the sidebar is doing. Round 22
// measured the three shipped widths and found the rule already held, but nothing failed if a
// later change moved it -- the single-result test only ever looks at 1400.
func TestTheColumnsSwitchAtMagpiesBreakpoint(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "a post", clockBase)
	seed(t, db, "2", "bea", "another post", clockBase)
	shell.Reload(context.Background())

	for _, tc := range []struct {
		width int
		want  int
	}{
		// Literals, not ResultsTwoColumnWidth: the number is the source's `lg` breakpoint, and a
		// test written against the constant would move with it and never notice.
		{1023, 1},
		{1024, 2},
		{1800, 2},
	} {
		tester := ui.NewTester(shell.View, tc.width, 880)
		tester.Frame()
		if got := shell.state.ResultsColumns; got != tc.want {
			t.Errorf("at %d points the feed drew %d columns, want %d", tc.width, got, tc.want)
		}
	}
}

// TestTheReadingColumnStopsGrowingAtTwelveHundred pins the other half of the width rule.
//
// magpie's view carries `padding-left: max(16px, calc(50% - 600px))` (app.css:609), so the
// page centres a 1200-point column and stops. Round 22 sampled a 1800-point capture and found
// the cards spanning 1198 points, which is that rule; this test makes the next widening keep
// it: a window twice as wide must not widen the text.
func TestTheReadingColumnStopsGrowingAtTwelveHundred(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", gridBody, clockBase)
	shell.Reload(context.Background())
	shell.state.ViewMode = domain.ViewGrid

	widths := make(map[int]float32, 2)
	for _, width := range []int{1800, 2400} {
		tester := ui.NewTester(shell.View, width, 880)
		tester.Frame()
		rect, ok := tester.Find(gridBody)
		if !ok {
			t.Fatalf("at %d points the post's body is not drawn; texts: %q", width, tester.Texts())
		}
		widths[width] = rect.W
	}

	wide, wider := widths[1800], widths[2400]
	if wider > wide*1.02 {
		t.Errorf("the body is %.0f points wide at 1800 and %.0f at 2400: the reading column "+
			"keeps growing instead of stopping at %d", wide, wider, ContentMaxWidth)
	}
}
