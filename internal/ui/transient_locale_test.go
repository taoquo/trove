package ui

import (
	"context"
	"testing"
)

// TestChangingTheLanguageDropsTheOldLanguageReports covers the eight one-shot message fields.
//
// Eight one-shot message fields hold reports of something that just happened -- "imported 3
// posts", "connection failed", "that is not a post link" -- and they are written in whatever
// language is current at the time, so switching languages leaves a sentence in the old one on
// screen unless the reports are dropped.
//
// Clearing is the answer rather than translating at draw time, because by then the numbers those
// sentences were built from are gone. The second half of this test is what keeps the clear from
// quietly growing: a persistent field must survive it.
func TestChangingTheLanguageDropsTheOldLanguageReports(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(context.Background())
	shell.state.LastSyncedAt = clockBase

	fill := func() {
		shell.state.SyncProgress = "report"
		shell.state.AutoTagStatus = "report"
		shell.state.LastBundleReport = "report"
		shell.state.AddProblem = "report"
		shell.state.ConnectionResult = "report"
		shell.state.TagActionMsg = "report"
		shell.state.TagCleanupMessage = "report"
		shell.state.ReviewApplied = "report"
	}
	reports := func() map[string]string {
		return map[string]string{
			"SyncProgress":      shell.state.SyncProgress,
			"AutoTagStatus":     shell.state.AutoTagStatus,
			"LastBundleReport":  shell.state.LastBundleReport,
			"AddProblem":        shell.state.AddProblem,
			"ConnectionResult":  shell.state.ConnectionResult,
			"TagActionMsg":      shell.state.TagActionMsg,
			"TagCleanupMessage": shell.state.TagCleanupMessage,
			"ReviewApplied":     shell.state.ReviewApplied,
		}
	}

	fill()
	if got := len(reports()); got != 8 {
		t.Fatalf("the fixture filled %d of 8 reports", got)
	}

	shell.SetLocale("zh-CN", "en")

	for name, value := range reports() {
		if value != "" {
			t.Errorf("after changing language %s still reads %q", name, value)
		}
	}

	// The clear is about reports, not about everything in state.
	if shell.state.LastSyncedAt.IsZero() {
		t.Error("changing language dropped the last sync time, which is state rather than a report")
	}
	if len(shell.state.Data.Bookmarks) == 0 {
		t.Error("changing language dropped the loaded bookmarks")
	}

	// And the same holds on the capture path, which sets the language a different way.
	fill()
	shell.ApplyCaptureLocale("en")
	for name, value := range reports() {
		if value != "" {
			t.Errorf("after a capture language change %s still reads %q", name, value)
		}
	}
}
