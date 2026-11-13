package importexport

import (
	"context"
	"testing"

	"github.com/taoquo/trove/internal/domain"
)

// localBookmark is a minimal stored row for the tombstone cases.
func localBookmark(id string) domain.Bookmark {
	return domain.Bookmark{
		TweetID: id, TweetURL: "https://x.com/x/status/" + id, Text: "post " + id,
		SavedAt: clockBase, LastSeenAt: clockBase, UpdatedAt: clockBase,
	}
}

// TestImportRevivesBookmarksTheFileCarries pins the merge rule: a backup the
// user explicitly imports is authoritative for the ids it carries, so those
// tombstones are cleared, while bookmarks deleted here and absent from the file
// stay deleted.
func TestImportRevivesBookmarksTheFileCarries(t *testing.T) {
	source, _, sourceRepos := newService(t)
	ctx := context.Background()
	seedLibrary(t, sourceRepos)

	raw, err := source.Export(ctx, DefaultExportOptions())
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	target, _, targetRepos := newService(t)
	targetBookmarks := targetRepos.Bookmarks

	// "1" is in the bundle and was deleted here; "local" is not in the bundle.
	for _, id := range []string{"1", "local"} {
		if _, err := targetBookmarks.UpsertMany(ctx, []domain.Bookmark{localBookmark(id)}); err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
	}
	if _, err := targetBookmarks.DeleteMany(ctx, []string{"1", "local"}); err != nil {
		t.Fatalf("DeleteMany: %v", err)
	}

	report, err := target.Import(ctx, raw)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if report.BookmarksInserted != 2 {
		t.Errorf("import inserted %d bookmarks, want the bundle's 2: a tombstoned id from the file must come back",
			report.BookmarksInserted)
	}

	if _, found, err := targetBookmarks.Get(ctx, "1"); err != nil {
		t.Fatalf("Get(1): %v", err)
	} else if !found {
		t.Error("the imported file carries bookmark 1, so its local tombstone must be forgotten")
	}

	// The id outside the file kept its tombstone: a later upsert is still skipped.
	if _, err := targetBookmarks.UpsertMany(ctx, []domain.Bookmark{localBookmark("local")}); err != nil {
		t.Fatalf("UpsertMany(local): %v", err)
	}
	if _, found, err := targetBookmarks.Get(ctx, "local"); err != nil {
		t.Fatalf("Get(local): %v", err)
	} else if found {
		t.Error("an import must not clear the tombstone of an id its file does not carry")
	}
}

// TestRestoreClearsEveryTombstone pins the replace rule: a restore replaces the
// library, so the bundle is the whole truth and no old local delete may survive
// to shadow a bookmark the bundle contains. The tombstones are cleared even for
// ids the bundle does not carry, because the restored library is a fresh start.
func TestRestoreClearsEveryTombstone(t *testing.T) {
	source, _, sourceRepos := newService(t)
	ctx := context.Background()
	seedLibrary(t, sourceRepos)

	raw, err := source.Export(ctx, DefaultExportOptions())
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	target, _, targetRepos := newService(t)
	targetBookmarks := targetRepos.Bookmarks
	if _, err := targetBookmarks.UpsertMany(ctx, []domain.Bookmark{localBookmark("local")}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := targetBookmarks.DeleteMany(ctx, []string{"local"}); err != nil {
		t.Fatalf("DeleteMany: %v", err)
	}

	if _, err := target.Restore(ctx, raw); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	// The replaced library is the bundle's, so the old bookmark is gone.
	if _, found, err := targetBookmarks.Get(ctx, "local"); err != nil {
		t.Fatalf("Get(local): %v", err)
	} else if found {
		t.Error("a restore must not keep data the bundle does not contain")
	}

	// Its tombstone is gone too: the id can be written again.
	if _, err := targetBookmarks.UpsertMany(ctx, []domain.Bookmark{localBookmark("local")}); err != nil {
		t.Fatalf("UpsertMany(local): %v", err)
	}
	if _, found, err := targetBookmarks.Get(ctx, "local"); err != nil {
		t.Fatalf("Get(local): %v", err)
	} else if !found {
		t.Error("a restore must clear every tombstone, including ids the bundle does not carry")
	}
}
