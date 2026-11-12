package ui

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/apps/macos/internal/i18n"
)

// TestTheGraphPageHasAWayBackToTheBookmarks pins the exit the full-page graph was missing.
//
// GraphMode replaces the results pane and takes the toolbar's graph switch with it, so the
// only way out was Escape -- a key nothing on the page mentions. Settings and tag management
// both answer this with the same back square in the page's head; the graph page now does too.
func TestTheGraphPageHasAWayBackToTheBookmarks(t *testing.T) {
	shell, _ := newShell(t)
	shell.SetLocale(i18n.EN, i18n.EN)
	ctx := context.Background()
	shell.Reload(ctx)
	shell.state.GraphMode = true

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	if _, ok := tester.Find(shell.t().T(i18n.TagExitManagement)); !ok {
		t.Fatalf("the graph page has no way back; texts: %q", tester.Texts())
	}

	if !tabUntil(tester, shell.t().T(i18n.TagExitManagement)) {
		t.Fatal("Tab never reached the graph's back square")
	}
	tester.Key(0, ui.KeyEnter)
	tester.Frame()

	if shell.state.GraphMode {
		t.Error("Enter on the back square left the graph open")
	}
}
