package workspace

import (
	"context"
	"github.com/taoquo/trove/internal/domain"
	"sort"
)

// SavedViews lists the stored views, newest first.
func (s *Service) SavedViews(ctx context.Context) ([]domain.SavedView, error) {
	if s.repos.SavedViews == nil {
		return nil, nil
	}
	views, err := s.repos.SavedViews.List(ctx)
	if err != nil {
		return nil, err
	}
	// Newest first, which is the order the sidebar shows.
	sort.SliceStable(views, func(i, j int) bool { return views[i].CreatedAt.After(views[j].CreatedAt) })
	return views, nil
}

// AddSavedView stores a view, keeping the newest first.
//
// The whole list is written on every change, so the ordering is part of the operation
// rather than something the reader has to impose.
func (s *Service) AddSavedView(ctx context.Context, view domain.SavedView) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.repos.SavedViews == nil {
		return nil
	}
	return s.repos.SavedViews.Upsert(ctx, view)
}

// RemoveSavedView deletes a view.
func (s *Service) RemoveSavedView(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.repos.SavedViews == nil {
		return nil
	}
	return s.repos.SavedViews.Delete(ctx, id)
}

// NewSavedView builds a view from the current filters.
//
// The id is generated here because two views made in the same second must not collide.
func (s *Service) NewSavedView(name string, snapshot domain.SavedView) domain.SavedView {
	now := s.clock.Now()
	snapshot.ID = s.newID("view")
	snapshot.Name = name
	snapshot.CreatedAt = now
	snapshot.UpdatedAt = now
	return snapshot
}
