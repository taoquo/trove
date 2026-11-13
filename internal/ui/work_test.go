package ui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo"

	engine "github.com/taoquo/trove/internal/autotag"
	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/i18n"
	"github.com/taoquo/trove/internal/importexport"
	"github.com/taoquo/trove/internal/ports"
	"github.com/taoquo/trove/internal/storage/sqlite"
)

// A command that reads or writes the whole library must not run in the handler that
// started it. The window draws on that thread, so the app stops drawing until the
// command finishes -- the reported symptom was "clicking import or export freezes the
// app". These tests hand the shell a background runner that captures the work and does
// not run it, which is how "the handler returned before the work happened" is provable
// without timing anything.
//
// The file panels are seams for the same reason in miniature: mygo.Dialog would open a
// real panel and block the suite, so a command that called it directly could only be
// tested by hand.

// bundleService attaches a real import/export service over the shell's database.
func bundleService(t *testing.T, shell *Shell, db *sqlite.DB) {
	t.Helper()
	repos := ports.Repositories{
		Bookmarks:   sqlite.NewBookmarkRepository(db),
		Tags:        sqlite.NewTagRepository(db),
		Lists:       sqlite.NewListRepository(db),
		Suggestions: sqlite.NewSuggestionRepository(db),
		Analysis:    sqlite.NewAnalysisRepository(db),
		Settings:    sqlite.NewSettingsRepository(db),
		SavedViews:  sqlite.NewSavedViewRepository(db),
		SyncRuns:    sqlite.NewSyncRunRepository(db),
	}
	shell.UseImportExport(importexport.New(repos, fixedClock{now: clockBase}, "0.3.0"))
}

// capturedBackground makes the shell collect its background work instead of running it.
func capturedBackground(shell *Shell) *func() {
	var work func()
	shell.background = func(fn func()) { work = fn }
	return &work
}

// TestExportRunsOffTheWindowThread covers the reported freeze, and the panel that came
// with it: the handler returns before either the panel or the export happens.
func TestExportRunsOffTheWindowThread(t *testing.T) {
	shell, db := newShell(t)
	bundleService(t, shell, db)
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(context.Background())

	path := filepath.Join(t.TempDir(), "out.trovebundle")
	panels := 0
	shell.saveFile = func(mygo.SaveDialogOptions) (string, error) {
		panels++
		return path, nil
	}
	work := capturedBackground(shell)

	if err := shell.ExportBundle(context.Background()); err != nil {
		t.Fatalf("ExportBundle: %v", err)
	}

	if *work == nil {
		t.Fatal("the export ran in the click handler: the window cannot draw until it finishes")
	}
	if panels != 0 {
		t.Fatal("the save panel was opened in the click handler, so the handler blocks until it is dismissed")
	}
	if !shell.state.BundleBusy {
		t.Error("an export is running and the window does not say so")
	}
	if got := shell.state.LastBundleReport; got != shell.t().T(i18n.BundleChoosingWhere) {
		t.Errorf("status line = %q, want the choosing-a-location sentence", got)
	}
	if _, err := os.Stat(path); err == nil {
		t.Error("the file was written before the work ran")
	}

	(*work)()

	if panels != 1 {
		t.Errorf("the save panel was opened %d times, want 1", panels)
	}
	if shell.state.BundleBusy {
		t.Error("the export finished and the busy flag stayed set")
	}
	if got := shell.state.LastBundleReport; got != shell.t().T(i18n.BundleExported) {
		t.Errorf("status line = %q, want the exported sentence", got)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(raw) == 0 {
		t.Error("the bundle is empty")
	}
}

// TestAFileCommandDoesNotStack is the double-click case: a second click while the first
// export is still waiting for a location must not start a second one.
func TestAFileCommandDoesNotStack(t *testing.T) {
	shell, db := newShell(t)
	bundleService(t, shell, db)
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(context.Background())

	panels := 0
	shell.saveFile = func(mygo.SaveDialogOptions) (string, error) {
		panels++
		return filepath.Join(t.TempDir(), "out.trovebundle"), nil
	}
	work := capturedBackground(shell) // never run: the first export is still in flight

	if err := shell.ExportBundle(context.Background()); err != nil {
		t.Fatalf("ExportBundle: %v", err)
	}
	if err := shell.ExportBundle(context.Background()); err != nil {
		t.Fatalf("second ExportBundle: %v", err)
	}

	if *work == nil {
		t.Fatal("the first click started no work, so there is nothing to protect")
	}
	if panels != 0 {
		t.Fatal("a panel was opened before the captured work ran")
	}
	(*work)()

	if panels != 1 {
		t.Errorf("the save panel was opened %d times, want 1: the second click must be ignored", panels)
	}
}

// TestACancelledDialogDoesNothing keeps the rule the panel already had: dismissing it is
// not an error and not a command.
func TestACancelledDialogDoesNothing(t *testing.T) {
	shell, db := newShell(t)
	bundleService(t, shell, db)
	shell.saveFile = func(mygo.SaveDialogOptions) (string, error) { return "", nil }
	work := capturedBackground(shell)

	if err := shell.ExportBundle(context.Background()); err != nil {
		t.Fatalf("a cancelled dialog must not be an error, got %v", err)
	}
	if *work == nil {
		t.Fatal("the click started no work, so the panel is still in the handler")
	}
	(*work)()

	if shell.state.BundleBusy {
		t.Error("a cancelled dialog left the shell busy, so every file command stays disabled")
	}
	if got := shell.state.LastBundleReport; got != "" {
		t.Errorf("a cancelled dialog left %q on screen", got)
	}
}

// TestImportReadsTheFileOffTheWindowThread covers the other half of the freeze: the
// preview parses the whole bundle before the confirmation can be drawn.
func TestImportReadsTheFileOffTheWindowThread(t *testing.T) {
	shell, db := newShell(t)
	bundleService(t, shell, db)
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(context.Background())

	path := exportedBundle(t, db, "bundle.trovebundle")
	shell.openFiles = func(mygo.OpenDialogOptions) ([]string, error) { return []string{path}, nil }
	work := capturedBackground(shell)

	if err := shell.AskBundleImport(context.Background(), false); err != nil {
		t.Fatalf("AskBundleImport: %v", err)
	}
	if *work == nil {
		t.Fatal("the bundle was read in the click handler")
	}
	if shell.bundle.Confirmation.Asked() {
		t.Error("the confirmation was armed before the file was read")
	}

	(*work)()

	if shell.state.BundleBusy {
		t.Error("the busy flag stayed set after the preview")
	}
	if !shell.bundle.Confirmation.Asked() {
		t.Error("the preview finished and the confirmation was never armed")
	}
}

// TestAGarbageBundleIsReportedNotArmed is the failure path: the user is told what is
// wrong instead of being asked to confirm a file that cannot load.
func TestAGarbageBundleIsReportedNotArmed(t *testing.T) {
	shell, db := newShell(t)
	bundleService(t, shell, db)

	path := filepath.Join(t.TempDir(), "junk.trovebundle")
	if err := os.WriteFile(path, []byte("not a bundle"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	shell.openFiles = func(mygo.OpenDialogOptions) ([]string, error) { return []string{path}, nil }
	work := capturedBackground(shell)

	if err := shell.AskBundleImport(context.Background(), false); err != nil {
		t.Fatalf("AskBundleImport: %v", err)
	}
	(*work)()

	if shell.bundle.Confirmation.Asked() {
		t.Error("a rejected file was armed for confirmation")
	}
	if shell.ToastText() == "" {
		t.Error("a rejected file was not reported")
	}
	if shell.state.BundleBusy {
		t.Error("the busy flag stayed set after a rejected file")
	}
}

// TestRestoreRunsOffTheWindowThread covers the confirmed action: it rewrites every row.
func TestRestoreRunsOffTheWindowThread(t *testing.T) {
	shell, db := newShell(t)
	bundleService(t, shell, db)
	ctx := context.Background()
	seed(t, db, "1", "ada", "to be replaced", clockBase)
	shell.Reload(ctx)

	path := exportedBundle(t, db, "restore.trovebundle")
	shell.bundle.Arm(BundleRequest{Path: path, Restore: true}, "Restore?", "body", "Replace")
	shell.bundle.Confirmation.Answer(0)

	work := capturedBackground(shell)
	if err := shell.applyBundle(ctx); err != nil {
		t.Fatalf("applyBundle: %v", err)
	}
	if *work == nil {
		t.Fatal("the restore ran in the confirmation handler")
	}
	if !shell.state.BundleBusy {
		t.Error("a restore is running and the window does not say so")
	}

	(*work)()

	if shell.state.BundleBusy {
		t.Error("the restore finished and the busy flag stayed set")
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM bookmarks`).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("%d bookmarks after the restore, want the bundle's 1", count)
	}
}

// TestResetRunsOffTheWindowThread covers the destructive command: it deletes from every
// table, and it used to do it in the click handler.
func TestResetRunsOffTheWindowThread(t *testing.T) {
	shell, db := newShell(t)
	bundleService(t, shell, db)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(ctx)

	work := capturedBackground(shell)
	shell.ConfirmResetLocalDataNow(ctx)

	if *work == nil {
		t.Fatal("the reset ran in the handler")
	}
	if !shell.state.BundleBusy {
		t.Error("a reset is running and the window does not say so")
	}

	(*work)()

	if shell.state.BundleBusy {
		t.Error("the reset finished and the busy flag stayed set")
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM bookmarks`).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Errorf("%d bookmarks remain after a reset", count)
	}
}

// TestTheProviderProbeRunsOffTheWindowThread covers the third command of this kind: it
// is a real HTTP request to the model provider, and ten seconds is an ordinary answer.
func TestTheProviderProbeRunsOffTheWindowThread(t *testing.T) {
	shell, _, tagger := shellWithTagging(t)
	probed := 0
	tagger.UseConnectionTester(func(engine.Settings) engine.ConnectionTestResult {
		probed++
		return engine.ConnectionTestResult{OK: true, Model: "test-model"}
	})

	work := capturedBackground(shell)
	shell.TestConnection(context.Background())

	if *work == nil {
		t.Fatal("the provider probe ran in the click handler")
	}
	if probed != 0 {
		t.Error("the probe went out before the background runner got it")
	}
	if !shell.state.ConnectionRunning {
		t.Error("the panel does not say a probe is running")
	}
	if got := shell.state.ConnectionResult; got != shell.t().T(i18n.ConnectionTesting) {
		t.Errorf("result line = %q, want the testing sentence", got)
	}

	(*work)()

	if probed != 1 {
		t.Errorf("the probe ran %d times, want 1", probed)
	}
	if shell.state.ConnectionRunning {
		t.Error("the probe finished and the running flag stayed set")
	}
	if !shell.state.ConnectionOK || !shell.state.ConnectionTested {
		t.Error("the successful probe was not recorded")
	}
	if got := shell.state.ConnectionResult; got == "" || got == shell.t().T(i18n.ConnectionTesting) {
		t.Errorf("result line = %q, want the outcome", got)
	}
}

// TestASecondProbeClickIsIgnored keeps the provider from being asked twice for one
// intention.
func TestASecondProbeClickIsIgnored(t *testing.T) {
	shell, _, tagger := shellWithTagging(t)
	probed := 0
	tagger.UseConnectionTester(func(engine.Settings) engine.ConnectionTestResult {
		probed++
		return engine.ConnectionTestResult{OK: true}
	})

	shell.background = func(func()) {} // capture and drop: the first probe is in flight
	shell.TestConnection(context.Background())
	shell.TestConnection(context.Background())

	if probed != 0 {
		t.Errorf("the probe ran %d times, want it still queued", probed)
	}
	if !shell.state.ConnectionRunning {
		t.Error("the running flag was cleared by the ignored click")
	}
}

// TestMergingTagsRunsOffTheWindowThread covers the slowest click in the app: a merge
// reads every tag relation and rewrites the matching ones.
func TestMergingTagsRunsOffTheWindowThread(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	tags := sqlite.NewTagRepository(db)
	createTags(t, tags, map[string]string{"target": "ai agent", "source": "agentic ai"})
	shell.ReloadTagLibrary(ctx)

	shell.state.TagSelectedIDs = map[string]bool{"source": true}
	shell.state.TagMergeTarget = "ai agent"

	work := capturedBackground(shell)
	shell.AskMergeSelectedTags()
	shell.ConfirmMergeSelectedTagsNow(ctx)

	if *work == nil {
		t.Fatal("the merge ran in the click handler")
	}
	if !shell.state.Mutating {
		t.Error("a merge is running and the window does not say so")
	}
	if got := shell.state.MutationNote; got != shell.t().T(i18n.TagMerging) {
		t.Errorf("status note = %q, want the merging sentence", got)
	}
	if _, found, err := tags.Get(ctx, "source"); err != nil || !found {
		t.Fatalf("the merge happened before the work ran: found=%v err=%v", found, err)
	}

	(*work)()

	if shell.state.Mutating {
		t.Error("the merge finished and the busy flag stayed set")
	}
	if _, found, err := tags.Get(ctx, "source"); err != nil || found {
		t.Errorf("the merged tag survived: found=%v err=%v", found, err)
	}
	if shell.state.TagActionMsg == "" {
		t.Error("the merge did not report what it did")
	}
}

// TestASecondMergeClickIsIgnored keeps a double press from running two merges over the
// same selection.
func TestASecondMergeClickIsIgnored(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	tags := sqlite.NewTagRepository(db)
	createTags(t, tags, map[string]string{"target": "ai agent", "source": "agentic ai"})
	shell.ReloadTagLibrary(ctx)
	shell.state.TagSelectedIDs = map[string]bool{"source": true}
	shell.state.TagMergeTarget = "ai agent"

	works := capturedBackgroundQueue(shell)
	shell.AskMergeSelectedTags()
	shell.ConfirmMergeSelectedTagsNow(ctx)
	shell.ConfirmMergeSelectedTagsNow(ctx)

	if len(*works) != 1 {
		t.Errorf("%d merges started, want 1: the second click must be ignored", len(*works))
	}
}

// TestTheReviewRunsOffTheWindowThread covers the batch behind the vocabulary panel: a
// review with several plans is several merges.
func TestTheReviewRunsOffTheWindowThread(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	tags := sqlite.NewTagRepository(db)
	createTags(t, tags, map[string]string{"target": "ai agent", "source": "agentic ai"})
	shell.ReloadTagLibrary(ctx)

	plan := engine.TagMergePlan{TargetTagID: "target", SourceTagIDs: []string{"source"}}
	shell.state.ReviewMerges = []engine.TagMergePlan{plan}
	shell.state.ReviewSelected = map[string]bool{engine.MergePlanKey(plan): true}
	shell.state.ReviewOpen = true

	work := capturedBackground(shell)
	shell.ApplyVocabularyReview(ctx)

	if *work == nil {
		t.Fatal("the review ran in the click handler")
	}
	if !shell.state.Mutating {
		t.Error("the review is running and the window does not say so")
	}
	if _, found, err := tags.Get(ctx, "source"); err != nil || !found {
		t.Fatalf("the review was applied before the work ran: found=%v err=%v", found, err)
	}

	(*work)()

	if shell.state.Mutating {
		t.Error("the review finished and the busy flag stayed set")
	}
	if _, found, err := tags.Get(ctx, "source"); err != nil || found {
		t.Errorf("the reviewed merge did not run: found=%v err=%v", found, err)
	}
	if shell.state.ReviewApplied == "" {
		t.Error("the review did not report how many changes it applied")
	}
}

// TestTheLegacyBackupRunsOffTheWindowThread covers the second export: the flat JSON legacy
// backup that can be restored elsewhere.
func TestTheLegacyBackupRunsOffTheWindowThread(t *testing.T) {
	shell, db := newShell(t)
	bundleService(t, shell, db)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(ctx)

	path := filepath.Join(t.TempDir(), "backup.json")
	shell.saveFile = func(mygo.SaveDialogOptions) (string, error) { return path, nil }
	work := capturedBackground(shell)

	if err := shell.ExportLegacyBackup(ctx); err != nil {
		t.Fatalf("ExportLegacyBackup: %v", err)
	}

	if *work == nil {
		t.Fatal("the export ran in the click handler")
	}
	if !shell.state.BundleBusy {
		t.Error("an export is running and the window does not say so")
	}
	if _, err := os.Stat(path); err == nil {
		t.Error("the file was written before the work ran")
	}

	(*work)()

	if shell.state.BundleBusy {
		t.Error("the export finished and the busy flag stayed set")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	// The file is the legacy backup format, which is what makes it restorable elsewhere.
	if !strings.Contains(string(raw), `"exportVersion"`) {
		t.Errorf("the backup is not the legacy format: %s", raw[:min(len(raw), 200)])
	}
	if !strings.Contains(string(raw), `"1"`) {
		t.Error("the backup does not hold the bookmark")
	}
}

// TestACancelledLegacyBackupDoesNothing keeps the rule every file panel has.
func TestACancelledLegacyBackupDoesNothing(t *testing.T) {
	shell, db := newShell(t)
	bundleService(t, shell, db)
	shell.saveFile = func(mygo.SaveDialogOptions) (string, error) { return "", nil }
	work := capturedBackground(shell)

	if err := shell.ExportLegacyBackup(context.Background()); err != nil {
		t.Fatalf("a cancelled dialog must not be an error, got %v", err)
	}
	if *work == nil {
		t.Fatal("the click started no work, so the panel is still in the handler")
	}
	(*work)()

	if shell.state.BundleBusy {
		t.Error("a cancelled dialog left the shell busy, so every file command stays disabled")
	}
	if got := shell.state.LastBundleReport; got != "" {
		t.Errorf("a cancelled dialog left %q on screen", got)
	}
}

// createTags inserts tags by id and name.
func createTags(t *testing.T, tags *sqlite.TagRepository, names map[string]string) {
	t.Helper()
	for id, name := range names {
		tag := domain.Tag{
			ID: id, Name: name, CanonicalKey: domain.CanonicalTagKey(name),
			CreatedAt: domain.FormatTimestamp(clockBase), UpdatedAt: domain.FormatTimestamp(clockBase),
		}
		if err := tags.Create(context.Background(), tag); err != nil {
			t.Fatalf("Create %s: %v", id, err)
		}
	}
}

// exportedBundle writes a real bundle built from the library in db.
func exportedBundle(t *testing.T, db *sqlite.DB, name string) string {
	t.Helper()
	repos := ports.Repositories{
		Bookmarks:   sqlite.NewBookmarkRepository(db),
		Tags:        sqlite.NewTagRepository(db),
		Lists:       sqlite.NewListRepository(db),
		Suggestions: sqlite.NewSuggestionRepository(db),
		Analysis:    sqlite.NewAnalysisRepository(db),
		Settings:    sqlite.NewSettingsRepository(db),
		SavedViews:  sqlite.NewSavedViewRepository(db),
		SyncRuns:    sqlite.NewSyncRunRepository(db),
	}
	service := importexport.New(repos, fixedClock{now: clockBase}, "0.3.0")
	raw, err := service.Export(context.Background(), importexport.DefaultExportOptions())
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}
