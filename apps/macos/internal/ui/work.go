package ui

import (
	"context"

	"github.com/egoist/mygo"

	"github.com/taoquo/x-master/apps/macos/internal/i18n"
)

// The seams every slow command goes through.
//
// The window's thread draws the interface and runs the event handlers. A command
// that reads a backup of the whole library, parses it and writes thousands of rows
// there stops drawing until it finishes: the window becomes a beachball, which is
// what "clicking import or export freezes the app" was. Sync, auto-tagging and both
// connection probes already work this way -- their work runs in a goroutine and
// comes back through Window.Update -- and these seams put the file commands on the
// same contract.
//
// They are fields for a second reason: mygo.Dialog opens a real panel, so a command
// that calls it directly can only be tested by a person with a mouse. A test answers
// the panel through saveFile/openFiles and sees the rest of the command run.

// savePanel is mygo.Dialog.Save behind a field, so a test can answer it.
func (s *Shell) savePanel(options mygo.SaveDialogOptions) (string, error) {
	if s.saveFile != nil {
		return s.saveFile(options)
	}
	return mygo.Dialog.Save(options)
}

// openPanel is mygo.Dialog.Open behind a field, for the same reason.
func (s *Shell) openPanel(options mygo.OpenDialogOptions) ([]string, error) {
	if s.openFiles != nil {
		return s.openFiles(options)
	}
	return mygo.Dialog.Open(options)
}

// runInBackground runs work that must not hold the window's thread.
//
// A shell without a window is a test: publish runs inline there, and so does this,
// for the same reason -- a goroutine writing state while the tester draws a frame on
// its own goroutine is a data race rather than a faster answer.
func (s *Shell) runInBackground(work func()) {
	if s.background != nil {
		s.background(work)
		return
	}
	if s.window == nil {
		work()
		return
	}
	go work()
}

// beginBundleWork says a file command is running.
//
// The line goes where the last report goes -- the toolbar's status area -- so the
// window says what it is doing instead of looking hung, and BundleBusy makes a
// second click on the same button a no-op rather than a second export racing the
// first for the same file.
func (s *Shell) beginBundleWork(note i18n.Key) {
	s.state.BundleBusy = true
	s.state.LastBundleReport = s.t().T(note)
	s.state.LastBundleOK = false
	s.invalidate()
}

// libraryWrite is what a library-wide edit reports when it finishes.
//
// It carries a count rather than a sentence: the work runs off the window's thread
// and the translator belongs to the window, so the sentence is built in done.
type libraryWrite struct {
	count int
	err   error
}

// runLibraryWrite runs an edit that can touch thousands of rows off the window's
// thread.
//
// Merging a tag and applying a vocabulary review both read every tag relation and
// rewrite the matching ones (workspace.MergeTags). At 50,000 bookmarks that is the
// slowest thing a click can do, and it used to happen in the handler. Mutating is what
// the shell says meanwhile, and it also makes a second click a no-op rather than a
// second merge racing the first.
//
// done runs on the window's thread after a successful edit, and says what happened.
func (s *Shell) runLibraryWrite(note i18n.Key, work func() libraryWrite, done func(libraryWrite)) bool {
	if s.state.Mutating {
		return false
	}

	s.state.Mutating = true
	s.state.MutationNote = s.t().T(note)
	s.invalidate()

	s.runInBackground(func() {
		outcome := work()
		s.publish(func() {
			s.state.Mutating = false
			s.state.MutationNote = ""
			if outcome.err != nil {
				s.reportErr(outcome.err)
				s.invalidate()
				return
			}
			if done != nil {
				done(outcome)
			}
			// The edit changed what the list would show, so the page is refreshed the
			// same way every other click refreshes it.
			s.ReloadAsync(context.Background())
			s.invalidate()
		})
	})
	return true
}
