package ui

import (
	"context"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/internal/i18n"
)

// TestBrandLogoDecodes covers the embedded asset that the sidebar draws.
//
// Embedding it means a typo in the path or a corrupt file fails here rather than at
// runtime, where it would show as a blank square beside the title.
func TestBrandLogoDecodes(t *testing.T) {
	mark := BrandLogo()
	if mark == nil {
		t.Fatal("the brand logo did not decode")
	}

	width, height := mark.Size()
	if width <= 0 || height <= 0 {
		t.Fatalf("the logo is %vx%v", width, height)
	}
	// The asset is 128 square, and the sidebar scales it to 38.
	if width != height {
		t.Errorf("the logo is %vx%v, want square", width, height)
	}
}

// TestSidebarShowsTheBrandRow covers the wiring: a decoded logo that nothing draws is
// the gap this shell has had before.
func TestSidebarShowsTheBrandRow(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(context.Background())

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	// The brand row's title.
	if !tester.HasText("Bookmarks") {
		t.Errorf("the sidebar is missing its brand title; texts: %q", tester.Texts())
	}

	// And in the other locale, which is also the title's source.
	shell.SetLocale("zh-CN")
	tester.Frame()
	if !tester.HasText("书签") {
		t.Errorf("the Chinese sidebar is missing its brand title; texts: %q", tester.Texts())
	}
}

// TestBrandLogoSizeMatchesTheWordmark pins the restyle's two values.
//
// This app's mark is a filled square rather than a line bird, so it sits at 22 with a
// 6-point radius: the size at which it reads level with the 13.5px wordmark beside it.
func TestBrandLogoSizeMatchesTheWordmark(t *testing.T) {
	if BrandLogoSize != 22 {
		t.Errorf("the mark is %d DIPs, want 22 beside the 13.5px wordmark", BrandLogoSize)
	}
	if BrandLogoRadius != 6 {
		t.Errorf("the mark's radius is %d, want the row's 6", BrandLogoRadius)
	}
}

// TestIconChipKeepsItsLabel covers what the wrapper must not lose.
//
// The shell's pattern has been "both ends built, nothing between", so a helper that
// renders only its label would look correct in a screenshot and be missing the icon.
func TestIconChipKeepsItsLabel(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "a post", clockBase)
	// One tag: the toolbar -- and with it the Graph control -- is not drawn for an empty
	// library (S27), since there is nothing to search, order or filter.
	seedTagOnly(t, db, "a", "rust")
	shell.Reload(context.Background())
	shell.ReloadTagLibrary(context.Background())

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	// The Graph control carries its name, and the name is visible.
	//
	// Tester.Texts() cannot see a tooltip, so a tooltip-only control would need its call site read.
	// This control is an IconChip in the tags view -- an icon and a word -- so the name is on
	// screen and the assertion can be about the rendered frame rather than about a string in the
	// source, which is the stronger question.
	shell.ToggleTagMode(context.Background())
	tester.Frame()
	if !tester.HasText(shell.t().T(i18n.ToolbarGraph)) {
		t.Errorf("the Graph control has no visible name in the tags view; texts: %q", tester.Texts())
	}
	shell.ToggleTagMode(context.Background())
	tester.Frame()
	if !strings.Contains(packageSource(t), "Tooltip(tooltip)") {
		t.Error("IconOnly drops the name it is given, so no icon-only button is labelled")
	}
	// The sync command is an icon beside the state it changes now, so there is no word to
	// find; what the foot prints is the last sync itself.
	if !tester.HasText(shell.t().T(i18n.LastSyncNever)) {
		t.Errorf("the sidebar foot has no last-sync line; texts: %q", tester.Texts())
	}

	// "Delete" appears once a bookmark is in the batch selection, because that is when the
	// bulk bar renders: the toolbar carries no delete, and the bulk bar stays empty until
	// something is selected. State.Selected is the inspector's bookmark and does not open
	// the bar -- SelectedIDs is the batch one.
	shell.State().SelectedIDs = map[string]bool{"1": true}
	tester.Frame()
	if !tester.HasText("Delete") {
		t.Errorf("the bulk bar lost its Delete label; texts: %q", tester.Texts())
	}
}
