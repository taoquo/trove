//go:build darwin

package keychain

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
)

type recordedCall struct {
	name string
	args []string
}

type fakeRunner struct {
	calls    []recordedCall
	stdout   []byte
	err      error
	errByAcc map[string]error
}

func (f *fakeRunner) Run(_ context.Context, name string, args []string) ([]byte, error) {
	f.calls = append(f.calls, recordedCall{name: name, args: args})

	if f.errByAcc != nil {
		for _, arg := range args {
			if err, ok := f.errByAcc[arg]; ok {
				return f.stdout, err
			}
		}
	}
	return f.stdout, f.err
}

func (f *fakeRunner) lastArgs() []string {
	if len(f.calls) == 0 {
		return nil
	}
	return f.calls[len(f.calls)-1].args
}

func testStore(r Runner) *Store {
	return &Store{Service: "com.taoquo.trove.test", Runner: r, Binary: "/bin/sh"}
}

func TestSetBuildsTheExpectedInvocation(t *testing.T) {
	runner := &fakeRunner{}
	if err := testStore(runner).Set(context.Background(), AccountXAuthToken, "s3cret"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	want := []string{
		"add-generic-password",
		"-s", "com.taoquo.trove.test",
		"-a", "x.auth_token",
		"-w", "s3cret",
		"-U", // update in place, so saving twice does not fail
	}
	got := runner.lastArgs()
	if len(got) != len(want) {
		t.Fatalf("args = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("arg %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestGetReturnsTheSecret(t *testing.T) {
	// The tool prints a trailing newline that must not become part of the value.
	runner := &fakeRunner{stdout: []byte("s3cret\n")}

	got, err := testStore(runner).Get(context.Background(), AccountXAuthToken)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "s3cret" {
		t.Errorf("secret = %q, want %q without the newline", got, "s3cret")
	}

	args := runner.lastArgs()
	if args[len(args)-1] != "-w" {
		t.Errorf("args = %v, want -w last so only the secret is printed", args)
	}
}

func TestGetReportsNotFound(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"exit 44", &ExecError{Op: "exec", ExitCode: 44}},
		{"stderr wording", &ExecError{Op: "exec", ExitCode: 1,
			Stderr: "security: SecKeychainSearchCopyNext: The specified item could not be found in the keychain."}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := testStore(&fakeRunner{err: tc.err}).Get(context.Background(), "missing")
			if !errors.Is(err, ErrNotFound) {
				t.Fatalf("err = %v, want ErrNotFound", err)
			}
		})
	}
}

// TestGetReportsALockedKeychain pins the distinction the sync's sentence hangs on.
//
// The tool states a lock on stderr and does not give it a stable exit code, so the
// phrases are what is matched. A locked store is not an empty one: the cookies are
// in there, and only an unlock will do.
func TestGetReportsALockedKeychain(t *testing.T) {
	for _, stderr := range []string{
		"security: SecKeychainSearchCopyNext: The user interaction is not allowed.",
		"security: The keychain is locked.",
		"security: The keychain could not be unlocked.",
		"security: The passphrase you entered is not correct.",
	} {
		t.Run(stderr, func(t *testing.T) {
			runner := &fakeRunner{err: &ExecError{Op: "exec", ExitCode: 1, Stderr: stderr}}
			_, err := testStore(runner).Get(context.Background(), "acct")
			if !errors.Is(err, ErrLocked) {
				t.Fatalf("err = %v, want ErrLocked", err)
			}
		})
	}
}

func TestGetSurfacesOtherFailures(t *testing.T) {
	runner := &fakeRunner{err: &ExecError{Op: "exec", ExitCode: 1, Stderr: "security: invalid service name"}}

	_, err := testStore(runner).Get(context.Background(), "acct")
	if err == nil || errors.Is(err, ErrNotFound) || errors.Is(err, ErrLocked) {
		t.Fatalf("err = %v, want a real failure that is neither missing nor locked", err)
	}
}

// TestSetFailureNeverLeaksTheSecret is the important one: a write takes the
// secret as an argument, so an error that echoed the command line or stderr
// could put a credential into a log exactly when a save went wrong.
func TestSetFailureNeverLeaksTheSecret(t *testing.T) {
	const secret = "SUPER-SECRET-TOKEN"

	runner := &fakeRunner{err: &ExecError{
		Op:       "exec",
		ExitCode: 1,
		// A hostile or verbose implementation could echo the arguments here.
		Stderr: "add-generic-password -s svc -a acct -w " + secret + " -U failed",
	}}

	err := testStore(runner).Set(context.Background(), "acct", secret)
	if err == nil {
		t.Fatal("expected the write to fail")
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("error %q leaks the secret", err.Error())
	}
	if strings.Contains(err.Error(), "-w") {
		t.Errorf("error %q leaks the command line", err.Error())
	}
	// The exit code is still reported, so a failure is diagnosable.
	if !strings.Contains(err.Error(), "exit 1") {
		t.Errorf("error %q should still carry the exit code", err.Error())
	}
}

func TestDeleteReportsNotFound(t *testing.T) {
	runner := &fakeRunner{err: &ExecError{Op: "exec", ExitCode: 44}}

	err := testStore(runner).Delete(context.Background(), "acct")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestDeleteAllCountsAndToleratesMissing(t *testing.T) {
	runner := &fakeRunner{errByAcc: map[string]error{
		"b": &ExecError{Op: "exec", ExitCode: 44}, // already gone
	}}

	deleted, err := testStore(runner).DeleteAll(context.Background(), []string{"a", "b", "c"})
	if err != nil {
		t.Fatalf("DeleteAll: %v", err)
	}
	if deleted != 2 {
		t.Errorf("deleted = %d, want 2 (a and c existed)", deleted)
	}

	// Every account is attempted, so a reset does not stop at the first gap.
	if len(runner.calls) != 3 {
		t.Errorf("made %d calls, want 3", len(runner.calls))
	}
}

func TestDeleteAllReportsFailures(t *testing.T) {
	runner := &fakeRunner{errByAcc: map[string]error{
		"b": &ExecError{Op: "exec", ExitCode: 1, Stderr: "keychain locked"},
	}}

	_, err := testStore(runner).DeleteAll(context.Background(), []string{"a", "b"})
	if err == nil {
		t.Fatal("a real failure must be reported")
	}
}

func TestValidateRejectsEmptyService(t *testing.T) {
	store := &Store{Service: "  ", Runner: &fakeRunner{}}
	if err := store.Set(context.Background(), "a", "b"); err == nil {
		t.Error("an empty service name must be refused")
	}
}

func TestValidateReportsMissingSecurityTool(t *testing.T) {
	store := &Store{Service: "svc", Runner: &fakeRunner{}, Binary: "/nonexistent/security"}
	if _, err := store.Get(context.Background(), "a"); !errors.Is(err, ErrUnavailable) {
		t.Errorf("err = %v, want ErrUnavailable", err)
	}
}

func TestAccountNaming(t *testing.T) {
	if got := AccountLLM("openai"); got != "llm.openai" {
		t.Errorf("AccountLLM(openai) = %q", got)
	}
	if !IsLLMAccount(AccountLLM("anthropic")) {
		t.Error("an llm account must be recognised")
	}
	if IsLLMAccount(AccountXAuthToken) {
		t.Error("the X account is not an LLM account")
	}
	// Every X account must be distinguishable, so a credential sweep can find
	// them all.
	for _, account := range []string{AccountXAuthToken, AccountXCSRFToken, AccountXExtraCookies} {
		if account == "" {
			t.Error("account names must be non-empty constants")
		}
	}
}

// TestKeychainRoundTrip exercises the real tool.
//
// It is opt-in because it needs an unrestricted process: under the development
// sandbox the keychain rejects writes with "Operation not permitted". Run it
// outside the sandbox with:
//
//	XM_KEYCHAIN_INTEGRATION=1 go test ./internal/credentials/keychain/ -run Integration -v
//
// It uses a scratch service and removes everything it creates.
func TestKeychainRoundTrip(t *testing.T) {
	if os.Getenv("XM_KEYCHAIN_INTEGRATION") != "1" {
		t.Skip("set XM_KEYCHAIN_INTEGRATION=1 outside the sandbox to exercise the real keychain")
	}

	store := &Store{Service: "com.taoquo.trove.integrationtest"}
	ctx := context.Background()
	const account = "round-trip"

	// Start from a clean slate.
	_ = store.Delete(ctx, account)
	t.Cleanup(func() { _ = store.Delete(ctx, account) })

	if err := store.Set(ctx, account, "first-value"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, err := store.Get(ctx, account)
	if err != nil {
		t.Fatalf("Get after Set: %v", err)
	}
	if got != "first-value" {
		t.Errorf("secret = %q, want first-value", got)
	}

	// Saving again must overwrite rather than fail.
	if err := store.Set(ctx, account, "rotated-value"); err != nil {
		t.Fatalf("second Set: %v", err)
	}
	got, err = store.Get(ctx, account)
	if err != nil {
		t.Fatalf("Get after rotate: %v", err)
	}
	if got != "rotated-value" {
		t.Errorf("secret = %q, want the rotated value", got)
	}

	if err := store.Delete(ctx, account); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := store.Get(ctx, account); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get after Delete: err = %v, want ErrNotFound", err)
	}
}

// TestSetRefusesSecretsItCannotReadBack pins the guard that turns a silent
// credential corruption into a loud save-time error.
//
// Verified against the real tool: `security ... -w` prints a printable-ASCII
// secret verbatim but prints any other secret as lowercase hex, so reading a
// non-ASCII value back would hand the caller a hex string.
func TestSetRefusesSecretsItCannotReadBack(t *testing.T) {
	runner := &fakeRunner{}
	store := testStore(runner)

	rejected := []struct {
		name   string
		secret string
	}{
		{"non-ASCII", "üñíçødé-token"},
		{"emoji", "token-🔑"},
		{"high byte", "tok\u00e9n"},
		{"tab", "tok\ten"},
		{"newline", "tok\nen"},
		{"NUL", "tok\x00en"},
		{"DEL", "tok\x7fen"},
	}
	for _, tc := range rejected {
		t.Run(tc.name, func(t *testing.T) {
			err := store.Set(context.Background(), "acct", tc.secret)
			if !errors.Is(err, ErrNonASCII) {
				t.Fatalf("err = %v, want ErrNonASCII", err)
			}
		})
	}
	if len(runner.calls) != 0 {
		t.Error("a rejected secret must not reach the keychain at all")
	}

	// The printable-ASCII range, including the boundaries, is accepted.
	for _, secret := range []string{"!\"#$%&'()*+,-./", "sk-plain", "AZaz09", "a b", "back\\slash"} {
		if err := store.Set(context.Background(), "acct", secret); err != nil {
			t.Errorf("Set(%q) = %v, want it accepted", secret, err)
		}
	}
	if len(runner.calls) != 5 {
		t.Errorf("made %d calls, want 5 accepted writes", len(runner.calls))
	}
}

func TestValidateSecretBoundaries(t *testing.T) {
	if err := validateSecret(""); err != nil {
		t.Errorf("an empty secret is the caller's problem, not a byte-range one: %v", err)
	}
	if err := validateSecret(" "); err != nil {
		t.Errorf("0x20 must be accepted: %v", err)
	}
	if err := validateSecret("~"); err != nil {
		t.Errorf("0x7e must be accepted: %v", err)
	}
	if err := validateSecret("\x1f"); err == nil {
		t.Error("0x1f must be rejected")
	}
	if err := validateSecret("\x7f"); err == nil {
		t.Error("0x7f must be rejected")
	}
}

// TestKeychainPrintableASCIIRoundTrip exercises the real tool with the awkward
// printable characters, so the guard's premise is checked rather than assumed.
func TestKeychainPrintableASCIIRoundTrip(t *testing.T) {
	if os.Getenv("XM_KEYCHAIN_INTEGRATION") != "1" {
		t.Skip("set XM_KEYCHAIN_INTEGRATION=1 to exercise the real keychain")
	}

	secrets := []string{
		"sk-plain-ascii-123",
		"with space",
		`with"quote`,
		`back\slash`,
		"tilde~and_caret^",
		"a!@#$%^&*()b",
		"0123456789abcdef", // a hex-looking token must not be reinterpreted
	}

	store := &Store{Service: "com.taoquo.trove.integrationtest"}
	ctx := context.Background()

	for i, secret := range secrets {
		account := fmt.Sprintf("ascii-%d", i)
		_ = store.Delete(ctx, account)
		t.Cleanup(func() { _ = store.Delete(ctx, account) })

		if err := store.Set(ctx, account, secret); err != nil {
			t.Fatalf("Set(%q): %v", secret, err)
		}
		got, err := store.Get(ctx, account)
		if err != nil {
			t.Fatalf("Get(%q): %v", secret, err)
		}
		if got != secret {
			t.Errorf("round trip of %q gave %q", secret, got)
		}
	}
}
