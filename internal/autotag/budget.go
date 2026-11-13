package autotag

import (
	"math"
	"strings"

	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/llm"
)

// Vocabulary modes.
type TagVocabularyMode = string

const (
	// VocabularyClosed never invents a tag: only the existing library is used.
	VocabularyClosed TagVocabularyMode = "closed"
	// VocabularyGuarded may invent a few, within an explicit budget. This is the
	// default: a library that grows without limit stops being navigable.
	VocabularyGuarded TagVocabularyMode = "guarded"
	// VocabularyOpen may invent freely.
	VocabularyOpen TagVocabularyMode = "open"
)

// AllVocabularyModes is every mode, for validation.
var AllVocabularyModes = []TagVocabularyMode{VocabularyClosed, VocabularyGuarded, VocabularyOpen}

// ValidVocabularyMode reports whether a stored mode is known.
func ValidVocabularyMode(mode string) bool {
	for _, candidate := range AllVocabularyModes {
		if mode == candidate {
			return true
		}
	}
	return false
}

// Healthy library size. Past the maximum a library is considered too large to
// browse, which is why guarded mode stops adding tags there.
const (
	HealthyTagCountMin = 30
	HealthyTagCountMax = 50
)

// guardedBudgetDivisor turns a run's candidate count into a tag allowance.
//
// One new tag per this many candidates keeps growth proportional to the work
// actually done, rather than to the size of the library.
const guardedBudgetDivisor = 150

// ResolveNewTagBudget returns how many new tags a run may create.
//
// The guard rail matters more than the number: an unbounded run against a large
// library can invent hundreds of near-duplicate tags, and the user has no way to
// see it happening until the sidebar is unreadable.
func ResolveNewTagBudget(mode TagVocabularyMode, vocabularySize, eligibleCount, maxBookmarksPerRun int) float64 {
	switch mode {
	case VocabularyClosed:
		return 0

	case VocabularyOpen:
		return math.Inf(1)

	case VocabularyGuarded:
		headroom := HealthyTagCountMax - vocabularySize
		if headroom <= 0 {
			// The library is at its healthy maximum, so guarded behaves like
			// closed rather than silently continuing to grow.
			return 0
		}

		candidates := eligibleCount
		if candidates > maxBookmarksPerRun {
			candidates = maxBookmarksPerRun
		}
		if candidates < 0 {
			candidates = 0
		}

		proportional := int(math.Ceil(float64(candidates) / guardedBudgetDivisor))
		if proportional < 1 {
			proportional = 1
		}
		if proportional > headroom {
			return float64(headroom)
		}
		return float64(proportional)
	}

	// An unknown mode must not silently become unbounded.
	return 0
}

// AllowsNewTags reports whether a mode may invent a tag at all.
func AllowsNewTags(mode TagVocabularyMode) bool {
	return mode == VocabularyGuarded || mode == VocabularyOpen
}

// BlockedTagKeySet builds a lookup from the canonical keys the user blocked by
// deleting a tag.
//
// The keys are canonical, so "AI Agent" and "ai-agents" block the same concept.
// An empty key never blocks, which keeps a name that normalizes to nothing from
// suppressing anything.
func BlockedTagKeySet(keys []string) map[string]bool {
	set := make(map[string]bool, len(keys))
	for _, key := range keys {
		if normalised := domain.CanonicalTagKey(key); normalised != "" {
			set[normalised] = true
		}
	}
	return set
}

// IsBlocked reports whether a tag name is on the block list.
func IsBlocked(blocked map[string]bool, tagName string) bool {
	if len(blocked) == 0 {
		return false
	}
	key := domain.CanonicalTagKey(tagName)
	if key == "" {
		return false
	}
	return blocked[key]
}

// KeywordTagRule tags any post whose text contains a keyword.
//
// Matching is a case-insensitive substring with no word boundaries: adding boundaries
// would silently stop matching the language the user configured.
type KeywordTagRule struct {
	Keyword string
	TagName string
}

// HeuristicSettings configures the rule-based half of auto-tagging.
type HeuristicSettings struct {
	UseHashtags  bool
	UseAuthors   bool
	KeywordRules []KeywordTagRule
}

// DefaultHeuristicSettings is what a fresh install uses.
func DefaultHeuristicSettings() HeuristicSettings {
	return HeuristicSettings{UseHashtags: true, UseAuthors: true}
}

// NormalizeKeywordRules trims and drops incomplete rules: a rule missing either
// half could never fire.
func NormalizeKeywordRules(rules []KeywordTagRule) []KeywordTagRule {
	out := make([]KeywordTagRule, 0, len(rules))
	for _, rule := range rules {
		keyword := strings.TrimSpace(rule.Keyword)
		tagName := strings.TrimSpace(rule.TagName)
		if keyword == "" || tagName == "" {
			continue
		}
		out = append(out, KeywordTagRule{Keyword: keyword, TagName: tagName})
	}
	return out
}

// Confidence values. They are not tuned: changing them would change which
// suggestions a user sees for data they already have.
const (
	HashtagConfidence = 0.9
	KeywordConfidence = 0.7
	AuthorConfidence  = 0.6
)

// Thresholds for the heuristic rules.
const (
	// HashtagMinBookmarks is how often a hashtag must appear before it is worth
	// proposing as a tag.
	HashtagMinBookmarks = 3

	// AuthorMinTaggedBookmarks and AuthorMinTagShare decide when an author is
	// strongly associated with a tag.
	//
	// Both are needed: a share alone would propose a tag after one post, and a
	// count alone would propose it for an author who posts about everything.
	AuthorMinTaggedBookmarks = 3
	AuthorMinTagShare        = 0.6
)

// Ranges that numeric settings are clamped to.
const (
	MinBatchSize           = 1
	MaxBatchSize           = 50
	MinConcurrency         = 1
	MinMaxBookmarksPerRun  = 1
	MaxMaxBookmarksPerRun  = 2000
	DefaultSyncStrategy    = 1
	DefaultStopBufferPages = 3
)

// NormalizeSettings repairs a decoded settings blob.
//
// LoadSettings has two layers, and they answer different questions. A blob that will not
// decode at all falls back to the defaults, because the setting is recoverable and the
// library is not. A blob that decodes but holds one bad field is repaired here instead:
// without these normalizers a stored 99999 or an unknown vocabulary mode is accepted
// forever.
//
// Every rule below is applied in order:
//   - a number that is not finite, or is not positive, falls back
//   - then it is clamped to its range
//   - a keyword rule missing either half is dropped
//   - a vocabulary mode outside the known set falls back
//   - blocked keys are de-duplicated
func NormalizeSettings(settings Settings) Settings {
	settings.LLM.BatchSize = clampPositive(settings.LLM.BatchSize, DefaultBatchSize, MinBatchSize, MaxBatchSize)
	settings.LLM.Concurrency = clampPositive(settings.LLM.Concurrency, DefaultConcurrency, MinConcurrency, MaxConcurrency)
	settings.LLM.MaxPerRun = clampPositive(settings.LLM.MaxPerRun,
		DefaultMaxBookmarksPerRun, MinMaxBookmarksPerRun, MaxMaxBookmarksPerRun)
	settings.Heuristics.KeywordRules = NormalizeKeywordRules(settings.Heuristics.KeywordRules)
	if settings.LLM.Mode == "" || !ValidVocabularyMode(settings.LLM.Mode) {
		settings.LLM.Mode = VocabularyGuarded
	}
	settings.AutoTagContinueCount = max(settings.AutoTagContinueCount, 0)
	settings.BlockedTagKeys = dedupeKeys(settings.BlockedTagKeys)
	settings.LLM.CapabilityOverrides = normalizeCapabilities(settings.LLM.CapabilityOverrides)
	settings.LLM.LearnedCapabilities = normalizeLearnedCapabilities(settings.LLM.LearnedCapabilities)
	return settings
}

// normalizeCapabilities mirrors normalizeCapabilityOverrides.
//
// Only the four known fields survive, and only in the forms the type can hold: a strategy
// outside the known set is dropped, and a size that is not a positive number is dropped
// rather than defaulted, because "the user did not say" and "the user said zero" are
// different answers here. An entry with nothing left becomes nil.
func normalizeCapabilities(in *llm.ModelCapabilities) *llm.ModelCapabilities {
	if in == nil {
		return nil
	}
	out := llm.ModelCapabilities{}
	if in.JsonStrategy != "" && llm.IsKnownJsonStrategy(in.JsonStrategy) {
		out.JsonStrategy = in.JsonStrategy
	}
	out.Temperature = in.Temperature
	if in.MaxOutputTokens != nil && *in.MaxOutputTokens > 0 {
		value := *in.MaxOutputTokens
		out.MaxOutputTokens = &value
	}
	if in.ContextWindow != nil && *in.ContextWindow > 0 {
		value := *in.ContextWindow
		out.ContextWindow = &value
	}
	// An entry carrying only Temperature=false says nothing: false is the zero value and
	// indistinguishable from "unset". An entry that asked for a strategy or a size does.
	if out.JsonStrategy == "" && out.MaxOutputTokens == nil && out.ContextWindow == nil {
		return nil
	}
	return &out
}

// normalizeLearnedCapabilities mirrors normalizeLearnedCapabilities: each entry is
// normalized the same way, and an empty key or an entry with nothing left is dropped.
func normalizeLearnedCapabilities(in map[string]llm.ModelCapabilities) map[string]llm.ModelCapabilities {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]llm.ModelCapabilities, len(in))
	for key, value := range in {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" {
			continue
		}
		entry := value
		normalized := normalizeCapabilities(&entry)
		if normalized == nil {
			continue
		}
		out[trimmed] = *normalized
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// clampPositive mirrors normalizeRange: a value that is not positive falls back, and the
// result is clamped. Zero is not positive, so it falls back rather than clamping up.
func clampPositive(value, fallback, lowest, highest int) int {
	if value <= 0 {
		return fallback
	}
	return min(max(value, lowest), highest)
}

// dedupeKeys keeps the first of each key and drops the empties.
func dedupeKeys(keys []string) []string {
	seen := make(map[string]bool, len(keys))
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, key)
	}
	return out
}
