package ui

import (
	"context"
	"fmt"

	"github.com/egoist/mygo/ui"
	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/i18n"
	"github.com/taoquo/trove/internal/usecase/settings"
)

// handleConfirmations reads the dialog's answer.
//
// MyGo's AlertDialog reports through its open pointer and its return value, so
// the answer can only be read during a build pass. Clearing the confirmation
// before acting is what keeps that safe.
// handleTagDeleteConfirmations draws the two tag-deletion dialogs and the merge confirmation.
//
// The merge asks here because it deletes what it folds in.
//
// The confirm button must not be labelled "Cancel" while choosing it performs the deletion: a
// button's label and what it does are two things, and only the pair shows that one is wrong.
// dangerDialog shows a destructive confirmation whose default button is Cancel.
//
// MyGo gives the last button the accent colour and the focus, and Enter clicks it. The old
// dialogs put Delete last, so the focused, Enter-activated button was the destructive one --
// the opposite of what a confirmation exists for. Delete now comes first and Cancel last
// The return value is the clicked button's index, 0 for the
// destructive one and 1 for Cancel; AlertDismissed means Escape, which is also a refusal.
// The second result is whether the dialog is still on screen. A build can run more than once
// per frame, and only a pass that closed the dialog may clear the state that asked for it --
// clearing on the first pass made the dialog vanish before the frame was photographed.
func (s *Shell) dangerDialog(c *ui.Context, title, message, confirmLabel string) (int, bool) {
	open := true
	choice := ui.AlertDialog(c, &open, title, message, confirmLabel, s.t().T(i18n.ActionCancel))
	return choice, open
}

func (s *Shell) handleTagDeleteConfirmations(c *ui.Context) {
	if s.state.ConfirmMergeTags {
		target, found := s.mergeTargetTag()
		count := 0
		if found {
			count = len(s.mergeSources(target.TagID))
		}
		// The body names the survivor and the count: "merged into X" alone does not say how
		// many rows are about to disappear from the library.
		choice, open := s.dangerDialog(c, s.t().T(i18n.TagMergeLabel),
			s.t().Tf(i18n.TagMergeConfirmBody, count, target.Name), s.t().T(i18n.TagMergeLabel))
		if open {
			return
		}
		if choice == 0 {
			s.ConfirmMergeSelectedTagsNow(context.Background())
			return
		}
		// Cancelled or dismissed: the tags stay, and the ticks stay too, so the reader can
		// change the target instead of rebuilding the selection.
		s.state.ConfirmMergeTags = false
		s.invalidate()
		return
	}

	if s.state.ConfirmDeleteTag != "" {
		choice, open := s.dangerDialog(c, s.t().T(i18n.ActionDelete),
			s.t().T(i18n.DeleteTagConfirm), s.t().T(i18n.ActionDelete))
		if open {
			return
		}
		if choice == 0 {
			s.ConfirmDeleteTagNow(context.Background())
			return
		}
		// A cancelled or dismissed dialog must also close: leaving the field set made the
		// next frame draw the same question again.
		s.state.ConfirmDeleteTag = ""
		s.invalidate()
		return
	}

	if s.state.ConfirmDeleteTags {
		choice, open := s.dangerDialog(c, s.t().T(i18n.ActionDelete),
			fmt.Sprintf("%s (%d)", s.t().T(i18n.DeleteTagsConfirm), s.SelectedTagCount()),
			s.t().T(i18n.ActionDelete))
		if open {
			return
		}
		if choice == 0 {
			s.ConfirmDeleteSelectedTagsNow(context.Background())
			return
		}
		s.state.ConfirmDeleteTags = false
		s.invalidate()
	}
}

// handleResetConfirmation draws the reset dialog.
func (s *Shell) handleResetConfirmation(c *ui.Context) {
	if !s.state.ConfirmResetData {
		return
	}

	choice, open := s.dangerDialog(c, s.t().T(i18n.ResetLocalData),
		s.t().T(i18n.ResetLocalDataConfirm), s.t().T(i18n.ResetLocalData))
	if open {
		return
	}
	if choice == 0 {
		s.ConfirmResetLocalDataNow(context.Background())
		return
	}
	// Dismissed or cancelled: nothing happens.
	s.state.ConfirmResetData = false
	s.invalidate()
}

// handleListDeleteConfirmation draws the list-deletion dialog.
//
// It follows the same rule as the other danger actions: the dialog's answer is the only
// thing that runs the delete, so dismissing it leaves the list alone.
func (s *Shell) handleListDeleteConfirmation(c *ui.Context) {
	if s.state.ConfirmDeleteList == "" {
		return
	}

	choice, open := s.dangerDialog(c, s.t().T(i18n.ActionDelete),
		s.t().T(i18n.DeleteListConfirm), s.t().T(i18n.ActionDelete))
	if open {
		return
	}
	if choice != 0 {
		// Cancelled or dismissed without choosing: nothing happens.
		s.state.ConfirmDeleteList = ""
		s.invalidate()
		return
	}

	s.ConfirmDeleteListNow(context.Background())
}

func (s *Shell) handleConfirmations(c *ui.Context) {
	if !s.deleteBookmarks.Confirmation.Asked() {
		return
	}

	// The bundle question is checked as well, because only one dialog is on screen
	// at a time and either may be the one that is open.
	if s.bundle.Confirmation.Asked() {
		confirmation := &s.bundle.Confirmation
		buttons := confirmation.Buttons()
		open := confirmation.Open()

		result := ui.AlertDialog(c, open, confirmation.Title, confirmation.Body, buttons...)
		if !*open {
			confirmation.Answer(result)
			_ = s.applyBundle(context.Background())
		}
		return
	}

	confirmation := &s.deleteBookmarks.Confirmation
	buttons := confirmation.Buttons()
	open := confirmation.Open()

	result := ui.AlertDialog(c, open, confirmation.Title, confirmation.Body, buttons...)
	if !*open {
		confirmation.Answer(result)
		s.applyDeleteBookmarks(context.Background())
	}
}

// State exposes the state for tests.

// handleCleanupConfirmation draws the orphan-cleanup dialog.
func (s *Shell) handleCleanupConfirmation(c *ui.Context) {
	if !s.state.ConfirmCleanupTags {
		return
	}

	choice, open := s.dangerDialog(c, s.t().T(i18n.TagCleanupOrphans),
		s.t().T(i18n.TagCleanupConfirm), s.t().T(i18n.TagCleanupOrphans))
	if open {
		return
	}
	s.state.ConfirmCleanupTags = false
	if choice == 0 {
		s.CleanupOrphanTags(context.Background())
	}
	if s.window != nil {
		s.window.Invalidate()
	}
}

// CleanupOrphanTags sweeps the tags that are safe to delete.
func (s *Shell) CleanupOrphanTags(ctx context.Context) {
	if s.service == nil {
		return
	}
	removed, err := s.service.CleanupOrphanTags(ctx)
	if err != nil {
		s.reportErr(err)
		return
	}
	s.state.TagCleanupMessage = s.t().Tf(i18n.TagCleanupDone, removed)
	// The sweep deletes tags, so the results must be read again too: the reload drops a
	// condition naming a swept tag (reload.go), which the sidebar and the chips then stop
	// drawing.
	s.ReloadAsync(ctx)
	s.ReloadTagLibraryAsync(ctx)
}

// ToggleGraph switches between the results list and the relation graph.
//
// The graph is built on the way in rather than on every frame: it walks every
// bookmark/tag pair, and rebuilding it per frame would make the window crawl.
func (s *Shell) ToggleGraph(ctx context.Context) {
	s.leaveSettings()
	s.state.GraphMode = !s.state.GraphMode
	s.markPageChange()
	if !s.state.GraphMode {
		s.ReloadAsync(ctx)
		return
	}
	// Entering the graph from tag management must leave the tag pages: they draw over the
	// graph, so flipping the flag alone made the tag page's graph button do nothing.
	s.state.TagMode = false
	s.RebuildGraph(ctx)
	s.invalidate()
}

// ToggleGraphAuthors rebuilds the graph with or without its author nodes.
//
// The author nodes are a separate layer that costs width, so they are opt-in.
func (s *Shell) ToggleGraphAuthors(ctx context.Context) {
	s.state.GraphIncludeAuthors = !s.state.GraphIncludeAuthors
	s.RebuildGraph(ctx)
}

// TagSelectionCount is how many tags the merge will act on.
func (s *Shell) TagSelectionCount() int {
	count := 0
	for _, selected := range s.state.TagSelectedIDs {
		if selected {
			count++
		}
	}
	return count
}

// SelectListedTags ticks every tag the current filter is showing.
func (s *Shell) SelectListedTags(items []domain.TagLibraryItem) {
	if s.state.TagSelectedIDs == nil {
		s.state.TagSelectedIDs = map[string]bool{}
	}
	for _, item := range items {
		s.state.TagSelectedIDs[item.TagID] = true
	}
	if s.window != nil {
		s.window.Invalidate()
	}
}

// DetachTagFromSelected removes a tag from the bookmark the detail is showing.
//
// RemoveTagFromSelection works on the batch, and a single selection is not in the batch:
// the detail's pill would have called it and nothing would have happened.
// ClearTagSelection drops the whole selection.
//
// The bulk bar's ✕ clears the selection: a selection made across several rows should not
// have to be undone one checkbox at a time.
func (s *Shell) ClearTagSelection() {
	s.state.TagSelectedIDs = map[string]bool{}
	if s.window != nil {
		s.window.Invalidate()
	}
}

func (s *Shell) DetachTagFromSelected(ctx context.Context, tagID string) {
	if s.service == nil || tagID == "" || s.state.Selected == "" {
		return
	}
	if _, err := s.service.RemoveTags(ctx, []string{s.state.Selected}, []string{tagID}); err != nil {
		s.reportErr(err)
		return
	}
	s.ReloadAsync(ctx)
	if s.window != nil {
		s.window.Invalidate()
	}
}

// focusedGraphNode is the node the graph is centred on, or nothing.
func (s *Shell) focusedGraphNode() (domain.GraphViewNode, bool) {
	tagID := s.focusedGraphTag()
	if tagID == "" {
		return domain.GraphViewNode{}, false
	}
	for _, node := range s.state.GraphModel.Nodes {
		if node.TagID == tagID {
			return node, true
		}
	}
	return domain.GraphViewNode{}, false
}

// OpenFocusedGraphTagBookmarks leaves the graph for the list of that tag's bookmarks.
//
// The focus is already a tag condition, so leaving the graph is all this does -- no
// toggle. The old version toggled, which meant "show me this tag's bookmarks" removed the
// filter whenever the tag was the one already narrowing the list.
func (s *Shell) OpenFocusedGraphTagBookmarks() {
	if s.focusedGraphTag() == "" {
		return
	}
	s.state.GraphMode = false
	s.ReloadAsync(context.Background())
}

// ClearGraphFocus drops the tag the graph is centred on.
//
// The graph's focus is the first tag condition, so clearing it removes that condition and
// the results widen with it. The focus is not separate from the feed: a control that
// quietly changed the list without saying so would be worse than one that says it does.
func (s *Shell) ClearGraphFocus(ctx context.Context) {
	if len(s.state.ActiveTagIDs) == 0 {
		return
	}
	s.toggleTagFilter(ctx, s.state.ActiveTagIDs[0])
}

// focusedGraphTag is the tag the graph highlights: the first tag condition.
//
// A tag is a condition now, so this reads ActiveTagIDs rather than the selection. There
// is one focus even when several tags narrow the list, and the first is the one the user
// added first -- the same one the chip row shows leftmost.
func (s *Shell) focusedGraphTag() string {
	if len(s.state.ActiveTagIDs) == 0 {
		return ""
	}
	return s.state.ActiveTagIDs[0]
}

// CycleSort advances to the next sort order and wraps.
//
// Wrapping rather than stopping at the end is what makes it a cycle: the control has
// no other way back to the first order.
func (s *Shell) CycleSort(ctx context.Context) {
	if len(s.sortCycle) == 0 {
		return
	}

	// Find the current order in the cycle rather than advancing the whitelist index:
	// the two lists differ, so an index into one is not an index into the other.
	position := 0
	for index, order := range s.sortCycle {
		if order == s.state.Sort {
			position = index
			break
		}
	}
	next := s.sortCycle[(position+1)%len(s.sortCycle)]

	// The whitelist index still has to move, or setSort rejects the order.
	for index, order := range s.sortOptions {
		if order == next {
			s.setSort(ctx, index)
			return
		}
	}
}

// CreateListFromInput adds a list from the sidebar's field.
//
// An empty field does nothing rather than creating a nameless list; a duplicate name is
// refused by the service, which is where the rule lives.

// AskDeleteList opens the confirmation for a list.
//
// Deleting a list is a danger action, so it goes through the same fail-closed
// confirmation as deleting bookmarks: the dialog's result is the only thing that runs
// the delete.

// ConfirmDeleteListNow deletes the list a confirmation is open for, moving its
// bookmarks to the Inbox.
//
// The destination is the Inbox rather than nothing: the service refuses a delete with no
// destination, and the bookmarks would otherwise lose their home.
func (s *Shell) ConfirmDeleteListNow(ctx context.Context) {
	listID := s.state.ConfirmDeleteList
	s.state.ConfirmDeleteList = ""
	if s.service == nil || listID == "" {
		return
	}

	if err := s.service.DeleteList(ctx, listID, domain.InboxListID); err != nil {
		s.reportErr(err)
		if s.window != nil {
			s.window.Invalidate()
		}
		return
	}

	s.state.Err = ""
	s.ReloadAsync(ctx)
}

// ApplySavedView restores a stored view's filters.

// DeleteSavedView removes a stored view.

// ToggleViewMode switches between the grid and the list.
func (s *Shell) ToggleViewMode() {
	s.leaveSettings()
	if s.state.ViewMode == domain.ViewGrid {
		s.state.ViewMode = domain.ViewList
	} else {
		s.state.ViewMode = domain.ViewGrid
	}
	// Grid and list are two arrangements of the same feed, but the reader changed the page
	// under them; the entrance is what tells them the click landed.
	s.markPageChange()
	// The preference is stored beside the layout, so a restart keeps the arrangement.
	if s.settings != nil {
		preferences, err := s.settings.Load(context.Background())
		if err == nil {
			preferences.ViewMode = settings.ViewMode(s.state.ViewMode)
			_ = s.settings.Save(context.Background(), preferences)
		}
	}
	if s.window != nil {
		s.window.Invalidate()
	}
}

// currentSortLabel is the active order's label, which the button shows.
//
// It reads the order rather than the index: the index is into the whitelist, and the
// label depends only on the order, so deriving one from the other would make a correct
// order display the wrong word whenever the two lists differ.
func (s *Shell) currentSortLabel() string {
	key, known := sortLabelKeys[s.state.Sort]
	if !known {
		// An unknown order falls back to the default.
		key = i18n.SortTimeline
	}
	return s.t().T(key)
}

// setSort changes the order and reloads.
func (s *Shell) setSort(ctx context.Context, index int) {
	if index < 0 || index >= len(s.sortOptions) {
		return
	}
	s.state.Sort = s.sortOptions[index]
	s.ReloadAsync(ctx)
}
