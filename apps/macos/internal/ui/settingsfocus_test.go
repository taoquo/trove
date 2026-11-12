package ui

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/apps/macos/internal/i18n"
)

// TestSettingsFindFocusIsSpentByAFrame is the Cmd-F defect: the request that puts
// the cursor in the settings find field was handed to every later frame, so it kept
// taking the cursor back. Clicking a number field and typing then sent the characters
// to the find box instead.
func TestSettingsFindFocusIsSpentByAFrame(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(context.Background())

	shell.OpenSettings()
	shell.State().SettingsSection = SettingsSectionSync

	tester := ui.NewTester(shell.View, 1180, 2400)
	tester.Frame()

	// Cmd-F on the settings page puts the cursor in that page's find field.
	findBox := shell.t().T(i18n.SettingsSearchPlaceholder)
	shell.FocusSearch()
	tester.Frame()
	if !tester.Focused(findBox) {
		t.Fatalf("Cmd-F did not put the cursor in the find field; texts: %q", tester.Texts())
	}

	// The request is one frame's worth: with the cursor already there, another field
	// can take it and typing lands in that field rather than in the find box.
	label := shell.t().T(i18n.SettingsStopBufferPages)
	rect, ok := tester.Find(label)
	if !ok {
		t.Fatalf("the buffer-pages field has no rect; texts: %q", tester.Texts())
	}
	// The number field is the row's right-hand control, 72 wide.
	tester.ClickAt(1110, rect.Y+rect.H/2)
	tester.Frame()
	if !tester.Focused(label) {
		t.Fatalf("the buffer-pages field did not take the cursor; texts: %q", tester.Texts())
	}

	tester.Type("7")
	tester.Frame()
	if got := shell.State().StopBufferPages; got != 7 {
		t.Errorf("typing 7 into the buffer-pages field set it to %v", got)
	}
	if got := shell.State().SettingsQuery; got != "" {
		t.Errorf("the characters went to the find field: query %q", got)
	}
}
