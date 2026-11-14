package ui

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/i18n"
	"github.com/taoquo/trove/internal/storage/sqlite"
)

// The sidebar's tag search, which promised more than it did.
//
// The field's placeholder is "Search tag name or alias", and the tag library matches names and
// aliases through one shared matcher. A sidebar that runs its own name-only loop finds a tag on
// one screen and nothing on the other, and an empty result says nothing about why.

// seedTagWithAliases adds a tag row the summaries query will return, aliases included.
func seedTagWithAliases(t *testing.T, db *sqlite.DB, name string, aliases ...string) {
	t.Helper()
	tag := domain.Tag{
		ID: name, Name: name, CanonicalKey: domain.CanonicalTagKey(name),
		Aliases:   aliases,
		CreatedAt: domain.FormatTimestamp(clockBase), UpdatedAt: domain.FormatTimestamp(clockBase),
	}
	if err := sqlite.NewTagRepository(db).Create(context.Background(), tag); err != nil {
		t.Fatalf("create tag %q: %v", name, err)
	}
}

func TestTheSidebarTagSearchFindsAliases(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "post 1", clockBase)
	seedTagWithAliases(t, db, "rust", "rs", "systems language")
	seedTagWithAliases(t, db, "design")
	shell.Reload(context.Background())

	shell.state.SidebarTagQuery = "rs"
	names := sidebarTagNames(shell.visibleSidebarTags())
	if len(names) != 1 || names[0] != "rust" {
		t.Errorf("searching %q found %v: the sidebar ignores the aliases its placeholder promises",
			"rs", names)
	}

	// The library's own matcher is the one both boxes now use, so the same query must agree.
	shell.state.SidebarTagQuery = "systems"
	if names := sidebarTagNames(shell.visibleSidebarTags()); len(names) != 1 || names[0] != "rust" {
		t.Errorf("searching an alias by a word inside it found %v, want rust", names)
	}
}

func sidebarTagNames(tags []domain.TagSummary) []string {
	names := make([]string, 0, len(tags))
	for _, tag := range tags {
		names = append(names, tag.Name)
	}
	return names
}

// TestTheSidebarSaysWhenASearchMatchesNoTag covers the other half: the empty list.
//
// With a query that excludes every tag the sidebar drew an empty column -- no rows, no
// sentence, nothing to tell "the library has no tags" from "the search hid them all". The
// authors tab has answered that since it got its own field.
func TestTheSidebarSaysWhenASearchMatchesNoTag(t *testing.T) {
	shell, db := newShell(t)
	seed(t, db, "1", "ada", "post 1", clockBase)
	seedTagWithAliases(t, db, "rust")
	shell.SetLocale(i18n.EN)
	shell.Reload(context.Background())

	shell.state.SidebarTagQuery = "zzz-no-such-tag"
	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	if !tester.HasText(shell.t().T(i18n.SidebarNoTagsMatch)) {
		t.Errorf("a search that matches no tag says nothing; texts: %q", tester.Texts())
	}
	if tester.HasText(shell.t().T(i18n.SidebarNoTagsCreated)) {
		t.Errorf("a library with tags reports that no tags were created; texts: %q", tester.Texts())
	}
}
