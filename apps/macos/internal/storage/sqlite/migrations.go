package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/taoquo/x-master/apps/macos/internal/domain"
)

// ErrSchemaTooNew is returned when the file was written by a newer build.
//
// Refusing is deliberate. Schema changes are not reversible, so an older build
// that "opened it anyway" would either misread the data or corrupt it. The user
// is told to update the app instead. See docs/release-runbook.md §4.2.
var ErrSchemaTooNew = errors.New("sqlite: database was created by a newer version of the app; update the app instead of downgrading")

type migration struct {
	version int
	name    string
	sql     string
	// post runs after sql, inside the same transaction, for backfills that need
	// Go: the longform flag depends on domain.IsLongformText, which counts UTF-16
	// code units and so cannot be expressed as a SQLite expression.
	post func(*sql.Tx) error
}

// migrations is append-only. Never edit a shipped migration: existing databases
// have already applied it, so the change would apply only to new installs and
// the two would diverge. Add a new one instead.
var migrations = []migration{
	{version: 1, name: "initial schema", sql: schemaV1},
	{version: 2, name: "denormalize the longform flag", sql: schemaV2, post: backfillLongform},
	{version: 3, name: "sortable copy of the X creation time", sql: schemaV3, post: backfillCreatedAt},
	{version: 4, name: "remember locally deleted bookmarks", sql: schemaV4},
}

// TargetSchemaVersion is the version this build expects.
func TargetSchemaVersion() int {
	return migrations[len(migrations)-1].version
}

// Migrate brings the database up to TargetSchemaVersion.
func (db *DB) Migrate() error {
	if err := db.ensureMigrationTable(); err != nil {
		return err
	}

	current, err := db.SchemaVersion()
	if err != nil {
		return err
	}
	if current > TargetSchemaVersion() {
		return fmt.Errorf("%w (file is at %d, this build knows %d)", ErrSchemaTooNew, current, TargetSchemaVersion())
	}

	for _, m := range migrations {
		if m.version <= current {
			continue
		}
		if err := db.applyMigration(m); err != nil {
			return err
		}
	}

	// The Inbox is a system row, so it is created as part of bringing the schema
	// up rather than lazily: an empty lists table would leave every bookmark with
	// nowhere to belong.
	return db.EnsureInbox(time.Now())
}

func (db *DB) ensureMigrationTable() error {
	const stmt = `CREATE TABLE IF NOT EXISTS schema_migrations (
		version    INTEGER PRIMARY KEY,
		applied_at TEXT NOT NULL
	)`
	if _, err := db.Exec(stmt); err != nil {
		return fmt.Errorf("sqlite: create schema_migrations: %w", err)
	}
	return nil
}

// SchemaVersion reports the highest applied migration, or 0 for an empty file.
func (db *DB) SchemaVersion() (int, error) {
	var version sql.NullInt64
	if err := db.QueryRow("SELECT MAX(version) FROM schema_migrations").Scan(&version); err != nil {
		return 0, fmt.Errorf("sqlite: read schema version: %w", err)
	}
	if !version.Valid {
		return 0, nil
	}
	return int(version.Int64), nil
}

// applyMigration runs one migration and records it in a single transaction, so a
// failure leaves neither the schema half-changed nor a phantom version row.
func (db *DB) applyMigration(m migration) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("sqlite: begin migration %d: %w", m.version, err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op once committed

	if _, err := tx.Exec(m.sql); err != nil {
		return fmt.Errorf("sqlite: migration %d (%s): %w", m.version, m.name, err)
	}
	if m.post != nil {
		if err := m.post(tx); err != nil {
			return fmt.Errorf("sqlite: migration %d (%s) backfill: %w", m.version, m.name, err)
		}
	}
	if _, err := tx.Exec(
		"INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)",
		m.version, domain.FormatTimestamp(time.Now()),
	); err != nil {
		return fmt.Errorf("sqlite: record migration %d: %w", m.version, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("sqlite: commit migration %d: %w", m.version, err)
	}
	return nil
}

// schemaV2 denormalizes the longform flag.
//
// The filter has to agree with the UI, and the UI's rule counts UTF-16 code
// units, which SQLite cannot reproduce: length() counts code points. Computing it
// in SQL would make a 280-character post containing one emoji classify
// differently from the same post in the extension.
const schemaV2 = `
ALTER TABLE bookmarks ADD COLUMN is_longform INTEGER NOT NULL DEFAULT 0;
CREATE INDEX bookmarks_longform_idx ON bookmarks(is_longform);
`

// backfillLongform fills the new column for rows that predate it.
func backfillLongform(tx *sql.Tx) error {
	rows, err := tx.Query(`SELECT tweet_id, text FROM bookmarks`)
	if err != nil {
		return err
	}

	type pending struct{ id string }
	var updates []pending
	for rows.Next() {
		var id, text string
		if err := rows.Scan(&id, &text); err != nil {
			rows.Close()
			return err
		}
		if domain.IsLongformText(text) {
			updates = append(updates, pending{id: id})
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	for _, update := range updates {
		if _, err := tx.Exec(`UPDATE bookmarks SET is_longform = 1 WHERE tweet_id = ?`, update.id); err != nil {
			return err
		}
	}
	return nil
}

// schemaV3 adds a sortable copy of X's creation time.
//
// The verbatim value cannot be sorted: X renders it as "Wed Oct 10 20:19:24
// +0000 2018", so ordering that text orders by weekday name first. The original
// column stays untouched, because it is the payload's own bytes.
const schemaV3 = `
ALTER TABLE bookmarks ADD COLUMN created_at_on_x_ts TEXT NOT NULL DEFAULT '';
CREATE INDEX bookmarks_created_ts_idx ON bookmarks(created_at_on_x_ts DESC);
`

// backfillCreatedAt fills the new column for rows that predate it.
func backfillCreatedAt(tx *sql.Tx) error {
	rows, err := tx.Query(`SELECT tweet_id, created_at_on_x FROM bookmarks`)
	if err != nil {
		return err
	}

	var updates [][2]string
	for rows.Next() {
		var id, raw string
		if err := rows.Scan(&id, &raw); err != nil {
			rows.Close()
			return err
		}
		if parsed := domain.FormatXTimestampOrEmpty(raw); parsed != "" {
			updates = append(updates, [2]string{id, parsed})
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	for _, update := range updates {
		if _, err := tx.Exec(`UPDATE bookmarks SET created_at_on_x_ts = ? WHERE tweet_id = ?`, update[1], update[0]); err != nil {
			return err
		}
	}
	return nil
}

// schemaV4 remembers bookmarks the user deleted here.
//
// The bookmarks table cannot carry the decision: a delete removes the row, and a
// sync only ever sees tweet ids from X, so the row alone leaves nothing to say
// "do not bring this back". A tombstone is that memory. It is cleared by an
// import or a restore, where the file the user supplied is authoritative.
const schemaV4 = `
CREATE TABLE deleted_bookmarks (
  tweet_id   TEXT PRIMARY KEY,
  deleted_at TEXT NOT NULL
);
`

// EnsureInbox makes sure the system list exists. It is idempotent, so it is safe
// to call at every startup: if a user's file somehow lost the row, it comes back
// rather than every bookmark losing its home.
func (db *DB) EnsureInbox(now time.Time) error {
	ts := domain.FormatTimestamp(now)
	_, err := db.Exec(
		`INSERT INTO lists (id, name, normalized_name, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO NOTHING`,
		domain.InboxListID, domain.InboxListName, domain.NormalizeListName(domain.InboxListName), ts, ts,
	)
	if err != nil {
		return fmt.Errorf("sqlite: ensure Inbox: %w", err)
	}
	return nil
}

// schemaV1 is the whole initial schema. It is a faithful implementation of
// docs/sqlite-schema.md, including the fields the original plan omitted
// (author_avatar_url, hashtags_json, source_kind, tag aliases and parent_id).
const schemaV1 = `
CREATE TABLE bookmarks (
  tweet_id          TEXT PRIMARY KEY,
  tweet_url         TEXT NOT NULL,
  author_name       TEXT NOT NULL DEFAULT '',
  author_handle     TEXT NOT NULL DEFAULT '',
  author_avatar_url TEXT,
  text              TEXT NOT NULL DEFAULT '',
  created_at_on_x   TEXT NOT NULL DEFAULT '',
  saved_at          TEXT NOT NULL,
  last_seen_at      TEXT,
  timeline_rank     INTEGER,
  hashtags_json     TEXT NOT NULL DEFAULT '[]',
  source_kind       TEXT NOT NULL DEFAULT 'x-bookmark',
  media_json        TEXT NOT NULL DEFAULT '[]',
  metrics_json      TEXT,
  updated_at        TEXT NOT NULL
);
CREATE INDEX bookmarks_author_handle_idx ON bookmarks(author_handle);
CREATE INDEX bookmarks_created_at_idx    ON bookmarks(created_at_on_x DESC);
CREATE INDEX bookmarks_saved_at_idx      ON bookmarks(saved_at DESC);
CREATE INDEX bookmarks_timeline_rank_idx ON bookmarks(timeline_rank);

CREATE TABLE raw_payloads (
  tweet_id     TEXT PRIMARY KEY REFERENCES bookmarks(tweet_id) ON DELETE CASCADE,
  payload_json TEXT NOT NULL,
  captured_at  TEXT NOT NULL
);

CREATE TABLE tags (
  id            TEXT PRIMARY KEY,
  name          TEXT NOT NULL,
  canonical_key TEXT NOT NULL UNIQUE,
  aliases_json  TEXT NOT NULL DEFAULT '[]',
  parent_id     TEXT REFERENCES tags(id) ON DELETE SET NULL,
  created_at    TEXT NOT NULL,
  updated_at    TEXT NOT NULL
);
CREATE INDEX tags_parent_idx ON tags(parent_id);

-- The taxonomy is at most two levels. These triggers are the database's own
-- guard, so an import or a future code path cannot create a third level even if
-- it forgets the application-level check.
CREATE TRIGGER tags_hierarchy_guard_insert
BEFORE INSERT ON tags
FOR EACH ROW WHEN NEW.parent_id IS NOT NULL
BEGIN
  SELECT CASE
    WHEN NEW.parent_id = NEW.id
      THEN RAISE(ABORT, 'a tag cannot be its own parent')
    WHEN (SELECT COUNT(*) FROM tags WHERE id = NEW.parent_id) = 0
      THEN RAISE(ABORT, 'tag parent does not exist')
    WHEN (SELECT parent_id FROM tags WHERE id = NEW.parent_id) IS NOT NULL
      THEN RAISE(ABORT, 'tag hierarchy would exceed two levels')
  END;
END;

CREATE TRIGGER tags_hierarchy_guard_update
BEFORE UPDATE OF parent_id ON tags
FOR EACH ROW WHEN NEW.parent_id IS NOT NULL
BEGIN
  SELECT CASE
    WHEN NEW.parent_id = NEW.id
      THEN RAISE(ABORT, 'a tag cannot be its own parent')
    WHEN (SELECT COUNT(*) FROM tags WHERE id = NEW.parent_id) = 0
      THEN RAISE(ABORT, 'tag parent does not exist')
    WHEN (SELECT parent_id FROM tags WHERE id = NEW.parent_id) IS NOT NULL
      THEN RAISE(ABORT, 'tag hierarchy would exceed two levels')
    WHEN EXISTS (SELECT 1 FROM tags WHERE parent_id = NEW.id)
      THEN RAISE(ABORT, 'a tag with children cannot become a child')
  END;
END;

CREATE TABLE bookmark_tags (
  bookmark_id TEXT NOT NULL REFERENCES bookmarks(tweet_id) ON DELETE CASCADE,
  tag_id      TEXT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
  created_at  TEXT NOT NULL,
  source      TEXT NOT NULL DEFAULT 'manual'
              CHECK (source IN ('manual','suggestion','heuristic','rule')),
  PRIMARY KEY (bookmark_id, tag_id)
);
CREATE INDEX bookmark_tags_tag_idx    ON bookmark_tags(tag_id);
CREATE INDEX bookmark_tags_source_idx ON bookmark_tags(source);

-- bookmark_id deliberately has no foreign key. Resolved rows are the user's
-- negative cache and must outlive the bookmark they were made for, so the row
-- cannot cascade away; pending rows are deleted explicitly when a bookmark goes.
-- See docs/domain-migration-map.md §3.7.
CREATE TABLE tag_suggestions (
  id          TEXT PRIMARY KEY,
  bookmark_id TEXT NOT NULL,
  tag_name    TEXT NOT NULL,
  source      TEXT NOT NULL CHECK (source IN ('hashtag','author','keyword','llm')),
  confidence  REAL NOT NULL DEFAULT 0,
  status      TEXT NOT NULL CHECK (status IN ('pending','accepted','rejected')),
  created_at  TEXT NOT NULL,
  resolved_at TEXT
);
CREATE INDEX tag_suggestions_bookmark_idx ON tag_suggestions(bookmark_id);
CREATE INDEX tag_suggestions_status_idx   ON tag_suggestions(status);
CREATE INDEX tag_suggestions_name_idx     ON tag_suggestions(tag_name);

CREATE TABLE llm_analysis (
  bookmark_id    TEXT PRIMARY KEY REFERENCES bookmarks(tweet_id) ON DELETE CASCADE,
  analyzed_at    TEXT NOT NULL,
  prompt_version INTEGER NOT NULL,
  model          TEXT NOT NULL,
  content_hash   TEXT NOT NULL,
  outcome        TEXT NOT NULL
                 CHECK (outcome IN ('tagged','no-topic','all-blocked','budget-dropped')),
  tag_count      INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX llm_analysis_model_idx ON llm_analysis(model);

CREATE TABLE saved_views (
  id                         TEXT PRIMARY KEY,
  name                       TEXT NOT NULL,
  query                      TEXT NOT NULL DEFAULT '',
  active_tag_ids_json        TEXT NOT NULL DEFAULT '[]',
  active_author_handles_json TEXT NOT NULL DEFAULT '[]',
  only_with_media            INTEGER NOT NULL DEFAULT 0,
  only_longform              INTEGER NOT NULL DEFAULT 0,
  sort_order                 TEXT NOT NULL
                             CHECK (sort_order IN ('timeline','saved-desc','saved-asc','created-desc','likes-desc')),
  view_mode                  TEXT NOT NULL CHECK (view_mode IN ('grid','list')),
  created_at                 TEXT NOT NULL,
  updated_at                 TEXT NOT NULL
);

CREATE TABLE lists (
  id              TEXT PRIMARY KEY,
  name            TEXT NOT NULL,
  normalized_name TEXT NOT NULL UNIQUE,
  created_at      TEXT NOT NULL,
  updated_at      TEXT NOT NULL
);

-- bookmark_id is the primary key, which is what enforces "a bookmark belongs to
-- at most one list". ON DELETE RESTRICT forces the caller to decide where the
-- bookmarks go before deleting a list, instead of losing their home silently.
CREATE TABLE bookmark_lists (
  bookmark_id TEXT PRIMARY KEY REFERENCES bookmarks(tweet_id) ON DELETE CASCADE,
  list_id     TEXT NOT NULL REFERENCES lists(id) ON DELETE RESTRICT,
  updated_at  TEXT NOT NULL
);
CREATE INDEX bookmark_lists_list_idx ON bookmark_lists(list_id);

-- error_kind carries no CHECK constraint on purpose: rows written by the old
-- extension use a different vocabulary, and validation must accept both.
-- See docs/domain-migration-map.md §7.4.
CREATE TABLE sync_runs (
  id               TEXT PRIMARY KEY,
  status           TEXT NOT NULL
                   CHECK (status IN ('idle','running','success','partial_success','cancelled','error')),
  started_at       TEXT NOT NULL,
  finished_at      TEXT,
  fetched_count    INTEGER NOT NULL DEFAULT 0,
  inserted_count   INTEGER NOT NULL DEFAULT 0,
  updated_count    INTEGER NOT NULL DEFAULT 0,
  failed_count     INTEGER NOT NULL DEFAULT 0,
  pages_completed  INTEGER NOT NULL DEFAULT 0,
  error_kind       TEXT,
  error_summary    TEXT,
  strategy_version INTEGER NOT NULL DEFAULT 1
);
CREATE INDEX sync_runs_started_at_idx ON sync_runs(started_at DESC);

CREATE TABLE app_settings (
  key        TEXT PRIMARY KEY,
  value_json TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
`
