package credentials

import (
	"context"
	"errors"
	"testing"

	"github.com/taoquo/x-master/apps/macos/internal/xsource"
)

// TestPermissionDenialIsToldApartFromNoCookies covers the two failures that both look like
// "found nothing" from the caller's side.
//
// On a machine that has not granted Full Disk Access, sweetcookie cannot even enumerate Chrome, so
// it reports "Chrome cookie store not found" for that browser and an "operation not permitted"
// warning for the one whose file it tried to open. Only the second is actionable, and telling a
// user "no cookies found, sign in to x.com" when the real problem is a permission would send them
// to fix the wrong thing.
func TestPermissionDenialIsToldApartFromNoCookies(t *testing.T) {
	if got := permissionDeniedWarning([]string{
		"sweetcookie: Chrome cookie store not found",
		"sweetcookie: Safari read failed: open /Users/x/Library/Cookies: operation not permitted",
	}); got == "" {
		t.Error("an 'operation not permitted' warning was not recognised as a permission problem")
	}
	if got := permissionDeniedWarning([]string{
		"sweetcookie: Chrome cookie store not found",
		"sweetcookie: Firefox cookie store not found",
	}); got != "" {
		t.Errorf("plain absence was reported as a permission problem: %q", got)
	}
	if got := permissionDeniedWarning(nil); got != "" {
		t.Error("no warnings produced a permission problem")
	}
}

// TestRefreshAdoptsOnlyADifferentBrowserSession covers the decision the sync retry hangs on.
//
// Refreshing has two jobs and both matter: adopting a fresh pair (so a rotated ct0 heals the
// session) and *not* claiming a change when the browser holds the same dead pair -- otherwise the
// sync would make one more doomed request on every failure.
func TestRefreshAdoptsOnlyADifferentBrowserSession(t *testing.T) {
	ctx := context.Background()

	t.Run("first credentials are adopted", func(t *testing.T) {
		store := New(newFakeKeychain())
		store.Browser = func(context.Context) (xsource.Credentials, error) {
			return xsource.Credentials{AuthToken: "auth-1", CSRFToken: "csrf-1"}, nil
		}
		changed, err := store.RefreshXCredentialsFromBrowser(ctx)
		if err != nil {
			t.Fatalf("RefreshXCredentialsFromBrowser: %v", err)
		}
		if !changed {
			t.Fatal("a first read reported no change")
		}
		stored, err := store.XCredentials(ctx)
		if err != nil || stored.AuthToken != "auth-1" || stored.CSRFToken != "csrf-1" {
			t.Fatalf("stored = %+v, %v", stored, err)
		}
	})

	t.Run("a rotated ct0 replaces the pair", func(t *testing.T) {
		store := New(newFakeKeychain())
		store.Browser = func(context.Context) (xsource.Credentials, error) {
			return xsource.Credentials{AuthToken: "auth-1", CSRFToken: "csrf-1"}, nil
		}
		if _, err := store.RefreshXCredentialsFromBrowser(ctx); err != nil {
			t.Fatalf("seed: %v", err)
		}
		store.Browser = func(context.Context) (xsource.Credentials, error) {
			return xsource.Credentials{AuthToken: "auth-1", CSRFToken: "csrf-2"}, nil
		}
		changed, err := store.RefreshXCredentialsFromBrowser(ctx)
		if err != nil || !changed {
			t.Fatalf("changed = %v, err = %v", changed, err)
		}
		stored, _ := store.XCredentials(ctx)
		if stored.CSRFToken != "csrf-2" {
			t.Errorf("CSRFToken = %q, want the rotated one", stored.CSRFToken)
		}
	})

	t.Run("the same pair is not a change", func(t *testing.T) {
		store := New(newFakeKeychain())
		store.Browser = func(context.Context) (xsource.Credentials, error) {
			return xsource.Credentials{AuthToken: "auth-1", CSRFToken: "csrf-1"}, nil
		}
		if _, err := store.RefreshXCredentialsFromBrowser(ctx); err != nil {
			t.Fatalf("seed: %v", err)
		}
		changed, err := store.RefreshXCredentialsFromBrowser(ctx)
		if err != nil {
			t.Fatalf("RefreshXCredentialsFromBrowser: %v", err)
		}
		if changed {
			t.Error("an unchanged browser pair was reported as a change")
		}
	})

	t.Run("a denied browser leaves the store alone", func(t *testing.T) {
		store := New(newFakeKeychain())
		store.Browser = func(context.Context) (xsource.Credentials, error) {
			return xsource.Credentials{}, ErrBrowserPermission
		}
		changed, err := store.RefreshXCredentialsFromBrowser(ctx)
		if changed {
			t.Error("a failed read reported a change")
		}
		if !errors.Is(err, ErrBrowserPermission) {
			t.Fatalf("error = %v, want ErrBrowserPermission", err)
		}
		if _, err := store.XCredentials(ctx); err == nil {
			t.Error("a failed read wrote credentials anyway")
		}
	})
}
