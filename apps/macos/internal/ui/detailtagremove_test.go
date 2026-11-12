package ui

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestEachTagPillNamesTheTagItRemoves covers a row of identical controls.
//
// The detail's tag row draws one 12-point remove control per pill, and every one of them was
// named "Remove": six pills read out as "ai agent, Remove, coding agent, Remove, ...", so the
// only way to tell which Remove belongs to which tag was to count. The source's pill is a
// single button whose text is the tag name (BookmarkInspector.tsx:414-420), so its name
// carries the tag.
func TestEachTagPillNamesTheTagItRemoves(t *testing.T) {
	shell, db := newShell(t)
	seedTagged(t, db, "1", "ai agent", "design")
	shell.SetLocale("en", "")
	shell.Reload(context.Background())
	shell.State().Selected = "1"
	shell.OpenSelectedDetail()

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	for _, tag := range []string{"ai agent", "design"} {
		if _, ok := tester.Find("Remove " + tag); !ok {
			t.Errorf("the %q pill's remove control is not named after its tag; texts: %q",
				tag, tester.Texts())
		}
	}
}
