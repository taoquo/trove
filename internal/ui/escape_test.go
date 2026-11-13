package ui

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestEscapeLeavesTheSurfaceOnTop covers the key every desktop window answers.
//
// The list's keys were registered only while no overlay was open, and Escape was registered
// with them -- so with the settings sheet, the detail, the tag pages or the graph on screen
// Escape did nothing at all, and the only way out was the pointer. Escape is registered
// unconditionally now and closes one layer per press.
func TestEscapeLeavesTheSurfaceOnTop(t *testing.T) {
	cases := []struct {
		name   string
		open   func(shell *Shell)
		closed func(shell *Shell) bool
	}{
		{
			name:   "settings",
			open:   func(shell *Shell) { shell.state.SettingsOpen = true },
			closed: func(shell *Shell) bool { return !shell.state.SettingsOpen },
		},
		{
			name:   "suggestions",
			open:   func(shell *Shell) { shell.state.SuggestionsOpen = true },
			closed: func(shell *Shell) bool { return !shell.state.SuggestionsOpen },
		},
		{
			name:   "review",
			open:   func(shell *Shell) { shell.state.ReviewOpen = true },
			closed: func(shell *Shell) bool { return !shell.state.ReviewOpen },
		},
		{
			name:   "detail",
			open:   func(shell *Shell) { shell.state.DetailOpen = true },
			closed: func(shell *Shell) bool { return !shell.state.DetailOpen },
		},
		{
			name:   "graph",
			open:   func(shell *Shell) { shell.state.GraphMode = true },
			closed: func(shell *Shell) bool { return !shell.state.GraphMode },
		},
		{
			name:   "tag pages",
			open:   func(shell *Shell) { shell.state.TagMode = true },
			closed: func(shell *Shell) bool { return !shell.state.TagMode },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			shell, db := newShell(t)
			seed(t, db, "1", "ada", "a post", clockBase)
			shell.Reload(context.Background())
			tc.open(shell)

			tester := ui.NewTester(shell.View, 1180, 760)
			tester.Frame()
			tester.Key(0, ui.KeyEscape)
			tester.Frame()

			if !tc.closed(shell) {
				t.Errorf("Escape did not close the %s", tc.name)
			}
		})
	}
}

// TestEscapeStillClearsTheSearch covers the surface underneath them all.
//
// With nothing open, Escape keeps doing what it did: clearing the search box. The
// alternative reading -- "Escape now only closes panels" -- would have removed the one
// keyboard way to clear a query.
func TestEscapeStillClearsTheSearch(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.state.Query = "ada"
	shell.Reload(context.Background())

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	tester.Key(0, ui.KeyEscape)
	tester.Frame()

	if shell.state.Query != "" {
		t.Errorf("Query = %q, want it cleared", shell.state.Query)
	}
}

// TestEscapeClosesOnlyOneLayer keeps a press from taking two steps at once.
//
// The detail can sit over the tag pages, and the lightbox over the detail. Closing all of
// them together would lose the reader's place in one keystroke.
func TestEscapeClosesOnlyOneLayer(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(context.Background())
	shell.state.TagMode = true
	shell.state.DetailOpen = true

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	tester.Key(0, ui.KeyEscape)
	tester.Frame()

	if shell.state.DetailOpen {
		t.Error("the detail is still open")
	}
	if !shell.state.TagMode {
		t.Error("one press also left the tag pages")
	}
}
