package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/internal/i18n"
)

// The settings page's controls.
//
// The page is drawn as cards of preference rows rather than as a form:
//
//	list card       a card ground, one hairline, a 12 radius
//	preference row  48 high, 12/12/12/14 inset, 14 gap
//	the row name    weight 500, 13px, the body size
//	the sub-line    11.5px, the muted ink, 2 above, wrapping
//	row + row       one subtle hairline above
//	row head label  11px, 600, .05em, uppercase
//	the pick        24 high, 0 6 0 9 inset, a 7 radius, the tray ground, 12px
//	the segments    2 inset, a 7 radius, the tray ground;
//	                each option 22 high, 0 10 inset, a 5 radius, 12px
//	the text action 27 high, 0 10 inset, one hairline, a 6 radius,
//	                a card ground, the muted ink, 12px
//	the number field 24 high, 0 8 inset, one hairline, a 7 radius;
//	                a number is 72 wide, its value to the right
//	the find field  26 high, 0 9 inset, a 7 radius, 12px
//
// The page
// before this file was a stack of labelled form fields, which reads as a form to fill in; a
// settings list is a set of switches and values, and the row is the unit.

// settingsItem is one preference row before it is drawn.
//
// name and sub are already translated: the caller owns the copy, so a row can
// carry a list's own name as easily as a catalog key.
type settingsItem struct {
	// keys are the rows' own words for the find field. A row is found by its
	// name and its explanation, which is what the find field matches on.
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
// The sub-line is the muted ink and wraps: a setting's explanation is read whole,
// and one cut with an ellipsis hides what the setting does.
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

// settingsSectionLabel is the label that names a group of cards: 11px, 600,
// .05em, uppercase, the muted ink, and no rule of its own.
func settingsSectionLabel(c *ui.Context, theme *ui.Theme, rail RailTokens, label string) {
	ui.Text(c, strings.ToUpper(label)).FontSize(SettingsGroupTitle).FontWeight(600).
		LetterSpacing(trackingEm(SettingsGroupTrack, SettingsGroupTitle)).
		TextColor(rail.TextTertiary).Padding(0, 2)
}

// settingsNote is the sentence a group of rows says about itself before them:
// 12px in the muted ink over the card, not in it.
func settingsNote(c *ui.Context, theme *ui.Theme, rail RailTokens, text string) {
	ui.Text(c, text).FontSize(FontSmall).TextColor(rail.TextTertiary).Padding(0, 4)
}

// settingsPick is the pick of a preference row: a pill with the current
// value and a caret, which opens the choice.
//
// ui.Select is the control rather than a hand-drawn one because it already
// carries the part a hand-drawn pill cannot: the menu marks the value in
// force and the arrows move through it. Its trigger is re-styled here; its
// menu is the toolkit's own panel, the pop card.
func settingsPick(c *ui.Context, theme *ui.Theme, rail RailTokens,
	label string, value *string, options []string, onChange func()) {
	pick := ui.Select(c, value, options).Height(SettingsPickHeight).
		Radius(SettingsPickRadius).Padding(0, SettingsPickPadRight, 0, SettingsPickPadLeft).
		BorderWidth(0).MinWidth(0).Label(label)
	pick = pick.Background(hoverColor(c, pick, "settings-pick", rail.Muted, theme.SurfacePressed, pick.Hovered()))
	pick.OnChange(onChange)
}

// warmTab is one tab: 22 high, 7-point side padding, a 6 radius,
// an 11px 600 .05em uppercase label in the muted ink, and the open one on the tray
// ground. mark puts a dot after the label, for a page the
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

// settingsSwitch is the segment pair of a preference row: an Off/On pair in a tray-ground
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

// settingsSegment is one option of a switch.
func settingsSegment(c *ui.Context, theme *ui.Theme, rail RailTokens, label string, on bool, choose func()) {
	seg := ui.Row(c).Height(SettingsSegOptionHeight).Padding(0, SettingsSegOptionPadX).
		Radius(SettingsSegOptionRadius).Center().Focusable().Label(label).OnClick(choose)
	if on {
		// The thumb is the card colour over the tray ground with the segmented edge's drop;
		// the track is a pill, not the header's deeper track.
		seg = seg.Background(segThumbFor(theme.Dark)).
			Shadow(0, 1, 2, 0, segShadowFor(theme.Dark))
	}
	// An unselected option is the tertiary ink, the chosen one the strong ink, and hover
	// only darkens the word one step to the muted ink: no ground appears.
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
	// where the glyph would be, as busy controls do.
	busy   bool
	danger bool
	// destructive is the button that ends something: the danger ink on the danger-soft
	// ground at rest, filling to the danger ink under the pointer. It is not `danger`, which
	// only tints the hover of an action that is otherwise ordinary -- a confirmation's
	// "yes" says what it will do before it is pressed.
	destructive bool
	// autoFocus gives the button the keyboard focus in the frame it appears, which is how
	// a dialog puts Return on the safe answer.
	autoFocus bool
	// fill stretches the button across its row, for the one action a narrow
	// panel puts under everything else.
	fill bool
	// height overrides the 27-point action height where the button has to sit
	// in a row of shorter controls (the filter chips' clear-all).
	height  float32
	onClick func()
}

// textAction is the one labelled button in this
// shell: 27 high, 10-point side padding, a 6 radius, one hairline edge, the card
// ground, a 12px muted-ink label and a breath of shadow.
//
//	hover          a hairline at the muted ink, a raised row ground, the strong ink
//	danger hover   the danger ink on the danger-soft ground
//	primary        the accent's foreground on the accent, weight 500
//
// It is the settings rows', the toolbars' and the dialogs' button: one button
// whose ground changes, not a family of them.
func textAction(c *ui.Context, theme *ui.Theme, rail RailTokens, spec textActionSpec) {
	height := float32(SettingsActionHeight)
	if spec.height > 0 {
		height = spec.height
	}
	button := ui.Row(c).Height(height).
		Padding(0, SettingsActionPadX).Radius(SettingsActionRadius).
		Border(1, rail.CardBorder).Background(theme.Surface).
		GapX(SettingsActionGap).AlignItems(ui.Center).Focusable().
		Label(spec.label).Shadow(0, 1, 1, 0, ui.RGBA(25, 21, 20, 0.04))
	if spec.fill {
		button = button.FillWidth().Center()
	} else {
		button = button.Shrink(0)
	}
	// A busy control dims to .55 and takes no pointer: the work is running, so the control
	// cannot be pressed again while it does.
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
	case spec.destructive:
		// At rest the danger ink on its soft ground -- readable, but plainly a warning
		// rather than the path of least resistance. Under the pointer the ground fills, so
		// the press the reader is about to make is the one they see.
		hovering := button.Hovered()
		button = button.
			Background(hoverColor(c, button, "confirm-action-bg", dangerSoftFor(theme.Dark), theme.Danger, hovering)).
			Border(1, theme.Danger)
		ink = hoverColor(c, button, "confirm-action-ink", theme.Danger, theme.AccentText, hovering)
		weight = 500
	case button.Hovered() && spec.danger:
		// The danger action under the pointer: the danger ink on the danger-soft ground.
		button = button.Background(hoverColor(c, button, "settings-action-bg", theme.Surface, dangerSoftFor(theme.Dark), true)).
			Border(1, theme.Danger)
		ink = theme.Danger
	case button.Hovered():
		button = button.Background(hoverColor(c, button, "settings-action-bg", theme.Surface, surfaceAltFor(theme.Dark), true)).
			Border(1, rail.TextTertiary)
		ink = theme.Text
	}
	if spec.autoFocus && !spec.disabled {
		button = button.AutoFocus()
	}
	if !spec.disabled && spec.onClick != nil {
		button.OnClick(spec.onClick)
	}
	// A pressed text action gives to .96 over .16s; a disabled button is not pressed, so it
	// stays still.
	button = pressScale(button, "press", PressScaleText, MotionPress)
	button.Children(func() {
		if spec.busy {
			// A busy control takes the pointer and dims the control while the work runs; the
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
// spinner turns in the glyph's place, as busy controls do.
func settingsActionBusy(c *ui.Context, theme *ui.Theme, rail RailTokens,
	label string, busy bool, onClick func()) {
	textAction(c, theme, rail, textActionSpec{
		label: label, disabled: busy, busy: busy, onClick: onClick,
	})
}

// settingsIcon is a row's quiet icon action, for the lists and saved views
// that carry two or three of them: a 28-point square, a muted glyph, and
// the tray ground under the pointer, as every icon control in the shell.
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

// settingsNumber is a preference row's numeric value: one compact field,
// 64 wide, right aligned and 27 high, with Up and Down stepping it.
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

// settingsField is the text field of a preference row: 27 high,
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

// settingsFindField is the page's search: a plain
// bordered field 26 high, with no magnifier glyph.
//
// It returns its own element rather than growing or shrinking itself: the
// caller draws it at one call site and only changes its width, so the caret
// survives the first character typed.
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

// dangerSoftFor is the danger-soft ground: the ground behind a destructive hover.
func dangerSoftFor(dark bool) ui.Color {
	if dark {
		return darkDangerFill
	}
	return lightDangerFill
}
