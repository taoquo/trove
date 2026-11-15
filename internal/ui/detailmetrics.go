package ui

// The detail's numbers.
//
// Split out of metrics.go, which had grown past the one-responsibility cap: the detail's reading
// pane, its context rail and its action bar are the largest single group of numbers in the build,
// and none of them is shared with the shell or with a card.
const (
	// The detail hero: 22/26/18 of padding, a bottom border and a 48px avatar. The avatar
	// is 30 here: the header is one line of text, and a 48px
	// portrait would turn the byline into a column.
	DetailHeroGap        = 18
	DetailHeroPadTop     = 22
	DetailHeroPadSide    = 26
	DetailHeroPadBottom  = 18
	DetailHeroMainGap    = 16
	DetailHeroAvatarSize = 30
	DetailHeroNameFont   = 16
	DetailHeroHandleFont = 12
	DetailHeroActionGap  = 10
	DetailHeroLineGap    = 10

	// The detail's reading pane: the post as one object.
	//
	// The post reads on the page scale: 13px, with the leading opened
	// to 1.6 for long-form blocks. The
	// section gaps are a paragraph's, keeping the post as one object rather than two. The
	// media is a grid: four attachments at most, two to a row, 6 apart, which is the shape a
	// tweet's own gallery has.
	DetailBodyFont      = 13
	DetailBodyLine      = 1.6
	DetailPostGap       = 14
	DetailMediaGridGap  = 6
	DetailMediaRadius   = 12
	DetailMediaColumns  = 2
	DetailMediaMaxCount = 4
	// The reading measure, narrower than the pane. At the pane's 728 a 13-point line holds
	// about 56 CJK characters, which is past the point where the eye loses the start of the
	// line it is returning to; 600 holds about 46. The picture may still use the whole pane.
	DetailReadWidth = 600
	// The single picture's height ceiling. At the cap the picture is scaled down whole and
	// centred rather than cropped to the pane's width, so the pane shows the frame the lightbox
	// will open, only smaller. Nothing is hidden, and there is no crop anchor to choose.
	DetailMediaCap = 420
	// A tile in the multi-picture grid: 3:2 whatever its picture's shape, so a row is one
	// height and two pictures read as a set rather than as two attachments of different sizes.
	DetailMediaTileRatio = 3.0 / 2.0
	// The ratio clamp: a picture taller than 4:5 fills the pane, and one wider than 3.2:1
	// becomes a band. Beyond these the whole-picture view is the lightbox's job.
	DetailMediaMinRatio = 0.8
	DetailMediaMaxRatio = 3.2
	// The video mark's largest disc: the detail draws it at 64, a card at 46 (grid) or 30
	// (list). playGlyphSize keeps the glyph in proportion, so the three are one drawing.
	DetailPlaySize   = 64
	DetailAltFont    = 12
	DetailAltGap     = 8
	DetailMetricGap  = 18
	DetailMetricIcon = 14
	// The card's count row is 4 apart at 10 points; the detail has room for the same row a
	// step larger.
	DetailMetricInnerGap = 5
	DetailMetricFont     = 11
	DetailMetricsPadTop  = 16
	// The tag row on the card's floor: pills and suggestions wrap 8 apart, 12 between the
	// "Tags" overline and the first pill.

	// The tag pill: padding 6 and 10, 4 between the name and its close.
	DetailTagPillHeight = 22
	DetailTagPillPadEnd = 6
	DetailTagPillPadIn  = 10
	DetailTagPillGap    = 4
	DetailTagPillIcon   = 14 // the close glyph itself is 12; the box adds the clickable square
	// The new-tag input: width clamp(82px, 10vw, 116px), height 28, radius 999.
	DetailTagInputWidth = 116
	DetailTagInputH     = 28
	// The header's icon controls: 26-point squares, 4
	// apart, so the three of them take less width than the worded buttons they replaced took for
	// one.
	DetailIconButton = 26
	DetailActionGap  = 4
	// The published date in the hero: a 12-point clock and the date beside it, in the same quiet
	// ink as the handle.
	DetailDateIcon       = 12
	DetailDateFont       = 12
	DetailDateGap        = 4
	DetailDialogMixWidth = 0.94 // width: min(94vw, 980px)
	DetailDialogMixHigh  = 0.86 // max-height: 86dvh
	// The dialog's hairline, inside its width: the pane is the dialog less this and the side
	// padding, so a picture that asks for the pane does not ask for two points it will not get.
	DetailDialogBorder = 1
	// DetailViewportFallback is the window height a tester draws at before it has a window,
	// matching main.go's default.
	DetailViewportFallback = 760
)
