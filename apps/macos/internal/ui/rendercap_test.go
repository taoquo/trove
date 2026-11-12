package ui

import (
	"context"
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestTheGridGrowsWhenTheReaderReachesItsEnd covers the end of a long result page.
//
// The grid builds one screenful at a time and used to offer a "Show more" control at the
// end. It now grows the way the reference does -- when the scroll reaches the end -- but
// with an anchor: one screenful per reach. The anchor is what makes a build pass safe,
// since a pass can run several times for one frame and the appended cards do not move the
// offset the page was asked at.
func TestTheGridGrowsWhenTheReaderReachesItsEnd(t *testing.T) {
	shell, db := newShell(t)
	total := ResultsRenderLimit + 20
	for i := 0; i < total; i++ {
		seed(t, db, fmt.Sprintf("%d", i), "ada", fmt.Sprintf("post %d", i), clockBase)
	}
	shell.Reload(context.Background())

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	if got := shell.state.RenderLimit; got != ResultsRenderLimit {
		t.Fatalf("RenderLimit = %d, want the first screenful %d", got, ResultsRenderLimit)
	}
	last := fmt.Sprintf("post %d", total-1)
	if tester.HasText(last) {
		t.Errorf("%q is drawn before the reader reached the end", last)
	}

	// At the top of the list nothing grows.
	tester.Frame()
	if got := shell.state.RenderLimit; got != ResultsRenderLimit {
		t.Errorf("RenderLimit = %d at the top, want %d", got, ResultsRenderLimit)
	}

	// At the end, one screenful appears -- and the rows that just arrived are drawn, not
	// merely counted.
	shell.resultsScroll.Y = 1_000_000
	tester.Frame()
	if got := shell.state.RenderLimit; got != total {
		t.Errorf("RenderLimit = %d at the end, want the rest of the page %d", got, total)
	}
	if !tester.HasText(last) {
		t.Errorf("%q is still missing after the grid grew; texts: %q", last, tester.Texts())
	}

	// Staying at the same offset asks for nothing more: the next screenful follows the
	// next reach, not the next frame.
	before := shell.state.RenderLimit
	tester.Frame()
	tester.Frame()
	if got := shell.state.RenderLimit; got != before {
		t.Errorf("RenderLimit moved from %d to %d without scrolling", before, got)
	}
}
