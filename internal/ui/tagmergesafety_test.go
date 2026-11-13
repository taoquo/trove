package ui

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/internal/i18n"
	"github.com/taoquo/trove/internal/storage/sqlite"
)

// TestMergeNeedsATargetTheReaderChose pins the merge target to a choice.
//
// Ticking a row used to fill the target with the first listed tag and enable Merge, so a
// reader who ticked rows and pressed Merge folded them into whichever tag sorted first --
// a destructive write nobody chose. The target starts empty, Merge stays disabled until one is
// picked, and the handler returns early on an empty target.
func TestMergeNeedsATargetTheReaderChose(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seedTagOnly(t, db, "tag-a", "ai agent")
	seedTagOnly(t, db, "tag-b", "agentic ai")
	shell.ReloadTagLibrary(ctx)
	shell.ToggleTagMode(ctx)

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	// The table head ticks every listed tag, which is the bar's own way into the merge.
	if err := tester.Click(shell.t().T(i18n.TagSelectAll)); err != nil {
		t.Fatalf("clicking the head checkbox: %v", err)
	}
	tester.Frame()

	if got := shell.State().TagMergeTarget; got != "" {
		t.Fatalf("ticking rows chose the merge target %q; the target starts unchosen", got)
	}

	// The bar says the target is unchosen rather than naming a default.
	if want := shell.t().T(i18n.TagMergeChoose); !tester.HasText(want) {
		t.Errorf("the picker names no placeholder; want %q in %q", want, tester.Texts())
	}

	if err := tester.Click(shell.t().T(i18n.TagMergeLabel)); err != nil {
		t.Fatalf("clicking Merge: %v", err)
	}
	tester.Frame()

	tags := sqlite.NewTagRepository(db)
	for _, id := range []string{"tag-a", "tag-b"} {
		if _, found, err := tags.Get(ctx, id); err != nil || !found {
			t.Errorf("tag %q was merged away without a chosen target: found=%v err=%v", id, found, err)
		}
	}

	// The disabled button never reaches the handler, so the guard is exercised directly:
	// a merge asked for without a target says so instead of guessing one.
	shell.AskMergeSelectedTags()
	if got := shell.State().TagActionMsg; got != shell.t().T(i18n.TagMergeNeedTarget) {
		t.Errorf("a merge without a target said %q; want the sentence that asks for one", got)
	}
}

// TestMergeAsksBeforeItFolds covers the confirmation before the fold.
//
// Merging removes every source tag, so it must ask before it runs and name the tag the selection
// is folded into; merging on the first press makes a miss-click a delete.
func TestMergeAsksBeforeItFolds(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seedTagOnly(t, db, "tag-a", "ai agent")
	seedTagOnly(t, db, "tag-b", "agentic ai")
	shell.ReloadTagLibrary(ctx)
	shell.ToggleTagMode(ctx)

	target := shell.State().TagItems[0].Name
	shell.state.TagMergeTarget = target
	shell.state.TagSelectedIDs = map[string]bool{"tag-a": true, "tag-b": true}

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	if err := tester.Click(shell.t().T(i18n.TagMergeLabel)); err != nil {
		t.Fatalf("clicking Merge: %v", err)
	}
	tester.Frame()

	tags := sqlite.NewTagRepository(db)
	for _, id := range []string{"tag-a", "tag-b"} {
		if _, found, err := tags.Get(ctx, id); err != nil || !found {
			t.Errorf("tag %q was merged before the reader answered: found=%v err=%v", id, found, err)
		}
	}
	if got := shell.TagSelectionCount(); got != 2 {
		t.Errorf("the merge consumed the selection before it was confirmed: %d tags left", got)
	}
}

// TestMergingOnlyTheTargetSaysThereIsNothingToMerge covers the other early return.
//
// The target is filtered out of the merge sources, and the handler returns when nothing is left.
// Running the write anyway reports "merged 0 tags", a sentence about work that never existed.
func TestMergingOnlyTheTargetSaysThereIsNothingToMerge(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seedTagOnly(t, db, "tag-a", "ai agent")
	seedTagOnly(t, db, "tag-b", "agentic ai")
	shell.ReloadTagLibrary(ctx)

	shell.state.TagSelectedIDs = map[string]bool{"tag-a": true}
	shell.state.TagMergeTarget = "ai agent"
	shell.AskMergeSelectedTags()

	if shell.State().ConfirmMergeTags {
		t.Error("a selection that is only the target opened a confirmation")
	}
	if got := shell.State().TagActionMsg; got != shell.t().T(i18n.TagMergeNeedsSource) {
		t.Errorf("merging a tag into itself said %q; want the sentence that asks for another tag", got)
	}
	tags := sqlite.NewTagRepository(db)
	for _, id := range []string{"tag-a", "tag-b"} {
		if _, found, err := tags.Get(ctx, id); err != nil || !found {
			t.Errorf("tag %q disappeared: found=%v err=%v", id, found, err)
		}
	}
}

// TestTheConfirmationNamesTheSurvivor keeps the question answerable.
//
// "Merge the selected tags?" does not say which one survives, and the survivor is the whole
// decision: the other names are about to be gone. The body carries the count and the target's
// name, and only the confirming button runs the write.
func TestTheConfirmationNamesTheSurvivor(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seedTagOnly(t, db, "tag-a", "ai agent")
	seedTagOnly(t, db, "tag-b", "agentic ai")
	shell.ReloadTagLibrary(ctx)
	shell.state.TagSelectedIDs = map[string]bool{"tag-a": true, "tag-b": true}
	shell.state.TagMergeTarget = "ai agent"
	shell.AskMergeSelectedTags()

	if !shell.State().ConfirmMergeTags {
		t.Fatal("a merge with a target and a source did not ask")
	}

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	want := shell.t().Tf(i18n.TagMergeConfirmBody, 1, "ai agent")
	if !tester.HasText(want) {
		t.Errorf("the dialog does not name the survivor; want %q, texts: %q", want, tester.Texts())
	}

	shell.ConfirmMergeSelectedTagsNow(ctx)

	tags := sqlite.NewTagRepository(db)
	if _, found, err := tags.Get(ctx, "tag-b"); err != nil || found {
		t.Errorf("the tag being merged survived its confirmed merge: found=%v err=%v", found, err)
	}
	if _, found, err := tags.Get(ctx, "tag-a"); err != nil || !found {
		t.Errorf("the surviving tag was merged away: found=%v err=%v", found, err)
	}
	if got := shell.TagSelectionCount(); got != 0 {
		t.Errorf("the selection survived the merge: %d tags", got)
	}
	if got := shell.State().TagMergeTarget; got != "" {
		t.Errorf("the merge kept its target after folding it in: %q", got)
	}
	if got := shell.State().TagActionMsg; got != shell.t().Tf(i18n.TagMergeDone, 1) {
		t.Errorf("the merge reported %q; want the count of tags it removed", got)
	}
}

// TestARefusedRenameSaysWhyInTheUiLanguage covers the collision path.
//
// Another tag may already answer to the name, in which case the rename is refused. The use
// case reports that with a typed error whose Error() is the internal sentence -- package
// prefix, English -- and the row printed it as-is, so a Chinese window answered a rename
// with "workspace: cannot rename ...". The typed error carries the wanted name, which is
// what the sentence needs.
func TestARefusedRenameSaysWhyInTheUiLanguage(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seedTagOnly(t, db, "tag-a", "rust")
	seedTagOnly(t, db, "tag-b", "rustlang")
	shell.ReloadTagLibrary(ctx)

	shell.state.TagFocus = "tag-b"
	shell.state.TagNameEdit = "Rust"
	shell.RenameFocusedTag(ctx)

	want := shell.t().Tf(i18n.TagRenameTaken, "Rust")
	if got := shell.State().TagActionMsg; got != want {
		t.Errorf("a refused rename said %q; want %q", got, want)
	}
}
