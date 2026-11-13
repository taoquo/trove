package domain

import (
	"errors"
	"time"
)

// The Inbox is the library's only system list. It is identified by this fixed
// id rather than by a "kind" column: a restored backup's Inbox stays recognizable,
// and it makes "exactly one Inbox" a property of the primary key instead of
// something that needs a partial unique index to enforce.
const (
	InboxListID   = "list-inbox"
	InboxListName = "Inbox"
)

// List is a user-created grouping of bookmarks.
type List struct {
	ID string
	// Name is the display name, keeping the user's casing and spacing.
	Name string
	// Normalized is the de-duplication key. It is deliberately *weaker* than a
	// tag's canonical key: list names are compared with trim + lowercase only,
	// with no depluralization.
	Normalized string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// IsInbox reports whether this is the system list, which cannot be renamed or
// deleted.
func (l List) IsInbox() bool { return l.ID == InboxListID }

// NormalizeListName produces the de-duplication key for a list name.
//
// The key is locale-independent. A host-locale lowercase would make the key depend
// on the machine's locale — under a Turkish locale "I" lowercases to a dotless "ı",
// so the same library would de-duplicate differently on different machines. This
// uses a fixed mapping instead: deterministic, and the divergence only shows up for
// list names differing solely in Turkish dotted/dotless i.
func NormalizeListName(name string) string {
	return lowerTagText(trimTagSpace(name))
}

// ValidateListRename rejects a rename that would break a system-list invariant.
func ValidateListRename(l List, newName string) error {
	if l.IsInbox() {
		return &ListError{Reason: "the Inbox cannot be renamed"}
	}
	if NormalizeListName(newName) == "" {
		return &ListError{Reason: "a list name cannot be blank"}
	}
	if NormalizeListName(newName) == NormalizeListName(InboxListName) {
		return &ListError{Reason: `"Inbox" is reserved for the system list`}
	}
	return nil
}

// ValidateListDelete rejects deleting the system list.
func ValidateListDelete(l List) error {
	if l.IsInbox() {
		return &ListError{Reason: "the Inbox cannot be deleted"}
	}
	return nil
}

// ErrListNotEmpty reports a delete refused because bookmarks are still in the
// list.
//
// It exists so the caller can tell "you must move the bookmarks first" apart from
// a genuine storage failure rather than guess.
var ErrListNotEmpty = errors.New("list: the list still contains bookmarks")

// ListError is returned for an illegal list operation.
type ListError struct{ Reason string }

func (e *ListError) Error() string { return "list: " + e.Reason }
