package syncer

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/idgen"
	"github.com/taoquo/trove/internal/ports"
	"github.com/taoquo/trove/internal/storage/sqlite"
)

// newServiceOver builds a service over an existing database.
//
// Two services over one file are how the window and `trove sync` meet, and the
// claim under test is the only thing between them.
func newServiceOver(t *testing.T, db *sqlite.DB, now time.Time, source PageSource) *Service {
	t.Helper()
	repos := ports.Repositories{
		Bookmarks: sqlite.NewBookmarkRepository(db),
		Tags:      sqlite.NewTagRepository(db),
		Lists:     sqlite.NewListRepository(db),
		Settings:  sqlite.NewSettingsRepository(db),
		SyncRuns:  sqlite.NewSyncRunRepository(db),
	}
	return New(repos, fixedClock{now: now}, idgen.New("test"), source)
}

func openLockDB(t *testing.T) *sqlite.DB {
	t.Helper()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "lock.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// TestASecondProcessCannotStartASecondSync is the cross-process mutex.
//
// The in-process flag cannot see another process, and the window and `trove sync`
// share one database. Without the claim, two writers upsert into the same library
// and each records half a history.
func TestASecondProcessCannotStartASecondSync(t *testing.T) {
	db := openLockDB(t)
	runs := sqlite.NewSyncRunRepository(db)
	ctx := context.Background()

	claimed, err := runs.ClaimStart(ctx, domain.SyncRun{
		ID: "other-process", Status: domain.SyncRunning,
		StartedAt: clockBase, StrategyVersion: domain.SyncStrategyVersion,
	}, clockBase, clockBase.Add(-heartbeatStaleAfter))
	if err != nil || !claimed {
		t.Fatalf("the first claim must succeed: claimed=%v err=%v", claimed, err)
	}

	service := newServiceOver(t, db, clockBase, &fakeSource{pages: []ports.Page{{Items: page("1").Items}}})
	run, err := service.Run(ctx, RunOptions{Sleep: noSleep})
	if !errors.Is(err, ErrSyncAlreadyRunning) {
		t.Fatalf("second Run error = %v, want ErrSyncAlreadyRunning", err)
	}
	if run.Status != domain.SyncError || run.ErrorKind != domain.ErrSyncInProgress {
		t.Errorf("run = %+v, want an error status carrying sync_in_progress for the sentence", run)
	}

	// The other process must keep its claim: healing it would let this one write.
	active, found, err := runs.Active(ctx)
	if err != nil || !found {
		t.Fatalf("Active: found=%v err=%v", found, err)
	}
	if active.ID != "other-process" {
		t.Errorf("active run = %q, want the other process's untouched", active.ID)
	}
}

// TestAStaleClaimIsTakenOver is the other half: a crashed process must not lock
// the library forever.
func TestAStaleClaimIsTakenOver(t *testing.T) {
	db := openLockDB(t)
	runs := sqlite.NewSyncRunRepository(db)
	ctx := context.Background()

	if claimed, err := runs.ClaimStart(ctx, domain.SyncRun{
		ID: "dead-process", Status: domain.SyncRunning,
		StartedAt: clockBase, StrategyVersion: domain.SyncStrategyVersion,
	}, clockBase, clockBase.Add(-heartbeatStaleAfter)); err != nil || !claimed {
		t.Fatalf("seed claim: claimed=%v err=%v", claimed, err)
	}

	later := clockBase.Add(2 * heartbeatStaleAfter)
	service := newServiceOver(t, db, later, &fakeSource{pages: []ports.Page{{Items: page("1").Items}}})
	if _, err := service.Run(ctx, RunOptions{Sleep: noSleep}); err != nil {
		t.Fatalf("a stale claim must not block the next run: %v", err)
	}

	dead, found, err := runs.Get(ctx, "dead-process")
	if err != nil || !found {
		t.Fatalf("Get: found=%v err=%v", found, err)
	}
	if dead.Status != domain.SyncError || dead.ErrorKind != domain.ErrInterrupted {
		t.Errorf("dead run = %+v, want it closed out as interrupted", dead)
	}
}

// TestTheStartupHealLeavesALiveRunAlone pins the cutoff.
//
// The startup self-heal used to close every running row, including one the other
// process was still working on. The wipe also freed the claim, so a second writer
// could start while the first was mid-walk.
func TestTheStartupHealLeavesALiveRunAlone(t *testing.T) {
	db := openLockDB(t)
	runs := sqlite.NewSyncRunRepository(db)
	ctx := context.Background()

	if claimed, err := runs.ClaimStart(ctx, domain.SyncRun{
		ID: "live-process", Status: domain.SyncRunning,
		StartedAt: clockBase, StrategyVersion: domain.SyncStrategyVersion,
	}, clockBase, clockBase.Add(-heartbeatStaleAfter)); err != nil || !claimed {
		t.Fatalf("seed claim: claimed=%v err=%v", claimed, err)
	}

	service := newServiceOver(t, db, clockBase.Add(time.Second), nil)
	healed, err := service.CleanupOrphanedRuns(ctx)
	if err != nil {
		t.Fatalf("CleanupOrphanedRuns: %v", err)
	}
	if healed != 0 {
		t.Errorf("healed %d run(s), want none while the heartbeat is fresh", healed)
	}
	if _, found, err := runs.Active(ctx); err != nil || !found {
		t.Errorf("the live run must survive the startup heal: found=%v err=%v", found, err)
	}
}

// TestResumeOptionsOnlyContinuesAFullWalk is the rule both front ends share.
func TestResumeOptionsOnlyContinuesAFullWalk(t *testing.T) {
	db := openLockDB(t)
	service := newServiceOver(t, db, clockBase, nil)
	ctx := context.Background()

	if err := sqlite.NewSettingsRepository(db).Set(ctx, resumeCursorKey, "cursor-42", clockBase); err != nil {
		t.Fatalf("seed: %v", err)
	}

	continued := service.ResumeOptions(ctx, RunOptions{})
	if continued.StartCursor != "cursor-42" {
		t.Errorf("StartCursor = %q, want the stored continuation point", continued.StartCursor)
	}
	// An incremental walk stops by itself; resuming would ask for page 500 of a
	// timeline it is meant to read from the top.
	if incremental := service.ResumeOptions(ctx, RunOptions{Incremental: true}); incremental.StartCursor != "" {
		t.Errorf("StartCursor = %q on an incremental run, want empty", incremental.StartCursor)
	}
}
