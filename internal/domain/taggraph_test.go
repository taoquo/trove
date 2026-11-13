package domain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Equivalence against the frozen graph fixture, not a re-reading of the rules.
//
// The fixture is produced by running the reference implementation through Node, so
// this test fails the moment the two implementations disagree. That matters more
// here than for a pure helper: the graph decides which edges a user sees, and a
// wrong picture looks exactly as convincing as a right one.
//

type graphFixture struct {
	GeneratedFrom string              `json:"generatedFrom"`
	Defaults      graphFixtureOptions `json:"defaults"`
	Cases         []graphFixtureCase  `json:"cases"`
}

type graphFixtureOptions struct {
	MinCooccurrenceCount  int     `json:"minCooccurrenceCount"`
	MinCooccurrenceWeight float64 `json:"minCooccurrenceWeight"`
	MinAuthorBridgeCount  int     `json:"minAuthorBridgeCount"`
	MaxEdgesPerNode       int     `json:"maxEdgesPerNode"`
	TopAuthorsPerTag      int     `json:"topAuthorsPerTag"`
	IncludeAuthors        bool    `json:"includeAuthors"`
	MinAuthorTagCount     int     `json:"minAuthorTagCount"`
	MaxAuthors            int     `json:"maxAuthors"`
}

type graphFixtureCase struct {
	Name     string               `json:"name"`
	Input    graphFixtureInput    `json:"input"`
	Options  graphFixtureOptions  `json:"options"`
	Expected graphFixtureExpected `json:"expected"`
}

type graphFixtureInput struct {
	Tags         []graphFixtureTag      `json:"tags"`
	BookmarkTags []graphFixtureRelation `json:"bookmarkTags"`
	Bookmarks    []graphFixtureBookmark `json:"bookmarks"`
}

type graphFixtureTag struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Aliases  []string `json:"aliases"`
	ParentID string   `json:"parentId"`
}

type graphFixtureRelation struct {
	BookmarkID string `json:"bookmarkId"`
	TagID      string `json:"tagId"`
}

type graphFixtureBookmark struct {
	ID           string `json:"id"`
	TweetID      string `json:"tweetId"`
	AuthorHandle string `json:"authorHandle"`
}

type graphFixtureExpected struct {
	Nodes []struct {
		TagID         string   `json:"tagId"`
		Name          string   `json:"name"`
		BookmarkCount int      `json:"bookmarkCount"`
		Aliases       []string `json:"aliases"`
		ParentID      string   `json:"parentId"`
		Depth         int      `json:"depth"`
		ClusterID     string   `json:"clusterId"`
		AuthorCount   int      `json:"authorCount"`
		TopAuthors    []struct {
			AuthorHandle string `json:"authorHandle"`
			Count        int    `json:"count"`
		} `json:"topAuthors"`
	} `json:"nodes"`
	Edges []struct {
		SourceTagID string  `json:"sourceTagId"`
		TargetTagID string  `json:"targetTagId"`
		Type        string  `json:"type"`
		Weight      float64 `json:"weight"`
		RawCount    int     `json:"rawCount"`
	} `json:"edges"`
	AuthorNodes []struct {
		AuthorHandle  string `json:"authorHandle"`
		BookmarkCount int    `json:"bookmarkCount"`
		TagCount      int    `json:"tagCount"`
	} `json:"authorNodes"`
	AuthorEdges []struct {
		AuthorHandle string  `json:"authorHandle"`
		TagID        string  `json:"tagId"`
		Weight       float64 `json:"weight"`
		RawCount     int     `json:"rawCount"`
	} `json:"authorEdges"`
	DroppedEdgeCount int `json:"droppedEdgeCount"`
}

func loadGraphFixture(t *testing.T) graphFixture {
	t.Helper()

	path := filepath.Join("..", "..", "testdata", "taggraph", "taggraph.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var fixture graphFixture
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	return fixture
}

// TestTagRelationGraphMatchesFixture is the equivalence check.
func TestTagRelationGraphMatchesFixture(t *testing.T) {
	fixture := loadGraphFixture(t)

	if len(fixture.Cases) == 0 {
		t.Fatal("the fixture has no cases")
	}
	if fixture.GeneratedFrom != "frozen-fixture" {
		t.Errorf("the fixture was generated from %q, not the frozen graph fixture", fixture.GeneratedFrom)
	}

	for _, tc := range fixture.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			got := BuildTagRelationGraph(toInput(tc.Input), toOptions(tc.Options))

			// Nodes.
			if len(got.Nodes) != len(tc.Expected.Nodes) {
				t.Fatalf("got %d nodes, want %d", len(got.Nodes), len(tc.Expected.Nodes))
			}
			for index, want := range tc.Expected.Nodes {
				actual := got.Nodes[index]
				if actual.TagID != want.TagID {
					t.Errorf("node %d id = %q, want %q", index, actual.TagID, want.TagID)
				}
				if actual.Name != want.Name {
					t.Errorf("node %s name = %q, want %q", actual.TagID, actual.Name, want.Name)
				}
				if actual.BookmarkCount != want.BookmarkCount {
					t.Errorf("node %s bookmarkCount = %d, want %d", actual.TagID, actual.BookmarkCount, want.BookmarkCount)
				}
				if actual.ParentID != want.ParentID {
					t.Errorf("node %s parentId = %q, want %q", actual.TagID, actual.ParentID, want.ParentID)
				}
				if actual.Depth != want.Depth {
					t.Errorf("node %s depth = %d, want %d", actual.TagID, actual.Depth, want.Depth)
				}
				if actual.ClusterID != want.ClusterID {
					t.Errorf("node %s clusterId = %q, want %q", actual.TagID, actual.ClusterID, want.ClusterID)
				}
				if actual.AuthorCount != want.AuthorCount {
					t.Errorf("node %s authorCount = %d, want %d", actual.TagID, actual.AuthorCount, want.AuthorCount)
				}
				if len(actual.Aliases) != len(want.Aliases) {
					t.Errorf("node %s aliases = %v, want %v", actual.TagID, actual.Aliases, want.Aliases)
				}
				if len(actual.TopAuthors) != len(want.TopAuthors) {
					t.Fatalf("node %s topAuthors = %v, want %v", actual.TagID, actual.TopAuthors, want.TopAuthors)
				}
				for authorIndex, wantAuthor := range want.TopAuthors {
					if actual.TopAuthors[authorIndex].AuthorHandle != wantAuthor.AuthorHandle ||
						actual.TopAuthors[authorIndex].Count != wantAuthor.Count {
						t.Errorf("node %s topAuthor %d = %+v, want %+v",
							actual.TagID, authorIndex, actual.TopAuthors[authorIndex], wantAuthor)
					}
				}
			}

			// Edges.
			if len(got.Edges) != len(tc.Expected.Edges) {
				t.Fatalf("got %d edges, want %d\ngot:  %+v\nwant: %+v",
					len(got.Edges), len(tc.Expected.Edges), got.Edges, tc.Expected.Edges)
			}
			for index, want := range tc.Expected.Edges {
				actual := got.Edges[index]
				if actual.SourceTagID != want.SourceTagID || actual.TargetTagID != want.TargetTagID {
					t.Errorf("edge %d = %s->%s, want %s->%s", index,
						actual.SourceTagID, actual.TargetTagID, want.SourceTagID, want.TargetTagID)
				}
				if string(actual.Type) != want.Type {
					t.Errorf("edge %d type = %q, want %q", index, actual.Type, want.Type)
				}
				if actual.Weight != want.Weight {
					t.Errorf("edge %d weight = %v, want %v", index, actual.Weight, want.Weight)
				}
				if actual.RawCount != want.RawCount {
					t.Errorf("edge %d rawCount = %d, want %d", index, actual.RawCount, want.RawCount)
				}
			}

			// The author layer.
			if len(got.Authors) != len(tc.Expected.AuthorNodes) {
				t.Fatalf("got %d author nodes, want %d", len(got.Authors), len(tc.Expected.AuthorNodes))
			}
			for index, want := range tc.Expected.AuthorNodes {
				actual := got.Authors[index]
				if actual.AuthorHandle != want.AuthorHandle ||
					actual.BookmarkCount != want.BookmarkCount ||
					actual.TagCount != want.TagCount {
					t.Errorf("author node %d = %+v, want %+v", index, actual, want)
				}
			}

			if len(got.AuthorEdges) != len(tc.Expected.AuthorEdges) {
				t.Fatalf("got %d author edges, want %d", len(got.AuthorEdges), len(tc.Expected.AuthorEdges))
			}
			for index, want := range tc.Expected.AuthorEdges {
				actual := got.AuthorEdges[index]
				if actual.AuthorHandle != want.AuthorHandle || actual.TagID != want.TagID ||
					actual.Weight != want.Weight || actual.RawCount != want.RawCount {
					t.Errorf("author edge %d = %+v, want %+v", index, actual, want)
				}
			}

			if got.DroppedEdgeCount != tc.Expected.DroppedEdgeCount {
				t.Errorf("droppedEdgeCount = %d, want %d", got.DroppedEdgeCount, tc.Expected.DroppedEdgeCount)
			}
		})
	}
}

// TestTagRelationGraphDefaultsMatchFixture pins the thresholds against the
// fixture's exported defaults, so a changed constant cannot pass unnoticed.
func TestTagRelationGraphDefaultsMatchFixture(t *testing.T) {
	want := loadGraphFixture(t).Defaults
	got := DefaultTagRelationOptions()

	if got.MinCooccurrenceCount != want.MinCooccurrenceCount ||
		got.MinCooccurrenceWeight != want.MinCooccurrenceWeight ||
		got.MinAuthorBridgeCount != want.MinAuthorBridgeCount ||
		got.MaxEdgesPerNode != want.MaxEdgesPerNode ||
		got.TopAuthorsPerTag != want.TopAuthorsPerTag ||
		got.IncludeAuthors != want.IncludeAuthors ||
		got.MinAuthorTagCount != want.MinAuthorTagCount ||
		got.MaxAuthors != want.MaxAuthors {
		t.Errorf("defaults = %+v, want %+v", got, want)
	}
}

func toInput(fixture graphFixtureInput) TagRelationInput {
	input := TagRelationInput{
		Tags:         make([]Tag, 0, len(fixture.Tags)),
		BookmarkTags: make([]BookmarkTag, 0, len(fixture.BookmarkTags)),
		Bookmarks:    make([]Bookmark, 0, len(fixture.Bookmarks)),
	}
	for _, tag := range fixture.Tags {
		input.Tags = append(input.Tags, Tag{
			ID: tag.ID, Name: tag.Name, Aliases: tag.Aliases, ParentID: tag.ParentID,
		})
	}
	for _, relation := range fixture.BookmarkTags {
		input.BookmarkTags = append(input.BookmarkTags, BookmarkTag{
			BookmarkID: relation.BookmarkID, TagID: relation.TagID,
		})
	}
	for _, bookmark := range fixture.Bookmarks {
		input.Bookmarks = append(input.Bookmarks, Bookmark{
			TweetID: bookmark.TweetID, AuthorHandle: bookmark.AuthorHandle,
		})
	}
	return input
}

func toOptions(fixture graphFixtureOptions) TagRelationGraphOptions {
	return TagRelationGraphOptions{
		MinCooccurrenceCount:  fixture.MinCooccurrenceCount,
		MinCooccurrenceWeight: fixture.MinCooccurrenceWeight,
		MinAuthorBridgeCount:  fixture.MinAuthorBridgeCount,
		MaxEdgesPerNode:       fixture.MaxEdgesPerNode,
		TopAuthorsPerTag:      fixture.TopAuthorsPerTag,
		IncludeAuthors:        fixture.IncludeAuthors,
		MinAuthorTagCount:     fixture.MinAuthorTagCount,
		MaxAuthors:            fixture.MaxAuthors,
	}
}
