// Package xsource talks to X's bookmarks timeline.
//
// It ports the wire protocol the old extension used, verified line by line
// against src/lib/x/client.ts, and adds the failure handling the extension
// lacked: cancellation, bounded retries, cursor loop detection, response size
// limits and a usable error taxonomy.
//
// See docs/x-sync-protocol.md.
package xsource

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/taoquo/trove/apps/macos/internal/domain"
)

// ErrCancelled reports that the caller cancelled the operation. It is not a
// failure: the sync records a cancelled run, not an error.
var ErrCancelled = errors.New("xsource: cancelled")

// Error is a classified failure from the X timeline.
//
// Every failure the caller can act on carries a domain.SyncErrorKind, so the UI
// can say "your login expired" instead of showing a stack trace, and the retry
// policy can tell a rate limit from a bad credential.
type Error struct {
	// Kind is the classification.
	Kind domain.SyncErrorKind
	// Status is the HTTP status, or 0 when the failure happened before a
	// response existed.
	Status int
	// Op names the step that failed, for diagnostics.
	Op string
	// RetryAfter is set when the server asked us to wait.
	RetryAfter time.Duration
	// Err is the underlying cause.
	Err error
}

func (e *Error) Error() string {
	var b strings.Builder
	b.WriteString("xsource")
	if e.Op != "" {
		b.WriteString(": ")
		b.WriteString(e.Op)
	}
	if e.Status != 0 {
		fmt.Fprintf(&b, ": HTTP %d", e.Status)
	}
	fmt.Fprintf(&b, ": %s", e.Kind)
	if e.Err != nil {
		fmt.Fprintf(&b, ": %v", e.Err)
	}
	return b.String()
}

func (e *Error) Unwrap() error { return e.Err }

// Is lets errors.Is match on the kind.
func (e *Error) Is(target error) bool {
	var other *Error
	if !errors.As(target, &other) {
		return false
	}
	return other.Kind == e.Kind
}

// newError builds a classified error.
func newError(kind domain.SyncErrorKind, op string, status int, err error) *Error {
	return &Error{Kind: kind, Op: op, Status: status, Err: err}
}

// KindOf returns the classification of err, or an empty kind when err is nil.
//
// Cancellation is deliberately *not* given a kind: the caller distinguishes it
// with errors.Is(err, ErrCancelled). The old extension folded an abort into
// "network error", which told the user to check their connection after they had
// pressed Stop.
func KindOf(err error) domain.SyncErrorKind {
	if err == nil {
		return ""
	}
	var classified *Error
	if errors.As(err, &classified) {
		return classified.Kind
	}
	if errors.Is(err, ErrCancelled) || errors.Is(err, context.Canceled) {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return domain.ErrNetwork
	}
	return domain.ErrUnknown
}

// IsCancelled reports whether err is a user cancellation.
func IsCancelled(err error) bool {
	return errors.Is(err, ErrCancelled) || errors.Is(err, context.Canceled)
}

// IsRetryable reports whether retrying the same request could succeed.
//
// Credential failures are not retryable on purpose: retrying an invalid token
// can never work, and looping on it is what the spec forbids.
func IsRetryable(err error) bool {
	var classified *Error
	if !errors.As(err, &classified) {
		return false
	}
	switch classified.Kind {
	case domain.ErrRateLimited, domain.ErrNetwork:
		return true
	default:
		return false
	}
}

// RetryAfter returns the server-requested wait, or zero.
func RetryAfter(err error) time.Duration {
	var classified *Error
	if errors.As(err, &classified) {
		return classified.RetryAfter
	}
	return 0
}

// classifyHTTP maps a status and response body to a kind.
//
// The body matters: X answers 400 for a stale query id and 403 for both an
// expired session and a rejected CSRF token, so the status alone cannot say what
// to tell the user.
func classifyHTTP(status int, body string) domain.SyncErrorKind {
	lower := strings.ToLower(body)

	switch {
	case status == http.StatusUnauthorized:
		return domain.ErrUnauthorized

	case status == http.StatusForbidden:
		// A rejected csrf token also comes back as 403. Telling the user to
		// re-authorize when the cookie is fine would send them in circles.
		if strings.Contains(lower, "csrf") || strings.Contains(lower, "authenticity") {
			return domain.ErrCSRFInvalid
		}
		return domain.ErrUnauthorized

	case status == http.StatusTooManyRequests:
		return domain.ErrRateLimited

	case status == http.StatusBadRequest:
		// A stale operation/query id shows up as a 400 whose body names the
		// operation. Everything else is a malformed request we cannot fix.
		if mentionsOperation(lower) {
			return domain.ErrQueryIDStale
		}
		return domain.ErrUnknown

	case status >= 500:
		// Transient server-side failure: retryable, so it is grouped with the
		// network case rather than being terminal.
		return domain.ErrNetwork

	case status >= 400:
		return domain.ErrUnknown

	default:
		return domain.ErrUnknown
	}
}

func mentionsOperation(lowerBody string) bool {
	for _, needle := range []string{"operation", "queryid", "query id", "not found"} {
		if strings.Contains(lowerBody, needle) {
			return true
		}
	}
	return false
}

// graphqlErrorKind classifies a GraphQL "errors" array carried in an otherwise
// successful HTTP response.
//
// The old implementation checked only response.ok, so a 200 whose body said the
// request had failed was parsed as a timeline and reported "no new bookmarks" —
// the user's library silently stopped updating.
func graphqlErrorKind(messages []string) domain.SyncErrorKind {
	joined := strings.ToLower(strings.Join(messages, " "))
	if joined == "" {
		return domain.ErrUnknown
	}
	if mentionsOperation(joined) {
		return domain.ErrQueryIDStale
	}
	if strings.Contains(joined, "csrf") || strings.Contains(joined, "authenticity") {
		return domain.ErrCSRFInvalid
	}
	if strings.Contains(joined, "rate limit") || strings.Contains(joined, "too many requests") {
		return domain.ErrRateLimited
	}
	return domain.ErrUnknown
}
