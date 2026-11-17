package xsource

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/netproxy"
	"github.com/taoquo/trove/internal/ports"
)

// xHost is the only host credentials may be sent to. The endpoint is not
// user-configurable: accepting an arbitrary host would turn a credential into
// something the app can be talked into leaking.
const xHost = "x.com"

// defaultTimeout bounds one request. A hung connection must not hang a sync.
const defaultTimeout = 25 * time.Second

// CredentialsFunc returns the current credentials.
//
// It is a function rather than a value so the client always reads the latest
// stored pair, and so tests can supply one without a Keychain.
type CredentialsFunc func(ctx context.Context) (Credentials, error)

// Client implements the bookmark source the sync service depends on.
var _ ports.BookmarkSource = (*Client)(nil)

// Client is the X bookmarks timeline client.
type Client struct {
	// Catalog resolves the operation id.
	Catalog QueryCatalog
	// Credentials supplies the stored cookies.
	Credentials CredentialsFunc
	// HTTP is the underlying client; nil uses a default with a timeout.
	HTTP *http.Client
	// MaxResponseBytes bounds a body; zero uses the package default.
	MaxResponseBytes int64
	// CaptureRawPayload also returns each tweet's raw JSON, driven by the
	// user's setting. See ParseOptions.
	CaptureRawPayload bool
	// Now is the clock, for tests.
	Now func() time.Time
}

// NewClient builds a client with sane defaults.
func NewClient(catalog QueryCatalog, credentials CredentialsFunc) *Client {
	return &Client{
		Catalog:     catalog,
		Credentials: credentials,
		// The shared transport, not http.DefaultTransport: it carries the proxy hook that
		// reads the machine's SystemConfiguration settings. A .app launched from Finder
		// inherits no HTTPS_PROXY, so on a machine where x.com is only reachable through a
		// local proxy every request died at the TLS handshake and the sync reported a
		// network failure while the cookies were fine.
		HTTP: &http.Client{
			Timeout:   defaultTimeout,
			Transport: netproxy.SharedTransport(),
			// Follow at most a couple of redirects, and never carry
			// credentials to another host.
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 3 {
					return fmt.Errorf("xsource: too many redirects")
				}
				if req.URL.Hostname() != xHost {
					return fmt.Errorf("xsource: refusing to follow a redirect to %q", req.URL.Hostname())
				}
				return nil
			},
		},
		Now: time.Now,
	}
}

// FetchPage fetches one page of the bookmarks timeline.
func (c *Client) FetchPage(ctx context.Context, cursor string) (ports.Page, error) {
	requestURL, err := BuildBookmarkTimelineURL(c.queryID(), cursor, PageSize)
	if err != nil {
		return ports.Page{}, err
	}

	payload, err := c.get(ctx, requestURL, "fetch")
	if err != nil {
		return ports.Page{}, err
	}

	seenAt := time.Now()
	if c.Now != nil {
		seenAt = c.Now()
	}

	parsed, err := ParseBookmarkEntries(payload, ParseOptions{
		SeenAt:            seenAt,
		CaptureRawPayload: c.CaptureRawPayload,
	})
	if err != nil {
		return ports.Page{}, err
	}

	return ports.Page{
		Items:        parsed.Bookmarks,
		NextCursor:   parsed.NextCursor,
		RawPayloads:  parsed.RawPayloads,
		Unrecognized: parsed.UnrecognizedEntries,
	}, nil
}

// TestConnection makes a real request and parses it.
//
// It is deliberately a real timeline call rather than a reachability probe: the
// question the settings screen needs answered is "do these cookies work for the
// bookmarks operation", and only that request answers it.
func (c *Client) TestConnection(ctx context.Context) error {
	requestURL, err := BuildBookmarkTimelineURL(c.queryID(), "", 1)
	if err != nil {
		return err
	}
	_, err = c.get(ctx, requestURL, "test-connection")
	return err
}

func (c *Client) queryID() string {
	id, _ := c.Catalog.Resolve()
	return id
}

// get performs the request and returns the decoded payload, having first
// rejected any GraphQL-level error.
func (c *Client) get(ctx context.Context, requestURL, op string) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, cancelledOrDeadline(err, op)
	}

	parsed, err := url.Parse(requestURL)
	if err != nil {
		return nil, newError(domain.ErrUnknown, op, 0, err)
	}
	if parsed.Scheme != "https" || parsed.Hostname() != xHost {
		return nil, newError(domain.ErrUnknown, op, 0,
			fmt.Errorf("refusing to send credentials to %q", parsed.Host))
	}

	credentials, err := c.credentials(ctx)
	if err != nil {
		return nil, err
	}
	if err := credentials.Validate(); err != nil {
		return nil, newError(domain.ErrUnauthorized, op, 0, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, newError(domain.ErrUnknown, op, 0, err)
	}

	// The header set includes content-type on a GET: it has no effect, but it keeps
	// the request fingerprint identical to the one already known to work.
	req.Header.Set("authorization", "Bearer "+BearerToken)
	req.Header.Set("cookie", credentials.CookieHeader())
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-csrf-token", credentials.CSRFToken)
	req.Header.Set("x-twitter-active-user", "yes")
	req.Header.Set("x-twitter-auth-type", "OAuth2Session")
	req.Header.Set("user-agent", UserAgent)

	client := c.HTTP
	if client == nil {
		client = netproxy.NewClient(defaultTimeout)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, wrapTransportError(err, op)
	}
	defer resp.Body.Close()

	limit := c.MaxResponseBytes
	if limit <= 0 {
		limit = maxResponseBytes
	}

	// Read one byte past the limit so an over-long body is detected rather than
	// silently truncated into unparseable JSON.
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, cancelledOrDeadline(err, op)
	}
	if int64(len(body)) > limit {
		return nil, newError(domain.ErrUnknown, op, resp.StatusCode,
			fmt.Errorf("response body exceeded %d bytes", limit))
	}

	if resp.StatusCode != http.StatusOK {
		// Retry-After is a hint; X's rate limiter states its window in
		// x-rate-limit-reset as an epoch second. Reading only the first meant a
		// real 429 whose window was minutes away ended the run after thirty
		// seconds of backoff.
		wait := parseRetryAfter(resp.Header.Get("Retry-After"))
		if wait <= 0 {
			now := time.Now()
			if c.Now != nil {
				now = c.Now()
			}
			wait = parseRateLimitReset(resp.Header.Get("x-rate-limit-reset"), now)
		}
		return nil, newError(classifyHTTP(resp.StatusCode, string(body)), op, resp.StatusCode, nil).
			withRetryAfter(wait)
	}

	payload, err := decodePayload(body)
	if err != nil {
		return nil, newError(domain.ErrResponseSchemaChanged, op, resp.StatusCode, err)
	}

	// A 200 can still carry a GraphQL error. Treating it as an empty timeline is
	// how a library silently stops updating.
	if messages := graphqlMessages(payload); len(messages) > 0 {
		return nil, newError(graphqlErrorKind(messages), op, resp.StatusCode,
			fmt.Errorf("GraphQL error: %s", strings.Join(messages, "; ")))
	}

	return payload, nil
}

func (c *Client) credentials(ctx context.Context) (Credentials, error) {
	if c.Credentials == nil {
		return Credentials{}, newError(domain.ErrUnauthorized, "credentials", 0,
			fmt.Errorf("no credential source configured"))
	}
	credentials, err := c.Credentials(ctx)
	if err != nil {
		// A source that classified its own failure keeps its kind. Only an
		// unclassified one defaults to "expired session", which is what a missing
		// pair means; a locked keychain must not be reported that way.
		var classified *Error
		if errors.As(err, &classified) && classified.Kind != "" {
			return Credentials{}, err
		}
		return Credentials{}, newError(domain.ErrUnauthorized, "credentials", 0, err)
	}
	return credentials, nil
}

func (e *Error) withRetryAfter(d time.Duration) *Error {
	e.RetryAfter = d
	return e
}

// cancelledOrDeadline keeps a cancellation distinguishable from a network fault.
//
// errors.Is rather than ==: net/http wraps a cancellation in *url.Error, so an
// identity check would miss it and the user would be told to check their
// connection right after pressing Stop.
func cancelledOrDeadline(err error, op string) error {
	if errors.Is(err, context.Canceled) {
		return ErrCancelled
	}
	return newError(domain.ErrNetwork, op, 0, err)
}

// wrapTransportError classifies a transport-level failure.
func wrapTransportError(err error, op string) error {
	if errors.Is(err, context.Canceled) {
		return ErrCancelled
	}
	return newError(domain.ErrNetwork, op, 0, err)
}

// decodePayload parses the body with json.Number, so a tweet id is never routed
// through a float64 that cannot hold it exactly.
func decodePayload(body []byte) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()

	var payload map[string]any
	if err := decoder.Decode(&payload); err != nil {
		return nil, fmt.Errorf("response was not a JSON object: %w", err)
	}
	return payload, nil
}

// graphqlMessages extracts the messages of a top-level GraphQL errors array.
func graphqlMessages(payload map[string]any) []string {
	raw, ok := payload["errors"]
	if !ok {
		return nil
	}
	list := slice(raw)
	if len(list) == 0 {
		return nil
	}

	messages := make([]string, 0, len(list))
	for _, item := range list {
		entry := obj(item)
		if entry == nil {
			continue
		}
		if message := str(entry["message"]); message != "" {
			messages = append(messages, message)
		}
	}
	return messages
}

// parseRetryAfter reads a Retry-After header in either of its two forms.
//
// An unparseable or absent value returns zero, which makes the caller fall back
// to exponential backoff rather than sleeping on a bogus number.
func parseRetryAfter(value string) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}

	if seconds, err := time.ParseDuration(value + "s"); err == nil {
		if seconds <= 0 {
			return 0
		}
		return seconds
	}

	if when, err := http.ParseTime(value); err == nil {
		if wait := time.Until(when); wait > 0 {
			return wait
		}
	}

	return 0
}

// parseRateLimitReset turns X's epoch-second reset header into a wait.
//
// A value in the past, or one that is not a number, returns zero so the caller
// falls back to exponential backoff.
func parseRateLimitReset(value string, now time.Time) time.Duration {
	seconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || seconds <= 0 {
		return 0
	}
	wait := time.Unix(seconds, 0).Sub(now)
	if wait <= 0 {
		return 0
	}
	return wait
}

// orderedFeatures marshals the feature flags in declaration order, so the query
// string is byte-stable across runs and against fixtures.
type orderedFeatures []FeatureFlag

func (f orderedFeatures) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, flag := range f {
		if i > 0 {
			buf.WriteByte(',')
		}
		key, err := json.Marshal(flag.Name)
		if err != nil {
			return nil, err
		}
		value, err := json.Marshal(flag.Value)
		if err != nil {
			return nil, err
		}
		buf.Write(key)
		buf.WriteByte(':')
		buf.Write(value)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// BuildBookmarkTimelineURL builds the request URL.
//
// count is clamped to PageSize, matching the endpoint's own ceiling: asking for
// more than 20 does not return more.
func BuildBookmarkTimelineURL(queryID, cursor string, count int) (string, error) {
	if strings.TrimSpace(queryID) == "" {
		return "", newError(domain.ErrQueryIDStale, "build-url", 0,
			fmt.Errorf("no query id configured"))
	}

	if count <= 0 {
		count = PageSize
	}
	if count > PageSize {
		count = PageSize
	}

	variables := orderedVariables{{"count", count}}
	if cursor != "" {
		variables = append(variables, orderedVariable{"cursor", cursor})
	}
	variables = append(variables, orderedVariable{"includePromotedContent", false})

	variablesJSON, err := json.Marshal(variables)
	if err != nil {
		return "", newError(domain.ErrUnknown, "build-url", 0, err)
	}
	featuresJSON, err := json.Marshal(orderedFeatures(BookmarksFeatures))
	if err != nil {
		return "", newError(domain.ErrUnknown, "build-url", 0, err)
	}

	query := url.Values{}
	query.Set("variables", string(variablesJSON))
	query.Set("features", string(featuresJSON))

	return "https://" + xHost + "/i/api/graphql/" + queryID + "/Bookmarks?" + query.Encode(), nil
}

// orderedVariables marshals the variables object in a fixed key order and omits
// cursor entirely when there is none, matching the original request.
type orderedVariables []orderedVariable

type orderedVariable struct {
	Key   string
	Value any
}

func (v orderedVariables) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, entry := range v {
		if i > 0 {
			buf.WriteByte(',')
		}
		key, err := json.Marshal(entry.Key)
		if err != nil {
			return nil, err
		}
		value, err := json.Marshal(entry.Value)
		if err != nil {
			return nil, err
		}
		buf.Write(key)
		buf.WriteByte(':')
		buf.Write(value)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}
