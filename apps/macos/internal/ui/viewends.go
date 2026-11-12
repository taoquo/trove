package ui

import (
	"math"

	"github.com/egoist/mygo/ui"
	"github.com/taoquo/trove/apps/macos/internal/i18n"
)

// The scroll-ends capsule: `scrollends.js`, `.view-ends` (app.css:2255-2266).
//
// A 26-wide tray at the page's lower right with a 24-high chevron above and below, a 14-point
// hairline between them, and a 9-point radius over a 2-point inset. It appears once the page
// is a screen down, as Safari's does, and its two controls jump to either end. The stylesheet
// gives the tray `--pop-bg` and a 20px backdrop blur; MyGo has no blur, so the tray is the
// card colour with the same edge and drop.
const (
	ViewEndsWidth        = 26
	ViewEndsButtonHeight = 24
	ViewEndsPad          = 2
	ViewEndsRadius       = 9
	ViewEndsButtonRadius = 7
	ViewEndsGlyph        = 14
	// ViewEndsSeparator is the `.ve-sep` hairline between the two controls.
	ViewEndsSeparator = 14
	// ViewEndsInset is the tray's distance from the page's right edge. The stylesheet's
	// `right: 16px` is overridden at runtime by its own script, which places the tray 10
	// points inside the open view's right edge (scrollends.js's `paint`).
	ViewEndsInset = 10
	// ViewEndsBottomInset is the script's 16 above the view's bottom.
	ViewEndsBottomInset = 16
	// ViewEndsRise is the 4-point drop the tray lifts from as it arrives
	// (`.view-ends { transform: translateY(4px) }`, app.css:2257).
	ViewEndsRise = 4
	// ViewEndsHeight is the tray's own height: two controls, the hairline between them and
	// its two margins, inside the tray's padding. scrollends.js measures it to decide when
	// the tray has to stand down at the page's end.
	ViewEndsHeight = 2*ViewEndsPad + 2*ViewEndsButtonHeight + 3
	// ViewEndsEndGap is the room the script keeps between the tray and the last row
	// (`box.offsetHeight + 24`).
	ViewEndsEndGap = 24
	// ViewEndsGutterMin is the right padding that counts as a gutter: the script's
	// `pad >= box.offsetWidth + 16`. Wider than this, the tray sits in the margin and needs
	// no stand-down.
	ViewEndsGutterMin = ViewEndsWidth + 16
)

// viewEnds draws the tray over the results list.
//
// It is mounted for as long as the list can scroll, and fades in once the reader is a screen
// down; while it is out of sight it takes no press. The offset it reads is the same
// ScrollState the infinite feed tracks, so the two always agree on where the reader is.
func (s *Shell) viewEnds(c *ui.Context, theme *ui.Theme, rail RailTokens) {
	scroll := s.resultsScrollState()
	// A list shorter than its pane has nowhere to go: MaxY 0 is the toolkit's own answer to
	// "the content fits", and the capsule must not appear over a page that does not scroll.
	if scroll.MaxY <= 0 {
		return
	}

	_, windowHeight := c.Size()
	inset := s.pageInset(c)
	gutter := inset >= ViewEndsGutterMin
	down := viewEndsShown(scroll, windowHeight, gutter)

	tray := ui.Box(c).Absolute().Right(viewEndsRight(inset)).Bottom(ViewEndsBottomInset).
		Padding(ViewEndsPad).Radius(ViewEndsRadius).Background(theme.Surface).
		Shadow(0, 4, 14, 0, ui.RGBA(20, 22, 30, 0.12)).
		Shadow(0, 0, 0, 0.5, rail.SubtleLine)
	// The first call starts at its target, so a list opened already scrolled shows the tray
	// without a fade -- and a fresh one, at rest, stays invisible until the reader moves.
	shown := float32(0)
	if down {
		shown = 1
	}
	// One value drives the fade and the lift, so the two cannot disagree; `reduceMotion`
	// is inside Animate, which jumps straight to the target (motion.go's note).
	arriving := tray.Animate("view-ends", shown, MotionBase.Duration)
	tray = tray.Opacity(arriving).
		Transform(ui.Translation(0, (1-arriving)*ViewEndsRise))
	if !down {
		// Out of sight, and out of the pointer's way: without this the invisible tray would
		// still swallow presses over the list's corner.
		tray = tray.PassThrough()
	}

	tray.Children(func() {
		s.viewEndsButton(c, theme, rail, true, scroll.Y < 1, scroll)
		ui.Box(c).Width(ViewEndsSeparator).Height(1).Background(rail.SubtleLine).
			Margin(1, 0, 1, 0)
		s.viewEndsButton(c, theme, rail, false, scroll.Y >= scroll.MaxY-1, scroll)
	})
}

// viewEndsShown reports whether the tray belongs on screen: the list scrolls at all, the
// reader is a screen down (`scrollends.js`'s `scrollTop > clientHeight`), and -- in a page
// with no right gutter to sit in -- the end is still far enough off that the tray cannot
// cover the last row (`scrollHeight - clientHeight - scrollTop < box.offsetHeight + 24`
// stands it down).
//
// The toolkit does not expose the scroll's own viewport, so the window's content height
// stands in for it: the page's head takes roughly a toolbar off that, which makes the capsule
// arrive at most one toolbar later than the stylesheet's rule, never earlier.
func viewEndsShown(scroll *ui.ScrollState, windowHeight float32, gutter bool) bool {
	if scroll.MaxY <= 0 || scroll.Y <= windowHeight-TitleBarHeight {
		return false
	}
	if gutter {
		return true
	}
	return scroll.MaxY-scroll.Y >= ViewEndsHeight+ViewEndsEndGap
}

// viewEndsRight is how far the tray sits from the page's edge.
//
// When the page's right padding is wide enough to hold it, the tray goes in that margin,
// centred (`(pad - box.offsetWidth) / 2`); otherwise it is 10 points inside the corner.
// scrollends.js's `paint` makes the same choice on every frame.
func viewEndsRight(inset float32) float32 {
	if inset >= ViewEndsGutterMin {
		return (inset - ViewEndsWidth) / 2
	}
	return ViewEndsInset
}

// viewEndsButton is one end of the tray.
func (s *Shell) viewEndsButton(c *ui.Context, theme *ui.Theme, rail RailTokens,
	up, disabled bool, scroll *ui.ScrollState) {
	ink := theme.TextMuted
	if disabled {
		ink = rail.TextQuaternary
	}
	label := i18n.ActionScrollBottom
	if up {
		label = i18n.ActionScrollTop
	}
	button := ui.Box(c).Width(ViewEndsWidth).Height(ViewEndsButtonHeight).
		Radius(ViewEndsButtonRadius).Center().Label(s.t().T(label)).
		Tooltip(s.t().T(label)).Disabled(disabled)
	if !disabled && button.Hovered() {
		button = button.Background(theme.SurfaceHover)
	}
	if !disabled {
		button = button.OnClick(func() {
			if up {
				scroll.ScrollTo(0, 0, MotionSlow)
				return
			}
			// MaxFloat32 is the toolkit's "the end", whatever the content's height turns
			// out to be (mygo's ScrollState).
			scroll.ScrollTo(0, math.MaxFloat32, MotionSlow)
		})
	}
	button.Draw(func(p *ui.Painter, r ui.Rect) {
		viewEndsChevron(p, r, up, ink)
	})
}

// The tray's two carets are scrollends.js's own inline SVGs, not Phosphor icons:
//
//	svg("m4.5 9.5 3.5-3.5 3.5 3.5", 14, 1.7)
//	svg("m4.5 6.5 3.5 3.5 3.5-3.5", 14, 1.7)
//
// app.js's `svg` writes them into a 16-unit box with a stroked `currentColor` path, so the
// documents below are the same shape at the same weight. Drawing them by hand with lines is
// not possible in MyGo -- Painter.Line paints a horizontal or vertical bar, not a diagonal --
// and a filled phosphor caret would be a different glyph.
var (
	viewEndsCaretUp   = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16"><path d="m4.5 9.5 3.5-3.5 3.5 3.5" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/></svg>`))
	viewEndsCaretDown = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16"><path d="m4.5 6.5 3.5 3.5 3.5-3.5" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/></svg>`))
)

// viewEndsChevron draws the caret, up or down, at the button's middle. The 16-unit document
// is drawn at 14 points, so its 1.7 stroke arrives at the same 1.49 the browser gives it.
func viewEndsChevron(p *ui.Painter, r ui.Rect, up bool, colour ui.Color) {
	glyph := viewEndsCaretDown
	if up {
		glyph = viewEndsCaretUp
	}
	size := float32(ViewEndsGlyph)
	p.Icon(glyph, ui.Rect{
		X: r.X + (r.W-size)/2,
		Y: r.Y + (r.H-size)/2,
		W: size,
		H: size,
	}, colour)
}
