package ui

import (
	"context"
	"strings"

	engine "github.com/taoquo/trove/internal/autotag"
	"github.com/taoquo/trove/internal/i18n"
	"github.com/taoquo/trove/internal/llm"
)

// loadSyncPreferences reads the sync preferences into the panel's fields.
func (s *Shell) loadSyncPreferences(ctx context.Context) {
	if s.settings == nil {
		return
	}
	preferences, err := s.settings.Load(ctx)
	if err != nil {
		s.reportErr(err)
		return
	}
	s.state.SyncOnLaunch = preferences.Sync.SyncOnLaunch
	s.state.SaveRawPayloads = preferences.Sync.SaveRawPayloads
	s.state.StopBufferPages = float64(preferences.Sync.StopBufferPages)
	s.state.SyncPageLimit = float64(preferences.Sync.PageLimit)
}

// saveSyncPreferences writes the sync preferences back.
func (s *Shell) saveSyncPreferences(ctx context.Context) {
	if s.settings == nil {
		return
	}
	preferences, err := s.settings.Load(ctx)
	if err != nil {
		s.reportErr(err)
		return
	}
	preferences.Sync.SyncOnLaunch = s.state.SyncOnLaunch
	preferences.Sync.SaveRawPayloads = s.state.SaveRawPayloads
	preferences.Sync.StopBufferPages = int(s.state.StopBufferPages)
	preferences.Sync.PageLimit = int(s.state.SyncPageLimit)
	if err := s.settings.Save(ctx, preferences); err != nil {
		s.reportErr(err)
	}
}

// TestConnection probes the configured provider and records the outcome.
//
// The outcome is kept as text plus a flag rather than as a status object: the panel
// only needs to show it, and a stored result would go stale the moment the endpoint
// or the key changed.
func (s *Shell) TestConnection(ctx context.Context) {
	if s.tagging == nil {
		s.state.ConnectionTested = true
		s.state.ConnectionOK = false
		s.state.ConnectionResult = s.t().T(i18n.ConnectionNoTester)
		return
	}
	if s.state.ConnectionRunning {
		return
	}

	// The probe is a real request to the provider, which takes as long as the network
	// takes -- ten seconds is ordinary and a timeout is longer. Run in the handler it
	// froze the window for that whole time, which is the same defect the import and
	// export buttons had.
	s.state.ConnectionRunning = true
	s.state.ConnectionTested = true
	s.state.ConnectionOK = false
	s.state.ConnectionResult = s.t().T(i18n.ConnectionTesting)
	s.invalidate()

	s.runInBackground(func() {
		result := s.tagging.TestConnection(ctx)

		s.publish(func() {
			s.state.ConnectionRunning = false
			s.state.ConnectionTested = true
			s.state.ConnectionOK = result.OK

			switch {
			case result.OK:
				s.state.ConnectionResult = s.t().Tf(i18n.ConnectionOK, result.Model)
			case result.Message == engine.MissingCredentialMessage:
				// Nothing was sent, and the fix is to enter a key rather than to
				// replace one: the missing-key sentence describes this case, and the
				// "invalid or expired" sentence describes a different mistake.
				s.state.ConnectionResult = s.t().T(i18n.SettingsLlmTestMissingKey)
			default:
				// A classified failure gets the sentence a person can act on; an
				// unclassified one keeps the provider's own words, because guessing at
				// those would mislead.
				readable, retry := s.localizeProviderError(result.ErrorKind, 0)
				switch {
				case readable != "" && retry != "":
					s.state.ConnectionResult = readable + " " + retry
				case readable != "":
					s.state.ConnectionResult = readable
				case result.Message != "":
					s.state.ConnectionResult = result.Message
				default:
					s.state.ConnectionResult = s.t().T(i18n.ConnectionFailed)
				}
			}
			s.invalidate()
		})
	})
}

// statusForResult maps a probe outcome onto a status badge.
func statusForResult(ok bool) string {
	if ok {
		return StatusSuccess
	}
	return StatusError
}

// providerLabels are the provider names shown in the select.
func (s *Shell) providerLabels() []string {
	labels := make([]string, 0, len(llm.Providers))
	for _, provider := range llm.Providers {
		labels = append(labels, provider.Name)
	}
	return labels
}

// providerLabel maps a stored provider id to its display name, falling back to the
// first provider so an id from a newer build shows something rather than a blank.
func (s *Shell) providerLabel(providerID string) string {
	return llm.ProviderByID(providerID).Name
}

// providerIDFor maps a display name back to the provider id.
func (s *Shell) providerIDFor(label string) string {
	for _, provider := range llm.Providers {
		if provider.Name == label {
			return provider.ID
		}
	}
	return llm.Providers[0].ID
}

// vocabularyModeLabels are the vocabulary modes in the order the engine accepts. The
// stored value is the engine's id; showing it raw put the word "guarded" in a settings
// field.

// vocabularyModeLabels are the vocabulary modes in the order the engine accepts.
func (s *Shell) vocabularyModeLabels() []string {
	return []string{
		s.t().T(i18n.SettingsVocabularyOpen),
		s.t().T(i18n.SettingsVocabularyGuarded),
		s.t().T(i18n.SettingsVocabularyClosed),
	}
}

// vocabularyModeLabel is the translated name of a stored mode.
func (s *Shell) vocabularyModeLabel(mode engine.TagVocabularyMode) string {
	switch mode {
	case engine.VocabularyClosed:
		return s.t().T(i18n.SettingsVocabularyClosed)
	case engine.VocabularyOpen:
		return s.t().T(i18n.SettingsVocabularyOpen)
	default:
		return s.t().T(i18n.SettingsVocabularyGuarded)
	}
}

// vocabularyModeFor maps the shown label back to the engine's id. An unknown label is the
// guarded mode, which is the engine's own default.
func (s *Shell) vocabularyModeFor(label string) engine.TagVocabularyMode {
	switch label {
	case s.t().T(i18n.SettingsVocabularyClosed):
		return engine.VocabularyClosed
	case s.t().T(i18n.SettingsVocabularyOpen):
		return engine.VocabularyOpen
	default:
		return engine.VocabularyGuarded
	}
}

// saveAPIKey stores the typed key and clears the field.
//
// The field is cleared so the secret does not sit in the window's state where a later
// frame could render it back.
func (s *Shell) saveAPIKey() {
	key := strings.TrimSpace(s.state.APIKeyInput)
	if key == "" {
		return
	}
	if err := s.credentials.SaveLLMKey(context.Background(), s.providerIDFor(s.state.ProviderLabel), key); err != nil {
		s.reportErr(err)
		return
	}
	s.state.APIKeyInput = ""
	// The old result described the old key.
	s.state.ConnectionTested = false
	s.refreshAPIKeyState(context.Background())
}

// clearAPIKey removes the stored key for the selected provider.
func (s *Shell) clearAPIKey() {
	if err := s.credentials.DeleteLLMKey(context.Background(), s.providerIDFor(s.state.ProviderLabel)); err != nil {
		s.reportErr(err)
		return
	}
	s.state.APIKeyInput = ""
	// The old result described the old key.
	s.state.ConnectionTested = false
	s.refreshAPIKeyState(context.Background())
}

// refreshAPIKeyState records only whether a key exists.
//
// Presence, never the value: the panel needs to say "stored" and has no reason to
// hold the secret to do it.
func (s *Shell) refreshAPIKeyState(ctx context.Context) {
	if s.credentials == nil {
		return
	}
	providerID := s.providerIDFor(s.state.ProviderLabel)
	credential, err := s.credentials.LLMCredential(ctx, providerID, true)
	if err != nil {
		s.reportErr(err)
		return
	}
	s.state.APIKeyStored = !credential.IsEmpty()
	s.state.APIKeyProviderID = providerID
}

// addKeywordRule appends the draft rule and saves.
//
// An incomplete draft is refused rather than stored: NormalizeKeywordRules would
// drop it on save anyway, so accepting it would make the row vanish for no visible
// reason.
func (s *Shell) addKeywordRule() {
	keyword := strings.TrimSpace(s.state.DraftKeyword)
	tagName := strings.TrimSpace(s.state.DraftRuleTag)
	if keyword == "" || tagName == "" {
		return
	}

	s.state.KeywordRules = append(s.state.KeywordRules, engine.KeywordTagRule{
		Keyword: keyword, TagName: tagName,
	})
	s.state.DraftKeyword = ""
	s.state.DraftRuleTag = ""
	s.saveAutoTagSettings(context.Background())
}

// removeKeywordRule drops one rule and saves.
func (s *Shell) removeKeywordRule(index int) {
	if index < 0 || index >= len(s.state.KeywordRules) {
		return
	}
	s.state.KeywordRules = append(
		append([]engine.KeywordTagRule{}, s.state.KeywordRules[:index]...),
		s.state.KeywordRules[index+1:]...,
	)
	s.saveAutoTagSettings(context.Background())
}

// HealthyTagCeiling is the count past which the library is reported as too large.
//
// It is engine.HealthyTagCountMax, which the auto-tagging budget also uses.
func HealthyTagCeiling() int { return engine.HealthyTagCountMax }

// displayFace is the face headings are drawn in.
//
// magpie sets every surface -- headings included -- in the system UI face:
//
//	--font: -apple-system, BlinkMacSystemFont, "SF Pro Text", "Segoe UI", system-ui, sans-serif
//	(app.css:41)
//
// so this returns "" and the toolkit uses its own system face. A serif heading would depart
// from magpie's look, which is what this shell follows. Both locales are the same face here:
// magpie's Chinese is PingFang, which is what the system stack resolves to.
//
// The empty string is deliberate, not a fallback: it is the toolkit's "system default".
func (s *Shell) displayFace() string { return "" }

// FocusSearch asks the next frame to put the cursor in the search field.
