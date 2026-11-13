// Package i18n holds the interface's text.
//
// Every user-visible string goes through here rather than being written inline, so
// a missing translation is a compile-time key and a test failure rather than a
// stray English word in a Chinese interface.
package i18n

import (
	"fmt"
	"sort"
	"strings"
)

// Locale is a language tag.
type Locale = string

const (
	// ZHCN is Simplified Chinese.
	ZHCN Locale = "zh-CN"
	// EN is English, and the fallback for anything untranslated.
	EN Locale = "en"
)

// AllLocales is the supported set, in display order.
var AllLocales = []Locale{ZHCN, EN}

// DefaultLocale is what a fresh install uses.
//
// It is the system locale when that is supported, and English otherwise, so a
// user in a third language gets readable text rather than empty labels.
const DefaultLocale = EN

// ValidLocale reports whether a stored preference is supported.
func ValidLocale(locale string) bool {
	for _, candidate := range AllLocales {
		if locale == candidate {
			return true
		}
	}
	return false
}

// ResolveLocale picks the locale to use from a preference and the system setting.
func ResolveLocale(preferred, system string) Locale {
	if ValidLocale(preferred) {
		return preferred
	}
	if ValidLocale(system) {
		return system
	}
	return DefaultLocale
}

// Key identifies a message.
type Key = string

// The message keys.
const (
	AppName Key = "app.name"

	MenuFile           Key = "menu.file"
	MenuEdit           Key = "menu.edit"
	MenuView           Key = "menu.view"
	MenuWindow         Key = "menu.window"
	BundleExportTitle  Key = "bundle.exportTitle"
	BundleExportLegacy Key = "bundle.exportLegacy"
	BundleExported     Key = "bundle.exported"
	// The in-flight sentences for the file commands. Each names what is happening so
	// the toolbar line is not empty while work runs off the window's thread.
	BundleChoosingWhere Key = "bundle.choosingWhere"
	BundleExporting     Key = "bundle.exporting"
	BundleReading       Key = "bundle.reading"
	BundleImporting     Key = "bundle.importing"
	BundleImportTitle   Key = "bundle.importTitle"
	BundleRestoreTitle  Key = "bundle.restoreTitle"
	BundleImportBody    Key = "bundle.importBody"
	BundleReport        Key = "bundle.report"
	ActionImport        Key = "action.import"
	ActionRestore       Key = "action.restore"

	MenuExport Key = "menu.export"
	// MenuAddBookmark opens the add form. A page-based client saves from x.com itself and has
	// no such command; this app has no page, so it needs an entry point of its own -- and the
	// sidebar header has no room for one, so it lives in the menu bar instead.
	MenuAddBookmark Key = "menu.addBookmark"
	MenuImport      Key = "menu.import"

	SidebarLibrary        Key = "sidebar.library"
	SidebarAllBookmarks   Key = "sidebar.allBookmarks"
	SidebarInbox          Key = "sidebar.inbox"
	SidebarUnclassified   Key = "sidebar.unclassified"
	SidebarLists          Key = "sidebar.lists"
	SidebarTags           Key = "sidebar.tags"
	SidebarNoLists        Key = "sidebar.noLists"
	SidebarNoTagsCreated  Key = "sidebar.noTagsCreated"
	SidebarNoAuthors      Key = "sidebar.noAuthors"
	SidebarNoAuthorsMatch Key = "sidebar.noAuthorsMatch"
	// SidebarNoTagsMatch is the same answer on the tags tab, which drew nothing at all.
	SidebarNoTagsMatch Key = "sidebar.noTagsMatch"

	ToolbarGraph Key = "toolbar.graph"
	// ViewGrid and ViewList name the toolbar's two icon-only view toggles. The toggles carry
	// no visible text, so these names are the shell's own copy.
	ViewGrid               Key = "view.grid"
	ViewList               Key = "view.list"
	GraphSummary           Key = "graph.summary"
	GraphHidden            Key = "graph.hidden"
	GraphAuthorNodesSuffix Key = "graphAuthorNodesSuffix"
	GraphAuthorsSuffix     Key = "graphAuthorsSuffix"
	GraphEmptyTitle        Key = "graphEmptyTitle"
	// GraphCanvas names the graph's drawing surface for a screen reader: the dots are drawn,
	// not elements, so the canvas is the only thing a reader can land on.
	GraphCanvas Key = "graphCanvas"
	// GraphLegend says what the two marks encode: a dot's size is the tag's bookmarks, a
	// line's weight is how often the two tags appear together. This build's addition -- the
	// source draws both encodings without saying so, and a map without a key is a picture.
	GraphLegend Key = "graphLegend"
	// GraphFocusPanel names the card the focused tag's numbers are shown on, so assistive
	// technology reads the panel as one thing rather than as three loose lines.
	GraphFocusPanel       Key = "graphFocusPanel"
	GraphEmptyDescription Key = "graphEmptyDescription"
	// GraphBuildingTitle stands in for the empty state while the graph is being built:
	// the read walks the whole library, so the first frame after the switch has nothing
	// to draw yet and "no relations" would be a lie.
	GraphBuildingTitle Key = "graphBuildingTitle"
	GraphShowAuthors   Key = "graphShowAuthors"
	GraphClearFocus    Key = "graphClearFocus"
	GraphOpenBookmarks Key = "graphOpenBookmarks"
	TagTableName       Key = "tagTableName"
	TagTableParent     Key = "tagTableParent"
	TagTableBookmarks  Key = "tagTableBookmarks"
	TagTablePending    Key = "tagTablePending"
	TagTableUpdated    Key = "tagTableUpdated"
	// SearchLabel names the magnifier that opens the sidebar's search field. The field is closed
	// by default, so this string is the only thing that says the search exists.
	SearchLabel         Key = "searchLabel"
	BookmarksCountLabel Key = "bookmarksCountLabel"
	TagsCountLabel      Key = "tagsCountLabel"
	ToolbarCount        Key = "toolbar.count"
	// The key is namespaced: a key whose text equals the key cannot be told apart from
	// a missing translation, and an existing test refuses that shape.
	ResultsLabel Key = "toolbar.results"
	// ResultsLoadingMore is the line at the end of the grid while the next page is
	// read. The action it replaces ("load more") is gone: the scroll performs it.
	ResultsLoadingMore Key = "toolbar.loadingMore"
	// ToolbarSearch holds the search-box placeholder. MyGo's SearchField takes no
	// placeholder, so nothing renders it -- but a key holding the string is what the other
	// count measures. The two counts pull in opposite directions here, and this is the side
	// that keeps the wording.
	ToolbarSearch      Key = "searchPlaceholder"
	ActiveFiltersLabel Key = "activeFilters"
	ToolbarFilters     Key = "filters"
	FilterConditions   Key = "filterConditions"
	FilterDescription  Key = "filterDescription"
	FilterHasMedia     Key = "hasMedia"
	FilterUnread       Key = "unread"
	FilterArchived     Key = "archived"
	ToolbarShowingOf   Key = "toolbar.showingOf"
	// ToolbarRefreshing is the quiet note beside the count while a background refresh
	// runs. The page stays on screen, so without it a click on a filter looks like it
	// did nothing until the new page lands.
	ToolbarRefreshing Key = "toolbar.refreshing"
	FilterLongform    Key = "filter.longform"
	FilterClearAll    Key = "filter.clearAll"

	SortTimeline    Key = "sort.timeline"
	SortSavedDesc   Key = "sort.savedDesc"
	SortSavedAsc    Key = "sort.savedAsc"
	SortCreatedDesc Key = "sort.createdDesc"
	SortLikesDesc   Key = "sort.likesDesc"

	LoadingFeedTitle   Key = "loadingFeedTitle"
	LoadFailedTitle    Key = "loadFailedTitle"
	InfoLabel          Key = "infoLabel"
	AddBookmark        Key = "addBookmark"
	AddLinkPlaceholder Key = "addLinkPlaceholder"
	AddTextPlaceholder Key = "addTextPlaceholder"
	AddNotAPostLink    Key = "addNotAPostLink"
	ActionRetry        Key = "retryLoad"
	EmptyLibrary       Key = "empty.library"
	EmptyLibraryDetail Key = "emptyFeedDescription"
	EmptyFilterDetail  Key = "noBookmarksDescription"
	EmptyFilter        Key = "empty.filter"

	InspectorTitle Key = "inspector.title"
	InspectorEmpty Key = "inspector.empty"
	// InspectorPublished is the tooltip on the published date, which the detail draws as a small
	// clock rather than as a labelled row. timelineTitle, savedAt, done and noTagsYet left
	// the catalog with the rail and the footer: the detail has no timeline, no saved date, no
	// Done button (the close control does that) and no sentence saying it has no tags (the tag
	// field says so).
	InspectorPublished Key = "publishedAt"

	ActionSync Key = "action.sync"
	// The scroll-ends capsule's two controls. They are the shell's own copy, in both
	// catalogs.
	ActionScrollTop    Key = "action.scrollTop"
	ActionScrollBottom Key = "action.scrollBottom"
	ActionCancel       Key = "action.cancel"
	ActionDelete       Key = "action.delete"
	ActionRemove       Key = "action.remove"
	// ActionRemoveNamed is ActionRemove with the thing it removes. A row of tag pills ends with
	// one remove control each, so six pills read out as six "Remove"s in a row; a single pill
	// is one button whose text is the tag, so its name carries the tag.
	ActionRemoveNamed Key = "action.removeNamed"
	// ActionOpenNamed, ActionRenameNamed and ActionDeleteNamed are the row actions of a
	// management table: several rows carry the same icon, and a generic name gives a screen
	// reader the same words once per row with nothing saying which row it acts on.
	ActionOpenNamed      Key = "action.openNamed"
	ActionRenameNamed    Key = "action.renameNamed"
	ActionDeleteNamed    Key = "action.deleteNamed"
	ActionSettings       Key = "action.settings"
	ActionAddTag         Key = "action.addTag"
	ActionMoveToList     Key = "action.moveToList"
	ActionReview         Key = "action.reviewSuggestions"
	ActionRunAutotag     Key = "action.runAutotag"
	ActionClearSelection Key = "action.clearSelection"
	BatchSelected        Key = "batch.selected"
	BatchSelectCurrent   Key = "batch.selectCurrent"
	BatchSelectLoaded    Key = "batch.selectLoaded"
	BatchApplyTag        Key = "batch.applyTag"
	BatchTooMany         Key = "batch.tooMany"
	// BatchChooseTag is the empty value of the bulk tag picker, and BatchChooseList is this
	// build's own for the move picker it adds. Both are what the trigger shows before
	// anything is chosen: a picker that draws nothing reads as broken.
	BatchChooseTag  Key = "batch.chooseTag"
	BatchChooseList Key = "batch.chooseList"
	// BatchTagged and BatchMoved are the two bulk writes' result sentences. The bar stays
	// standing after a write, so without them a completed action and a missed press look
	// the same.
	BatchTagged    Key = "batch.tagged"
	BatchTaggedOne Key = "batch.taggedOne"
	BatchMoved     Key = "batch.moved"
	BatchMovedOne  Key = "batch.movedOne"

	ConfirmDeleteBookmarksTitle Key = "confirm.deleteBookmarks.title"
	ConfirmDeleteBookmarksBody  Key = "confirm.deleteBookmarks.body"

	SettingsTitle Key = "settings.title"
	// SettingsSearchPlaceholder and SettingsNoMatch are magpie's "Find a setting" and its
	// no-hit line.
	SettingsSearchPlaceholder       Key = "settings.searchPlaceholder"
	SettingsNoMatch                 Key = "settings.noMatch"
	SettingsBack                    Key = "settings.back"
	SettingsLanguage                Key = "settings.language"
	SettingsData                    Key = "settings.data"
	ExportData                      Key = "exportData"
	ExportingData                   Key = "exporting"
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
	ActionAdd                       Key = "action.add"
	SettingsProvider                Key = "settings.provider"
	SettingsModel                   Key = "settings.model"
	SettingsBaseURL                 Key = "settings.baseUrl"
	SettingsBaseURLWarning          Key = "settings.baseUrlWarning"
	SettingsDeployment              Key = "settings.deployment"
	SettingsDeploymentHint          Key = "settings.deploymentHint"
	SettingsMaxPerRun               Key = "settings.maxPerRun"
	SettingsVocabularyMode          Key = "settings.vocabularyMode"
	SettingsVocabularyOpen          Key = "settings.vocabularyOpen"
	SettingsVocabularyGuarded       Key = "settings.vocabularyGuarded"
	SettingsVocabularyClosed        Key = "settings.vocabularyClosed"
	// The auto-tagging page's cards. They are page structure rather than settings copy,
	// which is why they are not named settings.*: the feature lives in one sheet and has no
	// card headings.
	AutoTagRunHint        Key = "autoTagRunHint"
	AutoTagNoRunsYet      Key = "autoTagNoRunsYet"
	AutoTagBasicsHint     Key = "autoTagBasicsHint"
	AutoTagSources        Key = "autoTagSources"
	AutoTagPacing         Key = "autoTagPacing"
	AutoTagPacingHint     Key = "autoTagPacingHint"
	AutoTagModelHint      Key = "autoTagModelHint"
	SettingsAPIKey        Key = "settings.apiKey"
	SettingsAPIKeyPresent Key = "settings.apiKeyPresent"
	SettingsAPIKeyAbsent  Key = "settings.apiKeyAbsent"
	SettingsAPIKeyMask    Key = "settings.apiKeyMask"
	ActionSave            Key = "action.save"
	ActionTestConnection  Key = "action.testConnection"
	TagManageLabel        Key = "manageTagsLabel"
	TagExitManagement     Key = "exitTagManagement"
	TagSearchPlaceholder  Key = "searchTagsPlaceholder"
	// SearchPlaceholder is the toolbar field's prompt.
	SearchPlaceholder   Key = ToolbarSearch
	TagFilterAll        Key = "tagFilterAll"
	TagFilterOrphan     Key = "tagFilterOrphan"
	TagFilterLongTail   Key = "tagFilterLongTail"
	TagFilterPending    Key = "tagFilterPending"
	TagFilterUnparented Key = "tagFilterUnparented"
	TagSortMost         Key = "tagSortMostBookmarks"
	TagSortFewest       Key = "tagSortFewestBookmarks"
	TagSortName         Key = "tagSortName"
	TagSortRecent       Key = "tagSortRecent"
	TagTagsSuffix       Key = "tagsCountSuffix"
	TagOrphanSuffix     Key = "orphanTagsSuffix"
	TagLongTailSuffix   Key = "longTailTagsSuffix"
	TagUnparentedSuffix Key = "unparentedTagsSuffix"
	TagChildCountSuffix Key = "tagChildCountSuffix"
	TagHealthOk         Key = "tagHealthOk"
	TagHealthTooMany    Key = "tagHealthTooMany"
	TagCleanupOrphans   Key = "cleanupOrphanTagsLabel"
	TagCleanupConfirm   Key = "cleanupOrphanTagsConfirmPrefix"
	TagMergeLabel       Key = "mergeTagsLabel"
	TagDeleteLabel      Key = "deleteTagsLabel"
	TagDeleteFailed     Key = "deleteTagsFailed"
	TagSelectAll        Key = "selectAllTags"
	TagNoTagsCreated    Key = "noTagsCreated"
	// TagLibraryReading stands in for the empty state on the first frame of the page:
	// the counts are read off the window's thread.
	TagLibraryReading     Key = "tagLibraryReading"
	TagNoTagsDescription  Key = "noTagsCreatedDescription"
	TagNoMatchTitle       Key = "noTagsMatchTitle"
	TagNoMatchDescription Key = "noTagsMatchDescription"
	TagBatchSelected      Key = "tagBatchSelected"
	TagAliasesLabel       Key = "tagAliasesLabel"
	TagParentLabel        Key = "tagParentLabel"
	TagParentNone         Key = "tagParentNone"
	TagCleanupDone        Key = "tag.cleanupDone"
	ReviewLabel           Key = "reviewVocabularyLabel"
	ReviewRunning         Key = "reviewVocabularyRunning"
	ReviewTitle           Key = "reviewVocabularyTitle"
	ReviewDescription     Key = "reviewVocabularyDescription"
	ReviewEmpty           Key = "reviewVocabularyEmpty"
	ReviewNoTags          Key = "reviewVocabularyNoTags"
	ReviewFailed          Key = "reviewVocabularyFailedPrefix"
	ReviewRetry           Key = "reviewRetry"
	ReviewMerge           Key = "reviewMergeLabel"
	ReviewRename          Key = "reviewRenameLabel"
	ReviewApply           Key = "reviewApplyLabel"
	ReviewDismiss         Key = "reviewDismissLabel"
	ReviewAppliedCount    Key = "review.appliedCount"
	TagMergeInto          Key = "mergeTargetLabel"
	TagAliasesPlaceholder Key = "tagAliasesPlaceholder"
	TagRenameLabel        Key = "renameTagLabel"
	TagRenameTaken        Key = "renameTagTaken"
	// The tag table's row controls carry their row's name: a table of dozens drew dozens of
	// identical "Rename tag" and "Delete" buttons, so a screen reader could not say which
	// row each belonged to and a test could only ever press the first one.
	TagBookmarksFor      Key = "tag.bookmarksFor"
	TagRenameFor         Key = "tag.renameFor"
	TagAliasesFor        Key = "tag.aliasesFor"
	TagThemeFor          Key = "tag.themeFor"
	TagDeleteFor         Key = "tag.deleteFor"
	TagSelectRowFor      Key = "tag.selectRowFor"
	SidebarNewList       Key = "newListPlaceholder"
	RenameListLabel      Key = "renameListLabel" // this app's own wording; lists are native
	DeleteListConfirm    Key = "deleteListConfirm"
	SuggestionsAcceptAll Key = "acceptAllSuggestions"
	LastSyncLabel        Key = "lastSync"
	// LastSyncNever is the word for a workspace that has never synced. It lives outside
	// the main copy catalog, which is why the catalog comparison never reported it missing.
	LastSyncNever            Key = "syncNever"
	SavedViewsTitle          Key = "savedViewsTitle"
	SavedViewNamePlaceholder Key = "savedViewNamePlaceholder"
	SaveCurrentView          Key = "saveCurrentView"
	DeleteSavedView          Key = "deleteSavedView"
	SavedViewApply           Key = "savedViewApply"
	// SavedViewApplyNamed is the apply button with the view it restores. This build draws an
	// icon, so the view's name has to be in the control's label.
	SavedViewApplyNamed      Key = "savedViewApplyNamed"
	DefaultSavedViewName     Key = "defaultSavedViewName"
	AuthorsTitle             Key = "authorsTitle"
	SearchAuthors            Key = "searchAuthors"
	SearchAuthorsPlaceholder Key = "searchAuthorsPlaceholder"
	DeleteTagConfirm         Key = "deleteTagConfirmPrefix"
	CreateTagLabel           Key = "createTagLabel"
	CreateTagPrompt          Key = "createTagPrompt"
	CreateTagSymbol          Key = "createTagSymbol" // a symbol, not a word: both catalogs hold "+"
	// AuthorAvatarFallback is the mark an author row shows when the author has no picture. It
	// is a literal "@", a symbol, and TestNoEnglishIsLeftInTheChineseCatalogue's letter test
	// lets it through for that reason.
	AuthorAvatarFallback    Key = "authorAvatarFallback"
	DeleteTagsConfirm       Key = "deleteTagsConfirmPrefix"
	SuggestionsPendingTitle Key = "pendingSuggestionsTitle"
	TagMergeNeedTarget      Key = "mergeNeedTarget"
	TagMergeNeedsSource     Key = "mergeNeedsSource"
	TagMergeChoose          Key = "mergeChoose"
	TagMergeConfirmBody     Key = "mergeConfirmBody"
	TagMergeDone            Key = "mergeDone"
	// The two notes the header shows while a library-wide edit runs off the window's
	// thread; the window keeps drawing while they are up.
	TagMerging                Key = "tagMerging"
	ReviewApplying            Key = "reviewApplying"
	SettingsSectionGeneral    Key = "settingsSectionGeneral"
	SettingsSectionSync       Key = "settingsSectionSync"
	SettingsSectionAutoTag    Key = "settingsSectionAutoTag"
	SettingsSectionDataSafety Key = "settingsSectionDataSafety"
	// SettingsSectionListsViews is the tab the lists and the saved views live on. They used
	// to sit at the bottom of Data safety, under a sentence about backups, where nobody
	// looking for their views would think to go.
	SettingsSectionListsViews Key = "settingsSectionListsViews"
	// SettingsAutoTagMoved is the sentence left in Settings after the auto-tagging controls
	// moved into tag management.
	SettingsAutoTagMoved       Key = "settingsAutoTagMoved"
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
	// Each preference row carries its own line, the way magpie's `.row.pref` does: what
	// the setting does, before the control that changes it.
	SettingsLanguageHint     Key = "settingsLanguageHint"
	LanguageSwitch           Key = "sidebarLanguageSwitch"
	SettingsThemeHint        Key = "settingsThemeHint"
	SettingsBackupHint       Key = "settingsBackupHint"
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
	SyncMoreAvailable        Key = "syncMoreAvailable"
	SyncStored               Key = "syncStored"
	SyncNothingNew           Key = "syncNothingNew"
	// SyncProgressLabel is the progress line: "Syncing... {pages} pages · {items} items".
	SyncProgressLabel    Key = "syncProgressLabel"
	SettingsRunAfterSync Key = "settingsAutoTagRunAfterSync"
	// The X credentials a desktop build has to be told about, because it has no browser
	// session of its own.
	SyncNeedsXCredentials Key = "syncNeedsXCredentials"
	SettingsXBrowserRead  Key = "settingsXBrowserRead"
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
	ActionSaveCookies                   Key = "action.saveCookies"
	ActionReconnectX                    Key = "action.reconnectX"
	SyncLookingForBrowser               Key = "syncLookingForBrowser"
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
	// SettingsOff and SettingsOn are the two halves of a settings switch, magpie's
	// Off/On (.opt). The words are magpie's own Chinese: 关闭 / 开启.
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
	AutoTagBlocked            Key = "settingsAutoTagResultBlocked"
	AutoTagBlockedDropped     Key = "settingsAutoTagResultBlockedDropped"
	AutoTagBudgetDropped      Key = "settingsAutoTagResultBudgetDropped"
	OutcomeOk                 Key = "settingsAutoTagResultOk"
	// OutcomeDisabled is the sentence for pressing run while the feature's own switch is
	// off. The fallthrough "Done: 0 new suggestions" reads as a finished run; saying the
	// switch is off is the only reading of the same fact that is true.
	OutcomeDisabled         Key = "settingsAutoTagResultDisabled"
	OutcomeNoCandidates     Key = "settingsAutoTagResultNoCandidates"
	OutcomeEmptyVocabulary  Key = "settingsAutoTagResultEmptyVocabulary"
	OutcomeLlmDisabled      Key = "settingsAutoTagResultLlmDisabled"
	OutcomeMissingKey       Key = "settingsAutoTagResultMissingKey"
	OutcomeFailed           Key = "settingsAutoTagResultFailed"
	OutcomeAllBlocked       Key = "settingsAutoTagResultAllBlocked"
	OutcomeBudgetExhausted  Key = "settingsAutoTagResultBudgetExhausted"
	OutcomeTimedOut         Key = "settingsAutoTagResultTimedOut"
	OutcomeInterrupted      Key = "settingsAutoTagResultInterrupted"
	OutcomeCancelled        Key = "settingsAutoTagResultCancelled"
	RunPhaseHeuristics      Key = "settingsAutoTagPhaseHeuristics"
	RunPhaseLlm             Key = "settingsAutoTagPhaseLlm"
	RunAnalyzing            Key = "settingsAutoTagProgress"
	RunWaiting              Key = "settingsAutoTagWaiting"
	RunStalled              Key = "settingsAutoTagStalled"
	RunElapsed              Key = "settingsAutoTagElapsed"
	RunRemaining            Key = "settingsAutoTagRemaining"
	RunBudgetRemaining      Key = "settingsAutoTagBudgetRemaining"
	RunSuggestionsMade      Key = "settingsAutoTagSuggestionsMade"
	RunRecentTags           Key = "settingsAutoTagRecentTags"
	RunStopping             Key = "settingsAutoTagStopping"
	RunCancelRequested      Key = "settingsAutoTagCancelRequested"
	RunAlreadyRunning       Key = "settingsAutoTagAlreadyRunning"
	ActionRunNow            Key = "settingsRunAutoTagNow"
	ActionStopRun           Key = "settingsAutoTagCancel"
	ActionOpenTagManagement Key = "action.openTagManagement"
	ConnectionOK            Key = "connection.ok"
	ConnectionFailed        Key = "connection.failed"
	ConnectionNoTester      Key = "connection.noTester"
	// ConnectionTesting covers the seconds a provider probe spends on the network.
	ConnectionTesting Key = "connection.testing"
	ActionClear       Key = "action.clear"
	ActionLoadMore    Key = "action.loadMore"
	ActionShowMore    Key = "action.showMore"
	ActionFocusSearch Key = "action.focusSearch"

	AppearanceSystem Key = "appearance.system"
	AppearanceLight  Key = "appearance.light"
	AppearanceDark   Key = "appearance.dark"
	ThemeLabel       Key = "themeLabel"
	AppTitle         Key = "pageTitle"
	ActionCopyLink   Key = "copyLink"
	// ActionCopied is the copy chip's own confirmation, shown in its place for a moment
	// after the link is on the clipboard.
	ActionCopied Key = "copied"
	// The row menu's own words, from magpie's openAgentMenu / openRowMenu.
	ActionOpenBookmark   Key = "action.openBookmark"
	ActionCopyText       Key = "action.copyText"
	ActionAddToSelection Key = "action.addToSelection"
	// ActionRemoveFromSelection is the same row of the menu once the bookmark is ticked.
	ActionRemoveFromSelection Key = "action.removeFromSelection"
	ActionFilterTag           Key = "action.filterTag"
	ActionOnlyTag             Key = "action.onlyTag"
	ActionFilterAuthor        Key = "action.filterAuthor"
	ActionOnlyAuthor          Key = "action.onlyAuthor"
	ActionCopyHandle          Key = "action.copyHandle"
	ActionCollapseSidebar     Key = "action.collapseSidebar"
	ActionExpandSidebar       Key = "action.expandSidebar"
	ActionToggleSidebar       Key = "action.toggleSidebar"
	OpenOnX                   Key = "openOnX"
	MediaPrevious             Key = "previousMedia"
	MediaNext                 Key = "nextMedia"
	MediaClose                Key = "closePreview"
	// MediaPlay is the video poster's play control in the preview, and MediaPlayGif the same
	// control over an animated GIF, which X serves as an mp4 too and which is shown as video.
	MediaPlay    Key = "playMedia"
	MediaPlayGif Key = "playGif"
	// MediaTitle names the preview dialog.
	MediaTitle  Key = "mediaTitle"
	ToastNoLink Key = "noLinkToCopy"
	ToastDone   Key = "done"

	StatusIdle      Key = "status.idle"
	StatusRunning   Key = "status.running"
	StatusSuccess   Key = "status.success"
	StatusPartial   Key = "status.partial"
	StatusCancelled Key = "status.cancelled" // this app's own label for this status
	StatusError     Key = "status.error"

	// The sync failure kinds. They are the reason the shell reports a failure at all: the
	// sidebar's chip says "Error", and these are what go where the explanation belongs.
	SyncErrorAuthExpired   Key = "sync.error.auth_expired"
	SyncErrorRateLimited   Key = "sync.error.rate_limited"
	SyncErrorNetworkError  Key = "sync.error.network_error"
	SyncErrorSchemaChanged Key = "sync.error.x_schema_changed"
	SyncErrorInterrupted   Key = "sync.error.interrupted"
	SyncErrorUnknown       Key = "sync.error.unknown"

	SuggestionsTitle  Key = "suggestions.title"
	SuggestionsEmpty  Key = "suggestions.empty"
	SuggestionsAccept Key = "suggestions.accept"
	SuggestionsReject Key = "suggestions.reject"
	// SuggestionsAcceptFor and SuggestionsRejectFor carry the tag the row proposes. A panel
	// of ten rows drawn with the bare words announces ten identically named buttons.
	SuggestionsAcceptFor Key = "suggestions.acceptFor"
	SuggestionsRejectFor Key = "suggestions.rejectFor"

	// CardSelectBookmark is the card's corner checkbox. The name carries the author, so each
	// box says which post it picks; the bare word alone repeated down a column identifies
	// nothing.
	CardSelectBookmark    Key = "card.selectBookmark"
	CardSelectBookmarkFor Key = "card.selectBookmarkFor"
	// ActionOpenSyncSettings is the next step under a sync failure. The sentence names
	// Settings > Sync; this is the control that goes there.
	ActionOpenSyncSettings Key = "action.openSyncSettings"
)

// catalogs holds every translation.
//
// English is complete by construction: a key missing from another catalog falls
// back to it, so a half-translated locale degrades to English rather than to a
// raw key.
var catalogs = map[Locale]map[Key]string{
	EN: {
		AppName: "trove",

		MenuFile: "File", MenuEdit: "Edit", MenuView: "View", MenuWindow: "Window",
		MenuExport: "Export…", MenuImport: "Import…",
		BundleExportTitle:   "Export backup",
		BundleExportLegacy:  "Export legacy backup",
		BundleExported:      "Backup exported",
		BundleChoosingWhere: "Choosing where to save…",
		BundleExporting:     "Exporting the library…",
		BundleReading:       "Reading the file…",
		BundleImporting:     "Importing…",
		BundleImportTitle:   "Import backup",
		BundleRestoreTitle:  "Restore backup",
		BundleImportBody:    "%d new, %d already saved",
		BundleReport:        "%d added · %d updated · %d tags",
		ActionImport:        "Import",
		ActionRestore:       "Restore",
		MenuAddBookmark:     "Add Bookmark…",

		SidebarLibrary:        "Library",
		SidebarAllBookmarks:   "All bookmarks",
		SidebarInbox:          "Inbox",
		SidebarUnclassified:   "Unclassified",
		SidebarLists:          "Lists",
		SidebarTags:           "Tags",
		SidebarNoLists:        "No lists yet",
		SidebarNoTagsCreated:  "No tags created yet.",
		SidebarNoAuthors:      "No authors yet.",
		SidebarNoAuthorsMatch: "No authors match your search.",
		SidebarNoTagsMatch:    "No tags match your search.",

		ToolbarGraph:           "Graph",
		ViewGrid:               "Grid view",
		ViewList:               "List view",
		GraphSummary:           "%d tags \u00b7 %d links",
		GraphHidden:            "%d hidden",
		GraphAuthorNodesSuffix: "author nodes",
		GraphAuthorsSuffix:     "authors",
		GraphEmptyTitle:        "No relations to show yet",
		GraphCanvas:            "Tag relation graph",
		GraphLegend:            "A bigger dot is a tag with more bookmarks · a heavier line is a pair that appears together more often",
		GraphFocusPanel:        "Focused tag",
		GraphEmptyDescription:  "Tag a few bookmarks, or widen the current filters.",
		GraphBuildingTitle:     "Building the relation graph…",
		GraphShowAuthors:       "Show authors",
		GraphClearFocus:        "Clear focus",
		GraphOpenBookmarks:     "View bookmarks",
		TagTableName:           "Tag",
		TagTableParent:         "Theme",
		TagTableBookmarks:      "Bookmarks",
		TagTablePending:        "Pending",
		TagTableUpdated:        "Updated",
		SearchLabel:            "Search",
		BookmarksCountLabel:    "bookmarks",
		TagsCountLabel:         "tags",
		ToolbarCount:           "%d",
		ResultsLabel:           "results",
		ResultsLoadingMore:     "Loading more...",
		ToolbarSearch:          "Search bookmarks, authors and notes...",
		ActiveFiltersLabel:     "Active filters:",
		ToolbarFilters:         "Filters",
		FilterConditions:       "Filter conditions",
		FilterDescription:      "Choose the filters to apply.",
		FilterHasMedia:         "Has media",
		FilterLongform:         "Longform",
		FilterUnread:           "Unread",
		FilterArchived:         "Archived",
		ToolbarShowingOf:       "%d / %d",
		ToolbarRefreshing:      "Updating…",
		FilterClearAll:         "Clear all",

		SortTimeline:    "Recently saved",
		SortSavedDesc:   "Recently saved",
		SortSavedAsc:    "Oldest saved",
		SortCreatedDesc: "Newest published",
		SortLikesDesc:   "Most liked",

		LoadingFeedTitle:   "Loading your bookmark feed",
		LoadFailedTitle:    "Failed to load bookmarks",
		InfoLabel:          "Info",
		AddBookmark:        "Add bookmark",
		AddLinkPlaceholder: "Paste a link to the post",
		AddTextPlaceholder: "Post text (optional)",
		AddNotAPostLink:    "That is not a link to an X post.",
		ActionRetry:        "Retry load",
		EmptyLibrary:       "No bookmarks saved yet",
		EmptyLibraryDetail: "Once you save items from X, they will appear here in the current card feed layout.",
		EmptyFilterDetail:  "Try adjusting your search, tags, or author scope.",
		EmptyFilter:        "No bookmarks match the current filters",

		InspectorTitle:     "Details",
		InspectorEmpty:     "Select a bookmark to view details",
		InspectorPublished: "Published",

		ActionSync:           "Sync now",
		ActionScrollTop:      "Scroll to top",
		ActionScrollBottom:   "Scroll to bottom",
		ActionCancel:         "Cancel",
		ActionDelete:         "Delete",
		ActionRemove:         "Remove",
		ActionRemoveNamed:    "Remove %s",
		ActionOpenNamed:      "Open %s",
		ActionRenameNamed:    "Rename %s",
		ActionDeleteNamed:    "Delete %s",
		ActionSettings:       "Settings",
		ActionAddTag:         "Add tag",
		ActionMoveToList:     "Move to list",
		ActionReview:         "Tidy vocabulary",
		ActionRunAutotag:     "Run auto-tagging",
		ActionClearSelection: "Clear selection",
		BatchSelected:        "%d selected",
		BatchSelectCurrent:   "Select current results",
		BatchSelectLoaded:    "Select loaded",
		BatchApplyTag:        "Apply tag",
		BatchChooseTag:       "Choose tag",
		BatchChooseList:      "Choose list",
		BatchTooMany:         "Too many results to select at once",
		BatchTagged:          "Tagged %d bookmarks",
		BatchTaggedOne:       "Tagged 1 bookmark",
		BatchMoved:           "Moved %d bookmarks to %s",
		BatchMovedOne:        "Moved 1 bookmark to %s",

		ConfirmDeleteBookmarksTitle: "Delete these bookmarks?",
		ConfirmDeleteBookmarksBody:  "This removes %d bookmark(s) from your library. Your X account is not affected.",

		SettingsTitle:                   "Settings",
		SettingsSearchPlaceholder:       "Find a setting",
		SettingsNoMatch:                 "No setting matches \u201c%s\u201d",
		SettingsBack:                    "Back to bookmarks",
		SettingsLanguage:                "Language",
		SettingsLanguageHint:            "The language of the interface and its copy; the sidebar footer switches it too.",
		LanguageSwitch:                  "Switch language",
		SettingsThemeHint:               "Follows the system unless you pick light or dark.",
		SettingsBackupHint:              "Every bookmark, tag and saved view, as a JSON file.",
		SettingsLegacyBackupHint:        "A backup in the legacy exportVersion 2 format.",
		SettingsRestoreHint:             "Read a backup file; this overwrites the local data.",
		SettingsLocalHint:               "Clear the local bookmarks, tags and settings. This cannot be undone.",
		SettingsData:                    "Data",
		ExportData:                      "Export data",
		ExportingData:                   "Exporting...",
		SettingsAutoTagEnabled:          "Enable auto-tagging",
		SettingsAutoTagEnabledHint:      "Generates tag suggestions for untagged bookmarks after sync. Nothing is applied until you accept.",
		SettingsAutoTagKeywordRulesHint: "Optional. A post whose text contains the keyword is suggested that tag without calling the model; matching ignores case. For example: sourdough → Baking.",
		SettingsAutoTagVocabularyFull:   "The tag library reached its healthy size, so this run cannot mint new tags.",
		SettingsAutoContinue:            "Continue automatically when work is left",
		SettingsBatchSize:               "Bookmarks per request",
		SettingsConcurrency:             "Parallel requests (1–4)",
		SettingsConcurrencyHint:         "More parallel requests can trigger rate limits.",
		SettingsUseHashtags:             "Use #hashtags from tweets",
		SettingsUseAuthors:              "Suggest tags from author history",
		SettingsKeywordRules:            "Keyword rules",
		SettingsKeywordRulesEmpty:       "No keyword rules yet. Add one below if a few words should always point at the same tag; this is optional and needs no model.",
		SettingsKeywordPlaceholder:      "keyword",
		SettingsRuleTagPlaceholder:      "tag",
		SettingsProvider:                "Provider",
		SettingsModel:                   "Model",
		SettingsBaseURL:                 "Endpoint (blank uses the provider's)",
		SettingsBaseURLWarning:          "Your API key is sent to this endpoint.",
		SettingsDeployment:              "Deployment name",
		SettingsDeploymentHint:          "Azure OpenAI only: the deployment name from the Azure portal.",
		SettingsMaxPerRun:               "Max bookmarks per run",
		SettingsVocabularyMode:          "Tag vocabulary",
		SettingsVocabularyOpen:          "Allow new tags",
		SettingsVocabularyGuarded:       "Limited new tags",
		SettingsVocabularyClosed:        "Existing tags only",
		AutoTagRunHint:                  "Suggests tags for untagged bookmarks. Nothing is written until you accept.",
		AutoTagNoRunsYet:                "No runs yet. Analyze now proposes the first tags.",
		AutoTagBasicsHint:               "When a pass starts, and which signals its suggestions come from.",
		AutoTagSources:                  "Sources",
		AutoTagPacing:                   "Request pacing",
		AutoTagPacingHint:               "How many bookmarks go in one request, and how many requests run at once.",
		AutoTagModelHint:                "Sends batches to a model to propose tags. With this off, only rules and author history are used.",
		SettingsAPIKey:                  "API key",
		SettingsAPIKeyPresent:           "A key is stored in the Keychain.",
		SettingsAPIKeyAbsent:            "No key stored.",
		SettingsAPIKeyMask:              "••••••••••••",
		ActionTestConnection:            "Test connection",
		TagManageLabel:                  "Manage tags",
		TagExitManagement:               "Back to bookmarks",
		TagSearchPlaceholder:            "Search tag names or aliases...",
		TagFilterAll:                    "All tags",
		TagFilterOrphan:                 "Orphan tags only",
		TagFilterLongTail:               "Long-tail tags (<=2)",
		TagFilterPending:                "With pending suggestions",
		TagFilterUnparented:             "Without a theme",
		TagSortMost:                     "Most bookmarks",
		TagSortFewest:                   "Fewest bookmarks",
		TagSortName:                     "Name A-Z",
		TagSortRecent:                   "Recently updated",
		TagTagsSuffix:                   "tags",
		TagOrphanSuffix:                 "orphan tags",
		TagLongTailSuffix:               "long-tail tags",
		TagUnparentedSuffix:             "without a theme",
		TagChildCountSuffix:             "child tags",
		TagHealthOk:                     "Library size is healthy",
		TagHealthTooMany:                "Too many tags, review the vocabulary",
		TagCleanupOrphans:               "Delete tags without bookmarks",
		TagCleanupConfirm:               "Delete tags without bookmarks",
		TagMergeLabel:                   "Merge tags",
		TagDeleteLabel:                  "Delete selected tags",
		TagDeleteFailed:                 "%d of %d tags could not be deleted",
		TagSelectAll:                    "Select listed tags",
		TagNoTagsCreated:                "No tags yet",
		TagLibraryReading:               "Reading tags…",
		TagNoTagsDescription:            "Tags appear here after a sync or auto-tagging, or add one yourself.",
		TagNoMatchTitle:                 "No tags match",
		TagNoMatchDescription:           "Try a different search term or filter.",
		TagBatchSelected:                "%d tags selected",
		TagAliasesLabel:                 "Edit aliases",
		TagParentLabel:                  "Theme",
		TagParentNone:                   "No theme",
		TagCleanupDone:                  "Removed %d empty tags.",
		ReviewLabel:                     "Tidy vocabulary",
		ReviewRunning:                   "Reviewing…",
		ReviewTitle:                     "Vocabulary suggestions",
		ReviewDescription:               "Compares the whole tag list and proposes duplicates to merge and names to make clearer. Only tag names are sent to the model, not bookmarks. Pick what you want, then apply.",
		ReviewEmpty:                     "The tag list has no duplicates or unclear names, so there is nothing to merge or rename.",
		ReviewNoTags:                    "The tag list is empty, so there is nothing to merge or rename yet. Accept a few suggestions or create tags first.",
		ReviewFailed:                    "Review failed",
		ReviewRetry:                     "Try again",
		ReviewMerge:                     "Merge",
		ReviewRename:                    "Rename",
		ReviewApply:                     "Apply selected",
		ReviewDismiss:                   "Dismiss",
		ReviewAppliedCount:              "Applied %d change(s).",
		TagMergeInto:                    "Merge into",
		TagAliasesPlaceholder:           "Separate aliases with commas",
		TagRenameLabel:                  "Rename tag",
		TagRenameTaken:                  "“%s” is already used by another tag.",
		TagBookmarksFor:                 "View bookmarks of %s",
		TagRenameFor:                    "Rename tag %s",
		TagAliasesFor:                   "Edit aliases of %s",
		TagThemeFor:                     "Theme of %s",
		TagDeleteFor:                    "Delete tag %s",
		TagSelectRowFor:                 "Select tag %s",
		// "List name", not "View name": this field creates a list, and it used to share the
		// saved-view field's wording -- two different things asking for the same word, so a
		// reader could not tell which row they were typing into.
		SidebarNewList:                      "List name",
		RenameListLabel:                     "Rename list",
		DeleteListConfirm:                   "Reset will clear all local bookmarks, tags, and settings. Continue?",
		SuggestionsAcceptAll:                "Accept all suggestions",
		LastSyncLabel:                       "Last sync",
		LastSyncNever:                       "Not synced yet",
		SavedViewsTitle:                     "Saved views",
		SavedViewNamePlaceholder:            "View name",
		SaveCurrentView:                     "Save current view",
		DeleteSavedView:                     "Delete view",
		SavedViewApply:                      "Apply view",
		SavedViewApplyNamed:                 "Apply view %s",
		DefaultSavedViewName:                "Saved view",
		AuthorsTitle:                        "Authors",
		SearchAuthors:                       "Search authors",
		SearchAuthorsPlaceholder:            "Search authors...",
		DeleteTagConfirm:                    "Delete tag",
		CreateTagLabel:                      "Create tag",
		CreateTagPrompt:                     "Enter a new tag name",
		CreateTagSymbol:                     "+",
		AuthorAvatarFallback:                "@",
		DeleteTagsConfirm:                   "Delete selected tags",
		SuggestionsPendingTitle:             "Pending tags",
		TagMergeNeedTarget:                  "Pick a tag to merge into.",
		TagMergeNeedsSource:                 "Pick at least one other tag to merge.",
		TagMergeChoose:                      "Choose tag",
		TagMergeConfirmBody:                 "Merge %d tag(s) into “%s”? The merged tags are removed.",
		TagMergeDone:                        "Merged %d tag(s).",
		TagMerging:                          "Merging tags…",
		ReviewApplying:                      "Applying the review…",
		SettingsSectionGeneral:              "General",
		SettingsSectionSync:                 "Sync",
		SettingsSectionAutoTag:              "Auto-tagging",
		SettingsSectionDataSafety:           "Data safety",
		SettingsSectionListsViews:           "Lists & views",
		SettingsAutoTagMoved:                "Auto-tagging and its model settings now live in tag management.",
		SettingsGroupAutoTagBasics:          "Rule-based tagging",
		SettingsGroupAutoTagLlm:             "LLM tagging",
		SettingsLlmEnabled:                  "Send batches to a model",
		SettingsAddKeywordRule:              "Add rule",
		SettingsRemoveKeywordRuleFor:        "Remove keyword rule %s",
		SettingsRuleArrow:                   "→",
		SettingsClearBlockedTags:            "Clear block list",
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
		SyncMoreAvailable:                   "Older bookmarks are waiting — sync again to continue.",
		SyncStored:                          "Synced %d new bookmark(s).",
		SyncNothingNew:                      "Sync complete. No new bookmarks.",
		SyncProgressLabel:                   "Syncing… {pages} pages · {items} items",
		SettingsRunAfterSync:                "Run automatically after sync",
		SyncNeedsXCredentials:               "Sync needs an X session. Reconnect from the browser you are signed in to, or add the cookies under Settings > Sync.",
		SettingsXBrowserRead:                "Read from browser",
		SettingsXBrowserReading:             "Looking for a signed-in browser…",
		SettingsXBrowserSaved:               "Read from the browser and saved to the Keychain.",
		SettingsXBrowserUnchanged:           "The browser and the Keychain hold the same cookies. Sign in to x.com in the browser if the session has expired.",
		SettingsXBrowserFailed:              "Could not read the browser: %s",
		SettingsXBrowserHint:                "Take the session from a browser signed in to x.com on this Mac; the cookies are kept in the Keychain.",
		SettingsXTestHint:                   "One request to x.com, so a bad cookie and an unreachable network are told apart before a sync.",
		SettingsXBrowserNeedsAccess:         "The system is blocking access to the browser profiles. Allow this app in your privacy settings, then try again -- or paste the Cookie header below.",
		SettingsXBrowserNoCookies:           "No X cookies found in any browser. Sign in to x.com in your browser first, or paste the Cookie header below. (%s)",
		SettingsXCookiesTitle:               "X credentials",
		SettingsXCookiesHint:                "Sign in to x.com in a browser first. Reading that session is one click; the cookies are kept in this Mac's Keychain and never leave it.",
		SettingsXCredentialPaste:            "Cookie or cURL",
		SettingsXCredentialPasteHint:        "Or paste it by hand: DevTools > Network > any request > copy the Cookie request header. A curl command or an exported cookie list works too; both values are read out of it.",
		SettingsXCredentialPastePlaceholder: "auth_token=…; ct0=…",
		SettingsXCredentialMissing:          "That paste has no %s. The whole Cookie request header carries both.",
		SettingsXCredentialNothingFound:     "No auth_token or ct0 in that paste. Copy the Cookie request header, not the response.",
		SettingsXCookiesSaved:               "Saved to the Keychain.",
		SettingsXCookiesAbsent:              "No X credentials stored yet, so sync cannot run.",
		ActionSaveCookies:                   "Save credentials",
		ActionReconnectX:                    "Reconnect from browser",
		SyncLookingForBrowser:               "Looking for a signed-in browser…",
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
		ActionRunNow:                        "Analyze now",
		ActionStopRun:                       "Stop this run",
		ActionOpenTagManagement:             "Open tag management",
		ConnectionOK:                        "Connected. %s answered.",
		ConnectionFailed:                    "The connection failed.",
		ConnectionNoTester:                  "No connection tester is configured.",
		ConnectionTesting:                   "Testing the connection…",
		ActionSave:                          "Save",
		ActionClear:                         "Clear",
		ActionLoadMore:                      "Load more",
		ActionShowMore:                      "Show more",
		ActionAdd:                           "Add",
		ActionFocusSearch:                   "Search",

		AppearanceSystem:          "System",
		AppearanceLight:           "Light",
		AppearanceDark:            "Dark",
		ThemeLabel:                "Theme",
		AppTitle:                  "Bookmarks",
		ActionCopyLink:            "Copy link",
		ActionCopied:              "Copied",
		ActionOpenBookmark:        "Open bookmark",
		ActionCopyText:            "Copy text",
		ActionAddToSelection:      "Add to selection",
		ActionRemoveFromSelection: "Remove from selection",
		ActionFilterTag:           "Filter by this tag",
		ActionOnlyTag:             "Only this tag",
		ActionFilterAuthor:        "Filter by this author",
		ActionOnlyAuthor:          "Only this author",
		ActionCopyHandle:          "Copy @handle",
		ActionCollapseSidebar:     "Hide sidebar",
		ActionExpandSidebar:       "Show sidebar",
		ActionToggleSidebar:       "Show/Hide Sidebar",
		OpenOnX:                   "Open on X",
		MediaPrevious:             "Previous media",
		MediaNext:                 "Next media",
		MediaClose:                "Close preview",
		MediaPlay:                 "Play video",
		MediaPlayGif:              "Play GIF",
		MediaTitle:                "Media",
		ToastNoLink:               "No link available",
		ToastDone:                 "Done",

		StatusIdle:      "Idle",
		StatusRunning:   "Syncing",
		StatusSuccess:   "Success",
		StatusPartial:   "Partial success",
		StatusCancelled: "Cancelled",
		StatusError:     "Error",
		// The English sync error sentences.
		SyncErrorAuthExpired:   "X login expired. Sign in to X again, then sync.",
		SyncErrorRateLimited:   "X is rate limiting sync requests. Try again later.",
		SyncErrorNetworkError:  "Network request failed. Check your connection, then retry.",
		SyncErrorSchemaChanged: "X page or API structure changed. Update the app, then sync.",
		SyncErrorInterrupted:   "The last sync was interrupted (the browser recycled the background worker). Sync again.",
		SyncErrorUnknown:       "Sync failed. Please retry.",

		SuggestionsTitle:     "Suggestions",
		SuggestionsEmpty:     "Nothing to review.",
		SuggestionsAccept:    "Accept suggestion",
		SuggestionsReject:    "Reject suggestion",
		SuggestionsAcceptFor: "Accept suggestion %s",
		SuggestionsRejectFor: "Reject suggestion %s",

		CardSelectBookmark:     "Select bookmark",
		CardSelectBookmarkFor:  "Select bookmark by %s",
		ActionOpenSyncSettings: "Open sync settings",
	},

	ZHCN: {
		AppName: "trove",

		MenuFile: "文件", MenuEdit: "编辑", MenuView: "显示", MenuWindow: "窗口",
		MenuExport: "导出…", MenuImport: "导入…",
		BundleExportTitle:   "导出备份",
		BundleExportLegacy:  "导出兼容备份",
		BundleExported:      "备份已导出",
		BundleChoosingWhere: "正在选择保存位置…",
		BundleExporting:     "正在导出书库…",
		BundleReading:       "正在读取文件…",
		BundleImporting:     "正在导入…",
		BundleImportTitle:   "导入备份",
		BundleRestoreTitle:  "恢复备份",
		BundleImportBody:    "%d 条新增，%d 条已存在",
		BundleReport:        "新增 %d 条 · 更新 %d 条 · 标签 %d 个",
		ActionImport:        "导入",
		ActionRestore:       "恢复",
		MenuAddBookmark:     "添加书签…",

		SidebarLibrary:        "资料库",
		SidebarAllBookmarks:   "全部书签",
		SidebarInbox:          "收件箱",
		SidebarUnclassified:   "未分类",
		SidebarLists:          "列表",
		SidebarTags:           "标签",
		SidebarNoLists:        "还没有列表",
		SidebarNoTagsCreated:  "还没有创建任何标签。",
		SidebarNoAuthors:      "还没有作者。",
		SidebarNoAuthorsMatch: "没有匹配的作者。",
		SidebarNoTagsMatch:    "没有匹配的标签。",

		ToolbarGraph:           "\u5173\u7cfb\u56fe",
		ViewGrid:               "\u7f51\u683c\u89c6\u56fe",
		ViewList:               "\u5217\u8868\u89c6\u56fe",
		GraphSummary:           "%d 个标签 \u00b7 %d 条关联",
		GraphHidden:            "\u5df2\u9690\u85cf %d \u4e2a",
		GraphAuthorNodesSuffix: "位作者节点",
		GraphAuthorsSuffix:     "位作者",
		GraphEmptyTitle:        "暂无可展示的关联",
		GraphCanvas:            "标签关系图",
		GraphLegend:            "点越大，标签下的书签越多 · 连线越粗，两个标签一起出现得越多",
		GraphFocusPanel:        "聚焦的标签",
		GraphEmptyDescription:  "先给书签打上标签，或放宽当前的筛选条件。",
		GraphBuildingTitle:     "正在构建关系图…",
		GraphShowAuthors:       "显示作者",
		GraphClearFocus:        "取消聚焦",
		GraphOpenBookmarks:     "查看该标签的书签",
		TagTableName:           "标签",
		TagTableParent:         "所属主题",
		TagTableBookmarks:      "书签数",
		TagTablePending:        "待确认",
		TagTableUpdated:        "更新时间",
		SearchLabel:            "搜索",
		BookmarksCountLabel:    "书签",
		TagsCountLabel:         "标签",
		ToolbarCount:           "%d",
		ResultsLabel:           "结果",
		ResultsLoadingMore:     "正在加载更多...",
		ToolbarSearch:          "搜索书签、作者和备注...",
		ActiveFiltersLabel:     "活跃筛选:",
		ToolbarFilters:         "筛选",
		FilterConditions:       "筛选条件",
		FilterDescription:      "选择要应用的过滤条件。",
		FilterHasMedia:         "包含媒体",
		FilterUnread:           "未读",
		FilterArchived:         "已归档",
		ToolbarShowingOf:       "%d / %d",
		ToolbarRefreshing:      "更新中…",
		FilterLongform:         "长文",
		FilterClearAll:         "清除全部",

		SortTimeline:    "最近保存",
		SortSavedDesc:   "最近保存",
		SortSavedAsc:    "最早保存",
		SortCreatedDesc: "最近发布",
		SortLikesDesc:   "最多喜欢",

		LoadingFeedTitle:   "正在整理你的书签",
		LoadFailedTitle:    "书签加载失败",
		InfoLabel:          "信息",
		AddBookmark:        "添加书签",
		AddLinkPlaceholder: "粘贴帖子链接",
		AddTextPlaceholder: "帖子正文（可选）",
		AddNotAPostLink:    "这不是一条 X 帖子的链接。",
		ActionRetry:        "重新加载",
		EmptyLibrary:       "还没有保存任何书签",
		EmptyLibraryDetail: "从 X 保存内容后，书签会以当前卡片流布局出现在这里。",
		EmptyFilterDetail:  "试试调整搜索词、标签或作者范围。",
		EmptyFilter:        "当前筛选条件下没有匹配的书签",

		InspectorTitle:     "详情",
		InspectorEmpty:     "选择一个书签以查看详情",
		InspectorPublished: "发布于",

		ActionSync:           "立即同步",
		ActionScrollTop:      "回到顶部",
		ActionScrollBottom:   "回到底部",
		ActionCancel:         "取消",
		ActionDelete:         "删除",
		ActionRemove:         "移除",
		ActionRemoveNamed:    "移除 %s",
		ActionOpenNamed:      "打开 %s",
		ActionRenameNamed:    "重命名 %s",
		ActionDeleteNamed:    "删除 %s",
		ActionSettings:       "设置",
		ActionAddTag:         "添加标签",
		ActionMoveToList:     "移动到列表",
		ActionReview:         "整理词表",
		ActionRunAutotag:     "运行自动打标",
		ActionClearSelection: "取消选择",
		BatchSelected:        "已选择 %d 条",
		BatchSelectCurrent:   "选择当前结果",
		BatchSelectLoaded:    "选择已加载",
		BatchApplyTag:        "应用标签",
		BatchChooseTag:       "选择标签",
		BatchChooseList:      "选择列表",
		BatchTooMany:         "结果太多，无法一次全选",
		BatchTagged:          "已为 %d 条书签添加标签",
		BatchTaggedOne:       "已为 1 条书签添加标签",
		BatchMoved:           "已将 %d 条书签移动到 %s",
		BatchMovedOne:        "已将 1 条书签移动到 %s",

		ConfirmDeleteBookmarksTitle: "删除这些书签？",
		ConfirmDeleteBookmarksBody:  "将从书库中移除 %d 条书签。你的 X 账号不受影响。",

		SettingsTitle:                       "设置",
		SettingsSearchPlaceholder:           "搜索设置",
		SettingsNoMatch:                     "没有匹配的设置：\u201c%s\u201d",
		SettingsBack:                        "返回书签",
		SettingsLanguage:                    "语言",
		SettingsLanguageHint:                "界面与文案使用的语言；侧边栏底部的切换也可更改。",
		LanguageSwitch:                      "切换语言",
		SettingsThemeHint:                   "默认跟随系统，也可以固定为浅色或深色。",
		SettingsBackupHint:                  "把全部书签、标签与已保存视图导出为 JSON 文件。",
		SettingsLegacyBackupHint:            "旧版 exportVersion 2 格式的备份。",
		SettingsRestoreHint:                 "读取备份文件，会覆盖当前本地数据。",
		SettingsLocalHint:                   "清除本地书签、标签与设置，此操作无法撤销。",
		SettingsData:                        "数据",
		ExportData:                          "导出数据",
		ExportingData:                       "导出中...",
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
		ActionTestConnection:                "测试连接",
		TagManageLabel:                      "管理标签",
		TagExitManagement:                   "返回书签",
		TagSearchPlaceholder:                "搜索标签名或别名...",
		TagFilterAll:                        "全部标签",
		TagFilterOrphan:                     "仅孤儿标签",
		TagFilterLongTail:                   "长尾标签（≤2 条）",
		TagFilterPending:                    "有待确认建议",
		TagFilterUnparented:                 "未归主题",
		TagSortMost:                         "书签最多",
		TagSortFewest:                       "书签最少",
		TagSortName:                         "名称 A-Z",
		TagSortRecent:                       "最近更新",
		TagTagsSuffix:                       "个标签",
		TagOrphanSuffix:                     "个孤儿标签",
		TagLongTailSuffix:                   "个长尾标签",
		TagUnparentedSuffix:                 "个未归主题",
		TagChildCountSuffix:                 "个子标签",
		TagHealthOk:                         "标签规模健康",
		TagHealthTooMany:                    "标签偏多，建议整理词表",
		TagCleanupOrphans:                   "确认删除没有书签的标签",
		TagCleanupConfirm:                   "确认删除没有书签的标签",
		TagMergeLabel:                       "合并标签",
		TagDeleteLabel:                      "删除选中标签",
		TagDeleteFailed:                     "有 %d/%d 个标签删除失败",
		TagSelectAll:                        "选择当前标签",
		TagNoTagsCreated:                    "还没有标签",
		TagLibraryReading:                   "正在读取标签…",
		TagNoTagsDescription:                "同步或自动标签生成的标签会出现在这里，也可以手动创建。",
		TagNoMatchTitle:                     "没有匹配的标签",
		TagNoMatchDescription:               "试试调整搜索词或筛选条件。",
		TagBatchSelected:                    "已选择 %d 个标签",
		TagAliasesLabel:                     "编辑别名",
		TagParentLabel:                      "所属主题",
		TagParentNone:                       "无主题",
		TagCleanupDone:                      "已清理 %d 个空标签。",
		ReviewLabel:                         "整理词表",
		ReviewRunning:                       "分析中…",
		ReviewTitle:                         "词表整理建议",
		ReviewDescription:                   "比较全部标签，找出可以合并的重复标签，以及可以改得更清楚的名字；只会把标签名发给模型，不会发送书签内容。勾选后再应用。",
		ReviewEmpty:                         "标签列表里没有重复或含糊的名字，暂时不需要合并或重命名。",
		ReviewNoTags:                        "词表还是空的，暂时没有可合并或重命名的标签。先确认几条建议或手动创建标签，再来整理。",
		ReviewFailed:                        "整理失败",
		ReviewRetry:                         "重试",
		ReviewMerge:                         "合并",
		ReviewRename:                        "重命名",
		ReviewApply:                         "应用所选",
		ReviewDismiss:                       "关闭建议",
		ReviewAppliedCount:                  "已应用 %d 项变更。",
		TagMergeInto:                        "合并到",
		TagAliasesPlaceholder:               "用逗号分隔多个别名",
		TagRenameLabel:                      "重命名标签",
		TagRenameTaken:                      "“%s”已被另一个标签使用。",
		TagBookmarksFor:                     "查看 %s 的书签",
		TagRenameFor:                        "重命名标签 %s",
		TagAliasesFor:                       "编辑 %s 的别名",
		TagThemeFor:                         "%s 的主题",
		TagDeleteFor:                        "删除标签 %s",
		TagSelectRowFor:                     "选择标签 %s",
		SidebarNewList:                      "列表名称",
		RenameListLabel:                     "重命名列表",
		DeleteListConfirm:                   "重置会清空所有本地书签、标签和设置，确认继续？",
		SuggestionsAcceptAll:                "接受全部建议",
		LastSyncLabel:                       "上次同步",
		LastSyncNever:                       "尚未同步",
		SavedViewsTitle:                     "保存视图",
		SavedViewNamePlaceholder:            "视图名称",
		SaveCurrentView:                     "保存当前视图",
		DeleteSavedView:                     "删除视图",
		SavedViewApply:                      "应用视图",
		SavedViewApplyNamed:                 "应用视图 %s",
		DefaultSavedViewName:                "保存视图",
		AuthorsTitle:                        "作者",
		SearchAuthors:                       "搜索作者",
		SearchAuthorsPlaceholder:            "搜索作者...",
		DeleteTagConfirm:                    "确认删除标签",
		CreateTagLabel:                      "创建标签",
		CreateTagPrompt:                     "输入新标签名称",
		CreateTagSymbol:                     "+",
		AuthorAvatarFallback:                "@",
		DeleteTagsConfirm:                   "确认删除选中的标签",
		SuggestionsPendingTitle:             "待确认标签",
		TagMergeNeedTarget:                  "请选择合并到的目标标签。",
		TagMergeNeedsSource:                 "请再选一个要合并的标签。",
		TagMergeChoose:                      "选择标签",
		TagMergeConfirmBody:                 "把 %d 个标签合并到“%s”？被合并的标签会被删除。",
		TagMergeDone:                        "已合并 %d 个标签。",
		TagMerging:                          "正在合并标签…",
		ReviewApplying:                      "正在应用整理建议…",
		SettingsSectionGeneral:              "通用",
		SettingsSectionSync:                 "同步",
		SettingsSectionAutoTag:              "自动标签",
		SettingsSectionDataSafety:           "数据安全",
		SettingsSectionListsViews:           "列表与视图",
		SettingsAutoTagMoved:                "自动标签与模型配置已移到标签管理。",
		SettingsGroupAutoTagBasics:          "基础规则",
		SettingsGroupAutoTagLlm:             "大模型标签",
		SettingsLlmEnabled:                  "向模型发送批次",
		SettingsAddKeywordRule:              "添加规则",
		SettingsRemoveKeywordRuleFor:        "移除关键词规则 %s",
		SettingsRuleArrow:                   "\u2192",
		SettingsClearBlockedTags:            "清空屏蔽名单",
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
		SyncMoreAvailable:                   "还有更早的书签，再次同步即可继续。",
		SyncStored:                          "已同步 %d 条新书签。",
		SyncNothingNew:                      "同步完成，没有新书签。",
		SyncProgressLabel:                   "同步中… {pages} 页 · {items} 条",
		SettingsRunAfterSync:                "同步完成后自动分析",
		SyncNeedsXCredentials:               "同步需要 X 登录状态。请从已登录的浏览器重新连接，或在 设置 > 同步 里填入 cookie。",
		SettingsXBrowserRead:                "从浏览器读取",
		SettingsXBrowserReading:             "正在查找已登录的浏览器…",
		SettingsXBrowserSaved:               "已从浏览器读取并保存到钥匙串。",
		SettingsXBrowserUnchanged:           "浏览器和钥匙串里是同一个会话。如果已过期，请在浏览器里重新登录 x.com。",
		SettingsXBrowserFailed:              "读取浏览器失败：%s",
		SettingsXBrowserHint:                "从这台 Mac 上已登录 x.com 的浏览器读取会话，Cookie 保存在钥匙串里。",
		SettingsXTestHint:                   "向 x.com 发一次请求，在同步之前分清是 Cookie 失效还是网络不通。",
		SettingsXBrowserNeedsAccess:         "系统拒绝访问浏览器数据。请在系统隐私设置里允许本应用访问，然后重试；或在下方粘贴 Cookie 请求头。",
		SettingsXBrowserNoCookies:           "没有在任何浏览器里找到 X 的 cookie。请先在浏览器里登录 x.com，或在下方粘贴 Cookie 请求头。（%s）",
		SettingsXCookiesTitle:               "X 凭证",
		SettingsXCookiesHint:                "请先在浏览器里登录 x.com。一键读取即可；cookie 保存在本机钥匙串，不会上传。",
		SettingsXCredentialPaste:            "Cookie 或 cURL",
		SettingsXCredentialPasteHint:        "或手工粘贴：开发者工具 > 网络 > 任意请求 > 复制 Cookie 请求头。curl 命令或导出的 cookie 列表也可以，两个值都会从里面读出来。",
		SettingsXCredentialPastePlaceholder: "auth_token=…; ct0=…",
		SettingsXCredentialMissing:          "粘贴的内容里没有 %s。完整的 Cookie 请求头两个都在。",
		SettingsXCredentialNothingFound:     "粘贴的内容里没有 auth_token 和 ct0。请复制 Cookie 请求头，不是响应头。",
		SettingsXCookiesSaved:               "已保存到钥匙串。",
		SettingsXCookiesAbsent:              "还没有保存 X 凭证，同步无法运行。",
		ActionSaveCookies:                   "保存凭证",
		ActionReconnectX:                    "从浏览器重新连接",
		SyncLookingForBrowser:               "正在查找已登录的浏览器…",
		SettingsXTestConnection:             "测试连接",
		SettingsXTestRunning:                "测试中…",
		SettingsXTestOK:                     "凭证可用：X 返回了书签数据。",
		SettingsXTestNoCredentials:          "先连接或保存 cookie，再测试。",
		SettingsXTestNoProbe:                "此构建没有安装连接测试，无法访问 X。",
		SettingsXTestDirect:                 "直连（无代理）",
		SettingsProxyRoute:                  "网络路径：%s",
		SettingsSyncOnLaunch:                "打开应用时同步",
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
		SettingsSaveRawPayloads:             "保留原始响应",
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
		ActionRunNow:                        "立即分析",
		ActionStopRun:                       "停止本次分析",
		ActionOpenTagManagement:             "打开标签管理",
		ConnectionOK:                        "连接成功，%s 已响应。",
		ConnectionFailed:                    "连接失败。",
		ConnectionNoTester:                  "未配置连接测试器。",
		ConnectionTesting:                   "正在测试连接…",
		ActionSave:                          "保存",
		ActionClear:                         "清除",
		ActionLoadMore:                      "加载更多",
		ActionShowMore:                      "显示更多",
		ActionAdd:                           "添加",
		ActionFocusSearch:                   "搜索",

		AppearanceSystem:          "跟随系统",
		AppearanceLight:           "浅色",
		AppearanceDark:            "深色",
		ThemeLabel:                "主题",
		AppTitle:                  "书签",
		ActionCopyLink:            "复制链接",
		ActionCopied:              "已复制",
		ActionOpenBookmark:        "打开书签",
		ActionCopyText:            "复制正文",
		ActionAddToSelection:      "加入选择",
		ActionRemoveFromSelection: "移出选择",
		ActionFilterTag:           "按此标签筛选",
		ActionOnlyTag:             "只看此标签",
		ActionFilterAuthor:        "按此作者筛选",
		ActionOnlyAuthor:          "只看此作者",
		ActionCopyHandle:          "复制 @handle",
		ActionCollapseSidebar:     "收起侧栏",
		ActionExpandSidebar:       "展开侧栏",
		ActionToggleSidebar:       "显示/隐藏侧栏",
		OpenOnX:                   "在 X 中打开",
		MediaPrevious:             "上一张",
		MediaNext:                 "下一张",
		MediaClose:                "关闭预览",
		MediaPlay:                 "播放视频",
		MediaPlayGif:              "播放 GIF",
		MediaTitle:                "媒体资源",
		ToastNoLink:               "暂无可复制链接",
		ToastDone:                 "完成",

		StatusIdle:      "空闲",
		StatusRunning:   "同步中",
		StatusSuccess:   "成功",
		StatusPartial:   "部分成功",
		StatusCancelled: "已取消",
		// “错误” 是该项状态的用词；“失败” 曾是本目录自己的用词，出现在应用不绘制的状态行中。
		StatusError: "错误",
		// 同步失败类型的中文文案。
		SyncErrorAuthExpired:   "X 登录已失效，请重新登录 X 后再同步。",
		SyncErrorRateLimited:   "X 暂时限制了同步请求，请稍后再试。",
		SyncErrorNetworkError:  "网络请求失败，请检查网络连接后重试。",
		SyncErrorSchemaChanged: "X 页面或 API 结构已变化，请更新应用后再同步。",
		SyncErrorInterrupted:   "上次同步被中断（浏览器回收了后台进程），请重新同步。",
		SyncErrorUnknown:       "同步失败，请重试。",

		SuggestionsTitle:     "建议",
		SuggestionsEmpty:     "没有待审阅的建议。",
		SuggestionsAccept:    "接受建议",
		SuggestionsReject:    "拒绝建议",
		SuggestionsAcceptFor: "接受建议 %s",
		SuggestionsRejectFor: "拒绝建议 %s",

		CardSelectBookmark:     "选择书签",
		CardSelectBookmarkFor:  "选择 %s 的书签",
		ActionOpenSyncSettings: "打开同步设置",
	},
}

// Translator renders messages for one locale.
type Translator struct {
	locale Locale
}

// New builds a translator, resolving the locale against the system setting.
func New(preferred, system string) *Translator {
	return &Translator{locale: ResolveLocale(preferred, system)}
}

// Locale returns the resolved locale.
func (t *Translator) Locale() Locale { return t.locale }

// T renders a message.
//
// A key with no translation falls back to English, and a key in neither returns
// the key itself: showing the key makes a missing string obvious in a screenshot,
// where an empty label would look like a layout bug.
func (t *Translator) T(key Key) string {
	if catalog, ok := catalogs[t.locale]; ok {
		if text, ok := catalog[key]; ok {
			return text
		}
	}
	if text, ok := catalogs[EN][key]; ok {
		return text
	}
	return key
}

// Tf renders a message with arguments.
func (t *Translator) Tf(key Key, args ...any) string {
	return fmt.Sprintf(t.T(key), args...)
}

// Keys is every key any catalogue holds, sorted.
//
// It used to read the English table alone, which made a key that existed only in Chinese
// invisible to Missing: SettingsRuleArrow was missing from English for the whole life of the
// build, and the first English screenshot of the rule editor printed the key's own name. A
// union is the only version that can notice a hole in the table it is reading from.
func Keys() []Key {
	seen := make(map[Key]bool, len(catalogs[EN]))
	for _, catalog := range catalogs {
		for key := range catalog {
			seen[key] = true
		}
	}
	keys := make([]Key, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}

// Missing returns the keys a locale does not translate.
//
// It exists so a half-finished translation is a test failure rather than something
// a user discovers by seeing English in a Chinese interface.
func Missing(locale Locale) []Key {
	catalog, ok := catalogs[locale]
	if !ok {
		return Keys()
	}

	var missing []Key
	for _, key := range Keys() {
		if _, present := catalog[key]; !present {
			missing = append(missing, key)
		}
	}
	return missing
}

// AssertComplete reports a locale that does not translate every key.
//
// The composition root calls it at startup. A missing string is a build defect
// rather than a runtime condition, and the interface is rendered from these
// catalogs: a hole shows a raw key to the user, which is worse than refusing to
// start with a message naming the key.
func AssertComplete() error {
	for _, locale := range AllLocales {
		if missing := Missing(locale); len(missing) > 0 {
			sort.Strings(missing)
			return fmt.Errorf("i18n: locale %q is missing %d key(s): %s",
				locale, len(missing), strings.Join(missing, ", "))
		}
	}
	return nil
}
