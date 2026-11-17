package ui

import (
	"context"
	"os"
	"path/filepath"

	"github.com/taoquo/trove/internal/i18n"
)

// Backups is the daily copy of the library.
//
// It is an interface rather than the concrete runner so a test can count launches and
// reproduce a failure without writing a database.
type Backups interface {
	// Run writes today's copy and prunes old ones. It returns the path written, or
	// "" when today already had a copy.
	Run(ctx context.Context) (string, error)
	// Directory is where the copies live.
	Directory() string
}

// UseBackups hands the shell the daily copy and the way to show its folder.
//
// Both are optional together: a shell built without storage, as the render tests are,
// keeps them nil and the Settings rows that use them are not drawn.
func (s *Shell) UseBackups(backups Backups, reveal func(dir string) error) {
	s.backups = backups
	s.revealBackupDir = reveal
}

// StartAutomaticBackup writes today's copy when the reader has left the feature on.
//
// It runs off the window's thread: the vacuum rewrites the whole library, which on a
// large one takes seconds, and the first frame must not wait for it. A failure is not
// fatal -- the library is still there and the next launch tries again -- so it is
// recorded where the reader would look for it, in Settings › Data safety.
func (s *Shell) StartAutomaticBackup(ctx context.Context) {
	if s.backups == nil || !s.state.AutoBackup {
		return
	}

	s.runInBackground(func() {
		path, err := s.backups.Run(ctx)

		s.publish(func() {
			switch {
			case err != nil:
				s.state.BackupNote = s.t().Tf(i18n.SettingsBackupFailed, err.Error())
			case path != "":
				s.state.BackupNote = s.t().Tf(i18n.SettingsBackupDone, filepath.Base(path))
			}
			s.invalidate()
		})
	})
}

// saveBackupPreference stores the switch as an explicit choice.
//
// The stored value is a pointer so "absent" can mean on; writing a bool keeps the
// reader's "off" from being filled back in by the next upgrade.
func (s *Shell) saveBackupPreference(ctx context.Context) {
	if s.settings == nil {
		return
	}
	preferences, err := s.settings.Load(ctx)
	if err != nil {
		s.reportErr(err)
		return
	}
	enabled := s.state.AutoBackup
	preferences.Backup.AutoBackups = &enabled
	if err := s.settings.Save(ctx, preferences); err != nil {
		s.reportErr(err)
	}
}

// OpenBackupDirectory shows where the copies live.
//
// The folder is created first, because "open the folder" before the first copy exists
// should show an empty folder rather than fail or open the data directory beside it.
func (s *Shell) OpenBackupDirectory() {
	if s.backups == nil || s.revealBackupDir == nil {
		return
	}
	dir := s.backups.Directory()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		s.reportErr(err)
		return
	}
	if err := s.revealBackupDir(dir); err != nil {
		s.reportErr(err)
	}
}
