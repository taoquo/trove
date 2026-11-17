package workspace

import (
	"context"
	"errors"
	"fmt"
	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/ports"
	"net/url"
	"strings"
	"time"
)

// AddTags applies tags to bookmarks.
//
// A tag name that does not exist yet is created, and an existing one is reused by
// its canonical key, so "AI Agent" does not become a second tag beside
// "ai-agents".
func (s *Service) AddTags(ctx context.Context, tweetIDs []string, tagNames []string) (int, error) {
	if len(tweetIDs) == 0 || len(tagNames) == 0 {
		return 0, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.clock.Now()
	resolved := make([]domain.Tag, 0, len(tagNames))

	for _, name := range tagNames {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}

		existing, found, err := s.repos.Tags.FindByName(ctx, trimmed)
		if err != nil {
			return 0, err
		}
		if found {
			resolved = append(resolved, existing)
			continue
		}

		display := domain.NormalizeTagDisplayName(trimmed)
		key := domain.CanonicalTagKey(display)
		if key == "" {
			// A name that normalizes away would create an unnamed tag.
			continue
		}

		tag := domain.Tag{
			ID:           s.newID("tag"),
			Name:         display,
			CanonicalKey: key,
			CreatedAt:    domain.FormatTimestamp(now),
			UpdatedAt:    domain.FormatTimestamp(now),
		}
		if err := s.repos.Tags.Create(ctx, tag); err != nil {
			return 0, err
		}
		resolved = append(resolved, tag)
	}

	if len(resolved) == 0 {
		return 0, nil
	}

	relations := make([]domain.BookmarkTag, 0, len(tweetIDs)*len(resolved))
	for _, tweetID := range tweetIDs {
		for _, tag := range resolved {
			relations = append(relations, domain.BookmarkTag{
				BookmarkID: tweetID,
				TagID:      tag.ID,
				CreatedAt:  now,
				// A tag a human applied by hand is manual, so no later automatic
				// pass may remove it.
				Source: domain.TagSourceManual,
			})
		}
	}

	if err := s.repos.Tags.AttachMany(ctx, relations); err != nil {
		return 0, err
	}

	if err := s.bumpGeneration(); err != nil {
		// A stale-version read is recoverable; the write already succeeded.
		_ = err
	}
	return len(relations), nil
}

// RemoveTags removes tags from bookmarks.
func (s *Service) RemoveTags(ctx context.Context, tweetIDs []string, tagIDs []string) (int, error) {
	if len(tweetIDs) == 0 || len(tagIDs) == 0 {
		return 0, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	pairs := make([]ports.BookmarkTagPair, 0, len(tweetIDs)*len(tagIDs))
	for _, tweetID := range tweetIDs {
		for _, tagID := range tagIDs {
			pairs = append(pairs, ports.BookmarkTagPair{BookmarkID: tweetID, TagID: tagID})
		}
	}

	removed, err := s.repos.Tags.DetachMany(ctx, pairs)
	if err != nil {
		return 0, err
	}
	_ = s.bumpGeneration()
	return removed, nil
}

// MoveToList moves bookmarks into a list.
func (s *Service) MoveToList(ctx context.Context, tweetIDs []string, listID string) error {
	if len(tweetIDs) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, found, err := s.repos.Lists.Get(ctx, listID); err != nil {
		return err
	} else if !found {
		return fmt.Errorf("workspace: no list %q", listID)
	}

	if err := s.repos.Bookmarks.AssignToList(ctx, tweetIDs, listID, s.clock.Now()); err != nil {
		return err
	}
	_ = s.bumpGeneration()
	return nil
}

// SetArchived moves a bookmark into or out of the archive.
//
// The generation moves even when the write changed nothing else, because every view
// decides whether to show archived rows and a stale one would keep listing a row that
// just left.
func (s *Service) SetArchived(ctx context.Context, tweetID string, archived bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	at := time.Time{}
	if archived {
		at = s.clock.Now()
	}
	if err := s.repos.Bookmarks.SetArchived(ctx, tweetID, at); err != nil {
		return err
	}
	_ = s.bumpGeneration()
	return nil
}

// SetArchivedMany moves a batch into or out of the archive.
//
// The generation moves once for the batch, not once per bookmark: the rows left or
// joined the archive together, and every view read before the write answers a question
// that no longer exists.
func (s *Service) SetArchivedMany(ctx context.Context, tweetIDs []string, archived bool) error {
	if len(tweetIDs) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	at := time.Time{}
	if archived {
		at = s.clock.Now()
	}
	if err := s.repos.Bookmarks.SetArchivedMany(ctx, tweetIDs, at); err != nil {
		return err
	}
	_ = s.bumpGeneration()
	return nil
}

// DeleteBookmarks removes bookmarks.
//
// Pending suggestions go with them; resolved ones stay, because they are the
// negative cache that stops the same tag being offered again.
func (s *Service) DeleteBookmarks(ctx context.Context, tweetIDs []string) (int, error) {
	if len(tweetIDs) == 0 {
		return 0, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.repos.Suggestions.DropPendingForBookmarks(ctx, tweetIDs); err != nil {
		return 0, err
	}

	removed, err := s.repos.Bookmarks.DeleteMany(ctx, tweetIDs)
	if err != nil {
		return 0, err
	}
	_ = s.bumpGeneration()
	return removed, nil
}

// AddBookmark stores a post the user entered by hand.
//
// This entry point takes a link rather than a page: a standalone manager has no page to
// read a tweet from, so the link is the input and the record is built from what can be read
// out of it.
//
// The record is merged into whatever is already stored rather than written over it --
// see domain.MergeDraft for why the rule is the opposite of the sync path's.
func (s *Service) AddBookmark(ctx context.Context, rawURL, text string) (domain.Bookmark, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tweetID, ok := domain.TweetIDFromURL(rawURL)
	if !ok {
		return domain.Bookmark{}, ErrNotAPostLink
	}
	handle := handleFromTweetURL(rawURL)
	now := s.clock.Now()

	draft := domain.Bookmark{
		TweetID:      tweetID,
		TweetURL:     postURL(tweetID, handle),
		AuthorHandle: handle,
		Text:         strings.TrimSpace(text),
		// sourceKind is left alone: a site-tweet save does not set it either, so a
		// hand-entered post is not a third shape.
		CreatedAtOnX: "",
		LastSeenAt:   now,
		SavedAt:      now,
		UpdatedAt:    now,
	}

	stored, found, err := s.repos.Bookmarks.Get(ctx, tweetID)
	if err != nil {
		return domain.Bookmark{}, err
	}
	merged := draft
	if found {
		merged = domain.MergeDraft(stored, draft, now)
	} else {
		merged = domain.MergeDraft(domain.Bookmark{}, draft, now)
	}

	if _, err := s.repos.Bookmarks.UpsertMany(ctx, []domain.Bookmark{merged}); err != nil {
		return domain.Bookmark{}, err
	}
	if err := s.bumpGeneration(); err != nil {
		return domain.Bookmark{}, err
	}
	return merged, nil
}

// ErrNotAPostLink is returned for input that is not a link to an X post.
var ErrNotAPostLink = errors.New("workspace: that is not a link to a post")

// postURL is the canonical link for a post, with the handle when one is known.
//
// A synced bookmark stores whatever tweetUrl the page carried. Rebuilding it here keeps one
// shape in the library, so two adds of the same post cannot differ by tracking parameters.
func postURL(tweetID, handle string) string {
	if handle == "" {
		return "https://x.com/i/web/status/" + tweetID
	}
	return "https://x.com/" + handle + "/status/" + tweetID
}

// handleFromTweetURL reads the handle out of a post link, if it has one.
func handleFromTweetURL(rawURL string) string {
	trimmed := strings.TrimSpace(rawURL)
	if !strings.Contains(trimmed, "://") {
		trimmed = "https://" + trimmed
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return ""
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	for i := 1; i < len(segments); i++ {
		if segments[i-1] != "status" && segments[i-1] != "statuses" {
			continue
		}
		// The handle is the segment before "status"; /i/web/status has none.
		if i-2 >= 0 {
			candidate := segments[i-2]
			if candidate == "i" || candidate == "web" {
				return ""
			}
			return candidate
		}
	}
	return ""
}
