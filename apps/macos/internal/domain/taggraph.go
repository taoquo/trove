package domain

import (
	"math"
	"sort"
	"strings"
)

// Tag relation graph, ported from src/lib/graph/tagRelations.ts.
//
// The graph answers "which topics sit together" from three signals: tags applied to
// the same bookmark (cooccurrence), parent/child structure (hierarchy), and authors
// who write about several topics (author-bridge). Reproducing it matters because the
// extension's graph view is a feature, not a decoration: the edge weights are what
// make a fifty-tag graph readable.
//
// The port is deliberately literal. Every threshold, every rounding and every
// tie-break is the source's, including where the source's choice looks arbitrary --
// a "cleaned up" port would produce a different picture for the same library, which
// is exactly the kind of difference nobody would notice until it mattered.

// TagRelationEdgeType names what connects two tags.
type TagRelationEdgeType string

const (
	// EdgeCooccurrence: the two tags are applied to the same bookmark.
	EdgeCooccurrence TagRelationEdgeType = "cooccurrence"
	// EdgeHierarchy: one tag is the other's parent.
	EdgeHierarchy TagRelationEdgeType = "hierarchy"
	// EdgeAuthorBridge: one author writes about both, but not in one bookmark.
	EdgeAuthorBridge TagRelationEdgeType = "author-bridge"
)

// TagRelationAuthor is one of a tag's most frequent authors.
type TagRelationAuthor struct {
	AuthorHandle string
	Count        int
}

// TagRelationAuthorNode is an author as a first-class graph node.
//
// The tag/author bipartite layer is what turns "topics that sit together" into
// "people who connect topics".
type TagRelationAuthorNode struct {
	AuthorHandle  string
	BookmarkCount int
	TagCount      int
}

// AuthorTagEdge links an author node to a tag node.
type AuthorTagEdge struct {
	AuthorHandle string
	TagID        string
	Weight       float64
	RawCount     int
}

// TagRelationNode is a tag in the graph.
type TagRelationNode struct {
	TagID         string
	Name          string
	BookmarkCount int
	Aliases       []string
	// ParentID is empty when the tag is a root, or when its parent was removed.
	ParentID string
	// Depth is 0 for a root and 1 for a child. There are only two levels, which is
	// what the schema's trigger enforces.
	Depth int
	// ClusterID is the root of the tag's hierarchy: the parent when there is one,
	// otherwise the tag itself.
	ClusterID   string
	AuthorCount int
	TopAuthors  []TagRelationAuthor
}

// TagRelationEdge connects two tags.
type TagRelationEdge struct {
	SourceTagID string
	TargetTagID string
	Type        TagRelationEdgeType
	Weight      float64
	RawCount    int
}

// TagRelationGraph is the built graph.
type TagRelationGraph struct {
	Nodes   []TagRelationNode
	Edges   []TagRelationEdge
	Authors []TagRelationAuthorNode
	// AuthorEdges are the bipartite links, present only when authors are included.
	AuthorEdges []AuthorTagEdge
	// DroppedEdgeCount records what the thresholds and the budget removed, so the
	// view can say "showing the strongest N" rather than pretending it is all of it.
	DroppedEdgeCount int
}

// TagRelationGraphOptions tunes the build. A zero value means "use the default",
// which is why every field is applied over the defaults rather than read directly.
type TagRelationGraphOptions struct {
	MinCooccurrenceCount  int
	MinCooccurrenceWeight float64
	MinAuthorBridgeCount  int
	MaxEdgesPerNode       int
	TopAuthorsPerTag      int
	IncludeAuthors        bool
	MinAuthorTagCount     int
	MaxAuthors            int
}

// DefaultTagRelationOptions are the source's defaults.
func DefaultTagRelationOptions() TagRelationGraphOptions {
	return TagRelationGraphOptions{
		MinCooccurrenceCount:  1,
		MinCooccurrenceWeight: 0.05,
		MinAuthorBridgeCount:  2,
		MaxEdgesPerNode:       8,
		TopAuthorsPerTag:      3,
		IncludeAuthors:        false,
		MinAuthorTagCount:     2,
		MaxAuthors:            24,
	}
}

// merge overlays the caller's explicit values on the defaults.
//
// The zero value is how "not specified" is spelled, so a caller cannot ask for a
// threshold of zero by leaving it out -- which is the behaviour the source has,
// because it spreads an options object over the defaults.
func (o TagRelationGraphOptions) merge() TagRelationGraphOptions {
	settings := DefaultTagRelationOptions()
	if o.MinCooccurrenceCount != 0 {
		settings.MinCooccurrenceCount = o.MinCooccurrenceCount
	}
	if o.MinCooccurrenceWeight != 0 {
		settings.MinCooccurrenceWeight = o.MinCooccurrenceWeight
	}
	if o.MinAuthorBridgeCount != 0 {
		settings.MinAuthorBridgeCount = o.MinAuthorBridgeCount
	}
	if o.MaxEdgesPerNode != 0 {
		settings.MaxEdgesPerNode = o.MaxEdgesPerNode
	}
	if o.TopAuthorsPerTag != 0 {
		settings.TopAuthorsPerTag = o.TopAuthorsPerTag
	}
	if o.IncludeAuthors {
		settings.IncludeAuthors = true
	}
	if o.MinAuthorTagCount != 0 {
		settings.MinAuthorTagCount = o.MinAuthorTagCount
	}
	if o.MaxAuthors != 0 {
		settings.MaxAuthors = o.MaxAuthors
	}
	return settings
}

// pairSeparator cannot occur in a tag id, so a pair key cannot be forged.
const pairSeparator = "\x00"

// pairKey orders the two ids, so a pair has one key however it is reached.
func pairKey(a, b string) string {
	if a < b {
		return a + pairSeparator + b
	}
	return b + pairSeparator + a
}

func splitPairKey(key string) (string, string) {
	parts := strings.SplitN(key, pairSeparator, 2)
	if len(parts) != 2 {
		return key, ""
	}
	return parts[0], parts[1]
}

// compareLocale approximates JavaScript's localeCompare for the identifiers and
// handles that reach it.
//
// The source sorts with localeCompare, whose default collation is case-insensitive
// in a way byte order is not: it puts "apple" before "Banana", where a byte compare
// puts "Banana" first because uppercase sorts lower. This reproduces that for ASCII
// by folding case first and falling back to the raw bytes to break ties, which keeps
// the sort total. It is an approximation, and the only place it shows is the order of
// edges that are otherwise equal.
func compareLocale(a, b string) int {
	lowerA, lowerB := strings.ToLower(a), strings.ToLower(b)
	if lowerA != lowerB {
		if lowerA < lowerB {
			return -1
		}
		return 1
	}
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func compareAuthors(left, right TagRelationAuthor) int {
	if left.Count != right.Count {
		// Descending by count.
		return right.Count - left.Count
	}
	return compareLocale(left.AuthorHandle, right.AuthorHandle)
}

// edgeTypePriority orders the edge list: structure first, then co-occurrence, then
// the weaker author signal.
var edgeTypePriority = map[TagRelationEdgeType]int{
	EdgeHierarchy:    0,
	EdgeCooccurrence: 1,
	EdgeAuthorBridge: 2,
}

func compareEdges(left, right TagRelationEdge) int {
	if left.Type != right.Type {
		return edgeTypePriority[left.Type] - edgeTypePriority[right.Type]
	}
	if left.Weight != right.Weight {
		if left.Weight > right.Weight {
			return -1
		}
		return 1
	}
	if comparison := compareLocale(left.SourceTagID, right.SourceTagID); comparison != 0 {
		return comparison
	}
	return compareLocale(left.TargetTagID, right.TargetTagID)
}

// roundWeight keeps three decimals, matching the source.
//
// It matters beyond tidiness: the value reaches a JSON round trip and a test that
// compares graphs, so an unrounded float would make two identical graphs differ.
func roundWeight(value float64) float64 {
	// Math.round(x) is floor(x + 0.5), not Go's round-half-away-from-zero, and the
	// difference shows on a weight that lands exactly on a half.
	return math.Floor(value*1000+0.5) / 1000
}

// TagRelationInput is what the graph is built from.
type TagRelationInput struct {
	Tags         []Tag
	BookmarkTags []BookmarkTag
	// Bookmarks supply author handles. Without them the graph has no author layer
	// and no author-bridge edges, which is a valid graph rather than an error.
	Bookmarks []Bookmark
}

// BuildTagRelationGraph builds the graph.
func BuildTagRelationGraph(input TagRelationInput, options TagRelationGraphOptions) TagRelationGraph {
	settings := options.merge()

	tagByID := make(map[string]Tag, len(input.Tags))
	for _, tag := range input.Tags {
		tagByID[tag.ID] = tag
	}

	// A tag counts once per bookmark, so a duplicated relation row cannot inflate
	// a tag's weight.
	tagIDsByBookmarkID := make(map[string]map[string]bool)
	bookmarkCountByTagID := make(map[string]int)

	for _, relation := range input.BookmarkTags {
		if _, known := tagByID[relation.TagID]; !known {
			continue
		}

		tagIDs := tagIDsByBookmarkID[relation.BookmarkID]
		if tagIDs == nil {
			tagIDs = make(map[string]bool)
			tagIDsByBookmarkID[relation.BookmarkID] = tagIDs
		}
		if tagIDs[relation.TagID] {
			continue
		}

		tagIDs[relation.TagID] = true
		bookmarkCountByTagID[relation.TagID]++
	}

	authorHandleByBookmarkID := make(map[string]string, len(input.Bookmarks))
	for _, bookmark := range input.Bookmarks {
		handle := strings.TrimSpace(bookmark.AuthorHandle)
		if bookmark.TweetID == "" || handle == "" {
			continue
		}
		authorHandleByBookmarkID[bookmark.TweetID] = handle
	}

	authorCountsByTagID := make(map[string]map[string]int)
	tagIDsByAuthorHandle := make(map[string]map[string]bool)
	taggedBookmarkCountByAuthor := make(map[string]int)

	for bookmarkID, tagIDsForBookmark := range tagIDsByBookmarkID {
		handle, known := authorHandleByBookmarkID[bookmarkID]
		if !known {
			continue
		}

		tagIDsForAuthor := tagIDsByAuthorHandle[handle]
		if tagIDsForAuthor == nil {
			tagIDsForAuthor = make(map[string]bool)
			tagIDsByAuthorHandle[handle] = tagIDsForAuthor
		}
		taggedBookmarkCountByAuthor[handle]++

		for tagID := range tagIDsForBookmark {
			tagIDsForAuthor[tagID] = true
			counts := authorCountsByTagID[tagID]
			if counts == nil {
				counts = make(map[string]int)
				authorCountsByTagID[tagID] = counts
			}
			counts[handle]++
		}
	}

	// Co-occurrence: every pair of tags on one bookmark.
	cooccurrenceByPairKey := make(map[string]int)
	for _, tagIDsForBookmark := range tagIDsByBookmarkID {
		sorted := sortedKeys(tagIDsForBookmark)
		for i := 0; i < len(sorted); i++ {
			for j := i + 1; j < len(sorted); j++ {
				cooccurrenceByPairKey[pairKey(sorted[i], sorted[j])]++
			}
		}
	}

	// Author bridges: pairs from one author's tag set, but only where the pair is
	// not already a co-occurrence. Adjacency within one bookmark is the stronger
	// signal, so it wins the pair.
	authorHandlesByPairKey := make(map[string]map[string]bool)
	for handle, tagIDsForAuthor := range tagIDsByAuthorHandle {
		if len(tagIDsForAuthor) < 2 {
			continue
		}

		sorted := sortedKeys(tagIDsForAuthor)
		for i := 0; i < len(sorted); i++ {
			for j := i + 1; j < len(sorted); j++ {
				key := pairKey(sorted[i], sorted[j])
				if _, isCooccurrence := cooccurrenceByPairKey[key]; isCooccurrence {
					continue
				}

				handles := authorHandlesByPairKey[key]
				if handles == nil {
					handles = make(map[string]bool)
					authorHandlesByPairKey[key] = handles
				}
				handles[handle] = true
			}
		}
	}

	// Nodes, in the order the tags arrived.
	nodes := make([]TagRelationNode, 0, len(input.Tags))
	for _, tag := range input.Tags {
		// A parentId pointing at a removed tag degrades to a root.
		parentID := ""
		if tag.ParentID != "" {
			if _, known := tagByID[tag.ParentID]; known {
				parentID = tag.ParentID
			}
		}

		authorCounts := authorCountsByTagID[tag.ID]
		topAuthors := make([]TagRelationAuthor, 0, len(authorCounts))
		for handle, count := range authorCounts {
			topAuthors = append(topAuthors, TagRelationAuthor{AuthorHandle: handle, Count: count})
		}
		sort.Slice(topAuthors, func(i, j int) bool { return compareAuthors(topAuthors[i], topAuthors[j]) < 0 })
		if limit := max(settings.TopAuthorsPerTag, 0); len(topAuthors) > limit {
			topAuthors = topAuthors[:limit]
		}

		depth := 0
		if parentID != "" {
			depth = 1
		}
		clusterID := tag.ID
		if parentID != "" {
			clusterID = parentID
		}

		aliases := tag.Aliases
		if aliases == nil {
			aliases = []string{}
		}

		nodes = append(nodes, TagRelationNode{
			TagID:         tag.ID,
			Name:          tag.Name,
			BookmarkCount: bookmarkCountByTagID[tag.ID],
			Aliases:       aliases,
			ParentID:      parentID,
			Depth:         depth,
			ClusterID:     clusterID,
			AuthorCount:   len(authorCounts),
			TopAuthors:    topAuthors,
		})
	}

	authorCountByTagID := make(map[string]int, len(nodes))
	for _, node := range nodes {
		authorCountByTagID[node.TagID] = node.AuthorCount
	}

	hierarchyEdges := make([]TagRelationEdge, 0)
	for _, node := range nodes {
		if node.ParentID == "" {
			continue
		}
		hierarchyEdges = append(hierarchyEdges, TagRelationEdge{
			SourceTagID: node.ParentID,
			TargetTagID: node.TagID,
			Type:        EdgeHierarchy,
			Weight:      1,
			RawCount:    1,
		})
	}

	droppedEdgeCount := 0
	weightedEdges := make([]TagRelationEdge, 0)

	for key, rawCount := range cooccurrenceByPairKey {
		sourceTagID, targetTagID := splitPairKey(key)
		sourceCount := bookmarkCountByTagID[sourceTagID]
		targetCount := bookmarkCountByTagID[targetTagID]
		union := sourceCount + targetCount - rawCount

		// Jaccard, so a high-volume tag cannot dominate every edge in the graph.
		weight := 0.0
		if union > 0 {
			weight = float64(rawCount) / float64(union)
		}

		if rawCount < settings.MinCooccurrenceCount || weight < settings.MinCooccurrenceWeight {
			droppedEdgeCount++
			continue
		}

		weightedEdges = append(weightedEdges, TagRelationEdge{
			SourceTagID: sourceTagID,
			TargetTagID: targetTagID,
			Type:        EdgeCooccurrence,
			Weight:      roundWeight(weight),
			RawCount:    rawCount,
		})
	}

	for key, handles := range authorHandlesByPairKey {
		sourceTagID, targetTagID := splitPairKey(key)
		rawCount := len(handles)

		if rawCount < settings.MinAuthorBridgeCount {
			droppedEdgeCount++
			continue
		}

		// Normalised by the *less* connected endpoint, so a bridge counts fully
		// when it explains the narrower of the two tags.
		denominator := min(authorCountByTagID[sourceTagID], authorCountByTagID[targetTagID])
		weight := 0.0
		if denominator > 0 {
			weight = min(1.0, float64(rawCount)/float64(denominator))
		}

		weightedEdges = append(weightedEdges, TagRelationEdge{
			SourceTagID: sourceTagID,
			TargetTagID: targetTagID,
			Type:        EdgeAuthorBridge,
			Weight:      roundWeight(weight),
			RawCount:    rawCount,
		})
	}

	keptEdges := applyPerNodeEdgeBudget(weightedEdges, settings.MaxEdgesPerNode)
	droppedEdgeCount += len(weightedEdges) - len(keptEdges)

	authorNodes := []TagRelationAuthorNode{}
	authorEdges := []AuthorTagEdge{}
	if settings.IncludeAuthors {
		authorNodes, authorEdges = buildAuthorLayer(
			tagIDsByAuthorHandle, taggedBookmarkCountByAuthor, authorCountsByTagID,
			settings.MinAuthorTagCount, settings.MaxAuthors)
	}

	edges := append(append([]TagRelationEdge{}, hierarchyEdges...), keptEdges...)
	sort.SliceStable(edges, func(i, j int) bool { return compareEdges(edges[i], edges[j]) < 0 })

	return TagRelationGraph{
		Nodes:            nodes,
		Edges:            edges,
		Authors:          authorNodes,
		AuthorEdges:      authorEdges,
		DroppedEdgeCount: droppedEdgeCount,
	}
}

// buildAuthorLayer keeps only authors who bridge at least minAuthorTagCount topics.
//
// Without the filter every one-off author becomes a leaf and the graph turns into a
// hairball. Ranking by bridged topics first keeps the connectors, not just the
// loudest accounts.
func buildAuthorLayer(
	tagIDsByAuthorHandle map[string]map[string]bool,
	taggedBookmarkCountByAuthor map[string]int,
	authorCountsByTagID map[string]map[string]int,
	minAuthorTagCount int,
	maxAuthors int,
) ([]TagRelationAuthorNode, []AuthorTagEdge) {
	if maxAuthors <= 0 {
		return []TagRelationAuthorNode{}, []AuthorTagEdge{}
	}

	threshold := max(1, minAuthorTagCount)
	ranked := make([]TagRelationAuthorNode, 0)
	for handle, tagIDsForAuthor := range tagIDsByAuthorHandle {
		if len(tagIDsForAuthor) < threshold {
			continue
		}
		ranked = append(ranked, TagRelationAuthorNode{
			AuthorHandle:  handle,
			BookmarkCount: taggedBookmarkCountByAuthor[handle],
			TagCount:      len(tagIDsForAuthor),
		})
	}

	sort.Slice(ranked, func(i, j int) bool {
		left, right := ranked[i], ranked[j]
		if left.TagCount != right.TagCount {
			return left.TagCount > right.TagCount
		}
		if left.BookmarkCount != right.BookmarkCount {
			return left.BookmarkCount > right.BookmarkCount
		}
		return compareLocale(left.AuthorHandle, right.AuthorHandle) < 0
	})

	if len(ranked) > maxAuthors {
		ranked = ranked[:maxAuthors]
	}
	authorNodes := ranked

	keptHandles := make(map[string]bool, len(authorNodes))
	for _, node := range authorNodes {
		keptHandles[node.AuthorHandle] = true
	}

	authorEdges := make([]AuthorTagEdge, 0)
	for tagID, counts := range authorCountsByTagID {
		for handle, rawCount := range counts {
			if !keptHandles[handle] {
				continue
			}

			authorTotal := taggedBookmarkCountByAuthor[handle]
			weight := 0.0
			if authorTotal > 0 {
				weight = roundWeight(min(1.0, float64(rawCount)/float64(authorTotal)))
			}

			authorEdges = append(authorEdges, AuthorTagEdge{
				AuthorHandle: handle,
				TagID:        tagID,
				Weight:       weight,
				RawCount:     rawCount,
			})
		}
	}

	sort.Slice(authorEdges, func(i, j int) bool {
		left, right := authorEdges[i], authorEdges[j]
		if comparison := compareLocale(left.AuthorHandle, right.AuthorHandle); comparison != 0 {
			return comparison < 0
		}
		if left.RawCount != right.RawCount {
			return left.RawCount > right.RawCount
		}
		return compareLocale(left.TagID, right.TagID) < 0
	})

	return authorNodes, authorEdges
}

// applyPerNodeEdgeBudget keeps an edge when it ranks inside the budget of *either*
// endpoint, which is what keeps a fifty-tag graph readable instead of nearly fully
// connected.
func applyPerNodeEdgeBudget(edges []TagRelationEdge, maxEdgesPerNode int) []TagRelationEdge {
	if maxEdgesPerNode <= 0 {
		return []TagRelationEdge{}
	}

	edgesByTagID := make(map[string][]int)
	for index, edge := range edges {
		for _, tagID := range []string{edge.SourceTagID, edge.TargetTagID} {
			edgesByTagID[tagID] = append(edgesByTagID[tagID], index)
		}
	}

	kept := make([]bool, len(edges))
	for _, bucket := range edgesByTagID {
		ranked := append([]int{}, bucket...)
		sort.SliceStable(ranked, func(i, j int) bool {
			return compareEdges(edges[ranked[i]], edges[ranked[j]]) < 0
		})
		if len(ranked) > maxEdgesPerNode {
			ranked = ranked[:maxEdgesPerNode]
		}
		for _, index := range ranked {
			kept[index] = true
		}
	}

	result := make([]TagRelationEdge, 0, len(edges))
	for index, edge := range edges {
		if kept[index] {
			result = append(result, edge)
		}
	}
	return result
}

func sortedKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
