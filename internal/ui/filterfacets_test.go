package ui

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/i18n"
	"github.com/taoquo/trove/internal/storage/sqlite"
)

// Every kind of condition is a chip, named the way a reader names it, removable on its own and
// all at once.

// TestEveryConditionHasAChip covers the table itself: a tag, an author and a switch each
// produce exactly one chip, in the facet's own order.
func TestEveryConditionHasAChip(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(ctx)

	tag, err := shell.service.CreateTag(ctx, "Go")
	if err != nil {
		t.Fatalf("CreateTag: %v", err)
	}

	shell.state.ActiveTagIDs = []string{tag.ID}
	shell.state.ActiveAuthors = []string{"ada"}
	shell.state.OnlyWithMedia = true
	shell.Reload(ctx)

	conditions := shell.activeConditions()
	if len(conditions) != 3 {
		t.Fatalf("%d conditions, want 3: %+v", len(conditions), conditions)
	}
	if conditions[0].Facet != "tag" || conditions[0].Label != tag.Name {
		t.Errorf("first condition = %+v, want the tag named %q", conditions[0], tag.Name)
	}
	if conditions[1].Facet != "author" || conditions[1].Label != "@ada" {
		t.Errorf("second condition = %+v, want the author, marked as one", conditions[1])
	}
	if conditions[2].Facet != "" || conditions[2].Label != shell.t().T(i18n.FilterHasMedia) {
		t.Errorf("third condition = %+v, want the media switch", conditions[2])
	}
}

// TestChipsShowNamesNotIDs is the defect the old chip list had: it printed the raw
// selection, so a tag chip read "tag:3f2a…".
func TestChipsShowNamesNotIDs(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(ctx)

	tag, err := shell.service.CreateTag(ctx, "Databases")
	if err != nil {
		t.Fatalf("CreateTag: %v", err)
	}
	shell.state.ActiveTagIDs = []string{tag.ID}
	shell.state.ActiveAuthors = []string{"ada"}
	shell.Reload(ctx)

	for _, condition := range shell.activeConditions() {
		if strings.HasPrefix(condition.Label, "tag:") || strings.HasPrefix(condition.Label, "list:") {
			t.Errorf("a chip shows an internal id: %q", condition.Label)
		}
		if condition.Label == tag.ID {
			t.Errorf("the tag chip shows the tag's uuid")
		}
		if condition.Label == "" {
			t.Errorf("a chip has no label: %+v", condition)
		}
	}
}

// TestRemovingAChipClearsOnlyThatCondition keeps the removal honest: a chip removes its
// own value and nothing else.
func TestRemovingAChipClearsOnlyThatCondition(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post", clockBase)
	seed(t, db, "2", "grace", "another post", clockBase)
	shell.Reload(ctx)

	// A tag the library holds: a condition naming nothing is dropped as soon as the page is
	// read (reload.go), which is the point of TestDeletingTheFilteredTagDropsTheCondition and
	// would leave this test two conditions instead of three.
	tag, err := shell.service.CreateTag(ctx, "Go")
	if err != nil {
		t.Fatalf("CreateTag: %v", err)
	}
	shell.state.ActiveTagIDs = []string{tag.ID}
	shell.state.ActiveAuthors = []string{"ada"}
	shell.state.OnlyLongform = true
	shell.Reload(ctx)

	conditions := shell.activeConditions()
	if len(conditions) != 3 {
		t.Fatalf("%d conditions, want 3: %+v", len(conditions), conditions)
	}
	conditions[1].Remove()

	if len(shell.state.ActiveAuthors) != 0 {
		t.Errorf("removing the author chip left %v", shell.state.ActiveAuthors)
	}
	if len(shell.state.ActiveTagIDs) != 1 || shell.state.ActiveTagIDs[0] != tag.ID {
		t.Errorf("removing the author chip changed the tag: %v", shell.state.ActiveTagIDs)
	}
	if !shell.state.OnlyLongform {
		t.Error("removing an author chip cleared the longform switch")
	}
}

// TestClearAllRemovesEveryCondition is the control the chip row did not have.
func TestClearAllRemovesEveryCondition(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(ctx)

	tag, err := shell.service.CreateTag(ctx, "Go")
	if err != nil {
		t.Fatalf("CreateTag: %v", err)
	}
	shell.state.ActiveTagIDs = []string{tag.ID}
	shell.state.ActiveAuthors = []string{"ada"}
	shell.state.OnlyWithMedia = true
	shell.state.OnlyLongform = true
	shell.state.OnlyPendingSuggestions = true
	shell.state.Selection = selectionInbox
	shell.Reload(ctx)

	if shell.conditionCount() != 5 {
		t.Fatalf("%d conditions, want 5", shell.conditionCount())
	}

	shell.clearConditions(ctx)

	if got := shell.conditionCount(); got != 0 {
		t.Errorf("%d conditions after clearing, want 0: %+v", got, shell.activeConditions())
	}
	// Clearing filters is not leaving the container the user is reading.
	if shell.state.Selection != selectionInbox {
		t.Errorf("selection = %q, want the inbox to stay", shell.state.Selection)
	}
}

// TestTheBadgeCountsTagsAndAuthors closes the badge's blind spot: it counted only the
// switches, so a tag-filtered list showed no badge at all.
func TestTheBadgeCountsTagsAndAuthors(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(ctx)

	if got := shell.conditionCount(); got != 0 {
		t.Fatalf("a fresh shell reports %d conditions", got)
	}

	shell.state.ActiveAuthors = []string{"ada"}
	if got := shell.conditionCount(); got != 1 {
		t.Errorf("one author reports %d", got)
	}

	tag, err := shell.service.CreateTag(ctx, "Go")
	if err != nil {
		t.Fatalf("CreateTag: %v", err)
	}
	shell.state.ActiveTagIDs = []string{tag.ID}
	if got := shell.conditionCount(); got != 2 {
		t.Errorf("an author and a tag report %d", got)
	}
}

// TestTheAuthorChipSurvivesTheOtherTab is acceptance criterion 1: the condition is named in
// the result area, so putting the sidebar on the tags tab does not hide which author is
// narrowing the list.
//
// The list is empty here on purpose -- with a card on screen the author's name would be drawn
// there too, and the test could pass without any chip existing.
func TestTheAuthorChipSurvivesTheOtherTab(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(ctx)

	shell.state.ActiveAuthors = []string{"ada"}
	// The seeded post has no media, so the author's own rows are behind a flag the list
	// cannot satisfy. A tag id that names nothing would empty it too, but the reload drops
	// such a condition now (reload.go) and the author would match again.
	shell.state.OnlyWithMedia = true
	// The tags tab is the one on screen, and it draws no author rows at all.
	shell.state.SidebarFacetTab = sidebarTabTags
	shell.Reload(ctx)
	if got := len(shell.state.Data.Bookmarks); got != 0 {
		t.Fatalf("%d results, want an empty list", got)
	}

	tester := ui.NewTester(shell.View, 1180, 900)
	tester.Frame()

	chip, found := tester.Find("@ada")
	if !found {
		t.Fatalf("the author condition is named nowhere while the sidebar shows the tags tab")
	}
	empty, found := tester.Find(shell.t().T(i18n.EmptyFilter))
	if !found {
		t.Fatalf("no empty state to compare against")
	}
	if chip.Y >= empty.Y {
		t.Errorf("the author chip sits at y=%.0f, not above the empty state (y=%.0f)",
			chip.Y, empty.Y)
	}
}

// TestAFilterChipSharesTheSearchRow is the second half of the header report: compacting the
// header left the active filters on a row of their own, so selecting one still grew the
// toolbar by a full-width block under the search. The chip now stands in that row, so the
// filter control keeps its place and the chip's band is the control's band.
func TestAFilterChipSharesTheSearchRow(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(ctx)

	tester := ui.NewTester(shell.View, 1180, 900)
	tester.Frame()
	before, found := tester.Find(shell.t().T(i18n.ToolbarFilters))
	if !found {
		t.Fatalf("the filter control is not on screen")
	}

	// An empty result on purpose: the chip is then the only thing naming the author, and
	// the toolbar's copy is the first one drawn (as in TestTheAuthorChipSurvivesTheOtherTab).
	// The media flag empties it; an unknown tag id would be dropped by the reload instead.
	shell.state.ActiveAuthors = []string{"ada"}
	shell.state.OnlyWithMedia = true
	shell.Reload(ctx)
	tester.Frame()

	after, found := tester.Find(shell.t().T(i18n.ToolbarFilters))
	if !found {
		t.Fatalf("the filter control vanished when a filter was applied")
	}
	if after.Y != before.Y || after.H != before.H {
		t.Errorf("the filter control moved from y=%.0f h=%.0f to y=%.0f h=%.0f; one condition must not add a row",
			before.Y, before.H, after.Y, after.H)
	}
	chip, found := tester.Find("@ada")
	if !found {
		t.Fatalf("the active author has no chip")
	}
	if chip.Y >= after.Y+after.H || chip.Y+chip.H <= after.Y {
		t.Errorf("the chip (y=%.0f h=%.0f) does not stand on the filter control's row (y=%.0f h=%.0f)",
			chip.Y, chip.H, after.Y, after.H)
	}
	if chip.X+chip.W > after.X {
		t.Errorf("the chip ends at x=%.0f, past the filter control at x=%.0f; it belongs to the search's side of the row",
			chip.X+chip.W, after.X)
	}
}

// TestChoosingAnAuthorLeavesTheListInPlace is the rule the sidebar got wrong: the chosen
// author was moved to the top of the list, so the row a reader had just clicked was a row
// and a half up the column by the next frame.
//
// The whole list is drawn now, in the service's order, so the assertion is that the order is
// the same before and after the choice -- and that the chosen author is still one of the rows
// (a filter with no row to undo it from is the defect the pinning was written to avoid).
func TestChoosingAnAuthorLeavesTheListInPlace(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	for _, handle := range []string{"aaron", "beth", "cara", "drew", "erin", "finn", "gwen", "hugo"} {
		seed(t, db, "id-"+handle, handle, "a post by "+handle, clockBase)
	}
	shell.Reload(ctx)

	if len(shell.state.Data.Authors) != 8 {
		t.Fatalf("%d authors loaded, want 8", len(shell.state.Data.Authors))
	}
	before := authorHandles(shell.visibleAuthors())

	shell.state.ActiveAuthors = []string{"hugo"}
	shell.Reload(ctx)
	after := authorHandles(shell.visibleAuthors())

	if strings.Join(before, ",") != strings.Join(after, ",") {
		t.Errorf("choosing an author reordered the list:\n before %v\n after  %v", before, after)
	}
	found := false
	for _, handle := range after {
		if handle == "hugo" {
			found = true
		}
	}
	if !found {
		t.Errorf("the chosen author has no row to undo it from: %v", after)
	}
}

// authorHandles is the order a list of summaries is drawn in.
func authorHandles(authors []domain.AuthorSummary) []string {
	handles := make([]string, 0, len(authors))
	for _, author := range authors {
		handles = append(handles, author.AuthorHandle)
	}
	return handles
}

// TestChipsWrapInsteadOfOverflowing covers the layout: a tag set is unbounded, and an
// unwrapped row pushed every chip past the window's right edge.
func TestChipsWrapInsteadOfOverflowing(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(ctx)

	names := make([]string, 0, 12)
	for i := 1; i <= 12; i++ {
		name := fmt.Sprintf("Topic %02d", i)
		tag, err := shell.service.CreateTag(ctx, name)
		if err != nil {
			t.Fatalf("CreateTag(%q): %v", name, err)
		}
		shell.state.ActiveTagIDs = append(shell.state.ActiveTagIDs, tag.ID)
		names = append(names, tag.Name)
	}
	// The sidebar's own tag rows carry the same words; showing the authors tab leaves the
	// chips as the only place the text is drawn.
	shell.state.SidebarFacetTab = sidebarTabAuthors
	shell.Reload(ctx)

	tester := ui.NewTester(shell.View, 1180, 900)
	tester.Frame()

	ys := map[float32]bool{}
	for _, name := range names {
		rect, found := tester.Find(name)
		if !found {
			t.Fatalf("no chip for %q", name)
		}
		if rect.X < 0 || rect.X+rect.W > 1180 {
			t.Errorf("chip %q spans %.0f..%.0f, outside the window", name, rect.X, rect.X+rect.W)
		}
		ys[rect.Y] = true
	}
	if len(ys) < 2 {
		t.Errorf("all %d chips are on one line at y=%.0f; the row did not wrap", len(names), firstKey(ys))
	}
}

func firstKey(set map[float32]bool) float32 {
	for key := range set {
		return key
	}
	return 0
}

// A tag is a condition, never a scope.

// TestATagClickLeavesTheScopeWhereItIs is the old behaviour's core confusion: one click
// both narrowed the list and moved the user to "the tag", so the title and the filter
// disagreed with each other as soon as a second condition was involved.
func TestATagClickLeavesTheScopeWhereItIs(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seedTagged(t, db, "a", "go")
	shell.Reload(ctx)

	shell.state.Selection = selectionInbox
	shell.toggleTagFilter(ctx, "go")

	if shell.state.Selection != selectionInbox {
		t.Errorf("selection = %q, want the inbox unchanged", shell.state.Selection)
	}
	if len(shell.state.ActiveTagIDs) != 1 || shell.state.ActiveTagIDs[0] != "go" {
		t.Errorf("tag conditions = %v, want [go]", shell.state.ActiveTagIDs)
	}
}

// TestTheTitleNamesTheScopeNotTheTag keeps the header honest: the title says which
// container is being read, and the chip row says what is narrowing it.
func TestTheTitleNamesTheScopeNotTheTag(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seedTagged(t, db, "a", "go")
	shell.Reload(ctx)

	shell.state.ActiveTagIDs = []string{"go"}
	if got, want := shell.scopeTitle(), shell.t().T(i18n.SidebarAllBookmarks); got != want {
		t.Errorf("title = %q, want the scope's name %q", got, want)
	}
}

// TestSwitchingScopeKeepsTheConditions is D3: clicking Inbox used to drop the tags
// silently, which turned "the inbox has nothing tagged #go" into "the inbox is empty".
func TestSwitchingScopeKeepsTheConditions(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seedTagged(t, db, "a", "go")
	seedTagged(t, db, "b", "rust")
	// Seeded rows carry no list row -- a sync assigns the Inbox after inserting -- so the
	// Inbox scope would be empty for a reason that has nothing to do with the filter.
	if _, err := sqlite.NewBookmarkRepository(db).AssignMissingToInbox(ctx, clockBase); err != nil {
		t.Fatalf("AssignMissingToInbox: %v", err)
	}
	shell.Reload(ctx)

	shell.toggleTagFilter(ctx, "go")
	shell.selectSidebar(ctx, selectionInbox)

	if len(shell.state.ActiveTagIDs) != 1 || shell.state.ActiveTagIDs[0] != "go" {
		t.Fatalf("the scope switch dropped the tag: %v", shell.state.ActiveTagIDs)
	}
	if got := len(shell.state.Data.Bookmarks); got != 1 {
		t.Fatalf("%d bookmarks in the inbox tagged #go, want 1", got)
	}
	if got := shell.state.Data.Bookmarks[0].TweetID; got != "a" {
		t.Errorf("the survivor is %q, want a", got)
	}
}

// One value per facet. Clicking a row shows that row; clicking another replaces it, so the
// list is never an intersection
// or a union the reader has to work out from the chips.

// TestAnotherTagReplacesTheFirst explains what a click means now: the row under the
// pointer is the list, not one more term in a query.
func TestAnotherTagReplacesTheFirst(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seedTagged(t, db, "both", "go", "rust")
	seedTagged(t, db, "goonly", "go")
	seedTagged(t, db, "rustonly", "rust")
	shell.Reload(ctx)

	shell.toggleTagFilter(ctx, "go")
	if got := len(shell.state.Data.Bookmarks); got != 2 {
		t.Fatalf("%d results for #go, want 2", got)
	}

	// A different tag substitutes for the one in effect rather than intersecting with it.
	shell.toggleTagFilter(ctx, "rust")
	if got := shell.state.ActiveTagIDs; len(got) != 1 || got[0] != "rust" {
		t.Fatalf("tag conditions = %v, want [rust] alone", got)
	}
	if got := len(shell.state.Data.Bookmarks); got != 2 {
		t.Errorf("%d results for #rust, want 2", got)
	}

	// Clicking the row in effect again clears it.
	shell.toggleTagFilter(ctx, "rust")
	if got := shell.conditionCount(); got != 0 {
		t.Errorf("%d conditions after clicking the active row, want 0", got)
	}
	if got := len(shell.state.Data.Bookmarks); got != 3 {
		t.Errorf("%d results after clearing, want the whole library", got)
	}
}

// TestAnotherAuthorReplacesTheFirst is the same rule for authors, whose union was the
// behaviour the reader noticed: two selected authors used to show both sets at once.
func TestAnotherAuthorReplacesTheFirst(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post", clockBase)
	seed(t, db, "2", "ada", "another post", clockBase)
	seed(t, db, "3", "grace", "a third post", clockBase)
	shell.Reload(ctx)

	shell.toggleAuthorFilter(ctx, "ada")
	if got := len(shell.state.Data.Bookmarks); got != 2 {
		t.Fatalf("%d results for @ada, want 2", got)
	}

	shell.toggleAuthorFilter(ctx, "grace")
	if got := shell.state.ActiveAuthors; len(got) != 1 || got[0] != "grace" {
		t.Fatalf("author conditions = %v, want [grace] alone", got)
	}
	if got := len(shell.state.Data.Bookmarks); got != 1 {
		t.Errorf("%d results for @grace, want 1", got)
	}

	shell.toggleAuthorFilter(ctx, "grace")
	if got := shell.conditionCount(); got != 0 {
		t.Errorf("%d conditions after clicking the active row, want 0", got)
	}
}

// TestAFacetHoldsOneValueWhenAViewComesBack is the saved-view half: an older view may
// hold several values, and restoring an invisible intersection is the behaviour this
// model removed.
func TestAFacetHoldsOneValueWhenAViewComesBack(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seedTagged(t, db, "both", "go", "rust")
	shell.Reload(ctx)

	shell.ApplySavedView(ctx, domain.SavedView{
		Name: "old", ActiveTagIDs: []string{"go", "rust"},
		ActiveAuthorHandles: []string{"ada", "grace"},
	})

	if got := shell.state.ActiveTagIDs; len(got) != 1 || got[0] != "go" {
		t.Errorf("tag conditions = %v, want the view's first value", got)
	}
	if got := shell.state.ActiveAuthors; len(got) != 1 || got[0] != "ada" {
		t.Errorf("author conditions = %v, want the view's first value", got)
	}
}

// Changing a filter does not destroy where the reader is.

// TestTheDetailSurvivesAFilterThatKeepsItsBookmark is the rule that replaced "every
// filter change clears the selection".
func TestTheDetailSurvivesAFilterThatKeepsItsBookmark(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seedTagged(t, db, "keep", "go")
	seedTagged(t, db, "drop", "rust")
	shell.Reload(ctx)

	shell.state.Selected = "keep"
	shell.state.DetailOpen = true

	// The bookmark survives this filter, so the panel stays open.
	shell.toggleTagFilter(ctx, "go")
	if shell.state.Selected != "keep" || !shell.state.DetailOpen {
		t.Fatalf("a filter that kept the bookmark closed its detail (selected=%q open=%v)",
			shell.state.Selected, shell.state.DetailOpen)
	}

	// This one removes it, so the panel goes with it.
	shell.toggleTagFilter(ctx, "go")
	shell.toggleTagFilter(ctx, "rust")
	if shell.state.Selected != "" {
		t.Errorf("selected = %q after the bookmark was filtered out", shell.state.Selected)
	}
	if shell.state.DetailOpen {
		t.Error("the detail stayed open on a bookmark the list no longer shows")
	}
}

// TestTheBatchSelectionFollowsTheFilter keeps bulk actions off rows that are no longer
// on screen: an action must not run on bookmarks the user cannot see.
func TestTheBatchSelectionFollowsTheFilter(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seedTagged(t, db, "keep", "go")
	seedTagged(t, db, "drop", "rust")
	shell.Reload(ctx)

	shell.state.SelectedIDs = map[string]bool{"keep": true, "drop": true}
	shell.toggleTagFilter(ctx, "rust")

	if len(shell.state.SelectedIDs) != 1 || !shell.state.SelectedIDs["drop"] {
		t.Errorf("batch selection = %v, want only the row still on screen", shell.state.SelectedIDs)
	}
}

// An empty result says which conditions emptied it, and how to get back.

// TestTheEmptyResultNamesEveryCondition is the state where the filter matters most: the
// reader is looking at nothing, and the toolbar's chip row is the only other place that
// explains why.
func TestTheEmptyResultNamesEveryCondition(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	// One bookmark carries "go"; nothing carries both tags, so the default all-of reading
	// empties the list. Both tags have to exist: a condition naming one the library does not
	// hold is dropped when the page is read (reload.go), and then nothing empties it.
	seedTagged(t, db, "a", "go")
	seedTagOnly(t, db, "rust", "rust")
	shell.Reload(ctx)

	shell.state.ActiveTagIDs = []string{"go", "rust"}
	shell.state.SidebarFacetTab = sidebarTabAuthors
	shell.Reload(ctx)
	if got := len(shell.state.Data.Bookmarks); got != 0 {
		t.Fatalf("%d results, want an empty list", got)
	}

	tester := ui.NewTester(shell.View, 1180, 900)
	tester.Frame()

	if !tester.HasText(shell.t().T(i18n.EmptyFilter)) {
		t.Errorf("the empty page does not say the filter emptied it")
	}
	counts := map[string]int{}
	for _, text := range tester.Texts() {
		counts[text]++
	}
	// Two of each: the toolbar's row above the list, and the empty page's own list.
	for _, label := range []string{"go", "rust", shell.t().T(i18n.FilterClearAll)} {
		if counts[label] < 2 {
			t.Errorf("%q is drawn %d time(s), want it on the empty page as well as the toolbar",
				label, counts[label])
		}
	}
}

// TestTheEmptyResultOffersAWayOut is the acceptance criterion: one control returns the
// list to unfiltered, without the reader having to find and undo each condition.
func TestTheEmptyResultOffersAWayOut(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seedTagged(t, db, "a", "go")
	seedTagOnly(t, db, "rust", "rust")
	shell.Reload(ctx)

	shell.state.ActiveTagIDs = []string{"go", "rust"}
	shell.state.OnlyWithMedia = true
	shell.state.SidebarFacetTab = sidebarTabAuthors
	shell.Reload(ctx)

	tester := ui.NewTester(shell.View, 1180, 900)
	tester.Frame()
	if err := tester.Click(shell.t().T(i18n.FilterClearAll)); err != nil {
		t.Fatalf("the empty page offers no way out: %v", err)
	}

	if got := shell.conditionCount(); got != 0 {
		t.Errorf("%d conditions after clearing, want 0", got)
	}
	if got := len(shell.state.Data.Bookmarks); got != 1 {
		t.Errorf("%d bookmarks after clearing, want the library back", got)
	}
}

// TestAnEmptyLibraryOffersNoFilterControls keeps the two empty states apart: a library with
// nothing in it has no conditions to remove, and a "clear filters" control there would
// suggest the reader had narrowed something.
func TestAnEmptyLibraryOffersNoFilterControls(t *testing.T) {
	shell, _ := newShell(t)
	ctx := context.Background()
	shell.Reload(ctx)

	if got := shell.conditionCount(); got != 0 {
		t.Fatalf("%d conditions in a fresh shell", got)
	}

	tester := ui.NewTester(shell.View, 1180, 900)
	tester.Frame()

	if !tester.HasText(shell.t().T(i18n.EmptyLibrary)) {
		t.Errorf("an empty library does not say so")
	}
	if tester.HasText(shell.t().T(i18n.EmptyFilter)) {
		t.Errorf("an empty library is described as a filtered one")
	}
	if tester.HasText(shell.t().T(i18n.FilterClearAll)) {
		t.Errorf("an empty library offers to clear filters it does not have")
	}
}

// A sidebar row's number is what the row shows, not the library's lifetime total.

// TestATagRowCountsTheCurrentResult is the property that keeps the sidebar honest: the
// number beside a row is the result clicking it produces. A click replaces the tag in
// effect, so the count is taken with the tag condition left out.
func TestATagRowCountsTheCurrentResult(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seedTagged(t, db, "a", "go")
	seedTagged(t, db, "b", "go", "rust")
	seedTagged(t, db, "c", "rust")
	shell.Reload(ctx)

	shell.toggleTagFilter(ctx, "go")
	if got := len(shell.state.Data.Bookmarks); got != 2 {
		t.Fatalf("%d results for #go, want 2", got)
	}

	// go is on two bookmarks, rust on two (b and c). The active row's number is the
	// current result; the other row's is what replacing the condition would show.
	counts := map[string]int{}
	for _, tag := range shell.state.Data.Tags {
		counts[tag.Name] = shell.tagCount(tag)
	}
	for name, want := range map[string]int{"go": 2, "rust": 2} {
		if counts[name] != want {
			t.Errorf("row %q shows %d, want %d", name, counts[name], want)
		}
	}

	// And the number is a promise: clicking the row shows exactly that many.
	shell.toggleTagFilter(ctx, "rust")
	if got := len(shell.state.Data.Bookmarks); got != counts["rust"] {
		t.Errorf("the row said %d and the list has %d", counts["rust"], got)
	}
}

// TestAuthorRowsFollowTheConditions is the same rule for the other facet.
func TestAuthorRowsFollowTheConditions(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post", clockBase)
	seed(t, db, "2", "ada", "another post", clockBase)
	seed(t, db, "3", "grace", "a third post", clockBase)
	shell.Reload(ctx)

	counts := map[string]int{}
	for _, author := range shell.state.Data.Authors {
		counts[author.AuthorHandle] = shell.authorCount(author)
	}
	if counts["ada"] != 2 || counts["grace"] != 1 {
		t.Fatalf("author rows show %v, want ada=2 grace=1", counts)
	}

	shell.toggleAuthorFilter(ctx, "grace")
	if got := len(shell.state.Data.Bookmarks); got != counts["grace"] {
		t.Errorf("the row said %d and the list has %d", counts["grace"], got)
	}
}

// TestTheSidebarCountFallsBackWithoutFacetCounts keeps an older view usable: a view built
// before the counts were loaded has no numbers of its own, and claiming zero would empty
// every row's label.
func TestTheSidebarCountFallsBackWithoutFacetCounts(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seedTagged(t, db, "a", "go")
	shell.Reload(ctx)

	shell.state.Data.FacetCounts = domain.FacetCounts{}
	for _, tag := range shell.state.Data.Tags {
		if tag.Name != "go" {
			continue
		}
		if got := shell.tagCount(tag); got != tag.BookmarkCount {
			t.Errorf("without facet counts the row shows %d, want the summary's %d",
				got, tag.BookmarkCount)
		}
	}
}

// The scroll offset needs no test here, and this note says why rather than leaving the
// gap unnamed. MyGo keeps it on the scroll node (context.go:508-510), keyed by the node's
// id; a condition change rebuilds the rows inside it but not the scroll node, so the
// offset survives. Measured on 40 seeded rows: after a 900-point wheel, changing an author
// condition moved row 20 by 12 points, which is exactly the height the new chip adds to
// the toolbar above the list -- a layout shift, not a return to the top. A test that
// asserted this through Tester.Find would compare content coordinates, which do not move
// with the scroll at all, so it would pass whether or not the offset was kept.

// TestALongFilterNameStaysOnOneLine is the reader's screenshot: "@歸藏(guizang.ai)" wrapped
// inside the chip, so the chip stood two lines tall in the search's 28-point row.
//
// It wrapped at every window width, not only a narrow one: the name is one CJK run followed
// by a Latin one, MyGo breaks between the two, and the chip had no line limit to stop it.
// A condition is a label, not a paragraph. The name is drawn on one line with an ellipsis and
// capped (ChipMaxWidth), so the row keeps its height whatever the author is called.
func TestALongFilterNameStaysOnOneLine(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	bookmark := domain.Bookmark{
		TweetID: "1", TweetURL: "https://x.com/op7418/status/1",
		AuthorName: "歸藏(guizang.ai)", AuthorHandle: "op7418", Text: "a post",
		SavedAt: clockBase, LastSeenAt: clockBase, UpdatedAt: clockBase,
	}
	if _, err := sqlite.NewBookmarkRepository(db).UpsertMany(ctx, []domain.Bookmark{bookmark}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}
	shell.Reload(ctx)

	// A default window: this is not a narrow-window defect. The label was two lines at 900,
	// 1180 and 1600 alike -- the break is in the text, not in the row.
	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	shell.state.ActiveAuthors = []string{"op7418"}
	shell.Reload(ctx)
	tester.Frame()

	name := "@" + bookmark.AuthorName
	label, found := tester.Find(name)
	if !found {
		t.Fatalf("the condition has no chip; texts: %q", tester.Texts())
	}
	if label.H >= ChipMinHeight {
		t.Errorf("the chip's label is %.0f high, past the %d-point chip: it wrapped",
			label.H, ChipMinHeight)
	}
	if label.W > ChipMaxWidth {
		t.Errorf("the chip's label is %.0f wide, past the %d cap", label.W, ChipMaxWidth)
	}
}

// TestAVeryLongFilterNameIsCapped is the same rule with a name that is longer than the cap:
// the chip keeps the width the toolbar lends it and the name ellipsises, rather than one
// condition pushing the search field and the actions off the row.
func TestAVeryLongFilterNameIsCapped(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	long := "一个很长很长的作者显示名称用来测试芯片宽度上限是否生效以及省略号是否正确显示"
	bookmark := domain.Bookmark{
		TweetID: "2", TweetURL: "https://x.com/longname/status/2",
		AuthorName: long, AuthorHandle: "longname", Text: "a post",
		SavedAt: clockBase, LastSeenAt: clockBase, UpdatedAt: clockBase,
	}
	if _, err := sqlite.NewBookmarkRepository(db).UpsertMany(ctx, []domain.Bookmark{bookmark}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}
	shell.SetLocale("zh-CN")
	shell.Reload(ctx)
	shell.state.ActiveAuthors = []string{"longname"}
	shell.Reload(ctx)

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	label, found := tester.Find("@" + long)
	if !found {
		t.Fatalf("the condition has no chip; texts: %q", tester.Texts())
	}
	if label.W > ChipMaxWidth+1 {
		t.Errorf("the chip's label is %.0f wide, past the %d cap", label.W, ChipMaxWidth)
	}
	if label.H >= ChipMinHeight {
		t.Errorf("the chip's label is %.0f high: it wrapped", label.H)
	}
}
