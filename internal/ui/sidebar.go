package ui

import (
	"context"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	engine "github.com/taoquo/trove/internal/autotag"
	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/i18n"
)

// selectSidebar changes the scope and reloads.
//
// The scope is navigation: all bookmarks, the inbox, unclassified, one list. A tag is a
// condition and stays one, so switching scope leaves ActiveTagIDs and ActiveAuthors
// alone -- the chip row keeps showing them, because dropping the tags on a scope switch
// would leave a filtered list looking empty with nothing on screen to explain it.
//
// The selection is not cleared here: Reload keeps it when the bookmark is still in the
// new scope's page and drops it when it is not, so switching scope from a filtered inbox
// to All does not close the panel under a reader whose bookmark is still listed.
func (s *Shell) selectSidebar(ctx context.Context, id string) {
	// Any navigation leaves the settings page: it is drawn instead of the results, so
	// without this a click on a scope reloaded a list nobody could see.
	s.leaveSettings()
	// A tag id reaching this function means "read that tag's bookmarks", which is the
	// scope All plus that tag as the only condition. The sidebar's own tag rows toggle a
	// condition and never call this; the branch keeps the function total for a deep link
	// or a restored view that names a tag.
	if strings.HasPrefix(id, prefixTag) {
		s.state.ActiveTagIDs = []string{strings.TrimPrefix(id, prefixTag)}
		id = selectionAll
	}
	s.state.Selection = id
	s.markPageChange()
	s.ReloadAsync(ctx)
}

// sidebar draws the left column: the workspace hero, one facet list, and the config footer.
//
// The middle is one tab at a time. A single scroll holding both sections, each with its own
// search field and the authors behind a six-row cap and a "show more" control, would leave
// about three author rows on screen at a 760-point window and put the rest behind a second
// interaction. The two lists answer different questions and are each long enough to want the
// whole column, so they use a segmented control and take the space by turn.
func (s *Shell) sidebar(c *ui.Context, theme *ui.Theme, rail RailTokens) {
	// The sidebar is the page ground, and its edge is the split's own line at its right,
	// not a border here.
	//
	// The sidebar carries a half-point right edge: one line at the column's edge and no
	// track beside it.
	// ui.Split's one-point divider is that line, so the column carries no edge of its own.
	ui.Column(c).Width(s.state.LeftSidebarWidth).FillHeight().
		Background(rail.Sidebar).
		Children(func() {
			// The hero is a run's progress and a failed sync's explanation, nothing
			// standing: with neither there is no block and no rule under it, so the facets
			// start at the top of the column.
			if s.sidebarHeroVisible() {
				s.sidebarHero(c, theme, rail)
				ui.Divider(c)
			}

			// The facet area: the tabs stay put and the list scrolls under them, with its own
			// scrollbar at the column's edge. The search field belongs to the tab it searches, so
			// typing a tag query never leaves an author list filtered by something invisible.
			ui.Column(c).FillWidth().Grow(1).
				Padding(SidebarListsPadTop, SidebarListsPadX, SidebarListsPadBottom, SidebarListsPadX).
				GapY(SidebarGroupContentGap).Children(func() {
				s.sidebarFacetTabs(c, theme, rail)
				if s.sidebarTab() == sidebarTabAuthors {
					s.sidebarAuthorsBody(c, theme, rail)
					return
				}
				s.sidebarTagsBody(c, theme, rail)
			})

			ui.Divider(c)
			s.sidebarFooter(c, theme, rail)
		})
}

// sidebarHero is the block above the list, and only while something is happening: a running
// sync, a failed one, a resumable one, or a live auto-tag run.
//
// The sidebar has no standing status block: its state line sits in the foot beside the
// window's commands. The last-sync line and the sync button
// live there too, which leaves this block for work in progress, and takes its rule with it
// when there is none.
func (s *Shell) sidebarHero(c *ui.Context, theme *ui.Theme, rail RailTokens) {
	ui.Column(c).FillWidth().Padding(SidebarHeroPadY, SidebarInlinePadding).
		GapY(SidebarHeroGap).Children(func() {
		// A running sync and the one control that stops it. The standing last-sync line and
		// the start button are not here: they live in the footer beside the other window
		// commands, so this block appears only while there is work to report.
		if s.state.SyncRunning {
			// One line, not two. The progress sentence already begins "Syncing…", so
			// drawing the bare state above it put "Syncing" over "Syncing… 3 pages · 60
			// items" -- the same word twice, in a block that has room for one. A single
			// label serves both cases.
			line := s.t().T(i18n.StatusRunning)
			if s.state.SyncProgress != "" {
				line = s.state.SyncProgress
			}
			ui.Row(c).FillWidth().GapX(SpaceBase).AlignItems(ui.Center).Children(func() {
				ui.Text(c, line).FontSize(FontTiny).TextColor(theme.TextMuted).Grow(1)
				settingsAction(c, theme, rail, s.t().T(i18n.ActionCancel), false, false,
					func() { s.CancelSync() })
			})
		}
		if s.state.SyncResumePending && !s.state.SyncRunning {
			ui.Text(c, s.t().T(i18n.SyncMoreAvailable)).FontSize(FontTiny).TextColor(theme.TextMuted)
			// The sentence names the action, so it carries the control. The start lives in
			// the footer as an icon with a tooltip; a sentence that names an action needs a
			// labelled button beside it.
			settingsAction(c, theme, rail, s.t().T(i18n.ActionSync), false, false, func() {
				s.StartSync(context.Background())
			})
		}

		// What went wrong, inside the sync block, and the way to the fix under it when the
		// failure is a dead session -- the app has no sign-in screen, so the new cookies go in
		// the same place either way. A bordered box with 8/12 padding costs 62 points and
		// wraps a sentence that fits on one line without the padding.
		if line := s.syncProblemLine(); line != "" {
			ui.Text(c, line).FontSize(FontSmall).TextColor(theme.Accent).MaxLines(2)
			// The sentence says what to do; this is where to do it. A dead session swaps in
			// the reconnect pair below, but every other failure would leave the reader with
			// the words "try again" and no control: the sync command lives in the footer as an
			// icon, and nothing connected the two. This block keeps a labelled sync button
			// beside the failure.
			if !s.syncNeedsNewCookies() {
				settingsAction(c, theme, rail, s.t().T(i18n.ActionSync), false, false, func() {
					s.StartSync(context.Background())
				})
			}
			if s.syncNeedsNewCookies() {
				// The fix, where the failure is. "Sign in again" had no destination in this
				// app -- there is no login screen -- so the reconnection reads the browser the
				// reader is already signed in to, and Settings stays for the paste fallback.
				ui.Row(c).GapX(SpaceBase).Wrap().Children(func() {
					settingsAction(c, theme, rail, s.t().T(i18n.ActionReconnectX), false, false, func() {
						s.ReconnectXSession()
					})
					settingsAction(c, theme, rail, s.t().T(i18n.ActionOpenSyncSettings), false, false, func() {
						s.openSyncSettings()
					})
				})
			}
			if s.state.XBrowserReadNote != "" {
				ui.Text(c, s.state.XBrowserReadNote).FontSize(FontTiny).
					TextColor(theme.TextMuted).MaxLines(2)
			}
		}

		// A live auto-tag run. It only draws while a run is live: a finished run belongs in
		// settings, not beside the feed. This is the whole progress view -- phase,
		// processed/total, the bar and the elapsed/remaining foot.
		liveRun := false
		live := s.liveRunState()
		if live != nil {
			if view, ok := s.RunProgressViewFor(live, time.Now()); ok {
				s.runProgressBar(c, theme, view)
				liveRun = true
			}
		}

		// The auto-tagging control. Running it and stopping it belong together: removing the
		// results toolbar's row without moving this would have left a running pass with no
		// way out.
		if s.state.AutoTagRunning && !liveRun {
			// The bar draws the cancel control, so this pair is the fallback for a run that
			// is marked active but has no live state to describe.
			ui.Row(c).GapX(SpaceBase).Children(func() {
				ui.Spinner(c)
				ui.Text(c, s.t().T(i18n.RunAnalyzing)).
					FontSize(FontTiny).TextColor(theme.TextMuted)
			})
			label, disabled := s.t().T(i18n.ActionStopRun), false
			if live := s.liveRunState(); live != nil && live.CancelRequested {
				// Already stopping: the same rule as the page, so the two surfaces do
				// not offer a press with nothing left to do.
				label, disabled = s.t().T(i18n.RunStopping), true
			}
			settingsAction(c, theme, rail, label, disabled, false, func() {
				s.StopAutoTag()
			})
		} else if !s.state.AutoTagRunning && liveRun {
			// Only when a run is being described above. There is no standalone start button:
			// a run's start control lives on the auto-tagging page of tag management, so
			// this is not drawn on a sidebar that has nothing to say about a run.
			settingsAction(c, theme, rail, s.t().T(i18n.ActionRunNow), false, false, func() {
				s.StartAutoTag(context.Background())
			})
		}
	})
}

// sidebarHeroVisible reports whether there is anything for the hero to say.
//
// It is the same question the hero's own branches ask, kept in one place so the divider under
// it cannot be drawn without the block.
func (s *Shell) sidebarHeroVisible() bool {
	if s.state.SyncRunning || s.state.SyncResumePending || s.state.AutoTagRunning {
		return true
	}
	if s.syncProblemLine() != "" {
		return true
	}
	// A run whose heartbeat stopped is not something the hero can describe: the
	// progress view declines it, so claiming the block would draw a divider with
	// nothing under it.
	if live := s.liveRunState(); live != nil && engine.IsActive(live, time.Now(), engine.RunStaleAfter) {
		return true
	}
	return false
}

// sidebarFooter is the pinned foot: the workspace's state, then the window's commands.
//
// The foot is 8 top, 10 sides and 10 bottom, with a half-point subtle hairline above,
// a 1-point gap, and a
// 26-point row carrying a 7-point dot and 11.5px at its head and 26-point icon buttons
// under it.
//
// The last-sync line and the sync button sit here rather than leading the sidebar, beside the
// rest of the window's commands, so the block above the list is only ever work in progress.
// Export, restore and reset stay in Settings' Data safety section and the File menu: they are
// file commands, not navigation.
func (s *Shell) sidebarFooter(c *ui.Context, theme *ui.Theme, rail RailTokens) {
	ui.Column(c).FillWidth().GapY(FooterGap).
		Padding(FooterPadTop, FooterPadX, FooterPadBottom, FooterPadX).Children(func() {
		// The state row: what the last sync did, and the dot that says whether it is running.
		// It takes the badge's own colours, so idle, running and an error read the same here as
		// they do anywhere else the status is drawn.
		status := ui.Row(c).FillWidth().MinHeight(SideFooterStatusHeight).Padding(0, SideFooterStatusPadX).
			Radius(SubRowRadius).GapX(SideFooterStatusGap).AlignItems(ui.Center).
			Tooltip(s.t().T(s.workspaceStatusLabel()))
		tokens := StatusBadge(s.workspaceStatus(), s.state.Appearance, s.state.SystemDark, c.Theme().Accent)
		dot := float32(1)
		if s.state.SyncRunning {
			dot = pulse(c, status, "side-status")
		}
		status.Children(func() {
			ui.Box(c).Width(StatusDotSize).Height(StatusDotSize).
				Background(tokens.Text).Radius(RadiusPill).Opacity(dot)
			ui.Text(c, s.lastSyncLine()).FontSize(SideFooterStatusFont).
				TextColor(rail.TextTertiary).MaxLines(1).Grow(1)
		})

		ui.Row(c).GapX(2).Children(func() {
			// The locale chip: 26 high, min-width 26, an 8 radius and a 12/700 label. Its one
			// character is "中" in Chinese and "EN" in English.
			locale := ui.Box(c).Height(FooterIconSize).MinWidth(FooterIconSize).Radius(8).Center().
				Label(s.t().T(i18n.LanguageSwitch)).Focusable()
			if locale.Hovered() {
				locale = locale.Background(theme.SurfaceHover)
			}
			locale.OnClick(func() {
				// The locale chip switches between the two catalogs. An unset preference is
				// English on screen, so the chip's first press is the one that makes it Chinese.
				next := i18n.ZHCN
				if s.state.Locale == i18n.ZHCN {
					next = i18n.EN
				}
				s.SetLocale(next)
			}).Children(func() {
				ui.Text(c, s.localeChipLabel()).FontSize(FontSmall).FontWeight(700).
					TextColor(theme.TextMuted).Padding(0, FooterChipPadX)
			})
			IconOnlySized(c, "sun", s.t().T(i18n.ThemeLabel)+": "+s.appearanceLabel(), theme, rail, FooterIconSize, func() {
				s.SetAppearance(NextAppearance(s.state.Appearance, s.resolvedAppearance()))
			})
			// The sync command, beside the state it changes. Running, it is the way out: a
			// run's stop control sits with the run, and the footer is where this build keeps
			// the start.
			icon, label := "sync", s.t().T(i18n.ActionSync)
			if s.state.SyncRunning {
				icon, label = "close", s.t().T(i18n.ActionCancel)
			}
			IconOnlySized(c, icon, label, theme, rail, FooterIconSize, func() {
				if s.state.SyncRunning {
					s.CancelSync()
					return
				}
				s.StartSync(context.Background())
			})
			IconOnlySized(c, "settings", s.t().T(i18n.SettingsTitle), theme, rail, FooterIconSize, func() {
				s.OpenSettings()
			})
		})
	})
}

// workspaceStatus is the summary status the badge shows.

func (s *Shell) workspaceStatus() domain.SyncStatus {
	// The status itself, not the word for it.
	//
	// Comparing `LastSyncStatus` against the *translated* strings would make the badge fall back
	// to idle whenever the language changed after a sync: the stored word was English and the
	// translation it was compared against was Chinese. Storing the status and translating where
	// it is drawn makes the comparison a comparison of states.
	switch {
	case s.state.SyncRunning:
		return domain.SyncRunning
	case s.state.LastSyncStatus == domain.SyncSuccess,
		s.state.LastSyncStatus == domain.SyncPartialSuccess,
		s.state.LastSyncStatus == domain.SyncCancelled,
		s.state.LastSyncStatus == domain.SyncError:
		return s.state.LastSyncStatus
	default:
		return ""
	}
}

// workspaceStatusLabel is the word beside the badge's dot.

func (s *Shell) workspaceStatusLabel() i18n.Key {
	switch s.workspaceStatus() {
	case domain.SyncRunning:
		return i18n.StatusRunning
	case domain.SyncSuccess:
		return i18n.StatusSuccess
	case domain.SyncPartialSuccess:
		// partial_success has its own status label. Without this case a run that stored
		// most of the timeline and lost one page reported "空闲" -- idle -- on a workspace
		// that had just synced.
		return i18n.StatusPartial
	case domain.SyncError:
		return i18n.StatusError
	case domain.SyncCancelled:
		// The dot already takes the status, and syncStatusLabel has a word for it; without
		// this case a stopped run's tooltip said "Idle".
		return i18n.StatusCancelled
	default:
		return i18n.StatusIdle
	}
}

// lastSyncLine is what the hero says about the last sync.

func (s *Shell) lastSyncLine() string {
	if s.state.LastSyncedAt.IsZero() {
		return s.t().T(i18n.LastSyncNever)
	}
	// FormatSavedAt, not FormatTimestamp: the latter is the *storage* formatter and produces
	// "2026-10-17T10:52:03.889Z". The card's Published line uses the display formatter for the
	// same reason; this branch is easy to miss because a zero LastSyncedAt never reaches it.
	return domain.FormatSavedAt(domain.FormatTimestamp(s.state.LastSyncedAt), string(s.t().Locale()))
}

// localeChipLabel is the two-letter locale code the footer chip shows.

func (s *Shell) localeChipLabel() string {
	if s.state.Locale == i18n.ZHCN {
		return "中"
	}
	return "EN"
}

// resolvedAppearance is the theme actually in use, which "system" resolves to.
//
// The cycling button needs it because from "system" the next press is the opposite of
// what is on screen -- without the resolved value the button could look like it did
// nothing.
func (s *Shell) resolvedAppearance() Appearance {
	if s.state.Appearance == AppearanceSystem {
		if s.state.SystemDark {
			return AppearanceDark
		}
		return AppearanceLight
	}
	return s.state.Appearance
}

// appearanceLabel names the current preference for the button.

func (s *Shell) appearanceLabel() string {
	switch s.state.Appearance {
	case AppearanceLight:
		return s.t().T(i18n.AppearanceLight)
	case AppearanceDark:
		return s.t().T(i18n.AppearanceDark)
	default:
		return s.t().T(i18n.AppearanceSystem)
	}
}

// tagRow draws a tag with its own active test and its own click.
//
// A tag is active when it is in the set, not when it is the selection: with several tags
// narrowing the results, the sidebar has to show all of them as active, and the
// selection only names the first.

// ToggleFilterPopover shows or hides the filter popover.
func (s *Shell) ToggleFilterPopover() {
	s.state.FilterPopoverOpen = !s.state.FilterPopoverOpen
	if s.state.FilterPopoverOpen {
		// The popover grows out of the button it drops from; the clock starts
		// here, at the click, not at the frame that first draws it.
		s.state.FilterPopoverOpenedAt = time.Now()
	}
	if s.window != nil {
		s.window.Invalidate()
	}
}

// filterPopover draws the filter conditions.
//
// Every row here is also a row in filterFlags, which is what the chips, the badge and
// "clear all" read. The panel is a rendering of that table, not a second list of
// conditions to keep in step.
func (s *Shell) filterPopover(c *ui.Context, theme *ui.Theme, rail RailTokens, panel, anchor ui.Element) {
	// The panel: a card ground, an 11-point radius, one hairline and the palette's
	// popover shadow, at 262 width and 4 inset.
	// There is no backdrop blur -- the popover ground is 94% opaque
	// over one -- so the card colour is drawn opaque, as elsewhere in this shell.
	//
	// A FillWidth column drawn inline in the toolbar would push the result grid down the page;
	// the panel is over the content rather than a row of it.
	panel = panel.Width(FilterPopoverWidth).Radius(FilterPopoverRadius).
		Padding(FilterPopoverPad).Border(1, rail.CardBorder).Background(theme.Surface).
		Label(s.t().T(i18n.FilterConditions))
	panel = popoverShadow(panel, theme.Dark)

	// The popover grows out of the button it drops from: a fade over .14s, and a rise from 6
	// points above at .94 over .24s on the ease-out curve, around the button's middle at the
	// panel's top. Our panel is anchored
	// to the trigger's bottom-left, so that origin is half the trigger's width in from the
	// panel's left edge.
	opacity, rise, scale := s.filterPopoverEntrance(c)
	panel = panel.Opacity(opacity)
	if rise != 0 || scale != 1 {
		originX := float32(0.5)
		if box := anchor.Bounds(); box.W > 0 {
			originX = box.W / 2 / FilterPopoverWidth
		}
		panel = panel.TransformOrigin(originX, 0).
			Transform(ui.Scaling(scale, scale).Then(ui.Translation(0, -rise)))
	}
	panel.Children(func() {
		ui.Column(c).FillWidth().GapY(SpaceBase).Children(func() {
			ui.Text(c, s.t().T(i18n.FilterConditions)).FontSize(FontSmall).Bold().TextColor(theme.Text)
			ui.Text(c, s.t().T(i18n.FilterDescription)).FontSize(FontTiny).TextColor(theme.TextMuted)

			for _, flag := range s.filterFlags() {
				if !flag.inPanel {
					continue
				}
				// The box is bound to a per-frame copy and writes back through the
				// table, so the panel cannot drift from the chips: a condition added
				// there gains a row here by saying so, and no row can mean something
				// the chip does not.
				on := flag.on(s)
				label := s.t().T(flag.key)
				if flag.panelLabel != nil {
					label = flag.panelLabel(s)
				}
				ui.Checkbox(c, &on, label).OnChange(func() {
					flag.set(s, on)
					s.ReloadAsync(context.Background())
				})
			}
		})
	})
}

// visibleSidebarTags is the sidebar's tag rows after its search field narrows them.
//
// Only the tag rows: the tags tab draws no navigation rows above them, so what the
// field hides and shows is the whole list.
func (s *Shell) visibleSidebarTags() []domain.TagSummary {
	tags := s.state.Data.Tags
	query := strings.TrimSpace(s.state.SidebarTagQuery)
	if query == "" {
		return tags
	}

	// One matcher for both search boxes. The field's placeholder promises names or aliases --
	// and the tag library has always matched both. A name-only loop here would make the same
	// query find a tag in one place and nothing in the other, with no way to tell whether the
	// tag or the field was wrong.
	values := make([]domain.Tag, 0, len(tags))
	for _, tag := range tags {
		values = append(values, tag.Tag)
	}
	kept := make(map[string]bool, len(tags))
	for _, tag := range domain.FilterSidebarTags(values, query) {
		kept[tag.ID] = true
	}

	matching := make([]domain.TagSummary, 0, len(kept))
	for _, tag := range tags {
		if kept[tag.ID] {
			matching = append(matching, tag)
		}
	}
	return matching
}
