package ui

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"

	engine "github.com/taoquo/trove/internal/autotag"
	"github.com/taoquo/trove/internal/i18n"
	"github.com/taoquo/trove/internal/storage/sqlite"
)

// reviewShell is a shell with a library and the review panel open on the given plans.
func reviewShell(t *testing.T) (*Shell, *sqlite.TagRepository, context.Context) {
	t.Helper()
	shell, db := newShell(t)
	ctx := context.Background()
	tags := sqlite.NewTagRepository(db)
	createTags(t, tags, map[string]string{
		"target": "ai agent", "source": "agentic ai", "lonely": "rust",
	})
	shell.ReloadTagLibrary(ctx)
	shell.state.TagMode = true
	shell.state.ReviewOpen = true
	shell.state.ReviewMerges = []engine.TagMergePlan{
		// A merge whose target no longer exists: the service refuses it.
		{TargetTagID: "ghost", TargetName: "gone", SourceTagIDs: []string{"target"}, SourceNames: []string{"ai agent"}},
		{TargetTagID: "target", TargetName: "ai agent", SourceTagIDs: []string{"source"}, SourceNames: []string{"agentic ai"}},
	}
	shell.state.ReviewRenames = []engine.TagRenamePlan{
		{TagID: "lonely", From: "rust", To: "rust-lang"},
	}
	shell.state.ReviewSelected = map[string]bool{}
	for _, merge := range shell.state.ReviewMerges {
		shell.state.ReviewSelected[engine.MergePlanKey(merge)] = true
	}
	for _, rename := range shell.state.ReviewRenames {
		shell.state.ReviewSelected[engine.RenamePlanKey(rename)] = true
	}
	return shell, tags, ctx
}

// TestOneFailedPlanDoesNotDiscardTheRest covers the review's apply loop.
//
// The loop must isolate each plan and collect the failures, then leave the panel open so the
// remainder can be retried. Returning on the first rejected plan throws away every good one
// behind it while the panel closes as if the whole review had run.
func TestOneFailedPlanDoesNotDiscardTheRest(t *testing.T) {
	shell, tags, ctx := reviewShell(t)

	shell.ApplyVocabularyReview(ctx)

	if _, found, err := tags.Get(ctx, "source"); err != nil || found {
		t.Errorf("the good merge behind a failed plan never ran: found=%v err=%v", found, err)
	}
	if tag, found, err := tags.Get(ctx, "lonely"); err != nil || !found || tag.Name != "rust-lang" {
		t.Errorf("the rename behind a failed plan never ran: found=%v name=%q err=%v", found, tag.Name, err)
	}

	if !shell.State().ReviewOpen {
		t.Error("the panel closed although a plan failed; the remainder has nowhere to be retried")
	}
	if got := shell.State().ReviewError; got == "" {
		t.Error("a failed plan was not reported in the panel")
	}
	if got := len(shell.State().ReviewMerges); got != 1 {
		t.Errorf("%d merge plans remain, want only the one that failed", got)
	}
	if got := len(shell.State().ReviewRenames); got != 0 {
		t.Errorf("%d rename plans remain, want none: it was applied", got)
	}
}

// TestThePanelStaysUpWhileItApplies covers the panel during the write.
//
// The panel must stay up and Apply must be disabled while saving; closing on the press leaves a
// slow edit on a large library with no visible cause.
func TestThePanelStaysUpWhileItApplies(t *testing.T) {
	shell, _, ctx := reviewShell(t)

	work := capturedBackground(shell)
	shell.ApplyVocabularyReview(ctx)

	if !shell.State().ReviewOpen {
		t.Error("the panel closed before the work started")
	}
	if !shell.State().ReviewApplying {
		t.Error("the panel does not say it is applying")
	}
	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	if want := shell.t().T(i18n.ReviewApplying); !tester.HasText(want) {
		t.Errorf("the panel shows no applying sentence; want %q, texts: %q", want, tester.Texts())
	}

	if *work == nil {
		t.Fatal("the review ran in the click handler")
	}
	(*work)()

	if shell.State().ReviewApplying {
		t.Error("the applying flag stayed set after the work finished")
	}
}

// TestApplyWaitsForASelection covers the button's disabled state.
//
// Apply is disabled when nothing is ticked. Running anyway closes the panel and reports "applied
// 0 changes", which reads as a successful no-op rather than as an unfilled form.
func TestApplyWaitsForASelection(t *testing.T) {
	shell, tags, ctx := reviewShell(t)
	shell.state.ReviewSelected = map[string]bool{}

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	if err := tester.Click(shell.t().T(i18n.ReviewApply)); err != nil {
		t.Fatalf("clicking Apply: %v", err)
	}
	tester.Frame()

	if !shell.State().ReviewOpen {
		t.Error("an Apply with nothing ticked closed the panel")
	}
	if _, found, err := tags.Get(ctx, "source"); err != nil || !found {
		t.Errorf("a plan ran although nothing was ticked: found=%v err=%v", found, err)
	}
	if got := shell.State().ReviewApplied; got != "" {
		t.Errorf("an empty selection reported %q", got)
	}
}

// TestTheTagPageSaysTheReviewLanded puts the receipt where the reader is.
//
// ReviewApplied belongs on the tag page the review is opened from, not in the results toolbar:
// there the sentence is written and never seen.
func TestTheTagPageSaysTheReviewLanded(t *testing.T) {
	shell, _, ctx := reviewShell(t)
	shell.state.ReviewMerges = shell.state.ReviewMerges[1:]
	shell.state.ReviewRenames = nil
	shell.state.ReviewSelected = map[string]bool{}
	shell.state.ReviewSelected[engine.MergePlanKey(shell.state.ReviewMerges[0])] = true

	shell.ApplyVocabularyReview(ctx)

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	want := shell.t().Tf(i18n.ReviewAppliedCount, 1)
	if !tester.HasText(want) {
		t.Errorf("the tag page does not report the applied review; want %q, texts: %q", want, tester.Texts())
	}
}

// TestAFailedReviewOffersAnotherTry keeps a request failure one press from a retry.
//
// The failed branch must offer another try; otherwise trying again means closing the panel and
// finding the toolbar button underneath, which the modal covers.
func TestAFailedReviewOffersAnotherTry(t *testing.T) {
	shell, _, _ := reviewShell(t)
	shell.state.ReviewMerges = nil
	shell.state.ReviewRenames = nil
	shell.state.ReviewError = "the model could not be reached"

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	if want := shell.t().T(i18n.ReviewRetry); !tester.HasText(want) {
		t.Errorf("a failed review offers no retry button; want %q, texts: %q", want, tester.Texts())
	}
}
