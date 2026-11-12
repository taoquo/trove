package ui

// The detail's numbers (shell-contract.md S41, S43).
//
// Split out of metrics.go, which had grown past the one-responsibility cap: the detail's reading
// pane, its context rail and its action bar are the largest single group of numbers in the build,
// and none of them is shared with the shell or with a card.
const (
	// The detail hero, from .options-detail-hero: 22/26/18 of padding, a bottom border and a
	// 48px avatar in the source. The avatar is 30 here: the header is one line of text and the
	// source's portrait turned the byline into a column (S43).
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

	// The detail's reading pane: the post as one object (shell-contract.md S41).
	//
	// The post reads on magpie's page scale: 13px (app.css:128-129), with the leading opened
	// to 1.6 the way its long-form blocks do (`.action-confirm p`, app.css:4074). The
	// section gaps are a paragraph's rather than the 22 the old two-section layout used. The
	// media is a grid: four attachments at most, two to a row, 6 apart, which is the shape a
	// tweet's own gallery has.
	DetailBodyFont      = 13
	DetailBodyLine      = 1.6
	DetailPostGap       = 14
	DetailMediaGridGap  = 6
	DetailMediaRadius   = 12
	DetailMediaColumns  = 2
	DetailMediaMaxCount = 4
	// The ratio clamp: a picture taller than 4:5 fills the pane, and one wider than 3.2:1
	// becomes a band. Beyond these the whole-picture view is the lightbox's job.
	DetailMediaMinRatio = 0.8
	DetailMediaMaxRatio = 3.2
	// The video mark: a 58-point disc carrying a 22-point glyph.
	DetailPlaySize   = 58
	DetailPlayGlyph  = 22
	DetailAltFont    = 12
	DetailAltGap     = 8
	DetailMetricGap  = 18
	DetailMetricIcon = 14
	// .options-card-stat { gap 4px; font-size 10px } is the card's count row; the detail has
	// room for the same row a step larger.
	DetailMetricInnerGap = 5
	DetailMetricFont     = 11
	DetailMetricsPadTop  = 16
	// The tag row on the card's floor: pills and suggestions wrap 8 apart, 12 between the
	// "Tags" overline and the first pill.

	// .options-tag-pill: min-height 28, padding 0 6px 0 10px, 4px between the name and its close.
	DetailTagPillHeight = 22
	DetailTagPillPadEnd = 6
	DetailTagPillPadIn  = 10
	DetailTagPillGap    = 4
	DetailTagPillIcon   = 14 // the close glyph itself is 12; the box adds the clickable square
	// .options-detail-new-tag-input: width clamp(82px, 10vw, 116px), height 28, radius 999.
	DetailTagInputWidth = 116
	DetailTagInputH     = 28
	// The header's icon controls (shell-contract.md S43): magpie's 26-point `.icon` squares, 4
	// apart, so the three of them take less width than the worded buttons they replaced took for
	// one (app.css:276).
	DetailIconButton = 26
	DetailActionGap  = 4
	// The published date in the hero: a 12-point clock and the date beside it, in the same quiet
	// ink as the handle (S43).
	DetailDateIcon       = 12
	DetailDateFont       = 12
	DetailDateGap        = 4
	DetailDialogMixWidth = 0.94 // .options-detail-focus-card { width: min(94vw, 980px) }
	DetailDialogMixHigh  = 0.86 // max-height: 86dvh
	// DetailViewportFallback is the window height a tester draws at before it has a window,
	// matching main.go's default.
	DetailViewportFallback = 760
)
