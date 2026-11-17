package ui

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/internal/i18n"
	"github.com/taoquo/trove/internal/llm"
)

// The endpoint field decides where the API key is sent, so it is the one field the panel
// does not store as typed.
//
// https is stored, and so is http on a loopback address: a local model server legitimately
// speaks it. http anywhere else would put the key on the wire in the clear, and that is a
// question rather than a refusal -- it is stored only after the reader allows that origin.
// A value that is not a URL cannot become a valid one by being saved, so it is refused
// inline.
func TestTheEndpointFieldRefusesPlaintextUntilAllowed(t *testing.T) {
	shell, _, tagger := shellWithTagging(t)
	ctx := context.Background()

	const storedEndpoint = "https://gateway.example/v1"

	// The panel saves every field it draws, so a shell that has not read the stored
	// settings would write zeros over them. loadAutoTagSettings is the path the page uses.
	reset := func() {
		t.Helper()
		settings, err := tagger.LoadSettings(ctx)
		if err != nil {
			t.Fatalf("LoadSettings: %v", err)
		}
		settings.LLM.ProviderID = "custom"
		settings.LLM.BaseURL = storedEndpoint
		if err := tagger.SaveSettings(ctx, settings); err != nil {
			t.Fatalf("SaveSettings: %v", err)
		}
		shell.loadAutoTagSettings(ctx)
	}

	stored := func() string {
		t.Helper()
		settings, err := tagger.LoadSettings(ctx)
		if err != nil {
			t.Fatalf("LoadSettings: %v", err)
		}
		return settings.LLM.BaseURL
	}

	cases := []struct {
		name        string
		endpoint    string
		approved    string
		wantStored  string
		wantPending string
		wantError   string
	}{
		{
			name:       "https is stored",
			endpoint:   "https://api.example.com/v1",
			wantStored: "https://api.example.com/v1",
		},
		{
			name:       "http on localhost is stored",
			endpoint:   "http://localhost:11434/v1",
			wantStored: "http://localhost:11434/v1",
		},
		{
			name:       "http on 127.0.0.1 is stored",
			endpoint:   "http://127.0.0.1:8080/v1",
			wantStored: "http://127.0.0.1:8080/v1",
		},
		{
			// The assertion this item exists for: the old endpoint stays stored and the
			// new one waits for an answer.
			name:        "http elsewhere waits for an answer",
			endpoint:    "http://1.2.3.4:8000/v1",
			wantStored:  storedEndpoint,
			wantPending: "http://1.2.3.4:8000/*",
		},
		{
			name:       "an allowed origin is stored",
			endpoint:   "http://1.2.3.4:8000/v1",
			approved:   "http://1.2.3.4:8000/*",
			wantStored: "http://1.2.3.4:8000/v1",
		},
		{
			name:       "a malformed endpoint is refused inline",
			endpoint:   "not a url",
			wantStored: storedEndpoint,
			wantError:  llm.BaseURLInvalid,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reset()
			shell.state.BaseURLApprovedOrigin = tc.approved
			shell.state.BaseURLInput = tc.endpoint

			shell.saveAutoTagSettings(ctx)

			if got := stored(); got != tc.wantStored {
				t.Errorf("stored endpoint is %q, want %q", got, tc.wantStored)
			}
			if shell.state.BaseURLPendingOrigin != tc.wantPending {
				t.Errorf("pending origin is %q, want %q",
					shell.state.BaseURLPendingOrigin, tc.wantPending)
			}
			if shell.state.BaseURLErrorKind != tc.wantError {
				t.Errorf("inline error kind is %q, want %q",
					shell.state.BaseURLErrorKind, tc.wantError)
			}
		})
	}
}

// A refused endpoint must not take the rest of the panel down with it.
//
// The save writes every row's value, so returning early on a bad endpoint would leave a
// switch the reader had already seen move unstored -- the lying-switch defect the save
// path was written to avoid.
func TestARefusedEndpointDoesNotFreezeTheRestOfThePanel(t *testing.T) {
	shell, _, tagger := shellWithTagging(t)
	ctx := context.Background()

	settings, err := tagger.LoadSettings(ctx)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	settings.LLM.ProviderID = "custom"
	settings.LLM.BaseURL = "https://gateway.example/v1"
	if err := tagger.SaveSettings(ctx, settings); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	shell.loadAutoTagSettings(ctx)

	shell.state.BaseURLInput = "http://1.2.3.4:8000/v1"
	shell.state.MaxPerRunInput = 123
	shell.saveAutoTagSettings(ctx)

	stored, err := tagger.LoadSettings(ctx)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if stored.LLM.MaxPerRun != 123 {
		t.Errorf("MaxPerRun is %d after a refused endpoint, want 123: the refusal froze the panel",
			stored.LLM.MaxPerRun)
	}
	if stored.LLM.BaseURL != "https://gateway.example/v1" {
		t.Errorf("stored endpoint is %q, want the previous one", stored.LLM.BaseURL)
	}
}

// The question is asked on the page, and Allow is what stores the answer.
func TestThePlaintextQuestionIsAnsweredOnThePage(t *testing.T) {
	shell, _, tagger := shellWithTagging(t)
	ctx := context.Background()
	shell.SetLocale(i18n.EN)
	openAutoTagPage(shell)

	settings, err := tagger.LoadSettings(ctx)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	settings.LLM.ProviderID = "custom"
	settings.LLM.BaseURL = "https://gateway.example/v1"
	if err := tagger.SaveSettings(ctx, settings); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	shell.loadAutoTagSettings(ctx)

	shell.state.BaseURLInput = "http://1.2.3.4:8000/v1"
	shell.saveAutoTagSettings(ctx)

	// Tall enough for the endpoint row and the question under it: Click works in
	// coordinates, and a row below the scroll viewport is not hit.
	tester := ui.NewTester(shell.View, 1180, 1600)
	tester.Frame()

	question := shell.t().Tf(i18n.SettingsBaseURLPermission, "http://1.2.3.4:8000/*")
	if !tester.HasText(question) {
		t.Fatalf("the page asks nothing about a plaintext endpoint; texts: %q", tester.Texts())
	}
	if !tester.HasText(shell.t().T(i18n.SettingsBaseURLPermissionHint)) {
		t.Errorf("the question does not say the key would travel unencrypted; texts: %q", tester.Texts())
	}

	if err := tester.Click(shell.t().T(i18n.ActionAllow)); err != nil {
		t.Fatalf("Allow is not clickable: %v", err)
	}
	tester.Frame()

	stored, err := tagger.LoadSettings(ctx)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if stored.LLM.BaseURL != "http://1.2.3.4:8000/v1" {
		t.Errorf("after Allow the stored endpoint is %q, want the plaintext one", stored.LLM.BaseURL)
	}
	if tester.HasText(question) {
		t.Error("the question is still on the page after it was answered")
	}
	if !tester.HasText(shell.t().T(i18n.SettingsBaseURLWarning)) {
		t.Errorf("the endpoint row does not say the key is sent there; texts: %q", tester.Texts())
	}

	// A later save at the same origin does not ask again: the answer belongs to the
	// origin, not to one keystroke.
	shell.state.MaxPerRunInput = 42
	shell.saveAutoTagSettings(ctx)
	if shell.state.BaseURLPendingOrigin != "" {
		t.Errorf("the same allowed origin is asked about again: %q", shell.state.BaseURLPendingOrigin)
	}
}
