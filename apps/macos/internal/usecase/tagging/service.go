// Package tagging runs the auto-tagger against real storage.
//
// The engine in internal/autotag is pure: it takes a snapshot and callbacks. This
// package is what supplies them from the repositories, which is what turns the
// engine's tested logic into something the app can actually run.
package tagging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	engine "github.com/taoquo/trove/apps/macos/internal/autotag"
	"github.com/taoquo/trove/apps/macos/internal/domain"
	"github.com/taoquo/trove/apps/macos/internal/ports"
)

// Setting keys. They live here rather than in a shared constants file because
// only this package reads them, and a key is a private detail of its writer.
const (
	settingsKey = "autotag.settings"
	summaryKey  = "autotag.lastSummary"
	runStateKey = "autotag.runState"
)

// ErrAlreadyRunning reports a second run started while one is in flight.
//
// This is the concurrency guard for defect S6: two runs over the same library
// would both write suggestions and both charge the same budget, and the run-id
// protocol only protects the state, not the work.
var ErrAlreadyRunning = errors.New("tagging: a run is already in progress")

// Service runs the auto-tagger.
type Service struct {
	repos  ports.Repositories
	clock  ports.Clock
	client engine.LLMClient
	// buildClient is the per-run factory, when one is installed.
	buildClient func(engine.Settings) (engine.LLMClient, error)
	// testConnection probes the configured provider, when one is installed.
	testConnection func(engine.Settings) engine.ConnectionTestResult
	// clientErr records why the last build failed.
	clientErr error

	mu sync.Mutex
	// running is the mutual exclusion; state is the live run the UI reads.
	running bool
	state   *engine.RunState
}

// New builds a service.
func New(repos ports.Repositories, clock ports.Clock, client engine.LLMClient) *Service {
	if clock == nil {
		clock = ports.SystemClock{}
	}
	return &Service{repos: repos, clock: clock, client: client}
}

// UseProviderClient installs a factory that builds the model client from the
// settings of each run.
//
// The client is built per run rather than once at construction because a provider, an
// endpoint or a key changed in the settings has to apply to the next run, and because
// a missing key should fail that run with a message rather than prevent the app from
// starting.
func (s *Service) UseProviderClient(build func(engine.Settings) (engine.LLMClient, error)) {
	s.buildClient = build
}

// UseConnectionTester installs the probe the settings panel calls.
//
// It is a separate seam from UseProviderClient because the two answer different
// questions: one builds the client a run needs, the other asks the provider whether
// the configuration works. Both are installed from the same credential resolver in
// the composition root, so a test cannot pass against a different configuration than
// the run would use.
func (s *Service) UseConnectionTester(test func(engine.Settings) engine.ConnectionTestResult) {
	s.testConnection = test
}

// TestConnection probes the stored configuration.
//
// With no tester installed it reports a failure rather than pretending success: a
// panel that says "connected" without having asked anyone is worse than one that
// says it cannot tell.
func (s *Service) TestConnection(ctx context.Context) engine.ConnectionTestResult {
	settings, err := s.LoadSettings(ctx)
	if err != nil {
		return engine.ConnectionTestResult{OK: false, ErrorKind: "unknown", Message: err.Error()}
	}
	if s.testConnection == nil {
		return engine.ConnectionTestResult{
			OK: false, ErrorKind: "unknown",
			Message: "no connection tester is configured", ProviderID: settings.LLM.ProviderID,
		}
	}
	return s.testConnection(settings)
}

// resolveClient returns the client for a run.
//
// A nil client from the factory is not an error: a disabled feature builds nothing,
// and the engine reports the disabled outcome on its own.
func (s *Service) resolveClient(ctx context.Context, settings engine.Settings) engine.LLMClient {
	if s.buildClient == nil {
		return s.client
	}

	client, err := s.buildClient(settings)
	if err != nil {
		// Kept for the run to report, rather than swallowed: the user asked for a run
		// and needs to know why it could not call the provider.
		s.clientErr = err
		return nil
	}
	s.clientErr = nil
	return client
}

// ClientError is why the model client could not be built, or nil.
//
// It exists because a construction failure happens before the engine has a run to
// report it in, so the caller has to be able to ask.
func (s *Service) ClientError() error { return s.clientErr }

// LoadSettings reads the stored configuration, falling back to the defaults.
func (s *Service) LoadSettings(ctx context.Context) (engine.Settings, error) {
	settings := engine.DefaultSettings()
	if s.repos.Settings == nil {
		return settings, nil
	}

	raw, found, err := s.repos.Settings.Get(ctx, settingsKey)
	if err != nil {
		return engine.Settings{}, err
	}
	if !found || raw == "" {
		return settings, nil
	}

	// A stored blob from an older or newer build must not brick the feature: an
	// unreadable value falls back to the defaults rather than failing the caller,
	// because the setting is recoverable and the library is not.
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		return engine.DefaultSettings(), nil
	}
	// The second layer. A blob that decoded can still hold a field this build cannot use --
	// a width from a newer version, a mode that was renamed -- and returning it unchanged
	// would let one bad value stand forever. The extension normalizes everything it reads
	// for the same reason.
	return engine.NormalizeSettings(settings), nil
}

// SaveSettings writes the configuration.
func (s *Service) SaveSettings(ctx context.Context, settings engine.Settings) error {
	if s.repos.Settings == nil {
		return errors.New("tagging: no settings store configured")
	}
	encoded, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("tagging: encode settings: %w", err)
	}
	return s.repos.Settings.Set(ctx, settingsKey, string(encoded), s.clock.Now())
}

// CurrentRun returns the live run state, or nil.
//
// It is already expired if its heartbeat stopped moving; the UI is expected to
// check IsActive rather than trust a non-nil pointer.
func (s *Service) CurrentRun() *engine.RunState {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == nil {
		return nil
	}
	copied := *s.state
	return &copied
}

// IsRunning reports whether a run is actually in flight right now.
func (s *Service) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return engine.IsActive(s.state, s.clock.Now(), engine.RunStaleAfter)
}

// RequestCancel asks the running pipeline to stop at its next wave boundary.
//
// It deliberately does not kill anything: suggestions already written are kept,
// and the pipeline stops before starting more work.
func (s *Service) RequestCancel() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == nil {
		return false
	}
	updated, changed := engine.RequestCancel(s.state)
	if !changed {
		return false
	}
	s.state = updated
	go s.persistRunState(context.Background(), updated)
	return true
}

// Run executes one pass.
//
// It takes the mutual exclusion before doing anything, so a second caller is
// rejected rather than queued: queueing would let a user's repeated clicks turn
// into a long chain of runs against the same key.
func (s *Service) Run(ctx context.Context, settings engine.Settings) (engine.RunSummary, error) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return engine.RunSummary{Outcome: engine.OutcomeFailed, Error: ErrAlreadyRunning.Error()}, ErrAlreadyRunning
	}
	s.running = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	summary, err := engine.Run(ctx, s.deps(ctx, settings))
	if err != nil && summary.Outcome == "" {
		return summary, err
	}
	return summary, err
}

// deps builds the engine's collaborators from the repositories.
func (s *Service) deps(ctx context.Context, settings engine.Settings) engine.Deps {
	return engine.Deps{
		Settings: settings,
		Snapshot: s.loadSnapshot,
		InsertSuggestions: func(ctx context.Context, suggestions []domain.TagSuggestion) (int, error) {
			if s.repos.Suggestions == nil {
				return 0, errors.New("tagging: no suggestion store configured")
			}
			return s.repos.Suggestions.InsertMany(ctx, suggestions)
		},
		PersistAnalysis: func(ctx context.Context, records []domain.LlmAnalysis) error {
			if s.repos.Analysis == nil {
				return errors.New("tagging: no analysis store configured")
			}
			_, err := s.repos.Analysis.UpsertMany(ctx, records)
			return err
		},
		Client:       s.resolveClient(ctx, settings),
		ClientError:  s.clientErr,
		SaveRunState: s.saveRunState,
		// LoadRunState is what carries a cancel from the UI into a running
		// pipeline: the engine's own copy of the flag is stale by construction.
		LoadRunState:  s.loadRunState,
		ClearRunState: s.clearRunState,
		RecordSummary: s.recordSummary,
		Now:           s.clock.Now,
	}
}

// loadRunState returns the live state, so a cancel reaches the pipeline.
func (s *Service) loadRunState(context.Context) (*engine.RunState, error) {
	return s.CurrentRun(), nil
}

// loadSnapshot reads everything a run works from.
func (s *Service) loadSnapshot(ctx context.Context) (engine.Snapshot, error) {
	snapshot := engine.Snapshot{}

	var err error
	if snapshot.Bookmarks, err = s.repos.Bookmarks.All(ctx); err != nil {
		return engine.Snapshot{}, err
	}
	if snapshot.Tags, err = s.repos.Tags.List(ctx); err != nil {
		return engine.Snapshot{}, err
	}
	if snapshot.BookmarkTags, err = s.repos.Tags.AllRelations(ctx); err != nil {
		return engine.Snapshot{}, err
	}

	// The negative cache must include every suggestion ever made, whatever its
	// status, or the same pair is offered again after the user answers it.
	if s.repos.Suggestions != nil {
		ids, err := s.repos.Suggestions.AllIDs(ctx)
		if err != nil {
			return engine.Snapshot{}, err
		}
		// The engine takes the ids through the existing-suggestions input, which
		// is keyed by the composite id.
		snapshot.Suggestions = make([]domain.TagSuggestion, 0, len(ids))
		for id := range ids {
			snapshot.Suggestions = append(snapshot.Suggestions, domain.TagSuggestion{ID: id})
		}
	}
	if s.repos.Analysis != nil {
		if snapshot.Analysis, err = s.repos.Analysis.List(ctx); err != nil {
			return engine.Snapshot{}, err
		}
	}

	return snapshot, nil
}

// saveRunState keeps the live state in memory for the UI and persists a copy.
func (s *Service) saveRunState(ctx context.Context, state *engine.RunState) error {
	if state == nil {
		return nil
	}

	s.mu.Lock()
	// A takeover installs a different run id. A superseded run must not overwrite
	// the state of the run that replaced it.
	if s.state != nil && s.state.RunID != state.RunID {
		s.mu.Unlock()
		return nil
	}

	copied := *state
	// The pipeline's own copy of the cancel flag is stale by construction: the UI
	// writes the flag to *this* state, and the engine only re-reads it at a wave
	// boundary. Overwriting here would discard a cancel the user just requested
	// and the run would carry on to the next wave.
	if s.state != nil && s.state.RunID == state.RunID && s.state.CancelRequested {
		copied.CancelRequested = true
	}
	s.state = &copied
	s.mu.Unlock()

	return s.persistRunState(ctx, state)
}

// clearRunState removes the state, but only for the run that owns it.
func (s *Service) clearRunState(ctx context.Context, runID string) (bool, error) {
	s.mu.Lock()
	if s.state != nil && s.state.RunID != runID {
		// Someone else owns it now; clearing would hide their run.
		s.mu.Unlock()
		return false, nil
	}
	s.state = nil
	s.mu.Unlock()

	if s.repos.Settings != nil {
		if _, err := s.repos.Settings.Delete(ctx, runStateKey); err != nil {
			return false, err
		}
	}
	return true, nil
}

// persistRunState writes the state so a run killed mid-flight leaves a trace.
//
// Without it a crash would leave nothing at all: the UI would show the previous
// result while looking frozen, which is the failure the double-window protocol
// exists to prevent.
func (s *Service) persistRunState(ctx context.Context, state *engine.RunState) error {
	if s.repos.Settings == nil || state == nil {
		return nil
	}
	encoded, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("tagging: encode run state: %w", err)
	}
	return s.repos.Settings.Set(ctx, runStateKey, string(encoded), s.clock.Now())
}

// recordSummary stores the finished run's report.
func (s *Service) recordSummary(ctx context.Context, summary engine.RunSummary) error {
	if s.repos.Settings == nil {
		return nil
	}
	encoded, err := json.Marshal(summary)
	if err != nil {
		return fmt.Errorf("tagging: encode summary: %w", err)
	}
	return s.repos.Settings.Set(ctx, summaryKey, string(encoded), s.clock.Now())
}

// LastSummary returns the most recent run's report.
func (s *Service) LastSummary(ctx context.Context) (engine.RunSummary, bool, error) {
	if s.repos.Settings == nil {
		return engine.RunSummary{}, false, nil
	}
	raw, found, err := s.repos.Settings.Get(ctx, summaryKey)
	if err != nil || !found || raw == "" {
		return engine.RunSummary{}, false, err
	}

	var summary engine.RunSummary
	if err := json.Unmarshal([]byte(raw), &summary); err != nil {
		return engine.RunSummary{}, false, nil
	}
	return summary, true, nil
}

// HealStaleRun closes out a run that was in flight when the app last stopped.
//
// This is the auto-tagger's half of startup self-heal. A run cannot survive a
// restart — the process that was doing the work is gone — so a persisted state is
// always the trace of a run that died, and leaving it would tell the user a run
// is progressing when nothing is.
//
// It returns whether anything was healed.
func (s *Service) HealStaleRun(ctx context.Context) (bool, error) {
	s.mu.Lock()
	s.state = nil
	s.mu.Unlock()

	if s.repos.Settings == nil {
		return false, nil
	}

	raw, found, err := s.repos.Settings.Get(ctx, runStateKey)
	if err != nil {
		return false, err
	}
	if !found || raw == "" {
		return false, nil
	}

	if _, err := s.repos.Settings.Delete(ctx, runStateKey); err != nil {
		return false, err
	}

	// Record why, so the panel can say what happened instead of silently showing
	// nothing. The state that died is the only record of how far the run got, so the
	// counts are read from it rather than from the previous run's summary -- which
	// reported a pass the user never watched.
	var stopped engine.RunState
	_ = json.Unmarshal([]byte(raw), &stopped)

	summary := engine.RunSummary{
		FinishedAt:        s.clock.Now(),
		CandidateCount:    stopped.Total,
		LLMProcessedCount: stopped.Processed,
		UnprocessedCount:  maxInt(0, stopped.Total-stopped.Processed),
	}
	// A run the user stopped is not a run the app interrupted: only the second may be
	// continued automatically without asking.
	if stopped.CancelRequested {
		summary.Outcome = engine.OutcomeCancelled
	} else {
		summary.Outcome = engine.OutcomeInterrupted
		summary.Error = "the app stopped before this run finished"
	}
	if s.repos.Suggestions != nil {
		if pending, err := s.repos.Suggestions.PendingCount(ctx); err == nil {
			summary.PendingCount = pending
		}
	}
	if settings, err := s.LoadSettings(ctx); err == nil {
		summary.BlockedTagKeyCount = len(settings.BlockedTagKeys)
	}

	if err := s.recordSummary(ctx, summary); err != nil {
		return false, err
	}
	return true, nil
}

// maxInt is this file's floor at zero. The toolchain has a builtin max, but the two
// values here are both counts, and naming the operation says what the floor is for.
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// EnsureInboxAfterSync gives every bookmarkless-of-a-list bookmark the Inbox.
//
// It lives here because it is the step that must follow any write that inserts
// bookmarks; without it a new bookmark is invisible in every list view.
func (s *Service) EnsureInboxAfterSync(ctx context.Context) (int, error) {
	if s.repos.Lists != nil {
		if err := s.repos.Lists.EnsureInbox(ctx, s.clock.Now()); err != nil {
			return 0, err
		}
	}
	return s.repos.Bookmarks.AssignMissingToInbox(ctx, s.clock.Now())
}

// ReviewVocabulary asks the model to consolidate the whole tag list.
//
// It is separate from Run: a review proposes merges and renames for the user to accept,
// while a run proposes per-bookmark tags. They share the provider, the endpoint and the
// credential, and nothing else.
//
// A missing client is reported in the result rather than as an error, because the panel
// shows a failure the same way whether the request failed or never left.
func (s *Service) ReviewVocabulary(ctx context.Context) engine.TagVocabularyReviewResult {
	empty := engine.TagVocabularyReviewResult{Merges: []engine.TagMergePlan{}, Renames: []engine.TagRenamePlan{}}

	settings, err := s.LoadSettings(ctx)
	if err != nil {
		return engine.TagVocabularyReviewResult{Error: err.Error(), Merges: empty.Merges, Renames: empty.Renames}
	}

	client := s.resolveClient(ctx, settings)
	if client == nil {
		message := "no model client configured"
		if s.clientErr != nil {
			message = s.clientErr.Error()
		}
		return engine.TagVocabularyReviewResult{Error: message, Merges: empty.Merges, Renames: empty.Renames}
	}

	chatClient, ok := client.(interface {
		Chat(ctx context.Context, prompt string) (string, error)
	})
	if !ok {
		return engine.TagVocabularyReviewResult{
			Error:  "the configured client cannot answer a free-form prompt",
			Merges: empty.Merges, Renames: empty.Renames,
		}
	}

	tags, err := s.repos.Tags.List(ctx)
	if err != nil {
		return engine.TagVocabularyReviewResult{Error: err.Error(), Merges: empty.Merges, Renames: empty.Renames}
	}
	relations, err := s.repos.Tags.AllRelations(ctx)
	if err != nil {
		return engine.TagVocabularyReviewResult{Error: err.Error(), Merges: empty.Merges, Renames: empty.Renames}
	}

	result, err := engine.ReviewTagVocabulary(tags, relations, func(prompt string) (string, error) {
		return chatClient.Chat(ctx, prompt)
	})
	if err != nil {
		return engine.TagVocabularyReviewResult{Error: err.Error(), Merges: empty.Merges, Renames: empty.Renames}
	}
	return result
}

// BlockTagKey remembers a tag name so auto-tagging never suggests it again.
//
// Deleting a tag is not enough on its own: the rule engine and the LLM step both work
// from names, so a deleted name comes back the next time a post mentions it. The
// extension's tagsStore says so at the delete site, and its caller does this as the third
// of three steps.
//
// Re-creating the tag by hand clears the block -- see UnblockTagKey -- because a user who
// types the name back out has answered the question the block was holding.
func (s *Service) BlockTagKey(ctx context.Context, tagName string) error {
	key := domain.CanonicalTagKey(tagName)
	if key == "" {
		return nil
	}
	settings, err := s.LoadSettings(ctx)
	if err != nil {
		return err
	}
	for _, existing := range settings.BlockedTagKeys {
		if existing == key {
			return nil // already blocked; a second delete must not grow the list
		}
	}
	settings.BlockedTagKeys = append(settings.BlockedTagKeys, key)
	return s.SaveSettings(ctx, settings)
}

// UnblockTagKey clears the block on a name.
func (s *Service) UnblockTagKey(ctx context.Context, tagName string) error {
	key := domain.CanonicalTagKey(tagName)
	if key == "" {
		return nil
	}
	settings, err := s.LoadSettings(ctx)
	if err != nil {
		return err
	}
	kept := settings.BlockedTagKeys[:0]
	for _, existing := range settings.BlockedTagKeys {
		if existing != key {
			kept = append(kept, existing)
		}
	}
	if len(kept) == len(settings.BlockedTagKeys) {
		return nil // nothing blocked under this key
	}
	settings.BlockedTagKeys = kept
	return s.SaveSettings(ctx, settings)
}

// ClearBlockedTagKeys empties the list, for the settings panel's button.
func (s *Service) ClearBlockedTagKeys(ctx context.Context) error {
	settings, err := s.LoadSettings(ctx)
	if err != nil {
		return err
	}
	if len(settings.BlockedTagKeys) == 0 {
		return nil
	}
	settings.BlockedTagKeys = nil
	return s.SaveSettings(ctx, settings)
}
