package domain

import (
	"sort"
	"strings"
)

// The tag library view.
//
// This is the tag hygiene surface: which tags are used by nothing and which are
// over-specific one-offs. It is the lever for shrinking a library that drifted past a
// workable size, so the rules are exact rather than approximated.

// LongTailMaxBookmarks is the threshold: a tag used by one or two bookmarks
// is almost always an over-specific label that belongs inside a broader theme.
const LongTailMaxBookmarks = 2

// DefaultSidebarTagCount is how many sidebar rows show before the toggle.
const DefaultSidebarTagCount = 8

// TagLibraryFilter narrows the tag library.
type TagLibraryFilter string

// The filters.
const (
	TagFilterAll      TagLibraryFilter = "all"
	TagFilterOrphan   TagLibraryFilter = "orphan"
	TagFilterLongTail TagLibraryFilter = "long-tail"
	TagFilterPending  TagLibraryFilter = "pending"
)

// AllTagLibraryFilters is the whitelist.
var AllTagLibraryFilters = []TagLibraryFilter{
	TagFilterAll, TagFilterOrphan, TagFilterLongTail, TagFilterPending,
}

// TagLibrarySortOrder is how the library is ordered.
type TagLibrarySortOrder string

// The sort orders.
const (
	TagSortCountDesc TagLibrarySortOrder = "count-desc"
	TagSortCountAsc  TagLibrarySortOrder = "count-asc"
	TagSortNameAsc   TagLibrarySortOrder = "name-asc"
	TagSortRecent    TagLibrarySortOrder = "recent"
)

// AllTagLibrarySortOrders is the whitelist.
var AllTagLibrarySortOrders = []TagLibrarySortOrder{
	TagSortCountDesc, TagSortCountAsc, TagSortNameAsc, TagSortRecent,
}

// TagLibraryItem is one tag as the management view sees it.
type TagLibraryItem struct {
	TagID   string
	Name    string
	Aliases []string
	// ChildCount is how many tags name this one as their parent. The theme belongs to
	// the data model -- old backups and imports still carry parents -- while no screen
	// sets or shows one; the count stays because the two hygiene rules below need it.
	ChildCount int
	// BookmarkCount is how many bookmarks carry this tag.
	BookmarkCount int
	// PendingSuggestionCount is how many unresolved suggestions propose this tag
	// name. It is what keeps a sweep from deleting a tag still under review.
	PendingSuggestionCount int
	UpdatedAt              string
}

// BuildTagLibraryItems builds the view from the library.
func BuildTagLibraryItems(tags []Tag, relations []BookmarkTag, suggestions []TagSuggestion) []TagLibraryItem {
	bookmarkCountByTagID := make(map[string]int, len(tags))
	for _, relation := range relations {
		bookmarkCountByTagID[relation.TagID]++
	}

	// Suggestions are matched by name rather than by id: a suggestion proposes a tag
	// that may not exist yet, which is the whole point of it.
	pendingCountByKey := make(map[string]int)
	for _, suggestion := range suggestions {
		if suggestion.Status != StatusPending {
			continue
		}
		key := CanonicalTagKey(suggestion.TagName)
		if key == "" {
			continue
		}
		pendingCountByKey[key]++
	}

	nameByID := make(map[string]string, len(tags))
	for _, tag := range tags {
		nameByID[tag.ID] = tag.Name
	}

	childCountByID := make(map[string]int)
	for _, tag := range tags {
		// A parentId pointing at a deleted tag does not make that tag a parent.
		if tag.ParentID == "" {
			continue
		}
		if _, exists := nameByID[tag.ParentID]; !exists {
			continue
		}
		childCountByID[tag.ParentID]++
	}

	items := make([]TagLibraryItem, 0, len(tags))
	for _, tag := range tags {
		item := TagLibraryItem{
			TagID:                  tag.ID,
			Name:                   tag.Name,
			Aliases:                append([]string{}, tag.Aliases...),
			ChildCount:             childCountByID[tag.ID],
			BookmarkCount:          bookmarkCountByTagID[tag.ID],
			PendingSuggestionCount: pendingCountByKey[CanonicalTagKey(tag.Name)],
			UpdatedAt:              tag.UpdatedAt,
		}
		if item.UpdatedAt == "" {
			item.UpdatedAt = tag.CreatedAt
		}

		items = append(items, item)
	}

	return items
}

// CountOrphanTags counts tags no bookmark uses.
func CountOrphanTags(items []TagLibraryItem) int {
	total := 0
	for _, item := range items {
		if item.BookmarkCount == 0 {
			total++
		}
	}
	return total
}

// IsLongTailTag reports whether a tag is an over-specific one-off.
//
// A tag with children is exempt: it is a theme, however few bookmarks currently
// carry it directly.
func IsLongTailTag(item TagLibraryItem) bool {
	return item.BookmarkCount > 0 &&
		item.BookmarkCount <= LongTailMaxBookmarks &&
		item.ChildCount == 0
}

// CountLongTailTags counts the over-specific one-offs.
func CountLongTailTags(items []TagLibraryItem) int {
	total := 0
	for _, item := range items {
		if IsLongTailTag(item) {
			total++
		}
	}
	return total
}

// GetRemovableOrphanTagIDs lists the tags safe to sweep in one go.
//
// The pending-suggestion check is not optional. Without it a sweep deleted a tag the
// user was still reviewing, which also rejected those suggestions and added the name
// to the block list, so the concept could never be suggested again. That is a
// data-loss path, and it is why this rule is reproduced rather than simplified.
func GetRemovableOrphanTagIDs(items []TagLibraryItem) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		if item.BookmarkCount == 0 && item.ChildCount == 0 && item.PendingSuggestionCount == 0 {
			ids = append(ids, item.TagID)
		}
	}
	return ids
}

// matchesTagQuery reports whether a tag matches the search box.
//
// It matches on the display name and the aliases, and it accepts either a plain
// substring or a canonical-key containment -- so "AI Agents" finds "ai agent".
func matchesTagQuery(item TagLibraryItem, searchQuery string) bool {
	trimmed := strings.TrimSpace(searchQuery)
	if trimmed == "" {
		return true
	}

	queryKey := CanonicalTagKey(trimmed)
	lowered := strings.ToLower(trimmed)

	haystack := make([]string, 0, len(item.Aliases)+1)
	haystack = append(haystack, item.Name)
	haystack = append(haystack, item.Aliases...)

	for _, value := range haystack {
		if strings.Contains(strings.ToLower(value), lowered) {
			return true
		}
		if queryKey != "" && strings.Contains(CanonicalTagKey(value), queryKey) {
			return true
		}
	}
	return false
}

// compareTagItems orders two items.
func compareTagItems(left, right TagLibraryItem, sortOrder TagLibrarySortOrder) int {
	switch sortOrder {
	case TagSortNameAsc:
		return CompareNaturalNames(left.Name, right.Name)
	case TagSortRecent:
		// Newest first. The timestamps are fixed-width UTC, so a plain comparison is
		// chronological.
		switch {
		case left.UpdatedAt > right.UpdatedAt:
			return -1
		case left.UpdatedAt < right.UpdatedAt:
			return 1
		default:
			return 0
		}
	}

	delta := right.BookmarkCount - left.BookmarkCount
	if sortOrder == TagSortCountAsc {
		delta = left.BookmarkCount - right.BookmarkCount
	}
	if delta != 0 {
		return delta
	}
	return CompareNaturalNames(left.Name, right.Name)
}

// GetVisibleTagLibraryItems filters and sorts the library.
func GetVisibleTagLibraryItems(items []TagLibraryItem, searchQuery string, filter TagLibraryFilter, sortOrder TagLibrarySortOrder) []TagLibraryItem {
	visible := make([]TagLibraryItem, 0, len(items))
	for _, item := range items {
		switch filter {
		case TagFilterOrphan:
			if item.BookmarkCount > 0 {
				continue
			}
		case TagFilterLongTail:
			if !IsLongTailTag(item) {
				continue
			}
		case TagFilterPending:
			if item.PendingSuggestionCount == 0 {
				continue
			}
		}

		if !matchesTagQuery(item, searchQuery) {
			continue
		}
		visible = append(visible, item)
	}

	sort.SliceStable(visible, func(i, j int) bool {
		return compareTagItems(visible[i], visible[j], sortOrder) < 0
	})
	return visible
}

// FilterSidebarTags filters the sidebar's tag list.
//
// It reuses the same matcher as the library view, so typing in either search box
// behaves the same.
func FilterSidebarTags(tags []Tag, searchQuery string) []Tag {
	trimmed := strings.TrimSpace(searchQuery)
	if trimmed == "" {
		return tags
	}

	matched := make([]Tag, 0, len(tags))
	for _, tag := range tags {
		item := TagLibraryItem{TagID: tag.ID, Name: tag.Name, Aliases: tag.Aliases}
		if matchesTagQuery(item, trimmed) {
			matched = append(matched, tag)
		}
	}
	return matched
}

// SidebarTags is the sidebar's tag list plus whether to show the expander.
type SidebarTags struct {
	Items            []Tag
	ShouldShowToggle bool
}

// GetVisibleSidebarTags collapses the sidebar's tag list until asked or searched.
//
// It mirrors the author sidebar: a library with hundreds of tags must not build
// hundreds of nav rows on every workspace update.
func GetVisibleSidebarTags(tags []Tag, searchQuery string, expanded bool, defaultVisibleCount int) SidebarTags {
	if defaultVisibleCount <= 0 {
		defaultVisibleCount = DefaultSidebarTagCount
	}

	matching := FilterSidebarTags(tags, searchQuery)
	collapsed := strings.TrimSpace(searchQuery) == "" && !expanded

	items := matching
	if collapsed && len(items) > defaultVisibleCount {
		items = items[:defaultVisibleCount]
	}

	return SidebarTags{
		Items:            items,
		ShouldShowToggle: collapsed && len(matching) > defaultVisibleCount,
	}
}
