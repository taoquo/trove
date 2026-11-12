package ui

import (
	"context"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/taoquo/trove/apps/macos/internal/domain"
	"github.com/taoquo/trove/apps/macos/internal/i18n"
	"github.com/taoquo/trove/apps/macos/internal/usecase/workspace"
)

// detailOverlay draws the detail, which the source renders as a modal rather than as a column.
//
// The card is .options-detail-focus-card: min(94vw, 980px) wide and 86dvh at most, with
// .options-detail-dialog's 22px radius, hairline and gradient. The dialog's own padding is
// zero -- the hero and the scroll region carry theirs -- so the panel DialogBase hands over is
// styled here rather than wrapped in another column, which is what kept the card ~40 points
// wider than the source: Modal's panel adds its own 5-space padding and backing colour.
func (s *Shell) detailOverlay(c *ui.Context, theme *ui.Theme, rail RailTokens, entrance float32, panel ui.Element) {
	// MaxHeight, and round 262 tried Height here and put it back. The reasoning for Height was
	// that a Grow(1) region inside a max-height column has no committed height to fill, so the
	// scroll region never becomes scrollable -- the source says max-height with the content
	// scrolling, and in CSS that works because a max-height box is given its content's height
	// capped at the maximum. The change was made on that reasoning and it did two things: the
	// overlay stopped appearing at all (the capture with TROVE_CAPTURE_STATE=detail came back as
	// the bare grid), and scrolling still did not work. So it is undone. What replaced it is the
	// note on TestTheInspectorOwnsAScrollRegion, which records what was tried.
	width := float32(DetailCardMaxWidth)
	if viewport := float32(s.windowWidth()); viewport > 0 && viewport*DetailDialogMixWidth < width {
		width = viewport * DetailDialogMixWidth
	}
	// magpie's dialog: a 14-point radius, one --line hairline and a deep soft drop
	// (`.action-confirm`, app.css:4038; `.modal .dialog`, :4063-4072). The gradient stays
	// flat -- both ends are the same card colour -- because magpie draws panels flat.
	panel.Width(width).
		MaxHeight(s.viewportHeight()*DetailDialogMixHigh).
		Radius(RadiusDialog).
		Border(1, rail.CardBorder).
		Shadow(0, 24, 60, 0, ui.RGBA(0, 0, 0, 0.32)).
		Gradient(rail.InspectorTop, rail.InspectorBottom, 180).
		Opacity(EntranceDetailCard.OpacityAt(entrance)).
		Scale(EntranceDetailCard.ScaleAt(entrance), EntranceDetailCard.ScaleAt(entrance)).
		Children(func() {
			s.inspector(c, theme, rail)
		})
}

// runProgressBar draws a live run.
//
// Three rows, as the source has them: phase and count, the track, then the status,
// timing and cancel button. The track is drawn from the stylesheet's values, and it is
// left unfilled while the run is waiting or stalled -- a frozen fill claims a position
// the run cannot vouch for.

// inspector draws the selected bookmark's detail: a fixed hero, a body split into the post
// (which owns the scroll) and the context rail, and an action bar on the card's floor.
//
// Spec 6.14: "it owns a vertical scroll region of its own; closing it or changing the selection
// must not move the main list's scroll position". The list and the sidebar both scroll already;
// the detail did not. It had a MaxHeight instead, so a bookmark with a long body or several
// pictures was cut off at the container's edge with no way to reach the rest -- and because the
// overlay and the pane are two different containers, putting the scroll here rather than in each
// of them is what keeps the two forms from drifting apart.
//
// The scroll is the reading pane's, not the card's (shell-contract.md S43). While the post
// scrolled, the rail scrolled with it and the two buttons came to rest wherever the post ended,
// so a short post left half the rail empty and a long one pushed the buttons out of sight. Now
// the hero, the rail and the bar stay put and only the post moves.
//
// The empty case returns before the layout: it is two short lines and giving it a scroll region
// would make it scrollable for no reason.
func (s *Shell) inspector(c *ui.Context, theme *ui.Theme, rail RailTokens) {
	row, found := s.selectedBookmark()
	if !found {
		s.inspectorEmpty(c, theme)
		return
	}
	// Grow(1) rather than Fill(), which is how the sidebar's own scroll is bounded: the parent
	// column gives this one the height that is left, and Fill() asked for a height the parent had
	// not committed to, so the region never became scrollable.
	ui.Column(c).FillWidth().Grow(1).Children(func() {
		s.inspectorBody(c, theme, rail, row)
	})
}

// inspectorEmpty is the two lines shown when nothing is chosen.
func (s *Shell) inspectorEmpty(c *ui.Context, theme *ui.Theme) {
	ui.Column(c).FillWidth().GapY(SpaceBase).Children(func() {
		ui.Text(c, s.t().T(i18n.InspectorTitle)).FontSize(FontTiny).Bold().TextColor(theme.TextMuted)
		ui.Text(c, s.t().T(i18n.InspectorEmpty)).FontSize(FontSmall).TextColor(theme.TextMuted)
	})
}

// inspectorBody is everything below the empty check, so that inspector can wrap it in a scroll
// region without the wrap being tangled in the drawing.
// inspectorBody draws the selected bookmark: a fixed hero, the post in the one scroll region the
// detail owns, and a strip on the floor with the tags and the two actions.
//
// The source splits the body into a reading pane and a 238-270px context rail; that rail is gone
// here, and so is the footer that briefly replaced it (S43). What survived them -- the published
// date, the tags, the actions -- are all in the header, so the card is a toolbar and the post, and
// its floor is wherever the post ends.
func (s *Shell) inspectorBody(c *ui.Context, theme *ui.Theme, rail RailTokens, row workspace.BookmarkRow) {
	s.detailHeader(c, theme, rail, row)
	ui.Divider(c)

	// ---- the post: one column, one scroll (S43) ----
	ui.Scroll(c).FillWidth().Grow(1).Children(func() {
		ui.Column(c).FillWidth().Padding(24, DetailHeroPadSide).Children(func() {
			s.detailPost(c, theme, rail, row)
		})
	})
}

// detailHeader is the card's single toolbar: who wrote it, when, what it is filed under, and the
// three actions -- and nothing else.
//
// The reader's third pass (S43). The footer strip of the previous round held the tags and two
// worded buttons, and the hero held the same three actions as another row; the "Done" button did
// nothing the close control beside the author does not. So the tag row moved up under the
// author's name, the worded buttons became 28-point icons with tooltips, and the card's floor is
// now the post's own end.
func (s *Shell) detailHeader(c *ui.Context, theme *ui.Theme, rail RailTokens, row workspace.BookmarkRow) {
	ui.Row(c).FillWidth().GapX(DetailHeroMainGap).
		Padding(DetailHeroPadTop, DetailHeroPadSide, DetailHeroPadBottom, DetailHeroPadSide).
		AlignItems(ui.Start).Children(func() {
		// The author's picture when there is one, the initials otherwise
		// (BookmarkInspector.tsx:141-147, the same three lines the card uses). 30 rather than the
		// source's 48: the header is one line now -- name, handle, date, tags -- and an avatar
		// taller than the text turns the byline into a portrait (S43).
		if picture, ready := s.mediaPicture(row.AuthorAvatarURL); ready {
			ui.Image(c, picture).Width(DetailHeroAvatarSize).Height(DetailHeroAvatarSize).
				Radius(RadiusPill).Fit(ui.Cover).Label(DetailAvatarLabel)
		} else {
			ui.Box(c).Width(DetailHeroAvatarSize).Height(DetailHeroAvatarSize).Radius(RadiusPill).
				Background(rail.HeroAvatarBg).Center().Label(DetailAvatarLabel).Children(func() {
				ui.Text(c, cardInitial(row.AuthorName, row.AuthorHandle)).
					FontSize(11).FontWeight(700).TextColor(theme.Text)
			})
		}

		// One line: who wrote it, when, and what it is filed under. The tags follow the date
		// rather than sitting on a row of their own -- a 30-point avatar beside one short row of
		// text left the header taller than its content, which is the space the reader saw.
		//
		// The row wraps, so a bookmark with many tags grows a second line by itself.
		ui.Row(c).Grow(1).MinWidth(0).GapX(DetailHeroLineGap).Wrap().
			AlignItems(ui.Center).Children(func() {
			ui.Text(c, row.AuthorName).Font(s.displayFace()).
				FontSize(DetailHeroNameFont).FontWeight(600).TextColor(theme.Text).MaxLines(1)
			ui.Text(c, "@"+row.AuthorHandle).FontSize(DetailHeroHandleFont).
				TextColor(rail.TextTertiary).MaxLines(1)
			// When it was posted, as a mark rather than a section: a small clock with the date
			// beside it. The saved and last-sync dates are gone -- neither is a fact a reader acts
			// on, and the card's own timestamp already says when this machine saved it.
			if published := s.publishedLabel(row); published != "" {
				ui.Row(c).GapX(DetailDateGap).AlignItems(ui.Center).
					Tooltip(s.t().T(i18n.InspectorPublished)).Children(func() {
					ui.Box(c).Width(DetailDateIcon).Height(DetailDateIcon).
						Draw(func(p *ui.Painter, r ui.Rect) {
							drawIcon(p, "globe", r.X, r.Y, min32(r.W, r.H), rail.TextTertiary)
						})
					ui.Text(c, published).FontSize(DetailDateFont).
						TextColor(rail.TextTertiary)
				})
			}
			s.detailTags(c, theme, rail, row)
		})

		s.detailActions(c, theme, rail, row)
	})
}

// detailTags is the tag row, under the author's own line: the pills, the pending suggestions the
// source draws in the same row (BookmarkInspector.tsx:428-448), and the field that adds one.
//
// It has no "Tags" heading: the pills name themselves, and the field's placeholder asks for one.
// The empty state is the field alone rather than a sentence saying there are no tags -- the
// sentence and the field said the same thing.
func (s *Shell) detailTags(c *ui.Context, theme *ui.Theme, rail RailTokens, row workspace.BookmarkRow) {
	for _, tag := range row.Tags {
		tag := tag
		ui.Row(c).MinHeight(DetailTagPillHeight).GapX(DetailTagPillGap).
			Radius(RadiusPill).
			Padding(0, DetailTagPillPadEnd, 0, DetailTagPillPadIn).
			Background(rail.PillBg).Border(1, rail.PillBorder).Children(func() {
			// .chip-button is 12px, and its close glyph is 12 -- not the 32-point square
			// IconOnly reserves for a toolbar button.
			ui.Text(c, tag.Name).FontSize(FontSmall).TextColor(rail.PillText)
			// Named with its tag: six pills in a row used to read out as six "Remove"s, and a
			// reader given the tag names beside them hears which is which only by counting.
			remove := ui.Box(c).Width(DetailTagPillIcon).Height(DetailTagPillIcon).
				Label(s.t().Tf(i18n.ActionRemoveNamed, tag.Name)).Focusable()
			remove.Draw(func(p *ui.Painter, r ui.Rect) {
				drawIcon(p, "close", r.X, r.Y, min32(r.W, r.H), rail.PillText)
			})
			remove.Tooltip(s.t().T(i18n.ActionRemove))
			remove.OnClick(func() {
				s.DetachTagFromSelected(context.Background(), tag.ID)
			})
		})
	}
	for _, suggestion := range s.pendingSuggestionsFor(row.TweetID) {
		s.cardSuggestionChip(c, theme, rail, suggestion,
			FontSmall, DetailTagPillHeight, DetailTagPillPadEnd)
	}
	// .options-detail-new-tag-input { height 28; radius 999; padding 0 12 }. Enter submits
	// because MyGo reports no blur to submit on; Escape clears.
	tagField := ui.TextInput(c, &s.state.DetailTagInput).
		Placeholder(s.t().T(i18n.CreateTagPrompt)).
		Width(DetailTagInputWidth).Height(DetailTagInputH).
		Radius(RadiusPill).Padding(0, 12).
		FontSize(FontTiny).Label(DetailTagInputLabel).
		Bind(&s.detailTagField)
	tagField.OnSubmit(func() { s.AttachTagFromDetail(context.Background()) })
	tagField.OnShortcut(0, ui.KeyEscape, func() {
		s.state.DetailTagInput = ""
		if s.window != nil {
			s.window.Invalidate()
		}
	})
}

// detailActions is the header's three icon controls: copy the link, open the post on X, close.
//
// They were worded buttons in the source ("Open on X", "Copy link", "Done") and two of them were
// drawn twice across the old header and footer. An icon with a tooltip says the same thing in 28
// points, and a dialog whose every control is a labelled pill is a settings page, not a reader
// (S43). "Done" is gone: the close control beside the author already does exactly that.
func (s *Shell) detailActions(c *ui.Context, theme *ui.Theme, rail RailTokens, row workspace.BookmarkRow) {
	ui.Row(c).GapX(DetailActionGap).AlignItems(ui.Start).Children(func() {
		// Copy link. The confirmation is the glyph turning into a tick for 900ms, with the
		// overshoot the card's checkbox has (motion.go, CopiedFeedbackFor); the tooltip changes
		// with it, so pointing at the tick says "Copied" rather than "Copy link".
		copied := s.copiedJustNow()
		icon, ink, label := "copy", theme.TextMuted, i18n.ActionCopyLink
		if copied {
			icon, ink, label = "check", theme.Accent, i18n.ActionCopied
		}
		popTarget := float32(0)
		if copied {
			popTarget = 1
		}
		copyLink := ui.Box(c).Width(DetailIconButton).Height(DetailIconButton).
			Radius(RadiusSmall).Center().Label(s.t().T(label)).Tooltip(s.t().T(label))
		copyLink = copyLink.Background(hoverColor(c, copyLink, "copy-bg",
			ui.Transparent, theme.SurfaceHover, copyLink.Hovered()))
		pop := copyLink.AnimateTo("copy-pop", popTarget, MotionPop)
		// `.icon:active`'s .9, under the copy's own pop (app.css:4053).
		copyLink = pressScale(copyLink, "press", PressScaleIcon, MotionPress)
		copyLink.OnClick(func() { s.CopySourceLink(c) })
		copyLink.Children(func() {
			ui.Box(c).Width(16).Height(16).Draw(func(p *ui.Painter, r ui.Rect) {
				size := min32(r.W, r.H)
				if copied {
					size *= pop
				}
				drawIcon(p, icon, r.X+(r.W-size)/2, r.Y+(r.H-size)/2, size, ink)
			})
		})

		// Open on X. Disabled when the record has no URL rather than opening a blank page.
		open := ui.Box(c).Width(DetailIconButton).Height(DetailIconButton).
			Radius(RadiusSmall).Center().Label(s.t().T(i18n.OpenOnX)).
			Tooltip(s.t().T(i18n.OpenOnX))
		open = open.Background(hoverColor(c, open, "open-bg",
			ui.Transparent, theme.SurfaceHover, open.Hovered()))
		if strings.TrimSpace(row.TweetURL) == "" {
			open.Children(func() {
				ui.Box(c).Width(16).Height(16).Draw(func(p *ui.Painter, r ui.Rect) {
					drawIcon(p, "external", r.X, r.Y, min32(r.W, r.H), rail.TextQuaternary)
				})
			})
		} else {
			open = pressScale(open, "press", PressScaleIcon, MotionPress)
			open.OnClick(func() { s.OpenSelectedOnX() })
			open.Children(func() {
				ui.Box(c).Width(16).Height(16).Draw(func(p *ui.Painter, r ui.Rect) {
					drawIcon(p, "external", r.X, r.Y, min32(r.W, r.H), theme.Accent)
				})
			})
		}

		// The close control takes the dialog's initial focus. MyGo otherwise hands it to the first
		// focusable in the tree (ui/scope.go, arrangeFocus), and that is the new-tag field now that
		// the tags share the author's line: the field would open focused -- a ring and a caret as
		// if the user had clicked it -- and, because it clears itself on Escape, stop Escape from
		// closing the dialog. AutoFocus names this control instead of relying on where it sits in
		// the tree, which the inline tag row no longer lets us control
		// (TestTheTagFieldDoesNotTakeTheFocus and TestAClickOpensTheDetail, S43).
		// The name is the source's: the close control carries detailsTitle as its
		// aria-label (BookmarkInspector.tsx:174-177), not a word of its own. Named as well
		// as tooltipped: a tooltip is a hover hint, and without the label this control --
		// the only way out of the overlay by pointer -- answers to no name at all (round 38).
		close := ui.Box(c).Width(DetailIconButton).Height(DetailIconButton).
			Radius(RadiusSmall).Focusable().AutoFocus().Bind(&s.detailCloseField).
			Label(s.t().T(i18n.InspectorTitle)).
			Tooltip(s.t().T(i18n.InspectorTitle)).Center()
		if close.Hovered() {
			close = close.Background(theme.SurfaceHover)
		}
		close.OnClick(func() { s.CloseDetail() })
		close.Children(func() {
			ui.Box(c).Width(16).Height(16).Draw(func(p *ui.Painter, r ui.Rect) {
				drawIcon(p, "close", r.X, r.Y, min32(r.W, r.H), theme.TextMuted)
			})
		})
	})
}
func (s *Shell) selectedBookmark() (workspace.BookmarkRow, bool) {
	if s.state.Selected == "" {
		return workspace.BookmarkRow{}, false
	}
	for _, row := range s.state.Data.Bookmarks {
		if row.TweetID == s.state.Selected {
			return row, true
		}
	}
	return workspace.BookmarkRow{}, false
}

// countForList finds a list's bookmark count.
func countForList(lists []domain.ListSummary, id string) int {
	for _, list := range lists {
		if list.ID == id {
			return list.BookmarkCount
		}
	}
	return 0
}

// SelectAllResults puts every bookmark the current query matches into the batch selection.
//
// The source's "select current results" does this from the result set the pane already
// holds. The shell holds one page, so the ids come from a query widened to the match count.

// SelectAllResults puts every bookmark the current query matches into the batch selection.
//
// The source's "select current results" does this from the result set the pane already
// holds. The shell holds one page, so the ids come from a query widened to the match count.
func (s *Shell) SelectAllResults(ctx context.Context) {
	if s.service == nil {
		return
	}
	ids, err := s.service.MatchingIDs(ctx, s.state.Data.Query)
	if err != nil {
		// Too many to select at once: the count is left alone rather than filled with a
		// partial set a bulk action would then run on.
		s.state.LastBundleReport = s.t().T(i18n.BatchTooMany)
		s.state.LastBundleOK = false
		if s.window != nil {
			s.window.Invalidate()
		}
		return
	}
	s.state.SelectedIDs = make(map[string]bool, len(ids))
	for _, id := range ids {
		s.state.SelectedIDs[id] = true
	}
	if s.window != nil {
		s.window.Invalidate()
	}
}

// SelectLoadedResults puts the rows the list has loaded into the batch selection.
//
// The source has two buttons because they answer two questions: "select current results"
// is everything the query matches, "select loaded" is what the list holds
// (BookmarkResultsPane.tsx:45-52). They are not interchangeable at real scale -- on a
// 9,562-bookmark library MatchingIDs refuses above MaxLibraryPageSize, so the first
// button can only say no. Until this existed, that refusal left no one-press bulk
// selection at all and the reader was left ticking boxes by hand (round 33).
func (s *Shell) SelectLoadedResults() {
	rows := s.state.Data.Bookmarks
	if len(rows) == 0 {
		return
	}
	s.state.SelectedIDs = make(map[string]bool, len(rows))
	for _, row := range rows {
		s.state.SelectedIDs[row.TweetID] = true
	}
	// The refusal above describes the previous press, not this one; leaving it on the
	// status line next to a successful selection would read as a failure of this one.
	s.state.LastBundleReport = ""
	s.state.LastBundleOK = false
	if s.window != nil {
		s.window.Invalidate()
	}
}

// tagLabels is the batch tag selector's options: every tag the workspace knows, by name.
func (s *Shell) tagLabels() []string {
	labels := make([]string, 0, len(s.state.Data.Tags))
	for _, tag := range s.state.Data.Tags {
		labels = append(labels, tag.Name)
	}
	return labels
}

// ApplyBulkTag gives every selected bookmark the one tag the selector names.
//
// It goes through AddTags rather than a new attach path: the service already knows how to
// resolve a name to a tag and to skip a relation that exists, so a second route would be a
// second set of rules for the same write.

// ApplyBulkTag gives every selected bookmark the one tag the selector names.
//
// It goes through AddTags rather than a new attach path: the service already knows how to
// resolve a name to a tag and to skip a relation that exists, so a second route would be a
// second set of rules for the same write.
func (s *Shell) ApplyBulkTag(ctx context.Context) error {
	ids := s.BatchSelection()
	// The picker's placeholder is not a tag name; batchTagChoice turns it back into "nothing
	// chosen", which is what the source's disabled Apply button means.
	name := s.batchTagChoice()
	if len(ids) == 0 || name == "" || s.service == nil {
		return nil
	}
	if _, err := s.service.AddTags(ctx, ids, []string{name}); err != nil {
		return err
	}
	s.ShowToast(s.batchTaggedLabel(len(ids)))
	s.clearSelection()
	s.Reload(ctx)
	return nil
}

// inspectorFits reports whether the detail can be a pane rather than an overlay.
//
// The test is arithmetic on the widths the layout would need: the sidebar, the results pane's
// minimum, and the inspector itself. windowWidth is 0 before the window exists, and 0 does not
// fit anything, so the first frame draws the overlay -- which is the safe way round, because an
// overlay over a narrow window is legible and a three-pane layout in one is not.
// No guard for a window that does not exist yet. windowWidth returns 0 before there is one, and
// 0 is below ThreeColumnMinWidth, so the first frame is an overlay -- which is the safe way
// round, because an overlay over a narrow window is legible and three panes in one is not. The
// guard that used to be here said the same thing a second time.
func (s *Shell) inspectorFits(sidebarWidth float32) bool {
	return inspectorFitsFor(float32(s.windowWidth()), sidebarWidth)
}

// inspectorFitsFor reports whether the detail may be drawn as a third pane. It never may.
//
// The source has no three-column layout at all. Its detail is a modal at every width:
//
//	.options-detail-modal      { position: fixed; inset: 0; z-index: 40; … }
//	.options-detail-focus-card { width: min(94vw, 980px); max-height: 86dvh }
//	.options-detail-dialog     { border-radius: 22px }
//
// and `grep -c "grid-template-columns.*sidebar.*main.*inspector" src/styles/extension.css` is 0.
// The reference picture agrees: its workspace is a sidebar and a results pane, and opening a
// bookmark covers the window with a centred card.
//
// This function used to answer "is the window wide enough for three columns", from spec 9.3's
// 1400 band, and the branch it fed drew an inspector beside the list. Its own comment said "below
// 1400 the detail is an overlay, as it is in the source at every width" -- the sentence was right
// and the code contradicted it. Round 480 makes it answer no, so the workspace has the two columns
// the source has.
func inspectorFitsFor(_, _ float32) bool {
	return false
}

// copiedJustNow reports whether the detail's copy chip is still holding its confirmation.
//
// The state is one timestamp and the view reads the clock; nothing is written while drawing,
// so a view rebuilt several times for one event asks the same question and gets the same answer
// until the deadline passes -- at which point the repaint CopySourceLink scheduled turns it off.
func (s *Shell) copiedJustNow() bool {
	return !s.state.CopiedAt.IsZero() && time.Since(s.state.CopiedAt) < CopiedFeedbackFor
}

// publishedLabel is when the author posted, in this locale, or "" when the source gave no date.
//
// The empty case draws nothing at all rather than a label with nothing after it: an empty date
// that rendered as "not synced yet" was one of the defects round 535 fixed.
func (s *Shell) publishedLabel(row workspace.BookmarkRow) string {
	if strings.TrimSpace(row.CreatedAtOnX) == "" {
		return ""
	}
	return domain.FormatSavedAt(row.CreatedAtOnX, string(s.t().Locale()))
}
