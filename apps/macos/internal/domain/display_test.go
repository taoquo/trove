package domain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Equivalence against the real formatting helpers in TypeScript.
//
// Regenerate with:
//
//	node apps/macos/testdata/format/genformat.mjs
type formatFixture struct {
	GeneratedFrom string `json:"generatedFrom"`
	Compact       []struct {
		Locale   string `json:"locale"`
		Value    int    `json:"value"`
		Expected string `json:"expected"`
	} `json:"compact"`
	Truncated []struct {
		Length         int    `json:"length"`
		ExpectedLength int    `json:"expectedLength"`
		ExpectedTail   string `json:"expectedTail"`
	} `json:"truncated"`
	TagCases []struct {
		ViewMode string   `json:"viewMode"`
		Count    int      `json:"count"`
		Expected []string `json:"expected"`
	} `json:"tagCases"`
	VideoCases []struct {
		Type     *string `json:"type"`
		Expected bool    `json:"expected"`
	} `json:"videoCases"`
	TimestampCases []struct {
		Locale   string `json:"locale"`
		Expected string `json:"expected"`
	} `json:"timestampCases"`
	DensityCases []struct {
		Length        int    `json:"length"`
		HasMedia      bool   `json:"hasMedia"`
		HasTags       bool   `json:"hasTags"`
		CopyDensity   string `json:"copyDensityClass"`
		TemplateClass string `json:"templateClass"`
	} `json:"densityCases"`
	TrimCases []struct {
		HasMedia bool   `json:"hasMedia"`
		HasTags  bool   `json:"hasTags"`
		Expected string `json:"expected"`
	} `json:"trimCases"`
	DefaultTruncateLength int `json:"defaultTruncateLength"`
}

func loadFormatFixture(t *testing.T) formatFixture {
	t.Helper()

	path := filepath.Join("..", "..", "testdata", "format", "format.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture (regenerate with `node apps/macos/testdata/format/genformat.mjs`): %v", err)
	}

	var fixture formatFixture
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	if fixture.GeneratedFrom != "src/options/utils.ts" {
		t.Errorf("fixture generated from %q", fixture.GeneratedFrom)
	}
	return fixture
}

// TestFormatCompactCountMatchesTypeScript is the check that matters most here: compact
// notation is locale-specific in a way no reading of the call site reveals.
func TestFormatCompactCountMatchesTypeScript(t *testing.T) {
	fixture := loadFormatFixture(t)

	if len(fixture.Compact) == 0 {
		t.Fatal("the fixture has no compact cases")
	}

	for _, tc := range fixture.Compact {
		if got := FormatCompactCount(tc.Value, tc.Locale); got != tc.Expected {
			t.Errorf("FormatCompactCount(%d, %q) = %q, want %q", tc.Value, tc.Locale, got, tc.Expected)
		}
	}

	// The two rules that are easy to get wrong, stated outright.
	if got := FormatCompactCount(12345, "en"); got != "12.3K" {
		t.Errorf("English at 12345 = %q, want 12.3K", got)
	}
	if got := FormatCompactCount(12345, "zh-CN"); got != "1.2万" {
		t.Errorf("Chinese at 12345 = %q, want 1.2万 -- Chinese abbreviates at ten thousand", got)
	}
	if got := FormatCompactCount(1234, "zh-CN"); got != "1234" {
		t.Errorf("Chinese at 1234 = %q, want the plain number: there is no thousands step", got)
	}
	// A whole number loses its decimal.
	if got := FormatCompactCount(1000, "en"); got != "1K" {
		t.Errorf("1000 = %q, want 1K rather than 1.0K", got)
	}
}

// TestTruncateTextMatchesTypeScript reproduces the source's arithmetic, including the
// part that looks like an off-by-one.
func TestTruncateTextMatchesTypeScript(t *testing.T) {
	fixture := loadFormatFixture(t)

	if DefaultTruncateLength != fixture.DefaultTruncateLength {
		t.Errorf("the default length is %d, want the source's %d",
			DefaultTruncateLength, fixture.DefaultTruncateLength)
	}

	for _, tc := range fixture.Truncated {
		// The fixture's one-character case is "a", not "x", so the body is rebuilt from
		// the same shape rather than assumed.
		text := strings.Repeat("x", tc.Length)
		if tc.Length == 1 {
			text = "a"
		}

		got := TruncateText(text, 0)
		if len([]rune(got)) != tc.ExpectedLength {
			t.Errorf("truncating %d characters gave %d, want %d",
				tc.Length, len([]rune(got)), tc.ExpectedLength)
		}
		if tail := string([]rune(got)[max(0, len([]rune(got))-4):]); tail != tc.ExpectedTail {
			t.Errorf("truncating %d characters ends with %q, want %q", tc.Length, tail, tc.ExpectedTail)
		}
	}
}

// TestVisibleCardTagNamesMatchesTypeScript covers the collapse, which is why a card
// stays readable.
func TestVisibleCardTagNamesMatchesTypeScript(t *testing.T) {
	fixture := loadFormatFixture(t)

	for _, tc := range fixture.TagCases {
		names := make([]string, 0, tc.Count)
		for i := 0; i < tc.Count; i++ {
			names = append(names, "t"+strconv.Itoa(i))
		}

		got := VisibleCardTagNames(names, tc.ViewMode)
		if len(got) != len(tc.Expected) {
			t.Errorf("%s with %d tags = %v, want %v", tc.ViewMode, tc.Count, got, tc.Expected)
			continue
		}
		for index := range tc.Expected {
			if got[index] != tc.Expected[index] {
				t.Errorf("%s with %d tags = %v, want %v", tc.ViewMode, tc.Count, got, tc.Expected)
				break
			}
		}
	}

	// Grid mode is not collapsed: it has the width.
	names := []string{"a", "b", "c", "d"}
	if got := VisibleCardTagNames(names, ViewGrid); len(got) != 4 {
		t.Errorf("grid mode collapsed %v", got)
	}
	// List mode collapses past two, and the last entry is the count.
	if got := VisibleCardTagNames(names, ViewList); len(got) != 3 || got[2] != "+2" {
		t.Errorf("list mode = %v, want two names and +2", got)
	}
}

// TestIsVideoMediaTypeMatchesTypeScript covers the play-affordance decision.
func TestIsVideoMediaTypeMatchesTypeScript(t *testing.T) {
	fixture := loadFormatFixture(t)

	for _, tc := range fixture.VideoCases {
		value := ""
		if tc.Type != nil {
			value = *tc.Type
		}
		if got := IsVideoMediaType(value); got != tc.Expected {
			t.Errorf("IsVideoMediaType(%q) = %v, want %v", value, got, tc.Expected)
		}
	}

	// An animated GIF is video because it plays, which is what earns it the affordance.
	if !IsVideoMediaType("animated_gif") {
		t.Error("an animated GIF must be treated as video")
	}
}

// TestNotSyncedYetMatchesTypeScript covers the empty-timestamp copy.
func TestNotSyncedYetMatchesTypeScript(t *testing.T) {
	for _, tc := range loadFormatFixture(t).TimestampCases {
		if got := NotSyncedYet(tc.Locale); got != tc.Expected {
			t.Errorf("NotSyncedYet(%q) = %q, want %q", tc.Locale, got, tc.Expected)
		}
	}
}

// TestCardDensityMatchesTypeScript covers the rule that decides how many lines a card
// reserves. The panel used to reserve four lines for every card, so a one-line post and
// a long article looked the same and the body was cut off at the same place either way.
func TestCardDensityMatchesTypeScript(t *testing.T) {
	fixture := loadFormatFixture(t)

	if len(fixture.DensityCases) == 0 {
		t.Fatal("the fixture has no density cases")
	}

	// The source's class name encodes the line count, so it is the expected value.
	linesFor := map[string]int{
		"is-list-1line": CardOneLine,
		"is-list-2line": CardTwoLines,
		"is-list-3line": CardThreeLines,
	}

	for _, tc := range fixture.DensityCases {
		expected, known := linesFor[tc.CopyDensity]
		if !known {
			t.Fatalf("the fixture has an unknown density class %q", tc.CopyDensity)
		}

		text := strings.Repeat("x", tc.Length)
		if got := CardLineCount(text, tc.HasMedia, tc.HasTags); got != expected {
			t.Errorf("CardLineCount(len=%d, media=%v, tags=%v) = %d, want %d (%s)",
				tc.Length, tc.HasMedia, tc.HasTags, got, expected, tc.CopyDensity)
		}
		if got := CardTemplate(tc.HasMedia, tc.HasTags, expected); got != tc.TemplateClass {
			t.Errorf("CardTemplate(media=%v, tags=%v, %d) = %q, want %q",
				tc.HasMedia, tc.HasTags, expected, got, tc.TemplateClass)
		}
	}

	// The four layouts, stated outright.
	if CardTemplate(true, true, CardTwoLines) != CardTemplateMedia {
		t.Error("media wins over tags")
	}
	if CardTemplate(false, true, CardTwoLines) != CardTemplateTagged {
		t.Error("tags pick the tagged layout")
	}
	if CardTemplate(false, false, CardOneLine) != CardTemplateCompact {
		t.Error("a one-line untagged post is compact")
	}
	if CardTemplate(false, false, CardThreeLines) != CardTemplateDefault {
		t.Error("a long untagged post is the default layout")
	}

	// Whitespace is trimmed before measuring, so padding does not buy a line. The
	// padded body is passed in as-is: trimming it here would test the test.
	for _, tc := range fixture.TrimCases {
		padded := strings.Repeat(" ", 60) + "short"
		if got := CardLineCount(padded, tc.HasMedia, tc.HasTags); got != linesFor[tc.Expected] {
			t.Errorf("a padded short post = %d lines, want %d (%s)",
				got, linesFor[tc.Expected], tc.Expected)
		}
	}

	// The trim is the whole point of those cases, so it is asserted directly.
	if got := CardLineCount(strings.Repeat(" ", 300), false, false); got != CardOneLine {
		t.Errorf("whitespace alone measures as %d lines, want 1: it trims to empty", got)
	}
}

// TestListCardMinHeightMatchesTheStylesheet pins the four template heights.
func TestListCardMinHeightMatchesTheStylesheet(t *testing.T) {
	cases := []struct {
		hasMedia, hasTags bool
		lines             int
		want              float32
		from              string
	}{
		{true, false, 2, 188, "template-media"},
		{true, true, 2, 188, "media wins over tags"},
		{false, true, 1, 176, "template-tagged"},
		{false, false, 1, 132, "template-compact"},
		{false, false, 2, 154, "template-default"},
		{false, false, 3, 154, "template-default"},
		{false, false, 0, 132, "compact, as a short copy is one line"},
	}
	for _, item := range cases {
		got := ListCardMinHeight(item.hasMedia, item.hasTags, item.lines)
		if got != item.want {
			t.Errorf("ListCardMinHeight(%v, %v, %d) = %v, want %v (%s)",
				item.hasMedia, item.hasTags, item.lines, got, item.want, item.from)
		}
	}
}

// TestMediaThumbnailURL covers which picture a card draws.
func TestMediaThumbnailURL(t *testing.T) {
	poster := "https://example.test/poster.jpg"
	cases := []struct {
		name  string
		media []Media
		want  string
	}{
		{"none", nil, ""},
		{"image", []Media{{Type: MediaPhoto, URL: "https://example.test/a.jpg"}},
			"https://example.test/a.jpg"},
		{"video takes the poster",
			[]Media{{Type: MediaVideo, URL: "https://example.test/v.mp4", PosterURL: &poster}},
			poster},
		{"video without a poster", []Media{{Type: MediaVideo, URL: "https://example.test/v.mp4"}}, ""},
		// media[0] decides: the source never looks past the first item
		// (BookmarkCard.tsx:25-28). This case used to answer the later photo, which is a
		// different picture from the one the reference shows.
		{"the first item decides, even when a photo follows",
			[]Media{
				{Type: MediaVideo, URL: "https://example.test/v.mp4", PosterURL: &poster},
				{Type: MediaPhoto, URL: "https://example.test/a.jpg"},
			},
			poster},
		{"an animated gif takes its poster too",
			[]Media{{Type: MediaAnimatedGif, URL: "https://example.test/g.mp4", PosterURL: &poster}},
			poster},
	}
	for _, item := range cases {
		if got := MediaThumbnailURL(item.media); got != item.want {
			t.Errorf("%s: MediaThumbnailURL = %q, want %q", item.name, got, item.want)
		}
	}
}

// TestTweetIDFromURL covers the links a user can paste.
//
// The extension reads the id off the page it is injected into; a standalone manager has no
// page, so the link is the only source and every shape X produces has to be handled.
func TestTweetIDFromURL(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{"x.com", "https://x.com/ada/status/1780000000000000001", "1780000000000000001", true},
		{"twitter.com", "https://twitter.com/ada/status/1780000000000000002", "1780000000000000002", true},
		{"no scheme", "x.com/ada/status/1780000000000000003", "1780000000000000003", true},
		{"www", "https://www.x.com/ada/status/1780000000000000004", "1780000000000000004", true},
		{"mobile", "https://mobile.twitter.com/ada/status/1780000000000000005", "1780000000000000005", true},
		{"tracking query", "https://x.com/ada/status/1780000000000000006?s=20&t=abc", "1780000000000000006", true},
		{"fragment", "https://x.com/ada/status/1780000000000000007#top", "1780000000000000007", true},
		{"handleless", "https://x.com/i/web/status/1780000000000000008", "1780000000000000008", true},
		{"surrounding space", "  https://x.com/ada/status/1780000000000000009  ", "1780000000000000009", true},
		{"old statuses path", "https://twitter.com/ada/statuses/1780000000000000010", "1780000000000000010", true},

		{"empty", "", "", false},
		{"another site", "https://example.com/ada/status/1780000000000000001", "", false},
		{"a profile", "https://x.com/ada", "", false},
		{"a search", "https://x.com/search?q=status", "", false},
		{"non-numeric id", "https://x.com/ada/status/notanid", "", false},
		{"status without an id", "https://x.com/ada/status/", "", false},
		// A lookalike host must not pass: the check is on the whole label, not a suffix.
		{"lookalike host", "https://notx.com/ada/status/1780000000000000001", "", false},
		{"lookalike subdomain", "https://x.com.evil.test/ada/status/1780000000000000001", "", false},
	}
	for _, tc := range cases {
		got, ok := TweetIDFromURL(tc.in)
		if ok != tc.ok || got != tc.want {
			t.Errorf("%s: TweetIDFromURL(%q) = %q, %v; want %q, %v",
				tc.name, tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

// TestMergeDraftKeepsWhatIsStored is one half of a pair.
//
// The other half pins the opposite rule on the repository. They exist together because
// the two paths need different semantics, and a single test cannot say which path it is
// protecting: making UpsertMany use COALESCE would keep this test green and quietly
// stop sync from updating an author's changed display name.
func TestMergeDraftKeepsWhatIsStored(t *testing.T) {
	saved := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	stored := Bookmark{
		TweetID: "1", TweetURL: "https://x.com/ada/status/1",
		AuthorName: "Ada Lovelace", AuthorHandle: "ada", AuthorAvatarURL: "https://a.test/ada.jpg",
		Text: "the stored copy", CreatedAtOnX: "2026-01-01T00:00:00.000Z",
		SavedAt: saved, TimelineRank: intPtr(42),
		Media:   []Media{{Type: MediaPhoto, URL: "https://a.test/p.jpg"}},
		Metrics: &Metrics{Likes: intPtr(7)},
	}
	// A draft from a pasted link carries almost nothing.
	draft := Bookmark{
		TweetID: "1", TweetURL: "https://x.com/ada/status/1",
		AuthorHandle: "ada", Text: "what the user typed",
	}

	now := saved.Add(time.Hour)
	merged := MergeDraft(stored, draft, now)

	// Every stored field the draft does not carry must survive.
	if merged.AuthorName != "Ada Lovelace" {
		t.Errorf("the author name became %q", merged.AuthorName)
	}
	if merged.AuthorAvatarURL != "https://a.test/ada.jpg" {
		t.Errorf("the avatar became %q", merged.AuthorAvatarURL)
	}
	if merged.Text != "the stored copy" {
		t.Errorf("the text became %q; the stored copy wins over the draft", merged.Text)
	}
	if merged.CreatedAtOnX != "2026-01-01T00:00:00.000Z" {
		t.Errorf("createdAtOnX became %q", merged.CreatedAtOnX)
	}
	if len(merged.Media) != 1 || len(merged.Media) != len(stored.Media) {
		t.Errorf("the media became %v", merged.Media)
	}
	if merged.Metrics == nil || merged.Metrics.Likes == nil || *merged.Metrics.Likes != 7 {
		t.Errorf("the metrics became %v", merged.Metrics)
	}
	// savedAt is the user's own timeline and must not move; lastSeenAt must.
	if !merged.SavedAt.Equal(saved) {
		t.Errorf("savedAt moved from %v to %v", saved, merged.SavedAt)
	}
	if !merged.LastSeenAt.Equal(now) {
		t.Errorf("lastSeenAt is %v, want %v", merged.LastSeenAt, now)
	}
	// An existing rank is kept; sync owns it.
	if merged.TimelineRank == nil || *merged.TimelineRank != 42 {
		t.Errorf("the timeline rank became %v, want 42", merged.TimelineRank)
	}
}

// TestMergeDraftFillsAnEmptyRecord is the other direction: a first-time add.
func TestMergeDraftFillsAnEmptyRecord(t *testing.T) {
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	draft := Bookmark{
		TweetID: "2", TweetURL: "https://x.com/ada/status/2",
		AuthorHandle: "ada", Text: "a first-time add",
	}
	merged := MergeDraft(Bookmark{}, draft, now)

	if merged.TweetID != "2" || merged.Text != "a first-time add" || merged.AuthorHandle != "ada" {
		t.Errorf("the draft did not land: %+v", merged)
	}
	if !merged.SavedAt.Equal(now) {
		t.Errorf("a new record's savedAt is %v, want %v", merged.SavedAt, now)
	}
	// A negative rank, so a hand-added post appears above the synced timeline.
	if merged.TimelineRank == nil || *merged.TimelineRank >= 0 {
		t.Errorf("a new record's rank is %v, want a negative one", merged.TimelineRank)
	}
}

func intPtr(v int) *int { return &v }

// TestFormatSavedAtIsNotTheStorageForm covers what a card shows.
//
// The card rendered domain.FormatTimestamp, which is the RFC 3339 storage form: a user saw
// "2026-10-15T00:00:00.000Z". The extension formats it through Intl.DateTimeFormat with a
// short month, a numeric day and a numeric hour, so this must not equal the storage string.
func TestFormatSavedAtIsNotTheStorageForm(t *testing.T) {
	stored := "2026-10-15T00:00:00.000Z"

	en := FormatSavedAt(stored, "en")
	if en == stored {
		t.Fatalf("the display form is still the storage form: %q", en)
	}
	if !strings.Contains(en, "Oct") || !strings.Contains(en, "15") {
		t.Errorf("en = %q, want a short month and the day", en)
	}
	if strings.Contains(en, "T00:00:00") || strings.Contains(en, "Z") {
		t.Errorf("en = %q, want no storage punctuation", en)
	}

	zh := FormatSavedAt(stored, "zh-CN")
	if zh == stored || zh == en {
		t.Errorf("zh = %q, want its own form", zh)
	}
	if !strings.Contains(zh, "10月") || !strings.Contains(zh, "15日") {
		t.Errorf("zh = %q, want 月 and 日", zh)
	}
}

// TestFormatSavedAtReadsTheTimestampXSent covers the inspector's published node.
//
// createdAtOnX is stored verbatim, in X's own rendering, and the inspector shows it
// through FormatSavedAt. That function parsed RFC 3339 only, so this value took the "will
// not parse, return it unchanged" branch: the panel read "发布于 Mon Nov 02 06:40:09 +0000
// 2026" beside a formatted saved date. The extension formats the same string because
// `new Date(value)` accepts that rendering (src/options/utils.ts:48).
func TestFormatSavedAtReadsTheTimestampXSent(t *testing.T) {
	const xRendered = "Wed Oct 10 20:19:24 +0000 2018"
	// The same instant, written the way storage writes it. Formatting both has to
	// produce the same sentence -- that is the whole property, and it does not depend
	// on this machine's time zone the way asserting "Oct 10" would.
	const stored = "2018-10-10T20:19:24.000Z"

	for _, locale := range []string{"en", "zh-CN"} {
		got := FormatSavedAt(xRendered, locale)
		if want := FormatSavedAt(stored, locale); got != want {
			t.Errorf("%s: an X timestamp rendered %q, want %q", locale, got, want)
		}
		if strings.Contains(got, "+0000") || strings.Contains(got, "2018 ") {
			t.Errorf("%s: %q is still the wire format", locale, got)
		}
	}
}

// TestFormatSavedAtKeepsTheTwoEdgeCases covers the rules that are not formatting.
func TestFormatSavedAtKeepsTheTwoEdgeCases(t *testing.T) {
	// An empty value is the "not synced yet" sentence, which is a statement about the
	// library rather than a missing value.
	for _, locale := range []string{"en", "zh-CN"} {
		if got := FormatSavedAt("", locale); got == "" {
			t.Errorf("an empty value produced an empty string for %q", locale)
		}
	}
	if FormatSavedAt("", "en") != "Not synced yet" || FormatSavedAt("", "zh-CN") != "尚未同步" {
		t.Errorf("empty: %q / %q", FormatSavedAt("", "en"), FormatSavedAt("", "zh-CN"))
	}

	// A value that will not parse is returned unchanged: replacing something a user could
	// inspect with a placeholder loses information.
	if got := FormatSavedAt("0001-01-01T00:00:00.000Z", "en"); got != "\u2014" {
		t.Errorf("a zero timestamp = %q, want an em dash rather than a year-1 date", got)
	}
	if got := FormatSavedAt("not a date", "en"); got != "not a date" {
		t.Errorf("unparseable = %q, want it unchanged", got)
	}
}
