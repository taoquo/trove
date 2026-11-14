package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/internal/i18n"
	"github.com/taoquo/trove/internal/xsource"
)

// The settings page.
//
// Settings are a page, not a sheet: the sidebar stays, the content
// column becomes the settings surface, and the reader goes back the way they
// came. The page is one row of tabs with a find field at its right, then the
// open tab as cards of preference rows (settingsui.go holds the row grammar).
//
// The title sits one step down and on the same line as the tabs, because this shell's
// navigation is the sidebar rather than a settings tab in a global nav, and a page with no
// name at all would be the only surface in the window without one.
const (
	SettingsSectionGeneral  = "general"
	SettingsSectionSync     = "sync"
	SettingsSectionDataSafe = "data-safety"
	// SettingsSectionListsViews is where the lists and saved views are managed. They do not
	// belong at the end of the data-safety card, under "backups and restores stay on this
	// Mac", which describes neither.
	SettingsSectionListsViews = "lists-views"
)

// settingsPage draws settings as a page of the results pane.
func (s *Shell) settingsPage(c *ui.Context, theme *ui.Theme, rail RailTokens) {
	if s.state.SettingsSection == "" {
		s.state.SettingsSection = SettingsSectionGeneral
	}

	ui.Column(c).Fill().FillHeight().Background(rail.Results).
		Padding(ResultsPadTop, s.pageInset(c), ResultsPadBottom, s.pageInset(c)).Children(func() {
		s.settingsPageHeader(c, theme, rail)

		ui.Scroll(c).FillWidth().Grow(1).Children(func() {
			ui.Column(c).FillWidth().GapY(SettingsCardGap).
				Padding(SettingsPagePadTop, 0, SettingsPagePadBottom, 0).Children(func() {
				if s.settingsFiltering() {
					s.settingsFilteredSections(c, theme, rail)
					return
				}
				switch s.state.SettingsSection {
				case SettingsSectionSync:
					s.settingsSyncSection(c, theme, rail)
				case SettingsSectionDataSafe:
					s.settingsDataSafetySection(c, theme, rail)
				case SettingsSectionListsViews:
					s.settingsListsViewsSection(c, theme, rail)
				default:
					s.settingsGeneralSection(c, theme, rail)
				}
			})
		})
	})
}

// settingsPageHeader is the way back, the page's name, the tabs and the find
// field, on one line.
//
// The head is the tabs with the search at their right; the name and the way
// back are this shell's own, because the sidebar is the navigation here and
// the page is one level deep. While the find field holds words the tabs step
// aside and the field takes the row, which is exactly what searching does (.set-tabs
// display:none, .set-q flex 1 1 100%).
func (s *Shell) settingsPageHeader(c *ui.Context, theme *ui.Theme, rail RailTokens) {
	ui.Row(c).FillWidth().MinHeight(SettingsPageHeaderHeight).
		AlignItems(ui.Center).Children(func() {
		// The way back is one square, as every icon control in this shell is.
		// The page name beside it says where the reader is.
		// Named as well as tooltipped, like the graph page's and the tag library's back
		// controls: this is the only way out of the page with a pointer, and a tooltip is a
		// hover hint, not a name.
		back := ui.Box(c).Width(SettingsBackControl).Height(SettingsBackControl).
			Radius(RadiusSmall).Center().Focusable().
			Label(s.t().T(i18n.SettingsBack)).
			Tooltip(s.t().T(i18n.SettingsBack))
		back = back.Background(hoverColor(c, back, "settings-back", ui.Transparent, theme.SurfaceHover, back.Hovered()))
		back.OnClick(func() { s.CloseSettings() }).Draw(func(p *ui.Painter, r ui.Rect) {
			side := min32(r.W, r.H) - 12
			drawIcon(p, "back", r.X+(r.W-side)/2, r.Y+(r.H-side)/2, side, rail.TextTertiary)
		})
		ui.Text(c, s.t().T(i18n.SettingsTitle)).
			Font(s.displayFace()).FontSize(FontSidebarTitleCompact).FontWeight(600).
			TextColor(theme.Text).Padding(0, 10, 0, 4)

		// The field is one element in one place whatever the page is doing. Two call sites --
		// a grown one while filtering, a fixed one otherwise -- would replace the element on
		// the first character typed and the caret would fall out, so the search would accept
		// exactly one letter. The tabs and the spacer keep their slots and go Invisible
		// instead of being skipped, so the field keeps its identity from frame to frame.
		filtering := s.settingsFiltering()
		tabs := s.settingsTabs(c, theme, rail)
		if filtering {
			tabs = tabs.Invisible()
		}
		spacer := ui.Spacer(c).FillWidth()
		if filtering {
			spacer = spacer.Invisible()
		}
		field := settingsFindField(c, theme, rail, &s.state.SettingsQuery,
			s.t().T(i18n.SettingsSearchPlaceholder),
			&s.settingsFindField, func() { s.invalidate() })
		if filtering {
			field = field.Grow(1)
		}
	})
}

// settingsTabs is the row of tabs: 22 high, a 6 radius, an 11px
// 600 .05em uppercase label, and the open one on a tray ground with no
// thumb of its own.
func (s *Shell) settingsTabs(c *ui.Context, theme *ui.Theme, rail RailTokens) ui.Element {
	tabs := ui.Row(c).GapX(SettingsTabGap).Wrap().AlignItems(ui.Center).
		Label(s.t().T(i18n.SettingsTitle))
	tabs.Children(func() {
		for _, section := range []struct {
			id    string
			label i18n.Key
		}{
			{SettingsSectionGeneral, i18n.SettingsSectionGeneral},
			{SettingsSectionSync, i18n.SettingsSectionSync},
			{SettingsSectionDataSafe, i18n.SettingsSectionDataSafety},
			{SettingsSectionListsViews, i18n.SettingsSectionListsViews},
		} {
			section := section
			warmTab(c, theme, rail, s.t().T(section.label),
				s.state.SettingsSection == section.id, false, func() {
					s.state.SettingsSection = section.id
					// Choosing a tab leaves the search: the reader asked for that section,
					// and a query still filtering it would look like an empty page.
					s.state.SettingsQuery = ""
					if s.window != nil {
						s.window.Invalidate()
					}
				})
		}
	})
	return tabs
}

// settingsFiltering reports whether the find field is narrowing the page.
func (s *Shell) settingsFiltering() bool {
	return strings.TrimSpace(s.state.SettingsQuery) != ""
}

// settingsShows reports whether a row belongs in the filtered page.
//
// The keys are every word the row shows, so a query can match the label, its
// hint or its button: a reader who types "keychain" finds the cookie block by
// its explanation rather than by a label that does not use the word.
func (s *Shell) settingsShows(keys ...i18n.Key) bool {
	query := strings.ToLower(strings.TrimSpace(s.state.SettingsQuery))
	if query == "" {
		return true
	}
	for _, key := range keys {
		if strings.Contains(strings.ToLower(s.t().T(key)), query) {
			return true
		}
	}
	return false
}

// settingsSectionKeys is what each section can be found by.
//
// It exists so the filtered page can tell an empty search from a section that
// simply has no match. It is a separate list from the drawing on purpose: a
// section that forgets an entry here loses only its heading in a filtered
// view, never the row itself.
var settingsSectionKeys = map[string][]i18n.Key{
	SettingsSectionGeneral: {
		i18n.SettingsLanguage, i18n.SettingsLanguageHint, i18n.ThemeLabel, i18n.SettingsThemeHint,
		i18n.SettingsVersion, i18n.SettingsVersionLine, i18n.SettingsVersionLineNever,
		i18n.MenuCheckUpdates, i18n.SettingsAutoCheck, i18n.SettingsAutoCheckHint,
	},
	SettingsSectionSync: {
		i18n.SettingsSyncOnLaunch, i18n.SettingsSaveRawPayloads,
		i18n.SettingsStopBufferPages, i18n.SettingsPageLimit,
		i18n.SettingsXCookiesTitle, i18n.SettingsXCookiesHint,
		i18n.SettingsXBrowserRow, i18n.SettingsXBrowserRead, i18n.SettingsXBrowserHint,
		i18n.SettingsXTestRow, i18n.SettingsXTestConnection, i18n.SettingsXTestHint,
		i18n.SettingsXCredentialPaste, i18n.SettingsXCredentialPasteHint,
		i18n.ActionSaveCookies,
	},
	SettingsSectionDataSafe: {
		i18n.SettingsData, i18n.SettingsDataSafetyHint,
		i18n.SettingsBackupRow, i18n.ExportData, i18n.SettingsBackupHint,
		i18n.SettingsLegacyBackupRow, i18n.BundleExportLegacy, i18n.SettingsLegacyBackupHint,
		i18n.SettingsRestoreRow, i18n.RestoreBackup, i18n.SettingsRestoreHint,
		i18n.SettingsLocalRow, i18n.ResetLocalData, i18n.SettingsLocalHint,
		i18n.SettingsBlockedTags, i18n.SettingsClearBlockedTags,
	},
	SettingsSectionListsViews: {
		i18n.SidebarLists, i18n.SettingsUnclassifiedHint, i18n.SettingsInboxHint,
		i18n.SidebarUnclassified, i18n.SidebarNoLists, i18n.SidebarNewList,
		i18n.SettingsNewList, i18n.RenameListLabel, i18n.ActionOpenNamed,
		i18n.ActionRenameNamed, i18n.ActionDeleteNamed,
		i18n.SavedViewsTitle, i18n.SavedViewNamePlaceholder, i18n.SavedViewApply,
		i18n.SavedViewApplyNamed, i18n.SaveCurrentView, i18n.DeleteSavedView,
		i18n.SettingsNewView,
	},
}

// settingsFilteredSections draws every matching section under its own name.
func (s *Shell) settingsFilteredSections(c *ui.Context, theme *ui.Theme, rail RailTokens) {
	drawn := 0
	for _, section := range []struct {
		id    string
		label i18n.Key
		draw  func(*ui.Context, *ui.Theme, RailTokens) int
	}{
		{SettingsSectionGeneral, i18n.SettingsSectionGeneral, s.settingsGeneralSection},
		{SettingsSectionSync, i18n.SettingsSectionSync, s.settingsSyncSection},
		{SettingsSectionDataSafe, i18n.SettingsSectionDataSafety, s.settingsDataSafetySection},
		{SettingsSectionListsViews, i18n.SettingsSectionListsViews, s.settingsListsViewsSection},
	} {
		if !s.settingsShows(settingsSectionKeys[section.id]...) {
			continue
		}
		settingsSectionLabel(c, theme, rail, s.t().T(section.label))
		drawn += section.draw(c, theme, rail)
	}
	if drawn == 0 {
		ui.Text(c, s.t().Tf(i18n.SettingsNoMatch, strings.TrimSpace(s.state.SettingsQuery))).
			FontSize(FontBody).TextColor(theme.TextMuted).Padding(18, 2)
	}
}

// CloseSettings leaves the page for the list it replaced.
func (s *Shell) CloseSettings() {
	if !s.state.SettingsOpen {
		return
	}
	s.leaveSettings()
	s.markPageChange()
	if s.window != nil {
		s.window.Invalidate()
	}
}

// settingsGeneralSection is 通用: language first, theme second. Each is one row
// with its pick.
func (s *Shell) settingsGeneralSection(c *ui.Context, theme *ui.Theme, rail RailTokens) int {
	drawn := s.settingsCard(c, theme, rail,
		settingsItem{
			keys: []i18n.Key{i18n.SettingsLanguage, i18n.SettingsLanguageHint},
			name: s.t().T(i18n.SettingsLanguage),
			sub:  s.t().T(i18n.SettingsLanguageHint),
			control: func() {
				settingsPick(c, theme, rail, s.t().T(i18n.SettingsLanguage),
					&s.state.LocaleLabel, s.localeLabels(), func() {
						s.SetLocale(s.localeFor(s.state.LocaleLabel))
						// The labels themselves are translated, so they change with the language
						// and have to be re-derived.
						s.syncPreferenceLabels()
						s.ReloadAsync(context.Background())
					})
			},
		},
		settingsItem{
			keys: []i18n.Key{i18n.ThemeLabel, i18n.SettingsThemeHint},
			name: s.t().T(i18n.ThemeLabel),
			sub:  s.t().T(i18n.SettingsThemeHint),
			control: func() {
				settingsPick(c, theme, rail, s.t().T(i18n.ThemeLabel),
					&s.state.AppearanceLabel, s.appearanceLabels(), func() {
						s.SetAppearance(s.appearanceFor(s.state.AppearanceLabel))
					})
			},
		},
	)

	// The update rows, under the same tab as the language and the theme: they answer the same
	// question as those two -- how this copy of the app behaves and what it is -- and one
	// version line does not earn a tab of its own in a row of five.
	return drawn + s.settingsUpdatesSection(c, theme, rail)
}

// settingsSyncSection is 同步: the sync preferences, then the X credentials a
// desktop build has to be given.
func (s *Shell) settingsSyncSection(c *ui.Context, theme *ui.Theme, rail RailTokens) int {
	save := func() { s.saveSyncPreferences(context.Background()) }
	drawn := s.settingsCard(c, theme, rail,
		settingsItem{
			keys: []i18n.Key{i18n.SettingsSyncOnLaunch},
			name: s.t().T(i18n.SettingsSyncOnLaunch),
			control: func() {
				settingsSwitch(c, theme, rail,
					s.t().T(i18n.SettingsOff), s.t().T(i18n.SettingsOn),
					s.t().T(i18n.SettingsSyncOnLaunch),
					&s.state.SyncOnLaunch, save)
			},
		},
		settingsItem{
			keys: []i18n.Key{i18n.SettingsSaveRawPayloads},
			name: s.t().T(i18n.SettingsSaveRawPayloads),
			control: func() {
				settingsSwitch(c, theme, rail,
					s.t().T(i18n.SettingsOff), s.t().T(i18n.SettingsOn),
					s.t().T(i18n.SettingsSaveRawPayloads),
					&s.state.SaveRawPayloads, save)
			},
		},
		settingsItem{
			keys: []i18n.Key{i18n.SettingsStopBufferPages, i18n.SettingsStopBufferHint},
			name: s.t().T(i18n.SettingsStopBufferPages),
			sub:  s.t().T(i18n.SettingsStopBufferHint),
			control: func() {
				settingsNumber(c, theme, rail, &s.state.StopBufferPages, 1, 20, 1,
					s.t().T(i18n.SettingsStopBufferPages), save)
			},
		},
		settingsItem{
			keys: []i18n.Key{i18n.SettingsPageLimit, i18n.SettingsPageLimitHint},
			name: s.t().T(i18n.SettingsPageLimit),
			sub:  s.t().T(i18n.SettingsPageLimitHint),
			control: func() {
				settingsNumber(c, theme, rail, &s.state.SyncPageLimit, 0, 200000, 1000,
					s.t().T(i18n.SettingsPageLimit), save)
			},
		},
	)

	// The credentials, which a desktop build has to be told.
	//
	// Without this block the sync could never succeed: Store.SaveXCredentials would be called
	// from nowhere, the Keychain would stay empty, and every run would fail with
	// "credentials: nothing stored (x.auth_token)".
	//
	// It is a card of its own. A muted group label above it was this card's own title word
	// for word: the reader met "X credentials" twice in a row, with nothing between them. The
	// sync section has no group heading above the cookies block either, and the page's first
	// half already runs without one, so the card's title is the only name it needs.
	credentialKeys := []i18n.Key{
		i18n.SettingsXCookiesTitle, i18n.SettingsXCookiesHint,
		i18n.SettingsXBrowserRow, i18n.SettingsXBrowserRead, i18n.SettingsXBrowserHint,
		i18n.SettingsXTestRow, i18n.SettingsXTestConnection, i18n.SettingsXTestHint,
		i18n.SettingsXCredentialPaste, i18n.SettingsXCredentialPasteHint,
		i18n.ActionSaveCookies,
	}
	if !s.settingsShows(credentialKeys...) {
		return drawn
	}
	statusKey := i18n.SettingsXCookiesAbsent
	statusColour := theme.Warning
	if s.state.XCredentialsStored {
		statusKey = i18n.SettingsXCookiesSaved
		statusColour = theme.Success
	}

	// One real request, so the two things that make a sync fail -- a bad cookie and
	// an unreachable network -- are told apart here rather than after a full run.
	// Until one has run the row says what the test does, as every preference row
	// explains itself; the result then takes the line.
	testSub, testInk := s.t().T(i18n.SettingsXTestHint), theme.TextMuted
	if s.state.XTestRunning {
		testSub, testInk = s.t().T(i18n.SettingsXTestRunning), rail.TextTertiary
	} else if s.state.XTestTested && s.state.XTestResult != "" {
		testSub = s.state.XTestResult
		testInk = theme.Warning
		if s.state.XTestOK {
			testInk = theme.Success
		}
		if s.state.XTestRoute != "" {
			testSub += " · " + s.state.XTestRoute
		}
	}

	pasteHint := s.t().T(i18n.SettingsXCredentialPasteHint)
	drawn += s.settingsCard(c, theme, rail,
		settingsItem{
			keys: []i18n.Key{i18n.SettingsXCookiesTitle, i18n.SettingsXCookiesHint},
			name: s.t().T(i18n.SettingsXCookiesTitle),
			sub:  s.t().T(i18n.SettingsXCookiesHint),
			control: func() {
				settingsStatus(c, theme, rail, s.t().T(statusKey), statusColour)
			},
		},
		settingsItem{
			keys: []i18n.Key{i18n.SettingsXBrowserRow, i18n.SettingsXBrowserRead, i18n.SettingsXBrowserHint},
			name: s.t().T(i18n.SettingsXBrowserRow),
			sub:  s.browserReadSub(),
			control: func() {
				settingsAction(c, theme, rail, s.t().T(i18n.SettingsXBrowserRead),
					false, false, func() { s.readXCookiesFromBrowser() })
			},
		},
		settingsItem{
			keys:     []i18n.Key{i18n.SettingsXTestRow, i18n.SettingsXTestConnection, i18n.SettingsXTestHint},
			name:     s.t().T(i18n.SettingsXTestRow),
			sub:      testSub,
			subColor: testInk,
			control: func() {
				settingsActionBusy(c, theme, rail, s.t().T(i18n.SettingsXTestConnection),
					s.state.XTestRunning, func() { s.TestXConnection(context.Background()) })
			},
		},
		settingsItem{
			keys: []i18n.Key{i18n.SettingsXCredentialPaste, i18n.SettingsXCredentialPasteHint, i18n.ActionSaveCookies},
			name: s.t().T(i18n.SettingsXCredentialPaste),
			sub:  pasteHint,
			control: func() {
				settingsAction(c, theme, rail, s.t().T(i18n.ActionSaveCookies),
					false, false, func() { s.saveXCookies() })
			},
			body: func() {
				// One field, not two: the reader copies one thing -- the request's Cookie
				// header, a cURL command, an exported cookie list -- and the parser takes the
				// pair out of it.
				ui.TextArea(c, &s.state.XCredentialPaste).FillWidth().Lines(1, 3).
					Radius(SettingsFieldRadius).
					Placeholder(s.t().T(i18n.SettingsXCredentialPastePlaceholder))
			},
		},
	)
	return drawn
}

// settingsDataSafetySection is 数据安全: where the backups go, and the lists and views this build
// keeps management controls for.
func (s *Shell) settingsDataSafetySection(c *ui.Context, theme *ui.Theme, rail RailTokens) int {
	drawn := 0
	if s.settingsShows(i18n.SettingsData, i18n.SettingsDataSafetyHint) {
		settingsSectionLabel(c, theme, rail, s.t().T(i18n.SettingsData))
		settingsNote(c, theme, rail, s.t().T(i18n.SettingsDataSafetyHint))
	}

	// The four file commands run off the window's thread, so they are disabled while
	// one is in flight. Without that the row sat unchanged for seconds after a click
	// and read as a dead button.
	busy := s.state.BundleBusy
	items := []settingsItem{
		{
			keys: []i18n.Key{i18n.SettingsBackupRow, i18n.ExportData, i18n.SettingsBackupHint},
			name: s.t().T(i18n.SettingsBackupRow),
			sub:  s.t().T(i18n.SettingsBackupHint),
			control: func() {
				settingsAction(c, theme, rail, s.t().T(i18n.ExportData), busy, false,
					func() { s.ExportBundle(context.Background()) })
			},
		},
		{
			keys: []i18n.Key{i18n.SettingsLegacyBackupRow, i18n.BundleExportLegacy, i18n.SettingsLegacyBackupHint},
			name: s.t().T(i18n.SettingsLegacyBackupRow),
			sub:  s.t().T(i18n.SettingsLegacyBackupHint),
			control: func() {
				settingsAction(c, theme, rail, s.t().T(i18n.BundleExportLegacy), busy, false,
					func() { s.ExportLegacyBackup(context.Background()) })
			},
		},
		{
			keys: []i18n.Key{i18n.SettingsRestoreRow, i18n.RestoreBackup, i18n.MenuImport, i18n.SettingsRestoreHint},
			name: s.t().T(i18n.SettingsRestoreRow),
			sub:  s.t().T(i18n.SettingsRestoreHint),
			control: func() {
				settingsAction(c, theme, rail, s.t().T(i18n.RestoreBackup), busy, false, func() {
					if err := s.AskBundleImport(context.Background(), true); err != nil {
						s.reportErr(err)
					}
				})
			},
		},
		{
			keys: []i18n.Key{i18n.SettingsLocalRow, i18n.ResetLocalData, i18n.SettingsLocalHint},
			name: s.t().T(i18n.SettingsLocalRow),
			sub:  s.t().T(i18n.SettingsLocalHint),
			control: func() {
				settingsAction(c, theme, rail, s.t().T(i18n.ResetLocalData), busy, true,
					func() { s.AskResetLocalData() })
			},
		},
	}
	if s.state.BlockedTagKeyCount > 0 {
		label := fmt.Sprintf("%s (%d)", s.t().T(i18n.SettingsClearBlockedTags),
			s.state.BlockedTagKeyCount)
		items = append(items, settingsItem{
			keys: []i18n.Key{i18n.SettingsBlockedTags, i18n.SettingsBlockedTagsHint,
				i18n.SettingsClearBlockedTags, i18n.SettingsClearBlockedTagsDone},
			name: s.t().T(i18n.SettingsBlockedTags),
			// Every other row in this card says what it does; this one used to carry its name
			// alone, which is the one word that does not explain why a tag name would be here.
			sub: s.t().T(i18n.SettingsBlockedTagsHint),
			control: func() {
				settingsAction(c, theme, rail, label, false, true, func() {
					if s.tagging != nil {
						_ = s.tagging.ClearBlockedTagKeys(context.Background())
					}
					s.state.BlockedTagKeyCount = 0
					// The row and its button vanish at zero, so without this line the action
					// would leave no trace at all.
					s.state.LastBundleReport = s.t().T(i18n.SettingsClearBlockedTagsDone)
					s.state.LastBundleOK = true
					if s.window != nil {
						s.window.Invalidate()
					}
				})
			},
		})
	}
	drawn += s.settingsCard(c, theme, rail, items...)

	if s.state.LastBundleReport != "" && !s.settingsFiltering() {
		// A finished action says so in the success colour; a progress note ("Exporting…")
		// and a refusal ("too many rows") stay muted, where they read as an aside rather
		// than as a result.
		if s.state.LastBundleOK && !s.state.BundleBusy {
			ui.Text(c, s.state.LastBundleReport).FontSize(FontSmall).
				TextColor(theme.Success).Padding(0, 4)
		} else {
			settingsNote(c, theme, rail, s.state.LastBundleReport)
		}
	}

	return drawn
}

// browserReadSub is the line under 浏览器登录: what the action does until it has
// run, and its result afterwards.
func (s *Shell) browserReadSub() string {
	if s.state.XBrowserReadNote != "" {
		return s.state.XBrowserReadNote
	}
	return s.t().T(i18n.SettingsXBrowserHint)
}

// saveXCookies stores the pair read out of the one paste field.
//
// The values come from a browser read (internal/credentials/browser.go) or from this paste,
// and either way they are kept in the Keychain.
func (s *Shell) saveXCookies() {
	if s.credentials == nil {
		return
	}
	parsed, err := xsource.ParseCredentialPaste(s.state.XCredentialPaste)
	if err != nil {
		s.ShowStatus(ToastError, s.pasteProblem(err))
		return
	}
	if err := s.credentials.SaveXCredentials(context.Background(), parsed); err != nil {
		s.reportErr(err)
		return
	}
	// The field is cleared rather than left holding the secret, which is the same rule the
	// provider key field follows.
	s.state.XCredentialPaste = ""
	s.state.XCredentialsStored = true
	s.state.XBrowserReadNote = ""
	s.state.LastBundleReport = s.t().T(i18n.SettingsXCookiesSaved)
	// The status line says the save worked, which also clears whatever the previous
	// attempt left there: a failed paste followed by a good one must not keep saying
	// "that paste has no ct0".
	s.ShowToast(s.t().T(i18n.SettingsXCookiesSaved))
	if s.window != nil {
		s.window.Invalidate()
	}
}

// RefreshXCookieStatus reads whether the Keychain holds both cookies, without reading them back.
func (s *Shell) RefreshXCookieStatus(ctx context.Context) {
	if s.credentials == nil {
		return
	}
	_, err := s.credentials.XCredentials(ctx)
	s.state.XCredentialsStored = err == nil
}
