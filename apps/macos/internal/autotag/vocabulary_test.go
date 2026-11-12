package autotag

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/taoquo/x-master/apps/macos/internal/domain"
)

// Equivalence against the real vocabulary review in TypeScript.
//
// The parser is tolerant by design, so every "drop this proposal" rule is a judgement
// call. A re-implementation would make different ones without anything looking wrong,
// which is why this compares whole results rather than spot-checking.
//
// Regenerate with:
//
//	node apps/macos/testdata/vocabreview/genvocabreview.mjs
type vocabFixture struct {
	GeneratedFrom string `json:"generatedFrom"`
	Tags          []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"tags"`
	BookmarkTags []struct {
		BookmarkID string `json:"bookmarkId"`
		TagID      string `json:"tagId"`
	} `json:"bookmarkTags"`
	Vocabulary []struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	} `json:"vocabulary"`
	Cases []struct {
		Name     string `json:"name"`
		Content  string `json:"content"`
		Expected struct {
			Review VocabularyReview `json:"review"`
		} `json:"expected"`
	} `json:"cases"`
	Prompts []struct {
		Name       string `json:"name"`
		Vocabulary []struct {
			Name  string `json:"name"`
			Count int    `json:"count"`
		} `json:"vocabulary"`
		Prompt string `json:"prompt"`
	} `json:"prompts"`
}

func loadVocabFixture(t *testing.T) vocabFixture {
	t.Helper()

	path := filepath.Join("..", "..", "testdata", "vocabreview", "vocabreview.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture (regenerate with `node apps/macos/testdata/vocabreview/genvocabreview.mjs`): %v", err)
	}

	var fixture vocabFixture
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	if fixture.GeneratedFrom != "src/lib/autotag/vocabularyReview.ts" {
		t.Errorf("fixture generated from %q", fixture.GeneratedFrom)
	}
	return fixture
}

// TestVocabularyReviewPromptMatchesTypeScript pins the prompt word for word.
//
// The wording is what stops the model merging across unrelated domains or renaming
// into an existing name. Paraphrasing it would change the proposals, and nothing else
// in the system would notice.
func TestVocabularyReviewPromptMatchesTypeScript(t *testing.T) {
	fixture := loadVocabFixture(t)

	if len(fixture.Prompts) == 0 {
		t.Fatal("the fixture has no prompts")
	}

	for _, tc := range fixture.Prompts {
		t.Run(tc.Name, func(t *testing.T) {
			vocabulary := make([]VocabularyEntry, 0, len(tc.Vocabulary))
			for _, entry := range tc.Vocabulary {
				vocabulary = append(vocabulary, VocabularyEntry{Name: entry.Name, Count: entry.Count})
			}

			got := BuildVocabularyReviewPrompt(vocabulary)
			if got != tc.Prompt {
				t.Errorf("the prompt differs from the source.\n--- got ---\n%s\n--- want ---\n%s", got, tc.Prompt)
			}
		})
	}

	// The real vocabulary's prompt must name the healthy range, since that is the
	// number the model is asked to move towards.
	real := BuildVocabularyReviewPrompt(entriesOf(fixture.Vocabulary))
	// The counts come from the fixture: llm is used four times, ai agent three.
	for _, want := range []string{"30-50", "llm (4)", "ai agent (3)"} {
		if !strings.Contains(real, want) {
			t.Errorf("the prompt must contain %q", want)
		}
	}
}

// TestParseVocabularyReviewMatchesTypeScript is the parser equivalence check.
func TestParseVocabularyReviewMatchesTypeScript(t *testing.T) {
	fixture := loadVocabFixture(t)

	names := make([]string, 0, len(fixture.Tags))
	for _, tag := range fixture.Tags {
		names = append(names, tag.Name)
	}

	if len(fixture.Cases) == 0 {
		t.Fatal("the fixture has no cases")
	}

	for _, tc := range fixture.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			got := ParseVocabularyReview(tc.Content, names)
			want := tc.Expected.Review

			if len(got.Merges) != len(want.Merges) {
				t.Fatalf("got %d merges, want %d\ngot:  %+v\nwant: %+v", len(got.Merges), len(want.Merges), got.Merges, want.Merges)
			}
			for index, expected := range want.Merges {
				merge := got.Merges[index]
				if merge.Into != expected.Into {
					t.Errorf("merge %d into = %q, want %q", index, merge.Into, expected.Into)
				}
				if merge.Reason != expected.Reason {
					t.Errorf("merge %d reason = %q, want %q", index, merge.Reason, expected.Reason)
				}
				if len(merge.From) != len(expected.From) {
					t.Errorf("merge %d from = %v, want %v", index, merge.From, expected.From)
					continue
				}
				for sourceIndex := range expected.From {
					if merge.From[sourceIndex] != expected.From[sourceIndex] {
						t.Errorf("merge %d from = %v, want %v", index, merge.From, expected.From)
						break
					}
				}
			}

			if len(got.Renames) != len(want.Renames) {
				t.Fatalf("got %d renames, want %d\ngot:  %+v\nwant: %+v", len(got.Renames), len(want.Renames), got.Renames, want.Renames)
			}
			for index, expected := range want.Renames {
				rename := got.Renames[index]
				if rename.From != expected.From || rename.To != expected.To || rename.Reason != expected.Reason {
					t.Errorf("rename %d = %+v, want %+v", index, rename, expected)
				}
			}
		})
	}
}

// TestResolveReviewMatchesTypeScript resolves the parsed proposals to plans against
// real tag ids, which is what the UI would apply.
func TestResolveReviewMatchesTypeScript(t *testing.T) {
	fixture := loadVocabFixture(t)

	tags := make([]domain.Tag, 0, len(fixture.Tags))
	for _, tag := range fixture.Tags {
		tags = append(tags, domain.Tag{ID: tag.ID, Name: tag.Name})
	}
	names := make([]string, 0, len(fixture.Tags))
	for _, tag := range fixture.Tags {
		names = append(names, tag.Name)
	}

	// A response that exercises both branches: a merge onto an existing tag and a
	// rename.
	content := `{
		"merges":[{"from":["agentic ai","llm agents"],"into":"ai agent","reason":"same concept"}],
		"renames":[{"from":"LLMs","to":"llm large language model","reason":"naming"}]
	}`

	review := ParseVocabularyReview(content, names)
	merges, renames := ResolveReview(review, tags)

	if len(merges) != 1 {
		t.Fatalf("got %d merges, want 1: %+v", len(merges), merges)
	}
	// The plan names real ids, not display names.
	if merges[0].TargetTagID != "t1" {
		t.Errorf("target id = %q, want t1", merges[0].TargetTagID)
	}
	if len(merges[0].SourceTagIDs) != 2 {
		t.Errorf("source ids = %v, want two", merges[0].SourceTagIDs)
	}
	// A merge never lists its own target as a source, or it would delete what it keeps.
	for _, id := range merges[0].SourceTagIDs {
		if id == merges[0].TargetTagID {
			t.Error("a merge must not have its target among its sources")
		}
	}

	if len(renames) != 1 {
		t.Fatalf("got %d renames, want 1: %+v", len(renames), renames)
	}
	// "LLMs" and "llm" share a canonical key, and the index keeps the first name
	// stored for a key -- so the proposal resolves to the existing "llm" tag (t4)
	// rather than minting a second one. That is the documented purpose of the index.
	if renames[0].TagID != "t4" {
		t.Errorf("rename id = %q, want t4 (the existing llm)", renames[0].TagID)
	}
	if renames[0].From != "llm" {
		t.Errorf("rename from = %q, want the local display name llm", renames[0].From)
	}
}

// TestReviewSkipsASingleTag pins the short circuit: one tag cannot be consolidated
// with anything, so asking would spend a request to be told so.
func TestReviewSkipsASingleTag(t *testing.T) {
	called := false
	result, err := ReviewTagVocabulary(
		[]domain.Tag{{ID: "t1", Name: "go"}}, nil,
		func(string) (string, error) { called = true; return "", nil },
	)
	if err != nil {
		t.Fatalf("ReviewTagVocabulary: %v", err)
	}
	if called {
		t.Error("a single tag must not reach the model")
	}
	if !result.OK || len(result.Merges) != 0 || len(result.Renames) != 0 {
		t.Errorf("result = %+v, want an empty success", result)
	}
}

// TestReviewReportsARequestFailure keeps a transport failure from looking like a
// review that found nothing.
func TestReviewReportsARequestFailure(t *testing.T) {
	tags := []domain.Tag{{ID: "t1", Name: "go"}, {ID: "t2", Name: "golang"}}

	result, err := ReviewTagVocabulary(tags, nil, func(string) (string, error) {
		return "", errStub("rate limited")
	})
	if err != nil {
		t.Fatalf("a request failure is reported in the result, not as an error: %v", err)
	}
	if result.OK {
		t.Error("a failed request must not report OK")
	}
	if !strings.Contains(result.Error, "rate limited") {
		t.Errorf("the error must be the real one, got %q", result.Error)
	}
}

// TestReviewEndToEnd covers the whole path: prompt, answer, parse, resolve.
func TestReviewEndToEnd(t *testing.T) {
	tags := []domain.Tag{
		{ID: "t1", Name: "ai agent"},
		{ID: "t2", Name: "agentic ai"},
		{ID: "t3", Name: "llm"},
	}
	relations := []domain.BookmarkTag{
		{BookmarkID: "b1", TagID: "t1"}, {BookmarkID: "b2", TagID: "t1"},
		{BookmarkID: "b3", TagID: "t2"}, {BookmarkID: "b4", TagID: "t3"},
	}

	var sawPrompt string
	result, err := ReviewTagVocabulary(tags, relations, func(prompt string) (string, error) {
		sawPrompt = prompt
		// The model answers with a merge, and the names it uses differ in case from
		// the local ones -- which is exactly what the known-name index resolves.
		return `{"merges":[{"from":["Agentic AI"],"into":"AI Agent","reason":"same concept"}]}`, nil
	})
	if err != nil {
		t.Fatalf("ReviewTagVocabulary: %v", err)
	}

	// The prompt carries the usage counts, most used first.
	if !strings.Contains(sawPrompt, "ai agent (2)") {
		t.Errorf("the prompt must carry usage counts:\n%s", sawPrompt)
	}

	if !result.OK {
		t.Fatalf("result = %+v", result)
	}
	if len(result.Merges) != 1 {
		t.Fatalf("got %d merges, want 1: %+v", len(result.Merges), result.Merges)
	}
	// "Agentic AI" resolved back to the local "agentic ai".
	if len(result.Merges[0].SourceNames) != 1 || result.Merges[0].SourceNames[0] != "agentic ai" {
		t.Errorf("source names = %v, want the local display name", result.Merges[0].SourceNames)
	}
	if result.Merges[0].TargetTagID != "t1" {
		t.Errorf("target = %q, want t1", result.Merges[0].TargetTagID)
	}
}

// TestPlanKeysAreStable covers the identity the review panel uses as a selection key.
func TestPlanKeysAreStable(t *testing.T) {
	merge := TagMergePlan{TargetTagID: "t1", SourceTagIDs: []string{"t2", "t3"}}
	if MergePlanKey(merge) != MergePlanKey(merge) {
		t.Error("a plan's key must be stable across calls")
	}
	if MergePlanKey(merge) == MergePlanKey(TagMergePlan{TargetTagID: "t1", SourceTagIDs: []string{"t2"}}) {
		t.Error("different plans must have different keys")
	}

	rename := TagRenamePlan{TagID: "t5", To: "llm"}
	if RenamePlanKey(rename) != RenamePlanKey(rename) {
		t.Error("a rename key must be stable")
	}
}

// errStub is a plain error for the tests above.
type errStub string

func (e errStub) Error() string { return string(e) }

func entriesOf(entries []struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}) []VocabularyEntry {
	out := make([]VocabularyEntry, 0, len(entries))
	for _, entry := range entries {
		out = append(out, VocabularyEntry{Name: entry.Name, Count: entry.Count})
	}
	return out
}
