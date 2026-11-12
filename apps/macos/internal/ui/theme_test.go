package ui

import (
	"testing"

	"github.com/taoquo/trove/apps/macos/internal/domain"
)

// TestStatusValuesMatchTheDomainVocabulary keeps the interface's status words and the
// domain's from drifting apart.
//
// They had already drifted: the domain spelled the partial state "partial_success" --
// as the source's type union, sync writer and copy catalogues all do -- while the badge
// was looking for "partial". A status crossing from one layer to the other would have
// rendered as the idle colour with no error anywhere, which is exactly the kind of
// mismatch that survives every test that only checks one layer.
func TestStatusValuesMatchTheDomainVocabulary(t *testing.T) {
	// Every status the interface can render must be one the domain can produce.
	domainStatuses := map[string]bool{
		string(domain.SyncIdle):           true,
		string(domain.SyncRunning):        true,
		string(domain.SyncSuccess):        true,
		string(domain.SyncPartialSuccess): true,
		string(domain.SyncError):          true,
	}

	for _, status := range AllStatuses {
		if !domainStatuses[status] {
			t.Errorf("the interface renders status %q, which the domain never produces", status)
		}
	}

	// And the partial state in particular: the source's spelling has the underscore.
	if StatusPartial != string(domain.SyncPartialSuccess) {
		t.Errorf("StatusPartial = %q, want the domain's %q", StatusPartial, domain.SyncPartialSuccess)
	}
	if StatusPartial != "partial_success" {
		t.Errorf("StatusPartial = %q, want the source's spelling", StatusPartial)
	}
}
