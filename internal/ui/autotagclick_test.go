package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	engine "github.com/taoquo/trove/internal/autotag"
	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/i18n"
	"github.com/taoquo/trove/internal/usecase/tagging"
)

// TestTheAutoTagPageAnswersEveryClick walks the page's own controls: the way in from the
// table, the first switch, the keyword rules and their per-row remove, and the way out.
//
// The page is the second half of tag management, and its rows are drawn by the settings
// grammar, so it shares the same traps: a row control named after its verb alone answers for
// every row at once.
func TestTheAutoTagPageAnswersEveryClick(t *testing.T) {
	shell, db := newBatchShell(t)
	ctx := context.Background()
	shell.SetLocale(i18n.EN)
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(ctx)
	shell.ToggleTagMode(ctx)
	shell.loadAutoTagSettings(ctx)

	tester := ui.NewTester(shell.View, 1180, 900)
	tester.Frame()
	click := func(label string) {
		t.Helper()
		tester.Frame()
		if err := tester.Click(label); err != nil {
			t.Fatalf("clicking %q: %v (texts: %q)", label, err, tester.Texts())
		}
		tester.Frame()
	}

	// The way in from the table: the warm tab at the header's right.
	click(strings.ToUpper(shell.t().T(i18n.SettingsSectionAutoTag)))
	if got := shell.tagView(); got != TagViewAutoTag {
		t.Fatalf("the switch opened %q, want the auto-tag page", got)
	}

	// The first switch is auto-tagging itself: on, then off, both by the word on the half.
	click(shell.t().T(i18n.SettingsOn))
	if !shell.state.AutoTagEnabled {
		t.Error("the auto-tagging switch did not turn on")
	}
	click(shell.t().T(i18n.SettingsOff))
	if shell.state.AutoTagEnabled {
		t.Error("the auto-tagging switch did not turn off")
	}

	// Two rules, each removed by the control that names it. One rule alone would hide the
	// defect: every remove control on the page carries the same word.
	for _, rule := range []struct{ keyword, tag string }{
		{"scheduler", "go"}, {"sourdough", "baking"},
	} {
		click(shell.t().T(i18n.SettingsKeywordPlaceholder))
		tester.Type(rule.keyword)
		click(shell.t().T(i18n.SettingsRuleTagPlaceholder))
		tester.Type(rule.tag)
		click(shell.t().T(i18n.SettingsAddKeywordRule))
	}
	if got := len(shell.state.KeywordRules); got != 2 {
		t.Fatalf("%d keyword rules after two adds, want 2", got)
	}
	click(shell.t().Tf(i18n.SettingsRemoveKeywordRuleFor, "scheduler"))
	if len(shell.state.KeywordRules) != 1 || shell.state.KeywordRules[0].Keyword != "sourdough" {
		t.Errorf("removing the scheduler rule left %+v", shell.state.KeywordRules)
	}
	click(shell.t().Tf(i18n.SettingsRemoveKeywordRuleFor, "sourdough"))
	if len(shell.state.KeywordRules) != 0 {
		t.Errorf("removing the last rule left %+v", shell.state.KeywordRules)
	}

	// The way out of the mode.
	click(shell.t().T(i18n.TagExitManagement))
	if shell.state.TagMode {
		t.Error("Back to bookmarks left tag management open")
	}
}

// TestTheRunAndItsSuggestionsAnswerEveryClick drives the run through its buttons: Run now,
// Stop, the pending tag it produced, and the accept control on that tag's row.
//
// The run itself is already covered end to end (autotagcancel_test.go, autotagchain_test.go);
// this one only asserts that the controls the page draws are the ones that drive it, and
// that a row of suggestions is not a row of identically named buttons.
func TestTheRunAndItsSuggestionsAnswerEveryClick(t *testing.T) {
	ctx := context.Background()

	started := make(chan struct{}, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseRun := func() { releaseOnce.Do(func() { close(release) }) }

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	defer server.Close()
	defer releaseRun()

	shell, db := newBatchShell(t)
	shell.SetLocale(i18n.EN)
	seed(t, db, "1", "ada", "a post about Go schedulers", clockBase)

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

	// The window's deferral: publish queues the update and the test applies it, so the
	// state the page reads is written on this goroutine.
	published := make(chan func(), 32)
	shell.testPublish = func(fn func()) { published <- fn }
	drain := func() {
		t.Helper()
		for {
			select {
			case fn := <-published:
				fn()
			case <-time.After(500 * time.Millisecond):
				return
			}
		}
	}

	shell.Reload(ctx)
	shell.ToggleTagMode(ctx)
	shell.state.TagView = TagViewAutoTag
	shell.loadAutoTagSettings(ctx)

	tester := ui.NewTester(shell.View, 1180, 900)
	tester.Frame()
	click := func(label string) {
		t.Helper()
		tester.Frame()
		if err := tester.Click(label); err != nil {
			t.Fatalf("clicking %q: %v (texts: %q)", label, err, tester.Texts())
		}
		tester.Frame()
	}

	click(shell.t().T(i18n.ActionRunNow))
	if !shell.state.AutoTagRunning {
		t.Fatal("Run now did not start a run")
	}

	select {
	case <-started:
	case <-time.After(10 * time.Second):
		t.Fatal("the run never reached the provider")
	}

	if err := tester.Click(shell.t().T(i18n.ActionStopRun)); err != nil {
		t.Fatalf("clicking Stop: %v (texts: %q)", err, tester.Texts())
	}
	if live := tagger.CurrentRun(); live == nil || !live.CancelRequested {
		t.Fatalf("Stop did not ask the run to stop: %+v", live)
	}

	releaseRun()
	deadline := time.Now().Add(15 * time.Second)
	for {
		recorded, found, err := tagger.LastSummary(ctx)
		if err != nil {
			t.Fatalf("LastSummary: %v", err)
		}
		if found && !recorded.FinishedAt.IsZero() {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the stopped run did not finish")
		}
		time.Sleep(5 * time.Millisecond)
	}
	drain()

	// The run's one suggestion is offered on the card that ran it.
	if got := len(shell.state.PendingSuggestions); got != 1 {
		t.Fatalf("%d pending suggestions after the run, want 1", got)
	}
	pending := fmt.Sprintf("%d %s", 1, shell.t().T(i18n.SuggestionsPendingTitle))
	click(pending)
	if !shell.state.SuggestionsOpen {
		t.Fatalf("the pending-tags button did not open the panel (texts: %q)", tester.Texts())
	}

	// The row's accept control says which tag it accepts.
	click(shell.t().Tf(i18n.SuggestionsAcceptFor, "go"))
	drain()

	tags, err := repos.Bookmarks.TagsByBookmark(ctx, []string{"1"})
	if err != nil {
		t.Fatalf("TagsByBookmark: %v", err)
	}
	if len(tags["1"]) != 1 || tags["1"][0].Name != "go" {
		t.Errorf("accepting the suggestion left %+v, want the bookmark tagged go", tags["1"])
	}
	rows, err := repos.Suggestions.List(ctx, domain.StatusPending)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("%d suggestions are still pending after the accept", len(rows))
	}

	tester.Key(0, ui.KeyEscape)
	tester.Frame()
	if shell.state.SuggestionsOpen {
		t.Error("Escape left the suggestions panel open")
	}
}
