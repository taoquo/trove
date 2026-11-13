package bundle

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// Two gaps mutation testing found, both in code that only ran one way in the suite.

// TestSettingsWithSeveralKeysRoundTrip covers the separator between settings entries.
//
// Every existing export test carried exactly one setting, so the branch that inserts a
// comma between keys never ran. Inverting it -- writing no separators -- left the suite
// green while producing invalid JSON, which on import means losing every setting at
// once.
func TestSettingsWithSeveralKeysRoundTrip(t *testing.T) {
	data := sampleData()
	data.Settings = map[string]json.RawMessage{
		"app.preferences": raw(`{"appearance":"dark"}`),
		"app.llm":         raw(`{"model":"gpt-4o"}`),
		"app.rules":       raw(`{"keywords":["go"]}`),
	}

	var buffer bytes.Buffer
	if err := Write(&buffer, data, WriteOptions{SourceAppVersion: "0.3.0"}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	bundle, err := Read(bytes.NewReader(buffer.Bytes()), int64(buffer.Len()), DefaultLimits())
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	// The settings file is one object holding the keys, so the object itself is what
	// must survive.
	var settings map[string]json.RawMessage
	if err := json.Unmarshal(bundle.Files[FileSettings], &settings); err != nil {
		t.Fatalf("the settings file did not parse: %v (body %s)", err, string(bundle.Files[FileSettings]))
	}
	if len(settings) != 3 {
		t.Fatalf("settings = %d keys, want 3: %s", len(settings), string(bundle.Files[FileSettings]))
	}
	for _, key := range []string{"app.preferences", "app.llm", "app.rules"} {
		if _, present := settings[key]; !present {
			t.Errorf("setting %q did not survive the round trip; the file is %s",
				key, string(bundle.Files[FileSettings]))
		}
	}
}

// TestSettingsFileIsValidJSON is the same property stated directly, so a broken
// separator is caught even if the decode above is ever loosened.
func TestSettingsFileIsValidJSON(t *testing.T) {
	data := sampleData()
	data.Settings = map[string]json.RawMessage{
		"a": raw(`{"x":1}`), "b": raw(`{"y":2}`), "c": raw(`{"z":3}`),
	}

	var buffer bytes.Buffer
	if err := Write(&buffer, data, WriteOptions{SourceAppVersion: "0.3.0"}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	bundle, err := Read(bytes.NewReader(buffer.Bytes()), int64(buffer.Len()), DefaultLimits())
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	body := string(bundle.Files[FileSettings])
	if !json.Valid([]byte(body)) {
		t.Errorf("the settings file is not valid JSON: %s", body)
	}
	if strings.Count(body, "\"a\"") != 1 || strings.Count(body, "\"b\"") != 1 {
		t.Errorf("the settings file lost a key: %s", body)
	}
}

// TestRecordLimitIsEnforced covers a limit no test reached.
//
// MaxRecords is two million by default, so no fixture comes near it and the comparison
// was never exercised: inverting it left the suite green. A limit that cannot fail is
// not a limit.
func TestRecordLimitIsEnforced(t *testing.T) {
	var buffer bytes.Buffer
	if err := Write(&buffer, sampleData(), WriteOptions{SourceAppVersion: "0.3.0"}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// A limit low enough that the sample exceeds it.
	limits := DefaultLimits()
	limits.MaxRecords = 2

	_, err := Read(bytes.NewReader(buffer.Bytes()), int64(buffer.Len()), limits)
	if err == nil {
		t.Fatal("a bundle over the record limit must be refused")
	}
	if step := StepOf(err); step != StepLimits {
		t.Errorf("refused at step %q, want %q", step, StepLimits)
	}

	// And a limit above it accepts the same bundle, so the check is a comparison and
	// not a blanket refusal.
	limits.MaxRecords = 1000
	if _, err := Read(bytes.NewReader(buffer.Bytes()), int64(buffer.Len()), limits); err != nil {
		t.Errorf("a bundle under the limit must be accepted: %v", err)
	}
}
