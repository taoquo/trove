package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	engine "github.com/taoquo/trove/apps/macos/internal/autotag"
	"github.com/taoquo/trove/apps/macos/internal/domain"
	"github.com/taoquo/trove/apps/macos/internal/i18n"
	"github.com/taoquo/trove/apps/macos/internal/llm"
)

// The auto-tagging page of tag management.
//
// Auto-tagging produces tags and the tag library is where they are read, renamed, merged and
// deleted; the controls lived in the settings sheet, so a run, its result and the table it
// writes were three screens apart (shell-contract.md S26). This page is the other half of
// TagMode: the table's page and this one are mutually exclusive, and it is drawn only when
// the reader chooses it.
//
// The page is drawn in the settings page's grammar (S46): cards of preference rows, a --muted
// uppercase label over each part, and one button (.text.action). The previous version was a
// stack of form groups -- labels above native checkboxes, number fields with their own minus
// and plus, and a card per heading -- which read as a form to fill in rather than as a set of
// switches and values.

// tagAutoTagView is the page: the run, then the parts that steer it.
func (s *Shell) tagAutoTagView(c *ui.Context, theme *ui.Theme, rail RailTokens) {
	ui.Scroll(c).FillWidth().Grow(1).Children(func() {
		ui.Column(c).FillWidth().GapY(SettingsCardGap).
			Padding(SettingsPagePadTop, 0, SettingsPagePadBottom, 0).Children(func() {
			s.autoTagRunCard(c, theme, rail)

			settingsSectionLabel(c, theme, rail, s.t().T(i18n.SettingsGroupAutoTagBasics))
			settingsNote(c, theme, rail, s.t().T(i18n.AutoTagBasicsHint))
			s.autoTagBasicsCard(c, theme, rail)

			settingsSectionLabel(c, theme, rail, s.t().T(i18n.AutoTagSources))
			s.autoTagSourcesCard(c, theme, rail)

			settingsSectionLabel(c, theme, rail, s.t().T(i18n.SettingsKeywordRules))
			settingsNote(c, theme, rail, s.t().T(i18n.SettingsAutoTagKeywordRulesHint))
			s.autoTagKeywordRules(c, theme, rail)

			settingsSectionLabel(c, theme, rail, s.t().T(i18n.AutoTagPacing))
			settingsNote(c, theme, rail, s.t().T(i18n.AutoTagPacingHint))
			s.autoTagPacingCard(c, theme, rail)

			settingsSectionLabel(c, theme, rail, s.t().T(i18n.SettingsGroupAutoTagLlm))
			settingsNote(c, theme, rail, s.t().T(i18n.AutoTagModelHint))
			s.autoTagModelCard(c, theme, rail)
		})
	})
}

// autoTagRunCard is the one thing that is happening now: the control, the live run and what
// the last one did.
//
// It also carries the two things that follow a run -- the suggestions waiting for review and
// the vocabulary review -- because that is the order a reader needs them in. Both were
// reachable before, from a menu and from the table's toolbar; neither was here, where the run
// had just finished.
func (s *Shell) autoTagRunCard(c *ui.Context, theme *ui.Theme, rail RailTokens) int {
	run := settingsItem{
		keys: []i18n.Key{i18n.SettingsSectionAutoTag, i18n.AutoTagRunHint},
		name: s.t().T(i18n.SettingsSectionAutoTag),
		sub:  s.t().T(i18n.AutoTagRunHint),
		control: func() {
			if s.state.AutoTagRunning {
				if live := s.liveRunState(); live != nil && live.CancelRequested {
					// The progress bar below already carries the disabled stopping
					// action. A second control here is one the user can press with
					// nothing left for it to do.
					return
				}
				settingsAction(c, theme, rail, s.t().T(i18n.ActionStopRun), false, false, func() {
					s.StopAutoTag()
				})
				return
			}
			textAction(c, theme, rail, textActionSpec{
				label: s.t().T(i18n.ActionRunNow), primary: true,
				onClick: func() { s.StartAutoTag(context.Background()) },
			})
		},
	}

	// The live run, if one is being described. The sidebar draws the same view while the
	// reader is on the results page; here it is the page's own status.
	live := s.liveRunState()
	var progress *RunProgressView
	if live != nil {
		if view, ok := s.RunProgressViewFor(live, time.Now()); ok {
			progress = &view
		}
	}

	items := []settingsItem{run}
	if progress != nil {
		view := *progress
		items = append(items, settingsItem{
			// A row with no name: the bar is the row's whole content.
			body: func() { s.runProgressBar(c, theme, view) },
		})
	}

	hasResult := s.state.AutoTagHeadline != "" || !s.state.AutoTagFinishedAt.IsZero()
	switch {
	case hasResult:
		sub := s.state.AutoTagHeadline
		if s.state.AutoTagDetail != "" {
			if sub != "" {
				sub += " · "
			}
			sub += s.state.AutoTagDetail
		}
		when := ""
		if !s.state.AutoTagFinishedAt.IsZero() {
			when = domain.FormatSavedAt(domain.FormatTimestamp(s.state.AutoTagFinishedAt),
				string(s.t().Locale()))
		}
		items = append(items, settingsItem{
			keys: []i18n.Key{i18n.AutoTagTitle},
			name: s.t().T(i18n.AutoTagTitle),
			sub:  sub,
			control: func() {
				settingsQuiet(c, theme, when)
			},
		})
	case progress == nil:
		// An empty card with only a button reads as broken; the page says it has not run.
		items = append(items, settingsItem{
			keys: []i18n.Key{i18n.AutoTagNoRunsYet},
			sub:  s.t().T(i18n.AutoTagNoRunsYet),
		})
	}

	if n := len(s.state.PendingSuggestions); n > 0 {
		items = append(items, settingsItem{
			keys: []i18n.Key{i18n.SuggestionsPendingTitle},
			name: s.t().T(i18n.SuggestionsPendingTitle),
			control: func() {
				settingsAction(c, theme, rail,
					fmt.Sprintf("%d %s", n, s.t().T(i18n.SuggestionsPendingTitle)),
					false, false, func() { s.OpenSuggestions(context.Background()) })
			},
		})
	}

	items = append(items, settingsItem{
		keys: []i18n.Key{i18n.ReviewTitle, i18n.ReviewDescription},
		name: s.t().T(i18n.ReviewTitle),
		sub:  s.t().T(i18n.ReviewDescription),
		control: func() {
			settingsAction(c, theme, rail, s.t().T(i18n.ReviewLabel), false, false, func() {
				s.OpenVocabularyReview()
			})
		},
	})
	return s.settingsCard(c, theme, rail, items...)
}

// autoTagBasicsCard is 基础规则: when a pass starts.
func (s *Shell) autoTagBasicsCard(c *ui.Context, theme *ui.Theme, rail RailTokens) int {
	save := func() { s.saveAutoTagSettings(context.Background()) }
	return s.settingsCard(c, theme, rail,
		settingsItem{
			keys: []i18n.Key{i18n.SettingsAutoTagEnabled, i18n.SettingsAutoTagEnabledHint},
			name: s.t().T(i18n.SettingsAutoTagEnabled),
			sub:  s.t().T(i18n.SettingsAutoTagEnabledHint),
			control: func() {
				settingsSwitch(c, theme, rail, s.t().T(i18n.SettingsOff), s.t().T(i18n.SettingsOn),
					s.t().T(i18n.SettingsAutoTagEnabled), &s.state.AutoTagEnabled, save)
			},
		},
		settingsItem{
			keys: []i18n.Key{i18n.SettingsRunAfterSync},
			name: s.t().T(i18n.SettingsRunAfterSync),
			control: func() {
				settingsSwitch(c, theme, rail, s.t().T(i18n.SettingsOff), s.t().T(i18n.SettingsOn),
					s.t().T(i18n.SettingsRunAfterSync), &s.state.RunAfterSync, save)
			},
		},
		settingsItem{
			keys: []i18n.Key{i18n.SettingsAutoContinue},
			name: s.t().T(i18n.SettingsAutoContinue),
			control: func() {
				settingsSwitch(c, theme, rail, s.t().T(i18n.SettingsOff), s.t().T(i18n.SettingsOn),
					s.t().T(i18n.SettingsAutoContinue), &s.state.AutoContinue, save)
			},
		},
	)
}

// autoTagSourcesCard is 标签来源: the signals a suggestion may come from.
func (s *Shell) autoTagSourcesCard(c *ui.Context, theme *ui.Theme, rail RailTokens) int {
	save := func() { s.saveAutoTagSettings(context.Background()) }
	return s.settingsCard(c, theme, rail,
		settingsItem{
			keys: []i18n.Key{i18n.SettingsUseHashtags},
			name: s.t().T(i18n.SettingsUseHashtags),
			control: func() {
				settingsSwitch(c, theme, rail, s.t().T(i18n.SettingsOff), s.t().T(i18n.SettingsOn),
					s.t().T(i18n.SettingsUseHashtags), &s.state.UseHashtags, save)
			},
		},
		settingsItem{
			keys: []i18n.Key{i18n.SettingsUseAuthors},
			name: s.t().T(i18n.SettingsUseAuthors),
			control: func() {
				settingsSwitch(c, theme, rail, s.t().T(i18n.SettingsOff), s.t().T(i18n.SettingsOn),
					s.t().T(i18n.SettingsUseAuthors), &s.state.UseAuthors, save)
			},
		},
	)
}

// autoTagKeywordRules is the rule list and its editor, as rows of one card: each rule is a
// keyword and the tag it suggests, with the way to remove it at the end of its row; the last
// row is the editor, so adding a rule sits where the rules are.
func (s *Shell) autoTagKeywordRules(c *ui.Context, theme *ui.Theme, rail RailTokens) int {
	items := make([]settingsItem, 0, len(s.state.KeywordRules)+2)
	if len(s.state.KeywordRules) == 0 {
		items = append(items, settingsItem{
			keys: []i18n.Key{i18n.SettingsKeywordRulesEmpty},
			sub:  s.t().T(i18n.SettingsKeywordRulesEmpty),
		})
	}
	for index, rule := range s.state.KeywordRules {
		index, rule := index, rule
		items = append(items, settingsItem{
			keys: []i18n.Key{i18n.SettingsKeywordRules},
			name: rule.Keyword,
			control: func() {
				ui.Row(c).Shrink(0).GapX(SettingsActionGap).AlignItems(ui.Center).Children(func() {
					settingsQuiet(c, theme, "→ "+rule.TagName)
					// The keyword names the row: one control per rule, all drawn with the
					// same glyph, so the bare "Remove" answers for every rule at once
					// (round 42, L28).
					settingsIcon(c, theme, rail, "close",
						s.t().Tf(i18n.SettingsRemoveKeywordRuleFor, rule.Keyword), func() {
							s.removeKeywordRule(index)
						})
				})
			},
		})
	}
	items = append(items, settingsItem{
		keys: []i18n.Key{i18n.SettingsAddKeywordRule, i18n.SettingsKeywordPlaceholder, i18n.SettingsRuleTagPlaceholder},
		// No row name: the button at the end of the editor is already labelled "Add
		// rule", and a name above it repeats those words, so the page draws the same
		// control twice and the row's copy covers the button's width (round 42, L30).
		body: func() {
			ui.Row(c).FillWidth().GapX(SettingsActionGap).AlignItems(ui.Center).Children(func() {
				settingsField(c, theme, rail, &s.state.DraftKeyword,
					s.t().T(i18n.SettingsKeywordPlaceholder), s.t().T(i18n.SettingsKeywordPlaceholder), 0, nil)
				ui.Text(c, s.t().T(i18n.SettingsRuleArrow)).FontSize(SettingsRowNameFont).
					TextColor(theme.TextMuted)
				settingsField(c, theme, rail, &s.state.DraftRuleTag,
					s.t().T(i18n.SettingsRuleTagPlaceholder), s.t().T(i18n.SettingsRuleTagPlaceholder), 0, nil)
				settingsAction(c, theme, rail, s.t().T(i18n.SettingsAddKeywordRule), false, false, func() {
					s.addKeywordRule()
				})
			})
		},
	})
	return s.settingsCard(c, theme, rail, items...)
}

// autoTagPacingCard is 请求节奏: the two numbers that bound one pass.
func (s *Shell) autoTagPacingCard(c *ui.Context, theme *ui.Theme, rail RailTokens) int {
	save := func() { s.saveAutoTagSettings(context.Background()) }
	return s.settingsCard(c, theme, rail,
		settingsItem{
			keys: []i18n.Key{i18n.SettingsBatchSize},
			name: s.t().T(i18n.SettingsBatchSize),
			control: func() {
				// 1..50 is the range the engine and the extension both enforce
				// (budget.go MaxBatchSize, settings.ts:271). Offering 100 here let the
				// field accept a value a run would silently clamp.
				settingsNumber(c, theme, rail, &s.state.BatchSizeInput, 1,
					float64(engine.MaxBatchSize), 1, s.t().T(i18n.SettingsBatchSize), save)
			},
		},
		settingsItem{
			keys: []i18n.Key{i18n.SettingsConcurrency, i18n.SettingsConcurrencyHint},
			name: s.t().T(i18n.SettingsConcurrency),
			sub:  s.t().T(i18n.SettingsConcurrencyHint),
			control: func() {
				// 1..4, not 0..4: NormalizeSettings turns 0 into the default of 3, so
				// a stored 0 never meant "one at a time" -- it meant three requests
				// at once, which is the opposite of what the field promised.
				settingsNumber(c, theme, rail, &s.state.ConcurrencyInput,
					float64(engine.MinConcurrency), float64(engine.MaxConcurrency), 1,
					s.t().T(i18n.SettingsConcurrency), save)
			},
		},
	)
}

// autoTagModelCard is 大模型标签: who answers, with what credential, and how much of the
// vocabulary a run may add.
func (s *Shell) autoTagModelCard(c *ui.Context, theme *ui.Theme, rail RailTokens) int {
	save := func() { s.saveAutoTagSettings(context.Background()) }

	// The endpoint warning is the row's explanation while the field holds a host: the key is
	// sent there.
	baseSub, baseInk := "", theme.TextMuted
	if strings.TrimSpace(s.state.BaseURLInput) != "" {
		baseSub, baseInk = s.t().T(i18n.SettingsBaseURLWarning), theme.Warning
	}

	// The test result takes the row's line once one has run; until then the row says what
	// the test does.
	testSub, testInk := s.t().T(i18n.SettingsLlmTestHint), theme.TextMuted
	if s.state.ConnectionTested {
		testSub, testInk = s.state.ConnectionResult, theme.Warning
		if s.state.ConnectionOK {
			testInk = theme.Success
		}
	}
	if s.state.ConnectionRunning {
		testSub, testInk = s.t().T(i18n.SettingsXTestRunning), rail.TextTertiary
	}

	keyStatus := s.t().T(i18n.SettingsAPIKeyAbsent)
	if s.state.APIKeyStored {
		keyStatus = s.t().T(i18n.SettingsAPIKeyPresent)
	}

	items := []settingsItem{
		{
			keys: []i18n.Key{i18n.SettingsLlmEnabled},
			name: s.t().T(i18n.SettingsLlmEnabled),
			control: func() {
				settingsSwitch(c, theme, rail, s.t().T(i18n.SettingsOff), s.t().T(i18n.SettingsOn),
					s.t().T(i18n.SettingsLlmEnabled), &s.state.LLMEnabled, save)
			},
		},
		{
			keys: []i18n.Key{i18n.SettingsProvider},
			name: s.t().T(i18n.SettingsProvider),
			control: func() {
				settingsPick(c, theme, rail, s.t().T(i18n.SettingsProvider),
					&s.state.ProviderLabel, s.providerLabels(), save)
			},
		},
		{
			keys: []i18n.Key{i18n.SettingsModel},
			name: s.t().T(i18n.SettingsModel),
			control: func() {
				settingsField(c, theme, rail, &s.state.ModelInput, "",
					s.t().T(i18n.SettingsModel), SettingsInputWidth, save)
			},
		},
		{
			keys:     []i18n.Key{i18n.SettingsBaseURL, i18n.SettingsBaseURLWarning},
			name:     s.t().T(i18n.SettingsBaseURL),
			sub:      baseSub,
			subColor: baseInk,
			control: func() {
				settingsField(c, theme, rail, &s.state.BaseURLInput,
					s.t().T(i18n.SettingsBaseURL), s.t().T(i18n.SettingsBaseURL), SettingsFieldWide, save)
			},
		},
	}
	// Only Azure reads a deployment name: it is part of the URL rather than the model field.
	if llm.ProviderByID(s.providerIDFor(s.state.ProviderLabel)).NeedsDeployment {
		items = append(items, settingsItem{
			keys: []i18n.Key{i18n.SettingsDeployment, i18n.SettingsDeploymentHint},
			name: s.t().T(i18n.SettingsDeployment),
			sub:  s.t().T(i18n.SettingsDeploymentHint),
			control: func() {
				settingsField(c, theme, rail, &s.state.DeploymentInput,
					s.t().T(i18n.SettingsDeployment), s.t().T(i18n.SettingsDeployment), SettingsInputWidth, save)
			},
		})
	}
	items = append(items,
		settingsItem{
			keys: []i18n.Key{i18n.SettingsMaxPerRun},
			name: s.t().T(i18n.SettingsMaxPerRun),
			control: func() {
				// 1..2000, the range the engine and the extension share. The field used
				// to promise "0 = no limit" while NormalizeSettings turned 0 into 200,
				// so the value the reader typed and the value a run used disagreed.
				settingsNumber(c, theme, rail, &s.state.MaxPerRunInput,
					float64(engine.MinMaxBookmarksPerRun), float64(engine.MaxMaxBookmarksPerRun), 10,
					s.t().T(i18n.SettingsMaxPerRun), save)
			},
		},
		settingsItem{
			keys: []i18n.Key{i18n.SettingsVocabularyMode},
			name: s.t().T(i18n.SettingsVocabularyMode),
			control: func() {
				settingsPick(c, theme, rail, s.t().T(i18n.SettingsVocabularyMode),
					&s.state.VocabularyModeLabel, s.vocabularyModeLabels(), save)
			},
		},
		settingsItem{
			keys: []i18n.Key{i18n.SettingsAPIKey, i18n.SettingsAPIKeyPresent, i18n.SettingsAPIKeyAbsent},
			name: s.t().T(i18n.SettingsAPIKey),
			sub:  keyStatus,
			body: func() {
				ui.Row(c).FillWidth().GapX(SettingsActionGap).AlignItems(ui.Center).Children(func() {
					// The field is cleared as soon as it is saved, so the secret is not left in
					// a control the next frame draws. A stored key still shows a mask in the
					// empty field: an empty box beside "a key is stored" reads as a missing
					// value, and a reader who cannot see that anything is there retypes it.
					keyPlaceholder := ""
					if s.state.APIKeyStored {
						keyPlaceholder = s.t().T(i18n.SettingsAPIKeyMask)
					}
					settingsField(c, theme, rail, &s.state.APIKeyInput, keyPlaceholder,
						s.t().T(i18n.SettingsAPIKey), 0, nil)
					// Save has nothing to do with an empty field, and Clear has nothing to do
					// when no key is stored. Both were drawn as live controls either way, so a
					// press did nothing and said nothing -- the reader cannot tell a dead
					// button from a broken one. The source disables Save until there is
					// something to save and does not render Clear at all without a key
					// (SettingsPanel.tsx:1032-1048).
					saveDisabled := strings.TrimSpace(s.state.APIKeyInput) == ""
					settingsAction(c, theme, rail, s.t().T(i18n.ActionSave), saveDisabled, false, func() {
						s.saveAPIKey()
					})
					if s.state.APIKeyStored {
						settingsAction(c, theme, rail, s.t().T(i18n.ActionClear), false, true, func() {
							s.clearAPIKey()
						})
					}
				})
			},
		},
		settingsItem{
			keys:     []i18n.Key{i18n.SettingsLlmConnectionRow, i18n.ActionTestConnection, i18n.SettingsLlmTestHint},
			name:     s.t().T(i18n.SettingsLlmConnectionRow),
			sub:      testSub,
			subColor: testInk,
			control: func() {
				settingsActionBusy(c, theme, rail, s.t().T(i18n.ActionTestConnection),
					s.state.ConnectionRunning, func() { s.TestConnection(context.Background()) })
			},
		},
	)

	drawn := s.settingsCard(c, theme, rail, items...)
	// A run at the ceiling cannot mint tags; the source says so before the run.
	if len(s.state.TagItems) > HealthyTagCeiling() {
		settingsNote(c, theme, rail, s.t().T(i18n.SettingsAutoTagVocabularyFull))
	}
	return drawn
}
