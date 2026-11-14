package ui

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/i18n"
	"github.com/taoquo/trove/internal/xsource"
)

// The sidebar's sync block is where the loading, empty and error states live.

// TestTheSyncingSidebarSaysItOnce pins the running block to one sentence.
//
// The progress line already begins "Syncing…", so drawing the bare state above it makes a
// running sync read "Syncing" / "Syncing… 3 pages · 60 items" in a block with room for one line.
// One label changes between the two.
func TestTheSyncingSidebarSaysItOnce(t *testing.T) {
	shell, _ := newShell(t)
	shell.SetLocale(i18n.EN)
	shell.state.SyncRunning = true
	shell.state.SyncProgress = syncProgressLabel(shell.t(),
		xsource.Progress{PagesCompleted: 3, Fetched: 60})

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	prefix := shell.t().T(i18n.StatusRunning)
	seen := make([]string, 0, 2)
	for _, text := range tester.Texts() {
		if strings.HasPrefix(text, prefix) {
			seen = append(seen, text)
		}
	}
	if len(seen) != 1 {
		t.Errorf("the running sync is announced %d times (%q), want once; texts: %q",
			len(seen), seen, tester.Texts())
	}
}

// TestAFailedSyncOffersTheWayToTryAgain covers the sentence with nothing behind it.
//
// "Sync failed, try again" was drawn alone: the app's sync command is an icon in the footer,
// and nothing tied the sentence to it. A labelled sync button belongs in this block whether it
// is idle, running or failed.
func TestAFailedSyncOffersTheWayToTryAgain(t *testing.T) {
	shell, _ := newShell(t)
	shell.SetLocale(i18n.EN)
	shell.state.LastSyncStatus = domain.SyncError
	shell.state.LastSyncErrorKind = "network"
	shell.state.LastSyncErrorSummary = "the network is unreachable"

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	if !tester.HasText(shell.t().T(i18n.ActionSync)) {
		t.Errorf("a failed sync offers no way to run it again; texts: %q", tester.Texts())
	}
	// A dead session is the one failure a retry cannot fix; there the reconnect pair answers,
	// and a second sync button would only invite the same failure again. The count is what
	// says so: the footer's own sync command is in this tree too, and it
	// carries its name as a label rather than only a tooltip, so "the word appears" no
	// longer distinguishes the block's retry from the global control.
	shell.state.LastSyncErrorKind = "unauthorized"
	tester.Frame()
	retries := 0
	for _, text := range tester.Texts() {
		if text == shell.t().T(i18n.ActionSync) {
			retries++
		}
	}
	if retries != 1 {
		t.Errorf("a dead session offers %d sync controls, want only the footer's one; texts: %q",
			retries, tester.Texts())
	}
	if !tester.HasText(shell.t().T(i18n.ActionReconnectX)) {
		t.Errorf("a dead session offers no reconnect; texts: %q", tester.Texts())
	}
}
