package importexport

import (
	"context"
	"errors"
	"testing"
)

// TestResetLocalDataClearsSecrets covers the hook the composition root installs.
//
// The reset's confirmation promises that local settings are cleared, and the stored
// secrets are part of that. Without this the dialog would promise more than the action
// does, which is worse than not offering it.
func TestResetLocalDataClearsSecrets(t *testing.T) {
	service, _, _ := newService(t)

	cleared := 0
	service.UseSecretClearer(func(context.Context) error {
		cleared++
		return nil
	})

	if err := service.ResetLocalData(context.Background()); err != nil {
		t.Fatalf("ResetLocalData: %v", err)
	}
	if cleared != 1 {
		t.Errorf("the secret hook ran %d times", cleared)
	}
}

// TestAFailingSecretClearStopsTheReset keeps a half-finished reset from looking done.
func TestAFailingSecretClearStopsTheReset(t *testing.T) {
	service, _, _ := newService(t)
	service.UseSecretClearer(func(context.Context) error {
		return errors.New("keychain unavailable")
	})

	if err := service.ResetLocalData(context.Background()); err == nil {
		t.Error("a failing secret clear must be reported, not swallowed")
	}
}
