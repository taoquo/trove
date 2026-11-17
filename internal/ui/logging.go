package ui

import (
	"context"

	"github.com/taoquo/trove/internal/i18n"
)

// LogSwitch is the optional troubleshooting log.
//
// It is an interface rather than the concrete controller so the settings tests can
// record the switches without touching the process's loggers.
type LogSwitch interface {
	// Apply opens the log when enabled is true and closes it when it is false.
	Apply(enabled bool) error
}

// UseLogging hands the shell the troubleshooting log.
//
// It is optional: a shell built without storage, as the render tests are, keeps it nil
// and the Settings row that uses it is not drawn.
func (s *Shell) UseLogging(logs LogSwitch) {
	s.logging = logs
}

// setFileLog applies the switch and stores it.
//
// The log opens first: a stored "on" that failed to open would make the page promise a
// file that does not exist. The state keeps the reader's press either way -- a switch
// that springs back is a worse answer than a note saying why -- so a failure is recorded
// under the row, where settingsNote draws it.
func (s *Shell) setFileLog(ctx context.Context) {
	if s.logging != nil {
		if err := s.logging.Apply(s.state.FileLog); err != nil {
			s.state.LogNote = s.t().Tf(i18n.SettingsFileLogFailed, err.Error())
			s.invalidate()
			return
		}
	}
	s.state.LogNote = ""
	s.saveLogPreference(ctx)
	s.invalidate()
}

// saveLogPreference writes the troubleshooting switch to the stored preferences.
func (s *Shell) saveLogPreference(ctx context.Context) {
	if s.settings == nil {
		return
	}
	preferences, err := s.settings.Load(ctx)
	if err != nil {
		s.reportErr(err)
		return
	}
	preferences.Diagnostics.FileLog = s.state.FileLog
	if err := s.settings.Save(ctx, preferences); err != nil {
		s.reportErr(err)
	}
}

// applyLogPreference opens or closes the file log to match the stored choice.
//
// It is called where the other preferences are applied, so a launch and a restored
// settings backup both leave the log in the state the file says. A failure is a note,
// not an error: the library is fine without a log.
func (s *Shell) applyLogPreference(enabled bool) {
	if s.logging == nil {
		return
	}
	if err := s.logging.Apply(enabled); err != nil {
		s.state.LogNote = s.t().Tf(i18n.SettingsFileLogFailed, err.Error())
		return
	}
	s.state.LogNote = ""
}
