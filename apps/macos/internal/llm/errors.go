// Package llm talks to the model providers the auto-tagger can use.
//
// It ports the extension's provider layer (src/lib/llm/) and adds what a
// long-lived desktop process needs: bounded retries, cancellable requests and a
// taxonomy the UI can explain.
//
// See docs/ai-autotagging.md.
package llm

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ErrorKind classifies a provider failure.
//
// The set is the extension's, kept whole: the auto-tagger's control flow keys off
// these, and a coarser taxonomy would lose the distinction between "retry" and
// "the user must fix their configuration".
type ErrorKind = string

const (
	KindUnauthorized     ErrorKind = "unauthorized"
	KindForbidden        ErrorKind = "forbidden"
	KindRateLimit        ErrorKind = "rate-limit"
	KindQuota            ErrorKind = "quota"
	KindBadRequest       ErrorKind = "bad-request"
	KindUnsupportedParam ErrorKind = "unsupported-param"
	KindContextLength    ErrorKind = "context-length"
	KindTimeout          ErrorKind = "timeout"
	KindNetwork          ErrorKind = "network"
	KindServer           ErrorKind = "server"
	KindContentFilter    ErrorKind = "content-filter"
	KindInvalidJSON      ErrorKind = "invalid-json"
	KindUnknown          ErrorKind = "unknown"
)

// AllErrorKinds is every kind, for validation and tests.
var AllErrorKinds = []ErrorKind{
	KindUnauthorized, KindForbidden, KindRateLimit, KindQuota, KindBadRequest,
	KindUnsupportedParam, KindContextLength, KindTimeout, KindNetwork,
	KindServer, KindContentFilter, KindInvalidJSON, KindUnknown,
}

// LlmError is a classified provider failure.
type LlmError struct {
	Kind   ErrorKind
	Status int
	// RetryAfter is set when the provider asked us to wait.
	RetryAfter time.Duration
	// DroppedOption names a request parameter the provider rejected, so the
	// caller can retry without it.
	DroppedOption string
	// Message is safe to show: it has been redacted.
	Message string
	// cause is the underlying error, kept for errors.Is but never printed
	// unredacted.
	cause error
}

func (e *LlmError) Error() string {
	base := "llm: " + e.Kind
	if e.Status != 0 {
		base += fmt.Sprintf(" (HTTP %d)", e.Status)
	}
	if e.Message != "" {
		base += ": " + e.Message
	}
	return base
}

func (e *LlmError) Unwrap() error { return e.cause }

// Is matches on the kind, so errors.Is(err, &LlmError{Kind: KindRateLimit}) works.
func (e *LlmError) Is(target error) bool {
	var other *LlmError
	if !errors.As(target, &other) {
		return false
	}
	return other.Kind == e.Kind
}

// ErrorKindOf returns the kind of err, or an empty string.
func ErrorKindOf(err error) ErrorKind {
	var classified *LlmError
	if errors.As(err, &classified) {
		return classified.Kind
	}
	if err == nil {
		return ""
	}
	return KindUnknown
}

// IsRateLimit reports whether err is a rate limit, by kind or by the wording the
// older engine produced.
func IsRateLimit(err error) bool {
	if err == nil {
		return false
	}
	if ErrorKindOf(err) == KindRateLimit {
		return true
	}
	message := err.Error()
	return strings.Contains(message, "429") || regexp.MustCompile(`(?i)rate ?limit`).MatchString(message)
}

// IsRetryable reports whether the same request could succeed later.
func IsRetryable(err error) bool {
	switch ErrorKindOf(err) {
	case KindRateLimit, KindNetwork, KindTimeout, KindServer:
		return true
	default:
		// An invalid credential or a malformed request will fail identically
		// forever, so retrying only wastes the user's quota.
		return false
	}
}

// --- classification ---

// Patterns that distinguish failures sharing an HTTP status.
var (
	quotaPattern       = regexp.MustCompile(`(?i)quota|billing|insufficient|exceeded your current|out of credits|no credits`)
	contentPattern     = regexp.MustCompile(`(?i)content[_ ]filter|content policy|safety|blocked by`)
	contextPattern     = regexp.MustCompile(`(?i)context length|context_length|too long|maximum context|max_tokens.*exceed|token limit`)
	unsupportedPattern = regexp.MustCompile(`(?i)unsupported|unknown (?:parameter|field)|unrecognized|not supported|invalid.*(?:temperature|response_format|json)`)
)

// classifyStatus maps a status and body to a kind.
//
// 403 is ambiguous in the wild: it means "forbidden" but also "you are out of
// credit", and those need different advice.
func classifyStatus(status int, body string) ErrorKind {
	switch {
	case status == 401:
		return KindUnauthorized

	case status == 402:
		return KindQuota

	case status == 403:
		if quotaPattern.MatchString(body) {
			return KindQuota
		}
		return KindForbidden

	case status == 429:
		return KindRateLimit

	case status >= 500:
		return KindServer

	case status >= 400:
		switch {
		case contentPattern.MatchString(body):
			return KindContentFilter
		case unsupportedPattern.MatchString(body):
			return KindUnsupportedParam
		case contextPattern.MatchString(body):
			return KindContextLength
		default:
			return KindBadRequest
		}
	}

	return KindUnknown
}

// maxErrorDetail bounds how much of a provider's error body is kept.
const maxErrorDetail = 200

// normalizeHTTPError builds a classified error from a non-2xx response.
//
// The detail is truncated and redacted; the message keeps the "HTTP <status>"
// shape the older engine produced, because some callers match on it.
func normalizeHTTPError(status int, body string, apiKey string) *LlmError {
	detail := redactSecret(extractErrorDetail(body), apiKey)
	kind := classifyStatus(status, detail)

	message := ""
	if detail != "" {
		message = fmt.Sprintf("HTTP %d: %s", status, detail)
	} else {
		message = fmt.Sprintf("HTTP %d", status)
	}

	classified := &LlmError{Kind: kind, Status: status, Message: message}
	if kind == KindUnsupportedParam {
		classified.DroppedOption = droppedOptionFrom(status, detail)
	}
	return classified
}

// extractErrorDetail pulls a readable message out of a provider's error body,
// tolerating the several shapes providers use.
func extractErrorDetail(body string) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(body), &decoded); err == nil {
		for _, key := range []string{"error", "message", "detail"} {
			value, present := decoded[key]
			if !present {
				continue
			}
			switch typed := value.(type) {
			case string:
				return truncateDetail(typed)
			case map[string]any:
				if message, ok := typed["message"].(string); ok {
					return truncateDetail(message)
				}
			}
		}
	}

	return truncateDetail(body)
}

func truncateDetail(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= maxErrorDetail {
		return value
	}
	return value[:maxErrorDetail]
}

// droppedOptionFrom names the parameter a provider complained about.
func droppedOptionFrom(status int, detail string) string {
	lower := strings.ToLower(detail)

	switch {
	case status == 400 || status == 404 || status == 422:
	default:
		return ""
	}

	// Order matters: temperature is the most common rejection, and the OpenAI
	// reasoning models reject exactly it.
	if strings.Contains(lower, "temperature") {
		return "temperature"
	}
	// "json schema" with a space is its own spelling: a gateway that words its refusal
	// that way would otherwise keep the schema and be rejected again. The extension
	// matches the spaced form as well as the underscored one.
	if strings.Contains(lower, "response_format") || strings.Contains(lower, "json_object") ||
		strings.Contains(lower, "json mode") || strings.Contains(lower, "json_schema") ||
		strings.Contains(lower, "json schema") {
		return "jsonSchema"
	}
	if strings.Contains(lower, "max_tokens") || strings.Contains(lower, "max_output_tokens") {
		return "maxOutputTokens"
	}
	return ""
}

// --- errors that are not HTTP ---

// TimeoutError reports a request that ran out of time.
func TimeoutError(message string) *LlmError {
	return &LlmError{Kind: KindTimeout, Message: message}
}

// NetworkError reports a transport failure.
func NetworkError(err error) *LlmError {
	return &LlmError{Kind: KindNetwork, Message: redactBaseUrl(errorText(err)), cause: err}
}

// InvalidJSONError reports a response that could not be parsed.
func InvalidJSONError(message string) *LlmError {
	return &LlmError{Kind: KindInvalidJSON, Message: message}
}

// DeadlineExceededError reports the caller's own budget running out, which is
// distinct from a provider timeout: it is a partial success, not a failure.
func DeadlineExceededError() *LlmError {
	return &LlmError{Kind: KindTimeout, Message: "run-deadline-exceeded"}
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// --- redaction ---

// urlCredentialPattern matches credentials embedded in a URL query or authority.
var urlCredentialPattern = regexp.MustCompile(`(?i)([?&](?:key|api[-_]?key|access_token|token)=)[^&\s]+`)

// redactBaseUrl removes credentials that a URL may carry in its query string.
//
// This matters most for the Google adapter, which puts the API key in the query,
// so any log line containing the request URL would otherwise contain the key.
func redactBaseUrl(value string) string {
	return urlCredentialPattern.ReplaceAllString(value, "${1}[redacted]")
}

// redactSecret removes a known secret from a string.
//
// A short secret is left alone: replacing every occurrence of two or three
// characters would mangle ordinary text without protecting anything.
func redactSecret(value, secret string) string {
	if len(secret) < 6 {
		return redactBaseUrl(value)
	}
	value = strings.ReplaceAll(value, secret, "[redacted]")
	return redactBaseUrl(value)
}

// SanitizeLlmError turns any error into a redacted, classified one.
//
// It must be used before an error reaches a log or the UI: a Go *url.Error
// prints the whole URL, which is how an API key ends up in a log file.
func SanitizeLlmError(err error, secrets ...string) error {
	if err == nil {
		return nil
	}

	message := errorText(err)

	// A *url.Error renders its URL, so the query string is scrubbed first.
	if unwrapped := errors.Unwrap(err); unwrapped != nil {
		message = errorText(unwrapped)
	}
	message = redactBaseUrl(message)
	for _, secret := range secrets {
		message = redactSecret(message, secret)
	}

	var classified *LlmError
	if errors.As(err, &classified) {
		copied := *classified
		copied.Message = redactBaseUrl(copied.Message)
		for _, secret := range secrets {
			copied.Message = redactSecret(copied.Message, secret)
		}
		copied.cause = nil
		return &copied
	}

	return &LlmError{Kind: KindUnknown, Message: message}
}

// parseRetryAfter reads a Retry-After header, in delay-seconds or HTTP-date form.
//
// The value is not capped here: the caller decides whether it is plausible, so
// that an absurd wait is reported rather than silently slept through.
func parseRetryAfter(value string) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}

	// A decimal is allowed: the header's delay-seconds form is a number, not an integer,
	// and the extension reads it with Number(). Parsing it with an integer conversion
	// rejects "1.5" -- which then falls through to the date branch, fails there too, and
	// ends up as no wait at all.
	if seconds, err := time.ParseDuration(value + "s"); err == nil {
		if seconds <= 0 {
			return 0
		}
		return seconds
	}

	if when, err := parseHTTPDate(value); err == nil {
		if wait := time.Until(when); wait > 0 {
			return wait
		}
	}

	return 0
}

func parseHTTPDate(value string) (time.Time, error) {
	for _, layout := range []string{time.RFC1123, time.RFC1123Z, time.ANSIC, time.RFC850} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("not an HTTP date: %q", value)
}

// shouldRetryHeader reads the non-standard header some gateways send to override
// the status code.
func shouldRetryHeader(value string) (bool, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false, false
	}
	parsed, err := strconv.ParseBool(trimmed)
	if err != nil {
		return false, false
	}
	return parsed, true
}
