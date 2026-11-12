package ui

import (
	"context"
	"errors"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/x-master/apps/macos/internal/credentials"
	"github.com/taoquo/x-master/apps/macos/internal/domain"
	"github.com/taoquo/x-master/apps/macos/internal/i18n"
	"github.com/taoquo/x-master/apps/macos/internal/xsource"
)

// awaitProbe waits for the probe goroutine, which a shell without a window does not
// serialise onto a main thread.
func (s *Shell) awaitProbe(t *testing.T) {
	t.Helper()
	if s.probeDone != nil {
		<-s.probeDone
	}
}

// storedCookies is a store whose Keychain really holds both cookies.
//
// The probe re-reads the Keychain before it runs, so a test that only set
// XCredentialsStored would be testing the flag rather than the path the panel takes.
func storedCookies(t *testing.T) *credentials.Store {
	t.Helper()
	store := credentials.New(&memKeychain{})
	if err := store.SaveXCredentials(context.Background(), xsource.Credentials{
		AuthToken: "auth-token",
		CSRFToken: "csrf-token",
	}); err != nil {
		t.Fatalf("SaveXCredentials: %v", err)
	}
	return store
}

// stubProbe answers the connection test without a network.
type stubProbe struct {
	err   error
	calls int
}

func (p *stubProbe) TestConnection(context.Context) error {
	p.calls++
	return p.err
}

// TestTheConnectionTestSaysTheCookiesWork is the successful half: one real request, one
// sentence, and the route it took.
func TestTheConnectionTestSaysTheCookiesWork(t *testing.T) {
	shell, _ := newShell(t)
	shell.UseCredentials(storedCookies(t))

	probe := &stubProbe{}
	shell.UseXProbe(probe)

	shell.TestXConnection(context.Background())
	shell.awaitProbe(t)

	if probe.calls != 1 {
		t.Errorf("the probe ran %d times, want once", probe.calls)
	}
	if !shell.state.XTestTested || !shell.state.XTestOK {
		t.Errorf("outcome tested=%v ok=%v, want a recorded success",
			shell.state.XTestTested, shell.state.XTestOK)
	}
	if got, want := shell.state.XTestResult, shell.t().T(i18n.SettingsXTestOK); got != want {
		t.Errorf("result = %q, want %q", got, want)
	}
	if shell.state.XTestRoute == "" {
		t.Error("the route is not reported; a proxy problem would be invisible")
	}
}

// TestTheConnectionTestNamesANetworkFailure is the bug report this was written for: a sync
// failed and said "X login expired, sign in again" while the cookies were fine, because
// the request had never reached X.
//
// The probe's job is to make that distinction legible: a transport failure is reported as
// a network failure, and the classified sentence is the network one rather than the login.
func TestTheConnectionTestNamesANetworkFailure(t *testing.T) {
	shell, _ := newShell(t)
	shell.UseCredentials(storedCookies(t))

	shell.UseXProbe(&stubProbe{err: &xsource.Error{
		Kind: domain.ErrNetwork,
		Op:   "test-connection",
		Err:  errors.New("net/http: TLS handshake timeout"),
	}})
	shell.TestXConnection(context.Background())
	shell.awaitProbe(t)

	if shell.state.XTestOK {
		t.Fatal("a failed probe reported success")
	}
	if got, want := shell.state.XTestResult, shell.t().T(i18n.SyncErrorNetworkError); got != want {
		t.Errorf("result = %q, want the network sentence %q", got, want)
	}
	if got := shell.state.XTestResult; got == shell.t().T(i18n.SyncErrorAuthExpired) {
		t.Error("a network failure was reported as an expired login")
	}
}

// TestTheConnectionTestKeepsAnUnclassifiedMessage is the source's `?? summary` rule applied
// here: a failure this build does not know keeps the transport's own words rather than a
// guess about what to do.
func TestTheConnectionTestKeepsAnUnclassifiedMessage(t *testing.T) {
	shell, _ := newShell(t)
	shell.UseCredentials(storedCookies(t))

	shell.UseXProbe(&stubProbe{err: errors.New("dial tcp: lookup x.com: no such host")})
	shell.TestXConnection(context.Background())
	shell.awaitProbe(t)

	if got, want := shell.state.XTestResult, "dial tcp: lookup x.com: no such host"; got != want {
		t.Errorf("result = %q, want %q", got, want)
	}
}

// TestTheConnectionTestRefusesAnEmptyKeychain pins the ordering: with no cookies there is
// nothing to test, and saying "the cookies work" would be a lie about a request that was
// never made.
func TestTheConnectionTestRefusesAnEmptyKeychain(t *testing.T) {
	shell, _ := newShell(t)
	shell.UseCredentials(credentials.New(&memKeychain{}))

	probe := &stubProbe{}
	shell.UseXProbe(probe)
	shell.TestXConnection(context.Background())
	shell.awaitProbe(t)

	if probe.calls != 0 {
		t.Errorf("the probe ran %d times with an empty Keychain", probe.calls)
	}
	if !shell.state.XTestTested || shell.state.XTestOK {
		t.Errorf("outcome tested=%v ok=%v, want a recorded non-success",
			shell.state.XTestTested, shell.state.XTestOK)
	}
	if got, want := shell.state.XTestResult, shell.t().T(i18n.SettingsXTestNoCredentials); got != want {
		t.Errorf("result = %q, want %q", got, want)
	}
}

// TestTheSyncPanelDrawsTheConnectionTest is the panel itself: the button exists, and
// pressing it goes through the same seam.
func TestTheSyncPanelDrawsTheConnectionTest(t *testing.T) {
	shell, _ := newShell(t)
	shell.UseCredentials(storedCookies(t))
	shell.state.LeftSidebarWidth = WidthSidebarDef
	shell.state.RightSidebarWidth = WidthInspectorDef

	probe := &stubProbe{}
	shell.UseXProbe(probe)

	shell.OpenSettings()
	shell.state.SettingsSection = SettingsSectionSync
	shell.state.XTestTested = true
	shell.state.XTestOK = true
	shell.state.XTestResult = shell.t().T(i18n.SettingsXTestOK)
	shell.state.XTestRoute = "Network route: http://127.0.0.1:7890"

	tester := ui.NewTester(shell.View, 1180, 900)
	tester.Frame()
	if !tester.HasText(shell.t().T(i18n.SettingsXTestConnection)) {
		t.Fatalf("the panel has no connection test: %q", tester.Texts())
	}
	if !tester.HasText(shell.t().T(i18n.SettingsXTestOK)) {
		t.Errorf("the recorded result is not drawn: %q", tester.Texts())
	}
	if !tester.HasText("Network route: http://127.0.0.1:7890") {
		t.Errorf("the route is not drawn: %q", tester.Texts())
	}

	if err := tester.Click(shell.t().T(i18n.SettingsXTestConnection)); err != nil {
		t.Fatalf("Click: %v", err)
	}
	// The click starts the probe; a shell without a window runs its publish on the
	// goroutine rather than on the window's thread, so the test has to wait for it
	// before it reads state or draws another frame.
	shell.awaitProbe(t)
	if probe.calls != 1 {
		t.Errorf("the button did not run the probe: %d calls", probe.calls)
	}
}
