// Browser cookie reading.
//
// A desktop app has no browser API to ask, and the two cookies X's own sign-in
// sets are the only way in. Reading them out of a signed-in profile is what
// yt-dlp and gallery-dl do (`--cookies-from-browser`), and
// github.com/steipete/sweetcookie is the Go implementation of the same idea: it
// snapshots the profile's cookie database, decrypts it with the browser's Safe
// Storage password from the Keychain, and filters to the requested origin.
//
// It is best-effort by design. macOS protects those directories with TCC, and
// Windows browser encryption can require an app-bound key, so a read can fail
// with "operation not permitted" or a decryption error that the app cannot fix.
// The failure is classified rather than flattened, because "the system is
// blocking this" and "you are not signed in" are fixed in different places.
package credentials

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/steipete/sweetcookie"

	"github.com/taoquo/trove/internal/xsource"
)

// Browser read failures.
//
// They are sentinels so the UI can pick a sentence, and they wrap the library's
// own warning so a diagnostic log still says which browser it was.
var (
	// ErrBrowserPermission means the system refused this app access to the
	// profile directory. The fix is a privacy setting, and no retry will help.
	ErrBrowserPermission = errors.New("credentials: the system denied access to the browser profiles")
	// ErrBrowserNoCookies means a profile was readable but had no X session.
	ErrBrowserNoCookies = errors.New("credentials: no signed-in X session in any browser")
	// ErrBrowserUnavailable is everything else: the library failed outright.
	ErrBrowserUnavailable = errors.New("credentials: the browser could not be read")
)

// BrowserCookies reads auth_token and ct0 from a browser the user is signed in to.
func BrowserCookies(ctx context.Context) (xsource.Credentials, error) {
	result, err := sweetcookie.Get(ctx, sweetcookie.Options{
		URL:   "https://x.com/",
		Names: []string{"auth_token", "ct0"},
		Mode:  sweetcookie.ModeMerge,
	})
	if err != nil {
		return xsource.Credentials{}, fmt.Errorf("%w: %v", ErrBrowserUnavailable, err)
	}

	var found xsource.Credentials
	for _, cookie := range result.Cookies {
		switch cookie.Name {
		case "auth_token":
			found.AuthToken = cookie.Value
		case "ct0":
			found.CSRFToken = cookie.Value
		}
	}
	if found.AuthToken != "" && found.CSRFToken != "" {
		return found, nil
	}

	// Two different failures hide behind "no cookies", and they need different
	// answers. macOS refusing the directory is the common one on a fresh install,
	// and the library reports it per browser as an "operation not permitted"
	// warning while the browsers it could not even enumerate never appear at all
	// -- so the check is on the warnings rather than on the count of browsers
	// tried.
	if denied := permissionDeniedWarning(result.Warnings); denied != "" {
		return xsource.Credentials{}, fmt.Errorf("%w: %s", ErrBrowserPermission, denied)
	}
	return xsource.Credentials{}, fmt.Errorf("%w (%s)", ErrBrowserNoCookies, firstWarning(result.Warnings))
}

// permissionDeniedWarning reports the first warning that means the system refused to let this app
// read a browser's files, which is what a missing privacy permission looks like from here.
func permissionDeniedWarning(warnings []string) string {
	for _, w := range warnings {
		if strings.Contains(w, "operation not permitted") || strings.Contains(w, "permission denied") {
			return w
		}
	}
	return ""
}

// firstWarning picks the one line worth showing when no cookie was found.
func firstWarning(warnings []string) string {
	if len(warnings) == 0 {
		return ""
	}
	return warnings[0]
}
