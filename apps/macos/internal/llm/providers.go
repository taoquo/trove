package llm

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// API ids identify a request/response shape, not a vendor. One vendor can be
// reachable through more than one shape, and the `custom` provider uses the
// OpenAI shape against a user-supplied endpoint.
const (
	APIOpenAIChat         = "openai-chat"
	APIAnthropicMessages  = "anthropic-messages"
	APIGoogleGenerativeAI = "google-generative-ai"
	APIAzureOpenAIChat    = "azure-openai-chat"
)

// CredentialKind describes what a provider needs before it can be called.
type CredentialKind = string

const (
	CredentialAPIKey          CredentialKind = "api-key"
	CredentialAPIKeyOrNone    CredentialKind = "api-key-or-none"
	CredentialAzureDeployment CredentialKind = "azure-deployment"
)

// ProviderDefinition is one selectable provider.
type ProviderDefinition struct {
	ID   string
	API  string
	Name string
	// DefaultBaseURL is empty for providers that cannot have a sensible default.
	DefaultBaseURL string
	CredentialKind CredentialKind
	// NeedsBaseURL means the user must supply an endpoint.
	NeedsBaseURL bool
	// NeedsDeployment and NeedsAPIVersion are Azure's extra configuration.
	NeedsDeployment   bool
	NeedsAPIVersion   bool
	DefaultAPIVersion string
	ExtraHeaders      map[string]string
	// APIKeyURL is where the user obtains a key.
	APIKeyURL string
}

// DefaultProviderID is used when nothing is configured or an id is unknown.
const DefaultProviderID = "openai"

// Providers is the selectable list, in the order a picker shows it.
var Providers = []ProviderDefinition{
	{
		ID:             "openai",
		API:            APIOpenAIChat,
		Name:           "OpenAI",
		DefaultBaseURL: "https://api.openai.com/v1",
		CredentialKind: CredentialAPIKey,
		APIKeyURL:      "https://platform.openai.com/api-keys",
	},
	{
		ID:             "anthropic",
		API:            APIAnthropicMessages,
		Name:           "Anthropic",
		DefaultBaseURL: "https://api.anthropic.com",
		CredentialKind: CredentialAPIKey,
		ExtraHeaders:   map[string]string{"anthropic-version": "2023-06-01"},
		APIKeyURL:      "https://console.anthropic.com/settings/keys",
	},
	{
		ID:             "google",
		API:            APIGoogleGenerativeAI,
		Name:           "Google",
		DefaultBaseURL: "https://generativelanguage.googleapis.com",
		CredentialKind: CredentialAPIKey,
		APIKeyURL:      "https://aistudio.google.com/app/apikey",
	},
	{
		ID:                "azure-openai",
		API:               APIAzureOpenAIChat,
		Name:              "Azure OpenAI",
		DefaultBaseURL:    "",
		CredentialKind:    CredentialAzureDeployment,
		NeedsBaseURL:      true,
		NeedsDeployment:   true,
		NeedsAPIVersion:   true,
		DefaultAPIVersion: "2024-10-21",
		APIKeyURL:         "https://portal.azure.com",
	},
	{
		ID:             "custom",
		API:            APIOpenAIChat,
		Name:           "Custom (OpenAI-compatible)",
		DefaultBaseURL: "",
		// A local server may have no authentication at all, so an absent key is
		// a supported configuration rather than an error.
		CredentialKind: CredentialAPIKeyOrNone,
		NeedsBaseURL:   true,
	},
}

// ProviderByID looks a provider up, falling back to the first entry.
//
// Falling back rather than failing keeps a settings file written by a newer
// build, naming a provider this one does not have, from bricking the app.
func ProviderByID(id string) ProviderDefinition {
	trimmed := strings.TrimSpace(id)
	for _, provider := range Providers {
		if provider.ID == trimmed {
			return provider
		}
	}
	return Providers[0]
}

// ResolveBaseURL returns the endpoint to call: the configured one when present,
// otherwise the provider's default.
func ResolveBaseURL(provider ProviderDefinition, configured string) string {
	if trimmed := strings.TrimSpace(configured); trimmed != "" {
		return trimmed
	}
	return provider.DefaultBaseURL
}

// JoinURL appends a path to a base URL without doubling or dropping the slash.
func JoinURL(base, path string) string {
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(path, "/")
}

// ResolveOpenAIBaseURL appends /v1 when the user supplied only a host, which is
// the common case for a local server.
func ResolveOpenAIBaseURL(configured string) string {
	// The order matters and it is the extension's: strip the trailing slashes first, then
	// parse. Parsing first leaves "https://host///" with a path of "///", which is neither
	// empty nor "/", so the origin branch is never reached.
	trimmed := strings.TrimRight(strings.TrimSpace(configured), "/")
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		// Not a URL at all: the extension returns the trimmed text unchanged and lets the
		// request fail with something the user can read.
		return trimmed
	}
	if parsed.Path == "" || parsed.Path == "/" {
		// The extension returns `${url.origin}/v1`, so the path is rebuilt from the origin
		// rather than appended to the input. Appending is the tempting version and it is
		// wrong: "https://host/?key=1" has a path of "/" and no trailing slash, so the
		// result would be "https://host/?key=1/v1".
		//
		// origin carries no userinfo, which means credentials in the configured URL are
		// dropped here -- as they are in the extension, which is where this comes from.
		return parsed.Scheme + "://" + parsed.Host + "/v1"
	}
	return strings.TrimRight(trimmed, "/")
}

// BaseURLErrorKind describes why an endpoint was rejected.
type BaseURLErrorKind = string

const (
	BaseURLEmpty    BaseURLErrorKind = "empty"
	BaseURLInvalid  BaseURLErrorKind = "invalid"
	BaseURLProtocol BaseURLErrorKind = "protocol"
)

// BaseURLError is a rejected endpoint.
type BaseURLError struct {
	Kind BaseURLErrorKind
}

func (e *BaseURLError) Error() string {
	switch e.Kind {
	case BaseURLEmpty:
		return "llm: the endpoint is empty"
	case BaseURLProtocol:
		return "llm: the endpoint must use https (http is allowed only for localhost)"
	default:
		return "llm: the endpoint is not a valid URL"
	}
}

// ValidateBaseURL checks a user-supplied endpoint.
//
// https is required except for the loopback addresses, where a local model server
// legitimately speaks plain http. Anything else would let a user be talked into
// sending their API key in the clear.
func ValidateBaseURL(raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return &BaseURLError{Kind: BaseURLEmpty}
	}

	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Host == "" {
		return &BaseURLError{Kind: BaseURLInvalid}
	}
	if parsed.Scheme == "https" {
		return nil
	}
	if parsed.Scheme == "http" && isLoopback(parsed.Hostname()) {
		return nil
	}
	return &BaseURLError{Kind: BaseURLProtocol}
}

func isLoopback(host string) bool {
	switch strings.ToLower(host) {
	case "localhost", "127.0.0.1", "::1", "[::1]":
		return true
	}
	return false
}

// PermissionOrigin renders the origin a user would be granting access to.
//
// The extension used this to request a host permission; here it is what the
// settings screen shows before the endpoint is saved.
func PermissionOrigin(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" {
		return "", &BaseURLError{Kind: BaseURLInvalid}
	}
	return parsed.Scheme + "://" + parsed.Host + "/*", nil
}

// Adapter performs one provider's request shape.
type Adapter interface {
	// ID is the API id this adapter implements.
	ID() string
	// NeedsAPIKey reports whether the caller must supply a key.
	NeedsAPIKey() bool
	// ChatJSON performs a completion request and returns parsed JSON.
	ChatJSON(ctx context.Context, request ChatRequest, env AdapterEnv) (ChatResult, error)
	// Probe checks that the endpoint and credential work, for "test connection".
	Probe(env AdapterEnv) error
}

// adapterRegistry is populated by the adapters in this package.
var adapterRegistry = map[string]Adapter{}

// AdapterFor returns the adapter for an API id.
func AdapterFor(api string) (Adapter, error) {
	adapter, ok := adapterRegistry[strings.TrimSpace(api)]
	if !ok {
		return nil, &LlmError{Kind: KindUnknown, Message: fmt.Sprintf("no adapter for API %q", api)}
	}
	return adapter, nil
}

// KnownAPIIDs lists the registered API ids, for diagnostics and tests.
func KnownAPIIDs() []string {
	ids := make([]string, 0, len(adapterRegistry))
	for id := range adapterRegistry {
		ids = append(ids, id)
	}
	return ids
}
