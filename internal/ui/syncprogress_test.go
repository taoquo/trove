package ui

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/i18n"
	"github.com/taoquo/trove/internal/idgen"
	"github.com/taoquo/trove/internal/ports"
	"github.com/taoquo/trove/internal/storage/sqlite"
	syncer "github.com/taoquo/trove/internal/usecase/syncer"
	"github.com/taoquo/trove/internal/xsource"
)

// TestTheProgressLineNamesPagesAndItems covers the sentence under the sync button.
//
// It used to be fmt.Sprintf("%d fetched", ...): English in both locales, a number
// with no unit, and nothing about how far the walk had got. The reference renders
// "Syncing... {pages} pages · {items} items" through formatSyncProgressLabel, so the template
// and the substitution stay explicit rather than being reinvented.
func TestTheProgressLineNamesPagesAndItems(t *testing.T) {
	shell, _ := newShell(t)
	progress := xsource.Progress{PagesCompleted: 4, Fetched: 60}

	en := syncProgressLabel(shell.t(), progress)
	if !strings.Contains(en, "4") || !strings.Contains(en, "60") {
		t.Errorf("en = %q, want both counters", en)
	}
	if !strings.Contains(en, "pages") || !strings.Contains(en, "items") {
		t.Errorf("en = %q, want the units the reference uses", en)
	}
	if strings.Contains(en, "{") {
		t.Errorf("en = %q, want the placeholders substituted", en)
	}
	if strings.Contains(en, "fetched") {
		t.Errorf("en = %q, the bare count is back", en)
	}

	shell.SetLocale("zh-CN")
	zh := syncProgressLabel(shell.t(), progress)
	if zh == en {
		t.Fatalf("zh = %q, want its own sentence", zh)
	}
	if !strings.Contains(zh, "4") || !strings.Contains(zh, "60") {
		t.Errorf("zh = %q, want both counters", zh)
	}
	if !strings.Contains(zh, "页") || !strings.Contains(zh, "条") {
		t.Errorf("zh = %q, want 页 and 条 as the units", zh)
	}
}

// TestTheSyncLineCountsDownARateLimitWait covers the one wait long enough to look like a
// hang.
//
// X's rate-limit window is minutes long, and the walk reported it once without changing
// the line, so a page count sat frozen for the whole wait. The line now counts the wait
// down, and only the wait: the quarter-second pace between pages is not news and must not
// make the line flicker.
func TestTheSyncLineCountsDownARateLimitWait(t *testing.T) {
	shell, _ := newShell(t)
	shell.SetLocale("en")
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	progress := xsource.Progress{
		PagesCompleted: 12,
		Fetched:        240,
		WaitReason:     xsource.WaitRateLimit,
		WaitingUntil:   now.Add(5 * time.Second),
	}

	if got, want := syncProgressLabelAt(shell.t(), progress, now), shell.t().Tf(i18n.SyncWaitLabel, 5); got != want {
		t.Errorf("at the start of the wait the line says %q, want %q", got, want)
	}
	if got, want := syncProgressLabelAt(shell.t(), progress, now.Add(2*time.Second)), shell.t().Tf(i18n.SyncWaitLabel, 3); got != want {
		t.Errorf("two seconds in the line says %q, want %q", got, want)
	}
	// A fraction left still counts as a whole second: "waiting 0 s" while the run is
	// still asleep would be a lie.
	if got, want := syncProgressLabelAt(shell.t(), progress, now.Add(4500*time.Millisecond)), shell.t().Tf(i18n.SyncWaitLabel, 1); got != want {
		t.Errorf("half a second left the line says %q, want %q", got, want)
	}
	// The wait is over: the line goes back to the pages and items, not "waiting 0 s".
	if got := syncProgressLabelAt(shell.t(), progress, now.Add(5*time.Second)); got != syncProgressLabel(shell.t(), progress) {
		t.Errorf("after the wait the line says %q, want the page sentence", got)
	}

	// The ordinary pace between pages carries no reason and keeps the page sentence.
	pacing := xsource.Progress{PagesCompleted: 12, Fetched: 240, WaitingUntil: now.Add(250 * time.Millisecond)}
	if got, want := syncProgressLabelAt(shell.t(), pacing, now), syncProgressLabel(shell.t(), pacing); got != want {
		t.Errorf("the pace between pages showed %q, want %q", got, want)
	}

	shell.SetLocale("zh-CN")
	zh := syncProgressLabelAt(shell.t(), progress, now)
	if !strings.Contains(zh, "限流") || !strings.Contains(zh, "5") {
		t.Errorf("zh = %q, want 限流 and the seconds left", zh)
	}
}

// TestTheLibraryRefreshesAtAWalkingPace covers the rate limit on mid-run reloads.
//
// The service writes a page at a time, so a progress event means there is new
// material to show. Reloading on every one of them would re-read the library and
// rebuild the sidebar four times a second through a first full sync, so the first
// page reloads and the next ones within the pace do not.
func TestTheLibraryRefreshesAtAWalkingPace(t *testing.T) {
	shell, _ := newShell(t)

	if !shell.syncReloadDue() {
		t.Fatal("the first page should refresh the library")
	}
	if shell.syncReloadDue() {
		t.Fatal("a second page inside the pace refreshed the library again")
	}

	// Once the pace has passed, the next page is allowed through.
	shell.lastSyncReload = time.Now().Add(-2 * syncReloadPace)
	if !shell.syncReloadDue() {
		t.Fatal("the library is still not refreshing after the pace elapsed")
	}
}

// rateLimitedSource answers the first page with a 429 that names a two-second window,
// then ends the timeline. It is the shape of a real X rate limit, which the walk honours
// by sleeping rather than failing.
type rateLimitedSource struct {
	mu    sync.Mutex
	calls int
}

func (s *rateLimitedSource) FetchPage(context.Context, string) (ports.Page, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if s.calls == 1 {
		return ports.Page{}, &xsource.Error{
			Kind:       domain.ErrRateLimited,
			Status:     429,
			Op:         "timeline",
			RetryAfter: 2 * time.Second,
		}
	}
	return ports.Page{}, nil
}

// waitSeconds reads the seconds out of the wait line, if that is the line.
func waitSeconds(label string) (int, bool) {
	const prefix = "Rate limited — waiting "
	if !strings.HasPrefix(label, prefix) {
		return 0, false
	}
	seconds, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(label, prefix), " s"))
	if err != nil {
		return 0, false
	}
	return seconds, true
}

// TestARateLimitWaitCountsDownInARunningSync covers the ticker, not just the sentence.
//
// The walk announces a wait once, when it begins, and then sleeps. X's window is minutes
// long, so a line that is written once would say the same number until the wait ended --
// which is what the app did before this: the page count froze. This drives a real run
// through a real 429 and watches the window thread's publishes, which must show the count
// going down with nothing else prompting them.
func TestARateLimitWaitCountsDownInARunningSync(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	shell.SetLocale("en")
	// The real clock: this test is about elapsed time, and the wait is measured against
	// it. A pinned clock would put the end of the window in the past, so the countdown
	// would never start.
	shell.UseSyncer(syncer.New(ports.Repositories{
		Bookmarks: sqlite.NewBookmarkRepository(db),
		Tags:      sqlite.NewTagRepository(db),
		Lists:     sqlite.NewListRepository(db),
		Settings:  sqlite.NewSettingsRepository(db),
		SyncRuns:  sqlite.NewSyncRunRepository(db),
	}, clockOnly{}, idgen.New("test"), &rateLimitedSource{}))

	published := make(chan func(), 64)
	shell.testPublish = func(fn func()) { published <- fn }

	if !shell.StartSync(ctx) {
		t.Fatal("the sync did not start")
	}

	seconds := make(map[int]bool)
	deadline := time.After(10 * time.Second)
	for shell.state.SyncRunning {
		select {
		case fn := <-published:
			fn()
			if left, ok := waitSeconds(shell.state.SyncProgress); ok {
				seconds[left] = true
			}
		case <-deadline:
			t.Fatalf("the run never finished; progress %q", shell.state.SyncProgress)
		}
	}
	// Apply whatever the run left queued, so it cannot leak into the next test.
	for {
		select {
		case fn := <-published:
			fn()
			continue
		default:
		}
		break
	}

	if len(seconds) < 2 {
		t.Fatalf("the wait line showed the seconds %v, want it to count down", seconds)
	}
	if got, want := shell.state.LastSyncStatus, domain.SyncSuccess; got != want {
		t.Errorf("the run recorded %q after the wait, want %q", got, want)
	}
}
