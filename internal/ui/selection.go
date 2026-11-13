package ui

import (
	"context"
	"sort"
	"strings"

	"github.com/egoist/mygo"
	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/i18n"
	"github.com/taoquo/trove/internal/usecase/settings"
	"github.com/taoquo/trove/internal/usecase/suggestions"
)

// sortKeys maps each order to its message key. The five orders are all reachable here.
//
// sortLabelKeys maps an order to its label.
//
// Two orders share the "recently saved" copy: `timeline` and `saved-desc`. That is
// intended, not an oversight -- the switch falls through to the same copy.
var sortLabelKeys = map[domain.SortOrder]i18n.Key{
	domain.SortTimeline:    i18n.SortTimeline,
	domain.SortSavedAsc:    i18n.SortSavedAsc,
	domain.SortCreatedDesc: i18n.SortCreatedDesc,
	domain.SortLikesDesc:   i18n.SortLikesDesc,
	domain.SortSavedDesc:   i18n.SortTimeline,
}

var sortKeys = []i18n.Key{
	i18n.SortTimeline, i18n.SortSavedDesc, i18n.SortSavedAsc,
	i18n.SortCreatedDesc, i18n.SortLikesDesc,
}

// sortLabels renders the sort control's labels in the current locale.
func (s *Shell) sortLabels() []string {
	labels := make([]string, 0, len(sortKeys))
	for _, key := range sortKeys {
		labels = append(labels, s.t().T(key))
	}
	return labels
}

// Attach records the window, which mutations need in order to repaint.
func (s *Shell) Attach(window *mygo.Window) { s.window = window }

// UseSettings attaches the preference store, so changes persist.
func (s *Shell) UseSettings(service *settings.Service) { s.settings = service }

// UseSuggestions attaches the suggestion review service.
func (s *Shell) UseSuggestions(service *suggestions.Service) { s.suggestions = service }

// BundleRequest is a file the user chose and what to do with it.

// batchTaggedLabel is the bulk tag write's result sentence. One bookmark gets the singular
// copy: "Tagged 1 bookmarks" is the kind of line a reader notices more than the result.
func (s *Shell) batchTaggedLabel(n int) string {
	if n == 1 {
		return s.t().T(i18n.BatchTaggedOne)
	}
	return s.t().Tf(i18n.BatchTagged, n)
}

// batchMovedLabel is the move's sentence: the count and the destination list.
func (s *Shell) batchMovedLabel(n int, list string) string {
	if n == 1 {
		return s.t().Tf(i18n.BatchMovedOne, list)
	}
	return s.t().Tf(i18n.BatchMoved, n, list)
}

// batchChooseTagLabel and batchChooseListLabel are the two pickers' resting text.
//
// The bulk tag picker rests on "Choose tag" as its empty value, so the trigger says what it
// wants before a choice is made. The list picker follows the same rule.
func (s *Shell) batchChooseTagLabel() string  { return s.t().T(i18n.BatchChooseTag) }
func (s *Shell) batchChooseListLabel() string { return s.t().T(i18n.BatchChooseList) }

// batchTagChoice and batchListChoice give the chosen value, or "" while a picker still shows
// its placeholder. An unchosen picker must act like no choice at all: Apply does nothing
// until a tag is picked.
func (s *Shell) batchTagChoice() string {
	if s.state.BulkTagLabel == s.batchChooseTagLabel() {
		return ""
	}
	return s.state.BulkTagLabel
}

func (s *Shell) batchListChoice() string {
	if s.state.MoveTargetLabel == s.batchChooseListLabel() {
		return ""
	}
	return s.state.MoveTargetLabel
}

// syncBatchLabels re-derives the pickers' resting text after a language change.
//
// The bound values are translated strings, not keys -- the same class as the settings
// selectors, and they have the same failure: a language switch leaves the old language's
// placeholder in the trigger. A real choice is left alone, which is
// why the current value is compared against both catalogs' placeholders rather than reset.
func (s *Shell) syncBatchLabels() {
	s.state.BulkTagLabel = s.relabelBatchChoice(s.state.BulkTagLabel, i18n.BatchChooseTag)
	s.state.MoveTargetLabel = s.relabelBatchChoice(s.state.MoveTargetLabel, i18n.BatchChooseList)
}

func (s *Shell) relabelBatchChoice(current string, key i18n.Key) string {
	if current == "" ||
		current == i18n.New("en", "en").T(key) ||
		current == i18n.New("zh-CN", "zh-CN").T(key) {
		return s.t().T(key)
	}
	return current
}

// BatchSelection returns the ids a bulk action would run on.
//
// It returns a copy, so a caller cannot mutate the selection it was handed.
func (s *Shell) BatchSelection() []string {
	ids := make([]string, 0, len(s.state.SelectedIDs))
	for id := range s.state.SelectedIDs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// toggleSelection adds or removes one bookmark from the batch selection.
//
// The box in a card's corner is the only caller besides the batch tests. It deliberately does
// not move Selected, so ticking a box for a bulk action never opens a detail and never moves
// the card the outline follows.
func (s *Shell) toggleSelection(tweetID string) {
	if s.state.SelectedIDs == nil {
		s.state.SelectedIDs = map[string]bool{}
	}
	if s.state.SelectedIDs[tweetID] {
		delete(s.state.SelectedIDs, tweetID)
	} else {
		s.state.SelectedIDs[tweetID] = true
	}
}

// clearSelection empties the batch selection.
func (s *Shell) clearSelection() { s.state.SelectedIDs = map[string]bool{} }

// AddTagsToSelection applies the typed tag names to the batch selection.
//
// It returns how many relations were written, so a caller can report the result
// rather than assume it worked.
func (s *Shell) AddTagsToSelection(ctx context.Context) (int, error) {
	ids := s.BatchSelection()
	if len(ids) == 0 || s.service == nil {
		return 0, nil
	}

	names := splitTagNames(s.state.TagInput)
	if len(names) == 0 {
		return 0, nil
	}

	added, err := s.service.AddTags(ctx, ids, names)
	if err != nil {
		s.reportErr(err)
		return 0, err
	}

	s.state.TagInput = ""
	// The bar is about to disappear with the selection, so the sentence is the receipt.
	s.ShowToast(s.batchTaggedLabel(len(ids)))
	s.clearSelection()
	s.ReloadAsync(ctx)
	if s.window != nil {
		s.window.Invalidate()
	}
	return added, nil
}

// MoveSelectionToList moves the batch selection into a list.
func (s *Shell) MoveSelectionToList(ctx context.Context) error {
	ids := s.BatchSelection()
	if len(ids) == 0 || s.service == nil {
		return nil
	}

	// No choice is not a choice: the trigger still reads "Choose list", and the first list is
	// not a safe guess at what the reader meant.
	target := s.batchListChoice()
	if target == "" {
		return nil
	}
	listID := s.listIDForLabel(target)
	if listID == "" {
		return nil
	}

	if err := s.service.MoveToList(ctx, ids, listID); err != nil {
		s.reportErr(err)
		return err
	}

	// A card does not show which list it belongs to, so on the all-bookmarks scope the
	// screen is unchanged by a successful move. The sentence carries the destination.
	s.ShowToast(s.batchMovedLabel(len(ids), target))
	s.clearSelection()
	s.ReloadAsync(ctx)
	if s.window != nil {
		s.window.Invalidate()
	}
	return nil
}

// RemoveTagFromSelection removes one tag from every selected bookmark.
//
// The tag ids are taken from the selection's shared tags: the button only ever
// offers a tag that some selected bookmark actually carries.
func (s *Shell) RemoveTagFromSelection(ctx context.Context, tagID string) error {
	ids := s.BatchSelection()
	if len(ids) == 0 || s.service == nil || tagID == "" {
		return nil
	}

	if _, err := s.service.RemoveTags(ctx, ids, []string{tagID}); err != nil {
		s.reportErr(err)
		return err
	}

	s.ReloadAsync(ctx)
	if s.window != nil {
		s.window.Invalidate()
	}
	return nil
}

// listLabels renders the destination list choices, Inbox first.
func (s *Shell) listLabels() []string {
	labels := make([]string, 0, len(s.state.Data.Lists))
	for _, list := range s.state.Data.Lists {
		labels = append(labels, list.Name)
	}
	return labels
}

// listIDForLabel maps a selector label back to a list id.
//
// It looks the label up among the lists currently rendered, so a rename cannot
// leave the selector pointing at a list that no longer has that name.
func (s *Shell) listIDForLabel(label string) string {
	for _, list := range s.state.Data.Lists {
		if list.Name == label {
			return list.ID
		}
	}
	// No fallback to the first list. It used to move a selection into Inbox whenever the label
	// matched nothing -- including the empty label the picker started with -- and the move's
	// receipt then named a destination the reader never chose.
	return ""
}

// splitTagNames splits a comma-separated tag field.
func splitTagNames(input string) []string {
	var names []string
	for _, part := range strings.Split(input, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			names = append(names, trimmed)
		}
	}
	return names
}

// SetLocale changes the language and rebuilds the translator.
//
// It is the only way the language changes: a view body may be rebuilt several
// times per event, so resolving the locale there would be a side effect.

// AskDeleteBookmarks arms the confirmation for the given bookmarks.
//
// The ids are stored with the confirmation, so the deletion runs on exactly the
// set the user was shown even if the list reloads in between.
func (s *Shell) AskDeleteBookmarks(tweetIDs []string) {
	if len(tweetIDs) == 0 {
		return
	}
	s.deleteBookmarks.Arm(tweetIDs,
		s.t().T(i18n.ConfirmDeleteBookmarksTitle),
		s.t().Tf(i18n.ConfirmDeleteBookmarksBody, len(tweetIDs)),
		s.t().T(i18n.ActionDelete))
}

// applyDeleteBookmarks runs the deletion, but only if it was approved.
//
// The confirmation is cleared before the work starts, which makes this idempotent:
// a view that rebuilds while the answer is being handled finds nothing armed and
// does nothing, so a delete can never run twice.
func (s *Shell) applyDeleteBookmarks(ctx context.Context) {
	tweetIDs, approved := s.deleteBookmarks.Approved()
	s.deleteBookmarks.Clear()

	if !approved {
		// Denied, dismissed or never asked: the library is left untouched.
		return
	}
	if s.service == nil {
		return
	}

	if _, err := s.service.DeleteBookmarks(ctx, tweetIDs); err != nil {
		s.reportErr(err)
		return
	}
	s.state.Selected = ""
	s.ReloadAsync(ctx)
	if s.window != nil {
		s.window.Invalidate()
	}
}

// appearanceLabels renders the three theme choices.
