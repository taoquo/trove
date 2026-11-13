package domain

import (
	"math"
	"sort"
	"strings"
)

// Graph view model.
//
// The relation graph says which tags are connected; this says which of them are on
// screen, where each one sits, and how big it is drawn. Keeping it separate from the
// renderer is what makes the scoping rules testable without a display: a node that
// should have been filtered out is a logic error, not a drawing error.

// GraphLimitNone asks for no nodes.
const GraphLimitNone = -1

// Graph scope defaults.
const (
	// DefaultGraphNodeLimit is 60. A tag graph stops being readable long before a
	// library runs out of tags.
	DefaultGraphNodeLimit = 60
	// DefaultGraphAuthorLimit is lower because authors explain why two topics touch;
	// they are not meant to become the bulk of the canvas.
	DefaultGraphAuthorLimit = 18
)

// GraphScope narrows what the graph shows.
type GraphScope struct {
	// ActiveTagIDs focus the graph. The literal "all" means no focus.
	ActiveTagIDs []string
	// ActiveAuthorHandles scope the data before the graph is built, so co-occurrence
	// weights reflect the visible slice rather than the whole workspace.
	ActiveAuthorHandles []string
	SearchQuery         string
}

// EmptyGraphScope is the scope that shows everything.
func EmptyGraphScope() GraphScope {
	return GraphScope{ActiveTagIDs: []string{"all"}}
}

// GraphViewNode is a tag plus its place on the canvas.
type GraphViewNode struct {
	TagRelationNode
	// X and Y are in the unit square, 0..1.
	X float64
	Y float64
	// Radius is in the renderer's own units, not the unit square.
	Radius float64
}

// GraphViewAuthorNode is an author plus its place on the canvas.
type GraphViewAuthorNode struct {
	TagRelationAuthorNode
	X      float64
	Y      float64
	Radius float64
}

// GraphViewModel is what a renderer needs.
type GraphViewModel struct {
	Nodes       []GraphViewNode
	Edges       []TagRelationEdge
	Authors     []GraphViewAuthorNode
	AuthorEdges []AuthorTagEdge
	// TotalTagCount is how many tags the graph holds before the limit.
	TotalTagCount int
	// HiddenTagCount is how many the limit removed, so the view can say "showing 60
	// of 214" instead of silently dropping them.
	HiddenTagCount   int
	DroppedEdgeCount int
}

// EmptyGraphViewModel is the model for an empty library.
func EmptyGraphViewModel() GraphViewModel {
	return GraphViewModel{
		Nodes: []GraphViewNode{}, Edges: []TagRelationEdge{},
		Authors: []GraphViewAuthorNode{}, AuthorEdges: []AuthorTagEdge{},
	}
}

// GraphViewModelOptions are the inputs to a build.
//
// The limits use 0 to mean "the default", because a zero-valued Go struct cannot
// distinguish an omitted field from an explicit zero. GraphLimitNone is the way to
// ask for no nodes at all.
type GraphViewModelOptions struct {
	Scope GraphScope
	// NodeLimit is how many tags to keep. 0 means DefaultGraphNodeLimit;
	// GraphLimitNone means none.
	NodeLimit int
	// AuthorLimit is how many authors to keep, with the same convention.
	AuthorLimit    int
	IncludeAuthors bool
	Relation       TagRelationGraphOptions
	// Prebuilt supplies an already-built graph.
	//
	// The relation graph walks every bookmark/tag pair and does not depend on the
	// search query, so a caller that rebuilds on each keystroke passes the memoised
	// graph and only the cheap scoping and layout run again.
	Prebuilt *TagRelationGraph
}

// normalizeHandle trims, lowercases and strips a leading "@".
func normalizeHandle(handle string) string {
	trimmed := strings.ToLower(strings.TrimSpace(handle))
	return strings.TrimPrefix(trimmed, "@")
}

// SelectScopedBookmarkTags drops the relations of bookmarks outside the author scope.
//
// It runs before the graph is built so the co-occurrence weights describe the slice
// the user is looking at. Exported because a caller memoises the graph separately
// from the search-dependent view model and has to scope both the same way.
func SelectScopedBookmarkTags(bookmarks []Bookmark, relations []BookmarkTag, activeAuthorHandles []string) []BookmarkTag {
	handles := make(map[string]bool, len(activeAuthorHandles))
	for _, handle := range activeAuthorHandles {
		if normalized := normalizeHandle(handle); normalized != "" {
			handles[normalized] = true
		}
	}
	if len(handles) == 0 {
		return relations
	}

	allowedBookmarkIDs := make(map[string]bool, len(bookmarks))
	for _, bookmark := range bookmarks {
		if !handles[normalizeHandle(bookmark.AuthorHandle)] {
			continue
		}
		allowedBookmarkIDs[bookmark.TweetID] = true
	}

	scoped := make([]BookmarkTag, 0, len(relations))
	for _, relation := range relations {
		if allowedBookmarkIDs[relation.BookmarkID] {
			scoped = append(scoped, relation)
		}
	}
	return scoped
}

// matchesSearchQuery reports whether a node's name or an alias contains the query.
func matchesSearchQuery(node TagRelationNode, searchQuery string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(searchQuery))
	if trimmed == "" {
		return true
	}

	if strings.Contains(strings.ToLower(node.Name), trimmed) {
		return true
	}
	for _, alias := range node.Aliases {
		if strings.Contains(strings.ToLower(alias), trimmed) {
			return true
		}
	}
	return false
}

// selectFocusTagIDs returns the focused tags plus their direct neighbours.
//
// Keeping the neighbours is what leaves a focused node with context instead of a
// lone dot. A nil map means "no focus", which accepts everything.
func selectFocusTagIDs(graph TagRelationGraph, activeTagIDs []string) map[string]bool {
	focused := make(map[string]bool, len(activeTagIDs))
	for _, tagID := range activeTagIDs {
		if tagID != "" && tagID != "all" {
			focused[tagID] = true
		}
	}
	if len(focused) == 0 {
		return nil
	}

	visible := make(map[string]bool, len(focused))
	for tagID := range focused {
		visible[tagID] = true
	}
	for _, edge := range graph.Edges {
		if focused[edge.SourceTagID] {
			visible[edge.TargetTagID] = true
		}
		if focused[edge.TargetTagID] {
			visible[edge.SourceTagID] = true
		}
	}

	return visible
}

// compareNodesByWeight ranks by how much of the library a tag covers.
func compareNodesByWeight(left, right TagRelationNode) int {
	if left.BookmarkCount != right.BookmarkCount {
		return right.BookmarkCount - left.BookmarkCount
	}
	if left.AuthorCount != right.AuthorCount {
		return right.AuthorCount - left.AuthorCount
	}
	return CompareNaturalNames(left.Name, right.Name)
}

// assignLayout places nodes on two rings.
//
// The layout is a deterministic radial seed: root themes on an inner ring, child
// tags on an outer ring next to their parent. It is not a force simulation, which is
// the point -- a seed that depends only on the input is reproducible, so a snapshot
// or a test does not drift.
func assignLayout(nodes []TagRelationNode) []GraphViewNode {
	if len(nodes) == 0 {
		return []GraphViewNode{}
	}

	maxBookmarkCount := 0
	for _, node := range nodes {
		if node.BookmarkCount > maxBookmarkCount {
			maxBookmarkCount = node.BookmarkCount
		}
	}

	angleByClusterID := make(map[string]float64)
	rootCount := 0
	for _, node := range nodes {
		if node.Depth == 0 {
			rootCount++
		}
	}
	rootIndex := 0
	for _, node := range nodes {
		if node.Depth != 0 {
			continue
		}
		angleByClusterID[node.ClusterID] = (float64(rootIndex) / math.Max(1, float64(rootCount))) * math.Pi * 2
		rootIndex++
	}

	childTotalByClusterID := make(map[string]int)
	for _, node := range nodes {
		if node.Depth == 1 {
			childTotalByClusterID[node.ClusterID]++
		}
	}

	childIndexByClusterID := make(map[string]int)
	view := make([]GraphViewNode, 0, len(nodes))
	for _, node := range nodes {
		baseAngle := angleByClusterID[node.ClusterID]
		angle := baseAngle
		distance := 0.42

		if node.Depth == 1 {
			total := childTotalByClusterID[node.ClusterID]
			if total == 0 {
				total = 1
			}
			index := childIndexByClusterID[node.ClusterID]
			childIndexByClusterID[node.ClusterID] = index + 1

			const spread = math.Pi / 6
			if total > 1 {
				angle = baseAngle + (float64(index)/float64(total-1)-0.5)*spread*2
			} else {
				angle = baseAngle
			}
			distance = 0.82
		}

		scale := 0.0
		if maxBookmarkCount > 0 {
			scale = float64(node.BookmarkCount) / float64(maxBookmarkCount)
		}

		view = append(view, GraphViewNode{
			TagRelationNode: node,
			X:               roundTo(0.5+math.Cos(angle)*distance*0.5, 1000),
			Y:               roundTo(0.5+math.Sin(angle)*distance*0.5, 1000),
			Radius:          roundTo(6+math.Sqrt(scale)*14, 100),
		})
	}

	return view
}

// assignAuthorLayout puts authors on an outer ring so the tag core stays readable.
//
// The order is the one the relation layer produced, which is already stable, so the
// ring does not reshuffle between frames.
func assignAuthorLayout(nodes []TagRelationAuthorNode) []GraphViewAuthorNode {
	if len(nodes) == 0 {
		return []GraphViewAuthorNode{}
	}

	maxBookmarkCount := 0
	for _, node := range nodes {
		if node.BookmarkCount > maxBookmarkCount {
			maxBookmarkCount = node.BookmarkCount
		}
	}

	view := make([]GraphViewAuthorNode, 0, len(nodes))
	for index, node := range nodes {
		angle := (float64(index)/float64(len(nodes)))*math.Pi*2 + math.Pi/float64(len(nodes))

		scale := 0.0
		if maxBookmarkCount > 0 {
			scale = float64(node.BookmarkCount) / float64(maxBookmarkCount)
		}

		view = append(view, GraphViewAuthorNode{
			TagRelationAuthorNode: node,
			X:                     roundTo(0.5+math.Cos(angle)*0.49, 1000),
			Y:                     roundTo(0.5+math.Sin(angle)*0.49, 1000),
			Radius:                roundTo(4+math.Sqrt(scale)*7, 100),
		})
	}

	return view
}

// roundTo rounds half up (floor(x + 0.5)).
func roundTo(value float64, factor float64) float64 {
	return math.Floor(value*factor+0.5) / factor
}

// BuildGraphViewModel scopes, ranks, limits and lays out the graph.
func BuildGraphViewModel(input TagRelationInput, options GraphViewModelOptions) GraphViewModel {
	scope := options.Scope
	if scope.ActiveTagIDs == nil {
		scope.ActiveTagIDs = []string{"all"}
	}

	scopedRelations := SelectScopedBookmarkTags(input.Bookmarks, input.BookmarkTags, scope.ActiveAuthorHandles)

	graph := TagRelationGraph{}
	if options.Prebuilt != nil {
		graph = *options.Prebuilt
	} else {
		relationOptions := options.Relation
		relationOptions.IncludeAuthors = options.IncludeAuthors
		authorLimit := options.AuthorLimit
		if authorLimit == 0 {
			authorLimit = DefaultGraphAuthorLimit
		}
		if authorLimit < 0 {
			authorLimit = 0
		}
		relationOptions.MaxAuthors = authorLimit
		graph = BuildTagRelationGraph(TagRelationInput{
			Tags: input.Tags, BookmarkTags: scopedRelations, Bookmarks: input.Bookmarks,
		}, relationOptions)
	}

	focusTagIDs := selectFocusTagIDs(graph, scope.ActiveTagIDs)

	candidates := make([]TagRelationNode, 0, len(graph.Nodes))
	for _, node := range graph.Nodes {
		if focusTagIDs != nil && !focusTagIDs[node.TagID] {
			continue
		}
		if !matchesSearchQuery(node, scope.SearchQuery) {
			continue
		}
		candidates = append(candidates, node)
	}

	ranked := append([]TagRelationNode{}, candidates...)
	sort.SliceStable(ranked, func(i, j int) bool { return compareNodesByWeight(ranked[i], ranked[j]) < 0 })

	limit := options.NodeLimit
	switch {
	case limit == 0:
		limit = DefaultGraphNodeLimit
	case limit == GraphLimitNone:
		limit = 0
	}
	if limit < 0 {
		limit = 0
	}
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}

	keptTagIDs := make(map[string]bool, len(ranked))
	for _, node := range ranked {
		keptTagIDs[node.TagID] = true
	}

	edges := make([]TagRelationEdge, 0, len(graph.Edges))
	for _, edge := range graph.Edges {
		if keptTagIDs[edge.SourceTagID] && keptTagIDs[edge.TargetTagID] {
			edges = append(edges, edge)
		}
	}

	// An author stays only while at least one of its topics is still visible, so
	// scoping a tag never leaves floating author dots behind.
	authorEdges := make([]AuthorTagEdge, 0, len(graph.AuthorEdges))
	connectedHandles := make(map[string]bool)
	for _, edge := range graph.AuthorEdges {
		if !keptTagIDs[edge.TagID] {
			continue
		}
		authorEdges = append(authorEdges, edge)
		connectedHandles[edge.AuthorHandle] = true
	}

	authorNodes := make([]TagRelationAuthorNode, 0, len(graph.Authors))
	for _, node := range graph.Authors {
		if connectedHandles[node.AuthorHandle] {
			authorNodes = append(authorNodes, node)
		}
	}

	return GraphViewModel{
		Nodes:            assignLayout(ranked),
		Edges:            edges,
		Authors:          assignAuthorLayout(authorNodes),
		AuthorEdges:      authorEdges,
		TotalTagCount:    len(graph.Nodes),
		HiddenTagCount:   len(graph.Nodes) - len(ranked),
		DroppedEdgeCount: graph.DroppedEdgeCount,
	}
}

// GetGraphNeighbourTagIDs returns the tags directly connected to one tag.
func GetGraphNeighbourTagIDs(edges []TagRelationEdge, tagID string) map[string]bool {
	neighbours := make(map[string]bool)
	for _, edge := range edges {
		if edge.SourceTagID == tagID {
			neighbours[edge.TargetTagID] = true
		}
		if edge.TargetTagID == tagID {
			neighbours[edge.SourceTagID] = true
		}
	}
	return neighbours
}
