package importexport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/taoquo/trove/apps/macos/internal/bundle"
	"github.com/taoquo/trove/apps/macos/internal/domain"
	"github.com/taoquo/trove/apps/macos/internal/ports"
	"github.com/taoquo/trove/apps/macos/internal/storage/sqlite"
)

var clockBase = time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

// newService opens a real database and wires the real adapters, so the round trip
// exercises the SQL rather than a stub that agrees with it.
func newService(t *testing.T) (*Service, *sqlite.DB, ports.Repositories) {
	t.Helper()

	db, err := sqlite.Open(filepath.Join(t.TempDir(), "bundle.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

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

	service := New(repos, fixedClock{now: clockBase}, "0.3.0")
	// The allowlist is the settings package's job; here it is simulated with a
	// single exportable key, and the applier mirrors it back on import.
	service.ExportableSettings = func(ctx context.Context) (map[string]json.RawMessage, error) {
		raw, found, err := repos.Settings.Get(ctx, "app.preferences")
		if err != nil || !found {
			return map[string]json.RawMessage{}, err
		}
		return map[string]json.RawMessage{"app.preferences": json.RawMessage(raw)}, nil
	}
	service.RestoreSettings = func(ctx context.Context, settings map[string]json.RawMessage) error {
		// Only allowlisted keys are applied, mirroring the exporter.
		for _, key := range []string{"app.preferences"} {
			value, present := settings[key]
			if !present {
				continue
			}
			if err := repos.Settings.Set(ctx, key, string(value), clockBase); err != nil {
				return err
			}
		}
		return nil
	}
	return service, db, repos
}

func seedLibrary(t *testing.T, repos ports.Repositories) {
	t.Helper()
	ctx := context.Background()

	bookmarks := []domain.Bookmark{
		{
			TweetID: "1", TweetURL: "https://x.com/ada/status/1", AuthorName: "Ada",
			AuthorHandle: "ada", Text: "first post", SavedAt: clockBase,
			LastSeenAt: clockBase, UpdatedAt: clockBase, Hashtags: []string{"go"},
		},
		{
			TweetID: "2", TweetURL: "https://x.com/grace/status/2", AuthorName: "Grace",
			AuthorHandle: "grace", Text: "second post", SavedAt: clockBase,
			LastSeenAt: clockBase, UpdatedAt: clockBase,
		},
	}
	if _, err := repos.Bookmarks.UpsertMany(ctx, bookmarks); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}

	tag := domain.Tag{
		ID: "t1", Name: "Go", CanonicalKey: domain.CanonicalTagKey("Go"),
		Aliases:   []string{"golang"},
		CreatedAt: domain.FormatTimestamp(clockBase), UpdatedAt: domain.FormatTimestamp(clockBase),
	}
	if err := repos.Tags.Create(ctx, tag); err != nil {
		t.Fatalf("Create tag: %v", err)
	}
	if err := repos.Tags.Attach(ctx, domain.BookmarkTag{
		BookmarkID: "1", TagID: "t1", CreatedAt: clockBase, Source: domain.TagSourceManual,
	}); err != nil {
		t.Fatalf("Attach: %v", err)
	}

	work := domain.List{
		ID: "l1", Name: "Work", Normalized: domain.NormalizeListName("Work"),
		CreatedAt: clockBase, UpdatedAt: clockBase,
	}
	if err := repos.Lists.Create(ctx, work); err != nil {
		t.Fatalf("Create list: %v", err)
	}
	if err := repos.Bookmarks.AssignToList(ctx, []string{"2"}, "l1", clockBase); err != nil {
		t.Fatalf("AssignToList: %v", err)
	}

	// A stored preference, so the export has something allowlisted to carry.
	if err := repos.Settings.Set(ctx, "app.preferences", `{"appearance":"dark"}`, clockBase); err != nil {
		t.Fatalf("Settings.Set: %v", err)
	}

	if _, err := repos.Suggestions.InsertMany(ctx, []domain.TagSuggestion{
		{
			ID: "s1", BookmarkID: "2", TagName: "Kubernetes", Source: domain.SourceLLM,
			Confidence: 0.9, Status: domain.StatusPending, CreatedAt: clockBase,
		},
	}); err != nil {
		t.Fatalf("InsertMany: %v", err)
	}
}

// TestExportContainsEveryTable covers the plan's gap: the manifest it proposed
// omitted tag aliases and parentId, and the file list omitted several tables.
func TestExportContainsEveryTable(t *testing.T) {
	service, _, repos := newService(t)
	seedLibrary(t, repos)

	raw, err := service.Export(context.Background(), DefaultExportOptions())
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	parsed, err := bundle.Read(newReader(raw), int64(len(raw)), bundle.DefaultLimits())
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	for _, name := range []string{
		bundle.FileBookmarks, bundle.FileTags, bundle.FileBookmarkTags,
		bundle.FileLists, bundle.FileBookmarkLists, bundle.FileTagSuggestions,
		bundle.FileSavedViews, bundle.FileSettings,
	} {
		if _, present := parsed.Files[name]; !present {
			t.Errorf("%s is missing from the export", name)
		}
	}

	// The tag's alias survives, which the proposed manifest list omitted.
	tags, err := decodeRecords[TagDTO](parsed.Files[bundle.FileTags])
	if err != nil {
		t.Fatalf("decode tags: %v", err)
	}
	if len(tags) != 1 || len(tags[0].Aliases) != 1 || tags[0].Aliases[0] != "golang" {
		t.Errorf("tag aliases did not survive the export: %+v", tags)
	}

	// The raw payload is deliberately not exported.
	bookmarks, err := decodeRecords[BookmarkDTO](parsed.Files[bundle.FileBookmarks])
	if err != nil {
		t.Fatalf("decode bookmarks: %v", err)
	}
	for _, bookmark := range bookmarks {
		if len(bookmark.RawPayload) > 0 {
			t.Error("a raw payload must not be exported by default")
		}
	}
}

// TestRoundTripPreservesTheLibrary is the central check.
func TestRoundTripPreservesTheLibrary(t *testing.T) {
	source, _, repos := newService(t)
	ctx := context.Background()
	seedLibrary(t, repos)

	raw, err := source.Export(ctx, DefaultExportOptions())
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	// A second, empty database is what a restore onto a new machine looks like.
	target, targetDB, targetRepos := newService(t)
	if _, err := target.Restore(ctx, raw); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	// Bookmarks and their relations came across.
	counts, err := targetRepos.Bookmarks.Counts(ctx)
	if err != nil {
		t.Fatalf("Counts: %v", err)
	}
	if counts.TotalBookmarks != 2 {
		t.Errorf("restored %d bookmarks, want 2", counts.TotalBookmarks)
	}
	if counts.TagCount != 1 {
		t.Errorf("restored %d tags, want 1", counts.TagCount)
	}
	if counts.PendingSuggestions != 1 {
		t.Errorf("restored %d pending suggestions, want 1", counts.PendingSuggestions)
	}

	tags, err := targetRepos.Bookmarks.TagsByBookmark(ctx, []string{"1"})
	if err != nil {
		t.Fatalf("TagsByBookmark: %v", err)
	}
	if len(tags["1"]) != 1 || tags["1"][0].Name != "Go" {
		t.Errorf("relations did not survive: %+v", tags["1"])
	}

	// The manual source survives, so a later automatic prune still spares it.
	var sourceKind string
	if err := targetDB.QueryRow(
		`SELECT source FROM bookmark_tags WHERE bookmark_id = '1'`).Scan(&sourceKind); err != nil {
		t.Fatalf("read source: %v", err)
	}
	if sourceKind != domain.TagSourceManual {
		t.Errorf("source = %q, want manual", sourceKind)
	}

	// A bookmark's list survives.
	lists, err := targetRepos.Bookmarks.ListByBookmark(ctx, []string{"2"})
	if err != nil {
		t.Fatalf("ListByBookmark: %v", err)
	}
	if lists["2"].ID != "l1" {
		t.Errorf("list = %q, want l1", lists["2"].ID)
	}

	// And the settings the allowlist permitted.
	settings, err := targetRepos.Settings.All(ctx)
	if err != nil {
		t.Fatalf("Settings.All: %v", err)
	}
	if _, present := settings["app.preferences"]; !present {
		t.Error("the exportable setting did not survive the round trip")
	}
}

// TestImportIsIdempotent is the property the gate names: importing the same bundle
// twice must leave the library exactly as one import did.
func TestImportIsIdempotent(t *testing.T) {
	source, _, repos := newService(t)
	ctx := context.Background()
	seedLibrary(t, repos)

	raw, err := source.Export(ctx, DefaultExportOptions())
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	target, targetDB, targetRepos := newService(t)

	first, err := target.Import(ctx, raw)
	if err != nil {
		t.Fatalf("first Import: %v", err)
	}
	if first.BookmarksInserted != 2 {
		t.Errorf("first import inserted %d bookmarks, want 2", first.BookmarksInserted)
	}

	before := snapshot(t, targetDB)

	second, err := target.Import(ctx, raw)
	if err != nil {
		t.Fatalf("second Import: %v", err)
	}
	if second.BookmarksInserted != 0 {
		t.Errorf("second import inserted %d bookmarks, want 0", second.BookmarksInserted)
	}
	if second.BookmarksUpdated != 2 {
		t.Errorf("second import updated %d bookmarks, want 2", second.BookmarksUpdated)
	}

	after := snapshot(t, targetDB)
	if before != after {
		t.Errorf("the library changed on a repeated import:\n before %s\n after  %s", before, after)
	}

	// And the counts did not multiply.
	counts, err := targetRepos.Bookmarks.Counts(ctx)
	if err != nil {
		t.Fatalf("Counts: %v", err)
	}
	if counts.TotalBookmarks != 2 || counts.TagCount != 1 {
		t.Errorf("counts = %+v, want 2 bookmarks and 1 tag", counts)
	}
}

// TestImportKeepsExistingData is what separates Import from Restore.
func TestImportKeepsExistingData(t *testing.T) {
	source, _, repos := newService(t)
	ctx := context.Background()
	seedLibrary(t, repos)

	raw, err := source.Export(ctx, DefaultExportOptions())
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	target, _, targetRepos := newService(t)
	// A bookmark that is not in the bundle.
	if _, err := targetRepos.Bookmarks.UpsertMany(ctx, []domain.Bookmark{{
		TweetID: "local", TweetURL: "https://x.com/x/status/9", Text: "local only",
		SavedAt: clockBase, LastSeenAt: clockBase, UpdatedAt: clockBase,
	}}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}

	if _, err := target.Import(ctx, raw); err != nil {
		t.Fatalf("Import: %v", err)
	}

	counts, err := targetRepos.Bookmarks.Counts(ctx)
	if err != nil {
		t.Fatalf("Counts: %v", err)
	}
	if counts.TotalBookmarks != 3 {
		t.Errorf("total = %d, want 3: an import must not delete what it does not contain", counts.TotalBookmarks)
	}
}

// TestRestoreReplacesTheLibrary is the extension's semantics.
func TestRestoreReplacesTheLibrary(t *testing.T) {
	source, _, repos := newService(t)
	ctx := context.Background()
	seedLibrary(t, repos)

	raw, err := source.Export(ctx, DefaultExportOptions())
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	target, _, targetRepos := newService(t)
	if _, err := targetRepos.Bookmarks.UpsertMany(ctx, []domain.Bookmark{{
		TweetID: "local", TweetURL: "https://x.com/x/status/9", Text: "local only",
		SavedAt: clockBase, LastSeenAt: clockBase, UpdatedAt: clockBase,
	}}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}

	if _, err := target.Restore(ctx, raw); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	if _, found, err := targetRepos.Bookmarks.Get(ctx, "local"); err != nil {
		t.Fatalf("Get: %v", err)
	} else if found {
		t.Error("a restore must remove data the bundle does not contain")
	}

	counts, err := targetRepos.Bookmarks.Counts(ctx)
	if err != nil {
		t.Fatalf("Counts: %v", err)
	}
	if counts.TotalBookmarks != 2 {
		t.Errorf("total = %d, want the bundle's 2", counts.TotalBookmarks)
	}
	// The Inbox is a system row and must survive the wipe.
	if counts.ListCount < 1 {
		t.Error("the Inbox must survive a restore")
	}
}

// TestRestoreValidatesBeforeDeleting is the safety property that matters most: a
// rejected bundle must leave the existing library exactly as it was.
func TestRestoreValidatesBeforeDeleting(t *testing.T) {
	service, db, repos := newService(t)
	ctx := context.Background()
	seedLibrary(t, repos)

	before := snapshot(t, db)

	// A corrupt bundle.
	if _, err := service.Restore(ctx, []byte("not a zip at all")); !errors.Is(err, ErrValidation) {
		t.Errorf("error = %v, want ErrValidation", err)
	}

	after := snapshot(t, db)
	if before != after {
		t.Errorf("a rejected restore changed the library:\n before %s\n after  %s", before, after)
	}
}

func TestImportRejectsACorruptBundleWithoutTouchingData(t *testing.T) {
	service, db, repos := newService(t)
	ctx := context.Background()
	seedLibrary(t, repos)

	before := snapshot(t, db)

	if _, err := service.Import(ctx, []byte("nonsense")); !errors.Is(err, ErrValidation) {
		t.Errorf("error = %v, want ErrValidation", err)
	}

	if after := snapshot(t, db); before != after {
		t.Error("a rejected import changed the library")
	}
}

// TestPreviewCountsNewAgainstExisting covers the merge preview, which is what
// lets a user see a merge before agreeing to it.
func TestPreviewCountsNewAgainstExisting(t *testing.T) {
	source, _, repos := newService(t)
	ctx := context.Background()
	seedLibrary(t, repos)

	raw, err := source.Export(ctx, DefaultExportOptions())
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	t.Run("everything is new in an empty library", func(t *testing.T) {
		target, _, _ := newService(t)

		preview, err := target.Preview(ctx, raw)
		if err != nil {
			t.Fatalf("Preview: %v", err)
		}
		if preview.NewRecords != 2 || preview.ExistingRecords != 0 {
			t.Errorf("preview = %+v, want 2 new", preview)
		}
		if preview.Manifest.Format != bundle.Format {
			t.Errorf("the preview must carry the manifest, got %q", preview.Manifest.Format)
		}
	})

	t.Run("everything is already present", func(t *testing.T) {
		target, _, targetRepos := newService(t)
		seedLibrary(t, targetRepos)

		preview, err := target.Preview(ctx, raw)
		if err != nil {
			t.Fatalf("Preview: %v", err)
		}
		if preview.NewRecords != 0 || preview.ExistingRecords != 2 {
			t.Errorf("preview = %+v, want 0 new and 2 existing", preview)
		}
	})
}

func TestExportUsesTheSettingsAllowlist(t *testing.T) {
	service, _, repos := newService(t)
	ctx := context.Background()
	seedLibrary(t, repos)

	// A sensitive value stored alongside the exportable ones.
	if err := repos.Settings.Set(ctx, "llm.endpoints", `{"custom":"https://internal.corp"}`, clockBase); err != nil {
		t.Fatalf("Set: %v", err)
	}

	raw, err := service.Export(ctx, DefaultExportOptions())
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	parsed, err := bundle.Read(newReader(raw), int64(len(raw)), bundle.DefaultLimits())
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	settings := string(parsed.Files[bundle.FileSettings])
	if strings.Contains(settings, "internal.corp") {
		t.Error("an endpoint leaked into the bundle")
	}
	if !strings.Contains(settings, "app.preferences") {
		t.Error("the exportable setting is missing from the bundle")
	}
}

// snapshot renders the library's shape, so two states can be compared directly.
//
// Every count is read into its own variable: scanning into throwaway values would
// make the comparison silently empty, which is the kind of test that passes
// without checking anything.
func snapshot(t *testing.T, db *sqlite.DB) string {
	t.Helper()

	var bookmarks, tags, relations, memberships, suggestions, lists int
	var ids string
	query := `
		SELECT
		  (SELECT COUNT(*) FROM bookmarks),
		  (SELECT COUNT(*) FROM tags),
		  (SELECT COUNT(*) FROM bookmark_tags),
		  (SELECT COUNT(*) FROM bookmark_lists),
		  (SELECT COUNT(*) FROM tag_suggestions),
		  (SELECT COUNT(*) FROM lists),
		  (SELECT COALESCE(GROUP_CONCAT(tweet_id), '') FROM bookmarks)`
	if err := db.QueryRow(query).Scan(
		&bookmarks, &tags, &relations, &memberships, &suggestions, &lists, &ids,
	); err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	return fmt.Sprintf("bookmarks=%d tags=%d relations=%d memberships=%d suggestions=%d lists=%d ids=%s",
		bookmarks, tags, relations, memberships, suggestions, lists, ids)
}

// newReader wraps bytes for bundle.Read.
func newReader(data []byte) io.ReaderAt { return bytes.NewReader(data) }

// TestSyncRunsAndAnalysisRoundTrip covers the two tables that were collected
// last: a restored library must not re-analyse every bookmark.
func TestSyncRunsAndAnalysisRoundTrip(t *testing.T) {
	source, _, repos := newService(t)
	ctx := context.Background()
	seedLibrary(t, repos)

	// A finished sync run and a verdict in the ledger.
	finishedAt := clockBase
	if err := repos.SyncRuns.Start(ctx, domain.SyncRun{
		ID: "run-1", Status: domain.SyncSuccess, StartedAt: clockBase,
		FinishedAt: &finishedAt, FetchedCount: 10, InsertedCount: 8,
		StrategyVersion: domain.SyncStrategyVersion,
	}); err != nil {
		t.Fatalf("SyncRuns.Start: %v", err)
	}
	if err := repos.SyncRuns.Finish(ctx, domain.SyncRun{
		ID: "run-1", Status: domain.SyncSuccess, StartedAt: clockBase,
		FinishedAt: &finishedAt, FetchedCount: 10, InsertedCount: 8,
		StrategyVersion: domain.SyncStrategyVersion,
	}); err != nil {
		t.Fatalf("SyncRuns.Finish: %v", err)
	}
	if _, err := repos.Analysis.UpsertMany(ctx, []domain.LlmAnalysis{{
		BookmarkID: "1", AnalyzedAt: domain.FormatTimestamp(clockBase),
		PromptVersion: domain.LlmPromptVersion, Model: "gpt-4o",
		ContentHash: domain.HashAnalyzedContent("first post"),
		Outcome:     domain.OutcomeTagged, TagCount: 1,
	}}); err != nil {
		t.Fatalf("Analysis.UpsertMany: %v", err)
	}

	raw, err := source.Export(ctx, DefaultExportOptions())
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	target, _, targetRepos := newService(t)
	report, err := target.Restore(ctx, raw)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}

	if report.SyncRunsAdded != 1 {
		t.Errorf("restored %d sync runs, want 1", report.SyncRunsAdded)
	}
	if report.AnalysisWritten != 1 {
		t.Errorf("restored %d analysis rows, want 1", report.AnalysisWritten)
	}

	// The ledger is what stops a restored library being re-analysed, so the hash
	// and model have to survive exactly.
	ledger, err := targetRepos.Analysis.List(ctx)
	if err != nil {
		t.Fatalf("Analysis.List: %v", err)
	}
	if len(ledger) != 1 {
		t.Fatalf("ledger has %d rows, want 1", len(ledger))
	}
	if ledger[0].Model != "gpt-4o" || ledger[0].ContentHash != domain.HashAnalyzedContent("first post") {
		t.Errorf("the verdict did not survive: %+v", ledger[0])
	}

	// And the verdict is still reusable for the same text.
	if !ledger[0].IsAnalysisReusable("gpt-4o", domain.HashAnalyzedContent("first post"), domain.LlmPromptVersion) {
		t.Error("the restored verdict must still be reusable, or the library is re-analysed")
	}
}

// TestSyncRunsAreNotExportedWhenExcluded covers the option.
func TestSyncRunsAreNotExportedWhenExcluded(t *testing.T) {
	service, _, repos := newService(t)
	ctx := context.Background()
	seedLibrary(t, repos)

	raw, err := service.Export(ctx, ExportOptions{IncludeSyncRuns: false})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	parsed, err := bundle.Read(newReader(raw), int64(len(raw)), bundle.DefaultLimits())
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if _, present := parsed.Files[bundle.FileSyncRuns]; present {
		t.Error("sync runs must not be present when excluded")
	}
}

// --- legacy flat-JSON backups ---

// legacyBackup renders a version 2 flat backup, the shape the extension wrote.
func legacyBackup(t *testing.T) []byte {
	t.Helper()

	payload := map[string]any{
		"exportVersion": 2,
		"schemaVersion": 9,
		"exportedAt":    "2026-10-12T14:22:31.000Z",
		"bookmarks": []map[string]any{
			{
				"tweetId": "legacy-1", "tweetUrl": "https://x.com/ada/status/legacy-1",
				"authorName": "Ada", "authorHandle": "ada", "text": "from a flat backup",
				"savedAt": "2026-10-12T12:00:00.000Z", "createdAtOnX": "Wed Oct 10 20:19:24 +0000 2018",
			},
		},
		"tags": []map[string]any{
			{"id": "lt1", "name": "Legacy", "createdAt": "2026-10-12T12:00:00.000Z"},
		},
		"bookmarkTags": []map[string]any{
			{"bookmarkId": "legacy-1", "tagId": "lt1", "createdAt": "2026-10-12T12:00:00.000Z"},
		},
		"settings": map[string]any{
			"savedViews": []map[string]any{
				{
					"id": "lv1", "name": "Unread", "query": "", "sortOrder": "timeline",
					"viewMode": "grid", "createdAt": "2026-10-12T12:00:00.000Z",
					"updatedAt": "2026-10-12T12:00:00.000Z",
				},
			},
		},
		"summary":       map[string]any{"status": "success"},
		"latestSyncRun": nil,
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal legacy: %v", err)
	}
	return encoded
}

// TestLegacyFlatBackupImports covers the compatibility path: refusing it would
// strand every user who has one.
func TestLegacyFlatBackupImports(t *testing.T) {
	service, _, repos := newService(t)
	ctx := context.Background()

	raw := legacyBackup(t)

	preview, err := service.Preview(ctx, raw)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if !preview.Legacy {
		t.Error("a flat backup must be reported as legacy")
	}
	if preview.NewRecords != 1 {
		t.Errorf("preview sees %d new records, want 1", preview.NewRecords)
	}

	report, err := service.Import(ctx, raw)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if report.BookmarksInserted != 1 {
		t.Errorf("imported %d bookmarks, want 1", report.BookmarksInserted)
	}
	if report.TagsInserted != 1 {
		t.Errorf("imported %d tags, want 1", report.TagsInserted)
	}
	if report.ViewsWritten != 1 {
		t.Errorf("imported %d saved views, want 1 (they live inside the settings blob)", report.ViewsWritten)
	}

	bookmark, found, err := repos.Bookmarks.Get(ctx, "legacy-1")
	if err != nil || !found {
		t.Fatalf("Get: found=%v err=%v", found, err)
	}
	if bookmark.Text != "from a flat backup" {
		t.Errorf("text = %q", bookmark.Text)
	}
	// The X rendering is preserved verbatim, and the sortable copy is derived.
	if bookmark.CreatedAtOnX != "Wed Oct 10 20:19:24 +0000 2018" {
		t.Errorf("createdAtOnX = %q, want it kept verbatim", bookmark.CreatedAtOnX)
	}

	// A tag whose canonical key was absent in the file is given one on import, or
	// the uniqueness the schema relies on would be defeated.
	tags, err := repos.Tags.List(ctx)
	if err != nil {
		t.Fatalf("Tags.List: %v", err)
	}
	if len(tags) != 1 {
		t.Fatalf("tags = %+v", tags)
	}
	if tags[0].CanonicalKey == "" {
		t.Error("the canonical key must be derived when the file omits it")
	}
}

// TestLegacyImportWarnsAboutTheCost is the warning the format document calls out:
// an old backup has no analysis ledger, so the first auto-tagging run re-analyses
// everything and charges the user's account.
func TestLegacyImportWarnsAboutTheCost(t *testing.T) {
	service, _, _ := newService(t)
	ctx := context.Background()

	raw := legacyBackup(t)

	preview, err := service.Preview(ctx, raw)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}

	kinds := map[string]bool{}
	for _, warning := range preview.Warnings {
		kinds[warning.Kind] = true
	}
	for _, want := range []string{WarningNoAnalysisLedger, WarningNoSuggestions, WarningNoChecksum} {
		if !kinds[want] {
			t.Errorf("the preview must warn about %q", want)
		}
	}

	// The cost warning must actually say what it costs.
	for _, warning := range preview.Warnings {
		if warning.Kind == WarningNoAnalysisLedger {
			if !strings.Contains(warning.Detail, "quota") {
				t.Errorf("the ledger warning must mention the cost, got %q", warning.Detail)
			}
		}
	}

	report, err := service.Import(ctx, raw)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if len(report.Warnings) == 0 {
		t.Error("the import report must carry the same warnings")
	}
}

// TestLegacyFoldersBecomeLists covers a version 1 backup, which predates the
// lists removal and carries folders.
func TestLegacyFoldersBecomeLists(t *testing.T) {
	service, _, repos := newService(t)
	ctx := context.Background()

	payload := map[string]any{
		"exportVersion": 1,
		"schemaVersion": 8,
		"exportedAt":    "2026-10-12T14:22:31.000Z",
		"bookmarks": []map[string]any{
			{"tweetId": "f1", "text": "in a folder", "savedAt": "2026-10-12T12:00:00.000Z"},
		},
		"tags":         []map[string]any{},
		"bookmarkTags": []map[string]any{},
		"folders": []map[string]any{
			{"id": "folder-1", "name": "Reading", "createdAt": "2026-10-12T12:00:00.000Z"},
			// The Inbox must never be taken from a file.
			{"id": domain.InboxListID, "name": "Inbox"},
		},
		"bookmarkFolders": []map[string]any{
			{"bookmarkId": "f1", "folderId": "folder-1"},
		},
		"settings": map[string]any{},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if _, err := service.Import(ctx, raw); err != nil {
		t.Fatalf("Import: %v", err)
	}

	lists, err := repos.Lists.List(ctx)
	if err != nil {
		t.Fatalf("Lists.List: %v", err)
	}
	names := map[string]bool{}
	for _, list := range lists {
		names[list.Name] = true
	}
	if !names["Reading"] {
		t.Errorf("the folder did not become a list: %+v", lists)
	}

	membership, err := repos.Bookmarks.ListByBookmark(ctx, []string{"f1"})
	if err != nil {
		t.Fatalf("ListByBookmark: %v", err)
	}
	if membership["f1"].ID != "folder-1" {
		t.Errorf("membership = %+v, want folder-1", membership["f1"])
	}
}

// TestLegacyUnsupportedVersionIsRejected covers the whitelist: a version this
// build does not know is refused rather than guessed at.
func TestLegacyUnsupportedVersionIsRejected(t *testing.T) {
	service, _, _ := newService(t)

	cases := []string{
		`{"exportVersion":99,"bookmarks":[]}`,
		// No version at all: some other JSON file, not a backup.
		`{"bookmarks":[]}`,
	}
	for _, raw := range cases {
		if _, err := service.Preview(context.Background(), []byte(raw)); err == nil {
			t.Errorf("%s must be rejected", raw)
		}
	}
}

func TestLooksLikeLegacy(t *testing.T) {
	if !LooksLikeLegacy([]byte("  \n {\"exportVersion\":2}")) {
		t.Error("leading whitespace must not hide a JSON file")
	}
	if LooksLikeLegacy([]byte("PK\x03\x04")) {
		t.Error("a ZIP must not be treated as JSON")
	}
	if LooksLikeLegacy([]byte("")) {
		t.Error("empty input is not a legacy backup")
	}
}

// TestABadSavedViewIsSkippedNotFatal pins the behaviour round 168 recorded as a divergence.
//
// The extension normalises each saved view and drops the ones it cannot use -- settings.ts:89
// maps then filters the nulls -- so one unusable view in a settings blob leaves the rest
// working. This side writes the views into a table whose schema carries
// `CHECK (sort_order IN (...))` and `CHECK (view_mode IN ('grid','list'))`, so a bad value
// cannot be stored; the import is the one path where one can arrive, from a hand-edited file,
// and it skips that row rather than failing the whole restore.
//
// The test exists because "recorded divergence, deliberately not changed" and "the divergence
// is closed" look identical in a log entry, and only a test tells them apart.
func TestABadSavedViewIsSkippedNotFatal(t *testing.T) {
	service, _, repos := newService(t)
	ctx := context.Background()

	payload := map[string]any{
		"exportVersion": 2,
		"schemaVersion": 9,
		"exportedAt":    "2026-10-12T14:22:31.000Z",
		"bookmarks":     []map[string]any{},
		"tags":          []map[string]any{},
		"bookmarkTags":  []map[string]any{},
		"settings": map[string]any{
			"savedViews": []map[string]any{
				{
					"id": "good", "name": "Unread", "query": "", "sortOrder": "timeline",
					"viewMode": "grid", "createdAt": "2026-10-12T12:00:00.000Z",
					"updatedAt": "2026-10-12T12:00:00.000Z",
				},
				{
					// A sort order this build does not know. The extension would drop it here;
					// the schema would reject it later.
					"id": "bad", "name": "Broken", "query": "", "sortOrder": "yolo",
					"viewMode": "grid", "createdAt": "2026-10-12T12:00:00.000Z",
					"updatedAt": "2026-10-12T12:00:00.000Z",
				},
			},
		},
		"summary":       map[string]any{"status": "success"},
		"latestSyncRun": nil,
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	report, err := service.Import(ctx, raw)
	if err != nil {
		t.Fatalf("one unusable saved view must not fail the restore: %v", err)
	}
	if report.ViewsWritten != 1 {
		t.Errorf("wrote %d saved views, want 1: the bad one is skipped and the good one kept",
			report.ViewsWritten)
	}

	views, err := repos.SavedViews.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(views) != 1 || views[0].ID != "good" {
		t.Errorf("stored views = %+v, want only the usable one", views)
	}
}

// extensionBackupView is the part of the extension's validator this package must satisfy,
// spelled out so the test fails on the same fields the extension would reject.
//
// See src/lib/export/workspaceBackup.ts:286-425: locale and themePreference are required,
// summary must be a SyncSummary, a bookmarkTag needs an id, a saved view needs both arrays,
// and a metrics object needs all three counts.
type extensionBackupView struct {
	ExportVersion int    `json:"exportVersion"`
	SchemaVersion int    `json:"schemaVersion"`
	ExportedAt    string `json:"exportedAt"`
	Bookmarks     []struct {
		TweetID      string `json:"tweetId"`
		TweetURL     string `json:"tweetUrl"`
		AuthorName   string `json:"authorName"`
		AuthorHandle string `json:"authorHandle"`
		Text         string `json:"text"`
		CreatedAtOnX string `json:"createdAtOnX"`
		SavedAt      string `json:"savedAt"`
		BookmarkRank *int   `json:"bookmarkTimelineRank"`
		Metrics      *struct {
			Likes    *int `json:"likes"`
			Retweets *int `json:"retweets"`
			Replies  *int `json:"replies"`
		} `json:"metrics"`
	} `json:"bookmarks"`
	Tags []struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		CreatedAt string `json:"createdAt"`
	} `json:"tags"`
	BookmarkTags []struct {
		ID         string `json:"id"`
		BookmarkID string `json:"bookmarkId"`
		TagID      string `json:"tagId"`
		CreatedAt  string `json:"createdAt"`
	} `json:"bookmarkTags"`
	Settings struct {
		SchemaVersion   int    `json:"schemaVersion"`
		Locale          string `json:"locale"`
		ThemePreference string `json:"themePreference"`
		SavedViews      []struct {
			ActiveTagIDs        []string `json:"activeTagIds"`
			ActiveAuthorHandles []string `json:"activeAuthorHandles"`
			SortOrder           string   `json:"sortOrder"`
			ViewMode            string   `json:"viewMode"`
		} `json:"savedViews"`
	} `json:"settings"`
	Summary struct {
		Status        string `json:"status"`
		FetchedCount  int    `json:"fetchedCount"`
		InsertedCount int    `json:"insertedCount"`
		UpdatedCount  int    `json:"updatedCount"`
		FailedCount   int    `json:"failedCount"`
		LastSyncedAt  string `json:"lastSyncedAt"`
		ErrorKind     string `json:"errorKind"`
	} `json:"summary"`
	LatestSyncRun *struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	} `json:"latestSyncRun"`
}

// TestExportExtensionJSONMeetsTheExtensionsContract covers the reason the format exists:
// the extension only reads its own flat JSON, and it validates every record before it
// restores anything. A round trip through the native parser is not the contract -- the
// native parser was written to be generous, and it accepted an export the extension
// rejected because summary was `{}` and the settings had no themePreference.
func TestExportExtensionJSONMeetsTheExtensionsContract(t *testing.T) {
	service, _, repos := newService(t)
	ctx := context.Background()
	seedLibrary(t, repos)

	// Preferences carry both fields the extension requires.
	if err := repos.Settings.Set(ctx, "app.preferences",
		`{"appearance":"dark","locale":"zh-CN"}`, clockBase); err != nil {
		t.Fatalf("Settings.Set: %v", err)
	}

	// A count the native DTO keeps absent, which the extension requires as a number.
	likes := 7
	if _, err := repos.Bookmarks.UpsertMany(ctx, []domain.Bookmark{{
		TweetID: "1", TweetURL: "https://x.com/ada/status/1", AuthorName: "Ada",
		AuthorHandle: "ada", Text: "first post", SavedAt: clockBase,
		LastSeenAt: clockBase, UpdatedAt: clockBase,
		Metrics: &domain.Metrics{Likes: &likes},
	}}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}

	// A view with no facets: both arrays must still be present.
	if err := repos.SavedViews.Upsert(ctx, domain.SavedView{
		ID: "v1", Name: "Everything", Query: "", SortOrder: domain.SortTimeline,
		ViewMode: domain.ViewGrid, CreatedAt: clockBase, UpdatedAt: clockBase,
	}); err != nil {
		t.Fatalf("SavedViews.Upsert: %v", err)
	}

	// A cancelled run, which the extension has no status for, and a native-only error
	// kind, which it has no name for: both must be mapped rather than passed through.
	finished := clockBase
	if err := repos.SyncRuns.Start(ctx, domain.SyncRun{
		ID: "run-1", Status: domain.SyncCancelled, StartedAt: clockBase,
		ErrorKind: domain.ErrNetwork, StrategyVersion: domain.SyncStrategyVersion,
	}); err != nil {
		t.Fatalf("SyncRuns.Start: %v", err)
	}
	if err := repos.SyncRuns.Finish(ctx, domain.SyncRun{
		ID: "run-1", Status: domain.SyncCancelled, StartedAt: clockBase,
		FinishedAt: &finished, FetchedCount: 10, InsertedCount: 8, FailedCount: 2,
		ErrorKind: domain.ErrNetwork, StrategyVersion: domain.SyncStrategyVersion,
	}); err != nil {
		t.Fatalf("SyncRuns.Finish: %v", err)
	}

	raw, err := service.ExportExtensionJSON(ctx)
	if err != nil {
		t.Fatalf("ExportExtensionJSON: %v", err)
	}

	var payload extensionBackupView
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("the export is not readable JSON: %v", err)
	}
	if payload.ExportVersion != 2 || payload.SchemaVersion != 7 {
		t.Errorf("versions = %d/%d, want 2/7", payload.ExportVersion, payload.SchemaVersion)
	}
	if payload.ExportedAt == "" {
		t.Error("exportedAt is empty")
	}

	// Settings: both required fields, with the native values mapped onto the extension's
	// two locales and three themes.
	if payload.Settings.Locale != "zh-CN" || payload.Settings.ThemePreference != "dark" {
		t.Errorf("settings = %q/%q, want zh-CN/dark",
			payload.Settings.Locale, payload.Settings.ThemePreference)
	}
	if payload.Settings.SchemaVersion != 7 {
		t.Errorf("settings.schemaVersion = %d, want 7", payload.Settings.SchemaVersion)
	}
	if len(payload.Settings.SavedViews) != 1 {
		t.Fatalf("savedViews = %d, want the one stored", len(payload.Settings.SavedViews))
	}
	view := payload.Settings.SavedViews[0]
	if view.ActiveTagIDs == nil || view.ActiveAuthorHandles == nil {
		t.Error("a saved view's facet arrays must be present, even when empty")
	}
	if view.SortOrder != "timeline" || view.ViewMode != "grid" {
		t.Errorf("saved view = %q/%q, want timeline/grid", view.SortOrder, view.ViewMode)
	}

	// Summary: a real run, in the extension's vocabulary.
	if payload.Summary.Status != "error" {
		t.Errorf("summary.status = %q, want error for a cancelled run", payload.Summary.Status)
	}
	if payload.Summary.ErrorKind != "network_error" {
		t.Errorf("summary.errorKind = %q, want network_error", payload.Summary.ErrorKind)
	}
	if payload.Summary.FetchedCount != 10 || payload.Summary.InsertedCount != 8 {
		t.Errorf("summary counts = %d/%d, want 10/8",
			payload.Summary.FetchedCount, payload.Summary.InsertedCount)
	}
	if payload.Summary.LastSyncedAt == "" {
		t.Error("summary.lastSyncedAt is empty")
	}
	if payload.LatestSyncRun == nil || payload.LatestSyncRun.ID != "run-1" ||
		payload.LatestSyncRun.Status != "error" {
		t.Errorf("latestSyncRun = %+v, want the mapped run", payload.LatestSyncRun)
	}

	// Relations: a unique id in the extension's own form.
	if len(payload.BookmarkTags) != 1 {
		t.Fatalf("bookmarkTags = %d, want the one attached", len(payload.BookmarkTags))
	}
	relation := payload.BookmarkTags[0]
	if relation.ID != "1:t1" {
		t.Errorf("bookmarkTag.id = %q, want 1:t1", relation.ID)
	}
	if relation.BookmarkID != "1" || relation.TagID != "t1" || relation.CreatedAt == "" {
		t.Errorf("bookmarkTag = %+v, want the stored relation", relation)
	}

	// Bookmarks: the counts exist as numbers whenever metrics does.
	bookmark := payload.Bookmarks[0]
	if bookmark.Metrics == nil {
		t.Fatal("the seeded metrics are missing")
	}
	if bookmark.Metrics.Likes == nil || *bookmark.Metrics.Likes != 7 {
		t.Errorf("metrics.likes = %v, want 7", bookmark.Metrics.Likes)
	}
	if bookmark.Metrics.Retweets == nil || bookmark.Metrics.Replies == nil {
		t.Error("an absent count must be written as 0, not omitted")
	}

	for _, tag := range payload.Tags {
		if tag.ID == "" || tag.Name == "" || tag.CreatedAt == "" {
			t.Errorf("tag = %+v, wants id, name and createdAt", tag)
		}
	}

	// The native importer still accepts the same bytes: one encoder, two readers.
	legacy, _, err := ParseLegacy(raw)
	if err != nil {
		t.Fatalf("ParseLegacy rejected the export: %v", err)
	}
	if len(legacy.Bookmarks) != 2 {
		t.Errorf("bookmarks = %d, want 2", len(legacy.Bookmarks))
	}
	report, err := service.Import(ctx, raw)
	if err != nil {
		t.Fatalf("Import of our own export: %v", err)
	}
	if report.BookmarksUpdated+report.BookmarksInserted != 2 {
		t.Errorf("report = %+v, want both bookmarks counted", report)
	}
}

// TestExportExtensionJSONPassesTheRealValidator runs the extension's own validation over the
// export instead of trusting the Go test's copy of its rules.
//
// The duplicate above is deliberate: this one is skipped when the extension's toolchain is
// not installed, so the package still tests on a machine that only builds Go. When tsx is
// present, the actual TypeScript validator decides, and a drift in its required fields
// fails here before a user discovers it by losing a backup.
func TestExportExtensionJSONPassesTheRealValidator(t *testing.T) {
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	tsx := filepath.Join(root, "node_modules", ".bin", "tsx")
	if _, err := os.Stat(tsx); err != nil {
		t.Skip("tsx is not installed; run npm install to enable this check")
	}

	service, _, repos := newService(t)
	ctx := context.Background()
	seedLibrary(t, repos)
	likes := 1
	if _, err := repos.Bookmarks.UpsertMany(ctx, []domain.Bookmark{{
		TweetID: "1", TweetURL: "https://x.com/ada/status/1", AuthorName: "Ada",
		AuthorHandle: "ada", Text: "first post", SavedAt: clockBase,
		LastSeenAt: clockBase, UpdatedAt: clockBase,
		Metrics: &domain.Metrics{Likes: &likes},
	}}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}

	// A saved view with no facets and a run in the native-only vocabulary, so the real
	// validator checks those fields as well as the records seedLibrary writes.
	if err := repos.SavedViews.Upsert(ctx, domain.SavedView{
		ID: "v1", Name: "Everything", Query: "", SortOrder: domain.SortTimeline,
		ViewMode: domain.ViewGrid, CreatedAt: clockBase, UpdatedAt: clockBase,
	}); err != nil {
		t.Fatalf("SavedViews.Upsert: %v", err)
	}
	finished := clockBase
	if err := repos.SyncRuns.Start(ctx, domain.SyncRun{
		ID: "run-1", Status: domain.SyncCancelled, StartedAt: clockBase,
		ErrorKind: domain.ErrNetwork, StrategyVersion: domain.SyncStrategyVersion,
	}); err != nil {
		t.Fatalf("SyncRuns.Start: %v", err)
	}
	if err := repos.SyncRuns.Finish(ctx, domain.SyncRun{
		ID: "run-1", Status: domain.SyncCancelled, StartedAt: clockBase,
		FinishedAt: &finished, FetchedCount: 10, InsertedCount: 8, FailedCount: 2,
		ErrorKind: domain.ErrNetwork, StrategyVersion: domain.SyncStrategyVersion,
	}); err != nil {
		t.Fatalf("SyncRuns.Finish: %v", err)
	}

	raw, err := service.ExportExtensionJSON(ctx)
	if err != nil {
		t.Fatalf("ExportExtensionJSON: %v", err)
	}

	dir := t.TempDir()
	backup := filepath.Join(dir, "backup.json")
	if err := os.WriteFile(backup, raw, 0o600); err != nil {
		t.Fatalf("write backup: %v", err)
	}
	validator := filepath.Join(dir, "validate.mjs")
	script := fmt.Sprintf(`import fs from "node:fs"
import { validateWorkspaceBackupText } from %q
try {
  validateWorkspaceBackupText(fs.readFileSync(process.argv[2], "utf8"))
} catch (error) {
  console.error(String(error))
  process.exit(1)
}
`, filepath.Join(root, "src", "lib", "export", "workspaceBackup.ts"))
	if err := os.WriteFile(validator, []byte(script), 0o600); err != nil {
		t.Fatalf("write validator: %v", err)
	}

	command := exec.Command(tsx, validator, backup)
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("the extension rejected the export: %v\n%s", err, output)
	}
}

// TestAnImportedTagHierarchyIsReducedToTwoLevels covers the shapes the extension rejects
// before restoring, which this app's importer used to accept.
//
// A bundle is written by this app, but a hand-edited file or one from a future version is
// still input: a self-parent, a cycle and a third level each produce a library the parent
// picker cannot represent. They are reduced to roots, which is the same repair the importer
// already applies to a parent outside the file.
func TestAnImportedTagHierarchyIsReducedToTwoLevels(t *testing.T) {
	service, _, repos := newService(t)
	ctx := context.Background()

	backup := `{
	  "exportVersion": 2,
	  "schemaVersion": 7,
	  "bookmarks": [],
	  "bookmarkTags": [],
	  "settings": {},
	  "tags": [
	    {"id":"r","name":"Root","createdAt":"2026-10-12T12:00:00.000Z"},
	    {"id":"c","name":"Child","parentId":"r","createdAt":"2026-10-12T12:00:00.000Z"},
	    {"id":"g","name":"Grandchild","parentId":"c","createdAt":"2026-10-12T12:00:00.000Z"},
	    {"id":"s","name":"Self","parentId":"s","createdAt":"2026-10-12T12:00:00.000Z"},
	    {"id":"a","name":"A","parentId":"b","createdAt":"2026-10-12T12:00:00.000Z"},
	    {"id":"b","name":"B","parentId":"a","createdAt":"2026-10-12T12:00:00.000Z"}
	  ]
	}`
	if _, err := service.Import(ctx, []byte(backup)); err != nil {
		t.Fatalf("Import: %v", err)
	}

	tags, err := repos.Tags.List(ctx)
	if err != nil {
		t.Fatalf("Tags.List: %v", err)
	}
	parentOf := make(map[string]string, len(tags))
	for _, tag := range tags {
		parentOf[tag.Name] = tag.ParentID
	}

	if parentOf["Root"] != "" {
		t.Errorf("Root parent = %q, want a root", parentOf["Root"])
	}
	if parentOf["Child"] != "r" {
		t.Errorf("Child parent = %q, want r: one level is legal", parentOf["Child"])
	}
	if parentOf["Grandchild"] != "" {
		t.Errorf("Grandchild parent = %q, want a root: the hierarchy is two levels", parentOf["Grandchild"])
	}
	if parentOf["Self"] != "" {
		t.Errorf("Self parent = %q, want a root", parentOf["Self"])
	}
	if parentOf["A"] != "" || parentOf["B"] != "" {
		t.Errorf("cycle = %q/%q, want both reduced to roots", parentOf["A"], parentOf["B"])
	}
}

// TestARelationToAMissingBookmarkIsSkipped covers the abort the foreign key used to cause.
//
// A backup whose relations name a bookmark it does not carry reached SQLite's foreign key
// after the tags and bookmarks had already been written, so a merge failed half applied.
// The valid relations are imported and the dangling ones are dropped.
func TestARelationToAMissingBookmarkIsSkipped(t *testing.T) {
	service, _, repos := newService(t)
	ctx := context.Background()

	backup := `{
	  "exportVersion": 2,
	  "schemaVersion": 7,
	  "bookmarks": [
	    {"tweetId":"1","tweetUrl":"https://x.com/ada/status/1","authorName":"Ada",
	     "authorHandle":"ada","text":"a post","createdAtOnX":"","savedAt":"2026-10-12T12:00:00.000Z"}
	  ],
	  "bookmarkTags": [
	    {"bookmarkId":"1","tagId":"t1","createdAt":"2026-10-12T12:00:00.000Z"},
	    {"bookmarkId":"ghost","tagId":"t1","createdAt":"2026-10-12T12:00:00.000Z"}
	  ],
	  "settings": {},
	  "tags": [{"id":"t1","name":"Go","createdAt":"2026-10-12T12:00:00.000Z"}]
	}`
	report, err := service.Import(ctx, []byte(backup))
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if report.RelationsAdded != 1 {
		t.Errorf("RelationsAdded = %d, want only the relation whose bookmark exists", report.RelationsAdded)
	}
	relations, err := repos.Tags.AllRelations(ctx)
	if err != nil {
		t.Fatalf("AllRelations: %v", err)
	}
	if len(relations) != 1 || relations[0].BookmarkID != "1" {
		t.Errorf("relations = %+v, want only bookmark 1", relations)
	}
}
