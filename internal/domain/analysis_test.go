package domain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// The fixture is generated from the fixture hashAnalyzedContent itself:
const hashFixturePath = "../../testdata/analysis/hashes.json"

type hashFixture struct {
	SourceFile    string `json:"sourceFile"`
	SourceCommit  string `json:"sourceCommit"`
	PromptVersion int    `json:"promptVersion"`
	Cases         []struct {
		Input       string `json:"input"`
		UTF16Length int    `json:"utf16Length"`
		Hash        string `json:"hash"`
	} `json:"cases"`
}

// TestHashAnalyzedContentMatchesFixture pins the ledger's content hash: it must stay
// byte-identical to the fixture one, or every bookmark whose text is not pure ASCII
// looks edited and is re-sent to the model.
func TestHashAnalyzedContentMatchesFixture(t *testing.T) {
	raw, err := os.ReadFile(filepath.Clean(hashFixturePath))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var fixture hashFixture
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	if len(fixture.Cases) == 0 {
		t.Fatal("fixture has no cases")
	}
	if fixture.PromptVersion != LlmPromptVersion {
		t.Errorf("fixture prompt version %d, Go has %d: bump the implementation together with the fixture",
			fixture.PromptVersion, LlmPromptVersion)
	}

	t.Logf("fixture: %d cases from %s @ %s", len(fixture.Cases), fixture.SourceFile, shortCommit(fixture.SourceCommit))

	for _, c := range fixture.Cases {
		// Guard the premise of the test: if Go disagreed about how long the
		// input is, the hash comparison below would be comparing different
		// questions.
		if got := utf16Length(c.Input); got != c.UTF16Length {
			t.Errorf("utf16Length(%q) = %d, the fixture reports %d", c.Input, got, c.UTF16Length)
			continue
		}
		if got := HashAnalyzedContent(c.Input); got != c.Hash {
			t.Errorf("HashAnalyzedContent(%q) = %q, the fixture gives %q", c.Input, got, c.Hash)
		}
	}
}

// TestHashIsUnitBasedNotByteBased states the trap directly, so a future
// "simplification" to hashing []byte(s) fails loudly rather than silently
// re-billing the user for a full re-analysis.
func TestHashIsUnitBasedNotByteBased(t *testing.T) {
	// Both are one UTF-16 code unit, so both hash the same as "a" XORed with a
	// different value; the point is that the CJK case below is length 2, not 6.
	if got := utf16Length("中文"); got != 2 {
		t.Fatalf("utf16Length(\"中文\") = %d, want 2 (not 6 UTF-8 bytes)", got)
	}

	distinct := map[string]string{}
	for _, s := range []string{"a", "中文", "机器学习", "\U0001F680", "\u00e9", "e\u0301"} {
		h := HashAnalyzedContent(s)
		if prev, dup := distinct[h]; dup {
			t.Errorf("hash collision between %q and %q (both %q): the hash is not discriminating",
				prev, s, h)
		}
		distinct[h] = s
	}
}

func TestHashIsStable(t *testing.T) {
	const text = "机器学习与人工智能代理系统设计"
	first := HashAnalyzedContent(text)
	for i := 0; i < 5; i++ {
		if got := HashAnalyzedContent(text); got != first {
			t.Fatalf("hash is not stable: %q then %q", first, got)
		}
	}
	if first != "172qipt" {
		t.Logf("note: fixture expects 172qipt for the 4-unit CJK sample")
	}
	if HashAnalyzedContent("") != "ztntfp" {
		t.Errorf("empty input must hash to the FNV offset basis rendered in base36 (ztntfp), got %q",
			HashAnalyzedContent(""))
	}
}

func TestIsAnalysisReusable(t *testing.T) {
	const (
		model = "gpt-4o-mini"
		hash  = "1r9wi7g"
	)
	stored := LlmAnalysis{
		BookmarkID: "1", PromptVersion: LlmPromptVersion, Model: model,
		ContentHash: hash, Outcome: OutcomeTagged, TagCount: 2,
	}

	tests := []struct {
		name          string
		record        LlmAnalysis
		model         string
		hash          string
		promptVersion int
		want          bool
	}{
		{"everything matches", stored, model, hash, LlmPromptVersion, true},
		{"different model", stored, "gpt-4.1", hash, LlmPromptVersion, false},
		{"edited text", stored, model, "different", LlmPromptVersion, false},
		{"new prompt version", stored, model, hash, LlmPromptVersion + 1, false},
		{"no record", LlmAnalysis{}, model, hash, LlmPromptVersion, false},
		{"record with no content hash", LlmAnalysis{PromptVersion: LlmPromptVersion, Model: model}, model, hash, LlmPromptVersion, false},
		{"record with no model", LlmAnalysis{PromptVersion: LlmPromptVersion, ContentHash: hash}, model, hash, LlmPromptVersion, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.record.IsAnalysisReusable(tc.model, tc.hash, tc.promptVersion); got != tc.want {
				t.Errorf("IsAnalysisReusable() = %v, want %v", got, tc.want)
			}
		})
	}
}
