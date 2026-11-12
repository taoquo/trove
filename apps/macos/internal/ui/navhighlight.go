package ui

import "github.com/egoist/mygo/ui"

// The sidebar's selection shape, as magpie draws it (`.nav-hl`, app.css:478-484).
//
// The shape belongs to the list, not to the row: it is one element that slides from row to
// row, so a change of selection reads as the ground moving rather than one row lighting up
// as another goes dark (app.js navSubMark, navSubGlide). That is also why a sub-row paints
// no ground of its own in any state -- "the selection is the only shape in the list: a row
// hovered or pressed has no ground of its own" (app.css:449-451).
//
// MyGo has no z-index, and an absolute child paints over the flow, so the shape cannot be a
// child of the list. It is painted by the list column itself, before its children -- the
// layer the source's z-index 0 gives it. Its place comes from the committed boxes of the
// column and of the selected row, which is the offsetTop the source reads: both carry the
// scroll's translation, so the difference is the row's place in the list whether or not the
// list has been scrolled.
const (
	// NavHighlightRadius is `.nav-hl`'s 6-point radius, the row's own (app.css:482).
	NavHighlightRadius = 6
	// NavHighlightTint is `color-mix(in srgb, var(--fg) 6.5%, transparent)` (app.css:482).
	NavHighlightTint = 0.065
)

// navHighlight attaches the list's selection shape, marking row.
//
// The shape spans the row it marks. `.nav-hl` asks for `left: 8px; right: 0` (app.css:482),
// but left is measured from `.nav-sub`'s padding box and the list carries the matching
// `padding-left: 8px` (app.css:438), so in the source the shape starts exactly at a row's
// left edge and ends at its right. This list has no such padding -- the column's inset is
// the list's own -- so the shape is the row's own width.
//
// A list with nothing chosen draws no shape. The list's first frame has no committed boxes
// yet, and a window at rest asks for no second frame, so the shape cannot wait for one: the
// chosen row paints the same tint itself that once, and the list owns it from the next
// frame. The first value AnimateTo returns is its target, which is the source's `.still`:
// a list drawn fresh puts the shape in place instead of sending it there (app.js
// navSubMark).
func navHighlight(c *ui.Context, list, row ui.Element, theme *ui.Theme) ui.Element {
	if !row.Valid() {
		return list
	}
	ink := theme.Text.Alpha(NavHighlightTint)
	rowBox, listBox := row.Bounds(), list.Bounds()
	if rowBox.H <= 0 || listBox.H <= 0 {
		// The row is the shape's own geometry, so this is the same mark, one frame early.
		row.Background(ink)
		return list
	}
	top := list.AnimateTo("nav-highlight-top", rowBox.Y-listBox.Y, MotionGlide)
	height := list.AnimateTo("nav-highlight-height", rowBox.H, MotionGlide)
	return list.Draw(func(p *ui.Painter, r ui.Rect) {
		if r.W < 1 || height < 1 {
			return
		}
		p.Fill(ui.Rect{X: r.X, Y: r.Y + top, W: r.W, H: height}, ink, NavHighlightRadius)
	})
}
