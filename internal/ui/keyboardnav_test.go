package ui

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/internal/i18n"
)

// TestEscapeClosesTheFilterPopover covers the popover's own key.
//
// The panel drops from the toolbar and covers the sidebar's first rows, so a reader who
// opened it by mistake needs the key every popover answers. MyGo's overlay takes Escape
// before the window does; this pins that the popover is one of those overlays rather than
// a box drawn inside the toolbar, which is the shape it would silently become.
func TestEscapeClosesTheFilterPopover(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	shell.SetLocale(i18n.EN)
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(ctx)

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	shell.ToggleFilterPopover()
	tester.Frame()
	if !tester.HasText(shell.t().T(i18n.FilterConditions)) {
		t.Fatalf("the popover did not open; texts: %q", tester.Texts())
	}

	tester.Key(0, ui.KeyEscape)
	tester.Frame()

	if shell.state.FilterPopoverOpen {
		t.Error("Escape left the filter popover open")
	}
	if tester.HasText(shell.t().T(i18n.FilterConditions)) {
		t.Errorf("the popover is still drawn after Escape; texts: %q", tester.Texts())
	}
	// Escape belongs to the panel on top, not to the list behind it: the search it would
	// otherwise clear stays as it was.
	if shell.state.Data.Query.Filter.Query != "" {
		t.Error("Escape cleared the search behind the popover")
	}
}

// TestUpAndDownWalkTheTagRows covers the sidebar's own two keys.
//
// Tab reaches every tag row and Enter presses the focused one, but moving from one tag to
// the next meant pressing Tab through the row's own count, the next search field and
// whatever control the tree holds between two rows. Up and Down walk the rows directly.
func TestUpAndDownWalkTheTagRows(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	shell.SetLocale(i18n.EN)
	for _, tag := range []struct{ id, name string }{{"t1", "alpha"}, {"t2", "bravo"}, {"t3", "charlie"}} {
		seedTagOnly(t, db, tag.id, tag.name)
	}
	shell.Reload(ctx)

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	if !tabUntil(tester, "alpha") {
		t.Fatalf("Tab never reached the first tag; texts: %q", tester.Texts())
	}

	tester.Key(0, ui.KeyDown)
	tester.Frame()
	tester.Frame()
	if !tester.Focused("bravo") {
		t.Fatalf("Down did not move to the second tag; texts: %q", tester.Texts())
	}

	tester.Key(0, ui.KeyDown)
	tester.Frame()
	tester.Frame()
	if !tester.Focused("charlie") {
		t.Fatalf("Down did not move to the third tag; texts: %q", tester.Texts())
	}

	// The end of the list is the end: Down there does nothing rather than wrapping.
	tester.Key(0, ui.KeyDown)
	tester.Frame()
	tester.Frame()
	if !tester.Focused("charlie") {
		t.Error("Down at the last tag left the list instead of staying put")
	}

	tester.Key(0, ui.KeyUp)
	tester.Frame()
	tester.Frame()
	if !tester.Focused("bravo") {
		t.Error("Up did not move back to the second tag")
	}

	// Enter presses the row the keyboard is on, as it would with the pointer.
	tester.Key(0, ui.KeyEnter)
	tester.Frame()
	if got := shell.state.Data.Query.Filter.TagIDs; !slices.Contains(got, "t2") {
		t.Errorf("Enter on the second tag filtered by %v, want t2", got)
	}
}

// TestUpAndDownWalkTheAuthorRows is the same walk on the authors tab.
func TestUpAndDownWalkTheAuthorRows(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	shell.SetLocale(i18n.EN)
	seed(t, db, "1", "ada", "a post", clockBase)
	seed(t, db, "2", "grace", "another post", clockBase.Add(1))
	shell.Reload(ctx)
	shell.state.SidebarFacetTab = sidebarTabAuthors

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	if !tabUntil(tester, "ada") {
		t.Fatalf("Tab never reached the first author; texts: %q", tester.Texts())
	}
	tester.Key(0, ui.KeyDown)
	tester.Frame()
	tester.Frame()
	if !tester.Focused("grace") {
		t.Fatalf("Down did not move to the second author; texts: %q", tester.Texts())
	}
	tester.Key(0, ui.KeyUp)
	tester.Frame()
	tester.Frame()
	if !tester.Focused("ada") {
		t.Error("Up did not move back to the first author")
	}
}

// TestTheResultsListIsOneTabStop covers what Tab does inside a page of cards.
//
// Every card used to be Focusable, and so was the card's one action button: a page of 80
// cards was 160 stops between the toolbar and the status line, and the only way past them
// was to press Tab 160 times. The list is one stop now -- arrows move between cards and
// Enter opens the selected one -- so Tab leaving the list lands somewhere else.
func TestTheResultsListIsOneTabStop(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	shell.SetLocale(i18n.EN)
	for i, id := range []string{"1", "2", "3", "4", "5", "6"} {
		seed(t, db, id, "ada", "post number "+id, clockBase.Add(time.Duration(i)))
	}
	shell.Reload(ctx)

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	stop := shell.t().T(i18n.ResultsListLabel)
	if !tabUntil(tester, stop) {
		t.Fatalf("Tab never reached the results list; texts: %q", tester.Texts())
	}

	// Tab moves on from the list, not into a card: no card, and nothing inside one, is the
	// next stop.
	tester.Key(0, ui.KeyTab)
	tester.Frame()
	if tester.Focused(stop) {
		t.Error("Tab did not leave the results list")
	}
	for _, card := range []string{"post number 1", "post number 6"} {
		if tester.Focused(card) {
			t.Errorf("Tab stopped inside a card (%q); the list is meant to be one stop", card)
		}
	}

	// The arrows still pick a card, which is how a card is reached without Tab.
	shell.state.Selected = ""
	shell.selectResultIndex(0)
	tester.Frame()
	first := shell.state.Selected
	tester.Key(0, ui.KeyDown)
	tester.Frame()
	if shell.state.Selected == first {
		t.Error("Down did not move the outline to the next card")
	}
}

// TestTheCardActionStaysReachableFromTheKeyboard covers what the removed stop promised.
//
// The card's "open on X" button stopped being a Tab stop; the same action is a labelled
// control in the detail, which the keyboard reaches with Enter on the selected card.
func TestTheCardActionStaysReachableFromTheKeyboard(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	shell.SetLocale(i18n.EN)
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(ctx)

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	shell.state.Selected = "1"
	shell.OpenSelectedDetail()
	tester.Frame()

	if !tabUntil(tester, shell.t().T(i18n.OpenOnX)) {
		t.Errorf("the detail does not reach %q from the keyboard; texts: %q",
			shell.t().T(i18n.OpenOnX), tester.Texts())
	}
}
