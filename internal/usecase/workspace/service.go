// Package workspace is the read model the three-column UI renders.
//
// It exists so the UI never assembles a view out of several stores itself: the
// sidebar counts, the list of results and the per-bookmark tags and list all have
// to agree with each other, and the only way to guarantee that is to build them
// from one snapshot.
package workspace

import (
	"errors"
	"fmt"
	"github.com/taoquo/trove/internal/ports"
	"sync"
	"sync/atomic"
	"time"
)

// Service builds views and applies user edits.
type Service struct {
	repos ports.Repositories
	clock ports.Clock
	ids   ports.IDGenerator

	// generation is monotonic, so the UI can discard a stale result.
	generation atomic.Uint64
	// mu serialises writes. Two edits to the same bookmark must not interleave.
	mu sync.Mutex
	// sidebarMu guards the sidebar cache; a read and a filter change can overlap.
	sidebarMu sync.Mutex
	sidebar   sidebarCache
}

// New builds a service.
func New(repos ports.Repositories, clock ports.Clock, ids ports.IDGenerator) *Service {
	if clock == nil {
		clock = ports.SystemClock{}
	}
	return &Service{repos: repos, clock: clock, ids: ids}
}

// IsCurrent reports whether a view is still the newest one.
//
// The UI calls this before rendering: a query that started before the user's last
// keystroke must not replace the results of one that started after.
func (s *Service) IsCurrent(generation uint64) bool {
	return generation == s.generation.Load()
}

// ErrTooManyToSelect is returned when a match set is larger than one query can carry.
var ErrTooManyToSelect = errors.New("workspace: too many bookmarks to select at once")

// ErrUnchanged reports an edit that would change nothing.
var ErrUnchanged = errors.New("workspace: nothing to change")

// bumpGeneration marks every previously built view stale.
func (s *Service) bumpGeneration() error {
	s.generation.Add(1)
	return nil
}

// newID mints an identifier, falling back to a timestamp when no generator is
// configured so a caller cannot accidentally write empty ids.
func (s *Service) newID(prefix string) string {
	if s.ids == nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}
	return s.ids.NewID()
}
