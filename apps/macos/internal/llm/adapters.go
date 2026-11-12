package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/taoquo/x-master/apps/macos/internal/netproxy"
)

// This file holds the four request shapes. They are registered in
// adapterRegistry by init, so AdapterFor can dispatch by API id without a switch
// scattered through the package.

func init() {
	for _, adapter := range []Adapter{
		openAIChatAdapter{},
		anthropicMessagesAdapter{},
		googleGenerativeAIAdapter{},
		azureOpenAIChatAdapter{},
	} {
		adapterRegistry[adapter.ID()] = adapter
	}
}

func transportFor(env AdapterEnv) *Transport {
	return &Transport{Client: env.HTTPClient, Sleep: env.Sleep}
}

func headersWith(base map[string]string, extra ...map[string]string) map[string]string {
	out := map[string]string{"content-type": "application/json"}
	for name, value := range base {
		out[name] = value
	}
	for _, group := range extra {
		for name, value := range group {
			out[name] = value
		}
	}
	return out
}

// encodeBody marshals a request body, reporting a classified error.
func encodeBody(payload any) ([]byte, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, &LlmError{Kind: KindBadRequest, Message: "could not encode the request"}
	}
	return encoded, nil
}

// decodeObject unmarshals a provider response into a generic object.
func decodeObject(body []byte) (map[string]any, error) {
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, InvalidJSONError("response was not a JSON object")
	}
	return decoded, nil
}

// --- OpenAI-compatible ---

// openAIChatAdapter serves openai and, through the custom provider, any
// OpenAI-compatible endpoint.
type openAIChatAdapter struct{}

func (openAIChatAdapter) ID() string { return APIOpenAIChat }

// NeedsAPIKey is false because a local server may be unauthenticated.
func (openAIChatAdapter) NeedsAPIKey() bool { return false }

func (a openAIChatAdapter) ChatJSON(ctx context.Context, request ChatRequest, env AdapterEnv) (ChatResult, error) {
	base := strings.TrimSpace(env.BaseURL)
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	endpoint := JoinURL(ResolveOpenAIBaseURL(base), "chat/completions")

	capabilities := capabilitiesFor(a.ID(), env.Model, env.Capabilities, env.Overrides, env.Learned)
	assembled, dropped := AssembleRequest(capabilities, request)
	prompt := promptWithSchema(assembled)

	messages := make([]map[string]string, 0, 2)
	if assembled.System != "" {
		messages = append(messages, map[string]string{"role": "system", "content": assembled.System})
	}
	messages = append(messages, map[string]string{"role": "user", "content": prompt})

	// The model belongs in the body of an OpenAI-compatible request. It was missing
	// here while the probe passed, because the probe asks /models and never names a
	// model: every chat call answered 400 invalid_model, so the settings test looked
	// healthy and no run could ever tag anything. Azure puts the deployment in the
	// URL and Anthropic puts the model in its body; this is the OpenAI shape.
	payload := map[string]any{"model": modelFromEnv(env), "messages": messages}
	if assembled.Temperature != nil {
		payload["temperature"] = *assembled.Temperature
	}
	if assembled.MaxOutputTokens != nil {
		payload["max_tokens"] = *assembled.MaxOutputTokens
	}
	if assembled.JSONSchema != nil {
		payload["response_format"] = map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   "suggestions",
				"strict": assembled.Capabilities.JsonStrategy == JsonSchemaStrict,
				"schema": assembled.JSONSchema,
			},
		}
	} else if request.JSONMode == JsonRequire {
		payload["response_format"] = map[string]any{"type": "json_object"}
	}

	body, err := encodeBody(payload)
	if err != nil {
		return ChatResult{}, err
	}

	headers := headersWith(nil)
	// A keyless local server must not receive an empty Authorization header.
	if env.APIKey != "" {
		headers["authorization"] = "Bearer " + env.APIKey
	}
	for name, value := range env.ExtraHeaders {
		headers[name] = value
	}

	responseBody, requestErr := transportFor(env).doJSON(ctx, requestSpec{
		Method:     http.MethodPost,
		URL:        endpoint,
		Headers:    headers,
		Body:       body,
		DeadlineMs: request.DeadlineMs,
	})
	if requestErr != nil {
		return ChatResult{}, requestErr
	}

	return parseOpenAIStyleResponse(responseBody, dropped)
}

// --- Anthropic ---

type anthropicMessagesAdapter struct{}

func (anthropicMessagesAdapter) ID() string { return APIAnthropicMessages }

func (anthropicMessagesAdapter) NeedsAPIKey() bool { return true }

// anthropicDefaultMaxTokens is required by the Messages API.
const anthropicDefaultMaxTokens = 1024

// anthropicToolName is the forced tool that carries the JSON answer.
const anthropicToolName = "emit_tags"

func (a anthropicMessagesAdapter) ChatJSON(ctx context.Context, request ChatRequest, env AdapterEnv) (ChatResult, error) {
	base := strings.TrimSpace(env.BaseURL)
	if base == "" {
		base = "https://api.anthropic.com"
	}
	endpoint := JoinURL(base, "v1/messages")

	maxTokens := anthropicDefaultMaxTokens
	if request.MaxOutputTokens != nil && *request.MaxOutputTokens < maxTokens {
		maxTokens = *request.MaxOutputTokens
	}

	payload := map[string]any{
		"model":      modelFromEnv(env),
		"max_tokens": maxTokens,
		"messages": []map[string]string{
			{"role": "user", "content": promptWithSchemaText(request.Prompt, request.JSONSchema)},
		},
	}
	if request.System != "" {
		payload["system"] = request.System
	}

	// Anthropic has no JSON mode, so the schema travels as a forced tool call.
	schema := request.JSONSchema
	if schema == nil {
		schema = map[string]any{"type": "object"}
	}
	payload["tools"] = []map[string]any{{
		"name":         anthropicToolName,
		"description":  "Return the tagging result as JSON.",
		"input_schema": schema,
	}}
	payload["tool_choice"] = map[string]any{"type": "tool", "name": anthropicToolName}

	body, err := encodeBody(payload)
	if err != nil {
		return ChatResult{}, err
	}

	headers := headersWith(map[string]string{
		"x-api-key":         env.APIKey,
		"anthropic-version": firstNonEmpty(env.ExtraHeaders["anthropic-version"], "2023-06-01"),
		"anthropic-dangerous-direct-browser-access": "true",
	}, env.ExtraHeaders)

	responseBody, requestErr := transportFor(env).doJSON(ctx, requestSpec{
		Method:     http.MethodPost,
		URL:        endpoint,
		Headers:    headers,
		Body:       body,
		DeadlineMs: request.DeadlineMs,
	})
	if requestErr != nil {
		return ChatResult{}, requestErr
	}

	decoded, err := decodeObject(responseBody)
	if err != nil {
		return ChatResult{}, err
	}

	// The answer arrives as a tool_use block's input.
	for _, rawBlock := range asSlice(decoded["content"]) {
		block, ok := rawBlock.(map[string]any)
		if !ok || asString(block["type"]) != "tool_use" {
			continue
		}
		if asString(block["name"]) != anthropicToolName {
			continue
		}
		input, ok := block["input"].(map[string]any)
		if !ok {
			break
		}
		return ChatResult{
			Text:         mustJSON(input),
			Parsed:       input,
			FinishReason: asString(decoded["stop_reason"]),
		}, nil
	}

	return ChatResult{}, &LlmError{Kind: KindInvalidJSON,
		Message: "the model did not call the " + anthropicToolName + " tool"}
}

// --- Google ---

type googleGenerativeAIAdapter struct{}

func (googleGenerativeAIAdapter) ID() string { return APIGoogleGenerativeAI }

func (googleGenerativeAIAdapter) NeedsAPIKey() bool { return true }

func (a googleGenerativeAIAdapter) ChatJSON(ctx context.Context, request ChatRequest, env AdapterEnv) (ChatResult, error) {
	base := strings.TrimSpace(env.BaseURL)
	if base == "" {
		base = "https://generativelanguage.googleapis.com"
	}

	// Google carries the key in the query string, which is why every error path
	// must redact URLs.
	endpoint := JoinURL(base, "v1beta/models/"+url.PathEscape(modelFromEnv(env))+":generateContent")
	endpoint += "?key=" + url.QueryEscape(env.APIKey)

	generationConfig := map[string]any{"responseMimeType": "application/json"}
	if request.Temperature != nil {
		generationConfig["temperature"] = *request.Temperature
	}
	if request.MaxOutputTokens != nil {
		generationConfig["maxOutputTokens"] = *request.MaxOutputTokens
	}

	payload := map[string]any{
		"contents": []map[string]any{{
			"role":  "user",
			"parts": []map[string]string{{"text": promptWithSchemaText(request.Prompt, request.JSONSchema)}},
		}},
		"generationConfig": generationConfig,
	}
	if request.System != "" {
		payload["systemInstruction"] = map[string]any{
			"parts": []map[string]string{{"text": request.System}},
		}
	}

	body, err := encodeBody(payload)
	if err != nil {
		return ChatResult{}, err
	}

	responseBody, requestErr := transportFor(env).doJSON(ctx, requestSpec{
		Method:     http.MethodPost,
		URL:        endpoint,
		Headers:    headersWith(env.ExtraHeaders),
		Body:       body,
		DeadlineMs: request.DeadlineMs,
	})
	if requestErr != nil {
		return ChatResult{}, requestErr
	}

	decoded, err := decodeObject(responseBody)
	if err != nil {
		return ChatResult{}, err
	}

	for _, rawCandidate := range asSlice(decoded["candidates"]) {
		candidate, ok := rawCandidate.(map[string]any)
		if !ok {
			continue
		}
		content, ok := candidate["content"].(map[string]any)
		if !ok {
			continue
		}
		var text strings.Builder
		for _, rawPart := range asSlice(content["parts"]) {
			part, ok := rawPart.(map[string]any)
			if !ok {
				continue
			}
			text.WriteString(asString(part["text"]))
		}
		if text.Len() == 0 {
			continue
		}

		result := ChatResult{Text: text.String(), FinishReason: asString(candidate["finishReason"])}
		if parsed, parseErr := ParseJSONLoose(result.Text); parseErr == nil {
			if object, ok := parsed.(map[string]any); ok {
				result.Parsed = object
			}
		}
		return result, nil
	}

	return ChatResult{}, InvalidJSONError("the response contained no candidate text")
}

// --- Azure OpenAI ---

type azureOpenAIChatAdapter struct{}

func (azureOpenAIChatAdapter) ID() string { return APIAzureOpenAIChat }

func (azureOpenAIChatAdapter) NeedsAPIKey() bool { return true }

func (a azureOpenAIChatAdapter) ChatJSON(ctx context.Context, request ChatRequest, env AdapterEnv) (ChatResult, error) {
	if strings.TrimSpace(env.DeploymentName) == "" {
		return ChatResult{}, &LlmError{Kind: KindBadRequest, Message: "an Azure deployment name is required"}
	}
	if strings.TrimSpace(env.BaseURL) == "" {
		return ChatResult{}, &LlmError{Kind: KindBadRequest, Message: "an Azure endpoint is required"}
	}

	apiVersion := firstNonEmpty(env.APIVersion, "2024-10-21")
	endpoint := JoinURL(env.BaseURL, "openai/deployments/"+url.PathEscape(env.DeploymentName)+"/chat/completions")
	endpoint += "?api-version=" + url.QueryEscape(apiVersion)

	capabilities := capabilitiesFor(a.ID(), env.Model, env.Capabilities, env.Overrides, env.Learned)
	assembled, dropped := AssembleRequest(capabilities, request)

	messages := make([]map[string]string, 0, 2)
	if assembled.System != "" {
		messages = append(messages, map[string]string{"role": "system", "content": assembled.System})
	}
	messages = append(messages, map[string]string{"role": "user", "content": promptWithSchema(assembled)})

	payload := map[string]any{"messages": messages}
	if assembled.Temperature != nil {
		payload["temperature"] = *assembled.Temperature
	}
	if assembled.MaxOutputTokens != nil {
		payload["max_tokens"] = *assembled.MaxOutputTokens
	}
	if assembled.JSONSchema != nil {
		payload["response_format"] = map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   "suggestions",
				"strict": assembled.Capabilities.JsonStrategy == JsonSchemaStrict,
				"schema": assembled.JSONSchema,
			},
		}
	}

	body, err := encodeBody(payload)
	if err != nil {
		return ChatResult{}, err
	}

	// Azure authenticates with api-key rather than a bearer token.
	headers := headersWith(map[string]string{"api-key": env.APIKey}, env.ExtraHeaders)

	responseBody, requestErr := transportFor(env).doJSON(ctx, requestSpec{
		Method:     http.MethodPost,
		URL:        endpoint,
		Headers:    headers,
		Body:       body,
		DeadlineMs: request.DeadlineMs,
	})
	if requestErr != nil {
		return ChatResult{}, requestErr
	}

	return parseOpenAIStyleResponse(responseBody, dropped)
}

// --- shared helpers ---

// parseOpenAIStyleResponse reads the chat-completions shape, shared by OpenAI and
// Azure.
func parseOpenAIStyleResponse(body []byte, dropped []DroppedOption) (ChatResult, error) {
	decoded, err := decodeObject(body)
	if err != nil {
		return ChatResult{}, err
	}

	choices := asSlice(decoded["choices"])
	if len(choices) == 0 {
		return ChatResult{}, InvalidJSONError("the response contained no choices")
	}

	choice, ok := choices[0].(map[string]any)
	if !ok {
		return ChatResult{}, InvalidJSONError("the first choice was not an object")
	}

	message, _ := choice["message"].(map[string]any)
	text := asString(message["content"])

	result := ChatResult{Text: text, FinishReason: asString(choice["finish_reason"]), DroppedOptions: dropped}
	if text != "" {
		if parsed, parseErr := ParseJSONLoose(text); parseErr == nil {
			if object, ok := parsed.(map[string]any); ok {
				result.Parsed = object
			}
		}
	}
	return result, nil
}

// promptWithSchema appends the schema to the prompt when the strategy cannot send
// one structurally.
func promptWithSchema(request AssembledRequest) string {
	if !request.PromptIncludesSchema || request.JSONSchema == nil {
		return request.Prompt
	}
	return promptWithSchemaText(request.Prompt, request.JSONSchema)
}

// promptWithSchemaText renders a prompt with the schema spelled out, for
// providers that describe the shape in words rather than structurally.
func promptWithSchemaText(prompt string, schema map[string]any) string {
	if schema == nil {
		return prompt
	}
	encoded, err := json.Marshal(schema)
	if err != nil {
		return prompt
	}
	return prompt + "\n\nRespond with a single JSON object matching this schema:\n" + string(encoded)
}

// modelFromEnv is a placeholder: the model belongs to the request, but the
// adapter env carries it so adapters do not each reach into configuration.
// Callers set it through AdapterEnv.Model.
func modelFromEnv(env AdapterEnv) string {
	if strings.TrimSpace(env.Model) == "" {
		return "unknown"
	}
	return env.Model
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func asSlice(value any) []any {
	if list, ok := value.([]any); ok {
		return list
	}
	return nil
}

func asString(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

func mustJSON(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(encoded)
}

// --- probe (test connection) ---

// Probe implementations issue a single unauthenticated-shape GET so the settings
// screen can tell a bad key from a bad endpoint.
func (openAIChatAdapter) Probe(env AdapterEnv) error {
	// A keyless endpoint must not receive an empty Authorization header. ChatJSON
	// already refuses to send one; a probe that sent "Bearer " anyway would fail the
	// test for a configuration that runs fine -- which is the custom provider's whole
	// point.
	headers := map[string]string{}
	if env.APIKey != "" {
		headers["authorization"] = "Bearer " + env.APIKey
	}
	return probeGet(env, JoinURL(firstNonEmpty(ResolveOpenAIBaseURL(env.BaseURL), "https://api.openai.com/v1"), "models"), headers)
}

func (anthropicMessagesAdapter) Probe(env AdapterEnv) error {
	return probeGet(env, JoinURL(firstNonEmpty(env.BaseURL, "https://api.anthropic.com"), "v1/models"),
		map[string]string{"x-api-key": env.APIKey, "anthropic-version": "2023-06-01"})
}

func (googleGenerativeAIAdapter) Probe(env AdapterEnv) error {
	base := firstNonEmpty(env.BaseURL, "https://generativelanguage.googleapis.com")
	return probeGet(env, JoinURL(base, "v1beta/models")+"?key="+url.QueryEscape(env.APIKey), nil)
}

func (azureOpenAIChatAdapter) Probe(env AdapterEnv) error {
	if strings.TrimSpace(env.BaseURL) == "" {
		return &LlmError{Kind: KindBadRequest, Message: "an Azure endpoint is required"}
	}
	apiVersion := firstNonEmpty(env.APIVersion, "2024-10-21")
	return probeGet(env, JoinURL(env.BaseURL, "openai/models")+"?api-version="+url.QueryEscape(apiVersion),
		map[string]string{"api-key": env.APIKey})
}

// probeGet performs one request and reports whether the credential and endpoint
// are usable.
func probeGet(env AdapterEnv, endpoint string, headers map[string]string) error {
	client := env.HTTPClient
	if client == nil {
		// The shared transport carries the proxy hook; see transport.go's attempt.
		client = netproxy.NewStrictClient(DefaultRequestTimeout)
	}

	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return &LlmError{Kind: KindBadRequest, Message: "invalid endpoint"}
	}
	for name, value := range headers {
		request.Header.Set(name, value)
	}

	response, err := client.Do(request)
	if err != nil {
		return NetworkError(err)
	}
	defer response.Body.Close()

	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return nil
	}

	// The detail is not read here: a probe only needs the classification, and
	// not reading the body avoids echoing anything sensitive.
	return &LlmError{
		Kind:    classifyStatus(response.StatusCode, ""),
		Status:  response.StatusCode,
		Message: fmt.Sprintf("HTTP %d", response.StatusCode),
	}
}
