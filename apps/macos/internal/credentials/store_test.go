package credentials

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/taoquo/x-master/apps/macos/internal/credentials/keychain"
	"github.com/taoquo/x-master/apps/macos/internal/xsource"
)

// fakeKeychain is an in-memory keychain.
type fakeKeychain struct {
	values map[string]string
	failOn map[string]error
}

func newFakeKeychain(pairs ...string) *fakeKeychain {
	f := &fakeKeychain{values: map[string]string{}, failOn: map[string]error{}}
	for i := 0; i+1 < len(pairs); i += 2 {
		f.values[pairs[i]] = pairs[i+1]
	}
	return f
}

func (f *fakeKeychain) Set(_ context.Context, account, secret string) error {
	if err, ok := f.failOn[account]; ok {
		return err
	}
	f.values[account] = secret
	return nil
}

func (f *fakeKeychain) Get(_ context.Context, account string) (string, error) {
	if err, ok := f.failOn[account]; ok {
		return "", err
	}
	value, present := f.values[account]
	if !present {
		return "", keychain.ErrNotFound
	}
	return value, nil
}

func (f *fakeKeychain) Delete(_ context.Context, account string) error {
	if err, ok := f.failOn[account]; ok {
		return err
	}
	if _, present := f.values[account]; !present {
		return keychain.ErrNotFound
	}
	delete(f.values, account)
	return nil
}

func TestXCredentialsRequiresBothHalves(t *testing.T) {
	ctx := context.Background()

	t.Run("both present", func(t *testing.T) {
		store := New(newFakeKeychain(
			keychain.AccountXAuthToken, "auth-1",
			keychain.AccountXCSRFToken, "csrf-1",
		))
		got, err := store.XCredentials(ctx)
		if err != nil {
			t.Fatalf("XCredentials: %v", err)
		}
		if got.AuthToken != "auth-1" || got.CSRFToken != "csrf-1" {
			t.Errorf("credentials = %+v", got)
		}
	})

	t.Run("missing entirely", func(t *testing.T) {
		store := New(newFakeKeychain())
		if _, err := store.XCredentials(ctx); !errors.Is(err, ErrNoCredential) {
			t.Errorf("err = %v, want ErrNoCredential", err)
		}
	})

	t.Run("half configured", func(t *testing.T) {
		// A partial pair can never authenticate, so it must not look usable.
		store := New(newFakeKeychain(keychain.AccountXAuthToken, "auth-1"))
		if _, err := store.XCredentials(ctx); !errors.Is(err, ErrNoCredential) {
			t.Errorf("err = %v, want ErrNoCredential", err)
		}
	})

	t.Run("blank value counts as missing", func(t *testing.T) {
		store := New(newFakeKeychain(
			keychain.AccountXAuthToken, "   ",
			keychain.AccountXCSRFToken, "csrf-1",
		))
		if _, err := store.XCredentials(ctx); !errors.Is(err, ErrIncompleteCredentials) {
			t.Errorf("err = %v, want ErrIncompleteCredentials", err)
		}
	})
}

func TestSaveXCredentials(t *testing.T) {
	ctx := context.Background()

	t.Run("stores both and the optional extras", func(t *testing.T) {
		fake := newFakeKeychain()
		store := New(fake)

		err := store.SaveXCredentials(ctx, xsource.Credentials{
			AuthToken: "a", CSRFToken: "c", ExtraCookies: "guest_id=g1",
		})
		if err != nil {
			t.Fatalf("SaveXCredentials: %v", err)
		}
		if fake.values[keychain.AccountXAuthToken] != "a" || fake.values[keychain.AccountXCSRFToken] != "c" {
			t.Errorf("stored = %v", fake.values)
		}
		if fake.values[keychain.AccountXExtraCookies] != "guest_id=g1" {
			t.Errorf("extras = %q", fake.values[keychain.AccountXExtraCookies])
		}
	})

	t.Run("clearing the extras removes the old value", func(t *testing.T) {
		// Otherwise a stale cookie keeps being sent after the user removes it.
		fake := newFakeKeychain(
			keychain.AccountXAuthToken, "old",
			keychain.AccountXCSRFToken, "old",
			keychain.AccountXExtraCookies, "stale=1",
		)
		store := New(fake)

		if err := store.SaveXCredentials(ctx, xsource.Credentials{AuthToken: "a", CSRFToken: "c"}); err != nil {
			t.Fatalf("SaveXCredentials: %v", err)
		}
		if _, present := fake.values[keychain.AccountXExtraCookies]; present {
			t.Error("the stale extras must be removed")
		}
	})

	t.Run("refuses an incomplete pair", func(t *testing.T) {
		store := New(newFakeKeychain())
		if err := store.SaveXCredentials(ctx, xsource.Credentials{AuthToken: "a"}); !errors.Is(err, ErrIncompleteCredentials) {
			t.Errorf("err = %v, want ErrIncompleteCredentials", err)
		}
	})
}

func TestDeleteXCredentialsToleratesMissing(t *testing.T) {
	// Forgetting credentials that were never stored is a no-op, not an error.
	store := New(newFakeKeychain(keychain.AccountXAuthToken, "a"))
	if err := store.DeleteXCredentials(context.Background()); err != nil {
		t.Errorf("DeleteXCredentials: %v", err)
	}
}

func TestLLMCredentialLegacyFallbackIsOptIn(t *testing.T) {
	ctx := context.Background()

	t.Run("the provider's own key wins", func(t *testing.T) {
		store := New(newFakeKeychain(
			keychain.AccountLLM("openai"), "own-key",
			legacyLLMKeyAccount, "legacy-key",
		))
		got, err := store.LLMCredential(ctx, "openai", true)
		if err != nil {
			t.Fatalf("LLMCredential: %v", err)
		}
		if got.APIKey != "own-key" || got.Legacy {
			t.Errorf("got %+v, want the provider's own key", got)
		}
	})

	t.Run("the legacy key fills in for the active provider", func(t *testing.T) {
		store := New(newFakeKeychain(legacyLLMKeyAccount, "legacy-key"))
		got, err := store.LLMCredential(ctx, "openai", true)
		if err != nil {
			t.Fatalf("LLMCredential: %v", err)
		}
		if got.APIKey != "legacy-key" || !got.Legacy {
			t.Errorf("got %+v, want the legacy key marked as such", got)
		}
	})

	// The important asymmetry: the old single key belonged to whichever provider
	// was configured, so offering it to every provider would send one vendor's
	// key to another.
	t.Run("the legacy key is not offered when not allowed", func(t *testing.T) {
		store := New(newFakeKeychain(legacyLLMKeyAccount, "legacy-key"))
		got, err := store.LLMCredential(ctx, "anthropic", false)
		if err != nil {
			t.Fatalf("LLMCredential: %v", err)
		}
		if !got.IsEmpty() {
			t.Errorf("got %+v, want nothing for a non-active provider", got)
		}
	})

	t.Run("no key at all is not an error", func(t *testing.T) {
		store := New(newFakeKeychain())
		got, err := store.LLMCredential(ctx, "openai", true)
		if err != nil {
			t.Fatalf("LLMCredential: %v", err)
		}
		if !got.IsEmpty() {
			t.Errorf("got %+v, want empty", got)
		}
	})

	t.Run("a blank provider is refused", func(t *testing.T) {
		store := New(newFakeKeychain())
		if _, err := store.LLMCredential(ctx, "  ", true); err == nil {
			t.Error("a blank provider id must be refused")
		}
	})
}

func TestSaveAndDeleteLLMKey(t *testing.T) {
	ctx := context.Background()

	t.Run("saves and trims", func(t *testing.T) {
		fake := newFakeKeychain()
		store := New(fake)
		if err := store.SaveLLMKey(ctx, "openai", "  sk-key  "); err != nil {
			t.Fatalf("SaveLLMKey: %v", err)
		}
		if fake.values[keychain.AccountLLM("openai")] != "sk-key" {
			t.Errorf("stored = %q", fake.values[keychain.AccountLLM("openai")])
		}
	})

	// A settings file from a newer build may name a provider this build does not
	// know; refusing would lose the key.
	t.Run("an unknown provider is accepted", func(t *testing.T) {
		fake := newFakeKeychain()
		store := New(fake)
		if err := store.SaveLLMKey(ctx, "provider-from-the-future", "sk-key"); err != nil {
			t.Errorf("SaveLLMKey: %v", err)
		}
	})

	t.Run("rejects blanks", func(t *testing.T) {
		store := New(newFakeKeychain())
		if err := store.SaveLLMKey(ctx, "", "key"); err == nil {
			t.Error("a blank provider must be refused")
		}
		if err := store.SaveLLMKey(ctx, "openai", "   "); err == nil {
			t.Error("a blank key must be refused")
		}
	})

	t.Run("deleting a key that is absent is a no-op", func(t *testing.T) {
		store := New(newFakeKeychain())
		if err := store.DeleteLLMKey(ctx, "openai"); err != nil {
			t.Errorf("DeleteLLMKey: %v", err)
		}
	})
}

func TestMigrateLegacyKey(t *testing.T) {
	ctx := context.Background()

	t.Run("moves the key to the active provider and clears the legacy slot", func(t *testing.T) {
		fake := newFakeKeychain(legacyLLMKeyAccount, "legacy-key")
		store := New(fake)

		migrated, err := store.MigrateLegacyKey(ctx, "openai")
		if err != nil {
			t.Fatalf("MigrateLegacyKey: %v", err)
		}
		if !migrated {
			t.Error("the key should have been migrated")
		}
		if fake.values[keychain.AccountLLM("openai")] != "legacy-key" {
			t.Errorf("active provider key = %q", fake.values[keychain.AccountLLM("openai")])
		}
		// Removing the legacy slot is what stops the fallback applying forever.
		if _, present := fake.values[legacyLLMKeyAccount]; present {
			t.Error("the legacy slot must be cleared once migrated")
		}
	})

	t.Run("does not overwrite a key the provider already has", func(t *testing.T) {
		fake := newFakeKeychain(
			legacyLLMKeyAccount, "legacy-key",
			keychain.AccountLLM("openai"), "own-key",
		)
		store := New(fake)

		migrated, err := store.MigrateLegacyKey(ctx, "openai")
		if err != nil {
			t.Fatalf("MigrateLegacyKey: %v", err)
		}
		if migrated {
			t.Error("nothing should be moved when the provider already has a key")
		}
		if fake.values[keychain.AccountLLM("openai")] != "own-key" {
			t.Error("the provider's own key must not be overwritten")
		}
		// The redundant legacy value is still cleared.
		if _, present := fake.values[legacyLLMKeyAccount]; present {
			t.Error("the redundant legacy value should be dropped")
		}
	})

	t.Run("nothing to migrate", func(t *testing.T) {
		store := New(newFakeKeychain())
		migrated, err := store.MigrateLegacyKey(ctx, "openai")
		if err != nil {
			t.Fatalf("MigrateLegacyKey: %v", err)
		}
		if migrated {
			t.Error("nothing should be reported when there is no legacy key")
		}
	})
}

// TestAccountsCoversEverything pins that a credential sweep can find every
// secret this app created, so "reset local data" leaves nothing behind.
func TestAccountsCoversEverything(t *testing.T) {
	accounts := Accounts([]string{"openai", "anthropic"})

	wanted := []string{
		keychain.AccountXAuthToken,
		keychain.AccountXCSRFToken,
		keychain.AccountXExtraCookies,
		legacyLLMKeyAccount,
		keychain.AccountLLM("openai"),
		keychain.AccountLLM("anthropic"),
	}
	for _, want := range wanted {
		found := false
		for _, account := range accounts {
			if account == want {
				found = true
			}
		}
		if !found {
			t.Errorf("account %q is missing from the sweep list", want)
		}
	}

	// A blank provider must not produce a bogus account name.
	for _, account := range Accounts([]string{"  "}) {
		if strings.HasSuffix(account, "llm.") {
			t.Errorf("a blank provider produced %q", account)
		}
	}
}

// TestDeleteAllForgetsEverything covers the reset's promise.
//
// The source clears its secrets on reset and says why: the API key lives under its own
// storage key, so it used to survive a reset the dialog described as total. This walks
// every account this package writes and checks none is left.
func TestDeleteAllForgetsEverything(t *testing.T) {
	ctx := context.Background()
	keychain := newFakeKeychain()
	store := New(keychain)

	// An X credential, a provider key and the legacy key.
	if err := store.SaveXCredentials(ctx, xsource.Credentials{AuthToken: "a", CSRFToken: "b"}); err != nil {
		t.Fatalf("SaveXCredentials: %v", err)
	}
	if err := store.SaveLLMKey(ctx, "openai", "sk-one"); err != nil {
		t.Fatalf("SaveLLMKey: %v", err)
	}
	if err := keychain.Set(ctx, legacyLLMKeyAccount, "sk-legacy"); err != nil {
		t.Fatalf("Set legacy: %v", err)
	}

	if err := store.DeleteAll(ctx, []string{"openai", "anthropic", "  "}); err != nil {
		t.Fatalf("DeleteAll: %v", err)
	}

	if len(keychain.values) != 0 {
		t.Errorf("accounts survived the reset: %v", keychain.values)
	}

	// Running it again is not an error, which is what makes the reset idempotent.
	if err := store.DeleteAll(ctx, []string{"openai"}); err != nil {
		t.Errorf("a second clear failed: %v", err)
	}
}
