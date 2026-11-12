package sqlite

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/taoquo/x-master/apps/macos/internal/domain"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "x-master.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestMigrateFromEmptyCreatesSchema(t *testing.T) {
	db := openTestDB(t)

	version, err := db.SchemaVersion()
	if err != nil {
		t.Fatalf("SchemaVersion: %v", err)
	}
	if version != TargetSchemaVersion() {
		t.Errorf("schema version %d, want %d", version, TargetSchemaVersion())
	}

	want := []string{
		"bookmarks", "raw_payloads", "tags", "bookmark_tags", "tag_suggestions",
		"llm_analysis", "saved_views", "lists", "bookmark_lists", "sync_runs",
		"app_settings", "schema_migrations",
	}
	for _, table := range want {
		var name string
		err := db.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %q is missing: %v", table, err)
		}
	}

	// The Inbox is a system row, so a migrated database must already have it.
	count := scalarInt(t, db, "SELECT COUNT(*) FROM lists WHERE id=?", domain.InboxListID)
	if count != 1 {
		t.Errorf("Inbox rows = %d, want exactly 1", count)
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x-master.db")

	for attempt := 1; attempt <= 3; attempt++ {
		db, err := Open(path)
		if err != nil {
			t.Fatalf("Open attempt %d: %v", attempt, err)
		}
		if got := scalarInt(t, db, "SELECT COUNT(*) FROM schema_migrations"); got != TargetSchemaVersion() {
			t.Errorf("attempt %d: %d migration rows, want %d", attempt, got, TargetSchemaVersion())
		}
		if got := scalarInt(t, db, "SELECT COUNT(*) FROM lists WHERE id=?", domain.InboxListID); got != 1 {
			t.Errorf("attempt %d: Inbox rows = %d, want 1", attempt, got)
		}
		if err := db.Close(); err != nil {
			t.Fatalf("Close attempt %d: %v", attempt, err)
		}
	}
}

func TestRefusesNewerSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x-master.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	future := TargetSchemaVersion() + 1
	if _, err := db.Exec("INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)",
		future, domain.FormatTimestamp(time.Now())); err != nil {
		t.Fatalf("insert future migration: %v", err)
	}
	db.Close()

	_, err = Open(path)
	if !errors.Is(err, ErrSchemaTooNew) {
		t.Fatalf("Open on a newer schema returned %v, want ErrSchemaTooNew", err)
	}
}

func TestForeignKeysAreEnforced(t *testing.T) {
	db := openTestDB(t)

	var enabled int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&enabled); err != nil {
		t.Fatalf("read foreign_keys: %v", err)
	}
	if enabled != 1 {
		t.Fatal("foreign_keys must be on, or every cascade silently does nothing")
	}

	insertBookmark(t, db, "b1")

	// An orphan row must be refused.
	_, err := db.Exec(
		"INSERT INTO bookmark_tags (bookmark_id, tag_id, created_at) VALUES ('missing','missing','x')")
	if err == nil {
		t.Error("a bookmark_tags row pointing at nothing must be rejected")
	}

	// Deleting the bookmark must cascade to its relations, payload and ledger.
	insertTag(t, db, "t1", "Tag")
	if _, err := db.Exec(
		"INSERT INTO bookmark_tags (bookmark_id, tag_id, created_at) VALUES ('b1','t1','x')"); err != nil {
		t.Fatalf("insert relation: %v", err)
	}
	if _, err := db.Exec(
		"INSERT INTO raw_payloads (tweet_id, payload_json, captured_at) VALUES ('b1','{}','x')"); err != nil {
		t.Fatalf("insert payload: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO llm_analysis (bookmark_id, analyzed_at, prompt_version, model, content_hash, outcome, tag_count)
		 VALUES ('b1','x',1,'m','h','tagged',1)`); err != nil {
		t.Fatalf("insert analysis: %v", err)
	}

	if _, err := db.Exec("DELETE FROM bookmarks WHERE tweet_id='b1'"); err != nil {
		t.Fatalf("delete bookmark: %v", err)
	}

	for _, check := range []struct {
		table string
		where string
	}{
		{"bookmark_tags", "bookmark_id='b1'"},
		{"raw_payloads", "tweet_id='b1'"},
		{"llm_analysis", "bookmark_id='b1'"},
	} {
		if got := scalarInt(t, db, "SELECT COUNT(*) FROM "+check.table+" WHERE "+check.where); got != 0 {
			t.Errorf("%s still has %d rows after the bookmark was deleted", check.table, got)
		}
	}
}

// TestResolvedSuggestionsSurviveBookmarkDeletion pins the negative-cache rule:
// an answer the user already gave must outlive the bookmark it was given for, or
// the same tag is offered again as soon as the bookmark comes back.
func TestResolvedSuggestionsSurviveBookmarkDeletion(t *testing.T) {
	db := openTestDB(t)
	insertBookmark(t, db, "b1")

	insertSuggestion(t, db, "s-pending", "b1", "Pending", domain.StatusPending)
	insertSuggestion(t, db, "s-accepted", "b1", "Accepted", domain.StatusAccepted)
	insertSuggestion(t, db, "s-rejected", "b1", "Rejected", domain.StatusRejected)

	// What the bookmark-deletion path does: drop only the pending rows.
	if _, err := db.Exec("DELETE FROM bookmarks WHERE tweet_id='b1'"); err != nil {
		t.Fatalf("delete bookmark: %v", err)
	}
	if _, err := db.Exec(
		"DELETE FROM tag_suggestions WHERE bookmark_id='b1' AND status='pending'"); err != nil {
		t.Fatalf("delete pending suggestions: %v", err)
	}

	if got := scalarInt(t, db, "SELECT COUNT(*) FROM tag_suggestions WHERE status='pending'"); got != 0 {
		t.Errorf("pending suggestions = %d, want 0", got)
	}
	if got := scalarInt(t, db, "SELECT COUNT(*) FROM tag_suggestions WHERE status IN ('accepted','rejected')"); got != 2 {
		t.Errorf("resolved suggestions = %d, want 2 to survive as the negative cache", got)
	}
}

func TestTagHierarchyTriggers(t *testing.T) {
	db := openTestDB(t)

	insertTag(t, db, "root", "Root")
	insertTag(t, db, "child", "Child")

	tests := []struct {
		name    string
		stmt    string
		args    []any
		wantErr bool
	}{
		{"a child may take a root as parent", "UPDATE tags SET parent_id='root' WHERE id='child'", nil, false},
		{"a tag cannot be its own parent", "UPDATE tags SET parent_id='root' WHERE id='root'", nil, true},
		{"three levels are refused", "INSERT INTO tags (id,name,canonical_key,parent_id,created_at,updated_at) VALUES ('grand','Grand','grand','child','x','x')", nil, true},
		{"a missing parent is refused", "INSERT INTO tags (id,name,canonical_key,parent_id,created_at,updated_at) VALUES ('orphan','Orphan','orphan','nope','x','x')", nil, true},
		{"a tag with children cannot become a child", "UPDATE tags SET parent_id='child' WHERE id='root'", nil, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := db.Exec(tc.stmt, tc.args...)
			if tc.wantErr && err == nil {
				t.Error("expected the database to refuse this")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("expected this to be allowed, got %v", err)
			}
		})
	}
}

func TestSingleListPerBookmark(t *testing.T) {
	db := openTestDB(t)
	insertBookmark(t, db, "b1")

	if _, err := db.Exec(
		`INSERT INTO lists (id,name,normalized_name,created_at,updated_at) VALUES ('l1','One','one','x','x')`); err != nil {
		t.Fatalf("insert list: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO lists (id,name,normalized_name,created_at,updated_at) VALUES ('l2','Two','two','x','x')`); err != nil {
		t.Fatalf("insert list: %v", err)
	}

	if _, err := db.Exec(
		"INSERT INTO bookmark_lists (bookmark_id, list_id, updated_at) VALUES ('b1','l1','x')"); err != nil {
		t.Fatalf("first assignment: %v", err)
	}
	if _, err := db.Exec(
		"INSERT INTO bookmark_lists (bookmark_id, list_id, updated_at) VALUES ('b1','l2','x')"); err == nil {
		t.Error("a bookmark must not be able to belong to two lists")
	}

	// A list that still has bookmarks must not vanish silently.
	if _, err := db.Exec("DELETE FROM lists WHERE id='l1'"); err == nil {
		t.Error("deleting a list that still has bookmarks must be refused (ON DELETE RESTRICT)")
	}
}

func TestListNameUniqueness(t *testing.T) {
	db := openTestDB(t)
	if _, err := db.Exec(
		`INSERT INTO lists (id,name,normalized_name,created_at,updated_at) VALUES ('l1','Reading','reading','x','x')`); err != nil {
		t.Fatalf("insert list: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO lists (id,name,normalized_name,created_at,updated_at) VALUES ('l2','READING','reading','x','x')`); err == nil {
		t.Error("two lists with the same normalized name must be refused")
	}
}

func TestTagCanonicalKeyUniqueness(t *testing.T) {
	db := openTestDB(t)
	insertTag(t, db, "t1", "AI Agent") // canonical key "aiagent"
	_, err := db.Exec(
		`INSERT INTO tags (id,name,canonical_key,created_at,updated_at) VALUES ('t2','ai-agents','aiagent','x','x')`)
	if err == nil {
		t.Error("two tags with the same canonical key must be refused")
	}
}

func TestCheckConstraintsRejectBadEnums(t *testing.T) {
	db := openTestDB(t)
	insertBookmark(t, db, "b1")
	insertTag(t, db, "t1", "Tag")

	tests := []struct {
		name string
		stmt string
	}{
		{"bookmark_tags.source", "INSERT INTO bookmark_tags (bookmark_id,tag_id,created_at,source) VALUES ('b1','t1','x','invented')"},
		{"tag_suggestions.source", "INSERT INTO tag_suggestions (id,bookmark_id,tag_name,source,status,created_at) VALUES ('s','b1','T','invented','pending','x')"},
		{"tag_suggestions.status", "INSERT INTO tag_suggestions (id,bookmark_id,tag_name,source,status,created_at) VALUES ('s','b1','T','llm','invented','x')"},
		{"llm_analysis.outcome", "INSERT INTO llm_analysis (bookmark_id,analyzed_at,prompt_version,model,content_hash,outcome,tag_count) VALUES ('b1','x',1,'m','h','invented',0)"},
		{"saved_views.sort_order", "INSERT INTO saved_views (id,name,sort_order,view_mode,created_at,updated_at) VALUES ('v','V','author','grid','x','x')"},
		{"saved_views.view_mode", "INSERT INTO saved_views (id,name,sort_order,view_mode,created_at,updated_at) VALUES ('v','V','timeline','masonry','x','x')"},
		{"sync_runs.status", "INSERT INTO sync_runs (id,status,started_at) VALUES ('r','invented','x')"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := db.Exec(tc.stmt); err == nil {
				t.Error("the CHECK constraint must reject this value")
			}
		})
	}

	// The legacy sync error vocabulary must still be accepted: rows written by
	// the old extension carry it, and so do old backups.
	for _, kind := range []string{"auth_expired", "network_error", "x_schema_changed", "unauthorized", ""} {
		if _, err := db.Exec(
			"INSERT INTO sync_runs (id,status,started_at,error_kind) VALUES (?,?,?,?)",
			"r-"+kind, domain.SyncSuccess, "x", kind); err != nil {
			t.Errorf("error_kind %q must be accepted: %v", kind, err)
		}
	}
}

func TestSuggestionIDIsThePrimaryKey(t *testing.T) {
	db := openTestDB(t)
	insertBookmark(t, db, "b1")

	id := domain.SuggestionID("b1", "AI Agents")
	insertSuggestionWithID(t, db, id, "b1", "AI Agents", domain.StatusPending)

	// The same suggestion under a name that canonicalizes alike must collide on
	// the primary key, so re-suggesting is a no-op rather than a duplicate.
	_, err := db.Exec(
		`INSERT INTO tag_suggestions (id,bookmark_id,tag_name,source,status,created_at)
		 VALUES (?,?,?,'hashtag','pending','x')`,
		domain.SuggestionID("b1", "ai-agent"), "b1", "ai-agent")
	if err == nil {
		t.Error("the canonical suggestion id must collide, making the second insert a duplicate")
	}
}

func TestOpenRestrictsPermissions(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "data")

	// Mimic what MyGo does: create the directory world-readable.
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	path := filepath.Join(dir, "x-master.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if perm := dirInfo.Mode().Perm(); perm != 0o700 {
		t.Errorf("data directory mode = %o, want 700", perm)
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat db: %v", err)
	}
	if perm := fileInfo.Mode().Perm(); perm != 0o600 {
		t.Errorf("database mode = %o, want 600", perm)
	}
}

func TestWALModeAndCheck(t *testing.T) {
	db := openTestDB(t)

	var mode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatalf("read journal_mode: %v", err)
	}
	if mode != "wal" {
		t.Errorf("journal_mode = %q, want wal", mode)
	}

	insertBookmark(t, db, "b1")
	if err := db.Check(); err != nil {
		t.Errorf("Check on a healthy database: %v", err)
	}
}

func TestCheckDetectsForeignKeyViolations(t *testing.T) {
	db := openTestDB(t)

	// Create an orphan with enforcement temporarily off, which is the only way
	// to produce a state the check exists to catch.
	if _, err := db.Exec("PRAGMA foreign_keys = OFF"); err != nil {
		t.Fatalf("disable foreign_keys: %v", err)
	}
	if _, err := db.Exec(
		"INSERT INTO bookmark_tags (bookmark_id, tag_id, created_at) VALUES ('ghost','ghost','x')"); err != nil {
		t.Fatalf("insert orphan: %v", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("re-enable foreign_keys: %v", err)
	}

	if err := db.Check(); err == nil {
		t.Error("Check must report the foreign key violation")
	}
}

func TestCloseCheckpointsWAL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x-master.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	insertBookmark(t, db, "b1")
	if err := db.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// After a truncating checkpoint the WAL is empty, so copying the .db file
	// alone is a complete backup.
	walPath := path + "-wal"
	if info, err := os.Stat(walPath); err == nil && info.Size() != 0 {
		t.Errorf("WAL is %d bytes after Close; a copy of the .db alone would be incomplete", info.Size())
	}
}

// --- helpers ---

func scalarInt(t *testing.T, db *DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatalf("query %q: %v", query, err)
	}
	return n
}

func insertBookmark(t *testing.T, db *DB, id string) {
	t.Helper()
	ts := domain.FormatTimestamp(time.Now())
	_, err := db.Exec(
		`INSERT INTO bookmarks (tweet_id, tweet_url, saved_at, updated_at) VALUES (?,?,?,?)`,
		id, "https://x.com/i/status/"+id, ts, ts)
	if err != nil {
		t.Fatalf("insert bookmark %s: %v", id, err)
	}
}

func insertTag(t *testing.T, db *DB, id, name string) {
	t.Helper()
	ts := domain.FormatTimestamp(time.Now())
	_, err := db.Exec(
		`INSERT INTO tags (id, name, canonical_key, created_at, updated_at) VALUES (?,?,?,?,?)`,
		id, name, domain.CanonicalTagKey(name), ts, ts)
	if err != nil {
		t.Fatalf("insert tag %s: %v", id, err)
	}
}

func insertSuggestion(t *testing.T, db *DB, id, bookmarkID, name string, status domain.SuggestionStatus) {
	t.Helper()
	insertSuggestionWithID(t, db, id, bookmarkID, name, status)
}

func insertSuggestionWithID(t *testing.T, db *DB, id, bookmarkID, name string, status domain.SuggestionStatus) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO tag_suggestions (id, bookmark_id, tag_name, source, confidence, status, created_at)
		 VALUES (?,?,?,'llm',0.5,?,?)`,
		id, bookmarkID, name, status, domain.FormatTimestamp(time.Now()))
	if err != nil {
		t.Fatalf("insert suggestion %s: %v", id, err)
	}
}
