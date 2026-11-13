package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type entranceFixture struct {
	KeyframeBodies map[string]struct {
		ScaleFrom   float32 `json:"scaleFrom"`
		OffsetYFrom float32 `json:"offsetYFrom"`
		CenteredX   bool    `json:"centeredX"`
		Fades       bool    `json:"fades"`
	} `json:"keyframeBodies"`
	AnimationUses []struct {
		Selector   string `json:"selector"`
		Keyframes  string `json:"keyframes"`
		DurationMs int    `json:"durationMs"`
		Curve      string `json:"curve"`
	} `json:"animationUses"`
}

// TestEntrancesMatchTheKeyframes checks the transforms against the stylesheet's own
// keyframes.
//
// The numbers matter and are not guessable: the detail card scales from 0.96, the
// lightbox image from 0.985, the toast lifts 4px and the lightbox 10px. Rounding them
// into one "scale up slightly" would look right and be wrong four times.
func TestEntrancesMatchTheKeyframes(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "format", "format.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var fixture entranceFixture
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	if len(fixture.KeyframeBodies) == 0 {
		t.Fatal("the fixture has no keyframes")
	}

	for _, entrance := range AllEntrances {
		body, known := fixture.KeyframeBodies[entrance.Keyframe]
		if !known {
			t.Errorf("%s names a keyframe the stylesheet does not declare", entrance.Keyframe)
			continue
		}
		if entrance.ScaleFrom != body.ScaleFrom {
			t.Errorf("%s scales from %v, the keyframes say %v", entrance.Keyframe, entrance.ScaleFrom, body.ScaleFrom)
		}
		if entrance.OffsetYFrom != body.OffsetYFrom {
			t.Errorf("%s lifts %vpx, the keyframes say %v", entrance.Keyframe, entrance.OffsetYFrom, body.OffsetYFrom)
		}
		if entrance.CenteredX != body.CenteredX {
			t.Errorf("%s centredX = %v, the keyframes say %v", entrance.Keyframe, entrance.CenteredX, body.CenteredX)
		}
		if !body.Fades {
			t.Errorf("%s does not fade, but every entrance does", entrance.Keyframe)
		}
	}

	// Every keyframe that is used as an entrance is modelled. The progress stripe is
	// the one that is not an entrance, and is excluded by name.
	modelled := map[string]bool{}
	for _, entrance := range AllEntrances {
		modelled[entrance.Keyframe] = true
	}
	for name := range fixture.KeyframeBodies {
		if name == "options-autotag-progress-slide" {
			continue
		}
		if !modelled[name] {
			t.Errorf("the stylesheet declares @keyframes %s and no entrance records it", name)
		}
	}
}

// TestEntranceCurvesAndDurationsComeFromTheAnimationUses keeps each entrance's numbers
// tied to the selector it belongs to: the same keyframe is used twice with different
// duration and curve.
func TestEntranceCurvesAndDurationsComeFromTheAnimationUses(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "format", "format.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var fixture entranceFixture
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}

	bySelector := map[string]struct {
		duration int
		curve    string
	}{}
	for _, use := range fixture.AnimationUses {
		bySelector[use.Selector] = struct {
			duration int
			curve    string
		}{use.DurationMs, use.Curve}
	}

	// options-fade-in is used by two selectors with different numbers, which is why
	// the duration is not a property of the keyframe.
	backdrop, ok := bySelector[".options-detail-backdrop"]
	if !ok {
		t.Fatal("the fixture is missing the detail backdrop")
	}
	lightbox, ok := bySelector[".options-media-lightbox-backdrop"]
	if !ok {
		t.Fatal("the fixture is missing the lightbox backdrop")
	}
	if backdrop.duration == lightbox.duration && backdrop.curve == lightbox.curve {
		t.Error("the two fade-in uses must differ, or the fixture is not distinguishing them")
	}
	if EntranceLightboxBackdrop.DurationMs != lightbox.duration || EntranceLightboxBackdrop.Curve != lightbox.curve {
		t.Errorf("the lightbox backdrop is %dms %s, recorded as %dms %s",
			lightbox.duration, lightbox.curve, EntranceLightboxBackdrop.DurationMs, EntranceLightboxBackdrop.Curve)
	}
}

// TestEntranceInterpolation covers the mapping from progress to the drawn values.
func TestEntranceInterpolation(t *testing.T) {
	card := EntranceDetailCard

	// At the start it is invisible at its starting scale.
	if got := card.OpacityAt(0); got != 0 {
		t.Errorf("opacity at 0 = %v", got)
	}
	if got := card.ScaleAt(0); got != 0.96 {
		t.Errorf("scale at 0 = %v, want 0.96", got)
	}
	// At the end it is fully visible at scale 1.
	if got := card.OpacityAt(1); got != 1 {
		t.Errorf("opacity at 1 = %v", got)
	}
	if got := card.ScaleAt(1); got != 1 {
		t.Errorf("scale at 1 = %v", got)
	}
	// Halfway it is between the two, not snapped to either.
	if got := card.ScaleAt(0.5); got <= 0.96 || got >= 1 {
		t.Errorf("scale at 0.5 = %v, want between 0.96 and 1", got)
	}

	// The lift interpolates to nothing.
	if got := EntranceLightbox.OffsetYAt(0); got != 10 {
		t.Errorf("the lightbox starts lifted 10px, got %v", got)
	}
	if got := EntranceLightbox.OffsetYAt(1); got != 0 {
		t.Errorf("the lightbox ends unlifted, got %v", got)
	}
	if got := EntranceLightbox.OffsetYAt(0.5); got != 5 {
		t.Errorf("halfway the lightbox is lifted %v, want 5", got)
	}

}

// TestEntranceProgressGoesBothWays covers the idiom the application depends on.
//
// An animation created on the frame an overlay first opens would start at its target
// and never play, which is why the value is driven on an element built in both states.
// The progression here is the interpolation, which is what the caller applies.
func TestEntranceProgressInterpolates(t *testing.T) {
	card := EntranceDetailCard

	// Closed: invisible at the starting scale.
	if got := card.OpacityAt(0); got != 0 {
		t.Errorf("closed opacity = %v, want 0", got)
	}
	// Open: fully visible at scale 1.
	if got := card.OpacityAt(1); got != 1 {
		t.Errorf("open opacity = %v, want 1", got)
	}
	// Partway: strictly between, so the overlay is genuinely mid-animation rather than
	// snapped to either end.
	for _, progress := range []float32{0.1, 0.25, 0.5, 0.75, 0.9} {
		opacity := card.OpacityAt(progress)
		if opacity <= 0 || opacity >= 1 {
			t.Errorf("opacity at %v = %v, want strictly between 0 and 1", progress, opacity)
		}
		scale := card.ScaleAt(progress)
		if scale <= card.ScaleFrom || scale >= 1 {
			t.Errorf("scale at %v = %v, want between %v and 1", progress, scale, card.ScaleFrom)
		}
	}

	// Out-of-range values clamp rather than overshooting, which is what a frame that
	// arrives late would otherwise draw.
	if got := card.OpacityAt(-1); got != 0 {
		t.Errorf("opacity below 0 = %v, want 0", got)
	}
	if got := card.OpacityAt(2); got != 1 {
		t.Errorf("opacity above 1 = %v, want 1", got)
	}
	if got := card.ScaleAt(-1); got != card.ScaleFrom {
		t.Errorf("scale below 0 = %v, want %v", got, card.ScaleFrom)
	}
	if got := card.ScaleAt(2); got != 1 {
		t.Errorf("scale above 1 = %v, want 1", got)
	}
}
