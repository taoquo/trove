package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/i18n"
)

// runProgressBar draws a live run.
//
// Three rows: phase and count, the track, then the status, timing and cancel button. The
// track is drawn from the stylesheet's values, and it is left unfilled while the run is
// waiting or stalled -- a frozen fill claims a position the run cannot vouch for.
func (s *Shell) runProgressBar(c *ui.Context, theme *ui.Theme, view RunProgressView) {
	rail := Rail(s.state.Appearance, s.state.SystemDark, s.translator.Locale() == i18n.ZHCN)

	ui.Column(c).FillWidth().GapY(SpaceSmall).Padding(SpaceBase, SpaceBase).
		Background(rail.Muted).Radius(RadiusMedium).Border(1, rail.SubtleLine).Children(func() {
		// The progress head: space-between with an 8 gap. Without the gap the phase and the
		// count run together as "LLM120/2000" in the 280-wide sidebar -- the block was written
		// for the settings panel, which is several times wider.
		ui.Row(c).FillWidth().GapX(SpaceBase).Children(func() {
			ui.Text(c, view.Phase).FontSize(FontSmall).Bold().TextColor(theme.Text)
			ui.Text(c, view.Count).FontSize(FontMicro).Grow(1).
				TextAlign(ui.End).TextColor(theme.TextMuted)
		})

		// MyGo has no progress bar, so the track is drawn: a rounded fill over a
		// rounded ground, both from the stylesheet's values. While the run is waiting
		// or stalled nothing is filled -- a frozen fill claims a position the run
		// cannot vouch for, which is the reason for the indeterminate state.
		// The user's motion preference changes what an indeterminate bar looks like, so
		// it is read here rather than assumed.
		reduceMotion := c.Preferences().ReduceMotion
		fraction := view.ProgressFillFraction(reduceMotion)
		opacity := view.FillOpacity(reduceMotion)
		fill := rail.Accent
		if view.Stalled {
			fill = theme.Warning
		}
		// ui.Box(c), not a bare ui.Element{}: an element with no context is detached
		// from the frame and draws nothing, which is how the track went missing.
		track := ui.Box(c).FillWidth().Height(ProgressTrackHeight).Radius(ProgressTrackRadius).
			Background(rail.BadgeBg)

		// While the run is waiting or stalled the stripe slides; MyGo's Loop keeps
		// frames arriving for as long as it moves, which is the repaint this needed.
		// Under reduced motion no loop is started at all.
		var slide float32
		if view.Indeterminate && !reduceMotion {
			slide = track.Loop("autotag-stripe",
				time.Duration(IndeterminateSlidePeriodMs)*time.Millisecond, ui.EaseInOut)
		}

		track.Draw(func(p *ui.Painter, r ui.Rect) {
			if fraction <= 0 {
				return
			}
			if opacity < 1 {
				fill = ui.RGBA(fill.R, fill.G, fill.B, opacity)
			}

			if view.Indeterminate && !reduceMotion {
				left, right, visible := StripeBounds(r.X, r.W, slide)
				if !visible {
					return
				}
				p.Fill(ui.Rect{X: left, Y: r.Y, W: right - left, H: r.H},
					fill, float32(ProgressTrackHeight)/2)
				return
			}

			p.Fill(ui.Rect{X: r.X, Y: r.Y, W: r.W * fraction, H: r.H},
				fill, float32(ProgressTrackHeight)/2)
		})

		// What the pass has produced, under the track: the count above says how far it
		// has read, this says what came out of it. Empty until the first suggestion.
		if view.Detail != "" {
			ui.Text(c, view.Detail).FontSize(FontMicro).TextColor(theme.TextMuted).MaxLines(2)
		}

		// The progress foot: align centre, gap 8, flex-wrap -- and in a 280-wide sidebar it
		// always wraps, which is why this is two rows rather than one.
		//
		// The first version put the status, the foot and the button in one wrapping row and the
		// foot column collapsed to one word per line ("Elapsed / 39s / · / ~611s / left"), because
		// a wrapping row gives every child its minimum width before it wraps. Two explicit rows say
		// what the wrap resolves to at this width: status and foot on one line, the button
		// across the panel under them.
		color := theme.TextMuted
		if view.Stalled {
			color = theme.Warning
		}
		ui.Row(c).FillWidth().GapX(SpaceBase).Children(func() {
			ui.Text(c, view.StatusLine).FontSize(FontMicro).TextColor(color)
			ui.Text(c, view.Foot).FontSize(FontMicro).Grow(1).TextColor(theme.TextMuted)
		})
		textAction(c, theme, rail, textActionSpec{
			label: view.CancelLabel, fill: true, disabled: view.Cancelling,
			// One path for every stop control: the progress bar, the sidebar and the
			// page must not differ in whether the batch in flight is kept.
			onClick: func() { s.StopAutoTag() },
		})
	})
}

// loadSyncPreferences reads the sync preferences into the panel's fields.

// FocusSearch asks the next frame to put the cursor in the search field.
func (s *Shell) FocusSearch() {
	// On the settings page Cmd-F belongs to that page's find field: it searches every
	// section, and focusing the list's search would be a field the reader cannot see.
	if s.state.SettingsOpen {
		// The handle focuses the field directly, the way the tag drafts are focused from
		// their openers. A request made before the page has ever drawn waits for its
		// binding (ref.go), so this works on the first frame too.
		s.settingsFindField.Focus()
		return
	}
	// Once, from here: a flag the builder read on every frame kept the cursor in the
	// field until some unrelated reload happened to clear it.
	s.searchField.Focus()
}

// OpenSuggestions loads the pending suggestions and shows the panel.
//
// It is exported because the menu bar opens it from outside this package.

// ToggleAddForm shows or hides the add row, which the menu bar's "Add bookmark" command and
// nothing in the window itself opens. The add row is this build's addition -- there is no
// other in-app form -- and its entry point lives where a macOS app puts commands that have no
// place in the window.
func (s *Shell) ToggleAddForm() {
	s.state.AddOpen = !s.state.AddOpen
	if s.window != nil {
		s.window.Invalidate()
	}
}

// addRow is the entry point for a post the user pastes in.
//
// It is shaped to the app rather than copied from a page's save action: without it a
// standalone manager could only ever show what a sync brought in.
func (s *Shell) addRow(c *ui.Context, theme *ui.Theme, rail RailTokens) {
	if !s.state.AddOpen {
		return
	}
	// The same field-and-action row as the toolbar: 28-point inputs with one hairline and a
	// compact action, on the page ground.
	//
	// Two blank boxes are not a form: each field says what it takes, the link field takes
	// the cursor when the row opens, and Enter submits from either one -- the same rule the
	// sidebar's draft row and the batch bar's tag field already follow.
	ui.Row(c).FillWidth().GapX(SpaceBase).Children(func() {
		link := ui.TextInput(c, &s.state.AddURL).Grow(1).MinHeight(SearchFieldHeight).
			Radius(RadiusSidebarRow).Border(1, rail.CardBorder).
			Placeholder(s.t().T(i18n.AddLinkPlaceholder)).
			Label(s.t().T(i18n.AddLinkPlaceholder)).AutoFocus()
		link.OnSubmit(func() { s.AddBookmarkFromDraft(context.Background()) })
		text := ui.TextInput(c, &s.state.AddText).Grow(1).MinHeight(SearchFieldHeight).
			Radius(RadiusSidebarRow).Border(1, rail.CardBorder).
			Placeholder(s.t().T(i18n.AddTextPlaceholder)).
			Label(s.t().T(i18n.AddTextPlaceholder))
		text.OnSubmit(func() { s.AddBookmarkFromDraft(context.Background()) })
		SecondaryButton(c, "", s.t().T(i18n.AddBookmark), theme, rail, func() {
			s.AddBookmarkFromDraft(context.Background())
		})
		// A way out, in the row itself. The row is opened from the menu bar and there was no
		// control in the window that closed it: a reader who opened it by mistake had to find
		// the menu command again. Every other row this build opens (the sidebar's draft tag,
		// the batch bar) can be dismissed where it stands.
		closeRow := ui.Box(c).Width(SearchFieldHeight).Height(SearchFieldHeight).
			Radius(RadiusSidebarRow).Center().Focusable().
			Label(s.t().T(i18n.ActionCancel)).Tooltip(s.t().T(i18n.ActionCancel))
		closeRow = closeRow.Background(hoverColor(c, closeRow, "add-close",
			ui.Transparent, theme.SurfaceHover, closeRow.Hovered()))
		closeRow.OnClick(func() { s.ToggleAddForm() })
		closeRow.Children(func() {
			ui.Box(c).Width(14).Height(14).Draw(func(p *ui.Painter, r ui.Rect) {
				drawIcon(p, "close", r.X, r.Y, min32(r.W, r.H), theme.TextMuted)
			})
		})
	})
	if s.state.AddProblem != "" {
		// A refused add is an error, not an accent: the field borders and the settings
		// page's invalid values use the danger ink.
		ui.Text(c, s.state.AddProblem).FontSize(FontSmall).TextColor(theme.Danger)
	}
}

// tagName resolves a tag id to its name from the loaded library.

// batchBar draws the bulk actions for the current selection.
//
// It is only drawn when something is selected, so the toolbar stays quiet until
// the user has chosen what to act on.
func (s *Shell) batchBar(c *ui.Context, theme *ui.Theme, rail RailTokens) {
	ids := s.BatchSelection()
	if len(ids) == 0 {
		return
	}

	// A card row, as every standing bar is drawn: white, one hairline, a 12-point
	// radius.
	ui.Column(c).FillWidth().Padding(12, 14).GapY(SpaceBase).
		Radius(RadiusResultCard).Border(1, rail.CardBorder).Background(theme.Surface).
		Children(func() {
			ui.Text(c, s.t().Tf(i18n.BatchSelected, len(ids))).FontSize(FontSmall).FontWeight(500).
				TextColor(theme.Text)

			ui.Row(c).FillWidth().GapX(8).Children(func() {
				// The field is this build's own: a bare TextInput has no name, and a screen
				// reader would announce "text field" beside an empty box with nothing saying
				// what to type. The placeholder asks the question; the label gives the field
				// the same answer as its name.
				batchTagField := ui.TextInput(c, &s.state.TagInput).
					Placeholder(s.t().T(i18n.CreateTagPrompt)).
					Label(s.t().T(i18n.CreateTagPrompt))
				// Enter is the keyboard form of the tag button beside it. The batch bar has
				// only the picker and Apply, so this field is this build's own -- and a field
				// with no Enter beside a button that works is the same defect the sidebar's
				// draft row had.
				batchTagField.OnSubmit(func() {
					if _, err := s.AddTagsToSelection(context.Background()); err != nil {
						s.reportErr(err)
					}
				})

				IconOnly(c, "tag", s.t().T(i18n.ActionAddTag), theme, rail, func() {
					if _, err := s.AddTagsToSelection(context.Background()); err != nil {
						s.reportErr(err)
					}
				})

				// A picker of every tag plus its apply button; the typed-name path stays
				// because it answers a different question.
				if tagLabels := s.tagLabels(); len(tagLabels) > 0 {
					// The placeholder is an option, as an empty-valued "Choose tag" option is.
					// Without it the trigger draws a bare chevron: nothing says which picker
					// tags and which one moves. The picker's name is "Choose tag", not the tag
					// button's "Add tag": two controls called "Add tag" side by side make the
					// reader guess which one opens a menu.
					settingsPick(c, theme, rail, s.batchChooseTagLabel(),
						&s.state.BulkTagLabel,
						append([]string{s.batchChooseTagLabel()}, tagLabels...), s.invalidate)
					// Apply stays dark until a tag is chosen.
					settingsAction(c, theme, rail, s.t().T(i18n.BatchApplyTag),
						s.state.Mutating || s.batchTagChoice() == "", false, func() {
							if err := s.ApplyBulkTag(context.Background()); err != nil {
								s.reportErr(err)
							}
						})
				}

				labels := s.listLabels()
				if len(labels) > 0 {
					// Same rule as the tag picker: the trigger says "Choose list", the button
					// beside it says what pressing it does.
					settingsPick(c, theme, rail, s.batchChooseListLabel(),
						&s.state.MoveTargetLabel,
						append([]string{s.batchChooseListLabel()}, labels...), s.invalidate)
					settingsAction(c, theme, rail, s.t().T(i18n.ActionMoveToList),
						s.state.Mutating || s.batchListChoice() == "", false, func() {
							if err := s.MoveSelectionToList(context.Background()); err != nil {
								s.reportErr(err)
							}
						})
				}

				ui.Spacer(c).FillWidth()

				// The two selection buttons, in order: everything the query matches, then what
				// the list has loaded. At real scale they are not interchangeable -- the first
				// refuses above MaxLibraryPageSize, and the second is the one that still works.
				settingsAction(c, theme, rail, s.t().T(i18n.BatchSelectCurrent), false, false, func() {
					s.SelectAllResults(context.Background())
				})

				settingsAction(c, theme, rail, s.t().T(i18n.BatchSelectLoaded), false, false, func() {
					s.SelectLoadedResults()
				})

				settingsAction(c, theme, rail, s.t().T(i18n.ActionClearSelection), false, false, func() {
					s.clearSelection()
				})

				// Deleting a batch goes through the same fail-closed confirmation as
				// a single deletion.
				IconChip(c, theme, rail, "trash", s.t().T(i18n.ActionDelete), func() {
					s.AskDeleteBookmarks(s.BatchSelection())
				})
			})
		})
}

// loadingState is the panel drawn while the first read is in flight.
//
// It is a skeleton rather than a spinner: six placeholder cards in the same grid the real
// cards use, so the page does not jump when the read lands. A bare spinner in an empty pane
// would do neither.

// resultsCountLine is the toolbar's count: "68 结果", or "20 / 68 结果" when the list is capped.
func (s *Shell) resultsCountLine() string {
	shown := s.t().Tf(i18n.ToolbarCount, s.state.Data.Total)
	if s.state.Data.HasMore {
		shown = s.t().Tf(i18n.ToolbarShowingOf, len(s.state.Data.Bookmarks), s.state.Data.Total)
	}
	return shown + " " + s.t().T(i18n.ResultsLabel)
}

// toolbar is the control strip at the top of the page.
//
// It is one row: search, filter, sort, the view-mode control, and the result count. It used
// to be a Column of five rows -- search and filter and sort, then the view toggles, then the
// run status, then Tidy and Settings, then the count -- so the strip was five lines tall and
// every control was a button with its own outline. That is the "six rows of stacked buttons"
// shape the layout was rewritten to remove once already, and it grew back one row at a time
// as each control was added.
//
// The two rows below the controls are conditional and empty in the state the screenshots are
// taken in, so the strip is one line there: the active filters, which are data rather than
// controls and have to be visible to be removable, and the run status, which is a sentence and
// not a control.
func (s *Shell) toolbar(c *ui.Context, theme *ui.Theme, rail RailTokens) {
	// The bottom inset is 10, not 8: the bulk bar's own `margin-top: -6` is measured against
	// it, so at 8 the bar sits 2 points high and everything below it rides up with it.
	ui.Column(c).FillWidth().Padding(0, 0, ResultsHeaderPadBottom, 0).GapY(SpaceBase).
		Background(rail.Results).Children(func() {
		// The page's controls sit straight on the page ground: no panel, no
		// outline -- only the search field and the labelled actions carry an edge.
		ui.Row(c).FillWidth().GapX(SpaceBase).Children(func() {
			searchField(c, theme, rail, &s.state.Query,
				s.t().T(i18n.SearchPlaceholder), s.t().T(i18n.ActionClear), &s.searchField,
				func() { s.ReloadAsync(context.Background()) },
				func() { s.ExitSearch(context.Background()) })

			// The active filters, in the search's own row and on its right.
			//
			// A second toolbar row for the active filters, with the label "活跃筛选:", would
			// be a full-width line under a field that is already the longest control in the
			// row, and selecting one filter would grow the toolbar by it. Beside the field, a
			// condition adds no height: the chip is 28 points in the search's 36-point row.
			//
			// The group is bounded and wraps, `flex-wrap: wrap`. Inline that rule needs a
			// width to wrap against; without one, six conditions would take the row from the
			// search and the actions and push them off the edge.
			chips := s.activeConditions()
			if len(chips) > 0 {
				ui.Row(c).AlignItems(ui.Center).Wrap().GapX(SpaceSmall).GapY(SpaceSmall).
					MaxWidthPercent(ToolbarChipsMaxPercent).
					// The label still names the group: the chips are a group of conditions,
					// and a screen reader gets their shared name even though the row has no
					// room to print it.
					Label(s.t().T(i18n.ActiveFiltersLabel)).Children(func() {
					for _, chip := range chips {
						s.conditionChip(c, rail, chip)
					}
					// One control for the whole set. Individual x's are right for one wrong
					// filter; going back to an unfiltered list should not be a click per chip.
					// At the chip's own height: the control sits in the search's 28-point row,
					// and a 32-point button here would grow the row and shift every control
					// beside it down by two points whenever a filter was applied.
					textAction(c, theme, rail, textActionSpec{
						label: s.t().T(i18n.FilterClearAll), height: ChipMinHeight,
						onClick: func() { s.clearConditions(context.Background()) },
					})
				})
			}

			// The toolbar action is an icon and a word, not an icon with a tooltip. The
			// intended design shows "筛选" beside the funnel and "最近保存" beside the arrows; this
			// build drew both as bare glyphs, so the only way to know what they were was to
			// hover one at a time.
			// PopoverBase rather than Popover: the panel's look is the pop card, and the
			// toolkit's own panel is a different shape.
			trigger := ui.Row(c).GapX(SpaceSmall).Children(func() {
				// The toolbar action, with a badge beside the word when filters are on.
				ToolbarAction(c, theme, rail, "filter", s.t().T(i18n.ToolbarFilters),
					s.conditionCount(), s.state.FilterPopoverOpen, func() {
						s.ToggleFilterPopover()
					})
			})
			ui.PopoverBase(c, trigger, &s.state.FilterPopoverOpen, func(panel ui.Element) {
				s.filterPopover(c, theme, rail, panel, trigger)
			})

			// The sort control cycles, as the stylesheet's toolbar does: one click advances to the
			// next order and the label says which is active. The label is the order alone, with no
			// prefix, so "Sort: Recently saved" adds a word the reference does not have.
			ToolbarAction(c, theme, rail, "sort", s.currentSortLabel(), 0, false, func() {
				s.CycleSort(context.Background())
			})

			// A hairline divider between the actions and the view toggles.
			ui.Box(c).Width(1).Height(20).Background(rail.SubtleLine)

			// The two toggles are icon-only with aria-pressed: with two options, seeing which
			// is on matters more than saving a click.
			ViewToggleGroup(c, func() {
				IconToggle(c, "grid", s.t().T(i18n.ViewGrid),
					s.state.ViewMode == domain.ViewGrid, theme, func() {
						if s.state.ViewMode != domain.ViewGrid {
							s.ToggleViewMode()
						}
					})
				IconToggle(c, "list", s.t().T(i18n.ViewList),
					s.state.ViewMode == domain.ViewList, theme, func() {
						if s.state.ViewMode != domain.ViewList {
							s.ToggleViewMode()
						}
					})
			})

			// How many results the list holds, at the row's end where the eye lands after
			// the controls. It used to be the page heading's right-hand annotation; with the
			// heading gone to the title bar's crumb, the count stays where the list is.
			// Small and tertiary: at 36 points it outranked the page's name.
			ui.Text(c, s.resultsCountLine()).FontSize(FontTiny).MaxLines(1).
				TextColor(rail.TextTertiary)
			// Nothing more here: the graph, the tag library and the vocabulary review belong to
			// the tags view, and the settings gear belongs to the sidebar footer. The graph and
			// the review would otherwise have no trigger, so they cannot simply be removed.

			// No trailing spacer. The primary row is a two-track grid,
			// `minmax(0, 1fr) auto`: the search takes every point the controls do not, and
			// the controls end at the shell's inset. A FillWidth spacer here competes with
			// the search's Grow(1) for the same slack and the search loses half of it:
			// measured 505 wide against 649, with a blank strip after the view toggles.

			// Through ButtonSmall like the rest of the strip. A raw ui.Button would keep the
			// platform's outline and fill while the control beside it did not: "Sort: Recently
			// saved" would be flat and "Tidy vocabulary" boxed, the same rule applied to one
			// of two neighbours.
		})

		// What the last run, import or review did, and what a refresh is doing right now.
		// One line, and only when there is something to say: these were four separate rows,
		// each empty most of the time. The page heading used to carry the refresh note; with
		// the heading gone it belongs here, where the numbers are.
		var status []string
		// A library-wide edit says what it is doing: "merging tags" is more use than a
		// generic "updating", and the edit refreshes the page when it finishes.
		switch {
		case s.state.MutationNote != "":
			status = append(status, s.state.MutationNote)
		case s.state.Refreshing:
			status = append(status, s.t().T(i18n.ToolbarRefreshing))
		}
		// The short word, not the sentence. The sentence that explains a failure belongs under
		// the sync button, which is the one place it is drawn; this line carried it too, so the
		// same paragraph appeared twice across the window and in the wrong place.
		// syncProblemLine owns it now.
		if label := syncStatusLabel(s.t(), s.state.LastSyncStatus); label != "" {
			status = append(status, label)
		}
		if s.state.LastBundleReport != "" {
			status = append(status, s.state.LastBundleReport)
		}
		if s.state.ReviewApplied != "" {
			status = append(status, s.state.ReviewApplied)
		}
		if s.state.AutoTagStatus != "" {
			status = append(status, s.state.AutoTagStatus)
		}
		if !s.state.LastSyncedAt.IsZero() {
			// FormatSavedAt, not FormatTimestamp: the latter would say
			// "Last sync 2026-10-17T10:52:03.889Z". The sidebar and the auto-tag panel use the
			// display formatter for the same reason, so every site must match.
			status = append(status, s.t().T(i18n.LastSyncLabel)+" "+
				domain.FormatSavedAt(domain.FormatTimestamp(s.state.LastSyncedAt), string(s.t().Locale())))
		}
		if len(status) > 0 {
			ui.Text(c, strings.Join(status, "  ·  ")).
				FontSize(FontTiny).TextColor(theme.TextMuted)
		}
	})
}

// cardScale is the set of type sizes a card draws with.
//
// The grid and the list differ per property rather than by one factor, and which set is in
// use is a decision worth asserting: a test that only pins the constants passes while the
// card quietly uses the other set.
type cardScale struct {
	copyFont       float32
	copyLineHeight float32
	authorFont     float32
	avatarSize     float32
	avatarFont     float32
}

func cardTypeScale(grid bool) cardScale {
	if grid {
		return cardScale{
			copyFont: CardCopyFont, copyLineHeight: CardCopyLineHeight,
			authorFont: CardAuthorFont, avatarSize: CardAvatarSize, avatarFont: CardAvatarFont,
		}
	}
	return cardScale{
		copyFont: CardListCopyFont, copyLineHeight: CardListCopyLineHeight,
		authorFont: CardListAuthorFont, avatarSize: CardListAvatarSize, avatarFont: CardListAvatarFont,
	}
}

// cardInitial is the letters the avatar shows when there is no picture to load.
//
// The card avatar carries them at 11px/700. One letter per word, at most two -- so "Yassine
// Bouanane" is YB and "Margaret Hamilton" is MH -- and the handle's first two letters when
// the name is empty. Taking only the first letter of the name drew a single glyph where the
// reference shows two.
func cardInitial(name, handle string) string {
	var initials strings.Builder
	words := 0
	for _, part := range strings.Fields(name) {
		if words == 2 {
			break
		}
		first := []rune(part)[0]
		initials.WriteString(strings.ToUpper(string(first)))
		words++
	}
	if initials.Len() > 0 {
		return initials.String()
	}

	trimmed := strings.TrimPrefix(strings.TrimSpace(handle), "@")
	runes := []rune(trimmed)
	if len(runes) > 2 {
		runes = runes[:2]
	}
	if len(runes) == 0 {
		return "?"
	}
	return strings.ToUpper(string(runes))
}

// pendingSuggestionsBar draws the bar for the tag suggestions waiting review.
//
// It is not batchBar: that one answers "what do I do with the rows I picked" and only appears when
// something is picked, while this appears whenever the workspace is holding suggestions, whatever
// is selected, because it is work the app is asking for rather than work the user started. A
// stylesheet class is not a surface -- two bars can wear the same class family with a different
// job, so this one is its own entry in the surface index.
func (s *Shell) pendingSuggestionsBar(c *ui.Context, theme *ui.Theme, rail RailTokens) {
	n := len(s.state.PendingSuggestions)
	if n == 0 {
		return
	}
	// The bar's ground is the panel surface, not the pressed/hover surface. rail.Muted is
	// theme.SurfacePressed -- a platform colour -- so the bar would render cool grey
	// (221,221,225) on a warm page instead of the panel's (251,247,243).
	// The bulk bar's `margin-top: -6px; margin-bottom: 14px` sets the gap. The bottom margin
	// is the whole gap between the bar and the first card: without it the grid starts at the
	// bar's edge, 14 points high.
	ui.Row(c).FillWidth().Wrap().GapX(BulkBarGap).GapY(SpaceBase).
		Padding(BulkBarPadding).Radius(BulkBarRadius).
		Margin(-BulkBarMarginTop, 0, BulkBarMarginBottom, 0).
		Border(1, rail.SubtleLine).Background(theme.Surface).Children(func() {
		// The meta row: the tag glyph and "N 待确认标签", 13px and secondary.
		ui.Row(c).GapX(SpaceBase).Grow(1).Children(func() {
			ui.Box(c).Width(SecondaryButtonIcon).Height(SecondaryButtonIcon).
				Draw(func(p *ui.Painter, r ui.Rect) {
					drawIcon(p, "tag", r.X, r.Y, min32(r.W, r.H), theme.TextMuted)
				})
			// The label is composed as `{count} {pendingSuggestionsTitle}`, not as one format
			// string, so the count and the word stay separately translatable.
			ui.Text(c, fmt.Sprintf("%d %s", n, s.t().T(i18n.SuggestionsPendingTitle))).
				FontSize(BulkMetaFont).TextColor(theme.TextMuted)
		})
		SecondaryButton(c, "check", s.t().T(i18n.SuggestionsAcceptAll), theme, rail, func() {
			s.acceptAllPendingSuggestions(context.Background())
		})
	})
}

// acceptAllPendingSuggestions accepts everything the banner counted.
//
// Through the same use case the panel's own "accept all" uses, then a reload: the count on the
// banner is what is pending now, and leaving the old number up would offer a button for work that
// has already been done.
func (s *Shell) acceptAllPendingSuggestions(ctx context.Context) {
	if s.suggestions == nil || len(s.state.PendingSuggestions) == 0 {
		return
	}
	ids := make([]string, 0, len(s.state.PendingSuggestions))
	for _, row := range s.state.PendingSuggestions {
		ids = append(ids, row.Suggestion.ID)
	}
	_, failures := s.suggestions.AcceptAll(ctx, ids)
	s.state.ReviewApplied = s.t().Tf(i18n.ReviewAppliedCount, len(ids)-len(failures))
	s.ReloadAsync(ctx)
}
