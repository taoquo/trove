package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/x-master/apps/macos/internal/i18n"
)

// The relation graph as a page.
//
// Third pass at this page (S57). The first drew the source's ring, which a real library cannot
// read; the second laid it out natively but left the map in a square at the middle of a wide
// pale canvas, named twenty captions at an opacity the reader could not read, and put the focus
// panel in a 90-point strip at the bottom. The reader's report: "是不是很乱？字也小，不知道要
// 体现什么内容，下面的区域也被严重的挤压".
//
// So the page is the source's shell (`.options-tag-graph-shell`, extension.css:4416-4421): one
// panel holding the head, the map and the focus panel, 14 of padding and 12 between them. The
// map takes every point the panel does not, one line says what the marks mean, and the focus
// panel is a card of its own with the sizes the source gives it (extension.css:4521-4530).

const (
	// GraphCardRadius and GraphCardPad are the source's shell: `border-radius: 16px;
	// padding: 14px` with `gap: 12px` (extension.css:4416-4421).
	GraphCardRadius = 16
	GraphCardPad    = 14
	GraphCardGap    = 12
	// GraphHeadMinHeight is `.options-tag-graph-head`'s `min-height: 30px` (extension.css:4424).
	GraphHeadMinHeight = 30
	// GraphCanvasMinHeight is what the map keeps when the focus panel is open: below it the
	// plot is too small to be a map, and the panel is what a focused reader came for.
	GraphCanvasMinHeight = 240
	// The focus panel: `padding: 12px`, `gap: 6px`, radius 14 and a 26% accent border
	// (extension.css:4521-4530).
	GraphFocusPad    = 12
	GraphFocusGap    = 6
	GraphFocusRadius = 14
	// GraphFocusBorderAlpha is `color-mix(in srgb, var(--accent-primary) 26%, transparent)`.
	GraphFocusBorderAlpha = 0.26
	// GraphFocusMinHeight is what the panel's three lines and its action need at these sizes:
	// 12 of padding twice, 6 between four rows, a 13-point name, two 12-point lines and the
	// action. It was about 90 when the panel was a strip, which is what "被严重的挤压" was.
	GraphFocusMinHeight = 100
)

// graphPage draws the relation graph as a page.
//
// It wears the settings and tag pages' head: the way out, then the page's name (S46). The
// graph replaced the results pane and took the toolbar's own graph switch with it, so the
// page had no visible exit at all -- Escape worked, but nothing on screen said so. The
// switch that opened it is one square; the square that closes it is the same one the two
// other full-page surfaces use.
func (s *Shell) graphPage(c *ui.Context, theme *ui.Theme, rail RailTokens) {
	inset := s.pageInset(c)
	ui.Column(c).Fill().Background(rail.Results).
		Padding(ResultsPadTop, inset, ResultsPadBottom, inset).GapY(SpaceBase).Children(func() {
		s.graphPageHead(c, theme, rail)

		// Nothing to draw yet. The graph is built off the window's thread, so the first
		// frame after the switch has no model; saying "no relations" then would be a
		// lie about a library that is merely still being read.
		if len(s.state.GraphModel.Nodes) == 0 {
			if s.state.GraphLoading {
				ui.Column(c).Fill().Center().Padding(48).Children(func() {
					ui.Text(c, s.t().T(i18n.GraphBuildingTitle)).
						FontSize(FontBodyLarge).TextColor(theme.TextMuted)
				})
				return
			}
			// magpie's empty state: the title in the page's ink at weight 500, the advice in
			// --muted under it (app.css:651-653).
			ui.Column(c).Fill().Center().Padding(36, 20).GapY(SpaceSmall).Children(func() {
				ui.Text(c, s.t().T(i18n.GraphEmptyTitle)).
					FontSize(FontBody).FontWeight(500).TextColor(theme.Text)
				ui.Text(c, s.t().T(i18n.GraphEmptyDescription)).
					FontSize(FontSmall).TextColor(theme.TextMuted)
			})
			return
		}

		ui.Column(c).FillWidth().Grow(1).GapY(GraphCardGap).
			Padding(GraphCardPad).Radius(GraphCardRadius).
			Background(theme.Surface).Border(1, rail.SubtleLine).Children(func() {
			s.graphHead(c, theme, rail)
			// What the marks mean, in one line. The source has no legend -- it draws the same
			// two encodings and leaves them unexplained -- and "不知道要体现什么内容" is
			// exactly what that costs. `line weight` follows the source's own stroke rule
			// (1 + weight * 4, TagGraphView.tsx:126).
			ui.Text(c, s.t().T(i18n.GraphLegend)).
				FontSize(FontTiny).TextColor(rail.TextTertiary)

			s.GraphView(c, s.state.GraphModel, s.focusedGraphTag()).
				FillWidth().Grow(1).MinHeight(GraphCanvasMinHeight)

			s.graphFocusPanel(c, theme, rail)
		})
	})
}

// graphPageHead is the way out and the page's name.
func (s *Shell) graphPageHead(c *ui.Context, theme *ui.Theme, rail RailTokens) {
	ui.Row(c).FillWidth().MinHeight(SettingsPageHeaderHeight).
		AlignItems(ui.Center).Children(func() {
		back := ui.Box(c).Width(SettingsBackControl).Height(SettingsBackControl).
			Radius(RadiusSmall).Center().Focusable().
			Label(s.t().T(i18n.TagExitManagement)).
			Tooltip(s.t().T(i18n.TagExitManagement))
		back = back.Background(hoverColor(c, back, "graph-back", ui.Transparent,
			theme.SurfaceHover, back.Hovered()))
		back.OnClick(func() { s.ToggleGraph(context.Background()) }).
			Draw(func(p *ui.Painter, r ui.Rect) {
				side := min32(r.W, r.H) - 12
				drawIcon(p, "back", r.X+(r.W-side)/2, r.Y+(r.H-side)/2, side, rail.TextTertiary)
			})
		ui.Text(c, s.t().T(i18n.ToolbarGraph)).
			FontSize(FontSidebarTitleCompact).FontWeight(600).TextColor(theme.Text).
			Padding(0, 10, 0, 4)
		ui.Spacer(c).FillWidth()
	})
}

// graphHead is the summary and the graph's two controls.
//
// `.options-tag-graph-head`: one line, space between, a 30-point floor (extension.css:4424-4436).
func (s *Shell) graphHead(c *ui.Context, theme *ui.Theme, rail RailTokens) {
	ui.Row(c).FillWidth().MinHeight(GraphHeadMinHeight).
		AlignItems(ui.Center).GapX(SpaceBase).Children(func() {
		ui.Text(c, s.t().Tf(i18n.GraphSummary,
			len(s.state.GraphModel.Nodes), len(s.state.GraphModel.Edges))).
			FontSize(FontSmall).TextColor(theme.TextMuted).Grow(1)
		if hidden := s.state.GraphModel.HiddenTagCount; hidden > 0 {
			ui.Text(c, s.t().Tf(i18n.GraphHidden, hidden)).
				FontSize(FontTiny).TextColor(rail.TextTertiary)
		}
		// The author layer's own count, shown when it is on. The source composes this as
		// `{authorNodes.length} {suffix}` (TagGraphView.tsx:99), and the suffix has no verb in
		// either locale: formatting it as one printed "位作者节点 %!(EXTRA int=18)".
		if s.state.GraphIncludeAuthors {
			ui.Text(c, fmt.Sprintf("%d %s", len(s.state.GraphModel.Authors),
				s.t().T(i18n.GraphAuthorNodesSuffix))).
				FontSize(FontTiny).TextColor(rail.TextTertiary)
		}
		// The source's two graph controls: show the author nodes, and drop the focus a
		// tag carries.
		IconChip(c, theme, rail, "graph", s.t().T(i18n.GraphShowAuthors), func() {
			s.ToggleGraphAuthors(context.Background())
		})
		if s.focusedGraphTag() != "" {
			IconChip(c, theme, rail, "close", s.t().T(i18n.GraphClearFocus), func() {
				s.ClearGraphFocus(context.Background())
			})
		}
	})
}

// graphFocusPanel is what the focused tag holds, who writes it, and the way to its bookmarks.
//
// The source's `.options-tag-graph-focus`: its own card on the selected ground, a 26% accent
// border, 12 of padding and 6 between the lines (extension.css:4521-4530). It was 8 of padding
// and FontTiny on the page's own ground, which is the strip the reader called squeezed.
func (s *Shell) graphFocusPanel(c *ui.Context, theme *ui.Theme, rail RailTokens) {
	node, found := s.focusedGraphNode()
	if !found {
		return
	}

	ui.Column(c).FillWidth().GapY(GraphFocusGap).Padding(GraphFocusPad).
		Radius(GraphFocusRadius).Background(rail.SelectedBg).
		Border(1, theme.Accent.Alpha(GraphFocusBorderAlpha)).
		Label(s.t().T(i18n.GraphFocusPanel)).Children(func() {
		ui.Text(c, node.Name).FontSize(FontBody).FontWeight(600).TextColor(theme.Text)
		ui.Text(c, fmt.Sprintf("%d %s · %d %s",
			node.BookmarkCount, s.t().T(i18n.TagTableBookmarks),
			node.AuthorCount, s.t().T(i18n.GraphAuthorsSuffix))).
			FontSize(FontSmall).TextColor(theme.TextMuted)

		if len(node.TopAuthors) > 0 {
			handles := make([]string, 0, len(node.TopAuthors))
			for _, author := range node.TopAuthors {
				handles = append(handles, fmt.Sprintf("@%s (%d)",
					author.AuthorHandle, author.Count))
			}
			ui.Text(c, strings.Join(handles, ", ")).
				FontSize(FontSmall).TextColor(rail.TextTertiary)
		}

		ui.Row(c).FillWidth().Children(func() {
			settingsAction(c, theme, rail, s.t().T(i18n.GraphOpenBookmarks),
				false, false, func() { s.OpenFocusedGraphTagBookmarks() })
			ui.Spacer(c).FillWidth()
		})
	})
}
