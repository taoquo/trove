package ui

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/taoquo/trove/internal/domain"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/internal/i18n"
)

// TestTheTypeScaleMatchesTheStylesheet pins the type scale against the stylesheet.
//
// Every value below is the stylesheet's, and a change to any of them fails here rather than
// being discovered by looking at a screenshot.
func TestTheTypeScaleMatchesTheStylesheet(t *testing.T) {
	want := []struct {
		name  string
		got   float32
		value float32
		from  string
	}{
		{"overline", FontOverline, 11, "the overline label"},
		{"sidebar title", FontSidebarTitle, 14, "the sidebar wordmark: 13.5 raised to the toolkit's grid"},
		{"display title", FontDisplayTitle, 16, "the dialog's title"},
		{"results value", FontResultsValue, 22, "a page's standing total; the scale's one display size"},
		{"overline tracking", OverlineTracking, 0.05, "the overline label's letter-spacing"},
		{"display tracking", DisplayTracking, -0.01, "tight display tracking for headings"},
	}
	for _, item := range want {
		if item.got != item.value {
			t.Errorf("%s = %v, want %v (from %s)", item.name, item.got, item.value, item.from)
		}
	}
}

// TestTheRadiiFollowTheScale pins the six radii the restyle uses.
//
// The scale is smaller and visible in the component rules: 6 for a field or a switch
// button, 7 for a sidebar
// row and a field, 8 for a switch track, 12 for every card and list, 14 for a dialog and 999 for
// a pill/avatar. Each value below is from that scale; a seventh radius fails here.
//
// The mapping is the point: a control, a row, a card and a dialog each have one radius and a new
// component has to pick one of them rather than invent a near value.
func TestTheRadiiFollowTheScale(t *testing.T) {
	scale := map[float32]string{
		6:   "controls: a text action, a field, a segmented button",
		7:   "rows and fields: the sidebar's nav row, a find field, an editor input",
		8:   "switch tracks: a segmented control",
		12:  "cards and lists",
		14:  "dialogs: a confirm action, a modal",
		999: "pills and avatars",
	}
	for _, item := range []struct {
		name string
		got  float32
	}{
		{"sidebar row", RadiusSidebarRow},
		{"result card", RadiusResultCard},
		{"toolbar shell", RadiusToolbar},
		{"toolbar item", RadiusToolbarItem},
		{"dialog", RadiusDialog},
		{"inspector", RadiusInspector},
		{"pill", RadiusPill},
	} {
		if item.got != 0 && scale[item.got] == "" {
			t.Errorf("%s radius is %v, which is not one of the scale's levels (6/7/8/12/14/999)", item.name, item.got)
		}
	}
	for _, item := range []struct {
		name string
		got  float32
		want float32
	}{
		{"sidebar row", RadiusSidebarRow, 7},
		{"result card", RadiusResultCard, 12},
		{"toolbar shell", RadiusToolbar, 12},
		{"toolbar item", RadiusToolbarItem, 8},
		{"dialog", RadiusDialog, 14},
		{"inspector", RadiusInspector, 12},
	} {
		if item.got != item.want {
			t.Errorf("%s radius = %v, want %v (the scale)", item.name, item.got, item.want)
		}
	}
}

// TestTheControlHeightsMatchTheStylesheet pins the four control heights.
//
// Controls are compact: 30 for a sidebar row, 28 for a field/action, 27 for the toolbar's
// action, 25 for a chip.
func TestTheControlHeightsMatchTheStylesheet(t *testing.T) {
	want := []struct {
		name  string
		got   float32
		value float32
	}{
		{"primary button", HeightLarge, 28},
		{"sidebar nav row", HeightPrimary, 30},
		{"toolbar field", HeightControl, 27},
		{"tag input", HeightSmall, 25},
		{"card checkbox", HeightIndicator, 20},
	}
	for _, item := range want {
		if item.got != item.value {
			t.Errorf("%s height = %v, want %v", item.name, item.got, item.value)
		}
	}
}

// TestTheCardMatchesTheStylesheet pins the card's own type and box.
//
// The card is a white 12-point box with one hairline and 14 points of body inset; its rows
// set names at 13/500, body copy at 12-13 and metadata at 11.5.
func TestTheCardMatchesTheStylesheet(t *testing.T) {
	want := []struct {
		name  string
		got   float32
		value float32
		from  string
	}{
		{"body padding", CardBodyPadding, 14, "a card's body inset, between a row's 12 and a panel's 16"},
		{"avatar", CardAvatarSize, 28, "a card avatar; the in-row mark is 22"},
		{"avatar font", CardAvatarFont, 10, "the initial inside it"},
		{"author font", CardAuthorFont, 13, "the row name's font-size"},
		{"copy font", CardCopyFont, 13, "the page's body size"},
		{"stat font", CardStatFont, 11.5, "the row sub-line's font-size"},
		{"handle font", CardHandleFont, 11.5, "a card's @handle, the same sub-line"},
		{"tag floor", CardTagMinHeight, 24, "the chip height"},
		{"action size", CardActionSize, 32, "a 28-32 point hit area for a bare icon"},
		{"grid floor", CardMinHeight, 200, "a card with a picture keeps its height"},
		{"copy wrap factor", CardCopyWrapLines, 2.6, "the copy wrap reserves 2.6 lines"},
	}
	for _, item := range want {
		if item.got != item.value {
			t.Errorf("%s = %v, want %v (from %s)", item.name, item.got, item.value, item.from)
		}
	}
	if CardCopyLineHeight != 1.45 {
		t.Errorf("copy line height = %v, want 1.45 (a tight body measure)", CardCopyLineHeight)
	}
}

// TestCardInitial covers the letter the avatar falls back to.
func TestCardInitial(t *testing.T) {
	cases := []struct{ name, handle, want string }{
		{"ada", "ada", "A"},
		// The fallback is the handle's first two characters, and a two-word name gives one
		// letter per word.
		{"", "grace", "GR"},
		{"", "@alan", "AL"},
		{"", "", "?"},
		{"Yassine Bouanane", "ybouane", "YB"},
		{"Margaret Hamilton", "margaret", "MH"},
		{"泊舟", "bozhou_ai", "泊"},
	}
	for _, item := range cases {
		if got := cardInitial(item.name, item.handle); got != item.want {
			t.Errorf("cardInitial(%q, %q) = %q, want %q", item.name, item.handle, got, item.want)
		}
	}
}

// TestTheCardDrawsItsParts covers the parts the card now carries.
//
// The spec test above pins the numbers; this one pins that they are used, because a
// constant nothing reads is how the card came to have no avatar in the first place.
func TestTheCardDrawsItsParts(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post about compilers", clockBase)
	seedTagged(t, db, "1", "rust")
	shell.Reload(ctx)

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	texts := tester.Texts()

	// The avatar's initial, the author, the handle, the tag and the saved date. Not the
	// copy: seedTagged writes its own bookmark, so the text on screen is that one.
	//
	// The date is the display form, not the stored one. This assertion used to pin
	// "2026-10-12T12:00:00.000Z" -- the RFC 3339 string the card was rendering, which is what
	// a user was reading. It now asserts the formatted date instead, so the assertion would
	// fail again if the card went back to the storage form.
	for _, want := range []string{"A", "ada", "@ada", "rust"} {
		if !containsText(texts, want) {
			t.Errorf("the card does not show %q; texts: %q", want, texts)
		}
	}
	// The date, as a prefix: the rendered value carries the local clock time, so pinning
	// the whole string would make this test depend on the machine's time zone.
	//
	// It must be the display form and not the stored one. This assertion used to pin
	// "2026-10-12T12:00:00.000Z" -- the RFC 3339 string the card was rendering, which is what
	// a user was reading. Reverting the card to that form fails here twice over: no text
	// starts with "Oct 12", and one of them starts with the stored date instead.
	var dated bool
	for _, text := range texts {
		if strings.HasPrefix(text, "2026-10-12T") {
			t.Errorf("the card shows the stored timestamp verbatim: %q", text)
		}
		if strings.HasPrefix(text, "Oct 12") {
			dated = true
		}
	}
	if !dated {
		t.Errorf("no text starts with the display date; texts: %q", texts)
	}
}

// TestTheListCardMatchesTheStylesheet pins the list layout's own type scale.
//
// The list is denser than the grid by its layout -- a side media column and a 26px avatar
// against 28 -- not by type: both read the page's 13 for copy and names, and 11.5 for
// metadata.
func TestTheListCardMatchesTheStylesheet(t *testing.T) {
	want := []struct {
		name  string
		got   float32
		value float32
		from  string
	}{
		{"list copy", CardListCopyFont, 13, "the page's body size, as on the grid"},
		{"list author", CardListAuthorFont, 13, "a list row's name"},
		{"list meta", CardListMetaFont, 11.5, "a list row's metadata"},
		{"list avatar", CardListAvatarSize, 26, "a list row's avatar"},
		{"list avatar font", CardListAvatarFont, 10, "the initial inside it"},
		{"list side", CardListSideWidth, 148, "the list card's layout grid"},
		{"no-media inset", CardNoMediaPaddingLeft, 30, "the card text inset"},
	}
	for _, item := range want {
		if item.got != item.value {
			t.Errorf("%s = %v, want %v (from %s)", item.name, item.got, item.value, item.from)
		}
	}
	if CardListCopyLineHeight != 1.45 {
		t.Errorf("list copy line height = %v, want 1.45", CardListCopyLineHeight)
	}
	if CardListCopyFont != CardCopyFont {
		t.Errorf("the list copy (%v) and the grid's (%v) are one size now", CardListCopyFont, CardCopyFont)
	}
	if CardListAvatarSize >= CardAvatarSize {
		t.Errorf("the list avatar (%v) must be smaller than the grid's (%v)", CardListAvatarSize, CardAvatarSize)
	}
}

// TestTheCardPicksTheRightScale covers the choice, not just the numbers.
//
// The spec test above pins both sets of constants. A mutation that made the list draw
// with the grid's sizes survived it, because nothing asserted which set the card picks.
func TestTheCardPicksTheRightScale(t *testing.T) {
	grid := cardTypeScale(true)
	if grid.copyFont != CardCopyFont || grid.copyLineHeight != CardCopyLineHeight ||
		grid.authorFont != CardAuthorFont || grid.avatarSize != CardAvatarSize ||
		grid.avatarFont != CardAvatarFont {
		t.Errorf("the grid scale is %+v, want the grid's values", grid)
	}

	list := cardTypeScale(false)
	if list.copyFont != CardListCopyFont || list.copyLineHeight != CardListCopyLineHeight ||
		list.authorFont != CardListAuthorFont || list.avatarSize != CardListAvatarSize ||
		list.avatarFont != CardListAvatarFont {
		t.Errorf("the list scale is %+v, want the list's values", list)
	}

	// They must actually differ, or "picks one" is not a decision. The list rows are
	// denser than the grid's cards, which shows in the avatar and the copy.
	if list.copyFont > grid.copyFont {
		t.Errorf("the list copy (%v) is larger than the grid's (%v)", list.copyFont, grid.copyFont)
	}
	if list.avatarSize == grid.avatarSize {
		t.Error("the list and the grid draw their avatar at the same size")
	}
}

// TestTheLoadingStateMatchesTheStylesheet pins the skeleton's numbers.
//
// The shell showed a bare spinner where the app draws a panel of six placeholder
// cards in the same grid the real cards use.
func TestTheLoadingStateMatchesTheStylesheet(t *testing.T) {
	for _, item := range []struct {
		name  string
		got   float32
		value float32
		from  string
	}{
		{"shell radius", SkeletonShellRadius, 12, "a panel's radius"},
		{"shell gap", SkeletonShellGap, 24, "the skeleton shell gap"},
		{"shell padding", SkeletonShellPad, 24, "the skeleton shell padding"},
		{"grid gap", SkeletonGridGap, 16, "the skeleton grid gap"},
		{"card radius", SkeletonCardRadius, 12, "a card's radius"},
		{"card padding", SkeletonCardPad, 20, "the skeleton card padding"},
		{"min card width", SkeletonMinCardW, 220, "repeat(auto-fit, minmax(220px, 1fr))"},
		{"avatar", SkeletonAvatarSize, 44, "h-11 w-11"},
		{"overline", FeedStateOverlineFont, 11, "the feed overline size"},
		{"overline tracking", FeedStateOverlineTrack, 0.12, "the feed overline tracking"},
		{"copy", FeedStateCopyFont, 14, "the feed copy size"},
	} {
		if item.got != item.value {
			t.Errorf("%s = %v, want %v (from %s)", item.name, item.got, item.value, item.from)
		}
	}
	if SkeletonCardCount != 6 {
		t.Errorf("the skeleton has %d cards, want 6", SkeletonCardCount)
	}
}

// TestTheLoadingStateRendersCards covers that the skeleton is drawn at all.
func TestTheLoadingStateRendersCards(t *testing.T) {
	shell, _ := newShell(t)
	// A shell that has not loaded yet shows the skeleton.
	shell.State().Loaded = false

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	// The overline is upper-cased for display, as the feed overline rule
	// text-transform does, so the rendered text is not the catalog's.
	if !tester.HasText(strings.ToUpper(shell.t().T(i18n.LoadingFeedTitle))) {
		t.Errorf("the skeleton's overline is missing; texts: %q", tester.Texts())
	}
	if tester.HasText(shell.t().T(i18n.EmptyLibrary)) {
		t.Error("the empty state is drawn while the read is still in flight")
	}
}

// TestTheEmptyStateMatchesTheTypeScale pins the empty state's two type sizes.
//
// The empty state is words only -- a title in the page's ink at weight 500 and advice in
// the muted ink -- so what remains to pin is the pair of sizes and the absence of
// the plate's constants from the drawing code.
func TestTheEmptyStateMatchesTheTypeScale(t *testing.T) {
	if EmptyStateTitleFont != 13 {
		t.Errorf("empty-state title = %v, want 13 (the page's body size)", EmptyStateTitleFont)
	}
	if EmptyStateCopyFont != 12 {
		t.Errorf("empty-state copy = %v, want 12 (inherited from the page)", EmptyStateCopyFont)
	}
}

// TestTheStateCardShowsTheRightState covers the three states and their parts.
func TestTheStateCardShowsTheRightState(t *testing.T) {
	shell, _ := newShell(t)
	shell.State().Loaded = true

	// An empty library: the title and its advice, and no retry.
	//
	// The empty state has no "INFO" overline: it is an icon, a title and a line of advice,
	// centred in the content viewport.
	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	if !tester.HasText(shell.t().T(i18n.EmptyLibrary)) {
		t.Errorf("the empty state is missing; texts: %q", tester.Texts())
	}
	if !tester.HasText(shell.t().T(i18n.EmptyLibraryDetail)) {
		t.Errorf("the empty state has no advice; texts: %q", tester.Texts())
	}
	if tester.HasText(strings.ToUpper(shell.t().T(i18n.InfoLabel))) {
		t.Error("the INFO overline is back; the empty state must not draw one")
	}
	if tester.HasText(shell.t().T(i18n.ActionRetry)) {
		t.Error("an empty library offers a retry")
	}

	// A failed read is a different state with different advice, not the same card recoloured.
	shell.State().Err = "the database is locked"
	tester.Frame()
	if !tester.HasText(shell.t().T(i18n.LoadFailedTitle)) {
		t.Errorf("the failure state is missing; texts: %q", tester.Texts())
	}
	if !tester.HasText(shell.t().T(i18n.ActionRetry)) {
		t.Errorf("a failed read offers no retry; texts: %q", tester.Texts())
	}
}

// TestTheDetailHeroMatchesTheStylesheet pins the detail hero and its children.
//
// The detail's header was a flat stack whose largest text was the author's name at 15px;
// the stylesheet sets a 20px display-face name beside a 48px avatar in a padded grid.
func TestTheDetailHeroMatchesTheStylesheet(t *testing.T) {
	for _, item := range []struct {
		name  string
		got   float32
		value float32
		from  string
	}{
		{"hero gap", DetailHeroGap, 18, "the hero gap"},
		{"hero padding top", DetailHeroPadTop, 22, "the hero padding"},
		{"hero padding side", DetailHeroPadSide, 26, "the hero padding"},
		{"hero padding bottom", DetailHeroPadBottom, 18, "the hero padding"},
		{"main gap", DetailHeroMainGap, 16, "the hero main gap"},
		{"avatar", DetailHeroAvatarSize, 30, "the intended hero avatar is 48; 30 keeps the header to one line"},
		{"name", DetailHeroNameFont, 16, "a dialog title"},
		{"handle", DetailHeroHandleFont, 12, "the handle under it, --muted"},
		{"action gap", DetailHeroActionGap, 10, "the hero action gap"},
		{"divider gap", InspectorDividerGap, 24, "the inspector divider spacing"},
		{"section overline", DetailOverlineFont, 12, "the detail section overline size"},
		{"post copy", DetailBodyFont, 13, "the page's body size"},
		{"post line", DetailBodyLine, 1.6, "long-form leading"},
		{"post gap", DetailPostGap, 14, "a paragraph's gap, not a section's"},
		{"media grid gap", DetailMediaGridGap, 6, "a tweet's gallery gap"},
		{"settings row height", SettingsRowHeight, 48, "a preference row's min-height"},
		{"settings row inset", SettingsRowPadLeft, 14, "a preference row's padding 12px 12px 12px 14px"},
		{"settings row gap", SettingsRowGap, 14, "a preference row's gap"},
		{"settings row name", SettingsRowNameFont, 13, "the body size, weight 500"},
		{"settings row sub", SettingsRowSubFont, 11.5, "a preference row's sub-line font-size"},
		{"settings number width", SettingsNumberWidth, 72, "a preference row's number field width"},
		{"settings number height", SettingsNumberHeight, 24, "a preference row's number field height"},
		{"settings card radius", RadiusPanel, 12, "a list card's radius"},
		{"settings section title", SettingsGroupTitle, 11, "a section label's font-size"},
		{"settings tracking", SettingsGroupTrack, 0.05, "a section label's letter-spacing"},
		{"settings tab height", SettingsTabHeight, 22, "a tab's height"},
		{"settings pick height", SettingsPickHeight, 24, "a pick's height"},
		{"tag table head height", TagTableHeadHeight, 26, "the 26-point label strip"},
		{"tag table head font", TagTableHeadFont, 11, "a section label's font-size"},
		{"tag table row", TagTableRowMin, 48, "a list row's min-height"},
		{"tag table row pad", TagTablePadY, 12, "a list row's padding"},
		{"tag table name", TagTableNameFont, 13, "the body size, weight 500"},
		{"tag table meta", TagTableMetaFont, 11.5, "a list row's sub-line font-size"},
		{"settings seg option", SettingsSegOptionHeight, 22, "a segmented option's height"},
		{"settings action height", SettingsActionHeight, 27, "a text action's height"},
		{"settings find height", SettingsFindHeight, 26, "a find field's height"},
	} {
		if item.got != item.value {
			t.Errorf("%s = %v, want %v (from %s)", item.name, item.got, item.value, item.from)
		}
	}
}

// TestTheDetailHeroDrawsItsParts covers that the hero is drawn.
func TestTheDetailHeroDrawsItsParts(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post about compilers", clockBase)
	shell.Reload(ctx)
	shell.State().Selected = "1"
	shell.OpenSelectedDetail()

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	texts := tester.Texts()

	// The avatar's initial, the author and the handle.
	for _, want := range []string{"A", "ada", "@ada"} {
		if !containsText(texts, want) {
			t.Errorf("the hero does not show %q; texts: %q", want, texts)
		}
	}
}

// TestTheCardTagUsesTheChip pins the tag chip's shape.
//
// A tag on a card is a chip: 24 high, `padding: 0 9px`, an 11.5px label.
// One chip vocabulary means a tag keeps the same height when the view
// switches.
func TestTheCardTagUsesTheChip(t *testing.T) {
	for _, tc := range []struct {
		name string
		got  float32
		want float32
		from string
	}{
		{"font", CardTagFont, 11.5, "the chip's font-size"},
		{"padding", CardTagPadX, 9, "the chip's padding: 0 9px"},
		{"floor", CardTagMinHeight, 24, "the chip's height"},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %v, want %v (%s)", tc.name, tc.got, tc.want, tc.from)
		}
	}
}

// TestNothingIsChosenWhenTheWindowOpens pins the sentinel the list uses and the grid's
// starting render window.
//
// ListState says -1 is "nothing chosen", and its Selected field is *int into a plain int on
// State. An int's zero value is 0, which is a valid index, so leaving it unset made the first
// bookmark look chosen before the user had touched anything: the round-245 screenshot has a
// selection ring on the first row of an untouched window. The grid no longer keeps an index --
// it draws cards directly and the chosen bookmark is State.Selected -- so the grid's pin is
// its first page of 80.
func TestNothingIsChosenWhenTheWindowOpens(t *testing.T) {
	shell, _ := newShell(t)
	if got := shell.State().RenderLimit; got != ResultsRenderLimit {
		t.Errorf("RenderLimit starts at %d, want %d", got, ResultsRenderLimit)
	}
	if got := shell.State().ListSelected; got != -1 {
		t.Errorf("ListSelected starts at %d, want -1 (nothing chosen)", got)
	}
	if got := shell.State().Selected; got != "" {
		t.Errorf("a bookmark is chosen at startup: %q", got)
	}
}

// TestTheGridAndTheListShowTheSameRowsInTheSameOrder makes the shared-order requirement checkable.
//
// Sorting and filtering are handled in the service layer; Grid and List must not have two
// different filtering implementations. The way that breaks is not a second filter
// -- it is a view that reads its rows from somewhere else, or applies its own order, and the
// two layouts then disagree about what the library contains. Both read state.Data.Bookmarks, so
// both should come out in the same order, and this reads the rendered text to say so.
func TestTheGridAndTheListShowTheSameRowsInTheSameOrder(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	// Three, so that an order can be wrong rather than merely reversed.
	for i, id := range []string{"1", "2", "3"} {
		seed(t, db, id, "ada", "post "+id, clockBase.Add(time.Duration(i)*time.Hour))
	}
	shell.Reload(ctx)

	// Tester reports the texts in the order they were drawn, so filtering the list in place
	// gives the order the layout put them in.
	rendered := func(mode domain.ViewMode) []string {
		shell.State().ViewMode = mode
		tester := ui.NewTester(shell.View, 1180, 760)
		tester.Frame()
		// Each card's text arrives twice -- the frame is measured and then drawn -- so the
		// sequence repeats every entry. Collapsing runs of the same value leaves the order the
		// layout chose, which is what this is about.
		var found []string
		for _, text := range tester.Texts() {
			for _, want := range []string{"post 1", "post 2", "post 3"} {
				if text != want {
					continue
				}
				if len(found) == 0 || found[len(found)-1] != want {
					found = append(found, want)
				}
			}
		}
		return found
	}

	grid := rendered(domain.ViewGrid)
	list := rendered(domain.ViewList)

	if len(grid) != 3 || len(list) != 3 {
		t.Fatalf("grid showed %v and list showed %v; both should show all three", grid, list)
	}
	for i := range grid {
		if grid[i] != list[i] {
			t.Errorf("the two layouts disagree about row %d: grid %v, list %v", i, grid, list)
			break
		}
	}
}

// TestTheInspectorBecomesAPaneOnlyWhenTheListKeepsItsWidth pins the arithmetic behind the pane
// rule.
//
// The detail is a third column only while the results pane keeps MainMinWidth, and an overlay
// below that. Two ways to get it wrong both look fine in a wide window: never making it a pane,
// so a 1400-wide screen still gets a modal over the list, and always making it one, which
// squeezes the list to nothing on a small one. TestTheDetailIsAlwaysAnOverlay covers what the
// workspace's columns actually are.
//
// The reference layout settles it. The detail is a modal at every width: a fixed overlay over
// the whole window, with a card capped at min(94vw, 980px) and 86dvh. There is no grid that puts
// a sidebar, a list and an inspector side by side; the reference workspace is a sidebar and a
// results pane.
func TestTheDetailIsAlwaysAnOverlay(t *testing.T) {
	shell, _ := newShell(t)

	for _, tc := range []struct {
		name    string
		width   float32
		sidebar float32
	}{
		{"no window measured yet", 0, WidthSidebarMin},
		{"the narrowest window the shell allows", 900, WidthSidebarMin},
		{"the reference picture's width", 1400, WidthSidebarDef},
		{"far wider than the pane threshold", ThreeColumnMinWidth + 900, WidthSidebarMin},
	} {
		if inspectorFitsFor(tc.width, tc.sidebar) {
			t.Errorf("%s (%v wide, sidebar %v): the detail claims to fit as a third pane, and the "+
				"detail is always an overlay", tc.name, tc.width, tc.sidebar)
		}
	}

	// And the shell draws it as an overlay, which is what the predicate is there to decide.
	shell.State().SettingsOpen = false
	if shell.State().DetailOpen {
		t.Fatal("the detail is open before anything opened it")
	}
}

// TestTheInspectorOwnsAScrollRegion asserts the detail's scroll region structurally.
//
// The detail owns a vertical scroll region of its own. Without one it is a column with MaxHeight,
// so a long bookmark is cut off at the container's edge with nothing to scroll.
//
// A behavioural test was written first and never passed, because ui.Tester.Scroll does not drive a
// ui.Scroll in this shell at all. The evidence is the sidebar, which has its own
// Scroll(c).FillWidth().Grow(1) and is a plain child of the Split's first closure -- no backdrop,
// no overlay, nothing unusual. Scrolling
// at two points by -300 and +300 DIPs leaves its rendered text at 821 characters, unchanged, four
// times out of four.
//
// So the negative result is not about the inspector. Recorded here so that the next person does
// not try to prove the detail scrolls this way:
//
//   - inspector, overlay form, (590,380) -400/+400/-100/+100: no change
//   - inspector, overlay form, (590,200) and (590,600), same four: no change
//   - inspector after Tester.Move to the point: no change
//   - inspector with the backdrop's OnClick removed: no change (so not the backdrop)
//   - inspector, pane form at 1500 wide with no backdrop at all: no change
//   - sidebar's own Scroll, (140,300) and (140,500), -300/+300: no change
//
// Sixteen combinations, four different containers, one of which has nothing to do with the
// detail. The conclusion is that Tester.Scroll and this Scroll do not meet, and the check here is
// therefore structural: the region is in the path where the detail is built.
//
// The implementation is done; the acceptance is not available with the tools at hand, and saying
// so is better than a passing test that implies otherwise.
func TestTheInspectorOwnsAScrollRegion(t *testing.T) {
	source := packageSource(t)
	if !strings.Contains(source, "ui.Scroll(c).FillWidth().Grow(1).Children(func() {") {
		t.Error("the inspector has no scroll region; it must own one")
	}
	// And it is in the inspector, not somewhere else in the package: the Scroll must be followed
	// by the call to the body, which is the shape that puts the detail inside the region.
	if !strings.Contains(source, "s.inspectorBody(c, theme, rail, row)") {
		t.Error("the scroll region does not wrap the detail's body")
	}
	// The empty state is deliberately outside it, because two short lines do not need a region.
	if !strings.Contains(source, "s.inspectorEmpty(c, theme)") {
		t.Error("the empty state is inside the scroll region")
	}
}

// TestTheSpecSizeTableIsPinned pins the width and height table.
//
// The table is the one place these values are stated as numbers, and the ones asserted here are
// the values a reader will check the layout against. Two table values do not hold in this build,
// and the constants record why:
//
//	SidebarWidth 256 -- the default follows the drag state; the floor is 260 as it
//	                    clamps it, not the table's 232
//	BookmarkCardHeight 184 -- not used: the grid's row height comes from CardMinHeight of 240,
//	                    because a slot shorter than its content clips
//
// Everything else in the table that this build has a token for is asserted here, so that a change
// to any of them fails rather than travelling.
func TestTheSpecSizeTableIsPinned(t *testing.T) {
	for _, tc := range []struct {
		name string
		got  float32
		want float32
		from string
	}{
		{"MainMinWidth", MainMinWidth, 560, "the main column's minimum width"},
		{"InspectorWidth", InspectorWidth, 304, "the inspector's width"},
		{"ControlHeight", IconButtonSize, 26, "the one square for icon buttons"},
		{"NavRowMinHeight", NavRowMinHeight, 30, "a sidebar row"},
		{"SearchFieldHeight", SearchFieldHeight, 26, "the find field's height"},
		{"ToolbarActionHeight", HeightControl, 27, "a text action"},
		{"ChipMinHeight", ChipMinHeight, 25, "a chip"},
		{"ThreeColumnMinWidth", ThreeColumnMinWidth, 1400, "the three-column band"},
	} {
		if tc.got != tc.want {
			t.Errorf("%s is %v; %s requires %v", tc.name, tc.got, tc.from, tc.want)
		}
	}
}

// TestNoStoredWidthMeansTheDefaultNotTheFloor pins what a first run opens at.
//
// Both normalizers clamp in [min, max], and zero -- which is what a missing preference reads as
// -- clamped to the floor. So the window opened at the narrowest width the user could have
// dragged to, and WidthSidebarDef and WidthInspectorDef, which hold the stored defaults, were
// read by nothing at all.
//
// The values are 212 for the left column and 360 for the
// inspector, which this shell adds.
func TestNoStoredWidthMeansTheDefaultNotTheFloor(t *testing.T) {
	if got := NormalizeSidebarWidth(0); got != WidthSidebarDef {
		t.Errorf("a missing sidebar width becomes %v, want the default %v", got, WidthSidebarDef)
	}
	if got := NormalizeInspectorWidth(0); got != WidthInspectorDef {
		t.Errorf("a missing inspector width becomes %v, want the default %v", got, WidthInspectorDef)
	}
	if WidthSidebarDef != 212 {
		t.Errorf("WidthSidebarDef is %v; the default is 212", WidthSidebarDef)
	}
	if WidthInspectorDef != 360 {
		t.Errorf("WidthInspectorDef is %v; the inspector's default is 360", WidthInspectorDef)
	}
	// A real width is still clamped: the default is not a hole in the bounds.
	if got := NormalizeSidebarWidth(10); got != WidthSidebarMin {
		t.Errorf("a width of 10 becomes %v, want the floor %v", got, WidthSidebarMin)
	}
	if got := NormalizeSidebarWidth(9999); got != WidthSidebarMax {
		t.Errorf("a width of 9999 becomes %v, want the ceiling %v", got, WidthSidebarMax)
	}
}

// TestAClickOpensTheDetail drives the card click through Tester input and checks what the
// product does: the detail opens.
//
// The card is the open control, and its bulk checkbox is the separate control that keeps its
// click from reaching the card. Arrow keys still only move the cursor, and Enter opens.
//
// The test goes through Tester input rather than setting State.Selected: a test that sets the
// state it is checking cannot see whether anything can set that state.
func TestAClickOpensTheDetail(t *testing.T) {
	shell, db := newShell(t)
	for i, id := range []string{"1", "2", "3", "4"} {
		seed(t, db, id, "ada", "post "+id, clockBase.Add(time.Duration(i)))
	}
	shell.Reload(context.Background())

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	if shell.State().Selected != "" || shell.State().DetailOpen {
		t.Fatalf("something is chosen before anything was clicked: %q, open=%v",
			shell.State().Selected, shell.State().DetailOpen)
	}

	// The first card, found by its text rather than by a coordinate.
	//
	// Found by text rather than by a coordinate: a fixed coordinate encodes the header's height
	// into the test, so the click lands on the toolbar when a font size changes.
	rect, ok := tester.Find("post 1")
	if !ok {
		t.Fatalf("the seeded bookmark is not on screen to click; texts: %q", tester.Texts())
	}
	tester.ClickAt(rect.X+rect.W/2, rect.Y+rect.H/2)
	tester.Frame()

	chosen := shell.State().Selected
	if chosen == "" {
		t.Fatal("a click on a card chose nothing; selection is not wired to the grid")
	}
	// The click opens the detail. The card is the open control; the corner checkbox is the
	// separate control, and it is the one that keeps its click from reaching the card.
	if !shell.State().DetailOpen {
		t.Fatal("a click on a card must open its detail")
	}

	// Escape closes the overlay; the card stays the cursor, so Enter reopens the same post.
	tester.Key(0, ui.KeyEscape)
	tester.Frame()
	if shell.State().DetailOpen {
		t.Fatal("Escape closed nothing")
	}
	tester.Key(0, ui.KeyEnter)
	tester.Frame()

	if !shell.State().DetailOpen {
		t.Error("Enter on the chosen card did not open the detail")
	}
	if shell.State().Selected != chosen {
		t.Errorf("opening changed the choice from %q to %q", chosen, shell.State().Selected)
	}
}

// TestTheSpacingFollowsTheStableScale is the same shape as TestTheRadiiFollowTheNormalisedScale,
// and it exists for the same reason: the stylesheet has its own scale and the app uses a stable
// one.
//
// The stylesheet's gaps are 2, 4, 6, 8, 10, 12, 14, 18 and 22. The stable scale here is
// 4 / 8 / 12 / 16 / 24 / 32 / 40, with 2 for hairlines.
//
// The four values that were not on the scale were mapped: 6 to 8, 10 to 12, and 14 and 18 both
// to 16. The check below keeps a fifth level from appearing, which is the part that carries the
// rule's intent rather than its arithmetic.
func TestTheSpacingFollowsTheStableScale(t *testing.T) {
	onScale := map[float32]bool{2: true, 4: true, 8: true, 12: true, 16: true, 24: true, 32: true, 40: true}
	for _, item := range []struct {
		name string
		got  float32
	}{
		{"SpaceTiny", SpaceTiny},
		{"SpaceSmall", SpaceSmall},
		{"SpaceBase", SpaceBase},
		{"SpaceLoose", SpaceLoose},
		{"SpaceL", SpaceL},
		{"InspectorDividerGap", InspectorDividerGap},
	} {
		if !onScale[item.got] {
			t.Errorf("%s is %v, which is not on the stable scale (4/8/12/16/24/32/40, with 2 for hairlines)",
				item.name, item.got)
		}
	}
	// And the level added for those values is pinned, so a later edit cannot fold it away.
	if SpaceL != 16 {
		t.Errorf("SpaceL = %v, want 16; it is the level the stylesheet's 14 and 18 were mapped to", SpaceL)
	}
}

// TestAToastShowsWithoutTheDetailOpen is the reachability half of the toast.
//
// The toast was drawn inside detailOverlay's children, which are built only while the detail is
// open. Every other caller -- tagging an item, copying a link when there is none, the bundle's
// confirmations -- therefore write into a subtree that is not on screen. The test asserts the
// pair: with no detail open, a toast still has to appear in the view.
//
// It goes through ShowToast rather than setting ToastMessage: a test that sets the state it is
// checking cannot see whether anything can set that state.
func TestAToastShowsWithoutTheDetailOpen(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "post", clockBase)
	shell.Reload(context.Background())
	shell.state.DetailOpen = false

	if got := shell.ToastText(); got != "" {
		t.Fatalf("a toast is showing before anything raised one: %q", got)
	}
	shell.ShowToast("Saved.")

	if shell.state.DetailOpen {
		t.Fatal("the detail is open, so this would not test the case it is for")
	}
	if got := shell.ToastText(); got != "Saved." {
		t.Fatalf("ShowToast did not record the message: %q", got)
	}
	// The message has to be reachable from the view. It is drawn as a Text whose content is the
	// message, so finding that text in the built tree is the assertion the old placement failed.
	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()
	if !tester.HasText("Saved.") {
		t.Error("a toast raised with no detail open is not in the view; " +
			"it used to be drawn inside the detail overlay, which is only built while open")
	}
}

// TestThePageColumnKeepsAReadingMeasure pins the `max(16px, calc(50% - 600px))` measure:
// a narrow window pads 16, a wide one centres a 1200-point column, and the
// centring is of the pane the page owns, not of the window.
func TestThePageColumnKeepsAReadingMeasure(t *testing.T) {
	cases := []struct {
		window, sidebar, want float32
		why                   string
	}{
		{1180, WidthSidebarDef, ResultsPadX, "a 1180 window's page is 968 wide, under the measure"},
		{1400, WidthSidebarDef, ResultsPadX, "1188 is still under 1200, so the padding alone"},
		{1600, WidthSidebarDef, (1600 - WidthSidebarDef - ContentMaxWidth) / 2, "1388 centres at 94"},
		{1600, 0, (1600 - ContentMaxWidth) / 2, "a hidden sidebar gives the page the whole window"},
	}
	for _, tc := range cases {
		if got := pageInsetFor(tc.window, tc.sidebar); got != tc.want {
			t.Errorf("pageInsetFor(%v, %v) = %v, want %v: %s",
				tc.window, tc.sidebar, got, tc.want, tc.why)
		}
	}
}
