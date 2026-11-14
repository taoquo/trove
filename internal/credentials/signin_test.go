package credentials

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/taoquo/trove/internal/credentials/keychain"
	"github.com/taoquo/trove/internal/xsource"
)

// signInBrowser returns a reader that fails its first misses calls, then hands back a
// session, and counts the reads it was asked for.
func signInBrowser(misses int, reads *int) BrowserReader {
	return func(context.Context) (xsource.Credentials, error) {
		*reads++
		if *reads <= misses {
			return xsource.Credentials{}, ErrBrowserNoCookies
		}
		return xsource.Credentials{AuthToken: "auth-1", CSRFToken: "csrf-1"}, nil
	}
}

// TestSignInWaitsForTheSessionToAppear is the whole point of `trove auth`: the first
// read of a fresh machine finds nothing, so the reader is sent to X once and the profiles
// are read again until the sign-in lands.
func TestSignInWaitsForTheSessionToAppear(t *testing.T) {
	kc := newFakeKeychain()
	var reads, opens int
	store := &Store{Keychain: kc, Browser: signInBrowser(2, &reads)}

	changed, err := store.SignInFromBrowser(context.Background(), time.Millisecond, func() error {
		opens++
		return nil
	})
	if err != nil {
		t.Fatalf("SignInFromBrowser: %v", err)
	}
	if !changed {
		t.Error("the first sign-in reported no change, so nothing was saved")
	}
	if opens != 1 {
		t.Errorf("the browser was opened %d times, want exactly once", opens)
	}
	if got := kc.values[keychain.AccountXAuthToken]; got != "auth-1" {
		t.Errorf("stored auth_token = %q, want the browser's", got)
	}
	if got := kc.values[keychain.AccountXCSRFToken]; got != "csrf-1" {
		t.Errorf("stored ct0 = %q, want the browser's", got)
	}
}

// TestSignInTakesASessionThatIsAlreadyThere keeps the command from opening a browser
// window for someone who is already signed in: the first read is immediate.
func TestSignInTakesASessionThatIsAlreadyThere(t *testing.T) {
	kc := newFakeKeychain()
	var reads, opens int
	store := &Store{Keychain: kc, Browser: signInBrowser(0, &reads)}

	if _, err := store.SignInFromBrowser(context.Background(), time.Millisecond, func() error {
		opens++
		return nil
	}); err != nil {
		t.Fatalf("SignInFromBrowser: %v", err)
	}
	if opens != 0 {
		t.Errorf("the browser was opened %d times although a session was found", opens)
	}
	if reads != 1 {
		t.Errorf("%d reads, want the one that found the session", reads)
	}
}

// countingKeychain is the fake with a tally of the writes, which is how "the store did
// not touch it" is told apart from "the store wrote the same value back".
type countingKeychain struct {
	*fakeKeychain
	sets map[string]int
}

func (c *countingKeychain) Set(ctx context.Context, account, secret string) error {
	c.sets[account]++
	return c.fakeKeychain.Set(ctx, account, secret)
}

// TestSignInLeavesAnIdenticalSessionAlone: the tokens do not change between runs, so a
// second sign-in must not rewrite them.
func TestSignInLeavesAnIdenticalSessionAlone(t *testing.T) {
	kc := &countingKeychain{
		fakeKeychain: newFakeKeychain(
			keychain.AccountXAuthToken, "auth-1",
			keychain.AccountXCSRFToken, "csrf-1",
		),
		sets: map[string]int{},
	}
	var reads int
	store := &Store{Keychain: kc, Browser: signInBrowser(0, &reads)}

	changed, err := store.SignInFromBrowser(context.Background(), time.Millisecond, nil)
	if err != nil {
		t.Fatalf("SignInFromBrowser: %v", err)
	}
	if changed {
		t.Error("an unchanged session reported a change")
	}
	if len(kc.sets) != 0 {
		t.Errorf("the identical session was rewritten %v", kc.sets)
	}
}

// TestSignInStopsAtAPermissionDenial: waiting cannot grant a privacy permission, and a
// command that sat quiet for five minutes before saying so would have wasted the reader's
// time. The denial is also the one failure whose fix is a sentence, not a retry.
func TestSignInStopsAtAPermissionDenial(t *testing.T) {
	var opens, reads int
	store := &Store{
		Keychain: newFakeKeychain(),
		Browser: func(context.Context) (xsource.Credentials, error) {
			reads++
			return xsource.Credentials{}, ErrBrowserPermission
		},
	}

	_, err := store.SignInFromBrowser(context.Background(), time.Hour, func() error {
		opens++
		return nil
	})
	if !errors.Is(err, ErrBrowserPermission) {
		t.Fatalf("SignInFromBrowser error = %v, want the permission denial", err)
	}
	if opens != 0 {
		t.Errorf("the browser was opened %d times despite the denial", opens)
	}
	if reads != 1 {
		t.Errorf("%d reads, want the denial to stop the wait on the first", reads)
	}
}

// TestSignInSaysWhyTheWaitEnded: the timeout alone does not tell the person waiting
// whether the browser was never signed in or the profile could not be read, so the last
// read's reason travels with the error.
func TestSignInSaysWhyTheWaitEnded(t *testing.T) {
	store := &Store{
		Keychain: newFakeKeychain(),
		Browser: func(context.Context) (xsource.Credentials, error) {
			return xsource.Credentials{}, ErrBrowserNoCookies
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	_, err := store.SignInFromBrowser(ctx, time.Millisecond, nil)
	if !errors.Is(err, ErrSignInTimedOut) {
		t.Fatalf("SignInFromBrowser error = %v, want the timeout", err)
	}
	if !strings.Contains(err.Error(), ErrBrowserNoCookies.Error()) {
		t.Errorf("the timeout does not say why it waited: %v", err)
	}
	if store.Keychain.(*fakeKeychain).values[keychain.AccountXAuthToken] != "" {
		t.Error("a timed-out sign-in stored something")
	}
}

// TestSignInReportsAnInterruption distinguishes Ctrl-C from the clock running out: the
// first is the reader's decision and does not read as a failure of the browser.
func TestSignInReportsAnInterruption(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store := &Store{
		Keychain: newFakeKeychain(),
		Browser: func(context.Context) (xsource.Credentials, error) {
			return xsource.Credentials{}, ErrBrowserNoCookies
		},
	}

	_, err := store.SignInFromBrowser(ctx, time.Millisecond, func() error {
		cancel()
		return nil
	})
	if !errors.Is(err, ErrSignInStopped) {
		t.Fatalf("SignInFromBrowser error = %v, want the interruption", err)
	}
}
