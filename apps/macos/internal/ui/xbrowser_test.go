package ui

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/x-master/apps/macos/internal/credentials"
	"github.com/taoquo/x-master/apps/macos/internal/domain"
	"github.com/taoquo/x-master/apps/macos/internal/i18n"
	"github.com/taoquo/x-master/apps/macos/internal/xsource"
)

// TestReadingFromTheBrowserIsBestEffort pins the behaviour that matters on a machine where macOS
// refuses the browser directories: the attempt must not panic, must not claim success, and must
// leave a note the user can act on. Whether a browser answers is a property of the machine, so the
// assertion is on the shape of the outcome rather than on a cookie being found.
func TestReadingFromTheBrowserIsBestEffort(t *testing.T) {
	shell, _ := newShell(t)
	store := credentials.New(&memKeychain{})
	shell.UseCredentials(store)

	shell.readXCookiesFromBrowser()

	// Wait for the goroutine rather than polling what it writes: without a window publish runs
	// on the goroutine, so a spin on the note is a read of a field being written concurrently --
	// a data race the -race gate reports. Closing done happens after the write.
	shell.awaitBrowserRead()
	if shell.state.XBrowserReadNote == "" {
		t.Fatal("the attempt left no note at all")
	}
	if len(shell.state.XBrowserReadNote) < 10 {
		t.Errorf("the note is too short to be actionable: %q", shell.state.XBrowserReadNote)
	}
	// If it did find cookies, they must be in the store; if it did not, the store must stay empty.
	_, err := store.XCredentials(context.Background())
	if shell.state.XCredentialsStored != (err == nil) {
		t.Errorf("XCredentialsStored = %v but the store says err=%v",
			shell.state.XCredentialsStored, err)
	}
}

// awaitBrowserRead waits for whichever browser read was started last.
//
// A shell without a window runs publish on the goroutine itself, so a test that read the note
// immediately would race the write rather than observe it.
func (s *Shell) awaitBrowserRead() {
	if s.browserReadDone != nil {
		<-s.browserReadDone
	}
}

// TestTheSidebarReconnectsFromTheBrowser covers the whole path from a failed sync to a new
// session.
//
// The failure the app could only describe before -- "X login expired, sign in again" -- has no
// sign-in screen to point at. The button beside the sentence now does what the sentence means:
// read the browser the reader is already signed in to, save the pair, and let the next sync run.
func TestTheSidebarReconnectsFromTheBrowser(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "a post", clockBase)

	store := credentials.New(&memKeychain{})
	reads := 0
	store.Browser = func(context.Context) (xsource.Credentials, error) {
		reads++
		return xsource.Credentials{AuthToken: "auth-fresh", CSRFToken: "csrf-fresh"}, nil
	}
	shell.UseCredentials(store)
	shell.state.XCredentialsStored = false
	shell.state.LastSyncStatus = domain.SyncError
	shell.state.LastSyncErrorKind = string(domain.ErrAuthExpired)
	shell.Reload(context.Background())

	tester := ui.NewTester(shell.View, 1180, 900)
	tester.Frame()
	if !tester.HasText(shell.t().T(i18n.ActionReconnectX)) {
		t.Fatalf("the sidebar offers no way to reconnect: %q", tester.Texts())
	}
	if err := tester.Click(shell.t().T(i18n.ActionReconnectX)); err != nil {
		t.Fatalf("Click: %v", err)
	}
	shell.awaitBrowserRead()

	if reads != 1 {
		t.Errorf("the button read the browser %d times, want 1", reads)
	}
	stored, err := store.XCredentials(context.Background())
	if err != nil {
		t.Fatalf("XCredentials after reconnecting: %v", err)
	}
	if stored.AuthToken != "auth-fresh" || stored.CSRFToken != "csrf-fresh" {
		t.Errorf("stored = %+v, want the browser's pair", stored)
	}
	if shell.state.XBrowserReadNote != shell.t().T(i18n.SettingsXBrowserSaved) {
		t.Errorf("note = %q, want the saved message", shell.state.XBrowserReadNote)
	}
}

// TestAnUnchangedBrowserPairSaysSo keeps "reconnected" from lying.
//
// When the browser holds the same pair, nothing was adopted and no sync is worth
// retrying. The note has to say that, because the alternative is a button that
// reports success and changes nothing.
func TestAnUnchangedBrowserPairSaysSo(t *testing.T) {
	shell, _ := newShell(t)
	keychain := &memKeychain{}
	store := credentials.New(keychain)
	store.Browser = func(context.Context) (xsource.Credentials, error) {
		return xsource.Credentials{AuthToken: "auth-fresh", CSRFToken: "csrf-fresh"}, nil
	}
	shell.UseCredentials(store)
	if _, err := store.RefreshXCredentialsFromBrowser(context.Background()); err != nil {
		t.Fatalf("seed: %v", err)
	}

	shell.ReconnectXSession()
	shell.awaitBrowserRead()

	if shell.state.XBrowserReadNote != shell.t().T(i18n.SettingsXBrowserUnchanged) {
		t.Errorf("note = %q, want the unchanged message", shell.state.XBrowserReadNote)
	}
}
