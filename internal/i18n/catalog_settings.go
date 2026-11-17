package i18n

// The settings page and the model's errors: every row, its hint and the auto-tag
// run's report.

const (
	SettingsTitle Key = "settings.title"
	// SettingsSearchPlaceholder and SettingsNoMatch are the "Find a setting" prompt and its
	// no-hit line.
	SettingsSearchPlaceholder       Key = "settings.searchPlaceholder"
	SettingsNoMatch                 Key = "settings.noMatch"
	SettingsBack                    Key = "settings.back"
	SettingsLanguage                Key = "settings.language"
	SettingsData                    Key = "settings.data"
	SettingsAutoTagEnabled          Key = "settings.autoTagEnabled"
	SettingsAutoTagEnabledHint      Key = "settingsAutoTagEnabledHint"
	SettingsAutoTagKeywordRulesHint Key = "settingsAutoTagKeywordRulesHint"
	SettingsAutoTagVocabularyFull   Key = "settingsAutoTagVocabularyFull"
	SettingsAutoContinue            Key = "settings.autoContinue"
	SettingsBatchSize               Key = "settings.batchSize"
	SettingsConcurrency             Key = "settings.concurrency"
	SettingsConcurrencyHint         Key = "settings.concurrencyHint"
	SettingsUseHashtags             Key = "settings.useHashtags"
	SettingsUseAuthors              Key = "settings.useAuthors"
	SettingsKeywordRules            Key = "settings.keywordRules"
	SettingsKeywordRulesEmpty       Key = "settings.keywordRulesEmpty"
	SettingsKeywordPlaceholder      Key = "settings.keywordPlaceholder"
	SettingsRuleTagPlaceholder      Key = "settings.ruleTagPlaceholder"
	SettingsProvider                Key = "settings.provider"
	SettingsModel                   Key = "settings.model"
	SettingsBaseURL                 Key = "settings.baseUrl"
	SettingsBaseURLWarning          Key = "settings.baseUrlWarning"
	// The endpoint field's two refusals: a value that is not a URL at all, and a plain
	// http address that would put the key on the wire in the clear. The second is a
	// question rather than an error -- the reader may allow that one address.
	SettingsBaseURLError          Key = "settings.baseUrlError"
	SettingsBaseURLPermission     Key = "settings.baseUrlPermission"
	SettingsBaseURLPermissionHint Key = "settings.baseUrlPermissionHint"
	SettingsDeployment            Key = "settings.deployment"
	SettingsDeploymentHint        Key = "settings.deploymentHint"
	SettingsMaxPerRun             Key = "settings.maxPerRun"
	SettingsVocabularyMode        Key = "settings.vocabularyMode"
	SettingsVocabularyOpen        Key = "settings.vocabularyOpen"
	SettingsVocabularyGuarded     Key = "settings.vocabularyGuarded"
	SettingsVocabularyClosed      Key = "settings.vocabularyClosed"
	// The auto-tagging page's cards. They are page structure rather than settings copy,
	// which is why they are not named settings.*: the feature lives in one sheet and has no
	// card headings.
	AutoTagRunHint            Key = "autoTagRunHint"
	AutoTagNoRunsYet          Key = "autoTagNoRunsYet"
	AutoTagBasicsHint         Key = "autoTagBasicsHint"
	AutoTagSources            Key = "autoTagSources"
	AutoTagPacing             Key = "autoTagPacing"
	AutoTagPacingHint         Key = "autoTagPacingHint"
	AutoTagModelHint          Key = "autoTagModelHint"
	SettingsAPIKey            Key = "settings.apiKey"
	SettingsAPIKeyPresent     Key = "settings.apiKeyPresent"
	SettingsAPIKeyAbsent      Key = "settings.apiKeyAbsent"
	SettingsAPIKeyMask        Key = "settings.apiKeyMask"
	SettingsSectionGeneral    Key = "settingsSectionGeneral"
	SettingsSectionSync       Key = "settingsSectionSync"
	SettingsSectionAutoTag    Key = "settingsSectionAutoTag"
	SettingsSectionDataSafety Key = "settingsSectionDataSafety"
	// SettingsSectionListsViews is the tab the lists and the saved views live on. They used
	// to sit at the bottom of Data safety, under a sentence about backups, where nobody
	// looking for their views would think to go.
	SettingsSectionListsViews Key = "settingsSectionListsViews"
	// SettingsVersion is the row's name; SettingsVersionLine and SettingsVersionLineNever are
	// what it says under it, with the version and, when there has been one, the last check.
	SettingsVersion          Key = "settingsVersion"
	SettingsVersionLine      Key = "settingsVersionLine"
	SettingsVersionLineNever Key = "settingsVersionLineNever"
	// SettingsAutoCheck is the switch that decides whether the app looks for a new version in
	// the background. Its hint says how often, so the switch is not a guess about the cost.
	SettingsAutoCheck          Key = "settingsAutoCheck"
	SettingsAutoCheckHint      Key = "settingsAutoCheckHint"
	SettingsGroupAutoTagBasics Key = "settingsGroupAutoTagBasics"
	SettingsGroupAutoTagLlm    Key = "settingsGroupAutoTagLlm"
	SettingsLlmEnabled         Key = "settingsLlmEnabled"
	SettingsAddKeywordRule     Key = "settingsAddKeywordRule"
	// SettingsRemoveKeywordRuleFor is the remove control at the end of one keyword rule's
	// row. Labelling every one of them with the same words answers "remove a rule" without
	// saying which; the keyword names the row, so the name carries it.
	SettingsRemoveKeywordRuleFor Key = "settingsRemoveKeywordRuleFor"
	// SettingsRuleArrow draws the arrow between a keyword and the tag it suggests. A symbol, so
	// both catalogues hold the same character.
	SettingsRuleArrow        Key = "settingsRuleArrow"
	SettingsClearBlockedTags Key = "settingsClearBlockedTags"
	// Each preference row carries its own line: what
	// the setting does, before the control that changes it.
	SettingsLanguageHint     Key = "settingsLanguageHint"
	LanguageSwitch           Key = "sidebarLanguageSwitch"
	SettingsThemeHint        Key = "settingsThemeHint"
	SettingsBackupHint       Key = "settingsBackupHint"
	SettingsAutoBackupRow    Key = "settingsAutoBackupRow"
	SettingsAutoBackupHint   Key = "settingsAutoBackupHint"
	SettingsOpenBackupDir    Key = "settingsOpenBackupDir"
	SettingsBackupDirHint    Key = "settingsBackupDirHint"
	SettingsBackupDone       Key = "settingsBackupDone"
	SettingsBackupFailed     Key = "settingsBackupFailed"
	SettingsFileLogRow       Key = "settingsFileLogRow"
	SettingsFileLogHint      Key = "settingsFileLogHint"
	SettingsFileLogFailed    Key = "settingsFileLogFailed"
	SettingsLegacyBackupHint Key = "settingsLegacyBackupHint"
	SettingsRestoreHint      Key = "settingsRestoreHint"
	SettingsLocalHint        Key = "settingsLocalHint"
	SettingsDataSafetyHint   Key = "settingsDataSafetyHint"
	ResetLocalData           Key = "resetLocalData"
	ResetLocalDataConfirm    Key = "resetLocalDataConfirm"
	ResettingData            Key = "resettingData"
	LocalDataResetDone       Key = "localDataReset"
	RestoreBackup            Key = "restoreBackup"
	RestoringBackup          Key = "restoringBackup"
	RestoreBackupConfirm     Key = "restoreBackupConfirm"
	SettingsStopBufferPages  Key = "settingsIncrementalStopBufferPages"
	SettingsStopBufferHint   Key = "settingsIncrementalStopBufferPagesHint"
	SettingsPageLimit        Key = "settingsSyncPageLimit"
	SettingsPageLimitHint    Key = "settingsSyncPageLimitHint"
	SettingsRunAfterSync     Key = "settingsAutoTagRunAfterSync"
	SettingsXBrowserRead     Key = "settingsXBrowserRead"
	// SettingsXBrowserHint and SettingsXTestHint explain the two credential actions
	// before they have run, so their rows carry a line like every other preference row.
	SettingsXBrowserHint                Key = "settingsXBrowserHint"
	SettingsXTestHint                   Key = "settingsXTestHint"
	SettingsXBrowserReading             Key = "settingsXBrowserReading"
	SettingsXBrowserSaved               Key = "settingsXBrowserSaved"
	SettingsXBrowserFailed              Key = "settingsXBrowserFailed"
	SettingsXBrowserNeedsAccess         Key = "settingsXBrowserNeedsAccess"
	SettingsXBrowserNoCookies           Key = "settingsXBrowserNoCookies"
	SettingsXCookiesTitle               Key = "settingsXCookiesTitle"
	SettingsXCookiesHint                Key = "settingsXCookiesHint"
	SettingsXCredentialPaste            Key = "settingsXCredentialPaste"
	SettingsXCredentialPasteHint        Key = "settingsXCredentialPasteHint"
	SettingsXCredentialPastePlaceholder Key = "settingsXCredentialPlaceholder"
	SettingsXCredentialMissing          Key = "settingsXCredentialMissing"
	SettingsXCredentialNothingFound     Key = "settingsXCredentialNoCookies"
	SettingsXBrowserUnchanged           Key = "settingsXBrowserUnchanged"
	SettingsXCookiesSaved               Key = "settingsXCookiesSaved"
	SettingsXCookiesAbsent              Key = "settingsXCookiesAbsent"
	// The connection test, which answers "are these cookies and this network enough"
	// without a full sync. It exists because a failed sync could not say which of the
	// two was wrong: an unreachable network and an expired cookie both ended as a
	// sentence about the login.
	SettingsXTestConnection    Key = "settingsXTestConnection"
	SettingsXTestRunning       Key = "settingsXTestRunning"
	SettingsXTestOK            Key = "settingsXTestOK"
	SettingsXTestNoCredentials Key = "settingsXTestNoCredentials"
	SettingsXTestNoProbe       Key = "settingsXTestNoProbe"
	SettingsXTestDirect        Key = "settingsXTestDirect"
	// SettingsProxyRoute names the proxy a request actually used, so a failure on a
	// proxied network is distinguishable from one that went direct.
	SettingsProxyRoute Key = "settingsProxyRoute"

	SettingsSyncOnLaunch    Key = "settingsSyncOnLaunch"
	SettingsSaveRawPayloads Key = "settingsSaveRawPayloads"
	// SettingsOff and SettingsOn are the two halves of a settings switch,
	// Off/On. The words are Chinese: 关闭 / 开启.
	SettingsOff Key = "settingsOff"
	SettingsOn  Key = "settingsOn"
	// SettingsNewList and SettingsNewView name the two rows that create one.
	SettingsNewList Key = "settingsNewList"
	SettingsNewView Key = "settingsNewView"
	// The two system list rows say why they are there and what cannot be done to them.
	SettingsUnclassifiedHint Key = "settingsUnclassifiedHint"
	SettingsInboxHint        Key = "settingsInboxHint"
	// The rows that name an object and put the verb on their action: a row called
	// "Export data" with a button "Export data" says one thing twice.
	SettingsBackupRow       Key = "settingsBackupRow"
	SettingsLegacyBackupRow Key = "settingsLegacyBackupRow"
	SettingsRestoreRow      Key = "settingsRestoreRow"
	SettingsLocalRow        Key = "settingsLocalRow"
	SettingsBlockedTags     Key = "settingsBlockedTags"
	// The row is native, so it needs its own explanation line and its own acknowledgement;
	// the "done" sentence belongs with it.
	SettingsBlockedTagsHint      Key = "settingsBlockedTagsHint"
	SettingsClearBlockedTagsDone Key = "settingsClearBlockedTagsDone"
	SettingsXBrowserRow          Key = "settingsXBrowserRow"
	SettingsXTestRow             Key = "settingsXTestRow"
	// The LLM endpoint gets the same pair: a row named for the test, and the
	// sentence saying what it does before one has run.
	SettingsLlmConnectionRow Key = "settingsLlmConnectionRow"
	SettingsLlmTestHint      Key = "settingsLlmTestHint"
	// SettingsLlmTestMissingKey is the sentence for a test with no key stored. Without it
	// the panel reused the "invalid or expired key" sentence, which sends the user to
	// replace a key that was never entered.
	SettingsLlmTestMissingKey Key = "settingsLlmTestMissingKey"
	LLMErrorUnauthorized      Key = "settingsLlmErrorUnauthorized"
	LLMErrorForbidden         Key = "settingsLlmErrorForbidden"
	LLMErrorRateLimit         Key = "settingsLlmErrorRateLimit"
	LLMErrorRateLimitLong     Key = "settingsLlmErrorRateLimitLong"
	LLMErrorQuota             Key = "settingsLlmErrorQuota"
	LLMErrorBadRequest        Key = "settingsLlmErrorBadRequest"
	LLMErrorUnsupportedParam  Key = "settingsLlmErrorUnsupportedParam"
	LLMErrorContextLength     Key = "settingsLlmErrorContextLength"
	LLMErrorTimeout           Key = "settingsLlmErrorTimeout"
	LLMErrorNetwork           Key = "settingsLlmErrorNetwork"
	LLMErrorServer            Key = "settingsLlmErrorServer"
	LLMErrorContentFilter     Key = "settingsLlmErrorContentFilter"
	LLMErrorInvalidJson       Key = "settingsLlmErrorInvalidJson"
	LLMErrorUnknown           Key = "settingsLlmErrorUnknown"
	AutoTagTitle              Key = "settingsAutoTagResultTitle"
	AutoTagAt                 Key = "settingsAutoTagResultAt"
	AutoTagDetail             Key = "settingsAutoTagResultDetail"
	AutoTagTokens             Key = "settingsAutoTagResultTokens"
	AutoTagBlocked            Key = "settingsAutoTagResultBlocked"
	AutoTagBlockedDropped     Key = "settingsAutoTagResultBlockedDropped"
	AutoTagBudgetDropped      Key = "settingsAutoTagResultBudgetDropped"
	OutcomeOk                 Key = "settingsAutoTagResultOk"
	// OutcomeDisabled is the sentence for pressing run while the feature's own switch is
	// off. The fallthrough "Done: 0 new suggestions" reads as a finished run; saying the
	// switch is off is the only reading of the same fact that is true.
	OutcomeDisabled        Key = "settingsAutoTagResultDisabled"
	OutcomeNoCandidates    Key = "settingsAutoTagResultNoCandidates"
	OutcomeEmptyVocabulary Key = "settingsAutoTagResultEmptyVocabulary"
	OutcomeLlmDisabled     Key = "settingsAutoTagResultLlmDisabled"
	OutcomeMissingKey      Key = "settingsAutoTagResultMissingKey"
	OutcomeFailed          Key = "settingsAutoTagResultFailed"
	OutcomeAllBlocked      Key = "settingsAutoTagResultAllBlocked"
	OutcomeBudgetExhausted Key = "settingsAutoTagResultBudgetExhausted"
	OutcomeTimedOut        Key = "settingsAutoTagResultTimedOut"
	OutcomeInterrupted     Key = "settingsAutoTagResultInterrupted"
	OutcomeCancelled       Key = "settingsAutoTagResultCancelled"
	OutcomeErrorBudget     Key = "settingsAutoTagResultErrorBudget"
	RunPhaseHeuristics     Key = "settingsAutoTagPhaseHeuristics"
	RunPhaseLlm            Key = "settingsAutoTagPhaseLlm"
	RunAnalyzing           Key = "settingsAutoTagProgress"
	RunWaiting             Key = "settingsAutoTagWaiting"
	RunStalled             Key = "settingsAutoTagStalled"
	RunElapsed             Key = "settingsAutoTagElapsed"
	RunRemaining           Key = "settingsAutoTagRemaining"
	RunBudgetRemaining     Key = "settingsAutoTagBudgetRemaining"
	RunSuggestionsMade     Key = "settingsAutoTagSuggestionsMade"
	RunRecentTags          Key = "settingsAutoTagRecentTags"
	RunStopping            Key = "settingsAutoTagStopping"
	RunCancelRequested     Key = "settingsAutoTagCancelRequested"
	RunAlreadyRunning      Key = "settingsAutoTagAlreadyRunning"
	ConnectionOK           Key = "connection.ok"
	ConnectionFailed       Key = "connection.failed"
	ConnectionNoTester     Key = "connection.noTester"
	// ConnectionTesting covers the seconds a provider probe spends on the network.
	ConnectionTesting Key = "connection.testing"
	ThemeLabel        Key = "themeLabel"
)

var settingsMessages = messages{
	EN: {
		SettingsTitle:                       "Settings",
		SettingsSearchPlaceholder:           "Find a setting",
		SettingsNoMatch:                     "No setting matches \u201c%s\u201d",
		SettingsBack:                        "Back to bookmarks",
		SettingsLanguage:                    "Language",
		SettingsData:                        "Data",
		SettingsAutoTagEnabled:              "Enable auto-tagging",
		SettingsAutoTagEnabledHint:          "Generates tag suggestions for untagged bookmarks after sync. Nothing is applied until you accept.",
		SettingsAutoTagKeywordRulesHint:     "Optional. A post whose text contains the keyword is suggested that tag without calling the model; matching ignores case. For example: sourdough → Baking.",
		SettingsAutoTagVocabularyFull:       "The tag library reached its healthy size, so this run cannot mint new tags.",
		SettingsAutoContinue:                "Continue automatically when work is left",
		SettingsBatchSize:                   "Bookmarks per request",
		SettingsConcurrency:                 "Parallel requests (1–4)",
		SettingsConcurrencyHint:             "More parallel requests can trigger rate limits.",
		SettingsUseHashtags:                 "Use #hashtags from tweets",
		SettingsUseAuthors:                  "Suggest tags from author history",
		SettingsKeywordRules:                "Keyword rules",
		SettingsKeywordRulesEmpty:           "No keyword rules yet. Add one below if a few words should always point at the same tag; this is optional and needs no model.",
		SettingsKeywordPlaceholder:          "keyword",
		SettingsRuleTagPlaceholder:          "tag",
		SettingsProvider:                    "Provider",
		SettingsModel:                       "Model",
		SettingsBaseURL:                     "Endpoint (blank uses the provider's)",
		SettingsBaseURLWarning:              "Your API key is sent to this endpoint.",
		SettingsBaseURLError:                "Enter a full URL, such as https://api.example.com/v1.",
		SettingsBaseURLPermission:           "Allow trove to send the API key to %s in plain text?",
		SettingsBaseURLPermissionHint:       "This address uses http, so the key travels unencrypted. Allow it only for a server you trust.",
		SettingsDeployment:                  "Deployment name",
		SettingsDeploymentHint:              "Azure OpenAI only: the deployment name from the Azure portal.",
		SettingsMaxPerRun:                   "Max bookmarks per run",
		SettingsVocabularyMode:              "Tag vocabulary",
		SettingsVocabularyOpen:              "Allow new tags",
		SettingsVocabularyGuarded:           "Limited new tags",
		SettingsVocabularyClosed:            "Existing tags only",
		AutoTagRunHint:                      "Suggests tags for untagged bookmarks. Nothing is written until you accept.",
		AutoTagNoRunsYet:                    "No runs yet. Analyze now proposes the first tags.",
		AutoTagBasicsHint:                   "When a pass starts, and which signals its suggestions come from.",
		AutoTagSources:                      "Sources",
		AutoTagPacing:                       "Request pacing",
		AutoTagPacingHint:                   "How many bookmarks go in one request, and how many requests run at once.",
		AutoTagModelHint:                    "Sends batches to a model to propose tags. With this off, only rules and author history are used.",
		SettingsAPIKey:                      "API key",
		SettingsAPIKeyPresent:               "A key is stored in the Keychain.",
		SettingsAPIKeyAbsent:                "No key stored.",
		SettingsAPIKeyMask:                  "••••••••••••",
		SettingsSectionGeneral:              "General",
		SettingsSectionSync:                 "Sync",
		SettingsSectionAutoTag:              "Auto-tagging",
		SettingsSectionDataSafety:           "Data safety",
		SettingsSectionListsViews:           "Lists & views",
		SettingsVersion:                     "Version",
		SettingsVersionLine:                 "trove %s · last checked %s",
		SettingsVersionLineNever:            "trove %s · not checked since it was installed",
		SettingsAutoCheck:                   "Check for updates automatically",
		SettingsAutoCheckHint:               "Once a day, in the background. A new version opens the update window.",
		SettingsGroupAutoTagBasics:          "Rule-based tagging",
		SettingsGroupAutoTagLlm:             "LLM tagging",
		SettingsLlmEnabled:                  "Send batches to a model",
		SettingsAddKeywordRule:              "Add rule",
		SettingsRemoveKeywordRuleFor:        "Remove keyword rule %s",
		SettingsRuleArrow:                   "→",
		SettingsClearBlockedTags:            "Clear block list",
		SettingsLanguageHint:                "The language of the interface and its copy; the sidebar footer switches it too.",
		LanguageSwitch:                      "Switch language",
		SettingsThemeHint:                   "Follows the system unless you pick light or dark.",
		SettingsBackupHint:                  "Every bookmark, tag and saved view, as a JSON file.",
		SettingsAutoBackupRow:               "Automatic backups",
		SettingsAutoBackupHint:              "One copy a day, up to seven, beside the library.",
		SettingsOpenBackupDir:               "Open backup folder",
		SettingsBackupDirHint:               "The copies are ordinary database files you can move or copy elsewhere.",
		SettingsBackupDone:                  "Backed up to %s",
		SettingsBackupFailed:                "Automatic backup failed: %s",
		SettingsFileLogRow:                  "Troubleshooting log",
		SettingsFileLogHint:                 "Writes events and counts to logs/trove.log, keeping three files of 1 MB each. Never keys, cookies or post text.",
		SettingsFileLogFailed:               "Could not open the log: %s",
		SettingsLegacyBackupHint:            "A backup in the legacy exportVersion 2 format.",
		SettingsRestoreHint:                 "Read a backup file; this overwrites the local data.",
		SettingsLocalHint:                   "Clear the local bookmarks, tags and settings. This cannot be undone.",
		SettingsDataSafetyHint:              "Backups stay on this device; nothing is uploaded. The sidebar footer keeps the same shortcuts.",
		ResetLocalData:                      "Reset local data",
		ResetLocalDataConfirm:               "Reset will clear all local bookmarks, tags, and settings. Continue?",
		ResettingData:                       "Resetting...",
		LocalDataResetDone:                  "Local data reset",
		RestoreBackup:                       "Import restore",
		RestoringBackup:                     "Restoring...",
		RestoreBackupConfirm:                "Import restore will overwrite current local data. Continue?",
		SettingsStopBufferPages:             "Incremental stop buffer pages",
		SettingsStopBufferHint:              "Extra pages fetched after incremental sync hits known bookmarks. Larger is safer but slower.",
		SettingsPageLimit:                   "First sync bookmark limit",
		SettingsPageLimitHint:               "How many bookmarks one full walk may collect. 0 uses the built-in 10,000; the next sync continues from where it stopped.",
		SettingsRunAfterSync:                "Run automatically after sync",
		SettingsXBrowserRead:                "Read from browser",
		SettingsXBrowserHint:                "Take the session from a browser signed in to x.com on this Mac; the cookies are kept in the Keychain.",
		SettingsXTestHint:                   "One request to x.com, so a bad cookie and an unreachable network are told apart before a sync.",
		SettingsXBrowserReading:             "Looking for a signed-in browser…",
		SettingsXBrowserSaved:               "Read from the browser and saved to the Keychain.",
		SettingsXBrowserFailed:              "Could not read the browser: %s",
		SettingsXBrowserNeedsAccess:         "The system is blocking access to the browser profiles. Allow this app in your privacy settings, then try again -- or paste the Cookie header below.",
		SettingsXBrowserNoCookies:           "No X cookies found in any browser. Sign in to x.com in your browser first, or paste the Cookie header below. (%s)",
		SettingsXCookiesTitle:               "X credentials",
		SettingsXCookiesHint:                "Sign in to x.com in a browser first. Reading that session is one click; the cookies are kept in this Mac's Keychain and never leave it.",
		SettingsXCredentialPaste:            "Cookie or cURL",
		SettingsXCredentialPasteHint:        "Or paste it by hand: DevTools > Network > any request > copy the Cookie request header. A curl command or an exported cookie list works too; both values are read out of it.",
		SettingsXCredentialPastePlaceholder: "auth_token=…; ct0=…",
		SettingsXCredentialMissing:          "That paste has no %s. The whole Cookie request header carries both.",
		SettingsXCredentialNothingFound:     "No auth_token or ct0 in that paste. Copy the Cookie request header, not the response.",
		SettingsXBrowserUnchanged:           "The browser and the Keychain hold the same cookies. Sign in to x.com in the browser if the session has expired.",
		SettingsXCookiesSaved:               "Saved to the Keychain.",
		SettingsXCookiesAbsent:              "No X credentials stored yet, so sync cannot run.",
		SettingsXTestConnection:             "Test connection",
		SettingsXTestRunning:                "Testing…",
		SettingsXTestOK:                     "The cookies work: X answered with a bookmarks page.",
		SettingsXTestNoCredentials:          "Connect or save the cookies first, then test.",
		SettingsXTestNoProbe:                "This build cannot reach X: no connection test is installed.",
		SettingsXTestDirect:                 "direct (no proxy)",
		SettingsProxyRoute:                  "Network route: %s",
		SettingsSyncOnLaunch:                "Sync when the app opens",
		SettingsSaveRawPayloads:             "Keep raw responses",
		SettingsOff:                         "Off",
		SettingsOn:                          "On",
		SettingsNewList:                     "New list",
		SettingsNewView:                     "New view",
		SettingsUnclassifiedHint:            "Bookmarks that belong to no list.",
		SettingsInboxHint:                   "Where unsorted bookmarks land; it cannot be renamed or deleted.",
		SettingsBackupRow:                   "Backup",
		SettingsLegacyBackupRow:             "Legacy backup",
		SettingsRestoreRow:                  "Restore",
		SettingsLocalRow:                    "Local data",
		SettingsBlockedTags:                 "Blocked tags",
		SettingsBlockedTagsHint:             "Names of deleted tags are remembered here so auto-tagging stops suggesting them.",
		SettingsClearBlockedTagsDone:        "Block list cleared.",
		SettingsXBrowserRow:                 "Browser sign-in",
		SettingsXTestRow:                    "Connection test",
		SettingsLlmConnectionRow:            "Connection test",
		SettingsLlmTestHint:                 "One request to the provider, so a bad key and an unreachable endpoint are told apart before a run.",
		SettingsLlmTestMissingKey:           "Enter and save an API key first.",
		LLMErrorUnauthorized:                "Authentication failed: the API key is invalid or expired.",
		LLMErrorForbidden:                   "The provider refused the request (403). Check permissions or regional limits.",
		LLMErrorRateLimit:                   "Rate limited (429). Try again shortly.",
		LLMErrorRateLimitLong:               "The provider asked to wait about %ds before retrying.",
		LLMErrorQuota:                       "Out of quota or a billing problem. Check the account balance.",
		LLMErrorBadRequest:                  "The request was rejected. Check the model id and endpoint.",
		LLMErrorUnsupportedParam:            "The provider does not support some parameters; they were dropped.",
		LLMErrorContextLength:               "The input exceeds the model's context length.",
		LLMErrorTimeout:                     "The request timed out. Check the network or try another provider.",
		LLMErrorNetwork:                     "Network error: could not reach that endpoint.",
		LLMErrorServer:                      "The provider returned a server error. Try again shortly.",
		LLMErrorContentFilter:               "The provider's safety policy blocked the content.",
		LLMErrorInvalidJson:                 "The model did not return parsable JSON.",
		LLMErrorUnknown:                     "Connection failed.",
		AutoTagTitle:                        "Last analysis",
		AutoTagAt:                           "Recorded %s",
		AutoTagDetail:                       "%d candidates, %d analyzed, %d batches (%d failed), %d tags in vocabulary, new tag budget %d.",
		AutoTagTokens:                       "This run used about %d input and %d output tokens.",
		AutoTagBlocked:                      "%d tag names are blocked from deletions and will never be suggested again.",
		AutoTagBlockedDropped:               "%d suggestions were dropped by the block list in this run.",
		AutoTagBudgetDropped:                "%d suggestions were dropped because the new-tag budget ran out.",
		OutcomeOk:                           "Done: %d new suggestions, %d awaiting review.",
		OutcomeDisabled:                     "Auto-tagging is off. Turn it on, then run the analysis.",
		OutcomeNoCandidates:                 "Nothing to analyze: every bookmark is already tagged or was analyzed before.",
		OutcomeEmptyVocabulary:              "The tag library is empty and the vocabulary mode is \"Existing tags only\", so the model had nothing to choose from and no request was sent. Switch the vocabulary mode or create a few tags first.",
		OutcomeLlmDisabled:                  "Rules only. LLM tagging is turned off.",
		OutcomeMissingKey:                   "No API key configured, LLM analysis was skipped.",
		OutcomeFailed:                       "LLM analysis failed",
		OutcomeAllBlocked:                   "The model returned tags, but every name is on the block list left behind by deleted tags, so nothing was suggested. Clear the block list and try again.",
		OutcomeBudgetExhausted:              "The model only proposed new tag names, but this run had no new-tag budget left, so nothing was suggested. Switch the vocabulary mode to allow new tags, or accept a few suggestions first to seed the vocabulary.",
		OutcomeTimedOut:                     "This run stopped at its time limit. %d suggestions were saved and %d bookmarks were not reached. Run again to continue.",
		OutcomeInterrupted:                  "The last run was interrupted (the browser or background worker ended). Saved suggestions were kept and %d bookmarks were left; it continued automatically.",
		OutcomeCancelled:                    "This run was cancelled. %d suggestions were saved and %d bookmarks were not analyzed. Run again to continue.",
		OutcomeErrorBudget:                  "The model service refused three batches in a row, so this run stopped early. %d suggestions were saved. Check the API key and the model name, then run again.",
		RunPhaseHeuristics:                  "Rule-based",
		RunPhaseLlm:                         "LLM",
		RunAnalyzing:                        "Analyzing",
		RunWaiting:                          "Waiting for the provider",
		RunStalled:                          "No heartbeat for %ds — it may have stopped",
		RunElapsed:                          "Elapsed %ds",
		RunRemaining:                        "~%ds left",
		RunBudgetRemaining:                  "this pass has ~%ds left",
		RunSuggestionsMade:                  "%d suggestion(s) added",
		RunRecentTags:                       "latest: %s",
		RunStopping:                         "Stopping…",
		RunCancelRequested:                  "Stop requested. The run ends after the current batch.",
		RunAlreadyRunning:                   "An analysis is already running. Wait for it to finish.",
		ConnectionOK:                        "Connected. %s answered.",
		ConnectionFailed:                    "The connection failed.",
		ConnectionNoTester:                  "No connection tester is configured.",
		ConnectionTesting:                   "Testing the connection…",
		ThemeLabel:                          "Theme",
	},
	ZHCN: {
		SettingsTitle:                       "设置",
		SettingsSearchPlaceholder:           "搜索设置",
		SettingsNoMatch:                     "没有匹配的设置：\u201c%s\u201d",
		SettingsBack:                        "返回书签",
		SettingsLanguage:                    "语言",
		SettingsData:                        "数据",
		SettingsAutoTagEnabled:              "启用自动标签",
		SettingsAutoTagEnabledHint:          "同步后自动为无标签书签产出建议，需你确认后才会生效。",
		SettingsAutoTagKeywordRulesHint:     "可选。推文文本里出现关键词时，直接建议对应标签，不调用大模型；匹配不区分大小写。例如：sourdough → 烘焙。",
		SettingsAutoTagVocabularyFull:       "标签数量已达健康上限，本次不会创建新标签。",
		SettingsAutoContinue:                "有剩余工作时自动续跑",
		SettingsBatchSize:                   "每请求书签数",
		SettingsConcurrency:                 "并行请求数（1–4）",
		SettingsConcurrencyHint:             "并行过多可能触发限流。",
		SettingsUseHashtags:                 "使用推文中的 #话题",
		SettingsUseAuthors:                  "根据作者历史标签推荐",
		SettingsKeywordRules:                "关键词规则",
		SettingsKeywordRulesEmpty:           "还没有关键词规则。如果有几个词总该对应同一个标签，可以在下面加一条；这是可选的，也不会调用大模型。",
		SettingsKeywordPlaceholder:          "关键词",
		SettingsRuleTagPlaceholder:          "标签",
		SettingsProvider:                    "服务商",
		SettingsModel:                       "模型",
		SettingsBaseURL:                     "接口地址（留空用服务商默认）",
		SettingsBaseURLWarning:              "你的 API key 会被发送到这个地址。",
		SettingsBaseURLError:                "请填写完整的 URL，例如 https://api.example.com/v1。",
		SettingsBaseURLPermission:           "允许 trove 把 API key 以明文发送到 %s 吗？",
		SettingsBaseURLPermissionHint:       "这个地址使用 http，API key 会以未加密的形式传输。只在你信任的服务器上允许。",
		SettingsDeployment:                  "部署名称",
		SettingsDeploymentHint:              "仅 Azure OpenAI 需要：Azure 门户里的 deployment 名称。",
		SettingsMaxPerRun:                   "单次分析上限",
		SettingsVocabularyMode:              "词表模式",
		SettingsVocabularyOpen:              "允许创建新标签",
		SettingsVocabularyGuarded:           "受限创建新标签",
		SettingsVocabularyClosed:            "仅使用现有标签",
		AutoTagRunHint:                      "为尚未打标签的书签生成建议；只有你确认后才会写入。",
		AutoTagNoRunsYet:                    "还没有运行记录。点「立即分析」生成第一批建议。",
		AutoTagBasicsHint:                   "自动分析何时开始，以及建议从哪些线索来。",
		AutoTagSources:                      "标签来源",
		AutoTagPacing:                       "请求节奏",
		AutoTagPacingHint:                   "单次请求发送多少书签，以及同时发几个请求。",
		AutoTagModelHint:                    "把批次交给模型生成候选标签；关闭时只用规则与作者历史。",
		SettingsAPIKey:                      "API key",
		SettingsAPIKeyPresent:               "Keychain 中已存有密钥。",
		SettingsAPIKeyAbsent:                "尚未存密钥。",
		SettingsAPIKeyMask:                  "••••••••••••",
		SettingsSectionGeneral:              "通用",
		SettingsSectionSync:                 "同步",
		SettingsSectionAutoTag:              "自动标签",
		SettingsSectionDataSafety:           "数据安全",
		SettingsSectionListsViews:           "列表与视图",
		SettingsVersion:                     "版本",
		SettingsVersionLine:                 "trove %s · 上次检查 %s",
		SettingsVersionLineNever:            "trove %s · 安装后尚未检查过",
		SettingsAutoCheck:                   "自动检查更新",
		SettingsAutoCheckHint:               "每天在后台检查一次；发现新版本会打开更新窗口。",
		SettingsGroupAutoTagBasics:          "基础规则",
		SettingsGroupAutoTagLlm:             "大模型标签",
		SettingsLlmEnabled:                  "向模型发送批次",
		SettingsAddKeywordRule:              "添加规则",
		SettingsRemoveKeywordRuleFor:        "移除关键词规则 %s",
		SettingsRuleArrow:                   "\u2192",
		SettingsClearBlockedTags:            "清空屏蔽名单",
		SettingsLanguageHint:                "界面与文案使用的语言；侧边栏底部的切换也可更改。",
		LanguageSwitch:                      "切换语言",
		SettingsThemeHint:                   "默认跟随系统，也可以固定为浅色或深色。",
		SettingsBackupHint:                  "把全部书签、标签与已保存视图导出为 JSON 文件。",
		SettingsAutoBackupRow:               "自动备份",
		SettingsAutoBackupHint:              "每天保留一份副本，最多 7 份，放在书库旁边。",
		SettingsOpenBackupDir:               "打开备份目录",
		SettingsBackupDirHint:               "副本就是普通的数据库文件，可以自己复制或移到别处。",
		SettingsBackupDone:                  "已备份到 %s",
		SettingsBackupFailed:                "自动备份失败：%s",
		SettingsFileLogRow:                  "诊断日志",
		SettingsFileLogHint:                 "把事件和计数写进 logs/trove.log，保留 3 份、每份 1 MB。不写密钥、Cookie 或正文。",
		SettingsFileLogFailed:               "无法打开日志：%s",
		SettingsLegacyBackupHint:            "旧版 exportVersion 2 格式的备份。",
		SettingsRestoreHint:                 "读取备份文件，会覆盖当前本地数据。",
		SettingsLocalHint:                   "清除本地书签、标签与设置，此操作无法撤销。",
		SettingsDataSafetyHint:              "备份与恢复全部在本地完成，不会上传任何数据。侧边栏底部保留了同样的快捷入口。",
		ResetLocalData:                      "重置本地数据",
		ResetLocalDataConfirm:               "重置会清空所有本地书签、标签和设置，确认继续？",
		ResettingData:                       "重置中...",
		LocalDataResetDone:                  "本地数据已重置",
		RestoreBackup:                       "导入恢复",
		RestoringBackup:                     "恢复中...",
		RestoreBackupConfirm:                "导入恢复会覆盖当前本地数据，确认继续？",
		SettingsStopBufferPages:             "增量同步缓冲页数",
		SettingsStopBufferHint:              "增量同步在命中已知书签后继续抓取的页数，越大越保险但越慢。",
		SettingsPageLimit:                   "首次同步书签上限",
		SettingsPageLimitHint:               "一次全量同步最多抓取多少条。0 表示内置的 10,000 条；下次同步会从上次停下的地方继续。",
		SettingsRunAfterSync:                "同步完成后自动分析",
		SettingsXBrowserRead:                "从浏览器读取",
		SettingsXBrowserHint:                "从这台 Mac 上已登录 x.com 的浏览器读取会话，Cookie 保存在钥匙串里。",
		SettingsXTestHint:                   "向 x.com 发一次请求，在同步之前分清是 Cookie 失效还是网络不通。",
		SettingsXBrowserReading:             "正在查找已登录的浏览器…",
		SettingsXBrowserSaved:               "已从浏览器读取并保存到钥匙串。",
		SettingsXBrowserFailed:              "读取浏览器失败：%s",
		SettingsXBrowserNeedsAccess:         "系统拒绝访问浏览器数据。请在系统隐私设置里允许本应用访问，然后重试；或在下方粘贴 Cookie 请求头。",
		SettingsXBrowserNoCookies:           "没有在任何浏览器里找到 X 的 cookie。请先在浏览器里登录 x.com，或在下方粘贴 Cookie 请求头。（%s）",
		SettingsXCookiesTitle:               "X 凭证",
		SettingsXCookiesHint:                "请先在浏览器里登录 x.com。一键读取即可；cookie 保存在本机钥匙串，不会上传。",
		SettingsXCredentialPaste:            "Cookie 或 cURL",
		SettingsXCredentialPasteHint:        "或手工粘贴：开发者工具 > 网络 > 任意请求 > 复制 Cookie 请求头。curl 命令或导出的 cookie 列表也可以，两个值都会从里面读出来。",
		SettingsXCredentialPastePlaceholder: "auth_token=…; ct0=…",
		SettingsXCredentialMissing:          "粘贴的内容里没有 %s。完整的 Cookie 请求头两个都在。",
		SettingsXCredentialNothingFound:     "粘贴的内容里没有 auth_token 和 ct0。请复制 Cookie 请求头，不是响应头。",
		SettingsXBrowserUnchanged:           "浏览器和钥匙串里是同一个会话。如果已过期，请在浏览器里重新登录 x.com。",
		SettingsXCookiesSaved:               "已保存到钥匙串。",
		SettingsXCookiesAbsent:              "还没有保存 X 凭证，同步无法运行。",
		SettingsXTestConnection:             "测试连接",
		SettingsXTestRunning:                "测试中…",
		SettingsXTestOK:                     "凭证可用：X 返回了书签数据。",
		SettingsXTestNoCredentials:          "先连接或保存 cookie，再测试。",
		SettingsXTestNoProbe:                "此构建没有安装连接测试，无法访问 X。",
		SettingsXTestDirect:                 "直连（无代理）",
		SettingsProxyRoute:                  "网络路径：%s",
		SettingsSyncOnLaunch:                "打开应用时同步",
		SettingsSaveRawPayloads:             "保留原始响应",
		SettingsOff:                         "关闭",
		SettingsOn:                          "开启",
		SettingsNewList:                     "新建列表",
		SettingsNewView:                     "新建视图",
		SettingsUnclassifiedHint:            "不属于任何清单的书签。",
		SettingsInboxHint:                   "未整理书签的默认去处，不能重命名或删除。",
		SettingsBackupRow:                   "数据备份",
		SettingsLegacyBackupRow:             "兼容备份",
		SettingsRestoreRow:                  "从备份恢复",
		SettingsLocalRow:                    "本地数据",
		SettingsBlockedTags:                 "被屏蔽的标签",
		SettingsBlockedTagsHint:             "删除过的标签名会记在这里，自动打标不会再建议它们。",
		SettingsClearBlockedTagsDone:        "屏蔽名单已清空。",
		SettingsXBrowserRow:                 "浏览器登录",
		SettingsXTestRow:                    "连接测试",
		SettingsLlmConnectionRow:            "连接测试",
		SettingsLlmTestHint:                 "向服务商发一次请求，在正式分析之前分清是密钥失效还是网络不通。",
		SettingsLlmTestMissingKey:           "请先填写并保存 API Key。",
		LLMErrorUnauthorized:                "鉴权失败：API Key 无效或已过期。",
		LLMErrorForbidden:                   "被服务商拒绝（403），请检查账号权限或区域限制。",
		LLMErrorRateLimit:                   "触发限流（429），请稍后重试。",
		LLMErrorRateLimitLong:               "服务商要求等待约 %d 秒后再试。",
		LLMErrorQuota:                       "额度不足或计费异常，请检查账户余额。",
		LLMErrorBadRequest:                  "请求被拒绝，请检查模型名与地址。",
		LLMErrorUnsupportedParam:            "服务商不支持部分参数，已自动降级。",
		LLMErrorContextLength:               "输入超出模型上下文长度。",
		LLMErrorTimeout:                     "请求超时，请检查网络或更换服务商。",
		LLMErrorNetwork:                     "网络错误，无法连接到该地址。",
		LLMErrorServer:                      "服务商返回服务器错误，请稍后重试。",
		LLMErrorContentFilter:               "内容被服务商的安全策略拦截。",
		LLMErrorInvalidJson:                 "模型没有返回可解析的 JSON。",
		LLMErrorUnknown:                     "连接失败。",
		AutoTagTitle:                        "上次分析结果",
		AutoTagAt:                           "记录时间 %s",
		AutoTagDetail:                       "候选 %d 条，本次分析 %d 条，批次 %d（失败 %d），词表 %d 个，新标签额度 %d。",
		AutoTagTokens:                       "本次约消耗 %d 输入 / %d 输出 token。",
		AutoTagBlocked:                      "已屏蔽 %d 个标签名（删除标签时写入），这些名字不会再被建议。",
		AutoTagBlockedDropped:               "本次有 %d 条建议因命中屏蔽名单被丢弃。",
		AutoTagBudgetDropped:                "本次有 %d 条建议因新标签额度不足被丢弃。",
		OutcomeOk:                           "完成：新增 %d 条建议，待确认 %d 条。",
		OutcomeDisabled:                     "自动标签已关闭，请先启用后再运行分析。",
		OutcomeNoCandidates:                 "没有需要分析的书签：所有书签都已有标签或已被分析过。",
		OutcomeEmptyVocabulary:              "标签库为空，而词表模式是「仅使用现有标签」，模型没有可选标签，本次未发起分析。请先切换词表模式，或手动建几个标签。",
		OutcomeLlmDisabled:                  "仅运行了规则分析，未启用 LLM 智能标签。",
		OutcomeMissingKey:                   "未配置 API Key，已跳过 LLM 分析。",
		OutcomeFailed:                       "LLM 分析失败",
		OutcomeAllBlocked:                   "模型给出了标签，但全部命中屏蔽名单（删除标签时写入），因此本次没有新增建议。清空屏蔽名单后再试。",
		OutcomeBudgetExhausted:              "模型给出的都是新标签，但本次的新标签额度已用完，因此没有新增建议。把词表模式改成「允许创建新标签」，或先确认几个建议把词表建起来。",
		OutcomeTimedOut:                     "本次运行达到时间上限后停止，已保存 %d 条建议，还有 %d 条书签没来得及分析。再运行一次即可继续。",
		OutcomeInterrupted:                  "上次运行被中断（浏览器或后台进程结束），已保留已写入的建议，还有 %d 条书签没分析。已自动继续。",
		OutcomeCancelled:                    "本次运行已被取消，已保存 %d 条建议，还有 %d 条书签没有分析。再运行一次即可继续。",
		OutcomeErrorBudget:                  "模型服务连续三批请求失败，本次运行已提前停止，已保存 %d 条建议。请检查 API Key 和模型名称后重新运行。",
		RunPhaseHeuristics:                  "规则匹配",
		RunPhaseLlm:                         "大模型",
		RunAnalyzing:                        "正在分析",
		RunWaiting:                          "正在等待模型响应",
		RunStalled:                          "已 %d 秒没有心跳，可能已中断",
		RunElapsed:                          "已用 %ds",
		RunRemaining:                        "约剩 %ds",
		RunBudgetRemaining:                  "本轮约剩 %ds",
		RunSuggestionsMade:                  "已新增 %d 条建议",
		RunRecentTags:                       "最近：%s",
		RunStopping:                         "正在停止…",
		RunCancelRequested:                  "已请求停止，当前批次结束后会停下来。",
		RunAlreadyRunning:                   "已有一次分析正在进行，请等待它结束。",
		ConnectionOK:                        "连接成功，%s 已响应。",
		ConnectionFailed:                    "连接失败。",
		ConnectionNoTester:                  "未配置连接测试器。",
		ConnectionTesting:                   "正在测试连接…",
		ThemeLabel:                          "主题",
	},
}
