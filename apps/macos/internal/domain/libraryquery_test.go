package domain

import (
	"strings"
	"testing"
	"time"
)

var libBase = time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)

// --- the filter model ported from searchBookmarks.ts ---

func TestFilterNormalize(t *testing.T) {
	filter := LibraryFilter{
		Query:         "  rust  ",
		TagIDs:        []string{" t1 ", "t1", "", "t2"},
		AuthorHandles: []string{" Ada ", "ADA", "grace"},
		ListID:        "  l1  ",
	}
	normalized := filter.Normalize()

	if normalized.Query != "rust" {
		t.Errorf("query = %q, want it trimmed", normalized.Query)
	}
	// Tag ids are opaque, so they keep their case; duplicates and blanks go.
	if len(normalized.TagIDs) != 2 || normalized.TagIDs[0] != "t1" || normalized.TagIDs[1] != "t2" {
		t.Errorf("tag ids = %v, want t1 and t2", normalized.TagIDs)
	}
	// Handles are compared case-insensitively, so they are lowercased and deduped.
	if len(normalized.AuthorHandles) != 2 || normalized.AuthorHandles[0] != "ada" {
		t.Errorf("handles = %v, want ada and grace", normalized.AuthorHandles)
	}
	if normalized.ListID != "l1" {
		t.Errorf("list = %q", normalized.ListID)
	}

	// The mode defaults are the extension's: tags "all", authors "any".
	if normalized.TagMatchMode != MatchAll {
		t.Errorf("tag match mode = %q, want %q", normalized.TagMatchMode, MatchAll)
	}
	if normalized.AuthorMatchMode != MatchAny {
		t.Errorf("author match mode = %q, want %q", normalized.AuthorMatchMode, MatchAny)
	}
	if normalized.TimeRange != RangeAll {
		t.Errorf("time range = %q, want %q", normalized.TimeRange, RangeAll)
	}

	// An explicit mode survives normalisation.
	filter.TagMatchMode = MatchAny
	if got := filter.Normalize().TagMatchMode; got != MatchAny {
		t.Errorf("an explicit mode was overwritten: %q", got)
	}
}

func TestFilterValidateRejectsUnknownValues(t *testing.T) {
	tests := []struct {
		name   string
		filter LibraryFilter
	}{
		{"tag mode", LibraryFilter{TagMatchMode: "sometimes"}},
		{"author mode", LibraryFilter{AuthorMatchMode: "maybe"}},
		{"time range", LibraryFilter{TimeRange: "forever"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.filter.Validate(); err == nil {
				t.Error("an unknown value must be reported, not defaulted")
			}
		})
	}

	// The defaults and every known value pass.
	for _, mode := range []MultiValueMatchMode{MatchAny, MatchAll} {
		if err := (LibraryFilter{TagMatchMode: mode, AuthorMatchMode: mode}).Validate(); err != nil {
			t.Errorf("mode %q must be valid: %v", mode, err)
		}
	}
	for _, window := range AllSavedTimeRanges {
		if err := (LibraryFilter{TimeRange: window}).Validate(); err != nil {
			t.Errorf("range %q must be valid: %v", window, err)
		}
	}
}

func TestFilterIsZero(t *testing.T) {
	if !(LibraryFilter{}).IsZero() {
		t.Error("an empty filter selects everything")
	}

	// Every field must be able to make it non-empty.
	notZero := []LibraryFilter{
		{Query: "x"},
		{TagIDs: []string{"t1"}},
		{AuthorHandles: []string{"ada"}},
		{ListID: "l1"},
		{OnlyUnclassified: true},
		{OnlyWithMedia: true},
		{OnlyLongform: true},
		{TimeRange: Range7d},
	}
	for i, filter := range notZero {
		if filter.IsZero() {
			t.Errorf("filter %d must not read as empty: %+v", i, filter)
		}
	}

	// Whitespace alone is still empty.
	if !(LibraryFilter{Query: "   ", TagIDs: []string{"  "}}).IsZero() {
		t.Error("blank values must not count as filters")
	}
}

// TestFilterActiveCountMatchesTheSpec covers the badge the UI shows: the spec
// counts only the hidden toggles, because the query and tags are visible in the
// toolbar and counting them would make the badge meaningless.
func TestFilterActiveCountMatchesTheSpec(t *testing.T) {
	tests := []struct {
		name   string
		filter LibraryFilter
		want   int
	}{
		{"nothing", LibraryFilter{}, 0},
		{"media only", LibraryFilter{OnlyWithMedia: true}, 1},
		{"longform only", LibraryFilter{OnlyLongform: true}, 1},
		{"both", LibraryFilter{OnlyWithMedia: true, OnlyLongform: true}, 2},
		// These are visible in the toolbar, so they do not inflate the badge.
		{"a query does not count", LibraryFilter{Query: "rust"}, 0},
		{"tags do not count", LibraryFilter{TagIDs: []string{"t1"}}, 0},
		{"authors do not count", LibraryFilter{AuthorHandles: []string{"ada"}}, 0},
		{"a list does not count", LibraryFilter{ListID: "l1"}, 0},
		{"a time range does not count", LibraryFilter{TimeRange: Range7d}, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.filter.ActiveCount(); got != tc.want {
				t.Errorf("ActiveCount = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestSavedAfterIsTheOnlyCutoffRule pins the rule the storage layer now calls
// instead of recomputing. The two had drifted apart before.
func TestSavedAfterIsTheOnlyCutoffRule(t *testing.T) {
	// "all" has no cutoff.
	if got := (LibraryFilter{TimeRange: RangeAll}).SavedAfter(libBase); !got.IsZero() {
		t.Errorf("RangeAll produced %v, want the zero time", got)
	}

	// Each window is counted back in days, and the result is UTC so it compares
	// against the stored timestamps, which are fixed-width UTC strings.
	for _, tc := range []struct {
		window SavedTimeRange
		want   time.Time
	}{
		{Range7d, libBase.AddDate(0, 0, -7)},
		{Range30d, libBase.AddDate(0, 0, -30)},
		{Range90d, libBase.AddDate(0, 0, -90)},
	} {
		got := (LibraryFilter{TimeRange: tc.window}).SavedAfter(libBase)
		if !got.Equal(tc.want) {
			t.Errorf("%s cutoff = %v, want %v", tc.window, got, tc.want)
		}
		if got.Location() != time.UTC {
			t.Errorf("%s cutoff is in %v, want UTC so it compares against stored timestamps",
				tc.window, got.Location())
		}
	}

	// A non-UTC clock reading must still produce a UTC cutoff.
	shanghai := time.FixedZone("CST", 8*3600)
	local := libBase.In(shanghai)
	if got := (LibraryFilter{TimeRange: Range7d}).SavedAfter(local); got.Location() != time.UTC {
		t.Errorf("cutoff location = %v, want UTC", got.Location())
	}
}

func TestSavedTimeRangeDays(t *testing.T) {
	for _, tc := range []struct {
		window SavedTimeRange
		want   int
	}{
		{RangeAll, 0},
		{Range7d, 7},
		{Range30d, 30},
		{Range90d, 90},
		{"invented", 0},
	} {
		if got := SavedTimeRangeDays(tc.window); got != tc.want {
			t.Errorf("SavedTimeRangeDays(%q) = %d, want %d", tc.window, got, tc.want)
		}
	}
}

func TestQueryNormalize(t *testing.T) {
	// Defaults.
	normalized, err := (LibraryQuery{}).Normalize()
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if normalized.Sort != SortTimeline {
		t.Errorf("sort = %q, want %q", normalized.Sort, SortTimeline)
	}
	if normalized.Limit != DefaultLibraryPageSize {
		t.Errorf("limit = %d, want the default", normalized.Limit)
	}

	// Every sort order is accepted.
	for _, order := range AllSortOrders {
		if _, err := (LibraryQuery{Sort: order}).Normalize(); err != nil {
			t.Errorf("sort %q must be accepted: %v", order, err)
		}
	}

	// An unknown order is reported rather than defaulted: a view that silently
	// reorders itself is worse than one that says the value is wrong.
	if _, err := (LibraryQuery{Sort: "by-vibes"}).Normalize(); err == nil {
		t.Error("an unknown sort order must be rejected")
	}

	// The window is clamped.
	if _, err := (LibraryQuery{Limit: -1}).Normalize(); err == nil {
		t.Error("a negative limit must be rejected")
	}
	if _, err := (LibraryQuery{Limit: MaxLibraryPageSize + 1}).Normalize(); err == nil {
		t.Error("a limit past the ceiling must be rejected")
	}
	clamped, err := (LibraryQuery{Offset: -5}).Normalize()
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if clamped.Offset != 0 {
		t.Errorf("offset = %d, want 0", clamped.Offset)
	}

	// An invalid filter is reported through the query.
	if _, err := (LibraryQuery{Filter: LibraryFilter{TimeRange: "forever"}}).Normalize(); err == nil {
		t.Error("an invalid filter must be rejected")
	}
}

func TestValidMatchModeAndRange(t *testing.T) {
	if !ValidMatchMode(MatchAny) || !ValidMatchMode(MatchAll) {
		t.Error("both match modes must be valid")
	}
	if ValidMatchMode("some") {
		t.Error("an unknown match mode must be rejected")
	}

	for _, window := range AllSavedTimeRanges {
		if !ValidSavedTimeRange(window) {
			t.Errorf("%q must be valid", window)
		}
	}
	if ValidSavedTimeRange("1y") {
		t.Error("an unknown range must be rejected")
	}
	if len(AllSavedTimeRanges) != 4 {
		t.Errorf("%d ranges, want the extension's 4", len(AllSavedTimeRanges))
	}
}

// --- the source column ---

func TestTagSourceVocabulary(t *testing.T) {
	for _, source := range AllTagSources {
		if !ValidTagSource(source) {
			t.Errorf("%q must be valid", source)
		}
	}
	if ValidTagSource("invented") {
		t.Error("an unknown source must be rejected")
	}
	if ValidTagSource("") {
		t.Error("an empty source must be rejected; the column is NOT NULL")
	}

	// The automatic set is what a prune filters on, so it must exclude manual.
	automatic := AutomaticSources()
	if len(automatic) == 0 {
		t.Fatal("the automatic set must not be empty")
	}
	for _, source := range automatic {
		if source == TagSourceManual {
			t.Error("manual must never be in the automatic set, or a prune would delete a user's tag")
		}
		if !ValidTagSource(source) {
			t.Errorf("the automatic set contains an unknown source %q", source)
		}
	}

	// Every non-manual source is in the set, or a prune would leave some behind.
	for _, source := range AllTagSources {
		if source == TagSourceManual {
			continue
		}
		found := false
		for _, automaticSource := range automatic {
			if automaticSource == source {
				found = true
			}
		}
		if !found {
			t.Errorf("automatic source %q is missing from the prune set", source)
		}
	}
}

func TestSuggestionVocabulary(t *testing.T) {
	for _, source := range AllSuggestionSources {
		if !ValidSuggestionSource(source) {
			t.Errorf("%q must be valid", source)
		}
	}
	for _, status := range AllSuggestionStatuses {
		if !ValidSuggestionStatus(status) {
			t.Errorf("%q must be valid", status)
		}
	}
	if ValidSuggestionSource("guessed") {
		t.Error("an unknown suggestion source must be rejected")
	}
	if ValidSuggestionStatus("maybe") {
		t.Error("an unknown status must be rejected: status decides whether it is offered again")
	}

	// The vocabularies must match the schema's CHECK constraints exactly.
	if len(AllSuggestionSources) != 4 {
		t.Errorf("%d sources, want the schema's 4", len(AllSuggestionSources))
	}
	if len(AllSuggestionStatuses) != 3 {
		t.Errorf("%d statuses, want the schema's 3", len(AllSuggestionStatuses))
	}
}

// --- X's timestamp rendering (the T8 fix) ---

// TestParseXTimestamp covers the format that cannot be sorted as text. X renders a
// post's creation time starting with a weekday name, so ordering those strings
// orders by weekday; this parse is what makes a correct sort possible.
func TestParseXTimestamp(t *testing.T) {
	parsed, ok := ParseXTimestamp("Wed Oct 10 20:19:24 +0000 2018")
	if !ok {
		t.Fatal("X's own rendering must parse")
	}
	want := time.Date(2018, 10, 10, 20, 19, 24, 0, time.UTC)
	if !parsed.Equal(want) {
		t.Errorf("parsed = %v, want %v", parsed, want)
	}

	// ISO is accepted too, because a restored backup may already hold it.
	parsed, ok = ParseXTimestamp("2026-10-12T14:22:31.000Z")
	if !ok {
		t.Fatal("an ISO timestamp must parse as well")
	}
	if parsed.Year() != 2026 {
		t.Errorf("parsed = %v", parsed)
	}

	// Leading and trailing space is tolerated.
	if _, ok := ParseXTimestamp("  Wed Oct 10 20:19:24 +0000 2018  "); !ok {
		t.Error("surrounding space must be tolerated")
	}

	// Anything else is reported as unparseable rather than guessed at, so the
	// column stays empty and sorts last instead of sorting wrong.
	for _, bad := range []string{"", "   ", "not a date", "2026-13-45"} {
		if _, ok := ParseXTimestamp(bad); ok {
			t.Errorf("%q must not parse", bad)
		}
	}

	// A non-UTC offset is honoured, not discarded.
	offset, ok := ParseXTimestamp("Wed Oct 10 20:19:24 +0800 2018")
	if !ok {
		t.Fatal("an offset rendering must parse")
	}
	if !offset.Equal(want.Add(-8 * time.Hour)) {
		t.Errorf("parsed = %v, want the instant the offset describes", offset)
	}
}

func TestFormatXTimestampOrEmpty(t *testing.T) {
	// The sortable form is the fixed-width UTC shape, so TEXT comparison is
	// chronological.
	got := FormatXTimestampOrEmpty("Wed Oct 10 20:19:24 +0000 2018")
	if got != "2018-10-10T20:19:24.000Z" {
		t.Errorf("formatted = %q", got)
	}

	// Unparseable input becomes the empty string, which sorts last in a descending
	// order -- the same place the extension's Date.parse fallback of 0 put it.
	for _, bad := range []string{"", "nonsense"} {
		if got := FormatXTimestampOrEmpty(bad); got != "" {
			t.Errorf("FormatXTimestampOrEmpty(%q) = %q, want empty", bad, got)
		}
	}

	// The empty string must sort before any real timestamp.
	if !("" < "2018-10-10T20:19:24.000Z") {
		t.Error("the unknown value must sort before a known one")
	}
}

// --- error messages a user can see ---

func TestErrorMessagesAreReadable(t *testing.T) {
	listErr := &ListError{Reason: "Inbox cannot be renamed"}
	if !strings.Contains(listErr.Error(), "Inbox cannot be renamed") {
		t.Errorf("ListError.Error() = %q", listErr.Error())
	}

	hierarchyErr := &HierarchyError{Reason: "too deep"}
	if !strings.Contains(hierarchyErr.Error(), "too deep") {
		t.Errorf("HierarchyError.Error() = %q", hierarchyErr.Error())
	}

	// The sentinel must be checkable with errors.Is, which is how the UI tells
	// "move the bookmarks first" from a storage failure.
	if ErrListNotEmpty == nil || ErrListNotEmpty.Error() == "" {
		t.Error("ErrListNotEmpty must be a usable sentinel")
	}
}
