package ui

import (
	"github.com/egoist/mygo/ui"
)

// IconOnly draws a button-shaped square that is just the glyph, for the places the
// extension uses an aria-labelled icon button.
// IconOnly is a bare glyph that takes a surface only while the pointer is over it.
//
// Spec 6.8: "State background: Default transparent / Hover SurfaceHover / Pressed
// SurfacePressed", and "do not show only a tooltip, always give an accessible name". It used to
// be a filled rounded chip in rail.Muted on every toolbar and every row, which is what made the
// toolbar read as a wall of buttons -- and after phase 1 it was a filled chip in the same colour
// as its own glyph.
func IconOnly(c *ui.Context, name, tooltip string, theme *ui.Theme, rail RailTokens, onClick func()) {
	IconOnlySized(c, name, tooltip, theme, rail, IconButtonSize, onClick)
}

// IconOnlySized is IconOnly with the square given, for the places the stylesheet sizes differently.
//
// The sidebar footer's controls are .options-footer-icon-button, which is 28x28 with an 8 radius --
// not the 32 that IconButtonSize carries for a bare icon button. One number was serving both, so
// every footer control was four points too large in each direction.
func IconOnlySized(c *ui.Context, name, tooltip string, theme *ui.Theme, rail RailTokens, size float32, onClick func()) {
	// Focusable: Box + OnClick listens to the pointer only, so every icon-only control was
	// unreachable from the keyboard. A focusable clickable takes Enter and Space.
	box := ui.Box(c).Width(size).Height(size).Focusable().
		Radius(RadiusMedium)
	// The fill eases in and out: magpie gives `.icon` a 150ms background transition
	// (app.css:297), and a chip that appears the instant the pointer crosses it is the
	// difference between a surface that reacts and a surface that flickers.
	box = box.Background(hoverColor(c, box, "icon-bg", ui.Transparent, theme.SurfaceHover, box.Hovered()))
	// `.icon:active` gives to .9 over .16s (app.css:4053, :4048-4052).
	box = pressScale(box, "press", PressScaleIcon, MotionPress)
	// The name is a Label, not just a tooltip. Spec 6.8 asks for an accessible name beside
	// the hover text, and the previous comment here assumed Tooltip carried it -- but a
	// tooltip is a hover hint: the tester's Find, which answers "anything that carries a
	// text or an accessibility label", could not see a single one of these controls, and
	// neither would a screen reader (round 35). Label and Tooltip now say the same words,
	// the way settingsIcon already did.
	box.Draw(func(p *ui.Painter, r ui.Rect) {
		size := min32(r.W, r.H) * 0.6
		drawIcon(p, name, r.X+(r.W-size)/2, r.Y+(r.H-size)/2, size, theme.Text)
	}).
		Label(tooltip).
		Tooltip(tooltip).
		OnClick(onClick)
}
