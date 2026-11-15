package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/usecase/workspace"
)

// The detail's reading pane is the post, not a set of sections.
//
// The post's words, the pictures they are about and what the post got are one object, so the
// pane draws them as one shape. Splitting them into two labelled blocks with a 22-point gap
// put an overline on content that has one owner, and a summary card inside a dialog panel was
// a third panel layer.

// detailPost draws the selected post: its words, its media, and its counts.
func (s *Shell) detailPost(c *ui.Context, theme *ui.Theme, rail RailTokens, row workspace.BookmarkRow) {
	ui.Column(c).Grow(1).Children(func() {
		text := strings.TrimSpace(row.Text)
		if text != "" {
			// The words get a column of their own, narrower than the pictures they are about:
			// at the pane's full width a 13-point line runs to ~56 CJK characters and the eye
			// loses the start of the line it is returning to. The measure is a cap, so a short
			// post is still a short line rather than a centred one.
			ui.Column(c).FillWidth().MaxWidth(DetailReadWidth).Label(DetailBodyLabel).
				Children(func() {
					ui.Text(c, row.Text).FontSize(DetailBodyFont).LineHeight(DetailBodyLine).
						TextColor(theme.Text)
				})
		}

		if len(row.Media) > 0 {
			// The pictures belong to the sentence above them, so the gap is a paragraph's, not
			// a section's -- and with no sentence above them there is nothing for them to
			// belong to, so the gap goes with the text.
			gap := float32(0)
			if text != "" {
				gap = DetailPostGap
			}
			ui.Column(c).FillWidth().Padding(gap, 0, 0, 0).Children(func() {
				s.detailMedia(c, theme, row.Media)
			})
			// A picture's description sits under the picture, in the quiet ink. It is the only text in the detail that is about the media rather
			// than in it, and it is what a reader with no picture gets.
			if alt := strings.TrimSpace(row.Media[0].AltText); alt != "" {
				ui.Text(c, alt).FontSize(DetailAltFont).TextColor(theme.TextMuted).
					Padding(DetailAltGap, 0, 0, 0)
			}
		}

		s.detailMetrics(c, rail, row)
	})
}

// detailMedia draws the post's pictures: one full width, two or more as a two-column grid,
// which is the shape a tweet's attachment has. Every picture is the lightbox's trigger for
// its own index, so the second picture of a pair is reachable by pressing it rather than by
// opening the first and stepping right.
func (s *Shell) detailMedia(c *ui.Context, theme *ui.Theme, media []domain.Media) {
	count := len(media)
	if count > DetailMediaMaxCount {
		count = DetailMediaMaxCount
	}
	if count == 1 {
		// A lonely picture keeps its whole frame. When the cap makes it narrower than the pane,
		// the row is what centres it; the box itself is never stretched.
		ui.Row(c).FillWidth().Justify(ui.Center).Children(func() {
			s.detailMediaItem(c, theme, 0, media[0], 1)
		})
		return
	}
	ui.Column(c).FillWidth().GapY(DetailMediaGridGap).Children(func() {
		for start := 0; start < count; start += DetailMediaColumns {
			ui.Row(c).FillWidth().GapX(DetailMediaGridGap).Children(func() {
				for i := start; i < start+DetailMediaColumns && i < count; i++ {
					s.detailMediaItem(c, theme, i, media[i], DetailMediaColumns)
				}
				// An odd last picture keeps the width it had in the grid rather than growing
				// to full width: the row is a set, and a lone picture that fills it reads as
				// a different kind of attachment.
				if start+DetailMediaColumns > count {
					ui.Spacer(c).Grow(1)
				}
			})
		}
	})
}

// detailMediaItem draws one picture, with a play mark over a video.
//
// One picture keeps its own shape: the height is capped rather than the width, so a portrait
// photo is scaled down whole and centred instead of being cropped to a band -- the pane shows
// the frame the lightbox will open. The ratio is still clamped at both ends: past 4:5 or
// 3.2:1 the frame would be a 236-point sliver or a 227-point line, and the lightbox is what
// those shapes are for.
//
// In the grid every tile takes one shape, so a row is one height; the pictures' own ratios are
// what the lightbox is for.
func (s *Shell) detailMediaItem(c *ui.Context, theme *ui.Theme, index int, item domain.Media, columns int) {
	url := domain.MediaThumbnailURL([]domain.Media{item})
	box := ui.Box(c).Radius(DetailMediaRadius).Clip().
		Background(theme.Background).
		Label(DetailMediaTriggerLabel).Focusable().
		OnClick(func() { s.OpenLightbox(index) })
	if columns > 1 {
		box = box.Grow(1).Basis(0).MinWidth(0).AspectRatio(DetailMediaTileRatio)
	} else {
		width, height := s.detailPictureFrame(s.detailMediaRatio(url))
		box = box.Width(width).Height(height)
	}
	box.Children(func() {
		if bitmap, ready := s.mediaPicture(url); ready {
			ui.Image(c, bitmap).FillWidth().FillHeight().Fit(ui.Cover)
		}
		// IsVideoMediaType, not a comparison with MediaVideo: X serves an animated GIF as an
		// mp4, and IsVideoMediaType covers both. A GIF compared as a photo got a still poster
		// and no play mark.
		if domain.IsVideoMediaType(string(item.Type)) {
			s.playMark(c, theme, DetailPlaySize, DetailPlayMarkLabel)
		}
	})
}

// detailPictureFrame is the box a post's only picture takes.
//
// The width is the pane's, so a wide picture fills the column; the height is capped at
// DetailMediaCap, so a tall one is scaled down whole. The two numbers always agree with the
// picture's ratio, which is why the crop anchor the capped-band layout needed does not exist
// here: there is no crop to anchor.
func (s *Shell) detailPictureFrame(ratio float32) (float32, float32) {
	width := s.detailPaneWidth()
	height := width / ratio
	if height > DetailMediaCap {
		height = DetailMediaCap
		width = height * ratio
	}
	return width, height
}

// detailMediaRatio is a picture's width over its height, clamped to what the pane will show.
//
// A picture whose size is not known yet gets 16:9, which is the box the placeholder takes:
// when the picture lands the frame changes once, in the same frame it appears, rather than
// growing from nothing.
func (s *Shell) detailMediaRatio(url string) float32 {
	const fallback = 16.0 / 9.0
	bitmap, ready := s.mediaPicture(url)
	if !ready {
		return fallback
	}
	width, height := bitmap.Size()
	if width <= 0 || height <= 0 {
		return fallback
	}
	ratio := float32(width) / float32(height)
	if ratio < DetailMediaMinRatio {
		ratio = DetailMediaMinRatio
	}
	if ratio > DetailMediaMaxRatio {
		ratio = DetailMediaMaxRatio
	}
	return ratio
}

// detailMetrics is what the post got, in the quiet ink under it.
//
// The card's list layout already draws these three counts, and the detail drew none of them:
// the same post said "42 replies" in one column and nothing in the one that had room for it.
// A nil Metrics still means "the API returned no counts", which is a different statement from
// three zeroes, so nothing is drawn rather than "0 0 0".
func (s *Shell) detailMetrics(c *ui.Context, rail RailTokens, row workspace.BookmarkRow) {
	if row.Metrics == nil {
		return
	}
	ui.Row(c).GapX(DetailMetricGap).Padding(DetailMetricsPadTop, 0, 0, 0).
		AlignItems(ui.Center).Children(func() {
		for _, metric := range []struct {
			icon  string
			value int
		}{
			{"comment", derefOrZero(row.Metrics.Replies)},
			{"share", derefOrZero(row.Metrics.Retweets)},
			{"heart", derefOrZero(row.Metrics.Likes)},
		} {
			ui.Row(c).GapX(DetailMetricInnerGap).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Width(DetailMetricIcon).Height(DetailMetricIcon).
					Draw(func(p *ui.Painter, r ui.Rect) {
						drawIcon(p, metric.icon, r.X, r.Y, min32(r.W, r.H), rail.TextTertiary)
					})
				ui.Text(c, domain.FormatCompactCount(metric.value, string(s.t().Locale()))).
					FontSize(DetailMetricFont).FontWeight(600).TextColor(rail.TextTertiary)
			})
		}
	})
}
