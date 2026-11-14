package ui

import (
	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/internal/i18n"
)

// One chip, drawn in the two places a condition has to be visible.
//
// The toolbar row says what is narrowing the list while results are on screen; the empty
// state says the same thing where the reader is looking when the result is nothing. They
// are one method because they are one control: the affordance learned in one place has to
// behave the same in the other, and two copies drift (the toolbar's chip used to print a
// stored value while the sidebar printed a name).

// conditionChip draws one active condition: its name, and one way to remove it.
//
// The label is the only text. The stored value -- a tag uuid, an author handle -- never
// reaches the window, because a chip reading "tag:3f2a…" tells the reader nothing.
func (s *Shell) conditionChip(c *ui.Context, rail RailTokens, chip filterCondition) {
	// A chip: 25 high, radius 999, one hairline, the card ground, a 12px label
	// and a 5/10 inset; hover takes the accent's edge and soft ground.
	// The chip stands in the search's 28-point row without making it taller.
	label, edge, ground := rail.ChipText, rail.ChipBorder, rail.ChipBg
	// The whole chip is the button: one control wraps the label and the x, and pressing it
	// removes the condition. The accessible name states the action, so a screen reader hears
	// "Remove rust" rather than the condition a second time.
	row := ui.Row(c).GapX(SpaceSmall).MinHeight(ChipMinHeight).
		Padding(0, 5, 0, 10).AlignItems(ui.Center).Radius(RadiusPill).MaxWidth(ChipMaxWidth).
		Focusable().
		Label(s.t().Tf(i18n.ActionRemoveNamed, chip.Label)).
		Tooltip(s.t().T(i18n.ActionRemove)).
		OnClick(chip.Remove)
	if row.Hovered() {
		// The accent-soft ground is the badge ground in this rail.
		label, ground, edge = rail.Accent, rail.BadgeBg, rail.Accent
	}
	// The chip has a 120ms transition, so it warms under the pointer instead of flipping to
	// the accent at once.
	row = row.Background(animateColor(c, row, "chip-ground", ground, MotionFast)).
		Border(1, animateColor(c, row, "chip-edge", edge, MotionFast))
	label = animateColor(c, row, "chip-ink", label, MotionFast)
	row.Children(func() {
		// One line, ended with an ellipsis: a condition is a label, not a paragraph. The name
		// can be a display name beside a handle ("@歸藏 (guizang.ai)"), and wrapped it made the
		// chip two lines tall inside the search's row.
		ui.Text(c, chip.Label).FontSize(FontSmall).TextColor(label).SingleLine()
		// The x is drawn, not a second control: it is part of the chip's face, and pressing
		// it lands on the chip's own handler. Two handlers over a facet's toggle would fire
		// twice and leave the condition on, so there is exactly one.
		ui.Box(c).Width(16).Height(16).Radius(RadiusPill).Center().
			Draw(func(p *ui.Painter, r ui.Rect) {
				drawIcon(p, "close", r.X+2, r.Y+2, min32(r.W, r.H)-4, label)
			})
	})
}
