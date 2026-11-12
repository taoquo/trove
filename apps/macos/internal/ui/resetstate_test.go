package ui

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/x-master/apps/macos/internal/credentials"
	"github.com/taoquo/x-master/apps/macos/internal/i18n"
	"github.com/taoquo/x-master/apps/macos/internal/importexport"
	"github.com/taoquo/x-master/apps/macos/internal/ports"
	"github.com/taoquo/x-master/apps/macos/internal/storage/sqlite"
	"github.com/taoquo/x-master/apps/macos/internal/usecase/settings"
	"github.com/taoquo/x-master/apps/macos/internal/xsource"
)

// TestResetSaysTheLibraryIsGone covers the one destructive command with no receipt.
//
// Export says "Backup exported", restore says what it wrote, and the reset -- the one
// command that removes everything -- cleared the report it had shown while working and
// left the reader with an empty library and no sentence. An empty library is also what a
// failed load looks like. The source renders copy.localDataReset in its data-safety panel
// after handleResetLocalData (DataSafetyPanel.tsx:37-39). Round 31.
func TestResetSaysTheLibraryIsGone(t *testing.T) {
	shell, db := newBatchShell(t)
	bundleService(t, shell, db)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(ctx)

	work := capturedBackground(shell)
	shell.ConfirmResetLocalDataNow(ctx)
	(*work)()

	want := shell.t().T(i18n.LocalDataResetDone)
	if got := shell.State().LastBundleReport; got != want {
		t.Errorf("a finished reset reports %q, want %q", got, want)
	}
	if !shell.State().LastBundleOK {
		t.Error("the receipt is filed like a progress note, so it is drawn as one")
	}

	shell.state.SettingsOpen = true
	shell.state.SettingsSection = SettingsSectionDataSafe
	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	if !tester.HasText(want) {
		t.Errorf("the data-safety page does not say the reset ran; want %q, texts: %q",
			want, tester.Texts())
	}
}

// TestResetForgetsTheSettingsItDeleted covers what the window remembers after the wipe.
//
// ResetLocalData deletes the settings rows and the Keychain secrets, and the window kept
// every one of them in memory: the page went on showing "cookies saved" and the sync
// section went on offering to sync on launch, for values the reset had just removed. The
// source answers with reloadOptionsPage() -- literally reloading the page -- because its
// state is the storage. This shell has to re-read the same sources the settings page reads
// when it opens. Round 31.
func TestResetForgetsTheSettingsItDeleted(t *testing.T) {
	shell, db := newBatchShell(t)
	ctx := context.Background()
	repos := ports.Repositories{
		Bookmarks:   sqlite.NewBookmarkRepository(db),
		Tags:        sqlite.NewTagRepository(db),
		Lists:       sqlite.NewListRepository(db),
		Suggestions: sqlite.NewSuggestionRepository(db),
		Analysis:    sqlite.NewAnalysisRepository(db),
		Settings:    sqlite.NewSettingsRepository(db),
		SavedViews:  sqlite.NewSavedViewRepository(db),
		SyncRuns:    sqlite.NewSyncRunRepository(db),
	}
	preferences := settings.New(repos, fixedClock{now: clockBase})
	shell.UseSettings(preferences)

	stored, err := preferences.Load(ctx)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	stored.Sync.SyncOnLaunch = true
	if err := preferences.Save(ctx, stored); err != nil {
		t.Fatalf("Save: %v", err)
	}
	shell.loadSyncPreferences(ctx)
	if !shell.State().SyncOnLaunch {
		t.Fatal("the preference was not loaded, so the test proves nothing")
	}

	secretStore := credentials.New(&memKeychain{})
	shell.UseCredentials(secretStore)
	if err := secretStore.SaveXCredentials(ctx, xsource.Credentials{
		AuthToken: "auth", CSRFToken: "csrf",
	}); err != nil {
		t.Fatalf("SaveXCredentials: %v", err)
	}
	shell.RefreshXCookieStatus(ctx)
	if !shell.State().XCredentialsStored {
		t.Fatal("the stored cookies were not seen, so the test proves nothing")
	}

	service := importexport.New(repos, fixedClock{now: clockBase}, "0.3.0")
	service.UseSecretClearer(func(ctx context.Context) error { return secretStore.DeleteAll(ctx, nil) })
	shell.UseImportExport(service)

	work := capturedBackground(shell)
	shell.ConfirmResetLocalDataNow(ctx)
	(*work)()

	if shell.State().SyncOnLaunch {
		t.Error("the reset deleted the preference and the window still offers to sync on launch")
	}
	if shell.State().XCredentialsStored {
		t.Error("the reset cleared the Keychain and the page still says the cookies are saved")
	}
}
