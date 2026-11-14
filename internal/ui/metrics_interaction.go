package ui

// The interaction pass's own numbers.
//
// They live apart from metrics.go only because that file is at its size cap; the reasoning
// for each number is with it. The groups are the keyboard's step, the folded sidebar, and the
// settings page.

// ResultsKeyboardPage is how far PageUp and PageDown move the selection.
//
// The browser's own page key scrolls by the viewport; a card is roughly 260 points high, so
// eight cards is a screen at the default window height. It is a fixed step rather than a
// measured one because the selection must land on a card, not at a scroll offset, and the
// arrow keys already reveal whatever they select.
const ResultsKeyboardPage = 8

// The folded sidebar.
//
// The column is hidden rather than folded to an icon rail: a drag hides at
// `SIDE_MIN - 64`, and the header's button brings it back. There is no rail, so none is
// drawn.
const (
	// SidebarCollapseDrag is how far past the minimum a drag has to go before the
	// column hides instead of stopping at 168: a drag of a few points is someone
	// adjusting, a drag of 64 is someone putting it away (168 - 64 = 104, the
	// threshold).
	SidebarCollapseDrag = 64
)

// The window's own title bar.
//
// The app draws it instead of the system's: the top row is 50 points tall with
// 96 points of left padding for the traffic lights, and the sidebar's top band
// is the same row. The numbers here are that row's.
const (
	// TitleBarHeight is the band the window's controls sit in. It is the stylesheet's 50,
	// not the system's 28: the row carries the brand and the sidebar's button, and 28 points
	// is a strip rather than a header.
	TitleBarHeight = 50
	// TitleBarLightsGap is the space between the traffic lights and the first control. A
	// Mac's three buttons end 72 points from the window's edge and the row starts at
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
	// TitleBarBrandGap is the space between the button, the mark and the name. Both the
	// brand and the sidebar's top row use 8.
	TitleBarBrandGap = 8
	// TitleBarCrumbPad is the space between the sidebar's edge and the page's crumb:
	// 20 points. The crumb names the page now that the content has no heading of its own.
	TitleBarCrumbPad = 20
	// TitleBarBrandMinText is the least width worth drawing a wordmark in: narrower than this
	// the name would be an ellipsis, so the mark stands alone as it does in a tight sidebar.
	TitleBarBrandMinText = 48
	// TitleBarBrandFoldedWidth is the room the name gets when the brand sits over the
	// results: enough for the full title, and bounded so a long translation cannot push the
	// status badge off the bar.
	TitleBarBrandFoldedWidth = 240
	// SidebarGlyphPane is the alpha of the glyph's left pane while the column is open.
	// It is also the target of the 300ms fade that reports the fold.
	SidebarGlyphPane = 0.32
)

// The settings page, rebuilt as cards of preference rows.
//
// The numbers are the settings page's own:
//
//	list card    12px radius, the card ground, one hairline edge
//	a preference row  48 high, 12/12/12/14 inset, 14 gap, the subtle hairline between rows
//	a row head   26 high, 4 above and 6 below
//	the find field  180 wide, 120 at least, 26 high
const (
	// FontPageTitle stays for the pages that still name themselves at display size; the
	// settings page's own name is set at the sidebar-title step, beside its tabs.
	FontPageTitle = 17

	SettingsPageHeaderHeight = 34
	// The page's own breathing room, 8 above and 8 below.
	SettingsPagePadTop    = 8
	SettingsPagePadBottom = 8
	// SettingsBackControl is the way back's square: 26.
	SettingsBackControl = 26

	// The head's tabs: 22 high, 0 7 inset, a 6 radius and an
	// 11px 600 .05em uppercase label; the chosen one takes the tray ground and nothing else.
	SettingsTabHeight = 22
	SettingsTabPadX   = 7
	SettingsTabRadius = 6
	SettingsTabGap    = 2
	SettingsTabFont   = 11

	// The find field: 26 high, a 7 radius, 9 inset and a
	// 180-point starting width.
	SettingsFindHeight = 26
	SettingsFindRadius = 7
	SettingsFindPadX   = 9
	SettingsFindWidth  = 180

	// One preference row: 48 high, 12px 12px 12px 14px inset, a 14px gap,
	// the name at the body's 13px in weight 500, the
	// sub-line at 11.5px in the muted ink, wrapping rather than cut.
	SettingsRowHeight   = 48
	SettingsRowPadY     = 12
	SettingsRowPadLeft  = 14
	SettingsRowPadRight = 12
	SettingsRowGap      = 14
	SettingsRowNameFont = 13
	SettingsRowSubFont  = 11.5
	// SettingsCardGap is the 8 between a card and the next.
	SettingsCardGap = 8

	// A pick: 24 high, 0 6 0 9 inset, a 7 radius and a 12px label on the tray ground.
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

	// A labelled action: 27 high, 10-point side padding, a 6 radius, a 12px label, the
	// muted ink and a hairline edge over the card.
	SettingsActionHeight = 27
	SettingsActionPadX   = 10
	SettingsActionRadius = 6
	SettingsActionFont   = 12
	SettingsActionGap    = 6
	SettingsActionIcon   = 14

	// A field in a settings row: 27 high, a 7 radius, 0 9 inset and a
	// 12.5px label. A number is 24 high, 0 8 inset,
	// 72 wide, its value to the right.
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
	// SettingsIconSize is a row's quiet icon action: 26 square.
	SettingsIconSize = 26
)
