package autotag

import (
	"math"
	"time"
)

// StallHintAfter is when the interface stops pretending a run is progressing.
//
// It is the source's STALL_HINT_MS, and it is deliberately *not* RunLivenessAfter. The
// two answer different questions: this one says "the provider is slow or the request is
// wedged" while a batch is in flight, and the liveness window says "the run is dead and
// may be taken over". Using the liveness window for both meant the interface stayed
// silent for two and a half minutes before admitting anything was wrong.
const StallHintAfter = 8 * time.Second

// RunProgress is the run state as the interface renders it.
//
// The source computes all of this in one place and shares it between the sidebar and the
// settings panel, so the two cannot disagree about what a run is doing.
type RunProgress struct {
	Active bool
	Status RunStatus
	Phase  RunPhase

	Processed int
	Total     int
	// Percent is 0 when the total is unknown, rather than a division by zero.
	Percent int

	// Stalled means the heartbeat went quiet while nothing else explains it.
	Stalled bool
	// InFlightWaiting means a request is out and the heartbeat is older than the
	// stall hint. That is a slow provider, not a dead run: the difference matters
	// because one is normal and the other is worth alarming about.
	InFlightWaiting bool
	// SecondsSinceHeartbeat is how long the heartbeat has been silent.
	SecondsSinceHeartbeat int
	// WaitingSecondsLeft is set only while a provider has asked us to back off.
	WaitingSecondsLeft int
	HasWaitingLeft     bool

	InFlight bool
	Elapsed  time.Duration
	// ETA is set only once enough has finished for an average to mean something,
	// and only when it fits inside what is left of the pass.
	ETA    time.Duration
	HasETA bool
	// BudgetSecondsLeft is what remains of the pass's own budget. It is set when
	// there is a deadline and the estimate does not fit inside it: the run cannot
	// finish in this pass, so the honest number is when the pass ends.
	BudgetSecondsLeft int
	HasBudgetLeft     bool

	// Suggested and RecentTags are this pass's products so far.
	Suggested  int
	RecentTags []string

	ProviderID string
	Model      string
	StartedAt  time.Time
}

// BuildRunProgress derives the display values from a run state.
//
// It returns an inactive progress for a nil or stale state, so the caller never has to
// ask whether a run is worth showing.
func BuildRunProgress(state *RunState, now time.Time) RunProgress {
	if state == nil || now.Sub(state.UpdatedAt) > RunLivenessAfter {
		return RunProgress{}
	}

	sinceHeartbeat := now.Sub(state.UpdatedAt)
	if sinceHeartbeat < 0 {
		sinceHeartbeat = 0
	}
	elapsed := now.Sub(state.StartedAt)
	if elapsed < 0 {
		elapsed = 0
	}

	total := state.Total
	if total < 0 {
		total = 0
	}
	processed := state.Processed
	if processed < 0 {
		processed = 0
	}
	// A processed count above the total means the total was revised downwards mid-run;
	// showing 12/10 would be worse than showing 10/10.
	if total > 0 && processed > total {
		processed = total
	}

	progress := RunProgress{
		Active:                true,
		Status:                state.Status,
		Phase:                 state.Phase,
		Processed:             processed,
		Total:                 total,
		SecondsSinceHeartbeat: int(math.Round(sinceHeartbeat.Seconds())),
		InFlight:              state.InFlight,
		Elapsed:               elapsed,
		Suggested:             state.Suggested,
		RecentTags:            append([]string(nil), state.RecentTags...),
		ProviderID:            state.ProviderID,
		Model:                 state.Model,
		StartedAt:             state.StartedAt,
	}

	if total > 0 {
		progress.Percent = int(math.Min(100, math.Round(float64(processed)/float64(total)*100)))
	}

	// Waiting for a provider's back-off is not a stall; saying so would train the user
	// to ignore the message.
	//
	// A quiet heartbeat during the rule phase is not a stall either: nothing there
	// talks to the network, and the phase ends when the local loop ends. And a quiet
	// heartbeat *with a request out* is a slow model, which is the normal case -- the
	// heartbeat only moves at batch boundaries, so a single 25-item batch can be quiet
	// for a minute. Calling that "it may have stopped" told every reader of a working
	// run that it was broken.
	switch {
	case state.WaitingUntil != nil && state.WaitingUntil.After(now):
		progress.WaitingSecondsLeft = int(math.Ceil(state.WaitingUntil.Sub(now).Seconds()))
		progress.HasWaitingLeft = true
	case state.Phase == PhaseLLM && sinceHeartbeat >= StallHintAfter:
		if state.InFlight {
			progress.InFlightWaiting = true
		} else {
			progress.Stalled = true
		}
	}

	// An average over too few finished items is noise, so no estimate is offered until
	// something has finished. That is the source's rule and it is the right one: a
	// single batch's timing says almost nothing about a hundred more.
	//
	// The estimate is only shown when it fits inside the pass. A run over nine
	// thousand bookmarks with a four-minute budget has an "estimate" of an hour or
	// more, and printing it beside a run that will stop in twenty seconds is worse
	// than printing nothing: the number is arithmetically right and practically a lie.
	// What is left of the pass is the number the reader can act on.
	if processed > 0 && total > processed && elapsed > 0 {
		remaining := float64(elapsed) / float64(processed) * float64(total-processed)
		estimate := time.Duration(math.Max(1, math.Round(remaining)))
		if state.Deadline.IsZero() {
			progress.ETA = estimate
			progress.HasETA = true
		} else {
			left := state.Deadline.Sub(now)
			if left < 0 {
				left = 0
			}
			if estimate <= left {
				progress.ETA = estimate
				progress.HasETA = true
			} else {
				progress.BudgetSecondsLeft = int(math.Ceil(left.Seconds()))
				progress.HasBudgetLeft = true
			}
		}
	}

	return progress
}
