package ui

import (
	"context"
	"fmt"
	"math"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/x-master/apps/macos/internal/domain"
	"github.com/taoquo/x-master/apps/macos/internal/i18n"
)

// The relation graph at the size a real library gives it.
//
// The reader's report of 2026-11-10: "这个标签图的这个功能几乎不能用". It was drawn from the
// view model's ring -- every tag on one circle at 0.21 of the canvas -- which is what the
// reference does and what its own screenshot, with a handful of tags, can carry. Sixty tags
// put every caption on the one beside it.

// ringGraph is the graph the view model draws: n tags round one circle, with relations
// between neighbours and a few across.
func ringGraph(n int) domain.GraphViewModel {
	names := []string{
		"ai agent", "ai coding", "3d modeling", "artificial intelligence", "machine learning",
		"product design", "video generation", "image generation", "web development",
		"personal finance", "knowledge management", "content creation", "open source",
		"computer vision", "language learning", "data visualization", "note taking",
	}
	model := domain.GraphViewModel{Nodes: make([]domain.GraphViewNode, 0, n)}
	for i := 0; i < n; i++ {
		angle := float64(i) / float64(n) * math.Pi * 2
		name := fmt.Sprintf("%s %d", names[i%len(names)], i)
		model.Nodes = append(model.Nodes, domain.GraphViewNode{
			TagRelationNode: domain.TagRelationNode{
				TagID:         fmt.Sprintf("t%d", i),
				Name:          name,
				BookmarkCount: n - i,
			},
			// assignGraphLayout's own numbers: 0.42 of the square from the middle, a radius
			// from 6 to 20.
			X:      0.5 + math.Cos(angle)*0.42*0.5,
			Y:      0.5 + math.Sin(angle)*0.42*0.5,
			Radius: 6 + math.Sqrt(float64(n-i)/float64(n))*14,
		})
	}
	// About six relations a node, which is what the 60-tag library this was drawn against
	// had (189 links). A plain ring is its own picture -- a cycle has to come out a circle --
	// and would say nothing about how a real library lands.
	// Irregular partners, from a fixed sequence rather than a stride: a regular graph lays
	// itself out on a lattice whatever the forces do, which says nothing about a library.
	state := uint32(20261110)
	next := func() int {
		state = state*1664525 + 1013904223
		return int(state>>8) % n
	}
	for i := 0; i < n; i++ {
		for edge := 0; edge < 6; edge++ {
			target := next()
			if target == i {
				continue
			}
			model.Edges = append(model.Edges, domain.TagRelationEdge{
				SourceTagID: fmt.Sprintf("t%d", i),
				TargetTagID: fmt.Sprintf("t%d", target),
				Type:        domain.EdgeCooccurrence,
			})
		}
	}
	return model
}

// captionsThatFit is how many captions survive placement on a canvas of that size.
func captionsThatFit(points []graphPoint, canvas float32) int {
	positions := make(map[string][2]float32, len(points))
	for _, point := range points {
		positions[point.TagID] = [2]float32{point.X * canvas, point.Y * canvas}
	}
	return len(graphCaptionIndexes(points, positions, -1, nil,
		ui.Rect{W: canvas, H: canvas}))
}

// TestTheGraphFindsRoomForItsCaptions covers the placement.
//
// Two things have to be true for the page to be worth opening: the nodes have to use the
// canvas rather than huddle in its middle, and most of the captions it means to draw have to
// be drawn. The source's ring fails the second one at this size -- which is why this build
// lays the graph out itself (graphlayout.go).
func TestTheGraphFindsRoomForItsCaptions(t *testing.T) {
	const n = 60
	model := ringGraph(n)

	points := layoutGraph(model)
	if len(points) != n {
		t.Fatalf("laid out %d of %d nodes", len(points), n)
	}

	// The picture fills the plot instead of sitting in a ball at its centre, and nothing is
	// left outside it.
	minX, maxX, minY, maxY := float32(1), float32(0), float32(1), float32(0)
	for _, point := range points {
		minX, maxX = min(minX, point.X), max(maxX, point.X)
		minY, maxY = min(minY, point.Y), max(maxY, point.Y)
	}
	if minX < 0 || minY < 0 || maxX > 1 || maxY > 1 {
		t.Errorf("nodes reach outside the unit square: x %.2f..%.2f, y %.2f..%.2f", minX, maxX, minY, maxY)
	}
	if span := max(maxX-minX, maxY-minY); span < 0.8 {
		t.Errorf("the nodes span %.2f of the canvas: they are huddled in its middle", span)
	}

	laidOut := captionsThatFit(points, 640)

	// The same graph in the view model's own ring, for the comparison that made this page
	// unreadable: the ring's positions, with the caption sizes the painter would give them.
	ringPoints := make([]graphPoint, 0, n)
	for _, node := range model.Nodes {
		ringPoints = append(ringPoints, graphPoint{
			TagID:  node.TagID,
			X:      float32(node.X),
			Y:      float32(node.Y),
			Radius: float32(node.Radius),
			LabelW: graphLabelWidth(node.Name),
			LabelH: GraphNodeLabelSize * 1.2,
		})
	}
	onTheRing := captionsThatFit(ringPoints, 640)

	// The page names the head of the ranking (GraphCaptionLimit) and no more; most of those
	// have to survive placement, or the page is a map without names on it.
	if want := GraphCaptionLimit * 3 / 5; laidOut < want {
		t.Errorf("%d captions fit on the canvas, of the %d the page names: it wants at least %d",
			laidOut, GraphCaptionLimit, want)
	}
	// The old picture was drawn from the ring, so this is the bar it has to fail: if a future
	// change to the source's arithmetic made the ring legible, this test would be asserting
	// a difference that no longer means anything.
	if onTheRing >= GraphCaptionLimit*3/5 {
		t.Fatalf("the source's ring already fits %d captions, so the comparison proves nothing", onTheRing)
	}
	if laidOut <= onTheRing {
		t.Errorf("%d captions fit after the layout and %d on the source's ring: the layout is not helping",
			laidOut, onTheRing)
	}
}

// TestClickingAGraphNodeFiltersByThatTag covers the node's one action.
//
// The graph drew its nodes but took nothing back: a dot was a picture, and the only way to
// reach what it stood for was to find the tag in the sidebar. The canvas is one control
// because the painter has no per-dot element, so the click is hit-tested against the same
// placement the dots were drawn with.
func TestClickingAGraphNodeFiltersByThatTag(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "a post", clockBase)
	// The library's own tags: a condition naming one the read cannot find is dropped by the
	// reload (applyReload, reload.go:146-158), which is right and would hide the click.
	seedTagOnly(t, db, "t0", "golang")
	seedTagOnly(t, db, "t1", "rust")
	seedTagOnly(t, db, "t2", "zig")
	shell.Reload(context.Background())

	model := ringGraph(3)
	shell.state.GraphModel = model
	shell.state.GraphLayout = layoutGraph(model)
	shell.state.GraphMode = true

	tester := ui.NewTester(shell.View, 900, 900)
	tester.Frame()

	canvas, ok := tester.Find(shell.t().T(i18n.GraphCanvas))
	if !ok {
		t.Fatalf("the graph's canvas is not drawn; texts: %q", tester.Texts())
	}
	point := shell.state.GraphLayout[0]
	originX, originY, plotW, plotH := graphFrame(canvas)
	// A click arrives after the pointer: the tester has to move it there first, because the
	// canvas asks where the pointer is when it is clicked.
	tester.Move(originX+point.X*plotW, originY+point.Y*plotH)
	tester.Frame()
	tester.ClickAt(originX+point.X*plotW, originY+point.Y*plotH)

	if len(shell.state.ActiveTagIDs) != 1 || shell.state.ActiveTagIDs[0] != point.TagID {
		t.Fatalf("clicking %q left the conditions at %v", point.TagID, shell.state.ActiveTagIDs)
	}
}
