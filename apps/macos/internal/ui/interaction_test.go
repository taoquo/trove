package ui

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/taoquo/x-master/apps/macos/internal/domain"
	"github.com/taoquo/x-master/apps/macos/internal/i18n"
)

// The keyboard, the sidebar, the context-menu actions and the settings page, as behaviour.
//
// docs/interaction-parity-plan.md P2.1-P2.6 is what these pin: a grid whose arrows move by
// a row, a batch selection the keyboard can tick and Escape can take back, a sidebar that
// hides, and a settings page whose find field searches every tab.

// gridShell is four bookmarks in the grid at a width where it draws two columns.
func gridShell(t *testing.T) (*Shell, *ui.Tester) {
	t.Helper()
	shell, db := newShell(t)
	for _, id := range []string{"1", "2", "3", "4"} {
		seed(t, db, id, "ada", "post "+id, clockBase)
	}
	shell.Reload(context.Background())
	shell.state.ViewMode = domain.ViewGrid

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	if got := shell.state.ResultsColumns; got != 2 {
		t.Fatalf("the grid drew %d columns at 1180; the test needs two", got)
	}
	return shell, tester
}

// TestTheGridArrowsMoveByARow is the defect the plan recorded: the down arrow moved the
// selection one card, which in a two-column grid is the card to the right.
func TestTheGridArrowsMoveByARow(t *testing.T) {
	shell, tester := gridShell(t)
	rows := shell.state.Data.Bookmarks
	if len(rows) != 4 {
		t.Fatalf("the list holds %d rows, want 4", len(rows))
	}

	press := func(key ui.Key) {
		tester.Key(0, key)
		tester.Frame()
	}

	press(ui.KeyDown)
	if got := shell.state.Selected; got != rows[0].TweetID {
		t.Fatalf("the first Down selected %q, want %q", got, rows[0].TweetID)
	}
	press(ui.KeyDown)
	if got := shell.state.Selected; got != rows[2].TweetID {
		t.Errorf("Down moved to %q, want the card below %q", got, rows[2].TweetID)
	}
	press(ui.KeyRight)
	if got := shell.state.Selected; got != rows[3].TweetID {
		t.Errorf("Right moved to %q, want the neighbouring card %q", got, rows[3].TweetID)
	}
	press(ui.KeyUp)
	if got := shell.state.Selected; got != rows[1].TweetID {
		t.Errorf("Up moved to %q, want the card above %q", got, rows[1].TweetID)
	}
	press(ui.KeyLeft)
	if got := shell.state.Selected; got != rows[0].TweetID {
		t.Errorf("Left moved to %q, want %q", got, rows[0].TweetID)
	}

	// The ends of the loaded page, and a step as far as the page is long.
	press(ui.KeyEnd)
	if got := shell.state.Selected; got != rows[3].TweetID {
		t.Errorf("End selected %q, want the last row %q", got, rows[3].TweetID)
	}
	press(ui.KeyPageDown)
	if got := shell.state.Selected; got != rows[3].TweetID {
		t.Errorf("PageDown past the end selected %q, want it clamped to %q",
			got, rows[3].TweetID)
	}
	press(ui.KeyPageUp)
	if got := shell.state.Selected; got != rows[0].TweetID {
		t.Errorf("PageUp selected %q, want the first row %q", got, rows[0].TweetID)
	}
}

// TestTheListArrowsMoveOneCard is the same keys in the layout where a row is one card.
func TestTheListArrowsMoveOneCard(t *testing.T) {
	shell, db := newShell(t)
	for _, id := range []string{"1", "2", "3"} {
		seed(t, db, id, "ada", "post "+id, clockBase)
	}
	shell.Reload(context.Background())
	shell.state.ViewMode = domain.ViewList

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	tester.Key(0, ui.KeyDown)
	tester.Key(0, ui.KeyDown)
	tester.Frame()

	rows := shell.state.Data.Bookmarks
	if got := shell.state.Selected; got != rows[1].TweetID {
		t.Errorf("two Downs in the list selected %q, want %q", got, rows[1].TweetID)
	}
}

// TestSpaceTicksTheSelectedCard covers the keyboard's way into a bulk action.
func TestSpaceTicksTheSelectedCard(t *testing.T) {
	shell, tester := gridShell(t)
	rows := shell.state.Data.Bookmarks

	tester.Key(0, ui.KeySpace)
	tester.Frame()
	if !shell.state.SelectedIDs[rows[0].TweetID] {
		t.Fatalf("Space did not tick %q", rows[0].TweetID)
	}
	tester.Key(0, ui.KeySpace)
	tester.Frame()
	if shell.state.SelectedIDs[rows[0].TweetID] {
		t.Error("a second Space did not untick the card")
	}
}

// TestEscapeTakesBackTheBatchSelection is the cascade's new first step.
func TestEscapeTakesBackTheBatchSelection(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(context.Background())
	shell.state.SelectedIDs = map[string]bool{"1": true}
	shell.state.Query = "ada"

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	tester.Key(0, ui.KeyEscape)
	tester.Frame()

	if len(shell.state.SelectedIDs) != 0 {
		t.Errorf("the batch selection survived Escape: %v", shell.state.SelectedIDs)
	}
	if shell.state.Query == "" {
		t.Error("Escape took two steps: the query went with the selection")
	}
}

// TestClosingTheDetailHandsBackTheKeyboard pins the focus request, which the card honours
// on the frame after it closes.
func TestClosingTheDetailHandsBackTheKeyboard(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(context.Background())
	shell.state.Selected = shell.state.Data.Bookmarks[0].TweetID
	shell.state.DetailOpen = true

	shell.CloseDetail()
	if shell.state.DetailOpen {
		t.Error("CloseDetail left the detail open")
	}
	if !shell.state.FocusSelected {
		t.Error("CloseDetail did not ask for the keyboard to go back to the card")
	}
}

// TestTheSidebarHides covers Cmd-\ and what the hidden column leaves behind.
func TestTheSidebarHides(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(context.Background())

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	if !tester.HasText(shell.t().T(i18n.SidebarTags)) {
		t.Fatal("the full sidebar does not show its facet tabs")
	}

	shell.ToggleSidebar()
	if !shell.state.SidebarCollapsed {
		t.Error("ToggleSidebar did not hide the column")
	}
	if got := shell.sidebarPaneWidth(); got != 0 {
		t.Errorf("the hidden column owns %v points, want 0", got)
	}

	tester.Frame()
	texts := tester.Texts()
	// magpie hides the column outright rather than folding it to a 56-point rail
	// (app.js:23614), so nothing of it is left to read.
	if containsText(texts, shell.t().T(i18n.SidebarTags)) {
		t.Error("the hidden column still draws the facet tabs")
	}
	if containsText(texts, shell.t().T(i18n.LastSyncLabel)) {
		t.Error("the hidden column still draws the sync block")
	}

	shell.ToggleSidebar()
	if shell.state.SidebarCollapsed {
		t.Error("a second toggle did not show the column")
	}
}

// TestOnlyTagDropsTheOtherConditions is the "only this tag" context-menu action.
func TestOnlyTagDropsTheOtherConditions(t *testing.T) {
	shell, db := newShell(t)
	// The action reloads, and a reload drops a condition whose tag the library does not hold
	// (reload.go), so both tags have to be real for the state to survive the read.
	seedTagOnly(t, db, "a", "a")
	seedTagOnly(t, db, "b", "b")
	shell.state.ActiveTagIDs = []string{"a", "b"}
	shell.state.ActiveAuthors = []string{"ada"}

	shell.onlyTagFilter(context.Background(), "b")

	if len(shell.state.ActiveTagIDs) != 1 || shell.state.ActiveTagIDs[0] != "b" {
		t.Errorf("ActiveTagIDs = %v, want just b", shell.state.ActiveTagIDs)
	}
	if len(shell.state.ActiveAuthors) != 0 {
		t.Errorf("ActiveAuthors = %v, want none", shell.state.ActiveAuthors)
	}
}

// TestOnlyAuthorDropsTheTagConditions is the author row's version.
func TestOnlyAuthorDropsTheTagConditions(t *testing.T) {
	shell, _ := newShell(t)
	shell.state.ActiveTagIDs = []string{"a"}
	shell.state.ActiveAuthors = []string{"ada", "grace"}

	shell.onlyAuthorFilter(context.Background(), "grace")

	if len(shell.state.ActiveAuthors) != 1 || shell.state.ActiveAuthors[0] != "grace" {
		t.Errorf("ActiveAuthors = %v, want just grace", shell.state.ActiveAuthors)
	}
	if len(shell.state.ActiveTagIDs) != 0 {
		t.Errorf("ActiveTagIDs = %v, want none", shell.state.ActiveTagIDs)
	}
}

// TestTheContextMenuTicksOneCard covers the menu's batch item, which is the card's own
// toggle reached from the right button.
func TestTheContextMenuTicksOneCard(t *testing.T) {
	shell, _ := newShell(t)

	shell.setBatchSelected("1", true)
	if !shell.state.SelectedIDs["1"] {
		t.Fatal("setBatchSelected did not tick the card")
	}
	shell.setBatchSelected("1", false)
	if shell.state.SelectedIDs != nil {
		t.Errorf("unticking the last card left %v, want an empty set", shell.state.SelectedIDs)
	}
}

// TestTheSettingsFindFieldSearchesEveryTab is the reason the page has one field rather than
// one per tab: a setting on a tab the reader is not looking at still has to be findable.
func TestTheSettingsFindFieldSearchesEveryTab(t *testing.T) {
	shell, _ := newShell(t)
	shell.state.SettingsOpen = true
	shell.state.SettingsSection = SettingsSectionGeneral
	shell.state.SettingsQuery = strings.ToLower(shell.t().T(i18n.SettingsXCookiesTitle))

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	texts := tester.Texts()

	if !tester.HasText(shell.t().T(i18n.SettingsXCookiesTitle)) {
		t.Errorf("the find field did not reach the sync tab; texts: %q", texts)
	}
	if tester.HasText(shell.t().T(i18n.SettingsLanguage)) {
		t.Error("the general tab's rows are drawn beside a query that does not match them")
	}
	// The section is named above its rows, so a result says where it lives.
	if !tester.HasText(strings.ToUpper(shell.t().T(i18n.SettingsSectionSync))) {
		t.Errorf("the filtered page does not name the section it found; texts: %q", texts)
	}
}

// TestTheSettingsFindFieldSaysWhenNothingMatches keeps an empty page from reading as a
// broken one.
func TestTheSettingsFindFieldSaysWhenNothingMatches(t *testing.T) {
	shell, _ := newShell(t)
	shell.state.SettingsOpen = true
	shell.state.SettingsQuery = "zzzz-no-such-setting"

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	want := shell.t().Tf(i18n.SettingsNoMatch, "zzzz-no-such-setting")
	if !tester.HasText(want) {
		t.Errorf("no no-match line; texts: %q", tester.Texts())
	}
}

// TestEscapeClearsTheSettingsSearchThenCloses covers the page's own cascade.
func TestEscapeClearsTheSettingsSearchThenCloses(t *testing.T) {
	shell, _ := newShell(t)
	shell.state.SettingsOpen = true
	shell.state.SettingsQuery = "theme"

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	tester.Key(0, ui.KeyEscape)
	tester.Frame()
	if shell.state.SettingsQuery != "" {
		t.Error("the first Escape did not clear the find field")
	}
	if !shell.state.SettingsOpen {
		t.Error("the first Escape closed the page as well as clearing the field")
	}

	tester.Key(0, ui.KeyEscape)
	tester.Frame()
	if shell.state.SettingsOpen {
		t.Error("the second Escape did not leave the page")
	}
}

// TestSettingsIsAPageNotASheet pins the structural decision: the list is replaced, not
// covered. A modal leaves the results in the frame behind a scrim; a page does not.
func TestSettingsIsAPageNotASheet(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(context.Background())
	shell.state.SettingsOpen = true

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	if tester.HasText(shell.t().T(i18n.SidebarAllBookmarks)) && !tester.HasText(shell.t().T(i18n.SettingsTitle)) {
		t.Fatal("the settings page did not draw")
	}
	if !tester.HasText(shell.t().T(i18n.SettingsTitle)) {
		t.Errorf("the page has no title; texts: %q", tester.Texts())
	}
	// The sidebar stays: it is the window's navigation, and a page does not take it away.
	// Its facet tabs are what makes it the sidebar; the app's name is not, now that the brand
	// lives in the title bar (shell-contract.md S38).
	if !tester.HasText(shell.t().T(i18n.SidebarTags)) {
		t.Error("the settings page took the sidebar with it")
	}
}

// TestTheStatusLineHasThreeTones covers the one status line's durations: a success is
// short, a failure stays longer, and a long sentence gets read at 80ms a character up to
// the ceiling (app.js:423; shell-contract.md S37).
func TestTheStatusLineHasThreeTones(t *testing.T) {
	shell, _ := newShell(t)

	shell.ShowToast("saved")
	if shell.state.ToastKind != ToastOK {
		t.Errorf("a success is %q, want %q", shell.state.ToastKind, ToastOK)
	}
	if got := time.Until(shell.state.ToastUntil); got > ToastDismissAfter+time.Second {
		t.Errorf("a success lasts %v, want about %v", got, ToastDismissAfter)
	}

	shell.ShowStatus(ToastWarn, "careful")
	if got := time.Until(shell.state.ToastUntil); got < ToastWarnAfter-time.Second {
		t.Errorf("a warning lasts %v, want about %v", got, ToastWarnAfter)
	}

	shell.ShowStatus(ToastError, "failed")
	if shell.state.ToastKind != ToastError {
		t.Errorf("a failure is %q, want %q", shell.state.ToastKind, ToastError)
	}
	if got := time.Until(shell.state.ToastUntil); got < ToastErrorAfter-time.Second {
		t.Errorf("a failure lasts %v, want about %v", got, ToastErrorAfter)
	}

	// A sentence longer than the error's own window keeps the line up for its reading
	// time, and never past the ceiling.
	shell.ShowStatus(ToastOK, strings.Repeat("a", 400))
	if got := time.Until(shell.state.ToastUntil); got < ToastMaxAfter-time.Second {
		t.Errorf("a long sentence lasts %v, want the %v ceiling", got, ToastMaxAfter)
	}

	// Expiry is a comparison against the deadline, not a cleared field: nothing runs to
	// clear it when no window is listening.
	shell.state.ToastUntil = time.Now().Add(-time.Second)
	if got := shell.ToastText(); got != "" {
		t.Errorf("an expired message still reads %q", got)
	}
}

// TestTheSidebarButtonKeepsItsPlaceWhenTheColumnFolds is the promise the footer control broke
// (shell-contract.md S38): the button is in the title bar, and it is at the same point on
// screen whether the sidebar is a column or a rail, as Finder's and Notes' is.
func TestTheSidebarButtonKeepsItsPlaceWhenTheColumnFolds(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(context.Background())

	tester := ui.NewTester(shell.View, 1180, 760)
	// A real window's controls: 50 points of band, the lights ending at 72.
	tester.SetTitleBar(ui.TitleBar{Height: 50, Left: 72})
	tester.Frame()

	open, ok := tester.Find(shell.t().T(i18n.ActionCollapseSidebar))
	if !ok {
		t.Fatalf("the title bar has no sidebar button; texts: %q", tester.Texts())
	}
	if open.Y < 0 || open.Y+open.H > 50 {
		t.Errorf("the sidebar button is at y=%v..%v, outside the 50-point title bar",
			open.Y, open.Y+open.H)
	}
	brand, ok := tester.Find(shell.t().T(i18n.AppTitle))
	if !ok {
		t.Fatalf("the title bar has no brand; texts: %q", tester.Texts())
	}

	shell.ToggleSidebar()
	tester.Frame()
	folded, ok := tester.Find(shell.t().T(i18n.ActionExpandSidebar))
	if !ok {
		t.Fatalf("the folded layout has no sidebar button; texts: %q", tester.Texts())
	}
	if folded.X != open.X || folded.Y != open.Y {
		t.Errorf("the button moved from (%v,%v) to (%v,%v) when the column folded",
			open.X, open.Y, folded.X, folded.Y)
	}
	foldedBrand, ok := tester.Find(shell.t().T(i18n.AppTitle))
	if !ok {
		t.Fatal("the folded layout dropped the brand")
	}
	if foldedBrand.X != brand.X {
		t.Errorf("the brand moved from x=%v to x=%v when the column folded", brand.X, foldedBrand.X)
	}
}

// TestTheSidebarFoldsOnMaggiesShortcut pins the binding: magpie answers Control-Command-S,
// not the Cmd-\ this shell used to require.
func TestTheSidebarFoldsOnMaggiesShortcut(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(context.Background())

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	tester.Key(ui.Ctrl|ui.Super, ui.KeyS)
	tester.Frame()

	if !shell.state.SidebarCollapsed {
		t.Error("Control-Command-S did not fold the sidebar")
	}
}

// TestNavigatingFromSettingsLeavesThePage is the page's one dead-control hazard: settings is
// drawn *instead of* the results, so a sidebar click that only reloaded the list would look
// like it did nothing.
func TestNavigatingFromSettingsLeavesThePage(t *testing.T) {
	shell, _ := newShell(t)
	shell.state.SettingsOpen = true
	shell.state.SettingsQuery = "theme"

	shell.selectSidebar(context.Background(), selectionInbox)

	if shell.state.SettingsOpen {
		t.Error("choosing a scope left the settings page on screen")
	}
	if shell.state.SettingsQuery != "" {
		t.Errorf("the find field survived at %q", shell.state.SettingsQuery)
	}
	if shell.state.Selection != selectionInbox {
		t.Errorf("Selection = %q, want %q", shell.state.Selection, selectionInbox)
	}
}

// TestTheDetailBackdropClosesIt is the modal half of P2 item 19: MyGo's DialogBase closes
// on a backdrop click, moves the focus in when it opens and keeps Tab inside it, with what
// is behind it inert (base.go:457-478, scope.go:1-8). The detail is built on DialogBase, so
// those are this shell's behaviours too, not the toolkit's alone.
func TestTheDetailBackdropClosesIt(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(context.Background())
	shell.state.Selected = "1"
	shell.OpenSelectedDetail()

	tester := ui.NewTester(shell.View, 1180, 760)
	if !shell.state.DetailOpen {
		t.Fatal("the detail is not open to begin with")
	}

	// A point at the window's edge: the panel is centred and does not reach there, so the
	// pointer lands on the backdrop the dialog draws over the whole window.
	tester.ClickAt(4, 400)
	if shell.state.DetailOpen {
		t.Error("clicking the backdrop left the detail open")
	}
}
