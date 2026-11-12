package autotag

import (
	"testing"
	"time"
)

// TestStallHintIsSeparateFromLiveness pins the distinction that was wrong: the interface
// hint fires in seconds, the liveness window in minutes. Using one for the other left the
// panel silent for two and a half minutes before admitting a run was wedged.
func TestStallHintIsSeparateFromLiveness(t *testing.T) {
	if StallHintAfter >= RunLivenessAfter {
		t.Fatalf("the stall hint (%s) must be far shorter than the liveness window (%s)",
			StallHintAfter, RunLivenessAfter)
	}

	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

	// Quiet for longer than the hint but well inside the liveness window: stalled.
	stalled := &RunState{
		StartedAt: now.Add(-time.Minute),
		UpdatedAt: now.Add(-StallHintAfter - time.Second),
		Phase:     PhaseLLM, Processed: 5, Total: 20,
	}
	progress := BuildRunProgress(stalled, now)
	if !progress.Active {
		t.Fatal("a run inside the liveness window is still active")
	}
	if !progress.Stalled {
		t.Errorf("a heartbeat quiet for %s must be reported as stalled", StallHintAfter+time.Second)
	}
	if progress.SecondsSinceHeartbeat != int(StallHintAfter.Seconds())+1 {
		t.Errorf("seconds since heartbeat = %d", progress.SecondsSinceHeartbeat)
	}

	// Past the liveness window: not active at all.
	dead := &RunState{StartedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-RunLivenessAfter - time.Second)}
	if got := BuildRunProgress(dead, now); got.Active {
		t.Error("a run past the liveness window must not be active")
	}

	// A fresh heartbeat is neither stalled nor inactive.
	fresh := &RunState{StartedAt: now.Add(-30 * time.Second), UpdatedAt: now.Add(-time.Second), Total: 10, Processed: 2}
	if got := BuildRunProgress(fresh, now); got.Stalled || !got.Active {
		t.Errorf("a fresh heartbeat = %+v", got)
	}
}

// TestARequestInFlightIsSlowNotDead is the distinction the panel got wrong.
//
// The heartbeat moves at batch boundaries, so one slow batch is quiet for far longer
// than the stall hint. Reporting that as "it may have stopped" told every reader of a
// working run that it was broken -- the exact screenshot that prompted this.
func TestARequestInFlightIsSlowNotDead(t *testing.T) {
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

	inFlight := BuildRunProgress(&RunState{
		StartedAt: now.Add(-4 * time.Minute), UpdatedAt: now.Add(-59 * time.Second),
		Phase: PhaseLLM, InFlight: true, Processed: 450, Total: 9509,
	}, now)
	if inFlight.Stalled {
		t.Error("a request still out is slow, not stalled")
	}
	if !inFlight.InFlightWaiting {
		t.Error("the in-flight wait must be reported so the panel can say what it is waiting for")
	}
	if inFlight.SecondsSinceHeartbeat != 59 {
		t.Errorf("seconds since heartbeat = %d, want 59", inFlight.SecondsSinceHeartbeat)
	}

	// The rule phase is local work with no network at all. A long one is not a stall
	// either, and it was the other half of the same false alarm.
	rules := BuildRunProgress(&RunState{
		StartedAt: now.Add(-time.Minute), UpdatedAt: now.Add(-30 * time.Second),
		Phase: PhaseHeuristics, Total: 9509,
	}, now)
	if rules.Stalled || rules.InFlightWaiting {
		t.Errorf("the rule phase must not be reported as stalled: %+v", rules)
	}

	// No request out and no back-off: that is the genuine silence worth reporting.
	quiet := BuildRunProgress(&RunState{
		StartedAt: now.Add(-time.Minute), UpdatedAt: now.Add(-30 * time.Second),
		Phase: PhaseLLM, Processed: 200, Total: 9000,
	}, now)
	if !quiet.Stalled {
		t.Error("a quiet heartbeat with no request out is a stall")
	}
}

// TestTheEstimateNeverOutlivesThePass pins the number a reader can act on.
//
// The source's estimate is arithmetically fine and practically useless here: nine
// thousand bookmarks against a four-minute pass produced "about 4352s left" beside a
// run that stopped in twenty seconds. When the estimate does not fit in the pass, the
// pass's own remaining time is the honest number; the summary explains the rest.
func TestTheEstimateNeverOutlivesThePass(t *testing.T) {
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

	overrun := BuildRunProgress(&RunState{
		StartedAt: now.Add(-4 * time.Minute), UpdatedAt: now, Deadline: now.Add(20 * time.Second),
		Phase: PhaseLLM, Processed: 450, Total: 9509,
	}, now)
	if overrun.HasETA {
		t.Errorf("an estimate of %s must not be shown beside a 20s pass", overrun.ETA)
	}
	if !overrun.HasBudgetLeft || overrun.BudgetSecondsLeft != 20 {
		t.Errorf("budget left = %d (has=%v), want 20", overrun.BudgetSecondsLeft, overrun.HasBudgetLeft)
	}

	// An estimate that fits inside the pass is still the better answer.
	fits := BuildRunProgress(&RunState{
		StartedAt: now.Add(-40 * time.Second), UpdatedAt: now,
		Deadline: now.Add(3 * time.Minute),
		Phase:    PhaseLLM, Processed: 30, Total: 60,
	}, now)
	if !fits.HasETA || fits.HasBudgetLeft {
		t.Errorf("a fitting estimate must be shown instead of the pass budget: %+v", fits)
	}
	if fits.ETA < 35*time.Second || fits.ETA > 45*time.Second {
		t.Errorf("eta = %s, want about 40s", fits.ETA)
	}

	// No deadline known: the estimate stands on its own, as before.
	unbounded := BuildRunProgress(&RunState{
		StartedAt: now.Add(-40 * time.Second), UpdatedAt: now, Phase: PhaseLLM, Processed: 10, Total: 40,
	}, now)
	if !unbounded.HasETA || unbounded.HasBudgetLeft {
		t.Errorf("without a deadline the estimate is all there is: %+v", unbounded)
	}
}

// TestTheProgressCarriesThePassesProducts is the "what has it made so far" answer.
func TestTheProgressCarriesThePassesProducts(t *testing.T) {
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

	progress := BuildRunProgress(&RunState{
		StartedAt: now.Add(-time.Minute), UpdatedAt: now,
		Phase: PhaseLLM, Processed: 50, Total: 200,
		Suggested: 7, RecentTags: []string{"go", "baking"},
	}, now)
	if progress.Suggested != 7 {
		t.Errorf("suggested = %d, want 7", progress.Suggested)
	}
	if len(progress.RecentTags) != 2 || progress.RecentTags[0] != "go" {
		t.Errorf("recent tags = %v", progress.RecentTags)
	}

	// The slice is copied, so a caller cannot reach into the run state's own slice.
	state := &RunState{StartedAt: now, UpdatedAt: now, RecentTags: []string{"go"}}
	copied := BuildRunProgress(state, now)
	copied.RecentTags[0] = "changed"
	if state.RecentTags[0] != "go" {
		t.Error("the progress must not alias the run state's tags")
	}
}

// TestProgressDerivedValues covers percent, the clamp and the estimate.
func TestProgressDerivedValues(t *testing.T) {
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

	// Percent, and the total-rounding that goes with it.
	progress := BuildRunProgress(&RunState{
		StartedAt: now.Add(-40 * time.Second), UpdatedAt: now, Processed: 10, Total: 40,
	}, now)
	if progress.Percent != 25 {
		t.Errorf("percent = %d, want 25", progress.Percent)
	}
	// 40 seconds for 10 items leaves 30 to go at 4 s each: about 120 seconds.
	if !progress.HasETA || progress.ETA < 115*time.Second || progress.ETA > 125*time.Second {
		t.Errorf("eta = %s (has=%v), want about 120s", progress.ETA, progress.HasETA)
	}

	// No total means no percentage rather than a division by zero.
	unknown := BuildRunProgress(&RunState{StartedAt: now.Add(-time.Second), UpdatedAt: now}, now)
	if unknown.Percent != 0 {
		t.Errorf("percent with no total = %d", unknown.Percent)
	}
	if unknown.HasETA {
		t.Error("an estimate needs a total")
	}

	// Processed is clamped to a total that was revised downwards.
	clamped := BuildRunProgress(&RunState{
		StartedAt: now.Add(-time.Second), UpdatedAt: now, Processed: 12, Total: 10,
	}, now)
	if clamped.Processed != 10 {
		t.Errorf("processed = %d, want it clamped to 10", clamped.Processed)
	}
	if clamped.Percent != 100 {
		t.Errorf("percent = %d, want 100", clamped.Percent)
	}

	// Nothing finished yet: no estimate, because an average over nothing is noise.
	early := BuildRunProgress(&RunState{
		StartedAt: now.Add(-40 * time.Second), UpdatedAt: now, Processed: 0, Total: 40,
	}, now)
	if early.HasETA {
		t.Error("a run with nothing finished must not offer an estimate")
	}

	// A back-off is reported with its remaining seconds, and is not a stall.
	waitingUntil := now.Add(25 * time.Second)
	waiting := BuildRunProgress(&RunState{
		StartedAt: now.Add(-5 * time.Minute), UpdatedAt: now.Add(-time.Minute),
		WaitingUntil: &waitingUntil,
	}, now)
	if !waiting.HasWaitingLeft || waiting.WaitingSecondsLeft != 25 {
		t.Errorf("waiting left = %d (has=%v), want 25", waiting.WaitingSecondsLeft, waiting.HasWaitingLeft)
	}
	if waiting.Stalled {
		t.Error("a back-off must not be reported as a stall")
	}

	// No state at all is not an active run.
	if got := BuildRunProgress(nil, now); got.Active {
		t.Error("no state must not be an active run")
	}
}
