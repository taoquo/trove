package ui

import (
	"github.com/egoist/mygo/ui"
)

// IconOnly draws a button-shaped square that is just the glyph, for the places an
// aria-labelled icon button fits.
// IconOnly is a bare glyph that takes a surface only while the pointer is over it.
//
// The state background is transparent by default, SurfaceHover on hover and SurfacePressed
// while pressed, and every icon-only control carries an accessible name.
func IconOnly(c *ui.Context, name, tooltip string, theme *ui.Theme, rail RailTokens, onClick func()) {
	IconOnlySized(c, name, tooltip, theme, rail, IconButtonSize, onClick)
}

// IconOnlySized is IconOnly with the square given, for the places the stylesheet sizes differently.
//
// The sidebar footer's controls are 28x28 with an 8 radius -- not the 32 that
// IconButtonSize carries for a bare icon button. One number was serving both, so
// every footer control was four points too large in each direction.
func IconOnlySized(c *ui.Context, name, tooltip string, theme *ui.Theme, rail RailTokens, size float32, onClick func()) {
	// Focusable: Box + OnClick listens to the pointer only, so every icon-only control was
	// unreachable from the keyboard. A focusable clickable takes Enter and Space.
	box := ui.Box(c).Width(size).Height(size).Focusable().
		Radius(RadiusMedium)
	// The fill eases in and out: an icon button gives its background a 150ms transition,
	// and a chip that appears the instant the pointer crosses it is the
	// difference between a surface that reacts and a surface that flickers.
	box = box.Background(hoverColor(c, box, "icon-bg", ui.Transparent, theme.SurfaceHover, box.Hovered()))
	// An icon button gives to .9 over .16s when pressed.
	box = pressScale(box, "press", PressScaleIcon, MotionPress)
	// The name is a Label, not just a tooltip: an accessible name has to sit beside the
	// hover text. A tooltip alone is a hover hint -- the tester's Find, which answers
	// "anything that carries a text or an accessibility label", cannot see these controls,
	// and neither would a screen reader. Label and Tooltip say the same words, the way
	// settingsIcon does.
	box.Draw(func(p *ui.Painter, r ui.Rect) {
		size := min32(r.W, r.H) * 0.6
		drawIcon(p, name, r.X+(r.W-size)/2, r.Y+(r.H-size)/2, size, theme.Text)
	}).
		Label(tooltip).
		Tooltip(tooltip).
		OnClick(onClick)
}
