package bundle

import (
	"archive/zip"
	"bytes"
	"testing"
)

// TestEntryNamesThatAreRejected covers the archive's path check.
//
// It had no test, which mutation testing found: making checkEntryName accept
// everything left the suite green. That matters more than most gaps, because this is the
// boundary that stops a crafted bundle from writing outside the files it is allowed to
// name -- a zip-slip, and the reason the check exists at all.
func TestEntryNamesThatAreRejected(t *testing.T) {
	rejected := []struct {
		name   string
		reason string
	}{
		{"", "an empty name"},
		{"a\x00b", "a NUL byte"},
		{"/etc/passwd", "an absolute path"},
		{`\windows\system32`, "a Windows absolute path"},
		{`C:secret`, "a drive letter"},
		{"a\\b", "a Windows separator"},
		{"../escape.json", "a parent traversal"},
		{"a/../../escape.json", "a traversal spelled through a subdirectory"},
		{"./sneaky.json", "a path that is not clean"},
		{"sub/dir/file.json", "a nested path"},
		{"a//b", "a doubled separator"},
	}

	for _, tc := range rejected {
		if err := checkEntryName(tc.name); err == nil {
			t.Errorf("checkEntryName(%q) accepted %s", tc.name, tc.reason)
		}
	}
}

// TestEntryNamesThatAreAccepted is the other half: a check that rejects everything is
// also broken, and would be caught by nothing if only the rejections were listed.
func TestEntryNamesThatAreAccepted(t *testing.T) {
	for _, name := range []string{"bookmarks.json", "tags.json", "manifest.json", "a-b_c.1.json"} {
		if err := checkEntryName(name); err != nil {
			t.Errorf("checkEntryName(%q) rejected a plain file name: %v", name, err)
		}
	}
}

// TestCheckEntryNameIsReachedFromRead pins the check to the path a real bundle takes.
//
// Testing the helper alone would still pass if Read stopped calling it, which is the
// same shape of gap: a correct function nobody invokes.
func TestCheckEntryNameIsReachedFromRead(t *testing.T) {
	// A real archive with one bad entry, so the check is exercised through the same
	// path a bundle takes rather than only as a helper.
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	entry, err := archive.Create("../escape.json")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := entry.Write([]byte("{}")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Through Read, which is where the name check lives: checkArchiveEntries only
	// enforces the size limits, and calling it alone would test the wrong barrier.
	if _, err := Read(bytes.NewReader(buffer.Bytes()), int64(buffer.Len()), DefaultLimits()); err == nil {
		t.Error("a bundle with a traversing entry must be refused")
	}
}
