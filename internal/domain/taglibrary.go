package domain

import (
	"sort"
	"strings"
)

// The tag library view.
//
// This is the tag hygiene surface: which tags are used by nothing, which are
// over-specific one-offs, which have no place in the hierarchy yet, and which could
// become a parent. It is the lever for shrinking a library that drifted past a
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
	TagFilterAll        TagLibraryFilter = "all"
	TagFilterOrphan     TagLibraryFilter = "orphan"
	TagFilterLongTail   TagLibraryFilter = "long-tail"
	TagFilterPending    TagLibraryFilter = "pending"
	TagFilterUnparented TagLibraryFilter = "unparented"
)

// AllTagLibraryFilters is the whitelist.
var AllTagLibraryFilters = []TagLibraryFilter{
	TagFilterAll, TagFilterOrphan, TagFilterLongTail, TagFilterPending, TagFilterUnparented,
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
	// ParentID and ParentName are empty when the tag is a root, and also when its
	// parent no longer exists.
	ParentID   string
	ParentName string
	// ChildCount is how many tags name this one as their parent.
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

		// A parentId pointing at a deleted tag degrades to no parent, rather than
		// rendering a dangling label.
		if parentName, exists := nameByID[tag.ParentID]; tag.ParentID != "" && exists {
			item.ParentID = tag.ParentID
			item.ParentName = parentName
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

// CountUnparentedTags counts tags that could reasonably sit under a theme.
//
// A tag that already has children is a theme itself, so it is not counted.
func CountUnparentedTags(items []TagLibraryItem) int {
	total := 0
	for _, item := range items {
		if item.ParentID == "" && item.ChildCount == 0 {
			total++
		}
	}
	return total
}

// GetRootTagCandidates lists the tags that may become a parent.
//
// Only root tags qualify, which is what keeps the taxonomy two levels deep on the
// view side as well as in the schema.
func GetRootTagCandidates(items []TagLibraryItem) []TagLibraryItem {
	candidates := make([]TagLibraryItem, 0, len(items))
	for _, item := range items {
		if item.ParentID == "" {
			candidates = append(candidates, item)
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return CompareNaturalNames(candidates[i].Name, candidates[j].Name) < 0
	})
	return candidates
}

// GetParentCandidates lists the tags a given tag could be nested under.
func GetParentCandidates(items []TagLibraryItem, tagID string) []TagLibraryItem {
	var self *TagLibraryItem
	for index := range items {
		if items[index].TagID == tagID {
			self = &items[index]
			break
		}
	}
	if self == nil || self.ChildCount > 0 {
		return []TagLibraryItem{}
	}

	candidates := make([]TagLibraryItem, 0, len(items))
	for _, candidate := range GetRootTagCandidates(items) {
		if candidate.TagID != tagID {
			candidates = append(candidates, candidate)
		}
	}
	return candidates
}

// HasParentCandidates reports whether a tag could be nested at all.
//
// It takes the shared root list rather than the whole library, so a large library
// does not rebuild the candidate list once per row.
func HasParentCandidates(rootCandidates []TagLibraryItem, item TagLibraryItem) bool {
	if item.ChildCount > 0 {
		return false
	}
	// The tag is itself in the root list while it has no parent, and it cannot be its
	// own parent.
	available := len(rootCandidates)
	if item.ParentID == "" {
		available--
	}
	return available > 0
}

// matchesTagQuery reports whether a tag matches the search box.
//
// It matches on the display name, the aliases and the parent's name, and it accepts
// either a plain substring or a canonical-key containment -- so "AI Agents" finds
// "ai agent".
func matchesTagQuery(item TagLibraryItem, searchQuery string) bool {
	trimmed := strings.TrimSpace(searchQuery)
	if trimmed == "" {
		return true
	}

	queryKey := CanonicalTagKey(trimmed)
	lowered := strings.ToLower(trimmed)

	haystack := make([]string, 0, len(item.Aliases)+2)
	haystack = append(haystack, item.Name)
	haystack = append(haystack, item.Aliases...)
	if item.ParentName != "" {
		haystack = append(haystack, item.ParentName)
	}

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
		case TagFilterUnparented:
			if item.ParentID != "" || item.ChildCount > 0 {
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
