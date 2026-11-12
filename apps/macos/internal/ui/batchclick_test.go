package ui

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/x-master/apps/macos/internal/domain"
	"github.com/taoquo/x-master/apps/macos/internal/i18n"
)

// TestTheBatchBarAnswersEveryClick walks the bar the reader gets after ticking a box:
// its count, the typed tag field, both pickers with their Apply buttons, the two selection
// buttons, the way to drop the selection, and delete with its confirmation.
//
// It follows rounds 35-38's rule: press what the control draws or is named, and assert the
// state its name promises. Round 39.
func TestTheBatchBarAnswersEveryClick(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	shell.SetLocale(i18n.EN, i18n.EN)

	seed(t, db, "1", "ada", "first", clockBase)
	seed(t, db, "2", "grace", "second", clockBase.Add(-1))
	seed(t, db, "3", "hopper", "third", clockBase.Add(-2))
	seedTagOnly(t, db, "t-rust", "rust")
	if _, err := shell.service.AddTags(ctx, []string{"1"}, []string{"rust"}); err != nil {
		t.Fatalf("seed the tag: %v", err)
	}
	if _, err := shell.service.CreateList(ctx, "Work"); err != nil {
		t.Fatalf("CreateList: %v", err)
	}
	shell.Reload(ctx)

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
	reselect := func(ids ...string) {
		t.Helper()
		shell.state.SelectedIDs = make(map[string]bool, len(ids))
		for _, id := range ids {
			shell.state.SelectedIDs[id] = true
		}
		tester.Frame()
	}
	tagsOf := func() map[string][]string {
		t.Helper()
		view, err := shell.service.Load(ctx, domain.LibraryQuery{})
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		out := map[string][]string{}
		for _, row := range view.Bookmarks {
			for _, tag := range row.Tags {
				out[row.TweetID] = append(out[row.TweetID], tag.Name)
			}
		}
		return out
	}

	reselect("1", "2")
	if !tester.HasText(shell.t().Tf(i18n.BatchSelected, 2)) {
		t.Errorf("the bar does not say how many are selected; texts: %q", tester.Texts())
	}

	// The typed-tag field is this build's own -- the source's bar has only its picker -- and
	// it is the one control in the bar with a name of its own to answer to. Without one a
	// screen reader hears "text field" and nothing else, and the tester cannot reach it.
	click(shell.t().T(i18n.CreateTagPrompt))
	tester.Type("go")
	tester.Key(0, ui.KeyEnter)
	tester.Frame()
	if got := tagsOf(); len(got["1"]) != 2 || len(got["2"]) != 1 {
		t.Errorf("Enter in the tag field left the rows tagged %v, want rust+go and go", got)
	}
	if shell.ToastText() == "" {
		t.Error("tagging the selection said nothing")
	}
	if got := shell.BatchSelection(); len(got) != 0 {
		t.Errorf("the selection survived the tag write: %v", got)
	}

	// The tag picker and its Apply are two names, as the source names the select
	// (BookmarkResultsPane.tsx:50-53): "Choose tag" opens the list, "Apply tag" writes.
	// The picker is a popup, not a platform menu, so it is walked from the keyboard: the
	// trigger has the focus, one step down leaves the placeholder, Enter chooses.
	reselect("1", "2")
	click(shell.t().T(i18n.BatchChooseTag))
	// The options are the placeholder, "go", then "rust"; End is the last one.
	tester.Key(0, ui.KeyEnd)
	tester.Key(0, ui.KeyEnter)
	tester.Frame()
	if got := shell.state.BulkTagLabel; got != "rust" {
		t.Fatalf("the picker chose %q, want rust", got)
	}
	click(shell.t().T(i18n.BatchApplyTag))
	if got := tagsOf(); len(got["1"]) != 2 || len(got["2"]) != 2 {
		t.Errorf("Apply tag left the rows tagged %v", got)
	}

	// The move picker and its action are likewise two names.
	reselect("1", "2")
	click(shell.t().T(i18n.BatchChooseList))
	tester.Key(0, ui.KeyEnd)
	tester.Key(0, ui.KeyEnter)
	tester.Frame()
	if got := shell.state.MoveTargetLabel; got != "Work" {
		t.Fatalf("the move picker chose %q, want Work", got)
	}
	click(shell.t().T(i18n.ActionMoveToList))
	if want := shell.t().Tf(i18n.BatchMoved, 2, "Work"); !tester.HasText(want) {
		t.Errorf("moving the selection did not name its destination; want %q, texts: %q", want, tester.Texts())
	}

	// "Select loaded" takes what the list holds; "Select current results" takes everything
	// the query matches. At this size both answer, and both set the same three rows.
	reselect("1")
	click(shell.t().T(i18n.BatchSelectLoaded))
	if got := shell.BatchSelection(); len(got) != 3 {
		t.Errorf("Select loaded selected %d rows, want the 3 the list holds", len(got))
	}
	// The bar only draws while something is selected, so the second button is pressed with
	// one row ticked; it then widens the selection to everything the query matches.
	reselect("1")
	click(shell.t().T(i18n.BatchSelectCurrent))
	if got := shell.BatchSelection(); len(got) != 3 {
		t.Errorf("Select current results selected %d rows, want 3", len(got))
	}

	// Clear selection empties it, and the bar goes with it.
	click(shell.t().T(i18n.ActionClearSelection))
	if got := shell.BatchSelection(); len(got) != 0 {
		t.Errorf("Clear selection left %v", got)
	}
	if tester.HasText(shell.t().Tf(i18n.BatchSelected, 3)) {
		t.Error("the bar stayed after the selection was cleared")
	}

	// Delete asks first, and a dismissed confirmation leaves the library alone.
	reselect("1")
	click(shell.t().T(i18n.ActionDelete))
	if !shell.deleteBookmarks.Confirmation.Asked() {
		t.Fatal("Delete did not ask for confirmation")
	}
	shell.deleteBookmarks.Confirmation.Answer(DismissedValue)
	shell.applyDeleteBookmarks(ctx)
	shell.Reload(ctx)
	tester.Frame()
	if got := len(shell.state.Data.Bookmarks); got != 3 {
		t.Errorf("a dismissed delete left %d bookmarks, want 3", got)
	}
}
