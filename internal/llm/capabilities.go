package llm

import (
	"net/http"
	"regexp"
	"strings"
	"time"
)

// JsonStrategy is how a request asks for JSON output.
type JsonStrategy = string

const (
	// JsonSchemaStrict asks the provider to enforce a schema.
	JsonSchemaStrict JsonStrategy = "json-schema-strict"
	// JsonSchema asks for schema-shaped output without a strict guarantee.
	JsonSchema JsonStrategy = "json-schema"
	// JsonObject asks only for valid JSON.
	JsonObject JsonStrategy = "json-object"
	// JsonTool obtains JSON through a forced tool call, which is how Anthropic
	// does it.
	JsonTool JsonStrategy = "tool"
	// JsonPromptOnly puts the schema in the prompt and hopes.
	JsonPromptOnly JsonStrategy = "prompt-only"
)

// AllJsonStrategies is used for validation.
var AllJsonStrategies = []JsonStrategy{
	JsonSchemaStrict, JsonSchema, JsonObject, JsonTool, JsonPromptOnly,
}

// JsonMode is how strongly JSON is required.
type JsonMode = string

const (
	JsonRequire JsonMode = "require"
	JsonPrefer  JsonMode = "prefer"
)

// DroppedOption names a parameter that was left out of a request.
type DroppedOption = string

const (
	DropTemperature     DroppedOption = "temperature"
	DropJSONSchema      DroppedOption = "jsonSchema"
	DropMaxOutputTokens DroppedOption = "maxOutputTokens"
)

// ModelCapabilities describes what a model supports.
//
// The zero value is not useful, so callers always go through ResolveCapabilities,
// which fills a default for anything unspecified.
type ModelCapabilities struct {
	JsonStrategy JsonStrategy
	Temperature  bool
	// MaxOutputTokens and ContextWindow are optional overrides.
	MaxOutputTokens *int
	ContextWindow   *int
}

// providerDefaults is the fallback capability set per API.
var providerDefaults = map[string]ModelCapabilities{
	APIOpenAIChat:         {JsonStrategy: JsonSchemaStrict, Temperature: true},
	APIAnthropicMessages:  {JsonStrategy: JsonTool, Temperature: true},
	APIGoogleGenerativeAI: {JsonStrategy: JsonObject, Temperature: true},
	APIAzureOpenAIChat:    {JsonStrategy: JsonSchemaStrict, Temperature: true},
}

// Prefix rules capture the models that reject a parameter.
//
// They exist because the failure is silent: a gateway that rejects
// `temperature` returns an error, and without a rule the same request is retried
// forever with the same parameter.
var prefixRules = []struct {
	api   string
	model *regexp.Regexp
	apply func(*ModelCapabilities)
}{
	{APIOpenAIChat, regexp.MustCompile(`^o[1-9]`), func(c *ModelCapabilities) { c.Temperature = false }},
	{APIOpenAIChat, regexp.MustCompile(`^gpt-5`), func(c *ModelCapabilities) { c.Temperature = false }},
	{APIAzureOpenAIChat, regexp.MustCompile(`^o[1-9]`), func(c *ModelCapabilities) { c.Temperature = false }},
	{APIAzureOpenAIChat, regexp.MustCompile(`^gpt-5`), func(c *ModelCapabilities) { c.Temperature = false }},
	{APIAnthropicMessages, regexp.MustCompile(`^claude-`), func(c *ModelCapabilities) { c.JsonStrategy = JsonTool }},
}

// LearnedCapabilityKey names the slot a learned capability is remembered under.
func LearnedCapabilityKey(providerID, model string) string {
	return strings.TrimSpace(providerID) + "::" + strings.TrimSpace(model)
}

// IsKnownJsonStrategy reports whether a stored strategy is one this build understands.
func IsKnownJsonStrategy(strategy JsonStrategy) bool {
	for _, candidate := range AllJsonStrategies {
		if strategy == candidate {
			return true
		}
	}
	return false
}

// ResolveCapabilities applies the four-level priority:
//
//  1. the user's explicit override
//  2. what a previous failure taught us about this endpoint
//  3. a prefix rule for the model
//  4. the provider's default
//
// Higher levels win field by field, not wholesale, so an override of just
// `temperature` keeps the provider's JSON strategy.
func ResolveCapabilities(api, model string, overrides, learned *ModelCapabilities) ModelCapabilities {
	resolved, ok := providerDefaults[api]
	if !ok {
		resolved = ModelCapabilities{JsonStrategy: JsonObject, Temperature: true}
	}

	trimmedModel := strings.TrimSpace(model)
	for _, rule := range prefixRules {
		if rule.api != api {
			continue
		}
		if rule.model.MatchString(trimmedModel) {
			rule.apply(&resolved)
			break // first match wins, as in the reference
		}
	}

	apply := func(source *ModelCapabilities) {
		if source == nil {
			return
		}
		if source.JsonStrategy != "" {
			resolved.JsonStrategy = source.JsonStrategy
		}
		// A boolean has no "unset", so learning and overriding are expressed by
		// setting Temperature to the value that is correct for the endpoint.
		resolved.Temperature = source.Temperature
		if source.MaxOutputTokens != nil {
			resolved.MaxOutputTokens = source.MaxOutputTokens
		}
		if source.ContextWindow != nil {
			resolved.ContextWindow = source.ContextWindow
		}
	}

	apply(learned)
	apply(overrides)

	return resolved
}

// ChatRequest is one completion request, independent of provider shape.
type ChatRequest struct {
	// System is an optional system instruction.
	System string
	// Prompt is the user message.
	Prompt string
	// JSONSchema, when set, describes the expected object.
	JSONSchema map[string]any
	// JSONMode says whether JSON is required or merely preferred.
	JSONMode JsonMode
	// Temperature is a pointer so "unset" and "zero" stay distinct.
	Temperature *float64
	// MaxOutputTokens bounds the response.
	MaxOutputTokens *int
	// DeadlineMs is the caller's remaining budget, in milliseconds.
	DeadlineMs int
}

// AssembledRequest is a request with the capabilities applied.
type AssembledRequest struct {
	ChatRequest
	Capabilities ModelCapabilities
	// Temperature is non-nil only when the model accepts it.
	Temperature *float64
	// JSONSchema is non-nil only when the strategy can send one.
	JSONSchema map[string]any
	// PromptIncludesSchema means the schema had to move into the prompt.
	PromptIncludesSchema bool
}

// AssembleRequest applies capabilities to a request and reports what was left out.
//
// Two rules do the work: a request with no schema cannot use a schema strategy,
// and a model that cannot accept a temperature must not be sent one.
func AssembleRequest(capabilities ModelCapabilities, request ChatRequest) (AssembledRequest, []DroppedOption) {
	// The embedded request must be carried through: dropping it would send an
	// empty prompt, since every adapter reads Prompt/System from here.
	assembled := AssembledRequest{ChatRequest: request, Capabilities: capabilities}
	dropped := []DroppedOption{}

	strategy := capabilities.JsonStrategy
	if request.JSONSchema == nil {
		// Nothing to enforce, so asking for schema output is pointless.
		strategy = JsonPromptOnly
	}
	assembled.Capabilities.JsonStrategy = strategy

	if request.JSONSchema != nil {
		if strategy == JsonPromptOnly {
			// The schema travels in the prompt instead; the caller has to render
			// it there.
			assembled.PromptIncludesSchema = true
			dropped = append(dropped, DropJSONSchema)
		} else {
			assembled.JSONSchema = request.JSONSchema
		}
	}

	if capabilities.Temperature {
		temperature := request.Temperature
		if temperature == nil {
			zero := 0.0
			temperature = &zero
		}
		assembled.Temperature = temperature
	} else if request.Temperature != nil {
		// Only report the drop when the caller actually asked for one.
		dropped = append(dropped, DropTemperature)
	}

	if request.MaxOutputTokens != nil {
		assembled.MaxOutputTokens = request.MaxOutputTokens
		if capabilities.ContextWindow != nil && *request.MaxOutputTokens > *capabilities.ContextWindow {
			clamped := *capabilities.ContextWindow
			assembled.MaxOutputTokens = &clamped
			dropped = append(dropped, DropMaxOutputTokens)
		}
	}

	return assembled, dropped
}

// capabilitiesFor picks the capability set to apply.
//
// A caller-supplied set wins, because it carries what a previous rejection
// taught us; resolving afresh would send the rejected parameter again.
func capabilitiesFor(api, model string, supplied, overrides, learned *ModelCapabilities) ModelCapabilities {
	if supplied != nil && supplied.JsonStrategy != "" {
		resolved := *supplied
		if resolved.MaxOutputTokens == nil {
			resolved.MaxOutputTokens = supplied.MaxOutputTokens
		}
		return resolved
	}
	// Both overrides and learned capabilities are passed through.
	return ResolveCapabilities(api, model, overrides, learned)
}

// DeriveMaxOutputTokens bounds a response from the batch size.
//
// Roughly 64 tokens per bookmark, floored so a single-item batch is still
// workable and ceilinged so one request cannot ask for an unbounded response.
func DeriveMaxOutputTokens(batchSize int) int {
	tokens := batchSize * 64
	if tokens < 256 {
		return 256
	}
	if tokens > 4096 {
		return 4096
	}
	return tokens
}

// AdapterEnv is everything an adapter needs to make a request.
type AdapterEnv struct {
	// Model is the model id, needed by providers that put it in the path.
	Model string
	// Capabilities, when set, overrides what the adapter would resolve for
	// itself. It is how a parameter the provider already rejected stays dropped
	// across the retry, and how a user override reaches the wire.
	Capabilities *ModelCapabilities
	// Overrides and Learned are the user's own answer and what a previous probe found,
	// which ResolveCapabilities applies below Model and above the built-in defaults.
	// They are separate from Capabilities because Capabilities is already resolved:
	// folding these in at the call site would lose the priority order.
	Overrides      *ModelCapabilities
	Learned        *ModelCapabilities
	APIKey         string
	BaseURL        string
	ExtraHeaders   map[string]string
	DeploymentName string
	APIVersion     string
	HTTPClient     *http.Client
	// Timeout bounds one request.
	Timeout time.Duration
	// Sleep is injectable so retry tests do not wait.
	Sleep func(time.Duration)
}

// DefaultRequestTimeout bounds a single provider request.
const DefaultRequestTimeout = 60 * time.Second

// Retry and backoff constants.
const (
	DefaultRateLimitBackoff = 2 * time.Second
	MaxRateLimitBackoff     = 30 * time.Second
	DefaultRateLimitRetries = 2
	MaxTransportAttempts    = 5
	// MaxRetryAfter caps how long a server-requested wait is honoured. A larger
	// value is reported instead of slept through, so a bogus header cannot park
	// the app.
	MaxRetryAfter = 30 * time.Second
)
