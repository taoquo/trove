package ui

import (
	"context"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/internal/i18n"
)

// TestFiveKeystrokesAskOneQuery covers the search field's debounce.
//
// Every keystroke used to run a query, and each query owns the only database connection
// while it runs, so a ten-character word put ten reads between the field and the list.
// One pause per burst asks once, and only the newest pause runs.
func TestFiveKeystrokesAskOneQuery(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "rust async", clockBase)
	shell.Reload(ctx)

	var pending []func()
	shell.after = func(delay time.Duration, fn func()) *time.Timer {
		if delay != SearchDebounce {
			t.Errorf("the field waits %v, want %v", delay, SearchDebounce)
		}
		pending = append(pending, fn)
		// The test drives the timers itself; nothing may fire behind its back.
		return time.AfterFunc(time.Hour, func() {})
	}

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	shell.FocusSearch()
	tester.Frame()

	before := shell.reloadSeq.Load()
	for _, letter := range []string{"r", "u", "s", "t", "s"} {
		tester.Type(letter)
		tester.Frame()
	}
	if len(pending) != 5 {
		t.Fatalf("five keystrokes scheduled %d reads, want five timers", len(pending))
	}
	if !tester.HasText(shell.t().T(i18n.ToolbarSearching)) {
		t.Errorf("the window does not say it is searching; texts: %q", tester.Texts())
	}
	if got := shell.reloadSeq.Load(); got != before {
		t.Errorf("a keystroke asked for a read immediately (%d new requests)", got-before)
	}

	// Every timer fires; only the newest owns the query.
	for _, fire := range pending {
		fire()
	}
	if got := shell.reloadSeq.Load() - before; got != 1 {
		t.Errorf("the burst asked for %d reads, want one", got)
	}
	if shell.state.Searching {
		t.Error("the searching note stayed after the read was asked for")
	}
	if got := shell.state.Data.Query.Filter.Query; got != "rusts" {
		t.Errorf("the read used %q, want the last keystroke's query", got)
	}

	tester.Frame()
	if tester.HasText(shell.t().T(i18n.ToolbarSearching)) {
		t.Error("the searching note is still on screen after the read landed")
	}
}
