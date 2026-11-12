package main

// The native half of the parity rig.
//
// tools/parity/render-source.mjs dumps every rendered element's box from the
// extension; this dumps the native shell's text boxes the same way, from the
// same fixture database, at the same size. tools/parity/compare.py then reports
// the distance between the two instead of asking a person to eyeball two pictures.
//
// It is inert unless XBM_PARITY_DB and XBM_PARITY_OUT are set, because it needs
// a database seeded by cmd/seed and it writes a file -- neither belongs in a
// normal go test run.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/taoquo/x-master/apps/macos/internal/idgen"
	"github.com/taoquo/x-master/apps/macos/internal/media"
	"github.com/taoquo/x-master/apps/macos/internal/ports"
	"github.com/taoquo/x-master/apps/macos/internal/storage/sqlite"
	appui "github.com/taoquo/x-master/apps/macos/internal/ui"
	"github.com/taoquo/x-master/apps/macos/internal/usecase/settings"
	"github.com/taoquo/x-master/apps/macos/internal/usecase/suggestions"
	"github.com/taoquo/x-master/apps/macos/internal/usecase/workspace"
)

type parityText struct {
	Text string  `json:"t"`
	X    float32 `json:"x"`
	Y    float32 `json:"y"`
	W    float32 `json:"w"`
	H    float32 `json:"h"`
}

func TestDumpParityGeometry(t *testing.T) {
	dbPath := os.Getenv("XBM_PARITY_DB")
	outPath := os.Getenv("XBM_PARITY_OUT")
	if dbPath == "" || outPath == "" {
		t.Skip("set XBM_PARITY_DB and XBM_PARITY_OUT to dump the parity geometry")
	}

	db, err := sqlite.Open(filepath.Join(dbPath, databaseFile))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	repos := ports.Repositories{
		Bookmarks:   sqlite.NewBookmarkRepository(db),
		Tags:        sqlite.NewTagRepository(db),
		Lists:       sqlite.NewListRepository(db),
		Suggestions: sqlite.NewSuggestionRepository(db),
		Analysis:    sqlite.NewAnalysisRepository(db),
		Settings:    sqlite.NewSettingsRepository(db),
		SavedViews:  sqlite.NewSavedViewRepository(db),
		SyncRuns:    sqlite.NewSyncRunRepository(db),
	}

	clock := ports.SystemClock{}
	ids := idgen.New("parity")
	ctx := context.Background()

	shell := appui.NewShell(workspace.New(repos, clock, ids), appui.AppearanceLight)
	// The same fetcher and disk cache the app installs, so the cards have their pictures
	// and the geometry is the geometry of the picture, not of the placeholder. A tester
	// does not tick a window, so the load has to have happened before the frames below.
	fetcher := media.NewFetcher()
	fetcher.UseDiskCache(media.NewDiskCache(filepath.Join(dbPath, "media-cache"), media.DefaultDiskBudget))
	shell.UseMediaFetcher(fetcher)
	shell.UseSettings(settings.New(repos, clock))
	shell.UseSuggestions(suggestions.New(repos, clock, ids))
	if err := shell.ApplyPreferences(ctx); err != nil {
		t.Fatalf("ApplyPreferences: %v", err)
	}

	width, height := 1297, 808
	if raw := os.Getenv("XBM_PARITY_WIDTH"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			width = parsed
		}
	}

	shell.Reload(ctx)

	tester := ui.NewTester(shell.View, width, height)
	// The detail's 86dvh cap and its media image's 35dvh are viewport-relative and a tester has
	// no window, so the height has to be recorded or the dump lays the panel out for main.go's
	// 760-point default while the capture it is compared with was taken at 808.
	shell.SetTestedViewportHeight(height)
	// The workspace loads its first page asynchronously; a single Frame would
	// photograph the empty state. Pump frames until a bookmark's author appears.
	for attempt := 0; attempt < 80; attempt++ {
		tester.Frame()
		if tester.HasText("泊舟") {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}

	// The state, through the same entry point a capture uses, so the list dump is the list.
	// After the first page has landed, because "detail" chooses the first bookmark and there
	// is nothing to choose before the read -- the capture path applies it after its own read
	// for exactly this reason, and applying it before Reload here silently dropped the detail
	// from the dump: the reload replaced the rows, and with them the selection.
	//
	// ApplyCaptureState resets the locale as its first act, so the language has to be applied
	// after it, exactly as capture.go does -- otherwise the dump draws a Chinese page whose
	// footer chip says "EN" because state.Locale was cleared while the translator was not.
	if state := os.Getenv("XBM_CAPTURE_STATE"); state != "" {
		shell.ApplyCaptureState(state)
	}
	if locale := os.Getenv("XBM_CAPTURE_LOCALE"); locale != "" {
		shell.ApplyCaptureLocale(locale)
	}
	if appearance := os.Getenv("XBM_CAPTURE_APPEARANCE"); appearance != "" {
		shell.ApplyCaptureAppearance(appearance)
	}
	// Rebuild with the state in place. Several frames, because opening the detail starts the
	// media load the picture's geometry depends on.
	for attempt := 0; attempt < 20; attempt++ {
		tester.Frame()
		time.Sleep(10 * time.Millisecond)
	}

	rows := make([]parityText, 0, 128)
	for _, text := range tester.Texts() {
		rect, ok := tester.Find(text)
		if !ok {
			continue
		}
		rows = append(rows, parityText{Text: text, X: rect.X, Y: rect.Y, W: rect.W, H: rect.H})
	}

	blob, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if err := os.WriteFile(outPath, blob, 0o644); err != nil {
		t.Fatalf("write %s: %v", outPath, err)
	}
	t.Logf("wrote %d text boxes to %s", len(rows), outPath)
}
