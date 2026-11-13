package ui

import (
	"context"
	"testing"
	"time"

	"github.com/taoquo/trove/internal/credentials"
	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/i18n"
)

// TestAStaleSummaryDoesNotOverwriteThisSession is the third defect behind the "X login
// expired" report.
//
// LoadSyncSummary runs on every Reload, including the one a sync's own publish callback
// triggers. When a run fails before its finishing write lands -- an unreachable host, a
// cancelled run, a crash -- the stored summary is still the *previous* run's. Adopting it
// put that run's sentence back on screen: measured on this machine, a 2026-10-24
// "unauthorized" summary was shown as the result of two 2026-10-25 runs that had failed on
// the network, with valid cookies in the Keychain.
//
// A summary older than what this window already knows is not news.
func TestAStaleSummaryDoesNotOverwriteThisSession(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()

	stale := clockBase
	writeSyncSummary(t, db, domain.SyncSummary{
		Status:       domain.SyncError,
		ErrorKind:    domain.ErrUnauthorized,
		ErrorSummary: "credentials: nothing stored (x.auth_token)",
		LastSyncedAt: &stale,
	})

	shell.UseSyncer(newSyncService(db))
	shell.UseCredentials(credentials.New(&memKeychain{}))
	shell.state.XCredentialsStored = true

	// What this session knows: a run that failed later, on the network.
	newer := clockBase.Add(48 * time.Hour)
	shell.state.LastSyncedAt = newer
	shell.state.LastSyncStatus = domain.SyncError
	shell.state.LastSyncErrorKind = string(domain.ErrNetwork)
	shell.state.LastSyncErrorSummary = "net/http: TLS handshake timeout"

	shell.LoadSyncSummary(ctx)

	if got, want := shell.state.LastSyncErrorKind, string(domain.ErrNetwork); got != want {
		t.Errorf("the reload replaced this session's kind with the stored one: %q, want %q", got, want)
	}
	if got, want := shell.state.LastSyncedAt, newer; !got.Equal(want) {
		t.Errorf("LastSyncedAt moved backwards to %v, want %v", got, want)
	}
	if got, want := shell.syncProblemLine(), shell.t().T(i18n.SyncErrorNetworkError); got != want {
		t.Errorf("the sidebar line reads %q, want %q", got, want)
	}
}

// TestANewerSummaryIsAdopted is the other direction, and it is why the guard compares
// timestamps rather than refusing every stored summary: a run recorded by another window,
// or by the run that just finished, must still reach the status line.
func TestANewerSummaryIsAdopted(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()

	stored := clockBase.Add(24 * time.Hour)
	writeSyncSummary(t, db, domain.SyncSummary{
		Status:       domain.SyncSuccess,
		FetchedCount: 42,
		LastSyncedAt: &stored,
	})

	shell.UseSyncer(newSyncService(db))
	shell.state.LastSyncedAt = clockBase

	shell.LoadSyncSummary(ctx)

	if got, want := shell.state.LastSyncedAt, stored; !got.Equal(want) {
		t.Errorf("LastSyncedAt = %v, want the newer stored %v", got, want)
	}
	if shell.state.LastSyncStatus != domain.SyncSuccess {
		t.Errorf("status = %q, want success", shell.state.LastSyncStatus)
	}
}

// TestASummaryWithoutATimestampIsAdopted pins the one case that cannot be compared. A
// summary written before the app recorded finish times is still better than nothing on a
// window that has never synced.
func TestASummaryWithoutATimestampIsAdopted(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()

	writeSyncSummary(t, db, domain.SyncSummary{
		Status:    domain.SyncError,
		ErrorKind: domain.ErrRateLimited,
	})

	shell.UseSyncer(newSyncService(db))
	shell.state.LastSyncedAt = clockBase.Add(time.Hour)

	shell.LoadSyncSummary(ctx)

	if got, want := shell.state.LastSyncErrorKind, string(domain.ErrRateLimited); got != want {
		t.Errorf("kind = %q, want %q: a summary with no timestamp must still be read", got, want)
	}
}
