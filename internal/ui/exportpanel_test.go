package ui

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/internal/i18n"
)

// TestClickingExportAsksForAPathOffTheWindowThread covers the button that did nothing.
//
// Every file command opened its panel in the click handler. mygo's dialogs block until
// they are dismissed, and on the window's thread that means the handler does not return
// while the panel is up -- the window cannot draw, and when the panel fails to appear
// there is no state change to tell the reader the click landed. A report of "exporting
// does nothing at all" is exactly that shape. The toolkit says its dialogs may be called
// from any goroutine, so the panel belongs in the work the handler starts, not in the
// handler itself: the click returns, the row shows what is happening, and the panel is
// presented by the main thread from a post like every other piece of background work.
func TestClickingExportAsksForAPathOffTheWindowThread(t *testing.T) {
	shell, db := newShell(t)
	bundleService(t, shell, db)
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(context.Background())
	shell.SetLocale(i18n.EN, i18n.EN)

	shell.state.SettingsOpen = true
	shell.state.SettingsSection = SettingsSectionDataSafe

	asked := 0
	shell.saveFile = func(mygo.SaveDialogOptions) (string, error) {
		asked++
		return "", nil
	}
	work := capturedBackground(shell)

	tester := ui.NewTester(shell.View, 1180, 900)
	tester.Frame()
	if !tester.HasText(shell.t().T(i18n.ExportData)) {
		t.Fatalf("the data-safety page has no export button; texts: %q", tester.Texts())
	}
	if err := tester.Click(shell.t().T(i18n.ExportData)); err != nil {
		t.Fatalf("Click: %v", err)
	}

	if asked != 0 {
		t.Fatal("the save panel was opened inside the click handler: the window cannot draw until it is dismissed")
	}
	if *work == nil {
		t.Fatal("clicking export started no work, so nothing will ever ask where to save")
	}
	if !shell.state.BundleBusy {
		t.Error("the row does not say a file command is waiting for the reader")
	}
	if got, want := shell.state.LastBundleReport, shell.t().T(i18n.BundleChoosingWhere); got != want {
		t.Errorf("status line = %q, want %q", got, want)
	}

	// The work asks for the path, and a cancelled panel leaves nothing behind.
	(*work)()
	if asked != 1 {
		t.Errorf("the save panel was opened %d times, want 1", asked)
	}
	if shell.state.BundleBusy {
		t.Error("a cancelled panel left the shell busy, so every file command stays disabled")
	}
	if got := shell.state.LastBundleReport; got != "" {
		t.Errorf("a cancelled panel left the note %q on screen", got)
	}
}

// TestAnExportFinishesAfterThePanelAnswers is the same click carried through: the chosen
// path, the export and the receipt all happen after the handler has returned.
func TestAnExportFinishesAfterThePanelAnswers(t *testing.T) {
	shell, db := newShell(t)
	bundleService(t, shell, db)
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(context.Background())

	path := filepath.Join(t.TempDir(), "out.trovebundle")
	shell.saveFile = func(mygo.SaveDialogOptions) (string, error) { return path, nil }
	work := capturedBackground(shell)

	tester := ui.NewTester(shell.View, 1180, 900)
	shell.state.SettingsOpen = true
	shell.state.SettingsSection = SettingsSectionDataSafe
	tester.Frame()
	if err := tester.Click(shell.t().T(i18n.ExportData)); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if *work == nil {
		t.Fatal("clicking export started no work")
	}
	(*work)()

	if got, want := shell.state.LastBundleReport, shell.t().T(i18n.BundleExported); got != want {
		t.Errorf("status line = %q, want %q", got, want)
	}
	if !shell.state.LastBundleOK {
		t.Error("a finished export is filed like a progress note")
	}
	if shell.state.BundleBusy {
		t.Error("the export finished and the busy flag stayed set")
	}
}
