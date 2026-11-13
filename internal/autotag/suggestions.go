package autotag

import (
	"errors"
	"time"

	"github.com/taoquo/trove/internal/domain"
)

// ErrNotPending reports an operation on a suggestion that already has an answer.
var ErrNotPending = errors.New("autotag: the suggestion is no longer pending")

// AcceptPlan is what accepting a suggestion must do, in the order it must happen.
//
// The order is the point. The tag and its relation are created first and the
// suggestion is resolved last, so a failure while creating the tag leaves the
// suggestion pending and retryable. Resolving first would lose the suggestion
// whenever the tag work failed.
type AcceptPlan struct {
	// Suggestion is the record to resolve, after the tag work succeeds.
	Suggestion domain.TagSuggestion
	// ExistingTag is set when a tag with this canonical key (or an alias of one)
	// already exists, which is the common case.
	ExistingTag *domain.Tag
	// NewTagName is the display name to create when ExistingTag is nil.
	NewTagName string
	// Relation is the relation to attach.
	Relation domain.BookmarkTag
}

// PlanAccept decides how to accept a suggestion.
//
// It reuses an existing tag rather than minting one, matching on the canonical key
// and then on aliases, so accepting "AI Agent" when "ai-agents" exists does not
// create a near-duplicate.
func PlanAccept(suggestion domain.TagSuggestion, tags []domain.Tag, now time.Time) (AcceptPlan, error) {
	if !suggestion.IsPending() {
		return AcceptPlan{}, ErrNotPending
	}

	plan := AcceptPlan{Suggestion: suggestion}

	if existing, found := domain.FindTagByName(tags, suggestion.TagName); found {
		match := existing
		plan.ExistingTag = &match
		plan.Relation = domain.BookmarkTag{
			BookmarkID: suggestion.BookmarkID,
			TagID:      existing.ID,
			CreatedAt:  now,
			Source:     domain.TagSourceSuggestion,
		}
	} else {
		plan.NewTagName = domain.NormalizeTagDisplayName(suggestion.TagName)
		plan.Relation = domain.BookmarkTag{
			BookmarkID: suggestion.BookmarkID,
			// The id is assigned when the tag is created; the caller fills it in.
			TagID:     "",
			CreatedAt: now,
			Source:    domain.TagSourceSuggestion,
		}
	}

	return plan, nil
}

// Resolve stamps an answer on a suggestion.
func Resolve(suggestion domain.TagSuggestion, status domain.SuggestionStatus, now time.Time) (domain.TagSuggestion, error) {
	if !suggestion.IsPending() {
		return suggestion, ErrNotPending
	}
	if status != domain.StatusAccepted && status != domain.StatusRejected {
		return suggestion, errors.New("autotag: a suggestion resolves to accepted or rejected")
	}

	resolved := suggestion
	resolved.Status = status
	resolved.ResolvedAt = &now
	return resolved, nil
}

// PlanReject records the user's refusal.
//
// The row is kept rather than deleted: it is the negative cache that stops the
// same tag being offered again for the same post.
func PlanReject(suggestion domain.TagSuggestion, now time.Time) (domain.TagSuggestion, error) {
	return Resolve(suggestion, domain.StatusRejected, now)
}

// RejectPendingByTagNames flips every pending suggestion for the given tag names.
//
// Deleting a tag must do this as well as dropping the tag, or accepting a leftover
// suggestion would recreate the tag the user just deleted.
func RejectPendingByTagNames(suggestions []domain.TagSuggestion, tagNames []string, now time.Time) []domain.TagSuggestion {
	if len(tagNames) == 0 {
		return nil
	}

	wanted := make(map[string]bool, len(tagNames))
	for _, name := range tagNames {
		if key := domain.CanonicalTagKey(name); key != "" {
			wanted[key] = true
		}
	}
	if len(wanted) == 0 {
		return nil
	}

	var changed []domain.TagSuggestion
	for _, suggestion := range suggestions {
		if !suggestion.IsPending() {
			continue
		}
		key := domain.CanonicalTagKey(suggestion.TagName)
		if !wanted[key] {
			continue
		}
		resolved, err := Resolve(suggestion, domain.StatusRejected, now)
		if err != nil {
			continue
		}
		changed = append(changed, resolved)
	}
	return changed
}

// DropPendingForBookmark is what removing a bookmark does to its suggestions.
//
// Only pending rows go. Accepted and rejected rows are the user's answer and must
// outlive the bookmark, or the same tag is offered again the moment it comes back.
func DropPendingForBookmark(suggestions []domain.TagSuggestion, bookmarkID string) []string {
	var doomed []string
	for _, suggestion := range suggestions {
		if suggestion.BookmarkID == bookmarkID && suggestion.IsPending() {
			doomed = append(doomed, suggestion.ID)
		}
	}
	return doomed
}

// PendingCount counts the pending suggestions, which is the sidebar badge.
func PendingCount(suggestions []domain.TagSuggestion) int {
	count := 0
	for _, suggestion := range suggestions {
		if suggestion.IsPending() {
			count++
		}
	}
	return count
}

// PassesBlockList reports whether a learned name may be suggested.
//
// A block list that silently starves both the rules and the model is invisible, so
// the count of blocked keys is surfaced in every run summary.
func PassesBlockList(blocked map[string]bool, tagName string) bool {
	return !IsBlocked(blocked, tagName)
}

// DedupeByID keeps the first occurrence of each suggestion id.
//
// Two rules can propose the same tag for the same post; the composite id makes
// them the same suggestion, and inserting both would violate the primary key.
func DedupeByID(suggestions []domain.TagSuggestion) []domain.TagSuggestion {
	seen := make(map[string]bool, len(suggestions))
	out := make([]domain.TagSuggestion, 0, len(suggestions))
	for _, suggestion := range suggestions {
		if seen[suggestion.ID] {
			continue
		}
		seen[suggestion.ID] = true
		out = append(out, suggestion)
	}
	return out
}
