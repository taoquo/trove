package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/taoquo/x-master/apps/macos/internal/autotag"
	"github.com/taoquo/x-master/apps/macos/internal/i18n"
)

// The auto-tagging progress bar, from src/ui/components.tsx and the stylesheet.
//
// The source answers three questions with one widget: how far along the run is, whether
// it is waiting on the provider, and whether it has stopped. The last two are why a
// percentage is not enough -- as its own comment puts it, "a bar frozen at 40% for a
// minute is indistinguishable from a dead run".
const (
	// ProgressTrackHeight is the stylesheet's track height.
	ProgressTrackHeight = 6
	// ProgressTrackRadius is the stylesheet's 9999px, i.e. fully rounded.
	ProgressTrackRadius = 9999
	// ProgressGap and ProgressPadding are the container's gap and padding.
	ProgressGap     = 6
	ProgressPadding = 10

	// IndeterminateFillFraction is the width of the indeterminate stripe, from the
	// stylesheet's 40%.
	IndeterminateFillFraction = 0.4
	// IndeterminateSlidePeriodMs is the stylesheet's 1.4s, ease-in-out, infinite.
	IndeterminateSlidePeriodMs = 1400
	// The keyframes move the stripe from -100% to +250% of its own width, which in
	// track-relative terms is from -0.4 track-widths to +1.0. Written out rather than
	// derived so the two endpoints can be read against the stylesheet.
	IndeterminateSlideFrom = -0.4
	IndeterminateSlideTo   = 1.0
	// ReducedMotionFillFraction and ReducedMotionFillOpacity are the stylesheet's
	// replacement: under prefers-reduced-motion the stripe is not animated, it is drawn
	// full width at 60% opacity. Drawing nothing instead -- which this did before --
	// showed an empty track where the source shows a state.
	ReducedMotionFillFraction = 1.0
	ReducedMotionFillOpacity  = 0.6
)

// RunProgressView is a run rendered for the panel.
//
// It is a value rather than a list of lines because the bar needs the fraction and the
// indeterminate flag, which a line of text cannot carry.
type RunProgressView struct {
	Phase string
	// Count is "processed/total", or empty when the total is unknown.
	Count string
	// StatusLine answers "what is it doing", in the source's precedence order.
	StatusLine string
	// Foot is the elapsed and estimated-remaining line.
	Foot string
	// Detail is what the pass has produced so far, empty until it has produced
	// something. It is its own line because it is the answer to "is it doing
	// anything", which a count of processed bookmarks does not give.
	Detail string
	// CancelLabel is the button's label, and Cancelling disables it.
	CancelLabel string
	Cancelling  bool

	// Percent is what the fill is drawn at, and is meaningless when Indeterminate.
	Percent int
	// Indeterminate is set while waiting or stalled. The source reports no value then,
	// so assistive technology does not announce a number that means nothing -- and the
	// same reasoning applies to drawing a fill that claims a position.
	Indeterminate bool
	Stalled       bool
}

// liveRunState is the run the shell should describe: the capture seam when a screenshot
// set one, otherwise the service's current state. The sidebar and the pages ask this one
// question, so they cannot disagree about whether a run is on screen.
func (s *Shell) liveRunState() *autotag.RunState {
	if s.captureRun != nil {
		return s.captureRun
	}
	if s.tagging != nil {
		return s.tagging.CurrentRun()
	}
	return nil
}

// RunProgressViewFor renders a live run.
//
// The status precedence is the source's and matters: a cancelling run that is also
// stalled must say it is stopping, because that is the thing the user just asked for.
// Checking the stall first would replace their own action with a diagnosis.
func (s *Shell) RunProgressViewFor(state *autotag.RunState, now time.Time) (RunProgressView, bool) {
	progress := autotag.BuildRunProgress(state, now)
	if !progress.Active {
		return RunProgressView{}, false
	}

	view := RunProgressView{Percent: progress.Percent}

	view.Phase = s.t().T(i18n.RunPhaseHeuristics)
	if progress.Phase == autotag.PhaseLLM {
		view.Phase = s.t().T(i18n.RunPhaseLlm)
	}

	if progress.Total > 0 {
		view.Count = fmt.Sprintf("%d/%d", progress.Processed, progress.Total)
	}

	switch {
	case progress.Status == autotag.StatusCancelling || (state != nil && state.CancelRequested):
		view.StatusLine = s.t().T(i18n.RunStopping)
		view.CancelLabel = s.t().T(i18n.RunStopping)
		view.Cancelling = true
		view.Indeterminate = true
	case progress.HasWaitingLeft:
		// Waiting for a provider's back-off is not a stall, and saying so would train
		// the user to ignore the message.
		view.StatusLine = fmt.Sprintf("%s %ds", s.t().T(i18n.RunWaiting), progress.WaitingSecondsLeft)
		view.CancelLabel = s.t().T(i18n.ActionStopRun)
		view.Indeterminate = true
	case progress.InFlightWaiting:
		// A request is out and the heartbeat is old, which is what a slow model looks
		// like. Not a stall: saying "it may have stopped" about a working run was the
		// single most misleading line this block drew.
		view.StatusLine = fmt.Sprintf("%s %ds", s.t().T(i18n.RunWaiting), progress.SecondsSinceHeartbeat)
		view.CancelLabel = s.t().T(i18n.ActionStopRun)
	case progress.Stalled:
		view.StatusLine = s.t().Tf(i18n.RunStalled, progress.SecondsSinceHeartbeat)
		view.CancelLabel = s.t().T(i18n.ActionStopRun)
		view.Indeterminate = true
		view.Stalled = true
	default:
		view.StatusLine = s.t().T(i18n.RunAnalyzing)
		view.CancelLabel = s.t().T(i18n.ActionStopRun)
	}

	view.Foot = s.t().Tf(i18n.RunElapsed, int(progress.Elapsed.Seconds()))
	switch {
	case progress.HasETA:
		view.Foot += " · " + s.t().Tf(i18n.RunRemaining, int(progress.ETA.Seconds()))
	case progress.HasBudgetLeft && progress.BudgetSecondsLeft > 0:
		// The pass cannot finish the backlog, so the useful number is when this pass
		// stops; the summary says the rest can continue.
		view.Foot += " · " + s.t().Tf(i18n.RunBudgetRemaining, progress.BudgetSecondsLeft)
	}

	if progress.Suggested > 0 {
		view.Detail = s.t().Tf(i18n.RunSuggestionsMade, progress.Suggested)
		if len(progress.RecentTags) > 0 {
			listSeparator := ", "
			if s.t().Locale() == i18n.ZHCN {
				listSeparator = "、"
			}
			view.Detail += " · " +
				s.t().Tf(i18n.RunRecentTags, strings.Join(progress.RecentTags, listSeparator))
		}
	}

	return view, true
}

// ProgressFillFraction is how much of the track to fill.
//
// An indeterminate bar carries no position: returning the last known fraction would be
// drawing a value the run cannot vouch for. It shows a stripe instead, and the stripe
// takes a different form when the user has asked for less motion.
func (v RunProgressView) ProgressFillFraction(reduceMotion bool) float32 {
	if v.Indeterminate {
		if reduceMotion {
			return ReducedMotionFillFraction
		}
		return IndeterminateFillFraction
	}
	return float32(v.Percent) / 100
}

// StripeBounds is the stripe's horizontal extent within the track, for one frame.
//
// It reproduces `overflow: hidden`: the stripe is drawn only where it overlaps the
// track, so it enters at the left edge and leaves at the right rather than being painted
// across the row.
//
// progress is the 0..1 value from the animation, which the caller obtains from MyGo's
// Loop so that frames keep arriving while it moves -- and so that a user who has asked
// for less motion is not given a looping animation at all.
func StripeBounds(trackX, trackW float32, progress float32) (float32, float32, bool) {
	stripeW := trackW * IndeterminateFillFraction
	left := trackX + trackW*(IndeterminateSlideFrom+(IndeterminateSlideTo-IndeterminateSlideFrom)*progress)
	right := left + stripeW

	// Clip to the track.
	if left < trackX {
		left = trackX
	}
	if right > trackX+trackW {
		right = trackX + trackW
	}
	if right <= left {
		return 0, 0, false
	}
	return left, right, true
}

// FillOpacity is the fill's alpha, which only differs in the reduced-motion
// indeterminate form.
func (v RunProgressView) FillOpacity(reduceMotion bool) float32 {
	if v.Indeterminate && reduceMotion {
		return ReducedMotionFillOpacity
	}
	return 1
}
