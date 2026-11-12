// Package credentials resolves the stored secrets the app needs.
//
// It is the one place that knows which keychain accounts exist, so "forget
// credentials" and "reset local data" can find every secret the app created and
// nothing else.
//
// Secrets live only in the macOS Keychain (decision D7). They never reach the
// database, a log, an export or a diagnostic bundle.
//
// See docs/security-and-privacy.md §2.
package credentials

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/taoquo/trove/apps/macos/internal/credentials/keychain"
	"github.com/taoquo/trove/apps/macos/internal/xsource"
)

// Keychain is the subset of the keychain store this package uses. It is an
// interface so the resolution rules can be tested without a real keychain.
type Keychain interface {
	Set(ctx context.Context, account, secret string) error
	Get(ctx context.Context, account string) (string, error)
	Delete(ctx context.Context, account string) error
}

// legacyLLMKeyAccount holds the old single-provider API key.
//
// The extension stored one key plus a per-provider map, and read the single key
// as a fallback for the *active* provider only. Keeping it as its own account is
// what lets that asymmetry survive migration instead of silently becoming a
// fallback for every provider.
const legacyLLMKeyAccount = "llm.legacy"

// Service is the keychain service name for this app.
//
// It keeps the pre-rename identifier on purpose: the service name is the key the user's
// stored X cookies and model key are filed under, and changing it would make them
// invisible to the app with no way to find them again. The bundle identifier and the
// visible name have moved to trove; this one is data, not branding.
const Service = "com.taoquo.xmaster"

// ErrNoCredential reports that nothing is stored for an account.
var ErrNoCredential = errors.New("credentials: nothing stored")

// ErrIncompleteCredentials reports a partially configured credential set.
var ErrIncompleteCredentials = errors.New("credentials: incomplete")

// BrowserReader reads the two X cookies out of a signed-in browser.
//
// It is a field so the refresh rules can be tested without touching a real
// browser profile, the same way Keychain is.
type BrowserReader func(ctx context.Context) (xsource.Credentials, error)

// Store resolves credentials.
type Store struct {
	Keychain Keychain
	// Browser is where a refresh looks for a live session. Nil uses
	// BrowserCookies, the real reader.
	Browser BrowserReader
}

// New builds a store backed by the app's keychain service.
func New(kc Keychain) *Store { return &Store{Keychain: kc} }

// NewWithKeychain builds a store for the default keychain service.
func NewWithKeychain() *Store {
	return &Store{Keychain: &keychain.Store{Service: Service}}
}

// XCredentials returns the stored X cookies.
func (s *Store) XCredentials(ctx context.Context) (xsource.Credentials, error) {
	authToken, err := s.get(ctx, keychain.AccountXAuthToken)
	if err != nil {
		return xsource.Credentials{}, err
	}
	csrfToken, err := s.get(ctx, keychain.AccountXCSRFToken)
	if err != nil {
		return xsource.Credentials{}, err
	}

	// Extra cookies are optional: the minimal pair is the documented default and
	// only grows if a real account turns out to need more.
	extra, err := s.get(ctx, keychain.AccountXExtraCookies)
	if err != nil && !errors.Is(err, ErrNoCredential) {
		return xsource.Credentials{}, err
	}

	credentials := xsource.Credentials{
		AuthToken:    authToken,
		CSRFToken:    csrfToken,
		ExtraCookies: extra,
	}
	if err := credentials.Validate(); err != nil {
		return xsource.Credentials{}, fmt.Errorf("%w: %v", ErrIncompleteCredentials, err)
	}
	return credentials, nil
}

// SaveXCredentials stores the X cookies.
//
// The caller is expected to have proved them with a real request first, so a bad
// pair is not persisted to be discovered at the next sync.
func (s *Store) SaveXCredentials(ctx context.Context, credentials xsource.Credentials) error {
	if err := credentials.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrIncompleteCredentials, err)
	}

	// Read the pair first so a failure part way through can put back what was there. The
	// two cookies authenticate as a pair: writing one and failing on the other left the
	// keychain holding a new auth_token with the previous ct0, and every sync then failed
	// with a mismatch the user could not see or explain.
	previousAuth, authErr := s.Keychain.Get(ctx, keychain.AccountXAuthToken)
	previousCSRF, csrfErr := s.Keychain.Get(ctx, keychain.AccountXCSRFToken)

	rollback := func() {
		if authErr == nil {
			_ = s.Keychain.Set(ctx, keychain.AccountXAuthToken, previousAuth)
		} else {
			_ = s.Keychain.Delete(ctx, keychain.AccountXAuthToken)
		}
		if csrfErr == nil {
			_ = s.Keychain.Set(ctx, keychain.AccountXCSRFToken, previousCSRF)
		} else {
			_ = s.Keychain.Delete(ctx, keychain.AccountXCSRFToken)
		}
	}

	if err := s.Keychain.Set(ctx, keychain.AccountXAuthToken, credentials.AuthToken); err != nil {
		return err
	}
	if err := s.Keychain.Set(ctx, keychain.AccountXCSRFToken, credentials.CSRFToken); err != nil {
		rollback()
		return err
	}

	trimmed := strings.TrimSpace(credentials.ExtraCookies)
	if trimmed == "" {
		// Clearing an optional value must not leave the previous one behind.
		_ = s.Keychain.Delete(ctx, keychain.AccountXExtraCookies)
		return nil
	}
	if err := s.Keychain.Set(ctx, keychain.AccountXExtraCookies, trimmed); err != nil {
		rollback()
		return err
	}
	return nil
}

// RefreshXCredentialsFromBrowser replaces the stored pair with the browser's.
//
// It reports whether the value actually changed, which is what lets a caller
// decide whether retrying a failed request is worth the request: when the
// browser holds the same dead session, a retry would fail identically.
//
// This is the answer to the one thing the stored pair cannot do by itself:
// X rotates ct0, and an app that only ever reads its own copy goes stale. The
// extension never has the problem because it asks the browser on every request.
func (s *Store) RefreshXCredentialsFromBrowser(ctx context.Context) (bool, error) {
	found, err := s.browserCookies(ctx)
	if err != nil {
		return false, err
	}

	stored, storedErr := s.XCredentials(ctx)
	if storedErr == nil && stored.AuthToken == found.AuthToken && stored.CSRFToken == found.CSRFToken {
		return false, nil
	}
	if err := s.SaveXCredentials(ctx, found); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) browserCookies(ctx context.Context) (xsource.Credentials, error) {
	if s.Browser != nil {
		return s.Browser(ctx)
	}
	return BrowserCookies(ctx)
}

// DeleteXCredentials forgets the X session.
func (s *Store) DeleteXCredentials(ctx context.Context) error {
	var firstErr error
	for _, account := range []string{
		keychain.AccountXAuthToken,
		keychain.AccountXCSRFToken,
		keychain.AccountXExtraCookies,
	} {
		if err := s.Keychain.Delete(ctx, account); err != nil && !errors.Is(err, keychain.ErrNotFound) {
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// LLMCredential is one provider's stored credential.
//
// Only the api-key kind is implemented; the extension reserved OAuth without
// implementing it, and nothing here invents a flow that has never been tested.
type LLMCredential struct {
	APIKey string
	// Legacy records that the value came from the old single-provider field.
	Legacy bool
}

// IsEmpty reports whether no key is stored.
func (c LLMCredential) IsEmpty() bool { return strings.TrimSpace(c.APIKey) == "" }

// LLMCredential returns a provider's key.
//
// allowLegacy must be true only for the *active* provider, matching the
// extension: the old single key belonged to whichever provider the user had
// configured, so offering it to every provider would silently send one vendor's
// key to another.
func (s *Store) LLMCredential(ctx context.Context, providerID string, allowLegacy bool) (LLMCredential, error) {
	trimmed := strings.TrimSpace(providerID)
	if trimmed == "" {
		return LLMCredential{}, errors.New("credentials: a provider id is required")
	}

	value, err := s.get(ctx, keychain.AccountLLM(trimmed))
	switch {
	case err == nil && strings.TrimSpace(value) != "":
		return LLMCredential{APIKey: strings.TrimSpace(value)}, nil
	case err != nil && !errors.Is(err, ErrNoCredential):
		return LLMCredential{}, err
	}

	if !allowLegacy {
		return LLMCredential{}, nil
	}

	legacy, legacyErr := s.get(ctx, legacyLLMKeyAccount)
	if legacyErr != nil {
		if errors.Is(legacyErr, ErrNoCredential) {
			return LLMCredential{}, nil
		}
		return LLMCredential{}, legacyErr
	}
	return LLMCredential{APIKey: strings.TrimSpace(legacy), Legacy: true}, nil
}

// SaveLLMKey stores a provider's key.
//
// An unknown provider id is accepted on purpose: a settings file from a newer
// build may name one this build does not know, and refusing would lose the key.
func (s *Store) SaveLLMKey(ctx context.Context, providerID, apiKey string) error {
	trimmedProvider := strings.TrimSpace(providerID)
	if trimmedProvider == "" {
		return errors.New("credentials: a provider id is required")
	}
	if strings.TrimSpace(apiKey) == "" {
		return errors.New("credentials: an API key is required")
	}
	return s.Keychain.Set(ctx, keychain.AccountLLM(trimmedProvider), strings.TrimSpace(apiKey))
}

// DeleteLLMKey forgets a provider's key.
func (s *Store) DeleteLLMKey(ctx context.Context, providerID string) error {
	err := s.Keychain.Delete(ctx, keychain.AccountLLM(providerID))
	if errors.Is(err, keychain.ErrNotFound) {
		return nil
	}
	return err
}

// MigrateLegacyKey moves the old single-provider key to the active provider.
//
// It exists so a migrated install keeps working: the old field is read once, the
// value is stored under the provider it belonged to, and the legacy account is
// removed so the fallback stops applying.
func (s *Store) MigrateLegacyKey(ctx context.Context, activeProviderID string) (bool, error) {
	legacy, err := s.get(ctx, legacyLLMKeyAccount)
	if err != nil {
		if errors.Is(err, ErrNoCredential) {
			return false, nil
		}
		return false, err
	}
	if strings.TrimSpace(legacy) == "" {
		return false, nil
	}

	// Only fill the active provider, and only when it has no key of its own.
	existing, err := s.LLMCredential(ctx, activeProviderID, false)
	if err != nil {
		return false, err
	}
	if !existing.IsEmpty() {
		// Something already owns the slot; the legacy value is redundant.
		return false, s.Keychain.Delete(ctx, legacyLLMKeyAccount)
	}

	if err := s.SaveLLMKey(ctx, activeProviderID, legacy); err != nil {
		return false, err
	}
	if err := s.Keychain.Delete(ctx, legacyLLMKeyAccount); err != nil {
		return false, err
	}
	return true, nil
}

// Accounts lists every keychain account this app owns, so a reset can remove
// them all without guessing.
func Accounts(providerIDs []string) []string {
	accounts := []string{
		keychain.AccountXAuthToken,
		keychain.AccountXCSRFToken,
		keychain.AccountXExtraCookies,
		legacyLLMKeyAccount,
	}
	for _, providerID := range providerIDs {
		if trimmed := strings.TrimSpace(providerID); trimmed != "" {
			accounts = append(accounts, keychain.AccountLLM(trimmed))
		}
	}
	return accounts
}

// get reads an account, mapping "not found" onto ErrNoCredential.
func (s *Store) get(ctx context.Context, account string) (string, error) {
	if s.Keychain == nil {
		return "", errors.New("credentials: no keychain configured")
	}
	value, err := s.Keychain.Get(ctx, account)
	if err != nil {
		if errors.Is(err, keychain.ErrNotFound) {
			return "", fmt.Errorf("%w (%s)", ErrNoCredential, account)
		}
		return "", err
	}
	return value, nil
}

// DeleteAll forgets every secret this app stores.
//
// The provider ids are passed in rather than imported: this package knows about
// accounts, not about which providers exist, and reaching for the llm package here would
// tie the two together for one call.
//
// It exists because the reset action's confirmation promises that local settings are
// cleared, and the source says why that has to include the key: the API key lives under
// its own storage key, so it used to survive a reset the dialog described as total.
func (s *Store) DeleteAll(ctx context.Context, providerIDs []string) error {
	if err := s.DeleteXCredentials(ctx); err != nil {
		return err
	}

	accounts := []string{legacyLLMKeyAccount}
	for _, providerID := range providerIDs {
		if trimmed := strings.TrimSpace(providerID); trimmed != "" {
			accounts = append(accounts, keychain.AccountLLM(trimmed))
		}
	}

	for _, account := range accounts {
		err := s.Keychain.Delete(ctx, account)
		// Nothing stored for an account is the state the caller wanted.
		if err != nil && !errors.Is(err, keychain.ErrNotFound) {
			return fmt.Errorf("credentials: clear %s: %w", account, err)
		}
	}
	return nil
}
