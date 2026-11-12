package ui

import (
	"context"
)

// OpenSuggestions loads the pending suggestions and shows the panel.
//
// It is exported because the menu bar opens it from outside this package.
func (s *Shell) OpenSuggestions(ctx context.Context) {
	s.openSuggestions(ctx)
	if s.window != nil {
		s.window.Invalidate()
	}
}

// openSuggestions loads the pending suggestions and shows the panel.
func (s *Shell) openSuggestions(ctx context.Context) {
	s.closePanels()
	s.state.SuggestionsOpen = true
	s.refreshSuggestions(ctx)
}

// closePanels closes every overlay, so only one is ever on screen.
func (s *Shell) closePanels() {
	s.state.SettingsOpen = false
	s.state.SuggestionsOpen = false
	s.state.ReviewOpen = false
	// The detail is an overlay too, so it takes part in the same exclusion. The source
	// states the reason: two live overlays leave two Escape handlers registered, and
	// whichever the framework sees first closes.
	s.state.DetailOpen = false
}

// refreshSuggestions re-reads the pending suggestions.
func (s *Shell) refreshSuggestions(ctx context.Context) {
	if s.suggestions == nil {
		return
	}

	rows, err := s.suggestions.Pending(ctx)
	if err != nil {
		s.reportErr(err)
		return
	}
	s.state.Suggestions = rows
}

// AcceptSuggestion applies one suggestion.
func (s *Shell) AcceptSuggestion(ctx context.Context, suggestionID string) error {
	if s.suggestions == nil {
		return nil
	}

	if _, err := s.suggestions.Accept(ctx, suggestionID); err != nil {
		s.reportErr(err)
		return err
	}

	s.refreshSuggestions(ctx)
	s.ReloadAsync(ctx)
	if s.window != nil {
		s.window.Invalidate()
	}
	return nil
}

// RejectSuggestion refuses one suggestion.
func (s *Shell) RejectSuggestion(ctx context.Context, suggestionID string) error {
	if s.suggestions == nil {
		return nil
	}

	if err := s.suggestions.Reject(ctx, suggestionID); err != nil {
		s.reportErr(err)
		return err
	}

	s.refreshSuggestions(ctx)
	s.ReloadAsync(ctx)
	if s.window != nil {
		s.window.Invalidate()
	}
	return nil
}

// suggestionsPanel draws the review list.
