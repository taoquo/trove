package ui

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	engine "github.com/taoquo/trove/internal/autotag"
	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/i18n"
	"github.com/taoquo/trove/internal/usecase/suggestions"
)

// The auto-tagging page of tag management.

// openAutoTagPage enters the tag library on its auto-tagging tab, which is the only way in:
// Settings used to carry a signpost to it and no longer does.
func openAutoTagPage(shell *Shell) {
	shell.closePanels()
	shell.state.TagMode = true
	shell.state.GraphMode = false
	shell.selectTagLibraryView(TagViewAutoTag)
}

// TestTheAutoTagPageIsReachedFromTheTable is the move's acceptance criterion: the controls
// that were in Settings are now one click from the tags they produce, and the table's own
// controls are gone while the other page is on screen.
func TestTheAutoTagPageIsReachedFromTheTable(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(ctx)
	shell.ToggleTagMode(ctx)

	tester := ui.NewTester(shell.View, 1180, 900)
	tester.Frame()

	if tester.HasText(shell.t().T(i18n.ActionRunNow)) {
		t.Fatalf("the run control is drawn on the tags page")
	}
	// The switch is a warm tab, so it reads uppercase, as the settings page's tabs do.
	switchLabel := strings.ToUpper(shell.t().T(i18n.SettingsSectionAutoTag))
	if !tester.HasText(switchLabel) {
		t.Fatalf("the auto-tagging page has no switch; texts: %q", tester.Texts())
	}

	if err := tester.Click(switchLabel); err != nil {
		t.Fatalf("the switch does not open the auto-tagging page: %v", err)
	}
	tester.Frame()

	if got := shell.tagView(); got != TagViewAutoTag {
		t.Fatalf("the page is %q after the switch, want %q", got, TagViewAutoTag)
	}
	for _, key := range []i18n.Key{
		i18n.ActionRunNow,
		i18n.SettingsAPIKey,
	} {
		if !tester.HasText(shell.t().T(key)) {
			t.Errorf("the auto-tagging page is missing %q; texts: %q", shell.t().T(key), tester.Texts())
		}
	}
	// The three parts of the page, each under its muted uppercase label, and their
	// one-line explanations under the label.
	for _, key := range []i18n.Key{
		i18n.SettingsGroupAutoTagBasics,
		i18n.AutoTagPacing,
		i18n.SettingsGroupAutoTagLlm,
	} {
		want := strings.ToUpper(shell.t().T(key))
		if !tester.HasText(want) {
			t.Errorf("the auto-tagging page is missing %q; texts: %q", want, tester.Texts())
		}
	}
	for _, key := range []i18n.Key{
		i18n.AutoTagRunHint,
		i18n.AutoTagBasicsHint,
		i18n.AutoTagPacingHint,
		i18n.AutoTagModelHint,
	} {
		want := shell.t().T(key)
		if !tester.HasText(want) {
			t.Errorf("the auto-tagging page is missing %q; texts: %q", want, tester.Texts())
		}
	}
	// The two halves of 基础规则 are separate groups: when a pass starts, and where its
	// suggestions come from.
	if want := strings.ToUpper(shell.t().T(i18n.AutoTagSources)); !tester.HasText(want) {
		t.Errorf("the auto-tagging page is missing %q; texts: %q", want, tester.Texts())
	}
	// The other page's controls are not drawn under it, and the page carries the way out of
	// the mode that the table's toolbar would otherwise hold.
	if tester.HasText(shell.t().T(i18n.TagSearchPlaceholder)) {
		t.Errorf("the tag table's search is drawn on the auto-tagging page")
	}
	if !tester.HasText(shell.t().T(i18n.TagExitManagement)) {
		t.Errorf("the auto-tagging page has no way back to the bookmarks")
	}
}

// TestTheTagsPageIsTheWayIn keeps the screen's purpose ahead of its last state: opening tag
// management must land on the table even if the reader left from the auto-tagging page.
func TestTheTagsPageIsTheWayIn(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(ctx)

	shell.ToggleTagMode(ctx)
	if got := shell.tagView(); got != TagViewTags {
		t.Fatalf("the first page is %q, want the table", got)
	}
	shell.state.TagView = TagViewAutoTag
	shell.ToggleTagMode(ctx) // leave
	shell.ToggleTagMode(ctx) // come back
	if got := shell.tagView(); got != TagViewTags {
		t.Errorf("tag management reopened on %q, want the table", got)
	}
}

// TestSwitchingPagesKeepsTheTableState is the difference between a view switch and a reload:
// going to the other page and back must not re-ask the table's questions.
func TestSwitchingPagesKeepsTheTableState(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(ctx)
	shell.ToggleTagMode(ctx)

	shell.state.TagQuery = "go"
	shell.state.TagFilter = domain.TagFilterOrphan
	shell.state.TagSort = domain.TagSortNameAsc
	shell.state.TagFocus = "some-tag"
	shell.state.TagSelectedIDs = map[string]bool{"some-tag": true}

	shell.selectTagLibraryView(TagViewAutoTag)
	shell.selectTagLibraryView(TagViewTags)

	if shell.state.TagQuery != "go" || shell.state.TagFilter != domain.TagFilterOrphan ||
		shell.state.TagSort != domain.TagSortNameAsc || shell.state.TagFocus != "some-tag" ||
		!shell.state.TagSelectedIDs["some-tag"] {
		t.Errorf("the table's state changed across the switch: query=%q filter=%q sort=%q focus=%q selected=%v",
			shell.state.TagQuery, shell.state.TagFilter, shell.state.TagSort,
			shell.state.TagFocus, shell.state.TagSelectedIDs)
	}
}

// TestTheAutoTagPageOffersTheSuggestionsItProduced closes the loop the old placement left
// open: the result's "N pending" used to be a number with no way to act on it.
func TestTheAutoTagPageOffersTheSuggestionsItProduced(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(ctx)
	shell.state.TagMode = true
	shell.state.TagView = TagViewAutoTag
	shell.state.PendingSuggestions = []suggestions.PendingRow{
		{Suggestion: domain.TagSuggestion{ID: "s1"}},
		{Suggestion: domain.TagSuggestion{ID: "s2"}},
	}

	tester := ui.NewTester(shell.View, 1180, 900)
	tester.Frame()

	want := fmt.Sprintf("2 %s", shell.t().T(i18n.SuggestionsPendingTitle))
	if !tester.HasText(want) {
		t.Fatalf("the page does not offer the two pending suggestions; texts: %q", tester.Texts())
	}
	if err := tester.Click(want); err != nil {
		t.Fatalf("the pending suggestions have no way in: %v", err)
	}
	tester.Frame()
	if !shell.state.SuggestionsOpen {
		t.Errorf("clicking the pending count did not open the suggestions panel")
	}
}

// TestTheAutoTagPageCanStopARun keeps the one state that needs a way out: a run in flight.
func TestTheAutoTagPageCanStopARun(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(ctx)
	shell.state.TagMode = true
	shell.state.TagView = TagViewAutoTag
	shell.state.AutoTagRunning = true

	tester := ui.NewTester(shell.View, 1180, 900)
	tester.Frame()

	if !tester.HasText(shell.t().T(i18n.ActionStopRun)) {
		t.Errorf("a running pass has no stop control on its page; texts: %q", tester.Texts())
	}
}

// TestTheViewSwitchLivesInTheHeader is the density claim made checkable: the switch is in the
// header band, above the table's toolbar, so it costs the table no row of its own.
func TestTheViewSwitchLivesInTheHeader(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post", clockBase)
	// A tag, so the toolbar the switch is measured against is drawn: an empty library hides it
	// (S27).
	seedTagOnly(t, db, "a", "rust")
	shell.Reload(ctx)
	shell.ReloadTagLibrary(ctx)
	shell.ToggleTagMode(ctx)

	tester := ui.NewTester(shell.View, 1180, 900)
	tester.Frame()

	view, ok := tester.Find(strings.ToUpper(shell.t().T(i18n.SettingsSectionAutoTag)))
	if !ok {
		t.Fatalf("the view switch is not on screen")
	}
	// 关系图 is on the toolbar's first row. The switch has to stand above it: that is what
	// "in the header" means, and it is the property that keeps the table's viewport.
	toolbar, ok := tester.Find(shell.t().T(i18n.ToolbarGraph))
	if !ok {
		t.Fatalf("the table's toolbar is not on screen")
	}
	if view.Y+view.H > toolbar.Y {
		t.Errorf("the switch (y=%.0f..%.0f) reaches into the toolbar row (y=%.0f)",
			view.Y, view.Y+view.H, toolbar.Y)
	}
}

// TestTheAutoTagPageReadsTheStoredConfiguration guards the move's sharpest edge: the page is
// now the only home of these fields. If it opened on zeroes, the first checkbox press would
// write them back over the real configuration -- an empty keyword list, no provider, no model.
func TestTheAutoTagPageReadsTheStoredConfiguration(t *testing.T) {
	shell, db, tagger := shellWithTagging(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(ctx)

	stored, err := tagger.LoadSettings(ctx)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	stored.Enabled = true
	stored.LLM.BatchSize = 12
	if err := tagger.SaveSettings(ctx, stored); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}

	// The in-memory copy starts as a fresh launch's does.
	reset := func() {
		shell.state.AutoTagEnabled = false
		shell.state.BatchSizeInput = 0
	}
	check := func(entry string) {
		t.Helper()
		if !shell.state.AutoTagEnabled || shell.state.BatchSizeInput != 12 {
			t.Errorf("%s opened on zeroes: enabled=%v batch=%v",
				entry, shell.state.AutoTagEnabled, shell.state.BatchSizeInput)
		}
	}

	reset()
	shell.ToggleTagMode(ctx)
	shell.selectTagLibraryView(TagViewAutoTag)
	check("the view switch")
}

// TestTheVocabularyModeShowsNamesAndStoresIDs closes a content defect the re-layout exposed:
// the selector bound itself to the engine's id, so the page said "guarded" instead of the
// label "受限创建新标签".
func TestTheVocabularyModeShowsNamesAndStoresIDs(t *testing.T) {
	shell, db, tagger := shellWithTagging(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(ctx)

	stored, err := tagger.LoadSettings(ctx)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	stored.LLM.Mode = engine.VocabularyClosed
	if err := tagger.SaveSettings(ctx, stored); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}

	shell.state.TagMode = true
	shell.selectTagLibraryView(TagViewAutoTag)
	tester := ui.NewTester(shell.View, 1180, 900)
	tester.Frame()

	closed := shell.t().T(i18n.SettingsVocabularyClosed)
	if !tester.HasText(closed) {
		t.Errorf("the selector does not show %q; texts: %q", closed, tester.Texts())
	}
	if tester.HasText(string(engine.VocabularyClosed)) {
		t.Errorf("the selector shows the raw mode id")
	}

	// A translated name is what the select binds to, so saving has to map it back.
	shell.state.VocabularyModeLabel = shell.t().T(i18n.SettingsVocabularyOpen)
	shell.saveAutoTagSettings(ctx)
	stored, err = tagger.LoadSettings(ctx)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if stored.LLM.Mode != engine.VocabularyOpen {
		t.Errorf("saving the shown name stored %q, want %q", stored.LLM.Mode, engine.VocabularyOpen)
	}
}

// TestTheAutoTagPageMarksItselfForWork is the only status the table pays for: a dot on the
// other page's chip when a run is live or suggestions are waiting. Nothing else from that
// page is drawn on the table.
func TestTheAutoTagPageMarksItselfForWork(t *testing.T) {
	shell, _ := newShell(t)
	if shell.autoTagNeedsAttention() {
		t.Errorf("a fresh shell asks for attention")
	}
	shell.state.PendingSuggestions = []suggestions.PendingRow{{Suggestion: domain.TagSuggestion{ID: "s1"}}}
	if !shell.autoTagNeedsAttention() {
		t.Errorf("waiting suggestions do not mark the page")
	}
	shell.state.PendingSuggestions = nil
	shell.state.AutoTagRunning = true
	if !shell.autoTagNeedsAttention() {
		t.Errorf("a running pass does not mark the page")
	}
}

// TestTheDeploymentFieldIsOnlyForAzure covers a field that belonged to one provider.
//
// The deployment name is part of Azure's URL, not its model, and the other four providers
// ignore it -- so the control was one more box to wonder about, and its hint had to explain
// that it did nothing here. It appears when Azure is chosen and nowhere else.
func TestTheDeploymentFieldIsOnlyForAzure(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(ctx)
	shell.ToggleTagMode(ctx)
	openAutoTagPage(shell)

	tester := ui.NewTester(shell.View, 1180, 900)
	tester.Frame()
	if tester.HasText(shell.t().T(i18n.SettingsDeployment)) {
		t.Errorf("the deployment field is drawn for %q; texts: %q",
			shell.state.ProviderLabel, tester.Texts())
	}

	shell.state.ProviderLabel = shell.providerLabel("azure-openai")
	tester.Frame()
	if !tester.HasText(shell.t().T(i18n.SettingsDeployment)) {
		t.Errorf("the deployment field is missing for Azure; texts: %q", tester.Texts())
	}
}
