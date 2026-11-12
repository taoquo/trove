package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/taoquo/x-master/apps/macos/internal/domain"
)

// The repository methods no current usecase calls.
//
// A port interface legitimately promises more than today's callers need, but an
// untested implementation is one a future caller would trust without reason. These
// tests exercise them so the promise is real, and the coverage report names which
// are currently unused.

func TestTagUpdatePersists(t *testing.T) {
	db := openTestDB(t)
	tags := NewTagRepository(db)
	ctx := context.Background()

	tag := domain.Tag{
		ID: "t1", Name: "Go", CanonicalKey: domain.CanonicalTagKey("Go"),
		Aliases: []string{"golang"}, ParentID: "",
		CreatedAt: domain.FormatTimestamp(clockBase), UpdatedAt: domain.FormatTimestamp(clockBase),
	}
	if err := tags.Create(ctx, tag); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// A rename, plus an alias absorbed by a merge, plus a parent.
	child := domain.Tag{
		ID: "t2", Name: "Rust", CanonicalKey: domain.CanonicalTagKey("Rust"),
		CreatedAt: domain.FormatTimestamp(clockBase), UpdatedAt: domain.FormatTimestamp(clockBase),
	}
	if err := tags.Create(ctx, child); err != nil {
		t.Fatalf("Create child: %v", err)
	}

	tag.Name = "Golang"
	tag.CanonicalKey = domain.CanonicalTagKey("Golang")
	tag.Aliases = []string{"golang", "go-lang"}
	tag.UpdatedAt = domain.FormatTimestamp(clockBase.Add(time.Hour))

	if err := tags.Update(ctx, tag); err != nil {
		t.Fatalf("Update: %v", err)
	}

	stored, found, err := tags.Get(ctx, "t1")
	if err != nil || !found {
		t.Fatalf("Get: found=%v err=%v", found, err)
	}
	if stored.Name != "Golang" || stored.CanonicalKey != domain.CanonicalTagKey("Golang") {
		t.Errorf("stored = %+v, want the renamed tag", stored)
	}
	if len(stored.Aliases) != 2 {
		t.Errorf("aliases = %v, want both", stored.Aliases)
	}

	// Setting a parent goes through the hierarchy trigger.
	tag.ParentID = "t2"
	if err := tags.Update(ctx, tag); err != nil {
		t.Fatalf("Update with parent: %v", err)
	}
	stored, _, err = tags.Get(ctx, "t1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if stored.ParentID != "t2" {
		t.Errorf("parent = %q, want t2", stored.ParentID)
	}

	// And the trigger still refuses a third level, even through an update.
	deep := domain.Tag{
		ID: "t3", Name: "Deep", CanonicalKey: domain.CanonicalTagKey("Deep"),
		ParentID: "t1", CreatedAt: domain.FormatTimestamp(clockBase), UpdatedAt: domain.FormatTimestamp(clockBase),
	}
	if err := tags.Create(ctx, deep); err == nil {
		t.Error("a third level must be refused by the trigger")
	}
}

func TestTagDetachRemovesOneRelation(t *testing.T) {
	db := openTestDB(t)
	tags := NewTagRepository(db)
	ctx := context.Background()

	if _, err := NewBookmarkRepository(db).UpsertMany(ctx, []domain.Bookmark{
		bookmarkFixture("a", clockBase), bookmarkFixture("b", clockBase),
	}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}

	for _, tag := range []domain.Tag{
		{ID: "t1", Name: "Go", CanonicalKey: "go"},
		{ID: "t2", Name: "Rust", CanonicalKey: "rust"},
	} {
		tag.CreatedAt = domain.FormatTimestamp(clockBase)
		tag.UpdatedAt = domain.FormatTimestamp(clockBase)
		if err := tags.Create(ctx, tag); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	if err := tags.AttachMany(ctx, []domain.BookmarkTag{
		{BookmarkID: "a", TagID: "t1", CreatedAt: clockBase, Source: domain.TagSourceManual},
		{BookmarkID: "a", TagID: "t2", CreatedAt: clockBase, Source: domain.TagSourceManual},
	}); err != nil {
		t.Fatalf("AttachMany: %v", err)
	}

	if err := tags.Detach(ctx, "a", "t1"); err != nil {
		t.Fatalf("Detach: %v", err)
	}

	byBookmark, err := NewBookmarkRepository(db).TagsByBookmark(ctx, []string{"a"})
	if err != nil {
		t.Fatalf("TagsByBookmark: %v", err)
	}
	if len(byBookmark["a"]) != 1 || byBookmark["a"][0].ID != "t2" {
		t.Errorf("remaining tags = %+v, want only t2", byBookmark["a"])
	}

	// Detaching one that is not there is a no-op, not an error.
	if err := tags.Detach(ctx, "b", "t1"); err != nil {
		t.Errorf("detaching an absent relation: %v", err)
	}
}

func TestBookmarkDeleteRemovesOne(t *testing.T) {
	db := openTestDB(t)
	repo := NewBookmarkRepository(db)
	ctx := context.Background()

	if _, err := repo.UpsertMany(ctx, []domain.Bookmark{
		bookmarkFixture("keep", clockBase), bookmarkFixture("drop", clockBase),
	}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}

	if err := repo.Delete(ctx, "drop"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, found, err := repo.Get(ctx, "drop"); err != nil || found {
		t.Errorf("the deleted bookmark must be gone: found=%v err=%v", found, err)
	}
	if _, found, err := repo.Get(ctx, "keep"); err != nil || !found {
		t.Errorf("the other bookmark must survive: found=%v err=%v", found, err)
	}

	// Deleting an absent id is a no-op.
	if err := repo.Delete(ctx, "never-existed"); err != nil {
		t.Errorf("deleting an absent bookmark: %v", err)
	}
}

func TestRawPayloadsAreStoredAndReadBack(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	if _, err := NewBookmarkRepository(db).UpsertMany(ctx,
		[]domain.Bookmark{bookmarkFixture("a", clockBase)}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}

	payload := `{"data":{"bookmark_timeline_v2":{"timeline":{"instructions":[]}}}}`
	if _, err := db.Exec(
		`INSERT INTO raw_payloads (tweet_id, payload_json, captured_at) VALUES (?, ?, ?)`,
		"a", payload, domain.FormatTimestamp(clockBase)); err != nil {
		t.Fatalf("insert payload: %v", err)
	}

	payloads, err := NewBookmarkRepository(db).RawPayloads(ctx, []string{"a", "missing"})
	if err != nil {
		t.Fatalf("RawPayloads: %v", err)
	}
	if len(payloads) != 1 {
		t.Fatalf("got %d payloads, want 1", len(payloads))
	}
	if payloads[0].TweetID != "a" || payloads[0].PayloadJSON != payload {
		t.Errorf("payload = %+v, want it stored verbatim", payloads[0])
	}

	// An empty request is not a query.
	if got, err := NewBookmarkRepository(db).RawPayloads(ctx, nil); err != nil || got != nil {
		t.Errorf("RawPayloads(nil) = %v, %v", got, err)
	}
}

// TestExistingForComparesTheContentHash is the ledger's reuse check. A verdict is
// only reusable while the text it was made about is unchanged, so the hash decides.
func TestExistingForComparesTheContentHash(t *testing.T) {
	db := openTestDB(t)
	repo := NewAnalysisRepository(db)
	ctx := context.Background()

	// llm_analysis has a foreign key to bookmarks, so the bookmarks come first.
	if _, err := NewBookmarkRepository(db).UpsertMany(ctx, []domain.Bookmark{
		bookmarkFixture("same", clockBase), bookmarkFixture("edited", clockBase),
	}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}

	unchanged := domain.HashAnalyzedContent("the text that was analysed")
	changed := domain.HashAnalyzedContent("the text has since been edited")

	if _, err := repo.UpsertMany(ctx, []domain.LlmAnalysis{
		{
			BookmarkID: "same", AnalyzedAt: domain.FormatTimestamp(clockBase),
			PromptVersion: domain.LlmPromptVersion, Model: "gpt-4o",
			ContentHash: unchanged, Outcome: domain.OutcomeTagged, TagCount: 1,
		},
		{
			BookmarkID: "edited", AnalyzedAt: domain.FormatTimestamp(clockBase),
			PromptVersion: domain.LlmPromptVersion, Model: "gpt-4o",
			ContentHash: unchanged, Outcome: domain.OutcomeTagged, TagCount: 1,
		},
	}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}

	reusable, err := repo.ExistingFor(ctx, "gpt-4o", domain.LlmPromptVersion, map[string]string{
		"same":   unchanged, // the text is what it was
		"edited": changed,   // the text has moved on
	})
	if err != nil {
		t.Fatalf("ExistingFor: %v", err)
	}

	if _, present := reusable["same"]; !present {
		t.Error("a verdict about unchanged text must be reusable")
	}
	if _, present := reusable["edited"]; present {
		t.Error("a verdict about edited text must not be reused")
	}

	// A different model's verdicts are not reusable either.
	other, err := repo.ExistingFor(ctx, "claude-3", domain.LlmPromptVersion, map[string]string{"same": unchanged})
	if err != nil {
		t.Fatalf("ExistingFor other model: %v", err)
	}
	if len(other) != 0 {
		t.Errorf("got %v, want nothing for a different model", other)
	}

	// Nor is a verdict from a different prompt version.
	older, err := repo.ExistingFor(ctx, "gpt-4o", domain.LlmPromptVersion+1, map[string]string{"same": unchanged})
	if err != nil {
		t.Fatalf("ExistingFor other version: %v", err)
	}
	if len(older) != 0 {
		t.Errorf("got %v, want nothing for a different prompt version", older)
	}

	if got, err := repo.ExistingFor(ctx, "gpt-4o", domain.LlmPromptVersion, nil); err != nil || len(got) != 0 {
		t.Errorf("ExistingFor(nil) = %v, %v", got, err)
	}
}

func TestAnalysisDeleteAll(t *testing.T) {
	db := openTestDB(t)
	repo := NewAnalysisRepository(db)
	ctx := context.Background()

	if _, err := NewBookmarkRepository(db).UpsertMany(ctx,
		[]domain.Bookmark{bookmarkFixture("a", clockBase)}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}
	if _, err := repo.UpsertMany(ctx, []domain.LlmAnalysis{{
		BookmarkID: "a", AnalyzedAt: domain.FormatTimestamp(clockBase),
		PromptVersion: domain.LlmPromptVersion, Model: "gpt-4o",
		ContentHash: "hash", Outcome: domain.OutcomeTagged,
	}}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}

	removed, err := repo.DeleteAll(ctx)
	if err != nil {
		t.Fatalf("DeleteAll: %v", err)
	}
	if removed != 1 {
		t.Errorf("removed %d, want 1", removed)
	}

	// Idempotent.
	if again, err := repo.DeleteAll(ctx); err != nil || again != 0 {
		t.Errorf("second DeleteAll = %d, %v", again, err)
	}
}

func TestSavedViewRoundTrip(t *testing.T) {
	db := openTestDB(t)
	repo := NewSavedViewRepository(db)
	ctx := context.Background()

	view := domain.SavedView{
		ID: "v1", Name: "Unread", Query: "kubernetes",
		ActiveTagIDs: []string{"t1", "t2"}, ActiveAuthorHandles: []string{"ada"},
		OnlyWithMedia: true, OnlyLongform: true,
		SortOrder: domain.SortLikesDesc, ViewMode: domain.ViewList,
		CreatedAt: clockBase, UpdatedAt: clockBase,
	}
	if err := repo.Upsert(ctx, view); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	stored, found, err := repo.Get(ctx, "v1")
	if err != nil || !found {
		t.Fatalf("Get: found=%v err=%v", found, err)
	}
	if stored.Name != "Unread" || stored.Query != "kubernetes" {
		t.Errorf("stored = %+v", stored)
	}
	if len(stored.ActiveTagIDs) != 2 || len(stored.ActiveAuthorHandles) != 1 {
		t.Errorf("filters did not round-trip: %+v", stored)
	}
	if !stored.OnlyWithMedia || !stored.OnlyLongform {
		t.Error("the boolean filters did not round-trip")
	}
	// Every sort order the schema allows must survive, which is why the whitelist
	// had to match the extension's exactly.
	for _, order := range domain.AllSortOrders {
		view.SortOrder = order
		if err := repo.Upsert(ctx, view); err != nil {
			t.Fatalf("Upsert %s: %v", order, err)
		}
		back, _, err := repo.Get(ctx, "v1")
		if err != nil {
			t.Fatalf("Get %s: %v", order, err)
		}
		if back.SortOrder != order {
			t.Errorf("sort order %q came back as %q", order, back.SortOrder)
		}
	}

	// An upsert replaces rather than duplicating.
	view.Name = "Renamed"
	if err := repo.Upsert(ctx, view); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	views, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(views) != 1 || views[0].Name != "Renamed" {
		t.Errorf("views = %+v, want one renamed view", views)
	}

	if _, found, err := repo.Get(ctx, "absent"); err != nil || found {
		t.Errorf("Get(absent): found=%v err=%v", found, err)
	}

	if err := repo.Delete(ctx, "v1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, found, err := repo.Get(ctx, "v1"); err != nil || found {
		t.Errorf("the deleted view must be gone: found=%v err=%v", found, err)
	}
}

// TestSavedViewRejectsAnUnknownSortOrder keeps the schema's CHECK from being the
// only thing standing between a bad value and the database.
func TestSavedViewRejectsAnUnknownSortOrder(t *testing.T) {
	db := openTestDB(t)
	repo := NewSavedViewRepository(db)

	err := repo.Upsert(context.Background(), domain.SavedView{
		ID: "bad", Name: "Bad", SortOrder: "by-vibes", ViewMode: domain.ViewGrid,
		CreatedAt: clockBase, UpdatedAt: clockBase,
	})
	if err == nil {
		t.Error("an unknown sort order must be refused")
	}
}

func TestSuggestionListForBookmark(t *testing.T) {
	db := openTestDB(t)
	repo := NewSuggestionRepository(db)
	ctx := context.Background()

	for _, row := range []struct{ id, bookmark, status string }{
		{"s1", "a", "pending"},
		{"s2", "a", "accepted"},
		{"s3", "b", "pending"},
	} {
		if _, err := db.Exec(
			`INSERT INTO tag_suggestions (id, bookmark_id, tag_name, source, confidence, status, created_at)
			 VALUES (?, ?, 'Go', 'llm', 0.5, ?, ?)`,
			row.id, row.bookmark, row.status, domain.FormatTimestamp(clockBase)); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	rows, err := repo.ListForBookmark(ctx, "a")
	if err != nil {
		t.Fatalf("ListForBookmark: %v", err)
	}
	// Resolved rows are included: they are the negative cache.
	if len(rows) != 2 {
		t.Fatalf("got %d suggestions, want 2 for bookmark a", len(rows))
	}

	if rows, err := repo.ListForBookmark(ctx, "absent"); err != nil || len(rows) != 0 {
		t.Errorf("ListForBookmark(absent) = %v, %v", rows, err)
	}
}

func TestListFindByNormalizedName(t *testing.T) {
	db := openTestDB(t)
	repo := NewListRepository(db)
	ctx := context.Background()

	if err := repo.Create(ctx, domain.List{
		ID: "l-work", Name: "Work", Normalized: domain.NormalizeListName("Work"),
		CreatedAt: clockBase, UpdatedAt: clockBase,
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	found, ok, err := repo.FindByNormalizedName(ctx, domain.NormalizeListName("  WORK  "))
	if err != nil || !ok {
		t.Fatalf("FindByNormalizedName: ok=%v err=%v", ok, err)
	}
	if found.ID != "l-work" {
		t.Errorf("found %q", found.ID)
	}

	if _, ok, err := repo.FindByNormalizedName(ctx, "absent"); err != nil || ok {
		t.Errorf("FindByNormalizedName(absent): ok=%v err=%v", ok, err)
	}
}

// TestPathIsTheOpenFile guards the accessor the app uses for diagnostics.
func TestPathIsTheOpenFile(t *testing.T) {
	db := openTestDB(t)

	if db.Path() == "" {
		t.Error("Path must report where the database is")
	}
}

// TestScanTimestampHandlesNull keeps the helper from panicking on the nullable
// columns, which is where a null would otherwise reach a parse.
func TestScanTimestampHandlesNull(t *testing.T) {
	if got := scanTimestamp(nullString("")); !got.IsZero() {
		t.Errorf("a null timestamp must read as the zero time, got %v", got)
	}

	stamp := domain.FormatTimestamp(clockBase)
	if got := scanTimestamp(nullString(stamp)); !got.Equal(clockBase) {
		t.Errorf("timestamp = %v, want %v", got, clockBase)
	}
}

// TestAuthorSummariesBreakTiesTheWayTheSourceDoes pins the tie-break comparator.
//
// Three call sites in the extension order handles with localeCompare
// (stats.ts:66, tagRelations.ts:98 and :374), so an equal count is broken naturally and not
// by bytes. This query used `ORDER BY total DESC, author_handle ASC`, and it also carried a
// LIMIT -- which cannot stay on the query once the order is applied in Go, because a limit
// after a byte sort keeps the wrong rows.
func TestAuthorSummariesBreakTiesTheWayTheSourceDoes(t *testing.T) {
	db := openTestDB(t)
	repo := NewBookmarkRepository(db)
	ctx := context.Background()

	// The same count each, so only the comparator decides. The byte order would be
	// Zebra, apple, Banana; the natural order is apple, Banana, Zebra.
	for _, handle := range []string{"Zebra", "apple", "Banana"} {
		bookmark := bookmarkFixture(handle, clockBase)
		bookmark.AuthorHandle = handle
		bookmark.AuthorName = handle
		if _, err := repo.UpsertMany(ctx, []domain.Bookmark{bookmark}); err != nil {
			t.Fatalf("seed %q: %v", handle, err)
		}
	}

	summaries, err := repo.AuthorSummaries(ctx, 2)
	if err != nil {
		t.Fatalf("AuthorSummaries: %v", err)
	}
	if len(summaries) != 2 {
		t.Fatalf("got %d summaries, want the limit of 2", len(summaries))
	}
	// The limit must cut after the ordering: a byte sort would have kept Zebra and apple.
	if summaries[0].AuthorHandle != "apple" || summaries[1].AuthorHandle != "Banana" {
		t.Errorf("got %q, %q; a byte comparison capped at 2 would keep Zebra and apple",
			summaries[0].AuthorHandle, summaries[1].AuthorHandle)
	}
}

// TestTimelineOrderPutsKnownRanksFirst covers the one comparator with four branches.
//
// The extension's `timeline` case is: both ranks known, compare them; one known, it comes
// first; neither known, fall back to savedAt descending. The SQL that reproduces it is
// `timeline_rank IS NULL, timeline_rank ASC, saved_at DESC` -- the first term is what puts a
// known rank ahead of an unknown one, and it is easy to drop by writing just
// `timeline_rank ASC`, which in SQLite puts NULLs first.
func TestTimelineOrderPutsKnownRanksFirst(t *testing.T) {
	db := openTestDB(t)
	repo := NewBookmarkRepository(db)
	ctx := context.Background()

	rank := func(n int) *int { return &n }
	later := clockBase.Add(time.Hour)

	// "known-two" and "known-one" carry ranks, so they come before the two that do not,
	// whatever their saved times are. Of the unknown pair, the later save comes first.
	fixtures := []domain.Bookmark{
		bookmarkFixture("unknown-late", later),
		bookmarkFixture("known-two", clockBase),
		bookmarkFixture("unknown-early", clockBase),
		bookmarkFixture("known-one", clockBase),
	}
	fixtures[1].TimelineRank = rank(2)
	fixtures[3].TimelineRank = rank(1)
	if _, err := repo.UpsertMany(ctx, fixtures); err != nil {
		t.Fatalf("seed: %v", err)
	}

	query := domain.LibraryQuery{}
	query.Sort = domain.SortTimeline
	rows, err := repo.Query(ctx, query)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}

	var got []string
	for _, row := range rows {
		got = append(got, row.TweetID)
	}
	want := []string{"known-one", "known-two", "unknown-late", "unknown-early"}
	if len(got) != len(want) {
		t.Fatalf("got %d rows, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("order is %v, want %v: a known rank must beat an unknown one, and the\n"+
				"unknown pair falls back to saved time descending", got, want)
			break
		}
	}
}
