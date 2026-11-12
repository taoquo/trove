package llm

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestErrorKindInventory(t *testing.T) {
	// The auto-tagger's control flow keys off the taxonomy, and the spec pins it
	// at thirteen kinds.
	if len(AllErrorKinds) != 13 {
		t.Errorf("AllErrorKinds has %d entries, want 13", len(AllErrorKinds))
	}
	seen := map[ErrorKind]bool{}
	for _, kind := range AllErrorKinds {
		if seen[kind] {
			t.Errorf("duplicate kind %q", kind)
		}
		seen[kind] = true
	}
	for _, expected := range []ErrorKind{
		KindUnauthorized, KindForbidden, KindRateLimit, KindQuota, KindBadRequest,
		KindUnsupportedParam, KindContextLength, KindTimeout, KindNetwork,
		KindServer, KindContentFilter, KindInvalidJSON, KindUnknown,
	} {
		if !seen[expected] {
			t.Errorf("kind %q is missing", expected)
		}
	}
}

func TestClassifyStatus(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   ErrorKind
	}{
		{"401", 401, `{}`, KindUnauthorized},
		{"402 is a quota signal", 402, `{}`, KindQuota},
		{"403 mentioning billing is a quota signal", 403, `{"error":"insufficient credits"}`, KindQuota},
		{"403 otherwise is forbidden", 403, `{"error":"nope"}`, KindForbidden},
		{"429", 429, `{}`, KindRateLimit},
		{"500", 500, `{}`, KindServer},
		{"503", 503, `{}`, KindServer},
		{"400 with a content filter", 400, `{"error":"content policy violation"}`, KindContentFilter},
		{"400 with an unsupported parameter", 400, `{"error":"unsupported parameter temperature"}`, KindUnsupportedParam},
		{"400 with a context problem", 400, `{"error":"maximum context length exceeded"}`, KindContextLength},
		{"400 otherwise", 400, `{"error":"bad input"}`, KindBadRequest},
		{"404", 404, `{}`, KindBadRequest},
		{"422", 422, `{}`, KindBadRequest},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyStatus(tc.status, tc.body); got != tc.want {
				t.Errorf("classifyStatus(%d, %q) = %q, want %q", tc.status, tc.body, got, tc.want)
			}
		})
	}
}

func TestDroppedOptionDetection(t *testing.T) {
	tests := []struct {
		name   string
		status int
		detail string
		want   DroppedOption
	}{
		{"temperature", 400, `{"error":"Unsupported value: temperature"}`, DropTemperature},
		{"response_format", 400, `{"error":"unknown parameter response_format"}`, DropJSONSchema},
		{"json_object", 400, `{"error":"json_object is not supported"}`, DropJSONSchema},
		{"max_tokens", 400, `{"error":"max_tokens too large"}`, DropMaxOutputTokens},
		{"nothing recognisable", 400, `{"error":"whatever"}`, ""},
		{"temperature on a 500 is not a parameter problem", 500, `temperature`, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := droppedOptionFrom(tc.status, tc.detail); got != tc.want {
				t.Errorf("droppedOptionFrom(%d, %q) = %q, want %q", tc.status, tc.detail, got, tc.want)
			}
		})
	}
}

// TestRedactionHidesCredentials covers the leak this port must not reintroduce:
// the Google adapter puts the API key in the query string, so anything that logs
// a request URL logs the key.
func TestRedactionHidesCredentials(t *testing.T) {
	const key = "AIzaSyFAKE-KEY-1234567890"

	t.Run("query string credentials", func(t *testing.T) {
		raw := "Get \"https://generativelanguage.googleapis.com/v1beta/models/gemini:generateContent?key=" + key + "\": dial tcp: timeout"
		redacted := redactBaseUrl(raw)
		if strings.Contains(redacted, key) {
			t.Errorf("redactBaseUrl leaked the key: %q", redacted)
		}
		if !strings.Contains(redacted, "[redacted]") {
			t.Errorf("redactBaseUrl did not mark the removal: %q", redacted)
		}
	})

	t.Run("known secret anywhere in the text", func(t *testing.T) {
		redacted := redactSecret("request failed for sk-"+key, "sk-"+key)
		if strings.Contains(redacted, key) {
			t.Errorf("redactSecret leaked the secret: %q", redacted)
		}
	})

	t.Run("a short secret is left alone", func(t *testing.T) {
		// Replacing two-character strings would mangle ordinary text without
		// protecting anything.
		short := "ab"
		if got := redactSecret("about ab", short); got != "about ab" {
			t.Errorf("redactSecret(%q) = %q, want it untouched", short, got)
		}
	})

	t.Run("URL credentials in several spellings", func(t *testing.T) {
		for _, param := range []string{"key", "api_key", "api-key", "access_token", "token"} {
			raw := "https://example.com/v1?x=1&" + param + "=" + key + "&y=2"
			if got := redactBaseUrl(raw); strings.Contains(got, key) {
				t.Errorf("param %q was not redacted: %q", param, got)
			}
		}
	})
}

// TestSanitizeLlmErrorUnwrapsURL verifies the specific Go hazard: a *url.Error
// renders its URL, which is how a key ends up in a log.
func TestSanitizeLlmErrorUnwrapsURL(t *testing.T) {
	const key = "AIzaSyFAKE-KEY-1234567890"

	inner := errors.New("dial tcp: i/o timeout")
	wrapped := &url.Error{
		Op:  "Get",
		URL: "https://generativelanguage.googleapis.com/v1beta/models/x?key=" + key,
		Err: inner,
	}

	sanitized := SanitizeLlmError(wrapped, key)
	if strings.Contains(sanitized.Error(), key) {
		t.Errorf("sanitized error leaks the key: %q", sanitized.Error())
	}
}

func TestIsRetryable(t *testing.T) {
	retryable := []ErrorKind{KindRateLimit, KindNetwork, KindTimeout, KindServer}
	for _, kind := range retryable {
		if !IsRetryable(&LlmError{Kind: kind}) {
			t.Errorf("%q must be retryable", kind)
		}
	}
	// A bad credential fails identically forever, so retrying only burns quota.
	notRetryable := []ErrorKind{
		KindUnauthorized, KindForbidden, KindQuota, KindBadRequest,
		KindUnsupportedParam, KindContextLength, KindContentFilter,
		KindInvalidJSON, KindUnknown,
	}
	for _, kind := range notRetryable {
		if IsRetryable(&LlmError{Kind: kind}) {
			t.Errorf("%q must not be retryable", kind)
		}
	}
}

func TestIsRateLimitFallsBackToWording(t *testing.T) {
	if !IsRateLimit(&LlmError{Kind: KindRateLimit}) {
		t.Error("the kind must be recognised")
	}
	// The older engine reported rate limits through the message.
	if !IsRateLimit(errors.New("HTTP 429 too many requests")) {
		t.Error("a 429 in the message must be recognised")
	}
	if !IsRateLimit(errors.New("Rate Limit exceeded")) {
		t.Error("rate-limit wording must be recognised")
	}
	if IsRateLimit(errors.New("connection refused")) {
		t.Error("an unrelated error must not read as a rate limit")
	}
}

func TestResolveCapabilitiesPriority(t *testing.T) {
	intPtr := func(v int) *int { return &v }

	t.Run("provider defaults", func(t *testing.T) {
		caps := ResolveCapabilities(APIOpenAIChat, "gpt-4o-mini", nil, nil)
		if caps.JsonStrategy != JsonSchemaStrict || !caps.Temperature {
			t.Errorf("got %+v, want the OpenAI defaults", caps)
		}
		caps = ResolveCapabilities(APIAnthropicMessages, "claude-3", nil, nil)
		if caps.JsonStrategy != JsonTool {
			t.Errorf("Anthropic must default to a tool call, got %q", caps.JsonStrategy)
		}
		caps = ResolveCapabilities(APIGoogleGenerativeAI, "gemini-pro", nil, nil)
		if caps.JsonStrategy != JsonObject {
			t.Errorf("Google must default to json-object, got %q", caps.JsonStrategy)
		}
	})

	t.Run("prefix rules turn temperature off", func(t *testing.T) {
		for _, model := range []string{"o1-preview", "o3-mini", "gpt-5", "gpt-5.1"} {
			if caps := ResolveCapabilities(APIOpenAIChat, model, nil, nil); caps.Temperature {
				t.Errorf("%q must not accept temperature", model)
			}
		}
		if caps := ResolveCapabilities(APIOpenAIChat, "gpt-4o", nil, nil); !caps.Temperature {
			t.Error("gpt-4o does accept temperature")
		}
		// A model that merely starts with "o" is not a reasoning model.
		if caps := ResolveCapabilities(APIOpenAIChat, "openchat", nil, nil); !caps.Temperature {
			t.Error("the prefix rule must be anchored to o followed by a digit")
		}
	})

	t.Run("learned beats the prefix rule", func(t *testing.T) {
		learned := &ModelCapabilities{Temperature: true}
		if caps := ResolveCapabilities(APIOpenAIChat, "o1", nil, learned); !caps.Temperature {
			t.Error("a learned capability must override a prefix rule")
		}
	})

	t.Run("override beats learned", func(t *testing.T) {
		learned := &ModelCapabilities{Temperature: true}
		override := &ModelCapabilities{Temperature: false}
		if caps := ResolveCapabilities(APIOpenAIChat, "o1", override, learned); caps.Temperature {
			t.Error("an explicit override must win")
		}
	})

	t.Run("resolution is field by field", func(t *testing.T) {
		// An override that only mentions maxOutputTokens must not reset the JSON
		// strategy back to the default.
		override := &ModelCapabilities{MaxOutputTokens: intPtr(2048)}
		caps := ResolveCapabilities(APIAnthropicMessages, "claude-3", override, nil)
		if caps.JsonStrategy != JsonTool {
			t.Errorf("JsonStrategy = %q, want the Anthropic default preserved", caps.JsonStrategy)
		}
		if caps.MaxOutputTokens == nil || *caps.MaxOutputTokens != 2048 {
			t.Errorf("MaxOutputTokens = %v, want 2048", caps.MaxOutputTokens)
		}
	})

	t.Run("an unknown API still resolves", func(t *testing.T) {
		caps := ResolveCapabilities("invented-api", "m", nil, nil)
		if caps.JsonStrategy == "" {
			t.Error("an unknown API must still produce a usable capability set")
		}
	})
}

func TestLearnedCapabilityKey(t *testing.T) {
	if got := LearnedCapabilityKey("openai", "gpt-4o"); got != "openai::gpt-4o" {
		t.Errorf("key = %q", got)
	}
	if LearnedCapabilityKey("a", "b") == LearnedCapabilityKey("a", "c") {
		t.Error("different models must have different keys")
	}
}

func TestAssembleRequest(t *testing.T) {
	t.Run("a request with no schema cannot use a schema strategy", func(t *testing.T) {
		caps := ModelCapabilities{JsonStrategy: JsonSchemaStrict, Temperature: true}
		assembled, dropped := AssembleRequest(caps, ChatRequest{Prompt: "hi"})

		if assembled.Capabilities.JsonStrategy != JsonPromptOnly {
			t.Errorf("strategy = %q, want prompt-only", assembled.Capabilities.JsonStrategy)
		}
		if assembled.JSONSchema != nil {
			t.Error("no schema must not be sent")
		}
		if len(dropped) != 0 {
			t.Errorf("dropped = %v, want nothing: the caller never asked for a schema", dropped)
		}
	})

	t.Run("a prompt-only strategy moves the schema into the prompt", func(t *testing.T) {
		caps := ModelCapabilities{JsonStrategy: JsonPromptOnly, Temperature: true}
		schema := map[string]any{"type": "object"}
		assembled, dropped := AssembleRequest(caps, ChatRequest{Prompt: "hi", JSONSchema: schema})

		if !assembled.PromptIncludesSchema {
			t.Error("the schema must be marked for the prompt")
		}
		if !contains(dropped, DropJSONSchema) {
			t.Errorf("dropped = %v, want the schema moved out of the request", dropped)
		}
	})

	t.Run("temperature is only sent when supported", func(t *testing.T) {
		supported := ModelCapabilities{JsonStrategy: JsonObject, Temperature: true}
		assembled, _ := AssembleRequest(supported, ChatRequest{Prompt: "hi"})
		if assembled.Temperature == nil {
			t.Error("a supported temperature must be defaulted, not omitted")
		}

		unsupported := ModelCapabilities{JsonStrategy: JsonObject, Temperature: false}
		assembled, dropped := AssembleRequest(unsupported, ChatRequest{Prompt: "hi", Temperature: floatPtr(0.7)})
		if assembled.Temperature != nil {
			t.Error("an unsupported temperature must not be sent")
		}
		if !contains(dropped, DropTemperature) {
			t.Errorf("dropped = %v, want temperature recorded", dropped)
		}

		// Nothing to report when the caller never asked for one.
		_, dropped = AssembleRequest(unsupported, ChatRequest{Prompt: "hi"})
		if contains(dropped, DropTemperature) {
			t.Error("an unrequested temperature must not be reported as dropped")
		}
	})

	t.Run("maxOutputTokens is clamped to the context window", func(t *testing.T) {
		window := 1000
		caps := ModelCapabilities{JsonStrategy: JsonObject, Temperature: true, ContextWindow: &window}
		requested := 5000
		assembled, dropped := AssembleRequest(caps, ChatRequest{Prompt: "hi", MaxOutputTokens: &requested})

		if assembled.MaxOutputTokens == nil || *assembled.MaxOutputTokens != 1000 {
			t.Errorf("MaxOutputTokens = %v, want it clamped to 1000", assembled.MaxOutputTokens)
		}
		if !contains(dropped, DropMaxOutputTokens) {
			t.Errorf("dropped = %v, want the clamp recorded", dropped)
		}
	})
}

func TestDeriveMaxOutputTokens(t *testing.T) {
	tests := []struct {
		batch int
		want  int
	}{
		{1, 256},    // floored
		{4, 256},    // 4*64 = 256
		{25, 1600},  // the default batch size
		{64, 4096},  // 64*64 = 4096
		{200, 4096}, // ceilinged
	}
	for _, tc := range tests {
		if got := DeriveMaxOutputTokens(tc.batch); got != tc.want {
			t.Errorf("DeriveMaxOutputTokens(%d) = %d, want %d", tc.batch, got, tc.want)
		}
	}
}

func TestProviderInventory(t *testing.T) {
	if len(Providers) != 5 {
		t.Fatalf("Providers has %d entries, want 5", len(Providers))
	}

	ids := map[string]ProviderDefinition{}
	for _, provider := range Providers {
		if provider.ID == "" || provider.API == "" || provider.Name == "" {
			t.Errorf("provider %+v is incomplete", provider)
		}
		ids[provider.ID] = provider
	}
	for _, expected := range []string{"openai", "anthropic", "google", "azure-openai", "custom"} {
		if _, ok := ids[expected]; !ok {
			t.Errorf("provider %q is missing", expected)
		}
	}

	if _, ok := ids["openai"]; ok && ids["openai"].DefaultBaseURL != "https://api.openai.com/v1" {
		t.Errorf("openai base URL = %q", ids["openai"].DefaultBaseURL)
	}
	if ids["azure-openai"].CredentialKind != CredentialAzureDeployment {
		t.Error("Azure needs a deployment name")
	}
	if !ids["azure-openai"].NeedsDeployment || !ids["azure-openai"].NeedsAPIVersion {
		t.Error("Azure needs both a deployment and an API version")
	}
	if ids["custom"].CredentialKind != CredentialAPIKeyOrNone {
		t.Error("a custom endpoint may legitimately have no key")
	}
}

func TestProviderByIDFallsBack(t *testing.T) {
	if got := ProviderByID("anthropic"); got.ID != "anthropic" {
		t.Errorf("got %q", got.ID)
	}
	// An id from a newer build must not brick the app.
	if got := ProviderByID("invented"); got.ID != Providers[0].ID {
		t.Errorf("an unknown id must fall back to %q, got %q", Providers[0].ID, got.ID)
	}
	if got := ProviderByID("  "); got.ID != Providers[0].ID {
		t.Errorf("a blank id must fall back, got %q", got.ID)
	}
}

func TestBaseURLHelpers(t *testing.T) {
	provider := ProviderByID("openai")

	if got := ResolveBaseURL(provider, "  https://proxy.example/v1 "); got != "https://proxy.example/v1" {
		t.Errorf("a configured URL must win and be trimmed, got %q", got)
	}
	if got := ResolveBaseURL(provider, ""); got != provider.DefaultBaseURL {
		t.Errorf("an empty URL must fall back to the default, got %q", got)
	}

	cases := []struct{ base, path, want string }{
		{"https://a.com", "v1/messages", "https://a.com/v1/messages"},
		{"https://a.com/", "/v1/messages", "https://a.com/v1/messages"},
		{"https://a.com/v1/", "chat", "https://a.com/v1/chat"},
	}
	for _, tc := range cases {
		if got := JoinURL(tc.base, tc.path); got != tc.want {
			t.Errorf("JoinURL(%q, %q) = %q, want %q", tc.base, tc.path, got, tc.want)
		}
	}

	// A bare host gets /v1 appended, which is the common local-server case.
	if got := ResolveOpenAIBaseURL("http://localhost:8080"); got != "http://localhost:8080/v1" {
		t.Errorf("ResolveOpenAIBaseURL = %q", got)
	}
	if got := ResolveOpenAIBaseURL("http://localhost:8080/v1"); got != "http://localhost:8080/v1" {
		t.Errorf("an existing path must be kept, got %q", got)
	}
}

func TestValidateBaseURL(t *testing.T) {
	ok := []string{
		"https://api.openai.com/v1",
		"http://localhost:11434",
		"http://127.0.0.1:8080",
		"http://[::1]:8080",
	}
	for _, raw := range ok {
		if err := ValidateBaseURL(raw); err != nil {
			t.Errorf("ValidateBaseURL(%q) = %v, want it accepted", raw, err)
		}
	}

	rejected := []struct {
		raw  string
		kind BaseURLErrorKind
	}{
		{"", BaseURLEmpty},
		{"   ", BaseURLEmpty},
		{"http://api.openai.com/v1", BaseURLProtocol}, // plain http to a remote host
		{"ftp://example.com", BaseURLProtocol},
		{"not a url", BaseURLInvalid},
	}
	for _, tc := range rejected {
		err := ValidateBaseURL(tc.raw)
		if err == nil {
			t.Errorf("ValidateBaseURL(%q) must be rejected", tc.raw)
			continue
		}
		var baseErr *BaseURLError
		if !errors.As(err, &baseErr) || baseErr.Kind != tc.kind {
			t.Errorf("ValidateBaseURL(%q) = %v, want kind %q", tc.raw, err, tc.kind)
		}
	}
}

func TestPermissionOrigin(t *testing.T) {
	got, err := PermissionOrigin("https://api.openai.com/v1")
	if err != nil {
		t.Fatalf("PermissionOrigin: %v", err)
	}
	if got != "https://api.openai.com/*" {
		t.Errorf("origin = %q", got)
	}
	if _, err := PermissionOrigin("nonsense"); err == nil {
		t.Error("an invalid URL must be rejected")
	}
}

func TestAdapterRegistry(t *testing.T) {
	// Four request shapes, shared by five providers.
	if len(adapterRegistry) != 4 {
		t.Errorf("registry has %d adapters, want 4", len(adapterRegistry))
	}
	for _, api := range []string{APIOpenAIChat, APIAnthropicMessages, APIGoogleGenerativeAI, APIAzureOpenAIChat} {
		adapter, err := AdapterFor(api)
		if err != nil {
			t.Errorf("AdapterFor(%q): %v", api, err)
			continue
		}
		if adapter.ID() != api {
			t.Errorf("adapter for %q reports id %q", api, adapter.ID())
		}
	}

	if _, err := AdapterFor("invented"); err == nil {
		t.Error("an unknown API must not resolve to an adapter")
	}

	// Every provider must map to a registered adapter, or selecting it would
	// fail at runtime.
	for _, provider := range Providers {
		if _, err := AdapterFor(provider.API); err != nil {
			t.Errorf("provider %q maps to unregistered API %q", provider.ID, provider.API)
		}
	}

	// A keyless local server is a supported configuration.
	openai, _ := AdapterFor(APIOpenAIChat)
	if openai.NeedsAPIKey() {
		t.Error("the OpenAI shape must allow a keyless endpoint")
	}
	for _, api := range []string{APIAnthropicMessages, APIGoogleGenerativeAI, APIAzureOpenAIChat} {
		adapter, _ := AdapterFor(api)
		if !adapter.NeedsAPIKey() {
			t.Errorf("%q must require a key", api)
		}
	}
}

func TestParseJSONLoose(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"bare object", `{"a":1}`, true},
		{"fenced", "```json\n{\"a\":1}\n```", true},
		{"fenced without a tag", "```\n{\"a\":1}\n```", true},
		{"prose around it", "Sure! Here you go:\n{\"a\":1}\nHope that helps.", true},
		{"bare array", `[1,2,3]`, true},
		{"nested braces", `{"a":{"b":{"c":1}}}`, true},
		{"a brace inside a string", `{"a":"}"}`, true},
		{"no json at all", "I cannot help with that.", false},
		{"empty", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseJSONLoose(tc.input)
			if tc.want && err != nil {
				t.Errorf("ParseJSONLoose(%q) failed: %v", tc.input, err)
			}
			if !tc.want && err == nil {
				t.Errorf("ParseJSONLoose(%q) unexpectedly succeeded", tc.input)
			}
		})
	}

	// A brace inside a string must not end the slice early.
	parsed, err := ParseJSONLoose(`{"text":"a } b","n":2}`)
	if err != nil {
		t.Fatalf("ParseJSONLoose: %v", err)
	}
	object, ok := parsed.(map[string]any)
	if !ok || object["text"] != "a } b" {
		t.Errorf("parsed = %#v", parsed)
	}
}

func TestParseRetryAfterForms(t *testing.T) {
	if got := parseRetryAfter(""); got != 0 {
		t.Errorf("empty = %v", got)
	}
	if got := parseRetryAfter("7"); got != 7_000_000_000 {
		t.Errorf("seconds = %v", got)
	}
	if got := parseRetryAfter("0"); got != 0 {
		t.Errorf("zero = %v", got)
	}
	if got := parseRetryAfter("-3"); got != 0 {
		t.Errorf("negative = %v", got)
	}
	if got := parseRetryAfter("garbage"); got != 0 {
		t.Errorf("garbage = %v", got)
	}
}

func TestShouldRetryHeader(t *testing.T) {
	if retry, present := shouldRetryHeader(""); present {
		t.Errorf("an absent header must not be present (got %v)", retry)
	}
	if retry, present := shouldRetryHeader("false"); !present || retry {
		t.Error("false must be read as do-not-retry")
	}
	if retry, present := shouldRetryHeader("TRUE"); !present || !retry {
		t.Error("TRUE must be read as retry")
	}
	if _, present := shouldRetryHeader("maybe"); present {
		t.Error("an unparsable value must be ignored rather than guessed")
	}
}

func floatPtr(v float64) *float64 { return &v }

func contains(list []DroppedOption, want DroppedOption) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

// TestResolveOpenAIBaseURLRebuildsFromTheOrigin covers the shapes a user can paste.
//
// The extension returns `${url.origin}/v1` when the path is bare. Appending "/v1" to the
// input instead is the tempting version and it is wrong for anything with a query: the
// input has no trailing slash to trim, so the result is "https://host/?key=1/v1".
func TestResolveOpenAIBaseURLRebuildsFromTheOrigin(t *testing.T) {
	cases := []struct{ in, want string }{
		{"https://api.openai.com", "https://api.openai.com/v1"},
		{"https://api.openai.com/", "https://api.openai.com/v1"},
		{"https://api.openai.com///", "https://api.openai.com/v1"},
		{"  https://api.openai.com/  ", "https://api.openai.com/v1"},
		// The one that breaks an append: a bare path with a query.
		{"https://api.openai.com/?key=1", "https://api.openai.com/v1"},
		// A port survives.
		{"http://localhost:8080/", "http://localhost:8080/v1"},
		// A real path is left alone, minus trailing slashes.
		{"https://proxy.test/openai/v1", "https://proxy.test/openai/v1"},
		{"https://proxy.test/openai/v1/", "https://proxy.test/openai/v1"},
		// Not a URL: returned as-is, so the request fails with something readable.
		{"not a url", "not a url"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := ResolveOpenAIBaseURL(tc.in); got != tc.want {
			t.Errorf("ResolveOpenAIBaseURL(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestParseRetryAfterAcceptsBothForms covers the shapes a server can send.
//
// The extension reads this header with Number() and falls back to Date.parse, so the
// delay-seconds form is a number and not an integer. Parsing it with an integer conversion
// rejects "1.5": the value then falls through to the date branch, fails there too, and the
// wait is dropped -- the client backs off exponentially instead of for the time it was
// asked to.
func TestParseRetryAfterAcceptsBothForms(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name string
		in   string
		want time.Duration
		// tolerance in seconds, for the date form whose value moves with the clock.
		slack time.Duration
	}{
		{"absent", "", 0, 0},
		{"blank", "   ", 0, 0},
		{"seconds", "30", 30 * time.Second, 0},
		{"a fraction of a second", "1.5", 1500 * time.Millisecond, 0},
		{"zero", "0", 0, 0},
		{"negative", "-5", 0, 0},
		{"not a number or a date", "soon", 0, 0},
		{"an HTTP date in the past", now.Add(-time.Hour).UTC().Format(http.TimeFormat), 0, 0},
		{"an HTTP date ahead", now.Add(45 * time.Second).UTC().Format(http.TimeFormat),
			45 * time.Second, 2 * time.Second},
	}
	for _, tc := range cases {
		got := parseRetryAfter(tc.in)
		if got < tc.want-tc.slack || got > tc.want+tc.slack {
			t.Errorf("%s: parseRetryAfter(%q) = %v, want %v", tc.name, tc.in, got, tc.want)
		}
	}
}

// TestDroppedOptionFromMatchesTheExtensionsSpellings covers which refusals are recognised.
//
// The extension matches the JSON family by five spellings, including "json schema" with a
// space. Missing one means the schema stays in the request, is refused again, and the run
// fails with the provider's message instead of softly degrading.
func TestDroppedOptionFromMatchesTheExtensionsSpellings(t *testing.T) {
	cases := []struct {
		detail string
		want   string
	}{
		{"Unsupported parameter: 'temperature'", "temperature"},
		{"response_format is not supported", "jsonSchema"},
		{"Invalid parameter: 'json_object'", "jsonSchema"},
		// The spaced spelling, which is the one that was missing.
		{"json schema is not supported for this model", "jsonSchema"},
		{"json_schema is not supported", "jsonSchema"},
		{"JSON mode is unavailable", "jsonSchema"},
		{"max_tokens is not supported", "maxOutputTokens"},
		{"model not found", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := droppedOptionFrom(400, tc.detail); got != tc.want {
			t.Errorf("droppedOptionFrom(400, %q) = %q, want %q", tc.detail, got, tc.want)
		}
	}

	// A settled status is not a parameter problem, and the extension will not probe it:
	// "a gateway refusing the caller, the model or the route will not change its mind
	// because the next request carries one parameter less."
	for _, status := range []int{200, 500} {
		if got := droppedOptionFrom(status, "temperature is not supported"); got != "" {
			t.Errorf("droppedOptionFrom(%d, …) = %q, want no drop", status, got)
		}
	}
}
