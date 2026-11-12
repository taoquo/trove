package ui

import (
	"context"

	"github.com/taoquo/x-master/apps/macos/internal/domain"
	"github.com/taoquo/x-master/apps/macos/internal/i18n"
	"github.com/taoquo/x-master/apps/macos/internal/netproxy"
	"github.com/taoquo/x-master/apps/macos/internal/xsource"
)

// XProbe is one real bookmarks request, made to answer "do these cookies work
// from this network".
//
// It is an interface rather than the concrete client because the shell must not
// depend on the wire protocol, and because a test needs to answer without a
// network. The X client's TestConnection is exactly this: a one-item bookmarks
// request, sent and parsed.
type XProbe interface {
	TestConnection(ctx context.Context) error
}

// UseXProbe attaches the connection probe the sync settings panel runs.
//
// Without one the button says the cookies have to be saved first rather than
// silently doing nothing: a control that appears to work and never reports is
// the defect this panel exists to prevent.
func (s *Shell) UseXProbe(probe XProbe) { s.xProbe = probe }

// TestXConnection probes X and records the outcome for the settings panel.
//
// Why this exists: a failed sync could not tell "these cookies are wrong" from
// "this machine cannot reach X". Both ended in the sentence about the login,
// because a request that never got a response has no HTTP status to classify.
// Measured on the machine this was written on: a direct request failed with
// "TLS handshake timeout" after 10.26s and was recorded as "network", while the
// same request through the system's proxy answered in 722ms -- and the user was
// being told to sign in again.
//
// The probe makes one real request and reports which of the two it was, plus
// the route it took, so the next step is not a guess.
func (s *Shell) TestXConnection(ctx context.Context) {
	s.RefreshXCookieStatus(ctx)
	if !s.state.XCredentialsStored {
		s.state.XTestTested = true
		s.state.XTestRunning = false
		s.state.XTestOK = false
		s.state.XTestResult = s.t().T(i18n.SettingsXTestNoCredentials)
		if s.window != nil {
			s.window.Invalidate()
		}
		return
	}
	if s.xProbe == nil {
		s.state.XTestTested = true
		s.state.XTestRunning = false
		s.state.XTestOK = false
		s.state.XTestResult = s.t().T(i18n.SettingsXTestNoProbe)
		if s.window != nil {
			s.window.Invalidate()
		}
		return
	}

	// The system proxy is re-read rather than reused: the user is often testing
	// precisely because they just changed it, and the resolver's answer is cached
	// for thirty seconds.
	netproxy.DefaultResolver.Invalidate()

	s.state.XTestTested = true
	s.state.XTestRunning = true
	s.state.XTestOK = false
	s.state.XTestResult = s.t().T(i18n.SettingsXTestRunning)
	if s.window != nil {
		s.window.Invalidate()
	}

	probe := s.xProbe
	run := func() {
		err := probe.TestConnection(ctx)
		route := netproxy.DefaultResolver.Describe(context.Background())

		s.publish(func() {
			// Every string is built here, on the thread that owns state and the
			// translator, not in the goroutine.
			s.state.XTestRunning = false
			s.state.XTestOK = err == nil

			// The sentence is chosen into a local rather than read back from state: the
			// running note is still there, and testing state for "" would keep it.
			result := ""
			if err == nil {
				result = s.t().T(i18n.SettingsXTestOK)
			} else if kind := xsource.KindOf(err); kind != "" && kind != domain.ErrUnknown {
				// The classified sentence when the client produced one. It names the
				// cause ("no network", "the login expired"), which is what tells the
				// user whether to fix the cookies or the connection.
				result = s.syncFailureMessage(string(kind))
			}
			// Unclassified, or classified as "unknown": the transport's own words beat a
			// guess. "Sync failed. Please retry." on a DNS failure hides the one detail
			// this test exists to surface.
			if result == "" {
				result = err.Error()
			}
			s.state.XTestResult = result

			// The route is shown on both outcomes. On success it says which proxy
			// answered; on failure it is the difference between "the cookies are
			// wrong" and "nothing could leave this machine".
			if route == "" {
				route = s.t().T(i18n.SettingsXTestDirect)
			}
			s.state.XTestRoute = s.t().Tf(i18n.SettingsProxyRoute, route)
			if s.window != nil {
				s.window.Invalidate()
			}
		})
	}

	if s.window == nil {
		// No window means a test, and publish runs inline there: a goroutine would write
		// state while the test draws a frame on its own goroutine, which is a data race
		// rather than a slower answer.
		s.probeDone = nil
		run()
		return
	}

	done := make(chan struct{})
	s.probeDone = done
	go func() {
		defer close(done)
		run()
	}()
}
