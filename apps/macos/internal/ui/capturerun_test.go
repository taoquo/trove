package ui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/x-master/apps/macos/internal/i18n"
)

// TestTheRunningCaptureDoesNotAlsoOfferAnalyzeNow pins the picture, not the pixels.
//
// The autotag-running capture built the run state the progress bar reads but never set the
// flag the shell reads, so the sidebar saw "no run is marked active" beside a live run and
// drew Analyze now under a bar that was already running. Every screenshot of that state since
// round 453 has shown a contradiction a real window can never produce.
func TestTheRunningCaptureDoesNotAlsoOfferAnalyzeNow(t *testing.T) {
	shell, _ := newShell(t)
	if !shell.ApplyCaptureState("autotag-running") {
		t.Fatal("the capture state is unknown")
	}
	shell.SetLocale(i18n.EN, i18n.EN)

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	if tester.HasText(shell.t().T(i18n.ActionRunNow)) {
		t.Errorf("a running pass also offers Analyze now; texts: %q", tester.Texts())
	}
}
