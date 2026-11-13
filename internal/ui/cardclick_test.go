package ui

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/internal/i18n"
)

// TestTheCardAnswersEveryClick walks a result card the way a pointer meets it: the
// selection box, the card itself, and the context menu behind a right click.
//
// The card is a clickable surface with interactive children, which is where click routing
// goes wrong: the box has to take its own click without opening the post, and the card click
// has to open the detail.
func TestTheCardAnswersEveryClick(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	shell.SetLocale(i18n.EN, i18n.EN)
	seed(t, db, "1", "ada", "first post", clockBase)
	seed(t, db, "2", "grace", "second post", clockBase.Add(-1))
	shell.Reload(ctx)

	tester := ui.NewTester(shell.View, 1180, 900)
	click := func(label string) {
		t.Helper()
		tester.Frame()
		if err := tester.Click(label); err != nil {
			t.Fatalf("clicking %q: %v (texts: %q)", label, err, tester.Texts())
		}
		tester.Frame()
	}

	// The box is named after the post's author: a column of identical "Select bookmark" boxes
	// tells a screen reader which one is which no better than no name at all.
	box := shell.t().Tf(i18n.CardSelectBookmarkFor, "ada")
	if _, ok := tester.Find(box); !ok {
		t.Fatalf("the card's box is not named after its author; texts: %q", tester.Texts())
	}

	// The selection box toggles the set a bulk action runs on, and does not open the post.
	click(box)
	if !shell.state.SelectedIDs["1"] {
		t.Fatalf("the card's box did not select the first post; selected=%v", shell.state.SelectedIDs)
	}
	if shell.state.DetailOpen {
		t.Error("ticking the box also opened the detail: the click did not stop at the box")
	}

	// The card's own "Open on X" acts on the row without also opening the detail.
	opened := ""
	shell.UseExternalOpener(func(url string) error { opened = url; return nil })
	click(shell.t().T(i18n.OpenOnX))
	if opened == "" {
		t.Error("the card's open action did not reach the browser")
	}
	if shell.state.DetailOpen {
		t.Error("the card's open action also opened the detail: the click did not stop at the button")
	}

	// The card opens its detail.
	click("first post")
	if !shell.state.DetailOpen || shell.state.Selected != "1" {
		t.Errorf("clicking the card left detail open=%v selected=%q", shell.state.DetailOpen, shell.state.Selected)
	}
	shell.state.DetailOpen = false
	tester.Frame()

	// The context menu is the card's second surface. Every enabled item has to answer.
	if err := tester.RightClick("second post"); err != nil {
		t.Fatalf("right-clicking the card: %v", err)
	}
	menu := tester.Menu()
	want := []i18n.Key{i18n.ActionOpenBookmark, i18n.OpenOnX, i18n.ActionCopyText,
		i18n.ActionCopyLink, i18n.ActionAddToSelection}
	for _, key := range want {
		found := false
		for _, item := range menu {
			if item == shell.t().T(key) {
				found = true
			}
		}
		if !found {
			t.Errorf("the card's context menu has no %q; menu: %q", shell.t().T(key), menu)
		}
	}

	if err := tester.ChooseMenuItem(shell.t().T(i18n.ActionAddToSelection)); err != nil {
		t.Fatalf("choosing %q: %v", shell.t().T(i18n.ActionAddToSelection), err)
	}
	tester.Frame()
	if !shell.state.SelectedIDs["2"] {
		t.Errorf("the context menu's add-to-selection did not select the post; selected=%v",
			shell.state.SelectedIDs)
	}

	// Copy link is the item that used to have no way to say it worked.
	if err := tester.RightClick("second post"); err != nil {
		t.Fatalf("second right click: %v", err)
	}
	if err := tester.ChooseMenuItem(shell.t().T(i18n.ActionCopyLink)); err != nil {
		t.Fatalf("choosing %q: %v", shell.t().T(i18n.ActionCopyLink), err)
	}
	tester.Frame()
	if shell.ToastText() == "" {
		t.Error("copying the link said nothing, so the reader cannot tell it worked")
	}
}
