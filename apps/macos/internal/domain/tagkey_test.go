package domain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// The fixture is generated from the TypeScript implementation itself:
//
//	node apps/macos/testdata/tagkey/gentagkeys.mjs
//
// so this test compares the Go port against the real code rather than against a
// re-reading of it. See docs/domain-migration-map.md §4.
const fixturePath = "../../testdata/tagkey/tagkeys.json"

type tagKeyFixture struct {
	SourceFile   string `json:"sourceFile"`
	SourceCommit string `json:"sourceCommit"`
	CaseCount    int    `json:"caseCount"`
	Cases        []struct {
		Input   string `json:"input"`
		Display string `json:"display"`
		Key     string `json:"key"`
	} `json:"cases"`
	SameTagName []struct {
		Left  string `json:"left"`
		Right string `json:"right"`
		Same  bool   `json:"same"`
	} `json:"sameTagName"`
}

func loadFixture(t *testing.T) tagKeyFixture {
	t.Helper()

	raw, err := os.ReadFile(filepath.Clean(fixturePath))
	if err != nil {
		t.Fatalf("read fixture: %v (regenerate with `node apps/macos/testdata/tagkey/gentagkeys.mjs`)", err)
	}

	var fixture tagKeyFixture
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	if len(fixture.Cases) == 0 {
		t.Fatal("fixture has no cases")
	}
	return fixture
}

// TestCanonicalTagKeyMatchesTypeScript is the equivalence test the migration
// spec requires verbatim: for the same inputs, the Go and TypeScript
// implementations must produce identical output.
func TestCanonicalTagKeyMatchesTypeScript(t *testing.T) {
	fixture := loadFixture(t)

	t.Logf("fixture: %d cases from %s @ %s", len(fixture.Cases), fixture.SourceFile, shortCommit(fixture.SourceCommit))

	var displayMismatch, keyMismatch int
	for _, c := range fixture.Cases {
		if got := NormalizeTagDisplayName(c.Input); got != c.Display {
			displayMismatch++
			if displayMismatch <= 10 {
				t.Errorf("NormalizeTagDisplayName(%q) = %q, TypeScript gives %q", c.Input, got, c.Display)
			}
		}
		if got := CanonicalTagKey(c.Input); got != c.Key {
			keyMismatch++
			if keyMismatch <= 10 {
				t.Errorf("CanonicalTagKey(%q) = %q, TypeScript gives %q", c.Input, got, c.Key)
			}
		}
	}
	if displayMismatch > 10 {
		t.Errorf("... and %d more display-name mismatches", displayMismatch-10)
	}
	if keyMismatch > 10 {
		t.Errorf("... and %d more key mismatches", keyMismatch-10)
	}
}

func TestIsSameTagNameMatchesTypeScript(t *testing.T) {
	fixture := loadFixture(t)

	for _, p := range fixture.SameTagName {
		if got := IsSameTagName(p.Left, p.Right); got != p.Same {
			t.Errorf("IsSameTagName(%q, %q) = %v, TypeScript gives %v", p.Left, p.Right, got, p.Same)
		}
	}
}

func shortCommit(c string) string {
	if len(c) > 12 {
		return c[:12]
	}
	return c
}

// TestJSAgreementTraps pins the specific places where the obvious Go standard
// library calls would silently diverge from JavaScript. Each of these failed
// against the real TypeScript before being handled explicitly.
func TestJSAgreementTraps(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
		why   string
	}{
		{
			name:  "NEL is not whitespace to JS",
			input: "\u0085AI",
			want:  "\u0085ai",
			why:   "strings.TrimSpace would strip U+0085 because Go's unicode.IsSpace says it is a space",
		},
		{
			name:  "ZWNBSP is whitespace to JS",
			input: "\ufeffAI",
			want:  "ai",
			why:   "strings.TrimSpace would keep U+FEFF because Go does not classify it as a space",
		},
		{
			name:  "capital I with dot above keeps its combining dot",
			input: "\u0130",
			want:  "i\u0307",
			why:   "strings.ToLower would return \"i\", losing the U+0307 that JS keeps",
		},
		{
			name:  "NBSP collapses",
			input: "A\u00a0B",
			want:  "ab",
			why:   "Go's regexp \\s class is ASCII-only and would keep U+00A0",
		},
		{
			name:  "ideographic space collapses",
			input: "\u3000AI\u3000",
			want:  "ai",
			why:   "Go's regexp \\s class is ASCII-only",
		},
		{
			name:  "line separator collapses",
			input: "A\u2028B",
			want:  "ab",
			why:   "Go's regexp \\s class is ASCII-only",
		},
		{
			name:  "NFKC expands the ff ligature",
			input: "\ufb00le",
			want:  "ffle",
			why:   "NFKC must be applied before anything else",
		},
		{
			name:  "NFKC expands a circled digit",
			input: "\u2460",
			want:  "1",
			why:   "NFKC compatibility mapping",
		},
		{
			name:  "NFKC expands a squared abbreviation",
			input: "\u3231",
			want:  "(株)",
			why:   "NFKC compatibility mapping, non-ASCII survives to the key",
		},
		{
			name:  "decomposed e-acute normalizes to composed",
			input: "e\u0301",
			want:  "\u00e9",
			why:   "NFKC composition",
		},
		{
			name:  "final sigma is left alone",
			input: "\u03c2",
			want:  "\u03c2",
			why:   "simple lowercase of a final sigma is itself",
		},
		{
			name:  "capital sigma lowercases",
			input: "\u03a3",
			want:  "\u03c3",
			why:   "ordinary Unicode lowercase",
		},
		{
			name:  "Final_Sigma: trailing capital sigma takes the final form",
			input: "\u03a3\u03a3",
			want:  "\u03c3\u03c2",
			why:   "strings.ToLower is a simple per-rune map and would give an ordinary sigma twice",
		},
		{
			name:  "Final_Sigma after a cased letter",
			input: "A\u03a3",
			want:  "a\u03c2",
			why:   "same conditional casing, triggered by the preceding cased letter",
		},
		{
			name:  "Final_Sigma across a combining mark",
			input: "\u03a3\u0301\u03a3",
			want:  "\u03c3\u0301\u03c2",
			why:   "case-ignorable characters do not break the Final_Sigma context",
		},
		{
			name:  "capital sigma before a cased letter stays ordinary",
			input: "\u03a3A",
			want:  "\u03c3a",
			why:   "a following cased letter suppresses the final form",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := CanonicalTagKey(tc.input); got != tc.want {
				t.Errorf("CanonicalTagKey(%q) = %q, want %q (%s)", tc.input, got, tc.want, tc.why)
			}
		})
	}
}

// TestDepluralizeNeverStripsStopListWords states the invariant the stop list
// exists for, independently of the fixture.
func TestDepluralizeNeverStripsStopListWords(t *testing.T) {
	for word := range neverDepluralize {
		if got := depluralize(word); got != word {
			t.Errorf("depluralize(%q) = %q; a stop-listed word must survive unchanged", word, got)
		}
	}
	if got := len(neverDepluralize); got != 24 {
		t.Errorf("stop list has %d entries, want the 24 the TypeScript defines", got)
	}
}

func TestFindTagByName(t *testing.T) {
	tags := []Tag{
		{ID: "1", Name: "AI Agents"},
		{ID: "2", Name: "Design Systems", Aliases: []string{"design-system", "ui systems"}},
		{ID: "3", Name: "News"},
	}

	t.Run("matches on display name", func(t *testing.T) {
		got, ok := FindTagByName(tags, "ai-agent")
		if !ok || got.ID != "1" {
			t.Fatalf("got %+v ok=%v, want tag 1", got, ok)
		}
	})

	t.Run("falls back to aliases", func(t *testing.T) {
		got, ok := FindTagByName(tags, "design system")
		if !ok || got.ID != "2" {
			t.Fatalf("got %+v ok=%v, want tag 2", got, ok)
		}
	})

	t.Run("display name wins over an alias elsewhere", func(t *testing.T) {
		withCollision := append([]Tag{{ID: "4", Name: "UI Systems"}}, tags...)
		got, ok := FindTagByName(withCollision, "ui systems")
		if !ok || got.ID != "4" {
			t.Fatalf("got %+v ok=%v, want the tag whose own name matches (4)", got, ok)
		}
	})

	t.Run("an empty key never matches", func(t *testing.T) {
		if _, ok := FindTagByName(tags, "   "); ok {
			t.Error("blank input must not match any tag")
		}
	})

	t.Run("a stop-listed word does not match its singular", func(t *testing.T) {
		if _, ok := FindTagByName(tags, "new"); ok {
			t.Error(`"new" must not resolve to "News"`)
		}
		if _, ok := FindTagByName(tags, "news"); !ok {
			t.Error(`"news" must resolve to "News"`)
		}
	})
}

func TestIsSameTagNameEmptyKeyNeverMatches(t *testing.T) {
	if IsSameTagName("", "") {
		t.Error("two names that normalize to nothing are not the same tag")
	}
	if IsSameTagName("#", "##") {
		t.Error(`"#" and "##" both normalize to "#", but an empty key must not match`)
	}
}

func TestValidateTagHierarchy(t *testing.T) {
	tags := []Tag{
		{ID: "root"},
		{ID: "child", ParentID: "root"},
		{ID: "other"},
	}

	if err := ValidateTagHierarchy(tags, "other", "root"); err != nil {
		t.Errorf("a free tag may join a root: %v", err)
	}
	if err := ValidateTagHierarchy(tags, "other", ""); err != nil {
		t.Errorf("clearing a parent is always legal: %v", err)
	}
	if err := ValidateTagHierarchy(tags, "other", "other"); err == nil {
		t.Error("a tag must not be its own parent")
	}
	if err := ValidateTagHierarchy(tags, "other", "missing"); err == nil {
		t.Error("a missing parent must be rejected")
	}
	if err := ValidateTagHierarchy(tags, "other", "child"); err == nil {
		t.Error("a tag must not take a child as its parent (would make three levels)")
	}
	if err := ValidateTagHierarchy(tags, "root", "other"); err == nil {
		t.Error("a tag with children must not become a child")
	}
}
