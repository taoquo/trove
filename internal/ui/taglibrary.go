package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/i18n"
)

// The tag library: the screen tag management mode shows in place of the results.
//
// Its shape is a page rather than a panel: a header with the counts, one toolbar panel (the
// search, the sort, the actions, the filters), the bulk bar, and then the tags as table rows.
// Every row draws its tag's name, can be clicked, and the filter and sort buttons show their
// state; the rename field is reachable from the row it acts on.

// The tag library's own scale: measurements from the table and its chips.
//
// They live beside the drawing code rather than in metrics.go for the reason runprogress.go's
// track height does: a measure only this screen uses is easier to check next to the screen.
const (
	// The filter pill's scale: TagChipHeight is its minimum height, TagChipPadX its horizontal
	// padding and TagChipFont its text size; the active state carries the accent fill.
	TagChipHeight = 25
	TagChipPadX   = 10
	TagChipFont   = 12
	// The table follows the list: a row is 48 high with 7/12/7/14 of padding and a 12
	// gap, its name is the page's 13 at weight 500, and its metadata is the 11.5px sub line.
	// The head is the 11px/600 uppercase label on a 26-point strip.
	TagTableShellRadius = 12
	TagTableShellPad    = 0
	TagTableHeadHeight  = 26
	TagTableHeadFont    = 11
	TagTableHeadTrack   = 0.04
	TagTableRowMin      = 48
	TagTablePadY        = 12
	TagTablePadX        = 12
	// The fixed columns are what the name column has to work around. They used to take 52 of
	// the name's points at the 900-point minimum window, which truncated every tag name to an
	// ellipsis; these leave it about 100.
	TagTableCheckCol   = 40
	TagTableCountCol   = 80
	TagTableThemeCol   = 120
	TagTableUpdatedCol = 108
	// Four row actions (bookmarks, rename, aliases, delete) at 22 points each with 2 between.
	TagTableActionCol = 96
	// TagTableMinNameWidth is the name column below which the table drops the timestamp
	// instead: a row of ellipses says less than a missing column of dates.
	TagTableMinNameWidth = 120
	TagTableNameFont     = 13
	// TagTableMetaFont is the row's sub line: the aliases and the dates share it.
	TagTableMetaFont = 11.5
	// TagTableThemePick is the picker in the theme cell: tall enough to read as a field and
	// short enough to keep the row at its 40-point height while it is open.
	TagTableThemePick = 28
)

// The two pages of tag management.
//
// The tags table is where tags are read and edited; the auto-tagging page is where they are
// produced. A run, its result and the tag it writes belong together, so both are drawn inside
// TagMode, and exactly one is drawn at a time, which keeps the table's density the same as
// before.
const (
	TagViewTags    = "tags"
	TagViewAutoTag = "autotag"
)

// The fields a table row can edit in place.
//
// The table does not open a panel under the focused row, each row with its own Save. The name
// becomes an input in the row and the alias editor appears in the alias slot, the way Linear
// and Notion edit a field where it is read. One field is open at a time, so there is one draft
// to commit and no question about which Save writes what.
const (
	TagEditName    = "name"
	TagEditAliases = "aliases"
	TagEditTheme   = "theme"
)

// tagView is the page on screen. The zero value is the tags table: the screen exists to
// manage tags, so opening it must never land on the configuration page.
func (s *Shell) tagView() string {
	if s.state.TagView == TagViewAutoTag {
		return TagViewAutoTag
	}
	return TagViewTags
}

// tagLibraryText renders the tag management view.
func (s *Shell) tagLibraryText(c *ui.Context, theme *ui.Theme, rail RailTokens) {
	items := s.VisibleTagItems()
	all := s.state.TagItems

	ui.Column(c).Fill().FillHeight().Background(rail.Results).
		Padding(ResultsPadTop, s.pageInset(c), ResultsPadBottom, s.pageInset(c)).
		GapY(SpaceL).Children(func() {
		s.tagLibraryHeader(c, theme, rail)
		if s.tagView() == TagViewAutoTag {
			s.tagAutoTagView(c, theme, rail)
			return
		}
		s.tagLibraryToolbar(c, theme, rail)
		if s.state.TagActionMsg != "" {
			ui.Text(c, s.state.TagActionMsg).FontSize(FontSmall).TextColor(theme.Warning)
		}
		// The review's receipt, drawn where the review was started. Writing it to the results
		// toolbar's status line would be invisible, because the review is opened from this
		// page, which has no toolbar.
		if s.state.ReviewApplied != "" {
			ui.Text(c, s.state.ReviewApplied).FontSize(FontSmall).TextColor(theme.Success)
		}
		if s.state.TagCleanupMessage != "" {
			ui.Text(c, s.state.TagCleanupMessage).FontSize(FontSmall).TextColor(theme.TextMuted)
		}

		switch {
		case s.state.TagLibraryLoading && len(all) == 0:
			// The counts are read off the window's thread, so the first frame of the
			// page has nothing yet. "No tags created" would be a lie about that frame.
			ui.Column(c).Fill().Center().Padding(48).Children(func() {
				ui.Text(c, s.t().T(i18n.TagLibraryReading)).
					FontSize(FontBodyLarge).TextColor(theme.TextMuted)
			})
			return
		case len(all) == 0:
			// An empty library used to be one muted sentence at the left edge of a blank page,
			// under a toolbar whose create row was the only way forward. The card says what the
			// page is for and offers that action where the reader is looking.
			s.stateCard(c, theme, rail, stateCard{
				title:       s.t().T(i18n.TagNoTagsCreated),
				detail:      s.t().T(i18n.TagNoTagsDescription),
				actionLabel: s.t().T(i18n.CreateTagLabel),
				onAction:    func() { s.BeginLibraryCreateTag() },
			})
			return
		case len(items) == 0:
			// An empty library and a filter that matched nothing are different states, so they
			// get different sentences and a way back to the whole list.
			s.stateCard(c, theme, rail, stateCard{
				title:       s.t().T(i18n.TagNoMatchTitle),
				detail:      s.t().T(i18n.TagNoMatchDescription),
				actionLabel: s.t().T(i18n.FilterClearAll),
				onAction:    func() { s.clearTagLibraryFilters() },
			})
			return
		}

		ui.Scroll(c).FillWidth().Grow(1).Children(func() {
			// The table shell: radius 16, 4 points of padding, inside the page's own panel
			// border. Without it the rows and the separators float on the page ground with
			// nothing holding them together.
			ui.Column(c).FillWidth().Radius(TagTableShellRadius).
				Border(1, rail.CardBorder).Background(theme.Surface).
				Padding(TagTableShellPad).Children(func() {
				s.tagTableHead(c, theme, rail, items)
				for i, item := range items {
					s.tagTableRow(c, theme, rail, item, i == len(items)-1)
				}
			})
		})
	})
}

// tagLibraryToolbar is the one panel holding this page's controls.
//
// Three rows, each with one job: the search, the order and the graph; the filter pills and what
// they left; then the actions -- creation on the left, maintenance on the right. It used to put
// the exit beside the search and the actions alone on a third row whose left two thirds were
// empty ground. The exit moved to the header breadcrumb, and the actions now start at the left
// edge. A selection replaces the action row with the bulk bar: the verbs a checked row enables
// are a different set, and drawing both made that row the longest one on the page.
//
// A row of pills shows which filter is in effect without opening anything, and the active
// pill carries the accent fill so the state is visible.
func (s *Shell) tagLibraryToolbar(c *ui.Context, theme *ui.Theme, rail RailTokens) {
	empty := len(s.state.TagItems) == 0
	// An empty library has nothing to search, order or filter, and no rows to act on. The empty
	// card below carries the one action that changes that, so the strip disappears with the
	// table; only the draft row still needs a panel to open in.
	if empty && !s.state.TagLibraryCreateOpen {
		return
	}
	ui.Column(c).FillWidth().GapY(SpaceBase).Children(func() {
		if !empty {
			ui.Row(c).FillWidth().GapX(SpaceBase).Children(func() {
				// The translated field: ui.SearchField's placeholder cannot be changed, which is
				// why this screen said "Search" in a Chinese window.
				// Any narrowing drops the ticks: a selection whose rows are no longer on
				// screen still armed the bulk bar, so Delete removed tags the reader could
				// not see.
				searchField(c, theme, rail, &s.state.TagQuery,
					s.t().T(i18n.TagSearchPlaceholder), s.t().T(i18n.ActionClear), nil,
					func() { s.state.TagSelectedIDs = map[string]bool{}; s.invalidate() },
					func() {
						s.state.TagQuery = ""
						s.state.TagSelectedIDs = map[string]bool{}
						s.invalidate()
					})
				s.tagSortSelect(c, theme, rail)
				// A hairline divider between the reading controls and the view.
				ui.Box(c).Width(1).Height(20).Background(rail.SubtleLine)
				ToolbarAction(c, theme, rail, "graph", s.t().T(i18n.ToolbarGraph), 0, false, func() {
					s.ToggleGraph(context.Background())
				})
			})

			// The filters get this row to themselves. Sharing it with the three actions made every
			// pill narrow enough to wrap its own label onto two lines, which is the opposite of
			// what a one-line filter strip is for.
			ui.Row(c).FillWidth().GapX(SpaceSmall).AlignItems(ui.Center).Children(func() {
				// How many tags are waiting on a suggestion. Without a number the filter a
				// reader is meant to start from gives no sign that it has anything behind it.
				pendingTags := 0
				for _, item := range s.state.TagItems {
					if item.PendingSuggestionCount > 0 {
						pendingTags++
					}
				}
				for _, option := range []struct {
					filter domain.TagLibraryFilter
					key    i18n.Key
					count  int
				}{
					{filter: domain.TagFilterAll, key: i18n.TagFilterAll},
					{filter: domain.TagFilterOrphan, key: i18n.TagFilterOrphan},
					{filter: domain.TagFilterLongTail, key: i18n.TagFilterLongTail},
					{filter: domain.TagFilterPending, key: i18n.TagFilterPending, count: pendingTags},
					{filter: domain.TagFilterUnparented, key: i18n.TagFilterUnparented},
				} {
					option := option
					s.tagLibraryChip(c, theme, rail, s.t().T(option.key), option.count,
						s.state.TagFilter == option.filter, func() {
							if s.state.TagFilter != option.filter {
								// A different filter shows different rows; keeping ticks for
								// rows that just left the screen armed Delete for invisible
								// tags.
								s.state.TagSelectedIDs = map[string]bool{}
							}
							s.state.TagFilter = option.filter
							if s.window != nil {
								s.window.Invalidate()
							}
						})
				}
				// What the query and the filter left, on the row that did the narrowing. The row
				// used to be the chips alone and half empty; a narrowed list is exactly when the
				// reader wants the number.
				if summary := s.tagLibrarySummary(s.VisibleTagItems()); summary != "" {
					ui.Spacer(c).FillWidth()
					ui.Text(c, summary).FontSize(FontSmall).TextColor(rail.TextTertiary).MaxLines(1)
				}
			})

			// The actions. A selection replaces them: what a checked row enables is a different
			// set of verbs, and drawing both sets at once made this row the longest one on the
			// page.
			if s.TagSelectionCount() > 0 {
				s.tagLibraryBulkControls(c, theme, rail)
			} else {
				ui.Row(c).FillWidth().GapX(SpaceBase).AlignItems(ui.Center).Children(func() {
					// The one action this screen could not offer: making a tag. It was reachable
					// only from the sidebar's "+", a row away, so a reader who came here to do
					// something about the library found a screen of controls and no way to add
					// to it.
					ToolbarAction(c, theme, rail, "hash", s.t().T(i18n.CreateTagLabel), 0,
						s.state.TagLibraryCreateOpen, func() { s.BeginLibraryCreateTag() })
					ui.Spacer(c).FillWidth()
					ToolbarAction(c, theme, rail, "sparkle", s.t().T(i18n.ReviewLabel), 0, false, func() {
						s.OpenVocabularyReview()
					})
					// Each action is drawn only when it can do something. A sweep with no orphan
					// to sweep is a control whose only possible answer is no.
					if removable := len(domain.GetRemovableOrphanTagIDs(s.state.TagItems)); removable > 0 {
						ToolbarAction(c, theme, rail, "trash", s.t().T(i18n.TagCleanupOrphans),
							removable, false, func() { s.AskCleanupOrphanTags() })
					}
				})
			}
		}

		if s.state.TagLibraryCreateOpen {
			// The same draft rules as the sidebar's row: Enter creates, Escape closes, and
			// an empty submit closes rather than sitting there (CreateTagFromDraft). The two
			// buttons stay because this management screen has no other create control, and
			// across a full-width content column a form with its actions reads as intended.
			ui.Row(c).FillWidth().GapX(SpaceBase).AlignItems(ui.Center).Children(func() {
				// Named as well as prompted: the placeholder is painted, not a name, so the
				// field would reach a screen reader as a bare text field.
				field := settingsInput(c, theme, rail, &s.state.TagDraft).
					Placeholder(s.t().T(i18n.CreateTagPrompt)).
					Label(s.t().T(i18n.CreateTagPrompt)).
					Grow(1).Bind(&s.tagLibraryDraftField)
				field.OnSubmit(func() { s.CreateTagFromDraft(context.Background()) })
				field.OnShortcut(0, ui.KeyEscape, func() { s.CancelCreateTag() })
				settingsAction(c, theme, rail, s.t().T(i18n.ActionAdd), false, false, func() {
					s.CreateTagFromDraft(context.Background())
				})
				settingsAction(c, theme, rail, s.t().T(i18n.ActionCancel), false, false, func() {
					s.CancelCreateTag()
				})
			})
		}
	})
}

// tagLibrarySummary says how much of the library the query and filter left, and says nothing
// when they left all of it: the header already carries the total, so the line exists for the
// narrowed case only.
func (s *Shell) tagLibrarySummary(items []domain.TagLibraryItem) string {
	all := len(s.state.TagItems)
	if len(items) == all {
		return ""
	}
	if all == 0 {
		return ""
	}
	return fmt.Sprintf("%d / %d %s", len(items), all, s.t().T(i18n.TagTagsSuffix))
}

// clearTagLibraryFilters undoes the narrowing that produced an empty table.
//
// Both halves are cleared, because either one can empty the list and the empty state cannot tell
// the reader which it was.
func (s *Shell) clearTagLibraryFilters() {
	s.state.TagQuery = ""
	s.state.TagFilter = domain.TagFilterAll
	if s.window != nil {
		s.window.Invalidate()
	}
}

// tagSortSelect is the order control: one drop-down rather than four buttons.
//
// It is the pick the settings page's rows use rather than a
// bordered box around a native select: one pick looks and opens the same everywhere in the
// shell, and the toolbar loses the second outline it used to carry.
func (s *Shell) tagSortSelect(c *ui.Context, theme *ui.Theme, rail RailTokens) {
	orders := []struct {
		order domain.TagLibrarySortOrder
		key   i18n.Key
	}{
		{domain.TagSortCountDesc, i18n.TagSortMost},
		{domain.TagSortCountAsc, i18n.TagSortFewest},
		{domain.TagSortNameAsc, i18n.TagSortName},
		{domain.TagSortRecent, i18n.TagSortRecent},
	}
	options := make([]string, 0, len(orders))
	current := ""
	for _, option := range orders {
		label := s.t().T(option.key)
		options = append(options, label)
		if option.order == s.state.TagSort {
			current = label
		}
	}
	if current == "" {
		current = options[0]
	}

	pick := current
	settingsPick(c, theme, rail, s.t().T(i18n.TagSortMost), &pick, options, func() {
		for _, option := range orders {
			if s.t().T(option.key) == pick {
				s.state.TagSort = option.order
				break
			}
		}
	})
}

// tagLibraryChip is the filter row's pill.
func (s *Shell) tagLibraryChip(c *ui.Context, theme *ui.Theme, rail RailTokens, label string, count int, active bool, onClick func()) {
	// The pill's own accessible name carries the count, which is otherwise a second text run
	// inside the pill: a focus stop that announces "Pending review" and then a bare number is
	// harder to read than one name, and the count is what makes the filter worth pressing.
	name := label
	if count > 0 {
		name = fmt.Sprintf("%s %d", label, count)
	}
	chip := ui.Row(c).MinHeight(TagChipHeight).Padding(0, TagChipPadX).GapX(SpaceSmall).
		Radius(RadiusPill).AlignItems(ui.Center).Focusable().Label(name)
	// A chip and its on state: a white pill with one hairline and the quiet-control ink;
	// on (or hovered) it takes the accent's edge, the accent's ink and the accent-soft
	// ground. The old active state was a translucent accent wash
	// on the page, which read as a highlighted row rather than as a chosen filter.
	switch {
	case active:
		chip = chip.Border(1, theme.Accent).Background(rail.BadgeBg)
	case chip.Hovered():
		chip = chip.Border(1, theme.Accent).Background(rail.BadgeBg)
	default:
		chip = chip.Border(1, rail.CardBorder).Background(theme.Surface)
	}
	ink := controlTextColour(theme.Dark)
	if active || chip.Hovered() {
		ink = theme.Accent
	}
	// MaxLines(1): a pill that wraps its own label is not a pill. The row has the filters to
	// itself, so this only bites on a window narrow enough to be a problem anyway.
	chip.OnClick(onClick).Children(func() {
		ui.Text(c, label).FontSize(TagChipFont).TextColor(ink).MaxLines(1)
		if count > 0 {
			ui.Text(c, fmt.Sprintf("%d", count)).FontSize(TagChipFont).TextColor(ink).MaxLines(1)
		}
	})
}

// tagLibraryBulkControls is the bulk bar: the selection, the merge it feeds, and the delete,
// with the way out at the far end.
//
// It replaces the action row while something is selected, so the panel never carries two sets
// of verbs. Merge belongs here rather than between two rows of unrelated controls, where it
// read as another filter.
func (s *Shell) tagLibraryBulkControls(c *ui.Context, theme *ui.Theme, rail RailTokens) {
	ui.Row(c).FillWidth().GapX(SpaceBase).AlignItems(ui.Center).Children(func() {
		ui.Text(c, s.t().Tf(i18n.TagBatchSelected, s.TagSelectionCount())).
			FontSize(FontSmall).TextColor(theme.Text)
		ui.Text(c, s.t().T(i18n.TagMergeInto)).FontSize(FontSmall).TextColor(theme.TextMuted)
		// The target is picked from the library's tags, and it starts unchosen: a merge deletes
		// what it folds in, so the one tag that survives is a choice, not whichever tag sorts
		// first. The picker opens on its own label and the Merge button waits for a choice;
		// pre-filling the first tag would silently merge ticked rows into it.
		//
		// The placeholder is a local, not a write to the state: a view may be rebuilt several
		// times per frame. ui.Select reports a choice only when it differs from the value it was
		// drawn with, and every real tag differs from the placeholder, so picking any tag
		// writes it. Picking the placeholder back clears the choice.
		placeholder := s.t().T(i18n.TagMergeChoose)
		labels := append([]string{placeholder}, tagNames(s.state.TagItems)...)
		target := s.state.TagMergeTarget
		if _, found := s.mergeTargetTag(); !found {
			target = placeholder
		}
		settingsPick(c, theme, rail, s.t().T(i18n.TagMergeInto), &target, labels, func() {
			if target == placeholder {
				target = ""
			}
			s.state.TagMergeTarget = target
		})
		_, chosen := s.mergeTargetTag()
		SecondaryButtonWhen(c, "merge", s.t().T(i18n.TagMergeLabel), theme, rail,
			chosen && !s.state.Mutating, func() {
				s.AskMergeSelectedTags()
			})
		ToolbarAction(c, theme, rail, "trash", s.t().T(i18n.TagDeleteLabel),
			s.TagSelectionCount(), false, func() { s.AskDeleteSelectedTags() })
		ui.Spacer(c).FillWidth()
		// A word rather than an ✕ glyph: the bar is the one place a reader can undo a
		// multi-row tick, and an unlabelled glyph at its far end is not where that is found.
		settingsAction(c, theme, rail, s.t().T(i18n.ActionClearSelection), false, false, func() {
			s.ClearTagSelection()
		})
	})
}

// tagTableFitsAllColumns reports whether the pane has room for the timestamp as well as a
// usable name.
//
// At the 900-point minimum window the content pane is about 550 points and the fixed columns
// alone took 508, so the name column was 43 wide and every tag read as an ellipsis while the
// last row action was clipped. The timestamp is the one column a reader can spare; the name is
// the tag and the counts are the library. A viewport breakpoint, not a state: the row and the
// head both ask the context they are drawn in, so they cannot disagree.
func (s *Shell) tagTableFitsAllColumns(c *ui.Context) bool {
	// c.Size() is the window's content, not this pane (mygo's Context.Size). The pane is the
	// window minus the sidebar and the page's horizontal padding, and the table shell adds its
	// own inset inside that.
	window, _ := c.Size()
	width := window - s.sidebarPaneWidth() - 2*ResultsPadX - 2*TagTableShellPad
	fixed := TagTableCheckCol + TagTableThemeCol + 2*TagTableCountCol +
		TagTableUpdatedCol + TagTableActionCol + 2*TagTablePadX
	return width >= float32(fixed+TagTableMinNameWidth)
}

// tagTableHead is the column names above the rows, and the checkbox that takes the listed ones.
//
// A table because the columns are what make a library of dozens comparable. The head's
// checkbox is the select column; a separate "select all listed" button in a bar would be one
// more control for the same job.
func (s *Shell) tagTableHead(c *ui.Context, theme *ui.Theme, rail RailTokens, items []domain.TagLibraryItem) {
	allSelected := len(items) > 0
	for _, item := range items {
		if !s.state.TagSelectedIDs[item.TagID] {
			allSelected = false
			break
		}
	}

	ui.Row(c).FillWidth().MinHeight(TagTableHeadHeight).
		Padding(0, TagTablePadX).BorderWidth(0, 0, 1, 0).
		BorderColor(rail.SubtleLine).AlignItems(ui.Center).Children(func() {
		ui.Row(c).Width(TagTableCheckCol).Children(func() {
			selectAll := allSelected
			ui.Checkbox(c, &selectAll, "").Label(s.t().T(i18n.TagSelectAll)).
				OnChange(func() {
					if selectAll {
						// The header's checkbox is the select-all action, in the table's
						// select column.
						s.SelectListedTags(items)
						return
					}
					for _, item := range items {
						delete(s.state.TagSelectedIDs, item.TagID)
					}
				})
		})
		ui.Text(c, strings.ToUpper(s.t().T(i18n.TagTableName))).
			FontSize(TagTableHeadFont).FontWeight(600).
			LetterSpacing(trackingEm(TagTableHeadTrack, TagTableHeadFont)).
			TextColor(rail.TextTertiary).Grow(1)
		// The theme is text, so its name starts where its values start. Right-aligning it with the
		// numbers left it over the wrong end of the column, which the screenshot showed.
		s.tagTableHeadCellStart(c, rail, i18n.TagTableParent, TagTableThemeCol)
		s.tagTableHeadCell(c, rail, i18n.TagTableBookmarks, TagTableCountCol)
		s.tagTableHeadCell(c, rail, i18n.TagTablePending, TagTableCountCol)
		if s.tagTableFitsAllColumns(c) {
			s.tagTableHeadCell(c, rail, i18n.TagTableUpdated, TagTableUpdatedCol)
		}
		ui.Box(c).Width(TagTableActionCol)
	})
}

// tagTableHeadCell is one column name, aligned like the cells under it.
func (s *Shell) tagTableHeadCell(c *ui.Context, rail RailTokens, key i18n.Key, width float32) {
	ui.Row(c).Width(width).Justify(ui.End).Children(func() {
		ui.Text(c, strings.ToUpper(s.t().T(key))).FontSize(TagTableHeadFont).FontWeight(600).
			LetterSpacing(trackingEm(TagTableHeadTrack, TagTableHeadFont)).
			TextColor(rail.TextTertiary)
	})
}

// tagTableHeadCellStart is tagTableHeadCell for a column whose cells start at its left edge.
func (s *Shell) tagTableHeadCellStart(c *ui.Context, rail RailTokens, key i18n.Key, width float32) {
	ui.Row(c).Width(width).Children(func() {
		ui.Text(c, strings.ToUpper(s.t().T(key))).FontSize(TagTableHeadFont).FontWeight(600).
			LetterSpacing(trackingEm(TagTableHeadTrack, TagTableHeadFont)).
			TextColor(rail.TextTertiary)
	})
}

// tagTableRow is one tag.
//
// Every part is a target of its own: the checkbox selects it for merging, its name opens its
// bookmarks, the pencil turns the name into an input, the tag icon edits the aliases where
// they are read, the theme cell picks a theme, and the trash asks before deleting. The row
// itself takes no click -- a row that both selects and navigates leaves the reader guessing
// which one a click did.
//
// Editing happens in the cell that shows the value, not in a panel under the row: the name
// becomes an input and the alias editor appears in the alias slot. That is the shape Linear
// and Notion use: one open field, saved where it is read, so there is never a question about
// which of three Saves writes what.
func (s *Shell) tagTableRow(c *ui.Context, theme *ui.Theme, rail RailTokens, item domain.TagLibraryItem, last bool) {
	focused := s.state.TagFocus == item.TagID
	editingName := focused && s.state.TagEditField == TagEditName
	editingAliases := focused && s.state.TagEditField == TagEditAliases
	editingTheme := focused && s.state.TagEditField == TagEditTheme
	checked := s.state.TagSelectedIDs[item.TagID]

	// The last row does not draw a line against the panel's own edge.
	var bottom float32
	if !last {
		bottom = 1
	}
	row := ui.Row(c).FillWidth().MinHeight(TagTableRowMin).
		Padding(TagTablePadY, TagTablePadX).BorderWidth(0, 0, bottom, 0).
		BorderColor(rail.SubtleLine.Alpha(0.6)).AlignItems(ui.Center)
	switch {
	case focused:
		// The row being edited keeps a mark after the pointer leaves: the open field is the one
		// thing on screen that belongs to it. Its ground has to differ from the panel's own
		// surface, so it is the accent tint plus a 3-point bar.
		row = row.Background(theme.Accent.Alpha(0.08)).Draw(func(p *ui.Painter, r ui.Rect) {
			p.Fill(ui.Rect{X: r.X, Y: r.Y, W: 3, H: r.H}, theme.Accent.Alpha(0.88), 2)
		})
	case checked:
		row = row.Background(theme.Accent.Alpha(0.04))
	case row.Hovered():
		row = row.Background(theme.SurfaceHover)
	}

	row.Children(func() {
		ui.Row(c).Width(TagTableCheckCol).Children(func() {
			selected := checked
			// Named after its row: a column of unnamed boxes gives a screen reader nothing,
			// and one repeated name gives it the wrong row.
			ui.Checkbox(c, &selected, "").
				Label(s.t().Tf(i18n.TagSelectRowFor, item.Name)).OnChange(func() {
				if s.state.TagSelectedIDs == nil {
					s.state.TagSelectedIDs = map[string]bool{}
				}
				s.state.TagSelectedIDs[item.TagID] = selected
			})
		})

		// The name cell: the hash, the name, and the aliases under it. The name identifies the
		// row; a count and a parent alone do not. While either draft is open, that line becomes
		// the field: the name at the name's place, the aliases at the aliases' place.
		ui.Column(c).Grow(1).GapY(1).Children(func() {
			ui.Row(c).GapX(NavRowMainGap).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Width(NavRowIcon).Height(NavRowIcon).
					Draw(func(p *ui.Painter, r ui.Rect) {
						drawIcon(p, "hash", r.X, r.Y, min32(r.W, r.H), rail.TextTertiary)
					})
				if editingName {
					ui.TextInput(c, &s.state.TagNameEdit).
						Label(s.t().T(i18n.TagRenameLabel)).
						FontSize(TagTableNameFont).FontWeight(500).Grow(1).Bind(&s.tagEditField).
						OnSubmit(func() { s.commitTagEdit(func(ctx context.Context) { s.RenameFocusedTag(ctx) }) }).
						OnShortcut(0, ui.KeyEscape, func() { s.closeTagEdit() })
					return
				}
				ui.Box(c).Radius(RadiusMedium).Padding(0, 2).
					// The name is clipped to one line here; the tooltip is where the whole
					// name is readable, which is why it is the name and not only the action.
					Tooltip(item.Name).Label(item.Name).Focusable().
					OnClick(func() { s.OpenTagBookmarks(context.Background(), item.TagID) }).
					Children(func() {
						text := ui.Text(c, item.Name).FontSize(TagTableNameFont).FontWeight(500).
							TextColor(theme.Text).MaxLines(1)
						if focused {
							text = text.TextColor(theme.Accent)
						}
					})
			})
			if editingAliases {
				s.tagAliasEditor(c, theme, rail)
				return
			}
			// The second line: the aliases, and how many tags hang off this one. The child
			// count has no column of its own, but a theme is only recognisable by it, and the
			// table draws one row shape.
			meta := make([]string, 0, 2)
			if len(item.Aliases) > 0 {
				meta = append(meta, strings.Join(item.Aliases, ", "))
			}
			if item.ChildCount > 0 {
				meta = append(meta, fmt.Sprintf("%d %s",
					item.ChildCount, s.t().T(i18n.TagChildCountSuffix)))
			}
			if len(meta) > 0 {
				ui.Text(c, strings.Join(meta, " · ")).FontSize(TagTableMetaFont).
					TextColor(rail.TextTertiary).MaxLines(1)
			}
		})

		// The theme column: a value at rest, a picker while it is being changed. A select on
		// every row would put a control in a table of dozens; the value reads as text until the
		// reader asks for the picker, which is how a database property cell behaves.
		ui.Row(c).Width(TagTableThemeCol).Children(func() {
			if editingTheme {
				options := []string{s.t().T(i18n.TagParentNone)}
				for _, candidate := range domain.GetRootTagCandidates(s.state.TagItems) {
					if candidate.TagID != item.TagID {
						options = append(options, candidate.Name)
					}
				}
				if !containsString(options, s.state.TagParentPick) {
					s.state.TagParentPick = options[0]
				}
				// SelectBase rather than Select: the platform's select carries a 140-point
				// minimum (mygo's coreSelect), which pushed this row's counts off their column
				// names. The trigger is drawn to the column's own width instead.
				picker := ui.SelectBase(c, &s.state.TagParentPick)
				picker.Trigger.Width(TagTableThemeCol).MinHeight(TagTableThemePick).
					Padding(0, 6).Radius(RadiusMedium).Border(1, rail.SubtleLine).
					Background(theme.Surface).Bind(&s.tagEditField).GapX(4).Justify(ui.SpaceBetween).
					AlignItems(ui.Center).
					OnChange(func() { s.commitTagEdit(func(ctx context.Context) { s.SetTagTheme(ctx) }) }).
					OnShortcut(0, ui.KeyEscape, func() { s.closeTagEdit() }).
					Children(func() {
						ui.Text(c, s.state.TagParentPick).FontSize(FontSmall).
							TextColor(theme.Text).MaxLines(1).Grow(1)
						ui.Box(c).Width(10).Height(10).Draw(func(p *ui.Painter, r ui.Rect) {
							drawIcon(p, "caretDown", r.X, r.Y, min32(r.W, r.H), rail.TextTertiary)
						})
					})
				picker.Popup(func(panel ui.Element) {
					panel.Padding(4).Radius(RadiusMedium).Border(1, rail.SubtleLine).
						Background(theme.Surface)
					for _, option := range options {
						item := picker.Item(option).Padding(4, 8).Radius(6)
						switch {
						case item.Highlighted():
							item = item.Background(theme.Accent).TextColor(theme.AccentText)
						case option == s.state.TagParentPick:
							item = item.Background(theme.SurfaceHover)
						}
						item.Children(func() {
							ui.Text(c, option).FontSize(FontSmall)
						})
					}
				})
				return
			}
			parent := item.ParentName
			if parent == "" {
				parent = s.t().T(i18n.TagParentNone)
			}
			// The cell's name says which tag it themes, not only which value it holds: without
			// the tag's name, every unthemed row would read out the same "No theme".
			cell := ui.Box(c).Radius(RadiusMedium).Padding(0, 2).
				Tooltip(s.t().T(i18n.TagParentLabel)).
				Label(s.t().Tf(i18n.TagThemeFor, item.Name)).Focusable().
				OnClick(func() { s.beginTagEdit(item.TagID, TagEditTheme) })
			if cell.Hovered() {
				cell = cell.Background(theme.SurfaceHover)
			}
			cell.Children(func() {
				ui.Text(c, parent).FontSize(FontSmall).TextColor(theme.TextMuted).MaxLines(1)
			})
		})

		s.tagTableCount(c, theme, rail, item.BookmarkCount)
		s.tagTableCount(c, theme, rail, item.PendingSuggestionCount)
		if s.tagTableFitsAllColumns(c) {
			ui.Row(c).Width(TagTableUpdatedCol).Justify(ui.End).Children(func() {
				ui.Text(c, domain.FormatSavedAt(item.UpdatedAt, s.translator.Locale())).
					FontSize(TagTableMetaFont).TextColor(rail.TextTertiary).MaxLines(1)
			})
		}

		// This row's own controls, at rest in the quaternary ink and taking colour with the
		// row.
		ink := rail.TextQuaternary
		if row.Hovered() || focused {
			ink = rail.TextTertiary
		}
		ui.Row(c).Width(TagTableActionCol).GapX(2).Justify(ui.End).Children(func() {
			// Named with the row's tag: four identical verbs down a table of dozens cannot say
			// which row they act on.
			for _, action := range []struct {
				icon, label string
				run         func()
			}{
				{"bookmark", s.t().Tf(i18n.TagBookmarksFor, item.Name), func() {
					s.OpenTagBookmarks(context.Background(), item.TagID)
				}},
				{"edit", s.t().Tf(i18n.TagRenameFor, item.Name), func() {
					s.beginTagEdit(item.TagID, TagEditName)
				}},
				{"tag", s.t().Tf(i18n.TagAliasesFor, item.Name), func() {
					s.beginTagEdit(item.TagID, TagEditAliases)
				}},
				{"trash", s.t().Tf(i18n.TagDeleteFor, item.Name), func() { s.AskDeleteSidebarTag(item.TagID) }},
			} {
				action := action
				ui.Box(c).Width(NavRowAction).Height(NavRowAction).Radius(RadiusPill).Focusable().
					Label(action.label).Tooltip(action.label).OnClick(action.run).
					Draw(func(p *ui.Painter, r ui.Rect) {
						drawIcon(p, action.icon, r.X+(r.W-12)/2, r.Y+(r.H-12)/2, 12, ink)
					})
			}
		})
	})
}

// tagAliasEditor is the alias line's field: an input, Save and cancel (✕). It sits on the
// alias line rather than in a panel so the field and the value it replaces occupy the same
// place.
func (s *Shell) tagAliasEditor(c *ui.Context, theme *ui.Theme, rail RailTokens) {
	ui.Row(c).GapX(SpaceSmall).AlignItems(ui.Center).Children(func() {
		ui.TextInput(c, &s.state.TagAliasInput).
			Label(s.t().T(i18n.TagAliasesLabel)).
			Placeholder(s.t().T(i18n.TagAliasesPlaceholder)).
			FontSize(TagTableMetaFont).Grow(1).Bind(&s.tagEditField).
			OnSubmit(func() { s.commitTagEdit(func(ctx context.Context) { s.SaveTagAliases(ctx) }) }).
			OnShortcut(0, ui.KeyEscape, func() { s.closeTagEdit() })
		SecondaryButton(c, "", s.t().T(i18n.ActionSave), theme, rail, func() {
			s.commitTagEdit(func(ctx context.Context) { s.SaveTagAliases(ctx) })
		})
		ui.Box(c).Width(NavRowAction).Height(NavRowAction).Radius(RadiusPill).
			Label(s.t().T(i18n.ActionCancel)).Tooltip(s.t().T(i18n.ActionCancel)).Focusable().
			OnClick(func() { s.closeTagEdit() }).
			Draw(func(p *ui.Painter, r ui.Rect) {
				drawIcon(p, "close", r.X+(r.W-12)/2, r.Y+(r.H-12)/2, 12, rail.TextTertiary)
			})
	})
}

// tagTableCount is one number cell, right-aligned under its column name.
func (s *Shell) tagTableCount(c *ui.Context, theme *ui.Theme, rail RailTokens, count int) {
	ui.Row(c).Width(TagTableCountCol).Justify(ui.End).Children(func() {
		ink := theme.TextMuted
		if count == 0 {
			ink = rail.TextQuaternary
		}
		ui.Text(c, domain.FormatCompactCount(count, s.translator.Locale())).
			FontSize(FontSmall).TextColor(ink)
	})
}

// beginTagEdit opens one field of one tag for editing in its own cell.
//
// The drafts are reset to what the tag is now, so a name typed for one tag can never be saved
// onto another. Opening a field closes whichever one was open: one draft, one Save, one
// question answered.
func (s *Shell) beginTagEdit(tagID, field string) {
	var focused domain.TagLibraryItem
	for _, item := range s.state.TagItems {
		if item.TagID == tagID {
			focused = item
			break
		}
	}
	s.state.TagFocus = tagID
	s.state.TagEditField = field
	s.state.TagNameEdit = focused.Name
	s.state.TagAliasInput = strings.Join(focused.Aliases, ", ")
	s.state.TagParentPick = focused.ParentName
	if s.state.TagParentPick == "" {
		s.state.TagParentPick = s.t().T(i18n.TagParentNone)
	}
	// Once, here: a Focus() inside the view took the cursor back on every frame, so Tab could
	// not leave an open editor.
	s.tagEditField.Focus()
	if s.window != nil {
		s.window.Invalidate()
	}
}

// commitTagEdit writes the open field and closes it when the write succeeded.
//
// A refusal keeps the field open with the tag's current text still in it: the message the use
// case put in TagActionMsg is the only place the reason appears, and closing the field would
// also throw away what the reader typed.
func (s *Shell) commitTagEdit(commit func(context.Context)) {
	commit(context.Background())
	if s.state.TagActionMsg == "" {
		s.closeTagEdit()
	}
}

// closeTagEdit closes the open field without writing it.
func (s *Shell) closeTagEdit() {
	s.state.TagFocus = ""
	s.state.TagEditField = ""
	s.state.TagNameEdit = ""
	s.state.TagAliasInput = ""
	s.state.TagParentPick = ""
	if s.window != nil {
		s.window.Invalidate()
	}
}

// OpenTagBookmarks leaves the tag library and shows that tag's bookmarks.
//
// A tag's name is the way to its bookmarks. Without that path the library could inspect tags
// and never show what was filed under them.
func (s *Shell) OpenTagBookmarks(ctx context.Context, tagID string) {
	if tagID == "" {
		return
	}
	s.state.ActiveTagIDs = []string{tagID}
	s.state.TagMode = false
	s.state.GraphMode = false
	s.ReloadAsync(ctx)
}

// tagNames is the merge picker's options: every tag in the library, in the order it is shown.
func tagNames(items []domain.TagLibraryItem) []string {
	labels := make([]string, 0, len(items))
	for _, item := range items {
		labels = append(labels, item.Name)
	}
	return labels
}

// containsString reports whether list holds value.
func containsString(list []string, value string) bool {
	for _, candidate := range list {
		if candidate == value {
			return true
		}
	}
	return false
}
