package ui

import (
	"context"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// The interface is a long-lived window: a user opens and closes the workspace
// hundreds of times in a session. Anything that grows per open is a leak the user
// eventually feels as a slow, memory-hungry app.
//
// docs/ui-spec.md asks for thirty cycles with no sustained growth. Thirty is enough
// to expose a per-open leak: a leak of even a few kilobytes becomes visible once
// the warm-up baseline is subtracted.

const leakCycles = 30

// TestRepeatedWorkspaceOpensDoNotLeak opens and closes the workspace repeatedly and
// asserts that nothing grows per cycle.
//
// Three things are measured because they leak differently: goroutines (a leaked
// worker), file descriptors (a leaked SQLite handle, which a desktop app is most
// likely to get wrong), and heap (a retained buffer).
func TestRepeatedWorkspaceOpensDoNotLeak(t *testing.T) {
	if testing.Short() {
		t.Skip("the leak check runs thirty cycles")
	}

	shell, _ := buildUILibrary(t, 2000)
	ctx := context.Background()

	// A warm-up: the first cycles allocate indexes and prepared statements, which
	// is setup rather than a leak. Measuring from cold would report that setup as
	// growth on every run and hide a real one.
	for i := 0; i < 5; i++ {
		shell.Reload(ctx)
		ui.NewTester(shell.View, 1180, 760).Frame()
	}

	runtime.GC()
	var baseline runtime.MemStats
	runtime.ReadMemStats(&baseline)
	baseGoroutines := runtime.NumGoroutine()
	baseDescriptors, descriptorsAvailable := countOpenFiles()

	for cycle := 0; cycle < leakCycles; cycle++ {
		shell.Reload(ctx)

		tester := ui.NewTester(shell.View, 1180, 760)
		tester.Frame()

		// A detail view, then a panel, then both closed: the panels allocate, and
		// closing them is where a leak would hide.
		if bookmarks := shell.State().Data.Bookmarks; len(bookmarks) > 0 {
			shell.State().Selected = bookmarks[cycle%len(bookmarks)].TweetID
		}
		tester.Frame()

		shell.OpenSettings()
		tester.Frame()
		shell.closePanels()
		tester.Frame()
	}

	// Let anything that was going to finish, finish.
	//
	// This waits for a *negative* -- for nothing more to happen -- so the window has to be longer
	// than the slowest build's time to finish, not the fastest. It was 50ms, which is a window
	// tuned without -race in mind; round 403 found the same mistake one file over, where a 10ms
	// window made a chained-run test fail about one time in three under -race. A goroutine that is
	// still winding down at the sample is indistinguishable from a leaked one, so the check fails
	// for a reason that has nothing to do with leaking.
	time.Sleep(250 * time.Millisecond)
	runtime.GC()

	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	afterGoroutines := runtime.NumGoroutine()

	growth := afterGoroutines - baseGoroutines
	if growth > 2 {
		t.Errorf("goroutines grew by %d over %d cycles (%d -> %d)",
			growth, leakCycles, baseGoroutines, afterGoroutines)
	}

	heapGrowth := int64(after.HeapAlloc) - int64(baseline.HeapAlloc)
	const maxGrowthPerCycle = 256 << 10 // 256 KiB
	if heapGrowth > int64(leakCycles)*maxGrowthPerCycle {
		t.Errorf("heap grew by %.1f MiB over %d cycles (%.1f KiB per cycle)",
			float64(heapGrowth)/1024/1024, leakCycles, float64(heapGrowth)/1024/leakCycles)
	}

	// Descriptors are checked only where the count is available: a missing count
	// must not turn into a silent pass for the checks above.
	if descriptorsAvailable {
		if after, _ := countOpenFiles(); after-baseDescriptors > 0 {
			t.Errorf("open descriptors grew by %d over %d cycles (%d -> %d); "+
				"a SQLite handle is not being released",
				after-baseDescriptors, leakCycles, baseDescriptors, after)
		}
	}

	t.Logf("over %d cycles: goroutines %d -> %d, heap %+.1f KiB per cycle, descriptors available=%v",
		leakCycles, baseGoroutines, afterGoroutines, float64(heapGrowth)/1024/leakCycles, descriptorsAvailable)
}

// countOpenFiles reports how many descriptors the process holds.
//
// It reads /dev/fd rather than /proc, because this is macOS. The second result is
// false when the directory cannot be read, so the caller can skip rather than treat
// an unavailable count as zero.
func countOpenFiles() (int, bool) {
	entries, err := os.ReadDir("/dev/fd")
	if err != nil {
		return 0, false
	}
	return len(entries), true
}
