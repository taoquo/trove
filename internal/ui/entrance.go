package ui

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// The overlays' entrance animations, from the stylesheet's six @keyframes.
//
// Each is a fade from nothing plus, in four cases, a small transform: a scale, a lift,
// or both. They are recorded as the keyframes state them -- 0.96 and 0.985 are not
// approximations of each other, and a toast lifts 4px where a lightbox lifts 10px.
type Entrance struct {
	// Keyframe is the stylesheet's animation name.
	Keyframe string
	// DurationMs is the animation's duration.
	DurationMs int
	// Curve is the easing, as the stylesheet writes it.
	Curve string
	// ScaleFrom is the starting scale, and is 1 when the animation does not scale.
	ScaleFrom float32
	// OffsetYFrom is the starting vertical offset in pixels, and is 0 when the
	// animation does not lift.
	OffsetYFrom float32
	// CenteredX is set for the toast, whose keyframe translates from a -50% x offset so
	// the transform it already carries is centred rather than adding an entrance of its
	// own. The spelling follows the fixture's json key, which decides whether the value
	// is read at all.
	CenteredX bool
}

// Every entrance the interface uses.
var (
	// EntranceDetailCard is a fade and a small scale.
	EntranceDetailCard = Entrance{Keyframe: "options-detail-card-in", DurationMs: AnimateDetailCardMs, Curve: CurveDecelerate, ScaleFrom: 0.96}
	// EntranceToast is a fade and a 4px lift.
	EntranceToast = Entrance{Keyframe: "options-detail-toast-in", DurationMs: AnimateDetailToastMs, Curve: CurveDecelerate, ScaleFrom: 1, OffsetYFrom: 4, CenteredX: true}
	// EntranceLightbox is a fade, a 10px lift and a scale.
	EntranceLightbox = Entrance{Keyframe: "options-lightbox-pop", DurationMs: AnimateLightboxContentMs, Curve: CurveEmphasized, ScaleFrom: 0.96, OffsetYFrom: 10}
	// EntranceLightboxImage is a fade and a 0.985 scale.
	EntranceLightboxImage = Entrance{Keyframe: "options-lightbox-image-in", DurationMs: AnimateLightboxImageMs, Curve: CurveEaseOut, ScaleFrom: 0.985}
	// EntranceLightboxBackdrop is the fade at the lightbox's own duration and curve: the
	// same keyframes, different numbers.
	EntranceLightboxBackdrop = Entrance{Keyframe: "options-fade-in", DurationMs: AnimateLightboxBackdropMs, Curve: CurveEmphasized, ScaleFrom: 1}
)

// AllEntrances is every entrance, for the drift check.
var AllEntrances = []Entrance{
	EntranceDetailCard, EntranceToast,
	EntranceLightbox, EntranceLightboxImage, EntranceLightboxBackdrop,
}

// Duration is the animation's length.
func (e Entrance) Duration() time.Duration {
	return time.Duration(e.DurationMs) * time.Millisecond
}

// Which entrances are suppressed under reduced motion, and the divergence taken here: the
// three detail entrances are reduced but the lightbox's is not, which leaves a user who
// asked for less motion still watching the lightbox pop. That is a gap rather than a
// decision, so the lightbox is suppressed here too. The caller reads
// Preferences().ReduceMotion; there is no method on Entrance that answers it, because one
// that always returned true said nothing while looking like a check.
// OpacityAt is the overlay's opacity partway through its entrance.
//
// It is a plain interpolation rather than MyGo's Animate because the caller already
// holds the animated progress; keeping the mapping here makes it checkable against the
// keyframes, which state opacity 0 to 1 and nothing else.
func (e Entrance) OpacityAt(progress float32) float32 {
	if progress <= 0 {
		return 0
	}
	if progress >= 1 {
		return 1
	}
	return progress
}

// ScaleAt is the overlay's scale partway through its entrance.
func (e Entrance) ScaleAt(progress float32) float32 {
	if progress <= 0 {
		return e.ScaleFrom
	}
	if progress >= 1 {
		return 1
	}
	return e.ScaleFrom + (1-e.ScaleFrom)*progress
}

// OffsetYAt is the overlay's vertical offset partway through its entrance.
func (e Entrance) OffsetYAt(progress float32) float32 {
	if progress <= 0 {
		return e.OffsetYFrom
	}
	if progress >= 1 {
		return 0
	}
	return e.OffsetYFrom * (1 - progress)
}

// EntranceProgress drives an overlay's entrance and returns how far it has come.
//
// It must be called on an element that is built whether or not the overlay is open.
// MyGo starts a new animation at its first target, so an animation created on the frame
// an overlay first opens would begin already finished and never play. Driving the value
// on the persistent root instead means opening animates 0 to 1 and closing animates
// back, which is the same shape MyGo's own collapsible uses.
//
// Under reduceMotion the value goes to its target at once, which is what the framework
// does for Animate and what the stylesheet asks for.
func EntranceProgress(root ui.Element, key string, open bool, e Entrance, reduceMotion bool) float32 {
	if reduceMotion {
		if open {
			return 1
		}
		return 0
	}

	target := float32(0)
	if open {
		target = 1
	}
	return root.Animate(key, target, e.Duration())
}

// Toast rendering values.
const (
	// ToastBottomOffset is the pill's bottom inset, 74 points.
	ToastBottomOffset = 74
	// ToastRadius is the fully rounded pill.
	ToastRadius = 999
	// ToastPaddingX and ToastPaddingY are the pill's 12/8 padding.
	ToastPaddingX = 12
	ToastPaddingY = 8
	// ToastFontSize is the pill's 12-point label.
	ToastFontSize = 12
	// ToastDismissAfter is the component's own timeout, which clears the message
	// whether or not anything else happens.
	ToastDismissAfter = 2200 * time.Millisecond
	// The other tones live longer, from magpie's status line: 8s for a failure and
	// 3.5s for a state that is neither success nor failure, with 15s the ceiling for a
	// long sentence at 80ms a character.
	ToastWarnAfter   = 6000 * time.Millisecond
	ToastErrorAfter  = 8000 * time.Millisecond
	ToastMaxAfter    = 15000 * time.Millisecond
	ToastReadPerChar = 80 * time.Millisecond
	// ToastLiftBottom and ToastMaxWidth are the lifted pill's inset and cap
	// (`.status.lift`, app.css:2242, :2255).
	ToastLiftBottom = 18
	ToastMaxWidth   = 560
)
