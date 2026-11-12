package ui

import (
	"context"
	"fmt"
	"time"

	engine "github.com/taoquo/x-master/apps/macos/internal/autotag"
	"github.com/taoquo/x-master/apps/macos/internal/domain"
	"github.com/taoquo/x-master/apps/macos/internal/i18n"
	"github.com/taoquo/x-master/apps/macos/internal/usecase/suggestions"
	"github.com/taoquo/x-master/apps/macos/internal/xsource"
)

// The screenshot harness lives here rather than beside the panels it opens.
//
// ApplyCaptureState is tool code: it exists so a picture can show a state that lives behind an
// interaction. Round 453 added the autotag-running case and pushed panels.go past the 900-line
// cap, which is what surfaced the grouping -- a block that photographs panels is not part of one.
// Splitting it is the cap doing its job, not an inconvenience to route around.
//
// ApplyCaptureReceipt writes the sentence a finished sync shows.
//
// It cannot be a case in ApplyCaptureState: the receipt is translated and carries a count, and
// the capture path applies its language after the state, which clears exactly this class of
// message by design (preferences.go). So the caller writes it last, just before drawing.
func (s *Shell) ApplyCaptureReceipt(count int) {
	s.state.LastSyncStatus = domain.SyncSuccess
	if count > 0 {
		s.ShowStatus(ToastOK, s.t().Tf(i18n.SyncStored, count))
		return
	}
	s.ShowStatus(ToastOK, s.t().T(i18n.SyncNothingNew))
}

// ApplyCaptureResetReport writes the sentence a finished reset shows.
//
// Same reason as ApplyCaptureReceipt: the sentence is translated, and the capture path
// pins the language after the state, which clears the reports it had written.
func (s *Shell) ApplyCaptureResetReport() {
	s.state.LastBundleReport = s.t().T(i18n.LocalDataResetDone)
	// The flag is what draws it as an outcome; without it the picture shows the muted
	// styling a progress note gets, which is the thing round 31 changed.
	s.state.LastBundleOK = true
}

// ApplyCaptureExportPrompt writes the sentence the data-safety page shows while a file
// panel is up.
//
// Same reason as ApplyCaptureResetReport: the sentence is translated, and the capture path
// pins the language after the state, which clears exactly this field.
func (s *Shell) ApplyCaptureExportPrompt() {
	s.state.LastBundleReport = s.t().T(i18n.BundleChoosingWhere)
	// A progress note, not a result: it is drawn muted like every other in-flight line.
	s.state.LastBundleOK = false
}

// ApplyCaptureState opens the panel a screenshot asked for.
//
// The states worth photographing are the ones behind an interaction, and the capture fires
// once on the first frames, so this is the only way to see them without a person at the
// screen. The caller gates it on XBM_CAPTURE; this method is deliberately inert for any
// name it does not know, so a typo photographs the normal window instead of failing.
func (s *Shell) ApplyCaptureState(state string) bool {
	// Screenshots get default preferences, not whatever the last screenshot saved.
	//
	// Round 295 compared two settings captures and found Appearance and Language had gone from
	// "System" to "Light" and "English" without anything having changed them: round 277 made this
	// path open the panel through OpenSettings, which is right, and the panel then reads what is
	// stored -- and a previous run's capture had stored it. So the pictures in docs/screenshots
	// differ in state as well as in build, which is the same problem round 279 found when the
	// empty-state pair turned out to be from round 242's build.
	//
	// Resetting the fields and then running the panel's own label refresh is deliberately not the
	// same as calling SetAppearance: that one persists, and a screenshot must not rewrite the
	// preferences of whoever runs it.
	s.state.Appearance = AppearanceSystem
	s.state.Locale = ""
	s.syncPreferenceLabels()

	switch state {
	case "workspace":
		// The normal window, which is what the source render calls this state. It is a named
		// case rather than a fallthrough so the name is *known*: an unknown state makes
		// capture.go return before it applies XBM_CAPTURE_LOCALE, and a "workspace" capture came
		// out with a Chinese page whose footer chip still said "EN" because state.Locale had
		// been reset to "" and the language was never applied.
	case "detail":
		// Choose the first row first. OpenSelectedDetail draws what is chosen, and a capture
		// that asks for the detail without anything chosen photographs an empty pane.
		if len(s.state.Data.Bookmarks) > 0 {
			s.state.Selected = s.state.Data.Bookmarks[0].TweetID
		}
		s.OpenSelectedDetail()
	case "detail-media":
		// The detail with its media preview open. The preview is behind two interactions -- pick
		// a row with media, then press the picture -- and its whole point is that it sits above
		// the detail, which is the one thing a picture can settle. The first bookmark carrying
		// media is chosen because the first row of a real library often has none.
		for _, row := range s.state.Data.Bookmarks {
			if len(row.Media) > 0 {
				s.state.Selected = row.TweetID
				break
			}
		}
		s.OpenSelectedDetail()
		s.OpenLightbox(0)
	case "detail-media-gif":
		// The preview of an animated GIF, which until round 27 had no play control on either
		// surface. A wider page than the window opens with is read: the library's GIFs are
		// spread through 9.5k rows and the newest hundred can hold none (round 27's first
		// attempt photographed the plain workspace). The rows are put into the loaded state,
		// not fetched into the page: a capture must not change what the next launch opens on.
		if s.service == nil {
			break
		}
		wide, err := s.service.Load(context.Background(), domain.LibraryQuery{
			Filter: s.filter(), Sort: s.state.Sort, Limit: domain.MaxLibraryPageSize,
		})
		if err != nil {
			break
		}
		foundRow, foundIndex := -1, -1
		for i, row := range wide.Bookmarks {
			for j, item := range row.Media {
				if item.Type == domain.MediaAnimatedGif {
					foundRow, foundIndex = i, j
					break
				}
			}
			if foundRow >= 0 {
				break
			}
		}
		if foundRow < 0 {
			break
		}
		s.state.Data.Bookmarks = wide.Bookmarks
		s.state.Selected = wide.Bookmarks[foundRow].TweetID
		s.OpenSelectedDetail()
		s.OpenLightbox(foundIndex)
	case "filtered":
		// The active filters in the search's row (shell-contract.md S25). A capture cannot click
		// a tag and an author, and the chips' whole job is to say what is narrowing the list, so
		// this is the only way to photograph them: the first tag and the first author the real
		// library holds.
		if len(s.state.Data.Tags) > 0 {
			s.state.ActiveTagIDs = []string{s.state.Data.Tags[0].ID}
		}
		if len(s.state.Data.Authors) > 0 {
			s.state.ActiveAuthors = []string{s.state.Data.Authors[0].AuthorHandle}
		}
		s.Reload(context.Background())
	case "settings":
		// Through OpenSettings, not by setting the flag.
		//
		// Round 277 photographed the settings panel for the first time and found the Appearance
		// and Language selectors blank. OpenSettings calls syncPreferenceLabels, which is what
		// fills the strings those two selectors are bound to; setting SettingsOpen directly
		// skipped it, so both rendered empty. The panel is not wrong -- going around its entry
		// point is, and that is the same mistake round 242 made by setting AddOpen from a test.
		s.OpenSettings()
	case "settings-sync":
		s.OpenSettings()
		s.state.SettingsSection = SettingsSectionSync
	case "settings-autotag":
		// The second settings tab, for comparing its content against the reference picture.
		s.OpenSettings()
		s.state.SettingsSection = SettingsSectionAutoTag
	case "settings-data":
		s.OpenSettings()
		s.state.SettingsSection = SettingsSectionDataSafe
	case "settings-export":
		// The data-safety page while a save panel is up. Round 34 moved the panel out of the
		// click handler; the note that says the window is waiting is translated, so it is
		// written by ApplyCaptureExportPrompt once the language is pinned, like the receipts.
		s.OpenSettings()
		s.state.SettingsSection = SettingsSectionDataSafe
		s.state.BundleBusy = true
	case "settings-lists-views":
		// The 列表与视图 tab. No capture had ever opened these two cards with rows in them
		// (round 26), which is how five identical row actions and an English default view
		// name went unseen. The rows are put into the loaded state, not written: a screenshot
		// must not add a list or a view to the user's library.
		s.OpenSettings()
		s.state.SettingsSection = SettingsSectionListsViews
		s.state.Data.Lists = []domain.ListSummary{
			{List: domain.List{ID: domain.InboxListID, Name: "Inbox"}, BookmarkCount: 37},
			{List: domain.List{ID: "capture-reading", Name: "Reading"}, BookmarkCount: 210},
			{List: domain.List{ID: "capture-research", Name: "Research"}, BookmarkCount: 64},
		}
		s.state.SavedViews = []domain.SavedView{
			{ID: "capture-view-1", Name: "Unread longform", Query: "longform"},
			{ID: "capture-view-2", Name: "AI coding", Query: "ai"},
		}
	case "settings-search":
		// The cross-section find field, showing a result from a tab other than the open one:
		// the page's whole reason for having one field rather than one per tab (S37). The
		// query is the cookie block's own title, so it matches in both locales.
		s.OpenSettings()
		s.state.SettingsQuery = s.t().T(i18n.SettingsXCookiesTitle)
	case "sidebar-collapsed":
		// The hidden column. Set directly rather than through ToggleSidebar, which persists:
		// a screenshot must not change what the user sees at next launch. The pick is marked
		// in memory only, so the width rule does not put the column back in the capture.
		s.state.SidebarCollapsed = true
		s.state.SidebarChosen = true
	case "tags":
		s.state.TagMode = true
		s.state.TagView = TagViewTags
		if s.service != nil {
			s.ReloadTagLibrary(context.Background())
		}
	case "tag-autotag":
		// The second page of tag management (shell-contract.md S26). No run and no result, so
		// the picture shows the controls at rest -- the state a reader meets first.
		s.state.TagMode = true
		s.state.TagView = TagViewAutoTag
		s.loadAutoTagSettings(context.Background())
		if s.service != nil {
			s.ReloadTagLibrary(context.Background())
		}
		s.ReloadPendingSuggestions(context.Background())
	case "tags-merge":
		// The tag library's bulk bar, which only exists while rows are ticked. Round 28 rebuilt
		// its merge: the target starts on a placeholder and Merge waits for a choice, and that
		// resting state had never been photographed (L17). Two rows, because the bar's count is
		// a plural sentence.
		s.state.TagMode = true
		s.state.TagView = TagViewTags
		if s.service != nil {
			s.ReloadTagLibrary(context.Background())
		}
		if listed := s.VisibleTagItems(); len(listed) >= 2 {
			s.state.TagSelectedIDs = map[string]bool{listed[0].TagID: true, listed[1].TagID: true}
			s.state.TagMergeTarget = ""
		}
	case "tags-merge-confirm":
		// The question itself: which tag survives, and how many disappear. Raised through the
		// bar's own entry point so the picture shows what the button shows.
		s.state.TagMode = true
		s.state.TagView = TagViewTags
		if s.service != nil {
			s.ReloadTagLibrary(context.Background())
		}
		if listed := s.VisibleTagItems(); len(listed) >= 2 {
			s.state.TagSelectedIDs = map[string]bool{listed[0].TagID: true, listed[1].TagID: true}
			s.state.TagMergeTarget = listed[0].Name
			s.AskMergeSelectedTags()
		}
	case "review-proposals":
		// The vocabulary review panel, which had no capture state at all until round 29: it
		// sits behind the toolbar button and an LLM request, so no picture had ever shown
		// the list, its checkboxes or its Apply row (L17). The proposals stand in; the names
		// come from the real library so the rows measure like real ones.
		s.captureReviewPanel(false)
	case "review-partial-error":
		// A partly failed apply: the error above a list that still holds what did not land,
		// which is the state the panel used to skip (it closed instead).
		s.captureReviewPanel(true)
	case "review-failed":
		// The request itself failed. The panel covers the page, so the retry has to be in it.
		s.state.TagMode = true
		s.state.TagView = TagViewTags
		if s.service != nil {
			s.ReloadTagLibrary(context.Background())
		}
		s.state.ReviewOpen = true
		// The engine's own sentence, the way a provider failure arrives: the prefix beside it
		// is the translated part.
		s.state.ReviewError = "no tagging service is configured"
	case "tags-editor":
		// The row's inline rename field, which is behind a click on a row's pencil. The first
		// row's name is a field in the name cell; the aliases and the theme edit in their own
		// cells the same way (S28).
		//
		// The first *visible* row, not TagItems[0]: the library is stored in name order
		// (tag_repo.go:59-84) and drawn in bookmark-count order (compareTagItems), so the two
		// disagree. Round 23 found every tags-editor capture identical to `tags` -- the editor
		// was open on a row below the fold.
		s.state.TagMode = true
		if s.service != nil {
			s.ReloadTagLibrary(context.Background())
		}
		if visible := s.VisibleTagItems(); len(visible) > 0 {
			s.beginTagEdit(visible[0].TagID, TagEditName)
		}
	case "tags-aliases", "tags-theme":
		// The other two in-place fields, for the same reason: the alias editor's Save and cancel
		// and the theme cell's picker are behind a click on the row.
		s.state.TagMode = true
		if s.service != nil {
			s.ReloadTagLibrary(context.Background())
		}
		if visible := s.VisibleTagItems(); len(visible) > 0 {
			field := TagEditAliases
			if state == "tags-theme" {
				field = TagEditTheme
			}
			s.beginTagEdit(visible[0].TagID, field)
		}
	case "sidebar-search":
		// The field the magnifier opens. A query is set so the picture also shows the list
		// answering it; the field is not drawn at all when it is closed.
		s.state.SidebarSearchOpen = true
		s.state.SidebarTagQuery = "a"
	case "sidebar-search-empty":
		// The other half of the field: a query that matches nothing. This state exists
		// because the column used to go blank with no sentence, and a blank column is
		// exactly what a capture of the row list cannot tell from "no tags" (round 17).
		s.state.SidebarSearchOpen = true
		s.state.SidebarTagQuery = "zzz-no-such-tag"
	case "tag-draft":
		// The sidebar's inline create row, which is behind a press on "+". BeginCreateTag is
		// the entry point rather than the flag, for the reason OpenSettings is: it is what
		// gives the field the cursor, and the row's ring only draws while it has it.
		s.BeginCreateTag()
	case "graph":
		// Built through the shell's own path, not by setting the flag: the model is what the
		// page has to show, and a capture that only raised GraphMode photographed the page's
		// "still being read" state. The read lands inside this call on the capture path, where
		// there is no window to run it off the thread of.
		s.state.GraphMode = true
		s.RebuildGraph(context.Background())
	case "graph-authors":
		// The graph with its author layer on, through the same path the control takes. This
		// state exists because the layer's head read "位作者节点 %!(EXTRA int=18)" and no capture
		// ever showed the layer at all (round 13): a state nobody photographs is a state nobody
		// reads.
		s.state.GraphMode = true
		s.state.GraphIncludeAuthors = true
		s.RebuildGraph(context.Background())
	case "scrolled":
		// The results read partway down: the toolbar stays at the top of the page and the feed
		// scrolls under it (pagination.go). A capture cannot scroll, so the offset is set here.
		s.resultsScroll.Y = 640
	case "loading":
		s.state.Loaded = false
	case "search-empty":
		// A query nothing matches, which is the state spec 12.3 calls search-no-results.
		s.state.Query = "zzz-no-such-bookmark-zzz"
		s.Reload(context.Background())
	case "autotag-result":
		// The auto-tagging page with a finished run, so the .options-settings-result card has
		// content to draw: the source shows it under the run control with the recorded time and
		// two sentences. The card and the controls moved here with the feature (S26).
		s.state.TagMode = true
		s.state.TagView = TagViewAutoTag
		s.loadAutoTagSettings(context.Background())
		if s.service != nil {
			s.ReloadTagLibrary(context.Background())
		}
		s.ReloadPendingSuggestions(context.Background())
		// The words come from the translator, as everywhere else; only the numbers are fixtures.
		s.state.AutoTagHeadline = s.t().Tf(i18n.OutcomeOk, 1605, 1550)
		s.state.AutoTagDetail = s.t().Tf(i18n.AutoTagDetail, 9300, 450, 48, 3, 0, 100000)
		s.state.AutoTagFinishedAt = time.Date(2026, 10, 17, 19, 2, 0, 0, time.UTC)
	case "autotag-running":
		// A run in progress, for the sidebar's progress block.
		//
		// Round 453 found the block had never been photographed -- it lives behind a live run
		// against a model provider, which a capture cannot start. Built through the engine's own
		// state so the picture goes through the same derivation the real run does: the phase, the
		// processed/total count and the elapsed/remaining foot all come from RunProgressViewFor.
		// The flag the shell reads, as well as the state the bar reads. Without it the
		// sidebar's branch saw "no run is marked active" beside a live run and drew
		// "Analyze now" under a bar that was already running (board Inbox, 2026-11-10).
		s.state.AutoTagRunning = true
		started := time.Now().Add(-39 * time.Second)
		s.captureRun = &engine.RunState{
			RunID:     "capture",
			StartedAt: started,
			UpdatedAt: time.Now(),
			Phase:     engine.PhaseLLM,
			Processed: 120,
			Total:     2000,
			Status:    engine.StatusRunning,
		}
	case "suggestions-pending":
		// The results pane's pending-suggestions bar. It needs rows to count, and a capture
		// cannot create suggestions without an auto-tagging run, so this stands some in.
		rows := make([]suggestions.PendingRow, 0, 1144)
		for i := 0; i < 1144; i++ {
			rows = append(rows, suggestions.PendingRow{
				Suggestion: domain.TagSuggestion{ID: fmt.Sprintf("capture-%d", i)},
			})
		}
		s.state.PendingSuggestions = rows
	case "sidebar-authors":
		// The other facet tab, which the tags tab hides completely. The first author is chosen so
		// the picture also shows what a row looks like when it is the condition in effect: the
		// accent label, the tinted ground and the left bar, and the count beside it.
		s.state.SidebarFacetTab = sidebarTabAuthors
		if len(s.state.Data.Authors) > 0 {
			s.state.ActiveAuthors = []string{s.state.Data.Authors[0].AuthorHandle}
		}
		s.Reload(context.Background())
	case "sync-running":
		s.state.SyncRunning = true
		// A mid-run snapshot, so the picture shows the progress line the run actually
		// draws rather than just the word "Syncing".
		s.state.SyncProgress = syncProgressLabel(s.t(),
			xsource.Progress{PagesCompleted: 3, Fetched: 60})
	case "sync-resume":
		// The sentence the footer used to leave as an instruction with no control. Round 30
		// put the labelled start under it, so the picture is the pair.
		s.state.SyncResumePending = true
	case "settings-reset":
		// The data-safety page after a reset. The report is written by
		// ApplyCaptureResetReport once the language is pinned, like the sync receipt.
		s.state.SettingsOpen = true
		s.state.SettingsSection = SettingsSectionDataSafe
	case "sync-done":
		// A finished run. The receipt itself is written by ApplyCaptureReceipt after the
		// language is pinned: it is a translated sentence, and this path runs first.
		s.state.LastSyncStatus = domain.SyncSuccess
	case "sync-error":
		// The badge compares against the translated word, so the state has to be set to it rather
		// than to a literal.
		s.state.LastSyncStatus = domain.SyncError
		// The capture path names a kind so the picture shows the sentence, not just the word.
		s.state.LastSyncErrorKind = string(domain.ErrUnknown)
	case "batch-selected":
		// The bulk bar only exists once a card's corner box is ticked, so no screenshot had
		// ever shown it (round 25). Ticked through the same function the card calls, and on
		// two rows so the count reads as the plural sentence.
		for _, row := range s.state.Data.Bookmarks {
			if len(s.state.SelectedIDs) == 2 {
				break
			}
			s.toggleSelection(row.TweetID)
		}
	case "filter":
		// The filter popover, which had no capture state until round 302 and therefore had never
		// been looked at. Raised through the same call the toolbar's button makes, so the picture
		// shows what pressing that button shows.
		s.ToggleFilterPopover()
	case "list":
		s.state.ViewMode = domain.ViewList
	case "add":
		s.state.AddOpen = true
	case "dark":
		// Batch A wants the same screen in both appearances.
		//
		// The state field is set directly rather than through SetAppearance, which persists:
		// the first version called it and the capture wrote dark mode into the real
		// preferences, so a screenshot tool changed what the user would see next launch.
		// Invalidate at the end of this function is what redraws.
		s.state.Appearance = AppearanceDark
	default:
		return false
	}
	if s.window != nil {
		s.window.Invalidate()
	}
	return true
}

// captureReviewPanel stands in for a finished vocabulary review.
//
// The plan list is data the model would return; the names are the library's own and the
// pairs read the way a real proposal does (a small tag folded into the larger one it
// duplicates). failed asks for the partly-failed rendering: the plan that did not land,
// ticked, with the reason above it -- which is what the panel now keeps on screen.
func (s *Shell) captureReviewPanel(failed bool) {
	s.state.TagMode = true
	s.state.TagView = TagViewTags
	if s.service != nil {
		s.ReloadTagLibrary(context.Background())
	}

	merges := []engine.TagMergePlan{
		{
			TargetTagID: "capture-target-1", TargetName: "ai agent",
			SourceTagIDs: []string{"capture-source-1"}, SourceNames: []string{"agent"},
		},
		{
			TargetTagID: "capture-target-2", TargetName: "ai coding",
			SourceTagIDs: []string{"capture-source-2"}, SourceNames: []string{"coding agent"},
		},
	}
	renames := []engine.TagRenamePlan{
		{TagID: "capture-rename-1", From: "prompt-engineering", To: "Prompt engineering"},
	}

	s.state.ReviewOpen = true
	s.state.ReviewRunning = false
	s.state.ReviewMerges = merges
	s.state.ReviewRenames = renames
	s.state.ReviewSelected = map[string]bool{}
	for _, merge := range merges {
		s.state.ReviewSelected[engine.MergePlanKey(merge)] = true
	}
	for _, rename := range renames {
		s.state.ReviewSelected[engine.RenamePlanKey(rename)] = true
	}
	if failed {
		s.state.ReviewMerges = nil
		s.state.ReviewError = s.t().Tf(i18n.TagRenameTaken, renames[0].To)
	}
}
