package autotag

import (
	"math"
	"testing"
	"time"

	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/llm"
)

var base = time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)

// --- run state: the two windows ---

// TestTwoWindowsDiffer pins why there are two: a run dead enough to replace must
// not also be able to refuse replacement for the longer UI window.
func TestTwoWindowsDiffer(t *testing.T) {
	if RunLivenessAfter >= RunStaleAfter {
		t.Fatalf("liveness (%v) must be shorter than staleness (%v)", RunLivenessAfter, RunStaleAfter)
	}

	state := Begin(base, BeginInput{Phase: PhaseHeuristics, Total: 10})
	// A heartbeat older than liveness but newer than staleness is the gap the two
	// windows exist for.
	heartbeat := base.Add(RunLivenessAfter + time.Second)

	if IsStale(state, heartbeat, RunLivenessAfter) != true {
		t.Error("past liveness the run must be replaceable")
	}
	if IsStale(state, heartbeat, RunStaleAfter) != false {
		t.Error("within staleness the UI must still show it as possibly alive")
	}
	if !IsActive(state, heartbeat, RunStaleAfter) {
		t.Error("IsActive must use the window it is given")
	}
}

func TestIsStaleEdgeCases(t *testing.T) {
	if IsStale(nil, base, RunStaleAfter) {
		t.Error("no state is not a stale state")
	}
	// A state with no usable heartbeat counts as dead.
	if !IsStale(&RunState{UpdatedAt: time.Time{}}, base, RunStaleAfter) {
		t.Error("a zero heartbeat must count as stale")
	}
	// Exactly at the boundary is not yet stale.
	state := Begin(base, BeginInput{Phase: PhaseLLM})
	if IsStale(state, base.Add(RunStaleAfter), RunStaleAfter) {
		t.Error("the boundary itself must not count as stale")
	}
	if !IsStale(state, base.Add(RunStaleAfter+time.Millisecond), RunStaleAfter) {
		t.Error("just past the boundary is stale")
	}
}

// --- run state: ownership and takeover ---

func TestTouchIsScopedToTheOwningRun(t *testing.T) {
	state := Begin(base, BeginInput{Phase: PhaseHeuristics, Total: 5})

	next, owns := Touch(state, TouchInput{
		RunID: state.RunID, Now: base.Add(time.Second), Processed: 3, Total: 5,
	})
	if !owns {
		t.Fatal("the owning run must be able to touch its own state")
	}
	if next.Processed != 3 {
		t.Errorf("Processed = %d, want 3", next.Processed)
	}
	if !next.UpdatedAt.Equal(base.Add(time.Second)) {
		t.Errorf("UpdatedAt was not refreshed: %v", next.UpdatedAt)
	}
}

// TestSupersededRunCannotClobberItsReplacement is the failure a dedicated run id
// fixes.
func TestSupersededRunCannotClobberItsReplacement(t *testing.T) {
	original := Begin(base, BeginInput{Phase: PhaseHeuristics, Total: 5})

	// A takeover installs a different run id.
	replacement := Begin(base.Add(time.Minute), BeginInput{Phase: PhaseHeuristics, Total: 9})

	next, owns := Touch(replacement, TouchInput{
		RunID: original.RunID, // the old run keeps trying to write
		Now:   base.Add(2 * time.Minute), Processed: 4,
	})
	if owns {
		t.Fatal("a superseded run must not own the state")
	}
	if next.Processed != replacement.Processed || next.RunID != replacement.RunID {
		t.Error("a superseded write must leave the replacement untouched")
	}
}

func TestTouchPreservesCancelAndDetail(t *testing.T) {
	state := Begin(base, BeginInput{Phase: PhaseLLM, Total: 4})

	waiting := base.Add(30 * time.Second)
	reason := "provider asked us to wait"
	state, _ = Touch(state, TouchInput{
		RunID: state.RunID, Now: base.Add(time.Second),
		WaitingUntil: &waiting, WaitingReason: &reason,
	})

	// The user cancels, and the pipeline then sends a plain heartbeat.
	state, _ = RequestCancel(state)
	next, owns := Touch(state, TouchInput{RunID: state.RunID, Now: base.Add(2 * time.Second), Processed: 2})
	if !owns {
		t.Fatal("the run still owns the state")
	}

	if !next.CancelRequested {
		t.Error("a heartbeat must not erase a pending cancel")
	}
	if next.WaitingUntil == nil || !next.WaitingUntil.Equal(waiting) {
		t.Error("a plain heartbeat must not erase a waiting marker")
	}
	if next.WaitingReason != reason {
		t.Errorf("WaitingReason = %q, want it preserved", next.WaitingReason)
	}
	if next.Status != StatusCancelling {
		t.Errorf("Status = %q, want the cancel state preserved", next.Status)
	}
}

func TestTouchClearsInFlightWhenItEnds(t *testing.T) {
	state := Begin(base, BeginInput{Phase: PhaseLLM, Total: 4})
	started := base.Add(time.Second)
	state, _ = Touch(state, TouchInput{RunID: state.RunID, Now: started, InFlight: boolPtr(true), BatchStartedAt: &started})

	if state.BatchStartedAt == nil {
		t.Fatal("BatchStartedAt should be set")
	}

	// A finished batch must clear the in-flight marker, or the UI reports a batch
	// that never ends.
	stopped := false
	state, _ = Touch(state, TouchInput{RunID: state.RunID, Now: base.Add(2 * time.Second), InFlight: &stopped})
	if state.InFlight {
		t.Error("InFlight must be cleared")
	}
	if state.BatchStartedAt != nil {
		t.Error("BatchStartedAt must be cleared with it")
	}
}

func TestRequestCancelOnNoState(t *testing.T) {
	if _, changed := RequestCancel(nil); changed {
		t.Error("cancelling nothing must report no change")
	}
}

func TestShouldCancel(t *testing.T) {
	state := Begin(base, BeginInput{Phase: PhaseLLM, Total: 4})

	t.Run("a live owning run is not cancelled", func(t *testing.T) {
		stop, owns := ShouldCancel(state, state.RunID, base)
		if stop || !owns {
			t.Errorf("stop=%v owns=%v, want false/true", stop, owns)
		}
	})

	t.Run("after a cancel request the pipeline stops", func(t *testing.T) {
		cancelled, _ := RequestCancel(state)
		stop, owns := ShouldCancel(cancelled, cancelled.RunID, base)
		if !stop || !owns {
			t.Errorf("stop=%v owns=%v, want true/true", stop, owns)
		}
	})

	t.Run("a superseded run is told to stop", func(t *testing.T) {
		// It no longer owns the state, and its work belongs to a run the user
		// already replaced.
		stop, owns := ShouldCancel(state, "some-older-run", base)
		if !stop || owns {
			t.Errorf("stop=%v owns=%v, want true/false", stop, owns)
		}
	})

	t.Run("no state and no run id means nothing to stop", func(t *testing.T) {
		stop, owns := ShouldCancel(nil, "", base)
		if stop || owns {
			t.Errorf("stop=%v owns=%v, want false/false", stop, owns)
		}
	})
}

func TestFinishIsOwnerOnly(t *testing.T) {
	state := Begin(base, BeginInput{Phase: PhaseLLM, Total: 4})

	// A superseded run must stay silent: clearing here would hide the run that
	// replaced it.
	if cleared, reported := Finish(state, "an-older-run"); reported || cleared == nil {
		t.Error("a superseded run must not clear or report")
	}

	cleared, reported := Finish(state, state.RunID)
	if !reported || cleared != nil {
		t.Error("the owner must clear the state and report")
	}

	// With nothing live, reporting is always safe.
	if cleared, reported := Finish(nil, ""); !reported || cleared != nil {
		t.Error("finishing no state must report success")
	}
}

func TestShouldContinue(t *testing.T) {
	leftover := RunSummary{Outcome: OutcomeTimedOut, UnprocessedCount: 5}

	if !ShouldContinue(leftover, 0, true) {
		t.Error("work left with auto-continue on must continue")
	}
	if ShouldContinue(leftover, 0, false) {
		t.Error("auto-continue off must stop")
	}
	if ShouldContinue(leftover, ContinueLimit, true) {
		t.Error("the chain must stop at the cap")
	}

	// A cancelled run must not be continued: the user asked it to stop.
	cancelled := RunSummary{Outcome: OutcomeCancelled, UnprocessedCount: 5}
	if ShouldContinue(cancelled, 0, true) {
		t.Error("a cancelled run must never be continued")
	}

	// Nothing left to do.
	done := RunSummary{Outcome: OutcomeOK, UnprocessedCount: 0}
	if ShouldContinue(done, 0, true) {
		t.Error("a finished run must not continue")
	}
}

func TestOutcomeInventory(t *testing.T) {
	if len(AllRunOutcomes) != 12 {
		t.Fatalf("AllRunOutcomes has %d entries, want 12", len(AllRunOutcomes))
	}
	seen := map[RunOutcome]bool{}
	for _, outcome := range AllRunOutcomes {
		if seen[outcome] {
			t.Errorf("duplicate outcome %q", outcome)
		}
		seen[outcome] = true
	}
	for _, expected := range []RunOutcome{
		OutcomeOK, OutcomeDisabled, OutcomeLLMDisabled, OutcomeMissingAPIKey,
		OutcomeNoCandidates, OutcomeEmptyVocabulary, OutcomeAllBlocked,
		OutcomeBudgetExhausted, OutcomeTimedOut, OutcomeCancelled,
		OutcomeInterrupted, OutcomeFailed,
	} {
		if !ValidRunOutcome(expected) {
			t.Errorf("outcome %q is missing", expected)
		}
	}
	if ValidRunOutcome("invented") {
		t.Error("an unknown outcome must be rejected on read")
	}
}

func TestBudgetSentinelRoundTrip(t *testing.T) {
	// JSON has no Infinity, so -1 is the stored form of "unbounded".
	if got := EncodeBudget(math.Inf(1)); got != UnboundedBudget {
		t.Errorf("EncodeBudget(Inf) = %d, want %d", got, UnboundedBudget)
	}
	if got := DecodeBudget(UnboundedBudget); !math.IsInf(got, 1) {
		t.Errorf("DecodeBudget(%d) = %v, want +Inf", UnboundedBudget, got)
	}
	// A finite budget must survive untouched, including zero.
	for _, budget := range []int{0, 1, 7, 50} {
		if got := DecodeBudget(budget); got != float64(budget) {
			t.Errorf("DecodeBudget(%d) = %v", budget, got)
		}
	}
	if got := EncodeBudget(0); got != 0 {
		t.Errorf("EncodeBudget(0) = %d", got)
	}
}

func TestResolveNewTagBudget(t *testing.T) {
	tests := []struct {
		name       string
		mode       TagVocabularyMode
		vocabulary int
		eligible   int
		maxPerRun  int
		want       float64
	}{
		{"closed never invents", VocabularyClosed, 0, 1000, 200, 0},
		{"open is unbounded", VocabularyOpen, 40, 1000, 200, math.Inf(1)},
		{"guarded floors at one", VocabularyGuarded, 10, 10, 200, 1},
		{"guarded scales with candidates", VocabularyGuarded, 0, 300, 200, 2},       // ceil(200/150)
		{"guarded is capped by the window", VocabularyGuarded, 0, 100000, 2000, 14}, // ceil(2000/150)
		{"guarded is capped by headroom", VocabularyGuarded, 45, 2000, 2000, 5},     // 50-45
		{"guarded at the healthy maximum stops", VocabularyGuarded, HealthyTagCountMax, 2000, 2000, 0},
		{"guarded past the maximum stops", VocabularyGuarded, 60, 2000, 2000, 0},
		{"unknown mode invents nothing", "invented", 0, 1000, 200, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveNewTagBudget(tc.mode, tc.vocabulary, tc.eligible, tc.maxPerRun)
			if math.IsInf(tc.want, 1) {
				if !math.IsInf(got, 1) {
					t.Errorf("got %v, want +Inf", got)
				}
				return
			}
			if got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestAllowsNewTags(t *testing.T) {
	if AllowsNewTags(VocabularyClosed) {
		t.Error("closed must not invent tags")
	}
	if !AllowsNewTags(VocabularyGuarded) || !AllowsNewTags(VocabularyOpen) {
		t.Error("guarded and open may invent tags")
	}
}

func TestBlockedKeysAreCanonical(t *testing.T) {
	blocked := BlockedTagKeySet([]string{"AI Agents", "  ", "Machine Learning"})

	for _, name := range []string{"AI Agent", "ai-agents", "AI_Agent", "a i agent"} {
		if !IsBlocked(blocked, name) {
			t.Errorf("%q must be blocked: canonical keys collapse these spellings", name)
		}
	}
	if IsBlocked(blocked, "machine-learning") != true {
		t.Error("Machine Learning must be blocked under any spelling")
	}
	if IsBlocked(blocked, "Cooking") {
		t.Error("an unrelated tag must not be blocked")
	}
	// A name that normalizes to nothing must not suppress anything.
	if IsBlocked(blocked, "   ") {
		t.Error("a blank name must never be blocked")
	}
	if IsBlocked(nil, "anything") {
		t.Error("an empty block list blocks nothing")
	}
}

func TestNormalizeKeywordRules(t *testing.T) {
	rules := NormalizeKeywordRules([]KeywordTagRule{
		{Keyword: "  kubernetes ", TagName: " K8s "},
		{Keyword: "", TagName: "x"},     // incomplete
		{Keyword: "y", TagName: "   "},  // incomplete
		{Keyword: "  ", TagName: " z "}, // incomplete after trimming
	})

	if len(rules) != 1 {
		t.Fatalf("got %d rules, want 1: an incomplete rule could never fire", len(rules))
	}
	if rules[0].Keyword != "kubernetes" || rules[0].TagName != "K8s" {
		t.Errorf("rule = %+v, want it trimmed", rules[0])
	}
}

func boolPtr(v bool) *bool { return &v }

// --- heuristics ---

func bookmarkWith(id, handle, text string, hashtags ...string) domain.Bookmark {
	return domain.Bookmark{
		TweetID: id, AuthorHandle: handle, AuthorName: handle,
		Text: text, Hashtags: hashtags, SavedAt: base, LastSeenAt: base, UpdatedAt: base,
	}
}

func TestHeuristicKeywordRule(t *testing.T) {
	input := HeuristicInput{
		Bookmarks: []domain.Bookmark{
			bookmarkWith("1", "ada", "Learning about Kubernetes today"),
			bookmarkWith("2", "bob", "nothing relevant"),
		},
		Settings: HeuristicSettings{KeywordRules: []KeywordTagRule{{Keyword: "kubernetes", TagName: "K8s"}}},
		Now:      base,
	}

	suggestions := SuggestHeuristics(input)
	if len(suggestions) != 1 {
		t.Fatalf("got %d suggestions, want 1: %+v", len(suggestions), suggestions)
	}
	if suggestions[0].BookmarkID != "1" || suggestions[0].Source != domain.SourceKeyword {
		t.Errorf("suggestion = %+v", suggestions[0])
	}
	if suggestions[0].Confidence != KeywordConfidence {
		t.Errorf("confidence = %v, want %v", suggestions[0].Confidence, KeywordConfidence)
	}
}

// TestHeuristicKeywordIsASubstringMatch pins the semantics: no word boundaries,
// which keeps matching the language the user configured.
func TestHeuristicKeywordIsASubstringMatch(t *testing.T) {
	input := HeuristicInput{
		Bookmarks: []domain.Bookmark{
			bookmarkWith("1", "ada", "prefixkubernetessuffix"),
			bookmarkWith("2", "bob", "KUBERNETES in caps"),
		},
		Settings: HeuristicSettings{KeywordRules: []KeywordTagRule{{Keyword: "kubernetes", TagName: "K8s"}}},
		Now:      base,
	}
	if got := len(SuggestHeuristics(input)); got != 2 {
		t.Errorf("got %d suggestions, want 2: matching is case-insensitive and not word-bounded", got)
	}
}

func TestHeuristicSkipsAlreadyTaggedBookmarks(t *testing.T) {
	input := HeuristicInput{
		Bookmarks: []domain.Bookmark{
			bookmarkWith("1", "ada", "kubernetes"),
			bookmarkWith("2", "bob", "kubernetes"),
		},
		BookmarkTags: []domain.BookmarkTag{{BookmarkID: "1", TagID: "t1", Source: domain.TagSourceManual}},
		Settings:     HeuristicSettings{KeywordRules: []KeywordTagRule{{Keyword: "kubernetes", TagName: "K8s"}}},
		Now:          base,
	}

	suggestions := SuggestHeuristics(input)
	if len(suggestions) != 1 || suggestions[0].BookmarkID != "2" {
		t.Fatalf("got %+v, want only bookmark 2: heuristics are for the backlog", suggestions)
	}
}

func TestHeuristicHashtagThreshold(t *testing.T) {
	t.Run("below the threshold is not proposed", func(t *testing.T) {
		input := HeuristicInput{
			Bookmarks: []domain.Bookmark{
				bookmarkWith("1", "a", "x", "golang"),
				bookmarkWith("2", "b", "x", "golang"),
			},
			Settings: HeuristicSettings{UseHashtags: true},
			Now:      base,
		}
		if got := SuggestHeuristics(input); len(got) != 0 {
			t.Errorf("got %+v, want nothing at %d occurrences", got, HashtagMinBookmarks-1)
		}
	})

	t.Run("at the threshold it is proposed", func(t *testing.T) {
		input := HeuristicInput{
			Bookmarks: []domain.Bookmark{
				bookmarkWith("1", "a", "x", "golang"),
				bookmarkWith("2", "b", "x", "golang"),
				bookmarkWith("3", "c", "x", "golang"),
			},
			Settings: HeuristicSettings{UseHashtags: true},
			Now:      base,
		}
		suggestions := SuggestHeuristics(input)
		if len(suggestions) != 3 {
			t.Fatalf("got %d suggestions, want 3 (one per post)", len(suggestions))
		}
		if suggestions[0].Source != domain.SourceHashtag || suggestions[0].Confidence != HashtagConfidence {
			t.Errorf("suggestion = %+v", suggestions[0])
		}
	})

	t.Run("a hashtag matching an existing tag always passes", func(t *testing.T) {
		// An established tag pre-seeds the counter, so the user's own vocabulary
		// is never blocked by the threshold.
		input := HeuristicInput{
			Bookmarks: []domain.Bookmark{bookmarkWith("1", "a", "x", "golang")},
			Tags:      []domain.Tag{{ID: "t1", Name: "Golang"}},
			Settings:  HeuristicSettings{UseHashtags: true},
			Now:       base,
		}
		if got := SuggestHeuristics(input); len(got) != 1 {
			t.Errorf("got %+v, want the established tag proposed", got)
		}
	})

	t.Run("disabled means no hashtag suggestions", func(t *testing.T) {
		input := HeuristicInput{
			Bookmarks: []domain.Bookmark{
				bookmarkWith("1", "a", "x", "golang"), bookmarkWith("2", "b", "x", "golang"),
				bookmarkWith("3", "c", "x", "golang"),
			},
			Settings: HeuristicSettings{UseHashtags: false},
			Now:      base,
		}
		if got := SuggestHeuristics(input); len(got) != 0 {
			t.Errorf("got %+v, want nothing when hashtags are off", got)
		}
	})
}

// TestHeuristicAuthorGateUsesTheAuthorsTaggedCount pins which count the gate
// applies. The threshold is on the *author's* tagged bookmarks, not on how often one
// tag appears, so two of three posts tagged Rust does qualify.
func TestHeuristicAuthorGateUsesTheAuthorsTaggedCount(t *testing.T) {
	tags := []domain.Tag{{ID: "rust", Name: "Rust"}, {ID: "go", Name: "Go"}}
	untagged := bookmarkWith("untagged", "ada", "no tag yet")

	t.Run("fewer than three tagged posts never fires", func(t *testing.T) {
		input := HeuristicInput{
			Bookmarks: []domain.Bookmark{
				bookmarkWith("1", "ada", "x"), bookmarkWith("2", "ada", "x"), untagged,
			},
			Tags: tags,
			BookmarkTags: []domain.BookmarkTag{
				{BookmarkID: "1", TagID: "rust"}, {BookmarkID: "2", TagID: "rust"},
			},
			Settings: HeuristicSettings{UseAuthors: true},
			Now:      base,
		}
		for _, suggestion := range SuggestHeuristics(input) {
			if suggestion.Source == domain.SourceAuthor {
				t.Errorf("only two tagged posts, so the count gate must block: %+v", suggestion)
			}
		}
	})

	t.Run("three tagged posts with a high share fires", func(t *testing.T) {
		input := HeuristicInput{
			Bookmarks: []domain.Bookmark{
				bookmarkWith("1", "ada", "x"), bookmarkWith("2", "ada", "x"),
				bookmarkWith("3", "ada", "x"), untagged,
			},
			Tags: tags,
			BookmarkTags: []domain.BookmarkTag{
				{BookmarkID: "1", TagID: "rust"}, {BookmarkID: "2", TagID: "rust"},
				{BookmarkID: "3", TagID: "go"},
			},
			Settings: HeuristicSettings{UseAuthors: true},
			Now:      base,
		}

		suggestions := SuggestHeuristics(input)
		var rust *domain.TagSuggestion
		for i := range suggestions {
			if suggestions[i].Source == domain.SourceAuthor && suggestions[i].TagName == "Rust" {
				rust = &suggestions[i]
			}
		}
		if rust == nil {
			t.Fatal("2 of 3 tagged posts meeting the share threshold must fire")
		}
		if rust.BookmarkID != "untagged" {
			t.Errorf("the suggestion must target an untagged post, got %q", rust.BookmarkID)
		}
		if rust.Confidence != AuthorConfidence {
			t.Errorf("confidence = %v, want %v", rust.Confidence, AuthorConfidence)
		}
		// The tag's own casing must survive into the suggestion.
		if rust.TagName != "Rust" {
			t.Errorf("TagName = %q, want the tag's casing preserved", rust.TagName)
		}
	})
}

func TestHeuristicAuthorShareThreshold(t *testing.T) {
	// 3 tagged posts but only 1 Rust: share 1/3 < 0.6.
	relations := []domain.BookmarkTag{
		{BookmarkID: "1", TagID: "rust"}, {BookmarkID: "2", TagID: "go"}, {BookmarkID: "3", TagID: "sql"},
	}
	input := HeuristicInput{
		Bookmarks: []domain.Bookmark{
			bookmarkWith("1", "ada", "x"), bookmarkWith("2", "ada", "x"),
			bookmarkWith("3", "ada", "x"), bookmarkWith("4", "ada", "x"),
		},
		Tags:         []domain.Tag{{ID: "rust", Name: "Rust"}, {ID: "go", Name: "Go"}, {ID: "sql", Name: "SQL"}},
		BookmarkTags: relations,
		Settings:     HeuristicSettings{UseAuthors: true},
		Now:          base,
	}

	for _, suggestion := range SuggestHeuristics(input) {
		if suggestion.Source == domain.SourceAuthor {
			t.Errorf("the share threshold must block this: %+v", suggestion)
		}
	}
}

func TestHeuristicRespectsTheNegativeCache(t *testing.T) {
	bookmark := bookmarkWith("1", "ada", "kubernetes")
	id := domain.SuggestionID("1", "K8s")

	input := HeuristicInput{
		Bookmarks:             []domain.Bookmark{bookmark},
		Settings:              HeuristicSettings{KeywordRules: []KeywordTagRule{{Keyword: "kubernetes", TagName: "K8s"}}},
		ExistingSuggestionIDs: map[string]bool{id: true},
		Now:                   base,
	}

	if got := SuggestHeuristics(input); len(got) != 0 {
		t.Errorf("got %+v, want nothing: the user already answered this", got)
	}
}

func TestHeuristicRespectsTheBlockList(t *testing.T) {
	input := HeuristicInput{
		Bookmarks:      []domain.Bookmark{bookmarkWith("1", "ada", "kubernetes")},
		Settings:       HeuristicSettings{KeywordRules: []KeywordTagRule{{Keyword: "kubernetes", TagName: "K8s"}}},
		BlockedTagKeys: BlockedTagKeySet([]string{"k8s"}),
		Now:            base,
	}
	if got := SuggestHeuristics(input); len(got) != 0 {
		t.Errorf("got %+v, want nothing: the user deleted this tag on purpose", got)
	}
}

func TestHeuristicDeduplicatesWithinARun(t *testing.T) {
	// Two rules propose the same tag for the same post; they are one suggestion.
	input := HeuristicInput{
		Bookmarks: []domain.Bookmark{bookmarkWith("1", "ada", "kubernetes and k8s")},
		Settings: HeuristicSettings{KeywordRules: []KeywordTagRule{
			{Keyword: "kubernetes", TagName: "K8s"},
			{Keyword: "k8s", TagName: "K8s"},
		}},
		Now: base,
	}
	if got := SuggestHeuristics(input); len(got) != 1 {
		t.Errorf("got %d suggestions, want 1: %+v", len(got), got)
	}
}

// --- suggestion lifecycle ---

func pendingSuggestion(id, bookmarkID, name string) domain.TagSuggestion {
	return domain.TagSuggestion{
		ID: domain.SuggestionID(bookmarkID, name), BookmarkID: bookmarkID, TagName: name,
		Source: domain.SourceLLM, Confidence: 0.5, Status: domain.StatusPending, CreatedAt: base,
	}
}

func TestPlanAcceptReusesAnExistingTag(t *testing.T) {
	tags := []domain.Tag{{ID: "t1", Name: "AI Agents"}}
	plan, err := PlanAccept(pendingSuggestion("s1", "b1", "ai-agent"), tags, base)
	if err != nil {
		t.Fatalf("PlanAccept: %v", err)
	}

	if plan.ExistingTag == nil || plan.ExistingTag.ID != "t1" {
		t.Fatalf("a canonical match must reuse the existing tag, got %+v", plan.ExistingTag)
	}
	if plan.NewTagName != "" {
		t.Error("no new tag should be named when one exists")
	}
	if plan.Relation.TagID != "t1" || plan.Relation.BookmarkID != "b1" {
		t.Errorf("relation = %+v", plan.Relation)
	}
	if plan.Relation.Source != domain.TagSourceSuggestion {
		t.Errorf("source = %q, want the suggestion source recorded", plan.Relation.Source)
	}
	// Resolving is the caller's last step, not this function's.
	if !plan.Suggestion.IsPending() {
		t.Error("planning must not resolve the suggestion")
	}
}

func TestPlanAcceptCreatesWhenMissing(t *testing.T) {
	plan, err := PlanAccept(pendingSuggestion("s1", "b1", "  Rust  "), nil, base)
	if err != nil {
		t.Fatalf("PlanAccept: %v", err)
	}
	if plan.ExistingTag != nil {
		t.Error("nothing exists to reuse")
	}
	if plan.NewTagName != "Rust" {
		t.Errorf("NewTagName = %q, want the trimmed display name", plan.NewTagName)
	}
}

func TestPlanAcceptRejectsANonPendingSuggestion(t *testing.T) {
	resolved := pendingSuggestion("s1", "b1", "Rust")
	resolved.Status = domain.StatusAccepted

	if _, err := PlanAccept(resolved, nil, base); err != ErrNotPending {
		t.Errorf("err = %v, want ErrNotPending: accepting twice must do nothing", err)
	}
}

func TestPlanRejectKeepsTheRow(t *testing.T) {
	resolved, err := PlanReject(pendingSuggestion("s1", "b1", "Rust"), base)
	if err != nil {
		t.Fatalf("PlanReject: %v", err)
	}
	if resolved.Status != domain.StatusRejected {
		t.Errorf("status = %q", resolved.Status)
	}
	if resolved.ResolvedAt == nil {
		t.Error("ResolvedAt must be stamped")
	}
	// The row is the negative cache, so it must still be resolvable later.
	if !resolved.IsResolved() {
		t.Error("a rejected suggestion must count as resolved")
	}
}

func TestResolveRejectsAnInvalidStatus(t *testing.T) {
	if _, err := Resolve(pendingSuggestion("s1", "b1", "Rust"), "maybe", base); err == nil {
		t.Error("only accepted and rejected are valid answers")
	}
}

func TestRejectPendingByTagNames(t *testing.T) {
	suggestions := []domain.TagSuggestion{
		pendingSuggestion("s1", "b1", "Rust"),
		pendingSuggestion("s2", "b2", "rust"), // same canonical key
		pendingSuggestion("s3", "b3", "Go"),
	}
	resolved := pendingSuggestion("s4", "b4", "Rust")
	resolved.Status = domain.StatusAccepted
	suggestions = append(suggestions, resolved)

	changed := RejectPendingByTagNames(suggestions, []string{"Rust"}, base)
	if len(changed) != 2 {
		t.Fatalf("rejected %d suggestions, want 2 pending Rust ones", len(changed))
	}
	for _, suggestion := range changed {
		if suggestion.Status != domain.StatusRejected {
			t.Errorf("status = %q", suggestion.Status)
		}
	}

	// An already-answered row must not be rewritten.
	if got := RejectPendingByTagNames(suggestions, []string{"Rust"}, base); len(got) != 2 {
		t.Errorf("an accepted row must not be flipped, got %d changes", len(got))
	}

	if got := RejectPendingByTagNames(suggestions, nil, base); got != nil {
		t.Error("no names means no work")
	}
}

func TestDropPendingForBookmarkKeepsResolvedRows(t *testing.T) {
	accepted := pendingSuggestion("s1", "b1", "Rust")
	accepted.Status = domain.StatusAccepted
	rejected := pendingSuggestion("s2", "b1", "Go")
	rejected.Status = domain.StatusRejected
	pending := pendingSuggestion("s3", "b1", "SQL")
	other := pendingSuggestion("s4", "b2", "Rust")

	doomed := DropPendingForBookmark([]domain.TagSuggestion{accepted, rejected, pending, other}, "b1")

	if len(doomed) != 1 || doomed[0] != pending.ID {
		t.Fatalf("doomed = %v, want only the pending row for b1", doomed)
	}
}

func TestPendingCount(t *testing.T) {
	accepted := pendingSuggestion("s1", "b1", "Rust")
	accepted.Status = domain.StatusAccepted

	if got := PendingCount([]domain.TagSuggestion{pendingSuggestion("s2", "b2", "Go"), accepted}); got != 1 {
		t.Errorf("PendingCount = %d, want 1", got)
	}
	if got := PendingCount(nil); got != 0 {
		t.Errorf("PendingCount(nil) = %d", got)
	}
}

func TestDedupeByID(t *testing.T) {
	a := pendingSuggestion("s1", "b1", "Rust")
	b := pendingSuggestion("s1", "b1", "Rust")
	c := pendingSuggestion("s2", "b1", "Go")

	got := DedupeByID([]domain.TagSuggestion{a, b, c})
	if len(got) != 2 {
		t.Errorf("got %d, want 2: the composite id makes the first two one suggestion", len(got))
	}
}

// TestShouldContinueIsInvokedWithTheChainsCount covers the decision that had no caller.
//
// ContinueLimit and ShouldContinue were both written and nothing called them, so the
// panel's "continue automatically when work is left" switch changed a field no logic read:
// a long run stopped with work behind it and the switch did nothing.
func TestShouldContinueIsInvokedWithTheChainsCount(t *testing.T) {
	leftover := RunSummary{Outcome: OutcomeBudgetExhausted, UnprocessedCount: 12}

	// The three conditions, each on its own.
	if !ShouldContinue(leftover, 0, true) {
		t.Error("work left, auto-continue on, budget fresh: should continue")
	}
	if ShouldContinue(leftover, 0, false) {
		t.Error("the switch is off")
	}
	if ShouldContinue(RunSummary{Outcome: OutcomeCancelled, UnprocessedCount: 12}, 0, true) {
		t.Error("the user asked it to stop")
	}
	if ShouldContinue(RunSummary{Outcome: OutcomeOK, UnprocessedCount: 0}, 0, true) {
		t.Error("nothing was left behind")
	}

	// The bound, which is the part that stops a run that keeps dying from restarting
	// forever: at the limit no further run starts, one below it does.
	if ShouldContinue(leftover, ContinueLimit, true) {
		t.Errorf("a chain of %d continues started another", ContinueLimit)
	}
	if !ShouldContinue(leftover, ContinueLimit-1, true) {
		t.Errorf("a chain of %d continues should have started another", ContinueLimit-1)
	}
	if ContinueLimit != 5 {
		t.Errorf("ContinueLimit = %d, want 5", ContinueLimit)
	}
}

// TestVocabularyIsOrderedByUse covers the order the prompt presents.
//
// The prompt says "most used first", and the sort is by count descending then by name. A
// tag with no bookmarks is still offered, at zero: its count reads as zero, so a tag the
// user made by hand and has not used is one the model may still reuse.
func TestVocabularyIsOrderedByUse(t *testing.T) {
	tags := []domain.Tag{
		{ID: "t1", Name: "Go"}, {ID: "t2", Name: "Rust"},
		{ID: "t3", Name: "Ada"}, {ID: "t4", Name: "Unused"},
		{ID: "t5", Name: "Blocked"},
	}
	relations := []domain.BookmarkTag{
		{BookmarkID: "b1", TagID: "t1"}, {BookmarkID: "b2", TagID: "t1"},
		{BookmarkID: "b3", TagID: "t1"},
		{BookmarkID: "b1", TagID: "t2"},
		{BookmarkID: "b2", TagID: "t3"}, {BookmarkID: "b3", TagID: "t3"},
		{BookmarkID: "b1", TagID: "t5"},
	}
	blocked := map[string]bool{domain.CanonicalTagKey("Blocked"): true}

	got := vocabularyForPrompt(tags, relations, blocked)

	want := []VocabEntry{
		{Name: "Go", Count: 3},
		{Name: "Ada", Count: 2},
		{Name: "Rust", Count: 1},
		// Zero uses, and still offered.
		{Name: "Unused", Count: 0},
	}
	if len(got) != len(want) {
		t.Fatalf("vocabulary = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// TestLearnedCapabilitiesAreLookedUpByProviderAndModel covers the lookup key: the
// provider id and the model id.
//
// An entry stored under a different provider or model must not be applied: the same model
// id can answer differently behind two endpoints.
func TestLearnedCapabilitiesAreLookedUpByProviderAndModel(t *testing.T) {
	settings := DefaultSettings()
	settings.LLM.Model = "gpt-4o-mini"
	settings.LLM.LearnedCapabilities = map[string]llm.ModelCapabilities{
		llm.LearnedCapabilityKey("openai", "gpt-4o-mini"): {Temperature: true},
	}

	if got := learnedCapabilitiesFor(settings.LLM, "openai"); got == nil || !got.Temperature {
		t.Errorf("the matching entry was not found: %+v", got)
	}
	if got := learnedCapabilitiesFor(settings.LLM, "azure-openai"); got != nil {
		t.Errorf("an entry under another provider was applied: %+v", got)
	}

	// And a different model under the same provider is a different entry.
	settings.LLM.Model = "gpt-4o"
	if got := learnedCapabilitiesFor(settings.LLM, "openai"); got != nil {
		t.Errorf("an entry under another model was applied: %+v", got)
	}

	// With nothing stored, there is nothing to apply.
	settings.LLM.LearnedCapabilities = nil
	if got := learnedCapabilitiesFor(settings.LLM, "openai"); got != nil {
		t.Errorf("got %+v from an empty map", got)
	}
}

// TestNormalizeCapabilitiesDropsWhatTheTypeCannotHold covers the two normalizers.
func TestNormalizeCapabilitiesDropsWhatTheTypeCannotHold(t *testing.T) {
	// An unknown strategy is dropped, a known one survives.
	if got := normalizeCapabilities(&llm.ModelCapabilities{JsonStrategy: "yolo"}); got != nil {
		t.Errorf("an unknown strategy survived: %+v", got)
	}
	if got := normalizeCapabilities(&llm.ModelCapabilities{JsonStrategy: llm.JsonObject}); got == nil ||
		got.JsonStrategy != llm.JsonObject {
		t.Errorf("a known strategy was dropped: %+v", got)
	}

	// A non-positive size is dropped rather than zeroed: "did not say" and "said zero"
	// are different answers.
	zero, negative := 0, -5
	if got := normalizeCapabilities(&llm.ModelCapabilities{MaxOutputTokens: &zero}); got != nil {
		t.Errorf("a zero size survived: %+v", got)
	}
	if got := normalizeCapabilities(&llm.ModelCapabilities{ContextWindow: &negative}); got != nil {
		t.Errorf("a negative window survived: %+v", got)
	}

	// An entry carrying only Temperature=false says nothing and becomes nil.
	if got := normalizeCapabilities(&llm.ModelCapabilities{Temperature: false}); got != nil {
		t.Errorf("an empty entry survived: %+v", got)
	}

	// Learned entries: the key is trimmed, empty keys and empty entries are dropped.
	learned := normalizeLearnedCapabilities(map[string]llm.ModelCapabilities{
		"  openai:gpt-4o  ": {JsonStrategy: llm.JsonObject},
		"":                  {JsonStrategy: llm.JsonObject},
		"openai:empty":      {},
	})
	if len(learned) != 1 {
		t.Fatalf("learned = %+v, want only the one real entry", learned)
	}
	if _, ok := learned["openai:gpt-4o"]; !ok {
		t.Errorf("the key was not trimmed: %+v", learned)
	}
	if normalizeLearnedCapabilities(nil) != nil {
		t.Error("an empty input should stay nil")
	}
}
