package xsource

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/taoquo/trove/internal/domain"
)

// roundTripFunc lets a test answer a request without a network, while leaving the
// URL that reached the client fully validated.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func respond(status int, body string, header map[string]string) roundTripFunc {
	return func(*http.Request) (*http.Response, error) {
		resp := &http.Response{
			StatusCode: status,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{},
		}
		for k, v := range header {
			resp.Header.Set(k, v)
		}
		return resp, nil
	}
}

func testClient(t roundTripFunc) *Client {
	return &Client{
		Catalog:     NewQueryCatalog(""),
		Credentials: func(context.Context) (Credentials, error) { return validCredentials(), nil },
		HTTP:        &http.Client{Transport: t, Timeout: 5 * time.Second},
	}
}

func validCredentials() Credentials {
	return Credentials{AuthToken: "auth-123", CSRFToken: "csrf-456"}
}

func TestBuildBookmarkTimelineURL(t *testing.T) {
	raw, err := BuildBookmarkTimelineURL(BuiltinQueryID, "cursor-1", 200)
	if err != nil {
		t.Fatalf("BuildBookmarkTimelineURL: %v", err)
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	if parsed.Scheme != "https" || parsed.Hostname() != "x.com" {
		t.Errorf("origin = %s://%s, want https://x.com", parsed.Scheme, parsed.Hostname())
	}
	if want := "/i/api/graphql/" + BuiltinQueryID + "/Bookmarks"; parsed.Path != want {
		t.Errorf("path = %q, want %q", parsed.Path, want)
	}

	var variables map[string]any
	if err := json.Unmarshal([]byte(parsed.Query().Get("variables")), &variables); err != nil {
		t.Fatalf("variables is not JSON: %v", err)
	}
	// count is clamped to the endpoint's own ceiling of 20.
	if got := variables["count"]; got != float64(PageSize) {
		t.Errorf("count = %v, want %d", got, PageSize)
	}
	if got := variables["cursor"]; got != "cursor-1" {
		t.Errorf("cursor = %v, want cursor-1", got)
	}
	if got := variables["includePromotedContent"]; got != false {
		t.Errorf("includePromotedContent = %v, want false", got)
	}

	var features map[string]any
	if err := json.Unmarshal([]byte(parsed.Query().Get("features")), &features); err != nil {
		t.Fatalf("features is not JSON: %v", err)
	}
	if len(features) != len(BookmarksFeatures) {
		t.Errorf("features has %d entries, want %d", len(features), len(BookmarksFeatures))
	}
	if features["longform_notetweets_consumption_enabled"] != true {
		t.Error("the note-tweet feature flag must be on, or long-form posts come back truncated")
	}
}

func TestBuildBookmarkTimelineURLOmitsCursorOnFirstPage(t *testing.T) {
	raw, err := BuildBookmarkTimelineURL(BuiltinQueryID, "", PageSize)
	if err != nil {
		t.Fatalf("BuildBookmarkTimelineURL: %v", err)
	}
	parsed, _ := url.Parse(raw)

	var variables map[string]any
	json.Unmarshal([]byte(parsed.Query().Get("variables")), &variables)

	if _, present := variables["cursor"]; present {
		t.Error("the first page must omit cursor entirely, not send it empty")
	}
}

func TestBuildBookmarkTimelineURLRejectsEmptyQueryID(t *testing.T) {
	if _, err := BuildBookmarkTimelineURL("", "", PageSize); err == nil {
		t.Fatal("an empty query id must be refused rather than sent")
	} else if KindOf(err) != domain.ErrQueryIDStale {
		t.Errorf("kind = %q, want %q", KindOf(err), domain.ErrQueryIDStale)
	}
}

func TestRequestCarriesTheVerifiedHeaderSet(t *testing.T) {
	var seen *http.Request

	client := testClient(func(r *http.Request) (*http.Response, error) {
		seen = r
		return respond(200, validTimelinePage, nil)(r)
	})

	if _, err := client.FetchPage(context.Background(), ""); err != nil {
		t.Fatalf("FetchPage: %v", err)
	}

	want := map[string]string{
		"authorization":         "Bearer " + BearerToken,
		"content-type":          "application/json",
		"x-csrf-token":          "csrf-456",
		"x-twitter-active-user": "yes",
		"x-twitter-auth-type":   "OAuth2Session",
		"user-agent":            UserAgent,
	}
	for name, value := range want {
		if got := seen.Header.Get(name); got != value {
			t.Errorf("header %s = %q, want %q", name, got, value)
		}
	}

	cookie := seen.Header.Get("cookie")
	if !strings.Contains(cookie, "auth_token=auth-123") || !strings.Contains(cookie, "ct0=csrf-456") {
		t.Errorf("cookie = %q, want both configured values", cookie)
	}
}

func TestClientRefusesNonXHost(t *testing.T) {
	client := testClient(func(*http.Request) (*http.Response, error) {
		t.Fatal("no request may be attempted to a non-X host")
		return nil, nil
	})

	_, err := client.get(context.Background(), "https://evil.example.com/i/api/graphql/x/Bookmarks", "fetch")
	if err == nil {
		t.Fatal("a request to another host must be refused, or credentials could be exfiltrated")
	}
}

func TestClientRefusesRedirectToAnotherHost(t *testing.T) {
	// Keep the redirect policy the constructor installs; swap only the transport.
	client := NewClient(NewQueryCatalog(""), func(context.Context) (Credentials, error) {
		return validCredentials(), nil
	})
	client.HTTP.Transport = respond(302, "", map[string]string{"Location": "https://evil.example.com/x"})

	_, err := client.FetchPage(context.Background(), "")
	if err == nil {
		t.Fatal("a cross-host redirect must be refused, not followed with credentials")
	}
}
func TestStatusClassification(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		wantKind   domain.SyncErrorKind
		retryable  bool
		retryAfter time.Duration
	}{
		{"401 is an expired session", 401, `{}`, domain.ErrUnauthorized, false, 0},
		{"403 without a csrf hint is an expired session", 403, `{"errors":[]}`, domain.ErrUnauthorized, false, 0},
		{"403 naming csrf is a token problem", 403, `{"message":"invalid csrf token"}`, domain.ErrCSRFInvalid, false, 0},
		{"429 is a rate limit", 429, `{}`, domain.ErrRateLimited, true, 0},
		{"429 honours Retry-After", 429, `{}`, domain.ErrRateLimited, true, 7 * time.Second},
		{"400 naming the operation is a stale query id", 400, `{"errors":[{"message":"The following operation was not found"}]}`, domain.ErrQueryIDStale, false, 0},
		{"400 otherwise is not actionable", 400, `{"errors":[{"message":"nope"}]}`, domain.ErrUnknown, false, 0},
		{"500 is transient", 500, `{}`, domain.ErrNetwork, true, 0},
		{"503 is transient", 503, `{}`, domain.ErrNetwork, true, 0},
		{"418 is unknown", 418, `{}`, domain.ErrUnknown, false, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			header := map[string]string{}
			if tc.retryAfter > 0 {
				header["Retry-After"] = "7"
			}
			client := testClient(respond(tc.status, tc.body, header))

			_, err := client.FetchPage(context.Background(), "")
			if err == nil {
				t.Fatal("expected an error")
			}
			if got := KindOf(err); got != tc.wantKind {
				t.Errorf("kind = %q, want %q", got, tc.wantKind)
			}
			if got := IsRetryable(err); got != tc.retryable {
				t.Errorf("IsRetryable = %v, want %v", got, tc.retryable)
			}
			if got := RetryAfter(err); got != tc.retryAfter {
				t.Errorf("RetryAfter = %v, want %v", got, tc.retryAfter)
			}
		})
	}
}

// TestGraphQLErrorsInA200ResponseFail guards against a silent failure: checking only
// response.ok would parse a 200 whose body said the request failed as an empty
// timeline, and the library would simply stop updating.
func TestGraphQLErrorsInA200ResponseFail(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantKind domain.SyncErrorKind
	}{
		{"an operation error is a stale query id",
			`{"errors":[{"message":"The following operation was not found: Bookmarks"}]}`,
			domain.ErrQueryIDStale},
		{"a csrf error",
			`{"errors":[{"message":"csrf token invalid"}]}`,
			domain.ErrCSRFInvalid},
		{"a rate limit error",
			`{"errors":[{"message":"Rate limit exceeded"}]}`,
			domain.ErrRateLimited},
		{"anything else is unknown",
			`{"errors":[{"message":"something went wrong"}]}`,
			domain.ErrUnknown},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := testClient(respond(200, tc.body, nil))

			_, err := client.FetchPage(context.Background(), "")
			if err == nil {
				t.Fatal("a GraphQL error in a 200 response must fail, not read as an empty page")
			}
			if got := KindOf(err); got != tc.wantKind {
				t.Errorf("kind = %q, want %q", got, tc.wantKind)
			}
		})
	}
}

func TestClientRejectsOversizedResponse(t *testing.T) {
	client := testClient(respond(200, strings.Repeat("x", 4096), nil))
	client.MaxResponseBytes = 1024

	_, err := client.FetchPage(context.Background(), "")
	if err == nil {
		t.Fatal("a body past the limit must be refused")
	}
	if !strings.Contains(err.Error(), "exceeded") {
		t.Errorf("err = %v, want a size complaint", err)
	}
}

func TestClientReportsNonJSONBodyAsASchemaChange(t *testing.T) {
	client := testClient(respond(200, "<html>not json</html>", nil))

	_, err := client.FetchPage(context.Background(), "")
	if err == nil {
		t.Fatal("a non-JSON body must fail")
	}
	if KindOf(err) != domain.ErrResponseSchemaChanged {
		t.Errorf("kind = %q, want %q", KindOf(err), domain.ErrResponseSchemaChanged)
	}
}

func TestClientRefusesMissingCredentials(t *testing.T) {
	client := testClient(respond(200, validTimelinePage, nil))
	client.Credentials = func(context.Context) (Credentials, error) { return Credentials{}, nil }

	_, err := client.FetchPage(context.Background(), "")
	if err == nil {
		t.Fatal("missing credentials must fail before any request")
	}
	if KindOf(err) != domain.ErrUnauthorized {
		t.Errorf("kind = %q, want %q", KindOf(err), domain.ErrUnauthorized)
	}
}

// TestClientKeepsCancellationDistinctFromNetworkFailure pins the distinction: an
// abort must not be reported as a network error, which would tell the user to
// check their connection right after they pressed Stop.
func TestClientKeepsCancellationDistinctFromNetworkFailure(t *testing.T) {
	client := testClient(func(*http.Request) (*http.Response, error) {
		return nil, context.Canceled
	})

	_, err := client.FetchPage(context.Background(), "")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !IsCancelled(err) {
		t.Errorf("err = %v, want a cancellation", err)
	}
	if KindOf(err) != "" {
		t.Errorf("kind = %q, want empty: a cancellation is not an error kind", KindOf(err))
	}
}

func TestClientCancelledBeforeRequest(t *testing.T) {
	client := testClient(func(*http.Request) (*http.Response, error) {
		t.Fatal("no request may go out for an already-cancelled context")
		return nil, nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := client.FetchPage(ctx, ""); !IsCancelled(err) {
		t.Errorf("err = %v, want a cancellation", err)
	}
}

func TestTestConnectionMakesARealRequest(t *testing.T) {
	var requested bool
	client := testClient(func(r *http.Request) (*http.Response, error) {
		requested = true
		// Asking for one item is enough to prove the credentials work.
		variables := r.URL.Query().Get("variables")
		if !strings.Contains(variables, `"count":1`) {
			t.Errorf("variables = %q, want a count of 1", variables)
		}
		return respond(200, validTimelinePage, nil)(r)
	})

	if err := client.TestConnection(context.Background()); err != nil {
		t.Fatalf("TestConnection: %v", err)
	}
	if !requested {
		t.Error("TestConnection must actually perform a request")
	}
}

func TestTestConnectionSurfacesBadCredentials(t *testing.T) {
	client := testClient(respond(401, `{}`, nil))
	if err := client.TestConnection(context.Background()); err == nil {
		t.Fatal("a 401 must fail the connection test")
	} else if KindOf(err) != domain.ErrUnauthorized {
		t.Errorf("kind = %q, want %q", KindOf(err), domain.ErrUnauthorized)
	}
}

const validTimelinePage = `{"data":{"bookmark_timeline_v2":{"timeline":{"instructions":[
	{"type":"TimelineAddEntries","entries":[
		{"entryId":"tweet-1","content":{"itemContent":{"tweet_results":{"result":{
			"rest_id":"1","legacy":{"full_text":"hello"}}}}}},
		{"entryId":"cursor-bottom-end","content":{"value":""}}
	]}]}}}}`

func TestParseRetryAfterForms(t *testing.T) {
	tests := []struct {
		value string
		want  time.Duration
	}{
		{"", 0},
		{"7", 7 * time.Second},
		{"0", 0},
		{"-1", 0},
		{"garbage", 0},
	}
	for _, tc := range tests {
		if got := parseRetryAfter(tc.value); got != tc.want {
			t.Errorf("parseRetryAfter(%q) = %v, want %v", tc.value, got, tc.want)
		}
	}

	// An HTTP-date form is also legal.
	future := time.Now().Add(30 * time.Second).UTC().Format(http.TimeFormat)
	if got := parseRetryAfter(future); got <= 0 {
		t.Errorf("parseRetryAfter(%q) = %v, want a positive wait", future, got)
	}
}

// TestRateLimitResetBecomesAWait pins the second 429 header X sends.
//
// x-rate-limit-reset is the epoch second a window reopens. Without reading it a
// 429 with no Retry-After waited only the backoff and hit the same window again,
// which is the run that died at page 101.
func TestRateLimitResetBecomesAWait(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)

	tests := []struct {
		name  string
		value string
		want  time.Duration
	}{
		{"a window in the future", "1800000090", 90 * time.Second},
		{"a window that already passed", "1799999990", 0},
		{"the current second", "1800000000", 0},
		{"empty", "", 0},
		{"not a number", "soon", 0},
		{"negative", "-1", 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseRateLimitReset(tc.value, now); got != tc.want {
				t.Errorf("parseRateLimitReset(%q) = %v, want %v", tc.value, got, tc.want)
			}
		})
	}
}

func TestCookieHeaderDeduplicates(t *testing.T) {
	credentials := Credentials{
		AuthToken:    "a",
		CSRFToken:    "c",
		ExtraCookies: "ct0=duplicate; guest_id=g1",
	}
	header := credentials.CookieHeader()

	if strings.Count(header, "ct0=") != 1 {
		t.Errorf("cookie = %q, want ct0 exactly once", header)
	}
	if !strings.Contains(header, "guest_id=g1") {
		t.Errorf("cookie = %q, want the extra cookie included", header)
	}
}

func TestCredentialsValidate(t *testing.T) {
	if err := (Credentials{AuthToken: "a", CSRFToken: "c"}).Validate(); err != nil {
		t.Errorf("complete credentials must validate: %v", err)
	}
	for _, tc := range []Credentials{{}, {AuthToken: "a"}, {CSRFToken: "c"}, {AuthToken: "  ", CSRFToken: "c"}} {
		if err := tc.Validate(); !errors.Is(err, ErrMissingCredentials) {
			t.Errorf("%+v must be rejected as incomplete, got %v", tc, err)
		}
	}
}

func TestExtractCSRFToken(t *testing.T) {
	tests := []struct {
		header  string
		want    string
		wantErr bool
	}{
		{"foo=bar; ct0=token123; baz=1", "token123", false},
		{"ct0=at-start", "at-start", false},
		{"nope=1", "", true},
		{"", "", true},
	}
	for _, tc := range tests {
		got, err := ExtractCSRFToken(tc.header)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ExtractCSRFToken(%q) must fail", tc.header)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("ExtractCSRFToken(%q) = %q, %v; want %q", tc.header, got, err, tc.want)
		}
	}
}

func TestQueryCatalogPriority(t *testing.T) {
	builtin, source := NewQueryCatalog("").Resolve()
	if builtin != BuiltinQueryID || source != QueryIDFromBuiltin {
		t.Errorf("got %q from %q, want the builtin", builtin, source)
	}

	override, source := NewQueryCatalog("  custom-id  ").Resolve()
	if override != "custom-id" || source != QueryIDFromOverride {
		t.Errorf("got %q from %q, want the trimmed override", override, source)
	}
}
