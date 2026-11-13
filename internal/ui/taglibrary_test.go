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

// The tag library's rewrite, tested at the two points the screen did not work.
//
// The old screen drew a count and a parent per row and never the tag's name, and the panel that
// renames a tag was reachable only by creating one. Both are behaviours a reader can point at,
// so both are tested by acting on the screen rather than by reading its state.

// TestATagNameOpensThatTagsBookmarks covers opening a tag's bookmarks.
//
// The library could inspect tags and had no way to show what was filed under them; its name is
// the link to the list.
func TestATagNameOpensThatTagsBookmarks(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seedTagOnly(t, db, "a", "rust")
	shell.ReloadTagLibrary(ctx)
	shell.ToggleTagMode(ctx)

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	// The only "rust" in the frame: the sidebar's tree is empty because only the library was
	// loaded, which is what makes the click unambiguous.
	if err := tester.Click("rust"); err != nil {
		t.Fatalf("clicking the name: %v", err)
	}
	tester.Frame()

	if shell.State().TagMode {
		t.Error("the library stayed open; the name should leave it")
	}
	if got := shell.State().ActiveTagIDs; len(got) != 1 || got[0] != "a" {
		t.Errorf("the filter is %q, want the clicked tag only", got)
	}
}

// TestTheTagRowRenamesInPlace covers the in-place rename.
//
// The pencil turns the name into an input where the name is read, and the input commits on
// Enter.
func TestTheTagRowRenamesInPlace(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seedTagOnly(t, db, "a", "rust")
	shell.ReloadTagLibrary(ctx)
	shell.ToggleTagMode(ctx)

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	if err := tester.Click(shell.t().Tf(i18n.TagRenameFor, "rust")); err != nil {
		t.Fatalf("clicking the row's rename: %v", err)
	}
	tester.Frame()

	if shell.State().TagFocus != "a" || shell.State().TagEditField != TagEditName {
		t.Fatalf("the open field is %q on %q, want the name on the clicked row",
			shell.State().TagEditField, shell.State().TagFocus)
	}
	if shell.State().TagNameEdit != "rust" {
		t.Errorf("the field starts at %q, want the tag's name", shell.State().TagNameEdit)
	}

	// Enter writes and closes. The text is typed into the field, not poked into the state: the
	// field owns the draft while it is open, and a value set behind it would be the value the
	// input writes back.
	tester.Type("-lang")
	tester.Key(0, ui.KeyEnter)
	tester.Frame()
	if shell.State().TagFocus != "" || shell.State().TagEditField != "" {
		t.Errorf("the field stayed open after Enter: %q on %q",
			shell.State().TagEditField, shell.State().TagFocus)
	}
	if tag, found, _ := sqlite.NewTagRepository(db).Get(ctx, "a"); !found || tag.Name != "rust-lang" {
		t.Errorf("the tag is named %q after Enter, want the draft", tag.Name)
	}

	// Escape closes without writing.
	if err := tester.Click(shell.t().Tf(i18n.TagRenameFor, "rust-lang")); err != nil {
		t.Fatalf("reopening the rename: %v", err)
	}
	tester.Frame()
	tester.Type("-discarded")
	tester.Key(0, ui.KeyEscape)
	tester.Frame()
	if shell.State().TagFocus != "" {
		t.Error("Escape left the field open")
	}
	if tag, _, _ := sqlite.NewTagRepository(db).Get(ctx, "a"); tag.Name != "rust-lang" {
		t.Errorf("Escape saved %q", tag.Name)
	}
}

// TestTheTagRowEditsAliasesInPlace covers the alias editor's placement.
//
// The input with its Save and cancel belongs in the alias slot of the row, so the field replaces
// the value it is about instead of sitting third in a column of unrelated fields, with the Save
// on a different edge from the renames.
func TestTheTagRowEditsAliasesInPlace(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seedTagOnly(t, db, "a", "rust")
	shell.ReloadTagLibrary(ctx)
	shell.ToggleTagMode(ctx)

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	if err := tester.Click(shell.t().Tf(i18n.TagAliasesFor, "rust")); err != nil {
		t.Fatalf("clicking the row's aliases: %v", err)
	}
	tester.Frame()

	if shell.State().TagEditField != TagEditAliases || shell.State().TagFocus != "a" {
		t.Fatalf("the open field is %q on %q, want the aliases",
			shell.State().TagEditField, shell.State().TagFocus)
	}
	if !tester.HasText(shell.t().T(i18n.ActionSave)) {
		t.Errorf("the alias field has no save; texts: %q", tester.Texts())
	}
	if !tester.HasText(shell.t().T(i18n.ActionCancel)) {
		t.Errorf("the alias field has no cancel; texts: %q", tester.Texts())
	}

	shell.State().TagAliasInput = "rs, rustlang"
	if err := tester.Click(shell.t().T(i18n.ActionSave)); err != nil {
		t.Fatalf("clicking save: %v", err)
	}
	tester.Frame()

	if shell.State().TagEditField != "" {
		t.Error("the alias field stayed open after saving")
	}
	item, found, err := sqlite.NewTagRepository(db).Get(ctx, "a")
	if err != nil || !found {
		t.Fatalf("Get: %v found=%v", err, found)
	}
	if len(item.Aliases) != 2 || item.Aliases[0] != "rs" || item.Aliases[1] != "rustlang" {
		t.Errorf("the aliases are %q, want both, trimmed", item.Aliases)
	}
}

// TestTheThemeCellOpensAPicker covers the third field's new home.
//
// A select in the row's parent cell writes on change. The cell shows the value until it is
// clicked, which is what a table does with an editable cell.
func TestTheThemeCellOpensAPicker(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seedTagOnly(t, db, "a", "rust")
	seedTagOnly(t, db, "b", "languages")
	shell.ReloadTagLibrary(ctx)
	shell.ToggleTagMode(ctx)

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	none := shell.t().T(i18n.TagParentNone)
	if err := tester.Click(none); err != nil {
		t.Fatalf("clicking the theme cell: %v", err)
	}
	tester.Frame()
	if shell.State().TagEditField != TagEditTheme {
		t.Fatalf("the open field is %q, want the theme", shell.State().TagEditField)
	}
	edited := shell.State().TagFocus

	// The picker writes on change, like a select does. It takes the keyboard
	// when it opens: Down opens the list, Down moves to the other root tag, Enter chooses it.
	tester.Key(0, ui.KeyDown)
	tester.Key(0, ui.KeyDown)
	tester.Key(0, ui.KeyEnter)
	tester.Frame()
	if shell.State().TagEditField != "" {
		t.Error("the picker stayed open after a choice")
	}
	if edited == "" {
		t.Fatal("the picker did not open on a row")
	}
	other := "a"
	if edited == "a" {
		other = "b"
	}
	tag, found, err := sqlite.NewTagRepository(db).Get(ctx, edited)
	if err != nil || !found {
		t.Fatalf("Get: %v found=%v", err, found)
	}
	if tag.ParentID != other {
		t.Errorf("the theme of %q is %q, want %q", edited, tag.ParentID, other)
	}
}

// TestTheSidebarSearchCostsNoSpaceUntilItIsOpened covers the row the field used to take.
//
// It sat above both lists on every frame, so the list always started 46 points down. The
// magnifier opens it for the tab in effect; closing it clears that tab's query, because a field
// that is not on screen must not be narrowing the list.
func TestTheSidebarSearchCostsNoSpaceUntilItIsOpened(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(ctx)

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	// The tags tab's list starts with its empty-tags line here, because the fixture has no tags.
	// The 全部书签 row is gone, so that line is the anchor.
	anchor := shell.t().T(i18n.SidebarNoTagsCreated)
	closed, found := tester.Find(anchor)
	if !found {
		t.Fatalf("the first row is missing; texts: %q", tester.Texts())
	}

	search := shell.t().T(i18n.SearchLabel)
	if err := tester.Click(search); err != nil {
		t.Fatalf("clicking the magnifier: %v", err)
	}
	tester.Frame()
	if !shell.State().SidebarSearchOpen {
		t.Fatal("the magnifier did not open the field")
	}
	opened, _ := tester.Find(anchor)
	if opened.Y <= closed.Y {
		t.Errorf("the list did not move down for the field: %v then %v", closed.Y, opened.Y)
	}

	// A query typed into it is cleared when it closes: a hidden field filtering the list is the
	// state this rule exists to prevent.
	shell.State().SidebarTagQuery = "rus"
	if err := tester.Click(search); err != nil {
		t.Fatalf("clicking the magnifier again: %v", err)
	}
	tester.Frame()
	if shell.State().SidebarSearchOpen {
		t.Error("the field stayed open")
	}
	if shell.State().SidebarTagQuery != "" {
		t.Errorf("the query survived the close: %q", shell.State().SidebarTagQuery)
	}
	again, _ := tester.Find(anchor)
	if again.Y != closed.Y {
		t.Errorf("the list did not return to its place: %v, want %v", again.Y, closed.Y)
	}
}

// TestTheFilterPillsShowWhichOneIsOn covers the state the old buttons never showed.
//
// Five flat buttons with no selected look left the reader unable to tell which filter was in
// effect, which on a screen about drift is the one thing it has to answer.
func TestTheFilterPillsShowWhichOneIsOn(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seedTagOnly(t, db, "a", "rust")
	shell.ReloadTagLibrary(ctx)
	shell.ToggleTagMode(ctx)

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	orphan := shell.t().T(i18n.TagFilterOrphan)
	if err := tester.Click(orphan); err != nil {
		t.Fatalf("clicking the filter: %v", err)
	}
	tester.Frame()

	if shell.State().TagFilter != domain.TagFilterOrphan {
		t.Errorf("the filter is %q, want the clicked one", shell.State().TagFilter)
	}
}

// TestTheTableHeadSelectsEveryListedTag covers the select column's checkbox.
//
// The bulk actions (merge, delete selected) only appear with a selection, and the way to make
// one in a step is the table head. It is the same call the old "select all listed"
// button made, so the test asserts the count rather than the label it is drawn with.
func TestTheTableHeadSelectsEveryListedTag(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seedTagOnly(t, db, "a", "rust")
	seedTagOnly(t, db, "b", "compilers")
	shell.ReloadTagLibrary(ctx)
	shell.ToggleTagMode(ctx)

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	if err := tester.Click(shell.t().T(i18n.TagSelectAll)); err != nil {
		t.Fatalf("clicking the head checkbox: %v", err)
	}
	tester.Frame()

	if got := shell.TagSelectionCount(); got != 2 {
		t.Errorf("selected %d tags, want the two listed", got)
	}
	// And the bulk actions the selection feeds are now on screen.
	if !tester.HasText(shell.t().T(i18n.TagMergeLabel)) {
		t.Errorf("the merge is missing with a selection; texts: %q", tester.Texts())
	}
}

// TestTheTagsPageLeavesThroughTheHeader covers the exit the toolbar used to own.
//
// Both pages of tag management are inside TagMode, which has no window chrome of its own, so the
// breadcrumb above the title is the only way back to the bookmarks. It is drawn on both pages,
// which is what let the tag table's toolbar lose a button.
func TestTheTagsPageLeavesThroughTheHeader(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seedTagOnly(t, db, "a", "rust")
	shell.ReloadTagLibrary(ctx)
	shell.ToggleTagMode(ctx)

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	if err := tester.Click(shell.t().T(i18n.TagExitManagement)); err != nil {
		t.Fatalf("clicking the breadcrumb: %v", err)
	}

	if shell.State().TagMode {
		t.Error("the breadcrumb did not leave tag management")
	}
}

// TestTheHeaderStatesTheLibraryInOneLine pins the density claim: the two count lines and the
// 36-point total the header used to carry are one line now, and no fact was dropped with them.
func TestTheHeaderStatesTheLibraryInOneLine(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seedTagOnly(t, db, "a", "rust")
	seedTagOnly(t, db, "b", "compilers")
	shell.ReloadTagLibrary(ctx)
	shell.ToggleTagMode(ctx)

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	// The two tags carry no bookmarks, and a long-tail tag is one with 1-2 bookmarks and no
	// children (domain.IsLongTailTag), so that count is zero rather than two.
	want := fmt.Sprintf("2 %s · 2 %s · 2 %s · 0 %s · %s",
		shell.t().T(i18n.TagTagsSuffix),
		shell.t().T(i18n.TagOrphanSuffix),
		shell.t().T(i18n.TagUnparentedSuffix),
		shell.t().T(i18n.TagLongTailSuffix),
		shell.t().T(i18n.TagHealthOk))
	if !tester.HasText(want) {
		t.Errorf("the header does not state the library in one line; texts: %q", tester.Texts())
	}
}

// TestTheToolbarGroupsItsRows covers the order the controls are read in: the order and the view
// above, what the filter left below them, and the actions under that. The actions used to share
// their row with nothing on the left, which is the empty band this rework removes.
func TestTheToolbarGroupsItsRows(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seedTagOnly(t, db, "a", "rust")
	shell.ReloadTagLibrary(ctx)
	shell.ToggleTagMode(ctx)

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	sortControl, ok := tester.Find(shell.t().T(i18n.TagSortMost))
	if !ok {
		t.Fatalf("the sort control is missing; texts: %q", tester.Texts())
	}
	chip, ok := tester.Find(shell.t().T(i18n.TagFilterAll))
	if !ok {
		t.Fatalf("the filter row is missing")
	}
	create, ok := tester.Find(shell.t().T(i18n.CreateTagLabel))
	if !ok {
		t.Fatalf("the create action is missing")
	}

	if sortControl.Y+sortControl.H > chip.Y {
		t.Errorf("the sort (y=%.0f..%.0f) is not above the filters (y=%.0f)",
			sortControl.Y, sortControl.Y+sortControl.H, chip.Y)
	}
	if chip.Y+chip.H > create.Y {
		t.Errorf("the filters (y=%.0f..%.0f) are not above the actions (y=%.0f)",
			chip.Y, chip.Y+chip.H, create.Y)
	}
}

// TestANarrowedTagListSaysHowManyItLeft covers the context row's count.
//
// The filter row was the chips alone with the rest of its width empty; the number is what the
// reader wants after narrowing, and it is absent again once the list is whole.
func TestANarrowedTagListSaysHowManyItLeft(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seedTagOnly(t, db, "a", "rust")
	seedTagOnly(t, db, "b", "compilers")
	shell.ReloadTagLibrary(ctx)
	shell.ToggleTagMode(ctx)

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	whole := fmt.Sprintf("2 %s", shell.t().T(i18n.TagTagsSuffix))
	for _, text := range tester.Texts() {
		if text == whole {
			t.Fatalf("the unfiltered list repeats the header's count: %q", text)
		}
	}

	shell.State().TagQuery = "rust"
	tester.Frame()
	narrowed := fmt.Sprintf("1 / 2 %s", shell.t().T(i18n.TagTagsSuffix))
	if !tester.HasText(narrowed) {
		t.Errorf("the narrowed list does not say %q; texts: %q", narrowed, tester.Texts())
	}
}

// TestTheEmptyLibraryOffersItsOwnWayIn covers the state the page used to render as one muted
// sentence: with no tags, the actions row is gone and the card carries the create action.
func TestTheEmptyLibraryOffersItsOwnWayIn(t *testing.T) {
	shell, _ := newShell(t)
	ctx := context.Background()
	shell.ReloadTagLibrary(ctx)
	shell.ToggleTagMode(ctx)

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	for _, key := range []i18n.Key{i18n.TagNoTagsCreated, i18n.TagNoTagsDescription} {
		if !tester.HasText(shell.t().T(key)) {
			t.Errorf("the empty card does not draw %q; texts: %q", shell.t().T(key), tester.Texts())
		}
	}
	if tester.HasText(shell.t().T(i18n.ReviewLabel)) {
		t.Error("the empty library still offers the vocabulary review")
	}

	if err := tester.Click(shell.t().T(i18n.CreateTagLabel)); err != nil {
		t.Fatalf("the card has no create action: %v", err)
	}
	if !shell.State().TagLibraryCreateOpen {
		t.Error("the create action did not open the draft row")
	}
}

// TestTheNoMatchCardClearsTheFilters covers the second empty state's action.
func TestTheNoMatchCardClearsTheFilters(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seedTagOnly(t, db, "a", "rust")
	shell.ReloadTagLibrary(ctx)
	shell.ToggleTagMode(ctx)
	shell.State().TagQuery = "zzz-no-such-tag"

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	if !tester.HasText(shell.t().T(i18n.TagNoMatchDescription)) {
		t.Errorf("the no-match card has no advice; texts: %q", tester.Texts())
	}
	if err := tester.Click(shell.t().T(i18n.FilterClearAll)); err != nil {
		t.Fatalf("the card has no way back to the list: %v", err)
	}
	tester.Frame()

	if shell.State().TagQuery != "" || shell.State().TagFilter != domain.TagFilterAll {
		t.Errorf("the filters survived: query %q, filter %q",
			shell.State().TagQuery, shell.State().TagFilter)
	}
	if tester.HasText(shell.t().T(i18n.TagNoMatchTitle)) {
		t.Error("the empty state stayed after the filters were cleared")
	}
}

// TestTheBulkBarCanBeDismissed covers the ✕ the selection bar gained.
//
// The table head's checkbox and the rows' checkboxes could make a selection; nothing undid it in
// a step, so a reader who had ticked a page of tags had to visit every row to get the actions
// row back.
func TestTheBulkBarCanBeDismissed(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seedTagOnly(t, db, "a", "rust")
	seedTagOnly(t, db, "b", "compilers")
	shell.ReloadTagLibrary(ctx)
	shell.ToggleTagMode(ctx)

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	if err := tester.Click(shell.t().T(i18n.TagSelectAll)); err != nil {
		t.Fatalf("clicking the head checkbox: %v", err)
	}
	tester.Frame()

	if !tester.HasText(shell.t().Tf(i18n.TagBatchSelected, 2)) {
		t.Errorf("the bar does not say how many are selected; texts: %q", tester.Texts())
	}
	if err := tester.Click(shell.t().T(i18n.ActionClearSelection)); err != nil {
		t.Fatalf("the bar has no clear control: %v", err)
	}
	tester.Frame()

	if got := shell.TagSelectionCount(); got != 0 {
		t.Errorf("%d tags are still selected", got)
	}
	if tester.HasText(shell.t().T(i18n.TagMergeLabel)) {
		t.Error("the bulk bar stayed after the selection was cleared")
	}
}

// TestTheTableDropsTheTimestampWhenThePaneIsNarrow covers the one column that gives way.
//
// At the 900-point minimum window the fixed columns left the name column 43 points, so the
// tags themselves read as ellipses. The timestamp is the column a reader can spare.
func TestTheTableDropsTheTimestampWhenThePaneIsNarrow(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seedTagOnly(t, db, "a", "rust")
	shell.ReloadTagLibrary(ctx)
	shell.ToggleTagMode(ctx)

	wide := ui.NewTester(shell.View, 1180, 760)
	wide.Frame()
	if !wide.HasText(strings.ToUpper(shell.t().T(i18n.TagTableUpdated))) {
		t.Errorf("the wide table has no timestamp column; texts: %q", wide.Texts())
	}

	narrow := ui.NewTester(shell.View, 900, 620)
	narrow.Frame()
	if narrow.HasText(strings.ToUpper(shell.t().T(i18n.TagTableUpdated))) {
		t.Error("the narrow table keeps a column it has no room for")
	}
	if !narrow.HasText("rust") {
		t.Errorf("the narrow table lost the tag name; texts: %q", narrow.Texts())
	}
}

// TestDeletingTheFilteredTagDropsTheCondition is the reader's report: "删除标签异常会导致渲染
// 异常，以及操作后页面不刷新".
//
// Deleting a tag refreshed only the tag library. The results kept the deleted tag in their
// cards, the sidebar kept listing it, and if it was the active condition the chip had nothing
// left to call it -- tagLabel falls back to the uuid -- so the page read "当前筛选条件下没有匹配
// 的书签" under a chip printing an internal id, with nothing saying a tag had been deleted.
//
// So the delete now does three things: it drops the condition naming the tag, it reads the
// results again, and (as before) it reads the library again.
func TestDeletingTheFilteredTagDropsTheCondition(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "a", "ada", "a post about compilers", clockBase)
	shell.Reload(ctx)

	// A tag with a name of its own: the id has to be distinguishable from it for the chip
	// assertion below to mean anything.
	if _, err := shell.service.AddTags(ctx, []string{"a"}, []string{"Rust"}); err != nil {
		t.Fatalf("AddTags: %v", err)
	}
	shell.Reload(ctx)
	if len(shell.state.Data.Tags) != 1 {
		t.Fatalf("%d tags, want the one just filed", len(shell.state.Data.Tags))
	}
	tag := shell.state.Data.Tags[0]

	shell.state.ActiveTagIDs = []string{tag.ID}
	shell.Reload(ctx)
	if got := len(shell.state.Data.Bookmarks); got != 1 {
		t.Fatalf("%d rows under the tag filter, want the tagged post", got)
	}

	shell.state.ConfirmDeleteTag = tag.ID
	shell.ConfirmDeleteTagNow(ctx)

	if len(shell.state.ActiveTagIDs) != 0 {
		t.Errorf("the filter still names the deleted tag: %v", shell.state.ActiveTagIDs)
	}
	// The page was re-read: the post is back, and it no longer carries the tag.
	if got := len(shell.state.Data.Bookmarks); got != 1 {
		t.Fatalf("%d rows after the delete, want the post the filter was hiding", got)
	}
	for _, row := range shell.state.Data.Bookmarks {
		if len(row.Tags) != 0 {
			t.Errorf("the row still carries %v", row.Tags)
		}
	}
	if len(shell.state.Data.Tags) != 0 {
		t.Errorf("the sidebar still lists %v", shell.state.Data.Tags)
	}

	tester := ui.NewTester(shell.View, 1180, 900)
	tester.Frame()
	if tester.HasText(shell.t().T(i18n.EmptyFilter)) {
		t.Error("the page still says a filter emptied it")
	}
	for _, text := range tester.Texts() {
		if text == tag.ID {
			t.Errorf("a chip prints the deleted tag's uuid: %q", tag.ID)
		}
	}
}

// TestTheAuthorFilterWorksAfterADeletedTag is the second half of the report: "这时候点击作者的
// 话，页面筛选的话，结果也是有异常的".
//
// The author filter was not at fault. It was stacking on a condition naming a tag that no
// longer existed, so any author the reader picked stayed on an empty list. The kill is the
// deleted condition, not the author.
func TestTheAuthorFilterWorksAfterADeletedTag(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "a", "ada", "a post about compilers", clockBase)
	shell.Reload(ctx)
	if _, err := shell.service.AddTags(ctx, []string{"a"}, []string{"Rust"}); err != nil {
		t.Fatalf("AddTags: %v", err)
	}
	shell.Reload(ctx)

	shell.state.ActiveTagIDs = []string{shell.state.Data.Tags[0].ID}
	shell.Reload(ctx)
	shell.state.ConfirmDeleteTag = shell.state.Data.Tags[0].ID
	shell.ConfirmDeleteTagNow(ctx)

	shell.toggleAuthorFilter(ctx, "ada")
	tester := ui.NewTester(shell.View, 1180, 900)
	tester.Frame()

	if got := len(shell.state.Data.Bookmarks); got != 1 {
		t.Errorf("%d rows under the author filter, want the post by that author", got)
	}
	if !tester.HasText("@ada") {
		t.Errorf("the author's chip is not drawn; texts: %q", tester.Texts())
	}
	if got := shell.conditionCount(); got != 1 {
		t.Errorf("%d conditions, want the author alone", got)
	}
}

// TestRenamingATagRefreshesTheCards is the same half-刷新 from the other direction: the cards
// draw the tag by name, and the library's rename refreshed only the library.
func TestRenamingATagRefreshesTheCards(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "a", "ada", "a post about compilers", clockBase)
	shell.Reload(ctx)
	if _, err := shell.service.AddTags(ctx, []string{"a"}, []string{"Rust"}); err != nil {
		t.Fatalf("AddTags: %v", err)
	}
	shell.Reload(ctx)

	shell.state.TagFocus = shell.state.Data.Tags[0].ID
	shell.state.TagNameEdit = "Compilers"
	shell.RenameFocusedTag(ctx)

	if len(shell.state.Data.Bookmarks) != 1 || len(shell.state.Data.Bookmarks[0].Tags) != 1 {
		t.Fatalf("the row lost its tag: %+v", shell.state.Data.Bookmarks)
	}
	if got := shell.state.Data.Bookmarks[0].Tags[0].Name; got != "Compilers" {
		t.Errorf("the card still draws the tag as %q, want the new name", got)
	}
}
