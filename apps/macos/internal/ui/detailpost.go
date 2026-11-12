package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/x-master/apps/macos/internal/domain"
	"github.com/taoquo/x-master/apps/macos/internal/usecase/workspace"
)

// The detail's reading pane is the post, not a set of sections (shell-contract.md S41).
//
// The pane used to be a "内容摘要" card with a quote glyph, then a "媒体资源" section below it:
// two labelled blocks with a 22-point gap, each with its own overline, for one tweet. The
// labels were filing marks on content that has one owner, and the card was a third panel
// inside a dialog panel. The source's own card draws the text and its media as one thing
// (BookmarkCard.tsx), and this is the detail of that card, so it has the same shape: words,
// then the pictures the words are about, then what the post got.

// The video mark's ground and ink: the lightbox controls' own pair (a dark translucent disc
// over the picture, a white glyph), so a video's poster and the player it opens read as one
// surface.
var (
	DetailPlayGround = ui.RGBA(25, 21, 20, 0.55)
	DetailPlayInk    = ui.RGBA(255, 255, 255, 1)
)

// detailPost draws the selected post: its words, its media, and its counts.
func (s *Shell) detailPost(c *ui.Context, theme *ui.Theme, rail RailTokens, row workspace.BookmarkRow) {
	ui.Column(c).Grow(1).Children(func() {
		if text := strings.TrimSpace(row.Text); text != "" {
			ui.Text(c, row.Text).FontSize(DetailBodyFont).LineHeight(DetailBodyLine).
				TextColor(theme.Text)
		}

		if len(row.Media) > 0 {
			// The pictures belong to the sentence above them, so the gap is a paragraph's, not
			// a section's.
			ui.Column(c).FillWidth().Padding(DetailPostGap, 0, 0, 0).Children(func() {
				s.detailMedia(c, theme, row.Media)
			})
			// A picture's description, where the source puts it: under the picture, in the
			// quiet ink. It is the only text in the detail that is about the media rather
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
		s.detailMediaItem(c, theme, 0, media[0], 1)
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

// detailMediaItem draws one picture at its own aspect ratio, with a play mark over a video.
//
// The height comes from the picture rather than from the stylesheet's clamp: a fixed
// 15-24rem band cropped every portrait photo to a strip, while the lightbox showed the same
// photo whole. The ratio is clamped at both ends so one tall picture cannot take the whole
// pane and one panorama does not become a line; beyond the clamp the crop is the point.
func (s *Shell) detailMediaItem(c *ui.Context, theme *ui.Theme, index int, item domain.Media, columns int) {
	url := domain.MediaThumbnailURL([]domain.Media{item})
	box := ui.Box(c).Radius(DetailMediaRadius).Clip().
		AspectRatio(s.detailMediaRatio(url)).
		Background(theme.Background).
		Label(DetailMediaTriggerLabel).Focusable().
		OnClick(func() { s.OpenLightbox(index) })
	if columns > 1 {
		box = box.Grow(1).Basis(0).MinWidth(0)
	} else {
		box = box.FillWidth()
	}
	box.Children(func() {
		if bitmap, ready := s.mediaPicture(url); ready {
			ui.Image(c, bitmap).FillWidth().FillHeight().Fit(ui.Cover)
		}
		// IsVideoMediaType, not a comparison with MediaVideo: X serves an animated GIF as an
		// mp4, and the source's isVideoMediaType covers both (BookmarkInspector.tsx:20-22).
		// A GIF compared as a photo got a still poster and no play mark.
		if domain.IsVideoMediaType(string(item.Type)) {
			s.detailPlayMark(c)
		}
	})
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

// detailPlayMark is the disc over a video's poster. It is a mark, not a control: the item
// under it opens the lightbox, and the lightbox is where the player lives, so the mark says
// "this one moves" and the press that follows is the same press for every picture.
func (s *Shell) detailPlayMark(c *ui.Context) {
	ui.Box(c).Absolute().Left(0).Top(0).Right(0).Bottom(0).Center().PassThrough().Children(func() {
		ui.Box(c).Width(DetailPlaySize).Height(DetailPlaySize).Radius(RadiusPill).
			Background(DetailPlayGround).Center().Children(func() {
			ui.Box(c).Width(DetailPlayGlyph).Height(DetailPlayGlyph).
				Draw(func(p *ui.Painter, r ui.Rect) {
					drawIcon(p, "play", r.X, r.Y, min32(r.W, r.H), DetailPlayInk)
				})
		})
	})
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
