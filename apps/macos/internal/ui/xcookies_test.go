package ui

import (
	"context"
	"testing"

	"github.com/taoquo/x-master/apps/macos/internal/credentials"
	"github.com/taoquo/x-master/apps/macos/internal/credentials/keychain"
	"github.com/taoquo/x-master/apps/macos/internal/i18n"
)

// memKeychain is the Keychain interface over a map, so the round trip can be tested without
// touching the machine's real keychain -- which would need the user's password and would leave
// a credential behind.
type memKeychain struct{ items map[string]string }

func (m *memKeychain) Set(_ context.Context, account, secret string) error {
	if m.items == nil {
		m.items = map[string]string{}
	}
	m.items[account] = secret
	return nil
}

func (m *memKeychain) Get(_ context.Context, account string) (string, error) {
	if v, ok := m.items[account]; ok {
		return v, nil
	}
	return "", keychain.ErrNotFound
}

func (m *memKeychain) Delete(_ context.Context, account string) error {
	delete(m.items, account)
	return nil
}

// TestSavingXCookiesMakesSyncPossible covers the defect this was written for.
//
// Every sync failed with "credentials: nothing stored (x.auth_token)" because
// credentials.Store.SaveXCredentials was called from nowhere: the Keychain stayed empty and the
// app had no field to fill it. The assertion is the round trip -- save through the shell, then
// read the same two values back through the store the syncer uses.
func TestSavingXCookiesMakesSyncPossible(t *testing.T) {
	shell, _ := newShell(t)
	store := credentials.New(&memKeychain{})
	shell.UseCredentials(store)

	ctx := context.Background()

	// Before: nothing stored, and the shell knows it.
	shell.RefreshXCookieStatus(ctx)
	if shell.state.XCredentialsStored {
		t.Fatal("the shell reports stored credentials in an empty keychain")
	}
	if _, err := store.XCredentials(ctx); err == nil {
		t.Fatal("XCredentials succeeded with nothing stored")
	}

	// A paste with one half missing is refused, and says which half.
	shell.state.XCredentialPaste = "auth_token=auth-token-fixture; guest_id=1"
	shell.saveXCookies()
	if shell.state.XCredentialsStored {
		t.Error("a paste without ct0 was accepted")
	}
	if want := shell.t().Tf(i18n.SettingsXCredentialMissing, "ct0"); shell.ToastText() != want {
		t.Errorf("error = %q, want %q", shell.ToastText(), want)
	}

	// The whole Cookie header works in one paste: this is the point of the single field.
	shell.state.XCredentialPaste = "Cookie: auth_token=auth-token-fixture; ct0=csrf-token-fixture; guest_id=1"
	shell.saveXCookies()
	if shell.state.ToastKind == ToastError {
		t.Fatalf("save failed: %s", shell.ToastText())
	}
	if !shell.state.XCredentialsStored {
		t.Error("the shell does not report the credentials as stored after saving them")
	}
	if shell.state.XCredentialPaste != "" {
		t.Error("the secret was left in the field")
	}

	// And the store the syncer reads now answers.
	got, err := store.XCredentials(ctx)
	if err != nil {
		t.Fatalf("XCredentials after saving: %v", err)
	}
	if got.AuthToken != "auth-token-fixture" || got.CSRFToken != "csrf-token-fixture" {
		t.Errorf("read back %+v, want the values that were typed", got)
	}
}

// TestAPasteThatIsNotACookieSaysSo keeps the third failure distinct.
//
// "You pasted the wrong thing" is not "your paste is missing ct0": the first
// means the clipboard never held the header, and it is fixed by copying again.
func TestAPasteThatIsNotACookieSaysSo(t *testing.T) {
	shell, _ := newShell(t)
	shell.UseCredentials(credentials.New(&memKeychain{}))

	shell.state.XCredentialPaste = "just some text"
	shell.saveXCookies()

	if want := shell.t().T(i18n.SettingsXCredentialNothingFound); shell.ToastText() != want {
		t.Errorf("error = %q, want %q", shell.ToastText(), want)
	}
}
