package autotag

import (
	"sort"
	"strings"
	"time"

	"github.com/taoquo/x-master/apps/macos/internal/domain"
)

// HeuristicInput is everything the rule-based pass needs.
type HeuristicInput struct {
	Bookmarks []domain.Bookmark
	Tags      []domain.Tag
	// BookmarkTags are the existing relations, used both to skip already-tagged
	// posts and to learn an author's affinity.
	BookmarkTags []domain.BookmarkTag
	// ExistingSuggestionIDs is the negative cache: a suggestion the user already
	// accepted or rejected must never be offered again, and a pending one must not
	// be duplicated.
	ExistingSuggestionIDs map[string]bool
	Settings              HeuristicSettings
	// BlockedTagKeys are canonical keys the user blocked by deleting a tag.
	BlockedTagKeys map[string]bool
	Now            time.Time
}

// SuggestHeuristics produces suggestions from the three rules.
//
// A post that already carries any tag is skipped entirely. That is the extension's
// behaviour and it is deliberate: heuristics are for the backlog, not for second-
// guessing tags the user already applied.
func SuggestHeuristics(input HeuristicInput) []domain.TagSuggestion {
	tagged := make(map[string]bool, len(input.BookmarkTags))
	for _, relation := range input.BookmarkTags {
		tagged[relation.BookmarkID] = true
	}

	// Within-run dedupe, so two rules proposing the same tag for the same post
	// produce one suggestion.
	seen := make(map[string]bool, len(input.ExistingSuggestionIDs))
	for id := range input.ExistingSuggestionIDs {
		seen[id] = true
	}

	var suggestions []domain.TagSuggestion

	add := func(bookmark domain.Bookmark, tagName string, source domain.SuggestionSource, confidence float64) {
		if IsBlocked(input.BlockedTagKeys, tagName) {
			return
		}
		id := domain.SuggestionID(bookmark.TweetID, tagName)
		if seen[id] {
			return
		}
		// Reject a name that normalizes away, which would create an unnamed tag.
		if domain.CanonicalTagKey(tagName) == "" {
			return
		}
		seen[id] = true
		suggestions = append(suggestions, domain.TagSuggestion{
			ID:         id,
			BookmarkID: bookmark.TweetID,
			TagName:    domain.NormalizeTagDisplayName(tagName),
			Source:     source,
			Confidence: confidence,
			Status:     domain.StatusPending,
			CreatedAt:  input.Now,
		})
	}

	// --- keyword rules: the only rule that applies regardless of other config ---
	keywordRules := NormalizeKeywordRules(input.Settings.KeywordRules)
	for _, bookmark := range input.Bookmarks {
		if tagged[bookmark.TweetID] {
			continue
		}
		lowered := strings.ToLower(bookmark.Text)
		for _, rule := range keywordRules {
			if strings.Contains(lowered, strings.ToLower(rule.Keyword)) {
				add(bookmark, rule.TagName, domain.SourceKeyword, KeywordConfidence)
			}
		}
	}

	if input.Settings.UseHashtags {
		applyHashtagRule(input, tagged, add)
	}
	if input.Settings.UseAuthors {
		applyAuthorRule(input, tagged, add)
	}

	// Stable order, so a run's output is reproducible and diffable.
	sort.SliceStable(suggestions, func(i, j int) bool {
		if suggestions[i].BookmarkID != suggestions[j].BookmarkID {
			return suggestions[i].BookmarkID < suggestions[j].BookmarkID
		}
		return suggestions[i].TagName < suggestions[j].TagName
	})

	return suggestions
}

// applyHashtagRule proposes a tag when a hashtag appears often enough to be a
// subject rather than a one-off.
func applyHashtagRule(
	input HeuristicInput,
	tagged map[string]bool,
	add func(domain.Bookmark, string, domain.SuggestionSource, float64),
) {
	counts := map[string]int{}
	display := map[string]string{}

	// Every tag already in the library counts as if it had already reached the
	// threshold, so a hashtag matching an established tag is always proposed.
	for _, tag := range input.Tags {
		if key := domain.CanonicalTagKey(tag.Name); key != "" {
			counts[key] = HashtagMinBookmarks
			display[key] = tag.Name
		}
	}

	for _, bookmark := range input.Bookmarks {
		for _, hashtag := range bookmark.Hashtags {
			key := domain.CanonicalTagKey(hashtag)
			if key == "" {
				continue
			}
			counts[key]++
			if _, ok := display[key]; !ok {
				display[key] = hashtag
			}
		}
	}

	for _, bookmark := range input.Bookmarks {
		if tagged[bookmark.TweetID] {
			continue
		}
		for _, hashtag := range bookmark.Hashtags {
			key := domain.CanonicalTagKey(hashtag)
			if key == "" || counts[key] < HashtagMinBookmarks {
				continue
			}
			add(bookmark, display[key], domain.SourceHashtag, HashtagConfidence)
		}
	}
}

// authorTagOccurrence is one author's leaning toward one tag.
type authorTagOccurrence struct {
	Count       int
	DisplayName string
}

// applyAuthorRule proposes the tag an author consistently uses.
//
// Both a minimum count and a minimum share are required: a share alone would fire
// after a single post, and a count alone would fire for an author who posts about
// everything.
func applyAuthorRule(
	input HeuristicInput,
	tagged map[string]bool,
	add func(domain.Bookmark, string, domain.SuggestionSource, float64),
) {
	// Which tags each bookmark carries, to measure an author's habits.
	tagsByBookmark := map[string]map[string]bool{}
	for _, relation := range input.BookmarkTags {
		names := tagsByBookmark[relation.BookmarkID]
		if names == nil {
			names = map[string]bool{}
			tagsByBookmark[relation.BookmarkID] = names
		}
		names[relation.TagID] = true
	}

	tagNameByID := make(map[string]string, len(input.Tags))
	for _, tag := range input.Tags {
		tagNameByID[tag.ID] = tag.Name
	}

	// taggedCount[author] is how many of that author's bookmarks carry any tag.
	taggedCount := map[string]int{}
	// occurrences[author][lowercased tag name] counts how many of them carry that
	// tag, and remembers the tag's own casing: the suggestion's display name is
	// what the user reads and what decides whether an existing tag matches, so it
	// must not be flattened to lower case.
	occurrences := map[string]map[string]*authorTagOccurrence{}

	for _, bookmark := range input.Bookmarks {
		names := tagsByBookmark[bookmark.TweetID]
		if len(names) == 0 {
			continue
		}
		author := strings.ToLower(bookmark.AuthorHandle)
		if author == "" {
			continue
		}
		taggedCount[author]++

		for tagID := range names {
			name := tagNameByID[tagID]
			if name == "" {
				continue
			}
			lowered := strings.ToLower(name)
			if occurrences[author] == nil {
				occurrences[author] = map[string]*authorTagOccurrence{}
			}
			entry := occurrences[author][lowered]
			if entry == nil {
				entry = &authorTagOccurrence{DisplayName: name}
				occurrences[author][lowered] = entry
			}
			entry.Count++
		}
	}

	for _, bookmark := range input.Bookmarks {
		if tagged[bookmark.TweetID] {
			continue
		}
		author := strings.ToLower(bookmark.AuthorHandle)
		total := taggedCount[author]
		if total < AuthorMinTaggedBookmarks {
			continue
		}

		for _, entry := range occurrences[author] {
			share := float64(entry.Count) / float64(total)
			if share < AuthorMinTagShare {
				continue
			}
			add(bookmark, entry.DisplayName, domain.SourceAuthor, AuthorConfidence)
		}
	}
}
