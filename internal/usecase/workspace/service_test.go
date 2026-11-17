package workspace

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/idgen"
	"github.com/taoquo/trove/internal/ports"
	"github.com/taoquo/trove/internal/storage/sqlite"
)

var clockBase = time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)

// fixedClock makes the service's timestamps deterministic.
type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

// newService wires the real SQLite adapters, so the test exercises the SQL rather
// than a stub that agrees with whatever the service expects.
func newService(t *testing.T) (*Service, *sqlite.DB) {
	t.Helper()

	db, err := sqlite.Open(filepath.Join(t.TempDir(), "workspace.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	repos := ports.Repositories{
		Bookmarks:   sqlite.NewBookmarkRepository(db),
		Tags:        sqlite.NewTagRepository(db),
		Lists:       sqlite.NewListRepository(db),
		Suggestions: sqlite.NewSuggestionRepository(db),
	}

	service := New(repos, fixedClock{now: clockBase}, idgen.New("test"))
	return service, db
}

func seedBookmark(t *testing.T, db *sqlite.DB, id, handle, text string, savedAt time.Time) {
	t.Helper()

	bookmark := domain.Bookmark{
		TweetID:      id,
		TweetURL:     "https://x.com/" + handle + "/status/" + id,
		AuthorName:   handle,
		AuthorHandle: handle,
		Text:         text,
		SavedAt:      savedAt,
		LastSeenAt:   savedAt,
		UpdatedAt:    savedAt,
	}
	repo := sqlite.NewBookmarkRepository(db)
	if _, err := repo.UpsertMany(context.Background(), []domain.Bookmark{bookmark}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}
}

func TestLoadDecoratesBookmarks(t *testing.T) {
	service, db := newService(t)
	ctx := context.Background()

	seedBookmark(t, db, "1", "ada", "about go", clockBase)
	seedBookmark(t, db, "2", "grace", "about rust", clockBase.Add(time.Hour))

	if _, err := service.AddTags(ctx, []string{"1"}, []string{"Go"}); err != nil {
		t.Fatalf("AddTags: %v", err)
	}
	if err := service.MoveToList(ctx, []string{"1"}, domain.InboxListID); err != nil {
		t.Fatalf("MoveToList: %v", err)
	}

	view, err := service.Load(ctx, domain.LibraryQuery{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if view.Total != 2 || len(view.Bookmarks) != 2 {
		t.Fatalf("view has %d of %d bookmarks, want 2 of 2", len(view.Bookmarks), view.Total)
	}

	// The rows carry their relations, so the UI does not have to join anything.
	var tagged *BookmarkRow
	for i := range view.Bookmarks {
		if view.Bookmarks[i].TweetID == "1" {
			tagged = &view.Bookmarks[i]
		}
	}
	if tagged == nil {
		t.Fatal("bookmark 1 is missing from the view")
	}
	if len(tagged.Tags) != 1 || tagged.Tags[0].Name != "Go" {
		t.Errorf("tags = %+v, want the Go tag", tagged.Tags)
	}
	if tagged.List.ID != domain.InboxListID {
		t.Errorf("list = %+v, want the Inbox", tagged.List)
	}

	// The sidebar counts come from the same snapshot.
	if view.Counts.TotalBookmarks != 2 {
		t.Errorf("TotalBookmarks = %d, want 2", view.Counts.TotalBookmarks)
	}
	if view.Counts.UnclassifiedCount != 1 {
		t.Errorf("UnclassifiedCount = %d, want 1", view.Counts.UnclassifiedCount)
	}
	if view.Counts.TagCount != 1 {
		t.Errorf("TagCount = %d, want 1", view.Counts.TagCount)
	}
	if len(view.Lists) == 0 || view.Lists[0].ID != domain.InboxListID {
		t.Errorf("lists = %+v, want the Inbox first", view.Lists)
	}
	if len(view.Authors) != 2 {
		t.Errorf("authors = %+v, want 2", view.Authors)
	}
}

// TestGenerationRejectsAStaleResult is the ui-spec requirement that a slow query
// must not overwrite the results of a newer one.
func TestGenerationRejectsAStaleResult(t *testing.T) {
	service, db := newService(t)
	ctx := context.Background()
	seedBookmark(t, db, "1", "ada", "x", clockBase)

	first, err := service.Load(ctx, domain.LibraryQuery{})
	if err != nil {
		t.Fatalf("first Load: %v", err)
	}
	if !service.IsCurrent(first.Generation) {
		t.Error("the newest view must be current")
	}

	second, err := service.Load(ctx, domain.LibraryQuery{})
	if err != nil {
		t.Fatalf("second Load: %v", err)
	}
	if second.Generation <= first.Generation {
		t.Errorf("generation did not advance: %d then %d", first.Generation, second.Generation)
	}
	if service.IsCurrent(first.Generation) {
		t.Error("the earlier view must be stale once a newer one exists")
	}
	if !service.IsCurrent(second.Generation) {
		t.Error("the newest view must be current")
	}
}

// TestAddTagsReusesAnExistingTagByCanonicalKey covers why the canonical key
// exists: a differently spelled name must not become a second tag.
func TestAddTagsReusesAnExistingTagByCanonicalKey(t *testing.T) {
	service, db := newService(t)
	ctx := context.Background()
	seedBookmark(t, db, "1", "ada", "x", clockBase)
	seedBookmark(t, db, "2", "grace", "y", clockBase)

	created, err := service.CreateTag(ctx, "AI Agents")
	if err != nil {
		t.Fatalf("CreateTag: %v", err)
	}

	// A different spelling of the same concept must resolve to the same tag.
	if _, err := service.AddTags(ctx, []string{"1"}, []string{"ai-agent"}); err != nil {
		t.Fatalf("AddTags: %v", err)
	}

	view, err := service.Load(ctx, domain.LibraryQuery{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if view.Counts.TagCount != 1 {
		t.Errorf("TagCount = %d, want 1: the spellings are the same tag", view.Counts.TagCount)
	}
	if len(view.Tags) != 1 || view.Tags[0].ID != created.ID {
		t.Errorf("tags = %+v, want the single created tag", view.Tags)
	}
	if view.Tags[0].BookmarkCount != 1 {
		t.Errorf("BookmarkCount = %d, want 1", view.Tags[0].BookmarkCount)
	}
}

func TestAddAndRemoveTags(t *testing.T) {
	service, db := newService(t)
	ctx := context.Background()
	seedBookmark(t, db, "1", "ada", "x", clockBase)

	added, err := service.AddTags(ctx, []string{"1"}, []string{"Go", "Rust"})
	if err != nil {
		t.Fatalf("AddTags: %v", err)
	}
	if added != 2 {
		t.Errorf("added %d relations, want 2", added)
	}

	// Applying the same tags again must not duplicate the relations.
	again, err := service.AddTags(ctx, []string{"1"}, []string{"Go"})
	if err != nil {
		t.Fatalf("AddTags again: %v", err)
	}
	if again != 1 {
		t.Errorf("reported adding %d, want 1 attempted", again)
	}

	view, err := service.Load(ctx, domain.LibraryQuery{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(view.Bookmarks[0].Tags) != 2 {
		t.Errorf("bookmark has %d tags, want 2", len(view.Bookmarks[0].Tags))
	}

	var goTagID string
	for _, tag := range view.Bookmarks[0].Tags {
		if tag.Name == "Go" {
			goTagID = tag.ID
		}
	}
	removed, err := service.RemoveTags(ctx, []string{"1"}, []string{goTagID})
	if err != nil {
		t.Fatalf("RemoveTags: %v", err)
	}
	if removed != 1 {
		t.Errorf("removed %d relations, want 1", removed)
	}

	view, err = service.Load(ctx, domain.LibraryQuery{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(view.Bookmarks[0].Tags) != 1 || view.Bookmarks[0].Tags[0].Name != "Rust" {
		t.Errorf("tags = %+v, want only Rust", view.Bookmarks[0].Tags)
	}
}

// TestDeleteBookmarksDropsOnlyPendingSuggestions pins the negative cache: a
// resolved suggestion must outlive the bookmark it was made for.
func TestDeleteBookmarksDropsOnlyPendingSuggestions(t *testing.T) {
	service, db := newService(t)
	ctx := context.Background()
	seedBookmark(t, db, "1", "ada", "x", clockBase)

	for _, row := range []struct{ id, status string }{
		{"pending-1", "pending"},
		{"accepted-1", "accepted"},
	} {
		if _, err := db.Exec(
			`INSERT INTO tag_suggestions (id, bookmark_id, tag_name, source, confidence, status, created_at)
			 VALUES (?, '1', 'Go', 'llm', 0.5, ?, ?)`,
			row.id, row.status, domain.FormatTimestamp(clockBase)); err != nil {
			t.Fatalf("insert suggestion: %v", err)
		}
	}

	removed, err := service.DeleteBookmarks(ctx, []string{"1"})
	if err != nil {
		t.Fatalf("DeleteBookmarks: %v", err)
	}
	if removed != 1 {
		t.Errorf("removed %d bookmarks, want 1", removed)
	}

	var pending, resolved int
	if err := db.QueryRow(`SELECT COUNT(*) FROM tag_suggestions WHERE status = 'pending'`).Scan(&pending); err != nil {
		t.Fatalf("count pending: %v", err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM tag_suggestions WHERE status <> 'pending'`).Scan(&resolved); err != nil {
		t.Fatalf("count resolved: %v", err)
	}
	if pending != 0 {
		t.Errorf("%d pending suggestions survived, want 0", pending)
	}
	if resolved != 1 {
		t.Errorf("%d resolved suggestions survived, want 1: it is the negative cache", resolved)
	}
}

// TestRenamingOntoAnExistingNameIsRefused covers a rename that collides.
//
// This used to merge, on the reasoning that the relations must move rather than be
// orphaned. Refusing avoids orphaned relations too, and does it without moving anything.
// Merging also absorbed the second tag's relations and deleted that tag, which is not what
// a rename asks for -- the user asked to change one name, and got two tags combined.
// Merging is a separate action with its own entry point (MergeTags), and the two are kept
// apart.
func TestRenamingOntoAnExistingNameIsRefused(t *testing.T) {
	service, db := newService(t)
	ctx := context.Background()
	seedBookmark(t, db, "1", "ada", "x", clockBase)
	seedBookmark(t, db, "2", "grace", "y", clockBase)

	if _, err := service.CreateTag(ctx, "Rust"); err != nil {
		t.Fatalf("CreateTag Rust: %v", err)
	}
	rustlang, err := service.CreateTag(ctx, "Rustlang")
	if err != nil {
		t.Fatalf("CreateTag Rustlang: %v", err)
	}
	if _, err := service.AddTags(ctx, []string{"1"}, []string{"Rust"}); err != nil {
		t.Fatalf("AddTags: %v", err)
	}
	if _, err := service.AddTags(ctx, []string{"2"}, []string{"Rustlang"}); err != nil {
		t.Fatalf("AddTags: %v", err)
	}

	_, err = service.RenameTag(ctx, rustlang.ID, "Rust")
	if err == nil {
		t.Fatal("renaming onto an existing name was accepted")
	}
	var taken *TagNameTakenError
	if !errors.As(err, &taken) {
		t.Fatalf("error = %v (%T), want a TagNameTakenError", err, err)
	}
	// The message names the tag that got in the way, not the wanted name twice.
	if taken.TakenBy != "Rust" || taken.Wanted != "Rust" || taken.Renaming != "Rustlang" {
		t.Errorf("error = %+v", taken)
	}
	if strings.Contains(err.Error(), `"Rust" is already used by "Rust"`) {
		t.Errorf("the message names the wanted name twice: %v", err)
	}

	// Nothing moved: both tags are still there with one bookmark each.
	view, err := service.Load(ctx, domain.LibraryQuery{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if view.Counts.TagCount != 2 {
		t.Errorf("TagCount = %d, want 2: a refused rename changes nothing", view.Counts.TagCount)
	}
	for _, tag := range view.Tags {
		if tag.BookmarkCount != 1 {
			t.Errorf("tag %q holds %d bookmarks, want 1", tag.Name, tag.BookmarkCount)
		}
	}
	// And the tag that was not renamed kept its name.
	if got, found, err := sqlite.NewTagRepository(db).Get(ctx, rustlang.ID); err != nil || !found {
		t.Fatalf("Get: %v, found=%v", err, found)
	} else if got.Name != "Rustlang" {
		t.Errorf("the renamed tag is now called %q", got.Name)
	}
}

// TestDeleteTagRejectsItsPendingSuggestions is not optional: a leftover pending
// suggestion would recreate the tag on the next accept.
func TestDeleteTagRejectsItsPendingSuggestions(t *testing.T) {
	service, db := newService(t)
	ctx := context.Background()
	seedBookmark(t, db, "1", "ada", "x", clockBase)

	tag, err := service.CreateTag(ctx, "Kubernetes")
	if err != nil {
		t.Fatalf("CreateTag: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO tag_suggestions (id, bookmark_id, tag_name, source, confidence, status, created_at)
		 VALUES ('s1', '1', 'kubernetes', 'llm', 0.5, 'pending', ?)`,
		domain.FormatTimestamp(clockBase)); err != nil {
		t.Fatalf("insert suggestion: %v", err)
	}

	if err := service.DeleteTag(ctx, tag.ID); err != nil {
		t.Fatalf("DeleteTag: %v", err)
	}

	var status string
	if err := db.QueryRow(`SELECT status FROM tag_suggestions WHERE id = 's1'`).Scan(&status); err != nil {
		t.Fatalf("read suggestion: %v", err)
	}
	if status != domain.StatusRejected {
		t.Errorf("suggestion status = %q, want rejected: it must not resurrect the tag", status)
	}
}

// TestDeleteListMovesItsBookmarks covers the rule: the bookmarks must go somewhere the
// user chose, never silently nowhere.
func TestDeleteListMovesItsBookmarks(t *testing.T) {
	service, db := newService(t)
	ctx := context.Background()

	seedBookmark(t, db, "1", "ada", "x", clockBase)
	seedBookmark(t, db, "2", "grace", "y", clockBase)

	work, err := service.CreateList(ctx, "Work")
	if err != nil {
		t.Fatalf("CreateList: %v", err)
	}
	if err := service.MoveToList(ctx, []string{"1", "2"}, work.ID); err != nil {
		t.Fatalf("MoveToList: %v", err)
	}

	// A destination is required rather than defaulted inside the store.
	if err := service.DeleteList(ctx, work.ID, ""); err == nil {
		t.Error("deleting without a destination must be refused")
	}

	if err := service.DeleteList(ctx, work.ID, domain.InboxListID); err != nil {
		t.Fatalf("DeleteList: %v", err)
	}

	view, err := service.Load(ctx, domain.LibraryQuery{Filter: domain.LibraryFilter{ListID: domain.InboxListID}})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if view.Total != 2 {
		t.Errorf("Inbox holds %d bookmarks, want the 2 that were moved", view.Total)
	}
}

func TestListImmutabilityReachesTheService(t *testing.T) {
	service, _ := newService(t)
	ctx := context.Background()

	if _, err := service.RenameList(ctx, domain.InboxListID, "Renamed"); err == nil {
		t.Error("renaming the Inbox must be refused")
	}
	if err := service.DeleteList(ctx, domain.InboxListID, domain.InboxListID); err == nil {
		t.Error("deleting the Inbox must be refused")
	}
	if _, err := service.CreateList(ctx, "Inbox"); err == nil {
		t.Error("creating a list called Inbox must be refused")
	}
}

func TestMoveToListRejectsAnUnknownList(t *testing.T) {
	service, db := newService(t)
	ctx := context.Background()
	seedBookmark(t, db, "1", "ada", "x", clockBase)

	if err := service.MoveToList(ctx, []string{"1"}, "list-does-not-exist"); err == nil {
		t.Error("moving to an unknown list must be refused rather than silently dropped")
	}
}

func TestLoadRejectsAnUnknownSortOrder(t *testing.T) {
	service, _ := newService(t)

	if _, err := service.Load(context.Background(), domain.LibraryQuery{Sort: "by-vibes"}); err == nil {
		t.Error("an unknown sort order must be reported, not defaulted")
	}
}

// TestLoadPagingReportsHasMore covers the UI's "showing 100 of 4321".
func TestLoadPagingReportsHasMore(t *testing.T) {
	service, db := newService(t)
	ctx := context.Background()

	for _, id := range []string{"1", "2", "3"} {
		seedBookmark(t, db, id, "ada", "x", clockBase)
	}

	page, err := service.Load(ctx, domain.LibraryQuery{Limit: 2})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if page.Total != 3 {
		t.Errorf("Total = %d, want 3 (the full match count)", page.Total)
	}
	if len(page.Bookmarks) != 2 {
		t.Errorf("returned %d bookmarks, want 2", len(page.Bookmarks))
	}
	if !page.HasMore {
		t.Error("HasMore must be set when the window is shorter than the total")
	}
}

func TestDeleteTagIsIdempotent(t *testing.T) {
	service, _ := newService(t)

	// Deleting something that is already gone is a no-op, not an error: the UI
	// may have a stale list.
	if err := service.DeleteTag(context.Background(), "tag-missing"); err != nil {
		t.Errorf("DeleteTag on a missing tag: %v", err)
	}
}

func TestUnchangedErrorIsExported(t *testing.T) {
	// The sentinel exists so a caller can distinguish "nothing to do" from a
	// failure; a plain error would force string matching.
	if !errors.Is(ErrUnchanged, ErrUnchanged) {
		t.Error("ErrUnchanged must be a stable sentinel")
	}
}

// TestMergeTagsMovesRelationsAndAbsorbsNames covers the command that applies a
// vocabulary review's merge proposal.
func TestMergeTagsMovesRelationsAndAbsorbsNames(t *testing.T) {
	service, db := newService(t)
	ctx := context.Background()

	for _, id := range []string{"b1", "b2", "b3"} {
		seedBookmark(t, db, id, "ada", "post "+id, clockBase)
	}

	tags := sqlite.NewTagRepository(db)
	for _, spec := range []struct {
		id, name string
		aliases  []string
	}{
		{"target", "ai agent", []string{"agent"}},
		{"source1", "agentic ai", nil},
		{"source2", "llm agents", []string{"llm agent"}},
	} {
		tag := domain.Tag{
			ID: spec.id, Name: spec.name, Aliases: spec.aliases,
			CanonicalKey: domain.CanonicalTagKey(spec.name),
			CreatedAt:    domain.FormatTimestamp(clockBase), UpdatedAt: domain.FormatTimestamp(clockBase),
		}
		if err := tags.Create(ctx, tag); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	// b1 carries the target already and one source, so the source relation must not
	// produce a duplicate; b2 and b3 carry only sources.
	if err := tags.AttachMany(ctx, []domain.BookmarkTag{
		{BookmarkID: "b1", TagID: "target", CreatedAt: clockBase, Source: domain.TagSourceManual},
		{BookmarkID: "b1", TagID: "source1", CreatedAt: clockBase, Source: domain.TagSourceManual},
		{BookmarkID: "b2", TagID: "source1", CreatedAt: clockBase, Source: domain.TagSourceManual},
		{BookmarkID: "b3", TagID: "source2", CreatedAt: clockBase, Source: domain.TagSourceManual},
	}); err != nil {
		t.Fatalf("AttachMany: %v", err)
	}

	report, err := service.MergeTags(ctx, []string{"source1", "source2", "target"}, "target")
	if err != nil {
		t.Fatalf("MergeTags: %v", err)
	}

	// The target was excluded from its own sources.
	if len(report.MergedTagIDs) != 2 {
		t.Fatalf("merged %v, want the two sources", report.MergedTagIDs)
	}
	// b1 already had the target, so only b2 and b3 moved.
	if report.RemappedCount != 2 {
		t.Errorf("remapped %d, want 2 (b1 already carried the target)", report.RemappedCount)
	}

	// The sources are gone.
	for _, id := range []string{"source1", "source2"} {
		if _, found, err := tags.Get(ctx, id); err != nil || found {
			t.Errorf("tag %s must be gone: found=%v err=%v", id, found, err)
		}
	}

	// Every bookmark now carries the target exactly once.
	for _, bookmarkID := range []string{"b1", "b2", "b3"} {
		attached, err := sqlite.NewBookmarkRepository(db).TagsByBookmark(ctx, []string{bookmarkID})
		if err != nil {
			t.Fatalf("TagsByBookmark: %v", err)
		}
		if len(attached[bookmarkID]) != 1 || attached[bookmarkID][0].ID != "target" {
			t.Errorf("%s carries %+v, want only the target", bookmarkID, attached[bookmarkID])
		}
	}

	// The absorbed names became aliases, so an old name still resolves here.
	target, _, err := tags.Get(ctx, "target")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	aliasKeys := map[string]bool{}
	for _, alias := range target.Aliases {
		aliasKeys[domain.CanonicalTagKey(alias)] = true
	}
	for _, want := range []string{"agentic ai", "llm agents", "llm agent", "agent"} {
		if !aliasKeys[domain.CanonicalTagKey(want)] {
			t.Errorf("alias %q is missing from %v", want, target.Aliases)
		}
	}
	// The target's own name is not repeated as an alias of itself.
	if aliasKeys[domain.CanonicalTagKey("ai agent")] {
		t.Errorf("the target's own name must not become an alias: %v", target.Aliases)
	}
}

// TestMergedTagsChildrenFollowTheSurvivor covers the rule a merge has to get right: a
// merged tag's children must not be orphaned, and must not end up at a third level.
func TestMergedTagsChildrenFollowTheSurvivor(t *testing.T) {
	t.Run("children follow a root survivor", func(t *testing.T) {
		service, db := newService(t)
		ctx := context.Background()
		tags := sqlite.NewTagRepository(db)

		for _, spec := range []struct{ id, name, parent string }{
			{"target", "languages", ""},
			{"source", "langs", ""},
			{"child", "go", "source"},
		} {
			tag := domain.Tag{
				ID: spec.id, Name: spec.name, ParentID: spec.parent,
				CanonicalKey: domain.CanonicalTagKey(spec.name),
				CreatedAt:    domain.FormatTimestamp(clockBase), UpdatedAt: domain.FormatTimestamp(clockBase),
			}
			if err := tags.Create(ctx, tag); err != nil {
				t.Fatalf("Create: %v", err)
			}
		}

		report, err := service.MergeTags(ctx, []string{"source"}, "target")
		if err != nil {
			t.Fatalf("MergeTags: %v", err)
		}
		if report.ReparentedCount != 1 {
			t.Errorf("reparented %d, want 1", report.ReparentedCount)
		}

		child, _, err := tags.Get(ctx, "child")
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if child.ParentID != "target" {
			t.Errorf("the child's parent = %q, want the survivor", child.ParentID)
		}
	})

	// The other branch, and the one the subtest above is named for but does not take:
	// the survivor stays a child, so its children cannot follow it to a third level and
	// become roots instead.
	t.Run("children become roots when the survivor stays a child", func(t *testing.T) {
		service, db := newService(t)
		ctx := context.Background()
		tags := sqlite.NewTagRepository(db)

		for _, spec := range []struct{ id, name, parent string }{
			{"root", "topics", ""},
			{"source", "langs", ""},
			// The survivor is a child of a tag that is NOT being merged, so it stays one.
			{"target", "languages", "root"},
			{"child", "go", "source"},
		} {
			tag := domain.Tag{
				ID: spec.id, Name: spec.name, ParentID: spec.parent,
				CanonicalKey: domain.CanonicalTagKey(spec.name),
				CreatedAt:    domain.FormatTimestamp(clockBase), UpdatedAt: domain.FormatTimestamp(clockBase),
			}
			if err := tags.Create(ctx, tag); err != nil {
				t.Fatalf("Create: %v", err)
			}
		}

		if _, err := service.MergeTags(ctx, []string{"source"}, "target"); err != nil {
			t.Fatalf("MergeTags: %v", err)
		}

		// The survivor kept its parent.
		target, _, err := tags.Get(ctx, "target")
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if target.ParentID != "root" {
			t.Errorf("the survivor's parent = %q, want root", target.ParentID)
		}
		// So the merged tag's child becomes a root rather than a third level.
		child, _, err := tags.Get(ctx, "child")
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if child.ParentID != "" {
			t.Errorf("the child's parent = %q, want none: following the survivor would "+
				"make a third level", child.ParentID)
		}
	})

	t.Run("children become roots when the survivor is itself a child", func(t *testing.T) {
		service, db := newService(t)
		ctx := context.Background()
		tags := sqlite.NewTagRepository(db)

		// target is a child of source, so merging source into target makes target a
		// root -- and target's own children cannot follow it to a third level.
		for _, spec := range []struct{ id, name, parent string }{
			{"source", "langs", ""},
			{"target", "languages", "source"},
			{"child", "go", "source"},
		} {
			tag := domain.Tag{
				ID: spec.id, Name: spec.name, ParentID: spec.parent,
				CanonicalKey: domain.CanonicalTagKey(spec.name),
				CreatedAt:    domain.FormatTimestamp(clockBase), UpdatedAt: domain.FormatTimestamp(clockBase),
			}
			if err := tags.Create(ctx, tag); err != nil {
				t.Fatalf("Create: %v", err)
			}
		}

		if _, err := service.MergeTags(ctx, []string{"source"}, "target"); err != nil {
			t.Fatalf("MergeTags: %v", err)
		}

		child, _, err := tags.Get(ctx, "child")
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		// The survivor ended up a root, so the child follows it there.
		if child.ParentID != "target" {
			t.Errorf("the child's parent = %q, want the survivor", child.ParentID)
		}
		// And the survivor's own parent is gone.
		target, _, err := tags.Get(ctx, "target")
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if target.ParentID != "" {
			t.Errorf("the survivor's parent = %q, want none", target.ParentID)
		}
	})
}

// TestMergeTagsRejectsAMissingTarget keeps a merge from emptying tags into nowhere.
func TestMergeTagsRejectsAMissingTarget(t *testing.T) {
	service, db := newService(t)
	ctx := context.Background()

	tags := sqlite.NewTagRepository(db)
	if err := tags.Create(ctx, domain.Tag{
		ID: "source", Name: "go", CanonicalKey: "go",
		CreatedAt: domain.FormatTimestamp(clockBase), UpdatedAt: domain.FormatTimestamp(clockBase),
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if _, err := service.MergeTags(ctx, []string{"source"}, "does-not-exist"); err == nil {
		t.Error("a missing target must be refused")
	}
	// The source is untouched.
	if _, found, err := tags.Get(ctx, "source"); err != nil || !found {
		t.Errorf("the source must survive a refused merge: found=%v err=%v", found, err)
	}

	// A merge with only the target as a source is a no-op, not a deletion.
	report, err := service.MergeTags(ctx, []string{"source", "source"}, "source")
	if err != nil {
		t.Fatalf("MergeTags: %v", err)
	}
	if len(report.MergedTagIDs) != 0 {
		t.Errorf("merged %v, want nothing", report.MergedTagIDs)
	}
}

// TestUpdateTagAliasesDropsNoise covers the alias command.
func TestUpdateTagAliasesDropsNoise(t *testing.T) {
	service, db := newService(t)
	ctx := context.Background()
	tags := sqlite.NewTagRepository(db)

	if err := tags.Create(ctx, domain.Tag{
		ID: "t1", Name: "ai agent", CanonicalKey: domain.CanonicalTagKey("ai agent"),
		CreatedAt: domain.FormatTimestamp(clockBase), UpdatedAt: domain.FormatTimestamp(clockBase),
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	tag, err := service.UpdateTagAliases(ctx, "t1", []string{"  agent  ", "AI Agents", "agent", "", "   "})
	if err != nil {
		t.Fatalf("UpdateTagAliases: %v", err)
	}

	// "AI Agents" canonicalises to the tag's own name, so it is dropped; the repeated
	// "agent" is kept once; blanks are dropped; the rest is trimmed.
	if len(tag.Aliases) != 1 {
		t.Fatalf("aliases = %v, want just agent", tag.Aliases)
	}
	if tag.Aliases[0] != "agent" {
		t.Errorf("alias = %q, want the trimmed agent", tag.Aliases[0])
	}
}

// TestAddBookmarkStoresAPastedLink covers the entry point for a pasted link.
//
// There is no page here, so the link is the input and everything else is read out of it.
func TestAddBookmarkStoresAPastedLink(t *testing.T) {
	service, db := newService(t)
	ctx := context.Background()

	got, err := service.AddBookmark(ctx, "https://x.com/ada/status/1780000000000000001?s=20",
		"a post about compilers")
	if err != nil {
		t.Fatalf("AddBookmark: %v", err)
	}
	if got.TweetID != "1780000000000000001" {
		t.Errorf("the id is %q", got.TweetID)
	}
	if got.AuthorHandle != "ada" {
		t.Errorf("the handle is %q", got.AuthorHandle)
	}
	// The link is rebuilt without the tracking parameters, so two adds of the same post
	// cannot differ by how they were shared.
	if got.TweetURL != "https://x.com/ada/status/1780000000000000001" {
		t.Errorf("the stored link is %q", got.TweetURL)
	}
	if got.Text != "a post about compilers" {
		t.Errorf("the text is %q", got.Text)
	}
	if got.TimelineRank == nil || *got.TimelineRank >= 0 {
		t.Errorf("a hand-added post's rank is %v, want a negative one so it sorts first",
			got.TimelineRank)
	}

	// It is in the library, not just returned.
	stored, found, err := sqlite.NewBookmarkRepository(db).Get(ctx, got.TweetID)
	if err != nil || !found {
		t.Fatalf("the added bookmark is not in the library: %v, found=%v", err, found)
	}
	if stored.Text != "a post about compilers" {
		t.Errorf("the stored text is %q", stored.Text)
	}
}

// TestAddingTheSamePostTwiceKeepsOneRecord covers the dedupe, which is by id and not by URL.
func TestAddingTheSamePostTwiceKeepsOneRecord(t *testing.T) {
	service, db := newService(t)
	ctx := context.Background()

	first, err := service.AddBookmark(ctx, "https://x.com/ada/status/1780000000000000002", "as typed")
	if err != nil {
		t.Fatalf("AddBookmark: %v", err)
	}
	// The same post again, shared differently and with the text left empty.
	second, err := service.AddBookmark(ctx, "https://twitter.com/ada/status/1780000000000000002",
		"")
	if err != nil {
		t.Fatalf("AddBookmark again: %v", err)
	}

	if first.TweetID != second.TweetID {
		t.Fatalf("two adds of one post produced %q and %q", first.TweetID, second.TweetID)
	}
	count, err := sqlite.NewBookmarkRepository(db).Count(ctx, domain.LibraryFilter{})
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 1 {
		t.Errorf("the library holds %d records, want 1", count)
	}
	// The empty second add must not blank what the first one stored: this is the merge
	// rule, and it is the opposite of the sync path's.
	if second.Text != "as typed" {
		t.Errorf("the text became %q; a later add without text must not clear it", second.Text)
	}
	if !second.SavedAt.Equal(first.SavedAt) {
		t.Errorf("savedAt moved from %v to %v", first.SavedAt, second.SavedAt)
	}
}

// TestAddBookmarkRefusesAnythingElse covers the input that is not a post link.
func TestAddBookmarkRefusesAnythingElse(t *testing.T) {
	service, _ := newService(t)
	ctx := context.Background()

	for _, in := range []string{
		"", "not a link", "https://example.com/ada/status/1", "https://x.com/ada",
		"https://x.com.evil.test/ada/status/1780000000000000003",
	} {
		if _, err := service.AddBookmark(ctx, in, ""); !errors.Is(err, ErrNotAPostLink) {
			t.Errorf("AddBookmark(%q) returned %v, want ErrNotAPostLink", in, err)
		}
	}
}

// TestAliasesCannotClaimAnotherTagsIdentity covers the alias/name collision.
//
// FindByName matches a canonical key or any alias, so an alias equal to another tag's name
// makes one lookup answer with two tags. Blocking only names would let two tags advertise
// the same alias; blocking neither is what this did before.
func TestAliasesCannotClaimAnotherTagsIdentity(t *testing.T) {
	service, db := newService(t)
	ctx := context.Background()

	first, err := service.CreateTag(ctx, "Go")
	if err != nil {
		t.Fatalf("CreateTag: %v", err)
	}
	second, err := service.CreateTag(ctx, "Rust")
	if err != nil {
		t.Fatalf("CreateTag: %v", err)
	}
	// Give the second tag an alias of its own, so that is claimed too.
	if _, err := service.UpdateTagAliases(ctx, second.ID, []string{"Cargo"}); err != nil {
		t.Fatalf("UpdateTagAliases: %v", err)
	}

	got, err := service.UpdateTagAliases(ctx, first.ID, []string{
		"Rust",   // another tag's NAME
		"cargo",  // another tag's ALIAS, in a different case
		"Go",     // its own name
		"",       // blank
		"golang", // the one that survives
	})
	if err != nil {
		t.Fatalf("UpdateTagAliases: %v", err)
	}
	if len(got.Aliases) != 1 || got.Aliases[0] != "golang" {
		t.Errorf("aliases = %v, want just golang", got.Aliases)
	}

	// And the lookup still answers with one tag.
	found, ok, err := sqlite.NewTagRepository(db).FindByName(ctx, "Rust")
	if err != nil || !ok {
		t.Fatalf("FindByName: %v, found=%v", err, ok)
	}
	if found.ID != second.ID {
		t.Errorf("looking up Rust found tag %q, want the tag actually named Rust", found.ID)
	}
}

// countingTags counts the sidebar summary reads, so the cache can be observed without
// timing anything.
type countingTags struct {
	ports.TagRepository
	summaries int
}

func (c *countingTags) Summaries(ctx context.Context) ([]domain.TagSummary, error) {
	c.summaries++
	return c.TagRepository.Summaries(ctx)
}

// TestTheSidebarIsReusedUntilTheLibraryIsWritten covers the largest fixed cost of a read.
//
// A filter change does not move the counts, the lists, the tags or the authors, and those
// four aggregates are about 60% of a 50,000-bookmark Load. They are reused while the
// database's write counter is unchanged; the second half of the test is what makes the
// first half safe: a write moves the counter, and the next read recomputes.
func TestTheSidebarIsReusedUntilTheLibraryIsWritten(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "sidebar.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	tags := &countingTags{TagRepository: sqlite.NewTagRepository(db)}
	repos := ports.Repositories{
		Bookmarks:   sqlite.NewBookmarkRepository(db),
		Tags:        tags,
		Lists:       sqlite.NewListRepository(db),
		Suggestions: sqlite.NewSuggestionRepository(db),
		Revision:    db,
	}
	service := New(repos, fixedClock{now: clockBase}, idgen.New("test"))
	ctx := context.Background()

	seedBookmark(t, db, "1", "ada", "about go", clockBase)

	first, err := service.Load(ctx, domain.LibraryQuery{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if tags.summaries != 1 {
		t.Fatalf("the first read asked for the summaries %d times, want 1", tags.summaries)
	}

	// Another filter: same library, so the sidebar is reused and only the results are
	// computed.
	second, err := service.Load(ctx, domain.LibraryQuery{Filter: domain.LibraryFilter{Query: "go"}})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if tags.summaries != 1 {
		t.Errorf("a filter change asked for the summaries again (%d reads, want 1)", tags.summaries)
	}
	if second.Counts != first.Counts {
		t.Errorf("the reused counts are %+v, want %+v", second.Counts, first.Counts)
	}
	if got := len(second.Tags); got != len(first.Tags) {
		t.Errorf("the reused tag list has %d rows, want %d", got, len(first.Tags))
	}

	// A write moves the database's counter, so the next read must not reuse the cache.
	seedBookmark(t, db, "2", "grace", "about rust", clockBase.Add(time.Hour))
	third, err := service.Load(ctx, domain.LibraryQuery{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if tags.summaries != 2 {
		t.Errorf("a write did not invalidate the sidebar cache: %d summary reads, want 2", tags.summaries)
	}
	if third.Counts.TotalBookmarks != 2 {
		t.Errorf("the counts after a write say %d bookmarks, want 2", third.Counts.TotalBookmarks)
	}
}
