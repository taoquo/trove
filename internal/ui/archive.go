package ui

import (
	"context"

	"github.com/taoquo/trove/internal/i18n"
)

// SetArchived moves one bookmark into or out of the archive.
//
// The row leaves the list as the action runs -- archived bookmarks are hidden unless the
// archive is the view -- so the toast is the receipt, and the archive row in the filter
// panel is where they went. There is no confirmation: archiving keeps the bookmark, its
// tags and its place in the backup, and the archive's own row menu is the way back.
func (s *Shell) SetArchived(ctx context.Context, tweetID string, archived bool) error {
	if s.service == nil || tweetID == "" {
		return nil
	}
	if err := s.service.SetArchived(ctx, tweetID, archived); err != nil {
		s.reportErr(err)
		return err
	}
	// The toast names where the bookmark went; in the archive, the menu's way back out is
	// the second message.
	key := i18n.ToastArchived
	if !archived {
		key = i18n.ToastUnarchived
	}
	s.ShowToast(s.t().T(key))
	s.ReloadAsync(ctx)
	if s.window != nil {
		s.window.Invalidate()
	}
	return nil
}
