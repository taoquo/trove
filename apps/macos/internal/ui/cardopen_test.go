package ui

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/apps/macos/internal/i18n"
)

// TestTheCardOpensItsOwnBookmarkOnX covers the card's only working action.
//
// The action looked up s.state.Selected instead of the card it was drawn on, so with
// nothing selected it said "No link", and after opening one card's detail every other
// card opened that card's URL. The context menu already acts on the row under the
// pointer; the button has to do the same.
func TestTheCardOpensItsOwnBookmarkOnX(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(ctx)

	var opened []string
	shell.UseExternalOpener(func(url string) error {
		opened = append(opened, url)
		return nil
	})

	// Another bookmark is selected, as it is after opening a detail and closing it.
	shell.State().Selected = "2"

	tester := ui.NewTester(shell.View, 1180, 900)
	tester.Frame()

	// The first card in reading order carries the first seed's URL. Its action must open
	// that URL, not the selected bookmark's.
	if err := tester.Click(shell.t().T(i18n.OpenOnX)); err != nil {
		t.Fatalf("the card has no %q control: %v", shell.t().T(i18n.OpenOnX), err)
	}

	if len(opened) != 1 {
		t.Fatalf("opened %v, want exactly the card's own URL", opened)
	}
	if opened[0] != "https://x.com/ada/status/1" {
		t.Errorf("opened %q, want the card's own URL", opened[0])
	}
}
