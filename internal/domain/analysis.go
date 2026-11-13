package domain

import (
	"strconv"
	"unicode/utf16"
)

// AnalysisOutcome is why a bookmark's LLM analysis ended the way it did.
//
// All four values are persisted: a bookmark the model deliberately skipped still
// needs a ledger row, otherwise it is re-sent on every later run.
type AnalysisOutcome = string

const (
	OutcomeTagged        AnalysisOutcome = "tagged"
	OutcomeNoTopic       AnalysisOutcome = "no-topic"
	OutcomeAllBlocked    AnalysisOutcome = "all-blocked"
	OutcomeBudgetDropped AnalysisOutcome = "budget-dropped"
)

// LlmPromptVersion must be bumped whenever the prompt changes in a way that
// could change a verdict, so answers given to the old question are re-asked. It is
// persisted in the ledger.
const LlmPromptVersion = 1

// LlmAnalysis is the ledger row recording that the model has judged a bookmark.
//
// PromptVersion, Model and ContentHash are the invalidation inputs: a verdict is
// reusable only while all three still match what produced it. Dropping any one
// of them re-analyses bookmarks that did not need it (or, worse, skips ones that
// did).
type LlmAnalysis struct {
	BookmarkID    string
	AnalyzedAt    string
	PromptVersion int
	Model         string
	ContentHash   string
	Outcome       AnalysisOutcome
	TagCount      int
}

// IsAnalysisReusable reports whether a stored verdict is still usable: it is usable only
// while every input that produced it still matches. An empty model or hash on
// the record is treated as a mismatch, so a malformed row re-queues rather than
// silently pinning a stale answer.
func (a LlmAnalysis) IsAnalysisReusable(model, contentHash string, promptVersion int) bool {
	return a.PromptVersion == promptVersion &&
		a.Model != "" && a.Model == model &&
		a.ContentHash != "" && a.ContentHash == contentHash
}

// HashAnalyzedContent hashes the content the model saw.
//
// FNV-1a, 32-bit, rendered in base 36. The subtlety is the input: the hash covers
// **UTF-16 code units**, not UTF-8 bytes. Hashing bytes instead gives identical
// results for ASCII and different results for everything else, which would mark
// every CJK bookmark as "the text changed" and re-send all of them to the model on
// the first run over existing data — a real cost to the user.
func HashAnalyzedContent(text string) string {
	hash := uint32(0x811c9dc5)

	for _, unit := range utf16.Encode([]rune(text)) {
		hash ^= uint32(unit)
		// uint32 multiplication wraps at 32 bits, keeping the low 32 bits.
		hash *= 0x01000193
	}

	return strconv.FormatUint(uint64(hash), 36)
}
