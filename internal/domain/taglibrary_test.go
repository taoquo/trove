package domain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Equivalence against the frozen tag library fixture.
type libFixture struct {
	GeneratedFrom        string `json:"generatedFrom"`
	LongTailMaxBookmarks int    `json:"longTailMaxBookmarks"`
	Tags                 []struct {
		ID        string   `json:"id"`
		Name      string   `json:"name"`
		Aliases   []string `json:"aliases"`
		ParentID  string   `json:"parentId"`
		CreatedAt string   `json:"createdAt"`
		UpdatedAt string   `json:"updatedAt"`
	} `json:"tags"`
	BookmarkTags []struct {
		BookmarkID string `json:"bookmarkId"`
		TagID      string `json:"tagId"`
	} `json:"bookmarkTags"`
	Suggestions []struct {
		TagName string `json:"tagName"`
		Status  string `json:"status"`
	} `json:"suggestions"`
	Expected struct {
		Items []struct {
			TagID                  string   `json:"tagId"`
			Name                   string   `json:"name"`
			Aliases                []string `json:"aliases"`
			ParentID               string   `json:"parentId"`
			ParentName             string   `json:"parentName"`
			ChildCount             int      `json:"childCount"`
			BookmarkCount          int      `json:"bookmarkCount"`
			PendingSuggestionCount int      `json:"pendingSuggestionCount"`
			UpdatedAt              string   `json:"updatedAt"`
		} `json:"items"`
		OrphanCount         int                 `json:"orphanCount"`
		LongTailCount       int                 `json:"longTailCount"`
		RemovableOrphanIDs  []string            `json:"removableOrphanIds"`
		UnparentedCount     int                 `json:"unparentedCount"`
		RootCandidateIDs    []string            `json:"rootCandidateIds"`
		ParentCandidates    map[string][]string `json:"parentCandidates"`
		HasParentCandidates map[string]bool     `json:"hasParentCandidates"`
	} `json:"expected"`
	Views []struct {
		Filter      string   `json:"filter"`
		SortOrder   string   `json:"sortOrder"`
		SearchQuery string   `json:"searchQuery"`
		Expected    []string `json:"expected"`
	} `json:"views"`
	Sidebar []struct {
		SearchQuery string `json:"searchQuery"`
		Expanded    bool   `json:"expanded"`
		Expected    struct {
			Items            []string `json:"items"`
			ShouldShowToggle bool     `json:"shouldShowToggle"`
		} `json:"expected"`
	} `json:"sidebar"`
}

func loadLibFixture(t *testing.T) (libFixture, []Tag, []BookmarkTag, []TagSuggestion) {
	t.Helper()

	path := filepath.Join("..", "..", "testdata", "taglibrary", "taglibrary.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var fixture libFixture
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	if fixture.GeneratedFrom != "frozen-fixture" {
		t.Errorf("fixture generated from %q", fixture.GeneratedFrom)
	}

	tags := make([]Tag, 0, len(fixture.Tags))
	for _, tag := range fixture.Tags {
		tags = append(tags, Tag{
			ID: tag.ID, Name: tag.Name, Aliases: tag.Aliases, ParentID: tag.ParentID,
			CreatedAt: tag.CreatedAt, UpdatedAt: tag.UpdatedAt,
		})
	}
	relations := make([]BookmarkTag, 0, len(fixture.BookmarkTags))
	for _, relation := range fixture.BookmarkTags {
		relations = append(relations, BookmarkTag{BookmarkID: relation.BookmarkID, TagID: relation.TagID})
	}
	suggestions := make([]TagSuggestion, 0, len(fixture.Suggestions))
	for _, suggestion := range fixture.Suggestions {
		suggestions = append(suggestions, TagSuggestion{TagName: suggestion.TagName, Status: suggestion.Status})
	}

	return fixture, tags, relations, suggestions
}

// TestTagLibraryItemsMatchFixture compares the built view field by field.
func TestTagLibraryItemsMatchFixture(t *testing.T) {
	fixture, tags, relations, suggestions := loadLibFixture(t)
	got := BuildTagLibraryItems(tags, relations, suggestions)

	if len(got) != len(fixture.Expected.Items) {
		t.Fatalf("got %d items, want %d", len(got), len(fixture.Expected.Items))
	}

	for index, expected := range fixture.Expected.Items {
		item := got[index]
		if item.TagID != expected.TagID {
			t.Errorf("item %d = %q, want %q", index, item.TagID, expected.TagID)
			continue
		}
		if item.Name != expected.Name {
			t.Errorf("%s name = %q, want %q", item.TagID, item.Name, expected.Name)
		}
		if item.ParentID != expected.ParentID || item.ParentName != expected.ParentName {
			t.Errorf("%s parent = %q/%q, want %q/%q",
				item.TagID, item.ParentID, item.ParentName, expected.ParentID, expected.ParentName)
		}
		if item.ChildCount != expected.ChildCount {
			t.Errorf("%s childCount = %d, want %d", item.TagID, item.ChildCount, expected.ChildCount)
		}
		if item.BookmarkCount != expected.BookmarkCount {
			t.Errorf("%s bookmarkCount = %d, want %d", item.TagID, item.BookmarkCount, expected.BookmarkCount)
		}
		if item.PendingSuggestionCount != expected.PendingSuggestionCount {
			t.Errorf("%s pendingSuggestionCount = %d, want %d",
				item.TagID, item.PendingSuggestionCount, expected.PendingSuggestionCount)
		}
		if item.UpdatedAt != expected.UpdatedAt {
			t.Errorf("%s updatedAt = %q, want %q", item.TagID, item.UpdatedAt, expected.UpdatedAt)
		}
		if len(item.Aliases) != len(expected.Aliases) {
			t.Errorf("%s aliases = %v, want %v", item.TagID, item.Aliases, expected.Aliases)
		}
	}

	// The counts and the candidate lists.
	if got := CountOrphanTags(BuildTagLibraryItems(tags, relations, suggestions)); got != fixture.Expected.OrphanCount {
		t.Errorf("orphan count = %d, want %d", got, fixture.Expected.OrphanCount)
	}
	if got := CountLongTailTags(BuildTagLibraryItems(tags, relations, suggestions)); got != fixture.Expected.LongTailCount {
		t.Errorf("long tail count = %d, want %d", got, fixture.Expected.LongTailCount)
	}
	if got := CountUnparentedTags(BuildTagLibraryItems(tags, relations, suggestions)); got != fixture.Expected.UnparentedCount {
		t.Errorf("unparented count = %d, want %d", got, fixture.Expected.UnparentedCount)
	}

	if LongTailMaxBookmarks != fixture.LongTailMaxBookmarks {
		t.Errorf("the long-tail threshold is %d, want %d",
			LongTailMaxBookmarks, fixture.LongTailMaxBookmarks)
	}

	compareIDs(t, "removable orphans", GetRemovableOrphanTagIDs(BuildTagLibraryItems(tags, relations, suggestions)), fixture.Expected.RemovableOrphanIDs)

	roots := GetRootTagCandidates(BuildTagLibraryItems(tags, relations, suggestions))
	rootIDs := make([]string, 0, len(roots))
	for _, root := range roots {
		rootIDs = append(rootIDs, root.TagID)
	}
	compareIDs(t, "root candidates", rootIDs, fixture.Expected.RootCandidateIDs)

	for tagID, expected := range fixture.Expected.ParentCandidates {
		candidates := GetParentCandidates(BuildTagLibraryItems(tags, relations, suggestions), tagID)
		ids := make([]string, 0, len(candidates))
		for _, candidate := range candidates {
			ids = append(ids, candidate.TagID)
		}
		compareIDs(t, "parent candidates of "+tagID, ids, expected)
	}

	for tagID, expected := range fixture.Expected.HasParentCandidates {
		items := BuildTagLibraryItems(tags, relations, suggestions)
		var self TagLibraryItem
		for _, item := range items {
			if item.TagID == tagID {
				self = item
				break
			}
		}
		if got := HasParentCandidates(GetRootTagCandidates(items), self); got != expected {
			t.Errorf("hasParentCandidates(%s) = %v, want %v", tagID, got, expected)
		}
	}
}

// TestTagLibraryViewsMatchFixture covers every filter, sort and query.
func TestTagLibraryViewsMatchFixture(t *testing.T) {
	fixture, tags, relations, suggestions := loadLibFixture(t)
	items := BuildTagLibraryItems(tags, relations, suggestions)

	if len(fixture.Views) == 0 {
		t.Fatal("the fixture has no views")
	}

	for _, view := range fixture.Views {
		got := GetVisibleTagLibraryItems(items, view.SearchQuery, TagLibraryFilter(view.Filter), TagLibrarySortOrder(view.SortOrder))

		ids := make([]string, 0, len(got))
		for _, item := range got {
			ids = append(ids, item.TagID)
		}
		compareIDs(t, "filter="+view.Filter+" sort="+view.SortOrder+" query="+view.SearchQuery, ids, view.Expected)
	}
}

// TestSidebarTagsMatchFixture covers the collapse behaviour.
func TestSidebarTagsMatchFixture(t *testing.T) {
	fixture, tags, _, _ := loadLibFixture(t)

	for _, tc := range fixture.Sidebar {
		got := GetVisibleSidebarTags(tags, tc.SearchQuery, tc.Expanded, 0)

		ids := make([]string, 0, len(got.Items))
		for _, tag := range got.Items {
			ids = append(ids, tag.ID)
		}
		compareIDs(t, "sidebar query="+tc.SearchQuery, ids, tc.Expected.Items)

		if got.ShouldShowToggle != tc.Expected.ShouldShowToggle {
			t.Errorf("sidebar query=%q expanded=%v: showToggle = %v, want %v",
				tc.SearchQuery, tc.Expanded, got.ShouldShowToggle, tc.Expected.ShouldShowToggle)
		}
	}
}

// TestTheSweepNeverRemovesATagUnderReview is the rule with a data-loss history: a
// sweep that ignored pending suggestions deleted a tag the user was still reviewing,
// rejected those suggestions, and blocked the name from being suggested again.
func TestTheSweepNeverRemovesATagUnderReview(t *testing.T) {
	items := []TagLibraryItem{
		{TagID: "empty", Name: "empty"},
		{TagID: "reviewed", Name: "under review", PendingSuggestionCount: 2},
		{TagID: "parent", Name: "theme", ChildCount: 1},
		{TagID: "used", Name: "used", BookmarkCount: 4},
	}

	removable := GetRemovableOrphanTagIDs(items)
	if len(removable) != 1 || removable[0] != "empty" {
		t.Fatalf("removable = %v, want only the plain empty tag", removable)
	}

	for _, id := range removable {
		if id == "reviewed" {
			t.Error("a tag with pending suggestions must never be swept")
		}
	}
}

func compareIDs(t *testing.T, label string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s = %v, want %v", label, got, want)
		return
	}
	for index := range want {
		if got[index] != want[index] {
			t.Errorf("%s = %v, want %v", label, got, want)
			return
		}
	}
}
