package ui

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/internal/i18n"
)

// TestTheUpdatesTabSaysWhatItRunsAndCallsTheUpdater covers the page a reader opens to ask "how
// do I update": it names the version, says when it last looked, and its two controls reach the
// updater instead of only looking like they do.
func TestTheUpdatesTabSaysWhatItRunsAndCallsTheUpdater(t *testing.T) {
	shell, _ := newShell(t)
	shell.SetLocale(i18n.EN)

	checks, saved := 0, 0
	shell.UseUpdates(Updates{
		Version:            "9.9.9",
		Check:              func() { checks++ },
		AutomaticChecks:    func() bool { return false },
		SetAutomaticChecks: func(bool) { saved++ },
		LastCheck:          func() time.Time { return time.Time{} },
	})

	shell.OpenSettings()
	shell.state.SettingsSection = SettingsSectionGeneral

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	texts := tester.Texts()

	// The version row: the number the bundle carries, and that no check has finished yet.
	if want := shell.t().Tf(i18n.SettingsVersionLineNever, "9.9.9"); !containsText(texts, want) {
		t.Errorf("the updates tab does not show %q; texts: %q", want, texts)
	}
	if !containsText(texts, shell.t().T(i18n.SettingsVersion)) {
		t.Errorf("the version row has no name; texts: %q", texts)
	}
	if !containsText(texts, shell.t().T(i18n.SettingsAutoCheck)) {
		t.Errorf("the updates tab has no automatic-check row; texts: %q", texts)
	}

	// The row's switch starts where the updater says it is -- off -- and clicking On writes the
	// other value through to the updater, not just to the state.
	if shell.state.UpdateAutoCheck {
		t.Fatal("the switch started on although the updater says it is off")
	}
	if err := tester.Click(shell.t().T(i18n.SettingsOn)); err != nil {
		t.Fatalf("clicking On: %v (texts: %q)", err, tester.Texts())
	}
	tester.Frame()
	if saved != 1 || !shell.state.UpdateAutoCheck {
		t.Errorf("clicking On wrote %d time(s) and left the switch at %v", saved, shell.state.UpdateAutoCheck)
	}

	// The row's action is the app menu's command under another name: it opens the update window.
	if err := tester.Click(shell.t().T(i18n.MenuCheckUpdates)); err != nil {
		t.Fatalf("clicking %q: %v (texts: %q)", shell.t().T(i18n.MenuCheckUpdates), err, tester.Texts())
	}
	if checks != 1 {
		t.Errorf("the check action ran %d time(s), want 1", checks)
	}
}

// TestTheUpdatesTabShowsWhenItLastChecked is the other half of the version line: once the
// updater reports a finished check the row says when, and the "not checked" copy is gone.
func TestTheUpdatesTabShowsWhenItLastChecked(t *testing.T) {
	shell, _ := newShell(t)
	shell.SetLocale(i18n.EN)
	checked := time.Date(2026, 11, 14, 9, 5, 0, 0, time.Local)
	shell.UseUpdates(Updates{
		Version:            "9.9.9",
		AutomaticChecks:    func() bool { return true },
		SetAutomaticChecks: func(bool) {},
		LastCheck:          func() time.Time { return checked },
	})

	shell.OpenSettings()
	shell.state.SettingsSection = SettingsSectionGeneral

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	texts := tester.Texts()

	want := shell.t().Tf(i18n.SettingsVersionLine, "9.9.9", "2026-11-14 09:05")
	if !containsText(texts, want) {
		t.Errorf("the updates tab does not show %q; texts: %q", want, texts)
	}
	if containsText(texts, shell.t().Tf(i18n.SettingsVersionLineNever, "9.9.9")) {
		t.Errorf("the row says it never checked although it checked at %s; texts: %q", checked, texts)
	}
	if !shell.state.UpdateAutoCheck {
		t.Error("the switch did not follow the updater's answer")
	}
}
