package ui

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/apps/macos/internal/i18n"
)

// TestTheConditionChipIsOneButton covers the chip a pointer meets first.
//
// The source draws each active condition as one button whose whole face removes it --
// the label and the x are inside the same <button onClick={() => clearRefinement(...)}>
// (WorkspaceToolbar.tsx:169-177). This build made only the 16-point x pressable, so
// clicking the word a reader is looking at did nothing, and the chip's own hover ground
// promised otherwise. Round 35.
func TestTheConditionChipIsOneButton(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	shell.SetLocale(i18n.EN, i18n.EN)
	seed(t, db, "1", "ada", "a post", clockBase)
	seedTagOnly(t, db, "t-rust", "rust")
	if _, err := shell.service.AddTags(ctx, []string{"1"}, []string{"rust"}); err != nil {
		t.Fatalf("attach tag: %v", err)
	}
	shell.Reload(ctx)

	// A flag condition: the word alone has to remove it.
	shell.state.OnlyWithMedia = true
	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	if err := tester.Click(shell.t().T(i18n.FilterHasMedia)); err != nil {
		t.Fatalf("clicking the condition %q: %v", shell.t().T(i18n.FilterHasMedia), err)
	}
	tester.Frame()
	if shell.state.OnlyWithMedia {
		t.Error("pressing the condition's own word did nothing: only its x removes it")
	}

	// A facet condition: the x still removes it exactly once. Remove toggles for facets,
	// so a press that fired both the chip and the x would leave the tag in place.
	shell.state.ActiveTagIDs = []string{"t-rust"}
	tester.Frame()
	if err := tester.Click(shell.t().Tf(i18n.ActionRemoveNamed, "rust")); err != nil {
		t.Fatalf("clicking the chip's remove control: %v", err)
	}
	tester.Frame()
	if len(shell.state.ActiveTagIDs) != 0 {
		t.Errorf("the remove control left the tag filter %v", shell.state.ActiveTagIDs)
	}
}
