package ui

import (
	"bytes"
	_ "embed"
	"image"
	_ "image/png"
	"sync"

	"github.com/egoist/mygo/ui"
)

// The sidebar's brand mark, from src/ui/branding.tsx.
//
// The extension renders <BrandLogo size={38} />, which is assets/branding/logo-128.png at
// 38 DIPs in a 12px rounded box. magpie draws its own mark at 18 in the same row as a 13.5px
// wordmark (index.html:62, app.css:366-374), so the restyle in round 531 puts this mark at 22
// and its radius at 6: the app's logo is a filled square rather than a line bird, and 22 is
// the size at which the two read as one row. The asset is embedded rather than read from disk
// so the app does not depend on a file beside it, and it is decoded once rather than per frame.

// BrandLogoSize is the size the sidebar asks for.
const BrandLogoSize = 22

// BrandLogoRadius rounds the mark to the sidebar's row radius, as magpie's
// 18px bird sits in its 30px rows (app.css:366-374).
const BrandLogoRadius = 6

//go:embed testdata/branding/logo-128.png
var brandLogoPNG []byte

var (
	brandLogoOnce sync.Once
	brandLogo     *ui.Bitmap
)

// BrandLogo returns the decoded mark, or nil when it could not be read.
//
// A nil mark is drawn as nothing rather than panicking: the logo is decoration, and an
// app that refuses to start because an icon failed to decode is worse than one without
// the icon.
func BrandLogo() *ui.Bitmap {
	brandLogoOnce.Do(func() {
		decoded, _, err := image.Decode(bytes.NewReader(brandLogoPNG))
		if err != nil {
			return
		}
		brandLogo = ui.NewBitmap(decoded)
	})
	return brandLogo
}

// BrandLogoSize_ is the mark's width and height in DIPs.
func brandLogoElement(c *ui.Context) ui.Element {
	mark := BrandLogo()
	if mark == nil {
		return ui.Box(c).Width(BrandLogoSize).Height(BrandLogoSize)
	}
	return ui.Image(c, mark).Width(BrandLogoSize).Height(BrandLogoSize).
		Radius(BrandLogoRadius)
}
