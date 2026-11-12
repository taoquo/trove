package ui

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	engine "github.com/taoquo/x-master/apps/macos/internal/autotag"
)

// TestSidebarShowsTheRunProgress covers the block the reference picture puts between the title and
// the sync controls, and which this build did not render anywhere except inside the settings panel.
//
// Round 453 found it by comparing a capture against the reference: the reference's most prominent
// sidebar block -- phase, processed/total, a bar, elapsed and remaining, and a stop button -- was
// absent, while RunProgressView and runProgressBar had both existed for many rounds. The data model
// and the renderer were finished; the wiring was not. That is the same shape as the sync time
// showing a stored timestamp: a piece that was built and never placed.
func TestSidebarShowsTheRunProgress(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(context.Background())

	// Nothing running: the slot is empty, so the two states do not look alike.
	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	if tester.HasText("120/2000") {
		t.Fatal("a run's progress is shown when no run is running")
	}

	// The capture seam stands in for a live run, which a test cannot start against a provider.
	shell.captureRun = &engine.RunState{
		RunID:     "test",
		StartedAt: time.Now().Add(-39 * time.Second),
		UpdatedAt: time.Now(),
		Phase:     engine.PhaseLLM,
		Processed: 120,
		Total:     2000,
		Status:    engine.StatusRunning,
	}
	tester.Frame()

	// The three parts the reference shows: what it is doing, how far it has come, and for how long.
	for _, want := range []struct {
		needle string
		why    string
	}{
		{"120/2000", "the processed/total count"},
		{"Analyzing", "the status line"},
		{"39s", "the elapsed time"},
	} {
		if !tester.HasText(want.needle) {
			t.Errorf("the sidebar is missing %s (%q); texts: %q", want.why, want.needle, tester.Texts())
		}
	}

	// And a stop control, because a run with no way out is what this pairing was written to avoid.
	stopFound := false
	for _, text := range tester.Texts() {
		if strings.Contains(strings.ToLower(text), "stop") {
			stopFound = true
		}
	}
	if !stopFound {
		t.Errorf("the progress block has no stop control; texts: %q", tester.Texts())
	}
}
