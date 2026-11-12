package autotag

import (
	"context"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/taoquo/x-master/apps/macos/internal/domain"
	"github.com/taoquo/x-master/apps/macos/internal/llm"
)

// fakeClient records requests and answers from a script.
type fakeClient struct {
	mu       sync.Mutex
	requests []TagRequest
	respond  func(TagRequest) ([]TagVerdict, error)
}

func (f *fakeClient) SuggestTags(_ context.Context, request TagRequest) ([]TagVerdict, error) {
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

func (f *fakeClient) lastRequest() TagRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.requests) == 0 {
		return TagRequest{}
	}
	return f.requests[len(f.requests)-1]
}

// harness is an in-memory set of collaborators.
type harness struct {
	deps        Deps
	client      *fakeClient
	saved       []*RunState
	cleared     []string
	summary     *RunSummary
	suggestions []domain.TagSuggestion
	analysis    []domain.LlmAnalysis
	clock       time.Time
}

func newHarness(t *testing.T, snapshot Snapshot) *harness {
	t.Helper()

	h := &harness{clock: base}
	h.client = &fakeClient{}

	h.deps = Deps{
		Settings: DefaultSettings(),
		Snapshot: func(context.Context) (Snapshot, error) { return snapshot, nil },
		// Every write fake honours its context.
		//
		// They used to ignore it, which made the harness unable to observe the defect
		// the bookkeeping context exists for: a write through a cancelled context fails
		// in production, so the run stays marked in flight. A fake that always succeeds
		// records the attempt and cannot tell a write that landed from one that did not
		// -- which is why changing bookkeepingContext back to the run's own context
		// survived the whole suite.
		InsertSuggestions: func(ctx context.Context, suggestions []domain.TagSuggestion) (int, error) {
			if err := ctx.Err(); err != nil {
				return 0, err
			}
			h.suggestions = append(h.suggestions, suggestions...)
			return len(suggestions), nil
		},
		PersistAnalysis: func(ctx context.Context, records []domain.LlmAnalysis) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			h.analysis = append(h.analysis, records...)
			return nil
		},
		Client: h.client,
		SaveRunState: func(ctx context.Context, state *RunState) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			h.saved = append(h.saved, state)
			return nil
		},
		ClearRunState: func(ctx context.Context, runID string) (bool, error) {
			if err := ctx.Err(); err != nil {
				return false, err
			}
			h.cleared = append(h.cleared, runID)
			return true, nil
		},
		RecordSummary: func(ctx context.Context, summary RunSummary) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			h.summary = &summary
			return nil
		},
		Now: func() time.Time { return h.clock },
		// Serial by default, so batches do not race in assertions.
		RunBudget: time.Minute, BatchBudget: time.Minute,
	}
	h.deps.Settings.Enabled = true
	h.deps.Settings.LLM.Enabled = true
	h.deps.Settings.LLM.Model = "test-model"
	h.deps.Settings.LLM.Concurrency = 0 // unset means serial

	return h
}

func (h *harness) run(t *testing.T) RunSummary {
	t.Helper()
	summary, err := Run(context.Background(), h.deps)
	if err != nil && summary.Outcome != OutcomeFailed {
		t.Fatalf("Run returned an unexpected error: %v", err)
	}
	return summary
}

func posts(n int, handle string) []domain.Bookmark {
	out := make([]domain.Bookmark, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, domain.Bookmark{
			TweetID: string(rune('a'+i)) + handle, AuthorHandle: handle,
			Text: "post " + string(rune('a'+i)), SavedAt: base, LastSeenAt: base, UpdatedAt: base,
		})
	}
	return out
}

func verdictsFor(batch []domain.Bookmark, tagName string) []TagVerdict {
	out := make([]TagVerdict, 0, len(batch))
	for _, bookmark := range batch {
		out = append(out, TagVerdict{BookmarkID: bookmark.TweetID, Tags: []TagCandidate{{Name: tagName, Confidence: 0.8}}})
	}
	return out
}

// --- outcome gates ---

func TestPipelineDisabled(t *testing.T) {
	h := newHarness(t, Snapshot{})
	h.deps.Settings.Enabled = false

	summary := h.run(t)

	if summary.Outcome != OutcomeDisabled {
		t.Errorf("outcome = %q, want %q", summary.Outcome, OutcomeDisabled)
	}
	if h.client.callCount() != 0 {
		t.Error("a disabled pipeline must not call the model")
	}
	if len(h.saved) != 0 {
		t.Error("a disabled run must not publish a run state")
	}
	// It still records the outcome, so the panel can explain the silence.
	if h.summary == nil || h.summary.Outcome != OutcomeDisabled {
		t.Error("the disabled outcome must still be reported")
	}
}

// TestPipelineHeuristicsRunWithoutTheModel covers the cheap half: rule-based
// tagging works with no provider configured at all.
func TestPipelineHeuristicsRunWithoutTheModel(t *testing.T) {
	snapshot := Snapshot{
		Bookmarks: []domain.Bookmark{{
			TweetID: "1", AuthorHandle: "ada", Text: "all about kubernetes",
			SavedAt: base, LastSeenAt: base, UpdatedAt: base,
		}},
	}
	h := newHarness(t, snapshot)
	h.deps.Settings.LLM.Enabled = false
	h.deps.Settings.Heuristics = HeuristicSettings{
		KeywordRules: []KeywordTagRule{{Keyword: "kubernetes", TagName: "K8s"}},
	}

	summary := h.run(t)

	if summary.Outcome != OutcomeLLMDisabled {
		t.Errorf("outcome = %q, want %q", summary.Outcome, OutcomeLLMDisabled)
	}
	if len(h.suggestions) != 1 || h.suggestions[0].TagName != "K8s" {
		t.Fatalf("heuristic suggestions = %+v, want the K8s one", h.suggestions)
	}
	if h.client.callCount() != 0 {
		t.Error("the model must not be called when it is disabled")
	}
	if h.summary.SuggestedCount != 1 {
		t.Errorf("SuggestedCount = %d, want 1", h.summary.SuggestedCount)
	}
}

func TestPipelineMissingAPIKey(t *testing.T) {
	snapshot := Snapshot{Bookmarks: posts(1, "ada")}
	h := newHarness(t, snapshot)
	h.deps.Settings.LLM.NeedsAPIKey = true
	h.deps.Settings.LLM.APIKey = ""

	summary := h.run(t)

	if summary.Outcome != OutcomeMissingAPIKey {
		t.Errorf("outcome = %q, want %q", summary.Outcome, OutcomeMissingAPIKey)
	}
	if h.client.callCount() != 0 {
		t.Error("no request may be attempted without a key")
	}
}

// TestPipelineKeylessProviderIsAllowed covers a local server that accepts no
// authentication: needing no key is a valid configuration, not a missing one.
// TestAMissingKeyFromTheClientBuildIsTheMissingKeyOutcome covers the shape the native
// app actually produces: the engine never sees the key, so its NeedsAPIKey gate cannot
// fire, and the client factory is what discovers the missing credential. That failure
// is an outcome the user can act on, not a "failed" run with an English sentence
// appended.
func TestAMissingKeyFromTheClientBuildIsTheMissingKeyOutcome(t *testing.T) {
	snapshot := Snapshot{Bookmarks: posts(1, "ada")}
	h := newHarness(t, snapshot)
	h.deps.Client = nil
	h.deps.ClientError = errMissingCredential

	summary := h.run(t)

	if summary.Outcome != OutcomeMissingAPIKey {
		t.Fatalf("outcome = %q, want %q", summary.Outcome, OutcomeMissingAPIKey)
	}
	if summary.Error != "" {
		t.Errorf("error = %q, want empty: a missing key is not a failed run", summary.Error)
	}
}

func TestPipelineKeylessProviderIsAllowed(t *testing.T) {
	snapshot := Snapshot{Bookmarks: posts(1, "ada")}
	h := newHarness(t, snapshot)
	h.deps.Settings.LLM.NeedsAPIKey = false
	h.deps.Settings.LLM.APIKey = ""

	summary := h.run(t)

	if summary.Outcome == OutcomeMissingAPIKey {
		t.Error("a keyless provider must not be reported as missing a key")
	}
}

func TestPipelineNoCandidates(t *testing.T) {
	// The only post is already tagged.
	snapshot := Snapshot{
		Bookmarks:    posts(1, "ada"),
		BookmarkTags: []domain.BookmarkTag{{BookmarkID: posts(1, "ada")[0].TweetID, TagID: "t1"}},
	}
	h := newHarness(t, snapshot)

	summary := h.run(t)

	if summary.Outcome != OutcomeNoCandidates {
		t.Errorf("outcome = %q, want %q", summary.Outcome, OutcomeNoCandidates)
	}
	if h.client.callCount() != 0 {
		t.Error("nothing to ask about means no request")
	}
}

// TestPipelineEmptyVocabularyIsReported pins why the outcome exists: a closed
// vocabulary with nothing in it can never produce a suggestion, and reporting
// "done, 0 suggestions" for that hides a misconfiguration.
func TestPipelineEmptyVocabularyIsReported(t *testing.T) {
	snapshot := Snapshot{Bookmarks: posts(2, "ada")}
	h := newHarness(t, snapshot)
	h.deps.Settings.LLM.Mode = VocabularyClosed
	// No tags in the library at all.

	summary := h.run(t)

	if summary.Outcome != OutcomeEmptyVocabulary {
		t.Errorf("outcome = %q, want %q", summary.Outcome, OutcomeEmptyVocabulary)
	}
	if h.client.callCount() != 0 {
		t.Error("a model with nothing to choose from must not be called")
	}
}

// --- candidate selection ---

func TestPipelineCandidatesExcludeTaggedAndAnalyzed(t *testing.T) {
	bookmarks := posts(4, "ada")
	snapshot := Snapshot{
		Bookmarks: bookmarks,
		// The first post is tagged.
		BookmarkTags: []domain.BookmarkTag{{BookmarkID: bookmarks[0].TweetID, TagID: "t1"}},
		// The second already has a verdict from this model.
		Analysis: []domain.LlmAnalysis{liveVerdict("test-model", bookmarks[1].Text, bookmarks[1].TweetID)},
	}
	h := newHarness(t, snapshot)
	h.client.respond = func(request TagRequest) ([]TagVerdict, error) {
		return verdictsFor(request.Batch, "Topic"), nil
	}

	h.run(t)

	if h.client.callCount() == 0 {
		t.Fatal("the two eligible posts should have been sent")
	}
	sent := h.client.lastRequest().Batch
	if len(sent) != 2 {
		t.Fatalf("sent %d bookmarks, want 2 (one tagged, one already judged)", len(sent))
	}
	for _, bookmark := range sent {
		if bookmark.TweetID == bookmarks[0].TweetID || bookmark.TweetID == bookmarks[1].TweetID {
			t.Errorf("excluded bookmark %q was sent", bookmark.TweetID)
		}
	}
}

// liveVerdict is an analysis row that still counts, carrying all three invalidation
// inputs.
func liveVerdict(model, text string, bookmarkID string) domain.LlmAnalysis {
	return domain.LlmAnalysis{
		BookmarkID:    bookmarkID,
		Model:         model,
		PromptVersion: domain.LlmPromptVersion,
		ContentHash:   domain.HashAnalyzedContent(text),
	}
}

// TestPipelineLedgerIsModelScoped is the invalidation rule: a verdict from
// another model must not suppress a re-run under a new one.
func TestPipelineLedgerIsModelScoped(t *testing.T) {
	bookmarks := posts(1, "ada")
	snapshot := Snapshot{
		Bookmarks: bookmarks,
		Analysis:  []domain.LlmAnalysis{liveVerdict("a-different-model", bookmarks[0].Text, bookmarks[0].TweetID)},
	}
	h := newHarness(t, snapshot)
	h.client.respond = func(request TagRequest) ([]TagVerdict, error) {
		return verdictsFor(request.Batch, "Topic"), nil
	}

	h.run(t)

	if h.client.callCount() != 1 {
		t.Error("a verdict from another model must not make a post ineligible")
	}
}

// --- the ledger ---

// TestPipelineWritesALedgerRowForEveryBatchBookmark pins the idempotency
// mechanism: a post the model said nothing about still gets a row, or it is
// re-sent on every later run.
func TestPipelineWritesALedgerRowForEveryBatchBookmark(t *testing.T) {
	bookmarks := posts(3, "ada")
	h := newHarness(t, Snapshot{Bookmarks: bookmarks})
	h.client.respond = func(request TagRequest) ([]TagVerdict, error) {
		// Only answer about the first post.
		return []TagVerdict{{BookmarkID: request.Batch[0].TweetID, Tags: []TagCandidate{{Name: "Topic", Confidence: 0.9}}}}, nil
	}

	summary := h.run(t)

	if summary.Outcome != OutcomeOK {
		t.Errorf("outcome = %q, want %q", summary.Outcome, OutcomeOK)
	}
	if len(h.analysis) != 3 {
		t.Fatalf("wrote %d ledger rows, want 3 (one per batch post): %+v", len(h.analysis), h.analysis)
	}

	outcomeByBookmark := map[string]string{}
	for _, record := range h.analysis {
		outcomeByBookmark[record.BookmarkID] = record.Outcome
	}
	if outcomeByBookmark[bookmarks[0].TweetID] != domain.OutcomeTagged {
		t.Errorf("the answered post must be tagged, got %q", outcomeByBookmark[bookmarks[0].TweetID])
	}
	if outcomeByBookmark[bookmarks[1].TweetID] != domain.OutcomeNoTopic {
		t.Errorf("a post the model skipped must be no-topic, not omitted; got %q",
			outcomeByBookmark[bookmarks[1].TweetID])
	}
}

// --- budget ---

func TestPipelineGuardedBudgetCapsNewTags(t *testing.T) {
	// Three posts, each asking for a different new tag, with a budget of 1.
	bookmarks := posts(3, "ada")
	h := newHarness(t, Snapshot{Bookmarks: bookmarks})
	h.deps.Settings.LLM.Mode = VocabularyGuarded
	h.deps.Settings.LLM.MaxPerRun = 3
	h.deps.Settings.LLM.BatchSize = 3

	names := []string{"One", "Two", "Three"}
	h.client.respond = func(request TagRequest) ([]TagVerdict, error) {
		out := make([]TagVerdict, 0, len(request.Batch))
		for i, bookmark := range request.Batch {
			out = append(out, TagVerdict{BookmarkID: bookmark.TweetID, Tags: []TagCandidate{{Name: names[i], Confidence: 0.8}}})
		}
		return out, nil
	}

	// vocabularySize 49 leaves a headroom of exactly 1.
	h.deps.Settings.LLM.MaxPerRun = 3
	tags := make([]domain.Tag, 0, 49)
	for i := 0; i < 49; i++ {
		tags = append(tags, domain.Tag{ID: string(rune('A' + i)), Name: "Existing" + string(rune('A'+i))})
	}
	h.deps.Snapshot = func(context.Context) (Snapshot, error) {
		return Snapshot{Bookmarks: bookmarks, Tags: tags}, nil
	}

	summary := h.run(t)

	if summary.NewTagBudget != 1 {
		t.Errorf("NewTagBudget = %d, want 1 (50 - 49)", summary.NewTagBudget)
	}
	// At most one new tag is created; the rest are dropped, not silently ignored.
	if len(h.suggestions) != 1 {
		t.Errorf("created %d suggestions, want 1: %+v", len(h.suggestions), h.suggestions)
	}
	if summary.BudgetDroppedCount == 0 {
		t.Error("dropped tags must be counted, or the shortfall is invisible")
	}
}

func TestPipelineBudgetExhaustedOutcome(t *testing.T) {
	// Nothing in the library and a closed vocabulary is the empty case; here the
	// library is full so guarded has no headroom, and the model still proposes
	// new names.
	tags := make([]domain.Tag, 0, HealthyTagCountMax)
	for i := 0; i < HealthyTagCountMax; i++ {
		tags = append(tags, domain.Tag{ID: string(rune('A' + i)), Name: "Existing" + string(rune('A'+i))})
	}
	h := newHarness(t, Snapshot{Bookmarks: posts(1, "ada"), Tags: tags})
	h.deps.Settings.LLM.Mode = VocabularyGuarded
	h.client.respond = func(request TagRequest) ([]TagVerdict, error) {
		return verdictsFor(request.Batch, "Brand New"), nil
	}

	summary := h.run(t)

	if summary.Outcome != OutcomeBudgetExhausted {
		t.Errorf("outcome = %q, want %q: a full library with no headroom must say so",
			summary.Outcome, OutcomeBudgetExhausted)
	}
	if len(h.suggestions) != 0 {
		t.Errorf("nothing may be created with no budget: %+v", h.suggestions)
	}
}

// --- block list ---

func TestPipelineAllBlockedOutcome(t *testing.T) {
	h := newHarness(t, Snapshot{Bookmarks: posts(1, "ada")})
	h.deps.Settings.LLM.Mode = VocabularyOpen
	h.deps.Settings.BlockedTagKeys = []string{"Topic"}
	h.client.respond = func(request TagRequest) ([]TagVerdict, error) {
		return verdictsFor(request.Batch, "Topic"), nil
	}

	summary := h.run(t)

	if summary.Outcome != OutcomeAllBlocked {
		t.Errorf("outcome = %q, want %q", summary.Outcome, OutcomeAllBlocked)
	}
	if summary.BlockedTagKeyCount != 1 {
		t.Errorf("BlockedTagKeyCount = %d, want 1 surfaced", summary.BlockedTagKeyCount)
	}
}

// TestPipelineExistingVocabularySurvivesTheBlockList checks the vocabulary the
// model is offered excludes blocked names.
func TestPipelineExistingVocabularySurvivesTheBlockList(t *testing.T) {
	h := newHarness(t, Snapshot{
		Bookmarks: posts(1, "ada"),
		Tags: []domain.Tag{
			{ID: "t1", Name: "Kept"},
			{ID: "t2", Name: "Blocked"},
		},
	})
	h.deps.Settings.BlockedTagKeys = []string{"Blocked"}
	h.deps.Settings.LLM.Mode = VocabularyClosed
	h.client.respond = func(TagRequest) ([]TagVerdict, error) { return nil, nil }

	h.run(t)

	vocabulary := h.client.lastRequest().Vocabulary
	for _, name := range vocabulary {
		if name.Name == "Blocked" {
			t.Error("a blocked tag must not be offered to the model")
		}
	}
	if len(vocabulary) != 1 || vocabulary[0].Name != "Kept" {
		t.Errorf("vocabulary = %v, want just Kept", vocabulary)
	}
}

// --- waves, concurrency and failure ---

func TestPipelineSendsWholeBatchInOneRequest(t *testing.T) {
	h := newHarness(t, Snapshot{Bookmarks: posts(5, "ada")})
	h.deps.Settings.LLM.BatchSize = 5
	h.client.respond = func(TagRequest) ([]TagVerdict, error) { return nil, nil }

	h.run(t)

	if h.client.callCount() != 1 {
		t.Fatalf("made %d requests, want 1 batch of 5", h.client.callCount())
	}
	if len(h.client.lastRequest().Batch) != 5 {
		t.Errorf("batch size = %d, want 5", len(h.client.lastRequest().Batch))
	}
}

// TestPipelineUnsetConcurrencyIsSerial pins the deliberate choice: an unset
// concurrency means one request at a time, not a default that silently
// multiplies a user's spend.
func TestPipelineUnsetConcurrencyIsSerial(t *testing.T) {
	h := newHarness(t, Snapshot{Bookmarks: posts(4, "ada")})
	h.deps.Settings.LLM.BatchSize = 2
	h.deps.Settings.LLM.Concurrency = 0

	var inFlight, maxInFlight int
	var mu sync.Mutex
	h.client.respond = func(request TagRequest) ([]TagVerdict, error) {
		mu.Lock()
		inFlight++
		if inFlight > maxInFlight {
			maxInFlight = inFlight
		}
		mu.Unlock()

		time.Sleep(2 * time.Millisecond)

		mu.Lock()
		inFlight--
		mu.Unlock()
		return verdictsFor(request.Batch, "Topic"), nil
	}

	h.run(t)

	if maxInFlight != 1 {
		t.Errorf("max in-flight requests = %d, want 1 when concurrency is unset", maxInFlight)
	}
}

func TestPipelineConcurrencyIsCapped(t *testing.T) {
	if got := resolveConcurrency(0); got != 1 {
		t.Errorf("resolveConcurrency(0) = %d, want 1", got)
	}
	if got := resolveConcurrency(-1); got != 1 {
		t.Errorf("resolveConcurrency(-1) = %d, want 1", got)
	}
	if got := resolveConcurrency(2); got != 2 {
		t.Errorf("resolveConcurrency(2) = %d, want 2", got)
	}
	if got := resolveConcurrency(99); got != MaxConcurrency {
		t.Errorf("resolveConcurrency(99) = %d, want the cap %d", got, MaxConcurrency)
	}
}

// TestPipelineRateLimitShrinksTheWave covers the adaptation the extension used:
// one throttled batch says something about the shared key, so the next wave is
// narrower.
func TestPipelineRateLimitShrinksTheWave(t *testing.T) {
	h := newHarness(t, Snapshot{Bookmarks: posts(8, "ada")})
	h.deps.Settings.LLM.BatchSize = 1
	h.deps.Settings.LLM.Concurrency = 4

	var mu sync.Mutex
	seen := 0
	h.client.respond = func(request TagRequest) ([]TagVerdict, error) {
		mu.Lock()
		seen++
		first := seen == 1
		mu.Unlock()
		if first {
			return nil, &llm.LlmError{Kind: llm.KindRateLimit, Message: "429"}
		}
		return verdictsFor(request.Batch, "Topic"), nil
	}

	summary := h.run(t)

	// The run still completes; the first wave's failure is recorded, not fatal.
	if summary.Outcome == OutcomeFailed {
		t.Errorf("a single rate-limited batch must not fail the run: %+v", summary)
	}
	if summary.LLMBatchCount <= 1 {
		t.Errorf("LLMBatchCount = %d, want more than one wave", summary.LLMBatchCount)
	}
}

// TestPipelineAllBatchesFailingIsAFailure distinguishes "produced nothing" from
// "everything broke".
func TestPipelineAllBatchesFailingIsAFailure(t *testing.T) {
	h := newHarness(t, Snapshot{Bookmarks: posts(2, "ada")})
	h.client.respond = func(TagRequest) ([]TagVerdict, error) {
		return nil, &llm.LlmError{Kind: llm.KindServer, Message: "boom"}
	}

	summary := h.run(t)

	if summary.Outcome != OutcomeFailed {
		t.Errorf("outcome = %q, want %q", summary.Outcome, OutcomeFailed)
	}
	if summary.LLMFailedBatchCount == 0 {
		t.Error("failed batches must be counted")
	}
	if summary.Error == "" {
		t.Error("a failure must carry a message")
	}
}

// --- run state and cancellation ---

func TestPipelinePublishesAndClearsRunState(t *testing.T) {
	h := newHarness(t, Snapshot{Bookmarks: posts(2, "ada")})
	h.client.respond = func(request TagRequest) ([]TagVerdict, error) {
		return verdictsFor(request.Batch, "Topic"), nil
	}

	h.run(t)

	if len(h.saved) == 0 {
		t.Fatal("a run must publish its state so a killed run leaves a trace")
	}
	// The first published state is before any work, which is the point: a run
	// that dies immediately is still visible.
	first := h.saved[0]
	if first.RunID == "" || first.Phase != PhaseHeuristics {
		t.Errorf("first state = %+v", first)
	}
	if first.Total != 2 {
		t.Errorf("Total = %d, want the bookmark count", first.Total)
	}
	if len(h.cleared) != 1 || h.cleared[0] != first.RunID {
		t.Errorf("cleared = %v, want the run's own id", h.cleared)
	}
}

// TestPipelinePublishesItsProductsWhileItRuns pins the live feedback a run owes a
// reader: how much it has produced and what the latest tags are. Both used to exist
// only in the finished summary, so a four-minute pass showed a bare count and no sign
// that anything was coming out of it.
func TestPipelinePublishesItsProductsWhileItRuns(t *testing.T) {
	h := newHarness(t, Snapshot{Bookmarks: posts(2, "ada")})
	h.client.respond = func(request TagRequest) ([]TagVerdict, error) {
		return verdictsFor(request.Batch, "Topic"), nil
	}

	h.run(t)

	var finished *RunState
	for _, state := range h.saved {
		if state.Phase == PhaseLLM && state.Suggested > 0 {
			finished = state
		}
	}
	if finished == nil {
		t.Fatal("no published state carried the pass's suggestions")
	}
	if finished.Suggested != 2 {
		t.Errorf("published suggestions = %d, want 2", finished.Suggested)
	}
	if len(finished.RecentTags) != 1 || finished.RecentTags[0] != "Topic" {
		t.Errorf("recent tags = %v, want [Topic]", finished.RecentTags)
	}
	if finished.InFlight {
		t.Error("a finished batch must take itself out of flight")
	}
	if finished.Deadline.IsZero() {
		t.Error("the pass budget must be published so the interface can bound its estimate")
	}
}

// TestTheModelPhaseMeasuresItsOwnWorkload pins the two denominators a run publishes.
//
// The rule phase reads every bookmark and finishes at Total/Total; the model phase
// sends the eligible posts, capped at MaxPerRun. Keeping the library's count as the
// model phase's total made the bar read 200/9509 for a run that would stop at 2000:
// a percentage that can never fill and an estimate built from the wrong remainder.
func TestTheModelPhaseMeasuresItsOwnWorkload(t *testing.T) {
	h := newHarness(t, Snapshot{Bookmarks: posts(8, "ada")})
	h.deps.Settings.LLM.MaxPerRun = 5
	h.deps.Settings.LLM.BatchSize = 2
	h.client.respond = func(request TagRequest) ([]TagVerdict, error) {
		return verdictsFor(request.Batch, "Topic"), nil
	}

	h.run(t)

	var rulesDone, model *RunState
	for _, state := range h.saved {
		switch state.Phase {
		case PhaseHeuristics:
			if state.Processed > 0 {
				rulesDone = state
			}
		case PhaseLLM:
			if model == nil {
				model = state
			}
		}
	}
	if rulesDone == nil {
		t.Fatal("the rule phase never published its completion")
	}
	if rulesDone.Processed != 8 || rulesDone.Total != 8 {
		t.Errorf("rule phase state = %d/%d, want 8/8", rulesDone.Processed, rulesDone.Total)
	}
	if model == nil {
		t.Fatal("the model phase never published its total")
	}
	if model.Total != 5 {
		t.Errorf("model phase total = %d, want the run's 5-post workload", model.Total)
	}
	if model.Processed != 0 {
		t.Errorf("model phase processed = %d, want 0 at its start", model.Processed)
	}
}

func TestPipelineCancellationStopsAtAWaveBoundary(t *testing.T) {
	h := newHarness(t, Snapshot{Bookmarks: posts(8, "ada")})
	h.deps.Settings.LLM.BatchSize = 1
	h.deps.Settings.LLM.Concurrency = 1

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel during the first batch; the boundary check then stops the run.
	h.client.respond = func(request TagRequest) ([]TagVerdict, error) {
		cancel()
		return verdictsFor(request.Batch, "Topic"), nil
	}

	summary, err := Run(ctx, h.deps)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if summary.Outcome != OutcomeCancelled {
		t.Errorf("outcome = %q, want %q", summary.Outcome, OutcomeCancelled)
	}
	// The batch already in flight is kept, so no completed work is thrown away.
	if len(h.suggestions) == 0 {
		t.Error("work finished before the cancel must be kept")
	}
	if summary.UnprocessedCount == 0 {
		t.Error("UnprocessedCount must show what is left, so the user knows to run again")
	}

	// The ledger is written with the suggestions, so a cancelled run must keep both:
	// the model has already been paid for that batch, and a ledger row that never
	// landed means the next run analyses the same bookmarks again.
	if len(h.analysis) == 0 {
		t.Error("a cancelled run must keep the ledger rows for the work already done")
	}

	// The state must also be cleared, and through a context of its own: the run's own
	// context is cancelled by the time this is written, so writing through it fails and
	// leaves the run marked in flight for the rest of the session. The user then sees a
	// run that never finishes, with nothing explaining why.
	if len(h.cleared) == 0 {
		t.Fatal("a cancelled run must still clear its state, or the panel shows a run that never ends")
	}
	// It is the run's own id that was cleared, not something else's: clearing blindly
	// could wipe the state of a run that replaced this one.
	if len(h.cleared) > 1 {
		t.Errorf("cleared %v, want exactly one clear", h.cleared)
	}
}

func TestPipelineDeadlineStopsTheRun(t *testing.T) {
	h := newHarness(t, Snapshot{Bookmarks: posts(10, "ada")})
	h.deps.Settings.LLM.BatchSize = 1
	h.deps.Settings.LLM.Concurrency = 1
	h.deps.RunBudget = time.Millisecond

	// Each call advances the clock past the run budget.
	h.clock = base
	h.client.respond = func(request TagRequest) ([]TagVerdict, error) {
		h.clock = h.clock.Add(time.Second)
		return verdictsFor(request.Batch, "Topic"), nil
	}

	summary := h.run(t)

	if summary.Outcome != OutcomeTimedOut {
		t.Errorf("outcome = %q, want %q", summary.Outcome, OutcomeTimedOut)
	}
}

func TestPipelineRunLimitCapsCandidates(t *testing.T) {
	h := newHarness(t, Snapshot{Bookmarks: posts(10, "ada")})
	h.deps.Settings.LLM.MaxPerRun = 3
	h.deps.Settings.LLM.BatchSize = 3
	h.client.respond = func(TagRequest) ([]TagVerdict, error) { return nil, nil }

	summary := h.run(t)

	if summary.CandidateCount != 10 {
		t.Errorf("CandidateCount = %d, want the full eligible set of 10", summary.CandidateCount)
	}
	if summary.LLMProcessedCount != 3 {
		t.Errorf("LLMProcessedCount = %d, want the capped 3", summary.LLMProcessedCount)
	}
	if summary.UnprocessedCount != 7 {
		t.Errorf("UnprocessedCount = %d, want 7 so the user knows to run again", summary.UnprocessedCount)
	}
}

// --- helpers ---

func TestChunk(t *testing.T) {
	bookmarks := posts(5, "a")
	batches := chunk(bookmarks, 2)
	if len(batches) != 3 {
		t.Fatalf("got %d batches, want 3", len(batches))
	}
	if len(batches[0]) != 2 || len(batches[2]) != 1 {
		t.Errorf("batch sizes = %d,%d,%d, want 2,2,1", len(batches[0]), len(batches[1]), len(batches[2]))
	}
	if got := chunk(nil, 2); len(got) != 0 {
		t.Errorf("chunk(nil) = %v", got)
	}
}

// TestDistributeBudgetIsExact pins why the budget is split rather than shared: a
// wave of concurrent batches sharing one allowance could overshoot it by a factor
// of the concurrency.
func TestDistributeBudgetIsExact(t *testing.T) {
	tests := []struct {
		name    string
		total   float64
		batches int
		want    []float64
	}{
		{"even split", 6, 3, []float64{2, 2, 2}},
		{"remainder goes to the first batches", 7, 3, []float64{3, 2, 2}},
		{"less than one batch each", 2, 3, []float64{1, 1, 0}},
		{"a single tag is not lost", 1, 3, []float64{1, 0, 0}},
		{"zero budget allows nothing", 0, 3, []float64{0, 0, 0}},
		{"negative budget allows nothing", -5, 2, []float64{0, 0}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := distributeBudget(tc.total, tc.batches)
			if len(got) != len(tc.want) {
				t.Fatalf("got %d shares, want %d", len(got), len(tc.want))
			}
			sum := 0.0
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("share[%d] = %v, want %v", i, got[i], tc.want[i])
				}
				sum += got[i]
			}
			// The whole point: the shares never add up to more than the budget.
			// A negative budget is covered by the per-element check above.
			if tc.total > 0 && sum > tc.total {
				t.Errorf("shares sum to %v, want at most %v", sum, tc.total)
			}
		})
	}

	t.Run("an unbounded budget stays unbounded", func(t *testing.T) {
		shares := distributeBudget(math.Inf(1), 3)
		for i, share := range shares {
			if !math.IsInf(share, 1) {
				t.Errorf("share[%d] = %v, want +Inf", i, share)
			}
		}
	})

	t.Run("no batches", func(t *testing.T) {
		if got := distributeBudget(5, 0); len(got) != 0 {
			t.Errorf("got %v, want nothing", got)
		}
	})
}

// TestWaveNeverExceedsTheBudget checks the split end to end: several batches
// running at once, each asking for its own new tag, must collectively respect one
// budget.
func TestWaveNeverExceedsTheBudget(t *testing.T) {
	bookmarks := posts(4, "ada")
	// A full library means guarded has no headroom, so the only way to create
	// anything is an explicit budget.
	tags := make([]domain.Tag, 0, 49)
	for i := 0; i < 49; i++ {
		tags = append(tags, domain.Tag{ID: string(rune('A' + i)), Name: "Existing" + string(rune('A'+i))})
	}

	h := newHarness(t, Snapshot{Bookmarks: bookmarks, Tags: tags})
	h.deps.Settings.LLM.Mode = VocabularyGuarded
	h.deps.Settings.LLM.BatchSize = 1
	h.deps.Settings.LLM.Concurrency = 4 // four batches in one wave

	names := []string{"Alpha", "Beta", "Gamma", "Delta"}
	h.client.respond = func(request TagRequest) ([]TagVerdict, error) {
		out := make([]TagVerdict, 0, len(request.Batch))
		for _, bookmark := range request.Batch {
			index := int(bookmark.TweetID[0] - 'a')
			if index < 0 || index >= len(names) {
				index = 0
			}
			out = append(out, TagVerdict{BookmarkID: bookmark.TweetID, Tags: []TagCandidate{{Name: names[index], Confidence: 0.8}}})
		}
		return out, nil
	}

	summary := h.run(t)

	// Headroom is 50-49 = 1, so exactly one new tag may appear.
	if summary.NewTagBudget != 1 {
		t.Fatalf("NewTagBudget = %d, want 1", summary.NewTagBudget)
	}
	if len(h.suggestions) > 1 {
		t.Errorf("created %d suggestions with a budget of 1: %+v", len(h.suggestions), h.suggestions)
	}
}

// TestHeuristicSuggestionsSurviveEveryEarlyExit is a regression test for a defect
// the UI work exposed: the LLM gate returned a fresh summary, discarding the
// heuristic suggestions that had already been written to storage.
//
// The visible symptom was a run reporting "0 suggested" while the suggestions were
// sitting in the list -- the worst kind of wrong, because the count and the list
// disagree and neither is obviously the liar.
//
// The source reports suggestedCount as insertedCount + llmInsertedCount, so the
// heuristic half is counted in every outcome. These cases pin that.
func TestHeuristicSuggestionsSurviveEveryEarlyExit(t *testing.T) {
	// One rule that matches, so the heuristic half always produces something.
	matched := []domain.Bookmark{{
		TweetID: "b1", AuthorHandle: "ada",
		Text: "a post about kubernetes", SavedAt: base, LastSeenAt: base, UpdatedAt: base,
	}}

	// Each case stops the run at a different gate after the heuristics have run.
	tests := []struct {
		name string
		tune func(*Deps)
		want RunOutcome
	}{
		{
			name: "llm disabled",
			tune: func(d *Deps) { d.Settings.LLM.Enabled = false },
			want: OutcomeLLMDisabled,
		},
		{
			name: "missing api key",
			tune: func(d *Deps) {
				d.Settings.LLM.Enabled = true
				d.Settings.LLM.NeedsAPIKey = true
				d.Settings.LLM.APIKey = ""
			},
			want: OutcomeMissingAPIKey,
		},
		{
			name: "no candidates",
			tune: func(d *Deps) {
				d.Settings.LLM.Enabled = true
				// A model that already analysed this text, with an unchanged hash, is
				// not a candidate.
				d.Settings.LLM.Model = "already-analysed"
			},
			want: OutcomeNoCandidates,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, Snapshot{
				Bookmarks: matched,
				Analysis: []domain.LlmAnalysis{{
					BookmarkID:    "b1",
					PromptVersion: domain.LlmPromptVersion,
					Model:         "already-analysed",
					ContentHash:   domain.HashAnalyzedContent("a post about kubernetes"),
					Outcome:       domain.OutcomeTagged,
				}},
			})
			h.deps.Settings.Heuristics = HeuristicSettings{
				KeywordRules: []KeywordTagRule{{Keyword: "kubernetes", TagName: "K8s"}},
			}
			tc.tune(&h.deps)

			summary := h.run(t)

			if summary.Outcome != tc.want {
				t.Errorf("outcome = %q, want %q", summary.Outcome, tc.want)
			}
			// The rule matched, so the heuristic half produced a suggestion, and the
			// summary must say so.
			if summary.SuggestedCount == 0 {
				t.Errorf("outcome %q reported 0 suggestions, but the keyword rule matched; "+
					"the heuristic half must be counted in every outcome", summary.Outcome)
			}
			if len(h.suggestions) == 0 {
				t.Errorf("outcome %q stored no suggestions", summary.Outcome)
			}
			if summary.PendingCount == 0 {
				t.Errorf("outcome %q reported no pending suggestions", summary.Outcome)
			}
		})
	}
}

// TestPipelineLedgerIsPromptScoped is the invalidation input the code used to skip.
//
// The ledger stores a prompt version per verdict and the reuse check compared only the
// model, so a bookmark judged by an older prompt kept its verdict and was never re-sent.
// That is how improving the prompt (rounds 176-178: the two rule lists, the per-tag
// confidence, the vocabulary with usage counts) reached only bookmarks saved after it.
func TestPipelineLedgerIsPromptScoped(t *testing.T) {
	bookmarks := posts(1, "ada")
	stale := liveVerdict("test-model", bookmarks[0].Text, bookmarks[0].TweetID)
	stale.PromptVersion = domain.LlmPromptVersion - 1

	h := newHarness(t, Snapshot{
		Bookmarks: bookmarks,
		Analysis:  []domain.LlmAnalysis{stale},
	})
	h.client.respond = func(request TagRequest) ([]TagVerdict, error) {
		return verdictsFor(request.Batch, "Topic"), nil
	}

	h.run(t)

	sent := h.client.lastRequest().Batch
	if len(sent) != 1 || sent[0].TweetID != bookmarks[0].TweetID {
		t.Errorf("sent %d bookmarks; a verdict from an older prompt must be re-asked", len(sent))
	}
}

// TestPipelineLedgerIsContentScoped is the other one: an edited post is re-asked.
func TestPipelineLedgerIsContentScoped(t *testing.T) {
	bookmarks := posts(1, "ada")
	// The verdict was made from text the bookmark no longer has.
	stale := liveVerdict("test-model", "the text as it was before the edit", bookmarks[0].TweetID)

	h := newHarness(t, Snapshot{
		Bookmarks: bookmarks,
		Analysis:  []domain.LlmAnalysis{stale},
	})
	h.client.respond = func(request TagRequest) ([]TagVerdict, error) {
		return verdictsFor(request.Batch, "Topic"), nil
	}

	h.run(t)

	sent := h.client.lastRequest().Batch
	if len(sent) != 1 || sent[0].TweetID != bookmarks[0].TweetID {
		t.Errorf("sent %d bookmarks; an edited post must be re-asked", len(sent))
	}
}

// TestPipelineLedgerSkipsOnlyWhenAllThreeMatch is the other direction: a live verdict is
// still skipped, so the three checks do not turn every run into a full re-analysis.
func TestPipelineLedgerSkipsOnlyWhenAllThreeMatch(t *testing.T) {
	bookmarks := posts(1, "ada")
	h := newHarness(t, Snapshot{
		Bookmarks: bookmarks,
		Analysis:  []domain.LlmAnalysis{liveVerdict("test-model", bookmarks[0].Text, bookmarks[0].TweetID)},
	})
	h.client.respond = func(request TagRequest) ([]TagVerdict, error) {
		return verdictsFor(request.Batch, "Topic"), nil
	}

	h.run(t)

	if sent := h.client.lastRequest().Batch; len(sent) != 0 {
		t.Errorf("sent %d bookmarks; a verdict that still matches must be reused", len(sent))
	}
}
