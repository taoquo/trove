package ui

import (
	"context"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/taoquo/trove/internal/i18n"
)

// TestTheSettingsPanelDrawsItsSections is the content half of verifying a surface a screenshot
// cannot reach.
//
// TROVE_CAPTURE_STATE=settings produces a picture byte-identical to the plain window, because a
// panel's visibility comes from EntranceProgress and root.Animate does not advance on the capture
// path. So the panel's appearance cannot be photographed -- and its contents can still be read,
// which is what ui.Tester does: it renders the frame and returns the text.
//
// This asserts the sections rather than every control: the point is that opening the panel
// reaches the code that draws it, which no other test does.
func TestTheSettingsPanelDrawsItsSections(t *testing.T) {
	shell, db := newShell(t)
	shell.Reload(context.Background())
	seed(t, db, "1", "ada", "a post", clockBase)

	shell.State().SettingsOpen = true
	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	texts := tester.Texts()

	// The section names are the tabs, so all of them are on screen whatever tab is chosen. The
	// labels are uppercase in magpie's tab row, which is why the comparison is against the
	// uppercased copy (app.css:415-449).
	for _, key := range []i18n.Key{
		i18n.SettingsSectionGeneral,
		i18n.SettingsSectionSync,
		i18n.SettingsSectionAutoTag,
		i18n.SettingsSectionDataSafety,
		i18n.SettingsSectionListsViews,
	} {
		want := strings.ToUpper(shell.t().T(key))
		if want == "" {
			t.Fatalf("%s has no copy", key)
		}
		if !containsText(texts, want) {
			t.Errorf("the settings tabs do not show %q (%s); texts: %q", want, key, texts)
		}
	}

	// The page names itself beside its tabs. The way back is an icon control now, as
	// magpie's heads carry their tools; its words live in the control's tooltip and
	// accessible name, which Tester's text list does not report (it lists text runs).
	if !containsText(texts, shell.t().T(i18n.SettingsTitle)) {
		t.Errorf("the page has no title; texts: %q", texts)
	}
	if shell.t().T(i18n.SettingsBack) == "" {
		t.Error("the way back has no name")
	}
	if shell.t().T(i18n.SettingsSearchPlaceholder) == "" {
		t.Error("the find field has no copy")
	}

	// The default tab is 通用, and only its content is drawn -- the sheet shows one section at a
	// time. The two general fields are language then theme; the second is "Theme", not
	// "Appearance".
	for _, key := range []i18n.Key{i18n.SettingsLanguage, i18n.ThemeLabel} {
		if !containsText(texts, shell.t().T(key)) {
			t.Errorf("the general tab is missing %q; texts: %q", shell.t().T(key), texts)
		}
	}
	if containsText(texts, shell.t().T(i18n.SettingsGroupAutoTagBasics)) {
		t.Error("the auto-tagging section is drawn while the general tab is selected")
	}

	// Choosing the auto-tagging section draws the signpost, not the controls: they live in tag
	// management, and a second copy here would be two homes for one setting.
	shell.state.SettingsSection = SettingsSectionAutoTag
	tester.Frame()
	if !containsText(tester.Texts(), shell.t().T(i18n.SettingsAutoTagMoved)) {
		t.Errorf("the auto-tagging tab does not say where the controls went; texts: %q", tester.Texts())
	}
	if containsText(tester.Texts(), strings.ToUpper(shell.t().T(i18n.SettingsGroupAutoTagBasics))) {
		t.Errorf("the auto-tagging controls are still drawn in Settings; texts: %q", tester.Texts())
	}

	// There is deliberately no "and it is not drawn with the panel closed" half. Tester
	// collects an element's text whatever its opacity, and a panel's opacity is what
	// EntranceProgress drives -- so the title reads the same at 0 as at 1. Visibility is not
	// expressible through Texts(), which is the same limitation that keeps the capture from
	// photographing the panel in the first place.
}

// TestTheTagLibraryDrawsItsTitle covers the second surface with no screenshot.
//
// TagMode is in-memory state like SettingsOpen, so the same limitation applies: the panel is
// in the frame, and whether the frame is captured does not depend on it.
func TestTheTagLibraryDrawsItsTitle(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(ctx)

	shell.State().TagMode = true
	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	texts := tester.Texts()

	// The page's heading is the page's name. It used to be the overline "标签库" above "管理标签";
	// the header now names the page once, because the sidebar already names the section (S27).
	want := shell.t().T(i18n.TagManageLabel)
	if !containsText(texts, want) {
		t.Errorf("the tag library does not show %q; texts: %q", want, texts)
	}
	if _, ok := tester.Find(shell.t().T(i18n.TagExitManagement)); !ok {
		t.Error("the header no longer offers the way back to the bookmarks")
	}

	// A second half that asserted the title is absent with TagMode off was removed: it was a
	// t.Log, which does not fail, so it read as coverage while binding nothing. The same
	// applies to the loading test's negative -- Tester's Texts() reports what was built, and a
	// surface that is built regardless of the state guarding it cannot be told apart this way.
}

// TestTheLoadingStateDrawsCardsNotText covers the third.
//
// The stylesheet's loading state is a skeleton -- six placeholder cards -- and an earlier
// version drew a line of text. Text is what a person reads when the app says "loading"; cards
// are what the app shows.
func TestTheLoadingStateDrawsCardsNotText(t *testing.T) {
	shell, _ := newShell(t)
	shell.State().Loaded = false

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	texts := tester.Texts()

	// The skeleton draws no text at all, so there is nothing here to assert positively -- and a
	// card count needs a rect, which Tester's Find returns empty for this tree. That is the
	// limitation of the tester, and it is why the loading state is pinned by
	// spec_test.go's shape constants instead of by looking at a frame.
	if len(texts) == 0 {
		t.Error("the frame rendered nothing at all, so this test proves nothing")
	}
}

// TestTheVocabularyReviewPanelDrawsItsText covers the third animated surface.
//
// This panel and suggestionsPanel had never been drawn by any test, because reaching them needs
// ReviewOpen or SuggestionsOpen set, and nothing set either. The panel cannot be photographed either -- EntranceProgress gates it --
// so Tester's text is the only evidence available.
func TestTheVocabularyReviewPanelDrawsItsText(t *testing.T) {
	shell, _ := newShell(t)
	shell.Reload(context.Background())
	shell.State().ReviewOpen = true

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	texts := tester.Texts()

	for _, key := range []i18n.Key{i18n.ReviewTitle, i18n.ReviewDescription} {
		want := shell.t().T(key)
		if want == "" {
			t.Fatalf("%s has no copy", key)
		}
		if !containsText(texts, want) {
			t.Errorf("the review panel does not show %q (%s); texts: %q", want, key, texts)
		}
	}
}

// TestTheSuggestionsPanelDrawsItsTitle covers the fourth.
func TestTheSuggestionsPanelDrawsItsTitle(t *testing.T) {
	shell, _ := newShell(t)
	shell.Reload(context.Background())
	shell.State().SuggestionsOpen = true

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	texts := tester.Texts()

	want := shell.t().T(i18n.SuggestionsTitle)
	if want == "" {
		t.Fatal("SuggestionsTitle has no copy")
	}
	if !containsText(texts, want) {
		t.Errorf("the suggestions panel does not show %q; texts: %q", want, texts)
	}
}

// TestTheSettingsPageKeepsMagpiesPagePadding pins the settings page's own top and bottom
// padding: 8.
//
// magpie's settings view takes 8 at the top (the side-nav override, app.css:511) and its
// bottom is the 8 every .view has (app.css:602). A page that adds 24 under its last card
// ends in a band of empty ground no other page has, which is exactly the kind of drift a
// screenshot review keeps finding late.
func TestTheSettingsPageKeepsMagpiesPagePadding(t *testing.T) {
	if SettingsPagePadTop != 8 {
		t.Errorf("SettingsPagePadTop = %v, want 8 (magpie #view-settings padding-top, app.css:511)", SettingsPagePadTop)
	}
	if SettingsPagePadBottom != 8 {
		t.Errorf("SettingsPagePadBottom = %v, want 8 (magpie .view padding-bottom, app.css:602)", SettingsPagePadBottom)
	}
}
