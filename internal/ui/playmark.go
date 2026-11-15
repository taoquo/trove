package ui

import (
	"github.com/egoist/mygo/ui"
)

// The video mark.
//
// A video's poster is a still frame, so on its own it is a photo to the eye: the same card, the
// same thumbnail, the same picture in the detail. One disc over it says "this one moves", and
// the disc is the same mark everywhere, scaled to what it sits on. Photos carry nothing -- the
// absence of the mark is what a photo looks like, and a second mark for the majority would only
// add ink to every card that needed none.
//
// The disc is a mark, not a control: every picture opens the same way (a card opens the detail,
// the detail opens the lightbox) and the player lives in the lightbox, so the press that follows
// is the same press it would be without the mark.
//
// It is drawn in the accent rather than in the neutral the lightbox controls use, because it is
// the one thing on a still frame that acts: a translucent dark disc disappeared into a dark
// poster, and the mark exists to be noticed at a glance while scrolling. The accent also means
// the mark follows the system accent and the appearance -- cinnabar in the light theme, its
// lighter step in the dark one -- instead of being a second hardcoded colour.

// playGlyphRatio is the triangle's width against the disc's. It is over a third of the disc,
// because a glyph sized for the old dark ground read small once the disc became the accent's
// solid fill and became the thing the eye lands on.
const playGlyphRatio = 0.42

// playGlyphSize keeps the glyph in proportion on every disc, so a card's mark is the same
// drawing smaller rather than a second drawing.
func playGlyphSize(size float32) float32 {
	return size * playGlyphRatio
}

// playMark draws the disc over a video's poster, centred in the box it is called inside.
//
// The size is the caller's, because the mark sits on three surfaces of different sizes: a grid
// card's 16:9 band, the list card's inline thumbnail, and the detail's picture. One size cannot
// serve all three -- 64 points fills a 108-point thumbnail whole, and 30 points is a speck on a
// 408-point picture.
func (s *Shell) playMark(c *ui.Context, theme *ui.Theme, size float32, label string) {
	ui.Box(c).Absolute().Left(0).Top(0).Right(0).Bottom(0).Center().PassThrough().Children(func() {
		ui.Box(c).Width(size).Height(size).Radius(RadiusPill).Background(theme.Accent).
			Center().Label(label).Children(func() {
			glyph := playGlyphSize(size)
			ui.Box(c).Width(glyph).Height(glyph).
				Draw(func(p *ui.Painter, r ui.Rect) {
					drawIcon(p, "play", r.X, r.Y, min32(r.W, r.H), theme.AccentText)
				})
		})
	})
}
