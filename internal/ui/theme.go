package ui

import "github.com/egoist/mygo/ui"

// The palette.
//
// The visual language is warm paper: a parchment canvas, ivory cards edged in a hairline,
// one cinnabar accent, and a compact type scale whose headings are serif.
//
// It is the language the brand mark and the download page are already set in, so the window
// and the page that sells it are one thing.
//
// The dark appearance inverts the paper to the ink: the ground is the ink colour, the cards
// are a warm charcoal, and the accent is cinnabar three steps lighter so it still reads on a
// dark ground.
//
// The light and dark palettes below are the two sources.
//
// The palette is used as a whole: every surface comes from these tokens, because mixing it
// with another palette would have produced a third thing that is neither.

// Light tokens.
var (
	lightPageBg = ui.Hex("#f6f0ea") // the page ground
	// Every page sits directly on the page ground and draws a card only where there is a
	// panel; the results column is the page too.
	lightSurfaceBg      = ui.Hex("#fbf7f3") // the card ground
	lightSurfaceAlt     = ui.Hex("#f9f3ec") // an expanded editor or a raised row
	lightRailBorder     = ui.Hex("#e6dacd") // the hairline
	lightBorderSubtle   = ui.Hex("#eadfd5") // the subtle hairline
	lightSurfaceMuted   = ui.Hex("#efe7df") // the tray ground
	lightTextStrong     = ui.Hex("#191514") // the strong ink
	lightTextMuted      = ui.Hex("#5a4a43") // the muted ink
	lightTextTertiary   = ui.Hex("#756a63") // the tertiary ink
	lightTextQuater     = ui.Hex("#8b8078") // the faint ink
	lightBadgeBg        = ui.Hex("#efe7df") // the tray ground
	lightBadgeFg        = ui.Hex("#5a4a43") // the muted ink
	lightBtnHover       = ui.Hex("#ede3da") // the pressed ground: one step below the tray
	lightResultsSurface = ui.Hex("#f6f0ea") // the page ground

	// A selected row is the selection ground with an accent-tinted edge. The selected list rows carry no
	// border; the 0.55 mix is this shell's, for a native selection that has to
	// read without a hover state.
	lightSelectedBg     = ui.Hex("#f7e6e1") // the selection ground
	lightSelectedBorder = ui.Hex("#c97a63") // cinnabar at 55% over the page
	// The scroll thumb is warm neutral paper, never the toolkit's neutral black: a grey
	// thumb over a parchment column reads as a piece of another palette.
	lightScrollThumb = ui.Hex("#c9b9ac")

	// A tag is a pill: the tray ground, the muted ink.
	lightCardTagBg = ui.Hex("#efe7df") // the tray ground
)

// Dark tokens.
var (
	darkPageBg         = ui.Hex("#191514") // the page ground
	darkSurfaceBg      = ui.Hex("#221c19") // the card ground
	darkSurfaceAlt     = ui.Hex("#1e1815") // an expanded editor or a raised row
	darkRailBorder     = ui.Hex("#3a2f2a") // the hairline
	darkSubtleLine     = ui.Hex("#322925") // the subtle hairline
	darkSurfaceMuted   = ui.Hex("#2b2421") // the tray ground
	darkTextStrong     = ui.Hex("#f6f0ea") // the strong ink
	darkTextMuted      = ui.Hex("#d7cbc2") // the muted ink
	darkTextTertiary   = ui.Hex("#b9aca3") // the tertiary ink
	darkTextQuater     = ui.Hex("#7f736b") // the faint ink
	darkBadgeBg        = ui.Hex("#2b2421") // the tray ground
	darkBadgeFg        = ui.Hex("#d7cbc2") // the muted ink
	darkBtnHover       = ui.Hex("#332b27") // the pressed ground: one step below the tray
	darkResultsSurface = ui.Hex("#191514") // the page ground

	darkSelectedBg = ui.Hex("#3a2521") // the selection ground
	// 55% #c97a63 over #221c19.
	darkSelectedBorder = ui.Hex("#a35b47")
	darkScrollThumb    = ui.Hex("#6b5b52") // the scroll thumb

	darkCardTagBg = ui.Hex("#2b2421") // the tray ground
)

// The quiet control ink is a control's quiet text: one step behind the muted ink, and still
// 5.7:1 on the light page and 7.5:1 on the dark one, so an unselected tab stays readable.
// The view switch's unselected tabs and the toolbar's labelled actions use it.
var (
	lightControlText = ui.Hex("#6b5b52")
	darkControlText  = ui.Hex("#b0a49b")
)

// modalScrim is the lightbox's backdrop, in both appearances.
//
// It is the one colour that is not mixed with the page: a photo needs the room dimmed
// further than the cards' 0.4 scrim, and mixing paper into a scrim greys the window instead
// of dimming it.
var modalScrim = ui.RGBA(0, 0, 0, 0.55)

// The segmented control's thumb.
//
// The view switch has no track behind the tabs: only the open tab's thumb, which is the
// card ground in the light theme and a low-alpha white in the dark one.
var (
	lightSegThumb = lightSurfaceBg
	darkSegThumb  = ui.RGBA(255, 255, 255, 0.10)
)

// Status colours, from the same two palettes.
var (
	lightSuccess = ui.Hex("#4a6b52") // the success ink
	lightWarning = ui.Hex("#906327") // the warning ink
	lightDanger  = ui.Hex("#7e241c") // the danger ink

	darkSuccess = ui.Hex("#8fb394") // the success ink
	darkWarning = ui.Hex("#e0b070") // the warning ink
	darkDanger  = ui.Hex("#f2a19a") // the danger ink

	// The soft grounds a badge sits on: the success, warning and danger soft grounds.
	lightSuccessFill = ui.Hex("#e7eee5")
	lightWarningFill = ui.Hex("#f6ebd9")
	lightDangerFill  = ui.Hex("#f7e2de")
	darkSuccessFill  = ui.Hex("#22322a")
	darkWarningFill  = ui.Hex("#3a2f1e")
	darkDangerFill   = ui.Hex("#40231f")
)

// The accent, with its soft and foreground steps.
//
// There is no hover token for the accent: a pressed primary button steps down the same hue
// to cinnabar-800 and the links do not change. The step below is that pressed value, so a
// pressed button deepens rather than inventing a second hue.
var (
	lightAccent      = ui.Hex("#b83d2e") // the accent
	lightAccentHover = ui.Hex("#7e241c") // the accent's pressed step, two shades deeper
	lightAccentSoft  = ui.Hex("#f7e6e1") // the accent's soft ground
	lightAccentFg    = ui.Hex("#ffffff") // the accent's foreground

	darkAccent      = ui.Hex("#c97a63")
	darkAccentHover = ui.Hex("#e3a595")
	darkAccentSoft  = ui.Hex("#3a2521")
	// A dark-theme accent is light enough for the ink to sit on it; the ink is the family's
	// ground colour, not a second near-black invented for this one role.
	darkAccentFg = darkPageBg
)

// StatusBadgeTokens are one status pill: its border, fill and text.
type StatusBadgeTokens struct {
	Border ui.Color
	Fill   ui.Color
	Text   ui.Color
}

// StatusBadge returns the pill colours for a status.
//
// A state is drawn as a 6px dot plus its word, and a soft ground when the state is the
// subject of a card (the status soft grounds and friends). The pill here keeps both: the soft ground,
// the dot's colour as its text, and a hairline of the same colour at 30%.
func StatusBadge(status string, appearance Appearance, systemDark bool, accent ui.Color) StatusBadgeTokens {
	dark := IsDark(appearance, systemDark)
	if dark {
		switch status {
		case StatusSuccess:
			return StatusBadgeTokens{darkSuccess.Alpha(0.30), darkSuccessFill, darkSuccess}
		case StatusPartial:
			return StatusBadgeTokens{darkWarning.Alpha(0.30), darkWarningFill, darkWarning}
		case StatusError:
			return StatusBadgeTokens{darkDanger.Alpha(0.30), darkDangerFill, darkDanger}
		case StatusRunning:
			return StatusBadgeTokens{accent.Alpha(0.30), darkAccentSoft, accent}
		default:
			return StatusBadgeTokens{darkRailBorder, darkBadgeBg, darkBadgeFg}
		}
	}
	switch status {
	case StatusSuccess:
		return StatusBadgeTokens{lightSuccess.Alpha(0.30), lightSuccessFill, lightSuccess}
	case StatusPartial:
		return StatusBadgeTokens{lightWarning.Alpha(0.30), lightWarningFill, lightWarning}
	case StatusError:
		return StatusBadgeTokens{lightDanger.Alpha(0.30), lightDangerFill, lightDanger}
	case StatusRunning:
		return StatusBadgeTokens{accent.Alpha(0.30), lightAccentSoft, accent}
	default:
		return StatusBadgeTokens{lightRailBorder, lightBadgeBg, lightBadgeFg}
	}
}

// The status names the interface uses. A sync run's own vocabulary maps onto
// these at the call site.
const (
	StatusIdle    = "idle"
	StatusRunning = "running"
	StatusSuccess = "success"
	// StatusPartial is spelled with the underscore because that is the canonical value the
	// status vocabulary and every copy catalogue use.
	StatusPartial = "partial_success"
	StatusError   = "error"
)

// AllStatuses is the whitelist, in display order.
var AllStatuses = []string{StatusIdle, StatusRunning, StatusSuccess, StatusPartial, StatusError}

// StatusDotSize is the 7px dot before a state.
const StatusDotSize = 7

// Appearance is the user's theme preference.
type Appearance = string

const (
	AppearanceSystem Appearance = "system"
	AppearanceLight  Appearance = "light"
	AppearanceDark   Appearance = "dark"
)

// AllAppearances is the whitelist for validation on read.
var AllAppearances = []Appearance{AppearanceSystem, AppearanceLight, AppearanceDark}

// NextAppearance is the theme the cycling button moves to.
//
// It is not a fixed three-step rotation:
// from "system" it goes to the *opposite of the theme currently resolved*, so the first
// press always looks like a toggle regardless of what the system is set to. From there
// it walks system -> light -> dark -> system.
func NextAppearance(current Appearance, resolved Appearance) Appearance {
	switch current {
	case AppearanceSystem:
		if resolved == AppearanceDark {
			return AppearanceLight
		}
		return AppearanceDark
	case AppearanceDark:
		return AppearanceLight
	default:
		return AppearanceSystem
	}
}

// ValidAppearance reports whether a stored preference is known.
func ValidAppearance(value string) bool {
	for _, candidate := range AllAppearances {
		if value == candidate {
			return true
		}
	}
	return false
}

// IsDark resolves a preference against what the OS reports.
func IsDark(appearance Appearance, systemDark bool) bool {
	return appearance == AppearanceDark || (appearance == AppearanceSystem && systemDark)
}

// FolioTheme returns the theme for an appearance.
//
// systemDark is what the OS currently reports, which only matters when the
// preference is "system".
func FolioTheme(appearance Appearance, systemDark bool) *ui.Theme {
	if IsDark(appearance, systemDark) {
		return folioDark()
	}
	return folioLight()
}

func folioLight() *ui.Theme { return folioTheme(false) }

func folioDark() *ui.Theme { return folioTheme(true) }

// folioTheme maps the palette's tokens onto the toolkit's roles.
//
// The toolkit's roles are coarser than the palette's tokens -- one Surface where the palette
// has the card and raised-row grounds, one SurfaceHover where it has the tray and pressed
// grounds -- so the mapping is stated once here rather than at each call site.
func folioTheme(dark bool) *ui.Theme {
	var theme *ui.Theme
	if dark {
		theme = ui.DarkTheme()
	} else {
		theme = ui.LightTheme()
	}
	theme.Dark = dark

	if dark {
		theme.Background = darkPageBg
		theme.Surface = darkSurfaceBg
		theme.SurfaceHover = darkSurfaceMuted
		theme.SurfacePressed = darkBtnHover
		theme.Border = darkRailBorder
		theme.Text = darkTextStrong
		theme.TextMuted = darkTextMuted
		theme.Selection = darkSelectedBg
		theme.Accent = darkAccent
		theme.AccentHover = darkAccentHover
		theme.AccentPressed = darkAccentHover
		theme.AccentText = darkAccentFg
		theme.Focus = darkAccent
		theme.Danger = darkDanger
		theme.Warning = darkWarning
		theme.Success = darkSuccess
		// The toolkit's own default is a neutral 35% white; the thumb is a token like every
		// other surface.
		theme.Scrollbar = darkScrollThumb
	} else {
		theme.Background = lightPageBg
		theme.Surface = lightSurfaceBg
		theme.SurfaceHover = lightSurfaceMuted
		theme.SurfacePressed = lightBtnHover
		theme.Border = lightRailBorder
		theme.Text = lightTextStrong
		theme.TextMuted = lightTextMuted
		theme.Selection = lightSelectedBg
		theme.Accent = lightAccent
		theme.AccentHover = lightAccentHover
		theme.AccentPressed = lightAccentHover
		theme.AccentText = lightAccentFg
		theme.Focus = lightAccent
		theme.Danger = lightDanger
		theme.Warning = lightWarning
		theme.Success = lightSuccess
		// The toolkit's own default is a neutral 32% black; the thumb is a token like every
		// other surface.
		theme.Scrollbar = lightScrollThumb
	}
	return theme
}

// BrandCinnabar is the accent under its old name.
//
// The brand mark is the app's own bookmark logo; the name is kept because branding and the
// icon tests refer to it, and the value follows the product's accent.
var BrandCinnabar = ui.Hex("#b83d2e")

type RailTokens struct {
	// Sidebar, Results and Inspector are the three columns.
	Sidebar ui.Color
	Results ui.Color
	// InspectorTop and InspectorBottom are the detail panel's gradient. The app draws panels
	// flat, so both ends are the same colour; the fields stay because a gradient call reads
	// them and a future layout may want one.
	InspectorTop    ui.Color
	InspectorBottom ui.Color
	Inspector       ui.Color
	// CardTagBg and CardTagText are a tag pill: the tray ground, the muted ink.
	CardTagBg   ui.Color
	CardTagText ui.Color
	// ChipBg, ChipBorder and ChipText are an interactive chip: the card ground, a hairline
	// edge, the muted ink, turning accent-soft with accent text when on.
	ChipBg     ui.Color
	ChipBorder ui.Color
	ChipText   ui.Color
	// PillBg, PillBorder and PillText are an inert pill on a panel.
	PillBg       ui.Color
	PillBorder   ui.Color
	PillText     ui.Color
	CardAvatarBg ui.Color
	HeroAvatarBg ui.Color
	// CardBorder is the result card's hairline.
	CardBorder ui.Color
	// RailLine separates the columns; SubtleLine separates rows inside them.
	RailLine   ui.Color
	SubtleLine ui.Color
	// PanelStrong is the raised surface: cards, the inspector's own panels.
	PanelStrong ui.Color
	// Muted is the recessed surface: secondary buttons, hovered rows.
	Muted ui.Color
	// Accent is the accent, the progress fill and the primary action colour.
	Accent ui.Color
	// Badge is the accent badge.
	BadgeBg   ui.Color
	BadgeText ui.Color
	// Text tiers below the two the theme carries.
	TextTertiary   ui.Color
	TextQuaternary ui.Color
	// SelectedBg and SelectedBorder are the two selection tokens.
	SelectedBg     ui.Color
	SelectedBorder ui.Color
	// FieldFocus is the search field's ground once it has the cursor.
	FieldFocus ui.Color
	// Overlay is the modal backdrop.
	Overlay ui.Color
}

// Rail returns the structural colours for an appearance.
//
// The zh locale used to select a serif stack for the shell. Headings now use the display
// stack in syncsettings.go, the same in both locales, so the argument stays for the call
// sites and changes nothing.
func Rail(appearance Appearance, systemDark bool, chineseLocale bool) RailTokens {
	tokens := railLight()
	if IsDark(appearance, systemDark) {
		tokens = railDark()
	}
	return tokens
}

func railLight() RailTokens { return railFrom(folioTheme(false), false) }
func railDark() RailTokens  { return railFrom(folioTheme(true), false) }

// cardTagBgFor is a tag pill's ground.
func cardTagBgFor(dark bool) ui.Color {
	if dark {
		return darkCardTagBg
	}
	return lightCardTagBg
}

// cardAvatarBgFor is the placeholder behind a picture or an avatar.
func cardAvatarBgFor(dark bool) ui.Color {
	if dark {
		return darkSurfaceMuted
	}
	return lightSurfaceMuted
}

// heroAvatarBgFor is the detail hero's circle.
func heroAvatarBgFor(dark bool) ui.Color {
	return cardAvatarBgFor(dark)
}

func resultsSurfaceFor(dark bool) ui.Color {
	if dark {
		return darkResultsSurface
	}
	return lightResultsSurface
}

func mutedTextColour(dark bool) ui.Color {
	if dark {
		return darkTextTertiary
	}
	return lightTextTertiary
}

// faintTextColour is the faint ink: the third text tier, for standing facts that must stay
// readable. It was mapped to theme.Border, which is a line colour and made the toolbar's
// last-sync sentence nearly invisible on the page.
func faintTextColour(dark bool) ui.Color {
	if dark {
		return darkTextQuater
	}
	return lightTextQuater
}

// controlTextColour is the quiet control ink.
func controlTextColour(dark bool) ui.Color {
	if dark {
		return darkControlText
	}
	return lightControlText
}

func segThumbFor(dark bool) ui.Color {
	if dark {
		return darkSegThumb
	}
	return lightSegThumb
}

// segShadowFor is the drop half of the segmented edge.
func segShadowFor(dark bool) ui.Color {
	if dark {
		return ui.RGBA(0, 0, 0, 0.30)
	}
	return ui.RGBA(25, 21, 20, 0.14)
}

// segEdge is the segmented edge: a one-point drop and a half-point ring, in that order.
//
// Light: `0 1px 2px rgba(25,21,20,.14), 0 0 0 .5px rgba(25,21,20,.08)`. Dark: the drop, plus an
// inset highlight MyGo cannot draw -- an inner shadow would need a blend the painter has no
// equivalent for, so the dark thumb keeps its drop alone.
func segEdge(el ui.Element, dark bool) ui.Element {
	el = el.Shadow(0, 1, 2, 0, segShadowFor(dark))
	if !dark {
		el = el.Shadow(0, 0, 0, 0.5, ui.RGBA(25, 21, 20, 0.08))
	}
	return el
}

// popoverShadow is the palette's popover shadow, the drop under a popover:
//
//	light: 0 16px 44px rgba(25,21,20,.16), 0 2px 8px rgba(25,21,20,.06),
//	       0 0 0 .5px rgba(25,21,20,.1)
//	dark:  0 16px 44px rgba(0,0,0,.55), 0 2px 8px rgba(0,0,0,.3),
//	       0 0 0 .5px rgba(255,255,255,.1)
//
// The half-point ring is a shadow rather than a border; the panels that use this also carry
// the hairline, so the two meet in the same place.
func popoverShadow(el ui.Element, dark bool) ui.Element {
	drop, near, ring := ui.RGBA(25, 21, 20, 0.16), ui.RGBA(25, 21, 20, 0.06), ui.RGBA(25, 21, 20, 0.10)
	if dark {
		drop, near, ring = ui.RGBA(0, 0, 0, 0.55), ui.RGBA(0, 0, 0, 0.30), ui.RGBA(255, 255, 255, 0.10)
	}
	return el.Shadow(0, 16, 44, 0, drop).
		Shadow(0, 2, 8, 0, near).
		Shadow(0, 0, 0, 0.5, ring)
}

func surfaceAltFor(dark bool) ui.Color {
	if dark {
		return darkSurfaceAlt
	}
	return lightSurfaceAlt
}

func railFrom(theme *ui.Theme, chineseLocale bool) RailTokens {
	dark := theme.Dark
	subtle := lightBorderSubtle
	if dark {
		subtle = darkSubtleLine
	}
	return RailTokens{
		Sidebar:         theme.Background,
		Results:         resultsSurfaceFor(dark),
		InspectorTop:    theme.Surface,
		InspectorBottom: theme.Surface,
		Inspector:       theme.Surface,

		CardBorder:  theme.Border,
		RailLine:    theme.Border,
		SubtleLine:  subtle,
		PanelStrong: theme.Surface,

		ChipBg:      theme.Surface,
		ChipBorder:  theme.Border,
		ChipText:    theme.TextMuted,
		CardTagBg:   cardTagBgFor(dark),
		CardTagText: theme.TextMuted,
		PillBg:      theme.Surface,
		PillBorder:  theme.Border,
		PillText:    theme.TextMuted,

		CardAvatarBg: cardAvatarBgFor(dark),
		HeroAvatarBg: heroAvatarBgFor(dark),

		Muted: theme.SurfaceHover,

		TextTertiary:   mutedTextColour(dark),
		TextQuaternary: faintTextColour(dark),

		Accent: theme.Accent,

		BadgeBg:   accentSoftFor(dark),
		BadgeText: theme.Accent,

		SelectedBg:     selectedBgFor(dark),
		SelectedBorder: selectedBorderFor(dark),
		FieldFocus:     theme.Surface,

		Overlay: modalScrim,
	}
}

func accentSoftFor(dark bool) ui.Color {
	if dark {
		return darkAccentSoft
	}
	return lightAccentSoft
}

func selectedBgFor(dark bool) ui.Color {
	if dark {
		return darkSelectedBg
	}
	return lightSelectedBg
}

func selectedBorderFor(dark bool) ui.Color {
	if dark {
		return darkSelectedBorder
	}
	return lightSelectedBorder
}
