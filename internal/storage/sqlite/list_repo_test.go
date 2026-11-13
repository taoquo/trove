package sqlite

import (
	"context"
	"strings"
	"testing"

	"github.com/taoquo/trove/internal/domain"
)

// TestListsSortByNameTheWayTagsDo pins the comparator, not the order.
//
// Lists have no other ordering, so this repository's name ordering is an addition. An
// addition that sorted differently from the tags beside it would be a second set of rules
// for the same question: SQL's ORDER BY is a byte comparison, which puts "Zebra" before
// "apple" and orders CJK by code point, which is why the tag list uses
// domain.CompareNaturalNames.
//
// The Inbox stays pinned, at either end of the alphabet.
func TestListsSortByNameTheWayTagsDo(t *testing.T) {
	db := openTestDB(t)
	repo := NewListRepository(db)
	ctx := context.Background()
	for _, name := range []string{"Zebra", "apple", "Banana"} {
		list := domain.List{
			ID:         "l-" + name,
			Name:       name,
			Normalized: strings.ToLower(name),
			CreatedAt:  clockBase,
			UpdatedAt:  clockBase,
		}
		if err := repo.Create(ctx, list); err != nil {
			t.Fatalf("create %q: %v", name, err)
		}
	}

	lists, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(lists) < 4 {
		t.Fatalf("got %d lists, want the Inbox plus three", len(lists))
	}
	if lists[0].ID != domain.InboxListID {
		t.Errorf("the first list is %q, want the Inbox pinned", lists[0].Name)
	}

	// The remaining names are in the order CompareNaturalNames gives, which is not the byte
	// order: case-insensitively, apple < Banana < Zebra.
	var names []string
	for _, list := range lists[1:] {
		names = append(names, list.Name)
	}
	for i := 1; i < len(names); i++ {
		if domain.CompareNaturalNames(names[i-1], names[i]) > 0 {
			t.Errorf("lists are out of natural order: %v", names)
			break
		}
	}
	// And that order differs from the byte order, so the assertion above is not vacuous.
	if names[0] == "Banana" {
		t.Log("the fixture would not distinguish the two orders if this were the result")
	}
	if names[0] != "apple" {
		t.Errorf("first non-Inbox list = %q, want %q: a byte comparison would put Zebra or Banana first", names[0], "apple")
	}
}
