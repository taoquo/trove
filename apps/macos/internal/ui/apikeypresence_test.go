package ui

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/taoquo/trove/apps/macos/internal/credentials"
	"github.com/taoquo/trove/apps/macos/internal/i18n"
	"github.com/taoquo/trove/apps/macos/internal/usecase/tagging"
)

// TestAStoredKeyStillReadsAsStoredAfterReopening is the panel contradicting a working run.
//
// APIKeyStored was only ever written by save and clear, so a key saved in an earlier
// session read as absent: the page said "no key stored" while every run used one. The
// presence is now read when the page loads, per provider.
func TestAStoredKeyStillReadsAsStoredAfterReopening(t *testing.T) {
	shell, db := newBatchShell(t)
	ctx := context.Background()
	tagger := tagging.New(autoTagRepos(db), fixedClock{now: clockBase}, nil)
	shell.UseTagging(tagger)
	store := credentials.New(&memKeychain{})
	shell.UseCredentials(store)

	shell.loadAutoTagSettings(ctx)
	if shell.State().APIKeyStored {
		t.Fatal("an empty Keychain must not read as stored")
	}

	shell.State().APIKeyInput = "sk-test"
	shell.saveAPIKey()
	if !shell.State().APIKeyStored {
		t.Fatal("saving a key must read as stored")
	}

	// Reopening the page is the path that used to forget. It reloads the settings and
	// has to ask the Keychain again, not the in-memory flag.
	shell.OpenTagManagementAutoTag()
	if !shell.State().APIKeyStored {
		t.Fatal("a key stored in an earlier session must still read as stored")
	}

	tester := ui.NewTester(shell.View, 1180, 2400)
	tester.Frame()
	if !tester.HasText(shell.t().T(i18n.SettingsAPIKeyPresent)) {
		t.Error("the page must say the key is stored")
	}
	if tester.HasText(shell.t().T(i18n.SettingsAPIKeyAbsent)) {
		t.Error("the page must not say no key is stored when one is")
	}
}

// TestSwitchingProvidersForgetsTheOtherKeysPresence keeps the answer scoped to the
// provider it describes: a key saved for one endpoint says nothing about another.
func TestSwitchingProvidersForgetsTheOtherKeysPresence(t *testing.T) {
	shell, db := newBatchShell(t)
	ctx := context.Background()
	tagger := tagging.New(autoTagRepos(db), fixedClock{now: clockBase}, nil)
	shell.UseTagging(tagger)
	shell.UseCredentials(credentials.New(&memKeychain{}))

	shell.loadAutoTagSettings(ctx)
	shell.State().APIKeyInput = "sk-test"
	shell.saveAPIKey()
	if !shell.State().APIKeyStored {
		t.Fatal("the fixture needs a stored key")
	}

	labels := shell.providerLabels()
	if len(labels) < 2 {
		t.Skip("only one provider is configured")
	}
	// Pick a label that is not the current one.
	for _, label := range labels {
		if label != shell.State().ProviderLabel {
			shell.State().ProviderLabel = label
			break
		}
	}
	shell.saveAutoTagSettings(ctx)

	if shell.State().APIKeyStored {
		t.Error("another provider's key must not read as this provider's")
	}
	if shell.State().ConnectionTested {
		t.Error("the connection result belongs to the provider that produced it")
	}
}
