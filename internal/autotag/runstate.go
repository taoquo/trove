// Package autotag holds the automatic-tagging rules: the heuristics, the
// vocabulary budget, the suggestion lifecycle and the run-state protocol.
//
// The run-state protocol is the part worth being careful with. A run that dies
// without reporting leaves the UI claiming a run is in flight forever, and a
// superseded run can clobber the state of the run that replaced it.
package autotag

import (
	"fmt"
	"math"
	"time"
)

// Timing constants.
//
// There are deliberately two windows, and the second is shorter than the first:
//
//   - RunStaleAfter is what the UI uses to stop claiming a run is in flight.
//   - RunLivenessAfter is what decides whether a new run may take over.
//
// Using one window for both would let a run that is dead enough to replace refuse
// replacement for the full, longer window.
const (
	RunStaleAfter    = 5 * time.Minute
	RunLivenessAfter = 2*time.Minute + 30*time.Second
)

// RunStatus is what a run is doing right now.
type RunStatus = string

const (
	// StatusRunning means batches are progressing.
	StatusRunning RunStatus = "running"
	// StatusWaiting means the heartbeat is fresh but no batch is progressing,
	// which is the honest answer to "is it stuck or is the provider slow?".
	StatusWaiting RunStatus = "waiting"
	// StatusCancelling means cancellation has been requested.
	StatusCancelling RunStatus = "cancelling"
)

// RunPhase is which stage of the pipeline is executing.
type RunPhase = string

const (
	PhaseHeuristics RunPhase = "heuristics"
	PhaseLLM        RunPhase = "llm"
)

// RunState is a run in flight, persisted so a run that dies leaves a trace.
//
// A run killed mid-flight used to write nothing at all: the panel showed the
// previous result while looking frozen.
type RunState struct {
	// RunID identifies the run that owns this state. A takeover replaces it
	// wholesale, so a superseded run can tell the state is no longer its own and
	// must neither clear nor overwrite its replacement.
	RunID     string
	StartedAt time.Time
	// UpdatedAt is the heartbeat, refreshed after every batch.
	UpdatedAt time.Time
	// Deadline is when this pass must stop, so the interface can tell an estimate
	// that fits inside the pass from one that does not.
	Deadline  time.Time
	Phase     RunPhase
	Processed int
	Total     int
	// Suggested is how many suggestions this pass has written so far, which is
	// the product a reader wants to see while it runs.
	Suggested int
	// RecentTags are the last tag names this pass suggested, newest last, capped
	// at RecentTagLimit. Names rather than ids because they are drawn as they are.
	RecentTags []string
	// CancelRequested is written by the UI and read back by the pipeline after
	// each batch, which is why the heartbeat write and the cancel read are the
	// same round trip.
	CancelRequested bool

	Status         RunStatus
	InFlight       bool
	BatchStartedAt *time.Time
	WaitingUntil   *time.Time
	WaitingReason  string
	Attempt        int
	ProviderID     string
	Model          string
	LastErrorKind  string
}

// NewRunID mints a run identity.
func NewRunID(now time.Time) string {
	return fmt.Sprintf("run-%d", now.UnixNano())
}

// IsStale reports whether a heartbeat stopped moving, meaning the run died
// without finishing.
//
// The comparison is `now - heartbeat > window` rather than `>=`, and a state with
// an unparseable heartbeat counts as stale.
func IsStale(state *RunState, now time.Time, staleAfter time.Duration) bool {
	if state == nil {
		return false
	}
	if state.UpdatedAt.IsZero() {
		return true
	}
	return now.Sub(state.UpdatedAt) > staleAfter
}

// IsActive reports whether a run is really in flight right now.
//
// An absent state means finished, and a frozen heartbeat means dead, so both read
// as not running. This is the only terminal signal a run produces.
func IsActive(state *RunState, now time.Time, staleAfter time.Duration) bool {
	return state != nil && !IsStale(state, now, staleAfter)
}

// OwnedBy reports whether this state belongs to the given run.
func (s *RunState) OwnedBy(runID string) bool {
	return s != nil && s.RunID == runID
}

// RecentTagLimit caps RunState.RecentTags.
//
// The whole state is persisted as one JSON row after every batch, so the list is a
// glance at the latest work, not a log.
const RecentTagLimit = 3

// BeginInput seeds a new run.
type BeginInput struct {
	Phase      RunPhase
	Total      int
	ProviderID string
	Model      string
	// Deadline is the run budget's end. Zero means no budget is known.
	Deadline time.Time
}

// Begin publishes "a run is in flight" before the first request goes out, so a
// run that dies mid-flight is still visible afterwards.
//
// Any stale state is replaced, and a pending cancel is discarded with it: that
// cancel belonged to a run that is already gone.
func Begin(now time.Time, input BeginInput) *RunState {
	total := input.Total
	if total < 0 {
		total = 0
	}
	return &RunState{
		RunID:      NewRunID(now),
		StartedAt:  now,
		UpdatedAt:  now,
		Deadline:   input.Deadline,
		Phase:      input.Phase,
		Processed:  0,
		Total:      total,
		Status:     StatusRunning,
		ProviderID: input.ProviderID,
		Model:      input.Model,
	}
}

// TouchInput is one heartbeat plus optional detail.
type TouchInput struct {
	RunID     string
	Phase     RunPhase
	Now       time.Time
	Processed int
	Total     int

	// The fields below are pointers or flags so "not provided" keeps whatever the
	// state already had. A plain heartbeat must not erase a pending
	// "waiting until" marker set by a different writer.
	Status         *RunStatus
	InFlight       *bool
	BatchStartedAt *time.Time
	WaitingUntil   *time.Time
	WaitingReason  *string
	Attempt        *int
	ProviderID     *string
	Model          *string
	LastErrorKind  *string
	Suggested      *int
	RecentTags     *[]string
}

// Touch refreshes the heartbeat and returns the state the UI may have written,
// which is how a cancel reaches a running pipeline without a second read.
//
// It is scoped to the run that owns the state. A takeover installs a different
// run id, so a superseded run stops refreshing someone else's heartbeat, stops
// reading someone else's cancel flag, and is told to stop.
func Touch(current *RunState, input TouchInput) (next *RunState, ownsState bool) {
	if current == nil || current.RunID != input.RunID {
		return current, false
	}

	updated := *current
	updated.UpdatedAt = input.Now
	if input.Phase != "" {
		updated.Phase = input.Phase
	}
	if input.Processed >= 0 {
		updated.Processed = input.Processed
	}
	if input.Total >= 0 {
		updated.Total = input.Total
	}

	if input.Status != nil {
		updated.Status = *input.Status
	}
	if input.InFlight != nil {
		updated.InFlight = *input.InFlight
		if !*input.InFlight {
			updated.BatchStartedAt = nil
		}
	}
	if input.BatchStartedAt != nil {
		updated.BatchStartedAt = input.BatchStartedAt
	}
	// Passing a nil pointer for these means "leave as is"; to clear one, set an
	// explicit zero value through the pointer.
	if input.WaitingUntil != nil {
		updated.WaitingUntil = input.WaitingUntil
	}
	if input.WaitingReason != nil {
		updated.WaitingReason = *input.WaitingReason
	}
	if input.Attempt != nil {
		updated.Attempt = *input.Attempt
	}
	if input.ProviderID != nil {
		updated.ProviderID = *input.ProviderID
	}
	if input.Model != nil {
		updated.Model = *input.Model
	}
	if input.LastErrorKind != nil {
		updated.LastErrorKind = *input.LastErrorKind
	}
	if input.Suggested != nil {
		updated.Suggested = *input.Suggested
	}
	if input.RecentTags != nil {
		updated.RecentTags = append([]string(nil), (*input.RecentTags)...)
	}

	// cancelRequested is preserved implicitly by copying the whole state.

	return &updated, true
}

// RequestCancel marks the state cancelled without stopping anything.
//
// The pipeline stops at its next batch boundary, so suggestions already written
// are never lost.
func RequestCancel(current *RunState) (*RunState, bool) {
	if current == nil {
		return current, false
	}
	updated := *current
	updated.CancelRequested = true
	updated.Status = StatusCancelling
	return &updated, true
}

// ShouldCancel is what a pipeline asks at each batch boundary.
//
// A superseded run is told to stop: it no longer owns the state, and its work
// belongs to a run the user has already replaced.
func ShouldCancel(current *RunState, runID string, now time.Time) (stop bool, ownsState bool) {
	if current == nil || current.RunID != runID {
		// A defined run id that no longer owns the state has been taken over.
		return runID != "", false
	}
	return current.CancelRequested, true
}

// Finish clears the in-flight state and records the outcome, but only for the
// run that owns it.
//
// A superseded run must stay silent: clearing here would hide the run that
// replaced it, and its own report would describe work that was thrown away.
func Finish(current *RunState, runID string) (cleared *RunState, reported bool) {
	if current == nil {
		// Nothing is live, so reporting is always safe.
		return nil, true
	}
	if runID == "" || current.RunID != runID {
		return current, false
	}
	return nil, true
}

// ContinueLimit caps how many times a run may be chained automatically.
//
// A run that stops at its budget with work left can start another, but a stuck
// run must not spin forever.
const ContinueLimit = 5

// ShouldContinue decides whether to start another run after one finished.
func ShouldContinue(summary RunSummary, continueCount int, autoContinue bool) bool {
	if !autoContinue {
		return false
	}
	if summary.Outcome == OutcomeCancelled {
		// The user asked it to stop; continuing would ignore that.
		return false
	}
	if summary.UnprocessedCount <= 0 {
		return false
	}
	return continueCount < ContinueLimit
}

// --- outcome ---

// RunOutcome explains why a run produced what it did.
//
// Every value exists because a distinct value was needed to stop a confusing
// report: "done, 0 suggestions" was indistinguishable from a failure, a skip, or
// a budget stop.
type RunOutcome = string

const (
	OutcomeOK              RunOutcome = "ok"
	OutcomeDisabled        RunOutcome = "disabled"
	OutcomeLLMDisabled     RunOutcome = "llm-disabled"
	OutcomeMissingAPIKey   RunOutcome = "missing-api-key"
	OutcomeNoCandidates    RunOutcome = "no-candidates"
	OutcomeEmptyVocabulary RunOutcome = "empty-vocabulary"
	OutcomeAllBlocked      RunOutcome = "all-blocked"
	OutcomeBudgetExhausted RunOutcome = "budget-exhausted"
	OutcomeTimedOut        RunOutcome = "timed-out"
	OutcomeCancelled       RunOutcome = "cancelled"
	OutcomeInterrupted     RunOutcome = "interrupted"
	OutcomeFailed          RunOutcome = "failed"
)

// AllRunOutcomes is every value, for validation on read.
var AllRunOutcomes = []RunOutcome{
	OutcomeOK, OutcomeDisabled, OutcomeLLMDisabled, OutcomeMissingAPIKey,
	OutcomeNoCandidates, OutcomeEmptyVocabulary, OutcomeAllBlocked,
	OutcomeBudgetExhausted, OutcomeTimedOut, OutcomeCancelled,
	OutcomeInterrupted, OutcomeFailed,
}

// ValidRunOutcome reports whether a stored outcome is one this build knows.
func ValidRunOutcome(outcome string) bool {
	for _, candidate := range AllRunOutcomes {
		if outcome == candidate {
			return true
		}
	}
	return false
}

// RunSummary is the report a finished run leaves behind.
type RunSummary struct {
	FinishedAt time.Time
	Outcome    RunOutcome
	// Counts a user needs to understand a run.
	SuggestedCount      int
	PendingCount        int
	CandidateCount      int
	LLMProcessedCount   int
	LLMBatchCount       int
	LLMFailedBatchCount int
	// NewTagBudget is -1 when unbounded: Infinity is not representable in JSON.
	NewTagBudget        int
	VocabularySize      int
	BlockedTagKeyCount  int
	BlockedDroppedCount int
	BudgetDroppedCount  int
	UnprocessedCount    int
	Error               string
}

// UnboundedBudget is the sentinel for an unlimited new-tag budget.
//
// It exists because JSON has no Infinity: -1 is the serialized form, so keeping
// that value is what makes an existing settings row still read correctly.
const UnboundedBudget = -1

// DecodeBudget turns a stored budget back into a number, mapping the sentinel.
func DecodeBudget(stored int) float64 {
	if stored == UnboundedBudget {
		return math.Inf(1)
	}
	return float64(stored)
}

// EncodeBudget renders a budget for storage, mapping infinity to the sentinel.
func EncodeBudget(budget float64) int {
	if math.IsInf(budget, 1) {
		return UnboundedBudget
	}
	return int(budget)
}
