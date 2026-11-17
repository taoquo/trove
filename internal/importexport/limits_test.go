package importexport

// Negative samples for the flat-backup limits.
//
// A legacy backup is one JSON object, so reading it means unmarshalling all of it:
// the format has no way to stream, and nothing in it bounds the size. These tests
// hold the refusal that stands between a pathological file and a crash.

import (
	"errors"
	"strconv"
	"strings"
	"testing"
)

// TestAFlatBackupOverTheSizeLimitIsRefused covers the byte ceiling.
func TestAFlatBackupOverTheSizeLimitIsRefused(t *testing.T) {
	data := make([]byte, LegacyMaxBytes+1)
	data[0] = '{'

	_, _, err := ParseLegacy(data)
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("error = %v, want ErrValidation", err)
	}
	if !strings.Contains(err.Error(), "MB") {
		t.Errorf("the sentence must say how large the file is: %v", err)
	}
}

// TestAFlatBackupWithTooManyRecordsIsRefused covers the row ceiling, which a file
// of tiny records can reach well under the byte limit.
func TestAFlatBackupWithTooManyRecordsIsRefused(t *testing.T) {
	_, _, err := ParseLegacy(flatBackupWith(t, LegacyMaxRecords+1))
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("error = %v, want ErrValidation", err)
	}
	if !strings.Contains(err.Error(), strconv.Itoa(LegacyMaxRecords)) {
		t.Errorf("the sentence must name the limit: %v", err)
	}
}

// TestAFlatBackupAtTheRecordLimitIsRead pins that the ceiling does not refuse the
// files it is meant to admit: the check is over, not at.
func TestAFlatBackupAtTheRecordLimitIsRead(t *testing.T) {
	if _, _, err := ParseLegacy(flatBackupWith(t, LegacyMaxRecords)); err != nil {
		t.Fatalf("a backup at the limit must be read: %v", err)
	}
}

// flatBackupWith renders a version-2 backup holding n empty bookmarks.
func flatBackupWith(t *testing.T, n int) []byte {
	t.Helper()

	var builder strings.Builder
	builder.Grow(n*3 + 32)
	builder.WriteString(`{"exportVersion":2,"bookmarks":[`)
	for i := 0; i < n; i++ {
		if i > 0 {
			builder.WriteByte(',')
		}
		builder.WriteString(`{}`)
	}
	builder.WriteString(`]}`)
	return []byte(builder.String())
}
