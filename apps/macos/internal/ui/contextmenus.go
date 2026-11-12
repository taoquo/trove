package ui

import (
	"context"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/taoquo/trove/apps/macos/internal/domain"
	"github.com/taoquo/trove/apps/macos/internal/i18n"
	"github.com/taoquo/trove/apps/macos/internal/usecase/workspace"
)

// The context menus, one per kind of row.
//
// magpie puts the actions for a thing on the thing itself: a row's menu answers "what can
// I do with this unit" without moving the reader to a toolbar first, and the list's own
// menu is short because everything rare lives in a page. The right button is a second
// pointer for what the card's controls already do, which is why every item here calls an
// existing method rather than a parallel one.
//
// MyGo builds a menu's function when it opens and again in the frame after an item was
// chosen, where Chosen reports the choice (menu.go:205-210). The labels are translated at
// build time, so opening the menu after a language change shows the new words.

// bookmarkContextMenu is the menu on a bookmark card.
//
// Five actions, in the order a reader reaches for them: open it, open it on X, copy what
// it says, copy its link, tick it for a bulk action. The card's own controls do three of
// these for the *selected* card; the menu acts on the card under the pointer, which is the
// point of a context menu.
func (s *Shell) bookmarkContextMenu(c *ui.Context, m *ui.Menu, row workspace.BookmarkRow) {
	if m.Item(s.t().T(i18n.ActionOpenBookmark)).Chosen() {
		s.showDetail(row.TweetID)
	}
	open := m.Item(s.t().T(i18n.OpenOnX)).Disabled(row.TweetURL == "")
	if open.Chosen() {
		s.state.Selected = row.TweetID
		s.OpenSelectedOnX()
	}
	m.Separator()
	if m.Item(s.t().T(i18n.ActionCopyText)).Disabled(strings.TrimSpace(row.Text) == "").Chosen() {
		s.copyBookmarkText(c, row)
	}
	copyLink := m.Item(s.t().T(i18n.ActionCopyLink)).Disabled(row.TweetURL == "")
	if copyLink.Chosen() {
		s.state.Selected = row.TweetID
		s.CopySourceLink(c)
	}
	m.Separator()

	label := i18n.ActionAddToSelection
	ticked := s.state.SelectedIDs[row.TweetID]
	if ticked {
		label = i18n.ActionRemoveFromSelection
	}
	if m.Item(s.t().T(label)).Chosen() {
		s.setBatchSelected(row.TweetID, !ticked)
	}
}

// copyBookmarkText copies the post's own words, which is what a reader quoting it wants.
// The link is a separate item because the two are different things to paste.
func (s *Shell) copyBookmarkText(c clipboardWriter, row workspace.BookmarkRow) {
	if strings.TrimSpace(row.Text) == "" {
		s.ShowStatus(ToastWarn, s.t().T(i18n.ToastNoLink))
		return
	}
	c.WriteClipboard(row.Text)
	s.ShowToast(s.t().T(i18n.ToastDone))
}

// setBatchSelected ticks or unticks one bookmark for a bulk action.
func (s *Shell) setBatchSelected(id string, selected bool) {
	if id == "" {
		return
	}
	if selected {
		if s.state.SelectedIDs == nil {
			s.state.SelectedIDs = map[string]bool{}
		}
		s.state.SelectedIDs[id] = true
	} else {
		delete(s.state.SelectedIDs, id)
		if len(s.state.SelectedIDs) == 0 {
			s.state.SelectedIDs = nil
		}
	}
	s.invalidate()
}

// tagContextMenu is the menu on a sidebar tag row.
//
// Filtering and "only" are both here because the sidebar has room for one click and the
// menu for the distinction: a click adds this tag to the conditions, "only" drops the
// others. Rename and delete are the row's own two hover controls, repeated where a reader
// who right-clicks will look for them.
func (s *Shell) tagContextMenu(m *ui.Menu, tagID, label string) {
	ctx := context.Background()
	if m.Item(s.t().T(i18n.ActionFilterTag)).Chosen() {
		s.toggleTagFilter(ctx, tagID)
	}
	only := m.Item(s.t().T(i18n.ActionOnlyTag)).Disabled(s.tagActive(tagID) && len(s.state.ActiveAuthors) == 0)
	if only.Chosen() {
		s.onlyTagFilter(ctx, tagID)
	}
	m.Separator()
	if m.Item(s.t().Tf(i18n.TagRenameLabel, label)).Chosen() {
		s.BeginRenameSidebarTag(tagID)
	}
	if m.Item(s.t().Tf(i18n.ActionDelete, label)).Chosen() {
		s.AskDeleteSidebarTag(tagID)
	}
}

// onlyTagFilter narrows to one tag and drops the other conditions, which is the whole of
// "only this tag" -- keeping a second tag or an author would make the item a lie.
func (s *Shell) onlyTagFilter(ctx context.Context, tagID string) {
	if tagID == "" {
		return
	}
	s.leaveSettings()
	s.state.ActiveTagIDs = []string{tagID}
	s.state.ActiveAuthors = nil
	s.markPageChange()
	s.ReloadAsync(ctx)
}

// authorContextMenu is the menu on a sidebar author row.
func (s *Shell) authorContextMenu(c *ui.Context, m *ui.Menu, author domain.AuthorSummary) {
	ctx := context.Background()
	if m.Item(s.t().T(i18n.ActionFilterAuthor)).Chosen() {
		s.toggleAuthorFilter(ctx, author.AuthorHandle)
	}
	only := m.Item(s.t().T(i18n.ActionOnlyAuthor)).Disabled(s.authorActive(author.AuthorHandle) && len(s.state.ActiveTagIDs) == 0)
	if only.Chosen() {
		s.onlyAuthorFilter(ctx, author.AuthorHandle)
	}
	m.Separator()
	if m.Item(s.t().T(i18n.ActionCopyHandle)).Chosen() {
		if author.AuthorHandle == "" {
			s.ShowStatus(ToastWarn, s.t().T(i18n.ToastNoLink))
			return
		}
		c.WriteClipboard("@" + author.AuthorHandle)
		s.ShowToast(s.t().T(i18n.ToastDone))
	}
}

// onlyAuthorFilter narrows to one author and drops the tag conditions.
func (s *Shell) onlyAuthorFilter(ctx context.Context, handle string) {
	if handle == "" {
		return
	}
	s.leaveSettings()
	s.state.ActiveAuthors = []string{handle}
	s.state.ActiveTagIDs = nil
	s.markPageChange()
	s.ReloadAsync(ctx)
}
