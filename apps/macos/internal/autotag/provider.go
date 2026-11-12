package autotag

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/taoquo/trove/apps/macos/internal/domain"
	"github.com/taoquo/trove/apps/macos/internal/llm"
)

// The prompt and schema the model answers against.
//
// They live here rather than in the provider package because the answer's shape is
// the auto-tagger's business: a provider only knows how to carry JSON, not what
// the JSON means.

// MaxTagsPerBookmark caps how many tags a model may put on one post.
//
// Without a cap a chatty model tags a post with everything it can see, and the
// library fills with near-duplicates nobody asked for.
const MaxTagsPerBookmark = 2

// MaxVocabularyEntries caps how many known tags are offered to the model, keeping
// a large library's prompt from growing without bound.
const MaxVocabularyEntries = 100

// MaxPromptTextLength truncates a post in the prompt, so one long thread does not
// crowd out the rest of the batch.
const MaxPromptTextLength = 500

// VerdictSchema is the JSON schema the model must satisfy.
func VerdictSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"results": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"tweetId": map[string]any{"type": "string"},
						"tags": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"name":       map[string]any{"type": "string"},
									"confidence": map[string]any{"type": "number"},
								},
								"required":             []string{"name", "confidence"},
								"additionalProperties": false,
							},
						},
					},
					"required":             []string{"tweetId", "tags"},
					"additionalProperties": false,
				},
			},
		},
		"required":             []string{"results"},
		"additionalProperties": false,
	}
}

// BuildPrompt renders the instruction and the batch.
//
// The vocabulary is listed explicitly and the new-tag allowance is stated in the
// prompt as well as enforced afterwards: a model that knows it may not invent tags
// usually reuses the library instead of proposing near-duplicates that are then
// silently dropped.
func BuildPrompt(request TagRequest) string {
	// The instruction is the extension's, section for section, because it decides what
	// tags a model proposes. Its two rule lists are the definition of a good tag: the
	// durable subject area rather than the one-off event, a tag worth dozens of posts,
	// the broader concept over a narrow instance, and an imperfect existing tag over a
	// precise new one. Leaving them out is not a shorter prompt, it is a different one --
	// the model then invents near-synonyms and one-off tags, and the pipeline drops them
	// afterwards, which wastes the run.
	var sections []string

	sections = append(sections,
		"You are a librarian assigning topical tags to bookmarked tweets. The tag set is "+
			"a small, stable taxonomy shared by thousands of bookmarks.")
	sections = append(sections, fmt.Sprintf(
		"The whole library should settle at %d-%d tags in total, so treat every new tag as expensive.",
		HealthyTagCountMin, HealthyTagCountMax))

	sections = append(sections, strings.Join([]string{
		"Rules for choosing tags:",
		"- Tag the durable subject area, not the specific event, product launch, person, or company mentioned once.",
		"- A good tag applies to dozens of bookmarks. If a tag would only ever fit this one bookmark, it is too specific.",
		"- Prefer the broader concept over a narrow instance (use \"llm\" over \"gpt-4o-mini-benchmark\", \"startup-funding\" over \"seriesb-of-companyx\").",
		"- An imperfect existing tag beats a precise new one. Only reach for a new tag when the subject area is missing from the vocabulary entirely.",
		"- Never introduce a near-synonym, translation, abbreviation, plural or hyphenated variant of a tag that already exists.",
		"- Assign 1-2 tags per bookmark. Never pad the list to reach a limit.",
		"- Skip a bookmark entirely if it has no clear topic.",
	}, "\n"))

	sections = append(sections, strings.Join([]string{
		"Naming rules for tags:",
		"- English only, lowercase, singular.",
		"- One or two words, words separated by a single space.",
		"- No leading '#', no emoji, no punctuation, no author handles.",
	}, "\n"))

	// The three allowances. The wording is the extension's: it states the budget in the
	// prompt as well as enforcing it afterwards, because a model that knows it may not
	// invent tags usually reuses the library instead of proposing near-duplicates.
	switch {
	case request.NewTagBudget == 0:
		sections = append(sections,
			"You must only use tags from the vocabulary below. Never invent new tags. "+
				"If nothing fits, return no tags for that bookmark.")
	case request.NewTagBudget == UnboundedBudget:
		sections = append(sections,
			"Reuse tags from the vocabulary whenever they are a reasonable fit. "+
				"Introduce a new tag only when nothing in the vocabulary comes close.")
	default:
		sections = append(sections, fmt.Sprintf(
			"Reuse a tag from the vocabulary whenever it is a reasonable fit. Across this "+
				"whole batch you may introduce at most %d new tag(s), and only when no "+
				"existing tag covers the topic.", request.NewTagBudget))
	}

	vocabulary := "(empty)"
	if len(request.Vocabulary) > 0 {
		entries := request.Vocabulary
		if len(entries) > MaxVocabularyEntries {
			entries = entries[:MaxVocabularyEntries]
		}
		// "name (count)", the extension's form. The count is what tells a model that a tag
		// is already well established, which is the whole point of listing the vocabulary
		// rather than just forbidding new tags.
		rendered := make([]string, 0, len(entries))
		for _, entry := range entries {
			rendered = append(rendered, fmt.Sprintf("%s (%d)", entry.Name, entry.Count))
		}
		vocabulary = strings.Join(rendered, ", ")
	}
	sections = append(sections,
		"Existing vocabulary, most used first (name and usage count): "+vocabulary)

	sections = append(sections, "Return a confidence between 0 and 1 for each tag.")
	// The reply shape has to be the shape ParseVerdicts reads and VerdictSchema
	// constrains: confidence belongs to each tag.
	sections = append(sections,
		`Respond with JSON only: {"results":[{"tweetId":"...","tags":[{"name":"...","confidence":0.8}]}]}`)

	sections = append(sections, "Bookmarks: "+renderPromptBookmarks(request.Batch))

	return strings.Join(sections, "\n\n")
}

// renderPromptBookmarks renders the batch as JSON, the form the extension sends.
//
// The field names are the ones the reply echoes back, so the model sees an id it can copy
// rather than one it has to infer from a label.
func renderPromptBookmarks(batch []domain.Bookmark) string {
	lines := make([]map[string]string, 0, len(batch))
	for _, bookmark := range batch {
		text := bookmark.Text
		if len(text) > MaxPromptTextLength {
			text = text[:MaxPromptTextLength]
		}
		lines = append(lines, map[string]string{
			"tweetId": bookmark.TweetID,
			"author":  bookmark.AuthorHandle,
			"text":    text,
		})
	}
	encoded, err := json.Marshal(lines)
	if err != nil {
		// A batch that cannot be encoded is a programming error, not a runtime one; the
		// prompt still has to be a string, so the caller gets an empty batch it can see.
		return "[]"
	}
	return string(encoded)
}

// ParseVerdicts reads the model's answer.
//
// An unknown tweet id is dropped rather than turned into a suggestion: a
// hallucinated id would otherwise create a suggestion pointing at nothing.
func ParseVerdicts(result llm.ChatResult, batch []domain.Bookmark) ([]TagVerdict, error) {
	payload := result.Parsed
	if payload == nil && strings.TrimSpace(result.Text) != "" {
		parsed, err := llm.ParseJSONLoose(result.Text)
		if err != nil {
			return nil, err
		}
		object, ok := parsed.(map[string]any)
		if !ok {
			return nil, llm.InvalidJSONError("the reply was not a JSON object")
		}
		payload = object
	}
	if payload == nil {
		return nil, llm.InvalidJSONError("the reply contained no JSON")
	}

	rawResults, present := payload["results"]
	if !present {
		return nil, llm.InvalidJSONError(`the reply had no "results" field`)
	}

	known := make(map[string]bool, len(batch))
	for _, bookmark := range batch {
		known[bookmark.TweetID] = true
	}

	items, ok := rawResults.([]any)
	if !ok {
		return nil, llm.InvalidJSONError(`"results" was not an array`)
	}

	verdicts := make([]TagVerdict, 0, len(items))
	for _, rawItem := range items {
		item, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}

		tweetID, _ := item["tweetId"].(string)
		if !known[tweetID] {
			// A hallucinated or mismatched id: dropping it keeps the ledger and
			// the suggestion table honest.
			continue
		}

		// The shape the prompt asks for, and the order the extension parses it in:
		// name trimmed and empties dropped, confidence defaulted to 0.5 when it is not a
		// number and clamped to [0,1], sorted by confidence descending, and only then cut
		// to the cap. Cutting before sorting would take the model's order instead of its
		// best two, which is a different answer whenever the model returns them unsorted.
		candidates := make([]TagCandidate, 0, MaxTagsPerBookmark)
		for _, rawTag := range asAnySlice(item["tags"]) {
			tag, ok := rawTag.(map[string]any)
			if !ok {
				continue
			}
			name, _ := tag["name"].(string)
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			confidence := 0.5
			if value, ok := toFloat(tag["confidence"]); ok {
				confidence = clamp01(value)
			}
			candidates = append(candidates, TagCandidate{Name: name, Confidence: confidence})
		}
		sort.SliceStable(candidates, func(i, j int) bool {
			return candidates[i].Confidence > candidates[j].Confidence
		})
		if len(candidates) > MaxTagsPerBookmark {
			candidates = candidates[:MaxTagsPerBookmark]
		}
		if len(candidates) == 0 {
			// Nothing usable: the extension skips the entry rather than recording a
			// verdict with no tags.
			continue
		}

		verdicts = append(verdicts, TagVerdict{BookmarkID: tweetID, Tags: candidates})
	}

	return verdicts, nil
}

// ProviderClient turns a provider adapter into the pipeline's model client.
//
// It owns the two recoveries the extension's orchestrator had: dropping a request
// parameter the provider rejected, and retrying once when the answer was not
// usable JSON. Both are retried exactly once — a provider that rejects a parameter
// twice will reject it forever, and retrying an unparseable reply usually just
// burns quota.
type ProviderClient struct {
	Adapter llm.Adapter
	Env     llm.AdapterEnv
	// Capabilities defaults to the provider's, resolved for the model.
	Capabilities llm.ModelCapabilities
	// System is the optional system instruction.
	System string
}

// SuggestTags performs one batch call.
func (c *ProviderClient) SuggestTags(ctx context.Context, request TagRequest) ([]TagVerdict, error) {
	if c.Adapter == nil {
		return nil, errors.New("autotag: no provider adapter configured")
	}

	capabilities := c.Capabilities
	if capabilities.JsonStrategy == "" {
		capabilities = llm.ResolveCapabilities(c.Adapter.ID(), c.Env.Model, nil, nil)
	}

	prompt := BuildPrompt(request)
	schema := VerdictSchema()

	var lastErr error

	for attempt := 0; attempt < 2; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		// Each attempt gets its own environment holding a copy of the current
		// capability set. Sharing one pointer across attempts meant every attempt
		// observed whatever the variable held by the time it was read.
		attemptEnv := c.Env
		if attemptEnv.Model == "" {
			attemptEnv.Model = request.Model
		}
		attemptCapabilities := capabilities
		attemptEnv.Capabilities = &attemptCapabilities

		result, err := c.Adapter.ChatJSON(ctx, llm.ChatRequest{
			System:     c.System,
			Prompt:     prompt,
			JSONSchema: schema,
			JSONMode:   llm.JsonRequire,
			DeadlineMs: request.DeadlineMs,
		}, attemptEnv)

		if err == nil {
			verdicts, parseErr := ParseVerdicts(result, request.Batch)
			if parseErr == nil {
				return verdicts, nil
			}
			// An unusable reply gets one more try, which is what a provider that
			// ignored the JSON instruction usually needs.
			lastErr = parseErr
			continue
		}

		lastErr = err

		var classified *llm.LlmError
		if !errors.As(err, &classified) {
			return nil, err
		}

		switch classified.Kind {
		case llm.KindUnsupportedParam:
			// Drop exactly the parameter the provider named, then try once more.
			if classified.DroppedOption == "" || attempt > 0 {
				return nil, err
			}
			capabilities = degradeCapabilities(capabilities, classified.DroppedOption)
		case llm.KindInvalidJSON:
			// Fall through to the retry.
		default:
			// Everything else fails identically on a retry.
			return nil, err
		}
	}

	if lastErr == nil {
		lastErr = llm.InvalidJSONError("the model did not return a usable result")
	}
	return nil, lastErr
}

// degradeCapabilities removes the parameter a provider rejected.
func degradeCapabilities(capabilities llm.ModelCapabilities, option llm.DroppedOption) llm.ModelCapabilities {
	switch option {
	case llm.DropTemperature:
		capabilities.Temperature = false
	case llm.DropJSONSchema:
		// The provider rejected a structural schema, so relaxing it to a
		// non-strict schema would just be rejected again. Move it into the prompt
		// instead: that is the only form the provider has not refused.
		capabilities.JsonStrategy = llm.JsonPromptOnly
	case llm.DropMaxOutputTokens:
		capabilities.MaxOutputTokens = nil
	}
	return capabilities
}

// learnedCapabilitiesFor is the entry the extension looks up:
// learnedCapabilities[learnedCapabilityKey(provider.id, model)].
func learnedCapabilitiesFor(settings LLMSettings, providerID string) *llm.ModelCapabilities {
	if len(settings.LearnedCapabilities) == 0 {
		return nil
	}
	entry, ok := settings.LearnedCapabilities[llm.LearnedCapabilityKey(providerID, settings.Model)]
	if !ok {
		return nil
	}
	return &entry
}

// --- small helpers ---

func asAnySlice(value any) []any {
	if list, ok := value.([]any); ok {
		return list
	}
	return nil
}

func toFloat(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case int:
		return float64(typed), true
	case json.Number:
		if parsed, err := typed.Float64(); err == nil {
			return parsed, true
		}
	}
	return 0, false
}

func clamp01(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

// SortedVocabulary returns the offered tag names in a stable order, so a request
// is reproducible and its prompt diffable.
func SortedVocabulary(vocabulary map[string]string) []string {
	names := make([]string, 0, len(vocabulary))
	for _, display := range vocabulary {
		names = append(names, display)
	}
	sort.Strings(names)
	return names
}

// CredentialResolver reads a provider's API key.
//
// It is a function rather than a stored value so a key re-entered in the settings
// takes effect on the next batch instead of on the next launch, and so the key is
// never held anywhere the run state could serialise it.
type CredentialResolver func(ctx context.Context, providerID string) (string, error)

// BuildProviderClient constructs the client a run uses from the settings.
//
// The extension resolves the provider, its endpoint, its capabilities and the
// Keychain credential at the moment of the run. Doing the same here means a settings
// change applies to the next run rather than the next launch, and a missing
// credential is a per-run condition rather than a startup failure.
func BuildProviderClient(settings Settings, resolve CredentialResolver) (LLMClient, error) {
	if !settings.LLM.Enabled {
		// A disabled feature is not a misconfiguration, so this is not an error.
		return nil, nil
	}

	// The same resolution the connection test uses, so a test that passes means the
	// run will use the configuration that was tested.
	_, adapter, env, err := buildProviderEnv(settings, resolve)
	if err != nil {
		return nil, err
	}

	return &ProviderClient{Adapter: adapter, Env: env}, nil
}

// Chat sends one prompt and returns the raw reply.
//
// The vocabulary review asks a different question from tagging -- it wants proposals
// about the whole tag list, not verdicts about a batch -- so it shares the adapter,
// the endpoint and the capabilities but not the tagging schema.
func (c *ProviderClient) Chat(ctx context.Context, prompt string) (string, error) {
	if c.Adapter == nil {
		return "", errors.New("autotag: no provider adapter configured")
	}

	capabilities := c.Capabilities
	if capabilities.JsonStrategy == "" {
		capabilities = llm.ResolveCapabilities(c.Adapter.ID(), c.Env.Model, nil, nil)
	}

	env := c.Env
	env.Capabilities = &capabilities

	result, err := c.Adapter.ChatJSON(ctx, llm.ChatRequest{
		System:   c.System,
		Prompt:   prompt,
		JSONMode: llm.JsonRequire,
	}, env)
	if err != nil {
		return "", err
	}
	return result.Text, nil
}

// ConnectionTestResult is what the settings panel's "test connection" shows.
type ConnectionTestResult struct {
	OK         bool   `json:"ok"`
	ProviderID string `json:"providerId"`
	Model      string `json:"model"`
	// ErrorKind is the classified failure, so the panel can say "the key is wrong"
	// rather than printing a status code.
	ErrorKind string `json:"errorKind,omitempty"`
	Message   string `json:"message,omitempty"`
}

// MissingCredentialMessage is what the source reports when no key is stored.
const MissingCredentialMessage = "no API key is stored for this provider"

// errMissingCredential marks the case the panel reports as a missing key rather than
// as a failed request: nothing was sent, so there is no status to show.
var errMissingCredential = errors.New(MissingCredentialMessage)

// buildProviderEnv resolves the provider, its endpoint and its credential.
//
// It is shared with BuildProviderClient so the settings panel tests exactly the
// configuration the run will use: a test that resolved the provider differently
// could pass while the run failed.
func buildProviderEnv(settings Settings, resolve CredentialResolver) (llm.ProviderDefinition, llm.Adapter, llm.AdapterEnv, error) {
	providerID := strings.TrimSpace(settings.LLM.ProviderID)
	if providerID == "" {
		return llm.ProviderDefinition{}, nil, llm.AdapterEnv{}, errors.New("autotag: no provider selected")
	}

	provider := llm.ProviderByID(providerID)
	// ProviderByID falls back to the first provider for an unknown id, so an id that
	// does not match what came back is how an unknown one is detected.
	if provider.ID != providerID {
		return llm.ProviderDefinition{}, nil, llm.AdapterEnv{}, fmt.Errorf("autotag: unknown provider %q", providerID)
	}

	adapter, err := llm.AdapterFor(provider.API)
	if err != nil {
		return llm.ProviderDefinition{}, nil, llm.AdapterEnv{}, err
	}

	baseURL := strings.TrimSpace(settings.LLM.BaseURL)
	if baseURL == "" {
		baseURL = provider.DefaultBaseURL
	}
	if provider.NeedsBaseURL && baseURL == "" {
		return llm.ProviderDefinition{}, nil, llm.AdapterEnv{}, fmt.Errorf("autotag: %s needs an endpoint", provider.Name)
	}

	env := llm.AdapterEnv{
		Model:          strings.TrimSpace(settings.LLM.Model),
		BaseURL:        baseURL,
		DeploymentName: strings.TrimSpace(settings.LLM.Deployment),
		ExtraHeaders:   provider.ExtraHeaders,
		Overrides:      settings.LLM.CapabilityOverrides,
		Learned:        learnedCapabilitiesFor(settings.LLM, providerID),
	}

	// CredentialAPIKeyOrNone means the endpoint may accept no authentication, so a
	// missing key must not stop the test. It does not mean the key is ignored: a
	// stored key is still sent, and skipping the read for this kind left every
	// custom endpoint unauthenticated -- a keyed proxy answered 401 and the panel
	// reported a valid Keychain key as invalid.
	optionalKey := provider.CredentialKind == llm.CredentialAPIKeyOrNone
	if resolve == nil && !optionalKey {
		return provider, adapter, env, errors.New("autotag: no credential source configured")
	}
	if resolve != nil {
		key, err := resolve(context.Background(), providerID)
		if err != nil {
			return provider, adapter, env, fmt.Errorf("autotag: read the %s credential: %w", providerID, err)
		}
		key = strings.TrimSpace(key)
		if key != "" {
			env.APIKey = key
		} else if !optionalKey {
			return provider, adapter, env, errMissingCredential
		}
	}

	if provider.NeedsAPIVersion {
		env.APIVersion = provider.DefaultAPIVersion
	}

	return provider, adapter, env, nil
}

// TestProviderConnection checks that the configured endpoint and credential work.
//
// The adapters have carried a Probe since they were written -- the comment on the
// interface even says it is for this -- but nothing called it, so the settings panel
// could not tell a working configuration from a typo until a full run was spent. This
// is the missing caller.
func TestProviderConnection(settings Settings, resolve CredentialResolver) ConnectionTestResult {
	result := ConnectionTestResult{ProviderID: strings.TrimSpace(settings.LLM.ProviderID)}

	provider, adapter, env, err := buildProviderEnv(settings, resolve)
	if err != nil {
		// Only a missing credential is an authentication failure. A bad provider id,
		// a missing endpoint or an unreadable Keychain is a configuration problem, and
		// reporting it as "the API key is invalid" sends the user to replace a key
		// that was never the reason -- the panel shows the message for an unknown
		// kind instead of guessing.
		if errors.Is(err, errMissingCredential) {
			result.ErrorKind = string(llm.KindUnauthorized)
			result.Message = MissingCredentialMessage
		} else {
			result.ErrorKind = string(llm.KindUnknown)
			result.Message = err.Error()
		}
		return result
	}

	result.ProviderID = provider.ID
	result.Model = env.Model

	if err := adapter.Probe(env); err != nil {
		result.ErrorKind = string(llm.ErrorKindOf(err))
		result.Message = err.Error()
		return result
	}

	result.OK = true
	return result
}
