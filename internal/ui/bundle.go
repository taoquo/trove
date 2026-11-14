package ui

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/egoist/mygo"
	"github.com/taoquo/trove/internal/bundle"
	"github.com/taoquo/trove/internal/credentials"
	"github.com/taoquo/trove/internal/i18n"
	"github.com/taoquo/trove/internal/importexport"
	"github.com/taoquo/trove/internal/usecase/workspace"
)

// BundleRequest is a file the user chose and what to do with it.
type BundleRequest struct {
	Path string
	// Restore replaces the library; Import merges into it. The two are separate
	// commands because their consequences are different enough that one button
	// cannot honestly cover both.
	Restore bool
}

// UseImportExport attaches the bundle service.
func (s *Shell) UseImportExport(service *importexport.Service) { s.importexport = service }

// UseCredentials attaches the credential store, which the settings panel uses to
// manage provider keys.
func (s *Shell) UseCredentials(store *credentials.Store) { s.credentials = store }

// BundleFilter is the file dialog's filter for a bundle.
func BundleFilter() mygo.FileFilter {
	return mygo.FileFilter{Name: "trove bundle", Extensions: []string{"trovebundle"}}
}

// ExportBundle writes a bundle to a file the user chooses.
//
// A cancelled dialog is not an error: the user changed their mind, and reporting
// that as a failure would put a message on screen for doing nothing.
//
// The panel and the export both run off the window's thread. mygo's dialogs block
// until they are dismissed and may be called from any goroutine, so opening the panel
// in the click handler would mean the handler did not return while the panel was up: the
// window could not draw, and if the panel did not appear the click would leave no trace at
// all. So the handler returns at once, the row says it is waiting for a save
// location, and the panel is presented by the main thread from a post like the rest
// of this command. The bundle is the whole library, so the marshalling
// stays off the window's thread too -- that is the freeze this seam exists to prevent.
func (s *Shell) ExportBundle(ctx context.Context) error {
	if s.importexport == nil || s.state.BundleBusy {
		return nil
	}

	options := mygo.SaveDialogOptions{
		Parent:      s.window,
		Title:       s.t().T(i18n.BundleExportTitle),
		DefaultPath: "trove.trovebundle",
		Filters:     []mygo.FileFilter{BundleFilter()},
	}
	s.beginBundleWork(i18n.BundleChoosingWhere)
	s.runInBackground(func() {
		path, err := s.savePanel(options)
		if err != nil {
			s.publish(func() { s.finishBundleWork(err) })
			return
		}
		if path == "" {
			s.publish(func() { s.finishBundleWork(nil) })
			return
		}

		s.publish(func() { s.state.LastBundleReport = s.t().T(i18n.BundleExporting) })
		raw, exportErr := s.importexport.Export(ctx, importexport.DefaultExportOptions())
		var writeErr error
		if exportErr == nil {
			// The file holds the user's whole library, so it is written private.
			writeErr = os.WriteFile(path, raw, 0o600)
		}

		s.publish(func() {
			if exportErr != nil {
				s.finishBundleWork(exportErr)
				return
			}
			if writeErr != nil {
				s.finishBundleWork(writeErr)
				return
			}
			s.state.BundleBusy = false
			s.state.LastBundleReport = s.t().T(i18n.BundleExported)
			s.state.LastBundleOK = true
			s.invalidate()
		})
	})
	return nil
}

// finishBundleWork ends a file command that produced no file.
//
// A cancelled panel is not an error, and it is not a result either: the note that said
// the window was waiting goes away, and the reader is back where they started. An
// error keeps its sentence and is drawn the way every other failure is.
func (s *Shell) finishBundleWork(err error) {
	s.state.BundleBusy = false
	s.state.LastBundleReport = ""
	s.state.LastBundleOK = false
	if err != nil {
		s.reportErr(err)
	}
	s.invalidate()
}

// ExportLegacyBackup writes the library as flat JSON.
//
// A bundle is this build's own container. The legacy JSON is the format the app already
// imports (legacy.go), so the two directions agree; this is the file a user needs to carry
// their library elsewhere.
func (s *Shell) ExportLegacyBackup(ctx context.Context) error {
	if s.importexport == nil || s.state.BundleBusy {
		return nil
	}

	options := mygo.SaveDialogOptions{
		Parent:      s.window,
		Title:       s.t().T(i18n.BundleExportLegacy),
		DefaultPath: "trove-workspace-" + time.Now().Format("2006-01-02") + ".json",
		Filters:     []mygo.FileFilter{{Name: "JSON", Extensions: []string{"json"}}},
	}
	// The panel waits for the reader, so it cannot run in the handler for the same
	// reason as the bundle export above.
	s.beginBundleWork(i18n.BundleChoosingWhere)
	s.runInBackground(func() {
		path, err := s.savePanel(options)
		if err != nil {
			s.publish(func() { s.finishBundleWork(err) })
			return
		}
		if path == "" {
			s.publish(func() { s.finishBundleWork(nil) })
			return
		}

		s.publish(func() { s.state.LastBundleReport = s.t().T(i18n.BundleExporting) })
		raw, exportErr := s.importexport.ExportLegacyJSON(ctx)
		var writeErr error
		if exportErr == nil {
			// A backup holds the whole library, so it is written private like the bundle.
			writeErr = os.WriteFile(path, raw, 0o600)
		}

		s.publish(func() {
			if exportErr != nil {
				s.finishBundleWork(exportErr)
				return
			}
			if writeErr != nil {
				s.finishBundleWork(writeErr)
				return
			}
			s.state.BundleBusy = false
			s.state.LastBundleReport = s.t().T(i18n.BundleExported)
			s.state.LastBundleOK = true
			s.ShowToast(s.t().T(i18n.BundleExported))
			s.invalidate()
		})
	})
	return nil
}

// UseExternalOpener installs the browser opener.
func (s *Shell) UseExternalOpener(open func(url string) error) { s.openExternal = open }

// OpenSelectedOnX opens the selected bookmark in a browser.
//
// A bookmark manager that cannot take you to the bookmark is missing the one thing the
// bookmark is for, so opening the selected row is a primary action.
//
// A missing URL says so through the same toast as copying one, rather than failing
// quietly.
func (s *Shell) OpenSelectedOnX() bool {
	row, found := s.selectedBookmark()
	if !found {
		s.ShowToast(s.t().T(i18n.ToastNoLink))
		return false
	}
	return s.OpenOnX(row)
}

// OpenOnX opens one bookmark's link in the browser.
//
// The row is the argument because the card's own action is drawn on a row, not on the
// selection: reading Selected made the button say "No link" with nothing selected and
// open the wrong post after a detail had been opened. The context menu already acts on
// the row under the pointer; this is the same rule for the button.
func (s *Shell) OpenOnX(row workspace.BookmarkRow) bool {
	if row.TweetURL == "" || s.openExternal == nil {
		s.ShowToast(s.t().T(i18n.ToastNoLink))
		return false
	}
	if err := s.openExternal(row.TweetURL); err != nil {
		s.reportErr(err)
		return false
	}
	return true
}

// AskResetLocalData opens the confirmation for emptying the library.
//
// Emptying the library is destructive, so danger actions fail closed: the dialog is the
// only way in and a dismissal changes nothing.
func (s *Shell) AskResetLocalData() {
	s.state.ConfirmResetData = true
	if s.window != nil {
		s.window.Invalidate()
	}
}

// ConfirmResetLocalDataNow empties the library.
func (s *Shell) ConfirmResetLocalDataNow(ctx context.Context) {
	s.state.ConfirmResetData = false
	if s.importexport == nil || s.state.BundleBusy {
		return
	}

	// Emptying a library is a delete over every table, so it runs off the window's
	// thread like the file commands do.
	s.beginBundleWork(i18n.ResettingData)
	s.runInBackground(func() {
		err := s.importexport.ResetLocalData(ctx)
		s.publish(func() {
			s.state.BundleBusy = false
			if err != nil {
				s.reportErr(err)
				s.invalidate()
				return
			}
			s.state.Err = ""
			s.state.LastBundleReport = ""
			s.clearSelection()
			s.Reload(ctx)
			// The wipe also removed the preferences and the Keychain accounts, and this
			// window would still hold them. Reloading in place after a reset prevents that.
			s.reloadAfterReset(ctx)
			// The one destructive command that must leave a word behind: an empty library
			// is also what a failed load looks like, so the data-safety panel reports
			// copy.localDataReset.
			s.state.LastBundleReport = s.t().T(i18n.LocalDataResetDone)
			s.state.LastBundleOK = true
			s.invalidate()
		})
	})
}

// AskBundleImport reads the chosen file, previews it, and asks for confirmation.
//
// The preview is computed before the question is asked, so the user sees how many
// records the bundle holds before agreeing to anything.
func (s *Shell) AskBundleImport(ctx context.Context, restore bool) error {
	if s.importexport == nil || s.state.BundleBusy {
		return nil
	}

	title := s.t().T(i18n.BundleImportTitle)
	if restore {
		title = s.t().T(i18n.BundleRestoreTitle)
	}

	options := mygo.OpenDialogOptions{
		Parent:  s.window,
		Title:   title,
		Filters: []mygo.FileFilter{BundleFilter()},
	}
	// The panel waits for the reader, so it runs in the work like the save panels; the
	// reading half below is already off the window's thread.
	s.beginBundleWork(i18n.BundleChoosingWhere)
	s.runInBackground(func() {
		paths, err := s.openPanel(options)
		if err != nil {
			s.publish(func() { s.finishBundleWork(err) })
			return
		}
		if len(paths) == 0 {
			s.publish(func() { s.finishBundleWork(nil) })
			return
		}
		path := paths[0]
		s.publish(func() { s.state.LastBundleReport = s.t().T(i18n.BundleReading) })

		// Reading and validating the file is the slow half: a bundle is the whole library,
		// and Preview parses all of it before the question can be asked. On the window's
		// thread that was a freeze with nothing on screen to explain it.
		raw, readErr := os.ReadFile(path)
		var preview bundle.Preview
		if readErr == nil {
			preview, readErr = s.importexport.Preview(ctx, raw)
		}

		s.publish(func() {
			s.state.BundleBusy = false
			if readErr != nil {
				// A rejected bundle never reaches the confirmation, so the user is told
				// what is wrong instead of being asked to approve a file that cannot load.
				s.state.LastBundleReport = ""
				s.reportErr(readErr)
				s.invalidate()
				return
			}

			body := s.t().Tf(i18n.BundleImportBody, preview.NewRecords, preview.ExistingRecords)
			label := s.t().T(i18n.ActionImport)
			if restore {
				// The backup is described before anything is replaced: how many bookmarks and
				// tags it holds and when it was made. The bookmark count alone could not tell
				// two archives of the same size apart by date.
				counts := preview.Manifest.Counts
				body = fmt.Sprintf("%d %s \u00b7 %d %s \u00b7 %s",
					counts.Bookmarks, s.t().T(i18n.BookmarksCountLabel),
					counts.Tags, s.t().T(i18n.TagsCountLabel),
					preview.Manifest.ExportedAt)
				label = s.t().T(i18n.ActionRestore)
			}

			s.state.LastBundleReport = ""
			s.bundle.Arm(BundleRequest{Path: path, Restore: restore}, title, body, label,
				s.t().T(i18n.ActionCancel))
			s.invalidate()
		})
	})
	return nil
}

// applyBundle runs the confirmed import or restore.
//
// It reads the file again rather than caching its bytes: the confirmation clears
// before the work starts, and re-reading keeps the bytes out of the window's state
// for as long as a dialog is on screen.
func (s *Shell) applyBundle(ctx context.Context) error {
	request, approved := s.bundle.Approved()
	s.bundle.Clear()

	if !approved || s.importexport == nil {
		return nil
	}

	// The confirmation is answered, so the sheet is already gone; the import below it
	// writes every row of a backup and must not run in the handler that closed it.
	s.beginBundleWork(i18n.BundleImporting)
	s.runInBackground(func() {
		raw, err := os.ReadFile(request.Path)
		var report importexport.ImportReport
		if err == nil {
			if request.Restore {
				report, err = s.importexport.Restore(ctx, raw)
			} else {
				report, err = s.importexport.Import(ctx, raw)
			}
		}

		s.publish(func() {
			s.state.BundleBusy = false
			if err != nil {
				s.state.LastBundleReport = ""
				s.state.LastBundleOK = false
				s.reportErr(err)
				s.invalidate()
				return
			}

			s.state.LastBundleReport = s.t().Tf(i18n.BundleReport,
				report.BookmarksInserted, report.BookmarksUpdated, report.TagsInserted)
			s.state.LastBundleOK = true

			// A restore or an import can change everything the workspace shows.
			s.clearSelection()
			s.Reload(ctx)
			s.invalidate()
		})
	})
	return nil
}

// UseSyncer attaches the sync service.
