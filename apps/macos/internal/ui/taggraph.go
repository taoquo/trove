package ui

import (
	"context"
	"math"
	"sort"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/apps/macos/internal/domain"
	"github.com/taoquo/trove/apps/macos/internal/i18n"
)

// The tag relation graph, drawn with the painter rather than laid out from elements.
//
// The source draws SVG. An edge is a line between two arbitrary points, which is not
// something a box layout can express, so this uses Element.Draw and the painter's
// Line/Fill/Text -- the closest native equivalent. Everything below the drawing is the
// already-verified view model: the positions, radii and limits all come from
// BuildGraphViewModel, which is checked against the TypeScript.

// GraphCanvasTokens are the graph's colours, resolved from the theme.
//
// The stylesheet expresses these as color-mix and rgba over the surface tokens. The
// painter takes a colour with an alpha channel, so they are kept as alpha rather than
// flattened: edges cross one another and the canvas, and a flattened edge would
// occlude what is behind it.
type GraphCanvasTokens struct {
	// EdgeCooccurrence, EdgeHierarchy and EdgeAuthorBridge are the three edge types.
	EdgeCooccurrence ui.Color
	EdgeHierarchy    ui.Color
	EdgeAuthorBridge ui.Color
	// EdgeDimmed multiplies an edge that is outside the focused neighbourhood.
	EdgeDimmed float32
	// NodeFill and NodeChildFill distinguish a root theme from its children.
	NodeFill      ui.Color
	NodeChildFill ui.Color
	// NodeStroke is the ring that separates overlapping dots.
	NodeStroke ui.Color
	// NodeFocusedFill and NodeFocusedStroke mark the focused tag.
	NodeFocusedFill   ui.Color
	NodeFocusedStroke ui.Color
	// NodeDimmed multiplies a node outside the focused neighbourhood.
	NodeDimmed float32
	// Label and LabelFocused are the node captions.
	Label        ui.Color
	LabelFocused ui.Color
	// AuthorFill, AuthorStroke and AuthorLabel are the outer ring.
	AuthorFill   ui.Color
	AuthorStroke ui.Color
	AuthorLabel  ui.Color
	// AuthorDimmed multiplies an author outside the focused set.
	AuthorDimmed float32
	// CanvasFill and CanvasBorder are the plotting surface.
	CanvasFill   ui.Color
	CanvasBorder ui.Color
}

// Graph drawing constants, from extension.css.
const (
	// GraphEdgeWidth is the stroke of a relation. The stylesheet leaves the SVG
	// default, which the painter has no equivalent of, so it is set explicitly.
	GraphEdgeWidth = 1.5
	// GraphNodeStrokeWidth is the ring around a node dot.
	GraphNodeStrokeWidth = 2
	// GraphFocusedStrokeWidth is the thicker ring on the focused node.
	GraphFocusedStrokeWidth = 5
	// GraphAuthorStrokeWidth is the ring around an author dot.
	GraphAuthorStrokeWidth = 2

	// GraphNodeLabelSize is 15px in the stylesheet, and 16 here: the extension draws its SVG
	// 1000 units wide in a 900-point pane, so a 15-unit label lands at 13.5 points, while this
	// canvas is drawn at its own scale. The reader's report was "字也小"; one point is what
	// brings the label back to the size the source means (S57).
	GraphNodeLabelSize = 16
	// GraphAuthorLabelSize is 12px.
	GraphAuthorLabelSize = 12
	// GraphLabelGap is the six units between a dot's rim and its caption. The source puts the
	// caption above the dot (TagGraphView.tsx:186) and so does this painter: a caption under
	// a dot covers the row of dots below it, and the eye reads a name above its mark.
	GraphLabelGap = 6
	// GraphEdgeWidthBase and GraphEdgeWidthScale are the source's stroke: 1 + weight * 4
	// (TagGraphView.tsx:126). A line that does not get heavier with the relation says every
	// pair is equally related, which is the one thing a relation map is for.
	GraphEdgeWidthBase  = 1
	GraphEdgeWidthScale = 4
	// GraphEdgeRestAlpha multiplies every edge's alpha while no tag is focused. The source
	// draws them all at the stylesheet's 34%; at 189 relations that is a mesh, and the reader's
	// "很乱" is what it looks like. Half is enough to see the shape of the web and little
	// enough to read the names over it.
	GraphEdgeRestAlpha = 0.5

	// The dash patterns for the two weaker edge types. The painter has no dashed
	// line, so a dash is drawn as a run of short segments.
	GraphAuthorBridgeDash = 6
)

// GraphCanvasTokensFor resolves the graph's colours.
// GraphCanvasTokensFor resolves the graph's colours. The accent comes in rather than being read
// from a token: the goal keeps interactive colours on the system accent and cinnabar in one
// place, and this was the second copy of it (round 284; StatusBadge was the first).
func GraphCanvasTokensFor(appearance Appearance, systemDark bool, accent ui.Color) GraphCanvasTokens {
	dark := IsDark(appearance, systemDark)
	rail := Rail(appearance, systemDark, false)

	textSecondary := rail.TextTertiary
	textTertiary := rail.TextQuaternary
	surfaceMuted := rail.Muted
	panel := rail.PanelStrong

	if !dark {
		textSecondary = lightTextMuted
		textTertiary = lightTextTertiary
	}

	return GraphCanvasTokens{
		// color-mix(in srgb, var(--text-secondary) 34%, transparent).
		EdgeCooccurrence: ui.RGBA(textSecondary.R, textSecondary.G, textSecondary.B, 0.34),
		// color-mix(in srgb, var(--accent-primary) 52%, transparent).
		EdgeHierarchy: ui.RGBA(accent.R, accent.G, accent.B, 0.52),
		// color-mix(in srgb, var(--text-tertiary) 46%, transparent).
		EdgeAuthorBridge: ui.RGBA(textTertiary.R, textTertiary.G, textTertiary.B, 0.46),
		EdgeDimmed:       0.16,

		// color-mix(in srgb, var(--accent-primary) 70%, transparent).
		NodeFill: ui.RGBA(accent.R, accent.G, accent.B, 0.70),
		// .is-child: color-mix(in srgb, var(--accent-primary) 38%, transparent).
		NodeChildFill: ui.RGBA(accent.R, accent.G, accent.B, 0.38),
		NodeStroke:    panel,

		NodeFocusedFill: accent,
		// color-mix(in srgb, var(--accent-primary) 30%, var(--theme-panel-bg)).
		NodeFocusedStroke: ui.RGBA(accent.R, accent.G, accent.B, 0.30),
		NodeDimmed:        0.24,

		Label:        textSecondary,
		LabelFocused: rail.TextTertiary,

		// color-mix(in srgb, var(--surface-muted) 40%, var(--theme-panel-bg)).
		AuthorFill: ui.RGBA(surfaceMuted.R, surfaceMuted.G, surfaceMuted.B, 0.40),
		// color-mix(in srgb, var(--text-secondary) 48%, transparent).
		AuthorStroke: ui.RGBA(textSecondary.R, textSecondary.G, textSecondary.B, 0.48),
		AuthorLabel:  textTertiary,
		AuthorDimmed: 0.2,

		CanvasFill: ui.RGBA(surfaceMuted.R, surfaceMuted.G, surfaceMuted.B, 0.55),
		// color-mix(in srgb, var(--theme-panel-border) 70%, transparent).
		CanvasBorder: ui.RGBA(rail.RailLine.R, rail.RailLine.G, rail.RailLine.B, 0.70),
	}
}

// GraphView draws the relation graph, and takes a click on a node.
//
// focusedTagID lights one tag and its neighbours; everything else is dimmed rather
// than removed, so the shape of the graph stays legible while the eye is guided.
//
// The positions are the ones graphlayout.go computed with the model, not the view model's
// ring: the ring is what the source draws, and at a real library's size it puts every
// caption on top of the next. The captions are then placed one at a time, in the order the
// model ranked them, and one that would land on a caption already drawn -- or on a dot --
// is left out rather than drawn through it: a name half over another name says less than no
// name, and the focused tag's caption is always drawn.
//
// A node is a control. The whole canvas is one, because the painter has no per-dot element
// to give a click to; the click finds the dot or caption under the pointer and filters the
// list by that tag, which is the one thing a reader wants from a map of their own tags.
func (s *Shell) GraphView(c *ui.Context, model domain.GraphViewModel, focusedTagID string) ui.Element {
	tokens := GraphCanvasTokensFor(s.state.Appearance, s.state.SystemDark, c.Theme().Accent)

	neighbours := map[string]bool{}
	if focusedTagID != "" {
		neighbours = domain.GetGraphNeighbourTagIDs(model.Edges, focusedTagID)
	}

	// Author handles connected to the focused tag. With no focus the whole ring would
	// be noise rather than explanation, which is the reason the source only lights
	// them when something is focused.
	focusedAuthorHandles := map[string]bool{}
	if focusedTagID != "" {
		for _, edge := range model.AuthorEdges {
			if edge.TagID == focusedTagID {
				focusedAuthorHandles[edge.AuthorHandle] = true
			}
		}
	}

	points := s.graphPoints(model)

	canvas := ui.Box(c).Fill().Focusable().Label(s.t().T(i18n.GraphCanvas))
	canvas.OnClick(func() {
		px, py, over := canvas.PointerPosition()
		if !over {
			return
		}
		// The pointer is reported in the canvas's own coordinates; the plot is worked out
		// from its size, so the rect starts at the origin here.
		bounds := canvas.Bounds()
		if tagID := graphNodeAt(ui.Rect{W: bounds.W, H: bounds.H}, px, py, points); tagID != "" {
			s.toggleTagFilter(context.Background(), tagID)
		}
	})

	return canvas.Draw(func(p *ui.Painter, r ui.Rect) {
		p.Fill(r, tokens.CanvasFill, RadiusPanel)
		p.Stroke(r, tokens.CanvasBorder, RadiusPanel, 1)

		if len(model.Nodes) == 0 {
			return
		}

		originX, originY, plotW, plotH := graphFrame(r)
		if plotW <= 0 || plotH <= 0 {
			return
		}

		// The unit square is stretched to the plot on each axis: the page is a wide pane, and
		// keeping the map square left a third of it as empty canvas on either side. The layout
		// separated its captions in a square, and stretching the x axis only ever adds room
		// along the axis captions are widest in.
		toCanvas := func(x, y float32) (float32, float32) {
			return originX + x*plotW, originY + y*plotH
		}

		positions := make(map[string][2]float32, len(points))
		for _, point := range points {
			x, y := toCanvas(point.X, point.Y)
			positions[point.TagID] = [2]float32{x, y}
		}

		// Edges first, so the dots sit on top of them.
		for _, edge := range model.Edges {
			source, sourceKnown := positions[edge.SourceTagID]
			target, targetKnown := positions[edge.TargetTagID]
			if !sourceKnown || !targetKnown {
				continue
			}

			active := focusedTagID == "" ||
				edge.SourceTagID == focusedTagID || edge.TargetTagID == focusedTagID

			color := tokens.EdgeCooccurrence
			dash := float32(0)
			switch edge.Type {
			case domain.EdgeHierarchy:
				color = tokens.EdgeHierarchy
			case domain.EdgeAuthorBridge:
				color = tokens.EdgeAuthorBridge
				dash = GraphAuthorBridgeDash
			}
			if !active {
				color = ui.RGBA(color.R, color.G, color.B, float32(color.A)/255*tokens.EdgeDimmed)
			} else if focusedTagID == "" {
				// At rest every relation is drawn, and 189 of them at the stylesheet's 34% is a
				// grey mesh over the dots. They keep the stylesheet's weight and lose half
				// their alpha, so the picture is dots and names until a tag is picked, and
				// then that tag's relations are the ones at full strength (S57).
				color = dimColor(color, GraphEdgeRestAlpha)
			}

			// The source's stroke, 1 + weight * 4 (TagGraphView.tsx:126): the line carries how
			// often the two tags come together, which is half of what the picture says.
			width := float32(GraphEdgeWidthBase) + float32(edge.Weight)*GraphEdgeWidthScale
			if dash > 0 {
				drawDashedLine(p, source[0], source[1], target[0], target[1], width, dash, color)
			} else {
				p.Line(source[0], source[1], target[0], target[1], width, color)
			}
		}

		// Author ring, drawn under the tags so a tag dot always wins an overlap.
		for _, author := range model.Authors {
			x, y := toCanvas(float32(author.X), float32(author.Y))
			radius := float32(author.Radius)

			fill, stroke := tokens.AuthorFill, tokens.AuthorStroke
			if focusedTagID != "" && !focusedAuthorHandles[author.AuthorHandle] {
				fill = dimColor(fill, tokens.AuthorDimmed)
				stroke = dimColor(stroke, tokens.AuthorDimmed)
			}

			p.Fill(ui.Rect{X: x - radius, Y: y - radius, W: radius * 2, H: radius * 2}, fill, radius)
			p.Stroke(ui.Rect{X: x - radius, Y: y - radius, W: radius * 2, H: radius * 2},
				stroke, radius, GraphAuthorStrokeWidth)
		}

		focused := -1
		for i := range points {
			if points[i].TagID == focusedTagID {
				focused = i
			}
		}

		for i := range points {
			point := points[i]
			position, known := positions[point.TagID]
			if !known {
				continue
			}

			radius := point.Radius
			dimmed := dimmedFor(point.TagID, focusedTagID, neighbours)

			fill := tokens.NodeFill
			if model.Nodes[i].Depth == 1 {
				fill = tokens.NodeChildFill
			}
			// The long tail is quieter than the named head: sixty dots at one weight is a
			// texture, and the page is about the twenty that carry a name (S57). A node the
			// reader is looking at -- focused, or a neighbour of it -- keeps its full weight.
			if !point.Labelled && !dimmed && point.TagID != focusedTagID {
				fill = tokens.NodeChildFill
			}
			stroke := tokens.NodeStroke
			strokeWidth := float32(GraphNodeStrokeWidth)

			if point.TagID == focusedTagID {
				fill = tokens.NodeFocusedFill
				stroke = tokens.NodeFocusedStroke
				strokeWidth = GraphFocusedStrokeWidth
			}
			if dimmed {
				fill = dimColor(fill, tokens.NodeDimmed)
				stroke = dimColor(stroke, tokens.NodeDimmed)
			}

			box := ui.Rect{X: position[0] - radius, Y: position[1] - radius, W: radius * 2, H: radius * 2}
			p.Fill(box, fill, radius)
			p.Stroke(box, stroke, radius, strokeWidth)
		}

		// Where the tag captions go, decided before the author layer draws its own: an author's
		// handle is placed against them, so the two layers' names do not land on one another.
		captions := graphCaptionIndexes(points, positions, focused, neighbours, r)
		placed := make([]ui.Rect, 0, len(captions)+len(model.Authors))
		for _, i := range captions {
			if position, known := positions[points[i].TagID]; known {
				placed = append(placed, graphLabelBox(points[i], position))
			}
		}

		// The author captions, under their ring, six points off it (TagGraphView.tsx:210-215).
		// An author layer whose rings carry no names is a set of anonymous circles around the
		// tags -- the reader turned the layer on to see who writes about what.
		for _, author := range graphAuthorLabels(r, model, toCanvas, placed) {
			color := tokens.AuthorLabel
			if focusedTagID != "" && !focusedAuthorHandles[author.Handle] {
				color = dimColor(color, tokens.AuthorDimmed)
			}
			p.Fill(author.Box, tokens.CanvasFill, 3)
			p.Text(author.Box.X+3, author.Box.Y+1, "@"+author.Handle, GraphAuthorLabelSize, color)
			placed = append(placed, author.Box)
		}

		// The captions go over the dots, on their own plates.
		for _, i := range captions {
			point := points[i]
			position, known := positions[point.TagID]
			if !known {
				continue
			}
			s.drawGraphLabel(p, model, tokens, i, point, position, focusedTagID,
				dimmedFor(point.TagID, focusedTagID, neighbours))
		}
	})
}

// graphAuthorLabel is one author's handle and the plate it goes on.
type graphAuthorLabel struct {
	Handle string
	Box    ui.Rect
}

// graphAuthorLabels places each author ring's handle under it.
//
// The source puts the handle at `cy + r + 12` at 12px (TagGraphView.tsx:210-215), and draws it
// unconditionally. Here a handle that would be cut by the plot's edge, or that would land on a
// tag caption already accepted, is left out instead: a handle drawn over a tag's name costs the
// reader both names, and the ring is still there to point at. The list is also where the
// painter's rule lives, so a unit test can check the placement without a window.
func graphAuthorLabels(plot ui.Rect, model domain.GraphViewModel,
	toCanvas func(x, y float32) (float32, float32), placed []ui.Rect) []graphAuthorLabel {
	labels := make([]graphAuthorLabel, 0, len(model.Authors))
	for _, author := range model.Authors {
		if author.AuthorHandle == "" {
			continue
		}
		x, y := toCanvas(float32(author.X), float32(author.Y))
		label := "@" + author.AuthorHandle
		width := graphTextWidth(label, GraphAuthorLabelSize)
		box := ui.Rect{
			X: x - width/2 - 3,
			Y: y + float32(author.Radius) + GraphLabelGap,
			W: width + 6,
			H: GraphAuthorLabelSize * 1.2,
		}
		if !graphBoxInside(box, plot) || graphOverlaps(box, placed) {
			continue
		}
		labels = append(labels, graphAuthorLabel{Handle: author.AuthorHandle, Box: box})
	}
	return labels
}

// graphCaptionIndexes decides which captions are drawn, in drawing order.
//
// Only the head of the model's ranking is named -- GraphCaptionLimit of it -- because sixty
// names at once is a wall of text with no shape in it, and the graph's own panel names
// whatever is clicked. Those captions are then placed one at a time against the captions
// already accepted, so the heaviest tags keep their names when two would collide; a name
// drawn through another name says less than no name. The focused tag's caption is always
// drawn, and last, on top: it is the one the reader asked for.
//
// A caption may cross a dot: the captions are drawn over the dots and carry a plate of the
// canvas's colour (drawGraphLabel), which reads as a label laid over the map. Requiring a
// caption to miss every dot cost 46 of 60 names on a real library -- and the dots are what
// the names are about.
func graphCaptionIndexes(points []graphPoint, positions map[string][2]float32,
	focused int, neighbours map[string]bool, plot ui.Rect) []int {
	placed := make([]ui.Rect, 0, len(points))
	drawn := make([]int, 0, len(points))

	// With a focus the named set changes: the focused tag and its neighbours, and nothing
	// else. The rest were drawn too, at the dimming the source gives a node outside the
	// neighbourhood (0.24 opacity) -- twenty ghost captions around one legible name, which is
	// what made the page read as noise. A name nobody can read is not a name, and the tag is
	// still a dot to point at.
	named := func(i int) bool {
		if focused < 0 {
			// The model ranks by weight, so the head is what the library is mostly about.
			return points[i].Labelled
		}
		return neighbours[points[i].TagID]
	}

	for i := range points {
		if i == focused || !named(i) {
			continue
		}
		position, known := positions[points[i].TagID]
		if !known {
			continue
		}
		box := graphLabelBox(points[i], position)
		// A caption at the plot's edge would be cut by the canvas; it is the one thing the
		// layout cannot know about, because it depends on the canvas the page is given.
		if !graphBoxInside(box, plot) {
			continue
		}
		if graphOverlaps(box, placed) {
			continue
		}
		placed = append(placed, box)
		drawn = append(drawn, i)
	}
	if focused >= 0 {
		if _, known := positions[points[focused].TagID]; known {
			drawn = append(drawn, focused)
		}
	}
	return drawn
}

// graphBoxInside reports whether a caption's plate is wholly inside the plot.
func graphBoxInside(box, plot ui.Rect) bool {
	return box.X >= plot.X && box.Y >= plot.Y &&
		box.X+box.W <= plot.X+plot.W && box.Y+box.H <= plot.Y+plot.H
}

// drawGraphLabel draws one node's caption under its dot.
//
// The caption sits on a plate of the canvas's own colour. The dots are drawn first, so a
// caption that crosses one is legible instead of being struck through by it, and the plate
// is what makes that read as a label laid over the map rather than as a hole in the dot.
func (s *Shell) drawGraphLabel(p *ui.Painter, model domain.GraphViewModel, tokens GraphCanvasTokens,
	i int, point graphPoint, position [2]float32, focusedTagID string, dimmed bool) {
	labelColor := tokens.Label
	if point.TagID == focusedTagID {
		labelColor = tokens.LabelFocused
	}
	if dimmed {
		labelColor = dimColor(labelColor, tokens.NodeDimmed)
	}
	box := graphLabelBox(point, position)
	p.Fill(box, tokens.CanvasFill, 3)
	p.Text(position[0]-point.LabelW/2, box.Y+1, model.Nodes[i].Name, GraphNodeLabelSize, labelColor)
}

// GraphCaptionLimit is how many captions the graph names: the head of the model's ranking,
// which is its order by weight.
const GraphCaptionLimit = 20

// dimmedFor reports whether a node is outside the focused neighbourhood.
func dimmedFor(tagID, focusedTagID string, neighbours map[string]bool) bool {
	return focusedTagID != "" && tagID != focusedTagID && !neighbours[tagID]
}

// graphLabelBox is the rectangle a node's caption occupies: centred above its dot, six points
// off the rim (TagGraphView.tsx:186).
func graphLabelBox(point graphPoint, position [2]float32) ui.Rect {
	return ui.Rect{
		X: position[0] - point.LabelW/2 - 3,
		Y: position[1] - point.Radius - GraphLabelGap - point.LabelH,
		W: point.LabelW + 6,
		H: point.LabelH,
	}
}

// graphOverlaps reports whether a box lands on any of the others.
func graphOverlaps(box ui.Rect, boxes []ui.Rect) bool {
	for _, other := range boxes {
		if box.X < other.X+other.W && other.X < box.X+box.W &&
			box.Y < other.Y+other.H && other.Y < box.Y+box.H {
			return true
		}
	}
	return false
}

// graphFrame is the plot inside the canvas: the canvas inset by GraphCanvasPadding, on each
// axis of its own.
//
// It used to be the largest square that fits, centred. On a page whose pane is twice as wide as
// it is tall, that square left a third of the canvas empty on either side -- a map drawn in the
// middle of a pale rectangle, which is part of why the page said nothing.
func graphFrame(r ui.Rect) (originX, originY, width, height float32) {
	return r.X + GraphCanvasPadding, r.Y + GraphCanvasPadding,
		max(r.W-2*GraphCanvasPadding, 0), max(r.H-2*GraphCanvasPadding, 0)
}

// graphNodeAt is the node under a point in the canvas, or "".
//
// The caption counts as the node: a 6-point dot is a hard target, and the word under it is
// what the reader is aiming at. The nearest hit wins, so an overlap resolves the way the
// eye does.
func graphNodeAt(bounds ui.Rect, px, py float32, points []graphPoint) string {
	originX, originY, plotW, plotH := graphFrame(bounds)
	if plotW <= 0 || plotH <= 0 {
		return ""
	}

	best := ""
	bestDistance := float32(0)
	for _, point := range points {
		x := originX + point.X*plotW
		y := originY + point.Y*plotH

		dx, dy := px-x, py-y
		distance := float32(math.Hypot(float64(dx), float64(dy)))
		if reach := point.Radius + GraphNodeHitPadding; distance <= reach {
			if best == "" || distance < bestDistance {
				best, bestDistance = point.TagID, distance
			}
			continue
		}
		label := graphLabelBox(point, [2]float32{x, y})
		if px >= label.X && px <= label.X+label.W && py >= label.Y && py <= label.Y+label.H {
			// A caption hit is measured from its middle, so the nearest caption still wins.
			distance = float32(math.Hypot(float64(px-(label.X+label.W/2)), float64(py-(label.Y+label.H/2))))
			if best == "" || distance < bestDistance {
				best, bestDistance = point.TagID, distance
			}
		}
	}
	return best
}

// GraphNodeHitPadding widens a dot's target by this much: the dot is 6 to 20 points across,
// which is small for a pointer, and the caption under it is the rest of the target.
const GraphNodeHitPadding = 4

// graphPoints is where the painter puts the model's nodes.
//
// The shell computes these with the model (graphlayout.go). A model the shell was handed
// without one -- a test, a capture -- still draws: the view model's own ring is a picture,
// just not one that holds a real library's captions.
func (s *Shell) graphPoints(model domain.GraphViewModel) []graphPoint {
	if len(s.state.GraphLayout) == len(model.Nodes) {
		return s.state.GraphLayout
	}
	points := make([]graphPoint, len(model.Nodes))
	for i, node := range model.Nodes {
		points[i] = graphPoint{
			TagID:    node.TagID,
			X:        float32(node.X),
			Y:        float32(node.Y),
			Radius:   float32(node.Radius),
			LabelW:   graphLabelWidth(node.Name),
			LabelH:   GraphNodeLabelSize * 1.2,
			Labelled: i < GraphCaptionLimit,
		}
	}
	return points
}

// GraphCanvasPadding keeps a node at the edge of the unit square inside the canvas.
const GraphCanvasPadding = 24

// drawDashedLine draws a line as a run of segments.
//
// The painter has no dash support and the SVG's stroke-dasharray has no equivalent, so
// the pattern is reproduced by walking the line.
func drawDashedLine(p *ui.Painter, x0, y0, x1, y1, width, dash float32, c ui.Color) {
	dx, dy := x1-x0, y1-y0
	length := float32(math.Hypot(float64(dx), float64(dy)))
	if length <= 0 {
		return
	}

	ux, uy := dx/length, dy/length
	// Equal on and off runs, which is what stroke-dasharray "6 6" means.
	for travelled := float32(0); travelled < length; travelled += dash * 2 {
		end := min(travelled+dash, length)
		p.Line(x0+ux*travelled, y0+uy*travelled, x0+ux*end, y0+uy*end, width, c)
	}
}

// dimColor multiplies a colour's alpha.
func dimColor(c ui.Color, factor float32) ui.Color {
	alpha := float32(c.A) / 255 * factor
	if alpha < 0 {
		alpha = 0
	}
	if alpha > 1 {
		alpha = 1
	}
	return ui.RGBA(c.R, c.G, c.B, alpha)
}

// sortedGraphTags is the node order the graph draws in, exposed so a test can assert
// the ranking rather than the drawing.
func sortedGraphTags(model domain.GraphViewModel) []string {
	tags := make([]string, 0, len(model.Nodes))
	for _, node := range model.Nodes {
		tags = append(tags, node.TagID)
	}
	sort.Strings(tags)
	return tags
}
