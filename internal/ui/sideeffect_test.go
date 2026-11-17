package ui

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/internal/ports"
	"github.com/taoquo/trove/internal/storage/sqlite"
	"github.com/taoquo/trove/internal/usecase/settings"
)

// builtByView is the state a build pass is allowed to write.
//
// Two writes are not events: the grid records how many columns it laid out and the
// pagination anchors fire when the scroll reaches the end. MyGo gives a scroll container
// no callback, so both are read during the build; what keeps them honest is that they are
// anchored -- the same scroll offset and the same window size must produce the same state
// however many times the pass runs, which is what this pins.
type builtByView struct {
	RenderLimit      int
	ResultsColumns   int
	autoLoadY        float32
	renderGrewY      float32
	LeftSidebar      float32
	RightSidebar     float32
	SidebarCollapsed bool
	SidebarChosen    bool
	Selected         string
	ListSelected     int
	ViewMode         string
}

func (s *Shell) builtByView() builtByView {
	return builtByView{
		RenderLimit:      s.state.RenderLimit,
		ResultsColumns:   s.state.ResultsColumns,
		autoLoadY:        s.state.autoLoadY,
		renderGrewY:      s.state.renderGrewY,
		LeftSidebar:      s.state.LeftSidebarWidth,
		RightSidebar:     s.state.RightSidebarWidth,
		SidebarCollapsed: s.state.SidebarCollapsed,
		SidebarChosen:    s.state.SidebarChosen,
		Selected:         s.state.Selected,
		ListSelected:     s.state.ListSelected,
		ViewMode:         string(s.state.ViewMode),
	}
}

// TestThreeBuildsChangeNothing covers the view's side effects.
//
// A view may be built several times for one event -- a window resize, an animation clock,
// a test frame -- so a build that writes state makes the second build a different app from
// the first. The three passes here follow a settled frame, which is what a window does
// after a scroll that reached the end of the page and after a resize: neither asks for
// anything new, so nothing may move.
func TestThreeBuildsChangeNothing(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	shell.SetLocale("en")
	repos := ports.Repositories{Settings: sqlite.NewSettingsRepository(db)}
	shell.UseSettings(settings.New(repos, fixedClock{now: clockBase}))
	for i, id := range []string{"1", "2", "3", "4", "5"} {
		seed(t, db, id, "ada", "a post "+id, clockBase.Add(time.Duration(i)))
	}
	shell.Reload(ctx)

	tester := ui.NewTester(shell.View, 1180, 760)
	// One frame to lay the page out, one to settle the scroll it measured.
	tester.Frame()
	tester.Frame()

	before := shell.builtByView()
	preferences, err := shell.settings.Load(ctx)
	if err != nil {
		t.Fatalf("Load preferences: %v", err)
	}
	beforeRendered := fmt.Sprint(tester.Texts())

	for i := 0; i < 3; i++ {
		tester.Frame()
	}

	if after := shell.builtByView(); after != before {
		t.Errorf("three builds changed what a build may not change:\nbefore %+v\nafter  %+v", before, after)
	}
	after, err := shell.settings.Load(ctx)
	if err != nil {
		t.Fatalf("Load preferences: %v", err)
	}
	if after.Layout != preferences.Layout {
		t.Errorf("a build rewrote the stored layout: before %+v, after %+v", preferences.Layout, after.Layout)
	}
	if got := fmt.Sprint(tester.Texts()); got != beforeRendered {
		t.Errorf("three builds changed the page:\nbefore %s\nafter  %s", beforeRendered, got)
	}
}

// TestTheGridRemembersTheColumnsItDrew covers what the recorded column count is for.
//
// The grid decides its columns from the viewport it is drawn in, and the handlers that
// answer the arrows read the number back to move a row at a time. With no window -- a
// tester -- the recorded count is the only measurement there is, so it has to be there
// after the first build and stay the same afterwards.
func TestTheGridRemembersTheColumnsItDrew(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	shell.SetLocale("en")
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(ctx)

	narrow := ui.NewTester(shell.View, 900, 760)
	narrow.Frame()
	if got := shell.state.ResultsColumns; got != 1 {
		t.Errorf("a 900-point window recorded %d columns, want 1", got)
	}

	wide := ui.NewTester(shell.View, 1297, 760)
	wide.Frame()
	if got := shell.state.ResultsColumns; got != 2 {
		t.Errorf("a 1297-point window recorded %d columns, want 2", got)
	}
}
