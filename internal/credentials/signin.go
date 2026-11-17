// Signing in from a terminal.
//
// The window gets its X session by reading a browser that is already signed in. The
// person who has just installed trove has no such browser, so `trove auth` has to send
// them to X first and then wait for the cookies to land. Everything after that -- the
// reader, the keychain accounts, the rule that the two cookies are one credential -- is
// exactly what the window already does.
package credentials

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/taoquo/trove/internal/xsource"
)

// SignInInterval is how often a waiting sign-in re-reads the browser profiles.
//
// A read snapshots and decrypts a cookie database, so the wait cannot spin. It is also
// roughly how long a browser takes to flush the cookies X sets during sign-in, which
// makes a shorter interval buy nothing but reads.
const SignInInterval = 2 * time.Second

// Sign-in failures that are about the wait rather than about the browser.
var (
	// ErrSignInTimedOut means no browser held a session before the wait ended.
	ErrSignInTimedOut = errors.New("credentials: no signed-in X session appeared in time")
	// ErrSignInStopped means the wait was interrupted.
	ErrSignInStopped = errors.New("credentials: the sign-in wait was stopped")
)

// SignInFromBrowser files the browser's X session, waiting for one to appear.
//
// This is what `trove auth` runs. The first read is immediate, so a machine whose
// browser is already signed in needs no wait at all. When that read fails, openBrowser
// (if given) is called once to send the reader to X's sign-in page, and the profiles
// are read again every interval until a session appears or ctx ends.
//
// A profile the system refuses to let this process read is not retried. The denial is a
// privacy setting; waiting cannot change it, and the reader needs to hear it while they
// are still looking at the terminal.
//
// It reports whether the stored pair changed, the same as
// RefreshXCredentialsFromBrowser: signing in with the session that is already stored
// leaves the keychain untouched rather than rewriting it every run.
func (s *Store) SignInFromBrowser(ctx context.Context, interval time.Duration, openBrowser func() error) (bool, error) {
	if interval <= 0 {
		interval = SignInInterval
	}

	found, err := s.browserCookies(ctx)
	if err != nil {
		if errors.Is(err, ErrBrowserPermission) {
			return false, err
		}
		if openBrowser != nil {
			if openErr := openBrowser(); openErr != nil {
				return false, fmt.Errorf("open the sign-in page: %w", openErr)
			}
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for err != nil {
			select {
			case <-ctx.Done():
				// Carry the last read's reason: "no session in any browser" and "the
				// profiles could not be decrypted" are different sentences to the person
				// waiting, and the wait alone says nothing about which one it was.
				if errors.Is(ctx.Err(), context.Canceled) {
					return false, fmt.Errorf("%w: %v", ErrSignInStopped, err)
				}
				return false, fmt.Errorf("%w: %v", ErrSignInTimedOut, err)
			case <-ticker.C:
				found, err = s.browserCookies(ctx)
				if err != nil && errors.Is(err, ErrBrowserPermission) {
					return false, err
				}
			}
		}
	}
	return s.saveIfChanged(ctx, found)
}

// saveIfChanged stores a browser session unless it is the pair already stored.
func (s *Store) saveIfChanged(ctx context.Context, found xsource.Credentials) (bool, error) {
	stored, storedErr := s.XCredentials(ctx)
	if storedErr == nil && stored.AuthToken == found.AuthToken && stored.CSRFToken == found.CSRFToken {
		return false, nil
	}
	// A store that would not open is not a store with nothing in it. Writing now
	// would either fail or replace a pair this process could not read, so the
	// refresh reports the store instead of trying; the caller's sentence is then
	// "the Keychain could not be read", which is the truth.
	if storedErr != nil &&
		!errors.Is(storedErr, ErrNoCredential) &&
		!errors.Is(storedErr, ErrIncompleteCredentials) {
		return false, fmt.Errorf("credentials: refresh: %w", storedErr)
	}
	if err := s.SaveXCredentials(ctx, found); err != nil {
		return false, err
	}
	return true, nil
}
