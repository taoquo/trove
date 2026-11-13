package ui

import (
	"context"
	"errors"
	"strings"

	"github.com/egoist/mygo/ui"
	engine "github.com/taoquo/trove/internal/autotag"
	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/i18n"
	"github.com/taoquo/trove/internal/usecase/workspace"
)

// selectSidebar changes the scope and reloads.
//
// The selection is cleared because a bookmark selected in one scope is not
// necessarily in the next one, and an inspector showing a bookmark the list no longer
// contains is worse than showing nothing.

// toggleTagFilter adds or removes a tag from the narrowing set.
//
// Tag membership is toggled rather than picked, and the query layer combines the set with
// AND. Picking a single tag would make a second click replace the first instead of adding to
// it.
//
// The scope is deliberately untouched: a tag is a condition, not somewhere to be. Writing the
// tag into the selection would make one control mean two things -- a filter and a place -- and
// leave the title naming a tag while the list is also narrowed by authors. The scope stays
// where the user put it and the chip row says what is narrowing.
func (s *Shell) toggleTagFilter(ctx context.Context, tagID string) {
	if tagID == "" {
		return
	}
	s.leaveSettings()

	// One tag at a time. Clicking another tag replaces the one in effect rather than
	// adding to it: the user asked for this after using the AND model, because a set of
	// tags reads as a query the reader has to work out from the chips. Clicking the active row
	// again clears it.
	if s.tagActive(tagID) {
		s.state.ActiveTagIDs = nil
	} else {
		s.state.ActiveTagIDs = []string{tagID}
	}
	s.markPageChange()
	s.ReloadAsync(ctx)
}

// ToggleTagMode switches between the results list and the tag management view.
func (s *Shell) ToggleTagMode(ctx context.Context) {
	s.leaveSettings()
	s.state.TagMode = !s.state.TagMode
	s.state.GraphMode = false
	s.markPageChange()
	if !s.state.TagMode {
		s.ReloadAsync(ctx)
		return
	}
	// Enter on the table. The auto-tagging page is a place you go for a task, not a mode to
	// resume; a screen that opens on configuration because that is where you left it buries
	// the table the reader asked for.
	s.state.TagView = TagViewTags
	s.ReloadTagLibraryAsync(ctx)
}

// OpenTagManagementAutoTag closes the settings sheet and opens the auto-tagging page of tag
// management. It is what the signpost in Settings calls; it does not go through ToggleTagMode
// because that resets the page to the table.
func (s *Shell) OpenTagManagementAutoTag() {
	s.closePanels()
	s.state.TagMode = true
	s.state.GraphMode = false
	s.state.TagView = TagViewAutoTag
	// The page draws stored configuration, not whatever the shell happens to hold: this entry
	// point must read the stored values itself.
	s.loadAutoTagSettings(context.Background())
	s.ReloadTagLibraryAsync(context.Background())
	s.ReloadPendingSuggestions(context.Background())
	if s.window != nil {
		s.window.Invalidate()
	}
}

// ReloadTagLibrary rebuilds the tag management view.
func (s *Shell) ReloadTagLibrary(ctx context.Context) {
	items, err := s.service.LoadTagLibrary(ctx)
	if err != nil {
		s.reportErr(err)
		return
	}
	s.state.TagItems = items
	if s.window != nil {
		s.window.Invalidate()
	}
}

// ReloadTagLibraryAsync reads the tag page off the window's thread.
//
// LoadTagLibrary reads every tag relation to count each tag's bookmarks, so on a large
// library it is in the same class as a workspace refresh (reload.go). Opening the tag
// page and every create, rename and delete used to pay that on the click.
//
// A request that arrives while a read is running is not dropped: it sets TagLibraryDirty
// and the read that is finishing runs again, so the page always ends up showing the
// library as it is now rather than as it was before the last edit.
func (s *Shell) ReloadTagLibraryAsync(ctx context.Context) {
	if s.service == nil {
		return
	}
	if s.state.TagLibraryLoading {
		s.state.TagLibraryDirty = true
		return
	}

	s.state.TagLibraryLoading = true
	s.invalidate()
	s.runInBackground(func() {
		items, err := s.service.LoadTagLibrary(ctx)
		s.publish(func() {
			s.state.TagLibraryLoading = false
			if err != nil {
				s.state.TagLibraryDirty = false
				s.reportErr(err)
				s.invalidate()
				return
			}
			s.state.TagItems = items
			s.invalidate()
			if s.state.TagLibraryDirty {
				s.state.TagLibraryDirty = false
				s.ReloadTagLibraryAsync(ctx)
			}
		})
	})
}

// VisibleTagItems applies the current query, filter and sort.
func (s *Shell) VisibleTagItems() []domain.TagLibraryItem {
	return domain.GetVisibleTagLibraryItems(
		s.state.TagItems, s.state.TagQuery, s.state.TagFilter, s.state.TagSort)
}

// OpenVocabularyReview shows the panel and asks the model.
//
// The request runs on a goroutine because it is a network call, and the result is
// published through the window so the main thread is the only one touching state.
func (s *Shell) OpenVocabularyReview() {
	s.closePanels()
	s.state.ReviewOpen = true
	s.state.ReviewRunning = true
	s.state.ReviewError = ""
	s.state.ReviewApplied = ""
	s.state.ReviewMerges = nil
	s.state.ReviewRenames = nil
	s.state.ReviewSelected = map[string]bool{}

	service := s.tagging
	window := s.window

	go func() {
		var result engine.TagVocabularyReviewResult
		if service != nil {
			result = service.ReviewVocabulary(context.Background())
		} else {
			result = engine.TagVocabularyReviewResult{Error: "no tagging service is configured"}
		}

		publish := func() {
			s.state.ReviewRunning = false
			if result.Error != "" {
				s.state.ReviewError = result.Error
				return
			}
			s.state.ReviewMerges = result.Merges
			s.state.ReviewRenames = result.Renames
			// Everything proposed is selected to begin with: a review the user asked
			// for and then has to tick every row of is a review they will abandon.
			for _, merge := range result.Merges {
				s.state.ReviewSelected[engine.MergePlanKey(merge)] = true
			}
			for _, rename := range result.Renames {
				s.state.ReviewSelected[engine.RenamePlanKey(rename)] = true
			}
		}

		if window != nil {
			window.Update(publish)
		} else {
			publish()
		}
	}()
}

// ApplyVocabularyReview applies the selected proposals.
//
// Merges run before renames: a rename onto a name a merge is about to free would
// otherwise collide, and a merge is the larger change so it should land first.
//
// Each plan is isolated: one rejected proposal must not throw away the good ones behind it,
// and what did not land stays in the panel with its box ticked so the button can finish the
// job. The panel stays up while the write runs -- it is the slowest edit in the app, and a
// panel that vanished on the press left nothing on screen to say it was working.
func (s *Shell) ApplyVocabularyReview(ctx context.Context) {
	if s.service == nil || s.state.ReviewApplying || s.state.Mutating {
		return
	}
	if s.ReviewSelectionCount() == 0 {
		return
	}

	// The plan is captured before the work leaves the window's thread: the review
	// selection is state, and state is read here, not there.
	merges := append([]engine.TagMergePlan(nil), s.state.ReviewMerges...)
	renames := append([]engine.TagRenamePlan(nil), s.state.ReviewRenames...)
	selected := make(map[string]bool, len(s.state.ReviewSelected))
	for key, on := range s.state.ReviewSelected {
		selected[key] = on
	}

	// The worker fills this; done reads it on the window's thread after the write, and the
	// publish between them is what orders the two.
	var leftover struct {
		merges   []engine.TagMergePlan
		renames  []engine.TagRenamePlan
		failures []error
	}

	s.state.ReviewApplying = true
	s.state.ReviewError = ""

	started := s.runLibraryWrite(i18n.ReviewApplying, func() libraryWrite {
		applied := 0
		for _, merge := range merges {
			if !selected[engine.MergePlanKey(merge)] {
				continue
			}
			if _, err := s.service.MergeTags(ctx, merge.SourceTagIDs, merge.TargetTagID); err != nil {
				leftover.failures = append(leftover.failures, err)
				leftover.merges = append(leftover.merges, merge)
				continue
			}
			applied++
		}

		for _, rename := range renames {
			if !selected[engine.RenamePlanKey(rename)] {
				continue
			}
			if _, err := s.service.RenameTag(ctx, rename.TagID, rename.To); err != nil {
				leftover.failures = append(leftover.failures, err)
				leftover.renames = append(leftover.renames, rename)
				continue
			}
			applied++
		}

		return libraryWrite{count: applied}
	}, func(outcome libraryWrite) {
		s.state.ReviewApplying = false
		// The library changed either way, so what landed is shown either way.
		s.ReloadTagLibraryAsync(ctx)

		if len(leftover.failures) > 0 {
			// The sentences are built here, on the window's thread, because the translator
			// belongs to the window (tagWriteMessage turns a refused rename into the
			// reader's language).
			messages := make([]string, len(leftover.failures))
			for i, err := range leftover.failures {
				messages[i] = s.tagWriteMessage(err)
			}
			s.state.ReviewError = strings.Join(messages, "; ")
			// The list holds only what did not land, ticked again: the button's next press
			// finishes the review instead of retrying plans that already succeeded.
			s.state.ReviewMerges = leftover.merges
			s.state.ReviewRenames = leftover.renames
			s.state.ReviewSelected = map[string]bool{}
			for _, merge := range leftover.merges {
				s.state.ReviewSelected[engine.MergePlanKey(merge)] = true
			}
			for _, rename := range leftover.renames {
				s.state.ReviewSelected[engine.RenamePlanKey(rename)] = true
			}
			if outcome.count > 0 {
				s.state.ReviewApplied = s.t().Tf(i18n.ReviewAppliedCount, outcome.count)
			}
			return
		}

		s.state.ReviewOpen = false
		s.state.ReviewMerges = nil
		s.state.ReviewRenames = nil
		s.state.ReviewSelected = map[string]bool{}
		s.state.ReviewApplied = s.t().Tf(i18n.ReviewAppliedCount, outcome.count)
	})

	if !started {
		// A write is already running; the panel stays open so the reader can press
		// again once it finishes rather than believing the plan was applied.
		s.state.ReviewApplying = false
	}
}

// vocabularyReviewPanel draws the review.

// tagName resolves a tag id to its name from the loaded library.
func (s *Shell) tagName(tagID string) string {
	for _, item := range s.state.TagItems {
		if item.TagID == tagID {
			return item.Name
		}
	}
	return ""
}

// CreateTagFromDraft creates the tag the open create row names.
//
// Creating a tag that already exists reuses it rather than failing: the service is
// find-or-create, which is what makes Enter safe to press twice.
//
// An empty draft closes the row instead of doing nothing. Enter on an empty field resets the
// draft row, and an Enter that neither creates nor dismisses reads as a broken field rather
// than as an empty one.
func (s *Shell) CreateTagFromDraft(ctx context.Context) {
	name := strings.TrimSpace(s.state.TagDraft)
	if name == "" {
		s.resetTagDraft()
		if s.window != nil {
			s.window.Invalidate()
		}
		return
	}
	if s.service == nil {
		return
	}

	tag, err := s.service.CreateTag(ctx, name)
	if err != nil {
		// The row and its text stay: a refused name is one the user has to change, and
		// emptying the field would make them type it again to find that out.
		s.reportErr(err)
		return
	}
	// Typing the name back out answers the question the block was holding: the block clears
	// at the same point.
	if s.tagging != nil {
		_ = s.tagging.UnblockTagKey(ctx, name)
	}

	s.state.Err = ""
	s.resetTagDraft()
	// The new tag becomes the focus, so the library shows what was just made.
	s.state.TagFocus = tag.ID
	s.ReloadTagLibraryAsync(ctx)
	s.ReloadAsync(ctx)
}

// resetTagDraft closes either create row and forgets what was typed.
func (s *Shell) resetTagDraft() {
	s.state.CreatingTag = false
	s.state.TagLibraryCreateOpen = false
	s.state.TagDraft = ""
}

// AttachTagFromDetail creates or finds the tag the inspector's field names and files it
// under the bookmark on screen.
//
// The field submits on Enter. MyGo reports no blur, so Enter is the commit here and Escape
// clears the field; a field that accepted text and did nothing with it would leave a bookmark
// untaggable from the inspector by any route.
func (s *Shell) AttachTagFromDetail(ctx context.Context) {
	name := strings.TrimSpace(s.state.DetailTagInput)
	if s.service == nil || s.state.Selected == "" || name == "" {
		return
	}

	// AddTags resolves the name to a tag and creates one that does not exist, then attaches it:
	// one write the service already offers.
	if _, err := s.service.AddTags(ctx, []string{s.state.Selected}, []string{name}); err != nil {
		s.reportErr(err)
		return
	}

	s.state.DetailTagInput = ""
	s.ReloadAsync(ctx)
	if s.window != nil {
		s.window.Invalidate()
	}
}

// BeginRenameList opens the inline rename for a list.

// CancelRenameList closes it without renaming.

// RenameListFromInput renames the list the inline row is for.
//
// The service refuses a name another list already holds, and refuses the Inbox, so those
// rule live there rather than being guessed at here.

// AskDeleteSelectedTags opens the confirmation for the selected tags.
func (s *Shell) AskDeleteSelectedTags() {
	if s.SelectedTagCount() == 0 {
		return
	}
	s.state.ConfirmDeleteTags = true
	if s.window != nil {
		s.window.Invalidate()
	}
}

// SelectedTagCount is how many tags the bulk deletion would remove.
func (s *Shell) SelectedTagCount() int {
	count := 0
	for _, selected := range s.state.TagSelectedIDs {
		if selected {
			count++
		}
	}
	return count
}

// filterTagConditions keeps the tag conditions whose value still exists, and reports whether
// it dropped any.
//
// A condition naming a tag that is gone can only narrow the list to nothing, and its chip can
// only show the stored uuid: filterCondition.Label is a name and never a value
// (filterfacets.go), while tagLabel falls back to the id precisely because there is nothing
// left to call it. So deleting a tag deletes the condition naming it, the same way deleting a
// row untickes it.
//
// The caller reloads afterwards: dropping the condition changes what the list should contain,
// and the page was showing the old answer.
func (s *Shell) filterTagConditions(exists func(id string) bool) bool {
	if len(s.state.ActiveTagIDs) == 0 {
		return false
	}
	kept := make([]string, 0, len(s.state.ActiveTagIDs))
	for _, id := range s.state.ActiveTagIDs {
		if exists(id) {
			kept = append(kept, id)
		}
	}
	if len(kept) == len(s.state.ActiveTagIDs) {
		return false
	}
	if len(kept) == 0 {
		kept = nil
	}
	s.state.ActiveTagIDs = kept
	return true
}

// ConfirmDeleteTagNow deletes one tag.
//
// It reloads the results as well as the tag library. Deleting used to refresh only the
// library, so the list behind it kept the deleted tag in its cards, in the sidebar and in the
// active-filter chip -- where, with the tag no longer named anywhere, the chip printed its
// uuid and the filter matched nothing. The reader saw an empty list with an internal id over
// it and no way to work out what had happened.
func (s *Shell) ConfirmDeleteTagNow(ctx context.Context) {
	tagID := s.state.ConfirmDeleteTag
	s.state.ConfirmDeleteTag = ""
	if s.service == nil || tagID == "" {
		return
	}

	// Deleting a tag is only half of it: the rule engine and the LLM step both work from
	// names, so a deleted name comes back the next time a post mentions it. Blocking the name
	// at the delete site is the third of the three steps.
	if name := s.tagName(tagID); name != "" && s.tagging != nil {
		_ = s.tagging.BlockTagKey(ctx, name)
	}
	if err := s.service.DeleteTag(ctx, tagID); err != nil {
		s.reportErr(err)
	} else {
		s.state.Err = ""
		if s.state.TagFocus == tagID {
			s.state.TagFocus = ""
			s.state.TagEditField = ""
		}
		s.filterTagConditions(func(id string) bool { return id != tagID })
		s.ReloadAsync(ctx)
	}
	// A deleted row must not stay ticked: the bulk bar counted it and Merge would name a tag
	// that no longer exists.
	delete(s.state.TagSelectedIDs, tagID)
	s.ReloadTagLibraryAsync(ctx)
}

// ConfirmDeleteSelectedTagsNow deletes every selected tag.
//
// One failure does not stop the rest: the bulk action is the same shape as the others, and
// stopping half way would leave the user unsure what happened. The failures are counted and
// reported.
func (s *Shell) ConfirmDeleteSelectedTagsNow(ctx context.Context) {
	s.state.ConfirmDeleteTags = false
	if s.service == nil {
		return
	}

	// Only the checked rows. TagSelectedIDs keeps an entry for a row that was checked and then
	// unchecked, so ranging over the map deleted tags the reader had just excluded.
	ids := make([]string, 0, len(s.state.TagSelectedIDs))
	for id, selected := range s.state.TagSelectedIDs {
		if selected {
			ids = append(ids, id)
		}
	}

	failed := 0
	gone := make(map[string]bool, len(ids))
	for _, id := range ids {
		// Same rule as the single delete: a deleted name must not come back from the rule
		// engine or the next LLM pass.
		if name := s.tagName(id); name != "" && s.tagging != nil {
			_ = s.tagging.BlockTagKey(ctx, name)
		}
		if err := s.service.DeleteTag(ctx, id); err != nil {
			failed++
			continue
		}
		gone[id] = true
	}

	if failed > 0 {
		s.ShowStatus(ToastError, s.t().Tf(i18n.TagDeleteFailed, failed, len(ids)))
	}
	s.state.TagSelectedIDs = map[string]bool{}
	// Same two steps as the single delete: a deleted tag cannot stay a condition, and the
	// list has to be read again to stop drawing it.
	s.filterTagConditions(func(id string) bool { return !gone[id] })
	s.ReloadAsync(ctx)
	s.ReloadTagLibraryAsync(ctx)
}

// tagWriteMessage is what a tag edit says when it fails.
//
// A refused rename is the one failure the reader can act on: another tag answers to the
// name. The use case reports it with a typed error whose Error() is an internal sentence
// ("workspace: cannot rename ...") in a window that speaks the reader's language, so the
// typed case is translated here and everything else keeps the existing sentence.
func (s *Shell) tagWriteMessage(err error) string {
	var taken *workspace.TagNameTakenError
	if errors.As(err, &taken) {
		return s.t().Tf(i18n.TagRenameTaken, taken.Wanted)
	}
	return err.Error()
}

// RenameFocusedTag renames the focused tag.
//
// A name another tag answers to is refused rather than merged: merging would absorb a second
// tag and delete it, and merging has its own action. There is no confirmation for the ordinary
// rename because nothing is destroyed -- only a collision is refused, and it says so.
func (s *Shell) RenameFocusedTag(ctx context.Context) {
	if s.service == nil || s.state.TagFocus == "" {
		return
	}
	name := strings.TrimSpace(s.state.TagNameEdit)
	if name == "" {
		return
	}

	if _, err := s.service.RenameTag(ctx, s.state.TagFocus, name); err != nil {
		s.state.TagActionMsg = s.tagWriteMessage(err)
		return
	}

	s.state.TagNameEdit = ""
	s.state.TagActionMsg = ""
	// The cards draw the tag by name, so a rename that only refreshed the library left the
	// old name on every card and in the active-filter chip.
	s.ReloadAsync(ctx)
	s.ReloadTagLibraryAsync(ctx)
}

// BeginRenameSidebarTag turns one sidebar row into an inline rename field.
//
// The row's label becomes an input in place, and the row keeps its icon, its count and its
// trailing buttons. Without this, a tag could be renamed only from the tag library, so the
// row's edit button had nowhere to go.
func (s *Shell) BeginRenameSidebarTag(tagID string) {
	s.state.RenameTagID = tagID
	s.state.RenameTagName = ""
	if s.window != nil {
		s.window.Invalidate()
	}
}

// CancelSidebarTagRename closes the inline row without renaming.
func (s *Shell) CancelSidebarTagRename() {
	s.state.RenameTagID = ""
	s.state.RenameTagName = ""
	if s.window != nil {
		s.window.Invalidate()
	}
}

// CommitSidebarTagRename renames the row being edited, or closes it when the draft is
// empty or unchanged.
//
// Through the same use case as the panel's rename, so both surfaces refuse the same names
// with the same sentence (tagWriteMessage).
func (s *Shell) CommitSidebarTagRename(ctx context.Context) {
	tagID := s.state.RenameTagID
	if s.service == nil || tagID == "" {
		return
	}
	name := strings.TrimSpace(s.state.RenameTagName)
	s.state.RenameTagID = ""
	s.state.RenameTagName = ""
	if name != "" {
		if _, err := s.service.RenameTag(ctx, tagID, name); err != nil {
			s.state.TagActionMsg = s.tagWriteMessage(err)
			return
		}
		s.state.TagActionMsg = ""
	}
	s.ReloadTagLibraryAsync(ctx)
	s.ReloadAsync(ctx)
}

// AskDeleteSidebarTag opens the delete confirmation for one row.
//
// A delete is a danger action, so the row's button asks rather than deleting; the dialog
// is the same one the tag library uses, so there is one confirmation in the app.
func (s *Shell) AskDeleteSidebarTag(tagID string) {
	if s.service == nil || tagID == "" {
		return
	}
	s.state.ConfirmDeleteTag = tagID
	if s.window != nil {
		s.window.Invalidate()
	}
}

// SetTagTheme nests the focused tag under the picked one, or clears its theme.
func (s *Shell) SetTagTheme(ctx context.Context) {
	if s.service == nil || s.state.TagFocus == "" {
		return
	}

	parentID := ""
	if s.state.TagParentPick != s.t().T(i18n.TagParentNone) {
		for _, item := range s.state.TagItems {
			if item.Name == s.state.TagParentPick {
				parentID = item.TagID
				break
			}
		}
	}

	if _, err := s.service.SetTagParent(ctx, s.state.TagFocus, parentID); err != nil {
		s.state.TagActionMsg = err.Error()
		return
	}
	s.state.TagActionMsg = ""
	s.ReloadTagLibraryAsync(ctx)
}

// SaveTagAliases writes the comma-separated aliases for the focused tag.
func (s *Shell) SaveTagAliases(ctx context.Context) {
	if s.service == nil || s.state.TagFocus == "" {
		return
	}

	aliases := []string{}
	for _, alias := range strings.Split(s.state.TagAliasInput, ",") {
		if trimmed := strings.TrimSpace(alias); trimmed != "" {
			aliases = append(aliases, trimmed)
		}
	}

	if _, err := s.service.UpdateTagAliases(ctx, s.state.TagFocus, aliases); err != nil {
		s.state.TagActionMsg = err.Error()
		return
	}
	s.state.TagActionMsg = ""
	s.ReloadTagLibraryAsync(ctx)
}

// AskCleanupOrphanTags opens the confirmation for the orphan sweep.
//
// The sweep deletes tags, so it goes through the same fail-closed confirmation as the other
// danger actions rather than running on the click under the label "Clean up empty tags", a
// euphemism on a destructive action.
func (s *Shell) AskCleanupOrphanTags() {
	s.state.ConfirmCleanupTags = true
	if s.window != nil {
		s.window.Invalidate()
	}
}

// handleCleanupConfirmation draws the orphan-cleanup dialog.

// tagRow draws a tag with its own active test and its own click. It returns the row so the
// list can put its selection shape under the first active one.
//
// A tag is active when it is in the set, not when it is the selection: with several tags
// narrowing the results, the sidebar has to show all of them as active, and the
// selection only names the first.
func (s *Shell) tagRow(c *ui.Context, theme *ui.Theme, rail RailTokens, id, label string, count int) (ui.Element, bool) {
	tagID := strings.TrimPrefix(id, prefixTag)
	active := false
	for _, candidate := range s.state.ActiveTagIDs {
		if candidate == tagID {
			active = true
		}
	}

	// A tag is a grouped sub-row: `.nav-sub > button` is 26 high, `padding: 0 10px`, a 6
	// radius, a 6 gap and 12.5px text (app.css:440-449). It used to take the top-level row's
	// 30/9/7/13, which is the row a page gets, not the row a tag under it gets.
	row := ui.Row(c).FillWidth().Padding(0, SubRowPadX).Radius(SubRowRadius).
		GapX(NavRowGap).MinHeight(SubRowMinHeight).Border(1, ui.Transparent).Focusable()
	row.Key(id).OnClick(func() { s.toggleTagFilter(context.Background(), tagID) }).
		ContextMenu(func(m *ui.Menu) { s.tagContextMenu(m, tagID, label) })
	// No ground of its own in any state: the list's selection shape is the list's only
	// shape, and the pointer's answer is ink (navhighlight.go, app.css:449-453).
	ink := subRowInk(c, row, theme, active)
	row.Children(func() {
		// The hash and the name, 6 apart, at 12.5px. The open tag is the page's ink at
		// weight 500 -- no accent in the list (app.css:449).
		renaming := s.state.RenameTagID == tagID
		ui.Row(c).Grow(1).GapX(SubRowGap).Children(func() {
			ui.Box(c).Width(NavRowIcon).Height(NavRowIcon).
				Draw(func(p *ui.Painter, r ui.Rect) {
					drawIcon(p, "hash", r.X, r.Y, min32(r.W, r.H), ink)
				})
			if renaming {
				// The field replaces the label in place; Enter submits and Escape abandons.
				field := ui.TextInput(c, &s.state.RenameTagName).
					Placeholder(label).FontSize(SubRowFont).Grow(1)
				if field.Submitted() {
					s.CommitSidebarTagRename(context.Background())
				}
				if field.Shortcut(0, ui.KeyEscape) {
					s.CancelSidebarTagRename()
				}
				return
			}
			// magpie's grouped row is `white-space: nowrap` with `overflow: hidden;
			// text-overflow: ellipsis` on the name (app.css:444-445). Without SingleLine the
			// name took a second line and pushed the count up beside its first half; Shrink
			// and MinWidth(0) are what let it give way to the count instead of the row.
			//
			// The whole name is in the tooltip as well: a cut name is the one thing the row
			// cannot say, and the tag library's table already answers it the same way
			// (taglibrary.go:757).
			text := ui.Text(c, label).FontSize(SubRowFont).TextColor(ink).
				SingleLine().Shrink(1).MinWidth(0).Tooltip(label)
			if active {
				text = text.FontWeight(500)
			}
		})
		// The count is the row's last thing and its right edge is the row's, as magpie's
		// `em { margin-left: auto }` puts it (app.css:446).
		//
		// The two row actions come after the count in a trailing cell that keeps their
		// 22-point boxes only while the row is under the pointer. At a 280-point column that
		// reserved band was a fifth of the row; at magpie's 212 it was a third, so names
		// were cut -- "ai image genera…" -- while the space beside them sat empty, and the
		// count stopped 56 points short of the edge the eye reads as the column's. Laid out
		// only while hovered, the space goes back to the name at rest, and nothing moves but
		// the cut, which is already the name's own answer.
		hovered := row.Hovered()
		ui.Row(c).GapX(NavRowTrailingGap).Children(func() {
			if hovered {
				action := rail.TextQuaternary
				for _, item := range []struct {
					icon, label string
					run         func()
				}{
					{"edit", s.t().Tf(i18n.TagRenameLabel, label), func() { s.BeginRenameSidebarTag(tagID) }},
					{"trash", s.t().Tf(i18n.ActionDelete, label), func() { s.AskDeleteSidebarTag(tagID) }},
				} {
					item := item
					ui.Box(c).Width(NavRowAction).Height(NavRowAction).Radius(RadiusPill).
						Tooltip(item.label).Label(item.label).Focusable().OnClick(item.run).
						Draw(func(p *ui.Painter, r ui.Rect) {
							drawIcon(p, item.icon, r.X+(r.W-12)/2, r.Y+(r.H-12)/2, 12, action)
						})
				}
			}
			// A zero is a number worth showing: a tag with nothing filed under it still says
			// 0, and the reference's 设计 reads 0. Drawing the count only above zero would
			// lose the row's trailing cell.
			ui.Text(c, domain.FormatCompactCount(count, s.translator.Locale())).FontSize(SubRowCountFont).
				TextColor(rail.TextTertiary)
		})
	})
	return row, active
}

// ToggleFilterPopover shows or hides the filter popover.
