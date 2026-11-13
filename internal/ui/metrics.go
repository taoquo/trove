package ui

// Layout metrics.
//
// Radii, control heights, type sizes, spacing and motion are what make the interface feel
// like the product rather than merely resemble its palette, so they are collected here with
// the value each was taken from.
//
// They are deliberately not configurable. A user preference for the accent colour
// would be reasonable; a user preference for the corner radius would just be a way
// to make the app look wrong.

const (
	// Radii, from magpie's app.css.
	//
	// magpie's scale is flat: a page is a 12px card, a control is 6-8px, and only pills and
	// avatars go fully round. The counts below are of literal `border-radius` declarations in
	// app.css: 6 (41), 7 (37), 8 (34), 5 (32), 10 (21),
	// 999 (20), 4 (19), 12 (13).
	//
	//   .list, cards           12px  (app.css:633)
	//   .field (a picker)       6px  (app.css:869-877)
	//   .icon                  7px  (app.css:273-278)
	//   .seg button             6px  (app.css:265-267)
	//   .nav-sub > button       6px  (app.css:440)
	//   sidebar nav row         7px  (app.css:409-419)
	//   .chip, .badge pillary 999px  (app.css:2165-2174)
	//   .action-confirm        14px  (app.css:4038)
	RadiusPanel  = 12
	RadiusMedium = 8
	// RadiusSmall is the 6px of a field, a segmented button and a small nav row.
	RadiusSmall = 6
	// RadiusPill is 999px in the stylesheet, which is "as round as it goes". A
	// fixed large value is used because a native radius is not clamped by a box.
	RadiusPill = 999

	// The component radii.
	RadiusSidebarRow  = 7  // sidebar nav rows
	RadiusResultCard  = 12 // .list / a page's card
	RadiusToolbar     = 12 // the toolbar's panel
	RadiusToolbarItem = 8  // a control in the toolbar
	RadiusDialog      = 14 // .action-confirm
	RadiusInspector   = 12
)

const (
	// Control heights, from magpie's app.css.
	//
	// magpie's controls are desktop-compact: a picker is 25px, a text action 27, an icon
	// button 26, a list row 48 in the window and 40 in the tray panel.
	//
	//   .field, .chip, .seg button   24-25px
	//   .text.action, inputs         27px
	//   .icon                        26px
	//   .row (window)                48px  (app.css:636)
	HeightPrimary = 30
	HeightLarge   = 28
	HeightControl = 27
	HeightSmall   = 25
	// HeightIndicator is the 20px dot-and-label row used by the status badges.
	HeightIndicator = 20

	// IconButtonSize is magpie's `.icon`: 26 square (app.css:276). One square for every bare
	// control in the window -- the toolbar's labelled actions keep their own .text.action
	// height -- so the search row, the footer and the detail header all agree.
	IconButtonSize = 26
	// FooterIconSize is the same 26 in the sidebar's foot.
	FooterIconSize = 26
	// The footer chip's `padding: 0 7px` for the locale character.
	FooterChipPadX = 7
	// The foot: `.side-foot { padding: 8px 10px 10px; gap: 1px }` (app.css:397).
	FooterPadTop    = 8
	FooterPadBottom = 10
	FooterPadX      = 10
	FooterGap       = 1
	// `.side-gw` is the state row: 26 high, `padding: 0 9px`, 11.5px, and a 7-point dot 8
	// from the words (app.css:513-515).
	SideFooterStatusHeight = 26
	SideFooterStatusPadX   = 9
	SideFooterStatusGap    = 8
	SideFooterStatusFont   = 11.5
	// A suggestion chip's dashed hairline adds the point above and below that a plain tag's
	// fill does not: the reference draws the card tag suggestion at 22 where the tag in the
	// same row is 20 (measured on the list fixture's third card).
	CardSuggestionBorder = 2
)

const (
	// Type sizes, by role. 12px appears most often in the stylesheet and is the
	// body size for secondary text; 13px is primary body; 14px is emphasised body;
	// 15px is a section title.
	FontMicro     = 10
	FontTiny      = 11
	FontSmall     = 12
	FontBody      = 13
	FontBodyLarge = 14
	FontTitle     = 15
	// The page's own type, from magpie's app.css.
	//
	// magpie is a compact desktop UI with no display heading: body text is 13px
	// (app.css:125-126), a section label is 11px/600/.05em uppercase (app.css:2140), a row's
	// name inherits the 13px and sets 500, a sub-line is 11.5px (app.css:643-646), and the
	// one large number on a page -- a usage total -- is 22px.
	//
	// FontOverline is the section label: 11, not 10, because at 10 the uppercase label was
	// smaller than magpie's own by a point and the tracking made it read as a different scale.
	FontOverline = 11
	// The brand is `.side-top .brand { font-size: 13.5px }` (app.css:386) at weight 600.
	FontSidebarTitle        = 14
	FontSidebarTitleCompact = 13.5
	// There is no 34px display title in magpie's window. A page names itself in the title
	// bar's crumb at 13.5/600 (`.crumb`, app.css:522), so no page heading is drawn in the
	// content at all. FontDisplayTitle stays for the pages that still draw one.
	FontDisplayTitle = 16
	FontPageCrumb    = 13.5
	// FontResultsValue is the large number magpie draws for a total (22px,
	// app.css:1310-1320's `.sub b` family of rules).
	FontResultsValue = 22
	// DisplayTracking is the tight tracking magpie applies to its large numerals.
	ResultsValueTracking = -0.01

	// Sidebar rows: magpie's nav buttons are 30 high with a 7 radius and 13px text
	// (app.css:409-419), and its grouped sub-rows are 26 high (app.css:440).
	NavRowMinHeight = 30
	NavRowRadius    = 7
	NavRowPadY      = 0
	NavRowPadX      = 9
	NavRowFont      = 13
	// The list card: magpie's `.row` is 48 high with 7/12/7/14 padding, a 22px icon, a 13px
	// name and an 11.5px sub (app.css:635-646). The result card's own body is 14.
	CardBodyPadding = 14

	// The bulk actions bar: flex, space-between, a 10 gap, 1px border, a 12 radius and 10 of
	// padding; its meta line is 13px. These are the pending-suggestions bar's own numbers, read
	// from the bar rather than borrowed from the card.
	BulkBarPadding = 10
	BulkBarRadius  = 12
	BulkBarGap     = 10
	// The bulk bar's bottom margin.
	BulkBarMarginBottom = 14
	// The bulk bar's `margin-top: -6px`. It pulls the bar up under the header's own 10 of
	// padding, which is the whole 4-point overlap between
	// the two blocks; without it the bar and every card below it sat 6 points low.
	BulkBarMarginTop = 6
	BulkMetaFont     = 13

	// A labelled action's glyph is 14px; the button's own height and padding are
	// SettingsAction*.
	SecondaryButtonIcon = 14

	// The sidebar's facet tabs are magpie's `.segs`: a 2-point inset in a 7 radius track,
	// each `.opt` 22 high with `padding: 0 10px`, a 5 radius and 12px text -- --muted for the
	// closed tab, --fg for the open one (app.css:3370-3375).
	SidebarTabRowHeight = 26
	SidebarSegRadius    = 7
	SidebarTabHeight    = 22
	SidebarTabRadius    = 5
	SidebarTabPadX      = 10
	SidebarTabGap       = 6
	SidebarTabDot       = 6
	// The "+" and the tag-library control on the tab row are magpie's `.icon`: 26 square with
	// a 15px glyph (app.css:276).
	SidebarHeadControl = 26
	SidebarHeadGlyph   = 15
	// The gap between the tabs and the controls beside them, and the row gap when the two do
	// not fit and the controls wrap under the tabs. 4 is magpie's `.gw-share` row gap for a
	// head that wraps (app.css:2478), so a wrapped head still reads as one block.
	SidebarHeaderGap     = 4
	SidebarHeaderWrapGap = 4
	// SidebarFacetCountsMin is the column width at which a chip's count fits beside the three
	// controls: two chips with a count (135 in the wide locale), the header gap, three
	// 26-point controls and 6 apart (90), and the column's 10-point insets at each side.
	// Below it the count is what gives way -- a chip cut to "标签" is a label the reader can
	// still read, where a row of three buttons 26 points under the chips it acts on is not.
	SidebarFacetCountsMin = 272
	// SidebarGroupContentGap still spaces the search field from the list under it.
	SidebarGroupContentGap = 12
	// SidebarRowsGap keeps the rows from butting against each other; without it the tree is
	// 4 points short per row.
	SidebarRowsGap = 4
	// The navigation row's icon sits 8 from its label, and the row's own 12 gap is between the
	// main cell and the trailing one.
	NavRowIcon    = 16
	NavRowMainGap = 8
	// NavRowGap is the row's own 12 between its main cell and its trailing one.
	NavRowGap = 12
	// NavRowTrailingGap holds the count and the row's two actions 6 apart. The count reads at
	// 12px, and the row's delete button is 22 square; the rename button beside it is the same
	// control.
	NavRowTrailingGap = 6
	NavRowCountFont   = 12
	NavRowAction      = 22
	// A grouped sub-row is what the tag list is: magpie's `.nav-sub > button` is 26 high,
	// `padding: 0 10px`, a 6 radius, 12.5px text, a 6 gap, and an 11px count at the far end
	// (app.css:440-449). The open row is --fg at weight 500; there is no accent in the list.
	SubRowMinHeight = 26
	SubRowRadius    = 6
	SubRowPadX      = 10
	SubRowGap       = 6
	SubRowFont      = 12.5
	SubRowCountFont = 11
	// AuthorAvatarSize is the avatar's square, and it doubles as the nav row's icon, which is
	// why the author rows look taller than the plain ones.
	AuthorAvatarSize = 22
	// The sidebar's find field is `.sess-filter`: 26 high, a 7 radius, a `0 9` inset and a
	// 12px label (app.css:1654).
	SidebarSearchHeight = 26
	SidebarSearchRadius = 7
	SidebarSearchPadX   = 9
	// The glyph inside it, and on the magnifier that opens it: magpie's `.set-q` glyph is 15.
	SidebarSearchGlyph = 15
	// FieldBlock wraps its input in `space-y-2` under an sr-only label, so the field carries
	// 8 points above it that no label accounts for.
	SidebarFieldLabelGap = 8

	// The toolbar's find field: magpie's `.set-q` / `.sess-filter` is 26 high with a `0 9`
	// inset and a 7 radius (app.css:1654, :3750), and its magnifier is a 15px glyph.
	SearchFieldHeight = 26
	SearchFieldGlyph  = 15
	SearchFieldPadX   = 9
	// The card's body gap is 12. Three gaps, each named:
	//   section gap 14   (the card's content)
	//   header gap  12   (the card main's row gap)
	//   footer gap  12   (the card body and layout)
	CardSectionGap = 14
	CardBodyGap    = 12
	// The list re-declares all three as 10px, so the list picks them as a set rather than
	// scaling.
	CardSectionGapList = 10
	CardBodyGapList    = 10
	// The card main is `auto minmax(0,1fr)` with a 10px column gap and `align-items: start` --
	// the avatar is a column of its own beside the name, not a vertically centred ornament for
	// the whole text block.
	CardMainGap = 10
	// With no top media the card's text indent starts under the select toggle instead of
	// beside it.
	CardMainInset = 30

	// The card's select toggle: absolutely placed 18 from the top-left, 20 square, a 1.5px
	// border at 6px radius, and a 13px check once chosen.
	CardSelectSize   = 20
	CardSelectInset  = 18
	CardSelectRadius = 6
	CardSelectBorder = 1.5
	CardSelectCheck  = 13

	// The card's actions: space-between with a 12px gap, 12 of padding above a hairline, a
	// 10px gap inside the leading group, and 32-square buttons carrying a 16px glyph.
	CardActionsPadTop = 12
	// The list layout shrinks the same two things: its action is 28 square and its actions
	// carry 10 above.
	CardListActionSize    = 28
	CardListActionsPadTop = 10
	CardActionsGap        = 12
	CardActionsStartGap   = 10
	CardActionIcon        = 16
	// The card header's gap: 12px on the card and 10px in the list layout.
	CardHeaderGapGrid = 12
	CardHeaderGapList = 10
	// The list layout's own body padding: 12px 14px.
	CardListBodyPadV = 12
	CardListBodyPadH = 14
	// The card's type scale is magpie's row: the name at the page's 13 and weight 500, and
	// everything that qualifies it -- handle, timestamp, counts -- on the 11.5px sub line
	// (`.row .name`, `.row .sub`, app.css:646-649).
	CardAvatarSize     = 28
	CardAvatarFont     = 10
	CardAuthorFont     = 13
	CardHandleFont     = 11.5
	CardCopyFont       = 13
	CardCopyLineHeight = 1.45
	CardStatFont       = 11.5
	// The list's stat row keeps its own spacing -- a 4px inner gap, an 8px gap between
	// stats, and a bare 14px glyph rather than the grid's button -- but reads at the same
	// 11.5px as every other sub line (app.css:649).
	CardListStatIcon     = 14
	CardListStatFont     = 11.5
	CardListStatGap      = 8 // between one stat and the next
	CardListStatInnerGap = 4 // between a stat's glyph and its count
	// A tag on a card is magpie's `.mchip`: 24 high, `padding: 0 9px`, an 11.5px label
	// (app.css:3539-3543). Two different rules -- 22/10 in the grid and 20/8 in the list --
	// would leave every card's tags short of the chip vocabulary the rest of the shell draws,
	// and make the same tag change height when the view switched.
	CardTagMinHeight = 24
	CardTagPadX      = 9
	CardTagFont      = 11.5
	// The tag pill's `min-height: 28px`, and its `padding: 0 6px 0 10px`. A chip's min-height
	// is 26px, against the card tag's 24 and the pill's 28.
	ChipMinHeight = 25
	PillMinHeight = 22
	PillPadLeft   = 10
	PillPadRight  = 6
	// ChipMaxWidth bounds one condition's name. A handle and a display name together --
	// "@歸藏 (guizang.ai)" -- are wider than the toolbar's share of the row, and the chip was
	// squeezed until the label wrapped onto a second line and doubled the search row's height.
	// Past this the label ellipsises: the condition is still readable and the controls beside
	// it keep their places.
	ChipMaxWidth = 220
	// The card's action is 32 square.
	CardActionSize = 32
	CardMinHeight  = 200
	// The grid's top media is `aspect-video`: 16:9 of the card's width. The measured box at
	// the reference capture is 458 by 257, which is 16:9. A fixed height is wrong at every
	// card width, not just at this one.
	CardMediaAspect = 16.0 / 9.0
	// CardMediaFrameLabel names the grid card's media frame for tests. It is not
	// user-visible: Tester.Find matches labels, and the aspect ratio of this frame is
	// the thing that decides how tall a card is.
	CardMediaFrameLabel = "card-media-frame"
	// CardAvatarLabel names the card's author avatar for tests: the picture when the
	// author has one, the initial otherwise.
	CardAvatarLabel = "card-avatar"
	// DetailAvatarLabel names the detail's hero avatar, the same rule as the card's: the
	// picture when there is one, the initials otherwise.
	DetailAvatarLabel = "detail-avatar"
	// DetailMediaTriggerLabel names the detail's media picture, which is the lightbox's
	// trigger.
	DetailMediaTriggerLabel = "detail-media-trigger"
	// DetailTagInputLabel names the detail's new-tag field, so a test can click it and
	// check whether it lets go of the focus.
	DetailTagInputLabel = "detail-tag-input"
	// The list's media has `aspect-ratio: 2.1 / 1`, the list's own shape; the two are
	// different rules, not a scale.
	CardMediaRatio = 2.1

	// CardCopyWrapLines is the grid's reserved height: calc(1.6em * 2.6).
	CardCopyWrapLines = 2.6
	// The list replaces that floor with its own, one per clamp class:
	//   base   min-height: calc(1.55em * 1.8)
	//   1line  calc(1.55em * 1.05)
	//   2line  calc(1.55em * 1.95)
	//   3line  calc(1.55em * 2.95)
	// The 1.8 base never applies in practice -- CardLineCount always names one of the three.
	CardCopyWrapListBase  = 1.8
	CardCopyWrapListOne   = 1.05
	CardCopyWrapListTwo   = 1.95
	CardCopyWrapListThree = 2.95
	// The `em` in those calc()s is the wrap's own font-size, and the wrap declares none -- it
	// inherits the page's 16px, not the 14/13 the copy inside it uses. The measurements settle
	// it: the grid wrap is 67 (16 × 1.6 × 2.6 = 66.56) and the list's 48 and 26
	// (16 × 1.55 × 1.95 = 48.36, × 1.05 = 26.04). Scaling by the copy's font instead leaves
	// every list card 9 points short and every grid card 11 short.
	CardCopyWrapEm = 16
	// CardCopyClampGrid is the grid's clamp of 5 lines. The 1/2/3-line classes belong only to
	// the list and must not be used here.
	CardCopyClampGrid = 5
	// The suggestion's reject button is a 16-point circle carrying a 10-point close glyph.
	CardSuggestionRejectSize  = 16
	CardSuggestionRejectGlyph = 10

	// The list layout is denser than the grid by its arrangement -- a side media column and
	// a 26px avatar against 28 -- not by type: both read 13 for copy and names and 11.5 for
	// metadata (app.css:646-649).
	CardListCopyFont       = 13
	CardListCopyLineHeight = 1.45
	CardListAuthorFont     = 13
	CardListMetaFont       = 11.5
	CardListAvatarSize     = 26
	CardListAvatarFont     = 10
	// CardListSideWidth is the media column: grid-template-columns 1fr 148px.
	CardListSideWidth = 148
	// The list layout's side gap is 14.
	CardListSideGap = 14
	// The inline media is at least 108 high with a 12px radius. It fills the side column's
	// height rather than holding an aspect ratio, so a tall card gets a tall picture.
	CardListSideMinHeight = 108
	CardListMediaRadius   = 12
	// CardNoMediaPaddingLeft leaves room for the card's select toggle, which floats in the
	// grid's top-left corner.
	CardNoMediaPaddingLeft = 30

	// The loading and empty states. The shell must not show a bare spinner where the reference
	// draws a panel of six placeholder cards.
	SkeletonShellRadius = 12
	SkeletonShellGap    = 24
	SkeletonShellPad    = 24
	SkeletonGridGap     = 16
	SkeletonCardRadius  = 12
	SkeletonCardPad     = 20
	SkeletonMinCardW    = 220
	// SkeletonCardCount is the number of placeholder cards: six.
	SkeletonCardCount = 6

	// The placeholder shapes, from the Tailwind utilities on the skeleton's children.
	SkeletonHeadBarW   = 224 // h-5 w-56
	SkeletonHeadBarH   = 20
	SkeletonSubBarW    = 320 // h-4 w-80
	SkeletonSubBarH    = 16
	SkeletonActionW    = 128 // h-10 w-32
	SkeletonActionH    = 40
	SkeletonAvatarSize = 44 // h-11 w-11
	SkeletonNameW      = 96 // h-4 w-24
	SkeletonNameH      = 16
	SkeletonHandleW    = 64 // h-3 w-16
	SkeletonHandleH    = 12
	SkeletonLineH      = 16 // h-4
	SkeletonMediaH     = 96
	SkeletonMediaTop   = 20 // mt-5

	// FeedStateOverlineFont and its tracking are the empty state's overline.
	FeedStateOverlineFont  = 11
	FeedStateOverlineTrack = 0.12
	FeedStateCopyFont      = 14
	FeedStateCopyLine      = 1.7
	FeedStateIconSize      = 52
	FeedStateIconRadius    = 10
	FeedStateShellPad      = 32
	FeedStateShellGap      = 24
	FeedStateShellRadius   = 12
	// The empty state's shell is min-height: min(420px, calc(100dvh - 240px)); 420 is
	// the fixed end of that, which is the part a window of unknown size can honour.
	FeedStateMinHeight = 420
	// The empty state's title is clamp(1.75rem, 2vw, 2.2rem) at -0.04em tracking; 35.2
	// is the clamp's upper end in pixels.
	FeedStateTitleMax   = 35.2
	FeedStateTitleTrack = -0.04
	// The empty state's copy is max-width: 42ch.
	FeedStateCopyMaxCh = 42
	FeedStateActionGap = 12

	// The sidebar's own spacing: padding-inline 24, the brand row's gap 12, and the sync
	// label's 0.12em tracking. (The 18 hero gap, 10 meta gap and 12 sync-block gap are gone
	// with the rows they separated: the hero is two rows now.)
	SidebarInlinePadding = 14
	// SidebarHeroPadY is the hero's vertical inset. Twenty-four around five rows; two rows do
	// not need the same air, and the 16 it saves belongs to the list below.
	SidebarHeroPadY = 10
	SidebarHeroGap  = 8
	// The brand row's height and gap; its mark is 38.
	SidebarBrandRowHeight = 46
	SidebarBrandGap       = 8
	SyncLabelTracking     = 0.12

	// OverlineTracking, DisplayTracking are letter-spacing values, in em as magpie writes
	// them (`letter-spacing: 0.14em`). They are NOT DIPs: see trackingEm, which every
	// LetterSpacing call has to go through.
	OverlineTracking = 0.05
	DisplayTracking  = -0.01
	// The display title's `line-height: 1.02`. The brand's font-size is 36 and the rule's line
	// box is 37, which is the whole reason the brand row is 37 tall.
	SidebarTitleLineHeight = 1.02

	// FontModalTitle is set in the display face at weight 600. It is larger than FontTitle
	// because a modal is a whole surface, not a section within one.
	FontModalTitle = 16
)

const (
	// Spacing. The most common gaps, in the order they are used.
	SpaceTiny  = 2
	SpaceSmall = 4
	// The stable scale is 4 / 8 / 12 / 16 / 24 / 32 / 40, with 2 kept for hairlines.
	//
	// The gaps of 6, 10, 14 and 18 are not on the scale, and the scale is a rule rather than a
	// preference -- "间距使用 4 / 8 / 12 / 16 / 24 / 32 / 40 的稳定尺度" -- so these tokens are
	// the runtime authority. The mapping is 6 to 8, 10 to 12, and 14 and 18 both to the 16
	// that had no constant until now.
	SpaceBase  = 8
	SpaceLoose = 12
	SpaceL     = 16
)

const (
	PaddingPanel = 18
	// The results shell's `px-5 pb-10 pt-4 lg:px-8` sits on the one element that wraps the
	// toolbar, the banner and the feed. The blocks inside it carry no horizontal padding of
	// their own, so the results pane has a single inset instead of one per block -- otherwise
	// the cards run to the window edge instead of leaving 32 points.
	ResultsPadX      = 16
	ResultsPadTop    = 2
	ResultsPadBottom = 8
	// ContentMaxWidth is the column every page centres itself in: magpie's
	// `padding-left: max(16px, calc(50% - 600px))` (app.css:609) holds the reading
	// measure at 1200 points however wide the window is.
	ContentMaxWidth = 1200
	// The space between the toolbar's control row and the feed. magpie draws its controls
	// straight on the page ground and leaves 8 to the first row under them.
	ResultsHeaderPadBottom = 8
	// The view toggle: an 8-radius button carrying its glyph.
	ViewToggleSize  = 24
	ViewToggleGlyph = 15
	// The sidebar's lists scroll carries 22/12/24 of padding.
	SidebarListsPadTop    = 12
	SidebarListsPadX      = 10
	SidebarListsPadBottom = 24
	PaddingControl        = 12
	// SidebarScrollbarHidden pushes a scroll thumb past the window's right edge.
	//
	// magpie draws its sidebar without a scrollbar: `.side-scroll { scrollbar-width: none }`
	// (app.css:402-403). MyGo 0.4.6 has no per-container switch -- the thumb's colour is the
	// window theme's -- and `ScrollbarInsets` is the one lever that moves it. A right inset of
	// this size puts it outside the window; the track is unchanged, so the list still scrolls
	// by wheel, trackpad and keyboard, and only the 9-point overlay that appeared whenever the
	// pointer was over the list is gone (a floating bar over the counts, which the native
	// scrollbar does not do either).
	SidebarScrollbarHidden = 4096
)

// Motion.
//
// The transitions, each with its own timing. They are named rather than
// inlined because "the app feels wrong" is usually one of these being off by a factor of
// two, or a control given the wrong neighbour's number.
const (
	// MotionWidthMs is the auto-tag progress fill's `transition: width 200ms ease-out`. It
	// does not drive the sidebar's collapse, which is a different animation (`.st-pane`'s
	// 0.3s fade, MotionSlow). The transition itself is not implemented: the bar is drawn at
	// its exact fraction, so a batch moves it in a step rather than a glide.
	MotionWidthMs = 200
	// MotionTransformMs drives panels appearing, on a decelerating curve.
	MotionTransformMs = 180
	// MotionFadeMs drives opacity changes.
	MotionFadeMs = 140

	// The six entrance animations, each with the surface it belongs to. They are separate
	// from the motion-duration tokens above: those drive transitions, these drive keyframes,
	// and their numbers do not agree.
	//
	// One curve is shared by the first three and a different one by the next two, so the
	// curve is part of each entry rather than a single global.
	//

	// The inspector divider is a rule with 24 above and 24 below.
	// SettingsRowMax is the settings toggle row's max-width, 520px. The row also sets
	// align-items: flex-start, because its label can carry a hint beneath and the box should
	// line up with the first line rather than the middle of the pair. The settings page no
	// longer uses it (its rows run the card's full width, as magpie's do); the auto-tagging
	// view keeps it for the one switch it draws on its own.
	SettingsRowMax = 520

	// The toolbar shell: an 8px gap, a 1px border, a 12px radius and 12 of padding.
	ToolbarShellGap    = 8
	ToolbarShellPad    = 12
	ToolbarShellRadius = 12

	// Active filter chips share the search's row, to its right, and wrap inside this share of
	// the row. They wrap across a full-width row when inline; this bound is what keeps a long
	// set from eating the search and the actions. Two fifths leaves the field the widest
	// control in the row, which is why it is the one the chips were moved beside.
	ToolbarChipsMaxPercent = 40
	// FilterPopoverWidth is magpie's `.pop.proto-menu` width (app.css:3024), the panel shape
	// this popover takes: 262 wide, an 11-point radius (`.pop`, app.css:2270-2280) and a
	// 4-point inset (app.css:3024).
	FilterPopoverWidth  = 262
	FilterPopoverRadius = 11
	FilterPopoverPad    = 4
	InspectorDividerGap = 24
	// The detail's section overline is 12px/600 with no tracking, unlike the 10px
	// overline the page header uses.
	DetailOverlineFont = 12
	// The detail's meta label and its value: 11px/600 quaternary above 14px/1.45.
	DetailMetaLabelFont = 11
	DetailMetaValueFont = 14
	DetailMetaValueLine = 1.45

	// The settings section label: magpie's `.label`, 11px/600/.05em uppercase. The settings
	// page's own numbers live in metrics_interaction.go, beside the rest of that page.
	SettingsGroupTitle = 11
	SettingsGroupTrack = 0.05

	// DetailContextMinWidth and DetailContextMaxWidth are the reading pane's companion:
	// `grid-template-columns: minmax(0, 1fr) minmax(238px, 270px)`. The rail takes its
	// maximum, which is what a wide window gives it.
	DetailContextMinWidth = 238
	DetailContextMaxWidth = 270
	// DetailLayoutGap is the gap between the reading pane and the context rail.
	DetailLayoutGap = 24
	// DetailCardMaxWidth and DetailCardMaxHeight are the focus card's min(94vw, 980px) and
	// 86dvh, taken at the wide end -- except the width, which is 780: 980 is a reading pane
	// plus a 238-270 rail plus a 24 gap, and this detail has no rail. 780 keeps the reading
	// measure, 980 - 52 - 24 - 270.
	DetailCardMaxWidth  = 780
	DetailCardMaxHeight = 720

	// AnimateDetailBackdropMs is the detail backdrop's entrance.
	AnimateDetailBackdropMs = 210
	// AnimateDetailCardMs is the detail focus card's entrance.
	AnimateDetailCardMs = 220
	// AnimateDetailToastMs is the detail toast's entrance.
	AnimateDetailToastMs = 180
	// AnimateLightboxBackdropMs is the lightbox backdrop's entrance.
	AnimateLightboxBackdropMs = 180
	// AnimateLightboxContentMs is the lightbox content's entrance.
	AnimateLightboxContentMs = 220
	// AnimateLightboxImageMs is the lightbox image's entrance, the only one on ease-out
	// rather than a cubic-bezier.
	AnimateLightboxImageMs = 160

	// CurveDecelerate is cubic-bezier(0.22, 1, 0.36, 1), used by the detail entrances.
	CurveDecelerate = "cubic-bezier(0.22, 1, 0.36, 1)"
	// CurveEmphasized is cubic-bezier(0.16, 1, 0.3, 1), used by the lightbox.
	CurveEmphasized = "cubic-bezier(0.16, 1, 0.3, 1)"
	// CurveEaseOut is the lightbox image's curve.
	CurveEaseOut = "ease-out"
)

// Column widths.
//
// Draggable rails persist their widths; these are the defaults a first run starts with.
const (
	// magpie's drag clamps to 168-320 and the settings layer validates the same pair, so a
	// stored width and a dragged one agree.
	WidthSidebarMin = 168
	WidthSidebarMax = 320
	// 212, magpie's `--side-w` (app.css:370); its drag clamps to 168-320 and to 36% of the
	// window.
	WidthSidebarDef = 212
	// MaxSidebarShare is the window share a dragged sidebar may take (`innerWidth * 0.36`).
	MaxSidebarShare = 0.36
	// NavSideWidth is where magpie gives the sidebar up on its own: `SIDE_AT = 1024`, with
	// `(min-width: 1024px)` choosing the sidebar and anything narrower the header layout. A
	// reader who has picked a layout keeps it at every width.
	NavSideWidth = 1024
	// The same pair, for the right column: settings.MinInspectorWidth/Max.
	WidthInspectorMin = 320
	WidthInspectorMax = 520
	// WidthInspectorDef is 360.
	WidthInspectorDef = 360
)

// NormalizeSidebarWidth and NormalizeInspectorWidth clamp a dragged width.
//
// The split enforces a minimum of its own, but not these bounds, and a stored value can be
// outside them. Clamping here keeps a width the user cannot drag back out of range from being
// persisted.
//
// Zero means "no stored preference", and it takes the default rather than the floor. The two
// must not be the same number: clamping 0 against a floor of 260 gives 260, so a first run
// would open at the narrowest width the user could drag to. The defaults are WidthSidebarDef
// and WidthInspectorDef, and the clamping functions below must leave zero to them.
// trackingEm converts a letter-spacing in em to the DIPs ui.LetterSpacing wants.
//
// magpie writes `letter-spacing: 0.14em` and MyGo writes `LetterSpacing(v)` in DIPs added
// after every character, so the constant cannot be passed through: 0.14 em at 10px is
// 1.4 points, and the bare 0.14 draws the overline at a tenth of the intended width. The
// mismatch is invisible on short Latin labels and makes "工作区" 30 wide where 34 is right.
func trackingEm(em, fontSize float32) float32 { return em * fontSize }

func NormalizeSidebarWidth(width float32) float32 {
	if width == 0 {
		return WidthSidebarDef
	}
	return min(max(width, WidthSidebarMin), WidthSidebarMax)
}

// ClampSidebarWidth applies magpie's window share on top of the stored bounds:
// `Math.max(SIDE_MIN, Math.min(w, SIDE_MAX, innerWidth * 0.36))`.
//
// Zero means "no window measured yet" (a tester, the first frame), in which case the
// stored bounds are the only ones that apply.
func ClampSidebarWidth(width float32, windowWidth int) float32 {
	clamped := NormalizeSidebarWidth(width)
	if windowWidth > 0 {
		share := float32(windowWidth) * MaxSidebarShare
		if share < WidthSidebarMin {
			share = WidthSidebarMin
		}
		clamped = min(clamped, share)
	}
	return clamped
}

// NormalizeInspectorWidth clamps an inspector width.
func NormalizeInspectorWidth(width float32) float32 {
	if width == 0 {
		return WidthInspectorDef
	}
	return min(max(width, WidthInspectorMin), WidthInspectorMax)
}

// The empty state: icon container 64, title 24-28, copy 13-15, and the copy width limited so
// it stays readable rather than running the width of the window.
const (
	EmptyStateIconSize  = 64
	EmptyStateTitleFont = 13
	EmptyStateCopyFont  = 12
)

// The results grid. The minimum width is what GridView fits columns against, so it decides how
// many cards a window shows.
//
// The height is not 184. A card may grow by variant -- which is what it does: it reserves
// CardMinHeight, because it draws an avatar, a name, a handle, the text, its tags and a time.
// Putting 184 in the grid lays every card out in a 184 slot and the card draws past the bottom
// of it, cutting off the row below the handle.
//
// So the row height comes from the card's own minimum rather than from a second number that
// can disagree with it. A grid whose slot is shorter than what goes in the slot is a grid that
// clips, and the two numbers have to be one.
// GridColumnMinWidth is the width handed to ui.GridView's column division, chosen so the grid
// lands on two columns at the widths this shell uses.
//
// GridView does not take a column count: it computes `floor((width+gap)/(minWidth+gap))` from the
// width the rows had last frame, and Element.Columns() does not override it. So the count is
// reached through this number instead, and the arithmetic is:
//
//	1180 window -> 888 for the pane -> (888+16)/(384+16) = 2.26 -> 2 columns
//	 900 window -> 608 for the pane -> (608+16)/(384+16) = 1.56 -> 1 column
//	1500 window with the inspector -> (904+16)/400 = 2.30      -> 2 columns
//
// A smaller value puts three narrower columns in a 1180 window, and three cramped columns make
// the cards read as wrong against the reference. Past roughly 1700 for the pane this would
// divide to three; the reference stays at two at any width, so that is a divergence worth
// knowing about rather than one worth hiding.
const GridColumnMinWidth = 384

// The results grid, from a multi-column layout rather than from a grid widget.
//
//	column-gap 18px, 22px at the large width
//	card margin-bottom 18px, 22px at the large width
//
// Two columns only exist at the 22px values, so those are the ones the two-column layout
// uses; a single column has no inner gap to draw.
const (
	GridColumnGap = 22
	GridCardGap   = 22
)

// ResultsListGap is the results list's row gap: `display: flex; flex-direction: column;
// gap: 12px`. A ui.List takes its row gap from its own GapY, so leaving it unset stacks the
// native cards flush and each one measures ~2 points higher than the reference's.
const ResultsListGap = 12

// ResultsTwoColumnWidth is the large breakpoint at which two columns switch on: 1024 points
// of viewport, not of results pane. At 900 there is one column.
const ResultsTwoColumnWidth = 1024

// The render window: 80 cards, then 80 more on the control at the end of the grid.
const (
	ResultsRenderLimit     = 80
	ResultsRenderIncrement = 80
)

// The three-pane layout.
//
// InspectorWidth is the detail pane's target; MainMinWidth is what the results pane must keep
// for the inspector to be allowed on screen at all. Do not squeeze the middle column until it
// is unreadable: below the sum of these the detail becomes an overlay instead of a third
// column.
const (
	InspectorWidth = 304
	MainMinWidth   = 560
	// ThreeColumnMinWidth is the width at which the detail may become a pane instead of an
	// overlay.
	//
	// It is a separate number from the arithmetic below because the arithmetic is not enough.
	// 1180 satisfies sidebar+MainMinWidth+InspectorWidth, so on arithmetic alone a 1180-wide
	// window gets three columns -- and the reference never does: its shell is
	// grid-cols-[256px_minmax(0,1fr)] with the detail as a position:fixed overlay, in every
	// window size, and there is no three-column rule anywhere in it. Reading "allowed" as
	// "should" is what puts the detail in a pane at 1180.
	ThreeColumnMinWidth = 1400
)
