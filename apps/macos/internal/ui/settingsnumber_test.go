package ui

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/taoquo/x-master/apps/macos/internal/i18n"
	"github.com/taoquo/x-master/apps/macos/internal/usecase/tagging"
)

// TestTypingIntoANumberFieldReplacesTheValue is the defect a reader described as
// "the number can only be 25".
//
// Clicking a MyGo text field puts the caret at the end, so typing 50 into "25" made
// "2550"; the range check refused it and the field reverted on blur, which looks
// exactly like a setting that cannot be changed. Taking the focus now empties the
// field, so the first keystroke is the whole new number.
func TestTypingIntoANumberFieldReplacesTheValue(t *testing.T) {
	shell, db := newBatchShell(t)
	ctx := context.Background()
	tagger := tagging.New(autoTagRepos(db), fixedClock{now: clockBase}, nil)
	shell.UseTagging(tagger)
	shell.OpenTagManagementAutoTag()

	// Tall enough that the pacing rows are not clipped: Find reports the clipped
	// rect, and a clipped rect would make the click land somewhere else.
	tester := ui.NewTester(shell.View, 1180, 2400)
	tester.Frame()

	label := shell.t().T(i18n.SettingsBatchSize)
	rect, ok := tester.Find(label)
	if !ok {
		t.Fatalf("the batch size field has no rect; texts: %q", tester.Texts())
	}
	// The number field is the row's right-hand control, 72 wide.
	tester.ClickAt(1110, rect.Y+rect.H/2)
	tester.Frame()
	if !tester.Focused(label) {
		t.Fatalf("the field did not take the cursor; texts: %q", tester.Texts())
	}

	// No select-all: this is exactly what a click and typing produces.
	tester.Type("40")
	tester.Frame()

	if got := shell.State().BatchSizeInput; got != 40 {
		t.Fatalf("the field holds %v after typing 40 into it", got)
	}
	stored, err := tagger.LoadSettings(ctx)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if stored.LLM.BatchSize != 40 {
		t.Errorf("stored batch size = %d, want 40", stored.LLM.BatchSize)
	}
}
