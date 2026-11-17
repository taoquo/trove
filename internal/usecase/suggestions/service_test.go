package suggestions

import (
	"context"
	"errors"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/idgen"
	"github.com/taoquo/trove/internal/ports"
	"github.com/taoquo/trove/internal/storage/sqlite"
)

var clockBase = time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

func newService(t *testing.T) (*Service, *sqlite.DB) {
	t.Helper()

	db, err := sqlite.Open(filepath.Join(t.TempDir(), "suggestions.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	repos := ports.Repositories{
		Bookmarks:   sqlite.NewBookmarkRepository(db),
		Tags:        sqlite.NewTagRepository(db),
		Suggestions: sqlite.NewSuggestionRepository(db),
	}
	return New(repos, fixedClock{now: clockBase}, idgen.New("test")), db
}

func seedBookmark(t *testing.T, db *sqlite.DB, id string) {
	t.Helper()

	bookmark := domain.Bookmark{
		TweetID:      id,
		TweetURL:     "https://x.com/ada/status/" + id,
		AuthorName:   "Ada",
		AuthorHandle: "ada",
		Text:         "post " + id,
		SavedAt:      clockBase,
		LastSeenAt:   clockBase,
		UpdatedAt:    clockBase,
	}
	if _, err := sqlite.NewBookmarkRepository(db).UpsertMany(context.Background(), []domain.Bookmark{bookmark}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}
}

func seedSuggestion(t *testing.T, db *sqlite.DB, id, bookmarkID, tagName string) {
	t.Helper()

	if _, err := db.Exec(
		`INSERT INTO tag_suggestions (id, bookmark_id, tag_name, source, confidence, status, created_at)
		 VALUES (?, ?, ?, 'llm', 0.8, 'pending', ?)`,
		id, bookmarkID, tagName, domain.FormatTimestamp(clockBase)); err != nil {
		t.Fatalf("insert suggestion: %v", err)
	}
}

// TestAcceptCreatesTheTagAndAttachment pins the order: the tag and relation are
// written before the suggestion is resolved, so a failure is retryable.
func TestAcceptCreatesTheTagAndAttachment(t *testing.T) {
	service, db := newService(t)
	ctx := context.Background()
	seedBookmark(t, db, "1")
	seedSuggestion(t, db, "s1", "1", "Kubernetes")

	result, err := service.Accept(ctx, "s1")
	if err != nil {
		t.Fatalf("Accept: %v", err)
	}
	if !result.CreatedTag {
		t.Error("the tag did not exist, so it must be reported as created")
	}
	if result.Tag.Name != "Kubernetes" {
		t.Errorf("tag name = %q", result.Tag.Name)
	}
	if result.Suggestion.Status != domain.StatusAccepted {
		t.Errorf("status = %q, want accepted", result.Suggestion.Status)
	}
	if result.Suggestion.ResolvedAt == nil {
		t.Error("an accepted suggestion must carry a resolve time")
	}

	// The relation must exist and be attributed to the suggestion, not to a
	// person, so a later automatic prune knows it may revisit it.
	var source string
	if err := db.QueryRow(
		`SELECT source FROM bookmark_tags WHERE bookmark_id = '1' AND tag_id = ?`,
		result.Tag.ID).Scan(&source); err != nil {
		t.Fatalf("read relation: %v", err)
	}
	if source != domain.TagSourceSuggestion {
		t.Errorf("source = %q, want %q", source, domain.TagSourceSuggestion)
	}
}

// TestAcceptReusesAnExistingTagByCanonicalKey is why the canonical key exists.
func TestAcceptReusesAnExistingTagByCanonicalKey(t *testing.T) {
	service, db := newService(t)
	ctx := context.Background()
	seedBookmark(t, db, "1")
	seedBookmark(t, db, "2")

	// Accept "AI Agents" first, then a differently spelled suggestion.
	seedSuggestion(t, db, "s1", "1", "AI Agents")
	first, err := service.Accept(ctx, "s1")
	if err != nil {
		t.Fatalf("first Accept: %v", err)
	}

	seedSuggestion(t, db, "s2", "2", "ai-agent")
	second, err := service.Accept(ctx, "s2")
	if err != nil {
		t.Fatalf("second Accept: %v", err)
	}

	if second.CreatedTag {
		t.Error("the second spelling must reuse the existing tag")
	}
	if second.Tag.ID != first.Tag.ID {
		t.Errorf("tags differ: %q and %q", first.Tag.ID, second.Tag.ID)
	}

	var tagCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM tags`).Scan(&tagCount); err != nil {
		t.Fatalf("count tags: %v", err)
	}
	if tagCount != 1 {
		t.Errorf("%d tags exist, want 1", tagCount)
	}
}

// TestAcceptRejectsANonPendingSuggestion keeps a double accept from rewriting an
// answer.
func TestAcceptRejectsANonPendingSuggestion(t *testing.T) {
	service, db := newService(t)
	ctx := context.Background()
	seedBookmark(t, db, "1")
	seedSuggestion(t, db, "s1", "1", "Go")

	if _, err := service.Accept(ctx, "s1"); err != nil {
		t.Fatalf("first Accept: %v", err)
	}
	if _, err := service.Accept(ctx, "s1"); !errors.Is(err, ErrNotPending) {
		t.Errorf("second Accept error = %v, want ErrNotPending", err)
	}
}

func TestAcceptUnknownSuggestion(t *testing.T) {
	service, _ := newService(t)

	if _, err := service.Accept(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

// TestRejectKeepsTheRowAsANegativeCache covers why a rejection is not a delete.
func TestRejectKeepsTheRowAsANegativeCache(t *testing.T) {
	service, db := newService(t)
	ctx := context.Background()
	seedBookmark(t, db, "1")
	seedSuggestion(t, db, "s1", "1", "Spam")

	if err := service.Reject(ctx, "s1"); err != nil {
		t.Fatalf("Reject: %v", err)
	}

	// The row must still exist, so the same pair is never offered again.
	var status string
	if err := db.QueryRow(`SELECT status FROM tag_suggestions WHERE id = 's1'`).Scan(&status); err != nil {
		t.Fatalf("the rejected row must survive: %v", err)
	}
	if status != domain.StatusRejected {
		t.Errorf("status = %q, want rejected", status)
	}

	// Rejecting twice is a no-op the caller can see.
	if err := service.Reject(ctx, "s1"); !errors.Is(err, ErrNotPending) {
		t.Errorf("second Reject error = %v, want ErrNotPending", err)
	}
}

func TestPendingListsRowsWithTheirBookmarks(t *testing.T) {
	service, db := newService(t)
	ctx := context.Background()
	seedBookmark(t, db, "1")
	seedSuggestion(t, db, "s1", "1", "Go")
	// A suggestion whose bookmark is gone must still be listed, so the user can
	// clear a pending row they can see.
	seedSuggestion(t, db, "s2", "vanished", "Rust")

	rows, err := service.Pending(ctx)
	if err != nil {
		t.Fatalf("Pending: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}

	var withBookmark, withoutBookmark int
	for _, row := range rows {
		if row.Bookmark != nil {
			withBookmark++
		} else {
			withoutBookmark++
		}
	}
	if withBookmark != 1 || withoutBookmark != 1 {
		t.Errorf("bookmarks attached: %d with, %d without; want 1 and 1", withBookmark, withoutBookmark)
	}

	count, err := service.PendingCount(ctx)
	if err != nil {
		t.Fatalf("PendingCount: %v", err)
	}
	if count != 2 {
		t.Errorf("PendingCount = %d, want 2", count)
	}
}

// TestAcceptAllContinuesPastAFailure covers batch review: aborting on the first
// bad row would make the user start over.
func TestAcceptAllContinuesPastAFailure(t *testing.T) {
	service, db := newService(t)
	ctx := context.Background()
	seedBookmark(t, db, "1")
	seedBookmark(t, db, "2")
	seedSuggestion(t, db, "s1", "1", "Go")
	seedSuggestion(t, db, "s2", "2", "Rust")

	accepts, failures := service.AcceptAll(ctx, []string{"s1", "missing", "s2"})

	if len(accepts) != 2 {
		t.Errorf("accepted %d, want 2", len(accepts))
	}
	if len(failures) != 1 {
		t.Errorf("recorded %d failures, want 1", len(failures))
	}
	if _, present := failures["missing"]; !present {
		t.Error("the failing id must be reported against itself")
	}
}

func TestRejectAll(t *testing.T) {
	service, db := newService(t)
	ctx := context.Background()
	seedBookmark(t, db, "1")
	seedSuggestion(t, db, "s1", "1", "Go")
	seedSuggestion(t, db, "s2", "1", "Rust")

	rejected, failures := service.RejectAll(ctx, []string{"s1", "s2", "missing"})
	if rejected != 2 {
		t.Errorf("rejected %d, want 2", rejected)
	}
	if len(failures) != 1 {
		t.Errorf("recorded %d failures, want 1", len(failures))
	}
}

// TestRejectByTagNamesMatchesCanonically covers the delete-a-tag path: a leftover
// pending suggestion would recreate the tag the user just deleted.
func TestRejectByTagNamesMatchesCanonically(t *testing.T) {
	service, db := newService(t)
	ctx := context.Background()
	seedBookmark(t, db, "1")
	seedSuggestion(t, db, "s1", "1", "ai-agents")
	seedSuggestion(t, db, "s2", "1", "Cooking")

	rejected, err := service.RejectByTagNames(ctx, []string{"AI Agents"})
	if err != nil {
		t.Fatalf("RejectByTagNames: %v", err)
	}
	if rejected != 1 {
		t.Errorf("rejected %d, want 1: the spellings are the same tag", rejected)
	}

	var status string
	if err := db.QueryRow(`SELECT status FROM tag_suggestions WHERE id = 's1'`).Scan(&status); err != nil {
		t.Fatalf("read: %v", err)
	}
	if status != domain.StatusRejected {
		t.Errorf("status = %q, want rejected", status)
	}

	// The unrelated suggestion is untouched.
	if err := db.QueryRow(`SELECT status FROM tag_suggestions WHERE id = 's2'`).Scan(&status); err != nil {
		t.Fatalf("read: %v", err)
	}
	if status != domain.StatusPending {
		t.Errorf("unrelated status = %q, want it still pending", status)
	}
}

// TestAcceptKeepsTheSuggestionPendingWhenTheTagCannotBeMade covers the ordering
// guarantee: the resolve is last, so a failed tag leaves work to retry.
func TestAcceptKeepsTheSuggestionPendingWhenTheTagCannotBeMade(t *testing.T) {
	service, db := newService(t)
	ctx := context.Background()
	seedBookmark(t, db, "1")
	// A name that normalizes to nothing cannot become a tag.
	seedSuggestion(t, db, "s1", "1", "   ")

	if _, err := service.Accept(ctx, "s1"); err == nil {
		t.Fatal("a name that normalizes away must be refused")
	}

	var status string
	if err := db.QueryRow(`SELECT status FROM tag_suggestions WHERE id = 's1'`).Scan(&status); err != nil {
		t.Fatalf("read: %v", err)
	}
	if status != domain.StatusPending {
		t.Errorf("status = %q, want it left pending so the user can retry", status)
	}
}

// countingBookmarks counts how the pending page reads its posts.
type countingBookmarks struct {
	ports.BookmarkRepository
	gets     int
	getManys int
}

func (c *countingBookmarks) Get(ctx context.Context, id string) (domain.Bookmark, bool, error) {
	c.gets++
	return c.BookmarkRepository.Get(ctx, id)
}

func (c *countingBookmarks) GetMany(ctx context.Context, ids []string) (map[string]domain.Bookmark, error) {
	c.getManys++
	return c.BookmarkRepository.GetMany(ctx, ids)
}

// TestThePendingPageReadsItsPostsOnce covers the shape of the panel's read.
//
// Pending used to fetch each suggested post with its own Get. The statements the
// panel needs therefore grew with the backlog: a review queue of two thousand
// suggestions was two thousand round trips before the first row was drawn. The read
// is one statement per page now, and this counts them.
func TestThePendingPageReadsItsPostsOnce(t *testing.T) {
	_, db := newService(t)
	ctx := context.Background()

	const count = 30
	for i := 0; i < count; i++ {
		id := strconv.Itoa(i + 1)
		seedBookmark(t, db, id)
		seedSuggestion(t, db, "s"+id, id, "Go")
	}

	bookmarks := &countingBookmarks{BookmarkRepository: sqlite.NewBookmarkRepository(db)}
	service := New(ports.Repositories{
		Bookmarks:   bookmarks,
		Tags:        sqlite.NewTagRepository(db),
		Suggestions: sqlite.NewSuggestionRepository(db),
	}, fixedClock{now: clockBase}, idgen.New("test"))

	rows, err := service.Pending(ctx)
	if err != nil {
		t.Fatalf("Pending: %v", err)
	}
	if len(rows) != count {
		t.Fatalf("got %d rows, want %d", len(rows), count)
	}
	if bookmarks.gets != 0 {
		t.Errorf("the page still reads one post at a time: %d Get calls", bookmarks.gets)
	}
	if bookmarks.getManys > 3 {
		t.Errorf("the page read its posts in %d statements, want at most 3", bookmarks.getManys)
	}
	for _, row := range rows {
		if row.Bookmark == nil {
			t.Fatalf("a row lost its post: %+v", row.Suggestion)
		}
	}
}
