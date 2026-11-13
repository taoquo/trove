package ui

import (
	"context"
	"errors"
	"time"

	"github.com/taoquo/trove/internal/credentials"
	"github.com/taoquo/trove/internal/i18n"
	"github.com/taoquo/trove/internal/xsource"
)

// Getting an X session, from the two places it can come from.
//
// A desktop app has to be told where its X session comes from, and the two ways to
// tell it are one click (read a signed-in profile) and one paste (a Cookie header).
// Both end in the same Keychain entry, and both report into the same note line.

// browserReadTimeout bounds the profile read. The library snapshots and
// decrypts a cookie database; a hung one must not leave the button spinning.
const browserReadTimeout = 45 * time.Second

// readXCookiesFromBrowser fills the stored pair from a signed-in browser.
//
// It is the settings panel's control: read, save, report. The sync itself uses
// the same path automatically when a run dies on an expired session, so this
// button exists for the case where the reader would rather fix it by hand.
func (s *Shell) readXCookiesFromBrowser() {
	s.startBrowserRead(false)
}

// ReconnectXSession is the sidebar's control: read the browser, then retry the
// sync that failed.
//
// The settings button stops at "saved". This one exists because the failure
// being answered happened on the results page, and making the reader open a
// sheet, press a second button and close the sheet is three steps for one
// intention.
func (s *Shell) ReconnectXSession() {
	s.startBrowserRead(true)
}

// startBrowserRead runs one refresh and reports it.
//
// A shell with a window reads on its own goroutine and publishes onto the window's
// thread. A shell without one runs the whole thing inline, for the same reason
// publish does: a tester draws its frames on the test's goroutine, so a background
// write would race the rebuild the click is about to cause.
func (s *Shell) startBrowserRead(retrySync bool) {
	if s.credentials == nil || s.state.XBrowserReadRunning {
		return
	}
	if retrySync && s.state.SyncRunning {
		return
	}
	s.state.XBrowserReadRunning = true
	s.state.XBrowserReadNote = s.t().T(i18n.SettingsXBrowserReading)
	if s.window != nil {
		s.window.Invalidate()
	}

	done := make(chan struct{})
	s.browserReadDone = done

	run := func() {
		defer close(done)
		ctx, cancel := context.WithTimeout(context.Background(), browserReadTimeout)
		defer cancel()

		changed, err := s.credentials.RefreshXCredentialsFromBrowser(ctx)
		s.publish(func() {
			s.state.XBrowserReadRunning = false
			s.noteBrowserResult(changed, err)
			if retrySync && err == nil && changed {
				s.StartSync(context.Background())
			}
			if s.window != nil {
				s.window.Invalidate()
			}
		})
	}

	if s.window == nil {
		run()
		return
	}
	go run()
}

// noteBrowserResult turns a refresh into the one line the reader sees.
//
// The three failures have three fixes and must not share a sentence: a denied
// profile is a privacy setting, an empty profile means the browser is not
// signed in, and anything else is the library's own error. "Unchanged" is not a
// failure either -- the browser holds the same pair, so the session really has
// expired everywhere and only signing in again helps.
func (s *Shell) noteBrowserResult(changed bool, err error) {
	switch {
	case err == nil && changed:
		s.state.XCredentialsStored = true
		s.state.XBrowserReadNote = s.t().T(i18n.SettingsXBrowserSaved)
	case err == nil:
		s.state.XCredentialsStored = true
		s.state.XBrowserReadNote = s.t().T(i18n.SettingsXBrowserUnchanged)
	case errors.Is(err, credentials.ErrBrowserPermission):
		s.state.XBrowserReadNote = s.t().T(i18n.SettingsXBrowserNeedsAccess)
	case errors.Is(err, credentials.ErrBrowserNoCookies):
		s.state.XBrowserReadNote = s.t().Tf(i18n.SettingsXBrowserNoCookies, err.Error())
	default:
		s.state.XBrowserReadNote = s.t().Tf(i18n.SettingsXBrowserFailed, err.Error())
	}
}

// pasteProblem says which half of a paste is missing.
//
// The two names are not translated: auth_token and ct0 are spelled that way in
// every language, and the reader has to find exactly those two in DevTools.
func (s *Shell) pasteProblem(err error) string {
	switch {
	case errors.Is(err, xsource.ErrMissingAuthToken):
		return s.t().Tf(i18n.SettingsXCredentialMissing, "auth_token")
	case errors.Is(err, xsource.ErrMissingCSRFToken):
		return s.t().Tf(i18n.SettingsXCredentialMissing, "ct0")
	default:
		return s.t().T(i18n.SettingsXCredentialNothingFound)
	}
}
