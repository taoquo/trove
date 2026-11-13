package ui

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/i18n"
)

// TestReachingTheEndReadsTheNextPage is the interaction that replaced the "load more"
// button: the end of the grid is the request.
func TestReachingTheEndReadsTheNextPage(t *testing.T) {
	shell, db := newShell(t)
	total := domain.DefaultLibraryPageSize + 10
	for i := 0; i < total; i++ {
		seed(t, db, fmt.Sprintf("%d", i), "ada", fmt.Sprintf("post %d", i), clockBase)
	}
	shell.Reload(context.Background())
	if !shell.state.Data.HasMore {
		t.Fatalf("the fixture has one page: %d rows", len(shell.state.Data.Bookmarks))
	}

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	if got := len(shell.state.Data.Bookmarks); got != domain.DefaultLibraryPageSize {
		t.Fatalf("%d bookmarks after the first read, want one page", got)
	}

	// The first page stands until the reader reaches its end.
	tester.Frame()
	if got := len(shell.state.Data.Bookmarks); got != domain.DefaultLibraryPageSize {
		t.Errorf("a frame at the top read another page: %d rows", got)
	}

	shell.resultsScroll.Y = 1_000_000
	tester.Frame()

	if got := len(shell.state.Data.Bookmarks); got != total {
		t.Errorf("%d bookmarks after reaching the end, want %d", got, total)
	}
	if shell.state.Data.HasMore {
		t.Error("the whole fixture is loaded and the page still says there is more")
	}
	if shell.state.LoadingMore {
		t.Error("the page arrived and the loading flag stayed set")
	}
	// The rows that arrived are drawn, not merely counted.
	if !tester.HasText(fmt.Sprintf("post %d", total-1)) {
		t.Errorf("the appended page is not drawn; texts: %q", tester.Texts())
	}
}

// TestTheNextPageWaitsForTheNextReach keeps the anchor honest: a page arrives once per
// reach of the end, not once per frame. Without it, a synchronous read (a test, or an
// update delivered inline) would chain through the whole library in one build pass.
func TestTheNextPageWaitsForTheNextReach(t *testing.T) {
	shell, db := newShell(t)
	total := domain.DefaultLibraryPageSize * 3
	for i := 0; i < total; i++ {
		seed(t, db, fmt.Sprintf("%d", i), "ada", fmt.Sprintf("post %d", i), clockBase)
	}
	shell.Reload(context.Background())

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	// The bottom of the grid, as the layout measured it -- not an unreachable number,
	// which every build would re-clamp to a different place and read as a new reach.
	shell.resultsScroll.Y = shell.resultsScroll.MaxY
	tester.Frame()

	loaded := len(shell.state.Data.Bookmarks)
	if loaded != domain.DefaultLibraryPageSize*2 {
		t.Fatalf("%d bookmarks after the first reach, want two pages", loaded)
	}

	// Standing at the same offset asks for nothing: the appended rows did not move it.
	tester.Frame()
	tester.Frame()
	if got := len(shell.state.Data.Bookmarks); got != loaded {
		t.Errorf("two frames at the same offset read %d more rows", got-loaded)
	}

	// Scrolling again (a new offset) asks for the next one.
	shell.resultsScroll.Y = shell.resultsScroll.MaxY
	tester.Frame()
	if got := len(shell.state.Data.Bookmarks); got != total {
		t.Errorf("%d bookmarks after the second reach, want %d", got, total)
	}
}

// TestTheListAlsoContinuesAtItsEnd covers the other layout. It is tracked with TrackScroll
// like the grid, so the same rule applies; the list's own Visible/AtEnd are not used
// because reading them every frame makes MyGo rebuild a list whose rows are still being
// measured, which settles on a page revealed one row at a time.
func TestTheListAlsoContinuesAtItsEnd(t *testing.T) {
	shell, db := newShell(t)
	total := domain.DefaultLibraryPageSize + 5
	for i := 0; i < total; i++ {
		seed(t, db, fmt.Sprintf("%d", i), "ada", fmt.Sprintf("post %d", i), clockBase)
	}
	shell.Reload(context.Background())
	shell.ToggleViewMode()

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	if got := len(shell.state.Data.Bookmarks); got != domain.DefaultLibraryPageSize {
		t.Fatalf("%d bookmarks before scrolling the list, want one page", got)
	}

	shell.listScroll.Y = 1_000_000
	tester.Frame()

	if got := len(shell.state.Data.Bookmarks); got != total {
		t.Errorf("%d bookmarks after reaching the list's end, want %d", got, total)
	}
}

// TestThePageNameLivesInTheTitleBar is magpie's crumb: the page has no heading of its own,
// so its name sits in the chrome and does not scroll away with the feed.
//
// The rectangle is the assertion. Finding the words somewhere is satisfied by a heading in
// the list's own scroll; the name has to be in the title bar's band, at the same y before and
// after the reader scrolls.
func TestThePageNameLivesInTheTitleBar(t *testing.T) {
	shell, db := newShell(t)
	for i := 0; i < 20; i++ {
		seed(t, db, fmt.Sprintf("%d", i), "ada", fmt.Sprintf("post %d", i), clockBase)
	}
	shell.Reload(context.Background())

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	name := shell.t().T(i18n.SidebarAllBookmarks)
	atTop, found := tester.Find(name)
	if !found {
		t.Fatalf("the page's name is missing; texts: %q", tester.Texts())
	}
	if atTop.Y >= float32(TitleBarHeight) {
		t.Errorf("the page's name is drawn at y=%v, below the %d-point title bar",
			atTop.Y, TitleBarHeight)
	}

	check := func(what string) {
		t.Helper()
		crumb, found := tester.Find(name)
		if !found {
			t.Fatalf("the page's name left the window when the %s scrolled; texts: %q", what, tester.Texts())
		}
		if crumb.Y != atTop.Y {
			t.Errorf("the page's name moved from y=%v to y=%v when the %s scrolled",
				atTop.Y, crumb.Y, what)
		}
		if !tester.HasText(shell.t().T(i18n.ToolbarFilters)) {
			t.Errorf("the toolbar went with the %s; the search and filters must stay", what)
		}
	}

	shell.resultsScroll.Y = 300
	tester.Frame()
	check("grid")

	// The list keeps its own scroll state; the crumb is outside both.
	shell.ToggleViewMode()
	shell.listScroll.Y = 300
	tester.Frame()
	check("list")
}

// TestTheStandingSyncLineStaysUnderTheToolbar pins the line that replaced the folded heading:
// the last sync and a report of what just happened are always drawn, because there is no
// heading left for them to fold into.
func TestTheStandingSyncLineStaysUnderTheToolbar(t *testing.T) {
	shell, db := newShell(t)
	for i := 0; i < 20; i++ {
		seed(t, db, fmt.Sprintf("%d", i), "ada", fmt.Sprintf("post %d", i), clockBase)
	}
	shell.state.LastSyncedAt = clockBase
	shell.state.LastSyncStatus = domain.SyncSuccess
	shell.Reload(context.Background())

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	// The toolbar's line names the last sync; the sidebar's foot prints the time alone beside
	// its status dot (`.side-gw`, app.css:513-517). Both are standing, so both survive the
	// feed scrolling.
	label := shell.t().T(i18n.LastSyncLabel)
	for _, y := range []float32{0, 200} {
		shell.resultsScroll.Y = y
		tester.Frame()
		if got := countTexts(tester.Texts(), label); got != 1 {
			t.Errorf("at scroll %v the toolbar's last-sync label appears %d times, want 1", y, got)
		}
		if !tester.HasText(shell.lastSyncLine()) {
			t.Errorf("at scroll %v the sidebar foot lost its last-sync line; texts: %q",
				y, tester.Texts())
		}
	}

	report := "imported 12 bookmarks"
	shell.state.LastBundleReport = report
	tester.Frame()
	if !tester.HasText(report) {
		t.Errorf("the report is not drawn; texts: %q", tester.Texts())
	}
}

// countTexts counts the labels containing s, which is how a test tells "the sidebar still says
// it" from "the toolbar still says it".
//
// Folded, because the sidebar draws its own line in small caps ("LAST SYNC  Oct 12, 8:00 PM"):
// a case-sensitive count would miss it and read the toolbar's copy alone as the sidebar's.
func countTexts(texts []string, s string) int {
	n := 0
	for _, text := range texts {
		if strings.Contains(strings.ToLower(text), strings.ToLower(s)) {
			n++
		}
	}
	return n
}

// TestANewFilterStartsAtTheTop keeps the previous list's place from following the reader
// into a different list: the window, the card limit and the anchor all restart.
func TestANewFilterStartsAtTheTop(t *testing.T) {
	shell, db := newShell(t)
	for i := 0; i < 20; i++ {
		seed(t, db, fmt.Sprintf("%d", i), "ada", fmt.Sprintf("post %d", i), clockBase)
	}
	shell.Reload(context.Background())

	shell.state.RenderLimit = 5
	shell.state.autoLoadY = 900
	shell.resultsScroll.Y = 900

	shell.state.Query = "post 1"
	shell.Reload(context.Background())

	if got := shell.state.RenderLimit; got != ResultsRenderLimit {
		t.Errorf("RenderLimit = %d after a new query, want %d", got, ResultsRenderLimit)
	}
	if shell.state.autoLoadY != -1 {
		t.Errorf("the auto-load anchor survived a new query: %v", shell.state.autoLoadY)
	}
	if shell.resultsScroll.Y != 0 {
		t.Errorf("the scroll offset survived a new query: %v", shell.resultsScroll.Y)
	}
}
