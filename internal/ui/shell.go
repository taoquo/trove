package ui

import (
	"context"
	"os"

	"github.com/egoist/mygo/ui"
	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/i18n"
)

// currentViewSnapshot is the filter state a saved view captures.
//
// It holds the query, the tags, the authors, the two switches, the order and the layout.
func (s *Shell) currentViewSnapshot() domain.SavedView {
	snapshot := domain.SavedView{
		Query:         s.state.Query,
		OnlyWithMedia: s.state.OnlyWithMedia,
		OnlyLongform:  s.state.OnlyLongform,
		SortOrder:     s.state.Sort,
		ViewMode:      s.state.ViewMode,
	}

	// The tag and author facets are copied into the snapshot. The shell narrows by one
	// tag selection at a time, so a saved view captures the current set rather than a
	// rule for combining several.
	snapshot.ActiveTagIDs = append([]string(nil), s.state.ActiveTagIDs...)
	snapshot.ActiveAuthorHandles = append([]string(nil), s.state.ActiveAuthors...)
	return snapshot
}

// SaveCurrentView stores the current filters under the field's name.
//
// An empty field falls back to the default name rather than refusing: that name is
// the literal "Saved view".

// View draws the workspace.
//
// It has no side effects: it reads the state and returns elements. Anything that
// changes state is attached to an event handler, which MyGo runs outside the
// build pass.

// listHasKeyboard reports whether the bookmark list is the surface on screen.
//
// It gates the list's global keys: while any overlay or another page covers the list, the
// arrows, Enter and Escape belong to that surface, not to rows nobody can see.
func (s *Shell) listHasKeyboard() bool {
	return !s.state.SettingsOpen && !s.state.SuggestionsOpen && !s.state.TagMode &&
		!s.state.GraphMode && !s.state.ReviewOpen && !s.state.DetailOpen && !s.LightboxOpen()
}

// escape leaves the surface on top, the way every other desktop window behaves.
//
// Only the topmost thing closes, so one press undoes one step: the lightbox over the
// detail, the detail over the tag pages, the tag pages over the results. The search box is
// last because it is part of the results surface, not a layer over it.
//
// A confirmation is a platform AlertDialog and never reaches here: MyGo keeps the window's
// shortcuts behind a modal, and the dialog already treats Escape as its dismiss gesture.
func (s *Shell) escape(ctx context.Context) {
	switch {
	case s.LightboxOpen():
		s.CloseLightbox()
	case s.state.DetailOpen:
		s.CloseDetail()
	case s.state.ReviewOpen:
		s.state.ReviewOpen = false
		s.invalidate()
	case s.state.SettingsOpen:
		// The find field first, then the page: one press takes back the narrowing, the
		// next returns to the bookmarks, the same one-layer-at-a-time rule the rest of
		// the cascade follows.
		if s.state.SettingsQuery != "" {
			s.state.SettingsQuery = ""
			s.invalidate()
			return
		}
		s.CloseSettings()
	case s.state.SuggestionsOpen:
		s.state.SuggestionsOpen = false
		s.invalidate()
	case s.state.GraphMode:
		// Leaving the graph reloads the results it was covering, as the graph's own
		// toggle does; flipping the flag alone would reveal whatever the list last held.
		s.state.GraphMode = false
		s.ReloadAsync(ctx)
	case s.state.TagMode:
		s.ToggleTagMode(ctx)
	case len(s.state.SelectedIDs) > 0:
		// The ticked boxes are the last thing Escape undoes before the search: one press
		// takes back the bulk selection, the next clears the query (magpie's cascade --
		// the sheet, then the search, then the selection).
		s.ClearBatchSelection()
	default:
		s.ExitSearch(ctx)
	}
}

// View draws the workspace.
//
// It has no side effects: it reads the state and returns elements. Anything that
// changes state is attached to an event handler, which MyGo runs outside the
// build pass.
func (s *Shell) View(c *ui.Context) {
	// A build pass is the unit "in use" is measured in: a picture drawn this time may not
	// be dropped to make room for one drawn next time.
	s.draw.beginFrame()

	theme := FolioTheme(s.state.Appearance, s.state.SystemDark)
	c.SetTheme(theme)

	// Keyboard navigation. Only while the result list is the surface on screen.
	//
	// A modal handles Escape itself, so the shortcuts used to be skipped for the settings
	// sheet and the suggestions panel only. The tag pages, the graph, the review sheet and
	// the lightbox all cover the list too: Enter there opened the detail of a row the reader
	// could not see, Escape cleared a search behind a panel, and the arrows moved a selection
	// nothing was drawing.
	ctx := context.Background()
	if s.listHasKeyboard() {
		// Up and down move by a row, which in the two-column grid means two cards; left
		// and right move by one. Home and End go to the ends of the loaded page, PageUp
		// and PageDown by a screenful, and Space ticks the selected card's batch box
		// without opening it -- the set of keys magpie's own lists answer to.
		c.OnShortcut(0, ui.KeyDown, func() { s.MoveSelection(1) })
		c.OnShortcut(0, ui.KeyUp, func() { s.MoveSelection(-1) })
		c.OnShortcut(0, ui.KeyLeft, func() { s.MoveSelectionBy(-1) })
		c.OnShortcut(0, ui.KeyRight, func() { s.MoveSelectionBy(1) })
		c.OnShortcut(0, ui.KeyHome, func() { s.SelectFirstResult() })
		c.OnShortcut(0, ui.KeyEnd, func() { s.SelectLastResult() })
		c.OnShortcut(0, ui.KeyPageUp, func() { s.PageSelection(-1) })
		c.OnShortcut(0, ui.KeyPageDown, func() { s.PageSelection(1) })
		c.OnShortcut(0, ui.KeySpace, func() { s.ToggleSelectedBatch() })
		// Shift-Cmd-A rather than Cmd-A: the Edit menu already claims Cmd-A for
		// `selectAll:` (main.go:467), and a menu key equivalent is dispatched by AppKit
		// before the window sees the key -- so a window shortcut on Cmd-A would never
		// fire while the menu item would still swallow the key in the search field.
		// Shift-Cmd-A is the same idea with a modifier the menu does not own.
		c.OnShortcut(ui.Super|ui.Shift, ui.KeyA, func() { s.SelectAllResults(ctx) })
		c.OnShortcut(0, ui.KeyEnter, func() { s.OpenSelectedDetail() })
	}
	// The sidebar hides from anywhere in the window, on magpie's own binding:
	// its button's shortcut is Control-Command-S. The key is
	// registered outside the list gate because it is a window-level layout command, not a
	// list one: it must work while the detail, the graph or a settings page has the surface.
	// The View menu carries the same command, which is how it is discoverable; a menu key
	// equivalent is dispatched before the window sees the key, so in the app the menu is what
	// fires and this one is what answers in a window without that menu (the tests).
	c.OnShortcut(ui.Ctrl|ui.Super, ui.KeyS, func() { s.ToggleSidebar() })
	// Escape is registered whether or not a panel is open: it closes the surface on top.
	// It used to be part of the block above, so with a panel open Escape did nothing -- a
	// reader who opened the settings sheet, the detail or the graph could only leave with
	// the pointer. A text field inside the panel that handles Escape itself (the tag
	// editors, both search fields) keeps it: MyGo routes a key to the innermost element
	// around the focus first, and only the unclaimed one reaches the window.
	c.OnShortcut(0, ui.KeyEscape, func() { s.escape(ctx) })
	rail := Rail(s.state.Appearance, s.state.SystemDark, s.translator.Locale() == i18n.ZHCN)

	// The workbench is two panes, not three: there is one split handle, on the left, and the
	// inspector is rendered inside the detail overlay.
	// A column: the window's own title bar, then the workbench. The title bar is drawn rather
	// than taken from the system; main.go hides the system one and keeps the traffic lights
	// over this row.
	root := ui.Column(c).Fill().Background(theme.Background)

	// The overlays' entrances are driven on the root, which is built whether or not an
	// overlay is open. An animation started on the frame an overlay first appears would
	// begin at its target and never play.
	reduceMotion := c.Preferences().ReduceMotion
	// A screenshot asks for a state, not for an animation mid-flight, and the entrance never
	// advances on that path: View builds once and neither Invalidate nor Update makes it build
	// again. So a panel gated by an entrance would be photographed at progress 0, which is
	// Opacity(0) -- invisible. That is why the detail overlay could not be captured, and why
	// tools/panes.sh's narrow assertion had been passing on the absence of the panel rather
	// than on its shape.
	//
	// reduceMotion is the framework's own switch for exactly this: EntranceProgress returns the
	// target at once when it is set (entrance.go:115). The system's setting is not ours to
	// change, so on the capture path it is forced -- which is the honest reading of "photograph
	// the state", and it is confined to the one path that never animates.
	//
	// ⚠️ It does not fix the detail overlay: with reduceMotion forced, the overlay is still
	// byte-for-byte identical to a capture with no detail at all, while the settings modal at
	// the same width renders fine. So the entrance value is not the cause, and this stays only
	// because it is the right thing to do on a path that never animates -- not because it
	// helped.
	if os.Getenv("TROVE_CAPTURE") != "" {
		reduceMotion = true
	}
	reviewEntrance := EntranceProgress(root, "entrance-review",
		s.state.ReviewOpen, EntranceDetailCard, reduceMotion)
	suggestionsEntrance := EntranceProgress(root, "entrance-suggestions",
		s.state.SuggestionsOpen, EntranceDetailCard, reduceMotion)
	detailEntrance := EntranceProgress(root, "entrance-detail",
		s.state.DetailOpen, EntranceDetailCard, reduceMotion)
	// The toast has its own entrance because the stylesheet gives it its own animation:
	// 180ms, a 4px lift, and a different duration from the card it sits over. Its open state is
	// whether there is a message to show, which ToastText already decides.
	// The backdrop fades over the detail's own duration and curve, and it is a separate
	// progress because a progress value is computed against one Entrance.

	root.Children(func() {
		// magpie hides the column rather than folding it; the header's button brings it back.
		// Which of the two is drawn is a read of state -- the toggle persists the choice.
		collapsed := s.sidebarCollapsed()
		track := s.sidebarPaneWidth()

		// The window's chrome, above both panes. It replaces the system title bar, so it is
		// the first row of the window and the workbench takes the rest of the height.
		s.titleBar(c, theme, collapsed, track)

		ui.Row(c).FillWidth().Grow(1).Children(func() {
			if collapsed {
				// No column and no handle: ui.Split's line is the sidebar's edge, and a
				// zero-width pane would draw that line at the window's left edge.
				s.resultsPage(c, theme, rail)
				return
			}
			// .Fill() is required: without it the split takes its natural width and the window
			// keeps a wide empty strip on the right -- about 40% of the window blank.
			//
			// The split sits at the sidebar's own width. magpie's sidebar is fixed with a
			// `border-right` (app.css:372-375), and ui.Split's one-point line is that border;
			// the twelve-point middle track is gone.
			pane := track
			outer := ui.Split(c, &pane, func() {
				ui.Row(c).Fill().Children(func() {
					ui.Column(c).Width(track).FillHeight().Children(func() {
						s.sidebar(c, theme, rail)
					})
				})
			}, func() {
				// The detail is a third pane when there is room for it and an overlay when there
				// is not: at 1160-1399 the inspector may appear as a third column as long as the
				// list keeps MainMinWidth; below that it is an overlay rather than a pane that
				// squeezes the list.
				if s.state.DetailOpen && s.inspectorFits(s.sidebarPaneWidth()) {
					// .MinWidth(0) on the middle pane is the whole fix, and it is one call.
					//
					// The middle track is a `minmax(0, 1fr)`. The 0 is not decoration -- a grid
					// item's min-width defaults to auto, so without it the track refuses to
					// shrink below its content's intrinsic width and the row overflows
					// instead. MyGo's Grow(1) has no such override, and Element.MinWidth exists
					// to supply it.
					//
					// Without this the toolbar's intrinsic width holds the middle pane open and
					// pushes the inspector's column off the right edge: measured by scanning a
					// horizontal line, the paint ran unbroken from x=560 to x=2359 when the
					// boundary is at 1752. With it the paint stops at 1751.
					//
					// It stays because it is the translation of a `minmax(0, 1fr)` track and
					// because the margin is 48 points, not a design: a fourth toolbar control
					// would spend it. No check here exercises it, so it is not a guard.
					ui.Row(c).Fill().Children(func() {
						ui.Column(c).Grow(1).MinWidth(0).FillHeight().Children(func() {
							s.resultsPage(c, theme, rail)
						})
						// The boundary the inspector never had.
						//
						// The shell is three tracks with the separator living in the middle
						// one, and it is a single 1px line (--border-subtle) rather than a
						// border on each pane -- do not stack two borders. MyGo paints it with
						// BorderWidth's CSS argument order, top right bottom left, so a left
						// edge is BorderWidth(0, 0, 0, 1).
						//
						// tools/columns.py reads a horizontal line and fails when paint runs
						// across this x; before this line existed it reported the run reaching
						// x=2360 from x=0.
						ui.Column(c).Width(InspectorWidth).FillHeight().
							BorderWidth(0, 0, 0, 1).BorderColor(theme.Border).
							Children(func() {
								s.inspector(c, theme, rail)
							})
					})
					return
				}
				s.resultsPage(c, theme, rail)
			}).Fill()
			// A view body may be rebuilt several times for one event, so the write is
			// guarded by a comparison against what is already stored: repeated builds
			// then cost nothing, and only a real drag reaches the database.
			if outer.Changed() && !collapsed {
				// Dragging well past the minimum hides the column instead of pinning it at the
				// floor: magpie hides below `SIDE_MIN - 64`, and it is the gesture a reader
				// tries when they want the column gone.
				if pane < WidthSidebarMin-SidebarCollapseDrag {
					// A drag past the floor is a pick too: magpie's `sideDrag` calls
					// `toggleSideNav` there, which writes the layout.
					s.state.SidebarCollapsed = true
					s.state.SidebarChosen = true
				} else {
					s.state.LeftSidebarWidth = ClampSidebarWidth(pane, s.windowWidth())
				}
				s.persistLayout()
			}
		})
	})

	// Settings is a page of the results pane, drawn by `results`; a modal over a page the
	// reader can see behind it is not a second way to reach the same settings.
	if s.state.ReviewOpen {
		ui.Modal(c, &s.state.ReviewOpen, func() {
			s.vocabularyReviewPanel(c, theme, reviewEntrance)
		})
	}
	// The overlay, for the windows the third column does not fit in. Both draws are guarded by
	// the same test so exactly one of them happens.
	if s.state.DetailOpen && !s.inspectorFits(s.sidebarPaneWidth()) {
		// DialogBase rather than Modal: its panel is unstyled, so the panel *is* the card.
		// Modal's panel adds 5 spaces of padding and its own background, which would put a
		// second, wider rounded surface behind the card -- visible as a rim on every side and
		// measured as a card ~40 points wider than the stylesheet's 980.
		//
		// The behaviours needed are DialogBase's own: clicking the backdrop and Escape both
		// clear the flag. The backdrop's colour is set here because DialogBase leaves it
		// transparent, and this is magpie's modal scrim: 32% black over the page
		// (`.modal { background: rgba(0, 0, 0, .28) }` plus a blur this toolkit has no
		// equivalent for, app.css:4048-4052). Mixing the page's own colour into the scrim
		// greys the window instead of dimming it.
		ui.DialogBase(c, &s.state.DetailOpen, func(backdrop, panel ui.Element) {
			backdrop.Background(ui.RGBA(0, 0, 0, 0.28))
			s.detailOverlay(c, theme, rail, detailEntrance, panel)
		})
	}
	// Drawn after the detail, at the stylesheet's higher stacking. It is an overlay
	// rather than a Modal because its open state is an index, not a flag: a Modal needs
	// a bool it can clear, and a lightbox that closed to a local would never close the
	// real one.
	//
	// Above the detail, in two steps, and both were needed.
	//
	// The detail is a DialogBase, and a dialog builds into the window's overlay layer, not
	// into the view's own tree (base.go:463, coreOverlay). A row built after it in View is
	// still in the normal tree, so the lightbox was drawn *under* the detail that opened it
	// and its close control could not be pressed; this is why the block is inside Overlay.
	//
	// Overlay alone was not enough: inside the overlay layer MyGo paints in-flow children
	// first and absolute ones after (paint.go:239-250), and the dialog's backdrop is
	// absolute. The lightbox row was in flow, so it still painted first. Both layers below
	// are absolute now, appended after the dialog, so they come last -- and are hit-tested
	// in the same order.
	if s.LightboxOpen() {
		lightboxEntrance := s.lightboxEntrance(root, reduceMotion)
		backdrop := s.lightboxBackdropEntrance(root, reduceMotion)
		imageEntrance := s.lightboxImageEntrance(root, reduceMotion)
		// The dim, and then the card above it.
		//
		// Two absolute siblings rather than one containing the other, because MyGo paints a
		// parent's absolute children after its in-flow ones (paint.go:246): a dim nested in
		// the same box as the card would be drawn last and cover the card instead of sitting
		// behind it.
		ui.Overlay(c, func() {
			// The backdrop is a control whose click closes the preview. A plain box would
			// swallow that press and nothing would happen.
			dim := ui.Box(c).Absolute().Left(0).Top(0).Right(0).Bottom(0).
				Background(rail.Overlay).
				Opacity(EntranceLightboxBackdrop.OpacityAt(backdrop)).
				OnClick(func() { s.CloseLightbox() })
			// Escape and the arrows belong to the preview, not to whatever it covers. An
			// overlay handler registered here is built after the detail's DialogBase backdrop,
			// and MyGo gives the key to the overlay made last, so the top layer closes
			// (scope.go:111-125; routeKeys). The window-level binding this replaces lost to the
			// dialog's own overlay handler.
			if dim.OverlayShortcut(0, ui.KeyEscape) {
				s.CloseLightbox()
			}
			// The preview steps with the arrows only when there is more than one item.
			if len(s.selectedMedia()) > 1 {
				if dim.OverlayShortcut(0, ui.KeyLeft) {
					s.PrevMedia()
				}
				if dim.OverlayShortcut(0, ui.KeyRight) {
					s.NextMedia()
				}
			}
			// The card's own layer passes the pointer through, so a press beside the card
			// still reaches the backdrop and closes the preview rather than doing nothing.
			// Modal, so Tab goes round the preview's controls alone and the detail underneath
			// is inert. It is labelled with the media title; without that a screen reader
			// would announce an unlabelled dialog over the post.
			ui.Box(c).Absolute().Left(0).Top(0).Right(0).Bottom(0).PassThrough().Center().Modal().
				Label(s.t().T(i18n.MediaTitle)).
				Children(func() {
					s.renderLightbox(c, theme, rail, lightboxEntrance, imageEntrance)
				})
		})
	}
	if s.state.SuggestionsOpen {
		ui.Modal(c, &s.state.SuggestionsOpen, func() {
			s.suggestionsPanel(c, theme, suggestionsEntrance)
		})
	}
	// The status line, drawn last so it sits above every panel it might be reporting on.
	//
	// It used to be a dark, inverse pill drawn inside the root's row, which put it under the
	// settings and review modals; magpie's line lifts to the foot of the window whenever the
	// footer is covered (`.status.lift`, app.css:2242), and it carries the outcome's own
	// colour rather than one colour for all three. This shell has no footer, so the pill
	// always floats, at the same inset the stylesheet's lifted state uses (bottom 18).
	if message := s.ToastText(); message != "" {
		toastEntrance := EntranceProgress(root, "entrance-toast",
			s.ToastText() != "", EntranceToast, reduceMotion)
		tone := toastTone(s.state.ToastKind, theme)
		ui.Overlay(c, func() {
			ui.Box(c).Absolute().Left(0).Right(0).Bottom(ToastLiftBottom).Center().PassThrough().
				Opacity(EntranceToast.OpacityAt(toastEntrance)).
				Transform(ui.Translation(0, EntranceToast.OffsetYAt(toastEntrance))).
				Children(func() {
					// The tinted pill: the tone at 16% over the page, with the same tone at
					// 30% as its inset hairline (app.css:2241).
					ui.Text(c, message).
						FontSize(ToastFontSize).FontWeight(500).
						MaxWidth(ToastMaxWidth).
						TextColor(tone).
						Background(tone.Alpha(0.16)).
						Border(1, tone.Alpha(0.30)).
						Padding(ToastPaddingY, ToastPaddingX).
						Radius(ToastRadius)
				})
		})
	}
	s.handleConfirmations(c)
	s.handleListDeleteConfirmation(c)
	s.handleTagDeleteConfirmations(c)
	s.handleResetConfirmation(c)
	s.handleCleanupConfirmation(c)
}

// sidebar draws the left column: the two system views, the lists and the tags.
