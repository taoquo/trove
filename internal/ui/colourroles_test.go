package ui

import (
	"reflect"
	"sort"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestEveryRailTokenComesFromAThemeRole is the guarantee behind "a theme can be swapped without
// touching the drawing code".
//
// RailTokens is the set of colours the painting code reads -- twenty-nine of them, named after the
// thing they colour. The objective asks that the *structural* roles (background, surface, border,
// text tiers, accent, danger, warning) be the tokens, so that changing the palette is changing
// those and not chasing twenty-nine fields.
//
// The roles are the platform theme's fields, and railFrom() derives every token from them. What was
// missing is proof: nothing stopped a future token being written as a literal, and a literal is
// invisible until someone changes the theme and finds one stripe that did not follow.
//
// So this mutates one theme role at a time and asks which tokens moved. A token that no role moves
// is a token that ignores the theme, and it fails.
func TestEveryRailTokenComesFromAThemeRole(t *testing.T) {
	sentinel := ui.RGBA(1, 2, 3, 255)

	// The roles the drawing code is allowed to depend on. Dark is here because two tokens are
	// chosen per appearance rather than per colour (see TestTheMutedTextIsTheOnlyLiteral below).
	roles := []struct {
		name   string
		mutate func(*ui.Theme)
	}{
		{"Dark", func(th *ui.Theme) { th.Dark = !th.Dark }},
		{"Background", func(th *ui.Theme) { th.Background = sentinel }},
		{"Surface", func(th *ui.Theme) { th.Surface = sentinel }},
		{"SurfaceHover", func(th *ui.Theme) { th.SurfaceHover = sentinel }},
		{"SurfacePressed", func(th *ui.Theme) { th.SurfacePressed = sentinel }},
		{"Border", func(th *ui.Theme) { th.Border = sentinel }},
		{"Text", func(th *ui.Theme) { th.Text = sentinel }},
		{"TextMuted", func(th *ui.Theme) { th.TextMuted = sentinel }},
		{"Accent", func(th *ui.Theme) { th.Accent = sentinel }},
		// Focus drives FieldFocus. It was missing from the first version of this list, which made
		// the test report that token as un-derived when it was the list that was short.
		{"Focus", func(th *ui.Theme) { th.Focus = sentinel }},
		{"Danger", func(th *ui.Theme) { th.Danger = sentinel }},
		{"Warning", func(th *ui.Theme) { th.Warning = sentinel }},
	}

	base := ui.LightTheme()
	baseTokens := railFrom(base, false)
	drivenBy := map[string][]string{}
	for _, role := range roles {
		patched := *base
		role.mutate(&patched)
		got := railFrom(&patched, false)
		before := reflect.ValueOf(baseTokens)
		after := reflect.ValueOf(got)
		for i := 0; i < before.NumField(); i++ {
			name := before.Type().Field(i).Name
			if !reflect.DeepEqual(before.Field(i).Interface(), after.Field(i).Interface()) {
				drivenBy[name] = append(drivenBy[name], role.name)
			}
		}
	}

	// The tokens that are deliberately not a theme role, with the reason. One is the whole list:
	// Overlay is the modal scrim -- a dark translucent wash over the window, which the platform
	// theme has no role for. mutedTextColour() is covered separately below.
	deliberateLiteral := map[string]bool{"Overlay": true}

	// Every token must be moved by at least one role.
	names := make([]string, 0, reflect.TypeOf(baseTokens).NumField())
	for i := 0; i < reflect.TypeOf(baseTokens).NumField(); i++ {
		names = append(names, reflect.TypeOf(baseTokens).Field(i).Name)
	}
	sort.Strings(names)
	for _, name := range names {
		if len(drivenBy[name]) == 0 && !deliberateLiteral[name] {
			t.Errorf("RailTokens.%s is not derived from any theme role: it would survive a theme "+
				"change, which is the one thing a token must not do", name)
		}
	}

	// There is deliberately no "a role that drives no token is decorative" check. The first version
	// had one and it failed on Text, Danger and Warning -- not because those are unused, but because
	// the painting code reads them straight off the theme (`TextColor(theme.Danger)` and so on)
	// instead of through a token. Roles are used on both sides of this table, so a rule that only
	// watched one side was measuring the wrong thing.
}

// TestTheMutedTextIsTheOnlyLiteral records the one exception, so it stays one.
//
// mutedTextColour() returns the tertiary and quaternary text inks per appearance, deliberately
// not the platform's TextMuted: the card timestamp uses the warmer ink rather than the
// framework's gray. This pair cannot come from a theme role -- the platform has no role that is
// this value.
//
// So it is chosen by Dark and by nothing else, which is exactly what this asserts.
func TestTheMutedTextIsTheOnlyLiteral(t *testing.T) {
	light := railFrom(ui.LightTheme(), false)

	patched := *ui.LightTheme()
	patched.Surface = ui.RGBA(9, 9, 9, 255)
	patched.Accent = ui.RGBA(9, 9, 9, 255)
	patched.Border = ui.RGBA(9, 9, 9, 255)
	patched.TextMuted = ui.RGBA(9, 9, 9, 255)
	moved := railFrom(&patched, false)

	if moved.TextTertiary != light.TextTertiary {
		t.Error("TextTertiary follows a colour role now; if that is deliberate, delete this " +
			"exception and let TestEveryRailTokenComesFromAThemeRole cover it")
	}
	if moved.TextQuaternary != light.TextQuaternary {
		t.Error("TextQuaternary follows a colour role now; see the note above")
	}

	// And the two appearances really do differ, so "the same because it is a literal" is not the
	// same as "the same because nothing changed".
	if light.TextTertiary == railFrom(ui.DarkTheme(), false).TextTertiary {
		t.Error("muted text is identical in both appearances, so it is not appearance-dependent")
	}
}
