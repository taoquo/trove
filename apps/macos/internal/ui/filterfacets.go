package ui

import (
	"context"

	"github.com/taoquo/trove/apps/macos/internal/domain"
	"github.com/taoquo/trove/apps/macos/internal/i18n"
)

// The filter model.
//
// A view is a scope plus a set of conditions. The scope says which container is being
// read -- all bookmarks, the inbox, one list -- and exactly one is active. A condition
// narrows inside that scope: the switches stack (media and longform can both be on), and
// each facet holds one value -- one tag, one author. Clicking a row shows that row;
// clicking another in the same facet replaces it, so the list is never a query the reader
// has to work out.
//
// Everything that narrows the list is described by one of the two tables below:
// filterFacets for multi-valued conditions (tags, authors) and filterFlags for switches
// (media, longform, pending suggestions). The toolbar's chips, the badge beside the
// filter control, "clear all" and the tests all read those tables. Adding a condition is
// therefore one row here plus its compilation in state.filter -- not another field
// threaded through the sidebar, the toolbar and the chip list separately.

// filterFacet is one single-valued condition.
//
// A facet owns how its value is read, toggled and cleared, and how it is named for a
// chip. The naming function is not decoration: a tag is stored as a uuid and an author as
// a handle, and a chip that shows either of those is unreadable.
type filterFacet struct {
	// key identifies the facet in tests and in diagnostics.
	key string
	// values are the values currently narrowing the list, in the order they were added.
	values func(*Shell) []string
	// toggle adds or removes one value, and reloads.
	toggle func(*Shell, context.Context, string)
	// clear removes every value. It does not reload.
	clear func(*Shell)
	// name turns a stored value into the words a chip shows.
	name func(*Shell, string) string
}

// filterFlag is one on/off condition.
type filterFlag struct {
	// key is the translated name the chip shows.
	key i18n.Key
	on  func(*Shell) bool
	// set turns the flag on or off. It does not reload.
	set func(*Shell, bool)
}

// filterCondition is one active narrowing, as a chip shows and removes it.
type filterCondition struct {
	// Facet is the facet's key, empty for a flag.
	Facet string
	// Value is the stored value, empty for a flag.
	Value string
	// Label is what the chip says. It is never the stored value.
	Label  string
	Remove func()
}

// filterFacets lists the multi-valued conditions, in the order the chips show them.
func (s *Shell) filterFacets() []filterFacet {
	return []filterFacet{
		{
			key:    "tag",
			values: func(shell *Shell) []string { return shell.state.ActiveTagIDs },
			toggle: func(shell *Shell, ctx context.Context, id string) { shell.toggleTagFilter(ctx, id) },
			clear:  func(shell *Shell) { shell.state.ActiveTagIDs = nil },
			name:   func(shell *Shell, id string) string { return shell.tagLabel(id) },
		},
		{
			key:    "author",
			values: func(shell *Shell) []string { return shell.state.ActiveAuthors },
			toggle: func(shell *Shell, ctx context.Context, handle string) { shell.toggleAuthorFilter(ctx, handle) },
			clear:  func(shell *Shell) { shell.state.ActiveAuthors = nil },
			name:   func(shell *Shell, handle string) string { return shell.authorLabel(handle) },
		},
	}
}

// filterFlags lists the on/off conditions, in the order the chips show them.
func (s *Shell) filterFlags() []filterFlag {
	return []filterFlag{
		{
			key: i18n.FilterHasMedia,
			on:  func(shell *Shell) bool { return shell.state.OnlyWithMedia },
			set: func(shell *Shell, on bool) { shell.state.OnlyWithMedia = on },
		},
		{
			key: i18n.SuggestionsPendingTitle,
			on:  func(shell *Shell) bool { return shell.state.OnlyPendingSuggestions },
			set: func(shell *Shell, on bool) { shell.state.OnlyPendingSuggestions = on },
		},
		{
			key: i18n.FilterLongform,
			on:  func(shell *Shell) bool { return shell.state.OnlyLongform },
			set: func(shell *Shell, on bool) { shell.state.OnlyLongform = on },
		},
	}
}

// activeConditions lists every narrowing in effect, one per chip.
//
// The search box is deliberately absent: it is a visible control the user clears where
// they typed it, and repeating it as a chip would give two places to clear one thing.
// The scope is absent for a different reason -- it is navigation, not a condition, and a
// chip would suggest these can be stacked.
func (s *Shell) activeConditions() []filterCondition {
	ctx := context.Background()
	conditions := make([]filterCondition, 0, 4)
	for _, facet := range s.filterFacets() {
		facet := facet
		for _, value := range facet.values(s) {
			value := value
			conditions = append(conditions, filterCondition{
				Facet:  facet.key,
				Value:  value,
				Label:  facet.name(s, value),
				Remove: func() { facet.toggle(s, ctx, value) },
			})
		}
	}
	return append(conditions, s.activeFlagConditions()...)
}

// activeFlagConditions lists the on/off conditions that are on.
func (s *Shell) activeFlagConditions() []filterCondition {
	ctx := context.Background()
	conditions := make([]filterCondition, 0, 3)
	for _, flag := range s.filterFlags() {
		flag := flag
		if !flag.on(s) {
			continue
		}
		conditions = append(conditions, filterCondition{
			Label: s.t().T(flag.key),
			Remove: func() {
				flag.set(s, false)
				s.ReloadAsync(ctx)
			},
		})
	}
	return conditions
}

// conditionCount is how many conditions are narrowing the list, for the badge.
func (s *Shell) conditionCount() int { return len(s.activeConditions()) }

// clearConditions removes every condition and reloads once.
//
// The scope is left alone: clearing filters is not leaving the container the user is
// reading. The search box is left alone too, for the same reason a chip does not
// represent it -- it is cleared where it was typed.
func (s *Shell) clearConditions(ctx context.Context) {
	for _, facet := range s.filterFacets() {
		facet.clear(s)
	}
	for _, flag := range s.filterFlags() {
		flag.set(s, false)
	}
	s.ReloadAsync(ctx)
}

// tagLabel is the tag's display name, for a chip.
//
// The sidebar's own list is read first, then the tag library, which is where a tag can
// still have a name while the sidebar is showing another scope.
//
// The id fallback should be unreachable: a condition naming a tag the library does not hold is
// dropped rather than drawn (filterTagConditions, reload.go), because a chip that shows the
// uuid is not a name and the filter behind it matches nothing. It stays as the last resort for
// the one frame between a deletion and the read that notices it.
func (s *Shell) tagLabel(id string) string {
	for _, tag := range s.state.Data.Tags {
		if tag.ID == id {
			return tag.Name
		}
	}
	if name := s.tagName(id); name != "" {
		return name
	}
	return id
}

// authorLabel is the author's display name, or the handle when there is none.
//
// The "@" is not decoration: a tag and an author are different kinds of condition, and a
// chip reading "泊舟" beside a chip reading "AI" leaves the reader to guess which is which.
// The reference's own author rows use the same marker (the handle line under the name).
func (s *Shell) authorLabel(handle string) string {
	for _, author := range s.state.Data.Authors {
		if author.AuthorHandle != handle {
			continue
		}
		if author.AuthorName != "" && author.AuthorName != handle {
			return "@" + author.AuthorName
		}
		return "@" + handle
	}
	return "@" + handle
}

// tagCount is how many bookmarks in the current result carry a tag.
//
// It is not the tag's global total: with a filter on, the number beside a tag is what
// clicking it intersects with, or the sidebar would keep promising rows the list no longer
// shows. A view built without facet counts falls back to the summary's own total rather
// than claiming zero.
func (s *Shell) tagCount(tag domain.TagSummary) int {
	if s.state.Data.FacetCounts.Ready {
		return s.state.Data.FacetCounts.Tags[tag.ID]
	}
	return tag.BookmarkCount
}

// authorCount is tagCount for an author row.
func (s *Shell) authorCount(author domain.AuthorSummary) int {
	if s.state.Data.FacetCounts.Ready {
		return s.state.Data.FacetCounts.Authors[author.AuthorHandle]
	}
	return author.BookmarkCount
}

// tagActive reports whether a tag is the condition in effect.
func (s *Shell) tagActive(id string) bool {
	return len(s.state.ActiveTagIDs) == 1 && s.state.ActiveTagIDs[0] == id
}

// authorActive reports whether an author is the condition in effect.
func (s *Shell) authorActive(handle string) bool {
	for _, selected := range s.state.ActiveAuthors {
		if selected == handle {
			return true
		}
	}
	return false
}
