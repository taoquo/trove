package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/taoquo/trove/internal/netproxy"
)

// ChatResult is one completion response.
type ChatResult struct {
	// Text is the model's raw output.
	Text string
	// Parsed is the JSON object, when one could be extracted.
	Parsed map[string]any
	// FinishReason is the provider's stop reason, when reported.
	FinishReason string
	// DroppedOptions lists request parameters this call had to leave out.
	DroppedOptions []DroppedOption
}

// maxProviderResponseBytes bounds a provider body.
const maxProviderResponseBytes = 2 << 20 // 2 MiB

// Transport performs the HTTP part shared by the adapters.
type Transport struct {
	// Client is the HTTP client; nil builds one with the default timeout.
	Client *http.Client
	// Sleep waits between attempts; nil sleeps for real.
	Sleep func(time.Duration)
	// Now is the clock, for tests.
	Now func() time.Time
}

// requestSpec describes one HTTP attempt.
type requestSpec struct {
	Method  string
	URL     string
	Headers map[string]string
	// Body is re-sent on each attempt, so it is kept as bytes rather than a
	// reader that would be consumed by the first try.
	Body []byte
	// DeadlineMs is the caller's remaining budget.
	DeadlineMs int
	// OnUnsupportedParam is called once when the provider rejects a parameter,
	// so the caller can drop it and retry.
	OnUnsupportedParam func(option DroppedOption) bool
}

// doJSON performs the request with retries and returns the body.
func (t *Transport) doJSON(ctx context.Context, spec requestSpec) ([]byte, *LlmError) {
	deadline := time.Duration(0)
	if spec.DeadlineMs > 0 {
		deadline = time.Duration(spec.DeadlineMs) * time.Millisecond
	}

	paramRetriesLeft := 1
	blindRetriesLeft := 1
	rateLimitRetriesLeft := DefaultRateLimitRetries
	rateLimitHits := 0
	var lastErr *LlmError

	// The budget is wall-clock, not an attempt count. It used to be computed as
	// deadline minus one millisecond per attempt, which is why a batch with a
	// two-minute budget could spend five minutes retrying: each retry was handed
	// almost the whole budget again, including the time the previous attempt had
	// already burned.
	now := t.Now
	if now == nil {
		now = time.Now
	}
	started := now()

	for attempt := 1; attempt <= MaxTransportAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, &LlmError{Kind: KindTimeout, Message: "cancelled"}
		}

		remaining := deadline
		if deadline > 0 {
			remaining = deadline - now().Sub(started)
		}
		if deadline > 0 && remaining <= 0 {
			return nil, DeadlineExceededError()
		}

		timeout := DefaultRequestTimeout
		if deadline > 0 && remaining < timeout {
			timeout = remaining
		}

		body, _, err := t.attempt(ctx, spec, timeout)
		if err == nil {
			return body, nil
		}

		lastErr = err

		// A server-requested wait longer than the cap is reported rather than
		// slept through.
		if err.RetryAfter > MaxRetryAfter {
			return nil, err
		}

		switch err.Kind {
		case KindUnsupportedParam:
			if paramRetriesLeft > 0 && spec.OnUnsupportedParam != nil && err.DroppedOption != "" {
				if spec.OnUnsupportedParam(err.DroppedOption) {
					paramRetriesLeft--
					continue
				}
			}
			return nil, err

		case KindRateLimit:
			if rateLimitRetriesLeft <= 0 {
				return nil, err
			}
			rateLimitRetriesLeft--
			rateLimitHits++
			t.wait(ctx, err.RetryAfter, rateLimitHits)
			continue

		case KindNetwork, KindTimeout, KindServer:
			if blindRetriesLeft <= 0 {
				return nil, err
			}
			blindRetriesLeft--
			t.wait(ctx, err.RetryAfter, 1)
			continue

		default:
			// Anything else fails identically on retry.
			return nil, err
		}
	}

	if lastErr == nil {
		lastErr = &LlmError{Kind: KindUnknown, Message: "request failed"}
	}
	return nil, lastErr
}

// attempt performs a single request.
func (t *Transport) attempt(ctx context.Context, spec requestSpec, timeout time.Duration) ([]byte, http.Header, *LlmError) {
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var reader io.Reader
	if spec.Body != nil {
		reader = bytes.NewReader(spec.Body)
	}

	request, err := http.NewRequestWithContext(requestCtx, spec.Method, spec.URL, reader)
	if err != nil {
		return nil, nil, &LlmError{Kind: KindBadRequest, Message: redactBaseUrl(err.Error())}
	}
	for name, value := range spec.Headers {
		request.Header.Set(name, value)
	}

	client := t.Client
	if client == nil {
		// The shared transport, so an auto-tagging run goes through the proxy the
		// machine is configured with. A provider endpoint is as unreachable as x.com
		// on a proxied network, and a run that silently fails every batch is worse
		// than one that says why.
		client = netproxy.NewStrictClient(timeout)
	}

	response, err := client.Do(request)
	if err != nil {
		return nil, nil, t.transportError(ctx, err)
	}
	defer response.Body.Close()

	body, readErr := io.ReadAll(io.LimitReader(response.Body, maxProviderResponseBytes+1))
	if readErr != nil {
		return nil, nil, t.transportError(ctx, readErr)
	}
	if len(body) > maxProviderResponseBytes {
		return nil, nil, &LlmError{Kind: KindUnknown, Status: response.StatusCode,
			Message: fmt.Sprintf("response exceeded %d bytes", maxProviderResponseBytes)}
	}

	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return body, response.Header, nil
	}

	classified := normalizeHTTPError(response.StatusCode, string(body), spec.Headers["x-api-key"])
	if retryAfter := parseRetryAfter(response.Header.Get("Retry-After")); retryAfter > 0 {
		classified.RetryAfter = retryAfter
	}

	// A gateway may override the status code's meaning.
	if retry, present := shouldRetryHeader(response.Header.Get("x-should-retry")); present && !retry {
		// The header says do not retry, which only matters for kinds we would
		// otherwise retry.
		if IsRetryable(classified) {
			classified.Kind = KindBadRequest
		}
	} else if present && retry {
		classified.Kind = KindServer
	}

	return nil, response.Header, classified
}

// transportError classifies a transport-level failure, keeping a cancellation
// distinct from a timeout.
func (t *Transport) transportError(ctx context.Context, err error) *LlmError {
	if ctx.Err() != nil {
		return &LlmError{Kind: KindTimeout, Message: "cancelled"}
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "timeout") || strings.Contains(message, "deadline exceeded") {
		return TimeoutError(redactBaseUrl(err.Error()))
	}
	return NetworkError(err)
}

// wait sleeps for the retry delay, honouring a server-requested value.
func (t *Transport) wait(ctx context.Context, requested time.Duration, hits int) {
	delay := requested
	if delay <= 0 {
		delay = DefaultRateLimitBackoff << (hits - 1)
		if delay > MaxRateLimitBackoff || delay <= 0 {
			delay = MaxRateLimitBackoff
		}
	}
	if t.Sleep != nil {
		t.Sleep(delay)
		return
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

// --- response parsing helpers ---

// ExtractJSONCandidates finds JSON objects or arrays inside a model's reply.
//
// Models wrap JSON in prose or code fences often enough that a strict decode
// fails on responses that are perfectly usable.
func ExtractJSONCandidates(text string) []string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil
	}

	candidates := []string{trimmed}

	// Fenced blocks, with or without a language tag.
	for _, fence := range []string{"```json", "```"} {
		parts := strings.Split(trimmed, fence)
		for i := 1; i < len(parts); i += 2 {
			if block := strings.TrimSpace(parts[i]); block != "" {
				candidates = append(candidates, block)
			}
		}
	}

	// A balanced object or array slice.
	if sliced := sliceBalanced(trimmed, '{', '}'); sliced != "" {
		candidates = append(candidates, sliced)
	}
	if sliced := sliceBalanced(trimmed, '[', ']'); sliced != "" {
		candidates = append(candidates, sliced)
	}

	return candidates
}

// sliceBalanced returns the first balanced region delimited by open and close,
// ignoring delimiters inside strings.
func sliceBalanced(text string, open, close byte) string {
	start := strings.IndexByte(text, open)
	if start < 0 {
		return ""
	}

	depth := 0
	inString := false
	escaped := false

	for i := start; i < len(text); i++ {
		ch := text[i]

		if inString {
			switch {
			case escaped:
				escaped = false
			case ch == '\\':
				escaped = true
			case ch == '"':
				inString = false
			}
			continue
		}

		switch ch {
		case '"':
			inString = true
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return text[start : i+1]
			}
		}
	}

	return ""
}

// ParseJSONLoose decodes the first candidate that yields a JSON value.
func ParseJSONLoose(text string) (any, error) {
	for _, candidate := range ExtractJSONCandidates(text) {
		var decoded any
		if err := json.Unmarshal([]byte(candidate), &decoded); err == nil {
			return decoded, nil
		}
	}
	return nil, InvalidJSONError("no JSON value found in the response")
}
