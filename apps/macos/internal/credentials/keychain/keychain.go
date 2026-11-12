// Package keychain stores credentials in the macOS login keychain.
//
// MyGo has no secure-storage API, so this shells out to /usr/bin/security. That
// choice keeps CGO_ENABLED=0 (decision D7) at a documented cost; see
// docs/security-and-privacy.md §2.2.1 and the note on argv below.
package keychain

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
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

// SecurityBinary is the tool that talks to the keychain.
const SecurityBinary = "/usr/bin/security"

// ErrNotFound reports that no item is stored for the account.
var ErrNotFound = errors.New("keychain: no stored item")

// ErrUnavailable reports that the keychain cannot be used at all, for example
// because the security tool is missing.
var ErrUnavailable = errors.New("keychain: unavailable")

// ExecError is a failed security invocation.
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

// Runner runs an external command. It is an interface so the store can be
// tested without touching a real keychain.
type Runner interface {
	// Run executes name with args and returns stdout. A non-zero exit must be
	// reported as an *ExecError.
	Run(ctx context.Context, name string, args []string) ([]byte, error)
}

// ExecRunner runs commands as real subprocesses.
type ExecRunner struct{}

// Run implements Runner.
//
// exec.Command is used rather than a shell, so the secret appears in exactly one
// process's argument vector and is never subject to shell parsing.
func (ExecRunner) Run(ctx context.Context, name string, args []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		return stdout.Bytes(), nil
	}

	exitCode := -1
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		exitCode = exitErr.ExitCode()
	}

	return stdout.Bytes(), &ExecError{
		Op:       "exec",
		ExitCode: exitCode,
		Stderr:   strings.TrimSpace(stderr.String()),
	}
}

// ErrNonASCII reports a secret this backend cannot read back reliably.
var ErrNonASCII = errors.New("keychain: this backend only round-trips printable ASCII secrets")

// validateSecret rejects a secret the security tool would not return verbatim.
//
// Verified against the real tool: `security find-generic-password -w` prints a
// printable-ASCII secret as-is, but prints a secret containing any other byte as
// lowercase hex. Reading such a value back would hand a caller a hex string
// instead of the credential — a silent authentication failure with no hint of
// the cause.
//
// `-g` does report a `0x…` form unambiguously, but its alternative representation
// is a quoted string with octal escapes, so parsing it means trusting an
// undocumented format. Refusing the write is the smaller risk: every credential
// this app handles (X's hex cookies, LLM API keys and tokens) is printable ASCII,
// so nothing real is lost, and an unexpected value fails loudly at save time
// instead of silently at use time.
func validateSecret(secret string) error {
	for i := 0; i < len(secret); i++ {
		if secret[i] < 0x20 || secret[i] > 0x7E {
			return fmt.Errorf("%w (byte %d is 0x%02x)", ErrNonASCII, i, secret[i])
		}
	}
	return nil
}

// Store reads and writes credentials in one keychain service.
type Store struct {
	// Service is the keychain service name, fixed for the app.
	Service string
	// Runner performs the calls; nil uses ExecRunner.
	Runner Runner
	// Binary overrides the security tool path, for tests.
	Binary string
}

// New builds a store for a service name.
func New(service string) *Store {
	return &Store{Service: service}
}

func (s *Store) runner() Runner {
	if s.Runner != nil {
		return s.Runner
	}
	return ExecRunner{}
}

func (s *Store) binary() string {
	if s.Binary != "" {
		return s.Binary
	}
	return SecurityBinary
}

// Set stores a secret, replacing any existing value.
//
// Known limitation: the value is passed as an argument, so it is briefly visible
// in the process table. `security` offers no non-interactive alternative —
// omitting the value makes it prompt on a TTY, and piping stdin does not work
// (both verified against the real tool). Writes happen only when the user saves
// credentials, and reads never pass a secret as an argument.
//
// A failed write reports the exit code and nothing else, so the secret cannot
// reach a log through the error.
func (s *Store) Set(ctx context.Context, account, secret string) error {
	if err := s.validate(); err != nil {
		return err
	}
	if strings.TrimSpace(account) == "" {
		return errors.New("keychain: account name is required")
	}
	// Refuse a secret the read path could not return verbatim, rather than
	// storing something that would be read back wrong.
	if err := validateSecret(secret); err != nil {
		return err
	}

	// -U updates in place, so saving twice does not fail with a duplicate error.
	args := []string{
		"add-generic-password",
		"-s", s.Service,
		"-a", account,
		"-w", secret,
		"-U",
	}

	if _, err := s.runner().Run(ctx, s.binary(), args); err != nil {
		var execErr *ExecError
		if errors.As(err, &execErr) {
			// Rebuild without stderr: it is not needed here and the write path is
			// the one place a secret could leak into a message.
			return &ExecError{Op: "set", ExitCode: execErr.ExitCode}
		}
		return fmt.Errorf("keychain: set: %w", err)
	}
	return nil
}

// Get returns the stored secret, or ErrNotFound.
func (s *Store) Get(ctx context.Context, account string) (string, error) {
	if err := s.validate(); err != nil {
		return "", err
	}

	// -w prints only the secret on stdout, which is captured through a pipe and
	// so is not visible to other processes.
	args := []string{"find-generic-password", "-s", s.Service, "-a", account, "-w"}

	stdout, err := s.runner().Run(ctx, s.binary(), args)
	if err != nil {
		if isNotFound(err) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("keychain: get %s: %w", account, err)
	}

	// Command substitution trims the trailing newline the tool prints.
	return strings.TrimRight(string(stdout), "\r\n"), nil
}

// Delete removes an account, reporting ErrNotFound when there was nothing stored.
func (s *Store) Delete(ctx context.Context, account string) error {
	if err := s.validate(); err != nil {
		return err
	}

	args := []string{"delete-generic-password", "-s", s.Service, "-a", account}

	if _, err := s.runner().Run(ctx, s.binary(), args); err != nil {
		if isNotFound(err) {
			return ErrNotFound
		}
		return fmt.Errorf("keychain: delete %s: %w", account, err)
	}
	return nil
}

// DeleteAll removes every account this app owns, reporting how many existed.
//
// It is used when the user resets local data, which must not leave credentials
// behind.
func (s *Store) DeleteAll(ctx context.Context, accounts []string) (int, error) {
	deleted := 0
	var firstErr error

	for _, account := range accounts {
		err := s.Delete(ctx, account)
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

func (s *Store) validate() error {
	if strings.TrimSpace(s.Service) == "" {
		return errors.New("keychain: service name is required")
	}
	if _, err := os.Stat(s.binary()); err != nil {
		return fmt.Errorf("%w: %s", ErrUnavailable, s.binary())
	}
	return nil
}

// isNotFound recognises the tool's "no such item" outcome.
//
// `security` has used exit code 44 for this, and older or future versions may
// only say so on stderr, so both are checked.
func isNotFound(err error) bool {
	var execErr *ExecError
	if !errors.As(err, &execErr) {
		return false
	}
	if execErr.ExitCode == 44 {
		return true
	}
	lower := strings.ToLower(execErr.Stderr)
	return strings.Contains(lower, "could not be found") ||
		strings.Contains(lower, "item could not be found") ||
		strings.Contains(lower, "the specified item could not be found")
}
