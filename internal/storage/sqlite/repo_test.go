package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/taoquo/trove/internal/domain"
)

var clockBase = time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)

func bookmarkFixture(id string, savedAt time.Time) domain.Bookmark {
	return domain.Bookmark{
		TweetID:      id,
		TweetURL:     "https://x.com/ada/status/" + id,
		AuthorName:   "Ada Lovelace",
		AuthorHandle: "ada",
		Text:         "about " + id,
		SavedAt:      savedAt,
		LastSeenAt:   savedAt,
		UpdatedAt:    savedAt,
	}
}

// --- migration and the denormalized columns ---

// TestMigrationDenormalizesLongformAtTheUTF16Boundary pins the fidelity trap: a
// post of 280 code points containing one emoji is 281 UTF-16 code units, so it is
// longform, while SQLite's length() would call it 280 and exclude it.
func TestMigrationDenormalizesLongformAtTheUTF16Boundary(t *testing.T) {
	db := openTestDB(t)
	repo := NewBookmarkRepository(db)

	// 279 ASCII characters plus one emoji = 280 code points, 281 UTF-16 units.
	text := ""
	for i := 0; i < 279; i++ {
		text += "a"
	}
	text += "😀"

	if got := len([]rune(text)); got != 280 {
		t.Fatalf("fixture has %d code points, want 280", got)
	}
	if !domain.IsLongformText(text) {
		t.Fatal("the fixture must be longform by the UTF-16 rule")
	}

	if _, err := repo.UpsertMany(context.Background(), []domain.Bookmark{{
		TweetID: "1", Text: text, SavedAt: clockBase, UpdatedAt: clockBase,
	}}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}

	// The stored flag is what the filter reads.
	count, err := repo.Count(context.Background(), domain.LibraryFilter{OnlyLongform: true})
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 1 {
		t.Errorf("longform count = %d, want 1: SQLite's length() would have said %d code points and excluded it",
			count, len([]rune(text)))
	}
}

// TestTheVideoFilterIsInsideTheMediaFilter pins the relation between the two media
// conditions: a video is media, an animation plays too, and a picture or an
// attachment this build does not recognise is media without being video.
func TestTheVideoFilterIsInsideTheMediaFilter(t *testing.T) {
	db := openTestDB(t)
	repo := NewBookmarkRepository(db)
	ctx := context.Background()

	withMedia := func(id string, media ...domain.Media) domain.Bookmark {
		bookmark := bookmarkFixture(id, clockBase)
		bookmark.Media = media
		return bookmark
	}
	photo := domain.Media{Type: domain.MediaPhoto, URL: "p"}
	video := domain.Media{Type: domain.MediaVideo, URL: "v"}

	if _, err := repo.UpsertMany(ctx, []domain.Bookmark{
		withMedia("plain"),
		withMedia("pictures", photo),
		withMedia("clip", photo, video),
		withMedia("gif", domain.Media{Type: domain.MediaAnimatedGif, URL: "g"}),
		withMedia("mystery", domain.Media{Type: "spaces", URL: "s"}),
	}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}

	found := func(filter domain.LibraryFilter) map[string]bool {
		t.Helper()
		got, err := repo.Query(ctx, domain.LibraryQuery{Filter: filter, Sort: domain.SortSavedDesc, Limit: 50})
		if err != nil {
			t.Fatalf("Query(%+v): %v", filter, err)
		}
		set := make(map[string]bool, len(got))
		for _, bookmark := range got {
			set[bookmark.TweetID] = true
		}
		return set
	}

	media := found(domain.LibraryFilter{OnlyWithMedia: true})
	for _, id := range []string{"pictures", "clip", "gif", "mystery"} {
		if !media[id] {
			t.Errorf("the media filter left out %q, which carries an attachment", id)
		}
	}
	if media["plain"] {
		t.Error("the media filter kept a post with no attachment")
	}

	videoOnly := found(domain.LibraryFilter{OnlyWithVideo: true})
	for _, id := range []string{"clip", "gif"} {
		if !videoOnly[id] {
			t.Errorf("the video filter left out %q, which plays", id)
		}
	}
	for _, id := range []string{"plain", "pictures", "mystery"} {
		if videoOnly[id] {
			t.Errorf("the video filter kept %q, which does not play", id)
		}
	}

	// A later sync that gains a video moves the post into the video filter: the
	// kind is refreshed on every upsert, not fixed at insert.
	if _, err := repo.UpsertMany(ctx, []domain.Bookmark{withMedia("pictures", photo, video)}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}
	if !found(domain.LibraryFilter{OnlyWithVideo: true})["pictures"] {
		t.Error("a post that gained a video stayed out of the video filter")
	}
}

// TestMigrationBackfillsExistingRows covers the path a real user takes: an
// existing file that predates the new columns, rather than a fresh install.
//
// The database is built at version 1 directly, because opening it through Open
// would record every migration as applied and the backfill would never run.
func TestMigrationBackfillsExistingRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "upgrade.db")

	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open raw: %v", err)
	}
	if _, err := raw.Exec(schemaV1); err != nil {
		t.Fatalf("apply v1 schema: %v", err)
	}
	if _, err := raw.Exec(
		`CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)`); err != nil {
		t.Fatalf("create migration table: %v", err)
	}
	if _, err := raw.Exec(`INSERT INTO schema_migrations (version, applied_at) VALUES (1, ?)`,
		domain.FormatTimestamp(clockBase)); err != nil {
		t.Fatalf("record v1: %v", err)
	}

	// A long post and an X-rendered creation time, both written before the
	// denormalized columns existed.
	longText := strings.Repeat("b", 400)
	if _, err := raw.Exec(
		`INSERT INTO bookmarks (tweet_id, tweet_url, text, created_at_on_x, saved_at, updated_at)
		 VALUES ('legacy', 'https://x.com/a/status/1', ?, 'Wed Oct 10 20:19:24 +0000 2018', ?, ?)`,
		longText, domain.FormatTimestamp(clockBase), domain.FormatTimestamp(clockBase)); err != nil {
		t.Fatalf("insert legacy row: %v", err)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("close raw: %v", err)
	}

	// Opening applies the pending migrations, including both backfills.
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	version, err := db.SchemaVersion()
	if err != nil {
		t.Fatalf("SchemaVersion: %v", err)
	}
	if version != TargetSchemaVersion() {
		t.Fatalf("schema version = %d, want %d", version, TargetSchemaVersion())
	}

	var isLongform int
	var createdAtTS string
	if err := db.QueryRow(
		`SELECT is_longform, created_at_on_x_ts FROM bookmarks WHERE tweet_id = 'legacy'`).
		Scan(&isLongform, &createdAtTS); err != nil {
		t.Fatalf("read backfilled row: %v", err)
	}

	if isLongform != 1 {
		t.Error("the backfill must mark the long row longform")
	}
	want := domain.FormatTimestamp(time.Date(2018, 10, 10, 20, 19, 24, 0, time.UTC))
	if createdAtTS != want {
		t.Errorf("created_at_on_x_ts = %q, want %q", createdAtTS, want)
	}
}

// TestCreatedDescSortsChronologically is the reason the parsed copy exists: X
// renders the creation time starting with a weekday name, so sorting that text
// would order by weekday.
func TestCreatedDescSortsChronologically(t *testing.T) {
	db := openTestDB(t)
	repo := NewBookmarkRepository(db)

	// Wednesday, then Friday, then Monday. Alphabetically: Fri, Mon, Wed — so a
	// text sort would produce exactly the wrong order.
	rows := []domain.Bookmark{
		bookmarkFixture("wed", clockBase),
		bookmarkFixture("fri", clockBase),
		bookmarkFixture("mon", clockBase),
	}
	rows[0].CreatedAtOnX = "Wed Oct 10 20:19:24 +0000 2018"
	rows[1].CreatedAtOnX = "Fri Oct 12 09:00:00 +0000 2018"
	rows[2].CreatedAtOnX = "Mon Oct 08 01:00:00 +0000 2018"

	if _, err := repo.UpsertMany(context.Background(), rows); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}

	got, err := repo.Query(context.Background(), domain.LibraryQuery{Sort: domain.SortCreatedDesc})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}

	want := []string{"fri", "wed", "mon"}
	if len(got) != len(want) {
		t.Fatalf("got %d rows, want %d", len(got), len(want))
	}
	for i, id := range want {
		if got[i].TweetID != id {
			t.Errorf("position %d = %q, want %q (newest first)", i, got[i].TweetID, id)
		}
	}
}

// TestUpsertPreservesSavedAt covers the rule that a later sync must never move
// the user's own timeline.
func TestUpsertPreservesSavedAt(t *testing.T) {
	db := openTestDB(t)
	repo := NewBookmarkRepository(db)
	ctx := context.Background()

	original := bookmarkFixture("1", clockBase)
	if result, err := repo.UpsertMany(ctx, []domain.Bookmark{original}); err != nil {
		t.Fatalf("first upsert: %v", err)
	} else if result.Inserted != 1 || result.Updated != 0 {
		t.Errorf("first upsert = %+v, want 1 inserted", result)
	}

	// A later sync reports a different saved_at and newer text.
	later := bookmarkFixture("1", clockBase.Add(48*time.Hour))
	later.Text = "edited"
	result, err := repo.UpsertMany(ctx, []domain.Bookmark{later})
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if result.Inserted != 0 || result.Updated != 1 {
		t.Errorf("second upsert = %+v, want 1 updated", result)
	}

	stored, found, err := repo.Get(ctx, "1")
	if err != nil || !found {
		t.Fatalf("Get: found=%v err=%v", found, err)
	}
	if !stored.SavedAt.Equal(clockBase) {
		t.Errorf("saved_at = %v, want the original %v", stored.SavedAt, clockBase)
	}
	if stored.Text != "edited" {
		t.Errorf("text = %q, want the newer payload", stored.Text)
	}
}

// TestMetricsAbsentIsNotZero pins the fixed defect from the audit: a missing
// metrics object must not read back as zero likes.
func TestMetricsAbsentIsNotZero(t *testing.T) {
	db := openTestDB(t)
	repo := NewBookmarkRepository(db)
	ctx := context.Background()

	zero := 0
	withZero := bookmarkFixture("zero", clockBase)
	withZero.Metrics = &domain.Metrics{Likes: &zero}

	without := bookmarkFixture("absent", clockBase)

	if _, err := repo.UpsertMany(ctx, []domain.Bookmark{withZero, without}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}

	storedZero, _, err := repo.Get(ctx, "zero")
	if err != nil {
		t.Fatalf("Get zero: %v", err)
	}
	if storedZero.Metrics == nil || storedZero.Metrics.Likes == nil || *storedZero.Metrics.Likes != 0 {
		t.Errorf("a real zero must survive as zero, got %+v", storedZero.Metrics)
	}

	storedAbsent, _, err := repo.Get(ctx, "absent")
	if err != nil {
		t.Fatalf("Get absent: %v", err)
	}
	if storedAbsent.Metrics != nil {
		t.Errorf("absent metrics must stay absent, got %+v", storedAbsent.Metrics)
	}
}

// TestLikesDescTreatsMissingAsZero treats a missing like count as zero.
func TestLikesDescTreatsMissingAsZero(t *testing.T) {
	db := openTestDB(t)
	repo := NewBookmarkRepository(db)
	ctx := context.Background()

	likes := 10
	high := bookmarkFixture("high", clockBase)
	high.Metrics = &domain.Metrics{Likes: &likes}

	rows := []domain.Bookmark{bookmarkFixture("none", clockBase), high, bookmarkFixture("also-none", clockBase)}
	if _, err := repo.UpsertMany(ctx, rows); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}

	got, err := repo.Query(ctx, domain.LibraryQuery{Sort: domain.SortLikesDesc})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d rows, want 3", len(got))
	}
	if got[0].TweetID != "high" {
		t.Errorf("first = %q, want the post with likes", got[0].TweetID)
	}
	if got[0].Metrics == nil || got[0].Metrics.Likes == nil {
		t.Error("the metrics must round-trip")
	}
}

// --- sorts ---

func TestTimelineSortPutsKnownRanksFirst(t *testing.T) {
	db := openTestDB(t)
	repo := NewBookmarkRepository(db)
	ctx := context.Background()

	rank0, rank5 := 0, 5
	first := bookmarkFixture("rank0", clockBase)
	first.TimelineRank = &rank0
	second := bookmarkFixture("rank5", clockBase)
	second.TimelineRank = &rank5
	unknown := bookmarkFixture("unknown", clockBase.Add(time.Hour))

	if _, err := repo.UpsertMany(ctx, []domain.Bookmark{unknown, second, first}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}

	got, err := repo.Query(ctx, domain.LibraryQuery{Sort: domain.SortTimeline})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}

	want := []string{"rank0", "rank5", "unknown"}
	for i, id := range want {
		if got[i].TweetID != id {
			t.Errorf("position %d = %q, want %q: known ranks order by rank, unknown ranks go last",
				i, got[i].TweetID, id)
		}
	}
}

func TestSavedSorts(t *testing.T) {
	db := openTestDB(t)
	repo := NewBookmarkRepository(db)
	ctx := context.Background()

	rows := []domain.Bookmark{
		bookmarkFixture("middle", clockBase.Add(time.Hour)),
		bookmarkFixture("newest", clockBase.Add(2*time.Hour)),
		bookmarkFixture("oldest", clockBase),
	}
	if _, err := repo.UpsertMany(ctx, rows); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}

	desc, err := repo.Query(ctx, domain.LibraryQuery{Sort: domain.SortSavedDesc})
	if err != nil {
		t.Fatalf("Query desc: %v", err)
	}
	if desc[0].TweetID != "newest" || desc[2].TweetID != "oldest" {
		t.Errorf("saved-desc order = %s,%s,%s", desc[0].TweetID, desc[1].TweetID, desc[2].TweetID)
	}

	asc, err := repo.Query(ctx, domain.LibraryQuery{Sort: domain.SortSavedAsc})
	if err != nil {
		t.Fatalf("Query asc: %v", err)
	}
	if asc[0].TweetID != "oldest" || asc[2].TweetID != "newest" {
		t.Errorf("saved-asc order = %s,%s,%s", asc[0].TweetID, asc[1].TweetID, asc[2].TweetID)
	}
}

// TestPagingIsStable pins the tie-break: without a total order, equal rows can
// swap between pages and a bookmark is shown twice while another is never shown.
func TestPagingIsStable(t *testing.T) {
	db := openTestDB(t)
	repo := NewBookmarkRepository(db)
	ctx := context.Background()

	// Every row has the same saved_at, so only the tie-break separates them.
	var rows []domain.Bookmark
	for _, id := range []string{"a", "b", "c", "d"} {
		rows = append(rows, bookmarkFixture(id, clockBase))
	}
	if _, err := repo.UpsertMany(ctx, rows); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}

	var seen []string
	for offset := 0; offset < 4; offset += 2 {
		page, err := repo.Query(ctx, domain.LibraryQuery{
			Sort: domain.SortSavedDesc, Limit: 2, Offset: offset,
		})
		if err != nil {
			t.Fatalf("Query offset %d: %v", offset, err)
		}
		for _, bookmark := range page {
			seen = append(seen, bookmark.TweetID)
		}
	}

	if len(seen) != 4 {
		t.Fatalf("paged through %d rows, want 4: %v", len(seen), seen)
	}
	unique := map[string]bool{}
	for _, id := range seen {
		if unique[id] {
			t.Errorf("%q appeared twice across pages", id)
		}
		unique[id] = true
	}
}

// --- filters ---

func TestSearchCoversFourFields(t *testing.T) {
	db := openTestDB(t)
	repo := NewBookmarkRepository(db)
	ctx := context.Background()

	row := bookmarkFixture("1", clockBase)
	row.Text = "nothing relevant"
	row.AuthorName = "Grace Hopper"
	row.AuthorHandle = "grace"
	row.TweetURL = "https://x.com/grace/status/999"

	if _, err := repo.UpsertMany(ctx, []domain.Bookmark{row}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}

	// The URL is searched too, so a pasted link finds its own bookmark.
	for _, term := range []string{"grace", "hopper", "status/999"} {
		count, err := repo.Count(ctx, domain.LibraryFilter{Query: term})
		if err != nil {
			t.Fatalf("Count(%q): %v", term, err)
		}
		if count != 1 {
			t.Errorf("searching %q found %d rows, want 1", term, count)
		}
	}

	count, err := repo.Count(ctx, domain.LibraryFilter{Query: "absent"})
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 0 {
		t.Errorf("an unrelated term found %d rows", count)
	}
}

// TestSearchCoversTagAndListNames is the other half of what "search" means: a user
// who files posts under a tag or a list expects the name they typed to find them, not
// only the words inside the posts.
func TestSearchCoversTagAndListNames(t *testing.T) {
	db := openTestDB(t)
	repo := NewBookmarkRepository(db)
	tags := NewTagRepository(db)
	lists := NewListRepository(db)
	ctx := context.Background()

	tagged := bookmarkFixture("tagged", clockBase)
	tagged.Text = "nothing relevant"
	filed := bookmarkFixture("filed", clockBase)
	filed.Text = "nothing relevant"
	plain := bookmarkFixture("plain", clockBase)
	plain.Text = "nothing relevant"
	if _, err := repo.UpsertMany(ctx, []domain.Bookmark{tagged, filed, plain}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}

	if err := tags.Create(ctx, domain.Tag{
		ID: "t1", Name: "Kubernetes", CanonicalKey: domain.CanonicalTagKey("Kubernetes"),
		CreatedAt: domain.FormatTimestamp(clockBase), UpdatedAt: domain.FormatTimestamp(clockBase),
	}); err != nil {
		t.Fatalf("Create tag: %v", err)
	}
	if err := tags.Attach(ctx, domain.BookmarkTag{
		BookmarkID: "tagged", TagID: "t1", CreatedAt: clockBase, Source: domain.TagSourceManual,
	}); err != nil {
		t.Fatalf("Attach: %v", err)
	}

	if err := lists.Create(ctx, domain.List{
		ID: "l1", Name: "Reading",
		Normalized: domain.NormalizeListName("Reading"),
		CreatedAt:  clockBase, UpdatedAt: clockBase,
	}); err != nil {
		t.Fatalf("Create list: %v", err)
	}
	if err := repo.AssignToList(ctx, []string{"filed"}, "l1", clockBase); err != nil {
		t.Fatalf("AssignToList: %v", err)
	}

	cases := []struct {
		query string
		want  string
	}{
		// Folded, like every other searched name: the stored tag is capitalised.
		{"kubernetes", "tagged"},
		{"reading", "filed"},
	}
	for _, c := range cases {
		found, err := repo.Query(ctx, domain.LibraryQuery{
			Sort: domain.SortSavedDesc, Filter: domain.LibraryFilter{Query: c.query},
		})
		if err != nil {
			t.Fatalf("Query(%q): %v", c.query, err)
		}
		if len(found) != 1 || found[0].TweetID != c.want {
			t.Errorf("searching %q found %v, want just %q", c.query, idsOf(found), c.want)
		}
	}

	// An unrelated post stays out, so this is a match and not a filter that lets
	// everything through.
	if count, err := repo.Count(ctx, domain.LibraryFilter{Query: "nomatch"}); err != nil || count != 0 {
		t.Errorf("an unrelated term found %d rows (err %v), want 0", count, err)
	}
}

// TestSearchWildcardsAreEscaped covers an input that would otherwise match
// everything.
func TestSearchWildcardsAreEscaped(t *testing.T) {
	db := openTestDB(t)
	repo := NewBookmarkRepository(db)
	ctx := context.Background()

	plain := bookmarkFixture("plain", clockBase)
	plain.Text = "no percent sign here"

	percent := bookmarkFixture("percent", clockBase)
	percent.Text = "100% done"

	if _, err := repo.UpsertMany(ctx, []domain.Bookmark{plain, percent}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}

	count, err := repo.Count(ctx, domain.LibraryFilter{Query: "100%"})
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 1 {
		t.Errorf("searching %q found %d rows, want 1: an unescaped %% matches everything", "100%", count)
	}
}

// TestSearchFoldsNonASCII covers the case SQLite's built-in lower() does not: it folds ASCII
// only, so the comparison folded the query in Go and left the column as written.
func TestSearchFoldsNonASCII(t *testing.T) {
	db := openTestDB(t)
	repo := NewBookmarkRepository(db)
	ctx := context.Background()

	lower := bookmarkFixture("lower", clockBase)
	lower.Text = "café au lait"
	upper := bookmarkFixture("upper", clockBase)
	upper.Text = "CAFÉ NOIR"
	cyrillic := bookmarkFixture("cyrillic", clockBase)
	cyrillic.Text = "ПРИВЕТ МИР"

	if _, err := repo.UpsertMany(ctx, []domain.Bookmark{lower, upper, cyrillic}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}

	for _, tc := range []struct {
		term string
		want int
	}{
		{"CAFÉ", 2},
		{"café", 2},
		{"привет", 1},
		{"ПРИВЕТ", 1},
	} {
		count, err := repo.Count(ctx, domain.LibraryFilter{Query: tc.term})
		if err != nil {
			t.Fatalf("Count(%q): %v", tc.term, err)
		}
		if count != tc.want {
			t.Errorf("searching %q found %d rows, want %d", tc.term, count, tc.want)
		}
	}
}

func TestTagFilterModes(t *testing.T) {
	db := openTestDB(t)
	repo := NewBookmarkRepository(db)
	tags := NewTagRepository(db)
	ctx := context.Background()

	if _, err := repo.UpsertMany(ctx, []domain.Bookmark{
		bookmarkFixture("both", clockBase),
		bookmarkFixture("only-go", clockBase),
		bookmarkFixture("neither", clockBase),
	}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}

	for _, tag := range []domain.Tag{
		{ID: "t-go", Name: "Go", CanonicalKey: domain.CanonicalTagKey("Go"), CreatedAt: domain.FormatTimestamp(clockBase), UpdatedAt: domain.FormatTimestamp(clockBase)},
		{ID: "t-rust", Name: "Rust", CanonicalKey: domain.CanonicalTagKey("Rust"), CreatedAt: domain.FormatTimestamp(clockBase), UpdatedAt: domain.FormatTimestamp(clockBase)},
	} {
		if err := tags.Create(ctx, tag); err != nil {
			t.Fatalf("Create tag: %v", err)
		}
	}

	if err := tags.AttachMany(ctx, []domain.BookmarkTag{
		{BookmarkID: "both", TagID: "t-go", CreatedAt: clockBase, Source: domain.TagSourceManual},
		{BookmarkID: "both", TagID: "t-rust", CreatedAt: clockBase, Source: domain.TagSourceManual},
		{BookmarkID: "only-go", TagID: "t-go", CreatedAt: clockBase, Source: domain.TagSourceManual},
	}); err != nil {
		t.Fatalf("AttachMany: %v", err)
	}

	// "all" is the default tag match mode.
	all, err := repo.Count(ctx, domain.LibraryFilter{TagIDs: []string{"t-go", "t-rust"}})
	if err != nil {
		t.Fatalf("Count all: %v", err)
	}
	if all != 1 {
		t.Errorf("MatchAll found %d rows, want 1 (only the bookmark with both tags)", all)
	}

	any, err := repo.Count(ctx, domain.LibraryFilter{
		TagIDs: []string{"t-go", "t-rust"}, TagMatchMode: domain.MatchAny,
	})
	if err != nil {
		t.Fatalf("Count any: %v", err)
	}
	if any != 2 {
		t.Errorf("MatchAny found %d rows, want 2", any)
	}
}

func TestAuthorFilterMatchesHandleAndName(t *testing.T) {
	db := openTestDB(t)
	repo := NewBookmarkRepository(db)
	ctx := context.Background()

	row := bookmarkFixture("1", clockBase)
	row.AuthorHandle = "ada"
	row.AuthorName = "Ada Lovelace"
	if _, err := repo.UpsertMany(ctx, []domain.Bookmark{row}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}

	// Author matching covers the handle and the name, so either resolves.
	for _, term := range []string{"ada", "Ada Lovelace", "ADA"} {
		count, err := repo.Count(ctx, domain.LibraryFilter{AuthorHandles: []string{term}})
		if err != nil {
			t.Fatalf("Count(%q): %v", term, err)
		}
		if count != 1 {
			t.Errorf("author filter %q found %d rows, want 1", term, count)
		}
	}
}

func TestUnclassifiedIgnoresLists(t *testing.T) {
	db := openTestDB(t)
	repo := NewBookmarkRepository(db)
	tags := NewTagRepository(db)
	ctx := context.Background()

	if _, err := repo.UpsertMany(ctx, []domain.Bookmark{
		bookmarkFixture("tagged", clockBase),
		bookmarkFixture("untagged", clockBase),
	}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}
	if err := tags.Create(ctx, domain.Tag{
		ID: "t1", Name: "Go", CanonicalKey: domain.CanonicalTagKey("Go"),
		CreatedAt: domain.FormatTimestamp(clockBase), UpdatedAt: domain.FormatTimestamp(clockBase),
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := tags.Attach(ctx, domain.BookmarkTag{
		BookmarkID: "tagged", TagID: "t1", CreatedAt: clockBase, Source: domain.TagSourceManual,
	}); err != nil {
		t.Fatalf("Attach: %v", err)
	}

	// Both bookmarks are in the Inbox, so the list must not affect the count:
	// "unclassified" means no tags, whatever list the bookmark is in.
	if err := repo.AssignToList(ctx, []string{"tagged", "untagged"}, domain.InboxListID, clockBase); err != nil {
		t.Fatalf("AssignToList: %v", err)
	}

	count, err := repo.Count(ctx, domain.LibraryFilter{OnlyUnclassified: true})
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 1 {
		t.Errorf("unclassified count = %d, want 1", count)
	}

	counts, err := repo.Counts(ctx)
	if err != nil {
		t.Fatalf("Counts: %v", err)
	}
	if counts.UnclassifiedCount != 1 {
		t.Errorf("UnclassifiedCount = %d, want 1", counts.UnclassifiedCount)
	}
	if counts.TotalBookmarks != 2 {
		t.Errorf("TotalBookmarks = %d, want 2", counts.TotalBookmarks)
	}
}

func TestTimeRangeFilter(t *testing.T) {
	db := openTestDB(t)
	repo := NewBookmarkRepository(db)
	ctx := context.Background()

	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	repo.SetClock(func() time.Time { return now })

	rows := []domain.Bookmark{
		bookmarkFixture("today", now.Add(-time.Hour)),
		bookmarkFixture("last-month", now.AddDate(0, 0, -20)),
		bookmarkFixture("ancient", now.AddDate(0, 0, -200)),
	}
	if _, err := repo.UpsertMany(ctx, rows); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}

	tests := []struct {
		window domain.SavedTimeRange
		want   int
	}{
		{domain.RangeAll, 3},
		{domain.Range7d, 1},
		{domain.Range30d, 2},
		{domain.Range90d, 2},
	}
	for _, tc := range tests {
		count, err := repo.Count(ctx, domain.LibraryFilter{TimeRange: tc.window})
		if err != nil {
			t.Fatalf("Count(%s): %v", tc.window, err)
		}
		if count != tc.want {
			t.Errorf("window %s matched %d rows, want %d", tc.window, count, tc.want)
		}
	}
}

// --- relations, lists and deletion ---

func TestAssignMissingToInbox(t *testing.T) {
	db := openTestDB(t)
	repo := NewBookmarkRepository(db)
	ctx := context.Background()

	if _, err := repo.UpsertMany(ctx, []domain.Bookmark{
		bookmarkFixture("a", clockBase), bookmarkFixture("b", clockBase),
	}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}

	// A sync inserts rows with no list; without this call they would be invisible
	// in every list view, including the Inbox.
	moved, err := repo.AssignMissingToInbox(ctx, clockBase)
	if err != nil {
		t.Fatalf("AssignMissingToInbox: %v", err)
	}
	if moved != 2 {
		t.Errorf("assigned %d bookmarks, want 2", moved)
	}

	// Idempotent: a second run has nothing to do.
	again, err := repo.AssignMissingToInbox(ctx, clockBase)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if again != 0 {
		t.Errorf("second call assigned %d, want 0", again)
	}

	count, err := repo.Count(ctx, domain.LibraryFilter{ListID: domain.InboxListID})
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 2 {
		t.Errorf("Inbox holds %d bookmarks, want 2", count)
	}
}

func TestTagsAndListsByBookmark(t *testing.T) {
	db := openTestDB(t)
	repo := NewBookmarkRepository(db)
	tags := NewTagRepository(db)
	ctx := context.Background()

	if _, err := repo.UpsertMany(ctx, []domain.Bookmark{
		bookmarkFixture("a", clockBase), bookmarkFixture("b", clockBase),
	}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}
	if err := tags.Create(ctx, domain.Tag{
		ID: "t1", Name: "Go", CanonicalKey: domain.CanonicalTagKey("Go"),
		CreatedAt: domain.FormatTimestamp(clockBase), UpdatedAt: domain.FormatTimestamp(clockBase),
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := tags.Attach(ctx, domain.BookmarkTag{
		BookmarkID: "a", TagID: "t1", CreatedAt: clockBase, Source: domain.TagSourceManual,
	}); err != nil {
		t.Fatalf("Attach: %v", err)
	}
	if err := repo.AssignToList(ctx, []string{"a"}, domain.InboxListID, clockBase); err != nil {
		t.Fatalf("AssignToList: %v", err)
	}

	byBookmark, err := repo.TagsByBookmark(ctx, []string{"a", "b"})
	if err != nil {
		t.Fatalf("TagsByBookmark: %v", err)
	}
	// A bookmark with no tags is absent from the map rather than present empty.
	if len(byBookmark["a"]) != 1 || byBookmark["a"][0].Name != "Go" {
		t.Errorf("tags for a = %+v", byBookmark["a"])
	}
	if _, present := byBookmark["b"]; present {
		t.Error("a bookmark with no tags must be absent from the map")
	}

	lists, err := repo.ListByBookmark(ctx, []string{"a", "b"})
	if err != nil {
		t.Fatalf("ListByBookmark: %v", err)
	}
	if lists["a"].ID != domain.InboxListID {
		t.Errorf("list for a = %+v", lists["a"])
	}
	if _, present := lists["b"]; present {
		t.Error("a bookmark with no list must be absent from the map")
	}
}

// TestDeleteAllKeepsResolvedSuggestions pins the deliberate absence of a foreign
// key: resolved suggestions are the negative cache and must outlive the bookmark.
func TestDeleteAllKeepsResolvedSuggestions(t *testing.T) {
	db := openTestDB(t)
	repo := NewBookmarkRepository(db)
	ctx := context.Background()

	if _, err := repo.UpsertMany(ctx, []domain.Bookmark{bookmarkFixture("a", clockBase)}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}
	for _, row := range []struct {
		id     string
		status string
	}{
		{"pending-1", "pending"},
		{"accepted-1", "accepted"},
		{"rejected-1", "rejected"},
	} {
		if _, err := db.Exec(
			`INSERT INTO tag_suggestions (id, bookmark_id, tag_name, source, confidence, status, created_at)
			 VALUES (?, 'a', 'Go', 'llm', 0.5, ?, ?)`,
			row.id, row.status, domain.FormatTimestamp(clockBase)); err != nil {
			t.Fatalf("insert suggestion: %v", err)
		}
	}

	removed, err := repo.DeleteAll(ctx)
	if err != nil {
		t.Fatalf("DeleteAll: %v", err)
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
	if resolved != 2 {
		t.Errorf("%d resolved suggestions survived, want 2: they are the negative cache", resolved)
	}
}

// --- tag repository invariants ---

func TestAttachDoesNotDowngradeAManualTag(t *testing.T) {
	db := openTestDB(t)
	tags := NewTagRepository(db)
	repo := NewBookmarkRepository(db)
	ctx := context.Background()

	// bookmark_tags has a real foreign key to bookmarks, so the bookmark has to
	// exist before anything can be attached to it.
	if _, err := repo.UpsertMany(ctx, []domain.Bookmark{bookmarkFixture("a", clockBase)}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}

	if err := tags.Create(ctx, domain.Tag{
		ID: "t1", Name: "Go", CanonicalKey: domain.CanonicalTagKey("Go"),
		CreatedAt: domain.FormatTimestamp(clockBase), UpdatedAt: domain.FormatTimestamp(clockBase),
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := tags.Attach(ctx, domain.BookmarkTag{
		BookmarkID: "a", TagID: "t1", CreatedAt: clockBase, Source: domain.TagSourceManual,
	}); err != nil {
		t.Fatalf("Attach: %v", err)
	}
	// A later automatic pass must not relabel it: the source column is what a
	// prune filters on, so a downgrade would let the prune delete a manual tag.
	if err := tags.Attach(ctx, domain.BookmarkTag{
		BookmarkID: "a", TagID: "t1", CreatedAt: clockBase, Source: domain.TagSourceHeuristic,
	}); err != nil {
		t.Fatalf("second Attach: %v", err)
	}

	var source string
	if err := db.QueryRow(
		`SELECT source FROM bookmark_tags WHERE bookmark_id = 'a' AND tag_id = 't1'`).Scan(&source); err != nil {
		t.Fatalf("read source: %v", err)
	}
	if source != domain.TagSourceManual {
		t.Errorf("source = %q, want it left as manual", source)
	}
}

func TestDetachAutomaticSparesManualTags(t *testing.T) {
	db := openTestDB(t)
	tags := NewTagRepository(db)
	repo := NewBookmarkRepository(db)
	ctx := context.Background()

	if _, err := repo.UpsertMany(ctx, []domain.Bookmark{bookmarkFixture("a", clockBase)}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}

	for _, tag := range []domain.Tag{
		{ID: "manual", Name: "Manual", CanonicalKey: "manual"},
		{ID: "auto", Name: "Auto", CanonicalKey: "auto"},
	} {
		tag.CreatedAt = domain.FormatTimestamp(clockBase)
		tag.UpdatedAt = domain.FormatTimestamp(clockBase)
		if err := tags.Create(ctx, tag); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	if err := tags.AttachMany(ctx, []domain.BookmarkTag{
		{BookmarkID: "a", TagID: "manual", CreatedAt: clockBase, Source: domain.TagSourceManual},
		{BookmarkID: "a", TagID: "auto", CreatedAt: clockBase, Source: domain.TagSourceHeuristic},
	}); err != nil {
		t.Fatalf("AttachMany: %v", err)
	}

	removed, err := tags.DetachAutomatic(ctx, nil)
	if err != nil {
		t.Fatalf("DetachAutomatic: %v", err)
	}
	if removed != 1 {
		t.Errorf("removed %d relations, want 1", removed)
	}

	var remaining string
	if err := db.QueryRow(`SELECT tag_id FROM bookmark_tags WHERE bookmark_id = 'a'`).Scan(&remaining); err != nil {
		t.Fatalf("read remaining: %v", err)
	}
	if remaining != "manual" {
		t.Errorf("remaining tag = %q, want the manual one", remaining)
	}
}

// TestFindByNameResolvesAliases covers why a merge records an alias: without it,
// a later suggestion of the absorbed name would recreate the merged tag.
func TestFindByNameResolvesAliases(t *testing.T) {
	db := openTestDB(t)
	tags := NewTagRepository(db)
	ctx := context.Background()

	if err := tags.Create(ctx, domain.Tag{
		ID: "t1", Name: "AI Agents", CanonicalKey: domain.CanonicalTagKey("AI Agents"),
		Aliases:   []string{"ai-agent", "AI Agent"},
		CreatedAt: domain.FormatTimestamp(clockBase), UpdatedAt: domain.FormatTimestamp(clockBase),
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// By canonical key.
	found, ok, err := tags.FindByName(ctx, "ai-agents")
	if err != nil || !ok {
		t.Fatalf("FindByName canonical: ok=%v err=%v", ok, err)
	}
	if found.ID != "t1" {
		t.Errorf("canonical lookup found %q", found.ID)
	}

	// By alias.
	found, ok, err = tags.FindByName(ctx, "AI Agent")
	if err != nil || !ok {
		t.Fatalf("FindByName alias: ok=%v err=%v", ok, err)
	}
	if found.ID != "t1" {
		t.Errorf("alias lookup found %q, want the tag that absorbed the name", found.ID)
	}

	if _, ok, err := tags.FindByName(ctx, "   "); err != nil || ok {
		t.Errorf("a blank name must resolve to nothing: ok=%v err=%v", ok, err)
	}
}

// --- list repository invariants ---

func TestListSummariesAndInboxFirst(t *testing.T) {
	db := openTestDB(t)
	repo := NewBookmarkRepository(db)
	lists := NewListRepository(db)
	ctx := context.Background()

	if err := lists.Create(ctx, domain.List{
		ID: "l-work", Name: "Work", Normalized: domain.NormalizeListName("Work"),
		CreatedAt: clockBase, UpdatedAt: clockBase,
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := repo.UpsertMany(ctx, []domain.Bookmark{
		bookmarkFixture("a", clockBase), bookmarkFixture("b", clockBase),
	}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}
	if err := repo.AssignToList(ctx, []string{"a"}, "l-work", clockBase); err != nil {
		t.Fatalf("AssignToList: %v", err)
	}

	summaries, err := lists.Summaries(ctx)
	if err != nil {
		t.Fatalf("Summaries: %v", err)
	}
	if len(summaries) != 2 {
		t.Fatalf("got %d lists, want 2 (Inbox and Work)", len(summaries))
	}
	// The Inbox is pinned first: it is where every new bookmark lands.
	if summaries[0].ID != domain.InboxListID {
		t.Errorf("first list = %q, want the Inbox", summaries[0].ID)
	}
	for _, summary := range summaries {
		if summary.ID == "l-work" && summary.BookmarkCount != 1 {
			t.Errorf("Work holds %d bookmarks, want 1", summary.BookmarkCount)
		}
	}
}

func TestListImmutabilityIsEnforcedAtTheStore(t *testing.T) {
	db := openTestDB(t)
	lists := NewListRepository(db)
	ctx := context.Background()

	if err := lists.Update(ctx, domain.List{
		ID: domain.InboxListID, Name: "Renamed", Normalized: "renamed", UpdatedAt: clockBase,
	}); err == nil {
		t.Error("renaming the Inbox must be refused")
	}

	if err := lists.Delete(ctx, domain.InboxListID); err == nil {
		t.Error("deleting the Inbox must be refused")
	}
}

// TestDeletingANonEmptyListIsRefused covers the rule that the caller must decide
// where the bookmarks go, rather than a cascade silently losing their home.
func TestDeletingANonEmptyListIsRefused(t *testing.T) {
	db := openTestDB(t)
	repo := NewBookmarkRepository(db)
	lists := NewListRepository(db)
	ctx := context.Background()

	if err := lists.Create(ctx, domain.List{
		ID: "l-work", Name: "Work", Normalized: "work", CreatedAt: clockBase, UpdatedAt: clockBase,
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := repo.UpsertMany(ctx, []domain.Bookmark{bookmarkFixture("a", clockBase)}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}
	if err := repo.AssignToList(ctx, []string{"a"}, "l-work", clockBase); err != nil {
		t.Fatalf("AssignToList: %v", err)
	}

	if err := lists.Delete(ctx, "l-work"); err == nil {
		t.Fatal("deleting a list that still holds bookmarks must be refused")
	}

	// After moving them out, the delete succeeds.
	if err := repo.AssignToList(ctx, []string{"a"}, domain.InboxListID, clockBase); err != nil {
		t.Fatalf("AssignToList back: %v", err)
	}
	if err := lists.Delete(ctx, "l-work"); err != nil {
		t.Errorf("Delete after moving: %v", err)
	}
}

func TestAuthorSummaries(t *testing.T) {
	db := openTestDB(t)
	repo := NewBookmarkRepository(db)
	ctx := context.Background()

	rows := []domain.Bookmark{
		bookmarkFixture("1", clockBase),
		bookmarkFixture("2", clockBase),
		bookmarkFixture("3", clockBase),
	}
	rows[0].AuthorHandle, rows[0].AuthorName = "ada", "Ada"
	rows[1].AuthorHandle, rows[1].AuthorName = "ada", "Ada"
	rows[2].AuthorHandle, rows[2].AuthorName = "grace", "Grace"

	if _, err := repo.UpsertMany(ctx, rows); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}

	summaries, err := repo.AuthorSummaries(ctx, 10)
	if err != nil {
		t.Fatalf("AuthorSummaries: %v", err)
	}
	if len(summaries) != 2 {
		t.Fatalf("got %d authors, want 2", len(summaries))
	}
	if summaries[0].AuthorHandle != "ada" || summaries[0].BookmarkCount != 2 {
		t.Errorf("first author = %+v, want ada with 2", summaries[0])
	}
}

// TestListOrdersTagsNaturally covers the collation the tag list uses.
//
// The tag list uses natural numeric ordering and base sensitivity. SQL's ORDER BY is a
// byte comparison and gets three things wrong: "Tag 10" before "Tag 2", "Zebra" before
// "apple", and any non-ASCII name ranked by its bytes. The repository sorted by SQL until
// this was added, so every caller saw that order.
func TestListOrdersTagsNaturally(t *testing.T) {
	db := openTestDB(t)
	tags := NewTagRepository(db)
	ctx := context.Background()

	// Inserted in an order that differs from both the byte order and the collated one.
	for _, name := range []string{"Tag 10", "apple", "Zebra", "Tag 2", "banana"} {
		if err := tags.Create(ctx, domain.Tag{
			ID: "tag-" + name, Name: name, CanonicalKey: domain.CanonicalTagKey(name),
			CreatedAt: domain.FormatTimestamp(clockBase), UpdatedAt: domain.FormatTimestamp(clockBase),
		}); err != nil {
			t.Fatalf("Create(%q): %v", name, err)
		}
	}

	got, err := tags.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	names := make([]string, 0, len(got))
	for _, tag := range got {
		names = append(names, tag.Name)
	}

	// Numeric: 2 before 10. Base sensitivity: "apple" before "banana", and case does not
	// decide, so "Zebra" lands next to the other z-word rather than first.
	want := []string{"apple", "banana", "Tag 2", "Tag 10", "Zebra"}
	if len(names) != len(want) {
		t.Fatalf("List returned %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("List returned %v, want %v", names, want)
		}
	}
}
