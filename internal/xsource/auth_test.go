package xsource

import (
	"errors"
	"testing"
)

// The values below are fixtures, not credentials: every case asserts that the
// parser recovers the pair, so the strings have to be recognisable at a glance.
const (
	fixtureAuthToken = "auth-token-fixture"
	fixtureCSRFToken = "csrf-token-fixture"
)

// TestParseCredentialPasteReadsEveryShapeAPersonCopies covers the one field the
// settings panel now shows.
//
// The two-field form assumed a DevTools excursion per value. What a reader
// actually has on the clipboard is one blob, and each source writes it
// differently; every case below is a copy that used to require taking apart by
// hand.
func TestParseCredentialPasteReadsEveryShapeAPersonCopies(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{
			name:  "cookie header",
			input: "auth_token=" + fixtureAuthToken + "; ct0=" + fixtureCSRFToken + "; guest_id=v1%3A123",
		},
		{
			name:  "header with its name",
			input: "Cookie: auth_token=" + fixtureAuthToken + "; ct0=" + fixtureCSRFToken,
		},
		{
			name: "curl command",
			input: `curl 'https://x.com/i/api/graphql/x/Bookmarks?variables=%7B%7D' ` +
				`-H 'authorization: Bearer AAAAA' ` +
				`-H 'cookie: auth_token=` + fixtureAuthToken + `; ct0=` + fixtureCSRFToken + `' --compressed`,
		},
		{
			name: "cookie export json",
			input: `[{"domain":".x.com","name":"auth_token","value":"` + fixtureAuthToken + `"},` +
				`{"domain":".x.com","name":"ct0","value":"` + fixtureCSRFToken + `"}]`,
		},
		{
			name:  "json with the keys reversed",
			input: `[{"value":"` + fixtureAuthToken + `","name":"auth_token"},{"value":"` + fixtureCSRFToken + `","name":"ct0"}]`,
		},
		{
			name:  "document.cookie",
			input: `"auth_token=` + fixtureAuthToken + `; ct0=` + fixtureCSRFToken + `"`,
		},
		{
			name:  "one pair per line",
			input: "auth_token=" + fixtureAuthToken + "\nct0=" + fixtureCSRFToken + "\n",
		},
		{
			name:  "url query",
			input: "https://x.com/?auth_token=" + fixtureAuthToken + "&ct0=" + fixtureCSRFToken,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			credentials, err := ParseCredentialPaste(tc.input)
			if err != nil {
				t.Fatalf("ParseCredentialPaste: %v", err)
			}
			if credentials.AuthToken != fixtureAuthToken {
				t.Errorf("AuthToken = %q, want %q", credentials.AuthToken, fixtureAuthToken)
			}
			if credentials.CSRFToken != fixtureCSRFToken {
				t.Errorf("CSRFToken = %q, want %q", credentials.CSRFToken, fixtureCSRFToken)
			}
		})
	}
}

// TestParseCredentialPasteNamesTheHalfThatIsMissing keeps the two failures apart.
//
// "You copied something that is not a cookie" and "you copied the cookie header
// but it has no ct0" need different corrections, and one message for both would
// leave the reader comparing their paste with a screenshot.
func TestParseCredentialPasteNamesTheHalfThatIsMissing(t *testing.T) {
	credentials, err := ParseCredentialPaste("auth_token=" + fixtureAuthToken + "; guest_id=1")
	if !errors.Is(err, ErrMissingCSRFToken) {
		t.Fatalf("error = %v, want ErrMissingCSRFToken", err)
	}
	if credentials.AuthToken != fixtureAuthToken {
		t.Errorf("the half that was found was dropped: %+v", credentials)
	}

	credentials, err = ParseCredentialPaste("ct0=" + fixtureCSRFToken)
	if !errors.Is(err, ErrMissingAuthToken) {
		t.Fatalf("error = %v, want ErrMissingAuthToken", err)
	}
	if credentials.CSRFToken != fixtureCSRFToken {
		t.Errorf("the half that was found was dropped: %+v", credentials)
	}

	if _, err := ParseCredentialPaste("hello, this is my shopping list"); !errors.Is(err, ErrNoCookiesPasted) {
		t.Fatalf("error = %v, want ErrNoCookiesPasted", err)
	}
	if _, err := ParseCredentialPaste(""); !errors.Is(err, ErrNoCookiesPasted) {
		t.Fatalf("an empty paste reported %v, want ErrNoCookiesPasted", err)
	}
}

// TestParseCredentialPasteDoesNotMatchInsideAnotherName is the boundary case.
//
// A cookie called `xauth_token` must not be handed to X as the session. The
// leading character class is the only thing preventing it.
func TestParseCredentialPasteDoesNotMatchInsideAnotherName(t *testing.T) {
	credentials, err := ParseCredentialPaste("xauth_token=nope; auth_token=" + fixtureAuthToken +
		"; ct0=" + fixtureCSRFToken)
	if err != nil {
		t.Fatalf("ParseCredentialPaste: %v", err)
	}
	if credentials.AuthToken != fixtureAuthToken {
		t.Errorf("AuthToken = %q, want the real one", credentials.AuthToken)
	}
}

// TestParseCredentialPasteKeepsAValueWithAnEqualsSign covers the separator
// inside the value. Taking everything to the next semicolon is the right rule for
// ct0.
func TestParseCredentialPasteKeepsAValueWithAnEqualsSign(t *testing.T) {
	credentials, err := ParseCredentialPaste("auth_token=abc=def; ct0=" + fixtureCSRFToken)
	if err != nil {
		t.Fatalf("ParseCredentialPaste: %v", err)
	}
	if credentials.AuthToken != "abc=def" {
		t.Errorf("AuthToken = %q, want abc=def", credentials.AuthToken)
	}
}
