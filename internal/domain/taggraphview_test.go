package domain

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// Equivalence against the frozen graph view model fixture, including the layout
// arithmetic: a rounding difference would move every node on the canvas, which is
// exactly the kind of drift that re-reading the rules would not catch.
type viewFixture struct {
	GeneratedFrom string            `json:"generatedFrom"`
	Tags          []viewFixtureTag  `json:"tags"`
	Bookmarks     []viewFixtureBm   `json:"bookmarks"`
	BookmarkTags  []viewFixtureRel  `json:"bookmarkTags"`
	Cases         []viewFixtureCase `json:"cases"`
}

type viewFixtureTag struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Aliases  []string `json:"aliases"`
	ParentID string   `json:"parentId"`
}

type viewFixtureBm struct {
	ID           string `json:"id"`
	TweetID      string `json:"tweetId"`
	AuthorHandle string `json:"authorHandle"`
}

type viewFixtureRel struct {
	BookmarkID string `json:"bookmarkId"`
	TagID      string `json:"tagId"`
}

type viewFixtureCase struct {
	Name           string           `json:"name"`
	Scope          viewFixtureScope `json:"scope"`
	NodeLimit      *int             `json:"nodeLimit"`
	AuthorLimit    *int             `json:"authorLimit"`
	IncludeAuthors bool             `json:"includeAuthors"`
	Expected       struct {
		Nodes []struct {
			TagID         string  `json:"tagId"`
			Name          string  `json:"name"`
			BookmarkCount int     `json:"bookmarkCount"`
			Depth         int     `json:"depth"`
			ClusterID     string  `json:"clusterId"`
			X             float64 `json:"x"`
			Y             float64 `json:"y"`
			Radius        float64 `json:"radius"`
		} `json:"nodes"`
		Edges []struct {
			SourceTagID string `json:"sourceTagId"`
			TargetTagID string `json:"targetTagId"`
		} `json:"edges"`
		AuthorNodes []struct {
			AuthorHandle string `json:"authorHandle"`
			TagCount     int    `json:"tagCount"`
		} `json:"authorNodes"`
		AuthorEdges []struct {
			AuthorHandle string `json:"authorHandle"`
			TagID        string `json:"tagId"`
		} `json:"authorEdges"`
		TotalTagCount    int `json:"totalTagCount"`
		HiddenTagCount   int `json:"hiddenTagCount"`
		DroppedEdgeCount int `json:"droppedEdgeCount"`
	} `json:"expected"`
	Neighbours map[string][]string `json:"neighbours"`
}

type viewFixtureScope struct {
	ActiveTagIDs        []string `json:"activeTagIds"`
	ActiveAuthorHandles []string `json:"activeAuthorHandles"`
	SearchQuery         string   `json:"searchQuery"`
}

func TestGraphViewModelMatchesFixture(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "graphview", "graphview.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var fixture viewFixture
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	if len(fixture.Cases) == 0 {
		t.Fatal("the fixture has no cases")
	}
	if fixture.GeneratedFrom != "frozen-fixture" {
		t.Errorf("fixture generated from %q", fixture.GeneratedFrom)
	}

	input := TagRelationInput{
		Tags:         make([]Tag, 0, len(fixture.Tags)),
		BookmarkTags: make([]BookmarkTag, 0, len(fixture.BookmarkTags)),
		Bookmarks:    make([]Bookmark, 0, len(fixture.Bookmarks)),
	}
	for _, tag := range fixture.Tags {
		input.Tags = append(input.Tags, Tag{ID: tag.ID, Name: tag.Name, Aliases: tag.Aliases, ParentID: tag.ParentID})
	}
	for _, bookmark := range fixture.Bookmarks {
		input.Bookmarks = append(input.Bookmarks, Bookmark{TweetID: bookmark.TweetID, AuthorHandle: bookmark.AuthorHandle})
	}
	for _, relation := range fixture.BookmarkTags {
		input.BookmarkTags = append(input.BookmarkTags, BookmarkTag{BookmarkID: relation.BookmarkID, TagID: relation.TagID})
	}

	for _, tc := range fixture.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			options := GraphViewModelOptions{
				Scope: GraphScope{
					ActiveTagIDs:        tc.Scope.ActiveTagIDs,
					ActiveAuthorHandles: tc.Scope.ActiveAuthorHandles,
					SearchQuery:         tc.Scope.SearchQuery,
				},
				IncludeAuthors: tc.IncludeAuthors,
			}
			// The fixture records null for an omitted limit, which is a different case
			// from an explicit zero.
			if tc.NodeLimit != nil {
				options.NodeLimit = *tc.NodeLimit
			}
			if tc.AuthorLimit != nil {
				options.AuthorLimit = *tc.AuthorLimit
			} else {
				options.AuthorLimit = GraphLimitNone
			}

			got := BuildGraphViewModel(input, options)
			want := tc.Expected

			if len(got.Nodes) != len(want.Nodes) {
				t.Fatalf("got %d nodes, want %d", len(got.Nodes), len(want.Nodes))
			}
			for index, expected := range want.Nodes {
				node := got.Nodes[index]
				if node.TagID != expected.TagID {
					t.Errorf("node %d = %q, want %q", index, node.TagID, expected.TagID)
					continue
				}
				if node.BookmarkCount != expected.BookmarkCount {
					t.Errorf("%s bookmarkCount = %d, want %d", node.TagID, node.BookmarkCount, expected.BookmarkCount)
				}
				if node.Depth != expected.Depth || node.ClusterID != expected.ClusterID {
					t.Errorf("%s depth/cluster = %d/%q, want %d/%q",
						node.TagID, node.Depth, node.ClusterID, expected.Depth, expected.ClusterID)
				}
				// The layout is compared exactly: the implementation rounds the same way,
				// and an epsilon here would hide a systematic offset.
				if math.Abs(node.X-expected.X) > 1e-9 || math.Abs(node.Y-expected.Y) > 1e-9 {
					t.Errorf("%s at (%v, %v), want (%v, %v)", node.TagID, node.X, node.Y, expected.X, expected.Y)
				}
				if math.Abs(node.Radius-expected.Radius) > 1e-9 {
					t.Errorf("%s radius %v, want %v", node.TagID, node.Radius, expected.Radius)
				}
			}

			if len(got.Edges) != len(want.Edges) {
				t.Fatalf("got %d edges, want %d", len(got.Edges), len(want.Edges))
			}
			for index, expected := range want.Edges {
				edge := got.Edges[index]
				if edge.SourceTagID != expected.SourceTagID || edge.TargetTagID != expected.TargetTagID {
					t.Errorf("edge %d = %s->%s, want %s->%s", index,
						edge.SourceTagID, edge.TargetTagID, expected.SourceTagID, expected.TargetTagID)
				}
			}

			if len(got.Authors) != len(want.AuthorNodes) {
				t.Errorf("got %d author nodes, want %d", len(got.Authors), len(want.AuthorNodes))
			} else {
				for index, expected := range want.AuthorNodes {
					author := got.Authors[index]
					if author.AuthorHandle != expected.AuthorHandle || author.TagCount != expected.TagCount {
						t.Errorf("author %d = %+v, want %+v", index, author, expected)
					}
				}
			}

			if len(got.AuthorEdges) != len(want.AuthorEdges) {
				t.Errorf("got %d author edges, want %d", len(got.AuthorEdges), len(want.AuthorEdges))
			}

			if got.TotalTagCount != want.TotalTagCount {
				t.Errorf("totalTagCount = %d, want %d", got.TotalTagCount, want.TotalTagCount)
			}
			if got.HiddenTagCount != want.HiddenTagCount {
				t.Errorf("hiddenTagCount = %d, want %d", got.HiddenTagCount, want.HiddenTagCount)
			}
			if got.DroppedEdgeCount != want.DroppedEdgeCount {
				t.Errorf("droppedEdgeCount = %d, want %d", got.DroppedEdgeCount, want.DroppedEdgeCount)
			}

			// The neighbour lookup, which drives the highlight when a tag is focused.
			for tagID, expectedNeighbours := range tc.Neighbours {
				gotNeighbours := GetGraphNeighbourTagIDs(got.Edges, tagID)
				if len(gotNeighbours) != len(expectedNeighbours) {
					t.Errorf("neighbours of %q = %d, want %d", tagID, len(gotNeighbours), len(expectedNeighbours))
					continue
				}
				for _, neighbour := range expectedNeighbours {
					if !gotNeighbours[neighbour] {
						t.Errorf("neighbours of %q must include %q", tagID, neighbour)
					}
				}
			}
		})
	}
}
