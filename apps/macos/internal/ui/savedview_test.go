package ui

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/x-master/apps/macos/internal/domain"
	"github.com/taoquo/x-master/apps/macos/internal/i18n"
	"github.com/taoquo/x-master/apps/macos/internal/importexport"
	"github.com/taoquo/x-master/apps/macos/internal/ports"
	"github.com/taoquo/x-master/apps/macos/internal/storage/sqlite"
)

// TestSavedViewsAreEmptyAtFirst keeps a fresh install from listing anything.
func TestSavedViewsAreEmptyAtFirst(t *testing.T) {
	shell, _ := newShell(t)
	shell.Reload(context.Background())
	if len(savedViewsOf(t, shell)) != 0 {
		t.Errorf("a fresh shell lists %d saved views", len(savedViewsOf(t, shell)))
	}
}

// TestSavingTheCurrentView covers the source's create field and its button.
func TestSavingTheCurrentView(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()

	// The filters a view is meant to capture.
	shell.State().Query = "rust async"
	shell.State().OnlyWithMedia = true
	shell.State().OnlyLongform = true
	shell.State().ViewMode = domain.ViewList
	shell.State().SavedViewName = "Async reading"

	shell.SaveCurrentView(ctx)

	if shell.State().SavedViewName != "" {
		t.Errorf("the field still holds %q", shell.State().SavedViewName)
	}
	if len(savedViewsOf(t, shell)) != 1 {
		t.Fatalf("the shell lists %d views", len(savedViewsOf(t, shell)))
	}

	stored, err := sqlite.NewSavedViewRepository(db).List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(stored) != 1 {
		t.Fatalf("the store holds %d views", len(stored))
	}

	got := stored[0]
	if got.Name != "Async reading" {
		t.Errorf("the name is %q", got.Name)
	}
	if got.Query != "rust async" {
		t.Errorf("the query is %q", got.Query)
	}
	if !got.OnlyWithMedia || !got.OnlyLongform {
		t.Errorf("the switches are media=%v longform=%v", got.OnlyWithMedia, got.OnlyLongform)
	}
	if got.ViewMode != domain.ViewList {
		t.Errorf("the layout is %q", got.ViewMode)
	}
}

// TestAnUnnamedViewGetsTheSourcesDefaultName covers the fallback.
//
// The source writes DEFAULT_SAVED_VIEW_NAME rather than refusing, and that constant is
// the literal English "Saved view" in both locales.
func TestAnUnnamedViewGetsTheSourcesDefaultName(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()

	for _, blank := range []string{"", "   "} {
		shell.State().SavedViewName = blank
		shell.SaveCurrentView(ctx)
	}

	stored, err := sqlite.NewSavedViewRepository(db).List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(stored) != 2 {
		t.Fatalf("a blank name stored %d views, want one per press", len(stored))
	}
	for _, view := range stored {
		if view.Name != "Saved view" {
			t.Errorf("a blank name produced %q, want the source's default", view.Name)
		}
	}
}

// TestApplyingASavedViewRestoresEveryField covers the round trip.
func TestApplyingASavedViewRestoresEveryField(t *testing.T) {
	shell, _ := newShell(t)
	ctx := context.Background()

	shell.State().Query = "rust async"
	shell.State().OnlyWithMedia = true
	shell.State().ViewMode = domain.ViewList
	shell.State().Sort = domain.SortSavedDesc
	shell.State().SavedViewName = "Async"
	shell.SaveCurrentView(ctx)

	// Somebody changes everything.
	shell.State().Query = ""
	shell.State().OnlyWithMedia = false
	shell.State().ViewMode = domain.ViewGrid
	shell.State().Sort = domain.SortTimeline
	shell.State().SelectedIDs = map[string]bool{"x": true}

	shell.ApplySavedView(ctx, savedViewsOf(t, shell)[0])

	if shell.State().Query != "rust async" {
		t.Errorf("the query came back as %q", shell.State().Query)
	}
	if !shell.State().OnlyWithMedia {
		t.Error("the media switch came back off")
	}
	if shell.State().ViewMode != domain.ViewList {
		t.Errorf("the layout came back as %q", shell.State().ViewMode)
	}
	if shell.State().Sort != domain.SortSavedDesc {
		t.Errorf("the order came back as %q", shell.State().Sort)
	}
	// The bulk selection is cleared: it belonged to the filters that were on screen.
	if len(shell.State().SelectedIDs) != 0 {
		t.Errorf("applying a view kept %d selections", len(shell.State().SelectedIDs))
	}
}

// TestDeletingASavedView covers the row's delete control.
func TestDeletingASavedView(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()

	shell.State().SavedViewName = "One"
	shell.SaveCurrentView(ctx)
	shell.State().SavedViewName = "Two"
	shell.SaveCurrentView(ctx)

	if len(savedViewsOf(t, shell)) != 2 {
		t.Fatalf("saved %d views", len(savedViewsOf(t, shell)))
	}
	target := savedViewsOf(t, shell)[0].ID

	shell.DeleteSavedView(ctx, target)

	if len(savedViewsOf(t, shell)) != 1 {
		t.Errorf("after deleting, %d views remain", len(savedViewsOf(t, shell)))
	}
	for _, view := range savedViewsOf(t, shell) {
		if view.ID == target {
			t.Error("the deleted view is still listed")
		}
	}
	stored, err := sqlite.NewSavedViewRepository(db).List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(stored) != 1 {
		t.Errorf("the store holds %d views", len(stored))
	}

	// Deleting nothing is a no-op rather than a panic.
	shell.DeleteSavedView(ctx, "")
	if len(savedViewsOf(t, shell)) != 1 {
		t.Error("an empty id deleted something")
	}
}

// TestSavedViewsSurviveARestart is the property that makes them useful.
func TestSavedViewsSurviveARestart(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()

	shell.State().Query = "kept"
	shell.State().SavedViewName = "Kept"
	shell.SaveCurrentView(ctx)

	// The reload that every launch performs reads from storage, not from the field that
	// was just written, so dropping the in-memory list and reloading is the same path.
	if _, found, err := sqlite.NewSavedViewRepository(db).Get(ctx, savedViewsOf(t, shell)[0].ID); err != nil || !found {
		t.Fatalf("the view was not stored: found=%v err=%v", found, err)
	}

	// The "drop the shell's in-memory copy and reload" half is gone with the copy.
	//
	// Round 472 removed State.SavedViews (its only reader was the sidebar section it also
	// removed), so there is nothing cached to drop and nothing for a reload to restore. The
	// storage assertion above is what this test was really making.
	if savedViewsOf(t, shell)[0].Query != "kept" {
		t.Errorf("the restored query is %q", savedViewsOf(t, shell)[0].Query)
	}
}

// seedTagged stores a bookmark carrying the given tags.
func seedTagged(t *testing.T, db *sqlite.DB, id string, tagIDs ...string) {
	t.Helper()

	seed(t, db, id, "ada", "post "+id, clockBase)
	for _, tagID := range tagIDs {
		tag := domain.Tag{
			ID: tagID, Name: tagID, CanonicalKey: domain.CanonicalTagKey(tagID),
			CreatedAt: domain.FormatTimestamp(clockBase), UpdatedAt: domain.FormatTimestamp(clockBase),
		}
		if err := sqlite.NewTagRepository(db).Create(context.Background(), tag); err != nil {
			// A tag that already exists is fine.
			_ = err
		}
		if _, err := db.Exec(
			`INSERT OR IGNORE INTO bookmark_tags (bookmark_id, tag_id, created_at) VALUES (?, ?, ?)`,
			id, tagID, domain.FormatTimestamp(clockBase)); err != nil {
			t.Fatalf("attach tag: %v", err)
		}
	}
}

// TestATagRowShowsOnlyThatTag is the interaction the sidebar has now: a row selects its
// own tag, and a second row replaces the first rather than adding to it.
func TestATagRowShowsOnlyThatTag(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()

	seedTagged(t, db, "both", "rust", "async")
	seedTagged(t, db, "only-rust", "rust")
	seedTagged(t, db, "neither", "cooking")
	shell.Reload(ctx)

	// One tag narrows to the two that carry it.
	shell.toggleTagFilter(ctx, "rust")
	if len(shell.State().ActiveTagIDs) != 1 {
		t.Fatalf("the set holds %v", shell.State().ActiveTagIDs)
	}
	if got := len(shell.State().Data.Bookmarks); got != 2 {
		t.Errorf("one tag matched %d bookmarks, want 2", got)
	}

	// A second tag replaces it: the list shows that tag's bookmarks, not the ones that
	// carry both.
	shell.toggleTagFilter(ctx, "async")
	if got := shell.State().ActiveTagIDs; len(got) != 1 || got[0] != "async" {
		t.Fatalf("the set holds %v, want [async] alone", got)
	}
	if got := len(shell.State().Data.Bookmarks); got != 1 {
		t.Fatalf("%d bookmarks for #async, want 1", got)
	}
	if got := shell.State().Data.Bookmarks[0].TweetID; got != "both" {
		t.Errorf("the survivor is %q", got)
	}

	// Clicking the active row again clears the filter.
	shell.toggleTagFilter(ctx, "async")
	if len(shell.State().ActiveTagIDs) != 0 {
		t.Errorf("clicking the active row left %v", shell.State().ActiveTagIDs)
	}
	if got := len(shell.State().Data.Bookmarks); got != 3 {
		t.Errorf("%d bookmarks after clearing, want the whole library", got)
	}
}

// TestAnotherScopeKeepsTheTagSet is the rule that replaced "a scope switch clears the
// tag set".
//
// The old rule existed because a leftover tag could hide an Inbox with no explanation on
// screen. Conditions now have chips, so the Inbox shows why it is narrow -- and dropping
// the tag silently was the more confusing half: it made "read the Inbox with #rust" (a
// real request) impossible, and turned a filtered view into an empty one without saying
// the filter had gone.
func TestAnotherScopeKeepsTheTagSet(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()

	seedTagged(t, db, "a", "rust")
	shell.Reload(ctx)
	shell.toggleTagFilter(ctx, "rust")

	shell.selectSidebar(ctx, selectionAll)
	if len(shell.State().ActiveTagIDs) != 1 || shell.State().ActiveTagIDs[0] != "rust" {
		t.Errorf("changing scope dropped the tag set %v", shell.State().ActiveTagIDs)
	}
	if len(shell.activeConditions()) != 1 {
		t.Errorf("the chip row lost the tag: %+v", shell.activeConditions())
	}
}

// TestSavedViewKeepsTheTag covers the round trip of the tag condition.
func TestSavedViewKeepsTheTag(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()

	seedTagged(t, db, "both", "rust", "async")
	shell.Reload(ctx)
	shell.toggleTagFilter(ctx, "rust")

	shell.State().SavedViewName = "Rust"
	shell.SaveCurrentView(ctx)

	stored, err := sqlite.NewSavedViewRepository(db).List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(stored) != 1 {
		t.Fatalf("stored %d views", len(stored))
	}
	if len(stored[0].ActiveTagIDs) != 1 || stored[0].ActiveTagIDs[0] != "rust" {
		t.Errorf("the view kept %v, want [rust]", stored[0].ActiveTagIDs)
	}

	// Applying it restores the condition.
	shell.selectSidebar(ctx, selectionAll)
	shell.clearConditions(ctx)
	shell.ApplySavedView(ctx, stored[0])
	if got := shell.State().ActiveTagIDs; len(got) != 1 || got[0] != "rust" {
		t.Errorf("applying restored %v", got)
	}
	if got := len(shell.State().Data.Bookmarks); got != 1 {
		t.Errorf("after applying, %d bookmarks show, want 1", got)
	}
}

// TestAnAuthorRowReplacesTheLast is the same rule for the author dimension, where the
// union of two selected authors was what the reader noticed.
func TestAnAuthorRowReplacesTheLast(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()

	seed(t, db, "a", "ada", "post a", clockBase)
	seed(t, db, "b", "grace", "post b", clockBase)
	seed(t, db, "c", "linus", "post c", clockBase)
	shell.Reload(ctx)

	if len(shell.State().Data.Authors) != 3 {
		t.Fatalf("the service listed %d authors", len(shell.State().Data.Authors))
	}

	shell.toggleAuthorFilter(ctx, "ada")
	if got := len(shell.State().Data.Bookmarks); got != 1 {
		t.Errorf("one author matched %d bookmarks", got)
	}

	// A second author replaces the first: one author's bookmarks, not both sets.
	shell.toggleAuthorFilter(ctx, "grace")
	if got := shell.State().ActiveAuthors; len(got) != 1 || got[0] != "grace" {
		t.Fatalf("the set holds %v, want [grace] alone", got)
	}
	if got := len(shell.State().Data.Bookmarks); got != 1 {
		t.Errorf("%d bookmarks for @grace, want 1", got)
	}

	// Clicking the active row again clears it.
	shell.toggleAuthorFilter(ctx, "grace")
	if len(shell.State().ActiveAuthors) != 0 {
		t.Errorf("clicking the active row left %v", shell.State().ActiveAuthors)
	}
	if got := len(shell.State().Data.Bookmarks); got != 3 {
		t.Errorf("%d bookmarks after clearing, want the whole library", got)
	}
}

// TestAuthorSearchNarrowsTheList covers the section's search field.
//
// The extension searches by handle and by display name.
func TestAuthorSearchNarrowsTheList(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()

	seed(t, db, "a", "ada", "post a", clockBase)
	seed(t, db, "b", "grace", "post b", clockBase)
	shell.Reload(ctx)

	// A handle fragment.
	shell.State().AuthorQuery = "ada"
	if got := len(shell.visibleAuthors()); got != 1 {
		t.Errorf("searching \"ada\" matched %d authors", got)
	}
	// Case-insensitively.
	shell.State().AuthorQuery = "ADA"
	if got := len(shell.visibleAuthors()); got != 1 {
		t.Errorf("searching \"ADA\" matched %d authors", got)
	}
	// The display name, which the seed sets to the same value here.
	shell.State().AuthorQuery = "grace"
	if got := len(shell.visibleAuthors()); got != 1 {
		t.Errorf("searching \"grace\" matched %d authors", got)
	}
	// Nothing matches.
	shell.State().AuthorQuery = "zzz"
	if got := len(shell.visibleAuthors()); got != 0 {
		t.Errorf("a miss matched %d authors", got)
	}
	// An empty search shows the whole list.
	shell.State().AuthorQuery = ""
	if got := len(shell.visibleAuthors()); got != 2 {
		t.Errorf("an empty search showed %d authors", got)
	}
}

// TestSavedViewKeepsTheAuthor covers the field that used to be dropped on apply.
func TestSavedViewKeepsTheAuthor(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()

	seed(t, db, "a", "ada", "post a", clockBase)
	seed(t, db, "b", "grace", "post b", clockBase)
	shell.Reload(ctx)

	shell.toggleAuthorFilter(ctx, "ada")
	shell.State().SavedViewName = "Ada"
	shell.SaveCurrentView(ctx)

	stored, err := sqlite.NewSavedViewRepository(db).List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(stored) != 1 || len(stored[0].ActiveAuthorHandles) != 1 || stored[0].ActiveAuthorHandles[0] != "ada" {
		t.Fatalf("the view stored %v", stored)
	}

	// Applying restores it, which is the part that used to be discarded.
	shell.selectSidebar(ctx, selectionAll)
	shell.clearConditions(ctx)
	shell.ApplySavedView(ctx, stored[0])
	if got := shell.State().ActiveAuthors; len(got) != 1 || got[0] != "ada" {
		t.Errorf("applying restored %v", got)
	}
	if got := len(shell.State().Data.Bookmarks); got != 1 {
		t.Errorf("after applying, %d bookmarks show, want 1", got)
	}
}

// TestReportsAreShown covers two strings that were computed and thrown away.
//
// After an import or a vocabulary review the shell built a report, stored it, and no
// widget read the field: the user performed the operation and was told nothing. A sweep
// for fields the interface never reads found both.
func TestReportsAreShown(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(context.Background())

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	// Nothing has happened yet, so neither report is on screen.
	if tester.HasText("imported") {
		t.Errorf("a bundle report appeared before any import; texts: %q", tester.Texts())
	}

	shell.State().LastBundleReport = "imported 12 bookmarks"
	shell.State().ReviewApplied = "applied 3 suggestions"
	tester.Frame()

	if !tester.HasText("imported 12 bookmarks") {
		t.Errorf("the bundle report is not shown; texts: %q", tester.Texts())
	}
	if !tester.HasText("applied 3 suggestions") {
		t.Errorf("the review report is not shown; texts: %q", tester.Texts())
	}
}

// seedTagOnly stores a tag with no bookmarks.
func seedTagOnly(t *testing.T, db *sqlite.DB, id, name string) {
	t.Helper()
	tag := domain.Tag{
		ID: id, Name: name, CanonicalKey: domain.CanonicalTagKey(name),
		CreatedAt: domain.FormatTimestamp(clockBase), UpdatedAt: domain.FormatTimestamp(clockBase),
	}
	if err := sqlite.NewTagRepository(db).Create(context.Background(), tag); err != nil {
		t.Fatalf("Create tag: %v", err)
	}
}

// TestDeletingATagAsksFirst covers the danger-action rule for tags.
func TestDeletingATagAsksFirst(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seedTagOnly(t, db, "t1", "rust")
	shell.ReloadTagLibrary(ctx)

	shell.State().TagFocus = "t1"
	shell.AskDeleteSidebarTag("t1")

	// Asking is not deleting.
	if _, found, _ := sqlite.NewTagRepository(db).Get(ctx, "t1"); !found {
		t.Fatal("asking deleted the tag")
	}
	if shell.State().ConfirmDeleteTag != "t1" {
		t.Errorf("the confirmation is open for %q", shell.State().ConfirmDeleteTag)
	}

	shell.ConfirmDeleteTagNow(ctx)

	if _, found, _ := sqlite.NewTagRepository(db).Get(ctx, "t1"); found {
		t.Error("confirming did not delete the tag")
	}
	if shell.State().ConfirmDeleteTag != "" {
		t.Error("the confirmation was left open")
	}
	if shell.State().TagFocus != "" {
		t.Errorf("the focus still names the deleted tag: %q", shell.State().TagFocus)
	}
}

// TestDeletingATagWithoutAFocusDoesNothing keeps an unset focus inert.
func TestDeletingATagWithoutAFocusDoesNothing(t *testing.T) {
	shell, _ := newShell(t)
	shell.AskDeleteSidebarTag("")
	if shell.State().ConfirmDeleteTag != "" {
		t.Errorf("no focus opened a confirmation for %q", shell.State().ConfirmDeleteTag)
	}
}

// TestDeletingTheSelectedTags covers the source's bulk action.
func TestDeletingTheSelectedTags(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	for _, spec := range []struct{ id, name string }{
		{"t1", "rust"}, {"t2", "async"}, {"t3", "cooking"},
	} {
		seedTagOnly(t, db, spec.id, spec.name)
	}
	shell.ReloadTagLibrary(ctx)

	// Nothing selected: the action does not even ask.
	shell.AskDeleteSelectedTags()
	if shell.State().ConfirmDeleteTags {
		t.Error("an empty selection opened a confirmation")
	}

	// t3 was checked and then unchecked: it keeps an entry so the row's box can be
	// rechecked, but it is not selected and must not be deleted.
	shell.State().TagSelectedIDs = map[string]bool{"t1": true, "t2": true, "t3": false}
	shell.AskDeleteSelectedTags()
	if !shell.State().ConfirmDeleteTags {
		t.Fatal("the confirmation did not open")
	}
	if shell.SelectedTagCount() != 2 {
		t.Errorf("the count is %d, want 2", shell.SelectedTagCount())
	}

	// Asking alone changes nothing.
	if _, found, _ := sqlite.NewTagRepository(db).Get(ctx, "t1"); !found {
		t.Fatal("asking deleted a tag")
	}

	shell.ConfirmDeleteSelectedTagsNow(ctx)

	for _, id := range []string{"t1", "t2"} {
		if _, found, _ := sqlite.NewTagRepository(db).Get(ctx, id); found {
			t.Errorf("%s survived the bulk delete", id)
		}
	}
	// The one that was not selected is untouched.
	if _, found, _ := sqlite.NewTagRepository(db).Get(ctx, "t3"); !found {
		t.Error("the bulk delete removed an unselected tag")
	}
	if len(shell.State().TagSelectedIDs) != 0 {
		t.Errorf("the selection was left holding %v", shell.State().TagSelectedIDs)
	}
	if shell.State().Err != "" {
		t.Errorf("a clean bulk delete reported %q", shell.State().Err)
	}
}

// TestDeletingATagRejectsItsPendingSuggestions is the behaviour the service exists to
// provide: a suggestion for a tag that no longer exists would never resolve.
func TestDeletingATagRejectsItsPendingSuggestions(t *testing.T) {
	shell, db := newBatchShell(t)
	ctx := context.Background()

	seed(t, db, "b1", "ada", "a post", clockBase)
	seedTagOnly(t, db, "t1", "rust")
	if _, err := sqlite.NewSuggestionRepository(db).InsertMany(ctx, []domain.TagSuggestion{{
		ID: "b1::rust", BookmarkID: "b1", TagName: "rust", Source: domain.SourceLLM,
		Confidence: 0.9, Status: domain.StatusPending, CreatedAt: clockBase,
	}}); err != nil {
		t.Fatalf("InsertMany: %v", err)
	}
	shell.ReloadTagLibrary(ctx)

	if err := shell.service.DeleteTag(ctx, "t1"); err != nil {
		t.Fatalf("DeleteTag: %v", err)
	}

	pending, err := sqlite.NewSuggestionRepository(db).List(ctx, domain.StatusPending)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(pending) != 0 {
		t.Errorf("%d suggestions are still pending for a deleted tag", len(pending))
	}
}

// TestCreatingATagInline covers the source's own control: a "+" that reveals a row in
// the sidebar, rather than a dialog.
func TestCreatingATagInline(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()

	if shell.State().CreatingTag {
		t.Error("the draft row is showing before it was asked for")
	}

	shell.BeginCreateTag()
	if !shell.State().CreatingTag {
		t.Fatal("the draft row did not open")
	}

	// A blank draft creates nothing, including one that is only spaces, and closes the row:
	// the source's Enter on an empty field resets the draft row (WorkspaceSidebar.tsx:265-268).
	for _, blank := range []string{"", "   "} {
		shell.BeginCreateTag()
		shell.State().TagDraft = blank
		shell.CreateTagFromDraft(ctx)
		if shell.State().CreatingTag {
			t.Errorf("a blank draft (%q) left the row open", blank)
		}
	}
	tags, err := sqlite.NewTagRepository(db).List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(tags) != 0 {
		t.Fatalf("a blank draft created %d tags", len(tags))
	}

	// A real name lands, the row closes, and the new tag becomes the focus.
	shell.State().TagDraft = "rust"
	shell.CreateTagFromDraft(ctx)

	if shell.State().CreatingTag {
		t.Error("the row stayed open after creating")
	}
	if shell.State().TagDraft != "" {
		t.Errorf("the draft still holds %q", shell.State().TagDraft)
	}
	if shell.State().Err != "" {
		t.Errorf("creating reported %q", shell.State().Err)
	}

	tags, err = sqlite.NewTagRepository(db).List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(tags) != 1 || tags[0].Name != "rust" {
		t.Fatalf("the store holds %+v", tags)
	}
	if shell.State().TagFocus != tags[0].ID {
		t.Errorf("the focus is %q, want the new tag", shell.State().TagFocus)
	}
}

// TestCreatingAnExistingTagReusesIt covers the service's find-or-create.
//
// Pressing submit twice must not make two tags, which is why the row is safe to use.
func TestCreatingAnExistingTagReusesIt(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		shell.BeginCreateTag()
		shell.State().TagDraft = "rust"
		shell.CreateTagFromDraft(ctx)
	}

	tags, err := sqlite.NewTagRepository(db).List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(tags) != 1 {
		t.Errorf("submitting the same name twice made %d tags", len(tags))
	}
	if shell.State().Err != "" {
		t.Errorf("reusing a tag reported %q", shell.State().Err)
	}
}

// TestCancellingTheDraftRow clears it without creating anything.
func TestCancellingTheDraftRow(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()

	shell.BeginCreateTag()
	shell.State().TagDraft = "half typed"
	shell.CancelCreateTag()

	if shell.State().CreatingTag || shell.State().TagDraft != "" {
		t.Errorf("cancel left creating=%v draft=%q", shell.State().CreatingTag, shell.State().TagDraft)
	}
	tags, _ := sqlite.NewTagRepository(db).List(ctx)
	if len(tags) != 0 {
		t.Errorf("cancel created %d tags", len(tags))
	}
}

// TestASyncIsIncrementalOnlyAfterAFullOneFinished is the defect the stored flag exists to
// prevent, and which nothing read.
//
// A first sync that was interrupted after storing some bookmarks has bookmarks and has
// still never walked the timeline. Deciding from the count made the next run incremental,
// so it stopped early and the rest of the history was never fetched.
func TestASyncIsIncrementalOnlyAfterAFullOneFinished(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()

	// An interrupted first sync: bookmarks exist, the flag does not.
	seed(t, db, "partial", "ada", "one page of the timeline", clockBase)
	shell.Reload(ctx)
	if shell.State().Data.Counts.TotalBookmarks == 0 {
		t.Fatal("the fixture stored no bookmarks")
	}

	shell.UseSyncer(newSyncService(db))
	if shell.syncIsIncremental(ctx) {
		t.Error("an unfinished first sync must not stop early: the rest of the history is still out there")
	}

	// Once a run genuinely reaches the end of the timeline, later runs may stop early.
	if err := sqlite.NewSettingsRepository(db).Set(ctx, "sync.hasCompletedInitialFullSync", "true", clockBase); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if !shell.syncIsIncremental(ctx) {
		t.Error("a finished first sync must let later runs stop early")
	}
}

// TestAnUnreadableFlagDoesNotStopEarly keeps a storage failure on the safe side.
func TestAnUnreadableFlagDoesNotStopEarly(t *testing.T) {
	shell, _ := newShell(t)

	// No syncer at all: nothing is known, so nothing stops early.
	if shell.syncIsIncremental(context.Background()) {
		t.Error("without a syncer the safe answer is a full walk")
	}
}

// listIDNamed finds a list by its name.
//
// Selecting by index is wrong here: the Inbox is a list too and sorts first, so
// lists[0] is not the one the test just made.
func listIDNamed(t *testing.T, db *sqlite.DB, name string) string {
	t.Helper()
	lists, err := sqlite.NewListRepository(db).List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, list := range lists {
		if list.Name == name {
			return list.ID
		}
	}
	t.Fatalf("no list named %q among %d", name, len(lists))
	return ""
}

// TestTheInboxCannotBeRenamed covers the service's rule and the missing control.
func TestTheInboxCannotBeRenamed(t *testing.T) {
	shell, db := newListShell(t)
	ctx := context.Background()

	// The service refuses it even when asked directly, which is what makes the absent
	// edit button a courtesy rather than the only guard.
	shell.BeginRenameList(ctx, domain.InboxListID, "Inbox")
	shell.State().RenameListName = "Everything"
	shell.RenameListFromInput(ctx)

	if shell.ToastText() == "" {
		t.Error("renaming the Inbox must be refused")
	}
	if _, found, _ := sqlite.NewListRepository(db).Get(ctx, domain.InboxListID); !found {
		t.Error("the Inbox is gone")
	}
}

// TestRenamingAList covers the inline rename.
//
// Lists are not in src/ -- the extension organises by tag and by saved view -- so this
// finishes an invented subsystem rather than replicating one. The rules it relies on are
// the service's: a duplicate name is refused and the Inbox cannot be renamed.
func TestRenamingAList(t *testing.T) {
	shell, db := newListShell(t)
	ctx := context.Background()

	shell.State().NewListName = "Research"
	shell.CreateListFromInput(ctx)
	shell.State().NewListName = "Reading"
	shell.CreateListFromInput(ctx)

	var researchID string
	lists, err := sqlite.NewListRepository(db).List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, list := range lists {
		if list.Name == "Research" {
			researchID = list.ID
		}
	}
	if researchID == "" {
		t.Fatal("the list was not created")
	}

	// Opening the row prefills the current name.
	shell.BeginRenameList(ctx, researchID, "Research")
	if shell.State().RenameListID != researchID || shell.State().RenameListName != "Research" {
		t.Fatalf("the row opened as %q for %q",
			shell.State().RenameListName, shell.State().RenameListID)
	}

	// An existing name is refused, and the row stays open so it can be corrected.
	shell.State().RenameListName = "Reading"
	shell.RenameListFromInput(ctx)
	if shell.ToastText() == "" {
		t.Error("a duplicate name must be refused")
	}
	if shell.State().RenameListID == "" {
		t.Error("the refusal closed the row")
	}

	// A real rename lands and closes it.
	shell.State().RenameListName = "Deep research"
	shell.RenameListFromInput(ctx)

	if shell.State().Err != "" {
		t.Errorf("renaming reported %q", shell.State().Err)
	}
	if shell.State().RenameListID != "" || shell.State().RenameListName != "" {
		t.Error("the row stayed open after renaming")
	}
	list, found, err := sqlite.NewListRepository(db).Get(ctx, researchID)
	if err != nil || !found {
		t.Fatalf("Get: %v found=%v", err, found)
	}
	if list.Name != "Deep research" {
		t.Errorf("the list is named %q", list.Name)
	}
}

// TestRenamingAListToNothingDoesNothing keeps a blank save inert.
func TestRenamingAListToNothingDoesNothing(t *testing.T) {
	shell, db := newListShell(t)
	ctx := context.Background()

	shell.State().NewListName = "Research"
	shell.CreateListFromInput(ctx)

	// By name, not by index: the Inbox is in this list too and sorts first.
	id := listIDNamed(t, db, "Research")

	shell.BeginRenameList(ctx, id, "Research")
	for _, blank := range []string{"", "   "} {
		shell.State().RenameListName = blank
		shell.RenameListFromInput(ctx)
	}

	list, _, _ := sqlite.NewListRepository(db).Get(ctx, id)
	if list.Name != "Research" {
		t.Errorf("a blank rename changed the name to %q", list.Name)
	}
}

// TestCancellingARenameLeavesTheList alone.
func TestCancellingARenameLeavesTheList(t *testing.T) {
	shell, db := newListShell(t)
	ctx := context.Background()

	shell.State().NewListName = "Research"
	shell.CreateListFromInput(ctx)
	id := listIDNamed(t, db, "Research")

	shell.BeginRenameList(ctx, id, "Research")
	shell.State().RenameListName = "Something else"
	shell.CancelRenameList()

	if shell.State().RenameListID != "" || shell.State().RenameListName != "" {
		t.Error("cancel left the row open")
	}
	list, _, _ := sqlite.NewListRepository(db).Get(ctx, id)
	if list.Name != "Research" {
		t.Errorf("cancel renamed the list to %q", list.Name)
	}
}

// TestTheSidebarDropsTheListsAndSavedViews records the round-472 decision.
//
// Lists and saved views are implemented -- domain, storage, use cases and their tests all stay --
// but the reference picture's sidebar holds only 标签 and 作者, and the source has no such sections
// at all (§22: `grep -rni "inbox" src/` is zero, `ENABLE_SAVED_VIEWS_UI` is false, and the source's
// own test asserts Inbox never renders). So the sidebar stopped drawing them.
//
// This replaces TestEveryListIsDrawn, which asserted the opposite. Deleting that test without a
// replacement would leave the removal invisible: a later round could put the sections back and
// nothing would say whether that was intended. Asserting absence makes the choice checkable.
func TestTheSidebarDropsTheListsAndSavedViews(t *testing.T) {
	shell, _ := newListShell(t)
	shell.Reload(context.Background())

	// A list and a saved view, set directly: the data layer keeps both (round 472 removed only the
	// sidebar's entry to them), so what this checks is that the sidebar draws neither.
	shell.state.Data.Lists = []domain.ListSummary{
		{List: domain.List{ID: "list-a", Name: "List A"}, BookmarkCount: 1},
	}

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	texts := tester.Texts()
	for _, gone := range []string{
		"List A",                              // a list row
		shell.t().T(i18n.SidebarLists),        // the section heading
		shell.t().T(i18n.SavedViewsTitle),     // the saved views heading
		shell.t().T(i18n.SidebarInbox),        // the inbox row
		shell.t().T(i18n.SidebarUnclassified), // the unclassified row
	} {
		for _, text := range texts {
			if text == gone {
				t.Errorf("the sidebar draws %q, which round 472 removed to match the reference", gone)
			}
		}
	}

	// And the two facets it does keep are still there, so this is a removal and not a blackout.
	// They are the sidebar's tab labels now rather than section overlines, so they are drawn in
	// the catalog's own case: a segmented control is a control, not a heading.
	for _, kept := range []string{
		shell.t().T(i18n.SidebarTags),
		shell.t().T(i18n.AuthorsTitle),
	} {
		if !tester.HasText(kept) {
			t.Errorf("the sidebar lost %q as well; texts: %q", kept, texts)
		}
	}
}

func TestEmptyStatesUseTheSourcesWording(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()

	// An empty library: the title and the advice under it. Reload marks the shell loaded,
	// which is what makes it show the empty state rather than the spinner.
	shell.Reload(ctx)
	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	if !tester.HasText("No bookmarks saved yet") {
		t.Errorf("the empty library is not the source's wording; texts: %q", tester.Texts())
	}
	if !tester.HasText("Once you save items from X") {
		t.Errorf("the advice under it is missing; texts: %q", tester.Texts())
	}

	// A filter that matches nothing: the other message, and no advice.
	seed(t, db, "1", "ada", "a post about rust", clockBase)
	shell.Reload(ctx)
	shell.State().Query = "nothing matches this"
	shell.Reload(ctx)
	tester.Frame()

	if !tester.HasText("No bookmarks match the current filters") {
		t.Errorf("a filter that matched nothing shows the wrong message; texts: %q", tester.Texts())
	}
	if tester.HasText("Once you save items from X") {
		t.Error("a filter that matched nothing must not advise saving items")
	}
	if !tester.HasText("Try adjusting your search, tags, or author scope.") {
		t.Errorf("a filter that matched nothing must say what to try; texts: %q", tester.Texts())
	}
}

// TestTheDetailPanelIsTitledDetails is the source's word for it.
// TestTheDetailHeroShowsTheAuthorRatherThanATitle covers what the panel's header actually is.
//
// This test used to require the visible word "Details", which this shell had invented. The source
// gives detailsTitle to the close button as its aria-label (BookmarkInspector.tsx:180) and draws
// the avatar, the name and the handle in the hero -- and the reference picture's header is
// 硅谷居士 @SVScholar with 在 X 中打开 on the right. There is no visible title to find.
func TestTheDetailHeroShowsTheAuthorRatherThanATitle(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(ctx)
	shell.State().Selected = "1"
	shell.OpenSelectedDetail()

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	if !tester.HasText("@ada") {
		t.Errorf("the hero is missing the author's handle; texts: %q", tester.Texts())
	}
	// The invented title is gone, and asserted gone so it cannot come back unnoticed.
	//
	// The word itself does exist: the close control's accessible name is the source's
	// detailsTitle (BookmarkInspector.tsx:174-177), which is not a drawn title. A title sat
	// at the hero's left, where the author's line starts; the close control sits at the
	// card's right edge, so that is where the word is allowed to be.
	for _, word := range []string{"Details", "详情"} {
		r, found := tester.Find(word)
		if found && r.X < 1180/2 {
			t.Errorf("the hero draws a title (%q at x=%.0f), which the source does not", word, r.X)
		}
	}
}

// TestAnEmptyGraphExplainsItself covers the source's graphEmptyTitle and
// graphEmptyDescription.
//
// This shell drew an empty canvas: the graph panel had no empty state at all.
func TestAnEmptyGraphExplainsItself(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post with no tags", clockBase)
	shell.Reload(ctx)

	shell.ToggleGraph(ctx)
	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	if !tester.HasText("No relations to show yet") {
		t.Errorf("an empty graph says nothing; texts: %q", tester.Texts())
	}
	if !tester.HasText("Tag a few bookmarks, or widen the current filters.") {
		t.Errorf("an empty graph gives no advice; texts: %q", tester.Texts())
	}
}

// TestTogglingGraphAuthors covers the source's opt-in author layer.
func TestTogglingGraphAuthors(t *testing.T) {
	shell, _ := newShell(t)
	ctx := context.Background()

	if shell.State().GraphIncludeAuthors {
		t.Error("author nodes start on")
	}
	shell.ToggleGraphAuthors(ctx)
	if !shell.State().GraphIncludeAuthors {
		t.Error("the toggle did not take")
	}
	shell.ToggleGraphAuthors(ctx)
	if shell.State().GraphIncludeAuthors {
		t.Error("the toggle did not come back")
	}
}

// TestClearingTheGraphFocusWidensTheList covers the divergence, stated rather than hidden.
//
// The source keeps a graph-local focus and leaves the feed alone. This shell's graph focus
// is the selection, so clearing it also widens the list -- which is the behaviour this
// asserts, so that a change to it is a change to a test.
func TestClearingTheGraphFocusWidensTheList(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()

	seedTagged(t, db, "tagged", "rust")
	shell.Reload(ctx)
	shell.toggleTagFilter(ctx, "rust")
	if shell.focusedGraphTag() == "" {
		t.Fatal("the graph has no focus after narrowing by a tag")
	}

	shell.ClearGraphFocus(ctx)
	if shell.focusedGraphTag() != "" {
		t.Errorf("the focus survived: %q", shell.focusedGraphTag())
	}
	if shell.State().Selection != selectionAll {
		t.Errorf("the selection is %q", shell.State().Selection)
	}
}

// TestResettingLocalDataAsksFirst covers the danger action that had no entry at all.
//
// The source offers "Reset local data" beside "Import restore", behind a confirmation.
// Here the cascade existed only as the first half of a restore and nothing could reach
// it. The confirmation is the only way in: the source's own check is
// `typeof window.confirm === "function" ? window.confirm(...) : true`, which proceeds
// when the dialog is unavailable, and goal clause 8 requires a danger action to fail
// closed instead.
func TestResettingLocalDataAsksFirst(t *testing.T) {
	// The bundle service has to be attached: the reset lives there, and a plain batch
	// shell does not carry it.
	shell, db := newBatchShell(t)
	shell.UseImportExport(importexport.New(ports.Repositories{
		Bookmarks:   sqlite.NewBookmarkRepository(db),
		Tags:        sqlite.NewTagRepository(db),
		Lists:       sqlite.NewListRepository(db),
		Suggestions: sqlite.NewSuggestionRepository(db),
		Analysis:    sqlite.NewAnalysisRepository(db),
		Settings:    sqlite.NewSettingsRepository(db),
		SavedViews:  sqlite.NewSavedViewRepository(db),
		SyncRuns:    sqlite.NewSyncRunRepository(db),
	}, fixedClock{now: clockBase}, "0.3.0"))
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post", clockBase)
	seedTagged(t, db, "2", "rust")
	shell.Reload(ctx)

	shell.AskResetLocalData()
	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	// Asking shows a dialog and changes nothing.
	if !tester.HasText(shell.t().T(i18n.ResetLocalDataConfirm)) {
		t.Errorf("the reset confirmation is not on screen; texts: %q", tester.Texts())
	}
	if _, found, _ := sqlite.NewBookmarkRepository(db).Get(ctx, "1"); !found {
		t.Fatal("asking emptied the library")
	}

	shell.ConfirmResetLocalDataNow(ctx)

	if _, found, _ := sqlite.NewBookmarkRepository(db).Get(ctx, "1"); found {
		t.Error("confirming did not empty the library")
	}
	if tags, _ := sqlite.NewTagRepository(db).List(ctx); len(tags) != 0 {
		t.Errorf("%d tags survived the reset", len(tags))
	}
	if shell.State().ConfirmResetData {
		t.Error("the dialog was left open")
	}
}

// TestDismissingTheResetDialogChangesNothing is the fail-closed half.
func TestDismissingTheResetDialogChangesNothing(t *testing.T) {
	shell, db := newBatchShell(t)
	shell.UseImportExport(importexport.New(ports.Repositories{
		Bookmarks: sqlite.NewBookmarkRepository(db),
		Tags:      sqlite.NewTagRepository(db),
		Settings:  sqlite.NewSettingsRepository(db),
	}, fixedClock{now: clockBase}, "0.3.0"))
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(ctx)

	shell.AskResetLocalData()
	// Dismissed without choosing, which is what the dialog does to its own flag.
	shell.State().ConfirmResetData = false

	if _, found, _ := sqlite.NewBookmarkRepository(db).Get(ctx, "1"); !found {
		t.Error("dismissing the dialog emptied the library")
	}
}

// TestTheFilterPopover covers the toolbar's filter control.
//
// The source opens a popover with a heading, an explanation and four conditions, two of
// which it renders disabled. This shell had no popover at all: the two working filters
// were only reachable by finding the right combination elsewhere.
func TestTheFilterPopover(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(ctx)

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	// Closed: none of the popover's wording is on screen.
	if tester.HasText(shell.t().T(i18n.FilterConditions)) {
		t.Fatalf("the popover is open before it was asked for; texts: %q", tester.Texts())
	}
	// The trigger is there with its name, and the name is on screen: round 454 made it an
	// .options-toolbar-action, an icon plus a word, as the reference has it.
	//
	// This used to grep the package source for `IconOnly(c, "filter", ...)`. That asserted the
	// implementation rather than the result: it would have passed with the trigger drawn off
	// screen, and it failed the moment the control became a labelled button -- which is a
	// change in the right direction. Asking the rendered frame is the stronger check.
	if !tester.HasText(shell.t().T(i18n.ToolbarFilters)) {
		t.Errorf("the filter trigger has no visible name; texts: %q", tester.Texts())
	}

	shell.ToggleFilterPopover()
	tester.Frame()

	// Opening starts `.pop`'s entrance clock; reopening restarts it, so a panel closed and
	// opened again is not already at rest.
	openedAt := shell.state.FilterPopoverOpenedAt
	if openedAt.IsZero() {
		t.Error("opening the popover did not start its entrance")
	}
	shell.ToggleFilterPopover()
	time.Sleep(time.Millisecond)
	shell.ToggleFilterPopover()
	if !shell.state.FilterPopoverOpenedAt.After(openedAt) {
		t.Error("reopening the popover kept the old entrance clock")
	}

	for _, key := range []i18n.Key{
		i18n.FilterConditions, i18n.FilterDescription,
		i18n.FilterHasMedia, i18n.FilterLongform, i18n.FilterUnread, i18n.FilterArchived,
	} {
		if !tester.HasText(shell.t().T(key)) {
			t.Errorf("the popover is missing %q; texts: %q", shell.t().T(key), tester.Texts())
		}
	}

	// Toggling closes it again, which is what the trigger does.
	shell.ToggleFilterPopover()
	if shell.State().FilterPopoverOpen {
		t.Error("the popover did not close")
	}
}

// TestTheFilterPopoverTakesMagpiePanelShape pins the panel's geometry.
//
// magpie's `.pop` is a card with an 11-point radius (app.css:2270-2280) and the filter
// variant is `.pop.proto-menu`: 262 wide with a 4-point inset (app.css:3024). The shell used
// to take ui.Popover's defaults instead -- theme.Radius+2 and a min width of the trigger --
// which made the panel as narrow as the "Filters" button and almost square-cornered.
func TestTheFilterPopoverTakesMagpiePanelShape(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(ctx)

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	shell.ToggleFilterPopover()
	tester.Frame()
	// Bounds are the previous frame's, so the panel's rectangle is readable one frame after
	// it first painted.
	tester.Frame()

	box, found := tester.Find(shell.t().T(i18n.FilterConditions))
	if !found {
		t.Fatalf("no element carries the popover's label; texts: %q", tester.Texts())
	}
	if got := int(box.W + 0.5); got != FilterPopoverWidth {
		t.Errorf("the popover is %d wide, want %d", got, FilterPopoverWidth)
	}

	// The radius, read as pixels at the top-right corner: 3 points in and 3 down lies
	// outside an 11-point arc, 11 points down the same edge lies inside it. A square corner
	// -- or the old theme.Radius+2 -- would paint the card colour at both.
	img := tester.Image()
	corner := img.RGBAAt(int(box.X+box.W-3), int(box.Y+3))
	panel := img.RGBAAt(int(box.X+box.W-3), int(box.Y+14))
	if corner == panel {
		t.Errorf("the panel's corner is square: both samples are %v", corner)
	}
	if panel.R < corner.R {
		t.Errorf("the panel is darker than the page at its edge: panel %d, page %d", panel.R, corner.R)
	}
}

// TestTheBadgeCountsTheActiveFilters covers the number beside the trigger.
func TestTheBadgeCountsTheActiveFilters(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(ctx)

	if got := len(shell.activeConditions()); got != 0 {
		t.Fatalf("a fresh shell reports %d filters", got)
	}
	shell.State().OnlyWithMedia = true
	if got := len(shell.activeConditions()); got != 1 {
		t.Errorf("one filter reports %d", got)
	}
	shell.State().OnlyLongform = true
	if got := len(shell.activeConditions()); got != 2 {
		t.Errorf("two filters report %d", got)
	}
}

// TestCleaningUpTagsAsksFirst covers a destructive action that used to run on the click.
//
// The button said "Clean up empty tags" and swept immediately. The source labels the same
// action "Delete tags without bookmarks" and confirms it: a euphemism on a destructive
// action is the pair of faults clause 8 exists to prevent.
func TestCleaningUpTagsAsksFirst(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seedTagOnly(t, db, "orphan", "unused")
	shell.ReloadTagLibrary(ctx)

	shell.AskCleanupOrphanTags()
	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	if !tester.HasText(shell.t().T(i18n.TagCleanupConfirm)) {
		t.Errorf("the confirmation is not on screen; texts: %q", tester.Texts())
	}
	// Asking is not deleting.
	if _, found, _ := sqlite.NewTagRepository(db).Get(ctx, "orphan"); !found {
		t.Fatal("asking deleted the tag")
	}

	// The dialog's own path runs the sweep.
	shell.State().ConfirmCleanupTags = false
	shell.CleanupOrphanTags(ctx)
	if _, found, _ := sqlite.NewTagRepository(db).Get(ctx, "orphan"); found {
		t.Error("the sweep did not remove the orphan")
	}
}

// TestTheCleanupLabelNamesTheConsequence pins the wording.
func TestTheCleanupLabelNamesTheConsequence(t *testing.T) {
	shell, _ := newShell(t)
	got := shell.t().T(i18n.TagCleanupOrphans)
	if got != "Delete tags without bookmarks" {
		t.Errorf("the label is %q, want the source's wording", got)
	}
	if shell.t().T(i18n.SuggestionsAccept) != "Accept suggestion" {
		t.Errorf("the accept label is %q", shell.t().T(i18n.SuggestionsAccept))
	}
}

// TestTheDetailShowsWhenThePostWasPublished covers the one date the detail keeps.
//
// The source's timeline shows both the published and the saved date; this build showed only the
// saved one, so CreatedAtOnX was stored and never drawn. Round 537 then dropped the saved and
// last-sync dates at the reader's request -- neither is a fact anyone acts on -- and left the
// published date as a clock and the date on the author's line (S43).
func TestTheDetailShowsWhenThePostWasPublished(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()

	bookmark := domain.Bookmark{
		TweetID: "dated", TweetURL: "https://x.com/ada/status/dated",
		AuthorName: "ada", AuthorHandle: "ada", Text: "a dated post",
		CreatedAtOnX: "Mar 3, 2024",
		SavedAt:      clockBase, LastSeenAt: clockBase, UpdatedAt: clockBase,
	}
	if _, err := sqlite.NewBookmarkRepository(db).UpsertMany(ctx, []domain.Bookmark{bookmark}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}
	shell.Reload(ctx)
	shell.State().Selected = "dated"
	shell.OpenSelectedDetail()

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	if !tester.HasText("Mar 3, 2024") {
		t.Errorf("the published date is missing; texts: %q", tester.Texts())
	}
	// The tooltip is the label now: the date itself is bare, and an icon with no name is a
	// decoration. It is looked up rather than drawn, which is why it is asserted through the
	// catalog instead of through Texts.
	if shell.t().T(i18n.InspectorPublished) == "" {
		t.Error("the published date has no label to explain its clock")
	}

	// A bookmark whose source gave no date draws no date at all -- not the "never synced"
	// wording FormatSavedAt returns for a zero time, which says something about this machine
	// and nothing about when the author posted.
	seed(t, db, "undated", "grace", "no date from the source", clockBase)
	shell.Reload(ctx)
	shell.State().Selected = "undated"
	shell.OpenSelectedDetail()
	tester.Frame()
	if shell.publishedLabel(shell.state.Data.Bookmarks[len(shell.state.Data.Bookmarks)-1]) != "" {
		t.Error("an undated post produced a date")
	}
}

// TestTheSettingsDataSection covers the section the source has and this shell lacked.
//
// Every action was already reachable from the application menu, so what was missing was
// the place: the source's hint says where backups go, and the three buttons sit together.
// Wiring the labels also cleared five catalog keys that nothing rendered.
func TestTheSettingsDataSection(t *testing.T) {
	shell, _ := newShell(t)
	ctx := context.Background()

	shell.OpenSettings()
	// The sheet is tabbed now (round 474), so the section has to be chosen before its content is
	// on screen -- the source's nav works the same way and its .options-settings-body shows one
	// section at a time.
	shell.state.SettingsSection = SettingsSectionDataSafe
	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	// The section draws as magpie's cards: a --muted label names it, and each row names
	// the thing while its action carries the verb (shell-contract.md S46).
	if !tester.HasText(strings.ToUpper(shell.t().T(i18n.SettingsData))) {
		t.Errorf("the data section is missing its label; texts: %q", tester.Texts())
	}
	for _, key := range []i18n.Key{
		i18n.SettingsDataSafetyHint,
		i18n.SettingsBackupRow, i18n.ExportData,
		i18n.SettingsRestoreRow, i18n.RestoreBackup,
		i18n.SettingsLocalRow, i18n.ResetLocalData,
	} {
		if !tester.HasText(shell.t().T(key)) {
			t.Errorf("the data section is missing %q; texts: %q", shell.t().T(key), tester.Texts())
		}
	}
	_ = ctx
}

// TestTheKeywordRuleButtonNamesItsObject pins the wording.
//
// The button said "Add", which beside two text fields could mean adding either of them.
// The source calls it "Add rule".
func TestTheKeywordRuleButtonNamesItsObject(t *testing.T) {
	shell, _ := newShell(t)
	if got := shell.t().T(i18n.SettingsAddKeywordRule); got != "Add rule" {
		t.Errorf("the label is %q, want the source's wording", got)
	}
}

// TestStartingARunWhileOneIsGoingSaysSo covers a button that used to do nothing.
//
// StartAutoTag returned false when a pass was in flight, so the press produced no change
// anywhere on screen. The message says why.
func TestStartingARunWhileOneIsGoingSaysSo(t *testing.T) {
	shell, _ := newShell(t)

	// A shell with no tagging service refuses for a different reason and says nothing.
	if shell.StartAutoTag(context.Background()) {
		t.Fatal("a shell with no tagging service started a run")
	}

	shell.State().AutoTagRunning = true
	if shell.StartAutoTag(context.Background()) {
		t.Fatal("a second run started")
	}
	if got := shell.State().AutoTagStatus; got != shell.t().T(i18n.RunAlreadyRunning) {
		t.Errorf("the status is %q, want the already-running message", got)
	}
}

// TestStoppingARunSaysWhatItDid covers the control that was missing entirely.
//
// The shell showed a spinner while a pass ran and offered no way to stop it.
func TestStoppingARunSaysWhatItDid(t *testing.T) {
	shell, _ := newShell(t)

	// Nothing to stop.
	if shell.StopAutoTag() {
		t.Error("a shell with no run reported stopping one")
	}

	cancelled := false
	shell.State().autoTagCancel = func() { cancelled = true }
	if !shell.StopAutoTag() {
		t.Fatal("stopping a run reported failure")
	}
	if !cancelled {
		t.Error("the run's context was not cancelled")
	}
	if got := shell.State().AutoTagStatus; got != shell.t().T(i18n.RunCancelRequested) {
		t.Errorf("the status is %q, want the stop-requested message", got)
	}
}

// TestThePendingSuggestionsRow covers the source's sidebar filter.
//
// The row narrows the list to bookmarks a review pass is waiting on. The filter had no
// such field at all, so this was a feature gap rather than a missing label.
func TestThePendingSuggestionsRow(t *testing.T) {
	shell, db := newBatchShell(t)
	ctx := context.Background()

	seed(t, db, "waiting", "ada", "a post to review", clockBase)
	seed(t, db, "answered", "grace", "a post already reviewed", clockBase)
	if _, err := sqlite.NewSuggestionRepository(db).InsertMany(ctx, []domain.TagSuggestion{{
		ID: "waiting::rust", BookmarkID: "waiting", TagName: "rust", Source: domain.SourceLLM,
		Confidence: 0.9, Status: domain.StatusPending, CreatedAt: clockBase,
	}}); err != nil {
		t.Fatalf("InsertMany: %v", err)
	}
	shell.Reload(ctx)

	if got := len(shell.State().Data.Bookmarks); got != 2 {
		t.Fatalf("the fixture shows %d bookmarks", got)
	}

	shell.State().OnlyPendingSuggestions = true
	shell.Reload(ctx)

	if got := len(shell.State().Data.Bookmarks); got != 1 {
		t.Errorf("the row matched %d bookmarks, want the one with a pending suggestion", got)
	}
	if len(shell.State().Data.Bookmarks) == 1 && shell.State().Data.Bookmarks[0].TweetID != "waiting" {
		t.Errorf("the survivor is %q", shell.State().Data.Bookmarks[0].TweetID)
	}

	// And it appears as a removable chip, so it can be cleared from the toolbar.
	found := false
	for _, chip := range shell.activeConditions() {
		if chip.Label == shell.t().T(i18n.SuggestionsPendingTitle) {
			found = true
			chip.Remove()
		}
	}
	if !found {
		t.Error("the row did not produce a filter chip")
	}
	if shell.State().OnlyPendingSuggestions {
		t.Error("clearing the chip left the filter on")
	}
}

// TestTheTagLibrarySummary covers the counts the domain already computed.
//
// CountOrphanTags, CountUnparentedTags and CountLongTailTags all existed, and the screen
// whose whole purpose is noticing drift rendered none of them.
func TestTheTagLibrarySummary(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()

	// One tag used by a bookmark, one orphan, one unparented child.
	seedTagged(t, db, "used", "kept")
	seedTagOnly(t, db, "orphan", "unused")
	child := domain.Tag{
		ID: "child", Name: "child", CanonicalKey: domain.CanonicalTagKey("child"),
		ParentID: "kept", CreatedAt: domain.FormatTimestamp(clockBase),
		UpdatedAt: domain.FormatTimestamp(clockBase),
	}
	if err := sqlite.NewTagRepository(db).Create(ctx, child); err != nil {
		t.Fatalf("Create: %v", err)
	}
	shell.ReloadTagLibrary(ctx)
	shell.ToggleTagMode(ctx)

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	// The visible count, the orphan count and their nouns.
	if !tester.HasText("3 " + shell.t().T(i18n.TagTagsSuffix)) {
		t.Errorf("the tag count is missing; texts: %q", tester.Texts())
	}
	// Two, not one: the child tag has no bookmark either, so both are orphans.
	if !tester.HasText("2 " + shell.t().T(i18n.TagOrphanSuffix)) {
		t.Errorf("the orphan count is missing; texts: %q", tester.Texts())
	}
	if !tester.HasText("1 " + shell.t().T(i18n.TagUnparentedSuffix)) {
		t.Errorf("the unparented count is missing; texts: %q", tester.Texts())
	}
}

// TestTheTagRowShowsItsChildCount covers another field that was counted and not shown.
//
// ChildCount was already on TagLibraryItem and the row rendered only the bookmark count,
// so a theme with children looked the same as one without.
func TestTheTagRowShowsItsChildCount(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()

	seedTagOnly(t, db, "theme", "compilers")
	// The parent is part of the tag, so the child is created with it rather than reparented.
	childTag := domain.Tag{
		ID: "child", Name: "rust", CanonicalKey: domain.CanonicalTagKey("rust"),
		ParentID: "theme", CreatedAt: domain.FormatTimestamp(clockBase),
		UpdatedAt: domain.FormatTimestamp(clockBase),
	}
	if err := sqlite.NewTagRepository(db).Create(ctx, childTag); err != nil {
		t.Fatalf("Create: %v", err)
	}
	shell.ReloadTagLibrary(ctx)
	shell.ToggleTagMode(ctx)

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	if !tester.HasText("1 " + shell.t().T(i18n.TagChildCountSuffix)) {
		t.Errorf("the child count is not shown; texts: %q", tester.Texts())
	}
}

// TestOpeningABookmarkOnX covers a primary action this shell did not have.
//
// A bookmark manager that cannot take you to the bookmark is missing the one thing the
// bookmark is for. The opener is a seam because the real call starts a browser.
func TestOpeningABookmarkOnX(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(ctx)
	shell.State().Selected = "1"

	// No opener installed: the user is told, rather than nothing happening.
	if shell.OpenSelectedOnX() {
		t.Error("opening reported success with no opener installed")
	}
	if got := shell.ToastText(); got != shell.t().T(i18n.ToastNoLink) {
		t.Errorf("the toast is %q", got)
	}

	var opened string
	shell.UseExternalOpener(func(url string) error {
		opened = url
		return nil
	})

	if !shell.OpenSelectedOnX() {
		t.Fatal("opening failed with an opener installed")
	}
	want := shell.State().Data.Bookmarks[0].TweetURL
	if opened != want {
		t.Errorf("opened %q, want %q", opened, want)
	}

	// A bookmark the source gave no URL opens nothing and says so.
	shell.State().Data.Bookmarks[0].TweetURL = ""
	opened = ""
	if shell.OpenSelectedOnX() {
		t.Error("a bookmark with no URL reported opening")
	}
	if opened != "" {
		t.Errorf("something was opened: %q", opened)
	}
}

// TestTheAutoTagResultIsDated covers the line that says when the shown result was recorded.
//
// Without it a result from last month reads the same as one from a minute ago, which is
// the difference between "nothing to do" and "nothing has run".
func TestTheAutoTagResultIsDated(t *testing.T) {
	shell, _ := newShell(t)
	// The result line lives on the auto-tagging page of tag management (shell-contract.md S26).
	shell.state.TagMode = true
	shell.state.TagView = TagViewAutoTag

	shell.State().AutoTagHeadline = "Done: 3 new suggestions, 1 awaiting review."
	shell.State().AutoTagFinishedAt = clockBase

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	// Assert the line a person can read, not whatever the call site happens to produce.
	//
	// Round 450 found this test asserting `Tf(AutoTagAt, FormatTimestamp(...))` -- that is, it
	// rendered the same expression the UI renders and checked it appeared. It therefore passed
	// while the panel showed "Recorded %!d(string=2026-10-12T12:00:00.000Z)", because the i18n
	// string carried %d where the source has {time}. A test built from the implementation cannot
	// see the implementation being wrong.
	// The date is the row's quiet value at the end of the line, so the row is named for
	// what it is and the time sits where a preference row's value sits.
	if !tester.HasText(shell.t().T(i18n.AutoTagTitle)) {
		t.Errorf("the result row is unnamed; texts: %q", tester.Texts())
	}
	want := domain.FormatSavedAt(domain.FormatTimestamp(clockBase), string(shell.t().Locale()))
	if !tester.HasText(want) {
		t.Errorf("the recorded time is missing (%q); texts: %q", want, tester.Texts())
	}
	for _, text := range tester.Texts() {
		if strings.Contains(text, "%!") {
			t.Errorf("a text carries a broken format verb: %q", text)
		}
	}

	// A shell with no recorded run shows no date.
	shell.State().AutoTagFinishedAt = time.Time{}
	tester.Frame()
	if tester.HasText(want) {
		t.Error("a shell with no run still shows a recorded time")
	}
}

// TestTheGraphFocusPanel covers the panel the source shows for a focused tag.
//
// The graph drew relations and stopped there: no counts, no authors, and no way from a
// node to the bookmarks behind it. AuthorCount and TopAuthors were already on the node.
func TestTheGraphFocusPanel(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()

	seedTagged(t, db, "a", "rust")
	seedTagged(t, db, "b", "rust")
	shell.Reload(ctx)
	shell.RebuildGraph(ctx)
	shell.ToggleGraph(ctx)

	// Focused on the tag, the panel describes it and offers the way back. The focus is the
	// tag condition, not the scope.
	shell.State().ActiveTagIDs = []string{"rust"}
	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	if !tester.HasText("rust") {
		t.Errorf("the focused tag is not named; texts: %q", tester.Texts())
	}
	if !tester.HasText(shell.t().T(i18n.GraphOpenBookmarks)) {
		t.Errorf("the way back to the list is missing; texts: %q", tester.Texts())
	}
	if !tester.HasText("2 " + shell.t().T(i18n.TagTableBookmarks)) {
		t.Errorf("the bookmark count is missing; texts: %q", tester.Texts())
	}

	// With no tag condition there is no focus, and so no panel.
	shell.State().ActiveTagIDs = nil
	tester.Frame()
	if tester.HasText(shell.t().T(i18n.GraphOpenBookmarks)) {
		t.Error("a panel appeared with nothing focused")
	}
}

// TestTheRunButtonUsesTheSourcesWords pins two labels taken from the source.
//
// The button said "Run auto-tagging" where the source says "Analyze now", and the running
// state showed a spinner with no word where the source says "Analyzing".
func TestTheRunButtonUsesTheSourcesWords(t *testing.T) {
	shell, _ := newShell(t)
	if got := shell.t().T(i18n.ActionRunNow); got != "Analyze now" {
		t.Errorf("the run button says %q, want the source's wording", got)
	}
	if got := shell.t().T(i18n.RunAnalyzing); got != "Analyzing" {
		t.Errorf("the running label says %q", got)
	}
	if shell.t().T(i18n.ActionRunAutotag) == shell.t().T(i18n.ActionRunNow) {
		t.Log("the old and new labels agree; the button was switched anyway")
	}
}

// TestTheInspectorShowsItsTags covers the section the reading pane did not have.
//
// The detail showed a summary, media and dates, and no tags: the only place to see or
// change a bookmark's tags was the list it came from.
func TestTheInspectorShowsItsTags(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()

	seed(t, db, "tagged", "ada", "a post about compilers", clockBase)
	seed(t, db, "bare", "grace", "a post with no tags", clockBase)
	seedTagged(t, db, "tagged", "rust")
	shell.Reload(ctx)

	// Tags present: they are named in the panel.
	shell.State().Selected = "tagged"
	shell.OpenSelectedDetail()
	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	if !tester.HasText("rust") {
		t.Errorf("the bookmark's tag is missing from the panel; texts: %q", tester.Texts())
	}
	// No tags: the field that adds one is the empty state -- the sentence that used to say so
	// was removed with the footer (S43).
	shell.State().Selected = "bare"
	shell.OpenSelectedDetail()
	tester.Frame()
	if _, found := tester.Find(DetailTagInputLabel); !found {
		t.Errorf("an untagged bookmark offers no way to add one; texts: %q", tester.Texts())
	}
}

// TestDetachingATagFromTheInspector covers the pill's close control.
//
// It goes through DetachTagFromSelected rather than RemoveTagFromSelection: the latter
// works on the batch, and a single selection is not in the batch, so the pill's first
// version called it and nothing happened.
func TestDetachingATagFromTheInspector(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()

	seed(t, db, "1", "ada", "a post", clockBase)
	seedTagged(t, db, "1", "rust")
	shell.Reload(ctx)
	shell.State().Selected = "1"

	shell.OpenSelectedDetail()
	shell.DetachTagFromSelected(ctx, "rust")

	row, found := shell.selectedBookmark()
	if !found {
		t.Fatal("the bookmark is gone")
	}
	if len(row.Tags) != 0 {
		t.Errorf("the tag survived detaching: %v", row.Tags)
	}
}

// TestAnEmptyTagLibrarySaysSo covers two states that were sharing one message.
//
// The screen said "nothing matches this filter" even with no tags at all, which tells a
// user to adjust a filter they never set. The source distinguishes the two.
func TestAnEmptyTagLibrarySaysSo(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	shell.ReloadTagLibrary(ctx)
	shell.ToggleTagMode(ctx)

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	if !tester.HasText(shell.t().T(i18n.TagNoTagsCreated)) {
		t.Errorf("an empty library does not say so; texts: %q", tester.Texts())
	}
	if tester.HasText(shell.t().T(i18n.TagNoMatchTitle)) {
		t.Error("an empty library blames a filter")
	}

	// With tags present but filtered to nothing, the other message is right.
	seedTagOnly(t, db, "kept", "rust")
	shell.ReloadTagLibrary(ctx)
	shell.State().TagQuery = "zzz-no-such-tag"
	tester.Frame()
	if !tester.HasText(shell.t().T(i18n.TagNoMatchTitle)) {
		t.Errorf("a filter that matched nothing does not say so; texts: %q", tester.Texts())
	}
}

// TestSelectingEveryListedTag covers the bulk bar's select-all.
func TestSelectingEveryListedTag(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seedTagOnly(t, db, "a", "rust")
	seedTagOnly(t, db, "b", "go")
	shell.ReloadTagLibrary(ctx)

	shell.State().TagQuery = "rust"
	shell.ReloadTagLibrary(ctx)

	listed := shell.VisibleTagItems()
	if len(listed) != 1 {
		t.Fatalf("the filter listed %d tags", len(listed))
	}
	shell.SelectListedTags(listed)

	if got := shell.TagSelectionCount(); got != 1 {
		t.Errorf("selected %d, want only the listed one", got)
	}
	if !shell.State().TagSelectedIDs["a"] {
		t.Error("the listed tag is not selected")
	}
	// The one the filter hid is left alone.
	if shell.State().TagSelectedIDs["b"] {
		t.Error("an unlisted tag was selected")
	}
}

// TestTheMergePickerUsesTheSourcesLabel pins the picker's own word.
func TestTheMergePickerUsesTheSourcesLabel(t *testing.T) {
	shell, _ := newShell(t)
	if got := shell.t().T(i18n.TagMergeInto); got != "Merge into" {
		t.Errorf("the picker says %q, want the source's wording", got)
	}
}

// TestTheSettingsPanelHasItsSections covers the two group headings the source has.
//
// Only the sync group was labelled, so "Appearance" and "Language" read as sections of
// their own rather than fields of the general one.
func TestTheSettingsPanelHasItsSections(t *testing.T) {
	shell, _ := newShell(t)
	shell.OpenSettings()

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	// Settings is a page now, and its sections are uppercase tabs (shell-contract.md S37).
	for _, key := range []i18n.Key{
		i18n.SettingsSectionGeneral, i18n.SettingsSectionSync, i18n.SettingsSectionAutoTag,
	} {
		want := strings.ToUpper(shell.t().T(key))
		if !tester.HasText(want) {
			t.Errorf("the page has no %q tab; texts: %q", want, tester.Texts())
		}
	}
}

// The restore preview's wording is not asserted here.
//
// Building the string in the test and then checking it would assert nothing. The shell has
// no seam for injecting a prepared bundle preview, and writing one means an export, a
// temp file and an import before the dialog is even reachable -- more machinery than the
// claim needs. The change is covered by the build and the package's other tests; a
// targeted assertion needs a seam that does not exist yet.

// TestATagCanLoseItsTheme covers clearing a tag's theme.
//
// The option and the handler were both already in place; this pins them together, because
// the pair is the kind of thing that comes apart silently. An earlier version of this
// round claimed the picker did not offer "No theme" -- it did, on the line above the one
// the reading started from, and a mutation that removed a duplicate showed the claim was
// wrong.
func TestATagCanLoseItsTheme(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()

	theme := domain.Tag{
		ID: "theme", Name: "compilers", CanonicalKey: domain.CanonicalTagKey("compilers"),
		CreatedAt: domain.FormatTimestamp(clockBase), UpdatedAt: domain.FormatTimestamp(clockBase),
	}
	child := domain.Tag{
		ID: "child", Name: "rust", CanonicalKey: domain.CanonicalTagKey("rust"),
		ParentID: "theme", CreatedAt: domain.FormatTimestamp(clockBase),
		UpdatedAt: domain.FormatTimestamp(clockBase),
	}
	repo := sqlite.NewTagRepository(db)
	for _, tag := range []domain.Tag{theme, child} {
		if err := repo.Create(ctx, tag); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}
	shell.ReloadTagLibrary(ctx)

	// The option has to be offered before it can be chosen, so the view is checked and
	// not just the handler.
	shell.State().TagFocus = "child"
	shell.ToggleTagMode(ctx)
	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	if !tester.HasText(shell.t().T(i18n.TagParentNone)) {
		t.Errorf("the picker does not offer %q; texts: %q",
			shell.t().T(i18n.TagParentNone), tester.Texts())
	}

	shell.State().TagParentPick = shell.t().T(i18n.TagParentNone)
	shell.SetTagTheme(ctx)

	tags, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, tag := range tags {
		if tag.ID == "child" && tag.ParentID != "" {
			t.Errorf("the theme survived; child still sits under %q", tag.ParentID)
		}
	}
}

// TestTheMergeButtonNamesTheAction pins the tag library's merge button.
//
// It said "Apply selected", which names the selection rather than the thing being done
// to it; the source says "Merge tags".
func TestTheMergeButtonNamesTheAction(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seedTagOnly(t, db, "a", "rust")
	shell.ReloadTagLibrary(ctx)
	shell.ToggleTagMode(ctx)

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	// The merge is a bulk action, and the bar that holds it exists only while something is
	// selected -- the source's `if (!selectedCount) return null`. Round 507 moved it there from
	// the open column of controls, so the test ticks a row first.
	shell.SelectListedTags(shell.VisibleTagItems())
	tester = ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	want := shell.t().T(i18n.TagMergeLabel)
	if want != "Merge tags" {
		t.Errorf("the label is %q, want the source's wording", want)
	}
	if !tester.HasText(want) {
		t.Errorf("the merge button is missing; texts: %q", tester.Texts())
	}
}

// TestTheAuthorSectionListsAuthors covers a list that was always empty.
//
// The section declared a local empty slice called visibleAuthors, shadowing the method of
// the same name. The list was always empty, the search field filtered nothing that was
// drawn, and the section headed "Authors" said "No tags yet".
func TestTheAuthorSectionListsAuthors(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()

	// No display name, so the sidebar row draws the bare handle while the result card
	// draws it with an "@". ContainsText is exact, so "ada-h" can only be the row: with a
	// display name the two places draw the same string and the assertion cannot tell them
	// apart, which is how the first version survived a mutation.
	repo := sqlite.NewBookmarkRepository(db)
	for _, handle := range []string{"ada-h", "grace-h", "alan-h"} {
		bookmark := domain.Bookmark{
			TweetID: handle, TweetURL: "https://x.com/" + handle + "/status/1",
			AuthorName: "", AuthorHandle: handle, Text: "a post",
			SavedAt: clockBase, LastSeenAt: clockBase, UpdatedAt: clockBase,
		}
		if _, err := repo.UpsertMany(ctx, []domain.Bookmark{bookmark}); err != nil {
			t.Fatalf("UpsertMany: %v", err)
		}
	}
	shell.Reload(ctx)

	// The list reaches the frame on the authors tab; the default tab is the tags one.
	// The switch itself is covered by TestTheFacetTabsSwitchTheList.
	shell.State().SidebarFacetTab = sidebarTabAuthors

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	// The handle, exactly. A result card draws the display name and the handle with an
	// "@", so "ada-h" on its own can only come from the author row. HasText would pass on
	// "@ada-h" and would also pass with the list empty, which is how the first version of
	// this assertion survived a mutation.
	for _, handle := range []string{"ada-h", "grace-h", "alan-h"} {
		if !containsText(tester.Texts(), handle) {
			t.Errorf("author %q is not listed; texts: %q", handle, tester.Texts())
		}
	}
	// The search narrows the model. The view cannot be asked, because the result cards
	// also carry their author's handle.
	shell.State().AuthorQuery = "grace"
	rows := shell.visibleAuthors()
	if len(rows) != 1 || rows[0].AuthorHandle != "grace-h" {
		t.Errorf("the search left %v", rows)
	}
	shell.State().AuthorQuery = ""
}

// TestTheAuthorListIsDrawnWhole covers what replaced the source's six-row cap: the list has
// the column to itself now and scrolls in it, so no author sits behind a second control.
func TestTheAuthorListIsDrawnWhole(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()

	for i := 0; i < 9; i++ {
		handle := fmt.Sprintf("author%02d", i)
		seed(t, db, handle, handle, "a post", clockBase)
	}
	shell.Reload(ctx)
	shell.State().SidebarFacetTab = sidebarTabAuthors

	if got := len(shell.visibleAuthors()); got != 9 {
		t.Fatalf("the model holds %d authors", got)
	}

	tester := ui.NewTester(shell.View, 1180, 900)
	tester.Frame()

	// author08 sorts last and had no row under the cap; it is on the frame now.
	if !containsText(tester.Texts(), "author08") {
		t.Errorf("the last author is not drawn; texts: %q", tester.Texts())
	}
}

// savedViewsOf asks the workspace rather than the shell's state.
//
// Round 472 removed the sidebar's saved-views section, which left State.SavedViews assigned and
// never read -- the shape tools/unread.py exists to catch. The field is gone; these tests were
// asking it what the storage held, and the storage is the thing they meant.
func savedViewsOf(t *testing.T, shell *Shell) []domain.SavedView {
	t.Helper()
	views, err := shell.service.SavedViews(context.Background())
	if err != nil {
		t.Fatalf("SavedViews: %v", err)
	}
	return views
}
