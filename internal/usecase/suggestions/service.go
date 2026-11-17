// Package suggestions is the review flow for auto-tag suggestions.
//
// The order of operations here is the whole point: the tag and its relation are
// created first and the suggestion is resolved last. Resolving first would lose
// the suggestion whenever the tag work failed, and the user would have no way to
// try again.
package suggestions

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/ports"
)

// ErrNotFound reports a suggestion that does not exist.
var ErrNotFound = errors.New("suggestions: no such suggestion")

// ErrNotPending reports a suggestion that already has an answer.
var ErrNotPending = errors.New("suggestions: the suggestion is no longer pending")

// PendingRow is a suggestion together with the post it is about.
type PendingRow struct {
	Suggestion domain.TagSuggestion
	// Bookmark is nil when the suggestion outlived its bookmark, which is
	// expected: resolved rows are the negative cache and are kept on purpose.
	Bookmark *domain.Bookmark
}

// AcceptResult reports what accepting one suggestion did.
type AcceptResult struct {
	Suggestion domain.TagSuggestion
	Tag        domain.Tag
	// CreatedTag is true when the tag did not exist and was made.
	CreatedTag bool
}

// Service reviews suggestions.
type Service struct {
	repos ports.Repositories
	clock ports.Clock
	ids   ports.IDGenerator
}

// New builds a service.
func New(repos ports.Repositories, clock ports.Clock, ids ports.IDGenerator) *Service {
	if clock == nil {
		clock = ports.SystemClock{}
	}
	return &Service{repos: repos, clock: clock, ids: ids}
}

// Pending returns the suggestions awaiting a decision, with their posts.
func (s *Service) Pending(ctx context.Context) ([]PendingRow, error) {
	suggestions, err := s.repos.Suggestions.List(ctx, domain.StatusPending)
	if err != nil {
		return nil, err
	}
	if len(suggestions) == 0 {
		return nil, nil
	}

	ids := make([]string, 0, len(suggestions))
	seen := make(map[string]bool, len(suggestions))
	for _, suggestion := range suggestions {
		if !seen[suggestion.BookmarkID] {
			seen[suggestion.BookmarkID] = true
			ids = append(ids, suggestion.BookmarkID)
		}
	}

	// A suggestion whose bookmark is gone is still listed: dropping it silently
	// would hide a pending row the user cannot clear.
	//
	// One read for the whole page. This used to be a Get per suggestion, so a
	// backlog of a few thousand suggestions cost a few thousand statements before
	// the panel could draw.
	bookmarks, err := s.repos.Bookmarks.GetMany(ctx, ids)
	if err != nil {
		return nil, err
	}

	rows := make([]PendingRow, 0, len(suggestions))
	for _, suggestion := range suggestions {
		row := PendingRow{Suggestion: suggestion}
		if bookmark, found := bookmarks[suggestion.BookmarkID]; found {
			copied := bookmark
			row.Bookmark = &copied
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// PendingCount returns how many suggestions await a decision.
func (s *Service) PendingCount(ctx context.Context) (int, error) {
	return s.repos.Suggestions.PendingCount(ctx)
}

// Accept applies one suggestion.
//
// The tag is resolved or created first, the relation is attached second, and the
// suggestion is resolved last. Every step before the last is idempotent, so a
// failure leaves the suggestion pending and the user can simply try again.
func (s *Service) Accept(ctx context.Context, suggestionID string) (AcceptResult, error) {
	suggestion, err := s.find(ctx, suggestionID)
	if err != nil {
		return AcceptResult{}, err
	}
	if suggestion == nil {
		return AcceptResult{}, ErrNotFound
	}
	if !suggestion.IsPending() {
		return AcceptResult{}, ErrNotPending
	}

	now := s.clock.Now()
	tag, created, err := s.ensureTag(ctx, suggestion.TagName, now)
	if err != nil {
		return AcceptResult{}, err
	}

	if err := s.repos.Tags.Attach(ctx, domain.BookmarkTag{
		BookmarkID: suggestion.BookmarkID,
		TagID:      tag.ID,
		CreatedAt:  now,
		// The relation records where it came from, so a later automatic prune
		// knows this one was accepted by a person and leaves it alone.
		Source: domain.TagSourceSuggestion,
	}); err != nil {
		return AcceptResult{}, err
	}

	// Resolving is last, on purpose.
	if err := s.repos.Suggestions.Resolve(ctx, suggestion.ID, domain.StatusAccepted, now); err != nil {
		return AcceptResult{}, err
	}

	suggestion.Status = domain.StatusAccepted
	suggestion.ResolvedAt = &now

	return AcceptResult{Suggestion: *suggestion, Tag: tag, CreatedTag: created}, nil
}

// Reject records the user's refusal.
//
// The row is kept rather than deleted: it is the negative cache that stops the
// same tag being offered again for the same post.
func (s *Service) Reject(ctx context.Context, suggestionID string) error {
	suggestion, err := s.find(ctx, suggestionID)
	if err != nil {
		return err
	}
	if suggestion == nil {
		return ErrNotFound
	}
	if !suggestion.IsPending() {
		return ErrNotPending
	}
	return s.repos.Suggestions.Resolve(ctx, suggestion.ID, domain.StatusRejected, s.clock.Now())
}

// AcceptAll applies several suggestions, reporting what happened to each.
//
// One failure does not stop the rest: a batch review is where the user expects
// progress, and aborting on the first bad row would make them start over.
func (s *Service) AcceptAll(ctx context.Context, suggestionIDs []string) (accepts []AcceptResult, failures map[string]error) {
	failures = map[string]error{}
	for _, id := range suggestionIDs {
		result, err := s.Accept(ctx, id)
		if err != nil {
			failures[id] = err
			continue
		}
		accepts = append(accepts, result)
	}
	return accepts, failures
}

// RejectAll records refusals for several suggestions.
func (s *Service) RejectAll(ctx context.Context, suggestionIDs []string) (rejected int, failures map[string]error) {
	failures = map[string]error{}
	for _, id := range suggestionIDs {
		if err := s.Reject(ctx, id); err != nil {
			failures[id] = err
			continue
		}
		rejected++
	}
	return rejected, failures
}

// RejectByTagNames rejects every pending suggestion for a tag name.
//
// It is what deleting a tag must do as well as dropping the tag, or accepting a
// leftover suggestion would recreate the tag the user just deleted.
func (s *Service) RejectByTagNames(ctx context.Context, names []string) (int, error) {
	return s.repos.Suggestions.RejectByTagNames(ctx, names, s.clock.Now())
}

// find loads a suggestion by id, whatever its status.
//
// Looking across every status is what lets the caller tell "no such suggestion"
// from "you already answered this"; the second is what a double tap produces, and
// reporting it as missing would be wrong.
func (s *Service) find(ctx context.Context, suggestionID string) (*domain.TagSuggestion, error) {
	if strings.TrimSpace(suggestionID) == "" {
		return nil, nil
	}

	suggestion, found, err := s.repos.Suggestions.Get(ctx, suggestionID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	return &suggestion, nil
}

// ensureTag resolves a tag by name, creating it when it does not exist.
//
// Resolution goes through the canonical key and the alias list, so accepting
// "AI Agent" when "ai-agents" exists attaches to the existing tag instead of
// minting a near-duplicate of it.
func (s *Service) ensureTag(ctx context.Context, name string, now time.Time) (domain.Tag, bool, error) {
	display := domain.NormalizeTagDisplayName(name)
	key := domain.CanonicalTagKey(display)
	if key == "" {
		// A name that normalizes away would create an unnamed tag, which the tag
		// list cannot render or let the user remove.
		return domain.Tag{}, false, errors.New("suggestions: the suggested name is empty")
	}

	if existing, found, err := s.repos.Tags.FindByName(ctx, display); err != nil {
		return domain.Tag{}, false, err
	} else if found {
		return existing, false, nil
	}

	tag := domain.Tag{
		ID:           s.newID("tag"),
		Name:         display,
		CanonicalKey: key,
		CreatedAt:    domain.FormatTimestamp(now),
		UpdatedAt:    domain.FormatTimestamp(now),
	}
	if err := s.repos.Tags.Create(ctx, tag); err != nil {
		return domain.Tag{}, false, err
	}
	return tag, true, nil
}

// newID mints an identifier, falling back to a timestamp when no generator is
// configured so a tag is never created with an empty id.
func (s *Service) newID(prefix string) string {
	if s.ids == nil {
		return fmt.Sprintf("%s-%d", prefix, s.clock.Now().UnixNano())
	}
	return s.ids.NewID()
}
