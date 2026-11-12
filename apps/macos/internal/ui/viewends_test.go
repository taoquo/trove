package ui

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestTheScrollEndsTrayWaitsForAScreen pins when scrollends.js shows its capsule: the list
// scrolls, the reader is a full screen down, and -- with no gutter to sit in -- the end is
// still far enough off that the tray would not cover the last row.
func TestTheScrollEndsTrayWaitsForAScreen(t *testing.T) {
	cases := []struct {
		name             string
		maxY, y, windowH float32
		gutter           bool
		want             bool
		why              string
	}{
		{"fits", 0, 0, 900, false, false, "a list shorter than its pane has no end to reach"},
		{"a little down", 2000, 400, 900, false, false, "one screen is the threshold"},
		{"a screen down", 2000, 860, 900, false, true, "past the window's height less the bar"},
		{"at the end", 2000, 2000, 900, false, false,
			"with no gutter the tray stands down rather than cover the last row"},
		{"near the end", 2000, 1950, 900, false, false,
			"inside the last tray-height plus its gap it stands down"},
		{"at the end, in a gutter", 2000, 2000, 900, true, true,
			"a wide window's margin holds it clear of the page's rows"},
	}
	for _, tc := range cases {
		scroll := ui.ScrollState{Y: tc.y, MaxY: tc.maxY}
		if got := viewEndsShown(&scroll, tc.windowH, tc.gutter); got != tc.want {
			t.Errorf("%s: viewEndsShown(Y=%v, MaxY=%v, window=%v, gutter=%v) = %v, want %v: %s",
				tc.name, tc.y, tc.maxY, tc.windowH, tc.gutter, got, tc.want, tc.why)
		}
	}
}

// TestTheScrollEndsTraySitsInAWidePagesMargin pins the tray's right offset: 10 points inside
// the page's corner, or centred in the right padding once that padding is a gutter
// (scrollends.js's `paint`).
func TestTheScrollEndsTraySitsInAWidePagesMargin(t *testing.T) {
	cases := []struct{ inset, want float32 }{
		{ResultsPadX, ViewEndsInset}, // 1180 and 1400 windows: the page has no margin
		{ViewEndsGutterMin, 8},       // the first inset that counts as a gutter
		{94, 34},                     // a 1600 window: centred in the 94-point margin
	}
	for _, tc := range cases {
		if got := viewEndsRight(tc.inset); got != tc.want {
			t.Errorf("viewEndsRight(%v) = %v, want %v", tc.inset, got, tc.want)
		}
	}
}
