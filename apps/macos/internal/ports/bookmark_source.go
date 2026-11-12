// Package ports holds the interfaces the use cases depend on. Implementations
// live in the adapter packages, so business logic can be tested without a
// network, a database or a window.
package ports

import (
	"context"
	"encoding/json"

	"github.com/taoquo/trove/apps/macos/internal/domain"
)

// BookmarkSource fetches saved posts from X, one page at a time.
//
// It deliberately knows nothing about the database, the UI or the sync policy:
// it answers "what is on this page", and the sync service decides what to do
// with it.
type BookmarkSource interface {
	// FetchPage returns one page. An empty cursor starts from the beginning.
	//
	// A returned error must be classified so the caller can decide whether to
	// retry, stop, or ask the user to re-authorize; see the xsource package.
	FetchPage(ctx context.Context, cursor string) (Page, error)

	// TestConnection makes one real request and parses it. It exists so the
	// settings screen can prove the stored credentials work *before* saving
	// them, rather than discovering a bad token at the first sync.
	TestConnection(ctx context.Context) error
}

// Page is one page of bookmarks.
type Page struct {
	Items []domain.Bookmark
	// NextCursor is empty when X reports no further page.
	NextCursor string
	// RawPayloads maps tweet id to the raw response for that post. It is empty
	// unless raw capture is enabled, and nothing but the diagnostics table reads
	// it: the list must render without any raw JSON.
	RawPayloads map[string]json.RawMessage
}
