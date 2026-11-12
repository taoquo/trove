package syncer

import (
	"context"
	"testing"

	"github.com/taoquo/trove/apps/macos/internal/ports"
	"github.com/taoquo/trove/apps/macos/internal/storage/sqlite"
)

// TestALocallyDeletedBookmarkStaysDeleted is the F7-1 defect: a delete removed
// only the local row, so the next sync upserted the same tweet and the bookmark
// came back. The user's delete must outlive a sync.
func TestALocallyDeletedBookmarkStaysDeleted(t *testing.T) {
	source := &fakeSource{pages: []ports.Page{{Items: page("1", "2", "3").Items, NextCursor: ""}}}
	service, db := newService(t, source)
	ctx := context.Background()
	repo := sqlite.NewBookmarkRepository(db)

	if _, err := repo.UpsertMany(ctx, page("1", "2", "3").Items); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := repo.DeleteMany(ctx, []string{"2"}); err != nil {
		t.Fatalf("DeleteMany: %v", err)
	}

	// The same timeline comes back on the next sync.
	if _, err := service.Run(ctx, RunOptions{Sleep: noSleep}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if _, found, err := repo.Get(ctx, "2"); err != nil {
		t.Fatalf("Get(2): %v", err)
	} else if found {
		t.Error("bookmark 2 was re-imported by the sync after the user deleted it")
	}
	for _, id := range []string{"1", "3"} {
		if _, found, err := repo.Get(ctx, id); err != nil {
			t.Fatalf("Get(%s): %v", id, err)
		} else if !found {
			t.Errorf("bookmark %s must survive the sync", id)
		}
	}
}
