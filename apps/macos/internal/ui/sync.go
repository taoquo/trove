package ui

import (
	"context"
	"strconv"
	"strings"
	"time"

	engine "github.com/taoquo/trove/apps/macos/internal/autotag"
	"github.com/taoquo/trove/apps/macos/internal/domain"
	"github.com/taoquo/trove/apps/macos/internal/i18n"
	syncer "github.com/taoquo/trove/apps/macos/internal/usecase/syncer"
	tagging "github.com/taoquo/trove/apps/macos/internal/usecase/tagging"
	"github.com/taoquo/trove/apps/macos/internal/xsource"
)

// UseSyncer attaches the sync service.
func (s *Shell) UseSyncer(service *syncer.Service) { s.syncer = service }

// UseTagging attaches the auto-tagging service.
func (s *Shell) UseTagging(service *tagging.Service) {
	s.tagging = service
	s.runAutoTag = service.Run
}

// UseAutoTagRunner replaces the engine call the chain makes.
//
// UseAutoTagRunner replaces the engine call the chain makes.
//
// The continuation is a decision about a summary, so a test that wants to watch a chain
// start a second run needs to choose the summary. An interface would have to carry the
// eleven methods the shell calls; the chain only calls this one.
func (s *Shell) UseAutoTagRunner(run func(context.Context, engine.Settings) (engine.RunSummary, error)) {
	s.runAutoTag = run
}

// publish hands a result to the main thread.
//
// Both a sync and an auto-tagging pass do network and database work for seconds at
// a time; running either inside an event handler would freeze the window for the
// duration. They run in a goroutine and publish their result through
// Window.Update, which is the only thread allowed to touch the state.
func (s *Shell) publish(fn func()) {
	if s.testPublish != nil {
		// A test that needs the window's deferral rather than the tester's inline call.
		s.testPublish(fn)
		return
	}
	if s.window == nil {
		// No window means a test; run it inline so assertions see the result.
		fn()
		return
	}
	s.window.Update(fn)
}

// StartSync begins a sync, or reports that one is already running.
func (s *Shell) StartSync(ctx context.Context) bool {
	if s.syncer == nil || s.state.SyncRunning {
		return false
	}

	s.state.SyncRunning = true
	s.state.SyncProgress = ""
	s.state.LastSyncStatus = domain.SyncIdle

	// With an empty Keychain the run first looks for a signed-in browser (the syncer's
	// credential refresher). Saying so beats a spinner with no explanation: the profile read
	// is the only part of a sync that can take seconds before the first page.
	if s.syncNeedsCredentials() {
		s.state.SyncProgress = s.t().T(i18n.SyncLookingForBrowser)
	}

	// A first sync walks the whole timeline; a later one may stop early once the pages
	// are familiar.
	//
	// The test is the stored flag, not "are there any bookmarks": a first sync that was
	// interrupted after storing some has bookmarks and has still never walked the
	// timeline. Deciding from the count made the next run incremental, so it stopped
	// early and the rest of the history was never fetched -- the exact defect the flag
	// exists to prevent, written carefully, and read by nothing until now.
	incremental := s.syncIsIncremental(ctx)

	options := syncer.RunOptions{
		Incremental:     incremental,
		StopBufferPages: int(s.state.StopBufferPages),
		RawPayloads:     s.state.SaveRawPayloads,
		Limit:           int(s.state.SyncPageLimit),
		OnProgress: func(progress xsource.Progress) {
			s.publish(func() {
				s.state.SyncProgress = syncProgressLabel(s.t(), progress)
				// The service writes each page as it arrives, so the list can grow
				// while the run is in flight. Reload at a walking pace rather than
				// on every page: a reload re-reads the library, the tags and the
				// sidebar, and a 400-page first sync would spend the window's whole
				// frame budget on it.
				if s.syncReloadDue() {
					s.ReloadAsync(context.Background())
				}
			})
		},
	}
	if !incremental {
		// Continue a walk that stopped at the cap instead of starting over.
		options.StartCursor = s.syncer.ResumeCursor(ctx)
	}

	go func() {
		run, err := s.syncer.Run(ctx, options)

		if err == nil {
			s.AssignSyncedBookmarksToInbox(context.Background())
		}

		// "Run automatically after sync" was a checkbox that changed a stored
		// field and nothing else. The decision is read here rather than from the
		// panel's state, because the panel may never have been opened.
		runAfterSync := false
		if err == nil && s.tagging != nil {
			if settings, loadErr := s.tagging.LoadSettings(context.Background()); loadErr == nil {
				runAfterSync = settings.Enabled && settings.RunAfterSync
			}
		}

		s.publish(func() {
			s.state.SyncRunning = false
			s.state.SyncProgress = ""
			// The sentence is not written to state.Err. That field is the *load* failure, and the
			// Reload at the end of this callback clears it on success -- which is what happened to
			// every sync failure until round 497: it was set here and wiped a few lines later, so
			// clicking 同步 produced a spinner and then nothing. The failure belongs to the sidebar
			// block, where the source renders it (WorkspaceSidebar.tsx:405), and syncProblemLine
			// derives it from these three fields.
			//
			// The Keychain is read again here because the sentence depends on it: "no cookies
			// stored, add them in Settings > Sync" and "the login expired, sign in again" are
			// different instructions, and the stored kind ("unauthorized") cannot tell them apart.
			s.state.LastSyncErrorKind = run.ErrorKind
			s.state.LastSyncErrorSummary = run.ErrorSummary
			if err != nil {
				s.RefreshXCookieStatus(context.Background())
			}
			// The recorded status, not "did Run return an error": a partial run returns the
			// page failure *and* a partial_success record, and calling that an error in the
			// badge contradicted the run history, the stored summary and the source
			// (src/background/syncBookmarks.ts:189). The sentence under the button still
			// carries the cause, because it reads the kind rather than the status.
			s.state.LastSyncStatus = run.Status
			if s.state.LastSyncStatus == "" {
				s.state.LastSyncStatus = domain.SyncError
			}
			s.state.SyncResumePending = s.syncer.ResumeCursor(context.Background()) != ""
			// What the run stored, while the reader is still looking. A successful run used
			// to end by clearing the progress line and changing a date in the footer: the
			// only evidence a minute of fetching worked was 11px of timestamp. The receipt
			// floats above every page (shell.go's status line). Partial and failed runs keep
			// their own sentences in the sidebar hero instead.
			if err == nil && s.state.LastSyncStatus == domain.SyncSuccess {
				if run.InsertedCount > 0 {
					s.ShowStatus(ToastOK, s.t().Tf(i18n.SyncStored, run.InsertedCount))
				} else {
					s.ShowStatus(ToastOK, s.t().T(i18n.SyncNothingNew))
				}
			}
			s.ReloadAsync(context.Background())
			if runAfterSync {
				s.StartAutoTag(context.Background())
			}
		})
	}()

	return true
}

// syncReloadPace is how often a running sync refreshes what the window shows.
//
// One page takes at least the service's own pace (syncer.defaultPagePace, 250ms)
// plus the request, so this lets most pages through without making the library
// query and the sidebar rebuild the dominant cost of a run.
const syncReloadPace = 900 * time.Millisecond

// syncReloadDue reports whether a progress event should refresh the library, and
// remembers the moment it allowed one.
//
// It is a rate limit rather than a page count because time is what the reload
// costs: refreshing on every page of a 400-page walk is 400 full library reads,
// and the ones a reader can perceive are far fewer than that.
func (s *Shell) syncReloadDue() bool {
	now := time.Now()
	if !s.lastSyncReload.IsZero() && now.Sub(s.lastSyncReload) < syncReloadPace {
		return false
	}
	s.lastSyncReload = now
	return true
}

// syncProgressLabel fills the reference's progress template.
//
// It used to be fmt.Sprintf("%d fetched", ...): English in both locales, and a
// number with no unit. The reference renders "Syncing... {pages} pages · {items}
// items" (src/options/i18n/copy.ts:20, src/popup/components/SyncPanel.tsx:140 via
// formatSyncProgressLabel), which is where the wording comes from.
func syncProgressLabel(t *i18n.Translator, progress xsource.Progress) string {
	return strings.NewReplacer(
		"{pages}", strconv.Itoa(progress.PagesCompleted),
		"{items}", strconv.Itoa(progress.Fetched),
	).Replace(t.T(i18n.SyncProgressLabel))
}

// AssignSyncedBookmarksToInbox is the step that must follow any write that inserts
// bookmarks.
//
// Without it a synced bookmark sits in no list and is invisible in every list view. It
// was written for exactly that and called from nowhere, which a sweep of unreachable
// usecase methods found; it has its own entry point here so the call can be tested
// rather than only inspected.

// AssignSyncedBookmarksToInbox is the step that must follow any write that inserts
// bookmarks.
//
// Without it a synced bookmark sits in no list and is invisible in every list view. It
// was written for exactly that and called from nowhere, which a sweep of unreachable
// usecase methods found; it has its own entry point here so the call can be tested
// rather than only inspected.
func (s *Shell) AssignSyncedBookmarksToInbox(ctx context.Context) int {
	if s.tagging == nil {
		return 0
	}
	assigned, err := s.tagging.EnsureInboxAfterSync(ctx)
	if err != nil {
		s.reportErr(err)
		return 0
	}
	return assigned
}

// syncIsIncremental reports whether a sync may stop early.
//
// It is the stored flag alone, not "are there any bookmarks". A first sync interrupted
// after storing some has bookmarks and has still never walked the timeline; deciding from
// the count made the next run incremental, so it stopped early and the rest of the
// history was never fetched.
//
// It has its own entry point so the decision can be asserted -- the flag is written by
// the service and, until this, read by nothing.

// syncIsIncremental reports whether a sync may stop early.
//
// It is the stored flag alone, not "are there any bookmarks". A first sync interrupted
// after storing some has bookmarks and has still never walked the timeline; deciding from
// the count made the next run incremental, so it stopped early and the rest of the
// history was never fetched.
//
// It has its own entry point so the decision can be asserted -- the flag is written by
// the service and, until this, read by nothing.
func (s *Shell) syncIsIncremental(ctx context.Context) bool {
	if s.syncer == nil {
		return false
	}
	done, err := s.syncer.HasCompletedInitialFullSync(ctx)
	if err != nil {
		// A flag that cannot be read means "not known to be complete", which is the
		// safe direction: a needless full walk costs time, a wrong early stop loses
		// bookmarks.
		return false
	}
	return done
}

// CancelSync asks the running sync to stop.
func (s *Shell) CancelSync() bool {
	if s.syncer == nil {
		return false
	}
	return s.syncer.Cancel()
}

// StartAutoTag begins an auto-tagging pass.
func (s *Shell) StartAutoTag(ctx context.Context) bool {
	// A run the user starts by hand restarts the continuation budget, which is the
	// extension's rule at background/index.ts:164. The chain calls the same path with the
	// flag set, so its count survives.
	return s.startAutoTag(ctx, false)
}

func (s *Shell) startAutoTag(ctx context.Context, continuing bool) bool {
	// The run-state check comes first: if a pass is in flight, saying so is the right
	// answer whether or not the service is reachable, and the order is what keeps a
	// missing service from turning the message into silence.
	if s.state.AutoTagRunning {
		// Saying so, rather than returning quietly: a button that does nothing when
		// pressed reads as a broken button, and the user has no way to tell that a run is
		// already going.
		s.state.AutoTagStatus = s.t().T(i18n.RunAlreadyRunning)
		if s.window != nil {
			s.window.Invalidate()
		}
		return false
	}
	if s.tagging == nil {
		return false
	}
	if !continuing {
		// The hand-started run resets the chain's count.
		s.state.AutoTagContinueCount = 0
	}

	settings, err := s.tagging.LoadSettings(ctx)
	if err != nil {
		s.reportErr(err)
		s.ShowToast(err.Error())
		if s.window != nil {
			s.window.Invalidate()
		}
		return false
	}

	runCtx, cancel := context.WithCancel(ctx)
	s.state.autoTagCancel = cancel
	s.state.AutoTagRunning = true
	s.state.AutoTagStatus = ""

	go func() {
		summary, err := s.runAutoTag(runCtx, settings)

		s.publish(func() {
			s.state.autoTagCancel = nil
			s.state.AutoTagRunning = false
			if err != nil && summary.Outcome == "" {
				s.reportErr(err)
				s.state.AutoTagStatus = s.t().T(i18n.StatusError)
			} else if headline, _ := s.SummarizeRun(summary.Outcome, summary); headline != "" {
				// The localized headline, not the engine's id: the status line used to read
				// "ok" or "no-candidates" in both locales.
				s.state.AutoTagStatus = headline
			} else {
				s.state.AutoTagStatus = ""
			}
			// The run wrote its summary; the panel reads that record, not the shell's copy,
			// so it has to be re-read or it keeps showing the run before this one.
			s.loadLastAutoTagSummary(context.Background())
			s.ReloadAsync(context.Background())

			// The chain. engine.ShouldContinue has been the decision since it was written
			// and nothing called it, so the panel's "continue automatically when work is
			// left" switch changed a field no logic read: a long run stopped with work
			// behind it and the switch did nothing.
			//
			// It is decided here, inside the published update, and not beside the publish
			// call. A real window applies a published update later, so the version that
			// asked outside it read AutoTagRunning while the previous pass's "finished"
			// update was still queued: startAutoTag saw a run in flight, answered "an
			// analysis is already running", and the chain died after the first pass.
			// Tests publish inline, which is why the suite never saw it.
			if engine.ShouldContinue(summary, s.state.AutoTagContinueCount, settings.AutoContinue) {
				s.state.AutoTagContinueCount++
				s.persistAutoTagContinueCount(ctx, s.state.AutoTagContinueCount)
				s.startAutoTag(ctx, true)
			}
		})
	}()

	return true
}

// StopAutoTag asks the running pass to stop.
//
// The run ends after the batch it is on, which is what the message says: the work in
// flight is written, so "stopped" and "stopped after this batch" are different claims.
// That is why the live run's state is asked first: cancelling its context would abort
// the request the message promised to finish.
func (s *Shell) StopAutoTag() bool {
	requested := false
	if s.tagging != nil {
		if live := s.tagging.CurrentRun(); live != nil && live.CancelRequested {
			// Already asked. Cancelling the context now would abort the batch the
			// previous press promised to keep.
			requested = true
		} else {
			requested = s.tagging.RequestCancel()
		}
	}
	if !requested {
		// The fallback for a run with no persisted state: the shell's own context is
		// the only handle left, and a run that never installed its state has no batch
		// to keep.
		if s.state.autoTagCancel == nil {
			return false
		}
		s.state.autoTagCancel()
	}
	s.state.AutoTagStatus = s.t().T(i18n.RunCancelRequested)
	if s.window != nil {
		s.window.Invalidate()
	}
	return true
}

// ResumeInterruptedAutoTag continues a run the app interrupted, when the settings still
// ask for it.
//
// It is called once at startup, after HealStaleRun has recorded the state the previous
// session left behind. The decision is the extension's
// (src/background/index.ts:411): a run the user stopped is never continued, and the
// chain's budget is spent across restarts rather than reset by one.
func (s *Shell) ResumeInterruptedAutoTag(ctx context.Context) bool {
	if s.tagging == nil {
		return false
	}
	summary, found, err := s.tagging.LastSummary(ctx)
	if err != nil || !found {
		return false
	}
	settings, err := s.tagging.LoadSettings(ctx)
	if err != nil {
		return false
	}
	if !shouldResumeInterrupted(summary, settings) {
		return false
	}

	// Stored before the run starts (background/index.ts:431), so a run that keeps
	// dying exhausts the same budget instead of asking once per launch.
	s.state.AutoTagContinueCount = settings.AutoTagContinueCount + 1
	settings.AutoTagContinueCount = s.state.AutoTagContinueCount
	if err := s.tagging.SaveSettings(ctx, settings); err != nil {
		s.reportErr(err)
		return false
	}
	return s.startAutoTag(ctx, true)
}

// shouldResumeInterrupted is the extension's continuation test for a healed run.
func shouldResumeInterrupted(summary engine.RunSummary, settings engine.Settings) bool {
	return summary.Outcome == engine.OutcomeInterrupted &&
		settings.Enabled &&
		engine.ShouldContinue(summary, settings.AutoTagContinueCount, settings.AutoContinue)
}

// persistAutoTagContinueCount stores the chain's position.
func (s *Shell) persistAutoTagContinueCount(ctx context.Context, count int) {
	if s.tagging == nil {
		return
	}
	// Re-read before writing: the run holds the settings from its start, and saving
	// that copy would undo anything the user changed while it ran.
	settings, err := s.tagging.LoadSettings(ctx)
	if err != nil {
		return
	}
	if settings.AutoTagContinueCount == count {
		return
	}
	settings.AutoTagContinueCount = count
	if err := s.tagging.SaveSettings(ctx, settings); err != nil {
		s.reportErr(err)
	}
}

// syncStatusLabel maps a sync status to a translated label.
func syncStatusLabel(t *i18n.Translator, status domain.SyncStatus) string {
	switch status {
	case domain.SyncRunning:
		return t.T(i18n.StatusRunning)
	case domain.SyncSuccess:
		return t.T(i18n.StatusSuccess)
	case domain.SyncPartialSuccess:
		return t.T(i18n.StatusPartial)
	case domain.SyncCancelled:
		// The source's statusLabels has no entry for a cancelled run, so it renders
		// nothing there. This shell names it rather than leaving a gap, and does not use
		// the button's word for it: "Cancel" is an action, not a state.
		return t.T(i18n.StatusCancelled)
	case domain.SyncError:
		// An explicit case, not the default. The default used to *be* the error branch,
		// which is how a workspace that had never synced got an error line.
		return t.T(i18n.StatusError)
	default:
		// No status is not a failure. This returned StatusError for every other input,
		// including the empty string a fresh workspace starts with, so every window that
		// had never synced carried a "失败" line -- measured at (325, 178.8) in the parity
		// capture, in a state the source draws no status line for at all.
		return ""
	}
}

// T renders a message in the current locale.
//
// The composition root needs it to label the menu bar, which is built outside this
// package.
