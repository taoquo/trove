package ui

import (
	"context"

	"github.com/taoquo/x-master/apps/macos/internal/domain"
	"github.com/taoquo/x-master/apps/macos/internal/i18n"
)

// AskMergeSelectedTags opens the merge confirmation.
//
// The source does not pick a target for the reader: the merge target starts empty, the button
// waits until one is chosen, and the merge asks first (TagResultsPane.tsx:920-944,
// OptionsApp.tsx:636-658). This bar used to fill the target with the first listed tag as soon
// as a row was ticked and merge on one press, so ticking rows and pressing Merge folded them
// into whichever tag sorted first -- irreversible, and never chosen (round 28).
//
// The two guards before the dialog are the source's two early returns: no target, and a
// selection that is only the target. The source returns silently on both; here the bar says
// which one is missing, because a press that does nothing is indistinguishable from a miss.
func (s *Shell) AskMergeSelectedTags() {
	if s.service == nil || s.TagSelectionCount() == 0 {
		return
	}
	target, found := s.mergeTargetTag()
	switch {
	case !found:
		s.state.TagActionMsg = s.t().T(i18n.TagMergeNeedTarget)
	case len(s.mergeSources(target.TagID)) == 0:
		s.state.TagActionMsg = s.t().T(i18n.TagMergeNeedsSource)
	default:
		s.state.ConfirmMergeTags = true
	}
	if s.window != nil {
		s.window.Invalidate()
	}
}

// mergeTargetTag is the tag the picker names, or nothing.
//
// Resolved by name over the library because that is what ui.Select carries, and only for a
// tag that still exists: deleting the chosen target must not leave the merge pointing at a
// name no row has.
func (s *Shell) mergeTargetTag() (domain.TagLibraryItem, bool) {
	if s.state.TagMergeTarget == "" {
		return domain.TagLibraryItem{}, false
	}
	for _, item := range s.state.TagItems {
		if item.Name == s.state.TagMergeTarget {
			return item, true
		}
	}
	return domain.TagLibraryItem{}, false
}

// mergeSources is what the merge folds in: every ticked tag except the target, which the
// source filters out too (OptionsApp.tsx:641).
func (s *Shell) mergeSources(targetID string) []string {
	sources := make([]string, 0, len(s.state.TagSelectedIDs))
	for id, selected := range s.state.TagSelectedIDs {
		if selected && id != targetID {
			sources = append(sources, id)
		}
	}
	return sources
}

// ConfirmMergeSelectedTagsNow runs the merge the dialog asked about.
//
// The selection is cleared only once the write has succeeded. Clearing it up front, which is
// what the pre-round-28 code did before starting the work, took the reader's ticks with it
// when the merge failed, and the rows they had picked had to be found again.
func (s *Shell) ConfirmMergeSelectedTagsNow(ctx context.Context) {
	s.state.ConfirmMergeTags = false
	if s.service == nil {
		return
	}
	target, found := s.mergeTargetTag()
	if !found {
		s.state.TagActionMsg = s.t().T(i18n.TagMergeNeedTarget)
		return
	}
	sources := s.mergeSources(target.TagID)
	if len(sources) == 0 {
		s.state.TagActionMsg = s.t().T(i18n.TagMergeNeedsSource)
		return
	}

	// Merging reads every tag relation and rewrites the matching ones, so it runs off
	// the window's thread like the file commands do (work.go).
	s.runLibraryWrite(i18n.TagMerging, func() libraryWrite {
		report, err := s.service.MergeTags(ctx, sources, target.TagID)
		if err != nil {
			return libraryWrite{err: err}
		}
		return libraryWrite{count: len(report.MergedTagIDs)}
	}, func(outcome libraryWrite) {
		s.state.TagActionMsg = s.t().Tf(i18n.TagMergeDone, outcome.count)
		s.state.TagSelectedIDs = map[string]bool{}
		s.state.TagMergeTarget = ""
		s.ReloadTagLibraryAsync(ctx)
	})
}
