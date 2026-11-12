package ui

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/apps/macos/internal/domain"
	"github.com/taoquo/trove/apps/macos/internal/i18n"
)

// TestTheToolbarAnswersEveryClick walks the toolbar's controls the way a pointer meets
// them: through the label each one draws, not through the method behind it.
//
// The click-through sweep of round 34 found that a control can be wired and still look
// dead, so this round asks the opposite question of every control in the row: does the
// word on screen turn into the change it promises? Each step asserts the state the
// control is named for, and the row is re-rendered between steps so a label that only
// exists after a click is clicked in the frame that shows it. Round 35.
func TestTheToolbarAnswersEveryClick(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post about agents", clockBase)
	seed(t, db, "2", "grace", "a longer post", clockBase.Add(-1))
	seed(t, db, "3", "ada", "another post", clockBase.Add(-2))
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

	// Sort cycles through every order and the label follows it.
	seen := map[domain.SortOrder]bool{shell.state.Sort: true}
	for i := 0; i < 2; i++ {
		sortLabel := shell.currentSortLabel()
		click(sortLabel)
		if shell.state.Sort == (domain.SortOrder("")) {
			t.Fatal("the sort control cleared the order instead of cycling it")
		}
		if seen[shell.state.Sort] {
			t.Fatalf("clicking %q did not advance the order (still %q)", sortLabel, shell.state.Sort)
		}
		seen[shell.state.Sort] = true
		if !tester.HasText(shell.currentSortLabel()) {
			t.Errorf("the sort button does not say the new order %q; texts: %q",
				shell.currentSortLabel(), tester.Texts())
		}
	}
	if len(seen) < 3 {
		t.Errorf("the sort control offers %d orders, want the source's three", len(seen))
	}

	// The view toggles switch the layout, and the label stays put.
	click(shell.t().T(i18n.ViewList))
	if shell.state.ViewMode != domain.ViewList {
		t.Errorf("clicking %q left the view as %q", shell.t().T(i18n.ViewList), shell.state.ViewMode)
	}
	click(shell.t().T(i18n.ViewGrid))
	if shell.state.ViewMode != domain.ViewGrid {
		t.Errorf("clicking %q did not come back to the grid", shell.t().T(i18n.ViewGrid))
	}

	// The filter button opens its panel, and the panel's rows are the conditions.
	click(shell.t().T(i18n.ToolbarFilters))
	if !shell.state.FilterPopoverOpen {
		t.Fatalf("clicking %q did not open the filter panel", shell.t().T(i18n.ToolbarFilters))
	}
	click(shell.t().T(i18n.FilterHasMedia))
	if !shell.state.OnlyWithMedia {
		t.Errorf("clicking %q did not set the condition", shell.t().T(i18n.FilterHasMedia))
	}

	// Close the panel, then take the condition off where the reader sees it: the chip in
	// the toolbar, whose whole face removes the condition it names.
	click(shell.t().T(i18n.ToolbarFilters))
	if shell.state.FilterPopoverOpen {
		t.Fatalf("clicking %q again did not close the filter panel", shell.t().T(i18n.ToolbarFilters))
	}
	click(shell.t().T(i18n.FilterHasMedia))
	if shell.state.OnlyWithMedia {
		t.Errorf("pressing the condition chip %q did not remove it", shell.t().T(i18n.FilterHasMedia))
	}

	// The clear button in the search field is only drawn with a query, and leaves nothing
	// of the search behind.
	shell.state.Query = "agents"
	shell.Reload(ctx)
	tester.Frame()
	click(shell.t().T(i18n.ActionClear))
	if shell.state.Query != "" {
		t.Errorf("the clear button left the query %q", shell.state.Query)
	}
}
