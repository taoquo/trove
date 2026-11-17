package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/taoquo/trove/internal/domain"
)

// ErrSchemaTooNew is returned when the file was written by a newer build.
//
// Refusing is deliberate. Schema changes are not reversible, so an older build
// that "opened it anyway" would either misread the data or corrupt it. The user
// is told to update the app instead.
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
	{version: 5, name: "read and archive state", sql: schemaV5},
	{version: 6, name: "drop the unread flag", sql: schemaV6},
	{version: 7, name: "sync heartbeat", sql: schemaV7},
	{version: 8, name: "remember failed analysis batches", sql: schemaV8},
	{version: 9, name: "saved views remember the archive", sql: schemaV9},
	{version: 10, name: "an index for every sort", sql: schemaV10},
	{version: 11, name: "indexed, folded author columns", sql: schemaV11},
	{version: 12, name: "folded suggestion names", sql: schemaV12},
	{version: 13, name: "the kind of media a post carries", sql: schemaV13, post: backfillMediaKind},
	{version: 14, name: "saved views remember the video filter", sql: schemaV14},
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

	// A migration is not reversible. Before touching a library that already has
	// rows, take a consistent copy beside it: VACUUM INTO writes a checkpointed
	// database without the WAL, so the copy opens on its own. The copy is what a
	// reader falls back to when a later migration has to be undone by hand.
	if current > 0 && current < TargetSchemaVersion() {
		if err := db.snapshotBeforeMigration(current); err != nil {
			return err
		}
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
// snapshotBeforeMigration copies the database beside itself before an upgrade
// touches it.
//
// The file is named after the target version rather than the current one, so a
// second attempt at the same upgrade replaces its own leftover instead of
// accumulating copies. Two upgrades are kept; anything older is pruned.
func (db *DB) snapshotBeforeMigration(current int) error {
	target := fmt.Sprintf("%s.before-v%d", db.path, TargetSchemaVersion())
	// VACUUM INTO refuses an existing file, so the target is replaced first. The
	// escaping of the path and the permissions on the copy live in SnapshotInto,
	// which the daily backups also use.
	if err := os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("sqlite: replace the pre-upgrade snapshot: %w", err)
	}
	if err := db.SnapshotInto(context.Background(), target); err != nil {
		return fmt.Errorf("sqlite: snapshot before migrating from v%d: %w", current, err)
	}

	return db.pruneSnapshots(2)
}

// pruneSnapshots keeps the newest keep pre-upgrade copies of this database and
// removes the rest. A failure to remove one is not worth failing the upgrade
// for: the copy it would have removed is still a valid backup.
func (db *DB) pruneSnapshots(keep int) error {
	matches, err := filepath.Glob(db.path + ".before-v*")
	if err != nil {
		return nil
	}
	type snapshot struct {
		path    string
		modTime time.Time
	}
	snapshots := make([]snapshot, 0, len(matches))
	for _, path := range matches {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		snapshots = append(snapshots, snapshot{path: path, modTime: info.ModTime()})
	}
	if len(snapshots) <= keep {
		return nil
	}
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].modTime.After(snapshots[j].modTime)
	})
	for _, old := range snapshots[keep:] {
		_ = os.Remove(old.path)
	}
	return nil
}

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
// in SQL would make a 280-character post containing one emoji classify by code
// points rather than UTF-16 units and disagree with the UI.
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

// schemaV5 added the two reader-owned states. It is kept as it was applied -- v6 takes
// read_at back out.
//
// Both are NULL until something local sets them, and both survive a sync: they are not
// in the upsert's update list, so the newest payload cannot clear them. There was no
// backfill, and there could not be one: a bookmark saved before these columns existed was
// never opened, so NULL -- unread -- is the value every existing row got.
const schemaV5 = `
ALTER TABLE bookmarks ADD COLUMN read_at TEXT;
ALTER TABLE bookmarks ADD COLUMN archived_at TEXT;
CREATE INDEX bookmarks_unread_idx ON bookmarks(read_at) WHERE read_at IS NULL;
CREATE INDEX bookmarks_archived_idx ON bookmarks(archived_at);
`

// schemaV6 removes read_at, which did not earn its place.
//
// It was set automatically the first time a detail was opened and could not be set back,
// so it was a one-way switch with nothing to show for it: the reader saw a dot until they
// opened a bookmark and no mark at all afterwards, and a library that had never been read
// through showed the dot on every card. Archiving changes what the list holds and stays;
// unread only decorated it. Dropping the column rather than ignoring it keeps a schema a
// future build can trust.
//
// The index goes first: SQLite refuses to drop a column an index mentions.
const schemaV6 = `
DROP INDEX IF EXISTS bookmarks_unread_idx;
ALTER TABLE bookmarks DROP COLUMN read_at;
`

// schemaV7 adds the sync heartbeat.
//
// The window and `trove sync` share a database and each keeps its own in-process
// "a sync is running" flag, so two processes could write the library at once and
// the startup self-heal could close a run that was still alive in the other. A
// running row whose heartbeat is fresh is the cross-process answer: the second
// process is refused, and only a heartbeat that stopped lets the next one take
// over.
const schemaV7 = `
ALTER TABLE sync_runs ADD COLUMN heartbeat_at TEXT;
`

// schemaV8 lets the ledger record a failed batch.
//
// The ledger only held answers, so a batch the provider refused left no row and the
// next run sent the same bookmarks again -- paying for the same failure, every time.
// A row with outcome 'error' and the failure's kind is the memory that stops that,
// and the kind is what tells a wrong key from a rate limit when it is inspected.
//
// The outcome column carries a CHECK constraint, and SQLite cannot alter one, so the
// table is rebuilt: create the new shape, copy every row, drop, rename. Rows that
// existed before this migration were all answers, so error_kind defaults to ”.
// schemaV9 lets a saved view remember the archive.
//
// A view is a named answer to "what am I looking at", and since the archive became a
// mode of the list the answer can be "the archive". Saved without this column, a view
// taken there reopened on the library instead -- the filters came back but the rows did
// not. Existing views get 0, which is the state they were saved in.
const schemaV9 = `
ALTER TABLE saved_views ADD COLUMN only_archived INTEGER NOT NULL DEFAULT 0;
`

const schemaV8 = `
CREATE TABLE llm_analysis_v8 (
  bookmark_id    TEXT PRIMARY KEY REFERENCES bookmarks(tweet_id) ON DELETE CASCADE,
  analyzed_at    TEXT NOT NULL,
  prompt_version INTEGER NOT NULL,
  model          TEXT NOT NULL,
  content_hash   TEXT NOT NULL,
  outcome        TEXT NOT NULL
                 CHECK (outcome IN ('tagged','no-topic','all-blocked','budget-dropped','error')),
  error_kind     TEXT NOT NULL DEFAULT '',
  tag_count      INTEGER NOT NULL DEFAULT 0
);
INSERT INTO llm_analysis_v8
  (bookmark_id, analyzed_at, prompt_version, model, content_hash, outcome, tag_count)
  SELECT bookmark_id, analyzed_at, prompt_version, model, content_hash, outcome, tag_count
  FROM llm_analysis;
DROP TABLE llm_analysis;
ALTER TABLE llm_analysis_v8 RENAME TO llm_analysis;
CREATE INDEX llm_analysis_model_idx ON llm_analysis(model);
`

// schemaV10 gives each of the five sorts an index that already carries its order.
//
// The list asks for 80 rows in one order. With only single-column indexes the
// planner either read the whole table or used one column's index and then sorted
// the matched rows into a temporary b-tree, which on a 10k library is a full sort
// per reload. Each sort gets one partial index per archive state: the view always
// filters on archived_at, and a partial index lets the planner skip the other
// state before it orders anything, then read the order straight off the index and
// stop at the LIMIT.
//
// Two partial indexes per sort are not two indexes' worth of writes: a row is
// either archived or it is not, so an insert lands in exactly one of each pair --
// the same cost as one full index, while each file is half the size.
//
// The timeline order is rewritten from "rank IS NULL, rank" to a COALESCE: an
// expression index has to match the ordering expression exactly, and both forms put
// every known rank first in rank order. The maximum int is the sentinel, and a rank
// never reaches it -- ranks are positions in a timeline that starts at zero.
const schemaV10 = `
-- The bare archive index is what the planner reached for before it had an ordered
-- one: it answered the WHERE and then sorted the result. Every partial index below
-- answers the same WHERE and carries the order, so this one only costs writes now.
DROP INDEX bookmarks_archived_idx;

CREATE INDEX bookmarks_hidden_saved_desc_idx
  ON bookmarks(saved_at DESC, tweet_id ASC) WHERE archived_at IS NULL;
CREATE INDEX bookmarks_hidden_saved_asc_idx
  ON bookmarks(saved_at ASC, tweet_id ASC) WHERE archived_at IS NULL;
CREATE INDEX bookmarks_hidden_created_idx
  ON bookmarks(created_at_on_x_ts DESC, tweet_id ASC) WHERE archived_at IS NULL;
CREATE INDEX bookmarks_hidden_timeline_idx
  ON bookmarks(COALESCE(timeline_rank, 9223372036854775807), saved_at DESC, tweet_id ASC)
  WHERE archived_at IS NULL;
CREATE INDEX bookmarks_hidden_likes_idx
  ON bookmarks(COALESCE(json_extract(metrics_json, '$.likes'), 0) DESC, tweet_id ASC)
  WHERE archived_at IS NULL;

CREATE INDEX bookmarks_archived_saved_desc_idx
  ON bookmarks(saved_at DESC, tweet_id ASC) WHERE archived_at IS NOT NULL;
CREATE INDEX bookmarks_archived_saved_asc_idx
  ON bookmarks(saved_at ASC, tweet_id ASC) WHERE archived_at IS NOT NULL;
CREATE INDEX bookmarks_archived_created_idx
  ON bookmarks(created_at_on_x_ts DESC, tweet_id ASC) WHERE archived_at IS NOT NULL;
CREATE INDEX bookmarks_archived_timeline_idx
  ON bookmarks(COALESCE(timeline_rank, 9223372036854775807), saved_at DESC, tweet_id ASC)
  WHERE archived_at IS NOT NULL;
CREATE INDEX bookmarks_archived_likes_idx
  ON bookmarks(COALESCE(json_extract(metrics_json, '$.likes'), 0) DESC, tweet_id ASC)
  WHERE archived_at IS NOT NULL;
`

// schemaV11 stores the folded text the search and the author condition compare.
//
// Both used to call trove_fold on the column inside the WHERE, which is a Go call
// per row per column -- four of them for every keystroke on every bookmark, before
// the comparison even happens. The folded values are columns now: the trigger keeps
// them in step on every write, whoever writes (a sync, an import, a restore), so the
// query reads a string instead of crossing into Go.
//
// They are plain columns kept by triggers rather than generated columns because
// SQLite refuses ALTER TABLE ... ADD COLUMN with a STORED generated column, and a
// VIRTUAL one is computed on read: it would call the function exactly as before.
// Rebuilding the table to get STORED columns is not an option either -- bookmarks is
// the parent of five ON DELETE CASCADE children, so dropping it would empty them.
//
// The author columns are folded without the leading wildcard, so the search's
// equality and the sidebar's filter can use their indexes.
const schemaV11 = `
ALTER TABLE bookmarks ADD COLUMN text_folded TEXT NOT NULL DEFAULT '';
ALTER TABLE bookmarks ADD COLUMN author_name_folded TEXT NOT NULL DEFAULT '';
ALTER TABLE bookmarks ADD COLUMN author_handle_folded TEXT NOT NULL DEFAULT '';
ALTER TABLE bookmarks ADD COLUMN tweet_url_folded TEXT NOT NULL DEFAULT '';

UPDATE bookmarks SET
  text_folded          = trove_fold(text),
  author_name_folded   = trove_fold(author_name),
  author_handle_folded = trove_fold(author_handle),
  tweet_url_folded     = trove_fold(tweet_url);

CREATE INDEX bookmarks_author_folded_idx ON bookmarks(author_handle_folded);
CREATE INDEX bookmarks_author_name_folded_idx ON bookmarks(author_name_folded);

CREATE TRIGGER bookmarks_folded_insert
AFTER INSERT ON bookmarks
BEGIN
  UPDATE bookmarks SET
    text_folded          = trove_fold(NEW.text),
    author_name_folded   = trove_fold(NEW.author_name),
    author_handle_folded = trove_fold(NEW.author_handle),
    tweet_url_folded     = trove_fold(NEW.tweet_url)
  WHERE tweet_id = NEW.tweet_id;
END;

CREATE TRIGGER bookmarks_folded_update
AFTER UPDATE OF text, author_name, author_handle, tweet_url ON bookmarks
BEGIN
  UPDATE bookmarks SET
    text_folded          = trove_fold(NEW.text),
    author_name_folded   = trove_fold(NEW.author_name),
    author_handle_folded = trove_fold(NEW.author_handle),
    tweet_url_folded     = trove_fold(NEW.tweet_url)
  WHERE tweet_id = NEW.tweet_id;
END;
`

// schemaV12 indexes the suggestion name the sidebar counts.
//
// Every tag summary counted the pending suggestions whose folded name matched the
// tag's, by calling trove_fold on both sides for every tag. With a few thousand
// pending suggestions and a few hundred tags that is a Go call per tag per
// suggestion, on every reload of the sidebar. The folded name is a column kept by
// triggers, and the index is on (status, name_folded) because the count is always
// asking for one status and one name.
const schemaV12 = `
ALTER TABLE tag_suggestions ADD COLUMN name_folded TEXT NOT NULL DEFAULT '';
UPDATE tag_suggestions SET name_folded = trove_fold(trim(tag_name));
CREATE INDEX tag_suggestions_pending_name_idx ON tag_suggestions(status, name_folded);

CREATE TRIGGER tag_suggestions_folded_insert
AFTER INSERT ON tag_suggestions
BEGIN
  UPDATE tag_suggestions SET name_folded = trove_fold(trim(NEW.tag_name)) WHERE id = NEW.id;
END;

CREATE TRIGGER tag_suggestions_folded_update
AFTER UPDATE OF tag_name ON tag_suggestions
BEGIN
  UPDATE tag_suggestions SET name_folded = trove_fold(trim(NEW.tag_name)) WHERE id = NEW.id;
END;
`

// schemaV13 stores the kind of media a post carries as its own column.
//
// The filter needs "has media" and "has video" as SQL conditions over every bookmark,
// and media_json is a JSON array that cannot answer either without a table scan that
// also parses. The column is derived, like is_longform: the payload column stays the
// source of truth and a backup round-trips through it.
const schemaV13 = `
ALTER TABLE bookmarks ADD COLUMN media_kind TEXT NOT NULL DEFAULT '';
`

// backfillMediaKind fills the new column from the payload column already stored.
func backfillMediaKind(tx *sql.Tx) error {
	rows, err := tx.Query(`SELECT tweet_id, media_json FROM bookmarks`)
	if err != nil {
		return err
	}

	type pending struct {
		id   string
		kind string
	}
	var updates []pending
	for rows.Next() {
		var id string
		var raw sql.NullString
		if err := rows.Scan(&id, &raw); err != nil {
			rows.Close()
			return err
		}
		media, err := decodeMedia(raw)
		if err != nil {
			rows.Close()
			return err
		}
		if kind := domain.SummarizeMediaKind(media); kind != "" {
			updates = append(updates, pending{id: id, kind: kind})
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	for _, update := range updates {
		if _, err := tx.Exec(`UPDATE bookmarks SET media_kind = ? WHERE tweet_id = ?`,
			update.kind, update.id); err != nil {
			return err
		}
	}
	return nil
}

// schemaV14 lets a saved view keep the video filter with the rest of its conditions.
const schemaV14 = `
ALTER TABLE saved_views ADD COLUMN only_with_video INTEGER NOT NULL DEFAULT 0;
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

// schemaV1 is the whole initial schema, including the fields an earlier plan
// omitted (author_avatar_url, hashtags_json, source_kind, tag aliases and
// parent_id).
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

-- error_kind carries no CHECK constraint on purpose: rows written by older
-- builds use a different vocabulary, and validation must accept both.
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
