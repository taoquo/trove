package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	engine "github.com/taoquo/trove/apps/macos/internal/autotag"
	"github.com/taoquo/trove/apps/macos/internal/domain"
	"github.com/taoquo/trove/apps/macos/internal/i18n"
	"github.com/taoquo/trove/apps/macos/internal/storage/sqlite"
	"github.com/taoquo/trove/apps/macos/internal/usecase/tagging"
)

// Stopping a run (F5-1).
//
// The copy promises that the batch in flight is kept, and the engine can do that: it
// reads the cancel flag at a wave boundary. The shell used to cancel the run's context
// instead, which aborts the request the user was told would finish, so the promise and
// the implementation were about different runs.

// TestAutoTagStoppingOffersNoSecondStopButton covers the two controls that were on screen
// at once: the progress bar's disabled "stopping" action and the run row's still-enabled
// "stop run". Pressing the second did nothing, which is worse than not drawing it.
func TestAutoTagStoppingOffersNoSecondStopButton(t *testing.T) {
	shell, _ := newShell(t)
	shell.state.TagMode = true
	shell.state.TagView = TagViewAutoTag
	shell.state.AutoTagRunning = true
	shell.captureRun = &engine.RunState{
		RunID:           "cancelling",
		StartedAt:       time.Now().Add(-time.Minute),
		UpdatedAt:       time.Now(),
		Phase:           engine.PhaseLLM,
		Processed:       1,
		Total:           2,
		Status:          engine.StatusCancelling,
		CancelRequested: true,
	}

	tester := ui.NewTester(shell.View, 1180, 900)
	tester.Frame()

	if !tester.HasText(shell.t().T(i18n.RunStopping)) {
		t.Fatalf("a cancelling run does not say it is stopping; texts: %q", tester.Texts())
	}
	if tester.HasText(shell.t().T(i18n.ActionStopRun)) {
		t.Errorf("a run that is already stopping still offers %q; texts: %q",
			shell.t().T(i18n.ActionStopRun), tester.Texts())
	}
}

// TestAutoTagStopKeepsTheBatchItIsOn is the promise in the status line, made checkable
// over HTTP: one request is out, the user stops the run, the request is not aborted, its
// suggestion is stored, and no further batch is sent.
func TestAutoTagStopKeepsTheBatchItIsOn(t *testing.T) {
	ctx := context.Background()

	var requests atomic.Int32
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseRun := func() { releaseOnce.Do(func() { close(release) }) }

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		answer := `{"results":[{"tweetId":"1","tags":[{"name":"go","confidence":0.95}]}]}`
		if id := tweetIDInPrompt(t, r); id != "" {
			answer = fmt.Sprintf(
				`{"results":[{"tweetId":%q,"tags":[{"name":"go","confidence":0.95}]}]}`, id)
		}
		select {
		case started <- struct{}{}:
		default:
		}
		<-release
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{
				"message":       map[string]any{"content": answer},
				"finish_reason": "stop",
			}},
		})
	}))
	// Registered after Close so it runs first: a handler blocked on release would
	// otherwise keep Close waiting forever.
	defer server.Close()
	defer releaseRun()

	shell, db := newBatchShell(t)
	seed(t, db, "1", "ada", "a post about Go schedulers", clockBase)
	seed(t, db, "2", "grace", "a post about sourdough", clockBase.Add(time.Minute))

	repos := autoTagRepos(db)
	tagger := tagging.New(repos, fixedClock{now: clockBase}, nil)
	tagger.UseProviderClient(func(settings engine.Settings) (engine.LLMClient, error) {
		return engine.BuildProviderClient(settings, func(context.Context, string) (string, error) {
			return "sk-test", nil
		})
	})
	shell.UseTagging(tagger)

	settings, err := tagger.LoadSettings(ctx)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	settings.Enabled = true
	settings.LLM.Enabled = true
	settings.LLM.ProviderID = "custom"
	settings.LLM.BaseURL = server.URL + "/v1"
	settings.LLM.Model = "test-model"
	settings.LLM.BatchSize = 1
	settings.LLM.Concurrency = 1
	settings.LLM.Mode = engine.VocabularyOpen
	settings.AutoContinue = false
	if err := tagger.SaveSettings(ctx, settings); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}

	if !shell.StartAutoTag(ctx) {
		t.Fatal("StartAutoTag refused a configured run")
	}

	select {
	case <-started:
	case <-time.After(10 * time.Second):
		t.Fatal("no request reached the provider")
	}

	if !shell.StopAutoTag() {
		t.Fatal("stopping a run in flight reported failure")
	}
	if live := tagger.CurrentRun(); live == nil || !live.CancelRequested {
		t.Fatalf("the run state does not carry the cancel; state = %+v", live)
	}

	// The request the run is waiting on completes, then the engine stops at the wave
	// boundary.
	releaseRun()

	deadline := time.Now().Add(15 * time.Second)
	var summary engine.RunSummary
	for {
		recorded, found, err := tagger.LastSummary(ctx)
		if err != nil {
			t.Fatalf("LastSummary: %v", err)
		}
		if found && !recorded.FinishedAt.IsZero() {
			summary = recorded
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the stopped run did not finish")
		}
		time.Sleep(5 * time.Millisecond)
	}

	if summary.Outcome != engine.OutcomeCancelled {
		t.Errorf("outcome = %q, want %q", summary.Outcome, engine.OutcomeCancelled)
	}
	if got := requests.Load(); got != 1 {
		t.Errorf("the provider saw %d requests, want 1: a stopped run sends no further batch", got)
	}
	if summary.UnprocessedCount != 1 {
		t.Errorf("unprocessed = %d, want 1", summary.UnprocessedCount)
	}

	rows, err := repos.Suggestions.List(ctx, domain.StatusPending)
	if err != nil {
		t.Fatalf("List suggestions: %v", err)
	}
	if len(rows) != 1 || rows[0].TagName != "go" {
		t.Fatalf("pending suggestions = %+v, want the batch in flight kept as go", rows)
	}
}

// TestAutoTagPanelReportsTheInterruptedRun is the join from the healed state to what
// the reader sees: after a restart the page must name the run that died and say how much
// of it was left, not show the previous run's result or an empty panel.
func TestAutoTagPanelReportsTheInterruptedRun(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()

	tagger := tagging.New(autoTagRepos(db), fixedClock{now: clockBase}, nil)
	shell.UseTagging(tagger)
	encoded := `{"RunID":"run-interrupted","Phase":"llm","Total":10,"Processed":3}`
	if err := sqlite.NewSettingsRepository(db).Set(ctx, "autotag.runState", encoded, clockBase); err != nil {
		t.Fatalf("seed run state: %v", err)
	}
	if healed, err := tagger.HealStaleRun(ctx); err != nil || !healed {
		t.Fatalf("HealStaleRun = %v, %v", healed, err)
	}

	shell.loadAutoTagSettings(ctx)

	if got, want := shell.state.AutoTagHeadline, shell.t().Tf(i18n.OutcomeInterrupted, 7); got != want {
		t.Errorf("headline = %q, want %q", got, want)
	}
	if shell.state.AutoTagRunning {
		t.Error("the shell claims a run is in flight after the stale one was healed")
	}
	if live := shell.liveRunState(); live != nil {
		t.Errorf("a healed run is still offered as live: %+v", live)
	}
}

// TestAutoTagResumeDecision is the startup continuation's decision
// table, from the extension's rule at src/background/index.ts:411.
func TestAutoTagResumeDecision(t *testing.T) {
	interrupted := engine.RunSummary{
		Outcome: engine.OutcomeInterrupted, CandidateCount: 2, LLMProcessedCount: 1, UnprocessedCount: 1,
	}
	settings := engine.DefaultSettings()
	settings.Enabled = true
	settings.AutoContinue = true

	off := settings
	off.Enabled = false
	noContinue := settings
	noContinue.AutoContinue = false
	spent := settings
	spent.AutoTagContinueCount = engine.ContinueLimit

	cases := []struct {
		name     string
		summary  engine.RunSummary
		settings engine.Settings
		want     bool
	}{
		{"work is left", interrupted, settings, true},
		{"the user stopped it", engine.RunSummary{
			Outcome: engine.OutcomeCancelled, UnprocessedCount: 1,
		}, settings, false},
		{"nothing is left", engine.RunSummary{Outcome: engine.OutcomeInterrupted}, settings, false},
		{"the feature is off", interrupted, off, false},
		{"auto-continue is off", interrupted, noContinue, false},
		{"the chain is spent", interrupted, spent, false},
	}
	for _, tc := range cases {
		if got := shouldResumeInterrupted(tc.summary, tc.settings); got != tc.want {
			t.Errorf("%s: shouldResumeInterrupted = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// tweetIDInPrompt reads the id the adapter put in the request's bookmark list.
func tweetIDInPrompt(t *testing.T, r *http.Request) string {
	t.Helper()

	var body struct {
		Messages []struct {
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.Messages) == 0 {
		return ""
	}
	text := body.Messages[len(body.Messages)-1].Content
	const marker = "Bookmarks: "
	index := strings.LastIndex(text, marker)
	if index < 0 {
		return ""
	}
	var batch []struct {
		TweetID string `json:"tweetId"`
	}
	if err := json.Unmarshal([]byte(text[index+len(marker):]), &batch); err != nil || len(batch) == 0 {
		return ""
	}
	return batch[0].TweetID
}
