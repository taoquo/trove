package syncer

import (
	"context"
	"sync"
	"testing"

	"github.com/taoquo/trove/apps/macos/internal/domain"
	"github.com/taoquo/trove/apps/macos/internal/ports"
	"github.com/taoquo/trove/apps/macos/internal/xsource"
)

// authFailSource rejects the first request until renew is called.
//
// It stands in for the stored pair going stale: X answers 401, the browser still
// has a live session, and the retry with the refreshed pair works.
type authFailSource struct {
	mu    sync.Mutex
	live  bool
	fail  domain.SyncErrorKind
	calls int
}

func (f *authFailSource) FetchPage(context.Context, string) (ports.Page, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if !f.live {
		kind := f.fail
		if kind == "" {
			kind = domain.ErrAuthExpired
		}
		return ports.Page{}, &xsource.Error{Kind: kind, Op: "fetch"}
	}
	return page("1"), nil
}

func (f *authFailSource) renew() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.live = true
}

func (f *authFailSource) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// TestAnExpiredSessionIsRefreshedAndTheRunRetriedOnce is the whole point of the
// refresher.
//
// The stored pair goes stale (X rotates ct0); the extension never notices
// because chrome.cookies gives it a new one on every request. The desktop app
// now does the same thing at the moment it matters: the run fails before its
// first page, the browser's live session is adopted, and the same run continues.
// One run record, one failure the reader never sees.
func TestAnExpiredSessionIsRefreshedAndTheRunRetriedOnce(t *testing.T) {
	source := &authFailSource{}
	service, _ := newService(t, source)

	refreshes := 0
	service.UseCredentialRefresher(func(context.Context) (bool, error) {
		refreshes++
		source.renew()
		return true, nil
	})

	run, err := service.Run(context.Background(), RunOptions{Sleep: noSleep})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if run.Status != domain.SyncSuccess {
		t.Errorf("status = %q, want %q", run.Status, domain.SyncSuccess)
	}
	if run.FetchedCount != 1 {
		t.Errorf("FetchedCount = %d, want 1", run.FetchedCount)
	}
	if refreshes != 1 {
		t.Errorf("refreshed %d times, want 1", refreshes)
	}
	if source.callCount() != 2 {
		t.Errorf("requests = %d, want the failed one and the retry", source.callCount())
	}
}

// TestAnUnchangedBrowserPairFailsTheRun keeps the retry from being decoration.
//
// When the browser holds the same dead pair, retrying would send the same
// request to X. The refresher says "no change" and the run reports the failure
// it originally had, which is what the sidebar acts on.
func TestAnUnchangedBrowserPairFailsTheRun(t *testing.T) {
	source := &authFailSource{}
	service, _ := newService(t, source)
	service.UseCredentialRefresher(func(context.Context) (bool, error) { return false, nil })

	run, err := service.Run(context.Background(), RunOptions{Sleep: noSleep})
	if err == nil {
		t.Fatal("Run succeeded with a dead session")
	}
	if run.Status != domain.SyncError {
		t.Errorf("status = %q, want %q", run.Status, domain.SyncError)
	}
	if run.ErrorKind != domain.ErrAuthExpired {
		t.Errorf("ErrorKind = %q, want %q", run.ErrorKind, domain.ErrAuthExpired)
	}
	if source.callCount() != 1 {
		t.Errorf("requests = %d, want only the failed one", source.callCount())
	}
}

// TestOnlyAnAuthFailureIsRefreshed is the boundary.
//
// A network fault is not a cookie problem; sending the reader's browser profile
// through the Keychain for it would be a strange thing to do, and the refresh
// cannot fix a 429 either.
func TestOnlyAnAuthFailureIsRefreshed(t *testing.T) {
	for _, kind := range []domain.SyncErrorKind{domain.ErrNetwork, domain.ErrRateLimited, domain.ErrUnknown} {
		t.Run(string(kind), func(t *testing.T) {
			source := &authFailSource{fail: kind}
			service, _ := newService(t, source)

			refreshes := 0
			service.UseCredentialRefresher(func(context.Context) (bool, error) {
				refreshes++
				source.renew()
				return true, nil
			})

			if _, err := service.Run(context.Background(), RunOptions{Sleep: noSleep}); err == nil {
				t.Fatal("Run succeeded")
			}
			if refreshes != 0 {
				t.Errorf("a %q failure triggered a credential refresh", kind)
			}
		})
	}
}
