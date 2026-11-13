package domain

import "testing"

func TestSyncErrorKindsCoverBothVocabularies(t *testing.T) {
	seen := map[string]int{}
	for _, k := range AllSyncErrorKinds {
		seen[k]++
	}
	for k, n := range seen {
		if n > 1 {
			t.Errorf("error kind %q appears %d times in AllSyncErrorKinds", k, n)
		}
	}

	// Every kind a stored row or an old backup may carry must validate.
	for _, k := range legacySyncErrorKinds {
		if !ValidSyncErrorKind(k) {
			t.Errorf("legacy kind %q must stay valid: existing rows and backups carry it", k)
		}
	}
	for _, k := range currentSyncErrorKinds {
		if !ValidSyncErrorKind(k) {
			t.Errorf("current kind %q must be valid", k)
		}
	}

	if !ValidSyncErrorKind("") {
		t.Error("an absent error kind is legal: a successful run has none")
	}
	if ValidSyncErrorKind("made-up") {
		t.Error("an unknown kind must be rejected")
	}
}

// TestSyncErrorKindRenameMapIsTotal documents the mapping from the old
// vocabulary to the new one. Every legacy kind must have a decided home.
func TestSyncErrorKindRenameMapIsTotal(t *testing.T) {
	rename := map[SyncErrorKind]SyncErrorKind{
		ErrAuthExpired:    ErrUnauthorized,
		ErrRateLimited:    ErrRateLimited, // unchanged
		ErrNetworkError:   ErrNetwork,
		ErrXSchemaChanged: ErrResponseSchemaChanged,
		ErrInterrupted:    ErrInterrupted, // the plan dropped this; it is kept
		ErrUnknown:        ErrUnknown,
	}
	for legacy, current := range rename {
		if !ValidSyncErrorKind(legacy) {
			t.Errorf("legacy kind %q is not accepted", legacy)
		}
		if !ValidSyncErrorKind(current) {
			t.Errorf("legacy kind %q maps to %q, which is not accepted", legacy, current)
		}
	}
	if len(rename) != len(legacySyncErrorKinds) {
		t.Errorf("rename map covers %d kinds but there are %d legacy kinds",
			len(rename), len(legacySyncErrorKinds))
	}
}

func TestSyncStatusValidation(t *testing.T) {
	valid := []SyncStatus{
		SyncIdle, SyncRunning, SyncSuccess, SyncPartialSuccess, SyncCancelled, SyncError,
	}
	for _, s := range valid {
		if !ValidSyncStatus(s) {
			t.Errorf("%q must be a valid status", s)
		}
	}
	for _, s := range []string{"partial", "done", "", "SUCCESS"} {
		if ValidSyncStatus(s) {
			t.Errorf("%q must not be a valid status", s)
		}
	}
}

// TestCompletesFullSyncGatesTheOldDefect is the regression test for the bug where a
// first full sync was marked complete no matter how the run ended, so a truncated or
// failed run switched the library to incremental mode and the missing history was
// never fetched.
func TestCompletesFullSyncGatesTheOldDefect(t *testing.T) {
	tests := []struct {
		name        string
		reason      TerminationReason
		failedCount int
		want        bool
	}{
		{"reached the end cleanly", TermEndOfPages, 0, true},
		{"reached the end but a page failed", TermEndOfPages, 1, false},
		{"hit the sync cap", TermHitLimit, 0, false},
		{"stopped early on known pages", TermStoppedEarly, 0, false},
		{"cancelled", TermCancelled, 0, false},
		{"failed", TermFailed, 0, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := CompletesFullSync(tc.reason, tc.failedCount); got != tc.want {
				t.Errorf("CompletesFullSync(%q, %d) = %v, want %v", tc.reason, tc.failedCount, got, tc.want)
			}
		})
	}
}

// TestSortOrderWhitelistIsComplete guards against shrinking the whitelist. A
// backup whose sortOrder is not listed is rejected outright rather than
// defaulted, so dropping a value makes old backups unimportable.
func TestSortOrderWhitelistIsComplete(t *testing.T) {
	want := []SortOrder{SortTimeline, SortSavedDesc, SortSavedAsc, SortCreatedDesc, SortLikesDesc}
	for _, s := range want {
		if !ValidSortOrder(s) {
			t.Errorf("sort order %q must stay valid", s)
		}
	}
	if len(AllSortOrders) != len(want) {
		t.Errorf("AllSortOrders has %d entries, want %d", len(AllSortOrders), len(want))
	}
	if ValidSortOrder("author") {
		t.Error(`"author" was never a sort order; the plan listed one that does not exist`)
	}
	if ValidSortOrder("") {
		t.Error("an empty sort order must not validate")
	}
}

func TestViewModeValidation(t *testing.T) {
	if !ValidViewMode(ViewGrid) || !ValidViewMode(ViewList) {
		t.Error("grid and list must both be valid")
	}
	if ValidViewMode("masonry") || ValidViewMode("") {
		t.Error("only grid and list are valid view modes")
	}
}

func TestNormalizeListName(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Inbox", "inbox"},
		{"  Reading  ", "reading"},
		{"READING", "reading"},
		{"", ""},
		{"   ", ""},
		// Deliberately no depluralization: list names are only trimmed and
		// lowercased, unlike tag keys.
		{"Agents", "agents"},
		{"Agent", "agent"},
	}
	for _, tc := range tests {
		if got := NormalizeListName(tc.in); got != tc.want {
			t.Errorf("NormalizeListName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	// The tag canonicalizer would collapse these; the list normalizer must not.
	if CanonicalTagKey("Agents") == CanonicalTagKey("Agent") {
		t.Log("tag keys do collapse plurals, as intended")
	}
	if NormalizeListName("Agents") == NormalizeListName("Agent") {
		t.Error("list names must not depluralize: the two are different lists")
	}
}

func TestListInvariants(t *testing.T) {
	inbox := List{ID: InboxListID, Name: InboxListName}
	user := List{ID: "l1", Name: "Reading"}

	if err := ValidateListRename(inbox, "Archive"); err == nil {
		t.Error("the Inbox must not be renamed")
	}
	if err := ValidateListDelete(inbox); err == nil {
		t.Error("the Inbox must not be deleted")
	}
	if err := ValidateListRename(user, "Later"); err != nil {
		t.Errorf("a user list may be renamed: %v", err)
	}
	if err := ValidateListRename(user, "   "); err == nil {
		t.Error("a blank list name must be rejected")
	}
	if err := ValidateListRename(user, "inbox"); err == nil {
		t.Error(`"inbox" is reserved for the system list`)
	}
	if err := ValidateListDelete(user); err != nil {
		t.Errorf("a user list may be deleted: %v", err)
	}
}

func TestSuggestionIDAndStatus(t *testing.T) {
	// The id collapses names that are the same tag, so re-suggesting is a no-op
	// by primary key.
	if got, want := SuggestionID("123", "AI Agents"), "123::aiagent"; got != want {
		t.Errorf("SuggestionID = %q, want %q", got, want)
	}
	if SuggestionID("123", "AI Agents") != SuggestionID("123", "ai-agent") {
		t.Error("names that canonicalize alike must produce the same suggestion id")
	}
	if SuggestionID("123", "AI Agents") == SuggestionID("456", "AI Agents") {
		t.Error("different bookmarks must produce different suggestion ids")
	}

	pending := TagSuggestion{Status: StatusPending}
	if !pending.IsPending() || pending.IsResolved() {
		t.Error("pending must not count as resolved")
	}
	for _, s := range []SuggestionStatus{StatusAccepted, StatusRejected} {
		ts := TagSuggestion{Status: s}
		if ts.IsPending() || !ts.IsResolved() {
			t.Errorf("%q must count as resolved", s)
		}
	}
}

// TestTimestampsRoundTrip pins the ISO 8601 storage shape, so existing and newly
// written rows are byte-identical and TEXT comparison stays chronological.
func TestTimestampsRoundTrip(t *testing.T) {
	const iso = "2026-10-12T14:22:31.123Z"
	parsed := MustParseTimestamp(iso)
	if got := FormatTimestamp(parsed); got != iso {
		t.Errorf("round trip = %q, want %q", got, iso)
	}
	if got := FormatTimestamp(MustParseTimestamp("2026-10-12T14:22:31Z")); got != "2026-10-12T14:22:31.000Z" {
		t.Errorf("a timestamp without millis must format back with them, got %q", got)
	}
	if got := FormatTimestamp(MustParseTimestamp("")); got != "0001-01-01T00:00:00.000Z" {
		t.Errorf("empty parses to the zero time, formatted as %q", got)
	}
}
