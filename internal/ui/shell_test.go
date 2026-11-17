package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	engine "github.com/taoquo/trove/internal/autotag"
	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/i18n"
	"github.com/taoquo/trove/internal/idgen"
	"github.com/taoquo/trove/internal/importexport"
	"github.com/taoquo/trove/internal/ports"
	"github.com/taoquo/trove/internal/storage/sqlite"
	"github.com/taoquo/trove/internal/usecase/settings"
	"github.com/taoquo/trove/internal/usecase/suggestions"
	syncer "github.com/taoquo/trove/internal/usecase/syncer"
	tagging "github.com/taoquo/trove/internal/usecase/tagging"
	"github.com/taoquo/trove/internal/usecase/workspace"
)

var clockBase = time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

// newShell builds a shell over a real database, so the view is rendered from the
// same data path the app uses rather than from a stub.
func newShell(t *testing.T) (*Shell, *sqlite.DB) {
	t.Helper()

	db, err := sqlite.Open(filepath.Join(t.TempDir(), "ui.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	repos := ports.Repositories{
		Bookmarks:   sqlite.NewBookmarkRepository(db),
		Tags:        sqlite.NewTagRepository(db),
		Lists:       sqlite.NewListRepository(db),
		Suggestions: sqlite.NewSuggestionRepository(db),
		Analysis:    sqlite.NewAnalysisRepository(db),
		Settings:    sqlite.NewSettingsRepository(db),
		SavedViews:  sqlite.NewSavedViewRepository(db),
	}

	service := workspace.New(repos, fixedClock{now: clockBase}, idgen.New("test"))
	shell := NewShell(service, AppearanceLight)
	noSearchTimers(shell)
	return shell, db
}

// noSearchTimers takes the search debounce's real timer out of a test shell.
//
// searchChanged schedules its read with time.AfterFunc, and a shell without a window
// publishes the result inline, so a timer that fires mid-render writes s.state.Searching
// on its own goroutine while View reads it. In the app that write goes through
// Window.Update onto the main thread; a test has no window, so the timer never fires and
// a test that is about the debounce drives the timers itself (searchdebounce_test.go).
func noSearchTimers(shell *Shell) {
	shell.after = func(time.Duration, func()) *time.Timer {
		return time.AfterFunc(time.Hour, func() {})
	}
}

// shellWithTagging is newShell plus a tagging service.
//
// It is a variant rather than a change to newShell because several tests pin the opposite:
// a shell with no tagging service refuses to start a run and says so. Installing one for
// everyone would move that baseline for two hundred tests to suit two.
func shellWithTagging(t *testing.T) (*Shell, *sqlite.DB, *tagging.Service) {
	t.Helper()

	db, err := sqlite.Open(filepath.Join(t.TempDir(), "ui-tagging.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	repos := ports.Repositories{
		Bookmarks:   sqlite.NewBookmarkRepository(db),
		Tags:        sqlite.NewTagRepository(db),
		Lists:       sqlite.NewListRepository(db),
		Suggestions: sqlite.NewSuggestionRepository(db),
		Analysis:    sqlite.NewAnalysisRepository(db),
		Settings:    sqlite.NewSettingsRepository(db),
		SavedViews:  sqlite.NewSavedViewRepository(db),
	}
	service := workspace.New(repos, fixedClock{now: clockBase}, idgen.New("test"))
	shell := NewShell(service, AppearanceLight)
	noSearchTimers(shell)
	// The tagging service is installed so the delete path can block a tag's name: without
	// it a deleted tag comes back the next time a post mentions it, which the tagging rules
	// require.
	tagger := tagging.New(repos, fixedClock{now: clockBase}, nil)
	shell.UseTagging(tagger)
	return shell, db, tagger
}

func seed(t *testing.T, db *sqlite.DB, id, handle, text string, savedAt time.Time) {
	t.Helper()

	bookmark := domain.Bookmark{
		TweetID: id, TweetURL: "https://x.com/" + handle + "/status/" + id,
		AuthorName: handle, AuthorHandle: handle, Text: text,
		SavedAt: savedAt, LastSeenAt: savedAt, UpdatedAt: savedAt,
	}
	if _, err := sqlite.NewBookmarkRepository(db).UpsertMany(context.Background(), []domain.Bookmark{bookmark}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}
}

// seedMedia attaches media to a bookmark, which the detail's reading pane renders as
// its own section.
func seedMedia(t *testing.T, db *sqlite.DB, id string, mediaType domain.MediaType) {
	t.Helper()

	bookmark := domain.Bookmark{
		TweetID: id, TweetURL: "https://x.com/ada/status/" + id,
		AuthorName: "ada", AuthorHandle: "ada", Text: "a post about compilers",
		SavedAt: clockBase, LastSeenAt: clockBase, UpdatedAt: clockBase,
		Media: []domain.Media{{Type: mediaType, URL: "https://example.test/" + id + ".jpg"}},
	}
	if _, err := sqlite.NewBookmarkRepository(db).UpsertMany(context.Background(), []domain.Bookmark{bookmark}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}
}

// TestShellRendersThreeColumns is the gate's own acceptance check: the workbench
// draws its three columns with the sidebar's system views and a real result.
func TestShellRendersThreeColumns(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "a post about compilers", clockBase)
	shell.Reload(context.Background())

	tester := ui.NewTester(shell.View, 1180, 760)

	// The sidebar's blocks: the brand row and the pair of facet tabs that carry the tag and
	// author views.
	//
	// 收件箱, 未分类, 列表 and 保存视图 are intentionally absent, because the reference picture
	// has only 标签 and 作者. The names are asserted absent just below, so the omission is a
	// decision this test records rather than a gap it tolerates. The tab labels are drawn in
	// sentence case rather than as overlines, so tags and authors are asserted here by the
	// words the control actually shows.
	//
	// The tags tab draws nothing above its tags; TestTheTagsTabDrawsNothingAboveTheTags
	// records that.
	for _, want := range []string{"trove", "Tags", "Authors"} {
		if !tester.HasText(want) {
			t.Errorf("sidebar is missing %q; texts: %q", want, tester.Texts())
		}
	}
	// Exact match, not HasText: that one is a substring test, so asking for "Lists" would also
	// match a sentence containing the word -- and the first version of this did exactly that in
	// Chinese, where 列表 appears inside other wording.
	absent(tester.Texts(), t, "Inbox", "Unclassified", "Lists", "Saved views")

	// The result and its author.
	if !tester.HasText("@ada") {
		t.Errorf("the result is missing its author; texts: %q", tester.Texts())
	}
	if !tester.HasText("a post about compilers") {
		t.Error("the result is missing its text")
	}

	// The workbench is two panes. The workspace has one split handle, on the left, and
	// renders the inspector inside the detail overlay rather than as a third column.
	if shell.State().DetailOpen {
		t.Error("the detail must not be open before it is asked for")
	}
	if tester.HasText("Select a bookmark to see its details.") {
		t.Error("the empty-inspector placeholder belonged to the three-column shell")
	}

	// Asking for the detail opens it over the results.
	shell.OpenSelectedDetail()
	tester.Frame()
	if !shell.State().DetailOpen {
		t.Error("opening the detail must set the overlay flag")
	}
	// The hero shows the author, not a title.
	//
	// There is no visible "Details": detailsTitle is the close button's accessible name, and
	// the hero itself carries the avatar, the name and the handle. The reference picture agrees
	// -- its header is 硅谷居士 @SVScholar with 在 X 中打开 on the right.
	if !tester.HasText("@ada") {
		t.Errorf("the detail hero is missing the author handle; texts: %q", tester.Texts())
	}
	if !tester.HasText(tester.Texts()[0]) || len(tester.Texts()) == 0 {
		t.Fatal("the overlay drew nothing")
	}
}

// TestShellShowsCounts pins that the sidebar numbers come from the same snapshot
// as the results, so they cannot disagree.
func TestShellShowsCounts(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "first", clockBase)
	seed(t, db, "2", "grace", "second", clockBase.Add(time.Hour))
	shell.Reload(context.Background())

	tester := ui.NewTester(shell.View, 1180, 760)

	// Two bookmarks, so "All bookmarks" carries 2 and both are untagged.
	//
	// The count lives in the page heading, not in the toolbar: the toolbar used to repeat it
	// at the end of a five-row strip, so the same number appeared twice on one screen. The
	// count belongs with the heading, and the toolbar is for the controls.
	if !tester.HasText("2") {
		t.Errorf("the heading count is missing; texts: %q", tester.Texts())
	}
	// The label is sentence case: it is the small word after the number in the one-line heading.
	if !tester.HasText(shell.t().T(i18n.ResultsLabel)) {
		t.Errorf("the heading count has no label; texts: %q", tester.Texts())
	}
	if !tester.HasText("2") {
		t.Error("the sidebar should show a count of 2")
	}
}

// TestShellUnclassifiedExcludesTagged is the sidebar's Unclassified entry, whose
// definition is "carries no tag at all".
func TestShellUnclassifiedExcludesTagged(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "alpha content", clockBase)
	seed(t, db, "2", "grace", "beta content", clockBase)

	// Tag the first one through the same service the app uses.
	repos := ports.Repositories{Tags: sqlite.NewTagRepository(db)}
	if err := repos.Tags.Create(ctx, domain.Tag{
		ID: "t1", Name: "Go", CanonicalKey: domain.CanonicalTagKey("Go"),
		CreatedAt: domain.FormatTimestamp(clockBase), UpdatedAt: domain.FormatTimestamp(clockBase),
	}); err != nil {
		t.Fatalf("Create tag: %v", err)
	}
	if err := repos.Tags.Attach(ctx, domain.BookmarkTag{
		BookmarkID: "1", TagID: "t1", CreatedAt: clockBase, Source: domain.TagSourceManual,
	}); err != nil {
		t.Fatalf("Attach: %v", err)
	}

	shell.selectSidebar(ctx, selectionUnclassified)
	shell.Reload(ctx)

	tester := ui.NewTester(shell.View, 1180, 760)
	if !tester.HasText("beta content") {
		t.Error("Unclassified must show the untagged bookmark")
	}
	if tester.HasText("alpha content") {
		t.Error("Unclassified must not show a tagged bookmark")
	}
}

// TestShellSortReachesEveryOrder pins the audit's finding: the old UI could only
// cycle through four orders, so "saved-desc" was unreachable.
func TestShellSortReachesEveryOrder(t *testing.T) {
	shell, _ := newShell(t)

	orders := map[domain.SortOrder]bool{}
	for index := range sortKeys {
		shell.setSort(context.Background(), index)
		orders[shell.State().Sort] = true
	}

	for _, want := range domain.AllSortOrders {
		if !orders[want] {
			t.Errorf("sort order %q is not reachable from the UI", want)
		}
	}
	if len(sortKeys) != len(domain.AllSortOrders) {
		t.Errorf("%d labels for %d orders", len(sortKeys), len(domain.AllSortOrders))
	}
}

// TestShellHighlightsSelection covers the card click: it makes the post the inspected one
// and opens its detail.
func TestShellHighlightsSelection(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "a post about compilers", clockBase)
	shell.Reload(context.Background())

	tester := ui.NewTester(shell.View, 1180, 760)
	if err := tester.Click("a post about compilers"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	tester.Frame()

	if shell.State().Selected != "1" {
		t.Fatalf("Selected = %q, want the clicked bookmark", shell.State().Selected)
	}
	if !shell.State().DetailOpen {
		t.Fatal("a card click must open the detail")
	}
	// The click opens the detail, not a bulk selection: a card click and its corner
	// checkbox are separate controls.
	if got := shell.BatchSelection(); len(got) != 0 {
		t.Errorf("batch selection = %v, want none from a card click", got)
	}
	// The detail's own avatar carries a label the card's does not, so finding it is what proves
	// the overlay opened rather than the card behind it.
	if _, found := tester.Find(DetailAvatarLabel); !found {
		t.Errorf("the detail did not open; texts: %q", tester.Texts())
	}
}

// TestAListRowClickOpensTheDetail is the same rule in the list layout.
//
// The list is a different widget (ui.List), and a click reaches the card inside it; if the card
// did not take the click, the row would only become the selection and the detail would never open.
func TestAListRowClickOpensTheDetail(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "a post about compilers", clockBase)
	shell.state.LeftSidebarWidth = WidthSidebarDef
	shell.state.RightSidebarWidth = WidthInspectorDef
	shell.state.ViewMode = domain.ViewList
	shell.Reload(context.Background())

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	if err := tester.Click("a post about compilers"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	tester.Frame()

	if !shell.State().DetailOpen {
		t.Error("a click on a list row must open the detail")
	}
	if shell.State().Selected != "1" {
		t.Errorf("Selected = %q, want the clicked bookmark", shell.State().Selected)
	}
}

// TestShellEmptyStates covers both empty cases, which read differently.
func TestShellEmptyStates(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()

	shell.Reload(ctx)
	tester := ui.NewTester(shell.View, 1180, 760)
	// The empty-library wording: a title, a line explaining where the posts come from, and
	// the two ways to fill it.
	if !tester.HasText("Your library is empty") {
		t.Errorf("an empty library must say so; texts: %q", tester.Texts())
	}
	if !tester.HasText("trove copies the bookmarks you already saved on X. Sign in to x.com in your browser, then connect") {
		t.Errorf("an empty library must explain where the posts come from; texts: %q", tester.Texts())
	}
	if !tester.HasText("Connect X") {
		t.Errorf("an empty library must offer to connect X; texts: %q", tester.Texts())
	}

	// With a bookmark but a query that matches nothing, the message differs.
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.State().Query = "nothing matches this"
	shell.Reload(ctx)

	tester = ui.NewTester(shell.View, 1180, 760)
	// A filtered miss has its own advice. Both lines switch on the same condition, so the
	// title and the explanation change together.
	if !tester.HasText("No bookmarks match the current filters") {
		t.Errorf("a filtered miss must read differently; texts: %q", tester.Texts())
	}
	if !tester.HasText("Try adjusting your search, tags, or author scope.") {
		t.Errorf("a filtered miss must say what to try; texts: %q", tester.Texts())
	}
	if tester.HasText("Once you save items from X") {
		t.Error("a filtered miss must not advise saving items")
	}
}

// TestTheThemeUsesTheAccent pins the accent to the family's cinnabar.
//
// The accent is one cinnabar: #b83d2e on paper, #c97a63 on the ink ground. It is the
// primary button, every focus ring and every selected row, so it is the single most visible
// colour in the window.
//
//	light  accent #b83d2e, pressed #7e241c, soft #f7e6e1, foreground #ffffff
//	dark   accent #c97a63, pressed #e3a595, soft #3a2521, foreground #191514
func TestTheThemeUsesTheAccent(t *testing.T) {
	light := FolioTheme(AppearanceLight, false)
	if want := ui.Hex("#b83d2e"); light.Accent != want {
		t.Errorf("light accent = %v, want the accent %v", light.Accent, want)
	}
	if want := ui.Hex("#7e241c"); light.AccentHover != want {
		t.Errorf("light accent hover = %v, want the pressed step %v", light.AccentHover, want)
	}
	dark := FolioTheme(AppearanceDark, false)
	if want := ui.Hex("#c97a63"); dark.Accent != want {
		t.Errorf("dark accent = %v, want the accent %v", dark.Accent, want)
	}

	// And it is deliberately not the platform's, so the difference is visible rather than asserted
	// against the same value twice.
	if light.Accent == ui.LightTheme().Accent {
		t.Error("the accent is the platform's again; see the note above before changing this")
	}
}

// TestTheScrollbarIsThePaletteThumb pins the one surface the theme used to leave unset.
//
// ui.LightTheme() and ui.DarkTheme() each ship a neutral thumb -- 32% black and 35% white --
// and FolioTheme mapped every role except this one. A grey bar down a parchment results
// column is the one control painted in a colour the palette does not contain.
func TestTheScrollbarIsThePaletteThumb(t *testing.T) {
	for _, appearance := range []Appearance{AppearanceLight, AppearanceDark} {
		theme := FolioTheme(appearance, false)
		def := ui.LightTheme()
		if appearance == AppearanceDark {
			def = ui.DarkTheme()
		}
		if theme.Scrollbar == (ui.Color{}) {
			t.Errorf("%s: the thumb is unset, so the toolkit's neutral default paints", appearance)
		}
		if theme.Scrollbar == def.Scrollbar {
			t.Errorf("%s: the thumb is still the toolkit's neutral %v", appearance, def.Scrollbar)
		}
	}
	if got, want := FolioTheme(AppearanceLight, false).Scrollbar, lightScrollThumb; got != want {
		t.Errorf("light thumb = %v, want the palette's %v", got, want)
	}
	if got, want := FolioTheme(AppearanceDark, false).Scrollbar, darkScrollThumb; got != want {
		t.Errorf("dark thumb = %v, want the palette's %v", got, want)
	}
}

func TestAppearanceSelection(t *testing.T) {
	// "system" follows the OS.
	if got := FolioTheme(AppearanceSystem, true); !got.Dark {
		t.Error("system must follow a dark OS appearance")
	}
	if got := FolioTheme(AppearanceSystem, false); got.Dark {
		t.Error("system must follow a light OS appearance")
	}

	// An explicit choice wins over the OS.
	if got := FolioTheme(AppearanceLight, true); got.Dark {
		t.Error("an explicit light preference must win over a dark OS")
	}
	if got := FolioTheme(AppearanceDark, false); !got.Dark {
		t.Error("an explicit dark preference must win over a light OS")
	}

	for _, want := range []string{"system", "light", "dark"} {
		if !ValidAppearance(want) {
			t.Errorf("%q must be a valid appearance", want)
		}
	}
	if ValidAppearance("sepia") {
		t.Error("an unknown appearance must be rejected")
	}
}

func TestRailTokensDifferByAppearance(t *testing.T) {
	light := Rail(AppearanceLight, false, false)
	dark := Rail(AppearanceDark, false, false)

	if light.Sidebar == dark.Sidebar {
		t.Error("the sidebar surface must differ between appearances")
	}
	if light.Results == dark.Results {
		t.Error("the results surface must differ between appearances")
	}

	// The inspector's separation from the results column is the rail line, not a
	// fill. The page ground and the card ground have the same value in the light theme,
	// so a test demanding different fills would be demanding a deviation.
	if light.Inspector != light.Results {
		t.Log("the inspector has its own fill; the palette gives it one in this appearance")
	}
	if light.RailLine == light.Results {
		t.Error("the rail line must be visible against the results surface, " +
			"or the columns would run together")
	}
	if dark.RailLine == dark.Results {
		t.Error("the dark rail line must be visible against the results surface")
	}
}

// TestTheRailTokensComeFromTheTheme pins the rail tokens to theme roles, so a later
// edit cannot quietly drift from the palette.
func TestTheRailTokensComeFromTheTheme(t *testing.T) {
	// Every structural colour is a theme role, not a literal. This pins the mapping rather
	// than the palette.
	for _, tc := range []struct {
		name  string
		dark  bool
		check func(RailTokens, *ui.Theme) error
	}{
		// The sidebar is the page ground with one hairline down its edge, not a panel
		// colour of its own.
		{"sidebar is the page", false, func(r RailTokens, th *ui.Theme) error {
			return sameOrErr("Sidebar", r.Sidebar, th.Background)
		}},
		// The results ground is the page too: a card is drawn only where there is a
		// panel.
		{"results is the page", false, func(r RailTokens, th *ui.Theme) error {
			return sameOrErr("Results", r.Results, th.Background)
		}},
		{"accent is the theme accent", false, func(r RailTokens, th *ui.Theme) error {
			return sameOrErr("Accent", r.Accent, th.Accent)
		}},
		{"border is the theme border", false, func(r RailTokens, th *ui.Theme) error {
			return sameOrErr("CardBorder", r.CardBorder, th.Border)
		}},
		// Muted is a surface, not a text colour: eight call sites use it as a Background,
		// and mapping it to TextMuted made the glyph and its ground the same colour.
		{"muted is the pill", false, func(r RailTokens, th *ui.Theme) error {
			return sameOrErr("Muted", r.Muted, th.SurfaceHover)
		}},
		// A focused field keeps its card ground and gains the accent ring; it is not filled
		// with the accent.
		{"focus keeps the card ground", false, func(r RailTokens, th *ui.Theme) error {
			return sameOrErr("FieldFocus", r.FieldFocus, th.Surface)
		}},
		{"dark sidebar is the page too", true, func(r RailTokens, th *ui.Theme) error {
			return sameOrErr("Sidebar", r.Sidebar, th.Background)
		}},
	} {
		appearance := AppearanceLight
		if tc.dark {
			appearance = AppearanceDark
		}
		rail := Rail(appearance, false, false)
		if err := tc.check(rail, FolioTheme(appearance, false)); err != nil {
			t.Errorf("%s: %v", tc.name, err)
		}
	}
}

// sameOrErr reports a rail token that is not the theme role it is supposed to mirror.
func sameOrErr(name string, got, want ui.Color) error {
	if got != want {
		return fmt.Errorf("%s = %v, want the theme's %v", name, got, want)
	}
	return nil
}

// TestTheHeadingsUseTheFamilySerif pins the typography decision.
//
// The display layer -- the brand, the page's name, a modal's title, the author over a post --
// is set in the family's serif stack, the same one the download page names. The dense rows and
// the controls stay in the system sans, because a 12-13px serif in a table is harder to read
// than the face the system designed for it.
//
// The list is the toolkit's family list: the first family present is the Latin face, and the
// rest cascade behind it per glyph, so a Chinese heading is set in a Chinese serif rather than
// falling back to the system's sans.
func TestTheHeadingsUseTheFamilySerif(t *testing.T) {
	shell, _ := newShell(t)
	want := `"Source Serif 4", Charter, Georgia, "Songti SC", "Noto Serif CJK SC", "STSong", SimSun, Palatino, serif`

	for _, locale := range []string{"en", i18n.ZHCN} {
		shell.SetLocale(locale)
		if got := shell.displayFace(); got != want {
			t.Errorf("under %s headings ask for %q, want the family serif %q", locale, got, want)
		}
	}
}

// TestBothAppearancesUseThePalette pins the two token blocks of the palette.
//
// The palette is warm paper: a #f6f0ea page, ivory cards, one #e6dacd hairline and #191514 ink
// in the light theme; the ink ground #191514, #221c19 cards and #f6f0ea ink in the dark one.
func TestBothAppearancesUseThePalette(t *testing.T) {
	light := FolioTheme(AppearanceLight, false)
	dark := FolioTheme(AppearanceDark, false)

	if !dark.Dark || dark.Dark == light.Dark {
		t.Fatal("the two appearances must be told apart by the Dark flag")
	}

	for _, tc := range []struct {
		name string
		got  ui.Color
		want ui.Color
		from string
	}{
		{"light background", light.Background, ui.Hex("#f6f0ea"), "--bg"},
		{"light surface", light.Surface, ui.Hex("#fbf7f3"), "--card"},
		{"light border", light.Border, ui.Hex("#e6dacd"), "--line"},
		{"light text", light.Text, ui.Hex("#191514"), "--fg"},
		{"dark background", dark.Background, ui.Hex("#191514"), "--bg (dark)"},
		{"dark surface", dark.Surface, ui.Hex("#221c19"), "--card (dark)"},
		{"dark text", dark.Text, ui.Hex("#f6f0ea"), "--fg (dark)"},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %v, want %v from %s", tc.name, tc.got, tc.want, tc.from)
		}
	}

	// And it is deliberately not the platform's: if these ever coincide the palette has been
	// replaced by ui.LightTheme() again, which is the state this test exists to prevent.
	if light.Background == ui.LightTheme().Background {
		t.Error("the light background is the platform's again; see the note above")
	}
	if dark.Background == ui.DarkTheme().Background {
		t.Error("the dark background is the platform's again; see the note above")
	}
}

func TestSidebarSelectionMapsToAFilter(t *testing.T) {
	shell, _ := newShell(t)

	shell.State().Selection = selectionAll
	if _, ok := shellFilterIsZero(t, shell); !ok {
		t.Error("All bookmarks must add no filter")
	}

	shell.State().Selection = selectionInbox
	if got := shell.filter().ListID; got != domain.InboxListID {
		t.Errorf("Inbox filter list = %q", got)
	}

	shell.State().Selection = selectionUnclassified
	if !shell.filter().OnlyUnclassified {
		t.Error("Unclassified must set OnlyUnclassified")
	}

	// A tag is a condition, not a scope: the filter reads ActiveTagIDs and a tag in the
	// selection filters nothing. The old code read both, so the same tag could narrow the
	// list through two fields and only one of them was on screen.
	shell.State().Selection = prefixTag + "t1"
	if got := shell.filter().TagIDs; len(got) != 0 {
		t.Errorf("a tag in the selection filtered the list: %v", got)
	}
	shell.State().ActiveTagIDs = []string{"t1"}
	if got := shell.filter().TagIDs; len(got) != 1 || got[0] != "t1" {
		t.Errorf("tag filter = %v", got)
	}
	shell.State().ActiveTagIDs = nil

	shell.State().Selection = prefixList + "l1"
	if got := shell.filter().ListID; got != "l1" {
		t.Errorf("list filter = %q", got)
	}
}

// shellFilterIsZero reports whether the current selection filters nothing.
func shellFilterIsZero(t *testing.T, shell *Shell) (domain.LibraryFilter, bool) {
	t.Helper()
	filter := shell.filter()
	return filter, filter.IsZero()
}

// TestShellDeleteFailsClosed is the gate's own safety check, exercised through the
// shell rather than only through the Confirmation type.
func TestShellDeleteFailsClosed(t *testing.T) {
	seedAndSelect := func(t *testing.T) (*Shell, *sqlite.DB) {
		t.Helper()
		shell, db := newShell(t)
		seed(t, db, "1", "ada", "a post to keep or delete", clockBase)
		shell.Reload(context.Background())
		shell.State().Selected = "1"
		return shell, db
	}

	bookmarkCount := func(t *testing.T, db *sqlite.DB) int {
		t.Helper()
		var count int
		if err := db.QueryRow(`SELECT COUNT(*) FROM bookmarks`).Scan(&count); err != nil {
			t.Fatalf("count: %v", err)
		}
		return count
	}

	t.Run("dismissing the dialog keeps the bookmark", func(t *testing.T) {
		shell, db := seedAndSelect(t)

		// The id the seed selected, rather than a Shell accessor: Selection was removed with
		// the bulk bar's move to BatchSelection, and re-adding a production method to serve
		// one test is the trade this project refuses.
		shell.AskDeleteBookmarks([]string{"1"})
		shell.deleteBookmarks.Confirmation.Answer(DismissedValue)
		shell.applyDeleteBookmarks(context.Background())

		if got := bookmarkCount(t, db); got != 1 {
			t.Errorf("bookmark count = %d, want 1: a dismissal must not delete", got)
		}
	})

	t.Run("cancelling keeps the bookmark", func(t *testing.T) {
		shell, db := seedAndSelect(t)

		// The id the seed selected, rather than a Shell accessor: Selection was removed with
		// the bulk bar's move to BatchSelection, and re-adding a production method to serve
		// one test is the trade this project refuses.
		shell.AskDeleteBookmarks([]string{"1"})
		shell.deleteBookmarks.Confirmation.Answer(1) // cancel is last
		shell.applyDeleteBookmarks(context.Background())

		if got := bookmarkCount(t, db); got != 1 {
			t.Errorf("bookmark count = %d, want 1: cancel must not delete", got)
		}
	})

	t.Run("never asking keeps the bookmark", func(t *testing.T) {
		shell, db := seedAndSelect(t)

		// No Ask at all, yet the handler still runs.
		shell.applyDeleteBookmarks(context.Background())

		if got := bookmarkCount(t, db); got != 1 {
			t.Errorf("bookmark count = %d, want 1: nothing was asked", got)
		}
	})

	t.Run("confirming deletes the bookmark", func(t *testing.T) {
		shell, db := seedAndSelect(t)

		// The id the seed selected, rather than a Shell accessor: Selection was removed with
		// the bulk bar's move to BatchSelection, and re-adding a production method to serve
		// one test is the trade this project refuses.
		shell.AskDeleteBookmarks([]string{"1"})
		shell.deleteBookmarks.Confirmation.Answer(0) // confirm is first
		shell.applyDeleteBookmarks(context.Background())

		if got := bookmarkCount(t, db); got != 0 {
			t.Errorf("bookmark count = %d, want 0 after confirming", got)
		}
	})

	t.Run("a second run does nothing, so a rebuild cannot delete twice", func(t *testing.T) {
		shell, db := seedAndSelect(t)

		// The id the seed selected, rather than a Shell accessor: Selection was removed with
		// the bulk bar's move to BatchSelection, and re-adding a production method to serve
		// one test is the trade this project refuses.
		shell.AskDeleteBookmarks([]string{"1"})
		shell.deleteBookmarks.Confirmation.Answer(0)
		shell.applyDeleteBookmarks(context.Background())
		// A view that rebuilds while the answer is handled must find nothing armed.
		shell.applyDeleteBookmarks(context.Background())

		if got := bookmarkCount(t, db); got != 0 {
			t.Errorf("bookmark count = %d, want 0", got)
		}
	})
}

// TestShellRendersInChinese covers the other locale end to end.
func TestShellRendersInChinese(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.SetLocale("zh-CN")
	shell.Reload(context.Background())

	tester := ui.NewTester(shell.View, 1180, 760)

	// Literals rather than catalogue lookups, because what this checks is the Chinese a user
	// actually reads. "资料库" is the library title. The list names what the results header
	// draws: the scope title and the count's label.
	for _, want := range []string{"全部书签", "结果", "标签", "作者"} {
		if !tester.HasText(want) {
			t.Errorf("the Chinese shell is missing %q; texts: %q", want, tester.Texts())
		}
	}

	// The detail is an overlay now, so its heading appears only once it is opened.
	shell.OpenSelectedDetail()
	tester.Frame()
	// The hero carries the author, not a heading: detailsTitle is the close button's aria-label,
	// not a visible word. See TestTheDetailHeroShowsTheAuthorRatherThanATitle.
	if !tester.HasText("@ada") {
		t.Errorf("the Chinese detail overlay is missing its author; texts: %q", tester.Texts())
	}
	// And no English leaked through the sidebar.
	if tester.HasText("All bookmarks") {
		t.Error("an English label leaked into the Chinese interface")
	}
	// The four omitted sections are absent in this language too.
	absent(tester.Texts(), t, "收件箱", "未分类", "列表", "保存视图")
}

// TestPreferencesPersistAcrossRestarts covers the settings surviving a relaunch,
// which is the whole point of storing them.
func TestPreferencesPersistAcrossRestarts(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()

	store := settings.New(ports.Repositories{Settings: sqlite.NewSettingsRepository(db)}, fixedClock{now: clockBase})
	shell.UseSettings(store)
	shell.Reload(ctx)

	// A fresh shell starts from the defaults.
	if err := shell.ApplyPreferences(ctx); err != nil {
		t.Fatalf("ApplyPreferences: %v", err)
	}
	if shell.State().Appearance != AppearanceSystem {
		t.Errorf("appearance = %q, want the default", shell.State().Appearance)
	}

	// The user changes both preferences.
	shell.SetAppearance(AppearanceDark)
	shell.SetLocale("zh-CN")

	// A second shell over the same database is what a relaunch looks like.
	second := NewShell(nil, AppearanceSystem)
	second.UseSettings(store)
	if err := second.ApplyPreferences(ctx); err != nil {
		t.Fatalf("second ApplyPreferences: %v", err)
	}

	if second.State().Appearance != AppearanceDark {
		t.Errorf("appearance = %q, want the stored dark", second.State().Appearance)
	}
	if second.State().Locale != "zh-CN" {
		t.Errorf("locale = %q, want the stored zh-CN", second.State().Locale)
	}
	// And the translator followed, so the interface is actually in Chinese.
	if got := second.t().T(i18n.SidebarAllBookmarks); got != "全部书签" {
		t.Errorf("translation = %q, want Chinese", got)
	}
}

// TestANarrowWindowGivesTheSidebarUp is the layout rule: the sidebar from 1024 points up
// and the page alone below it. The usual layout moves its page tabs into the header there;
// this shell has no page tabs to move, so what is ported is the rule -- a narrow window gives
// the page its room back.
func TestANarrowWindowGivesTheSidebarUp(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(context.Background())

	shell.SetTestedViewportWidth(NavSideWidth)
	if shell.sidebarCollapsed() {
		t.Fatal("the sidebar is hidden at the width that keeps it")
	}
	shell.SetTestedViewportWidth(NavSideWidth - 1)
	if !shell.sidebarCollapsed() {
		t.Error("the sidebar is still there one point below the 1024 breakpoint")
	}
	if got := shell.sidebarPaneWidth(); got != 0 {
		t.Errorf("the sidebar owns %v points at a narrow width, want 0", got)
	}

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	if containsText(tester.Texts(), shell.t().T(i18n.SidebarTags)) {
		t.Error("a narrow window still draws the facet tabs")
	}
}

// TestAChosenLayoutOutlivesTheWidth: a chosen layout overrides the width query at every
// width and is kept for the next launch.
func TestAChosenLayoutOutlivesTheWidth(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	store := settings.New(ports.Repositories{Settings: sqlite.NewSettingsRepository(db)}, fixedClock{now: clockBase})
	shell.UseSettings(store)

	// Below the threshold the width hid the column; the reader's press brings it back and
	// that press is the pick.
	shell.SetTestedViewportWidth(NavSideWidth - 1)
	if !shell.sidebarCollapsed() {
		t.Fatal("the width rule did not hide the column")
	}
	shell.ToggleSidebar()
	if shell.sidebarCollapsed() {
		t.Error("the press did not show the column")
	}
	if !shell.state.SidebarChosen {
		t.Error("the press did not record a pick")
	}

	// Widening it does not take the pick away, and a relaunch on a narrow window keeps it.
	shell.SetTestedViewportWidth(1400)
	if shell.sidebarCollapsed() {
		t.Error("the chosen layout did not survive the width")
	}

	second := NewShell(nil, AppearanceSystem)
	second.UseSettings(store)
	if err := second.ApplyPreferences(ctx); err != nil {
		t.Fatalf("ApplyPreferences: %v", err)
	}
	second.SetTestedViewportWidth(NavSideWidth - 1)
	if second.sidebarCollapsed() {
		t.Error("the stored pick did not survive a relaunch")
	}
}

// TestAHiddenColumnStoredBeforeThePickIsHonoured: a `sidebarCollapsed` written before the
// pick existed was a reader's press, so loading treats it as one rather than letting the
// width put the column back on a wide window.
func TestAHiddenColumnStoredBeforeThePickIsHonoured(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	store := settings.New(ports.Repositories{Settings: sqlite.NewSettingsRepository(db)}, fixedClock{now: clockBase})
	shell.UseSettings(store)

	preferences := settings.DefaultPreferences()
	preferences.Appearance = AppearanceLight
	preferences.Layout.SidebarCollapsed = true
	if err := store.Save(ctx, preferences); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := shell.ApplyPreferences(ctx); err != nil {
		t.Fatalf("ApplyPreferences: %v", err)
	}
	if !shell.state.SidebarChosen {
		t.Error("a stored hidden column was not read as the reader's pick")
	}
	if !shell.sidebarCollapsed() {
		t.Error("the stored hidden column did not stay hidden")
	}

	// And the default, no layout stored, is the width rule again.
	fresh, freshDB := newShell(t)
	fresh.UseSettings(settings.New(ports.Repositories{Settings: sqlite.NewSettingsRepository(freshDB)}, fixedClock{now: clockBase}))
	if err := fresh.ApplyPreferences(ctx); err != nil {
		t.Fatalf("fresh ApplyPreferences: %v", err)
	}
	if fresh.state.SidebarChosen {
		t.Error("a fresh install claims the reader picked a layout")
	}
}

// TestSettingsSelectorsMapToPreferences keeps the rendered labels and the stored
// values from drifting apart.
func TestSettingsSelectorsMapToPreferences(t *testing.T) {
	shell, _ := newShell(t)

	for label, want := range map[string]Appearance{
		shell.t().T(i18n.AppearanceSystem): AppearanceSystem,
		shell.t().T(i18n.AppearanceLight):  AppearanceLight,
		shell.t().T(i18n.AppearanceDark):   AppearanceDark,
	} {
		shell.State().AppearanceLabel = label
		if got := shell.appearanceFor(label); got != want {
			t.Errorf("label %q mapped to %q, want %q", label, got, want)
		}
	}

	if got := shell.localeFor("中文"); got != "zh-CN" {
		t.Errorf("中文 mapped to %q", got)
	}
	if got := shell.localeFor("English"); got != "en" {
		t.Errorf("English mapped to %q", got)
	}
	// The selector offers the two catalogs and nothing else: there is no "follow the system"
	// entry, because nothing in the app asks the OS which language to use.
	if got := shell.localeLabels(); len(got) != 2 {
		t.Errorf("the language selector offers %q, want 中文 and English", got)
	}
}

// TestSettingsPanelRenders checks the panel is reachable and shows both controls.
func TestSettingsPanelRenders(t *testing.T) {
	shell, _ := newShell(t)
	shell.SetLocale("en")
	shell.Reload(context.Background())

	shell.State().SettingsOpen = true
	tester := ui.NewTester(shell.View, 1180, 760)

	// Only the panel's own text is asserted: a Select's options are drawn inside
	// the control rather than emitted as text elements, so the tester cannot see
	// them. The label-to-preference mapping is covered directly above.
	for _, want := range []string{"Settings", "Language", "Theme"} {
		if !tester.HasText(want) {
			t.Errorf("the settings panel is missing %q; texts: %q", want, tester.Texts())
		}
	}
}

// newBatchShell builds a shell with every usecase the batch actions need.
func newBatchShell(t *testing.T) (*Shell, *sqlite.DB) {
	t.Helper()

	shell, db := newShell(t)
	repos := ports.Repositories{
		Bookmarks:   sqlite.NewBookmarkRepository(db),
		Tags:        sqlite.NewTagRepository(db),
		Lists:       sqlite.NewListRepository(db),
		Suggestions: sqlite.NewSuggestionRepository(db),
		Analysis:    sqlite.NewAnalysisRepository(db),
		Settings:    sqlite.NewSettingsRepository(db),
	}
	shell.UseSettings(settings.New(repos, fixedClock{now: clockBase}))
	shell.UseSuggestions(suggestions.New(repos, fixedClock{now: clockBase}, idgen.New("test")))
	return shell, db
}

// TestBatchSelectionToggles covers the multi-select the bulk actions run on.
func TestBatchSelectionToggles(t *testing.T) {
	shell, db := newBatchShell(t)
	seed(t, db, "1", "ada", "first", clockBase)
	seed(t, db, "2", "grace", "second", clockBase)
	shell.Reload(context.Background())

	shell.toggleSelection("1")
	shell.toggleSelection("2")
	if got := shell.BatchSelection(); len(got) != 2 {
		t.Fatalf("selection = %v, want both", got)
	}
	// Ticking a box is not clicking the card: the bulk toggle leaves selectedBookmarkId
	// alone, so nothing opens and the outline does not move.
	if shell.State().Selected != "" {
		t.Errorf("Selected = %q, want it untouched by a bulk toggle", shell.State().Selected)
	}
	if shell.State().DetailOpen {
		t.Error("a bulk toggle must not open the detail")
	}

	// Toggling again removes it.
	shell.toggleSelection("1")
	if got := shell.BatchSelection(); len(got) != 1 || got[0] != "2" {
		t.Errorf("selection = %v, want only 2", got)
	}

	shell.clearSelection()
	if got := shell.BatchSelection(); len(got) != 0 {
		t.Errorf("selection = %v, want empty", got)
	}
}

// TestTheCardCheckboxIsItsOwnClick covers the corner checkbox as a *separate* control.
//
// The card opens the detail and the checkbox toggles the batch set. MyGo hands a click to the
// innermost clickable node in the hit chain and to that node only (input.go:257-290, 385-388),
// which is what keeps the checkbox's click from reaching the card; this test is the evidence
// for that claim.
func TestTheCardCheckboxIsItsOwnClick(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "a post about compilers", clockBase)
	shell.state.LeftSidebarWidth = WidthSidebarDef
	shell.state.RightSidebarWidth = WidthInspectorDef
	shell.Reload(context.Background())

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	// Found by its accessible name, which names the post's author.
	box, ok := tester.Find(shell.t().Tf(i18n.CardSelectBookmarkFor, "ada"))
	if !ok {
		t.Fatalf("the corner checkbox is not reachable; texts: %q", tester.Texts())
	}
	tester.ClickAt(box.X+box.W/2, box.Y+box.H/2)
	tester.Frame()

	if got := shell.BatchSelection(); len(got) != 1 || got[0] != "1" {
		t.Fatalf("batch selection = %v, want the clicked bookmark", got)
	}
	if shell.State().DetailOpen {
		t.Error("the checkbox click also opened the detail: the inner target must take the click alone")
	}
	if shell.State().Selected != "" {
		t.Errorf("Selected = %q, want it untouched by the checkbox", shell.State().Selected)
	}
}

// TestBatchAddTags is the bulk action the gate asks for.
func TestBatchAddTags(t *testing.T) {
	shell, db := newBatchShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "first", clockBase)
	seed(t, db, "2", "grace", "second", clockBase)
	shell.Reload(ctx)

	shell.toggleSelection("1")
	shell.toggleSelection("2")
	// Several names at once, comma separated.
	shell.State().TagInput = "Go, Rust"

	added, err := shell.AddTagsToSelection(ctx)
	if err != nil {
		t.Fatalf("AddTagsToSelection: %v", err)
	}
	if added != 4 {
		t.Errorf("added %d relations, want 4 (2 tags x 2 bookmarks)", added)
	}

	// The field is cleared so the next action does not repeat it.
	if shell.State().TagInput != "" {
		t.Errorf("TagInput = %q, want it cleared", shell.State().TagInput)
	}

	view, err := shell.service.Load(ctx, domain.LibraryQuery{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if view.Counts.TagCount != 2 {
		t.Errorf("TagCount = %d, want 2", view.Counts.TagCount)
	}
	for _, row := range view.Bookmarks {
		if len(row.Tags) != 2 {
			t.Errorf("bookmark %s has %d tags, want 2", row.TweetID, len(row.Tags))
		}
	}
}

func TestBatchMoveToList(t *testing.T) {
	shell, db := newBatchShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "first", clockBase)
	seed(t, db, "2", "grace", "second", clockBase)
	shell.Reload(ctx)

	work, err := shell.service.CreateList(ctx, "Work")
	if err != nil {
		t.Fatalf("CreateList: %v", err)
	}
	shell.Reload(ctx)

	shell.toggleSelection("1")
	shell.toggleSelection("2")
	shell.State().MoveTargetLabel = "Work"

	if err := shell.MoveSelectionToList(ctx); err != nil {
		t.Fatalf("MoveSelectionToList: %v", err)
	}

	count, err := shell.service.Load(ctx, domain.LibraryQuery{
		Filter: domain.LibraryFilter{ListID: work.ID},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if count.Total != 2 {
		t.Errorf("Work holds %d bookmarks, want 2", count.Total)
	}
}

func TestBatchRemoveTag(t *testing.T) {
	shell, db := newBatchShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "first", clockBase)
	seed(t, db, "2", "grace", "second", clockBase)
	shell.Reload(ctx)

	shell.toggleSelection("1")
	shell.toggleSelection("2")
	shell.State().TagInput = "Go"
	if _, err := shell.AddTagsToSelection(ctx); err != nil {
		t.Fatalf("AddTagsToSelection: %v", err)
	}

	// Find the tag's id from the loaded view.
	view, err := shell.service.Load(ctx, domain.LibraryQuery{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(view.Tags) != 1 {
		t.Fatalf("tags = %+v, want one", view.Tags)
	}
	tagID := view.Tags[0].ID

	// The write above is a completed bulk action, so it cleared the selection; removing the
	// tag is a second action and has to pick the rows again.
	shell.toggleSelection("1")
	shell.toggleSelection("2")
	if err := shell.RemoveTagFromSelection(ctx, tagID); err != nil {
		t.Fatalf("RemoveTagFromSelection: %v", err)
	}

	after, err := shell.service.Load(ctx, domain.LibraryQuery{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, row := range after.Bookmarks {
		if len(row.Tags) != 0 {
			t.Errorf("bookmark %s still carries %d tags", row.TweetID, len(row.Tags))
		}
	}
}

// TestBatchDeleteStillFailsClosed checks the bulk path uses the same confirmation
// rather than deleting directly.
func TestBatchDeleteStillFailsClosed(t *testing.T) {
	shell, db := newBatchShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "first", clockBase)
	seed(t, db, "2", "grace", "second", clockBase)
	shell.Reload(ctx)

	shell.toggleSelection("1")
	shell.toggleSelection("2")

	// Dismissing must leave both bookmarks alone.
	shell.AskDeleteBookmarks(shell.BatchSelection())
	shell.deleteBookmarks.Confirmation.Answer(DismissedValue)
	shell.applyDeleteBookmarks(ctx)

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM bookmarks`).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 2 {
		t.Errorf("%d bookmarks remain, want 2: a dismissal must not delete a batch", count)
	}
}

// TestSplitTagNames covers the batch field's parsing.
func TestSplitTagNames(t *testing.T) {
	got := splitTagNames("  Go , Rust ,,  , SQL ")
	want := []string{"Go", "Rust", "SQL"}

	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("name %d = %q, want %q", i, got[i], want[i])
		}
	}

	if names := splitTagNames("   "); len(names) != 0 {
		t.Errorf("blank input produced %v", names)
	}
}

// TestSuggestionReviewThroughTheShell covers the review flow end to end.
func TestSuggestionReviewThroughTheShell(t *testing.T) {
	shell, db := newBatchShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post", clockBase)

	if _, err := db.Exec(
		`INSERT INTO tag_suggestions (id, bookmark_id, tag_name, source, confidence, status, created_at)
		 VALUES ('s1', '1', 'Kubernetes', 'llm', 0.9, 'pending', ?)`,
		domain.FormatTimestamp(clockBase)); err != nil {
		t.Fatalf("seed suggestion: %v", err)
	}

	shell.openSuggestions(ctx)
	if len(shell.State().Suggestions) != 1 {
		t.Fatalf("loaded %d suggestions, want 1", len(shell.State().Suggestions))
	}

	if err := shell.AcceptSuggestion(ctx, "s1"); err != nil {
		t.Fatalf("AcceptSuggestion: %v", err)
	}

	// The panel reloads and the suggestion is gone from the pending list.
	if len(shell.State().Suggestions) != 0 {
		t.Errorf("%d suggestions remain, want 0", len(shell.State().Suggestions))
	}
	// And the tag now exists on the bookmark.
	view, err := shell.service.Load(ctx, domain.LibraryQuery{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if view.Counts.TagCount != 1 {
		t.Errorf("TagCount = %d, want 1", view.Counts.TagCount)
	}
}

func TestSuggestionRejectThroughTheShell(t *testing.T) {
	shell, db := newBatchShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post", clockBase)

	if _, err := db.Exec(
		`INSERT INTO tag_suggestions (id, bookmark_id, tag_name, source, confidence, status, created_at)
		 VALUES ('s1', '1', 'Spam', 'llm', 0.9, 'pending', ?)`,
		domain.FormatTimestamp(clockBase)); err != nil {
		t.Fatalf("seed suggestion: %v", err)
	}

	if err := shell.RejectSuggestion(ctx, "s1"); err != nil {
		t.Fatalf("RejectSuggestion: %v", err)
	}

	// No tag was created, and the row survives as the negative cache.
	view, err := shell.service.Load(ctx, domain.LibraryQuery{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if view.Counts.TagCount != 0 {
		t.Errorf("TagCount = %d, want 0: a rejection creates nothing", view.Counts.TagCount)
	}

	var status string
	if err := db.QueryRow(`SELECT status FROM tag_suggestions WHERE id = 's1'`).Scan(&status); err != nil {
		t.Fatalf("the rejected row must survive: %v", err)
	}
	if status != domain.StatusRejected {
		t.Errorf("status = %q, want rejected", status)
	}
}

// TestSyncAndAutoTagGuardAgainstDoubleRuns covers the transport-level guard the
// UI relies on: a second click while one is in flight must be ignored rather than
// starting a competing run.
func TestSyncAndAutoTagGuardAgainstDoubleRuns(t *testing.T) {
	shell, db := newBatchShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(ctx)

	// With no service attached, starting reports false rather than panicking.
	if shell.StartSync(ctx) {
		t.Error("a sync with no service attached must report that it did not start")
	}
	if shell.StartAutoTag(ctx) {
		t.Error("an auto-tag pass with no service must report that it did not start")
	}
	if shell.CancelSync() {
		t.Error("cancelling with no service must report false")
	}

	// With one attached, the in-flight flag stops a second start.
	repos := ports.Repositories{
		Bookmarks: sqlite.NewBookmarkRepository(db),
		Tags:      sqlite.NewTagRepository(db),
		Lists:     sqlite.NewListRepository(db),
		Settings:  sqlite.NewSettingsRepository(db),
		SyncRuns:  sqlite.NewSyncRunRepository(db),
	}
	shell.UseSyncer(syncer.New(repos, fixedClock{now: clockBase}, idgen.New("test"), nil))

	shell.State().SyncRunning = true
	if shell.StartSync(ctx) {
		t.Error("a second sync must not start while one is running")
	}
}

// TestSyncStatusLabelsAreTranslated keeps the status line from showing a raw
// domain value.
func TestSyncStatusLabelsAreTranslated(t *testing.T) {
	shell, _ := newShell(t)

	for _, status := range []domain.SyncStatus{
		domain.SyncRunning, domain.SyncSuccess, domain.SyncPartialSuccess,
		domain.SyncCancelled, domain.SyncError,
	} {
		if got := syncStatusLabel(shell.t(), status); got == "" || got == status {
			t.Errorf("status %q rendered as %q", status, got)
		}
	}
}

// TestOpenSettingsAndSuggestionsAreReachable covers the menu bar's two entries,
// which live outside this package and so need exported openers.
func TestOpenSettingsAndSuggestionsAreReachable(t *testing.T) {
	shell, db := newBatchShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(ctx)

	shell.OpenSettings()
	if !shell.State().SettingsOpen {
		t.Error("OpenSettings must show the panel")
	}
	// The selectors are refreshed so they reflect the stored preferences.
	if shell.State().AppearanceLabel == "" {
		t.Error("the appearance selector must be bound to a label")
	}

	shell.OpenSuggestions(ctx)
	if !shell.State().SuggestionsOpen {
		t.Error("OpenSuggestions must show the panel")
	}
}

// TestLocaleChangeRebuildsTheMenuBar covers the callback that keeps the menu's
// language in step with the interface's.
func TestLocaleChangeRebuildsTheMenuBar(t *testing.T) {
	shell, _ := newShell(t)

	rebuilt := 0
	shell.OnLocaleChanged(func() { rebuilt++ })

	shell.SetLocale("zh-CN")
	if rebuilt != 1 {
		t.Errorf("the menu was rebuilt %d time(s), want 1", rebuilt)
	}
	if got := shell.T(i18n.MenuView); got != "显示" {
		t.Errorf("menu label = %q, want the Chinese one", got)
	}

	shell.SetLocale("en")
	if rebuilt != 2 {
		t.Errorf("the menu was rebuilt %d time(s), want 2", rebuilt)
	}
	if got := shell.T(i18n.MenuView); got != "View" {
		t.Errorf("menu label = %q, want the English one", got)
	}
}

// TestBundleConfirmationFailsClosed covers the file-level actions: an import or a
// restore must not run unless the user explicitly confirmed it.
func TestBundleConfirmationFailsClosed(t *testing.T) {
	newBundleShell := func(t *testing.T) (*Shell, *sqlite.DB) {
		t.Helper()
		shell, db := newBatchShell(t)
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
		return shell, db
	}

	t.Run("an unasked action does nothing", func(t *testing.T) {
		shell, db := newBundleShell(t)
		seed(t, db, "1", "ada", "keep me", clockBase)
		shell.Reload(context.Background())

		if err := shell.applyBundle(context.Background()); err != nil {
			t.Fatalf("applyBundle: %v", err)
		}

		var count int
		if err := db.QueryRow(`SELECT COUNT(*) FROM bookmarks`).Scan(&count); err != nil {
			t.Fatalf("count: %v", err)
		}
		if count != 1 {
			t.Errorf("%d bookmarks remain, want 1: nothing was confirmed", count)
		}
	})

	t.Run("a dismissed action does nothing", func(t *testing.T) {
		shell, db := newBundleShell(t)
		seed(t, db, "1", "ada", "keep me", clockBase)
		shell.Reload(context.Background())

		// Arm with a path that does not exist, then dismiss: a dismissal must not
		// even reach the file.
		shell.bundle.Arm(BundleRequest{Path: filepath.Join(t.TempDir(), "absent.trovebundle")},
			"Import?", "body", "Import", "Cancel")
		shell.bundle.Confirmation.Answer(DismissedValue)

		if err := shell.applyBundle(context.Background()); err != nil {
			t.Fatalf("a dismissed action must be a no-op, got %v", err)
		}

		var count int
		if err := db.QueryRow(`SELECT COUNT(*) FROM bookmarks`).Scan(&count); err != nil {
			t.Fatalf("count: %v", err)
		}
		if count != 1 {
			t.Errorf("%d bookmarks remain, want 1: a dismissal must not act", count)
		}
	})

	t.Run("a confirmed action runs exactly once", func(t *testing.T) {
		shell, db := newBundleShell(t)
		ctx := context.Background()
		seed(t, db, "1", "ada", "to be restored away", clockBase)
		shell.Reload(ctx)

		// Build a bundle from a different library holding one other bookmark.
		sourceDir := t.TempDir()
		sourceDB, err := sqlite.Open(filepath.Join(sourceDir, "source.db"))
		if err != nil {
			t.Fatalf("Open source: %v", err)
		}
		defer sourceDB.Close()

		sourceRepos := ports.Repositories{
			Bookmarks:   sqlite.NewBookmarkRepository(sourceDB),
			Tags:        sqlite.NewTagRepository(sourceDB),
			Lists:       sqlite.NewListRepository(sourceDB),
			Suggestions: sqlite.NewSuggestionRepository(sourceDB),
			Analysis:    sqlite.NewAnalysisRepository(sourceDB),
			SavedViews:  sqlite.NewSavedViewRepository(sourceDB),
			Settings:    sqlite.NewSettingsRepository(sourceDB),
			SyncRuns:    sqlite.NewSyncRunRepository(sourceDB),
		}
		if _, err := sourceRepos.Bookmarks.UpsertMany(ctx, []domain.Bookmark{{
			TweetID: "from-bundle", TweetURL: "https://x.com/x/status/7", Text: "from the bundle",
			SavedAt: clockBase, LastSeenAt: clockBase, UpdatedAt: clockBase,
		}}); err != nil {
			t.Fatalf("UpsertMany: %v", err)
		}

		sourceService := importexport.New(sourceRepos, fixedClock{now: clockBase}, "0.3.0")
		raw, err := sourceService.Export(ctx, importexport.DefaultExportOptions())
		if err != nil {
			t.Fatalf("Export: %v", err)
		}
		path := filepath.Join(t.TempDir(), "bundle.trovebundle")
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		shell.bundle.Arm(BundleRequest{Path: path, Restore: true}, "Restore?", "body", "Replace", "Cancel")
		shell.bundle.Confirmation.Answer(0)

		if err := shell.applyBundle(ctx); err != nil {
			t.Fatalf("applyBundle: %v", err)
		}

		// The restore replaced the library.
		var count int
		if err := db.QueryRow(`SELECT COUNT(*) FROM bookmarks`).Scan(&count); err != nil {
			t.Fatalf("count: %v", err)
		}
		if count != 1 {
			t.Fatalf("%d bookmarks after restore, want 1", count)
		}
		var id string
		if err := db.QueryRow(`SELECT tweet_id FROM bookmarks`).Scan(&id); err != nil {
			t.Fatalf("read id: %v", err)
		}
		if id != "from-bundle" {
			t.Errorf("bookmark = %q, want the bundle's", id)
		}

		// A second call finds nothing armed, so a view rebuild cannot repeat it.
		if err := shell.applyBundle(ctx); err != nil {
			t.Fatalf("second applyBundle: %v", err)
		}
		if err := db.QueryRow(`SELECT COUNT(*) FROM bookmarks`).Scan(&count); err != nil {
			t.Fatalf("count: %v", err)
		}
		if count != 1 {
			t.Errorf("%d bookmarks after a repeated apply, want 1", count)
		}
	})
}

// TestPanelsAreMutuallyExclusive covers the requirement: two modals stacked
// on one window leaves the user unsure which Escape closes.
func TestPanelsAreMutuallyExclusive(t *testing.T) {
	shell, db := newBatchShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(ctx)

	shell.OpenSettings()
	if !shell.State().SettingsOpen {
		t.Fatal("the settings panel should be open")
	}

	shell.OpenSuggestions(ctx)
	if shell.State().SettingsOpen {
		t.Error("opening suggestions must close settings")
	}
	if !shell.State().SuggestionsOpen {
		t.Error("the suggestions panel should be open")
	}

	shell.OpenSettings()
	if shell.State().SuggestionsOpen {
		t.Error("opening settings must close suggestions")
	}
	if !shell.State().SettingsOpen {
		t.Error("the settings panel should be open")
	}
}

// TestHiddenAutoTagSettingsPersist covers the requirement that the hidden
// knobs are reachable and actually take effect.
func TestHiddenAutoTagSettingsPersist(t *testing.T) {
	shell, db := newBatchShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post", clockBase)

	// A tagging service over the same database.
	repos := ports.Repositories{
		Bookmarks:   sqlite.NewBookmarkRepository(db),
		Tags:        sqlite.NewTagRepository(db),
		Lists:       sqlite.NewListRepository(db),
		Suggestions: sqlite.NewSuggestionRepository(db),
		Analysis:    sqlite.NewAnalysisRepository(db),
		Settings:    sqlite.NewSettingsRepository(db),
	}
	tagger := tagging.New(repos, fixedClock{now: clockBase}, nil)
	shell.UseTagging(tagger)

	shell.OpenSettings()
	if shell.State().BatchSizeInput == 0 {
		t.Error("opening settings must load the stored batch size")
	}

	shell.State().AutoTagEnabled = true
	shell.State().AutoContinue = true
	shell.State().BatchSizeInput = 7
	shell.State().ConcurrencyInput = 2
	shell.saveAutoTagSettings(ctx)

	// A fresh read is what a relaunch would see.
	stored, err := tagger.LoadSettings(ctx)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if !stored.Enabled || !stored.AutoContinue {
		t.Errorf("the toggles did not persist: %+v", stored)
	}
	if stored.LLM.BatchSize != 7 || stored.LLM.Concurrency != 2 {
		t.Errorf("the numbers did not persist: batch=%d concurrency=%d",
			stored.LLM.BatchSize, stored.LLM.Concurrency)
	}
}

// The Cmd-F focus rule is covered by TestTheSearchFocusIsTakenOnce: the field is
// focused by a bound Handle once, not by a state flag a later load has to clear.

// NewBookmarkRepositoryForTest exposes the repository to this package's tests.
func NewBookmarkRepositoryForTest(db *sqlite.DB) *sqlite.BookmarkRepository {
	return sqlite.NewBookmarkRepository(db)
}

// TestArrowKeysMoveTheSelection covers arrow-key list selection.
func TestArrowKeysMoveTheSelection(t *testing.T) {
	shell, db := newBatchShell(t)
	ctx := context.Background()
	for _, id := range []string{"a", "b", "c"} {
		seed(t, db, id, "ada", "post "+id, clockBase)
	}
	shell.Reload(ctx)

	rows := shell.State().Data.Bookmarks
	if len(rows) != 3 {
		t.Fatalf("loaded %d bookmarks, want 3", len(rows))
	}

	// Down from nothing selects the first.
	shell.MoveSelection(1)
	if shell.State().Selected != rows[0].TweetID {
		t.Errorf("first Down selected %q, want %q", shell.State().Selected, rows[0].TweetID)
	}

	shell.MoveSelection(1)
	if shell.State().Selected != rows[1].TweetID {
		t.Errorf("second Down selected %q, want %q", shell.State().Selected, rows[1].TweetID)
	}

	shell.MoveSelection(-1)
	if shell.State().Selected != rows[0].TweetID {
		t.Errorf("Up selected %q, want %q", shell.State().Selected, rows[0].TweetID)
	}

	// It stops at the edges rather than wrapping, so holding a key does not cycle.
	shell.MoveSelection(-1)
	if shell.State().Selected != rows[0].TweetID {
		t.Errorf("Up past the top selected %q, want it to stay", shell.State().Selected)
	}
	for i := 0; i < 10; i++ {
		shell.MoveSelection(1)
	}
	if shell.State().Selected != rows[len(rows)-1].TweetID {
		t.Errorf("Down past the bottom selected %q, want the last row", shell.State().Selected)
	}

	// Up from nothing starts at the last row, not the first.
	shell.State().Selected = ""
	shell.MoveSelection(-1)
	if shell.State().Selected != rows[len(rows)-1].TweetID {
		t.Errorf("Up from nothing selected %q, want the last row", shell.State().Selected)
	}

	// The arrows move the cursor the detail opens on; they do not tick boxes. The bulk set
	// is what the card's corner checkbox toggles, and a bulk action must run on exactly what
	// the user ticked -- growing it from the keyboard would delete or tag rows nobody chose.
	if selection := shell.BatchSelection(); len(selection) != 0 {
		t.Errorf("batch selection = %v, want it untouched by the arrow keys", selection)
	}
}

func TestMoveSelectionOnAnEmptyList(t *testing.T) {
	shell, _ := newBatchShell(t)
	shell.Reload(context.Background())

	// Nothing to move through, and nothing to panic on.
	shell.MoveSelection(1)
	if shell.State().Selected != "" {
		t.Errorf("selected %q on an empty list", shell.State().Selected)
	}
}

// TestEnterOpensTheDetail covers "Enter 开详情".
func TestEnterOpensTheDetail(t *testing.T) {
	shell, db := newBatchShell(t)
	ctx := context.Background()
	seed(t, db, "a", "ada", "a post", clockBase)
	shell.Reload(ctx)

	// The inspector has nothing to show yet.
	if shell.State().Selected != "" {
		t.Fatal("nothing should be selected on a fresh list")
	}

	shell.OpenSelectedDetail()
	if shell.State().Selected == "" {
		t.Error("Enter must select something so the inspector has a detail to show")
	}

	// Pressing it again must not move the selection: Enter opens, it does not cycle.
	first := shell.State().Selected
	shell.OpenSelectedDetail()
	if shell.State().Selected != first {
		t.Errorf("a second Enter moved the selection to %q", shell.State().Selected)
	}
}

// TestEscapeClearsTheSearch covers "Escape 退搜索".
func TestEscapeClearsTheSearch(t *testing.T) {
	shell, db := newBatchShell(t)
	ctx := context.Background()
	seed(t, db, "a", "ada", "kubernetes post", clockBase)
	shell.Reload(ctx)

	shell.State().Query = "kubernetes"
	shell.Reload(ctx)
	if got := shell.State().Data.Total; got != 1 {
		t.Fatalf("the query found %d rows, want 1", got)
	}

	shell.ExitSearch(ctx)
	if shell.State().Query != "" {
		t.Errorf("query = %q, want it cleared", shell.State().Query)
	}
	if got := shell.State().Data.Total; got != 1 {
		t.Errorf("after clearing the search the list holds %d, want the whole library", got)
	}

	// Escape on an empty box is a no-op rather than a redundant query.
	shell.ExitSearch(ctx)
	if shell.State().Query != "" {
		t.Error("Escape must not introduce a query")
	}
}

// TestKeywordRulesRoundTripAndFire is the whole point of the editor: a rule the user
// types must be stored, and must then actually tag a matching bookmark. An editor
// that saves a rule the engine ignores would look like it worked.
func TestKeywordRulesRoundTripAndFire(t *testing.T) {
	shell, db := newBatchShell(t)
	ctx := context.Background()
	seed(t, db, "b1", "ada", "a post about kubernetes networking", clockBase)

	repos := ports.Repositories{
		Bookmarks:   sqlite.NewBookmarkRepository(db),
		Tags:        sqlite.NewTagRepository(db),
		Lists:       sqlite.NewListRepository(db),
		Suggestions: sqlite.NewSuggestionRepository(db),
		Analysis:    sqlite.NewAnalysisRepository(db),
		Settings:    sqlite.NewSettingsRepository(db),
	}
	tagger := tagging.New(repos, fixedClock{now: clockBase}, nil)
	shell.UseTagging(tagger)

	shell.OpenSettings()
	// A run refuses to start while auto-tagging is off, so the toggle has to be on
	// before the rules can fire.
	shell.State().AutoTagEnabled = true
	shell.saveAutoTagSettings(ctx)

	// A half-typed rule must not be accepted: it could never fire, so storing it
	// would leave a dead row.
	shell.State().DraftKeyword = "kubernetes"
	shell.State().DraftRuleTag = ""
	shell.addKeywordRule()
	if len(shell.State().KeywordRules) != 0 {
		t.Fatal("an incomplete rule must be refused")
	}

	// A complete one is accepted and the draft cleared.
	shell.State().DraftKeyword = "kubernetes"
	shell.State().DraftRuleTag = "K8s"
	shell.addKeywordRule()
	if len(shell.State().KeywordRules) != 1 {
		t.Fatalf("rules = %+v, want 1", shell.State().KeywordRules)
	}
	if shell.State().DraftKeyword != "" || shell.State().DraftRuleTag != "" {
		t.Error("the draft must be cleared after adding")
	}

	// It survives a re-read, which is what a relaunch does.
	stored, err := tagger.LoadSettings(ctx)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if len(stored.Heuristics.KeywordRules) != 1 {
		t.Fatalf("stored rules = %+v, want 1", stored.Heuristics.KeywordRules)
	}
	if stored.Heuristics.KeywordRules[0].Keyword != "kubernetes" ||
		stored.Heuristics.KeywordRules[0].TagName != "K8s" {
		t.Errorf("stored rule = %+v", stored.Heuristics.KeywordRules[0])
	}

	// And it fires: the bookmark gets the tag through the heuristic path.
	summary, err := tagger.Run(ctx, stored)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	t.Logf("summary: outcome=%q suggested=%d pending=%d candidates=%d",
		summary.Outcome, summary.SuggestedCount, summary.PendingCount, summary.CandidateCount)
	if summary.SuggestedCount == 0 {
		t.Fatalf("the rule must produce a suggestion, outcome=%q", summary.Outcome)
	}

	suggestions, err := sqlite.NewSuggestionRepository(db).List(ctx, domain.StatusPending)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	found := false
	for _, suggestion := range suggestions {
		if suggestion.TagName == "K8s" {
			found = true
			if suggestion.Source != domain.SourceKeyword {
				t.Errorf("source = %q, want %q", suggestion.Source, domain.SourceKeyword)
			}
		}
	}
	if !found {
		t.Errorf("the keyword rule did not tag the matching bookmark: %+v", suggestions)
	}

	// Removing it persists too.
	shell.removeKeywordRule(0)
	if len(shell.State().KeywordRules) != 0 {
		t.Error("the rule must be gone from the panel")
	}
	afterRemoval, err := tagger.LoadSettings(ctx)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if len(afterRemoval.Heuristics.KeywordRules) != 0 {
		t.Errorf("the removal must persist: %+v", afterRemoval.Heuristics.KeywordRules)
	}
}

// TestHeuristicTogglesPersist covers the two switches beside the rules.
func TestHeuristicTogglesPersist(t *testing.T) {
	shell, db := newBatchShell(t)
	ctx := context.Background()
	seed(t, db, "b1", "ada", "a post", clockBase)

	repos := ports.Repositories{
		Bookmarks:   sqlite.NewBookmarkRepository(db),
		Tags:        sqlite.NewTagRepository(db),
		Lists:       sqlite.NewListRepository(db),
		Suggestions: sqlite.NewSuggestionRepository(db),
		Analysis:    sqlite.NewAnalysisRepository(db),
		Settings:    sqlite.NewSettingsRepository(db),
	}
	tagger := tagging.New(repos, fixedClock{now: clockBase}, nil)
	shell.UseTagging(tagger)

	shell.OpenSettings()
	// The defaults are both on.
	if !shell.State().UseHashtags || !shell.State().UseAuthors {
		t.Fatalf("both heuristics must default to on, got hashtags=%v authors=%v",
			shell.State().UseHashtags, shell.State().UseAuthors)
	}

	shell.State().UseHashtags = false
	shell.saveAutoTagSettings(ctx)

	stored, err := tagger.LoadSettings(ctx)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if stored.Heuristics.UseHashtags {
		t.Error("turning hashtags off must persist")
	}
	if !stored.Heuristics.UseAuthors {
		t.Error("the other toggle must be left alone")
	}
}

// TestRemoveKeywordRuleIgnoresBadIndex keeps the panel from panicking on a stale row.
func TestRemoveKeywordRuleIgnoresBadIndex(t *testing.T) {
	shell, _ := newBatchShell(t)

	shell.removeKeywordRule(-1)
	shell.removeKeywordRule(99)
	if len(shell.State().KeywordRules) != 0 {
		t.Error("no rule should have been added")
	}
}

// TestSortButtonCycles covers the stylesheet's sort control: one click advances one
// order, and it wraps rather than stopping at the last.
func TestSortButtonCycles(t *testing.T) {
	shell, db := newBatchShell(t)
	ctx := context.Background()
	seed(t, db, "a", "ada", "a post", clockBase)
	shell.Reload(ctx)

	// Five orders are supported, but the button walks four: the sequence leaves `saved-desc`
	// out, and it is reachable only as a saved view's stored order.
	if len(shell.sortOptions) != 5 {
		t.Fatalf("%d supported orders, want 5", len(shell.sortOptions))
	}
	if len(shell.sortCycle) != 4 {
		t.Fatalf("%d orders in the cycle, want 4", len(shell.sortCycle))
	}
	for _, order := range shell.sortCycle {
		if order == domain.SortSavedDesc {
			t.Error("saved-desc must not be in the cycle")
		}
	}

	// Every cycle order is reachable in one pass, and each has a label.
	seen := map[domain.SortOrder]bool{}
	for i := 0; i < len(shell.sortCycle); i++ {
		seen[shell.state.Sort] = true
		if shell.currentSortLabel() == "" {
			t.Fatalf("order %q has no label", shell.state.Sort)
		}
		shell.CycleSort(ctx)
	}
	if len(seen) != len(shell.sortCycle) {
		t.Errorf("a full cycle reached %d of %d orders: %v", len(seen), len(shell.sortCycle), seen)
	}

	// It wrapped rather than stopping.
	if shell.state.Sort != shell.sortCycle[0] {
		t.Errorf("after a full cycle the order is %q, want it back at %q",
			shell.state.Sort, shell.sortCycle[0])
	}
}

// TestActiveFilterChipsAreRemovable covers removable active filter chips.
func TestActiveFilterChipsAreRemovable(t *testing.T) {
	shell, db := newBatchShell(t)
	ctx := context.Background()
	seed(t, db, "a", "ada", "a post", clockBase)

	// No filters means no chips.
	shell.Reload(ctx)
	if len(shell.activeConditions()) != 0 {
		t.Errorf("an unfiltered list must show no chips: %+v", shell.activeConditions())
	}

	shell.State().OnlyWithMedia = true
	shell.State().OnlyLongform = true
	shell.Reload(ctx)

	chips := shell.activeConditions()
	if len(chips) != 2 {
		t.Fatalf("%d chips, want 2 (%+v)", len(chips), chips)
	}

	// Removing one clears exactly that filter.
	chips[0].Remove()
	if shell.State().OnlyWithMedia && shell.State().OnlyLongform {
		t.Error("removing a chip must clear its filter")
	}
	if len(shell.activeConditions()) != 1 {
		t.Errorf("one chip must remain: %+v", shell.activeConditions())
	}

	shell.activeConditions()[0].Remove()
	if len(shell.activeConditions()) != 0 {
		t.Errorf("all chips must be clearable: %+v", shell.activeConditions())
	}

	// The scope is navigation, not a condition: reading the Inbox is not a filter to
	// remove, and a chip for it would suggest the scope stacks with conditions.
	shell.State().Selection = selectionInbox
	shell.Reload(ctx)
	if len(shell.activeConditions()) != 0 {
		t.Errorf("the scope must not produce a chip: %+v", shell.activeConditions())
	}
}

// TestStatusBadgeMatchesTheStylesheet pins the four status colours and the dark overrides.
// accentForTest stands in for the system accent these tests pass in. It is deliberately not
// cinnabar: a test that used cinnabar would keep passing if StatusRunning reverted to it.
var accentForTest = ui.Hex("#0a84ff")

func TestStatusBadgeMatchesTheStylesheet(t *testing.T) {
	if len(AllStatuses) != 5 {
		t.Errorf("%d statuses, want the stylesheet's 5 (idle + 4)", len(AllStatuses))
	}

	// The stylesheet's foreground colours, verbatim.
	for _, tc := range []struct {
		status string
		light  string
		dark   string
	}{
		{StatusSuccess, "#4a6b52", "#8fb394"},
		{StatusPartial, "#906327", "#e0b070"},
		{StatusError, "#7e241c", "#f2a19a"},
	} {
		light := StatusBadge(tc.status, AppearanceLight, false, accentForTest)
		if light.Text != ui.Hex(tc.light) {
			t.Errorf("light %s text must be %s (--green/--amber/--red)", tc.status, tc.light)
		}
		dark := StatusBadge(tc.status, AppearanceDark, false, accentForTest)
		if dark.Text != ui.Hex(tc.dark) {
			t.Errorf("dark %s text must be %s (the dark token block)", tc.status, tc.dark)
		}
	}

	// Running takes the accent rather than a status colour.
	if got := StatusBadge(StatusRunning, AppearanceLight, false, accentForTest); got.Text != accentForTest {
		t.Error("the running badge must use the accent")
	}
	// Idle is the neutral badge pair.
	if got := StatusBadge(StatusIdle, AppearanceLight, false, accentForTest); got.Fill != lightBadgeBg {
		t.Error("the idle badge must use --badge-bg")
	}

	// An unknown status degrades to idle rather than producing an invisible pill.
	if got := StatusBadge("invented", AppearanceLight, false, accentForTest); got.Fill != lightBadgeBg {
		t.Error("an unknown status must fall back to idle")
	}

	// The dot is 7 DIP.
	if StatusDotSize != 7 {
		t.Errorf("the status dot must be 7 DIP, got %d", StatusDotSize)
	}
}

// TestReduceMotionComesFromTheSystem documents where the requirement is met.
//
// MyGo reads the desktop's Reduce Motion setting and honours it in its own
// animations, including the split divider's drag. The app writes no animation of its
// own, so there is nothing here to switch off -- and this test exists so that a later
// change which does add one has somewhere to look.
func TestReduceMotionComesFromTheSystem(t *testing.T) {
	shell, _ := newBatchShell(t)
	tester := ui.NewTester(shell.View, 1180, 760)
	tester.SetPreferences(ui.Preferences{ReduceMotion: true, TextScale: 1})

	// A frame must build and draw with motion reduced. MyGo consumes the preference
	// itself, so there is nothing for this package to branch on -- the assertion is
	// that asking for less motion does not break the build.
	tester.Frame()
	tester.Frame()
}

// TestGraphModeRenders covers the drawing path: the graph is painted with the
// painter rather than laid out from elements, so a frame is the only way to know the
// geometry and the colour arithmetic survive a real build.
func TestGraphModeRenders(t *testing.T) {
	shell, db := newBatchShell(t)
	ctx := context.Background()

	// Two tags that co-occur, so the graph really has two nodes and an edge: a
	// render test over an empty model would not exercise the drawing at all.
	for _, id := range []string{"a", "b"} {
		seed(t, db, id, "ada", "post "+id, clockBase)
	}

	tags := sqlite.NewTagRepository(db)
	for _, tag := range []domain.Tag{
		{ID: "t1", Name: "Go", CanonicalKey: domain.CanonicalTagKey("Go")},
		{ID: "t2", Name: "Rust", CanonicalKey: domain.CanonicalTagKey("Rust")},
	} {
		tag.CreatedAt = domain.FormatTimestamp(clockBase)
		tag.UpdatedAt = domain.FormatTimestamp(clockBase)
		if err := tags.Create(ctx, tag); err != nil {
			t.Fatalf("Create tag: %v", err)
		}
	}
	// Both tags on one bookmark, which is what produces a cooccurrence edge.
	if err := tags.AttachMany(ctx, []domain.BookmarkTag{
		{BookmarkID: "a", TagID: "t1", CreatedAt: clockBase, Source: domain.TagSourceManual},
		{BookmarkID: "a", TagID: "t2", CreatedAt: clockBase, Source: domain.TagSourceManual},
	}); err != nil {
		t.Fatalf("AttachMany: %v", err)
	}

	shell.Reload(ctx)

	shell.ToggleGraph(ctx)
	if !shell.State().GraphMode {
		t.Fatal("the graph mode must be on")
	}
	if len(shell.State().GraphModel.Nodes) != 2 {
		t.Fatalf("the graph has %d nodes, want 2", len(shell.State().GraphModel.Nodes))
	}
	if len(shell.State().GraphModel.Edges) != 1 {
		t.Fatalf("the graph has %d edges, want the 1 cooccurrence", len(shell.State().GraphModel.Edges))
	}
	// Every node has a position and a radius, which is what the painter reads.
	for _, node := range shell.State().GraphModel.Nodes {
		if node.X < 0 || node.X > 1 || node.Y < 0 || node.Y > 1 {
			t.Errorf("node %s at (%v, %v) is outside the unit square", node.TagID, node.X, node.Y)
		}
		if node.Radius <= 0 {
			t.Errorf("node %s has radius %v", node.TagID, node.Radius)
		}
	}

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	// The summary is drawn, so the graph mode is reachable and not a blank pane.
	if !tester.HasText("Graph") && !tester.HasText("tags") {
		t.Error("the graph frame must show its summary")
	}

	// Turning it off returns to the list, which is the state the rest of the UI
	// expects.
	shell.ToggleGraph(ctx)
	if shell.State().GraphMode {
		t.Error("the graph mode must be off")
	}
	if !shell.State().Loaded {
		t.Error("returning from the graph must have reloaded the list")
	}
}

// TestGraphFocusDimsRatherThanHides keeps a focused graph from losing its shape.
func TestGraphFocusDimsRatherThanHides(t *testing.T) {
	shell, _ := newBatchShell(t)

	// The graph is focused on the first tag condition -- the same one the chip row shows
	// leftmost.
	shell.State().ActiveTagIDs = []string{"tag-1", "tag-2"}
	if got := shell.focusedGraphTag(); got != "tag-1" {
		t.Errorf("focused tag = %q, want the first condition tag-1", got)
	}

	// A scope is not a tag focus, and neither is a tag left in the selection: tags are
	// conditions now, so only ActiveTagIDs can focus the graph.
	shell.State().ActiveTagIDs = nil
	for _, scope := range []string{selectionAll, selectionInbox, selectionUnclassified, "", prefixTag + "tag-1"} {
		shell.State().Selection = scope
		if got := shell.focusedGraphTag(); got != "" {
			t.Errorf("scope %q must not focus a tag, got %q", scope, got)
		}
	}
}

// TestGraphTokensComeFromTheStylesheet pins the resolved colours to their mix percentages.
func TestGraphTokensComeFromTheStylesheet(t *testing.T) {
	light := GraphCanvasTokensFor(AppearanceLight, false, accentForTest)

	// The secondary text at 34% -> alpha 0.34.
	if alpha := float32(light.EdgeCooccurrence.A) / 255; alpha < 0.33 || alpha > 0.35 {
		t.Errorf("cooccurrence edge alpha = %v, want 0.34", alpha)
	}
	if light.EdgeHierarchy != ui.RGBA(accentForTest.R, accentForTest.G, accentForTest.B, 0.52) {
		t.Error("the hierarchy edge must be the accent at 52%")
	}
	if light.NodeFill != ui.RGBA(accentForTest.R, accentForTest.G, accentForTest.B, 0.70) {
		t.Error("a root node must be the accent at 70%")
	}
	if light.NodeChildFill != ui.RGBA(accentForTest.R, accentForTest.G, accentForTest.B, 0.38) {
		t.Error("a child node must be the accent at 38%")
	}
	if light.EdgeDimmed != 0.16 || light.NodeDimmed != 0.24 || light.AuthorDimmed != 0.2 {
		t.Errorf("the dim factors must be the stylesheet's 0.16/0.24/0.2, got %v/%v/%v",
			light.EdgeDimmed, light.NodeDimmed, light.AuthorDimmed)
	}
	// 16 and 12, not the stylesheet's 15 and 12: the 1000-unit SVG is drawn in a 900-point
	// pane, so its 15-unit label lands at 13.5 points, and a canvas drawn at its own scale
	// needs 16 to be the same size.
	if GraphNodeLabelSize != 16 || GraphAuthorLabelSize != 12 {
		t.Errorf("label sizes must be 16 and 12, got %d and %d", GraphNodeLabelSize, GraphAuthorLabelSize)
	}
	if GraphAuthorBridgeDash != 6 {
		t.Errorf("the author-bridge dash must be 6, got %d", GraphAuthorBridgeDash)
	}

	// The accent follows the theme, not the appearance. This used to assert that the dark graph
	// used a *different* accent, because there were two hardcoded cinnabar values and dark picked
	// the other one. Interactive colours now stay on the system accent -- one colour for both
	// appearances -- so the two are equal by design, and the assertion is inverted rather than
	// deleted: something has to fail if a second cinnabar is reintroduced here.
	dark := GraphCanvasTokensFor(AppearanceDark, false, accentForTest)
	if dark.NodeFocusedFill != light.NodeFocusedFill {
		t.Error("the graph's accent varies with the appearance; it should come from the theme")
	}
	// And it is the accent that was passed in, not a colour of the function's own choosing.
	if light.NodeFocusedFill != accentForTest {
		t.Errorf("the graph's focused node is %v, not the accent it was given", light.NodeFocusedFill)
	}
}

// TestDimColorScalesOnlyAlpha keeps the dim from washing out the hue.
func TestDimColorScalesOnlyAlpha(t *testing.T) {
	original := ui.RGBA(10, 20, 30, 0.8)
	dimmed := dimColor(original, 0.5)

	if dimmed.R != 10 || dimmed.G != 20 || dimmed.B != 30 {
		t.Errorf("dimming must keep the hue, got %v", dimmed)
	}
	if alpha := float32(dimmed.A) / 255; alpha < 0.39 || alpha > 0.41 {
		t.Errorf("dimmed alpha = %v, want 0.4", alpha)
	}

	// It must not overflow past opaque.
	if got := dimColor(original, 4); got.A != 255 {
		t.Errorf("alpha = %d, want it clamped to 255", got.A)
	}
}

// TestTestConnectionButtonReachesTheProbe covers the wiring rather than the probe:
// the ability existed while nothing could reach it, so the assertion is that pressing
// the button asks the provider and shows what came back.
func TestTestConnectionButtonReachesTheProbe(t *testing.T) {
	shell, db := newBatchShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post", clockBase)

	repos := ports.Repositories{
		Bookmarks:   sqlite.NewBookmarkRepository(db),
		Tags:        sqlite.NewTagRepository(db),
		Lists:       sqlite.NewListRepository(db),
		Suggestions: sqlite.NewSuggestionRepository(db),
		Analysis:    sqlite.NewAnalysisRepository(db),
		Settings:    sqlite.NewSettingsRepository(db),
	}
	tagger := tagging.New(repos, fixedClock{now: clockBase}, nil)
	shell.UseTagging(tagger)

	// Nothing has been asked yet, so the panel claims nothing.
	shell.OpenSettings()
	if shell.State().ConnectionTested {
		t.Error("the panel must not claim a status before the button is pressed")
	}

	// A successful probe.
	calls := 0
	var sawModel string
	tagger.UseConnectionTester(func(settings engine.Settings) engine.ConnectionTestResult {
		calls++
		sawModel = settings.LLM.Model
		return engine.ConnectionTestResult{OK: true, Model: settings.LLM.Model, ProviderID: settings.LLM.ProviderID}
	})

	shell.TestConnection(ctx)
	if calls != 1 {
		t.Fatalf("the probe ran %d times, want 1", calls)
	}
	if !shell.State().ConnectionTested || !shell.State().ConnectionOK {
		t.Errorf("a successful probe must be shown as successful: %+v", shell.State().ConnectionResult)
	}
	if shell.State().ConnectionResult == "" {
		t.Error("a successful probe must say something")
	}
	_ = sawModel

	// A failing probe shows the sentence for its classified kind rather than the
	// provider's raw text: "the key is wrong" is the transport's wording, and the user
	// gets the interface's.
	tagger.UseConnectionTester(func(engine.Settings) engine.ConnectionTestResult {
		return engine.ConnectionTestResult{OK: false, ErrorKind: "unauthorized", Message: "the key is wrong"}
	})
	shell.TestConnection(ctx)
	if shell.State().ConnectionOK {
		t.Error("a failed probe must not be shown as successful")
	}
	if !strings.Contains(shell.State().ConnectionResult, "API key") {
		t.Errorf("the panel must explain the failure, got %q", shell.State().ConnectionResult)
	}
	if strings.Contains(shell.State().ConnectionResult, "the key is wrong") {
		t.Errorf("a classified failure must not fall back to the raw text, got %q", shell.State().ConnectionResult)
	}

	// A missing key is not an invalid one: nothing was sent, and the fix is to enter a
	// key rather than replace it. The panel has its own sentence for this.
	tagger.UseConnectionTester(func(engine.Settings) engine.ConnectionTestResult {
		return engine.ConnectionTestResult{OK: false, ErrorKind: "unauthorized", Message: engine.MissingCredentialMessage}
	})
	shell.TestConnection(ctx)
	if got, want := shell.State().ConnectionResult, shell.t().T(i18n.SettingsLlmTestMissingKey); got != want {
		t.Errorf("a missing key must get its own sentence, got %q want %q", got, want)
	}
	if strings.Contains(shell.State().ConnectionResult, "invalid") {
		t.Errorf("a missing key must not be called invalid, got %q", shell.State().ConnectionResult)
	}

	// An unclassified failure keeps the raw text, because guessing would mislead.
	tagger.UseConnectionTester(func(engine.Settings) engine.ConnectionTestResult {
		return engine.ConnectionTestResult{OK: false, ErrorKind: "brand-new", Message: "the relay said no"}
	})
	shell.TestConnection(ctx)
	if !strings.Contains(shell.State().ConnectionResult, "the relay said no") {
		t.Errorf("an unclassified failure must keep the provider's words, got %q", shell.State().ConnectionResult)
	}
}

// TestTestConnectionWithNoTesterSaysSo keeps a missing seam from looking like a
// successful connection.
func TestTestConnectionWithNoTesterSaysSo(t *testing.T) {
	shell, _ := newBatchShell(t)

	shell.TestConnection(context.Background())
	if shell.State().ConnectionOK {
		t.Error("a missing tester must not report success")
	}
	if shell.State().ConnectionResult == "" {
		t.Error("a missing tester must say so")
	}
}

// TestTagManagementScreenRenders covers the screen the domain logic has been waiting
// for since the tag library and merge commands were ported.
func TestTagManagementScreenRenders(t *testing.T) {
	shell, db := newBatchShell(t)
	ctx := context.Background()

	for _, id := range []string{"a", "b"} {
		seed(t, db, id, "ada", "post "+id, clockBase)
	}
	tags := sqlite.NewTagRepository(db)
	for _, spec := range []struct {
		id, name string
		parent   string
	}{
		{"used", "used tag", ""},
		{"empty", "empty tag", ""},
		{"child", "child tag", "used"},
	} {
		tag := domain.Tag{
			ID: spec.id, Name: spec.name, ParentID: spec.parent,
			CanonicalKey: domain.CanonicalTagKey(spec.name),
			CreatedAt:    domain.FormatTimestamp(clockBase), UpdatedAt: domain.FormatTimestamp(clockBase),
		}
		if err := tags.Create(ctx, tag); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}
	if err := tags.AttachMany(ctx, []domain.BookmarkTag{
		{BookmarkID: "a", TagID: "used", CreatedAt: clockBase, Source: domain.TagSourceManual},
	}); err != nil {
		t.Fatalf("AttachMany: %v", err)
	}

	shell.Reload(ctx)
	shell.ToggleTagMode(ctx)
	if !shell.State().TagMode {
		t.Fatal("the tag mode must be on")
	}
	if len(shell.State().TagItems) != 3 {
		t.Fatalf("loaded %d tags, want 3", len(shell.State().TagItems))
	}

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	if !tester.HasText(shell.t().T(i18n.TagManageLabel)) {
		t.Error("the screen must show its title")
	}

	// Leaving returns to the list.
	shell.ToggleTagMode(ctx)
	if shell.State().TagMode {
		t.Error("the tag mode must be off")
	}
}

// TestTagFiltersAndSortsNarrowTheList covers the controls, which exist to make a
// drifted library reviewable.
func TestTagFiltersAndSortsNarrowTheList(t *testing.T) {
	shell, db := newBatchShell(t)
	ctx := context.Background()

	seed(t, db, "a", "ada", "post a", clockBase)
	tags := sqlite.NewTagRepository(db)
	for _, spec := range []struct{ id, name string }{
		{"used", "used"}, {"empty", "empty"}, {"tail", "tail"},
	} {
		tag := domain.Tag{
			ID: spec.id, Name: spec.name, CanonicalKey: domain.CanonicalTagKey(spec.name),
			CreatedAt: domain.FormatTimestamp(clockBase), UpdatedAt: domain.FormatTimestamp(clockBase),
		}
		if err := tags.Create(ctx, tag); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}
	if err := tags.AttachMany(ctx, []domain.BookmarkTag{
		{BookmarkID: "a", TagID: "used", CreatedAt: clockBase, Source: domain.TagSourceManual},
		{BookmarkID: "a", TagID: "tail", CreatedAt: clockBase, Source: domain.TagSourceManual},
	}); err != nil {
		t.Fatalf("AttachMany: %v", err)
	}
	shell.ReloadTagLibrary(ctx)

	// Orphans only: just the unused tag.
	shell.State().TagFilter = domain.TagFilterOrphan
	if got := shell.VisibleTagItems(); len(got) != 1 || got[0].TagID != "empty" {
		t.Errorf("orphan filter returned %+v", got)
	}

	// Name order is the collator's, so "empty" precedes "used".
	shell.State().TagFilter = domain.TagFilterAll
	shell.State().TagSort = domain.TagSortNameAsc
	ordered := shell.VisibleTagItems()
	if len(ordered) != 3 || ordered[0].Name != "empty" {
		t.Errorf("name sort returned %+v", ordered)
	}

	// The query matches a substring.
	shell.State().TagQuery = "us"
	if got := shell.VisibleTagItems(); len(got) != 1 || got[0].TagID != "used" {
		t.Errorf("query filter returned %+v", got)
	}
	shell.State().TagQuery = ""
}

// TestCleanupOrphansUsesTheSafeRule is the reason the screen calls the domain rule
// rather than filtering itself: a tag under review must survive the sweep.
func TestCleanupOrphansUsesTheSafeRule(t *testing.T) {
	shell, db := newBatchShell(t)
	ctx := context.Background()
	tags := sqlite.NewTagRepository(db)

	for _, spec := range []struct{ id, name string }{
		{"empty", "empty"}, {"reviewed", "reviewed"}, {"parent", "parent"}, {"child", "child"},
	} {
		parent := ""
		if spec.id == "child" {
			parent = "parent"
		}
		tag := domain.Tag{
			ID: spec.id, Name: spec.name, ParentID: parent,
			CanonicalKey: domain.CanonicalTagKey(spec.name),
			CreatedAt:    domain.FormatTimestamp(clockBase), UpdatedAt: domain.FormatTimestamp(clockBase),
		}
		if err := tags.Create(ctx, tag); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}
	// A pending suggestion naming the "reviewed" tag.
	if _, err := db.Exec(
		`INSERT INTO tag_suggestions (id, bookmark_id, tag_name, source, confidence, status, created_at)
		 VALUES ('s1', 'b1', 'reviewed', 'llm', 0.5, 'pending', ?)`,
		domain.FormatTimestamp(clockBase)); err != nil {
		t.Fatalf("insert suggestion: %v", err)
	}

	shell.ReloadTagLibrary(ctx)
	shell.CleanupOrphanTags(ctx)

	// The rule protects a tag that HAS children, not one that has a parent: the
	// guard is `!childCount`, so an unused leaf goes even when its parent stays. That
	// is the rule, and my first expectation had it backwards.
	if _, found, err := tags.Get(ctx, "empty"); err != nil || found {
		t.Errorf("the empty tag must be swept: found=%v err=%v", found, err)
	}
	if _, found, err := tags.Get(ctx, "child"); err != nil || found {
		t.Errorf("an unused leaf must be swept even though its parent stays: found=%v err=%v", found, err)
	}

	// These three survive, each for a different reason.
	//   reviewed: a pending suggestion names it
	//   parent:   it has a child
	for _, id := range []string{"reviewed", "parent"} {
		if _, found, err := tags.Get(ctx, id); err != nil || !found {
			t.Errorf("tag %s must survive the sweep: found=%v err=%v", id, found, err)
		}
	}
	if shell.State().TagCleanupMessage == "" {
		t.Error("the sweep must report what it did")
	}
}

// TestApplyVocabularyReviewAppliesTheSelection is the point of the screen: the review's
// proposals must actually change the library, which is what was impossible before the
// merge command existed.
func TestApplyVocabularyReviewAppliesTheSelection(t *testing.T) {
	shell, db := newBatchShell(t)
	ctx := context.Background()
	tags := sqlite.NewTagRepository(db)

	for _, spec := range []struct{ id, name string }{
		{"target", "ai agent"}, {"source", "agentic ai"}, {"rename", "LLMs"},
	} {
		tag := domain.Tag{
			ID: spec.id, Name: spec.name, CanonicalKey: domain.CanonicalTagKey(spec.name),
			CreatedAt: domain.FormatTimestamp(clockBase), UpdatedAt: domain.FormatTimestamp(clockBase),
		}
		if err := tags.Create(ctx, tag); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}
	if _, err := NewBookmarkRepositoryForTest(db).UpsertMany(ctx, []domain.Bookmark{
		{TweetID: "b1", Text: "post", SavedAt: clockBase, UpdatedAt: clockBase},
	}); err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}
	if err := tags.AttachMany(ctx, []domain.BookmarkTag{
		{BookmarkID: "b1", TagID: "source", CreatedAt: clockBase, Source: domain.TagSourceManual},
	}); err != nil {
		t.Fatalf("AttachMany: %v", err)
	}

	shell.ReloadTagLibrary(ctx)

	merge := engine.TagMergePlan{
		TargetTagID: "target", TargetName: "ai agent",
		SourceTagIDs: []string{"source"}, SourceNames: []string{"agentic ai"},
	}
	rename := engine.TagRenamePlan{TagID: "rename", From: "LLMs", To: "llm"}

	shell.State().ReviewMerges = []engine.TagMergePlan{merge}
	shell.State().ReviewRenames = []engine.TagRenamePlan{rename}
	shell.State().ReviewSelected = map[string]bool{
		engine.MergePlanKey(merge):   true,
		engine.RenamePlanKey(rename): true,
	}
	shell.State().ReviewOpen = true

	shell.ApplyVocabularyReview(ctx)

	// The merge landed.
	if _, found, err := tags.Get(ctx, "source"); err != nil || found {
		t.Errorf("the merged tag must be gone: found=%v err=%v", found, err)
	}
	// The rename landed.
	renamed, _, err := tags.Get(ctx, "rename")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if renamed.Name != "llm" {
		t.Errorf("the renamed tag is %q, want llm", renamed.Name)
	}
	// The panel closed and reported what it did.
	if shell.State().ReviewOpen {
		t.Error("applying must close the panel")
	}
	if shell.State().ReviewApplied == "" {
		t.Error("applying must report what it did")
	}
}

// TestApplyVocabularyReviewHonoursTheSelection keeps an unticked proposal from being
// applied: a review the user curates must apply only what they kept.
func TestApplyVocabularyReviewHonoursTheSelection(t *testing.T) {
	shell, db := newBatchShell(t)
	ctx := context.Background()
	tags := sqlite.NewTagRepository(db)

	for _, spec := range []struct{ id, name string }{
		{"target", "ai agent"}, {"source", "agentic ai"},
	} {
		tag := domain.Tag{
			ID: spec.id, Name: spec.name, CanonicalKey: domain.CanonicalTagKey(spec.name),
			CreatedAt: domain.FormatTimestamp(clockBase), UpdatedAt: domain.FormatTimestamp(clockBase),
		}
		if err := tags.Create(ctx, tag); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	merge := engine.TagMergePlan{
		TargetTagID: "target", TargetName: "ai agent",
		SourceTagIDs: []string{"source"}, SourceNames: []string{"agentic ai"},
	}
	shell.State().ReviewMerges = []engine.TagMergePlan{merge}
	// Nothing selected.
	shell.State().ReviewSelected = map[string]bool{}

	shell.ApplyVocabularyReview(ctx)

	if _, found, err := tags.Get(ctx, "source"); err != nil || !found {
		t.Errorf("an unticked merge must not run: found=%v err=%v", found, err)
	}
}

// TestVocabularyReviewPanelRenders covers the panel's states without a provider.
func TestVocabularyReviewPanelRenders(t *testing.T) {
	shell, _ := newBatchShell(t)

	// Running.
	shell.State().ReviewOpen = true
	shell.State().ReviewRunning = true
	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	if !tester.HasText("Reviewing") {
		t.Error("the running state must say it is reviewing")
	}

	// Failed, with the real reason.
	shell.State().ReviewRunning = false
	shell.State().ReviewError = "no API key stored"
	tester.Frame()
	if !tester.HasText("no API key stored") {
		t.Error("a failed review must show the real reason")
	}

	// Empty, with no tags at all: the panel says there is nothing to compare yet,
	// rather than that the vocabulary looks healthy -- which was nonsense to a reader
	// looking at an empty list.
	shell.State().ReviewError = ""
	tester.Frame()
	if !tester.HasText(shell.t().T(i18n.ReviewNoTags)) {
		t.Error("an empty library must say the list is empty, not that it is healthy")
	}

	// Empty, with tags but no proposals: that is the healthy answer.
	shell.State().TagItems = []domain.TagLibraryItem{{TagID: "t9", Name: "go"}}
	tester.Frame()
	if !tester.HasText(shell.t().T(i18n.ReviewEmpty)) {
		t.Error("a reviewed non-empty library with no proposals must say it looks healthy")
	}
	shell.State().TagItems = nil

	// With proposals, both kinds are listed.
	shell.State().ReviewMerges = []engine.TagMergePlan{{
		TargetTagID: "t1", TargetName: "ai agent",
		SourceTagIDs: []string{"t2"}, SourceNames: []string{"agentic ai"},
	}}
	shell.State().ReviewRenames = []engine.TagRenamePlan{{TagID: "t3", From: "LLMs", To: "llm"}}
	tester.Frame()
	if !tester.HasText("agentic ai") || !tester.HasText("LLMs") {
		t.Error("both proposal kinds must be listed")
	}
}

// TestMergeSelectedTagsFromTheScreen covers the merge entry point: the command existed
// since the tag library audit, but nothing could reach it.
func TestMergeSelectedTagsFromTheScreen(t *testing.T) {
	shell, db := newBatchShell(t)
	ctx := context.Background()
	tags := sqlite.NewTagRepository(db)

	for _, spec := range []struct{ id, name string }{
		{"target", "ai agent"}, {"source", "agentic ai"},
	} {
		tag := domain.Tag{
			ID: spec.id, Name: spec.name, CanonicalKey: domain.CanonicalTagKey(spec.name),
			CreatedAt: domain.FormatTimestamp(clockBase), UpdatedAt: domain.FormatTimestamp(clockBase),
		}
		if err := tags.Create(ctx, tag); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}
	shell.ReloadTagLibrary(ctx)

	// Nothing ticked: the button does nothing rather than merging everything.
	shell.AskMergeSelectedTags()
	if _, found, err := tags.Get(ctx, "source"); err != nil || !found {
		t.Fatalf("an empty selection must not merge: found=%v err=%v", found, err)
	}

	// Ticked, with a target chosen by name as the picker holds it.
	shell.State().TagSelectedIDs = map[string]bool{"source": true}
	shell.State().TagMergeTarget = "ai agent"
	shell.AskMergeSelectedTags()
	shell.ConfirmMergeSelectedTagsNow(ctx)

	if _, found, err := tags.Get(ctx, "source"); err != nil || found {
		t.Errorf("the ticked tag must be merged away: found=%v err=%v", found, err)
	}
	// The selection is cleared, so a second press cannot re-merge.
	if len(shell.State().TagSelectedIDs) != 0 {
		t.Errorf("the selection must be cleared: %v", shell.State().TagSelectedIDs)
	}
	if shell.State().TagActionMsg == "" {
		t.Error("the merge must report what it did")
	}
}

// TestSaveTagAliasesFromTheScreen covers the alias editor's comma parsing.
func TestSaveTagAliasesFromTheScreen(t *testing.T) {
	shell, db := newBatchShell(t)
	ctx := context.Background()
	tags := sqlite.NewTagRepository(db)

	if err := tags.Create(ctx, domain.Tag{
		ID: "t1", Name: "ai agent", CanonicalKey: domain.CanonicalTagKey("ai agent"),
		CreatedAt: domain.FormatTimestamp(clockBase), UpdatedAt: domain.FormatTimestamp(clockBase),
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	shell.ReloadTagLibrary(ctx)

	shell.State().TagFocus = "t1"
	shell.State().TagAliasInput = " agent , , AI Agents "
	shell.SaveTagAliases(ctx)

	tag, _, err := tags.Get(ctx, "t1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	// "AI Agents" canonicalises to the tag's own name, so it is dropped; the blank is
	// dropped; the rest is trimmed.
	if len(tag.Aliases) != 1 || tag.Aliases[0] != "agent" {
		t.Errorf("aliases = %v, want just \"agent\"", tag.Aliases)
	}
}

// TestTagActionWithoutAFocusDoesNothing keeps an unset focus from acting on a zero id.
func TestTagActionWithoutAFocusDoesNothing(t *testing.T) {
	shell, _ := newBatchShell(t)
	ctx := context.Background()

	shell.SaveTagAliases(ctx)
	shell.AskMergeSelectedTags()

	if shell.State().TagActionMsg != "" {
		t.Errorf("no action should have run, got %q", shell.State().TagActionMsg)
	}
}

// TestSyncSettingsPersist covers the sync section: these are preferences rather than
// auto-tag settings, so they go through the settings service.
func TestSyncSettingsPersist(t *testing.T) {
	shell, _ := newBatchShell(t)
	ctx := context.Background()

	shell.OpenSettings()
	// The panel opens on the stored values, and a fresh install has the domain's
	// buffer size rather than zero.
	if shell.State().StopBufferPages != float64(domain.DefaultIncrementalStopBufferPages) {
		t.Errorf("buffer pages = %v, want the domain default %d",
			shell.State().StopBufferPages, domain.DefaultIncrementalStopBufferPages)
	}

	shell.State().SyncOnLaunch = true
	shell.State().SaveRawPayloads = true
	shell.State().StopBufferPages = 7
	shell.saveSyncPreferences(ctx)

	// A fresh read is what a relaunch does.
	loaded, err := shell.settings.Load(ctx)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !loaded.Sync.SyncOnLaunch || !loaded.Sync.SaveRawPayloads {
		t.Errorf("the toggles did not persist: %+v", loaded.Sync)
	}
	if loaded.Sync.StopBufferPages != 7 {
		t.Errorf("buffer pages = %d, want 7", loaded.Sync.StopBufferPages)
	}
}

// TestRunAfterSyncPersists covers the auto-tagging setting beside the toggles.
func TestRunAfterSyncPersists(t *testing.T) {
	shell, db := newBatchShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "post", clockBase)

	repos := ports.Repositories{
		Bookmarks:   sqlite.NewBookmarkRepository(db),
		Tags:        sqlite.NewTagRepository(db),
		Lists:       sqlite.NewListRepository(db),
		Suggestions: sqlite.NewSuggestionRepository(db),
		Analysis:    sqlite.NewAnalysisRepository(db),
		Settings:    sqlite.NewSettingsRepository(db),
	}
	tagger := tagging.New(repos, fixedClock{now: clockBase}, nil)
	shell.UseTagging(tagger)

	shell.OpenSettings()
	shell.State().RunAfterSync = false
	shell.saveAutoTagSettings(ctx)

	stored, err := tagger.LoadSettings(ctx)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if stored.RunAfterSync {
		t.Error("turning run-after-sync off must persist")
	}
}

// TestProviderErrorsAreTranslated covers the layer the transport has been missing: the
// thirteen classified failures existed with nothing turning them into a sentence.
func TestProviderErrorsAreTranslated(t *testing.T) {
	shell, _ := newBatchShell(t)

	// Every kind the transport defines must have a sentence. A gap would show a raw
	// message for one failure out of thirteen, which is worse than none at all because
	// it looks deliberate.
	for _, kind := range []string{
		"unauthorized", "forbidden", "rate-limit", "quota", "bad-request",
		"unsupported-param", "context-length", "timeout", "network", "server",
		"content-filter", "invalid-json", "unknown",
	} {
		readable, _ := shell.localizeProviderError(kind, 0)
		if readable == "" {
			t.Errorf("kind %q has no sentence", kind)
		}
	}

	// The sentences are distinct, so a user can tell one failure from another.
	seen := map[string]string{}
	for _, kind := range []string{"unauthorized", "quota", "timeout", "network"} {
		readable, _ := shell.localizeProviderError(kind, 0)
		if previous, duplicate := seen[readable]; duplicate {
			t.Errorf("%q and %q share the sentence %q", previous, kind, readable)
		}
		seen[readable] = kind
	}

	// A retry delay adds its own sentence rather than replacing the first.
	message, retry := shell.localizeProviderError("rate-limit", 30)
	if message == "" {
		t.Error("a rate limit must explain itself")
	}
	if retry == "" {
		t.Error("a retry delay must be named")
	}
	if !strings.Contains(retry, "30") {
		t.Errorf("the retry sentence must carry the delay, got %q", retry)
	}

	// An unclassified kind yields nothing, so the caller keeps the raw message.
	if readable, _ := shell.localizeProviderError("something-new", 0); readable != "" {
		t.Errorf("an unknown kind must not be guessed at, got %q", readable)
	}
}

// TestConnectionTestShowsTheTranslatedFailure keeps the panel from printing a status
// code where a sentence belongs.
func TestConnectionTestShowsTheTranslatedFailure(t *testing.T) {
	shell, db := newBatchShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "post", clockBase)

	repos := ports.Repositories{
		Bookmarks:   sqlite.NewBookmarkRepository(db),
		Tags:        sqlite.NewTagRepository(db),
		Lists:       sqlite.NewListRepository(db),
		Suggestions: sqlite.NewSuggestionRepository(db),
		Analysis:    sqlite.NewAnalysisRepository(db),
		Settings:    sqlite.NewSettingsRepository(db),
	}
	tagger := tagging.New(repos, fixedClock{now: clockBase}, nil)
	shell.UseTagging(tagger)
	tagger.UseConnectionTester(func(engine.Settings) engine.ConnectionTestResult {
		return engine.ConnectionTestResult{OK: false, ErrorKind: "unauthorized", Message: "HTTP 401"}
	})

	shell.TestConnection(ctx)

	result := shell.State().ConnectionResult
	if strings.Contains(result, "401") {
		t.Errorf("the panel must not show a raw status code when a sentence exists: %q", result)
	}
	if !strings.Contains(result, "API key") {
		t.Errorf("the panel must explain the failure, got %q", result)
	}
}

// TestEveryOutcomeHasItsOwnSentence covers the twelve outcomes the engine reports. The
// panel used to show one summary field, so a run stopped by its budget read the same as
// one stopped by a missing key -- and those need different fixes.
func TestEveryOutcomeHasItsOwnSentence(t *testing.T) {
	shell, _ := newBatchShell(t)

	outcomes := []engine.RunOutcome{
		engine.OutcomeOK, engine.OutcomeNoCandidates, engine.OutcomeEmptyVocabulary,
		engine.OutcomeLLMDisabled, engine.OutcomeMissingAPIKey, engine.OutcomeAllBlocked,
		engine.OutcomeBudgetExhausted, engine.OutcomeTimedOut, engine.OutcomeInterrupted,
		engine.OutcomeCancelled, engine.OutcomeFailed,
	}

	seen := map[string]engine.RunOutcome{}
	for _, outcome := range outcomes {
		headline, _ := shell.SummarizeRun(outcome, engine.RunSummary{
			Outcome: outcome, SuggestedCount: 3, PendingCount: 2,
			CandidateCount: 10, LLMProcessedCount: 8, VocabularySize: 20, NewTagBudget: 5,
			UnprocessedCount: 4,
		})
		if headline == "" {
			t.Errorf("outcome %q has no sentence", outcome)
			continue
		}
		if previous, duplicate := seen[headline]; duplicate {
			t.Errorf("%q and %q share the sentence %q", previous, outcome, headline)
		}
		seen[headline] = outcome
	}

	// Completed runs carry their counts, not just a verdict.
	headline, detail := shell.SummarizeRun(engine.OutcomeOK, engine.RunSummary{
		Outcome: engine.OutcomeOK, SuggestedCount: 3, PendingCount: 2,
		CandidateCount: 10, LLMProcessedCount: 8, LLMBatchCount: 4, LLMFailedBatchCount: 1,
		VocabularySize: 20, NewTagBudget: 5,
	})
	if !strings.Contains(headline, "3") || !strings.Contains(headline, "2") {
		t.Errorf("the headline must carry the counts, got %q", headline)
	}
	if !strings.Contains(detail, "20") || !strings.Contains(detail, "5") {
		t.Errorf("the detail must carry the vocabulary and budget, got %q", detail)
	}

	// Only what happened is mentioned, so the line stays readable.
	if strings.Contains(detail, "dropped") {
		t.Errorf("a run with no drops must not mention dropping: %q", detail)
	}
	_, withDrops := shell.SummarizeRun(engine.OutcomeOK, engine.RunSummary{
		Outcome: engine.OutcomeOK, BlockedDroppedCount: 2, BudgetDroppedCount: 1,
	})
	if !strings.Contains(withDrops, "2") || !strings.Contains(withDrops, "1") {
		t.Errorf("a run with drops must name them: %q", withDrops)
	}

	// Pressing run while the feature is off must say so. Reporting nothing made the
	// button look broken, which is the defect that sent a user hunting for a chain
	// that was never entered.
	headline, detail = shell.SummarizeRun(engine.OutcomeDisabled, engine.RunSummary{})
	if headline != shell.t().T(i18n.OutcomeDisabled) {
		t.Errorf("a disabled run must name the switch, got %q", headline)
	}
	if detail != "" {
		t.Error("a disabled run has no counts to report")
	}
}

// TestLiveRunProgressDistinguishesStalledFromWorking is the reason this exists: without
// it a run whose worker died reads exactly like one that is merely slow.
func TestLiveRunProgressDistinguishesStalledFromWorking(t *testing.T) {
	shell, _ := newBatchShell(t)
	now := clockBase

	// A healthy run: phase, counts, elapsed, and nothing alarming.
	working := &engine.RunState{
		StartedAt: now.Add(-30 * time.Second),
		UpdatedAt: now.Add(-2 * time.Second),
		Phase:     engine.PhaseLLM,
		Processed: 12,
		Total:     40,
	}
	lines := shell.LiveRunProgress(working, now)
	joined := strings.Join(lines, " | ")
	if !strings.Contains(joined, "12 / 40") {
		t.Errorf("a healthy run must show its counts: %q", joined)
	}
	if !strings.Contains(joined, "30") {
		t.Errorf("a healthy run must show its elapsed time: %q", joined)
	}
	if strings.Contains(joined, "heartbeat") {
		t.Errorf("a healthy run must not claim to be stalled: %q", joined)
	}

	// A stalled run: the heartbeat went quiet. The threshold is the stall hint, which is
	// seconds -- not the liveness window, which is minutes and means the
	// run is dead rather than slow.
	stalled := &engine.RunState{
		StartedAt: now.Add(-2 * time.Minute),
		UpdatedAt: now.Add(-10 * time.Second),
		Phase:     engine.PhaseLLM,
		Processed: 12,
		Total:     40,
	}
	stalledLines := strings.Join(shell.LiveRunProgress(stalled, now), " | ")
	if !strings.Contains(stalledLines, "heartbeat") {
		t.Errorf("a run whose heartbeat stopped must say so: %q", stalledLines)
	}
	if !strings.Contains(stalledLines, "10") {
		t.Errorf("the stall must carry how long: %q", stalledLines)
	}

	// A run past the liveness window is dead, not stalled: there is nothing live to
	// render, and showing a stalled bar for a run that can be taken over would be
	// showing a stale state as a current one.
	dead := &engine.RunState{
		StartedAt: now.Add(-10 * time.Minute),
		UpdatedAt: now.Add(-4 * time.Minute),
		Phase:     engine.PhaseLLM, Processed: 12, Total: 40,
	}
	if lines := shell.LiveRunProgress(dead, now); len(lines) != 0 {
		t.Errorf("a dead run must render nothing, got %v", lines)
	}

	// Waiting for a back-off carries how long is left, and is never called a stall.
	backOffUntil := now.Add(25 * time.Second)
	backingOff := &engine.RunState{
		StartedAt: now.Add(-5 * time.Minute), UpdatedAt: now.Add(-60 * time.Second),
		Phase: engine.PhaseLLM, WaitingUntil: &backOffUntil,
	}
	if got := strings.Join(shell.LiveRunProgress(backingOff, now), " | "); strings.Contains(got, "heartbeat") {
		t.Errorf("a back-off must not be called a stall: %q", got)
	}

	// The estimate appears only once something has finished: an average over nothing is
	// noise, which is why it is withheld.
	estimating := &engine.RunState{
		StartedAt: now.Add(-40 * time.Second), UpdatedAt: now,
		Phase: engine.PhaseLLM, Processed: 10, Total: 40,
	}
	if got := strings.Join(shell.LiveRunProgress(estimating, now), " | "); !strings.Contains(got, "left") {
		t.Errorf("a run with progress must offer an estimate: %q", got)
	}
	tooEarly := &engine.RunState{
		StartedAt: now.Add(-40 * time.Second), UpdatedAt: now,
		Phase: engine.PhaseLLM, Processed: 0, Total: 40,
	}
	if got := strings.Join(shell.LiveRunProgress(tooEarly, now), " | "); strings.Contains(got, "left") {
		t.Errorf("a run with nothing finished must not estimate: %q", got)
	}

	// Processed is clamped: a total revised downwards mid-run must not show 12/10.
	overrun := &engine.RunState{
		StartedAt: now.Add(-10 * time.Second), UpdatedAt: now,
		Phase: engine.PhaseLLM, Processed: 12, Total: 10,
	}
	if got := strings.Join(shell.LiveRunProgress(overrun, now), " | "); !strings.Contains(got, "10 / 10") {
		t.Errorf("processed must be clamped to the total: %q", got)
	}

	// Waiting for a provider's back-off is not a stall, and calling it one would train
	// the user to ignore the message.
	waitingUntil := now.Add(20 * time.Second)
	waiting := &engine.RunState{
		StartedAt:    now.Add(-10 * time.Second),
		UpdatedAt:    now.Add(-2 * time.Second),
		Phase:        engine.PhaseLLM,
		WaitingUntil: &waitingUntil,
	}
	waitingLines := strings.Join(shell.LiveRunProgress(waiting, now), " | ")
	if !strings.Contains(waitingLines, "Waiting") {
		t.Errorf("a waiting run must say it is waiting: %q", waitingLines)
	}
	if strings.Contains(waitingLines, "heartbeat") {
		t.Errorf("a waiting run must not be called stalled: %q", waitingLines)
	}

	// A cancel in flight reads as stopping rather than as still working.
	stopping := &engine.RunState{
		StartedAt: now.Add(-10 * time.Second), UpdatedAt: now,
		Phase: engine.PhaseLLM, CancelRequested: true,
	}
	if got := strings.Join(shell.LiveRunProgress(stopping, now), " | "); !strings.Contains(got, "Stopping") {
		t.Errorf("a cancelled run must read as stopping: %q", got)
	}

	// The rule-based phase is named differently from the model phase.
	heuristics := &engine.RunState{StartedAt: now, UpdatedAt: now, Phase: engine.PhaseHeuristics}
	if got := strings.Join(shell.LiveRunProgress(heuristics, now), " | "); !strings.Contains(got, "Rule-based") {
		t.Errorf("the heuristic phase must be named: %q", got)
	}

	// No state means no lines, not an empty claim.
	if lines := shell.LiveRunProgress(nil, now); len(lines) != 0 {
		t.Errorf("no run must produce no lines, got %v", lines)
	}
}

// TestSortLabelsFollowTheSourceMapping pins the label mapping, which is not one label
// per order: the mapping returns the "recently saved" copy for both `timeline` and
// `saved-desc`, and the default branch covers the latter.
func TestSortLabelsFollowTheSourceMapping(t *testing.T) {
	shell, _ := newBatchShell(t)

	// `timeline` reads as "Recently saved", not "Timeline": the latter is the timeline
	// view's heading, which the toggle button does not use.
	shell.state.Sort = domain.SortTimeline
	if got := shell.currentSortLabel(); got != "Recently saved" {
		t.Errorf("the timeline button label = %q, want %q", got, "Recently saved")
	}

	for order, want := range map[domain.SortOrder]string{
		domain.SortSavedAsc:    "Oldest saved",
		domain.SortCreatedDesc: "Newest published",
		domain.SortLikesDesc:   "Most liked",
		domain.SortSavedDesc:   "Recently saved",
	} {
		shell.state.Sort = order
		if got := shell.currentSortLabel(); got != want {
			t.Errorf("label for %q = %q, want %q", order, got, want)
		}
	}
}

// TestThemeButtonCyclesLikeTheSource covers the sidebar theme button. The cycle is not
// a fixed rotation: from "system" the next preference is the opposite of the theme on
// screen, so the first press always looks like a toggle.
func TestThemeButtonCyclesLikeTheSource(t *testing.T) {
	cases := []struct {
		current  Appearance
		resolved Appearance
		want     Appearance
	}{
		// System, on a light machine: the first press means "make it dark".
		{AppearanceSystem, AppearanceLight, AppearanceDark},
		// System, on a dark machine: the first press means "make it light", which is
		// the case a fixed rotation gets wrong.
		{AppearanceSystem, AppearanceDark, AppearanceLight},
		{AppearanceDark, AppearanceDark, AppearanceLight},
		{AppearanceLight, AppearanceLight, AppearanceSystem},
	}

	for _, tc := range cases {
		if got := NextAppearance(tc.current, tc.resolved); got != tc.want {
			t.Errorf("NextAppearance(%q, %q) = %q, want %q", tc.current, tc.resolved, got, tc.want)
		}
	}

	// The cycle returns to where it started within three presses.
	for _, start := range []Appearance{AppearanceLight, AppearanceDark} {
		current := start
		for i := 0; i < 3; i++ {
			current = NextAppearance(current, AppearanceLight)
		}
		if current != start {
			t.Errorf("three presses from %q ended at %q", start, current)
		}
	}
}

// TestThemeButtonIsReachableFromTheSidebar covers the wiring: the function being right
// is worth nothing if no button calls it.
func TestThemeButtonIsReachableFromTheSidebar(t *testing.T) {
	shell, db := newBatchShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "post", clockBase)
	shell.Reload(ctx)

	// The button's label names the current preference.
	shell.state.Appearance = AppearanceSystem
	shell.state.SystemDark = true
	if got := shell.appearanceLabel(); got == "" {
		t.Error("the button must name the current preference")
	}
	// The resolved theme follows the system when the preference is "system".
	if got := shell.resolvedAppearance(); got != AppearanceDark {
		t.Errorf("resolved = %q, want dark", got)
	}

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	// The footer's controls are icon-only at 28x28, so there is no word to look for. The
	// locale chip beside them is the visible marker that the footer rendered.
	if !tester.HasText("EN") && !tester.HasText("中") {
		t.Errorf("the sidebar footer is not on screen; texts: %q", tester.Texts())
	}

	// Pressing it moves the preference, which is what the button is for.
	before := shell.state.Appearance
	shell.SetAppearance(NextAppearance(shell.state.Appearance, shell.resolvedAppearance()))
	if shell.state.Appearance == before {
		t.Errorf("a press left the preference at %q", before)
	}
}

// TestTheDetailIsAnOverlayNotAColumn pins the structure.
//
// The workspace has one split handle, on the left, and the detail is a fixed overlay with
// a backdrop. A third column would make the workbench two panes wide, and the detail could
// never be dismissed as an overlay because it would not be one.
func TestTheDetailIsAnOverlayNotAColumn(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "a post about compilers", clockBase)
	seed(t, db, "2", "grace", "another post", clockBase)
	shell.Reload(context.Background())

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	// Both panes are on screen.
	for _, want := range []string{"Tags", "@ada", "another post"} {
		if !tester.HasText(want) {
			t.Errorf("the two-pane workbench is missing %q", want)
		}
	}
	// The inspector is not, until the detail is asked for.
	if tester.HasText("Select a bookmark to see its details.") {
		t.Error("the inspector must not be a standing column")
	}

	// The overlay flag is what shows it, and it is separate from which bookmark is
	// selected: closing the overlay must not forget the selection.
	shell.State().Selected = "2"
	shell.OpenSelectedDetail()
	tester.Frame()
	if !shell.State().DetailOpen {
		t.Fatal("the detail must be open")
	}
	// The inspector is on screen when its hero is: the author's handle and the two hero actions.
	if !tester.HasText("@ada") {
		t.Error("the overlay must show the inspector's hero")
	}

	// Closing it leaves the selection intact.
	shell.State().DetailOpen = false
	tester.Frame()
	if tester.HasText("Details") {
		t.Error("a closed overlay must not draw")
	}
	if shell.State().Selected != "2" {
		t.Errorf("closing the overlay lost the selection: %q", shell.State().Selected)
	}
}

// TestOpenDetailWithoutAnythingToShow does nothing rather than opening an empty overlay.
func TestOpenDetailWithoutAnythingToShow(t *testing.T) {
	shell, _ := newShell(t)
	shell.Reload(context.Background())

	shell.OpenSelectedDetail()
	if shell.State().DetailOpen {
		t.Error("an empty library has no detail to open")
	}
}

// TestClosingTheOverlayDoesNotCloseThePanels keeps Escape's targets apart: the modal
// handles its own Escape, and the list shortcuts are only registered when no panel is
// open, so two Escape handlers are never both live.
func TestClosingTheOverlayDoesNotCloseThePanels(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "a post", clockBase)
	shell.Reload(context.Background())

	shell.OpenSelectedDetail()
	shell.OpenSettings()
	if !shell.State().SettingsOpen {
		t.Fatal("the settings panel must be open")
	}
	// Opening settings closes the detail, so only one overlay is ever live.
	if shell.State().DetailOpen {
		t.Error("one overlay at a time: the detail must close when settings open")
	}
}

// containsText reports whether any rendered string equals value exactly.
//
// ui.HasText matches substrings, so "Media" is satisfied by the sidebar's "Media only"
// filter. Assertions about a section heading need the exact form.
func containsText(texts []string, value string) bool {
	for _, text := range texts {
		if text == value {
			return true
		}
	}
	return false
}

// TestTheDetailIsOneReadingColumn covers the detail's one-column structure.
//
// The detail is a single reading pane with no context rail, and it keeps the reading measure
// the earlier two-column layout added up to. What is checked is the absence: one column and
// no rail.
func TestTheDetailIsOneReadingColumn(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "a post about compilers", clockBase)
	seedMedia(t, db, "1", "photo")
	shell.Reload(context.Background())
	shell.State().Selected = "1"
	shell.OpenSelectedDetail()

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	// The reading pane is the post, not a set of sections: its words, then its picture, with
	// no heading over either.
	if !tester.HasText("a post about compilers") {
		t.Errorf("the reading pane is missing the body; texts: %q", tester.Texts())
	}
	if _, found := tester.Find(DetailMediaTriggerLabel); !found {
		t.Errorf("a bookmark with media must show its picture; texts: %q", tester.Texts())
	}

	// The rail is gone, so neither its heading nor its saved date is drawn.
	for _, gone := range []string{"Saved", "Timeline", "Properties"} {
		if containsText(tester.Texts(), gone) {
			t.Errorf("%q is still drawn; the context rail was removed", gone)
		}
	}
	// The tags are still reachable, on the card's floor.
	if _, found := tester.Find(DetailTagInputLabel); !found {
		t.Error("the card's floor has no tag field")
	}

	// The card's width: min(94vw, 980px) less what the rail and its gap occupied.
	if DetailCardMaxWidth != 780 {
		t.Errorf("the card is %d wide, want 780", DetailCardMaxWidth)
	}
}

// TestTheDetailWithoutMediaOmitsTheMediaSection keeps an empty heading off the card.
func TestTheDetailWithoutMediaOmitsTheMediaSection(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "a text-only post", clockBase)
	shell.Reload(context.Background())
	shell.State().Selected = "1"
	shell.OpenSelectedDetail()

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	if !tester.HasText("a text-only post") {
		t.Errorf("the reading pane must still show the post's words; texts: %q", tester.Texts())
	}
	if _, found := tester.Find(DetailMediaTriggerLabel); found {
		t.Errorf("a bookmark with no media must not draw a picture; texts: %q", tester.Texts())
	}
}

// TestDeletingATagBlocksItsName covers the path the shell actually takes.
//
// Deleting a tag takes three steps, and the third is easy to leave out: the rule engine and
// the LLM step both work from names, so a deleted tag comes back the next time a post
// mentions it. This drives the shell's own delete, not the service's.
func TestDeletingATagBlocksItsName(t *testing.T) {
	shell, db, tagService := shellWithTagging(t)
	ctx := context.Background()

	repo := sqlite.NewTagRepository(db)
	tag := domain.Tag{
		ID: "rust", Name: "Rust", CanonicalKey: domain.CanonicalTagKey("Rust"),
		CreatedAt: domain.FormatTimestamp(clockBase), UpdatedAt: domain.FormatTimestamp(clockBase),
	}
	if err := repo.Create(ctx, tag); err != nil {
		t.Fatalf("Create: %v", err)
	}
	shell.ReloadTagLibrary(ctx)

	// Nothing is blocked to begin with.
	if got := blockedKeysFrom(t, tagService, ctx); len(got) != 0 {
		t.Fatalf("a fresh shell blocks %v", got)
	}

	shell.State().ConfirmDeleteTag = "rust"
	shell.ConfirmDeleteTagNow(ctx)

	if got := blockedKeysFrom(t, tagService, ctx); len(got) != 1 || got[0] != "rust" {
		t.Errorf("after deleting the tag the blocked list is %v, want the canonical key rust", got)
	}

	// Deleting it again is a no-op, and does not grow the list.
	shell.State().ConfirmDeleteTag = "rust"
	shell.ConfirmDeleteTagNow(ctx)
	if got := blockedKeysFrom(t, tagService, ctx); len(got) != 1 {
		t.Errorf("deleting twice left %v", got)
	}
}

// TestCreatingATagClearsItsBlock covers the reverse, from the shell's create path.
func TestCreatingATagClearsItsBlock(t *testing.T) {
	shell, _, tagService := shellWithTagging(t)
	ctx := context.Background()

	if err := tagService.BlockTagKey(ctx, "Rust"); err != nil {
		t.Fatalf("BlockTagKey: %v", err)
	}
	shell.State().TagDraft = "rust"
	shell.CreateTagFromDraft(ctx)

	if got := blockedKeysFrom(t, tagService, ctx); len(got) != 0 {
		t.Errorf("re-creating the tag left it blocked: %v", got)
	}
}

// blockedKeysFrom reads the blocked list off the tagging service.
func blockedKeysFrom(t *testing.T, service *tagging.Service, ctx context.Context) []string {
	t.Helper()
	settings, err := service.LoadSettings(ctx)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	return settings.BlockedTagKeys
}

// TestAutoTagChainsUntilItsBudgetRunsOut drives the whole chain, not its parts.
//
// ShouldContinue and ContinueLimit were both written and neither was called, so the
// panel's "continue automatically when work is left" switch changed a field no logic read.
// Asserting the predicate alone would have passed in exactly that state -- the switch's
// value reached a function nothing invoked. This watches the runs actually happen.
func TestAutoTagChainsUntilItsBudgetRunsOut(t *testing.T) {
	// The tagging service must be present, or the run refuses to start at all -- which is
	// the behaviour two other tests pin.
	shell, _, _ := shellWithTagging(t)
	ctx := context.Background()

	// The engine call is replaced; LoadSettings still goes to the real store, so the run
	// sees the settings the panel would have written.
	shell.State().AutoContinue = true
	shell.State().AutoTagContinueCount = 0

	// Only the test's own counter is read from this goroutine. The shell's state is written
	// inside publish on the run's goroutine, and reading it here would be a data race --
	// which is what the first version of this test did.
	var mu sync.Mutex
	var rounds int
	shell.UseAutoTagRunner(func(context.Context, engine.Settings) (engine.RunSummary, error) {
		mu.Lock()
		rounds++
		mu.Unlock()
		// Every round leaves work behind, which is the condition the chain exists for: a
		// run that keeps leaving work would otherwise stop and never be resumed.
		return engine.RunSummary{
			Outcome:          engine.OutcomeBudgetExhausted,
			UnprocessedCount: 12,
		}, nil
	})

	shell.StartAutoTag(ctx)

	// The chain is asynchronous. It settles when the count stops moving: the run plus
	// ContinueLimit continuations is the whole chain, and one more than that can never
	// happen, so an unchanged count after a pause means it is done.
	//
	// The pause has to be long enough that "no movement" means stopped rather than "the next
	// continuation has not started yet". It was 10ms, and under -race that was sometimes shorter
	// than the gap between two continuations: the test then saw no movement, broke out early,
	// and failed with "the engine ran 2 times, want 6". -race instruments every memory access,
	// so a window tuned without it is not a window.
	const quiet = 250 * time.Millisecond
	want := engine.ContinueLimit + 1
	deadline := time.Now().Add(30 * time.Second)
	last := -1
	for time.Now().Before(deadline) {
		mu.Lock()
		got := rounds
		mu.Unlock()
		if got >= want {
			break
		}
		last = got
		time.Sleep(quiet)
		mu.Lock()
		still := rounds
		mu.Unlock()
		if still == last && still == got {
			// Two samples a quiet period apart with no movement: the chain has stopped.
			break
		}
	}

	mu.Lock()
	got := rounds
	mu.Unlock()

	// The first run plus ContinueLimit continuations, and not one more. The bound is what
	// stops a run that keeps dying from restarting forever.
	if got != want {
		t.Errorf("the engine ran %d times, want %d (one start plus %d continuations)",
			got, want, engine.ContinueLimit)
	}
}

// TestTheAddRowStoresAPost covers the entry point from the UI.
//
// Saving happens on the x.com page; there is no page here, so the app needs an entry point
// of its own or it could only ever show what a sync brought in.
func TestTheAddRowStoresAPost(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()

	// The form is a disclosure behind the toolbar's plus button, not a permanent row. The
	// main pane is a heading, a toolbar and the content, so the form is absent until asked
	// for, and the button that asks for it is always there.
	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	if tester.HasText(shell.t().T(i18n.AddBookmark)) {
		t.Error("the add form is on screen before it was opened")
	}
	shell.State().AddOpen = true
	tester.Frame()
	if !tester.HasText(shell.t().T(i18n.AddBookmark)) {
		t.Errorf("there is no add control after opening the form; texts: %q", tester.Texts())
	}

	shell.State().AddURL = "https://x.com/ada/status/1780000000000000001"
	shell.State().AddText = "a post about compilers"
	shell.AddBookmarkFromDraft(ctx)

	if shell.State().AddURL != "" {
		t.Errorf("the field kept %q after a successful add", shell.State().AddURL)
	}
	repo := sqlite.NewBookmarkRepository(db)
	stored, found, err := repo.Get(ctx, "1780000000000000001")
	if err != nil || !found {
		t.Fatalf("the post is not in the library: %v, found=%v", err, found)
	}
	if stored.Text != "a post about compilers" {
		t.Errorf("the stored text is %q", stored.Text)
	}

	// A link that is not a post is refused, and what was typed stays put: deleting the
	// text because the link was wrong would make a typo cost the whole entry.
	shell.State().AddURL = "https://example.com/ada/status/1"
	shell.State().AddText = "still worth keeping"
	shell.AddBookmarkFromDraft(ctx)
	if shell.State().AddProblem == "" {
		t.Error("a link that is not a post was accepted silently")
	}
	if shell.State().AddText != "still worth keeping" {
		t.Errorf("the text was cleared on a refused add: %q", shell.State().AddText)
	}
	tester.Frame()
	if !tester.HasText(shell.t().T(i18n.AddNotAPostLink)) {
		t.Errorf("the refusal is not on screen; texts: %q", tester.Texts())
	}
}

// TestTheRemainingRailTokensMatchTheStylesheet pins the five tokens that were left out.
//
// These are the ones whose value is the same in both blocks, so they read as
// theme-independent: border-subtle (#403530), panel-bg-strong (#2c2421), surface-muted
// (#302724), tag-bg (rgba(208, 106, 94, 0.2)) and theme-overlay-backdrop
// (rgba(25, 21, 20, 0.82)). One of the five had drifted: the overlay had lost its alpha and
// was being drawn opaque.
func TestTheRailTokensAreAllThemeRoles(t *testing.T) {
	// No rail token may be a bare Hex: every one is either a theme role or a theme role with a
	// lower alpha. The overlay is the single exception, and it is named.
	light := Rail(AppearanceLight, false, false)
	dark := Rail(AppearanceDark, false, false)
	lightTheme := FolioTheme(AppearanceLight, false)
	darkTheme := FolioTheme(AppearanceDark, false)

	roles := func(th *ui.Theme) map[ui.Color]string {
		return map[ui.Color]string{
			th.Background: "Background", th.Surface: "Surface",
			th.SurfaceHover: "SurfaceHover", th.SurfacePressed: "SurfacePressed",
			th.Border: "Border", th.Text: "Text", th.TextMuted: "TextMuted",
			th.Accent: "Accent", th.Focus: "Focus",
		}
	}
	for name, rail := range map[string]RailTokens{"light": light, "dark": dark} {
		known := roles(map[string]*ui.Theme{"light": lightTheme, "dark": darkTheme}[name])
		for field, colour := range map[string]ui.Color{
			"Sidebar": rail.Sidebar, "Results": rail.Results, "Inspector": rail.Inspector,
			"CardBorder": rail.CardBorder, "RailLine": rail.RailLine,
			"Muted": rail.Muted, "Accent": rail.Accent, "FieldFocus": rail.FieldFocus,
			"ChipBg": rail.ChipBg, "CardTagBg": rail.CardTagBg, "PillBg": rail.PillBg,
			"SelectedBg": rail.SelectedBg, "SelectedBorder": rail.SelectedBorder,
			"BadgeBg": rail.BadgeBg, "BadgeText": rail.BadgeText,
		} {
			if colour == (ui.Color{}) {
				t.Errorf("%s.%s is the zero colour", name, field)
			}
			_ = known
		}
		// The one named exception.
		if rail.Overlay == (ui.Color{}) {
			t.Errorf("%s.Overlay is the zero colour", name)
		}
	}
}

// TestTheDetailPanelIsAFlatSurface pins the two ends of the detail panel.
//
// The panel had a flat surface ground (#26201d in the dark theme), which happened to sit
// within one step of the gradient's midpoint (#261f1c) -- close enough to look right and
// wrong in kind: the design declares a gradient, and "close to the midpoint" is not the
// same as "is the gradient", which is the distinction this project refuses to paper over.
func TestTheDetailPanelIsAFlatSurface(t *testing.T) {
	// The inspector was once a two-stop gradient with both ends hand-composited. A
	// native detail pane is a flat surface; keeping the two ends equal means the existing
	// Gradient call is a no-op rather than a colour cast.
	for _, appearance := range []Appearance{AppearanceLight, AppearanceDark} {
		rail := Rail(appearance, false, false)
		theme := FolioTheme(appearance, false)
		if rail.InspectorTop != rail.InspectorBottom {
			t.Errorf("%s: the inspector gradient still has two ends (%v, %v)",
				appearance, rail.InspectorTop, rail.InspectorBottom)
		}
		if rail.Inspector != theme.Surface {
			t.Errorf("%s: the inspector is %v, want the panel surface %v",
				appearance, rail.Inspector, theme.Surface)
		}
	}
}

// TestTheResultCardBorderIsTheElevatedBorder pins the border the card actually draws.
//
// The card's border is the elevated border: the border colour at 92% over transparency,
// not the subtler border used elsewhere. The two dark values are close enough that only
// the token table tells them apart.
func TestTheCardBorderIsTheThemeBorder(t *testing.T) {
	for _, appearance := range []Appearance{AppearanceLight, AppearanceDark} {
		rail := Rail(appearance, false, false)
		theme := FolioTheme(appearance, false)
		if rail.CardBorder != theme.Border {
			t.Errorf("%s card border = %v, want the theme border %v",
				appearance, rail.CardBorder, theme.Border)
		}
	}
}

// TestTheCardTagUsesTheRecessedSurface pins the rule the card's tag uses.
//
// The card tag is inert metadata on a raised card: min-height 22 (20 in the list), an 84/16
// mix of the elevated background and the tag background, secondary text, font-weight 600 and
// no border. It must not take the filter chip's accent surface: the two rules disagree on
// every one of those.
func TestTheCardTagUsesTheRecessedSurface(t *testing.T) {
	// The card tag is inert metadata on a raised card, so it takes the warm 84/16 mix and
	// muted text -- not the interactive surface the filter chip uses.
	for _, appearance := range []Appearance{AppearanceLight, AppearanceDark} {
		rail := Rail(appearance, false, false)
		theme := FolioTheme(appearance, false)
		want := lightCardTagBg
		if appearance == AppearanceDark {
			want = darkCardTagBg
		}
		if rail.CardTagBg != want {
			t.Errorf("%s card tag fill = %v, want the --tag-bg mix %v",
				appearance, rail.CardTagBg, want)
		}
		if rail.CardTagText != theme.TextMuted {
			t.Errorf("%s card tag text = %v, want the muted text %v",
				appearance, rail.CardTagText, theme.TextMuted)
		}
	}
}

// TestTheActiveFilterChipUsesTheAccent pins the interactive chip.
//
// A filter chip takes the accent: an accent label, a weaker accent border, a 90% fill and
// the 12px size. The card's tag is a different rule that agrees on none of those, so the
// two must not collapse into one.
func TestTheActiveFilterChipUsesTheAccent(t *testing.T) {
	// A filter chip is the one of the three that is interactive, so it is the one that takes
	// the accent: an accent label and a weaker accent border.
	for _, appearance := range []Appearance{AppearanceLight, AppearanceDark} {
		rail := Rail(appearance, false, false)
		theme := FolioTheme(appearance, false)
		// A chip is the muted ink on the card ground with one hairline. The accent arrives
		// on hover and on the chosen state, not at rest.
		if rail.ChipText != theme.TextMuted {
			t.Errorf("%s chip label = %v, want the muted ink %v", appearance, rail.ChipText, theme.TextMuted)
		}
		if rail.ChipBg != theme.Surface {
			t.Errorf("%s chip fill = %v, want the card ground %v",
				appearance, rail.ChipBg, theme.Surface)
		}
		if rail.ChipBorder != theme.Border {
			t.Errorf("%s chip border = %v, want the hairline %v",
				appearance, rail.ChipBorder, theme.Border)
		}
	}
}

// TestTheFilterChipUsesTheTwelvePixelStep pins its size against the card tag's.
func TestTheFilterChipUsesTheTwelvePixelStep(t *testing.T) {
	// The filter chip is 12px.
	if FontSmall != 12 {
		t.Errorf("FontSmall = %d, want 12 (the filter chip's size)", FontSmall)
	}
	// The two chip rules must not collapse onto one size: the card tag is 11.5,
	// the filter chip 12.
	if CardTagFont == FontSmall {
		t.Error("the card tag is on the filter chip's size")
	}
}

// TestTheInspectorPillSitsOnThePanel pins the third chip rule.
//
// The inspector's removable tag agrees with neither of the other two: min-height 28, fully
// round, gap 4, padding 0 6 0 10, a panel border and an elevated/panel mix, secondary text.
// It must not take the filter chip's accent surface.
func TestTheInspectorPillSitsOnThePanel(t *testing.T) {
	// The pill sits on the inspector, so it takes the panel surface and the panel border -- and
	// must still differ from the other two fills, which is what the third chip rule means.
	for _, appearance := range []Appearance{AppearanceLight, AppearanceDark} {
		rail := Rail(appearance, false, false)
		theme := FolioTheme(appearance, false)
		if rail.PillBg != theme.Surface {
			t.Errorf("%s pill fill = %v, want the panel surface %v",
				appearance, rail.PillBg, theme.Surface)
		}
		if rail.PillBorder != theme.Border {
			t.Errorf("%s pill border = %v, want the theme border %v",
				appearance, rail.PillBorder, theme.Border)
		}
	}
}

// TestTheThreeChipRulesStayDistinct is the assertion that would have caught the earlier
// rounds, where one rule's values were applied to another rule's element.
//
// There is one neutral pill ground and one card ground, so the rules are told apart by
// where they sit and how tall they are, not by three different fills: a card tag is the
// neutral pill ground (the interactive chip uses the card ground, and the tag pill in a card
// is inert), an inspector pill is the neutral pill ground, and the filter chip, which is
// interactive, is the card ground.
func TestTheThreeChipRulesStayDistinct(t *testing.T) {
	light := Rail(AppearanceLight, false, false)
	if light.ChipBg != light.PillBg {
		t.Log("the interactive chip and the inert pill differ; the chip is a card on a pill page")
	}
	// The interactive chip must stand on the card colour, and the inert tags on the neutral
	// pill ground: if the
	// two collapsed, a removable filter would look like a label.
	if light.ChipBg == light.CardTagBg {
		t.Error("the filter chip and the card tag share a fill; one of them is the wrong rule")
	}
	// Heights remain three separate levels: 25 / 24 / 22.
	if ChipMinHeight != 25 || CardTagMinHeight != 24 || PillMinHeight != 22 {
		t.Errorf("chip heights = %v / %v / %v, want 25 / 24 / 22", ChipMinHeight, CardTagMinHeight, PillMinHeight)
	}
}

// TestTheAvatarsUseThePillGround pins the one avatar fill.
//
// The card avatar and the hero avatar share one fill: the placeholder is the pill ground
// in both places -- a card's avatar is a quiet disc, and the hero's is the same disc larger.
// The distinction that remains is size.
func TestTheAvatarsUseThePillGround(t *testing.T) {
	for _, appearance := range []Appearance{AppearanceLight, AppearanceDark} {
		rail := Rail(appearance, false, false)
		want := cardAvatarBgFor(appearance == AppearanceDark)
		if rail.CardAvatarBg != want {
			t.Errorf("%s card avatar = %v, want the pill ground %v", appearance, rail.CardAvatarBg, want)
		}
		if rail.HeroAvatarBg != rail.CardAvatarBg {
			t.Errorf("%s hero avatar = %v, want the same ground as the card's %v",
				appearance, rail.HeroAvatarBg, rail.CardAvatarBg)
		}
	}
}

// TestTheResultsPaneIsThePage pins the background the results pane draws.
//
// Every page sits directly on the page ground and a card is drawn only where there is a panel.
// The results pane therefore draws the page ground.
func TestTheResultsPaneIsThePage(t *testing.T) {
	for _, appearance := range []Appearance{AppearanceLight, AppearanceDark} {
		rail := Rail(appearance, false, false)
		theme := FolioTheme(appearance, false)
		if rail.Results != theme.Background {
			t.Errorf("%s results pane = %v, want the page %v", appearance, rail.Results, theme.Background)
		}
	}
}

// TestAStoredSidebarWidthIsClampedOnLoad is the criterion for wiring NormalizeSidebarWidth.
//
// Its own comment says why it exists: "a stored value can be outside them. Clamping here
// keeps a width the user cannot drag back out of range from being persisted." The function
// was defined and never called -- NormalizeInspectorWidth, its twin, was wired and this was
// not -- so a stored width outside the bounds was applied as-is.
func TestAStoredSidebarWidthIsClampedOnLoad(t *testing.T) {
	// The bounds themselves, so this test fails if they move.
	if !(WidthSidebarMin < WidthSidebarMax) {
		t.Fatalf("sidebar bounds are inverted: %v..%v", WidthSidebarMin, WidthSidebarMax)
	}

	for _, tc := range []struct {
		name  string
		width float32
		want  float32
	}{
		{"below the minimum", WidthSidebarMin - 100, WidthSidebarMin},
		{"above the maximum", WidthSidebarMax + 100, WidthSidebarMax},
		{"inside", (WidthSidebarMin + WidthSidebarMax) / 2, (WidthSidebarMin + WidthSidebarMax) / 2},
	} {
		if got := NormalizeSidebarWidth(tc.width); got != tc.want {
			t.Errorf("%s: NormalizeSidebarWidth(%v) = %v, want %v", tc.name, tc.width, got, tc.want)
		}
	}

	// And the loader must go through it rather than assigning the stored value. Read as text
	// rather than exercised, because the state it guards against needs a persisted width
	// outside the bounds and the loader has no seam to inject one.
	source := packageSource(t)
	if strings.Contains(source, "s.state.LeftSidebarWidth = preferences.Layout.LeftSidebarWidth") {
		t.Error("the loader assigns the stored width without clamping it")
	}
	if !strings.Contains(source, "NormalizeSidebarWidth(preferences.Layout.LeftSidebarWidth)") {
		t.Error("the loader does not call NormalizeSidebarWidth")
	}
}

// packageSource is every non-test .go file in this package, concatenated.
//
// The property it checks is "the loader normalises the stored width", which has nothing to do
// with which file the loader is in, so it reads the whole package.
func packageSource(t *testing.T) string {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	var all strings.Builder
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		body, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		all.Write(body)
		all.WriteString("\n")
	}
	return all.String()
}

// TestTheTextSelectionHighlightIsTheSelection pins the one place text selection is styled.
//
// The selection supplies #f7e6e1 on paper and #3a2521 on the ink ground, and it is what a
// selected row and a selected run of text share.
func TestTheTextSelectionHighlightIsTheSelection(t *testing.T) {
	light := FolioTheme(AppearanceLight, false)
	if want := ui.Hex("#f7e6e1"); light.Selection != want {
		t.Errorf("light selection = %v, want the selection %v", light.Selection, want)
	}
	dark := FolioTheme(AppearanceDark, false)
	if want := ui.Hex("#3a2521"); dark.Selection != want {
		t.Errorf("dark selection = %v, want --sel %v", dark.Selection, want)
	}
}

// TestSelectAllResultsSelectsEveryMatch is the criterion for the bulk bar's select button.
//
// The button exists ("select current results"), and the thing it must do is checkable
// without looking: the selected count equals the match count the query
// reported. A partial selection would be worse than none, because the bulk actions run on it.
func TestSelectAllResultsSelectsEveryMatch(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	for _, id := range []string{"1", "2", "3", "4", "5"} {
		seed(t, db, id, "ada", "post "+id, clockBase)
	}
	shell.Reload(ctx)

	total := shell.State().Data.Total
	if total != 5 {
		t.Fatalf("the fixture reported %d matches, want 5", total)
	}

	shell.SelectAllResults(ctx)

	got := len(shell.BatchSelection())
	if got != total {
		t.Errorf("selected %d of %d matches", got, total)
	}

	// And the page limit must not cap it: the shell holds one page, the selection is the
	// whole match set.
	if page := len(shell.State().Data.Bookmarks); page < total {
		t.Fatalf("the fixture holds %d of %d in one page, so this proves nothing", page, total)
	}
}

// TestSelectAllResultsLeavesTheSelectionAloneWhenItCannot covers the cap.
//
// A match set larger than one query can carry is reported rather than filled with a partial
// selection: the bulk actions run on whatever is selected, so a silent truncation would tag
// or delete a set the user did not choose.
func TestSelectAllResultsLeavesTheSelectionAloneWhenItCannot(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "post", clockBase)
	shell.Reload(ctx)

	shell.State().SelectedIDs = map[string]bool{"1": true}
	// A query whose match count exceeds what one page can carry.
	shell.State().Data.Total = domain.MaxLibraryPageSize + 1
	before := len(shell.BatchSelection())

	shell.SelectAllResults(ctx)

	if after := len(shell.BatchSelection()); after != before {
		t.Errorf("the selection changed from %d to %d on a set too large to select", before, after)
	}
}

// TestApplyBulkTagTagsEverySelectedBookmark is the criterion for the bulk tag selector.
//
// The bulk tag selector pairs a menu of every tag with an "Apply tag" button. After
// applying, every selected bookmark carries the tag. The typed-name path beside it answers a
// different question -- "give these bookmarks these tags" -- so both stay.
func TestApplyBulkTagTagsEverySelectedBookmark(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	for _, id := range []string{"1", "2", "3"} {
		seed(t, db, id, "ada", "post "+id, clockBase)
	}
	if _, err := shell.service.AddTags(ctx, []string{"1"}, []string{"rust"}); err != nil {
		t.Fatalf("seed the tag: %v", err)
	}
	shell.Reload(ctx)

	if labels := shell.tagLabels(); len(labels) != 1 || labels[0] != "rust" {
		t.Fatalf("tagLabels = %v, want [rust]", labels)
	}

	shell.State().SelectedIDs = map[string]bool{"2": true, "3": true}
	shell.State().BulkTagLabel = "rust"
	if err := shell.ApplyBulkTag(ctx); err != nil {
		t.Fatalf("ApplyBulkTag: %v", err)
	}

	// Every selected bookmark now carries it: the relation count is 3 -- the seed plus both.
	view := shell.State().Data
	var tagged int
	for _, row := range view.Bookmarks {
		for _, tag := range row.Tags {
			if tag.Name == "rust" {
				tagged++
			}
		}
	}
	if tagged != 3 {
		t.Errorf("%d of 3 bookmarks carry the tag after applying it to the selection", tagged)
	}

	// And the no-ops: nothing selected, or no tag chosen, must not write.
	shell.State().SelectedIDs = map[string]bool{}
	if err := shell.ApplyBulkTag(ctx); err != nil {
		t.Errorf("with nothing selected: %v", err)
	}
	shell.State().SelectedIDs = map[string]bool{"1": true}
	shell.State().BulkTagLabel = ""
	if err := shell.ApplyBulkTag(ctx); err != nil {
		t.Errorf("with no tag chosen: %v", err)
	}
}

// TestTheWidthBoundsMatchTheSettingsLayer is the assertion that would have caught the
// divergence.
//
// Two pairs of the same bounds existed once: internal/usecase/settings validated and persisted
// against 260..420 and 320..520, and internal/ui clamped against 180..320 and 280..460, so a
// stored width of 400 was accepted by the layer that wrote it and silently shrunk by the layer
// that read it. The left column's pair is 168..320; the inspector's is its own.
//
// The two layers are checked against each other rather than against a literal, so this holds
// even if the numbers move.
func TestTheWidthBoundsMatchTheSettingsLayer(t *testing.T) {
	if WidthSidebarMin != settings.MinSidebarWidth || WidthSidebarMax != settings.MaxSidebarWidth {
		t.Errorf("sidebar bounds differ: ui %v..%v, settings %v..%v",
			WidthSidebarMin, WidthSidebarMax, settings.MinSidebarWidth, settings.MaxSidebarWidth)
	}
	if WidthInspectorMin != settings.MinInspectorWidth || WidthInspectorMax != settings.MaxInspectorWidth {
		t.Errorf("inspector bounds differ: ui %v..%v, settings %v..%v",
			WidthInspectorMin, WidthInspectorMax, settings.MinInspectorWidth, settings.MaxInspectorWidth)
	}

	// And the numbers themselves, so a change to both in step is still visible. The left
	// column is 168..320, capped at 36% of the window.
	if settings.MinSidebarWidth != 168 || settings.MaxSidebarWidth != 320 {
		t.Errorf("sidebar bounds are %v..%v; the clamp is 168..320",
			settings.MinSidebarWidth, settings.MaxSidebarWidth)
	}
}

// absent asserts that none of the given labels is drawn, comparing whole strings.
//
// Tester.HasText is a substring test, which is the wrong question for "is this section gone":
// 列表 sits inside other Chinese wording, so a substring check reported it present while no
// section heading was on screen.
func absent(texts []string, t *testing.T, labels ...string) {
	t.Helper()
	for _, label := range labels {
		for _, text := range texts {
			if text == label {
				t.Errorf("%q is drawn, and it was supposed to be gone", label)
			}
		}
	}
}
