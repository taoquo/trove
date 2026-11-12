package ui

import (
	"context"
	"strings"

	"github.com/taoquo/trove/apps/macos/internal/domain"
)

// toggleAuthorFilter adds or removes an author from the narrowing set.
//
// The source toggles membership so several authors can narrow at once. This shell holds one
// author: a second click replaces rather than unions, and clicking the active row clears it,
// so the row always says exactly what the list is showing.
func (s *Shell) toggleAuthorFilter(ctx context.Context, handle string) {
	if handle == "" {
		return
	}
	s.leaveSettings()

	if s.authorActive(handle) {
		s.state.ActiveAuthors = nil
	} else {
		s.state.ActiveAuthors = []string{handle}
	}
	s.markPageChange()
	s.ReloadAsync(ctx)
}

// visibleAuthors are the authors matching the section's search field.
//
// The extension searches the list by handle and by display name, so typing either finds
// the author. The order is the service's and is never rearranged for the selection: a
// chosen author stays on the row it was clicked on.
func (s *Shell) visibleAuthors() []domain.AuthorSummary {
	query := strings.ToLower(strings.TrimSpace(s.state.AuthorQuery))
	if query == "" {
		return s.state.Data.Authors
	}

	matching := make([]domain.AuthorSummary, 0, len(s.state.Data.Authors))
	for _, author := range s.state.Data.Authors {
		if strings.Contains(strings.ToLower(author.AuthorHandle), query) ||
			strings.Contains(strings.ToLower(author.AuthorName), query) {
			matching = append(matching, author)
		}
	}
	return matching
}
