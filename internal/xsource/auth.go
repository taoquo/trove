package xsource

import (
	"errors"
	"regexp"
	"strings"
)

// Credentials is what the user pastes from their own logged-in X session.
//
// The app never asks for an X password or a 2FA code, and never reads a
// browser's cookie store: the user supplies two cookie values explicitly and
// they are kept in the macOS Keychain.
type Credentials struct {
	// AuthToken is the value of the auth_token cookie.
	AuthToken string
	// CSRFToken is the value of the ct0 cookie, sent both as a cookie and as the
	// x-csrf-token header.
	CSRFToken string
	// ExtraCookies carries any additional cookies X turns out to require. It
	// exists because the minimal pair is an assumption that must be verified
	// against a real account, not a guarantee.
	ExtraCookies string
}

// The struct deliberately has no String method: a credential value that can be
// printed is a credential that ends up in a log.

// ErrMissingCredentials reports that the stored credentials are incomplete.
var ErrMissingCredentials = errors.New("xsource: credentials are incomplete")

// Validate reports whether the credentials can be used at all.
func (c Credentials) Validate() error {
	if strings.TrimSpace(c.AuthToken) == "" {
		return ErrMissingCredentials
	}
	if strings.TrimSpace(c.CSRFToken) == "" {
		return ErrMissingCredentials
	}
	return nil
}

// CookieHeader builds the Cookie header value.
//
// Only the cookies that are needed are sent, each exactly once, so a caller that
// supplies ct0 both as CSRFToken and inside ExtraCookies does not produce a
// duplicate.
func (c Credentials) CookieHeader() string {
	pairs := make([]string, 0, 3)
	seen := map[string]bool{}

	add := func(name, value string) {
		name = strings.TrimSpace(name)
		value = strings.TrimSpace(value)
		if name == "" || value == "" || seen[name] {
			return
		}
		seen[name] = true
		pairs = append(pairs, name+"="+value)
	}

	add("auth_token", c.AuthToken)
	add("ct0", c.CSRFToken)

	// ExtraCookies is a semicolon-separated list, the same shape a browser sends.
	for _, part := range strings.Split(c.ExtraCookies, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, value, found := strings.Cut(part, "=")
		if !found {
			continue
		}
		add(name, value)
	}

	return strings.Join(pairs, "; ")
}

// Errors a pasted credential blob can fail with.
//
// They are separate sentinels rather than one "invalid" error because the two
// cookies are copied from different places: "the paste has no ct0" tells the
// reader they copied the wrong half, while "neither" means they copied
// something that is not a cookie at all.
var (
	ErrNoCookiesPasted  = errors.New("xsource: the pasted text has neither auth_token nor ct0")
	ErrMissingAuthToken = errors.New("xsource: the pasted text has no auth_token")
	ErrMissingCSRFToken = errors.New("xsource: the pasted text has no ct0")
)

// ParseCredentialPaste reads the two cookies out of whatever the user copied.
//
// The two-field form assumed the reader would go to DevTools, open the cookie
// table and copy each value separately. What people actually copy is one thing:
// the request's Cookie header, a cURL command, a cookie-export JSON, or a
// document.cookie line. Every one of those carries the pair in a different
// shape, and asking the reader to take it apart is the friction this removes.
//
// The pair is recognised by name, never by position or length. X may change the
// shape of either value; it will not rename its own cookies.
func ParseCredentialPaste(input string) (Credentials, error) {
	text := strings.ReplaceAll(input, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	authToken := findCookieValue(text, "auth_token")
	csrfToken := findCookieValue(text, "ct0")

	credentials := Credentials{AuthToken: authToken, CSRFToken: csrfToken}
	switch {
	case authToken == "" && csrfToken == "":
		return Credentials{}, ErrNoCookiesPasted
	case authToken == "":
		return credentials, ErrMissingAuthToken
	case csrfToken == "":
		return credentials, ErrMissingCSRFToken
	}
	return credentials, nil
}

// findCookieValue looks for one cookie in the shapes a paste arrives in.
//
// Order matters only for readability: the first shape that matches wins, and a
// blob that contains both a header and an export names the same value twice.
func findCookieValue(text, name string) string {
	for _, pattern := range cookiePatterns(name) {
		if match := pattern.FindStringSubmatch(text); match != nil {
			if value := strings.TrimSpace(match[1]); value != "" {
				return value
			}
		}
	}
	return ""
}

// cookiePatterns are the three shapes worth accepting.
//
//   - `name=value`, which covers a Cookie header, a cURL -H argument and a
//     document.cookie line. The separator is also allowed to be a colon, so a
//     JSON object of plain pairs works too.
//   - `"name":"x","value":"y"`, the shape a cookie-export tool writes.
//   - `"value":"y","name":"x"`, the same object with its keys in the other order.
//
// The leading character class is what stops `xauth_token` and `my_ct0` from
// matching: a cookie name starts where the previous character is not part of a
// name.
func cookiePatterns(name string) []*regexp.Regexp {
	quoted := regexp.QuoteMeta(name)
	return []*regexp.Regexp{
		regexp.MustCompile(`(?:^|[^A-Za-z0-9_])` + quoted + `\s*[=:]\s*"?([^;"'\s,&}]+)`),
		regexp.MustCompile(`"name"\s*:\s*"` + quoted + `"\s*,\s*"value"\s*:\s*"([^"]*)"`),
		regexp.MustCompile(`"value"\s*:\s*"([^"]*)"\s*,\s*"name"\s*:\s*"` + quoted + `"`),
	}
}

// csrftokenPattern matches ct0 at the start of the header or immediately after a
// semicolon.
var csrftokenPattern = regexp.MustCompile(`(?:^|;\s*)ct0=([^;]+)`)

// ExtractCSRFToken pulls ct0 out of a Cookie header.
func ExtractCSRFToken(cookieHeader string) (string, error) {
	match := csrftokenPattern.FindStringSubmatch(cookieHeader)
	if match == nil {
		return "", errors.New("xsource: missing ct0 cookie")
	}
	return strings.TrimSpace(match[1]), nil
}
