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

// fakeBackups records the launch copies without writing a database.
type fakeBackups struct {
	runs int
	path string
	err  error
}

func (f *fakeBackups) Run(context.Context) (string, error) {
	f.runs++
	return f.path, f.err
}

func (f *fakeBackups) Directory() string { return "/data/backups" }

// withBackups installs the settings store and the daily copy on a shell.
func withBackups(t *testing.T, shell *Shell, db *sqlite.DB) (*settings.Service, *fakeBackups) {
	t.Helper()
	service := settings.New(ports.Repositories{Settings: sqlite.NewSettingsRepository(db)},
		fixedClock{now: clockBase})
	shell.UseSettings(service)
	backups := &fakeBackups{}
	shell.UseBackups(backups, func(string) error { return nil })
	return service, backups
}

// TestTheLaunchCopiesTheLibraryOnceAndSaysSo covers the automatic half: a fresh install
// backs up without being asked, and the result is a sentence the reader can find.
func TestTheLaunchCopiesTheLibraryOnceAndSaysSo(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	_, backups := withBackups(t, shell, db)
	backups.path = "/data/backups/trove-20261012.db"

	if err := shell.ApplyPreferences(ctx); err != nil {
		t.Fatalf("ApplyPreferences: %v", err)
	}
	if !shell.state.AutoBackup {
		t.Fatal("a fresh install turned automatic backups off")
	}

	shell.StartAutomaticBackup(ctx)
	if backups.runs != 1 {
		t.Errorf("the launch asked for %d copies, want 1", backups.runs)
	}
	if !strings.Contains(shell.state.BackupNote, "trove-20261012.db") {
		t.Errorf("the result note is %q, want the copy's name", shell.state.BackupNote)
	}
}

// TestAFailedLaunchCopySaysWhy covers the read-only folder: the failure is not fatal,
// but it must leave something the reader can act on rather than a silent no-op.
func TestAFailedLaunchCopySaysWhy(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	_, backups := withBackups(t, shell, db)
	backups.err = errors.New("read-only file system")

	if err := shell.ApplyPreferences(ctx); err != nil {
		t.Fatalf("ApplyPreferences: %v", err)
	}
	shell.StartAutomaticBackup(ctx)

	if !strings.Contains(shell.state.BackupNote, "read-only file system") {
		t.Errorf("the failure note is %q, want the reason", shell.state.BackupNote)
	}
}

// TestTheDataSafetySwitchTurnsTheLaunchCopyOff walks the switch a reader would press:
// the Data safety tab's Off segment stores the choice, and the next launch respects it.
func TestTheDataSafetySwitchTurnsTheLaunchCopyOff(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	shell.SetLocale(i18n.EN)
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(ctx)
	service, backups := withBackups(t, shell, db)
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
	if !tester.HasText(shell.t().T(i18n.SettingsAutoBackupRow)) {
		t.Fatalf("the Data safety card has no automatic-backup row; texts: %q", tester.Texts())
	}
	click(shell.t().T(i18n.SettingsOff))
	if shell.state.AutoBackup {
		t.Fatal("the Off segment left automatic backups on")
	}

	stored, err := service.Load(ctx)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if stored.AutoBackupEnabled() {
		t.Error("the switch did not store the choice")
	}

	backups.runs = 0
	shell.StartAutomaticBackup(ctx)
	if backups.runs != 0 {
		t.Errorf("a launch after choosing Off still wrote %d copy(ies)", backups.runs)
	}

	// The choice survives a reload of the preferences, which is what a restart does.
	shell.state.AutoBackup = true
	if err := shell.ApplyPreferences(ctx); err != nil {
		t.Fatalf("ApplyPreferences: %v", err)
	}
	if shell.state.AutoBackup {
		t.Error("the stored Off was lost on the next load")
	}
}
