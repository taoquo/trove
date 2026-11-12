package ui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/apps/macos/internal/i18n"
)

// TestTheSidebarLanguageChipRelabelsTheSettingsPage pins the two ways into the language.
//
// The settings page's two selectors are bound to translated strings held in state
// (LocaleLabel, AppearanceLabel), which syncPreferenceLabels derives. The settings
// section's own handler re-derives them after SetLocale; the sidebar footer's chip did
// not, so switching the language from the footer while Settings was open left the theme
// row reading "Light" inside an otherwise Chinese page, and the language row still
// claiming "English" was the current choice. Round 23 found this by walking every screen
// in both languages; the capture path cannot show it, because TROVE_CAPTURE_APPEARANCE
// re-derives the labels after the language is applied.
func TestTheSidebarLanguageChipRelabelsTheSettingsPage(t *testing.T) {
	shell, _ := newShell(t)
	shell.SetLocale(i18n.EN, i18n.EN)
	shell.OpenSettings()

	tester := ui.NewTester(shell.View, 1180, 900)
	tester.Frame()
	if !tester.HasText(shell.t().T(i18n.SettingsLanguage)) {
		t.Fatalf("the settings page is not open; texts: %q", tester.Texts())
	}
	if !tester.HasText(i18n.New(i18n.EN, i18n.EN).T(i18n.AppearanceLight)) {
		t.Fatalf("the theme row does not show its English label to begin with; texts: %q", tester.Texts())
	}

	if err := tester.Click(shell.t().T(i18n.LanguageSwitch)); err != nil {
		t.Fatalf("the footer language chip is not reachable: %v", err)
	}
	tester.Frame()

	if shell.state.Locale != i18n.ZHCN {
		t.Fatalf("the chip left the language at %q", shell.state.Locale)
	}
	zh := i18n.New(i18n.ZHCN, i18n.ZHCN)
	if !tester.HasText(zh.T(i18n.AppearanceLight)) {
		t.Errorf("the theme row still does not read %q after switching to Chinese; texts: %q",
			zh.T(i18n.AppearanceLight), tester.Texts())
	}
	if got := shell.state.AppearanceLabel; got != zh.T(i18n.AppearanceLight) {
		t.Errorf("the theme selector holds %q, want %q", got, zh.T(i18n.AppearanceLight))
	}
}
