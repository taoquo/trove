package ui

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
)

// The icon shapes are embedded from the generated fixture rather than transcribed into
// Go literals: 34 icons of Phosphor path data is thousands of characters that nobody
// should hand-copy, and a typo in one would change a glyph without failing anything.
//
//go:embed testdata/icons.json
var iconsJSON []byte

func init() {
	var fixture struct {
		GeneratedFrom string `json:"generatedFrom"`
		Icons         map[string]struct {
			Component string   `json:"component"`
			ViewBox   int      `json:"viewBox"`
			Paths     []string `json:"paths"`
		} `json:"icons"`
	}
	if err := json.Unmarshal(iconsJSON, &fixture); err != nil {
		panic(fmt.Sprintf("ui: the embedded icon fixture is unreadable: %v", err))
	}
	if len(fixture.Icons) == 0 {
		panic("ui: the embedded icon fixture is empty")
	}

	for name, icon := range fixture.Icons {
		if icon.ViewBox != IconViewBox {
			panic(fmt.Sprintf("ui: %s uses viewBox %d, not %d", name, icon.ViewBox, IconViewBox))
		}
		iconPaths[name] = icon.Paths
		iconSVGs[name] = ui.MustParseSVG(iconDocument(icon.Paths))
	}
}

// iconDocument wraps one icon's path data in the SVG document MyGo parses.
//
// The paths are Phosphor's own and carry no fill-rule, so the default nonzero fill applies.
// Subpaths that look like holes (the magnifier's
// lens, the tag's eyelet) are drawn by their winding, which nonzero honours and a hand-rolled
// even-odd fill would have to guess.
func iconDocument(paths []string) []byte {
	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 256 256">`)
	for _, d := range paths {
		b.WriteString(`<path d="`)
		b.WriteString(d)
		b.WriteString(`"/>`)
	}
	b.WriteString(`</svg>`)
	return []byte(b.String())
}
