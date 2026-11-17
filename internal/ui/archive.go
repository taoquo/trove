package ui

import (
	"context"

	"github.com/taoquo/trove/internal/i18n"
)

// SetArchived moves one bookmark into or out of the archive.
//
// The row leaves the list as the action runs -- archived bookmarks are hidden unless the
// archive is the view -- so the toast is the receipt, and its Undo is the way back from
// where the reader is looking. The archive row in the filter panel opens the other view.
func (s *Shell) SetArchived(ctx context.Context, tweetID string, archived bool) error {
	if tweetID == "" {
		return nil
	}
	return s.setArchivedMany(ctx, []string{tweetID}, archived)
}

// ArchiveSelectedCard archives the card the outline is on, or brings it back out when the
// archive is the view.
//
// It is the keyboard's form of the row menu's one archive item. An empty selection is a
// key press on no card, so it does nothing rather than choosing one for the reader.
func (s *Shell) ArchiveSelectedCard(ctx context.Context) {
	if s.state.Selected == "" {
		return
	}
	_ = s.SetArchived(ctx, s.state.Selected, !s.state.OnlyArchived)
}

// ArchiveSelection moves every selected bookmark into or out of the archive.
//
// The batch bar asks which direction from the view it is in, the same way the row menu
// does: the archive's own bar says "Move out of archive", and every other view says
// "Archive". Nothing here needs a confirmation -- archiving keeps the row, its tags and
// its place in the backup -- so the undo in the receipt is the whole safety net.
func (s *Shell) ArchiveSelection(ctx context.Context, archived bool) error {
	return s.setArchivedMany(ctx, s.BatchSelection(), archived)
}

// archiveFilterLabel is the archive row in the filter panel, with its count once the
// library has been read.
func (s *Shell) archiveFilterLabel() string {
	if !s.state.Loaded {
		return s.t().T(i18n.FilterArchived)
	}
	return s.t().Tf(i18n.FilterArchivedWithCount, s.state.Data.Counts.ArchivedCount)
}

// setArchivedMany writes one archive change and reports it.
func (s *Shell) setArchivedMany(ctx context.Context, ids []string, archived bool) error {
	if s.service == nil || len(ids) == 0 {
		return nil
	}
	if err := s.service.SetArchivedMany(ctx, ids, archived); err != nil {
		s.reportErr(err)
		return err
	}

	// The write is reversible, and the receipt is where the way back belongs while the
	// reader still knows what it would restore.
	message := s.t().T(i18n.ToastArchived)
	if !archived {
		message = s.t().T(i18n.ToastUnarchived)
	}
	if len(ids) > 1 {
		if archived {
			message = s.t().Tf(i18n.ToastArchivedMany, len(ids))
		} else {
			message = s.t().Tf(i18n.ToastUnarchivedMany, len(ids))
		}
	}
	undo := func() { _ = s.setArchivedMany(context.Background(), ids, !archived) }
	s.ShowToastAction(message, s.t().T(i18n.ActionUndo), undo)

	// The rows just left this view, so they are no longer part of what is selected here.
	// A background refresh of the same list would not prune them: the selection survives a
	// refresh by design, and this is a write, not a refresh.
	for _, id := range ids {
		delete(s.state.SelectedIDs, id)
		if s.state.Selected == id {
			s.state.Selected = ""
			s.state.DetailOpen = false
		}
	}

	s.ReloadAsync(ctx)
	if s.window != nil {
		s.window.Invalidate()
	}
	return nil
}
