package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

type stubTransport func(*http.Request) (*http.Response, error)

func (f stubTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func jsonResponse(status int, body string, header map[string]string) *http.Response {
	resp := &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{},
	}
	for k, v := range header {
		resp.Header.Set(k, v)
	}
	return resp
}

// recorder captures what the adapter sent.
type recorder struct {
	mu      sync.Mutex
	urls    []string
	headers []http.Header
	bodies  []string
}

func (r *recorder) capture(req *http.Request) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.urls = append(r.urls, req.URL.String())
	r.headers = append(r.headers, req.Header.Clone())
	if req.Body != nil {
		raw, _ := io.ReadAll(req.Body)
		r.bodies = append(r.bodies, string(raw))
	} else {
		r.bodies = append(r.bodies, "")
	}
}

func (r *recorder) lastURL() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.urls) == 0 {
		return ""
	}
	return r.urls[len(r.urls)-1]
}

func (r *recorder) lastBody(t *testing.T) map[string]any {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.bodies) == 0 {
		t.Fatal("no request body was captured")
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(r.bodies[len(r.bodies)-1]), &decoded); err != nil {
		t.Fatalf("captured body is not JSON: %v (%s)", err, r.bodies[len(r.bodies)-1])
	}
	return decoded
}

func (r *recorder) callCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.urls)
}

// responseQueue answers with a scripted sequence, repeating the last entry.
func responseQueue(responses ...*http.Response) (stubTransport, *int) {
	index := 0
	calls := 0
	return func(*http.Request) (*http.Response, error) {
		calls++
		position := index
		if position >= len(responses) {
			position = len(responses) - 1
		}
		index++
		return responses[position], nil
	}, &calls
}

func noSleep(*time.Duration) func(time.Duration) { return func(time.Duration) {} }

// --- transport retry semantics ---

func TestTransportRetriesRateLimits(t *testing.T) {
	var slept []time.Duration

	transport, calls := responseQueue(
		jsonResponse(429, `{}`, map[string]string{"Retry-After": "3"}),
		jsonResponse(200, `{"ok":true}`, nil),
	)
	_ = transport

	client := &http.Client{Transport: transport}
	engine := &Transport{Client: client, Sleep: func(d time.Duration) { slept = append(slept, d) }}

	body, err := engine.doJSON(context.Background(), requestSpec{Method: http.MethodGet, URL: "https://x.test/v1"})
	if err != nil {
		t.Fatalf("doJSON: %v", err)
	}
	if !strings.Contains(string(body), "ok") {
		t.Errorf("body = %q", body)
	}
	if *calls != 2 {
		t.Errorf("made %d attempts, want 2", *calls)
	}
	if len(slept) != 1 || slept[0] != 3*time.Second {
		t.Errorf("slept %v, want the server-requested 3s", slept)
	}
}

// TestTransportReportsAnAbsurdRetryAfter pins the spec's choice: a header asking
// for longer than the cap is reported rather than slept through, so it cannot
// park the app.
func TestTransportReportsAnAbsurdRetryAfter(t *testing.T) {
	var slept []time.Duration

	transport, calls := responseQueue(jsonResponse(429, `{}`, map[string]string{"Retry-After": "86400"}))
	engine := &Transport{
		Client: &http.Client{Transport: transport},
		Sleep:  func(d time.Duration) { slept = append(slept, d) },
	}

	_, err := engine.doJSON(context.Background(), requestSpec{Method: http.MethodGet, URL: "https://x.test/v1"})
	if err == nil {
		t.Fatal("expected an error")
	}
	if len(slept) != 0 {
		t.Errorf("slept %v, want none: a day-long wait must be reported instead", slept)
	}
	if *calls != 1 {
		t.Errorf("made %d attempts, want 1", *calls)
	}
	if got := ErrorKindOf(err); got != KindRateLimit {
		t.Errorf("kind = %q, want %q", got, KindRateLimit)
	}
	if err.RetryAfter != 24*time.Hour {
		t.Errorf("RetryAfter = %v, want the value surfaced", err.RetryAfter)
	}
}

// TestTransportGivesOneBlindRetryForServerErrors pins the semantics: exactly one
// blind retry for a transient failure. That count is deliberate, so it is preserved
// rather than quietly changed.
func TestTransportGivesOneBlindRetryForServerErrors(t *testing.T) {
	var slept []time.Duration

	transport, calls := responseQueue(jsonResponse(503, `{}`, nil))
	engine := &Transport{
		Client: &http.Client{Transport: transport},
		Sleep:  func(d time.Duration) { slept = append(slept, d) },
	}

	_, err := engine.doJSON(context.Background(), requestSpec{Method: http.MethodGet, URL: "https://x.test/v1"})
	if err == nil {
		t.Fatal("expected the retries to run out")
	}
	if got := ErrorKindOf(err); got != KindServer {
		t.Errorf("kind = %q, want %q", got, KindServer)
	}
	if *calls != 2 {
		t.Errorf("made %d attempts, want the original attempt plus one blind retry", *calls)
	}
	if len(slept) != 1 {
		t.Errorf("slept %d times, want 1", len(slept))
	}
}

// TestTransportDoesNotRetryBadCredentials states the rule the spec is explicit
// about: an invalid key will fail identically forever.
func TestTransportDoesNotRetryBadCredentials(t *testing.T) {
	var slept []time.Duration

	transport, calls := responseQueue(jsonResponse(401, `{"error":"bad key"}`, nil))
	engine := &Transport{
		Client: &http.Client{Transport: transport},
		Sleep:  func(d time.Duration) { slept = append(slept, d) },
	}

	_, err := engine.doJSON(context.Background(), requestSpec{Method: http.MethodGet, URL: "https://x.test/v1"})
	if err == nil {
		t.Fatal("expected an error")
	}
	if got := ErrorKindOf(err); got != KindUnauthorized {
		t.Errorf("kind = %q, want %q", got, KindUnauthorized)
	}
	if *calls != 1 {
		t.Errorf("made %d attempts, want exactly 1", *calls)
	}
	if len(slept) != 0 {
		t.Errorf("slept %v, want none", slept)
	}
}

// TestTransportHonoursShouldRetryHeader covers the gateway header that overrides
// the status code's meaning.
func TestTransportHonoursShouldRetryHeader(t *testing.T) {
	t.Run("false suppresses a retry", func(t *testing.T) {
		transport, calls := responseQueue(
			jsonResponse(429, `{}`, map[string]string{"x-should-retry": "false"}),
		)
		engine := &Transport{Client: &http.Client{Transport: transport}, Sleep: noSleep(nil)}

		_, err := engine.doJSON(context.Background(), requestSpec{Method: http.MethodGet, URL: "https://x.test/v1"})
		if err == nil {
			t.Fatal("expected an error")
		}
		if *calls != 1 {
			t.Errorf("made %d attempts, want 1: the header said not to retry", *calls)
		}
	})

	t.Run("true forces a retry of an otherwise terminal status", func(t *testing.T) {
		transport, calls := responseQueue(
			jsonResponse(400, `{}`, map[string]string{"x-should-retry": "true"}),
			jsonResponse(200, `{"ok":true}`, nil),
		)
		engine := &Transport{Client: &http.Client{Transport: transport}, Sleep: noSleep(nil)}

		if _, err := engine.doJSON(context.Background(), requestSpec{Method: http.MethodGet, URL: "https://x.test/v1"}); err != nil {
			t.Fatalf("doJSON: %v", err)
		}
		if *calls != 2 {
			t.Errorf("made %d attempts, want 2", *calls)
		}
	})
}

func TestTransportDropsARejectedParameterAndRetries(t *testing.T) {
	var dropped []DroppedOption

	transport, calls := responseQueue(
		jsonResponse(400, `{"error":"unsupported parameter temperature"}`, nil),
		jsonResponse(200, `{"ok":true}`, nil),
	)
	engine := &Transport{Client: &http.Client{Transport: transport}, Sleep: noSleep(nil)}

	_, err := engine.doJSON(context.Background(), requestSpec{
		Method: http.MethodPost, URL: "https://x.test/v1",
		OnUnsupportedParam: func(option DroppedOption) bool {
			dropped = append(dropped, option)
			return true // drop it and try again
		},
	})
	if err != nil {
		t.Fatalf("doJSON: %v", err)
	}
	if *calls != 2 {
		t.Errorf("made %d attempts, want 2", *calls)
	}
	if len(dropped) != 1 || dropped[0] != DropTemperature {
		t.Errorf("dropped = %v, want [temperature]", dropped)
	}
}

func TestTransportRejectsOversizedProviderResponse(t *testing.T) {
	big := strings.Repeat("x", maxProviderResponseBytes+10)
	transport, _ := responseQueue(jsonResponse(200, big, nil))
	engine := &Transport{Client: &http.Client{Transport: transport}, Sleep: noSleep(nil)}

	_, err := engine.doJSON(context.Background(), requestSpec{Method: http.MethodGet, URL: "https://x.test/v1"})
	if err == nil {
		t.Fatal("an oversized body must be refused")
	}
	if !strings.Contains(err.Error(), "exceeded") {
		t.Errorf("err = %v", err)
	}
}

func TestTransportRespectsContextCancellation(t *testing.T) {
	transport, _ := responseQueue(jsonResponse(200, `{}`, nil))
	engine := &Transport{Client: &http.Client{Transport: transport}, Sleep: noSleep(nil)}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := engine.doJSON(ctx, requestSpec{Method: http.MethodGet, URL: "https://x.test/v1"}); err == nil {
		t.Fatal("a cancelled context must not perform a request")
	}
}

// --- adapter wire details ---

func adapterClient(recorder *recorder, response *http.Response) *http.Client {
	return &http.Client{Transport: stubTransport(func(r *http.Request) (*http.Response, error) {
		recorder.capture(r)
		return response, nil
	})}
}

func TestOpenAIAdapterRequest(t *testing.T) {
	recorder := &recorder{}
	adapter, _ := AdapterFor(APIOpenAIChat)

	result, err := adapter.ChatJSON(context.Background(), ChatRequest{
		System: "be terse", Prompt: "tag this",
	}, AdapterEnv{
		Model:      "gpt-4o-mini",
		APIKey:     "sk-test-key",
		BaseURL:    "https://api.openai.com/v1",
		HTTPClient: adapterClient(recorder, jsonResponse(200, `{"choices":[{"message":{"content":"{\"a\":1}"},"finish_reason":"stop"}]}`, nil)),
	})
	if err != nil {
		t.Fatalf("ChatJSON: %v", err)
	}

	if !strings.HasSuffix(recorder.lastURL(), "/v1/chat/completions") {
		t.Errorf("url = %q", recorder.lastURL())
	}
	if got := recorder.headers[0].Get("authorization"); got != "Bearer sk-test-key" {
		t.Errorf("authorization = %q", got)
	}

	body := recorder.lastBody(t)
	// The model is what makes this an OpenAI-compatible request at all: an endpoint
	// answers "a valid model must be specified" without it, while /models still answers
	// 200, which is how the settings test could pass while every run failed.
	if got, _ := body["model"].(string); got != "gpt-4o-mini" {
		t.Errorf("model = %q, want gpt-4o-mini", got)
	}
	messages, _ := body["messages"].([]any)
	if len(messages) != 2 {
		t.Fatalf("messages = %v, want a system and a user message", messages)
	}
	if result.Parsed == nil || result.Parsed["a"] != float64(1) {
		t.Errorf("parsed = %#v", result.Parsed)
	}
	if result.FinishReason != "stop" {
		t.Errorf("finish reason = %q", result.FinishReason)
	}
}

// TestOpenAIAdapterIsKeylessFriendly covers a local server with no auth: sending
// an empty Authorization header would make it fail.
func TestOpenAIAdapterIsKeylessFriendly(t *testing.T) {
	recorder := &recorder{}
	adapter, _ := AdapterFor(APIOpenAIChat)

	if _, err := adapter.ChatJSON(context.Background(), ChatRequest{Prompt: "hi"}, AdapterEnv{
		Model:      "local",
		BaseURL:    "http://localhost:11434",
		HTTPClient: adapterClient(recorder, jsonResponse(200, `{"choices":[{"message":{"content":"{}"}}]}`, nil)),
	}); err != nil {
		t.Fatalf("ChatJSON: %v", err)
	}

	if got := recorder.headers[0].Get("authorization"); got != "" {
		t.Errorf("authorization = %q, want no header at all for a keyless endpoint", got)
	}
}

func TestAnthropicAdapterUsesAToolCall(t *testing.T) {
	recorder := &recorder{}
	adapter, _ := AdapterFor(APIAnthropicMessages)

	response := `{"content":[{"type":"tool_use","name":"emit_tags","input":{"results":[]}}],"stop_reason":"tool_use"}`
	result, err := adapter.ChatJSON(context.Background(), ChatRequest{Prompt: "tag this"}, AdapterEnv{
		Model:      "claude-3-5-sonnet",
		APIKey:     "sk-ant-test",
		BaseURL:    "https://api.anthropic.com",
		HTTPClient: adapterClient(recorder, jsonResponse(200, response, nil)),
	})
	if err != nil {
		t.Fatalf("ChatJSON: %v", err)
	}

	if !strings.HasSuffix(recorder.lastURL(), "/v1/messages") {
		t.Errorf("url = %q", recorder.lastURL())
	}
	headers := recorder.headers[0]
	if headers.Get("x-api-key") != "sk-ant-test" {
		t.Errorf("x-api-key = %q", headers.Get("x-api-key"))
	}
	if headers.Get("anthropic-version") == "" {
		t.Error("anthropic-version is required by the API")
	}

	body := recorder.lastBody(t)
	if body["tool_choice"] == nil {
		t.Error("the schema must travel as a forced tool call")
	}
	if _, ok := body["max_tokens"]; !ok {
		t.Error("max_tokens is required by the Messages API")
	}
	if result.Parsed == nil {
		t.Error("the tool input must be surfaced as the parsed result")
	}
}

// TestAnthropicAdapterUsesTheRequestedCeiling is the fixed 1024 this adapter used to
// send: a full batch of 25 needs room for 25 verdicts, and a reply cut off mid-JSON
// looks like a bad answer rather than a small request.
func TestAnthropicAdapterUsesTheRequestedCeiling(t *testing.T) {
	recorder := &recorder{}
	adapter, _ := AdapterFor(APIAnthropicMessages)
	ceiling := DeriveMaxOutputTokens(25)

	if _, err := adapter.ChatJSON(context.Background(), ChatRequest{
		Prompt: "tag this", MaxOutputTokens: &ceiling,
	}, AdapterEnv{
		Model:  "claude-3-5-sonnet",
		APIKey: "k",
		HTTPClient: adapterClient(recorder, jsonResponse(200,
			`{"content":[{"type":"tool_use","name":"emit_tags","input":{"results":[]}}]}`, nil)),
	}); err != nil {
		t.Fatalf("ChatJSON: %v", err)
	}

	body := recorder.lastBody(t)
	if got, want := asInt(body["max_tokens"]), ceiling; got != want {
		t.Errorf("max_tokens = %d, want %d (the derived ceiling, not the old fixed 1024)", got, want)
	}
	if got := asInt(body["max_tokens"]); got == anthropicDefaultMaxTokens {
		t.Error("the adapter ignored the requested ceiling")
	}
}

// TestAdaptersReportUsage pins the four spellings of the same two numbers. The run
// summary reads them, and a provider that reports nothing must stay at zero rather
// than be guessed at.
func TestAdaptersReportUsage(t *testing.T) {
	tests := []struct {
		name     string
		api      string
		env      AdapterEnv
		response string
		want     TokenUsage
	}{
		{
			name: "openai",
			api:  APIOpenAIChat,
			env:  AdapterEnv{Model: "gpt-4o", APIKey: "k", BaseURL: "https://api.openai.com"},
			response: `{"choices":[{"message":{"content":"{}"}}],
			            "usage":{"prompt_tokens":120,"completion_tokens":45}}`,
			want: TokenUsage{InputTokens: 120, OutputTokens: 45},
		},
		{
			name: "anthropic",
			api:  APIAnthropicMessages,
			env:  AdapterEnv{Model: "claude-3-5-sonnet", APIKey: "k", BaseURL: "https://api.anthropic.com"},
			response: `{"content":[{"type":"tool_use","name":"emit_tags","input":{"results":[]}}],
			            "usage":{"input_tokens":300,"output_tokens":20}}`,
			want: TokenUsage{InputTokens: 300, OutputTokens: 20},
		},
		{
			name: "google",
			api:  APIGoogleGenerativeAI,
			env:  AdapterEnv{Model: "gemini-1.5-flash", APIKey: "k", BaseURL: "https://generativelanguage.googleapis.com"},
			response: `{"candidates":[{"content":{"parts":[{"text":"{}"}]}}],
			            "usageMetadata":{"promptTokenCount":90,"candidatesTokenCount":12}}`,
			want: TokenUsage{InputTokens: 90, OutputTokens: 12},
		},
		{
			name: "azure",
			api:  APIAzureOpenAIChat,
			env: AdapterEnv{Model: "gpt-4o", APIKey: "k", BaseURL: "https://example.openai.azure.com",
				DeploymentName: "my-deployment"},
			response: `{"choices":[{"message":{"content":"{}"}}],
			            "usage":{"prompt_tokens":77,"completion_tokens":8}}`,
			want: TokenUsage{InputTokens: 77, OutputTokens: 8},
		},
		{
			name:     "a provider that reports nothing",
			api:      APIOpenAIChat,
			env:      AdapterEnv{Model: "local", BaseURL: "http://localhost:11434"},
			response: `{"choices":[{"message":{"content":"{}"}}]}`,
			want:     TokenUsage{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			recorder := &recorder{}
			adapter, err := AdapterFor(tc.api)
			if err != nil {
				t.Fatalf("AdapterFor: %v", err)
			}
			env := tc.env
			env.HTTPClient = adapterClient(recorder, jsonResponse(200, tc.response, nil))

			result, err := adapter.ChatJSON(context.Background(), ChatRequest{Prompt: "hi"}, env)
			if err != nil {
				t.Fatalf("ChatJSON: %v", err)
			}
			if result.Usage != tc.want {
				t.Errorf("usage = %+v, want %+v", result.Usage, tc.want)
			}
		})
	}
}

func TestAnthropicAdapterRejectsAMissingToolCall(t *testing.T) {
	recorder := &recorder{}
	adapter, _ := AdapterFor(APIAnthropicMessages)

	_, err := adapter.ChatJSON(context.Background(), ChatRequest{Prompt: "hi"}, AdapterEnv{
		Model:      "claude-3",
		APIKey:     "k",
		HTTPClient: adapterClient(recorder, jsonResponse(200, `{"content":[{"type":"text","text":"nope"}]}`, nil)),
	})
	if err == nil {
		t.Fatal("a response without the tool call must fail rather than read as an empty result")
	}
	if got := ErrorKindOf(err); got != KindInvalidJSON {
		t.Errorf("kind = %q, want %q", got, KindInvalidJSON)
	}
}

func TestGoogleAdapterPutsTheKeyInTheQuery(t *testing.T) {
	recorder := &recorder{}
	adapter, _ := AdapterFor(APIGoogleGenerativeAI)

	response := `{"candidates":[{"content":{"parts":[{"text":"{\"a\":1}"}]},"finishReason":"STOP"}]}`
	result, err := adapter.ChatJSON(context.Background(), ChatRequest{Prompt: "hi"}, AdapterEnv{
		Model:      "gemini-1.5-flash",
		APIKey:     "AIza-test-key",
		HTTPClient: adapterClient(recorder, jsonResponse(200, response, nil)),
	})
	if err != nil {
		t.Fatalf("ChatJSON: %v", err)
	}

	url := recorder.lastURL()
	if !strings.Contains(url, "v1beta/models/gemini-1.5-flash:generateContent") {
		t.Errorf("url = %q", url)
	}
	if !strings.Contains(url, "key=AIza-test-key") {
		t.Errorf("url = %q, want the key in the query as the API expects", url)
	}
	// Which is exactly why any error path must redact URLs.
	if redacted := redactBaseUrl(url); strings.Contains(redacted, "AIza-test-key") {
		t.Errorf("redaction failed on the adapter's own URL: %q", redacted)
	}

	body := recorder.lastBody(t)
	config, _ := body["generationConfig"].(map[string]any)
	if config["responseMimeType"] != "application/json" {
		t.Errorf("generationConfig = %v, want a JSON mime type", config)
	}
	if result.Parsed == nil {
		t.Error("candidate text must be parsed")
	}
}

func TestAzureAdapterRouting(t *testing.T) {
	recorder := &recorder{}
	adapter, _ := AdapterFor(APIAzureOpenAIChat)

	_, err := adapter.ChatJSON(context.Background(), ChatRequest{Prompt: "hi"}, AdapterEnv{
		Model:          "gpt-4o",
		APIKey:         "az-key",
		BaseURL:        "https://example.openai.azure.com",
		DeploymentName: "my-deployment",
		HTTPClient:     adapterClient(recorder, jsonResponse(200, `{"choices":[{"message":{"content":"{}"}}]}`, nil)),
	})
	if err != nil {
		t.Fatalf("ChatJSON: %v", err)
	}

	url := recorder.lastURL()
	if !strings.Contains(url, "/openai/deployments/") {
		t.Errorf("url = %q, want a deployment path", url)
	}
	if !strings.Contains(url, "api-version=") {
		t.Errorf("url = %q, want an api-version", url)
	}
	if got := recorder.headers[0].Get("api-key"); got != "az-key" {
		t.Errorf("api-key = %q, want Azure's header rather than a bearer token", got)
	}
}

// TestAzureAdapterRequiresADeployment is a configuration error, not a request
// error, so it must fail before anything is sent.
func TestAzureAdapterRequiresADeployment(t *testing.T) {
	recorder := &recorder{}
	adapter, _ := AdapterFor(APIAzureOpenAIChat)

	_, err := adapter.ChatJSON(context.Background(), ChatRequest{Prompt: "hi"}, AdapterEnv{
		Model:      "gpt-4o",
		APIKey:     "az-key",
		BaseURL:    "https://example.openai.azure.com",
		HTTPClient: adapterClient(recorder, jsonResponse(200, `{}`, nil)),
	})
	if err == nil {
		t.Fatal("a missing deployment name must be refused")
	}
	if got := ErrorKindOf(err); got != KindBadRequest {
		t.Errorf("kind = %q, want %q", got, KindBadRequest)
	}
	if recorder.callCount() != 0 {
		t.Error("nothing must be sent when the configuration is incomplete")
	}
}

func TestAdapterProbesReportCredentialFailures(t *testing.T) {
	tests := []struct {
		name string
		api  string
		env  AdapterEnv
	}{
		{"openai", APIOpenAIChat, AdapterEnv{APIKey: "k", BaseURL: "https://api.openai.com/v1"}},
		{"anthropic", APIAnthropicMessages, AdapterEnv{APIKey: "k", BaseURL: "https://api.anthropic.com"}},
		{"google", APIGoogleGenerativeAI, AdapterEnv{APIKey: "k"}},
		{"azure", APIAzureOpenAIChat, AdapterEnv{APIKey: "k", BaseURL: "https://example.openai.azure.com"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			adapter, err := AdapterFor(tc.api)
			if err != nil {
				t.Fatalf("AdapterFor: %v", err)
			}

			env := tc.env
			env.HTTPClient = &http.Client{Transport: stubTransport(func(*http.Request) (*http.Response, error) {
				return jsonResponse(401, `{}`, nil), nil
			})}
			if err := adapter.Probe(env); err == nil {
				t.Error("a 401 must fail the probe")
			} else if got := ErrorKindOf(err); got != KindUnauthorized {
				t.Errorf("kind = %q, want %q", got, KindUnauthorized)
			}

			env.HTTPClient = &http.Client{Transport: stubTransport(func(*http.Request) (*http.Response, error) {
				return jsonResponse(200, `{}`, nil), nil
			})}
			if err := adapter.Probe(env); err != nil {
				t.Errorf("a 200 must pass the probe: %v", err)
			}
		})
	}
}
