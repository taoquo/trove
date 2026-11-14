package ui

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// The motion language.
//
// The interface moves in three sizes: what the pointer touches changes over 120-180ms, a page
// enters over 240ms, and a change of layout -- the sidebar's fold, the toggle's pane --
// takes 300ms. There is one curve, cubic-bezier(.2,.8,.2,1), and one slower, spring-like
// curve for the nav highlight.
//
// Nothing here decorates. Every use belongs to a state that changed: a control under the
// pointer, a row becoming selected, a page being replaced. The intensity is deliberately
// restrained -- 120-300ms, short lifts, no idle animation except the two the reader is
// waiting on (the loading skeleton and a running sync).
const (
	MotionFastMs = 120
	MotionBaseMs = 180
	MotionSlowMs = 300
	// MotionPageMs is the page entrance's 0.24s.
	MotionPageMs = 240
	// PageEnterOffset is the page entrance's translateY(5px).
	PageEnterOffset = 5
	// MotionPressMs is a pressed control's give: the global control rules
	// transition transform over .16s.
	MotionPressMs = 160
	// MotionPressSegMs is a segmented button's quicker .14s.
	MotionPressSegMs = 140
	// MotionInkMs is a sub-row's colour changing under the pointer: a sub-row
	// transitions color over .2s.
	MotionInkMs = 200
)

// CopiedFeedbackFor is how long the copy chip holds its confirmation. Long enough to be
// seen, short enough that it is gone before the reader wants to copy again.
const CopiedFeedbackFor = 900 * time.Millisecond

// MotionEase is the standard curve.
var MotionEase = ui.CubicBezier(0.2, 0.8, 0.2, 1)

// MotionEaseOut is the slower settle curve, the one the selection glides along.
var MotionEaseOut = ui.CubicBezier(0.23, 1, 0.32, 1)

var (
	// MotionFast is a control's colour changing under the pointer: nav rows and
	// icon buttons both use 120-150ms.
	MotionFast = ui.Animation{Duration: MotionFastMs * time.Millisecond, Ease: MotionEase}
	// MotionBase is a row arriving at its selected state.
	MotionBase = ui.Animation{Duration: MotionBaseMs * time.Millisecond, Ease: MotionEase}
	// MotionSlow is the sidebar's button filling its left pane: .3s.
	MotionSlow = ui.Animation{Duration: MotionSlowMs * time.Millisecond, Ease: MotionEase}
	// MotionPop is the overshoot of the copy pop, a scale from .55 through 1.16 to 1.
	MotionPop = ui.Snappy
	// MotionPress is a text-family control's give.
	MotionPress = ui.Animation{Duration: MotionPressMs * time.Millisecond, Ease: MotionEase}
	// MotionPressSeg is the quicker give of a segmented button.
	MotionPressSeg = ui.Animation{Duration: MotionPressSegMs * time.Millisecond, Ease: MotionEase}
	// MotionInk is a sub-row's ink changing under the pointer.
	MotionInk = ui.Animation{Duration: MotionInkMs * time.Millisecond, Ease: MotionEase}
	// MotionGlide is the sidebar's selection sliding to the row picked:
	// transform and height transition over .24s along the settle curve.
	MotionGlide = ui.Animation{Duration: MotionPageMs * time.Millisecond, Ease: MotionEaseOut}
)

// The two scales a press gives a control: the text family (buttons,
// tiles, chips, the segmented control) goes to .96, and icon buttons go to .9.
const (
	PressScaleText = 0.96
	PressScaleIcon = 0.9
)

// pressScale is the active transform: while the pointer holds the control it scales
// down, and lets go over its own duration.
//
// MyGo has no pointer-down event, but Pressed is the same question asked of the toolkit --
// it is true from the press until the pointer leaves or lifts -- so the factor is 1 except
// while the reader is holding the control. At rest it attaches no transform at all: a
// transform on every button would ride through each descendant's paint and hit test.
func pressScale(e ui.Element, key string, to float32, a ui.Animation) ui.Element {
	target := float32(1)
	if e.Pressed() {
		target = to
	}
	value := e.Animate(key, target, a.Duration)
	if value == 1 {
		return e
	}
	return e.Transform(ui.Scaling(value, value))
}

// animateColor is AnimateColor that honours the desktop's ReduceMotion setting.
//
// MyGo's AnimateColor deliberately does not read it (only Animate, Keyframes and a
// Transition's movement do; mygo's motion.go), so a reader who asked the system for less
// motion would still get 120ms of colour sliding under the pointer. The
// reduced-motion preference switches those transitions off, so under ReduceMotion the
// value must simply be the target.
func animateColor(c *ui.Context, e ui.Element, key string, target ui.Color, a ui.Animation) ui.Color {
	if c.Preferences().ReduceMotion {
		return target
	}
	return e.AnimateColor(key, target, a)
}

// hoverColor is the ground a control shows this frame, easing to its hover fill instead of
// switching: every control gets a `transition` on background and colour, and this is
// the MyGo equivalent of that one line. Under ReduceMotion it switches, as the
// reduced-motion media query does.
func hoverColor(c *ui.Context, e ui.Element, key string, base, hover ui.Color, hovered bool) ui.Color {
	if hovered {
		return animateColor(c, e, key, hover, MotionFast)
	}
	return animateColor(c, e, key, base, MotionFast)
}

// resultsEntrance is the results pane's partway-in state after a page change.
//
// Every view animates in over .24s -- a fade from nothing and a 5-point lift -- and, when
// the desktop asks for less motion, the fade alone, which keeps the fade and drops the
// lift. That is exactly the pair returned
// here. While the pane is still moving it asks for the next frame, which is what makes the
// value advance; MyGo's own Animate cannot be used because a fresh channel starts at its
// first target and the pane is rebuilt as the page changes.
func (s *Shell) resultsEntrance(c *ui.Context) (opacity, offset float32) {
	if s.state.PageChangedAt.IsZero() {
		return 1, 0
	}
	elapsed := c.Now().Sub(s.state.PageChangedAt)
	opacity, offset = pageEntrance(elapsed, c.Preferences().ReduceMotion)
	if elapsed < time.Duration(MotionPageMs)*time.Millisecond {
		// Still moving: the next frame is what advances the value.
		c.After(time.Second / 60)
	}
	return opacity, offset
}

// pageEntrance is the arithmetic of the page's entrance, apart from the clock so a test can
// ask it questions without a window.
func pageEntrance(elapsed time.Duration, reduceMotion bool) (opacity, offset float32) {
	duration := time.Duration(MotionPageMs) * time.Millisecond
	if elapsed >= duration {
		return 1, 0
	}
	if elapsed < 0 {
		elapsed = 0
	}
	progress := MotionEase(float32(elapsed) / float32(duration))
	offset = PageEnterOffset * (1 - progress)
	if reduceMotion {
		// The fade stays, the movement goes: the reduced-motion rule names fade-in.
		offset = 0
	}
	return progress, offset
}

// The popover's entrance: a fade over .14s on the ease-out keyword and a rise over .24s
// from 6 points above at .94, and, for a reader who asked for less motion, the .12s fade
// alone: the fade stays and the rise and the scale go.
const (
	PopFadeMs      = 140
	PopFadeReduced = 120
	PopInMs        = 240
	// PopInRise is the 6 points the panel starts above its place.
	PopInRise = 6
	// PopInScale is where it starts, growing to 1.
	PopInScale = 0.94
)

// popFadeEase is the ease-out keyword -- not the slower settle curve the rise uses.
var popFadeEase = ui.CubicBezier(0, 0, 0.58, 1)

// popoverEntrance is the popover's entrance at an elapsed time: the panel's opacity, how far above
// its place it still is, and its scale. It is arithmetic apart from the clock so a test can
// ask it questions without a window, as pageEntrance is.
func popoverEntrance(elapsed time.Duration, reduceMotion bool) (opacity, rise, scale float32) {
	if elapsed < 0 {
		elapsed = 0
	}
	fade := time.Duration(PopFadeMs) * time.Millisecond
	if reduceMotion {
		fade = time.Duration(PopFadeReduced) * time.Millisecond
	}
	opacity = 1
	if elapsed < fade {
		opacity = popFadeEase(float32(elapsed) / float32(fade))
	}
	scale = 1
	if reduceMotion {
		return opacity, 0, 1
	}
	move := time.Duration(PopInMs) * time.Millisecond
	if elapsed < move {
		progress := MotionEaseOut(float32(elapsed) / float32(move))
		rise = PopInRise * (1 - progress)
		scale = PopInScale + (1-PopInScale)*progress
	}
	return opacity, rise, scale
}

// filterPopoverEntrance is the popover's partway-in state this frame.
//
// It needs the same two things resultsEntrance does: the clock, because the panel is built
// fresh each frame, and a request for the next frame while it is still moving. `Animate`
// cannot be used for the same reason it cannot for a page -- a fresh channel starts at its
// first target.
func (s *Shell) filterPopoverEntrance(c *ui.Context) (opacity, rise, scale float32) {
	if s.state.FilterPopoverOpenedAt.IsZero() {
		return 1, 0, 1
	}
	elapsed := c.Now().Sub(s.state.FilterPopoverOpenedAt)
	opacity, rise, scale = popoverEntrance(elapsed, c.Preferences().ReduceMotion)
	if elapsed < time.Duration(PopInMs)*time.Millisecond {
		c.After(time.Second / 60)
	}
	return opacity, rise, scale
}

// pulse drives a repeating 0.55-to-1 breathing value, for the two waits that have no
// progress of their own: the loading skeleton and a sync in flight.
//
// Under reduced motion it is not started at all. The toolkit's reduced-motion path takes a
// sequence to its final value, and for an alternate loop that never ends the final value is
// the *first* frame (mygo motion.go's sequence.final: RepeatForever plays 0 times, and 0 is
// even) -- 0.55 here. The skeleton and the syncing dot were therefore frozen at 55% opacity
// for exactly the reader who asked for stillness: dimmer than the page around them, reading
// as disabled rather than as still. Returning 1 is the steady state.
func pulse(c *ui.Context, e ui.Element, key string) float32 {
	if c.Preferences().ReduceMotion {
		return 1
	}
	const half = 650 * time.Millisecond
	return e.Keyframes(key, []ui.Keyframe{
		{At: 0, Value: 0.55},
		{At: half, Value: 1, Ease: ui.EaseInOut},
	}, ui.KeyframeOptions{Repeat: ui.RepeatForever, Alternate: true})
}
