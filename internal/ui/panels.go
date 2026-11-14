package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/i18n"
	"github.com/taoquo/trove/internal/usecase/settings"
)

// OpenSettings shows the settings panel.
//
// It is exported because the menu bar opens it from outside this package.
//
// The panels are mutually exclusive: two modals stacked on one window leaves the
// user unsure which Escape closes.
func (s *Shell) OpenSettings() {
	// Read whether the Keychain holds the X cookies before the sheet draws, so the sync tab can
	// say why a sync cannot run instead of offering a button that is going to fail.
	s.RefreshXCookieStatus(context.Background())
	s.closePanels()
	s.syncPreferenceLabels()
	s.loadAutoTagSettings(context.Background())
	s.loadSyncPreferences(context.Background())
	s.state.SettingsOpen = true
	s.markPageChange()
	// The page opens on what it was last showing, and with no query: a filter left from a
	// previous visit would hide the section the menu command was aiming at.
	s.state.SettingsQuery = ""
	if s.window != nil {
		s.window.Invalidate()
	}
}

// MoveSelection moves the list selection by delta rows.
//
// The arrow keys are how a list is navigated without a mouse. It moves the cursor the detail
// opens on, not the batch selection: a bulk action runs on the boxes the user ticked, and the
// arrow keys have no opinion about those -- row navigation is the native shell's own
// affordance.
func (s *Shell) MoveSelection(delta int) {
	rows := s.state.Data.Bookmarks
	if len(rows) == 0 {
		return
	}

	// In the grid, one press of an arrow moves one *row*, not one card. The grid is two
	// columns wide from 1024 points (card.go's `lg:columns-2`), and moving by one card made
	// the down arrow jump to the neighbour on the right -- a cursor that goes sideways when
	// the reader asked it to go down.
	index := -1
	for i := range rows {
		if rows[i].TweetID == s.state.Selected {
			index = i
			break
		}
	}

	step := delta
	if s.state.ViewMode == domain.ViewGrid {
		step = delta * s.resultsColumns()
	}

	// With nothing selected, either direction starts at the nearest end rather than
	// jumping to the far one.
	next := index + step
	if index < 0 {
		if delta < 0 {
			next = len(rows) - 1
		} else {
			next = 0
		}
	}
	s.selectResultIndex(next)
}

// MoveSelectionBy moves one card, whatever the layout: the left and right arrows.
func (s *Shell) MoveSelectionBy(delta int) { s.selectResultIndex(s.selectedIndex() + delta) }

// SelectFirstResult and SelectLastResult are Home and End.
func (s *Shell) SelectFirstResult() { s.selectResultIndex(0) }

func (s *Shell) SelectLastResult() {
	s.selectResultIndex(len(s.state.Data.Bookmarks) - 1)
}

// PageSelection moves a screenful, which is what PageUp and PageDown mean.
func (s *Shell) PageSelection(delta int) { s.MoveSelection(delta * ResultsKeyboardPage) }

// selectedIndex is the selected bookmark's place in the loaded page, or -1.
func (s *Shell) selectedIndex() int {
	for i := range s.state.Data.Bookmarks {
		if s.state.Data.Bookmarks[i].TweetID == s.state.Selected {
			return i
		}
	}
	return -1
}

// resultsColumns is how many cards the grid lays side by side.
//
// The breakpoint is the window's, not the pane's: the grid switches to two columns at
// 1024 points of viewport. A tester has no window, so the state's own measured width
// stands in for it.
func (s *Shell) resultsColumns() int {
	if s.state.ViewMode != domain.ViewGrid {
		return 1
	}
	if s.state.ResultsColumns > 0 {
		return s.state.ResultsColumns
	}
	width := s.windowWidth()
	if width <= 0 {
		width = int(s.sidebarPaneWidth() + s.state.MiddleWidth)
	}
	if width >= ResultsTwoColumnWidth {
		return 2
	}
	return 1
}

// selectResultIndex makes the card at index the selection and reveals it.
func (s *Shell) selectResultIndex(index int) {
	rows := s.state.Data.Bookmarks
	if len(rows) == 0 {
		return
	}
	if index < 0 {
		index = 0
	}
	if index >= len(rows) {
		index = len(rows) - 1
	}
	s.state.Selected = rows[index].TweetID
	// The card that follows the arrows has to be on screen: with no scroll, moving past the
	// bottom of the grid selected rows nobody could see. The flag is one frame's worth, so
	// the reveal does not keep the reader from scrolling where they like.
	s.state.RevealSelected = true
	if s.window != nil {
		window := s.window
		time.AfterFunc(120*time.Millisecond, func() {
			window.Update(func() {
				s.state.RevealSelected = false
				window.Invalidate()
			})
		})
		window.Invalidate()
	}
}

// ToggleSelectedBatch is the space bar: it ticks the selected card's box without opening
// the detail. With nothing selected it picks the first result first, so space on a fresh
// list does something.
func (s *Shell) ToggleSelectedBatch() {
	if s.state.Selected == "" {
		s.SelectFirstResult()
	}
	id := s.state.Selected
	if id == "" {
		return
	}
	if s.state.SelectedIDs == nil {
		s.state.SelectedIDs = map[string]bool{}
	}
	if s.state.SelectedIDs[id] {
		delete(s.state.SelectedIDs, id)
	} else {
		s.state.SelectedIDs[id] = true
	}
	if s.window != nil {
		s.window.Invalidate()
	}
}

// ClearBatchSelection empties the ticked set.
func (s *Shell) ClearBatchSelection() {
	if len(s.state.SelectedIDs) == 0 {
		return
	}
	s.state.SelectedIDs = nil
	if s.window != nil {
		s.window.Invalidate()
	}
}

// showDetail is what clicking a card does: make it the inspected bookmark and open the
// detail.
//
// The card click sets the selected id, which decides both the card's selected outline and
// whether the detail modal is mounted. It is not a bulk toggle, and it does not touch
// SelectedIDs.
func (s *Shell) showDetail(tweetID string) {
	if tweetID == "" {
		return
	}
	s.state.Selected = tweetID
	s.state.DetailOpen = true
	if s.window != nil {
		s.window.Invalidate()
	}
}

// leaveSettings takes the window off the settings page without redrawing.
//
// Every navigation control has to call it: settings is drawn *instead of* the results, so a
// sidebar click that only reloads the list changes nothing on screen and reads as a dead
// control. It is separate from CloseSettings because the caller is about to redraw anyway.
func (s *Shell) leaveSettings() {
	if !s.state.SettingsOpen {
		return
	}
	s.state.SettingsOpen = false
	s.state.SettingsQuery = ""
}

// CloseDetail puts the detail away and hands the keyboard back to the card.
//
// The card is the element the detail was opened from, so closing returns the reader to the
// row they were on rather than to whatever the framework focuses next. Only the surfaces
// that are a detail use this: the reload path clears the flag without a reader involved.
func (s *Shell) CloseDetail() {
	if !s.state.DetailOpen {
		return
	}
	// The player is a view over the window and outlives the state it was opened from; closing
	// the detail with a video playing would leave the video over the results.
	s.StopVideo()
	s.state.DetailOpen = false
	s.state.FocusSelected = true
	s.invalidate()
}

// OpenSelectedDetail is what Enter means: show the selected bookmark's detail.
//
// With nothing selected it selects the first result, so pressing Enter on a fresh
// list does something rather than nothing.
func (s *Shell) OpenSelectedDetail() {
	if s.state.Selected == "" {
		if rows := s.state.Data.Bookmarks; len(rows) > 0 {
			s.state.Selected = rows[0].TweetID
		}
	}
	s.showDetail(s.state.Selected)
}

// openSyncSettings takes the user to the one place the two X cookies can be entered.
//
// The sync failure says "add the cookies under Settings > Sync"; a sentence that names a
// destination and leaves the user to find it is half an instruction.
func (s *Shell) openSyncSettings() {
	s.OpenSettings()
	s.state.SettingsSection = SettingsSectionSync
	if s.window != nil {
		s.window.Invalidate()
	}
}

// ExitSearch clears the search box.
//
// It is what Escape means in a search field, and it reloads only when there was
// something to clear, so Escape on an empty box does not trigger a query.
func (s *Shell) ExitSearch(ctx context.Context) {
	if s.state.Query == "" {
		return
	}
	s.state.Query = ""
	s.ReloadAsync(ctx)
}

// reconcileMiddleWidth derives the results pane's width from the two rail widths.
//
// It runs before the frame is built rather than inside it, because a view body must
// have no side effects: the columns are laid out from state, and this is what keeps
// that state consistent after a window resize.
func (s *Shell) reconcileMiddleWidth() {
	width := s.windowWidth()
	if width <= 0 {
		if s.state.MiddleWidth <= 0 {
			s.state.MiddleWidth = 600
		}
		return
	}

	middle := float32(width) - s.sidebarPaneWidth() - s.state.RightSidebarWidth
	// The dividers and the two 1 DIP lines need room of their own.
	if middle < 240 {
		middle = 240
	}
	s.state.MiddleWidth = middle
}

// windowWidth is the current window width, or 0 before the window exists.

// ToggleSidebar hides or restores the sidebar.
//
// The column is hidden rather than folded to an icon rail; the header's button
// brings it back. The width is remembered, so showing it again returns the column the
// reader dragged to rather than the default.
//
// The press is also a pick: from then on the column keeps the state the reader left it in
// at every width, because the layout follows the reader's pick once there is one.
func (s *Shell) ToggleSidebar() {
	s.state.SidebarCollapsed = !s.sidebarCollapsed()
	s.state.SidebarChosen = true
	s.persistLayout()
	s.invalidate()
}

// sidebarCollapsed reports whether the column is hidden.
//
// A reader's own choice wins at every width. Until there is one, the window decides, as
// the 1024-point breakpoint does: the sidebar is there when there is room
// and goes when there is not, which is what gives a narrow window its page back. The usual
// layout puts its page tabs in the header at that width; this build has no page tabs to
// move -- its navigation is the facet library the column holds, and the header already
// carries the sidebar's button and the page's crumb -- so the shell applies the rule itself.
func (s *Shell) sidebarCollapsed() bool {
	if s.state.SidebarChosen {
		return s.state.SidebarCollapsed
	}
	width := s.windowWidth()
	return width > 0 && width < NavSideWidth
}

// sidebarPaneWidth is the track the sidebar owns. Hidden, it owns nothing: the results
// start at the window's left edge.
func (s *Shell) sidebarPaneWidth() float32 {
	if s.sidebarCollapsed() {
		return 0
	}
	return s.state.LeftSidebarWidth
}

// windowWidth is the current window width, or 0 before the window exists.
func (s *Shell) windowWidth() int {
	if s.window == nil {
		return s.testedViewportWidth
	}
	width, _ := s.window.Size()
	return width
}

// SetTestedViewportWidth records the width a tester draws at.
//
// A tester has no window, so the rule that gives the sidebar up below NavSideWidth would
// never fire in a test without this; and the inspector's share and the page's measure read
// the same width, so a test that sets it lays out a page of that size in every respect.
func (s *Shell) SetTestedViewportWidth(width int) { s.testedViewportWidth = width }

// viewportHeight is the height the page is drawn at.
func (s *Shell) viewportHeight() float32 {
	if s.window != nil {
		if _, height := s.window.Size(); height > 0 {
			return float32(height)
		}
	}
	return DetailViewportFallback
}

// persistLayout writes the widths back.
//
// The inspector's width is recomputed here because the inner split resized the
// results pane, and it is the inspector's width that is persisted.
func (s *Shell) persistLayout() {
	if width := s.windowWidth(); width > 0 {
		derived := NormalizeInspectorWidth(float32(width) - s.sidebarPaneWidth() - s.state.MiddleWidth)
		s.state.RightSidebarWidth = derived
	}
	if s.settings == nil {
		return
	}
	// Idempotent: several rebuilds of one frame must not each write.
	if s.state.LeftSidebarWidth == s.persistedLeft && s.state.RightSidebarWidth == s.persistedRight &&
		s.state.SidebarCollapsed == s.persistedCollapsed && s.state.SidebarChosen == s.persistedChosen {
		return
	}
	s.persistedLeft, s.persistedRight = s.state.LeftSidebarWidth, s.state.RightSidebarWidth
	s.persistedCollapsed = s.state.SidebarCollapsed
	s.persistedChosen = s.state.SidebarChosen

	ctx := context.Background()
	preferences, err := s.settings.Load(ctx)
	if err != nil {
		s.reportErr(err)
		return
	}
	preferences.Layout = settings.LayoutPreferences{
		LeftSidebarWidth:  s.state.LeftSidebarWidth,
		RightSidebarWidth: s.state.RightSidebarWidth,
		// Written as it is on screen, so a build that predates the pick still opens with the
		// layout the reader last saw; the pick itself is what this build obeys.
		SidebarCollapsed: s.sidebarCollapsed(),
		SidebarChosen:    s.state.SidebarChosen,
	}
	if err := s.settings.Save(ctx, preferences); err != nil {
		s.reportErr(err)
	}
}

// ShowToast shows a transient line under the detail overlay.
func (s *Shell) invalidate() {
	if s.window != nil {
		s.window.Invalidate()
	}
}

// markPageChange starts the results pane's entrance.
//
// Every navigation calls it: another scope, a tag or an author, the other layout, the
// settings page, the tag library, the graph. It is not called for a new query or a new
// sort, because the page is the same page and the page entrance is per view, not per
// keystroke.
func (s *Shell) markPageChange() {
	s.state.PageChangedAt = time.Now()
}

// reportErr records a failure and shows it where the reader is looking.
//
// The results pane draws state.Err, but a settings sheet, the tag pages and the detail
// overlay all cover it: saving cookies or importing a bundle could fail with the message
// painted underneath the panel that caused it. The toast is at the top level of the shell,
// so it is visible whatever is open.
func (s *Shell) reportErr(err error) {
	if err == nil {
		return
	}
	s.ShowStatus(ToastError, err.Error())
}

// The status line's three tones: ok / warn / err.
const (
	ToastOK    = "ok"
	ToastWarn  = "warn"
	ToastError = "err"
)

// ShowToast is a message with the plain tone, which is what most callers mean.
func (s *Shell) ShowToast(message string) { s.ShowStatus(ToastOK, message) }

// toastTone is the ink and the tint for one of the three outcomes.
func toastTone(kind string, theme *ui.Theme) ui.Color {
	switch kind {
	case ToastOK:
		return theme.Success
	case ToastWarn:
		return theme.Warning
	case ToastError:
		return theme.Danger
	default:
		return theme.Text
	}
}

// ShowStatus shows a transient line at the foot of the window.
//
// One status line serves every outcome and its life is set by how long the
// sentence takes to read: 3.5s for a success, 8s for a failure, never more than 15s.
// The hover pause is not reproduced here: MyGo reports
// hover as a read (Element.Hovered) and this view must not write state, so there is no
// place to record "the pointer is on it".
func (s *Shell) ShowStatus(kind, message string) {
	s.state.ToastMessage = message
	s.state.ToastKind = kind
	duration := ToastDismissAfter
	if kind == ToastWarn {
		duration = ToastWarnAfter
	}
	if kind == ToastError {
		duration = ToastErrorAfter
	}
	if longest := time.Duration(len(message)) * ToastReadPerChar; longest > duration {
		duration = longest
	}
	if duration > ToastMaxAfter {
		duration = ToastMaxAfter
	}
	s.state.ToastUntil = time.Now().Add(duration)
	if s.window != nil {
		s.window.Invalidate()
		// Nothing else repaints when the deadline passes: ToastText starts returning ""
		// but the window keeps the last frame, so the toast stayed on screen until some
		// unrelated event forced a redraw. Schedule the repaint the expiry needs.
		window := s.window
		time.AfterFunc(duration+time.Millisecond, func() {
			window.Update(func() { window.Invalidate() })
		})
	}
}

// ToastText is the message still worth showing, and empty once it has expired.
func (s *Shell) ToastText() string {
	if s.state.ToastMessage == "" {
		return ""
	}
	if time.Now().After(s.state.ToastUntil) {
		return ""
	}
	return s.state.ToastMessage
}

// clipboardWriter is the one clipboard method the copy action needs.
//
// Narrowing it to an interface is what makes the action testable: ui.Context is a
// concrete struct, so a test cannot substitute one, and a test that drove the real
// clipboard would be testing the operating system.
type clipboardWriter interface {
	WriteClipboard(s string)
}

// CopySourceLink copies the selected bookmark's link, or says why it cannot.
//
// The toast appears when there is no link to copy, which is the only path a
// user can reach without a clipboard; the same message covers a bookmark whose URL is
// empty, and copying is only attempted when a writer is installed.
func (s *Shell) CopySourceLink(c clipboardWriter) {
	row, found := s.selectedBookmark()
	// This appears when there is no link to copy, which is the case for a bookmark the
	// API returned without a URL.
	if !found || row.TweetURL == "" {
		s.ShowToast(s.t().T(i18n.ToastNoLink))
		return
	}

	c.WriteClipboard(row.TweetURL)
	// The chip says so in place as well as the toast: the reader's eye is on the button they
	// pressed, and a confirmation that appears somewhere else is a confirmation they have to
	// go looking for. The deadline repaints, because nothing else will when it passes.
	s.state.CopiedAt = time.Now()
	if s.window != nil {
		window := s.window
		time.AfterFunc(CopiedFeedbackFor+time.Millisecond, func() {
			window.Update(func() { window.Invalidate() })
		})
	}
	s.ShowToast(s.t().T(i18n.ToastDone))
}

// detailOverlay draws the detail as a modal rather than as a column.
//
// The card is min(94vw, 980px) wide with a max-height of 86dvh, and it fades in while
// scaling from 0.96. The inspector is drawn as the whole card.

// suggestionsPanel draws the review list.
func (s *Shell) suggestionsPanel(c *ui.Context, theme *ui.Theme, entrance float32) {
	rail := Rail(s.state.Appearance, s.state.SystemDark, s.translator.Locale() == i18n.ZHCN)
	ui.Column(c).Width(560).GapY(SpaceLoose).Padding(PaddingPanel).Radius(RadiusDialog).
		Opacity(EntranceDetailCard.OpacityAt(entrance)).
		Scale(EntranceDetailCard.ScaleAt(entrance), EntranceDetailCard.ScaleAt(entrance)).Children(func() {
		ui.Text(c, s.t().T(i18n.SuggestionsTitle)).
			Font(s.displayFace()).FontSize(FontModalTitle).FontWeight(600)

		if len(s.state.Suggestions) == 0 {
			ui.Text(c, s.t().T(i18n.SuggestionsEmpty)).FontSize(FontBody).TextColor(theme.TextMuted)
			return
		}

		// The bulk bar: how many are waiting, and one button that accepts the ones on
		// screen. There is no reject-all, so there is none here.
		ui.Row(c).FillWidth().GapX(SpaceBase).Children(func() {
			ui.Text(c, fmt.Sprintf("%d %s", len(s.state.Suggestions), s.t().T(i18n.SuggestionsPendingTitle))).
				FontSize(FontSmall).TextColor(theme.TextMuted)
			ui.Spacer(c).FillWidth()
			IconChip(c, theme, rail, "check", s.t().T(i18n.SuggestionsAcceptAll), func() {
				s.AcceptVisibleSuggestions(context.Background())
			})
		})

		ui.Scroll(c).MaxHeight(420).Children(func() {
			ui.Column(c).FillWidth().GapY(6).Children(func() {
				for i := range s.state.Suggestions {
					row := s.state.Suggestions[i]
					id := row.Suggestion.ID

					ui.Row(c).FillWidth().Padding(8).GapX(8).Radius(theme.Radius).
						Background(theme.Surface).Key(id).Children(func() {
						ui.Column(c).FillWidth().GapY(2).Children(func() {
							ui.Text(c, row.Suggestion.TagName).FontSize(FontBody).Bold().TextColor(theme.Text)
							if row.Bookmark != nil {
								ui.Text(c, row.Bookmark.Text).FontSize(FontTiny).
									TextColor(theme.TextMuted).MaxLines(2)
							}
						})

						// Each row's pair says which tag it acts on: the label carries the
						// tag name, because a list of rows drawn with the bare words is a
						// list of equally named buttons.
						IconChip(c, theme, rail, "check",
							s.t().Tf(i18n.SuggestionsAcceptFor, row.Suggestion.TagName), func() {
								if err := s.AcceptSuggestion(context.Background(), id); err != nil {
									s.reportErr(err)
								}
							})
						IconChip(c, theme, rail, "close",
							s.t().Tf(i18n.SuggestionsRejectFor, row.Suggestion.TagName), func() {
								if err := s.RejectSuggestion(context.Background(), id); err != nil {
									s.reportErr(err)
								}
							})
					})
				}
			})
		})
	})
}

// LoadSyncSummary reads the stored sync result into the status line.
//
// Without this the line is empty after every launch: the summary is persisted, so the
// information exists and only the read was missing.
func (s *Shell) LoadSyncSummary(ctx context.Context) {
	if s.syncer == nil {
		return
	}

	summary, found, err := s.syncer.Summary(ctx)
	if err != nil || !found {
		return
	}
	s.applySyncSummary(summary)
}

// applySyncSummary installs a stored sync result.
//
// The read and the install are separate because the reload engine reads off the
// window's thread and applies on it (reload.go); this half only touches state.
func (s *Shell) applySyncSummary(summary domain.SyncSummary) {
	// A stored summary older than what this window already knows is not news.
	//
	// LoadSyncSummary runs on every Reload, including the one the sync's own publish
	// callback triggers. When a run fails before its finishing write lands -- a network
	// failure the proxy would have fixed, say -- the stored summary is still the *previous*
	// run's, and adopting it told the user "X login expired, sign in again" about a sync
	// that had just failed for an unrelated reason, while their cookies were fine.
	// Measured on this machine: a 2026-10-24 unauthorized summary was being shown as the
	// result of two 2026-10-25 runs that never finished recording.
	//
	// The comparison is on LastSyncedAt, the moment the stored run finished. An absent
	// timestamp cannot be compared, so it is adopted: a summary written before the app had
	// a clock is still better than nothing on a window that has never synced.
	if summary.LastSyncedAt != nil {
		if s.state.LastSyncedAt.After(*summary.LastSyncedAt) {
			return
		}
		s.state.LastSyncedAt = *summary.LastSyncedAt
	}
	// A run in flight owns the status line; a stale summary must not overwrite it.
	if !s.state.SyncRunning {
		s.state.LastSyncStatus = summary.Status
		// The stored run's kind is what makes the failure sentence survive a restart. Only a
		// run finishing in this session used to set it, so a relaunch showed the bare "失败"
		// with no explanation of what to fix. Cleared on success, because the sentence belongs
		// to the failure: leaving the last run's kind in place showed it under a successful sync.
		s.state.LastSyncErrorKind = string(summary.ErrorKind)
		s.state.LastSyncErrorSummary = summary.ErrorSummary
	}
}

// AcceptVisibleSuggestions applies every suggestion currently listed.
//
// The button accepts all suggestions over what is visible, and the batch is what is on
// screen rather than everything pending: a filtered review should not accept rows the
// user cannot see.
//
// One failure does not stop the rest, which is the service's behaviour and the right one
// for a batch: aborting on the first bad row would make the user start over.
func (s *Shell) AcceptVisibleSuggestions(ctx context.Context) int {
	if s.suggestions == nil || len(s.state.Suggestions) == 0 {
		return 0
	}

	ids := make([]string, 0, len(s.state.Suggestions))
	for _, row := range s.state.Suggestions {
		ids = append(ids, row.Suggestion.ID)
	}

	accepts, failures := s.suggestions.AcceptAll(ctx, ids)
	if len(failures) > 0 {
		// A count rather than one row's message: the batch is what failed.
		s.ShowStatus(ToastError, fmt.Sprintf("%d of %d suggestions could not be applied", len(failures), len(ids)))
	}
	s.refreshSuggestions(ctx)

	// The batch writes outside AcceptSuggestion, so nothing else re-read what it changed.
	// Refreshing the panel's own list left the feed's banner, the auto-tagging page's
	// pending row and the cards' tags saying what was true before the batch, and the tag
	// page underneath the panel still held its old counts when the panel closed.
	s.ReloadAsync(ctx)
	if s.state.TagMode {
		s.ReloadTagLibraryAsync(ctx)
	}
	return len(accepts)
}

// BeginCreateTag shows the sidebar's inline create row, or sends the cursor back to it.
//
// The "+" is not a toggle: pressed while the row is open it focuses the field and leaves the
// half-typed name alone, because a second press means "let me finish this" far more often
// than "throw this away". Cancel discards it.
func (s *Shell) BeginCreateTag() {
	if s.state.CreatingTag {
		s.tagDraftField.Focus()
		return
	}

	// One draft row at a time: both rows edit TagDraft, and drawing two fields over one
	// string let each show what the other had typed.
	s.state.TagLibraryCreateOpen = false
	s.state.CreatingTag = true
	s.state.TagDraft = ""
	s.tagDraftField.Focus()
	if s.window != nil {
		s.window.Invalidate()
	}
}

// CancelCreateTag hides the row and forgets what was typed into it.
func (s *Shell) CancelCreateTag() {
	s.resetTagDraft()
	if s.window != nil {
		s.window.Invalidate()
	}
}

// BeginLibraryCreateTag shows the tag library's own create row.
//
// Same rule as the sidebar's "+": an open row takes the cursor rather than closing, and a
// draft the other row holds is dropped so only one field is ever editing TagDraft.
func (s *Shell) BeginLibraryCreateTag() {
	if s.state.TagLibraryCreateOpen {
		s.tagLibraryDraftField.Focus()
		return
	}

	s.state.CreatingTag = false
	s.state.TagLibraryCreateOpen = true
	s.state.TagDraft = ""
	s.tagLibraryDraftField.Focus()
	if s.window != nil {
		s.window.Invalidate()
	}
}

// AddBookmarkFromDraft stores the link in the add row.
func (s *Shell) AddBookmarkFromDraft(ctx context.Context) {
	if s.service == nil {
		return
	}
	raw := strings.TrimSpace(s.state.AddURL)
	if raw == "" {
		return
	}
	if _, err := s.service.AddBookmark(ctx, raw, s.state.AddText); err != nil {
		// A refused add keeps the text in the field: deleting what the user typed because
		// the link was wrong would make a typo cost the whole entry.
		s.state.AddProblem = s.t().T(i18n.AddNotAPostLink)
		if s.window != nil {
			s.window.Invalidate()
		}
		return
	}
	s.state.AddURL = ""
	s.state.AddText = ""
	s.state.AddProblem = ""
	s.ReloadAsync(ctx)
}

// addRow is the entry point for a post the user pastes in.
//
// It is the app's own entry point: without it a standalone manager could only ever show
// what a sync brought in.
