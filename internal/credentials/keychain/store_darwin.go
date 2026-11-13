//go:build darwin

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

// SecurityBinary is the tool that talks to the keychain.
const SecurityBinary = "/usr/bin/security"

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
func (s *Store) DeleteAll(ctx context.Context, accounts []string) (int, error) {
	return deleteAll(ctx, s, accounts)
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
