// Package keychain stores credentials in the platform's secure store: the macOS
// login keychain, or Windows Credential Manager. Other platforms report that no
// secure store is available.
//
// The store is deliberately small and synchronous. Every credential this app
// handles is printable ASCII; the validation below keeps a value from being
// written where it could not be read back verbatim.
package keychain

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Account names. They are fixed and traceable: forgetting credentials has to be
// able to find every entry this app created.
const (
	// AccountXAuthToken holds the X session cookie value.
	AccountXAuthToken = "x.auth_token"
	// AccountXCSRFToken holds the X ct0 cookie value.
	AccountXCSRFToken = "x.ct0"
	// AccountXExtraCookies holds any additional cookies X requires.
	AccountXExtraCookies = "x.cookie_extra"
	// llmAccountPrefix namespaces one entry per LLM provider.
	llmAccountPrefix = "llm."
)

// AccountLLM returns the account name holding a provider's credential.
func AccountLLM(providerID string) string { return llmAccountPrefix + providerID }

// IsLLMAccount reports whether an account name holds an LLM credential.
func IsLLMAccount(account string) bool { return strings.HasPrefix(account, llmAccountPrefix) }

// ErrNotFound reports that no item is stored for the account.
var ErrNotFound = errors.New("keychain: no stored item")

// ErrUnavailable reports that the secure store cannot be used at all, for example
// because the platform has none or the tool is missing.
var ErrUnavailable = errors.New("keychain: unavailable")

// ErrNonASCII reports a secret this backend cannot read back reliably.
var ErrNonASCII = errors.New("keychain: this backend only round-trips printable ASCII secrets")

// ExecError is a failed security invocation on macOS.
//
// It deliberately does not carry the command line: a write passes the secret as
// an argument, so an error that quoted the command would put the credential into
// a log the moment a save failed.
type ExecError struct {
	// Op names the operation, e.g. "set".
	Op string
	// ExitCode is the process exit status, or -1 when it never started.
	ExitCode int
	// Stderr is captured output, for read and delete only. It is never
	// populated for a write.
	Stderr string
}

func (e *ExecError) Error() string {
	message := fmt.Sprintf("keychain: %s failed (exit %d)", e.Op, e.ExitCode)
	if e.Stderr != "" {
		message += ": " + e.Stderr
	}
	return message
}

// validateSecret rejects a secret the backends would not return verbatim.
//
// macOS `security find-generic-password -w` prints a printable-ASCII secret as-is,
// but prints a secret containing any other byte as lowercase hex. Windows
// Credential Manager stores bytes, but keeping one rule across platforms is what
// makes a credential saved on one machine readable on the other.
func validateSecret(secret string) error {
	for i := 0; i < len(secret); i++ {
		if secret[i] < 0x20 || secret[i] > 0x7E {
			return fmt.Errorf("%w (byte %d is 0x%02x)", ErrNonASCII, i, secret[i])
		}
	}
	return nil
}

// deleter is the part of a platform store DeleteAll needs.
type deleter interface {
	Delete(ctx context.Context, account string) error
}

// deleteAll removes every named account, reporting how many existed. It is used
// when the user resets local data, which must not leave credentials behind.
func deleteAll(ctx context.Context, store deleter, accounts []string) (int, error) {
	deleted := 0
	var firstErr error

	for _, account := range accounts {
		err := store.Delete(ctx, account)
		switch {
		case err == nil:
			deleted++
		case errors.Is(err, ErrNotFound):
			// Nothing stored: not a failure.
		default:
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	return deleted, firstErr
}
