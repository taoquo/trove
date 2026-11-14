package ui

import (
	"context"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/internal/i18n"
)

// TestTheSettingsPageAnswersEveryClick walks the settings page: the way back, all four tabs,
// the general tab's appearance picker, the sync tab's switch, the find field, and the way out.
//
// The page is a stack of four surfaces in one slot, so a control that only exists on one tab is
// easy to leave unnamed or unwired; the back control, for one, was tooltip-only.
func TestTheSettingsPageAnswersEveryClick(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	shell.SetLocale(i18n.EN)
	seed(t, db, "1", "ada", "first", clockBase)
	shell.Reload(ctx)
	shell.OpenSettings()

	tester := ui.NewTester(shell.View, 1400, 880)
	tester.Frame()
	click := func(label string) {
		t.Helper()
		tester.Frame()
		if err := tester.Click(label); err != nil {
			t.Fatalf("clicking %q: %v (texts: %q)", label, err, tester.Texts())
		}
		tester.Frame()
	}
	// The tabs draw their labels in the uppercase warm-tab style; the label node is that text.
	tab := func(key i18n.Key) string { return strings.ToUpper(shell.t().T(key)) }

	// The way back is one square glyph, so it has to be named as well as tooltipped.
	if _, found := tester.Find(shell.t().T(i18n.SettingsBack)); !found {
		t.Errorf("the settings page's back control has no name; texts: %q", tester.Texts())
	}

	// Every tab switches the section in the same slot.
	for _, section := range []struct {
		id  string
		key i18n.Key
	}{
		{SettingsSectionSync, i18n.SettingsSectionSync},
		{SettingsSectionDataSafe, i18n.SettingsSectionDataSafety},
		{SettingsSectionListsViews, i18n.SettingsSectionListsViews},
		{SettingsSectionGeneral, i18n.SettingsSectionGeneral},
	} {
		click(tab(section.key))
		if got := shell.state.SettingsSection; got != section.id {
			t.Errorf("clicking %q opened the %q section", tab(section.key), got)
		}
	}

	// The general tab's appearance picker: open it, take the last option, and the appearance
	// changes. The trigger shows the current value, which is what the pointer presses.
	before := shell.state.Appearance
	click(shell.state.AppearanceLabel)
	tester.Key(0, ui.KeyEnd)
	tester.Key(0, ui.KeyEnter)
	tester.Frame()
	if shell.state.Appearance == before {
		t.Errorf("the appearance picker left the appearance at %q", before)
	}

	// The sync tab's first switch flips sync-on-launch. Its halves are named Off and On;
	// the first On in this section is the first switch's, which is this row's.
	click(tab(i18n.SettingsSectionSync))
	wasSyncOnLaunch := shell.state.SyncOnLaunch
	click(shell.t().T(i18n.SettingsOn))
	if shell.state.SyncOnLaunch == wasSyncOnLaunch {
		t.Error("the sync-on-launch switch did not flip")
	}

	// The find field narrows the page; the clear key puts the tabs back.
	click(shell.t().T(i18n.SettingsSearchPlaceholder))
	tester.Type("backup")
	tester.Frame()
	if !shell.settingsFiltering() {
		t.Fatal("typing in the find field did not filter the page")
	}
	if tester.HasText(tab(i18n.SettingsSectionGeneral)) {
		t.Error("the tabs stayed while the find field held words")
	}
	for range len("backup") {
		tester.Key(0, ui.KeyBackspace)
	}
	tester.Frame()
	if shell.settingsFiltering() {
		t.Error("clearing the find field left the page filtered")
	}
	if !tester.HasText(tab(i18n.SettingsSectionGeneral)) {
		t.Error("the tabs did not come back after the query was cleared")
	}

	// The way out.
	click(shell.t().T(i18n.SettingsBack))
	if shell.state.SettingsOpen {
		t.Error("the back control left the settings page open")
	}
}
