package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/x-master/apps/macos/internal/domain"
	"github.com/taoquo/x-master/apps/macos/internal/i18n"
)

// The tag management page's header band: the way out, the screen's name, the switch between
// its two pages, and the library's health in one line. It is split from taglibrary.go because
// bigfiles.py put that file over its 900-line cap, and this band is the one block on the page
// that draws nothing the table or the toolbar uses.
//
// docs/shell-contract.md S27 records why the band is compact, and S47 the move to the settings
// page's head.

// tagLibraryHeader is the page heading, in the settings page's grammar (S46): one 34-point
// row with the back square, the screen's name and the two pages as `.warm-tabs`, then the
// library's health as the one quiet line under it.
//
// The screen's name is 管理标签 on both pages; the tabs say which page is open. Naming the
// page in the title and again in the switch was the same word twice, and the breadcrumb line
// it replaced made the band three rows tall before the table's own toolbar.
func (s *Shell) tagLibraryHeader(c *ui.Context, theme *ui.Theme, rail RailTokens) {
	ui.Column(c).FillWidth().GapY(4).Children(func() {
		ui.Row(c).FillWidth().MinHeight(SettingsPageHeaderHeight).
			AlignItems(ui.Center).Children(func() {
			s.tagLibraryBack(c, theme, rail)

			ui.Text(c, s.t().T(i18n.TagManageLabel)).
				FontSize(FontSidebarTitleCompact).FontWeight(600).TextColor(theme.Text).
				Padding(0, 10, 0, 4)

			s.tagLibraryTabs(c, theme, rail)
			ui.Spacer(c).FillWidth()
		})

		ui.Text(c, s.tagLibraryStatsLine(s.state.TagItems)).FontSize(FontSmall).
			TextColor(rail.TextTertiary).MaxLines(1).Padding(0, 2)
	})
}

// tagLibraryBack is the way out of tag management: one square, as the settings page's back is,
// and for the same reason -- the page name beside it already says where the reader is.
//
// It used to be a `← 返回书签` breadcrumb on its own line. TagMode has no window chrome of its
// own, so this is still the only way out; the words are now the control's label and tooltip.
func (s *Shell) tagLibraryBack(c *ui.Context, theme *ui.Theme, rail RailTokens) {
	back := ui.Box(c).Width(SettingsBackControl).Height(SettingsBackControl).
		Radius(RadiusSmall).Center().Focusable().
		Label(s.t().T(i18n.TagExitManagement)).
		Tooltip(s.t().T(i18n.TagExitManagement))
	back = back.Background(hoverColor(c, back, "tag-back", ui.Transparent, theme.SurfaceHover, back.Hovered()))
	back.OnClick(func() { s.ToggleTagMode(context.Background()) }).Draw(func(p *ui.Painter, r ui.Rect) {
		side := min32(r.W, r.H) - 12
		drawIcon(p, "back", r.X+(r.W-side)/2, r.Y+(r.H-side)/2, side, rail.TextTertiary)
	})
}

// tagLibraryStatsLine is the library's health in one line: the counts the source splits over
// two, and the size verdict. The second line was there to hold the verdict, which is one word.
func (s *Shell) tagLibraryStatsLine(all []domain.TagLibraryItem) string {
	health := i18n.TagHealthOk
	if len(all) > HealthyTagCeiling() {
		health = i18n.TagHealthTooMany
	}
	return strings.Join([]string{
		fmt.Sprintf("%d %s", len(all), s.t().T(i18n.TagTagsSuffix)),
		fmt.Sprintf("%d %s", domain.CountOrphanTags(all), s.t().T(i18n.TagOrphanSuffix)),
		fmt.Sprintf("%d %s", domain.CountUnparentedTags(all), s.t().T(i18n.TagUnparentedSuffix)),
		fmt.Sprintf("%d %s", domain.CountLongTailTags(all), s.t().T(i18n.TagLongTailSuffix)),
		s.t().T(health),
	}, " · ")
}

// tagLibraryTabs is the 标签 / 自动标签 switch: `.warm-tabs`, the same control the settings
// page's sections use.
//
// The dot on 自动标签 is the only status the tags page spends on the other page: a run in
// flight or suggestions waiting are things the reader should notice without opening it, and a
// 6-point dot costs no row. Nothing else from the auto-tagging page leaks into the table.
func (s *Shell) tagLibraryTabs(c *ui.Context, theme *ui.Theme, rail RailTokens) {
	ui.Row(c).GapX(SettingsTabGap).AlignItems(ui.Center).Children(func() {
		warmTab(c, theme, rail, s.t().T(i18n.SidebarTags),
			s.tagView() == TagViewTags, false, func() { s.selectTagLibraryView(TagViewTags) })
		warmTab(c, theme, rail, s.t().T(i18n.SettingsSectionAutoTag),
			s.tagView() == TagViewAutoTag,
			s.tagView() != TagViewAutoTag && s.autoTagNeedsAttention(),
			func() { s.selectTagLibraryView(TagViewAutoTag) })
	})
}

// selectTagLibraryView switches the page body.
//
// A view change only: the table's query, filter, sort, selection and focused row are all
// left alone, so going to the auto-tagging page and back costs one click and no re-finding.
func (s *Shell) selectTagLibraryView(view string) {
	if view != TagViewAutoTag {
		view = TagViewTags
	}
	s.state.TagView = view
	if view == TagViewAutoTag {
		// This page is the only home of the auto-tag configuration now, so entering it has to
		// read the stored values -- the settings sheet used to do that on open, and without it
		// the page would draw zeroes and the first toggle would write them over the real
		// configuration (shell-contract.md S26).
		s.loadAutoTagSettings(context.Background())
		// The run result's "N pending" is the reason to come here, and the count is cheap to
		// read. Reload's copy is fresh after a run; this covers suggestions accepted elsewhere.
		s.ReloadPendingSuggestions(context.Background())
	}
	if s.window != nil {
		s.window.Invalidate()
	}
}

// autoTagNeedsAttention reports whether the tags page should mark the auto-tagging page: a run
// in flight, or suggestions it produced that are still waiting.
func (s *Shell) autoTagNeedsAttention() bool {
	return s.state.AutoTagRunning || len(s.state.PendingSuggestions) > 0
}
