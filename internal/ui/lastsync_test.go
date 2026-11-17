package ui

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/internal/credentials"
	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/i18n"
)

// TestSyncFailureSaysWhatToDo is the bug report this was written for: pressing 同步 showed an
// error and then no next step at all.
//
// Two things were wrong together.
//
//  1. The sync publish wrote its sentence to state.Err, and the Reload a few lines later clears
//     that field on a successful load -- so the sentence was set and then wiped before a frame
//     drew it. It is now derived from the stored run and drawn in the sidebar's sync block, the
//     one place it belongs.
//  2. An empty Keychain records the kind "unauthorized", whose sentence was "X login expired,
//     sign in again". This app has no login to expire: the two cookies are pasted into Settings >
//     Sync. The line now says that, and carries the control that goes there.
func TestSyncFailureSaysWhatToDo(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()

	writeSyncSummary(t, db, domain.SyncSummary{
		Status:       domain.SyncError,
		ErrorKind:    domain.ErrUnauthorized,
		ErrorSummary: "xsource: credentials: unauthorized: credentials: nothing stored (x.auth_token)",
	})
	shell.UseSyncer(newSyncService(db))
	shell.UseCredentials(credentials.New(&memKeychain{}))
	shell.RefreshXCookieStatus(ctx)
	shell.Reload(ctx)

	if shell.state.XCredentialsStored {
		t.Fatal("the empty test Keychain reports stored credentials")
	}
	// The reload above is the one that used to erase the message.
	if got, want := shell.state.LastSyncErrorKind, string(domain.ErrUnauthorized); got != want {
		t.Fatalf("the stored kind was not read back: %q, want %q", got, want)
	}
	if got, want := shell.syncProblemLine(), shell.t().T(i18n.SyncNeedsXCredentials); got != want {
		t.Fatalf("the failure line reads %q, want %q", got, want)
	}

	// A shell built in a test has no stored preferences, and a zero-width sidebar would wrap the
	// sentence one character per line and lay the button out below the window. The running app
	// gets these from ApplyPreferences.
	shell.state.LeftSidebarWidth = WidthSidebarDef
	shell.state.RightSidebarWidth = WidthInspectorDef

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	if !tester.HasText(shell.t().T(i18n.SyncNeedsXCredentials)) {
		t.Fatalf("the sidebar does not show the failure: %q", tester.Texts())
	}

	action := shell.t().T(i18n.ActionOpenSyncSettings)
	if err := tester.Click(action); err != nil {
		t.Fatalf("Click %q: %v", action, err)
	}
	tester.Frame()
	if !shell.state.SettingsOpen || shell.state.SettingsSection != SettingsSectionSync {
		t.Errorf("the next step landed on open=%v section=%q, want Settings > Sync",
			shell.state.SettingsOpen, shell.state.SettingsSection)
	}
}

// TestSyncFailureWithCookiesSaysTheLoginExpired is the other branch of the same sentence: when
// the Keychain does hold both cookies, "unauthorized" really does mean the session died, and the
// sign-in-again sentence is the right one.
func TestSyncFailureWithCookiesSaysTheLoginExpired(t *testing.T) {
	shell, _ := newShell(t)
	shell.UseCredentials(credentials.New(&memKeychain{}))
	shell.state.XCredentialsStored = true
	shell.state.LastSyncStatus = domain.SyncError
	shell.state.LastSyncErrorKind = string(domain.ErrUnauthorized)

	if got, want := shell.syncProblemLine(), shell.t().T(i18n.SyncErrorAuthExpired); got != want {
		t.Errorf("the failure line reads %q, want %q", got, want)
	}
}

// TestALockedStoreIsNotReportedAsAnExpiredLogin is the sentence boundary.
//
// A locked Keychain leaves the startup check saying "no session stored", so the
// empty-store branch would print "sync needs an X session" and send the reader to
// paste cookies that are already in the store. The stored kind knows better, and the
// keychain sentence is what has to win.
func TestALockedStoreIsNotReportedAsAnExpiredLogin(t *testing.T) {
	shell, _ := newShell(t)
	shell.UseCredentials(credentials.New(&memKeychain{}))
	shell.state.XCredentialsStored = false
	shell.state.LastSyncStatus = domain.SyncError
	shell.state.LastSyncErrorKind = string(domain.ErrCredentialStore)
	shell.state.LastSyncErrorSummary = "xsource: credentials: credential_store: credentials: the secure store is locked"

	line := shell.syncProblemLine()
	if line == shell.t().T(i18n.SyncNeedsXCredentials) {
		t.Fatalf("a locked store is reported as missing credentials: %q", line)
	}
	if line == shell.t().T(i18n.SyncErrorAuthExpired) {
		t.Fatalf("a locked store is reported as an expired login: %q", line)
	}
	if want := shell.t().T(i18n.SyncErrorCredentialStore); line != want {
		t.Errorf("the failure line reads %q, want %q", line, want)
	}
}

// TestAnUnclassifiedFailureKeepsTheRunsOwnMessage follows the `?? summary` branch: an unknown
// kind is not guessed at.
func TestAnUnclassifiedFailureKeepsTheRunsOwnMessage(t *testing.T) {
	shell, _ := newShell(t)
	shell.state.LastSyncStatus = domain.SyncError
	shell.state.LastSyncErrorKind = "a_kind_from_the_future"
	shell.state.LastSyncErrorSummary = "x returned 500"

	if got, want := shell.syncProblemLine(), "x returned 500"; got != want {
		t.Errorf("the failure line reads %q, want %q", got, want)
	}
}

// TestLastSyncLineShowsADateNotAStoredTimestamp pins the line to a date rather than a stored
// timestamp: the sidebar must not read "LAST SYNC 2026-10-17T10:52:03.889Z".
//
// Why it survived without notice
// ------------------------------
// lastSyncLine called domain.FormatTimestamp, which is the *storage* formatter. The same mistake
// had already been fixed on the card's Published line without a sweep for the others. Every one
// of the eight baseline screenshots was captured with LastSyncedAt at its zero value -- the "Not
// synced yet" branch -- so the broken branch was never rendered by anything that was looked at.
//
// The assertion is on the shape, not on a formatted string: a timezone in the machine's zone would
// make a literal date assertion fail for reasons that have nothing to do with the fix. What must be
// true in every zone and both locales is that the line is not an ISO 8601 instant and that it does
// carry a locale's date form.
func TestLastSyncLineShowsADateNotAStoredTimestamp(t *testing.T) {
	shell, _ := newShell(t)
	shell.state.LastSyncedAt = time.Date(2026, 10, 17, 10, 52, 3, 889000000, time.UTC)

	for _, tc := range []struct {
		locale string
		marker string
	}{
		{"en", "Oct"},
		{"zh-CN", "月"},
	} {
		shell.SetLocale(tc.locale)
		got := shell.lastSyncLine()

		if strings.Contains(got, "T") && strings.HasSuffix(got, "Z") {
			t.Errorf("%s: the line is a stored timestamp (%q), not a date", tc.locale, got)
		}
		if !strings.Contains(got, tc.marker) {
			t.Errorf("%s: %q does not contain %q -- it is not formatted for the locale",
				tc.locale, got, tc.marker)
		}
	}
}

// TestLastSyncLineStillSaysNeverWhenItNeverHas keeps the zero-value branch honest: the fix must not
// turn "not synced yet" into a formatted 1970.
func TestLastSyncLineStillSaysNeverWhenItNeverHas(t *testing.T) {
	shell, _ := newShell(t)
	shell.state.LastSyncedAt = time.Time{}
	if got := shell.lastSyncLine(); !strings.EqualFold(got, "not synced yet") && !strings.Contains(got, "同步") {
		t.Errorf("the unsynced line reads %q, want the never-synced wording", got)
	}
}

// TestNoSurfaceRendersAStoredTimestamp sweeps rather than naming one site.
//
// The storage formatter appeared in three places -- the sidebar, the auto-tag panel and the
// results toolbar -- and fixing them one at a time kept missing the next one. This renders every
// surface that shows a sync time and fails on any text that still looks like an ISO instant.
func TestNoSurfaceRendersAStoredTimestamp(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.state.LastSyncedAt = clockBase
	shell.state.LastSyncStatus = domain.SyncError
	shell.state.LastSyncErrorKind = string(domain.ErrUnknown)
	shell.Reload(context.Background())

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	iso := regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}`)
	for _, text := range tester.Texts() {
		if iso.MatchString(text) {
			t.Errorf("a surface shows a stored timestamp: %q", text)
		}
	}
}

// TestADeadSessionStillOffersTheWayToFixIt is the state the installed build was left in: the
// Keychain held both cookies, and the stored summary said the session had died.
//
// The sentence ("X login expired. Sign in to X again, then sync.") assumes a sign-in screen this
// app does not have. The two cookies are pasted or read from a browser under Settings > Sync, so
// the line has to offer that destination, or the instruction is a dead end.
func TestADeadSessionStillOffersTheWayToFixIt(t *testing.T) {
	shell, _ := newShell(t)
	shell.UseCredentials(credentials.New(&memKeychain{}))
	shell.state.XCredentialsStored = true
	shell.state.LastSyncStatus = domain.SyncError
	shell.state.LastSyncErrorKind = string(domain.ErrAuthExpired)
	shell.state.LeftSidebarWidth = WidthSidebarDef
	shell.state.RightSidebarWidth = WidthInspectorDef

	if got, want := shell.syncProblemLine(), shell.t().T(i18n.SyncErrorAuthExpired); got != want {
		t.Fatalf("the sentence reads %q, want the expected %q", got, want)
	}
	if !shell.syncNeedsNewCookies() {
		t.Fatal("an expired session with cookies stored offers no next step")
	}

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	action := shell.t().T(i18n.ActionOpenSyncSettings)
	if !tester.HasText(action) {
		t.Fatalf("the next step is missing: %q", tester.Texts())
	}
	if err := tester.Click(action); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if !shell.state.SettingsOpen || shell.state.SettingsSection != SettingsSectionSync {
		t.Errorf("landed on open=%v section=%q, want Settings > Sync",
			shell.state.SettingsOpen, shell.state.SettingsSection)
	}
}

// TestASuccessfulSyncOffersNoNextStep is the other side: the control belongs to the failure, so a
// successful run must not keep it on screen.
func TestASuccessfulSyncOffersNoNextStep(t *testing.T) {
	shell, _ := newShell(t)
	shell.state.XCredentialsStored = true
	shell.state.LastSyncStatus = domain.SyncSuccess
	shell.state.LastSyncErrorKind = string(domain.ErrAuthExpired)

	if shell.syncNeedsNewCookies() {
		t.Error("a successful sync still offers to replace the cookies")
	}
}

// TestAPartialSyncIsNotDrawnAsIdleOrAsATotalFailure covers the badge and the sentence for a
// run that stored most of the timeline and lost one page.
//
// The badge needs a partial_success case, or it falls through to the idle branch and a workspace
// that has just synced says "空闲". The sentence must not be suppressed for anything but error,
// or the reason the library is incomplete goes unshown.
func TestAPartialSyncIsNotDrawnAsIdleOrAsATotalFailure(t *testing.T) {
	shell, _ := newShell(t)
	shell.state.LastSyncStatus = domain.SyncPartialSuccess
	shell.state.LastSyncErrorKind = string(domain.ErrRateLimited)

	if got, want := shell.workspaceStatusLabel(), i18n.StatusPartial; got != want {
		t.Errorf("badge label = %q, want %q", got, want)
	}
	if got, want := shell.syncProblemLine(), shell.t().T(i18n.SyncErrorRateLimited); got != want {
		t.Errorf("the line reads %q, want the cause %q", got, want)
	}
}

// TestAPartialSyncWithoutACauseSaysNothing is the truncated run: stopped by the cap, nothing
// failed, so there is no cause to explain.
func TestAPartialSyncWithoutACauseSaysNothing(t *testing.T) {
	shell, _ := newShell(t)
	shell.state.LastSyncStatus = domain.SyncPartialSuccess

	if line := shell.syncProblemLine(); line != "" {
		t.Errorf("a partial run with no failure explains %q, want nothing", line)
	}
	if got, want := shell.workspaceStatusLabel(), i18n.StatusPartial; got != want {
		t.Errorf("badge label = %q, want %q", got, want)
	}
}
