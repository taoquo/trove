package domain

// Tag is a user-visible label in the library.
//
// See docs/sqlite-schema.md §4.3 and docs/domain-migration-map.md §3.5.
type Tag struct {
	ID   string
	Name string
	// CanonicalKey is CanonicalTagKey(Name). It is derived, never assigned from
	// input: it carries the depluralization rules described in tagkey.go, so it
	// is what enforces "these two names are the same tag". It is stored (and
	// uniquely indexed) rather than recomputed on read because SQLite has no
	// equivalent function.
	CanonicalKey string
	// Aliases records names this tag absorbed through a merge, so a later
	// suggestion of an absorbed name resolves here instead of minting a
	// near-duplicate.
	Aliases []string
	// ParentID is the owning tag for the two-level taxonomy. Empty means no
	// parent; storage writes SQL NULL for it, because an empty string would
	// violate the self-referencing foreign key.
	ParentID  string
	CreatedAt string
	UpdatedAt string
}

// TagHierarchyMaxDepth is the deepest allowed taxonomy. The old extension capped
// it at two levels: a tag may have a parent, but that parent may not have one.
// docs/sqlite-schema.md §4.3 enforces it with triggers plus an application-level
// check for the case triggers cannot see.
const TagHierarchyMaxDepth = 2

// ValidateTagHierarchy reports whether assigning parentID to the tag identified
// by id is legal given the current library.
//
// It covers the path the SQL triggers cannot: giving a parent to a tag that
// already has children would push those children to a third level.
func ValidateTagHierarchy(tags []Tag, id, parentID string) error {
	if parentID == "" {
		return nil
	}
	if parentID == id {
		return &HierarchyError{Reason: "a tag cannot be its own parent"}
	}

	byID := make(map[string]Tag, len(tags))
	for _, tag := range tags {
		byID[tag.ID] = tag
	}

	parent, ok := byID[parentID]
	if !ok {
		return &HierarchyError{Reason: "parent tag does not exist"}
	}
	if parent.ParentID != "" {
		return &HierarchyError{Reason: "the taxonomy is at most two levels deep"}
	}

	// The tag being re-parented must not already have children.
	for _, tag := range tags {
		if tag.ParentID == id {
			return &HierarchyError{Reason: "a tag with children cannot become a child"}
		}
	}

	return nil
}

// HierarchyError is returned for an illegal tag parent assignment.
type HierarchyError struct{ Reason string }

func (e *HierarchyError) Error() string { return "tag hierarchy: " + e.Reason }
