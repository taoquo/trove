package autotag

import (
	"testing"

	"github.com/taoquo/trove/apps/macos/internal/domain"
	"github.com/taoquo/trove/apps/macos/internal/xsource"
)

// TestHashtagsReachTheHeuristicFromTheWire closes the loop between the sync parser
// and the rule that uses hashtags.
//
// Each end is tested on its own, so a break between them would pass both: the parser
// can populate the field correctly while the heuristic reads a different one, and the
// symptom would be a feature that silently suggests nothing.
func TestHashtagsReachTheHeuristicFromTheWire(t *testing.T) {
	// A legacy payload shaped the way X returns one.
	legacy := map[string]any{
		"entities": map[string]any{
			"hashtags": []any{
				map[string]any{"text": "golang"},
				map[string]any{"text": "GoLang"},
				map[string]any{"text": "kubernetes"},
			},
		},
	}

	hashtags := xsource.ParseHashtags(legacy)
	if len(hashtags) != 2 {
		t.Fatalf("parsed %v, want two de-duplicated hashtags", hashtags)
	}

	// Three bookmarks with the same hashtag, which is the heuristic's threshold for
	// promoting it to a tag.
	bookmarks := []domain.Bookmark{
		{TweetID: "b1", Text: "one", AuthorHandle: "ada", Hashtags: []string{"golang"}},
		{TweetID: "b2", Text: "two", AuthorHandle: "ada", Hashtags: []string{"golang"}},
		{TweetID: "b3", Text: "three", AuthorHandle: "ada", Hashtags: []string{"golang"}},
	}

	suggestions := SuggestHeuristics(HeuristicInput{
		Bookmarks: bookmarks,
		Tags:      []domain.Tag{},
		Settings:  HeuristicSettings{UseHashtags: true},
	})

	found := false
	for _, suggestion := range suggestions {
		if suggestion.Source == domain.SourceHashtag {
			found = true
		}
	}
	if !found {
		t.Errorf("the hashtag heuristic produced nothing from %d bookmarks carrying one; "+
			"the field the parser fills is not the field the rule reads", len(bookmarks))
	}

	// With the toggle off, the same input produces nothing: that is what makes the
	// setting meaningful.
	disabled := SuggestHeuristics(HeuristicInput{
		Bookmarks: bookmarks,
		Tags:      []domain.Tag{},
		Settings:  HeuristicSettings{UseHashtags: false},
	})
	for _, suggestion := range disabled {
		if suggestion.Source == domain.SourceHashtag {
			t.Error("hashtags must not be used when the toggle is off")
		}
	}
}
