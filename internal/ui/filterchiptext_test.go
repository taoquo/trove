package ui

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/i18n"
	"github.com/taoquo/trove/internal/storage/sqlite"
)

// TestEachActiveFilterChipNamesWhatItRemoves pins the chip row's remove control to the same
// rule as the detail's tag pills: a row of identical x's has to say what each one takes away. The condition chip labelled its x with the condition itself, so a screen reader
// met the same name twice -- once as the chip and once as the button -- with no verb saying
// the second one removes it.
func TestEachActiveFilterChipNamesWhatItRemoves(t *testing.T) {
	shell, db := newShell(t)
	ctx := context.Background()
	seed(t, db, "1", "ada", "a post", clockBase)

	repo := sqlite.NewTagRepository(db)
	tag := domain.Tag{ID: "t-rust", Name: "rust", CanonicalKey: domain.CanonicalTagKey("rust")}
	tag.CreatedAt = domain.FormatTimestamp(clockBase)
	tag.UpdatedAt = domain.FormatTimestamp(clockBase)
	if err := repo.Create(ctx, tag); err != nil {
		t.Fatalf("Create tag: %v", err)
	}
	if err := repo.Attach(ctx, domain.BookmarkTag{
		BookmarkID: "1", TagID: tag.ID, CreatedAt: clockBase, Source: domain.TagSourceManual,
	}); err != nil {
		t.Fatalf("Attach: %v", err)
	}
	shell.Reload(ctx)
	shell.state.ActiveTagIDs = []string{tag.ID}

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	want := shell.t().Tf(i18n.ActionRemoveNamed, "rust")
	if _, ok := tester.Find(want); !ok {
		t.Errorf("the chip's remove control is not named %q; texts: %q", want, tester.Texts())
	}
}

// TestTheMediaChipUsesTheSameWordAsTheFilterItRepresents pins one condition to one name.
//
// The filter popover's row says "Has media" and the chip row must say the same, not "Media
// only": a second key and a second phrase for the same checkbox on the same screen.
func TestTheMediaChipUsesTheSameWordAsTheFilterItRepresents(t *testing.T) {
	shell, _ := newShell(t)
	shell.SetLocale(i18n.EN)
	shell.state.OnlyWithMedia = true

	tester := ui.NewTester(shell.View, 1180, 760)
	tester.Frame()

	if !tester.HasText(shell.t().T(i18n.FilterHasMedia)) {
		t.Errorf("the chip does not carry the popover's own word %q; texts: %q",
			shell.t().T(i18n.FilterHasMedia), tester.Texts())
	}
}
