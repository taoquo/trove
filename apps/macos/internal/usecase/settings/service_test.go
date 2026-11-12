package settings

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/taoquo/trove/apps/macos/internal/ports"
	"github.com/taoquo/trove/apps/macos/internal/storage/sqlite"
)

var clockBase = time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

func newService(t *testing.T) (*Service, *sqlite.DB) {
	t.Helper()

	db, err := sqlite.Open(filepath.Join(t.TempDir(), "settings.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	repos := ports.Repositories{Settings: sqlite.NewSettingsRepository(db)}
	return New(repos, fixedClock{now: clockBase}), db
}

func TestFreshInstallUsesDefaults(t *testing.T) {
	service, _ := newService(t)

	preferences, err := service.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if preferences.Appearance != AppearanceSystem {
		t.Errorf("appearance = %q, want system", preferences.Appearance)
	}
	if preferences.Locale != "" {
		t.Errorf("locale = %q, want empty (follow the system)", preferences.Locale)
	}
}

func TestPreferencesRoundTrip(t *testing.T) {
	service, _ := newService(t)
	ctx := context.Background()

	saved := Preferences{
		Appearance: AppearanceDark,
		Locale:     "zh-CN",
		Sync:       SyncPreferences{PageLimit: 5000, SyncOnLaunch: true, SaveRawPayloads: true},
	}
	if err := service.Save(ctx, saved); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := service.Load(ctx)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Appearance != AppearanceDark || loaded.Locale != "zh-CN" {
		t.Errorf("loaded = %+v", loaded)
	}
	if loaded.Sync.PageLimit != 5000 || !loaded.Sync.SyncOnLaunch || !loaded.Sync.SaveRawPayloads {
		t.Errorf("sync preferences = %+v", loaded.Sync)
	}
}

func TestSaveRejectsUnknownValues(t *testing.T) {
	service, _ := newService(t)
	ctx := context.Background()

	if err := service.Save(ctx, Preferences{Appearance: "sepia"}); err == nil {
		t.Error("an unknown appearance must be refused on write")
	}
	if err := service.Save(ctx, Preferences{Appearance: AppearanceLight, Locale: "fr"}); err == nil {
		t.Error("an unsupported locale must be refused on write")
	}
	if err := service.Save(ctx, Preferences{Appearance: AppearanceLight, Sync: SyncPreferences{PageLimit: -1}}); err == nil {
		t.Error("a negative page limit must be refused")
	}
}

// TestUnreadableStoredValueFallsBack covers an upgrade: a blob from another build
// must not stop the app from opening.
func TestUnreadableStoredValueFallsBack(t *testing.T) {
	service, db := newService(t)
	ctx := context.Background()

	if err := sqlite.NewSettingsRepository(db).Set(ctx, preferencesKey, "{not json", clockBase); err != nil {
		t.Fatalf("seed: %v", err)
	}

	preferences, err := service.Load(ctx)
	if err != nil {
		t.Fatalf("Load must not fail on a bad blob: %v", err)
	}
	if preferences.Appearance != AppearanceSystem {
		t.Errorf("appearance = %q, want the default", preferences.Appearance)
	}
}

// TestUnknownStoredAppearanceFallsBack is the other half: valid JSON holding a
// value this build does not know.
func TestUnknownStoredAppearanceFallsBack(t *testing.T) {
	service, db := newService(t)
	ctx := context.Background()

	blob := `{"appearance":"neon","locale":"en"}`
	if err := sqlite.NewSettingsRepository(db).Set(ctx, preferencesKey, blob, clockBase); err != nil {
		t.Fatalf("seed: %v", err)
	}

	preferences, err := service.Load(ctx)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if preferences.Appearance != AppearanceSystem {
		t.Errorf("appearance = %q, want the default after an unknown value", preferences.Appearance)
	}
}

// TestExportWithholdsSensitiveKeys is the safety property that makes a shared
// backup safe: a custom endpoint is often a private gateway, and it must never
// travel with the file.
func TestExportWithholdsSensitiveKeys(t *testing.T) {
	service, _ := newService(t)
	ctx := context.Background()

	if err := service.Save(ctx, Preferences{Appearance: AppearanceDark, Locale: "en"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := service.SaveEndpoints(ctx, map[string]string{
		"custom": "https://internal-gateway.corp.example/v1",
	}); err != nil {
		t.Fatalf("SaveEndpoints: %v", err)
	}

	exported, err := service.Export(ctx)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	if _, present := exported[llmEndpointsKey]; present {
		t.Error("the endpoints key must never be exported")
	}
	for _, value := range exported {
		if value == "" {
			continue
		}
		if strings.Contains(value, "internal-gateway") {
			t.Errorf("an endpoint leaked into the export: %q", value)
		}
	}

	// The preferences do travel.
	if _, present := exported[preferencesKey]; !present {
		t.Error("the preferences must be exportable")
	}

	// And the user can be told what was withheld.
	withheld, err := service.Withheld(ctx)
	if err != nil {
		t.Fatalf("Withheld: %v", err)
	}
	if len(withheld) != 1 || withheld[0] != llmEndpointsKey {
		t.Errorf("withheld = %v, want the endpoints key", withheld)
	}
}

// TestExportIsAnAllowlist pins the direction of the default: a key nobody has
// classified is withheld, not shared.
func TestExportIsAnAllowlist(t *testing.T) {
	service, db := newService(t)
	ctx := context.Background()

	// A key added by some future build.
	if err := sqlite.NewSettingsRepository(db).Set(ctx, "something.new", `"secret"`, clockBase); err != nil {
		t.Fatalf("seed: %v", err)
	}

	exported, err := service.Export(ctx)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if _, present := exported["something.new"]; present {
		t.Error("an unclassified key must be withheld by default")
	}
}

func TestEndpointsRoundTrip(t *testing.T) {
	service, _ := newService(t)
	ctx := context.Background()

	if err := service.SaveEndpoints(ctx, map[string]string{
		"custom":    "  https://gateway.example/v1  ",
		"":          "https://ignored.example",
		"anthropic": "   ",
	}); err != nil {
		t.Fatalf("SaveEndpoints: %v", err)
	}

	endpoints, err := service.Endpoints(ctx)
	if err != nil {
		t.Fatalf("Endpoints: %v", err)
	}
	if len(endpoints) != 1 {
		t.Fatalf("endpoints = %v, want only the one complete pair", endpoints)
	}
	if endpoints["custom"] != "https://gateway.example/v1" {
		t.Errorf("endpoint = %q, want it trimmed", endpoints["custom"])
	}
}

func TestValidators(t *testing.T) {
	for _, want := range []string{"system", "light", "dark"} {
		if !ValidAppearance(want) {
			t.Errorf("%q must be valid", want)
		}
	}
	if ValidAppearance("sepia") {
		t.Error("an unknown appearance must be rejected")
	}

	// An empty locale means "follow the system", which is valid.
	if !ValidLocale("") {
		t.Error("an empty locale must be valid")
	}
	for _, want := range []string{"zh-CN", "en"} {
		if !ValidLocale(want) {
			t.Errorf("%q must be valid", want)
		}
	}
	if ValidLocale("fr") {
		t.Error("an unsupported locale must be rejected")
	}
}

func TestClassificationOfKeys(t *testing.T) {
	if !IsExportable(preferencesKey) {
		t.Error("the preferences must be exportable")
	}
	if IsExportable(llmEndpointsKey) {
		t.Error("the endpoints must not be exportable")
	}
	if !IsSensitive(llmEndpointsKey) {
		t.Error("the endpoints must be classified as sensitive")
	}
	// Nothing may be both.
	for _, key := range ExportableKeys {
		if IsSensitive(key) {
			t.Errorf("key %q is both exportable and sensitive", key)
		}
	}
}

// TestLayoutWidthsPersist covers the draggable rails: a width the user drags is a
// preference, and a preference that does not survive a relaunch is not one.
func TestLayoutWidthsPersist(t *testing.T) {
	service, _ := newService(t)
	ctx := context.Background()

	// A fresh install uses the stylesheet's defaults.
	loaded, err := service.Load(ctx)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Layout.LeftSidebarWidth != DefaultLeftSidebarWidth {
		t.Errorf("left width = %v, want the default %v", loaded.Layout.LeftSidebarWidth, DefaultLeftSidebarWidth)
	}
	if loaded.Layout.RightSidebarWidth != DefaultRightSidebarWidth {
		t.Errorf("right width = %v, want the default %v", loaded.Layout.RightSidebarWidth, DefaultRightSidebarWidth)
	}

	// A dragged width survives.
	loaded.Layout.LeftSidebarWidth = 260
	loaded.Layout.RightSidebarWidth = 400
	if err := service.Save(ctx, loaded); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// This read is what a relaunch does.
	again, err := service.Load(ctx)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if again.Layout.LeftSidebarWidth != 260 {
		t.Errorf("left width = %v, want 260", again.Layout.LeftSidebarWidth)
	}
	if again.Layout.RightSidebarWidth != 400 {
		t.Errorf("right width = %v, want 400", again.Layout.RightSidebarWidth)
	}
}

// TestLayoutWidthsAreClamped covers the three ways a bad width arrives: an older
// build, a hand-edited database, or a bundle from a larger display. A width outside
// the bounds would produce a column that cannot be dragged back.
func TestLayoutWidthsAreClamped(t *testing.T) {
	for _, tc := range []struct {
		name        string
		left, right float32
		wantLeft    float32
		wantRight   float32
	}{
		{"below the minimum", 10, 10, MinSidebarWidth, MinInspectorWidth},
		{"above the maximum", 9999, 9999, MaxSidebarWidth, MaxInspectorWidth},
		{"absent falls back to the default", 0, 0, DefaultLeftSidebarWidth, DefaultRightSidebarWidth},
		{"in range is untouched", 260, 400, 260, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := NormalizeLayout(LayoutPreferences{
				LeftSidebarWidth: tc.left, RightSidebarWidth: tc.right,
			})
			if got.LeftSidebarWidth != tc.wantLeft {
				t.Errorf("left = %v, want %v", got.LeftSidebarWidth, tc.wantLeft)
			}
			if got.RightSidebarWidth != tc.wantRight {
				t.Errorf("right = %v, want %v", got.RightSidebarWidth, tc.wantRight)
			}
		})
	}

	// A stored out-of-range value is corrected on read rather than trusted.
	service, _ := newService(t)
	ctx := context.Background()
	if err := service.Save(ctx, Preferences{
		Appearance: AppearanceSystem,
		Layout:     LayoutPreferences{LeftSidebarWidth: 5, RightSidebarWidth: 100000},
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := service.Load(ctx)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Layout.LeftSidebarWidth != MinSidebarWidth {
		t.Errorf("left = %v, want it clamped to %v", loaded.Layout.LeftSidebarWidth, MinSidebarWidth)
	}
	if loaded.Layout.RightSidebarWidth != MaxInspectorWidth {
		t.Errorf("right = %v, want it clamped to %v", loaded.Layout.RightSidebarWidth, MaxInspectorWidth)
	}
}

// TestLayoutIsInTheExportedPreferences pins that the widths survive a backup, since
// a restored library that forgets the user's layout is a small but real annoyance.
func TestLayoutIsInTheExportedPreferences(t *testing.T) {
	service, _ := newService(t)
	ctx := context.Background()

	if err := service.Save(ctx, Preferences{
		Appearance: AppearanceDark,
		Layout:     LayoutPreferences{LeftSidebarWidth: 300, RightSidebarWidth: 380},
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	exported, err := service.Export(ctx)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	raw, present := exported[preferencesKey]
	if !present {
		t.Fatalf("the preferences key must be exported: %v", exported)
	}
	for _, want := range []string{"leftSidebarWidth", "rightSidebarWidth"} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("the exported preferences must carry %s: %s", want, raw)
		}
	}
}
