package autotag

import (
	"context"
	"errors"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/llm"
)

// Defaults.
const (
	DefaultRunBudget          = 4 * time.Minute
	DefaultBatchBudget        = 2 * time.Minute
	DefaultMaxBookmarksPerRun = 200
	DefaultBatchSize          = 25
	DefaultConcurrency        = 3
	// MaxConcurrency caps the batches in flight. More than this against one key
	// mostly produces rate limits.
	MaxConcurrency = 4
)

// Snapshot is the stored state a run works from.
//
// It is loaded once, so a run sees a consistent library even while the user
// edits it.
type Snapshot struct {
	Bookmarks    []domain.Bookmark
	Tags         []domain.Tag
	BookmarkTags []domain.BookmarkTag
	Suggestions  []domain.TagSuggestion
	Analysis     []domain.LlmAnalysis
}

// TagRequest is one batch to send to the model.
type TagRequest struct {
	ProviderID string
	Model      string
	Batch      []domain.Bookmark
	// Vocabulary is the tags the model may choose from, most used first. The count is
	// carried because the prompt states it, as "name (count)", which is what tells a
	// model that a tag is already well established.
	Vocabulary []VocabEntry
	// NewTagBudget is how many new tags this batch may introduce. Zero means it
	// may not invent any; UnboundedBudget means no limit.
	NewTagBudget int
	// DeadlineMs is the remaining run budget, so a request cannot outlive it.
	DeadlineMs int
}

// TagVerdict is the model's answer for one bookmark.
type TagVerdict struct {
	BookmarkID string
	// Tags carry their own confidence: the prompt asks for one per tag, and the cap is
	// applied after sorting by it. An entry-level confidence cannot express that -- with one
	// score per bookmark there is nothing to sort the tags against each other by.
	Tags []TagCandidate
}

// TagCandidate is one tag the model proposed, with the confidence it gave for that tag.
type TagCandidate struct {
	Name       string
	Confidence float64
}

// LLMClient performs the model call. It is an interface so the pipeline can be
// tested without a provider.
type LLMClient interface {
	SuggestTags(ctx context.Context, request TagRequest) ([]TagVerdict, error)
}

// LLMSettings is the part of the configuration the pipeline needs.
type LLMSettings struct {
	Enabled    bool
	ProviderID string
	// BaseURL overrides the provider's default endpoint. It is empty for the
	// hosted providers, and required for a self-hosted or proxied one.
	BaseURL string
	// Deployment is Azure's deployment name, which is part of the URL rather
	// than the model field. Without it the Azure adapter refuses the request, so
	// choosing Azure used to be a guaranteed failure.
	Deployment  string
	Model       string
	BatchSize   int
	Concurrency int
	MaxPerRun   int
	Mode        TagVocabularyMode
	// CapabilityOverrides is the user's own answer about what this model supports, and
	// LearnedCapabilities is what a previous probe found, keyed by provider and model.
	//
	// Both are passed into every capability resolution. There is no editor for either,
	// but they are part of the settings blob, so a backup that carries them must
	// round-trip rather than be dropped.
	CapabilityOverrides *llm.ModelCapabilities
	LearnedCapabilities map[string]llm.ModelCapabilities
	// APIKey is resolved by the caller from the credential store.
	APIKey string
	// NeedsAPIKey is false for an endpoint that accepts no authentication.
	NeedsAPIKey bool
}

// Settings is the pipeline's configuration.
type Settings struct {
	Enabled      bool
	RunAfterSync bool
	AutoContinue bool

	// AutoTagContinueCount is how many times this chain has already continued itself.
	// A run the user starts by hand resets it.
	AutoTagContinueCount int
	Heuristics           HeuristicSettings
	BlockedTagKeys       []string
	LLM                  LLMSettings
}

// DefaultSettings is a fresh install: rule-based tagging ready, the model off.
func DefaultSettings() Settings {
	return Settings{
		Enabled:              false,
		RunAfterSync:         true,
		AutoContinue:         true,
		AutoTagContinueCount: 0,
		Heuristics:           DefaultHeuristicSettings(),
		LLM: LLMSettings{
			Enabled:     false,
			ProviderID:  "openai",
			BatchSize:   DefaultBatchSize,
			Concurrency: DefaultConcurrency,
			MaxPerRun:   DefaultMaxBookmarksPerRun,
			Mode:        VocabularyGuarded,
		},
	}
}

// Deps are the pipeline's collaborators.
type Deps struct {
	Settings Settings
	// Snapshot loads the current state.
	Snapshot func(ctx context.Context) (Snapshot, error)
	// InsertSuggestions persists suggestions, returning how many were new.
	InsertSuggestions func(ctx context.Context, suggestions []domain.TagSuggestion) (int, error)
	// PersistAnalysis records the verdict ledger.
	PersistAnalysis func(ctx context.Context, records []domain.LlmAnalysis) error
	// Client performs model calls.
	Client LLMClient
	// ClientError is why the client could not be built, when it could not.
	ClientError error
	// SaveRunState persists the in-flight state after every batch, which is what
	// makes a killed run visible instead of invisible.
	SaveRunState func(ctx context.Context, state *RunState) error
	// LoadRunState re-reads the in-flight state at each wave boundary.
	//
	// It is how a cancel requested by the UI reaches a pipeline that is already
	// running: the engine's own copy cannot see a flag written by anyone else, so
	// without this a cancel would be recorded and then ignored.
	LoadRunState func(ctx context.Context) (*RunState, error)
	// ClearRunState removes the state, but only when the caller still owns it.
	ClearRunState func(ctx context.Context, runID string) (reported bool, err error)
	// RecordSummary stores the finished run's report.
	RecordSummary func(ctx context.Context, summary RunSummary) error
	// Now is the clock, for tests.
	Now func() time.Time
	// RunBudget and BatchBudget default to the constants above.
	RunBudget   time.Duration
	BatchBudget time.Duration
}

func (d Deps) now() time.Time {
	if d.Now != nil {
		return d.Now()
	}
	return time.Now()
}

// Run executes one auto-tagging pass.
//
// It never returns an error for a condition the user can see in the outcome: a
// disabled feature, a missing key or an empty vocabulary are outcomes, not
// failures. A returned error means the run could not be attempted at all.
func Run(ctx context.Context, deps Deps) (RunSummary, error) {
	runBudget := deps.RunBudget
	if runBudget <= 0 {
		runBudget = DefaultRunBudget
	}
	batchBudget := deps.BatchBudget
	if batchBudget <= 0 {
		batchBudget = DefaultBatchBudget
	}

	settings := deps.Settings
	started := deps.now()
	deadline := started.Add(runBudget)

	if !settings.Enabled {
		return finish(ctx, deps, "", RunSummary{Outcome: OutcomeDisabled}, nil)
	}

	snapshot, err := deps.Snapshot(ctx)
	if err != nil {
		summary := RunSummary{Outcome: OutcomeFailed, Error: err.Error()}
		return finish(ctx, deps, "", summary, err)
	}

	state := Begin(started, BeginInput{
		Phase:      PhaseHeuristics,
		Total:      len(snapshot.Bookmarks),
		ProviderID: settings.LLM.ProviderID,
		Model:      settings.LLM.Model,
		Deadline:   deadline,
	})
	if err := saveState(ctx, deps, state); err != nil {
		summary := RunSummary{Outcome: OutcomeFailed, Error: err.Error()}
		return finish(ctx, deps, "", summary, err)
	}

	// The whole run is reported under one run id, so a superseded run goes quiet.
	report := newReport(state.RunID, deps, snapshot)

	// --- heuristics ---
	blocked := BlockedTagKeySet(settings.BlockedTagKeys)
	report.blockedKeyCount = len(blocked)
	report.blocked = blocked
	heuristicSuggestions := SuggestHeuristics(HeuristicInput{
		Bookmarks:             snapshot.Bookmarks,
		Tags:                  snapshot.Tags,
		BookmarkTags:          snapshot.BookmarkTags,
		ExistingSuggestionIDs: existingSuggestionIDs(snapshot.Suggestions),
		Settings:              settings.Heuristics,
		BlockedTagKeys:        blocked,
		Now:                   started,
	})

	report.heuristics = heuristicSuggestions
	if len(heuristicSuggestions) > 0 {
		inserted, err := deps.InsertSuggestions(ctx, heuristicSuggestions)
		if err != nil {
			// The heuristics are the cheap half; losing them is worth reporting.
			summary := report.summary(OutcomeFailed)
			summary.Error = err.Error()
			return finish(ctx, deps, state.RunID, summary, err)
		}
		report.suggested += inserted
		report.pending += inserted
	}

	// Publish what the rule phase produced before the first request goes out, and
	// refresh the heartbeat with it. Rule matching over a large library is seconds of
	// local work with no network at all; without this the interface saw a heartbeat
	// older than the stall hint and reported a healthy run as possibly dead.
	//
	// No lock: the wave goroutines do not exist yet.
	{
		// The rule phase reads every bookmark, so it finishes at Total/Total. A rule
		// phase left at 0 while the model phase starts at its own denominator reads as
		// two different bars that disagree, which is what a reader sees as "the numbers
		// do not add up".
		names := recentTagNames(nil, heuristicSuggestions)
		updated, owns := Touch(state, TouchInput{
			RunID: state.RunID, Now: started, Phase: PhaseHeuristics,
			Processed: state.Total, Total: state.Total,
			Suggested: &report.suggested, RecentTags: &names,
		})
		if owns {
			*state = *updated
			if err := saveState(ctx, deps, state); err != nil {
				report.lastError = err.Error()
			}
		}
	}

	// Checkpoint: a cancel during the heuristics stops before any model spend.
	if stop, _ := refreshState(ctx, deps, state); stop {
		return finish(ctx, deps, state.RunID, report.summary(OutcomeCancelled), nil)
	}

	// --- LLM gate ---
	//
	// Every exit from here on reports through `report.summary`, which already holds
	// the heuristic suggestions. Returning early with a fresh summary would throw
	// them away: the rules ran, the rows are in the database, and the user would be
	// told "0 suggested" for suggestions they can see. The outcome is a label applied
	// after the heuristic insert, not a gate that precedes it.
	if !settings.LLM.Enabled {
		return finish(ctx, deps, state.RunID, report.summary(OutcomeLLMDisabled), nil)
	}
	if settings.LLM.NeedsAPIKey && strings.TrimSpace(settings.LLM.APIKey) == "" {
		return finish(ctx, deps, state.RunID, report.summary(OutcomeMissingAPIKey), nil)
	}
	if deps.Client == nil {
		if deps.ClientError != nil && errors.Is(deps.ClientError, errMissingCredential) {
			// No key is the one construction failure the user can act on: the rules ran,
			// the model was skipped, and the headline says so in the user's language.
			// Reporting it as a failure appended the factory's English sentence to a
			// localized "failed", which reads like a broken run rather than an unset key.
			return finish(ctx, deps, state.RunID, report.summary(OutcomeMissingAPIKey), nil)
		}
		// The real reason when there is one. "no model client configured" is what a
		// wiring mistake looks like, but an unknown provider or an unreadable Keychain
		// built the client on purpose and failed for a reason the user can act on.
		summary := report.summary(OutcomeFailed)
		if deps.ClientError != nil {
			summary.Error = deps.ClientError.Error()
		} else {
			summary.Error = "no model client configured"
		}
		return finish(ctx, deps, state.RunID, summary, errors.New(summary.Error))
	}

	// --- candidates ---
	vocabulary, vocabularySize := buildVocabulary(snapshot.Tags, blocked)
	entries := vocabularyForPrompt(snapshot.Tags, snapshot.BookmarkTags, blocked)
	candidates := eligibleBookmarks(snapshot, settings.LLM.Model)

	report.vocabularySize = vocabularySize
	report.candidateCount = len(candidates)

	if len(candidates) == 0 {
		return finish(ctx, deps, state.RunID, report.summary(OutcomeNoCandidates), nil)
	}
	// A closed vocabulary with nothing in it can never produce a suggestion, and
	// reporting "ok, 0 suggestions" for that hides a misconfiguration.
	if !AllowsNewTags(settings.LLM.Mode) && len(vocabulary) == 0 {
		return finish(ctx, deps, state.RunID, report.summary(OutcomeEmptyVocabulary), nil)
	}

	budget := ResolveNewTagBudget(settings.LLM.Mode, vocabularySize, len(candidates), settings.LLM.MaxPerRun)
	report.newTagBudget = EncodeBudget(budget)

	// --- wave batching ---
	stopped := report.runWaves(ctx, deps, candidates, vocabulary, entries, budget, deadline, batchBudget, state)

	return finish(ctx, deps, state.RunID, report.summary(stopped), nil)
}

// report accumulates a run's counts, so the summary is built in one place.
type report struct {
	runID string
	deps  Deps

	suggested int
	pending   int

	candidateCount int
	unprocessed    int

	processedCount   int
	batchCount       int
	failedBatchCount int
	// inFlight is how many batches are waiting on the provider. A bool was not
	// enough once batches finished independently: the first reply cleared the flag
	// while another request was still out.
	inFlight int

	budgetDropped   int
	blockedDropped  int
	vocabularySize  int
	newTagBudget    int
	blockedKeyCount int

	heuristics []domain.TagSuggestion
	// blocked is the canonical block list. The vocabulary the model is offered
	// already excludes these names, but a model can propose one anyway.
	blocked map[string]bool

	lastError string
}

func newReport(runID string, deps Deps, snapshot Snapshot) *report {
	return &report{runID: runID, deps: deps, pending: len(snapshot.Suggestions)}
}

func (r *report) summary(outcome RunOutcome) RunSummary {
	return RunSummary{
		FinishedAt:          r.deps.now(),
		Outcome:             outcome,
		SuggestedCount:      r.suggested,
		PendingCount:        r.pending,
		CandidateCount:      r.candidateCount,
		LLMProcessedCount:   r.processedCount,
		LLMBatchCount:       r.batchCount,
		LLMFailedBatchCount: r.failedBatchCount,
		NewTagBudget:        r.newTagBudget,
		VocabularySize:      r.vocabularySize,
		BlockedTagKeyCount:  r.blockedKeyCount,
		BlockedDroppedCount: r.blockedDropped,
		BudgetDroppedCount:  r.budgetDropped,
		UnprocessedCount:    r.unprocessed,
		Error:               r.lastError,
	}
}

// runWaves sends the candidates in waves and returns why it stopped.
//
// The wave shape is: `concurrency` batches of `BatchSize` run together, and the
// next wave starts only when the previous one finishes. That keeps a rate-limited
// key usable while still cutting a long run's wall time.
func (r *report) runWaves(
	ctx context.Context,
	deps Deps,
	candidates []domain.Bookmark,
	vocabulary map[string]string,
	entries []VocabEntry,
	budget float64,
	deadline time.Time,
	batchBudget time.Duration,
	state *RunState,
) RunOutcome {
	limit := deps.Settings.LLM.MaxPerRun
	if limit <= 0 {
		limit = DefaultMaxBookmarksPerRun
	}
	// Everything eligible, including what the run cap will leave out: that gap is
	// exactly what tells the user to run again.
	eligibleTotal := len(candidates)
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	r.unprocessed = eligibleTotal

	// The model phase's denominator is this run's workload -- the eligible posts,
	// capped at MaxPerRun -- not the library. That is the only total the run can
	// reach: a bar reading 200/9509 for a run that will stop at 2000 can never fill,
	// and every estimate built on that denominator is wrong in the same direction.
	if err := r.beginLLMPhase(ctx, state, len(candidates)); err != nil {
		r.lastError = err.Error()
	}

	batchSize := deps.Settings.LLM.BatchSize
	if batchSize <= 0 {
		batchSize = DefaultBatchSize
	}
	concurrency := resolveConcurrency(deps.Settings.LLM.Concurrency)

	remainingBudget := budget
	offset := 0

	for offset < len(candidates) {
		if deps.now().After(deadline) {
			return OutcomeTimedOut
		}
		// Cancellation and the deadline are checked at a wave boundary, so a wave
		// already in flight always finishes and its results are kept.
		if ctx.Err() != nil {
			return OutcomeCancelled
		}
		if stop, _ := refreshState(ctx, deps, state); stop {
			return OutcomeCancelled
		}

		waveEnd := offset + concurrency*batchSize
		if waveEnd > len(candidates) {
			waveEnd = len(candidates)
		}
		wave := candidates[offset:waveEnd]
		batches := chunk(wave, batchSize)

		// Each batch gets a private vocabulary and an exact share of the
		// remaining budget. Sharing one map across the goroutines was both a data
		// race and a source of nondeterministic prompts, and sharing one budget
		// let a wave overshoot it by a factor of the concurrency.
		waveBudgets := distributeBudget(remainingBudget, len(batches))

		results := r.runWave(ctx, deps, batches, vocabulary, entries, waveBudgets, deadline, batchBudget, state)

		// A rate limit anywhere in the wave halves the width: the key is the
		// shared resource, so one batch being throttled says something about all
		// of them.
		for _, result := range results {
			if result.rateLimited {
				concurrency = concurrency / 2
				if concurrency < 1 {
					concurrency = 1
				}
			}
		}

		// Merge at the wave boundary, which is single-threaded, so the next wave
		// sees the tags this one introduced.
		for _, result := range results {
			for key, display := range result.newTags {
				vocabulary[key] = display
			}
			remainingBudget -= float64(result.newTagsAccepted)
		}
		if remainingBudget < 0 {
			remainingBudget = 0
		}

		offset = waveEnd
		r.unprocessed = eligibleTotal - offset

		if err := saveState(ctx, deps, state); err != nil {
			r.lastError = err.Error()
		}
	}

	// Every batch failing is a failure, not a run that happened to produce
	// nothing.
	if r.batchCount > 0 && r.failedBatchCount == r.batchCount {
		if r.lastError == "" {
			r.lastError = "all-batches-failed"
		}
		return OutcomeFailed
	}
	if r.suggested == 0 && r.blockedDropped > 0 {
		return OutcomeAllBlocked
	}
	if r.suggested == 0 && r.budgetDropped > 0 {
		return OutcomeBudgetExhausted
	}
	return OutcomeOK
}

// waveResult is what one batch contributed.
type waveResult struct {
	newTagsAccepted int
	rateLimited     bool
	// newTags are the tags this batch introduced, merged back at the wave
	// boundary so the next wave can reuse them.
	newTags map[string]string
}

// distributeBudget splits a wave's budget across its batches.
//
// It is exact rather than approximate: the remainder is handed out one per batch,
// so a wave can never create more tags than the budget allows, however many
// batches run at once.
func distributeBudget(total float64, batches int) []float64 {
	shares := make([]float64, batches)
	if batches == 0 {
		return shares
	}
	if math.IsInf(total, 1) {
		for i := range shares {
			shares[i] = math.Inf(1)
		}
		return shares
	}
	if total <= 0 {
		return shares
	}

	whole := int(total)
	base := whole / batches
	remainder := whole % batches
	for i := range shares {
		shares[i] = float64(base)
		if i < remainder {
			shares[i]++
		}
	}
	return shares
}

// cloneVocabulary copies a vocabulary, so a batch can add to it without touching
// another batch's view.
func cloneVocabulary(vocabulary map[string]string) map[string]string {
	cloned := make(map[string]string, len(vocabulary)+4)
	for key, display := range vocabulary {
		cloned[key] = display
	}
	return cloned
}

func (r *report) runWave(
	ctx context.Context,
	deps Deps,
	batches [][]domain.Bookmark,
	vocabulary map[string]string,
	entries []VocabEntry,
	budgets []float64,
	deadline time.Time,
	batchBudget time.Duration,
	state *RunState,
) []waveResult {
	var (
		mu      sync.Mutex
		results = make([]waveResult, len(batches))
		wg      sync.WaitGroup
	)

	for index, batch := range batches {
		wg.Add(1)
		go func(index int, batch []domain.Bookmark) {
			defer wg.Done()

			// A private copy per batch: one batch inventing a tag must not change
			// another batch's prompt mid-flight, and concurrent map writes are a
			// data race.
			batchVocabulary := cloneVocabulary(vocabulary)
			budget := 0.0
			if index < len(budgets) {
				budget = budgets[index]
			}

			results[index] = r.runBatch(ctx, deps, batch, batchVocabulary, entries, budget, deadline, batchBudget, state, &mu)
		}(index, batch)
	}
	wg.Wait()
	return results
}

// runBatch sends one batch, records the verdicts and persists them.
func (r *report) runBatch(
	ctx context.Context,
	deps Deps,
	batch []domain.Bookmark,
	vocabulary map[string]string,
	entries []VocabEntry,
	budget float64,
	deadline time.Time,
	batchBudget time.Duration,
	state *RunState,
	mu *sync.Mutex,
) waveResult {
	result := waveResult{}

	// A batch may not outlive either budget.
	//
	// The remaining time is measured against the injected clock, not the wall
	// clock: mixing the two made every batch bail out the moment `Now` differed
	// from the real time, and it is the injected clock that the run budget uses.
	now := r.deps.now()
	remaining := deadline.Sub(now)
	if remaining <= 0 {
		return result
	}
	if batchBudget < remaining {
		remaining = batchBudget
	}

	request := TagRequest{
		ProviderID:   deps.Settings.LLM.ProviderID,
		Model:        deps.Settings.LLM.Model,
		Batch:        batch,
		Vocabulary:   entries,
		NewTagBudget: EncodeBudget(budget),
		DeadlineMs:   int(remaining.Milliseconds()),
	}

	mu.Lock()
	r.batchCount++
	startedErr := r.markBatchStarted(ctx, state, deps)
	mu.Unlock()
	if startedErr != nil {
		mu.Lock()
		r.lastError = startedErr.Error()
		mu.Unlock()
	}

	verdicts, err := deps.Client.SuggestTags(ctx, request)
	if err != nil {
		mu.Lock()
		r.failedBatchCount++
		r.lastError = err.Error()
		if llm.IsRateLimit(err) {
			result.rateLimited = true
		}
		if finishedErr := r.markBatchFinished(ctx, state, deps, nil); finishedErr != nil {
			r.lastError = finishedErr.Error()
		}
		mu.Unlock()
		return result
	}

	mu.Lock()
	defer mu.Unlock()

	suggestions, records, newTags, accepted, blockedDropped, budgetDropped :=
		r.applyVerdicts(verdicts, batch, vocabulary, budget)

	r.processedCount += len(batch)
	r.blockedDropped += blockedDropped
	r.budgetDropped += budgetDropped
	result.newTagsAccepted = accepted
	result.newTags = newTags

	// Suggestions and the ledger are written together per batch, so a run that is
	// killed keeps the work already done.
	//
	// Through a context of their own: the run's is cancelled as soon as the user asks
	// to stop, which is while this batch's reply is in flight, and writing through it
	// threw away the batch that had just come back.
	if len(suggestions) > 0 || len(records) > 0 {
		persist, cancelPersist := persistContext(ctx)

		if len(suggestions) > 0 {
			if _, err := deps.InsertSuggestions(persist, suggestions); err != nil {
				r.lastError = err.Error()
			} else {
				r.suggested += len(suggestions)
				r.pending += len(suggestions)
			}
		}
		if len(records) > 0 && deps.PersistAnalysis != nil {
			if err := deps.PersistAnalysis(persist, records); err != nil {
				r.lastError = err.Error()
			}
		}

		cancelPersist()
	}

	// The batch boundary the window sees: the reply landed, the rows are stored,
	// and the count, the suggestion total and the latest tag names move together.
	// Publishing only at the wave boundary made a working run look frozen for as
	// long as a wave of slow replies took.
	if finishedErr := r.markBatchFinished(ctx, state, deps, suggestions); finishedErr != nil {
		r.lastError = finishedErr.Error()
	}

	return result
}

// applyVerdicts turns the model's answers into suggestions and ledger rows.
//
// Every bookmark in a finished batch gets a ledger row, including the ones the
// model said nothing about. Without that row a skipped bookmark is re-sent on
// every later run, which is what kept newer bookmarks out of the window.
func (r *report) applyVerdicts(
	verdicts []TagVerdict,
	batch []domain.Bookmark,
	vocabulary map[string]string,
	budget float64,
) (suggestions []domain.TagSuggestion, records []domain.LlmAnalysis, newTags map[string]string, accepted, blockedDropped, budgetDropped int) {
	verdictByBookmark := make(map[string]TagVerdict, len(verdicts))
	for _, verdict := range verdicts {
		verdictByBookmark[verdict.BookmarkID] = verdict
	}

	remainingBudget := budget
	now := r.deps.now()
	// Tags this batch introduces, reported back so the wave boundary can merge
	// them into the shared vocabulary.
	newTags = map[string]string{}

	for _, bookmark := range batch {
		verdict, answered := verdictByBookmark[bookmark.TweetID]
		tagged := 0
		// The ledger row describes this bookmark, so its verdict counts only what
		// happened to this bookmark. It used to be handed the batch totals: one
		// blocked tag on A marked every other bookmark in the batch all-blocked,
		// and their rows then disagreed with the tags actually suggested for them.
		bookmarkBlocked := 0
		bookmarkBudget := 0

		if answered {
			for _, candidate := range verdict.Tags {
				name := strings.TrimSpace(candidate.Name)
				key := domain.CanonicalTagKey(name)
				if key == "" {
					continue
				}

				if IsBlocked(r.blocked, name) {
					blockedDropped++
					bookmarkBlocked++
					continue
				}

				if display, known := vocabulary[key]; known {
					// Reuse the stored spelling, so the library does not grow a
					// second spelling of a tag it already has.
					name = display
				} else if remainingBudget > 0 {
					if !math.IsInf(remainingBudget, 1) {
						remainingBudget--
					}
					display := domain.NormalizeTagDisplayName(name)
					vocabulary[key] = display
					newTags[key] = display
					accepted++
				} else {
					budgetDropped++
					bookmarkBudget++
					continue
				}

				id := domain.SuggestionID(bookmark.TweetID, name)
				suggestions = append(suggestions, domain.TagSuggestion{
					ID:         id,
					BookmarkID: bookmark.TweetID,
					TagName:    domain.NormalizeTagDisplayName(name),
					Source:     domain.SourceLLM,
					Confidence: candidate.Confidence,
					Status:     domain.StatusPending,
					CreatedAt:  now,
				})
				tagged++
			}
		}

		records = append(records, domain.LlmAnalysis{
			BookmarkID:    bookmark.TweetID,
			AnalyzedAt:    domain.FormatTimestamp(now),
			PromptVersion: domain.LlmPromptVersion,
			Model:         r.deps.Settings.LLM.Model,
			ContentHash:   domain.HashAnalyzedContent(bookmark.Text),
			Outcome:       analysisOutcome(tagged, bookmarkBlocked, bookmarkBudget),
			TagCount:      tagged,
		})
	}

	return suggestions, records, newTags, accepted, blockedDropped, budgetDropped
}

// analysisOutcome picks a verdict's outcome in priority order.
func analysisOutcome(tagged, blockedDropped, budgetDropped int) domain.AnalysisOutcome {
	switch {
	case tagged > 0:
		return domain.OutcomeTagged
	case blockedDropped > 0:
		return domain.OutcomeAllBlocked
	case budgetDropped > 0:
		return domain.OutcomeBudgetDropped
	default:
		return domain.OutcomeNoTopic
	}
}

// beginLLMPhase republishes the totals the model phase is measured against.
//
// The rule phase counts every bookmark; the model phase counts the posts this run
// will actually send. Publishing the second total is what makes "processed/total",
// the percentage and the estimate agree with the run rather than with the library.
func (r *report) beginLLMPhase(ctx context.Context, state *RunState, workload int) error {
	if state == nil {
		return nil
	}
	status := StatusRunning
	updated, owns := Touch(state, TouchInput{
		RunID: state.RunID, Now: r.deps.now(), Phase: PhaseLLM,
		Processed: r.processedCount, Total: workload, Status: &status,
		Suggested: &r.suggested,
	})
	if !owns {
		return nil
	}
	*state = *updated
	return saveState(ctx, r.deps, state)
}

// markBatchStarted publishes "a request is out" before it goes out.
//
// It is saved, not only touched: the window reads the persisted state, and a
// request can take longer than the stall hint. Without this the interface saw the
// last batch boundary and called a working request a dead run.
//
// Called with the report's lock held.
func (r *report) markBatchStarted(ctx context.Context, state *RunState, deps Deps) error {
	if state == nil {
		return nil
	}
	r.inFlight++
	started := deps.now()
	inFlight := true
	status := StatusRunning
	names := append([]string(nil), state.RecentTags...)
	updated, owns := Touch(state, TouchInput{
		RunID: state.RunID, Now: started, Phase: PhaseLLM,
		Processed: r.processedCount, Total: state.Total,
		Status: &status, InFlight: &inFlight, BatchStartedAt: &started,
		Suggested: &r.suggested, RecentTags: &names,
	})
	if !owns {
		return nil
	}
	*state = *updated
	return saveState(ctx, deps, state)
}

// markBatchFinished publishes the batch's products and takes it out of flight.
//
// Called with the report's lock held, after the suggestions are stored.
func (r *report) markBatchFinished(ctx context.Context, state *RunState, deps Deps, suggestions []domain.TagSuggestion) error {
	if state == nil {
		return nil
	}
	r.inFlight--
	if r.inFlight < 0 {
		r.inFlight = 0
	}
	inFlight := r.inFlight > 0
	names := recentTagNames(state.RecentTags, suggestions)
	updated, owns := Touch(state, TouchInput{
		RunID: state.RunID, Now: deps.now(), Phase: PhaseLLM,
		Processed: r.processedCount, Total: state.Total,
		InFlight:  &inFlight,
		Suggested: &r.suggested, RecentTags: &names,
	})
	if !owns {
		return nil
	}
	*state = *updated
	return saveState(ctx, deps, state)
}

// recentTagNames keeps the last few names a pass suggested, newest last.
//
// A repeated name moves to the end rather than appearing twice: the list answers
// "what did it just suggest", and a tag suggested in two batches is one tag.
func recentTagNames(existing []string, suggestions []domain.TagSuggestion) []string {
	names := append([]string(nil), existing...)
	for _, suggestion := range suggestions {
		name := strings.TrimSpace(suggestion.TagName)
		if name == "" {
			continue
		}
		key := domain.CanonicalTagKey(name)
		for index, seen := range names {
			if domain.CanonicalTagKey(seen) == key {
				names = append(names[:index], names[index+1:]...)
				break
			}
		}
		names = append(names, name)
	}
	if len(names) > RecentTagLimit {
		names = names[len(names)-RecentTagLimit:]
	}
	return names
}

func resolveConcurrency(configured int) int {
	if configured <= 0 {
		// An unset concurrency means strictly serial: defaulting it to 3 here would
		// silently multiply a user's spend.
		return 1
	}
	if configured > MaxConcurrency {
		return MaxConcurrency
	}
	return configured
}

// eligibleBookmarks selects the posts a model run should consider.
//
// A post is a candidate when it carries no tag and no reusable verdict exists.
// Posts that already have a tag are skipped, and the ledger is what makes the
// second run cheap instead of re-asking the same questions.
func eligibleBookmarks(snapshot Snapshot, model string) []domain.Bookmark {
	tagged := make(map[string]bool, len(snapshot.BookmarkTags))
	for _, relation := range snapshot.BookmarkTags {
		tagged[relation.BookmarkID] = true
	}

	// The text each verdict was made from, so an edited post is re-queued.
	textByID := make(map[string]string, len(snapshot.Bookmarks))
	for _, bookmark := range snapshot.Bookmarks {
		textByID[bookmark.TweetID] = bookmark.Text
	}

	reusable := make(map[string]bool, len(snapshot.Analysis))
	for _, record := range snapshot.Analysis {
		// All three invalidation inputs are checked: prompt version, model and
		// content hash.
		//
		// Only the model used to be compared here, while the comment above it claimed all
		// three. The consequence was not a wrong tag but a missing one: a bookmark judged
		// by an older prompt, or one whose text was edited since, kept its old verdict and
		// was never re-sent -- so an improved prompt reached only bookmarks saved after it.
		if record.Model != model {
			continue
		}
		if record.PromptVersion != domain.LlmPromptVersion {
			continue
		}
		if record.ContentHash != domain.HashAnalyzedContent(textByID[record.BookmarkID]) {
			continue
		}
		reusable[record.BookmarkID] = true
	}

	// Suggestions made before the ledger existed still count as "already asked":
	// without this, upgrading re-asks every post the model has already answered.
	legacy := make(map[string]bool, len(snapshot.Suggestions))
	for _, suggestion := range snapshot.Suggestions {
		if suggestion.Source == domain.SourceLLM {
			legacy[suggestion.BookmarkID] = true
		}
	}

	out := make([]domain.Bookmark, 0, len(snapshot.Bookmarks))
	for _, bookmark := range snapshot.Bookmarks {
		if tagged[bookmark.TweetID] {
			continue
		}
		if reusable[bookmark.TweetID] || legacy[bookmark.TweetID] {
			continue
		}
		out = append(out, bookmark)
	}
	return out
}

// VocabEntry is one tag offered to the model, with how many bookmarks carry it.
type VocabEntry struct {
	Name  string
	Count int
}

// vocabularyForPrompt orders the vocabulary the way the prompt presents it.
//
// Sorting is by usage descending and then by name, so the model reads the established
// tags first and the prompt's "most used first" is true. It is built from the relations
// the snapshot already carries, so this costs no extra read.
func vocabularyForPrompt(tags []domain.Tag, bookmarkTags []domain.BookmarkTag, blocked map[string]bool) []VocabEntry {
	// Every tag is offered, including one no bookmark carries yet: its count reads as
	// zero, so a tag the user created by hand and has not used is still a tag the model
	// may reuse. Dropping it would make the
	// vocabulary smaller than the library and push the model towards a new near-synonym.
	byID := make(map[string]string, len(tags))
	entries := make([]VocabEntry, 0, len(tags))
	for _, tag := range tags {
		key := domain.CanonicalTagKey(tag.Name)
		if key == "" || blocked[key] {
			continue
		}
		byID[tag.ID] = tag.Name
		entries = append(entries, VocabEntry{Name: tag.Name})
	}

	for _, relation := range bookmarkTags {
		name, ok := byID[relation.TagID]
		if !ok {
			continue
		}
		for i := range entries {
			if entries[i].Name == name {
				entries[i].Count++
				break
			}
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Count != entries[j].Count {
			return entries[i].Count > entries[j].Count
		}
		return entries[i].Name < entries[j].Name
	})
	return entries
}

// buildVocabulary seeds the allowed tag names from the library, skipping blocked
// ones.
func buildVocabulary(tags []domain.Tag, blocked map[string]bool) (map[string]string, int) {
	vocabulary := make(map[string]string, len(tags))
	size := 0
	for _, tag := range tags {
		key := domain.CanonicalTagKey(tag.Name)
		if key == "" || blocked[key] {
			continue
		}
		vocabulary[key] = tag.Name
		size++
	}
	return vocabulary, size
}

func existingSuggestionIDs(suggestions []domain.TagSuggestion) map[string]bool {
	ids := make(map[string]bool, len(suggestions))
	for _, suggestion := range suggestions {
		ids[suggestion.ID] = true
	}
	return ids
}

// chunk splits a wave into batches.
func chunk(bookmarks []domain.Bookmark, size int) [][]domain.Bookmark {
	if size <= 0 {
		size = DefaultBatchSize
	}
	var batches [][]domain.Bookmark
	for start := 0; start < len(bookmarks); start += size {
		end := start + size
		if end > len(bookmarks) {
			end = len(bookmarks)
		}
		batches = append(batches, bookmarks[start:end])
	}
	return batches
}

// refreshState re-reads the run state and reports whether to stop.
//
// It returns ownsState=false when the state now belongs to a different run, which
// means this run was taken over and must stop rather than keep writing work that
// will be discarded.
func refreshState(ctx context.Context, deps Deps, state *RunState) (stop bool, ownsState bool) {
	if deps.LoadRunState != nil {
		current, err := deps.LoadRunState(ctx)
		if err == nil && current != nil {
			if current.RunID != state.RunID {
				return true, false
			}
			// Adopt what the UI wrote; the rest of the state is ours.
			state.CancelRequested = current.CancelRequested
		}
	}
	return ShouldCancel(state, state.RunID, deps.now())
}

// saveState persists the in-flight state so a killed run stays visible.
func saveState(ctx context.Context, deps Deps, state *RunState) error {
	if deps.SaveRunState == nil || state == nil {
		return nil
	}
	// A copy, so a store that keeps the pointer sees the state as it was at this
	// moment rather than however it looks after the next batch.
	snapshot := *state
	return deps.SaveRunState(ctx, &snapshot)
}

// BookkeepingTimeout bounds a run's own final writes.
const BookkeepingTimeout = 10 * time.Second

// bookkeepingContext returns a context for recording a run's outcome.
//
// The run's own context may already be cancelled when the outcome is written — by
// a cancel, a deadline, or the app quitting. Writing through it fails, which
// leaves the run state set forever and the summary unrecorded: the user sees a run
// that never finishes and nothing explaining why. Only the next launch's heal
// would clear it, and a stuck panel for a whole session is not acceptable.
func bookkeepingContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), BookkeepingTimeout)
}

// PersistTimeout bounds a batch's writes after the run's context may have gone.
const PersistTimeout = 60 * time.Second

// persistContext returns a context for writing work the model has already paid for.
//
// The run's own context is cancelled the moment a user cancels, which is usually while
// a batch's reply is in flight. Writing that batch through it fails, so the suggestions
// the run had already produced were discarded -- the opposite of what the comment above
// the write claims, and the same defect the syncer had. The work is already done and
// already paid for; only the next launch's ledger would have shown it was lost.
func persistContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), PersistTimeout)
}

// finish clears the in-flight state and records the report.
//
// Its writes go through their own context, for the reason above.
//
// Clearing is skipped when there is no run id, which is the case for a run that
// never started: there is nothing of ours to clear, and clearing blindly could
// wipe the state of a run that replaced us.
func finish(ctx context.Context, deps Deps, runID string, summary RunSummary, runErr error) (RunSummary, error) {
	bookkeeping, cancelBookkeeping := bookkeepingContext(ctx)
	defer cancelBookkeeping()

	if runID != "" && deps.ClearRunState != nil {
		if _, err := deps.ClearRunState(bookkeeping, runID); err != nil && runErr == nil {
			runErr = err
		}
	}
	if deps.RecordSummary != nil {
		if err := deps.RecordSummary(bookkeeping, summary); err != nil && runErr == nil {
			runErr = err
		}
	}
	return summary, runErr
}
