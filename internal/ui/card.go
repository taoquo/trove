package ui

import (
	"context"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/i18n"
	"github.com/taoquo/trove/internal/usecase/workspace"
)

func (s *Shell) bookmarkCard(c *ui.Context, theme *ui.Theme, rail RailTokens, index int, halfWidth bool) {
	row := s.state.Data.Bookmarks[index]
	// Two different selections, kept apart:
	//
	//	selected : the bookmark the detail is about. Clicking a card sets it, and the card's
	//	           outline follows it.
	//	bulk     : the checkbox in the card's corner, which stops the click from reaching the
	//	           card and toggles the batch set instead.
	//
	// They must stay separate: one field for both makes the whole card toggle the batch
	// selection, so clicking a post never opens its detail, and the outline follows the
	// checkboxes rather than the post the inspector is showing.
	selected := s.state.Selected == row.TweetID
	bulkSelected := s.state.SelectedIDs[row.TweetID]
	// Whether there is a picture at all, not whether the media slice is non-nil. A stored
	// row decodes its media column as an empty slice, so `row.Media != nil` was true for
	// every bookmark and every card was sized as though it carried one -- Margaret's
	// three-line post was clamped to the two-line media height and ended in an ellipsis.
	hasMedia := domain.MediaThumbnailURL(row.Media) != ""
	// A pending suggestion is a tag chip too, and it counts as one when the list card's
	// density is chosen: `hasTags` covers attached tags and pending suggestions. Counting
	// only attached tags would reserve three lines instead of two, which pushes the
	// suggestion chip 25 points below the reference's and the card 35 points tall.
	pending := s.pendingSuggestionsFor(row.TweetID)
	hasTagChips := len(row.Tags) > 0 || len(pending) > 0

	// FillWidth, never Grow. Grow was how the old two-per-row GridView split a row's width
	// between the pair; inside the masonry's column it grows the card's *height* instead,
	// and four cards sharing a column's height were compressed to about 340 points each,
	// with the copy cut mid-line.
	card := ui.Column(c).FillWidth()
	// The card: a 1px border of --theme-elevated-border, a 16px radius, and Clip.
	//
	// Clip is what makes the card's `overflow: hidden` real, and it is why the body padding
	// sits on the body rather than on this element: with padding here, the media could never
	// run to the card's edges.
	//
	// The 240 floor is the grid's; the list picks one of four template heights so a one-line post
	// and a long one do not sit at the same size.
	card = card.Radius(RadiusResultCard).
		Background(theme.Surface).Border(1, rail.CardBorder).Clip()
	if halfWidth {
		card = card.MinHeight(CardMinHeight)
	} else {
		lines := domain.CardLineCount(row.Text, hasMedia, hasTagChips)
		card = card.MinHeight(domain.ListCardMinHeight(hasMedia, hasTagChips, lines))
	}
	if selected {
		// magpie keeps the accent for what acts: a chosen card is the same hairline and a
		// warmer ground, not an accent ring (app.css:366-375).
		card = card.Border(1, rail.CardBorder).Background(rail.SelectedBg)
	}
	// The list layout is denser, and the stylesheet gives it its own values rather than a
	// scale factor.
	scale := cardTypeScale(halfWidth)

	// Clicking the card opens its detail: the modal is the card's onClick, not a selection
	// state. MyGo hands a click to the innermost clickable node only (input.go:257-290), so the
	// checkbox below taking its own OnClick stops the click from also reaching the card.
	item := card.Key(row.TweetID).Focusable()
	if selected && s.state.RevealSelected {
		item = item.ScrollIntoView()
	}
	// Handing the keyboard back after the detail closes. The flag is one frame's worth,
	// for the same reason RevealSelected is: focus is a request per element, and leaving it
	// set would pull the keyboard back to this card on every later rebuild.
	if selected && s.state.FocusSelected {
		item = item.Focus()
		if s.window != nil {
			window := s.window
			time.AfterFunc(120*time.Millisecond, func() {
				window.Update(func() {
					s.state.FocusSelected = false
				})
			})
		}
	}
	item.OnClick(func() {
		s.showDetail(row.TweetID)
	}).ContextMenu(func(m *ui.Menu) {
		s.bookmarkContextMenu(c, m, row)
	}).Children(func() {
		// The select toggle: absolutely placed 18 from the top-left, 20 square,
		// carrying a check once chosen. It is drawn first so everything after it paints over it,
		// and it is the batch-selection target: the box toggles what a bulk action runs on, and
		// the check follows that set, not the post the detail is showing.
		who := row.AuthorName
		if who == "" {
			who = row.AuthorHandle
		}
		s.cardSelectToggle(c, theme, rail, bulkSelected, hasMedia && halfWidth, row.TweetID, who)

		// Card media in the grid layout: a direct child of the card, before the body, so
		// it runs edge to edge, carries no radius of its own and takes a hairline along its
		// bottom. The list layout keeps its inline thumbnail instead -- the stylesheet moves that
		// one into the layout grid at 148 wide -- so this draws for the grid only.
		if mediaURL := domain.MediaThumbnailURL(row.Media); mediaURL != "" && halfWidth {
			if picture, ready := s.mediaPicture(mediaURL); ready {
				// Cover, not the default: card media is `object-fit: cover`. Without it ui.Image
				// keeps the picture's own aspect ratio and centres it, so a 3:2 photo in a card
				// twice as wide as it is tall is drawn 305 wide in a 437 card with margins either
				// side. The media frame is `aspect-video`: 16:9 of the card's width, which at a
				// 458-point card is 257 tall. A fixed 240 was short by 17 and, worse, wrong at
				// every other card width.
				ui.Image(c, picture).FillWidth().AspectRatio(CardMediaAspect).Fit(ui.Cover).
					BorderWidth(0, 0, 1, 0).BorderColor(rail.SubtleLine).
					Label(CardMediaFrameLabel)
			} else {
				// A block of the same shape, so a picture that lands a frame later does not move
				// everything under it.
				ui.Box(c).FillWidth().AspectRatio(CardMediaAspect).Background(rail.CardAvatarBg).
					Label(CardMediaFrameLabel)
			}
		}

		// The card body: 18 of padding and a 12 gap on the grid, its own padding in the list.
		// The list card re-declares the gap as 10 along with the other two.
		bodyGap := float32(CardBodyGap)
		if !halfWidth {
			bodyGap = CardBodyGapList
		}
		// The body is the card's growing child in *both* layouts. Only the grid was given Grow,
		// so a list card whose content came to less than its template floor (188 for a media
		// card, 154 for a long copy) kept the slack *below* the action row instead of letting
		// the main row absorb it -- measured on the fixture's second card, whose stat row sat
		// 29.6 points high.
		body := ui.Column(c).FillWidth().GapY(bodyGap).Grow(1)
		if halfWidth {
			body = body.Padding(CardBodyPadding)
		} else {
			body = body.Padding(CardListBodyPadV, CardListBodyPadH)
		}

		body.Children(func() {
			// The card's content column, as a closure because the list layout wraps it in the
			// stylesheet's two-column grid and the grid layout does not.
			mainRow := func() {
				// The main row is a grid of "auto minmax(0, 1fr)": the avatar keeps its
				// width and everything else takes what is left. `align-items: start` is why the
				// avatar sits level with the author name: without it the avatar centres itself
				// against the whole text column, and every card whose copy is taller than its
				// header showed the portrait halfway down (measured: 泊 31.8pt low).
				//
				// `padding-left: 30px` applies whenever the card has no top media preview --
				// which is every list card and every grid card without a picture. The select
				// toggle is at 18, so the indent is what keeps the avatar clear of it.
				main := ui.Row(c).FillWidth().GapX(CardMainGap).AlignItems(ui.Start)
				if !(halfWidth && hasMedia) {
					main = main.Padding(0, 0, 0, CardMainInset)
				}
				// The main row is `flex: 1 1 auto` in the layout column, so in the list
				// it is the main row -- not the copy wrap -- that takes the card's slack. That
				// keeps the tags directly under the copy and the actions on the card's bottom
				// edge; growing the wrap instead pushed the tags down with the slack and left
				// a hole between the copy and its tags.
				if !halfWidth {
					main = main.Grow(1)
				}
				main.Children(func() {
					// The avatar: the author's picture, or the initial when there is none. The
					// picture must win when it exists: drawing only the initial shows two letters
					// for an author whose picture has been stored all along. The initials are
					// centred (`display: flex; align-items: center; justify-content: center`);
					// a Box lays its child out from its own origin, so the letter would be drawn
					// at the disc's left edge without the Center() below.
					if picture, ready := s.mediaPicture(row.AuthorAvatarURL); ready {
						ui.Image(c, picture).Width(scale.avatarSize).Height(scale.avatarSize).
							Radius(RadiusPill).Fit(ui.Cover).Label(CardAvatarLabel)
					} else {
						ui.Box(c).Width(scale.avatarSize).Height(scale.avatarSize).
							Radius(RadiusPill).Background(rail.CardAvatarBg).Center().
							Label(CardAvatarLabel).Children(func() {
							ui.Text(c, cardInitial(row.AuthorName, row.AuthorHandle)).
								FontSize(scale.avatarFont).FontWeight(700).TextColor(theme.Text)
						})
					}

					// The content column is a flex column with a 14-point section gap -- 14, not
					// the 12 the body and layout use; the list's own value is 10. The three gaps
					// are named separately, and this column needs this one.
					sectionGap := float32(CardSectionGap)
					if !halfWidth {
						sectionGap = CardSectionGapList
					}
					ui.Column(c).GapY(sectionGap).Grow(1).Children(func() {
						// The card's header: a row of the author block and the timestamp, space-
						// between with a 12 gap. The block takes the slack and the timestamp keeps
						// its width, which is what puts the time at the top right of the card.
						ui.Row(c).FillWidth().GapX(CardHeaderGap(halfWidth)).Children(func() {
							ui.Column(c).GapY(3).Grow(1).Children(func() {
								ui.Text(c, row.AuthorName).FontSize(scale.authorFont).
									FontWeight(500).TextColor(theme.Text)
								ui.Text(c, "@"+row.AuthorHandle).FontSize(CardHandleFont).
									TextColor(theme.TextMuted)
							})
							// The timestamp is the card's meta line: 11.5px, regular, right
							// aligned, tertiary.
							ui.Text(c, domain.FormatSavedAt(
								domain.FormatTimestamp(row.SavedAt), string(s.t().Locale()))).
								FontSize(CardStatFont).
								TextColor(rail.TextTertiary).TextAlign(ui.End)
						})

						// The copy wrap: `flex: 1 1 auto; min-height: calc(1.6em * 2.6)`. It is
						// what takes the card's slack: it grows, so the tag row and the action row
						// sit on the card's bottom edge even when the copy is one line. The wrap
						// reserves 2.6 lines of height, which is also the height every card
						// without tags shows as air.
						//
						// The clamp differs by layout: the grid clamps at 5 lines, and only the
						// list chooses 1, 2 or 3 by density. Applying the list's rule to both would
						// cut a 116-character post to two lines and an ellipsis.
						copyLines := CardCopyClampGrid
						if !halfWidth {
							copyLines = domain.CardLineCount(row.Text, hasMedia, hasTagChips)
						}
						// The floor differs by layout, and in the list by the clamp the copy uses.
						// Applying the grid's 2.6 lines everywhere gives a one-line list card 58
						// points of reserved copy where 1.05 lines (21) is right, which is why
						// every short list card measured ~37 tall.
						wrapLines := float32(CardCopyWrapLines)
						if !halfWidth {
							switch copyLines {
							case domain.CardOneLine:
								wrapLines = CardCopyWrapListOne
							case domain.CardTwoLines:
								wrapLines = CardCopyWrapListTwo
							case domain.CardThreeLines:
								wrapLines = CardCopyWrapListThree
							default:
								wrapLines = CardCopyWrapListBase
							}
						}
						// In the grid the wrap takes the card's slack (flex: 1 1 auto); in the list
						// the main row already does, and a second grow here split the slack between
						// the two -- which is what left a hole between the copy and its tags in the
						// list capture.
						// The em is the inherited 16, not the copy's own size: the wrap declares
						// no font-size, so `1.55em` resolves against the page, while the copy inside
						// it sets 13. See CardCopyWrapEm.
						wrap := ui.Column(c).MinHeight(
							CardCopyWrapEm * scale.copyLineHeight * wrapLines,
						)
						if halfWidth {
							wrap = wrap.Grow(1)
						}
						wrap.Children(func() {
							ui.Text(c, row.Text).FontSize(scale.copyFont).TextColor(theme.Text).
								LineHeight(scale.copyLineHeight).
								MaxLines(copyLines)
						})

						if hasTagChips {
							// A list card has one line for tags, so past two the rest collapse into a
							// count. Showing all twelve would push the body off the card.
							names := make([]string, 0, len(row.Tags))
							for _, tag := range row.Tags {
								names = append(names, tag.Name)
							}
							visible := domain.VisibleCardTagNames(names, s.state.ViewMode)

							// A tag on a card is magpie's `.mchip`: one height, one inset and
							// one size in both layouts (app.css:3539-3543), so the same tag no
							// longer changes shape when the view switches.
							ui.Row(c).FillWidth().GapX(8).Children(func() {
								for _, name := range visible {
									ui.Text(c, name).FontSize(CardTagFont).FontWeight(500).
										TextColor(rail.CardTagText).Background(rail.CardTagBg).
										Padding(0, CardTagPadX).Radius(RadiusPill).
										MinHeight(CardTagMinHeight)
								}
								// The pending suggestions, as dashed chips beside the tags a card already
								// has. They count toward the card's height like a tag chip, so leaving them
								// out would make every card with a pending suggestion 24 points short.
								for _, suggestion := range pending {
									s.cardSuggestionChip(c, theme, rail, suggestion,
										CardTagFont, CardTagMinHeight, CardTagPadX)
								}
							})
						}
					})
				})

			}

			// The list card's layout is a grid:
			//   grid-template-columns: minmax(0, 1fr) 148px; gap: 14px; align-items: stretch
			// with the side track carrying the picture.
			//
			// The second track is there on *every* list card, not only the ones with a picture:
			// a hidden grid item does not collapse its fixed 148px track. Drawing the copy
			// straight into the body for a card without media gives it 911 points of line where
			// 677 is right, so the third and fifth cards wrap in one line instead of two and
			// every chip below them sits 20 points low. The empty box below is that track.
			if !halfWidth {
				mediaURL := domain.MediaThumbnailURL(row.Media)
				// The layout is `flex: 1 1 auto`: it fills the body, which is what carries the
				// card's slack down to the action row.
				ui.Row(c).FillWidth().Grow(1).GapX(CardListSideGap).AlignItems(ui.Stretch).Children(func() {
					ui.Column(c).Grow(1).GapY(bodyGap).Children(func() {
						mainRow()
						// The copy wrap above grows instead of a spacer here: the wrap is
						// `flex: 1 1 auto` and the actions have `margin-top: auto`, so both the tags
						// and the actions ride the bottom of a short card.
						s.cardActions(c, theme, rail, row, halfWidth)
					})
					if mediaURL == "" {
						ui.Box(c).Width(CardListSideWidth)
						return
					}
					// The inline media: height 100%, min-height 108, a 12 radius and a hairline.
					ui.Column(c).Width(CardListSideWidth).FillHeight().
						MinHeight(CardListSideMinHeight).Children(func() {
						if picture, ready := s.mediaPicture(mediaURL); ready {
							ui.Image(c, picture).FillWidth().FillHeight().Fit(ui.Cover).
								Radius(CardListMediaRadius).Border(1, rail.SubtleLine)
						} else {
							// A block of the same shape, so a picture that lands a frame later does
							// not move anything beside it.
							ui.Box(c).FillWidth().FillHeight().Radius(CardListMediaRadius).
								Background(rail.CardAvatarBg)
						}
					})
				})
				return
			}

			mainRow()
			// The copy wrap above grows instead of a spacer here: the wrap is
			// `flex: 1 1 auto` and the actions have `margin-top: auto`, so both the tags
			// and the actions ride the bottom of a short card.
			s.cardActions(c, theme, rail, row, halfWidth)
		})
	})
}

// pendingSuggestionsFor is the suggestions waiting on one bookmark, in the order loaded.
func (s *Shell) pendingSuggestionsFor(bookmarkID string) []domain.TagSuggestion {
	rows := make([]domain.TagSuggestion, 0, 2)
	for _, pending := range s.state.PendingSuggestions {
		if pending.Suggestion.BookmarkID == bookmarkID {
			rows = append(rows, pending.Suggestion)
		}
	}
	return rows
}

// cardSuggestionChip draws one pending-suggestion tag chip.
//
// The chip is transparent with a dashed accent hairline, the tag name as the accept target
// and a 16-point reject button beside it. MyGo has no dashed border, so the hairline is
// solid at the same accent alpha; that is the one visual difference, and it is recorded as
// a deviation rather than hidden.
func (s *Shell) cardSuggestionChip(c *ui.Context, theme *ui.Theme, rail RailTokens, suggestion domain.TagSuggestion, tagFont, minHeight, padX float32) {
	// The chip borrows the shape of the tags beside it -- the card's 24/9 chip, the detail's
	// 28-point pill -- so a pending tag reads as the tag it will become. The suggestion's own
	// padding would put the label elsewhere; the neighbour's rule is the one the specificity
	// picks.
	ui.Row(c).GapX(4).Padding(0, padX).Radius(RadiusPill).
		MinHeight(minHeight+CardSuggestionBorder).
		Border(1, theme.Accent.Alpha(0.55)).
		Children(func() {
			ui.Text(c, suggestion.TagName).FontSize(tagFont).FontWeight(600).
				TextColor(theme.TextMuted).Padding(0, 4, 0, 10).
				Tooltip(s.t().T(i18n.SuggestionsAccept)).
				OnClick(func() {
					_ = s.AcceptSuggestion(context.Background(), suggestion.ID)
				})
			box := ui.Box(c).Width(CardSuggestionRejectSize).
				Height(CardSuggestionRejectSize).Radius(RadiusPill)
			box = box.Background(hoverColor(c, box, "reject-bg", ui.Transparent, rail.CardTagBg, box.Hovered()))
			box.Draw(func(p *ui.Painter, r ui.Rect) {
				size := float32(CardSuggestionRejectGlyph)
				drawIcon(p, "close", r.X+(r.W-size)/2, r.Y+(r.H-size)/2, size, rail.TextTertiary)
			}).Tooltip(s.t().T(i18n.SuggestionsReject)).
				OnClick(func() {
					_ = s.RejectSuggestion(context.Background(), suggestion.ID)
				})
		})
}

// cardSelectToggle draws the card's select toggle.
//
// It gets a background and a shadow when the card has a top media preview, because a
// hairline ring over a photograph is invisible. That is what hasMedia switches.
func (s *Shell) cardSelectToggle(c *ui.Context, theme *ui.Theme, rail RailTokens, selected, hasMedia bool, tweetID, who string) {
	box := ui.Box(c).Width(CardSelectSize).Height(CardSelectSize).
		Radius(CardSelectRadius).Absolute().
		Top(CardSelectInset).Left(CardSelectInset)

	switch {
	case selected:
		box = box.Border(CardSelectBorder, rail.SelectedBorder).Background(rail.SelectedBg)
	case hasMedia:
		// With top media, the toggle becomes a translucent panel, a white hairline ringing it
		// and a soft drop shadow, all three because the box now sits on a photograph rather
		// than on the card's own surface. An outline alone disappears against a light picture,
		// which is why the rule exists.
		//
		// Every colour is theme-derived rather than literal: the fill is the surface at 82%,
		// the ring is the background colour at its 24% (white on a light theme, because the
		// background *is* white), and the shadow is the text colour at 8%. Writing RGBAs here
		// would have added three entries to the deliberate-literal list in
		// TestEveryRailTokenComesFromAThemeRole, and a list that grows to fit is not a check.
		box = box.Border(1, theme.Background.Alpha(0.24)).
			Background(theme.Surface.Alpha(0.82)).
			Shadow(0, 2, 7, 0, theme.Text.Alpha(0.08))
	default:
		box = box.Border(CardSelectBorder, theme.TextMuted)
	}

	// The box takes its own click. MyGo routes a click to the innermost interactive node in the
	// hit chain and increments that node only (input.go:257-290, 385-388), so this click does not
	// also run the card's. Without it the card would be the only target and there would be no way
	// to tick a box for a bulk action at all.
	// Label as well as Tooltip: the box needs an accessible name, and MyGo's Label is what a
	// reader of the accessibility tree -- and ui.Tester's Find, which is how the test reaches
	// this 20-point box -- sees.
	// The tick arrives rather than appearing: magpie's `copy-pop` overshoots from .55
	// through 1.16 to 1 (app.css:2550), and a spring is the same gesture with the same
	// shape. The value lives on the box, which every card has whether it is ticked or not,
	// so the 0 it animates from exists before the reader ticks it; under reduce motion it
	// jumps to 1 and the tick is simply there.
	tick := float32(0)
	if selected {
		tick = 1
	}
	tick = box.AnimateTo("select-pop", tick, MotionPop)

	box.Label(s.t().Tf(i18n.CardSelectBookmarkFor, who)).
		Tooltip(s.t().T(i18n.CardSelectBookmark)).
		OnClick(func() { s.toggleSelection(tweetID) }).
		Draw(func(p *ui.Painter, r ui.Rect) {
			if !selected || tick <= 0.01 {
				return
			}
			size := float32(CardSelectCheck) * tick
			drawIcon(p, "check", r.X+(r.W-size)/2, r.Y+(r.H-size)/2, size, theme.Text)
		})
}

// cardActions draws the action row: what the card shows besides being selectable.
//
// The leading group is decorative -- not focusable, with no hit area -- because the card
// itself is the button; it shows a post's reach without pretending to be three more controls.
// The trailing one opens the post, and it is the only one that acts.
//
// The list layout swaps the leading group for the post's metrics (reply, retweet, like).
func (s *Shell) cardActions(c *ui.Context, theme *ui.Theme, rail RailTokens, row workspace.BookmarkRow, halfWidth bool) {
	actionSize := float32(CardActionSize)
	padTop := float32(CardActionsPadTop)
	if !halfWidth {
		actionSize, padTop = CardListActionSize, CardListActionsPadTop
	}
	ui.Row(c).FillWidth().GapX(CardActionsGap).
		Padding(padTop, 0, 0, 0).
		BorderWidth(1, 0, 0, 0).BorderColor(rail.SubtleLine).
		Children(func() {
			// The leading group takes the slack so the trailing button is pushed to the far
			// edge, the same effect as justify-content: space-between.
			startGap := float32(CardActionsStartGap)
			statIcon, statFont := float32(CardActionIcon), float32(CardStatFont)
			if !halfWidth {
				// The list's actions-start gap is 8 and its stat gap is 4 at 10px. The glyph is
				// a bare 14-point icon inside the span, not a 28-square action button: drawing it
				// in the action box would put every count 15 points right of the reference's and
				// the three counts 32 apart where 8 is right.
				startGap = CardListStatGap
				statIcon, statFont = CardListStatIcon, CardListStatFont
			}
			ui.Row(c).GapX(startGap).Grow(1).Children(func() {
				if halfWidth {
					for _, icon := range []string{"comment", "heart", "share"} {
						cardActionGlyph(c, rail, icon, actionSize)
					}
					return
				}
				// A nil Metrics means no engagement counts came back at all, which is different
				// from three zeroes: draw nothing rather than "0 0 0".
				if row.Metrics == nil {
					return
				}
				for _, m := range []struct {
					icon  string
					value int
				}{
					{"comment", derefOrZero(row.Metrics.Replies)},
					{"share", derefOrZero(row.Metrics.Retweets)},
					{"heart", derefOrZero(row.Metrics.Likes)},
				} {
					ui.Row(c).GapX(CardListStatInnerGap).Children(func() {
						ui.Box(c).Width(statIcon).Height(statIcon).
							Draw(func(p *ui.Painter, r ui.Rect) {
								drawIcon(p, m.icon, r.X, r.Y, min32(r.W, r.H), rail.TextTertiary)
							})
						ui.Text(c, domain.FormatCompactCount(m.value, string(s.t().Locale()))).
							FontSize(statFont).TextColor(rail.TextTertiary)
					})
				}
			})
			cardActionButton(c, theme, rail, "external", s.t().T(i18n.OpenOnX), actionSize, func() {
				s.OpenOnX(row)
			})
		})
}

// cardActionGlyph draws one square decorative icon (32 in the grid, 28 in the list): no hit area, no tooltip, nothing to press.
func cardActionGlyph(c *ui.Context, rail RailTokens, name string, size float32) {
	ui.Box(c).Width(size).Height(size).Radius(RadiusPill).
		Draw(func(p *ui.Painter, r ui.Rect) {
			size := float32(CardActionIcon)
			drawIcon(p, name, r.X+(r.W-size)/2, r.Y+(r.H-size)/2, size, rail.TextTertiary)
		})
}

// cardActionButton draws one 32-square icon that acts.
func cardActionButton(c *ui.Context, theme *ui.Theme, rail RailTokens, name, tooltip string, size float32, onClick func()) {
	// Focusable and labelled, like every icon-only control: the card's one real action
	// was pointer-only and had no accessible name.
	box := ui.Box(c).Width(size).Height(size).Focusable().Label(tooltip).Radius(RadiusPill)
	box = box.Background(hoverColor(c, box, "action-bg", ui.Transparent, theme.SurfaceHover, box.Hovered()))
	// An icon control, so `.icon:active`'s .9 (app.css:4053).
	box = pressScale(box, "press", PressScaleIcon, MotionPress)
	box.Draw(func(p *ui.Painter, r ui.Rect) {
		size := float32(CardActionIcon)
		drawIcon(p, name, r.X+(r.W-size)/2, r.Y+(r.H-size)/2, size, rail.TextTertiary)
	}).Tooltip(tooltip).OnClick(onClick)
}

// bookmarkGrid draws the results as a masonry grid.
//
// It is not a row-major grid of equal cells: it is multi-columns,
//
//	columns: 1, two at the large width
//	column-gap: 22px
//	cards { break-inside: avoid; margin-bottom: 22px }
//
// which fills the first column top to bottom and then the second, and lets every card keep
// its own height. ui.GridView gives the opposite: row-major, one cell height for every card,
// and a two-column split decided by a minimum width. That pulls cards side by side where they
// should stack, and pads short text cards out to the height of a media card.
//
// So this draws two columns of cards in a scroll, distributed in reading order. The split is
// by count, which is what balanced columns come to for cards of similar height; CSS really
// balances by height, and that is the part this approximates.
//
// The render limit matches the reference: at most 80 cards, then 80 more when the scroll
// nears the bottom. That is a bounded build without pretending to be virtualisation -- the
// reference does not virtualise either.
//
// The extra page follows the scroll with an anchor: one page per reach of the end. Growing
// the window on every frame that saw the old MaxY would grow repeatedly, because a build pass
// can run several times per frame. autoLoadY is the offset the last page was asked at, and
// the appended cards do not move that offset, so the next frame compares equal and does
// nothing; only scrolling further asks again.
func (s *Shell) bookmarkGrid(c *ui.Context, theme *ui.Theme, rail RailTokens) {
	s.autoLoadMore(context.Background())

	rows := s.state.Data.Bookmarks
	limit := s.state.RenderLimit
	if limit <= 0 || limit > len(rows) {
		limit = len(rows)
	}
	shown := rows[:limit]

	// Two columns is a viewport breakpoint, not a pane width: the switch is at 1024 points
	// of window, whatever the sidebar is doing.
	columns := 1
	if width, _ := c.Size(); width >= ResultsTwoColumnWidth {
		columns = 2
	}
	s.state.ResultsColumns = columns
	split := len(shown)
	if columns == 2 {
		split = (len(shown) + 1) / 2
	}

	ui.Scroll(c).FillWidth().Grow(1).TrackScroll(&s.resultsScroll).Children(func() {
		ui.Row(c).FillWidth().GapX(GridColumnGap).AlignItems(ui.Start).Children(func() {
			column := func(from, to int) {
				ui.Column(c).Grow(1).GapY(GridCardGap).Children(func() {
					for i := from; i < to; i++ {
						s.bookmarkCard(c, theme, rail, i, true)
					}
				})
			}
			column(0, split)
			// The second column is drawn even when it is empty. A single result used to fill
			// the pane: the split put it in the first column and the second was skipped, so
			// that column grew to the whole width and the one card the reader had filtered down
			// to was twice the size of every other card. `columns-2` keeps the column whether
			// or not an item has reached it.
			if columns == 2 {
				column(split, len(shown))
			}
		})

		// Nothing to press: the grid grows when the reader reaches its end, and the next
		// database page follows when the loaded rows run out. The line says why the page
		// stopped moving, which is the only thing the removed control used to say.
		if s.state.LoadingMore {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(0, 0, SpaceBase*2, 0).Children(func() {
				ui.Text(c, s.t().T(i18n.ResultsLoadingMore)).
					FontSize(FontSmall).TextColor(rail.TextTertiary)
			})
		}
	})
}

// bookmarkList draws the results as a virtualised list.
//
// ui.List's ListState virtualises the rows, so building a large library does not cost a row
// per bookmark on every frame. The grid draws the same data its own way, with separate
// scroll state.
//
// The choice is shared state with the grid rather than a second one: ListState.Selected is
// an index like GridState.Selected, and both write s.state.Selected, which is the bookmark
// id the inspector follows. Filtering stays in the query, not in either layout.
func (s *Shell) bookmarkList(c *ui.Context, theme *ui.Theme, rail RailTokens) {
	rows := s.state.Data.Bookmarks
	s.list.Key = func(i int) any { return rows[i].TweetID }
	s.list.Label = func(i int) string { return rows[i].Text }
	s.list.Selected = &s.state.ListSelected

	// The list spaces its cards 12 apart; ui.List reads the row gap from the element, so it has
	// to be set here or the cards butt together.
	list := ui.List(c, &s.list, len(rows), func(i int) {
		s.bookmarkCard(c, theme, rail, i, false)
	}).Fill().GapY(ResultsListGap).TrackScroll(&s.listScroll)

	s.autoLoadMore(context.Background())

	if list.Changed() && s.state.ListSelected >= 0 && s.state.ListSelected < len(rows) {
		s.state.Selected = rows[s.state.ListSelected].TweetID
	}
	if list.Submitted() && s.state.ListSelected >= 0 && s.state.ListSelected < len(rows) {
		s.state.Selected = rows[s.state.ListSelected].TweetID
		s.OpenSelectedDetail()
	}
}

// CardHeaderGap is the card header's gap: 12 on the card and 10 in the list layout.
func CardHeaderGap(halfWidth bool) float32 {
	if halfWidth {
		return CardHeaderGapGrid
	}
	return CardHeaderGapList
}

// derefOrZero turns a missing count into zero: a count the API did not return reads as
// zero in the card's stat row. The pointer is preserved in the domain because "missing" and "zero"
// are different facts, but the card is not where that difference is shown.
func derefOrZero(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}
