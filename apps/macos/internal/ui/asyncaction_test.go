package ui

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/x-master/apps/macos/internal/idgen"
	"github.com/taoquo/x-master/apps/macos/internal/ports"
	"github.com/taoquo/x-master/apps/macos/internal/storage/sqlite"
	tagging "github.com/taoquo/x-master/apps/macos/internal/usecase/tagging"
	"github.com/taoquo/x-master/apps/macos/internal/usecase/workspace"
)

// A busy action, and a switch that fails to save (shell-contract.md S52).

// TestABusyActionSpinsInItsGlyphSlotAndTakesNoClick: magpie's `.busy` controls keep their
// label, turn a spinner where the glyph would be, dim, and take no pointer while the work
// runs (app.css:289, :306, :3456).
func TestABusyActionSpinsInItsGlyphSlotAndTakesNoClick(t *testing.T) {
	theme := ui.LightTheme()
	rail := Rail(AppearanceLight, false, false)
	var clicks []string

	tester := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Padding(20).GapY(10).Children(func() {
			textAction(c, theme, rail, textActionSpec{
				label: "Idle", onClick: func() { clicks = append(clicks, "idle") },
			})
			textAction(c, theme, rail, textActionSpec{
				label: "Busy", busy: true, onClick: func() { clicks = append(clicks, "busy") },
			})
		})
	}, 400, 200)

	// Find answers with the button, whose label it matches.
	idle, ok := tester.Find("Idle")
	if !ok {
		t.Fatalf("the idle action is not drawn; texts: %q", tester.Texts())
	}
	busy, ok := tester.Find("Busy")
	if !ok {
		t.Fatalf("the busy action is not drawn; texts: %q", tester.Texts())
	}

	// The glyph slot is the button's own 10 of padding and the 14 a glyph takes. A spinner
	// there turns, so those pixels change from frame to frame; the idle action's stay put.
	slot := func(button ui.Rect) []uint8 {
		x := button.X + SettingsActionPadX
		y := button.Y + button.H/2 - SettingsActionIcon/2
		px := make([]uint8, 0, int(SettingsActionIcon)*int(SettingsActionIcon))
		for dy := float32(0); dy < SettingsActionIcon; dy++ {
			for dx := float32(0); dx < SettingsActionIcon; dx++ {
				px = append(px, navHighlightInk(tester, x+dx, y+dy))
			}
		}
		return px
	}
	busyBefore, idleBefore := slot(busy), slot(idle)
	if slices.Min(busyBefore) == slices.Max(busyBefore) {
		t.Fatal("the busy action's glyph slot is blank: no spinner was drawn")
	}
	time.Sleep(150 * time.Millisecond)
	tester.Frame()
	if slices.Equal(busyBefore, slot(busy)) {
		t.Error("the busy action's glyph does not turn: it is not a spinner")
	}
	if !slices.Equal(idleBefore, slot(idle)) {
		t.Error("the idle action's control changed with nothing to change it")
	}

	// The work is running: the pointer cannot start it again.
	if err := tester.Click("Busy"); err != nil {
		t.Fatalf("clicking the busy action: %v", err)
	}
	if err := tester.Click("Idle"); err != nil {
		t.Fatalf("clicking the idle action: %v", err)
	}
	if !slices.Equal(clicks, []string{"idle"}) {
		t.Errorf("clicks were %v, want only the idle action", clicks)
	}
}

// failingSettings is a settings store whose writes fail: the read side is real, which is
// what a full disk or a read-only file looks like to the panel.
type failingSettings struct {
	ports.SettingsRepository
	fail bool
}

var errReadOnly = errors.New("settings store is read-only")

func (f *failingSettings) Set(ctx context.Context, key, value string, at time.Time) error {
	if f.fail {
		return errReadOnly
	}
	return f.SettingsRepository.Set(ctx, key, value, at)
}

// TestAFailedSaveRollsTheSwitchBack: the switch moves before the write, as magpie's do, so
// a write that fails has to put the panel back on what is stored. A panel that keeps the
// switch claims a setting that was never saved.
func TestAFailedSaveRollsTheSwitchBack(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "rollback.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	store := &failingSettings{SettingsRepository: sqlite.NewSettingsRepository(db)}
	repos := ports.Repositories{
		Bookmarks:   sqlite.NewBookmarkRepository(db),
		Tags:        sqlite.NewTagRepository(db),
		Lists:       sqlite.NewListRepository(db),
		Suggestions: sqlite.NewSuggestionRepository(db),
		Analysis:    sqlite.NewAnalysisRepository(db),
		Settings:    store,
		SavedViews:  sqlite.NewSavedViewRepository(db),
	}
	shell := NewShell(workspace.New(repos, fixedClock{now: clockBase}, idgen.New("test")), AppearanceLight)
	shell.UseTagging(tagging.New(repos, fixedClock{now: clockBase}, nil))

	ctx := context.Background()
	shell.loadAutoTagSettings(ctx)
	stored := shell.state.UseHashtags

	shell.state.UseHashtags = !stored
	store.fail = true
	shell.saveAutoTagSettings(ctx)

	if shell.state.UseHashtags != stored {
		t.Errorf("the hashtag switch stayed at %v after a failed save; storage holds %v",
			shell.state.UseHashtags, stored)
	}
	if shell.state.ToastKind != ToastError || shell.state.ToastMessage == "" {
		t.Errorf("a failed save reported %q/%q, want an error the reader can see",
			shell.state.ToastKind, shell.state.ToastMessage)
	}
}
