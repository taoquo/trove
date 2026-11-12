package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/taoquo/x-master/apps/macos/internal/domain"
	"github.com/taoquo/x-master/apps/macos/internal/ports"
)

// This file holds the four stores that back the app's own state rather than the
// library: the auto-tagger's ledger, key-value settings, saved views and the sync
// history.

// --- the auto-tagger's idempotency ledger ---

// AnalysisRepository stores one row per analysed bookmark.
type AnalysisRepository struct {
	db *DB
}

// NewAnalysisRepository builds a repository over an open database.
func NewAnalysisRepository(db *DB) *AnalysisRepository { return &AnalysisRepository{db: db} }

var _ ports.AnalysisRepository = (*AnalysisRepository)(nil)

const analysisColumns = `bookmark_id, analyzed_at, prompt_version, model, content_hash, outcome, tag_count`

// List returns the whole ledger.
func (r *AnalysisRepository) List(ctx context.Context) ([]domain.LlmAnalysis, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+analysisColumns+` FROM llm_analysis`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list analysis: %w", err)
	}
	defer rows.Close()

	var records []domain.LlmAnalysis
	for rows.Next() {
		var record domain.LlmAnalysis
		if err := rows.Scan(&record.BookmarkID, &record.AnalyzedAt, &record.PromptVersion,
			&record.Model, &record.ContentHash, &record.Outcome, &record.TagCount); err != nil {
			return nil, fmt.Errorf("sqlite: scan analysis: %w", err)
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

// UpsertMany records verdicts, replacing any previous row for the same bookmark.
//
// bookmark_id is the primary key, so one bookmark has one current verdict. That
// is what makes the ledger an answer cache rather than a history: a stale verdict
// for a different model or text must be replaced, not appended beside.
func (r *AnalysisRepository) UpsertMany(ctx context.Context, records []domain.LlmAnalysis) (int, error) {
	if len(records) == 0 {
		return 0, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("sqlite: begin analysis upsert: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op once committed

	statement, err := tx.PrepareContext(ctx,
		`INSERT INTO llm_analysis (`+analysisColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(bookmark_id) DO UPDATE SET
		   analyzed_at    = excluded.analyzed_at,
		   prompt_version = excluded.prompt_version,
		   model          = excluded.model,
		   content_hash   = excluded.content_hash,
		   outcome        = excluded.outcome,
		   tag_count      = excluded.tag_count`)
	if err != nil {
		return 0, fmt.Errorf("sqlite: prepare analysis upsert: %w", err)
	}
	defer statement.Close()

	written := 0
	for _, record := range records {
		if _, err := statement.ExecContext(ctx, record.BookmarkID, record.AnalyzedAt,
			record.PromptVersion, record.Model, record.ContentHash, record.Outcome, record.TagCount); err != nil {
			return 0, fmt.Errorf("sqlite: upsert analysis for %s: %w", record.BookmarkID, err)
		}
		written++
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("sqlite: commit analysis upsert: %w", err)
	}
	return written, nil
}

// ExistingFor returns the content hash of each bookmark whose stored verdict is
// still valid for the given model and prompt version.
//
// A bookmark's text can change between syncs, so the hash is compared: without it
// an edited post would keep a verdict about text that no longer exists.
func (r *AnalysisRepository) ExistingFor(
	ctx context.Context,
	model string,
	promptVersion int,
	contentHashes map[string]string,
) (map[string]string, error) {
	reusable := map[string]string{}
	if len(contentHashes) == 0 {
		return reusable, nil
	}

	ids := make([]string, 0, len(contentHashes))
	for id := range contentHashes {
		ids = append(ids, id)
	}

	for _, batch := range chunkStrings(ids, 500) {
		query := `SELECT bookmark_id, content_hash FROM llm_analysis
		          WHERE model = ? AND prompt_version = ?
		            AND bookmark_id IN (` + placeholders(len(batch)) + `)`
		args := append([]any{model, promptVersion}, argsFor(batch)...)

		rows, err := r.db.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, fmt.Errorf("sqlite: read reusable analysis: %w", err)
		}
		for rows.Next() {
			var id, hash string
			if err := rows.Scan(&id, &hash); err != nil {
				rows.Close()
				return nil, err
			}
			// The hash must match the current text, or the verdict is about text
			// that has since changed.
			if expected, present := contentHashes[id]; present && expected != "" && expected == hash {
				reusable[id] = hash
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}

	return reusable, nil
}

// DeleteAll empties the ledger and returns how many rows went.
func (r *AnalysisRepository) DeleteAll(ctx context.Context) (int, error) {
	var before int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM llm_analysis`).Scan(&before); err != nil {
		return 0, fmt.Errorf("sqlite: count analysis: %w", err)
	}
	if before == 0 {
		return 0, nil
	}
	if _, err := r.db.ExecContext(ctx, `DELETE FROM llm_analysis`); err != nil {
		return 0, fmt.Errorf("sqlite: delete analysis: %w", err)
	}
	return before, nil
}

// --- key-value settings ---

// SettingsRepository stores the configuration as JSON values.
type SettingsRepository struct {
	db *DB
}

// NewSettingsRepository builds a repository over an open database.
func NewSettingsRepository(db *DB) *SettingsRepository { return &SettingsRepository{db: db} }

var _ ports.SettingsRepository = (*SettingsRepository)(nil)

// Get reads one setting.
func (r *SettingsRepository) Get(ctx context.Context, key string) (string, bool, error) {
	var value string
	err := r.db.QueryRowContext(ctx, `SELECT value_json FROM app_settings WHERE key = ?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("sqlite: get setting %s: %w", key, err)
	}
	return value, true, nil
}

// Set writes one setting.
func (r *SettingsRepository) Set(ctx context.Context, key, valueJSON string, now time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO app_settings (key, value_json, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET value_json = excluded.value_json, updated_at = excluded.updated_at`,
		key, valueJSON, domain.FormatTimestamp(now))
	if err != nil {
		return fmt.Errorf("sqlite: set setting %s: %w", key, err)
	}
	return nil
}

// All returns every setting.
func (r *SettingsRepository) All(ctx context.Context) (map[string]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT key, value_json FROM app_settings`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list settings: %w", err)
	}
	defer rows.Close()

	values := map[string]string{}
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		values[key] = value
	}
	return values, rows.Err()
}

// Delete removes settings and returns how many existed.
func (r *SettingsRepository) Delete(ctx context.Context, keys ...string) (int, error) {
	if len(keys) == 0 {
		return 0, nil
	}

	total := 0
	for _, batch := range chunkStrings(keys, 500) {
		result, err := r.db.ExecContext(ctx,
			`DELETE FROM app_settings WHERE key IN (`+placeholders(len(batch))+`)`, argsFor(batch)...)
		if err != nil {
			return 0, fmt.Errorf("sqlite: delete settings: %w", err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}
		total += int(affected)
	}
	return total, nil
}

// --- saved views ---

// SavedViewRepository stores named filter sets.
type SavedViewRepository struct {
	db *DB
}

// NewSavedViewRepository builds a repository over an open database.
func NewSavedViewRepository(db *DB) *SavedViewRepository { return &SavedViewRepository{db: db} }

var _ ports.SavedViewRepository = (*SavedViewRepository)(nil)

const savedViewColumns = `id, name, query, active_tag_ids_json, active_author_handles_json,
	only_with_media, only_longform, sort_order, view_mode, created_at, updated_at`

func scanSavedView(scanner interface{ Scan(...any) error }) (domain.SavedView, error) {
	var (
		view         domain.SavedView
		tagIDsRaw    sql.NullString
		handlesRaw   sql.NullString
		onlyMedia    int
		onlyLongform int
		createdAt    string
		updatedAt    string
	)
	err := scanner.Scan(&view.ID, &view.Name, &view.Query, &tagIDsRaw, &handlesRaw,
		&onlyMedia, &onlyLongform, &view.SortOrder, &view.ViewMode, &createdAt, &updatedAt)
	if err != nil {
		return domain.SavedView{}, err
	}

	view.ActiveTagIDs, _ = decodeStrings(tagIDsRaw)
	view.ActiveAuthorHandles, _ = decodeStrings(handlesRaw)
	view.OnlyWithMedia = onlyMedia != 0
	view.OnlyLongform = onlyLongform != 0
	view.CreatedAt = domain.MustParseTimestamp(createdAt)
	view.UpdatedAt = domain.MustParseTimestamp(updatedAt)
	return view, nil
}

// List returns every saved view.
func (r *SavedViewRepository) List(ctx context.Context) ([]domain.SavedView, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+savedViewColumns+` FROM saved_views ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list saved views: %w", err)
	}
	defer rows.Close()

	var views []domain.SavedView
	for rows.Next() {
		view, err := scanSavedView(rows)
		if err != nil {
			return nil, fmt.Errorf("sqlite: scan saved view: %w", err)
		}
		views = append(views, view)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// The extension does not order its saved views, so the name order here is an addition --
	// and an addition that compared bytes would disagree with the lists and tags beside it.
	// One comparator for one question: domain.CompareNaturalNames.
	sort.SliceStable(views, func(i, j int) bool {
		return domain.CompareNaturalNames(views[i].Name, views[j].Name) < 0
	})
	return views, nil
}

// Get returns one saved view.
func (r *SavedViewRepository) Get(ctx context.Context, id string) (domain.SavedView, bool, error) {
	view, err := scanSavedView(r.db.QueryRowContext(ctx,
		`SELECT `+savedViewColumns+` FROM saved_views WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.SavedView{}, false, nil
	}
	if err != nil {
		return domain.SavedView{}, false, fmt.Errorf("sqlite: get saved view %s: %w", id, err)
	}
	return view, true, nil
}

// Upsert writes a saved view.
func (r *SavedViewRepository) Upsert(ctx context.Context, view domain.SavedView) error {
	tagIDsJSON, err := encodeStrings(view.ActiveTagIDs)
	if err != nil {
		return err
	}
	handlesJSON, err := encodeStrings(view.ActiveAuthorHandles)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx,
		`INSERT INTO saved_views (`+savedViewColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   name = excluded.name, query = excluded.query,
		   active_tag_ids_json = excluded.active_tag_ids_json,
		   active_author_handles_json = excluded.active_author_handles_json,
		   only_with_media = excluded.only_with_media, only_longform = excluded.only_longform,
		   sort_order = excluded.sort_order, view_mode = excluded.view_mode,
		   updated_at = excluded.updated_at`,
		view.ID, view.Name, view.Query, tagIDsJSON, handlesJSON,
		boolToInt(view.OnlyWithMedia), boolToInt(view.OnlyLongform),
		view.SortOrder, view.ViewMode,
		domain.FormatTimestamp(view.CreatedAt), domain.FormatTimestamp(view.UpdatedAt))
	if err != nil {
		return fmt.Errorf("sqlite: upsert saved view %s: %w", view.ID, err)
	}
	return nil
}

// Delete removes a saved view.
func (r *SavedViewRepository) Delete(ctx context.Context, id string) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM saved_views WHERE id = ?`, id); err != nil {
		return fmt.Errorf("sqlite: delete saved view %s: %w", id, err)
	}
	return nil
}

// --- sync history ---

// SyncRunRepository stores sync attempts.
type SyncRunRepository struct {
	db *DB
}

// NewSyncRunRepository builds a repository over an open database.
func NewSyncRunRepository(db *DB) *SyncRunRepository { return &SyncRunRepository{db: db} }

var _ ports.SyncRunRepository = (*SyncRunRepository)(nil)

const syncRunColumns = `id, status, started_at, finished_at, fetched_count, inserted_count,
	updated_count, failed_count, pages_completed, error_kind, error_summary, strategy_version`

func scanSyncRun(scanner interface{ Scan(...any) error }) (domain.SyncRun, error) {
	var (
		run          domain.SyncRun
		startedAt    string
		finishedAt   sql.NullString
		errorKind    sql.NullString
		errorSummary sql.NullString
	)
	err := scanner.Scan(&run.ID, &run.Status, &startedAt, &finishedAt,
		&run.FetchedCount, &run.InsertedCount, &run.UpdatedCount, &run.FailedCount,
		&run.PagesCompleted, &errorKind, &errorSummary, &run.StrategyVersion)
	if err != nil {
		return domain.SyncRun{}, err
	}

	run.StartedAt = domain.MustParseTimestamp(startedAt)
	if finishedAt.Valid && finishedAt.String != "" {
		finished := domain.MustParseTimestamp(finishedAt.String)
		run.FinishedAt = &finished
	}
	run.ErrorKind = errorKind.String
	run.ErrorSummary = errorSummary.String
	return run, nil
}

// Start records a run that is beginning.
func (r *SyncRunRepository) Start(ctx context.Context, run domain.SyncRun) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO sync_runs (`+syncRunColumns+`) VALUES (?, ?, ?, NULL, ?, ?, ?, ?, ?, NULL, NULL, ?)
		 ON CONFLICT(id) DO UPDATE SET status = excluded.status, started_at = excluded.started_at`,
		run.ID, run.Status, domain.FormatTimestamp(run.StartedAt),
		run.FetchedCount, run.InsertedCount, run.UpdatedCount, run.FailedCount,
		run.PagesCompleted, run.StrategyVersion)
	if err != nil {
		return fmt.Errorf("sqlite: start sync run %s: %w", run.ID, err)
	}
	return nil
}

// Finish writes a run's final state.
func (r *SyncRunRepository) Finish(ctx context.Context, run domain.SyncRun) error {
	finishedAt := sql.NullString{}
	if run.FinishedAt != nil {
		finishedAt = sql.NullString{String: domain.FormatTimestamp(*run.FinishedAt), Valid: true}
	}

	_, err := r.db.ExecContext(ctx,
		`UPDATE sync_runs SET status = ?, finished_at = ?, fetched_count = ?, inserted_count = ?,
		   updated_count = ?, failed_count = ?, pages_completed = ?,
		   error_kind = ?, error_summary = ?, strategy_version = ?
		 WHERE id = ?`,
		run.Status, finishedAt, run.FetchedCount, run.InsertedCount, run.UpdatedCount,
		run.FailedCount, run.PagesCompleted, nullString(run.ErrorKind), nullString(run.ErrorSummary),
		run.StrategyVersion, run.ID)
	if err != nil {
		return fmt.Errorf("sqlite: finish sync run %s: %w", run.ID, err)
	}
	return nil
}

// Get returns one run.
func (r *SyncRunRepository) Get(ctx context.Context, id string) (domain.SyncRun, bool, error) {
	run, err := scanSyncRun(r.db.QueryRowContext(ctx,
		`SELECT `+syncRunColumns+` FROM sync_runs WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.SyncRun{}, false, nil
	}
	if err != nil {
		return domain.SyncRun{}, false, fmt.Errorf("sqlite: get sync run %s: %w", id, err)
	}
	return run, true, nil
}

// Latest returns the most recently started run.
func (r *SyncRunRepository) Latest(ctx context.Context) (domain.SyncRun, bool, error) {
	run, err := scanSyncRun(r.db.QueryRowContext(ctx,
		`SELECT `+syncRunColumns+` FROM sync_runs ORDER BY started_at DESC, id DESC LIMIT 1`))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.SyncRun{}, false, nil
	}
	if err != nil {
		return domain.SyncRun{}, false, fmt.Errorf("sqlite: latest sync run: %w", err)
	}
	return run, true, nil
}

// Recent returns the newest runs.
func (r *SyncRunRepository) Recent(ctx context.Context, limit int) ([]domain.SyncRun, error) {
	if limit <= 0 {
		limit = 200
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT `+syncRunColumns+` FROM sync_runs ORDER BY started_at DESC, id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("sqlite: recent sync runs: %w", err)
	}
	defer rows.Close()

	runs := make([]domain.SyncRun, 0, limit)
	for rows.Next() {
		run, err := scanSyncRun(rows)
		if err != nil {
			return nil, fmt.Errorf("sqlite: scan sync run: %w", err)
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

// Active returns a run that is still marked running.
func (r *SyncRunRepository) Active(ctx context.Context) (domain.SyncRun, bool, error) {
	run, err := scanSyncRun(r.db.QueryRowContext(ctx,
		`SELECT `+syncRunColumns+` FROM sync_runs WHERE status = 'running'
		 ORDER BY started_at DESC, id DESC LIMIT 1`))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.SyncRun{}, false, nil
	}
	if err != nil {
		return domain.SyncRun{}, false, fmt.Errorf("sqlite: active sync run: %w", err)
	}
	return run, true, nil
}

// MarkInterrupted closes out runs left running by a crash.
//
// This is the startup self-heal (defect S7). A run that died without reporting
// leaves the UI claiming a sync is in flight forever, and nothing else in the app
// will ever clear it: the process that owned the run is gone.
//
// The kind is "interrupted" specifically, not "unknown": the cause is known, and
// its remedy — just sync again — is different from a schema change's.
func (r *SyncRunRepository) MarkInterrupted(ctx context.Context, now time.Time) (int, error) {
	result, err := r.db.ExecContext(ctx,
		`UPDATE sync_runs
		 SET status = 'error', finished_at = ?, error_kind = ?, error_summary = ?
		 WHERE status = 'running'`,
		domain.FormatTimestamp(now), domain.ErrInterrupted,
		"the app stopped before this sync finished")
	if err != nil {
		return 0, fmt.Errorf("sqlite: mark interrupted sync runs: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(affected), nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

// The reset action's confirmation promises that local settings are cleared too.
func (r *SettingsRepository) DeleteAll(ctx context.Context) (int, error) {
	result, err := r.db.ExecContext(ctx, `DELETE FROM app_settings`)
	if err != nil {
		return 0, fmt.Errorf("sqlite: delete all settings: %w", err)
	}
	removed, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("sqlite: delete all settings: %w", err)
	}
	return int(removed), nil
}
