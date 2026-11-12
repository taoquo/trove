package ui

// The interaction pass's own numbers (docs/interaction-parity-plan.md P2.1-P2.6).
//
// They live apart from metrics.go only because that file is at its size cap; the reasoning
// for each number is with it, and the groups are the same three the pass added: the
// keyboard's step, the folded sidebar, and the settings page.

// ResultsKeyboardPage is how far PageUp and PageDown move the selection.
//
// The browser's own page key scrolls by the viewport; a card is roughly 260 points high, so
// eight cards is a screen at the default window height. It is a fixed step rather than a
// measured one because the selection must land on a card, not at a scroll offset, and the
// arrow keys already reveal whatever they select.
const ResultsKeyboardPage = 8

// The folded sidebar.
//
// magpie hides the column rather than folding it to an icon rail: its drag hides at
// `SIDE_MIN - 64` (app.js:23614), and the header's button brings it back. There is no
// rail in the source, so this build no longer draws one.
const (
	// SidebarCollapseDrag is how far past the minimum a drag has to go before the
	// column hides instead of stopping at 168: a drag of a few points is someone
	// adjusting, a drag of 64 is someone putting it away (168 - 64 = 104, magpie's own
	// threshold).
	SidebarCollapseDrag = 64
)

// The window's own title bar (shell-contract.md S38).
//
// magpie draws it instead of the system's: `body.window.mac .top` is 50 points tall with
// 96 points of left padding for the traffic lights (app.css:210), and its sidebar's top band
// is the same row (app.css:391). The numbers here are that row's.
const (
	// TitleBarHeight is the band the window's controls sit in. It is the stylesheet's 50,
	// not the system's 28: the row carries the brand and the sidebar's button, and 28 points
	// is a strip rather than a header.
	TitleBarHeight = 50
	// TitleBarLightsGap is the space between the traffic lights and the first control. A
	// Mac's three buttons end 72 points from the window's edge and magpie starts its row at
	// 96, so this is the 24 it leaves.
	TitleBarLightsGap = 24
	// TitleBarFlush is the left inset where there are no window controls to clear -- full
	// screen, a headless capture, the tests. It is also the floor when a folded rail puts the
	// row's own edge past the controls' inset.
	TitleBarFlush = 14
	// TitleBarPadRight keeps the status badge off the window's edge.
	TitleBarPadRight = 16
	// TitleBarButton is the sidebar button's square. It is larger than the 16-point glyph so
	// the target is a comfortable one, and small enough to read as part of the brand row.
	TitleBarButton = 26
	// TitleBarBrandGap is the space between the button, the mark and the name. magpie's
	// `.brand` and `.side-top` both use 8 (app.css:385).
	TitleBarBrandGap = 8
	// TitleBarCrumbPad is the space between the sidebar's edge and the page's crumb:
	// `body.side-nav .top { padding-left: 20px }` (app.css:522-524). The crumb names the page
	// now that the content has no heading of its own.
	TitleBarCrumbPad = 20
	// TitleBarBrandMinText is the least width worth drawing a wordmark in: narrower than this
	// the name would be an ellipsis, so the mark stands alone as it does in a tight sidebar
	// (app.css:388).
	TitleBarBrandMinText = 48
	// TitleBarBrandFoldedWidth is the room the name gets when the brand sits over the
	// results: enough for the full title, and bounded so a long translation cannot push the
	// status badge off the bar.
	TitleBarBrandFoldedWidth = 240
	// SidebarGlyphPane is the alpha of the glyph's left pane while the column is open
	// (index.html:58). It is also the target of the 300ms fade that reports the fold.
	SidebarGlyphPane = 0.32
)

// The settings page, which replaced the 820x660 sheet (shell-contract.md S37), rebuilt as
// magpie's cards of preference rows (S46).
//
// The numbers are magpie's own, from the `/* ---------- settings ---------- */` block and the
// row rules it sits on (app.css:630-646, 2135-2140, 3013-3023, 3368-3396, 3632-3662):
//
//	.list        12px radius, --card ground, one --line edge
//	.row.pref    48 high, 12/12/12/14 inset, 14 gap, --line-2 between rows
//	.row-head    26 high, 4 above and 6 below
//	.set-q       180 wide, 120 at least, 26 high
const (
	// FontPageTitle stays for the pages that still name themselves at display size; the
	// settings page's own name is set at the sidebar-title step, beside its tabs.
	FontPageTitle = 17

	SettingsPageHeaderHeight = 34
	// The page's own breathing room, 8 above and 8 below, as magpie's settings view has
	// (padding-top 8 at app.css:511, bottom 8 at app.css:602). It was 12/24 from the
	// extension dialog this page replaced.
	SettingsPagePadTop    = 8
	SettingsPagePadBottom = 8
	// SettingsBackControl is the way back's square: magpie's `.icon` is 26.
	SettingsBackControl = 26

	// The head's tabs are `.warm-tabs button`: 22 high, 0 7 inset, a 6 radius and an
	// 11px 600 .05em uppercase label; the chosen one takes --pill and nothing else.
	SettingsTabHeight = 22
	SettingsTabPadX   = 7
	SettingsTabRadius = 6
	SettingsTabGap    = 2
	SettingsTabFont   = 11

	// The find field is `.sess-filter` / `.set-q`: 26 high, a 7 radius, 9 inset and a
	// 180-point starting width.
	SettingsFindHeight = 26
	SettingsFindRadius = 7
	SettingsFindPadX   = 9
	SettingsFindWidth  = 180

	// One preference row: `.row.pref, body.window .row.pref { min-height: 48px; padding:
	// 12px 12px 12px 14px; gap: 14px }`, the name at the body's 13px in weight 500, the
	// sub-line at 11.5px in --fg-2, wrapping rather than cut (app.css:3785-3791).
	SettingsRowHeight   = 48
	SettingsRowPadY     = 12
	SettingsRowPadLeft  = 14
	SettingsRowPadRight = 12
	SettingsRowGap      = 14
	SettingsRowNameFont = 13
	SettingsRowSubFont  = 11.5
	// SettingsCardGap is the 8 between a card and the next (`.row-head + .list`,
	// `.set-page > .list:not(:last-child)`).
	SettingsCardGap = 8

	// A pick: `.row.pref .proto.pick` is 24 high, `padding: 0 6px 0 9px`, a 7 radius and
	// a 12px label on --pill.
	SettingsPickHeight      = 24
	SettingsPickPadLeft     = 9
	SettingsPickPadRight    = 6
	SettingsPickRadius      = 7
	SettingsPickFont        = 12
	SettingsSegPad          = 2
	SettingsSegRadius       = 7
	SettingsSegOptionHeight = 22
	SettingsSegOptionPadX   = 10
	SettingsSegOptionRadius = 5
	SettingsSegFont         = 12

	// A labelled action: `.text.action` is 27 high, `padding: 0 10px`, a 6 radius, a 12px
	// label, --fg-2 ink and a --line edge over the card.
	SettingsActionHeight = 27
	SettingsActionPadX   = 10
	SettingsActionRadius = 6
	SettingsActionFont   = 12
	SettingsActionGap    = 6
	SettingsActionIcon   = 14

	// A field in a settings row: `.editor input` is 27 high, a 7 radius, 0 9 inset and a
	// 12.5px label. A number is `.row.pref .set-field[type=number]`: 24 high, 0 8 inset,
	// 72 wide, its value to the right (app.css:3750-3751).
	SettingsFieldHeight  = 27
	SettingsFieldPadX    = 9
	SettingsFieldRadius  = 7
	SettingsFieldFont    = 12.5
	SettingsNumberHeight = 24
	SettingsNumberPadX   = 8
	SettingsNumberWidth  = 72
	// SettingsInputWidth is a name field inside a row, such as a new list's.
	SettingsInputWidth = 180
	// SettingsFieldWide is a row's field for a value longer than a name -- a base URL
	// or a model id -- so it reads at a glance instead of scrolling.
	SettingsFieldWide = 280
	// SettingsIconSize is a row's quiet icon action: magpie's `.icon`, 26 square (app.css:276).
	SettingsIconSize = 26
)
