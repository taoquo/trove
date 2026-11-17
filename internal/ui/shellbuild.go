package ui

import (
	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/i18n"
	"github.com/taoquo/trove/internal/usecase/workspace"
)

// NewShell builds the shell.
func NewShell(service *workspace.Service, appearance Appearance) *Shell {
	shell := &Shell{
		service:    service,
		translator: i18n.New(""),
		updates:    defaultUpdates(),
		state: &State{
			Appearance: appearance,
			// Open on the grid.
			ViewMode:      domain.ViewGrid,
			LightboxIndex: -1,
			// The first page: 80 cards, then 80 per scroll.
			RenderLimit: ResultsRenderLimit,
			// -1, not 0: both anchors are compared with a scroll offset, and 0 is the top
			// of the list -- a real place where the first automatic page must be allowed.
			autoLoadY:   -1,
			renderGrewY: -1,
			// The tags tab is the one the window opens on: it is the first of the two
			// facets and the one a new library fills first. The authors tab keeps its own
			// search text and scroll position while it is away.
			SidebarFacetTab: sidebarTabTags,
			// The default width, applied here as well as on load because zero is not a width:
			// the split's own 40-point minimum is what a zero leaves, so a preferences read that
			// fails would open the app with a clipped sidebar rather than the default one.
			// ApplyPreferences overwrites this when it has a stored value, which matters more
			// than the default does.
			LeftSidebarWidth: WidthSidebarDef,
			// -1 is "nothing chosen", which is how ListState carries it. Its zero value is 0,
			// so leaving it unset opened the window with the first bookmark looking chosen:
			// the list drew its selected state on row 0 and the inspector would have followed
			// it on the first click anywhere.
			ListSelected: -1,
			Selection:    selectionAll,
			Sort:         domain.SortTimeline,
		},
		// The whitelist the ORDER BY is chosen from. It holds every order the query
		// layer accepts, including one the cycle does not reach.
		sortOptions: []domain.SortOrder{
			domain.SortTimeline,
			domain.SortSavedDesc,
			domain.SortSavedAsc,
			domain.SortCreatedDesc,
			domain.SortLikesDesc,
		},
		// The cycle the sort button walks. The sequence has four entries and leaves
		// `saved-desc` out: it is supported by the query layer and by a saved view's stored
		// order, but the button never reaches it.
		sortCycle: []domain.SortOrder{
			domain.SortTimeline,
			domain.SortSavedAsc,
			domain.SortCreatedDesc,
			domain.SortLikesDesc,
		},
	}
	// Both batch pickers show a word before anything is chosen. Their bound values start
	// empty, and an empty value would draw a bare chevron with no way to tell the tag picker
	// from the list one.
	shell.syncBatchLabels()
	return shell
}

// sortKeys maps each order to its message key.
//
// The five orders are all reachable here.
// sortLabelKeys maps an order to its label.
//
// Two orders share the "recently saved" copy: `timeline` and `saved-desc`. That is
// deliberate, not an oversight: the switch falls through to the same copy.

// State exposes the state for tests.
func (s *Shell) State() *State { return s.state }
