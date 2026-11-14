package ui

import (
	"github.com/egoist/mygo/ui"
)

// The interface's icons, from the Phosphor icon set.
//
// The shapes are Phosphor's, at weight "regular" in a 256x256 viewBox. The path data
// is the real set rather than hand-drawn approximations: drawing a cross by hand would
// look roughly right and be a different icon.
//
// The icons are filled outlines rather than strokes, and MyGo's painter has no path
// support, so the data is parsed and flattened into line segments once at startup.

// IconSizeDefault is the default icon size.
const IconSizeDefault = 18

// IconViewBox is Phosphor's coordinate space.
const IconViewBox = 256

// iconPaths maps an alias to its path data.
var iconPaths = map[string][]string{}

// iconSVGs maps an alias to the parsed document drawIcon hands the painter. Parsed once at
// startup: an icon is a vector, not a per-frame parse.
var iconSVGs = map[string]*ui.SVG{}

// IconPaths returns the path data for an alias, for the drift check.
func IconPaths(name string) []string { return iconPaths[name] }

// IconNames returns every alias the interface can draw.
func IconNames() []string {
	names := make([]string, 0, len(iconPaths))
	for name := range iconPaths {
		names = append(names, name)
	}
	return names
}

// iconPainter is the one painter method the icons need.
//
// Narrowing it to an interface is what makes the renderer testable: the alternative is a
// test that reimplements the drawing and therefore cannot fail when the drawing breaks,
// which is how the first version of this test was written.
type iconPainter interface {
	Icon(s *ui.SVG, r ui.Rect, c ui.Color)
}

// drawIcon renders an icon at a point, scaled from the 256-unit view box.
//
// The glyph is a real SVG, drawn by MyGo's own rasteriser: it flattens the curves, fills
// them with the same coverage rasteriser text uses, and caches the result in the glyph
// atlas by icon and pixel size. The previous renderer flattened the paths itself and
// filled them with horizontal scanlines, which had no vertical anti-aliasing -- the icons
// were visibly stepped -- and re-walked every curve on every frame.
//
// The colour is the icon's own: Phosphor's regular weight is a solid glyph, and the
// The SVG's "currentColor" becomes an explicit colour here.
func drawIcon(p iconPainter, name string, x, y, size float32, color ui.Color) {
	svg, known := iconSVGs[name]
	if !known || size <= 0 || color.A == 0 {
		return
	}
	p.Icon(svg, ui.Rect{X: x, Y: y, W: size, H: size}, color)
}

// IconChip draws a labelled action with a glyph: a text action with its icon slot filled.
//
// It was a flat glyph and a borderless platform button, which made it the one labelled control
// in the shell that did not look like the others. The dotted call sites -- the graph's two
// controls, the batch bar's delete, the suggestions panel's accept and reject -- are all
// actions, and every action is drawn one way.
func IconChip(c *ui.Context, theme *ui.Theme, rail RailTokens, name, label string, onClick func()) {
	textAction(c, theme, rail, textActionSpec{icon: name, label: label, onClick: onClick})
}

// IconToggle draws one of a pair of icon-only toggles, pressed or not.
//
// The active one takes the page's ink and the raised thumb: colour is the whole state,
// with the shared thumb under the pair.
func IconToggle(c *ui.Context, name, label string, active bool, theme *ui.Theme, onClick func()) {
	// The segmented control: a 24-point button in a 6-point radius, the open one carrying the
	// switch's thumb -- the card colour over the track, with a two-point drop.
	// The unselected glyph is the quiet control ink.
	background, glyph := ui.Transparent, controlTextColour(theme.Dark)
	if active {
		background = segThumbFor(theme.Dark)
		glyph = theme.Text
	}

	// The pair is icon-only. A focus stop that announces only "pressed" leaves a keyboard
	// reader choosing between two squares; the label costs nothing on screen.
	toggle := ui.Box(c).Width(ViewToggleSize).Height(ViewToggleSize).Focusable().
		Label(label).Tooltip(label).
		Radius(RadiusSmall).Background(background)
	if active {
		toggle = segEdge(toggle, theme.Dark)
	} else if toggle.Hovered() {
		toggle = toggle.Background(theme.SurfaceHover)
	}
	// A segmented button gives to .96 over .14s when pressed.
	toggle = pressScale(toggle, "press", PressScaleText, MotionPressSeg)
	toggle.
		Draw(func(p *ui.Painter, r ui.Rect) {
			drawIcon(p, name, r.X+(r.W-ViewToggleGlyph)/2, r.Y+(r.H-ViewToggleGlyph)/2,
				ViewToggleGlyph, glyph)
		}).
		OnClick(onClick)
}

func min32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

// SecondaryButton is a labelled action with an optional glyph: textAction with
// its icon slot filled.
//
// There is one labelled button (27 high, 12px), so this is that button;
// the name stays because its call sites are the ones that want the glyph.
func SecondaryButton(c *ui.Context, icon, label string, theme *ui.Theme, rail RailTokens, onClick func()) {
	SecondaryButtonWhen(c, icon, label, theme, rail, true, onClick)
}

// SecondaryButtonWhen is SecondaryButton with a state, for the one bar that has to show an
// action waiting on a choice (the tag merge, whose target starts unchosen).
func SecondaryButtonWhen(c *ui.Context, icon, label string, theme *ui.Theme, rail RailTokens, enabled bool, onClick func()) {
	textAction(c, theme, rail, textActionSpec{
		icon: icon, label: label, disabled: !enabled, onClick: onClick,
	})
}

// searchField is the toolbar's search box, built from parts.
//
// The platform's search control was tried here and cannot be translated: its placeholder is
// the literal word "Search", and Element.Placeholder on it does nothing, so a Chinese window
// still read "Search". A plain text input with a magnifier beside it is the field this builds
// from TextInput, which does honour a placeholder.
//
// What it gives up is SearchField's built-in clear button and Escape handling, so both are here.
// onChange runs after each edit; onEscape is the field's own Escape action. Both
// are required for a search field to do anything: the results are a database
// query, so a new string without a reload leaves the old list on screen -- which
// is exactly what the toolbar's search did until now.
func searchField(c *ui.Context, theme *ui.Theme, rail RailTokens, query *string, placeholder, clearLabel string, handle *ui.Handle, onChange, onEscape func()) {
	// A field in the app's language: white ground, one hairline, a 7-point radius, a `0 9`
	// inset and a 12px label. Grow(1), not
	// FillWidth: the toolbar is a row, and a child that fills it pushes the controls beside
	// it out of the way.
	ui.Row(c).Grow(1).MinHeight(SearchFieldHeight).PaddingX(SearchFieldPadX).PaddingY(0).
		Radius(RadiusSidebarRow).Border(1, rail.CardBorder).Background(theme.Surface).
		GapX(SpaceSmall).Children(func() {
		ui.Box(c).Width(SearchFieldGlyph).Height(SearchFieldGlyph).
			Draw(func(p *ui.Painter, r ui.Rect) {
				drawIcon(p, "search", r.X, r.Y, min32(r.W, r.H), rail.TextQuaternary)
			})
		// Label as well as Placeholder: the placeholder is painted, not a name, so a
		// rebuilt field would answer to nothing at all -- the settings page's find box and
		// the batch bar's field are fixed the same way. This is the field Cmd-F lands in,
		// and the one every query starts from.
		field := ui.TextInput(c, query).BorderWidth(0).Background(ui.Transparent).
			Placeholder(placeholder).Label(placeholder).Grow(1).FontSize(FontSmall).Bind(handle)
		if onChange != nil {
			field = field.OnChange(onChange)
			// Submit consumes Enter. Without it the shell's global Enter
			// shortcut opened the selected bookmark while the reader was typing
			// a query, which made the field unusable from the keyboard.
			field = field.OnSubmit(onChange)
		}
		if onEscape != nil {
			field = field.OnShortcut(0, ui.KeyEscape, onEscape)
		}
		if *query == "" {
			return
		}
		// The visible control that clears it, which is half of what SearchField gave for free. The
		// other half is Escape, and MyGo dispatches keys through the focused element rather than
		// through a value on the context, so it is not available from in here -- recorded as a
		// deviation rather than faked.
		IconOnly(c, "close", clearLabel, theme, rail, func() {
			*query = ""
			if onChange != nil {
				onChange()
			}
		})
	})
}

// ToolbarAction is an outlined button with an icon and a word: 36 points tall, inline,
// with a 6-point gap, a 1-point panel-border edge, a 12-point radius, the elevated ground
// and 10-point side padding.
//
// The filter and sort controls are not IconChips -- a 26-tall pill with no border, used
// elsewhere -- because beside the toolbar's own search field they read as loose labels
// rather than as buttons.
//
// The label is 14 points at medium weight in the secondary ink on the elevated ground.
// Drawn at FontSmall on the page ground with no weight, it sat too far from its icon: a
// 12px label is narrower, so the pair of chips started further in, and without the elevated
// ground they disappeared into the toolbar.
func ToolbarAction(c *ui.Context, theme *ui.Theme, rail RailTokens, icon, label string, badge int, active bool, onClick func()) {
	// One button for the whole shell: textAction. A toolbar's action may carry a glyph,
	// a count and an on state; the button changes its ground rather than drawing another.
	textAction(c, theme, rail, textActionSpec{
		icon: icon, label: label, badge: badge, active: active, onClick: onClick,
	})
}

// ViewToggleGroup is the segmented control the two view buttons sit in.
//
// The segmented control has 2-point padding and an 8-point radius with no background: "no
// grey track behind the tabs: only the open one's thumb". Its buttons are 24 high with a
// 6 radius, and the open one carries the segmented edge shadow.
func ViewToggleGroup(c *ui.Context, children func()) {
	ui.Row(c).GapX(2).Padding(2).Radius(RadiusMedium).Children(children)
}

// IconButtonFlat is a borderless square button carrying a text glyph.
//
// The "+" in the sidebar's section head is 24 square, radius 6, no border, the tertiary ink at
// 15/1. Drawn as a ui.Button it takes the platform's bordered button chrome instead, which
// puts a box inside a section head that has none.
func IconButtonFlat(c *ui.Context, label string, theme *ui.Theme, rail RailTokens, size float32, onClick func()) {
	button := ui.Box(c).Width(size).Height(size).Radius(6).Center()
	button = button.Background(hoverColor(c, button, "flat-bg", ui.Transparent, theme.SurfaceHover, button.Hovered()))
	// An icon control, so a pressed one shrinks to .9.
	button = pressScale(button, "press", PressScaleIcon, MotionPress)
	button.OnClick(onClick).Children(func() {
		ui.Text(c, label).FontSize(SidebarHeadGlyph).TextColor(rail.TextTertiary)
	})
}
