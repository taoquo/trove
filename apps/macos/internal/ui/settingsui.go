package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/x-master/apps/macos/internal/i18n"
)

// The settings page's controls, in magpie's language.
//
// magpie draws settings as cards of preference rows rather than as a form:
//
//	.list            { background: var(--card); border: 1px solid var(--line); border-radius: 12px }
//	.row.pref        { min-height: 48px; padding: 12px 12px 12px 14px; gap: 14px }
//	.row .name       { font-weight: 500 }                                       (13px, the body size)
//	.row.pref .sub   { font-size: 11.5px; color: var(--fg-2); margin-top: 2px; white-space: normal }
//	.row + .row      { border-top: 1px solid var(--line-2) }
//	.row-head .label { font-size: 11px; font-weight: 600; letter-spacing: .05em; text-transform: uppercase }
//	.proto.pick      { height: 24px; padding: 0 6px 0 9px; border-radius: 7px; background: var(--pill); font-size: 12px }
//	.segs / .opt     { padding: 2px; border-radius: 7px; background: var(--pill);
//	                   .opt 22 high, padding 0 10px, border-radius 5px, font-size 12px }
//	.text.action     { height: 27px; padding: 0 10px; border: 1px solid var(--line); border-radius: 6px;
//	                   background: var(--card); color: var(--fg-2); font-size: 12px }
//	.set-field       { height: 24px; padding: 0 8px; border: 1px solid var(--line); border-radius: 7px }
//	                   a number is 72 wide, its value to the right
//	.sess-filter     { height: 26px; padding: 0 9px; border-radius: 7px; font-size: 12px }
//
// (app.css:632-646, 2135-2140, 3013-3023, 3368-3378, 3750-3751, 3785-3791, 1654.) The page
// before this file was a stack of labelled form fields, which reads as a form to fill in; a
// settings list is a set of switches and values, and the row is the unit.

// settingsItem is one preference row before it is drawn.
//
// name and sub are already translated: the caller owns the copy, so a row can
// carry a list's own name as easily as a catalog key.
type settingsItem struct {
	// keys are the rows' own words for the find field. A row is found by its
	// name and its explanation, which is what magpie matches on.
	keys []i18n.Key
	name string
	sub  string
	// subColor is the sub-line's ink when the line carries a state -- a
	// success, a failure -- rather than an explanation.
	subColor ui.Color
	// control sits at the end of the name's line.
	control func()
	// body is a control that takes the width under the name, as a text area
	// does; a row with a body grows past the 48-point line.
	body func()
}

// settingsCard draws one .list of preference rows.
//
// It returns how many rows the find field let through, so a section that
// drew nothing can keep its own heading off the page.
func (s *Shell) settingsCard(c *ui.Context, theme *ui.Theme, rail RailTokens, items ...settingsItem) int {
	visible := make([]settingsItem, 0, len(items))
	for _, item := range items {
		// The find field only filters the settings page. Other pages draw cards in this
		// grammar too (the auto-tagging page), and a query left over from Settings must not
		// hide their rows.
		if !s.state.SettingsOpen || s.settingsShows(item.keys...) {
			visible = append(visible, item)
		}
	}
	if len(visible) == 0 {
		return 0
	}
	ui.Column(c).FillWidth().Background(theme.Surface).Border(1, rail.CardBorder).
		Radius(RadiusPanel).Children(func() {
		for i, item := range visible {
			s.settingsRow(c, theme, rail, item, i == 0)
		}
	})
	return len(visible)
}

// settingsRow draws one .row.pref: what the setting is on the left, the
// control that changes it on the right, and -- for the one control that
// cannot share a line with its name -- that control under both.
func (s *Shell) settingsRow(c *ui.Context, theme *ui.Theme, rail RailTokens, item settingsItem, first bool) {
	row := ui.Row(c).FillWidth().MinHeight(SettingsRowHeight).
		Padding(SettingsRowPadY, SettingsRowPadRight, SettingsRowPadY, SettingsRowPadLeft).
		GapX(SettingsRowGap).AlignItems(ui.Center)
	if !first {
		row = row.BorderWidth(1, 0, 0, 0).BorderColor(rail.SubtleLine)
	}
	if item.body == nil {
		row.Children(func() { s.settingsRowLine(c, theme, item) })
		return
	}
	// A body turns the row into a column: the name line, then the control.
	row.AlignItems(ui.Start).Children(func() {
		ui.Column(c).FillWidth().GapY(SettingsRowGap).Children(func() {
			ui.Row(c).FillWidth().GapX(SettingsRowGap).AlignItems(ui.Center).Children(func() {
				s.settingsRowLine(c, theme, item)
			})
			item.body()
		})
	})
}

// settingsRowLine is the name and its line of explanation.
//
// The sub-line is --fg-2 and wraps: a setting's explanation is read whole,
// and one cut with an ellipsis hides what the setting does (app.css:3788-3791).
func (s *Shell) settingsRowLine(c *ui.Context, theme *ui.Theme, item settingsItem) {
	ui.Column(c).Grow(1).MinWidth(0).GapY(2).Children(func() {
		// A row may be body-only, with the sub-line as its whole text (a caption over a
		// progress bar); it then draws no empty name line.
		if item.name != "" {
			ui.Text(c, item.name).FontSize(SettingsRowNameFont).FontWeight(500).
				TextColor(theme.Text).SingleLine()
		}
		if item.sub == "" {
			return
		}
		ink := theme.TextMuted
		if item.subColor.A > 0 {
			ink = item.subColor
		}
		ui.Text(c, item.sub).FontSize(SettingsRowSubFont).TextColor(ink)
	})
	if item.control != nil {
		item.control()
	}
}

// settingsSectionLabel is the .label that names a group of cards: 11px, 600,
// .05em, uppercase, --muted, and no rule of its own.
func settingsSectionLabel(c *ui.Context, theme *ui.Theme, rail RailTokens, label string) {
	ui.Text(c, strings.ToUpper(label)).FontSize(SettingsGroupTitle).FontWeight(600).
		LetterSpacing(trackingEm(SettingsGroupTrack, SettingsGroupTitle)).
		TextColor(rail.TextTertiary).Padding(0, 2)
}

// settingsNote is the sentence a group of rows says about itself before them,
// magpie's .feat-say: 12px in --muted over the card, not in it.
func settingsNote(c *ui.Context, theme *ui.Theme, rail RailTokens, text string) {
	ui.Text(c, text).FontSize(FontSmall).TextColor(rail.TextTertiary).Padding(0, 4)
}

// settingsPick is the .proto.pick of a preference row: a pill with the current
// value and a caret, which opens the choice.
//
// ui.Select is the control rather than a hand-drawn one because it already
// carries the part a hand-drawn pill cannot: the menu marks the value in
// force and the arrows move through it. Its trigger is re-styled here; its
// menu is the toolkit's own panel, which is magpie's pop card.
func settingsPick(c *ui.Context, theme *ui.Theme, rail RailTokens,
	label string, value *string, options []string, onChange func()) {
	pick := ui.Select(c, value, options).Height(SettingsPickHeight).
		Radius(SettingsPickRadius).Padding(0, SettingsPickPadRight, 0, SettingsPickPadLeft).
		BorderWidth(0).MinWidth(0).Label(label)
	pick = pick.Background(hoverColor(c, pick, "settings-pick", rail.Muted, theme.SurfacePressed, pick.Hovered()))
	pick.OnChange(onChange)
}

// warmTab is one `.warm-tabs button`: 22 high, `padding: 0 7px`, a 6 radius,
// an 11px 600 .05em uppercase label in --muted, and the open one on the --pill
// ground (app.css:3632-3635). mark puts a dot after the label, for a page the
// reader has a reason to notice without opening it.
func warmTab(c *ui.Context, theme *ui.Theme, rail RailTokens,
	label string, active, mark bool, onClick func()) {
	tab := ui.Row(c).Height(SettingsTabHeight).Padding(0, SettingsTabPadX).
		Radius(SettingsTabRadius).GapX(5).Center().Focusable()
	if active {
		tab = tab.Background(rail.Muted)
	}
	ink := rail.TextTertiary
	switch {
	case active:
		ink = theme.Text
	case tab.Hovered():
		ink = theme.TextMuted
	}
	tab.OnClick(onClick).Children(func() {
		ui.Text(c, strings.ToUpper(label)).FontSize(SettingsTabFont).FontWeight(600).
			LetterSpacing(trackingEm(SettingsGroupTrack, SettingsTabFont)).
			TextColor(ink).SingleLine()
		if mark {
			ui.Box(c).Width(SidebarTabDot).Height(SidebarTabDot).
				Radius(RadiusPill).Background(theme.Accent)
		}
	})
}

// settingsSwitch is the .segs of a preference row: an Off/On pair in a --pill
// track, the chosen half on a card-coloured thumb.
func settingsSwitch(c *ui.Context, theme *ui.Theme, rail RailTokens,
	offLabel, onLabel, label string, on *bool, onChange func()) {
	ui.Row(c).Shrink(0).Padding(SettingsSegPad).GapX(2).Radius(SettingsSegRadius).
		Background(rail.Muted).AlignItems(ui.Center).Label(label).Children(func() {
		settingsSegment(c, theme, rail, offLabel, !*on, func() {
			if *on {
				*on = false
				onChange()
			}
		})
		settingsSegment(c, theme, rail, onLabel, *on, func() {
			if !*on {
				*on = true
				onChange()
			}
		})
	})
}

// settingsSegment is one .opt of a switch.
func settingsSegment(c *ui.Context, theme *ui.Theme, rail RailTokens, label string, on bool, choose func()) {
	seg := ui.Row(c).Height(SettingsSegOptionHeight).Padding(0, SettingsSegOptionPadX).
		Radius(SettingsSegOptionRadius).Center().Focusable().Label(label).OnClick(choose)
	if on {
		// The thumb is --card over --pill with --seg-edge's drop; the track is a pill,
		// not the header's deeper --seg-track.
		seg = seg.Background(segThumbFor(theme.Dark)).
			Shadow(0, 1, 2, 0, segShadowFor(theme.Dark))
	}
	// .opt is --muted, the chosen one --fg, and hover only darkens the word to
	// --fg-2: no ground appears (app.css:3374-3396).
	ink := rail.TextTertiary
	switch {
	case on:
		ink = theme.Text
	case seg.Hovered():
		ink = theme.TextMuted
	}
	seg.Children(func() {
		ui.Text(c, label).FontSize(SettingsSegFont).TextColor(ink).SingleLine()
	})
}

// textActionSpec is one .text.action before it is drawn: the label, and the
// glyph, count and state a toolbar or a row may add to it.
type textActionSpec struct {
	icon    string
	label   string
	badge   int
	primary bool
	// active marks a control whose state is on (the create row is open), the
	// toolbar's own equivalent of a chosen filter.
	active   bool
	disabled bool
	// busy marks an action whose work is in flight: the label stays and a spinner turns
	// where the glyph would be, as magpie's `.busy` controls do (app.css:306, :3456).
	busy   bool
	danger bool
	// fill stretches the button across its row, for the one action a narrow
	// panel puts under everything else.
	fill bool
	// height overrides the 27-point action height where the button has to sit
	// in a row of shorter controls (the filter chips' clear-all).
	height  float32
	onClick func()
}

// textAction is magpie's `.text.action`, the one labelled button in this
// shell: 27 high, `padding: 0 10px`, a 6 radius, one --line edge, the --card
// ground, a 12px --fg-2 label and a breath of shadow (app.css:2163-2165).
//
//	.text.action:hover { border-color: var(--muted); background: var(--card-2); color: var(--fg) }
//	.text.danger:hover { color: var(--red); background: var(--red-soft) }
//	.text.primary     { color: var(--accent-fg); background: var(--accent); font-weight: 500 }
//
// It is the settings rows', the toolbars' and the dialogs' button, because
// magpie draws one button and changes its ground, not a family of them.
func textAction(c *ui.Context, theme *ui.Theme, rail RailTokens, spec textActionSpec) {
	height := float32(SettingsActionHeight)
	if spec.height > 0 {
		height = spec.height
	}
	button := ui.Row(c).Height(height).
		Padding(0, SettingsActionPadX).Radius(SettingsActionRadius).
		Border(1, rail.CardBorder).Background(theme.Surface).
		GapX(SettingsActionGap).AlignItems(ui.Center).Focusable().
		Label(spec.label).Shadow(0, 1, 1, 0, ui.RGBA(0, 0, 0, 0.04))
	if spec.fill {
		button = button.FillWidth().Center()
	} else {
		button = button.Shrink(0)
	}
	// `.busy { opacity: .55; pointer-events: none }` (app.css:3456): the work is running, so
	// the control cannot be pressed again while it does.
	if spec.busy {
		button = button.Disabled(true).Opacity(0.55)
	}
	ink := theme.TextMuted
	weight := 400
	switch {
	case spec.disabled:
		button = button.Disabled(true).Opacity(0.5)
		ink = rail.TextQuaternary
	case spec.primary:
		button = button.Background(theme.Accent).Border(1, theme.Accent)
		ink = theme.AccentText
		weight = 500
	case spec.active:
		button = button.Background(surfaceAltFor(theme.Dark)).Border(1, theme.TextMuted)
		ink = theme.Text
	case button.Hovered() && spec.danger:
		// .text.danger:hover: --red on --red-soft.
		button = button.Background(hoverColor(c, button, "settings-action-bg", theme.Surface, dangerSoftFor(theme.Dark), true)).
			Border(1, theme.Danger)
		ink = theme.Danger
	case button.Hovered():
		button = button.Background(hoverColor(c, button, "settings-action-bg", theme.Surface, surfaceAltFor(theme.Dark), true)).
			Border(1, rail.TextTertiary)
		ink = theme.Text
	}
	if !spec.disabled && spec.onClick != nil {
		button.OnClick(spec.onClick)
	}
	// `.text:active` gives to .96 over .16s (app.css:4052, :4048-4052); a disabled button is
	// not pressed, so it stays still.
	button = pressScale(button, "press", PressScaleText, MotionPress)
	button.Children(func() {
		if spec.busy {
			// `.busy` takes the pointer and dims the control while the work runs; the
			// spinner takes the glyph's place and the label stays, so the button says what
			// it is doing rather than changing into a different control.
			ui.Spinner(c).Size(SettingsActionIcon, SettingsActionIcon).TextColor(ink)
		} else if spec.icon != "" {
			ui.Box(c).Width(SettingsActionIcon).Height(SettingsActionIcon).
				Draw(func(p *ui.Painter, r ui.Rect) {
					drawIcon(p, spec.icon, r.X, r.Y, min32(r.W, r.H), ink.Alpha(0.8))
				})
		}
		ui.Text(c, spec.label).FontSize(SettingsActionFont).FontWeight(weight).
			TextColor(ink).SingleLine()
		if spec.badge > 0 {
			ui.Text(c, fmt.Sprintf("%d", spec.badge)).FontSize(FontMicro).
				TextColor(theme.Accent).Background(theme.Accent.Alpha(0.14)).
				Padding(1, 6).Radius(RadiusPill).MinWidth(20).Center()
		}
	})
}

// settingsAction is textAction for a row's one-word action.
func settingsAction(c *ui.Context, theme *ui.Theme, rail RailTokens,
	label string, disabled, danger bool, onClick func()) {
	textAction(c, theme, rail, textActionSpec{
		label: label, disabled: disabled, danger: danger, onClick: onClick,
	})
}

// settingsActionBusy is settingsAction while its work is in flight: the label stays and a
// spinner turns in the glyph's place, as magpie's `.busy` controls do (app.css:306, :3456).
func settingsActionBusy(c *ui.Context, theme *ui.Theme, rail RailTokens,
	label string, busy bool, onClick func()) {
	textAction(c, theme, rail, textActionSpec{
		label: label, disabled: busy, busy: busy, onClick: onClick,
	})
}

// settingsIcon is a row's quiet icon action, for the lists and saved views
// that carry two or three of them: a 28-point square, a --muted glyph, and
// the --pill ground under the pointer, as every icon control in the shell.
func settingsIcon(c *ui.Context, theme *ui.Theme, rail RailTokens,
	name, label string, onClick func()) {
	// Label as well as Tooltip: a management row carries the same icon once per row, so the
	// name has to be readable without a pointer. The tooltip is the same words on hover.
	button := ui.Box(c).Width(SettingsIconSize).Height(SettingsIconSize).
		Radius(RadiusSmall).Center().Focusable().Label(label).Tooltip(label)
	button = button.Background(hoverColor(c, button, "settings-icon-"+name,
		ui.Transparent, theme.SurfaceHover, button.Hovered()))
	ink := rail.TextTertiary
	if button.Hovered() {
		ink = theme.Text
	}
	button = pressScale(button, "press", PressScaleIcon, MotionPress)
	button.OnClick(onClick).Draw(func(p *ui.Painter, r ui.Rect) {
		size := min32(r.W, r.H) * 0.58
		drawIcon(p, name, r.X+(r.W-size)/2, r.Y+(r.H-size)/2, size, ink)
	})
}

// settingsNumber is a preference row's numeric value: one compact field, the
// way magpie's .warm-at draws a number (64 wide, right aligned, 27 high),
// with Up and Down stepping it.
//
// The field keeps its own text so a half-typed number stays on screen; the
// value in the row is only adopted once what was typed is a number in range.
func settingsNumber(c *ui.Context, theme *ui.Theme, rail RailTokens,
	value *float64, lo, hi, step float64, label string, onChange func()) {
	holder := ui.Row(c).Shrink(0).AlignItems(ui.Center)
	text := ui.Local[string](holder, "settings-number", func() string {
		return formatSettingsNumber(*value)
	})
	focused := ui.Local[bool](holder, "settings-number-focus", func() bool { return false })
	in := ui.TextInput(c, text).Width(SettingsNumberWidth).Height(SettingsNumberHeight).
		Padding(0, SettingsNumberPadX).Radius(SettingsFieldRadius).
		TextAlign(ui.End).FontSize(SettingsFieldFont).Label(label)
	commit := func(v float64) {
		if v < lo {
			v = lo
		}
		if v > hi {
			v = hi
		}
		if v == *value {
			return
		}
		*value = v
		*text = formatSettingsNumber(v)
		onChange()
	}
	in.OnChange(func() {
		v, err := strconv.ParseFloat(strings.TrimSpace(*text), 64)
		if err != nil {
			return
		}
		// Clamped rather than refused: a refused value leaves the field holding a
		// number the setting never took, and nothing says so.
		if v < lo {
			v = lo
		}
		if v > hi {
			v = hi
		}
		if v != *value {
			*value = v
			onChange()
		}
	})
	in.OnShortcut(0, ui.KeyUp, func() { commit(*value + step) })
	in.OnShortcut(0, ui.KeyDown, func() { commit(*value - step) })
	// Outside the field the value in force is what shows: a number typed out of
	// range, or one the app changed, must not leave stale text behind.
	//
	// Taking the focus empties the field, the way selecting the whole value would:
	// without it a click lands the caret after "25" and typing "50" makes "2550",
	// which the range check refuses -- so the setting looked impossible to change.
	// An empty field cannot append, and the number returns as soon as it loses focus.
	if in.Focused() {
		if !*focused {
			*focused = true
			*text = ""
		}
	} else {
		*focused = false
		*text = formatSettingsNumber(*value)
	}
}

// formatSettingsNumber writes a setting the way the field shows it: whole
// numbers without a decimal point, and nothing else padded.
func formatSettingsNumber(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// settingsField is the text field of a preference row: `.row.pref .set-field`, 27 high,
// a 7 radius, 0 9 inset and a 12.5px label. width 0 lets it take the rest of the row,
// which is what a row's one long value does; anything else is a fixed measure.
func settingsField(c *ui.Context, theme *ui.Theme, rail RailTokens,
	value *string, placeholder, label string, width float32, onChange func()) {
	field := settingsInput(c, theme, rail, value).Placeholder(placeholder).Label(label)
	if width > 0 {
		field = field.Width(width)
	} else {
		field = field.Grow(1)
	}
	if onChange != nil {
		field.OnChange(onChange)
	}
}

// settingsInput is the field itself, for the forms that need to chain their
// own placeholder, binding or focus onto it.
func settingsInput(c *ui.Context, theme *ui.Theme, rail RailTokens, value *string) ui.Element {
	return ui.TextInput(c, value).Height(SettingsFieldHeight).
		Padding(0, SettingsFieldPadX).Radius(SettingsFieldRadius).
		Border(1, rail.CardBorder).Background(theme.Surface).
		FontSize(SettingsFieldFont)
}

// settingsQuiet is a row's quiet value: the time a run finished, the tag a
// keyword suggests -- a reading rather than an action, so no button box.
func settingsQuiet(c *ui.Context, theme *ui.Theme, text string) {
	ui.Text(c, text).FontSize(SettingsRowSubFont).TextColor(theme.TextMuted).SingleLine()
}

// settingsFindField is the page's search: magpie's .sess-filter, a plain
// bordered field 26 high, with no magnifier glyph.
//
// It returns its own element rather than growing or shrinking itself: the
// caller draws it at one call site and only changes its width, so the caret
// survives the first character typed (round 41).
func settingsFindField(c *ui.Context, theme *ui.Theme, rail RailTokens,
	query *string, placeholder string, handle *ui.Handle, onChange func()) ui.Element {
	field := ui.TextInput(c, query).Height(SettingsFindHeight).
		Radius(SettingsFindRadius).Padding(0, SettingsFindPadX).
		Border(1, rail.CardBorder).Background(theme.Surface).
		FontSize(SettingsFieldFont).Placeholder(placeholder).Label(placeholder).
		Bind(handle).Width(SettingsFindWidth)
	field.OnChange(onChange)
	return field
}

// settingsStatus is a row's state: a dot in its colour and the word.
func settingsStatus(c *ui.Context, theme *ui.Theme, rail RailTokens, label string, colour ui.Color) {
	ui.Row(c).Shrink(0).GapX(6).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Width(StatusDotSize).Height(StatusDotSize).
			Background(colour).Radius(RadiusPill)
		ui.Text(c, label).FontSize(FontSmall).TextColor(colour).SingleLine()
	})
}

// dangerSoftFor is --red-soft: the ground behind a destructive hover.
func dangerSoftFor(dark bool) ui.Color {
	if dark {
		return darkDangerFill
	}
	return lightDangerFill
}
