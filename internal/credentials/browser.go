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
	// ErrBrowserNoCookies means a profile was readable but no single profile held a
	// complete X session. It wraps ErrIncompleteCredentials: the two cookies are one
	// credential, and a set assembled from two profiles cannot authenticate.
	ErrBrowserNoCookies = fmt.Errorf("%w: no signed-in X session in any browser", ErrIncompleteCredentials)
	// ErrBrowserUnavailable is everything else: the library failed outright.
	ErrBrowserUnavailable = errors.New("credentials: the browser could not be read")
)

// loadBrowserCookies is the library call, held in a variable so the pairing rule
// can be tested without a signed-in profile on the machine running the tests.
var loadBrowserCookies = func(ctx context.Context, options sweetcookie.Options) (sweetcookie.Result, error) {
	return sweetcookie.Get(ctx, options)
}

// BrowserCookies reads auth_token and ct0 from a browser the user is signed in to.
//
// The pair is taken from one profile. Merging by name looks equivalent and is not:
// with Chrome signed in to one account and Safari to another, the merged result
// held Chrome's auth_token beside Safari's ct0, and X rejected it with a CSRF
// mismatch the user could not explain. A half-filled profile is not a candidate.
func BrowserCookies(ctx context.Context) (xsource.Credentials, error) {
	result, err := loadBrowserCookies(ctx, sweetcookie.Options{
		URL:   "https://x.com/",
		Names: []string{"auth_token", "ct0"},
		// Merge so every profile is seen; the choice between them is made below,
		// per profile, rather than by the reader's name-level merge.
		Mode: sweetcookie.ModeMerge,
	})
	if err != nil {
		return xsource.Credentials{}, fmt.Errorf("%w: %v", ErrBrowserUnavailable, err)
	}

	if found, ok := pairFromProfiles(result.Cookies); ok {
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

// profileKey identifies one browser profile's cookie set.
//
// The store path is part of it because a browser can have profiles the reader names
// identically, and the container id because Firefox keeps separate jars per
// container inside one profile.
type profileKey struct {
	browser   sweetcookie.Browser
	profile   string
	storePath string
	container int
}

// pairFromProfiles returns the first complete {auth_token, ct0} in reader order.
//
// Reader order is browser priority: the default browser's profiles come first, so
// the first complete profile is the one the person is most likely signed in to.
// An empty value does not count as a half of the pair -- a cleared cookie is not a
// cookie.
func pairFromProfiles(cookies []sweetcookie.Cookie) (xsource.Credentials, bool) {
	order := make([]profileKey, 0, len(cookies))
	pairs := make(map[profileKey]xsource.Credentials, len(cookies))

	for _, cookie := range cookies {
		key := profileKey{
			browser:   cookie.Source.Browser,
			profile:   cookie.Source.Profile,
			storePath: cookie.Source.StorePath,
			container: cookie.Container.ID,
		}
		if _, seen := pairs[key]; !seen {
			order = append(order, key)
		}
		if strings.TrimSpace(cookie.Value) == "" {
			continue
		}
		pair := pairs[key]
		switch cookie.Name {
		case "auth_token":
			pair.AuthToken = cookie.Value
		case "ct0":
			pair.CSRFToken = cookie.Value
		}
		pairs[key] = pair
	}

	for _, key := range order {
		if pair := pairs[key]; pair.AuthToken != "" && pair.CSRFToken != "" {
			return pair, true
		}
	}
	return xsource.Credentials{}, false
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
