package ui

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/i18n"
)

// The media lightbox.
//
// It is a fixed overlay above the detail overlay (z-index 60 against the detail's 40). Its
// shell holds three rows: a toolbar with the close control, a stage with the picture between
// two arrow tracks, and a counter. The arrows and the counter are only there when the
// bookmark has more than one item.

// LightboxBackdropZ and LightboxOverlayZ are its stacking, relative to the detail overlay's.
const (
	LightboxOverlayZ  = 60
	LightboxBackdropZ = 59
)

// The shell: width min(94vw, 1320px), max-height min(92vh, 980px), gap 14, radius 30,
// padding 16.
const (
	LightboxShellWidthPercent  = 94
	LightboxShellMaxWidth      = 1320
	LightboxShellMaxHeightPct  = 92
	LightboxShellMaxHeight     = 980
	LightboxShellGap           = 14
	LightboxShellRadius        = 30
	LightboxShellPadding       = 16
	LightboxToolbarMinHeight   = 42
	LightboxCloseSize          = 38
	LightboxStageSide          = 72
	LightboxStageGap           = 18
	LightboxNavSize            = 46
	LightboxNavGlyphSize       = 30
	LightboxFooterMinHeight    = 34
	LightboxCounterFontSize    = 12
	LightboxCounterPadY        = 8
	LightboxCounterPadX        = 14
	LightboxControlRestOpacity = 0.82
	// The video poster's play control: a 64-point disc carrying a 26-point glyph, on the
	// same dark ground as the other controls in this overlay.
	LightboxPlaySize  = 64
	LightboxPlayGlyph = 26
)

// The play control's grounds, at rest and under the pointer.
var (
	LightboxPlayGround = ui.RGBA(25, 21, 20, 0.55)
	LightboxPlayHover  = ui.RGBA(25, 21, 20, 0.72)
)

// The picture's own card: max-width min(80vw, 1100px), max-height min(76vh, 800px), padding
// clamp(10px, 1.6vw, 16px), radius 24, and the image inside it at radius 18.
const (
	LightboxContentMaxWidthPct  = 80
	LightboxContentMaxWidth     = 1100
	LightboxContentMaxHeightPct = 76
	LightboxContentMaxHeight    = 800
	LightboxContentPadding      = 16
	LightboxContentRadius       = 24
	LightboxImageRadius         = 18
	// LightboxViewport is the 76vh fraction, and LightboxImageMaxH its 800 cap.
	LightboxViewport  = 0.76
	LightboxImageMaxH = 800
	LightboxImageMinH = 240
)

// OpenLightbox shows the media at an index, clamped to what the bookmark has.
func (s *Shell) OpenLightbox(index int) {
	row, found := s.selectedBookmark()
	if !found || len(row.Media) == 0 {
		return
	}
	// A player showing the previous post's video would sit over this one's picture.
	s.StopVideo()
	if index < 0 {
		index = 0
	}
	if index >= len(row.Media) {
		index = len(row.Media) - 1
	}
	s.state.LightboxIndex = index
	if s.window != nil {
		s.window.Invalidate()
	}
}

// CloseLightbox hides it.
func (s *Shell) CloseLightbox() {
	s.state.LightboxIndex = -1
	// The player is a view over the window, not a child of this overlay: closing the preview
	// has to take it away explicitly, or the video keeps playing over the results.
	s.StopVideo()
	// The preview's focus goes back to the detail. Left to itself, MyGo moves it to the first
	// control of the dialog -- the new-tag field -- and the next Escape then clears that field
	// instead of closing the detail.
	if s.state.DetailOpen {
		s.detailCloseField.Focus()
	}
	if s.window != nil {
		s.window.Invalidate()
	}
}

// LightboxOpen reports whether the overlay is showing.
func (s *Shell) LightboxOpen() bool { return s.state.LightboxIndex >= 0 }

// NextMedia steps forward, wrapping to the first item.
//
// It wraps rather than stopping: with two pictures, a "next" that does nothing is
// indistinguishable from a broken button.
func (s *Shell) NextMedia() {
	media := s.selectedMedia()
	// A closed lightbox stays closed. Without this, an index of -1 steps to 0 and the
	// overlay opens -- so a "next" bound anywhere would show it.
	if len(media) == 0 || s.state.LightboxIndex < 0 {
		return
	}
	s.StopVideo()
	s.state.LightboxIndex = (s.state.LightboxIndex + 1) % len(media)
	if s.window != nil {
		s.window.Invalidate()
	}
}

// PrevMedia steps back, wrapping to the last item.
func (s *Shell) PrevMedia() {
	media := s.selectedMedia()
	if len(media) == 0 || s.state.LightboxIndex < 0 {
		return
	}
	s.StopVideo()
	s.state.LightboxIndex = (s.state.LightboxIndex - 1 + len(media)) % len(media)
	if s.window != nil {
		s.window.Invalidate()
	}
}

// selectedMedia is the selected bookmark's media, or nothing.
func (s *Shell) selectedMedia() []domain.Media {
	row, found := s.selectedBookmark()
	if !found {
		return nil
	}
	return row.Media
}

// lightboxIndex is the current index, clamped to the media that is actually there.
//
// A bookmark can change under an open lightbox -- a sync replaces the row -- so an index
// held from the previous bookmark would point past the end.
func (s *Shell) lightboxIndex() (int, bool) {
	media := s.selectedMedia()
	if len(media) == 0 || s.state.LightboxIndex < 0 {
		return 0, false
	}
	if s.state.LightboxIndex >= len(media) {
		return len(media) - 1, true
	}
	return s.state.LightboxIndex, true
}

// lightboxEntrance drives the lightbox's own entrance.
//
// The detail overlay's entrance would be the wrong animation: the lightbox pops rather
// than scales, and lifts 10px rather than 4.
func (s *Shell) lightboxEntrance(root ui.Element, reduceMotion bool) float32 {
	return EntranceProgress(root, "entrance-lightbox", s.LightboxOpen(), EntranceLightbox, reduceMotion)
}

// lightboxBackdropEntrance drives the dim behind the lightbox.
//
// It needs its own call rather than reusing the card's progress: a progress value is computed
// against one Entrance, so the card's would give the backdrop the pop's 220ms and curve where
// the backdrop takes 180ms. The two are separate animations on separate elements, and
// EntranceLightboxBackdrop exists to say so -- the backdrop needs its own element to apply it
// to.
func (s *Shell) lightboxBackdropEntrance(root ui.Element, reduceMotion bool) float32 {
	return EntranceProgress(root, "entrance-lightbox-backdrop", s.LightboxOpen(), EntranceLightboxBackdrop, reduceMotion)
}

// lightboxImageEntrance drives the picture inside the card.
//
// The third of the lightbox's three animations. The picture is drawn by a shared media
// helper, so it needs a wrapper carrying only the lightbox's image animation.
func (s *Shell) lightboxImageEntrance(root ui.Element, reduceMotion bool) float32 {
	return EntranceProgress(root, "entrance-lightbox-image", s.LightboxOpen(), EntranceLightboxImage, reduceMotion)
}

// renderLightbox draws the media overlay.
//
// The lightbox is a column of a toolbar, a stage and a footer, and the stage is a three-track
// grid -- 72px, minmax(0, 1fr), 72px -- with the picture in the middle track. The stage's two
// side tracks
// are drawn even when there is nothing in them, so a single picture sits where a picture
// between arrows sits rather than 72 points off centre.
func (s *Shell) renderLightbox(c *ui.Context, theme *ui.Theme, rail RailTokens, entrance, imageEntrance float32) {
	index, open := s.lightboxIndex()
	if !open {
		return
	}
	media := s.selectedMedia()
	multiple := len(media) > 1

	ui.Column(c).
		WidthPercent(LightboxShellWidthPercent).MaxWidth(LightboxShellMaxWidth).
		MaxHeightPercent(LightboxShellMaxHeightPct).MaxHeight(LightboxShellMaxHeight).
		Padding(LightboxShellPadding).GapY(LightboxShellGap).Radius(LightboxShellRadius).
		Background(rail.Inspector).
		// The shell is a panel with a hairline and a deep shadow: `1px solid color-mix(panel-border
		// 78%, rgba(255,255,255,0.22))` and `0 36px 96px -40px rgba(25,21,20,0.5)`. Without them
		// the card and the picture were one flat surface -- the two fills are the same colour.
		Border(1, rail.SubtleLine.Mix(ui.RGBA(255, 255, 255, 0.22), 0.22)).
		Shadow(0, 36, 96, 0, ui.RGBA(25, 21, 20, 0.5)).
		Children(func() {
			// The toolbar is a right-aligned row with the close control in it and nothing
			// else.
			ui.Row(c).FillWidth().MinHeight(LightboxToolbarMinHeight).Justify(ui.End).Children(func() {
				s.lightboxClose(c, rail, func() { s.CloseLightbox() })
			})

			ui.Row(c).FillWidth().GapX(LightboxStageGap).AlignItems(ui.Center).Children(func() {
				if multiple {
					s.lightboxNav(c, "‹", s.t().T(i18n.MediaPrevious), rail, ui.Start, func() { s.PrevMedia() })
				} else {
					ui.Box(c).Width(LightboxStageSide).Height(LightboxNavSize)
				}

				// The picture's card. The pop animation belongs to this element, not the
				// shell.
				ui.Column(c).Grow(1).MinWidth(0).
					MaxWidthPercent(LightboxContentMaxWidthPct).MaxWidth(LightboxContentMaxWidth).
					MaxHeightPercent(LightboxContentMaxHeightPct).MaxHeight(LightboxContentMaxHeight).
					Padding(LightboxContentPadding).Radius(LightboxContentRadius).
					Background(rail.PillBg).Center().
					Border(1, theme.Border.Mix(ui.RGBA(255, 255, 255, 0.24), 0.28)).
					Shadow(0, 30, 76, 0, ui.RGBA(25, 21, 20, 0.34)).
					Opacity(EntranceLightbox.OpacityAt(entrance)).
					// The lightbox scales from 0.96 and lifts 10px, where the detail card
					// scales from the same 0.96 without lifting.
					Scale(EntranceLightbox.ScaleAt(entrance), EntranceLightbox.ScaleAt(entrance)).
					Transform(ui.Translation(0, EntranceLightbox.OffsetYAt(entrance))).
					Children(func() {
						s.mediaLightboxImage(c, media[index], s.lightboxImageHeight(), imageEntrance, theme)
						// A video's poster gets the one control the poster cannot have: play.
						// The player replaces it once it starts, so the mark goes with it.
						// An animated GIF takes the same path -- X serves it as an mp4, and
						// domain.IsVideoMediaType covers both.
						if domain.IsVideoMediaType(string(media[index].Type)) && !s.VideoPlaying() {
							s.lightboxPlay(c, media[index], func() { s.PlayVideo(media[index].URL) })
						}
					})

				if multiple {
					s.lightboxNav(c, "›", s.t().T(i18n.MediaNext), rail, ui.End, func() { s.NextMedia() })
				} else {
					ui.Box(c).Width(LightboxStageSide).Height(LightboxNavSize)
				}
			})

			// The counter, and only when there is more than one item to count.
			if multiple {
				ui.Row(c).FillWidth().MinHeight(LightboxFooterMinHeight).Justify(ui.Center).Children(func() {
					ui.Text(c, fmt.Sprintf("%d / %d", index+1, len(media))).
						FontSize(LightboxCounterFontSize).FontWeight(600).
						TextColor(ui.RGBA(255, 255, 255, 1)).
						Background(rail.Inspector.Mix(ui.RGBA(25, 21, 20, 0.42), 0.18)).
						Padding(LightboxCounterPadY, LightboxCounterPadX).Radius(RadiusPill)
				})
			}
		})
}

// lightboxPlay is the play control over a video's or GIF's poster.
//
// It is the only control the preview adds for a moving picture, and it starts the native
// player in the same box the poster occupies (video.go). The mark is labelled for the
// keyboard and the accessibility tree, not just drawn: a circle with a triangle in it is not
// a name. The name says which kind it is, because "Play video" over a GIF is the wrong noun.
func (s *Shell) lightboxPlay(c *ui.Context, item domain.Media, onClick func()) {
	label := s.t().T(i18n.MediaPlay)
	if item.Type == domain.MediaAnimatedGif {
		label = s.t().T(i18n.MediaPlayGif)
	}
	ui.Box(c).Absolute().Left(0).Top(0).Right(0).Bottom(0).Center().Children(func() {
		disc := ui.Box(c).Width(LightboxPlaySize).Height(LightboxPlaySize).Radius(RadiusPill).
			Background(LightboxPlayGround).Center().Focusable().
			Label(label).Tooltip(label).
			OnClick(onClick)
		if disc.Hovered() {
			disc = disc.Background(LightboxPlayHover)
		}
		disc.Children(func() {
			ui.Box(c).Width(LightboxPlayGlyph).Height(LightboxPlayGlyph).
				Draw(func(p *ui.Painter, r ui.Rect) {
					drawIcon(p, "play", r.X, r.Y, min32(r.W, r.H), ui.RGBA(255, 255, 255, 1))
				})
		})
	})
}

// lightboxClose draws the close control: a 38-point round control with the close glyph in
// it, at 82% opacity until the pointer is over it.
func (s *Shell) lightboxClose(c *ui.Context, rail RailTokens, onClick func()) {
	box := ui.Box(c).Width(LightboxCloseSize).Height(LightboxCloseSize).Radius(RadiusPill).
		Background(rail.Inspector.Mix(ui.RGBA(25, 21, 20, 0.46), 0.22)).
		Opacity(LightboxControlRestOpacity).Focusable().AutoFocus().
		Label(s.t().T(i18n.MediaClose)).
		OnClick(onClick)
	if box.Hovered() {
		box = box.Opacity(1)
	}
	box.Draw(func(p *ui.Painter, r ui.Rect) {
		const glyph = 18
		drawIcon(p, "close", r.X+(r.W-glyph)/2, r.Y+(r.H-glyph)/2, glyph, ui.RGBA(255, 255, 255, 1))
	})
}

// lightboxNav draws one of the preview's arrow controls in its 72-point track.
//
// The glyph is ‹ or › -- text, not an icon: the icon fixture has no chevron, and a character
// is enough here.
func (s *Shell) lightboxNav(c *ui.Context, glyph, label string, rail RailTokens, place ui.Align, onClick func()) {
	ui.Row(c).Width(LightboxStageSide).Justify(place).Children(func() {
		box := ui.Box(c).Width(LightboxNavSize).Height(LightboxNavSize).Radius(RadiusPill).
			Background(rail.Inspector.Mix(ui.RGBA(25, 21, 20, 0.46), 0.22)).
			Opacity(LightboxControlRestOpacity).Focusable().
			Center().
			Label(label).
			OnClick(onClick)
		if box.Hovered() {
			box = box.Opacity(1)
		}
		box.Children(func() {
			ui.Text(c, glyph).FontSize(LightboxNavGlyphSize).TextColor(ui.RGBA(255, 255, 255, 1))
		})
	})
}

// lightboxImageHeight is the stage's picture.
//
// The image is `max-height: min(76vh, 800px)` and `object-fit: contain`: the whole picture,
// not a square crop of its middle. The card's own padding is taken off the height rather than
// added to it, so the card -- not the shell's max-height -- decides how tall the picture is.
func (s *Shell) lightboxImageHeight() float32 {
	height := s.viewportHeight()*LightboxViewport - 2*LightboxContentPadding
	if height > LightboxImageMaxH {
		height = LightboxImageMaxH
	}
	if height < LightboxImageMinH {
		height = LightboxImageMinH
	}
	return height
}
