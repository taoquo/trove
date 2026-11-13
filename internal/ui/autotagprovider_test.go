package ui

import (
	"context"
	"testing"
)

// The provider picker has one trap.
//
// Choosing a different provider must not leave the endpoint field as the previous provider had
// it. The key is sent to the address in that field, so a reader who had pointed "Custom" at a
// self-hosted gateway and then switched to OpenAI would post the OpenAI key to the old host.
// The picker's own change handler answers this: a provider that needs an endpoint starts empty,
// every other provider starts at its documented default.

func TestSwitchingProviderDoesNotKeepTheOldEndpoint(t *testing.T) {
	shell, _, tagger := shellWithTagging(t)
	ctx := context.Background()

	// Start as a reader who configured a self-hosted endpoint.
	settings, err := tagger.LoadSettings(ctx)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	settings.LLM.ProviderID = "custom"
	settings.LLM.BaseURL = "https://old.example/v1"
	if err := tagger.SaveSettings(ctx, settings); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	shell.state.ProviderLabel = shell.providerLabel("custom")
	shell.state.BaseURLInput = "https://old.example/v1"
	// A typed-but-unsaved key. The value is a stand-in; nothing in this test reaches a
	// provider or the Keychain.
	shell.state.APIKeyInput = "draft-key-for-the-old-provider"

	// Pick OpenAI: it has its own endpoint, so the old one must not survive.
	shell.state.ProviderLabel = shell.providerLabel("openai")
	shell.saveAutoTagSettings(ctx)

	stored, err := tagger.LoadSettings(ctx)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if stored.LLM.BaseURL != "https://api.openai.com/v1" {
		t.Errorf("after switching to OpenAI the endpoint is %q: the previous provider's URL "+
			"survived and the OpenAI key would go there", stored.LLM.BaseURL)
	}
	if shell.state.APIKeyInput != "" {
		t.Errorf("the draft key for the old provider is still in the field: %q", shell.state.APIKeyInput)
	}

	// And the other direction: a provider that needs an endpoint starts with a blank one.
	shell.state.ProviderLabel = shell.providerLabel("custom")
	shell.saveAutoTagSettings(ctx)

	stored, err = tagger.LoadSettings(ctx)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if stored.LLM.BaseURL != "" {
		t.Errorf("after switching to Custom the endpoint is %q, want it blank", stored.LLM.BaseURL)
	}
}
