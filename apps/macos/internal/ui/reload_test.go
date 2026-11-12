package ui

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/x-master/apps/macos/internal/domain"
	"github.com/taoquo/x-master/apps/macos/internal/i18n"
	"github.com/taoquo/x-master/apps/macos/internal/storage/sqlite"
)

// The refresh engine (reload.go) is what keeps the window responsive: every filter
// click, tag click, scope switch and post-write refresh reads the workspace off the
// window's thread. These tests use the same captured-background seam as the file
// commands (work_test.go): they prove "the handler returned before the read happened"
// without timing anything.

// TestARefreshDoesNotStopTheWindow covers the reported class of freeze: a click that
// recomputes the page used to run the query in the click handler.
func TestARefreshDoesNotStopTheWindow(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "the first post", clockBase)
	shell.Reload(ctx)

	// The row that appears while the refresh is in flight.
	seed(t, db, "2", "bob", "a post saved a moment later", clockBase.Add(1))

	work := capturedBackground(shell)
	shell.ReloadAsync(ctx)

	if *work == nil {
		t.Fatal("the refresh ran in the click handler: the window cannot draw until it finishes")
	}
	if !shell.state.Refreshing {
		t.Error("a refresh is running and the window does not say so")
	}
	if got := len(shell.state.Data.Bookmarks); got != 1 {
		t.Errorf("%d bookmarks on screen during the refresh, want the previous page's 1", got)
	}

	(*work)()

	if shell.state.Refreshing {
		t.Error("the refresh finished and the note is still up")
	}
	if got := len(shell.state.Data.Bookmarks); got != 2 {
		t.Errorf("%d bookmarks after the refresh, want 2", got)
	}
}

// TestTypingCoalescesIntoOneRead is the search field: every keystroke asks for a
// refresh, and eight keystrokes must not queue eight queries behind each other.
func TestTypingCoalescesIntoOneRead(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post about compilers", clockBase)
	seed(t, db, "2", "bob", "a post about databases", clockBase)
	shell.Reload(ctx)
	first := shell.state.Data.Generation

	work := capturedBackground(shell)
	for _, query := range []string{"a", "ad", "ada", "b", "bo", "bob"} {
		shell.state.Query = query
		shell.ReloadAsync(ctx)
	}
	if *work == nil {
		t.Fatal("typing never started a refresh")
	}

	(*work)()

	// One Load bumps the generation once; a queue would bump it six times.
	if got := shell.state.Data.Generation - first; got != 1 {
		t.Errorf("the six keystrokes ran %d reads, want 1 (the last query)", got)
	}
	if got := shell.state.Data.Query.Filter.Query; got != "bob" {
		t.Errorf("the installed page is for %q, want the last query", got)
	}
	if got := len(shell.state.Data.Bookmarks); got != 1 {
		t.Errorf("%d bookmarks for the last query, want 1", got)
	}
}

// TestAStalePageIsNotInstalled covers the ordering rule: a read that finished after a
// newer one must not overwrite it, whatever thread it comes back on.
func TestAStalePageIsNotInstalled(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(ctx)
	applied := shell.state.Data.Generation

	older := shell.gatherReload(ctx, shell.reloadQuery())
	newer := shell.gatherReload(ctx, shell.reloadQuery())

	shell.applyReload(older)
	if got := shell.state.Data.Generation; got != applied {
		t.Errorf("a superseded page was installed: generation %d, want the applied %d", got, applied)
	}

	shell.applyReload(newer)
	if got := shell.state.Data.Generation; got != newer.view.Generation {
		t.Errorf("the newest page was not installed: generation %d, want %d",
			got, newer.view.Generation)
	}
}

// TestTheCountLineSaysItIsUpdating is the visible half of the contract: the page stays
// on screen, so the header has to say that it is about to change.
func TestTheCountLineSaysItIsUpdating(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(context.Background())

	shell.state.Refreshing = true
	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	if !tester.HasText(shell.t().T(i18n.ToolbarRefreshing)) {
		t.Errorf("the header does not say it is updating; texts: %q", tester.Texts())
	}

	shell.state.Refreshing = false
	tester.Frame()
	if tester.HasText(shell.t().T(i18n.ToolbarRefreshing)) {
		t.Error("the note stays after the refresh finished")
	}
}

// TestLoadMoreReadsOffTheWindowThread covers the other click that reads the library:
// the page past the first one.
func TestLoadMoreReadsOffTheWindowThread(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	want := domain.DefaultLibraryPageSize * 3 / 2
	for i := 0; i < want; i++ {
		seed(t, db, fmt.Sprintf("%d", i), "ada", fmt.Sprintf("post %d", i),
			clockBase.Add(-time.Minute*time.Duration(i)))
	}
	shell.Reload(ctx)
	if !shell.state.Data.HasMore {
		t.Fatalf("the fixture does not have a second page: %d rows", len(shell.state.Data.Bookmarks))
	}

	work := capturedBackground(shell)
	shell.LoadMoreResults(ctx)

	if *work == nil {
		t.Fatal("loading the next page ran in the click handler")
	}
	if !shell.state.LoadingMore {
		t.Error("the page is being read and the button does not say so")
	}

	// A second click while the page is in flight must not start a second read.
	shell.LoadMoreResults(ctx)

	(*work)()

	if shell.state.LoadingMore {
		t.Error("the page arrived and the loading flag stayed set")
	}
	if got := len(shell.state.Data.Bookmarks); got != want {
		t.Errorf("%d bookmarks after loading more, want the whole fixture's %d", got, want)
	}
}

// TestTheGraphBuildsOffTheWindowThread covers the one read that walks the whole
// library: switching to the graph used to block the window until it finished.
func TestTheGraphBuildsOffTheWindowThread(t *testing.T) {
	shell, _ := newShell(t)
	ctx := context.Background()
	shell.Reload(ctx)

	work := capturedBackground(shell)
	shell.ToggleGraph(ctx)

	if *work == nil {
		t.Fatal("the graph was built in the click handler")
	}
	if !shell.state.GraphLoading {
		t.Error("the graph is being built and the window does not say so")
	}

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	if !tester.HasText(shell.t().T(i18n.GraphBuildingTitle)) {
		t.Errorf("the empty graph claims there are no relations while it builds; texts: %q",
			tester.Texts())
	}

	(*work)()

	if shell.state.GraphLoading {
		t.Error("the build finished and the loading flag stayed set")
	}
	tester.Frame()
	if tester.HasText(shell.t().T(i18n.GraphBuildingTitle)) {
		t.Error("the building note stayed after the graph arrived")
	}
}

// TestGraphToggleDuringABuildIsNotDropped keeps the author layer honest: the switch is
// a toggle, and one pressed while the build runs is the one that counts.
func TestGraphToggleDuringABuildIsNotDropped(t *testing.T) {
	shell, _ := newShell(t)
	ctx := context.Background()
	shell.Reload(ctx)

	works := capturedBackgroundQueue(shell)
	shell.ToggleGraph(ctx)
	shell.ToggleGraphAuthors(ctx) // arrives while the first build is in flight

	if !shell.state.GraphDirty {
		t.Fatal("the toggle during the build was dropped")
	}

	(*works)[0]() // finishes the first build, which queues the second

	if len(*works) != 2 {
		t.Fatalf("the queued rebuild never started: %d captured jobs", len(*works))
	}
	(*works)[1]()

	if shell.state.GraphLoading {
		t.Error("the rebuild after the toggle is still marked running")
	}
	if shell.state.GraphDirty {
		t.Error("the queued rebuild was not consumed")
	}
	if !shell.state.GraphIncludeAuthors {
		t.Error("the rebuild lost the author layer the toggle turned on")
	}
}

// TestTheGraphFocusPanelOpensTheTag is the panel's one action: leaving the graph for the
// bookmarks behind the focused node. The focus is already a tag condition, so the click
// must not remove it (the old toggle did).
func TestTheGraphFocusPanelOpensTheTag(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seedTagged(t, db, "a", "rust")
	seedTagged(t, db, "b", "rust")
	shell.Reload(ctx)
	shell.RebuildGraph(ctx)
	shell.ToggleGraph(ctx)
	shell.state.ActiveTagIDs = []string{"rust"}

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	if err := tester.Click(shell.t().T(i18n.GraphOpenBookmarks)); err != nil {
		t.Fatalf("clicking the way back: %v", err)
	}
	tester.Frame()

	if shell.state.GraphMode {
		t.Error("the graph stayed open")
	}
	if got := shell.state.ActiveTagIDs; len(got) != 1 || got[0] != "rust" {
		t.Errorf("the tag condition is %q, want it kept", got)
	}
	if got := len(shell.state.Data.Bookmarks); got != 2 {
		t.Errorf("%d bookmarks after the click, want the tag's 2", got)
	}
}

// TestABusyWriteDisablesTheBulkActions keeps a second bulk action from starting while
// the first is still running: the buttons are disabled, so the click does nothing.
func TestABusyWriteDisablesTheBulkActions(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	tags := sqlite.NewTagRepository(db)
	createTags(t, tags, map[string]string{"t1": "rust"})
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(ctx)
	shell.ReloadTagLibrary(ctx)

	shell.state.SelectedIDs = map[string]bool{"1": true}
	shell.state.BulkTagLabel = "rust"
	shell.state.Mutating = true

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	if err := tester.Click(shell.t().T(i18n.BatchApplyTag)); err != nil {
		t.Fatalf("clicking the disabled action: %v", err)
	}
	tester.Frame()

	// The tag was not applied: the click landed on a disabled control.
	links, err := tags.AllRelations(ctx)
	if err != nil {
		t.Fatalf("AllRelations: %v", err)
	}
	if len(links) != 0 {
		t.Errorf("%d tag links after clicking a disabled action, want 0", len(links))
	}
}

// TestTheTagPageReadsOffTheWindowThread covers the second page that reads the whole
// library: the tag table counts each tag's bookmarks from every relation.
func TestTheTagPageReadsOffTheWindowThread(t *testing.T) {
	shell, db := newShell(t)
	seedTagOnly(t, db, "a", "rust")

	work := capturedBackground(shell)
	shell.ReloadTagLibraryAsync(context.Background())

	if *work == nil {
		t.Fatal("the tag page was read in the click handler")
	}
	if !shell.state.TagLibraryLoading {
		t.Error("the page is being read and the window does not say so")
	}
	if got := len(shell.state.TagItems); got != 0 {
		t.Errorf("%d tags before the read finished, want 0", got)
	}

	(*work)()

	if shell.state.TagLibraryLoading {
		t.Error("the read finished and the loading flag stayed set")
	}
	if got := len(shell.state.TagItems); got != 1 {
		t.Errorf("%d tags after the read, want 1", got)
	}
}

// TestATagEditDuringTheReadRunsAgain keeps the page from ending on a stale count: an
// edit that lands while the read runs queues one more read.
func TestATagEditDuringTheReadRunsAgain(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seedTagOnly(t, db, "a", "rust")

	works := capturedBackgroundQueue(shell)
	shell.ReloadTagLibraryAsync(ctx)

	seedTagOnly(t, db, "b", "go")
	shell.ReloadTagLibraryAsync(ctx) // arrives while the first read is running

	if !shell.state.TagLibraryDirty {
		t.Fatal("the edit during the read was dropped")
	}

	(*works)[0]()
	if len(*works) != 2 {
		t.Fatalf("the queued read never started: %d jobs", len(*works))
	}
	(*works)[1]()

	if shell.state.TagLibraryLoading || shell.state.TagLibraryDirty {
		t.Error("the page is still marked as reading after the queued read")
	}
	if got := len(shell.state.TagItems); got != 2 {
		t.Errorf("%d tags after the second read, want 2", got)
	}
}

// TestTheTagPageSaysItIsReading is the visible half: the first frame of the page has no
// counts yet, and the empty state would claim the library has no tags at all.
func TestTheTagPageSaysItIsReading(t *testing.T) {
	shell, _ := newShell(t)
	shell.state.TagMode = true
	shell.state.TagView = TagViewTags
	shell.state.TagLibraryLoading = true

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	if !tester.HasText(shell.t().T(i18n.TagLibraryReading)) {
		t.Errorf("the page says nothing about reading; texts: %q", tester.Texts())
	}
	if tester.HasText(shell.t().T(i18n.TagNoTagsCreated)) {
		t.Error("the page claims the library is empty while it is still reading")
	}
}

// capturedBackgroundQueue collects every background job in order, for work that starts
// more work when it finishes (the graph rebuild is one).
func capturedBackgroundQueue(shell *Shell) *[]func() {
	works := &[]func(){}
	shell.background = func(fn func()) { *works = append(*works, fn) }
	return works
}
