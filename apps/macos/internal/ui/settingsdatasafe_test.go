package ui

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/apps/macos/internal/i18n"
)

// Round 18 walks settings · sync and settings · data safety.
//
// Two things were wrong on those two pages, and both are the same kind of thing: a row that
// does not say what it does. The credentials card was named twice in a row -- once by its
// group label, once by its own first row -- and the block list was the one row in the data
// card with no explanation, and it threw its contents away without saying so.

// TestTheSyncSectionNamesTheCredentialCardOnce pins the credential card's heading.
//
// The source's sync section is a run of rows with no group heading above the cookies block
// (SettingsPanel.tsx:726-744), and the page's own other half already runs without one. The
// native group label repeated the card's first row word for word, so the reader met
// "X credentials" and then immediately "X credentials" again.
func TestTheSyncSectionNamesTheCredentialCardOnce(t *testing.T) {
	shell, _ := newShell(t)
	shell.SetLocale(i18n.EN, i18n.EN)
	shell.OpenSettings()
	shell.state.SettingsSection = SettingsSectionSync

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	// The group label draws uppercased (settingsSectionLabel), so it only matches the card's
	// title case-insensitively; in Chinese the two strings are identical outright.
	label := shell.t().T(i18n.SettingsXCookiesTitle)
	seen := 0
	for _, text := range tester.Texts() {
		if strings.EqualFold(text, label) {
			seen++
		}
	}
	if seen != 1 {
		t.Errorf("the credentials card is named %d times, want 1; texts: %q", seen, tester.Texts())
	}
}

// TestTheBlockedTagRowSaysWhatItIsAndConfirmsTheClear covers the block list row.
//
// Every other row in the data card carries a line under its name saying what it does; this
// one carried nothing, so "blocked tags" was the only word a reader had. Clearing it was
// then silent: the button and its row disappeared, and the settings page said nothing about
// what had just been thrown away. The extension sets its own "Block list cleared." message
// at the same moment (SettingsPanel.tsx:237-243).
func TestTheBlockedTagRowSaysWhatItIsAndConfirmsTheClear(t *testing.T) {
	shell, _, tagger := shellWithTagging(t)
	ctx := context.Background()
	shell.SetLocale(i18n.EN, i18n.EN)
	for _, name := range []string{"spam", "crypto"} {
		if err := tagger.BlockTagKey(ctx, name); err != nil {
			t.Fatalf("block %q: %v", name, err)
		}
	}
	shell.state.BlockedTagKeyCount = 2
	shell.OpenSettings()
	shell.state.SettingsSection = SettingsSectionDataSafe

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	if !tester.HasText(shell.t().T(i18n.SettingsBlockedTagsHint)) {
		t.Errorf("the block list row has no line saying what it is; texts: %q", tester.Texts())
	}
	if !tester.HasText(shell.t().T(i18n.SettingsBlockedTags)) {
		t.Fatalf("the block list row is not drawn at all; texts: %q", tester.Texts())
	}

	label := fmt.Sprintf("%s (%d)", shell.t().T(i18n.SettingsClearBlockedTags), 2)
	tester.Click(label)
	tester.Frame()

	if !tester.HasText(shell.t().T(i18n.SettingsClearBlockedTagsDone)) {
		t.Errorf("clearing the block list said nothing; texts: %q", tester.Texts())
	}
	settings, err := tagger.LoadSettings(ctx)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if len(settings.BlockedTagKeys) != 0 {
		t.Errorf("the list still holds %v", settings.BlockedTagKeys)
	}
}
