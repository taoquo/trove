package logging

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTheLogIsOffUntilTheReaderAsks covers both halves of the switch: off creates nothing
// at all, and on creates a private log that records the event it was given.
func TestTheLogIsOffUntilTheReaderAsks(t *testing.T) {
	dir := filepath.Join(t.TempDir(), DirName)
	logs := New(dir)
	t.Cleanup(func() { logs.Close() })

	if err := logs.Apply(false); err != nil {
		t.Fatalf("Apply(false): %v", err)
	}
	if _, err := os.Stat(dir); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("logging off created %s", dir)
	}
	// An event while closed is dropped rather than buffered: a line that appeared after
	// the reader turned logging off would be a surprise.
	logs.Event("launch")

	if err := logs.Apply(true); err != nil {
		t.Fatalf("Apply(true): %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Stat the folder: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o700 {
		t.Errorf("the log folder is %o, want 700", mode)
	}
	path := filepath.Join(dir, FileName)
	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat the log: %v", err)
	}
	if mode := fileInfo.Mode().Perm(); mode != 0o600 {
		t.Errorf("the log is %o, want 600", mode)
	}

	logs.Event("launch", Count{Name: "migrations", Value: 14})
	body := readFile(t, path)
	if !strings.Contains(body, "launch") || !strings.Contains(body, "migrations=14") {
		t.Errorf("the log does not hold the event: %q", body)
	}
}

// TestAClosedLogWritesNothing pins the other direction: closing stops the writing rather
// than leaving a handler pointed at a closed file.
func TestAClosedLogWritesNothing(t *testing.T) {
	logs := New(filepath.Join(t.TempDir(), DirName))
	t.Cleanup(func() { logs.Close() })
	if err := logs.Apply(true); err != nil {
		t.Fatalf("Apply(true): %v", err)
	}
	logs.Event("launch")

	if err := logs.Apply(false); err != nil {
		t.Fatalf("Apply(false): %v", err)
	}
	path := filepath.Join(logs.dir, FileName)
	before := readFile(t, path)
	logs.Event("after the switch")

	if after := readFile(t, path); after != before {
		t.Errorf("a closed log grew: %q -> %q", before, after)
	}
}

// TestTheLogRotatesAndKeepsThree runs the file past its limit and checks the set: the
// current file and the two before it, nothing older, and the oldest lines are the ones
// that went.
func TestTheLogRotatesAndKeepsThree(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	file, err := openRotating(path, 256, Keep)
	if err != nil {
		t.Fatalf("openRotating: %v", err)
	}
	defer file.Close()

	// 50-byte lines, so five fit in 256 bytes and every rotation shifts exactly five.
	const lines = 40
	for i := 0; i < lines; i++ {
		line := fmt.Sprintf("%02d", i) + strings.Repeat("a", 47) + "\n"
		if _, err := file.Write([]byte(line)); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
	}

	dir := filepath.Dir(path)
	var body string
	for _, name := range []string{FileName, FileName + ".1", FileName + ".2"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if info.Size() > 256 {
			t.Errorf("%s is %d bytes, above the limit", name, info.Size())
		}
		body += readFile(t, filepath.Join(dir, name))
	}
	if got := strings.Count(body, "\n"); got != 15 {
		t.Errorf("%d lines survived, want the newest 15", got)
	}
	if !strings.Contains(body, "25") || !strings.Contains(body, "39") {
		t.Errorf("the surviving lines are not the newest ones")
	}
	if strings.Contains(body, "24") {
		t.Error("rotation kept a line older than the three files can hold")
	}
	if current := readFile(t, path); !strings.Contains(current, "39") {
		t.Errorf("the newest line is not in the current file: %q", current)
	}

	if _, err := os.Stat(path + ".3"); !errors.Is(err, fs.ErrNotExist) {
		t.Error("rotation kept a fourth file")
	}
}

// TestASecretNeverReachesTheDisk is the red line as a check: the shapes a key or a cookie
// takes are removed by the writer, whichever logger composed the line.
func TestASecretNeverReachesTheDisk(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	file, err := openRotating(path, MaxBytes, Keep)
	if err != nil {
		t.Fatalf("openRotating: %v", err)
	}
	defer file.Close()
	writer := &redactWriter{w: file}

	const (
		valueOne   = "cookie-value-one"
		valueTwo   = "cookie-value-two"
		bearer     = "bearer-token-value"
		modelKey   = "sk-model-key-value"
		valueThree = "cookie-value-three"
	)
	lines := []string{
		"cookie: ct0=" + valueOne + "; auth_token=" + valueTwo,
		"authorization: Bearer " + bearer,
		"model said 401 for " + modelKey,
		"url failed: https://example.test/?ct0=" + valueThree + "&x=1",
	}
	secretValues := []string{valueOne, valueTwo, bearer, modelKey, valueThree}
	for _, line := range lines {
		if _, err := writer.Write([]byte(line + "\n")); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}
	// An ordinary event stays readable; redaction is not a blanket.
	if _, err := writer.Write([]byte("synced 120 posts in 4 pages\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	body := readFile(t, path)
	for _, value := range secretValues {
		if strings.Contains(body, value) {
			t.Errorf("the log holds %q: %s", value, body)
		}
	}
	if !strings.Contains(body, redacted) {
		t.Errorf("nothing was redacted: %s", body)
	}
	if !strings.Contains(body, "synced 120 posts in 4 pages") {
		t.Errorf("an ordinary line was disturbed: %s", body)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile %s: %v", path, err)
	}
	return string(body)
}
