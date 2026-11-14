package ui

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/internal/i18n"
	"github.com/taoquo/trove/internal/storage/sqlite"
)

// TestTheTagLibraryAnswersEveryClick walks the tag library as a pointer meets it: the row's
// six controls, the theme cell's picker, the bulk bar's merge, the orphan sweep, and the
// page's own create row.
//
// The census starts with the names, because that is what a reader (and this test) has to go
// on: every row used to carry four identical verbs and an unnamed checkbox, so nothing said
// which row a control belonged to and only the first row was reachable at all.
func TestTheTagLibraryAnswersEveryClick(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	shell.SetLocale(i18n.EN)

	seed(t, db, "1", "ada", "first", clockBase)
	seed(t, db, "2", "grace", "second", clockBase.Add(-1))
	seedTagOnly(t, db, "t-go", "go")
	seedTagOnly(t, db, "t-rust", "rust")
	seedTagOnly(t, db, "t-orphan", "orphan")
	if _, err := shell.service.AddTags(ctx, []string{"1"}, []string{"go", "rust"}); err != nil {
		t.Fatalf("tag the first bookmark: %v", err)
	}
	if _, err := shell.service.AddTags(ctx, []string{"2"}, []string{"go"}); err != nil {
		t.Fatalf("tag the second bookmark: %v", err)
	}
	shell.ReloadTagLibrary(ctx)
	shell.ToggleTagMode(ctx)

	tester := ui.NewTester(shell.View, 1400, 880)
	tester.Frame()
	click := func(label string) {
		t.Helper()
		tester.Frame()
		if err := tester.Click(label); err != nil {
			t.Fatalf("clicking %q: %v (texts: %q)", label, err, tester.Texts())
		}
		tester.Frame()
	}
	tagNamed := func(id string) string {
		t.Helper()
		tag, found, err := sqlite.NewTagRepository(db).Get(ctx, id)
		if err != nil || !found {
			t.Fatalf("reading tag %q: found=%v err=%v", id, found, err)
		}
		return tag.Name
	}

	// Every control on a row says which row it is on.
	for _, key := range []i18n.Key{
		i18n.TagSelectRowFor, i18n.TagBookmarksFor, i18n.TagRenameFor,
		i18n.TagAliasesFor, i18n.TagThemeFor, i18n.TagDeleteFor,
	} {
		if _, ok := tester.Find(shell.t().Tf(key, "go")); !ok {
			t.Errorf("the row for go has no %q; texts: %q", shell.t().Tf(key, "go"), tester.Texts())
		}
	}

	// The pencil turns that row's name into a field, and Enter writes.
	click(shell.t().Tf(i18n.TagRenameFor, "go"))
	if shell.state.TagFocus != "t-go" || shell.state.TagEditField != TagEditName {
		t.Fatalf("the rename opened %q on %q, want the name on go",
			shell.state.TagEditField, shell.state.TagFocus)
	}
	tester.Type("lang")
	tester.Key(0, ui.KeyEnter)
	tester.Frame()
	if got := tagNamed("t-go"); got != "golang" {
		t.Errorf("Enter left the tag named %q, want golang", got)
	}

	// The tag glyph opens the alias editor in the alias slot, and its Save writes.
	click(shell.t().Tf(i18n.TagAliasesFor, "golang"))
	if shell.state.TagFocus != "t-go" || shell.state.TagEditField != TagEditAliases {
		t.Fatalf("the aliases opened %q on %q", shell.state.TagEditField, shell.state.TagFocus)
	}
	click(shell.t().T(i18n.TagAliasesLabel))
	tester.Type("gopher")
	click(shell.t().T(i18n.ActionSave))
	items, err := shell.service.LoadTagLibrary(ctx)
	if err != nil {
		t.Fatalf("reading the library: %v", err)
	}
	aliased := false
	for _, item := range items {
		if item.TagID == "t-go" && len(item.Aliases) > 0 {
			aliased = true
		}
	}
	if !aliased {
		t.Errorf("the alias editor's Save wrote no alias; items: %+v", items)
	}

	// The theme cell opens its own picker; Escape leaves without writing.
	click(shell.t().Tf(i18n.TagThemeFor, "golang"))
	if shell.state.TagEditField != TagEditTheme {
		t.Fatalf("the theme cell opened %q", shell.state.TagEditField)
	}
	tester.Key(0, ui.KeyEscape)
	tester.Frame()
	if shell.state.TagEditField != "" {
		t.Error("Escape left the theme picker open")
	}

	// The row's box feeds the bulk bar; two rows ticked make it appear.
	click(shell.t().Tf(i18n.TagSelectRowFor, "golang"))
	click(shell.t().Tf(i18n.TagSelectRowFor, "rust"))
	if got := shell.TagSelectionCount(); got != 2 {
		t.Fatalf("the boxes selected %d rows, want 2", got)
	}

	// Merge: the picker's last choice is rust, and Merge asks before it folds anything.
	click(shell.t().T(i18n.TagMergeChoose))
	tester.Key(0, ui.KeyEnd)
	tester.Key(0, ui.KeyEnter)
	tester.Frame()
	if got := shell.state.TagMergeTarget; got != "rust" {
		t.Fatalf("the merge target is %q, want rust", got)
	}
	click(shell.t().T(i18n.TagMergeLabel))
	if !shell.state.ConfirmMergeTags {
		t.Fatal("Merge did not ask for confirmation")
	}
	shell.state.ConfirmMergeTags = false
	// A selection replaces the action row, so the sweep below needs the ticks dropped first.
	shell.ClearTagSelection()
	tester.Frame()

	// The orphan sweep is offered while an orphan exists, and asks first.
	click(shell.t().T(i18n.TagCleanupOrphans))
	if !shell.state.ConfirmCleanupTags {
		t.Fatal("the orphan sweep did not ask for confirmation")
	}
	shell.state.ConfirmCleanupTags = false
	tester.Frame()

	// The create row: the button opens it, Add creates the typed tag, Cancel closes it.
	click(shell.t().T(i18n.CreateTagLabel))
	if !shell.state.TagLibraryCreateOpen {
		t.Fatal("Create tag did not open the draft row")
	}
	click(shell.t().T(i18n.CreateTagPrompt))
	tester.Type("zeta")
	click(shell.t().T(i18n.ActionAdd))
	shell.ReloadTagLibrary(ctx)
	tester.Frame()
	created := false
	for _, item := range shell.state.TagItems {
		if item.Name == "zeta" {
			created = true
		}
	}
	if !created {
		t.Errorf("Add did not create the typed tag; items: %+v", shell.state.TagItems)
	}
	click(shell.t().T(i18n.CreateTagLabel))
	click(shell.t().T(i18n.ActionCancel))
	if shell.state.TagLibraryCreateOpen {
		t.Error("Cancel left the draft row open")
	}
}
