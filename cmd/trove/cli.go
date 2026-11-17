// Headless commands, for a scheduler or a terminal.
//
// The window is the app. These exist so that "keep the library current overnight" does
// not need a window left open: `trove sync` runs the same service the window runs, over
// the same database, and `trove export` writes the same bundle the export panel writes.
//
// The output is English and goes to stdout, because it is read by a scheduler's log and
// by people with a terminal; the window's two catalogs are for the window.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/egoist/mygo"
	"github.com/taoquo/trove/internal/credentials"
	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/idgen"
	"github.com/taoquo/trove/internal/importexport"
	"github.com/taoquo/trove/internal/ports"
	"github.com/taoquo/trove/internal/storage/sqlite"
	"github.com/taoquo/trove/internal/usecase/settings"
	syncer "github.com/taoquo/trove/internal/usecase/syncer"
	"github.com/taoquo/trove/internal/xsource"
)

// commandHelp is what `trove help` prints. It names the app binary rather than
// os.Args[0], which inside a bundle is a path no one types.
const commandHelp = `trove — a local-first library for your X bookmarks.

Without a command, trove opens its window.

  trove sync [flags]        sync bookmarks now, without a window
  trove export <file>       write a .trovebundle backup
  trove auth [flags]        sign in to X through your browser
  trove version             print the version
  trove help                print this text

sync flags:
  -full        walk the whole timeline instead of stopping at familiar pages
  -quiet       print nothing unless the run fails
  -data-dir    read and write another data directory

auth flags:
  -timeout     how long to wait for the sign-in (default 5m0s)
  -no-open     do not open the browser; wait for a session you sign in to yourself

Scheduling: run "trove sync -quiet" from launchd on macOS or Task Scheduler on
Windows. A run that fails exits non-zero and prints why, so the scheduler records it.
`

// signInURL is where `trove auth` sends the browser. The flow URL is what x.com/login
// redirects to; naming it directly saves the reader a redirect and a page that then
// bounces them again.
const signInURL = "https://x.com/i/flow/login"

// runCommand runs a headless command when the arguments name one. ok is false for
// anything else, including the "-psn_…" argument macOS adds when it opens the app, and
// main goes on to show the window.
func runCommand(args []string) (ok bool, err error) {
	if len(args) == 0 {
		return false, nil
	}
	switch args[0] {
	case "sync":
		return true, runSync(args[1:])
	case "export":
		return true, runExport(args[1:])
	case "auth":
		return true, runAuth(args[1:])
	case "version":
		fmt.Println(appVersion())
		return true, nil
	case "help", "-h", "--help":
		fmt.Print(commandHelp)
		return true, nil
	}
	return false, nil
}

// runSync performs one sync and reports what it stored.
func runSync(args []string) error {
	flags := flag.NewFlagSet("sync", flag.ContinueOnError)
	full := flags.Bool("full", false, "walk the whole timeline")
	quiet := flags.Bool("quiet", false, "print nothing unless the run fails")
	dataDir := flags.String("data-dir", "", "read and write another data directory")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("sync takes no arguments, got %q", flags.Arg(0))
	}

	ctx, stop := signalContext()
	defer stop()

	library, err := openLibrary(*dataDir)
	if err != nil {
		return err
	}
	defer library.close()

	syncService := library.syncer()
	// A sync that was in flight when this command's predecessor died cannot still be
	// running, and the record of it would make the window say a sync is under way.
	if healed, err := syncService.CleanupOrphanedRuns(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "trove sync: could not close stale runs: %v\n", err)
	} else if healed > 0 && !*quiet {
		fmt.Printf("closed %d sync run(s) left in flight\n", healed)
	}

	preferences, err := settings.New(library.repos, ports.SystemClock{}).Load(ctx)
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}

	// The same rule the window follows: the first run walks the timeline, later ones
	// may stop early -- but only once a full walk is known to have completed.
	incremental := false
	if !*full {
		done, err := syncService.HasCompletedInitialFullSync(ctx)
		if err != nil {
			return fmt.Errorf("read sync history: %w", err)
		}
		incremental = done
	}

	if !*quiet {
		fmt.Println("syncing…")
	}
	started := time.Now()
	// The continuation point comes from the same helper the window uses. Spelling
	// the options out here again is what let the CLI drift: it never resumed, so a
	// capped first sync restarted from the top on every run.
	options := syncService.ResumeOptions(ctx, syncer.RunOptions{
		Incremental: incremental,
		RawPayloads: preferences.Sync.SaveRawPayloads,
		Limit:       preferences.Sync.PageLimit,
	})
	run, err := syncService.Run(ctx, options)
	return reportSyncResult(os.Stdout, *quiet, run, err, time.Since(started))
}

// reportSyncResult turns a finished run into the command's outcome and its line.
//
// It is separate from runSync so the exit rule can be tested without a network: the
// status, not the error, decides. A partial run returns its page failure as well as a
// partial_success record, and returning that error made a run that stored thousands of
// bookmarks exit non-zero -- which a scheduler reads as a broken sync, when it was the
// normal outcome of a flaky connection.
func reportSyncResult(w io.Writer, quiet bool, run domain.SyncRun, err error, elapsed time.Duration) error {
	// Another process owns the library. Nothing was attempted, so this is not a
	// failed run: a scheduler must not record it as one, and starting a second
	// writer is exactly what the claim exists to prevent.
	if errors.Is(err, syncer.ErrSyncAlreadyRunning) {
		fmt.Fprintln(w, "another sync is running; leaving it to finish")
		return nil
	}

	if run.Status == domain.SyncPartialSuccess {
		if !quiet {
			fmt.Fprintf(w, "saved %d new, updated %d, %d page(s) in %s (partial: %d page(s) failed)\n",
				run.InsertedCount, run.UpdatedCount, run.PagesCompleted,
				elapsed.Round(time.Second), run.FailedCount)
		}
		return nil
	}
	if err != nil {
		return syncFailure(run, err)
	}
	if run.Status != domain.SyncSuccess {
		return syncFailure(run, nil)
	}
	if !quiet {
		fmt.Fprintf(w, "saved %d new, updated %d, %d page(s) in %s\n",
			run.InsertedCount, run.UpdatedCount, run.PagesCompleted,
			elapsed.Round(time.Second))
	}
	return nil
}

// syncFailure turns a failed run into the sentence a scheduler's log should hold. The
// run's own summary is the app's account of what happened; the error may only say that
// the context was cancelled, which is what Ctrl-C looks like.
func syncFailure(run domain.SyncRun, err error) error {
	switch {
	case err != nil && run.ErrorSummary != "" && run.ErrorSummary != err.Error():
		return fmt.Errorf("sync failed: %s: %w", run.ErrorSummary, err)
	case err != nil:
		return fmt.Errorf("sync failed: %w", err)
	case run.ErrorSummary != "":
		return fmt.Errorf("sync failed: %s", run.ErrorSummary)
	}
	return fmt.Errorf("sync ended as %s", run.Status)
}

// runExport writes a .trovebundle backup.
func runExport(args []string) error {
	flags := flag.NewFlagSet("export", flag.ContinueOnError)
	dataDir := flags.String("data-dir", "", "read another data directory")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return errors.New("usage: trove export <file>")
	}
	target := flags.Arg(0)

	ctx, stop := signalContext()
	defer stop()

	library, err := openLibrary(*dataDir)
	if err != nil {
		return err
	}
	defer library.close()

	// An unconfigured app exports sync history by default, the same as the export panel.
	service := importexport.New(library.repos, ports.SystemClock{}, appVersion())
	// Write beside the target and rename, so an interrupted export never leaves a
	// half-written bundle where the previous good one was. The bundle is streamed
	// into the temporary file rather than built in memory first: a full library can
	// be much larger than the process should hold.
	temporary, err := os.CreateTemp(filepath.Dir(target), ".trovebundle-*")
	if err != nil {
		return err
	}
	defer os.Remove(temporary.Name())
	if err := service.ExportTo(ctx, temporary, importexport.DefaultExportOptions()); err != nil {
		temporary.Close()
		return fmt.Errorf("export: %w", err)
	}
	info, err := temporary.Stat()
	if err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporary.Name(), target); err != nil {
		return err
	}
	fmt.Printf("wrote %s (%s)\n", target, humanBytes(info.Size()))
	return nil
}

// runAuth signs in through the browser and files the session in the keychain.
//
// The window's settings panel does the same read, but it can only read a browser that
// is already signed in. The command is the reader with the missing half in front of it:
// open X's sign-in page, wait for the cookies to land, save them. No window, no
// database -- a session is the keychain's, and `trove sync` is what proves it works.
func runAuth(args []string) error {
	flags := flag.NewFlagSet("auth", flag.ContinueOnError)
	timeout := flags.Duration("timeout", 5*time.Minute, "how long to wait for the sign-in")
	noBrowser := flags.Bool("no-open", false, "do not open the browser")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("auth takes no arguments, got %q", flags.Arg(0))
	}

	ctx, stop := signalContext()
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()

	open := func() error {
		fmt.Println("no X session in any browser yet")
		if !*noBrowser {
			fmt.Printf("opening %s\n", signInURL)
			if err := openWebURL(signInURL); err != nil {
				return fmt.Errorf("could not open a browser: %w\nopen %s yourself, or run trove auth -no-open", err, signInURL)
			}
		}
		fmt.Println("sign in to X in the browser; waiting for the session… (Ctrl-C to stop)")
		return nil
	}

	changed, err := credentials.NewWithKeychain().SignInFromBrowser(ctx, credentials.SignInInterval, open)
	switch {
	case errors.Is(err, credentials.ErrBrowserPermission):
		// The wait stops here, so the fix has to be on the line itself: the profile
		// directory is behind a privacy setting, not behind a sign-in the browser has not
		// finished yet.
		fmt.Fprintln(os.Stderr, "trove auth: this terminal cannot read the browser profiles.")
		fmt.Fprintln(os.Stderr, "Grant it Full Disk Access (System Settings › Privacy & Security) and run trove auth again.")
		return err
	case errors.Is(err, credentials.ErrSignInTimedOut):
		return fmt.Errorf("gave up after %s: %w", *timeout, err)
	case errors.Is(err, credentials.ErrSignInStopped):
		return err
	case err != nil:
		return fmt.Errorf("auth: %w", err)
	case changed:
		fmt.Println("saved the X session (auth_token, ct0) to the keychain")
	default:
		fmt.Println("the stored X session already matches this browser; nothing changed")
	}
	return nil
}

// library is the storage half of the composition root, for the commands that need no
// window, no tagger and no model client.
type library struct {
	db    *sqlite.DB
	repos ports.Repositories
	store *credentials.Store
}

func openLibrary(dataDir string) (*library, error) {
	dir, err := dataDirectory(dataDir)
	if err != nil {
		return nil, err
	}
	db, err := sqlite.Open(filepath.Join(dir, databaseFile))
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", dir, err)
	}
	repos := ports.Repositories{
		Bookmarks:   sqlite.NewBookmarkRepository(db),
		Tags:        sqlite.NewTagRepository(db),
		Lists:       sqlite.NewListRepository(db),
		Suggestions: sqlite.NewSuggestionRepository(db),
		Analysis:    sqlite.NewAnalysisRepository(db),
		Settings:    sqlite.NewSettingsRepository(db),
		SavedViews:  sqlite.NewSavedViewRepository(db),
		SyncRuns:    sqlite.NewSyncRunRepository(db),
		Revision:    db,
	}
	return &library{db: db, repos: repos, store: credentials.NewWithKeychain()}, nil
}

func (l *library) close() error { return l.db.Close() }

// syncer is the same client the window syncs with: credentials are read from the
// system vault per request, so a re-authenticated session applies to this run.
func (l *library) syncer() *syncer.Service {
	client := xsource.NewClient(
		xsource.NewQueryCatalog(""),
		func(ctx context.Context) (xsource.Credentials, error) {
			return l.store.XCredentialsForSync(ctx)
		},
	)
	client.CaptureRawPayload = true

	service := syncer.New(l.repos, ports.SystemClock{}, idgen.New("trove"), client)
	// An expired session heals itself from the browser's live cookies, which is what
	// makes an unattended nightly run survive a token that rolled over since yesterday.
	service.UseCredentialRefresher(l.store.RefreshXCredentialsFromBrowser)
	return service
}

// dataDirectory resolves the directory the window uses, with the same override the app
// honours: TROVE_DATA_DIR, or -data-dir for one command.
func dataDirectory(override string) (string, error) {
	if override != "" {
		return override, os.MkdirAll(override, 0o755)
	}
	if dir := os.Getenv("TROVE_DATA_DIR"); dir != "" {
		return dir, os.MkdirAll(dir, 0o755)
	}
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return "", err
	}
	return dir, nil
}

// signalContext cancels on Ctrl-C and on the scheduler's termination signal, so a run
// interrupted at 3 a.m. is recorded as interrupted rather than left in flight.
func signalContext() (context.Context, func()) {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	return ctx, cancel
}

// humanBytes prints a size the way a person reads it.
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for n/div >= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
