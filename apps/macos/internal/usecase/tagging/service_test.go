package tagging

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	engine "github.com/taoquo/x-master/apps/macos/internal/autotag"
	"github.com/taoquo/x-master/apps/macos/internal/domain"
	"github.com/taoquo/x-master/apps/macos/internal/llm"
	"github.com/taoquo/x-master/apps/macos/internal/ports"
	"github.com/taoquo/x-master/apps/macos/internal/storage/sqlite"
)

var clockBase = time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

// fakeClient answers from a script.
type fakeClient struct {
	mu       sync.Mutex
	requests []engine.TagRequest
	respond  func(engine.TagRequest) ([]engine.TagVerdict, error)
}

func (f *fakeClient) SuggestTags(_ context.Context, request engine.TagRequest) ([]engine.TagVerdict, error) {
	f.mu.Lock()
	f.requests = append(f.requests, request)
	respond := f.respond
	f.mu.Unlock()

	if respond == nil {
		return nil, nil
	}
	return respond(request)
}

func (f *fakeClient) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.requests)
}

// newService wires the real SQLite adapters, so the pipeline is exercised against
// the actual schema rather than a stub that agrees with it.
func newService(t *testing.T, client engine.LLMClient) (*Service, *sqlite.DB) {
	t.Helper()

	db, err := sqlite.Open(filepath.Join(t.TempDir(), "tagging.db"))
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
	}

	return New(repos, fixedClock{now: clockBase}, client), db
}

func seedBookmark(t *testing.T, db *sqlite.DB, id, handle, text string) {
	t.Helper()

	bookmark := domain.Bookmark{
		TweetID:      id,
		TweetURL:     "https://x.com/" + handle + "/status/" + id,
		AuthorName:   handle,
		AuthorHandle: handle,
		Text:         text,
		SavedAt:      clockBase,
		LastSeenAt:   clockBase,
		UpdatedAt:    clockBase,
	}
	if _, err := sqlite.NewBookmarkRepository(db).UpsertMany(context.Background(), []domain.Bookmark{bookmark}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}
}

// enabledSettings turns the feature on with the model configured.
func enabledSettings() engine.Settings {
	settings := engine.DefaultSettings()
	settings.Enabled = true
	settings.LLM.Enabled = true
	settings.LLM.Model = "test-model"
	settings.LLM.Mode = engine.VocabularyOpen
	settings.LLM.Concurrency = 0 // serial, so assertions are deterministic
	return settings
}

// TestRunPersistsHeuristicSuggestions is the integration the gate was missing: the
// engine's output must actually reach the database.
func TestRunPersistsHeuristicSuggestions(t *testing.T) {
	service, db := newService(t, &fakeClient{})
	ctx := context.Background()

	seedBookmark(t, db, "1", "ada", "learning about kubernetes")

	settings := enabledSettings()
	settings.LLM.Enabled = false // heuristics only
	settings.Heuristics = engine.HeuristicSettings{
		KeywordRules: []engine.KeywordTagRule{{Keyword: "kubernetes", TagName: "K8s"}},
	}

	summary, err := service.Run(ctx, settings)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if summary.Outcome != engine.OutcomeLLMDisabled {
		t.Errorf("outcome = %q, want %q", summary.Outcome, engine.OutcomeLLMDisabled)
	}
	if summary.SuggestedCount != 1 {
		t.Fatalf("SuggestedCount = %d, want 1", summary.SuggestedCount)
	}

	// The suggestion must be readable back out of SQLite.
	stored, err := sqlite.NewSuggestionRepository(db).List(ctx, domain.StatusPending)
	if err != nil {
		t.Fatalf("List suggestions: %v", err)
	}
	if len(stored) != 1 {
		t.Fatalf("stored %d suggestions, want 1", len(stored))
	}
	if stored[0].TagName != "K8s" || stored[0].BookmarkID != "1" {
		t.Errorf("stored suggestion = %+v", stored[0])
	}
	if stored[0].Source != domain.SourceKeyword {
		t.Errorf("source = %q, want %q", stored[0].Source, domain.SourceKeyword)
	}
}

// TestRunPersistsModelVerdictsAndLedger covers the model half end to end.
func TestRunPersistsModelVerdictsAndLedger(t *testing.T) {
	client := &fakeClient{respond: func(request engine.TagRequest) ([]engine.TagVerdict, error) {
		out := make([]engine.TagVerdict, 0, len(request.Batch))
		for _, bookmark := range request.Batch {
			out = append(out, engine.TagVerdict{
				BookmarkID: bookmark.TweetID, Tags: []engine.TagCandidate{{Name: "Topic", Confidence: 0.9}},
			})
		}
		return out, nil
	}}
	service, db := newService(t, client)
	ctx := context.Background()

	seedBookmark(t, db, "1", "ada", "first post")
	seedBookmark(t, db, "2", "grace", "second post")

	summary, err := service.Run(ctx, enabledSettings())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if summary.Outcome != engine.OutcomeOK {
		t.Fatalf("outcome = %q (%s), want %q", summary.Outcome, summary.Error, engine.OutcomeOK)
	}
	if summary.SuggestedCount != 2 {
		t.Errorf("SuggestedCount = %d, want 2", summary.SuggestedCount)
	}

	suggestions, err := sqlite.NewSuggestionRepository(db).List(ctx, domain.StatusPending)
	if err != nil {
		t.Fatalf("List suggestions: %v", err)
	}
	if len(suggestions) != 2 {
		t.Fatalf("stored %d suggestions, want 2", len(suggestions))
	}
	for _, suggestion := range suggestions {
		if suggestion.Source != domain.SourceLLM {
			t.Errorf("source = %q, want llm", suggestion.Source)
		}
	}

	// Every analysed bookmark gets a ledger row, including any the model skipped.
	records, err := sqlite.NewAnalysisRepository(db).List(ctx)
	if err != nil {
		t.Fatalf("List analysis: %v", err)
	}
	if len(records) != 2 {
		t.Errorf("ledger has %d rows, want 2", len(records))
	}
	for _, record := range records {
		if record.Outcome != domain.OutcomeTagged {
			t.Errorf("outcome = %q, want tagged", record.Outcome)
		}
		if record.Model != "test-model" {
			t.Errorf("model = %q", record.Model)
		}
	}
}

// TestSecondRunSkipsAnalysedBookmarks is the idempotency the ledger exists for:
// without it every run re-asks the same questions and burns the user's quota.
func TestSecondRunSkipsAnalysedBookmarks(t *testing.T) {
	client := &fakeClient{respond: func(request engine.TagRequest) ([]engine.TagVerdict, error) {
		out := make([]engine.TagVerdict, 0, len(request.Batch))
		for _, bookmark := range request.Batch {
			out = append(out, engine.TagVerdict{BookmarkID: bookmark.TweetID, Tags: []engine.TagCandidate{{Name: "Topic", Confidence: 0.9}}})
		}
		return out, nil
	}}
	service, db := newService(t, client)
	ctx := context.Background()

	seedBookmark(t, db, "1", "ada", "a post that will be analysed")

	if _, err := service.Run(ctx, enabledSettings()); err != nil {
		t.Fatalf("first Run: %v", err)
	}
	if client.callCount() != 1 {
		t.Fatalf("first run made %d calls, want 1", client.callCount())
	}

	second, err := service.Run(ctx, enabledSettings())
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if client.callCount() != 1 {
		t.Errorf("second run made %d calls, want 0 more: the ledger already answered", client.callCount())
	}
	if second.Outcome != engine.OutcomeNoCandidates {
		t.Errorf("second outcome = %q, want %q", second.Outcome, engine.OutcomeNoCandidates)
	}
}

// TestConcurrentRunsAreRejected covers defect S6's remaining half: two runs over
// the same library would double-charge the budget and double-write suggestions.
func TestConcurrentRunsAreRejected(t *testing.T) {
	release := make(chan struct{})
	client := &fakeClient{respond: func(request engine.TagRequest) ([]engine.TagVerdict, error) {
		// Hold the first run inside the model call.
		<-release
		return nil, nil
	}}
	service, db := newService(t, client)
	ctx := context.Background()
	seedBookmark(t, db, "1", "ada", "x")

	settings := enabledSettings()
	settings.LLM.Concurrency = 0

	var wg sync.WaitGroup
	wg.Add(1)
	var firstErr error
	go func() {
		defer wg.Done()
		_, firstErr = service.Run(ctx, settings)
	}()

	// Wait until the first run is genuinely in flight.
	deadline := time.Now().Add(2 * time.Second)
	for !service.IsRunning() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !service.IsRunning() {
		close(release)
		wg.Wait()
		t.Fatal("the first run never started")
	}

	// The second run must be refused, not queued.
	_, err := service.Run(ctx, settings)
	if !errors.Is(err, ErrAlreadyRunning) {
		t.Errorf("second Run error = %v, want ErrAlreadyRunning", err)
	}

	close(release)
	wg.Wait()
	if firstErr != nil {
		t.Errorf("first Run: %v", firstErr)
	}
}

// TestRequestCancelStopsAtAWaveBoundary checks that a cancel keeps completed work.
func TestRequestCancelStopsAtAWaveBoundary(t *testing.T) {
	client := &fakeClient{}
	service, db := newService(t, client)
	ctx := context.Background()

	for _, id := range []string{"a", "b", "c", "d"} {
		seedBookmark(t, db, id, "ada", "post "+id)
	}

	settings := enabledSettings()
	settings.LLM.BatchSize = 1

	// Cancel from inside the first batch, so the boundary check stops the rest.
	client.respond = func(request engine.TagRequest) ([]engine.TagVerdict, error) {
		service.RequestCancel()
		out := make([]engine.TagVerdict, 0, len(request.Batch))
		for _, bookmark := range request.Batch {
			out = append(out, engine.TagVerdict{BookmarkID: bookmark.TweetID, Tags: []engine.TagCandidate{{Name: "Topic", Confidence: 0.9}}})
		}
		return out, nil
	}

	summary, err := service.Run(ctx, settings)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if summary.Outcome != engine.OutcomeCancelled {
		t.Errorf("outcome = %q, want %q", summary.Outcome, engine.OutcomeCancelled)
	}
	if summary.SuggestedCount == 0 {
		t.Error("work finished before the cancel must be kept")
	}
	if summary.UnprocessedCount == 0 {
		t.Error("UnprocessedCount must show what is left")
	}
}

// TestHealStaleRun is the startup self-heal: a run that was in flight when the app
// stopped cannot still be running, and leaving its state would lie to the user.
func TestHealStaleRun(t *testing.T) {
	service, db := newService(t, &fakeClient{})
	ctx := context.Background()

	// Leave a persisted run state behind, as a crash mid-run would.
	state := engine.Begin(clockBase, engine.BeginInput{Phase: engine.PhaseLLM, Total: 10})
	encoded := `{"RunID":"` + state.RunID + `","Phase":"llm","Total":10,"Processed":3}`
	if err := sqlite.NewSettingsRepository(db).Set(ctx, runStateKey, encoded, clockBase); err != nil {
		t.Fatalf("seed run state: %v", err)
	}

	healed, err := service.HealStaleRun(ctx)
	if err != nil {
		t.Fatalf("HealStaleRun: %v", err)
	}
	if !healed {
		t.Fatal("a persisted run state must be healed")
	}

	// Nothing is left claiming to be in flight.
	if service.IsRunning() {
		t.Error("no run may be reported as running after healing")
	}
	if service.CurrentRun() != nil {
		t.Error("the live state must be cleared")
	}

	_, present, err := sqlite.NewSettingsRepository(db).Get(ctx, runStateKey)
	if err != nil {
		t.Fatalf("read run state: %v", err)
	}
	if present {
		t.Error("the persisted run state must be removed")
	}

	// The user is told why, rather than shown an empty panel.
	summary, found, err := service.LastSummary(ctx)
	if err != nil {
		t.Fatalf("LastSummary: %v", err)
	}
	if !found {
		t.Fatal("healing must record a summary")
	}
	if summary.Outcome != engine.OutcomeInterrupted {
		t.Errorf("outcome = %q, want %q", summary.Outcome, engine.OutcomeInterrupted)
	}
	// The counts come from the state that died, because it is the only record of how far
	// the run got. Copying the previous summary's counts reported a run that never started.
	if summary.CandidateCount != 10 || summary.LLMProcessedCount != 3 || summary.UnprocessedCount != 7 {
		t.Errorf("counts = candidate %d, processed %d, unprocessed %d; want 10, 3, 7",
			summary.CandidateCount, summary.LLMProcessedCount, summary.UnprocessedCount)
	}
}

// TestHealingACancelledRunKeepsItCancelled covers the outcome the heal used to overwrite.
//
// A run the user stopped is not one the app interrupted: the first must not auto-continue,
// and the second may. Recording both as "interrupted" lost the user's decision.
func TestHealingACancelledRunKeepsItCancelled(t *testing.T) {
	service, db := newService(t, &fakeClient{})
	ctx := context.Background()

	encoded := `{"RunID":"run-cancelled","Phase":"llm","Total":10,"Processed":3,"CancelRequested":true}`
	if err := sqlite.NewSettingsRepository(db).Set(ctx, runStateKey, encoded, clockBase); err != nil {
		t.Fatalf("seed run state: %v", err)
	}

	healed, err := service.HealStaleRun(ctx)
	if err != nil || !healed {
		t.Fatalf("HealStaleRun = %v, %v", healed, err)
	}

	summary, found, err := service.LastSummary(ctx)
	if err != nil || !found {
		t.Fatalf("LastSummary = found %v, err %v", found, err)
	}
	if summary.Outcome != engine.OutcomeCancelled {
		t.Errorf("outcome = %q, want %q: the user asked this run to stop", summary.Outcome, engine.OutcomeCancelled)
	}
	if summary.Error != "" {
		t.Errorf("a cancelled run reports an error %q", summary.Error)
	}
	if summary.UnprocessedCount != 7 {
		t.Errorf("unprocessed = %d, want 7", summary.UnprocessedCount)
	}
}

func TestHealStaleRunIsANoOpWhenNothingWasInFlight(t *testing.T) {
	service, _ := newService(t, &fakeClient{})

	healed, err := service.HealStaleRun(context.Background())
	if err != nil {
		t.Fatalf("HealStaleRun: %v", err)
	}
	if healed {
		t.Error("nothing to heal must report false")
	}
}

func TestSettingsRoundTrip(t *testing.T) {
	service, _ := newService(t, &fakeClient{})
	ctx := context.Background()

	// A fresh install reads the defaults.
	settings, err := service.LoadSettings(ctx)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if settings.Enabled {
		t.Error("auto-tagging must default to off")
	}

	settings.Enabled = true
	settings.LLM.Model = "gpt-4o-mini"
	settings.LLM.Mode = engine.VocabularyGuarded
	settings.BlockedTagKeys = []string{"spam"}
	if err := service.SaveSettings(ctx, settings); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}

	loaded, err := service.LoadSettings(ctx)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if !loaded.Enabled || loaded.LLM.Model != "gpt-4o-mini" || loaded.LLM.Mode != engine.VocabularyGuarded {
		t.Errorf("loaded = %+v", loaded)
	}
	if len(loaded.BlockedTagKeys) != 1 || loaded.BlockedTagKeys[0] != "spam" {
		t.Errorf("BlockedTagKeys = %v", loaded.BlockedTagKeys)
	}
}

// TestUnreadableSettingsFallBackToDefaults covers an upgrade: a blob written by
// another build must not brick the feature.
func TestUnreadableSettingsFallBackToDefaults(t *testing.T) {
	service, db := newService(t, &fakeClient{})
	ctx := context.Background()

	if err := sqlite.NewSettingsRepository(db).Set(ctx, settingsKey, "{not json", clockBase); err != nil {
		t.Fatalf("seed settings: %v", err)
	}

	settings, err := service.LoadSettings(ctx)
	if err != nil {
		t.Fatalf("LoadSettings must not fail on an unreadable blob: %v", err)
	}
	if settings.LLM.Mode != engine.VocabularyGuarded {
		t.Errorf("mode = %q, want the default", settings.LLM.Mode)
	}
}

// TestCancelWithoutARunReportsNothing keeps the UI from believing it stopped
// something that never started.
func TestCancelWithoutARunReportsNothing(t *testing.T) {
	service, _ := newService(t, &fakeClient{})

	if service.RequestCancel() {
		t.Error("cancelling with no run must report false")
	}
}

// TestEnsureInboxAfterSync covers the step that must follow any bookmark insert,
// without which a new bookmark is invisible in every list view.
func TestEnsureInboxAfterSync(t *testing.T) {
	service, db := newService(t, &fakeClient{})
	ctx := context.Background()

	seedBookmark(t, db, "1", "ada", "x")
	seedBookmark(t, db, "2", "grace", "y")

	moved, err := service.EnsureInboxAfterSync(ctx)
	if err != nil {
		t.Fatalf("EnsureInboxAfterSync: %v", err)
	}
	if moved != 2 {
		t.Errorf("assigned %d bookmarks, want 2", moved)
	}

	// Idempotent.
	again, err := service.EnsureInboxAfterSync(ctx)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if again != 0 {
		t.Errorf("second call assigned %d, want 0", again)
	}
}

// TestRunRecordsASummarySoThePanelCanExplainSilence pins that every outcome,
// including a disabled feature, leaves a report.
func TestRunRecordsASummarySoThePanelCanExplainSilence(t *testing.T) {
	service, _ := newService(t, &fakeClient{})
	ctx := context.Background()

	settings := engine.DefaultSettings() // disabled
	if _, err := service.Run(ctx, settings); err != nil {
		t.Fatalf("Run: %v", err)
	}

	summary, found, err := service.LastSummary(ctx)
	if err != nil {
		t.Fatalf("LastSummary: %v", err)
	}
	if !found {
		t.Fatal("even a disabled run must leave a summary")
	}
	if summary.Outcome != engine.OutcomeDisabled {
		t.Errorf("outcome = %q, want %q", summary.Outcome, engine.OutcomeDisabled)
	}
}

// TestRateLimitDegradesGracefully checks a throttled run still records what it
// managed and does not fail outright.
func TestRateLimitDegradesGracefully(t *testing.T) {
	client := &fakeClient{respond: func(request engine.TagRequest) ([]engine.TagVerdict, error) {
		return nil, &llm.LlmError{Kind: llm.KindRateLimit, Message: "429"}
	}}
	service, db := newService(t, client)
	ctx := context.Background()
	seedBookmark(t, db, "1", "ada", "x")

	summary, err := service.Run(ctx, enabledSettings())
	if err != nil {
		t.Fatalf("Run must not fail the caller on a rate limit: %v", err)
	}
	if summary.LLMFailedBatchCount == 0 {
		t.Error("the failed batch must be counted")
	}
	if summary.Outcome != engine.OutcomeFailed {
		t.Errorf("outcome = %q, want %q when every batch failed", summary.Outcome, engine.OutcomeFailed)
	}
}

// TestOutcomeIsRecordedEvenWhenTheContextIsCancelled is the same defect class that
// the sync path had: a run whose bookkeeping is written through a cancelled
// context leaves its state set forever and records no report, so the user sees a
// run that never finishes and nothing explaining why.
func TestOutcomeIsRecordedEvenWhenTheContextIsCancelled(t *testing.T) {
	service, db := newService(t, &fakeClient{})
	ctx := context.Background()
	seedBookmark(t, db, "1", "ada", "a post")

	settings := engine.DefaultSettings() // disabled, so the run ends immediately

	cancelled, cancel := context.WithCancel(ctx)
	cancel()

	summary, err := service.Run(cancelled, settings)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if summary.Outcome != engine.OutcomeDisabled {
		t.Errorf("outcome = %q, want %q", summary.Outcome, engine.OutcomeDisabled)
	}

	// The state must not be left claiming a run is in flight.
	if service.IsRunning() {
		t.Error("a finished run must not be reported as running")
	}
	if service.CurrentRun() != nil {
		t.Error("the live state must be cleared even though the context was cancelled")
	}

	// And the report must exist, or the panel has nothing to show.
	if _, present, err := sqlite.NewSettingsRepository(db).Get(ctx, runStateKey); err != nil {
		t.Fatalf("read run state: %v", err)
	} else if present {
		t.Error("the persisted run state must be removed")
	}

	recorded, found, err := service.LastSummary(ctx)
	if err != nil {
		t.Fatalf("LastSummary: %v", err)
	}
	if !found {
		t.Fatal("the summary must be recorded even though the context was cancelled")
	}
	if recorded.Outcome != engine.OutcomeDisabled {
		t.Errorf("recorded outcome = %q", recorded.Outcome)
	}
}

// TestAFailedRunStillLeavesAReport covers the other side of the same property: a
// run that cannot even start must still record why, or the panel shows nothing and
// the user has no way to tell "never ran" from "ran and failed".
func TestAFailedRunStillLeavesAReport(t *testing.T) {
	service, db := newService(t, &fakeClient{})
	ctx := context.Background()
	seedBookmark(t, db, "1", "ada", "a post")

	// A context that is already cancelled: the run cannot load its snapshot.
	cancelled, cancel := context.WithCancel(ctx)
	cancel()

	_, err := service.Run(cancelled, enabledSettings())
	if err == nil {
		t.Fatal("a run with a cancelled context must report that it could not start")
	}

	// Nothing is left claiming to be in flight.
	if service.IsRunning() {
		t.Error("a run that never started must not be reported as running")
	}
	if service.CurrentRun() != nil {
		t.Error("no state must be left behind")
	}

	// And the failure is recorded, so the user can be told what happened.
	recorded, found, err := service.LastSummary(ctx)
	if err != nil {
		t.Fatalf("LastSummary: %v", err)
	}
	if !found {
		t.Fatal("a failed run must still record a summary")
	}
	if recorded.Outcome != engine.OutcomeFailed {
		t.Errorf("recorded outcome = %q, want %q", recorded.Outcome, engine.OutcomeFailed)
	}
	if recorded.Error == "" {
		t.Error("the summary must carry the reason")
	}
}

// TestCancellingMidRunKeepsWorkAndRecordsTheOutcome is the realistic cancellation:
// the user presses stop while the run is working.
func TestCancellingMidRunKeepsWorkAndRecordsTheOutcome(t *testing.T) {
	client := &fakeClient{}
	service, db := newService(t, client)
	ctx := context.Background()

	for _, id := range []string{"a", "b", "c", "d"} {
		seedBookmark(t, db, id, "ada", "post "+id)
	}

	settings := enabledSettings()
	settings.LLM.BatchSize = 1

	client.respond = func(request engine.TagRequest) ([]engine.TagVerdict, error) {
		service.RequestCancel()
		out := make([]engine.TagVerdict, 0, len(request.Batch))
		for _, bookmark := range request.Batch {
			out = append(out, engine.TagVerdict{BookmarkID: bookmark.TweetID, Tags: []engine.TagCandidate{{Name: "Topic", Confidence: 0.9}}})
		}
		return out, nil
	}

	summary, err := service.Run(ctx, settings)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if summary.Outcome != engine.OutcomeCancelled {
		t.Errorf("outcome = %q, want %q", summary.Outcome, engine.OutcomeCancelled)
	}

	// The work finished before the cancel is kept.
	if summary.SuggestedCount == 0 {
		t.Error("work finished before the cancel must be kept")
	}
	if summary.UnprocessedCount == 0 {
		t.Error("what is left must be reported")
	}

	// The cancelled run is not left hanging, and its outcome is recorded.
	if service.IsRunning() {
		t.Error("a cancelled run must not be reported as running")
	}
	recorded, found, err := service.LastSummary(ctx)
	if err != nil || !found {
		t.Fatalf("the cancelled run must record a summary: found=%v err=%v", found, err)
	}
	if recorded.Outcome != engine.OutcomeCancelled {
		t.Errorf("recorded outcome = %q, want %q", recorded.Outcome, engine.OutcomeCancelled)
	}

	// And the suggestions it did make are in the database.
	suggestions, err := sqlite.NewSuggestionRepository(db).List(ctx, domain.StatusPending)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(suggestions) == 0 {
		t.Error("the suggestions made before the cancel must be stored")
	}
}

// TestProviderClientIsBuiltPerRun covers the reason the factory exists: a key entered
// after launch must apply without a relaunch.
func TestProviderClientIsBuiltPerRun(t *testing.T) {
	service, db := newService(t, nil)
	ctx := context.Background()
	seedBookmark(t, db, "1", "ada", "a post about kubernetes")

	built := 0
	var lastProvider string
	service.UseProviderClient(func(settings engine.Settings) (engine.LLMClient, error) {
		built++
		lastProvider = settings.LLM.ProviderID
		return &fakeClient{}, nil
	})

	settings := enabledSettings()
	settings.LLM.Enabled = true
	settings.LLM.ProviderID = "anthropic"

	if _, err := service.Run(ctx, settings); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if built != 1 {
		t.Errorf("the factory ran %d times, want 1", built)
	}
	if lastProvider != "anthropic" {
		t.Errorf("provider = %q, want the one from the run's settings", lastProvider)
	}

	// A second run rebuilds with the settings it is given, which is what makes a
	// changed provider take effect without a relaunch.
	settings.LLM.ProviderID = "google"
	if _, err := service.Run(ctx, settings); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if built != 2 {
		t.Errorf("the factory ran %d times, want 2", built)
	}
	if lastProvider != "google" {
		t.Errorf("provider = %q, want the changed one", lastProvider)
	}
}

// TestAFailedClientBuildIsVisible keeps a construction failure from looking like a
// run that simply found nothing.
func TestAFailedClientBuildIsVisible(t *testing.T) {
	service, db := newService(t, nil)
	ctx := context.Background()
	seedBookmark(t, db, "1", "ada", "post")

	service.UseProviderClient(func(engine.Settings) (engine.LLMClient, error) {
		return nil, errors.New("no API key stored for anthropic")
	})

	settings := enabledSettings()
	settings.LLM.Enabled = true

	summary, err := service.Run(ctx, settings)
	if err == nil {
		t.Fatal("a run whose client could not be built must report a failure")
	}
	// The run carries the real reason, not a generic "no client configured": the
	// user needs to know it was the key, because that is what they can fix.
	if !strings.Contains(summary.Error, "no API key") {
		t.Errorf("the run must report the real reason, got %q", summary.Error)
	}

	// And the caller can ask before or after, since the failure happens while the
	// run is being assembled.
	if err := service.ClientError(); err == nil {
		t.Error("a failed client build must be visible to the caller")
	} else if !strings.Contains(err.Error(), "no API key") {
		t.Errorf("the error must be the real one: %v", err)
	}
}

// TestBlockingATagKeyKeepsTheNameGone covers the third of the three steps the extension's
// tagActions names: deleting a tag drops the tag but does not stop the name coming back,
// because the rule engine and the LLM step both work from names.
func TestBlockingATagKeyKeepsTheNameGone(t *testing.T) {
	service, _ := newService(t, nil)
	ctx := context.Background()

	if got := blockedKeys(t, service, ctx); len(got) != 0 {
		t.Fatalf("a fresh install blocks %v", got)
	}

	if err := service.BlockTagKey(ctx, "Rust"); err != nil {
		t.Fatalf("BlockTagKey: %v", err)
	}
	// The canonical key, not the display name: a different case is the same tag.
	if got := blockedKeys(t, service, ctx); len(got) != 1 || got[0] != "rust" {
		t.Errorf("blocked %v, want the canonical key rust", got)
	}

	// Deleting the same tag twice must not grow the list.
	if err := service.BlockTagKey(ctx, "rust"); err != nil {
		t.Fatalf("BlockTagKey again: %v", err)
	}
	if got := blockedKeys(t, service, ctx); len(got) != 1 {
		t.Errorf("blocking twice left %v", got)
	}
}

// TestRecreatingATagClearsTheBlock covers the extension's createTagUnblocked: a user who
// types the name back out has answered the question the block was holding.
func TestRecreatingATagClearsTheBlock(t *testing.T) {
	service, _ := newService(t, nil)
	ctx := context.Background()

	for _, name := range []string{"rust", "go"} {
		if err := service.BlockTagKey(ctx, name); err != nil {
			t.Fatalf("BlockTagKey(%q): %v", name, err)
		}
	}
	if err := service.UnblockTagKey(ctx, "Rust"); err != nil {
		t.Fatalf("UnblockTagKey: %v", err)
	}

	// Only the named key goes, and an unblocked name is not an error.
	got := blockedKeys(t, service, ctx)
	if len(got) != 1 || got[0] != "go" {
		t.Errorf("after unblocking rust the list is %v, want [go]", got)
	}
	if err := service.UnblockTagKey(ctx, "nothing-blocks-this"); err != nil {
		t.Errorf("unblocking an unblocked name: %v", err)
	}
}

// TestClearingTheBlockList covers the settings panel's button.
func TestClearingTheBlockList(t *testing.T) {
	service, _ := newService(t, nil)
	ctx := context.Background()

	if err := service.ClearBlockedTagKeys(ctx); err != nil {
		t.Fatalf("clearing an empty list: %v", err)
	}
	for _, name := range []string{"rust", "go", "ada"} {
		if err := service.BlockTagKey(ctx, name); err != nil {
			t.Fatalf("BlockTagKey(%q): %v", name, err)
		}
	}
	if err := service.ClearBlockedTagKeys(ctx); err != nil {
		t.Fatalf("ClearBlockedTagKeys: %v", err)
	}
	if got := blockedKeys(t, service, ctx); len(got) != 0 {
		t.Errorf("the list survived clearing: %v", got)
	}
}

// blockedKeys is the blocked list as it stands on disk.
func blockedKeys(t *testing.T, service *Service, ctx context.Context) []string {
	t.Helper()
	settings, err := service.LoadSettings(ctx)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	return settings.BlockedTagKeys
}

// TestAutoTagContinueCountSurvivesTheStore covers the field the continuation bound reads.
//
// The count has to outlive the process, not just the run: a chain that was interrupted
// mid-way must not get a fresh budget on the next start, or a run that keeps dying
// restarts forever.
func TestAutoTagContinueCountSurvivesTheStore(t *testing.T) {
	service, _ := newService(t, nil)
	ctx := context.Background()

	for i := 1; i <= 3; i++ {
		settings, err := service.LoadSettings(ctx)
		if err != nil {
			t.Fatalf("LoadSettings: %v", err)
		}
		settings.AutoTagContinueCount = i
		if err := service.SaveSettings(ctx, settings); err != nil {
			t.Fatalf("SaveSettings: %v", err)
		}
	}

	settings, err := service.LoadSettings(ctx)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if settings.AutoTagContinueCount != 3 {
		t.Errorf("the count came back as %d, want 3", settings.AutoTagContinueCount)
	}
}

// TestLoadSettingsRepairsEveryKindOfBadValue is the point of normalization.
//
// A test that feeds good values through proves nothing: the code path would pass with no
// normalizer at all. Each case below is a value this build must not accept, and each one
// was written straight into the store the way a newer build, a hand-edited database or a
// restored backup would leave it.
func TestLoadSettingsRepairsEveryKindOfBadValue(t *testing.T) {
	cases := []struct {
		name  string
		blob  string
		check func(t *testing.T, got engine.Settings)
	}{
		{
			name: "a batch size above the range",
			blob: `{"autoTagContinueCount":0,"llm":{"batchSize":9999}}`,
			check: func(t *testing.T, got engine.Settings) {
				if got.LLM.BatchSize != engine.MaxBatchSize {
					t.Errorf("batchSize = %d, want the ceiling %d", got.LLM.BatchSize, engine.MaxBatchSize)
				}
			},
		},
		{
			name: "a negative batch size falls back rather than clamping to one",
			blob: `{"autoTagContinueCount":0,"llm":{"batchSize":-4}}`,
			check: func(t *testing.T, got engine.Settings) {
				if got.LLM.BatchSize != engine.DefaultBatchSize {
					t.Errorf("batchSize = %d, want the default %d: a non-positive value falls back",
						got.LLM.BatchSize, engine.DefaultBatchSize)
				}
			},
		},
		{
			name: "a concurrency above the ceiling",
			blob: `{"autoTagContinueCount":0,"llm":{"concurrency":99}}`,
			check: func(t *testing.T, got engine.Settings) {
				if got.LLM.Concurrency != engine.MaxConcurrency {
					t.Errorf("concurrency = %d, want %d", got.LLM.Concurrency, engine.MaxConcurrency)
				}
			},
		},
		{
			name: "a per-run budget above the range",
			blob: `{"autoTagContinueCount":0,"llm":{"maxPerRun":100000}}`,
			check: func(t *testing.T, got engine.Settings) {
				if got.LLM.MaxPerRun != engine.MaxMaxBookmarksPerRun {
					t.Errorf("maxPerRun = %d, want %d", got.LLM.MaxPerRun, engine.MaxMaxBookmarksPerRun)
				}
			},
		},
		{
			name: "an unknown vocabulary mode",
			blob: `{"autoTagContinueCount":0,"llm":{"mode":"yolo"}}`,
			check: func(t *testing.T, got engine.Settings) {
				if got.LLM.Mode != engine.VocabularyGuarded {
					t.Errorf("mode = %q, want %q", got.LLM.Mode, engine.VocabularyGuarded)
				}
			},
		},
		{
			name: "a keyword rule missing half of itself",
			blob: `{"autoTagContinueCount":0,"heuristics":{"keywordRules":[` +
				`{"keyword":"rust","tagName":"Rust"},{"keyword":"  ","tagName":"Go"},` +
				`{"keyword":"go","tagName":"   "}]}}`,
			check: func(t *testing.T, got engine.Settings) {
				if len(got.Heuristics.KeywordRules) != 1 {
					t.Fatalf("kept %d rules, want 1: a rule missing either half can never fire",
						len(got.Heuristics.KeywordRules))
				}
				if got.Heuristics.KeywordRules[0].Keyword != "rust" {
					t.Errorf("kept %q", got.Heuristics.KeywordRules[0].Keyword)
				}
			},
		},
		{
			name: "a repeated blocked key",
			blob: `{"autoTagContinueCount":0,"blockedTagKeys":["rust","go","rust","",  "go"]}`,
			check: func(t *testing.T, got engine.Settings) {
				if len(got.BlockedTagKeys) != 2 {
					t.Errorf("blocked keys = %v, want the two distinct ones", got.BlockedTagKeys)
				}
			},
		},
		{
			name: "a negative continuation count",
			blob: `{"autoTagContinueCount":-7}`,
			check: func(t *testing.T, got engine.Settings) {
				if got.AutoTagContinueCount != 0 {
					t.Errorf("autoTagContinueCount = %d, want 0: a negative budget would make "+
						"the continuation bound refuse every run", got.AutoTagContinueCount)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service, db := newService(t, nil)
			ctx := context.Background()

			// Written straight to the store, the way the value would really arrive.
			repo := sqlite.NewSettingsRepository(db)
			if err := repo.Set(ctx, "autotag.settings", tc.blob, time.Now()); err != nil {
				t.Fatalf("Set: %v", err)
			}
			got, err := service.LoadSettings(ctx)
			if err != nil {
				t.Fatalf("LoadSettings: %v", err)
			}
			tc.check(t, got)
		})
	}
}

// TestLoadSettingsFallsBackOnAnUndecodableBlob is the other layer.
//
// The two are separate on purpose: a blob that will not decode at all falls back to the
// defaults, while a blob that decodes with one bad field is repaired. Collapsing them
// would throw away a whole configuration because one field was unknown.
func TestLoadSettingsFallsBackOnAnUndecodableBlob(t *testing.T) {
	service, db := newService(t, nil)
	ctx := context.Background()

	repo := sqlite.NewSettingsRepository(db)
	for _, blob := range []string{"{not json", "", "null"} {
		if err := repo.Set(ctx, "autotag.settings", blob, time.Now()); err != nil {
			t.Fatalf("Set: %v", err)
		}
		got, err := service.LoadSettings(ctx)
		if err != nil {
			t.Fatalf("LoadSettings(%q): %v", blob, err)
		}
		if got.LLM.BatchSize != engine.DefaultBatchSize {
			t.Errorf("blob %q produced batchSize %d, want the default", blob, got.LLM.BatchSize)
		}
	}
}

// TestCapabilityFieldsRoundTripThroughTheStore is the criterion this was added for.
//
// The extension keeps capabilityOverrides and learnedCapabilities in its settings and
// passes both into every capability resolution. Nothing writes them on either side -- there
// is no editor -- so the only thing that can carry them is a backup. Dropping them was the
// difference: an imported workspace would silently lose a user's model answers.
func TestCapabilityFieldsRoundTripThroughTheStore(t *testing.T) {
	service, _ := newService(t, nil)
	ctx := context.Background()

	window := 128000
	settings, err := service.LoadSettings(ctx)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	settings.LLM.CapabilityOverrides = &llm.ModelCapabilities{
		JsonStrategy:    llm.JsonObject,
		Temperature:     true,
		MaxOutputTokens: &window,
	}
	settings.LLM.LearnedCapabilities = map[string]llm.ModelCapabilities{
		llm.LearnedCapabilityKey("openai", "gpt-4o-mini"): {JsonStrategy: llm.JsonSchemaStrict},
	}
	if err := service.SaveSettings(ctx, settings); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}

	got, err := service.LoadSettings(ctx)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if got.LLM.CapabilityOverrides == nil {
		t.Fatal("the overrides did not survive the round trip")
	}
	if got.LLM.CapabilityOverrides.JsonStrategy != llm.JsonObject ||
		!got.LLM.CapabilityOverrides.Temperature {
		t.Errorf("overrides = %+v", got.LLM.CapabilityOverrides)
	}
	if got.LLM.CapabilityOverrides.MaxOutputTokens == nil ||
		*got.LLM.CapabilityOverrides.MaxOutputTokens != window {
		t.Errorf("maxOutputTokens = %v, want %d", got.LLM.CapabilityOverrides.MaxOutputTokens, window)
	}
	learned, ok := got.LLM.LearnedCapabilities[llm.LearnedCapabilityKey("openai", "gpt-4o-mini")]
	if !ok {
		t.Fatalf("the learned entry did not survive: %v", got.LLM.LearnedCapabilities)
	}
	if learned.JsonStrategy != llm.JsonSchemaStrict {
		t.Errorf("learned = %+v", learned)
	}
}
