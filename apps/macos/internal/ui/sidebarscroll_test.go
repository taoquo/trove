package ui

import (
	"context"
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"
)

// The sidebar list draws no scrollbar.
//
// The reader's report of 2026-11-10: "这种滚动条呢，感觉没有必要，把它移除掉吧". It was MyGo's
// overlay thumb, which appears as soon as the pointer is anywhere over the list and sits on
// top of the counts, because the list is as wide as the column -- there is no gutter for it.
// magpie's sidebar has none either (`.side-scroll { scrollbar-width: none }`,
// app.css:402-403); macOS itself shows scrollbars only while scrolling.
//
// The check is a pair of frames: nothing about the list changes between a pointer in the gap
// between two rows and a pointer outside the list, so any pixel that differs in the list's
// right-hand column is the thumb being drawn. That is what makes this red on the old build
// without knowing the thumb's colour, which differs by theme.
func TestTheSidebarListDrawsNoScrollbar(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "a post", clockBase)
	for i := 0; i < 40; i++ {
		seedTagOnly(t, db, fmt.Sprintf("t%02d", i), fmt.Sprintf("tag %02d", i))
	}
	shell.Reload(context.Background())

	tester := ui.NewTester(shell.View, 900, 760)
	tester.Frame()
	resting := tester.Image()

	first, ok := tester.Find("tag 00")
	if !ok {
		t.Fatalf("the tag list is not drawn; texts: %q", tester.Texts())
	}
	last, ok := tester.Find("tag 10")
	if !ok {
		t.Fatalf("the eleventh tag row is not drawn; texts: %q", tester.Texts())
	}

	// The pointer goes onto a row, which is what puts the scroll container in the hover chain;
	// the strip that is sampled runs down the list and across the gaps between rows, where no
	// row's ground reaches, so the thumb is the only thing that can paint there.
	tester.Move(first.X+20, first.Y+first.H/2)
	tester.Frame()
	hovered := tester.Image()

	// The thumb is nine points wide, ending three points short of the list's right edge
	// (scrollBars: width + 6 of track, inset 3 from the edge). The list is inset by
	// SidebarListsPadX from the column's edge.
	right := shell.state.LeftSidebarWidth - SidebarListsPadX
	top, bottom := first.Y-8, last.Y+last.H+8

	differing := 0
	firstDiff := ""
	for y := int(top); y <= int(bottom) && y < hovered.Bounds().Dy(); y++ {
		for x := int(right) - 9; x <= int(right)-3; x++ {
			if x < 0 || x >= hovered.Bounds().Dx() {
				continue
			}
			before := resting.RGBAAt(x, y)
			after := hovered.RGBAAt(x, y)
			if before != after {
				differing++
				if firstDiff == "" {
					firstDiff = fmt.Sprintf("(%d,%d): %v -> %v", x, y, before, after)
				}
			}
		}
	}
	if differing > 0 {
		t.Errorf("%d pixels of the list's right-hand column changed when the pointer entered it (first %s): the sidebar is drawing a scrollbar",
			differing, firstDiff)
	}
}

// TestTheSidebarListStillScrolls guards the other half: hiding the thumb must not take the
// scroll with it.
func TestTheSidebarListStillScrolls(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "a post", clockBase)
	for i := 0; i < 40; i++ {
		seedTagOnly(t, db, fmt.Sprintf("t%02d", i), fmt.Sprintf("tag %02d", i))
	}
	shell.Reload(context.Background())

	tester := ui.NewTester(shell.View, 900, 760)
	tester.Frame()

	first, ok := tester.Find("tag 00")
	if !ok {
		t.Fatalf("the list is not drawn; texts: %q", tester.Texts())
	}
	tester.Move(shell.state.LeftSidebarWidth/2, 300)
	tester.Frame()
	// A wheel notch moves the content up by its offset (a positive DY scrolls down).
	tester.Scroll(shell.state.LeftSidebarWidth/2, 300, 0, 240)
	tester.Frame()

	after, ok := tester.Find("tag 00")
	if ok && after.Y >= first.Y {
		t.Errorf("the first row is at y=%.1f after a wheel notch and was at y=%.1f: the list did not move",
			after.Y, first.Y)
	}
	if _, ok := tester.Find("tag 24"); !ok {
		t.Errorf("the rows below the fold are not drawn after scrolling; texts: %q", tester.Texts())
	}
}
