package ui

import (
	"context"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/x-master/apps/macos/internal/domain"
	"github.com/taoquo/x-master/apps/macos/internal/i18n"
)

// The graph page at the size a reader actually gets, which is what the second report was about.
//
// "是不是很乱？字也小，不知道要体现什么内容，下面的区域也被严重的挤压" -- four complaints, and
// the three tests here are the mechanical half of them: the map uses the canvas it is given,
// the page says what the marks mean, and a focus names only what it lights.

// TestTheGraphUsesTheWholeCanvas covers the empty sides.
//
// The plot was the largest square that fitted the canvas, centred. On a page whose pane is
// twice as wide as it is tall that square is half the pane's width, so the map sat in the
// middle of a pale rectangle with a third of the canvas empty at either side -- part of why
// the page read as saying nothing.
func TestTheGraphUsesTheWholeCanvas(t *testing.T) {
	shell, _ := newShell(t)
	shell.SetLocale(i18n.EN, i18n.EN)
	shell.Reload(context.Background())

	model := ringGraph(60)
	shell.state.GraphModel = model
	shell.state.GraphLayout = layoutGraph(model)
	shell.state.GraphMode = true

	tester := ui.NewTester(shell.View, 1280, 820)
	tester.Frame()

	canvas, ok := tester.Find(shell.t().T(i18n.GraphCanvas))
	if !ok {
		t.Fatalf("the graph's canvas is not drawn; texts: %q", tester.Texts())
	}
	_, _, plotW, plotH := graphFrame(canvas)

	minX, maxX := float32(1), float32(0)
	minY, maxY := float32(1), float32(0)
	for _, point := range shell.state.GraphLayout {
		minX, maxX = min(minX, point.X), max(maxX, point.X)
		minY, maxY = min(minY, point.Y), max(maxY, point.Y)
	}
	spanX := (maxX - minX) * plotW
	spanY := (maxY - minY) * plotH

	if spanX < 0.8*canvas.W {
		t.Errorf("the map spans %.0f of the canvas's %.0f points across: %.0f points of the canvas are empty",
			spanX, canvas.W, canvas.W-spanX)
	}
	if spanY < 0.8*canvas.H {
		t.Errorf("the map spans %.0f of the canvas's %.0f points down: %.0f points of the canvas are empty",
			spanY, canvas.H, canvas.H-spanY)
	}
	if canvas.W <= canvas.H {
		t.Fatalf("the canvas is %.0fx%.0f: this test needs a wider-than-tall canvas to prove anything",
			canvas.W, canvas.H)
	}
}

// TestTheGraphSaysWhatItsMarksMean covers "不知道要体现什么内容".
//
// A dot's size is the tag's bookmarks and a line's weight is how often the two tags come
// together; the source draws both encodings and explains neither. One line under the summary
// is the whole of the fix, and it has to be on the page for the page to be about something.
func TestTheGraphSaysWhatItsMarksMean(t *testing.T) {
	shell, _ := newShell(t)
	shell.SetLocale(i18n.EN, i18n.EN)
	shell.Reload(context.Background())

	model := ringGraph(12)
	shell.state.GraphModel = model
	shell.state.GraphLayout = layoutGraph(model)
	shell.state.GraphMode = true

	tester := ui.NewTester(shell.View, 1280, 820)
	tester.Frame()

	legend := shell.t().T(i18n.GraphLegend)
	rect, ok := tester.Find(legend)
	if !ok {
		t.Fatalf("the graph page does not say what its dots and lines mean; texts: %q", tester.Texts())
	}
	summary, ok := tester.Find(shell.t().Tf(i18n.GraphSummary, len(model.Nodes), len(model.Edges)))
	if !ok {
		t.Fatalf("the graph's summary is not drawn; texts: %q", tester.Texts())
	}
	if rect.Y <= summary.Y {
		t.Errorf("the legend is at y=%.1f, above the summary at y=%.1f: it reads as a subtitle of the head rather than as the key to the map",
			rect.Y, summary.Y)
	}
}

// TestTheAuthorLayerIsCountedInWords covers a format string that reached the reader.
//
// The author count was drawn with Tf on the suffix key: "author nodes" has no verb in either
// locale, so the header read "位作者节点 %!(EXTRA int=18)". The source composes the count and
// the suffix as two strings (TagGraphView.tsx:99). The rings themselves also carried no names,
// so the whole layer was anonymous circles around the tags.
func TestTheAuthorLayerIsCountedInWords(t *testing.T) {
	shell, _ := newShell(t)
	shell.SetLocale(i18n.EN, i18n.EN)
	shell.Reload(context.Background())

	model := ringGraph(8)
	model.Authors = []domain.GraphViewAuthorNode{
		{TagRelationAuthorNode: domain.TagRelationAuthorNode{AuthorHandle: "ada"}, X: 0.12, Y: 0.12, Radius: 8},
		{TagRelationAuthorNode: domain.TagRelationAuthorNode{AuthorHandle: "grace"}, X: 0.88, Y: 0.12, Radius: 8},
		{TagRelationAuthorNode: domain.TagRelationAuthorNode{AuthorHandle: "linus"}, X: 0.12, Y: 0.88, Radius: 8},
	}
	shell.state.GraphModel = model
	shell.state.GraphLayout = layoutGraph(model)
	shell.state.GraphIncludeAuthors = true
	shell.state.GraphMode = true

	tester := ui.NewTester(shell.View, 1280, 820)
	tester.Frame()

	if _, ok := tester.Find("3 author nodes"); !ok {
		t.Errorf("the head does not count the author layer in words; texts: %q", tester.Texts())
	}
	for _, text := range tester.Texts() {
		if strings.Contains(text, "%!") {
			t.Errorf("a format string reached the page: %q", text)
		}
	}
}

// TestAnAuthorRingCarriesItsHandle covers the other half: the rings had no names at all.
//
// The chart is painted, so the handles are not elements the tester can find; where they go is
// decided by graphAuthorLabels, which takes the plotted rect and the tag captions already
// placed and returns the ones that fit.
func TestAnAuthorRingCarriesItsHandle(t *testing.T) {
	model := ringGraph(8)
	model.Authors = []domain.GraphViewAuthorNode{
		{TagRelationAuthorNode: domain.TagRelationAuthorNode{AuthorHandle: "ada"}, X: 0.12, Y: 0.12, Radius: 8},
		{TagRelationAuthorNode: domain.TagRelationAuthorNode{AuthorHandle: "grace"}, X: 0.88, Y: 0.12, Radius: 8},
		{TagRelationAuthorNode: domain.TagRelationAuthorNode{AuthorHandle: "linus"}, X: 0.5, Y: 0.5, Radius: 8},
	}
	plot := ui.Rect{W: 900, H: 600}
	toCanvas := func(x, y float32) (float32, float32) { return x * plot.W, y * plot.H }

	labels := graphAuthorLabels(plot, model, toCanvas, nil)
	if len(labels) != 3 {
		t.Fatalf("%d of the model's 3 rings were named, want all 3 on an empty canvas", len(labels))
	}
	if labels[0].Handle != "ada" {
		t.Errorf("the first handle is %q, want ada: the layer is anonymous again", labels[0].Handle)
	}
	// Under the ring, six points off it, as the source draws it (TagGraphView.tsx:210-215).
	ada := labels[0].Box
	if want := float32(0.12)*plot.H + 8 + GraphLabelGap; ada.Y < want-0.5 || ada.Y > want+0.5 {
		t.Errorf("ada's handle is at y=%.1f, want %.1f: not under its ring", ada.Y, want)
	}

	// A handle that would land on a tag caption is dropped rather than drawn through it.
	busy := []ui.Rect{ada}
	labels = graphAuthorLabels(plot, model, toCanvas, busy)
	for _, label := range labels {
		if label.Handle == "ada" {
			t.Error("ada's handle is drawn on top of the caption already there")
		}
	}
	if len(labels) != 2 {
		t.Errorf("%d handles survive one collision, want 2", len(labels))
	}

	// And one at the edge of the plot, with no room for its plate, is left out too.
	model.Authors[0].X = 1
	labels = graphAuthorLabels(plot, model, toCanvas, nil)
	for _, label := range labels {
		if label.Handle == "ada" {
			t.Error("ada's handle is drawn past the canvas's edge")
		}
	}
}

// TestTheFocusPanelHasRoom covers "下面的区域也被严重的挤压".
//
// The panel was 8 of padding with 2 between its lines and FontTiny for the numbers: a strip
// about 90 points tall under a map that had taken the rest of the page. The source's own panel
// is a card -- 12 of padding, 6 between the lines, a 26% accent border on the selected ground
// (extension.css:4521-4530) -- and at 12 and 13 points the three lines are readable.
//
// The panel is found by its accessible name, which is part of the fix: three loose lines under
// a map were also three loose lines to a screen reader.
func TestTheFocusPanelHasRoom(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seedTagged(t, db, "a", "rust")
	seedTagged(t, db, "b", "rust")
	seedTagged(t, db, "c", "go")
	shell.Reload(ctx)
	shell.RebuildGraph(ctx)
	shell.ToggleGraph(ctx)
	shell.State().ActiveTagIDs = []string{"rust"}

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	panel, ok := tester.Find(shell.t().T(i18n.GraphFocusPanel))
	if !ok {
		t.Fatalf("the focused tag's panel is not drawn as a labelled card; texts: %q", tester.Texts())
	}
	if panel.H < GraphFocusMinHeight {
		t.Errorf("the focus panel is %.0f points tall, under the %d it needs: its lines are crammed into a strip",
			panel.H, GraphFocusMinHeight)
	}

	meta, ok := tester.Find(shell.t().Tf("%d %s · %d %s", 2,
		shell.t().T(i18n.TagTableBookmarks), 1, shell.t().T(i18n.GraphAuthorsSuffix)))
	if !ok {
		t.Fatalf("the panel does not say what the tag holds; texts: %q", tester.Texts())
	}
	// The border takes one of the twelve.
	if inset := meta.X - panel.X; inset < GraphFocusPad {
		t.Errorf("the panel's text starts %.0f points in from its edge, want %d: it is a strip, not a card",
			inset, GraphFocusPad)
	}
	// 12 points of text measures about 14.1 in the tester; the panel used FontTiny (11), which
	// measures about 13.0. Halfway between the two, so either a size change or a font change
	// that shrinks it again is caught.
	const metaFloor = 13.5
	if meta.H < metaFloor {
		t.Errorf("the panel's numbers are %.1f points tall, under %.1f: they are drawn smaller than the source's 12px meta copy",
			meta.H, metaFloor)
	}
}

// TestAFocusedGraphNamesOnlyWhatItLights covers "很乱".
//
// With a tag focused, every node outside its neighbourhood stays on the canvas dimmed to 24%
// -- and so did its caption, so the page carried twenty ghost names around one legible one.
// A name nobody can read is not a name; the node is still a dot to point at, and the panel
// below names what was clicked.
func TestAFocusedGraphNamesOnlyWhatItLights(t *testing.T) {
	model := ringGraph(8)
	points := layoutGraph(model)

	positions := make(map[string][2]float32, len(points))
	for _, point := range points {
		positions[point.TagID] = [2]float32{point.X * 900, point.Y * 900}
	}

	focused := 0
	neighbours := map[string]bool{"t1": true, "t2": true}
	named := graphCaptionIndexes(points, positions, focused, neighbours,
		ui.Rect{W: 900, H: 900})

	if len(named) == 0 {
		t.Fatal("a focused graph names nothing at all")
	}
	for _, i := range named {
		id := points[i].TagID
		if i != focused && !neighbours[id] {
			t.Errorf("%s is named but neither focused nor a neighbour: the page draws names it dims to unreadable", id)
		}
	}
	if last := named[len(named)-1]; last != focused {
		t.Errorf("the last caption drawn is %s, not the focused tag: the focused name can be covered by a dimmed one",
			points[last].TagID)
	}
}
