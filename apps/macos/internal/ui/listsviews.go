package ui

import (
	"context"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/apps/macos/internal/domain"
	"github.com/taoquo/trove/apps/macos/internal/i18n"
)

// The list and saved-view operations, rebuilt here in round 472.
//
// The sidebar stopped drawing those two sections -- the reference picture and the source have only
// 标签 and 作者 -- so the blocks that called these went with them. The operations themselves are the
// data layer's face and their tests are the contract they have to keep, so they live on in this
// file rather than being deleted with the drawing code.
//
// What is gone is the entry point: nothing in the interface calls these now. tools/uncalled.py will
// say so, and that is correct rather than a defect to hide -- it is the same shape as the missing
// sidebar sections themselves. When a home is found for them (a sheet, a menu with a field, a
// settings row), the methods are here and tested.

// CreateListFromInput creates a list from the sidebar's new-list field.
//
// A blank name creates nothing, which is the service's rule as much as this one's: the field is
// trimmed here so a name of spaces never reaches the repository.
func (s *Shell) CreateListFromInput(ctx context.Context) {
	name := strings.TrimSpace(s.state.NewListName)
	if name == "" || s.service == nil {
		return
	}
	if _, err := s.service.CreateList(ctx, name); err != nil {
		s.reportErr(err)
		return
	}
	s.state.NewListName = ""
	s.ReloadAsync(ctx)
}

// AskDeleteList opens the confirmation for a list.
//
// It asks rather than deleting: the dialog's answer is the only thing that runs the delete, which
// is the rule the other danger actions follow too.
func (s *Shell) AskDeleteList(id string) {
	// The Inbox is where unsorted bookmarks land, so it cannot stop existing.
	if id == domain.InboxListID {
		return
	}
	s.state.ConfirmDeleteList = id
	if s.window != nil {
		s.window.Invalidate()
	}
}

// BeginRenameList starts renaming a list, seeding the draft with its current name.
func (s *Shell) BeginRenameList(_ context.Context, id, name string) {
	s.state.RenameListID = id
	s.state.RenameListName = name
	if s.window != nil {
		s.window.Invalidate()
	}
}

// CancelRenameList abandons the rename and leaves the list alone.
func (s *Shell) CancelRenameList() {
	s.state.RenameListID = ""
	s.state.RenameListName = ""
	if s.window != nil {
		s.window.Invalidate()
	}
}

// RenameListFromInput commits the draft name.
//
// A blank draft does nothing rather than renaming the list to nothing, and the Inbox is refused by
// the service -- the Inbox is where unsorted bookmarks land, so it cannot stop existing.
func (s *Shell) RenameListFromInput(ctx context.Context) {
	id := s.state.RenameListID
	name := strings.TrimSpace(s.state.RenameListName)
	if id == "" || name == "" || s.service == nil {
		return
	}
	if _, err := s.service.RenameList(ctx, id, name); err != nil {
		s.reportErr(err)
		return
	}
	s.CancelRenameList()
	s.ReloadAsync(ctx)
}

// SaveCurrentView stores the current filters under the field's name.
//
// An empty field falls back to the source's own default name rather than refusing: the source
// writes DEFAULT_SAVED_VIEW_NAME, which is the literal "Saved view".
func (s *Shell) SaveCurrentView(ctx context.Context) {
	if s.service == nil {
		return
	}
	name := strings.TrimSpace(s.state.SavedViewName)
	if name == "" {
		// The source's DEFAULT_SAVED_VIEW_NAME (OptionsApp.tsx:556) is a translation key here,
		// as every string is. The literal "Saved view" this used to write put English into a
		// Chinese row and left the key undrawn.
		name = s.t().T(i18n.DefaultSavedViewName)
	}
	snapshot := s.currentViewSnapshot()
	if err := s.service.AddSavedView(ctx, s.service.NewSavedView(name, snapshot)); err != nil {
		s.reportErr(err)
		return
	}
	s.state.SavedViewName = ""
	s.ReloadAsync(ctx)
}

// ApplySavedView restores every field a stored view holds.
//
// It leaves the settings page the way OpenList does. The rows are in settings (round 472), so
// without this the sheet stayed over the results the view had just restored and the only visible
// change was on the page behind it -- the press looked like a no-op.
func (s *Shell) ApplySavedView(ctx context.Context, view domain.SavedView) {
	s.closePanels()
	s.state.TagMode = false
	s.state.GraphMode = false
	s.state.Query = view.Query
	s.state.OnlyWithMedia = view.OnlyWithMedia
	s.state.OnlyLongform = view.OnlyLongform
	s.state.Sort = view.SortOrder
	s.state.ViewMode = view.ViewMode
	// One value per facet, which is the model this build uses deliberately: the user asked
	// for "click one row, see that row" over the extension's AND set
	// (docs/filter-interaction-plan.md §10.4). A view saved while several values were
	// selected comes back as its first rather than as an intersection the reader cannot see.
	s.state.ActiveTagIDs = firstValue(view.ActiveTagIDs)
	s.state.ActiveAuthors = firstValue(view.ActiveAuthorHandles)
	// The bulk selection belonged to the filters that were on screen, not to the ones coming back.
	s.state.SelectedIDs = nil
	s.state.Selected = ""
	s.ReloadAsync(ctx)
}

// DeleteSavedView removes a stored view.
func (s *Shell) DeleteSavedView(ctx context.Context, id string) {
	if s.service == nil {
		return
	}
	if err := s.service.RemoveSavedView(ctx, id); err != nil {
		s.reportErr(err)
		return
	}
	s.ReloadAsync(ctx)
}

// OpenScope leaves settings and shows one of the fixed scopes.
func (s *Shell) OpenScope(ctx context.Context, scope string) {
	s.closePanels()
	s.selectSidebar(ctx, scope)
	s.invalidate()
}

// OpenList shows one list's bookmarks in the results pane.
//
// Lists could be created and deleted but never read: the sidebar that used to carry them was
// removed in favour of the reference picture's two tabs, and the settings section only manages
// names. This closes the sheet and puts the list in the scope, which is what a list is for.
func (s *Shell) OpenList(ctx context.Context, listID string) {
	if listID == "" {
		return
	}
	s.closePanels()
	s.state.Selection = prefixList + listID
	s.state.TagMode = false
	s.state.GraphMode = false
	s.ReloadAsync(ctx)
	s.invalidate()
}

// firstValue keeps the first selection, if any.
func firstValue(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	return []string{values[0]}
}

// settingsListsViewsSection is the 列表与视图 tab: the two management cards and nothing else.
//
// Round 26 gave them their own tab. They had been drawn at the end of 数据安全, under the
// sentence about backups staying on this Mac -- a place a reader looking for a saved view has no
// reason to open, and a section whose name describes neither card.
func (s *Shell) settingsListsViewsSection(c *ui.Context, theme *ui.Theme, rail RailTokens) int {
	return s.listsAndViewsSettings(c, theme, rail)
}

// listsAndViewsSettings draws the two management sections the settings panel now carries.
//
// Round 472 removed the sidebar's versions of these, because the reference picture's sidebar is
// only 标签 and 作者. That left the operations with no caller and their wording with no renderer,
// which tools/uncalled.py and the orphan-key check both said out loud. Settings is where a
// management list belongs in a macOS window: it is configuration, not navigation, and it keeps the
// sidebar matching the picture without withdrawing a feature.
func (s *Shell) listsAndViewsSettings(c *ui.Context, theme *ui.Theme, rail RailTokens) int {
	// settingsNameField is the small name input a row's control can carry.
	nameField := func(value *string, placeholder i18n.Key) {
		ui.TextInput(c, value).Width(SettingsInputWidth).Height(SettingsFieldHeight).
			Padding(0, SettingsFieldPadX).Radius(SettingsFieldRadius).
			FontSize(SettingsFieldFont).Placeholder(s.t().T(placeholder)).
			Label(s.t().T(placeholder))
	}

	drawn := 0
	if s.settingsShows(i18n.SidebarLists, i18n.SidebarUnclassified, i18n.SidebarNoLists,
		i18n.SidebarNewList, i18n.SettingsNewList, i18n.GraphOpenBookmarks,
		i18n.RenameListLabel, i18n.ActionAdd, i18n.ActionDelete) {
		settingsSectionLabel(c, theme, rail, s.t().T(i18n.SidebarLists))
		items := []settingsItem{{
			// The two scopes the sidebar no longer draws, kept reachable where the lists
			// live: the sidebar is the reference picture's two tabs, but "unclassified" is
			// still a place a reader needs to get to.
			keys: []i18n.Key{i18n.SidebarUnclassified, i18n.SettingsUnclassifiedHint},
			name: s.t().T(i18n.SidebarUnclassified),
			sub:  s.t().T(i18n.SettingsUnclassifiedHint),
			control: func() {
				settingsIcon(c, theme, rail, "list",
					s.t().Tf(i18n.ActionOpenNamed, s.t().T(i18n.SidebarUnclassified)), func() {
						s.OpenScope(context.Background(), selectionUnclassified)
					})
			},
		}}
		if len(s.state.Data.Lists) == 0 {
			items = append(items, settingsItem{keys: []i18n.Key{i18n.SidebarNoLists},
				name: s.t().T(i18n.SidebarNoLists)})
		}
		for _, list := range s.state.Data.Lists {
			list := list
			if s.state.RenameListID == list.ID {
				items = append(items, settingsItem{
					keys: []i18n.Key{i18n.RenameListLabel},
					name: list.Name,
					control: func() {
						ui.Row(c).Shrink(0).GapX(SpaceSmall).AlignItems(ui.Center).Children(func() {
							nameField(&s.state.RenameListName, i18n.SidebarNewList)
							settingsAction(c, theme, rail, s.t().T(i18n.ActionSave), false, false, func() {
								s.RenameListFromInput(context.Background())
							})
							settingsAction(c, theme, rail, s.t().T(i18n.ActionCancel), false, false, func() {
								s.CancelRenameList()
							})
						})
					},
				})
				continue
			}
			// The Inbox is the one list with a rule of its own, so it is the one row
			// that says so; another list's name is all there is to say about it.
			sub := ""
			keys := []i18n.Key{i18n.GraphOpenBookmarks}
			if list.ID == domain.InboxListID {
				sub = s.t().T(i18n.SettingsInboxHint)
				keys = append(keys, i18n.SettingsInboxHint)
			}
			items = append(items, settingsItem{
				keys: keys,
				name: list.Name,
				sub:  sub,
				control: func() {
					ui.Row(c).Shrink(0).GapX(SpaceTiny).AlignItems(ui.Center).Children(func() {
						// Every list, the Inbox included, can be read. The section used to
						// hide the Inbox entirely and offer no way into any other list.
						settingsIcon(c, theme, rail, "bookmark",
							s.t().Tf(i18n.ActionOpenNamed, list.Name), func() {
								s.OpenList(context.Background(), list.ID)
							})
						// The Inbox is where unsorted bookmarks land: it is not renamed and
						// not deleted.
						if list.ID == domain.InboxListID {
							return
						}
						settingsIcon(c, theme, rail, "edit",
							s.t().Tf(i18n.ActionRenameNamed, list.Name), func() {
								s.BeginRenameList(context.Background(), list.ID, list.Name)
							})
						settingsIcon(c, theme, rail, "trash",
							s.t().Tf(i18n.ActionDeleteNamed, list.Name), func() {
								s.AskDeleteList(list.ID)
							})
					})
				},
			})
		}
		items = append(items, settingsItem{
			keys: []i18n.Key{i18n.SidebarNewList, i18n.ActionAdd},
			name: s.t().T(i18n.SettingsNewList),
			control: func() {
				ui.Row(c).Shrink(0).GapX(SpaceSmall).AlignItems(ui.Center).Children(func() {
					nameField(&s.state.NewListName, i18n.SidebarNewList)
					settingsAction(c, theme, rail, s.t().T(i18n.ActionAdd), false, false, func() {
						s.CreateListFromInput(context.Background())
					})
				})
			},
		})
		drawn += s.settingsCard(c, theme, rail, items...)
	}

	if s.settingsShows(i18n.SavedViewsTitle, i18n.SavedViewNamePlaceholder, i18n.SavedViewApply,
		i18n.SaveCurrentView, i18n.DeleteSavedView, i18n.SettingsNewView) {
		settingsSectionLabel(c, theme, rail, s.t().T(i18n.SavedViewsTitle))
		items := make([]settingsItem, 0, len(s.state.SavedViews)+1)
		for _, view := range s.state.SavedViews {
			view := view
			items = append(items, settingsItem{
				keys: []i18n.Key{i18n.SavedViewsTitle},
				name: view.Name,
				control: func() {
					ui.Row(c).Shrink(0).GapX(SpaceTiny).AlignItems(ui.Center).Children(func() {
						settingsIcon(c, theme, rail, "play",
							s.t().Tf(i18n.SavedViewApplyNamed, view.Name), func() {
								s.ApplySavedView(context.Background(), view)
							})
						settingsIcon(c, theme, rail, "trash",
							s.t().Tf(i18n.ActionDeleteNamed, view.Name), func() {
								s.DeleteSavedView(context.Background(), view.ID)
							})
					})
				},
			})
		}
		items = append(items, settingsItem{
			keys: []i18n.Key{i18n.SavedViewNamePlaceholder, i18n.SaveCurrentView},
			name: s.t().T(i18n.SettingsNewView),
			control: func() {
				ui.Row(c).Shrink(0).GapX(SpaceSmall).AlignItems(ui.Center).Children(func() {
					nameField(&s.state.SavedViewName, i18n.SavedViewNamePlaceholder)
					settingsAction(c, theme, rail, s.t().T(i18n.SaveCurrentView), false, false, func() {
						s.SaveCurrentView(context.Background())
					})
				})
			},
		})
		drawn += s.settingsCard(c, theme, rail, items...)
	}
	return drawn
}
