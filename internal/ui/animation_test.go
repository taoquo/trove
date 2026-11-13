package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// The entrance animations, checked against the stylesheet.
//
// The fixture is generated from the stylesheet, not transcribed, because
// these numbers disagree with the motion-duration tokens: the transitions use 200/180/
// 140 while the keyframes use 210/220/180/180/220/160. Copying one set over the other
// would look plausible and be wrong for six selectors.
type animationFixture struct {
	AnimationUses []struct {
		Selector   string `json:"selector"`
		Keyframes  string `json:"keyframes"`
		DurationMs int    `json:"durationMs"`
		Curve      string `json:"curve"`
	} `json:"animationUses"`
}

// animationMetrics maps each animated selector to the constant that records it.
var animationMetrics = map[string]struct {
	duration int
	curve    string
}{
	".options-detail-backdrop":         {AnimateDetailBackdropMs, CurveDecelerate},
	".options-detail-focus-card":       {AnimateDetailCardMs, CurveDecelerate},
	".options-detail-toast":            {AnimateDetailToastMs, CurveDecelerate},
	".options-media-lightbox-backdrop": {AnimateLightboxBackdropMs, CurveEmphasized},
	".options-media-lightbox-content":  {AnimateLightboxContentMs, CurveEmphasized},
	".options-media-lightbox-image":    {AnimateLightboxImageMs, CurveEaseOut},
}

func TestEntranceAnimationsMatchTheStylesheet(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "format", "format.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var fixture animationFixture
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	if len(fixture.AnimationUses) == 0 {
		t.Fatal("the fixture has no animations")
	}

	for _, use := range fixture.AnimationUses {
		metric, known := animationMetrics[use.Selector]
		if !known {
			t.Errorf("the stylesheet animates %s and no constant records it", use.Selector)
			continue
		}
		if metric.duration != use.DurationMs {
			t.Errorf("%s is %dms, recorded as %d", use.Selector, use.DurationMs, metric.duration)
		}
		if metric.curve != use.Curve {
			t.Errorf("%s uses %q, recorded as %q", use.Selector, use.Curve, metric.curve)
		}
	}

	// Every constant is used, so a selector renamed in the stylesheet shows up as a
	// stale constant rather than as silent coverage.
	seen := map[string]bool{}
	for _, use := range fixture.AnimationUses {
		seen[use.Selector] = true
	}
	for selector := range animationMetrics {
		if !seen[selector] {
			t.Errorf("%s is recorded but the stylesheet no longer animates it", selector)
		}
	}

	// The point of generating the fixture: the keyframe durations are not the motion
	// tokens. Some coincide by chance -- the toast is 180ms and so is the transform
	// transition -- so the check is that the sets differ, not that any pair does.
	tokenOnly := 0
	keyframes := []int{
		AnimateDetailBackdropMs, AnimateDetailCardMs, AnimateDetailToastMs,
		AnimateLightboxBackdropMs, AnimateLightboxContentMs, AnimateLightboxImageMs,
	}
	for _, duration := range []int{MotionWidthMs, MotionTransformMs, MotionFadeMs} {
		found := false
		for _, keyframe := range keyframes {
			if keyframe == duration {
				found = true
				break
			}
		}
		if !found {
			tokenOnly++
		}
	}
	if tokenOnly == 0 {
		t.Error("the keyframe durations and the motion tokens must not be the same set")
	}
	// The specific one that would have been copied across: the detail backdrop is
	// 210ms, not the width token's 200ms.
	if AnimateDetailBackdropMs == MotionWidthMs {
		t.Errorf("the backdrop keyframe is %dms, not the width token's %dms",
			AnimateDetailBackdropMs, MotionWidthMs)
	}
}
