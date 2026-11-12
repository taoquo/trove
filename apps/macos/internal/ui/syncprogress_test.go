package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/taoquo/x-master/apps/macos/internal/xsource"
)

// TestTheProgressLineNamesPagesAndItems covers the sentence under the sync button.
//
// It used to be fmt.Sprintf("%d fetched", ...): English in both locales, a number
// with no unit, and nothing about how far the walk had got. The reference renders
// "Syncing... {pages} pages · {items} items" (src/options/i18n/copy.ts:20) through
// formatSyncProgressLabel, so the template and the substitution are ported rather
// than reinvented.
func TestTheProgressLineNamesPagesAndItems(t *testing.T) {
	shell, _ := newShell(t)
	progress := xsource.Progress{PagesCompleted: 4, Fetched: 60}

	en := syncProgressLabel(shell.t(), progress)
	if !strings.Contains(en, "4") || !strings.Contains(en, "60") {
		t.Errorf("en = %q, want both counters", en)
	}
	if !strings.Contains(en, "pages") || !strings.Contains(en, "items") {
		t.Errorf("en = %q, want the units the reference uses", en)
	}
	if strings.Contains(en, "{") {
		t.Errorf("en = %q, want the placeholders substituted", en)
	}
	if strings.Contains(en, "fetched") {
		t.Errorf("en = %q, the bare count is back", en)
	}

	shell.SetLocale("zh-CN", "en")
	zh := syncProgressLabel(shell.t(), progress)
	if zh == en {
		t.Fatalf("zh = %q, want its own sentence", zh)
	}
	if !strings.Contains(zh, "4") || !strings.Contains(zh, "60") {
		t.Errorf("zh = %q, want both counters", zh)
	}
	if !strings.Contains(zh, "页") || !strings.Contains(zh, "条") {
		t.Errorf("zh = %q, want 页 and 条 as the units", zh)
	}
}

// TestTheLibraryRefreshesAtAWalkingPace covers the rate limit on mid-run reloads.
//
// The service writes a page at a time, so a progress event means there is new
// material to show. Reloading on every one of them would re-read the library and
// rebuild the sidebar four times a second through a first full sync, so the first
// page reloads and the next ones within the pace do not.
func TestTheLibraryRefreshesAtAWalkingPace(t *testing.T) {
	shell, _ := newShell(t)

	if !shell.syncReloadDue() {
		t.Fatal("the first page should refresh the library")
	}
	if shell.syncReloadDue() {
		t.Fatal("a second page inside the pace refreshed the library again")
	}

	// Once the pace has passed, the next page is allowed through.
	shell.lastSyncReload = time.Now().Add(-2 * syncReloadPace)
	if !shell.syncReloadDue() {
		t.Fatal("the library is still not refreshing after the pace elapsed")
	}
}
