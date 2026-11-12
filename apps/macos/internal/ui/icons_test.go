package ui

import (
	"encoding/json"
	"image"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

type iconsFixture struct {
	GeneratedFrom string `json:"generatedFrom"`
	Icons         map[string]struct {
		Component string   `json:"component"`
		ViewBox   int      `json:"viewBox"`
		Paths     []string `json:"paths"`
	} `json:"icons"`
}

func loadIconsFixture(t *testing.T) iconsFixture {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "icons", "icons.json"))
	if err != nil {
		t.Fatalf("read fixture (regenerate with `node apps/macos/testdata/icons/genicons.mjs`): %v", err)
	}

	var fixture iconsFixture
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	return fixture
}

// TestEveryIconReachesTheRenderer is the check that matters: the interface had no icons
// at all, so the failure mode is a name that silently draws nothing.
func TestEveryIconReachesTheRenderer(t *testing.T) {
	fixture := loadIconsFixture(t)

	if len(fixture.Icons) != 34 {
		t.Errorf("the extension has %d icons, the fixture has %d", 34, len(fixture.Icons))
	}
	if len(iconPaths) != len(fixture.Icons) {
		t.Fatalf("the renderer knows %d icons, the fixture has %d", len(iconPaths), len(fixture.Icons))
	}
	if len(iconSVGs) != len(fixture.Icons) {
		t.Fatalf("the renderer parsed %d icons, the fixture has %d", len(iconSVGs), len(fixture.Icons))
	}

	for name, icon := range fixture.Icons {
		paths, known := iconPaths[name]
		if !known {
			t.Errorf("the extension draws %q and the renderer does not know it", name)
			continue
		}
		if len(paths) != len(icon.Paths) {
			t.Errorf("%s has %d paths, the fixture has %d", name, len(paths), len(icon.Paths))
		}
		if iconSVGs[name] == nil {
			t.Errorf("%s has path data but no parsed document", name)
		}
	}
}

// renderIcon draws one icon on white through the real painter and returns the picture.
//
// This is deliberately the whole path -- SVG document, MyGo's rasteriser, the frame's
// glyph atlas -- because every earlier icon defect was found at the seam between two
// pieces that each looked right on their own.
func renderIcon(t *testing.T, name string, size int) *image.RGBA {
	t.Helper()

	view := func(c *ui.Context) {
		ui.Box(c).Fill().Background(ui.RGBA(255, 255, 255, 1)).
			Draw(func(p *ui.Painter, r ui.Rect) {
				drawIcon(p, name, r.X, r.Y, r.W, ui.RGB(0, 0, 0))
			})
	}
	tester := ui.NewTester(view, size, size)
	tester.Frame()

	img := tester.Image()
	if img == nil {
		t.Fatalf("%q produced no image", name)
	}
	return img
}

// inkPixels counts the pixels the glyph covers, the fully inked and the partly inked ones.
func inkPixels(img *image.RGBA) (solid, partial int) {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, _, _, a := img.At(x, y).RGBA()
			if a == 0 {
				continue
			}
			// Composited over white: black is 0, untouched is 65535.
			switch {
			case r < 32768:
				solid++
			case r < 65535:
				partial++
			}
		}
	}
	return solid, partial
}

// TestEveryIconDrawsPixels is the end-to-end check: a parsed document that paints nothing
// fails here, whatever its path data says.
func TestEveryIconDrawsPixels(t *testing.T) {
	for _, name := range IconNames() {
		solid, _ := inkPixels(renderIcon(t, name, 32))
		if solid < 8 {
			t.Errorf("%q covers %d pixels at 32x32; it is not being drawn", name, solid)
		}
	}
}

// TestAnUnknownIconDrawsNothing keeps a typo from panicking or drawing a stray mark.
func TestAnUnknownIconDrawsNothing(t *testing.T) {
	if _, known := iconSVGs["not-an-icon"]; known {
		t.Error("an unknown name must not resolve")
	}
	if paths := IconPaths("not-an-icon"); len(paths) != 0 {
		t.Error("an unknown name must have no paths")
	}
	if solid, _ := inkPixels(renderIcon(t, "not-an-icon", 32)); solid != 0 {
		t.Errorf("an unknown icon covers %d pixels", solid)
	}
}

// TestIconsAreAntiAliased is the regression test for the reported jagged edges.
//
// The old renderer flattened the paths itself and filled them one horizontal scanline at a
// time, each span a snapped rectangle: every edge was either fully inked or fully clear, so
// a curve read as a staircase. A vector rasteriser leaves partly covered pixels along the
// edges, which is what makes the curve look smooth at 14-18 points.
func TestIconsAreAntiAliased(t *testing.T) {
	for _, name := range []string{"close", "search", "tag", "check", "grid"} {
		solid, partial := inkPixels(renderIcon(t, name, 18))
		if solid == 0 {
			t.Errorf("%q drew nothing at 18x18", name)
			continue
		}
		if partial == 0 {
			t.Errorf("%q has no partly covered edge pixels: its edges are hard-stepped", name)
		}
		// A glyph that is all soft edge and no body would pass the check above.
		if partial > solid*3 {
			t.Errorf("%q is %d edge pixels to %d solid ones: the shape is dissolving", name, partial, solid)
		}
	}
}

// TestTheGlyphKeepsItsHole covers the subpaths that read as holes. The magnifier's lens is a
// ring: the middle is not part of the shape, and a renderer that fills every subpath solid
// draws a disc. Phosphor relies on the default nonzero winding to cut it out.
func TestTheGlyphKeepsItsHole(t *testing.T) {
	const size = 64
	img := renderIcon(t, "search", size)

	// The lens is centred at (112,112) in the 256 view box; at 64 points that is (28,28).
	r, _, _, _ := img.At(size*112/256, size*112/256).RGBA()
	if r < 50000 {
		t.Errorf("the lens centre is inked (r=%d): the hole was filled in", r)
	}

	// And the ring itself must be drawn, or the check above passes on a blank icon.
	ring := size * 72 / 256 // the lens radius is 72 units
	r, _, _, _ = img.At(size*112/256+ring, size*112/256).RGBA()
	if r > 40000 {
		t.Errorf("the lens ring is blank (r=%d): only the hole was drawn", r)
	}
}

// TestIconsLandInsideTheirBox is what catches a scaling mistake: a renderer that forgot the
// 256-unit view box would draw the same shape at 256 times the size, mostly off the edge.
func TestIconsLandInsideTheirBox(t *testing.T) {
	const size, origin = 40, 10

	view := func(c *ui.Context) {
		ui.Box(c).Fill().Background(ui.RGBA(255, 255, 255, 1)).
			Draw(func(p *ui.Painter, r ui.Rect) {
				drawIcon(p, "close", origin, origin, size, ui.RGB(0, 0, 0))
			})
	}
	tester := ui.NewTester(view, 64, 64)
	tester.Frame()
	img := tester.Image()

	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			if x >= origin && x < origin+size && y >= origin && y < origin+size {
				continue
			}
			r, _, _, _ := img.At(x, y).RGBA()
			if r < 60000 {
				t.Fatalf("the glyph inks (%d,%d), outside its %d..%d box", x, y, origin, origin+size)
			}
		}
	}
}

// inkCoverage is the glyph's area in pixels, counting partly covered edges for what they
// cover. Counting whole pixels instead would undercount a thin stroke at a small size,
// where most of the stroke is anti-aliased edge.
func inkCoverage(img *image.RGBA) float64 {
	b := img.Bounds()
	total := 0.0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, _, _, a := img.At(x, y).RGBA()
			if a == 0 {
				continue
			}
			total += 1 - float64(r)/65535
		}
	}
	return total
}

// TestIconSizeScalesTheGlyph keeps the size parameter meaningful: a filled shape's area
// grows with the square of its size.
func TestIconSizeScalesTheGlyph(t *testing.T) {
	small := inkCoverage(renderIcon(t, "close", 16))
	large := inkCoverage(renderIcon(t, "close", 32))

	if small == 0 || large == 0 {
		t.Fatalf("the glyph covers %.2f pixels at 16 and %.2f at 32", small, large)
	}
	ratio := large / small
	if ratio < 3.6 || ratio > 4.4 {
		t.Errorf("doubling the size multiplied the covered area by %.2f, want about 4", ratio)
	}
}

// TestEveryIconHasEnoughDetail keeps a document that failed to parse from passing on a
// stray pixel: the simplest glyph in the set covers far more than a crumb.
func TestEveryIconHasEnoughDetail(t *testing.T) {
	for _, name := range IconNames() {
		solid, _ := inkPixels(renderIcon(t, name, 32))
		if solid < 20 {
			t.Errorf("%q covers only %d pixels at 32x32", name, solid)
		}
	}
}
