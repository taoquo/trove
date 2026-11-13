package ui

import (
	"context"

	"github.com/taoquo/trove/internal/domain"
)

// RebuildGraph recomputes the relation graph from the current library.
//
// It is a separate read from Reload because the graph needs the whole library rather
// than a page of it, and building it on every frame would make the window crawl.
//
// Switching to the graph is a click, and this read walks every bookmark and every
// tag relation: at 50,000 bookmarks it is the longest single read the app does. It runs
// off the window's thread so the switch itself is instant and the graph fades in when
// it is built; GraphLoading is what the graph area says meanwhile.
func (s *Shell) RebuildGraph(ctx context.Context) {
	if s.service == nil {
		return
	}
	// A request that arrives while a build is running is not dropped: the author layer
	// can be toggled faster than the read, and the last toggle is the one that counts.
	if s.state.GraphLoading {
		s.state.GraphDirty = true
		return
	}
	includeAuthors := s.state.GraphIncludeAuthors

	s.state.GraphLoading = true
	s.invalidate()
	s.runInBackground(func() {
		input, err := s.service.LoadGraphInput(ctx)
		var model domain.GraphViewModel
		if err == nil {
			model = domain.BuildGraphViewModel(input, domain.GraphViewModelOptions{
				Scope:          domain.EmptyGraphScope(),
				IncludeAuthors: includeAuthors,
			})
		}

		s.publish(func() {
			s.state.GraphLoading = false
			if err != nil {
				s.state.GraphDirty = false
				s.reportErr(err)
				s.invalidate()
				return
			}
			s.state.GraphModel = model
			s.state.GraphLayout = layoutGraph(model)
			s.invalidate()
			if s.state.GraphDirty {
				s.state.GraphDirty = false
				s.RebuildGraph(ctx)
			}
		})
	})
}
