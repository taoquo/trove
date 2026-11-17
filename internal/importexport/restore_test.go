package importexport

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/taoquo/trove/internal/bundle"
	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/ports"
)

// libraryFiles exports a library and returns its counts and every file's bytes.
//
// The file bytes are the content hash of each table: the writer emits records in a
// stable order and the clock is fixed, so two exports of the same data are equal file
// for file. That is stronger than counting rows, and it names the table that differs.
func libraryFiles(t *testing.T, service *Service, ctx context.Context) (bundle.Counts, map[string][]byte) {
	t.Helper()
	raw, err := service.Export(ctx, SnapshotExportOptions())
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	parsed, err := service.parse(raw)
	if err != nil {
		t.Fatalf("parse export: %v", err)
	}
	return parsed.Bundle.Manifest.Counts, parsed.Bundle.Files
}

func assertSameLibrary(t *testing.T, want bundle.Counts, wantFiles map[string][]byte,
	got bundle.Counts, gotFiles map[string][]byte) {
	t.Helper()
	if want != got {
		t.Fatalf("the library's counts are %+v, want %+v", got, want)
	}
	for name, wantBody := range wantFiles {
		gotBody, present := gotFiles[name]
		if !present {
			t.Errorf("%s is missing after the rollback", name)
			continue
		}
		if !bytes.Equal(wantBody, gotBody) {
			t.Errorf("%s changed across the rollback:\n got %s\nwant %s", name, gotBody, wantBody)
		}
	}
}

// A bundle carries raw payloads only when the export asked for them, and a restore
// must write them back.
//
// The payload is the diagnostic record of a response body. It was the one table the
// restore silently dropped: the file was written by "keep raw responses" and never
// read, so a restore cost the user every capture it had.
func TestRawPayloadsSurviveAnExportRestoreRoundTrip(t *testing.T) {
	ctx := context.Background()

	source, _, sourceRepos := newService(t)
	seedLibrary(t, sourceRepos)

	payloads := []ports.RawPayload{
		{TweetID: "1", PayloadJSON: `{"data":{"id":"1","text":"first post"}}`, CapturedAt: "2026-10-12T12:00:00Z"},
		{TweetID: "2", PayloadJSON: `{"data":{"id":"2","text":"second post"}}`, CapturedAt: "2026-10-12T12:00:01Z"},
	}
	if err := sourceRepos.Bookmarks.SaveRawPayloads(ctx, payloads); err != nil {
		t.Fatalf("SaveRawPayloads: %v", err)
	}

	raw, err := source.Export(ctx, ExportOptions{IncludeSyncRuns: true, IncludeRawPayloads: true})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	target, _, targetRepos := newService(t)
	report, err := target.Restore(ctx, raw)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if report.RawPayloadsWritten != len(payloads) {
		t.Errorf("the report says %d raw payloads were written, want %d",
			report.RawPayloadsWritten, len(payloads))
	}

	stored, err := targetRepos.Bookmarks.RawPayloads(ctx, []string{"1", "2"})
	if err != nil {
		t.Fatalf("RawPayloads: %v", err)
	}
	byID := make(map[string]ports.RawPayload, len(stored))
	for _, payload := range stored {
		byID[payload.TweetID] = payload
	}
	for _, want := range payloads {
		got, found := byID[want.TweetID]
		if !found {
			t.Errorf("raw payload for %s did not survive the round trip", want.TweetID)
			continue
		}
		if got.PayloadJSON != want.PayloadJSON || got.CapturedAt != want.CapturedAt {
			t.Errorf("raw payload for %s is %+v, want %+v", want.TweetID, got, want)
		}
	}
}

// A restore brings back the tombstones the bundle carries.
//
// A tombstone is the only record that a post was deleted on purpose. The wipe a
// restore performs clears the local ones (the file is the whole truth), so without
// the file's copy the next sync happily re-imports every post the user had removed
// on the machine that made the backup.
func TestARestoredBundleKeepsItsDeletedPostsDeleted(t *testing.T) {
	ctx := context.Background()

	source, _, sourceRepos := newService(t)
	seedLibrary(t, sourceRepos)
	if _, err := sourceRepos.Bookmarks.UpsertMany(ctx, []domain.Bookmark{localBookmark("4")}); err != nil {
		t.Fatalf("seed bookmark 4: %v", err)
	}
	if _, err := sourceRepos.Bookmarks.DeleteMany(ctx, []string{"4"}); err != nil {
		t.Fatalf("DeleteMany: %v", err)
	}

	raw, err := source.Export(ctx, DefaultExportOptions())
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	target, _, targetRepos := newService(t)
	if _, err := targetRepos.Bookmarks.UpsertMany(ctx, []domain.Bookmark{localBookmark("local")}); err != nil {
		t.Fatalf("seed local bookmark: %v", err)
	}
	if _, err := targetRepos.Bookmarks.DeleteMany(ctx, []string{"local"}); err != nil {
		t.Fatalf("DeleteMany(local): %v", err)
	}

	report, err := target.Restore(ctx, raw)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if report.TombstonesWritten != 1 {
		t.Errorf("the report says %d tombstones were written, want 1", report.TombstonesWritten)
	}

	tombstones, err := targetRepos.Bookmarks.Tombstones(ctx)
	if err != nil {
		t.Fatalf("Tombstones: %v", err)
	}
	found := map[string]bool{}
	for _, tombstone := range tombstones {
		found[tombstone.TweetID] = true
	}
	if !found["4"] {
		t.Errorf("the bundle's tombstone for 4 did not survive the restore; got %+v", tombstones)
	}
	// The local delete is not in the bundle, so the restored library is free to
	// contain that post again.
	if found["local"] {
		t.Error("a tombstone the bundle does not carry survived a restore")
	}

	// The proof that matters: a sync's upsert of the deleted id is still skipped.
	if _, err := targetRepos.Bookmarks.UpsertMany(ctx, []domain.Bookmark{localBookmark("4")}); err != nil {
		t.Fatalf("UpsertMany(4): %v", err)
	}
	if _, found, err := targetRepos.Bookmarks.Get(ctx, "4"); err != nil {
		t.Fatalf("Get(4): %v", err)
	} else if found {
		t.Error("the restored tombstone does not stop an upsert, so the next sync would resurrect the post")
	}
}

// failingSave refuses the first raw-payload write and lets everything else through.
//
// It stands in for a disk that fills up mid-restore. SaveRawPayloads runs after the
// bookmarks, relations and memberships, so the failure lands on a library that has
// already been wiped and partly rewritten -- the exact state the rollback exists for.
type failingSave struct {
	ports.BookmarkRepository
	failed bool
}

func (r *failingSave) SaveRawPayloads(ctx context.Context, payloads []ports.RawPayload) error {
	if !r.failed {
		r.failed = true
		return errors.New("disk full")
	}
	return r.BookmarkRepository.SaveRawPayloads(ctx, payloads)
}

// A restore that fails part way through puts the whole library back.
//
// The apply path is not transactional, so the snapshot is the only way back. It must
// contain everything a failed restore replaced -- raw payloads included -- and the
// rollback is verified rather than assumed, because a table that silently skipped its
// rows leaves no error behind.
func TestAFailedRestorePutsTheWholeLibraryBack(t *testing.T) {
	ctx := context.Background()

	source, _, sourceRepos := newService(t)
	seedLibrary(t, sourceRepos)
	if err := sourceRepos.Bookmarks.SaveRawPayloads(ctx, []ports.RawPayload{
		{TweetID: "1", PayloadJSON: `{"from":"the backup"}`, CapturedAt: "2026-10-12T12:00:00Z"},
	}); err != nil {
		t.Fatalf("SaveRawPayloads(source): %v", err)
	}
	// The backup knows about a delete this machine should inherit.
	if _, err := sourceRepos.Bookmarks.UpsertMany(ctx, []domain.Bookmark{localBookmark("4")}); err != nil {
		t.Fatalf("seed bookmark 4: %v", err)
	}
	if _, err := sourceRepos.Bookmarks.DeleteMany(ctx, []string{"4"}); err != nil {
		t.Fatalf("DeleteMany(4): %v", err)
	}
	raw, err := source.Export(ctx, SnapshotExportOptions())
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	target, _, targetRepos := newService(t)
	seedLibrary(t, targetRepos)
	// seedLibrary leaves the first bookmark without a list, which the app's invariant
	// does not allow; put it in one so the snapshot is a real library and the rollback
	// has nothing to repair.
	if err := targetRepos.Bookmarks.AssignToList(ctx, []string{"1"}, "l1", clockBase); err != nil {
		t.Fatalf("AssignToList: %v", err)
	}
	// A local payload, a local tombstone and a pending suggestion: the three rows the
	// plan's assertion names, and the ones an incomplete snapshot loses first.
	if err := targetRepos.Bookmarks.SaveRawPayloads(ctx, []ports.RawPayload{
		{TweetID: "1", PayloadJSON: `{"from":"this machine"}`, CapturedAt: "2026-10-12T11:00:00Z"},
	}); err != nil {
		t.Fatalf("SaveRawPayloads(target): %v", err)
	}
	if _, err := targetRepos.Bookmarks.UpsertMany(ctx, []domain.Bookmark{localBookmark("9")}); err != nil {
		t.Fatalf("seed bookmark 9: %v", err)
	}
	if _, err := targetRepos.Bookmarks.DeleteMany(ctx, []string{"9"}); err != nil {
		t.Fatalf("DeleteMany(9): %v", err)
	}

	wantCounts, wantFiles := libraryFiles(t, target, ctx)

	target.repos.Bookmarks = &failingSave{BookmarkRepository: targetRepos.Bookmarks}

	_, err = target.Restore(ctx, raw)
	if err == nil {
		t.Fatal("the restore succeeded although a write failed")
	}
	if !strings.Contains(err.Error(), "previous library was put back") {
		t.Fatalf("the restore failed without a verified rollback: %v", err)
	}

	gotCounts, gotFiles := libraryFiles(t, target, ctx)
	assertSameLibrary(t, wantCounts, wantFiles, gotCounts, gotFiles)

	// The failed attempt did not resurrect the backup's tombstone either: the library
	// is the one that was there before, not part of the backup.
	tombstones, err := targetRepos.Bookmarks.Tombstones(ctx)
	if err != nil {
		t.Fatalf("Tombstones: %v", err)
	}
	if len(tombstones) != 1 || tombstones[0].TweetID != "9" {
		t.Errorf("tombstones after the rollback are %+v, want only 9", tombstones)
	}

	// A second attempt, now that the disk works, replaces the library properly and
	// keeps the backup's own delete record.
	target.repos.Bookmarks = targetRepos.Bookmarks
	report, err := target.Restore(ctx, raw)
	if err != nil {
		t.Fatalf("second Restore: %v", err)
	}
	if report.TombstonesWritten != 1 {
		t.Errorf("the successful restore wrote %d tombstones, want 1", report.TombstonesWritten)
	}
	tombstones, err = targetRepos.Bookmarks.Tombstones(ctx)
	if err != nil {
		t.Fatalf("Tombstones: %v", err)
	}
	if len(tombstones) != 1 || tombstones[0].TweetID != "4" {
		t.Errorf("tombstones after the successful restore are %+v, want only 4", tombstones)
	}
}
