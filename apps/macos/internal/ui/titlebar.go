package ui

import (
	"github.com/egoist/mygo/ui"
	"github.com/taoquo/trove/apps/macos/internal/i18n"
)

// The window's own title bar.
//
// magpie draws its chrome rather than taking the system's: the window has no title bar of
// its own, the header is 50 points tall and starts at the traffic lights, and the sidebar
// owns the left end of that band (`body.window.mac .top { height: 50px; padding-left: 96px }`,
// app.css:210; `body.window.mac .side-top`, app.css:391). The sidebar's button is the first
// thing in the row, then the brand, so the button is in one place whether the sidebar shows
// or not -- as Finder's and Notes' are (index.html:54-58).
//
// This is the same structure in MyGo's terms. The bar is one row, split at the sidebar's
// edge: the left segment is the sidebar's own top band (its ground, its edge carried to the
// window's top), the right segment is the header over the results. When the column is hidden,
// the left segment goes with it, and the button, the brand and the page's crumb all start
// from the bar's own inset -- magpie moves the same node between the two hosts for this
// reason (app.js layoutNav). Both segments drag the window, which is what makes the band feel
// like a title bar; a double click zooms it (MyGo's DragWindow handles both).
//
// It is drawn instead of the system title bar, so the window style has to say so: main.go
// passes TitleBarHiddenInset with the traffic lights centred in this height.
func (s *Shell) titleBar(c *ui.Context, theme *ui.Theme, collapsed bool, track float32) {
	bar := c.TitleBar()
	height := float32(TitleBarHeight)
	if bar.Height > height {
		height = bar.Height
	}

	// Where the row's content starts. The traffic lights take the first 72 points on a Mac
	// and magpie puts the button 24 past them, which is its 96 (app.css:210). A window with
	// no controls -- full screen, a capture, the tests -- has nothing to keep clear of, so it
	// keeps only the bar's own inset, and the button sits there in both sidebar states.
	inset := bar.Left + TitleBarLightsGap
	if bar.Left == 0 {
		inset = TitleBarFlush
	}
	rightPad := float32(TitleBarPadRight)
	if bar.Right > 0 {
		rightPad = bar.Right + TitleBarPadRight
	}

	// The band's left segment ends at the split's line, which is the sidebar's own edge: the
	// twelve-point track between the column and the line is gone (S48), so the bar's left
	// segment is exactly the column.
	edge := track

	// Shown, the right segment starts at the column's edge and its crumb is 20 in from it
	// (`body.side-nav .top`, app.css:522-524). Hidden, the results own the whole width and
	// the row's content starts at the bar's own inset, clear of the window's controls.
	foldedPad := float32(TitleBarCrumbPad)
	if collapsed {
		foldedPad = inset
	}

	row := ui.Row(c).FillWidth().Height(height)
	// The button's pane fades over 300ms as the column folds -- `.side-toggle .st-pane`'s
	// `transition: opacity .3s` (app.css:525-526). The value is animated here, on the bar's
	// own row, and not on the button: the button moves from the left segment into the right
	// one when the column folds, so it is a different element on the two sides of the change
	// and an animation started on it would snap to its target. The row is in the same place
	// in the tree either way.
	paneTarget := float32(0)
	if !collapsed {
		paneTarget = SidebarGlyphPane
	}
	pane := row.AnimateTo("sidebar-pane", paneTarget, MotionSlow)

	row.Children(func() {
		// The sidebar's own top band. Its hairline is the column's edge carried to the
		// window's top, so the sidebar's boundary runs the window's whole height instead of
		// starting under the header. Hidden, there is no band to draw.
		if !collapsed {
			ui.Row(c).Width(edge).Height(height).DragWindow().
				Padding(0, TitleBarPadRight, 0, inset).
				AlignItems(ui.Center).GapX(TitleBarBrandGap).
				BorderWidth(0, 1, 0, 0).BorderColor(theme.Border).Children(func() {
				s.sidebarToggle(c, theme, pane)
				s.brandLockup(c, theme,
					edge-inset-TitleBarButton-TitleBarBrandGap-TitleBarPadRight)
			})
		}

		// The header over the results. Hidden, the button and the brand are here instead of
		// in the sidebar's band, at the bar's own inset.
		ui.Row(c).Grow(1).FillHeight().DragWindow().AlignItems(ui.Center).
			GapX(TitleBarBrandGap).Padding(0, rightPad, 0, foldedPad).Children(func() {
			if collapsed {
				s.sidebarToggle(c, theme, pane)
				s.brandLockup(c, theme, TitleBarBrandFoldedWidth)
			}
			s.pageCrumb(c, theme)
		})
	})
}

// sidebarToggle is the sidebar's button, in the title bar where a Mac app has it.
//
// Its glyph is magpie's: an outer rounded rectangle, the divider, and the left pane filled
// while the sidebar shows (`#sideToggle`, index.html:58; `.side-toggle .st-pane`, app.css:525).
// It used to be a back arrow in the footer between export and reset, which read as "go back"
// among the file commands; here it says what it does -- the column -- and stays put while the
// column comes and goes.
func (s *Shell) sidebarToggle(c *ui.Context, theme *ui.Theme, pane float32) {
	open := !s.sidebarCollapsed()
	label := i18n.ActionExpandSidebar
	if open {
		label = i18n.ActionCollapseSidebar
	}
	box := ui.Box(c).Width(TitleBarButton).Height(TitleBarButton).
		Radius(RadiusMedium).Focusable()
	box = box.Background(hoverColor(c, box, "toggle-bg", ui.Transparent, theme.SurfaceHover, box.Hovered()))
	// An icon control, so `.icon:active`'s .9 (app.css:4053).
	box = pressScale(box, "press", PressScaleIcon, MotionPress)
	// Label is the accessible name and Tooltip the same words on hover: a glyph-only control
	// needs both, and the name is also what a test can find the button by.
	box.Draw(func(p *ui.Painter, r ui.Rect) {
		drawSidebarGlyph(p, r, theme.Text, pane)
	}).Label(s.t().T(label)).Tooltip(s.t().T(label)).OnClick(func() { s.ToggleSidebar() })
}

// drawSidebarGlyph draws the sidebar's shape: a rounded frame on a 16-point grid, the divider
// at 6.75, and the left pane filled at pane's alpha -- 32% when the column is open, zero when
// it is folded, and partway between while it moves (index.html:58).
func drawSidebarGlyph(p *ui.Painter, r ui.Rect, ink ui.Color, pane float32) {
	size := min32(r.W, r.H) * 0.6
	scale := size / 16
	x := r.X + (r.W-size)/2
	y := r.Y + (r.H-size)/2
	// The frame: 1.75,2.75 to 14.25,13.25, radius 2.25, stroke 1.25.
	p.Stroke(ui.Rect{X: x + 1.75*scale, Y: y + 2.75*scale, W: 12.5 * scale, H: 10.5 * scale},
		ink, 2.25*scale, 1.25*scale)
	if pane > 0.001 {
		// The filled left pane: 2.5,3.5 to 6.5,12.5, radius 1.
		p.Fill(ui.Rect{X: x + 2.5*scale, Y: y + 3.5*scale, W: 4 * scale, H: 9 * scale},
			ink.Alpha(pane), 1*scale)
	}
	// The divider under the pane's edge, at x 6.75.
	p.Fill(ui.Rect{X: x + 6.125*scale, Y: y + 2.75*scale, W: 1.25 * scale, H: 10.5 * scale},
		ink, 0)
}

// brandLockup is the mark and the name, the pair the title bar starts with in either layout.
// maxWidth is what the row has left for the words; a narrow column keeps the mark and drops
// the name, as magpie's fitTop does (`.side-top.tight .brand`, app.css:388).
func (s *Shell) brandLockup(c *ui.Context, theme *ui.Theme, maxWidth float32) {
	brandLogoElement(c)
	if maxWidth < TitleBarBrandMinText {
		return
	}
	ui.Text(c, s.t().T(i18n.AppTitle)).
		Font(s.displayFace()).FontSize(FontSidebarTitleCompact).FontWeight(600).
		LineHeight(SidebarTitleLineHeight).MaxLines(1).MaxWidth(maxWidth).
		LetterSpacing(trackingEm(0.01, FontSidebarTitleCompact)).
		TextColor(theme.Text)
}

// pageCrumb names the page the reader is on, in the title bar beside the brand.
//
// magpie's `.crumb` is where the window says "Work / Library / MCP servers" (app.css:520-527):
// the pages have no headings of their own, so the reader's place lives in the chrome. This
// build draws the current page's name alone at 13.5/600 -- the path prefix would name a
// hierarchy the sidebar already shows.
func (s *Shell) pageCrumb(c *ui.Context, theme *ui.Theme) {
	label := s.scopeTitle()
	switch {
	case s.state.SettingsOpen:
		label = s.t().T(i18n.SettingsTitle)
	case s.state.TagMode:
		label = s.t().T(i18n.TagManageLabel)
	case s.state.GraphMode:
		label = s.t().T(i18n.ToolbarGraph)
	}
	ui.Text(c, label).FontSize(FontPageCrumb).FontWeight(600).
		MaxLines(1).TextColor(theme.Text)
}
