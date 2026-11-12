package syncer

import (
	"context"
	"testing"

	"github.com/taoquo/x-master/apps/macos/internal/domain"
	"github.com/taoquo/x-master/apps/macos/internal/ports"
	"github.com/taoquo/x-master/apps/macos/internal/storage/sqlite"
)

// watchingSource records what the database holds when each page is requested.
//
// It is how "the pages are written as they arrive" is observed from outside: a run
// that collects everything and writes once at the end shows an empty database at
// every fetch but the last.
type watchingSource struct {
	db    *sqlite.DB
	calls int
	// stored[i] is the row count seen when fetch i+1 began.
	stored []int
}

func (s *watchingSource) FetchPage(ctx context.Context, _ string) (ports.Page, error) {
	count, err := sqlite.NewBookmarkRepository(s.db).AllIDs(ctx)
	if err != nil {
		return ports.Page{}, err
	}
	s.stored = append(s.stored, len(count))

	s.calls++
	switch s.calls {
	case 1:
		return ports.Page{Items: page("1", "2").Items, NextCursor: "cursor-1"}, nil
	case 2:
		return ports.Page{Items: page("3").Items, NextCursor: "cursor-2"}, nil
	case 3:
		return ports.Page{Items: page("4").Items}, nil
	}
	return ports.Page{}, nil
}

// TestPagesAreStoredBeforeTheNextOneIsFetched is the regression test for the empty
// window.
//
// A first full sync of a large library is minutes of paced requests. The run used
// to hold every page in memory and write once, so the library stayed empty until
// the last page landed -- and a cancel or a crash in between threw away every page
// already paid for. Each page now reaches storage before the next request goes out.
func TestPagesAreStoredBeforeTheNextOneIsFetched(t *testing.T) {
	source := &watchingSource{}
	service, db := newService(t, source)
	source.db = db

	run, err := service.Run(context.Background(), RunOptions{Sleep: noSleep})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	want := []int{0, 2, 3} // nothing, then page 1's two rows, then page 2's one
	if len(source.stored) != len(want) {
		t.Fatalf("saw %d fetches, want %d", len(source.stored), len(want))
	}
	for i := range want {
		if source.stored[i] != want[i] {
			t.Errorf("before fetch %d the library held %d rows, want %d -- the pages are not written "+
				"as they arrive", i+1, source.stored[i], want[i])
		}
	}

	if run.FetchedCount != 4 || run.InsertedCount != 4 {
		t.Errorf("run fetched %d and inserted %d, want 4 and 4", run.FetchedCount, run.InsertedCount)
	}
	if run.Status != domain.SyncSuccess {
		t.Errorf("status = %q, want success", run.Status)
	}

	// And the closing steps still ran: every row is in a list, or it would be
	// invisible in every view including the Inbox.
	unfiled, err := sqlite.NewBookmarkRepository(db).AllIDs(context.Background())
	if err != nil {
		t.Fatalf("AllIDs: %v", err)
	}
	if len(unfiled) != 4 {
		t.Errorf("the library holds %d rows after the run, want 4", len(unfiled))
	}
}
