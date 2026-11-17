package ui

import (
	"context"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/i18n"
	"github.com/taoquo/trove/internal/usecase/workspace"
)

// The sidebar's two facet lists, one tab at a time.
//
// Two long lists in one 280-point column at a laptop window height leave three author rows on
// screen and the rest behind a six-row cap; this build gives the visible list the whole column
// instead and keeps the other one a click away, with a dot when it is the one that is filtering.

const (
	// sidebarTabTags is the default, and the zero value of State.SidebarFacetTab.
	sidebarTabTags = "tags"
	// sidebarTabAuthors shows the author list in the same slot.
	sidebarTabAuthors = "authors"
)

// sidebarTab is the tab in effect, with "" read as the tags tab.
func (s *Shell) sidebarTab() string {
	if s.state.SidebarFacetTab == sidebarTabAuthors {
		return sidebarTabAuthors
	}
	return sidebarTabTags
}

// selectSidebarTab switches which facet list the column shows.
//
// Only the view changes: both filters stay in effect, and switching tabs is navigation rather
// than a way to drop a condition. The other tab carries a dot while its list is the one
// narrowing the results, which is the only way the sidebar can show two facets in one slot.
func (s *Shell) selectSidebarTab(tab string) {
	if s.sidebarTab() == tab {
		return
	}
	s.state.SidebarFacetTab = tab
	if s.window != nil {
		s.window.Invalidate()
	}
}

// sidebarFacetTabs draws the segmented control and the controls that belong to the active tab.
func (s *Shell) sidebarFacetTabs(c *ui.Context, theme *ui.Theme, rail RailTokens) {
	// One row, wrapping only when it has to: the tabs at the left, the tab's controls at the
	// right, level with each other.
	//
	// A section's tabs and its controls sit on one line -- the head is 34 high
	// with the refresh beside the tabs -- and wrap under one another
	// only when the row runs out (`flex-wrap: wrap`, whose
	// wrapped line keeps a 4-point row gap). Drawing the controls on a second row of their own
	// at every width would leave a band of empty chrome between the tabs and the list and put
	// the three buttons 26 points below the chips they belong to: at the 168 minimum the tabs
	// and the controls cannot both fit, and MyGo would stack 标 over 签. Wrapping answers that
	// case the same way.
	//
	// The counts come off the chips below SidebarFacetCountsMin: a chip with its count and
	// three 26-point controls need 253 points of column, and at the 212 default the choice is
	// between the count and the row. The list's own rows carry their counts, so the chip's is
	// the one that can go.
	ui.Column(c).FillWidth().GapY(SidebarGroupContentGap).Children(func() {
		ui.Row(c).FillWidth().MinHeight(SidebarTabRowHeight).
			AlignItems(ui.Center).GapX(SidebarHeaderGap).GapY(SidebarHeaderWrapGap).
			Wrap().Justify(ui.End).Children(func() {
			// The segmented track: 2 points around the tabs, a 7-point radius and a tray
			// ground, with the open tab's thumb standing off it.
			ui.Row(c).GapX(2).Padding(2).Radius(SidebarSegRadius).
				Background(rail.Muted).Children(func() {
				s.sidebarFacetTab(c, theme, rail, sidebarTabTags)
				s.sidebarFacetTab(c, theme, rail, sidebarTabAuthors)
			})

			// The spacer is what holds the controls at the right while the row fits, and what
			// the line breaks at when it does not: the two ends of the row are one unit each.
			// Grow rather than FillWidth: a percentage width is a flex *basis*, so the spacer
			// alone would fill the line and push the controls onto a second one at every width.
			ui.Spacer(c).Grow(1)

			ui.Row(c).GapX(SidebarHeaderGap).Children(func() {
				// The search, shared by both tabs and living here rather than above the list.
				// It used to be a permanent 38-point field at the top of each body, which is the row
				// the reader most wants back: the field is one click away and the list starts at the
				// top of the column until then.
				s.sidebarSearchToggle(c, theme, rail)

				// The tag section's own two controls, drawn on the tab they act on rather than in a
				// header of their own: a literal "+" that opens the inline draft row, and the table
				// glyph that opens the tag library.
				if s.sidebarTab() == sidebarTabTags {
					IconButtonFlat(c, s.t().T(i18n.CreateTagSymbol), theme, rail, SidebarHeadControl, func() {
						s.BeginCreateTag()
					})
					IconOnlySized(c, "table", s.t().T(i18n.TagManageLabel), theme, rail, SidebarHeadControl, func() {
						s.ToggleTagMode(context.Background())
					})
				}
			})
		})
	})
}

// sidebarTabCountsFit reports whether the facet chips have room for their counts.
//
// The row is measured against the column rather than the window: it is the sidebar's own
// drag that decides, and the constants are the ones the row is drawn with -- two chips with
// a 3-digit count, the gap, and three 26-point controls (SidebarFacetCountsMin is that sum
// plus the column's two insets).
func (s *Shell) sidebarTabCountsFit() bool {
	return s.state.LeftSidebarWidth >= SidebarFacetCountsMin
}

// sidebarFacetTab draws one chip of the segmented control.
//
// The count is the number of rows the list holds right now, so a search in the field below moves
// it -- it answers "how much is behind this tab", not "how many exist". A library with more
// authors than the service sends says so with a "+" rather than reporting the cap as a total.
func (s *Shell) sidebarFacetTab(c *ui.Context, theme *ui.Theme, rail RailTokens, tab string) {
	active := s.sidebarTab() == tab

	key := i18n.SidebarTags
	count := len(s.visibleSidebarTags())
	capped := false
	if tab == sidebarTabAuthors {
		key = i18n.AuthorsTitle
		count = len(s.visibleAuthors())
		capped = len(s.state.Data.Authors) >= workspace.SidebarLimit
	}

	chip := ui.Row(c).MinHeight(SidebarTabHeight).Padding(0, SidebarTabPadX).
		Radius(SidebarTabRadius).GapX(SidebarTabGap).AlignItems(ui.Center).Focusable()
	// The thumb crosses the track rather than switching sides: it fades
	// over .3s and the segmented thumb is a background transition like every
	// other. Both colours are animated -- the thumb's ground and the label's ink -- so the
	// open tab arrives instead of appearing.
	thumb := ui.Transparent
	switch {
	case active:
		thumb = segThumbFor(theme.Dark)
	case chip.Hovered():
		thumb = theme.SurfaceHover
	}
	chip = chip.Background(animateColor(c, chip, "facet-tab-ground", thumb, MotionBase))
	if active {
		// The edge cannot animate; it follows the thumb, which has already faded in.
		chip = segEdge(chip, theme.Dark)
	}

	ink := rail.TextTertiary
	weight := 400
	if active {
		ink = theme.Text
		weight = 500
	}
	ink = animateColor(c, chip, "facet-tab-ink", ink, MotionFast)

	// SingleLine on both runs: a squeezed tab must give up letters, never stack them. Two
	// characters on two lines make the control twice its height and read as a broken column.
	chip.OnClick(func() { s.selectSidebarTab(tab) }).Children(func() {
		ui.Text(c, s.t().T(key)).FontSize(FontSmall).FontWeight(weight).TextColor(ink).SingleLine()
		// The count is what makes the chip wide enough to push its three controls onto a
		// second line at the default width; the tab's own list carries the numbers anyway.
		if count > 0 && s.sidebarTabCountsFit() {
			text := domain.FormatCompactCount(count, s.translator.Locale())
			if capped {
				text += "+"
			}
			ui.Text(c, text).FontSize(FontTiny).TextColor(rail.TextTertiary).SingleLine()
		}
		// A filter in the list the reader is not looking at. The active tab shows its
		// selected row instead, so the dot only ever means "over there".
		if !active && s.facetTabFiltered(tab) {
			ui.Box(c).Width(SidebarTabDot).Height(SidebarTabDot).
				Radius(RadiusPill).Background(theme.Accent)
		}
	})
}

// facetTabFiltered reports whether that facet is narrowing the results.
func (s *Shell) facetTabFiltered(tab string) bool {
	if tab == sidebarTabAuthors {
		return len(s.state.ActiveAuthors) > 0
	}
	return len(s.state.ActiveTagIDs) > 0
}

// sidebarSearchToggle is the magnifier that shows and hides the sidebar's search field.
//
// One control for both tabs: a permanent field per section would take two rows of a column
// this build just freed up. The fill says whether the field is showing.
func (s *Shell) sidebarSearchToggle(c *ui.Context, theme *ui.Theme, rail RailTokens) {
	control := ui.Box(c).Width(SidebarHeadControl).Height(SidebarHeadControl).
		Radius(6).Center().Label(s.t().T(i18n.SearchLabel)).
		Tooltip(s.t().T(i18n.SearchLabel)).Focusable().
		OnClick(func() { s.toggleSidebarSearch() })
	ground := ui.Transparent
	switch {
	case s.state.SidebarSearchOpen:
		ground = theme.Accent.Alpha(0.10)
	case control.Hovered():
		ground = theme.SurfaceHover
	}
	control = control.Background(animateColor(c, control, "search-toggle-bg", ground, MotionFast))
	ink := rail.TextTertiary
	if s.state.SidebarSearchOpen {
		ink = theme.Accent
	}
	ink = animateColor(c, control, "search-toggle-ink", ink, MotionFast)
	control.Draw(func(p *ui.Painter, r ui.Rect) {
		drawIcon(p, "search", r.X+(r.W-SidebarSearchGlyph)/2, r.Y+(r.H-SidebarSearchGlyph)/2,
			SidebarSearchGlyph, ink)
	})
}

// toggleSidebarSearch shows or hides the field for the tab in effect.
//
// Closing it clears that tab's query. A field that is not on screen must not be filtering: the
// reader would be looking at a narrowed list with nothing saying why, and the row count on the
// tab would be the only clue.
func (s *Shell) clearSidebarSearch() {
	if s.sidebarTab() == sidebarTabAuthors {
		s.state.AuthorQuery = ""
	} else {
		s.state.SidebarTagQuery = ""
	}
	s.state.SidebarSearchOpen = false
	s.invalidate()
}

func (s *Shell) toggleSidebarSearch() {
	if s.state.SidebarSearchOpen {
		s.state.SidebarSearchOpen = false
		if s.sidebarTab() == sidebarTabAuthors {
			s.state.AuthorQuery = ""
		} else {
			s.state.SidebarTagQuery = ""
		}
	} else {
		s.state.SidebarSearchOpen = true
	}
	if s.window != nil {
		s.window.Invalidate()
	}
}

// sidebarSearchVisible reports whether the sidebar draws the field.
//
// A query that is not empty keeps it on screen even after the toggle has been closed elsewhere,
// so the two can never disagree: the field is drawn exactly when the tab's query can be seen.
func (s *Shell) sidebarSearchVisible() bool {
	if s.state.SidebarSearchOpen {
		return true
	}
	if s.sidebarTab() == sidebarTabAuthors {
		return s.state.AuthorQuery != ""
	}
	return s.state.SidebarTagQuery != ""
}

// sidebarSearchField is the sidebar's search box on either tab: one magnifier, one grounded
// field, the same height and the same inset, so the two lists start at the same place.
//
// AutoFocus, rather than Focus: the field appears when the toggle is clicked, and it should take
// the keyboard once, on that frame -- Focus would take it back on every frame after it and the
// reader could never type anywhere else while the field was open.
func (s *Shell) sidebarSearchField(c *ui.Context, theme *ui.Theme, rail RailTokens, query *string, placeholder string) {
	ui.Row(c).FillWidth().MinHeight(SidebarSearchHeight).Radius(SidebarSearchRadius).
		Border(1, rail.SubtleLine).Background(theme.Surface).
		Padding(0, SidebarSearchPadX).GapX(SpaceBase).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Width(SidebarSearchGlyph).Height(SidebarSearchGlyph).
			Draw(func(p *ui.Painter, r ui.Rect) {
				drawIcon(p, "search", r.X, r.Y, min32(r.W, r.H), rail.TextTertiary)
			})
		ui.TextInput(c, query).BorderWidth(0).Background(ui.Transparent).
			Placeholder(placeholder).FontSize(FontSmall).Grow(1).AutoFocus().
			// Escape clears the query and closes the field, the same answer the main
			// search gives: a field the reader cannot get out of keeps filtering the
			// list behind them.
			OnShortcut(0, ui.KeyEscape, func() { s.clearSidebarSearch() })
	})
}

// sidebarListScroll is the sidebar's own scrolling region.
//
// Both tabs draw their list through it. It takes the rest of the column and scrolls inside
// it, and it draws no thumb: the sidebar has none, and the overlay MyGo draws on hover sits on top of the counts rather
// than beside the rows, because the list is as wide as the column. The bar was also the only
// thing that moved when the pointer entered the sidebar, which read as the list shifting.
func sidebarListScroll(c *ui.Context) ui.Element {
	return ui.Scroll(c).FillWidth().Grow(1).
		ScrollbarInsets(0, -SidebarScrollbarHidden, 0, 0)
}

// navRowHandle is the keyboard handle of one sidebar row, made once and kept.
func (s *Shell) navRowHandle(id string) *ui.Handle {
	if s.navRows == nil {
		s.navRows = make(map[string]*ui.Handle)
	}
	handle := s.navRows[id]
	if handle == nil {
		handle = &ui.Handle{}
		s.navRows[id] = handle
	}
	return handle
}

// sidebarRowKeys makes Up and Down walk one sidebar list's rows.
//
// Each row is a focusable clickable, so Tab already reaches it and Enter or Space presses
// the one under the keyboard. What a reader could not do is move down the list directly:
// Tab made them pass every control between two rows, and the pointer's next row had no key.
// The list is the ancestor of every row, so MyGo hands it an arrow the row itself did not
// take, and the row it moves to is the one the focus names.
func (s *Shell) sidebarRowKeys(c *ui.Context, list ui.Element, ids []string) ui.Element {
	move := func(step int) {
		if len(ids) == 0 {
			return
		}
		at := -1
		for i, id := range ids {
			if s.navRowHandle(id).Focused(c) {
				at = i
				break
			}
		}
		next := 0
		switch {
		case at >= 0:
			next = at + step
		case step < 0:
			// Nothing is focused yet: Down starts at the top, Up at the bottom.
			next = len(ids) - 1
		}
		if next < 0 || next >= len(ids) {
			// A list is not a ring: the ends stay put.
			return
		}
		s.navRowHandle(ids[next]).Focus(c)
	}
	return list.
		OnShortcut(0, ui.KeyDown, func() { move(1) }).
		OnShortcut(0, ui.KeyUp, func() { move(-1) })
}

// sidebarTagsBody is the tags tab: its search field and the tag rows under it.
//
// This tab does not open with two navigation rows (全部书签 and 待确认标签): it is the tags and
// nothing else, and both rows repeat something already on screen -- the results header names
// 全部书签 as the scope and counts it, while 待确认 is a filter of the tag library, where tag
// work belongs. The capability behind the pending row is still there: the filter popover has
// 只有待确认建议 (filterfacets.go) and the library has its own 待确认 chip.
func (s *Shell) sidebarTagsBody(c *ui.Context, theme *ui.Theme, rail RailTokens) {
	ui.Column(c).FillWidth().Grow(1).GapY(SidebarGroupContentGap).Children(func() {
		if s.sidebarSearchVisible() {
			s.sidebarSearchField(c, theme, rail, &s.state.SidebarTagQuery,
				s.t().T(i18n.TagSearchPlaceholder))
		}

		// The list owns the rest of the column and scrolls inside it, so the tab row and the
		// field stay where they are however many tags the library holds.
		sidebarListScroll(c).Children(func() {
			list := ui.Column(c).FillWidth().GapY(SidebarRowsGap)
			var selected ui.Element
			// The rows Up and Down walk, in the order they are drawn.
			var rowIDs []string
			list.Children(func() {
				if s.state.CreatingTag {
					// The draft is the whole row: the 14-point hash and the field 8 apart, and
					// nothing after them. 添加/取消 buttons on this row would squeeze the field
					// in a 300-point sidebar until its placeholder read 输入新标…; Enter commits
					// and Escape cancels.
					//
					// The row's ground is surface-bg and its border is the subtle line; while
					// the field has the cursor the ring is an accent ring at 28%. MyGo has no
					// :focus-within, so the ring follows the field's own committed focus, which
					// is a frame behind and invisible.
					draft := ui.Row(c).FillWidth().Padding(NavRowPadY, NavRowPadX).
						Radius(NavRowRadius).GapX(NavRowMainGap).MinHeight(NavRowMinHeight).
						Background(theme.Surface).Border(1, rail.SubtleLine)
					if s.tagDraftField.Focused(c) {
						draft = draft.Border(1, theme.Accent.Alpha(0.28))
					}
					draft.Children(func() {
						ui.Box(c).Width(NavRowIcon).Height(NavRowIcon).
							Draw(func(p *ui.Painter, r ui.Rect) {
								drawIcon(p, "hash", r.X, r.Y, min32(r.W, r.H), rail.TextTertiary)
							})
						// The row's input is borderless, transparent and 14px, which is
						// TextInputBase: the styled TextInput drew a second box inside the row.
						field := ui.TextInputBase(c, &s.state.TagDraft).
							Placeholder(s.t().T(i18n.CreateTagPrompt)).
							Label(s.t().T(i18n.CreateTagLabel)).
							FontSize(NavRowFont).Grow(1).Bind(&s.tagDraftField)
						field.OnSubmit(func() { s.CreateTagFromDraft(context.Background()) })
						field.OnShortcut(0, ui.KeyEscape, func() { s.CancelCreateTag() })
					})
				}
				// There is no separate empty-tags branch: the sidebar draws the search field
				// and the tree, and an empty tree is just empty. The message is the same one
				// the tags pane uses when the library has no tags.
				//
				// A search that excludes every tag is a different empty list that would
				// otherwise draw nothing at all -- the same dead end the authors tab answers
				// with its own empty message.
				visible := s.visibleSidebarTags()
				if len(visible) == 0 {
					message := i18n.SidebarNoTagsCreated
					if len(s.state.Data.Tags) > 0 {
						message = i18n.SidebarNoTagsMatch
					}
					ui.Text(c, s.t().T(message)).
						FontSize(FontSmall).TextColor(theme.TextMuted).PaddingY(3)
				}
				for _, tag := range visible {
					id := prefixTag + tag.ID
					row, active := s.tagRow(c, theme, rail, id, tag.Name, s.tagCount(tag))
					rowIDs = append(rowIDs, id)
					// Several tags can be active at once, but there is one shape: it marks
					// the first.
					if active && !selected.Valid() {
						selected = row
					}
				}
			})
			navHighlight(c, s.sidebarRowKeys(c, list, rowIDs), selected, theme)
		})
	})
}

// sidebarAuthorsBody is the authors tab: its search field and every author the service sent.
//
// No cap and no "show more" here. The list scrolls in a column of its own now, so the cap only
// hid people behind an extra click; the one thing it guaranteed -- that a chosen author stays
// reachable -- follows from drawing the whole list in its natural order.
func (s *Shell) sidebarAuthorsBody(c *ui.Context, theme *ui.Theme, rail RailTokens) {
	ui.Column(c).FillWidth().Grow(1).GapY(SidebarGroupContentGap).Children(func() {
		if s.sidebarSearchVisible() {
			s.sidebarSearchField(c, theme, rail, &s.state.AuthorQuery,
				s.t().T(i18n.SearchAuthorsPlaceholder))
		}

		sidebarListScroll(c).Children(func() {
			list := ui.Column(c).FillWidth().GapY(SidebarRowsGap)
			var selected ui.Element
			var rowIDs []string
			list.Children(func() {
				authors := s.visibleAuthors()
				// An empty column is a dead end otherwise: nothing says whether the library
				// has no authors or the search excluded them all.
				if len(authors) == 0 {
					message := i18n.SidebarNoAuthors
					if len(s.state.Data.Authors) > 0 {
						message = i18n.SidebarNoAuthorsMatch
					}
					ui.Text(c, s.t().T(message)).
						FontSize(FontSmall).TextColor(theme.TextMuted).PaddingY(3)
				}
				for _, author := range authors {
					row := s.authorRow(c, theme, rail, author)
					rowIDs = append(rowIDs, "author:"+author.AuthorHandle)
					// One shape, as for the tags: several authors may narrow the results,
					// and the shape marks the first.
					if s.authorActive(author.AuthorHandle) && !selected.Valid() {
						selected = row
					}
				}
			})
			navHighlight(c, s.sidebarRowKeys(c, list, rowIDs), selected, theme)
		})
	})
}

// authorRow is one author in the sidebar: avatar, name over handle, the count under the current
// conditions, and a tick when chosen.
//
// The row's order is the service's and never the selection's. This used to move the chosen
// author to the top, which made the list jump under the pointer at the moment of the click --
// the row a reader had just picked was a row and a half up the column by the next frame.
func (s *Shell) authorRow(c *ui.Context, theme *ui.Theme, rail RailTokens, author domain.AuthorSummary) ui.Element {
	active := s.authorActive(author.AuthorHandle)
	label := author.AuthorHandle
	if author.AuthorName != "" && author.AuthorName != author.AuthorHandle {
		label = author.AuthorName
	}

	// Two lines need more than a sub-row's 26: the avatar and the name over the handle are
	// 30 high, while the type is the sub-row's own (12.5 name, 11 handle).
	row := ui.Row(c).FillWidth().Padding(0, SubRowPadX).Radius(NavRowRadius).
		GapX(SpaceBase).MinHeight(NavRowMinHeight).Border(1, ui.Transparent).Focusable().
		Bind(s.navRowHandle("author:" + author.AuthorHandle))
	row.Key("author:" + author.AuthorHandle).OnClick(func() {
		s.toggleAuthorFilter(context.Background(), author.AuthorHandle)
	}).ContextMenu(func(m *ui.Menu) {
		s.authorContextMenu(c, m, author)
	})
	// The row paints no ground of its own -- the list's selection shape is the only shape
	// (navhighlight.go) -- so only its ink answers the pointer.
	ink := subRowInk(c, row, theme, active)
	row.Children(func() {
		// The avatar: 24 square, the picture or a fallback mark. It doubles as the row's
		// icon, which is why these rows are taller than the plain ones.
		//
		// One element. This used to make the disc and then draw the picture beside it, so an
		// author with a face carried an empty 24-point box as well: their name sat 32 points
		// right of an author without one, and a picture still loading read as an author who
		// had none.
		if picture, ready := s.mediaPicture(author.AuthorAvatarURL); ready {
			ui.Image(c, picture).Width(AuthorAvatarSize).Height(AuthorAvatarSize).
				Radius(RadiusPill).Fit(ui.Cover).Border(1, rail.SubtleLine)
		} else {
			disc := ui.Box(c).Width(AuthorAvatarSize).Height(AuthorAvatarSize).Radius(RadiusPill).
				Background(rail.CardAvatarBg).Border(1, rail.SubtleLine)
			// The "@" is for an author the data has no picture for, and only for them: while
			// a picture is on the wire there is nothing to draw, so the disc stays empty.
			if author.AuthorAvatarURL == "" {
				disc = disc.Center().Children(func() {
					ui.Text(c, s.t().T(i18n.AuthorAvatarFallback)).FontSize(FontSmall).
						FontWeight(700).TextColor(rail.TextTertiary)
				})
			}
		}

		// Two lines: the name over the handle. The name carries the row grammar's 500 at
		// 12.5, the handle is the 11px sub-line; choosing an author puts the list's shape
		// under the row and takes the name to the strong ink -- no accent in the list.
		// The same rule as a tag's name: the label is nowrap with an ellipsis, and the row
		// is what gives way to the count.
		ui.Column(c).Grow(1).GapY(1).MinWidth(0).Children(func() {
			ui.Text(c, label).FontSize(SubRowFont).FontWeight(500).
				TextColor(ink).MaxLines(1).Shrink(1).MinWidth(0)
			ui.Text(c, "@"+author.AuthorHandle).FontSize(FontTiny).
				TextColor(rail.TextTertiary).MaxLines(1).Shrink(1).MinWidth(0)
		})

		// The count under the conditions in effect, not the author's lifetime total: the
		// number beside a row is what clicking it shows.
		if count := s.authorCount(author); count > 0 {
			ui.Text(c, domain.FormatCompactCount(count, s.translator.Locale())).
				FontSize(FontTiny).TextColor(theme.TextMuted)
		}
	})
	return row
}

// subRowInk is a sub-row's own ink: the quiet control ink at rest, the strong ink under the pointer or
// on the chosen row, and 500 on the chosen row.
//
// A sub-row paints no ground in any state. The selection is the list's only shape -- "a row
// hovered or pressed has no ground of its own, which would show where the finger is before
// the selection slides there" -- and the shape itself is the list's, not
// the row's (navhighlight.go). Its colour eases over the sub-row's own .2s so the pointer
// lights the row instead of switching it.
func subRowInk(c *ui.Context, row ui.Element, theme *ui.Theme, active bool) ui.Color {
	target := controlTextColour(theme.Dark)
	if active || row.Hovered() {
		target = theme.Text
	}
	return animateColor(c, row, "sub-row-ink", target, MotionInk)
}
