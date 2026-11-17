package autotag

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/llm"
)

// fakeAdapter answers from a script and records the requests it saw.
type fakeAdapter struct {
	results  []llm.ChatResult
	errs     []error
	requests []llm.ChatRequest
	envs     []llm.AdapterEnv
}

func (f *fakeAdapter) ID() string        { return llm.APIOpenAIChat }
func (f *fakeAdapter) NeedsAPIKey() bool { return false }

func (f *fakeAdapter) ChatJSON(_ context.Context, request llm.ChatRequest, env llm.AdapterEnv) (llm.ChatResult, error) {
	f.requests = append(f.requests, request)
	f.envs = append(f.envs, env)
	index := len(f.requests) - 1
	if index < len(f.errs) && f.errs[index] != nil {
		return llm.ChatResult{}, f.errs[index]
	}
	if index < len(f.results) {
		return f.results[index], nil
	}
	return llm.ChatResult{}, errors.New("no scripted answer")
}

func (f *fakeAdapter) Probe(llm.AdapterEnv) error { return nil }

func jsonResult(text string) llm.ChatResult {
	parsed, _ := llm.ParseJSONLoose(text)
	object, _ := parsed.(map[string]any)
	return llm.ChatResult{Text: text, Parsed: object}
}

func TestBuildPromptStatesTheRules(t *testing.T) {
	request := TagRequest{
		Batch: []domain.Bookmark{
			{TweetID: "1", AuthorHandle: "ada", Text: "hello"},
		},
		Vocabulary:   []VocabEntry{{Name: "Go", Count: 3}, {Name: "Rust", Count: 1}},
		NewTagBudget: 2,
	}
	prompt := BuildPrompt(request)

	// The two rule lists are the definition of a good tag, and they are the part of the
	// prompt a model actually acts on: without them it invents near-synonyms and one-off
	// tags, which the pipeline then drops.
	for _, want := range []string{
		"Go", "Rust",
		// The role and the target size.
		"librarian assigning topical tags",
		"The whole library should settle at 30-50 tags",
		// Rules for choosing tags.
		"Tag the durable subject area, not the specific event",
		"A good tag applies to dozens of bookmarks",
		"Prefer the broader concept over a narrow instance",
		"An imperfect existing tag beats a precise new one",
		"Never introduce a near-synonym, translation, abbreviation, plural or hyphenated variant",
		"Assign 1-2 tags per bookmark",
		"Skip a bookmark entirely if it has no clear topic",
		// Naming rules.
		"English only, lowercase, singular",
		"One or two words, words separated by a single space",
		"No leading '#', no emoji, no punctuation, no author handles",
		// The budget for this batch.
		"at most 2 new tag(s)",
		// The vocabulary and the batch.
		"Existing vocabulary, most used first",
		// The count is part of the entry: it is what tells the model a tag is established.
		"Go (3), Rust (1)",
		`"tweetId":"1"`,
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt is missing %q:\n%s", want, prompt)
		}
	}
	// The required answer shape is spelled out, so a model that ignores the
	// schema still has a chance of replying usefully.
	if !strings.Contains(prompt, `"results"`) {
		t.Error("the prompt must show the expected shape")
	}
}

// TestBuildPromptForbidsInventionWhenTheBudgetIsZero covers the closed vocabulary:
// telling the model is cheaper than dropping what it proposes.
func TestBuildPromptForbidsInventionWhenTheBudgetIsZero(t *testing.T) {
	prompt := BuildPrompt(TagRequest{
		Batch:        []domain.Bookmark{{TweetID: "1", Text: "x"}},
		Vocabulary:   []VocabEntry{{Name: "Go", Count: 2}},
		NewTagBudget: 0,
	})
	// The wording for a closed vocabulary.
	if !strings.Contains(prompt, "You must only use tags from the vocabulary below") ||
		!strings.Contains(prompt, "Never invent new tags") {
		t.Errorf("prompt = %s", prompt)
	}
}

func TestBuildPromptAllowsInventionWhenUnbounded(t *testing.T) {
	prompt := BuildPrompt(TagRequest{
		Batch:        []domain.Bookmark{{TweetID: "1", Text: "x"}},
		NewTagBudget: UnboundedBudget,
	})
	// The wording when new tags are allowed without a budget.
	if !strings.Contains(prompt, "Introduce a new tag only when nothing in the vocabulary comes close") {
		t.Errorf("prompt = %s", prompt)
	}
}

func TestBuildPromptTruncatesLongText(t *testing.T) {
	long := strings.Repeat("x", MaxPromptTextLength+500)
	prompt := BuildPrompt(TagRequest{Batch: []domain.Bookmark{{TweetID: "1", Text: long}}})

	if strings.Contains(prompt, long) {
		t.Error("a long post must be truncated so it does not crowd out the batch")
	}
	// A plain slice. The marker used to be an ellipsis added here, and that was an
	// invention: the model was reading a character the post never contained, and an
	// earlier test pinned it.
	if strings.Contains(prompt, "…") {
		t.Error("the truncation should be a plain cut, not an ellipsis")
	}
	if !strings.Contains(prompt, strings.Repeat("x", MaxPromptTextLength)) {
		t.Error("the prompt should carry the first MaxPromptTextLength characters")
	}
}

func TestParseVerdicts(t *testing.T) {
	batch := []domain.Bookmark{{TweetID: "1"}, {TweetID: "2"}}

	// The shape the prompt asks for:
	//   {"results":[{"tweetId":"...","tags":[{"name":"...","confidence":0.8}]}]}
	t.Run("a well-formed reply", func(t *testing.T) {
		verdicts, err := ParseVerdicts(jsonResult(`{"results":[
			{"tweetId":"1","tags":[{"name":"Go","confidence":0.9},{"name":"CLI","confidence":0.4}]},
			{"tweetId":"2","tags":[{"name":"Rust","confidence":0.7}]}]}`), batch)
		if err != nil {
			t.Fatalf("ParseVerdicts: %v", err)
		}
		if len(verdicts) != 2 {
			t.Fatalf("got %d verdicts, want 2", len(verdicts))
		}
		if len(verdicts[0].Tags) != 2 || verdicts[0].Tags[0].Name != "Go" || verdicts[0].Tags[0].Confidence != 0.9 {
			t.Errorf("verdict = %+v", verdicts[0])
		}
	})

	// Sorting is by confidence and only then cutting to the cap, so the two that
	// survive are the model's best two rather than its first two.
	t.Run("the cap keeps the highest confidence, not the first two", func(t *testing.T) {
		verdicts, err := ParseVerdicts(jsonResult(`{"results":[
			{"tweetId":"1","tags":[
				{"name":"weak","confidence":0.1},
				{"name":"strongest","confidence":0.95},
				{"name":"middle","confidence":0.5}]}]}`), batch)
		if err != nil {
			t.Fatalf("ParseVerdicts: %v", err)
		}
		if len(verdicts) != 1 || len(verdicts[0].Tags) != MaxTagsPerBookmark {
			t.Fatalf("verdicts = %+v, want one verdict at the cap", verdicts)
		}
		if verdicts[0].Tags[0].Name != "strongest" || verdicts[0].Tags[1].Name != "middle" {
			t.Errorf("kept %+v, want strongest then middle: the cut is after the sort",
				verdicts[0].Tags)
		}
	})

	// An entry with nothing usable is skipped rather than recorded empty.
	t.Run("an entry with no tags is skipped", func(t *testing.T) {
		verdicts, err := ParseVerdicts(jsonResult(`{"results":[
			{"tweetId":"1","tags":[]},
			{"tweetId":"2","tags":[{"name":"   ","confidence":0.5},{"name":"real","confidence":0.5}]}]}`), batch)
		if err != nil {
			t.Fatalf("ParseVerdicts: %v", err)
		}
		if len(verdicts) != 1 || verdicts[0].BookmarkID != "2" {
			t.Fatalf("verdicts = %+v, want only the one with a usable tag", verdicts)
		}
		if len(verdicts[0].Tags) != 1 || verdicts[0].Tags[0].Name != "real" {
			t.Errorf("tags = %+v, want the blank name dropped", verdicts[0].Tags)
		}
	})

	// A hallucinated id would otherwise create a suggestion pointing at nothing.
	t.Run("an unknown tweet id is dropped", func(t *testing.T) {
		verdicts, err := ParseVerdicts(jsonResult(`{"results":[
			{"tweetId":"1","tags":[{"name":"Go","confidence":0.5}]},
			{"tweetId":"does-not-exist","tags":[{"name":"Ghost","confidence":0.5}]}]}`), batch)
		if err != nil {
			t.Fatalf("ParseVerdicts: %v", err)
		}
		if len(verdicts) != 1 || verdicts[0].BookmarkID != "1" {
			t.Fatalf("verdicts = %+v, want only the known id", verdicts)
		}
	})

	t.Run("confidence is clamped and defaulted", func(t *testing.T) {
		verdicts, err := ParseVerdicts(jsonResult(`{"results":[
			{"tweetId":"1","tags":[{"name":"A","confidence":7}]},
			{"tweetId":"2","tags":[{"name":"B","confidence":-3}]},
			{"tweetId":"1","tags":[]}]}`), batch)
		if err != nil {
			t.Fatalf("ParseVerdicts: %v", err)
		}
		if verdicts[0].Tags[0].Confidence != 1 {
			t.Errorf("confidence = %v, want 1", verdicts[0].Tags[0].Confidence)
		}
		if verdicts[1].Tags[0].Confidence != 0 {
			t.Errorf("confidence = %v, want 0", verdicts[1].Tags[0].Confidence)
		}
	})

	// A tag without a numeric confidence still counts, at the default. Dropping it
	// would lose a tag the model did propose.
	t.Run("a missing confidence is half", func(t *testing.T) {
		verdicts, err := ParseVerdicts(jsonResult(`{"results":[
			{"tweetId":"1","tags":[{"name":"Go"}]}]}`), batch)
		if err != nil {
			t.Fatalf("ParseVerdicts: %v", err)
		}
		if len(verdicts) != 1 || verdicts[0].Tags[0].Confidence != 0.5 {
			t.Errorf("verdicts = %+v, want a confidence of 0.5", verdicts)
		}
	})
}

func TestProviderClientSucceeds(t *testing.T) {
	adapter := &fakeAdapter{results: []llm.ChatResult{
		jsonResult(`{"results":[{"tweetId":"1","tags":[{"name":"Go","confidence":0.9}]}]}`),
	}}
	client := &ProviderClient{Adapter: adapter}

	verdicts, err := client.SuggestTags(context.Background(), TagRequest{
		Batch: []domain.Bookmark{{TweetID: "1", Text: "x"}},
	})
	if err != nil {
		t.Fatalf("SuggestTags: %v", err)
	}
	if len(verdicts) != 1 || verdicts[0].Tags[0].Name != "Go" {
		t.Errorf("verdicts = %+v", verdicts)
	}
	if adapter.requests[0].JSONSchema == nil {
		t.Error("the schema must be sent so the provider can enforce it")
	}
}

// TestProviderClientAsksForABatchSizedCeiling pins item 11: the response ceiling is
// derived from the batch. DeriveMaxOutputTokens existed with no production caller,
// so every request carried the provider's default and a full batch could be cut off
// mid-JSON.
func TestProviderClientAsksForABatchSizedCeiling(t *testing.T) {
	adapter := &fakeAdapter{results: []llm.ChatResult{jsonResult(`{"results":[]}`)}}
	client := &ProviderClient{Adapter: adapter}

	if _, err := client.SuggestTags(context.Background(), TagRequest{
		Batch: posts(DefaultBatchSize, "ada"), Model: "test-model",
	}); err != nil {
		t.Fatalf("SuggestTags: %v", err)
	}

	if len(adapter.requests) != 1 {
		t.Fatalf("made %d requests, want 1", len(adapter.requests))
	}
	ceiling := adapter.requests[0].MaxOutputTokens
	if ceiling == nil {
		t.Fatal("the request carried no output ceiling")
	}
	if want := llm.DeriveMaxOutputTokens(DefaultBatchSize); *ceiling != want {
		t.Errorf("MaxOutputTokens = %d, want %d", *ceiling, want)
	}
	if *ceiling != 1600 {
		t.Errorf("a batch of %d asks for %d tokens, want 1600", DefaultBatchSize, *ceiling)
	}
}

// TestProviderClientAccumulatesUsage is the other half of item 11: the numbers the
// provider reports reach the run summary.
func TestProviderClientAccumulatesUsage(t *testing.T) {
	first := jsonResult(`{"results":[]}`)
	first.Usage = llm.TokenUsage{InputTokens: 100, OutputTokens: 10}
	// A reply that does not parse still cost money: it is counted before parsing.
	second := llm.ChatResult{Text: "not json", Usage: llm.TokenUsage{InputTokens: 50, OutputTokens: 5}}

	adapter := &fakeAdapter{results: []llm.ChatResult{first, second}}
	client := &ProviderClient{Adapter: adapter}

	request := TagRequest{Batch: []domain.Bookmark{{TweetID: "1", Text: "x"}}}
	if _, err := client.SuggestTags(context.Background(), request); err != nil {
		t.Fatalf("first SuggestTags: %v", err)
	}
	if _, err := client.SuggestTags(context.Background(), request); err == nil {
		t.Fatal("the unparseable reply should have failed")
	}

	if got, want := client.Usage(), (llm.TokenUsage{InputTokens: 150, OutputTokens: 15}); got != want {
		t.Errorf("usage = %+v, want %+v", got, want)
	}
}

// TestProviderClientDropsARejectedParameter covers the recovery that stops an
// unusable run: a provider that rejects `temperature` will reject it forever, so
// the parameter is dropped and the request retried once.
func TestProviderClientDropsARejectedParameter(t *testing.T) {
	adapter := &fakeAdapter{
		errs: []error{
			&llm.LlmError{Kind: llm.KindUnsupportedParam, DroppedOption: llm.DropTemperature, Message: "no temperature"},
			nil,
		},
		results: []llm.ChatResult{
			{},
			jsonResult(`{"results":[{"tweetId":"1","tags":[{"name":"Go","confidence":0.5}]}]}`),
		},
	}
	client := &ProviderClient{
		Adapter:      adapter,
		Capabilities: llm.ModelCapabilities{JsonStrategy: llm.JsonSchemaStrict, Temperature: true},
	}

	verdicts, err := client.SuggestTags(context.Background(), TagRequest{
		Batch: []domain.Bookmark{{TweetID: "1", Text: "x"}},
	})
	if err != nil {
		t.Fatalf("SuggestTags: %v", err)
	}
	if len(verdicts) != 1 {
		t.Errorf("verdicts = %+v", verdicts)
	}
	if len(adapter.requests) != 2 {
		t.Errorf("made %d requests, want the original plus one retry", len(adapter.requests))
	}
	// The retry must no longer ask for a temperature.
	if adapter.requests[1].Temperature != nil {
		t.Error("the retry must drop the rejected temperature")
	}
}

func TestProviderClientDegradesARejectedSchema(t *testing.T) {
	adapter := &fakeAdapter{
		errs: []error{
			&llm.LlmError{Kind: llm.KindUnsupportedParam, DroppedOption: llm.DropJSONSchema},
			nil,
		},
		results: []llm.ChatResult{
			{},
			jsonResult(`{"results":[{"tweetId":"1","tags":[{"name":"Go","confidence":0.5}]}]}`),
		},
	}
	client := &ProviderClient{
		Adapter:      adapter,
		Capabilities: llm.ModelCapabilities{JsonStrategy: llm.JsonSchemaStrict, Temperature: true},
	}

	if _, err := client.SuggestTags(context.Background(), TagRequest{
		Batch: []domain.Bookmark{{TweetID: "1", Text: "x"}},
	}); err != nil {
		t.Fatalf("SuggestTags: %v", err)
	}

	// The client's contract is the capability set it hands the adapter; moving
	// the schema into the prompt is the adapter's job, and is covered by the llm
	// package's assembly tests.
	if len(adapter.envs) != 2 {
		t.Fatalf("recorded %d environments, want 2", len(adapter.envs))
	}
	if adapter.envs[0].Capabilities == nil || adapter.envs[0].Capabilities.JsonStrategy != llm.JsonSchemaStrict {
		t.Error("the first attempt should use the original strategy")
	}
	if adapter.envs[1].Capabilities == nil || adapter.envs[1].Capabilities.JsonStrategy != llm.JsonPromptOnly {
		t.Errorf("the retry must fall back to prompt-only, got %+v", adapter.envs[1].Capabilities)
	}
}

// TestProviderClientRetriesAnUnusableReplyOnce pins the count: retrying an
// unparseable answer once usually works, and retrying it repeatedly just burns
// quota.
func TestProviderClientRetriesAnUnusableReplyOnce(t *testing.T) {
	adapter := &fakeAdapter{results: []llm.ChatResult{
		{Text: "not json at all"},
		{Text: "still not json"},
		{Text: "never"},
	}}
	client := &ProviderClient{Adapter: adapter}

	if _, err := client.SuggestTags(context.Background(), TagRequest{
		Batch: []domain.Bookmark{{TweetID: "1", Text: "x"}},
	}); err == nil {
		t.Fatal("expected a failure")
	}
	if len(adapter.requests) != 2 {
		t.Errorf("made %d requests, want exactly 2", len(adapter.requests))
	}
}

func TestProviderClientDoesNotRetryTerminalErrors(t *testing.T) {
	for _, kind := range []llm.ErrorKind{llm.KindUnauthorized, llm.KindQuota, llm.KindContentFilter} {
		t.Run(kind, func(t *testing.T) {
			adapter := &fakeAdapter{errs: []error{&llm.LlmError{Kind: kind}}}
			client := &ProviderClient{Adapter: adapter}

			if _, err := client.SuggestTags(context.Background(), TagRequest{
				Batch: []domain.Bookmark{{TweetID: "1", Text: "x"}},
			}); err == nil {
				t.Fatal("expected the error to surface")
			}
			if len(adapter.requests) != 1 {
				t.Errorf("%q made %d requests, want exactly 1", kind, len(adapter.requests))
			}
		})
	}
}

func TestProviderClientNeedsAnAdapter(t *testing.T) {
	client := &ProviderClient{}
	if _, err := client.SuggestTags(context.Background(), TagRequest{}); err == nil {
		t.Error("a client with no adapter must fail rather than panic")
	}
}

func TestVerdictSchemaIsObjectRooted(t *testing.T) {
	schema := VerdictSchema()
	if schema["type"] != "object" {
		t.Errorf("schema type = %v, want object: providers reject a non-object root", schema["type"])
	}
	if _, ok := schema["properties"].(map[string]any)["results"]; !ok {
		t.Error("the schema must describe the results array")
	}
}

// TestBuildProviderClientResolvesFromSettings covers the run-time wiring: the
// provider, its endpoint and the Keychain credential are resolved when the run
// starts, so a settings change applies to the next run.
func TestBuildProviderClientResolvesFromSettings(t *testing.T) {
	t.Run("a disabled feature builds nothing", func(t *testing.T) {
		client, err := BuildProviderClient(Settings{}, nil)
		if err != nil {
			t.Fatalf("a disabled feature must not be an error: %v", err)
		}
		if client != nil {
			t.Error("a disabled feature must not build a client")
		}
	})

	t.Run("an unknown provider is refused rather than falling back", func(t *testing.T) {
		settings := DefaultSettings()
		settings.LLM.Enabled = true
		settings.LLM.ProviderID = "not-a-provider"

		// ProviderByID falls back to the first provider, so this is the case the
		// factory has to catch: silently using OpenAI for a typo would spend the
		// wrong account's quota.
		if _, err := BuildProviderClient(settings, func(context.Context, string) (string, error) {
			return "key", nil
		}); err == nil {
			t.Error("an unknown provider must be refused")
		}
	})

	t.Run("a missing credential is reported", func(t *testing.T) {
		settings := DefaultSettings()
		settings.LLM.Enabled = true
		settings.LLM.ProviderID = "openai"

		_, err := BuildProviderClient(settings, func(context.Context, string) (string, error) {
			return "", nil
		})
		if err == nil {
			t.Fatal("a provider that needs a key must fail without one")
		}
		// The message is the generic one: the provider travels as its own field in the
		// connection-test result, so the message does not repeat it.
		if !strings.Contains(err.Error(), "no API key") {
			t.Errorf("the error must say the key is missing: %v", err)
		}
	})

	t.Run("the credential is read for the selected provider", func(t *testing.T) {
		settings := DefaultSettings()
		settings.LLM.Enabled = true
		settings.LLM.ProviderID = "anthropic"
		settings.LLM.Model = "claude-3-5-sonnet"

		var asked string
		client, err := BuildProviderClient(settings, func(_ context.Context, providerID string) (string, error) {
			asked = providerID
			return "sk-test", nil
		})
		if err != nil {
			t.Fatalf("BuildProviderClient: %v", err)
		}
		if client == nil {
			t.Fatal("a client must be built")
		}
		if asked != "anthropic" {
			t.Errorf("asked for %q, want anthropic", asked)
		}
	})

	t.Run("a custom endpoint is required where the provider has none", func(t *testing.T) {
		settings := DefaultSettings()
		settings.LLM.Enabled = true
		settings.LLM.ProviderID = "custom"
		settings.LLM.BaseURL = ""

		if _, err := BuildProviderClient(settings, func(context.Context, string) (string, error) {
			return "key", nil
		}); err == nil {
			t.Error("a provider with no default endpoint must fail without one")
		}

		// With one supplied it builds.
		settings.LLM.BaseURL = "https://example.test/v1"
		if _, err := BuildProviderClient(settings, func(context.Context, string) (string, error) {
			return "key", nil
		}); err != nil {
			t.Errorf("a supplied endpoint must be accepted: %v", err)
		}
	})
}

// TestConnectionReportsAMissingKey covers the case the panel must explain without
// spending a request: nothing was sent, so there is no status to show.
func TestConnectionReportsAMissingKey(t *testing.T) {
	settings := DefaultSettings()
	settings.LLM.ProviderID = "openai"
	settings.LLM.Model = "gpt-4o"

	result := TestProviderConnection(settings, func(context.Context, string) (string, error) {
		return "", nil
	})

	if result.OK {
		t.Fatal("a test with no key must not report success")
	}
	if result.ProviderID != "openai" {
		t.Errorf("providerId = %q, want openai", result.ProviderID)
	}
	if result.ErrorKind != "unauthorized" {
		t.Errorf("errorKind = %q, want unauthorized", result.ErrorKind)
	}
	if result.Message != MissingCredentialMessage {
		t.Errorf("message = %q, want the missing-credential message", result.Message)
	}
}

// TestConnectionReportsAnUnknownProvider keeps a typo from being tested against the
// wrong provider, which is what ProviderByID's fallback would otherwise do.
func TestConnectionReportsAnUnknownProvider(t *testing.T) {
	settings := DefaultSettings()
	settings.LLM.ProviderID = "not-a-provider"

	result := TestProviderConnection(settings, func(context.Context, string) (string, error) {
		return "key", nil
	})
	if result.OK {
		t.Fatal("an unknown provider must not report success")
	}
	if !strings.Contains(result.Message, "not-a-provider") {
		t.Errorf("the message must name the bad id: %q", result.Message)
	}
}

// TestConnectionReportsAProviderNeedingAnEndpoint covers the custom provider, whose
// endpoint cannot be guessed.
func TestConnectionReportsAProviderNeedingAnEndpoint(t *testing.T) {
	settings := DefaultSettings()
	settings.LLM.ProviderID = "custom"
	settings.LLM.BaseURL = ""

	result := TestProviderConnection(settings, func(context.Context, string) (string, error) {
		return "key", nil
	})
	if result.OK {
		t.Fatal("a provider with no endpoint must not report success")
	}
	if !strings.Contains(strings.ToLower(result.Message), "endpoint") {
		t.Errorf("the message must explain the missing endpoint: %q", result.Message)
	}
}

// TestConnectionSendsNothingWithoutAKey pins that the probe is not attempted when
// there is no credential, so a wrong key cannot be charged for.
func TestConnectionSendsNothingWithoutAKey(t *testing.T) {
	settings := DefaultSettings()
	settings.LLM.ProviderID = "anthropic"

	calls := 0
	_ = TestProviderConnection(settings, func(context.Context, string) (string, error) {
		calls++
		return "", nil
	})

	if calls != 1 {
		t.Errorf("the credential was read %d times, want once", calls)
	}
	// There is no way to observe the probe from here without a server, so the
	// assertion is on the credential path: the result is a missing-key result, which
	// is only reachable before the probe.
	result := TestProviderConnection(settings, func(context.Context, string) (string, error) { return "", nil })
	if result.ErrorKind != "unauthorized" {
		t.Errorf("errorKind = %q, want unauthorized", result.ErrorKind)
	}
}

// TestTheCustomProviderSendsTheStoredKey pins the defect a live endpoint exposed.
//
// "api-key-or-none" describes whether a key is required, not whether it is used. The
// first implementation read it as "do not read a key", so every custom endpoint was
// called unauthenticated: a proxy that does require a key answered 401, and the panel
// told the user their valid Keychain key was invalid.
func TestTheCustomProviderSendsTheStoredKey(t *testing.T) {
	var mu sync.Mutex
	var auths, paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		auths = append(auths, r.Header.Get("Authorization"))
		paths = append(paths, r.URL.Path)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"object":"list","data":[]}`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"ok\":true}"},"finish_reason":"stop"}]}`))
	}))
	defer server.Close()

	settings := DefaultSettings()
	settings.LLM.ProviderID = "custom"
	settings.LLM.BaseURL = server.URL + "/v1"
	settings.LLM.Model = "test-model"

	result := TestProviderConnection(settings, func(context.Context, string) (string, error) {
		// The store trims when it reads; a resolver that does not must not leak
		// whitespace into the header either.
		return "  sk-test\n", nil
	})

	if !result.OK {
		t.Fatalf("a keyed custom endpoint must pass: %+v", result)
	}
	// Two calls now: the listing and the one real completion. Both must carry the
	// stored key -- the completion is the one a run actually needs.
	if len(paths) != 2 {
		t.Fatalf("made %d requests, want the listing and one completion: %v", len(paths), paths)
	}
	if paths[0] != "/v1/models" || paths[1] != "/v1/chat/completions" {
		t.Errorf("paths = %v, want the listing then a completion", paths)
	}
	for i, auth := range auths {
		if auth != "Bearer sk-test" {
			t.Errorf("request %d Authorization = %q, want the stored key", i, auth)
		}
	}
}

// TestTheCustomProviderMayRunWithoutAKey keeps the other half of the same rule: an
// endpoint that accepts no authentication is still a valid configuration.
func TestTheCustomProviderMayRunWithoutAKey(t *testing.T) {
	var mu sync.Mutex
	var auths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		auths = append(auths, r.Header.Get("Authorization"))
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"object":"list","data":[]}`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"ok\":true}"},"finish_reason":"stop"}]}`))
	}))
	defer server.Close()

	settings := DefaultSettings()
	settings.LLM.ProviderID = "custom"
	settings.LLM.BaseURL = server.URL + "/v1"
	settings.LLM.Model = "test-model"

	result := TestProviderConnection(settings, func(context.Context, string) (string, error) {
		return "", nil
	})

	if !result.OK {
		t.Fatalf("a keyless custom endpoint must pass: %+v", result)
	}
	if len(auths) != 2 {
		t.Errorf("made %d requests, want the listing and one completion", len(auths))
	}
	for _, auth := range auths {
		if auth != "" {
			t.Errorf("Authorization = %q, want none", auth)
		}
	}
}

// TestConnectionMakesARealRequest is item 12's assertion.
//
// A key that can list models is not a key that can complete: the /models endpoint
// answers 200 for a misspelled model name, and the panel used to report "connected"
// for a configuration every run would fail on. The failure must show the provider's
// own sentence, because that is what names the typo.
func TestConnectionMakesARealRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"gpt-4o"}]}`))
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("{\"error\":{\"message\":\"The model 'gpt-4o-typo' does not exist\",\"type\":\"invalid_request_error\"}}"))
	}))
	defer server.Close()

	settings := DefaultSettings()
	settings.LLM.ProviderID = "custom"
	settings.LLM.BaseURL = server.URL + "/v1"
	settings.LLM.Model = "gpt-4o-typo"

	result := TestProviderConnection(settings, func(context.Context, string) (string, error) {
		return "sk-test", nil
	})

	if result.OK {
		t.Fatal("a model the provider does not have must not report a working connection")
	}
	if result.ErrorKind != string(llm.KindBadRequest) {
		t.Errorf("errorKind = %q, want %q", result.ErrorKind, llm.KindBadRequest)
	}
	if !strings.Contains(result.Message, "gpt-4o-typo") {
		t.Errorf("message = %q, want the provider's own sentence naming the model", result.Message)
	}
}

// TestAConfigurationErrorDoesNotBlameTheKey separates two failures the panel used to
// show with one sentence. A missing endpoint is not an invalid key, and saying so sent
// the user to replace a credential that was never the problem.
func TestAConfigurationErrorDoesNotBlameTheKey(t *testing.T) {
	settings := DefaultSettings()
	settings.LLM.ProviderID = "custom"
	settings.LLM.BaseURL = ""

	result := TestProviderConnection(settings, func(context.Context, string) (string, error) {
		return "sk-test", nil
	})

	if result.OK {
		t.Fatal("a provider with no endpoint must not report success")
	}
	if result.ErrorKind == string(llm.KindUnauthorized) {
		t.Error("a missing endpoint must not be reported as an invalid key")
	}
	if result.Message == MissingCredentialMessage {
		t.Error("the message must name the real problem, not the key")
	}
}

// TestConnectionTestAndClientShareResolution is the property that makes the test
// worth having: a test that resolved the provider differently from the run could pass
// while every run failed.
func TestConnectionTestAndClientShareResolution(t *testing.T) {
	settings := DefaultSettings()
	// Enabled, or the client build short-circuits before resolving anything -- which
	// is the disabled-feature path, not the one under test.
	settings.LLM.Enabled = true
	settings.LLM.ProviderID = "openai"
	settings.LLM.Model = "gpt-4o"
	settings.LLM.BaseURL = "https://example.test/v1"

	// A key that cannot be read: both paths must fail the same way.
	broken := func(context.Context, string) (string, error) {
		return "", errStub("the keychain is locked")
	}

	result := TestProviderConnection(settings, broken)
	if _, err := BuildProviderClient(settings, broken); err == nil {
		t.Fatal("the client build must fail when the credential read fails")
	}
	if result.OK {
		t.Error("the connection test must fail when the credential read fails")
	}
	if !strings.Contains(result.Message, "keychain is locked") {
		t.Errorf("the test must report the real reason: %q", result.Message)
	}
}

// TestThePromptTheSchemaAndTheParserAgreeOnOneShape ties three things together.
//
// The parser, the prompt and the schema could drift apart without anything failing: the
// parser only reads what a provider returns, and the provider answers what it was asked.
// This asserts the three agree on one shape.
func TestThePromptTheSchemaAndTheParserAgreeOnOneShape(t *testing.T) {
	prompt := BuildPrompt(TagRequest{
		Batch:      []domain.Bookmark{{TweetID: "1", AuthorHandle: "ada", Text: "x"}},
		Vocabulary: []VocabEntry{{Name: "go", Count: 1}},
	})

	// The prompt asks for a per-tag confidence, and says so.
	if !strings.Contains(prompt, `"tags":[{"name":`) {
		t.Errorf("the prompt does not ask for a per-tag confidence:\n%s", prompt)
	}
	if !strings.Contains(prompt, "confidence between 0 and 1 for each tag") {
		t.Errorf("the prompt does not state the per-tag instruction:\n%s", prompt)
	}
	if strings.Contains(prompt, `"tags":["`) {
		t.Errorf("the prompt still shows tags as bare strings:\n%s", prompt)
	}

	// The schema constrains the same shape.
	schema, err := json.Marshal(VerdictSchema())
	if err != nil {
		t.Fatalf("marshal schema: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal(schema, &result); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	// The shape is nested: results.items.properties.tags.items.properties.
	rootProps, _ := result["properties"].(map[string]any)
	results, ok := rootProps["results"].(map[string]any)
	if !ok {
		t.Fatalf("the schema has no results: %v", result)
	}
	entry, _ := results["items"].(map[string]any)
	entryProps, _ := entry["properties"].(map[string]any)
	tags, ok := entryProps["tags"].(map[string]any)
	if !ok {
		t.Fatalf("the schema has no tags property: %v", entryProps)
	}
	if tags["type"] != "array" {
		t.Fatalf("tags is %v, want an array", tags["type"])
	}
	items, ok := tags["items"].(map[string]any)
	if !ok {
		t.Fatalf("tags items is %T, want an object schema: the schema still takes bare strings",
			tags["items"])
	}
	itemProps, ok := items["properties"].(map[string]any)
	if !ok {
		t.Fatalf("tags items has no properties: %v", items)
	}
	if _, ok := itemProps["confidence"]; !ok {
		t.Errorf("the per-tag schema has no confidence: %v", itemProps)
	}

	// And the parser accepts exactly that.
	parsed, err := ParseVerdicts(jsonResult(`{"results":[
		{"tweetId":"1","tags":[{"name":"go","confidence":0.8}]}]}`),
		[]domain.Bookmark{{TweetID: "1"}})
	if err != nil || len(parsed) != 1 || parsed[0].Tags[0].Confidence != 0.8 {
		t.Errorf("the parser did not accept the shape the prompt asks for: %+v, %v", parsed, err)
	}
}

// TestAFailureDoesNotCarryTheKey closes the loop between the transport and the run.
//
// The adapter sends the key in a header; the provider quotes the header back in a 401. The
// error that leaves this package is what the pipeline writes into its summary, so the
// secret must already be gone here.
func TestAFailureDoesNotCarryTheKey(t *testing.T) {
	const key = "sk-testredaction-0123456789"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"the key was rejected; header was ` +
			r.Header.Get("Authorization") + `"}}`))
	}))
	defer server.Close()

	settings := DefaultSettings()
	settings.LLM.Enabled = true
	settings.LLM.ProviderID = "custom"
	settings.LLM.BaseURL = server.URL + "/v1"
	settings.LLM.Model = "gpt-4o"

	client, err := BuildProviderClient(settings, func(context.Context, string) (string, error) {
		return key, nil
	})
	if err != nil {
		t.Fatalf("BuildProviderClient: %v", err)
	}

	_, err = client.SuggestTags(context.Background(), TagRequest{
		Model: "gpt-4o",
		Batch: []domain.Bookmark{{TweetID: "1", Text: "a post"}},
	})
	if err == nil {
		t.Fatal("a 401 must fail the batch")
	}
	if strings.Contains(err.Error(), key) {
		t.Errorf("the failure carries the credential: %q", err.Error())
	}
	if !strings.Contains(err.Error(), "[redacted]") {
		t.Errorf("the failure does not mark the removal: %q", err.Error())
	}
}
