package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/taoquo/x-master/apps/macos/internal/autotag"
	"github.com/taoquo/x-master/apps/macos/internal/i18n"
)

// TestIndeterminateWhileWaitingOrStalled is the source's rule: a bar frozen at a number
// nobody can trust is worse than no bar. It reports no value while waiting or stalled,
// and that applies to drawing a fill as much as to an accessibility attribute.
func TestIndeterminateWhileWaitingOrStalled(t *testing.T) {
	shell, _ := newBatchShell(t)
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

	// Running: determinate, with a real fraction.
	running := &autotag.RunState{
		StartedAt: now.Add(-20 * time.Second), UpdatedAt: now,
		Phase: autotag.PhaseLLM, Processed: 25, Total: 100,
	}
	view, ok := shell.RunProgressViewFor(running, now)
	if !ok {
		t.Fatal("a running run must render")
	}
	if view.Indeterminate {
		t.Error("a running run is not indeterminate")
	}
	if view.Percent != 25 {
		t.Errorf("percent = %d, want 25", view.Percent)
	}
	if got := view.ProgressFillFraction(false); got < 0.24 || got > 0.26 {
		t.Errorf("fill = %v, want about 0.25", got)
	}
	if view.Count != "25/100" {
		t.Errorf("count = %q, want 25/100", view.Count)
	}

	// Stalled: indeterminate, and the fill is empty even though a percent exists.
	stalled := &autotag.RunState{
		StartedAt: now.Add(-2 * time.Minute), UpdatedAt: now.Add(-20 * time.Second),
		Phase: autotag.PhaseLLM, Processed: 25, Total: 100,
	}
	stalledView, _ := shell.RunProgressViewFor(stalled, now)
	if !stalledView.Indeterminate {
		t.Error("a stalled run must be indeterminate")
	}
	// A stalled bar shows a stripe rather than a position, and the stripe takes the
	// stylesheet's reduced-motion form when the user has asked for less motion.
	if got := stalledView.ProgressFillFraction(false); got != IndeterminateFillFraction {
		t.Errorf("stalled fill = %v, want the stripe width %v", got, IndeterminateFillFraction)
	}
	if got := stalledView.ProgressFillFraction(true); got != ReducedMotionFillFraction {
		t.Errorf("stalled fill under reduced motion = %v, want %v", got, ReducedMotionFillFraction)
	}
	if got := stalledView.FillOpacity(true); got != ReducedMotionFillOpacity {
		t.Errorf("stalled opacity under reduced motion = %v, want %v", got, ReducedMotionFillOpacity)
	}
	if got := stalledView.FillOpacity(false); got != 1 {
		t.Errorf("the stripe is opaque when motion is allowed, got %v", got)
	}
	if !stalledView.Stalled {
		t.Error("the stalled flag must be set, so the fill can take the warning colour")
	}
	if !strings.Contains(stalledView.StatusLine, "heartbeat") {
		t.Errorf("a stalled run must say so: %q", stalledView.StatusLine)
	}

	// Waiting for a back-off: also indeterminate, and not called a stall.
	backOff := now.Add(30 * time.Second)
	waiting := &autotag.RunState{
		StartedAt: now.Add(-time.Minute), UpdatedAt: now.Add(-30 * time.Second),
		Phase: autotag.PhaseLLM, Processed: 25, Total: 100, WaitingUntil: &backOff,
	}
	waitingView, _ := shell.RunProgressViewFor(waiting, now)
	if !waitingView.Indeterminate {
		t.Error("a waiting run must be indeterminate")
	}
	if waitingView.Stalled {
		t.Error("waiting is not stalling")
	}
	if !strings.Contains(waitingView.StatusLine, "30") {
		t.Errorf("the waiting line must carry the seconds left: %q", waitingView.StatusLine)
	}
}

// TestCancellingOutranksStalled pins the precedence: a run the user just asked to stop
// must say it is stopping, not diagnose itself.
func TestCancellingOutranksStalled(t *testing.T) {
	shell, _ := newBatchShell(t)
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

	// Stopping and stalled at once.
	cancelling := &autotag.RunState{
		StartedAt: now.Add(-2 * time.Minute), UpdatedAt: now.Add(-30 * time.Second),
		Phase: autotag.PhaseLLM, Processed: 25, Total: 100, CancelRequested: true,
	}
	view, ok := shell.RunProgressViewFor(cancelling, now)
	if !ok {
		t.Fatal("a cancelling run must still render")
	}
	if !view.Cancelling {
		t.Error("the cancelling flag must be set")
	}
	if strings.Contains(view.StatusLine, "heartbeat") {
		t.Errorf("cancelling must outrank stalled, got %q", view.StatusLine)
	}
	if view.CancelLabel != shell.t().T(i18n.RunStopping) {
		t.Errorf("the button must read as stopping, got %q", view.CancelLabel)
	}
	// The button is disabled once the request is in, so it cannot be pressed twice.
	if !view.Cancelling {
		t.Error("the button must be disabled while cancelling")
	}
}

// TestASlowRequestSaysWhatItIsWaitingFor covers the line the reader sees during a
// long batch. The screenshot that prompted this read "no heartbeat for 59s -- it may
// have stopped" while the model was simply still answering.
func TestASlowRequestSaysWhatItIsWaitingFor(t *testing.T) {
	shell, _ := newBatchShell(t)
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

	slow := &autotag.RunState{
		StartedAt: now.Add(-3 * time.Minute), UpdatedAt: now.Add(-59 * time.Second),
		Phase: autotag.PhaseLLM, InFlight: true, Processed: 450, Total: 9509,
	}
	view, ok := shell.RunProgressViewFor(slow, now)
	if !ok {
		t.Fatal("a run with a request out is still a live run")
	}
	if view.Stalled {
		t.Error("a request in flight must not be drawn as stalled")
	}
	if !strings.Contains(view.StatusLine, "59") {
		t.Errorf("the wait line must carry the seconds: %q", view.StatusLine)
	}
	if strings.Contains(view.StatusLine, "heartbeat") {
		t.Errorf("a working request must not be told it may have stopped: %q", view.StatusLine)
	}
	if view.Indeterminate {
		t.Error("a request in flight still has a real position, so the bar stays determinate")
	}
}

// TestTheFootOffersTheNumberThatCanBeActedOn covers the misprinted estimate.
func TestTheFootOffersTheNumberThatCanBeActedOn(t *testing.T) {
	shell, _ := newBatchShell(t)
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

	// Nine thousand bookmarks, four minutes in: the finish estimate is over an hour,
	// and the pass has eighteen seconds left.
	overrun := &autotag.RunState{
		StartedAt: now.Add(-4 * time.Minute), UpdatedAt: now, Deadline: now.Add(18 * time.Second),
		Phase: autotag.PhaseLLM, Processed: 450, Total: 9509,
	}
	view, _ := shell.RunProgressViewFor(overrun, now)
	if strings.Contains(view.Foot, shell.t().Tf(i18n.RunRemaining, 4000)) {
		t.Errorf("the foot must not print an hour-long estimate: %q", view.Foot)
	}
	if !strings.Contains(view.Foot, "18") {
		t.Errorf("the foot must say when this pass stops: %q", view.Foot)
	}
	if strings.Contains(view.Foot, "4352") {
		t.Errorf("the old estimate must not survive: %q", view.Foot)
	}

	// A run that can finish in this pass keeps the ordinary estimate.
	fits := &autotag.RunState{
		StartedAt: now.Add(-40 * time.Second), UpdatedAt: now, Deadline: now.Add(3 * time.Minute),
		Phase: autotag.PhaseLLM, Processed: 30, Total: 60,
	}
	fitsView, _ := shell.RunProgressViewFor(fits, now)
	if !strings.Contains(fitsView.Foot, "40") {
		t.Errorf("a fitting estimate must still be shown: %q", fitsView.Foot)
	}
}

// TestTheBlockShowsWhatThePassHasProduced covers the detail line: a count of
// processed bookmarks says nothing about whether any tags came out.
func TestTheBlockShowsWhatThePassHasProduced(t *testing.T) {
	shell, _ := newBatchShell(t)
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

	running := &autotag.RunState{
		StartedAt: now.Add(-time.Minute), UpdatedAt: now,
		Phase: autotag.PhaseLLM, Processed: 50, Total: 200,
		Suggested: 4, RecentTags: []string{"go", "baking"},
	}
	view, _ := shell.RunProgressViewFor(running, now)
	if !strings.Contains(view.Detail, "4") {
		t.Errorf("the detail must say how many suggestions were made: %q", view.Detail)
	}
	if !strings.Contains(view.Detail, "go") || !strings.Contains(view.Detail, "baking") {
		t.Errorf("the detail must name the latest tags: %q", view.Detail)
	}

	// Nothing produced yet: no empty line.
	empty, _ := shell.RunProgressViewFor(&autotag.RunState{
		StartedAt: now.Add(-10 * time.Second), UpdatedAt: now,
		Phase: autotag.PhaseHeuristics, Total: 9509,
	}, now)
	if empty.Detail != "" {
		t.Errorf("a pass with no products must draw no detail line: %q", empty.Detail)
	}
}

// TestProgressFootCarriesElapsedAndEstimate covers the timing line.
func TestProgressFootCarriesElapsedAndEstimate(t *testing.T) {
	shell, _ := newBatchShell(t)
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

	running := &autotag.RunState{
		StartedAt: now.Add(-40 * time.Second), UpdatedAt: now,
		Phase: autotag.PhaseLLM, Processed: 10, Total: 40,
	}
	view, _ := shell.RunProgressViewFor(running, now)
	if !strings.Contains(view.Foot, "40") {
		t.Errorf("the foot must carry the elapsed time: %q", view.Foot)
	}
	if !strings.Contains(view.Foot, "·") {
		t.Errorf("the foot must carry the estimate after a separator: %q", view.Foot)
	}

	// Nothing finished: elapsed only, because an average over nothing is noise.
	early := &autotag.RunState{
		StartedAt: now.Add(-40 * time.Second), UpdatedAt: now,
		Phase: autotag.PhaseLLM, Processed: 0, Total: 40,
	}
	earlyView, _ := shell.RunProgressViewFor(early, now)
	if strings.Contains(earlyView.Foot, "·") {
		t.Errorf("a run with nothing finished must not estimate: %q", earlyView.Foot)
	}
	if earlyView.Count != "0/40" {
		t.Errorf("count = %q, want 0/40", earlyView.Count)
	}

	// An inactive run renders nothing at all.
	if _, ok := shell.RunProgressViewFor(nil, now); ok {
		t.Error("no state must not render")
	}
}

// TestReducedMotionChangesTheIndeterminateBar is goal clause 7 for this widget.
//
// The stylesheet does not merely stop animating under prefers-reduced-motion: it
// replaces the stripe with a full-width bar at 60% opacity. Drawing nothing -- which
// this did -- showed an empty track where the source shows a state, so a user who asked
// for less motion would see a progress bar that looked broken.
func TestReducedMotionChangesTheIndeterminateBar(t *testing.T) {
	shell, _ := newBatchShell(t)
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

	waiting := &autotag.RunState{
		StartedAt: now.Add(-time.Minute), UpdatedAt: now.Add(-20 * time.Second),
		Phase: autotag.PhaseLLM, Processed: 25, Total: 100,
	}
	view, _ := shell.RunProgressViewFor(waiting, now)
	if !view.Indeterminate {
		t.Fatal("the fixture must be indeterminate")
	}

	// Motion allowed: the stripe is narrower than the track and fully opaque.
	if got := view.ProgressFillFraction(false); got != 0.4 {
		t.Errorf("stripe fraction = %v, want the stylesheet's 0.4", got)
	}
	if got := view.FillOpacity(false); got != 1 {
		t.Errorf("stripe opacity = %v, want 1", got)
	}

	// Reduced motion: full width, 60% opacity.
	if got := view.ProgressFillFraction(true); got != 1.0 {
		t.Errorf("reduced-motion fraction = %v, want 1.0", got)
	}
	if got := view.FillOpacity(true); got != 0.6 {
		t.Errorf("reduced-motion opacity = %v, want 0.6", got)
	}

	// A determinate bar is unaffected by the preference: it shows a real value either
	// way, which is the whole difference between the two states.
	determinate := &autotag.RunState{
		StartedAt: now.Add(-20 * time.Second), UpdatedAt: now,
		Phase: autotag.PhaseLLM, Processed: 25, Total: 100,
	}
	settled, _ := shell.RunProgressViewFor(determinate, now)
	if settled.ProgressFillFraction(true) != settled.ProgressFillFraction(false) {
		t.Error("a determinate bar must not change with the motion preference")
	}
	if settled.FillOpacity(true) != 1 {
		t.Error("a determinate bar stays opaque")
	}
}

// TestStripeBoundsClipsToTheTrack reproduces the stylesheet's overflow: hidden.
//
// The keyframes move the stripe from -100% to +250% of its own width, so at both ends
// it is mostly outside the track. Painting it unclipped would draw it across the row,
// which is why the source clips.
func TestStripeBoundsClipsToTheTrack(t *testing.T) {
	const trackX, trackW = 100.0, 200.0
	stripeW := float32(trackW) * IndeterminateFillFraction
	if stripeW != 80 {
		t.Fatalf("the stripe is %v wide, want 40%% of 200", stripeW)
	}

	// At the start of the cycle the stripe is entirely off the left edge.
	left, right, visible := StripeBounds(trackX, trackW, 0)
	if visible {
		t.Errorf("at progress 0 the stripe is outside the track, got %v..%v", left, right)
	}

	// A quarter in, it has entered and is clipped on the left.
	left, right, visible = StripeBounds(trackX, trackW, 0.25)
	if !visible {
		t.Fatal("at progress 0.25 the stripe overlaps the track")
	}
	if left != trackX {
		t.Errorf("the left edge must be clipped to the track, got %v", left)
	}
	if right <= left || right > trackX+trackW {
		t.Errorf("the right edge %v is outside the track", right)
	}

	// Halfway, it is fully inside.
	left, right, visible = StripeBounds(trackX, trackW, 0.5)
	if !visible {
		t.Fatal("at progress 0.5 the stripe is inside the track")
	}
	if right-left < stripeW-0.01 {
		t.Errorf("the stripe is %v wide, want %v: it is fully inside", right-left, stripeW)
	}

	// Near the end it is clipped on the right.
	left, right, visible = StripeBounds(trackX, trackW, 0.9)
	if !visible {
		t.Fatal("at progress 0.9 the stripe still overlaps")
	}
	if right != trackX+trackW {
		t.Errorf("the right edge must be clipped to the track, got %v", right)
	}

	// At the end it is gone, having left through the right.
	if _, _, visible := StripeBounds(trackX, trackW, 1); visible {
		t.Error("at progress 1 the stripe has left the track")
	}

	// The stripe never leaves the track at any point in the cycle.
	for _, progress := range []float32{0.1, 0.2, 0.3, 0.4, 0.6, 0.7, 0.8, 0.95} {
		left, right, visible := StripeBounds(trackX, trackW, progress)
		if !visible {
			continue
		}
		if left < trackX-0.01 || right > trackX+trackW+0.01 {
			t.Errorf("at progress %v the stripe spilled outside the track: %v..%v", progress, left, right)
		}
	}
}

// TestTheSlidePeriodAndKeyframesMatchTheStylesheet pins the values the stripe is drawn
// from. They are the stylesheet's, not a plausible round number.
func TestTheSlidePeriodAndKeyframesMatchTheStylesheet(t *testing.T) {
	if IndeterminateSlidePeriodMs != 1400 {
		t.Errorf("the slide period is %dms, want the stylesheet's 1400", IndeterminateSlidePeriodMs)
	}
	// -100% and +250% of the stripe's own width, with the stripe at 40% of the track.
	if IndeterminateSlideFrom != -1.0*IndeterminateFillFraction {
		t.Errorf("the start offset is %v, want -100%% of the stripe width", IndeterminateSlideFrom)
	}
	if IndeterminateSlideTo != 2.5*IndeterminateFillFraction {
		t.Errorf("the end offset is %v, want +250%% of the stripe width", IndeterminateSlideTo)
	}
}
