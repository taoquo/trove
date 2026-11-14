package ui

import (
	"context"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// The motion language, as behaviour.
//
// These pin the parts that are arithmetic or state -- the page's entrance, the navigation
// that starts it, and the copy chip's confirmation. The colour transitions and the pulses
// are MyGo's own animations; what belongs to this shell is when they are asked for, and that
// is what these ask about.

// TestThePageEntranceStartsLowAndSettles is `view-in`: opacity from
// nothing and a 5-point lift, both gone at 0.24s.
func TestThePageEntranceStartsLowAndSettles(t *testing.T) {
	opacity, offset := pageEntrance(0, false)
	if opacity != 0 {
		t.Errorf("the entrance starts at opacity %v, want 0", opacity)
	}
	if offset != PageEnterOffset {
		t.Errorf("the entrance starts %v points down, want %d", offset, PageEnterOffset)
	}

	// Part way through it is between the two, in both.
	opacity, offset = pageEntrance(time.Duration(MotionPageMs/2)*time.Millisecond, false)
	if opacity <= 0 || opacity >= 1 {
		t.Errorf("half way through the entrance opacity is %v, want between 0 and 1", opacity)
	}
	if offset <= 0 || offset >= PageEnterOffset {
		t.Errorf("half way through the entrance lift is %v, want between 0 and %d",
			offset, PageEnterOffset)
	}

	// And at the duration it is finished -- exactly, not nearly.
	opacity, offset = pageEntrance(time.Duration(MotionPageMs)*time.Millisecond, false)
	if opacity != 1 || offset != 0 {
		t.Errorf("the finished entrance is opacity %v, lift %v; want 1 and 0", opacity, offset)
	}
}

// TestReducedMotionKeepsTheFadeAndDropsTheLift is the reduced-motion rule: the
// animation becomes `fade-in`, which is the same fade without the translateY.
func TestReducedMotionKeepsTheFadeAndDropsTheLift(t *testing.T) {
	opacity, offset := pageEntrance(0, true)
	if opacity != 0 {
		t.Errorf("the reduced-motion entrance starts at opacity %v, want 0", opacity)
	}
	if offset != 0 {
		t.Errorf("the reduced-motion entrance lifts %v points, want 0", offset)
	}
}

// TestNavigationStartsThePageEntrance: the entrance belongs to a change of page, so the
// controls that change it mark the change. A control that reloads the list without
// changing the page (applying a tag, deleting a bookmark) must not, or the feed would fade
// on every edit.
func TestNavigationStartsThePageEntrance(t *testing.T) {
	shell, _ := newShell(t)

	shell.ToggleViewMode()
	if shell.state.PageChangedAt.IsZero() {
		t.Error("switching between grid and list did not start the page's entrance")
	}

	shell.state.PageChangedAt = time.Time{}
	shell.selectSidebar(context.Background(), selectionUnclassified)
	if shell.state.PageChangedAt.IsZero() {
		t.Error("selecting a scope did not start the page's entrance")
	}
}

// TestAPressedControlGivesUnderThePointer is a pressed labelled button: it shrinks to .96
// while the pointer holds it, and comes back when it lets go.
func TestAPressedControlGivesUnderThePointer(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(context.Background())

	tester := ui.NewTester(shell.View, 1180, 760)
	before, ok := tester.Find("Filters")
	if !ok {
		t.Fatalf("the toolbar's filter button is not drawn; texts: %q", tester.Texts())
	}
	// The press eases over MotionPress, and the tester's frames advance with the clock.
	x, y := before.X+before.W/2, before.Y+before.H/2
	tester.Press(x, y)
	time.Sleep(2 * time.Duration(MotionPressMs) * time.Millisecond)
	tester.Frame()
	pressed, ok := tester.Find("Filters")
	if !ok {
		t.Fatal("the filter button disappears while pressed")
	}
	if pressed.W >= before.W {
		t.Errorf("a pressed button is %v wide, no narrower than its %v at rest",
			pressed.W, before.W)
	}
	if want := before.W * PressScaleText; pressed.W > want*1.02 || pressed.W < want*0.98 {
		t.Errorf("a pressed button is %v wide; .96 of %v is about %v",
			pressed.W, before.W, want)
	}

	// Let go outside the button: a release over it would be a click, and this test is about
	// the give, not what the button does.
	tester.Move(x, y-40)
	tester.Release(x, y-40)
	time.Sleep(2 * time.Duration(MotionPressMs) * time.Millisecond)
	tester.Frame()
	if released, _ := tester.Find("Filters"); released.W != before.W {
		t.Errorf("the button is %v wide once released, want its %v back", released.W, before.W)
	}
	if shell.state.FilterPopoverOpen {
		t.Error("the press opened the filter popover; the test is about the button's give")
	}
}

// TestASubRowPaintsNoGroundOfItsOwn: in the list only the selection is a shape. A
// row hovered or pressed has no ground, "which would show where the finger is before the
// selection slides there", and the row never shrinks either. Its ground
// at rest, under the pointer and while pressed is the sidebar's own.
func TestASubRowPaintsNoGroundOfItsOwn(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(context.Background())
	if _, err := shell.service.CreateTag(context.Background(), "Go"); err != nil {
		t.Fatalf("CreateTag: %v", err)
	}
	shell.Reload(context.Background())

	tester := ui.NewTester(shell.View, 1180, 760)
	label, ok := tester.Find("Go")
	if !ok {
		t.Fatalf("the sidebar's tag row is not drawn; texts: %q", tester.Texts())
	}
	// A point inside the row clear of its words.
	sampleX, sampleY := int(label.X)-4, int(label.Y+label.H/2)
	ink := func() uint8 {
		c := tester.Image().RGBAAt(sampleX, sampleY)
		return uint8((int(c.R) + int(c.G) + int(c.B)) / 3)
	}
	rest := ink()

	tester.Move(label.X+4, label.Y+label.H/2)
	time.Sleep(2 * time.Duration(MotionFastMs) * time.Millisecond)
	tester.Frame()
	if hovered := ink(); hovered != rest {
		t.Errorf("a hovered row is %d, not the list's own %d: it has no ground", hovered, rest)
	}

	tester.Press(label.X+4, label.Y+label.H/2)
	time.Sleep(2 * time.Duration(MotionFastMs) * time.Millisecond)
	tester.Frame()
	if pressed := ink(); pressed != rest {
		t.Errorf("a pressed row is %d, not the list's own %d: it has no ground", pressed, rest)
	}
	if after, _ := tester.Find("Go"); after.W != label.W {
		t.Errorf("the pressed row's label is %v wide, not its %v: a row must not shrink",
			after.W, label.W)
	}

	// Let go clear of the row, so the press does not become a click.
	tester.Move(label.X+4, label.Y-40)
	tester.Release(label.X+4, label.Y-40)
}

// TestReducedMotionSwitchesAColourAtOnce: MyGo's AnimateColor does not read ReduceMotion,
// so the shell's own helper must. Under reduced motion the first frame of a hover is
// already the hover colour; without it, that frame is partway there.
func TestReducedMotionSwitchesAColourAtOnce(t *testing.T) {
	base, hover := ui.RGBA(255, 255, 255, 1), ui.RGBA(0, 0, 0, 1)
	measure := func(reduceMotion bool) ui.Color {
		var shown ui.Color
		tester := ui.NewTester(func(c *ui.Context) {
			ui.Column(c).Fill().Padding(100).Children(func() {
				box := ui.Box(c).Width(80).Height(80).Background(base)
				shown = hoverColor(c, box, "hover", base, hover, box.Hovered())
			})
		}, 400, 400)
		tester.SetPreferences(ui.Preferences{ReduceMotion: reduceMotion, TextScale: 1})
		// The pointer starts away from the box; moving onto it is the change under test.
		tester.Move(150, 150)
		tester.Frame()
		return shown
	}

	if got := measure(true); got != hover {
		t.Errorf("under reduced motion a hover shows %v on its first frame, want %v", got, hover)
	}
	if got := measure(false); got == hover {
		t.Errorf("without reduced motion a hover jumps to %v in one frame; it should ease", got)
	}
}

// TestThePopoverGrowsOutOfItsButton is the popover's entrance: a fade over
// 0.14s, and a rise from 6 points up at .94 over 0.24s, the movement dropped under reduced
// motion while the fade stays.
func TestThePopoverGrowsOutOfItsButton(t *testing.T) {
	opacity, rise, scale := popoverEntrance(0, false)
	if opacity != 0 {
		t.Errorf("the panel starts at opacity %v, want 0", opacity)
	}
	if rise != PopInRise || scale != PopInScale {
		t.Errorf("the panel starts %v points up at %v, want %d at %v", rise, scale, PopInRise, PopInScale)
	}

	// Part way through both are between: the movement is still going when the fade has
	// finished, which is why they run on two clocks.
	opacity, rise, scale = popoverEntrance(time.Duration(PopFadeMs)*time.Millisecond, false)
	if opacity != 1 {
		t.Errorf("after the fade the panel is at opacity %v, want 1", opacity)
	}
	if rise <= 0 || rise >= PopInRise {
		t.Errorf("after the fade the panel is %v points up, want between 0 and %d", rise, PopInRise)
	}
	if scale <= PopInScale || scale >= 1 {
		t.Errorf("after the fade the panel is at %v, want between %v and 1", scale, PopInScale)
	}

	if opacity, rise, scale = popoverEntrance(time.Duration(PopInMs)*time.Millisecond, false); opacity != 1 || rise != 0 || scale != 1 {
		t.Errorf("the settled panel is %v / %v / %v, want 1 / 0 / 1", opacity, rise, scale)
	}

	// A reader who asked for less motion gets the fade alone, and a shorter one.
	if opacity, rise, scale = popoverEntrance(0, true); opacity != 0 || rise != 0 || scale != 1 {
		t.Errorf("reduced motion starts %v / %v / %v, want 0 / 0 / 1", opacity, rise, scale)
	}
	if opacity, _, _ = popoverEntrance(time.Duration(PopFadeReduced)*time.Millisecond, true); opacity != 1 {
		t.Errorf("reduced motion is at opacity %v after its shorter fade, want 1", opacity)
	}
}

// TestTheCopyChipHoldsItsConfirmationForAMoment: the chip says "copied" in place, and stops
// saying it on its own -- the deadline is what the view reads, and CopySourceLink schedules
// the repaint that turns it off.
func TestTheCopyChipHoldsItsConfirmationForAMoment(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(context.Background())
	shell.state.Selected = "1"

	if shell.copiedJustNow() {
		t.Error("a fresh shell claims something was just copied")
	}

	var captured string
	shell.CopySourceLink(&clipboardContext{written: &captured})
	if !shell.copiedJustNow() {
		t.Error("the copy chip does not confirm a copy")
	}

	shell.state.CopiedAt = time.Now().Add(-CopiedFeedbackFor - time.Millisecond)
	if shell.copiedJustNow() {
		t.Error("the copy chip still confirms after its deadline")
	}
}

// TestAReducedMotionPulseStaysFullyVisible pins what the comment above pulse promises.
//
// pulse is a two-frame alternate loop that never ends. MyGo's reduced-motion path takes a
// sequence to its final value, and for an alternate no-repeat-end loop that value is the
// *first* frame, not the last (mygo motion.go:646-651: RepeatForever plays 0 times, and 0
// is even). So the skeleton and the syncing dot were frozen at 0.55 opacity for a reader
// who asked for less motion -- dimmer than the page around them, which reads as disabled
// rather than as still.
func TestAReducedMotionPulseStaysFullyVisible(t *testing.T) {
	var shown float32
	tester := ui.NewTester(func(c *ui.Context) {
		shown = pulse(c, ui.Box(c).Width(10).Height(10), "pulse")
	}, 40, 40)
	tester.SetPreferences(ui.Preferences{ReduceMotion: true})
	tester.Frame()

	if shown != 1 {
		t.Errorf("a pulse under reduced motion shows at opacity %v, want the steady 1", shown)
	}
}
