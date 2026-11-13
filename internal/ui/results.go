package ui

import (
	"context"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/i18n"
)

// resultsPage draws the middle column with the page's entrance, which is what makes a
// navigation read as a change of page rather than a repaint (motion.go, magpie's `view-in`).
//
// The wrapper is always there, even at rest: taking it away when the animation ended would
// move every child up one level in the element tree, which is how a list loses its scroll
// position and a card its animation state.
func (s *Shell) resultsPage(c *ui.Context, theme *ui.Theme, rail RailTokens) {
	opacity, offset := s.resultsEntrance(c)
	page := ui.Column(c).Fill().FillHeight()
	// At rest neither property is set, so a still page is the plain column it was before:
	// no alpha and no transform to carry through every descendant's paint and hit test.
	if opacity < 1 {
		page = page.Opacity(opacity)
	}
	if offset != 0 {
		page = page.Transform(ui.Translation(0, offset))
	}
	page.Children(func() {
		s.results(c, theme, rail)
	})
}

// pageInset is the page's horizontal padding: 16 points, and more when the window is wide
// enough to centre a 1200-point column.
//
// magpie's rule is `padding-left: max(16px, calc(50% - 600px))` on the view (app.css:609), so
// the reading measure does not stretch edge to edge on a large display. The 50% is of the
// pane the page occupies, which is the window minus the sidebar; c.Size() is the window's
// content, not this pane (mygo's Context.Size).
func (s *Shell) pageInset(c *ui.Context) float32 {
	window, _ := c.Size()
	return pageInsetFor(window, s.sidebarPaneWidth())
}

// pageInsetFor is pageInset's arithmetic, split out so the measure can be tested without a
// window: 16 points of padding, or the centring offset once the pane is wider than the column.
func pageInsetFor(window, sidebar float32) float32 {
	return max(float32(ResultsPadX), (window-sidebar-ContentMaxWidth)/2)
}

// results draws the middle column: the toolbar and the list of bookmarks.
func (s *Shell) results(c *ui.Context, theme *ui.Theme, rail RailTokens) {
	// Settings is a page here, not a sheet over this one: it takes the results pane and
	// the sidebar stays where the reader left it.
	if s.state.SettingsOpen {
		s.settingsPage(c, theme, rail)
		return
	}
	if s.state.TagMode {
		s.tagLibraryText(c, theme, rail)
		return
	}
	if s.state.GraphMode {
		s.graphPage(c, theme, rail)
		return
	}

	// One inset for the whole page: `body.window .view { padding: 2px ... 8px 16px }` and the
	// centred column's `max(16px, calc(50% - 600px))` (app.css:602, :609). The blocks inside
	// carry none of their own. The page's name is not here: magpie keeps the reader's place in
	// the title bar's crumb, so the toolbar starts at the top of the page.
	inset := s.pageInset(c)
	ui.Column(c).Fill().FillHeight().Background(rail.Results).
		Padding(ResultsPadTop, inset, ResultsPadBottom, inset).Children(func() {
		s.toolbar(c, theme, rail)
		// Below the toolbar: it is a disclosure the toolbar opens, not a permanent line.
		s.addRow(c, theme, rail)

		s.batchBar(c, theme, rail)
		// The suggestions the workspace is holding, above the feed: work the app is asking for,
		// so it appears whatever is selected, unlike the selection bar above it.
		s.pendingSuggestionsBar(c, theme, rail)

		// A failed action reports through the window's status line, not through a bar here.
		// This block was a second channel for the same sentence: the status line already
		// speaks for a failed tag, export or sync, and the bar said it again under the
		// toolbar while the reader was looking at the results.
		// `state.Err` still means "the library could not be read", which is the empty state's
		// job below.

		// The two states with nothing to scroll are drawn against the viewport, not inside the
		// scroll region. The empty state needs to be centred in the content viewport; inside a
		// scroll container "centred" means centred in the content's own height, which with one
		// short paragraph left the icon in the middle and the advice far below it.
		switch {
		case !s.state.Loaded:
			s.loadingState(c, theme, rail)
		case len(s.state.Data.Bookmarks) == 0:
			s.emptyState(c, theme, rail)
		default:
			if s.state.ViewMode == domain.ViewGrid {
				s.bookmarkGrid(c, theme, rail)
			} else {
				s.bookmarkList(c, theme, rail)
			}
			// The tray floats over the list, in the page's corner, once the reader is a
			// screen down.
			s.viewEnds(c, theme, rail)
		}
	})
}

// batchBar draws the bulk actions for the current selection.
//
// It is only drawn when something is selected, so the toolbar stays quiet until
// the user has chosen what to act on.

// loadingState is the panel drawn while the first read is in flight.
//
// It is a skeleton rather than a spinner: six placeholder cards in the same grid the real
// cards use, so the page does not jump when the read lands.
func (s *Shell) loadingState(c *ui.Context, theme *ui.Theme, rail RailTokens) {
	panel := ui.Column(c).FillWidth().GapY(SkeletonShellGap).Padding(SkeletonShellPad).
		Radius(SkeletonShellRadius).Border(1, rail.SubtleLine).
		Background(rail.PanelStrong)
	// The placeholders breathe while the read is in flight. One value drives every bar, so
	// the panel moves as one object rather than forty parts blinking out of step, and it is
	// the wait's only sign of life: a still skeleton and a hung read look the same.
	breath := pulse(c, panel, "skeleton")
	panel.Children(func() {
		// The head: the state's overline, a title bar and a subtitle bar, with an action
		// block on the right.
		ui.Row(c).FillWidth().GapX(SkeletonShellGap).Children(func() {
			ui.Column(c).GapY(SpaceSmall).Grow(1).Children(func() {
				ui.Text(c, strings.ToUpper(s.t().T(i18n.LoadingFeedTitle))).
					FontSize(FeedStateOverlineFont).FontWeight(600).
					LetterSpacing(trackingEm(FeedStateOverlineTrack, FeedStateOverlineFont)).
					TextColor(rail.TextQuaternary)
				skeletonBar(c, rail, SkeletonHeadBarW, SkeletonHeadBarH, breath)
				skeletonBar(c, rail, SkeletonSubBarW, SkeletonSubBarH, breath)
			})
			skeletonBar(c, rail, SkeletonActionW, SkeletonActionH, breath)
		})

		// Six cards, each a stand-in for the shape a real card has.
		ui.Row(c).FillWidth().GapX(SkeletonGridGap).Wrap().Children(func() {
			for i := 0; i < SkeletonCardCount; i++ {
				ui.Column(c).Width(SkeletonMinCardW).Grow(1).GapY(SpaceSmall).
					Padding(SkeletonCardPad).Radius(SkeletonCardRadius).
					Border(1, rail.SubtleLine).Background(theme.Surface).Children(func() {
					ui.Row(c).FillWidth().GapX(SpaceBase).Children(func() {
						ui.Box(c).Width(SkeletonAvatarSize).Height(SkeletonAvatarSize).
							Radius(RadiusPill).Background(rail.Muted).Opacity(breath)
						ui.Column(c).GapY(SpaceSmall).Grow(1).Children(func() {
							skeletonBar(c, rail, SkeletonNameW, SkeletonNameH, breath)
							skeletonBar(c, rail, SkeletonHandleW, SkeletonHandleH, breath)
						})
					})
					skeletonBar(c, rail, 0, SkeletonLineH, breath)
					skeletonBar(c, rail, SkeletonSubBarW, SkeletonLineH, breath)
					skeletonBar(c, rail, float32(SkeletonSubBarW)*0.8, SkeletonLineH, breath)
					ui.Box(c).FillWidth().Height(SkeletonMediaH).PaddingY(SkeletonMediaTop).
						Radius(RadiusResultCard).Background(rail.Muted).Opacity(breath)
				})
			}
		})
	})
}

// skeletonBar is one placeholder line. A width of zero fills the row, and breath is the
// shared pulse that keeps the panel alive while the read is in flight (motion.go).
func skeletonBar(c *ui.Context, rail RailTokens, width, height, breath float32) {
	bar := ui.Box(c).Height(height).Radius(RadiusPill).Background(rail.Muted).Opacity(breath)
	if width <= 0 {
		bar = bar.FillWidth()
	} else {
		bar = bar.Width(width)
	}
}

// emptyState explains an empty result, which is different for a filter than for
// an empty library.
func (s *Shell) emptyState(c *ui.Context, theme *ui.Theme, rail RailTokens) {
	message := s.t().T(i18n.EmptyLibrary)
	// Each empty state has its own advice, from emptyFeedDescription and
	// noBookmarksDescription.
	detail := s.t().T(i18n.EmptyLibraryDetail)
	if !s.state.Data.Query.Filter.IsZero() {
		// Both lines switch on the same condition, so a filtered miss has its own advice.
		message = s.t().T(i18n.EmptyFilter)
		detail = s.t().T(i18n.EmptyFilterDetail)
	}
	// The read failing is a third state: the icon and the background take the accent colour
	// and the card offers a retry.
	failed := s.state.Err != ""
	if failed {
		message = s.t().T(i18n.LoadFailedTitle)
		detail = s.state.Err
	}

	card := stateCard{
		title:   message,
		detail:  detail,
		failed:  failed,
		onRetry: func() { s.ReloadAsync(context.Background()) },
	}
	// The way out, where the reader is looking. The toolbar still lists the chips above the
	// list, but an empty result is the moment the conditions stop being a convenience and
	// become the reason there is nothing to read: the page repeats them and offers to
	// remove them one at a time, or all at once. A failed read and an empty library have
	// no conditions to clear -- there is nothing the reader narrowed.
	if !failed {
		card.conditions = s.activeConditions()
	}
	s.stateCard(c, theme, rail, card)
}

// stateCard is the panel for "nothing here" and "that failed".
//
// It is a centred column in a rounded panel with a 52px icon block, an overline, a title
// and a copy line, and — when the read failed — a retry.
type stateCard struct {
	title   string
	detail  string
	failed  bool
	onRetry func()
	// actionLabel and onAction are the one way forward the card itself offers ("create a tag",
	// "clear the filters"). The results page reaches its actions from the toolbar; the tag
	// library's page is a table whose empty states have no rows to act on, so the card is where
	// the reader is looking.
	actionLabel string
	onAction    func()
	// conditions are the narrowings that produced an empty result, drawn beneath the
	// advice so "nothing matched" says which conditions it means and how to undo them.
	conditions []filterCondition
}

// stateCard is where "nothing here" and "that failed" are drawn.
//
// "There are no bookmarks" and "the filter matched none" are different states with different
// advice. magpie's empty state is two lines of words and, at most, one quiet action, drawn in
// the page's flow (`.empty-state`, app.css:651-653).
func (s *Shell) stateCard(c *ui.Context, theme *ui.Theme, rail RailTokens, card stateCard) {
	// In the flow, directly under the toolbar: magpie's empty state is part of the page, not
	// a block floated to the middle of the viewport, so the advice reads where the list would
	// have started. The inner column centres its own lines horizontally. With Fill().Center()
	// on the outer column the three parts spread across the whole viewport, which left the
	// title and the advice hundreds of points apart.
	ui.Column(c).FillWidth().Children(func() {
		// magpie's empty state is words only: padding 36/20, the title in the page's ink at
		// weight 500, the advice in --muted, no icon plate competing with the copy
		// (` .empty-state` and ` .empty-state b`, app.css:651-653).
		ui.Column(c).Center().GapY(SpaceSmall).Padding(36, 20).Children(func() {
			colour := theme.Text
			if card.failed {
				colour = theme.Danger
			}
			ui.Text(c, card.title).FontSize(EmptyStateTitleFont).FontWeight(500).
				TextColor(colour)

			if card.detail != "" {
				// No explicit width here, on the Text or on a wrapper column. Both were tried
				// and both broke the group: with a width, this line jumped about three hundred
				// pixels below the title, because a fixed width on a child of a centring column
				// is not laid out the way the surrounding code expects. This is a deliberate
				// deviation with a measurement behind it: the advice runs to its natural width,
				// which for these two strings is one line.
				ui.Text(c, card.detail).FontSize(EmptyStateCopyFont).
					TextColor(theme.TextMuted)
			}

			if card.actionLabel != "" && card.onAction != nil {
				SecondaryButton(c, "", card.actionLabel, theme, rail, card.onAction)
			}

			// The conditions, under the advice that says nothing matched them. They are
			// stacked vertically rather than wrapped: a centring column lays a fixed width
			// child out badly (see the note above on the detail line), and a column reads as
			// a list of reasons -- which is what an empty result is.
			if len(card.conditions) > 0 {
				ui.Column(c).Center().GapY(SpaceSmall).Children(func() {
					for _, condition := range card.conditions {
						s.conditionChip(c, rail, condition)
					}
					settingsAction(c, theme, rail, s.t().T(i18n.FilterClearAll), false, false, func() {
						s.clearConditions(context.Background())
					})
				})
			}

			if card.failed && card.onRetry != nil {
				settingsAction(c, theme, rail, s.t().T(i18n.ActionRetry), false, false, card.onRetry)
			}
		})
	})
}

// scopeTitle is the name of the container being read.
//
// It is the scope alone. A tag used to name it, because a tag click also moved the
// selection; tags are conditions now, so they appear in the chip row where every other
// condition appears, and the title keeps saying which container the list is in.
func (s *Shell) scopeTitle() string {
	switch {
	case s.state.Selection == selectionInbox:
		return s.t().T(i18n.SidebarInbox)
	case s.state.Selection == selectionUnclassified:
		return s.t().T(i18n.SidebarUnclassified)
	default:
		return s.t().T(i18n.SidebarAllBookmarks)
	}
}
