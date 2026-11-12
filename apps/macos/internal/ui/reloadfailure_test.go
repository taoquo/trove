package ui

import (
	"context"
	"testing"

	"github.com/taoquo/trove/apps/macos/internal/i18n"
)

// TestAFailedRefreshKeepsTheRowsAndSaysSo pins the half of the error channel the empty
// state cannot cover.
//
// state.Err is drawn by the results page's empty state, so a failed read was only ever
// visible when it left nothing to show. A refresh that fails while rows are on screen --
// a follow-up read after a write, or the read behind a filter click -- keeps the last
// good page and used to say nothing: the reader saw an unchanged list and no reason to
// think it was stale. The shell's rule is that a failed action reports through the
// window's status line (shell-contract.md S37), so that is where this failure goes.
func TestAFailedRefreshKeepsTheRowsAndSaysSo(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "the first bookmark", clockBase)
	shell.Reload(context.Background())
	if got := len(shell.State().Data.Bookmarks); got != 1 {
		t.Fatalf("the first read left %d rows, want 1", got)
	}

	// The database goes away under the reader: every later read fails.
	if err := db.Close(); err != nil {
		t.Fatalf("close the database: %v", err)
	}

	shell.Reload(context.Background())

	if got := len(shell.State().Data.Bookmarks); got != 1 {
		t.Errorf("the failed refresh left %d rows, want the last good page's 1", got)
	}
	if got, want := shell.ToastText(), shell.t().T(i18n.LoadFailedTitle); got != want {
		t.Errorf("a failed refresh shows toast %q, want %q", got, want)
	}
}
