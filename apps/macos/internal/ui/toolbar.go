package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/taoquo/x-master/apps/macos/internal/domain"
	"github.com/taoquo/x-master/apps/macos/internal/i18n"
)

// runProgressBar draws a live run.
//
// Three rows, as the source has them: phase and count, the track, then the status,
// timing and cancel button. The track is drawn from the stylesheet's values, and it is
// left unfilled while the run is waiting or stalled -- a frozen fill claims a position
// the run cannot vouch for.
func (s *Shell) runProgressBar(c *ui.Context, theme *ui.Theme, view RunProgressView) {
	rail := Rail(s.state.Appearance, s.state.SystemDark, s.translator.Locale() == i18n.ZHCN)

	ui.Column(c).FillWidth().GapY(SpaceSmall).Padding(SpaceBase, SpaceBase).
		Background(rail.Muted).Radius(RadiusMedium).Border(1, rail.SubtleLine).Children(func() {
		// .options-autotag-progress-head: space-between with an 8 gap. Without the gap the phase
		// and the count ran together as "LLM120/2000" in the 280-wide sidebar -- this block was
		// written for the settings panel, which is several times wider, and round 453 only found
		// out by putting it where the source puts it.
		ui.Row(c).FillWidth().GapX(SpaceBase).Children(func() {
			ui.Text(c, view.Phase).FontSize(FontSmall).Bold().TextColor(theme.Text)
			ui.Text(c, view.Count).FontSize(FontMicro).Grow(1).
				TextAlign(ui.End).TextColor(theme.TextMuted)
		})

		// MyGo has no progress bar, so the track is drawn: a rounded fill over a
		// rounded ground, both from the stylesheet's values. While the run is waiting
		// or stalled nothing is filled -- a frozen fill claims a position the run
		// cannot vouch for, which is the source's stated reason for its indeterminate
		// state.
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

		// .options-autotag-progress-foot: align centre, gap 8, flex-wrap -- and in a 280-wide
		// sidebar it always wraps, which is why this is two rows rather than one.
		//
		// The first version put the status, the foot and the button in one wrapping row and the
		// foot column collapsed to one word per line ("Elapsed / 39s / · / ~611s / left"), because
		// a wrapping row gives every child its minimum width before it wraps. Two explicit rows say
		// what the source's wrap actually resolves to at this width: status and foot on one line,
		// the button across the panel under them.
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
// nothing in the window itself opens. The source saves from x.com's own page and has no in-app
// form, so this one is this build's addition and its entry point lives where a macOS app puts
// commands that have no place in the window.
func (s *Shell) ToggleAddForm() {
	s.state.AddOpen = !s.state.AddOpen
	if s.window != nil {
		s.window.Invalidate()
	}
}

// addRow is the entry point for a post the user pastes in.
//
// It has no counterpart in the extension, which is why it is shaped to the app rather than
// copied: the extension's save button lives on the x.com page and posts a tweet the content
// script already read. Without it a standalone manager could only ever show what a sync
// brought in.
func (s *Shell) addRow(c *ui.Context, theme *ui.Theme, rail RailTokens) {
	if !s.state.AddOpen {
		return
	}
	// The same field-and-action row as the toolbar: 28-point inputs with one hairline and a
	// compact action, on the page ground (app.css:3309-3316, :2151-2157).
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
		// page's invalid values use --red.
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

	// A card row, as magpie draws every standing bar: white, one hairline, a 12-point
	// radius (`.list`, app.css:633).
	ui.Column(c).FillWidth().Padding(12, 14).GapY(SpaceBase).
		Radius(RadiusResultCard).Border(1, rail.CardBorder).Background(theme.Surface).
		Children(func() {
			ui.Text(c, s.t().Tf(i18n.BatchSelected, len(ids))).FontSize(FontSmall).FontWeight(500).
				TextColor(theme.Text)

			ui.Row(c).FillWidth().GapX(8).Children(func() {
				// The field is this build's own, so nothing in the source names it: a bare
				// TextInput has no name, and a screen reader announced "text field" beside an
				// empty box with nothing saying what to type (round 39). The placeholder asks
				// the question; the label gives the field the same answer as its name.
				batchTagField := ui.TextInput(c, &s.state.TagInput).
					Placeholder(s.t().T(i18n.CreateTagPrompt)).
					Label(s.t().T(i18n.CreateTagPrompt))
				// Enter is the keyboard form of the tag button beside it. The source's batch bar
				// has only the picker and Apply, so this field is this build's own -- and a field
				// with no Enter beside a button that works is the same defect the sidebar's draft
				// row had.
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

				// .options-bulk-tag-select plus its apply button. The source pairs a <select>
				// of every tag with "Apply tag"; this is the same pair, and the typed-name path
				// below stays because it answers a different question.
				if tagLabels := s.tagLabels(); len(tagLabels) > 0 {
					// The placeholder is an option, as the source's empty-valued "Choose tag"
					// option is (BookmarkResultsPane.tsx:54-60). Without it the trigger drew a
					// bare chevron: nothing said which picker tags and which one moves.
					// The picker's name is the source's chooseTag, not the tag button's "Add
					// tag": two controls called "Add tag" side by side made the reader guess
					// which one opens a menu (round 39; BookmarkResultsPane.tsx:50-53).
					settingsPick(c, theme, rail, s.batchChooseTagLabel(),
						&s.state.BulkTagLabel,
						append([]string{s.batchChooseTagLabel()}, tagLabels...), s.invalidate)
					// Apply stays dark until a tag is chosen, the source's disabled state.
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

				// The source's two selection buttons, in its order: everything the query
				// matches, then what the list has loaded (BookmarkResultsPane.tsx:45-52).
				// At real scale they are not interchangeable -- the first refuses above
				// MaxLibraryPageSize, and the second is the one that still works (round 33).
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

// loadingState is the panel the source draws while the first read is in flight.
//
// It is a skeleton rather than a spinner: six placeholder cards in the same grid the real
// cards use, so the page does not jump when the read lands. The shell showed a bare
// spinner in an empty pane.

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
// Spec 6.7 gives it one row: search, filter, sort, the view-mode control, and the result count.
// It used to be a Column of five rows -- search and filter and sort, then the view toggles,
// then the run status, then Tidy and Settings, then the count -- so the strip was five lines
// tall and every control was a button with its own outline. That is the "six rows of stacked
// buttons" shape the layout was rewritten to remove once already, and it grew back one row at a
// time as each control was added.
//
// The two rows below the controls are conditional and empty in the state the screenshots are
// taken in, so the strip is one line there: the active filters, which are data rather than
// controls and have to be visible to be removable, and the run status, which is a sentence and
// not a control.
func (s *Shell) toolbar(c *ui.Context, theme *ui.Theme, rail RailTokens) {
	// .options-main-header { padding: 0 0 10px } (extension.css:1432-1438). The bottom inset
	// is 10, not the 8 this used: the bulk bar's own `margin-top: -6` is measured against it,
	// so the bar sat 2 points high and everything below it rode up with it.
	ui.Column(c).FillWidth().Padding(0, 0, ResultsHeaderPadBottom, 0).GapY(SpaceBase).
		Background(rail.Results).Children(func() {
		// magpie puts the page's controls straight on the page ground: no panel, no
		// outline -- only the search field and the labelled actions carry an edge
		// (app.css:869-922, :2135-2154). The extension's bordered toolbar shell is gone
		// with the rest of its chrome (docs/magpie-visual-spec.md).
		ui.Row(c).FillWidth().GapX(SpaceBase).Children(func() {
			searchField(c, theme, rail, &s.state.Query,
				s.t().T(i18n.SearchPlaceholder), s.t().T(i18n.ActionClear), &s.searchField,
				func() { s.ReloadAsync(context.Background()) },
				func() { s.ExitSearch(context.Background()) })

			// The active filters, in the search's own row and on its right.
			//
			// The source gives them a second row of the toolbar
			// (.options-toolbar-context-row, extension.css:1671-1673) with the label
			// "活跃筛选:" (WorkspaceToolbar.tsx:167-191). That is a full-width line under a
			// field that is already the longest control in the row, and selecting one
			// filter grew the toolbar by it -- the user reported the block twice before
			// the row moved here (shell-contract.md S25). Beside the field, a condition
			// adds no height: the chip is 28 points in the search's 36-point row.
			//
			// The group is bounded and wraps, which is the source's own answer to many
			// chips (`.options-active-filters { flex-wrap: wrap }`, extension.css:1781-1786).
			// Inline that rule needs a width to wrap against; without one, six conditions
			// would take the row from the search and the actions and push them off the edge.
			chips := s.activeConditions()
			if len(chips) > 0 {
				ui.Row(c).AlignItems(ui.Center).Wrap().GapX(SpaceSmall).GapY(SpaceSmall).
					MaxWidthPercent(ToolbarChipsMaxPercent).
					// The label is the source's, and it is still read out: the chips are a
					// group of conditions, and a screen reader gets their shared name even
					// though the row has no room to print it.
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

			// .options-toolbar-action: an icon and a word, not an icon with a tooltip. The
			// reference shows "筛选" beside the funnel and "最近保存" beside the arrows; this
			// build drew both as bare glyphs, so the only way to know what they were was to
			// hover one at a time.
			// PopoverBase rather than Popover: the panel's look is magpie's `.pop`, and the
			// toolkit's own panel is a different shape (S53).
			trigger := ui.Row(c).GapX(SpaceSmall).Children(func() {
				// .options-toolbar-action, with .options-toolbar-badge beside the word when
				// filters are on.
				ToolbarAction(c, theme, rail, "filter", s.t().T(i18n.ToolbarFilters),
					s.conditionCount(), s.state.FilterPopoverOpen, func() {
						s.ToggleFilterPopover()
					})
			})
			ui.PopoverBase(c, trigger, &s.state.FilterPopoverOpen, func(panel ui.Element) {
				s.filterPopover(c, theme, rail, panel, trigger)
			})

			// The sort control cycles, as the stylesheet's toolbar does: one click advances to the
			// next order and the label says which is active. The label is the order alone -- the
			// source renders getSortLabel(copy, sortOrder) with no prefix, so "Sort: Recently
			// saved" was a word the reference does not have.
			ToolbarAction(c, theme, rail, "sort", s.currentSortLabel(), 0, false, func() {
				s.CycleSort(context.Background())
			})

			// .options-toolbar-divider, between the actions and the view toggles.
			ui.Box(c).Width(1).Height(20).Background(rail.SubtleLine)

			// The source's two toggles are icon-only with aria-pressed: with two options,
			// seeing which is on matters more than saving a click.
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
			// Small and tertiary: at the source's 36 points it outranked the page's name.
			ui.Text(c, s.resultsCountLine()).FontSize(FontTiny).MaxLines(1).
				TextColor(rail.TextTertiary)
			// Nothing more here: the graph, the tag library and the vocabulary review belong to
			// the tags view (TagResultsPane renders TagGraphView and the review, and the sidebar's
			// 标签 header opens that view), and the settings gear belongs to the sidebar footer,
			// where the source puts it. Round 470 moved them rather than deleting them: the graph
			// and the review had no other trigger, so removing them here would have made both
			// unreachable -- round 242's defect in the opposite direction.

			// No trailing spacer. The source's primary row is a two-track grid,
			// `minmax(0, 1fr) auto` (extension.css:1606-1611): the search takes every
			// point the controls do not, and the controls end at the shell's inset. A
			// FillWidth spacer here competed with the search's Grow(1) for the same
			// slack and the search lost half of it -- measured: 505 wide against the
			// source's 649, with a blank strip after the view toggles.

			// Through ButtonSmall like the rest of the strip. It was a raw ui.Button, so it kept
			// the platform's outline and fill while the control beside it did not: the round-266
			// screenshot has "Sort: Recently saved" flat and "Tidy vocabulary" boxed, which is
			// the same rule applied to one of two neighbours.
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
		// The short word, not the sentence. The sentence that explains a failure belongs under the
		// sync button, which is the one place the source renders it (WorkspaceSidebar.tsx:405);
		// this line carried it too, so the same paragraph appeared twice across the window and in
		// the wrong place. syncProblemLine owns it now.
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
			// FormatSavedAt, not FormatTimestamp: the toolbar said
			// "Last sync 2026-10-17T10:52:03.889Z" until round 451. This was the third site
			// with the storage formatter, after the sidebar and the auto-tag panel -- fixing
			// one and not sweeping for the rest is how the first two survived.
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
// The stylesheet's .options-card-avatar carries them at 11px/700, and the source builds
// them rather than taking one: `authorName.split(/\s+/).filter(Boolean).slice(0, 2)
// .map(part => part[0]?.toUpperCase() ?? "").join("") || handle.slice(0, 2).toUpperCase()`
// (BookmarkCard.tsx:100-105). One letter per word, at most two -- so "Yassine Bouanane" is
// YB and "Margaret Hamilton" is MH. Taking only the first letter of the name drew a single
// glyph where the reference shows two.
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

// pendingSuggestionsBar draws .options-bulk-actions-bar for the tag suggestions waiting review.
//
// It is not batchBar: that one answers "what do I do with the rows I picked" and only appears when
// something is picked, while this appears whenever the workspace is holding suggestions, whatever
// is selected, because it is work the app is asking for rather than work the user started. Sharing
// a stylesheet class is not sharing a surface -- round 216 found two bars saying the same thing
// because the surface index was not consulted, and this is the same class family with a different
// job, so it is registered there as its own entry.
func (s *Shell) pendingSuggestionsBar(c *ui.Context, theme *ui.Theme, rail RailTokens) {
	n := len(s.state.PendingSuggestions)
	if n == 0 {
		return
	}
	// .options-theme-panel's ground is --theme-panel-bg, which is the panel surface, not
	// the pressed/hover surface. rail.Muted is theme.SurfacePressed -- a platform colour --
	// so the bar rendered cool grey (221,221,225) on a warm page while the source's is
	// (251,247,243).
	// `.options-bulk-actions-bar { margin-top: -6px; margin-bottom: 14px }`
	// (extension.css:1950-1961). The bottom margin is the whole gap between the bar and
	// the first card: without it the grid started at the bar's edge, 14 points high.
	ui.Row(c).FillWidth().Wrap().GapX(BulkBarGap).GapY(SpaceBase).
		Padding(BulkBarPadding).Radius(BulkBarRadius).
		Margin(-BulkBarMarginTop, 0, BulkBarMarginBottom, 0).
		Border(1, rail.SubtleLine).Background(theme.Surface).Children(func() {
		// .options-bulk-actions-meta: the tag glyph and "N 待确认标签", 13px and secondary.
		ui.Row(c).GapX(SpaceBase).Grow(1).Children(func() {
			ui.Box(c).Width(SecondaryButtonIcon).Height(SecondaryButtonIcon).
				Draw(func(p *ui.Painter, r ui.Rect) {
					drawIcon(p, "tag", r.X, r.Y, min32(r.W, r.H), theme.TextMuted)
				})
			// The source composes this as `{count} {pendingSuggestionsTitle}`, not as one
			// format string, so the count and the word stay separately translatable.
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
