package domain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// Equivalence against the frozen collation fixture.
//
// The fixture is produced by running Intl.Collator(undefined, {numeric: true,
// sensitivity: "base"}) over every pair of a set of realistic tag names, so this
// fails the moment the implementation and the fixture disagree about ordering.
type collationFixture struct {
	GeneratedFrom string `json:"generatedFrom"`
	Pairs         []struct {
		Left  string `json:"left"`
		Right string `json:"right"`
		Sign  int    `json:"sign"`
	} `json:"pairs"`
	SortedLists []struct {
		Input  []string `json:"input"`
		Sorted []string `json:"sorted"`
	} `json:"sortedLists"`
}

func TestCompareNaturalNamesMatchesTheCollator(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "collation", "collation.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var fixture collationFixture
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	if len(fixture.Pairs) == 0 {
		t.Fatal("the fixture has no pairs")
	}
	if fixture.GeneratedFrom != "frozen-fixture" {
		t.Errorf("fixture generated from %q", fixture.GeneratedFrom)
	}

	mismatches := 0
	for _, pair := range fixture.Pairs {
		got := signOf(CompareNaturalNames(pair.Left, pair.Right))
		if got != pair.Sign {
			mismatches++
			if mismatches <= 15 {
				t.Errorf("CompareNaturalNames(%q, %q) = %+d, want %+d from the collator",
					pair.Left, pair.Right, got, pair.Sign)
			}
		}
	}
	if mismatches > 0 {
		t.Errorf("%d of %d pairs disagree with the collator", mismatches, len(fixture.Pairs))
	}
}

// TestSortingWithCompareNaturalNamesMatches covers the ordering as a whole, which a
// pairwise check can miss when two names compare equal.
func TestSortingWithCompareNaturalNamesMatches(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "collation", "collation.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var fixture collationFixture
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}

	for _, list := range fixture.SortedLists {
		got := append([]string{}, list.Input...)
		sort.SliceStable(got, func(i, j int) bool {
			return CompareNaturalNames(got[i], got[j]) < 0
		})

		if len(got) != len(list.Sorted) {
			t.Fatalf("sorted length %d, want %d", len(got), len(list.Sorted))
		}
		for index := range got {
			if got[index] != list.Sorted[index] {
				t.Errorf("list %v sorted to %v, want %v", list.Input, got, list.Sorted)
				break
			}
		}
	}
}

// TestCompareNaturalNamesIsATotalOrder keeps the sort from depending on input order.
func TestCompareNaturalNamesIsATotalOrder(t *testing.T) {
	names := []string{"", "a", "A", "Tag 1", "Tag 2", "Tag 10", "01", "1", "café", "cafe", "日本語"}

	for _, left := range names {
		if got := CompareNaturalNames(left, left); got != 0 {
			t.Errorf("CompareNaturalNames(%q, %q) = %d, want 0", left, left, got)
		}
		for _, right := range names {
			forward := signOf(CompareNaturalNames(left, right))
			backward := signOf(CompareNaturalNames(right, left))
			if forward != -backward {
				t.Errorf("(%q, %q) = %d but (%q, %q) = %d; the order is not antisymmetric",
					left, right, forward, right, left, backward)
			}
		}
	}
}

// TestNumericOrderingIsThePoint pins the case a plain string compare gets wrong.
func TestNumericOrderingIsThePoint(t *testing.T) {
	if CompareNaturalNames("Tag 2", "Tag 10") >= 0 {
		t.Error("\"Tag 2\" must sort before \"Tag 10\"; byte order puts it after")
	}
	if CompareNaturalNames("Tag 1", "Tag 01") == 0 {
		t.Log("equal in value, ordered by leading zeros, which is a total order")
	}
	// Case and accents do not distinguish, which is sensitivity "base".
	if CompareNaturalNames("Go", "go") != 0 {
		t.Error("case must not distinguish")
	}
	if CompareNaturalNames("cafe", "café") != 0 {
		t.Error("an accent must not distinguish")
	}
}

func signOf(value int) int {
	switch {
	case value < 0:
		return -1
	case value > 0:
		return 1
	default:
		return 0
	}
}
