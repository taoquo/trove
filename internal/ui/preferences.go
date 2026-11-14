package ui

import (
	"context"
	"strings"
	"time"

	engine "github.com/taoquo/trove/internal/autotag"
	"github.com/taoquo/trove/internal/i18n"
	"github.com/taoquo/trove/internal/llm"
)

// T renders a message in the current locale.
//
// The composition root needs it to label the menu bar, which is built outside this
// package.
func (s *Shell) T(key i18n.Key) string { return s.t().T(key) }

// OnLocaleChanged registers a callback for when the language changes.
//
// The menu bar is built once by the composition root, so without this its labels
// would keep the language the app started in.
func (s *Shell) OnLocaleChanged(fn func()) { s.onLocaleChanged = fn }

// BatchSelection returns the ids a bulk action would run on.
//
// It returns a copy, so a caller cannot mutate the selection it was handed.

// SetLocale changes the language and rebuilds the translator.
//
// It is the only way the language changes: a view body may be rebuilt several
// times per event, so resolving the locale there would be a side effect.
func (s *Shell) SetLocale(preferred, system string) {
	s.state.Locale = preferred
	s.state.SystemLocale = system
	s.translator = i18n.New(preferred, system)
	s.clearTransientMessages()
	// The auto-tagging page holds translated select values, not just keys, so a language
	// change has to re-derive them (the vocabulary mode's name is held as text).
	s.state.VocabularyModeLabel = s.vocabularyModeLabel(engine.TagVocabularyMode(s.state.VocabularyMode))
	// The settings selectors hold translated strings, not keys, so they are re-derived here
	// for every caller; without this a language switch leaves the theme row reading "Light"
	// on a Chinese page. The batch pickers' placeholders are the same class of value.
	s.syncPreferenceLabels()
	s.syncBatchLabels()
	s.persistPreferences(context.Background())
	if s.onLocaleChanged != nil {
		s.onLocaleChanged()
	}
}

// ApplyCaptureLocale sets the language for a screenshot without persisting it.
//
// SetLocale writes the preference, and a screenshot has no business rewriting what the user sees on
// the next launch -- the same reason ApplyCaptureState assigns Appearance directly rather than
// calling SetAppearance, which would write dark mode into the real preferences. This
// is the locale equivalent of that rule.
//
// It exists because the reference picture is a Chinese interface and this build's captures had all
// been English: comparing the two said "the strings differ" when what needed checking was whether
// the Chinese strings fit the same boxes.
func (s *Shell) ApplyCaptureLocale(locale string) {
	s.state.Locale = locale
	s.state.SystemLocale = locale
	s.translator = i18n.New(locale, locale)
	s.clearTransientMessages()
	// Capture states run before the language is pinned, so a translated value loaded there
	// still holds the previous language. Re-derive only that value: a full settings reload here
	// would overwrite the result fixture the state just installed.
	s.state.VocabularyModeLabel = s.vocabularyModeLabel(engine.TagVocabularyMode(s.state.VocabularyMode))
	// Same reason as SetLocale: the two selectors' labels are translated values. The capture
	// path usually pins the appearance afterwards, which re-derives them too, but a capture
	// that only names a language has the same stale-label problem.
	s.syncPreferenceLabels()
	s.syncBatchLabels()
	if s.onLocaleChanged != nil {
		s.onLocaleChanged()
	}
}

// ApplyCaptureAppearance pins the theme for a screenshot without persisting it.
//
// ApplyCaptureState resets the appearance to System so one picture does not inherit the last
// one's choice, and System resolves to whatever the machine is running. A capture is compared
// against a reference render taken in the light theme, so the appearance has to be pinned too
// -- otherwise the same command produces a dark picture on a dark Mac and every colour
// comparison fails for a reason that has nothing to do with the build.
func (s *Shell) ApplyCaptureAppearance(appearance string) {
	switch Appearance(appearance) {
	case AppearanceLight, AppearanceDark, AppearanceSystem:
		s.state.Appearance = Appearance(appearance)
	default:
		return
	}
	s.syncPreferenceLabels()
}

// clearTransientMessages drops the one-shot reports written in the old language.
//
// State fields that hold a translated string are two classes and want two answers:
//
//   - The sync status is *state*. It is shown for as long as it is true, so it stores a
//     domain.SyncStatus and is translated where it is drawn.
//   - These eight are *reports of something that just happened*: "imported 3 posts", "connection
//     failed", "that is not a post link". They carry numbers, so storing keys would mean storing
//     arguments too, and they are true only for a moment. Clearing them is the honest answer --
//     a stale sentence in the language the user just left is worse than no sentence.
//
// The alternative, translating at draw time, cannot work for these: by then the numbers they were
// built from are gone.
func (s *Shell) clearTransientMessages() {
	s.state.SyncProgress = ""
	s.state.AutoTagStatus = ""
	s.state.LastBundleReport = ""
	s.state.AddProblem = ""
	s.state.ConnectionResult = ""
	s.state.TagActionMsg = ""
	s.state.TagCleanupMessage = ""
	s.state.ReviewApplied = ""
}

// SetAppearance changes the theme preference.
func (s *Shell) SetAppearance(appearance Appearance) {
	if !ValidAppearance(appearance) {
		return
	}
	s.state.Appearance = appearance
	s.persistPreferences(context.Background())
}

// reloadAfterReset re-reads what a reset has just deleted.
//
// ResetLocalData empties the settings rows and the Keychain accounts. The window kept
// every one of them in memory: the page went on saying the cookies were saved and the
// sync section went on offering its stored preferences, for values that no longer
// existed. This reloads in place: the preferences the startup path applies, the settings
// page's own fields, and the two presence flags. The value of a secret is still never
// read into the window; only whether one exists.
func (s *Shell) reloadAfterReset(ctx context.Context) {
	if err := s.ApplyPreferences(ctx); err != nil {
		s.reportErr(err)
	}
	s.syncPreferenceLabels()
	s.loadAutoTagSettings(ctx)
	s.loadSyncPreferences(ctx)
	s.RefreshXCookieStatus(ctx)
}

// ApplyPreferences loads the stored preferences and applies them.
//
// It runs at startup, before the first frame, so the window never appears in the
// wrong theme and then flips.
func (s *Shell) ApplyPreferences(ctx context.Context) error {
	if s.settings == nil {
		return nil
	}

	preferences, err := s.settings.Load(ctx)
	if err != nil {
		return err
	}

	if ValidAppearance(preferences.Appearance) {
		// Clamped, not taken as stored. NormalizeSidebarWidth's own comment says why: a
		// stored value can be outside the stylesheet's bounds, and a width the user cannot
		// drag back out of range must not be applied. NormalizeInspectorWidth was wired and
		// this was not, so the pair was half connected.
		s.state.LeftSidebarWidth = NormalizeSidebarWidth(preferences.Layout.LeftSidebarWidth)
		s.state.RightSidebarWidth = NormalizeSidebarWidth(preferences.Layout.RightSidebarWidth)
		s.state.SidebarCollapsed = preferences.Layout.SidebarCollapsed
		// A hidden column stored before the pick existed was hidden by the reader's press, so
		// it is a pick: honouring it as one keeps a reader who put the sidebar away from
		// finding it back on a wide window after the update.
		s.state.SidebarChosen = preferences.Layout.SidebarChosen || preferences.Layout.SidebarCollapsed
		// The loaded layout is what is stored, so the first rebuild is not mistaken for a
		// drag and written back.
		s.persistedLeft = s.state.LeftSidebarWidth
		s.persistedRight = s.state.RightSidebarWidth
		s.persistedCollapsed = s.state.SidebarCollapsed
		s.persistedChosen = s.state.SidebarChosen
		s.state.Appearance = preferences.Appearance
	}
	s.state.Locale = preferences.Locale
	// The translator is rebuilt from the loaded values; systemLocale is untouched
	// because only the caller knows what the OS reports.
	s.translator = i18n.New(preferences.Locale, s.state.SystemLocale)

	// The sync block's fields have to be in state before the first frame: the
	// launch-time sync reads them, and until now they were loaded only when the
	// settings sheet was opened. The panel would have shown defaults while a
	// launch sync used them.
	s.loadSyncPreferences(ctx)

	return nil
}

// persistPreferences writes the current preferences back.
//
// A failure is recorded rather than swallowed: silently losing a preference means
// the user changes it again on every launch and never learns why.
func (s *Shell) persistPreferences(ctx context.Context) {
	if s.settings == nil {
		return
	}

	preferences, err := s.settings.Load(ctx)
	if err != nil {
		s.reportErr(err)
		return
	}
	preferences.Appearance = s.state.Appearance
	preferences.Locale = s.state.Locale

	if err := s.settings.Save(ctx, preferences); err != nil {
		s.reportErr(err)
	}
}

// t returns the translator.
//
// It has no side effects: a view body calls it on every rebuild, so it must not
// build anything.
func (s *Shell) t() *i18n.Translator {
	if s.translator == nil {
		s.translator = i18n.New("", "")
	}
	return s.translator
}

// AskDeleteBookmarks arms the confirmation for the given bookmarks.
//
// The ids are stored with the confirmation, so the deletion runs on exactly the
// set the user was shown even if the list reloads in between.

// appearanceLabels renders the three theme choices.
func (s *Shell) appearanceLabels() []string {
	return []string{
		s.t().T(i18n.AppearanceSystem),
		s.t().T(i18n.AppearanceLight),
		s.t().T(i18n.AppearanceDark),
	}
}

// localeLabels renders the language choices, with an entry for following the OS.
func (s *Shell) localeLabels() []string {
	return []string{s.t().T(i18n.AppearanceSystem), "中文", "English"}
}

// appearanceFor maps a selector label back to a preference.
//
// It compares against the rendered labels rather than parsing, so the mapping
// cannot drift from what the user was shown.
func (s *Shell) appearanceFor(label string) Appearance {
	labels := s.appearanceLabels()
	values := []Appearance{AppearanceSystem, AppearanceLight, AppearanceDark}
	for i, candidate := range labels {
		if label == candidate {
			return values[i]
		}
	}
	return AppearanceSystem
}

// localeFor maps a language label to a locale.
func (s *Shell) localeFor(label string) string {
	switch label {
	case "中文":
		return "zh-CN"
	case "English":
		return "en"
	default:
		// The first entry means "follow the system".
		return ""
	}
}

// syncPreferenceLabels refreshes the bound selector values from the state.
func (s *Shell) syncPreferenceLabels() {
	appearances := s.appearanceLabels()
	switch s.state.Appearance {
	case AppearanceLight:
		s.state.AppearanceLabel = appearances[1]
	case AppearanceDark:
		s.state.AppearanceLabel = appearances[2]
	default:
		s.state.AppearanceLabel = appearances[0]
	}

	switch s.state.Locale {
	case "zh-CN":
		s.state.LocaleLabel = "中文"
	case "en":
		s.state.LocaleLabel = "English"
	default:
		s.state.LocaleLabel = s.localeLabels()[0]
	}
}

// loadAutoTagSettings reads the auto-tagging settings into the panel's fields.
func (s *Shell) loadAutoTagSettings(ctx context.Context) {
	if s.tagging == nil {
		return
	}
	settings, err := s.tagging.LoadSettings(ctx)
	if err != nil {
		s.reportErr(err)
		return
	}

	s.applyAutoTagSettings(settings)

	// The last run's outcome, so a user who opens the panel after a run can see what
	// happened without reading a log.
	if summary, found, err := s.tagging.LastSummary(ctx); err == nil && found {
		s.state.AutoTagHeadline, s.state.AutoTagDetail =
			s.SummarizeRun(summary.Outcome, summary)
		s.state.AutoTagFinishedAt = summary.FinishedAt
	} else {
		s.state.AutoTagHeadline, s.state.AutoTagDetail = "", ""
		s.state.AutoTagFinishedAt = time.Time{}
	}
	// The key field is never filled from storage: the field is for a new key, and the
	// stored one is not read back into the window.
	s.state.APIKeyInput = ""

	// Whether a key exists, however, is the panel's business: it decides between
	// "stored" and "no key stored", and a run that works while the page says no key
	// is stored is the panel calling its own reader a liar. Presence only; the value
	// is never read into the window.
	s.refreshAPIKeyState(ctx)
}

// applyAutoTagSettings copies the stored settings into the panel's fields -- every field
// saveAutoTagSettings writes, and no more. A failed write reads them back through this, so
// the panel cannot keep a switch that storage never took.
func (s *Shell) applyAutoTagSettings(settings engine.Settings) {
	s.state.AutoTagEnabled = settings.Enabled
	s.state.AutoContinue = settings.AutoContinue
	s.state.BatchSizeInput = float64(settings.LLM.BatchSize)
	s.state.ConcurrencyInput = float64(settings.LLM.Concurrency)
	s.state.UseHashtags = settings.Heuristics.UseHashtags
	s.state.BlockedTagKeyCount = len(settings.BlockedTagKeys)
	// The chain's budget. A run that was interrupted mid-chain keeps its count, which is
	// what stops it restarting forever -- so it is stored, not just held in memory.
	s.state.AutoTagContinueCount = settings.AutoTagContinueCount
	s.state.UseAuthors = settings.Heuristics.UseAuthors
	s.state.RunAfterSync = settings.RunAfterSync
	s.state.LLMEnabled = settings.LLM.Enabled
	s.state.ProviderLabel = s.providerLabel(settings.LLM.ProviderID)
	s.state.BaseURLInput = settings.LLM.BaseURL
	s.state.DeploymentInput = settings.LLM.Deployment
	s.state.ModelInput = settings.LLM.Model
	s.state.MaxPerRunInput = float64(settings.LLM.MaxPerRun)
	s.state.VocabularyMode = settings.LLM.Mode
	s.state.VocabularyModeLabel = s.vocabularyModeLabel(settings.LLM.Mode)
	s.state.KeywordRules = append([]engine.KeywordTagRule{}, settings.Heuristics.KeywordRules...)
}

// loadLastAutoTagSummary re-reads the run record the panel shows.
//
// It is not loadAutoTagSettings: that also copies the stored continuation count back over the
// chain's in-memory counter, which reset the budget after every pass and made the chain restart
// until the test gave up (202 runs for a budget of 5).
func (s *Shell) loadLastAutoTagSummary(ctx context.Context) {
	if s.tagging == nil {
		return
	}
	if summary, found, err := s.tagging.LastSummary(ctx); err == nil && found {
		s.state.AutoTagHeadline, s.state.AutoTagDetail =
			s.SummarizeRun(summary.Outcome, summary)
		s.state.AutoTagFinishedAt = summary.FinishedAt
		return
	}
	s.state.AutoTagHeadline, s.state.AutoTagDetail = "", ""
	s.state.AutoTagFinishedAt = time.Time{}
}

// saveAutoTagSettings writes the panel's fields back.
//
// A switch moves before the write; when the write fails the fields are read
// back from storage, so the panel rolls the switch back instead of claiming a setting that
// is not stored.
func (s *Shell) saveAutoTagSettings(ctx context.Context) {
	if s.tagging == nil {
		return
	}
	settings, err := s.tagging.LoadSettings(ctx)
	if err != nil {
		s.reportErr(err)
		return
	}

	settings.Enabled = s.state.AutoTagEnabled
	settings.AutoContinue = s.state.AutoContinue
	settings.LLM.BatchSize = int(s.state.BatchSizeInput)
	settings.LLM.Concurrency = int(s.state.ConcurrencyInput)
	settings.RunAfterSync = s.state.RunAfterSync
	settings.Heuristics.UseHashtags = s.state.UseHashtags
	settings.AutoTagContinueCount = s.state.AutoTagContinueCount
	settings.Heuristics.UseAuthors = s.state.UseAuthors
	// Normalized on the way in, so a rule with a blank half is dropped rather than
	// stored as a row that can never match.
	settings.Heuristics.KeywordRules = engine.NormalizeKeywordRules(s.state.KeywordRules)
	settings.LLM.Enabled = s.state.LLMEnabled

	// A provider change resets what belonged to the old one.
	//
	// The endpoint field survives a pick otherwise, and the key is sent to the address in
	// that field: a reader who pointed "Custom" at a gateway and then chose OpenAI would
	// post the OpenAI key to the old host. The reset starts needsBaseUrl blank, every other
	// provider at its documented default, and drops the draft key.
	nextProviderID := s.providerIDFor(s.state.ProviderLabel)
	providerChanged := settings.LLM.ProviderID != nextProviderID
	if providerChanged {
		provider := llm.ProviderByID(nextProviderID)
		nextBaseURL := provider.DefaultBaseURL
		if provider.NeedsBaseURL {
			nextBaseURL = ""
		}
		s.state.BaseURLInput = nextBaseURL
		s.state.APIKeyInput = ""
		if !provider.NeedsDeployment {
			s.state.DeploymentInput = ""
		}
	}
	settings.LLM.ProviderID = nextProviderID
	settings.LLM.BaseURL = strings.TrimSpace(s.state.BaseURLInput)
	settings.LLM.Deployment = strings.TrimSpace(s.state.DeploymentInput)
	settings.LLM.Model = strings.TrimSpace(s.state.ModelInput)
	settings.LLM.MaxPerRun = int(s.state.MaxPerRunInput)
	settings.LLM.Mode = s.vocabularyModeFor(s.state.VocabularyModeLabel)
	s.state.VocabularyMode = settings.LLM.Mode

	// The engine clamps these itself, but a value the UI accepted and the engine
	// silently changed would look like the setting did not stick.
	if err := s.tagging.SaveSettings(ctx, settings); err != nil {
		s.reportErr(err)
		// The switch had already moved. Read the stored settings back so the panel shows
		// what is stored rather than the change that did not stick -- an optimistic
		// switch that never rolls back is a switch that lies.
		if stored, loadErr := s.tagging.LoadSettings(ctx); loadErr == nil {
			s.applyAutoTagSettings(stored)
		}
		return
	}

	// A different provider has a different key, and the connection result belongs to
	// the provider that produced it. Presence is read here rather than inside the
	// generic save path because it costs a Keychain round trip: doing it on every
	// keystroke of the endpoint field would be a subprocess per character.
	if s.credentials != nil && s.state.APIKeyProviderID != settings.LLM.ProviderID {
		s.state.ConnectionTested = false
		s.refreshAPIKeyState(ctx)
	}
}

// settingsPanel draws the settings.
//
// It is a panel rather than a second window: the settings are small, and a modal
// keeps the workspace visible behind it so a user can see the effect of a change.
