package ui

import (
	"math"

	"github.com/taoquo/trove/apps/macos/internal/domain"
)

// The relation graph's placement, which the source's ring cannot give it.
//
// graphModel.ts puts every tag on one circle at 0.21 of the canvas and every child on a
// second at 0.41 (assignGraphLayout, src/options/graph/graphModel.ts). That is legible for
// the handful of tags its own screenshot holds; a real library is not. With the 60 tags
// this library had on 2026-11-10, the inner ring is 923 points around at the widest canvas
// the window offers, or 15 points per node, while the captions under them are 60 to 120
// points wide: every label landed on the one beside it and the graph read as a ball of grey
// with a ring of dots around it. The reader's verdict was "this feature is unusable".
//
// So the positions are laid out here instead, and the view model keeps the source's own
// arithmetic for the parity test (domain/taggraphview.go). The inputs are still the source's:
// the rank order decides which nodes survive the limit, and the ring's positions are where
// the simulation starts -- children outside their themes, as the source draws them -- so a
// small graph settles near the shape the reference has.
//
// The simulation is Fruchterman-Reingold with a caption-sized separation term: repulsion
// between every pair, a spring along every relation, a pull toward the middle, and a hard
// push when two nodes' boxes overlap. It is deterministic -- no randomness, a fixed
// iteration count and a fixed order -- because a graph that rearranged itself between
// frames would be a worse map than a ball of grey.

const (
	// graphLayoutIterations is how many steps the simulation takes. 500 is enough for the
	// 60-node library to stop moving (the last steps' displacement is under a point at any
	// canvas size the column allows) and still costs a millisecond or two off the window's
	// thread, where it runs: graphLayout is called once per build, beside the model.
	graphLayoutIterations = 500
	// graphLayoutCanvas is the canvas the unit square stands for, in points. It is only used
	// to turn a caption's point width into a share of the square; the painter scales the
	// result to the canvas it actually has.
	graphLayoutCanvas = 640
	// graphLayoutStart is the first step's temperature in the unit square, and the ceiling on
	// any single step. It cools to zero over the run.
	graphLayoutStart = 0.09
	// graphLayoutMargin keeps a node's centre off the canvas edge by this share, so a caption
	// that runs past its dot stays inside the plot.
	graphLayoutMargin = 0.06
	// graphLayoutGravity pulls a node toward the middle. Without it a graph of few nodes
	// spreads to the corners, which reads as scattered rather than as one picture; too little
	// of it leaves the weakly-related tail stranded at the edges, where a handful of pale dots
	// set the picture's bounds and shrink the part with the names in it to the middle.
	graphLayoutGravity = 0.16
	// graphLayoutSeedScale pulls the source's ring in toward the middle before the
	// simulation starts.
	graphLayoutSeedScale = 0.35
	// graphLayoutSeparation is how hard two overlapping dots push apart, as a multiple of the
	// ordinary repulsion at that distance.
	graphLayoutSeparation = 2
	// graphCaptionPasses bounds the caption relaxation (spreadCaptions). Twenty plates need
	// fewer than a hundred moves to clear on a real library; the bound is what keeps the run
	// deterministic when they cannot all clear.
	graphCaptionPasses = 160
	// graphCaptionNudge is added to every caption push so two plates exactly touching still
	// part, which is what stops the relaxation from settling on a boundary.
	graphCaptionNudge = 0.004
	// graphCaptionAir is the space kept between two plates, in canvas points, on top of their
	// own extents.
	graphCaptionAir = 8
)

// graphPoint is one node's place in the unit square, with the size of the mark and its caption.
//
// The caption's box is what the layout separates: the dots alone never collided -- the labels
// did -- so a placement that only kept dots apart would look unchanged on the page.
//
// Only the head of the ranking is named (GraphCaptionLimit), so only that head reserves a
// caption's box. Reserving one for every node was the first version, and at sixty nodes it
// could not succeed: the boxes alone need more than the canvas has, so the simulation stopped
// separating anything and settled into a ball in the middle, which is the picture the reader
// called a mess. A node that is a dot reserves a dot.
type graphPoint struct {
	TagID  string
	X, Y   float32
	Radius float32
	LabelW float32
	LabelH float32
	// Labelled says whether this node's caption is drawn, and so whether the layout keeps
	// room for it.
	Labelled bool
}

// graphLabelWidth estimates a caption's width, as the painter draws it.
//
// The painter has no text measurement, and the graph's caption is drawn on one line at
// GraphNodeLabelSize; 0.55 of the em is the average advance this file already assumes
// (taggraph.go). A CJK tag is wider than that estimate and a short Latin one narrower, which
// is why the separation below carries a margin rather than trusting the number exactly.
func graphLabelWidth(name string) float32 {
	return graphTextWidth(name, GraphNodeLabelSize)
}

// graphTextWidth estimates a line's width at a size, by the same 0.55 of an em.
func graphTextWidth(text string, size float32) float32 {
	return float32(len([]rune(text))) * size * 0.55
}

// layoutGraph places the model's nodes in the unit square.
//
// It returns nothing for an empty model. The result is indexed like model.Nodes, so the
// painter can read a node's caption size from the same place it reads its position.
func layoutGraph(model domain.GraphViewModel) []graphPoint {
	n := len(model.Nodes)
	if n == 0 {
		return nil
	}

	points := make([]graphPoint, n)
	index := make(map[string]int, n)
	xs := make([]float64, n)
	ys := make([]float64, n)
	for i, node := range model.Nodes {
		index[node.TagID] = i
		points[i] = graphPoint{
			TagID:    node.TagID,
			Radius:   float32(node.Radius),
			LabelW:   graphLabelWidth(node.Name),
			LabelH:   GraphNodeLabelSize * 1.2,
			Labelled: i < GraphCaptionLimit,
		}
		// The source's ring is the starting shape, not the answer, and it starts pulled in
		// toward the middle so the simulation expands it: a ring left at its own radius is a
		// stable shape -- repulsion pushes it out and the relations hold it there.
		xs[i] = 0.5 + (node.X-0.5)*graphLayoutSeedScale
		ys[i] = 0.5 + (node.Y-0.5)*graphLayoutSeedScale
	}

	if n == 1 {
		xs[0], ys[0] = 0.5, 0.5
		points[0].X, points[0].Y = 0.5, 0.5
		return points
	}

	// The ideal distance between two nodes, and the box each one occupies while the network
	// settles.
	//
	// The separation here is the *dot*, not the caption. A caption's plate is three to six
	// times a dot's width, and twenty of them cannot be packed into the square, so making the
	// caption the separation radius meant the overlap term fought the repulsion to a standstill
	// in the middle: that is the ball the reader saw. The captions get their room afterwards,
	// from the twenty nodes that carry one (spreadCaptions), where a push that cannot be
	// satisfied only leaves two names out instead of collapsing the picture.
	ideal := math.Sqrt(1/float64(n)) * 0.95
	halfWidth := make([]float64, n)
	halfHeight := make([]float64, n)
	for i := range points {
		halfWidth[i] = (float64(points[i].Radius) + 3) / graphLayoutCanvas
		halfHeight[i] = (float64(points[i].Radius) + 3) / graphLayoutCanvas
	}

	displacement := make([][2]float64, n)
	for step := 0; step < graphLayoutIterations; step++ {
		for i := range displacement {
			displacement[i] = [2]float64{}
		}

		for i := 0; i < n; i++ {
			for j := i + 1; j < n; j++ {
				dx, dy := xs[i]-xs[j], ys[i]-ys[j]
				distance := math.Hypot(dx, dy)
				if distance < 1e-6 {
					// Two nodes on the same point have no direction to separate along. The
					// nudge is a constant, not a random value, so the run stays reproducible.
					dx, dy, distance = 1e-4, 1e-4, math.Sqrt2*1e-4
				}
				ux, uy := dx/distance, dy/distance

				force := ideal * ideal / distance
				displacement[i][0] += ux * force
				displacement[i][1] += uy * force
				displacement[j][0] -= ux * force
				displacement[j][1] -= uy * force

				// Overlapping boxes push apart along the line between them, as far as the
				// smaller overlap needs. Pushing along an axis instead packed the boxes as
				// neatly and laid the graph out on a lattice: sixty dots in rows with their
				// relations running as straight wires between them, which reads as a table.
				overlapX := halfWidth[i] + halfWidth[j] - math.Abs(dx)
				overlapY := halfHeight[i] + halfHeight[j] - math.Abs(dy)
				if overlapX <= 0 || overlapY <= 0 {
					continue
				}
				push := math.Min(overlapX, overlapY) * graphLayoutSeparation
				displacement[i][0] += ux * push
				displacement[i][1] += uy * push
				displacement[j][0] -= ux * push
				displacement[j][1] -= uy * push
			}
		}

		for _, edge := range model.Edges {
			from, okFrom := index[edge.SourceTagID]
			to, okTo := index[edge.TargetTagID]
			if !okFrom || !okTo || from == to {
				continue
			}
			dx, dy := xs[from]-xs[to], ys[from]-ys[to]
			distance := math.Hypot(dx, dy)
			if distance < 1e-6 {
				continue
			}
			// A relation pulls its two ends together, but never through one another, and never
			// harder than the ideal distance: the square in Fruchterman-Reingold's own formula
			// is what lets a well-connected graph fold itself into a single ball, because at a
			// third of the canvas the pull is a hundred times the repulsion holding it out.
			ux, uy := dx/distance, dy/distance
			force := math.Min(distance*distance/ideal, ideal)
			displacement[from][0] -= ux * force
			displacement[from][1] -= uy * force
			displacement[to][0] += ux * force
			displacement[to][1] += uy * force
		}

		for i := 0; i < n; i++ {
			displacement[i][0] += (0.5 - xs[i]) * graphLayoutGravity
			displacement[i][1] += (0.5 - ys[i]) * graphLayoutGravity
		}

		// Cool the run: the first steps rearrange, the last ones tidy.
		temperature := graphLayoutStart * (1 - float64(step)/float64(graphLayoutIterations))
		for i := 0; i < n; i++ {
			dx, dy := displacement[i][0], displacement[i][1]
			length := math.Hypot(dx, dy)
			if length < 1e-9 {
				continue
			}
			// No clamping inside the plot: a step that pushed against the edge would pile the
			// nodes onto it, and sixty dots on a rectangle's four sides is what the page came
			// out as. The settled shape is fitted to the canvas at the end instead.
			step := math.Min(length, temperature)
			xs[i] += dx / length * step
			ys[i] += dy / length * step
		}
	}

	// The run settles a shape; this fits it to the canvas. The forces only decide the shape,
	// and how large it comes out depends on how many relations the library happens to have --
	// a dense one came out as a ball in the middle of a page of white. Stretching the result
	// to the plot keeps every relative distance and uses the room the page has.
	fillGraph(xs, ys)

	// The names last, and after the fit: a plate's size is fixed in canvas points, so it is
	// only in the fitted coordinates that the room a plate needs is the room it gets.
	spreadCaptions(xs, ys, points)

	for i := range points {
		points[i].X = float32(xs[i])
		points[i].Y = float32(ys[i])
	}
	return points
}

// captionHalf is the half-size of a node's caption plate in the unit square, and whether the
// node carries one at all.
//
// The plate sits above the dot and is wide and short. The vertical extent is measured as if the
// plate were centred on the dot -- a generous circle around a shape that is not symmetric -- so
// two plates that clear each other by this measure always clear each other on the page.
func captionHalf(point graphPoint) (halfW, halfH float64, named bool) {
	if !point.Labelled {
		return 0, 0, false
	}
	// A little air beyond the plate's own edges: two captions that clear by exactly zero read
	// as one block of text, which is the picture this page was fixing.
	halfW = (math.Max(float64(point.LabelW), float64(point.Radius)*2)/2 + graphCaptionAir) / graphLayoutCanvas
	halfH = (float64(point.Radius)+GraphLabelGap+float64(point.LabelH))/2/graphLayoutCanvas +
		graphCaptionAir/graphLayoutCanvas
	return halfW, halfH, true
}

// spreadCaptions gives the named nodes room for their plates.
//
// The network layout separates dots, so the head of the ranking comes out of it clustered like
// the rest -- and the head is exactly what carries names. This relaxes only those nodes, pair by
// pair, along whichever axis needs the shorter move, until the plates clear or the passes run
// out. A pair that still collides after that is left to the declutter in the painter: one name
// short is a name the reader can still reach by pointing at the dot.
func spreadCaptions(xs, ys []float64, points []graphPoint) {
	for pass := 0; pass < graphCaptionPasses; pass++ {
		moved := false
		for i := range points {
			halfWi, halfHi, named := captionHalf(points[i])
			if !named {
				continue
			}
			for j := i + 1; j < len(points); j++ {
				halfWj, halfHj, namedJ := captionHalf(points[j])
				if !namedJ {
					continue
				}
				overlapX := halfWi + halfWj - math.Abs(xs[i]-xs[j])
				overlapY := halfHi + halfHj - math.Abs(ys[i]-ys[j])
				if overlapX <= 0 || overlapY <= 0 {
					continue
				}
				moved = true
				push := math.Min(overlapX, overlapY)/2 + graphCaptionNudge
				if overlapX < overlapY {
					step := push
					if xs[i] < xs[j] {
						step = -push
					}
					xs[i] += step
					xs[j] -= step
					continue
				}
				step := push
				if ys[i] < ys[j] {
					step = -push
				}
				ys[i] += step
				ys[j] -= step
			}
		}
		// Inside the fitted plot: a plate at the canvas edge would be cut, and the position is
		// already as far out as the picture goes.
		for i := range points {
			xs[i] = math.Min(1-graphLayoutMargin, math.Max(graphLayoutMargin, xs[i]))
			ys[i] = math.Min(1-graphLayoutMargin, math.Max(graphLayoutMargin, ys[i]))
		}
		if !moved {
			return
		}
	}
}

// fillGraph scales the settled positions to the plot, on each axis of its own.
//
// One scale for both axes kept the picture's proportions, and a graph that settled wider than
// it was tall then filled the unit square's width and left a band of empty canvas above and
// below -- the same empty band the centred square plot left at the sides. The painter stretches
// the unit square to the canvas on each axis anyway (graphFrame), so the proportions a uniform
// scale would keep are not kept on screen either way. Filling both axes is what uses the canvas.
func fillGraph(xs, ys []float64) {
	minX, maxX := math.Inf(1), math.Inf(-1)
	minY, maxY := math.Inf(1), math.Inf(-1)
	for i := range xs {
		minX, maxX = math.Min(minX, xs[i]), math.Max(maxX, xs[i])
		minY, maxY = math.Min(minY, ys[i]), math.Max(maxY, ys[i])
	}
	width, height := maxX-minX, maxY-minY
	if math.Max(width, height) < 1e-6 {
		// One node, or all of them on a point: put it in the middle.
		for i := range xs {
			xs[i], ys[i] = 0.5, 0.5
		}
		return
	}

	// Both ways: the run has no edges of its own, so it can settle wider than the canvas as
	// easily as narrower, and a picture that kept its own size drew dots past the plot and
	// over the page's header.
	target := 1 - 2*graphLayoutMargin
	scaleX, scaleY := target, target
	if width > 1e-6 {
		scaleX = target / width
	}
	if height > 1e-6 {
		scaleY = target / height
	}
	centreX, centreY := (minX+maxX)/2, (minY+maxY)/2
	for i := range xs {
		xs[i] = 0.5 + (xs[i]-centreX)*scaleX
		ys[i] = 0.5 + (ys[i]-centreY)*scaleY
	}
}
