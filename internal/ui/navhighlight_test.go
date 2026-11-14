package ui

import (
	"context"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// The sidebar's selection shape, as behaviour.
//
// The list has one shape that slides from row to row, and a row has no ground of its
// own in any state. These pin where the shape is painted and that
// it follows the row picked; the slide itself is MyGo's AnimateTo, along the settle curve
// over .24s.

// navHighlightInk reads the grey level of one point of the current frame.
func navHighlightInk(tester *ui.Tester, x, y float32) uint8 {
	c := tester.Image().RGBAAt(int(x), int(y))
	return uint8((int(c.R) + int(c.G) + int(c.B)) / 3)
}

// navHighlightList is a list of three 26-high rows, 4 apart, on white, with the row at
// *pick marked -- the geometry of the sidebar's sub-rows. It records each row's committed
// rectangle so a test can sample inside them and between them.
func navHighlightList(pick *int, rows *[]ui.Rect) func(c *ui.Context) {
	return func(c *ui.Context) {
		*rows = (*rows)[:0]
		ui.Column(c).Fill().Background(ui.RGBA(255, 255, 255, 1)).Padding(50).Children(func() {
			list := ui.Column(c).Width(200).GapY(SidebarRowsGap)
			var elements []ui.Element
			list.Children(func() {
				for range 3 {
					elements = append(elements, ui.Box(c).FillWidth().Height(SubRowMinHeight))
				}
			})
			for _, row := range elements {
				*rows = append(*rows, row.Bounds())
			}
			if *pick >= 0 && *pick < len(elements) {
				navHighlight(c, list, elements[*pick], ui.LightTheme())
			}
		})
	}
}

// TestTheSelectionShapeMarksExactlyItsRow: the tint lands under the chosen row, spans its
// width, and nowhere else -- not in the gap above it, not outside the list.
func TestTheSelectionShapeMarksExactlyItsRow(t *testing.T) {
	pick := 1
	var rows []ui.Rect
	tester := ui.NewTester(navHighlightList(&pick, &rows), 400, 400)
	// The first frame has no committed boxes: the row paints the tint itself. The second
	// hands the shape to the list, which is what this test is about.
	tester.Frame()
	tester.Frame()
	if len(rows) != 3 || rows[1].H != SubRowMinHeight {
		t.Fatalf("the three rows are not laid out as expected: %+v", rows)
	}

	mid := rows[1].Y + rows[1].H/2
	if got := navHighlightInk(tester, rows[1].X+10, mid); got == 255 {
		t.Error("the chosen row carries no tint")
	}
	for i, row := range rows {
		if i == 1 {
			continue
		}
		if got := navHighlightInk(tester, row.X+10, row.Y+row.H/2); got != 255 {
			t.Errorf("row %d carries the tint (%d) and is not chosen", i, got)
		}
	}
	// The 4-point gap between rows stays clean.
	if got := navHighlightInk(tester, rows[1].X+10, rows[0].Y+rows[0].H+SidebarRowsGap/2); got != 255 {
		t.Errorf("the gap above the chosen row is tinted (%d)", got)
	}
	// The shape is the row's width, edge to edge (clear of the 6-point corners).
	for _, x := range []float32{rows[1].X + 1, rows[1].X + rows[1].W - 2} {
		if got := navHighlightInk(tester, x, mid); got == 255 {
			t.Errorf("the shape stops at %v, inside the row [%v, %v]",
				x, rows[1].X, rows[1].X+rows[1].W)
		}
	}
	if got := navHighlightInk(tester, rows[1].X-2, mid); got != 255 {
		t.Error("the shape spills out of the list's left edge")
	}
}

// TestTheSelectionShapeTravelsToTheNextRow: picking another row sends the one shape there,
// across the gap -- the difference between a sliding selection and a row lighting up.
func TestTheSelectionShapeTravelsToTheNextRow(t *testing.T) {
	pick := 0
	var rows []ui.Rect
	tester := ui.NewTester(navHighlightList(&pick, &rows), 400, 400)
	tester.Frame()
	tester.Frame()

	// The frame that retargets the shape shows it still on the old row: the clock a frame
	// builds with has not moved. The journey is visible on the frames after it.
	pick = 1
	tester.Frame()
	gapY := rows[0].Y + rows[0].H + SidebarRowsGap/2
	underway := false
	for range 8 {
		time.Sleep(20 * time.Millisecond)
		tester.Frame()
		if navHighlightInk(tester, rows[0].X+10, gapY) != 255 {
			underway = true
			break
		}
	}
	if !underway {
		t.Error("the shape is not on its way: the gap between the rows stayed clean")
	}

	// And it settles exactly on the row picked, leaving the gap and the row it left clean.
	time.Sleep(2 * MotionGlide.Duration)
	tester.Frame()
	if got := navHighlightInk(tester, rows[1].X+10, rows[1].Y+rows[1].H/2); got == 255 {
		t.Error("the shape did not arrive at the row picked")
	}
	if got := navHighlightInk(tester, rows[0].X+10, rows[0].Y+rows[0].H/2); got != 255 {
		t.Error("the shape is still on the row it left")
	}
	if got := navHighlightInk(tester, rows[0].X+10, gapY); got != 255 {
		t.Error("the shape rests over the gap instead of on its row")
	}
}

// TestClickingATagMarksItAndUnmarksTheLastOne is the sidebar end of the same rule: the tag
// rows carry no selection ground of their own, so what moves is the list's one shape.
func TestClickingATagMarksItAndUnmarksTheLastOne(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(context.Background())
	alpha, err := shell.service.CreateTag(context.Background(), "Alpha")
	if err != nil {
		t.Fatalf("CreateTag: %v", err)
	}
	if _, err := shell.service.CreateTag(context.Background(), "Beta"); err != nil {
		t.Fatalf("CreateTag: %v", err)
	}
	shell.Reload(context.Background())
	shell.state.ActiveTagIDs = []string{alpha.ID}
	shell.Reload(context.Background())

	tester := ui.NewTester(shell.View, 1180, 760)
	// The jump is the same shape at the same place; the slide has its own test, and a
	// reduced-motion reader is a state this must also hold in.
	tester.SetPreferences(ui.Preferences{ReduceMotion: true, TextScale: 1})
	tester.Frame()

	// A point inside a row, clear of its hash and its words: the row's 10 of padding and
	// two more.
	rowX := func(label ui.Rect) float32 {
		return label.X - SubRowPadX - NavRowIcon - SubRowGap + 12
	}
	sample := func(label ui.Rect) uint8 {
		return navHighlightInk(tester, rowX(label), label.Y+label.H/2)
	}
	chosen, ok := tester.Find("Alpha")
	if !ok {
		t.Fatalf("the tag rows are not drawn; texts: %q", tester.Texts())
	}
	other, ok := tester.Find("Beta")
	if !ok {
		t.Fatalf("the second tag row is not drawn; texts: %q", tester.Texts())
	}
	// The list's own ground: the empty column below the last of the two rows.
	ground := navHighlightInk(tester, rowX(other), other.Y+other.H/2+40)
	if sample(chosen) == ground {
		t.Fatalf("the chosen row is the list's own %d: the shape marks nothing", ground)
	}
	if sample(other) != ground {
		t.Errorf("the unchosen row is %d, not the list's own %d: it has a ground of its own",
			sample(other), ground)
	}

	if err := tester.Click("Beta"); err != nil {
		t.Fatalf("clicking the tag row: %v", err)
	}
	tester.Frame()
	if len(shell.state.ActiveTagIDs) != 1 || shell.state.ActiveTagIDs[0] == alpha.ID {
		t.Fatalf("the click did not move the tag condition: %v", shell.state.ActiveTagIDs)
	}
	alphaLabel, ok := tester.Find("Alpha")
	if !ok {
		t.Fatalf("the tag rows are gone; texts: %q", tester.Texts())
	}
	betaLabel, ok := tester.Find("Beta")
	if !ok {
		t.Fatalf("the second tag row is gone; texts: %q", tester.Texts())
	}
	if sample(betaLabel) == sample(alphaLabel) {
		t.Fatalf("both rows read %d after the click: the shape did not move", sample(betaLabel))
	}
}
