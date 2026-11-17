package bundle

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"
)

var exportTime = time.Date(2026, 10, 12, 14, 22, 31, 0, time.UTC)

func raw(value string) json.RawMessage { return json.RawMessage(value) }

// sampleData is a small but complete bundle payload.
func sampleData() ExportData {
	return ExportData{
		Bookmarks: []json.RawMessage{
			raw(`{"tweetId":"1","text":"first"}`),
			raw(`{"tweetId":"2","text":"second"}`),
		},
		Tags: []json.RawMessage{
			raw(`{"id":"t1","name":"Go","canonicalKey":"go","aliases":["golang"],"parentId":"p1"}`),
		},
		BookmarkTags:   []json.RawMessage{raw(`{"bookmarkId":"1","tagId":"t1","source":"manual"}`)},
		Lists:          []json.RawMessage{raw(`{"id":"list-inbox","name":"Inbox"}`)},
		BookmarkLists:  []json.RawMessage{raw(`{"bookmarkId":"1","listId":"list-inbox"}`)},
		TagSuggestions: []json.RawMessage{raw(`{"id":"s1","status":"pending"}`)},
		LLMAnalysis:    []json.RawMessage{raw(`{"bookmarkId":"1","model":"gpt-4o"}`)},
		SavedViews:     []json.RawMessage{raw(`{"id":"v1","name":"Unread"}`)},
		Settings:       map[string]json.RawMessage{"app.preferences": raw(`{"appearance":"dark"}`)},
	}
}

func writeSample(t *testing.T) []byte {
	t.Helper()

	var buffer bytes.Buffer
	if err := Write(&buffer, sampleData(), WriteOptions{
		SourceAppVersion: "0.3.0",
		Baseline:         SourceBaseline{LegacyCommit: "fixture", LegacyDBVersion: 9},
		IncludeSyncRuns:  true,
		Now:              exportTime,
	}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	return buffer.Bytes()
}

func readBytes(t *testing.T, data []byte) *Bundle {
	t.Helper()

	bundle, err := Read(bytes.NewReader(data), int64(len(data)), DefaultLimits())
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	return bundle
}

// --- round trip ---

func TestRoundTrip(t *testing.T) {
	bundle := readBytes(t, writeSample(t))

	if bundle.Manifest.Format != Format {
		t.Errorf("format = %q", bundle.Manifest.Format)
	}
	if bundle.Manifest.FormatVersion != FormatVersion {
		t.Errorf("formatVersion = %d", bundle.Manifest.FormatVersion)
	}
	if bundle.Manifest.ExportedAt != "2026-10-12T14:22:31.000Z" {
		t.Errorf("exportedAt = %q", bundle.Manifest.ExportedAt)
	}

	// The counts describe the data actually written.
	if bundle.Manifest.Counts.Bookmarks != 2 {
		t.Errorf("bookmarks count = %d, want 2", bundle.Manifest.Counts.Bookmarks)
	}
	if bundle.Manifest.Counts.Tags != 1 {
		t.Errorf("tags count = %d, want 1", bundle.Manifest.Counts.Tags)
	}

	bookmarks, err := Decode[map[string]any](bundle, FileBookmarks)
	if err != nil {
		t.Fatalf("Decode bookmarks: %v", err)
	}
	if len(bookmarks) != 2 || bookmarks[0]["tweetId"] != "1" {
		t.Errorf("bookmarks = %v", bookmarks)
	}

	// The alias and parent fields survive, which the plan's manifest omitted.
	tags, err := Decode[map[string]any](bundle, FileTags)
	if err != nil {
		t.Fatalf("Decode tags: %v", err)
	}
	if len(tags) != 1 {
		t.Fatalf("tags = %v", tags)
	}
	if _, present := tags[0]["aliases"]; !present {
		t.Error("tag aliases were dropped from the bundle")
	}
	if _, present := tags[0]["parentId"]; !present {
		t.Error("tag parentId was dropped from the bundle")
	}
}

// TestEveryFileIsDeclaredWithItsHash pins that the manifest describes the bytes
// that were actually written.
func TestEveryFileIsDeclaredWithItsHash(t *testing.T) {
	bundle := readBytes(t, writeSample(t))

	for _, declared := range bundle.Manifest.Files {
		contents, present := bundle.Files[declared.Path]
		if !present {
			t.Errorf("%s is declared but was not read", declared.Path)
			continue
		}
		if int64(len(contents)) != declared.Bytes {
			t.Errorf("%s: declared %d bytes, read %d", declared.Path, declared.Bytes, len(contents))
		}
		sum := sha256.Sum256(contents)
		if got := hex.EncodeToString(sum[:]); !strings.EqualFold(got, declared.SHA256) {
			t.Errorf("%s: hash %s does not match the declaration", declared.Path, got)
		}
	}
}

// TestTheManifestIsWrittenLast pins the order streaming requires.
//
// Each data file is encoded straight into its zip entry, so the manifest can only
// be written once every file's size and hash are known. That is safe because a
// reader indexes the archive by name; the test is here to notice if either side
// ever starts assuming the manifest comes first.
func TestTheManifestIsWrittenLast(t *testing.T) {
	data := writeSample(t)
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("the output must still be a zip: %v", err)
	}
	if len(archive.File) == 0 {
		t.Fatal("the bundle has no entries")
	}
	last := archive.File[len(archive.File)-1].Name
	if last != FileManifest {
		t.Errorf("last entry is %q, want %q", last, FileManifest)
	}
}

// TestExcludedFilesAreAbsent covers the options: an excluded file is not written
// at all, and the manifest says so.
func TestExcludedFilesAreAbsent(t *testing.T) {
	var buffer bytes.Buffer
	if err := Write(&buffer, sampleData(), WriteOptions{
		SourceAppVersion:   "0.3.0",
		IncludeSyncRuns:    false,
		IncludeRawPayloads: false,
		Now:                exportTime,
	}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	bundle := readBytes(t, buffer.Bytes())

	for _, name := range []string{FileSyncRuns, FileRawPayloads} {
		if _, present := bundle.Files[name]; present {
			t.Errorf("%s must not be present when it was excluded", name)
		}
	}
	if bundle.Manifest.Options.IncludeSyncRuns {
		t.Error("the manifest must record that sync runs were excluded")
	}
}

// --- Zip Slip ---

// TestRejectsTraversalNames is the Zip Slip defence. Nothing is written to disk
// by this package, so traversal needs an entry name to be used as a path; these
// names are refused before that could ever happen.
func TestRejectsTraversalNames(t *testing.T) {
	bad := []struct {
		name string
		why  string
	}{
		{"../escape.json", "a relative traversal"},
		{"../../etc/passwd", "a deeper traversal"},
		{"a/../b.json", "a traversal that cleans to something else"},
		{"/etc/passwd", "an absolute path"},
		{`\windows\system32`, "a backslash root"},
		{"C:/windows", "a Windows drive letter"},
		{"nested/bookmarks.json", "a nested path"},
		{"with\\backslash.json", "a Windows separator"},
	}

	for _, tc := range bad {
		t.Run(tc.why, func(t *testing.T) {
			data := buildArchive(t, map[string][]byte{tc.name: []byte("[]")})

			_, err := Read(bytes.NewReader(data), int64(len(data)), DefaultLimits())
			if err == nil {
				t.Fatalf("entry %q must be rejected", tc.name)
			}
			if step := StepOf(err); step != StepZipStructure {
				t.Errorf("rejected at step %q, want %q", step, StepZipStructure)
			}
		})
	}
}

func TestAcceptsCleanRootNames(t *testing.T) {
	// The whitelist's own names must pass the name checks, or nothing would ever
	// import.
	for _, name := range KnownFiles {
		if err := checkEntryName(name); err != nil {
			t.Errorf("whitelisted name %q was rejected: %v", name, err)
		}
	}
}

// --- compression bombs ---

// TestRejectsACompressionBomb covers the ratio check. A few kilobytes of zeroes
// can claim to expand enormously, and the ratio is computed from the declared
// sizes before anything is decompressed.
func TestRejectsACompressionBomb(t *testing.T) {
	// Two megabytes of zeroes compresses to a few hundred bytes.
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	entry, err := archive.CreateHeader(&zip.FileHeader{Name: FileManifest, Method: zip.Deflate})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := entry.Write(make([]byte, 2<<20)); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	limits := DefaultLimits()
	limits.MaxCompressionRatio = 10 // a realistic bundle is nowhere near this

	_, err = Read(bytes.NewReader(buffer.Bytes()), int64(buffer.Len()), limits)
	if err == nil {
		t.Fatal("a highly compressed entry must be rejected")
	}
	if step := StepOf(err); step != StepLimits {
		t.Errorf("rejected at step %q, want %q", step, StepLimits)
	}
}

func TestRejectsTooManyEntries(t *testing.T) {
	entries := map[string][]byte{}
	for i := 0; i < 40; i++ {
		entries[FileManifest+string(rune('a'+i))] = []byte("{}")
	}
	data := buildArchive(t, entries)

	limits := DefaultLimits()
	limits.MaxEntries = 8

	_, err := Read(bytes.NewReader(data), int64(len(data)), limits)
	if err == nil {
		t.Fatal("too many entries must be rejected")
	}
	if step := StepOf(err); step != StepLimits {
		t.Errorf("rejected at step %q, want %q", step, StepLimits)
	}
}

// TestRejectsAnOversizedDeclaredFile covers the size limit: a declared size is a
// claim, and a lying archive must be refused before its bytes are materialised.
func TestRejectsAnOversizedDeclaredFile(t *testing.T) {
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	entry, err := archive.CreateHeader(&zip.FileHeader{Name: FileManifest, Method: zip.Deflate})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := entry.Write(bytes.Repeat([]byte("a"), 5000)); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	limits := DefaultLimits()
	limits.MaxFileBytes = 1024
	limits.MaxCompressionRatio = 0 // isolate the size check

	_, err = Read(bytes.NewReader(buffer.Bytes()), int64(buffer.Len()), limits)
	if err == nil {
		t.Fatal("an oversized file must be rejected")
	}
	if step := StepOf(err); step != StepLimits {
		t.Errorf("rejected at step %q, want %q", step, StepLimits)
	}
}

// --- manifest and integrity ---

func TestRejectsAMissingManifest(t *testing.T) {
	data := buildArchive(t, map[string][]byte{FileBookmarks: []byte("[]")})

	_, err := Read(bytes.NewReader(data), int64(len(data)), DefaultLimits())
	if err == nil {
		t.Fatal("a bundle without a manifest must be rejected")
	}
	if step := StepOf(err); step != StepManifest {
		t.Errorf("rejected at step %q, want %q", step, StepManifest)
	}
}

func TestRejectsAWrongFormat(t *testing.T) {
	manifest := `{"format":"something-else","formatVersion":1,"files":[]}`
	data := buildArchive(t, map[string][]byte{FileManifest: []byte(manifest)})

	_, err := Read(bytes.NewReader(data), int64(len(data)), DefaultLimits())
	if err == nil {
		t.Fatal("a foreign format must be rejected")
	}
	if step := StepOf(err); step != StepManifest {
		t.Errorf("rejected at step %q", step)
	}
}

// TestRejectsAFutureFormatVersion is the forward-compatibility rule: a bundle from
// a newer build is refused with advice to update, never partially read.
func TestRejectsAFutureFormatVersion(t *testing.T) {
	manifest := `{"format":"trove-bundle","formatVersion":99,"files":[]}`
	data := buildArchive(t, map[string][]byte{FileManifest: []byte(manifest)})

	_, err := Read(bytes.NewReader(data), int64(len(data)), DefaultLimits())
	if err == nil {
		t.Fatal("a newer format version must be rejected")
	}
	if !strings.Contains(err.Error(), "update the app") {
		t.Errorf("the error must say what to do, got %v", err)
	}
}

func TestRejectsACorruptFile(t *testing.T) {
	var buffer bytes.Buffer
	body := []byte(`[{"tweetId":"1"}]`)
	if err := Write(&buffer, ExportData{Bookmarks: []json.RawMessage{raw(`{"tweetId":"1"}`)}}, WriteOptions{
		SourceAppVersion: "0.3.0", Now: exportTime,
	}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Rebuild the archive with the same manifest but a different bookmarks file,
	// so the declared hash no longer matches.
	bundle := readBytes(t, buffer.Bytes())
	manifestBody := bundle.Files[FileManifest]

	tampered := buildArchive(t, map[string][]byte{
		FileManifest:  manifestBody,
		FileBookmarks: append(body, []byte(" ")...),
	})

	_, err := Read(bytes.NewReader(tampered), int64(len(tampered)), DefaultLimits())
	if err == nil {
		t.Fatal("a file whose hash does not match must be rejected")
	}
	if step := StepOf(err); step != StepChecksum {
		t.Errorf("rejected at step %q, want %q", step, StepChecksum)
	}
}

func TestRejectsAMissingDeclaredFile(t *testing.T) {
	var buffer bytes.Buffer
	if err := Write(&buffer, sampleData(), WriteOptions{
		SourceAppVersion: "0.3.0", Now: exportTime,
	}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	bundle := readBytes(t, buffer.Bytes())

	// Keep the manifest, drop one declared file.
	files := map[string][]byte{FileManifest: bundle.Files[FileManifest]}
	for name, contents := range bundle.Files {
		if name != FileManifest && name != FileTags {
			files[name] = contents
		}
	}
	tampered := buildArchive(t, files)

	_, err := Read(bytes.NewReader(tampered), int64(len(tampered)), DefaultLimits())
	if err == nil {
		t.Fatal("a declared but absent file must be rejected")
	}
	if step := StepOf(err); step != StepFileMissing {
		t.Errorf("rejected at step %q, want %q", step, StepFileMissing)
	}
}

// TestRejectsAWrongRecordCount covers the manifest lying about how many records a
// file holds, which would otherwise make a preview wrong.
func TestRejectsAWrongRecordCount(t *testing.T) {
	body := []byte(`[{"tweetId":"1"}]`)
	sum := sha256.Sum256(body)

	manifest := Manifest{
		Format:        Format,
		FormatVersion: 1,
		Files: []FileEntry{{
			Path: FileBookmarks, Records: 99, Bytes: int64(len(body)),
			SHA256: hex.EncodeToString(sum[:]),
		}},
	}
	manifestBody, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	data := buildArchive(t, map[string][]byte{
		FileManifest:  manifestBody,
		FileBookmarks: body,
	})

	_, err = Read(bytes.NewReader(data), int64(len(data)), DefaultLimits())
	if err == nil {
		t.Fatal("a wrong record count must be rejected")
	}
	if step := StepOf(err); step != StepRecordCount {
		t.Errorf("rejected at step %q, want %q", step, StepRecordCount)
	}
}

// TestValidationIsAllOrNothing pins that a failure yields no bundle at all, so a
// caller cannot act on a partially validated file.
func TestValidationIsAllOrNothing(t *testing.T) {
	var buffer bytes.Buffer
	if err := Write(&buffer, sampleData(), WriteOptions{
		SourceAppVersion: "0.3.0", Now: exportTime,
	}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	bundle := readBytes(t, buffer.Bytes())

	// Corrupt one file, leaving the rest valid.
	files := map[string][]byte{}
	for name, contents := range bundle.Files {
		files[name] = contents
	}
	files[FileTags] = []byte(`[{"id":"t1","name":"tampered"}]`)
	tampered := buildArchive(t, files)

	result, err := Read(bytes.NewReader(tampered), int64(len(tampered)), DefaultLimits())
	if err == nil {
		t.Fatal("a tampered bundle must be rejected")
	}
	if result != nil {
		t.Error("a failed read must return no bundle, so nothing can be acted on")
	}
}

// TestUnknownExtraEntriesAreIgnored keeps a forward-compatible reader working: a
// file this build does not know is not an error, because the manifest is what is
// acted on.
func TestUnknownExtraEntriesAreIgnored(t *testing.T) {
	var buffer bytes.Buffer
	if err := Write(&buffer, sampleData(), WriteOptions{
		SourceAppVersion: "0.3.0", Now: exportTime,
	}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	bundle := readBytes(t, buffer.Bytes())

	files := map[string][]byte{}
	for name, contents := range bundle.Files {
		files[name] = contents
	}
	files["future-thing.json"] = []byte(`{"anything":true}`)

	data := buildArchive(t, files)
	if _, err := Read(bytes.NewReader(data), int64(len(data)), DefaultLimits()); err != nil {
		t.Errorf("an undeclared extra file must not fail the read: %v", err)
	}
}

func TestNotAZipIsRejected(t *testing.T) {
	data := []byte("this is not a zip file at all")

	_, err := Read(bytes.NewReader(data), int64(len(data)), DefaultLimits())
	if err == nil {
		t.Fatal("a non-ZIP must be rejected")
	}
	if step := StepOf(err); step != StepZipStructure {
		t.Errorf("rejected at step %q, want %q", step, StepZipStructure)
	}
}

// --- the export allowlist ---

// TestWriteRefusesANonBundleFile keeps a caller from smuggling a file in.
func TestWriteRefusesANonBundleFile(t *testing.T) {
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)

	if err := writeEntry(archive, "../../evil.json", []byte("{}")); err == nil {
		t.Error("writing a non-bundle file must be refused")
	}
	if err := writeEntry(archive, "notes.txt", []byte("{}")); err == nil {
		t.Error("writing an unknown file must be refused")
	}
	archive.Close()
}

// TestSettingsAreSuppliedAlreadyFiltered pins the layering: which keys may leave
// the machine is decided by the settings package, and the writer only carries what
// it is handed. This test asserts the writer does not add anything on its own.
func TestSettingsAreSuppliedAlreadyFiltered(t *testing.T) {
	var buffer bytes.Buffer
	if err := Write(&buffer, ExportData{
		Settings: map[string]json.RawMessage{"app.preferences": raw(`{"appearance":"dark"}`)},
	}, WriteOptions{SourceAppVersion: "0.3.0", Now: exportTime}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	bundle := readBytes(t, buffer.Bytes())
	settings, present := bundle.Files[FileSettings]
	if !present {
		t.Fatal("the settings file is missing")
	}
	if strings.Contains(string(settings), "endpoints") {
		t.Error("the writer must not add keys that were not supplied")
	}
}

// --- helpers ---

// buildArchive builds a ZIP with the given entries, bypassing Write so a test can
// construct archives that Write would refuse to produce.
func buildArchive(t *testing.T, entries map[string][]byte) []byte {
	t.Helper()

	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)

	// Deterministic order, so a failure is reproducible.
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		entry, err := archive.CreateHeader(header)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		if _, err := entry.Write(entries[name]); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	if err := archive.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	return buffer.Bytes()
}

func TestStepOfOnAnUnrelatedError(t *testing.T) {
	if step := StepOf(errors.New("something else")); step != "" {
		t.Errorf("step = %q, want empty", step)
	}
}

// TestAFormatVersionOneBundleIsStillRead is the compatibility half of the version bump.
//
// Version 2 added tombstones.json. A version 1 archive does not carry it, and its
// absence means "nothing was deleted locally" -- the only thing a version 1 backup
// could know. Reading must accept it, and the missing file must decode as no deletes
// rather than as an error or as a partial bundle.
func TestAFormatVersionOneBundleIsStillRead(t *testing.T) {
	newer := readBytes(t, writeSample(t))

	var manifest Manifest
	if err := json.Unmarshal(newer.Files[FileManifest], &manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	manifest.FormatVersion = 1
	manifest.Counts.Tombstones = 0
	files := manifest.Files[:0]
	for _, file := range manifest.Files {
		if file.Path != FileTombstones {
			files = append(files, file)
		}
	}
	manifest.Files = files

	entries := make(map[string][]byte, len(newer.Files))
	for name, body := range newer.Files {
		if name == FileTombstones {
			continue
		}
		entries[name] = body
	}
	manifestBody, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("encode manifest: %v", err)
	}
	entries[FileManifest] = manifestBody

	older := readBytes(t, buildArchive(t, entries))
	if _, present := older.Files[FileTombstones]; present {
		t.Fatalf("the archive still holds %s", FileTombstones)
	}
	tombstones, err := Decode[map[string]any](older, FileTombstones)
	if err != nil {
		t.Fatalf("Decode tombstones: %v", err)
	}
	if len(tombstones) != 0 {
		t.Errorf("a version 1 bundle yields %d tombstones, want none", len(tombstones))
	}
}
