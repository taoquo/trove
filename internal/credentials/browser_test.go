package credentials

import (
	"context"
	"errors"
	"testing"

	"github.com/steipete/sweetcookie"

	"github.com/taoquo/trove/internal/xsource"
)

// browserCookie builds one cookie as the library would report it, with the source
// that says which profile it came from.
func browserCookie(browser sweetcookie.Browser, profile, name, value string) sweetcookie.Cookie {
	return sweetcookie.Cookie{
		Name:   name,
		Value:  value,
		Source: sweetcookie.Source{Browser: browser, Profile: profile},
	}
}

// TestAPairComesFromOneProfile is the rule the merged read got wrong.
//
// X's two cookies authenticate as a pair. Taking auth_token from Chrome and ct0
// from Safari produces a set that no single session ever had, and the failure the
// user sees is a CSRF rejection with no explanation.
func TestAPairComesFromOneProfile(t *testing.T) {
	t.Run("the first complete profile wins", func(t *testing.T) {
		found, ok := pairFromProfiles([]sweetcookie.Cookie{
			// Interleaved on purpose: the reader does not group for us.
			browserCookie(sweetcookie.BrowserChrome, "Default", "auth_token", "chrome-auth"),
			browserCookie(sweetcookie.BrowserSafari, "", "auth_token", "safari-auth"),
			browserCookie(sweetcookie.BrowserChrome, "Default", "ct0", "chrome-csrf"),
			browserCookie(sweetcookie.BrowserSafari, "", "ct0", "safari-csrf"),
		})
		if !ok {
			t.Fatal("two complete profiles produced no pair")
		}
		if found.AuthToken != "chrome-auth" || found.CSRFToken != "chrome-csrf" {
			t.Errorf("pair = %+v, want Chrome's two", found)
		}
	})

	t.Run("two halves are not a pair", func(t *testing.T) {
		_, ok := pairFromProfiles([]sweetcookie.Cookie{
			browserCookie(sweetcookie.BrowserChrome, "Default", "auth_token", "chrome-auth"),
			browserCookie(sweetcookie.BrowserSafari, "", "ct0", "safari-csrf"),
		})
		if ok {
			t.Fatal("a pair assembled from two profiles was accepted")
		}
	})

	t.Run("a blank value is not a cookie", func(t *testing.T) {
		_, ok := pairFromProfiles([]sweetcookie.Cookie{
			browserCookie(sweetcookie.BrowserChrome, "Default", "auth_token", "  "),
			browserCookie(sweetcookie.BrowserChrome, "Default", "ct0", "chrome-csrf"),
		})
		if ok {
			t.Fatal("a blank auth_token completed a pair")
		}
	})

	t.Run("a second profile still counts when the first is half filled", func(t *testing.T) {
		found, ok := pairFromProfiles([]sweetcookie.Cookie{
			browserCookie(sweetcookie.BrowserChrome, "Profile 1", "auth_token", "one"),
			browserCookie(sweetcookie.BrowserChrome, "Profile 2", "auth_token", "two"),
			browserCookie(sweetcookie.BrowserChrome, "Profile 2", "ct0", "csrf-two"),
		})
		if !ok {
			t.Fatal("a complete second profile was not found")
		}
		if found.AuthToken != "two" || found.CSRFToken != "csrf-two" {
			t.Errorf("pair = %+v, want Profile 2's two", found)
		}
	})
}

// TestAHalfPairIsRejectedAndNotWritten is the negative sample end to end: the
// reader that produces it is the real BrowserCookies, and the store must neither
// accept the result nor write anything.
func TestAHalfPairIsRejectedAndNotWritten(t *testing.T) {
	original := loadBrowserCookies
	t.Cleanup(func() { loadBrowserCookies = original })
	loadBrowserCookies = func(context.Context, sweetcookie.Options) (sweetcookie.Result, error) {
		return sweetcookie.Result{Cookies: []sweetcookie.Cookie{
			browserCookie(sweetcookie.BrowserChrome, "Default", "auth_token", "chrome-auth"),
			browserCookie(sweetcookie.BrowserSafari, "", "ct0", "safari-csrf"),
		}}, nil
	}

	keychain := newFakeKeychain()
	store := &Store{Keychain: keychain}
	changed, err := store.RefreshXCredentialsFromBrowser(context.Background())
	if err == nil {
		t.Fatal("a half pair was accepted")
	}
	if changed {
		t.Error("a half pair reported a change")
	}
	if !errors.Is(err, ErrBrowserNoCookies) {
		t.Errorf("err = %v, want ErrBrowserNoCookies", err)
	}
	if !errors.Is(err, ErrIncompleteCredentials) {
		t.Errorf("err = %v, want it to wrap ErrIncompleteCredentials", err)
	}
	if len(keychain.values) != 0 {
		t.Errorf("a half pair was written: %v", keychain.values)
	}
}

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
