package ui

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/apps/macos/internal/domain"
	"github.com/taoquo/trove/apps/macos/internal/i18n"
	"github.com/taoquo/trove/apps/macos/internal/storage/sqlite"
)

// The tag create flow, as the user reported it: "标签的添加按钮，这部分的逻辑UI也有很大问题".
//
// The row looked like a form with 添加 and 取消 buttons, but only the buttons did anything.
// Enter -- the gesture the row's own comment promised -- did nothing, Escape did nothing,
// the field never took the cursor, the hash before it was a clickable square with an empty
// handler, and 添加/取消 left the placeholder cut to 输入新标… inside a 300-point sidebar.
// The source's row has neither button: it is a hash and a borderless input, Enter submits,
// Escape cancels, an empty Enter closes it, and "+" pressed again returns the cursor to the
// field instead of discarding the half-typed name (WorkspaceSidebar.tsx:242-278, 516-551).

func TestEnterCreatesTheSidebarDraftTag(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	shell.Reload(ctx)

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	shell.BeginCreateTag()
	tester.Frame()

	label := shell.t().T(i18n.CreateTagLabel)
	if !tester.Focused(label) {
		t.Fatalf("the draft field did not take the cursor; texts: %q", tester.Texts())
	}

	tester.Type("rust")
	tester.Key(0, ui.KeyEnter)
	tester.Frame()

	tags, err := sqlite.NewTagRepository(db).List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(tags) != 1 || tags[0].Name != "rust" {
		t.Fatalf("Enter left the store with %+v, want one tag named rust", tags)
	}
	if shell.State().CreatingTag {
		t.Error("Enter created the tag and left the row open")
	}
	if shell.State().TagDraft != "" {
		t.Errorf("the draft still holds %q", shell.State().TagDraft)
	}
}

func TestEscapeClosesTheSidebarDraftRow(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	shell.Reload(ctx)

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	shell.BeginCreateTag()
	tester.Frame()
	tester.Type("half typed")
	tester.Key(0, ui.KeyEscape)
	tester.Frame()

	if shell.State().CreatingTag {
		t.Error("Escape left the row open")
	}
	if shell.State().TagDraft != "" {
		t.Errorf("Escape kept the draft %q", shell.State().TagDraft)
	}
	tags, err := sqlite.NewTagRepository(db).List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(tags) != 0 {
		t.Errorf("Escape created %+v", tags)
	}
}

func TestEmptyEnterClosesTheSidebarDraftRow(t *testing.T) {
	shell, _ := newShell(t)
	ctx := context.Background()
	shell.Reload(ctx)

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	shell.BeginCreateTag()
	tester.Frame()
	tester.Key(0, ui.KeyEnter)
	tester.Frame()

	// The source's submitInlineTagDraft resets the row when the name trims to empty
	// (WorkspaceSidebar.tsx:265-268): Enter is a way out of an empty row, not a no-op.
	if shell.State().CreatingTag {
		t.Error("Enter on an empty draft left the row open")
	}
}

func TestTheCreateRowCarriesNoSubmitButtons(t *testing.T) {
	shell, _ := newShell(t)
	shell.Reload(context.Background())

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	shell.BeginCreateTag()
	tester.Frame()

	for _, key := range []i18n.Key{i18n.ActionAdd, i18n.ActionCancel} {
		if tester.HasText(shell.t().T(key)) {
			t.Errorf("the draft row draws %q, which the source's row does not have", shell.t().T(key))
		}
	}
	// And the row it does have: the field, by its accessible name.
	if _, ok := tester.Find(shell.t().T(i18n.CreateTagLabel)); !ok {
		t.Error("the draft field is not on screen")
	}
}

func TestOpeningTheDraftTwiceKeepsWhatWasTyped(t *testing.T) {
	shell, _ := newShell(t)
	shell.Reload(context.Background())

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	shell.BeginCreateTag()
	tester.Frame()
	tester.Type("half typed")

	// The source's "+" while its row is open focuses the field and returns
	// (WorkspaceSidebar.tsx:271-278); it does not spend the draft.
	shell.BeginCreateTag()
	tester.Frame()

	if !shell.State().CreatingTag {
		t.Error("a second press closed the row")
	}
	if shell.State().TagDraft != "half typed" {
		t.Errorf("a second press left the draft %q", shell.State().TagDraft)
	}
}

func TestTheTwoDraftRowsNeverShareTheCursor(t *testing.T) {
	shell, _ := newShell(t)
	ctx := context.Background()
	shell.ReloadTagLibrary(ctx)
	shell.ToggleTagMode(ctx)

	shell.BeginCreateTag()
	shell.State().TagDraft = "half typed"
	shell.BeginLibraryCreateTag()

	if shell.State().CreatingTag {
		t.Error("opening the library's row left the sidebar's open")
	}
	if !shell.State().TagLibraryCreateOpen {
		t.Error("the library's row is not the one open")
	}
	// Both rows edit TagDraft, so the draft that belonged to the closed row has to go.
	if shell.State().TagDraft != "" {
		t.Errorf("the library's row starts with %q", shell.State().TagDraft)
	}
}

func TestEnterInTheInspectorTagsTheBookmark(t *testing.T) {
	shell, tester := detailShell(t, domain.Bookmark{
		TweetID: "detail-tag", TweetURL: "https://x.com/ada/status/detail-tag",
		AuthorName: "Ada Lovelace", AuthorHandle: "ada",
		Text:    "a post to tag from its own detail",
		SavedAt: clockBase, LastSeenAt: clockBase, UpdatedAt: clockBase,
	})
	ctx := context.Background()

	if err := tester.Click(DetailTagInputLabel); err != nil {
		t.Fatalf("Click: %v", err)
	}
	tester.Type("rust")
	tester.Key(0, ui.KeyEnter)
	tester.Frame()

	// The source's detail submission creates or finds the tag and attaches it to the
	// bookmark on screen (BookmarkInspector.tsx:815-845). The field had no submission at
	// all, so this path did not exist.
	view, err := shell.service.Load(ctx, domain.LibraryQuery{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if view.Counts.TagCount != 1 {
		t.Errorf("TagCount = %d, want the one typed tag", view.Counts.TagCount)
	}
	for _, row := range view.Bookmarks {
		if row.TweetID != "detail-tag" {
			continue
		}
		if len(row.Tags) != 1 || row.Tags[0].Name != "rust" {
			t.Errorf("the bookmark carries %+v, want rust", row.Tags)
		}
	}
	if shell.State().DetailTagInput != "" {
		t.Errorf("the field still holds %q", shell.State().DetailTagInput)
	}
}

func TestEscapeClearsTheInspectorTagField(t *testing.T) {
	shell, tester := detailShell(t, domain.Bookmark{
		TweetID: "detail-escape", TweetURL: "https://x.com/ada/status/detail-escape",
		AuthorName: "Ada Lovelace", AuthorHandle: "ada",
		Text:    "a post whose tag field is cleared",
		SavedAt: clockBase, LastSeenAt: clockBase, UpdatedAt: clockBase,
	})

	if err := tester.Click(DetailTagInputLabel); err != nil {
		t.Fatalf("Click: %v", err)
	}
	tester.Type("half typed")
	tester.Key(0, ui.KeyEscape)
	tester.Frame()

	// The source clears the draft on Escape (BookmarkInspector.tsx:340-345) and attaches
	// nothing.
	if shell.State().DetailTagInput != "" {
		t.Errorf("the field still holds %q", shell.State().DetailTagInput)
	}
	view, err := shell.service.Load(context.Background(), domain.LibraryQuery{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if view.Counts.TagCount != 0 {
		t.Errorf("Escape created %d tags", view.Counts.TagCount)
	}
}

func TestEnterCreatesFromTheTagLibrary(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	shell.ReloadTagLibrary(ctx)
	shell.ToggleTagMode(ctx)

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	shell.BeginLibraryCreateTag()
	tester.Frame()
	tester.Type("rust")
	tester.Key(0, ui.KeyEnter)
	tester.Frame()

	tags, err := sqlite.NewTagRepository(db).List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(tags) != 1 || tags[0].Name != "rust" {
		t.Fatalf("Enter left the store with %+v", tags)
	}
	if shell.State().TagLibraryCreateOpen {
		t.Error("Enter created the tag and left the row open")
	}
}
