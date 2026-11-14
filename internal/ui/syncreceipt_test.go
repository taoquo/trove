package ui

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/i18n"
	"github.com/taoquo/trove/internal/idgen"
	"github.com/taoquo/trove/internal/ports"
	"github.com/taoquo/trove/internal/storage/sqlite"
	syncer "github.com/taoquo/trove/internal/usecase/syncer"
)

// TestACancelledSyncSaysCancelled keeps the footer's word in step with the dot.
//
// workspaceStatus already reported a stopped run, and syncStatusLabel has a sentence for it
// because the status needs a word of its own. The sidebar's label must not skip the case: a run
// the reader has just stopped would leave the state row's tooltip saying "Idle" -- the same
// workspace, described as if the run had never happened.
func TestACancelledSyncSaysCancelled(t *testing.T) {
	shell, _ := newShell(t)
	shell.state.LastSyncStatus = domain.SyncCancelled

	if got, want := shell.workspaceStatus(), domain.SyncCancelled; got != want {
		t.Fatalf("workspaceStatus = %q, want %q", got, want)
	}
	if got, want := shell.workspaceStatusLabel(), i18n.StatusCancelled; got != want {
		t.Errorf("a stopped sync is labelled %q, want %q", got, want)
	}
}

// TestTheResumeLineOffersTheSyncItAsksFor covers the sentence with nothing under it.
//
// "Older bookmarks are waiting — sync again to continue" is an instruction: it names the
// action and expects the reader to find it. The start lives in the footer as an icon whose
// name is a tooltip, and the failure sentence has a labelled button for exactly this reason. The
// resume sentence needs one too.
func TestTheResumeLineOffersTheSyncItAsksFor(t *testing.T) {
	shell, _ := newShell(t)
	shell.SetLocale(i18n.EN)
	shell.state.SyncResumePending = true

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	if !tester.HasText(shell.t().T(i18n.SyncMoreAvailable)) {
		t.Fatalf("the resume sentence is missing; texts: %q", tester.Texts())
	}
	if !tester.HasText(shell.t().T(i18n.ActionSync)) {
		t.Errorf("the resume sentence names an action it does not offer; texts: %q", tester.Texts())
	}
}

// syncPageSource serves one scripted page and then the end of the timeline.
type syncPageSource struct {
	mu   sync.Mutex
	call int
}

func (s *syncPageSource) FetchPage(context.Context, string) (ports.Page, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.call++
	if s.call > 1 {
		return ports.Page{}, nil
	}
	return ports.Page{Items: []domain.Bookmark{
		{TweetID: "901", TweetURL: "https://x.com/ada/status/901", AuthorName: "Ada",
			AuthorHandle: "ada", Text: "first new post", SavedAt: clockBase, LastSeenAt: clockBase},
		{TweetID: "902", TweetURL: "https://x.com/ada/status/902", AuthorName: "Ada",
			AuthorHandle: "ada", Text: "second new post", SavedAt: clockBase, LastSeenAt: clockBase},
	}}, nil
}

// TestASuccessfulSyncSaysWhatItStored closes the loop on a manual run.
//
// A run ends by clearing the progress line and updating the footer's timestamp: the
// strongest signal that a minute of fetching worked is a date changing in 11px type. The
// run already knows how many bookmarks it stored, and the status line floats above every
// page, so the outcome belongs there.
func TestASuccessfulSyncSaysWhatItStored(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	shell.UseSyncer(syncer.New(ports.Repositories{
		Bookmarks: sqlite.NewBookmarkRepository(db),
		Tags:      sqlite.NewTagRepository(db),
		Lists:     sqlite.NewListRepository(db),
		Settings:  sqlite.NewSettingsRepository(db),
		SyncRuns:  sqlite.NewSyncRunRepository(db),
	}, fixedClock{now: clockBase}, idgen.New("test"), &syncPageSource{}))

	published := make(chan func(), 16)
	shell.testPublish = func(fn func()) { published <- fn }

	if !shell.StartSync(ctx) {
		t.Fatal("the sync did not start")
	}

	deadline := time.After(5 * time.Second)
	for shell.state.SyncRunning {
		select {
		case fn := <-published:
			fn()
		case <-deadline:
			t.Fatalf("the run never finished; running=%v progress=%q", shell.state.SyncRunning, shell.state.SyncProgress)
		}
	}
	// The last publication is the completion; apply anything still queued.
	for {
		select {
		case fn := <-published:
			fn()
			continue
		default:
		}
		break
	}

	if got, want := shell.state.LastSyncStatus, domain.SyncSuccess; got != want {
		t.Fatalf("the run recorded %q, want %q", got, want)
	}
	if got, want := shell.ToastText(), shell.t().Tf(i18n.SyncStored, 2); got != want {
		t.Errorf("a successful sync said %q; want %q", got, want)
	}
}
