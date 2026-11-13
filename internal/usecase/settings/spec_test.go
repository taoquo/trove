package settings

import "testing"

// TestTheLayoutMatchesTheSource pins the widths and bounds to the shell's layout.
//
// The left column is magpie's: `--side-w: 212px`, dragged inside
// `Math.min(320, Math.max(168, x))` and capped at 36% of the window. The inspector is this
// app's own, so its default 360 and bounds 320/520 are the app's.
func TestTheLayoutMatchesTheSource(t *testing.T) {
	want := []struct {
		name  string
		got   float32
		value float32
		from  string
	}{
		{"left default", DefaultLeftSidebarWidth, 212, "left column default 212"},
		{"right default", DefaultRightSidebarWidth, 360, "inspector default 360"},
		{"left min", MinSidebarWidth, 168, "left minimum 168"},
		{"left max", MaxSidebarWidth, 320, "left maximum 320"},
		{"right min", MinInspectorWidth, 320, "Math.max(320, ...)"},
		{"right max", MaxInspectorWidth, 520, "Math.min(520, ...)"},
	}
	for _, item := range want {
		if item.got != item.value {
			t.Errorf("%s = %v, want %v (from %s)", item.name, item.got, item.value, item.from)
		}
	}
}

// TestTheDefaultsSitInsideTheBounds keeps a default from being unreachable.
func TestTheDefaultsSitInsideTheBounds(t *testing.T) {
	if DefaultLeftSidebarWidth < MinSidebarWidth || DefaultLeftSidebarWidth > MaxSidebarWidth {
		t.Errorf("the left default %v is outside [%v, %v]",
			DefaultLeftSidebarWidth, MinSidebarWidth, MaxSidebarWidth)
	}
	if DefaultRightSidebarWidth < MinInspectorWidth || DefaultRightSidebarWidth > MaxInspectorWidth {
		t.Errorf("the right default %v is outside [%v, %v]",
			DefaultRightSidebarWidth, MinInspectorWidth, MaxInspectorWidth)
	}
}
