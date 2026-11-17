package credentials

import (
	"context"
	"errors"
	"testing"

	"github.com/taoquo/trove/internal/credentials/keychain"
	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/xsource"
)

// writeCountingKeychain refuses every read and counts writes.
//
// It is what a locked Keychain looks like from here: the secrets are in there, the
// process just cannot open the store, and a write would be a change made blind.
type writeCountingKeychain struct {
	sets int
}

func (k *writeCountingKeychain) Set(context.Context, string, string) error {
	k.sets++
	return nil
}

func (k *writeCountingKeychain) Get(context.Context, string) (string, error) {
	return "", keychain.ErrLocked
}

func (k *writeCountingKeychain) Delete(context.Context, string) error {
	return keychain.ErrNotFound
}

// TestAStoreFailureIsNotAnExpiredLogin is the assertion the plan starts from: the
// sync must be able to tell "there is no session" from "the session could not be
// read", because only the first one is fixed by supplying cookies.
func TestAStoreFailureIsNotAnExpiredLogin(t *testing.T) {
	ctx := context.Background()

	t.Run("a locked keychain", func(t *testing.T) {
		locked := newFakeKeychain()
		locked.failOn[keychain.AccountXAuthToken] = keychain.ErrLocked
		locked.failOn[keychain.AccountXCSRFToken] = keychain.ErrLocked

		_, err := New(locked).XCredentialsForSync(ctx)
		if err == nil {
			t.Fatal("a locked keychain read as a stored session")
		}
		if got := xsource.KindOf(err); got != domain.ErrCredentialStore {
			t.Errorf("kind = %q, want %q", got, domain.ErrCredentialStore)
		}
		if !errors.Is(err, ErrStoreLocked) {
			t.Errorf("err = %v, want it to wrap ErrStoreLocked", err)
		}
	})

	t.Run("nothing stored", func(t *testing.T) {
		_, err := New(newFakeKeychain()).XCredentialsForSync(ctx)
		if got := xsource.KindOf(err); got != domain.ErrUnauthorized {
			t.Errorf("kind = %q, want %q", got, domain.ErrUnauthorized)
		}
	})

	t.Run("half a pair", func(t *testing.T) {
		// Half a pair cannot authenticate, so it is the same fix as nothing stored.
		store := New(newFakeKeychain(keychain.AccountXAuthToken, "auth-1"))
		_, err := store.XCredentialsForSync(ctx)
		if got := xsource.KindOf(err); got != domain.ErrUnauthorized {
			t.Errorf("kind = %q, want %q", got, domain.ErrUnauthorized)
		}
	})
}

// TestARefreshDoesNotWriteOverAStoreItCouldNotRead is the second half: a browser
// that holds a live session must not be saved into a store this process cannot
// open. Doing so would either fail or replace a pair nobody could compare against.
func TestARefreshDoesNotWriteOverAStoreItCouldNotRead(t *testing.T) {
	locked := &writeCountingKeychain{}
	store := &Store{
		Keychain: locked,
		Browser: func(context.Context) (xsource.Credentials, error) {
			return xsource.Credentials{AuthToken: "auth-1", CSRFToken: "csrf-1"}, nil
		},
	}

	changed, err := store.RefreshXCredentialsFromBrowser(context.Background())
	if err == nil {
		t.Fatal("a refresh over a locked store reported success")
	}
	if changed {
		t.Error("a refresh over a locked store reported a change")
	}
	if !errors.Is(err, ErrStoreLocked) {
		t.Errorf("err = %v, want it to name the locked store", err)
	}
	if locked.sets != 0 {
		t.Errorf("wrote %d time(s) into a store it could not read", locked.sets)
	}
}

// TestAnEmptyStoreAcceptsARefresh is the positive control: the guard above must
// not stop the ordinary case, where a fresh install has nothing stored and the
// browser's session is what fills it.
func TestAnEmptyStoreAcceptsARefresh(t *testing.T) {
	kc := newFakeKeychain()
	store := &Store{
		Keychain: kc,
		Browser: func(context.Context) (xsource.Credentials, error) {
			return xsource.Credentials{AuthToken: "auth-1", CSRFToken: "csrf-1"}, nil
		},
	}

	changed, err := store.RefreshXCredentialsFromBrowser(context.Background())
	if err != nil {
		t.Fatalf("RefreshXCredentialsFromBrowser: %v", err)
	}
	if !changed {
		t.Fatal("the browser's session was not saved")
	}
	if got := kc.values[keychain.AccountXAuthToken]; got != "auth-1" {
		t.Errorf("stored auth_token = %q", got)
	}
}
