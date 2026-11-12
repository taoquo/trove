package ui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	engine "github.com/taoquo/x-master/apps/macos/internal/autotag"
	"github.com/taoquo/x-master/apps/macos/internal/domain"
	"github.com/taoquo/x-master/apps/macos/internal/ports"
	"github.com/taoquo/x-master/apps/macos/internal/storage/sqlite"
	"github.com/taoquo/x-master/apps/macos/internal/usecase/tagging"
)

func autoTagRepos(db *sqlite.DB) ports.Repositories {
	return ports.Repositories{
		Bookmarks:   sqlite.NewBookmarkRepository(db),
		Tags:        sqlite.NewTagRepository(db),
		Lists:       sqlite.NewListRepository(db),
		Suggestions: sqlite.NewSuggestionRepository(db),
		Analysis:    sqlite.NewAnalysisRepository(db),
		Settings:    sqlite.NewSettingsRepository(db),
	}
}

// TestTheRunChainReachesTheProviderAndRecordsItsSuggestions walks the path the "run now"
// button takes, end to end and over HTTP: stored settings, the client factory, the
// request the adapter writes, the parser, the suggestion store, the recorded summary and
// the status line.
//
// The pieces were each tested and the joins were not. Two of the joins were wrong in the
// same release -- the custom provider never sent its key, and the OpenAI-compatible body
// named no model -- so the settings page passed its connection test while every run
// failed. A test that stops at the adapter would have passed both times.
func TestTheRunChainReachesTheProviderAndRecordsItsSuggestions(t *testing.T) {
	ctx := context.Background()

	var sawKey, sawModel, sawPath bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path = %q, want /v1/chat/completions", r.URL.Path)
		}
		sawPath = true
		sawKey = r.Header.Get("Authorization") == "Bearer sk-test"
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		model, _ := body["model"].(string)
		sawModel = model == "test-model"

		answer := `{"results":[` +
			`{"tweetId":"1","tags":[{"name":"go","confidence":0.9}]},` +
			`{"tweetId":"2","tags":[{"name":"baking","confidence":0.8}]}]}`
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{
				"message":       map[string]any{"content": answer},
				"finish_reason": "stop",
			}},
		})
	}))
	defer server.Close()

	shell, db := newBatchShell(t)
	seed(t, db, "1", "ada", "a post about Go schedulers", clockBase)
	seed(t, db, "2", "grace", "a post about sourdough", clockBase)

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
	settings.LLM.BatchSize = 10
	settings.LLM.Concurrency = 1
	// Guarded mode allows one new tag for two candidates (one per 150, minimum one),
	// which would drop the second row and make this a test of the budget rather than
	// of the chain.
	settings.LLM.Mode = engine.VocabularyOpen
	if err := tagger.SaveSettings(ctx, settings); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}

	if !shell.StartAutoTag(ctx) {
		t.Fatal("StartAutoTag refused a configured run")
	}

	// The run is asynchronous, and the shell's state is written inside publish on the
	// run's goroutine: reading it here would be the data race the first version of this
	// test had. The recorded summary is the run's own synchronized signal, so wait on it
	// instead, then give the publish that follows it a moment to finish.
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
			t.Fatal("the run did not finish")
		}
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)

	if !sawPath {
		t.Fatal("no request reached the provider")
	}
	if !sawKey {
		t.Error("the request carried no API key")
	}
	if !sawModel {
		t.Error("the request named no model")
	}

	rows, err := repos.Suggestions.List(ctx, domain.StatusPending)
	if err != nil {
		t.Fatalf("List suggestions: %v", err)
	}
	names := map[string]bool{}
	for _, row := range rows {
		names[row.TagName] = true
	}
	if len(rows) != 2 || !names["go"] || !names["baking"] {
		t.Fatalf("pending suggestions = %+v, want go and baking", rows)
	}

	if summary.Outcome != engine.OutcomeOK {
		t.Errorf("outcome = %q, want %q (error %q)", summary.Outcome, engine.OutcomeOK, summary.Error)
	}
	if summary.SuggestedCount != 2 {
		t.Errorf("the summary reports %d suggestions, want 2", summary.SuggestedCount)
	}
	// The status line is what the button's press shows; it is drawn from this summary
	// through SummarizeRun, which the outcome tests cover.
	if headline, _ := shell.SummarizeRun(summary.Outcome, summary); headline == "" {
		t.Error("the panel must report the finished run, not leave the button looking dead")
	}
}

// TestTheChainContinuesAfterTheFinishedStateIsApplied pins the ordering that only exists
// with a real window.
//
// publish hands an update to the window, which applies it later. The chain used to be
// decided beside the publish call rather than inside it, so it read AutoTagRunning while
// the previous pass's "finished" update was still queued: startAutoTag answered "an
// analysis is already running" and the chain died after its first pass. A tester publishes
// inline, so the suite passed while every real run stopped after one batch of 200.
func TestTheChainContinuesAfterTheFinishedStateIsApplied(t *testing.T) {
	ctx := context.Background()
	shell, db := newBatchShell(t)

	tagger := tagging.New(autoTagRepos(db), fixedClock{now: clockBase}, nil)
	shell.UseTagging(tagger)

	settings, err := tagger.LoadSettings(ctx)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	settings.Enabled = true
	settings.LLM.Enabled = true
	settings.AutoContinue = true
	if err := tagger.SaveSettings(ctx, settings); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}

	// The window's deferral, simulated: an update is applied only when the test drains it.
	published := make(chan func(), 32)
	shell.testPublish = func(fn func()) { published <- fn }

	var mu sync.Mutex
	passes := 0
	shell.UseAutoTagRunner(func(context.Context, engine.Settings) (engine.RunSummary, error) {
		mu.Lock()
		passes++
		mu.Unlock()
		// Every pass leaves work behind, so only the chain's own limit can stop it.
		return engine.RunSummary{Outcome: engine.OutcomeOK, UnprocessedCount: 50}, nil
	})

	if !shell.StartAutoTag(ctx) {
		t.Fatal("StartAutoTag refused")
	}

	// Drain updates as the window would. Each one may start another pass, whose update
	// arrives a moment later; a quiet channel means the chain has settled.
	deadline := time.Now().Add(20 * time.Second)
	for draining := true; draining; {
		select {
		case fn := <-published:
			fn()
		case <-time.After(500 * time.Millisecond):
			draining = false
		case <-time.After(time.Until(deadline)):
			t.Fatal("the chain did not settle")
		}
	}

	mu.Lock()
	got := passes
	mu.Unlock()
	if got != engine.ContinueLimit+1 {
		t.Fatalf("the chain ran %d passes, want %d (the hand-started pass plus the limit)",
			got, engine.ContinueLimit+1)
	}
}
