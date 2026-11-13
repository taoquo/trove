package ui

import (
	"context"
	"testing"

	"github.com/taoquo/trove/internal/domain"
)

// TestTheStatusBadgeSurvivesALanguageChange is the regression test for how the sync status is
// stored.
//
// The shell kept the *translated* word in state and compared it against freshly translated words to
// decide what the badge shows. That works until the language changes: the stored word is English,
// the comparison is against Chinese, the comparison fails, and a workspace that had just failed a
// sync is shown as idle. The value was right and the type was wrong.
//
// It asks workspaceStatus() rather than reading the frame, because the first version did read the
// frame and could not tell the badge from the toolbar: 失败 appears in the toolbar's status line
// whatever the badge does, so the frame assertion passed with the bug still in place. A test that
// passes for the wrong reason is worse than no test.
func TestTheStatusBadgeSurvivesALanguageChange(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(context.Background())

	for _, status := range []domain.SyncStatus{
		domain.SyncError, domain.SyncSuccess, domain.SyncPartialSuccess, domain.SyncCancelled,
	} {
		shell.state.LastSyncStatus = status
		if got := shell.workspaceStatus(); got != status {
			t.Errorf("in English, status %q reached the badge as %q", status, got)
		}

		shell.SetLocale("zh-CN", "en")
		if got := shell.workspaceStatus(); got != status {
			t.Errorf("after switching to Chinese, status %q reached the badge as %q -- the badge "+
				"compares a stored word against a fresh translation again", status, got)
		}
		shell.SetLocale("en", "en")
	}

	// And the idle case stays idle, so the mapping is not simply passing everything through.
	shell.state.LastSyncStatus = domain.SyncIdle
	if got := shell.workspaceStatus(); got != "" {
		t.Errorf("idle reached the badge as %q, want the empty status", got)
	}
}
