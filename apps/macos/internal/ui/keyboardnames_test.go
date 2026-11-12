package ui

import (
	"context"
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/x-master/apps/macos/internal/domain"
	"github.com/taoquo/x-master/apps/macos/internal/i18n"
)

// Round 21 walks the keyboard and the focus path. The tester's Find answers with anything that
// carries a text or an accessibility label, so a focus stop with neither is invisible here --
// which is exactly how it feels to a reader who tabs onto it.

// TestTheViewTogglesTellTheKeyboardWhichIsWhich covers the toolbar's grid/list pair.
//
// Both are icon-only squares with a focus stop and no name. The source is the same
// (WorkspaceToolbar.tsx:140-158), so this is a native addition, not a replication: two
// unnamed squares in the tab order make the reader guess.
func TestTheViewTogglesTellTheKeyboardWhichIsWhich(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.SetLocale(i18n.EN, i18n.EN)
	shell.Reload(context.Background())

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	for _, key := range []i18n.Key{i18n.ViewGrid, i18n.ViewList} {
		if _, ok := tester.Find(shell.t().T(key)); !ok {
			t.Errorf("the %s toggle has no name; texts: %q", shell.t().T(key), tester.Texts())
		}
	}
}

// TestThePendingFilterChipNamesItsCount covers the tag library's filter pills.
//
// The count is a second text run inside the pill, so the pill's own name was the filter word
// alone. Naming the control with both means one announcement instead of a word followed by a
// stray number.
func TestThePendingFilterChipNamesItsCount(t *testing.T) {
	shell, _ := newShell(t)
	shell.SetLocale(i18n.EN, i18n.EN)
	shell.state.TagMode = true
	// The chip counts tags with something pending, not the suggestions themselves.
	shell.state.TagItems = []domain.TagLibraryItem{
		{TagID: "a", Name: "rust", PendingSuggestionCount: 3},
		{TagID: "b", Name: "design", PendingSuggestionCount: 1},
	}

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	label := shell.t().T(i18n.TagFilterPending)
	want := fmt.Sprintf("%s %d", label, 2)
	if _, ok := tester.Find(want); !ok {
		t.Errorf("the pending filter pill is named %q, not %q; texts: %q", label, want, tester.Texts())
	}
}
