// Command trove is the desktop app: a local-first X bookmark manager whose
// interface MyGo draws itself. No webview, no HTML, no JavaScript.
//
// The layers below this file are independent of it:
//
//	domain   pure rules, no dependencies
//	ports    the interfaces the usecases need
//	usecase  workspace, tagging, syncer, suggestions
//	adapter  storage/sqlite, xsource, llm, credentials
//	ui       the MyGo workspace, which talks only to usecases
//
// This file is the composition root: the one place that knows which concrete
// implementations are in play. Swapping the storage engine or the sync client is
// a change here and nowhere else.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"encoding/json"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	updatenative "github.com/egoist/mygo/plugins/updater/native"
	"github.com/egoist/mygo/ui"
	engine "github.com/taoquo/trove/internal/autotag"
	"github.com/taoquo/trove/internal/credentials"

	"github.com/taoquo/trove/internal/i18n"
	"github.com/taoquo/trove/internal/idgen"
	"github.com/taoquo/trove/internal/importexport"
	"github.com/taoquo/trove/internal/llm"
	"github.com/taoquo/trove/internal/media"
	"github.com/taoquo/trove/internal/ports"
	"github.com/taoquo/trove/internal/storage/sqlite"
	appui "github.com/taoquo/trove/internal/ui"
	"github.com/taoquo/trove/internal/usecase/settings"
	"github.com/taoquo/trove/internal/usecase/suggestions"
	syncer "github.com/taoquo/trove/internal/usecase/syncer"
	tagging "github.com/taoquo/trove/internal/usecase/tagging"
	"github.com/taoquo/trove/internal/usecase/workspace"
	"github.com/taoquo/trove/internal/xsource"
)

// databaseFile is the SQLite file inside the per-user data directory.
const databaseFile = "trove.db"

// mediaCacheDir is where fetched pictures live, under the app's data directory.
const mediaCacheDir = "media-cache"

func main() {
	// `trove sync` and `trove export` run without a window: they are what a scheduler
	// and a terminal use, and opening a window for them would be a bug. Anything else
	// (including macOS's own launch argument) means the app.
	if handled, err := runCommand(os.Args[1:]); handled {
		if err != nil {
			fmt.Fprintln(os.Stderr, "trove:", err)
			os.Exit(1)
		}
		return
	}

	// Every window here is native UI, so the update window is too: no webview, and
	// it is the same window MyGo gives apps on every platform. Development builds
	// and builds without `updates` in mygo.json simply cannot update themselves.
	mygo.Use(updatenative.Plugin)

	mygo.App.WhenReady(func() {
		if err := start(); err != nil {
			// A failure here is fatal: without a database there is no library, and
			// showing an empty window would look like the data was lost.
			log.Fatal(err)
		}
	})

	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

// appVersion is the version the app stamps into what it writes. The bundle records it
// beside the data, so an exporter reading an old backup can tell what produced it.
// Outside a bundle -- `go run`, a test binary -- MyGo knows no version, and "dev" is
// more honest there than a number copied into the source and forgotten.
func appVersion() string {
	if v := mygo.App.Version(); v != "" {
		return v
	}
	return "dev"
}

// start opens storage, builds the usecases and shows the workspace.
func start() error {
	// The interface is rendered from two catalogs, and a hole in one shows a raw
	// key to the user. That is a build defect, not a runtime condition, so it is
	// asserted at startup rather than left to a test that may not run.
	if err := i18n.AssertComplete(); err != nil {
		return err
	}

	dataDir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return err
	}
	// TROVE_DATA_DIR points the app at another data directory, and exists because the empty-state
	// baseline needs a database with no bookmarks in it. The seeded database has rows, and the only
	// way to get zero was to move the user's database aside -- which a capture has no business
	// doing.
	//
	// The same shape as TROVE_CAPTURE_HEIGHT, and for the same reason: a screenshot sometimes needs
	// an input the running app cannot be asked for.
	if dir := os.Getenv("TROVE_DATA_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		dataDir = dir
	}

	db, err := sqlite.Open(filepath.Join(dataDir, databaseFile))
	if err != nil {
		return err
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
		// The sidebar aggregates are cached against the database's own write
		// counter, so the cache cannot outlive a write whatever path made it.
		Revision: db,
	}

	clock := ports.SystemClock{}
	ids := idgen.New("trove")
	ctx := context.Background()
	credentialStore := credentials.NewWithKeychain()

	// Startup self-heal. A sync that was in flight when the app last
	// stopped cannot still be running: the process that owned it is gone, and
	// leaving the row would tell the user a sync is progressing when nothing is.
	//
	// A failure here is not fatal: it is bookkeeping, and the app is usable
	// without it.
	// The X client is built here and nowhere else: it is the only thing that knows
	// the wire protocol, and the syncer only knows the PageSource it needs.
	//
	// Credentials are read per request from the Keychain rather than captured at
	// startup, so re-authenticating takes effect without a relaunch.
	xClient := xsource.NewClient(
		xsource.NewQueryCatalog(""),
		func(ctx context.Context) (xsource.Credentials, error) {
			return credentialStore.XCredentials(ctx)
		},
	)
	// Capture every page's raw JSON; whether it is *stored* is the sync
	// preference's decision. Capturing only when the setting is on would put the
	// switch in two places, and the cost while off is one page's JSON in memory.
	xClient.CaptureRawPayload = true

	syncService := syncer.New(repos, clock, ids, xClient)
	// An expired session heals itself: when a run dies before its first page with an
	// auth failure, the browser's live cookies replace the stored pair and the run
	// retries once. The refresher reads the browser's live cookies, so the stored pair heals
	// without the reader re-entering anything.
	syncService.UseCredentialRefresher(credentialStore.RefreshXCredentialsFromBrowser)
	if healed, err := syncService.CleanupOrphanedRuns(ctx); err != nil {
		log.Printf("sync self-heal: %v", err)
	} else if healed > 0 {
		log.Printf("closed %d sync run(s) left in flight by a previous session", healed)
	}

	// The auto-tagger's half of the same heal: a persisted run state is always the
	// trace of a run that died, because the run cannot outlive its process.
	tagService := tagging.New(repos, clock, nil)
	// The model client is built per run from the settings and the Keychain, so a
	// changed provider, endpoint or key applies to the next run rather than the next
	// launch, and a missing key fails that run with a message instead of the app
	// refusing to start.
	resolveLLMKey := func(ctx context.Context, providerID string) (string, error) {
		credential, err := credentialStore.LLMCredential(ctx, providerID, true)
		if err != nil {
			return "", err
		}
		return credential.APIKey, nil
	}
	tagService.UseProviderClient(func(settings engine.Settings) (engine.LLMClient, error) {
		return engine.BuildProviderClient(settings, resolveLLMKey)
	})
	// The same resolver as the client build, so "test connection" tests the
	// configuration a run would actually use.
	tagService.UseConnectionTester(func(settings engine.Settings) engine.ConnectionTestResult {
		return engine.TestProviderConnection(settings, resolveLLMKey)
	})
	healed, err := tagService.HealStaleRun(ctx)
	if err != nil {
		log.Printf("auto-tag self-heal: %v", err)
	} else if healed {
		log.Print("closed an auto-tag run left in flight by a previous session")
	}

	// Credentials live only in the Keychain, never in the database.

	suggestionService := suggestions.New(repos, clock, ids)

	settingsService := settings.New(repos, clock)

	bundleService := importexport.New(repos, clock, appVersion())
	// The reset's confirmation promises that local settings are cleared, and the stored
	// secrets are part of that: the API key lives under its own account, so it is not
	// reached by clearing bookmarks, tags or settings.
	bundleService.UseSecretClearer(func(ctx context.Context) error {
		ids := make([]string, 0, len(llm.Providers))
		for _, provider := range llm.Providers {
			ids = append(ids, provider.ID)
		}
		return credentialStore.DeleteAll(ctx, ids)
	})
	// The export allowlist and its applier are the settings package's policy, so
	// it is injected rather than duplicated here.
	bundleService.ExportableSettings = func(ctx context.Context) (map[string]json.RawMessage, error) {
		exported, err := settingsService.Export(ctx)
		if err != nil {
			return nil, err
		}
		out := make(map[string]json.RawMessage, len(exported))
		for key, value := range exported {
			out[key] = json.RawMessage(value)
		}
		return out, nil
	}
	bundleService.RestoreSettings = func(ctx context.Context, incoming map[string]json.RawMessage) error {
		for key, value := range incoming {
			if !settings.IsExportable(key) {
				// A bundle may be hostile; only keys this build would have exported
				// are applied.
				continue
			}
			if err := repos.Settings.Set(ctx, key, string(value), clock.Now()); err != nil {
				return err
			}
		}
		return nil
	}
	workspaceService := workspace.New(repos, clock, ids)
	shell := appui.NewShell(workspaceService, appui.AppearanceSystem)
	// Opening a bookmark in the browser. The opener, its platform branches and its
	// web-only rule live in openurl.go, where `trove auth` uses the same ones.
	shell.UseExternalOpener(openWebURL)
	shell.UseSettings(settingsService)
	shell.UseSuggestions(suggestionService)
	shell.UseSyncer(syncService)
	shell.UseTagging(tagService)
	shell.UseImportExport(bundleService)
	// The picture loader: a native build owns the fetch, the cache and the decode. The
	// cache is on disk so a picture
	// fetched on an earlier run is not downloaded again.
	mediaFetcher := media.NewFetcher()
	mediaFetcher.UseDiskCache(media.NewDiskCache(filepath.Join(dataDir, mediaCacheDir), media.DefaultDiskBudget))
	shell.UseMediaFetcher(mediaFetcher)
	// The video player. X ships an mp4 per video and the app stores it; this is the native
	// AVKit view that plays it (internal/ui/video_darwin.go). Without it the shell hands the
	// file to the system's player instead.
	shell.UseVideoPlayer(appui.NewNativeVideoPlayer())
	shell.UseCredentials(credentialStore)
	// The update card on Settings › General. It shows the version this bundle runs and when it
	// last checked, and its switch writes the plugin's own preference; the plugin is handed over
	// as functions rather than called here, because it starts listening when the app runs.
	shell.UseUpdates(appui.Updates{
		Version:            appVersion(),
		Check:              updater.CheckForUpdates,
		AutomaticChecks:    updater.AutomaticChecks,
		SetAutomaticChecks: updater.SetAutomaticChecks,
		LastCheck:          updater.LastCheck,
	})
	// The connection test the sync settings run. It is the same client the syncer uses, so a
	// green test is a statement about the exact path a sync will take: same cookies, same
	// transport, same proxy.
	shell.UseXProbe(xClient)
	// One Keychain read before the first frame. Whether the two X cookies exist decides what a
	// failed sync says -- "add them under Settings > Sync" or "the login expired" -- and the view
	// may not do I/O, so the answer is fetched here and refreshed again after a failure and when
	// the settings sheet opens.
	shell.RefreshXCookieStatus(ctx)
	// The menu bar is built from the resolved language, so it is rebuilt when the
	// user changes it rather than keeping the language the app started in.
	shell.OnLocaleChanged(func() { mygo.App.SetMenu(buildMenu(shell)) })

	// The stored preferences are applied before the first frame, so the window
	// never appears in the wrong theme and then flips.
	if err := shell.ApplyPreferences(ctx); err != nil {
		log.Printf("could not load preferences, using defaults: %v", err)
	}

	// A screenshot drawn in memory, before any window exists.
	//
	// The capture path used to open a window and photograph it, which took focus on the machine
	// doing the capturing -- and tools/baselines.sh does eight of them in a row. ui.Render draws
	// the same view without a window, so the app never appears. TROVE_HEADLESS asks for it;
	// without it the window path below is unchanged.
	if path := os.Getenv("TROVE_CAPTURE"); path != "" && os.Getenv("TROVE_HEADLESS") != "" {
		shell.Reload(ctx)
		applyCaptureState(shell)
		headlessCapture(shell, path)
	}

	window := mygo.NewWindow(mygo.WindowOptions{
		Title:  "trove",
		Width:  windowWidthFromEnv(),
		Height: windowHeightFromEnv(),
		// The stylesheet sets min-height: 100dvh on its shells, which is "fill the viewport" and
		// not a floor, so no window minimum follows from it. MinWidth was at 900 and MinHeight
		// at 560, which was a number nobody had chosen.
		MinWidth:  900,
		MinHeight: 620,
		// The window's chrome is the app's own.
		//
		// The app draws its header instead of taking the system's title bar, and this is how
		// MyGo does it: the system bar is hidden, the traffic lights stay, and the view's
		// first row is a 50-point header that clears them and drags the window. HiddenInset
		// rather than Hidden keeps the window's rounded corners and its usual shadow, which
		// are what make a Mac window look like one.
		//
		// The lights are placed so they are centred in that 50-point row rather than in a
		// system title bar the user cannot see: 12 points of button with 19 above and below.
		// The view reads where they ended up from Context.TitleBar, so the button and the
		// brand stay clear of them without a hardcoded inset.
		TitleBarStyle:        mygo.TitleBarHiddenInset,
		TrafficLightPosition: &mygo.Point{X: 20, Y: 19},
		// Restores the window where the user left it. Not when a screenshot asked for a width:
		// the restored size wins over the requested one, so asking for 1500 and getting the
		// saved 1180 is exactly what happened the first time tools/panes.sh tried to photograph
		// the wide layout -- the capture came out 2360 px and the boundary was off the image.
		StateKey: windowStateKey(),
		// A hidden window when the smoke test asked for one.
		//
		// tools/smoke.sh launches the binary for real -- that is the whole point of it: the pid it
		// finds has to be this build's, the app has to be single-instance, and the quit path has to
		// run. What it does not need is the window on screen, and for a while that is what it got:
		// every gate run put a window in front of whoever was using the machine. Hidden still
		// creates the window and runs NSApplication, the menu bar and the sync runner, so all four
		// assertions mean what they meant, and nothing appears.
		Hidden:  os.Getenv("TROVE_SMOKE") != "",
		Content: ui.View(shell.View),
	})
	shell.Attach(window)
	closeQuits(window)
	// The smoke run can press the red button's path for real: Window.Close fires the same
	// OnClose the button does, so a build whose close handler stopped quitting fails the
	// check instead of quietly passing it. Nothing but tools/smoke.sh sets this.
	if delay := smokeCloseDelay(); delay > 0 {
		time.AfterFunc(delay, window.Close)
	}
	captureForReview(window, shell)

	// The first read happens after the window exists, so a slow query cannot delay
	// the window appearing.
	mygo.App.SetMenu(buildMenu(shell))

	shell.Reload(ctx)

	// A run the app interrupted is continued when the user's settings still ask for it, as
	// it is at startup. It is gated like every other automatic network
	// path: a capture and the smoke run are deterministic fixtures.
	if healed && os.Getenv("TROVE_CAPTURE") == "" && os.Getenv("TROVE_SMOKE") == "" {
		if shell.ResumeInterruptedAutoTag(ctx) {
			log.Print("continuing the auto-tag run the previous session interrupted")
		}
	}

	// "Sync when the app opens" was stored, shown and read by nothing. Capture
	// and the smoke run must not touch the network: both are deterministic
	// fixtures, and a sync would rewrite the database under them.
	if shell.State().SyncOnLaunch && os.Getenv("TROVE_CAPTURE") == "" && os.Getenv("TROVE_SMOKE") == "" {
		shell.StartSync(ctx)
	}

	// Then the state a screenshot asked for, if one did.
	//
	// Here and not in captureForReview's goroutine: the content is built once on this path,
	// and it is built after start returns, so a state applied now is in place for that build
	// while one applied a moment later is not; the order was traced and there is exactly one
	// build.
	applyCaptureState(shell)

	window.Invalidate()

	return nil
}

// closeQuits makes the window's red button quit the app.
//
// The first version hid the window and kept the app running, so that a stray close could not
// stop a sync in flight. It read as a broken button: MyGo prevents a close by leaving the window
// exactly where it was, so the click did nothing visible, and a reader who wanted out had to
// find Quit in the menu. A single-window app whose window is the whole interface has one answer
// for the red button, and this is it.
//
// The handler still has to tell the two closes apart. MyGo asks before terminating and then
// closes every window; a handler that prevented that close too would cancel the quit, and Cmd-Q
// would report "User canceled. (-128)" while nothing happened. quitting is set by the quit
// sequence before it walks the windows, so the same handler lets the quitting close through.
func closeQuits(window *mygo.Window) {
	// Windows closes its last window by quitting, and has no Dock icon to bring a hidden
	// window back, so there is nothing to change there.
	if runtime.GOOS != "darwin" {
		return
	}

	quitting := false
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { quitting = true })
	window.OnClose(func(event *mygo.CloseEvent) {
		if quitting {
			return
		}
		// The close is postponed rather than performed: Quit runs the app's own quit sequence,
		// which walks this window again with quitting set and lets that close through.
		event.PreventDefault()
		mygo.App.Quit()
	})
}

// smokeCloseDelay reads the delayed window close tools/smoke.sh asks for, and reports zero
// unless both the smoke run and a positive duration are in the environment.
//
// A close that asked for itself is the only way to test the red button without a pointer: the
// checker cannot press it (macOS wants assistive access for that), and Window.Close is the same
// close event. The delay leaves the script time to see the process alive first, so "gone after
// the close" means the close did it rather than a crash at launch.
func smokeCloseDelay() time.Duration {
	if os.Getenv("TROVE_SMOKE") == "" {
		return 0
	}
	delay, err := time.ParseDuration(os.Getenv("TROVE_SMOKE_CLOSE"))
	if err != nil || delay <= 0 {
		return 0
	}
	return delay
}

// buildMenu assembles the menu bar.
//
// The labels come from the shell's translator, so the menu follows the interface's
// language. Items for features that do not exist yet are disabled rather than
// omitted: a menu that grows and shrinks between versions is harder to learn than
// one where a command is visibly unavailable.
// checkForUpdatesItem opens MyGo's update window in the interface's language. The
// plugin ships its own item, but its label is fixed when the item is built and it has
// no id; this one is rebuilt with the rest of the bar on a language change.
func checkForUpdatesItem(shell *appui.Shell) *mygo.MenuItem {
	return &mygo.MenuItem{
		ID: "check-updates", Label: shell.T(i18n.MenuCheckUpdates),
		Click: func(*mygo.MenuItem, *mygo.Window) { updater.CheckForUpdates() },
	}
}

func buildMenu(shell *appui.Shell) *mygo.Menu {
	return mygo.NewMenu([]*mygo.MenuItem{
		{
			Label: mygo.App.Name(),
			Submenu: []*mygo.MenuItem{
				{Role: mygo.RoleAbout},
				checkForUpdatesItem(shell),
				{Type: mygo.MenuItemSeparator},
				{Role: mygo.RoleQuit},
			},
		},
		{
			Label: shell.T(i18n.MenuFile),
			Submenu: []*mygo.MenuItem{
				{
					ID: "sync", Label: shell.T(i18n.ActionSync), Accelerator: "CmdOrCtrl+R",
					Click: func(*mygo.MenuItem, *mygo.Window) { shell.StartSync(context.Background()) },
				},
				{
					ID: "autotag", Label: shell.T(i18n.ActionRunAutotag), Accelerator: "CmdOrCtrl+Shift+T",
					Click: func(*mygo.MenuItem, *mygo.Window) { shell.StartAutoTag(context.Background()) },
				},
				{Type: mygo.MenuItemSeparator},
				{
					ID: "review", Label: shell.T(i18n.ActionReview), Accelerator: "CmdOrCtrl+Shift+S",
					Click: func(*mygo.MenuItem, *mygo.Window) { shell.OpenSuggestions(context.Background()) },
				},
				{Type: mygo.MenuItemSeparator},
				{
					// This app has no page, so it needs one entry point for saving a link by
					// hand -- and the sidebar header is not the place for it: its section head
					// carries the create-tag "+", the tag-manager `table` and the fold caret,
					// three controls and no fourth.
					ID: "add-bookmark", Label: shell.T(i18n.MenuAddBookmark), Accelerator: "CmdOrCtrl+N",
					Click: func(*mygo.MenuItem, *mygo.Window) { shell.ToggleAddForm() },
				},
				{Type: mygo.MenuItemSeparator},
				{
					ID: "export", Label: shell.T(i18n.MenuExport), Accelerator: "CmdOrCtrl+E",
					Click: func(*mygo.MenuItem, *mygo.Window) { _ = shell.ExportBundle(context.Background()) },
				},
				{
					ID: "import", Label: shell.T(i18n.MenuImport), Accelerator: "CmdOrCtrl+O",
					Click: func(*mygo.MenuItem, *mygo.Window) { _ = shell.AskBundleImport(context.Background(), false) },
				},
				{
					ID: "restore", Label: shell.T(i18n.BundleRestoreTitle),
					Click: func(*mygo.MenuItem, *mygo.Window) { _ = shell.AskBundleImport(context.Background(), true) },
				},
				{Type: mygo.MenuItemSeparator},
				{
					// This action belongs beside "Import restore", behind a
					// confirmation. It was missing here: the cascade existed only as the
					// first half of a restore, and no checker looked in this layer.
					ID: "reset", Label: shell.T(i18n.ResetLocalData),
					Click: func(*mygo.MenuItem, *mygo.Window) { shell.AskResetLocalData() },
				},
			},
		},
		{
			Label: shell.T(i18n.MenuEdit),
			Submenu: []*mygo.MenuItem{
				{Role: mygo.RoleUndo}, {Role: mygo.RoleRedo},
				{Type: mygo.MenuItemSeparator},
				{Role: mygo.RoleCut}, {Role: mygo.RoleCopy}, {Role: mygo.RolePaste},
				{Role: mygo.RoleSelectAll},
			},
		},
		{
			Label: shell.T(i18n.MenuView),
			Submenu: []*mygo.MenuItem{
				{
					ID: "appearance-system", Label: shell.T(i18n.AppearanceSystem),
					Type: mygo.MenuItemRadio, Checked: shell.State().Appearance == appui.AppearanceSystem,
					Click: func(*mygo.MenuItem, *mygo.Window) { shell.SetAppearance(appui.AppearanceSystem) },
				},
				{
					ID: "appearance-light", Label: shell.T(i18n.AppearanceLight),
					Type: mygo.MenuItemRadio, Checked: shell.State().Appearance == appui.AppearanceLight,
					Click: func(*mygo.MenuItem, *mygo.Window) { shell.SetAppearance(appui.AppearanceLight) },
				},
				{
					ID: "appearance-dark", Label: shell.T(i18n.AppearanceDark),
					Type: mygo.MenuItemRadio, Checked: shell.State().Appearance == appui.AppearanceDark,
					Click: func(*mygo.MenuItem, *mygo.Window) { shell.SetAppearance(appui.AppearanceDark) },
				},
				{Type: mygo.MenuItemSeparator},
				{
					// The app's own binding for the sidebar: Control-Command-S.
					// The item is where the shortcut is discoverable, and a
					// menu key equivalent is dispatched before the window sees the key.
					ID: "toggle-sidebar", Label: shell.T(i18n.ActionToggleSidebar),
					Accelerator: "Ctrl+Cmd+S",
					Click:       func(*mygo.MenuItem, *mygo.Window) { shell.ToggleSidebar() },
				},
				{Type: mygo.MenuItemSeparator},
				{
					// Cmd+F is the search shortcut every macOS user already knows,
					// and the spec requires it to work rather than to be documented.
					ID: "focus-search", Label: shell.T(i18n.ActionFocusSearch), Accelerator: "CmdOrCtrl+F",
					Click: func(*mygo.MenuItem, *mygo.Window) { shell.FocusSearch() },
				},
				{Type: mygo.MenuItemSeparator},
				{
					ID: "settings", Label: shell.T(i18n.ActionSettings), Accelerator: "CmdOrCtrl+,",
					Click: func(*mygo.MenuItem, *mygo.Window) { shell.OpenSettings() },
				},
			},
		},
		{
			Label: shell.T(i18n.MenuWindow),
			Submenu: []*mygo.MenuItem{
				{Role: mygo.RoleMinimize}, {Role: mygo.RoleZoom},
				{Type: mygo.MenuItemSeparator},
				{Role: mygo.RoleClose},
			},
		},
	})
}

// windowWidthFromEnv is the window's opening width, overridable for screenshots.
//
// The three-column behaviour depends on the width -- the boundary is 1400 points -- and a
// screenshot that cannot vary it can only ever check one side of that boundary. tools/panes.sh
// sets TROVE_CAPTURE_WIDTH so the same build can be photographed wide and narrow.
// windowStateKey is the key the window's place is saved under, or empty when a screenshot asked
// for a width and the saved place would overrule it.
func windowStateKey() string {
	if os.Getenv("TROVE_CAPTURE_WIDTH") != "" {
		return ""
	}
	return "workspace"
}

// windowHeightFromEnv is the height a screenshot asks for, and 760 otherwise.
//
// It exists for the same reason windowWidthFromEnv does, and it was needed to test one claim:
// a Modal's panel is capped at the window's height less ten theme spaces (mygo
// ui/widgets.go coreModal). At 760 with the theme's default spacing of 4 that cap is 720 points,
// and the settings body measures about 750 -- so the last line does not fit and the panel cannot
// grow. Raising the window by 40 points should therefore make the line appear, and that is what
// this is here to check.
func windowHeightFromEnv() int {
	if value := os.Getenv("TROVE_CAPTURE_HEIGHT"); value != "" {
		if height, err := strconv.Atoi(value); err == nil && height > 0 {
			return height
		}
	}
	return 760
}

func windowWidthFromEnv() int {
	const fallback = 1180
	raw := os.Getenv("TROVE_CAPTURE_WIDTH")
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed < 900 {
		return fallback
	}
	return parsed
}
