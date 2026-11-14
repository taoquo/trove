package ui

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/egoist/mygo"
	engine "github.com/taoquo/trove/internal/autotag"
	"github.com/taoquo/trove/internal/credentials"
	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/i18n"
	"github.com/taoquo/trove/internal/importexport"
	"github.com/taoquo/trove/internal/media"
	"github.com/taoquo/trove/internal/usecase/settings"
	"github.com/taoquo/trove/internal/usecase/suggestions"
	syncer "github.com/taoquo/trove/internal/usecase/syncer"
	tagging "github.com/taoquo/trove/internal/usecase/tagging"
	"github.com/taoquo/trove/internal/usecase/workspace"
)

// State is everything the workspace view draws from.
//
// It holds data only. A view may be rebuilt several times per event, so nothing
// in a view body may do I/O, allocate a handle, or mutate this: elements from an
// earlier pass have already expired by the time the next one runs.
type State struct {
	Appearance Appearance
	// SystemDark is what the OS reports, which only matters for "system".
	SystemDark bool

	Selection string
	Query     string
	Sort      domain.SortOrder
	// OnlyWithMedia and OnlyLongform are the filter toggles. They are kept here
	// because the view binds controls to them, and the filter they produce is
	// derived on each load rather than stored.
	OnlyWithMedia bool
	OnlyLongform  bool

	// Data is the loaded view.
	Data workspace.View
	// Loaded is false until the first read finishes, so the empty state is not
	// shown before the database has been read.
	Loaded bool
	// Refreshing is true while a background refresh is recomputing the page. The page
	// already on screen stays there (see reload.go), so this only drives a quiet
	// "updating" note beside the count -- it never swaps in the skeleton, which would
	// blink on every keystroke.
	Refreshing bool
	// LoadingMore is true while the next result page is being read, so the button that
	// asked for it can say so and cannot be pressed twice.
	LoadingMore bool
	// GraphLoading is true while the relation graph is being built. The graph walks the
	// whole library, which is the one read that grows with it.
	GraphLoading bool
	// GraphDirty says a build was requested while one was running, so the result about
	// to be installed is already out of date.
	GraphDirty bool
	// Mutating is true while a library-wide edit runs off the window's thread, and
	// MutationNote is what it is doing. A second click while one is in flight is a
	// no-op rather than a second merge racing the first.
	Mutating     bool
	MutationNote string
	// TagLibraryLoading is true while the tag page's counts are being read, and
	// TagLibraryDirty says an edit arrived while that read was running.
	TagLibraryLoading bool
	TagLibraryDirty   bool
	// Err is the last load or mutation failure, shown rather than swallowed.
	Err string
	// Selected is the bookmark the inspector shows and the card outline marks. A card click
	// sets it (and opens the detail); the arrow keys move it.
	Selected string
	// RevealSelected asks the frame to scroll the selected card into view. It is set by the
	// arrow keys and cleared after the frame that reveals it, so the reader can still scroll
	// the selection off screen afterwards.
	RevealSelected bool
	// SelectedIDs is the batch selection, which is what a bulk action runs on. It is separate
	// from Selected because the inspector follows one bookmark while a bulk action may cover
	// many: the card's corner checkbox toggles this one and stops the click from reaching the
	// card.
	SelectedIDs map[string]bool

	// TagInput is the batch tag field's contents.
	TagInput string
	// DetailTagInput is the inspector's new-tag field, separate from the batch bar: pressing
	// Enter there tags the one bookmark on screen, while the batch field tags the selection.
	// One string for both would mean typing in the detail moves the batch bar's draft, and
	// the two are on screen together whenever a bookmark is open beside a selection.
	DetailTagInput string
	// MoveTargetLabel is the batch list selector's bound value.
	MoveTargetLabel string
	// BulkTagLabel is the batch tag selector's bound value. It is separate from TagInput on
	// purpose: the picker means "give every selected bookmark this one tag" and the input
	// means "give them these tags".
	BulkTagLabel string

	// SuggestionsOpen shows the review panel.
	SuggestionsOpen bool
	// Suggestions holds the pending suggestions while the panel is open.
	Suggestions []suggestions.PendingRow
	// SidebarFacetTab is which of the two facet lists the sidebar shows.
	//
	// One at a time rather than both stacked: on a laptop-height window the two sections
	// together left about three author rows visible, and each one's rows are long enough to
	// deserve the whole column. Empty means the tags tab, so a first frame needs no setup.
	SidebarFacetTab string
	// SidebarSearchOpen is whether the sidebar is showing its search field.
	//
	// Closed by default: the field used to sit permanently above both lists, where it took a
	// row and a half of the column from the list on every frame, including the ones where the
	// reader was browsing rather than searching. One magnifier in the tab row opens the same
	// field for whichever tab is showing, and closing it clears that tab's query so a hidden
	// field can never leave the list silently narrowed.
	SidebarSearchOpen bool
	// PendingSuggestions is what the results pane counts for its banner. It is loaded on every
	// reload rather than drawn from storage: a draw function must not do IO, and a count that
	// only existed while the suggestions panel was open could never be shown on the feed.
	PendingSuggestions []suggestions.PendingRow

	// SyncRunning is true while a sync is in flight.
	SyncRunning bool
	// SyncProgress is how far the sync has got.
	SyncProgress string
	// LastSyncStatus is the outcome of the last sync, for the status line, and
	// LastSyncedAt is when it happened. Both are read back from storage rather than
	// kept only in memory, so a restart does not forget when the library was last
	// brought up to date.
	LastSyncStatus domain.SyncStatus
	// LastSyncErrorKind is the stored kind of the last failure, from domain.SyncErrorKind. The
	// status line turns it into the sentence shown; LastSyncStatus stays the short word the
	// sidebar's chip needs.
	LastSyncErrorKind string
	// LastSyncErrorSummary is the run's own message, kept for a kind this build does not know.
	// The rule is `summary.errorKind ? localized(kind) : summary.errorSummary`, so an
	// unclassified failure shows what X reported rather than a guess. Both fields are read back
	// from the stored run, so the sentence survives a restart.
	LastSyncErrorSummary string
	LastSyncedAt         time.Time

	// AutoTagRunning is true while an auto-tagging pass is in flight, and
	// autoTagCancel stops the one that is. The shell owns the context, so cancelling it is
	// the signal.
	AutoTagRunning bool
	autoTagCancel  context.CancelFunc
	// AutoTagStatus is the outcome of the last pass.
	AutoTagStatus string
	// LastBundleReport is what the last import or restore changed.
	LastBundleReport string
	// LastBundleOK says whether that report is an outcome or a refusal. The same field
	// carries "Backup exported" and "too many rows to act on", and drawing both in the same
	// muted grey as the hint above them would make a finished action read as another
	// paragraph of explanation. An outcome is drawn in the success colour; a refusal and a
	// progress note keep the muted one.
	LastBundleOK bool
	// BundleBusy is a file-level command in flight: exporting, reading a backup,
	// importing or resetting. It disables the buttons that would start a second one
	// and gives the toolbar line something to say while the work runs off the
	// window's thread.
	BundleBusy bool

	// The auto-tagging settings the panel exposes. The hidden knobs must be reachable,
	// because a batch size or concurrency that can only be changed by editing a file is not
	// a setting.
	AutoTagEnabled   bool
	AutoContinue     bool
	BatchSizeInput   float64
	ConcurrencyInput float64
	UseHashtags      bool
	UseAuthors       bool

	// The sync settings the panel exposes.
	SyncOnLaunch    bool
	SaveRawPayloads bool
	StopBufferPages float64
	// SyncPageLimit caps a first full walk. It was stored and validated but had
	// no control and never reached the syncer, so a library larger than the
	// built-in cap could not be walked in one run.
	SyncPageLimit float64
	// SyncResumePending reports that the last full walk stopped at the cap and
	// there are older bookmarks still to fetch.
	SyncResumePending bool

	// The update preferences the Settings › General rows show. They are read when the page
	// opens rather than in a frame, because the updater keeps them in a file of its own.
	UpdateAutoCheck bool
	UpdateLastCheck time.Time
	RunAfterSync    bool

	// The LLM fields. They are strings because a provider id, a model name and an
	// endpoint are free text: a closed list of models would go stale the week after
	// it shipped.
	LLMEnabled      bool
	ProviderLabel   string
	BaseURLInput    string
	DeploymentInput string
	ModelInput      string
	MaxPerRunInput  float64
	// VocabularyMode is the engine's id, and VocabularyModeLabel is its translated name -- what
	// the select displays. Both are held because a language change has to re-derive the name
	// without re-reading the database (and, on the capture path, without wiping a fixture).
	VocabularyMode      string
	VocabularyModeLabel string
	// APIKeyInput is write-only. The stored key is never read back into the field,
	// because a settings panel that can display a secret is a settings panel that
	// leaks one over a shoulder.
	// ViewMode is how the results are laid out. The grid is the default, not the list,
	// which is why this is not the zero value of the type.
	ViewMode domain.ViewMode
	// ResultsColumns is how many cards the grid last drew side by side, recorded by the
	// grid itself. The keyboard moves by a row, and only the drawing knows the width the
	// breakpoint was measured against -- a tester has no window to ask.
	ResultsColumns int

	// openExternal opens a URL in the user's browser. It is a seam because the call is
	// "tell the operating system", and a test that wrote it would be testing the OS.
	openExternal func(url string) error

	// LightboxIndex is which media item the lightbox shows, or -1 when it is closed.
	LightboxIndex int

	// VideoURL is the file the native player is showing, and empty when nothing plays. The
	// player itself is a view over the window, so this is the record the interface draws
	// from: the poster's play mark hides while it is set (video.go).
	VideoURL string

	// ToastMessage is the transient line at the foot of the window, and ToastUntil is
	// when it stops being shown. The deadline is stored and checked as the overlay is
	// built, so an idle app is not repainted just to expire a message nobody is looking at.
	ToastMessage string
	ToastUntil   time.Time
	// ToastKind is the message's tone: ok, warn or err. The status line carries the
	// same three and colours the pill from them.
	ToastKind string

	// PageChangedAt is when the results pane was last replaced by a different page:
	// another scope, another tag or author, the other layout, the settings page, the tag
	// library or the graph. It is what the pane's entrance counts from (motion.go), and
	// it is a time rather than a flag so an idle window needs no timer to settle.
	PageChangedAt time.Time
	// CopiedAt is when the detail's copy-link button last copied. The button shows a
	// check for a moment afterwards, the local half of the feedback the status line gives.
	CopiedAt time.Time

	// The last auto-tagging run, rendered. The engine reports twelve outcomes and the
	// panel used to show one summary field, so a run stopped by its budget looked the
	// same as one stopped by a missing key.
	AutoTagHeadline string
	// AddURL is the link typed into the add row, and AddProblem is why the last add was
	// refused. A standalone manager has no page to read, so this is its own entry point.
	AddURL     string
	AddText    string
	AddProblem string
	// ListSelected is the list's chosen row index, -1 for none, as GridSelected is the grid's.
	// The two are separate because they are indices into whatever each was handed; the bookmark
	// they stand for is the same field in both cases.
	ListSelected int

	// RenderLimit is how many cards the grid builds: 80 at first and 80 more as the scroll
	// nears the bottom.
	RenderLimit int

	// autoLoadY and renderGrewY are where the scroll was when the last page was read and
	// when the grid last grew, in DIPs. They are the anchors that make "load the next page
	// when the reader reaches the end" run once per reach rather than once per frame: a
	// frame may build several times, and the appended cards do not move the offset the
	// page was asked at. See autoLoadMore.
	autoLoadY   float32
	renderGrewY float32

	// AddOpen is whether the add form is showing.
	//
	// It is a disclosure rather than a permanent row: the main pane is a page heading, a
	// toolbar and the content, and the two inputs and their button would be a fourth line
	// above the toolbar on every screen, including the empty one where there is nothing to
	// add to yet.
	AddOpen bool

	// AutoTagContinueCount is how many times the current chain has continued itself. A run
	// the user starts by hand resets it.
	AutoTagContinueCount int

	// BlockedTagKeyCount is how many tag names auto-tagging may not suggest. The settings
	// panel shows the clear button only when there is something to clear.
	BlockedTagKeyCount int

	// AutoTagFinishedAt is when the last run finished, which the panel says beside the
	// result: a result with no date cannot be told from a stale one.
	AutoTagFinishedAt time.Time
	AutoTagDetail     string
	APIKeyInput       string
	APIKeyStored      bool
	APIKeyProviderID  string // the provider APIKeyStored describes
	// ConnectionResult is what the last "test connection" found. It stays empty
	// until the button is pressed, so the panel never claims a status nobody asked
	// for.
	ConnectionResult string
	ConnectionOK     bool
	ConnectionTested bool
	// ConnectionRunning is the provider probe in flight. The probe is a real request
	// to the provider, so it runs off the window's thread; this is what the panel says
	// while it does, and what keeps a second click from starting a second request.
	ConnectionRunning bool

	// KeywordRules are the user's keyword-to-tag rules, and the two draft fields are
	// the row being typed. The draft is separate so a half-typed rule is never saved:
	// a rule missing either half could never fire, so persisting one would leave a
	// dead row in the list.
	KeywordRules []engine.KeywordTagRule
	DraftKeyword string
	DraftRuleTag string

	// FocusSelected asks the list to put the keyboard back on the selected card, which is
	// what closing the detail does: the card is where the detail was opened from.
	FocusSelected bool

	// GraphMode draws the tag relation graph in place of the results list, GraphModel is
	// what it draws, and GraphIncludeAuthors adds the author nodes.
	//
	// GraphLayout is where each node goes, in the unit square. It is computed with the model,
	// off the window's thread (graphlayout.go): a ring cannot hold a real library's captions,
	// and a simulation run per frame would make the page crawl.
	GraphMode           bool
	GraphModel          domain.GraphViewModel
	GraphLayout         []graphPoint
	GraphIncludeAuthors bool

	// TagMode draws the tag management view in place of the results list.
	TagMode bool
	// TagView is which page of tag management is on screen: the tags table or the
	// auto-tagging controls. Both are inside TagMode; this only chooses the body. Empty is
	// the table, so entering the screen never lands on the configuration page.
	TagView string
	// SidebarTagQuery narrows the sidebar's tag tree.
	//
	// Separate from TagQuery on purpose: that one narrows the tag library screen, and typing in one
	// place must not silently re-filter the other.
	SidebarTagQuery string
	// TagItems is the built tag library, TagQuery/Filter/Sort narrow it.
	TagItems          []domain.TagLibraryItem
	TagQuery          string
	TagFilter         domain.TagLibraryFilter
	TagSort           domain.TagLibrarySortOrder
	TagCleanupMessage string
	// TagSelectedIDs is the multi-selection the merge acts on, TagMergeTarget is the
	// tag it merges into, and TagFocus is the tag with a field open for editing --
	// TagEditField says which. The table edits in place, so
	// TagFocus is a single open field, not a detail panel.
	TagSelectedIDs map[string]bool
	TagMergeTarget string
	TagFocus       string
	TagEditField   string
	TagAliasInput  string
	// TagNameEdit is the focused tag's name while it is being renamed.
	TagNameEdit string
	// CreatingTag shows the sidebar's inline create row, and TagDraft is what has been
	// typed into it.
	CreatingTag bool
	TagDraft    string
	// TagLibraryCreateOpen shows the tag library's own create row.
	//
	// The library is the screen for managing tags and it could not make one: the only create
	// control was the sidebar's "+", which is a row away and easy to miss when the library is
	// the page on screen. One TagDraft feeds whichever row is open, so BeginCreateTag and
	// BeginLibraryCreateTag close the other; without that the two fields edited one string
	// and each showed the other's text.
	TagLibraryCreateOpen bool
	// RenameTagID is the sidebar row whose name is being edited in place, and
	// RenameTagName is the draft. The rename happens on the row itself, not in the tag
	// library's detail panel, and a rename started from one row has to stay on that row
	// while the frame rebuilds.
	RenameTagID   string
	RenameTagName string

	// ConfirmCleanupTags is whether the orphan-cleanup dialog is open.
	ConfirmCleanupTags bool

	// OnlyPendingSuggestions narrows the list to bookmarks a review pass is waiting on.
	OnlyPendingSuggestions bool

	// FilterPopoverOpen shows the toolbar's filter popover.
	FilterPopoverOpen bool
	// FilterPopoverOpenedAt is when it was opened, for the popover's entrance.
	// Zero means at rest -- a restored state or a capture does not animate.
	FilterPopoverOpenedAt time.Time
	// UnavailableFilter backs the two condition rows drawn disabled. It is never read: the
	// rows cannot be toggled, and a checkbox needs somewhere to point.
	UnavailableFilter bool

	// ConfirmResetData is whether the reset dialog is open.
	ConfirmResetData bool

	// RenameListID is the list being renamed, and RenameListName is the draft.
	//
	// Lists are this shell's own addition, so the actions they need are finished here.
	RenameListID   string
	RenameListName string

	// ConfirmDeleteTag is the tag a deletion is open for, and ConfirmDeleteTags says the
	// bulk deletion is open.
	ConfirmDeleteTag  string
	ConfirmDeleteTags bool

	// ConfirmMergeTags says the merge is waiting for its answer.
	//
	// A merge deletes the tags it folds in, so it asks first. The fields it needs are the ones
	// the bar already holds -- the selection and TagMergeTarget -- and the dialog is modal, so
	// they cannot move while it is up.
	ConfirmMergeTags bool

	// ActiveAuthors narrows the results by author, and AuthorQuery searches the list.
	//
	// One author at a time: a click replaces the value, and clicking the active row clears it.
	// The list is capped by the service at SidebarLimit and scrolls in full inside the tab.
	ActiveAuthors []string
	AuthorQuery   string

	// ActiveTagIDs is the set of tags narrowing the results.
	//
	// The query layer combines the set with AND; the shell narrows by one tag at a time, so
	// the set is a single element or empty.
	ActiveTagIDs []string

	// SavedViews are the stored views, read by the settings panel's saved-views section.
	SavedViews []domain.SavedView
	// XCredentialPaste is the one credentials field in the sync settings.
	//
	// The values come from a browser read (internal/credentials/browser.go) or from this
	// paste. Without the field, Store.SaveXCredentials would be called from nowhere and every
	// sync would fail with "credentials: nothing stored (x.auth_token)" with no way to fix it
	// from inside the app. The field holds one blob rather than one field per cookie value,
	// because that is what a reader actually copies, and xsource.ParseCredentialPaste reads
	// the pair out of it.
	XCredentialPaste string
	// XBrowserReadNote is the result line under the browser-import button: reading, or what went
	// wrong. Empty before the button is pressed.
	XBrowserReadNote string
	// XBrowserReadRunning reports that a browser read is in flight, so both controls that can
	// start one are disabled while it runs and the note says what is happening.
	XBrowserReadRunning bool
	// XCredentialsStored reports whether the Keychain holds both, read at startup and after a
	// save. It never causes a token to be read back into the view.
	XCredentialsStored bool

	// The connection test's outcome, drawn under the cookie fields.
	//
	// XTestTested distinguishes "not run yet" from "ran and failed": without it a
	// failure and a fresh panel look the same, and the empty line reads as success.
	// XTestRoute is the network route the probe took, so a proxied machine can tell
	// a bad cookie from an unreachable host.
	XTestTested  bool
	XTestRunning bool
	XTestOK      bool
	XTestResult  string
	XTestRoute   string

	// SettingsQuery is the settings page's "find a setting" field. The page filters every
	// tab's rows with one; an empty field shows the open tab alone.
	SettingsQuery string
	// SidebarCollapsed is the sidebar hidden. It is the reader's choice once they have made
	// one; until then the window's width decides.
	SidebarCollapsed bool
	// SidebarChosen is whether they have: the title bar's button, the shortcut, or a drag past
	// the minimum, kept as the stored `navLayout` choice and overriding the width at every width.
	SidebarChosen bool

	// SettingsSection is which tab the settings sheet shows.
	//
	// The body is a 168-point nav beside the section's content. The sections are a list, so a
	// reader chooses rather than scrolls through them in whatever order they accumulated.
	SettingsSection string
	// SavedViewName is the name a new view is saved under.
	//
	// The list of stored views is not kept here: it belonged to the sidebar's saved-views
	// section, which no longer draws it. tools/unread.py reports any field that outlives its
	// reader, which is what that check is for.
	SavedViewName string

	// NewListName is the sidebar's new-list field, and ConfirmDeleteList is the list a
	// confirmation is open for.
	NewListName       string
	ConfirmDeleteList string
	TagParentPick     string
	TagActionMsg      string

	// The vocabulary review. ReviewSelected keys are the plan identities, so a
	// checked box survives a re-render.
	ReviewOpen    bool
	ReviewRunning bool
	// ReviewApplying is the panel's busy state while the selected plans are written.
	ReviewApplying bool
	ReviewMerges   []engine.TagMergePlan
	ReviewRenames  []engine.TagRenamePlan
	ReviewSelected map[string]bool
	ReviewError    string
	ReviewApplied  string

	// LeftSidebarWidth and RightSidebarWidth are the rail widths, persisted so a dragged
	// divider stays where it was left.
	LeftSidebarWidth  float32
	RightSidebarWidth float32
	// DetailOpen draws the detail as an overlay, about Selected. A card click sets both at
	// once: the id decides the outline and whether the modal is mounted.
	//
	// Closing the overlay (Escape, the backdrop, 完成) clears only this flag. The id stays so
	// the list keeps a cursor for the arrow keys and Enter.
	DetailOpen bool

	// MiddleWidth is the results pane's width, which is what the inner split binds to: a
	// split sizes its first pane. The inspector's width is derived from it so the persisted
	// value matches.
	MiddleWidth float32

	// Locale is the user's language preference; empty means none has been chosen, which
	// resolves to English.
	Locale string

	// SettingsOpen shows the settings panel.
	SettingsOpen bool
	// AppearanceLabel and LocaleLabel are the settings selectors' bound values.
	// They are bound rather than derived, because a Select needs a pointer to
	// write into and a view body may not recompute state.
	AppearanceLabel string
	LocaleLabel     string
}

// Shell owns the window and the state.
type Shell struct {
	state   *State
	service *workspace.Service
	window  *mygo.Window

	// sortOptions maps the control's index to a sort order. It is the whitelist
	// the ORDER BY is chosen from, never a string built from input.
	sortOptions []domain.SortOrder
	// sortCycle is the order the sort button walks, which is not the same list.
	sortCycle []domain.SortOrder

	// list is the results list's own state, for the same reason and with the same lifetime as
	// grid: ui.List virtualises the rows, and its scroll position, choice and batch selection
	// belong to it rather than to the query.
	list ui.ListState

	// The controls a cursor must be sent to when they open: the list's search (Cmd-F), the
	// tag drafts (whose "+" focuses the row it opened), the detail's close control, the
	// settings find field (Cmd-F), and the tag library's open cell editor. Each is focused
	// from the event that opens it, never every later frame.
	searchField          ui.Handle
	tagDraftField        ui.Handle
	tagLibraryDraftField ui.Handle
	detailTagField       ui.Handle
	detailCloseField     ui.Handle
	settingsFindField    ui.Handle
	tagEditField         ui.Handle

	// resultsScroll is where the grid's two-column scroll is. It stays on the Shell for the
	// same reason the list's state does: it belongs to a build-pass element and has to
	// survive the rebuilds a scroll causes.
	resultsScroll ui.ScrollState
	// listScroll is the same for the list view. The two are separate containers with
	// separate offsets: switching the layout must not carry the other one's place over.
	listScroll ui.ScrollState

	// media loads the pictures a bookmark points at, when one is installed.
	// openExternal opens a URL in the user's browser. It is a seam because the call is
	// "tell the operating system", and a test that ran it would be testing the OS.
	openExternal func(url string) error
	media        *media.Fetcher
	// updates is the updater plugin behind the Settings › General update card
	// (updatesview.go). It is never nil: a shell built outside the app gets
	// defaultUpdates, which checks nothing.
	updates *Updates
	// video is the native player, or nil where the platform has none (video.go).
	video VideoPlayer
	// draw is the decoded form of what the interface has shown. It is per shell rather
	// than package-level so that two shells -- the window's and a test's -- do not share
	// a budget or a frame counter.
	draw drawCache

	// translator renders every user-visible string.
	translator *i18n.Translator
	// settings persists the preferences. It is optional so the shell can be built
	// without storage, as the render tests do.
	settings *settings.Service
	// suggestions reviews the auto-tagger's proposals.
	suggestions *suggestions.Service
	// syncer performs syncs. It is optional so the render tests need no network.
	syncer *syncer.Service
	// lastSyncReload is when the running sync last refreshed the library. The service
	// writes a page at a time, and reloading on every page would re-read the library
	// four times a second; this rate-limits the refresh (see syncReloadDue).
	lastSyncReload time.Time

	// saveFile and openFiles answer the two file panels, and background runs work off
	// the window's thread. They are seams rather than direct calls so a test can drive
	// the file commands without a window and without opening a real panel; nil means
	// the real dialog and a goroutine when there is a window (see work.go).
	saveFile   func(mygo.SaveDialogOptions) (string, error)
	openFiles  func(mygo.OpenDialogOptions) ([]string, error)
	background func(func())

	// The refresh queue (reload.go). reloadSeq numbers requests so a page computed for
	// an older filter is dropped; reloadPending holds the newest request while a read is
	// in flight, and reloadRunning says the drain loop is alive.
	reloadMu      sync.Mutex
	reloadSeq     atomic.Uint64
	reloadPending *reloadRequest
	reloadRunning bool
	// tagging runs the auto-tagger.
	tagging *tagging.Service
	// captureRun, when set, stands in for a live auto-tagging run in a screenshot.
	//
	// RunProgressViewFor wants an actual *engine.RunState, and a capture cannot start a real run
	// against a model provider. This is the seam the capture path uses so that the sidebar's
	// progress block can be photographed at all; it is nil everywhere else, and the sidebar
	// checks it before the service.
	captureRun *engine.RunState

	// runAutoTag is the engine call, behind a seam so a test can choose the summary the
	// continuation decides on.
	runAutoTag func(context.Context, engine.Settings) (engine.RunSummary, error)
	// testPublish, when set, replaces the window in publish, so a test can reproduce the
	// deferral a real window gives (the auto-tag chain's ordering defect lived there).
	testPublish func(func())
	// persistedLeft and persistedRight are the widths last written, so several
	// rebuilds of one frame are not mistaken for several drags.
	persistedLeft      float32
	persistedRight     float32
	persistedCollapsed bool
	persistedChosen    bool

	// testedViewportWidth is the width a tester draws at; zero means no window, so not narrow.
	testedViewportWidth int

	// importexport moves the library in and out of a bundle.
	importexport *importexport.Service
	// credentials reads and writes the Keychain. The settings panel needs it to
	// manage provider keys, and it never reads a key back into the view.
	credentials *credentials.Store
	// xProbe is one real bookmarks request, for the sync settings' connection test.
	//
	// It is a seam rather than the concrete client because the X client is built in
	// main.go for the syncer, and the shell must not grow a dependency on the wire
	// protocol just to press a button.
	xProbe XProbe
	// onLocaleChanged rebuilds anything built outside a view, such as the menu.
	onLocaleChanged func()
	// browserReadDone is closed when readXCookiesFromBrowser's goroutine has finished.
	// In the app publish serialises the write onto the window's thread; a tester has no
	// window, so the write runs on the goroutine and polling the field was a data race.
	// The channel is that wait, and closing it after the write orders the read.
	browserReadDone chan struct{}
	// probeDone is the same wait for the connection test, for the same reason: with no
	// window, publish runs the write on the goroutine and a test reading the outcome
	// immediately would race it.
	probeDone chan struct{}
	// confirmAnswered and confirmAnswer carry a confirmation card's click from the handler
	// that recorded it to the next build, which is the pass that acts on it: a click handler
	// runs after the build, so the card cannot read its own button's press in the same pass
	// that drew it. The zero value is "nothing answered".
	confirmAnswered bool
	confirmAnswer   int
	// deleteBookmarks is a deletion awaiting confirmation.
	deleteBookmarks PendingAction[[]string]
	// removeTags is a tag removal awaiting confirmation.
	removeTags PendingAction[struct {
		BookmarkIDs []string
		TagIDs      []string
	}]
	// bundle holds an import or restore awaiting confirmation.
	bundle PendingAction[BundleRequest]
}

// NewShell builds the shell.
func NewShell(service *workspace.Service, appearance Appearance) *Shell {
	shell := &Shell{
		service:    service,
		translator: i18n.New(""),
		updates:    defaultUpdates(),
		state: &State{
			Appearance: appearance,
			// Open on the grid.
			ViewMode:      domain.ViewGrid,
			LightboxIndex: -1,
			// The first page: 80 cards, then 80 per scroll.
			RenderLimit: ResultsRenderLimit,
			// -1, not 0: both anchors are compared with a scroll offset, and 0 is the top
			// of the list -- a real place where the first automatic page must be allowed.
			autoLoadY:   -1,
			renderGrewY: -1,
			// The tags tab is the one the window opens on: it is the first of the two
			// facets and the one a new library fills first. The authors tab keeps its own
			// search text and scroll position while it is away.
			SidebarFacetTab: sidebarTabTags,
			// The default width, applied here as well as on load because zero is not a width:
			// the split's own 40-point minimum is what a zero leaves, so a preferences read that
			// fails would open the app with a clipped sidebar rather than the default one.
			// ApplyPreferences overwrites this when it has a stored value, which matters more
			// than the default does.
			LeftSidebarWidth: WidthSidebarDef,
			// -1 is "nothing chosen", which is how ListState carries it. Its zero value is 0,
			// so leaving it unset opened the window with the first bookmark looking chosen:
			// the list drew its selected state on row 0 and the inspector would have followed
			// it on the first click anywhere.
			ListSelected: -1,
			Selection:    selectionAll,
			Sort:         domain.SortTimeline,
		},
		// The whitelist the ORDER BY is chosen from. It holds every order the query
		// layer accepts, including one the cycle does not reach.
		sortOptions: []domain.SortOrder{
			domain.SortTimeline,
			domain.SortSavedDesc,
			domain.SortSavedAsc,
			domain.SortCreatedDesc,
			domain.SortLikesDesc,
		},
		// The cycle the sort button walks. The sequence has four entries and leaves
		// `saved-desc` out: it is supported by the query layer and by a saved view's stored
		// order, but the button never reaches it.
		sortCycle: []domain.SortOrder{
			domain.SortTimeline,
			domain.SortSavedAsc,
			domain.SortCreatedDesc,
			domain.SortLikesDesc,
		},
	}
	// Both batch pickers show a word before anything is chosen. Their bound values start
	// empty, and an empty value would draw a bare chevron with no way to tell the tag picker
	// from the list one.
	shell.syncBatchLabels()
	return shell
}

// sortKeys maps each order to its message key.
//
// The five orders are all reachable here.
// sortLabelKeys maps an order to its label.
//
// Two orders share the "recently saved" copy: `timeline` and `saved-desc`. That is
// deliberate, not an oversight: the switch falls through to the same copy.

// State exposes the state for tests.
func (s *Shell) State() *State { return s.state }

// filter turns the scope and the active conditions into a query filter.
func (s *Shell) filter() domain.LibraryFilter {
	filter := domain.LibraryFilter{
		Query:         s.state.Query,
		OnlyWithMedia: s.state.OnlyWithMedia,
		OnlyLongform:  s.state.OnlyLongform,
		// The review row is a filter too, so a query carrying it must reach storage.
		OnlyWithPendingSuggestions: s.state.OnlyPendingSuggestions,
	}

	switch {
	case s.state.Selection == selectionInbox:
		filter.ListID = domain.InboxListID
	case s.state.Selection == selectionUnclassified:
		// No tags, whatever list the bookmark is in.
		filter.OnlyUnclassified = true
	case strings.HasPrefix(s.state.Selection, prefixList):
		filter.ListID = strings.TrimPrefix(s.state.Selection, prefixList)
	}

	// Conditions come from the facets and nowhere else. The scope above is navigation, so
	// a tag in the selection is not a filter: the old fallback here meant the same tag
	// could narrow the list through two different fields, and only one of them was on
	// screen when it did.
	// Each facet holds at most one value, so the match rule cannot change the result: one
	// tag is "all of them" and "any of them" at the same time. The modes still use the same
	// defaults, so a saved view restored with more than one value keeps its meaning.
	if len(s.state.ActiveAuthors) > 0 {
		filter.AuthorMatchMode = domain.MatchAny
		filter.AuthorHandles = append([]string(nil), s.state.ActiveAuthors...)
	}

	if len(s.state.ActiveTagIDs) > 0 {
		filter.TagMatchMode = domain.MatchAll
		filter.TagIDs = append([]string(nil), s.state.ActiveTagIDs...)
	}
	// selectionAll adds nothing, which is what makes it show everything.
	return filter
}

// Reload re-reads the workspace synchronously.
//
// Interactive callers use ReloadAsync instead (reload.go): on a large library this read
// is 60-120 ms, and running it in the click handler that asked for it stopped the window
// drawing. This form stays for the calls that are a statement about a write rather than
// a redraw -- after an import or a sync page -- and for tests, which assert on the state
// the read produced on the very next line.
func (s *Shell) Reload(ctx context.Context) {
	if s.service == nil {
		s.state.Loaded = true
		return
	}
	s.applyReload(s.gatherReload(ctx, s.reloadQuery()))
}

// keepSelection drops a selection the new page does not contain.
//
// Filtering and switching scope used to clear the selection outright, which closed the
// detail under the reader even when the bookmark they were reading survived the change --
// the cost of every click was losing your place. The rule is the one a reader expects: a
// bookmark still in the list keeps its panel and its row; one the new filter removed is
// let go, because a detail pane describing something the list no longer shows is worse
// than no pane.
//
// The batch selection follows the same rule. A bulk action must not run on rows that are
// no longer on screen, or it acts on bookmarks the user cannot see.
func (s *Shell) keepSelection() {
	present := make(map[string]bool, len(s.state.Data.Bookmarks))
	for _, row := range s.state.Data.Bookmarks {
		present[row.TweetID] = true
	}

	if s.state.Selected != "" && !present[s.state.Selected] {
		s.state.Selected = ""
		s.state.DetailOpen = false
	}

	if len(s.state.SelectedIDs) > 0 {
		for id := range s.state.SelectedIDs {
			if !present[id] {
				delete(s.state.SelectedIDs, id)
			}
		}
		if len(s.state.SelectedIDs) == 0 {
			s.state.SelectedIDs = nil
		}
	}
}

// tagLibraryText renders the tag management view.

// ReloadPendingSuggestions reads the suggestions awaiting review.
//
// Small and read on every reload: the results pane shows "N 待确认标签" beside a button that
// accepts them all, and the count is whatever is pending right now, not what was pending
// when some panel was last opened.
func (s *Shell) ReloadPendingSuggestions(ctx context.Context) {
	if s.suggestions == nil {
		s.state.PendingSuggestions = nil
		return
	}
	rows, err := s.suggestions.Pending(ctx)
	if err != nil {
		return
	}
	s.state.PendingSuggestions = rows
}
