package ui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/apps/macos/internal/i18n"
)

// TestTheClearKeyButtonAppearsOnlyWithAStoredKey covers the second half of the API-key row.
//
// Save and Clear were both drawn as live controls whatever the state: Save with an empty field
// and Clear with no key each did nothing and said nothing, so a press looked broken rather than
// unnecessary. The source disables Save until there is something to save and does not render
// Clear at all without a key (SettingsPanel.tsx:1032-1048); this pins the half a headless
// tester can see -- the button that should not exist.
func TestTheClearKeyButtonAppearsOnlyWithAStoredKey(t *testing.T) {
	shell, _, _ := shellWithTagging(t)
	shell.SetLocale(i18n.EN, i18n.EN)
	shell.state.TagMode = true
	shell.selectTagLibraryView(TagViewAutoTag)
	clearLabel := shell.t().T(i18n.ActionClear)

	shell.state.APIKeyStored = false
	tester := ui.NewTester(shell.View, 1180, 900)
	tester.Frame()
	if tester.HasText(clearLabel) {
		t.Errorf("Clear is offered with no key stored; texts: %q", tester.Texts())
	}

	shell.state.APIKeyStored = true
	tester.Frame()
	if !tester.HasText(clearLabel) {
		t.Errorf("Clear is missing while a key is stored; texts: %q", tester.Texts())
	}
}
