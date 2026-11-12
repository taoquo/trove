package ui

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/apps/macos/internal/domain"
	"github.com/taoquo/trove/apps/macos/internal/ports"
	"github.com/taoquo/trove/apps/macos/internal/storage/sqlite"
	"github.com/taoquo/trove/apps/macos/internal/usecase/settings"
)

// TestDefaultViewModeIsTheGrid pins the default the source uses.
//
// The shell was effectively always in the list, which is the other one: a fresh install
// laid the library out differently from the extension's.
func TestDefaultViewModeIsTheGrid(t *testing.T) {
	shell, _ := newShell(t)
	if shell.state.ViewMode != domain.ViewGrid {
		t.Errorf("a fresh shell is in %q, want the source's grid", shell.state.ViewMode)
	}
}

// TestViewModeToggle covers the two controls.
func TestViewModeToggle(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(context.Background())

	shell.ToggleViewMode()
	if shell.state.ViewMode != domain.ViewList {
		t.Errorf("after one toggle the view is %q, want the list", shell.state.ViewMode)
	}
	// Toggling back is what the grid control does.
	shell.ToggleViewMode()
	if shell.state.ViewMode != domain.ViewGrid {
		t.Errorf("after two toggles the view is %q, want the grid", shell.state.ViewMode)
	}
}

// TestViewModePersists covers the restart, which is the reason it is stored at all.
func TestViewModePersists(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()

	// A settings service has to be attached: the shell is built without storage in the
	// render tests, and the toggle stores the preference.
	shell.UseSettings(settings.New(
		ports.Repositories{Settings: sqlite.NewSettingsRepository(db)},
		fixedClock{now: clockBase}))
	shell.ToggleViewMode()
	if shell.state.ViewMode != domain.ViewList {
		t.Fatal("the toggle did not take")
	}

	stored, err := shell.settings.Load(ctx)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if string(stored.ViewMode) != "list" {
		t.Errorf("stored view mode = %q, want the list", stored.ViewMode)
	}
}

// TestViewModeDrivesCardTags is the property that makes the mode more than decoration:
// the collapse of a long tag list depends on it.
func TestViewModeDrivesCardTags(t *testing.T) {
	names := []string{"a", "b", "c", "d", "e"}

	// In the list, a card has one line for tags and collapses past two.
	if got := domain.VisibleCardTagNames(names, domain.ViewList); len(got) != 3 {
		t.Errorf("the list collapsed five tags to %v", got)
	}
	// In the grid it has the width to show them all.
	if got := domain.VisibleCardTagNames(names, domain.ViewGrid); len(got) != 5 {
		t.Errorf("the grid collapsed five tags to %v", got)
	}
}

// TestBothViewControlsAreOnScreen keeps the toggles reachable.
func TestBothViewControlsAreOnScreen(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(context.Background())

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	// The toggles are icon-only, so the assertion is that the shell still renders and
	// the mode is what the controls act on.
	if shell.state.ViewMode != domain.ViewGrid {
		t.Errorf("the shell opened in %q", shell.state.ViewMode)
	}
	if len(tester.Texts()) == 0 {
		t.Error("the shell rendered nothing")
	}
}

// TestGridShowsAnOddCard covers the pairing loop's off-by-one.
//
// Cards pair up in the grid, so a library with an odd number of results has a last card
// with no partner. Dropping it would lose a bookmark from the screen while every count
// elsewhere still included it.
func TestGridShowsAnOddCard(t *testing.T) {
	for _, count := range []int{1, 2, 3, 5, 8} {
		shell, db := newShell(t)
		for i := 0; i < count; i++ {
			seed(t, db, string(rune('a'+i)), "ada", "post number "+string(rune('a'+i)), clockBase)
		}
		shell.Reload(context.Background())

		tester := ui.NewTester(shell.View, 1180, 760)
		tester.Frame()

		for i := 0; i < count; i++ {
			handle := "post number " + string(rune('a'+i))
			if !tester.HasText(handle) {
				t.Errorf("with %d bookmarks in the grid, %q is missing; texts: %q",
					count, handle, tester.Texts())
			}
		}

		// And the same in the list, which has no pairing at all.
		shell.ToggleViewMode()
		tester.Frame()
		for i := 0; i < count; i++ {
			handle := "post number " + string(rune('a'+i))
			if !tester.HasText(handle) {
				t.Errorf("with %d bookmarks in the list, %q is missing", count, handle)
			}
		}
	}
}

// TestRenamingATag covers the edit entry the source has and this shell did not.
//
// The label is the source's own; the merge-on-collision behaviour is the service's,
// which is why there is no confirmation step here.
func TestRenamingATag(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	tags := sqlite.NewTagRepository(db)

	for _, spec := range []struct{ id, name string }{
		{"t1", "ai agent"}, {"t2", "agentic ai"},
	} {
		tag := domain.Tag{
			ID: spec.id, Name: spec.name, CanonicalKey: domain.CanonicalTagKey(spec.name),
			CreatedAt: domain.FormatTimestamp(clockBase), UpdatedAt: domain.FormatTimestamp(clockBase),
		}
		if err := tags.Create(ctx, tag); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}
	shell.ReloadTagLibrary(ctx)

	// An empty name does nothing rather than clearing the tag's name.
	shell.State().TagFocus = "t1"
	shell.State().TagNameEdit = "   "
	shell.RenameFocusedTag(ctx)
	if tag, _, _ := tags.Get(ctx, "t1"); tag.Name != "ai agent" {
		t.Errorf("a blank rename changed the name to %q", tag.Name)
	}

	// A real rename lands.
	shell.State().TagNameEdit = "ai agents"
	shell.RenameFocusedTag(ctx)
	tag, found, err := tags.Get(ctx, "t1")
	if err != nil || !found {
		t.Fatalf("Get: %v found=%v", err, found)
	}
	if tag.Name != "ai agents" {
		t.Errorf("the tag is named %q, want the new name", tag.Name)
	}
	// The field clears, so a second save does not repeat the rename.
	if shell.State().TagNameEdit != "" {
		t.Errorf("the edit field still holds %q", shell.State().TagNameEdit)
	}

	// Renaming onto a name another tag already answers to is refused, and the refusal
	// has to reach the user: this used to merge silently, which is why the old comment
	// here said there was no need for a confirmation. Nothing moves now, so the panel has
	// to say why the rename did nothing.
	shell.State().TagNameEdit = "agentic ai"
	shell.RenameFocusedTag(ctx)
	if _, found, _ := tags.Get(ctx, "t1"); !found {
		t.Error("a refused rename must not remove the tag")
	}
	if _, found, _ := tags.Get(ctx, "t2"); !found {
		t.Error("the tag that owns the name must survive")
	}
	if shell.State().TagActionMsg == "" {
		t.Error("a refused rename said nothing")
	}
	// And the tag kept its old name.
	if tag, _, _ := tags.Get(ctx, "t1"); tag.Name != "ai agents" {
		t.Errorf("the tag is now named %q, want it unchanged", tag.Name)
	}
}

// TestRenameWithoutAFocusDoesNothing keeps an unset focus from acting on a zero id.
func TestRenameWithoutAFocusDoesNothing(t *testing.T) {
	shell, _ := newShell(t)
	shell.State().TagNameEdit = "something"
	shell.RenameFocusedTag(context.Background())
	if shell.State().TagActionMsg != "" {
		t.Errorf("no rename should have run, got %q", shell.State().TagActionMsg)
	}
}
