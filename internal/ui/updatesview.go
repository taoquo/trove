package ui

import (
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/internal/i18n"
)

// Updates is what the update card on Settings › General reads and calls.
//
// It is a seam rather than a direct call because the update window and the check behind it
// belong to the updater plugin, which the app registers and a test does not: internal/ui draws
// the page, and the app decides what checking means.
type Updates struct {
	// Version is what this bundle says it is, the same string the About panel shows.
	Version string
	// Check opens the update window, as the app menu's item does.
	Check func()
	// AutomaticChecks is the preference the row's switch shows; SetAutomaticChecks writes it.
	AutomaticChecks    func() bool
	SetAutomaticChecks func(bool)
	// LastCheck is when a check last finished, zero before the first one.
	LastCheck func() time.Time
}

// defaultUpdates is what a shell built outside the app gets: a test, or a capture run. It has
// checked nothing and changes nothing, which is what those runs do.
func defaultUpdates() *Updates {
	return &Updates{
		Version:            "dev",
		Check:              func() {},
		AutomaticChecks:    func() bool { return true },
		SetAutomaticChecks: func(bool) {},
		LastCheck:          func() time.Time { return time.Time{} },
	}
}

// UseUpdates wires the update card to the updater plugin. The app calls it once, at wiring
// time, with the plugin's own functions: nothing is called here, because the plugin is not
// listening until the app runs. The page reads them when it opens.
func (s *Shell) UseUpdates(updates Updates) {
	s.updates = &updates
}

// loadUpdatePreferences reads the two values the update rows show. It runs when the page opens,
// never in a frame: the plugin keeps them in a file of its own, and a view does no I/O.
func (s *Shell) loadUpdatePreferences() {
	if s.updates == nil {
		return
	}
	s.state.UpdateAutoCheck = s.updates.AutomaticChecks()
	s.state.UpdateLastCheck = s.updates.LastCheck()
}

// settingsUpdatesSection is the update card on 通用: which version this is, when it last looked
// for a newer one, and the switch that decides whether it keeps looking.
//
// The app menu carries the same check, next to About. That is where someone who already knows
// the app looks; this is where someone asking "how do I update" looks, which is the page they
// opened to find out how anything else about the app is set. It sits under General rather than
// on a tab of its own: one version line is not a section.
func (s *Shell) settingsUpdatesSection(c *ui.Context, theme *ui.Theme, rail RailTokens) int {
	updates := s.updates
	if updates == nil {
		updates = defaultUpdates()
	}
	return s.settingsCard(c, theme, rail,
		settingsItem{
			keys: []i18n.Key{i18n.SettingsVersion, i18n.SettingsVersionLine, i18n.MenuCheckUpdates},
			name: s.t().T(i18n.SettingsVersion),
			sub:  s.versionLine(updates.Version),
			control: func() {
				settingsAction(c, theme, rail, s.t().T(i18n.MenuCheckUpdates), false, false, updates.Check)
			},
		},
		settingsItem{
			keys: []i18n.Key{i18n.SettingsAutoCheck, i18n.SettingsAutoCheckHint},
			name: s.t().T(i18n.SettingsAutoCheck),
			sub:  s.t().T(i18n.SettingsAutoCheckHint),
			control: func() {
				settingsSwitch(c, theme, rail, s.t().T(i18n.SettingsOff), s.t().T(i18n.SettingsOn),
					s.t().T(i18n.SettingsAutoCheck), &s.state.UpdateAutoCheck, func() {
						updates.SetAutomaticChecks(s.state.UpdateAutoCheck)
					})
			},
		},
	)
}

// versionLine is the version and when the app last finished a check -- "trove <version> ·
// last checked 2026-11-14 11:34" -- or that it has not checked yet. A date is local time and minute
// precision: seconds would be noise, and the day is what the reader is asking about.
func (s *Shell) versionLine(version string) string {
	if s.state.UpdateLastCheck.IsZero() {
		return s.t().Tf(i18n.SettingsVersionLineNever, version)
	}
	return s.t().Tf(i18n.SettingsVersionLine, version, s.state.UpdateLastCheck.Format("2006-01-02 15:04"))
}
