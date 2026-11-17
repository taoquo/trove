package ui

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/internal/i18n"
	"github.com/taoquo/trove/internal/ports"
	"github.com/taoquo/trove/internal/storage/sqlite"
	"github.com/taoquo/trove/internal/usecase/settings"
)

// fakeLogSwitch records the switches without touching the process's loggers.
type fakeLogSwitch struct {
	applied []bool
	err     error
}

func (f *fakeLogSwitch) Apply(enabled bool) error {
	f.applied = append(f.applied, enabled)
	return f.err
}

// last answers what the most recent switch asked for.
func (f *fakeLogSwitch) last() (bool, bool) {
	if len(f.applied) == 0 {
		return false, false
	}
	return f.applied[len(f.applied)-1], true
}

// withLogging installs the settings store and the remembering log on a shell.
func withLogging(t *testing.T, shell *Shell, db *sqlite.DB) (*settings.Service, *fakeLogSwitch) {
	t.Helper()
	service := settings.New(ports.Repositories{Settings: sqlite.NewSettingsRepository(db)},
		fixedClock{now: clockBase})
	shell.UseSettings(service)
	logs := &fakeLogSwitch{}
	shell.UseLogging(logs)
	return service, logs
}

// TestTheStoredChoiceOpensTheLogAtLaunch covers the launch half: a reader who turned the
// log on last time finds it on the next start without touching the page again.
func TestTheStoredChoiceOpensTheLogAtLaunch(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	service, logs := withLogging(t, shell, db)

	stored, err := service.Load(ctx)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	stored.Diagnostics.FileLog = true
	if err := service.Save(ctx, stored); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := shell.ApplyPreferences(ctx); err != nil {
		t.Fatalf("ApplyPreferences: %v", err)
	}
	applied, asked := logs.last()
	if !asked || !applied {
		t.Errorf("the launch asked for %v, want the log open", logs.applied)
	}
	if !shell.state.FileLog {
		t.Error("the switch does not show the stored choice")
	}
}

// TestTheDataSafetySwitchRemembersTheLog walks the switch: turning it on opens the log and
// stores the choice, and turning it off closes the log and stores that.
func TestTheDataSafetySwitchRemembersTheLog(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	shell.SetLocale(i18n.EN)
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(ctx)
	service, logs := withLogging(t, shell, db)
	if err := shell.ApplyPreferences(ctx); err != nil {
		t.Fatalf("ApplyPreferences: %v", err)
	}
	shell.OpenSettings()

	tester := ui.NewTester(shell.View, 1400, 880)
	click := func(label string) {
		t.Helper()
		tester.Frame()
		if err := tester.Click(label); err != nil {
			t.Fatalf("clicking %q: %v (texts: %q)", label, err, tester.Texts())
		}
		tester.Frame()
	}

	click(strings.ToUpper(shell.t().T(i18n.SettingsSectionDataSafety)))
	if !tester.HasText(shell.t().T(i18n.SettingsFileLogRow)) {
		t.Fatalf("the Data safety card has no log row; texts: %q", tester.Texts())
	}

	click(shell.t().T(i18n.SettingsOn))
	if !shell.state.FileLog {
		t.Fatal("the On segment left the log off")
	}
	if applied, _ := logs.last(); !applied {
		t.Errorf("the On segment did not open the log: %v", logs.applied)
	}

	click(shell.t().T(i18n.SettingsOff))
	if shell.state.FileLog {
		t.Fatal("the Off segment left the log on")
	}
	if applied, _ := logs.last(); applied {
		t.Errorf("the Off segment did not close the log: %v", logs.applied)
	}

	stored, err := service.Load(ctx)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if stored.Diagnostics.FileLog {
		t.Error("the Off choice was not stored")
	}

	// The next launch reads the stored Off, which is what ApplyPreferences is.
	if err := shell.ApplyPreferences(ctx); err != nil {
		t.Fatalf("ApplyPreferences: %v", err)
	}
	if applied, _ := logs.last(); applied {
		t.Error("a launch after choosing Off opened the log")
	}
}

// TestAFailedLogSwitchSaysWhy covers a folder the app cannot write: the switch keeps the
// reader's press, and the reason is a sentence under the card rather than a control that
// springs back with no explanation.
func TestAFailedLogSwitchSaysWhy(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	shell.SetLocale(i18n.EN)
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(ctx)
	_, logs := withLogging(t, shell, db)
	logs.err = errors.New("permission denied")
	if err := shell.ApplyPreferences(ctx); err != nil {
		t.Fatalf("ApplyPreferences: %v", err)
	}
	shell.OpenSettings()

	tester := ui.NewTester(shell.View, 1400, 880)
	tester.Frame()
	if err := tester.Click(strings.ToUpper(shell.t().T(i18n.SettingsSectionDataSafety))); err != nil {
		t.Fatalf("clicking the tab: %v", err)
	}
	tester.Frame()
	if err := tester.Click(shell.t().T(i18n.SettingsOn)); err != nil {
		t.Fatalf("clicking On: %v", err)
	}
	tester.Frame()

	if !strings.Contains(shell.state.LogNote, "permission denied") {
		t.Errorf("the failure note is %q, want the reason", shell.state.LogNote)
	}
	if !tester.HasText(shell.state.LogNote) {
		t.Errorf("the failure note is not drawn; texts: %q", tester.Texts())
	}
}
