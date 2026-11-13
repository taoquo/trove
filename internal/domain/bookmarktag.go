package domain

import "time"

// BookmarkTagSource records what created a bookmark-to-tag relation.
//
// Provenance is recorded explicitly rather than implied by how a relation was
// reached. That is what makes "re-running a rule only adds what it owns" an
// enforceable rule rather than a convention, and it is what lets a manual tag
// survive any automatic pass.
type BookmarkTagSource = string

const (
	// TagSourceManual is a tag the user applied by hand. No automatic pass may
	// ever remove one.
	TagSourceManual BookmarkTagSource = "manual"
	// TagSourceSuggestion came from accepting an auto-tag suggestion.
	TagSourceSuggestion BookmarkTagSource = "suggestion"
	// TagSourceHeuristic came from the rule-based heuristics.
	TagSourceHeuristic BookmarkTagSource = "heuristic"
	// TagSourceRule came from a user-defined classification rule.
	TagSourceRule BookmarkTagSource = "rule"
)

// AllTagSources is every value the `source` column accepts, matching the CHECK
// constraint in the schema.
var AllTagSources = []BookmarkTagSource{
	TagSourceManual, TagSourceSuggestion, TagSourceHeuristic, TagSourceRule,
}

// ValidTagSource reports whether a stored source is known.
func ValidTagSource(source string) bool {
	for _, candidate := range AllTagSources {
		if source == candidate {
			return true
		}
	}
	return false
}

// BookmarkTag links a bookmark to a tag.
//
// The identity is the pair, not a surrogate id: a surrogate id with non-unique
// indexes would allow the same pair to be stored twice and make the sidebar's
// counts read high.
type BookmarkTag struct {
	BookmarkID string
	TagID      string
	CreatedAt  time.Time
	Source     BookmarkTagSource
}

// AutomaticSources lists the provenance values an automatic pass may add or replace.
//
// A pruner must filter on this set so a manual tag is never removed. That filter
// lives in the repository's SQL rather than in a per-row helper, which is why
// there is no IsManual method here: a helper nobody calls is a rule that looks
// enforced and is not.
func AutomaticSources() []BookmarkTagSource {
	return []BookmarkTagSource{TagSourceSuggestion, TagSourceHeuristic, TagSourceRule}
}
