package ui

import "github.com/egoist/mygo/ui"

// The palette.
//
// The visual language is a cool grey canvas, white cards with a hairline edge, one
// indigo accent, and a compact system-sans type scale.
//
// The light and dark palettes below are the two sources.
//
// The palette is used as a whole: every surface comes from these tokens, because mixing it
// with another palette would have produced a third thing that is neither.

// Light tokens.
var (
	lightPageBg = ui.Hex("#f4f4f6") // the page ground
	// The results ground is the page: every page sits directly on the page ground and draws
	// a card only where there is a panel. lightMainBg is kept as a second name because call
	// sites name it.
	lightMainBg         = ui.Hex("#f4f4f6") // the page ground
	lightSideBg         = ui.Hex("#f4f4f6") // the page ground (the sidebar sits on the page)
	lightSurfaceBg      = ui.Hex("#ffffff") // the card ground
	lightSurfaceAlt     = ui.Hex("#f9f9fb") // an expanded editor or a raised row
	lightRailBg         = ui.Hex("#f4f4f6") // the page ground
	lightRailBorder     = ui.Hex("#e3e3e8") // the hairline
	lightBorderSubtle   = ui.Hex("#ececf0") // the subtle hairline
	lightSurfaceMuted   = ui.Hex("#f1f1f4") // the tray ground
	lightTextStrong     = ui.Hex("#1c1c21") // the strong ink
	lightTextMuted      = ui.Hex("#4b4b53") // the muted ink
	lightTextTertiary   = ui.Hex("#85858d") // the tertiary ink
	lightTextQuater     = ui.Hex("#b9b9c2") // the faint ink
	lightBadgeBg        = ui.Hex("#f1f1f4") // the tray ground
	lightBadgeFg        = ui.Hex("#4b4b53") // the muted ink
	lightPanelStrong    = ui.Hex("#ffffff") // the card ground
	lightBtnHover       = ui.Hex("#e9e9ee") // the pressed ground
	lightFieldFocus     = ui.Hex("#ffffff") // the card ground; a field keeps its ground and gains a ring
	lightOptionsPage    = ui.Hex("#f4f4f6") // the page ground
	lightOptionsSurf    = ui.Hex("#ffffff") // the card ground
	lightOptionsBorder  = ui.Hex("#e3e3e8") // the hairline
	lightResultsSurface = ui.Hex("#f4f4f6") // the page ground

	// A selected row is the selection ground with an accent-tinted edge. The selected list rows carry no
	// border; the 0.55 mix is this shell's, for a native selection that has to
	// read without a hover state.
	lightSelectedBg     = ui.Hex("#ecebfb") // the selection ground
	lightSelectedBorder = ui.Hex("#a5a0f2") // 55% #4f46e5 over #ffffff
	lightScrollThumb    = ui.Hex("#b9b9c2") // the faint ink

	// A tag is a pill: the tray ground, the muted ink. Badges and chips are grey until
	// they are interactive.
	lightTagBg     = ui.Hex("#f1f1f4") // the tray ground
	lightCardTagBg = ui.Hex("#f1f1f4") // the tray ground

	// detailDoneWarm is kept for the one reader that names it (the detail's done button) and
	// points at the accent.
	detailDoneWarm = ui.Hex("#4f46e5")
)

// Dark tokens.
var (
	darkPageBg         = ui.Hex("#1a1a1e") // the page ground
	darkMainBg         = ui.Hex("#1a1a1e") // the page ground
	darkSideBg         = ui.Hex("#1a1a1e") // the page ground
	darkSurfaceBg      = ui.Hex("#232327") // the card ground
	darkSurfaceAlt     = ui.Hex("#1f1f23") // an expanded editor or a raised row
	darkRailBorder     = ui.Hex("#333339") // the hairline
	darkSubtleLine     = ui.Hex("#2c2c32") // the subtle hairline
	darkSurfaceMuted   = ui.Hex("#2d2d33") // the tray ground
	darkTextStrong     = ui.Hex("#ededf1") // the strong ink
	darkTextMuted      = ui.Hex("#b9b9c1") // the muted ink
	darkTextTertiary   = ui.Hex("#8b8b94") // the tertiary ink
	darkTextQuater     = ui.Hex("#56565f") // the faint ink
	darkBadgeBg        = ui.Hex("#2d2d33") // the tray ground
	darkBadgeFg        = ui.Hex("#b9b9c1") // the muted ink
	darkPanelStrong    = ui.Hex("#232327") // the card ground
	darkBtnHover       = ui.Hex("#36363d") // the pressed ground
	darkFieldFocus     = ui.Hex("#232327") // the card ground
	darkOptionsPage    = ui.Hex("#1a1a1e") // the page ground
	darkOptionsSurf    = ui.Hex("#232327") // the card ground
	darkOptionsBorder  = ui.Hex("#333339") // the hairline
	darkResultsSurface = ui.Hex("#1a1a1e") // the page ground

	darkSelectedBg = ui.Hex("#33345a") // the selection ground
	// 55% #a5b4fc over #232327.
	darkSelectedBorder = ui.Hex("#6c74a6")
	darkScrollThumb    = ui.Hex("#56565f") // the faint ink

	darkTagBg     = ui.Hex("#2d2d33") // the tray ground
	darkCardTagBg = ui.Hex("#2d2d33") // the tray ground

	darkOverlay = ui.RGBA(0, 0, 0, 0.55)

	// The quiet control ink is a control's quiet text: darker than the muted ink so it clears 4.5:1 on every
	// surface it sits on. The view switch's unselected tabs and the toolbar's labelled
	// actions use it.
	lightControlText = ui.Hex("#63636b")
	darkControlText  = ui.Hex("#a1a1aa")
)

// The segmented control's thumb.
//
// The view switch has no track behind the tabs: only the open tab's thumb, which is the
// card ground in the light theme and a low-alpha white in the dark one.
var (
	lightSegThumb = ui.Hex("#ffffff")
	darkSegThumb  = ui.RGBA(255, 255, 255, 0.15)
)

// Status colours, from the same two palettes.
var (
	lightSuccess = ui.Hex("#189a58") // the success ink
	lightWarning = ui.Hex("#b45309") // the warning ink
	lightDanger  = ui.Hex("#d64545") // the danger ink

	darkSuccess = ui.Hex("#6fd99b") // the success ink
	darkWarning = ui.Hex("#f2b544") // the warning ink
	darkDanger  = ui.Hex("#f28b82") // the danger ink

	// The soft grounds a badge sits on: the success, warning and danger soft grounds.
	lightSuccessFill = ui.Hex("#e4f6ec")
	lightWarningFill = ui.Hex("#fbf0df")
	lightDangerFill  = ui.Hex("#fbe9e9")
	darkSuccessFill  = ui.Hex("#1e3a2b")
	darkWarningFill  = ui.Hex("#3f3220")
	darkDangerFill   = ui.Hex("#45272a")
)

// The accent, with its soft and foreground steps.
//
// There is no hover token for the accent: the primary button brightens by 8% and the
// links do not change. The two hovers below are that 8% adjustment evaluated, so a hovered
// primary button moves that way without inventing a second hue.
var (
	lightAccent      = ui.Hex("#4f46e5") // the accent
	lightAccentHover = ui.Hex("#4338ca") // the accent at 92% brightness (a brightness of 1.08 would be lighter; this is the pressed step)
	lightAccentSoft  = ui.Hex("#eeedfd") // the accent's soft ground
	lightAccentFg    = ui.Hex("#ffffff") // the accent's foreground

	darkAccent      = ui.Hex("#a5b4fc")
	darkAccentHover = ui.Hex("#c7d2fe")
	darkAccentSoft  = ui.Hex("#2b2c47")
	darkAccentFg    = ui.Hex("#14142a")
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
	}
	return theme
}

// BrandCinnabar is the accent under its old name.
//
// The brand mark is the app's own bookmark logo; the name is kept because branding and the
// icon tests refer to it, and the value follows the product's accent.
var BrandCinnabar = ui.Hex("#4f46e5")

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
// The zh locale used to select a serif stack; both locales use the system sans here,
// so the argument stays for the call sites and changes nothing.
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
	return ui.RGBA(0, 0, 0, 0.14)
}

// segEdge is the segmented edge: a one-point drop and a half-point ring, in that order.
//
// Light: `0 1px 2px rgba(0,0,0,.14), 0 0 0 .5px rgba(0,0,0,.08)`. Dark: the drop, plus an
// inset highlight MyGo cannot draw -- an inner shadow would need a blend the painter has no
// equivalent for, so the dark thumb keeps its drop alone.
func segEdge(el ui.Element, dark bool) ui.Element {
	el = el.Shadow(0, 1, 2, 0, segShadowFor(dark))
	if !dark {
		el = el.Shadow(0, 0, 0, 0.5, ui.RGBA(0, 0, 0, 0.08))
	}
	return el
}

// popoverShadow is the palette's popover shadow, the drop under a popover:
//
//	light: 0 16px 44px rgba(20,22,30,.16), 0 2px 8px rgba(20,22,30,.06),
//	       0 0 0 .5px rgba(20,22,30,.1)
//	dark:  0 16px 44px rgba(0,0,0,.55), 0 2px 8px rgba(0,0,0,.3),
//	       0 0 0 .5px rgba(255,255,255,.1)
//
// The half-point ring is a shadow rather than a border; the panels that use this also carry
// the hairline, so the two meet in the same place.
func popoverShadow(el ui.Element, dark bool) ui.Element {
	drop, near, ring := ui.RGBA(20, 22, 30, 0.16), ui.RGBA(20, 22, 30, 0.06), ui.RGBA(20, 22, 30, 0.10)
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

		Overlay: darkOverlay,
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
